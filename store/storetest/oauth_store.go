// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestOAuthStore(t *testing.T, ss store.Store) {
	t.Run("SaveApp", func(t *testing.T) { testOAuthStoreSaveApp(t, ss) })
	t.Run("GetApp", func(t *testing.T) { testOAuthStoreGetApp(t, ss) })
	t.Run("UpdateApp", func(t *testing.T) { testOAuthStoreUpdateApp(t, ss) })
	t.Run("SaveAccessData", func(t *testing.T) { testOAuthStoreSaveAccessData(t, ss) })
	t.Run("OAuthUpdateAccessData", func(t *testing.T) { testOAuthUpdateAccessData(t, ss) })
	t.Run("GetAccessData", func(t *testing.T) { testOAuthStoreGetAccessData(t, ss) })
	t.Run("RemoveAccessData", func(t *testing.T) { testOAuthStoreRemoveAccessData(t, ss) })
	t.Run("RemoveAllAccessData", func(t *testing.T) { testOAuthStoreRemoveAllAccessData(t, ss) })
	t.Run("SaveAuthData", func(t *testing.T) { testOAuthStoreSaveAuthData(t, ss) })
	t.Run("GetAuthData", func(t *testing.T) { testOAuthStoreGetAuthData(t, ss) })
	t.Run("RemoveAuthData", func(t *testing.T) { testOAuthStoreRemoveAuthData(t, ss) })
	t.Run("RemoveAuthDataByUser", func(t *testing.T) { testOAuthStoreRemoveAuthDataByUser(t, ss) })
	t.Run("OAuthGetAuthorizedApps", func(t *testing.T) { testOAuthGetAuthorizedApps(t, ss) })
	t.Run("OAuthGetAccessDataByUserForApp", func(t *testing.T) { testOAuthGetAccessDataByUserForApp(t, ss) })
	t.Run("DeleteApp", func(t *testing.T) { testOAuthStoreDeleteApp(t, ss) })
}

func testOAuthStoreSaveApp(t *testing.T, ss store.Store) {
	a1 := model.OAuthApp{}
	a1.CreatorID = model.NewID()
	a1.CallbackUrls = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"

	// Try to save an app that already has an Id
	a1.ID = model.NewID()
	_, err := ss.OAuth().SaveApp(&a1)
	require.Error(t, err, "Should have failed, cannot add an OAuth app cannot be save with an Id, it has to be updated")

	// Try to save an Invalid App
	a1.ID = ""
	_, err = ss.OAuth().SaveApp(&a1)
	require.Error(t, err, "Should have failed, app should be invalid cause it doesn' have a name set")

	// Save the app
	a1.ID = ""
	a1.Name = "TestApp" + model.NewID()
	_, err = ss.OAuth().SaveApp(&a1)
	require.NoError(t, err)
}

func testOAuthStoreGetApp(t *testing.T, ss store.Store) {
	a1 := model.OAuthApp{}
	a1.CreatorID = model.NewID()
	a1.Name = "TestApp" + model.NewID()
	a1.CallbackUrls = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"
	_, err := ss.OAuth().SaveApp(&a1)
	require.NoError(t, err)

	// Lets try to get and app that does not exists
	_, err = ss.OAuth().GetApp("fake0123456789abcderfgret1")
	require.Error(t, err, "Should have failed. App does not exists")

	_, err = ss.OAuth().GetApp(a1.ID)
	require.NoError(t, err)

	// Lets try and get the app from a user that hasn't created any apps
	apps, err := ss.OAuth().GetAppByUser("fake0123456789abcderfgret1", 0, 1000)
	require.NoError(t, err)
	assert.Empty(t, apps, "Should have failed. Fake user hasn't created any apps")

	_, err = ss.OAuth().GetAppByUser(a1.CreatorID, 0, 1000)
	require.NoError(t, err)

	_, err = ss.OAuth().GetApps(0, 1000)
	require.NoError(t, err)
}

func testOAuthStoreUpdateApp(t *testing.T, ss store.Store) {
	a1 := model.OAuthApp{}
	a1.CreatorID = model.NewID()
	a1.Name = "TestApp" + model.NewID()
	a1.CallbackUrls = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"
	_, err := ss.OAuth().SaveApp(&a1)
	require.NoError(t, err)

	// temporarily save the created app id
	id := a1.ID

	a1.CreateAt = 1
	a1.ClientSecret = "pwd"
	a1.CreatorID = "12345678901234567890123456"

	// Lets update the app by removing the name
	a1.Name = ""
	_, err = ss.OAuth().UpdateApp(&a1)
	require.Error(t, err, "Should have failed. App name is not set")

	// Lets not find the app that we are trying to update
	a1.ID = "fake0123456789abcderfgret1"
	a1.Name = "NewName"
	_, err = ss.OAuth().UpdateApp(&a1)
	require.Error(t, err, "Should have failed. Not able to find the app")

	a1.ID = id
	ua, err := ss.OAuth().UpdateApp(&a1)
	require.NoError(t, err)
	require.Equal(t, ua.Name, "NewName", "name did not update")
	require.NotEqual(t, ua.CreateAt, 1, "create at should not have updated")
	require.NotEqual(t, ua.CreatorID, "12345678901234567890123456", "creator id should not have updated")
}

