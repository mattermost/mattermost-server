// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/einterfaces/mocks"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func getDefaultPluginSettingsSchema() string {
	ret, _ := json.Marshal(model.PluginSettingsSchema{
		Settings: []*model.PluginSetting{
			{Key: "BasicChannelName", Type: "text"},
			{Key: "BasicChannelId", Type: "text"},
			{Key: "BasicTeamDisplayName", Type: "text"},
			{Key: "BasicTeamName", Type: "text"},
			{Key: "BasicTeamId", Type: "text"},
			{Key: "BasicUserEmail", Type: "text"},
			{Key: "BasicUserId", Type: "text"},
			{Key: "BasicUser2Email", Type: "text"},
			{Key: "BasicUser2Id", Type: "text"},
			{Key: "BasicPostMessage", Type: "text"},
		},
	})
	return string(ret)
}

func setDefaultPluginConfig(th *TestHelper, pluginID string) {
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.PluginSettings.Plugins[pluginID] = map[string]interface{}{
			"BasicChannelName":     th.BasicChannel.Name,
			"BasicChannelId":       th.BasicChannel.Id,
			"BasicTeamName":        th.BasicTeam.Name,
			"BasicTeamId":          th.BasicTeam.Id,
			"BasicTeamDisplayName": th.BasicTeam.DisplayName,
			"BasicUserEmail":       th.BasicUser.Email,
			"BasicUserId":          th.BasicUser.Id,
			"BasicUser2Email":      th.BasicUser2.Email,
			"BasicUser2Id":         th.BasicUser2.Id,
			"BasicPostMessage":     th.BasicPost.Message,
		}
	})
}

func setupMultiPluginApiTest(t *testing.T, pluginCodes []string, pluginManifests []string, pluginIDs []string, asMain bool, app *App, c *request.Context) string {
	pluginDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.RemoveAll(pluginDir)
		if err != nil {
			t.Logf("Failed to cleanup pluginDir %s", err.Error())
		}
	})

	webappPluginDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.RemoveAll(webappPluginDir)
		if err != nil {
			t.Logf("Failed to cleanup webappPluginDir %s", err.Error())
		}
	})

	newPluginAPI := func(manifest *model.Manifest) plugin.API {
		return app.NewPluginAPI(c, manifest)
	}

	env, err := plugin.NewEnvironment(newPluginAPI, NewDriverImpl(app.srv), pluginDir, webappPluginDir, app.Log(), nil)
	require.NoError(t, err)

	require.Equal(t, len(pluginCodes), len(pluginIDs))
	require.Equal(t, len(pluginManifests), len(pluginIDs))

	for i, pluginID := range pluginIDs {
		backend := filepath.Join(pluginDir, pluginID, "backend.exe")
		if asMain {
			utils.CompileGo(t, pluginCodes[i], backend)
		} else {
			utils.CompileGoTest(t, pluginCodes[i], backend)
		}

		ioutil.WriteFile(filepath.Join(pluginDir, pluginID, "plugin.json"), []byte(pluginManifests[i]), 0600)
		manifest, activated, reterr := env.Activate(pluginID)
		require.NoError(t, reterr)
		require.NotNil(t, manifest)
		require.True(t, activated)

		app.UpdateConfig(func(cfg *model.Config) {
			cfg.PluginSettings.PluginStates[pluginID] = &model.PluginState{
				Enable: true,
			}
		})
	}

	app.SetPluginsEnvironment(env)

	return pluginDir
}

func setupPluginApiTest(t *testing.T, pluginCode string, pluginManifest string, pluginID string, app *App, c *request.Context) string {

	asMain := pluginID != "test_db_driver"
	return setupMultiPluginApiTest(t,
		[]string{pluginCode}, []string{pluginManifest}, []string{pluginID},
		asMain, app, c)
}

func TestPublicFilesPathConfiguration(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	pluginID := "com.mattermost.sample"

	pluginDir := setupPluginApiTest(t,
		`
		package main

		import (
			"github.com/mattermost/mattermost-server/v5/plugin"
		)

		type MyPlugin struct {
			plugin.MattermostPlugin
		}

		func main() {
			plugin.ClientMain(&MyPlugin{})
		}
	`,
		`{"id": "com.mattermost.sample", "server": {"executable": "backend.exe"}, "settings_schema": {"settings": []}}`, pluginID, th.App, th.Context)

	publicFilesFolderInTest := filepath.Join(pluginDir, pluginID, "public")
	publicFilesPath, err := th.App.GetPluginsEnvironment().PublicFilesPath(pluginID)
	assert.NoError(t, err)
	assert.Equal(t, publicFilesPath, publicFilesFolderInTest)
}

func TestPluginAPIGetUserPreferences(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	user1, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user1" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user1)

	preferences, err := api.GetPreferencesForUser(user1.Id)
	require.Nil(t, err)
	assert.Equal(t, 1, len(preferences))

	assert.Equal(t, user1.Id, preferences[0].UserId)
	assert.Equal(t, model.PREFERENCE_CATEGORY_TUTORIAL_STEPS, preferences[0].Category)
	assert.Equal(t, user1.Id, preferences[0].Name)
	assert.Equal(t, "0", preferences[0].Value)
}

func TestPluginAPIDeleteUserPreferences(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	user1, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user1" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user1)

	preferences, err := api.GetPreferencesForUser(user1.Id)
	require.Nil(t, err)
	assert.Equal(t, 1, len(preferences))

	err = api.DeletePreferencesForUser(user1.Id, preferences)
	require.Nil(t, err)
	preferences, err = api.GetPreferencesForUser(user1.Id)
	require.Nil(t, err)
	assert.Equal(t, 0, len(preferences))

	user2, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user2" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user2)

	preference := model.Preference{
		Name:     user2.Id,
		UserId:   user2.Id,
		Category: model.PREFERENCE_CATEGORY_THEME,
		Value:    `{"color": "#ff0000", "color2": "#faf"}`,
	}
	err = api.UpdatePreferencesForUser(user2.Id, []model.Preference{preference})
	require.Nil(t, err)

	preferences, err = api.GetPreferencesForUser(user2.Id)
	require.Nil(t, err)
	assert.Equal(t, 2, len(preferences))

	err = api.DeletePreferencesForUser(user2.Id, []model.Preference{preference})
	require.Nil(t, err)
	preferences, err = api.GetPreferencesForUser(user2.Id)
	require.Nil(t, err)
	assert.Equal(t, 1, len(preferences))
	assert.Equal(t, model.PREFERENCE_CATEGORY_TUTORIAL_STEPS, preferences[0].Category)
}

