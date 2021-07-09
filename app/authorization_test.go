// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestCheckIfRolesGrantPermission(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	cases := []struct {
		roles        []string
		permissionID string
		shouldGrant  bool
	}{
		{[]string{model.SystemAdminRoleID}, model.PermissionManageSystem.ID, true},
		{[]string{model.SystemAdminRoleID}, "non-existent-permission", false},
		{[]string{model.ChannelUserRoleID}, model.PermissionReadChannel.ID, true},
		{[]string{model.ChannelUserRoleID}, model.PermissionManageSystem.ID, false},
		{[]string{model.SystemAdminRoleID, model.ChannelUserRoleID}, model.PermissionManageSystem.ID, true},
		{[]string{model.ChannelUserRoleID, model.SystemAdminRoleID}, model.PermissionManageSystem.ID, true},
		{[]string{model.TeamUserRoleID, model.TeamAdminRoleID}, model.PermissionManageSlashCommands.ID, true},
		{[]string{model.TeamAdminRoleID, model.TeamUserRoleID}, model.PermissionManageSlashCommands.ID, true},
	}

	for _, testcase := range cases {
		require.Equal(t, th.App.RolesGrantPermission(testcase.roles, testcase.permissionID), testcase.shouldGrant)
	}

}

func TestChannelRolesGrantPermission(t *testing.T) {
	testPermissionInheritance(t, func(t *testing.T, th *TestHelper, testData permissionInheritanceTestData) {
		require.Equal(t, testData.shouldHavePermission, th.App.RolesGrantPermission([]string{testData.channelRole.Name}, testData.permission.ID), "row: %+v\n", testData.truthTableRow)
	})
}

func TestHasPermissionToTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	assert.True(t, th.App.HasPermissionToTeam(th.BasicUser.ID, th.BasicTeam.ID, model.PermissionListTeamChannels))
	th.RemoveUserFromTeam(th.BasicUser, th.BasicTeam)
	assert.False(t, th.App.HasPermissionToTeam(th.BasicUser.ID, th.BasicTeam.ID, model.PermissionListTeamChannels))

	assert.True(t, th.App.HasPermissionToTeam(th.SystemAdminUser.ID, th.BasicTeam.ID, model.PermissionListTeamChannels))
	th.LinkUserToTeam(th.SystemAdminUser, th.BasicTeam)
	assert.True(t, th.App.HasPermissionToTeam(th.SystemAdminUser.ID, th.BasicTeam.ID, model.PermissionListTeamChannels))
	th.RemovePermissionFromRole(model.PermissionListTeamChannels.ID, model.TeamUserRoleID)
	assert.True(t, th.App.HasPermissionToTeam(th.SystemAdminUser.ID, th.BasicTeam.ID, model.PermissionListTeamChannels))
	th.RemoveUserFromTeam(th.SystemAdminUser, th.BasicTeam)
	assert.True(t, th.App.HasPermissionToTeam(th.SystemAdminUser.ID, th.BasicTeam.ID, model.PermissionListTeamChannels))
}

func TestSessionHasPermissionToChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	session := model.Session{
		UserID: th.BasicUser.ID,
	}

	t.Run("basic user can access basic channel", func(t *testing.T) {
		assert.True(t, th.App.SessionHasPermissionToChannel(session, th.BasicChannel.ID, model.PermissionAddReaction))
	})

	t.Run("does not panic if fetching channel causes an error", func(t *testing.T) {
		// Regression test for MM-29812
		// Mock the channel store so getting the channel returns with an error, as per the bug report.
		mockStore := mocks.Store{}
		mockChannelStore := mocks.ChannelStore{}
		mockChannelStore.On("Get", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("arbitrary error"))
		mockChannelStore.On("GetAllChannelMembersForUser", mock.Anything, mock.Anything, mock.Anything).Return(th.App.Srv().Store.Channel().GetAllChannelMembersForUser(th.BasicUser.ID, false, false))
		mockChannelStore.On("ClearCaches").Return()
		mockStore.On("Channel").Return(&mockChannelStore)
		mockStore.On("FileInfo").Return(th.App.Srv().Store.FileInfo())
		mockStore.On("License").Return(th.App.Srv().Store.License())
		mockStore.On("Post").Return(th.App.Srv().Store.Post())
		mockStore.On("Role").Return(th.App.Srv().Store.Role())
		mockStore.On("System").Return(th.App.Srv().Store.System())
		mockStore.On("Team").Return(th.App.Srv().Store.Team())
		mockStore.On("User").Return(th.App.Srv().Store.User())
		mockStore.On("Webhook").Return(th.App.Srv().Store.Webhook())
		mockStore.On("Close").Return(nil)
		th.App.Srv().Store = &mockStore

		// If there's an error returned from the GetChannel call the code should continue to cascade and since there
		// are no session level permissions in this test case, the permission should be denied.
		assert.False(t, th.App.SessionHasPermissionToChannel(session, th.BasicUser.ID, model.PermissionAddReaction))
	})
}

func TestHasPermissionToCategory(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	session, err := th.App.CreateSession(&model.Session{UserID: th.BasicUser.ID, Props: model.StringMap{}})
	require.Nil(t, err)

	categories, err := th.App.GetSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID)
	require.Nil(t, err)

	_, err = th.App.GetSession(session.Token)
	require.Nil(t, err)
	require.True(t, th.App.SessionHasPermissionToCategory(*session, th.BasicUser.ID, th.BasicTeam.ID, categories.Order[0]))

	categories2, err := th.App.GetSidebarCategories(th.BasicUser2.ID, th.BasicTeam.ID)
	require.Nil(t, err)
	require.False(t, th.App.SessionHasPermissionToCategory(*session, th.BasicUser.ID, th.BasicTeam.ID, categories2.Order[0]))
}
