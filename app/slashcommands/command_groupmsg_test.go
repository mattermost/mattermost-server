// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
)

func TestGroupMsgUsernames(t *testing.T) {
	assert := assert.New(t)

	users, parsedMessage := groupMsgUsernames("")
	assert.Len(users, 0)
	assert.Empty(parsedMessage)

	users, parsedMessage = groupMsgUsernames("test")
	assert.Len(users, 1)
	assert.Empty(parsedMessage)

	users, parsedMessage = groupMsgUsernames("test1, test2, test3 , test4")
	assert.Len(users, 4)
	assert.Empty(parsedMessage)

	users, parsedMessage = groupMsgUsernames("test1, test2 message with spaces")
	assert.Len(users, 2)
	assert.Equal(parsedMessage, "message with spaces", "error parsing message")

	users, parsedMessage = groupMsgUsernames("test1, test2 message with, comma")
	assert.Len(users, 2)
	assert.Equal(parsedMessage, "message with, comma", "error parsing messages with comma")

	users, parsedMessage = groupMsgUsernames("test1,,,test2")
	assert.Len(users, 2)
	assert.Empty(parsedMessage)

	users, parsedMessage = groupMsgUsernames("    test1,       test2     other message         ")
	assert.Len(users, 2)
	assert.Equal(parsedMessage, "other message", "error parsing strange usage of spaces")

	users, _ = groupMsgUsernames("    test1,       test2,,123,@321,+123")
	assert.Len(users, 5)
	assert.Equal(users[0], "test1")
	assert.Equal(users[1], "test2")
	assert.Equal(users[2], "123")
	assert.Equal(users[3], "321")
	assert.Equal(users[4], "+123")
	assert.Equal(parsedMessage, "other message", "error parsing different types of users")
}

func TestGroupMsgProvider(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	user3 := th.createUser()
	targetUsers := "@" + th.BasicUser2.Username + ",@" + user3.Username + " "

	team := th.createTeam()
	th.linkUserToTeam(th.BasicUser, team)
	cmd := &groupmsgProvider{}

	th.removePermissionFromRole(model.PermissionCreateGroupChannel.ID, model.SystemUserRoleID)

	t.Run("Check without permission to create a GM channel.", func(t *testing.T) {
		resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
			T:       i18n.IdentityTfunc(),
			SiteURL: "http://test.url",
			TeamID:  team.ID,
			UserID:  th.BasicUser.ID,
		}, targetUsers+"hello")

		assert.Equal(t, "api.command_groupmsg.permission.app_error", resp.Text)
		assert.Equal(t, "", resp.GotoLocation)
	})

	th.addPermissionToRole(model.PermissionCreateGroupChannel.ID, model.SystemUserRoleID)

	t.Run("Check without permissions to view a user in the list.", func(t *testing.T) {
		th.removePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
		defer th.addPermissionToRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
		resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
			T:       i18n.IdentityTfunc(),
			SiteURL: "http://test.url",
			TeamID:  team.ID,
			UserID:  th.BasicUser.ID,
		}, targetUsers+"hello")

		assert.Equal(t, "api.command_groupmsg.invalid_user.app_error", resp.Text)
		assert.Equal(t, "", resp.GotoLocation)
	})

	t.Run("Check with permission to create a GM channel.", func(t *testing.T) {
		resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
			T:       i18n.IdentityTfunc(),
			SiteURL: "http://test.url",
			TeamID:  team.ID,
			UserID:  th.BasicUser.ID,
		}, targetUsers+"hello")

		channelName := model.GetGroupNameFromUserIDs([]string{th.BasicUser.ID, th.BasicUser2.ID, user3.ID})
		assert.Equal(t, "", resp.Text)
		assert.Equal(t, "http://test.url/"+team.Name+"/channels/"+channelName, resp.GotoLocation)
	})

	t.Run("Check without permission to post to an existing GM channel.", func(t *testing.T) {
		resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
			T:       i18n.IdentityTfunc(),
			SiteURL: "http://test.url",
			TeamID:  team.ID,
			UserID:  th.BasicUser.ID,
		}, targetUsers+"hello")

		channelName := model.GetGroupNameFromUserIDs([]string{th.BasicUser.ID, th.BasicUser2.ID, user3.ID})
		assert.Equal(t, "", resp.Text)
		assert.Equal(t, "http://test.url/"+team.Name+"/channels/"+channelName, resp.GotoLocation)
	})
}
