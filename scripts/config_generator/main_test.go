// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/mattermost/mattermost-server/v5/model"

	"github.com/stretchr/testify/require"
)

func TestDefaultsGenerator(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "tempconfig")
	defer os.Remove(tmpFile.Name())
	require.NoError(t, err)
	require.NoError(t, generateDefaultConfig(tmpFile))
	_ = tmpFile.Close()
	var config model.Config

	b, err := ioutil.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &config))
	require.True(t, *config.ServiceSettings.DisableLegacyMFA)
	require.Equal(t, *config.SqlSettings.AtRestEncryptKey, "")
	require.Equal(t, *config.FileSettings.PublicLinkSalt, "")

	require.Equal(t, *config.Office365Settings.Scope, model.Office365SettingsDefaultScope)
	require.Equal(t, *config.Office365Settings.AuthEndpoint, model.Office365SettingsDefaultAuthEndpoint)
	require.Equal(t, *config.Office365Settings.UserAPIEndpoint, model.Office365SettingsDefaultUserAPIEndpoint)
	require.Equal(t, *config.Office365Settings.TokenEndpoint, model.Office365SettingsDefaultTokenEndpoint)

	require.Equal(t, *config.GoogleSettings.Scope, model.GoogleSettingsDefaultScope)
	require.Equal(t, *config.GoogleSettings.AuthEndpoint, model.GoogleSettingsDefaultAuthEndpoint)
	require.Equal(t, *config.GoogleSettings.UserAPIEndpoint, model.GoogleSettingsDefaultUserAPIEndpoint)
	require.Equal(t, *config.GoogleSettings.TokenEndpoint, model.GoogleSettingsDefaultTokenEndpoint)
}
