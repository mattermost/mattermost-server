// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/services/imageproxy"
	"github.com/mattermost/mattermost-server/v5/services/searchengine/mocks"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
	storemocks "github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/testlib"
)

func TestCreatePostDeduplicate(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("duplicate create post is idempotent", func(t *testing.T) {
		pendingPostID := model.NewID()
		post, err := th.App.CreatePostAsUser(th.Context, &model.Post{
			UserID:        th.BasicUser.ID,
			ChannelID:     th.BasicChannel.ID,
			Message:       "message",
			PendingPostID: pendingPostID,
		}, "", true)
		require.Nil(t, err)
		require.Equal(t, "message", post.Message)

		duplicatePost, err := th.App.CreatePostAsUser(th.Context, &model.Post{
			UserID:        th.BasicUser.ID,
			ChannelID:     th.BasicChannel.ID,
			Message:       "message",
			PendingPostID: pendingPostID,
		}, "", true)
		require.Nil(t, err)
		require.Equal(t, post.ID, duplicatePost.ID, "should have returned previously created post id")
		require.Equal(t, "message", duplicatePost.Message)
	})

	t.Run("post rejected by plugin leaves cache ready for non-deduplicated try", func(t *testing.T) {
		setupPluginApiTest(t, `
			package main

			import (
				"github.com/mattermost/mattermost-server/v5/plugin"
				"github.com/mattermost/mattermost-server/v5/model"
			)

			type MyPlugin struct {
				plugin.MattermostPlugin
				allow bool
			}

			func (p *MyPlugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
				if !p.allow {
					p.allow = true
					return nil, "rejected"
				}

				return nil, ""
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`, `{"id": "testrejectfirstpost", "backend": {"executable": "backend.exe"}}`, "testrejectfirstpost", th.App, th.Context)

		pendingPostID := model.NewID()
		post, err := th.App.CreatePostAsUser(th.Context, &model.Post{
			UserID:        th.BasicUser.ID,
			ChannelID:     th.BasicChannel.ID,
			Message:       "message",
			PendingPostID: pendingPostID,
		}, "", true)
		require.NotNil(t, err)
		require.Equal(t, "Post rejected by plugin. rejected", err.ID)
		require.Nil(t, post)

		duplicatePost, err := th.App.CreatePostAsUser(th.Context, &model.Post{
			UserID:        th.BasicUser.ID,
			ChannelID:     th.BasicChannel.ID,
			Message:       "message",
			PendingPostID: pendingPostID,
		}, "", true)
		require.Nil(t, err)
		require.Equal(t, "message", duplicatePost.Message)
	})

	t.Run("slow posting after cache entry blocks duplicate request", func(t *testing.T) {
		setupPluginApiTest(t, `
			package main

			import (
				"github.com/mattermost/mattermost-server/v5/plugin"
				"github.com/mattermost/mattermost-server/v5/model"
				"time"
			)

			type MyPlugin struct {
				plugin.MattermostPlugin
				instant bool
			}

			func (p *MyPlugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
				if !p.instant {
					p.instant = true
					time.Sleep(3 * time.Second)
				}

				return nil, ""
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
		`, `{"id": "testdelayfirstpost", "backend": {"executable": "backend.exe"}}`, "testdelayfirstpost", th.App, th.Context)

		var post *model.Post
		pendingPostID := model.NewID()

		wg := sync.WaitGroup{}

		// Launch a goroutine to make the first CreatePost call that will get delayed
		// by the plugin above.
		wg.Add(1)
		go func() {
			defer wg.Done()
			var appErr *model.AppError
			post, appErr = th.App.CreatePostAsUser(th.Context, &model.Post{
				UserID:        th.BasicUser.ID,
				ChannelID:     th.BasicChannel.ID,
				Message:       "plugin delayed",
				PendingPostID: pendingPostID,
			}, "", true)
			require.Nil(t, appErr)
			require.Equal(t, post.Message, "plugin delayed")
		}()

		// Give the goroutine above a chance to start and get delayed by the plugin.
		time.Sleep(2 * time.Second)

		// Try creating a duplicate post
		duplicatePost, err := th.App.CreatePostAsUser(th.Context, &model.Post{
			UserID:        th.BasicUser.ID,
			ChannelID:     th.BasicChannel.ID,
			Message:       "plugin delayed",
			PendingPostID: pendingPostID,
		}, "", true)
		require.NotNil(t, err)
		require.Equal(t, "api.post.deduplicate_create_post.pending", err.ID)
		require.Nil(t, duplicatePost)

		// Wait for the first CreatePost to finish to ensure assertions are made.
		wg.Wait()
	})

	t.Run("duplicate create post after cache expires is not idempotent", func(t *testing.T) {
		pendingPostID := model.NewID()
		post, err := th.App.CreatePostAsUser(th.Context, &model.Post{
			UserID:        th.BasicUser.ID,
			ChannelID:     th.BasicChannel.ID,
			Message:       "message",
			PendingPostID: pendingPostID,
		}, "", true)
		require.Nil(t, err)
		require.Equal(t, "message", post.Message)

		time.Sleep(PendingPostIDsCacheTTL)

		duplicatePost, err := th.App.CreatePostAsUser(th.Context, &model.Post{
			UserID:        th.BasicUser.ID,
			ChannelID:     th.BasicChannel.ID,
			Message:       "message",
			PendingPostID: pendingPostID,
		}, "", true)
		require.Nil(t, err)
		require.NotEqual(t, post.ID, duplicatePost.ID, "should have created new post id")
		require.Equal(t, "message", duplicatePost.Message)
	})
}

func TestAttachFilesToPost(t *testing.T) {
	t.Run("should attach files", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		info1, err := th.App.Srv().Store.FileInfo().Save(&model.FileInfo{
			CreatorID: th.BasicUser.ID,
			Path:      "path.txt",
		})
		require.NoError(t, err)

		info2, err := th.App.Srv().Store.FileInfo().Save(&model.FileInfo{
			CreatorID: th.BasicUser.ID,
			Path:      "path.txt",
		})
		require.NoError(t, err)

		post := th.BasicPost
		post.FileIDs = []string{info1.ID, info2.ID}

		appErr := th.App.attachFilesToPost(post)
		assert.Nil(t, appErr)

		infos, appErr := th.App.GetFileInfosForPost(post.ID, false)
		assert.Nil(t, appErr)
		assert.Len(t, infos, 2)
	})

	t.Run("should update File.PostIds after failing to add files", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		info1, err := th.App.Srv().Store.FileInfo().Save(&model.FileInfo{
			CreatorID: th.BasicUser.ID,
			Path:      "path.txt",
			PostID:    model.NewID(),
		})
		require.NoError(t, err)

		info2, err := th.App.Srv().Store.FileInfo().Save(&model.FileInfo{
			CreatorID: th.BasicUser.ID,
			Path:      "path.txt",
		})
		require.NoError(t, err)

		post := th.BasicPost
		post.FileIDs = []string{info1.ID, info2.ID}

		appErr := th.App.attachFilesToPost(post)
		assert.Nil(t, appErr)

		infos, appErr := th.App.GetFileInfosForPost(post.ID, false)
		assert.Nil(t, appErr)
		assert.Len(t, infos, 1)
		assert.Equal(t, info2.ID, infos[0].ID)

		updated, appErr := th.App.GetSinglePost(post.ID)
		require.Nil(t, appErr)
		assert.Len(t, updated.FileIDs, 1)
		assert.Contains(t, updated.FileIDs, info2.ID)
	})
}

