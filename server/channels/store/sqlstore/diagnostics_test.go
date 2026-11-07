// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
)

func TestGetDiagnostics(t *testing.T) {
	StoreTest(t, func(t *testing.T, rctx request.CTX, ss store.Store) {
		sqlStore := ss.(*SqlStore)

		diagnostics, err := sqlStore.GetDiagnostics(rctx)
		require.NoError(t, err)
		require.NotNil(t, diagnostics)

		// Pool stats are populated for every supported driver.
		assert.GreaterOrEqual(t, diagnostics.MasterConnectionsInUse, 0)
		assert.GreaterOrEqual(t, diagnostics.MasterConnectionsIdle, 0)

		if sqlStore.DriverName() != model.DatabaseDriverPostgres {
			assert.Nil(t, diagnostics.CacheHitRatio)
			return
		}

		require.NotNil(t, diagnostics.CacheHitRatio)
		assert.GreaterOrEqual(t, *diagnostics.CacheHitRatio, 0.0)
		assert.LessOrEqual(t, *diagnostics.CacheHitRatio, 1.0)
		require.NotNil(t, diagnostics.Deadlocks)
		require.NotNil(t, diagnostics.TempFiles)
		require.NotNil(t, diagnostics.TempBytesMB)
		require.NotNil(t, diagnostics.Rollbacks)
		require.NotNil(t, diagnostics.IdleInTransactionCount)
		require.NotNil(t, diagnostics.LongestQueryDurationSeconds)
		require.NotNil(t, diagnostics.WaitingForLockCount)
		require.NotNil(t, diagnostics.PostsDeadTuples)
		require.NotNil(t, diagnostics.PostsLiveTuples)
		require.NotNil(t, diagnostics.PostsVacuumInProgress)

		const lastVacuumQuery = `
SELECT last_vacuum
FROM pg_stat_user_tables
WHERE lower(relname) = 'posts'
  AND schemaname = current_schema()
LIMIT 1`
		var lastVacuumBefore sql.NullTime
		require.NoError(t, sqlStore.GetMaster().Get(&lastVacuumBefore, lastVacuumQuery))

		_, err = sqlStore.GetMaster().Exec("VACUUM posts")
		require.NoError(t, err)
		var lastVacuumAfter sql.NullTime
		require.NoError(t, sqlStore.GetMaster().Get(&lastVacuumAfter, lastVacuumQuery))
		require.True(t, lastVacuumAfter.Valid)
		assert.True(t, lastVacuumAfter.Time.After(lastVacuumBefore.Time))

		diagnostics, err = sqlStore.GetDiagnostics(rctx)
		require.NoError(t, err)
		require.NotNil(t, diagnostics.PostsLastVacuum)
		assert.Equal(t, time.UTC, diagnostics.PostsLastVacuum.Location())
		assert.Equal(t, lastVacuumAfter.Time.UTC(), *diagnostics.PostsLastVacuum)
	})
}

func TestGetDiagnosticsPostsVacuumInProgress(t *testing.T) {
	// A pooled test store's database can hold a snapshot left open by an earlier test. Any snapshot
	// older than the probe row lets VACUUM (FREEZE) skip the pinned page instead of waiting for it,
	// so the vacuum finishes before it can be observed. Use a fresh database with nothing else
	// connected, and a minimal posts table, which is all the collector needs.
	db := makeVacuumProbeDB(t)
	ctx := context.Background()

	postsVacuumInProgress := func() bool {
		diagnostics := &store.DatabaseDiagnostics{}
		require.NoError(t, collectPGStatUserTablesDiagnostics(ctx, db, diagnostics))
		require.NotNil(t, diagnostics.PostsVacuumInProgress)
		return *diagnostics.PostsVacuumInProgress
	}

	for _, table := range []string{"posts", "vacuumprobe"} {
		_, err := db.Exec("CREATE TABLE " + table + " (id varchar(26) PRIMARY KEY)")
		require.NoError(t, err)
	}

	t.Run("vacuum of posts", func(t *testing.T) {
		release := holdVacuumInProgress(t, db.DB, "posts")
		assert.True(t, postsVacuumInProgress())

		release()
		assert.Eventually(t, func() bool { return !postsVacuumInProgress() }, 10*time.Second, 50*time.Millisecond)
	})

	t.Run("vacuum of an unrelated table", func(t *testing.T) {
		release := holdVacuumInProgress(t, db.DB, "vacuumprobe")
		defer release()
		assert.False(t, postsVacuumInProgress())
	})
}

// makeVacuumProbeDB returns a connection to a new, empty PostgreSQL database that is dropped when
// the test ends.
func makeVacuumProbeDB(t *testing.T) *sqlx.DB {
	t.Helper()

	settings, err := makeSqlSettings(model.DatabaseDriverPostgres)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { storetest.CleanupSqlSettings(settings) })

	sqlStore := &SqlStore{
		settings:    settings,
		logger:      mlog.CreateConsoleTestLogger(t),
		quitMonitor: make(chan struct{}),
		wgMonitor:   &sync.WaitGroup{},
	}
	require.NoError(t, sqlStore.initConnection())
	t.Cleanup(sqlStore.Close)

	return sqlStore.GetMaster().DB()
}

