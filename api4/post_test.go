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

	post := &model.Post{ChannelID: th.BasicChannel.ID, Message: "#hashtag a" + model.NewID() + "a", Props: model.StringInterface{model.PropsAddChannelMember: "no good"}}
	rpost, resp := Client.CreatePost(post)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	require.Equal(t, post.Message, rpost.Message, "message didn't match")
	require.Equal(t, "#hashtag", rpost.Hashtags, "hashtag didn't match")
	require.Empty(t, rpost.FileIDs)
	require.Equal(t, 0, int(rpost.EditAt), "newly created post shouldn't have EditAt set")
	require.Nil(t, rpost.GetProp(model.PropsAddChannelMember), "newly created post shouldn't have Props['add_channel_member'] set")

	post.RootID = rpost.ID
	post.ParentID = rpost.ID
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.RootID = "junk"
	_, resp = Client.CreatePost(post)
	CheckBadRequestStatus(t, resp)

	post.RootID = rpost.ID
	post.ParentID = "junk"
	_, resp = Client.CreatePost(post)
	CheckBadRequestStatus(t, resp)

	post2 := &model.Post{ChannelID: th.BasicChannel2.ID, Message: "zz" + model.NewID() + "a", CreateAt: 123}
	rpost2, _ := Client.CreatePost(post2)
	require.NotEqual(t, post2.CreateAt, rpost2.CreateAt, "create at should not match")

	t.Run("with file uploaded by same user", func(t *testing.T) {
		fileResp, subResponse := Client.UploadFile([]byte("data"), th.BasicChannel.ID, "test")
		CheckNoError(t, subResponse)
		fileID := fileResp.FileInfos[0].ID

		postWithFiles, subResponse := Client.CreatePost(&model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "with files",
			FileIDs:   model.StringArray{fileID},
		})
		CheckNoError(t, subResponse)
		assert.Equal(t, model.StringArray{fileID}, postWithFiles.FileIDs)

		actualPostWithFiles, subResponse := Client.GetPost(postWithFiles.ID, "")
		CheckNoError(t, subResponse)
		assert.Equal(t, model.StringArray{fileID}, actualPostWithFiles.FileIDs)
	})

	t.Run("with file uploaded by different user", func(t *testing.T) {
		fileResp, subResponse := th.SystemAdminClient.UploadFile([]byte("data"), th.BasicChannel.ID, "test")
		CheckNoError(t, subResponse)
		fileID := fileResp.FileInfos[0].ID

		postWithFiles, subResponse := Client.CreatePost(&model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "with files",
			FileIDs:   model.StringArray{fileID},
		})
		CheckNoError(t, subResponse)
		assert.Empty(t, postWithFiles.FileIDs)

		actualPostWithFiles, subResponse := Client.GetPost(postWithFiles.ID, "")
		CheckNoError(t, subResponse)
		assert.Empty(t, actualPostWithFiles.FileIDs)
	})

	t.Run("with file uploaded by nouser", func(t *testing.T) {
		fileInfo, err := th.App.UploadFile(th.Context, []byte("data"), th.BasicChannel.ID, "test")
		require.Nil(t, err)
		fileID := fileInfo.ID

		postWithFiles, subResponse := Client.CreatePost(&model.Post{
			ChannelID: th.BasicChannel.ID,
			Message:   "with files",
			FileIDs:   model.StringArray{fileID},
		})
		CheckNoError(t, subResponse)
		assert.Equal(t, model.StringArray{fileID}, postWithFiles.FileIDs)

		actualPostWithFiles, subResponse := Client.GetPost(postWithFiles.ID, "")
		CheckNoError(t, subResponse)
		assert.Equal(t, model.StringArray{fileID}, actualPostWithFiles.FileIDs)
	})

	t.Run("Create posts without the USE_CHANNEL_MENTIONS Permission - returns ephemeral message with mentions and no ephemeral message without mentions", func(t *testing.T) {
		WebSocketClient, err := th.CreateWebSocketClient()
		WebSocketClient.Listen()
		require.Nil(t, err)

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.RemovePermissionFromRole(model.PermissionUseChannelMentions.ID, model.ChannelUserRoleID)

		post.RootID = rpost.ID
		post.ParentID = rpost.ID
		post.Message = "a post with no channel mentions"
		_, resp = Client.CreatePost(post)
		CheckNoError(t, resp)

		// Message with no channel mentions should result in no ephemeral message
		timeout := time.After(300 * time.Millisecond)
		waiting := true
		for waiting {
			select {
			case event := <-WebSocketClient.EventChannel:
				require.NotEqual(t, model.WebsocketEventEphemeralMessage, event.EventType(), "should not have ephemeral message event")
			case <-timeout:
				waiting = false
			}
		}

		post.RootID = rpost.ID
		post.ParentID = rpost.ID
		post.Message = "a post with @channel"
		_, resp = Client.CreatePost(post)
		CheckNoError(t, resp)

		post.RootID = rpost.ID
		post.ParentID = rpost.ID
		post.Message = "a post with @all"
		_, resp = Client.CreatePost(post)
		CheckNoError(t, resp)

		post.RootID = rpost.ID
		post.ParentID = rpost.ID
		post.Message = "a post with @here"
		_, resp = Client.CreatePost(post)
		CheckNoError(t, resp)

		timeout = time.After(600 * time.Millisecond)
		eventsToGo := 3 // 3 Posts created with @ mentions should result in 3 websocket events
		for eventsToGo > 0 {
			select {
			case event := <-WebSocketClient.EventChannel:
				if event.Event == model.WebsocketEventEphemeralMessage {
					require.Equal(t, model.WebsocketEventEphemeralMessage, event.Event)
					eventsToGo = eventsToGo - 1
				}
			case <-timeout:
				require.Fail(t, "Should have received ephemeral message event and not timedout")
				eventsToGo = 0
			}
		}
	})

	post.RootID = ""
	post.ParentID = ""
	post.Type = model.PostTypeSystemGeneric
	_, resp = Client.CreatePost(post)
	CheckBadRequestStatus(t, resp)

	post.Type = ""
	post.RootID = rpost2.ID
	post.ParentID = rpost2.ID
	_, resp = Client.CreatePost(post)
	CheckBadRequestStatus(t, resp)

	post.RootID = ""
	post.ParentID = ""
	post.ChannelID = "junk"
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	post.ChannelID = model.NewID()
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	r, err := Client.DoApiPost("/posts", "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	Client.Logout()
	_, resp = Client.CreatePost(post)
	CheckUnauthorizedStatus(t, resp)

	post.ChannelID = th.BasicChannel.ID
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
		UserID: th.BasicUser2.ID,
		Post:   &model.Post{ChannelID: th.BasicChannel.ID, Message: "a" + model.NewID() + "a", Props: model.StringInterface{model.PropsAddChannelMember: "no good"}},
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
	fileIDs []string,
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
			TeamID:      hook.TeamID,
			TeamDomain:  team.Name,
			ChannelID:   post.ChannelID,
			ChannelName: channel.Name,
			Timestamp:   post.CreateAt,
			UserID:      post.UserID,
			UserName:    user.Username,
			PostID:      post.ID,
			Text:        post.Message,
			TriggerWord: triggerWord,
			FileIDs:     strings.Join(post.FileIDs, ","),
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
			respPostType = model.OutgoingHookResponseTypeComment
		}

		outGoingHookResponse := &model.OutgoingWebhookResponse{
			Text:         model.NewString("some test text"),
			Username:     "TestCommandServer",
			IconURL:      "https://www.mattermost.org/wp-content/uploads/2016/04/icon.png",
			Type:         "custom_as",
			ResponseType: respPostType,
		}

		fmt.Fprintf(w, outGoingHookResponse.ToJSON())
		success <- true
	}))
	defer ts.Close()

	// create an outgoing webhook, passing it the test server URL
	var triggerWords []string
	if triggerWord != "" {
		triggerWords = []string{triggerWord}
	}

	hook = &model.OutgoingWebhook{
		ChannelID:    channel.ID,
		TeamID:       team.ID,
		ContentType:  hookContentType,
		TriggerWords: triggerWords,
		TriggerWhen:  triggerWhen,
		CallbackURLs: []string{ts.URL},
	}

	hook, resp := th.SystemAdminClient.CreateOutgoingWebhook(hook)
	CheckNoError(t, resp)

	// create a post to trigger the webhook
	post = &model.Post{
		ChannelID: channel.ID,
		Message:   message,
		FileIDs:   fileIDs,
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
		postList, resp := th.SystemAdminClient.GetPostThread(post.ID, "", false)
		CheckNoError(t, resp)
		require.Equal(t, post.ID, postList.Order[0], "wrong order")

		_, ok := postList.Posts[post.ID]
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

	post := &model.Post{ChannelID: th.BasicChannel.ID, Message: "#hashtag a" + model.NewID() + "a"}

	user := model.User{Email: th.GenerateTestEmail(), Nickname: "Joram Wilander", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SystemUserRoleID}

	ruser, resp := Client.CreateUser(&user)
	CheckNoError(t, resp)

	Client.Login(user.Email, user.Password)

	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	th.App.UpdateUserRoles(ruser.ID, model.SystemUserRoleID+" "+model.SystemPostAllPublicRoleID, false)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Email, user.Password)

	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelID = th.BasicPrivateChannel.ID
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	th.App.UpdateUserRoles(ruser.ID, model.SystemUserRoleID, false)
	th.App.JoinUserToTeam(th.Context, th.BasicTeam, ruser, "")
	th.App.UpdateTeamMemberRoles(th.BasicTeam.ID, ruser.ID, model.TeamUserRoleID+" "+model.TeamPostAllPublicRoleID)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Email, user.Password)

	post.ChannelID = th.BasicPrivateChannel.ID
	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	post.ChannelID = th.BasicChannel.ID
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)
}