func TestUpdatePostEditAt(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	post := th.BasicPost.Clone()

	post.IsPinned = true
	saved, err := th.App.UpdatePost(th.Context, post, true)
	require.Nil(t, err)
	assert.Equal(t, saved.EditAt, post.EditAt, "shouldn't have updated post.EditAt when pinning post")
	post = saved.Clone()

	time.Sleep(time.Millisecond * 100)

	post.Message = model.NewID()
	saved, err = th.App.UpdatePost(th.Context, post, true)
	require.Nil(t, err)
	assert.NotEqual(t, saved.EditAt, post.EditAt, "should have updated post.EditAt when updating post message")

	time.Sleep(time.Millisecond * 200)
}

func TestUpdatePostTimeLimit(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	post := th.BasicPost.Clone()

	th.App.Srv().SetLicense(model.NewTestLicense())

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.PostEditTimeLimit = -1
	})
	_, err := th.App.UpdatePost(th.Context, post, true)
	require.Nil(t, err)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.PostEditTimeLimit = 1000000000
	})
	post.Message = model.NewID()

	_, err = th.App.UpdatePost(th.Context, post, true)
	require.Nil(t, err, "should allow you to edit the post")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.PostEditTimeLimit = 1
	})
	post.Message = model.NewID()
	_, err = th.App.UpdatePost(th.Context, post, true)
	require.NotNil(t, err, "should fail on update old post")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.PostEditTimeLimit = -1
	})
}

func TestUpdatePostInArchivedChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	archivedChannel := th.CreateChannel(th.BasicTeam)
	post := th.CreatePost(archivedChannel)
	th.App.DeleteChannel(th.Context, archivedChannel, "")

	_, err := th.App.UpdatePost(th.Context, post, true)
	require.NotNil(t, err)
	require.Equal(t, "api.post.update_post.can_not_update_post_in_deleted.error", err.ID)
}

func TestPostReplyToPostWhereRootPosterLeftChannel(t *testing.T) {
	// This test ensures that when replying to a root post made by a user who has since left the channel, the reply
	// post completes successfully. This is a regression test for PLT-6523.
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel
	userInChannel := th.BasicUser2
	userNotInChannel := th.BasicUser
	rootPost := th.BasicPost

	_, err := th.App.AddUserToChannel(userInChannel, channel, false)
	require.Nil(t, err)

	err = th.App.RemoveUserFromChannel(th.Context, userNotInChannel.ID, "", channel)
	require.Nil(t, err)
	replyPost := model.Post{
		Message:       "asd",
		ChannelID:     channel.ID,
		RootID:        rootPost.ID,
		ParentID:      rootPost.ID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        userInChannel.ID,
		CreateAt:      0,
	}

	_, err = th.App.CreatePostAsUser(th.Context, &replyPost, "", true)
	require.Nil(t, err)
}

func TestPostAttachPostToChildPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel
	user := th.BasicUser
	rootPost := th.BasicPost

	replyPost1 := model.Post{
		Message:       "reply one",
		ChannelID:     channel.ID,
		RootID:        rootPost.ID,
		ParentID:      rootPost.ID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        user.ID,
		CreateAt:      0,
	}

	res1, err := th.App.CreatePostAsUser(th.Context, &replyPost1, "", true)
	require.Nil(t, err)

	replyPost2 := model.Post{
		Message:       "reply two",
		ChannelID:     channel.ID,
		RootID:        res1.ID,
		ParentID:      res1.ID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        user.ID,
		CreateAt:      0,
	}

	_, err = th.App.CreatePostAsUser(th.Context, &replyPost2, "", true)
	assert.Equalf(t, err.StatusCode, http.StatusBadRequest, "Expected BadRequest error, got %v", err)

	replyPost3 := model.Post{
		Message:       "reply three",
		ChannelID:     channel.ID,
		RootID:        rootPost.ID,
		ParentID:      rootPost.ID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        user.ID,
		CreateAt:      0,
	}

	_, err = th.App.CreatePostAsUser(th.Context, &replyPost3, "", true)
	assert.Nil(t, err)
}

func TestPostChannelMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel
	user := th.BasicUser

	channelToMention, err := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "Mention Test",
		Name:        "mention-test",
		Type:        model.ChannelTypeOpen,
		TeamID:      th.BasicTeam.ID,
	}, false)
	require.Nil(t, err)
	defer th.App.PermanentDeleteChannel(channelToMention)

	_, err = th.App.AddUserToChannel(user, channel, false)
	require.Nil(t, err)

	post := &model.Post{
		Message:       fmt.Sprintf("hello, ~%v!", channelToMention.Name),
		ChannelID:     channel.ID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        user.ID,
		CreateAt:      0,
	}

	result, err := th.App.CreatePostAsUser(th.Context, post, "", true)
	require.Nil(t, err)
	assert.Equal(t, map[string]interface{}{
		"mention-test": map[string]interface{}{
			"display_name": "Mention Test",
			"team_name":    th.BasicTeam.Name,
		},
	}, result.GetProp("channel_mentions"))

	post.Message = fmt.Sprintf("goodbye, ~%v!", channelToMention.Name)
	result, err = th.App.UpdatePost(th.Context, post, false)
	require.Nil(t, err)
	assert.Equal(t, map[string]interface{}{
		"mention-test": map[string]interface{}{
			"display_name": "Mention Test",
			"team_name":    th.BasicTeam.Name,
		},
	}, result.GetProp("channel_mentions"))
}

