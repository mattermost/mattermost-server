// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestUserTermsOfServiceStore(t *testing.T, ss store.Store) {
	t.Run("TestSaveUserTermsOfService", func(t *testing.T) { testSaveUserTermsOfService(t, ss) })
	t.Run("TestGetByUserTermsOfService", func(t *testing.T) { testGetByUserTermsOfService(t, ss) })
	t.Run("TestDeleteUserTermsOfService", func(t *testing.T) { testDeleteUserTermsOfService(t, ss) })
}

func testSaveUserTermsOfService(t *testing.T, ss store.Store) {
	userTermsOfService := &model.UserTermsOfService{
		UserID:           model.NewID(),
		TermsOfServiceID: model.NewID(),
	}

	savedUserTermsOfService, err := ss.UserTermsOfService().Save(userTermsOfService)
	require.NoError(t, err)
	assert.Equal(t, userTermsOfService.UserID, savedUserTermsOfService.UserID)
	assert.Equal(t, userTermsOfService.TermsOfServiceID, savedUserTermsOfService.TermsOfServiceID)
	assert.NotEmpty(t, savedUserTermsOfService.CreateAt)
}

func testGetByUserTermsOfService(t *testing.T, ss store.Store) {
	userTermsOfService := &model.UserTermsOfService{
		UserID:           model.NewID(),
		TermsOfServiceID: model.NewID(),
	}

	_, err := ss.UserTermsOfService().Save(userTermsOfService)
	require.NoError(t, err)

	fetchedUserTermsOfService, err := ss.UserTermsOfService().GetByUser(userTermsOfService.UserID)
	require.NoError(t, err)
	assert.Equal(t, userTermsOfService.UserID, fetchedUserTermsOfService.UserID)
	assert.Equal(t, userTermsOfService.TermsOfServiceID, fetchedUserTermsOfService.TermsOfServiceID)
	assert.NotEmpty(t, fetchedUserTermsOfService.CreateAt)
}

func testDeleteUserTermsOfService(t *testing.T, ss store.Store) {
	userTermsOfService := &model.UserTermsOfService{
		UserID:           model.NewID(),
		TermsOfServiceID: model.NewID(),
	}

	_, err := ss.UserTermsOfService().Save(userTermsOfService)
	require.NoError(t, err)

	_, err = ss.UserTermsOfService().GetByUser(userTermsOfService.UserID)
	require.NoError(t, err)

	err = ss.UserTermsOfService().Delete(userTermsOfService.UserID, userTermsOfService.TermsOfServiceID)
	require.NoError(t, err)

	_, err = ss.UserTermsOfService().GetByUser(userTermsOfService.UserID)
	var nfErr *store.ErrNotFound
	assert.Error(t, err)
	assert.True(t, errors.As(err, &nfErr))
}