func TestCreatePostAll(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post := &model.Post{ChannelID: th.BasicChannel.ID, Message: "#hashtag a" + model.NewID() + "a"}

	user := model.User{Email: th.GenerateTestEmail(), Nickname: "Joram Wilander", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SystemUserRoleID}

	directChannel, _ := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.ID, th.BasicUser2.ID)

	ruser, resp := Client.CreateUser(&user)
	CheckNoError(t, resp)

	Client.Login(user.Email, user.Password)

	_, resp = Client.CreatePost(post)
	CheckForbiddenStatus(t, resp)

	th.App.UpdateUserRoles(ruser.ID, model.SystemUserRoleID+" "+model.SystemPostAllRoleID, false)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Email, user.Password)

	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelID = th.BasicPrivateChannel.ID
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelID = directChannel.ID
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	th.App.UpdateUserRoles(ruser.ID, model.SystemUserRoleID, false)
	th.App.JoinUserToTeam(th.Context, th.BasicTeam, ruser, "")
	th.App.UpdateTeamMemberRoles(th.BasicTeam.ID, ruser.ID, model.TeamUserRoleID+" "+model.TeamPostAllRoleID)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Email, user.Password)

	post.ChannelID = th.BasicPrivateChannel.ID
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelID = th.BasicChannel.ID
	_, resp = Client.CreatePost(post)
	CheckNoError(t, resp)

	post.ChannelID = directChannel.ID
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

	post1 := &model.Post{ChannelID: th.BasicChannel.ID, Message: "@" + inChannelUser.Username}
	_, resp := Client.CreatePost(post1)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	timeout := time.After(300 * time.Millisecond)
	waiting := true
	for waiting {
		select {
		case event := <-WebSocketClient.EventChannel:
			require.NotEqual(t, model.WebsocketEventEphemeralMessage, event.EventType(), "should not have ephemeral message event")
		case <-timeout:
			waiting = false
		}
	}

	outOfChannelUser := th.CreateUser()
	th.LinkUserToTeam(outOfChannelUser, th.BasicTeam)

	post2 := &model.Post{ChannelID: th.BasicChannel.ID, Message: "@" + outOfChannelUser.Username}
	_, resp = Client.CreatePost(post2)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	timeout = time.After(300 * time.Millisecond)
	waiting = true
	for waiting {
		select {
		case event := <-WebSocketClient.EventChannel:
			if event.EventType() != model.WebsocketEventEphemeralMessage {
				// Ignore any other events
				continue
			}

			wpost := model.PostFromJSON(strings.NewReader(event.GetData()["post"].(string)))

			acm, ok := wpost.GetProp(model.PropsAddChannelMember).(map[string]interface{})
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
				if ev.EventType() == model.WebsocketEventPosted {
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
		ChannelID: th.BasicChannel.ID,
		Message:   "some message",
	}

	req := httptest.NewRequest("POST", "/api/v4/posts?set_online=false", strings.NewReader(post.ToJSON()))
	req.Header.Set(model.HeaderAuth, "Bearer "+session.Token)

	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusCreated, resp.Code)
	waitForEvent(false)

	_, err = th.App.GetStatus(th.BasicUser.ID)
	require.NotNil(t, err)
	assert.Equal(t, "app.status.get.missing.app_error", err.ID)

	req = httptest.NewRequest("POST", "/api/v4/posts", strings.NewReader(post.ToJSON()))
	req.Header.Set(model.HeaderAuth, "Bearer "+session.Token)

	handler.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusCreated, resp.Code)
	waitForEvent(true)

	st, err := th.App.GetStatus(th.BasicUser.ID)
	require.Nil(t, err)
	assert.Equal(t, "online", st.Status)
}

func TestUpdatePost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	th.App.Srv().SetLicense(model.NewTestLicense())

	fileIDs := make([]string, 3)
	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)
	for i := 0; i < len(fileIDs); i++ {
		fileResp, resp := Client.UploadFile(data, channel.ID, "test.png")
		CheckNoError(t, resp)
		fileIDs[i] = fileResp.FileInfos[0].ID
	}

	rpost, appErr := th.App.CreatePost(th.Context, &model.Post{
		UserID:    th.BasicUser.ID,
		ChannelID: channel.ID,
		Message:   "zz" + model.NewID() + "a",
		FileIDs:   fileIDs,
	}, channel, false, true)
	require.Nil(t, appErr)

	assert.Equal(t, rpost.Message, rpost.Message, "full name didn't match")
	assert.EqualValues(t, 0, rpost.EditAt, "Newly created post shouldn't have EditAt set")
	assert.Equal(t, model.StringArray(fileIDs), rpost.FileIDs, "FileIds should have been set")

	t.Run("same message, fewer files", func(t *testing.T) {
		msg := "zz" + model.NewID() + " update post"
		rpost.Message = msg
		rpost.UserID = ""

		rupost, resp := Client.UpdatePost(rpost.ID, &model.Post{
			ID:      rpost.ID,
			Message: rpost.Message,
			FileIDs: fileIDs[0:2], // one fewer file id
		})
		CheckNoError(t, resp)

		assert.Equal(t, rupost.Message, msg, "failed to updates")
		assert.NotEqual(t, 0, rupost.EditAt, "EditAt not updated for post")
		assert.Equal(t, model.StringArray(fileIDs), rupost.FileIDs, "FileIds should have not have been updated")

		actual, resp := Client.GetPost(rpost.ID, "")
		CheckNoError(t, resp)

		assert.Equal(t, actual.Message, msg, "failed to updates")
		assert.NotEqual(t, 0, actual.EditAt, "EditAt not updated for post")
		assert.Equal(t, model.StringArray(fileIDs), actual.FileIDs, "FileIds should have not have been updated")
	})

	t.Run("new message, invalid props", func(t *testing.T) {
		msg1 := "#hashtag a" + model.NewID() + " update post again"
		rpost.Message = msg1
		rpost.AddProp(model.PropsAddChannelMember, "no good")
		rrupost, resp := Client.UpdatePost(rpost.ID, rpost)
		CheckNoError(t, resp)

		assert.Equal(t, msg1, rrupost.Message, "failed to update message")
		assert.Equal(t, "#hashtag", rrupost.Hashtags, "failed to update hashtags")
		assert.Nil(t, rrupost.GetProp(model.PropsAddChannelMember), "failed to sanitize Props['add_channel_member'], should be nil")

		actual, resp := Client.GetPost(rpost.ID, "")
		CheckNoError(t, resp)

		assert.Equal(t, msg1, actual.Message, "failed to update message")
		assert.Equal(t, "#hashtag", actual.Hashtags, "failed to update hashtags")
		assert.Nil(t, actual.GetProp(model.PropsAddChannelMember), "failed to sanitize Props['add_channel_member'], should be nil")
	})

	t.Run("join/leave post", func(t *testing.T) {
		rpost2, err := th.App.CreatePost(th.Context, &model.Post{
			ChannelID: channel.ID,
			Message:   "zz" + model.NewID() + "a",
			Type:      model.PostTypeJoinLeave,
			UserID:    th.BasicUser.ID,
		}, channel, false, true)
		require.Nil(t, err)

		up2 := &model.Post{
			ID:        rpost2.ID,
			ChannelID: channel.ID,
			Message:   "zz" + model.NewID() + " update post 2",
		}
		_, resp := Client.UpdatePost(rpost2.ID, up2)
		CheckBadRequestStatus(t, resp)
	})

	rpost3, appErr := th.App.CreatePost(th.Context, &model.Post{
		ChannelID: channel.ID,
		Message:   "zz" + model.NewID() + "a",
		UserID:    th.BasicUser.ID,
	}, channel, false, true)
	require.Nil(t, appErr)

	t.Run("new message, add files", func(t *testing.T) {
		up3 := &model.Post{
			ID:        rpost3.ID,
			ChannelID: channel.ID,
			Message:   "zz" + model.NewID() + " update post 3",
			FileIDs:   fileIDs[0:2],
		}
		rrupost3, resp := Client.UpdatePost(rpost3.ID, up3)
		CheckNoError(t, resp)
		assert.Empty(t, rrupost3.FileIDs)

		actual, resp := Client.GetPost(rpost.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, model.StringArray(fileIDs), actual.FileIDs)
	})

	t.Run("add slack attachments", func(t *testing.T) {
		up4 := &model.Post{
			ID:        rpost3.ID,
			ChannelID: channel.ID,
			Message:   "zz" + model.NewID() + " update post 3",
		}
		up4.AddProp("attachments", []model.SlackAttachment{
			{
				Text: "Hello World",
			},
		})
		rrupost3, resp := Client.UpdatePost(rpost3.ID, up4)
		CheckNoError(t, resp)
		assert.NotEqual(t, rpost3.EditAt, rrupost3.EditAt)
		assert.NotEqual(t, rpost3.Attachments(), rrupost3.Attachments())
	})

	t.Run("logged out", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.UpdatePost(rpost.ID, rpost)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("different user", func(t *testing.T) {
		th.LoginBasic2()
		_, resp := Client.UpdatePost(rpost.ID, rpost)
		CheckForbiddenStatus(t, resp)

		Client.Logout()
	})

	t.Run("different user, but team admin", func(t *testing.T) {
		th.LoginTeamAdmin()
		_, resp := Client.UpdatePost(rpost.ID, rpost)
		CheckForbiddenStatus(t, resp)

		Client.Logout()
	})

	t.Run("different user, but system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.UpdatePost(rpost.ID, rpost)
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
		ChannelID:     dmChannel.ID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        th.BasicUser.ID,
		CreateAt:      0,
	}

	post, resp := th.Client.CreatePost(post)
	CheckNoError(t, resp)

	post.Message = "changed"
	post, resp = th.SystemAdminClient.UpdatePost(post.ID, post)
	CheckNoError(t, resp)
}

func TestPatchPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	th.App.Srv().SetLicense(model.NewTestLicense())

	fileIDs := make([]string, 3)
	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)
	for i := 0; i < len(fileIDs); i++ {
		fileResp, resp := Client.UploadFile(data, channel.ID, "test.png")
		CheckNoError(t, resp)
		fileIDs[i] = fileResp.FileInfos[0].ID
	}

	post := &model.Post{
		ChannelID:    channel.ID,
		IsPinned:     true,
		Message:      "#hashtag a message",
		Props:        model.StringInterface{"channel_header": "old_header"},
		FileIDs:      fileIDs[0:2],
		HasReactions: true,
	}
	post, _ = Client.CreatePost(post)

	var rpost *model.Post
	t.Run("new message, props, files, HasReactions bit", func(t *testing.T) {
		patch := &model.PostPatch{}

		patch.IsPinned = model.NewBool(false)
		patch.Message = model.NewString("#otherhashtag other message")
		patch.Props = &model.StringInterface{"channel_header": "new_header"}
		patchFileIDs := model.StringArray(fileIDs) // one extra file
		patch.FileIDs = &patchFileIDs
		patch.HasReactions = model.NewBool(false)

		var resp *model.Response
		rpost, resp = Client.PatchPost(post.ID, patch)
		CheckNoError(t, resp)

		assert.False(t, rpost.IsPinned, "IsPinned did not update properly")
		assert.Equal(t, "#otherhashtag other message", rpost.Message, "Message did not update properly")
		assert.Equal(t, *patch.Props, rpost.GetProps(), "Props did not update properly")
		assert.Equal(t, "#otherhashtag", rpost.Hashtags, "Message did not update properly")
		assert.Equal(t, model.StringArray(fileIDs[0:2]), rpost.FileIDs, "FileIds should not update")
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

		rpost2, resp := Client.PatchPost(post.ID, patch2)
		CheckNoError(t, resp)
		assert.NotEmpty(t, rpost2.GetProp("attachments"))
		assert.NotEqual(t, rpost.EditAt, rpost2.EditAt)
	})

	t.Run("invalid requests", func(t *testing.T) {
		r, err := Client.DoApiPut("/posts/"+post.ID+"/patch", "garbage")
		require.EqualError(t, err, ": Invalid or missing post in request body., ")
		require.Equal(t, http.StatusBadRequest, r.StatusCode, "wrong status code")

		patch := &model.PostPatch{}
		_, resp := Client.PatchPost("junk", patch)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("unknown post", func(t *testing.T) {
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(GenerateTestID(), patch)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("logged out", func(t *testing.T) {
		Client.Logout()
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(post.ID, patch)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("different user", func(t *testing.T) {
		th.LoginBasic2()
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(post.ID, patch)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("different user, but team admin", func(t *testing.T) {
		th.LoginTeamAdmin()
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(post.ID, patch)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("different user, but system admin", func(t *testing.T) {
		patch := &model.PostPatch{}
		_, resp := th.SystemAdminClient.PatchPost(post.ID, patch)
		CheckNoError(t, resp)
	})

	t.Run("edit others posts permission can function independently of edit own post", func(t *testing.T) {
		th.LoginBasic2()
		patch := &model.PostPatch{}
		_, resp := Client.PatchPost(post.ID, patch)
		CheckForbiddenStatus(t, resp)

		// Add permission to edit others'
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.RemovePermissionFromRole(model.PermissionEditPost.ID, model.ChannelUserRoleID)
		th.AddPermissionToRole(model.PermissionEditOthersPosts.ID, model.ChannelUserRoleID)

		_, resp = Client.PatchPost(post.ID, patch)
		CheckNoError(t, resp)
	})
}

func TestPinPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post := th.BasicPost
	pass, resp := Client.PinPost(post.ID)
	CheckNoError(t, resp)

	require.True(t, pass, "should have passed")
	rpost, err := th.App.GetSinglePost(post.ID)
	require.Nil(t, err)
	require.True(t, rpost.IsPinned, "failed to pin post")

	pass, resp = Client.PinPost("junk")
	CheckBadRequestStatus(t, resp)
	require.False(t, pass, "should have failed")

	_, resp = Client.PinPost(GenerateTestID())
	CheckForbiddenStatus(t, resp)

	t.Run("unable-to-pin-post-in-read-only-town-square", func(t *testing.T) {
		townSquareIsReadOnly := *th.App.Config().TeamSettings.ExperimentalTownSquareIsReadOnly
		th.App.Srv().SetLicense(model.NewTestLicense())
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.ExperimentalTownSquareIsReadOnly = true })

		defer th.App.Srv().RemoveLicense()
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.ExperimentalTownSquareIsReadOnly = townSquareIsReadOnly })

		channel, err := th.App.GetChannelByName("town-square", th.BasicTeam.ID, true)
		assert.Nil(t, err)
		adminPost := th.CreatePostWithClient(th.SystemAdminClient, channel)

		_, resp = Client.PinPost(adminPost.ID)
		CheckForbiddenStatus(t, resp)
	})

	Client.Logout()
	_, resp = Client.PinPost(post.ID)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.PinPost(post.ID)
	CheckNoError(t, resp)
}

func TestUnpinPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	pinnedPost := th.CreatePinnedPost()
	pass, resp := Client.UnpinPost(pinnedPost.ID)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	rpost, err := th.App.GetSinglePost(pinnedPost.ID)
	require.Nil(t, err)
	require.False(t, rpost.IsPinned)

	pass, resp = Client.UnpinPost("junk")
	CheckBadRequestStatus(t, resp)
	require.False(t, pass, "should have failed")

	_, resp = Client.UnpinPost(GenerateTestID())
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.UnpinPost(pinnedPost.ID)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.UnpinPost(pinnedPost.ID)
	CheckNoError(t, resp)
}

func TestGetPostsForChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	post1 := th.CreatePost()
	post2 := th.CreatePost()
	post3 := &model.Post{ChannelID: th.BasicChannel.ID, Message: "zz" + model.NewID() + "a", RootID: post1.ID}
	post3, _ = Client.CreatePost(post3)

	time.Sleep(300 * time.Millisecond)
	since := model.GetMillis()
	time.Sleep(300 * time.Millisecond)

	post4 := th.CreatePost()

	th.TestForAllClients(t, func(t *testing.T, c *model.Client4) {
		posts, resp := c.GetPostsForChannel(th.BasicChannel.ID, 0, 60, "", false)
		CheckNoError(t, resp)
		require.Equal(t, post4.ID, posts.Order[0], "wrong order")
		require.Equal(t, post3.ID, posts.Order[1], "wrong order")
		require.Equal(t, post2.ID, posts.Order[2], "wrong order")
		require.Equal(t, post1.ID, posts.Order[3], "wrong order")

		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 0, 3, resp.Etag, false)
		CheckEtag(t, posts, resp)

		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 0, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "wrong number returned")

		_, ok := posts.Posts[post3.ID]
		require.True(t, ok, "missing comment")
		_, ok = posts.Posts[post1.ID]
		require.True(t, ok, "missing root post")

		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 1, 1, "", false)
		CheckNoError(t, resp)
		require.Equal(t, post3.ID, posts.Order[0], "wrong order")

		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 10000, 10000, "", false)
		CheckNoError(t, resp)
		require.Empty(t, posts.Order, "should be no posts")
	})

	post5 := th.CreatePost()

	th.TestForAllClients(t, func(t *testing.T, c *model.Client4) {
		posts, resp := c.GetPostsSince(th.BasicChannel.ID, since, false)
		CheckNoError(t, resp)
		require.Len(t, posts.Posts, 2, "should return 2 posts")

		// "since" query to return empty NextPostId and PrevPostId
		require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
		require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")

		found := make([]bool, 2)
		for _, p := range posts.Posts {
			require.LessOrEqual(t, since, p.CreateAt, "bad create at for post returned")

			if p.ID == post4.ID {
				found[0] = true
			} else if p.ID == post5.ID {
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

	_, resp := Client.GetPostsForChannel(model.NewID(), 0, 60, "", false)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPostsForChannel(model.NewID(), 0, 60, "", false)
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
		posts, resp = c.GetPostsBefore(th.BasicChannel.ID, post1.ID, 0, 2, "", false)
		systemPostID1 := posts.Order[1]

		// similar to '/posts'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 0, 60, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 12, "expected 12 posts")
		require.Equal(t, post10.ID, posts.Order[0], "posts not in order")
		require.Equal(t, systemPostID1, posts.Order[11], "posts not in order")
		require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
		require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")

		// similar to '/posts?per_page=3'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 0, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "expected 3 posts")
		require.Equal(t, post10.ID, posts.Order[0], "posts not in order")
		require.Equal(t, post8.ID, posts.Order[2], "should return 3 posts and match order")
		require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
		require.Equal(t, post7.ID, posts.PrevPostID, "should return post7.Id as PrevPostId")

		// similar to '/posts?per_page=3&page=1'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 1, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "expected 3 posts")
		require.Equal(t, post7.ID, posts.Order[0], "posts not in order")
		require.Equal(t, post5.ID, posts.Order[2], "posts not in order")
		require.Equal(t, post8.ID, posts.NextPostID, "should return post8.Id as NextPostId")
		require.Equal(t, post4.ID, posts.PrevPostID, "should return post4.Id as PrevPostId")

		// similar to '/posts?per_page=3&page=2'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 2, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "expected 3 posts")
		require.Equal(t, post4.ID, posts.Order[0], "posts not in order")
		require.Equal(t, post2.ID, posts.Order[2], "should return 3 posts and match order")
		require.Equal(t, post5.ID, posts.NextPostID, "should return post5.Id as NextPostId")
		require.Equal(t, post1.ID, posts.PrevPostID, "should return post1.Id as PrevPostId")

		// similar to '/posts?per_page=3&page=3'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 3, 3, "", false)
		CheckNoError(t, resp)
		require.Len(t, posts.Order, 3, "expected 3 posts")
		require.Equal(t, post1.ID, posts.Order[0], "posts not in order")
		require.Equal(t, systemPostID1, posts.Order[2], "should return 3 posts and match order")
		require.Equal(t, post2.ID, posts.NextPostID, "should return post2.Id as NextPostId")
		require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")

		// similar to '/posts?per_page=3&page=4'
		posts, resp = c.GetPostsForChannel(th.BasicChannel.ID, 4, 3, "", false)
		CheckNoError(t, resp)
		require.Empty(t, posts.Order, "should return 0 post")
		require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
		require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")
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
		UserID:   user.ID,
		Category: model.PreferenceCategoryFlaggedPost,
		Name:     post1.ID,
		Value:    "true",
	}
	_, resp := Client.UpdatePreferences(user.ID, &model.Preferences{preference})
	CheckNoError(t, resp)
	preference.Name = post2.ID
	_, resp = Client.UpdatePreferences(user.ID, &model.Preferences{preference})
	CheckNoError(t, resp)

	opl := model.NewPostList()
	opl.AddPost(post1)
	opl.AddOrder(post1.ID)

	rpl, resp := Client.GetFlaggedPostsForUserInChannel(user.ID, channel1.ID, 0, 10)
	CheckNoError(t, resp)

	require.Len(t, rpl.Posts, 1, "should have returned 1 post")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	rpl, resp = Client.GetFlaggedPostsForUserInChannel(user.ID, channel1.ID, 0, 1)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 1, "should have returned 1 post")

	rpl, resp = Client.GetFlaggedPostsForUserInChannel(user.ID, channel1.ID, 1, 1)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	rpl, resp = Client.GetFlaggedPostsForUserInChannel(user.ID, GenerateTestID(), 0, 10)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	rpl, resp = Client.GetFlaggedPostsForUserInChannel(user.ID, "junk", 0, 10)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rpl)

	opl.AddPost(post2)
	opl.AddOrder(post2.ID)

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.ID, team1.ID, 0, 10)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 2, "should have returned 2 posts")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.ID, team1.ID, 0, 1)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 1, "should have returned 1 post")

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.ID, team1.ID, 1, 1)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 1, "should have returned 1 post")

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.ID, team1.ID, 1000, 10)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.ID, GenerateTestID(), 0, 10)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	rpl, resp = Client.GetFlaggedPostsForUserInTeam(user.ID, "junk", 0, 10)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rpl)

	channel3 := th.CreatePrivateChannel()
	post4 := th.CreatePostWithClient(Client, channel3)

	preference.Name = post4.ID
	Client.UpdatePreferences(user.ID, &model.Preferences{preference})

	opl.AddPost(post4)
	opl.AddOrder(post4.ID)

	rpl, resp = Client.GetFlaggedPostsForUser(user.ID, 0, 10)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 3, "should have returned 3 posts")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	rpl, resp = Client.GetFlaggedPostsForUser(user.ID, 0, 2)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 2, "should have returned 2 posts")

	rpl, resp = Client.GetFlaggedPostsForUser(user.ID, 2, 2)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 1, "should have returned 1 post")

	rpl, resp = Client.GetFlaggedPostsForUser(user.ID, 1000, 10)
	CheckNoError(t, resp)
	require.Empty(t, rpl.Posts)

	channel4 := th.CreateChannelWithClient(th.SystemAdminClient, model.ChannelTypePrivate)
	post5 := th.CreatePostWithClient(th.SystemAdminClient, channel4)

	preference.Name = post5.ID
	_, resp = Client.UpdatePreferences(user.ID, &model.Preferences{preference})
	CheckForbiddenStatus(t, resp)

	rpl, resp = Client.GetFlaggedPostsForUser(user.ID, 0, 10)
	CheckNoError(t, resp)
	require.Len(t, rpl.Posts, 3, "should have returned 3 posts")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	th.AddUserToChannel(user, channel4)
	_, resp = Client.UpdatePreferences(user.ID, &model.Preferences{preference})
	CheckNoError(t, resp)

	rpl, resp = Client.GetFlaggedPostsForUser(user.ID, 0, 10)
	CheckNoError(t, resp)

	opl.AddPost(post5)
	opl.AddOrder(post5.ID)
	require.Len(t, rpl.Posts, 4, "should have returned 4 posts")
	require.Equal(t, opl.Posts, rpl.Posts, "posts should have matched")

	err := th.App.RemoveUserFromChannel(th.Context, user.ID, "", channel4)
	assert.Nil(t, err, "unable to remove user from channel")

	rpl, resp = Client.GetFlaggedPostsForUser(user.ID, 0, 10)
	CheckNoError(t, resp)

	opl2 := model.NewPostList()
	opl2.AddPost(post1)
	opl2.AddOrder(post1.ID)
	opl2.AddPost(post2)
	opl2.AddOrder(post2.ID)
	opl2.AddPost(post4)
	opl2.AddOrder(post4.ID)

	require.Len(t, rpl.Posts, 3, "should have returned 3 posts")
	require.Equal(t, opl2.Posts, rpl.Posts, "posts should have matched")

	_, resp = Client.GetFlaggedPostsForUser("junk", 0, 10)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFlaggedPostsForUser(GenerateTestID(), 0, 10)
	CheckForbiddenStatus(t, resp)

	Client.Logout()

	_, resp = Client.GetFlaggedPostsForUserInChannel(user.ID, channel1.ID, 0, 10)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.GetFlaggedPostsForUserInTeam(user.ID, team1.ID, 0, 10)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.GetFlaggedPostsForUser(user.ID, 0, 10)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetFlaggedPostsForUserInChannel(user.ID, channel1.ID, 0, 10)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetFlaggedPostsForUserInTeam(user.ID, team1.ID, 0, 10)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetFlaggedPostsForUser(user.ID, 0, 10)
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

	_, resp = th.SystemAdminClient.GetFlaggedPostsForUser(user.ID, 0, 10)
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

	posts, resp := Client.GetPostsBefore(th.BasicChannel.ID, post3.ID, 0, 100, "", false)
	CheckNoError(t, resp)

	found := make([]bool, 2)
	for _, p := range posts.Posts {
		if p.ID == post1.ID {
			found[0] = true
		} else if p.ID == post2.ID {
			found[1] = true
		}

		require.NotEqual(t, post4.ID, p.ID, "returned posts after")
		require.NotEqual(t, post5.ID, p.ID, "returned posts after")
	}

	for _, f := range found {
		require.True(t, f, "missing post")
	}

	require.Equal(t, post3.ID, posts.NextPostID, "should match NextPostId")
	require.Equal(t, "", posts.PrevPostID, "should match empty PrevPostId")

	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post4.ID, 1, 1, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "too many posts returned")
	require.Equal(t, post2.ID, posts.Order[0], "should match returned post")
	require.Equal(t, post3.ID, posts.NextPostID, "should match NextPostId")
	require.Equal(t, post1.ID, posts.PrevPostID, "should match PrevPostId")

	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, "junk", 1, 1, "", false)
	CheckBadRequestStatus(t, resp)

	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post5.ID, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 3, "should match length of posts returned")
	require.Equal(t, post4.ID, posts.Order[0], "should match returned post")
	require.Equal(t, post2.ID, posts.Order[2], "should match returned post")
	require.Equal(t, post5.ID, posts.NextPostID, "should match NextPostId")
	require.Equal(t, post1.ID, posts.PrevPostID, "should match PrevPostId")

	// get the system post IDs posted before the created posts above
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post1.ID, 0, 2, "", false)
	CheckNoError(t, resp)
	systemPostID2 := posts.Order[0]
	systemPostID1 := posts.Order[1]

	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post5.ID, 1, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 3, "should match length of posts returned")
	require.Equal(t, post1.ID, posts.Order[0], "should match returned post")
	require.Equal(t, systemPostID2, posts.Order[1], "should match returned post")
	require.Equal(t, systemPostID1, posts.Order[2], "should match returned post")
	require.Equal(t, post2.ID, posts.NextPostID, "should match NextPostId")
	require.Equal(t, "", posts.PrevPostID, "should return empty PrevPostId")

	// more tests for next_post_id, prev_post_id, and order
	// There are 12 posts composed of first 2 system messages and 10 created posts
	post6 := th.CreatePost()
	th.CreatePost() // post7
	post8 := th.CreatePost()
	post9 := th.CreatePost()
	th.CreatePost() // post10

	// similar to '/posts?before=post9'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post9.ID, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 10, "expected 10 posts")
	require.Equal(t, post8.ID, posts.Order[0], "posts not in order")
	require.Equal(t, systemPostID1, posts.Order[9], "posts not in order")
	require.Equal(t, post9.ID, posts.NextPostID, "should return post9.Id as NextPostId")
	require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")

	// similar to '/posts?before=post9&per_page=3'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post9.ID, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post8.ID, posts.Order[0], "posts not in order")
	require.Equal(t, post6.ID, posts.Order[2], "should return 3 posts and match order")
	require.Equal(t, post9.ID, posts.NextPostID, "should return post9.Id as NextPostId")
	require.Equal(t, post5.ID, posts.PrevPostID, "should return post5.Id as PrevPostId")

	// similar to '/posts?before=post9&per_page=3&page=1'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post9.ID, 1, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post5.ID, posts.Order[0], "posts not in order")
	require.Equal(t, post3.ID, posts.Order[2], "posts not in order")
	require.Equal(t, post6.ID, posts.NextPostID, "should return post6.Id as NextPostId")
	require.Equal(t, post2.ID, posts.PrevPostID, "should return post2.Id as PrevPostId")

	// similar to '/posts?before=post9&per_page=3&page=2'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post9.ID, 2, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post2.ID, posts.Order[0], "posts not in order")
	require.Equal(t, systemPostID2, posts.Order[2], "posts not in order")
	require.Equal(t, post3.ID, posts.NextPostID, "should return post3.Id as NextPostId")
	require.Equal(t, systemPostID1, posts.PrevPostID, "should return systemPostId1 as PrevPostId")

	// similar to '/posts?before=post1&per_page=3'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post1.ID, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "expected 2 posts")
	require.Equal(t, systemPostID2, posts.Order[0], "posts not in order")
	require.Equal(t, systemPostID1, posts.Order[1], "posts not in order")
	require.Equal(t, post1.ID, posts.NextPostID, "should return post1.Id as NextPostId")
	require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")

	// similar to '/posts?before=systemPostId1'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, systemPostID1, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, systemPostID1, posts.NextPostID, "should return systemPostId1 as NextPostId")
	require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")

	// similar to '/posts?before=systemPostId1&per_page=60&page=1'
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, systemPostID1, 1, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 posts")
	require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
	require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")

	// similar to '/posts?before=non-existent-post'
	nonExistentPostID := model.NewID()
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, nonExistentPostID, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, nonExistentPostID, posts.NextPostID, "should return nonExistentPostId as NextPostId")
	require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")
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

	posts, resp := Client.GetPostsAfter(th.BasicChannel.ID, post3.ID, 0, 100, "", false)
	CheckNoError(t, resp)

	found := make([]bool, 2)
	for _, p := range posts.Posts {
		if p.ID == post4.ID {
			found[0] = true
		} else if p.ID == post5.ID {
			found[1] = true
		}
		require.NotEqual(t, post1.ID, p.ID, "returned posts before")
		require.NotEqual(t, post2.ID, p.ID, "returned posts before")
	}

	for _, f := range found {
		require.True(t, f, "missing post")
	}
	require.Equal(t, "", posts.NextPostID, "should match empty NextPostId")
	require.Equal(t, post3.ID, posts.PrevPostID, "should match PrevPostId")

	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post2.ID, 1, 1, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "too many posts returned")
	require.Equal(t, post4.ID, posts.Order[0], "should match returned post")
	require.Equal(t, post5.ID, posts.NextPostID, "should match NextPostId")
	require.Equal(t, post3.ID, posts.PrevPostID, "should match PrevPostId")

	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, "junk", 1, 1, "", false)
	CheckBadRequestStatus(t, resp)

	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post1.ID, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 3, "should match length of posts returned")
	require.Equal(t, post4.ID, posts.Order[0], "should match returned post")
	require.Equal(t, post2.ID, posts.Order[2], "should match returned post")
	require.Equal(t, post5.ID, posts.NextPostID, "should match NextPostId")
	require.Equal(t, post1.ID, posts.PrevPostID, "should match PrevPostId")

	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post1.ID, 1, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "should match length of posts returned")
	require.Equal(t, post5.ID, posts.Order[0], "should match returned post")
	require.Equal(t, "", posts.NextPostID, "should match NextPostId")
	require.Equal(t, post4.ID, posts.PrevPostID, "should match PrevPostId")

	// more tests for next_post_id, prev_post_id, and order
	// There are 12 posts composed of first 2 system messages and 10 created posts
	post6 := th.CreatePost()
	th.CreatePost() // post7
	post8 := th.CreatePost()
	post9 := th.CreatePost()
	post10 := th.CreatePost()

	// similar to '/posts?after=post2'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post2.ID, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 8, "expected 8 posts")
	require.Equal(t, post10.ID, posts.Order[0], "should match order")
	require.Equal(t, post3.ID, posts.Order[7], "should match order")
	require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
	require.Equal(t, post2.ID, posts.PrevPostID, "should return post2.Id as PrevPostId")

	// similar to '/posts?after=post2&per_page=3'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post2.ID, 0, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post5.ID, posts.Order[0], "should match order")
	require.Equal(t, post3.ID, posts.Order[2], "should return 3 posts and match order")
	require.Equal(t, post6.ID, posts.NextPostID, "should return post6.Id as NextPostId")
	require.Equal(t, post2.ID, posts.PrevPostID, "should return post2.Id as PrevPostId")

	// similar to '/posts?after=post2&per_page=3&page=1'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post2.ID, 1, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "expected 3 posts")
	require.Equal(t, post8.ID, posts.Order[0], "should match order")
	require.Equal(t, post6.ID, posts.Order[2], "should match order")
	require.Equal(t, post9.ID, posts.NextPostID, "should return post9.Id as NextPostId")
	require.Equal(t, post5.ID, posts.PrevPostID, "should return post5.Id as PrevPostId")

	// similar to '/posts?after=post2&per_page=3&page=2'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post2.ID, 2, 3, "", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "expected 2 posts")
	require.Equal(t, post10.ID, posts.Order[0], "should match order")
	require.Equal(t, post9.ID, posts.Order[1], "should match order")
	require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
	require.Equal(t, post8.ID, posts.PrevPostID, "should return post8.Id as PrevPostId")

	// similar to '/posts?after=post10'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post10.ID, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
	require.Equal(t, post10.ID, posts.PrevPostID, "should return post10.Id as PrevPostId")

	// similar to '/posts?after=post10&page=1'
	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, post10.ID, 1, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
	require.Equal(t, "", posts.PrevPostID, "should return an empty PrevPostId")

	// similar to '/posts?after=non-existent-post'
	nonExistentPostID := model.NewID()
	posts, resp = Client.GetPostsAfter(th.BasicChannel.ID, nonExistentPostID, 0, 60, "", false)
	CheckNoError(t, resp)
	require.Empty(t, posts.Order, "should return 0 post")
	require.Equal(t, "", posts.NextPostID, "should return an empty NextPostId")
	require.Equal(t, nonExistentPostID, posts.PrevPostID, "should return nonExistentPostId as PrevPostId")
}

