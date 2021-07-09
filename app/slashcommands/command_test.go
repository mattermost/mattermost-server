// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/httpservice"
)

type InfiniteReader struct {
	Prefix string
}

func (r InfiniteReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 'a'
	}

	return len(p), nil
}

func TestMoveCommand(t *testing.T) {
	th := setup(t)
	defer th.tearDown()

	sourceTeam := th.createTeam()
	targetTeam := th.createTeam()

	command := &model.Command{}
	command.CreatorID = model.NewID()
	command.Method = model.CommandMethodPost
	command.TeamID = sourceTeam.ID
	command.URL = "http://nowhere.com/"
	command.Trigger = "trigger1"

	command, err := th.App.CreateCommand(command)
	assert.Nil(t, err)

	defer func() {
		th.App.PermanentDeleteTeam(sourceTeam)
		th.App.PermanentDeleteTeam(targetTeam)
	}()

	// Move a command and check the team is updated.
	assert.Nil(t, th.App.MoveCommand(targetTeam, command))
	retrievedCommand, err := th.App.GetCommand(command.ID)
	assert.Nil(t, err)
	assert.EqualValues(t, targetTeam.ID, retrievedCommand.TeamID)

	// Move it to the team it's already in. Nothing should change.
	assert.Nil(t, th.App.MoveCommand(targetTeam, command))
	retrievedCommand, err = th.App.GetCommand(command.ID)
	assert.Nil(t, err)
	assert.EqualValues(t, targetTeam.ID, retrievedCommand.TeamID)
}

func TestCreateCommandPost(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	post := &model.Post{
		ChannelID: th.BasicChannel.ID,
		UserID:    th.BasicUser.ID,
		Type:      model.PostTypeSystemGeneric,
	}

	resp := &model.CommandResponse{
		Text: "some message",
	}

	skipSlackParsing := false
	_, err := th.App.CreateCommandPost(th.Context, post, th.BasicTeam.ID, resp, skipSlackParsing)
	require.NotNil(t, err)
	require.Equal(t, err.ID, "api.context.invalid_param.app_error")
}

func TestExecuteCommand(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	t.Run("valid tests with different whitespace characters", func(t *testing.T) {
		TestCases := map[string]string{
			"/code happy path":             "    happy path",
			"/code\nnewline path":          "    newline path",
			"/code\n/nDouble newline path": "    /nDouble newline path",
			"/code  double space":          "     double space",
			"/code\ttab":                   "    tab",
		}

		for TestCase, result := range TestCases {
			args := &model.CommandArgs{
				Command:   TestCase,
				TeamID:    th.BasicTeam.ID,
				ChannelID: th.BasicChannel.ID,
				UserID:    th.BasicUser.ID,
				T:         func(s string, args ...interface{}) string { return s },
			}
			resp, err := th.App.ExecuteCommand(th.Context, args)
			require.Nil(t, err)
			require.NotNil(t, resp)

			assert.Equal(t, resp.Text, result)
		}
	})

	t.Run("missing slash character", func(t *testing.T) {
		argsMissingSlashCharacter := &model.CommandArgs{
			Command: "missing leading slash character",
			T:       func(s string, args ...interface{}) string { return s },
		}
		_, err := th.App.ExecuteCommand(th.Context, argsMissingSlashCharacter)
		require.Equal(t, "api.command.execute_command.format.app_error", err.ID)
	})

	t.Run("empty", func(t *testing.T) {
		argsMissingSlashCharacter := &model.CommandArgs{
			Command: "",
			T:       func(s string, args ...interface{}) string { return s },
		}
		_, err := th.App.ExecuteCommand(th.Context, argsMissingSlashCharacter)
		require.Equal(t, "api.command.execute_command.format.app_error", err.ID)
	})
}

