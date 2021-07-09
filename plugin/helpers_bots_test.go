// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func TestEnsureBot(t *testing.T) {
	setupAPI := func() *plugintest.API {
		return &plugintest.API{}
	}

	testbot := &model.Bot{
		Username:    "testbot",
		DisplayName: "Test Bot",
		Description: "testbotdescription",
	}

	t.Run("server version incompatible", func(t *testing.T) {
		api := setupAPI()
		api.On("GetServerVersion").Return("5.9.0")
		defer api.AssertExpectations(t)

		p := &plugin.HelpersImpl{}
		p.API = api

		_, retErr := p.EnsureBot(nil)

		assert.Error(t, retErr)
		assert.Equal(t, "failed to ensure bot: incompatible server version for plugin, minimum required version: 5.10.0, current version: 5.9.0", retErr.Error())
	})

	t.Run("bad parameters", func(t *testing.T) {
		t.Run("no bot", func(t *testing.T) {
			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")

			p := &plugin.HelpersImpl{}
			p.API = api
			botID, err := p.EnsureBot(nil)
			assert.Equal(t, "", botID)
			assert.Error(t, err)
		})
		t.Run("bad username", func(t *testing.T) {
			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")

			p := &plugin.HelpersImpl{}
			p.API = api
			botID, err := p.EnsureBot(&model.Bot{
				Username: "",
			})
			assert.Equal(t, "", botID)
			assert.Error(t, err)
		})
	})

	t.Run("if bot already exists", func(t *testing.T) {
		t.Run("should find and return the existing bot ID", func(t *testing.T) {
			expectedBotID := model.NewID()

			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")
			api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
			api.On("PatchBot", expectedBotID, &model.BotPatch{
				Username:    &testbot.Username,
				DisplayName: &testbot.DisplayName,
				Description: &testbot.Description,
			}).Return(nil, nil)
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot)

			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should return an error if unable to get bot", func(t *testing.T) {
			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")
			api.On("KVGet", plugin.BotUserKey).Return(nil, &model.AppError{})
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot)

			assert.Equal(t, "", botID)
			assert.Error(t, err)
		})

		t.Run("should set the bot profile image when specified", func(t *testing.T) {
			expectedBotID := model.NewID()
			api := setupAPI()

			testsDir, _ := fileutils.FindDir("tests")
			testImage := filepath.Join(testsDir, "test.png")
			imageBytes, err := ioutil.ReadFile(testImage)

			api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
			api.On("GetBundlePath").Return("", nil)
			api.On("SetProfileImage", expectedBotID, imageBytes).Return(nil)
			api.On("GetServerVersion").Return("5.10.0")
			api.On("PatchBot", expectedBotID, &model.BotPatch{
				Username:    &testbot.Username,
				DisplayName: &testbot.DisplayName,
				Description: &testbot.Description,
			}).Return(nil, nil)
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			assert.NoError(t, err)

			botID, err := p.EnsureBot(testbot, plugin.ProfileImagePath(testImage))
			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should set the bot icon image when specified", func(t *testing.T) {
			expectedBotID := model.NewID()
			api := setupAPI()

			testsDir, _ := fileutils.FindDir("tests")
			testImage := filepath.Join(testsDir, "test.png")
			imageBytes, err := ioutil.ReadFile(testImage)
			assert.NoError(t, err)

			api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
			api.On("GetBundlePath").Return("", nil)
			api.On("SetBotIconImage", expectedBotID, imageBytes).Return(nil)
			api.On("GetServerVersion").Return("5.10.0")
			api.On("PatchBot", expectedBotID, &model.BotPatch{
				Username:    &testbot.Username,
				DisplayName: &testbot.DisplayName,
				Description: &testbot.Description,
			}).Return(nil, nil)
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot, plugin.IconImagePath(testImage))
			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should set both the profile image and bot icon image when specified", func(t *testing.T) {
			expectedBotID := model.NewID()
			api := setupAPI()

			testsDir, _ := fileutils.FindDir("tests")
			testImage := filepath.Join(testsDir, "test.png")
			imageBytes, err := ioutil.ReadFile(testImage)
			assert.NoError(t, err)

			api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
			api.On("GetBundlePath").Return("", nil)
			api.On("SetProfileImage", expectedBotID, imageBytes).Return(nil)
			api.On("SetBotIconImage", expectedBotID, imageBytes).Return(nil)
			api.On("GetServerVersion").Return("5.10.0")
			api.On("PatchBot", expectedBotID, &model.BotPatch{
				Username:    &testbot.Username,
				DisplayName: &testbot.DisplayName,
				Description: &testbot.Description,
			}).Return(nil, nil)
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot, plugin.ProfileImagePath(testImage), plugin.IconImagePath(testImage))
			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should find and update the bot with new bot details", func(t *testing.T) {
			expectedBotID := model.NewID()

			expectedBotUsername := "updated_testbot"
			expectedBotDisplayName := "Updated Test Bot"
			expectedBotDescription := "updated testbotdescription"

			testsDir, _ := fileutils.FindDir("tests")
			testImage := filepath.Join(testsDir, "test.png")
			imageBytes, err := ioutil.ReadFile(testImage)
			assert.NoError(t, err)

			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")
			api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
			api.On("GetBundlePath").Return("", nil)
			api.On("SetProfileImage", expectedBotID, imageBytes).Return(nil)
			api.On("SetBotIconImage", expectedBotID, imageBytes).Return(nil)
			api.On("PatchBot", expectedBotID, &model.BotPatch{
				Username:    &expectedBotUsername,
				DisplayName: &expectedBotDisplayName,
				Description: &expectedBotDescription,
			}).Return(nil, nil)
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			updatedTestbot := &model.Bot{
				Username:    "updated_testbot",
				DisplayName: "Updated Test Bot",
				Description: "updated testbotdescription",
			}
			botID, err := p.EnsureBot(updatedTestbot, plugin.ProfileImagePath(testImage), plugin.IconImagePath(testImage))

			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})
	})

	t.Run("if bot doesn't exist", func(t *testing.T) {
		t.Run("should create the bot and return the ID", func(t *testing.T) {
			expectedBotID := model.NewID()

			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")
			api.On("KVGet", plugin.BotUserKey).Return(nil, nil)
			api.On("GetUserByUsername", testbot.Username).Return(nil, nil)
			api.On("CreateBot", testbot).Return(&model.Bot{
				UserID: expectedBotID,
			}, nil)
			api.On("KVSet", plugin.BotUserKey, []byte(expectedBotID)).Return(nil)
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot)

			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should claim existing bot and return the ID", func(t *testing.T) {
			expectedBotID := model.NewID()

			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")
			api.On("KVGet", plugin.BotUserKey).Return(nil, nil)
			api.On("GetUserByUsername", testbot.Username).Return(&model.User{
				ID:    expectedBotID,
				IsBot: true,
			}, nil)
			api.On("KVSet", plugin.BotUserKey, []byte(expectedBotID)).Return(nil)
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot)

			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should return the non-bot account but log a message if user exists with the same name and is not a bot", func(t *testing.T) {
			expectedBotID := model.NewID()
			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")
			api.On("KVGet", plugin.BotUserKey).Return(nil, nil)
			api.On("GetUserByUsername", testbot.Username).Return(&model.User{
				ID:    expectedBotID,
				IsBot: false,
			}, nil)
			api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot)

			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should fail if create bot fails", func(t *testing.T) {
			api := setupAPI()
			api.On("GetServerVersion").Return("5.10.0")
			api.On("KVGet", plugin.BotUserKey).Return(nil, nil)
			api.On("GetUserByUsername", testbot.Username).Return(nil, nil)
			api.On("CreateBot", testbot).Return(nil, &model.AppError{})
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot)

			assert.Equal(t, "", botID)
			assert.Error(t, err)
		})

		t.Run("should create bot and set the bot profile image when specified", func(t *testing.T) {
			expectedBotID := model.NewID()
			api := setupAPI()

			testsDir, _ := fileutils.FindDir("tests")
			testImage := filepath.Join(testsDir, "test.png")
			imageBytes, err := ioutil.ReadFile(testImage)
			assert.NoError(t, err)

			api.On("KVGet", plugin.BotUserKey).Return(nil, nil)
			api.On("GetUserByUsername", testbot.Username).Return(nil, nil)
			api.On("CreateBot", testbot).Return(&model.Bot{
				UserID: expectedBotID,
			}, nil)
			api.On("KVSet", plugin.BotUserKey, []byte(expectedBotID)).Return(nil)
			api.On("GetBundlePath").Return("", nil)
			api.On("SetProfileImage", expectedBotID, imageBytes).Return(nil)
			api.On("GetServerVersion").Return("5.10.0")
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot, plugin.ProfileImagePath(testImage))
			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should create bot and set the bot icon image when specified", func(t *testing.T) {
			expectedBotID := model.NewID()
			api := setupAPI()

			testsDir, _ := fileutils.FindDir("tests")
			testImage := filepath.Join(testsDir, "test.png")
			imageBytes, err := ioutil.ReadFile(testImage)
			assert.NoError(t, err)

			api.On("KVGet", plugin.BotUserKey).Return(nil, nil)
			api.On("GetUserByUsername", testbot.Username).Return(nil, nil)
			api.On("CreateBot", testbot).Return(&model.Bot{
				UserID: expectedBotID,
			}, nil)
			api.On("KVSet", plugin.BotUserKey, []byte(expectedBotID)).Return(nil)
			api.On("GetBundlePath").Return("", nil)
			api.On("SetBotIconImage", expectedBotID, imageBytes).Return(nil)
			api.On("GetServerVersion").Return("5.10.0")
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot, plugin.IconImagePath(testImage))
			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})

		t.Run("should create bot and set both the profile image and bot icon image when specified", func(t *testing.T) {
			expectedBotID := model.NewID()
			api := setupAPI()

			testsDir, _ := fileutils.FindDir("tests")
			testImage := filepath.Join(testsDir, "test.png")
			imageBytes, err := ioutil.ReadFile(testImage)
			assert.NoError(t, err)

			api.On("KVGet", plugin.BotUserKey).Return(nil, nil)
			api.On("GetUserByUsername", testbot.Username).Return(nil, nil)
			api.On("CreateBot", testbot).Return(&model.Bot{
				UserID: expectedBotID,
			}, nil)
			api.On("KVSet", plugin.BotUserKey, []byte(expectedBotID)).Return(nil)
			api.On("GetBundlePath").Return("", nil)
			api.On("SetProfileImage", expectedBotID, imageBytes).Return(nil)
			api.On("SetBotIconImage", expectedBotID, imageBytes).Return(nil)
			api.On("GetServerVersion").Return("5.10.0")
			defer api.AssertExpectations(t)

			p := &plugin.HelpersImpl{}
			p.API = api

			botID, err := p.EnsureBot(testbot, plugin.ProfileImagePath(testImage), plugin.IconImagePath(testImage))
			assert.Equal(t, expectedBotID, botID)
			assert.NoError(t, err)
		})
	})
}

