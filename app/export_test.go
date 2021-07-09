// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func TestReactionsOfPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	post := th.BasicPost
	post.HasReactions = true
	th.BasicUser2.DeleteAt = 1234
	reactionObject := model.Reaction{
		UserID:    th.BasicUser.ID,
		PostID:    post.ID,
		EmojiName: "emoji",
		CreateAt:  model.GetMillis(),
	}
	reactionObjectDeleted := model.Reaction{
		UserID:    th.BasicUser2.ID,
		PostID:    post.ID,
		EmojiName: "emoji",
		CreateAt:  model.GetMillis(),
	}

	th.App.SaveReactionForPost(th.Context, &reactionObject)
	th.App.SaveReactionForPost(th.Context, &reactionObjectDeleted)
	reactionsOfPost, err := th.App.BuildPostReactions(post.ID)
	require.Nil(t, err)

	assert.Equal(t, reactionObject.EmojiName, *(*reactionsOfPost)[0].EmojiName)
}

func TestExportUserNotifyProps(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	userNotifyProps := model.StringMap{
		model.DesktopNotifyProp:         model.UserNotifyAll,
		model.DesktopSoundNotifyProp:    "true",
		model.EmailNotifyProp:           "true",
		model.PushNotifyProp:            model.UserNotifyAll,
		model.PushStatusNotifyProp:      model.StatusOnline,
		model.ChannelMentionsNotifyProp: "true",
		model.CommentsNotifyProp:        model.CommentsNotifyRoot,
		model.MentionKeysNotifyProp:     "valid,misc",
	}

	exportNotifyProps := th.App.buildUserNotifyProps(userNotifyProps)

	require.Equal(t, userNotifyProps[model.DesktopNotifyProp], *exportNotifyProps.Desktop)
	require.Equal(t, userNotifyProps[model.DesktopSoundNotifyProp], *exportNotifyProps.DesktopSound)
	require.Equal(t, userNotifyProps[model.EmailNotifyProp], *exportNotifyProps.Email)
	require.Equal(t, userNotifyProps[model.PushNotifyProp], *exportNotifyProps.Mobile)
	require.Equal(t, userNotifyProps[model.PushStatusNotifyProp], *exportNotifyProps.MobilePushStatus)
	require.Equal(t, userNotifyProps[model.ChannelMentionsNotifyProp], *exportNotifyProps.ChannelTrigger)
	require.Equal(t, userNotifyProps[model.CommentsNotifyProp], *exportNotifyProps.CommentsTrigger)
	require.Equal(t, userNotifyProps[model.MentionKeysNotifyProp], *exportNotifyProps.MentionKeys)
}

func TestExportUserChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	channel := th.BasicChannel
	user := th.BasicUser
	team := th.BasicTeam
	channelName := channel.Name
	notifyProps := model.StringMap{
		model.DesktopNotifyProp: model.UserNotifyAll,
		model.PushNotifyProp:    model.UserNotifyNone,
	}
	preference := model.Preference{
		UserID:   user.ID,
		Category: model.PreferenceCategoryFavoriteChannel,
		Name:     channel.ID,
		Value:    "true",
	}
	var preferences model.Preferences
	preferences = append(preferences, preference)
	err := th.App.Srv().Store.Preference().Save(&preferences)
	require.NoError(t, err)

	th.App.UpdateChannelMemberNotifyProps(notifyProps, channel.ID, user.ID)
	exportData, appErr := th.App.buildUserChannelMemberships(user.ID, team.ID)
	require.Nil(t, appErr)
	assert.Equal(t, len(*exportData), 3)
	for _, data := range *exportData {
		if *data.Name == channelName {
			assert.Equal(t, *data.NotifyProps.Desktop, "all")
			assert.Equal(t, *data.NotifyProps.Mobile, "none")
			assert.Equal(t, *data.NotifyProps.MarkUnread, "all") // default value
			assert.True(t, *data.Favorite)
		} else { // default values
			assert.Equal(t, *data.NotifyProps.Desktop, "default")
			assert.Equal(t, *data.NotifyProps.Mobile, "default")
			assert.Equal(t, *data.NotifyProps.MarkUnread, "all")
			assert.False(t, *data.Favorite)
		}
	}
}

