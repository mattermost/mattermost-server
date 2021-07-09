// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/api4"
	"github.com/mattermost/mattermost-server/v5/model"
)

func TestListWebhooks(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	adminClient := th.SystemAdminClient

	config := th.Config()
	*config.ServiceSettings.EnableCommands = true
	*config.ServiceSettings.EnableIncomingWebhooks = true
	*config.ServiceSettings.EnableOutgoingWebhooks = true
	*config.ServiceSettings.EnablePostUsernameOverride = true
	*config.ServiceSettings.EnablePostIconOverride = true
	th.SetConfig(config)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostUsernameOverride = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostIconOverride = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageIncomingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageIncomingWebhooks.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamUserRoleID)

	dispName := "myhookinc"
	hook := &model.IncomingWebhook{DisplayName: dispName, ChannelID: th.BasicChannel.ID, TeamID: th.BasicChannel.TeamID}
	_, resp := adminClient.CreateIncomingWebhook(hook)
	api4.CheckNoError(t, resp)

	dispName2 := "myhookout"
	outHook := &model.OutgoingWebhook{DisplayName: dispName2, ChannelID: th.BasicChannel.ID, TeamID: th.BasicChannel.TeamID, CallbackURLs: []string{"http://nowhere.com"}, Username: "some-user-name", IconURL: "http://some-icon-url/"}
	_, resp = adminClient.CreateOutgoingWebhook(outHook)
	api4.CheckNoError(t, resp)

	output := th.CheckCommand(t, "webhook", "list", th.BasicTeam.Name)

	assert.Contains(t, output, dispName, "should have incoming webhooks")
	assert.Contains(t, output, dispName2, "should have outgoing webhooks")
}

func TestShowWebhook(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	adminClient := th.SystemAdminClient

	config := th.Config()
	*config.ServiceSettings.EnableCommands = true
	*config.ServiceSettings.EnableIncomingWebhooks = true
	*config.ServiceSettings.EnableOutgoingWebhooks = true
	*config.ServiceSettings.EnablePostUsernameOverride = true
	*config.ServiceSettings.EnablePostIconOverride = true
	th.SetConfig(config)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostUsernameOverride = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostIconOverride = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageIncomingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageIncomingWebhooks.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamUserRoleID)

	dispName := "incominghook"
	hook := &model.IncomingWebhook{
		DisplayName: dispName,
		ChannelID:   th.BasicChannel.ID,
		TeamID:      th.BasicChannel.TeamID,
	}
	incomingWebhook, resp := adminClient.CreateIncomingWebhook(hook)
	api4.CheckNoError(t, resp)

	// should return an error when no webhookid is provided
	require.Error(t, th.RunCommand(t, "webhook", "show"))

	// invalid webhook should return error
	require.Error(t, th.RunCommand(t, "webhook", "show", "invalid-webhook"))

	// valid incoming webhook should return webhook data
	output := th.CheckCommand(t, "webhook", "show", incomingWebhook.ID)
	assert.Contains(t, output, "DisplayName: \""+dispName+"\"", "incoming: should have incominghook as displayname")
	assert.Contains(t, output, "ChannelId: \""+hook.ChannelID+"\"", "incoming: should have a valid channelId")

	dispName = "outgoinghook"
	outgoingHook := &model.OutgoingWebhook{
		DisplayName:  dispName,
		ChannelID:    th.BasicChannel.ID,
		TeamID:       th.BasicChannel.TeamID,
		CallbackURLs: []string{"http://nowhere.com"},
		Username:     "some-user-name",
		IconURL:      "http://some-icon-url/",
	}
	outgoingWebhook, resp := adminClient.CreateOutgoingWebhook(outgoingHook)
	api4.CheckNoError(t, resp)

	// valid outgoing webhook should return webhook data
	output = th.CheckCommand(t, "webhook", "show", outgoingWebhook.ID)

	assert.Contains(t, output, "DisplayName: \""+dispName+"\"", "outgoing: should have outgoinghook as displayname")
	assert.Contains(t, output, "ChannelId: \""+hook.ChannelID+"\"", "outgoing: should have a valid channelId")
}

