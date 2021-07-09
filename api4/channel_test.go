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

	channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamID: team.ID}
	private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypePrivate, TeamID: team.ID}

	rchannel, resp := Client.CreateChannel(channel)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	require.Equal(t, channel.Name, rchannel.Name, "names did not match")
	require.Equal(t, channel.DisplayName, rchannel.DisplayName, "display names did not match")
	require.Equal(t, channel.TeamID, rchannel.TeamID, "team ids did not match")

	rprivate, resp := Client.CreateChannel(private)
	CheckNoError(t, resp)

	require.Equal(t, private.Name, rprivate.Name, "names did not match")
	require.Equal(t, model.ChannelTypePrivate, rprivate.Type, "wrong channel type")
	require.Equal(t, th.BasicUser.ID, rprivate.CreatorID, "wrong creator id")

	_, resp = Client.CreateChannel(channel)
	CheckErrorMessage(t, resp, "store.sql_channel.save_channel.exists.app_error")
	CheckBadRequestStatus(t, resp)

	direct := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeDirect, TeamID: team.ID}
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

	th.AddPermissionToRole(model.PermissionCreatePublicChannel.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionCreatePrivateChannel.ID, model.TeamUserRoleID)

	th.LoginBasic()

	channel.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(channel)
	CheckNoError(t, resp)

	private.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(private)
	CheckNoError(t, resp)

	th.AddPermissionToRole(model.PermissionCreatePublicChannel.ID, model.TeamAdminRoleID)
	th.AddPermissionToRole(model.PermissionCreatePrivateChannel.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionCreatePublicChannel.ID, model.TeamUserRoleID)
	th.RemovePermissionFromRole(model.PermissionCreatePrivateChannel.ID, model.TeamUserRoleID)

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
	r, err := Client.DoAPIPost("/channels", "garbage")
	require.NotNil(t, err, "expected error")
	require.Equal(t, http.StatusBadRequest, r.StatusCode, "Expected 400 Bad Request")

	// Test GroupConstrained flag
	groupConstrainedChannel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamID: team.ID, GroupConstrained: model.NewBool(true)}
	rchannel, resp = Client.CreateChannel(groupConstrainedChannel)
	CheckNoError(t, resp)

	require.Equal(t, *groupConstrainedChannel.GroupConstrained, *rchannel.GroupConstrained, "GroupConstrained flags do not match")
}

func TestUpdateChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamID: team.ID}
	private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypePrivate, TeamID: team.ID}

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

	private.Type = model.ChannelTypeOpen
	newPrivateChannel, resp = Client.UpdateChannel(private)
	CheckBadRequestStatus(t, resp)

	// Test that keeping the same type succeeds

	private.Type = model.ChannelTypePrivate
	newPrivateChannel, resp = Client.UpdateChannel(private)
	CheckNoError(t, resp)

	//Non existing channel
	channel1 := &model.Channel{DisplayName: "Test API Name for apiv4", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamID: team.ID}
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

	groupChannel, resp := Client.CreateGroupChannel([]string{user1.ID, user2.ID})
	CheckNoError(t, resp)

	groupChannel.Header = "lolololol"
	Client.Logout()
	Client.Login(user3.Email, user3.Password)
	_, resp = Client.UpdateChannel(groupChannel)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	Client.Logout()
	Client.Login(user.Email, user.Password)

	directChannel, resp := Client.CreateDirectChannel(user.ID, user1.ID)
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
	*patch.Name = model.NewID()
	*patch.DisplayName = model.NewID()
	*patch.Header = model.NewID()
	*patch.Purpose = model.NewID()

	channel, resp := Client.PatchChannel(th.BasicChannel.ID, patch)
	CheckNoError(t, resp)

	require.Equal(t, *patch.Name, channel.Name, "do not match")
	require.Equal(t, *patch.DisplayName, channel.DisplayName, "do not match")
	require.Equal(t, *patch.Header, channel.Header, "do not match")
	require.Equal(t, *patch.Purpose, channel.Purpose, "do not match")

	patch.Name = nil
	oldName := channel.Name
	channel, resp = Client.PatchChannel(th.BasicChannel.ID, patch)
	CheckNoError(t, resp)

	require.Equal(t, oldName, channel.Name, "should not have updated")

	// Test GroupConstrained flag
	patch.GroupConstrained = model.NewBool(true)
	rchannel, resp := Client.PatchChannel(th.BasicChannel.ID, patch)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	require.Equal(t, *rchannel.GroupConstrained, *patch.GroupConstrained, "GroupConstrained flags do not match")
	patch.GroupConstrained = nil

	_, resp = Client.PatchChannel("junk", patch)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.PatchChannel(model.NewID(), patch)
	CheckNotFoundStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.PatchChannel(th.BasicChannel.ID, patch)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.PatchChannel(th.BasicChannel.ID, patch)
		CheckNoError(t, resp)

		_, resp = client.PatchChannel(th.BasicPrivateChannel.ID, patch)
		CheckNoError(t, resp)
	})

	// Test updating the header of someone else's GM channel.
	user1 := th.CreateUser()
	user2 := th.CreateUser()
	user3 := th.CreateUser()

	groupChannel, resp := Client.CreateGroupChannel([]string{user1.ID, user2.ID})
	CheckNoError(t, resp)

	Client.Logout()
	Client.Login(user3.Email, user3.Password)

	channelPatch := &model.ChannelPatch{}
	channelPatch.Header = new(string)
	*channelPatch.Header = "lolololol"

	_, resp = Client.PatchChannel(groupChannel.ID, channelPatch)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	Client.Logout()
	Client.Login(user.Email, user.Password)

	directChannel, resp := Client.CreateDirectChannel(user.ID, user1.ID)
	CheckNoError(t, resp)

	Client.Logout()
	Client.Login(user3.Email, user3.Password)
	_, resp = Client.PatchChannel(directChannel.ID, channelPatch)
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
			Type:        model.ChannelTypeOpen,
			TeamID:      team.ID}

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
			Type:        model.ChannelTypeOpen,
			TeamID:      team.ID,
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

		channel, resp := Client.PatchChannel(th.BasicChannel.ID, patch)
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

	dm, resp := Client.CreateDirectChannel(user1.ID, user2.ID)
	CheckNoError(t, resp)

	channelName := ""
	if user2.ID > user1.ID {
		channelName = user1.ID + "__" + user2.ID
	} else {
		channelName = user2.ID + "__" + user1.ID
	}

	require.Equal(t, channelName, dm.Name, "dm name didn't match")

	_, resp = Client.CreateDirectChannel("junk", user2.ID)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(user1.ID, model.NewID())
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(model.NewID(), user1.ID)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(model.NewID(), user2.ID)
	CheckForbiddenStatus(t, resp)

	r, err := Client.DoAPIPost("/channels/direct", "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	_, resp = th.SystemAdminClient.CreateDirectChannel(user3.ID, user2.ID)
	CheckNoError(t, resp)

	// Normal client should not be allowed to create a direct channel if users are
	// restricted to messaging members of their own team
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.RestrictDirectMessage = model.DirectMessageTeam
	})
	user4 := th.CreateUser()
	_, resp = th.Client.CreateDirectChannel(user1.ID, user4.ID)
	CheckForbiddenStatus(t, resp)
	th.LinkUserToTeam(user4, th.BasicTeam)
	_, resp = th.Client.CreateDirectChannel(user1.ID, user4.ID)
	CheckNoError(t, resp)

	Client.Logout()
	_, resp = Client.CreateDirectChannel(model.NewID(), user2.ID)
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

	_, resp := Client.Login(guest.Username, "Password1")
	CheckNoError(t, resp)

	t.Run("Try to created DM with not visible user", func(t *testing.T) {
		_, resp := Client.CreateDirectChannel(guest.ID, user1.ID)
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateDirectChannel(user1.ID, guest.ID)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Creating DM with visible user", func(t *testing.T) {
		th.LinkUserToTeam(guest, th.BasicTeam)
		th.AddUserToChannel(guest, th.BasicChannel)

		_, resp := Client.CreateDirectChannel(guest.ID, user1.ID)
		CheckNoError(t, resp)
	})
}

func TestDeleteDirectChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2

	rgc, resp := Client.CreateDirectChannel(user.ID, user2.ID)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.NotNil(t, rgc, "should have created a direct channel")

	deleted, resp := Client.DeleteChannel(rgc.ID)
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

	userIDs := []string{user.ID, user2.ID, user3.ID}

	rgc, resp := Client.CreateGroupChannel(userIDs)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	require.NotNil(t, rgc, "should have created a group channel")
	require.Equal(t, model.ChannelTypeGroup, rgc.Type, "should have created a channel of group type")

	m, _ := th.App.GetChannelMembersPage(rgc.ID, 0, 10)
	require.Len(t, *m, 3, "should have 3 channel members")

	// saving duplicate group channel
	rgc2, resp := Client.CreateGroupChannel([]string{user3.ID, user2.ID})
	CheckNoError(t, resp)
	require.Equal(t, rgc.ID, rgc2.ID, "should have returned existing channel")

	m2, _ := th.App.GetChannelMembersPage(rgc2.ID, 0, 10)
	require.Equal(t, m, m2)

	_, resp = Client.CreateGroupChannel([]string{user2.ID})
	CheckBadRequestStatus(t, resp)

	user4 := th.CreateUser()
	user5 := th.CreateUser()
	user6 := th.CreateUser()
	user7 := th.CreateUser()
	user8 := th.CreateUser()
	user9 := th.CreateUser()

	rgc, resp = Client.CreateGroupChannel([]string{user.ID, user2.ID, user3.ID, user4.ID, user5.ID, user6.ID, user7.ID, user8.ID, user9.ID})
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rgc)

	_, resp = Client.CreateGroupChannel([]string{user.ID, user2.ID, user3.ID, GenerateTestID()})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateGroupChannel([]string{user.ID, user2.ID, user3.ID, "junk"})
	CheckBadRequestStatus(t, resp)

	Client.Logout()

	_, resp = Client.CreateGroupChannel(userIDs)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.CreateGroupChannel(userIDs)
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

	_, resp := Client.Login(guest.Username, "Password1")
	CheckNoError(t, resp)

	t.Run("Try to created GM with not visible users", func(t *testing.T) {
		_, resp := Client.CreateGroupChannel([]string{guest.ID, user1.ID, user2.ID, user3.ID})
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateGroupChannel([]string{user1.ID, user2.ID, guest.ID, user3.ID})
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Try to created GM with visible and not visible users", func(t *testing.T) {
		th.LinkUserToTeam(guest, th.BasicTeam)
		th.AddUserToChannel(guest, th.BasicChannel)

		_, resp := Client.CreateGroupChannel([]string{guest.ID, user1.ID, user3.ID, user4.ID, user5.ID})
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateGroupChannel([]string{user1.ID, user2.ID, guest.ID, user4.ID, user5.ID})
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Creating GM with visible users", func(t *testing.T) {
		_, resp := Client.CreateGroupChannel([]string{guest.ID, user1.ID, user2.ID, user3.ID})
		CheckNoError(t, resp)
	})
}

func TestDeleteGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()

	userIDs := []string{user.ID, user2.ID, user3.ID}

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		rgc, resp := th.Client.CreateGroupChannel(userIDs)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, rgc, "should have created a group channel")
		deleted, resp := client.DeleteChannel(rgc.ID)
		CheckErrorMessage(t, resp, "api.channel.delete_channel.type.invalid")
		require.False(t, deleted, "should not have been able to delete group channel.")
	})

}

func TestGetChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channel, resp := Client.GetChannel(th.BasicChannel.ID, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.ID, channel.ID, "ids did not match")

	Client.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser.ID)
	_, resp = Client.GetChannel(th.BasicChannel.ID, "")
	CheckNoError(t, resp)

	channel, resp = Client.GetChannel(th.BasicPrivateChannel.ID, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicPrivateChannel.ID, channel.ID, "ids did not match")

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.ID, th.BasicUser.ID)
	_, resp = Client.GetChannel(th.BasicPrivateChannel.ID, "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannel(model.NewID(), "")
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannel(th.BasicChannel.ID, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannel(th.BasicChannel.ID, "")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.GetChannel(th.BasicChannel.ID, "")
		CheckNoError(t, resp)

		_, resp = client.GetChannel(th.BasicPrivateChannel.ID, "")
		CheckNoError(t, resp)

		_, resp = client.GetChannel(th.BasicUser.ID, "")
		CheckNotFoundStatus(t, resp)
	})
}

func TestGetDeletedChannelsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	team := th.BasicTeam

	th.LoginTeamAdmin()

	channels, resp := Client.GetDeletedChannelsForTeam(team.ID, 0, 100, "")
	CheckNoError(t, resp)
	numInitialChannelsForTeam := len(channels)

	// create and delete public channel
	publicChannel1 := th.CreatePublicChannel()
	Client.DeleteChannel(publicChannel1.ID)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		channels, resp = client.GetDeletedChannelsForTeam(team.ID, 0, 100, "")
		CheckNoError(t, resp)
		require.Len(t, channels, numInitialChannelsForTeam+1, "should be 1 deleted channel")
	})

	publicChannel2 := th.CreatePublicChannel()
	Client.DeleteChannel(publicChannel2.ID)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		channels, resp = client.GetDeletedChannelsForTeam(team.ID, 0, 100, "")
		CheckNoError(t, resp)
		require.Len(t, channels, numInitialChannelsForTeam+2, "should be 2 deleted channels")
	})

	th.LoginBasic()

	privateChannel1 := th.CreatePrivateChannel()
	Client.DeleteChannel(privateChannel1.ID)

	channels, resp = Client.GetDeletedChannelsForTeam(team.ID, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, numInitialChannelsForTeam+3)

	// Login as different user and create private channel
	th.LoginBasic2()
	privateChannel2 := th.CreatePrivateChannel()
	Client.DeleteChannel(privateChannel2.ID)

	// Log back in as first user
	th.LoginBasic()

	channels, resp = Client.GetDeletedChannelsForTeam(team.ID, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, numInitialChannelsForTeam+3)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		channels, resp = client.GetDeletedChannelsForTeam(team.ID, 0, 100, "")
		CheckNoError(t, resp)
		require.Len(t, channels, numInitialChannelsForTeam+2)
	})

	channels, resp = Client.GetDeletedChannelsForTeam(team.ID, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetDeletedChannelsForTeam(team.ID, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")
}

func TestGetPrivateChannelsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	team := th.BasicTeam

	// normal user
	_, resp := th.Client.GetPrivateChannelsForTeam(team.ID, 0, 100, "")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		channels, resp := c.GetPrivateChannelsForTeam(team.ID, 0, 100, "")
		CheckNoError(t, resp)
		// th.BasicPrivateChannel and th.BasicPrivateChannel2
		require.Len(t, channels, 2, "wrong number of private channels")
		for _, c := range channels {
			// check all channels included are private
			require.Equal(t, model.ChannelTypePrivate, c.Type, "should include private channels only")
		}

		channels, resp = c.GetPrivateChannelsForTeam(team.ID, 0, 1, "")
		CheckNoError(t, resp)
		require.Len(t, channels, 1, "should be one channel per page")

		channels, resp = c.GetPrivateChannelsForTeam(team.ID, 1, 1, "")
		CheckNoError(t, resp)
		require.Len(t, channels, 1, "should be one channel per page")

		channels, resp = c.GetPrivateChannelsForTeam(team.ID, 10000, 100, "")
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

	channels, resp := Client.GetPublicChannelsForTeam(team.ID, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 4, "wrong path")

	for i, c := range channels {
		// check all channels included are open
		require.Equal(t, model.ChannelTypeOpen, c.Type, "should include open channel only")

		// only check the created 2 public channels
		require.False(t, i < 2 && !(c.DisplayName == publicChannel1.DisplayName || c.DisplayName == publicChannel2.DisplayName), "should match public channel display name")
	}

	privateChannel := th.CreatePrivateChannel()
	channels, resp = Client.GetPublicChannelsForTeam(team.ID, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 4, "incorrect length of team public channels")

	for _, c := range channels {
		require.Equal(t, model.ChannelTypeOpen, c.Type, "should not include private channel")
		require.NotEqual(t, privateChannel.DisplayName, c.DisplayName, "should not match private channel display name")
	}

	channels, resp = Client.GetPublicChannelsForTeam(team.ID, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetPublicChannelsForTeam(team.ID, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetPublicChannelsForTeam(team.ID, 10000, 100, "")
	CheckNoError(t, resp)
	require.Empty(t, channels, "should be no channel")

	_, resp = Client.GetPublicChannelsForTeam("junk", 0, 100, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsForTeam(model.NewID(), 0, 100, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPublicChannelsForTeam(team.ID, 0, 100, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetPublicChannelsForTeam(team.ID, 0, 100, "")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.GetPublicChannelsForTeam(team.ID, 0, 100, "")
		CheckNoError(t, resp)
	})
}

func TestGetPublicChannelsByIDsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	teamID := th.BasicTeam.ID
	input := []string{th.BasicChannel.ID}
	output := []string{th.BasicChannel.DisplayName}

	channels, resp := Client.GetPublicChannelsByIDsForTeam(teamID, input)
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should return 1 channel")
	require.Equal(t, output[0], channels[0].DisplayName, "missing channel")

	input = append(input, GenerateTestID())
	input = append(input, th.BasicChannel2.ID)
	input = append(input, th.BasicPrivateChannel.ID)
	output = append(output, th.BasicChannel2.DisplayName)
	sort.Strings(output)

	channels, resp = Client.GetPublicChannelsByIDsForTeam(teamID, input)
	CheckNoError(t, resp)
	require.Len(t, channels, 2, "should return 2 channels")

	for i, c := range channels {
		require.Equal(t, output[i], c.DisplayName, "missing channel")
	}

	_, resp = Client.GetPublicChannelsByIDsForTeam(GenerateTestID(), input)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIDsForTeam(teamID, []string{})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIDsForTeam(teamID, []string{"junk"})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIDsForTeam(teamID, []string{GenerateTestID()})
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIDsForTeam(teamID, []string{th.BasicPrivateChannel.ID})
	CheckNotFoundStatus(t, resp)

	Client.Logout()

	_, resp = Client.GetPublicChannelsByIDsForTeam(teamID, input)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPublicChannelsByIDsForTeam(teamID, input)
	CheckNoError(t, resp)
}

func TestGetChannelsForTeamForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	t.Run("get channels for the team for user", func(t *testing.T) {
		channels, resp := Client.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser.ID, false, "")
		CheckNoError(t, resp)

		found := make([]bool, 3)
		for _, c := range channels {
			if c.ID == th.BasicChannel.ID {
				found[0] = true
			} else if c.ID == th.BasicChannel2.ID {
				found[1] = true
			} else if c.ID == th.BasicPrivateChannel.ID {
				found[2] = true
			}

			require.True(t, c.TeamID == "" || c.TeamID == th.BasicTeam.ID)
		}

		for _, f := range found {
			require.True(t, f, "missing a channel")
		}

		channels, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser.ID, false, resp.Etag)
		CheckEtag(t, channels, resp)

		_, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.ID, "junk", false, "")
		CheckBadRequestStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser("junk", th.BasicUser.ID, false, "")
		CheckBadRequestStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser2.ID, false, "")
		CheckForbiddenStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser(model.NewID(), th.BasicUser.ID, false, "")
		CheckForbiddenStatus(t, resp)

		_, resp = th.SystemAdminClient.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser.ID, false, "")
		CheckNoError(t, resp)
	})

	t.Run("deleted channel could be retrieved using the proper flag", func(t *testing.T) {
		testChannel := &model.Channel{
			DisplayName: "dn_" + model.NewID(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamID:      th.BasicTeam.ID,
			CreatorID:   th.BasicUser.ID,
		}
		th.App.CreateChannel(th.Context, testChannel, true)
		defer th.App.PermanentDeleteChannel(testChannel)
		channels, resp := Client.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser.ID, false, "")
		CheckNoError(t, resp)
		assert.Equal(t, 6, len(channels))
		th.App.DeleteChannel(th.Context, testChannel, th.BasicUser.ID)
		channels, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser.ID, false, "")
		CheckNoError(t, resp)
		assert.Equal(t, 5, len(channels))

		// Should return all channels including basicDeleted.
		channels, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser.ID, true, "")
		CheckNoError(t, resp)
		assert.Equal(t, 7, len(channels))

		// Should stil return all channels including basicDeleted.
		now := time.Now().Add(-time.Minute).Unix() * 1000
		Client.GetChannelsForTeamAndUserWithLastDeleteAt(th.BasicTeam.ID, th.BasicUser.ID,
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
			require.NotEqual(t, c.TeamID, "")
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

		ok, resp := client.DeleteChannel(firstChannel.ID)
		require.Nil(t, resp.Error)
		require.True(t, ok)

		channels, resp = client.GetAllChannels(0, 10000, "")
		var ids []string
		for _, item := range *channels {
			ids = append(ids, item.Channel.ID)
		}
		require.Nil(t, resp.Error)
		require.Len(t, *channels, beforeCount-1)
		require.NotContains(t, ids, firstChannel.ID)

		channels, resp = client.GetAllChannelsIncludeDeleted(0, 10000, "")
		ids = []string{}
		for _, item := range *channels {
			ids = append(ids, item.Channel.ID)
		}
		require.Nil(t, resp.Error)
		require.True(t, len(*channels) > beforeCount)
		require.Contains(t, ids, firstChannel.ID)
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
		ChannelIDs: []string{policyChannel.ID},
	})
	require.NoError(t, savePolicyErr)

	t.Run("exclude policy constrained", func(t *testing.T) {
		_, resp := th.SystemManagerClient.GetAllChannelsExcludePolicyConstrained(0, 10000, "")
		CheckForbiddenStatus(t, resp)

		channels, resp := th.SystemAdminClient.GetAllChannelsExcludePolicyConstrained(0, 10000, "")
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range *channels {
			if channel.ID == policyChannel.ID {
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
			if channel.ID == policyChannel.ID {
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
			if channel.ID == policyChannel.ID {
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
		require.NotEqual(t, c.TeamID, "")
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

	channels, resp := Client.SearchChannels(th.BasicTeam.ID, search)
	CheckNoError(t, resp)

	found := false
	for _, c := range channels {
		require.Equal(t, model.ChannelTypeOpen, c.Type, "should only return public channels")

		if c.ID == th.BasicChannel.ID {
			found = true
		}
	}
	require.True(t, found, "didn't find channel")

	search.Term = th.BasicPrivateChannel.Name
	channels, resp = Client.SearchChannels(th.BasicTeam.ID, search)
	CheckNoError(t, resp)

	found = false
	for _, c := range channels {
		if c.ID == th.BasicPrivateChannel.ID {
			found = true
		}
	}
	require.False(t, found, "shouldn't find private channel")

	search.Term = ""
	_, resp = Client.SearchChannels(th.BasicTeam.ID, search)
	CheckNoError(t, resp)

	search.Term = th.BasicChannel.Name
	_, resp = Client.SearchChannels(model.NewID(), search)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.SearchChannels("junk", search)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.SearchChannels(th.BasicTeam.ID, search)
	CheckNoError(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// Remove list channels permission from the user
	th.RemovePermissionFromRole(model.PermissionListTeamChannels.ID, model.TeamUserRoleID)

	t.Run("Search for a BasicChannel, which the user is a member of", func(t *testing.T) {
		search.Term = th.BasicChannel.Name
		channelList, resp := Client.SearchChannels(th.BasicTeam.ID, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicChannel.Name)
	})

	t.Run("Remove the user from BasicChannel and search again, should not be returned", func(t *testing.T) {
		th.App.RemoveUserFromChannel(th.Context, th.BasicUser.ID, th.BasicUser.ID, th.BasicChannel)

		search.Term = th.BasicChannel.Name
		channelList, resp := Client.SearchChannels(th.BasicTeam.ID, search)
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

	Client.DeleteChannel(th.BasicChannel.ID)

	channels, resp := Client.SearchArchivedChannels(th.BasicTeam.ID, search)
	CheckNoError(t, resp)

	found := false
	for _, c := range channels {
		require.Equal(t, model.ChannelTypeOpen, c.Type)

		if c.ID == th.BasicChannel.ID {
			found = true
		}
	}

	require.True(t, found)

	search.Term = th.BasicPrivateChannel.Name
	Client.DeleteChannel(th.BasicPrivateChannel.ID)

	channels, resp = Client.SearchArchivedChannels(th.BasicTeam.ID, search)
	CheckNoError(t, resp)

	found = false
	for _, c := range channels {
		if c.ID == th.BasicPrivateChannel.ID {
			found = true
		}
	}

	require.True(t, found)

	search.Term = ""
	_, resp = Client.SearchArchivedChannels(th.BasicTeam.ID, search)
	CheckNoError(t, resp)

	search.Term = th.BasicDeletedChannel.Name
	_, resp = Client.SearchArchivedChannels(model.NewID(), search)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.SearchArchivedChannels("junk", search)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.SearchArchivedChannels(th.BasicTeam.ID, search)
	CheckNoError(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// Remove list channels permission from the user
	th.RemovePermissionFromRole(model.PermissionListTeamChannels.ID, model.TeamUserRoleID)

	t.Run("Search for a BasicDeletedChannel, which the user is a member of", func(t *testing.T) {
		search.Term = th.BasicDeletedChannel.Name
		channelList, resp := Client.SearchArchivedChannels(th.BasicTeam.ID, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicDeletedChannel.Name)
	})

	t.Run("Remove the user from BasicDeletedChannel and search again, should still return", func(t *testing.T) {
		th.App.RemoveUserFromChannel(th.Context, th.BasicUser.ID, th.BasicUser.ID, th.BasicDeletedChannel)

		search.Term = th.BasicDeletedChannel.Name
		channelList, resp := Client.SearchArchivedChannels(th.BasicTeam.ID, search)
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
		Type:        model.ChannelTypeOpen,
		TeamID:      th.BasicTeam.ID,
	})
	CheckNoError(t, chanErr)

	privateChannel, privErr := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "SearchAllChannels-private1",
		Name:        "private1",
		Type:        model.ChannelTypePrivate,
		TeamID:      th.BasicTeam.ID,
	})
	CheckNoError(t, privErr)

	team := th.CreateTeam()
	groupConstrainedChannel, groupErr := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName:      "SearchAllChannels-groupConstrained-1",
		Name:             "groupconstrained1",
		Type:             model.ChannelTypePrivate,
		GroupConstrained: model.NewBool(true),
		TeamID:           team.ID,
	})
	CheckNoError(t, groupErr)

	testCases := []struct {
		Description        string
		Search             *model.ChannelSearch
		ExpectedChannelIDs []string
	}{
		{
			"Middle of word search",
			&model.ChannelSearch{Term: "bardisplay"},
			[]string{openChannel.ID},
		},
		{
			"Prefix search",
			&model.ChannelSearch{Term: "SearchAllChannels-foobar"},
			[]string{openChannel.ID},
		},
		{
			"Suffix search",
			&model.ChannelSearch{Term: "displayname"},
			[]string{openChannel.ID},
		},
		{
			"Name search",
			&model.ChannelSearch{Term: "what"},
			[]string{openChannel.ID},
		},
		{
			"Name suffix search",
			&model.ChannelSearch{Term: "ever"},
			[]string{openChannel.ID},
		},
		{
			"Basic channel name middle of word search",
			&model.ChannelSearch{Term: th.BasicChannel.Name[2:14]},
			[]string{th.BasicChannel.ID},
		},
		{
			"Upper case search",
			&model.ChannelSearch{Term: strings.ToUpper(th.BasicChannel.Name)},
			[]string{th.BasicChannel.ID},
		},
		{
			"Mixed case search",
			&model.ChannelSearch{Term: th.BasicChannel.Name[0:2] + strings.ToUpper(th.BasicChannel.Name[2:5]) + th.BasicChannel.Name[5:]},
			[]string{th.BasicChannel.ID},
		},
		{
			"Non mixed case search",
			&model.ChannelSearch{Term: th.BasicChannel.Name},
			[]string{th.BasicChannel.ID},
		},
		{
			"Search private channel name",
			&model.ChannelSearch{Term: th.BasicPrivateChannel.Name},
			[]string{th.BasicPrivateChannel.ID},
		},
		{
			"Search with private channel filter",
			&model.ChannelSearch{Private: true},
			[]string{th.BasicPrivateChannel.ID, th.BasicPrivateChannel2.ID, privateChannel.ID, groupConstrainedChannel.ID},
		},
		{
			"Search with public channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", Public: true},
			[]string{openChannel.ID},
		},
		{
			"Search with private channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", Private: true},
			[]string{privateChannel.ID, groupConstrainedChannel.ID},
		},
		{
			"Search with teamIds channel filter",
			&model.ChannelSearch{Term: "SearchAllChannels", TeamIDs: []string{th.BasicTeam.ID}},
			[]string{openChannel.ID, privateChannel.ID},
		},
		{
			"Search with deleted without IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name},
			[]string{},
		},
		{
			"Search with deleted IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name, IncludeDeleted: true},
			[]string{th.BasicDeletedChannel.ID},
		},
		{
			"Search with deleted IncludeDeleted filter",
			&model.ChannelSearch{Term: th.BasicDeletedChannel.Name, IncludeDeleted: true},
			[]string{th.BasicDeletedChannel.ID},
		},
		{
			"Search with deleted Deleted filter and empty term",
			&model.ChannelSearch{Term: "", Deleted: true},
			[]string{th.BasicDeletedChannel.ID},
		},
		{
			"Search for group constrained",
			&model.ChannelSearch{Term: "SearchAllChannels", GroupConstrained: true},
			[]string{groupConstrainedChannel.ID},
		},
		{
			"Search for group constrained and public",
			&model.ChannelSearch{Term: "SearchAllChannels", GroupConstrained: true, Public: true},
			[]string{},
		},
		{
			"Search for exclude group constrained",
			&model.ChannelSearch{Term: "SearchAllChannels", ExcludeGroupConstrained: true},
			[]string{openChannel.ID, privateChannel.ID},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			channels, resp := th.SystemAdminClient.SearchAllChannels(testCase.Search)
			CheckNoError(t, resp)
			assert.Equal(t, len(testCase.ExpectedChannelIDs), len(*channels))
			actualChannelIDs := []string{}
			for _, channelWithTeamData := range *channels {
				actualChannelIDs = append(actualChannelIDs, channelWithTeamData.Channel.ID)
			}
			assert.ElementsMatch(t, testCase.ExpectedChannelIDs, actualChannelIDs)
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
		ChannelIDs: []string{policyChannel.ID},
	})
	require.NoError(t, savePolicyErr)

	t.Run("does not return policy ID", func(t *testing.T) {
		channels, resp := th.SystemManagerClient.SearchAllChannels(&model.ChannelSearch{Term: policyChannel.Name})
		CheckOKStatus(t, resp)
		found := false
		for _, channel := range *channels {
			if channel.ID == policyChannel.ID {
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
			if channel.ID == policyChannel.ID {
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
	gc1, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.ID, th.BasicUser2.ID, u1.ID})
	CheckNoError(t, resp)
	defer th.Client.DeleteChannel(gc1.ID)

	gc2, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.ID, th.BasicUser2.ID, th.SystemAdminUser.ID})
	CheckNoError(t, resp)
	defer th.Client.DeleteChannel(gc2.ID)

	search := &model.ChannelSearch{Term: th.BasicUser2.Username}

	// sysadmin should only find gc2 as he doesn't belong to gc1
	channels, resp := th.SystemAdminClient.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 1)
	assert.Equal(t, channels[0].ID, gc2.ID)

	// basic user should find both
	Client.Login(th.BasicUser.Username, th.BasicUser.Password)
	channels, resp = Client.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 2)
	channelIDs := []string{}
	for _, c := range channels {
		channelIDs = append(channelIDs, c.ID)
	}
	assert.ElementsMatch(t, channelIDs, []string{gc1.ID, gc2.ID})

	// searching for sysadmin, it should only find gc1
	search = &model.ChannelSearch{Term: th.SystemAdminUser.Username}
	channels, resp = Client.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 1)
	assert.Equal(t, channels[0].ID, gc2.ID)

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
		pass, resp := client.DeleteChannel(publicChannel1.ID)
		CheckNoError(t, resp)

		require.True(t, pass, "should have passed")

		ch, err := th.App.GetChannel(publicChannel1.ID)
		require.True(t, err != nil || ch.DeleteAt != 0, "should have failed to get deleted channel, or returned one with a populated DeleteAt.")

		post1 := &model.Post{ChannelID: publicChannel1.ID, Message: "a" + GenerateTestID() + "a"}
		_, resp = client.CreatePost(post1)
		require.NotNil(t, resp, "expected response to not be nil")

		// successful delete of private channel
		privateChannel2 := th.CreatePrivateChannel()
		_, resp = client.DeleteChannel(privateChannel2.ID)
		CheckNoError(t, resp)

		// successful delete of channel with multiple members
		publicChannel3 := th.CreatePublicChannel()
		th.App.AddUserToChannel(user, publicChannel3, false)
		th.App.AddUserToChannel(user2, publicChannel3, false)
		_, resp = client.DeleteChannel(publicChannel3.ID)
		CheckNoError(t, resp)

		// default channel cannot be deleted.
		defaultChannel, _ := th.App.GetChannelByName(model.DefaultChannelName, team.ID, false)
		pass, resp = client.DeleteChannel(defaultChannel.ID)
		CheckBadRequestStatus(t, resp)
		require.False(t, pass, "should have failed")

		// check system admin can delete a channel without any appropriate team or channel membership.
		sdTeam := th.CreateTeamWithClient(c)
		sdPublicChannel := &model.Channel{
			DisplayName: "dn_" + model.NewID(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypeOpen,
			TeamID:      sdTeam.ID,
		}
		sdPublicChannel, resp = c.CreateChannel(sdPublicChannel)
		CheckNoError(t, resp)
		_, resp = client.DeleteChannel(sdPublicChannel.ID)
		CheckNoError(t, resp)

		sdPrivateChannel := &model.Channel{
			DisplayName: "dn_" + model.NewID(),
			Name:        GenerateTestChannelName(),
			Type:        model.ChannelTypePrivate,
			TeamID:      sdTeam.ID,
		}
		sdPrivateChannel, resp = c.CreateChannel(sdPrivateChannel)
		CheckNoError(t, resp)
		_, resp = client.DeleteChannel(sdPrivateChannel.ID)
		CheckNoError(t, resp)
	})
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {

		th.LoginBasic()
		publicChannel5 := th.CreatePublicChannel()
		c.Logout()

		c.Login(user.ID, user.Password)
		_, resp := c.DeleteChannel(publicChannel5.ID)
		CheckUnauthorizedStatus(t, resp)

		_, resp = c.DeleteChannel("junk")
		CheckUnauthorizedStatus(t, resp)

		c.Logout()
		_, resp = c.DeleteChannel(GenerateTestID())
		CheckUnauthorizedStatus(t, resp)

		_, resp = client.DeleteChannel(publicChannel5.ID)
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

	th.AddPermissionToRole(model.PermissionDeletePublicChannel.ID, model.ChannelUserRoleID)
	th.AddPermissionToRole(model.PermissionDeletePrivateChannel.ID, model.ChannelUserRoleID)

	// channels created by SystemAdmin
	publicChannel6 := th.CreateChannelWithClient(th.SystemAdminClient, model.ChannelTypeOpen)
	privateChannel7 := th.CreateChannelWithClient(th.SystemAdminClient, model.ChannelTypePrivate)
	th.App.AddUserToChannel(user, publicChannel6, false)
	th.App.AddUserToChannel(user, privateChannel7, false)
	th.App.AddUserToChannel(user, privateChannel7, false)

	// successful delete by user
	_, resp := Client.DeleteChannel(publicChannel6.ID)
	CheckNoError(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.ID)
	CheckNoError(t, resp)

	// Restrict permissions to Channel Admins
	th.RemovePermissionFromRole(model.PermissionDeletePublicChannel.ID, model.ChannelUserRoleID)
	th.RemovePermissionFromRole(model.PermissionDeletePrivateChannel.ID, model.ChannelUserRoleID)
	th.AddPermissionToRole(model.PermissionDeletePublicChannel.ID, model.ChannelAdminRoleID)
	th.AddPermissionToRole(model.PermissionDeletePrivateChannel.ID, model.ChannelAdminRoleID)

	// channels created by SystemAdmin
	publicChannel6 = th.CreateChannelWithClient(th.SystemAdminClient, model.ChannelTypeOpen)
	privateChannel7 = th.CreateChannelWithClient(th.SystemAdminClient, model.ChannelTypePrivate)
	th.App.AddUserToChannel(user, publicChannel6, false)
	th.App.AddUserToChannel(user, privateChannel7, false)
	th.App.AddUserToChannel(user, privateChannel7, false)

	// cannot delete by user
	_, resp = Client.DeleteChannel(publicChannel6.ID)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.ID)
	CheckForbiddenStatus(t, resp)

	// successful delete by channel admin
	th.MakeUserChannelAdmin(user, publicChannel6)
	th.MakeUserChannelAdmin(user, privateChannel7)
	th.App.Srv().Store.Channel().ClearCaches()

	_, resp = Client.DeleteChannel(publicChannel6.ID)
	CheckNoError(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.ID)
	CheckNoError(t, resp)

	// Make sure team admins don't have permission to delete channels.
	th.RemovePermissionFromRole(model.PermissionDeletePublicChannel.ID, model.ChannelAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionDeletePrivateChannel.ID, model.ChannelAdminRoleID)

	// last member of a public channel should have required permission to delete
	publicChannel6 = th.CreateChannelWithClient(th.Client, model.ChannelTypeOpen)
	_, resp = Client.DeleteChannel(publicChannel6.ID)
	CheckForbiddenStatus(t, resp)

	// last member of a private channel should not be able to delete it if they don't have required permissions
	privateChannel7 = th.CreateChannelWithClient(th.Client, model.ChannelTypePrivate)
	_, resp = Client.DeleteChannel(privateChannel7.ID)
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
		_, resp := th.SystemAdminClient.PermanentDeleteChannel(publicChannel1.ID)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("Permanent deletion available through local mode even if EnableAPIChannelDeletion is not set", func(t *testing.T) {
		ok, resp := th.LocalClient.PermanentDeleteChannel(publicChannel1.ID)
		CheckNoError(t, resp)
		assert.True(t, ok)
	})

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPIChannelDeletion = true })
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		publicChannel := th.CreatePublicChannel()
		ok, resp := c.PermanentDeleteChannel(publicChannel.ID)
		CheckNoError(t, resp)
		assert.True(t, ok)

		_, err := th.App.GetChannel(publicChannel.ID)
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

	defaultChannel, _ := th.App.GetChannelByName(model.DefaultChannelName, th.BasicTeam.ID, false)
	_, resp := Client.ConvertChannelToPrivate(defaultChannel.ID)
	CheckForbiddenStatus(t, resp)

	privateChannel := th.CreatePrivateChannel()
	_, resp = Client.ConvertChannelToPrivate(privateChannel.ID)
	CheckForbiddenStatus(t, resp)

	publicChannel := th.CreatePublicChannel()
	_, resp = Client.ConvertChannelToPrivate(publicChannel.ID)
	CheckForbiddenStatus(t, resp)

	th.LoginTeamAdmin()
	th.RemovePermissionFromRole(model.PermissionConvertPublicChannelToPrivate.ID, model.TeamAdminRoleID)

	_, resp = Client.ConvertChannelToPrivate(publicChannel.ID)
	CheckForbiddenStatus(t, resp)

	th.AddPermissionToRole(model.PermissionConvertPublicChannelToPrivate.ID, model.TeamAdminRoleID)

	rchannel, resp := Client.ConvertChannelToPrivate(publicChannel.ID)
	CheckOKStatus(t, resp)
	require.Equal(t, model.ChannelTypePrivate, rchannel.Type, "channel should be converted from public to private")

	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(privateChannel.ID)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rchannel, "should not return a channel")

	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(defaultChannel.ID)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rchannel, "should not return a channel")

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	WebSocketClient.Listen()

	publicChannel2 := th.CreatePublicChannel()
	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(publicChannel2.ID)
	CheckOKStatus(t, resp)
	require.Equal(t, model.ChannelTypePrivate, rchannel.Type, "channel should be converted from public to private")

	timeout := time.After(10 * time.Second)

	for {
		select {
		case resp := <-WebSocketClient.EventChannel:
			if resp.EventType() == model.WebsocketEventChannelConverted && resp.GetData()["channel_id"].(string) == publicChannel2.ID {
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

	defaultChannel, _ := th.App.GetChannelByName(model.DefaultChannelName, th.BasicTeam.ID, false)

	type testTable []struct {
		name            string
		channel         *model.Channel
		expectedPrivacy string
	}

	t.Run("Should get a forbidden response if not logged in", func(t *testing.T) {
		privateChannel := th.CreatePrivateChannel()
		publicChannel := th.CreatePublicChannel()

		tt := testTable{
			{"Updating default channel should fail with forbidden status if not logged in", defaultChannel, model.ChannelTypeOpen},
			{"Updating private channel should fail with forbidden status if not logged in", privateChannel, model.ChannelTypePrivate},
			{"Updating public channel should fail with forbidden status if not logged in", publicChannel, model.ChannelTypeOpen},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				_, resp := th.Client.UpdateChannelPrivacy(tc.channel.ID, tc.expectedPrivacy)
				CheckForbiddenStatus(t, resp)
			})
		}
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel()
		publicChannel := th.CreatePublicChannel()

		tt := testTable{
			{"Converting default channel to private should fail", defaultChannel, model.ChannelTypePrivate},
			{"Updating privacy to an invalid setting should fail", publicChannel, "invalid"},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				_, resp := client.UpdateChannelPrivacy(tc.channel.ID, tc.expectedPrivacy)
				CheckBadRequestStatus(t, resp)
			})
		}

		tt = testTable{
			{"Default channel should stay public", defaultChannel, model.ChannelTypeOpen},
			{"Public channel should stay public", publicChannel, model.ChannelTypeOpen},
			{"Private channel should stay private", privateChannel, model.ChannelTypePrivate},
			{"Public channel should convert to private", publicChannel, model.ChannelTypePrivate},
			{"Private channel should convert to public", privateChannel, model.ChannelTypeOpen},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				updatedChannel, resp := client.UpdateChannelPrivacy(tc.channel.ID, tc.expectedPrivacy)
				CheckNoError(t, resp)
				assert.Equal(t, tc.expectedPrivacy, updatedChannel.Type)
				updatedChannel, err := th.App.GetChannel(tc.channel.ID)
				require.Nil(t, err)
				assert.Equal(t, tc.expectedPrivacy, updatedChannel.Type)
			})
		}
	})

	t.Run("Enforces convert channel permissions", func(t *testing.T) {
		privateChannel := th.CreatePrivateChannel()
		publicChannel := th.CreatePublicChannel()

		th.LoginTeamAdmin()

		th.RemovePermissionFromRole(model.PermissionConvertPublicChannelToPrivate.ID, model.TeamAdminRoleID)
		th.RemovePermissionFromRole(model.PermissionConvertPrivateChannelToPublic.ID, model.TeamAdminRoleID)

		_, resp := th.Client.UpdateChannelPrivacy(publicChannel.ID, model.ChannelTypePrivate)
		CheckForbiddenStatus(t, resp)
		_, resp = th.Client.UpdateChannelPrivacy(privateChannel.ID, model.ChannelTypeOpen)
		CheckForbiddenStatus(t, resp)

		th.AddPermissionToRole(model.PermissionConvertPublicChannelToPrivate.ID, model.TeamAdminRoleID)
		th.AddPermissionToRole(model.PermissionConvertPrivateChannelToPublic.ID, model.TeamAdminRoleID)

		_, resp = th.Client.UpdateChannelPrivacy(privateChannel.ID, model.ChannelTypeOpen)
		CheckNoError(t, resp)
		_, resp = th.Client.UpdateChannelPrivacy(publicChannel.ID, model.ChannelTypePrivate)
		CheckNoError(t, resp)
	})
}

func TestRestoreChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	publicChannel1 := th.CreatePublicChannel()
	th.Client.DeleteChannel(publicChannel1.ID)

	privateChannel1 := th.CreatePrivateChannel()
	th.Client.DeleteChannel(privateChannel1.ID)

	_, resp := th.Client.RestoreChannel(publicChannel1.ID)
	CheckForbiddenStatus(t, resp)

	_, resp = th.Client.RestoreChannel(privateChannel1.ID)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		defer func() {
			client.DeleteChannel(publicChannel1.ID)
			client.DeleteChannel(privateChannel1.ID)
		}()

		_, resp = client.RestoreChannel(publicChannel1.ID)
		CheckOKStatus(t, resp)

		_, resp = client.RestoreChannel(privateChannel1.ID)
		CheckOKStatus(t, resp)
	})
}

func TestGetChannelByName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channel, resp := Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.ID, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	channel, resp = Client.GetChannelByName(th.BasicPrivateChannel.Name, th.BasicTeam.ID, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicPrivateChannel.Name, channel.Name, "names did not match")

	_, resp = Client.GetChannelByName(strings.ToUpper(th.BasicPrivateChannel.Name), th.BasicTeam.ID, "")
	CheckNoError(t, resp)

	_, resp = Client.GetChannelByName(th.BasicDeletedChannel.Name, th.BasicTeam.ID, "")
	CheckNotFoundStatus(t, resp)

	channel, resp = Client.GetChannelByNameIncludeDeleted(th.BasicDeletedChannel.Name, th.BasicTeam.ID, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicDeletedChannel.Name, channel.Name, "names did not match")

	Client.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser.ID)
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.ID, "")
	CheckNoError(t, resp)

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.ID, th.BasicUser.ID)
	_, resp = Client.GetChannelByName(th.BasicPrivateChannel.Name, th.BasicTeam.ID, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByName(GenerateTestChannelName(), th.BasicTeam.ID, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByName(GenerateTestChannelName(), "junk", "")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.ID, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.ID, "")
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.ID, "")
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

	Client.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser.ID)
	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.ID, th.BasicUser.ID)
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
		members, resp := client.GetChannelMembers(th.BasicChannel.ID, 0, 60, "")
		CheckNoError(t, resp)
		require.Len(t, *members, 3, "should only be 3 users in channel")

		members, resp = client.GetChannelMembers(th.BasicChannel.ID, 0, 2, "")
		CheckNoError(t, resp)
		require.Len(t, *members, 2, "should only be 2 users")

		members, resp = client.GetChannelMembers(th.BasicChannel.ID, 1, 1, "")
		CheckNoError(t, resp)
		require.Len(t, *members, 1, "should only be 1 user")

		members, resp = client.GetChannelMembers(th.BasicChannel.ID, 1000, 100000, "")
		CheckNoError(t, resp)
		require.Empty(t, *members, "should be 0 users")

		_, resp = client.GetChannelMembers("junk", 0, 60, "")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetChannelMembers("", 0, 60, "")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetChannelMembers(th.BasicChannel.ID, 0, 60, "")
		CheckNoError(t, resp)
	})

	_, resp := th.Client.GetChannelMembers(model.NewID(), 0, 60, "")
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.GetChannelMembers(th.BasicChannel.ID, 0, 60, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	th.Client.Login(user.Email, user.Password)
	_, resp = th.Client.GetChannelMembers(th.BasicChannel.ID, 0, 60, "")
	CheckForbiddenStatus(t, resp)
}