func TestCopyEmojiImages(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	emoji := &model.Emoji{
		ID: model.NewID(),
	}

	// Creating a dir named `exported_emoji_test` in the root of the repo
	pathToDir := "../exported_emoji_test"

	os.Mkdir(pathToDir, 0777)
	defer os.RemoveAll(pathToDir)

	filePath := "../data/emoji/" + emoji.ID
	emojiImagePath := filePath + "/image"

	var _, err = os.Stat(filePath)
	if os.IsNotExist(err) {
		os.MkdirAll(filePath, 0777)
	}

	// Creating a file with the name `image` to copy it to `exported_emoji_test`
	os.OpenFile(filePath+"/image", os.O_RDONLY|os.O_CREATE, 0777)
	defer os.RemoveAll(filePath)

	copyError := th.App.copyEmojiImages(emoji.ID, emojiImagePath, pathToDir)
	require.NoError(t, copyError)

	_, err = os.Stat(pathToDir + "/" + emoji.ID + "/image")
	require.False(t, os.IsNotExist(err), "File should exist ")
}

func TestExportCustomEmoji(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	filePath := "../demo.json"

	fileWriter, err := os.Create(filePath)
	require.NoError(t, err)
	defer os.Remove(filePath)

	dirNameToExportEmoji := "exported_emoji_test"
	defer os.RemoveAll("../" + dirNameToExportEmoji)

	outPath, err := filepath.Abs(filePath)
	require.NoError(t, err)

	_, appErr := th.App.exportCustomEmoji(fileWriter, outPath, dirNameToExportEmoji, false)
	require.Nil(t, appErr, "should not have failed")
}

func TestExportAllUsers(t *testing.T) {
	th1 := Setup(t)
	defer th1.TearDown()

	// Adding a user and deactivating it to check whether it gets included in bulk export
	user := th1.CreateUser()
	_, err := th1.App.UpdateActive(th1.Context, user, false)
	require.Nil(t, err)

	var b bytes.Buffer
	err = th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
	require.Nil(t, err)

	th2 := Setup(t)
	defer th2.TearDown()
	err, i := th2.App.BulkImport(th2.Context, &b, nil, false, 5)
	assert.Nil(t, err)
	assert.Equal(t, 0, i)

	users1, err := th1.App.GetUsers(&model.UserGetOptions{
		Page:    0,
		PerPage: 10,
	})
	assert.Nil(t, err)
	users2, err := th2.App.GetUsers(&model.UserGetOptions{
		Page:    0,
		PerPage: 10,
	})
	assert.Nil(t, err)
	assert.Equal(t, len(users1), len(users2))
	assert.ElementsMatch(t, users1, users2)

	// Checking whether deactivated users were included in bulk export
	deletedUsers1, err := th1.App.GetUsers(&model.UserGetOptions{
		Inactive: true,
		Page:     0,
		PerPage:  10,
	})
	assert.Nil(t, err)
	deletedUsers2, err := th1.App.GetUsers(&model.UserGetOptions{
		Inactive: true,
		Page:     0,
		PerPage:  10,
	})
	assert.Nil(t, err)
	assert.Equal(t, len(deletedUsers1), len(deletedUsers2))
	assert.ElementsMatch(t, deletedUsers1, deletedUsers2)
}

func TestExportDMChannel(t *testing.T) {
	t.Run("Export a DM channel to another server", func(t *testing.T) {
		th1 := Setup(t).InitBasic()
		defer th1.TearDown()

		// DM Channel
		th1.CreateDmChannel(th1.BasicUser2)

		var b bytes.Buffer
		err := th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
		require.Nil(t, err)

		channels, nErr := th1.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
		require.NoError(t, nErr)
		assert.Equal(t, 1, len(channels))

		th2 := Setup(t).InitBasic()
		defer th2.TearDown()

		channels, nErr = th2.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
		require.NoError(t, nErr)
		assert.Equal(t, 0, len(channels))

		// import the exported channel
		err, i := th2.App.BulkImport(th2.Context, &b, nil, false, 5)
		require.Nil(t, err)
		assert.Equal(t, 0, i)

		// Ensure the Members of the imported DM channel is the same was from the exported
		channels, nErr = th2.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
		require.NoError(t, nErr)
		require.Equal(t, 1, len(channels))
		assert.ElementsMatch(t, []string{th1.BasicUser.Username, th1.BasicUser2.Username}, *channels[0].Members)
	})

	t.Run("Invalid DM channel export", func(t *testing.T) {
		th1 := Setup(t).InitBasic()
		defer th1.TearDown()

		// DM Channel
		th1.CreateDmChannel(th1.BasicUser2)

		channels, nErr := th1.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
		require.NoError(t, nErr)
		assert.Equal(t, 1, len(channels))

		th1.App.PermanentDeleteUser(th1.Context, th1.BasicUser2)
		th1.App.PermanentDeleteUser(th1.Context, th1.BasicUser)

		var b bytes.Buffer
		err := th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
		require.Nil(t, err)

		th2 := Setup(t).InitBasic()
		defer th2.TearDown()

		// import the exported channel
		err, _ = th2.App.BulkImport(th2.Context, &b, nil, true, 5)
		require.Nil(t, err)

		channels, nErr = th2.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
		require.NoError(t, nErr)
		assert.Empty(t, channels)
	})
}

