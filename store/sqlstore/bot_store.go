// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

// bot is a subset of the model.Bot type, omitting the model.User fields.
type bot struct {
	UserID         string `json:"user_id"`
	Description    string `json:"description"`
	OwnerID        string `json:"owner_id"`
	LastIconUpdate int64  `json:"last_icon_update"`
	CreateAt       int64  `json:"create_at"`
	UpdateAt       int64  `json:"update_at"`
	DeleteAt       int64  `json:"delete_at"`
}

func botFromModel(b *model.Bot) *bot {
	return &bot{
		UserID:         b.UserID,
		Description:    b.Description,
		OwnerID:        b.OwnerID,
		LastIconUpdate: b.LastIconUpdate,
		CreateAt:       b.CreateAt,
		UpdateAt:       b.UpdateAt,
		DeleteAt:       b.DeleteAt,
	}
}

// SqlBotStore is a store for managing bots in the database.
// Bots are otherwise normal users with extra metadata record in the Bots table. The primary key
// for a bot matches the primary key value for corresponding User record.
type SQLBotStore struct {
	*SQLStore
	metrics einterfaces.MetricsInterface
}

// newSqlBotStore creates an instance of SqlBotStore, registering the table schema in question.
func newSQLBotStore(sqlStore *SQLStore, metrics einterfaces.MetricsInterface) store.BotStore {
	us := &SQLBotStore{
		SQLStore: sqlStore,
		metrics:  metrics,
	}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(bot{}, "Bots").SetKeys(false, "UserId")
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("Description").SetMaxSize(1024)
		table.ColMap("OwnerId").SetMaxSize(model.BotCreatorIDMaxRunes)
	}

	return us
}

func (us SQLBotStore) createIndexesIfNotExists() {
}

// Get fetches the given bot in the database.
func (us SQLBotStore) Get(botUserID string, includeDeleted bool) (*model.Bot, error) {
	var excludeDeletedSQL = "AND b.DeleteAt = 0"
	if includeDeleted {
		excludeDeletedSQL = ""
	}

	query := `
		SELECT
			b.UserId,
			u.Username,
			u.FirstName AS DisplayName,
			b.Description,
			b.OwnerId,
			COALESCE(b.LastIconUpdate, 0) AS LastIconUpdate,
			b.CreateAt,
			b.UpdateAt,
			b.DeleteAt
		FROM
			Bots b
		JOIN
			Users u ON (u.Id = b.UserId)
		WHERE
			b.UserId = :user_id
			` + excludeDeletedSQL + `
	`

	var bot *model.Bot
	if err := us.GetReplica().SelectOne(&bot, query, map[string]interface{}{"user_id": botUserID}); err == sql.ErrNoRows {
		return nil, store.NewErrNotFound("Bot", botUserID)
	} else if err != nil {
		return nil, errors.Wrapf(err, "selectone: user_id=%s", botUserID)
	}

	return bot, nil
}

// GetAll fetches from all bots in the database.
func (us SQLBotStore) GetAll(options *model.BotGetOptions) ([]*model.Bot, error) {
	params := map[string]interface{}{
		"offset": options.Page * options.PerPage,
		"limit":  options.PerPage,
	}

	var conditions []string
	var conditionsSQL string
	var additionalJoin string

	if !options.IncludeDeleted {
		conditions = append(conditions, "b.DeleteAt = 0")
	}
	if options.OwnerID != "" {
		conditions = append(conditions, "b.OwnerId = :creator_id")
		params["creator_id"] = options.OwnerID
	}
	if options.OnlyOrphaned {
		additionalJoin = "JOIN Users o ON (o.Id = b.OwnerId)"
		conditions = append(conditions, "o.DeleteAt != 0")
	}

	if len(conditions) > 0 {
		conditionsSQL = "WHERE " + strings.Join(conditions, " AND ")
	}

	sql := `
			SELECT
			    b.UserId,
			    u.Username,
			    u.FirstName AS DisplayName,
			    b.Description,
			    b.OwnerId,
			    COALESCE(b.LastIconUpdate, 0) AS LastIconUpdate,
			    b.CreateAt,
			    b.UpdateAt,
			    b.DeleteAt
			FROM
			    Bots b
			JOIN
			    Users u ON (u.Id = b.UserId)
			` + additionalJoin + `
			` + conditionsSQL + `
			ORDER BY
			    b.CreateAt ASC,
			    u.Username ASC
			LIMIT
			    :limit
			OFFSET
			    :offset
		`

	var bots []*model.Bot
	if _, err := us.GetReplica().Select(&bots, sql, params); err != nil {
		return nil, errors.Wrap(err, "select")
	}

	return bots, nil
}

// Save persists a new bot to the database.
// It assumes the corresponding user was saved via the user store.
func (us SQLBotStore) Save(bot *model.Bot) (*model.Bot, error) {
	bot = bot.Clone()
	bot.PreSave()

	if err := bot.IsValid(); err != nil { // TODO: change to return error in v6.
		return nil, err
	}

	if err := us.GetMaster().Insert(botFromModel(bot)); err != nil {
		return nil, errors.Wrapf(err, "insert: user_id=%s", bot.UserID)
	}

	return bot, nil
}

// Update persists an updated bot to the database.
// It assumes the corresponding user was updated via the user store.
func (us SQLBotStore) Update(bot *model.Bot) (*model.Bot, error) {
	bot = bot.Clone()

	bot.PreUpdate()
	if err := bot.IsValid(); err != nil { // TODO: needs to return error in v6
		return nil, err
	}

	oldBot, err := us.Get(bot.UserID, true)
	if err != nil {
		return nil, err
	}

	oldBot.Description = bot.Description
	oldBot.OwnerID = bot.OwnerID
	oldBot.LastIconUpdate = bot.LastIconUpdate
	oldBot.UpdateAt = bot.UpdateAt
	oldBot.DeleteAt = bot.DeleteAt
	bot = oldBot

	if count, err := us.GetMaster().Update(botFromModel(bot)); err != nil {
		return nil, errors.Wrapf(err, "update: user_id=%s", bot.UserID)
	} else if count > 1 {
		return nil, fmt.Errorf("unexpected count while updating bot: count=%d, userId=%s", count, bot.UserID)
	}

	return bot, nil
}

// PermanentDelete removes the bot from the database altogether.
// If the corresponding user is to be deleted, it must be done via the user store.
func (us SQLBotStore) PermanentDelete(botUserID string) error {
	query := "DELETE FROM Bots WHERE UserId = :user_id"
	if _, err := us.GetMaster().Exec(query, map[string]interface{}{"user_id": botUserID}); err != nil {
		return store.NewErrInvalidInput("Bot", "UserId", botUserID)
	}
	return nil
}
