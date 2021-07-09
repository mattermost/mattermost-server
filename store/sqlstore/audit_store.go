// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SqlAuditStore struct {
	*SqlStore
}

func newSqlAuditStore(sqlStore *SqlStore) store.AuditStore {
	s := &SqlAuditStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Audit{}, "Audits").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("Action").SetMaxSize(512)
		table.ColMap("ExtraInfo").SetMaxSize(1024)
		table.ColMap("IpAddress").SetMaxSize(64)
		table.ColMap("SessionId").SetMaxSize(26)
	}

	return s
}

func (s SqlAuditStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_audits_user_id", "Audits", "UserId")
}

func (s SqlAuditStore) Save(audit *model.Audit) error {
	audit.ID = model.NewID()
	audit.CreateAt = model.GetMillis()

	if err := s.GetMaster().Insert(audit); err != nil {
		return errors.Wrapf(err, "failed to save Audit with userId=%s and action=%s", audit.UserID, audit.Action)
	}
	return nil
}

func (s SqlAuditStore) Get(userID string, offset int, limit int) (model.Audits, error) {
	if limit > 1000 {
		return nil, store.NewErrOutOfBounds(limit)
	}

	query := s.getQueryBuilder().
		Select("*").
		From("Audits").
		OrderBy("CreateAt DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	if userID != "" {
		query = query.Where(sq.Eq{"UserId": userID})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "audits_tosql")
	}

	var audits model.Audits
	if _, err := s.GetReplica().Select(&audits, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to get Audit list for userId=%s", userID)
	}
	return audits, nil
}

func (s SqlAuditStore) PermanentDeleteByUser(userID string) error {
	if _, err := s.GetMaster().Exec("DELETE FROM Audits WHERE UserId = :userId",
		map[string]interface{}{"userId": userID}); err != nil {
		return errors.Wrapf(err, "failed to delete Audit with userId=%s", userID)
	}
	return nil
}
