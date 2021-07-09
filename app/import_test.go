// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func ptrStr(s string) *string {
	return &s
}

func ptrInt64(i int64) *int64 {
	return &i
}

func ptrInt(i int) *int {
	return &i
}

func ptrBool(b bool) *bool {
	return &b
}

func checkPreference(t *testing.T, a *App, userID string, category string, name string, value string) {
	preferences, err := a.Srv().Store.Preference().GetCategory(userID, category)
	require.NoErrorf(t, err, "Failed to get preferences for user %v with category %v", userID, category)
	found := false
	for _, preference := range preferences {
		if preference.Name == name {
			found = true
			require.Equal(t, preference.Value, value, "Preference for user %v in category %v with name %v has value %v, expected %v", userID, category, name, preference.Value, value)
			break
		}
	}
	require.Truef(t, found, "Did not find preference for user %v in category %v with name %v", userID, category, name)
}

func checkNotifyProp(t *testing.T, user *model.User, key string, value string) {
	actual, ok := user.NotifyProps[key]
	require.True(t, ok, "Notify prop %v not found. User: %v", key, user.ID)
	require.Equalf(t, actual, value, "Notify Prop %v was %v but expected %v. User: %v", key, actual, value, user.ID)
}

func checkError(t *testing.T, err *model.AppError) {
	require.NotNil(t, err, "Should have returned an error.")
}

func checkNoError(t *testing.T, err *model.AppError) {
	require.Nil(t, err, "Unexpected Error: %v", err)
}

func AssertAllPostsCount(t *testing.T, a *App, initialCount int64, change int64, teamName string) {
	result, err := a.Srv().Store.Post().AnalyticsPostCount(teamName, false, false)
	require.NoError(t, err)
	require.Equal(t, initialCount+change, result, "Did not find the expected number of posts.")
}

func AssertChannelCount(t *testing.T, a *App, channelType string, expectedCount int64) {
	count, err := a.Srv().Store.Channel().AnalyticsTypeCount("", channelType)
	require.Equalf(t, expectedCount, count, "Channel count of type: %v. Expected: %v, Got: %v", channelType, expectedCount, count)
	require.NoError(t, err, "Failed to get channel count.")
}

func TestImportImportLine(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// Try import line with an invalid type.
	line := LineImportData{
		Type: "gibberish",
	}

	err := th.App.importLine(th.Context, line, false)
	require.NotNil(t, err, "Expected an error when importing a line with invalid type.")

	// Try import line with team type but nil team.
	line.Type = "team"
	err = th.App.importLine(th.Context, line, false)
	require.NotNil(t, err, "Expected an error when importing a line of type team with a nil team.")

	// Try import line with channel type but nil channel.
	line.Type = "channel"
	err = th.App.importLine(th.Context, line, false)
	require.NotNil(t, err, "Expected an error when importing a line with type channel with a nil channel.")

	// Try import line with user type but nil user.
	line.Type = "user"
	err = th.App.importLine(th.Context, line, false)
	require.NotNil(t, err, "Expected an error when importing a line with type user with a nil user.")

	// Try import line with post type but nil post.
	line.Type = "post"
	err = th.App.importLine(th.Context, line, false)
	require.NotNil(t, err, "Expected an error when importing a line with type post with a nil post.")

	// Try import line with direct_channel type but nil direct_channel.
	line.Type = "direct_channel"
	err = th.App.importLine(th.Context, line, false)
	require.NotNil(t, err, "Expected an error when importing a line with type direct_channel with a nil direct_channel.")

	// Try import line with direct_post type but nil direct_post.
	line.Type = "direct_post"
	err = th.App.importLine(th.Context, line, false)
	require.NotNil(t, err, "Expected an error when importing a line with type direct_post with a nil direct_post.")

	// Try import line with scheme type but nil scheme.
	line.Type = "scheme"
	err = th.App.importLine(th.Context, line, false)
	require.NotNil(t, err, "Expected an error when importing a line with type scheme with a nil scheme.")
}