func TestImageProxy(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	mockStore := th.App.Srv().Store.(*storemocks.Store)
	mockUserStore := storemocks.UserStore{}
	mockUserStore.On("Count", mock.Anything).Return(int64(10), nil)
	mockPostStore := storemocks.PostStore{}
	mockPostStore.On("GetMaxPostSize").Return(65535, nil)
	mockSystemStore := storemocks.SystemStore{}
	mockSystemStore.On("GetByName", "UpgradedFromTE").Return(&model.System{Name: "UpgradedFromTE", Value: "false"}, nil)
	mockSystemStore.On("GetByName", "InstallationDate").Return(&model.System{Name: "InstallationDate", Value: "10"}, nil)
	mockSystemStore.On("GetByName", "FirstServerRunTimestamp").Return(&model.System{Name: "FirstServerRunTimestamp", Value: "10"}, nil)

	mockStore.On("User").Return(&mockUserStore)
	mockStore.On("Post").Return(&mockPostStore)
	mockStore.On("System").Return(&mockSystemStore)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.SiteURL = "http://mymattermost.com"
	})

	th.Server.ImageProxy = imageproxy.MakeImageProxy(th.Server, th.Server.HTTPService, th.Server.Log)

	for name, tc := range map[string]struct {
		ProxyType              string
		ProxyURL               string
		ProxyOptions           string
		ImageURL               string
		ProxiedImageURL        string
		ProxiedRemovedImageURL string
	}{
		"atmos/camo": {
			ProxyType:              model.ImageProxyTypeAtmosCamo,
			ProxyURL:               "https://127.0.0.1",
			ProxyOptions:           "foo",
			ImageURL:               "http://mydomain.com/myimage",
			ProxiedRemovedImageURL: "http://mydomain.com/myimage",
			ProxiedImageURL:        "http://mymattermost.com/api/v4/image?url=http%3A%2F%2Fmydomain.com%2Fmyimage",
		},
		"atmos/camo_SameSite": {
			ProxyType:              model.ImageProxyTypeAtmosCamo,
			ProxyURL:               "https://127.0.0.1",
			ProxyOptions:           "foo",
			ImageURL:               "http://mymattermost.com/myimage",
			ProxiedRemovedImageURL: "http://mymattermost.com/myimage",
			ProxiedImageURL:        "http://mymattermost.com/myimage",
		},
		"atmos/camo_PathOnly": {
			ProxyType:              model.ImageProxyTypeAtmosCamo,
			ProxyURL:               "https://127.0.0.1",
			ProxyOptions:           "foo",
			ImageURL:               "/myimage",
			ProxiedRemovedImageURL: "http://mymattermost.com/myimage",
			ProxiedImageURL:        "http://mymattermost.com/myimage",
		},
		"atmos/camo_EmptyImageURL": {
			ProxyType:              model.ImageProxyTypeAtmosCamo,
			ProxyURL:               "https://127.0.0.1",
			ProxyOptions:           "foo",
			ImageURL:               "",
			ProxiedRemovedImageURL: "",
			ProxiedImageURL:        "",
		},
		"local": {
			ProxyType:              model.ImageProxyTypeLocal,
			ImageURL:               "http://mydomain.com/myimage",
			ProxiedRemovedImageURL: "http://mydomain.com/myimage",
			ProxiedImageURL:        "http://mymattermost.com/api/v4/image?url=http%3A%2F%2Fmydomain.com%2Fmyimage",
		},
		"local_SameSite": {
			ProxyType:              model.ImageProxyTypeLocal,
			ImageURL:               "http://mymattermost.com/myimage",
			ProxiedRemovedImageURL: "http://mymattermost.com/myimage",
			ProxiedImageURL:        "http://mymattermost.com/myimage",
		},
		"local_PathOnly": {
			ProxyType:              model.ImageProxyTypeLocal,
			ImageURL:               "/myimage",
			ProxiedRemovedImageURL: "http://mymattermost.com/myimage",
			ProxiedImageURL:        "http://mymattermost.com/myimage",
		},
		"local_EmptyImageURL": {
			ProxyType:              model.ImageProxyTypeLocal,
			ImageURL:               "",
			ProxiedRemovedImageURL: "",
			ProxiedImageURL:        "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			th.App.UpdateConfig(func(cfg *model.Config) {
				cfg.ImageProxySettings.Enable = model.NewBool(true)
				cfg.ImageProxySettings.ImageProxyType = model.NewString(tc.ProxyType)
				cfg.ImageProxySettings.RemoteImageProxyOptions = model.NewString(tc.ProxyOptions)
				cfg.ImageProxySettings.RemoteImageProxyURL = model.NewString(tc.ProxyURL)
			})

			post := &model.Post{
				ID:      model.NewID(),
				Message: "![foo](" + tc.ImageURL + ")",
			}

			list := model.NewPostList()
			list.Posts[post.ID] = post

			assert.Equal(t, "![foo]("+tc.ProxiedImageURL+")", th.App.PostWithProxyAddedToImageURLs(post).Message)

			assert.Equal(t, "![foo]("+tc.ImageURL+")", th.App.PostWithProxyRemovedFromImageURLs(post).Message)
			post.Message = "![foo](" + tc.ProxiedImageURL + ")"
			assert.Equal(t, "![foo]("+tc.ProxiedRemovedImageURL+")", th.App.PostWithProxyRemovedFromImageURLs(post).Message)

			if tc.ImageURL != "" {
				post.Message = "![foo](" + tc.ImageURL + " =500x200)"
				assert.Equal(t, "![foo]("+tc.ProxiedImageURL+" =500x200)", th.App.PostWithProxyAddedToImageURLs(post).Message)
				assert.Equal(t, "![foo]("+tc.ImageURL+" =500x200)", th.App.PostWithProxyRemovedFromImageURLs(post).Message)
				post.Message = "![foo](" + tc.ProxiedImageURL + " =500x200)"
				assert.Equal(t, "![foo]("+tc.ProxiedRemovedImageURL+" =500x200)", th.App.PostWithProxyRemovedFromImageURLs(post).Message)
			}
		})
	}
}

func TestMaxPostSize(t *testing.T) {
	t.Skip("TODO: fix flaky test")
	t.Parallel()

	testCases := []struct {
		Description         string
		StoreMaxPostSize    int
		ExpectedMaxPostSize int
	}{
		{
			"Max post size less than model.model.POST_MESSAGE_MAX_RUNES_V1 ",
			0,
			model.PostMessageMaxRunesV1,
		},
		{
			"4000 rune limit",
			4000,
			4000,
		},
		{
			"16383 rune limit",
			16383,
			16383,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Description, func(t *testing.T) {
			t.Parallel()

			mockStore := &storetest.Store{}
			defer mockStore.AssertExpectations(t)

			mockStore.PostStore.On("GetMaxPostSize").Return(testCase.StoreMaxPostSize)

			app := App{
				srv: &Server{
					Store: mockStore,
				},
			}

			assert.Equal(t, testCase.ExpectedMaxPostSize, app.MaxPostSize())
		})
	}
}

func TestDeletePostWithFileAttachments(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create a post with a file attachment.
	teamID := th.BasicTeam.ID
	channelID := th.BasicChannel.ID
	userID := th.BasicUser.ID
	filename := "test"
	data := []byte("abcd")

	info1, err := th.App.DoUploadFile(th.Context, time.Date(2007, 2, 4, 1, 2, 3, 4, time.Local), teamID, channelID, userID, filename, data)
	require.Nil(t, err)
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info1.ID)
		th.App.RemoveFile(info1.Path)
	}()

	post := &model.Post{
		Message:       "asd",
		ChannelID:     channelID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        userID,
		CreateAt:      0,
		FileIDs:       []string{info1.ID},
	}

	post, err = th.App.CreatePost(th.Context, post, th.BasicChannel, false, true)
	assert.Nil(t, err)

	// Delete the post.
	post, err = th.App.DeletePost(post.ID, userID)
	assert.Nil(t, err)

	// Wait for the cleanup routine to finish.
	time.Sleep(time.Millisecond * 100)

	// Check that the file can no longer be reached.
	_, err = th.App.GetFileInfo(info1.ID)
	assert.NotNil(t, err)
}

func TestDeletePostInArchivedChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	archivedChannel := th.CreateChannel(th.BasicTeam)
	post := th.CreatePost(archivedChannel)
	th.App.DeleteChannel(th.Context, archivedChannel, "")

	_, err := th.App.DeletePost(post.ID, "")
	require.NotNil(t, err)
	require.Equal(t, "api.post.delete_post.can_not_delete_post_in_deleted.error", err.ID)
}