func TestPluginAPIUpdateUserPreferences(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	user1, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user1" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user1)

	preferences, err := api.GetPreferencesForUser(user1.Id)
	require.Nil(t, err)
	assert.Equal(t, 1, len(preferences))
	assert.Equal(t, user1.Id, preferences[0].UserId)
	assert.Equal(t, model.PREFERENCE_CATEGORY_TUTORIAL_STEPS, preferences[0].Category)
	assert.Equal(t, user1.Id, preferences[0].Name)
	assert.Equal(t, "0", preferences[0].Value)

	preference := model.Preference{
		Name:     user1.Id,
		UserId:   user1.Id,
		Category: model.PREFERENCE_CATEGORY_THEME,
		Value:    `{"color": "#ff0000", "color2": "#faf"}`,
	}

	err = api.UpdatePreferencesForUser(user1.Id, []model.Preference{preference})
	require.Nil(t, err)

	preferences, err = api.GetPreferencesForUser(user1.Id)
	require.Nil(t, err)

	assert.Equal(t, 2, len(preferences))
	expectedCategories := []string{model.PREFERENCE_CATEGORY_TUTORIAL_STEPS, model.PREFERENCE_CATEGORY_THEME}
	for _, pref := range preferences {
		assert.Contains(t, expectedCategories, pref.Category)
		assert.Equal(t, user1.Id, pref.UserId)
		assert.Equal(t, user1.Id, pref.Name)
		if pref.Category == model.PREFERENCE_CATEGORY_TUTORIAL_STEPS {
			assert.Equal(t, "0", pref.Value)
		} else {
			newTheme, _ := json.Marshal(map[string]string{"color": "#ff0000", "color2": "#faf"})
			assert.Equal(t, string(newTheme), pref.Value)
		}
	}
}

func TestPluginAPIGetUsers(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	user1, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user1" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user1)

	user2, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user2" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user2)

	user3, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user3" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user3)

	user4, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user4" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user4)

	testCases := []struct {
		Description   string
		Page          int
		PerPage       int
		ExpectedUsers []*model.User
	}{
		{
			"page 0, perPage 0",
			0,
			0,
			[]*model.User{},
		},
		{
			"page 0, perPage 10",
			0,
			10,
			[]*model.User{user1, user2, user3, user4},
		},
		{
			"page 0, perPage 2",
			0,
			2,
			[]*model.User{user1, user2},
		},
		{
			"page 1, perPage 2",
			1,
			2,
			[]*model.User{user3, user4},
		},
		{
			"page 10, perPage 10",
			10,
			10,
			[]*model.User{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			users, err := api.GetUsers(&model.UserGetOptions{
				Page:    testCase.Page,
				PerPage: testCase.PerPage,
			})
			assert.Nil(t, err)
			assert.Equal(t, testCase.ExpectedUsers, users)
		})
	}
}

func TestPluginAPIGetUsersInTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	team1 := th.CreateTeam()
	team2 := th.CreateTeam()

	user1, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user1" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user1)

	user2, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user2" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user2)

	user3, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user3" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user3)

	user4, err := th.App.CreateUser(th.Context, &model.User{
		Email:    strings.ToLower(model.NewId()) + "success+test@example.com",
		Password: "password",
		Username: "user4" + model.NewId(),
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteUser(th.Context, user4)

	// Add all users to team 1
	_, _, err = th.App.joinUserToTeam(team1, user1)
	require.Nil(t, err)
	_, _, err = th.App.joinUserToTeam(team1, user2)
	require.Nil(t, err)
	_, _, err = th.App.joinUserToTeam(team1, user3)
	require.Nil(t, err)
	_, _, err = th.App.joinUserToTeam(team1, user4)
	require.Nil(t, err)

	// Add only user3 and user4 to team 2
	_, _, err = th.App.joinUserToTeam(team2, user3)
	require.Nil(t, err)
	_, _, err = th.App.joinUserToTeam(team2, user4)
	require.Nil(t, err)

	testCases := []struct {
		Description   string
		TeamId        string
		Page          int
		PerPage       int
		ExpectedUsers []*model.User
	}{
		{
			"unknown team",
			model.NewId(),
			0,
			0,
			[]*model.User{},
		},
		{
			"team 1, page 0, perPage 10",
			team1.Id,
			0,
			10,
			[]*model.User{user1, user2, user3, user4},
		},
		{
			"team 1, page 0, perPage 2",
			team1.Id,
			0,
			2,
			[]*model.User{user1, user2},
		},
		{
			"team 1, page 1, perPage 2",
			team1.Id,
			1,
			2,
			[]*model.User{user3, user4},
		},
		{
			"team 2, page 0, perPage 10",
			team2.Id,
			0,
			10,
			[]*model.User{user3, user4},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			users, err := api.GetUsersInTeam(testCase.TeamId, testCase.Page, testCase.PerPage)
			assert.Nil(t, err)
			assert.Equal(t, testCase.ExpectedUsers, users)
		})
	}
}

func TestPluginAPIGetFile(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	// check a valid file first
	uploadTime := time.Date(2007, 2, 4, 1, 2, 3, 4, time.Local)
	filename := "testGetFile"
	fileData := []byte("Hello World")
	info, err := th.App.DoUploadFile(th.Context, uploadTime, th.BasicTeam.Id, th.BasicChannel.Id, th.BasicUser.Id, filename, fileData)
	require.Nil(t, err)
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info.Id)
		th.App.RemoveFile(info.Path)
	}()

	data, err1 := api.GetFile(info.Id)
	require.Nil(t, err1)
	assert.Equal(t, data, fileData)

	// then checking invalid file
	data, err = api.GetFile("../fake/testingApi")
	require.NotNil(t, err)
	require.Nil(t, data)
}

func TestPluginAPIGetFileInfos(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	fileInfo1, err := th.App.DoUploadFile(th.Context,
		time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC),
		th.BasicTeam.Id,
		th.BasicChannel.Id,
		th.BasicUser.Id,
		"testFile1",
		[]byte("testfile1 Content"),
	)
	require.Nil(t, err)
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(fileInfo1.Id)
		th.App.RemoveFile(fileInfo1.Path)
	}()

	fileInfo2, err := th.App.DoUploadFile(th.Context,
		time.Date(2020, 1, 2, 1, 1, 1, 1, time.UTC),
		th.BasicTeam.Id,
		th.BasicChannel.Id,
		th.BasicUser2.Id,
		"testFile2",
		[]byte("testfile2 Content"),
	)
	require.Nil(t, err)
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(fileInfo2.Id)
		th.App.RemoveFile(fileInfo2.Path)
	}()

	fileInfo3, err := th.App.DoUploadFile(th.Context,
		time.Date(2020, 1, 3, 1, 1, 1, 1, time.UTC),
		th.BasicTeam.Id,
		th.BasicChannel.Id,
		th.BasicUser.Id,
		"testFile3",
		[]byte("testfile3 Content"),
	)
	require.Nil(t, err)
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(fileInfo3.Id)
		th.App.RemoveFile(fileInfo3.Path)
	}()

	_, err = api.CreatePost(&model.Post{
		Message:   "testFile1",
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		FileIds:   model.StringArray{fileInfo1.Id},
	})
	require.Nil(t, err)

	_, err = api.CreatePost(&model.Post{
		Message:   "testFile2",
		UserId:    th.BasicUser2.Id,
		ChannelId: th.BasicChannel.Id,
		FileIds:   model.StringArray{fileInfo2.Id},
	})
	require.Nil(t, err)

	t.Run("get file infos with no options 2nd page of 1 per page", func(t *testing.T) {
		fileInfos, err := api.GetFileInfos(1, 1, nil)
		require.Nil(t, err)
		require.Len(t, fileInfos, 1)
	})
	t.Run("get file infos filtered by user", func(t *testing.T) {
		fileInfos, err := api.GetFileInfos(0, 5, &model.GetFileInfosOptions{
			UserIds: []string{th.BasicUser.Id},
		})
		require.Nil(t, err)
		require.Len(t, fileInfos, 2)
	})
	t.Run("get file infos filtered by channel ordered by created at descending", func(t *testing.T) {
		fileInfos, err := api.GetFileInfos(0, 5, &model.GetFileInfosOptions{
			ChannelIds:     []string{th.BasicChannel.Id},
			SortBy:         model.FILEINFO_SORT_BY_CREATED,
			SortDescending: true,
		})
		require.Nil(t, err)
		require.Len(t, fileInfos, 2)
		require.Equal(t, fileInfos[0].Id, fileInfo2.Id)
		require.Equal(t, fileInfos[1].Id, fileInfo1.Id)
	})
}

