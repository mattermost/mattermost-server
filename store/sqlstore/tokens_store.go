// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SQLTokenStore struct {
	*SQLStore
}

func newSQLTokenStore(sqlStore *SQLStore) store.TokenStore {
	s := &SQLTokenStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Token{}, "Tokens").SetKeys(false, "Token")
		table.ColMap("Token").SetMaxSize(64)
		table.ColMap("Type").SetMaxSize(64)
		table.ColMap("Extra").SetMaxSize(2048)
	}

	return s
}

func (s SQLTokenStore) createIndexesIfNotExists() {
}

func (s SQLTokenStore) Save(token *model.Token) error {
	if err := token.IsValid(); err != nil {
		return err
	}

	if err := s.GetMaster().Insert(token); err != nil {
		return errors.Wrap(err, "failed to save Token")
	}
	return nil
}

func (s SQLTokenStore) Delete(token string) error {
	if _, err := s.GetMaster().Exec("DELETE FROM Tokens WHERE Token = :Token", map[string]interface{}{"Token": token}); err != nil {
		return errors.Wrapf(err, "failed to delete Token with value %s", token)
	}
	return nil
}

func (s SQLTokenStore) GetByToken(tokenString string) (*model.Token, error) {
	token := &model.Token{}

	if err := s.GetReplica().SelectOne(token, "SELECT * FROM Tokens WHERE Token = :Token", map[string]interface{}{"Token": tokenString}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Token", fmt.Sprintf("Token=%s", tokenString))
		}

		return nil, errors.Wrapf(err, "failed to get Token with value %s", tokenString)
	}

	return token, nil
}

func (s SQLTokenStore) Cleanup() {
	mlog.Debug("Cleaning up token store.")
	deltime := model.GetMillis() - model.MaxTokenExipryTime
	if _, err := s.GetMaster().Exec("DELETE FROM Tokens WHERE CreateAt < :DelTime", map[string]interface{}{"DelTime": deltime}); err != nil {
		mlog.Error("Unable to cleanup token store.")
	}
}

func (s SQLTokenStore) RemoveAllTokensByType(tokenType string) error {
	if _, err := s.GetMaster().Exec("DELETE FROM Tokens WHERE Type = :TokenType", map[string]interface{}{"TokenType": tokenType}); err != nil {
		return errors.Wrapf(err, "failed to remove all Tokens with Type=%s", tokenType)
	}
	return nil
}