func testOAuthStoreSaveAccessData(t *testing.T, ss store.Store) {
	a1 := model.AccessData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()

	// Lets try and save an incomplete access data
	_, err := ss.OAuth().SaveAccessData(&a1)
	require.Error(t, err, "Should have failed. Access data needs the token")

	a1.Token = model.NewID()
	a1.RefreshToken = model.NewID()
	a1.RedirectURI = "http://example.com"

	_, err = ss.OAuth().SaveAccessData(&a1)
	require.NoError(t, err)
}

func testOAuthUpdateAccessData(t *testing.T, ss store.Store) {
	a1 := model.AccessData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()
	a1.Token = model.NewID()
	a1.RefreshToken = model.NewID()
	a1.ExpiresAt = model.GetMillis()
	a1.RedirectURI = "http://example.com"
	_, err := ss.OAuth().SaveAccessData(&a1)
	require.NoError(t, err)

	//Try to update to invalid Refresh Token
	refreshToken := a1.RefreshToken
	a1.RefreshToken = model.NewID() + "123"
	_, err = ss.OAuth().UpdateAccessData(&a1)
	require.Error(t, err, "Should have failed with invalid token")

	//Try to update to invalid RedirectUri
	a1.RefreshToken = model.NewID()
	a1.RedirectURI = ""
	_, err = ss.OAuth().UpdateAccessData(&a1)
	require.Error(t, err, "Should have failed with invalid Redirect URI")

	// Should update fine
	a1.RedirectURI = "http://example.com"
	ra1, err := ss.OAuth().UpdateAccessData(&a1)
	require.NoError(t, err)
	require.NotEqual(t, ra1.RefreshToken, refreshToken, "refresh tokens didn't match")
}

func testOAuthStoreGetAccessData(t *testing.T, ss store.Store) {
	a1 := model.AccessData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()
	a1.Token = model.NewID()
	a1.RefreshToken = model.NewID()
	a1.ExpiresAt = model.GetMillis()
	a1.RedirectURI = "http://example.com"
	_, err := ss.OAuth().SaveAccessData(&a1)
	require.NoError(t, err)

	_, err = ss.OAuth().GetAccessData("invalidToken")
	require.Error(t, err, "Should have failed. There is no data with an invalid token")

	ra1, err := ss.OAuth().GetAccessData(a1.Token)
	require.NoError(t, err)
	assert.Equal(t, a1.Token, ra1.Token, "tokens didn't match")

	_, err = ss.OAuth().GetPreviousAccessData(a1.UserID, a1.ClientID)
	require.NoError(t, err)

	_, err = ss.OAuth().GetPreviousAccessData("user", "junk")
	require.NoError(t, err)

	// Try to get the Access data using an invalid refresh token
	_, err = ss.OAuth().GetAccessDataByRefreshToken(a1.Token)
	require.Error(t, err, "Should have failed. There is no data with an invalid token")

	// Get the Access Data using the refresh token
	ra1, err = ss.OAuth().GetAccessDataByRefreshToken(a1.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, a1.RefreshToken, ra1.RefreshToken, "tokens didn't match")
}

func testOAuthStoreRemoveAccessData(t *testing.T, ss store.Store) {
	a1 := model.AccessData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()
	a1.Token = model.NewID()
	a1.RefreshToken = model.NewID()
	a1.RedirectURI = "http://example.com"
	_, err := ss.OAuth().SaveAccessData(&a1)
	require.NoError(t, err)

	err = ss.OAuth().RemoveAccessData(a1.Token)
	require.NoError(t, err)

	result, _ := ss.OAuth().GetPreviousAccessData(a1.UserID, a1.ClientID)
	require.Nil(t, result, "did not delete access token")
}

func testOAuthStoreRemoveAllAccessData(t *testing.T, ss store.Store) {
	a1 := model.AccessData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()
	a1.Token = model.NewID()
	a1.RefreshToken = model.NewID()
	a1.RedirectURI = "http://example.com"
	_, err := ss.OAuth().SaveAccessData(&a1)
	require.NoError(t, err)

	err = ss.OAuth().RemoveAllAccessData()
	require.NoError(t, err)

	result, _ := ss.OAuth().GetPreviousAccessData(a1.UserID, a1.ClientID)
	require.Nil(t, result, "did not delete access token")
}

func testOAuthStoreSaveAuthData(t *testing.T, ss store.Store) {
	a1 := model.AuthData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()
	a1.Code = model.NewID()
	a1.RedirectURI = "http://example.com"
	_, err := ss.OAuth().SaveAuthData(&a1)
	require.NoError(t, err)
}

func testOAuthStoreGetAuthData(t *testing.T, ss store.Store) {
	a1 := model.AuthData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()
	a1.Code = model.NewID()
	a1.RedirectURI = "http://example.com"
	_, err := ss.OAuth().SaveAuthData(&a1)
	require.NoError(t, err)

	_, err = ss.OAuth().GetAuthData(a1.Code)
	require.NoError(t, err)
}