func TestExportDMChannelToSelf(t *testing.T) {
	th1 := Setup(t).InitBasic()
	defer th1.TearDown()

	// DM Channel with self (me channel)
	th1.CreateDmChannel(th1.BasicUser)

	var b bytes.Buffer
	err := th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
	require.Nil(t, err)

	channels, nErr := th1.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
	require.NoError(t, nErr)
	assert.Equal(t, 1, len(channels))

	th2 := Setup(t)
	defer th2.TearDown()

	channels, nErr = th2.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
	require.NoError(t, nErr)
	assert.Equal(t, 0, len(channels))

	// import the exported channel
	err, i := th2.App.BulkImport(th2.Context, &b, nil, false, 5)
	assert.Nil(t, err)
	assert.Equal(t, 0, i)

	channels, nErr = th2.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
	require.NoError(t, nErr)
	assert.Equal(t, 1, len(channels))
	assert.Equal(t, 1, len((*channels[0].Members)))
	assert.Equal(t, th1.BasicUser.Username, (*channels[0].Members)[0])
}

func TestExportGMChannel(t *testing.T) {
	th1 := Setup(t).InitBasic()

	user1 := th1.CreateUser()
	th1.LinkUserToTeam(user1, th1.BasicTeam)
	user2 := th1.CreateUser()
	th1.LinkUserToTeam(user2, th1.BasicTeam)

	// GM Channel
	th1.CreateGroupChannel(user1, user2)

	var b bytes.Buffer
	err := th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
	require.Nil(t, err)

	channels, nErr := th1.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
	require.NoError(t, nErr)
	assert.Equal(t, 1, len(channels))

	th1.TearDown()

	th2 := Setup(t)
	defer th2.TearDown()

	channels, nErr = th2.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
	require.NoError(t, nErr)
	assert.Equal(t, 0, len(channels))
}

func TestExportGMandDMChannels(t *testing.T) {
	th1 := Setup(t).InitBasic()

	// DM Channel
	th1.CreateDmChannel(th1.BasicUser2)

	user1 := th1.CreateUser()
	th1.LinkUserToTeam(user1, th1.BasicTeam)
	user2 := th1.CreateUser()
	th1.LinkUserToTeam(user2, th1.BasicTeam)

	// GM Channel
	th1.CreateGroupChannel(user1, user2)

	var b bytes.Buffer
	err := th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
	require.Nil(t, err)

	channels, nErr := th1.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
	require.NoError(t, nErr)
	assert.Equal(t, 2, len(channels))

	th1.TearDown()

	th2 := Setup(t)
	defer th2.TearDown()

	channels, nErr = th2.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
	require.NoError(t, nErr)
	assert.Equal(t, 0, len(channels))

	// import the exported channel
	err, i := th2.App.BulkImport(th2.Context, &b, nil, false, 5)
	assert.Nil(t, err)
	assert.Equal(t, 0, i)

	// Ensure the Members of the imported GM channel is the same was from the exported
	channels, nErr = th2.App.Srv().Store.Channel().GetAllDirectChannelsForExportAfter(1000, "00000000")
	require.NoError(t, nErr)

	// Adding some deteminism so its possible to assert on slice index
	sort.Slice(channels, func(i, j int) bool { return channels[i].Type > channels[j].Type })
	assert.Equal(t, 2, len(channels))
	assert.ElementsMatch(t, []string{th1.BasicUser.Username, user1.Username, user2.Username}, *channels[0].Members)
	assert.ElementsMatch(t, []string{th1.BasicUser.Username, th1.BasicUser2.Username}, *channels[1].Members)
}

