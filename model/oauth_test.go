// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOAuthAppJSON(t *testing.T) {
	a1 := OAuthApp{}
	a1.ID = NewID()
	a1.Name = "TestOAuthApp" + NewID()
	a1.CallbackURLs = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"
	a1.IconURL = "https://nowhere.com/icon_image.png"
	a1.ClientSecret = NewID()

	json := a1.ToJSON()
	ra1 := OAuthAppFromJSON(strings.NewReader(json))

	require.Equal(t, a1.ID, ra1.ID, "ids did not match")
}

func TestOAuthAppPreSave(t *testing.T) {
	a1 := OAuthApp{}
	a1.ID = NewID()
	a1.Name = "TestOAuthApp" + NewID()
	a1.CallbackURLs = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"
	a1.IconURL = "https://nowhere.com/icon_image.png"
	a1.ClientSecret = NewID()
	a1.PreSave()
	a1.Etag()
	a1.Sanitize()
}

func TestOAuthAppPreUpdate(t *testing.T) {
	a1 := OAuthApp{}
	a1.ID = NewID()
	a1.Name = "TestOAuthApp" + NewID()
	a1.CallbackURLs = []string{"https://nowhere.com"}
	a1.Homepage = "https://nowhere.com"
	a1.IconURL = "https://nowhere.com/icon_image.png"
	a1.ClientSecret = NewID()
	a1.PreUpdate()
}

func TestOAuthAppIsValid(t *testing.T) {
	app := OAuthApp{}

	require.NotNil(t, app.IsValid())

	app.ID = NewID()
	require.NotNil(t, app.IsValid())

	app.CreateAt = 1
	require.NotNil(t, app.IsValid())

	app.UpdateAt = 1
	require.NotNil(t, app.IsValid())

	app.CreatorID = NewID()
	require.NotNil(t, app.IsValid())

	app.ClientSecret = NewID()
	require.NotNil(t, app.IsValid())

	app.Name = "TestOAuthApp"
	require.NotNil(t, app.IsValid())

	app.CallbackURLs = []string{"https://nowhere.com"}
	require.NotNil(t, app.IsValid())

	app.Homepage = "https://nowhere.com"
	require.Nil(t, app.IsValid())

	app.IconURL = "https://nowhere.com/icon_image.png"
	require.Nil(t, app.IsValid())
}