func TestPluginAPISavePluginConfig(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	manifest := &model.Manifest{
		Id: "pluginid",
		SettingsSchema: &model.PluginSettingsSchema{
			Settings: []*model.PluginSetting{
				{Key: "MyStringSetting", Type: "text"},
				{Key: "MyIntSetting", Type: "text"},
				{Key: "MyBoolSetting", Type: "bool"},
			},
		},
	}

	api := NewPluginAPI(th.App, th.Context, manifest)

	pluginConfigJsonString := `{"mystringsetting": "str", "MyIntSetting": 32, "myboolsetting": true}`

	var pluginConfig map[string]interface{}
	err := json.Unmarshal([]byte(pluginConfigJsonString), &pluginConfig)
	require.NoError(t, err)

	appErr := api.SavePluginConfig(pluginConfig)
	require.Nil(t, appErr)

	type Configuration struct {
		MyStringSetting string
		MyIntSetting    int
		MyBoolSetting   bool
	}

	savedConfiguration := new(Configuration)
	err = api.LoadPluginConfiguration(savedConfiguration)
	require.NoError(t, err)

	expectedConfiguration := new(Configuration)
	err = json.Unmarshal([]byte(pluginConfigJsonString), &expectedConfiguration)
	require.NoError(t, err)

	assert.Equal(t, expectedConfiguration, savedConfiguration)
}

func TestPluginAPIGetPluginConfig(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	manifest := &model.Manifest{
		Id: "pluginid",
		SettingsSchema: &model.PluginSettingsSchema{
			Settings: []*model.PluginSetting{
				{Key: "MyStringSetting", Type: "text"},
				{Key: "MyIntSetting", Type: "text"},
				{Key: "MyBoolSetting", Type: "bool"},
			},
		},
	}

	api := NewPluginAPI(th.App, th.Context, manifest)

	pluginConfigJsonString := `{"mystringsetting": "str", "myintsetting": 32, "myboolsetting": true}`
	var pluginConfig map[string]interface{}

	err := json.Unmarshal([]byte(pluginConfigJsonString), &pluginConfig)
	require.NoError(t, err)

	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.PluginSettings.Plugins["pluginid"] = pluginConfig
	})

	savedPluginConfig := api.GetPluginConfig()
	assert.Equal(t, pluginConfig, savedPluginConfig)
}

func TestPluginAPILoadPluginConfiguration(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	var pluginJson map[string]interface{}
	err := json.Unmarshal([]byte(`{"mystringsetting": "str", "MyIntSetting": 32, "myboolsetting": true}`), &pluginJson)
	require.NoError(t, err)

	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.PluginSettings.Plugins["testloadpluginconfig"] = pluginJson
	})

	testFolder, found := fileutils.FindDir("mattermost-server/app/plugin_api_tests")
	require.True(t, found, "Cannot find tests folder")
	fullPath := path.Join(testFolder, "manual.test_load_configuration_plugin", "main.go")

	err = pluginAPIHookTest(t, th, fullPath, "testloadpluginconfig", `{"id": "testloadpluginconfig", "backend": {"executable": "backend.exe"}, "settings_schema": {
		"settings": [
			{
				"key": "MyStringSetting",
				"type": "text"
			},
			{
				"key": "MyIntSetting",
				"type": "text"
			},
			{
				"key": "MyBoolSetting",
				"type": "bool"
			}
		]
	}}`)
	require.NoError(t, err)

}

func TestPluginAPILoadPluginConfigurationDefaults(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	var pluginJson map[string]interface{}
	err := json.Unmarshal([]byte(`{"mystringsetting": "override"}`), &pluginJson)
	require.NoError(t, err)

	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.PluginSettings.Plugins["testloadpluginconfig"] = pluginJson
	})

	testFolder, found := fileutils.FindDir("mattermost-server/app/plugin_api_tests")
	require.True(t, found, "Cannot find tests folder")
	fullPath := path.Join(testFolder, "manual.test_load_configuration_defaults_plugin", "main.go")

	err = pluginAPIHookTest(t, th, fullPath, "testloadpluginconfig", `{
		"settings": [
			{
				"key": "MyStringSetting",
				"type": "text",
				"default": "notthis"
			},
			{
				"key": "MyIntSetting",
				"type": "text",
				"default": 35
			},
			{
				"key": "MyBoolSetting",
				"type": "bool",
				"default": true
			}
		]
	}`)

	require.NoError(t, err)

}

func TestPluginAPIGetPlugins(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	pluginCode := `
    package main

    import (
      "github.com/mattermost/mattermost-server/v5/plugin"
    )

    type MyPlugin struct {
      plugin.MattermostPlugin
    }

    func main() {
      plugin.ClientMain(&MyPlugin{})
    }
  `

	pluginDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	webappPluginDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(pluginDir)
	defer os.RemoveAll(webappPluginDir)

	env, err := plugin.NewEnvironment(th.NewPluginAPI, NewDriverImpl(th.Server), pluginDir, webappPluginDir, th.App.Log(), nil)
	require.NoError(t, err)

	pluginIDs := []string{"pluginid1", "pluginid2", "pluginid3"}
	var pluginManifests []*model.Manifest
	for _, pluginID := range pluginIDs {
		backend := filepath.Join(pluginDir, pluginID, "backend.exe")
		utils.CompileGo(t, pluginCode, backend)

		ioutil.WriteFile(filepath.Join(pluginDir, pluginID, "plugin.json"), []byte(fmt.Sprintf(`{"id": "%s", "server": {"executable": "backend.exe"}}`, pluginID)), 0600)
		manifest, activated, reterr := env.Activate(pluginID)

		require.NoError(t, reterr)
		require.NotNil(t, manifest)
		require.True(t, activated)
		pluginManifests = append(pluginManifests, manifest)
	}
	th.App.SetPluginsEnvironment(env)

	// Deactivate the last one for testing
	success := env.Deactivate(pluginIDs[len(pluginIDs)-1])
	require.True(t, success)

	// check existing user first
	plugins, appErr := api.GetPlugins()
	assert.Nil(t, appErr)
	assert.NotEmpty(t, plugins)
	assert.Equal(t, pluginManifests, plugins)
}

