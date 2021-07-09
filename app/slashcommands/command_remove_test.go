// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestRemoveProviderDoCommand(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	rp := RemoveProvider{}

	publicChannel, _ := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "AA",
		Name:        "aa" + model.NewID() + "a",
		Type:        model.ChannelTypeOpen,
		TeamID:      th.BasicTeam.ID,
		CreatorID:   th.BasicUser.ID,
	}, false)

	privateChannel, _ := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "BB",
		Name:        "aa" + model.NewID() + "a",
		Type:        model.ChannelTypeOpen,
		TeamID:      th.BasicTeam.ID,
		CreatorID:   th.BasicUser.ID,
	}, false)

	targetUser := th.createUser()
	th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, targetUser.ID, targetUser.ID)
	th.App.AddUserToChannel(targetUser, publicChannel, false)
	th.App.AddUserToChannel(targetUser, privateChannel, false)

	// Try a public channel *without* permission.
	args := &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: publicChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual := rp.DoCommand(th.App, th.Context, args, targetUser.Username).Text
	assert.Equal(t, "api.command_remove.permission.app_error", actual)

	// Try a public channel *with* permission.
	th.App.AddUserToChannel(th.BasicUser, publicChannel, false)
	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: publicChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, targetUser.Username).Text
	assert.Equal(t, "", actual)

	// Try a private channel *without* permission.
	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: privateChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, targetUser.Username).Text
	assert.Equal(t, "api.command_remove.permission.app_error", actual)

	// Try a private channel *with* permission.
	th.App.AddUserToChannel(th.BasicUser, privateChannel, false)
	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: privateChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, targetUser.Username).Text
	assert.Equal(t, "", actual)

	// Try a group channel
	user1 := th.createUser()
	user2 := th.createUser()

	groupChannel := th.createGroupChannel(user1, user2)

	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: groupChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, user1.Username).Text
	assert.Equal(t, "api.command_remove.direct_group.app_error", actual)

	// Try a direct channel *with* being a member.
	directChannel := th.createDmChannel(user1)

	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: directChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, user1.Username).Text
	assert.Equal(t, "api.command_remove.direct_group.app_error", actual)

	// Try a public channel with a deactivated user.
	deactivatedUser := th.createUser()
	th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, deactivatedUser.ID, deactivatedUser.ID)
	th.App.AddUserToChannel(deactivatedUser, publicChannel, false)
	th.App.UpdateActive(th.Context, deactivatedUser, false)

	args = &model.CommandArgs{
		T:         func(s string, args ...interface{}) string { return s },
		ChannelID: publicChannel.ID,
		UserID:    th.BasicUser.ID,
	}

	actual = rp.DoCommand(th.App, th.Context, args, deactivatedUser.Username).Text
	assert.Equal(t, "api.command_remove.missing.app_error", actual)
}
