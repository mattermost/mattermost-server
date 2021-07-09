// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SQLWebhookStore struct {
	*SQLStore
	metrics einterfaces.MetricsInterface
}

func (s SQLWebhookStore) ClearCaches() {
}

func newSQLWebhookStore(sqlStore *SQLStore, metrics einterfaces.MetricsInterface) store.WebhookStore {
	s := &SQLWebhookStore{
		SQLStore: sqlStore,
		metrics:  metrics,
	}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.IncomingWebhook{}, "IncomingWebhooks").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("ChannelId").SetMaxSize(26)
		table.ColMap("TeamId").SetMaxSize(26)
		table.ColMap("DisplayName").SetMaxSize(64)
		table.ColMap("Description").SetMaxSize(500)
		table.ColMap("Username").SetMaxSize(255)
		table.ColMap("IconURL").SetMaxSize(1024)

		tableo := db.AddTableWithName(model.OutgoingWebhook{}, "OutgoingWebhooks").SetKeys(false, "Id")
		tableo.ColMap("Id").SetMaxSize(26)
		tableo.ColMap("Token").SetMaxSize(26)
		tableo.ColMap("CreatorId").SetMaxSize(26)
		tableo.ColMap("ChannelId").SetMaxSize(26)
		tableo.ColMap("TeamId").SetMaxSize(26)
		tableo.ColMap("TriggerWords").SetMaxSize(1024)
		tableo.ColMap("CallbackURLs").SetMaxSize(1024)
		tableo.ColMap("DisplayName").SetMaxSize(64)
		tableo.ColMap("Description").SetMaxSize(500)
		tableo.ColMap("ContentType").SetMaxSize(128)
		tableo.ColMap("TriggerWhen").SetMaxSize(1)
		tableo.ColMap("Username").SetMaxSize(64)
		tableo.ColMap("IconURL").SetMaxSize(1024)
	}

	return s
}

func (s SQLWebhookStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_incoming_webhook_user_id", "IncomingWebhooks", "UserId")
	s.CreateIndexIfNotExists("idx_incoming_webhook_team_id", "IncomingWebhooks", "TeamId")
	s.CreateIndexIfNotExists("idx_outgoing_webhook_team_id", "OutgoingWebhooks", "TeamId")

	s.CreateIndexIfNotExists("idx_incoming_webhook_update_at", "IncomingWebhooks", "UpdateAt")
	s.CreateIndexIfNotExists("idx_incoming_webhook_create_at", "IncomingWebhooks", "CreateAt")
	s.CreateIndexIfNotExists("idx_incoming_webhook_delete_at", "IncomingWebhooks", "DeleteAt")

	s.CreateIndexIfNotExists("idx_outgoing_webhook_update_at", "OutgoingWebhooks", "UpdateAt")
	s.CreateIndexIfNotExists("idx_outgoing_webhook_create_at", "OutgoingWebhooks", "CreateAt")
	s.CreateIndexIfNotExists("idx_outgoing_webhook_delete_at", "OutgoingWebhooks", "DeleteAt")
}

func (s SQLWebhookStore) InvalidateWebhookCache(webhookID string) {
}

func (s SQLWebhookStore) SaveIncoming(webhook *model.IncomingWebhook) (*model.IncomingWebhook, error) {

	if webhook.ID != "" {
		return nil, store.NewErrInvalidInput("IncomingWebhook", "id", webhook.ID)
	}

	webhook.PreSave()
	if err := webhook.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(webhook); err != nil {
		return nil, errors.Wrapf(err, "failed to save IncomingWebhook with id=%s", webhook.ID)
	}

	return webhook, nil

}

func (s SQLWebhookStore) UpdateIncoming(hook *model.IncomingWebhook) (*model.IncomingWebhook, error) {
	hook.UpdateAt = model.GetMillis()

	if _, err := s.GetMaster().Update(hook); err != nil {
		return nil, errors.Wrapf(err, "failed to update IncomingWebhook with id=%s", hook.ID)
	}
	return hook, nil
}

func (s SQLWebhookStore) GetIncoming(id string, allowFromCache bool) (*model.IncomingWebhook, error) {
	var webhook model.IncomingWebhook
	if err := s.GetReplica().SelectOne(&webhook, "SELECT * FROM IncomingWebhooks WHERE Id = :Id AND DeleteAt = 0", map[string]interface{}{"Id": id}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("IncomingWebhook", id)
		}
		return nil, errors.Wrapf(err, "failed to get IncomingWebhook with id=%s", id)
	}

	return &webhook, nil
}

func (s SQLWebhookStore) DeleteIncoming(webhookID string, time int64) error {
	_, err := s.GetMaster().Exec("Update IncomingWebhooks SET DeleteAt = :DeleteAt, UpdateAt = :UpdateAt WHERE Id = :Id", map[string]interface{}{"DeleteAt": time, "UpdateAt": time, "Id": webhookID})
	if err != nil {
		return errors.Wrapf(err, "failed to update IncomingWebhook with id=%s", webhookID)
	}

	return nil
}