func TestHandleCommandResponsePost(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	command := &model.Command{}
	args := &model.CommandArgs{
		ChannelID: th.BasicChannel.ID,
		TeamID:    th.BasicTeam.ID,
		UserID:    th.BasicUser.ID,
		RootID:    "",
		ParentID:  "",
	}

	resp := &model.CommandResponse{
		Type:         model.PostTypeDefault,
		ResponseType: model.CommandResponseTypeInChannel,
		Props:        model.StringInterface{"some_key": "some value"},
		Text:         "some message",
	}

	builtIn := true

	post, err := th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, args.ChannelID, post.ChannelID)
	assert.Equal(t, args.RootID, post.RootID)
	assert.Equal(t, args.ParentID, post.ParentID)
	assert.Equal(t, args.UserID, post.UserID)
	assert.Equal(t, resp.Type, post.Type)
	assert.Equal(t, resp.Props, post.GetProps())
	assert.Equal(t, resp.Text, post.Message)
	assert.Nil(t, post.GetProp("override_icon_url"))
	assert.Nil(t, post.GetProp("override_username"))
	assert.Nil(t, post.GetProp("from_webhook"))

	// Command is not built in, so it is a bot command.
	builtIn = false
	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, "true", post.GetProp("from_webhook"))

	builtIn = true

	// Channel id is specified by response, it should override the command args value.
	channel := th.CreateChannel(th.BasicTeam)
	resp.ChannelID = channel.ID
	th.addUserToChannel(th.BasicUser, channel)

	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, resp.ChannelID, post.ChannelID)
	assert.NotEqual(t, args.ChannelID, post.ChannelID)

	// Override username config is turned off. No override should occur.
	*th.App.Config().ServiceSettings.EnablePostUsernameOverride = false
	resp.ChannelID = ""
	command.Username = "Command username"
	resp.Username = "Response username"

	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Nil(t, post.GetProp("override_username"))

	*th.App.Config().ServiceSettings.EnablePostUsernameOverride = true

	// Override username config is turned on. Override username through command property.
	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, command.Username, post.GetProp("override_username"))
	assert.Equal(t, "true", post.GetProp("from_webhook"))

	command.Username = ""

	// Override username through response property.
	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, resp.Username, post.GetProp("override_username"))
	assert.Equal(t, "true", post.GetProp("from_webhook"))

	*th.App.Config().ServiceSettings.EnablePostUsernameOverride = false

	// Override icon url config is turned off. No override should occur.
	*th.App.Config().ServiceSettings.EnablePostIconOverride = false
	command.IconURL = "Command icon url"
	resp.IconURL = "Response icon url"

	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Nil(t, post.GetProp("override_icon_url"))

	*th.App.Config().ServiceSettings.EnablePostIconOverride = true

	// Override icon url config is turned on. Override icon url through command property.
	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, command.IconURL, post.GetProp("override_icon_url"))
	assert.Equal(t, "true", post.GetProp("from_webhook"))

	command.IconURL = ""

	// Override icon url through response property.
	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, resp.IconURL, post.GetProp("override_icon_url"))
	assert.Equal(t, "true", post.GetProp("from_webhook"))

	// Test Slack text conversion.
	resp.Text = "<!channel>"

	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, "@channel", post.Message)
	assert.Equal(t, "true", post.GetProp("from_webhook"))

	// Test Slack attachments text conversion.
	resp.Attachments = []*model.SlackAttachment{
		{
			Text: "<!here>",
		},
	}

	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
	assert.Equal(t, "@channel", post.Message)
	if assert.Len(t, post.Attachments(), 1) {
		assert.Equal(t, "@here", post.Attachments()[0].Text)
	}
	assert.Equal(t, "true", post.GetProp("from_webhook"))

	channel = th.createPrivateChannel(th.BasicTeam)
	resp.ChannelID = channel.ID
	args.UserID = th.BasicUser2.ID
	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)

	require.NotNil(t, err)
	require.Equal(t, err.ID, "api.command.command_post.forbidden.app_error")

	// Test that /code text is not converted with the Slack text conversion.
	command.Trigger = "code"
	resp.ChannelID = ""
	resp.Text = "<test.com|test website>"
	resp.Attachments = []*model.SlackAttachment{
		{
			Text: "<!here>",
		},
	}

	// set and unset SkipSlackParsing here seems the nicest way as no separate response objects are created for every testcase.
	resp.SkipSlackParsing = true
	post, err = th.App.HandleCommandResponsePost(th.Context, command, args, resp, builtIn)
	resp.SkipSlackParsing = false

	assert.Nil(t, err)
	assert.Equal(t, resp.Text, post.Message, "/code text should not be converted to Slack links")
	assert.Equal(t, "<!here>", resp.Attachments[0].Text)
}

func TestHandleCommandResponse(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	command := &model.Command{}

	args := &model.CommandArgs{
		Command:   "/invite username",
		UserID:    th.BasicUser.ID,
		ChannelID: th.BasicChannel.ID,
	}

	resp := &model.CommandResponse{
		Text: "message 1",
		Type: model.PostTypeSystemGeneric,
	}

	builtIn := true

	_, err := th.App.HandleCommandResponse(th.Context, command, args, resp, builtIn)
	require.NotNil(t, err)
	require.Equal(t, err.ID, "api.command.execute_command.create_post_failed.app_error")

	resp = &model.CommandResponse{
		Text: "message 1",
	}

	_, err = th.App.HandleCommandResponse(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)

	resp = &model.CommandResponse{
		Text: "message 1",
		ExtraResponses: []*model.CommandResponse{
			{
				Text: "message 2",
			},
			{
				Type: model.PostTypeSystemGeneric,
				Text: "message 3",
			},
		},
	}

	_, err = th.App.HandleCommandResponse(th.Context, command, args, resp, builtIn)
	require.NotNil(t, err)
	require.Equal(t, err.ID, "api.command.execute_command.create_post_failed.app_error")

	resp = &model.CommandResponse{
		ExtraResponses: []*model.CommandResponse{
			{},
			{},
		},
	}

	_, err = th.App.HandleCommandResponse(th.Context, command, args, resp, builtIn)
	assert.Nil(t, err)
}