func TestPluginAPIInstallPlugin(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	path, _ := fileutils.FindDir("tests")
	tarData, err := ioutil.ReadFile(filepath.Join(path, "testplugin.tar.gz"))
	require.NoError(t, err)

	_, appErr := api.InstallPlugin(bytes.NewReader(tarData), true)
	assert.NotNil(t, appErr, "should not allow upload if upload disabled")
	assert.Equal(t, appErr.Error(), "installPlugin: Plugins and/or plugin uploads have been disabled., ")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.Enable = true
		*cfg.PluginSettings.EnableUploads = true
	})

	manifest, appErr := api.InstallPlugin(bytes.NewReader(tarData), true)
	defer os.RemoveAll("plugins/testplugin")
	require.Nil(t, appErr)
	assert.Equal(t, "testplugin", manifest.Id)

	// Successfully installed
	pluginsResp, appErr := api.GetPlugins()
	require.Nil(t, appErr)

	found := false
	for _, m := range pluginsResp {
		if m.Id == manifest.Id {
			found = true
		}
	}

	assert.True(t, found)
}

func TestInstallPlugin(t *testing.T) {
	// TODO(ilgooz): remove this setup func to use existent setupPluginApiTest().
	// following setupTest() func is a modified version of setupPluginApiTest().
	// we need a modified version of setupPluginApiTest() because it wasn't possible to use it directly here
	// since it removes plugin dirs right after it returns, does not update App configs with the plugin
	// dirs and this behavior tends to break this test as a result.
	setupTest := func(t *testing.T, pluginCode string, pluginManifest string, pluginID string, app *App, c *request.Context) (func(), string) {
		pluginDir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		webappPluginDir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		app.UpdateConfig(func(cfg *model.Config) {
			*cfg.PluginSettings.Directory = pluginDir
			*cfg.PluginSettings.ClientDirectory = webappPluginDir
		})

		newPluginAPI := func(manifest *model.Manifest) plugin.API {
			return app.NewPluginAPI(c, manifest)
		}

		env, err := plugin.NewEnvironment(newPluginAPI, NewDriverImpl(app.srv), pluginDir, webappPluginDir, app.Log(), nil)
		require.NoError(t, err)

		app.SetPluginsEnvironment(env)

		backend := filepath.Join(pluginDir, pluginID, "backend.exe")
		utils.CompileGo(t, pluginCode, backend)

		ioutil.WriteFile(filepath.Join(pluginDir, pluginID, "plugin.json"), []byte(pluginManifest), 0600)
		manifest, activated, reterr := env.Activate(pluginID)
		require.NoError(t, reterr)
		require.NotNil(t, manifest)
		require.True(t, activated)

		return func() {
			os.RemoveAll(pluginDir)
			os.RemoveAll(webappPluginDir)
		}, pluginDir
	}

	th := Setup(t)
	defer th.TearDown()

	// start an http server to serve plugin's tarball to the test.
	path, _ := fileutils.FindDir("tests")
	ts := httptest.NewServer(http.FileServer(http.Dir(path)))
	defer ts.Close()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.Enable = true
		*cfg.PluginSettings.EnableUploads = true
		cfg.PluginSettings.Plugins["testinstallplugin"] = map[string]interface{}{
			"DownloadURL": ts.URL + "/testplugin.tar.gz",
		}
	})

	tearDown, _ := setupTest(t,
		`
		package main

		import (
			"net/http"

			"github.com/pkg/errors"

			"github.com/mattermost/mattermost-server/v5/plugin"
		)

		type configuration struct {
			DownloadURL string
		}

		type Plugin struct {
			plugin.MattermostPlugin

			configuration configuration
		}

		func (p *Plugin) OnConfigurationChange() error {
			if err := p.API.LoadPluginConfiguration(&p.configuration); err != nil {
				return err
			}
			return nil
		}

		func (p *Plugin) OnActivate() error {
			resp, err := http.Get(p.configuration.DownloadURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			_, aerr := p.API.InstallPlugin(resp.Body, true)
			if aerr != nil {
				return errors.Wrap(aerr, "cannot install plugin")
			}
			return nil
		}

		func main() {
			plugin.ClientMain(&Plugin{})
		}

	`,
		`{"id": "testinstallplugin", "backend": {"executable": "backend.exe"}, "settings_schema": {
		"settings": [
			{
				"key": "DownloadURL",
				"type": "text"
			}
		]
	}}`, "testinstallplugin", th.App, th.Context)
	defer tearDown()

	hooks, err := th.App.GetPluginsEnvironment().HooksForPlugin("testinstallplugin")
	require.NoError(t, err)

	err = hooks.OnActivate()
	require.NoError(t, err)

	plugins, aerr := th.App.GetPlugins()
	require.Nil(t, aerr)
	require.Len(t, plugins.Inactive, 1)
	require.Equal(t, "testplugin", plugins.Inactive[0].Id)
}

func TestPluginAPIGetTeamIcon(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	// Create an 128 x 128 image
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	// Draw a red dot at (2, 3)
	img.Set(2, 3, color.RGBA{255, 0, 0, 255})
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	require.NoError(t, err)
	dataBytes := buf.Bytes()
	fileReader := bytes.NewReader(dataBytes)

	// Set the Team Icon
	appErr := th.App.SetTeamIconFromFile(th.BasicTeam, fileReader)
	require.Nil(t, appErr)

	// Get the team icon to check
	teamIcon, appErr := api.GetTeamIcon(th.BasicTeam.Id)
	require.Nil(t, appErr)
	require.NotEmpty(t, teamIcon)

	colorful := color.NRGBA{255, 0, 0, 255}
	byteReader := bytes.NewReader(teamIcon)
	img2, _, err2 := image.Decode(byteReader)
	require.NoError(t, err2)
	require.Equal(t, img2.At(2, 3), colorful)
}

func TestPluginAPISetTeamIcon(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	// Create an 128 x 128 image
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	// Draw a red dot at (2, 3)
	img.Set(2, 3, color.RGBA{255, 0, 0, 255})
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	require.NoError(t, err)
	dataBytes := buf.Bytes()

	// Set the user profile image
	appErr := api.SetTeamIcon(th.BasicTeam.Id, dataBytes)
	require.Nil(t, appErr)

	// Get the user profile image to check
	teamIcon, appErr := api.GetTeamIcon(th.BasicTeam.Id)
	require.Nil(t, appErr)
	require.NotEmpty(t, teamIcon)

	colorful := color.NRGBA{255, 0, 0, 255}
	byteReader := bytes.NewReader(teamIcon)
	img2, _, err2 := image.Decode(byteReader)
	require.NoError(t, err2)
	require.Equal(t, img2.At(2, 3), colorful)
}

