// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/utils/testutils"
)

func TestCreatePost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post := &model.Post{ChannelId: th.BasicChannel.Id, Message: "#hashtag a" + model.NewId() + "a", Props: model.StringInterface{model.PROPS_ADD_CHANNEL_MEMBER: "no good"}}
	rpost, resp := Client.CreatePost(post)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	require.Equal(t, post.Message, rpost.Message, "message didn't match")
	require.Equal(t, "#hashtag", rpost.Hashtags, "hashtag didn't match")
	require.Empty(t, rpost.FileIds)
	require.Equal(t, 0, int(rpost.EditAt), "newly created post shouldn't have EditAt set")
	require.Nil(t, rpost.GetProp(model.PROPS_ADD_CHANNEL_MEMBER), "newly created post shouldn't have Props['add_channel_member'] set")

	post.RootId = rpost.Id
	post.ParentId = rpost.Id
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.RootId = "junk"
	_, resp = Client.CreatePost(post)
	CheckBadRequestStatus(t, resp)

	post.RootId = rpost.Id
	post.ParentId = "junk"
	_, resp = Client.CreatePost(post)
	CheckBadRequestStatus(t, resp)

	post2 := &model.Post{ChannelId: th.BasicChannel2.Id, Message: "zz" + model.NewId() + "a", CreateAt: 123}
	rpost2, _ := Client.CreatePost(post2)
	require.NotEqual(t, post2.CreateAt, rpost2.CreateAt, "create at should not match")

	t.Run("with file uploaded by same user", func(t *testing.T) {
		fileResp, subResponse := Client.UploadFile([]byte("data"), th.BasicChannel.Id, "test")
		CheckNoError(t, subResponse)
		fileId := fileResp.FileInfos[0].Id

		postWithFiles, subResponse := Client.CreatePost(&model.Post{
			ChannelId: th.BasicChannel.Id,
			Message:   "with files",
			FileIds:   model.StringArray{fileId},
		})
		CheckNoError(t, subResponse)
		assert.Equal(t, model.StringArray{fileId}, postWithFiles.FileIds)

		actualPostWithFiles, subResponse := Client.GetPost(postWithFiles.Id, "")
		CheckNoError(t, subResponse)
		assert.Equal(t, model.StringArray{fileId}, actualPostWithFiles.FileIds)
	})

	t.Run("with file uploaded by different user", func(t *testing.T) {
		fileResp, subResponse := th.SystemAdminClient.UploadFile([]byte("data"), th.BasicChannel.Id, "test")
		CheckNoError(t, subResponse)
		fileId := fileResp.FileInfos[0].Id

		postWithFiles, subResponse := Client.CreatePost(&model.Post{
			ChannelId: th.BasicChannel.Id,
			Message:   "with files",
			FileIds:   model.StringArray{fileId},
		})
		CheckNoError(t, subResponse)
		assert.Empty(t, postWithFiles.FileIds)

		actualPostWithFiles, subResponse := Client.GetPost(postWithFiles.Id, "")
		CheckNoError(t, subResponse)
		assert.Empty(t, actualPostWithFiles.FileIds)
	})

	t.Run("with file uploaded by nouser", func(t *testing.T) {
		fileInfo, err := th.App.UploadFile(th.Context, []byte("data"), th.BasicChannel.Id, "test")
		require.Nil(t, err)
		fileId := fileInfo.Id

		postWithFiles, subResponse := Client.CreatePost(&model.Post{
			ChannelId: th.BasicChannel.Id,
			Message:   "with files",
			FileIds:   model.StringArray{fileId},
		})
		CheckNoError(t, subResponse)
		assert.Equal(t, model.StringArray{fileId}, postWithFiles.FileIds)

		actualPostWithFiles, subResponse := Client.GetPost(postWithFiles.Id, "")
		CheckNoError(t, subResponse)
		assert.Equal(t, model.StringArray{fileId}, actualPostWithFiles.FileIds)
	})

	t.Run("Create posts without the USE_CHANNEL_MENTIONS Permission - returns ephemeral message with mentions and no ephemeral message without mentions", func(t *testing.T) {
		WebSocketClient, err := th.CreateWebSocketClient()
		WebSocketClient.Listen()
		require.Nil(t, err)

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.RemovePermissionFromRole(model.PERMISSION_USE_CHANNEL_MENTIONS.Id, model.CHANNEL_USER_ROLE_ID)

		post.RootId = rpost.Id
		post.ParentId = rpost.Id
		post.Message = "a post with no channel mentions"
		_, resp = Client.CreatePost(post)
		CheckNoError(t, resp)

		// Message with no channel mentions should result in no ephemeral message
		timeout := time.After(300 * time.Millisecond)
		waiting := true
		for waiting {
			select {
			case event := <-WebSocketClient.EventChannel:
				require.NotEqual(t, model.WEBSOCKET_EVENT_EPHEMERAL_MESSAGE, event.EventType(), "should not have ephemeral message event")
			case <-timeout:
				waiting = false
			}
		}

		post.RootId = rpost.Id
		post.ParentId = rpost.Id
		post.Message = "a post with @channel"
		_, resp = Client.CreatePost(post)
		CheckNoError(t, resp)

		post.RootId = rpost.Id
		post.ParentId = rpost.Id
		post.Message = "a post with @all"
		_, resp = Client.CreatePost(post)
		CheckNoError(t, resp)

		post.RootId = rpost.Id
		post.ParentId = rpost.Id
		post.Message = "a post with @here"
		_, resp = Client.CreatePost(post)
		CheckNoError(t, resp)

		timeout = time.After(600 * time.Millisecond)
		eventsToGo := 3 // 3 Posts created with @ mentions should result in 3 websocket events
		for eventsToGo > 0 {
			select {
			case event := <-WebSocketClient.EventChannel:
				if event.Event == model.WEBSOCKET_EVENT_EPHEMERAL_MESSAGE {
					require.Equal(t, model.WEBSOCKET_EVENT_EPHEMERAL_MESSAGE, event.Event)
					eventsToGo = eventsToGo - 1
				}
			case <-timeout:
				require.Fail(t, "Should have received ephemeral message event and not timedout")
				eventsToGo = 0
			}
		}
	})

	post.RootId = ""
	post.ParentId = ""
	post.Type = model.POST_SYSTEM_GENERIC
	_, resp = Client.CreatePost(post)
	CheckBadRequestStatus(t, resp)

	post.Type = ""
	post.RootId = rpost2.Id
	post.ParentId = rpost2.Id
	_, resp = Client.CreatePost(post)
	CheckBadRequestStatus(t, resp)

	post.RootId = ""
	post.ParentId = ""
	post.ChannelId = "junk"
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	post.ChannelId = model.NewId()
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	r, err := Client.DoApiPost("/posts", "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	Client.Logout()
	_, resp = Client.CreatePost(post)
	CheckUnauthorizedStatus(t, resp)

	post.ChannelId = th.BasicChannel.Id
	post.CreateAt = 123
	rpost, resp = th.SystemAdminClient.CreatePost(post)
	CheckNoError(t, resp)
	require.Equal(t, post.CreateAt, rpost.CreateAt, "create at should match")
}

func TestCreatePostEphemeral(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.SystemAdminClient

	ephemeralPost := &model.PostEphemeral{
		UserID: th.BasicUser2.Id,
		Post:   &model.Post{ChannelId: th.BasicChannel.Id, Message: "a" + model.NewId() + "a", Props: model.StringInterface{model.PROPS_ADD_CHANNEL_MEMBER: "no good"}},
	}

	rpost, resp := Client.CreatePostEphemeral(ephemeralPost)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.Equal(t, ephemeralPost.Post.Message, rpost.Message, "message didn't match")
	require.Equal(t, 0, int(rpost.EditAt), "newly created ephemeral post shouldn't have EditAt set")

	r, err := Client.DoApiPost("/posts/ephemeral", "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	Client.Logout()
	_, resp = Client.CreatePostEphemeral(ephemeralPost)
	CheckUnauthorizedStatus(t, resp)

	Client = th.Client
	_, resp = Client.CreatePostEphemeral(ephemeralPost)
	CheckForbiddenStatus(t, resp)
}

func testCreatePostWithOutgoingHook(
	t *testing.T,
	hookContentType, expectedContentType, message, triggerWord string,
	fileIds []string,
	triggerWhen int,
	commentPostType bool,
) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.SystemAdminUser
	team := th.BasicTeam
	channel := th.BasicChannel

	enableOutgoingWebhooks := *th.App.Config().ServiceSettings.EnableOutgoingWebhooks
	allowedUntrustedInternalConnections := *th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = enableOutgoingWebhooks })
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = allowedUntrustedInternalConnections
		})
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	var hook *model.OutgoingWebhook
	var post *model.Post

	// Create a test server that is the target of the outgoing webhook. It will
	// validate the webhook body fields and write to the success channel on
	// success/failure.
	success := make(chan bool)
	wait := make(chan bool, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-wait

		requestContentType := r.Header.Get("Content-Type")
		if requestContentType != expectedContentType {
			t.Logf("Content-Type is %s, should be %s", requestContentType, expectedContentType)
			success <- false
			return
		}

		expectedPayload := &model.OutgoingWebhookPayload{
			Token:       hook.Token,
			TeamId:      hook.TeamId,
			TeamDomain:  team.Name,
			ChannelId:   post.ChannelId,
			ChannelName: channel.Name,
			Timestamp:   post.CreateAt,
			UserId:      post.UserId,
			UserName:    user.Username,
			PostId:      post.Id,
			Text:        post.Message,
			TriggerWord: triggerWord,
			FileIds:     strings.Join(post.FileIds, ","),
		}

		// depending on the Content-Type, we expect to find a JSON or form encoded payload
		if requestContentType == "application/json" {
			decoder := json.NewDecoder(r.Body)
			o := &model.OutgoingWebhookPayload{}
			decoder.Decode(&o)

			if !reflect.DeepEqual(expectedPayload, o) {
				t.Logf("JSON payload is %+v, should be %+v", o, expectedPayload)
				success <- false
				return
			}
		} else {
			err := r.ParseForm()
			if err != nil {
				t.Logf("Error parsing form: %q", err)
				success <- false
				return
			}

			expectedFormValues, _ := url.ParseQuery(expectedPayload.ToFormValues())

			if !reflect.DeepEqual(expectedFormValues, r.Form) {
				t.Logf("Form values are: %q\n, should be: %q\n", r.Form, expectedFormValues)
				success <- false
				return
			}
		}

		respPostType := "" //if is empty or post will do a normal post.
		if commentPostType {
			respPostType = model.OUTGOING_HOOK_RESPONSE_TYPE_COMMENT
		}

		outGoingHookResponse := &model.OutgoingWebhookResponse{
			Text:         model.NewString("some test text"),
			Username:     "TestCommandServer",
			IconURL:      "https://www.mattermost.org/wp-content/uploads/2016/04/icon.png",
			Type:         "custom_as",
			ResponseType: respPostType,
		}

		fmt.Fprintf(w, outGoingHookResponse.ToJson())
		success <- true
	}))
	defer ts.Close()

	// create an outgoing webhook, passing it the test server URL
	var triggerWords []string
	if triggerWord != "" {
		triggerWords = []string{triggerWord}
	}

	hook = &model.OutgoingWebhook{
		ChannelId:    channel.Id,
		TeamId:       team.Id,
		ContentType:  hookContentType,
		TriggerWords: triggerWords,
		TriggerWhen:  triggerWhen,
		CallbackURLs: []string{ts.URL},
	}

	hook, resp := th.SystemAdminClient.CreateOutgoingWebhook(hook)
	CheckNoError(t, resp)

	// create a post to trigger the webhook
	post = &model.Post{
		ChannelId: channel.Id,
		Message:   message,
		FileIds:   fileIds,
	}

	post, resp = th.SystemAdminClient.CreatePost(post)
	CheckNoError(t, resp)

	wait <- true

	// We wait for the test server to write to the success channel and we make
	// the test fail if that doesn't happen before the timeout.
	select {
	case ok := <-success:
		require.True(t, ok, "Test server did send an invalid webhook.")
	case <-time.After(time.Second):
		require.FailNow(t, "Timeout, test server did not send the webhook.")
	}

	if commentPostType {
		time.Sleep(time.Millisecond * 100)
		postList, resp := th.SystemAdminClient.GetPostThread(post.Id, "", false)
		CheckNoError(t, resp)
		require.Equal(t, post.Id, postList.Order[0], "wrong order")

		_, ok := postList.Posts[post.Id]
		require.True(t, ok, "should have had post")
		require.Len(t, postList.Posts, 2, "should have 2 posts")
	}
}