func TestStopOnError(t *testing.T) {
	assert.True(t, stopOnError(LineImportWorkerError{
		model.NewAppError("test", "app.import.attachment.bad_file.error", nil, "", http.StatusBadRequest),
		1,
	}))

	assert.True(t, stopOnError(LineImportWorkerError{
		model.NewAppError("test", "app.import.attachment.file_upload.error", nil, "", http.StatusBadRequest),
		1,
	}))

	assert.False(t, stopOnError(LineImportWorkerError{
		model.NewAppError("test", "api.file.upload_file.large_image.app_error", nil, "", http.StatusBadRequest),
		1,
	}))

	assert.False(t, stopOnError(LineImportWorkerError{
		model.NewAppError("test", "app.import.validate_direct_channel_import_data.members_too_few.error", nil, "", http.StatusBadRequest),
		1,
	}))

	assert.False(t, stopOnError(LineImportWorkerError{
		model.NewAppError("test", "app.import.validate_direct_channel_import_data.members_too_many.error", nil, "", http.StatusBadRequest),
		1,
	}))
}

func TestImportBulkImport(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

	teamName := model.NewRandomTeamName()
	channelName := model.NewID()
	username := model.NewID()
	username2 := model.NewID()
	username3 := model.NewID()
	emojiName := model.NewID()
	testsDir, _ := fileutils.FindDir("tests")
	testImage := filepath.Join(testsDir, "test.png")
	teamTheme1 := `{\"awayIndicator\":\"#DBBD4E\",\"buttonBg\":\"#23A1FF\",\"buttonColor\":\"#FFFFFF\",\"centerChannelBg\":\"#ffffff\",\"centerChannelColor\":\"#333333\",\"codeTheme\":\"github\",\"image\":\"/static/files/a4a388b38b32678e83823ef1b3e17766.png\",\"linkColor\":\"#2389d7\",\"mentionBg\":\"#2389d7\",\"mentionColor\":\"#ffffff\",\"mentionHighlightBg\":\"#fff2bb\",\"mentionHighlightLink\":\"#2f81b7\",\"newMessageSeparator\":\"#FF8800\",\"onlineIndicator\":\"#7DBE00\",\"sidebarBg\":\"#fafafa\",\"sidebarHeaderBg\":\"#3481B9\",\"sidebarHeaderTextColor\":\"#ffffff\",\"sidebarText\":\"#333333\",\"sidebarTextActiveBorder\":\"#378FD2\",\"sidebarTextActiveColor\":\"#111111\",\"sidebarTextHoverBg\":\"#e6f2fa\",\"sidebarUnreadText\":\"#333333\",\"type\":\"Mattermost\"}`
	teamTheme2 := `{\"awayIndicator\":\"#DBBD4E\",\"buttonBg\":\"#23A100\",\"buttonColor\":\"#EEEEEE\",\"centerChannelBg\":\"#ffffff\",\"centerChannelColor\":\"#333333\",\"codeTheme\":\"github\",\"image\":\"/static/files/a4a388b38b32678e83823ef1b3e17766.png\",\"linkColor\":\"#2389d7\",\"mentionBg\":\"#2389d7\",\"mentionColor\":\"#ffffff\",\"mentionHighlightBg\":\"#fff2bb\",\"mentionHighlightLink\":\"#2f81b7\",\"newMessageSeparator\":\"#FF8800\",\"onlineIndicator\":\"#7DBE00\",\"sidebarBg\":\"#fafafa\",\"sidebarHeaderBg\":\"#3481B9\",\"sidebarHeaderTextColor\":\"#ffffff\",\"sidebarText\":\"#333333\",\"sidebarTextActiveBorder\":\"#378FD2\",\"sidebarTextActiveColor\":\"#222222\",\"sidebarTextHoverBg\":\"#e6f2fa\",\"sidebarUnreadText\":\"#444444\",\"type\":\"Mattermost\"}`

	// Run bulk import with a valid 1 of everything.
	data1 := `{"type": "version", "version": 1}
{"type": "team", "team": {"type": "O", "display_name": "lskmw2d7a5ao7ppwqh5ljchvr4", "name": "` + teamName + `"}}
{"type": "channel", "channel": {"type": "O", "display_name": "xr6m6udffngark2uekvr3hoeny", "team": "` + teamName + `", "name": "` + channelName + `"}}
{"type": "user", "user": {"username": "` + username + `", "email": "` + username + `@example.com", "teams": [{"name": "` + teamName + `","theme": "` + teamTheme1 + `", "channels": [{"name": "` + channelName + `"}]}]}}
{"type": "user", "user": {"username": "` + username2 + `", "email": "` + username2 + `@example.com", "teams": [{"name": "` + teamName + `","theme": "` + teamTheme2 + `", "channels": [{"name": "` + channelName + `"}]}]}}
{"type": "user", "user": {"username": "` + username3 + `", "email": "` + username3 + `@example.com", "teams": [{"name": "` + teamName + `", "channels": [{"name": "` + channelName + `"}], "delete_at": 123456789016}]}}
{"type": "post", "post": {"team": "` + teamName + `", "channel": "` + channelName + `", "user": "` + username + `", "message": "Hello World", "create_at": 123456789012, "attachments":[{"path": "` + testImage + `"}]}}
{"type": "post", "post": {"team": "` + teamName + `", "channel": "` + channelName + `", "user": "` + username3 + `", "message": "Hey Everyone!", "create_at": 123456789013, "attachments":[{"path": "` + testImage + `"}]}}
{"type": "direct_channel", "direct_channel": {"members": ["` + username + `", "` + username + `"]}}
{"type": "direct_channel", "direct_channel": {"members": ["` + username + `", "` + username2 + `"]}}
{"type": "direct_channel", "direct_channel": {"members": ["` + username + `", "` + username2 + `", "` + username3 + `"]}}
{"type": "direct_post", "direct_post": {"channel_members": ["` + username + `", "` + username + `"], "user": "` + username + `", "message": "Hello Direct Channel to myself", "create_at": 123456789014}}
{"type": "direct_post", "direct_post": {"channel_members": ["` + username + `", "` + username2 + `"], "user": "` + username + `", "message": "Hello Direct Channel", "create_at": 123456789014}}
{"type": "direct_post", "direct_post": {"channel_members": ["` + username + `", "` + username2 + `", "` + username3 + `"], "user": "` + username + `", "message": "Hello Group Channel", "create_at": 123456789015}}
{"type": "emoji", "emoji": {"name": "` + emojiName + `", "image": "` + testImage + `"}}`

	err, line := th.App.BulkImport(th.Context, strings.NewReader(data1), nil, false, 2)
	require.Nil(t, err, "BulkImport should have succeeded")
	require.Equal(t, 0, line, "BulkImport line should be 0")

	// Run bulk import using a string that contains a line with invalid json.
	data2 := `{"type": "version", "version": 1`
	err, line = th.App.BulkImport(th.Context, strings.NewReader(data2), nil, false, 2)
	require.NotNil(t, err, "Should have failed due to invalid JSON on line 1.")
	require.Equal(t, 1, line, "Should have failed due to invalid JSON on line 1.")

	// Run bulk import using valid JSON but missing version line at the start.
	data3 := `{"type": "team", "team": {"type": "O", "display_name": "lskmw2d7a5ao7ppwqh5ljchvr4", "name": "` + teamName + `"}}
{"type": "channel", "channel": {"type": "O", "display_name": "xr6m6udffngark2uekvr3hoeny", "team": "` + teamName + `", "name": "` + channelName + `"}}
{"type": "user", "user": {"username": "kufjgnkxkrhhfgbrip6qxkfsaa", "email": "kufjgnkxkrhhfgbrip6qxkfsaa@example.com"}}
{"type": "user", "user": {"username": "bwshaim6qnc2ne7oqkd5b2s2rq", "email": "bwshaim6qnc2ne7oqkd5b2s2rq@example.com", "teams": [{"name": "` + teamName + `", "channels": [{"name": "` + channelName + `"}]}]}}`
	err, line = th.App.BulkImport(th.Context, strings.NewReader(data3), nil, false, 2)
	require.NotNil(t, err, "Should have failed due to missing version line on line 1.")
	require.Equal(t, 1, line, "Should have failed due to missing version line on line 1.")

	// Run bulk import using a valid and large input and a \r\n line break.
	t.Run("", func(t *testing.T) {
		posts := `{"type": "post"` + strings.Repeat(`, "post": {"team": "`+teamName+`", "channel": "`+channelName+`", "user": "`+username+`", "message": "Repeat after me", "create_at": 193456789012}`, 1e4) + "}"
		data4 := `{"type": "version", "version": 1}
{"type": "team", "team": {"type": "O", "display_name": "lskmw2d7a5ao7ppwqh5ljchvr4", "name": "` + teamName + `"}}
{"type": "channel", "channel": {"type": "O", "display_name": "xr6m6udffngark2uekvr3hoeny", "team": "` + teamName + `", "name": "` + channelName + `"}}
{"type": "user", "user": {"username": "` + username + `", "email": "` + username + `@example.com", "teams": [{"name": "` + teamName + `","theme": "` + teamTheme1 + `", "channels": [{"name": "` + channelName + `"}]}]}}
{"type": "post", "post": {"team": "` + teamName + `", "channel": "` + channelName + `", "user": "` + username + `", "message": "Hello World", "create_at": 123456789012}}`
		err, line = th.App.BulkImport(th.Context, strings.NewReader(data4+"\r\n"+posts), nil, false, 2)
		require.Nil(t, err, "BulkImport should have succeeded")
		require.Equal(t, 0, line, "BulkImport line should be 0")
	})

	t.Run("First item after version without type", func(t *testing.T) {
		data := `{"type": "version", "version": 1}
{"name": "custom-emoji-troll", "image": "bulkdata/emoji/trollolol.png"}`
		err, line := th.App.BulkImport(th.Context, strings.NewReader(data), nil, false, 2)
		require.NotNil(t, err, "Should have failed due to invalid type on line 2.")
		require.Equal(t, 2, line, "Should have failed due to invalid type on line 2.")
	})

	t.Run("Posts with prop information", func(t *testing.T) {
		data6 := `{"type": "version", "version": 1}
{"type": "team", "team": {"type": "O", "display_name": "lskmw2d7a5ao7ppwqh5ljchvr4", "name": "` + teamName + `"}}
{"type": "channel", "channel": {"type": "O", "display_name": "xr6m6udffngark2uekvr3hoeny", "team": "` + teamName + `", "name": "` + channelName + `"}}
{"type": "user", "user": {"username": "` + username + `", "email": "` + username + `@example.com", "teams": [{"name": "` + teamName + `","theme": "` + teamTheme1 + `", "channels": [{"name": "` + channelName + `"}]}]}}
{"type": "post", "post": {"team": "` + teamName + `", "channel": "` + channelName + `", "user": "` + username + `", "message": "Hello World", "create_at": 123456789012, "attachments":[{"path": "` + testImage + `"}], "props":{"attachments":[{"id":0,"fallback":"[February 4th, 2020 2:46 PM] author: fallback","color":"D0D0D0","pretext":"","author_name":"author","author_link":"","title":"","title_link":"","text":"this post has props","fields":null,"image_url":"","thumb_url":"","footer":"Posted in #general","footer_icon":"","ts":"1580823992.000100"}]}}}
{"type": "direct_channel", "direct_channel": {"members": ["` + username + `", "` + username + `"]}}
{"type": "direct_post", "direct_post": {"channel_members": ["` + username + `", "` + username + `"], "user": "` + username + `", "message": "Hello Direct Channel to myself", "create_at": 123456789014, "props":{"attachments":[{"id":0,"fallback":"[February 4th, 2020 2:46 PM] author: fallback","color":"D0D0D0","pretext":"","author_name":"author","author_link":"","title":"","title_link":"","text":"this post has props","fields":null,"image_url":"","thumb_url":"","footer":"Posted in #general","footer_icon":"","ts":"1580823992.000100"}]}}}}`

		err, line := th.App.BulkImport(th.Context, strings.NewReader(data6), nil, false, 2)
		require.Nil(t, err, "BulkImport should have succeeded")
		require.Equal(t, 0, line, "BulkImport line should be 0")
	})
}