func TestShouldProcessMessage(t *testing.T) {
	p := &plugin.HelpersImpl{}
	expectedBotID := model.NewID()

	setupAPI := func() *plugintest.API {
		return &plugintest.API{}
	}

	t.Run("should not respond to itself", func(t *testing.T) {
		api := setupAPI()
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
		p.API = api
		shouldProcessMessage, _ := p.ShouldProcessMessage(&model.Post{Type: model.PostTypeHeaderChange, UserID: expectedBotID}, plugin.AllowSystemMessages(), plugin.AllowBots())

		assert.False(t, shouldProcessMessage)
	})

	t.Run("should not process as the post is generated by system", func(t *testing.T) {
		shouldProcessMessage, _ := p.ShouldProcessMessage(&model.Post{Type: model.PostTypeHeaderChange})

		assert.False(t, shouldProcessMessage)
	})

	t.Run("should not process as the post is sent to another channel", func(t *testing.T) {
		channelID := "channel-id"
		api := setupAPI()
		api.On("GetChannel", channelID).Return(&model.Channel{ID: channelID, Type: model.ChannelTypeGroup}, nil)
		p.API = api
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)

		shouldProcessMessage, _ := p.ShouldProcessMessage(&model.Post{ChannelID: channelID}, plugin.AllowSystemMessages(), plugin.AllowBots(), plugin.FilterChannelIDs([]string{"another-channel-id"}))

		assert.False(t, shouldProcessMessage)
	})

	t.Run("should not process as the post is created by bot", func(t *testing.T) {
		userID := "user-id"
		channelID := "1"
		api := setupAPI()
		p.API = api
		api.On("GetUser", userID).Return(&model.User{IsBot: true}, nil)
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)

		shouldProcessMessage, _ := p.ShouldProcessMessage(&model.Post{UserID: userID, ChannelID: channelID},
			plugin.AllowSystemMessages(), plugin.FilterUserIDs([]string{"another-user-id"}))

		assert.False(t, shouldProcessMessage)
	})

	t.Run("should not process the message as the post is not in bot dm channel", func(t *testing.T) {
		userID := "user-id"
		channelID := "1"
		channel := model.Channel{
			Name: "user1__" + expectedBotID,
			Type: model.ChannelTypeOpen,
		}
		api := setupAPI()
		api.On("GetChannel", channelID).Return(&channel, nil)
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
		p.API = api

		shouldProcessMessage, _ := p.ShouldProcessMessage(&model.Post{UserID: userID, ChannelID: channelID}, plugin.AllowSystemMessages(), plugin.AllowBots(), plugin.OnlyBotDMs())

		assert.False(t, shouldProcessMessage)
	})

	t.Run("should process the message", func(t *testing.T) {
		channelID := "1"
		api := setupAPI()
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
		p.API = api

		shouldProcessMessage, _ := p.ShouldProcessMessage(&model.Post{UserID: "1", Type: model.PostTypeHeaderChange, ChannelID: channelID},
			plugin.AllowSystemMessages(), plugin.FilterChannelIDs([]string{channelID}), plugin.AllowBots(), plugin.FilterUserIDs([]string{"1"}))

		assert.True(t, shouldProcessMessage)
	})

	t.Run("should process the message for plugin without a bot", func(t *testing.T) {
		channelID := "1"
		api := setupAPI()
		api.On("KVGet", plugin.BotUserKey).Return(nil, nil)
		p.API = api

		shouldProcessMessage, _ := p.ShouldProcessMessage(&model.Post{UserID: "1", Type: model.PostTypeHeaderChange, ChannelID: channelID},
			plugin.AllowSystemMessages(), plugin.FilterChannelIDs([]string{channelID}), plugin.AllowBots(), plugin.FilterUserIDs([]string{"1"}))

		assert.True(t, shouldProcessMessage)
	})

	t.Run("should process the message when filter channel and filter users list is empty", func(t *testing.T) {
		channelID := "1"
		api := setupAPI()
		channel := model.Channel{
			Name: "user1__" + expectedBotID,
			Type: model.ChannelTypeDirect,
		}
		api.On("GetChannel", channelID).Return(&channel, nil)
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)
		p.API = api

		shouldProcessMessage, _ := p.ShouldProcessMessage(&model.Post{UserID: "1", Type: model.PostTypeHeaderChange, ChannelID: channelID},
			plugin.AllowSystemMessages(), plugin.AllowBots())

		assert.True(t, shouldProcessMessage)
	})

	t.Run("should not process the message which have from_webhook", func(t *testing.T) {
		channelID := "1"
		api := setupAPI()
		api.On("GetChannel", channelID).Return(&model.Channel{ID: channelID, Type: model.ChannelTypeGroup}, nil)
		p.API = api
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)

		shouldProcessMessage, err := p.ShouldProcessMessage(&model.Post{ChannelID: channelID, Props: model.StringInterface{"from_webhook": "true"}}, plugin.AllowBots())

		assert.False(t, shouldProcessMessage)
		assert.NoError(t, err)
	})

	t.Run("should process the message which have from_webhook with allow webhook plugin", func(t *testing.T) {
		channelID := "1"
		api := setupAPI()
		api.On("GetChannel", channelID).Return(&model.Channel{ID: channelID, Type: model.ChannelTypeGroup}, nil)
		p.API = api
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)

		shouldProcessMessage, err := p.ShouldProcessMessage(&model.Post{ChannelID: channelID, Props: model.StringInterface{"from_webhook": "true"}}, plugin.AllowBots(), plugin.AllowWebhook())
		assert.NoError(t, err)

		assert.True(t, shouldProcessMessage)
	})

	t.Run("should process the message where from_webhook is not set", func(t *testing.T) {
		channelID := "1"
		api := setupAPI()
		api.On("GetChannel", channelID).Return(&model.Channel{ID: channelID, Type: model.ChannelTypeGroup}, nil)
		p.API = api
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)

		shouldProcessMessage, err := p.ShouldProcessMessage(&model.Post{ChannelID: channelID}, plugin.AllowBots())
		assert.NoError(t, err)

		assert.True(t, shouldProcessMessage)
	})

	t.Run("should process the message which have from_webhook false", func(t *testing.T) {
		channelID := "1"
		api := setupAPI()
		api.On("GetChannel", channelID).Return(&model.Channel{ID: channelID, Type: model.ChannelTypeGroup}, nil)
		p.API = api
		api.On("KVGet", plugin.BotUserKey).Return([]byte(expectedBotID), nil)

		shouldProcessMessage, err := p.ShouldProcessMessage(&model.Post{ChannelID: channelID, Props: model.StringInterface{"from_webhook": "false"}}, plugin.AllowBots())
		assert.NoError(t, err)

		assert.True(t, shouldProcessMessage)
	})

	t.Run("should process the message when we pass the botId as input", func(t *testing.T) {
		userID := "user-id"
		channelID := "1"
		api := setupAPI()
		api.On("GetChannel", channelID).Return(&model.Channel{ID: channelID, Type: model.ChannelTypeGroup}, nil)
		p.API = api
		api.On("GetUser", userID).Return(&model.User{IsBot: false}, nil)

		// we should skip the store Get
		api.On("KVGet", plugin.BotUserKey).Return(nil, nil)

		shouldProcessMessage, err := p.ShouldProcessMessage(&model.Post{ChannelID: channelID, UserID: userID}, plugin.BotID(expectedBotID))
		assert.NoError(t, err)

		assert.True(t, shouldProcessMessage)
	})
}
