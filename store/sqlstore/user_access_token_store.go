// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/gorp"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SqlUserAccessTokenStore struct {
	*SqlStore
}

func newSqlUserAccessTokenStore(sqlStore *SqlStore) store.UserAccessTokenStore {
	s := &SqlUserAccessTokenStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.UserAccessToken{}, "UserAccessTokens").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("Token").SetMaxSize(26).SetUnique(true)
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("Description").SetMaxSize(512)
	}

	return s
}

func (s SqlUserAccessTokenStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_user_access_tokens_user_id", "UserAccessTokens", "UserId")
}

func (s SqlUserAccessTokenStore) Save(token *model.UserAccessToken) (*model.UserAccessToken, error) {
	token.PreSave()

	if err := token.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(token); err != nil {
		return nil, errors.Wrap(err, "failed to save UserAccessToken")
	}
	return token, nil
}

func (s SqlUserAccessTokenStore) Delete(tokenID string) error {
	transaction, err := s.GetMaster().Begin()
	if err != nil {
		return errors.Wrap(err, "begin_transaction")
	}

	defer finalizeTransaction(transaction)

	if err := s.deleteSessionsAndTokensByID(transaction, tokenID); err == nil {
		if err := transaction.Commit(); err != nil {
			// don't need to rollback here since the transaction is already closed
			return errors.Wrap(err, "commit_transaction")
		}
	}

	return nil

}

func (s SqlUserAccessTokenStore) deleteSessionsAndTokensByID(transaction *gorp.Transaction, tokenID string) error {

	query := ""
	if s.DriverName() == model.DatabaseDriverPostgres {
		query = "DELETE FROM Sessions s USING UserAccessTokens o WHERE o.Token = s.Token AND o.Id = :Id"
	} else if s.DriverName() == model.DatabaseDriverMysql {
		query = "DELETE s.* FROM Sessions s INNER JOIN UserAccessTokens o ON o.Token = s.Token WHERE o.Id = :Id"
	}

	if _, err := transaction.Exec(query, map[string]interface{}{"Id": tokenID}); err != nil {
		return errors.Wrapf(err, "failed to delete Sessions with UserAccessToken id=%s", tokenID)
	}

	return s.deleteTokensByID(transaction, tokenID)
}

func (s SqlUserAccessTokenStore) deleteTokensByID(transaction *gorp.Transaction, tokenID string) error {

	if _, err := transaction.Exec("DELETE FROM UserAccessTokens WHERE Id = :Id", map[string]interface{}{"Id": tokenID}); err != nil {
		return errors.Wrapf(err, "failed to delete UserAccessToken id=%s", tokenID)
	}

	return nil
}

func (s SqlUserAccessTokenStore) DeleteAllForUser(userID string) error {
	transaction, err := s.GetMaster().Begin()
	if err != nil {
		return errors.Wrap(err, "begin_transaction")
	}
	defer finalizeTransaction(transaction)
	if err := s.deleteSessionsandTokensByUser(transaction, userID); err != nil {
		return err
	}

	if err := transaction.Commit(); err != nil {
		// don't need to rollback here since the transaction is already closed
		return errors.Wrap(err, "commit_transaction")
	}
	return nil
}

func (s SqlUserAccessTokenStore) deleteSessionsandTokensByUser(transaction *gorp.Transaction, userID string) error {
	query := ""
	if s.DriverName() == model.DatabaseDriverPostgres {
		query = "DELETE FROM Sessions s USING UserAccessTokens o WHERE o.Token = s.Token AND o.UserId = :UserId"
	} else if s.DriverName() == model.DatabaseDriverMysql {
		query = "DELETE s.* FROM Sessions s INNER JOIN UserAccessTokens o ON o.Token = s.Token WHERE o.UserId = :UserId"
	}

	if _, err := transaction.Exec(query, map[string]interface{}{"UserId": userID}); err != nil {
		return errors.Wrapf(err, "failed to delete Sessions with UserAccessToken userId=%s", userID)
	}

	return s.deleteTokensByUser(transaction, userID)
}

func (s SqlUserAccessTokenStore) deleteTokensByUser(transaction *gorp.Transaction, userID string) error {
	if _, err := transaction.Exec("DELETE FROM UserAccessTokens WHERE UserId = :UserId", map[string]interface{}{"UserId": userID}); err != nil {
		return errors.Wrapf(err, "failed to delete UserAccessToken userId=%s", userID)
	}

	return nil
}

func (s SqlUserAccessTokenStore) Get(tokenID string) (*model.UserAccessToken, error) {
	token := model.UserAccessToken{}

	if err := s.GetReplica().SelectOne(&token, "SELECT * FROM UserAccessTokens WHERE Id = :Id", map[string]interface{}{"Id": tokenID}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("UserAccessToken", tokenID)
		}
		return nil, errors.Wrapf(err, "failed to get UserAccessToken with id=%s", tokenID)
	}

	return &token, nil
}