func TestImportProcessImportDataFileVersionLine(t *testing.T) {
	data := LineImportData{
		Type:    "version",
		Version: ptrInt(1),
	}
	version, err := processImportDataFileVersionLine(data)
	require.Nil(t, err, "Expected no error")
	require.Equal(t, 1, version, "Expected version 1")

	data.Type = "NotVersion"
	_, err = processImportDataFileVersionLine(data)
	require.NotNil(t, err, "Expected error on invalid version line.")

	data.Type = "version"
	data.Version = nil
	_, err = processImportDataFileVersionLine(data)
	require.NotNil(t, err, "Expected error on invalid version line.")
}

func GetAttachments(userID string, th *TestHelper, t *testing.T) []*model.FileInfo {
	fileInfos, err := th.App.Srv().Store.FileInfo().GetForUser(userID)
	require.NoError(t, err)
	return fileInfos
}

func AssertFileIDsInPost(files []*model.FileInfo, th *TestHelper, t *testing.T) {
	postID := files[0].PostID
	require.NotNil(t, postID)

	posts, err := th.App.Srv().Store.Post().GetPostsByIDs([]string{postID})
	require.NoError(t, err)

	require.Len(t, posts, 1)
	for _, file := range files {
		assert.Contains(t, posts[0].FileIDs, file.ID)
	}
}

