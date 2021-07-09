// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	SessionsCleanupDelayMilliseconds = 100
)

type SQLSessionStore struct {
	*SQLStore
}

func newSQLSessionStore(sqlStore *SQLStore) store.SessionStore {
	us := &SQLSessionStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Session{}, "Sessions").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("Token").SetMaxSize(26)
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("DeviceId").SetMaxSize(512)
		table.ColMap("Roles").SetMaxSize(64)
		table.ColMap("Props").SetMaxSize(1000)
	}

	return us
}

func (me SQLSessionStore) createIndexesIfNotExists() {
	me.CreateIndexIfNotExists("idx_sessions_user_id", "Sessions", "UserId")
	me.CreateIndexIfNotExists("idx_sessions_token", "Sessions", "Token")
	me.CreateIndexIfNotExists("idx_sessions_expires_at", "Sessions", "ExpiresAt")
	me.CreateIndexIfNotExists("idx_sessions_create_at", "Sessions", "CreateAt")
	me.CreateIndexIfNotExists("idx_sessions_last_activity_at", "Sessions", "LastActivityAt")
}

func (me SQLSessionStore) Save(session *model.Session) (*model.Session, error) {
	if session.ID != "" {
		return nil, store.NewErrInvalidInput("Session", "id", session.ID)
	}
	session.PreSave()

	if err := me.GetMaster().Insert(session); err != nil {
		return nil, errors.Wrapf(err, "failed to save Session with id=%s", session.ID)
	}

	teamMembers, err := me.Team().GetTeamsForUser(context.Background(), session.UserID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find TeamMembers for Session with userId=%s", session.UserID)
	}

	session.TeamMembers = make([]*model.TeamMember, 0, len(teamMembers))
	for _, tm := range teamMembers {
		if tm.DeleteAt == 0 {
			session.TeamMembers = append(session.TeamMembers, tm)
		}
	}

	return session, nil
}

func (me SQLSessionStore) Get(ctx context.Context, sessionIDOrToken string) (*model.Session, error) {
	var sessions []*model.Session

	if _, err := me.DBFromContext(ctx).Select(&sessions, "SELECT * FROM Sessions WHERE Token = :Token OR Id = :Id LIMIT 1", map[string]interface{}{"Token": sessionIDOrToken, "Id": sessionIDOrToken}); err != nil {
		return nil, errors.Wrapf(err, "failed to find Sessions with sessionIdOrToken=%s", sessionIDOrToken)
	} else if len(sessions) == 0 {
		return nil, store.NewErrNotFound("Session", fmt.Sprintf("sessionIdOrToken=%s", sessionIDOrToken))
	}
	session := sessions[0]

	tempMembers, err := me.Team().GetTeamsForUser(
		WithMaster(context.Background()),
		session.UserID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find TeamMembers for Session with userId=%s", session.UserID)
	}
	sessions[0].TeamMembers = make([]*model.TeamMember, 0, len(tempMembers))
	for _, tm := range tempMembers {
		if tm.DeleteAt == 0 {
			sessions[0].TeamMembers = append(sessions[0].TeamMembers, tm)
		}
	}
	return session, nil
}

func (me SQLSessionStore) GetSessions(userID string) ([]*model.Session, error) {
	var sessions []*model.Session

	if _, err := me.GetReplica().Select(&sessions, "SELECT * FROM Sessions WHERE UserId = :UserId ORDER BY LastActivityAt DESC", map[string]interface{}{"UserId": userID}); err != nil {
		return nil, errors.Wrapf(err, "failed to find Sessions with userId=%s", userID)
	}

	teamMembers, err := me.Team().GetTeamsForUser(context.Background(), userID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find TeamMembers for Session with userId=%s", userID)
	}

	for _, session := range sessions {
		session.TeamMembers = make([]*model.TeamMember, 0, len(teamMembers))
		for _, tm := range teamMembers {
			if tm.DeleteAt == 0 {
				session.TeamMembers = append(session.TeamMembers, tm)
			}
		}
	}
	return sessions, nil
}

func (me SQLSessionStore) GetSessionsWithActiveDeviceIDs(userID string) ([]*model.Session, error) {
	query :=
		`SELECT *
		FROM
			Sessions
		WHERE
			UserId = :UserId AND
			ExpiresAt != 0 AND
			:ExpiresAt <= ExpiresAt AND
			DeviceId != ''`

	var sessions []*model.Session

	_, err := me.GetReplica().Select(&sessions, query, map[string]interface{}{"UserId": userID, "ExpiresAt": model.GetMillis()})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Sessions with userId=%s", userID)
	}
	return sessions, nil
}

func (me SQLSessionStore) GetSessionsExpired(thresholdMillis int64, mobileOnly bool, unnotifiedOnly bool) ([]*model.Session, error) {
	now := model.GetMillis()
	builder := me.getQueryBuilder().
		Select("*").
		From("Sessions").
		Where(sq.NotEq{"ExpiresAt": 0}).
		Where(sq.Lt{"ExpiresAt": now}).
		Where(sq.Gt{"ExpiresAt": now - thresholdMillis})
	if mobileOnly {
		builder = builder.Where(sq.NotEq{"DeviceId": ""})
	}
	if unnotifiedOnly {
		builder = builder.Where(sq.NotEq{"ExpiredNotify": true})
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "sessions_tosql")
	}

	var sessions []*model.Session

	_, err = me.GetReplica().Select(&sessions, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Sessions")
	}
	return sessions, nil
}

