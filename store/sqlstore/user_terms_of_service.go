// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SQLUserTermsOfServiceStore struct {
	*SQLStore
}

func newSQLUserTermsOfServiceStore(sqlStore *SQLStore) store.UserTermsOfServiceStore {
	s := SQLUserTermsOfServiceStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.UserTermsOfService{}, "UserTermsOfService").SetKeys(false, "UserId")
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("TermsOfServiceId").SetMaxSize(26)
	}

	return s
}

func (s SQLUserTermsOfServiceStore) createIndexesIfNotExists() {
}

func (s SQLUserTermsOfServiceStore) GetByUser(userID string) (*model.UserTermsOfService, error) {
	var userTermsOfService *model.UserTermsOfService

	err := s.GetReplica().SelectOne(&userTermsOfService, "SELECT * FROM UserTermsOfService WHERE UserId = :userId", map[string]interface{}{"userId": userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("UserTermsOfService", "userId="+userID)
		}
		return nil, errors.Wrapf(err, "failed to get UserTermsOfService with userId=%s", userID)
	}
	return userTermsOfService, nil
}

func (s SQLUserTermsOfServiceStore) Save(userTermsOfService *model.UserTermsOfService) (*model.UserTermsOfService, error) {
	userTermsOfService.PreSave()

	if err := userTermsOfService.IsValid(); err != nil {
		return nil, err
	}

	c, err := s.GetMaster().Update(userTermsOfService)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update UserTermsOfService with userId=%s and termsOfServiceId=%s", userTermsOfService.UserID, userTermsOfService.TermsOfServiceID)
	}

	if c == 0 {
		if err := s.GetMaster().Insert(userTermsOfService); err != nil {
			return nil, errors.Wrapf(err, "failed to save UserTermsOfService with userId=%s and termsOfServiceId=%s", userTermsOfService.UserID, userTermsOfService.TermsOfServiceID)
		}
	}

	return userTermsOfService, nil
}

func (s SQLUserTermsOfServiceStore) Delete(userID, termsOfServiceID string) error {
	if _, err := s.GetMaster().Exec("DELETE FROM UserTermsOfService WHERE UserId = :UserId AND TermsOfServiceId = :TermsOfServiceId", map[string]interface{}{"UserId": userID, "TermsOfServiceId": termsOfServiceID}); err != nil {
		return errors.Wrapf(err, "failed to delete UserTermsOfService with userId=%s and termsOfServiceId=%s", userID, termsOfServiceID)
	}
	return nil
}
