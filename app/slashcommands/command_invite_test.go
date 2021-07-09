// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
)

func TestInviteProvider(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	channel := th.createChannel(th.BasicTeam, model.ChannelTypeOpen)
	privateChannel := th.createChannel(th.BasicTeam, model.ChannelTypePrivate)
	dmChannel := th.createDmChannel(th.BasicUser2)
	privateChannel2 := th.createChannelWithAnotherUser(th.BasicTeam, model.ChannelTypePrivate, th.BasicUser2.ID)

	basicUser3 := th.createUser()
	th.linkUserToTeam(basicUser3, th.BasicTeam)
	basicUser4 := th.createUser()
	deactivatedUser := th.createUser()
	th.App.UpdateActive(th.Context, deactivatedUser, false)

	var err *model.AppError
	_, err = th.App.CreateBot(th.Context, &model.Bot{
		Username:    "bot1",
		OwnerID:     basicUser3.ID,
		Description: "a test bot",
	})
	require.Nil(t, err)

	bot2, err := th.App.CreateBot(th.Context, &model.Bot{
		Username:    "bot2",
		OwnerID:     basicUser3.ID,
		Description: "a test bot",
	})
	require.Nil(t, err)
	_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, bot2.UserID, basicUser3.ID)
	require.Nil(t, err)

	bot3, err := th.App.CreateBot(th.Context, &model.Bot{
		Username:    "bot3",
		OwnerID:     basicUser3.ID,
		Description: "a test bot",
	})
	require.Nil(t, err)
	_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, bot3.UserID, basicUser3.ID)
	require.Nil(t, err)
	err = th.App.RemoveUserFromTeam(th.Context, th.BasicTeam.ID, bot3.UserID, basicUser3.ID)
	require.Nil(t, err)

	InviteP := InviteProvider{}
	args := &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: th.BasicChannel.ID,
		TeamID:    th.BasicTeam.ID,
		UserID:    th.BasicUser.ID,
	}

	userAndWrongChannel := "@" + th.BasicUser2.Username + " wrongchannel1"
	userAndChannel := "@" + th.BasicUser2.Username + " ~" + channel.Name + " "
	userAndDisplayChannel := "@" + th.BasicUser2.Username + " ~" + channel.DisplayName + " "
	userAndPrivateChannel := "@" + th.BasicUser2.Username + " ~" + privateChannel.Name
	userAndDMChannel := "@" + basicUser3.Username + " ~" + dmChannel.Name
	userAndInvalidPrivate := "@" + basicUser3.Username + " ~" + privateChannel2.Name
	deactivatedUserPublicChannel := "@" + deactivatedUser.Username + " ~" + channel.Name

	groupChannel := th.createChannel(th.BasicTeam, model.ChannelTypePrivate)
	_, err = th.App.AddChannelMember(th.Context, th.BasicUser.ID, groupChannel, app.ChannelMemberOpts{})
	require.Nil(t, err)
	groupChannel.GroupConstrained = model.NewBool(true)
	groupChannel, _ = th.App.UpdateChannel(groupChannel)

	groupChannelNonUser := "@" + th.BasicUser2.Username + " ~" + groupChannel.Name

	tests := []struct {
		desc     string
		expected string
		msg      string
	}{
		{
			desc:     "Missing user and channel in the command",
			expected: "api.command_invite.missing_message.app_error",
			msg:      "",
		},
		{
			desc:     "User added in the current channel",
			expected: "",
			msg:      th.BasicUser2.Username,
		},
		{
			desc:     "Add user to another channel not the current",
			expected: "api.command_invite.success",
			msg:      userAndChannel,
		},
		{
			desc:     "try to add a user to a direct channel",
			expected: "api.command_invite.directchannel.app_error",
			msg:      userAndDMChannel,
		},
		{
			desc:     "Try to add a user to a invalid channel",
			expected: "api.command_invite.channel.error",
			msg:      userAndWrongChannel,
		},
		{
			desc:     "Try to add a user to an private channel",
			expected: "api.command_invite.success",
			msg:      userAndPrivateChannel,
		},
		{
			desc:     "Using display channel name which is different form Channel name",
			expected: "api.command_invite.channel.error",
			msg:      userAndDisplayChannel,
		},
		{
			desc:     "Invalid user to current channel",
			expected: "api.command_invite.missing_user.app_error",
			msg:      "@invalidUser123",
		},
		{
			desc:     "Invalid user to current channel without @",
			expected: "api.command_invite.missing_user.app_error",
			msg:      "invalidUser321",
		},
		{
			desc:     "try to add a user which is not part of the team",
			expected: "api.command_invite.user_not_in_team.app_error",
			msg:      basicUser4.Username,
		},
		{
			desc:     "try to add a user not part of the group to a group channel",
			expected: "api.command_invite.group_constrained_user_denied",
			msg:      groupChannelNonUser,
		},
		{
			desc:     "try to add a user to a private channel with no permission",
			expected: "api.command_invite.private_channel.app_error",
			msg:      userAndInvalidPrivate,
		},
		{
			desc:     "try to add a deleted user to a public channel",
			expected: "api.command_invite.missing_user.app_error",
			msg:      deactivatedUserPublicChannel,
		},
		{
			desc:     "try to add bot to a public channel",
			expected: "api.command_invite.user_not_in_team.app_error",
			msg:      "@bot1",
		},
		{
			desc:     "add bot to a public channel",
			expected: "",
			msg:      "@bot2",
		},
		{
			desc:     "try to add bot removed from a team to a public channel",
			expected: "api.command_invite.user_not_in_team.app_error",
			msg:      "@bot3",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			actual := InviteP.DoCommand(th.App, th.Context, args, test.msg).Text
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestInviteGroup(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	th.BasicTeam.GroupConstrained = model.NewBool(true)
	var err *model.AppError
	_, _ = th.App.AddTeamMember(th.Context, th.BasicTeam.ID, th.BasicUser.ID)
	_, err = th.App.AddTeamMember(th.Context, th.BasicTeam.ID, th.BasicUser2.ID)
	require.Nil(t, err)
	th.BasicTeam, _ = th.App.UpdateTeam(th.BasicTeam)

	privateChannel := th.createChannel(th.BasicTeam, model.ChannelTypePrivate)

	groupChannelUser1 := "@" + th.BasicUser.Username + " ~" + privateChannel.Name
	groupChannelUser2 := "@" + th.BasicUser2.Username + " ~" + privateChannel.Name
	basicUser3 := th.createUser()
	groupChannelUser3 := "@" + basicUser3.Username + " ~" + privateChannel.Name

	InviteP := InviteProvider{}
	args := &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: th.BasicChannel.ID,
		TeamID:    th.BasicTeam.ID,
		UserID:    th.BasicUser.ID,
	}

	tests := []struct {
		desc     string
		expected string
		msg      string
	}{
		{
			desc:     "try to add an existing user part of the group to a group channel",
			expected: "api.command_invite.user_already_in_channel.app_error",
			msg:      groupChannelUser1,
		},
		{
			desc:     "try to add a user part of the group to a group channel",
			expected: "api.command_invite.success",
			msg:      groupChannelUser2,
		},
		{
			desc:     "try to add a user NOT part of the group to a group channel",
			expected: "api.command_invite.user_not_in_team.app_error",
			msg:      groupChannelUser3,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			actual := InviteP.DoCommand(th.App, th.Context, args, test.msg).Text
			assert.Equal(t, test.expected, actual)
		})
	}
}
