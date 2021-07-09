// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestApiResctrictedViewMembers(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// Create first account for system admin
	_, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user0", Password: "test-password-0", Username: "test-user-0", Roles: model.SystemUserRoleID})
	require.Nil(t, err)

	user1, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user1", Password: "test-password-1", Username: "test-user-1", Roles: model.SystemUserRoleID})
	require.Nil(t, err)
	user2, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user2", Password: "test-password-2", Username: "test-user-2", Roles: model.SystemUserRoleID})
	require.Nil(t, err)
	user3, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user3", Password: "test-password-3", Username: "test-user-3", Roles: model.SystemUserRoleID})
	require.Nil(t, err)
	user4, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user4", Password: "test-password-4", Username: "test-user-4", Roles: model.SystemUserRoleID})
	require.Nil(t, err)
	user5, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user5", Password: "test-password-5", Username: "test-user-5", Roles: model.SystemUserRoleID})
	require.Nil(t, err)

	team1, err := th.App.CreateTeam(th.Context, &model.Team{DisplayName: "dn_" + model.NewID(), Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen})
	require.Nil(t, err)
	team2, err := th.App.CreateTeam(th.Context, &model.Team{DisplayName: "dn_" + model.NewID(), Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen})
	require.Nil(t, err)

	channel1, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "dn_" + model.NewID(), Name: "name_" + model.NewID(), Type: model.ChannelTypeOpen, TeamID: team1.ID, CreatorID: model.NewID()}, false)
	require.Nil(t, err)
	channel2, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "dn_" + model.NewID(), Name: "name_" + model.NewID(), Type: model.ChannelTypeOpen, TeamID: team1.ID, CreatorID: model.NewID()}, false)
	require.Nil(t, err)
	channel3, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "dn_" + model.NewID(), Name: "name_" + model.NewID(), Type: model.ChannelTypeOpen, TeamID: team2.ID, CreatorID: model.NewID()}, false)
	require.Nil(t, err)

	th.LinkUserToTeam(user1, team1)
	th.LinkUserToTeam(user2, team1)
	th.LinkUserToTeam(user3, team2)
	th.LinkUserToTeam(user4, team1)
	th.LinkUserToTeam(user4, team2)

	th.AddUserToChannel(user1, channel1)
	th.AddUserToChannel(user2, channel2)
	th.AddUserToChannel(user3, channel3)
	th.AddUserToChannel(user4, channel1)
	th.AddUserToChannel(user4, channel3)

	th.App.SetStatusOnline(user1.ID, true)
	th.App.SetStatusOnline(user2.ID, true)
	th.App.SetStatusOnline(user3.ID, true)
	th.App.SetStatusOnline(user4.ID, true)
	th.App.SetStatusOnline(user5.ID, true)

	_, resp := th.Client.Login(user1.Username, "test-password-1")
	CheckNoError(t, resp)

	t.Run("getUser", func(t *testing.T) {
		testCases := []struct {
			Name          string
			RestrictedTo  string
			UserID        string
			ExpectedError string
		}{
			{
				"Get visible user without restrictions",
				"",
				user5.ID,
				"",
			},
			{
				"Get not existing user without restrictions",
				"",
				model.NewID(),
				"app.user.missing_account.const",
			},
			{
				"Get not existing user with restrictions to teams",
				"teams",
				model.NewID(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to teams",
				"teams",
				user2.ID,
				"",
			},
			{
				"Get not visible user with restrictions to teams",
				"teams",
				user5.ID,
				"api.context.permissions.app_error",
			},
			{
				"Get not existing user with restrictions to channels",
				"channels",
				model.NewID(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to channels",
				"channels",
				user4.ID,
				"",
			},
			{
				"Get not visible user with restrictions to channels",
				"channels",
				user3.ID,
				"api.context.permissions.app_error",
			},
		}
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				if tc.RestrictedTo == "channels" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else if tc.RestrictedTo == "teams" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
				}

				_, resp := th.Client.GetUser(tc.UserID, "")
				require.Nil(t, err)
				if tc.ExpectedError != "" {
					CheckErrorMessage(t, resp, tc.ExpectedError)
				} else {
					CheckNoError(t, resp)
				}
			})
		}
	})

	t.Run("getUserByUsername", func(t *testing.T) {
		testCases := []struct {
			Name          string
			RestrictedTo  string
			Username      string
			ExpectedError string
		}{
			{
				"Get visible user without restrictions",
				"",
				user5.Username,
				"",
			},
			{
				"Get not existing user without restrictions",
				"",
				model.NewID(),
				"app.user.get_by_username.app_error",
			},
			{
				"Get not existing user with restrictions to teams",
				"teams",
				model.NewID(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to teams",
				"teams",
				user2.Username,
				"",
			},
			{
				"Get not visible user with restrictions to teams",
				"teams",
				user5.Username,
				"api.context.permissions.app_error",
			},
			{
				"Get not existing user with restrictions to channels",
				"channels",
				model.NewID(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to channels",
				"channels",
				user4.Username,
				"",
			},
			{
				"Get not visible user with restrictions to channels",
				"channels",
				user3.Username,
				"api.context.permissions.app_error",
			},
		}
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				if tc.RestrictedTo == "channels" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else if tc.RestrictedTo == "teams" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
				}

				_, resp := th.Client.GetUserByUsername(tc.Username, "")
				require.Nil(t, err)
				if tc.ExpectedError != "" {
					CheckErrorMessage(t, resp, tc.ExpectedError)
				} else {
					CheckNoError(t, resp)
				}
			})
		}
	})

	t.Run("getUserByEmail", func(t *testing.T) {
		testCases := []struct {
			Name          string
			RestrictedTo  string
			Email         string
			ExpectedError string
		}{
			{
				"Get visible user without restrictions",
				"",
				user5.Email,
				"",
			},
			{
				"Get not existing user without restrictions",
				"",
				th.GenerateTestEmail(),
				"app.user.missing_account.const",
			},
			{
				"Get not existing user with restrictions to teams",
				"teams",
				th.GenerateTestEmail(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to teams",
				"teams",
				user2.Email,
				"",
			},
			{
				"Get not visible user with restrictions to teams",
				"teams",
				user5.Email,
				"api.context.permissions.app_error",
			},
			{
				"Get not existing user with restrictions to channels",
				"channels",
				th.GenerateTestEmail(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to channels",
				"channels",
				user4.Email,
				"",
			},
			{
				"Get not visible user with restrictions to channels",
				"channels",
				user3.Email,
				"api.context.permissions.app_error",
			},
		}
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				if tc.RestrictedTo == "channels" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else if tc.RestrictedTo == "teams" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
				}

				_, resp := th.Client.GetUserByEmail(tc.Email, "")
				require.Nil(t, err)
				if tc.ExpectedError != "" {
					CheckErrorMessage(t, resp, tc.ExpectedError)
				} else {
					CheckNoError(t, resp)
				}
			})
		}
	})

	t.Run("getDefaultProfileImage", func(t *testing.T) {
		testCases := []struct {
			Name          string
			RestrictedTo  string
			UserID        string
			ExpectedError string
		}{
			{
				"Get visible user without restrictions",
				"",
				user5.ID,
				"",
			},
			{
				"Get not existing user without restrictions",
				"",
				model.NewID(),
				"app.user.missing_account.const",
			},
			{
				"Get not existing user with restrictions to teams",
				"teams",
				model.NewID(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to teams",
				"teams",
				user2.ID,
				"",
			},
			{
				"Get not visible user with restrictions to teams",
				"teams",
				user5.ID,
				"api.context.permissions.app_error",
			},
			{
				"Get not existing user with restrictions to channels",
				"channels",
				model.NewID(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to channels",
				"channels",
				user4.ID,
				"",
			},
			{
				"Get not visible user with restrictions to channels",
				"channels",
				user3.ID,
				"api.context.permissions.app_error",
			},
		}
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				if tc.RestrictedTo == "channels" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else if tc.RestrictedTo == "teams" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
				}

				_, resp := th.Client.GetDefaultProfileImage(tc.UserID)
				require.Nil(t, err)
				if tc.ExpectedError != "" {
					CheckErrorMessage(t, resp, tc.ExpectedError)
				} else {
					CheckNoError(t, resp)
				}
			})
		}
	})

	t.Run("getProfileImage", func(t *testing.T) {
		testCases := []struct {
			Name          string
			RestrictedTo  string
			UserID        string
			ExpectedError string
		}{
			{
				"Get visible user without restrictions",
				"",
				user5.ID,
				"",
			},
			{
				"Get not existing user without restrictions",
				"",
				model.NewID(),
				"app.user.missing_account.const",
			},
			{
				"Get not existing user with restrictions to teams",
				"teams",
				model.NewID(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to teams",
				"teams",
				user2.ID,
				"",
			},
			{
				"Get not visible user with restrictions to teams",
				"teams",
				user5.ID,
				"api.context.permissions.app_error",
			},
			{
				"Get not existing user with restrictions to channels",
				"channels",
				model.NewID(),
				"api.context.permissions.app_error",
			},
			{
				"Get visible user with restrictions to channels",
				"channels",
				user4.ID,
				"",
			},
			{
				"Get not visible user with restrictions to channels",
				"channels",
				user3.ID,
				"api.context.permissions.app_error",
			},
		}
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				if tc.RestrictedTo == "channels" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else if tc.RestrictedTo == "teams" {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
				} else {
					th.RemovePermissionFromRole(model.PermissionViewMembers.ID, model.TeamUserRoleID)
					th.AddPermissionToRole(model.PermissionViewMembers.ID, model.SystemUserRoleID)
				}

				_, resp := th.Client.GetProfileImage(tc.UserID, "")
				require.Nil(t, err)
				if tc.ExpectedError != "" {
					CheckErrorMessage(t, resp, tc.ExpectedError)
				} else {
					CheckNoError(t, resp)
				}
			})
		}
	})
}
