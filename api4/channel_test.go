// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func TestCreateChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_OPEN, TeamId: team.Id}
	private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_PRIVATE, TeamId: team.Id}

	rchannel, resp := Client.CreateChannel(channel)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	require.Equal(t, channel.Name, rchannel.Name, "names did not match")
	require.Equal(t, channel.DisplayName, rchannel.DisplayName, "display names did not match")
	require.Equal(t, channel.TeamId, rchannel.TeamId, "team ids did not match")

	rprivate, resp := Client.CreateChannel(private)
	CheckNoError(t, resp)

	require.Equal(t, private.Name, rprivate.Name, "names did not match")
	require.Equal(t, model.CHANNEL_PRIVATE, rprivate.Type, "wrong channel type")
	require.Equal(t, th.BasicUser.Id, rprivate.CreatorId, "wrong creator id")

	_, resp = Client.CreateChannel(channel)
	CheckErrorMessage(t, resp, "store.sql_channel.save_channel.exists.app_error")
	CheckBadRequestStatus(t, resp)

	direct := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_DIRECT, TeamId: team.Id}
	_, resp = Client.CreateChannel(direct)
	CheckErrorMessage(t, resp, "api.channel.create_channel.direct_channel.app_error")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.CreateChannel(channel)
	CheckUnauthorizedStatus(t, resp)

	userNotOnTeam := th.CreateUser()
	Client.Login(userNotOnTeam.Email, userNotOnTeam.Password)

	_, resp = Client.CreateChannel(channel)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.CreateChannel(private)
	CheckForbiddenStatus(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PERMISSION_CREATE_PUBLIC_CHANNEL.Id, model.TEAM_USER_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_CREATE_PRIVATE_CHANNEL.Id, model.TEAM_USER_ROLE_ID)

	th.LoginBasic()

	channel.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(channel)
	CheckNoError(t, resp)

	private.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(private)
	CheckNoError(t, resp)

	th.AddPermissionToRole(model.PERMISSION_CREATE_PUBLIC_CHANNEL.Id, model.TEAM_ADMIN_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_CREATE_PRIVATE_CHANNEL.Id, model.TEAM_ADMIN_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_CREATE_PUBLIC_CHANNEL.Id, model.TEAM_USER_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_CREATE_PRIVATE_CHANNEL.Id, model.TEAM_USER_ROLE_ID)

	_, resp = Client.CreateChannel(channel)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.CreateChannel(private)
	CheckForbiddenStatus(t, resp)

	th.LoginTeamAdmin()

	channel.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(channel)
	CheckNoError(t, resp)

	private.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(private)
	CheckNoError(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		channel.Name = GenerateTestChannelName()
		_, resp = client.CreateChannel(channel)
		CheckNoError(t, resp)

		private.Name = GenerateTestChannelName()
		_, resp = client.CreateChannel(private)
		CheckNoError(t, resp)
	})

	// Test posting Garbage
	r, err := Client.DoApiPost("/channels", "garbage")
	require.NotNil(t, err, "expected error")
	require.Equal(t, http.StatusBadRequest, r.StatusCode, "Expected 400 Bad Request")

	// Test GroupConstrained flag
	groupConstrainedChannel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_OPEN, TeamId: team.Id, GroupConstrained: model.NewBool(true)}
	rchannel, resp = Client.CreateChannel(groupConstrainedChannel)
	CheckNoError(t, resp)

	require.Equal(t, *groupConstrainedChannel.GroupConstrained, *rchannel.GroupConstrained, "GroupConstrained flags do not match")
}

func TestUpdateChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_OPEN, TeamId: team.Id}
	private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_PRIVATE, TeamId: team.Id}

	channel, _ = Client.CreateChannel(channel)
	private, _ = Client.CreateChannel(private)

	//Update a open channel
	channel.DisplayName = "My new display name"
	channel.Header = "My fancy header"
	channel.Purpose = "Mattermost ftw!"

	newChannel, resp := Client.UpdateChannel(channel)
	CheckNoError(t, resp)

	require.Equal(t, channel.DisplayName, newChannel.DisplayName, "Update failed for DisplayName")
	require.Equal(t, channel.Header, newChannel.Header, "Update failed for Header")
	require.Equal(t, channel.Purpose, newChannel.Purpose, "Update failed for Purpose")

	// Test GroupConstrained flag
	channel.GroupConstrained = model.NewBool(true)
	rchannel, resp := Client.UpdateChannel(channel)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	require.Equal(t, *channel.GroupConstrained, *rchannel.GroupConstrained, "GroupConstrained flags do not match")

	//Update a private channel
	private.DisplayName = "My new display name for private channel"
	private.Header = "My fancy private header"
	private.Purpose = "Mattermost ftw! in private mode"

	newPrivateChannel, resp := Client.UpdateChannel(private)
	CheckNoError(t, resp)

	require.Equal(t, private.DisplayName, newPrivateChannel.DisplayName, "Update failed for DisplayName in private channel")
	require.Equal(t, private.Header, newPrivateChannel.Header, "Update failed for Header in private channel")
	require.Equal(t, private.Purpose, newPrivateChannel.Purpose, "Update failed for Purpose in private channel")

	// Test that changing the type fails and returns error

	private.Type = model.CHANNEL_OPEN
	newPrivateChannel, resp = Client.UpdateChannel(private)
	CheckBadRequestStatus(t, resp)

	// Test that keeping the same type succeeds

	private.Type = model.CHANNEL_PRIVATE
	newPrivateChannel, resp = Client.UpdateChannel(private)
	CheckNoError(t, resp)

	//Non existing channel
	channel1 := &model.Channel{DisplayName: "Test API Name for apiv4", Name: GenerateTestChannelName(), Type: model.CHANNEL_OPEN, TeamId: team.Id}
	_, resp = Client.UpdateChannel(channel1)
	CheckNotFoundStatus(t, resp)

	//Try to update with not logged user
	Client.Logout()
	_, resp = Client.UpdateChannel(channel)
	CheckUnauthorizedStatus(t, resp)

	//Try to update using another user
	user := th.CreateUser()
	Client.Login(user.Email, user.Password)

	channel.DisplayName = "Should not update"
	_, resp = Client.UpdateChannel(channel)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	user1 := th.CreateUser()
	user2 := th.CreateUser()
	user3 := th.CreateUser()

	groupChannel, resp := Client.CreateGroupChannel([]string{user1.Id, user2.Id})
	CheckNoError(t, resp)

	groupChannel.Header = "lolololol"
	Client.Logout()
	Client.Login(user3.Email, user3.Password)
	_, resp = Client.UpdateChannel(groupChannel)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	Client.Logout()
	Client.Login(user.Email, user.Password)

	directChannel, resp := Client.CreateDirectChannel(user.Id, user1.Id)
	CheckNoError(t, resp)

	directChannel.Header = "lolololol"
	Client.Logout()
	Client.Login(user3.Email, user3.Password)
	_, resp = Client.UpdateChannel(directChannel)
	CheckForbiddenStatus(t, resp)
}

func TestPatchChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	patch := &model.ChannelPatch{
		Name:        new(string),
		DisplayName: new(string),
		Header:      new(string),
		Purpose:     new(string),
	}
	*patch.Name = model.NewId()
	*patch.DisplayName = model.NewId()
	*patch.Header = model.NewId()
	*patch.Purpose = model.NewId()

	channel, resp := Client.PatchChannel(th.BasicChannel.Id, patch)
	CheckNoError(t, resp)

	require.Equal(t, *patch.Name, channel.Name, "do not match")
	require.Equal(t, *patch.DisplayName, channel.DisplayName, "do not match")
	require.Equal(t, *patch.Header, channel.Header, "do not match")
	require.Equal(t, *patch.Purpose, channel.Purpose, "do not match")

	patch.Name = nil
	oldName := channel.Name
	channel, resp = Client.PatchChannel(th.BasicChannel.Id, patch)
	CheckNoError(t, resp)

	require.Equal(t, oldName, channel.Name, "should not have updated")

	// Test GroupConstrained flag
	patch.GroupConstrained = model.NewBool(true)
	rchannel, resp := Client.PatchChannel(th.BasicChannel.Id, patch)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	require.Equal(t, *rchannel.GroupConstrained, *patch.GroupConstrained, "GroupConstrained flags do not match")
	patch.GroupConstrained = nil

	_, resp = Client.PatchChannel("junk", patch)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.PatchChannel(model.NewId(), patch)
	CheckNotFoundStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.PatchChannel(th.BasicChannel.Id, patch)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.PatchChannel(th.BasicChannel.Id, patch)
		CheckNoError(t, resp)

		_, resp = client.PatchChannel(th.BasicPrivateChannel.Id, patch)
		CheckNoError(t, resp)
	})

	// Test updating the header of someone else's GM channel.
	user1 := th.CreateUser()
	user2 := th.CreateUser()
	user3 := th.CreateUser()

	groupChannel, resp := Client.CreateGroupChannel([]string{user1.Id, user2.Id})
	CheckNoError(t, resp)

	Client.Logout()
	Client.Login(user3.Email, user3.Password)

	channelPatch := &model.ChannelPatch{}
	channelPatch.Header = new(string)
	*channelPatch.Header = "lolololol"

	_, resp = Client.PatchChannel(groupChannel.Id, channelPatch)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	Client.Logout()
	Client.Login(user.Email, user.Password)

	directChannel, resp := Client.CreateDirectChannel(user.Id, user1.Id)
	CheckNoError(t, resp)

	Client.Logout()
	Client.Login(user3.Email, user3.Password)
	_, resp = Client.PatchChannel(directChannel.Id, channelPatch)
	CheckForbiddenStatus(t, resp)
}

func TestChannelUnicodeNames(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	t.Run("create channel unicode", func(t *testing.T) {
		channel := &model.Channel{
			Name:        "\u206cenglish\u206dchannel",
			DisplayName: "The \u206cEnglish\u206d Channel",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id}

		rchannel, resp := Client.CreateChannel(channel)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		require.Equal(t, "englishchannel", rchannel.Name, "bad unicode should be filtered from name")
		require.Equal(t, "The English Channel", rchannel.DisplayName, "bad unicode should be filtered from display name")
	})

	t.Run("update channel unicode", func(t *testing.T) {
		channel := &model.Channel{
			DisplayName: "Test API Name",
			Name:        GenerateTestChannelName(),
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, _ = Client.CreateChannel(channel)

		channel.Name = "\u206ahistorychannel"
		channel.DisplayName = "UFO's and \ufff9stuff\ufffb."

		newChannel, resp := Client.UpdateChannel(channel)
		CheckNoError(t, resp)

		require.Equal(t, "historychannel", newChannel.Name, "bad unicode should be filtered from name")
		require.Equal(t, "UFO's and stuff.", newChannel.DisplayName, "bad unicode should be filtered from display name")
	})

	t.Run("patch channel unicode", func(t *testing.T) {
		patch := &model.ChannelPatch{
			Name:        new(string),
			DisplayName: new(string),
			Header:      new(string),
			Purpose:     new(string),
		}
		*patch.Name = "\u206ecommunitychannel\u206f"
		*patch.DisplayName = "Natalie Tran's \ufffcAwesome Channel"

		channel, resp := Client.PatchChannel(th.BasicChannel.Id, patch)
		CheckNoError(t, resp)

		require.Equal(t, "communitychannel", channel.Name, "bad unicode should be filtered from name")
		require.Equal(t, "Natalie Tran's Awesome Channel", channel.DisplayName, "bad unicode should be filtered from display name")
	})
}

func TestCreateDirectChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()

	dm, resp := Client.CreateDirectChannel(user1.Id, user2.Id)
	CheckNoError(t, resp)

	channelName := ""
	if user2.Id > user1.Id {
		channelName = user1.Id + "__" + user2.Id
	} else {
		channelName = user2.Id + "__" + user1.Id
	}

	require.Equal(t, channelName, dm.Name, "dm name didn't match")

	_, resp = Client.CreateDirectChannel("junk", user2.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(user1.Id, model.NewId())
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(model.NewId(), user1.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(model.NewId(), user2.Id)
	CheckForbiddenStatus(t, resp)

	r, err := Client.DoApiPost("/channels/direct", "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	_, resp = th.SystemAdminClient.CreateDirectChannel(user3.Id, user2.Id)
	CheckNoError(t, resp)

	// Normal client should not be allowed to create a direct channel if users are
	// restricted to messaging members of their own team
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.RestrictDirectMessage = model.DIRECT_MESSAGE_TEAM
	})
	user4 := th.CreateUser()
	_, resp = th.Client.CreateDirectChannel(user1.Id, user4.Id)
	CheckForbiddenStatus(t, resp)
	th.LinkUserToTeam(user4, th.BasicTeam)
	_, resp = th.Client.CreateDirectChannel(user1.Id, user4.Id)
	CheckNoError(t, resp)

	Client.Logout()
	_, resp = Client.CreateDirectChannel(model.NewId(), user2.Id)
	CheckUnauthorizedStatus(t, resp)
}

func TestCreateDirectChannelAsGuest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user1 := th.BasicUser

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		th.App.Srv().RemoveLicense()
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	id := model.NewId()
	guest := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}
	guest, err := th.App.CreateGuest(th.Context, guest)
	require.Nil(t, err)

	_, resp := Client.Login(guest.Username, "Password1")
	CheckNoError(t, resp)

	t.Run("Try to created DM with not visible user", func(t *testing.T) {
		_, resp := Client.CreateDirectChannel(guest.Id, user1.Id)
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateDirectChannel(user1.Id, guest.Id)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Creating DM with visible user", func(t *testing.T) {
		th.LinkUserToTeam(guest, th.BasicTeam)
		th.AddUserToChannel(guest, th.BasicChannel)

		_, resp := Client.CreateDirectChannel(guest.Id, user1.Id)
		CheckNoError(t, resp)
	})
}

func TestDeleteDirectChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2

	rgc, resp := Client.CreateDirectChannel(user.Id, user2.Id)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.NotNil(t, rgc, "should have created a direct channel")

	deleted, resp := Client.DeleteChannel(rgc.Id)
	CheckErrorMessage(t, resp, "api.channel.delete_channel.type.invalid")
	require.False(t, deleted, "should not have been able to delete direct channel.")
}

func TestCreateGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()

	userIds := []string{user.Id, user2.Id, user3.Id}

	rgc, resp := Client.CreateGroupChannel(userIds)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	require.NotNil(t, rgc, "should have created a group channel")
	require.Equal(t, model.CHANNEL_GROUP, rgc.Type, "should have created a channel of group type")

	m, _ := th.App.GetChannelMembersPage(rgc.Id, 0, 10)
	require.Len(t, *m, 3, "should have 3 channel members")

	// saving duplicate group channel
	rgc2, resp := Client.CreateGroupChannel([]string{user3.Id, user2.Id})
	CheckNoError(t, resp)
	require.Equal(t, rgc.Id, rgc2.Id, "should have returned existing channel")

	m2, _ := th.App.GetChannelMembersPage(rgc2.Id, 0, 10)
	require.Equal(t, m, m2)

	_, resp = Client.CreateGroupChannel([]string{user2.Id})
	CheckBadRequestStatus(t, resp)

	user4 := th.CreateUser()
	user5 := th.CreateUser()
	user6 := th.CreateUser()
	user7 := th.CreateUser()
	user8 := th.CreateUser()
	user9 := th.CreateUser()

	rgc, resp = Client.CreateGroupChannel([]string{user.Id, user2.Id, user3.Id, user4.Id, user5.Id, user6.Id, user7.Id, user8.Id, user9.Id})
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rgc)

	_, resp = Client.CreateGroupChannel([]string{user.Id, user2.Id, user3.Id, GenerateTestId()})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateGroupChannel([]string{user.Id, user2.Id, user3.Id, "junk"})
	CheckBadRequestStatus(t, resp)

	Client.Logout()

	_, resp = Client.CreateGroupChannel(userIds)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.CreateGroupChannel(userIds)
	CheckNoError(t, resp)
}

func TestCreateGroupChannelAsGuest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()
	user4 := th.CreateUser()
	user5 := th.CreateUser()
	th.LinkUserToTeam(user2, th.BasicTeam)
	th.AddUserToChannel(user2, th.BasicChannel)
	th.LinkUserToTeam(user3, th.BasicTeam)
	th.AddUserToChannel(user3, th.BasicChannel)

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		th.App.Srv().RemoveLicense()
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	id := model.NewId()
	guest := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}
	guest, err := th.App.CreateGuest(th.Context, guest)
	require.Nil(t, err)

	_, resp := Client.Login(guest.Username, "Password1")
	CheckNoError(t, resp)

	t.Run("Try to created GM with not visible users", func(t *testing.T) {
		_, resp := Client.CreateGroupChannel([]string{guest.Id, user1.Id, user2.Id, user3.Id})
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateGroupChannel([]string{user1.Id, user2.Id, guest.Id, user3.Id})
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Try to created GM with visible and not visible users", func(t *testing.T) {
		th.LinkUserToTeam(guest, th.BasicTeam)
		th.AddUserToChannel(guest, th.BasicChannel)

		_, resp := Client.CreateGroupChannel([]string{guest.Id, user1.Id, user3.Id, user4.Id, user5.Id})
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateGroupChannel([]string{user1.Id, user2.Id, guest.Id, user4.Id, user5.Id})
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Creating GM with visible users", func(t *testing.T) {
		_, resp := Client.CreateGroupChannel([]string{guest.Id, user1.Id, user2.Id, user3.Id})
		CheckNoError(t, resp)
	})
}

func TestDeleteGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()

	userIds := []string{user.Id, user2.Id, user3.Id}

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		rgc, resp := th.Client.CreateGroupChannel(userIds)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, rgc, "should have created a group channel")
		deleted, resp := client.DeleteChannel(rgc.Id)
		CheckErrorMessage(t, resp, "api.channel.delete_channel.type.invalid")
		require.False(t, deleted, "should not have been able to delete group channel.")
	})

}

func TestGetChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channel, resp := Client.GetChannel(th.BasicChannel.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Id, channel.Id, "ids did not match")

	Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannel(th.BasicChannel.Id, "")
	CheckNoError(t, resp)

	channel, resp = Client.GetChannel(th.BasicPrivateChannel.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicPrivateChannel.Id, channel.Id, "ids did not match")

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannel(th.BasicPrivateChannel.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannel(model.NewId(), "")
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannel(th.BasicChannel.Id, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannel(th.BasicChannel.Id, "")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.GetChannel(th.BasicChannel.Id, "")
		CheckNoError(t, resp)

		_, resp = client.GetChannel(th.BasicPrivateChannel.Id, "")
		CheckNoError(t, resp)

		_, resp = client.GetChannel(th.BasicUser.Id, "")
		CheckNotFoundStatus(t, resp)
	})
}

func TestGetDeletedChannelsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	team := th.BasicTeam

	th.LoginTeamAdmin()

	channels, resp := Client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	numInitialChannelsForTeam := len(channels)

	// create and delete public channel
	publicChannel1 := th.CreatePublicChannel()
	Client.DeleteChannel(publicChannel1.Id)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		channels, resp = client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
		CheckNoError(t, resp)
		require.Len(t, channels, numInitialChannelsForTeam+1, "should be 1 deleted channel")
	})

	publicChannel2 := th.CreatePublicChannel()
	Client.DeleteChannel(publicChannel2.Id)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		channels, resp = client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
		CheckNoError(t, resp)
		require.Len(t, channels, numInitialChannelsForTeam+2, "should be 2 deleted channels")
	})

	th.LoginBasic()

	privateChannel1 := th.CreatePrivateChannel()
	Client.DeleteChannel(privateChannel1.Id)

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, numInitialChannelsForTeam+3)

	// Login as different user and create private channel
	th.LoginBasic2()
	privateChannel2 := th.CreatePrivateChannel()
	Client.DeleteChannel(privateChannel2.Id)

	// Log back in as first user
	th.LoginBasic()

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, numInitialChannelsForTeam+3)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		channels, resp = client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
		CheckNoError(t, resp)
		require.Len(t, channels, numInitialChannelsForTeam+2)
	})

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")
}

func TestGetPrivateChannelsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	team := th.BasicTeam

	// normal user
	_, resp := th.Client.GetPrivateChannelsForTeam(team.Id, 0, 100, "")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		channels, resp := c.GetPrivateChannelsForTeam(team.Id, 0, 100, "")
		CheckNoError(t, resp)
		// th.BasicPrivateChannel and th.BasicPrivateChannel2
		require.Len(t, channels, 2, "wrong number of private channels")
		for _, c := range channels {
			// check all channels included are private
			require.Equal(t, model.CHANNEL_PRIVATE, c.Type, "should include private channels only")
		}

		channels, resp = c.GetPrivateChannelsForTeam(team.Id, 0, 1, "")
		CheckNoError(t, resp)
		require.Len(t, channels, 1, "should be one channel per page")

		channels, resp = c.GetPrivateChannelsForTeam(team.Id, 1, 1, "")
		CheckNoError(t, resp)
		require.Len(t, channels, 1, "should be one channel per page")

		channels, resp = c.GetPrivateChannelsForTeam(team.Id, 10000, 100, "")
		CheckNoError(t, resp)
		require.Empty(t, channels, "should be no channel")

		_, resp = c.GetPrivateChannelsForTeam("junk", 0, 100, "")
		CheckBadRequestStatus(t, resp)
	})
}

func TestGetPublicChannelsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam
	publicChannel1 := th.BasicChannel
	publicChannel2 := th.BasicChannel2

	channels, resp := Client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 4, "wrong path")

	for i, c := range channels {
		// check all channels included are open
		require.Equal(t, model.CHANNEL_OPEN, c.Type, "should include open channel only")

		// only check the created 2 public channels
		require.False(t, i < 2 && !(c.DisplayName == publicChannel1.DisplayName || c.DisplayName == publicChannel2.DisplayName), "should match public channel display name")
	}

	privateChannel := th.CreatePrivateChannel()
	channels, resp = Client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 4, "incorrect length of team public channels")

	for _, c := range channels {
		require.Equal(t, model.CHANNEL_OPEN, c.Type, "should not include private channel")
		require.NotEqual(t, privateChannel.DisplayName, c.DisplayName, "should not match private channel display name")
	}

	channels, resp = Client.GetPublicChannelsForTeam(team.Id, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetPublicChannelsForTeam(team.Id, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetPublicChannelsForTeam(team.Id, 10000, 100, "")
	CheckNoError(t, resp)
	require.Empty(t, channels, "should be no channel")

	_, resp = Client.GetPublicChannelsForTeam("junk", 0, 100, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsForTeam(model.NewId(), 0, 100, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
		CheckNoError(t, resp)
	})
}

func TestGetPublicChannelsByIdsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	teamId := th.BasicTeam.Id
	input := []string{th.BasicChannel.Id}
	output := []string{th.BasicChannel.DisplayName}

	channels, resp := Client.GetPublicChannelsByIdsForTeam(teamId, input)
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should return 1 channel")
	require.Equal(t, output[0], channels[0].DisplayName, "missing channel")

	input = append(input, GenerateTestId())
	input = append(input, th.BasicChannel2.Id)
	input = append(input, th.BasicPrivateChannel.Id)
	output = append(output, th.BasicChannel2.DisplayName)
	sort.Strings(output)

	channels, resp = Client.GetPublicChannelsByIdsForTeam(teamId, input)
	CheckNoError(t, resp)
	require.Len(t, channels, 2, "should return 2 channels")

	for i, c := range channels {
		require.Equal(t, output[i], c.DisplayName, "missing channel")
	}

	_, resp = Client.GetPublicChannelsByIdsForTeam(GenerateTestId(), input)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, []string{})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, []string{"junk"})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, []string{GenerateTestId()})
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, []string{th.BasicPrivateChannel.Id})
	CheckNotFoundStatus(t, resp)

	Client.Logout()

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, input)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPublicChannelsByIdsForTeam(teamId, input)
	CheckNoError(t, resp)
}

func TestGetChannelsForTeamForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	t.Run("get channels for the team for user", func(t *testing.T) {
		channels, resp := Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, "")
		CheckNoError(t, resp)

		found := make([]bool, 3)
		for _, c := range channels {
			if c.Id == th.BasicChannel.Id {
				found[0] = true
			} else if c.Id == th.BasicChannel2.Id {
				found[1] = true
			} else if c.Id == th.BasicPrivateChannel.Id {
				found[2] = true
			}

			require.True(t, c.TeamId == "" || c.TeamId == th.BasicTeam.Id)
		}

		for _, f := range found {
			require.True(t, f, "missing a channel")
		}

		channels, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, resp.Etag)
		CheckEtag(t, channels, resp)

		_, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, "junk", false, "")
		CheckBadRequestStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser("junk", th.BasicUser.Id, false, "")
		CheckBadRequestStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser2.Id, false, "")
		CheckForbiddenStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser(model.NewId(), th.BasicUser.Id, false, "")
		CheckForbiddenStatus(t, resp)

		_, resp = th.SystemAdminClient.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, "")
		CheckNoError(t, resp)
	})

	t.Run("deleted channel could be retrieved using the proper flag", func(t *testing.T) {
		testChannel := &model.Channel{
			DisplayName: "dn_" + model.NewId(),
			Name:        GenerateTestChannelName(),
			Type:        model.CHANNEL_OPEN,
			TeamId:      th.BasicTeam.Id,
			CreatorId:   th.BasicUser.Id,
		}
		th.App.CreateChannel(th.Context, testChannel, true)
		defer th.App.PermanentDeleteChannel(testChannel)
		channels, resp := Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, "")
		CheckNoError(t, resp)
		assert.Equal(t, 6, len(channels))
		th.App.DeleteChannel(th.Context, testChannel, th.BasicUser.Id)
		channels, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, "")
		CheckNoError(t, resp)
		assert.Equal(t, 5, len(channels))

		// Should return all channels including basicDeleted.
		channels, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, true, "")
		CheckNoError(t, resp)
		assert.Equal(t, 7, len(channels))

		// Should stil return all channels including basicDeleted.
		now := time.Now().Add(-time.Minute).Unix() * 1000
		Client.GetChannelsForTeamAndUserWithLastDeleteAt(th.BasicTeam.Id, th.BasicUser.Id,
			true, int(now), "")
		CheckNoError(t, resp)
		assert.Equal(t, 7, len(channels))
	})
}

func TestGetAllChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		channels, resp := client.GetAllChannels(0, 20, "")
		CheckNoError(t, resp)

		// At least, all the not-deleted channels created during the InitBasic
		require.True(t, len(*channels) >= 3)
		for _, c := range *channels {
			require.NotEqual(t, c.TeamId, "")
		}

		channels, resp = client.GetAllChannels(0, 10, "")
		CheckNoError(t, resp)
		require.True(t, len(*channels) >= 3)

		channels, resp = client.GetAllChannels(1, 1, "")
		CheckNoError(t, resp)
		require.Len(t, *channels, 1)

		channels, resp = client.GetAllChannels(10000, 10000, "")
		CheckNoError(t, resp)
		require.Empty(t, *channels)

		channels, resp = client.GetAllChannels(0, 10000, "")
		require.Nil(t, resp.Error)
		beforeCount := len(*channels)

		firstChannel := (*channels)[0].Channel

		ok, resp := client.DeleteChannel(firstChannel.Id)
		require.Nil(t, resp.Error)
		require.True(t, ok)

		channels, resp = client.GetAllChannels(0, 10000, "")
		var ids []string
		for _, item := range *channels {
			ids = append(ids, item.Channel.Id)
		}
		require.Nil(t, resp.Error)
		require.Len(t, *channels, beforeCount-1)
		require.NotContains(t, ids, firstChannel.Id)

		channels, resp = client.GetAllChannelsIncludeDeleted(0, 10000, "")
		ids = []string{}
		for _, item := range *channels {
			ids = append(ids, item.Channel.Id)
		}
		require.Nil(t, resp.Error)
		require.True(t, len(*channels) > beforeCount)
		require.Contains(t, ids, firstChannel.Id)
	})

	_, resp := Client.GetAllChannels(0, 20, "")
	CheckForbiddenStatus(t, resp)

	sysManagerChannels, resp := th.SystemManagerClient.GetAllChannels(0, 10000, "")
	CheckOKStatus(t, resp)
	policyChannel := (*sysManagerChannels)[0]
	policy, savePolicyErr := th.App.Srv().Store.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:  "Policy 1",
			PostDuration: model.NewInt64(30),
		},
		ChannelIDs: []string{policyChannel.Id},
	})
	require.NoError(t, savePolicyErr)

	t.Run("exclude policy constrained", func(t *testing.T) {
		_, resp := th.SystemManagerClient.GetAllChannelsExcludePolicyConstrained(0, 10000, "")
		CheckForbiddenStatus(t, resp)

		channels, resp := th.SystemAdminClient.GetAllChannelsExcludePolicyConstrained(0, 10000, "")
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range *channels {
			if channel.Id == policyChannel.Id {
				found = true
				break
			}
		}
		require.False(t, found)
	})

	t.Run("does not return policy ID", func(t *testing.T) {
		channels, resp := th.SystemManagerClient.GetAllChannels(0, 10000, "")
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range *channels {
			if channel.Id == policyChannel.Id {
				found = true
				require.Nil(t, channel.PolicyID)
				break
			}
		}
		require.True(t, found)
	})

	t.Run("returns policy ID", func(t *testing.T) {
		channels, resp := th.SystemAdminClient.GetAllChannels(0, 10000, "")
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range *channels {
			if channel.Id == policyChannel.Id {
				found = true
				require.Equal(t, *channel.PolicyID, policy.ID)
				break
			}
		}
		require.True(t, found)
	})
}

