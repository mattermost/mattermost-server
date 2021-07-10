// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"image"
	_ "image/gif" // Registration for decoding
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func TestCreateEmoji(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	EnableCustomEmoji := *th.App.Config().ServiceSettings.EnableCustomEmoji
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = EnableCustomEmoji })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = false })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// constants to be used along with checkEmojiFile
	emojiWidth := app.MaxEmojiWidth
	emojiHeight := app.MaxEmojiHeight * 2
	// check that emoji gets resized correctly, respecting proportions, and is of expected type
	checkEmojiFile := func(id, expectedImageType string) {
		path, _ := fileutils.FindDir("data")
		file, fileErr := os.Open(filepath.Join(path, "/emoji/"+id+"/image"))
		require.NoError(t, fileErr)
		defer file.Close()
		config, imageType, err := image.DecodeConfig(file)
		require.NoError(t, err)
		require.Equal(t, expectedImageType, imageType)
		require.Equal(t, emojiWidth/2, config.Width)
		require.Equal(t, emojiHeight/2, config.Height)
	}

	emoji := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	// try to create an emoji when they're disabled
	_, resp := Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNotImplementedStatus(t, resp)

	// enable emoji creation for next cases
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	// try to create a valid gif emoji when they're enabled
	newEmoji, resp := Client.CreateEmoji(emoji, utils.CreateTestGif(t, emojiWidth, emojiHeight), "image.gif")
	CheckNoError(t, resp)
	require.Equal(t, newEmoji.Name, emoji.Name, "create with wrong name")
	checkEmojiFile(newEmoji.ID, "gif")

	// try to create an emoji with a duplicate name
	emoji2 := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      newEmoji.Name,
	}
	_, resp = Client.CreateEmoji(emoji2, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckBadRequestStatus(t, resp)
	CheckErrorMessage(t, resp, "api.emoji.create.duplicate.app_error")

	// try to create a valid animated gif emoji
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestAnimatedGif(t, emojiWidth, emojiHeight, 10), "image.gif")
	CheckNoError(t, resp)
	require.Equal(t, newEmoji.Name, emoji.Name, "create with wrong name")
	checkEmojiFile(newEmoji.ID, "gif")

	// try to create a valid jpeg emoji
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestJpeg(t, emojiWidth, emojiHeight), "image.jpeg")
	CheckNoError(t, resp)
	require.Equal(t, newEmoji.Name, emoji.Name, "create with wrong name")
	checkEmojiFile(newEmoji.ID, "png") // emoji must be converted from jpeg to png

	// try to create a valid png emoji
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestPng(t, emojiWidth, emojiHeight), "image.png")
	CheckNoError(t, resp)
	require.Equal(t, newEmoji.Name, emoji.Name, "create with wrong name")
	checkEmojiFile(newEmoji.ID, "png")

	// try to create an emoji that's too wide
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 1000, 10), "image.gif")
	CheckNoError(t, resp)
	require.Equal(t, newEmoji.Name, emoji.Name, "create with wrong name")

	// try to create an emoji that's too wide
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, app.MaxEmojiOriginalWidth+1), "image.gif")
	require.NotNil(t, resp.Error, "should fail - emoji is too wide")

	// try to create an emoji that's too tall
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, app.MaxEmojiOriginalHeight+1, 10), "image.gif")
	require.NotNil(t, resp.Error, "should fail - emoji is too tall")

	// try to create an emoji that's too large
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	_, resp = Client.CreateEmoji(emoji, utils.CreateTestAnimatedGif(t, 100, 100, 10000), "image.gif")
	require.NotNil(t, resp.Error, "should fail - emoji is too big")

	// try to create an emoji with data that isn't an image
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	_, resp = Client.CreateEmoji(emoji, make([]byte, 100), "image.gif")
	CheckBadRequestStatus(t, resp)
	CheckErrorMessage(t, resp, "api.emoji.upload.image.app_error")

	// try to create an emoji as another user
	emoji = &model.Emoji{
		CreatorID: th.BasicUser2.ID,
		Name:      model.NewID(),
	}

	_, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckForbiddenStatus(t, resp)

	// try to create an emoji without permissions
	th.RemovePermissionFromRole(model.PermissionCreateEmojis.ID, model.SystemUserRoleID)

	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	_, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckForbiddenStatus(t, resp)

	// create an emoji with permissions in one team
	th.AddPermissionToRole(model.PermissionCreateEmojis.ID, model.TeamUserRoleID)

	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	_, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)
}

