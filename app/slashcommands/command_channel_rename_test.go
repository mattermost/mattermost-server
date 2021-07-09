// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestRenameProviderDoCommand(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	th.addPermissionToRole(model.PermissionManagePublicChannelProperties.ID, model.ChannelUserRoleID)

	rp := RenameProvider{}
	args := &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: th.BasicChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	// Table Test for basic cases. Blank text in response indicates success
	for msg, expected := range map[string]string{
		"":                                    "api.command_channel_rename.message.app_error",
		"o":                                   "api.command_channel_rename.too_short.app_error",
		"joram":                               "",
		"More than 22 chars but less than 64": "",
		strings.Repeat("12345", 13):           "api.command_channel_rename.too_long.app_error",
	} {
		actual := rp.DoCommand(th.App, th.Context, args, msg).Text
		assert.Equal(t, expected, actual)
	}

	// Try a public channel *without* permission.
	th.removePermissionFromRole(model.PermissionManagePublicChannelProperties.ID, model.ChannelUserRoleID)

	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: th.BasicChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual := rp.DoCommand(th.App, th.Context, args, "hello").Text
	assert.Equal(t, "api.command_channel_rename.permission.app_error", actual)

	// Try a private channel *with* permission.
	privateChannel := th.createPrivateChannel(th.BasicTeam)

	th.addPermissionToRole(model.PermissionManagePrivateChannelProperties.ID, model.ChannelUserRoleID)

	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: privateChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, "hello").Text
	assert.Equal(t, "", actual)

	// Try a private channel *without* permission.
	th.removePermissionFromRole(model.PermissionManagePrivateChannelProperties.ID, model.ChannelUserRoleID)

	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: privateChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, "hello").Text
	assert.Equal(t, "api.command_channel_rename.permission.app_error", actual)

	// Try a group channel *with* being a member.
	user1 := th.createUser()
	user2 := th.createUser()

	groupChannel := th.createGroupChannel(user1, user2)

	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: groupChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, "hello").Text
	assert.Equal(t, "api.command_channel_rename.direct_group.app_error", actual)

	// Try a direct channel *with* being a member.
	directChannel := th.createDmChannel(user1)

	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: directChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, "hello").Text
	assert.Equal(t, "api.command_channel_rename.direct_group.app_error", actual)
}