func TestPluginAPIRemoveTeamIcon(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	// Create an 128 x 128 image
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))

	// Draw a red dot at (2, 3)
	img.Set(2, 3, color.RGBA{255, 0, 0, 255})
	buf := new(bytes.Buffer)
	err1 := png.Encode(buf, img)
	require.NoError(t, err1)
	dataBytes := buf.Bytes()
	fileReader := bytes.NewReader(dataBytes)

	// Set the Team Icon
	err := th.App.SetTeamIconFromFile(th.BasicTeam, fileReader)
	require.Nil(t, err)
	err = api.RemoveTeamIcon(th.BasicTeam.Id)
	require.Nil(t, err)
}

func pluginAPIHookTest(t *testing.T, th *TestHelper, fileName string, id string, settingsSchema string) error {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	code := string(data)
	schema := `{"settings": [ ]	}`
	if settingsSchema != "" {
		schema = settingsSchema
	}
	th.App.srv.sqlStore = th.GetSqlStore()
	setupPluginApiTest(t, code,
		fmt.Sprintf(`{"id": "%v", "backend": {"executable": "backend.exe"}, "settings_schema": %v}`, id, schema),
		id, th.App, th.Context)
	hooks, err := th.App.GetPluginsEnvironment().HooksForPlugin(id)
	require.NoError(t, err)
	require.NotNil(t, hooks)
	_, ret := hooks.MessageWillBePosted(nil, nil)
	if ret != "OK" {
		return errors.New(ret)
	}
	return nil
}

// This is a meta-test function. It does the following:
// 1. Scans "tests/plugin_tests" folder
// 2. For each folder - compiles the main.go inside and executes it, validating it's result
// 3. If folder starts with "manual." it is skipped ("manual." tests executed in other part of this file)
// 4. Before compiling the main.go file is passed through templating and the following values are available in the template: BasicUser, BasicUser2, BasicChannel, BasicTeam, BasicPost
// 5. Successfully running test should return nil, "OK". Any other returned string is considered and error

func TestBasicAPIPlugins(t *testing.T) {
	defaultSchema := getDefaultPluginSettingsSchema()
	testFolder, found := fileutils.FindDir("mattermost-server/app/plugin_api_tests")
	require.True(t, found, "Cannot read find app folder")
	dirs, err := ioutil.ReadDir(testFolder)
	require.NoError(t, err, "Cannot read test folder %v", testFolder)
	for _, dir := range dirs {
		d := dir.Name()
		if dir.IsDir() && !strings.HasPrefix(d, "manual.") {
			t.Run(d, func(t *testing.T) {
				mainPath := path.Join(testFolder, d, "main.go")
				_, err := os.Stat(mainPath)
				require.NoError(t, err, "Cannot find plugin main file at %v", mainPath)
				th := Setup(t).InitBasic()
				defer th.TearDown()
				setDefaultPluginConfig(th, dir.Name())
				err = pluginAPIHookTest(t, th, mainPath, dir.Name(), defaultSchema)
				require.NoError(t, err)
			})
		}
	}
}

func TestPluginAPIKVCompareAndSet(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	testCases := []struct {
		Description   string
		ExpectedValue []byte
	}{
		{
			Description:   "Testing non-nil, non-empty value",
			ExpectedValue: []byte("value1"),
		},
		{
			Description:   "Testing empty value",
			ExpectedValue: []byte(""),
		},
	}

	for i, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			expectedKey := fmt.Sprintf("Key%d", i)
			expectedValueEmpty := []byte("")
			expectedValue1 := testCase.ExpectedValue
			expectedValue2 := []byte("value2")
			expectedValue3 := []byte("value3")

			// Attempt update using an incorrect old value
			updated, err := api.KVCompareAndSet(expectedKey, expectedValue2, expectedValue1)
			require.Nil(t, err)
			require.False(t, updated)

			// Make sure no key is already created
			value, err := api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Nil(t, value)

			// Insert using nil old value
			updated, err = api.KVCompareAndSet(expectedKey, nil, expectedValue1)
			require.Nil(t, err)
			require.True(t, updated)

			// Get inserted value
			value, err = api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Equal(t, expectedValue1, value)

			// Attempt to insert again using nil old value
			updated, err = api.KVCompareAndSet(expectedKey, nil, expectedValue2)
			require.Nil(t, err)
			require.False(t, updated)

			// Get old value to assert nothing has changed
			value, err = api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Equal(t, expectedValue1, value)

			// Update using correct old value
			updated, err = api.KVCompareAndSet(expectedKey, expectedValue1, expectedValue2)
			require.Nil(t, err)
			require.True(t, updated)

			value, err = api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Equal(t, expectedValue2, value)

			// Update using incorrect old value
			updated, err = api.KVCompareAndSet(expectedKey, []byte("incorrect"), expectedValue3)
			require.Nil(t, err)
			require.False(t, updated)

			value, err = api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Equal(t, expectedValue2, value)

			// Update using nil old value
			updated, err = api.KVCompareAndSet(expectedKey, nil, expectedValue3)
			require.Nil(t, err)
			require.False(t, updated)

			value, err = api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Equal(t, expectedValue2, value)

			// Update using empty old value
			updated, err = api.KVCompareAndSet(expectedKey, expectedValueEmpty, expectedValue3)
			require.Nil(t, err)
			require.False(t, updated)

			value, err = api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Equal(t, expectedValue2, value)
		})
	}
}

func TestPluginAPIKVCompareAndDelete(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	testCases := []struct {
		Description   string
		ExpectedValue []byte
	}{
		{
			Description:   "Testing non-nil, non-empty value",
			ExpectedValue: []byte("value1"),
		},
		{
			Description:   "Testing empty value",
			ExpectedValue: []byte(""),
		},
	}

	for i, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			expectedKey := fmt.Sprintf("Key%d", i)
			expectedValue1 := testCase.ExpectedValue
			expectedValue2 := []byte("value2")

			// Set the value
			err := api.KVSet(expectedKey, expectedValue1)
			require.Nil(t, err)

			// Attempt delete using an incorrect old value
			deleted, err := api.KVCompareAndDelete(expectedKey, expectedValue2)
			require.Nil(t, err)
			require.False(t, deleted)

			// Make sure the value is still there
			value, err := api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Equal(t, expectedValue1, value)

			// Attempt delete using the proper value
			deleted, err = api.KVCompareAndDelete(expectedKey, expectedValue1)
			require.Nil(t, err)
			require.True(t, deleted)

			// Verify it's deleted
			value, err = api.KVGet(expectedKey)
			require.Nil(t, err)
			require.Nil(t, value)
		})
	}
}

