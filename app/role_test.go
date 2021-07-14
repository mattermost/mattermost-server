// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"encoding/csv"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type permissionInheritanceTestData struct {
	channelRole          *model.Role
	permission           *model.Permission
	shouldHavePermission bool
	channel              *model.Channel
	higherScopedRole     *model.Role
	truthTableRow        []string
}

func TestGetRolesByNames(t *testing.T) {
	testPermissionInheritance(t, func(t *testing.T, th *TestHelper, testData permissionInheritanceTestData) {
		actualRoles, err := th.App.GetRolesByNames([]string{testData.channelRole.Name})
		require.Nil(t, err)
		require.Len(t, actualRoles, 1)

		actualRole := actualRoles[0]
		require.NotNil(t, actualRole)
		require.Equal(t, testData.channelRole.Name, actualRole.Name)

		require.Equal(t, testData.shouldHavePermission, utils.StringInSlice(testData.permission.Id, actualRole.Permissions))
	})
}

func TestGetRoleByName(t *testing.T) {
	testPermissionInheritance(t, func(t *testing.T, th *TestHelper, testData permissionInheritanceTestData) {
		actualRole, err := th.App.GetRoleByName(context.Background(), testData.channelRole.Name)
		require.Nil(t, err)
		require.NotNil(t, actualRole)
		require.Equal(t, testData.channelRole.Name, actualRole.Name)
		require.Equal(t, testData.shouldHavePermission, utils.StringInSlice(testData.permission.Id, actualRole.Permissions), "row: %+v", testData.truthTableRow)
	})
}

func TestGetRoleByID(t *testing.T) {
	testPermissionInheritance(t, func(t *testing.T, th *TestHelper, testData permissionInheritanceTestData) {
		actualRole, err := th.App.GetRole(testData.channelRole.Id)
		require.Nil(t, err)
		require.NotNil(t, actualRole)
		require.Equal(t, testData.channelRole.Id, actualRole.Id)
		require.Equal(t, testData.shouldHavePermission, utils.StringInSlice(testData.permission.Id, actualRole.Permissions), "row: %+v", testData.truthTableRow)
	})
}