func TestCreateIncomingWebhook(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	config := th.Config()
	*config.ServiceSettings.EnableCommands = true
	*config.ServiceSettings.EnableIncomingWebhooks = true
	*config.ServiceSettings.EnableOutgoingWebhooks = true
	*config.ServiceSettings.EnablePostUsernameOverride = true
	*config.ServiceSettings.EnablePostIconOverride = true
	th.SetConfig(config)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostUsernameOverride = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostIconOverride = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageIncomingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageIncomingWebhooks.ID, model.TeamUserRoleID)

	// should fail because you need to specify valid channel
	require.Error(t, th.RunCommand(t, "webhook", "create-incoming"))
	require.Error(t, th.RunCommand(t, "webhook", "create-incoming", "--channel", th.BasicTeam.Name+":doesnotexist"))

	// should fail because you need to specify valid user
	require.Error(t, th.RunCommand(t, "webhook", "create-incoming", "--channel", th.BasicChannel.ID))
	require.Error(t, th.RunCommand(t, "webhook", "create-incoming", "--channel", th.BasicChannel.ID, "--user", "doesnotexist"))

	description := "myhookinc"
	displayName := "myhookinc"
	th.CheckCommand(t, "webhook", "create-incoming", "--channel", th.BasicChannel.ID, "--user", th.BasicUser.Email, "--description", description, "--display-name", displayName)

	webhooks, err := th.App.GetIncomingWebhooksPage(0, 1000)
	require.Nil(t, err, "unable to retrieve incoming webhooks")

	found := false
	for _, webhook := range webhooks {
		if webhook.Description == description && webhook.UserID == th.BasicUser.ID {
			found = true
		}
	}
	require.True(t, found, "Failed to create incoming webhook")
}

func TestModifyIncomingWebhook(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	config := th.Config()
	*config.ServiceSettings.EnableCommands = true
	*config.ServiceSettings.EnableIncomingWebhooks = true
	*config.ServiceSettings.EnableOutgoingWebhooks = true
	*config.ServiceSettings.EnablePostUsernameOverride = true
	*config.ServiceSettings.EnablePostIconOverride = true
	th.SetConfig(config)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostUsernameOverride = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostIconOverride = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageIncomingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageIncomingWebhooks.ID, model.TeamUserRoleID)

	description := "myhookincdesc"
	displayName := "myhookincname"

	incomingWebhook := &model.IncomingWebhook{
		ChannelID:   th.BasicChannel.ID,
		DisplayName: displayName,
		Description: description,
	}

	oldHook, err := th.App.CreateIncomingWebhookForChannel(th.BasicUser.ID, th.BasicChannel, incomingWebhook)
	require.Nil(t, err, "unable to create incoming webhooks")

	defer func() {
		th.App.DeleteIncomingWebhook(oldHook.ID)
	}()

	// should fail because you need to specify valid incoming webhook
	require.Error(t, th.RunCommand(t, "webhook", "modify-incoming", "doesnotexist"))
	// should fail because you need to specify valid channel
	require.Error(t, th.RunCommand(t, "webhook", "modify-incoming", oldHook.ID, "--channel", th.BasicTeam.Name+":doesnotexist"))

	modifiedDescription := "myhookincdesc2"
	modifiedDisplayName := "myhookincname2"
	modifiedIconUrl := "myhookincicon2"
	modifiedChannelLocked := true
	modifiedChannelID := th.BasicChannel2.ID

	th.CheckCommand(t, "webhook", "modify-incoming", oldHook.ID, "--channel", modifiedChannelID, "--description", modifiedDescription, "--display-name", modifiedDisplayName, "--icon", modifiedIconUrl, "--lock-to-channel", strconv.FormatBool(modifiedChannelLocked))

	modifiedHook, err := th.App.GetIncomingWebhook(oldHook.ID)
	require.Nil(t, err, "unable to retrieve modified incoming webhook")

	successUpdate := modifiedHook.DisplayName != modifiedDisplayName ||
		modifiedHook.Description != modifiedDescription ||
		modifiedHook.IconURL != modifiedIconUrl ||
		modifiedHook.ChannelLocked != modifiedChannelLocked ||
		modifiedHook.ChannelID != modifiedChannelID
	require.False(t, successUpdate, "Failed to update incoming webhook")
}