func TestGetAllChannelsWithCount(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channels, total, resp := th.SystemAdminClient.GetAllChannelsWithCount(0, 20, "")
	CheckNoError(t, resp)

	// At least, all the not-deleted channels created during the InitBasic
	require.True(t, len(*channels) >= 3)
	for _, c := range *channels {
		require.NotEqual(t, c.TeamId, "")
	}
	require.Equal(t, int64(6), total)

	channels, _, resp = th.SystemAdminClient.GetAllChannelsWithCount(0, 10, "")
	CheckNoError(t, resp)
	require.True(t, len(*channels) >= 3)

	channels, _, resp = th.SystemAdminClient.GetAllChannelsWithCount(1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, *channels, 1)

	channels, _, resp = th.SystemAdminClient.GetAllChannelsWithCount(10000, 10000, "")
	CheckNoError(t, resp)
	require.Empty(t, *channels)

	_, _, resp = Client.GetAllChannelsWithCount(0, 20, "")
	CheckForbiddenStatus(t, resp)
}

func TestSearchChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}

	channels, resp := Client.SearchChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	found := false
	for _, c := range channels {
		require.Equal(t, model.CHANNEL_OPEN, c.Type, "should only return public channels")

		if c.Id == th.BasicChannel.Id {
			found = true
		}
	}
	require.True(t, found, "didn't find channel")

	search.Term = th.BasicPrivateChannel.Name
	channels, resp = Client.SearchChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	found = false
	for _, c := range channels {
		if c.Id == th.BasicPrivateChannel.Id {
			found = true
		}
	}
	require.False(t, found, "shouldn't find private channel")

	search.Term = ""
	_, resp = Client.SearchChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	search.Term = th.BasicChannel.Name
	_, resp = Client.SearchChannels(model.NewId(), search)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.SearchChannels("junk", search)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.SearchChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// Remove list channels permission from the user
	th.RemovePermissionFromRole(model.PERMISSION_LIST_TEAM_CHANNELS.Id, model.TEAM_USER_ROLE_ID)

	t.Run("Search for a BasicChannel, which the user is a member of", func(t *testing.T) {
		search.Term = th.BasicChannel.Name
		channelList, resp := Client.SearchChannels(th.BasicTeam.Id, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicChannel.Name)
	})

	t.Run("Remove the user from BasicChannel and search again, should not be returned", func(t *testing.T) {
		th.App.RemoveUserFromChannel(th.Context, th.BasicUser.Id, th.BasicUser.Id, th.BasicChannel)

		search.Term = th.BasicChannel.Name
		channelList, resp := Client.SearchChannels(th.BasicTeam.Id, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.NotContains(t, channelNames, th.BasicChannel.Name)
	})
}

func TestSearchArchivedChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}

	Client.DeleteChannel(th.BasicChannel.Id)

	channels, resp := Client.SearchArchivedChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	found := false
	for _, c := range channels {
		require.Equal(t, model.CHANNEL_OPEN, c.Type)

		if c.Id == th.BasicChannel.Id {
			found = true
		}
	}

	require.True(t, found)

	search.Term = th.BasicPrivateChannel.Name
	Client.DeleteChannel(th.BasicPrivateChannel.Id)

	channels, resp = Client.SearchArchivedChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	found = false
	for _, c := range channels {
		if c.Id == th.BasicPrivateChannel.Id {
			found = true
		}
	}

	require.True(t, found)

	search.Term = ""
	_, resp = Client.SearchArchivedChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	search.Term = th.BasicDeletedChannel.Name
	_, resp = Client.SearchArchivedChannels(model.NewId(), search)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.SearchArchivedChannels("junk", search)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.SearchArchivedChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// Remove list channels permission from the user
	th.RemovePermissionFromRole(model.PERMISSION_LIST_TEAM_CHANNELS.Id, model.TEAM_USER_ROLE_ID)

	t.Run("Search for a BasicDeletedChannel, which the user is a member of", func(t *testing.T) {
		search.Term = th.BasicDeletedChannel.Name
		channelList, resp := Client.SearchArchivedChannels(th.BasicTeam.Id, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicDeletedChannel.Name)
	})

	t.Run("Remove the user from BasicDeletedChannel and search again, should still return", func(t *testing.T) {
		th.App.RemoveUserFromChannel(th.Context, th.BasicUser.Id, th.BasicUser.Id, th.BasicDeletedChannel)

		search.Term = th.BasicDeletedChannel.Name
		channelList, resp := Client.SearchArchivedChannels(th.BasicTeam.Id, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicDeletedChannel.Name)
	})
}

func TestSearchAllChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	openChannel, chanErr := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "SearchAllChannels-FOOBARDISPLAYNAME",
		Name:        "whatever",
		Type:        model.CHANNEL_OPEN,
		TeamId:      th.BasicTeam.Id,
	})
	CheckNoError(t, chanErr)

	privateChannel, privErr := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "SearchAllChannels-private1",
		Name:        "private1",
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      th.BasicTeam.Id,
	})
	CheckNoError(t, privErr)

	team := th.CreateTeam()
	groupConstrainedChannel, groupErr := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName:      "SearchAllChannels-groupConstrained-1",
		Name:             "groupconstrained1",
		Type:             model.CHANNEL_PRIVATE,
		GroupConstrained: model.NewBool(true),
		TeamId:           team.Id,
	})
	CheckNoError(t, groupErr)

	testCases := []struct {
		Description        string
		Search             *model.ChannelSearch
		ExpectedChannelIds []string
	}{
		{
			"Middle of word search",
			&model.ChannelSearch{Term: "bardisplay"},
			[]string{openChannel.Id},
		},
		{
			"Prefix search",
			&model.ChannelSearch{Term: "SearchAllChannels-foobar"},
			[]string{openChannel.Id},
		},
		{
			"Suffix search",
			&model.ChannelSearch{Term: "displayname"},
			[]string{openChannel.Id},
		},
		{
			"Name search",
			&model.ChannelSearch{Term: "what"},
			[]string{openChannel.Id},
		},
		{
			"Name suffix search",
			&model.ChannelSearch{Term: "ever"},
			[]string{openChannel.Id},
		},
		{
			"Basic channel name middle of word search",
			&model.ChannelSearch{Term: th.BasicChannel.Name[2:14]},
			[]string{th.BasicChannel.Id},
		},
		{
			"Upper case search",
			&model.ChannelSearch{Term: strings.ToUpper(th.BasicChannel.Name)},
			[]string{th.BasicChannel.Id},
		},
		{
			"Mixed case search",
			&model.ChannelSearch{Term: th.BasicChannel.Name[0:2] + strings.ToUpper(th.BasicChannel.Name[2:5]) + th.BasicChannel.Name[5:]},
			[]string{th.BasicChannel.Id},
		},
		{
			"Non mixed case search",
			&model.ChannelSearch{Term: th.BasicChannel.Name},
			[]string{th.BasicChannel.Id},
		},
		{
			"Search private channel name",
			&model.ChannelSearch{Term: th.BasicPrivateChannel.Name},
			[]string{th.BasicPrivateChannel.Id},
		},
		{
			"Search with private channel filter",
			&model.ChannelSearch{Private: true},
			[]string{th.BasicPrivateChannel.Id, th.BasicPrivateChannel2.Id, privateChannel.Id, groupConstrainedChannel.Id},
		},
		{
			"Search with public channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", Public: true},
			[]string{openChannel.Id},
		},
		{
			"Search with private channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", Private: true},
			[]string{privateChannel.Id, groupConstrainedChannel.Id},
		},
		{
			"Search with teamIds channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", TeamIds: []string{th.BasicTeam.Id}},
			[]string{openChannel.Id, privateChannel.Id},
		},
		{
			"Search with deleted without IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name},
			[]string{},
		},
		{
			"Search with deleted IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name, IncludeDeleted: true},
			[]string{th.BasicDeletedChannel.Id},
		},
		{
			"Search with deleted IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name, IncludeDeleted: true},
			[]string{th.BasicDeletedChannel.Id},
		},
		{
			"Search with deleted Deleted filter and empty term",
			&model.ChannelSearch{Term: "", Deleted: true},
			[]string{th.BasicDeletedChannel.Id},
		},
		{
			"Search for group constrained",
			&model.ChannelSearch{Term: "SearchAllChannels", GroupConstrained: true},
			[]string{groupConstrainedChannel.Id},
		},
		{
			"Search for group constrained and public",
			&model.ChannelSearch{Term: "SearchAllChannels", GroupConstrained: true, Public: true},
			[]string{},
		},
		{
			"Search for exclude group constrained",
			&model.ChannelSearch{Term: "SearchAllChannels", ExcludeGroupConstrained: true},
			[]string{openChannel.Id, privateChannel.Id},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			channels, resp := th.SystemAdminClient.SearchAllChannels(testCase.Search)
			CheckNoError(t, resp)
			assert.Equal(t, len(testCase.ExpectedChannelIds), len(*channels))
			actualChannelIds := []string{}
			for _, channelWithTeamData := range *channels {
				actualChannelIds = append(actualChannelIds, channelWithTeamData.Channel.Id)
			}
			assert.ElementsMatch(t, testCase.ExpectedChannelIds, actualChannelIds)
		})
	}

	// Searching with no terms returns all default channels
	allChannels, resp := th.SystemAdminClient.SearchAllChannels(&model.ChannelSearch{Term: ""})
	CheckNoError(t, resp)
	assert.True(t, len(*allChannels) >= 3)

	_, resp = Client.SearchAllChannels(&model.ChannelSearch{Term: ""})
	CheckForbiddenStatus(t, resp)

	// Choose a policy which the system manager can read
	sysManagerChannels, resp := th.SystemManagerClient.GetAllChannels(0, 10000, "")
	CheckOKStatus(t, resp)
	policyChannel := (*sysManagerChannels)[0]
	policy, savePolicyErr := th.App.Srv().Store.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:  "Policy 1",
			PostDuration: model.NewInt64(30),
		},
		ChannelIDs: []string{policyChannel.Id},
	})
	require.NoError(t, savePolicyErr)

	t.Run("does not return policy ID", func(t *testing.T) {
		channels, resp := th.SystemManagerClient.SearchAllChannels(&model.ChannelSearch{Term: policyChannel.Name})
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range *channels {
			if channel.Id == policyChannel.Id {
				found = true
				require.Nil(t, channel.PolicyID)
				break
			}
		}
		require.True(t, found)
	})
	t.Run("returns policy ID", func(t *testing.T) {
		channels, resp := th.SystemAdminClient.SearchAllChannels(&model.ChannelSearch{Term: policyChannel.Name})
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range *channels {
			if channel.Id == policyChannel.Id {
				found = true
				require.Equal(t, *channel.PolicyID, policy.ID)
				break
			}
		}
		require.True(t, found)
	})
}

func TestSearchAllChannelsPaged(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}
	search.Term = ""
	search.Page = model.NewInt(0)
	search.PerPage = model.NewInt(2)
	channelsWithCount, resp := th.SystemAdminClient.SearchAllChannelsPaged(search)
	CheckNoError(t, resp)
	require.Len(t, *channelsWithCount.Channels, 2)

	search.Term = th.BasicChannel.Name
	_, resp = Client.SearchAllChannels(search)
	CheckForbiddenStatus(t, resp)
}

func TestSearchGroupChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	u1 := th.CreateUserWithClient(th.SystemAdminClient)

	// Create a group channel in which base user belongs but not sysadmin
	gc1, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.Id, th.BasicUser2.Id, u1.Id})
	CheckNoError(t, resp)
	defer th.Client.DeleteChannel(gc1.Id)

	gc2, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id})
	CheckNoError(t, resp)
	defer th.Client.DeleteChannel(gc2.Id)

	search := &model.ChannelSearch{Term: th.BasicUser2.Username}

	// sysadmin should only find gc2 as he doesn't belong to gc1
	channels, resp := th.SystemAdminClient.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 1)
	assert.Equal(t, channels[0].Id, gc2.Id)

	// basic user should find both
	Client.Login(th.BasicUser.Username, th.BasicUser.Password)
	channels, resp = Client.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 2)
	channelIds := []string{}
	for _, c := range channels {
		channelIds = append(channelIds, c.Id)
	}
	assert.ElementsMatch(t, channelIds, []string{gc1.Id, gc2.Id})

	// searching for sysadmin, it should only find gc1
	search = &model.ChannelSearch{Term: th.SystemAdminUser.Username}
	channels, resp = Client.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 1)
	assert.Equal(t, channels[0].Id, gc2.Id)

	// with an empty search, response should be empty
	search = &model.ChannelSearch{Term: ""}
	channels, resp = Client.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Empty(t, channels)

	// search unprivileged, forbidden
	th.Client.Logout()
	_, resp = Client.SearchAllChannels(search)
	CheckUnauthorizedStatus(t, resp)
}

func TestDeleteChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	c := th.Client
	team := th.BasicTeam
	user := th.BasicUser
	user2 := th.BasicUser2

	// successful delete of public channel
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		publicChannel1 := th.CreatePublicChannel()
		pass, resp := client.DeleteChannel(publicChannel1.Id)
		CheckNoError(t, resp)

		require.True(t, pass, "should have passed")

		ch, err := th.App.GetChannel(publicChannel1.Id)
		require.True(t, err != nil || ch.DeleteAt != 0, "should have failed to get deleted channel, or returned one with a populated DeleteAt.")

		post1 := &model.Post{ChannelId: publicChannel1.Id, Message: "a" + GenerateTestId() + "a"}
		_, resp = client.CreatePost(post1)
		require.NotNil(t, resp, "expected response to not be nil")

		// successful delete of private channel
		privateChannel2 := th.CreatePrivateChannel()
		_, resp = client.DeleteChannel(privateChannel2.Id)
		CheckNoError(t, resp)

		// successful delete of channel with multiple members
		publicChannel3 := th.CreatePublicChannel()
		th.App.AddUserToChannel(user, publicChannel3, false)
		th.App.AddUserToChannel(user2, publicChannel3, false)
		_, resp = client.DeleteChannel(publicChannel3.Id)
		CheckNoError(t, resp)

		// default channel cannot be deleted.
		defaultChannel, _ := th.App.GetChannelByName(model.DEFAULT_CHANNEL, team.Id, false)
		pass, resp = client.DeleteChannel(defaultChannel.Id)
		CheckBadRequestStatus(t, resp)
		require.False(t, pass, "should have failed")

		// check system admin can delete a channel without any appropriate team or channel membership.
		sdTeam := th.CreateTeamWithClient(c)
		sdPublicChannel := &model.Channel{
			DisplayName: "dn_" + model.NewId(),
			Name:        GenerateTestChannelName(),
			Type:        model.CHANNEL_OPEN,
			TeamId:      sdTeam.Id,
		}
		sdPublicChannel, resp = c.CreateChannel(sdPublicChannel)
		CheckNoError(t, resp)
		_, resp = client.DeleteChannel(sdPublicChannel.Id)
		CheckNoError(t, resp)

		sdPrivateChannel := &model.Channel{
			DisplayName: "dn_" + model.NewId(),
			Name:        GenerateTestChannelName(),
			Type:        model.CHANNEL_PRIVATE,
			TeamId:      sdTeam.Id,
		}
		sdPrivateChannel, resp = c.CreateChannel(sdPrivateChannel)
		CheckNoError(t, resp)
		_, resp = client.DeleteChannel(sdPrivateChannel.Id)
		CheckNoError(t, resp)
	})
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {

		th.LoginBasic()
		publicChannel5 := th.CreatePublicChannel()
		c.Logout()

		c.Login(user.Id, user.Password)
		_, resp := c.DeleteChannel(publicChannel5.Id)
		CheckUnauthorizedStatus(t, resp)

		_, resp = c.DeleteChannel("junk")
		CheckUnauthorizedStatus(t, resp)

		c.Logout()
		_, resp = c.DeleteChannel(GenerateTestId())
		CheckUnauthorizedStatus(t, resp)

		_, resp = client.DeleteChannel(publicChannel5.Id)
		CheckNoError(t, resp)

	})

}