func TestGetChannelMembersByIDs(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	cm, resp := Client.GetChannelMembersByIDs(th.BasicChannel.ID, []string{th.BasicUser.ID})
	CheckNoError(t, resp)
	require.Equal(t, th.BasicUser.ID, (*cm)[0].UserID, "returned wrong user")

	_, resp = Client.GetChannelMembersByIDs(th.BasicChannel.ID, []string{})
	CheckBadRequestStatus(t, resp)

	cm1, resp := Client.GetChannelMembersByIDs(th.BasicChannel.ID, []string{"junk"})
	CheckNoError(t, resp)
	require.Empty(t, *cm1, "no users should be returned")

	cm1, resp = Client.GetChannelMembersByIDs(th.BasicChannel.ID, []string{"junk", th.BasicUser.ID})
	CheckNoError(t, resp)
	require.Len(t, *cm1, 1, "1 member should be returned")

	cm1, resp = Client.GetChannelMembersByIDs(th.BasicChannel.ID, []string{th.BasicUser2.ID, th.BasicUser.ID})
	CheckNoError(t, resp)
	require.Len(t, *cm1, 2, "2 members should be returned")

	_, resp = Client.GetChannelMembersByIDs("junk", []string{th.BasicUser.ID})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersByIDs(model.NewID(), []string{th.BasicUser.ID})
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelMembersByIDs(th.BasicChannel.ID, []string{th.BasicUser.ID})
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelMembersByIDs(th.BasicChannel.ID, []string{th.BasicUser2.ID, th.BasicUser.ID})
	CheckNoError(t, resp)
}

func TestGetChannelMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	c := th.Client
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		member, resp := client.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
		CheckNoError(t, resp)
		require.Equal(t, th.BasicChannel.ID, member.ChannelID, "wrong channel id")
		require.Equal(t, th.BasicUser.ID, member.UserID, "wrong user id")

		_, resp = client.GetChannelMember("", th.BasicUser.ID, "")
		CheckNotFoundStatus(t, resp)

		_, resp = client.GetChannelMember("junk", th.BasicUser.ID, "")
		CheckBadRequestStatus(t, resp)
		_, resp = client.GetChannelMember(th.BasicChannel.ID, "", "")
		CheckNotFoundStatus(t, resp)

		_, resp = client.GetChannelMember(th.BasicChannel.ID, "junk", "")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetChannelMember(th.BasicChannel.ID, model.NewID(), "")
		CheckNotFoundStatus(t, resp)

		_, resp = client.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
		CheckNoError(t, resp)
	})

	_, resp := c.GetChannelMember(model.NewID(), th.BasicUser.ID, "")
	CheckForbiddenStatus(t, resp)

	c.Logout()
	_, resp = c.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	c.Login(user.Email, user.Password)
	_, resp = c.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
	CheckForbiddenStatus(t, resp)
}

func TestGetChannelMembersForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	members, resp := Client.GetChannelMembersForUser(th.BasicUser.ID, th.BasicTeam.ID, "")
	CheckNoError(t, resp)
	require.Len(t, *members, 6, "should have 6 members on team")

	_, resp = Client.GetChannelMembersForUser("", th.BasicTeam.ID, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser("junk", th.BasicTeam.ID, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(model.NewID(), th.BasicTeam.ID, "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.ID, "", "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.ID, "junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.ID, model.NewID(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelMembersForUser(th.BasicUser.ID, th.BasicTeam.ID, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelMembersForUser(th.BasicUser.ID, th.BasicTeam.ID, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelMembersForUser(th.BasicUser.ID, th.BasicTeam.ID, "")
	CheckNoError(t, resp)
}

func TestViewChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	view := &model.ChannelView{
		ChannelID: th.BasicChannel.ID,
	}

	viewResp, resp := Client.ViewChannel(th.BasicUser.ID, view)
	CheckNoError(t, resp)
	require.Equal(t, "OK", viewResp.Status, "should have passed")

	channel, _ := th.App.GetChannel(th.BasicChannel.ID)

	require.Equal(t, channel.LastPostAt, viewResp.LastViewedAtTimes[channel.ID], "LastPostAt does not match returned LastViewedAt time")

	view.PrevChannelID = th.BasicChannel.ID
	_, resp = Client.ViewChannel(th.BasicUser.ID, view)
	CheckNoError(t, resp)

	view.PrevChannelID = ""
	_, resp = Client.ViewChannel(th.BasicUser.ID, view)
	CheckNoError(t, resp)

	view.PrevChannelID = "junk"
	_, resp = Client.ViewChannel(th.BasicUser.ID, view)
	CheckBadRequestStatus(t, resp)

	// All blank is OK we use it for clicking off of the browser.
	view.PrevChannelID = ""
	view.ChannelID = ""
	_, resp = Client.ViewChannel(th.BasicUser.ID, view)
	CheckNoError(t, resp)

	view.PrevChannelID = ""
	view.ChannelID = "junk"
	_, resp = Client.ViewChannel(th.BasicUser.ID, view)
	CheckBadRequestStatus(t, resp)

	view.ChannelID = "correctlysizedjunkdddfdfdf"
	_, resp = Client.ViewChannel(th.BasicUser.ID, view)
	CheckBadRequestStatus(t, resp)
	view.ChannelID = th.BasicChannel.ID

	member, resp := Client.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
	CheckNoError(t, resp)
	channel, resp = Client.GetChannel(th.BasicChannel.ID, "")
	CheckNoError(t, resp)
	require.Equal(t, channel.TotalMsgCount, member.MsgCount, "should match message counts")
	require.Equal(t, int64(0), member.MentionCount, "should have no mentions")
	require.Equal(t, int64(0), member.MentionCountRoot, "should have no mentions")

	_, resp = Client.ViewChannel("junk", view)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.ViewChannel(th.BasicUser2.ID, view)
	CheckForbiddenStatus(t, resp)

	r, err := Client.DoAPIPost(fmt.Sprintf("/channels/members/%v/view", th.BasicUser.ID), "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	Client.Logout()
	_, resp = Client.ViewChannel(th.BasicUser.ID, view)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.ViewChannel(th.BasicUser.ID, view)
	CheckNoError(t, resp)
}

func TestGetChannelUnread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	channel := th.BasicChannel

	channelUnread, resp := Client.GetChannelUnread(channel.ID, user.ID)
	CheckNoError(t, resp)
	require.Equal(t, th.BasicTeam.ID, channelUnread.TeamID, "wrong team id returned for a regular user call")
	require.Equal(t, channel.ID, channelUnread.ChannelID, "wrong team id returned for a regular user call")

	_, resp = Client.GetChannelUnread("junk", user.ID)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelUnread(channel.ID, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelUnread(channel.ID, model.NewID())
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannelUnread(model.NewID(), user.ID)
	CheckForbiddenStatus(t, resp)

	newUser := th.CreateUser()
	Client.Login(newUser.Email, newUser.Password)
	_, resp = Client.GetChannelUnread(th.BasicChannel.ID, user.ID)
	CheckForbiddenStatus(t, resp)

	Client.Logout()

	_, resp = th.SystemAdminClient.GetChannelUnread(channel.ID, user.ID)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetChannelUnread(model.NewID(), user.ID)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelUnread(channel.ID, model.NewID())
	CheckNotFoundStatus(t, resp)
}

func TestGetChannelStats(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.CreatePrivateChannel()

	stats, resp := Client.GetChannelStats(channel.ID, "")
	CheckNoError(t, resp)

	require.Equal(t, channel.ID, stats.ChannelID, "couldnt't get extra info")
	require.Equal(t, int64(1), stats.MemberCount, "got incorrect member count")
	require.Equal(t, int64(0), stats.PinnedPostCount, "got incorrect pinned post count")

	th.CreatePinnedPostWithClient(th.Client, channel)
	stats, resp = Client.GetChannelStats(channel.ID, "")
	CheckNoError(t, resp)
	require.Equal(t, int64(1), stats.PinnedPostCount, "should have returned 1 pinned post count")

	_, resp = Client.GetChannelStats("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelStats(model.NewID(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelStats(channel.ID, "")
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic2()

	_, resp = Client.GetChannelStats(channel.ID, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelStats(channel.ID, "")
	CheckNoError(t, resp)
}

func TestGetPinnedPosts(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	posts, resp := Client.GetPinnedPosts(channel.ID, "")
	CheckNoError(t, resp)
	require.Empty(t, posts.Posts, "should not have gotten a pinned post")

	pinnedPost := th.CreatePinnedPost()
	posts, resp = Client.GetPinnedPosts(channel.ID, "")
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "should have returned 1 pinned post")
	require.Contains(t, posts.Posts, pinnedPost.ID, "missing pinned post")

	posts, resp = Client.GetPinnedPosts(channel.ID, resp.Etag)
	CheckEtag(t, posts, resp)

	_, resp = Client.GetPinnedPosts(GenerateTestID(), "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetPinnedPosts("junk", "")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPinnedPosts(channel.ID, "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPinnedPosts(channel.ID, "")
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
	pass, resp := Client.UpdateChannelRoles(channel.ID, th.BasicUser2.ID, ChannelAdmin)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	member, resp := Client.GetChannelMember(channel.ID, th.BasicUser2.ID, "")
	CheckNoError(t, resp)
	require.Equal(t, ChannelAdmin, member.Roles, "roles don't match")

	// User 1 demotes User 2
	_, resp = Client.UpdateChannelRoles(channel.ID, th.BasicUser2.ID, ChannelMember)
	CheckNoError(t, resp)

	th.LoginBasic2()

	// User 2 cannot demote User 1
	_, resp = Client.UpdateChannelRoles(channel.ID, th.BasicUser.ID, ChannelMember)
	CheckForbiddenStatus(t, resp)

	// User 2 cannot promote self
	_, resp = Client.UpdateChannelRoles(channel.ID, th.BasicUser2.ID, ChannelAdmin)
	CheckForbiddenStatus(t, resp)

	th.LoginBasic()

	// User 1 demotes self
	_, resp = Client.UpdateChannelRoles(channel.ID, th.BasicUser.ID, ChannelMember)
	CheckNoError(t, resp)

	// System Admin promotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.ID, th.BasicUser.ID, ChannelAdmin)
	CheckNoError(t, resp)

	// System Admin demotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.ID, th.BasicUser.ID, ChannelMember)
	CheckNoError(t, resp)

	// System Admin promotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.ID, th.BasicUser.ID, ChannelAdmin)
	CheckNoError(t, resp)

	th.LoginBasic()

	_, resp = Client.UpdateChannelRoles(channel.ID, th.BasicUser.ID, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(channel.ID, "junk", ChannelMember)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles("junk", th.BasicUser.ID, ChannelMember)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(channel.ID, model.NewID(), ChannelMember)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(model.NewID(), th.BasicUser.ID, ChannelMember)
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
	_, r1 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, th.BasicUser.ID, s1)
	CheckNoError(t, r1)

	timeout := time.After(600 * time.Millisecond)
	waiting := true
	for waiting {
		select {
		case event := <-WebSocketClient.EventChannel:
			if event.Event == model.WebsocketEventChannelMemberUpdated {
				require.Equal(t, model.WebsocketEventChannelMemberUpdated, event.Event)
				waiting = false
			}
		case <-timeout:
			require.Fail(t, "Should have received event channel member websocket event and not timedout")
			waiting = false
		}
	}

	tm1, rtm1 := SystemAdminClient.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm1)
	assert.Equal(t, false, tm1.SchemeGuest)
	assert.Equal(t, false, tm1.SchemeUser)
	assert.Equal(t, false, tm1.SchemeAdmin)

	s2 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, r2 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, th.BasicUser.ID, s2)
	CheckNoError(t, r2)

	tm2, rtm2 := SystemAdminClient.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm2)
	assert.Equal(t, false, tm2.SchemeGuest)
	assert.Equal(t, true, tm2.SchemeUser)
	assert.Equal(t, false, tm2.SchemeAdmin)

	s3 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  false,
		SchemeGuest: false,
	}
	_, r3 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, th.BasicUser.ID, s3)
	CheckNoError(t, r3)

	tm3, rtm3 := SystemAdminClient.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm3)
	assert.Equal(t, false, tm3.SchemeGuest)
	assert.Equal(t, false, tm3.SchemeUser)
	assert.Equal(t, true, tm3.SchemeAdmin)

	s4 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, r4 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, th.BasicUser.ID, s4)
	CheckNoError(t, r4)

	tm4, rtm4 := SystemAdminClient.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm4)
	assert.Equal(t, false, tm4.SchemeGuest)
	assert.Equal(t, true, tm4.SchemeUser)
	assert.Equal(t, true, tm4.SchemeAdmin)

	s5 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: true,
	}
	_, r5 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, th.BasicUser.ID, s5)
	CheckNoError(t, r5)

	tm5, rtm5 := SystemAdminClient.GetChannelMember(th.BasicChannel.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm5)
	assert.Equal(t, true, tm5.SchemeGuest)
	assert.Equal(t, false, tm5.SchemeUser)
	assert.Equal(t, false, tm5.SchemeAdmin)

	s6 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: true,
	}
	_, resp := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, th.BasicUser.ID, s6)
	CheckBadRequestStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(model.NewID(), th.BasicUser.ID, s4)
	CheckForbiddenStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, model.NewID(), s4)
	CheckNotFoundStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles("ASDF", th.BasicUser.ID, s4)
	CheckBadRequestStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, "ASDF", s4)
	CheckBadRequestStatus(t, resp)

	th.LoginBasic2()
	_, resp = th.Client.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, th.BasicUser.ID, s4)
	CheckForbiddenStatus(t, resp)

	SystemAdminClient.Logout()
	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.ID, th.SystemAdminUser.ID, s4)
	CheckUnauthorizedStatus(t, resp)
}