func TestCreatePostWithOutgoingHook_form_urlencoded(t *testing.T) {
	testCreatePostWithOutgoingHook(t, "application/x-www-form-urlencoded", "application/x-www-form-urlencoded", "triggerword lorem ipsum", "triggerword", []string{"file_id_1"}, app.TriggerwordsExactMatch, false)
	testCreatePostWithOutgoingHook(t, "application/x-www-form-urlencoded", "application/x-www-form-urlencoded", "triggerwordaaazzz lorem ipsum", "triggerword", []string{"file_id_1"}, app.TriggerwordsStartsWith, false)
	testCreatePostWithOutgoingHook(t, "application/x-www-form-urlencoded", "application/x-www-form-urlencoded", "", "", []string{"file_id_1"}, app.TriggerwordsExactMatch, false)
	testCreatePostWithOutgoingHook(t, "application/x-www-form-urlencoded", "application/x-www-form-urlencoded", "", "", []string{"file_id_1"}, app.TriggerwordsStartsWith, false)
	testCreatePostWithOutgoingHook(t, "application/x-www-form-urlencoded", "application/x-www-form-urlencoded", "triggerword lorem ipsum", "triggerword", []string{"file_id_1"}, app.TriggerwordsExactMatch, true)
	testCreatePostWithOutgoingHook(t, "application/x-www-form-urlencoded", "application/x-www-form-urlencoded", "triggerwordaaazzz lorem ipsum", "triggerword", []string{"file_id_1"}, app.TriggerwordsStartsWith, true)
}

func TestCreatePostWithOutgoingHook_json(t *testing.T) {
	testCreatePostWithOutgoingHook(t, "application/json", "application/json", "triggerword lorem ipsum", "triggerword", []string{"file_id_1, file_id_2"}, app.TriggerwordsExactMatch, false)
	testCreatePostWithOutgoingHook(t, "application/json", "application/json", "triggerwordaaazzz lorem ipsum", "triggerword", []string{"file_id_1, file_id_2"}, app.TriggerwordsStartsWith, false)
	testCreatePostWithOutgoingHook(t, "application/json", "application/json", "triggerword lorem ipsum", "", []string{"file_id_1"}, app.TriggerwordsExactMatch, false)
	testCreatePostWithOutgoingHook(t, "application/json", "application/json", "triggerwordaaazzz lorem ipsum", "", []string{"file_id_1"}, app.TriggerwordsStartsWith, false)
	testCreatePostWithOutgoingHook(t, "application/json", "application/json", "triggerword lorem ipsum", "triggerword", []string{"file_id_1, file_id_2"}, app.TriggerwordsExactMatch, true)
	testCreatePostWithOutgoingHook(t, "application/json", "application/json", "triggerwordaaazzz lorem ipsum", "", []string{"file_id_1"}, app.TriggerwordsStartsWith, true)
}

// hooks created before we added the ContentType field should be considered as
// application/x-www-form-urlencoded
func TestCreatePostWithOutgoingHook_no_content_type(t *testing.T) {
	testCreatePostWithOutgoingHook(t, "", "application/x-www-form-urlencoded", "triggerword lorem ipsum", "triggerword", []string{"file_id_1"}, app.TriggerwordsExactMatch, false)
	testCreatePostWithOutgoingHook(t, "", "application/x-www-form-urlencoded", "triggerwordaaazzz lorem ipsum", "triggerword", []string{"file_id_1"}, app.TriggerwordsStartsWith, false)
	testCreatePostWithOutgoingHook(t, "", "application/x-www-form-urlencoded", "triggerword lorem ipsum", "", []string{"file_id_1, file_id_2"}, app.TriggerwordsExactMatch, false)
	testCreatePostWithOutgoingHook(t, "", "application/x-www-form-urlencoded", "triggerwordaaazzz lorem ipsum", "", []string{"file_id_1, file_id_2"}, app.TriggerwordsStartsWith, false)
	testCreatePostWithOutgoingHook(t, "", "application/x-www-form-urlencoded", "triggerword lorem ipsum", "triggerword", []string{"file_id_1"}, app.TriggerwordsExactMatch, true)
	testCreatePostWithOutgoingHook(t, "", "application/x-www-form-urlencoded", "triggerword lorem ipsum", "", []string{"file_id_1, file_id_2"}, app.TriggerwordsExactMatch, true)
}

func TestCreatePostPublic(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post := &model.Post{ChannelId: th.BasicChannel.Id, Message: "#hashtag a" + model.NewId() + "a"}

	user := model.User{Email: th.GenerateTestEmail(), Nickname: "Joram Wilander", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_USER_ROLE_ID}

	ruser, resp := Client.CreateUser(&user)
	CheckNoError(t, resp)

	Client.Login(user.Email, user.Password)

	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	th.App.UpdateUserRoles(ruser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_POST_ALL_PUBLIC_ROLE_ID, false)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Email, user.Password)

	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelId = th.BasicPrivateChannel.Id
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	th.App.UpdateUserRoles(ruser.Id, model.SYSTEM_USER_ROLE_ID, false)
	th.App.JoinUserToTeam(th.Context, th.BasicTeam, ruser, "")
	th.App.UpdateTeamMemberRoles(th.BasicTeam.Id, ruser.Id, model.TEAM_USER_ROLE_ID+" "+model.TEAM_POST_ALL_PUBLIC_ROLE_ID)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Email, user.Password)

	post.ChannelId = th.BasicPrivateChannel.Id
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	post.ChannelId = th.BasicChannel.Id
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)
}

func TestCreatePostAll(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post := &model.Post{ChannelId: th.BasicChannel.Id, Message: "#hashtag a" + model.NewId() + "a"}

	user := model.User{Email: th.GenerateTestEmail(), Nickname: "Joram Wilander", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_USER_ROLE_ID}

	directChannel, _ := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, th.BasicUser2.Id)

	ruser, resp := Client.CreateUser(&user)
	CheckNoError(t, resp)

	Client.Login(user.Email, user.Password)

	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	th.App.UpdateUserRoles(ruser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_POST_ALL_ROLE_ID, false)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Email, user.Password)

	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelId = th.BasicPrivateChannel.Id
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelId = directChannel.Id
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	th.App.UpdateUserRoles(ruser.Id, model.SYSTEM_USER_ROLE_ID, false)
	th.App.JoinUserToTeam(th.Context, th.BasicTeam, ruser, "")
	th.App.UpdateTeamMemberRoles(th.BasicTeam.Id, ruser.Id, model.TEAM_USER_ROLE_ID+" "+model.TEAM_POST_ALL_ROLE_ID)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Email, user.Password)

	post.ChannelId = th.BasicPrivateChannel.Id
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelId = th.BasicChannel.Id
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelId = directChannel.Id
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)
}

func TestCreatePostSendOutOfChannelMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	WebSocketClient.Listen()

	inChannelUser := th.CreateUser()
	th.LinkUserToTeam(inChannelUser, th.BasicTeam)
	th.App.AddUserToChannel(inChannelUser, th.BasicChannel, false)

	post1 := &model.Post{ChannelId: th.BasicChannel.Id, Message: "@" + inChannelUser.Username}
	_, resp := Client.CreatePost(post1)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	timeout := time.After(300 * time.Millisecond)
	waiting := true
	for waiting {
		select {
		case event := <-WebSocketClient.EventChannel:
			require.NotEqual(t, model.WEBSOCKET_EVENT_EPHEMERAL_MESSAGE, event.EventType(), "should not have ephemeral message event")
		case <-timeout:
			waiting = false
		}
	}

	outOfChannelUser := th.CreateUser()
	th.LinkUserToTeam(outOfChannelUser, th.BasicTeam)

	post2 := &model.Post{ChannelId: th.BasicChannel.Id, Message: "@" + outOfChannelUser.Username}
	_, resp = Client.CreatePost(post2)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	timeout = time.After(300 * time.Millisecond)
	waiting = true
	for waiting {
		select {
		case event := <-WebSocketClient.EventChannel:
			if event.EventType() != model.WEBSOCKET_EVENT_EPHEMERAL_MESSAGE {
				// Ignore any other events
				continue
			}

			wpost := model.PostFromJson(strings.NewReader(event.GetData()["post"].(string)))

			acm, ok := wpost.GetProp(model.PROPS_ADD_CHANNEL_MEMBER).(map[string]interface{})
			require.True(t, ok, "should have received ephemeral post with 'add_channel_member' in props")
			require.True(t, acm["post_id"] != nil, "should not be nil")
			require.True(t, acm["user_ids"] != nil, "should not be nil")
			require.True(t, acm["usernames"] != nil, "should not be nil")
			waiting = false
		case <-timeout:
			require.FailNow(t, "timed out waiting for ephemeral message event")
		}
	}
}

func TestCreatePostCheckOnlineStatus(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	api := Init(th.App, th.Server.Router)
	session, _ := th.App.GetSession(th.Client.AuthToken)

	cli := th.CreateClient()
	_, loginResp := cli.Login(th.BasicUser2.Username, th.BasicUser2.Password)
	require.Nil(t, loginResp.Error)

	wsClient, err := th.CreateWebSocketClientWithClient(cli)
	require.Nil(t, err)
	defer wsClient.Close()

	wsClient.Listen()

	waitForEvent := func(isSetOnline bool) {
		timeout := time.After(5 * time.Second)
		for {
			select {
			case ev := <-wsClient.EventChannel:
				if ev.EventType() == model.WEBSOCKET_EVENT_POSTED {
					assert.True(t, ev.GetData()["set_online"].(bool) == isSetOnline)
					return
				}
			case <-timeout:
				// We just skip the test instead of failing because waiting for more than 5 seconds
				// to get a response does not make sense, and it will unnecessarily slow down
				// the tests further in an already congested CI environment.
				t.Skip("timed out waiting for event")
			}
		}
	}

	handler := api.ApiHandler(createPost)
	resp := httptest.NewRecorder()
	post := &model.Post{
		ChannelId: th.BasicChannel.Id,
		Message:   "some message",
	}

	req := httptest.NewRequest("POST", "/api/v4/posts?set_online=false", strings.NewReader(post.ToJson()))
	req.Header.Set(model.HEADER_AUTH, "Bearer "+session.Token)

	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusCreated, resp.Code)
	waitForEvent(false)

	_, err = th.App.GetStatus(th.BasicUser.Id)
	require.NotNil(t, err)
	assert.Equal(t, "app.status.get.missing.app_error", err.Id)

	req = httptest.NewRequest("POST", "/api/v4/posts", strings.NewReader(post.ToJson()))
	req.Header.Set(model.HEADER_AUTH, "Bearer "+session.Token)

	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusCreated, resp.Code)
	waitForEvent(true)

	st, err := th.App.GetStatus(th.BasicUser.Id)
	require.Nil(t, err)
	assert.Equal(t, "online", st.Status)
}