func testOAuthStoreRemoveAuthData(t *testing.T, ss store.Store) {
	a1 := model.AuthData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()
	a1.Code = model.NewID()
	a1.RedirectURI = "http://example.com"
	_, err := ss.OAuth().SaveAuthData(&a1)
	require.NoError(t, err)

	err = ss.OAuth().RemoveAuthData(a1.Code)
	require.NoError(t, err)

	_, err = ss.OAuth().GetAuthData(a1.Code)
	require.Error(t, err, "should have errored - auth code removed")
}

func testOAuthStoreRemoveAuthDataByUser(t *testing.T, ss store.Store) {
	a1 := model.AuthData{}
	a1.ClientID = model.NewID()
	a1.UserID = model.NewID()
	a1.Code = model.NewID()
	a1.RedirectURI = "http://example.com"
	_, err := ss.OAuth().SaveAuthData(&a1)
	require.NoError(t, err)

	err = ss.OAuth().PermanentDeleteAuthDataByUser(a1.UserID)
	require.NoError(t, err)
}

func testOAuthGetAuthorizedApps(t *testing.T, ss store.Store) {
	a1 := model.OAuthApp{}
	a1.CreatorID = model.NewID()
	a1.Name = "TestApp" + model.NewID()
	a1.CallbackUrls = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"
	_, err := ss.OAuth().SaveApp(&a1)
	require.NoError(t, err)

	// Lets try and get an Authorized app for a user who hasn't authorized it
	apps, err := ss.OAuth().GetAuthorizedApps("fake0123456789abcderfgret1", 0, 1000)
	require.NoError(t, err)
	assert.Empty(t, apps, "Should have failed. Fake user hasn't authorized the app")

	// allow the app
	p := model.Preference{}
	p.UserID = a1.CreatorID
	p.Category = model.PreferenceCategoryAuthorizedOAuthApp
	p.Name = a1.ID
	p.Value = "true"
	nErr := ss.Preference().Save(&model.Preferences{p})
	require.NoError(t, nErr)

	apps, err = ss.OAuth().GetAuthorizedApps(a1.CreatorID, 0, 1000)
	require.NoError(t, err)
	assert.NotEqual(t, len(apps), 0, "It should have return apps")
}

func testOAuthGetAccessDataByUserForApp(t *testing.T, ss store.Store) {
	a1 := model.OAuthApp{}
	a1.CreatorID = model.NewID()
	a1.Name = "TestApp" + model.NewID()
	a1.CallbackUrls = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"
	_, err := ss.OAuth().SaveApp(&a1)
	require.NoError(t, err)

	// allow the app
	p := model.Preference{}
	p.UserID = a1.CreatorID
	p.Category = model.PreferenceCategoryAuthorizedOAuthApp
	p.Name = a1.ID
	p.Value = "true"
	nErr := ss.Preference().Save(&model.Preferences{p})
	require.NoError(t, nErr)

	apps, err := ss.OAuth().GetAuthorizedApps(a1.CreatorID, 0, 1000)
	require.NoError(t, err)
	assert.NotEqual(t, len(apps), 0, "It should have return apps")

	// save the token
	ad1 := model.AccessData{}
	ad1.ClientID = a1.ID
	ad1.UserID = a1.CreatorID
	ad1.Token = model.NewID()
	ad1.RefreshToken = model.NewID()
	ad1.RedirectURI = "http://example.com"

	_, err = ss.OAuth().SaveAccessData(&ad1)
	require.NoError(t, err)

	accessData, err := ss.OAuth().GetAccessDataByUserForApp(a1.CreatorID, a1.ID)
	require.NoError(t, err)
	assert.NotEqual(t, len(accessData), 0, "It should have return access data")
}

func testOAuthStoreDeleteApp(t *testing.T, ss store.Store) {
	a1 := model.OAuthApp{}
	a1.CreatorID = model.NewID()
	a1.Name = "TestApp" + model.NewID()
	a1.CallbackUrls = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"
	_, err := ss.OAuth().SaveApp(&a1)
	require.NoError(t, err)

	// delete a non-existent app
	err = ss.OAuth().DeleteApp("fakeclientId")
	require.NoError(t, err)

	s1 := &model.Session{}
	s1.UserID = model.NewID()
	s1.Token = model.NewID()
	s1.IsOAuth = true

	s1, nErr := ss.Session().Save(s1)
	require.NoError(t, nErr)

	ad1 := model.AccessData{}
	ad1.ClientID = a1.ID
	ad1.UserID = a1.CreatorID
	ad1.Token = s1.Token
	ad1.RefreshToken = model.NewID()
	ad1.RedirectURI = "http://example.com"

	_, err = ss.OAuth().SaveAccessData(&ad1)
	require.NoError(t, err)

	err = ss.OAuth().DeleteApp(a1.ID)
	require.NoError(t, err)

	_, nErr = ss.Session().Get(context.Background(), s1.Token)
	require.Error(t, nErr, "should error - session should be deleted")

	_, err = ss.OAuth().GetAccessData(s1.Token)
	require.Error(t, err, "should error - access data should be deleted")
}