// holdVacuumInProgress inserts a row into table, starts VACUUM (FREEZE) on it and keeps the vacuum
// running until the returned function is called. A cursor in another transaction pins the heap page
// holding the row, and an aggressive vacuum must wait for a cleanup lock on that page before it can
// freeze the row, so the vacuum stays visible in pg_stat_progress_vacuum while the pin is held.
//
// The vacuum only has to freeze the row if the row's xmin precedes every snapshot in the database.
// Otherwise it leaves the row alone, skips the pinned page and finishes at once, so wait for any
// older snapshot, such as an autovacuum worker's ANALYZE, to end before starting the vacuum.
func holdVacuumInProgress(t *testing.T, db *sql.DB, table string) func() {
	t.Helper()

	id := model.NewId()
	_, err := db.Exec("INSERT INTO "+table+" (id) VALUES ($1)", id)
	require.NoError(t, err)

	var rowXmin string
	require.NoError(t, db.QueryRow("SELECT xmin::text FROM "+table+" WHERE id = $1", id).Scan(&rowXmin))
	var olderSnapshots string
	require.Eventually(t, func() bool {
		queryErr := db.QueryRow(`
SELECT coalesce(string_agg(format('pid=%s type=%s state=%s query=%s', pid, backend_type, state, query), '; '), '')
FROM pg_stat_activity
WHERE datname = current_database()
  AND pid <> pg_backend_pid()
  AND (age(backend_xmin) >= age($1::xid) OR age(backend_xid) >= age($1::xid))`,
			rowXmin,
		).Scan(&olderSnapshots)
		return queryErr == nil && olderSnapshots == ""
	}, 30*time.Second, 50*time.Millisecond, "snapshots older than the %s probe row did not end: %s", table, olderSnapshots)

	tx, err := db.Begin()
	require.NoError(t, err)

	// Register the release before anything can fail, so a failed assertion below still ends the
	// transaction instead of leaving its lock to block later cleanup such as DROP DATABASE.
	var vacuumDone chan error
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		assert.NoError(t, tx.Rollback())
		if vacuumDone == nil {
			return
		}
		select {
		case vacuumErr := <-vacuumDone:
			assert.NoError(t, vacuumErr)
		case <-time.After(30 * time.Second):
			assert.Fail(t, "VACUUM did not finish after the page pin was released", table)
		}
	}
	t.Cleanup(release)

	// DECLARE does not accept bind parameters; table is a fixed test identifier and id is generated by model.NewId.
	_, err = tx.Exec("DECLARE vacuum_pin CURSOR FOR SELECT id FROM " + table + " WHERE id = '" + id + "'")
	require.NoError(t, err)
	var pinned string
	require.NoError(t, tx.QueryRow("FETCH 1 FROM vacuum_pin").Scan(&pinned))
	require.Equal(t, id, pinned)

	vacuumDone = make(chan error, 1)
	go func() {
		_, vacuumErr := db.Exec("VACUUM (FREEZE) " + table)
		vacuumDone <- vacuumErr
	}()

	require.Eventually(t, func() bool {
		var running bool
		queryErr := db.QueryRow(
			"SELECT EXISTS (SELECT 1 FROM pg_stat_progress_vacuum WHERE relid = $1::regclass)",
			table,
		).Scan(&running)
		return queryErr == nil && running
	}, 10*time.Second, 50*time.Millisecond, "VACUUM of %s did not start", table)

	return release
}

func TestOnlineReplicas(t *testing.T) {
	replica := func(online bool) *atomic.Pointer[sqlxDBWrapper] {
		w := &sqlxDBWrapper{isOnline: &atomic.Bool{}}
		w.isOnline.Store(online)
		r := &atomic.Pointer[sqlxDBWrapper]{}
		r.Store(w)
		return r
	}

	assert.Equal(t, 0, onlineReplicas(nil))
	assert.Equal(t, 0, onlineReplicas([]*atomic.Pointer[sqlxDBWrapper]{replica(false)}))
	assert.Equal(t, 2, onlineReplicas([]*atomic.Pointer[sqlxDBWrapper]{replica(true), replica(false), replica(true)}))
}

func TestApplyDBPoolStats(t *testing.T) {
	diagnostics := &store.DatabaseDiagnostics{}
	applyDBPoolStats(
		diagnostics,
		sql.DBStats{
			InUse:             3,
			Idle:              7,
			WaitCount:         11,
			WaitDuration:      2*time.Second + 25*time.Millisecond,
			MaxIdleClosed:     13,
			MaxLifetimeClosed: 17,
		},
		sql.DBStats{
			InUse:             5,
			Idle:              9,
			WaitCount:         19,
			WaitDuration:      4*time.Second + 90*time.Millisecond,
			MaxIdleClosed:     23,
			MaxLifetimeClosed: 29,
		},
	)

	assert.Equal(t, 3, diagnostics.MasterConnectionsInUse)
	assert.Equal(t, 7, diagnostics.MasterConnectionsIdle)
	assert.Equal(t, int64(11), diagnostics.MasterPoolWaitCount)
	assert.Equal(t, int64(2025), diagnostics.MasterPoolWaitDurationMs)
	assert.Equal(t, int64(13), diagnostics.MasterConnectionsClosedMaxIdle)
	assert.Equal(t, int64(17), diagnostics.MasterConnectionsClosedMaxLifetime)
	assert.Equal(t, 5, diagnostics.ReplicaConnectionsInUse)
	assert.Equal(t, 9, diagnostics.ReplicaConnectionsIdle)
	assert.Equal(t, int64(19), diagnostics.ReplicaPoolWaitCount)
	assert.Equal(t, int64(4090), diagnostics.ReplicaPoolWaitDurationMs)
	assert.Equal(t, int64(23), diagnostics.ReplicaConnectionsClosedMaxIdle)
	assert.Equal(t, int64(29), diagnostics.ReplicaConnectionsClosedMaxLifetime)
}
