// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SQLCommandWebhookStore struct {
	*SQLStore
}

func newSQLCommandWebhookStore(sqlStore *SQLStore) store.CommandWebhookStore {
	s := &SQLCommandWebhookStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		tablec := db.AddTableWithName(model.CommandWebhook{}, "CommandWebhooks").SetKeys(false, "Id")
		tablec.ColMap("Id").SetMaxSize(26)
		tablec.ColMap("CommandId").SetMaxSize(26)
		tablec.ColMap("UserId").SetMaxSize(26)
		tablec.ColMap("ChannelId").SetMaxSize(26)
		tablec.ColMap("RootId").SetMaxSize(26)
		tablec.ColMap("ParentId").SetMaxSize(26)
	}

	return s
}

func (s SQLCommandWebhookStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_command_webhook_create_at", "CommandWebhooks", "CreateAt")
}

func (s SQLCommandWebhookStore) Save(webhook *model.CommandWebhook) (*model.CommandWebhook, error) {
	if webhook.ID != "" {
		return nil, store.NewErrInvalidInput("CommandWebhook", "id", webhook.ID)
	}

	webhook.PreSave()
	if err := webhook.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(webhook); err != nil {
		return nil, errors.Wrapf(err, "save: id=%s", webhook.ID)
	}

	return webhook, nil
}

func (s SQLCommandWebhookStore) Get(id string) (*model.CommandWebhook, error) {
	var webhook model.CommandWebhook

	exptime := model.GetMillis() - model.CommandWebhookLifetime

	query := s.getQueryBuilder().
		Select("*").
		From("CommandWebhooks").
		Where(sq.Eq{"Id": id}).
		Where(sq.Gt{"CreateAt": exptime})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_tosql")
	}

	if err := s.GetReplica().SelectOne(&webhook, queryString, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("CommandWebhook", id)
		}
		return nil, errors.Wrapf(err, "get: id=%s", id)
	}

	return &webhook, nil
}

func (s SQLCommandWebhookStore) TryUse(id string, limit int) error {
	query := s.getQueryBuilder().
		Update("CommandWebhooks").
		Set("UseCount", sq.Expr("UseCount + 1")).
		Where(sq.Eq{"Id": id}).
		Where(sq.Lt{"UseCount": limit})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "tryuse_tosql")
	}

	if sqlResult, err := s.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrapf(err, "tryuse: id=%s limit=%d", id, limit)
	} else if rows, _ := sqlResult.RowsAffected(); rows == 0 {
		return store.NewErrInvalidInput("CommandWebhook", "id", id)
	}

	return nil
}

func (s SQLCommandWebhookStore) Cleanup() {
	mlog.Debug("Cleaning up command webhook store.")
	exptime := model.GetMillis() - model.CommandWebhookLifetime

	query := s.getQueryBuilder().
		Delete("CommandWebhooks").
		Where(sq.Lt{"CreateAt": exptime})

	queryString, args, err := query.ToSql()
	if err != nil {
		mlog.Error("Failed to build query when trying to perform a cleanup in command webhook store.")
		return
	}

	if _, err := s.GetMaster().Exec(queryString, args...); err != nil {
		mlog.Error("Unable to cleanup command webhook store.")
	}
}