func TestCreatePost(t *testing.T) {
	t.Run("call PreparePostForClient before returning", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.SiteURL = "http://mymattermost.com"
			*cfg.ImageProxySettings.Enable = true
			*cfg.ImageProxySettings.ImageProxyType = "atmos/camo"
			*cfg.ImageProxySettings.RemoteImageProxyURL = "https://127.0.0.1"
			*cfg.ImageProxySettings.RemoteImageProxyOptions = "foo"
		})

		th.Server.ImageProxy = imageproxy.MakeImageProxy(th.Server, th.Server.HTTPService, th.Server.Log)

		imageURL := "http://mydomain.com/myimage"
		proxiedImageURL := "http://mymattermost.com/api/v4/image?url=http%3A%2F%2Fmydomain.com%2Fmyimage"

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "![image](" + imageURL + ")",
			UserID:    th.BasicUser.ID,
		}

		rpost, err := th.App.CreatePost(th.Context, post, th.BasicChannel, false, true)
		require.Nil(t, err)
		assert.Equal(t, "![image]("+proxiedImageURL+")", rpost.Message)
	})

	t.Run("Sets prop MENTION_HIGHLIGHT_DISABLED when it should", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		th.AddUserToChannel(th.BasicUser, th.BasicChannel)

		t.Run("Does not set prop when user has USE_CHANNEL_MENTIONS", func(t *testing.T) {
			postWithNoMention := &model.Post{
				ChannelID: th.BasicChannel.ID,
				Message:   "This post does not have mentions",
				UserID:    th.BasicUser.ID,
			}
			rpost, err := th.App.CreatePost(th.Context, postWithNoMention, th.BasicChannel, false, true)
			require.Nil(t, err)
			assert.Equal(t, rpost.GetProps(), model.StringInterface{})

			postWithMention := &model.Post{
				ChannelID: th.BasicChannel.ID,
				Message:   "This post has @here mention @all",
				UserID:    th.BasicUser.ID,
			}
			rpost, err = th.App.CreatePost(th.Context, postWithMention, th.BasicChannel, false, true)
			require.Nil(t, err)
			assert.Equal(t, rpost.GetProps(), model.StringInterface{})
		})

		t.Run("Sets prop when post has mentions and user does not have USE_CHANNEL_MENTIONS", func(t *testing.T) {
			th.RemovePermissionFromRole(model.PermissionUseChannelMentions.ID, model.ChannelUserRoleID)
			th.RemovePermissionFromRole(model.PermissionUseChannelMentions.ID, model.ChannelAdminRoleID)

			postWithNoMention := &model.Post{
				ChannelID: th.BasicChannel.ID,
				Message:   "This post does not have mentions",
				UserID:    th.BasicUser.ID,
			}
			rpost, err := th.App.CreatePost(th.Context, postWithNoMention, th.BasicChannel, false, true)
			require.Nil(t, err)
			assert.Equal(t, rpost.GetProps(), model.StringInterface{})

			postWithMention := &model.Post{
				ChannelID: th.BasicChannel.ID,
				Message:   "This post has @here mention @all",
				UserID:    th.BasicUser.ID,
			}
			rpost, err = th.App.CreatePost(th.Context, postWithMention, th.BasicChannel, false, true)
			require.Nil(t, err)
			assert.Equal(t, rpost.GetProp(model.PostPropsMentionHighlightDisabled), true)

			th.AddPermissionToRole(model.PermissionUseChannelMentions.ID, model.ChannelUserRoleID)
			th.AddPermissionToRole(model.PermissionUseChannelMentions.ID, model.ChannelAdminRoleID)
		})
	})
}

func TestPatchPost(t *testing.T) {
	t.Run("call PreparePostForClient before returning", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.SiteURL = "http://mymattermost.com"
			*cfg.ImageProxySettings.Enable = true
			*cfg.ImageProxySettings.ImageProxyType = "atmos/camo"
			*cfg.ImageProxySettings.RemoteImageProxyURL = "https://127.0.0.1"
			*cfg.ImageProxySettings.RemoteImageProxyOptions = "foo"
		})

		th.Server.ImageProxy = imageproxy.MakeImageProxy(th.Server, th.Server.HTTPService, th.Server.Log)

		imageURL := "http://mydomain.com/myimage"
		proxiedImageURL := "http://mymattermost.com/api/v4/image?url=http%3A%2F%2Fmydomain.com%2Fmyimage"

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "![image](http://mydomain/anotherimage)",
			UserID:    th.BasicUser.ID,
		}

		rpost, err := th.App.CreatePost(th.Context, post, th.BasicChannel, false, true)
		require.Nil(t, err)
		assert.NotEqual(t, "![image]("+proxiedImageURL+")", rpost.Message)

		patch := &model.PostPatch{
			Message: model.NewString("![image](" + imageURL + ")"),
		}

		rpost, err = th.App.PatchPost(th.Context, rpost.ID, patch)
		require.Nil(t, err)
		assert.Equal(t, "![image]("+proxiedImageURL+")", rpost.Message)
	})

	t.Run("Sets Prop MENTION_HIGHLIGHT_DISABLED when it should", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.AddUserToChannel(th.BasicUser, th.BasicChannel)

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "This post does not have mentions",
			UserID:    th.BasicUser.ID,
		}

		rpost, err := th.App.CreatePost(th.Context, post, th.BasicChannel, false, true)
		require.Nil(t, err)

		t.Run("Does not set prop when user has USE_CHANNEL_MENTIONS", func(t *testing.T) {
			patchWithNoMention := &model.PostPatch{Message: model.NewString("This patch has no channel mention")}

			rpost, err = th.App.PatchPost(th.Context, rpost.ID, patchWithNoMention)
			require.Nil(t, err)
			assert.Equal(t, rpost.GetProps(), model.StringInterface{})

			patchWithMention := &model.PostPatch{Message: model.NewString("This patch has a mention now @here")}

			rpost, err = th.App.PatchPost(th.Context, rpost.ID, patchWithMention)
			require.Nil(t, err)
			assert.Equal(t, rpost.GetProps(), model.StringInterface{})
		})

		t.Run("Sets prop when user does not have USE_CHANNEL_MENTIONS", func(t *testing.T) {
			th.RemovePermissionFromRole(model.PermissionUseChannelMentions.ID, model.ChannelUserRoleID)
			th.RemovePermissionFromRole(model.PermissionUseChannelMentions.ID, model.ChannelAdminRoleID)

			patchWithNoMention := &model.PostPatch{Message: model.NewString("This patch still does not have a mention")}
			rpost, err = th.App.PatchPost(th.Context, rpost.ID, patchWithNoMention)
			require.Nil(t, err)
			assert.Equal(t, rpost.GetProps(), model.StringInterface{})

			patchWithMention := &model.PostPatch{Message: model.NewString("This patch has a mention now @here")}

			rpost, err = th.App.PatchPost(th.Context, rpost.ID, patchWithMention)
			require.Nil(t, err)
			assert.Equal(t, rpost.GetProp(model.PostPropsMentionHighlightDisabled), true)

			th.AddPermissionToRole(model.PermissionUseChannelMentions.ID, model.ChannelUserRoleID)
			th.AddPermissionToRole(model.PermissionUseChannelMentions.ID, model.ChannelAdminRoleID)
		})
	})
}