func TestPluginCreateBot(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	bot, err := api.CreateBot(&model.Bot{
		Username:    model.NewRandomString(10),
		DisplayName: "bot",
		Description: "bot",
	})
	require.Nil(t, err)

	_, err = api.CreateBot(&model.Bot{
		Username:    model.NewRandomString(10),
		OwnerId:     bot.UserId,
		DisplayName: "bot2",
		Description: "bot2",
	})
	require.NotNil(t, err)

}

func TestPluginCreatePostWithUploadedFile(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	data := []byte("Hello World")
	channelID := th.BasicChannel.Id
	filename := "testGetFile"
	fileInfo, err := api.UploadFile(data, channelID, filename)
	require.Nil(t, err)
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(fileInfo.Id)
		th.App.RemoveFile(fileInfo.Path)
	}()

	actualData, err := api.GetFile(fileInfo.Id)
	require.Nil(t, err)
	assert.Equal(t, data, actualData)

	userID := th.BasicUser.Id
	post, err := api.CreatePost(&model.Post{
		Message:   "test",
		UserId:    userID,
		ChannelId: channelID,
		FileIds:   model.StringArray{fileInfo.Id},
	})
	require.Nil(t, err)
	assert.Equal(t, model.StringArray{fileInfo.Id}, post.FileIds)

	actualPost, err := api.GetPost(post.Id)
	require.Nil(t, err)
	assert.Equal(t, model.StringArray{fileInfo.Id}, actualPost.FileIds)
}

func TestPluginAPIGetConfig(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	config := api.GetConfig()
	if config.LdapSettings.BindPassword != nil && *config.LdapSettings.BindPassword != "" {
		assert.Equal(t, *config.LdapSettings.BindPassword, model.FAKE_SETTING)
	}

	assert.Equal(t, *config.FileSettings.PublicLinkSalt, model.FAKE_SETTING)

	if *config.FileSettings.AmazonS3SecretAccessKey != "" {
		assert.Equal(t, *config.FileSettings.AmazonS3SecretAccessKey, model.FAKE_SETTING)
	}

	if config.EmailSettings.SMTPPassword != nil && *config.EmailSettings.SMTPPassword != "" {
		assert.Equal(t, *config.EmailSettings.SMTPPassword, model.FAKE_SETTING)
	}

	if *config.GitLabSettings.Secret != "" {
		assert.Equal(t, *config.GitLabSettings.Secret, model.FAKE_SETTING)
	}

	assert.Equal(t, *config.SqlSettings.DataSource, model.FAKE_SETTING)
	assert.Equal(t, *config.SqlSettings.AtRestEncryptKey, model.FAKE_SETTING)
	assert.Equal(t, *config.ElasticsearchSettings.Password, model.FAKE_SETTING)

	for i := range config.SqlSettings.DataSourceReplicas {
		assert.Equal(t, config.SqlSettings.DataSourceReplicas[i], model.FAKE_SETTING)
	}

	for i := range config.SqlSettings.DataSourceSearchReplicas {
		assert.Equal(t, config.SqlSettings.DataSourceSearchReplicas[i], model.FAKE_SETTING)
	}
}

func TestPluginAPIGetUnsanitizedConfig(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	api := th.SetupPluginAPI()

	config := api.GetUnsanitizedConfig()
	if config.LdapSettings.BindPassword != nil && *config.LdapSettings.BindPassword != "" {
		assert.NotEqual(t, *config.LdapSettings.BindPassword, model.FAKE_SETTING)
	}

	assert.NotEqual(t, *config.FileSettings.PublicLinkSalt, model.FAKE_SETTING)

	if *config.FileSettings.AmazonS3SecretAccessKey != "" {
		assert.NotEqual(t, *config.FileSettings.AmazonS3SecretAccessKey, model.FAKE_SETTING)
	}

	if config.EmailSettings.SMTPPassword != nil && *config.EmailSettings.SMTPPassword != "" {
		assert.NotEqual(t, *config.EmailSettings.SMTPPassword, model.FAKE_SETTING)
	}

	if *config.GitLabSettings.Secret != "" {
		assert.NotEqual(t, *config.GitLabSettings.Secret, model.FAKE_SETTING)
	}

	assert.NotEqual(t, *config.SqlSettings.DataSource, model.FAKE_SETTING)
	assert.NotEqual(t, *config.SqlSettings.AtRestEncryptKey, model.FAKE_SETTING)
	assert.NotEqual(t, *config.ElasticsearchSettings.Password, model.FAKE_SETTING)

	for i := range config.SqlSettings.DataSourceReplicas {
		assert.NotEqual(t, config.SqlSettings.DataSourceReplicas[i], model.FAKE_SETTING)
	}

	for i := range config.SqlSettings.DataSourceSearchReplicas {
		assert.NotEqual(t, config.SqlSettings.DataSourceSearchReplicas[i], model.FAKE_SETTING)
	}
}

func TestPluginAddUserToChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	member, err := api.AddUserToChannel(th.BasicChannel.Id, th.BasicUser.Id, th.BasicUser2.Id)
	require.Nil(t, err)
	require.NotNil(t, member)
	require.Equal(t, th.BasicChannel.Id, member.ChannelId)
	require.Equal(t, th.BasicUser.Id, member.UserId)
}

func TestInterpluginPluginHTTP(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	setupMultiPluginApiTest(t,
		[]string{`
		package main

		import (
			"github.com/mattermost/mattermost-server/v5/plugin"
			"bytes"
			"net/http"
		)

		type MyPlugin struct {
			plugin.MattermostPlugin
		}

		func (p *MyPlugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/test":
				if r.URL.Query().Get("abc") != "xyz" {
					return
				}

				if r.Header.Get("Mattermost-Plugin-ID") != "testplugininterclient" {
					return
				}

				buf := bytes.Buffer{}
				buf.ReadFrom(r.Body)
				resp := "we got:" + buf.String()
				w.WriteHeader(598)
				w.Write([]byte(resp))
				if r.URL.Path != "/api/v2/test" {
					return
				}
			case "/nobody":
				w.WriteHeader(599)
			}
		}

		func main() {
			plugin.ClientMain(&MyPlugin{})
		}
		`,
			`
		package main

		import (
			"github.com/mattermost/mattermost-server/v5/plugin"
			"github.com/mattermost/mattermost-server/v5/model"
			"bytes"
			"net/http"
			"io/ioutil"
		)

		type MyPlugin struct {
			plugin.MattermostPlugin
		}

		func (p *MyPlugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
			buf := bytes.Buffer{}
			buf.WriteString("This is the request")
			req, err := http.NewRequest("GET", "/testplugininterserver/api/v2/test?abc=xyz", &buf)
			if err != nil {
				return nil, err.Error()
			}
			req.Header.Add("Mattermost-User-Id", "userid")
			resp := p.API.PluginHTTP(req)
			if resp == nil {
				return nil, "Nil resp"
			}
			if resp.Body == nil {
				return nil, "Nil body"
			}
			respbody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, err.Error()
			}
			if resp.StatusCode != 598 {
				return nil, "wrong status " + string(respbody)
			}

			if string(respbody) !=  "we got:This is the request" {
				return nil, "wrong response " + string(respbody)
			}

			req, err = http.NewRequest("GET", "/testplugininterserver/nobody", nil)
			if err != nil {
				return nil, err.Error()
			}

			resp = p.API.PluginHTTP(req)
			if resp == nil {
				return nil, "Nil resp"
			}

			if resp.StatusCode != 599 {
				return nil, "wrong status " + string(respbody)
			}

			return nil, "ok"
		}

		func main() {
			plugin.ClientMain(&MyPlugin{})
		}
		`,
		},
		[]string{
			`{"id": "testplugininterserver", "backend": {"executable": "backend.exe"}}`,
			`{"id": "testplugininterclient", "backend": {"executable": "backend.exe"}}`,
		},
		[]string{
			"testplugininterserver",
			"testplugininterclient",
		},
		true,
		th.App,
		th.Context,
	)

	hooks, err := th.App.GetPluginsEnvironment().HooksForPlugin("testplugininterclient")
	require.NoError(t, err)
	_, ret := hooks.MessageWillBePosted(nil, nil)
	assert.Equal(t, "ok", ret)
}