func TestUpdatePost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	th.App.Srv().SetLicense(model.NewTestLicense())

	fileIds := make([]string, 3)
	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)
	for i := 0; i < len(fileIds); i++ {
		fileResp, resp := Client.UploadFile(data, channel.Id, "test.png")
		CheckNoError(t, resp)
		fileIds[i] = fileResp.FileInfos[0].Id
	}

	rpost, appErr := th.App.CreatePost(th.Context, &model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: channel.Id,
		Message:   "zz" + model.NewId() + "a",
		FileIds:   fileIds,
	}, channel, false, true)
	require.Nil(t, appErr)

	assert.Equal(t, rpost.Message, rpost.Message, "full name didn't match")
	assert.EqualValues(t, 0, rpost.EditAt, "Newly created post shouldn't have EditAt set")
	assert.Equal(t, model.StringArray(fileIds), rpost.FileIds, "FileIds should have been set")

	t.Run("same message, fewer files", func(t *testing.T) {
		msg := "zz" + model.NewId() + " update post"
		rpost.Message = msg
		rpost.UserId = ""

		rupost, resp := Client.UpdatePost(rpost.Id, &model.Post{
			Id:      rpost.Id,
			Message: rpost.Message,
			FileIds: fileIds[0:2], // one fewer file id
		})
		CheckNoError(t, resp)

		assert.Equal(t, rupost.Message, msg, "failed to updates")
		assert.NotEqual(t, 0, rupost.EditAt, "EditAt not updated for post")
		assert.Equal(t, model.StringArray(fileIds), rupost.FileIds, "FileIds should have not have been updated")

		actual, resp := Client.GetPost(rpost.Id, "")
		CheckNoError(t, resp)

		assert.Equal(t, actual.Message, msg, "failed to updates")
		assert.NotEqual(t, 0, actual.EditAt, "EditAt not updated for post")
		assert.Equal(t, model.StringArray(fileIds), actual.FileIds, "FileIds should have not have been updated")
	})

	t.Run("new message, invalid props", func(t *testing.T) {
		msg1 := "#hashtag a" + model.NewId() + " update post again"
		rpost.Message = msg1
		rpost.AddProp(model.PROPS_ADD_CHANNEL_MEMBER, "no good")
		rrupost, resp := Client.UpdatePost(rpost.Id, rpost)
		CheckNoError(t, resp)

		assert.Equal(t, msg1, rrupost.Message, "failed to update message")
		assert.Equal(t, "#hashtag", rrupost.Hashtags, "failed to update hashtags")
		assert.Nil(t, rrupost.GetProp(model.PROPS_ADD_CHANNEL_MEMBER), "failed to sanitize Props['add_channel_member'], should be nil")

		actual, resp := Client.GetPost(rpost.Id, "")
		CheckNoError(t, resp)

		assert.Equal(t, msg1, actual.Message, "failed to update message")
		assert.Equal(t, "#hashtag", actual.Hashtags, "failed to update hashtags")
		assert.Nil(t, actual.GetProp(model.PROPS_ADD_CHANNEL_MEMBER), "failed to sanitize Props['add_channel_member'], should be nil")
	})

	t.Run("join/leave post", func(t *testing.T) {
		rpost2, err := th.App.CreatePost(th.Context, &model.Post{
			ChannelId: channel.Id,
			Message:   "zz" + model.NewId() + "a",
			Type:      model.POST_JOIN_LEAVE,
			UserId:    th.BasicUser.Id,
		}, channel, false, true)
		require.Nil(t, err)

		up2 := &model.Post{
			Id:        rpost2.Id,
			ChannelId: channel.Id,
			Message:   "zz" + model.NewId() + " update post 2",
		}
		_, resp := Client.UpdatePost(rpost2.Id, up2)
		CheckBadRequestStatus(t, resp)
	})

	rpost3, appErr := th.App.CreatePost(th.Context, &model.Post{
		ChannelId: channel.Id,
		Message:   "zz" + model.NewId() + "a",
		UserId:    th.BasicUser.Id,
	}, channel, false, true)
	require.Nil(t, appErr)

	t.Run("new message, add files", func(t *testing.T) {
		up3 := &model.Post{
			Id:        rpost3.Id,
			ChannelId: channel.Id,
			Message:   "zz" + model.NewId() + " update post 3",
			FileIds:   fileIds[0:2],
		}
		rrupost3, resp := Client.UpdatePost(rpost3.Id, up3)
		CheckNoError(t, resp)
		assert.Empty(t, rrupost3.FileIds)

		actual, resp := Client.GetPost(rpost.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, model.StringArray(fileIds), actual.FileIds)
	})

	t.Run("add slack attachments", func(t *testing.T) {
		up4 := &model.Post{
			Id:        rpost3.Id,
			ChannelId: channel.Id,
			Message:   "zz" + model.NewId() + " update post 3",
		}
		up4.AddProp("attachments", []model.SlackAttachment{
			{
				Text: "Hello World",
			},
		})
		rrupost3, resp := Client.UpdatePost(rpost3.Id, up4)
		CheckNoError(t, resp)
		assert.NotEqual(t, rpost3.EditAt, rrupost3.EditAt)
		assert.NotEqual(t, rpost3.Attachments(), rrupost3.Attachments())
	})

	t.Run("logged out", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.UpdatePost(rpost.Id, rpost)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("different user", func(t *testing.T) {
		th.LoginBasic2()
		_, resp := Client.UpdatePost(rpost.Id, rpost)
		CheckForbiddenStatus(t, resp)

		Client.Logout()
	})

	t.Run("different user, but team admin", func(t *testing.T) {
		th.LoginTeamAdmin()
		_, resp := Client.UpdatePost(rpost.Id, rpost)
		CheckForbiddenStatus(t, resp)

		Client.Logout()
	})

	t.Run("different user, but system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.UpdatePost(rpost.Id, rpost)
		CheckNoError(t, resp)
	})
}

func TestUpdateOthersPostInDirectMessageChannel(t *testing.T) {
	// This test checks that a sysadmin with the "EDIT_OTHERS_POSTS" permission can edit someone else's post in a
	// channel without a team (DM/GM). This indirectly checks for the proper cascading all the way to system-wide roles
	// on the user object of permissions based on a post in a channel with no team ID.
	th := Setup(t).InitBasic()
	defer th.TearDown()

	dmChannel := th.CreateDmChannel(th.SystemAdminUser)

	post := &model.Post{
		Message:       "asd",
		ChannelId:     dmChannel.Id,
		PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
		UserId:        th.BasicUser.Id,
		CreateAt:      0,
	}

	post, resp := th.Client.CreatePost(post)
	CheckNoError(t, resp)

	post.Message = "changed"
	post, resp = th.SystemAdminClient.UpdatePost(post.Id, post)
	CheckNoError(t, resp)
}

func TestPatchPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	th.App.Srv().SetLicense(model.NewTestLicense())

	fileIds := make([]string, 3)
	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)
	for i := 0; i < len(fileIds); i++ {
		fileResp, resp := Client.UploadFile(data, channel.Id, "test.png")
		CheckNoError(t, resp)
		fileIds[i] = fileResp.FileInfos[0].Id
	}

	post := &model.Post{
		ChannelId:    channel.Id,
		IsPinned:     true,
		Message:      "#hashtag a message",
		Props:        model.StringInterface{"channel_header": "old_header"},
		FileIds:      fileIds[0:2],
		HasReactions: true,
	}
	post, _ = Client.CreatePost(post)

	var rpost *model.Post
	t.Run("new message, props, files, HasReactions bit", func(t *testing.T) {
		patch := &model.PostPatch{}

		patch.IsPinned = model.NewBool(false)
		patch.Message = model.NewString("#otherhashtag other message")
		patch.Props = &model.StringInterface{"channel_header": "new_header"}
		patchFileIds := model.StringArray(fileIds) // one extra file
		patch.FileIds = &patchFileIds
		patch.HasReactions = model.NewBool(false)

		var resp *model.Response
		rpost, resp = Client.PatchPost(post.Id, patch)
		CheckNoError(t, resp)

		assert.False(t, rpost.IsPinned, "IsPinned did not update properly")
		assert.Equal(t, "#otherhashtag other message", rpost.Message, "Message did not update properly")
		assert.Equal(t, *patch.Props, rpost.GetProps(), "Props did not update properly")
		assert.Equal(t, "#otherhashtag", rpost.Hashtags, "Message did not update properly")
		assert.Equal(t, model.StringArray(fileIds[0:2]), rpost.FileIds, "FileIds should not update")
		assert.False(t, rpost.HasReactions, "HasReactions did not update properly")
	})

	t.Run("add slack attachments", func(t *testing.T) {
		patch2 := &model.PostPatch{}
		attachments := []model.SlackAttachment{
			{
				Text: "Hello World",
			},
		}
		patch2.Props = &model.StringInterface{"attachments": attachments}

		rpost2, resp := Client.PatchPost(post.Id, patch2)
		CheckNoError(t, resp)
		assert.NotEmpty(t, rpost2.GetProp("attachments"))
		assert.NotEqual(t, rpost.EditAt, rpost2.EditAt)
	})

	t.Run("invalid requests", func(t *testing.T) {
		r, err := Client.DoApiPut("/posts/"+post.Id+"/patch", "garbage")
		require.EqualError(t, err, ": Invalid or missing post in request body., ")
		require.Equal(t, http.StatusBadRequest, r.StatusCode, "wrong status code")

		patch := &model.PostPatch{}
		_, resp := Client.PatchPost("junk", patch)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("unknown post", func(t *testing.T) {
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(GenerateTestId(), patch)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("logged out", func(t *testing.T) {
		Client.Logout()
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(post.Id, patch)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("different user", func(t *testing.T) {
		th.LoginBasic2()
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(post.Id, patch)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("different user, but team admin", func(t *testing.T) {
		th.LoginTeamAdmin()
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(post.Id, patch)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("different user, but system admin", func(t *testing.T) {
		patch := &model.PostPatch{}
		_, resp := th.SystemAdminClient.PatchPost(post.Id, patch)
		CheckNoError(t, resp)
	})

	t.Run("edit others posts permission can function independently of edit own post", func(t *testing.T) {
		th.LoginBasic2()
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(post.Id, patch)
		CheckForbiddenStatus(t, resp)

		// Add permission to edit others'
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.RemovePermissionFromRole(model.PERMISSION_EDIT_POST.Id, model.CHANNEL_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_EDIT_OTHERS_POSTS.Id, model.CHANNEL_USER_ROLE_ID)

		_, resp = Client.PatchPost(post.Id, patch)
		CheckNoError(t, resp)
	})
}

func TestPinPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post := th.BasicPost
	pass, resp := Client.PinPost(post.Id)
	CheckNoError(t, resp)

	require.True(t, pass, "should have passed")
	rpost, err := th.App.GetSinglePost(post.Id)
	require.Nil(t, err)
	require.True(t, rpost.IsPinned, "failed to pin post")

	pass, resp = Client.PinPost("junk")
	CheckBadRequestStatus(t, resp)
	require.False(t, pass, "should have failed")

	_, resp = Client.PinPost(GenerateTestId())
	CheckForbiddenStatus(t, resp)

	t.Run("unable-to-pin-post-in-read-only-town-square", func(t *testing.T) {
		townSquareIsReadOnly := *th.App.Config().TeamSettings.ExperimentalTownSquareIsReadOnly
		th.App.Srv().SetLicense(model.NewTestLicense())
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.ExperimentalTownSquareIsReadOnly = true })

		defer th.App.Srv().RemoveLicense()
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.ExperimentalTownSquareIsReadOnly = townSquareIsReadOnly })

		channel, err := th.App.GetChannelByName("town-square", th.BasicTeam.Id, true)
		assert.Nil(t, err)
		adminPost := th.CreatePostWithClient(th.SystemAdminClient, channel)

		_, resp = Client.PinPost(adminPost.Id)
		CheckForbiddenStatus(t, resp)
	})

	Client.Logout()
	_, resp = Client.PinPost(post.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.PinPost(post.Id)
	CheckNoError(t, resp)
}

func TestUnpinPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	pinnedPost := th.CreatePinnedPost()
	pass, resp := Client.UnpinPost(pinnedPost.Id)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	rpost, err := th.App.GetSinglePost(pinnedPost.Id)
	require.Nil(t, err)
	require.False(t, rpost.IsPinned)

	pass, resp = Client.UnpinPost("junk")
	CheckBadRequestStatus(t, resp)
	require.False(t, pass, "should have failed")

	_, resp = Client.UnpinPost(GenerateTestId())
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.UnpinPost(pinnedPost.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.UnpinPost(pinnedPost.Id)
	CheckNoError(t, resp)
}

func TestGetPostsForChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post1 := th.CreatePost()
	post2 := th.CreatePost()
	post3 := &model.Post{ChannelId: th.BasicChannel.Id, Message: "zz" + model.NewId() + "a", RootId: post1.Id}
	post3, _ = Client.CreatePost(post3)

	time.Sleep(300 * time.Millisecond)
	since := model.GetMillis()
	time.Sleep(300 * time.Millisecond)

	post4 := th.CreatePost()

	th.TestForAllClients(t, func(t *testing.T, c *model.Client4) {
		posts, resp := c.GetPostsForChannel(th.BasicChannel.Id, 0, 60, "", false)
		CheckNoError(t, resp)
		require.Equal(t, post4.Id, posts.Order[0], "wrong order")
		require.Equal(t, post3.Id, posts.Order[1], "wrong order")
		require.Equal(t, post2.Id, posts.Order[2], "wrong order")
		require.Equal(t, post1.Id, posts.Order[3], "wrong order")

		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 0, 3, resp.Etag, false)
		CheckEtag(t, posts, resp)

		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 0, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "wrong number returned")

		_, ok := posts.Posts[post3.Id]
		require.True(t, ok, "missing comment")
		_, ok = posts.Posts[post1.Id]
		require.True(t, ok, "missing root post")

		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 1, 1, "", false)
		CheckNoError(t, resp)
		require.Equal(t, post3.Id, posts.Order[0], "wrong order")

		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 10000, 10000, "", false)
		CheckNoError(t, resp)
		require.Empty(t, posts.Order, "should be no posts")
	})

	post5 := th.CreatePost()

	th.TestForAllClients(t, func(t *testing.T, c *model.Client4) {
		posts, resp := c.GetPostsSince(th.BasicChannel.Id, since, false)
		CheckNoError(t, resp)
		require.Len(t, posts.Posts, 2, "should return 2 posts")

		// "since" query to return empty NextPostId and PrevPostId
		require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
		require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")

		found := make([]bool, 2)
		for _, p := range posts.Posts {
			require.LessOrEqual(t, since, p.CreateAt, "bad create at for post returned")

			if p.Id == post4.Id {
				found[0] = true
			} else if p.Id == post5.Id {
				found[1] = true
			}
		}
		for _, f := range found {
			require.True(t, f, "missing post")
		}

		_, resp = c.GetPostsForChannel("", 0, 60, "", false)
		CheckBadRequestStatus(t, resp)

		_, resp = c.GetPostsForChannel("junk", 0, 60, "", false)
		CheckBadRequestStatus(t, resp)
	})

	_, resp := Client.GetPostsForChannel(model.NewId(), 0, 60, "", false)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPostsForChannel(model.NewId(), 0, 60, "", false)
	CheckUnauthorizedStatus(t, resp)

	// more tests for next_post_id, prev_post_id, and order
	// There are 12 posts composed of first 2 system messages and 10 created posts
	Client.Login(th.BasicUser.Email, th.BasicUser.Password)
	th.CreatePost() // post6
	post7 := th.CreatePost()
	post8 := th.CreatePost()
	th.CreatePost() // post9
	post10 := th.CreatePost()

	var posts *model.PostList
	th.TestForAllClients(t, func(t *testing.T, c *model.Client4) {
		// get the system post IDs posted before the created posts above
		posts, resp = c.GetPostsBefore(th.BasicChannel.Id, post1.Id, 0, 2, "", false)
		systemPostId1 := posts.Order[1]

		// similar to '/posts'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 0, 60, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 12, "expected 12 posts")
		require.Equal(t, post10.Id, posts.Order[0], "posts not in order")
		require.Equal(t, systemPostId1, posts.Order[11], "posts not in order")
		require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
		require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")

		// similar to '/posts?per_page=3'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 0, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "expected 3 posts")
		require.Equal(t, post10.Id, posts.Order[0], "posts not in order")
		require.Equal(t, post8.Id, posts.Order[2], "should return 3 posts and match order")
		require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
		require.Equal(t, post7.Id, posts.PrevPostId, "should return post7.Id as PrevPostId")

		// similar to '/posts?per_page=3&page=1'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 1, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "expected 3 posts")
		require.Equal(t, post7.Id, posts.Order[0], "posts not in order")
		require.Equal(t, post5.Id, posts.Order[2], "posts not in order")
		require.Equal(t, post8.Id, posts.NextPostId, "should return post8.Id as NextPostId")
		require.Equal(t, post4.Id, posts.PrevPostId, "should return post4.Id as PrevPostId")

		// similar to '/posts?per_page=3&page=2'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 2, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "expected 3 posts")
		require.Equal(t, post4.Id, posts.Order[0], "posts not in order")
		require.Equal(t, post2.Id, posts.Order[2], "should return 3 posts and match order")
		require.Equal(t, post5.Id, posts.NextPostId, "should return post5.Id as NextPostId")
		require.Equal(t, post1.Id, posts.PrevPostId, "should return post1.Id as PrevPostId")

		// similar to '/posts?per_page=3&page=3'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 3, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "expected 3 posts")
		require.Equal(t, post1.Id, posts.Order[0], "posts not in order")
		require.Equal(t, systemPostId1, posts.Order[2], "should return 3 posts and match order")
		require.Equal(t, post2.Id, posts.NextPostId, "should return post2.Id as NextPostId")
		require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")

		// similar to '/posts?per_page=3&page=4'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.Id, 4, 3, "", false)
		CheckNoError(t, resp)
		require.Empty(t, posts.Order, "should return 0 post")
		require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
		require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")
	})
}

func TestGetFlaggedPostsForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	team1 := th.BasicTeam
	channel1 := th.BasicChannel
	post1 := th.CreatePost()
	channel2 := th.CreatePublicChannel()
	post2 := th.CreatePostWithClient(Client, channel2)

	preference := model.Preference{
		UserId:   user.Id,
		Category: model.PREFERENCE_CATEGORY_FLAGGED_POST,
		Name:     post1.Id,
		Value:    "true",
	}
	_, resp := Client.UpdatePreferences(user.Id, &model.Preferences{preference})
	CheckNoError(t, resp)
	preference.Name = post2.Id
	_, resp = Client.UpdatePreferences(user.Id, &model.Preferences{preference})
	CheckNoError(t, resp)

	opl := model.NewPostList()
	opl.AddPost(post1)
	opl.AddOrder(post1.Id)

	rpl, resp := Client.GetFlaggedPostsForUserInChannel(user.Id, channel1.Id, 0, 10)
	CheckNoError(t, resp)

	require.Len(t, rpl.Posts, 1, "should have returned 1 post")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	rpl, resp = Client.GetFlaggedPostsForUserInChannel(user.Id, channel1.Id, 0, 1)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 1, "should have returned 1 post")

	rpl, resp = Client.GetFlaggedPostsForUserInChannel(user.Id, channel1.Id, 1, 1)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	rpl, resp = Client.GetFlaggedPostsForUserInChannel(user.Id, GenerateTestId(), 0, 10)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	rpl, resp = Client.GetFlaggedPostsForUserInChannel(user.Id, "junk", 0, 10)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rpl)

	opl.AddPost(post2)
	opl.AddOrder(post2.Id)

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.Id, team1.Id, 0, 10)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 2, "should have returned 2 posts")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.Id, team1.Id, 0, 1)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 1, "should have returned 1 post")

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.Id, team1.Id, 1, 1)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 1, "should have returned 1 post")

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.Id, team1.Id, 1000, 10)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.Id, GenerateTestId(), 0, 10)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.Id, "junk", 0, 10)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rpl)

	channel3 := th.CreatePrivateChannel()
	post4 := th.CreatePostWithClient(Client, channel3)

	preference.Name = post4.Id
	Client.UpdatePreferences(user.Id, &model.Preferences{preference})

	opl.AddPost(post4)
	opl.AddOrder(post4.Id)

	rpl, resp = Client.GetFlaggedPostsForUser(user.Id, 0, 10)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 3, "should have returned 3 posts")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	rpl, resp = Client.GetFlaggedPostsForUser(user.Id, 0, 2)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 2, "should have returned 2 posts")

	rpl, resp = Client.GetFlaggedPostsForUser(user.Id, 2, 2)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 1, "should have returned 1 post")

	rpl, resp = Client.GetFlaggedPostsForUser(user.Id, 1000, 10)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	channel4 := th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_PRIVATE)
	post5 := th.CreatePostWithClient(th.SystemAdminClient, channel4)

	preference.Name = post5.Id
	_, resp = Client.UpdatePreferences(user.Id, &model.Preferences{preference})
	CheckForbiddenStatus(t, resp)

	rpl, resp = Client.GetFlaggedPostsForUser(user.Id, 0, 10)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 3, "should have returned 3 posts")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	th.AddUserToChannel(user, channel4)
	_, resp = Client.UpdatePreferences(user.Id, &model.Preferences{preference})
	CheckNoError(t, resp)

	rpl, resp = Client.GetFlaggedPostsForUser(user.Id, 0, 10)
	CheckNoError(t, resp)

	opl.AddPost(post5)
	opl.AddOrder(post5.Id)
	require.Len(t, rpl.Posts, 4, "should have returned 4 posts")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	err := th.App.RemoveUserFromChannel(th.Context, user.Id, "", channel4)
	assert.Nil(t, err, "unable to remove user from channel")

	rpl, resp = Client.GetFlaggedPostsForUser(user.Id, 0, 10)
	CheckNoError(t, resp)

	opl2 := model.NewPostList()
	opl2.AddPost(post1)
	opl2.AddOrder(post1.Id)
	opl2.AddPost(post2)
	opl2.AddOrder(post2.Id)
	opl2.AddPost(post4)
	opl2.AddOrder(post4.Id)

	require.Len(t, rpl.Posts, 3, "should have returned 3 posts")
	require.Equal(t, opl2.Posts, rpl.Posts, "posts should have matched")

	_, resp = Client.GetFlaggedPostsForUser("junk", 0, 10)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFlaggedPostsForUser(GenerateTestId(), 0, 10)
	CheckForbiddenStatus(t, resp)

	Client.Logout()

	_, resp = Client.GetFlaggedPostsForUserInChannel(user.Id, channel1.Id, 0, 10)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.GetFlaggedPostsForUserInTeam(user.Id, team1.Id, 0, 10)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.GetFlaggedPostsForUser(user.Id, 0, 10)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetFlaggedPostsForUserInChannel(user.Id, channel1.Id, 0, 10)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetFlaggedPostsForUserInTeam(user.Id, team1.Id, 0, 10)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetFlaggedPostsForUser(user.Id, 0, 10)
	CheckNoError(t, resp)

	mockStore := mocks.Store{}
	mockPostStore := mocks.PostStore{}
	mockPostStore.On("GetFlaggedPosts", mock.AnythingOfType("string"), mock.AnythingOfType("int"), mock.AnythingOfType("int")).Return(nil, errors.New("some-error"))
	mockPostStore.On("ClearCaches").Return()
	mockStore.On("Team").Return(th.App.Srv().Store.Team())
	mockStore.On("Channel").Return(th.App.Srv().Store.Channel())
	mockStore.On("User").Return(th.App.Srv().Store.User())
	mockStore.On("Scheme").Return(th.App.Srv().Store.Scheme())
	mockStore.On("Post").Return(&mockPostStore)
	mockStore.On("FileInfo").Return(th.App.Srv().Store.FileInfo())
	mockStore.On("Webhook").Return(th.App.Srv().Store.Webhook())
	mockStore.On("System").Return(th.App.Srv().Store.System())
	mockStore.On("License").Return(th.App.Srv().Store.License())
	mockStore.On("Role").Return(th.App.Srv().Store.Role())
	mockStore.On("Close").Return(nil)
	th.App.Srv().Store = &mockStore

	_, resp = th.SystemAdminClient.GetFlaggedPostsForUser(user.Id, 0, 10)
	CheckInternalErrorStatus(t, resp)
}