func TestCreatePostAsUser(t *testing.T) {
	t.Run("marks channel as viewed for regular user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "test",
			UserID:    th.BasicUser.ID,
		}

		channelMemberBefore, err := th.App.Srv().Store.Channel().GetMember(context.Background(), th.BasicChannel.ID, th.BasicUser.ID)
		require.NoError(t, err)

		time.Sleep(1 * time.Millisecond)
		_, appErr := th.App.CreatePostAsUser(th.Context, post, "", true)
		require.Nil(t, appErr)

		channelMemberAfter, err := th.App.Srv().Store.Channel().GetMember(context.Background(), th.BasicChannel.ID, th.BasicUser.ID)
		require.NoError(t, err)

		require.Greater(t, channelMemberAfter.LastViewedAt, channelMemberBefore.LastViewedAt)
	})

	t.Run("does not mark channel as viewed for webhook from user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "test",
			UserID:    th.BasicUser.ID,
		}
		post.AddProp("from_webhook", "true")

		channelMemberBefore, err := th.App.Srv().Store.Channel().GetMember(context.Background(), th.BasicChannel.ID, th.BasicUser.ID)
		require.NoError(t, err)

		time.Sleep(1 * time.Millisecond)
		_, appErr := th.App.CreatePostAsUser(th.Context, post, "", true)
		require.Nil(t, appErr)

		channelMemberAfter, err := th.App.Srv().Store.Channel().GetMember(context.Background(), th.BasicChannel.ID, th.BasicUser.ID)
		require.NoError(t, err)

		require.Equal(t, channelMemberAfter.LastViewedAt, channelMemberBefore.LastViewedAt)
	})

	t.Run("does not mark channel as viewed for bot user in channel", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		bot := th.CreateBot()

		botUser, appErr := th.App.GetUser(bot.UserID)
		require.Nil(t, appErr)

		th.LinkUserToTeam(botUser, th.BasicTeam)
		th.AddUserToChannel(botUser, th.BasicChannel)

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "test",
			UserID:    bot.UserID,
		}

		channelMemberBefore, nErr := th.App.Srv().Store.Channel().GetMember(context.Background(), th.BasicChannel.ID, th.BasicUser.ID)
		require.NoError(t, nErr)

		time.Sleep(1 * time.Millisecond)
		_, appErr = th.App.CreatePostAsUser(th.Context, post, "", true)
		require.Nil(t, appErr)

		channelMemberAfter, nErr := th.App.Srv().Store.Channel().GetMember(context.Background(), th.BasicChannel.ID, th.BasicUser.ID)
		require.NoError(t, nErr)

		require.Equal(t, channelMemberAfter.LastViewedAt, channelMemberBefore.LastViewedAt)
	})

	t.Run("logs warning for user not in channel", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user := th.CreateUser()
		th.LinkUserToTeam(user, th.BasicTeam)

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "test",
			UserID:    user.ID,
		}

		_, appErr := th.App.CreatePostAsUser(th.Context, post, "", true)
		require.Nil(t, appErr)

		testlib.AssertLog(t, th.LogBuffer, mlog.LevelWarn, "Failed to get membership")
	})

	t.Run("does not log warning for bot user not in channel", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		bot := th.CreateBot()

		botUser, appErr := th.App.GetUser(bot.UserID)
		require.Nil(t, appErr)

		th.LinkUserToTeam(botUser, th.BasicTeam)

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "test",
			UserID:    bot.UserID,
		}

		_, appErr = th.App.CreatePostAsUser(th.Context, post, "", true)
		require.Nil(t, appErr)

		testlib.AssertNoLog(t, th.LogBuffer, mlog.LevelWarn, "Failed to get membership")
	})
}

func TestPatchPostInArchivedChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	archivedChannel := th.CreateChannel(th.BasicTeam)
	post := th.CreatePost(archivedChannel)
	th.App.DeleteChannel(th.Context, archivedChannel, "")

	_, err := th.App.PatchPost(th.Context, post.ID, &model.PostPatch{IsPinned: model.NewBool(true)})
	require.NotNil(t, err)
	require.Equal(t, "api.post.patch_post.can_not_update_post_in_deleted.error", err.ID)
}

func TestUpdatePost(t *testing.T) {
	t.Run("call PreparePostForClient before returning", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.SiteURL = "http://mymattermost.com"
			*cfg.ImageProxySettings.Enable = true
			*cfg.ImageProxySettings.ImageProxyType = "atmos/camo"
			*cfg.ImageProxySettings.RemoteImageProxyURL = "https://127.0.0.1"
			*cfg.ImageProxySettings.RemoteImageProxyOptions = "foo"
		})

		th.Server.ImageProxy = imageproxy.MakeImageProxy(th.Server, th.Server.HTTPService, th.Server.Log)

		imageURL := "http://mydomain.com/myimage"
		proxiedImageURL := "http://mymattermost.com/api/v4/image?url=http%3A%2F%2Fmydomain.com%2Fmyimage"

		post := &model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "![image](http://mydomain/anotherimage)",
			UserID:    th.BasicUser.ID,
		}

		rpost, err := th.App.CreatePost(th.Context, post, th.BasicChannel, false, true)
		require.Nil(t, err)
		assert.NotEqual(t, "![image]("+proxiedImageURL+")", rpost.Message)

		post.ID = rpost.ID
		post.Message = "![image](" + imageURL + ")"

		rpost, err = th.App.UpdatePost(th.Context, post, false)
		require.Nil(t, err)
		assert.Equal(t, "![image]("+proxiedImageURL+")", rpost.Message)
	})
}

func TestSearchPostsInTeamForUser(t *testing.T) {
	perPage := 5
	searchTerm := "searchTerm"

	setup := func(t *testing.T, enableElasticsearch bool) (*TestHelper, []*model.Post) {
		th := Setup(t).InitBasic()

		posts := make([]*model.Post, 7)
		for i := 0; i < cap(posts); i++ {
			post, err := th.App.CreatePost(th.Context, &model.Post{
				UserID:    th.BasicUser.ID,
				ChannelID: th.BasicChannel.ID,
				Message:   searchTerm,
			}, th.BasicChannel, false, true)

			require.Nil(t, err)

			posts[i] = post
		}

		if enableElasticsearch {
			th.App.Srv().SetLicense(model.NewTestLicense("elastic_search"))

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ElasticsearchSettings.EnableIndexing = true
				*cfg.ElasticsearchSettings.EnableSearching = true
			})
		} else {
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ElasticsearchSettings.EnableSearching = false
			})
		}

		return th, posts
	}

	t.Run("should return everything as first page of posts from database", func(t *testing.T) {
		th, posts := setup(t, false)
		defer th.TearDown()

		page := 0

		results, err := th.App.SearchPostsInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		assert.Nil(t, err)
		assert.Equal(t, []string{
			posts[6].ID,
			posts[5].ID,
			posts[4].ID,
			posts[3].ID,
			posts[2].ID,
			posts[1].ID,
			posts[0].ID,
		}, results.Order)
	})

	t.Run("should not return later pages of posts from database", func(t *testing.T) {
		th, _ := setup(t, false)
		defer th.TearDown()

		page := 1

		results, err := th.App.SearchPostsInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		assert.Nil(t, err)
		assert.Equal(t, []string{}, results.Order)
	})

	t.Run("should return first page of posts from ElasticSearch", func(t *testing.T) {
		th, posts := setup(t, true)
		defer th.TearDown()

		page := 0
		resultsPage := []string{
			posts[6].ID,
			posts[5].ID,
			posts[4].ID,
			posts[3].ID,
			posts[2].ID,
		}

		es := &mocks.SearchEngineInterface{}
		es.On("SearchPosts", mock.Anything, mock.Anything, page, perPage).Return(resultsPage, nil, nil)
		es.On("GetName").Return("mock")
		es.On("Start").Return(nil).Maybe()
		es.On("IsActive").Return(true)
		es.On("IsSearchEnabled").Return(true)
		th.App.Srv().SearchEngine.ElasticsearchEngine = es
		defer func() {
			th.App.Srv().SearchEngine.ElasticsearchEngine = nil
		}()

		results, err := th.App.SearchPostsInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		assert.Nil(t, err)
		assert.Equal(t, resultsPage, results.Order)
		es.AssertExpectations(t)
	})

	t.Run("should return later pages of posts from ElasticSearch", func(t *testing.T) {
		th, posts := setup(t, true)
		defer th.TearDown()

		page := 1
		resultsPage := []string{
			posts[1].ID,
			posts[0].ID,
		}

		es := &mocks.SearchEngineInterface{}
		es.On("SearchPosts", mock.Anything, mock.Anything, page, perPage).Return(resultsPage, nil, nil)
		es.On("GetName").Return("mock")
		es.On("Start").Return(nil).Maybe()
		es.On("IsActive").Return(true)
		es.On("IsSearchEnabled").Return(true)
		th.App.Srv().SearchEngine.ElasticsearchEngine = es
		defer func() {
			th.App.Srv().SearchEngine.ElasticsearchEngine = nil
		}()

		results, err := th.App.SearchPostsInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		assert.Nil(t, err)
		assert.Equal(t, resultsPage, results.Order)
		es.AssertExpectations(t)
	})

	t.Run("should fall back to database if ElasticSearch fails on first page", func(t *testing.T) {
		th, posts := setup(t, true)
		defer th.TearDown()

		page := 0

		es := &mocks.SearchEngineInterface{}
		es.On("SearchPosts", mock.Anything, mock.Anything, page, perPage).Return(nil, nil, &model.AppError{})
		es.On("GetName").Return("mock")
		es.On("Start").Return(nil).Maybe()
		es.On("IsActive").Return(true)
		es.On("IsSearchEnabled").Return(true)
		th.App.Srv().SearchEngine.ElasticsearchEngine = es
		defer func() {
			th.App.Srv().SearchEngine.ElasticsearchEngine = nil
		}()

		results, err := th.App.SearchPostsInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		assert.Nil(t, err)
		assert.Equal(t, []string{
			posts[6].ID,
			posts[5].ID,
			posts[4].ID,
			posts[3].ID,
			posts[2].ID,
			posts[1].ID,
			posts[0].ID,
		}, results.Order)
		es.AssertExpectations(t)
	})

	t.Run("should return nothing if ElasticSearch fails on later pages", func(t *testing.T) {
		th, _ := setup(t, true)
		defer th.TearDown()

		page := 1

		es := &mocks.SearchEngineInterface{}
		es.On("SearchPosts", mock.Anything, mock.Anything, page, perPage).Return(nil, nil, &model.AppError{})
		es.On("GetName").Return("mock")
		es.On("Start").Return(nil).Maybe()
		es.On("IsActive").Return(true)
		es.On("IsSearchEnabled").Return(true)
		th.App.Srv().SearchEngine.ElasticsearchEngine = es
		defer func() {
			th.App.Srv().SearchEngine.ElasticsearchEngine = nil
		}()

		results, err := th.App.SearchPostsInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		assert.Nil(t, err)
		assert.Equal(t, []string{}, results.Order)
		es.AssertExpectations(t)
	})
}