func (s SQLWebhookStore) PermanentDeleteIncomingByUser(userID string) error {
	_, err := s.GetMaster().Exec("DELETE FROM IncomingWebhooks WHERE UserId = :UserId", map[string]interface{}{"UserId": userID})
	if err != nil {
		return errors.Wrapf(err, "failed to delete IncomingWebhook with userId=%s", userID)
	}

	return nil
}

func (s SQLWebhookStore) PermanentDeleteIncomingByChannel(channelID string) error {
	_, err := s.GetMaster().Exec("DELETE FROM IncomingWebhooks WHERE ChannelId = :ChannelId", map[string]interface{}{"ChannelId": channelID})
	if err != nil {
		return errors.Wrapf(err, "failed to delete IncomingWebhook with channelId=%s", channelID)
	}

	return nil
}

func (s SQLWebhookStore) GetIncomingList(offset, limit int) ([]*model.IncomingWebhook, error) {
	return s.GetIncomingListByUser("", offset, limit)
}

func (s SQLWebhookStore) GetIncomingListByUser(userID string, offset, limit int) ([]*model.IncomingWebhook, error) {
	var webhooks []*model.IncomingWebhook

	query := s.getQueryBuilder().
		Select("*").
		From("IncomingWebhooks").
		Where(sq.Eq{"DeleteAt": int(0)}).Limit(uint64(limit)).Offset(uint64(offset))

	if userID != "" {
		query = query.Where(sq.Eq{"UserId": userID})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "incoming_webhook_tosql")
	}

	if _, err := s.GetReplica().Select(&webhooks, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find IncomingWebhooks")
	}

	return webhooks, nil

}

func (s SQLWebhookStore) GetIncomingByTeamByUser(teamID string, userID string, offset, limit int) ([]*model.IncomingWebhook, error) {
	var webhooks []*model.IncomingWebhook

	query := s.getQueryBuilder().
		Select("*").
		From("IncomingWebhooks").
		Where(sq.And{
			sq.Eq{"TeamId": teamID},
			sq.Eq{"DeleteAt": int(0)},
		}).Limit(uint64(limit)).Offset(uint64(offset))

	if userID != "" {
		query = query.Where(sq.Eq{"UserId": userID})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "incoming_webhook_tosql")
	}

	if _, err := s.GetReplica().Select(&webhooks, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find IncomingWebhoook with teamId=%s", teamID)
	}

	return webhooks, nil
}

func (s SQLWebhookStore) GetIncomingByTeam(teamID string, offset, limit int) ([]*model.IncomingWebhook, error) {
	return s.GetIncomingByTeamByUser(teamID, "", offset, limit)
}

func (s SQLWebhookStore) GetIncomingByChannel(channelID string) ([]*model.IncomingWebhook, error) {
	var webhooks []*model.IncomingWebhook

	if _, err := s.GetReplica().Select(&webhooks, "SELECT * FROM IncomingWebhooks WHERE ChannelId = :ChannelId AND DeleteAt = 0", map[string]interface{}{"ChannelId": channelID}); err != nil {
		return nil, errors.Wrapf(err, "failed to find IncomingWebhooks with channelId=%s", channelID)
	}

	return webhooks, nil
}

func (s SQLWebhookStore) SaveOutgoing(webhook *model.OutgoingWebhook) (*model.OutgoingWebhook, error) {
	if webhook.ID != "" {
		return nil, store.NewErrInvalidInput("OutgoingWebhook", "id", webhook.ID)
	}

	webhook.PreSave()
	if err := webhook.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(webhook); err != nil {
		return nil, errors.Wrapf(err, "failed to save OutgoingWebhook with id=%s", webhook.ID)
	}

	return webhook, nil
}

func (s SQLWebhookStore) GetOutgoing(id string) (*model.OutgoingWebhook, error) {

	var webhook model.OutgoingWebhook

	if err := s.GetReplica().SelectOne(&webhook, "SELECT * FROM OutgoingWebhooks WHERE Id = :Id AND DeleteAt = 0", map[string]interface{}{"Id": id}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("OutgoingWebhook", id)
		}

		return nil, errors.Wrapf(err, "failed to get OutgoingWebhook with id=%s", id)
	}

	return &webhook, nil
}

func (s SQLWebhookStore) GetOutgoingListByUser(userID string, offset, limit int) ([]*model.OutgoingWebhook, error) {
	var webhooks []*model.OutgoingWebhook

	query := s.getQueryBuilder().
		Select("*").
		From("OutgoingWebhooks").
		Where(sq.And{
			sq.Eq{"DeleteAt": int(0)},
		}).Limit(uint64(limit)).Offset(uint64(offset))

	if userID != "" {
		query = query.Where(sq.Eq{"CreatorId": userID})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "outgoing_webhook_tosql")
	}

	if _, err := s.GetReplica().Select(&webhooks, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find OutgoingWebhooks")
	}

	return webhooks, nil
}

func (s SQLWebhookStore) GetOutgoingList(offset, limit int) ([]*model.OutgoingWebhook, error) {
	return s.GetOutgoingListByUser("", offset, limit)

}