func (me SQLSessionStore) UpdateExpiredNotify(sessionID string, notified bool) error {
	query, args, err := me.getQueryBuilder().
		Update("Sessions").
		Set("ExpiredNotify", notified).
		Where(sq.Eq{"Id": sessionID}).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "sessions_tosql")
	}

	_, err = me.GetMaster().Exec(query, args...)
	if err != nil {
		return errors.Wrapf(err, "failed to update Session with id=%s", sessionID)
	}
	return nil
}

func (me SQLSessionStore) Remove(sessionIDOrToken string) error {
	_, err := me.GetMaster().Exec("DELETE FROM Sessions WHERE Id = :Id Or Token = :Token", map[string]interface{}{"Id": sessionIDOrToken, "Token": sessionIDOrToken})
	if err != nil {
		return errors.Wrapf(err, "failed to delete Session with sessionIdOrToken=%s", sessionIDOrToken)
	}
	return nil
}

func (me SQLSessionStore) RemoveAllSessions() error {
	_, err := me.GetMaster().Exec("DELETE FROM Sessions")
	if err != nil {
		return errors.Wrap(err, "failed to delete all Sessions")
	}
	return nil
}

func (me SQLSessionStore) PermanentDeleteSessionsByUser(userID string) error {
	_, err := me.GetMaster().Exec("DELETE FROM Sessions WHERE UserId = :UserId", map[string]interface{}{"UserId": userID})
	if err != nil {
		return errors.Wrapf(err, "failed to delete Session with userId=%s", userID)
	}

	return nil
}

func (me SQLSessionStore) UpdateExpiresAt(sessionID string, time int64) error {
	_, err := me.GetMaster().Exec("UPDATE Sessions SET ExpiresAt = :ExpiresAt, ExpiredNotify = false WHERE Id = :Id", map[string]interface{}{"ExpiresAt": time, "Id": sessionID})
	if err != nil {
		return errors.Wrapf(err, "failed to update Session with sessionId=%s", sessionID)
	}
	return nil
}

func (me SQLSessionStore) UpdateLastActivityAt(sessionID string, time int64) error {
	_, err := me.GetMaster().Exec("UPDATE Sessions SET LastActivityAt = :LastActivityAt WHERE Id = :Id", map[string]interface{}{"LastActivityAt": time, "Id": sessionID})
	if err != nil {
		return errors.Wrapf(err, "failed to update Session with id=%s", sessionID)
	}
	return nil
}

func (me SQLSessionStore) UpdateRoles(userID, roles string) (string, error) {
	query := "UPDATE Sessions SET Roles = :Roles WHERE UserId = :UserId"

	_, err := me.GetMaster().Exec(query, map[string]interface{}{"Roles": roles, "UserId": userID})
	if err != nil {
		return "", errors.Wrapf(err, "failed to update Session with userId=%s and roles=%s", userID, roles)
	}
	return userID, nil
}

func (me SQLSessionStore) UpdateDeviceID(id string, deviceID string, expiresAt int64) (string, error) {
	query := "UPDATE Sessions SET DeviceId = :DeviceId, ExpiresAt = :ExpiresAt, ExpiredNotify = false WHERE Id = :Id"

	_, err := me.GetMaster().Exec(query, map[string]interface{}{"DeviceId": deviceID, "Id": id, "ExpiresAt": expiresAt})
	if err != nil {
		return "", errors.Wrapf(err, "failed to update Session with id=%s", id)
	}
	return deviceID, nil
}

func (me SQLSessionStore) UpdateProps(session *model.Session) error {
	oldSession, err := me.Get(context.Background(), session.ID)
	if err != nil {
		return err
	}
	oldSession.Props = session.Props

	count, err := me.GetMaster().Update(oldSession)
	if err != nil {
		return errors.Wrap(err, "failed to update Session")
	}
	if count != 1 {
		return fmt.Errorf("updated Sessions were %d, expected 1", count)
	}
	return nil
}

func (me SQLSessionStore) AnalyticsSessionCount() (int64, error) {
	query :=
		`SELECT
			COUNT(*)
		FROM
			Sessions
		WHERE ExpiresAt > :Time`
	count, err := me.GetReplica().SelectInt(query, map[string]interface{}{"Time": model.GetMillis()})
	if err != nil {
		return int64(0), errors.Wrap(err, "failed to count Sessions")
	}
	return count, nil
}

func (me SQLSessionStore) Cleanup(expiryTime int64, batchSize int64) {
	mlog.Debug("Cleaning up session store.")

	var query string
	if me.DriverName() == model.DatabaseDriverPostgres {
		query = "DELETE FROM Sessions WHERE Id = any (array (SELECT Id FROM Sessions WHERE ExpiresAt != 0 AND :ExpiresAt > ExpiresAt LIMIT :Limit))"
	} else {
		query = "DELETE FROM Sessions WHERE ExpiresAt != 0 AND :ExpiresAt > ExpiresAt LIMIT :Limit"
	}

	var rowsAffected int64 = 1

	for rowsAffected > 0 {
		sqlResult, err := me.GetMaster().Exec(query, map[string]interface{}{"ExpiresAt": expiryTime, "Limit": batchSize})
		if err != nil {
			mlog.Error("Unable to cleanup session store.", mlog.Err(err))
			return
		}
		var rowErr error
		rowsAffected, rowErr = sqlResult.RowsAffected()
		if rowErr != nil {
			mlog.Error("Unable to cleanup session store.", mlog.Err(err))
			return
		}

		time.Sleep(SessionsCleanupDelayMilliseconds * time.Millisecond)
	}
}