func TestGetPostsBefore(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post1 := th.CreatePost()
	post2 := th.CreatePost()
	post3 := th.CreatePost()
	post4 := th.CreatePost()
	post5 := th.CreatePost()

	posts, resp := Client.GetPostsBefore(th.BasicChannel.Id, post3.Id, 0, 100, "", false)
	CheckNoError(t, resp)

	found := make([]bool, 2)
	for _, p := range posts.Posts {
		if p.Id == post1.Id {
			found[0] = true
		} else if p.Id == post2.Id {
			found[1] = true
		}

		require.NotEqual(t, post4.Id, p.Id, "returned posts after")
		require.NotEqual(t, post5.Id, p.Id, "returned posts after")
	}

	for _, f := range found {
		require.True(t, f, "missing post")
	}

	require.Equal(t, post3.Id, posts.NextPostId, "should match NextPostId")
	require.Equal(t, "", posts.PrevPostId, "should match empty PrevPostId")

	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post4.Id, 1, 1, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "too many posts returned")
	require.Equal(t, post2.Id, posts.Order[0], "should match returned post")
	require.Equal(t, post3.Id, posts.NextPostId, "should match NextPostId")
	require.Equal(t, post1.Id, posts.PrevPostId, "should match PrevPostId")

	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, "junk", 1, 1, "", false)
	CheckBadRequestStatus(t, resp)

	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post5.Id, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 3, "should match length of posts returned")
	require.Equal(t, post4.Id, posts.Order[0], "should match returned post")
	require.Equal(t, post2.Id, posts.Order[2], "should match returned post")
	require.Equal(t, post5.Id, posts.NextPostId, "should match NextPostId")
	require.Equal(t, post1.Id, posts.PrevPostId, "should match PrevPostId")

	// get the system post IDs posted before the created posts above
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post1.Id, 0, 2, "", false)
	CheckNoError(t, resp)
	systemPostId2 := posts.Order[0]
	systemPostId1 := posts.Order[1]

	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post5.Id, 1, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 3, "should match length of posts returned")
	require.Equal(t, post1.Id, posts.Order[0], "should match returned post")
	require.Equal(t, systemPostId2, posts.Order[1], "should match returned post")
	require.Equal(t, systemPostId1, posts.Order[2], "should match returned post")
	require.Equal(t, post2.Id, posts.NextPostId, "should match NextPostId")
	require.Equal(t, "", posts.PrevPostId, "should return empty PrevPostId")

	// more tests for next_post_id, prev_post_id, and order
	// There are 12 posts composed of first 2 system messages and 10 created posts
	post6 := th.CreatePost()
	th.CreatePost() // post7
	post8 := th.CreatePost()
	post9 := th.CreatePost()
	th.CreatePost() // post10

	// similar to '/posts?before=post9'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post9.Id, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 10, "expected 10 posts")
	require.Equal(t, post8.Id, posts.Order[0], "posts not in order")
	require.Equal(t, systemPostId1, posts.Order[9], "posts not in order")
	require.Equal(t, post9.Id, posts.NextPostId, "should return post9.Id as NextPostId")
	require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")

	// similar to '/posts?before=post9&per_page=3'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post9.Id, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post8.Id, posts.Order[0], "posts not in order")
	require.Equal(t, post6.Id, posts.Order[2], "should return 3 posts and match order")
	require.Equal(t, post9.Id, posts.NextPostId, "should return post9.Id as NextPostId")
	require.Equal(t, post5.Id, posts.PrevPostId, "should return post5.Id as PrevPostId")

	// similar to '/posts?before=post9&per_page=3&page=1'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post9.Id, 1, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post5.Id, posts.Order[0], "posts not in order")
	require.Equal(t, post3.Id, posts.Order[2], "posts not in order")
	require.Equal(t, post6.Id, posts.NextPostId, "should return post6.Id as NextPostId")
	require.Equal(t, post2.Id, posts.PrevPostId, "should return post2.Id as PrevPostId")

	// similar to '/posts?before=post9&per_page=3&page=2'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post9.Id, 2, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post2.Id, posts.Order[0], "posts not in order")
	require.Equal(t, systemPostId2, posts.Order[2], "posts not in order")
	require.Equal(t, post3.Id, posts.NextPostId, "should return post3.Id as NextPostId")
	require.Equal(t, systemPostId1, posts.PrevPostId, "should return systemPostId1 as PrevPostId")

	// similar to '/posts?before=post1&per_page=3'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post1.Id, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "expected 2 posts")
	require.Equal(t, systemPostId2, posts.Order[0], "posts not in order")
	require.Equal(t, systemPostId1, posts.Order[1], "posts not in order")
	require.Equal(t, post1.Id, posts.NextPostId, "should return post1.Id as NextPostId")
	require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")

	// similar to '/posts?before=systemPostId1'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, systemPostId1, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, systemPostId1, posts.NextPostId, "should return systemPostId1 as NextPostId")
	require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")

	// similar to '/posts?before=systemPostId1&per_page=60&page=1'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, systemPostId1, 1, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 posts")
	require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
	require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")

	// similar to '/posts?before=non-existent-post'
	nonExistentPostId := model.NewId()
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, nonExistentPostId, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, nonExistentPostId, posts.NextPostId, "should return nonExistentPostId as NextPostId")
	require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")
}

func TestGetPostsAfter(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post1 := th.CreatePost()
	post2 := th.CreatePost()
	post3 := th.CreatePost()
	post4 := th.CreatePost()
	post5 := th.CreatePost()

	posts, resp := Client.GetPostsAfter(th.BasicChannel.Id, post3.Id, 0, 100, "", false)
	CheckNoError(t, resp)

	found := make([]bool, 2)
	for _, p := range posts.Posts {
		if p.Id == post4.Id {
			found[0] = true
		} else if p.Id == post5.Id {
			found[1] = true
		}
		require.NotEqual(t, post1.Id, p.Id, "returned posts before")
		require.NotEqual(t, post2.Id, p.Id, "returned posts before")
	}

	for _, f := range found {
		require.True(t, f, "missing post")
	}
	require.Equal(t, "", posts.NextPostId, "should match empty NextPostId")
	require.Equal(t, post3.Id, posts.PrevPostId, "should match PrevPostId")

	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post2.Id, 1, 1, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "too many posts returned")
	require.Equal(t, post4.Id, posts.Order[0], "should match returned post")
	require.Equal(t, post5.Id, posts.NextPostId, "should match NextPostId")
	require.Equal(t, post3.Id, posts.PrevPostId, "should match PrevPostId")

	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, "junk", 1, 1, "", false)
	CheckBadRequestStatus(t, resp)

	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post1.Id, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 3, "should match length of posts returned")
	require.Equal(t, post4.Id, posts.Order[0], "should match returned post")
	require.Equal(t, post2.Id, posts.Order[2], "should match returned post")
	require.Equal(t, post5.Id, posts.NextPostId, "should match NextPostId")
	require.Equal(t, post1.Id, posts.PrevPostId, "should match PrevPostId")

	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post1.Id, 1, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "should match length of posts returned")
	require.Equal(t, post5.Id, posts.Order[0], "should match returned post")
	require.Equal(t, "", posts.NextPostId, "should match NextPostId")
	require.Equal(t, post4.Id, posts.PrevPostId, "should match PrevPostId")

	// more tests for next_post_id, prev_post_id, and order
	// There are 12 posts composed of first 2 system messages and 10 created posts
	post6 := th.CreatePost()
	th.CreatePost() // post7
	post8 := th.CreatePost()
	post9 := th.CreatePost()
	post10 := th.CreatePost()

	// similar to '/posts?after=post2'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post2.Id, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 8, "expected 8 posts")
	require.Equal(t, post10.Id, posts.Order[0], "should match order")
	require.Equal(t, post3.Id, posts.Order[7], "should match order")
	require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
	require.Equal(t, post2.Id, posts.PrevPostId, "should return post2.Id as PrevPostId")

	// similar to '/posts?after=post2&per_page=3'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post2.Id, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post5.Id, posts.Order[0], "should match order")
	require.Equal(t, post3.Id, posts.Order[2], "should return 3 posts and match order")
	require.Equal(t, post6.Id, posts.NextPostId, "should return post6.Id as NextPostId")
	require.Equal(t, post2.Id, posts.PrevPostId, "should return post2.Id as PrevPostId")

	// similar to '/posts?after=post2&per_page=3&page=1'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post2.Id, 1, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post8.Id, posts.Order[0], "should match order")
	require.Equal(t, post6.Id, posts.Order[2], "should match order")
	require.Equal(t, post9.Id, posts.NextPostId, "should return post9.Id as NextPostId")
	require.Equal(t, post5.Id, posts.PrevPostId, "should return post5.Id as PrevPostId")

	// similar to '/posts?after=post2&per_page=3&page=2'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post2.Id, 2, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "expected 2 posts")
	require.Equal(t, post10.Id, posts.Order[0], "should match order")
	require.Equal(t, post9.Id, posts.Order[1], "should match order")
	require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
	require.Equal(t, post8.Id, posts.PrevPostId, "should return post8.Id as PrevPostId")

	// similar to '/posts?after=post10'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post10.Id, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
	require.Equal(t, post10.Id, posts.PrevPostId, "should return post10.Id as PrevPostId")

	// similar to '/posts?after=post10&page=1'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, post10.Id, 1, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
	require.Equal(t, "", posts.PrevPostId, "should return an empty PrevPostId")

	// similar to '/posts?after=non-existent-post'
	nonExistentPostId := model.NewId()
	posts, resp = Client.GetPostsAfter(th.BasicChannel.Id, nonExistentPostId, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, "", posts.NextPostId, "should return an empty NextPostId")
	require.Equal(t, nonExistentPostId, posts.PrevPostId, "should return nonExistentPostId as PrevPostId")
}