func TestDoCommandRequest(t *testing.T) {
	th := setup(t)
	defer th.tearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.ServiceSettings.AllowedUntrustedInternalConnections = model.NewString("127.0.0.1")
		cfg.ServiceSettings.EnableCommands = model.NewBool(true)
	})

	t.Run("with a valid text response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(w, strings.NewReader("Hello, World!"))
		}))
		defer server.Close()

		_, resp, err := th.App.DoCommandRequest(&model.Command{URL: server.URL}, url.Values{})
		require.Nil(t, err)

		assert.NotNil(t, resp)
		assert.Equal(t, "Hello, World!", resp.Text)
	})

	t.Run("with a valid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")

			io.Copy(w, strings.NewReader(`{"text": "Hello, World!"}`))
		}))
		defer server.Close()

		_, resp, err := th.App.DoCommandRequest(&model.Command{URL: server.URL}, url.Values{})
		require.Nil(t, err)

		assert.NotNil(t, resp)
		assert.Equal(t, "Hello, World!", resp.Text)
	})

	t.Run("with a large text response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(w, InfiniteReader{})
		}))
		defer server.Close()

		// Since we limit the length of the response, no error will be returned and resp.Text will be a finite string

		_, resp, err := th.App.DoCommandRequest(&model.Command{URL: server.URL}, url.Values{})
		require.Nil(t, err)
		require.NotNil(t, resp)
	})

	t.Run("with a large, valid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")

			io.Copy(w, io.MultiReader(strings.NewReader(`{"text": "`), InfiniteReader{}, strings.NewReader(`"}`)))
		}))
		defer server.Close()

		_, _, err := th.App.DoCommandRequest(&model.Command{URL: server.URL}, url.Values{})
		require.NotNil(t, err)
		require.Equal(t, "api.command.execute_command.failed.app_error", err.ID)
	})

	t.Run("with a large, invalid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")

			io.Copy(w, InfiniteReader{})
		}))
		defer server.Close()

		_, _, err := th.App.DoCommandRequest(&model.Command{URL: server.URL}, url.Values{})
		require.NotNil(t, err)
		require.Equal(t, "api.command.execute_command.failed.app_error", err.ID)
	})

	t.Run("with a slow response", func(t *testing.T) {
		done := make(chan bool)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-done
			io.Copy(w, strings.NewReader(`{"text": "Hello, World!"}`))
		}))
		defer server.Close()

		th.App.HTTPService().(*httpservice.HTTPServiceImpl).RequestTimeout = 100 * time.Millisecond
		defer func() {
			th.App.HTTPService().(*httpservice.HTTPServiceImpl).RequestTimeout = httpservice.RequestTimeout
		}()

		_, _, err := th.App.DoCommandRequest(&model.Command{URL: server.URL}, url.Values{})
		require.NotNil(t, err)
		require.Equal(t, "api.command.execute_command.failed.app_error", err.ID)
		close(done)
	})
}