func TestGetPostsForChannelAroundLastUnread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	userID := th.BasicUser.ID
	channelID := th.BasicChannel.ID

	// 12 posts = 2 systems posts + 10 created posts below
	post1 := th.CreatePost()
	post2 := th.CreatePost()
	post3 := th.CreatePost()
	post4 := th.CreatePost()
	post5 := th.CreatePost()
	replyPost := &model.Post{ChannelID: channelID, Message: model.NewID(), RootID: post4.ID, ParentID: post4.ID}
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

	postIDNames := map[string]string{
		post1.ID:  "post1",
		post2.ID:  "post2",
		post3.ID:  "post3",
		post4.ID:  "post4",
		post5.ID:  "post5",
		post6.ID:  "post6 (reply to post4)",
		post7.ID:  "post7 (reply to post4)",
		post8.ID:  "post8 (reply to post4)",
		post9.ID:  "post9 (reply to post4)",
		post10.ID: "post10 (reply to post4)",
	}

	namePost := func(postID string) string {
		name, ok := postIDNames[postID]
		if ok {
			return name
		}

		return fmt.Sprintf("unknown (%s)", postID)
	}

	namePosts := func(postIDs []string) []string {
		namedPostIDs := make([]string, 0, len(postIDs))
		for _, postID := range postIDs {
			namedPostIDs = append(namedPostIDs, namePost(postID))
		}

		return namedPostIDs
	}

	namePostsMap := func(posts map[string]*model.Post) []string {
		namedPostIDs := make([]string, 0, len(posts))
		for postID := range posts {
			namedPostIDs = append(namedPostIDs, namePost(postID))
		}
		sort.Strings(namedPostIDs)

		return namedPostIDs
	}

	assertPostList := func(t *testing.T, expected, actual *model.PostList) {
		t.Helper()

		require.Equal(t, namePosts(expected.Order), namePosts(actual.Order), "unexpected post order")
		require.Equal(t, namePostsMap(expected.Posts), namePostsMap(actual.Posts), "unexpected posts")
		require.Equal(t, namePost(expected.NextPostID), namePost(actual.NextPostID), "unexpected next post id")
		require.Equal(t, namePost(expected.PrevPostID), namePost(actual.PrevPostID), "unexpected prev post id")
	}

	// Setting limit_after to zero should fail with a 400 BadRequest.
	posts, resp := Client.GetPostsAroundLastUnread(userID, channelID, 20, 0, false)
	require.NotNil(t, resp.Error)
	require.Equal(t, "api.context.invalid_url_param.app_error", resp.Error.ID)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Nil(t, posts)

	// All returned posts are all read by the user, since it's created by the user itself.
	posts, resp = Client.GetPostsAroundLastUnread(userID, channelID, 20, 20, false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 12, "Should return 12 posts only since there's no unread post")

	// Set channel member's last viewed to 0.
	// All returned posts are latest posts as if all previous posts were already read by the user.
	channelMember, err := th.App.Srv().Store.Channel().GetMember(context.Background(), channelID, userID)
	require.NoError(t, err)
	channelMember.LastViewedAt = 0
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelID)

	posts, resp = Client.GetPostsAroundLastUnread(userID, channelID, 20, 20, false)
	CheckNoError(t, resp)

	require.Len(t, posts.Order, 12, "Should return 12 posts only since there's no unread post")

	// get the first system post generated before the created posts above
	posts, resp = Client.GetPostsBefore(th.BasicChannel.ID, post1.ID, 0, 2, "", false)
	CheckNoError(t, resp)
	systemPost0 := posts.Posts[posts.Order[0]]
	postIDNames[systemPost0.ID] = "system post 0"
	systemPost1 := posts.Posts[posts.Order[1]]
	postIDNames[systemPost1.ID] = "system post 1"

	// Set channel member's last viewed before post1.
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelID, userID)
	require.NoError(t, err)
	channelMember.LastViewedAt = post1.CreateAt - 1
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelID)

	posts, resp = Client.GetPostsAroundLastUnread(userID, channelID, 3, 3, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post3.ID, post2.ID, post1.ID, systemPost0.ID, systemPost1.ID},
		Posts: map[string]*model.Post{
			systemPost0.ID: systemPost0,
			systemPost1.ID: systemPost1,
			post1.ID:       post1,
			post2.ID:       post2,
			post3.ID:       post3,
		},
		NextPostID: post4.ID,
		PrevPostID: "",
	}, posts)

	// Set channel member's last viewed before post6.
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelID, userID)
	require.NoError(t, err)
	channelMember.LastViewedAt = post6.CreateAt - 1
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelID)

	posts, resp = Client.GetPostsAroundLastUnread(userID, channelID, 3, 3, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post8.ID, post7.ID, post6.ID, post5.ID, post4.ID, post3.ID},
		Posts: map[string]*model.Post{
			post3.ID:  post3,
			post4.ID:  post4,
			post5.ID:  post5,
			post6.ID:  post6,
			post7.ID:  post7,
			post8.ID:  post8,
			post9.ID:  post9,
			post10.ID: post10,
		},
		NextPostID: post9.ID,
		PrevPostID: post2.ID,
	}, posts)

	// Set channel member's last viewed before post10.
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelID, userID)
	require.NoError(t, err)
	channelMember.LastViewedAt = post10.CreateAt - 1
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelID)

	posts, resp = Client.GetPostsAroundLastUnread(userID, channelID, 3, 3, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post10.ID, post9.ID, post8.ID, post7.ID},
		Posts: map[string]*model.Post{
			post4.ID:  post4,
			post6.ID:  post6,
			post7.ID:  post7,
			post8.ID:  post8,
			post9.ID:  post9,
			post10.ID: post10,
		},
		NextPostID: "",
		PrevPostID: post6.ID,
	}, posts)

	// Set channel member's last viewed equal to post10.
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelID, userID)
	require.NoError(t, err)
	channelMember.LastViewedAt = post10.CreateAt
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelID)

	posts, resp = Client.GetPostsAroundLastUnread(userID, channelID, 3, 3, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post10.ID, post9.ID, post8.ID},
		Posts: map[string]*model.Post{
			post4.ID:  post4,
			post6.ID:  post6,
			post7.ID:  post7,
			post8.ID:  post8,
			post9.ID:  post9,
			post10.ID: post10,
		},
		NextPostID: "",
		PrevPostID: post7.ID,
	}, posts)

	// Set channel member's last viewed to just before a new reply to a previous thread, not
	// otherwise in the requested window.
	post11 := th.CreatePost()
	post12, resp := Client.CreatePost(&model.Post{
		ChannelID: channelID,
		Message:   model.NewID(),
		RootID:    post4.ID,
		ParentID:  post4.ID,
	})
	CheckNoError(t, resp)
	post13 := th.CreatePost()

	postIDNames[post11.ID] = "post11"
	postIDNames[post12.ID] = "post12 (reply to post4)"
	postIDNames[post13.ID] = "post13"

	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channelID, userID)
	require.NoError(t, err)
	channelMember.LastViewedAt = post12.CreateAt - 1
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)
	th.App.Srv().Store.Post().InvalidateLastPostTimeCache(channelID)

	posts, resp = Client.GetPostsAroundLastUnread(userID, channelID, 1, 2, false)
	CheckNoError(t, resp)

	assertPostList(t, &model.PostList{
		Order: []string{post13.ID, post12.ID, post11.ID},
		Posts: map[string]*model.Post{
			post4.ID:  post4,
			post6.ID:  post6,
			post7.ID:  post7,
			post8.ID:  post8,
			post9.ID:  post9,
			post10.ID: post10,
			post11.ID: post11,
			post12.ID: post12,
			post13.ID: post13,
		},
		NextPostID: "",
		PrevPostID: post10.ID,
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

		post, resp := c.GetPost(th.BasicPost.ID, "")
		CheckNoError(t, resp)

		require.Equal(t, th.BasicPost.ID, post.ID, "post ids don't match")

		post, resp = c.GetPost(th.BasicPost.ID, resp.Etag)
		CheckEtag(t, post, resp)

		_, resp = c.GetPost("", "")
		CheckNotFoundStatus(t, resp)

		_, resp = c.GetPost("junk", "")
		CheckBadRequestStatus(t, resp)

		_, resp = c.GetPost(model.NewID(), "")
		CheckNotFoundStatus(t, resp)

		Client.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser.ID)

		// Channel is public, should be able to read post
		_, resp = c.GetPost(th.BasicPost.ID, "")
		CheckNoError(t, resp)

		privatePost = th.CreatePostWithClient(Client, th.BasicPrivateChannel)

		_, resp = c.GetPost(privatePost.ID, "")
		CheckNoError(t, resp)
	})

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.ID, th.BasicUser.ID)

	// Channel is private, should not be able to read post
	_, resp := Client.GetPost(privatePost.ID, "")
	CheckForbiddenStatus(t, resp)

	// But local client should.
	_, resp = th.LocalClient.GetPost(privatePost.ID, "")
	CheckNoError(t, resp)

	Client.Logout()

	// Normal client should get unauthorized, but local client should get 404.
	_, resp = Client.GetPost(model.NewID(), "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.LocalClient.GetPost(model.NewID(), "")
	CheckNotFoundStatus(t, resp)
}