func TestRewriteFilePaths(t *testing.T) {
	genAttachments := func() *[]AttachmentImportData {
		return &[]AttachmentImportData{
			{
				Path: model.NewString("file.jpg"),
			},
			{
				Path: model.NewString("somedir/file.jpg"),
			},
		}
	}

	line := LineImportData{
		Type: "post",
		Post: &PostImportData{
			Attachments: genAttachments(),
		},
	}

	line2 := LineImportData{
		Type: "direct_post",
		DirectPost: &DirectPostImportData{
			Attachments: genAttachments(),
		},
	}

	userLine := LineImportData{
		Type: "user",
		User: &UserImportData{
			ProfileImage: model.NewString("profile.jpg"),
		},
	}

	emojiLine := LineImportData{
		Type: "emoji",
		Emoji: &EmojiImportData{
			Image: model.NewString("emoji.png"),
		},
	}

	t.Run("empty path", func(t *testing.T) {
		expected := &[]AttachmentImportData{
			{
				Path: model.NewString("file.jpg"),
			},
			{
				Path: model.NewString("somedir/file.jpg"),
			},
		}
		rewriteFilePaths(&line, "")
		require.Equal(t, expected, line.Post.Attachments)
		rewriteFilePaths(&line2, "")
		require.Equal(t, expected, line2.DirectPost.Attachments)
	})

	t.Run("valid path", func(t *testing.T) {
		expected := &[]AttachmentImportData{
			{
				Path: model.NewString("/tmp/file.jpg"),
			},
			{
				Path: model.NewString("/tmp/somedir/file.jpg"),
			},
		}

		t.Run("post attachments", func(t *testing.T) {
			rewriteFilePaths(&line, "/tmp")
			require.Equal(t, expected, line.Post.Attachments)
		})

		t.Run("direct post attachments", func(t *testing.T) {
			rewriteFilePaths(&line2, "/tmp")
			require.Equal(t, expected, line2.DirectPost.Attachments)
		})

		t.Run("profile image", func(t *testing.T) {
			expected := "/tmp/profile.jpg"
			rewriteFilePaths(&userLine, "/tmp")
			require.Equal(t, expected, *userLine.User.ProfileImage)
		})

		t.Run("emoji", func(t *testing.T) {
			expected := "/tmp/emoji.png"
			rewriteFilePaths(&emojiLine, "/tmp")
			require.Equal(t, expected, *emojiLine.Emoji.Image)
		})
	})
}

func BenchmarkBulkImport(b *testing.B) {
	th := Setup(b)
	defer th.TearDown()

	testsDir, _ := fileutils.FindDir("tests")

	importFile, err := os.Open(testsDir + "/import_test.zip")
	require.NoError(b, err)
	defer importFile.Close()

	info, err := importFile.Stat()
	require.NoError(b, err)

	dir, err := ioutil.TempDir("", "testimport")
	require.NoError(b, err)
	defer os.RemoveAll(dir)

	_, err = utils.UnzipToPath(importFile, info.Size(), dir)
	require.NoError(b, err)

	jsonFile, err := os.Open(dir + "/import.jsonl")
	require.NoError(b, err)
	defer jsonFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err, _ := th.App.BulkImportWithPath(th.Context, jsonFile, nil, false, runtime.NumCPU(), dir)
		require.Nil(b, err)
	}
	b.StopTimer()
}