func TestDeleteChannel2(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PERMISSION_DELETE_PUBLIC_CHANNEL.Id, model.CHANNEL_USER_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_DELETE_PRIVATE_CHANNEL.Id, model.CHANNEL_USER_ROLE_ID)

	// channels created by SystemAdmin
	publicChannel6 := th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_OPEN)
	privateChannel7 := th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_PRIVATE)
	th.App.AddUserToChannel(user, publicChannel6, false)
	th.App.AddUserToChannel(user, privateChannel7, false)
	th.App.AddUserToChannel(user, privateChannel7, false)

	// successful delete by user
	_, resp := Client.DeleteChannel(publicChannel6.Id)
	CheckNoError(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.Id)
	CheckNoError(t, resp)

	// Restrict permissions to Channel Admins
	th.RemovePermissionFromRole(model.PERMISSION_DELETE_PUBLIC_CHANNEL.Id, model.CHANNEL_USER_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_DELETE_PRIVATE_CHANNEL.Id, model.CHANNEL_USER_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_DELETE_PUBLIC_CHANNEL.Id, model.CHANNEL_ADMIN_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_DELETE_PRIVATE_CHANNEL.Id, model.CHANNEL_ADMIN_ROLE_ID)

	// channels created by SystemAdmin
	publicChannel6 = th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_OPEN)
	privateChannel7 = th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_PRIVATE)
	th.App.AddUserToChannel(user, publicChannel6, false)
	th.App.AddUserToChannel(user, privateChannel7, false)
	th.App.AddUserToChannel(user, privateChannel7, false)

	// cannot delete by user
	_, resp = Client.DeleteChannel(publicChannel6.Id)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.Id)
	CheckForbiddenStatus(t, resp)

	// successful delete by channel admin
	th.MakeUserChannelAdmin(user, publicChannel6)
	th.MakeUserChannelAdmin(user, privateChannel7)
	th.App.Srv().Store.Channel().ClearCaches()

	_, resp = Client.DeleteChannel(publicChannel6.Id)
	CheckNoError(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.Id)
	CheckNoError(t, resp)

	// Make sure team admins don't have permission to delete channels.
	th.RemovePermissionFromRole(model.PERMISSION_DELETE_PUBLIC_CHANNEL.Id, model.CHANNEL_ADMIN_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_DELETE_PRIVATE_CHANNEL.Id, model.CHANNEL_ADMIN_ROLE_ID)

	// last member of a public channel should have required permission to delete
	publicChannel6 = th.CreateChannelWithClient(th.Client, model.CHANNEL_OPEN)
	_, resp = Client.DeleteChannel(publicChannel6.Id)
	CheckForbiddenStatus(t, resp)

	// last member of a private channel should not be able to delete it if they don't have required permissions
	privateChannel7 = th.CreateChannelWithClient(th.Client, model.CHANNEL_PRIVATE)
	_, resp = Client.DeleteChannel(privateChannel7.Id)
	CheckForbiddenStatus(t, resp)
}

func TestPermanentDeleteChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	enableAPIChannelDeletion := *th.App.Config().ServiceSettings.EnableAPIChannelDeletion
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableAPIChannelDeletion = &enableAPIChannelDeletion })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPIChannelDeletion = false })

	publicChannel1 := th.CreatePublicChannel()
	t.Run("Permanent deletion not available through API if EnableAPIChannelDeletion is not set", func(t *testing.T) {
		_, resp := th.SystemAdminClient.PermanentDeleteChannel(publicChannel1.Id)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("Permanent deletion available through local mode even if EnableAPIChannelDeletion is not set", func(t *testing.T) {
		ok, resp := th.LocalClient.PermanentDeleteChannel(publicChannel1.Id)
		CheckNoError(t, resp)
		assert.True(t, ok)
	})

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPIChannelDeletion = true })
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		publicChannel := th.CreatePublicChannel()
		ok, resp := c.PermanentDeleteChannel(publicChannel.Id)
		CheckNoError(t, resp)
		assert.True(t, ok)

		_, err := th.App.GetChannel(publicChannel.Id)
		assert.NotNil(t, err)

		ok, resp = c.PermanentDeleteChannel("junk")
		CheckBadRequestStatus(t, resp)
		require.False(t, ok, "should have returned false")
	}, "Permanent deletion with EnableAPIChannelDeletion set")
}

func TestConvertChannelToPrivate(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	defaultChannel, _ := th.App.GetChannelByName(model.DEFAULT_CHANNEL, th.BasicTeam.Id, false)
	_, resp := Client.ConvertChannelToPrivate(defaultChannel.Id)
	CheckForbiddenStatus(t, resp)

	privateChannel := th.CreatePrivateChannel()
	_, resp = Client.ConvertChannelToPrivate(privateChannel.Id)
	CheckForbiddenStatus(t, resp)

	publicChannel := th.CreatePublicChannel()
	_, resp = Client.ConvertChannelToPrivate(publicChannel.Id)
	CheckForbiddenStatus(t, resp)

	th.LoginTeamAdmin()
	th.RemovePermissionFromRole(model.PERMISSION_CONVERT_PUBLIC_CHANNEL_TO_PRIVATE.Id, model.TEAM_ADMIN_ROLE_ID)

	_, resp = Client.ConvertChannelToPrivate(publicChannel.Id)
	CheckForbiddenStatus(t, resp)

	th.AddPermissionToRole(model.PERMISSION_CONVERT_PUBLIC_CHANNEL_TO_PRIVATE.Id, model.TEAM_ADMIN_ROLE_ID)

	rchannel, resp := Client.ConvertChannelToPrivate(publicChannel.Id)
	CheckOKStatus(t, resp)
	require.Equal(t, model.CHANNEL_PRIVATE, rchannel.Type, "channel should be converted from public to private")

	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(privateChannel.Id)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rchannel, "should not return a channel")

	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(defaultChannel.Id)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rchannel, "should not return a channel")

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	WebSocketClient.Listen()

	publicChannel2 := th.CreatePublicChannel()
	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(publicChannel2.Id)
	CheckOKStatus(t, resp)
	require.Equal(t, model.CHANNEL_PRIVATE, rchannel.Type, "channel should be converted from public to private")

	timeout := time.After(10 * time.Second)

	for {
		select {
		case resp := <-WebSocketClient.EventChannel:
			if resp.EventType() == model.WEBSOCKET_EVENT_CHANNEL_CONVERTED && resp.GetData()["channel_id"].(string) == publicChannel2.Id {
				return
			}
		case <-timeout:
			require.Fail(t, "timed out waiting for channel_converted event")
			return
		}
	}
}

func TestUpdateChannelPrivacy(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	defaultChannel, _ := th.App.GetChannelByName(model.DEFAULT_CHANNEL, th.BasicTeam.Id, false)

	type testTable []struct {
		name            string
		channel         *model.Channel
		expectedPrivacy string
	}

	t.Run("Should get a forbidden response if not logged in", func(t *testing.T) {
		privateChannel := th.CreatePrivateChannel()
		publicChannel := th.CreatePublicChannel()

		tt := testTable{
			{"Updating default channel should fail with forbidden status if not logged in", defaultChannel, model.CHANNEL_OPEN},
			{"Updating private channel should fail with forbidden status if not logged in", privateChannel, model.CHANNEL_PRIVATE},
			{"Updating public channel should fail with forbidden status if not logged in", publicChannel, model.CHANNEL_OPEN},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				_, resp := th.Client.UpdateChannelPrivacy(tc.channel.Id, tc.expectedPrivacy)
				CheckForbiddenStatus(t, resp)
			})
		}
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel()
		publicChannel := th.CreatePublicChannel()

		tt := testTable{
			{"Converting default channel to private should fail", defaultChannel, model.CHANNEL_PRIVATE},
			{"Updating privacy to an invalid setting should fail", publicChannel, "invalid"},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				_, resp := client.UpdateChannelPrivacy(tc.channel.Id, tc.expectedPrivacy)
				CheckBadRequestStatus(t, resp)
			})
		}

		tt = testTable{
			{"Default channel should stay public", defaultChannel, model.CHANNEL_OPEN},
			{"Public channel should stay public", publicChannel, model.CHANNEL_OPEN},
			{"Private channel should stay private", privateChannel, model.CHANNEL_PRIVATE},
			{"Public channel should convert to private", publicChannel, model.CHANNEL_PRIVATE},
			{"Private channel should convert to public", privateChannel, model.CHANNEL_OPEN},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				updatedChannel, resp := client.UpdateChannelPrivacy(tc.channel.Id, tc.expectedPrivacy)
				CheckNoError(t, resp)
				assert.Equal(t, tc.expectedPrivacy, updatedChannel.Type)
				updatedChannel, err := th.App.GetChannel(tc.channel.Id)
				require.Nil(t, err)
				assert.Equal(t, tc.expectedPrivacy, updatedChannel.Type)
			})
		}
	})

	t.Run("Enforces convert channel permissions", func(t *testing.T) {
		privateChannel := th.CreatePrivateChannel()
		publicChannel := th.CreatePublicChannel()

		th.LoginTeamAdmin()

		th.RemovePermissionFromRole(model.PERMISSION_CONVERT_PUBLIC_CHANNEL_TO_PRIVATE.Id, model.TEAM_ADMIN_ROLE_ID)
		th.RemovePermissionFromRole(model.PERMISSION_CONVERT_PRIVATE_CHANNEL_TO_PUBLIC.Id, model.TEAM_ADMIN_ROLE_ID)

		_, resp := th.Client.UpdateChannelPrivacy(publicChannel.Id, model.CHANNEL_PRIVATE)
		CheckForbiddenStatus(t, resp)
		_, resp = th.Client.UpdateChannelPrivacy(privateChannel.Id, model.CHANNEL_OPEN)
		CheckForbiddenStatus(t, resp)

		th.AddPermissionToRole(model.PERMISSION_CONVERT_PUBLIC_CHANNEL_TO_PRIVATE.Id, model.TEAM_ADMIN_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CONVERT_PRIVATE_CHANNEL_TO_PUBLIC.Id, model.TEAM_ADMIN_ROLE_ID)

		_, resp = th.Client.UpdateChannelPrivacy(privateChannel.Id, model.CHANNEL_OPEN)
		CheckNoError(t, resp)
		_, resp = th.Client.UpdateChannelPrivacy(publicChannel.Id, model.CHANNEL_PRIVATE)
		CheckNoError(t, resp)
	})
}

func TestRestoreChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	publicChannel1 := th.CreatePublicChannel()
	th.Client.DeleteChannel(publicChannel1.Id)

	privateChannel1 := th.CreatePrivateChannel()
	th.Client.DeleteChannel(privateChannel1.Id)

	_, resp := th.Client.RestoreChannel(publicChannel1.Id)
	CheckForbiddenStatus(t, resp)

	_, resp = th.Client.RestoreChannel(privateChannel1.Id)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		defer func() {
			client.DeleteChannel(publicChannel1.Id)
			client.DeleteChannel(privateChannel1.Id)
		}()

		_, resp = client.RestoreChannel(publicChannel1.Id)
		CheckOKStatus(t, resp)

		_, resp = client.RestoreChannel(privateChannel1.Id)
		CheckOKStatus(t, resp)
	})
}

func TestGetChannelByName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channel, resp := Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	channel, resp = Client.GetChannelByName(th.BasicPrivateChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicPrivateChannel.Name, channel.Name, "names did not match")

	_, resp = Client.GetChannelByName(strings.ToUpper(th.BasicPrivateChannel.Name), th.BasicTeam.Id, "")
	CheckNoError(t, resp)

	_, resp = Client.GetChannelByName(th.BasicDeletedChannel.Name, th.BasicTeam.Id, "")
	CheckNotFoundStatus(t, resp)

	channel, resp = Client.GetChannelByNameIncludeDeleted(th.BasicDeletedChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicDeletedChannel.Name, channel.Name, "names did not match")

	Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannelByName(th.BasicPrivateChannel.Name, th.BasicTeam.Id, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByName(GenerateTestChannelName(), th.BasicTeam.Id, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByName(GenerateTestChannelName(), "junk", "")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
		CheckNoError(t, resp)
	})
}

func TestGetChannelByNameForTeamName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channel, resp := th.SystemAdminClient.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	channel, resp = Client.GetChannelByNameForTeamName(th.BasicPrivateChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicPrivateChannel.Name, channel.Name, "names did not match")

	_, resp = Client.GetChannelByNameForTeamName(th.BasicDeletedChannel.Name, th.BasicTeam.Name, "")
	CheckNotFoundStatus(t, resp)

	channel, resp = Client.GetChannelByNameForTeamNameIncludeDeleted(th.BasicDeletedChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicDeletedChannel.Name, channel.Name, "names did not match")

	Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannelByNameForTeamName(th.BasicPrivateChannel.Name, th.BasicTeam.Name, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, model.NewRandomString(15), "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByNameForTeamName(GenerateTestChannelName(), th.BasicTeam.Name, "")
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckForbiddenStatus(t, resp)
}

func TestGetChannelMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		members, resp := client.GetChannelMembers(th.BasicChannel.Id, 0, 60, "")
		CheckNoError(t, resp)
		require.Len(t, *members, 3, "should only be 3 users in channel")

		members, resp = client.GetChannelMembers(th.BasicChannel.Id, 0, 2, "")
		CheckNoError(t, resp)
		require.Len(t, *members, 2, "should only be 2 users")

		members, resp = client.GetChannelMembers(th.BasicChannel.Id, 1, 1, "")
		CheckNoError(t, resp)
		require.Len(t, *members, 1, "should only be 1 user")

		members, resp = client.GetChannelMembers(th.BasicChannel.Id, 1000, 100000, "")
		CheckNoError(t, resp)
		require.Empty(t, *members, "should be 0 users")

		_, resp = client.GetChannelMembers("junk", 0, 60, "")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetChannelMembers("", 0, 60, "")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetChannelMembers(th.BasicChannel.Id, 0, 60, "")
		CheckNoError(t, resp)
	})

	_, resp := th.Client.GetChannelMembers(model.NewId(), 0, 60, "")
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.GetChannelMembers(th.BasicChannel.Id, 0, 60, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	th.Client.Login(user.Email, user.Password)
	_, resp = th.Client.GetChannelMembers(th.BasicChannel.Id, 0, 60, "")
	CheckForbiddenStatus(t, resp)
}

func TestGetChannelMembersByIds(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	cm, resp := Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{th.BasicUser.Id})
	CheckNoError(t, resp)
	require.Equal(t, th.BasicUser.Id, (*cm)[0].UserId, "returned wrong user")

	_, resp = Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{})
	CheckBadRequestStatus(t, resp)

	cm1, resp := Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{"junk"})
	CheckNoError(t, resp)
	require.Empty(t, *cm1, "no users should be returned")

	cm1, resp = Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{"junk", th.BasicUser.Id})
	CheckNoError(t, resp)
	require.Len(t, *cm1, 1, "1 member should be returned")

	cm1, resp = Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{th.BasicUser2.Id, th.BasicUser.Id})
	CheckNoError(t, resp)
	require.Len(t, *cm1, 2, "2 members should be returned")

	_, resp = Client.GetChannelMembersByIds("junk", []string{th.BasicUser.Id})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersByIds(model.NewId(), []string{th.BasicUser.Id})
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{th.BasicUser.Id})
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelMembersByIds(th.BasicChannel.Id, []string{th.BasicUser2.Id, th.BasicUser.Id})
	CheckNoError(t, resp)
}

func TestGetChannelMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	c := th.Client
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		member, resp := client.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
		CheckNoError(t, resp)
		require.Equal(t, th.BasicChannel.Id, member.ChannelId, "wrong channel id")
		require.Equal(t, th.BasicUser.Id, member.UserId, "wrong user id")

		_, resp = client.GetChannelMember("", th.BasicUser.Id, "")
		CheckNotFoundStatus(t, resp)

		_, resp = client.GetChannelMember("junk", th.BasicUser.Id, "")
		CheckBadRequestStatus(t, resp)
		_, resp = client.GetChannelMember(th.BasicChannel.Id, "", "")
		CheckNotFoundStatus(t, resp)

		_, resp = client.GetChannelMember(th.BasicChannel.Id, "junk", "")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetChannelMember(th.BasicChannel.Id, model.NewId(), "")
		CheckNotFoundStatus(t, resp)

		_, resp = client.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
		CheckNoError(t, resp)
	})

	_, resp := c.GetChannelMember(model.NewId(), th.BasicUser.Id, "")
	CheckForbiddenStatus(t, resp)

	c.Logout()
	_, resp = c.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	c.Login(user.Email, user.Password)
	_, resp = c.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckForbiddenStatus(t, resp)
}

func TestGetChannelMembersForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	members, resp := Client.GetChannelMembersForUser(th.BasicUser.Id, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
	require.Len(t, *members, 6, "should have 6 members on team")

	_, resp = Client.GetChannelMembersForUser("", th.BasicTeam.Id, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser("junk", th.BasicTeam.Id, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(model.NewId(), th.BasicTeam.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, "", "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, "junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, model.NewId(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, th.BasicTeam.Id, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, th.BasicTeam.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelMembersForUser(th.BasicUser.Id, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
}

func TestViewChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	view := &model.ChannelView{
		ChannelId: th.BasicChannel.Id,
	}

	viewResp, resp := Client.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)
	require.Equal(t, "OK", viewResp.Status, "should have passed")

	channel, _ := th.App.GetChannel(th.BasicChannel.Id)

	require.Equal(t, channel.LastPostAt, viewResp.LastViewedAtTimes[channel.Id], "LastPostAt does not match returned LastViewedAt time")

	view.PrevChannelId = th.BasicChannel.Id
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)

	view.PrevChannelId = ""
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)

	view.PrevChannelId = "junk"
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckBadRequestStatus(t, resp)

	// All blank is OK we use it for clicking off of the browser.
	view.PrevChannelId = ""
	view.ChannelId = ""
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)

	view.PrevChannelId = ""
	view.ChannelId = "junk"
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckBadRequestStatus(t, resp)

	view.ChannelId = "correctlysizedjunkdddfdfdf"
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckBadRequestStatus(t, resp)
	view.ChannelId = th.BasicChannel.Id

	member, resp := Client.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, resp)
	channel, resp = Client.GetChannel(th.BasicChannel.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, channel.TotalMsgCount, member.MsgCount, "should match message counts")
	require.Equal(t, int64(0), member.MentionCount, "should have no mentions")
	require.Equal(t, int64(0), member.MentionCountRoot, "should have no mentions")

	_, resp = Client.ViewChannel("junk", view)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.ViewChannel(th.BasicUser2.Id, view)
	CheckForbiddenStatus(t, resp)

	r, err := Client.DoApiPost(fmt.Sprintf("/channels/members/%v/view", th.BasicUser.Id), "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	Client.Logout()
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)
}

func TestGetChannelUnread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	channel := th.BasicChannel

	channelUnread, resp := Client.GetChannelUnread(channel.Id, user.Id)
	CheckNoError(t, resp)
	require.Equal(t, th.BasicTeam.Id, channelUnread.TeamId, "wrong team id returned for a regular user call")
	require.Equal(t, channel.Id, channelUnread.ChannelId, "wrong team id returned for a regular user call")

	_, resp = Client.GetChannelUnread("junk", user.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelUnread(channel.Id, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelUnread(channel.Id, model.NewId())
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannelUnread(model.NewId(), user.Id)
	CheckForbiddenStatus(t, resp)

	newUser := th.CreateUser()
	Client.Login(newUser.Email, newUser.Password)
	_, resp = Client.GetChannelUnread(th.BasicChannel.Id, user.Id)
	CheckForbiddenStatus(t, resp)

	Client.Logout()

	_, resp = th.SystemAdminClient.GetChannelUnread(channel.Id, user.Id)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetChannelUnread(model.NewId(), user.Id)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelUnread(channel.Id, model.NewId())
	CheckNotFoundStatus(t, resp)
}

func TestGetChannelStats(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.CreatePrivateChannel()

	stats, resp := Client.GetChannelStats(channel.Id, "")
	CheckNoError(t, resp)

	require.Equal(t, channel.Id, stats.ChannelId, "couldnt't get extra info")
	require.Equal(t, int64(1), stats.MemberCount, "got incorrect member count")
	require.Equal(t, int64(0), stats.PinnedPostCount, "got incorrect pinned post count")

	th.CreatePinnedPostWithClient(th.Client, channel)
	stats, resp = Client.GetChannelStats(channel.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, int64(1), stats.PinnedPostCount, "should have returned 1 pinned post count")

	_, resp = Client.GetChannelStats("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelStats(model.NewId(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelStats(channel.Id, "")
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic2()

	_, resp = Client.GetChannelStats(channel.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelStats(channel.Id, "")
	CheckNoError(t, resp)
}

func TestGetPinnedPosts(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	posts, resp := Client.GetPinnedPosts(channel.Id, "")
	CheckNoError(t, resp)
	require.Empty(t, posts.Posts, "should not have gotten a pinned post")

	pinnedPost := th.CreatePinnedPost()
	posts, resp = Client.GetPinnedPosts(channel.Id, "")
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "should have returned 1 pinned post")
	require.Contains(t, posts.Posts, pinnedPost.Id, "missing pinned post")

	posts, resp = Client.GetPinnedPosts(channel.Id, resp.Etag)
	CheckEtag(t, posts, resp)

	_, resp = Client.GetPinnedPosts(GenerateTestId(), "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetPinnedPosts("junk", "")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPinnedPosts(channel.Id, "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPinnedPosts(channel.Id, "")
	CheckNoError(t, resp)
}

func TestUpdateChannelRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	const ChannelAdmin = "channel_user channel_admin"
	const ChannelMember = "channel_user"

	// User 1 creates a channel, making them channel admin by default.
	channel := th.CreatePublicChannel()

	// Adds User 2 to the channel, making them a channel member by default.
	th.App.AddUserToChannel(th.BasicUser2, channel, false)

	// User 1 promotes User 2
	pass, resp := Client.UpdateChannelRoles(channel.Id, th.BasicUser2.Id, ChannelAdmin)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	member, resp := Client.GetChannelMember(channel.Id, th.BasicUser2.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, ChannelAdmin, member.Roles, "roles don't match")

	// User 1 demotes User 2
	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser2.Id, ChannelMember)
	CheckNoError(t, resp)

	th.LoginBasic2()

	// User 2 cannot demote User 1
	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser.Id, ChannelMember)
	CheckForbiddenStatus(t, resp)

	// User 2 cannot promote self
	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser2.Id, ChannelAdmin)
	CheckForbiddenStatus(t, resp)

	th.LoginBasic()

	// User 1 demotes self
	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser.Id, ChannelMember)
	CheckNoError(t, resp)

	// System Admin promotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.Id, th.BasicUser.Id, ChannelAdmin)
	CheckNoError(t, resp)

	// System Admin demotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.Id, th.BasicUser.Id, ChannelMember)
	CheckNoError(t, resp)

	// System Admin promotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.Id, th.BasicUser.Id, ChannelAdmin)
	CheckNoError(t, resp)

	th.LoginBasic()

	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser.Id, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(channel.Id, "junk", ChannelMember)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles("junk", th.BasicUser.Id, ChannelMember)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(channel.Id, model.NewId(), ChannelMember)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(model.NewId(), th.BasicUser.Id, ChannelMember)
	CheckForbiddenStatus(t, resp)
}

func TestUpdateChannelMemberSchemeRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	SystemAdminClient := th.SystemAdminClient
	WebSocketClient, err := th.CreateWebSocketClient()
	WebSocketClient.Listen()
	require.Nil(t, err)

	th.LoginBasic()

	s1 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: false,
	}
	_, r1 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s1)
	CheckNoError(t, r1)

	timeout := time.After(600 * time.Millisecond)
	waiting := true
	for waiting {
		select {
		case event := <-WebSocketClient.EventChannel:
			if event.Event == model.WEBSOCKET_EVENT_CHANNEL_MEMBER_UPDATED {
				require.Equal(t, model.WEBSOCKET_EVENT_CHANNEL_MEMBER_UPDATED, event.Event)
				waiting = false
			}
		case <-timeout:
			require.Fail(t, "Should have received event channel member websocket event and not timedout")
			waiting = false
		}
	}

	tm1, rtm1 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm1)
	assert.Equal(t, false, tm1.SchemeGuest)
	assert.Equal(t, false, tm1.SchemeUser)
	assert.Equal(t, false, tm1.SchemeAdmin)

	s2 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, r2 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s2)
	CheckNoError(t, r2)

	tm2, rtm2 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm2)
	assert.Equal(t, false, tm2.SchemeGuest)
	assert.Equal(t, true, tm2.SchemeUser)
	assert.Equal(t, false, tm2.SchemeAdmin)

	s3 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  false,
		SchemeGuest: false,
	}
	_, r3 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s3)
	CheckNoError(t, r3)

	tm3, rtm3 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm3)
	assert.Equal(t, false, tm3.SchemeGuest)
	assert.Equal(t, false, tm3.SchemeUser)
	assert.Equal(t, true, tm3.SchemeAdmin)

	s4 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, r4 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s4)
	CheckNoError(t, r4)

	tm4, rtm4 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm4)
	assert.Equal(t, false, tm4.SchemeGuest)
	assert.Equal(t, true, tm4.SchemeUser)
	assert.Equal(t, true, tm4.SchemeAdmin)

	s5 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: true,
	}
	_, r5 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s5)
	CheckNoError(t, r5)

	tm5, rtm5 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm5)
	assert.Equal(t, true, tm5.SchemeGuest)
	assert.Equal(t, false, tm5.SchemeUser)
	assert.Equal(t, false, tm5.SchemeAdmin)

	s6 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: true,
	}
	_, resp := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s6)
	CheckBadRequestStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(model.NewId(), th.BasicUser.Id, s4)
	CheckForbiddenStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, model.NewId(), s4)
	CheckNotFoundStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles("ASDF", th.BasicUser.Id, s4)
	CheckBadRequestStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, "ASDF", s4)
	CheckBadRequestStatus(t, resp)

	th.LoginBasic2()
	_, resp = th.Client.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s4)
	CheckForbiddenStatus(t, resp)

	SystemAdminClient.Logout()
	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.SystemAdminUser.Id, s4)
	CheckUnauthorizedStatus(t, resp)
}

func TestUpdateChannelNotifyProps(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	props := map[string]string{}
	props[model.DESKTOP_NOTIFY_PROP] = model.CHANNEL_NOTIFY_MENTION
	props[model.MARK_UNREAD_NOTIFY_PROP] = model.CHANNEL_MARK_UNREAD_MENTION

	pass, resp := Client.UpdateChannelNotifyProps(th.BasicChannel.Id, th.BasicUser.Id, props)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	member, err := th.App.GetChannelMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.Nil(t, err)
	require.Equal(t, model.CHANNEL_NOTIFY_MENTION, member.NotifyProps[model.DESKTOP_NOTIFY_PROP], "bad update")
	require.Equal(t, model.CHANNEL_MARK_UNREAD_MENTION, member.NotifyProps[model.MARK_UNREAD_NOTIFY_PROP], "bad update")

	_, resp = Client.UpdateChannelNotifyProps("junk", th.BasicUser.Id, props)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.Id, "junk", props)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(model.NewId(), th.BasicUser.Id, props)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.Id, model.NewId(), props)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.Id, th.BasicUser.Id, map[string]string{})
	CheckNoError(t, resp)

	Client.Logout()
	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.Id, th.BasicUser.Id, props)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel.Id, th.BasicUser.Id, props)
	CheckNoError(t, resp)
}

func TestAddChannelMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	team := th.BasicTeam
	publicChannel := th.CreatePublicChannel()
	privateChannel := th.CreatePrivateChannel()

	user3 := th.CreateUserWithClient(th.SystemAdminClient)
	_, resp := th.SystemAdminClient.AddTeamMember(team.Id, user3.Id)
	CheckNoError(t, resp)

	cm, resp := Client.AddChannelMember(publicChannel.Id, user2.Id)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.Equal(t, publicChannel.Id, cm.ChannelId, "should have returned exact channel")
	require.Equal(t, user2.Id, cm.UserId, "should have returned exact user added to public channel")

	cm, resp = Client.AddChannelMember(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)
	require.Equal(t, privateChannel.Id, cm.ChannelId, "should have returned exact channel")
	require.Equal(t, user2.Id, cm.UserId, "should have returned exact user added to private channel")

	post := &model.Post{ChannelId: publicChannel.Id, Message: "a" + GenerateTestId() + "a"}
	rpost, err := Client.CreatePost(post)
	require.NotNil(t, err)

	Client.RemoveUserFromChannel(publicChannel.Id, user.Id)
	_, resp = Client.AddChannelMemberWithRootId(publicChannel.Id, user.Id, rpost.Id)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	Client.RemoveUserFromChannel(publicChannel.Id, user.Id)
	_, resp = Client.AddChannelMemberWithRootId(publicChannel.Id, user.Id, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.AddChannelMemberWithRootId(publicChannel.Id, user.Id, GenerateTestId())
	CheckNotFoundStatus(t, resp)

	Client.RemoveUserFromChannel(publicChannel.Id, user.Id)
	_, resp = Client.AddChannelMember(publicChannel.Id, user.Id)
	CheckNoError(t, resp)

	cm, resp = Client.AddChannelMember(publicChannel.Id, "junk")
	CheckBadRequestStatus(t, resp)
	require.Nil(t, cm, "should return nothing")

	_, resp = Client.AddChannelMember(publicChannel.Id, GenerateTestId())
	CheckNotFoundStatus(t, resp)

	_, resp = Client.AddChannelMember("junk", user2.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.AddChannelMember(GenerateTestId(), user2.Id)
	CheckNotFoundStatus(t, resp)

	otherUser := th.CreateUser()
	otherChannel := th.CreatePublicChannel()
	Client.Logout()
	Client.Login(user2.Id, user2.Password)

	_, resp = Client.AddChannelMember(publicChannel.Id, otherUser.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(privateChannel.Id, otherUser.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(otherChannel.Id, otherUser.Id)
	CheckUnauthorizedStatus(t, resp)

	Client.Logout()
	Client.Login(user.Id, user.Password)

	// should fail adding user who is not a member of the team
	_, resp = Client.AddChannelMember(otherChannel.Id, otherUser.Id)
	CheckUnauthorizedStatus(t, resp)

	Client.DeleteChannel(otherChannel.Id)

	// should fail adding user to a deleted channel
	_, resp = Client.AddChannelMember(otherChannel.Id, user2.Id)
	CheckUnauthorizedStatus(t, resp)

	Client.Logout()
	_, resp = Client.AddChannelMember(publicChannel.Id, user2.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(privateChannel.Id, user2.Id)
	CheckUnauthorizedStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.AddChannelMember(publicChannel.Id, user2.Id)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(privateChannel.Id, user2.Id)
		CheckNoError(t, resp)
	})

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_USER_ROLE_ID)

	// Check that a regular channel user can add other users.
	Client.Login(user2.Username, user2.Password)
	privateChannel = th.CreatePrivateChannel()
	_, resp = Client.AddChannelMember(privateChannel.Id, user.Id)
	CheckNoError(t, resp)
	Client.Logout()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.Id, user3.Id)
	CheckNoError(t, resp)
	Client.Logout()

	// Restrict the permission for adding users to Channel Admins
	th.AddPermissionToRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_ADMIN_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_USER_ROLE_ID)

	Client.Login(user2.Username, user2.Password)
	privateChannel = th.CreatePrivateChannel()
	_, resp = Client.AddChannelMember(privateChannel.Id, user.Id)
	CheckNoError(t, resp)
	Client.Logout()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.Id, user3.Id)
	CheckForbiddenStatus(t, resp)
	Client.Logout()

	th.MakeUserChannelAdmin(user, privateChannel)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.Id, user3.Id)
	CheckNoError(t, resp)
	Client.Logout()

	// Set a channel to group-constrained
	privateChannel.GroupConstrained = model.NewBool(true)
	_, appErr := th.App.UpdateChannel(privateChannel)
	require.Nil(t, appErr)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		// User is not in associated groups so shouldn't be allowed
		_, resp = client.AddChannelMember(privateChannel.Id, user.Id)
		CheckErrorMessage(t, resp, "api.channel.add_members.user_denied")
	})

	// Associate group to team
	_, appErr = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupId:    th.Group.Id,
		SyncableId: privateChannel.Id,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.Nil(t, appErr)

	// Add user to group
	_, appErr = th.App.UpsertGroupMember(th.Group.Id, user.Id)
	require.Nil(t, appErr)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.AddChannelMember(privateChannel.Id, user.Id)
		CheckNoError(t, resp)
	})
}

func TestAddChannelMemberAddMyself(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.CreateUser()
	th.LinkUserToTeam(user, th.BasicTeam)
	notMemberPublicChannel1 := th.CreatePublicChannel()
	notMemberPublicChannel2 := th.CreatePublicChannel()
	notMemberPrivateChannel := th.CreatePrivateChannel()

	memberPublicChannel := th.CreatePublicChannel()
	memberPrivateChannel := th.CreatePrivateChannel()
	th.AddUserToChannel(user, memberPublicChannel)
	th.AddUserToChannel(user, memberPrivateChannel)

	testCases := []struct {
		Name                     string
		Channel                  *model.Channel
		WithJoinPublicPermission bool
		ExpectedError            string
	}{
		{
			"Add myself to a public channel with JOIN_PUBLIC_CHANNEL permission",
			notMemberPublicChannel1,
			true,
			"",
		},
		{
			"Try to add myself to a private channel with the JOIN_PUBLIC_CHANNEL permission",
			notMemberPrivateChannel,
			true,
			"api.context.permissions.app_error",
		},
		{
			"Try to add myself to a public channel without the JOIN_PUBLIC_CHANNEL permission",
			notMemberPublicChannel2,
			false,
			"api.context.permissions.app_error",
		},
		{
			"Add myself a public channel where I'm already a member, not having JOIN_PUBLIC_CHANNEL or MANAGE MEMBERS permission",
			memberPublicChannel,
			false,
			"",
		},
		{
			"Add myself a private channel where I'm already a member, not having JOIN_PUBLIC_CHANNEL or MANAGE MEMBERS permission",
			memberPrivateChannel,
			false,
			"",
		},
	}
	Client.Login(user.Email, user.Password)
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {

			// Check the appropriate permissions are enforced.
			defaultRolePermissions := th.SaveDefaultRolePermissions()
			defer func() {
				th.RestoreDefaultRolePermissions(defaultRolePermissions)
			}()

			if !tc.WithJoinPublicPermission {
				th.RemovePermissionFromRole(model.PERMISSION_JOIN_PUBLIC_CHANNELS.Id, model.TEAM_USER_ROLE_ID)
			}

			_, resp := Client.AddChannelMember(tc.Channel.Id, user.Id)
			if tc.ExpectedError == "" {
				CheckNoError(t, resp)
			} else {
				CheckErrorMessage(t, resp, tc.ExpectedError)
			}
		})
	}
}

func TestRemoveChannelMember(t *testing.T) {
	th := Setup(t).InitBasic()
	user1 := th.BasicUser
	user2 := th.BasicUser2
	team := th.BasicTeam
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})
	bot := th.CreateBotWithSystemAdminClient()
	th.App.AddUserToTeam(th.Context, team.Id, bot.UserId, "")

	pass, resp := Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser2.Id)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.Id, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.Id, model.NewId())
	CheckNotFoundStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(model.NewId(), th.BasicUser2.Id)
	CheckNotFoundStatus(t, resp)

	th.LoginBasic2()
	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
	CheckForbiddenStatus(t, resp)

	t.Run("success", func(t *testing.T) {
		// Setup the system administrator to listen for websocket events from the channels.
		th.LinkUserToTeam(th.SystemAdminUser, th.BasicTeam)
		_, err := th.App.AddUserToChannel(th.SystemAdminUser, th.BasicChannel, false)
		require.Nil(t, err)
		_, err = th.App.AddUserToChannel(th.SystemAdminUser, th.BasicChannel2, false)
		require.Nil(t, err)
		props := map[string]string{}
		props[model.DESKTOP_NOTIFY_PROP] = model.CHANNEL_NOTIFY_ALL
		_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel.Id, th.SystemAdminUser.Id, props)
		_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel2.Id, th.SystemAdminUser.Id, props)
		CheckNoError(t, resp)

		wsClient, err := th.CreateWebSocketSystemAdminClient()
		require.Nil(t, err)
		wsClient.Listen()
		var closeWsClient sync.Once
		defer closeWsClient.Do(func() {
			wsClient.Close()
		})

		wsr := <-wsClient.EventChannel
		require.Equal(t, model.WEBSOCKET_EVENT_HELLO, wsr.EventType())

		// requirePost listens for websocket events and tries to find the post matching
		// the expected post's channel and message.
		requirePost := func(expectedPost *model.Post) {
			t.Helper()
			for {
				select {
				case event := <-wsClient.EventChannel:
					postData, ok := event.GetData()["post"]
					if !ok {
						continue
					}

					post := model.PostFromJson(strings.NewReader(postData.(string)))
					if post.ChannelId == expectedPost.ChannelId && post.Message == expectedPost.Message {
						return
					}
				case <-time.After(5 * time.Second):
					require.FailNow(t, "failed to find expected post after 5 seconds")
					return
				}
			}
		}

		th.App.AddUserToChannel(th.BasicUser2, th.BasicChannel, false)
		_, resp = Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser2.Id)
		CheckNoError(t, resp)

		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s left the channel.", th.BasicUser2.Username),
			ChannelId: th.BasicChannel.Id,
		})

		_, resp = Client.RemoveUserFromChannel(th.BasicChannel2.Id, th.BasicUser.Id)
		CheckNoError(t, resp)
		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s removed from the channel.", th.BasicUser.Username),
			ChannelId: th.BasicChannel2.Id,
		})

		_, resp = th.SystemAdminClient.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
		CheckNoError(t, resp)
		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s removed from the channel.", th.BasicUser.Username),
			ChannelId: th.BasicChannel.Id,
		})

		closeWsClient.Do(func() {
			wsClient.Close()
		})
	})

	// Leave deleted channel
	th.LoginBasic()
	deletedChannel := th.CreatePublicChannel()
	th.App.AddUserToChannel(th.BasicUser, deletedChannel, false)
	th.App.AddUserToChannel(th.BasicUser2, deletedChannel, false)

	deletedChannel.DeleteAt = 1
	th.App.UpdateChannel(deletedChannel)

	_, resp = Client.RemoveUserFromChannel(deletedChannel.Id, th.BasicUser.Id)
	CheckNoError(t, resp)

	th.LoginBasic()
	private := th.CreatePrivateChannel()
	th.App.AddUserToChannel(th.BasicUser2, private, false)

	_, resp = Client.RemoveUserFromChannel(private.Id, th.BasicUser2.Id)
	CheckNoError(t, resp)

	th.LoginBasic2()
	_, resp = Client.RemoveUserFromChannel(private.Id, th.BasicUser.Id)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		th.App.AddUserToChannel(th.BasicUser, private, false)
		_, resp = client.RemoveUserFromChannel(private.Id, th.BasicUser.Id)
		CheckNoError(t, resp)
	})

	th.LoginBasic()
	th.UpdateUserToNonTeamAdmin(user1, team)
	th.App.Srv().InvalidateAllCaches()

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_USER_ROLE_ID)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		// Check that a regular channel user can remove other users.
		privateChannel := th.CreateChannelWithClient(client, model.CHANNEL_PRIVATE)
		_, resp = client.AddChannelMember(privateChannel.Id, user1.Id)
		CheckNoError(t, resp)
		_, resp = client.AddChannelMember(privateChannel.Id, user2.Id)
		CheckNoError(t, resp)

		_, resp = Client.RemoveUserFromChannel(privateChannel.Id, user2.Id)
		CheckNoError(t, resp)
	})

	// Restrict the permission for adding users to Channel Admins
	th.AddPermissionToRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_ADMIN_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_USER_ROLE_ID)

	privateChannel := th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_PRIVATE)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user1.Id)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, bot.UserId)
	CheckNoError(t, resp)

	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, user2.Id)
	CheckForbiddenStatus(t, resp)

	th.MakeUserChannelAdmin(user1, privateChannel)
	th.App.Srv().InvalidateAllCaches()

	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, th.SystemAdminUser.Id)
	CheckNoError(t, resp)

	// If the channel is group-constrained the user cannot be removed
	privateChannel.GroupConstrained = model.NewBool(true)
	_, err := th.App.UpdateChannel(privateChannel)
	require.Nil(t, err)
	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, user2.Id)
	require.Equal(t, "api.channel.remove_member.group_constrained.app_error", resp.Error.Id)

	// If the channel is group-constrained user can remove self
	_, resp = th.SystemAdminClient.RemoveUserFromChannel(privateChannel.Id, th.SystemAdminUser.Id)
	CheckNoError(t, resp)

	// Test on preventing removal of user from a direct channel
	directChannel, resp := Client.CreateDirectChannel(user1.Id, user2.Id)
	CheckNoError(t, resp)

	// If the channel is group-constrained a user can remove a bot
	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, bot.UserId)
	CheckNoError(t, resp)

	_, resp = Client.RemoveUserFromChannel(directChannel.Id, user1.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(directChannel.Id, user2.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.RemoveUserFromChannel(directChannel.Id, user1.Id)
	CheckBadRequestStatus(t, resp)

	// Test on preventing removal of user from a group channel
	user3 := th.CreateUser()
	groupChannel, resp := Client.CreateGroupChannel([]string{user1.Id, user2.Id, user3.Id})
	CheckNoError(t, resp)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.RemoveUserFromChannel(groupChannel.Id, user1.Id)
		CheckBadRequestStatus(t, resp)
	})
}

func TestAutocompleteChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// A private channel to make sure private channels are not used
	utils.DisableDebugLogForTest()
	ptown, _ := th.Client.CreateChannel(&model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      th.BasicTeam.Id,
	})
	tower, _ := th.Client.CreateChannel(&model.Channel{
		DisplayName: "Tower",
		Name:        "tower",
		Type:        model.CHANNEL_OPEN,
		TeamId:      th.BasicTeam.Id,
	})
	utils.EnableDebugLogForTest()
	defer func() {
		th.Client.DeleteChannel(ptown.Id)
		th.Client.DeleteChannel(tower.Id)
	}()

	for _, tc := range []struct {
		description      string
		teamId           string
		fragment         string
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"Basic town-square",
			th.BasicTeam.Id,
			"town",
			[]string{"town-square", "town"},
			[]string{"off-topic", "tower"},
		},
		{
			"Basic off-topic",
			th.BasicTeam.Id,
			"off-to",
			[]string{"off-topic"},
			[]string{"town-square", "town", "tower"},
		},
		{
			"Basic town square and off topic",
			th.BasicTeam.Id,
			"tow",
			[]string{"town-square", "tower", "town"},
			[]string{"off-topic"},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, resp := th.Client.AutocompleteChannelsForTeam(tc.teamId, tc.fragment)
			require.Nil(t, resp.Error)
			names := make([]string, len(*channels))
			for i, c := range *channels {
				names[i] = c.Name
			}
			for _, name := range tc.expectedIncludes {
				require.Contains(t, names, name, "channel not included")
			}
			for _, name := range tc.expectedExcludes {
				require.NotContains(t, names, name, "channel not excluded")
			}
		})
	}
}

func TestAutocompleteChannelsForSearch(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.LoginSystemAdminWithClient(th.SystemAdminClient)
	th.LoginBasicWithClient(th.Client)

	u1 := th.CreateUserWithClient(th.SystemAdminClient)
	defer th.App.PermanentDeleteUser(th.Context, u1)
	u2 := th.CreateUserWithClient(th.SystemAdminClient)
	defer th.App.PermanentDeleteUser(th.Context, u2)
	u3 := th.CreateUserWithClient(th.SystemAdminClient)
	defer th.App.PermanentDeleteUser(th.Context, u3)
	u4 := th.CreateUserWithClient(th.SystemAdminClient)
	defer th.App.PermanentDeleteUser(th.Context, u4)

	// A private channel to make sure private channels are not used
	utils.DisableDebugLogForTest()
	ptown, _ := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      th.BasicTeam.Id,
	})
	defer func() {
		th.Client.DeleteChannel(ptown.Id)
	}()
	mypriv, _ := th.Client.CreateChannel(&model.Channel{
		DisplayName: "My private town",
		Name:        "townpriv",
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      th.BasicTeam.Id,
	})
	defer func() {
		th.Client.DeleteChannel(mypriv.Id)
	}()
	utils.EnableDebugLogForTest()

	dc1, resp := th.Client.CreateDirectChannel(th.BasicUser.Id, u1.Id)
	CheckNoError(t, resp)
	defer func() {
		th.Client.DeleteChannel(dc1.Id)
	}()

	dc2, resp := th.SystemAdminClient.CreateDirectChannel(u2.Id, u3.Id)
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(dc2.Id)
	}()

	gc1, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.Id, u2.Id, u3.Id})
	CheckNoError(t, resp)
	defer func() {
		th.Client.DeleteChannel(gc1.Id)
	}()

	gc2, resp := th.SystemAdminClient.CreateGroupChannel([]string{u2.Id, u3.Id, u4.Id})
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(gc2.Id)
	}()

	for _, tc := range []struct {
		description      string
		teamID           string
		fragment         string
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"Basic town-square",
			th.BasicTeam.Id,
			"town",
			[]string{"town-square", "townpriv"},
			[]string{"off-topic", "town"},
		},
		{
			"Basic off-topic",
			th.BasicTeam.Id,
			"off-to",
			[]string{"off-topic"},
			[]string{"town-square", "town", "townpriv"},
		},
		{
			"Basic town square and townpriv",
			th.BasicTeam.Id,
			"tow",
			[]string{"town-square", "townpriv"},
			[]string{"off-topic", "town"},
		},
		{
			"Direct and group messages",
			th.BasicTeam.Id,
			"fakeuser",
			[]string{dc1.Name, gc1.Name},
			[]string{dc2.Name, gc2.Name},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, resp := th.Client.AutocompleteChannelsForTeamForSearch(tc.teamID, tc.fragment)
			require.Nil(t, resp.Error)
			names := make([]string, len(*channels))
			for i, c := range *channels {
				names[i] = c.Name
			}
			for _, name := range tc.expectedIncludes {
				require.Contains(t, names, name, "channel not included")
			}
			for _, name := range tc.expectedExcludes {
				require.NotContains(t, names, name, "channel not excluded")
			}
		})
	}
}

func TestAutocompleteChannelsForSearchGuestUsers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	u1 := th.CreateUserWithClient(th.SystemAdminClient)
	defer th.App.PermanentDeleteUser(th.Context, u1)

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		th.App.Srv().RemoveLicense()
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	id := model.NewId()
	guest := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}
	guest, err := th.App.CreateGuest(th.Context, guest)
	require.Nil(t, err)

	th.LoginSystemAdminWithClient(th.SystemAdminClient)

	_, resp := th.SystemAdminClient.AddTeamMember(th.BasicTeam.Id, guest.Id)
	CheckNoError(t, resp)

	// A private channel to make sure private channels are not used
	utils.DisableDebugLogForTest()
	town, _ := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.CHANNEL_OPEN,
		TeamId:      th.BasicTeam.Id,
	})
	defer func() {
		th.SystemAdminClient.DeleteChannel(town.Id)
	}()
	_, resp = th.SystemAdminClient.AddChannelMember(town.Id, guest.Id)
	CheckNoError(t, resp)

	mypriv, _ := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "My private town",
		Name:        "townpriv",
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      th.BasicTeam.Id,
	})
	defer func() {
		th.SystemAdminClient.DeleteChannel(mypriv.Id)
	}()
	_, resp = th.SystemAdminClient.AddChannelMember(mypriv.Id, guest.Id)
	CheckNoError(t, resp)

	utils.EnableDebugLogForTest()

	dc1, resp := th.SystemAdminClient.CreateDirectChannel(th.BasicUser.Id, guest.Id)
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(dc1.Id)
	}()

	dc2, resp := th.SystemAdminClient.CreateDirectChannel(th.BasicUser.Id, th.BasicUser2.Id)
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(dc2.Id)
	}()

	gc1, resp := th.SystemAdminClient.CreateGroupChannel([]string{th.BasicUser.Id, th.BasicUser2.Id, guest.Id})
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(gc1.Id)
	}()

	gc2, resp := th.SystemAdminClient.CreateGroupChannel([]string{th.BasicUser.Id, th.BasicUser2.Id, u1.Id})
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(gc2.Id)
	}()

	_, resp = th.Client.Login(guest.Username, "Password1")
	CheckNoError(t, resp)

	for _, tc := range []struct {
		description      string
		teamID           string
		fragment         string
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"Should return those channel where is member",
			th.BasicTeam.Id,
			"town",
			[]string{"town", "townpriv"},
			[]string{"town-square", "off-topic"},
		},
		{
			"Should return empty if not member of the searched channels",
			th.BasicTeam.Id,
			"off-to",
			[]string{},
			[]string{"off-topic", "town-square", "town", "townpriv"},
		},
		{
			"Should return direct and group messages",
			th.BasicTeam.Id,
			"fakeuser",
			[]string{dc1.Name, gc1.Name},
			[]string{dc2.Name, gc2.Name},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, resp := th.Client.AutocompleteChannelsForTeamForSearch(tc.teamID, tc.fragment)
			require.Nil(t, resp.Error)
			names := make([]string, len(*channels))
			for i, c := range *channels {
				names[i] = c.Name
			}
			for _, name := range tc.expectedIncludes {
				require.Contains(t, names, name, "channel not included")
			}
			for _, name := range tc.expectedExcludes {
				require.NotContains(t, names, name, "channel not excluded")
			}
		})
	}
}

func TestUpdateChannelScheme(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense(""))

	th.App.SetPhase2PermissionsMigrationStatus(true)

	team, resp := th.SystemAdminClient.CreateTeam(&model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteId:        "inviteid0",
		Name:            "z-z-" + model.NewId() + "a",
		Email:           "success+" + model.NewId() + "@simulator.amazonses.com",
		Type:            model.TEAM_OPEN,
	})
	CheckNoError(t, resp)

	channel, resp := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "Name",
		Name:        "z-z-" + model.NewId() + "a",
		Type:        model.CHANNEL_OPEN,
		TeamId:      team.Id,
	})
	CheckNoError(t, resp)

	channelScheme, resp := th.SystemAdminClient.CreateScheme(&model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewId(),
		Description: "Some description",
		Scope:       model.SCHEME_SCOPE_CHANNEL,
	})
	CheckNoError(t, resp)

	teamScheme, resp := th.SystemAdminClient.CreateScheme(&model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewId(),
		Description: "Some description",
		Scope:       model.SCHEME_SCOPE_TEAM,
	})
	CheckNoError(t, resp)

	// Test the setup/base case.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, channelScheme.Id)
	CheckNoError(t, resp)

	// Test various invalid channel and scheme id combinations.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, "x")
	CheckBadRequestStatus(t, resp)
	_, resp = th.SystemAdminClient.UpdateChannelScheme("x", channelScheme.Id)
	CheckBadRequestStatus(t, resp)
	_, resp = th.SystemAdminClient.UpdateChannelScheme("x", "x")
	CheckBadRequestStatus(t, resp)

	// Test that permissions are required.
	_, resp = th.Client.UpdateChannelScheme(channel.Id, channelScheme.Id)
	CheckForbiddenStatus(t, resp)

	// Test that a license is required.
	th.App.Srv().SetLicense(nil)
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, channelScheme.Id)
	CheckNotImplementedStatus(t, resp)
	th.App.Srv().SetLicense(model.NewTestLicense(""))

	// Test an invalid scheme scope.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, teamScheme.Id)
	CheckBadRequestStatus(t, resp)

	// Test that an unauthenticated user gets rejected.
	th.SystemAdminClient.Logout()
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, channelScheme.Id)
	CheckUnauthorizedStatus(t, resp)
}

func TestGetChannelMembersTimezones(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	user := th.BasicUser
	user.Timezone["useAutomaticTimezone"] = "false"
	user.Timezone["manualTimezone"] = "XOXO/BLABLA"
	_, resp := Client.UpdateUser(user)
	CheckNoError(t, resp)

	user2 := th.BasicUser2
	user2.Timezone["automaticTimezone"] = "NoWhere/Island"
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	timezone, resp := Client.GetChannelMembersTimezones(th.BasicChannel.Id)
	CheckNoError(t, resp)
	require.Len(t, timezone, 2, "should return 2 timezones")

	//both users have same timezone
	user2.Timezone["automaticTimezone"] = "XOXO/BLABLA"
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	timezone, resp = Client.GetChannelMembersTimezones(th.BasicChannel.Id)
	CheckNoError(t, resp)
	require.Len(t, timezone, 1, "should return 1 timezone")

	//no timezone set should return empty
	user2.Timezone["automaticTimezone"] = ""
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	user.Timezone["manualTimezone"] = ""
	_, resp = Client.UpdateUser(user)
	CheckNoError(t, resp)

	timezone, resp = Client.GetChannelMembersTimezones(th.BasicChannel.Id)
	CheckNoError(t, resp)
	require.Empty(t, timezone, "should return 0 timezone")
}

func TestChannelMembersMinusGroupMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user1 := th.BasicUser
	user2 := th.BasicUser2

	channel := th.CreatePrivateChannel()

	_, err := th.App.AddChannelMember(th.Context, user1.Id, channel, app.ChannelMemberOpts{})
	require.Nil(t, err)
	_, err = th.App.AddChannelMember(th.Context, user2.Id, channel, app.ChannelMemberOpts{})
	require.Nil(t, err)

	channel.GroupConstrained = model.NewBool(true)
	channel, err = th.App.UpdateChannel(channel)
	require.Nil(t, err)

	group1 := th.CreateGroup()
	group2 := th.CreateGroup()

	_, err = th.App.UpsertGroupMember(group1.Id, user1.Id)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupMember(group2.Id, user2.Id)
	require.Nil(t, err)

	// No permissions
	_, _, res := th.Client.ChannelMembersMinusGroupMembers(channel.Id, []string{group1.Id, group2.Id}, 0, 100, "")
	require.Equal(t, "api.context.permissions.app_error", res.Error.Id)

	testCases := map[string]struct {
		groupIDs        []string
		page            int
		perPage         int
		length          int
		count           int
		otherAssertions func([]*model.UserWithGroups)
	}{
		"All groups, expect no users removed": {
			groupIDs: []string{group1.Id, group2.Id},
			page:     0,
			perPage:  100,
			length:   0,
			count:    0,
		},
		"Some nonexistent group, page 0": {
			groupIDs: []string{model.NewId()},
			page:     0,
			perPage:  1,
			length:   1,
			count:    2,
		},
		"Some nonexistent group, page 1": {
			groupIDs: []string{model.NewId()},
			page:     1,
			perPage:  1,
			length:   1,
			count:    2,
		},
		"One group, expect one user removed": {
			groupIDs: []string{group1.Id},
			page:     0,
			perPage:  100,
			length:   1,
			count:    1,
			otherAssertions: func(uwg []*model.UserWithGroups) {
				require.Equal(t, uwg[0].Id, user2.Id)
			},
		},
		"Other group, expect other user removed": {
			groupIDs: []string{group2.Id},
			page:     0,
			perPage:  100,
			length:   1,
			count:    1,
			otherAssertions: func(uwg []*model.UserWithGroups) {
				require.Equal(t, uwg[0].Id, user1.Id)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uwg, count, res := th.SystemAdminClient.ChannelMembersMinusGroupMembers(channel.Id, tc.groupIDs, tc.page, tc.perPage, "")
			require.Nil(t, res.Error)
			require.Len(t, uwg, tc.length)
			require.Equal(t, tc.count, int(count))
			if tc.otherAssertions != nil {
				tc.otherAssertions(uwg)
			}
		})
	}
}