func (s SqlUserAccessTokenStore) GetAll(offset, limit int) ([]*model.UserAccessToken, error) {
	tokens := []*model.UserAccessToken{}

	if _, err := s.GetReplica().Select(&tokens, "SELECT * FROM UserAccessTokens LIMIT :Limit OFFSET :Offset", map[string]interface{}{"Offset": offset, "Limit": limit}); err != nil {
		return nil, errors.Wrap(err, "failed to find UserAccessTokens")
	}

	return tokens, nil
}

func (s SqlUserAccessTokenStore) GetByToken(tokenString string) (*model.UserAccessToken, error) {
	token := model.UserAccessToken{}

	if err := s.GetReplica().SelectOne(&token, "SELECT * FROM UserAccessTokens WHERE Token = :Token", map[string]interface{}{"Token": tokenString}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("UserAccessToken", fmt.Sprintf("token=%s", tokenString))
		}
		return nil, errors.Wrapf(err, "failed to get UserAccessToken with token=%s", tokenString)
	}

	return &token, nil
}

func (s SqlUserAccessTokenStore) GetByUser(userID string, offset, limit int) ([]*model.UserAccessToken, error) {
	tokens := []*model.UserAccessToken{}

	if _, err := s.GetReplica().Select(&tokens, "SELECT * FROM UserAccessTokens WHERE UserId = :UserId LIMIT :Limit OFFSET :Offset", map[string]interface{}{"UserId": userID, "Offset": offset, "Limit": limit}); err != nil {
		return nil, errors.Wrapf(err, "failed to find UserAccessTokens with userId=%s", userID)
	}

	return tokens, nil
}

func (s SqlUserAccessTokenStore) Search(term string) ([]*model.UserAccessToken, error) {
	term = sanitizeSearchTerm(term, "\\")
	tokens := []*model.UserAccessToken{}
	params := map[string]interface{}{"Term": term + "%"}
	query := `
		SELECT
			uat.*
		FROM UserAccessTokens uat
		INNER JOIN Users u
			ON uat.UserId = u.Id
		WHERE uat.Id LIKE :Term OR uat.UserId LIKE :Term OR u.Username LIKE :Term`

	if _, err := s.GetReplica().Select(&tokens, query, params); err != nil {
		return nil, errors.Wrapf(err, "failed to find UserAccessTokens by term with value '%s'", term)
	}

	return tokens, nil
}

func (s SqlUserAccessTokenStore) UpdateTokenEnable(tokenID string) error {
	if _, err := s.GetMaster().Exec("UPDATE UserAccessTokens SET IsActive = TRUE WHERE Id = :Id", map[string]interface{}{"Id": tokenID}); err != nil {
		return errors.Wrapf(err, "failed to update UserAccessTokens with id=%s", tokenID)
	}
	return nil
}

func (s SqlUserAccessTokenStore) UpdateTokenDisable(tokenID string) error {
	transaction, err := s.GetMaster().Begin()
	if err != nil {
		return errors.Wrap(err, "begin_transaction")
	}
	defer finalizeTransaction(transaction)

	if err := s.deleteSessionsAndDisableToken(transaction, tokenID); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		// don't need to rollback here since the transaction is already closed
		return errors.Wrap(err, "commit_transaction")
	}
	return nil
}

func (s SqlUserAccessTokenStore) deleteSessionsAndDisableToken(transaction *gorp.Transaction, tokenID string) error {
	query := ""
	if s.DriverName() == model.DatabaseDriverPostgres {
		query = "DELETE FROM Sessions s USING UserAccessTokens o WHERE o.Token = s.Token AND o.Id = :Id"
	} else if s.DriverName() == model.DatabaseDriverMysql {
		query = "DELETE s.* FROM Sessions s INNER JOIN UserAccessTokens o ON o.Token = s.Token WHERE o.Id = :Id"
	}

	if _, err := transaction.Exec(query, map[string]interface{}{"Id": tokenID}); err != nil {
		return errors.Wrapf(err, "failed to delete Sessions with UserAccessToken id=%s", tokenID)
	}

	return s.updateTokenDisable(transaction, tokenID)
}

func (s SqlUserAccessTokenStore) updateTokenDisable(transaction *gorp.Transaction, tokenID string) error {
	if _, err := transaction.Exec("UPDATE UserAccessTokens SET IsActive = FALSE WHERE Id = :Id", map[string]interface{}{"Id": tokenID}); err != nil {
		return errors.Wrapf(err, "failed to update UserAccessToken with id=%s", tokenID)
	}

	return nil
}