func TestCreateOutgoingWebhook(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	config := th.Config()
	*config.ServiceSettings.EnableCommands = true
	*config.ServiceSettings.EnableIncomingWebhooks = true
	*config.ServiceSettings.EnableOutgoingWebhooks = true
	*config.ServiceSettings.EnablePostUsernameOverride = true
	*config.ServiceSettings.EnablePostIconOverride = true
	th.SetConfig(config)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostUsernameOverride = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostIconOverride = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamUserRoleID)

	// team, user, display name, trigger words, callback urls are required
	team := th.BasicTeam.ID
	user := th.BasicUser.ID
	displayName := "totally radical webhook"
	triggerWord1 := "build"
	triggerWord2 := "defenestrate"
	callbackURL1 := "http://localhost:8000/my-webhook-handler"
	callbackURL2 := "http://localhost:8000/my-webhook-handler2"

	// should fail because team is not specified
	require.Error(t, th.RunCommand(t, "webhook", "create-outgoing", "--display-name", displayName, "--trigger-word", triggerWord1, "--trigger-word", triggerWord2, "--url", callbackURL1, "--url", callbackURL2, "--user", user))

	// should fail because user is not specified
	require.Error(t, th.RunCommand(t, "webhook", "create-outgoing", "--team", team, "--display-name", displayName, "--trigger-word", triggerWord1, "--trigger-word", triggerWord2, "--url", callbackURL1, "--url", callbackURL2))

	// should fail because display name is not specified
	require.Error(t, th.RunCommand(t, "webhook", "create-outgoing", "--team", team, "--trigger-word", triggerWord1, "--trigger-word", triggerWord2, "--url", callbackURL1, "--url", callbackURL2, "--user", user))

	// should fail because trigger words are not specified
	require.Error(t, th.RunCommand(t, "webhook", "create-outgoing", "--team", team, "--display-name", displayName, "--url", callbackURL1, "--url", callbackURL2, "--user", user))

	// should fail because callback URLs are not specified
	require.Error(t, th.RunCommand(t, "webhook", "create-outgoing", "--team", team, "--display-name", displayName, "--trigger-word", triggerWord1, "--trigger-word", triggerWord2, "--user", user))

	// should fail because outgoing webhooks cannot be made for private channels
	require.Error(t, th.RunCommand(t, "webhook", "create-outgoing", "--team", team, "--channel", th.BasicPrivateChannel.ID, "--display-name", displayName, "--trigger-word", triggerWord1, "--trigger-word", triggerWord2, "--url", callbackURL1, "--url", callbackURL2, "--user", user))

	th.CheckCommand(t, "webhook", "create-outgoing", "--team", team, "--channel", th.BasicChannel.ID, "--display-name", displayName, "--trigger-word", triggerWord1, "--trigger-word", triggerWord2, "--url", callbackURL1, "--url", callbackURL2, "--user", user)

	webhooks, err := th.App.GetOutgoingWebhooksPage(0, 1000)
	require.Nil(t, err, "Unable to retrieve outgoing webhooks")

	found := false
	for _, webhook := range webhooks {
		if webhook.DisplayName == displayName && webhook.CreatorID == th.BasicUser.ID {
			found = true
		}
	}
	require.True(t, found, "Failed to create incoming webhook")
}

