// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestTermsOfServiceStore(t *testing.T, ss store.Store) {
	t.Run("TestSaveTermsOfService", func(t *testing.T) { testSaveTermsOfService(t, ss) })
	t.Run("TestGetLatestTermsOfService", func(t *testing.T) { testGetLatestTermsOfService(t, ss) })
	t.Run("TestGetTermsOfService", func(t *testing.T) { testGetTermsOfService(t, ss) })
}

func cleanUpTOS(ss store.Store) {
	// Clearing out the table before starting the test.
	// Otherwise the row inserted by the previous Save call from testSaveTermsOfService
	// gets picked up.
	// We call DropAllTables but we actually need to delete only TermsOfService.
	// However, there is no straightforward way to just clear that table without introducing
	// new methods. So we use the hammer.
	ss.DropAllTables()
}

func testSaveTermsOfService(t *testing.T, ss store.Store) {
	t.Cleanup(func() { cleanUpTOS(ss) })

	u1 := model.User{}
	u1.Username = model.NewID()
	u1.Email = MakeEmail()
	u1.Nickname = model.NewID()
	_, err := ss.User().Save(&u1)
	require.NoError(t, err)

	termsOfService := &model.TermsOfService{Text: "terms of service", UserID: u1.ID}
	savedTermsOfService, err := ss.TermsOfService().Save(termsOfService)
	require.NoError(t, err)

	require.Len(t, savedTermsOfService.ID, 26, "Id should have been populated")

	require.NotEqual(t, savedTermsOfService.CreateAt, 0, "Create at should have been populated")
}

func testGetLatestTermsOfService(t *testing.T, ss store.Store) {
	t.Cleanup(func() { cleanUpTOS(ss) })

	u1 := model.User{}
	u1.Username = model.NewID()
	u1.Email = MakeEmail()
	u1.Nickname = model.NewID()
	_, err := ss.User().Save(&u1)
	require.NoError(t, err)

	termsOfService := &model.TermsOfService{Text: "terms of service 2", UserID: u1.ID}
	_, err = ss.TermsOfService().Save(termsOfService)
	require.NoError(t, err)

	fetchedTermsOfService, err := ss.TermsOfService().GetLatest(true)
	require.NoError(t, err)
	assert.Equal(t, termsOfService.Text, fetchedTermsOfService.Text)
	assert.Equal(t, termsOfService.UserID, fetchedTermsOfService.UserID)
}

func testGetTermsOfService(t *testing.T, ss store.Store) {
	t.Cleanup(func() { cleanUpTOS(ss) })

	u1 := model.User{}
	u1.Username = model.NewID()
	u1.Email = MakeEmail()
	u1.Nickname = model.NewID()
	_, err := ss.User().Save(&u1)
	require.NoError(t, err)

	termsOfService := &model.TermsOfService{Text: "terms of service", UserID: u1.ID}
	_, err = ss.TermsOfService().Save(termsOfService)
	require.NoError(t, err)

	r1, err := ss.TermsOfService().Get("an_invalid_id", true)
	assert.Error(t, err)
	assert.Nil(t, r1)

	receivedTermsOfService, err := ss.TermsOfService().Get(termsOfService.ID, true)
	assert.NoError(t, err)
	assert.Equal(t, "terms of service", receivedTermsOfService.Text)
}