func TestGetEmojiList(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	EnableCustomEmoji := *th.App.Config().ServiceSettings.EnableCustomEmoji
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = EnableCustomEmoji })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	emojis := []*model.Emoji{
		{
			CreatorID: th.BasicUser.ID,
			Name:      model.NewID(),
		},
		{
			CreatorID: th.BasicUser.ID,
			Name:      model.NewID(),
		},
		{
			CreatorID: th.BasicUser.ID,
			Name:      model.NewID(),
		},
	}

	for idx, emoji := range emojis {
		newEmoji, resp := Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
		CheckNoError(t, resp)
		emojis[idx] = newEmoji
	}

	listEmoji, resp := Client.GetEmojiList(0, 100)
	CheckNoError(t, resp)
	for _, emoji := range emojis {
		found := false
		for _, savedEmoji := range listEmoji {
			if emoji.ID == savedEmoji.ID {
				found = true
				break
			}
		}
		require.Truef(t, found, "failed to get emoji with id %v, %v", emoji.ID, len(listEmoji))
	}

	_, resp = Client.DeleteEmoji(emojis[0].ID)
	CheckNoError(t, resp)
	listEmoji, resp = Client.GetEmojiList(0, 100)
	CheckNoError(t, resp)
	found := false
	for _, savedEmoji := range listEmoji {
		if savedEmoji.ID == emojis[0].ID {
			found = true
			break
		}
	}
	require.Falsef(t, found, "should not get a deleted emoji %v", emojis[0].ID)

	listEmoji, resp = Client.GetEmojiList(0, 1)
	CheckNoError(t, resp)

	require.Len(t, listEmoji, 1, "should only return 1")

	listEmoji, resp = Client.GetSortedEmojiList(0, 100, model.EmojiSortByName)
	CheckNoError(t, resp)

	require.Greater(t, len(listEmoji), 0, "should return more than 0")
}

func TestDeleteEmoji(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	EnableCustomEmoji := *th.App.Config().ServiceSettings.EnableCustomEmoji
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = EnableCustomEmoji })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	emoji := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp := Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	ok, resp := Client.DeleteEmoji(newEmoji.ID)
	CheckNoError(t, resp)
	require.True(t, ok, "delete did not return OK")

	_, resp = Client.GetEmoji(newEmoji.ID)
	require.NotNil(t, resp, "nil response")
	require.NotNil(t, resp.Error, "expected error fetching deleted emoji")

	//Admin can delete other users emoji
	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	ok, resp = th.SystemAdminClient.DeleteEmoji(newEmoji.ID)
	CheckNoError(t, resp)
	require.True(t, ok, "delete did not return OK")

	_, resp = th.SystemAdminClient.GetEmoji(newEmoji.ID)
	require.NotNil(t, resp, "nil response")
	require.NotNil(t, resp.Error, "expected error fetching deleted emoji")

	// Try to delete just deleted emoji
	_, resp = Client.DeleteEmoji(newEmoji.ID)
	CheckNotFoundStatus(t, resp)

	//Try to delete non-existing emoji
	_, resp = Client.DeleteEmoji(model.NewID())
	CheckNotFoundStatus(t, resp)

	//Try to delete without Id
	_, resp = Client.DeleteEmoji("")
	CheckNotFoundStatus(t, resp)

	//Try to delete my custom emoji without permissions
	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	th.RemovePermissionFromRole(model.PermissionDeleteEmojis.ID, model.SystemUserRoleID)
	_, resp = Client.DeleteEmoji(newEmoji.ID)
	CheckForbiddenStatus(t, resp)
	th.AddPermissionToRole(model.PermissionDeleteEmojis.ID, model.SystemUserRoleID)

	//Try to delete other user's custom emoji without DELETE_EMOJIS permissions
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	th.RemovePermissionFromRole(model.PermissionDeleteEmojis.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionDeleteOthersEmojis.ID, model.SystemUserRoleID)

	Client.Logout()
	th.LoginBasic2()

	_, resp = Client.DeleteEmoji(newEmoji.ID)
	CheckForbiddenStatus(t, resp)

	th.RemovePermissionFromRole(model.PermissionDeleteOthersEmojis.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionDeleteEmojis.ID, model.SystemUserRoleID)

	Client.Logout()
	th.LoginBasic()

	//Try to delete other user's custom emoji without DELETE_OTHERS_EMOJIS permissions
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	Client.Logout()
	th.LoginBasic2()

	_, resp = Client.DeleteEmoji(newEmoji.ID)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	th.LoginBasic()

	//Try to delete other user's custom emoji with permissions
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	th.AddPermissionToRole(model.PermissionDeleteEmojis.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionDeleteOthersEmojis.ID, model.SystemUserRoleID)

	Client.Logout()
	th.LoginBasic2()

	_, resp = Client.DeleteEmoji(newEmoji.ID)
	CheckNoError(t, resp)

	Client.Logout()
	th.LoginBasic()

	//Try to delete my custom emoji with permissions at team level
	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	th.RemovePermissionFromRole(model.PermissionDeleteEmojis.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionDeleteEmojis.ID, model.TeamUserRoleID)
	_, resp = Client.DeleteEmoji(newEmoji.ID)
	CheckNoError(t, resp)
	th.AddPermissionToRole(model.PermissionDeleteEmojis.ID, model.SystemUserRoleID)
	th.RemovePermissionFromRole(model.PermissionDeleteEmojis.ID, model.TeamUserRoleID)

	//Try to delete other user's custom emoji with permissions at team level
	emoji = &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp = Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	th.RemovePermissionFromRole(model.PermissionDeleteEmojis.ID, model.SystemUserRoleID)
	th.RemovePermissionFromRole(model.PermissionDeleteOthersEmojis.ID, model.SystemUserRoleID)

	th.AddPermissionToRole(model.PermissionDeleteEmojis.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionDeleteOthersEmojis.ID, model.TeamUserRoleID)

	Client.Logout()
	th.LoginBasic2()

	_, resp = Client.DeleteEmoji(newEmoji.ID)
	CheckNoError(t, resp)
}