func TestGetPostsForChannelAroundLastUnread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	userId := th.BasicUser.Id
	channelId := th.BasicChannel.Id

	// 12 posts = 2 systems posts + 10 created posts below
	post1 := th.CreatePost()
	post2 := th.CreatePost()
	post3 := th.CreatePost()
	post4 := th.CreatePost()
	post5 := th.CreatePost()
	replyPost := &model.Post{ChannelId: channelId, Message: model.NewId(), RootId: post4.Id, ParentId: post4.Id}
	post6, resp := Client.CreatePost(replyPost)
	CheckNoError(t, resp)
	post7, resp := Client.CreatePost(replyPost)
	CheckNoError(t, resp)
	post8, resp := Client.CreatePost(replyPost)
	CheckNoError(t, resp)
	post9, resp := Client.CreatePost(replyPost)
	CheckNoError(t, resp)
	post10, resp := Client.CreatePost(replyPost)
	CheckNoError(t, resp)

	postIdNames := map[string]string{
		post1.Id:  "post1",
		post2.Id:  "post2",
		post3.Id:  "post3",
		post4.Id:  "post4",
		post5.Id:  "post5",
		post6.Id:  "post6 (reply to post4)",
		post7.Id:  "post7 (reply to post4)",
		post8.Id:  "post8 (reply to post4)",
		post9.Id:  "post9 (reply to post4)",
		post10.Id: "post10 (reply to post4)",
	}

	namePost := func(postId string) string {
		name, ok := postIdNames[postId]
		if ok {
			return name
		}

		return fmt.Sprintf("unknown (%s)", postId)
	}

	namePosts := func(postIds []string) []string {
		namedPostIds := make([]string, 0, len(postIds))
		for _, postId := range postIds {
			namedPostIds = append(namedPostIds, namePost(postId))
		}

		return namedPostIds
	}

	namePostsMap := func(posts map[string]*model.Post) []string {
		namedPostIds := make([]string, 0, len(posts))
		for postId := range posts {
			namedPostIds = append(namedPostIds, namePost(postId))
		}
		sort.Strings(namedPostIds)

		return namedPostIds
	}

	assertPostList := func(t *testing.T, expected, actual *model.PostList) {
		t.Helper()

		require.Equal(t, namePosts(expected.Order), namePosts(actual.Order), "unexpected post order")
		require.Equal(t, namePostsMap(expected.Posts), namePostsMap(actual.Posts), "unexpected posts")
		require.Equal(t, namePost(expected.NextPostId), namePost(actual.NextPostId), "unexpected next post id")
		require.Equal(t, namePost(expected.PrevPostId), namePost(actual.PrevPostId), "unexpected prev post id")
	}

	// Setting limit_after to zero should fail with a 400 BadRequest.
	posts, resp := Client.GetPostsAroundLastUnread(userId, channelId, 20, 0, false)
	require.NotNil(t, resp.Error)
	require.Equal(t, "api.context.invalid_url_param.app_error", resp.Error.Id)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Nil(t, posts)

	// All returned posts are all read by the user, since it's created by the user itself.
	posts, resp = Client.GetPostsAroundLastUnread(userId, channelId, 20, 20, false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 12, "Should return 12 posts only since there's no unread post")

	// Set channel member's last viewed to 0.
	// All returned posts are latest posts as if all previous posts were already read by the user.
	channelMember, err := th.App.Srv().Store.Channel().GetMember(context.Background(), channelId, userId)
	require.NoError(t, err)
	channelMember.LastViewedAt = 0
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelId)

	posts, resp = Client.GetPostsAroundLastUnread(userId, channelId, 20, 20, false)
	CheckNoError(t, resp)

	require.Len(t, posts.Order, 12, "Should return 12 posts only since there's no unread post")

	// get the first system post generated before the created posts above
	posts, resp = Client.GetPostsBefore(th.BasicChannel.Id, post1.Id, 0, 2, "", false)
	CheckNoError(t, resp)
	systemPost0 := posts.Posts[posts.Order[0]]
	postIdNames[systemPost0.Id] = "system post 0"
	systemPost1 := posts.Posts[posts.Order[1]]
	postIdNames[systemPost1.Id] = "system post 1"

	// Set channel member's last viewed before post1.
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelId, userId)
	require.NoError(t, err)
	channelMember.LastViewedAt = post1.CreateAt - 1
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelId)

	posts, resp = Client.GetPostsAroundLastUnread(userId, channelId, 3, 3, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post3.Id, post2.Id, post1.Id, systemPost0.Id, systemPost1.Id},
		Posts: map[string]*model.Post{
			systemPost0.Id: systemPost0,
			systemPost1.Id: systemPost1,
			post1.Id:       post1,
			post2.Id:       post2,
			post3.Id:       post3,
		},
		NextPostId: post4.Id,
		PrevPostId: "",
	}, posts)

	// Set channel member's last viewed before post6.
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelId, userId)
	require.NoError(t, err)
	channelMember.LastViewedAt = post6.CreateAt - 1
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelId)

	posts, resp = Client.GetPostsAroundLastUnread(userId, channelId, 3, 3, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post8.Id, post7.Id, post6.Id, post5.Id, post4.Id, post3.Id},
		Posts: map[string]*model.Post{
			post3.Id:  post3,
			post4.Id:  post4,
			post5.Id:  post5,
			post6.Id:  post6,
			post7.Id:  post7,
			post8.Id:  post8,
			post9.Id:  post9,
			post10.Id: post10,
		},
		NextPostId: post9.Id,
		PrevPostId: post2.Id,
	}, posts)

	// Set channel member's last viewed before post10.
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelId, userId)
	require.NoError(t, err)
	channelMember.LastViewedAt = post10.CreateAt - 1
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelId)

	posts, resp = Client.GetPostsAroundLastUnread(userId, channelId, 3, 3, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post10.Id, post9.Id, post8.Id, post7.Id},
		Posts: map[string]*model.Post{
			post4.Id:  post4,
			post6.Id:  post6,
			post7.Id:  post7,
			post8.Id:  post8,
			post9.Id:  post9,
			post10.Id: post10,
		},
		NextPostId: "",
		PrevPostId: post6.Id,
	}, posts)

	// Set channel member's last viewed equal to post10.
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelId, userId)
	require.NoError(t, err)
	channelMember.LastViewedAt = post10.CreateAt
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelId)

	posts, resp = Client.GetPostsAroundLastUnread(userId, channelId, 3, 3, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post10.Id, post9.Id, post8.Id},
		Posts: map[string]*model.Post{
			post4.Id:  post4,
			post6.Id:  post6,
			post7.Id:  post7,
			post8.Id:  post8,
			post9.Id:  post9,
			post10.Id: post10,
		},
		NextPostId: "",
		PrevPostId: post7.Id,
	}, posts)

	// Set channel member's last viewed to just before a new reply to a previous thread, not
	// otherwise in the requested window.
	post11 := th.CreatePost()
	post12, resp := Client.CreatePost(&model.Post{
		ChannelId: channelId,
		Message:   model.NewId(),
		RootId:    post4.Id,
		ParentId:  post4.Id,
	})
	CheckNoError(t, resp)
	post13 := th.CreatePost()

	postIdNames[post11.Id] = "post11"
	postIdNames[post12.Id] = "post12 (reply to post4)"
	postIdNames[post13.Id] = "post13"

	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelId, userId)
	require.NoError(t, err)
	channelMember.LastViewedAt = post12.CreateAt - 1
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelId)

	posts, resp = Client.GetPostsAroundLastUnread(userId, channelId, 1, 2, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post13.Id, post12.Id, post11.Id},
		Posts: map[string]*model.Post{
			post4.Id:  post4,
			post6.Id:  post6,
			post7.Id:  post7,
			post8.Id:  post8,
			post9.Id:  post9,
			post10.Id: post10,
			post11.Id: post11,
			post12.Id: post12,
			post13.Id: post13,
		},
		NextPostId: "",
		PrevPostId: post10.Id,
	}, posts)
}

func TestGetPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	// TODO: migrate this entirely to the subtest's client
	// once the other methods are migrated too.
	Client := th.Client

	var privatePost *model.Post
	th.TestForAllClients(t, func(t *testing.T, c *model.Client4) {
		t.Helper()

		post, resp := c.GetPost(th.BasicPost.Id, "")
		CheckNoError(t, resp)

		require.Equal(t, th.BasicPost.Id, post.Id, "post ids don't match")

		post, resp = c.GetPost(th.BasicPost.Id, resp.Etag)
		CheckEtag(t, post, resp)

		_, resp = c.GetPost("", "")
		CheckNotFoundStatus(t, resp)

		_, resp = c.GetPost("junk", "")
		CheckBadRequestStatus(t, resp)

		_, resp = c.GetPost(model.NewId(), "")
		CheckNotFoundStatus(t, resp)

		Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)

		// Channel is public, should be able to read post
		_, resp = c.GetPost(th.BasicPost.Id, "")
		CheckNoError(t, resp)

		privatePost = th.CreatePostWithClient(Client, th.BasicPrivateChannel)

		_, resp = c.GetPost(privatePost.Id, "")
		CheckNoError(t, resp)
	})

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.Id, th.BasicUser.Id)

	// Channel is private, should not be able to read post
	_, resp := Client.GetPost(privatePost.Id, "")
	CheckForbiddenStatus(t, resp)

	// But local client should.
	_, resp = th.LocalClient.GetPost(privatePost.Id, "")
	CheckNoError(t, resp)

	Client.Logout()

	// Normal client should get unauthorized, but local client should get 404.
	_, resp = Client.GetPost(model.NewId(), "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.LocalClient.GetPost(model.NewId(), "")
	CheckNotFoundStatus(t, resp)
}