func TestUpdateChannelNotifyProps(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	props := map[string]string{}
	props[model.DesktopNotifyProp] = model.ChannelNotifyMention
	props[model.MarkUnreadNotifyProp] = model.ChannelMarkUnreadMention

	pass, resp := Client.UpdateChannelNotifyProps(th.BasicChannel.ID, th.BasicUser.ID, props)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	member, err := th.App.GetChannelMember(context.Background(), th.BasicChannel.ID, th.BasicUser.ID)
	require.Nil(t, err)
	require.Equal(t, model.ChannelNotifyMention, member.NotifyProps[model.DesktopNotifyProp], "bad update")
	require.Equal(t, model.ChannelMarkUnreadMention, member.NotifyProps[model.MarkUnreadNotifyProp], "bad update")

	_, resp = Client.UpdateChannelNotifyProps("junk", th.BasicUser.ID, props)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.ID, "junk", props)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(model.NewID(), th.BasicUser.ID, props)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.ID, model.NewID(), props)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.ID, th.BasicUser.ID, map[string]string{})
	CheckNoError(t, resp)

	Client.Logout()
	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.ID, th.BasicUser.ID, props)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel.ID, th.BasicUser.ID, props)
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
	_, resp := th.SystemAdminClient.AddTeamMember(team.ID, user3.ID)
	CheckNoError(t, resp)

	cm, resp := Client.AddChannelMember(publicChannel.ID, user2.ID)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.Equal(t, publicChannel.ID, cm.ChannelID, "should have returned exact channel")
	require.Equal(t, user2.ID, cm.UserID, "should have returned exact user added to public channel")

	cm, resp = Client.AddChannelMember(privateChannel.ID, user2.ID)
	CheckNoError(t, resp)
	require.Equal(t, privateChannel.ID, cm.ChannelID, "should have returned exact channel")
	require.Equal(t, user2.ID, cm.UserID, "should have returned exact user added to private channel")

	post := &model.Post{ChannelID: publicChannel.ID, Message: "a" + GenerateTestID() + "a"}
	rpost, err := Client.CreatePost(post)
	require.NotNil(t, err)

	Client.RemoveUserFromChannel(publicChannel.ID, user.ID)
	_, resp = Client.AddChannelMemberWithRootID(publicChannel.ID, user.ID, rpost.ID)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	Client.RemoveUserFromChannel(publicChannel.ID, user.ID)
	_, resp = Client.AddChannelMemberWithRootID(publicChannel.ID, user.ID, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.AddChannelMemberWithRootID(publicChannel.ID, user.ID, GenerateTestID())
	CheckNotFoundStatus(t, resp)

	Client.RemoveUserFromChannel(publicChannel.ID, user.ID)
	_, resp = Client.AddChannelMember(publicChannel.ID, user.ID)
	CheckNoError(t, resp)

	cm, resp = Client.AddChannelMember(publicChannel.ID, "junk")
	CheckBadRequestStatus(t, resp)
	require.Nil(t, cm, "should return nothing")

	_, resp = Client.AddChannelMember(publicChannel.ID, GenerateTestID())
	CheckNotFoundStatus(t, resp)

	_, resp = Client.AddChannelMember("junk", user2.ID)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.AddChannelMember(GenerateTestID(), user2.ID)
	CheckNotFoundStatus(t, resp)

	otherUser := th.CreateUser()
	otherChannel := th.CreatePublicChannel()
	Client.Logout()
	Client.Login(user2.ID, user2.Password)

	_, resp = Client.AddChannelMember(publicChannel.ID, otherUser.ID)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(privateChannel.ID, otherUser.ID)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(otherChannel.ID, otherUser.ID)
	CheckUnauthorizedStatus(t, resp)

	Client.Logout()
	Client.Login(user.ID, user.Password)

	// should fail adding user who is not a member of the team
	_, resp = Client.AddChannelMember(otherChannel.ID, otherUser.ID)
	CheckUnauthorizedStatus(t, resp)

	Client.DeleteChannel(otherChannel.ID)

	// should fail adding user to a deleted channel
	_, resp = Client.AddChannelMember(otherChannel.ID, user2.ID)
	CheckUnauthorizedStatus(t, resp)

	Client.Logout()
	_, resp = Client.AddChannelMember(publicChannel.ID, user2.ID)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(privateChannel.ID, user2.ID)
	CheckUnauthorizedStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.AddChannelMember(publicChannel.ID, user2.ID)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(privateChannel.ID, user2.ID)
		CheckNoError(t, resp)
	})

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PermissionManagePrivateChannelMembers.ID, model.ChannelUserRoleID)

	// Check that a regular channel user can add other users.
	Client.Login(user2.Username, user2.Password)
	privateChannel = th.CreatePrivateChannel()
	_, resp = Client.AddChannelMember(privateChannel.ID, user.ID)
	CheckNoError(t, resp)
	Client.Logout()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.ID, user3.ID)
	CheckNoError(t, resp)
	Client.Logout()

	// Restrict the permission for adding users to Channel Admins
	th.AddPermissionToRole(model.PermissionManagePrivateChannelMembers.ID, model.ChannelAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManagePrivateChannelMembers.ID, model.ChannelUserRoleID)

	Client.Login(user2.Username, user2.Password)
	privateChannel = th.CreatePrivateChannel()
	_, resp = Client.AddChannelMember(privateChannel.ID, user.ID)
	CheckNoError(t, resp)
	Client.Logout()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.ID, user3.ID)
	CheckForbiddenStatus(t, resp)
	Client.Logout()

	th.MakeUserChannelAdmin(user, privateChannel)
	th.App.Srv().InvalidateAllCaches()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.ID, user3.ID)
	CheckNoError(t, resp)
	Client.Logout()

	// Set a channel to group-constrained
	privateChannel.GroupConstrained = model.NewBool(true)
	_, appErr := th.App.UpdateChannel(privateChannel)
	require.Nil(t, appErr)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		// User is not in associated groups so shouldn't be allowed
		_, resp = client.AddChannelMember(privateChannel.ID, user.ID)
		CheckErrorMessage(t, resp, "api.channel.add_members.user_denied")
	})

	// Associate group to team
	_, appErr = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupID:    th.Group.ID,
		SyncableID: privateChannel.ID,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.Nil(t, appErr)

	// Add user to group
	_, appErr = th.App.UpsertGroupMember(th.Group.ID, user.ID)
	require.Nil(t, appErr)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.AddChannelMember(privateChannel.ID, user.ID)
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
			"Add myself to a public channel with JoinPublicChannel permission",
			notMemberPublicChannel1,
			true,
			"",
		},
		{
			"Try to add myself to a private channel with the JoinPublicChannel permission",
			notMemberPrivateChannel,
			true,
			"api.context.permissions.app_error",
		},
		{
			"Try to add myself to a public channel without the JoinPublicChannel permission",
			notMemberPublicChannel2,
			false,
			"api.context.permissions.app_error",
		},
		{
			"Add myself a public channel where I'm already a member, not having JoinPublicChannel or ManageMembers permission",
			memberPublicChannel,
			false,
			"",
		},
		{
			"Add myself a private channel where I'm already a member, not having JoinPublicChannel or ManageMembers permission",
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
				th.RemovePermissionFromRole(model.PermissionJoinPublicChannels.ID, model.TeamUserRoleID)
			}

			_, resp := Client.AddChannelMember(tc.Channel.ID, user.ID)
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
	th.App.AddUserToTeam(th.Context, team.ID, bot.UserID, "")

	pass, resp := Client.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser2.ID)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.ID, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.ID, model.NewID())
	CheckNotFoundStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(model.NewID(), th.BasicUser2.ID)
	CheckNotFoundStatus(t, resp)

	th.LoginBasic2()
	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser.ID)
	CheckForbiddenStatus(t, resp)

	t.Run("success", func(t *testing.T) {
		// Setup the system administrator to listen for websocket events from the channels.
		th.LinkUserToTeam(th.SystemAdminUser, th.BasicTeam)
		_, err := th.App.AddUserToChannel(th.SystemAdminUser, th.BasicChannel, false)
		require.Nil(t, err)
		_, err = th.App.AddUserToChannel(th.SystemAdminUser, th.BasicChannel2, false)
		require.Nil(t, err)
		props := map[string]string{}
		props[model.DesktopNotifyProp] = model.ChannelNotifyAll
		_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel.ID, th.SystemAdminUser.ID, props)
		_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel2.ID, th.SystemAdminUser.ID, props)
		CheckNoError(t, resp)

		wsClient, err := th.CreateWebSocketSystemAdminClient()
		require.Nil(t, err)
		wsClient.Listen()
		var closeWsClient sync.Once
		defer closeWsClient.Do(func() {
			wsClient.Close()
		})

		wsr := <-wsClient.EventChannel
		require.Equal(t, model.WebsocketEventHello, wsr.EventType())

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

					post := model.PostFromJSON(strings.NewReader(postData.(string)))
					if post.ChannelID == expectedPost.ChannelID && post.Message == expectedPost.Message {
						return
					}
				case <-time.After(5 * time.Second):
					require.FailNow(t, "failed to find expected post after 5 seconds")
					return
				}
			}
		}

		th.App.AddUserToChannel(th.BasicUser2, th.BasicChannel, false)
		_, resp = Client.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser2.ID)
		CheckNoError(t, resp)

		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s left the channel.", th.BasicUser2.Username),
			ChannelID: th.BasicChannel.ID,
		})

		_, resp = Client.RemoveUserFromChannel(th.BasicChannel2.ID, th.BasicUser.ID)
		CheckNoError(t, resp)
		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s removed from the channel.", th.BasicUser.Username),
			ChannelID: th.BasicChannel2.ID,
		})

		_, resp = th.SystemAdminClient.RemoveUserFromChannel(th.BasicChannel.ID, th.BasicUser.ID)
		CheckNoError(t, resp)
		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s removed from the channel.", th.BasicUser.Username),
			ChannelID: th.BasicChannel.ID,
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

	_, resp = Client.RemoveUserFromChannel(deletedChannel.ID, th.BasicUser.ID)
	CheckNoError(t, resp)

	th.LoginBasic()
	private := th.CreatePrivateChannel()
	th.App.AddUserToChannel(th.BasicUser2, private, false)

	_, resp = Client.RemoveUserFromChannel(private.ID, th.BasicUser2.ID)
	CheckNoError(t, resp)

	th.LoginBasic2()
	_, resp = Client.RemoveUserFromChannel(private.ID, th.BasicUser.ID)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		th.App.AddUserToChannel(th.BasicUser, private, false)
		_, resp = client.RemoveUserFromChannel(private.ID, th.BasicUser.ID)
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

	th.AddPermissionToRole(model.PermissionManagePrivateChannelMembers.ID, model.ChannelUserRoleID)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		// Check that a regular channel user can remove other users.
		privateChannel := th.CreateChannelWithClient(client, model.ChannelTypePrivate)
		_, resp = client.AddChannelMember(privateChannel.ID, user1.ID)
		CheckNoError(t, resp)
		_, resp = client.AddChannelMember(privateChannel.ID, user2.ID)
		CheckNoError(t, resp)

		_, resp = Client.RemoveUserFromChannel(privateChannel.ID, user2.ID)
		CheckNoError(t, resp)
	})

	// Restrict the permission for adding users to Channel Admins
	th.AddPermissionToRole(model.PermissionManagePrivateChannelMembers.ID, model.ChannelAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionManagePrivateChannelMembers.ID, model.ChannelUserRoleID)

	privateChannel := th.CreateChannelWithClient(th.SystemAdminClient, model.ChannelTypePrivate)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.ID, user1.ID)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.ID, user2.ID)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.ID, bot.UserID)
	CheckNoError(t, resp)

	_, resp = Client.RemoveUserFromChannel(privateChannel.ID, user2.ID)
	CheckForbiddenStatus(t, resp)

	th.MakeUserChannelAdmin(user1, privateChannel)
	th.App.Srv().InvalidateAllCaches()

	_, resp = Client.RemoveUserFromChannel(privateChannel.ID, user2.ID)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.ID, th.SystemAdminUser.ID)
	CheckNoError(t, resp)

	// If the channel is group-constrained the user cannot be removed
	privateChannel.GroupConstrained = model.NewBool(true)
	_, err := th.App.UpdateChannel(privateChannel)
	require.Nil(t, err)
	_, resp = Client.RemoveUserFromChannel(privateChannel.ID, user2.ID)
	require.Equal(t, "api.channel.remove_member.group_constrained.app_error", resp.Error.ID)

	// If the channel is group-constrained user can remove self
	_, resp = th.SystemAdminClient.RemoveUserFromChannel(privateChannel.ID, th.SystemAdminUser.ID)
	CheckNoError(t, resp)

	// Test on preventing removal of user from a direct channel
	directChannel, resp := Client.CreateDirectChannel(user1.ID, user2.ID)
	CheckNoError(t, resp)

	// If the channel is group-constrained a user can remove a bot
	_, resp = Client.RemoveUserFromChannel(privateChannel.ID, bot.UserID)
	CheckNoError(t, resp)

	_, resp = Client.RemoveUserFromChannel(directChannel.ID, user1.ID)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(directChannel.ID, user2.ID)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.RemoveUserFromChannel(directChannel.ID, user1.ID)
	CheckBadRequestStatus(t, resp)

	// Test on preventing removal of user from a group channel
	user3 := th.CreateUser()
	groupChannel, resp := Client.CreateGroupChannel([]string{user1.ID, user2.ID, user3.ID})
	CheckNoError(t, resp)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.RemoveUserFromChannel(groupChannel.ID, user1.ID)
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
		Type:        model.ChannelTypePrivate,
		TeamID:      th.BasicTeam.ID,
	})
	tower, _ := th.Client.CreateChannel(&model.Channel{
		DisplayName: "Tower",
		Name:        "tower",
		Type:        model.ChannelTypeOpen,
		TeamID:      th.BasicTeam.ID,
	})
	utils.EnableDebugLogForTest()
	defer func() {
		th.Client.DeleteChannel(ptown.ID)
		th.Client.DeleteChannel(tower.ID)
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
			th.BasicTeam.ID,
			"town",
			[]string{"town-square"},
			[]string{"off-topic", "town", "tower"},
		},
		{
			"Basic off-topic",
			th.BasicTeam.ID,
			"off-to",
			[]string{"off-topic"},
			[]string{"town-square", "town", "tower"},
		},
		{
			"Basic town square and off topic",
			th.BasicTeam.ID,
			"tow",
			[]string{"town-square", "tower"},
			[]string{"off-topic", "town"},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, resp := th.Client.AutocompleteChannelsForTeam(tc.teamID, tc.fragment)
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
		Type:        model.ChannelTypePrivate,
		TeamID:      th.BasicTeam.ID,
	})
	defer func() {
		th.Client.DeleteChannel(ptown.ID)
	}()
	mypriv, _ := th.Client.CreateChannel(&model.Channel{
		DisplayName: "My private town",
		Name:        "townpriv",
		Type:        model.ChannelTypePrivate,
		TeamID:      th.BasicTeam.ID,
	})
	defer func() {
		th.Client.DeleteChannel(mypriv.ID)
	}()
	utils.EnableDebugLogForTest()

	dc1, resp := th.Client.CreateDirectChannel(th.BasicUser.ID, u1.ID)
	CheckNoError(t, resp)
	defer func() {
		th.Client.DeleteChannel(dc1.ID)
	}()

	dc2, resp := th.SystemAdminClient.CreateDirectChannel(u2.ID, u3.ID)
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(dc2.ID)
	}()

	gc1, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.ID, u2.ID, u3.ID})
	CheckNoError(t, resp)
	defer func() {
		th.Client.DeleteChannel(gc1.ID)
	}()

	gc2, resp := th.SystemAdminClient.CreateGroupChannel([]string{u2.ID, u3.ID, u4.ID})
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(gc2.ID)
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
			th.BasicTeam.ID,
			"town",
			[]string{"town-square", "townpriv"},
			[]string{"off-topic", "town"},
		},
		{
			"Basic off-topic",
			th.BasicTeam.ID,
			"off-to",
			[]string{"off-topic"},
			[]string{"town-square", "town", "townpriv"},
		},
		{
			"Basic town square and townpriv",
			th.BasicTeam.ID,
			"tow",
			[]string{"town-square", "townpriv"},
			[]string{"off-topic", "town"},
		},
		{
			"Direct and group messages",
			th.BasicTeam.ID,
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

	th.LoginSystemAdminWithClient(th.SystemAdminClient)

	_, resp := th.SystemAdminClient.AddTeamMember(th.BasicTeam.ID, guest.ID)
	CheckNoError(t, resp)

	// A private channel to make sure private channels are not used
	utils.DisableDebugLogForTest()
	town, _ := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.ChannelTypeOpen,
		TeamID:      th.BasicTeam.ID,
	})
	defer func() {
		th.SystemAdminClient.DeleteChannel(town.ID)
	}()
	_, resp = th.SystemAdminClient.AddChannelMember(town.ID, guest.ID)
	CheckNoError(t, resp)

	mypriv, _ := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "My private town",
		Name:        "townpriv",
		Type:        model.ChannelTypePrivate,
		TeamID:      th.BasicTeam.ID,
	})
	defer func() {
		th.SystemAdminClient.DeleteChannel(mypriv.ID)
	}()
	_, resp = th.SystemAdminClient.AddChannelMember(mypriv.ID, guest.ID)
	CheckNoError(t, resp)

	utils.EnableDebugLogForTest()

	dc1, resp := th.SystemAdminClient.CreateDirectChannel(th.BasicUser.ID, guest.ID)
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(dc1.ID)
	}()

	dc2, resp := th.SystemAdminClient.CreateDirectChannel(th.BasicUser.ID, th.BasicUser2.ID)
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(dc2.ID)
	}()

	gc1, resp := th.SystemAdminClient.CreateGroupChannel([]string{th.BasicUser.ID, th.BasicUser2.ID, guest.ID})
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(gc1.ID)
	}()

	gc2, resp := th.SystemAdminClient.CreateGroupChannel([]string{th.BasicUser.ID, th.BasicUser2.ID, u1.ID})
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(gc2.ID)
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
			th.BasicTeam.ID,
			"town",
			[]string{"town", "townpriv"},
			[]string{"town-square", "off-topic"},
		},
		{
			"Should return empty if not member of the searched channels",
			th.BasicTeam.ID,
			"off-to",
			[]string{},
			[]string{"off-topic", "town-square", "town", "townpriv"},
		},
		{
			"Should return direct and group messages",
			th.BasicTeam.ID,
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
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	})
	CheckNoError(t, resp)

	channel, resp := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "Name",
		Name:        "z-z-" + model.NewID() + "a",
		Type:        model.ChannelTypeOpen,
		TeamID:      team.ID,
	})
	CheckNoError(t, resp)

	channelScheme, resp := th.SystemAdminClient.CreateScheme(&model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewID(),
		Description: "Some description",
		Scope:       model.SchemeScopeChannel,
	})
	CheckNoError(t, resp)

	teamScheme, resp := th.SystemAdminClient.CreateScheme(&model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewID(),
		Description: "Some description",
		Scope:       model.SchemeScopeTeam,
	})
	CheckNoError(t, resp)

	// Test the setup/base case.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.ID, channelScheme.ID)
	CheckNoError(t, resp)

	// Test various invalid channel and scheme id combinations.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.ID, "x")
	CheckBadRequestStatus(t, resp)
	_, resp = th.SystemAdminClient.UpdateChannelScheme("x", channelScheme.ID)
	CheckBadRequestStatus(t, resp)
	_, resp = th.SystemAdminClient.UpdateChannelScheme("x", "x")
	CheckBadRequestStatus(t, resp)

	// Test that permissions are required.
	_, resp = th.Client.UpdateChannelScheme(channel.ID, channelScheme.ID)
	CheckForbiddenStatus(t, resp)

	// Test that a license is required.
	th.App.Srv().SetLicense(nil)
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.ID, channelScheme.ID)
	CheckNotImplementedStatus(t, resp)
	th.App.Srv().SetLicense(model.NewTestLicense(""))

	// Test an invalid scheme scope.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.ID, teamScheme.ID)
	CheckBadRequestStatus(t, resp)

	// Test that an unauthenticated user gets rejected.
	th.SystemAdminClient.Logout()
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.ID, channelScheme.ID)
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

	timezone, resp := Client.GetChannelMembersTimezones(th.BasicChannel.ID)
	CheckNoError(t, resp)
	require.Len(t, timezone, 2, "should return 2 timezones")

	//both users have same timezone
	user2.Timezone["automaticTimezone"] = "XOXO/BLABLA"
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	timezone, resp = Client.GetChannelMembersTimezones(th.BasicChannel.ID)
	CheckNoError(t, resp)
	require.Len(t, timezone, 1, "should return 1 timezone")

	//no timezone set should return empty
	user2.Timezone["automaticTimezone"] = ""
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	user.Timezone["manualTimezone"] = ""
	_, resp = Client.UpdateUser(user)
	CheckNoError(t, resp)

	timezone, resp = Client.GetChannelMembersTimezones(th.BasicChannel.ID)
	CheckNoError(t, resp)
	require.Empty(t, timezone, "should return 0 timezone")
}

func TestChannelMembersMinusGroupMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user1 := th.BasicUser
	user2 := th.BasicUser2

	channel := th.CreatePrivateChannel()

	_, err := th.App.AddChannelMember(th.Context, user1.ID, channel, app.ChannelMemberOpts{})
	require.Nil(t, err)
	_, err = th.App.AddChannelMember(th.Context, user2.ID, channel, app.ChannelMemberOpts{})
	require.Nil(t, err)

	channel.GroupConstrained = model.NewBool(true)
	channel, err = th.App.UpdateChannel(channel)
	require.Nil(t, err)

	group1 := th.CreateGroup()
	group2 := th.CreateGroup()

	_, err = th.App.UpsertGroupMember(group1.ID, user1.ID)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupMember(group2.ID, user2.ID)
	require.Nil(t, err)

	// No permissions
	_, _, res := th.Client.ChannelMembersMinusGroupMembers(channel.ID, []string{group1.ID, group2.ID}, 0, 100, "")
	require.Equal(t, "api.context.permissions.app_error", res.Error.ID)

	testCases := map[string]struct {
		groupIDs        []string
		page            int
		perPage         int
		length          int
		count           int
		otherAssertions func([]*model.UserWithGroups)
	}{
		"All groups, expect no users removed": {
			groupIDs: []string{group1.ID, group2.ID},
			page:     0,
			perPage:  100,
			length:   0,
			count:    0,
		},
		"Some nonexistent group, page 0": {
			groupIDs: []string{model.NewID()},
			page:     0,
			perPage:  1,
			length:   1,
			count:    2,
		},
		"Some nonexistent group, page 1": {
			groupIDs: []string{model.NewID()},
			page:     1,
			perPage:  1,
			length:   1,
			count:    2,
		},
		"One group, expect one user removed": {
			groupIDs: []string{group1.ID},
			page:     0,
			perPage:  100,
			length:   1,
			count:    1,
			otherAssertions: func(uwg []*model.UserWithGroups) {
				require.Equal(t, uwg[0].ID, user2.ID)
			},
		},
		"Other group, expect other user removed": {
			groupIDs: []string{group2.ID},
			page:     0,
			perPage:  100,
			length:   1,
			count:    1,
			otherAssertions: func(uwg []*model.UserWithGroups) {
				require.Equal(t, uwg[0].ID, user1.ID)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uwg, count, res := th.SystemAdminClient.ChannelMembersMinusGroupMembers(channel.ID, tc.groupIDs, tc.page, tc.perPage, "")
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
		_, res := th.SystemAdminClient.GetChannelModerations(channel.ID, "")
		require.Equal(t, "api.channel.get_channel_moderations.license.error", res.Error.ID)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors as a non sysadmin", func(t *testing.T) {
		_, res := th.Client.GetChannelModerations(channel.ID, "")
		require.Equal(t, "api.context.permissions.app_error", res.Error.ID)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Returns default moderations with default roles", func(t *testing.T) {
		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.ID, "")
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
		team.SchemeID = &scheme.ID
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		th.RemovePermissionFromRole(model.PermissionCreatePost.ID, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PermissionCreatePost.ID, scheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.ID, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PermissionCreatePost.ID {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}
		}
	})

	t.Run("Returns value false and enabled true for permissions that are not present in channel scheme but present in team scheme", func(t *testing.T) {
		scheme := th.SetupChannelScheme()
		channel.SchemeID = &scheme.ID
		_, err := th.App.UpdateChannelScheme(channel)
		require.Nil(t, err)

		th.RemovePermissionFromRole(model.PermissionCreatePost.ID, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PermissionCreatePost.ID, scheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.ID, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PermissionCreatePost.ID {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}
		}
	})

	t.Run("Returns value false and enabled false for permissions that are not present in channel & team scheme", func(t *testing.T) {
		teamScheme := th.SetupTeamScheme()
		team.SchemeID = &teamScheme.ID
		th.App.UpdateTeamScheme(team)

		scheme := th.SetupChannelScheme()
		channel.SchemeID = &scheme.ID
		th.App.UpdateChannelScheme(channel)

		th.RemovePermissionFromRole(model.PermissionCreatePost.ID, scheme.DefaultChannelGuestRole)
		th.RemovePermissionFromRole(model.PermissionCreatePost.ID, teamScheme.DefaultChannelGuestRole)

		defer th.AddPermissionToRole(model.PermissionCreatePost.ID, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PermissionCreatePost.ID, teamScheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.ID, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PermissionCreatePost.ID {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}
		}
	})

	t.Run("Returns the correct value for manage_members depending on whether the channel is public or private", func(t *testing.T) {
		scheme := th.SetupTeamScheme()
		team.SchemeID = &scheme.ID
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		th.RemovePermissionFromRole(model.PermissionManagePublicChannelMembers.ID, scheme.DefaultChannelUserRole)
		defer th.AddPermissionToRole(model.PermissionCreatePost.ID, scheme.DefaultChannelUserRole)

		// public channel does not have the permission
		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.ID, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Equal(t, moderation.Roles.Members.Value, false)
			}
		}

		// private channel does have the permission
		moderations, res = th.SystemAdminClient.GetChannelModerations(th.BasicPrivateChannel.ID, "")
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

		team.SchemeID = &scheme.ID
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		_, res := th.SystemAdminClient.GetChannelModerations(channel.ID, "")
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
		_, res := th.SystemAdminClient.PatchChannelModerations(channel.ID, emptyPatch)
		require.Equal(t, "api.channel.patch_channel_moderations.license.error", res.Error.ID)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors as a non sysadmin", func(t *testing.T) {
		_, res := th.Client.PatchChannelModerations(channel.ID, emptyPatch)
		require.Equal(t, "api.context.permissions.app_error", res.Error.ID)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Returns default moderations with empty patch", func(t *testing.T) {
		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.ID, emptyPatch)
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

		require.Nil(t, channel.SchemeID)
	})

	t.Run("Creates a scheme and returns the updated channel moderations when patching an existing permission", func(t *testing.T) {
		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(false)},
			},
		}

		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.ID, patch)
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
		channel, _ = th.App.GetChannel(channel.ID)
		require.NotNil(t, channel.SchemeID)
	})

	t.Run("Removes the existing scheme when moderated permissions are set back to higher scoped values", func(t *testing.T) {
		channel, _ = th.App.GetChannel(channel.ID)
		schemeID := channel.SchemeID

		scheme, _ := th.App.GetScheme(*schemeID)
		require.Equal(t, scheme.DeleteAt, int64(0))

		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(true)},
			},
		}

		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.ID, patch)
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

		channel, _ = th.App.GetChannel(channel.ID)
		require.Nil(t, channel.SchemeID)

		scheme, _ = th.App.GetScheme(*schemeID)
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

		team.SchemeID = &scheme.ID
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.ID, emptyPatch)
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

		moderations, res = th.SystemAdminClient.PatchChannelModerations(channel.ID, patch)
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
		_, res := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.ID, false, "")
		require.Equal(t, "api.channel.channel_member_counts_by_group.license.error", res.Error.ID)
	})

	th.App.Srv().SetLicense(model.NewTestLicense())

	t.Run("Errors without read permission to the channel", func(t *testing.T) {
		_, res := th.Client.GetChannelMemberCountsByGroup(model.NewID(), false, "")
		require.Equal(t, "api.context.permissions.app_error", res.Error.ID)
	})

	t.Run("Returns empty for a channel with no members or groups", func(t *testing.T) {
		memberCounts, _ := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.ID, false, "")
		require.Equal(t, []*model.ChannelMemberCountByGroup{}, memberCounts)
	})

	user := th.BasicUser
	user.Timezone["useAutomaticTimezone"] = "false"
	user.Timezone["manualTimezone"] = "XOXO/BLABLA"
	_, err := th.App.UpsertGroupMember(th.Group.ID, user.ID)
	require.Nil(t, err)
	_, resp := th.SystemAdminClient.UpdateUser(user)
	CheckNoError(t, resp)

	user2 := th.BasicUser2
	user2.Timezone["automaticTimezone"] = "NoWhere/Island"
	_, err = th.App.UpsertGroupMember(th.Group.ID, user2.ID)
	require.Nil(t, err)
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	t.Run("Returns users in group without timezones", func(t *testing.T) {
		memberCounts, _ := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.ID, false, "")
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupID:                     th.Group.ID,
				ChannelMemberCount:          2,
				ChannelMemberTimezonesCount: 0,
			},
		}
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	t.Run("Returns users in group with timezones", func(t *testing.T) {
		memberCounts, _ := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.ID, true, "")
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupID:                     th.Group.ID,
				ChannelMemberCount:          2,
				ChannelMemberTimezonesCount: 2,
			},
		}
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	id := model.NewID()
	group := &model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}

	_, err = th.App.CreateGroup(group)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupMember(group.ID, user.ID)
	require.Nil(t, err)

	t.Run("Returns multiple groups with users in group with timezones", func(t *testing.T) {
		memberCounts, _ := th.SystemAdminClient.GetChannelMemberCountsByGroup(channel.ID, true, "")
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupID:                     group.ID,
				ChannelMemberCount:          1,
				ChannelMemberTimezonesCount: 1,
			},
			{
				GroupID:                     th.Group.ID,
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
		ch, resp := th.SystemAdminClient.MoveChannel(publicChannel.ID, team2.ID, false)
		require.Nil(t, resp.Error)
		require.Equal(t, team2.ID, ch.TeamID)
	})

	t.Run("Should move private channel", func(t *testing.T) {
		channel := th.CreatePrivateChannel()
		ch, resp := th.SystemAdminClient.MoveChannel(channel.ID, team1.ID, false)
		require.Nil(t, resp.Error)
		require.Equal(t, team1.ID, ch.TeamID)
	})

	t.Run("Should fail when trying to move a DM channel", func(t *testing.T) {
		user := th.CreateUser()
		dmChannel := th.CreateDmChannel(user)
		_, resp := Client.MoveChannel(dmChannel.ID, team1.ID, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "api.channel.move_channel.type.invalid")
	})

	t.Run("Should fail when trying to move a group channel", func(t *testing.T) {
		user := th.CreateUser()

		gmChannel, err := th.App.CreateGroupChannel([]string{th.BasicUser.ID, th.SystemAdminUser.ID, th.TeamAdminUser.ID}, user.ID)
		require.Nil(t, err)
		_, resp := Client.MoveChannel(gmChannel.ID, team1.ID, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "api.channel.move_channel.type.invalid")
	})

	t.Run("Should fail due to permissions", func(t *testing.T) {
		publicChannel := th.CreatePublicChannel()
		_, resp := Client.MoveChannel(publicChannel.ID, team1.ID, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		publicChannel := th.CreatePublicChannel()
		user := th.BasicUser

		_, resp := client.RemoveTeamMember(team2.ID, user.ID)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(publicChannel.ID, user.ID)
		CheckNoError(t, resp)

		_, resp = client.MoveChannel(publicChannel.ID, team2.ID, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "app.channel.move_channel.members_do_not_match.error")
	}, "Should fail to move public channel due to a member not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel()
		user := th.BasicUser

		_, resp := client.RemoveTeamMember(team2.ID, user.ID)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(privateChannel.ID, user.ID)
		CheckNoError(t, resp)

		_, resp = client.MoveChannel(privateChannel.ID, team2.ID, false)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "app.channel.move_channel.members_do_not_match.error")
	}, "Should fail to move private channel due to a member not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		publicChannel := th.CreatePublicChannel()
		user := th.BasicUser

		_, resp := client.RemoveTeamMember(team2.ID, user.ID)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(publicChannel.ID, user.ID)
		CheckNoError(t, resp)

		newChannel, resp := client.MoveChannel(publicChannel.ID, team2.ID, true)
		require.Nil(t, resp.Error)
		require.Equal(t, team2.ID, newChannel.TeamID)
	}, "Should be able to (force) move public channel by a member that is not member of target team")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		privateChannel := th.CreatePrivateChannel()
		user := th.BasicUser

		_, resp := client.RemoveTeamMember(team2.ID, user.ID)
		CheckNoError(t, resp)

		_, resp = client.AddChannelMember(privateChannel.ID, user.ID)
		CheckNoError(t, resp)

		newChannel, resp := client.MoveChannel(privateChannel.ID, team2.ID, true)
		require.Nil(t, resp.Error)
		require.Equal(t, team2.ID, newChannel.TeamID)
	}, "Should be able to (force) move private channel by a member that is not member of target team")
}