func TestDeletePost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	_, resp := Client.DeletePost("")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.DeletePost("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.DeletePost(th.BasicPost.ID)
	CheckForbiddenStatus(t, resp)

	Client.Login(th.TeamAdminUser.Email, th.TeamAdminUser.Password)
	_, resp = Client.DeletePost(th.BasicPost.ID)
	CheckNoError(t, resp)

	post := th.CreatePost()
	user := th.CreateUser()

	Client.Logout()
	Client.Login(user.Email, user.Password)

	_, resp = Client.DeletePost(post.ID)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.DeletePost(model.NewID())
	CheckUnauthorizedStatus(t, resp)

	status, resp := th.SystemAdminClient.DeletePost(post.ID)
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
		{"Send delete_by to system admin user", th.SystemAdminClient, th.SystemAdminUser.ID},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			wsClient, err := th.CreateWebSocketClientWithClient(tc.client)
			require.Nil(t, err)
			defer wsClient.Close()

			wsClient.Listen()

			post := th.CreatePost()

			status, resp := th.SystemAdminClient.DeletePost(post.ID)
			require.True(t, status, "post should return status OK")
			CheckNoError(t, resp)

			timeout := time.After(5 * time.Second)

			for {
				select {
				case ev := <-wsClient.EventChannel:
					if ev.EventType() == model.WebsocketEventPostDeleted {
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

	post := &model.Post{ChannelID: th.BasicChannel.ID, Message: "zz" + model.NewID() + "a", RootID: th.BasicPost.ID}
	post, _ = Client.CreatePost(post)

	list, resp := Client.GetPostThread(th.BasicPost.ID, "", false)
	CheckNoError(t, resp)

	var list2 *model.PostList
	list2, resp = Client.GetPostThread(th.BasicPost.ID, resp.Etag, false)
	CheckEtag(t, list2, resp)
	require.Equal(t, th.BasicPost.ID, list.Order[0], "wrong order")

	_, ok := list.Posts[th.BasicPost.ID]
	require.True(t, ok, "should have had post")

	_, ok = list.Posts[post.ID]
	require.True(t, ok, "should have had post")

	_, resp = Client.GetPostThread("junk", "", false)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPostThread(model.NewID(), "", false)
	CheckNotFoundStatus(t, resp)

	Client.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser.ID)

	// Channel is public, should be able to read post
	_, resp = Client.GetPostThread(th.BasicPost.ID, "", false)
	CheckNoError(t, resp)

	privatePost := th.CreatePostWithClient(Client, th.BasicPrivateChannel)

	_, resp = Client.GetPostThread(privatePost.ID, "", false)
	CheckNoError(t, resp)

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.ID, th.BasicUser.ID)

	// Channel is private, should not be able to read post
	_, resp = Client.GetPostThread(privatePost.ID, "", false)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPostThread(model.NewID(), "", false)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPostThread(th.BasicPost.ID, "", false)
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
	th.Client.DeleteChannel(archivedChannel.ID)

	terms := "search"
	isOrSearch := false
	timezoneOffset := 5
	searchParams := model.SearchParameter{
		Terms:          &terms,
		IsOrSearch:     &isOrSearch,
		TimeZoneOffset: &timezoneOffset,
	}
	posts, resp := Client.SearchPostsWithParams(th.BasicTeam.ID, &searchParams)
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
	posts2, resp := Client.SearchPostsWithParams(th.BasicTeam.ID, &searchParams)
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
	posts2, resp = Client.SearchPostsWithParams(th.BasicTeam.ID, &searchParams)
	CheckNoError(t, resp)
	// We don't support paging for DB search yet, modify this when we do.
	require.Empty(t, posts2.Order, "Wrong number of posts")

	posts, resp = Client.SearchPosts(th.BasicTeam.ID, "search", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 3, "wrong search")

	posts, resp = Client.SearchPosts(th.BasicTeam.ID, "post2", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 1, "wrong number of posts")
	require.Equal(t, post2.ID, posts.Order[0], "wrong search")

	posts, resp = Client.SearchPosts(th.BasicTeam.ID, "#hashtag", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 1, "wrong number of posts")
	require.Equal(t, post3.ID, posts.Order[0], "wrong search")

	terms = "#hashtag"
	includeDeletedChannels := true
	searchParams = model.SearchParameter{
		Terms:                  &terms,
		IsOrSearch:             &isOrSearch,
		TimeZoneOffset:         &timezoneOffset,
		IncludeDeletedChannels: &includeDeletedChannels,
	}
	posts, resp = Client.SearchPostsWithParams(th.BasicTeam.ID, &searchParams)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "wrong search")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.ExperimentalViewArchivedChannels = false
	})

	posts, resp = Client.SearchPostsWithParams(th.BasicTeam.ID, &searchParams)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 1, "wrong search")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "*", false)
	require.Empty(t, posts.Order, "searching for just * shouldn't return any results")

	posts, resp = Client.SearchPosts(th.BasicTeam.ID, "post1 post2", true)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "wrong search results")

	_, resp = Client.SearchPosts("junk", "#sgtitlereview", false)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.SearchPosts(model.NewID(), "#sgtitlereview", false)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.SearchPosts(th.BasicTeam.ID, "", false)
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.SearchPosts(th.BasicTeam.ID, "#sgtitlereview", false)
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

	posts, resp := Client.SearchPosts(th.BasicTeam.ID, "#sgtitlereview", false)
	CheckNoError(t, resp)
	require.Len(t, posts.Order, 2, "wrong search results")

	Client.Logout()
	_, resp = Client.SearchPosts(th.BasicTeam.ID, "#sgtitlereview", false)
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

	posts, _ := Client.SearchPosts(th.BasicTeam.ID, "channel:", false)
	require.Empty(t, posts.Order, "wrong number of posts for search 'channel:'")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "in:", false)
	require.Empty(t, posts.Order, "wrong number of posts for search 'in:'")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "channel:"+th.BasicChannel.Name, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts returned for search 'channel:%v'", th.BasicChannel.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "in:"+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts returned for search 'in:%v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "channel:"+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'channel:%v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "ChAnNeL:"+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'ChAnNeL:%v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "sgtitlereview", false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'sgtitlereview'")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "sgtitlereview channel:"+th.BasicChannel.Name, false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'sgtitlereview channel:%v'", th.BasicChannel.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "sgtitlereview in: "+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'sgtitlereview in: %v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "sgtitlereview channel: "+th.BasicChannel2.Name, false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'sgtitlereview channel: %v'", th.BasicChannel2.Name)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "channel: "+th.BasicChannel2.Name+" channel: "+channel.Name, false)
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

	posts, _ := Client.SearchPosts(th.BasicTeam.ID, "from: "+th.TeamAdminUser.Username, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'from: %v'", th.TeamAdminUser.Username)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "from: "+th.BasicUser2.Username, false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'from: %v", th.BasicUser2.Username)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "from: "+th.BasicUser2.Username+" sgtitlereview", false)
	require.Lenf(t, posts.Order, 1, "wrong number of posts for search 'from: %v'", th.BasicUser2.Username)

	message = "hullo"
	_ = th.CreateMessagePost(message)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "from: "+th.BasicUser2.Username+" in:"+th.BasicChannel.Name, false)
	require.Len(t, posts.Order, 1, "wrong number of posts for search 'from: %v in:", th.BasicUser2.Username, th.BasicChannel.Name)

	Client.Login(user.Email, user.Password)

	// wait for the join/leave messages to be created for user3 since they're done asynchronously
	time.Sleep(100 * time.Millisecond)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "from: "+th.BasicUser2.Username, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'from: %v'", th.BasicUser2.Username)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "from: "+th.BasicUser2.Username+" from: "+user.Username, false)
	require.Lenf(t, posts.Order, 2, "wrong number of posts for search 'from: %v from: %v'", th.BasicUser2.Username, user.Username)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "from: "+th.BasicUser2.Username+" from: "+user.Username+" in:"+th.BasicChannel2.Name, false)
	require.Len(t, posts.Order, 1, "wrong number of posts")

	message = "coconut"
	_ = th.CreateMessagePostWithClient(Client, th.BasicChannel2, message)

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "from: "+th.BasicUser2.Username+" from: "+user.Username+" in:"+th.BasicChannel2.Name+" coconut", false)
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

	posts, _ := Client.SearchPosts(th.BasicTeam.ID, "return", false)
	require.Len(t, posts.Order, 3, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "on:", false)
	require.Empty(t, posts.Order, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "after:", false)
	require.Empty(t, posts.Order, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "before:", false)
	require.Empty(t, posts.Order, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "on:2018-08-01", false)
	require.Len(t, posts.Order, 1, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "after:2018-08-01", false)
	resultCount := 0
	for _, post := range posts.Posts {
		if post.UserID == th.BasicUser.ID {
			resultCount = resultCount + 1
		}
	}
	require.Equal(t, 2, resultCount, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "before:2018-08-02", false)
	require.Len(t, posts.Order, 1, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "before:2018-08-03 after:2018-08-02", false)
	require.Empty(t, posts.Order, "wrong number of posts")

	posts, _ = Client.SearchPosts(th.BasicTeam.ID, "before:2018-08-03 after:2018-08-01", false)
	require.Len(t, posts.Order, 1, "wrong number of posts")
}

func TestGetFileInfosForPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	fileIDs := make([]string, 3)
	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		fileResp, _ := Client.UploadFile(data, th.BasicChannel.ID, "test.png")
		fileIDs[i] = fileResp.FileInfos[0].ID
	}

	post := &model.Post{ChannelID: th.BasicChannel.ID, Message: "zz" + model.NewID() + "a", FileIDs: fileIDs}
	post, _ = Client.CreatePost(post)

	infos, resp := Client.GetFileInfosForPost(post.ID, "")
	CheckNoError(t, resp)

	require.Len(t, infos, 3, "missing file infos")

	found := false
	for _, info := range infos {
		if info.ID == fileIDs[0] {
			found = true
		}
	}

	require.True(t, found, "missing file info")

	infos, resp = Client.GetFileInfosForPost(post.ID, resp.Etag)
	CheckEtag(t, infos, resp)

	infos, resp = Client.GetFileInfosForPost(th.BasicPost.ID, "")
	CheckNoError(t, resp)

	require.Empty(t, infos, "should have no file infos")

	_, resp = Client.GetFileInfosForPost("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFileInfosForPost(model.NewID(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetFileInfosForPost(model.NewID(), "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetFileInfosForPost(th.BasicPost.ID, "")
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
	c1toc2 := &model.ChannelView{ChannelID: th.BasicChannel2.ID, PrevChannelID: c1.ID}
	now := utils.MillisFromTime(time.Now())
	th.CreateMessagePostNoClient(c1, "AAA", now)
	p2 := th.CreateMessagePostNoClient(c1, "BBB", now+10)
	th.CreateMessagePostNoClient(c1, "CCC", now+20)

	pp1 := th.CreateMessagePostNoClient(th.BasicPrivateChannel, "Sssh!", now)
	pp2 := th.CreateMessagePostNoClient(th.BasicPrivateChannel, "You Sssh!", now+10)
	require.NotNil(t, pp1)
	require.NotNil(t, pp2)

	// Ensure that post have been read
	unread, err := th.App.GetChannelUnread(c1.ID, u1.ID)
	require.Nil(t, err)
	require.Equal(t, int64(4), unread.MsgCount)
	unread, err = th.App.GetChannelUnread(c1.ID, u2.ID)
	require.Nil(t, err)
	require.Equal(t, int64(4), unread.MsgCount)
	_, err = th.App.ViewChannel(c1toc2, u2.ID, s2.ID, false)
	require.Nil(t, err)
	unread, err = th.App.GetChannelUnread(c1.ID, u2.ID)
	require.Nil(t, err)
	require.Equal(t, int64(0), unread.MsgCount)

	t.Run("Unread last one", func(t *testing.T) {
		r := th.Client.SetPostUnread(u1.ID, p2.ID, true)
		checkHTTPStatus(t, r, 200, false)
		unread, err := th.App.GetChannelUnread(c1.ID, u1.ID)
		require.Nil(t, err)
		assert.Equal(t, int64(2), unread.MsgCount)
	})

	t.Run("Unread on a private channel", func(t *testing.T) {
		r := th.Client.SetPostUnread(u1.ID, pp2.ID, true)
		assert.Equal(t, 200, r.StatusCode)
		unread, err := th.App.GetChannelUnread(th.BasicPrivateChannel.ID, u1.ID)
		require.Nil(t, err)
		assert.Equal(t, int64(1), unread.MsgCount)
		r = th.Client.SetPostUnread(u1.ID, pp1.ID, true)
		assert.Equal(t, 200, r.StatusCode)
		unread, err = th.App.GetChannelUnread(th.BasicPrivateChannel.ID, u1.ID)
		require.Nil(t, err)
		assert.Equal(t, int64(2), unread.MsgCount)
	})

	t.Run("Can't unread an imaginary post", func(t *testing.T) {
		r := th.Client.SetPostUnread(u1.ID, "invalid4ofngungryquinj976y", true)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
	})

	// let's create another user to test permissions
	u3 := th.CreateUser()
	c3 := th.CreateClient()
	c3.Login(u3.Email, u3.Password)

	t.Run("Can't unread channels you don't belong to", func(t *testing.T) {
		r := c3.SetPostUnread(u3.ID, pp1.ID, true)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
	})

	t.Run("Can't unread users you don't have permission to edit", func(t *testing.T) {
		r := c3.SetPostUnread(u1.ID, pp1.ID, true)
		assert.Equal(t, http.StatusForbidden, r.StatusCode)
	})

	t.Run("Can't unread if user is not logged in", func(t *testing.T) {
		th.Client.Logout()
		response := th.Client.SetPostUnread(u1.ID, p2.ID, true)
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
		*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDefaultOn
	})

	// user2: first root mention @user1
	//   - user1: hello
	//   - user2: mention @u1
	//   - user1: another repoy
	//   - user2: another mention @u1
	// user1: a root post
	// user2: Another root mention @u1
	user1Mention := " @" + th.BasicUser.Username
	rootPost1, appErr := th.App.CreatePost(th.Context, &model.Post{UserID: th.BasicUser2.ID, CreateAt: model.GetMillis(), ChannelID: th.BasicChannel.ID, Message: "first root mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootID: rootPost1.ID, UserID: th.BasicUser.ID, CreateAt: model.GetMillis(), ChannelID: th.BasicChannel.ID, Message: "hello"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	replyPost1, appErr := th.App.CreatePost(th.Context, &model.Post{RootID: rootPost1.ID, UserID: th.BasicUser2.ID, CreateAt: model.GetMillis(), ChannelID: th.BasicChannel.ID, Message: "mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootID: rootPost1.ID, UserID: th.BasicUser.ID, CreateAt: model.GetMillis(), ChannelID: th.BasicChannel.ID, Message: "another reply"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootID: rootPost1.ID, UserID: th.BasicUser2.ID, CreateAt: model.GetMillis(), ChannelID: th.BasicChannel.ID, Message: "another mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{UserID: th.BasicUser.ID, CreateAt: model.GetMillis(), ChannelID: th.BasicChannel.ID, Message: "a root post"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{UserID: th.BasicUser2.ID, CreateAt: model.GetMillis(), ChannelID: th.BasicChannel.ID, Message: "another root mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)

	t.Run("Mark reply post as unread", func(t *testing.T) {
		resp := th.Client.SetPostUnread(th.BasicUser.ID, replyPost1.ID, false)
		CheckNoError(t, resp)
		channelUnread, appErr := th.App.GetChannelUnread(th.BasicChannel.ID, th.BasicUser.ID)
		require.Nil(t, appErr)

		require.Equal(t, int64(3), channelUnread.MentionCount)
		//  MentionCountRoot should be zero so that supported clients don't show a mention badge for the channel
		require.Equal(t, int64(0), channelUnread.MentionCountRoot)

		require.Equal(t, int64(5), channelUnread.MsgCount)
		//  MentionCountRoot should be zero so that supported clients don't show the channel as unread
		require.Equal(t, channelUnread.MsgCountRoot, int64(0))

		threadMembership, err := th.App.GetThreadMembershipForUser(th.BasicUser.ID, rootPost1.ID)
		require.Nil(t, err)
		thread, err := th.App.GetThreadForUser(th.BasicTeam.ID, threadMembership, false)
		require.Nil(t, err)
		require.Equal(t, int64(2), thread.UnreadMentions)
		require.Equal(t, int64(3), thread.UnreadReplies)
	})

	t.Run("Mark root post as unread", func(t *testing.T) {
		resp := th.Client.SetPostUnread(th.BasicUser.ID, rootPost1.ID, false)
		CheckNoError(t, resp)
		channelUnread, appErr := th.App.GetChannelUnread(th.BasicChannel.ID, th.BasicUser.ID)
		require.Nil(t, appErr)

		require.Equal(t, int64(4), channelUnread.MentionCount)
		require.Equal(t, int64(2), channelUnread.MentionCountRoot)

		require.Equal(t, int64(7), channelUnread.MsgCount)
		require.Equal(t, int64(3), channelUnread.MsgCountRoot)
	})
}