func TestGetEmoji(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	EnableCustomEmoji := *th.App.Config().ServiceSettings.EnableCustomEmoji
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = EnableCustomEmoji })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	emoji := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp := Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	emoji, resp = Client.GetEmoji(newEmoji.ID)
	CheckNoError(t, resp)
	require.Equal(t, newEmoji.ID, emoji.ID, "wrong emoji was returned")

	_, resp = Client.GetEmoji(model.NewID())
	CheckNotFoundStatus(t, resp)
}

func TestGetEmojiByName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	emoji := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	newEmoji, resp := Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	emoji, resp = Client.GetEmojiByName(newEmoji.Name)
	CheckNoError(t, resp)
	assert.Equal(t, newEmoji.Name, emoji.Name)

	_, resp = Client.GetEmojiByName(model.NewID())
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetEmojiByName(newEmoji.Name)
	CheckUnauthorizedStatus(t, resp)
}

func TestGetEmojiImage(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	emoji1 := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	emoji1, resp := Client.CreateEmoji(emoji1, utils.CreateTestGif(t, 10, 10), "image.gif")
	CheckNoError(t, resp)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = false })

	_, resp = Client.GetEmojiImage(emoji1.ID)
	CheckNotImplementedStatus(t, resp)
	CheckErrorMessage(t, resp, "api.emoji.disabled.app_error")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.DriverName = "local" })

	emojiImage, resp := Client.GetEmojiImage(emoji1.ID)
	CheckNoError(t, resp)
	require.Greater(t, len(emojiImage), 0, "should return the image")

	_, imageType, err := image.DecodeConfig(bytes.NewReader(emojiImage))
	require.NoError(t, err)
	require.Equal(t, imageType, "gif", "expected gif")

	emoji2 := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}

	emoji2, resp = Client.CreateEmoji(emoji2, utils.CreateTestAnimatedGif(t, 10, 10, 10), "image.gif")
	CheckNoError(t, resp)

	emojiImage, resp = Client.GetEmojiImage(emoji2.ID)
	CheckNoError(t, resp)
	require.Greater(t, len(emojiImage), 0, "no image returned")

	_, imageType, err = image.DecodeConfig(bytes.NewReader(emojiImage))
	require.NoError(t, err, "unable to indentify received image")
	require.Equal(t, imageType, "gif", "expected gif")

	emoji3 := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}
	emoji3, resp = Client.CreateEmoji(emoji3, utils.CreateTestJpeg(t, 10, 10), "image.jpg")
	CheckNoError(t, resp)

	emojiImage, resp = Client.GetEmojiImage(emoji3.ID)
	CheckNoError(t, resp)
	require.Greater(t, len(emojiImage), 0, "no image returned")

	_, imageType, err = image.DecodeConfig(bytes.NewReader(emojiImage))
	require.NoError(t, err, "unable to indentify received image")
	require.Equal(t, imageType, "jpeg", "expected jpeg")

	emoji4 := &model.Emoji{
		CreatorID: th.BasicUser.ID,
		Name:      model.NewID(),
	}
	emoji4, resp = Client.CreateEmoji(emoji4, utils.CreateTestPng(t, 10, 10), "image.png")
	CheckNoError(t, resp)

	emojiImage, resp = Client.GetEmojiImage(emoji4.ID)
	CheckNoError(t, resp)
	require.Greater(t, len(emojiImage), 0, "no image returned")

	_, imageType, err = image.DecodeConfig(bytes.NewReader(emojiImage))
	require.NoError(t, err, "unable to idenitify received image")
	require.Equal(t, imageType, "png", "expected png")

	_, resp = Client.DeleteEmoji(emoji4.ID)
	CheckNoError(t, resp)

	_, resp = Client.GetEmojiImage(emoji4.ID)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetEmojiImage(model.NewID())
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetEmojiImage("")
	CheckBadRequestStatus(t, resp)
}