func TestRootMentionsCount(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	user := th.BasicUser
	channel := th.BasicChannel

	// initially, MentionCountRoot is 0 in the database
	channelMember, err := th.App.Srv().Store.Channel().GetMember(context.Background(), channel.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), channelMember.MentionCountRoot)
	require.Equal(t, int64(0), channelMember.MentionCount)

	// mention the user in a root post
	post1, resp := th.SystemAdminClient.CreatePost(&model.Post{ChannelID: channel.ID, Message: "hey @" + user.Username})
	CheckNoError(t, resp)
	// mention the user in a reply post
	post2 := &model.Post{ChannelID: channel.ID, Message: "reply at @" + user.Username, RootID: post1.ID}
	_, resp = th.SystemAdminClient.CreatePost(post2)
	CheckNoError(t, resp)

	// this should perform lazy migration and populate the field
	channelUnread, resp := Client.GetChannelUnread(channel.ID, user.ID)
	CheckNoError(t, resp)
	// reply post is not counted, so we should have one root mention
	require.EqualValues(t, int64(1), channelUnread.MentionCountRoot)
	// regular count stays the same
	require.Equal(t, int64(2), channelUnread.MentionCount)
	// validate that DB is updated
	channelMember, err = th.App.Srv().Store.Channel().GetMember(context.Background(), channel.ID, user.ID)
	require.NoError(t, err)
	require.EqualValues(t, int64(1), channelMember.MentionCountRoot)

	// validate that Team level counts are calculated
	counts, appErr := th.App.GetTeamUnread(channel.TeamID, user.ID)
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
		*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDefaultOn
	})

	Client := th.Client
	user := th.BasicUser
	team := th.BasicTeam
	channel := th.BasicChannel

	// mention the user in a root post
	post1, resp := th.SystemAdminClient.CreatePost(&model.Post{ChannelID: channel.ID, Message: "hey @" + user.Username})
	CheckNoError(t, resp)
	// mention the user in a reply post
	post2 := &model.Post{ChannelID: channel.ID, Message: "reply at @" + user.Username, RootID: post1.ID}
	_, resp = th.SystemAdminClient.CreatePost(post2)
	CheckNoError(t, resp)

	threads, resp := Client.GetUserThreads(user.ID, team.ID, model.GetUserThreadsOpts{})
	CheckNoError(t, resp)
	require.EqualValues(t, int64(1), threads.TotalUnreadMentions)

	// simulate opening the channel from an old client
	_, resp = Client.ViewChannel(user.ID, &model.ChannelView{
		ChannelID:                 channel.ID,
		PrevChannelID:             "",
		CollapsedThreadsSupported: false,
	})
	CheckNoError(t, resp)

	threads, resp = Client.GetUserThreads(user.ID, team.ID, model.GetUserThreadsOpts{})
	CheckNoError(t, resp)
	require.Zero(t, threads.TotalUnreadMentions)
}