func TestCountMentionsFromPost(t *testing.T) {
	t.Run("should not count posts without mentions", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test2",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test3",
		}, channel, false, true)
		require.Nil(t, err)

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("should count keyword mentions", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		user2.NotifyProps[model.MentionKeysNotifyProp] = "apple"

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test2",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "apple",
		}, channel, false, true)
		require.Nil(t, err)

		// post1 and post3 should mention the user

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("should count channel-wide mentions when enabled", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		user2.NotifyProps[model.ChannelMentionsNotifyProp] = "true"

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "@channel",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "@all",
		}, channel, false, true)
		require.Nil(t, err)

		// post2 and post3 should mention the user

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("should not count channel-wide mentions when disabled for user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		user2.NotifyProps[model.ChannelMentionsNotifyProp] = "false"

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "@channel",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "@all",
		}, channel, false, true)
		require.Nil(t, err)

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("should not count channel-wide mentions when disabled for channel", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		user2.NotifyProps[model.ChannelMentionsNotifyProp] = "true"

		_, err := th.App.UpdateChannelMemberNotifyProps(map[string]string{
			model.IgnoreChannelMentionsNotifyProp: model.IgnoreChannelMentionsOn,
		}, channel.ID, user2.ID)
		require.Nil(t, err)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "@channel",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "@all",
		}, channel, false, true)
		require.Nil(t, err)

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("should count comment mentions when using COMMENTS_NOTIFY_ROOT", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		user2.NotifyProps[model.CommentsNotifyProp] = model.CommentsNotifyRoot

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			Message:   "test",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			RootID:    post1.ID,
			Message:   "test2",
		}, channel, false, true)
		require.Nil(t, err)
		post3, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test3",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			RootID:    post3.ID,
			Message:   "test4",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			RootID:    post3.ID,
			Message:   "test5",
		}, channel, false, true)
		require.Nil(t, err)

		// post2 should mention the user

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("should count comment mentions when using COMMENTS_NOTIFY_ANY", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		user2.NotifyProps[model.CommentsNotifyProp] = model.CommentsNotifyAny

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			Message:   "test",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			RootID:    post1.ID,
			Message:   "test2",
		}, channel, false, true)
		require.Nil(t, err)
		post3, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test3",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			RootID:    post3.ID,
			Message:   "test4",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			RootID:    post3.ID,
			Message:   "test5",
		}, channel, false, true)
		require.Nil(t, err)

		// post2 and post5 should mention the user

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("should count mentions caused by being added to the channel", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test",
			Type:      model.PostTypeAddToChannel,
			Props: map[string]interface{}{
				model.PostPropsAddedUserID: model.NewID(),
			},
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test2",
			Type:      model.PostTypeAddToChannel,
			Props: map[string]interface{}{
				model.PostPropsAddedUserID: user2.ID,
			},
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test3",
			Type:      model.PostTypeAddToChannel,
			Props: map[string]interface{}{
				model.PostPropsAddedUserID: user2.ID,
			},
		}, channel, false, true)
		require.Nil(t, err)

		// should be mentioned by post2 and post3

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("should return the number of posts made by the other user for a direct channel", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel, err := th.App.createDirectChannel(user1.ID, user2.ID)
		require.Nil(t, err)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test",
		}, channel, false, true)
		require.Nil(t, err)

		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test2",
		}, channel, false, true)
		require.Nil(t, err)

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 2, count)

		count, _, err = th.App.countMentionsFromPost(user1, post1)

		assert.Nil(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("should not count mentions from the before the given post", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		_, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)
		post2, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test2",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)

		// post1 and post3 should mention the user, but we only count post3

		count, _, err := th.App.countMentionsFromPost(user2, post2)

		assert.Nil(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("should not count mentions from the user's own posts", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)

		// post2 should mention the user

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("should include comments made before the given post when counting comment mentions", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		user2.NotifyProps[model.CommentsNotifyProp] = model.CommentsNotifyAny

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test1",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			RootID:    post1.ID,
			Message:   "test2",
		}, channel, false, true)
		require.Nil(t, err)
		post3, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test3",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			RootID:    post1.ID,
			Message:   "test4",
		}, channel, false, true)
		require.Nil(t, err)

		// post4 should mention the user

		count, _, err := th.App.countMentionsFromPost(user2, post3)

		assert.Nil(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("should count mentions from the user's webhook posts", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test1",
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)
		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
			Props: map[string]interface{}{
				"from_webhook": "true",
			},
		}, channel, false, true)
		require.Nil(t, err)

		// post3 should mention the user

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("should count multiple pages of mentions", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		numPosts := 215

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)

		for i := 0; i < numPosts-1; i++ {
			_, err = th.App.CreatePost(th.Context, &model.Post{
				UserID:    user1.ID,
				ChannelID: channel.ID,
				Message:   fmt.Sprintf("@%s", user2.Username),
			}, channel, false, true)
			require.Nil(t, err)
		}

		// Every post should mention the user

		count, _, err := th.App.countMentionsFromPost(user2, post1)

		assert.Nil(t, err)
		assert.Equal(t, numPosts, count)
	})
}

