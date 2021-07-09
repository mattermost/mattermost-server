// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestCreateCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	LocalClient := th.LocalClient

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })

	newCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "trigger"}

	_, resp := Client.CreateCommand(newCmd)
	CheckForbiddenStatus(t, resp)

	createdCmd, resp := th.SystemAdminClient.CreateCommand(newCmd)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.Equal(t, th.SystemAdminUser.ID, createdCmd.CreatorID, "user ids didn't match")
	require.Equal(t, th.BasicTeam.ID, createdCmd.TeamID, "team ids didn't match")

	_, resp = th.SystemAdminClient.CreateCommand(newCmd)
	CheckBadRequestStatus(t, resp)
	CheckErrorMessage(t, resp, "api.command.duplicate_trigger.app_error")

	newCmd.Trigger = "Local"
	localCreatedCmd, resp := LocalClient.CreateCommand(newCmd)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.Equal(t, th.BasicUser.ID, localCreatedCmd.CreatorID, "local client: user ids didn't match")
	require.Equal(t, th.BasicTeam.ID, localCreatedCmd.TeamID, "local client: team ids didn't match")

	newCmd.Method = "Wrong"
	newCmd.Trigger = "testcommand"
	_, resp = th.SystemAdminClient.CreateCommand(newCmd)
	CheckBadRequestStatus(t, resp)
	CheckErrorMessage(t, resp, "model.command.is_valid.method.app_error")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = false })
	newCmd.Method = "P"
	newCmd.Trigger = "testcommand"
	_, resp = th.SystemAdminClient.CreateCommand(newCmd)
	CheckNotImplementedStatus(t, resp)
	CheckErrorMessage(t, resp, "api.command.disabled.app_error")

	// Confirm that local clients can't override disable command setting
	newCmd.Trigger = "LocalOverride"
	_, resp = LocalClient.CreateCommand(newCmd)
	CheckErrorMessage(t, resp, "api.command.disabled.app_error")
}

func TestUpdateCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.SystemAdminUser
	team := th.BasicTeam

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })

	cmd1 := &model.Command{
		CreatorID: user.ID,
		TeamID:    team.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "trigger1",
	}

	cmd1, _ = th.App.CreateCommand(cmd1)

	cmd2 := &model.Command{
		CreatorID: GenerateTestID(),
		TeamID:    team.ID,
		URL:       "http://nowhere.com/change",
		Method:    model.CommandMethodGet,
		Trigger:   "trigger2",
		ID:        cmd1.ID,
		Token:     "tokenchange",
	}

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		rcmd, resp := client.UpdateCommand(cmd2)
		CheckNoError(t, resp)

		require.Equal(t, cmd2.Trigger, rcmd.Trigger, "Trigger should have updated")

		require.Equal(t, cmd2.Method, rcmd.Method, "Method should have updated")

		require.Equal(t, cmd2.URL, rcmd.URL, "URL should have updated")

		require.Equal(t, cmd1.CreatorID, rcmd.CreatorID, "CreatorId should have not updated")

		require.Equal(t, cmd1.Token, rcmd.Token, "Token should have not updated")

		cmd2.ID = GenerateTestID()

		rcmd, resp = client.UpdateCommand(cmd2)
		CheckNotFoundStatus(t, resp)

		require.Nil(t, rcmd, "should be empty")

		cmd2.ID = "junk"

		_, resp = client.UpdateCommand(cmd2)
		CheckBadRequestStatus(t, resp)

		cmd2.ID = cmd1.ID
		cmd2.TeamID = GenerateTestID()

		_, resp = client.UpdateCommand(cmd2)
		CheckBadRequestStatus(t, resp)

		cmd2.TeamID = team.ID

		_, resp = th.Client.UpdateCommand(cmd2)
		CheckNotFoundStatus(t, resp)
	})
	th.SystemAdminClient.Logout()
	_, resp := th.SystemAdminClient.UpdateCommand(cmd2)
	CheckUnauthorizedStatus(t, resp)
}

func TestMoveCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.SystemAdminUser
	team := th.BasicTeam
	newTeam := th.CreateTeam()

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })

	cmd1 := &model.Command{
		CreatorID: user.ID,
		TeamID:    team.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "trigger1",
	}

	rcmd1, _ := th.App.CreateCommand(cmd1)
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {

		ok, resp := client.MoveCommand(newTeam.ID, rcmd1.ID)
		CheckNoError(t, resp)
		require.True(t, ok)

		rcmd1, _ = th.App.GetCommand(rcmd1.ID)
		require.NotNil(t, rcmd1)
		require.Equal(t, newTeam.ID, rcmd1.TeamID)

		ok, resp = client.MoveCommand(newTeam.ID, "bogus")
		CheckBadRequestStatus(t, resp)
		require.False(t, ok)

		ok, resp = client.MoveCommand(GenerateTestID(), rcmd1.ID)
		CheckNotFoundStatus(t, resp)
		require.False(t, ok)
	})
	cmd2 := &model.Command{
		CreatorID: user.ID,
		TeamID:    team.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "trigger2",
	}

	rcmd2, _ := th.App.CreateCommand(cmd2)

	_, resp := th.Client.MoveCommand(newTeam.ID, rcmd2.ID)
	CheckNotFoundStatus(t, resp)

	th.SystemAdminClient.Logout()
	_, resp = th.SystemAdminClient.MoveCommand(newTeam.ID, rcmd2.ID)
	CheckUnauthorizedStatus(t, resp)
}

func TestDeleteCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.SystemAdminUser
	team := th.BasicTeam

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })

	cmd1 := &model.Command{
		CreatorID: user.ID,
		TeamID:    team.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "trigger1",
	}

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		cmd1.ID = ""
		rcmd1, err := th.App.CreateCommand(cmd1)
		require.Nil(t, err)
		ok, resp := client.DeleteCommand(rcmd1.ID)
		CheckNoError(t, resp)

		require.True(t, ok)

		rcmd1, _ = th.App.GetCommand(rcmd1.ID)
		require.Nil(t, rcmd1)

		ok, resp = client.DeleteCommand("junk")
		CheckBadRequestStatus(t, resp)

		require.False(t, ok)

		_, resp = client.DeleteCommand(GenerateTestID())
		CheckNotFoundStatus(t, resp)
	})
	cmd2 := &model.Command{
		CreatorID: user.ID,
		TeamID:    team.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "trigger2",
	}

	rcmd2, _ := th.App.CreateCommand(cmd2)

	_, resp := th.Client.DeleteCommand(rcmd2.ID)
	CheckNotFoundStatus(t, resp)

	th.SystemAdminClient.Logout()
	_, resp = th.SystemAdminClient.DeleteCommand(rcmd2.ID)
	CheckUnauthorizedStatus(t, resp)
}

func TestListCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })

	newCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "custom_command"}

	_, resp := th.SystemAdminClient.CreateCommand(newCmd)
	CheckNoError(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		listCommands, resp := c.ListCommands(th.BasicTeam.ID, false)
		CheckNoError(t, resp)

		foundEcho := false
		foundCustom := false
		for _, command := range listCommands {
			if command.Trigger == "echo" {
				foundEcho = true
			}
			if command.Trigger == "custom_command" {
				foundCustom = true
			}
		}
		require.True(t, foundEcho, "Couldn't find echo command")
		require.True(t, foundCustom, "Should list the custom command")
	}, "ListSystemAndCustomCommands")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		listCommands, resp := c.ListCommands(th.BasicTeam.ID, true)
		CheckNoError(t, resp)

		require.Len(t, listCommands, 1, "Should list just one custom command")
		require.Equal(t, listCommands[0].Trigger, "custom_command", "Wrong custom command trigger")
	}, "ListCustomOnlyCommands")

	t.Run("UserWithNoPermissionForCustomCommands", func(t *testing.T) {
		_, resp := Client.ListCommands(th.BasicTeam.ID, true)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("RegularUserCanListOnlySystemCommands", func(t *testing.T) {
		listCommands, resp := Client.ListCommands(th.BasicTeam.ID, false)
		CheckNoError(t, resp)

		foundEcho := false
		foundCustom := false
		for _, command := range listCommands {
			if command.Trigger == "echo" {
				foundEcho = true
			}
			if command.Trigger == "custom_command" {
				foundCustom = true
			}
		}
		require.True(t, foundEcho, "Couldn't find echo command")
		require.False(t, foundCustom, "Should not list the custom command")
	})

	t.Run("NoMember", func(t *testing.T) {
		Client.Logout()
		user := th.CreateUser()
		th.SystemAdminClient.RemoveTeamMember(th.BasicTeam.ID, user.ID)
		Client.Login(user.Email, user.Password)
		_, resp := Client.ListCommands(th.BasicTeam.ID, false)
		CheckForbiddenStatus(t, resp)
		_, resp = Client.ListCommands(th.BasicTeam.ID, true)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("NotLoggedIn", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.ListCommands(th.BasicTeam.ID, false)
		CheckUnauthorizedStatus(t, resp)
		_, resp = Client.ListCommands(th.BasicTeam.ID, true)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestListAutocompleteCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	newCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "custom_command"}

	_, resp := th.SystemAdminClient.CreateCommand(newCmd)
	CheckNoError(t, resp)

	t.Run("ListAutocompleteCommandsOnly", func(t *testing.T) {
		listCommands, resp := th.SystemAdminClient.ListAutocompleteCommands(th.BasicTeam.ID)
		CheckNoError(t, resp)

		foundEcho := false
		foundCustom := false
		for _, command := range listCommands {
			if command.Trigger == "echo" {
				foundEcho = true
			}
			if command.Trigger == "custom_command" {
				foundCustom = true
			}
		}
		require.True(t, foundEcho, "Couldn't find echo command")
		require.False(t, foundCustom, "Should not list the custom command")
	})

	t.Run("RegularUserCanListOnlySystemCommands", func(t *testing.T) {
		listCommands, resp := Client.ListAutocompleteCommands(th.BasicTeam.ID)
		CheckNoError(t, resp)

		foundEcho := false
		foundCustom := false
		for _, command := range listCommands {
			if command.Trigger == "echo" {
				foundEcho = true
			}
			if command.Trigger == "custom_command" {
				foundCustom = true
			}
		}
		require.True(t, foundEcho, "Couldn't find echo command")
		require.False(t, foundCustom, "Should not list the custom command")
	})

	t.Run("NoMember", func(t *testing.T) {
		Client.Logout()
		user := th.CreateUser()
		th.SystemAdminClient.RemoveTeamMember(th.BasicTeam.ID, user.ID)
		Client.Login(user.Email, user.Password)
		_, resp := Client.ListAutocompleteCommands(th.BasicTeam.ID)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("NotLoggedIn", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.ListAutocompleteCommands(th.BasicTeam.ID)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestListCommandAutocompleteSuggestions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	newCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "custom_command"}

	_, resp := th.SystemAdminClient.CreateCommand(newCmd)
	CheckNoError(t, resp)

	t.Run("ListAutocompleteSuggestionsOnly", func(t *testing.T) {
		suggestions, resp := th.SystemAdminClient.ListCommandAutocompleteSuggestions("/", th.BasicTeam.ID)
		CheckNoError(t, resp)

		foundEcho := false
		foundShrug := false
		foundCustom := false
		for _, command := range suggestions {
			if command.Suggestion == "echo" {
				foundEcho = true
			}
			if command.Suggestion == "shrug" {
				foundShrug = true
			}
			if command.Suggestion == "custom_command" {
				foundCustom = true
			}
		}
		require.True(t, foundEcho, "Couldn't find echo command")
		require.True(t, foundShrug, "Couldn't find shrug command")
		require.False(t, foundCustom, "Should not list the custom command")
	})

	t.Run("ListAutocompleteSuggestionsOnlyWithInput", func(t *testing.T) {
		suggestions, resp := th.SystemAdminClient.ListCommandAutocompleteSuggestions("/e", th.BasicTeam.ID)
		CheckNoError(t, resp)

		foundEcho := false
		foundShrug := false
		for _, command := range suggestions {
			if command.Suggestion == "echo" {
				foundEcho = true
			}
			if command.Suggestion == "shrug" {
				foundShrug = true
			}
		}
		require.True(t, foundEcho, "Couldn't find echo command")
		require.False(t, foundShrug, "Should not list the shrug command")
	})

	t.Run("RegularUserCanListOnlySystemCommands", func(t *testing.T) {
		suggestions, resp := Client.ListCommandAutocompleteSuggestions("/", th.BasicTeam.ID)
		CheckNoError(t, resp)

		foundEcho := false
		foundCustom := false
		for _, suggestion := range suggestions {
			if suggestion.Suggestion == "echo" {
				foundEcho = true
			}
			if suggestion.Suggestion == "custom_command" {
				foundCustom = true
			}
		}
		require.True(t, foundEcho, "Couldn't find echo command")
		require.False(t, foundCustom, "Should not list the custom command")
	})

	t.Run("NoMember", func(t *testing.T) {
		Client.Logout()
		user := th.CreateUser()
		th.SystemAdminClient.RemoveTeamMember(th.BasicTeam.ID, user.ID)
		Client.Login(user.Email, user.Password)
		_, resp := Client.ListCommandAutocompleteSuggestions("/", th.BasicTeam.ID)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("NotLoggedIn", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.ListCommandAutocompleteSuggestions("/", th.BasicTeam.ID)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestGetCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })

	newCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "roger"}

	newCmd, resp := th.SystemAdminClient.CreateCommand(newCmd)
	CheckNoError(t, resp)
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {

		t.Run("ValidId", func(t *testing.T) {
			cmd, resp := client.GetCommandByID(newCmd.ID)
			CheckNoError(t, resp)

			require.Equal(t, newCmd.ID, cmd.ID)
			require.Equal(t, newCmd.CreatorID, cmd.CreatorID)
			require.Equal(t, newCmd.TeamID, cmd.TeamID)
			require.Equal(t, newCmd.URL, cmd.URL)
			require.Equal(t, newCmd.Method, cmd.Method)
			require.Equal(t, newCmd.Trigger, cmd.Trigger)
		})

		t.Run("InvalidId", func(t *testing.T) {
			_, resp := client.GetCommandByID(strings.Repeat("z", len(newCmd.ID)))
			require.NotNil(t, resp.Error)
		})
	})
	t.Run("UserWithNoPermissionForCustomCommands", func(t *testing.T) {
		_, resp := th.Client.GetCommandByID(newCmd.ID)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("NoMember", func(t *testing.T) {
		th.Client.Logout()
		user := th.CreateUser()
		th.SystemAdminClient.RemoveTeamMember(th.BasicTeam.ID, user.ID)
		th.Client.Login(user.Email, user.Password)
		_, resp := th.Client.GetCommandByID(newCmd.ID)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("NotLoggedIn", func(t *testing.T) {
		th.Client.Logout()
		_, resp := th.Client.GetCommandByID(newCmd.ID)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestRegenToken(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })

	newCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "trigger"}

	createdCmd, resp := th.SystemAdminClient.CreateCommand(newCmd)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	token, resp := th.SystemAdminClient.RegenCommandToken(createdCmd.ID)
	CheckNoError(t, resp)
	require.NotEqual(t, createdCmd.Token, token, "should update the token")

	token, resp = Client.RegenCommandToken(createdCmd.ID)
	CheckNotFoundStatus(t, resp)
	require.Empty(t, token, "should not return the token")
}

func TestExecuteInvalidCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	allowedInternalConnections := *th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.AllowedUntrustedInternalConnections = &allowedInternalConnections
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.0/8" })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := &model.CommandResponse{}

		w.Write([]byte(rc.ToJSON()))
	}))
	defer ts.Close()

	getCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       ts.URL,
		Method:    model.CommandMethodGet,
		Trigger:   "getcommand",
	}

	_, err := th.App.CreateCommand(getCmd)
	require.Nil(t, err, "failed to create get command")

	_, resp := Client.ExecuteCommand(channel.ID, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.ExecuteCommand(channel.ID, "/")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.ExecuteCommand(channel.ID, "getcommand")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.ExecuteCommand(channel.ID, "/junk")
	CheckNotFoundStatus(t, resp)

	otherUser := th.CreateUser()
	Client.Login(otherUser.Email, otherUser.Password)

	_, resp = Client.ExecuteCommand(channel.ID, "/getcommand")
	CheckForbiddenStatus(t, resp)

	Client.Logout()

	_, resp = Client.ExecuteCommand(channel.ID, "/getcommand")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.ExecuteCommand(channel.ID, "/getcommand")
	CheckNoError(t, resp)
}

func TestExecuteGetCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	allowedInternalConnections := *th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.AllowedUntrustedInternalConnections = &allowedInternalConnections
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.0/8" })

	token := model.NewID()
	expectedCommandResponse := &model.CommandResponse{
		Text:         "test get command response",
		ResponseType: model.CommandResponseTypeInChannel,
		Type:         "custom_test",
		Props:        map[string]interface{}{"someprop": "somevalue"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)

		values, err := url.ParseQuery(r.URL.RawQuery)
		require.NoError(t, err)

		require.Equal(t, token, values.Get("token"))
		require.Equal(t, th.BasicTeam.Name, values.Get("team_domain"))
		require.Equal(t, "ourCommand", values.Get("cmd"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedCommandResponse.ToJSON()))
	}))
	defer ts.Close()

	getCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       ts.URL + "/?cmd=ourCommand",
		Method:    model.CommandMethodGet,
		Trigger:   "getcommand",
		Token:     token,
	}

	_, err := th.App.CreateCommand(getCmd)
	require.Nil(t, err, "failed to create get command")

	commandResponse, resp := Client.ExecuteCommand(channel.ID, "/getcommand")
	CheckNoError(t, resp)
	assert.True(t, len(commandResponse.TriggerID) == 26)

	expectedCommandResponse.TriggerID = commandResponse.TriggerID
	require.Equal(t, expectedCommandResponse, commandResponse)
}

func TestExecutePostCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	allowedInternalConnections := *th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.AllowedUntrustedInternalConnections = &allowedInternalConnections
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.0/8" })

	token := model.NewID()
	expectedCommandResponse := &model.CommandResponse{
		Text:         "test post command response",
		ResponseType: model.CommandResponseTypeInChannel,
		Type:         "custom_test",
		Props:        map[string]interface{}{"someprop": "somevalue"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)

		r.ParseForm()

		require.Equal(t, token, r.FormValue("token"))
		require.Equal(t, th.BasicTeam.Name, r.FormValue("team_domain"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedCommandResponse.ToJSON()))
	}))
	defer ts.Close()

	postCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       ts.URL,
		Method:    model.CommandMethodPost,
		Trigger:   "postcommand",
		Token:     token,
	}

	_, err := th.App.CreateCommand(postCmd)
	require.Nil(t, err, "failed to create get command")

	commandResponse, resp := Client.ExecuteCommand(channel.ID, "/postcommand")
	CheckNoError(t, resp)
	assert.True(t, len(commandResponse.TriggerID) == 26)

	expectedCommandResponse.TriggerID = commandResponse.TriggerID
	require.Equal(t, expectedCommandResponse, commandResponse)
}

func TestExecuteCommandAgainstChannelOnAnotherTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	allowedInternalConnections := *th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.AllowedUntrustedInternalConnections = &allowedInternalConnections
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	expectedCommandResponse := &model.CommandResponse{
		Text:         "test post command response",
		ResponseType: model.CommandResponseTypeInChannel,
		Type:         "custom_test",
		Props:        map[string]interface{}{"someprop": "somevalue"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedCommandResponse.ToJSON()))
	}))
	defer ts.Close()

	// create a slash command on some other team where we have permission to do so
	team2 := th.CreateTeam()
	postCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    team2.ID,
		URL:       ts.URL,
		Method:    model.CommandMethodPost,
		Trigger:   "postcommand",
	}
	_, err := th.App.CreateCommand(postCmd)
	require.Nil(t, err, "failed to create post command")

	// the execute command endpoint will always search for the command by trigger and team id, inferring team id from the
	// channel id, so there is no way to use that slash command on a channel that belongs to some other team
	_, resp := Client.ExecuteCommand(channel.ID, "/postcommand")
	CheckNotFoundStatus(t, resp)
}