func TestGetChannelModerations(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel
	team := th.BasicTeam

	th.App.SetPhase2PermissionsMigrationStatus(true)

	t.Run("Errors without a license", func(t *testing.T) {
		_, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Equal(t, "api.channel.get_channel_moderations.license.error", res.Error.Id)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors as a non sysadmin", func(t *testing.T) {
		_, res := th.Client.GetChannelModerations(channel.Id, "")
		require.Equal(t, "api.context.permissions.app_error", res.Error.Id)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Returns default moderations with default roles", func(t *testing.T) {
		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}
	})

	t.Run("Returns value false and enabled false for permissions that are not present in higher scoped scheme when no channel scheme present", func(t *testing.T) {
		scheme := th.SetupTeamScheme()
		team.SchemeId = &scheme.Id
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		th.RemovePermissionFromRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PERMISSION_CREATE_POST.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}
		}
	})

	t.Run("Returns value false and enabled true for permissions that are not present in channel scheme but present in team scheme", func(t *testing.T) {
		scheme := th.SetupChannelScheme()
		channel.SchemeId = &scheme.Id
		_, err := th.App.UpdateChannelScheme(channel)
		require.Nil(t, err)

		th.RemovePermissionFromRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PERMISSION_CREATE_POST.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}
		}
	})

	t.Run("Returns value false and enabled false for permissions that are not present in channel & team scheme", func(t *testing.T) {
		teamScheme := th.SetupTeamScheme()
		team.SchemeId = &teamScheme.Id
		th.App.UpdateTeamScheme(team)

		scheme := th.SetupChannelScheme()
		channel.SchemeId = &scheme.Id
		th.App.UpdateChannelScheme(channel)

		th.RemovePermissionFromRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)
		th.RemovePermissionFromRole(model.PERMISSION_CREATE_POST.Id, teamScheme.DefaultChannelGuestRole)

		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, teamScheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PERMISSION_CREATE_POST.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}
		}
	})

	t.Run("Returns the correct value for manage_members depending on whether the channel is public or private", func(t *testing.T) {
		scheme := th.SetupTeamScheme()
		team.SchemeId = &scheme.Id
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		th.RemovePermissionFromRole(model.PERMISSION_MANAGE_PUBLIC_CHANNEL_MEMBERS.Id, scheme.DefaultChannelUserRole)
		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelUserRole)

		// public channel does not have the permission
		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Equal(t, moderation.Roles.Members.Value, false)
			}
		}

		// private channel does have the permission
		moderations, res = th.SystemAdminClient.GetChannelModerations(th.BasicPrivateChannel.Id, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Equal(t, moderation.Roles.Members.Value, true)
			}
		}
	})

	t.Run("Does not return an error if the team scheme has a blank DefaultChannelGuestRole field", func(t *testing.T) {
		scheme := th.SetupTeamScheme()
		scheme.DefaultChannelGuestRole = ""

		mockStore := mocks.Store{}
		mockSchemeStore := mocks.SchemeStore{}
		mockSchemeStore.On("Get", mock.Anything).Return(scheme, nil)
		mockStore.On("Scheme").Return(&mockSchemeStore)
		mockStore.On("Team").Return(th.App.Srv().Store.Team())
		mockStore.On("Channel").Return(th.App.Srv().Store.Channel())
		mockStore.On("User").Return(th.App.Srv().Store.User())
		mockStore.On("Post").Return(th.App.Srv().Store.Post())
		mockStore.On("FileInfo").Return(th.App.Srv().Store.FileInfo())
		mockStore.On("Webhook").Return(th.App.Srv().Store.Webhook())
		mockStore.On("System").Return(th.App.Srv().Store.System())
		mockStore.On("License").Return(th.App.Srv().Store.License())
		mockStore.On("Role").Return(th.App.Srv().Store.Role())
		mockStore.On("Close").Return(nil)
		th.App.Srv().Store = &mockStore

		team.SchemeId = &scheme.Id
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		_, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
	})
}

func TestPatchChannelModerations(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel

	emptyPatch := []*model.ChannelModerationPatch{}

	createPosts := model.ChannelModeratedPermissions[0]

	th.App.SetPhase2PermissionsMigrationStatus(true)

	t.Run("Errors without a license", func(t *testing.T) {
		_, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, emptyPatch)
		require.Equal(t, "api.channel.patch_channel_moderations.license.error", res.Error.Id)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors as a non sysadmin", func(t *testing.T) {
		_, res := th.Client.PatchChannelModerations(channel.Id, emptyPatch)
		require.Equal(t, "api.context.permissions.app_error", res.Error.Id)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Returns default moderations with empty patch", func(t *testing.T) {
		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, emptyPatch)
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}

		require.Nil(t, channel.SchemeId)
	})

	t.Run("Creates a scheme and returns the updated channel moderations when patching an existing permission", func(t *testing.T) {
		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(false)},
			},
		}

		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, patch)
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			if moderation.Name == createPosts {
				require.Equal(t, moderation.Roles.Members.Value, false)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
			} else {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
			}
		}
		channel, _ = th.App.GetChannel(channel.Id)
		require.NotNil(t, channel.SchemeId)
	})

	t.Run("Removes the existing scheme when moderated permissions are set back to higher scoped values", func(t *testing.T) {
		channel, _ = th.App.GetChannel(channel.Id)
		schemeId := channel.SchemeId

		scheme, _ := th.App.GetScheme(*schemeId)
		require.Equal(t, scheme.DeleteAt, int64(0))

		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(true)},
			},
		}

		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, patch)
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}

		channel, _ = th.App.GetChannel(channel.Id)
		require.Nil(t, channel.SchemeId)

		scheme, _ = th.App.GetScheme(*schemeId)
		require.NotEqual(t, scheme.DeleteAt, int64(0))
	})

	t.Run("Does not return an error if the team scheme has a blank DefaultChannelGuestRole field", func(t *testing.T) {
		team := th.BasicTeam
		scheme := th.SetupTeamScheme()
		scheme.DefaultChannelGuestRole = ""

		mockStore := mocks.Store{}
		mockSchemeStore := mocks.SchemeStore{}
		mockSchemeStore.On("Get", mock.Anything).Return(scheme, nil)
		mockSchemeStore.On("Save", mock.Anything).Return(scheme, nil)
		mockSchemeStore.On("Delete", mock.Anything).Return(scheme, nil)
		mockStore.On("Scheme").Return(&mockSchemeStore)
		mockStore.On("Team").Return(th.App.Srv().Store.Team())
		mockStore.On("Channel").Return(th.App.Srv().Store.Channel())
		mockStore.On("User").Return(th.App.Srv().Store.User())
		mockStore.On("Post").Return(th.App.Srv().Store.Post())
		mockStore.On("FileInfo").Return(th.App.Srv().Store.FileInfo())
		mockStore.On("Webhook").Return(th.App.Srv().Store.Webhook())
		mockStore.On("System").Return(th.App.Srv().Store.System())
		mockStore.On("License").Return(th.App.Srv().Store.License())
		mockStore.On("Role").Return(th.App.Srv().Store.Role())
		mockStore.On("Close").Return(nil)
		th.App.Srv().Store = &mockStore

		team.SchemeId = &scheme.Id
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, emptyPatch)
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}

		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(true)},
			},
		}

		moderations, res = th.SystemAdminClient.PatchChannelModerations(channel.Id, patch)
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}
	})

}

func TestGetChannelMemberCountsByGroup(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel
	t.Run("Errors without a license", func(t *testing.T) {
		_, res := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.Id, false, "")
		require.Equal(t, "api.channel.channel_member_counts_by_group.license.error", res.Error.Id)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors without read permission to the channel", func(t *testing.T) {
		_, res := th.Client.GetChannelMemberCountsByGroup(model.NewId(), false, "")
		require.Equal(t, "api.context.permissions.app_error", res.Error.Id)
	})

	t.Run("Returns empty for a channel with no members or groups", func(t *testing.T) {
		memberCounts, _ := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.Id, false, "")
		require.Equal(t, []*model.ChannelMemberCountByGroup{}, memberCounts)
	})

	user := th.BasicUser
	user.Timezone["useAutomaticTimezone"] = "false"
	user.Timezone["manualTimezone"] = "XOXO/BLABLA"
	_, err := th.App.UpsertGroupMember(th.Group.Id, user.Id)
	require.Nil(t, err)
	_, resp := th.SystemAdminClient.UpdateUser(user)
	CheckNoError(t, resp)

	user2 := th.BasicUser2
	user2.Timezone["automaticTimezone"] = "NoWhere/Island"
	_, err = th.App.UpsertGroupMember(th.Group.Id, user2.Id)
	require.Nil(t, err)
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	t.Run("Returns users in group without timezones", func(t *testing.T) {
		memberCounts, _ := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.Id, false, "")
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     th.Group.Id,
				ChannelMemberCount:          2,
				ChannelMemberTimezonesCount: 0,
			},
		}
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	t.Run("Returns users in group with timezones", func(t *testing.T) {
		memberCounts, _ := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.Id, true, "")
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     th.Group.Id,
				ChannelMemberCount:          2,
				ChannelMemberTimezonesCount: 2,
			},
		}
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	id := model.NewId()
	group := &model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		RemoteId:    model.NewId(),
	}

	_, err = th.App.CreateGroup(group)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupMember(group.Id, user.Id)
	require.Nil(t, err)

	t.Run("Returns multiple groups with users in group with timezones", func(t *testing.T) {
		memberCounts, _ := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.Id, true, "")
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     group.Id,
				ChannelMemberCount:          1,
				ChannelMemberTimezonesCount: 1,
			},
			{
				GroupId:                     th.Group.Id,
				ChannelMemberCount:          2,
				ChannelMemberTimezonesCount: 2,
			},
		}
		require.ElementsMatch(t, expectedMemberCounts, memberCounts)
	})
}

func TestMoveChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	team1 := th.BasicTeam
	team2 := th.CreateTeam()

	t.Run("Should move channel", func(t *testing.T) {
		publicChannel := th.CreatePublicChannel()
		ch, resp := th.SystemAdminClient.MoveChannel(publicChannel.Id, team2.Id, false)
		require.Nil(t, resp.Error)
		require.Equal(t, team2.Id, ch.TeamId)
	})

	t.Run("Should move private channel", func(t *testing.T) {
		channel := th.CreatePrivateChannel()
		ch, resp := th.SystemAdminClient.MoveChannel(channel.Id, team1.Id, false)
		require.Nil(t, resp.Error)
		require.Equal(t, team1.Id, ch.TeamId)
	})

	t.Run("Should fail when trying to move a DM channel", func(t *testing.T) {
		user := th.CreateUser()
		dmChannel := th.CreateDmChannel(user)
		_, resp := Client.MoveChannel(dmChannel.Id, team1.Id, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "api.channel.move_channel.type.invalid")
	})

	t.Run("Should fail when trying to move a group channel", func(t *testing.T) {
		user := th.CreateUser()

		gmChannel, err := th.App.CreateGroupChannel([]string{th.BasicUser.Id, th.SystemAdminUser.Id, th.TeamAdminUser.Id}, user.Id)
		require.Nil(t, err)
		_, resp := Client.MoveChannel(gmChannel.Id, team1.Id, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "api.channel.move_channel.type.invalid")
	})

	t.Run("Should fail due to permissions", func(t *testing.T) {
		publicChannel := th.CreatePublicChannel()
		_, resp := Client.MoveChannel(publicChannel.Id, team1.Id, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		publicChannel := th.CreatePublicChannel()
		user := th.BasicUser

		_, resp := client.RemoveTeamMember(team2.Id, user.Id)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(publicChannel.Id, user.Id)
		CheckNoError(t, resp)

		_, resp = client.MoveChannel(publicChannel.Id, team2.Id, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "app.channel.move_channel.members_do_not_match.error")
	}, "Should fail to move public channel due to a member not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel()
		user := th.BasicUser

		_, resp := client.RemoveTeamMember(team2.Id, user.Id)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(privateChannel.Id, user.Id)
		CheckNoError(t, resp)

		_, resp = client.MoveChannel(privateChannel.Id, team2.Id, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "app.channel.move_channel.members_do_not_match.error")
	}, "Should fail to move private channel due to a member not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		publicChannel := th.CreatePublicChannel()
		user := th.BasicUser

		_, resp := client.RemoveTeamMember(team2.Id, user.Id)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(publicChannel.Id, user.Id)
		CheckNoError(t, resp)

		newChannel, resp := client.MoveChannel(publicChannel.Id, team2.Id, true)
		require.Nil(t, resp.Error)
		require.Equal(t, team2.Id, newChannel.TeamId)
	}, "Should be able to (force) move public channel by a member that is not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel()
		user := th.BasicUser

		_, resp := client.RemoveTeamMember(team2.Id, user.Id)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(privateChannel.Id, user.Id)
		CheckNoError(t, resp)

		newChannel, resp := client.MoveChannel(privateChannel.Id, team2.Id, true)
		require.Nil(t, resp.Error)
		require.Equal(t, team2.Id, newChannel.TeamId)
	}, "Should be able to (force) move private channel by a member that is not member of target team")
}

func TestRootMentionsCount(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	user := th.BasicUser
	channel := th.BasicChannel

	// initially, MentionCountRoot is 0 in the database
	channelMember, err := th.App.Srv().Store.Channel().GetMember(context.Background(), channel.Id, user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(0), channelMember.MentionCountRoot)
	require.Equal(t, int64(0), channelMember.MentionCount)

	// mention the user in a root post
	post1, resp := th.SystemAdminClient.CreatePost(&model.Post{ChannelId: channel.Id, Message: "hey @" + user.Username})
	CheckNoError(t, resp)
	// mention the user in a reply post
	post2 := &model.Post{ChannelId: channel.Id, Message: "reply at @" + user.Username, RootId: post1.Id}
	_, resp = th.SystemAdminClient.CreatePost(post2)
	CheckNoError(t, resp)

	// this should perform lazy migration and populate the field
	channelUnread, resp := Client.GetChannelUnread(channel.Id, user.Id)
	CheckNoError(t, resp)
	// reply post is not counted, so we should have one root mention
	require.EqualValues(t, int64(1), channelUnread.MentionCountRoot)
	// regular count stays the same
	require.Equal(t, int64(2), channelUnread.MentionCount)
	// validate that DB is updated
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channel.Id, user.Id)
	require.NoError(t, err)
	require.EqualValues(t, int64(1), channelMember.MentionCountRoot)

	// validate that Team level counts are calculated
	counts, appErr := th.App.GetTeamUnread(channel.TeamId, user.Id)
	require.Nil(t, appErr)
	require.Equal(t, int64(1), counts.MentionCountRoot)
	require.Equal(t, int64(2), counts.MentionCount)
}

func TestViewChannelWithoutCollapsedThreads(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	Client := th.Client
	user := th.BasicUser
	team := th.BasicTeam
	channel := th.BasicChannel

	// mention the user in a root post
	post1, resp := th.SystemAdminClient.CreatePost(&model.Post{ChannelId: channel.Id, Message: "hey @" + user.Username})
	CheckNoError(t, resp)
	// mention the user in a reply post
	post2 := &model.Post{ChannelId: channel.Id, Message: "reply at @" + user.Username, RootId: post1.Id}
	_, resp = th.SystemAdminClient.CreatePost(post2)
	CheckNoError(t, resp)

	threads, resp := Client.GetUserThreads(user.Id, team.Id, model.GetUserThreadsOpts{})
	CheckNoError(t, resp)
	require.EqualValues(t, int64(1), threads.TotalUnreadMentions)

	// simulate opening the channel from an old client
	_, resp = Client.ViewChannel(user.Id, &model.ChannelView{
		ChannelId:                 channel.Id,
		PrevChannelId:             "",
		CollapsedThreadsSupported: false,
	})
	CheckNoError(t, resp)

	threads, resp = Client.GetUserThreads(user.Id, team.Id, model.GetUserThreadsOpts{})
	CheckNoError(t, resp)
	require.Zero(t, threads.TotalUnreadMentions)
}