func TestModifyOutgoingWebhook(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	config := th.Config()
	*config.ServiceSettings.EnableOutgoingWebhooks = true
	th.SetConfig(config)

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamUserRoleID)

	description := "myhookoutdesc"
	displayName := "myhookoutname"
	triggerWords := model.StringArray{"myhookoutword1"}
	triggerWhen := 0
	callbackURLs := model.StringArray{"http://myhookouturl1"}
	iconURL := "myhookicon1"
	contentType := "myhookcontent1"

	outgoingWebhook := &model.OutgoingWebhook{
		CreatorID:    th.BasicUser.ID,
		Username:     th.BasicUser.Username,
		TeamID:       th.BasicTeam.ID,
		ChannelID:    th.BasicChannel.ID,
		DisplayName:  displayName,
		Description:  description,
		TriggerWords: triggerWords,
		TriggerWhen:  triggerWhen,
		CallbackURLs: callbackURLs,
		IconURL:      iconURL,
		ContentType:  contentType,
	}

	oldHook, err := th.App.CreateOutgoingWebhook(outgoingWebhook)
	require.Nil(t, err, "unable to create outgoing webhooks: ")

	defer func() {
		th.App.DeleteOutgoingWebhook(oldHook.ID)
	}()

	// should fail because you need to specify valid outgoing webhook
	require.Error(t, th.RunCommand(t, "webhook", "modify-outgoing", "doesnotexist"))
	// should fail because you need to specify valid channel
	require.Error(t, th.RunCommand(t, "webhook", "modify-outgoing", oldHook.ID, "--channel", th.BasicTeam.Name+":doesnotexist"))
	// should fail because you need to specify valid trigger when
	require.Error(t, th.RunCommand(t, "webhook", "modify-outgoing", oldHook.ID, "--channel", th.BasicTeam.Name+th.BasicChannel.ID, "--trigger-when", "invalid"))
	// should fail because you need to specify a valid callback URL
	require.Error(t, th.RunCommand(t, "webhook", "modify-outgoing", oldHook.ID, "--channel", th.BasicTeam.Name+th.BasicChannel.ID, "--callback-url", "invalid"))

	modifiedChannelID := th.BasicChannel2.ID
	modifiedDisplayName := "myhookoutname2"
	modifiedDescription := "myhookoutdesc2"
	modifiedTriggerWords := model.StringArray{"myhookoutword2A", "myhookoutword2B"}
	modifiedTriggerWhen := "start"
	modifiedIconURL := "myhookouticon2"
	modifiedContentType := "myhookcontent2"
	modifiedCallbackURLs := model.StringArray{"http://myhookouturl2A", "http://myhookouturl2B"}

	th.CheckCommand(t, "webhook", "modify-outgoing", oldHook.ID,
		"--channel", modifiedChannelID,
		"--display-name", modifiedDisplayName,
		"--description", modifiedDescription,
		"--trigger-word", modifiedTriggerWords[0],
		"--trigger-word", modifiedTriggerWords[1],
		"--trigger-when", modifiedTriggerWhen,
		"--icon", modifiedIconURL,
		"--content-type", modifiedContentType,
		"--url", modifiedCallbackURLs[0],
		"--url", modifiedCallbackURLs[1],
	)

	modifiedHook, err := th.App.GetOutgoingWebhook(oldHook.ID)
	require.Nil(t, err, "unable to retrieve modified outgoing webhook")

	updateFailed := modifiedHook.ChannelID != modifiedChannelID ||
		modifiedHook.DisplayName != modifiedDisplayName ||
		modifiedHook.Description != modifiedDescription ||
		len(modifiedHook.TriggerWords) != len(modifiedTriggerWords) ||
		modifiedHook.TriggerWords[0] != modifiedTriggerWords[0] ||
		modifiedHook.TriggerWords[1] != modifiedTriggerWords[1] ||
		modifiedHook.TriggerWhen != 1 ||
		modifiedHook.IconURL != modifiedIconURL ||
		modifiedHook.ContentType != modifiedContentType ||
		len(modifiedHook.CallbackURLs) != len(modifiedCallbackURLs) ||
		modifiedHook.CallbackURLs[0] != modifiedCallbackURLs[0] ||
		modifiedHook.CallbackURLs[1] != modifiedCallbackURLs[1]

	require.False(t, updateFailed, "Failed to update outgoing webhook")
}

func TestDeleteWebhooks(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	adminClient := th.SystemAdminClient

	config := th.Config()
	*config.ServiceSettings.EnableCommands = true
	*config.ServiceSettings.EnableIncomingWebhooks = true
	*config.ServiceSettings.EnableOutgoingWebhooks = true
	*config.ServiceSettings.EnablePostUsernameOverride = true
	*config.ServiceSettings.EnablePostIconOverride = true
	th.SetConfig(config)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOutgoingWebhooks = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostUsernameOverride = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnablePostIconOverride = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageIncomingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageIncomingWebhooks.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamUserRoleID)

	dispName := "myhookinc"
	inHookStruct := &model.IncomingWebhook{DisplayName: dispName, ChannelID: th.BasicChannel.ID, TeamID: th.BasicChannel.TeamID}
	incomingHook, resp := adminClient.CreateIncomingWebhook(inHookStruct)
	api4.CheckNoError(t, resp)

	dispName2 := "myhookout"
	outHookStruct := &model.OutgoingWebhook{DisplayName: dispName2, ChannelID: th.BasicChannel.ID, TeamID: th.BasicChannel.TeamID, CallbackURLs: []string{"http://nowhere.com"}, Username: "some-user-name", IconURL: "http://some-icon-url/"}
	outgoingHook, resp := adminClient.CreateOutgoingWebhook(outHookStruct)
	api4.CheckNoError(t, resp)

	hooksBeforeDeletion := th.CheckCommand(t, "webhook", "list", th.BasicTeam.Name)

	assert.Contains(t, hooksBeforeDeletion, dispName, "should have incoming webhooks")
	assert.Contains(t, hooksBeforeDeletion, dispName2, "Should have outgoing webhooks")

	th.CheckCommand(t, "webhook", "delete", incomingHook.ID)
	th.CheckCommand(t, "webhook", "delete", outgoingHook.ID)

	hooksAfterDeletion := th.CheckCommand(t, "webhook", "list", th.BasicTeam.Name)

	assert.NotContains(t, hooksAfterDeletion, dispName, "Should not have incoming webhooks")
	assert.NotContains(t, hooksAfterDeletion, dispName2, "Should not have outgoing webhooks")
}