// testPermissionInheritance tests 48 combinations of scheme, permission, role data.
func testPermissionInheritance(t *testing.T, testCallback func(t *testing.T, th *TestHelper, testData permissionInheritanceTestData)) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense(""))
	th.App.SetPhase2PermissionsMigrationStatus(true)

	permissionsDefault := []string{
		model.PERMISSION_MANAGE_CHANNEL_ROLES.Id,
		model.PERMISSION_MANAGE_PUBLIC_CHANNEL_MEMBERS.Id,
	}

	// Defer resetting the system scheme permissions
	systemSchemeRoles, err := th.App.GetRolesByNames([]string{
		model.CHANNEL_GUEST_ROLE_ID,
		model.CHANNEL_USER_ROLE_ID,
		model.CHANNEL_ADMIN_ROLE_ID,
	})
	require.Nil(t, err)
	require.Len(t, systemSchemeRoles, 3)

	// defer resetting the system role permissions
	for _, systemRole := range systemSchemeRoles {
		defer th.App.PatchRole(systemRole, &model.RolePatch{
			Permissions: &systemRole.Permissions,
		})
	}

	// Make a channel scheme, clear its permissions
	channelScheme, err := th.App.CreateScheme(&model.Scheme{
		Name:        model.NewId(),
		DisplayName: model.NewId(),
		Scope:       model.SCHEME_SCOPE_CHANNEL,
	})
	require.Nil(t, err)
	defer th.App.DeleteScheme(channelScheme.Id)

	team := th.CreateTeam()
	defer th.App.PermanentDeleteTeamId(team.Id)

	// Make a channel
	channel := th.CreateChannel(team)
	defer th.App.PermanentDeleteChannel(channel)

	// Set the channel scheme
	channel.SchemeId = &channelScheme.Id
	channel, err = th.App.UpdateChannelScheme(channel)
	require.Nil(t, err)

	// Get the truth table from CSV
	file, e := os.Open("tests/channel-role-has-permission.csv")
	require.NoError(t, e)
	defer file.Close()

	b, e := ioutil.ReadAll(file)
	require.NoError(t, e)

	r := csv.NewReader(strings.NewReader(string(b)))
	records, e := r.ReadAll()
	require.NoError(t, e)

	test := func(higherScopedGuest, higherScopedUser, higherScopedAdmin string) {
		for _, roleNameUnderTest := range []string{higherScopedGuest, higherScopedUser, higherScopedAdmin} {
			for i, row := range records {
				// skip csv header
				if i == 0 {
					continue
				}

				higherSchemeHasPermission, e := strconv.ParseBool(row[0])
				require.NoError(t, e)

				permissionIsModerated, e := strconv.ParseBool(row[1])
				require.NoError(t, e)

				channelSchemeHasPermission, e := strconv.ParseBool(row[2])
				require.NoError(t, e)

				channelRoleIsChannelAdmin, e := strconv.ParseBool(row[3])
				require.NoError(t, e)

				shouldHavePermission, e := strconv.ParseBool(row[4])
				require.NoError(t, e)

				// skip some invalid combinations because of the outer loop iterating all 3 channel roles
				if (channelRoleIsChannelAdmin && roleNameUnderTest != higherScopedAdmin) || (!channelRoleIsChannelAdmin && roleNameUnderTest == higherScopedAdmin) {
					continue
				}

				// select the permission to test (moderated or non-moderated)
				var permission *model.Permission
				if permissionIsModerated {
					permission = model.PERMISSION_CREATE_POST // moderated
				} else {
					permission = model.PERMISSION_READ_CHANNEL // non-moderated
				}

				// add or remove the permission from the higher-scoped scheme
				higherScopedRole, testErr := th.App.GetRoleByName(context.Background(), roleNameUnderTest)
				require.Nil(t, testErr)

				var higherScopedPermissions []string
				if higherSchemeHasPermission {
					higherScopedPermissions = []string{permission.Id}
				} else {
					higherScopedPermissions = permissionsDefault
				}
				higherScopedRole, testErr = th.App.PatchRole(higherScopedRole, &model.RolePatch{Permissions: &higherScopedPermissions})
				require.Nil(t, testErr)

				// get channel role
				var channelRoleName string
				switch roleNameUnderTest {
				case higherScopedGuest:
					channelRoleName = channelScheme.DefaultChannelGuestRole
				case higherScopedUser:
					channelRoleName = channelScheme.DefaultChannelUserRole
				case higherScopedAdmin:
					channelRoleName = channelScheme.DefaultChannelAdminRole
				}
				channelRole, testErr := th.App.GetRoleByName(context.Background(), channelRoleName)
				require.Nil(t, testErr)

				// add or remove the permission from the channel scheme
				var channelSchemePermissions []string
				if channelSchemeHasPermission {
					channelSchemePermissions = []string{permission.Id}
				} else {
					channelSchemePermissions = permissionsDefault
				}
				channelRole, testErr = th.App.PatchRole(channelRole, &model.RolePatch{Permissions: &channelSchemePermissions})
				require.Nil(t, testErr)

				testCallback(t, th, permissionInheritanceTestData{
					channelRole:          channelRole,
					permission:           permission,
					shouldHavePermission: shouldHavePermission,
					channel:              channel,
					higherScopedRole:     higherScopedRole,
					truthTableRow:        row,
				})
			}
		}
	}

	// test 24 combinations where the higher-scoped scheme is the SYSTEM scheme
	test(model.CHANNEL_GUEST_ROLE_ID, model.CHANNEL_USER_ROLE_ID, model.CHANNEL_ADMIN_ROLE_ID)

	// create a team scheme
	teamScheme, err := th.App.CreateScheme(&model.Scheme{
		Name:        model.NewId(),
		DisplayName: model.NewId(),
		Scope:       model.SCHEME_SCOPE_TEAM,
	})
	require.Nil(t, err)
	defer th.App.DeleteScheme(teamScheme.Id)

	// assign the scheme to the team
	team.SchemeId = &teamScheme.Id
	_, err = th.App.UpdateTeamScheme(team)
	require.Nil(t, err)

	// test 24 combinations where the higher-scoped scheme is a TEAM scheme
	test(teamScheme.DefaultChannelGuestRole, teamScheme.DefaultChannelUserRole, teamScheme.DefaultChannelAdminRole)
}