func TestSearchEmoji(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	searchTerm1 := model.NewID()
	searchTerm2 := model.NewID()

	emojis := []*model.Emoji{
		{
			CreatorID: th.BasicUser.ID,
			Name:      searchTerm1,
		},
		{
			CreatorID: th.BasicUser.ID,
			Name:      "blargh_" + searchTerm2,
		},
	}

	for idx, emoji := range emojis {
		newEmoji, resp := Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
		CheckNoError(t, resp)
		emojis[idx] = newEmoji
	}

	search := &model.EmojiSearch{Term: searchTerm1}
	remojis, resp := Client.SearchEmoji(search)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	found := false
	for _, e := range remojis {
		if e.Name == emojis[0].Name {
			found = true
		}
	}

	assert.True(t, found)

	search.Term = searchTerm2
	search.PrefixOnly = true
	remojis, resp = Client.SearchEmoji(search)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	found = false
	for _, e := range remojis {
		if e.Name == emojis[1].Name {
			found = true
		}
	}

	assert.False(t, found)

	search.PrefixOnly = false
	remojis, resp = Client.SearchEmoji(search)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	found = false
	for _, e := range remojis {
		if e.Name == emojis[1].Name {
			found = true
		}
	}

	assert.True(t, found)

	search.Term = ""
	_, resp = Client.SearchEmoji(search)
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.SearchEmoji(search)
	CheckUnauthorizedStatus(t, resp)
}

func TestAutocompleteEmoji(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	searchTerm1 := model.NewID()

	emojis := []*model.Emoji{
		{
			CreatorID: th.BasicUser.ID,
			Name:      searchTerm1,
		},
		{
			CreatorID: th.BasicUser.ID,
			Name:      "blargh_" + searchTerm1,
		},
	}

	for idx, emoji := range emojis {
		newEmoji, resp := Client.CreateEmoji(emoji, utils.CreateTestGif(t, 10, 10), "image.gif")
		CheckNoError(t, resp)
		emojis[idx] = newEmoji
	}

	remojis, resp := Client.AutocompleteEmoji(searchTerm1, "")
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	found1 := false
	found2 := false
	for _, e := range remojis {
		if e.Name == emojis[0].Name {
			found1 = true
		}

		if e.Name == emojis[1].Name {
			found2 = true
		}
	}

	assert.True(t, found1)
	assert.False(t, found2)

	_, resp = Client.AutocompleteEmoji("", "")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.AutocompleteEmoji(searchTerm1, "")
	CheckUnauthorizedStatus(t, resp)
}
