// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SqlEmojiStore struct {
	*SqlStore
	metrics einterfaces.MetricsInterface
}

func newSqlEmojiStore(sqlStore *SqlStore, metrics einterfaces.MetricsInterface) store.EmojiStore {
	s := &SqlEmojiStore{
		SqlStore: sqlStore,
		metrics:  metrics,
	}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Emoji{}, "Emoji").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("CreatorId").SetMaxSize(26)
		table.ColMap("Name").SetMaxSize(64)

		table.SetUniqueTogether("Name", "DeleteAt")
	}

	return s
}

func (es SqlEmojiStore) createIndexesIfNotExists() {
	es.CreateIndexIfNotExists("idx_emoji_update_at", "Emoji", "UpdateAt")
	es.CreateIndexIfNotExists("idx_emoji_create_at", "Emoji", "CreateAt")
	es.CreateIndexIfNotExists("idx_emoji_delete_at", "Emoji", "DeleteAt")
}

func (es SqlEmojiStore) Save(emoji *model.Emoji) (*model.Emoji, error) {
	emoji.PreSave()
	if err := emoji.IsValid(); err != nil {
		return nil, err
	}

	if err := es.GetMaster().Insert(emoji); err != nil {
		return nil, errors.Wrap(err, "error saving emoji")
	}

	return emoji, nil
}

func (es SqlEmojiStore) Get(ctx context.Context, id string, allowFromCache bool) (*model.Emoji, error) {
	return es.getBy(ctx, "Id", id)
}

func (es SqlEmojiStore) GetByName(ctx context.Context, name string, allowFromCache bool) (*model.Emoji, error) {
	return es.getBy(ctx, "Name", name)
}

func (es SqlEmojiStore) GetMultipleByName(names []string) ([]*model.Emoji, error) {
	keys, params := MapStringsToQueryParams(names, "Emoji")

	var emojis []*model.Emoji

	if _, err := es.GetReplica().Select(&emojis,
		`SELECT
			*
		FROM
			Emoji
		WHERE
			Name IN `+keys+`
			AND DeleteAt = 0`, params); err != nil {
		return nil, errors.Wrapf(err, "error getting emoji by names %v", names)
	}
	return emojis, nil
}

func (es SqlEmojiStore) GetList(offset, limit int, sort string) ([]*model.Emoji, error) {
	var emoji []*model.Emoji

	query := "SELECT * FROM Emoji WHERE DeleteAt = 0"

	if sort == model.EmojiSortByName {
		query += " ORDER BY Name"
	}

	query += " LIMIT :Limit OFFSET :Offset"

	if _, err := es.GetReplica().Select(&emoji, query, map[string]interface{}{"Offset": offset, "Limit": limit}); err != nil {
		return nil, errors.Wrap(err, "could not get list of emojis")
	}
	return emoji, nil
}

func (es SqlEmojiStore) Delete(emoji *model.Emoji, time int64) error {
	if sqlResult, err := es.GetMaster().Exec(
		`UPDATE
			Emoji
		SET
			DeleteAt = :DeleteAt,
			UpdateAt = :UpdateAt
		WHERE
			Id = :Id
			AND DeleteAt = 0`, map[string]interface{}{"DeleteAt": time, "UpdateAt": time, "Id": emoji.ID}); err != nil {
		return errors.Wrap(err, "could not delete emoji")
	} else if rows, _ := sqlResult.RowsAffected(); rows == 0 {
		return store.NewErrNotFound("Emoji", emoji.ID)
	}

	return nil
}

func (es SqlEmojiStore) Search(name string, prefixOnly bool, limit int) ([]*model.Emoji, error) {
	var emojis []*model.Emoji

	name = sanitizeSearchTerm(name, "\\")

	term := ""
	if !prefixOnly {
		term = "%"
	}

	term += name + "%"

	if _, err := es.GetReplica().Select(&emojis,
		`SELECT
			*
		FROM
			Emoji
		WHERE
			Name LIKE :Name
			AND DeleteAt = 0
			ORDER BY Name
			LIMIT :Limit`, map[string]interface{}{"Name": term, "Limit": limit}); err != nil {
		return nil, errors.Wrapf(err, "could not search emojis by name %s", name)
	}
	return emojis, nil
}

// getBy returns one active (not deleted) emoji, found by any one column (what/key).
func (es SqlEmojiStore) getBy(ctx context.Context, what, key string) (*model.Emoji, error) {
	var emoji *model.Emoji

	err := es.DBFromContext(ctx).SelectOne(&emoji,
		`SELECT
			*
		FROM
			Emoji
		WHERE
			`+what+` = :Key
			AND DeleteAt = 0`, map[string]string{"Key": key})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Emoji", fmt.Sprintf("%s=%s", what, key))
		}

		return nil, errors.Wrapf(err, "could not get emoji by %s with value %s", what, key)
	}

	return emoji, nil
}