func (s SQLWebhookStore) GetOutgoingByChannelByUser(channelID string, userID string, offset, limit int) ([]*model.OutgoingWebhook, error) {
	var webhooks []*model.OutgoingWebhook

	query := s.getQueryBuilder().
		Select("*").
		From("OutgoingWebhooks").
		Where(sq.And{
			sq.Eq{"ChannelId": channelID},
			sq.Eq{"DeleteAt": int(0)},
		})

	if userID != "" {
		query = query.Where(sq.Eq{"CreatorId": userID})
	}
	if limit >= 0 && offset >= 0 {
		query = query.Limit(uint64(limit)).Offset(uint64(offset))
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "outgoing_webhook_tosql")
	}

	if _, err := s.GetReplica().Select(&webhooks, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find OutgoingWebhooks")
	}

	return webhooks, nil
}

func (s SQLWebhookStore) GetOutgoingByChannel(channelID string, offset, limit int) ([]*model.OutgoingWebhook, error) {
	return s.GetOutgoingByChannelByUser(channelID, "", offset, limit)
}

func (s SQLWebhookStore) GetOutgoingByTeamByUser(teamID string, userID string, offset, limit int) ([]*model.OutgoingWebhook, error) {
	var webhooks []*model.OutgoingWebhook

	query := s.getQueryBuilder().
		Select("*").
		From("OutgoingWebhooks").
		Where(sq.And{
			sq.Eq{"TeamId": teamID},
			sq.Eq{"DeleteAt": int(0)},
		})

	if userID != "" {
		query = query.Where(sq.Eq{"CreatorId": userID})
	}
	if limit >= 0 && offset >= 0 {
		query = query.Limit(uint64(limit)).Offset(uint64(offset))
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "outgoing_webhook_tosql")
	}

	if _, err := s.GetReplica().Select(&webhooks, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find OutgoingWebhooks")
	}

	return webhooks, nil
}

func (s SQLWebhookStore) GetOutgoingByTeam(teamID string, offset, limit int) ([]*model.OutgoingWebhook, error) {
	return s.GetOutgoingByTeamByUser(teamID, "", offset, limit)
}

func (s SQLWebhookStore) DeleteOutgoing(webhookID string, time int64) error {
	_, err := s.GetMaster().Exec("Update OutgoingWebhooks SET DeleteAt = :DeleteAt, UpdateAt = :UpdateAt WHERE Id = :Id", map[string]interface{}{"DeleteAt": time, "UpdateAt": time, "Id": webhookID})
	if err != nil {
		return errors.Wrapf(err, "failed to update OutgoingWebhook with id=%s", webhookID)
	}

	return nil
}

func (s SQLWebhookStore) PermanentDeleteOutgoingByUser(userID string) error {
	_, err := s.GetMaster().Exec("DELETE FROM OutgoingWebhooks WHERE CreatorId = :UserId", map[string]interface{}{"UserId": userID})
	if err != nil {
		return errors.Wrapf(err, "failed to delete OutgoingWebhook with creatorId=%s", userID)
	}

	return nil
}

func (s SQLWebhookStore) PermanentDeleteOutgoingByChannel(channelID string) error {
	_, err := s.GetMaster().Exec("DELETE FROM OutgoingWebhooks WHERE ChannelId = :ChannelId", map[string]interface{}{"ChannelId": channelID})
	if err != nil {
		return errors.Wrapf(err, "failed to delete OutgoingWebhook with channelId=%s", channelID)
	}

	s.ClearCaches()

	return nil
}

func (s SQLWebhookStore) UpdateOutgoing(hook *model.OutgoingWebhook) (*model.OutgoingWebhook, error) {
	hook.UpdateAt = model.GetMillis()

	if _, err := s.GetMaster().Update(hook); err != nil {
		return nil, errors.Wrapf(err, "failed to update OutgoingWebhook with id=%s", hook.ID)
	}

	return hook, nil
}

func (s SQLWebhookStore) AnalyticsIncomingCount(teamID string) (int64, error) {
	query :=
		`SELECT
			COUNT(*)
		FROM
			IncomingWebhooks
		WHERE
			DeleteAt = 0`

	if teamID != "" {
		query += " AND TeamId = :TeamId"
	}

	v, err := s.GetReplica().SelectInt(query, map[string]interface{}{"TeamId": teamID})
	if err != nil {
		return 0, errors.Wrap(err, "failed to count IncomingWebhooks")
	}

	return v, nil
}

func (s SQLWebhookStore) AnalyticsOutgoingCount(teamID string) (int64, error) {
	query :=
		`SELECT
			COUNT(*)
		FROM
			OutgoingWebhooks
		WHERE
			DeleteAt = 0`

	if teamID != "" {
		query += " AND TeamId = :TeamId"
	}

	v, err := s.GetReplica().SelectInt(query, map[string]interface{}{"TeamId": teamID})
	if err != nil {
		return 0, errors.Wrap(err, "failed to count OutgoingWebhooks")
	}

	return v, nil
}