func TestMoveOutgoingWebhook(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	config := th.Config()
	*config.ServiceSettings.EnableOutgoingWebhooks = true
	th.SetConfig(config)

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer th.RestoreDefaultRolePermissions(defaultRolePermissions)

	th.AddPermissionToRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManageOutgoingWebhooks.ID, model.TeamUserRoleID)

	description := "myhookoutdesc"
	displayName := "myhookoutname"
	triggerWords := model.StringArray{"myhookoutword1"}
	triggerWhen := 0
	callbackURLs := model.StringArray{"http://myhookouturl1"}
	iconURL := "myhookicon1"
	contentType := "myhookcontent1"

	outgoingWebhookWithChannel := &model.OutgoingWebhook{
		CreatorID:    th.BasicUser.ID,
		Username:     th.BasicUser.Username,
		TeamID:       th.BasicTeam.ID,
		ChannelID:    th.BasicChannel.ID,
		DisplayName:  displayName,
		Description:  description,
		TriggerWords: triggerWords,
		TriggerWhen:  triggerWhen,
		CallbackURLs: callbackURLs,
		IconURL:      iconURL,
		ContentType:  contentType,
	}

	oldHook, err := th.App.CreateOutgoingWebhook(outgoingWebhookWithChannel)
	require.Nil(t, err)
	defer th.App.DeleteOutgoingWebhook(oldHook.ID)

	require.Error(t, th.RunCommand(t, "webhook", "move-outgoing"))
	require.Error(t, th.RunCommand(t, "webhook", "move-outgoing", th.BasicTeam.ID))
	require.Error(t, th.RunCommand(t, "webhook", "move-outgoing", "invalid-team", "webhook"))
	require.Error(t, th.RunCommand(t, "webhook", "move-outgoing", "invalid-team", "webhook", "--channel"))

	newTeam := th.CreateTeam()

	webhookInformation := "oldTeam" + ":" + "webhookId"
	require.Error(t, th.RunCommand(t, "webhook", "move-outgoing", newTeam.ID, webhookInformation))

	webhookInformation = th.BasicTeam.ID + ":" + "webhookId"
	require.Error(t, th.RunCommand(t, "webhook", "move-outgoing", newTeam.ID, webhookInformation))

	require.Error(t, th.RunCommand(t, "webhook", "move-outgoing", newTeam.ID, th.BasicTeam.ID+":"+oldHook.ID, "--channel", "invalid"))

	channel := th.CreateChannelWithClientAndTeam(th.SystemAdminClient, model.ChannelTypeOpen, newTeam.ID)
	th.CheckCommand(t, "webhook", "move-outgoing", newTeam.ID, th.BasicTeam.ID+":"+oldHook.ID, "--channel", channel.Name)

	_, webhookErr := th.App.GetOutgoingWebhook(oldHook.ID)
	assert.NotNil(t, webhookErr)

	output := th.CheckCommand(t, "webhook", "list", newTeam.Name)
	assert.True(t, strings.Contains(output, displayName))

	outgoingWebhookWithoutChannel := &model.OutgoingWebhook{
		CreatorID:    th.BasicUser.ID,
		Username:     th.BasicUser.Username,
		TeamID:       th.BasicTeam.ID,
		DisplayName:  displayName + "2",
		Description:  description,
		TriggerWords: triggerWords,
		TriggerWhen:  triggerWhen,
		CallbackURLs: callbackURLs,
		IconURL:      iconURL,
		ContentType:  contentType,
	}

	oldHook2, err := th.App.CreateOutgoingWebhook(outgoingWebhookWithoutChannel)
	require.Nil(t, err)
	defer th.App.DeleteOutgoingWebhook(oldHook2.ID)

	th.CheckCommand(t, "webhook", "move-outgoing", newTeam.ID, th.BasicTeam.ID+":"+oldHook2.ID)
	output = th.CheckCommand(t, "webhook", "list", newTeam.Name)
	assert.True(t, strings.Contains(output, displayName+"2"))
}