func TestExecuteCommandAgainstChannelUserIsNotIn(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	client := th.Client

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	allowedInternalConnections := *th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.AllowedUntrustedInternalConnections = &allowedInternalConnections
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	expectedCommandResponse := &model.CommandResponse{
		Text:         "test post command response",
		ResponseType: model.CommandResponseTypeInChannel,
		Type:         "custom_test",
		Props:        map[string]interface{}{"someprop": "somevalue"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedCommandResponse.ToJSON()))
	}))
	defer ts.Close()

	// create a slash command on some other team where we have permission to do so
	team2 := th.CreateTeam()
	postCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    team2.ID,
		URL:       ts.URL,
		Method:    model.CommandMethodPost,
		Trigger:   "postcommand",
	}
	_, err := th.App.CreateCommand(postCmd)
	require.Nil(t, err, "failed to create post command")

	// make a channel on that team, ensuring that our test user isn't in it
	channel2 := th.CreateChannelWithClientAndTeam(client, model.ChannelTypeOpen, team2.ID)
	success, _ := client.RemoveUserFromChannel(channel2.ID, th.BasicUser.ID)
	require.True(t, success, "Failed to remove user from channel")

	// we should not be able to run the slash command in channel2, because we aren't in it
	_, resp := client.ExecuteCommandWithTeam(channel2.ID, team2.ID, "/postcommand")
	CheckForbiddenStatus(t, resp)
}

func TestExecuteCommandInDirectMessageChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	client := th.Client

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	allowedInternalConnections := *th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.AllowedUntrustedInternalConnections = &allowedInternalConnections
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	// create a team that the user isn't a part of
	team2 := th.CreateTeam()

	expectedCommandResponse := &model.CommandResponse{
		Text:         "test post command response",
		ResponseType: model.CommandResponseTypeInChannel,
		Type:         "custom_test",
		Props:        map[string]interface{}{"someprop": "somevalue"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedCommandResponse.ToJSON()))
	}))
	defer ts.Close()

	// create a slash command on some other team where we have permission to do so
	postCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    team2.ID,
		URL:       ts.URL,
		Method:    model.CommandMethodPost,
		Trigger:   "postcommand",
	}
	_, err := th.App.CreateCommand(postCmd)
	require.Nil(t, err, "failed to create post command")

	// make a direct message channel
	dmChannel, response := client.CreateDirectChannel(th.BasicUser.ID, th.BasicUser2.ID)
	CheckCreatedStatus(t, response)

	// we should be able to run the slash command in the DM channel
	_, resp := client.ExecuteCommandWithTeam(dmChannel.ID, team2.ID, "/postcommand")
	CheckOKStatus(t, resp)

	// but we can't run the slash command in the DM channel if we sub in some other team's id
	_, resp = client.ExecuteCommandWithTeam(dmChannel.ID, th.BasicTeam.ID, "/postcommand")
	CheckNotFoundStatus(t, resp)
}

func TestExecuteCommandInTeamUserIsNotOn(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	client := th.Client

	enableCommands := *th.App.Config().ServiceSettings.EnableCommands
	allowedInternalConnections := *th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableCommands = &enableCommands })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.ServiceSettings.AllowedUntrustedInternalConnections = &allowedInternalConnections
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCommands = true })
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	// create a team that the user isn't a part of
	team2 := th.CreateTeam()

	expectedCommandResponse := &model.CommandResponse{
		Text:         "test post command response",
		ResponseType: model.CommandResponseTypeInChannel,
		Type:         "custom_test",
		Props:        map[string]interface{}{"someprop": "somevalue"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		r.ParseForm()
		require.Equal(t, team2.Name, r.FormValue("team_domain"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedCommandResponse.ToJSON()))
	}))
	defer ts.Close()

	// create a slash command on that team
	postCmd := &model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    team2.ID,
		URL:       ts.URL,
		Method:    model.CommandMethodPost,
		Trigger:   "postcommand",
	}
	_, err := th.App.CreateCommand(postCmd)
	require.Nil(t, err, "failed to create post command")

	// make a direct message channel
	dmChannel, response := client.CreateDirectChannel(th.BasicUser.ID, th.BasicUser2.ID)
	CheckCreatedStatus(t, response)

	// we should be able to run the slash command in the DM channel
	_, resp := client.ExecuteCommandWithTeam(dmChannel.ID, team2.ID, "/postcommand")
	CheckOKStatus(t, resp)

	// if the user is removed from the team, they should NOT be able to run the slash command in the DM channel
	success, _ := client.RemoveTeamMember(team2.ID, th.BasicUser.ID)
	require.True(t, success, "Failed to remove user from team")

	_, resp = client.ExecuteCommandWithTeam(dmChannel.ID, team2.ID, "/postcommand")
	CheckForbiddenStatus(t, resp)

	// if we omit the team id from the request, the slash command will fail because this is a DM channel, and the
	// team id can't be inherited from the channel
	_, resp = client.ExecuteCommand(dmChannel.ID, "/postcommand")
	CheckForbiddenStatus(t, resp)
}
