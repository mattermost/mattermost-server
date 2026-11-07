// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

type SqlSystemStore struct {
	*SqlStore

	systemSelectQuery sq.SelectBuilder
}

func newSqlSystemStore(sqlStore *SqlStore) store.SystemStore {
	s := SqlSystemStore{SqlStore: sqlStore}
	s.systemSelectQuery = s.getQueryBuilder().Select("Name", "Value").From("Systems")
	return &s
}

func (s SqlSystemStore) Save(system *model.System) error {
	query := "INSERT INTO Systems (Name, Value) VALUES (:Name, :Value)"
	if _, err := s.GetMaster().NamedExec(query, system); err != nil {
		return errors.Wrapf(err, "failed to save system property with name=%s", system.Name)
	}

	return nil
}

func (s SqlSystemStore) SaveOrUpdate(system *model.System) error {
	query := s.getQueryBuilder().
		Insert("Systems").
		Columns("Name", "Value").
		Values(system.Name, system.Value)

	query = query.SuffixExpr(sq.Expr("ON CONFLICT (name) DO UPDATE SET Value = ?", system.Value))

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "system_tosql")
	}

	if _, err := s.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrap(err, "failed to upsert system property")
	}

	return nil
}

// TryClaimIfOlderThan atomically claims a Systems row for a cooldown/throttle
// pattern: it sets Name's Value to value, but only if the row is absent or its
// existing Value (read as a millisecond timestamp) is at or before nowMillis -
// minAgeMillis, in one statement. Reports whether this call is the one that set
// it.
//
// This exists so a caller doing "read the timestamp, decide, then write it"
// across two separate calls (SaveOrUpdate) cannot have two concurrent callers
// both observe "old enough" before either writes -- the classic double-send
// race for this kind of throttle. Postgres-only: ON CONFLICT ... DO UPDATE ...
// WHERE only performs (and counts as an affected row) the update when the WHERE
// condition holds, so RowsAffected() > 0 is exactly "this call won the claim,"
// with the race window closed by the database rather than by application code.
func (s SqlSystemStore) TryClaimIfOlderThan(name, value string, minAgeMillis int64) (bool, error) {
	threshold := model.GetMillis() - minAgeMillis

	query := s.getQueryBuilder().
		Insert("Systems").
		Columns("Name", "Value").
		Values(name, value)

	// The CASE guards the cast: a malformed marker (never written by this
	// code path, but not impossible -- a manual DB edit, a future bug) would
	// otherwise make Value::bigint raise and this claim (and every one
	// after it, forever) fail with a 500. Treating anything that doesn't
	// look like a plain non-negative integer as 0 -- "as old as possible" --
	// instead matches the pre-atomic-claim behavior, which read the value
	// with strconv.ParseInt and treated any parse failure as never-sent.
	query = query.SuffixExpr(sq.Expr(
		"ON CONFLICT (name) DO UPDATE SET Value = ? WHERE (CASE WHEN Systems.Value ~ '^[0-9]+$' THEN Systems.Value::bigint ELSE 0 END) <= ?",
		value, threshold,
	))

	queryString, args, err := query.ToSql()
	if err != nil {
		return false, errors.Wrap(err, "system_tosql")
	}

	result, err := s.GetMaster().Exec(queryString, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to atomically claim system throttle")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "failed to read rows affected for system throttle claim")
	}

	return rows > 0, nil
}

// DeleteIfValueEquals atomically deletes name's row only if its current Value
// still equals expectedValue, and reports whether it deleted anything.
//
// This exists to release a TryClaimIfOlderThan claim without a TOCTOU gap: an
// unconditional delete-by-name would delete whatever is there *now*, not
// specifically the row this caller claimed. If this caller's own claim
// outlives the cooldown window before it gets around to releasing (a slow
// request, a stalled goroutine), a second caller can legitimately claim in
// between -- and an unconditional release would then delete that second
// claim, opening the door to a third, overlapping one. Matching on both name
// and the exact value this caller wrote makes the delete a no-op once
// superseded.
func (s SqlSystemStore) DeleteIfValueEquals(name, expectedValue string) (bool, error) {
	result, err := s.GetMaster().Exec("DELETE FROM Systems WHERE Name = ? AND Value = ?", name, expectedValue)
	if err != nil {
		return false, errors.Wrap(err, "failed to conditionally delete system property")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "failed to read rows affected for conditional system property delete")
	}

	return rows > 0, nil
}

func (s SqlSystemStore) Update(system *model.System) error {
	query := "UPDATE Systems SET Value=:Value WHERE Name=:Name"
	if _, err := s.GetMaster().NamedExec(query, system); err != nil {
		return errors.Wrapf(err, "failed to update system property with name=%s", system.Name)
	}

	return nil
}

func (s SqlSystemStore) Get() (model.StringMap, error) {
	return s.GetWithContext(request.EmptyContext(s.logger))
}

func (s SqlSystemStore) GetWithContext(rctx request.CTX) (model.StringMap, error) {
	systems := []model.System{}
	props := make(model.StringMap)

	query := s.systemSelectQuery
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&systems, query); err != nil {
		return nil, errors.Wrap(err, "failed to get System list")
	}

	for _, prop := range systems {
		props[prop.Name] = prop.Value
	}

	return props, nil
}

func (s SqlSystemStore) GetByName(name string) (*model.System, error) {
	return s.GetByNameWithContext(store.RequestContextWithMaster(request.EmptyContext(s.logger)), name)
}

func (s SqlSystemStore) GetByNameWithContext(rctx request.CTX, name string) (*model.System, error) {
	var system model.System
	query := s.systemSelectQuery.Where(sq.Eq{"Name": name})
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&system, query); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("System", fmt.Sprintf("name=%s", system.Name))
		}
		return nil, errors.Wrapf(err, "failed to get system property with name=%s", system.Name)
	}

	return &system, nil
}

func (s SqlSystemStore) PermanentDeleteByName(name string) (*model.System, error) {
	var system model.System
	if _, err := s.GetMaster().Exec("DELETE FROM Systems WHERE Name = ?", name); err != nil {
		return nil, errors.Wrapf(err, "failed to permanent delete system property with name=%s", system.Name)
	}

	return &system, nil
}

// InsertIfExists inserts a given system value if it does not already exist. If a value
// already exists, it returns the old one, else returns the new one.
func (s SqlSystemStore) InsertIfExists(system *model.System) (_ *model.System, err error) {
	tx, err := s.GetMaster().BeginWithIsolation(&sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, errors.Wrap(err, "begin_transaction")
	}
	defer finalizeTransactionX(tx, &err)

	var origSystem model.System
	query := s.systemSelectQuery.Where(sq.Eq{"Name": system.Name})
	if err := tx.GetBuilder(&origSystem, query); err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrapf(err, "failed to get system property with name=%s", system.Name)
	}

	if origSystem.Value != "" {
		// Already a value exists, return that.
		return &origSystem, nil
	}

	// Key does not exist, need to insert.
	if _, err := tx.NamedExec("INSERT INTO Systems (Name, Value) VALUES (:Name, :Value)", system); err != nil {
		return nil, errors.Wrapf(err, "failed to save system property with name=%s", system.Name)
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "commit_transaction")
	}
	return system, nil
}