func TestApiMetrics(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("", func(t *testing.T) {
		metricsMock := &mocks.MetricsInterface{}

		pluginDir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		webappPluginDir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		defer os.RemoveAll(pluginDir)
		defer os.RemoveAll(webappPluginDir)

		env, err := plugin.NewEnvironment(th.NewPluginAPI, NewDriverImpl(th.Server), pluginDir, webappPluginDir, th.App.Log(), metricsMock)
		require.NoError(t, err)

		th.App.SetPluginsEnvironment(env)

		pluginID := model.NewId()
		backend := filepath.Join(pluginDir, pluginID, "backend.exe")
		code :=
			`
	package main

	import (
		"github.com/mattermost/mattermost-server/v5/model"
		"github.com/mattermost/mattermost-server/v5/plugin"
	)

	type MyPlugin struct {
		plugin.MattermostPlugin
	}

	func (p *MyPlugin) UserHasBeenCreated(c *plugin.Context, user *model.User) {
		user.Nickname = "plugin-callback-success"
		p.API.UpdateUser(user)
	}

	func main() {
		plugin.ClientMain(&MyPlugin{})
	}
`
		utils.CompileGo(t, code, backend)
		ioutil.WriteFile(filepath.Join(pluginDir, pluginID, "plugin.json"), []byte(`{"id": "`+pluginID+`", "backend": {"executable": "backend.exe"}}`), 0600)

		// Don't care about these mocks
		metricsMock.On("ObservePluginHookDuration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		metricsMock.On("ObservePluginMultiHookIterationDuration", mock.Anything, mock.Anything, mock.Anything).Return()
		metricsMock.On("ObservePluginMultiHookDuration", mock.Anything).Return()

		// Setup mocks
		metricsMock.On("ObservePluginApiDuration", pluginID, "UpdateUser", true, mock.Anything).Return()

		_, _, activationErr := env.Activate(pluginID)
		require.NoError(t, activationErr)

		require.True(t, th.App.GetPluginsEnvironment().IsActive(pluginID))

		user1 := &model.User{
			Email:       model.NewId() + "success+test@example.com",
			Nickname:    "Darth Vader1",
			Username:    "vader" + model.NewId(),
			Password:    "passwd1",
			AuthService: "",
		}
		_, appErr := th.App.CreateUser(th.Context, user1)
		require.Nil(t, appErr)
		time.Sleep(1 * time.Second)
		user1, appErr = th.App.GetUser(user1.Id)
		require.Nil(t, appErr)
		require.Equal(t, "plugin-callback-success", user1.Nickname)

		// Disable plugin
		require.True(t, th.App.GetPluginsEnvironment().Deactivate(pluginID))
		require.False(t, th.App.GetPluginsEnvironment().IsActive(pluginID))

		metricsMock.AssertExpectations(t)
	})
}

func TestPluginAPIGetPostsForChannel(t *testing.T) {
	require := require.New(t)

	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	numPosts := 10

	// GetPostsForChannel returns posts ordered with the most recent first, so we
	// need to invert the expected slice, the oldest post being BasicPost
	expectedPosts := make([]*model.Post, numPosts)
	expectedPosts[numPosts-1] = th.BasicPost
	for i := numPosts - 2; i >= 0; i-- {
		expectedPosts[i] = th.CreatePost(th.BasicChannel)
	}
	// CreatePost does not add Metadata, but initializes the structure. GetPostsForChannel
	// returns nil for an empty Metadata, so we need to match that behaviour
	for _, post := range expectedPosts {
		post.Metadata = nil
	}

	postList, err := api.GetPostsForChannel(th.BasicChannel.Id, 0, 0)
	require.Nil(err)
	require.Nil(postList.ToSlice())

	postList, err = api.GetPostsForChannel(th.BasicChannel.Id, 0, numPosts/2)
	require.Nil(err)
	require.Equal(expectedPosts[:numPosts/2], postList.ToSlice())

	postList, err = api.GetPostsForChannel(th.BasicChannel.Id, 1, numPosts/2)
	require.Nil(err)
	require.Equal(expectedPosts[numPosts/2:], postList.ToSlice())

	postList, err = api.GetPostsForChannel(th.BasicChannel.Id, 2, numPosts/2)
	require.Nil(err)
	require.Nil(postList.ToSlice())

	postList, err = api.GetPostsForChannel(th.BasicChannel.Id, 0, numPosts+1)
	require.Nil(err)
	require.Equal(expectedPosts, postList.ToSlice())
}

func TestPluginHTTPConnHijack(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	testFolder, found := fileutils.FindDir("mattermost-server/app/plugin_api_tests")
	require.True(t, found, "Cannot find tests folder")
	fullPath := path.Join(testFolder, "manual.test_http_hijack_plugin", "main.go")

	pluginCode, err := ioutil.ReadFile(fullPath)
	require.NoError(t, err)
	require.NotEmpty(t, pluginCode)

	tearDown, ids, errors := SetAppEnvironmentWithPlugins(t, []string{string(pluginCode)}, th.App, th.NewPluginAPI)
	defer tearDown()
	require.NoError(t, errors[0])
	require.Len(t, ids, 1)

	pluginID := ids[0]
	require.NotEmpty(t, pluginID)

	reqURL := fmt.Sprintf("http://localhost:%d/plugins/%s", th.Server.ListenAddr.Port, pluginID)
	req, err := http.NewRequest("GET", reqURL, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "OK", string(body))
}

func TestPluginHTTPUpgradeWebSocket(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	testFolder, found := fileutils.FindDir("mattermost-server/app/plugin_api_tests")
	require.True(t, found, "Cannot find tests folder")
	fullPath := path.Join(testFolder, "manual.test_http_upgrade_websocket_plugin", "main.go")

	pluginCode, err := ioutil.ReadFile(fullPath)
	require.NoError(t, err)
	require.NotEmpty(t, pluginCode)

	tearDown, ids, errors := SetAppEnvironmentWithPlugins(t, []string{string(pluginCode)}, th.App, th.NewPluginAPI)
	defer tearDown()
	require.NoError(t, errors[0])
	require.Len(t, ids, 1)

	pluginID := ids[0]
	require.NotEmpty(t, pluginID)

	reqURL := fmt.Sprintf("ws://localhost:%d/plugins/%s", th.Server.ListenAddr.Port, pluginID)
	wsc, appErr := model.NewWebSocketClient(reqURL, "")
	require.Nil(t, appErr)
	require.NotNil(t, wsc)

	wsc.Listen()
	defer wsc.Close()

	resp := <-wsc.ResponseChannel
	require.Equal(t, resp.Status, model.STATUS_OK)

	for i := 0; i < 10; i++ {
		wsc.SendMessage("custom_action", map[string]interface{}{"value": i})
		var resp *model.WebSocketResponse
		select {
		case resp = <-wsc.ResponseChannel:
		case <-time.After(1 * time.Second):
		}
		require.NotNil(t, resp)
		require.Equal(t, resp.Status, model.STATUS_OK)
		require.Equal(t, "custom_action", resp.Data["action"])
		require.Equal(t, float64(i), resp.Data["value"])
	}
}

type MockSlashCommandProvider struct {
	Args    *model.CommandArgs
	Message string
}

func (*MockSlashCommandProvider) GetTrigger() string {
	return "mock"
}
func (*MockSlashCommandProvider) GetCommand(a *App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          "mock",
		AutoComplete:     true,
		AutoCompleteDesc: "mock",
		AutoCompleteHint: "mock",
		DisplayName:      "mock",
	}
}
func (mscp *MockSlashCommandProvider) DoCommand(a *App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	mscp.Args = args
	mscp.Message = message
	return &model.CommandResponse{
		Text:         "mock",
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
	}
}

func TestPluginExecuteSlashCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	slashCommandMock := &MockSlashCommandProvider{}
	RegisterCommandProvider(slashCommandMock)

	newUser := th.CreateUser()
	th.LinkUserToTeam(newUser, th.BasicTeam)

	t.Run("run invite command", func(t *testing.T) {
		args := &model.CommandArgs{
			Command:   "/mock @" + newUser.Username,
			TeamId:    th.BasicTeam.Id,
			UserId:    th.BasicUser.Id,
			ChannelId: th.BasicChannel.Id,
		}
		_, err := api.ExecuteSlashCommand(args)
		require.NoError(t, err)
		require.Equal(t, args, slashCommandMock.Args)
		require.Equal(t, "@"+newUser.Username, slashCommandMock.Message)
	})
}

func TestPluginAPISearchPostsInTeamByUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	basicPostText := &th.BasicPost.Message
	unknwonTerm := "Unknown Message"

	testCases := []struct {
		description      string
		teamID           string
		userID           string
		params           model.SearchParameter
		expectedPostsLen int
	}{
		{
			"empty params",
			th.BasicTeam.Id,
			th.BasicUser.Id,
			model.SearchParameter{},
			0,
		},
		{
			"doesn't match any posts",
			th.BasicTeam.Id,
			th.BasicUser.Id,
			model.SearchParameter{Terms: &unknwonTerm},
			0,
		},
		{
			"matched posts",
			th.BasicTeam.Id,
			th.BasicUser.Id,
			model.SearchParameter{Terms: basicPostText},
			1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			searchResults, err := api.SearchPostsInTeamForUser(testCase.teamID, testCase.userID, testCase.params)
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedPostsLen, len(searchResults.Posts))
		})
	}
}

func TestPluginAPICreateCommandAndListCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	foundCommand := func(listXCommand func(teamID string) ([]*model.Command, error)) bool {
		cmds, appErr := listXCommand(th.BasicTeam.Id)
		require.NoError(t, appErr)

		for _, cmd := range cmds {
			if cmd.Trigger == "testcmd" {
				return true
			}
		}
		return false
	}

	require.False(t, foundCommand(api.ListCommands))

	cmd := &model.Command{
		TeamId:  th.BasicTeam.Id,
		Trigger: "testcmd",
		Method:  "G",
		URL:     "http://test.com/testcmd",
	}

	cmd, appErr := api.CreateCommand(cmd)
	require.NoError(t, appErr)

	newCmd, appErr := api.GetCommand(cmd.Id)
	require.NoError(t, appErr)
	require.Equal(t, "pluginid", newCmd.PluginId)
	require.Equal(t, "", newCmd.CreatorId)
	require.True(t, foundCommand(api.ListCommands))
	require.True(t, foundCommand(api.ListCustomCommands))
	require.False(t, foundCommand(api.ListPluginCommands))
}

func TestPluginAPIUpdateCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	api := th.SetupPluginAPI()

	cmd := &model.Command{
		TeamId:  th.BasicTeam.Id,
		Trigger: "testcmd",
		Method:  "G",
		URL:     "http://test.com/testcmd",
	}

	cmd, appErr := api.CreateCommand(cmd)
	require.NoError(t, appErr)

	newCmd, appErr := api.GetCommand(cmd.Id)
	require.NoError(t, appErr)
	require.Equal(t, "pluginid", newCmd.PluginId)
	require.Equal(t, "", newCmd.CreatorId)

	newCmd.Trigger = "NewTrigger"
	newCmd.PluginId = "CannotChangeMe"
	newCmd2, appErr := api.UpdateCommand(newCmd.Id, newCmd)
	require.NoError(t, appErr)
	require.Equal(t, "pluginid", newCmd2.PluginId)
	require.Equal(t, "newtrigger", newCmd2.Trigger)

	team1 := th.CreateTeam()

	newCmd2.PluginId = "CannotChangeMe"
	newCmd2.Trigger = "anotherNewTrigger"
	newCmd2.TeamId = team1.Id
	newCmd3, appErr := api.UpdateCommand(newCmd2.Id, newCmd2)
	require.NoError(t, appErr)
	require.Equal(t, "pluginid", newCmd3.PluginId)
	require.Equal(t, "anothernewtrigger", newCmd3.Trigger)
	require.Equal(t, team1.Id, newCmd3.TeamId)

	newCmd3.Trigger = "anotherNewTriggerAgain"
	newCmd3.TeamId = ""
	newCmd4, appErr := api.UpdateCommand(newCmd2.Id, newCmd2)
	require.NoError(t, appErr)
	require.Equal(t, "anothernewtriggeragain", newCmd4.Trigger)
	require.Equal(t, team1.Id, newCmd4.TeamId)

}