func TestMentionsToTeamMembers(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	otherTeam := th.createTeam()
	otherUser := th.createUser()
	th.linkUserToTeam(otherUser, otherTeam)

	fixture := []struct {
		message     string
		inTeam      string
		expectedMap model.UserMentionMap
	}{
		{
			"",
			th.BasicTeam.ID,
			model.UserMentionMap{},
		},
		{
			"/trigger",
			th.BasicTeam.ID,
			model.UserMentionMap{},
		},
		{
			"/trigger 0 mentions",
			th.BasicTeam.ID,
			model.UserMentionMap{},
		},
		{
			fmt.Sprintf("/trigger 1 valid user @%s", th.BasicUser.Username),
			th.BasicTeam.ID,
			model.UserMentionMap{th.BasicUser.Username: th.BasicUser.ID},
		},
		{
			fmt.Sprintf("/trigger 2 valid users @%s @%s",
				th.BasicUser.Username, th.BasicUser2.Username,
			),
			th.BasicTeam.ID,
			model.UserMentionMap{
				th.BasicUser.Username:  th.BasicUser.ID,
				th.BasicUser2.Username: th.BasicUser2.ID,
			},
		},
		{
			fmt.Sprintf("/trigger 1 user from another team @%s", otherUser.Username),
			th.BasicTeam.ID,
			model.UserMentionMap{},
		},
		{
			fmt.Sprintf("/trigger 2 valid users + 1 from another team @%s @%s @%s",
				th.BasicUser.Username, th.BasicUser2.Username, otherUser.Username,
			),
			th.BasicTeam.ID,
			model.UserMentionMap{
				th.BasicUser.Username:  th.BasicUser.ID,
				th.BasicUser2.Username: th.BasicUser2.ID,
			},
		},
		{
			fmt.Sprintf("/trigger a valid channel ~%s", th.BasicChannel.Name),
			th.BasicTeam.ID,
			model.UserMentionMap{},
		},
		{
			fmt.Sprintf("/trigger channel and mentions ~%s @%s",
				th.BasicChannel.Name, th.BasicUser.Username),
			th.BasicTeam.ID,
			model.UserMentionMap{th.BasicUser.Username: th.BasicUser.ID},
		},
		{
			fmt.Sprintf("/trigger repeated users @%s @%s @%s",
				th.BasicUser.Username, th.BasicUser2.Username, th.BasicUser.Username),
			th.BasicTeam.ID,
			model.UserMentionMap{
				th.BasicUser.Username:  th.BasicUser.ID,
				th.BasicUser2.Username: th.BasicUser2.ID,
			},
		},
	}

	for _, data := range fixture {
		actualMap := th.App.MentionsToTeamMembers(data.message, data.inTeam)
		require.Equal(t, actualMap, data.expectedMap)
	}
}

func TestMentionsToPublicChannels(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	otherPublicChannel := th.CreateChannel(th.BasicTeam)
	privateChannel := th.createPrivateChannel(th.BasicTeam)

	fixture := []struct {
		message     string
		inTeam      string
		expectedMap model.ChannelMentionMap
	}{
		{
			"",
			th.BasicTeam.ID,
			model.ChannelMentionMap{},
		},
		{
			"/trigger",
			th.BasicTeam.ID,
			model.ChannelMentionMap{},
		},
		{
			"/trigger 0 mentions",
			th.BasicTeam.ID,
			model.ChannelMentionMap{},
		},
		{
			fmt.Sprintf("/trigger 1 public channel ~%s", th.BasicChannel.Name),
			th.BasicTeam.ID,
			model.ChannelMentionMap{th.BasicChannel.Name: th.BasicChannel.ID},
		},
		{
			fmt.Sprintf("/trigger 2 public channels ~%s ~%s",
				th.BasicChannel.Name, otherPublicChannel.Name,
			),
			th.BasicTeam.ID,
			model.ChannelMentionMap{
				th.BasicChannel.Name:    th.BasicChannel.ID,
				otherPublicChannel.Name: otherPublicChannel.ID,
			},
		},
		{
			fmt.Sprintf("/trigger 1 private channel ~%s", privateChannel.Name),
			th.BasicTeam.ID,
			model.ChannelMentionMap{},
		},
		{
			fmt.Sprintf("/trigger 2 public channel + 1 private ~%s ~%s ~%s",
				th.BasicChannel.Name, otherPublicChannel.Name, privateChannel.Name,
			),
			th.BasicTeam.ID,
			model.ChannelMentionMap{
				th.BasicChannel.Name:    th.BasicChannel.ID,
				otherPublicChannel.Name: otherPublicChannel.ID,
			},
		},
		{
			fmt.Sprintf("/trigger a valid user @%s", th.BasicUser.Username),
			th.BasicTeam.ID,
			model.ChannelMentionMap{},
		},
		{
			fmt.Sprintf("/trigger channel and mentions ~%s @%s",
				th.BasicChannel.Name, th.BasicUser.Username),
			th.BasicTeam.ID,
			model.ChannelMentionMap{th.BasicChannel.Name: th.BasicChannel.ID},
		},
		{
			fmt.Sprintf("/trigger repeated channels ~%s ~%s ~%s",
				th.BasicChannel.Name, otherPublicChannel.Name, th.BasicChannel.Name),
			th.BasicTeam.ID,
			model.ChannelMentionMap{
				th.BasicChannel.Name:    th.BasicChannel.ID,
				otherPublicChannel.Name: otherPublicChannel.ID,
			},
		},
	}

	for _, data := range fixture {
		actualMap := th.App.MentionsToPublicChannels(data.message, data.inTeam)
		require.Equal(t, actualMap, data.expectedMap)
	}
}