func TestGetPosts(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	// TODO: migrate this entirely to the subtest's client
	// once the other methods are migrated too.
	Client := th.Client

	var privatePost *model.Post
	th.TestForAllClients(t, func(t *testing.T, c *model.Client4) {
		t.Helper()

		posts, resp := c.GetPosts([]string{th.BasicPost.Id})
		CheckNoError(t, resp)

		require.Equal(t, th.BasicPost.Id, posts[0].Id, "post ids don't match")
		require.Len(t, posts, 1)

		posts, resp = c.GetPosts([]string{""})
		CheckNoError(t, resp)
		require.Len(t, posts, 0)

		posts, resp = c.GetPosts([]string{"junk"})
		CheckNoError(t, resp)
		require.Len(t, posts, 0)

		posts, resp = c.GetPosts([]string{model.NewId()})
		CheckNoError(t, resp)
		require.Len(t, posts, 0)

		Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)

		// Channel is public, should be able to read post
		posts, resp = c.GetPosts([]string{th.BasicPost.Id})
		CheckNoError(t, resp)
		require.Equal(t, th.BasicPost.Id, posts[0].Id, "post ids don't match")
		require.Len(t, posts, 1)

		privatePost = th.CreatePostWithClient(Client, th.BasicPrivateChannel)
	})

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.Id, th.BasicUser.Id)

	// Channel is private, should not be able to read post
	posts, resp := Client.GetPosts([]string{privatePost.Id})
	CheckNoError(t, resp)
	require.Len(t, posts, 0)

	// But local client should.
	posts, resp = th.LocalClient.GetPosts([]string{privatePost.Id})
	CheckNoError(t, resp)
	require.Len(t, posts, 1)

	Client.Logout()

	// Normal client should get unauthorized, but local client should get 404.
	_, resp = Client.GetPosts([]string{model.NewId()})
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.LocalClient.GetPosts([]string{model.NewId()})
	CheckNoError(t, resp)
}

func TestDeletePost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	_, resp := Client.DeletePost("")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.DeletePost("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.DeletePost(th.BasicPost.Id)
	CheckForbiddenStatus(t, resp)

	Client.Login(th.TeamAdminUser.Email, th.TeamAdminUser.Password)
	_, resp = Client.DeletePost(th.BasicPost.Id)
	CheckNoError(t, resp)

	post := th.CreatePost()
	user := th.CreateUser()

	Client.Logout()
	Client.Login(user.Email, user.Password)

	_, resp = Client.DeletePost(post.Id)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.DeletePost(model.NewId())
	CheckUnauthorizedStatus(t, resp)

	status, resp := th.SystemAdminClient.DeletePost(post.Id)
	require.True(t, status, "post should return status OK")
	CheckNoError(t, resp)
}

func TestDeletePostMessage(t *testing.T) {
	th := Setup(t).InitBasic()
	th.LinkUserToTeam(th.SystemAdminUser, th.BasicTeam)
	th.App.AddUserToChannel(th.SystemAdminUser, th.BasicChannel, false)

	defer th.TearDown()

	testCases := []struct {
		description string
		client      *model.Client4
		delete_by   interface{}
	}{
		{"Do not send delete_by to regular user", th.Client, nil},
		{"Send delete_by to system admin user", th.SystemAdminClient, th.SystemAdminUser.Id},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			wsClient, err := th.CreateWebSocketClientWithClient(tc.client)
			require.Nil(t, err)
			defer wsClient.Close()

			wsClient.Listen()

			post := th.CreatePost()

			status, resp := th.SystemAdminClient.DeletePost(post.Id)
			require.True(t, status, "post should return status OK")
			CheckNoError(t, resp)

			timeout := time.After(5 * time.Second)

			for {
				select {
				case ev := <-wsClient.EventChannel:
					if ev.EventType() == model.WEBSOCKET_EVENT_POST_DELETED {
						assert.Equal(t, tc.delete_by, ev.GetData()["delete_by"])
						return
					}
				case <-timeout:
					// We just skip the test instead of failing because waiting for more than 5 seconds
					// to get a response does not make sense, and it will unnecessarily slow down
					// the tests further in an already congested CI environment.
					t.Skip("timed out waiting for event")
				}
			}
		})
	}
}

func TestGetPostThread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post := &model.Post{ChannelId: th.BasicChannel.Id, Message: "zz" + model.NewId() + "a", RootId: th.BasicPost.Id}
	post, _ = Client.CreatePost(post)

	list, resp := Client.GetPostThread(th.BasicPost.Id, "", false)
	CheckNoError(t, resp)

	var list2 *model.PostList
	list2, resp = Client.GetPostThread(th.BasicPost.Id, resp.Etag, false)
	CheckEtag(t, list2, resp)
	require.Equal(t, th.BasicPost.Id, list.Order[0], "wrong order")

	_, ok := list.Posts[th.BasicPost.Id]
	require.True(t, ok, "should have had post")

	_, ok = list.Posts[post.Id]
	require.True(t, ok, "should have had post")

	_, resp = Client.GetPostThread("junk", "", false)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPostThread(model.NewId(), "", false)
	CheckNotFoundStatus(t, resp)

	Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)

	// Channel is public, should be able to read post
	_, resp = Client.GetPostThread(th.BasicPost.Id, "", false)
	CheckNoError(t, resp)

	privatePost := th.CreatePostWithClient(Client, th.BasicPrivateChannel)

	_, resp = Client.GetPostThread(privatePost.Id, "", false)
	CheckNoError(t, resp)

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.Id, th.BasicUser.Id)

	// Channel is private, should not be able to read post
	_, resp = Client.GetPostThread(privatePost.Id, "", false)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPostThread(model.NewId(), "", false)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPostThread(th.BasicPost.Id, "", false)
	CheckNoError(t, resp)
}

func TestSearchPosts(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	experimentalViewArchivedChannels := *th.App.Config().TeamSettings.ExperimentalViewArchivedChannels
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.TeamSettings.ExperimentalViewArchivedChannels = &experimentalViewArchivedChannels
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.ExperimentalViewArchivedChannels = true
	})

	th.LoginBasic()
	Client := th.Client

	message := "search for post1"
	_ = th.CreateMessagePost(message)

	message = "search for post2"
	post2 := th.CreateMessagePost(message)

	message = "#hashtag search for post3"
	post3 := th.CreateMessagePost(message)

	message = "hashtag for post4"
	_ = th.CreateMessagePost(message)

	archivedChannel := th.CreatePublicChannel()
	_ = th.CreateMessagePostWithClient(th.Client, archivedChannel, "#hashtag for post3")
	th.Client.DeleteChannel(archivedChannel.Id)

	terms := "search"
	isOrSearch := false
	timezoneOffset := 5
	searchParams := model.SearchParameter{
		Terms:          &terms,
		IsOrSearch:     &isOrSearch,
		TimeZoneOffset: &timezoneOffset,
	}
	posts, resp := Client.SearchPostsWithParams(th.BasicTeam.Id, &searchParams)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "wrong search")

	terms = "search"
	page := 0
	perPage := 2
	searchParams = model.SearchParameter{
		Terms:          &terms,
		IsOrSearch:     &isOrSearch,
		TimeZoneOffset: &timezoneOffset,
		Page:           &page,
		PerPage:        &perPage,
	}
	posts2, resp := Client.SearchPostsWithParams(th.BasicTeam.Id, &searchParams)
	CheckNoError(t, resp)
	// We don't support paging for DB search yet, modify this when we do.
	require.Len(t, posts2.Order, 3, "Wrong number of posts")
	assert.Equal(t, posts.Order[0], posts2.Order[0])
	assert.Equal(t, posts.Order[1], posts2.Order[1])

	page = 1
	searchParams = model.SearchParameter{
		Terms:          &terms,
		IsOrSearch:     &isOrSearch,
		TimeZoneOffset: &timezoneOffset,
		Page:           &page,
		PerPage:        &perPage,
	}
	posts2, resp = Client.SearchPostsWithParams(th.BasicTeam.Id, &searchParams)
	CheckNoError(t, resp)
	// We don't support paging for DB search yet, modify this when we do.
	require.Empty(t, posts2.Order, "Wrong number of posts")

	posts, resp = Client.SearchPosts(th.BasicTeam.Id, "search", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "wrong search")

	posts, resp = Client.SearchPosts(th.BasicTeam.Id, "post2", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 1, "wrong number of posts")
	require.Equal(t, post2.Id, posts.Order[0], "wrong search")

	posts, resp = Client.SearchPosts(th.BasicTeam.Id, "#hashtag", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 1, "wrong number of posts")
	require.Equal(t, post3.Id, posts.Order[0], "wrong search")

	terms = "#hashtag"
	includeDeletedChannels := true
	searchParams = model.SearchParameter{
		Terms:                  &terms,
		IsOrSearch:             &isOrSearch,
		TimeZoneOffset:         &timezoneOffset,
		IncludeDeletedChannels: &includeDeletedChannels,
	}
	posts, resp = Client.SearchPostsWithParams(th.BasicTeam.Id, &searchParams)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "wrong search")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.ExperimentalViewArchivedChannels = false
	})

	posts, resp = Client.SearchPostsWithParams(th.BasicTeam.Id, &searchParams)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 1, "wrong search")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "*", false)
	require.Empty(t, posts.Order, "searching for just * shouldn't return any results")

	posts, resp = Client.SearchPosts(th.BasicTeam.Id, "post1 post2", true)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "wrong search results")

	_, resp = Client.SearchPosts("junk", "#sgtitlereview", false)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.SearchPosts(model.NewId(), "#sgtitlereview", false)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.SearchPosts(th.BasicTeam.Id, "", false)
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.SearchPosts(th.BasicTeam.Id, "#sgtitlereview", false)
	CheckUnauthorizedStatus(t, resp)
}

func TestSearchHashtagPosts(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.LoginBasic()
	Client := th.Client

	message := "#sgtitlereview with space"
	assert.NotNil(t, th.CreateMessagePost(message))

	message = "#sgtitlereview\n with return"
	assert.NotNil(t, th.CreateMessagePost(message))

	message = "no hashtag"
	assert.NotNil(t, th.CreateMessagePost(message))

	posts, resp := Client.SearchPosts(th.BasicTeam.Id, "#sgtitlereview", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "wrong search results")

	Client.Logout()
	_, resp = Client.SearchPosts(th.BasicTeam.Id, "#sgtitlereview", false)
	CheckUnauthorizedStatus(t, resp)
}

func TestSearchPostsInChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.LoginBasic()
	Client := th.Client

	channel := th.CreatePublicChannel()

	message := "sgtitlereview with space"
	_ = th.CreateMessagePost(message)

	message = "sgtitlereview\n with return"
	_ = th.CreateMessagePostWithClient(Client, th.BasicChannel2, message)

	message = "other message with no return"
	_ = th.CreateMessagePostWithClient(Client, th.BasicChannel2, message)

	message = "other message with no return"
	_ = th.CreateMessagePostWithClient(Client, channel, message)

	posts, _ := Client.SearchPosts(th.BasicTeam.Id, "channel:", false)
	require.Empty(t, posts.Order, "wrong number of posts for search 'channel:'")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "in:", false)
	require.Empty(t, posts.Order, "wrong number of posts for search 'in:'")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "channel:"+th.BasicChannel.Name, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts returned for search 'channel:%v'", th.BasicChannel.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "in:"+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts returned for search 'in:%v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "channel:"+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'channel:%v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "ChAnNeL:"+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'ChAnNeL:%v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "sgtitlereview", false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'sgtitlereview'")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "sgtitlereview channel:"+th.BasicChannel.Name, false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'sgtitlereview channel:%v'", th.BasicChannel.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "sgtitlereview in: "+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'sgtitlereview in: %v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "sgtitlereview channel: "+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'sgtitlereview channel: %v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "channel: "+th.BasicChannel2.Name+" channel: "+channel.Name, false)
	require.Lenf(t, posts.Order, 3, "wrong number of posts for 'channel: %v channel: %v'", th.BasicChannel2.Name, channel.Name)
}

func TestSearchPostsFromUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.LoginTeamAdmin()
	user := th.CreateUser()
	th.LinkUserToTeam(user, th.BasicTeam)
	th.App.AddUserToChannel(user, th.BasicChannel, false)
	th.App.AddUserToChannel(user, th.BasicChannel2, false)

	message := "sgtitlereview with space"
	_ = th.CreateMessagePost(message)

	Client.Logout()
	th.LoginBasic2()

	message = "sgtitlereview\n with return"
	_ = th.CreateMessagePostWithClient(Client, th.BasicChannel2, message)

	posts, _ := Client.SearchPosts(th.BasicTeam.Id, "from: "+th.TeamAdminUser.Username, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'from: %v'", th.TeamAdminUser.Username)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "from: "+th.BasicUser2.Username, false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'from: %v", th.BasicUser2.Username)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "from: "+th.BasicUser2.Username+" sgtitlereview", false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'from: %v'", th.BasicUser2.Username)

	message = "hullo"
	_ = th.CreateMessagePost(message)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "from: "+th.BasicUser2.Username+" in:"+th.BasicChannel.Name, false)
	require.Len(t, posts.Order, 1, "wrong number of posts for search 'from: %v in:", th.BasicUser2.Username, th.BasicChannel.Name)

	Client.Login(user.Email, user.Password)

	// wait for the join/leave messages to be created for user3 since they're done asynchronously
	time.Sleep(100 * time.Millisecond)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "from: "+th.BasicUser2.Username, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'from: %v'", th.BasicUser2.Username)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "from: "+th.BasicUser2.Username+" from: "+user.Username, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'from: %v from: %v'", th.BasicUser2.Username, user.Username)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "from: "+th.BasicUser2.Username+" from: "+user.Username+" in:"+th.BasicChannel2.Name, false)
	require.Len(t, posts.Order, 1, "wrong number of posts")

	message = "coconut"
	_ = th.CreateMessagePostWithClient(Client, th.BasicChannel2, message)

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "from: "+th.BasicUser2.Username+" from: "+user.Username+" in:"+th.BasicChannel2.Name+" coconut", false)
	require.Len(t, posts.Order, 1, "wrong number of posts")
}

func TestSearchPostsWithDateFlags(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.LoginBasic()
	Client := th.Client

	message := "sgtitlereview\n with return"
	createDate := time.Date(2018, 8, 1, 5, 0, 0, 0, time.UTC)
	_ = th.CreateMessagePostNoClient(th.BasicChannel, message, utils.MillisFromTime(createDate))

	message = "other message with no return"
	createDate = time.Date(2018, 8, 2, 5, 0, 0, 0, time.UTC)
	_ = th.CreateMessagePostNoClient(th.BasicChannel, message, utils.MillisFromTime(createDate))

	message = "other message with no return"
	createDate = time.Date(2018, 8, 3, 5, 0, 0, 0, time.UTC)
	_ = th.CreateMessagePostNoClient(th.BasicChannel, message, utils.MillisFromTime(createDate))

	posts, _ := Client.SearchPosts(th.BasicTeam.Id, "return", false)
	require.Len(t, posts.Order, 3, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "on:", false)
	require.Empty(t, posts.Order, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "after:", false)
	require.Empty(t, posts.Order, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "before:", false)
	require.Empty(t, posts.Order, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "on:2018-08-01", false)
	require.Len(t, posts.Order, 1, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "after:2018-08-01", false)
	resultCount := 0
	for _, post := range posts.Posts {
		if post.UserId == th.BasicUser.Id {
			resultCount = resultCount + 1
		}
	}
	require.Equal(t, 2, resultCount, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "before:2018-08-02", false)
	require.Len(t, posts.Order, 1, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "before:2018-08-03 after:2018-08-02", false)
	require.Empty(t, posts.Order, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.Id, "before:2018-08-03 after:2018-08-01", false)
	require.Len(t, posts.Order, 1, "wrong number of posts")
}

func TestGetFileInfosForPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	fileIds := make([]string, 3)
	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		fileResp, _ := Client.UploadFile(data, th.BasicChannel.Id, "test.png")
		fileIds[i] = fileResp.FileInfos[0].Id
	}

	post := &model.Post{ChannelId: th.BasicChannel.Id, Message: "zz" + model.NewId() + "a", FileIds: fileIds}
	post, _ = Client.CreatePost(post)

	infos, resp := Client.GetFileInfosForPost(post.Id, "")
	CheckNoError(t, resp)

	require.Len(t, infos, 3, "missing file infos")

	found := false
	for _, info := range infos {
		if info.Id == fileIds[0] {
			found = true
		}
	}

	require.True(t, found, "missing file info")

	infos, resp = Client.GetFileInfosForPost(post.Id, resp.Etag)
	CheckEtag(t, infos, resp)

	infos, resp = Client.GetFileInfosForPost(th.BasicPost.Id, "")
	CheckNoError(t, resp)

	require.Empty(t, infos, "should have no file infos")

	_, resp = Client.GetFileInfosForPost("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFileInfosForPost(model.NewId(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetFileInfosForPost(model.NewId(), "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetFileInfosForPost(th.BasicPost.Id, "")
	CheckNoError(t, resp)
}

func TestSetChannelUnread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	u1 := th.BasicUser
	u2 := th.BasicUser2
	s2, _ := th.App.GetSession(th.Client.AuthToken)
	th.Client.Login(u1.Email, u1.Password)
	c1 := th.BasicChannel
	c1toc2 := &model.ChannelView{ChannelId: th.BasicChannel2.Id, PrevChannelId: c1.Id}
	now := utils.MillisFromTime(time.Now())
	th.CreateMessagePostNoClient(c1, "AAA", now)
	p2 := th.CreateMessagePostNoClient(c1, "BBB", now+10)
	th.CreateMessagePostNoClient(c1, "CCC", now+20)

	pp1 := th.CreateMessagePostNoClient(th.BasicPrivateChannel, "Sssh!", now)
	pp2 := th.CreateMessagePostNoClient(th.BasicPrivateChannel, "You Sssh!", now+10)
	require.NotNil(t, pp1)
	require.NotNil(t, pp2)

	// Ensure that post have been read
	unread, err := th.App.GetChannelUnread(c1.Id, u1.Id)
	require.Nil(t, err)
	require.Equal(t, int64(4), unread.MsgCount)
	unread, err = th.App.GetChannelUnread(c1.Id, u2.Id)
	require.Nil(t, err)
	require.Equal(t, int64(4), unread.MsgCount)
	_, err = th.App.ViewChannel(c1toc2, u2.Id, s2.Id, false)
	require.Nil(t, err)
	unread, err = th.App.GetChannelUnread(c1.Id, u2.Id)
	require.Nil(t, err)
	require.Equal(t, int64(0), unread.MsgCount)

	t.Run("Unread last one", func(t *testing.T) {
		r := th.Client.SetPostUnread(u1.Id, p2.Id, true)
		checkHTTPStatus(t, r, 200, false)
		unread, err := th.App.GetChannelUnread(c1.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(2), unread.MsgCount)
	})

	t.Run("Unread on a private channel", func(t *testing.T) {
		r := th.Client.SetPostUnread(u1.Id, pp2.Id, true)
		assert.Equal(t, 200, r.StatusCode)
		unread, err := th.App.GetChannelUnread(th.BasicPrivateChannel.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(1), unread.MsgCount)
		r = th.Client.SetPostUnread(u1.Id, pp1.Id, true)
		assert.Equal(t, 200, r.StatusCode)
		unread, err = th.App.GetChannelUnread(th.BasicPrivateChannel.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(2), unread.MsgCount)
	})

	t.Run("Can't unread an imaginary post", func(t *testing.T) {
		r := th.Client.SetPostUnread(u1.Id, "invalid4ofngungryquinj976y", true)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
	})

	// let's create another user to test permissions
	u3 := th.CreateUser()
	c3 := th.CreateClient()
	c3.Login(u3.Email, u3.Password)

	t.Run("Can't unread channels you don't belong to", func(t *testing.T) {
		r := c3.SetPostUnread(u3.Id, pp1.Id, true)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
	})

	t.Run("Can't unread users you don't have permission to edit", func(t *testing.T) {
		r := c3.SetPostUnread(u1.Id, pp1.Id, true)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
	})

	t.Run("Can't unread if user is not logged in", func(t *testing.T) {
		th.Client.Logout()
		response := th.Client.SetPostUnread(u1.Id, p2.Id, true)
		checkHTTPStatus(t, response, http.StatusUnauthorized, true)
	})
}

func TestSetPostUnreadWithoutCollapsedThreads(t *testing.T) {
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	// user2: first root mention @user1
	//   - user1: hello
	//   - user2: mention @u1
	//   - user1: another repoy
	//   - user2: another mention @u1
	// user1: a root post
	// user2: Another root mention @u1
	user1Mention := " @" + th.BasicUser.Username
	rootPost1, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "first root mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost1.Id, UserId: th.BasicUser.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hello"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	replyPost1, appErr := th.App.CreatePost(th.Context, &model.Post{RootId: rootPost1.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost1.Id, UserId: th.BasicUser.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "another reply"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost1.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "another mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "a root post"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "another root mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)

	t.Run("Mark reply post as unread", func(t *testing.T) {
		resp := th.Client.SetPostUnread(th.BasicUser.Id, replyPost1.Id, false)
		CheckNoError(t, resp)
		channelUnread, appErr := th.App.GetChannelUnread(th.BasicChannel.Id, th.BasicUser.Id)
		require.Nil(t, appErr)

		require.Equal(t, int64(3), channelUnread.MentionCount)
		//  MentionCountRoot should be zero so that supported clients don't show a mention badge for the channel
		require.Equal(t, int64(0), channelUnread.MentionCountRoot)

		require.Equal(t, int64(5), channelUnread.MsgCount)
		//  MentionCountRoot should be zero so that supported clients don't show the channel as unread
		require.Equal(t, channelUnread.MsgCountRoot, int64(0))

		threadMembership, err := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost1.Id)
		require.Nil(t, err)
		thread, err := th.App.GetThreadForUser(th.BasicTeam.Id, threadMembership, false)
		require.Nil(t, err)
		require.Equal(t, int64(2), thread.UnreadMentions)
		require.Equal(t, int64(3), thread.UnreadReplies)
	})

	t.Run("Mark root post as unread", func(t *testing.T) {
		resp := th.Client.SetPostUnread(th.BasicUser.Id, rootPost1.Id, false)
		CheckNoError(t, resp)
		channelUnread, appErr := th.App.GetChannelUnread(th.BasicChannel.Id, th.BasicUser.Id)
		require.Nil(t, appErr)

		require.Equal(t, int64(4), channelUnread.MentionCount)
		require.Equal(t, int64(2), channelUnread.MentionCountRoot)

		require.Equal(t, int64(7), channelUnread.MsgCount)
		require.Equal(t, int64(3), channelUnread.MsgCountRoot)
	})
}
