// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package database

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var vacuumCollectedAt = time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)

func vacuumSnapshot(dbType *string, dead, live *int64, manual, auto *time.Time) *healthcheck.Snapshot {
	diag := &model.SupportPacketDiagnostics{}
	diag.Database.Type = dbType
	diag.Database.PostsDeadTuples = dead
	diag.Database.PostsLiveTuples = live
	diag.Database.PostsVacuumInProgress = new(false)
	diag.Database.PostsLastVacuum = manual
	diag.Database.PostsLastAutovacuum = auto

	// The exact post count is deliberately absent: the rule must not depend on the Stats section.
	s := newSnapshot(nil, nil, diag)
	s.CollectedAt = vacuumCollectedAt
	return s
}

func TestDBPostsDeadTuples(t *testing.T) {
	t.Parallel()

	postgres := model.DatabaseDriverPostgres
	mysql := "mysql"
	old := vacuumCollectedAt.Add(-8 * 24 * time.Hour)
	exactlySevenDays := vacuumCollectedAt.Add(-7 * 24 * time.Hour)
	recent := vacuumCollectedAt.Add(-time.Hour)
	future := vacuumCollectedAt.Add(time.Hour)
	dead := int64(200_000)
	lowDead := int64(100_000)
	negativeDead := int64(-1)
	posts := int64(500_000)
	positiveRatioPosts := int64(400_000)
	exactRatioPosts := int64(1_000_000)
	zeroPosts := int64(0)

	progressAbsent := vacuumSnapshot(&postgres, &dead, &posts, &old, nil)
	progressAbsentDiag, ok := progressAbsent.LeaderDiag(model.SectionDatabaseStats)
	require.True(t, ok)
	progressAbsentDiag.Database.PostsVacuumInProgress = nil
	vacuumRunning := vacuumSnapshot(&postgres, &dead, &posts, &old, nil)
	vacuumRunningDiag, ok := vacuumRunning.LeaderDiag(model.SectionDatabaseStats)
	require.True(t, ok)
	vacuumRunningDiag.Database.PostsVacuumInProgress = new(true)

	stale := firingWant("health.rule.db_posts_dead_tuples.message.stale").withValue(float64(dead))
	blocked := firingWant("health.rule.db_posts_dead_tuples.message.blocked").withValue(float64(dead))
	testCases := []struct {
		name     string
		snapshot *healthcheck.Snapshot
		want     want
	}{
		{"leader diagnostics absent", newSnapshot(nil, &model.SupportPacketStats{Posts: &posts}, nil), unknownDiagWant},
		{"database type absent", vacuumSnapshot(nil, &dead, &posts, &old, nil), unknownDiagWant},
		{"identity section failed", failSection(vacuumSnapshot(&postgres, &dead, &posts, &old, nil), model.SectionDatabaseIdentity), unknownDiagWant},
		{"non-Postgres database", vacuumSnapshot(&mysql, nil, nil, nil, nil), resolvedWant},
		{"stats section failed", failSection(vacuumSnapshot(&postgres, &dead, &posts, &old, nil), model.SectionDatabaseStats), unknownDiagWant},
		{"dead tuples absent", vacuumSnapshot(&postgres, nil, &posts, &old, nil), unknownDiagWant},
		{"negative dead tuples", vacuumSnapshot(&postgres, &negativeDead, &posts, &old, nil), unknownDiagWant},
		{"live tuples absent", vacuumSnapshot(&postgres, &dead, nil, &old, nil), unknownDiagWant},
		{"zero live tuples with high dead tuples", vacuumSnapshot(&postgres, &dead, &zeroPosts, &old, nil), unknownDiagWant},
		{"at dead tuple threshold", vacuumSnapshot(&postgres, &lowDead, &positiveRatioPosts, &old, nil), resolvedWant.withValue(float64(lowDead))},
		{"at dead tuple ratio", vacuumSnapshot(&postgres, &dead, &exactRatioPosts, &old, nil), resolvedWant.withValue(float64(dead))},
		{"vacuum progress absent", progressAbsent, unknownDiagWant},
		{"vacuum in progress", vacuumRunning, want{state: healthcheck.StateUnknown, messageID: "health.rule.db_posts_dead_tuples.in_progress"}},
		{"manual vacuum recent", vacuumSnapshot(&postgres, &dead, &posts, &recent, &old), blocked},
		{"autovacuum recent", vacuumSnapshot(&postgres, &dead, &posts, &old, &recent), blocked},
		{"at stale threshold", vacuumSnapshot(&postgres, &dead, &posts, &exactlySevenDays, nil), blocked},
		{"manual vacuum stale", vacuumSnapshot(&postgres, &dead, &posts, &old, nil), stale},
		{"autovacuum stale", vacuumSnapshot(&postgres, &dead, &posts, nil, &old), stale},
		{"recent vacuum below dead tuple ratio", vacuumSnapshot(&postgres, &dead, &exactRatioPosts, &recent, nil), resolvedWant.withValue(float64(dead))},
		{"never vacuumed", vacuumSnapshot(&postgres, &dead, &posts, nil, nil), firingWant("health.rule.db_posts_dead_tuples.message.never").withValue(float64(dead))},
		{"never vacuumed below dead tuple ratio", vacuumSnapshot(&postgres, &dead, &exactRatioPosts, nil, nil), resolvedWant.withValue(float64(dead))},
		{"vacuum timestamp after collection", vacuumSnapshot(&postgres, &dead, &posts, &future, nil), unknownDiagWant},
		{
			"collection time absent",
			func() *healthcheck.Snapshot {
				s := vacuumSnapshot(&postgres, &dead, &posts, &old, nil)
				s.CollectedAt = time.Time{}
				return s
			}(),
			want{state: healthcheck.StateUnknown, messageID: healthcheck.ReasonCollectedAtUnknown},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertResult(t, dbPostsDeadTuples, tc.snapshot, tc.want)
		})
	}

	results := dbPostsDeadTuples.Eval(vacuumSnapshot(&postgres, &dead, &posts, &old, nil))
	require.Len(t, results, 1)
	assert.Equal(t, map[string]string{
		"age_days":     "8",
		"dead_percent": "40.0",
		"dead_tuples":  "200000",
		"last_vacuum":  "2026-10-28T12:00:00Z",
		"live_tuples":  "500000",
	}, results[0].Details)

	results = dbPostsDeadTuples.Eval(vacuumSnapshot(&postgres, &dead, &posts, &recent, nil))
	require.Len(t, results, 1)
	assert.Equal(t, map[string]string{
		"dead_percent": "40.0",
		"dead_tuples":  "200000",
		"last_vacuum":  "2026-11-05T11:00:00Z",
		"live_tuples":  "500000",
	}, results[0].Details)

	results = dbPostsDeadTuples.Eval(vacuumSnapshot(&postgres, &dead, &posts, nil, nil))
	require.Len(t, results, 1)
	assert.Equal(t, map[string]string{
		"dead_percent": "40.0",
		"dead_tuples":  "200000",
		"live_tuples":  "500000",
	}, results[0].Details)
}