func TestExportDMandGMPost(t *testing.T) {
	th1 := Setup(t).InitBasic()

	// DM Channel
	dmChannel := th1.CreateDmChannel(th1.BasicUser2)
	dmMembers := []string{th1.BasicUser.Username, th1.BasicUser2.Username}

	user1 := th1.CreateUser()
	th1.LinkUserToTeam(user1, th1.BasicTeam)
	user2 := th1.CreateUser()
	th1.LinkUserToTeam(user2, th1.BasicTeam)

	// GM Channel
	gmChannel := th1.CreateGroupChannel(user1, user2)
	gmMembers := []string{th1.BasicUser.Username, user1.Username, user2.Username}

	// DM posts
	p1 := &model.Post{
		ChannelID: dmChannel.ID,
		Message:   "aa" + model.NewID() + "a",
		UserID:    th1.BasicUser.ID,
	}
	th1.App.CreatePost(th1.Context, p1, dmChannel, false, true)

	p2 := &model.Post{
		ChannelID: dmChannel.ID,
		Message:   "bb" + model.NewID() + "a",
		UserID:    th1.BasicUser.ID,
	}
	th1.App.CreatePost(th1.Context, p2, dmChannel, false, true)

	// GM posts
	p3 := &model.Post{
		ChannelID: gmChannel.ID,
		Message:   "cc" + model.NewID() + "a",
		UserID:    th1.BasicUser.ID,
	}
	th1.App.CreatePost(th1.Context, p3, gmChannel, false, true)

	p4 := &model.Post{
		ChannelID: gmChannel.ID,
		Message:   "dd" + model.NewID() + "a",
		UserID:    th1.BasicUser.ID,
	}
	th1.App.CreatePost(th1.Context, p4, gmChannel, false, true)

	posts, err := th1.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, err)
	assert.Equal(t, 4, len(posts))

	var b bytes.Buffer
	appErr := th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
	require.Nil(t, appErr)

	th1.TearDown()

	th2 := Setup(t)
	defer th2.TearDown()

	posts, err = th2.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, err)
	assert.Equal(t, 0, len(posts))

	// import the exported posts
	appErr, i := th2.App.BulkImport(th2.Context, &b, nil, false, 5)
	assert.Nil(t, appErr)
	assert.Equal(t, 0, i)

	posts, err = th2.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, err)

	// Adding some deteminism so its possible to assert on slice index
	sort.Slice(posts, func(i, j int) bool { return posts[i].Message > posts[j].Message })
	assert.Equal(t, 4, len(posts))
	assert.ElementsMatch(t, gmMembers, *posts[0].ChannelMembers)
	assert.ElementsMatch(t, gmMembers, *posts[1].ChannelMembers)
	assert.ElementsMatch(t, dmMembers, *posts[2].ChannelMembers)
	assert.ElementsMatch(t, dmMembers, *posts[3].ChannelMembers)
}