func TestFillInPostProps(t *testing.T) {
	t.Run("should not add disable group highlight to post props for user with group mention permissions", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

		user1 := th.BasicUser

		channel := th.CreateChannel(th.BasicTeam)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "test123123 @group1 @group2 blah blah blah",
		}, channel, false, true)
		require.Nil(t, err)

		err = th.App.FillInPostProps(post1, channel)

		assert.Nil(t, err)
		assert.Equal(t, post1.Props, model.StringInterface{})
	})

	t.Run("should not add disable group highlight to post props for app without license", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		id := model.NewID()
		guest := &model.User{
			Email:         "success+" + id + "@simulator.amazonses.com",
			Username:      "un_" + id,
			Nickname:      "nn_" + id,
			Password:      "Password1",
			EmailVerified: true,
		}
		guest, err := th.App.CreateGuest(th.Context, guest)
		require.Nil(t, err)
		th.LinkUserToTeam(guest, th.BasicTeam)

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(guest, channel)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    guest.ID,
			ChannelID: channel.ID,
			Message:   "test123123 @group1 @group2 blah blah blah",
		}, channel, false, true)
		require.Nil(t, err)

		err = th.App.FillInPostProps(post1, channel)

		assert.Nil(t, err)
		assert.Equal(t, post1.Props, model.StringInterface{})
	})

	t.Run("should add disable group highlight to post props for guest user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

		id := model.NewID()
		guest := &model.User{
			Email:         "success+" + id + "@simulator.amazonses.com",
			Username:      "un_" + id,
			Nickname:      "nn_" + id,
			Password:      "Password1",
			EmailVerified: true,
		}
		guest, err := th.App.CreateGuest(th.Context, guest)
		require.Nil(t, err)
		th.LinkUserToTeam(guest, th.BasicTeam)

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(guest, channel)

		post1, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    guest.ID,
			ChannelID: channel.ID,
			Message:   "test123123 @group1 @group2 blah blah blah",
		}, channel, false, true)
		require.Nil(t, err)

		err = th.App.FillInPostProps(post1, channel)

		assert.Nil(t, err)
		assert.Equal(t, post1.Props, model.StringInterface{"disable_group_highlight": true})
	})
}

func TestThreadMembership(t *testing.T) {
	t.Run("should update memberships for conversation participants", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user1 := th.BasicUser
		user2 := th.BasicUser2

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)

		postRoot, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "root post",
		}, channel, false, true)
		require.Nil(t, err)

		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			RootID:    postRoot.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)

		// first user should now be part of the thread since they replied to a post
		memberships, err2 := th.App.GetThreadMembershipsForUser(user1.ID, th.BasicTeam.ID)
		require.NoError(t, err2)
		require.Len(t, memberships, 1)
		// second user should also be part of a thread since they were mentioned
		memberships, err2 = th.App.GetThreadMembershipsForUser(user2.ID, th.BasicTeam.ID)
		require.NoError(t, err2)
		require.Len(t, memberships, 1)

		post2, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			Message:   "second post",
		}, channel, false, true)
		require.Nil(t, err)

		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user2.ID,
			ChannelID: channel.ID,
			RootID:    post2.ID,
			Message:   fmt.Sprintf("@%s", user1.Username),
		}, channel, false, true)
		require.Nil(t, err)

		// first user should now be part of two threads
		memberships, err2 = th.App.GetThreadMembershipsForUser(user1.ID, th.BasicTeam.ID)
		require.NoError(t, err2)
		require.Len(t, memberships, 2)
	})
}

func TestFollowThreadSkipsParticipants(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDefaultOn
	})

	channel := th.BasicChannel
	user := th.BasicUser
	user2 := th.BasicUser2
	sysadmin := th.SystemAdminUser

	appErr := th.App.JoinChannel(th.Context, channel, user.ID)
	require.Nil(t, appErr)
	appErr = th.App.JoinChannel(th.Context, channel, user2.ID)
	require.Nil(t, appErr)
	_, appErr = th.App.JoinUserToTeam(th.Context, th.BasicTeam, sysadmin, sysadmin.ID)
	require.Nil(t, appErr)
	appErr = th.App.JoinChannel(th.Context, channel, sysadmin.ID)
	require.Nil(t, appErr)

	p1, err := th.App.CreatePost(th.Context, &model.Post{UserID: user.ID, ChannelID: channel.ID, Message: "Hi @" + sysadmin.Username}, channel, false, false)
	require.Nil(t, err)
	_, err = th.App.CreatePost(th.Context, &model.Post{RootID: p1.ID, UserID: user.ID, ChannelID: channel.ID, Message: "Hola"}, channel, false, false)
	require.Nil(t, err)

	threadMembership, err := th.App.GetThreadMembershipForUser(user.ID, p1.ID)
	require.Nil(t, err)
	thread, err := th.App.GetThreadForUser(th.BasicTeam.ID, threadMembership, false)
	require.Nil(t, err)
	require.Len(t, thread.Participants, 1) // length should be 1, the original poster, since sysadmin was just mentioned but didn't post

	_, err = th.App.CreatePost(th.Context, &model.Post{RootID: p1.ID, UserID: sysadmin.ID, ChannelID: channel.ID, Message: "sysadmin reply"}, channel, false, false)
	require.Nil(t, err)

	threadMembership, err = th.App.GetThreadMembershipForUser(user.ID, p1.ID)
	require.Nil(t, err)
	thread, err = th.App.GetThreadForUser(th.BasicTeam.ID, threadMembership, false)
	require.Nil(t, err)
	require.Len(t, thread.Participants, 2) // length should be 2, the original poster and sysadmin, since sysadmin participated now

	// another user follows the thread
	th.App.UpdateThreadFollowForUser(user2.ID, th.BasicTeam.ID, p1.ID, true)

	threadMembership, err = th.App.GetThreadMembershipForUser(user2.ID, p1.ID)
	require.Nil(t, err)
	thread, err = th.App.GetThreadForUser(th.BasicTeam.ID, threadMembership, false)
	require.Nil(t, err)
	require.Len(t, thread.Participants, 2) // length should be 2, since follow shouldn't update participant list, only user1 and sysadmin are participants
	for _, p := range thread.Participants {
		require.True(t, p.ID == sysadmin.ID || p.ID == user.ID)
	}
}

func TestAutofollowBasedOnRootPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDefaultOn
	})

	channel := th.BasicChannel
	user := th.BasicUser
	user2 := th.BasicUser2
	appErr := th.App.JoinChannel(th.Context, channel, user.ID)
	require.Nil(t, appErr)
	appErr = th.App.JoinChannel(th.Context, channel, user2.ID)
	require.Nil(t, appErr)
	p1, err := th.App.CreatePost(th.Context, &model.Post{UserID: user.ID, ChannelID: channel.ID, Message: "Hi @" + user2.Username}, channel, false, false)
	require.Nil(t, err)
	m, e := th.App.GetThreadMembershipsForUser(user2.ID, th.BasicTeam.ID)
	require.NoError(t, e)
	require.Len(t, m, 0)
	_, err2 := th.App.CreatePost(th.Context, &model.Post{RootID: p1.ID, UserID: user.ID, ChannelID: channel.ID, Message: "Hola"}, channel, false, false)
	require.Nil(t, err2)
	m, e = th.App.GetThreadMembershipsForUser(user2.ID, th.BasicTeam.ID)
	require.NoError(t, e)
	require.Len(t, m, 1)
}

func TestViewChannelShouldNotUpdateThreads(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDefaultOn
	})

	channel := th.BasicChannel
	user := th.BasicUser
	user2 := th.BasicUser2
	appErr := th.App.JoinChannel(th.Context, channel, user.ID)
	require.Nil(t, appErr)
	appErr = th.App.JoinChannel(th.Context, channel, user2.ID)
	require.Nil(t, appErr)
	p1, err := th.App.CreatePost(th.Context, &model.Post{UserID: user.ID, ChannelID: channel.ID, Message: "Hi @" + user2.Username}, channel, false, false)
	require.Nil(t, err)
	_, err2 := th.App.CreatePost(th.Context, &model.Post{RootID: p1.ID, UserID: user.ID, ChannelID: channel.ID, Message: "Hola"}, channel, false, false)
	require.Nil(t, err2)
	m, e := th.App.GetThreadMembershipsForUser(user2.ID, th.BasicTeam.ID)
	require.NoError(t, e)

	th.App.ViewChannel(&model.ChannelView{
		ChannelID:     channel.ID,
		PrevChannelID: "",
	}, user2.ID, "", true)

	m1, e1 := th.App.GetThreadMembershipsForUser(user2.ID, th.BasicTeam.ID)
	require.NoError(t, e1)
	require.Equal(t, m[0].LastViewed, m1[0].LastViewed) // opening the channel shouldn't update threads
}

func TestCollapsedThreadFetch(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDefaultOn
	})
	user1 := th.BasicUser
	user2 := th.BasicUser2

	t.Run("should only return root posts, enriched", func(t *testing.T) {
		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)
		defer th.App.DeleteChannel(th.Context, channel, user1.ID)

		postRoot, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "root post",
		}, channel, false, true)
		require.Nil(t, err)

		_, err = th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			RootID:    postRoot.ID,
			Message:   fmt.Sprintf("@%s", user2.Username),
		}, channel, false, true)
		require.Nil(t, err)
		thread, nErr := th.App.Srv().Store.Thread().Get(postRoot.ID)
		require.NoError(t, nErr)
		require.Len(t, thread.Participants, 1)
		th.App.MarkChannelAsUnreadFromPost(postRoot.ID, user1.ID, true, true)
		l, err := th.App.GetPostsForChannelAroundLastUnread(channel.ID, user1.ID, 10, 10, true, true, false)
		require.Nil(t, err)
		require.Len(t, l.Order, 1)
		require.EqualValues(t, 1, l.Posts[postRoot.ID].ReplyCount)
		require.EqualValues(t, []string{user1.ID}, []string{l.Posts[postRoot.ID].Participants[0].ID})
		require.Empty(t, l.Posts[postRoot.ID].Participants[0].Email)
		require.NotZero(t, l.Posts[postRoot.ID].LastReplyAt)
		require.True(t, *l.Posts[postRoot.ID].IsFollowing)

		// try extended fetch
		l, err = th.App.GetPostsForChannelAroundLastUnread(channel.ID, user1.ID, 10, 10, true, true, true)
		require.Nil(t, err)
		require.Len(t, l.Order, 1)
		require.NotEmpty(t, l.Posts[postRoot.ID].Participants[0].Email)
	})

	t.Run("Should not panic on unexpected db error", func(t *testing.T) {
		os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
		defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.FeatureFlags.CollapsedThreads = true
		})

		channel := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(user2, channel)
		defer th.App.DeleteChannel(th.Context, channel, user1.ID)

		postRoot, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user1.ID,
			ChannelID: channel.ID,
			Message:   "root post",
		}, channel, false, true)
		require.Nil(t, err)

		// we introduce a race to trigger an unexpected error from the db side.
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			th.Server.Store.Post().PermanentDeleteByUser(user1.ID)
		}()

		require.NotPanics(t, func() {
			_, err = th.App.CreatePost(th.Context, &model.Post{
				UserID:    user1.ID,
				ChannelID: channel.ID,
				RootID:    postRoot.ID,
				Message:   fmt.Sprintf("@%s", user2.Username),
			}, channel, false, true)
			require.Nil(t, err)
		})

		wg.Wait()
	})
}

func TestReplyToPostWithLag(t *testing.T) {
	if !replicaFlag {
		t.Skipf("requires test flag -mysql-replica")
	}

	th := Setup(t).InitBasic()
	defer th.TearDown()

	if *th.App.Srv().Config().SqlSettings.DriverName != model.DatabaseDriverMysql {
		t.Skipf("requires %q database driver", model.DatabaseDriverMysql)
	}

	mainHelper.SQLStore.UpdateLicense(model.NewTestLicense("somelicense"))

	t.Run("replication lag time great than reply time", func(t *testing.T) {
		err := mainHelper.SetReplicationLagForTesting(5)
		require.NoError(t, err)
		defer mainHelper.SetReplicationLagForTesting(0)
		mainHelper.ToggleReplicasOn()
		defer mainHelper.ToggleReplicasOff()

		root, appErr := th.App.CreatePost(th.Context, &model.Post{
			UserID:    th.BasicUser.ID,
			ChannelID: th.BasicChannel.ID,
			Message:   "root post",
		}, th.BasicChannel, false, true)
		require.Nil(t, appErr)

		reply, appErr := th.App.CreatePost(th.Context, &model.Post{
			UserID:    th.BasicUser2.ID,
			ChannelID: th.BasicChannel.ID,
			RootID:    root.ID,
			ParentID:  root.ID,
			Message:   fmt.Sprintf("@%s", th.BasicUser2.Username),
		}, th.BasicChannel, false, true)
		require.Nil(t, appErr)
		require.NotNil(t, reply)
	})
}

func TestSharedChannelSyncForPostActions(t *testing.T) {
	t.Run("creating a post in a shared channel performs a content sync when sync service is running on that node", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		remoteClusterService := NewMockSharedChannelService(nil)
		th.App.srv.sharedChannelService = remoteClusterService
		testCluster := &testlib.FakeClusterInterface{}
		th.Server.Cluster = testCluster

		user := th.BasicUser

		channel := th.CreateChannel(th.BasicTeam, WithShared(true))

		_, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user.ID,
			ChannelID: channel.ID,
			Message:   "Hello folks",
		}, channel, false, true)
		require.Nil(t, err, "Creating a post should not error")

		assert.Len(t, remoteClusterService.channelNotifications, 1)
		assert.Equal(t, channel.ID, remoteClusterService.channelNotifications[0])
	})

	t.Run("updating a post in a shared channel performs a content sync when sync service is running on that node", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		remoteClusterService := NewMockSharedChannelService(nil)
		th.App.srv.sharedChannelService = remoteClusterService
		testCluster := &testlib.FakeClusterInterface{}
		th.Server.Cluster = testCluster

		user := th.BasicUser

		channel := th.CreateChannel(th.BasicTeam, WithShared(true))

		post, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user.ID,
			ChannelID: channel.ID,
			Message:   "Hello folks",
		}, channel, false, true)
		require.Nil(t, err, "Creating a post should not error")

		_, err = th.App.UpdatePost(th.Context, post, true)
		require.Nil(t, err, "Updating a post should not error")

		assert.Len(t, remoteClusterService.channelNotifications, 2)
		assert.Equal(t, channel.ID, remoteClusterService.channelNotifications[0])
		assert.Equal(t, channel.ID, remoteClusterService.channelNotifications[1])
	})

	t.Run("deleting a post in a shared channel performs a content sync when sync service is running on that node", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		remoteClusterService := NewMockSharedChannelService(nil)
		th.App.srv.sharedChannelService = remoteClusterService
		testCluster := &testlib.FakeClusterInterface{}
		th.Server.Cluster = testCluster

		user := th.BasicUser

		channel := th.CreateChannel(th.BasicTeam, WithShared(true))

		post, err := th.App.CreatePost(th.Context, &model.Post{
			UserID:    user.ID,
			ChannelID: channel.ID,
			Message:   "Hello folks",
		}, channel, false, true)
		require.Nil(t, err, "Creating a post should not error")

		_, err = th.App.DeletePost(post.ID, user.ID)
		require.Nil(t, err, "Deleting a post should not error")

		// one creation and two deletes
		assert.Len(t, remoteClusterService.channelNotifications, 3)
		assert.Equal(t, channel.ID, remoteClusterService.channelNotifications[0])
		assert.Equal(t, channel.ID, remoteClusterService.channelNotifications[1])
		assert.Equal(t, channel.ID, remoteClusterService.channelNotifications[2])
	})
}