func TestExportPostWithProps(t *testing.T) {
	th1 := Setup(t).InitBasic()

	attachments := []*model.SlackAttachment{{Footer: "footer"}}

	// DM Channel
	dmChannel := th1.CreateDmChannel(th1.BasicUser2)
	dmMembers := []string{th1.BasicUser.Username, th1.BasicUser2.Username}

	user1 := th1.CreateUser()
	th1.LinkUserToTeam(user1, th1.BasicTeam)
	user2 := th1.CreateUser()
	th1.LinkUserToTeam(user2, th1.BasicTeam)

	// GM Channel
	gmChannel := th1.CreateGroupChannel(user1, user2)
	gmMembers := []string{th1.BasicUser.Username, user1.Username, user2.Username}

	// DM posts
	p1 := &model.Post{
		ChannelID: dmChannel.ID,
		Message:   "aa" + model.NewID() + "a",
		Props: map[string]interface{}{
			"attachments": attachments,
		},
		UserID: th1.BasicUser.ID,
	}
	th1.App.CreatePost(th1.Context, p1, dmChannel, false, true)

	p2 := &model.Post{
		ChannelID: gmChannel.ID,
		Message:   "dd" + model.NewID() + "a",
		Props: map[string]interface{}{
			"attachments": attachments,
		},
		UserID: th1.BasicUser.ID,
	}
	th1.App.CreatePost(th1.Context, p2, gmChannel, false, true)

	posts, err := th1.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, err)
	assert.Len(t, posts, 2)
	require.NotEmpty(t, posts[0].Props)
	require.NotEmpty(t, posts[1].Props)

	var b bytes.Buffer
	appErr := th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
	require.Nil(t, appErr)

	th1.TearDown()

	th2 := Setup(t)
	defer th2.TearDown()

	posts, err = th2.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, err)
	assert.Len(t, posts, 0)

	// import the exported posts
	appErr, i := th2.App.BulkImport(th2.Context, &b, nil, false, 5)
	assert.Nil(t, appErr)
	assert.Equal(t, 0, i)

	posts, err = th2.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, err)

	// Adding some determinism so its possible to assert on slice index
	sort.Slice(posts, func(i, j int) bool { return posts[i].Message > posts[j].Message })
	assert.Len(t, posts, 2)
	assert.ElementsMatch(t, gmMembers, *posts[0].ChannelMembers)
	assert.ElementsMatch(t, dmMembers, *posts[1].ChannelMembers)
	assert.Contains(t, posts[0].Props["attachments"].([]interface{})[0], "footer")
	assert.Contains(t, posts[1].Props["attachments"].([]interface{})[0], "footer")
}

func TestExportDMPostWithSelf(t *testing.T) {
	th1 := Setup(t).InitBasic()

	// DM Channel with self (me channel)
	dmChannel := th1.CreateDmChannel(th1.BasicUser)

	th1.CreatePost(dmChannel)

	var b bytes.Buffer
	err := th1.App.BulkExport(&b, "somePath", BulkExportOpts{})
	require.Nil(t, err)

	posts, nErr := th1.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, nErr)
	assert.Equal(t, 1, len(posts))

	th1.TearDown()

	th2 := Setup(t)
	defer th2.TearDown()

	posts, nErr = th2.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, nErr)
	assert.Equal(t, 0, len(posts))

	// import the exported posts
	err, i := th2.App.BulkImport(th2.Context, &b, nil, false, 5)
	assert.Nil(t, err)
	assert.Equal(t, 0, i)

	posts, nErr = th2.App.Srv().Store.Post().GetDirectPostParentsForExportAfter(1000, "0000000")
	require.NoError(t, nErr)
	assert.Equal(t, 1, len(posts))
	assert.Equal(t, 1, len((*posts[0].ChannelMembers)))
	assert.Equal(t, th1.BasicUser.Username, (*posts[0].ChannelMembers)[0])
}

func TestBulkExport(t *testing.T) {
	th := Setup(t)
	testsDir, _ := fileutils.FindDir("tests")

	dir, err := ioutil.TempDir("", "import_test")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	extractImportFile := func(filePath string) *os.File {
		importFile, err2 := os.Open(filePath)
		require.NoError(t, err2)
		defer importFile.Close()

		info, err2 := importFile.Stat()
		require.NoError(t, err2)

		paths, err2 := utils.UnzipToPath(importFile, info.Size(), dir)
		require.NoError(t, err2)
		require.NotEmpty(t, paths)

		jsonFile, err2 := os.Open(filepath.Join(dir, "import.jsonl"))
		require.NoError(t, err2)

		return jsonFile
	}

	jsonFile := extractImportFile(filepath.Join(testsDir, "import_test.zip"))
	defer jsonFile.Close()

	appErr, _ := th.App.BulkImportWithPath(th.Context, jsonFile, nil, false, 1, dir)
	require.Nil(t, appErr)

	exportFile, err := os.Create(filepath.Join(dir, "export.zip"))
	require.NoError(t, err)
	defer exportFile.Close()

	opts := BulkExportOpts{
		IncludeAttachments: true,
		CreateArchive:      true,
	}
	appErr = th.App.BulkExport(exportFile, dir, opts)
	require.Nil(t, appErr)

	th.TearDown()
	th = Setup(t)
	defer th.TearDown()

	jsonFile = extractImportFile(filepath.Join(dir, "export.zip"))
	defer jsonFile.Close()

	appErr, _ = th.App.BulkImportWithPath(th.Context, jsonFile, nil, false, 1, filepath.Join(dir, "data"))
	require.Nil(t, appErr)
}
