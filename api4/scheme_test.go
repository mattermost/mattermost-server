// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestCreateScheme(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

	th.App.SetPhase2PermissionsMigrationStatus(true)

	// Basic test of creating a team scheme.
	scheme1 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}

	s1, r1 := th.SystemAdminClient.CreateScheme(scheme1)
	CheckNoError(t, r1)

	assert.Equal(t, s1.DisplayName, scheme1.DisplayName)
	assert.Equal(t, s1.Name, scheme1.Name)
	assert.Equal(t, s1.Description, scheme1.Description)
	assert.NotZero(t, s1.CreateAt)
	assert.Equal(t, s1.CreateAt, s1.UpdateAt)
	assert.Zero(t, s1.DeleteAt)
	assert.Equal(t, s1.Scope, scheme1.Scope)
	assert.NotZero(t, len(s1.DefaultTeamAdminRole))
	assert.NotZero(t, len(s1.DefaultTeamUserRole))
	assert.NotZero(t, len(s1.DefaultTeamGuestRole))
	assert.NotZero(t, len(s1.DefaultChannelAdminRole))
	assert.NotZero(t, len(s1.DefaultChannelUserRole))
	assert.NotZero(t, len(s1.DefaultChannelGuestRole))

	// Check the default roles have been created.
	_, roleRes1 := th.SystemAdminClient.GetRoleByName(s1.DefaultTeamAdminRole)
	CheckNoError(t, roleRes1)
	_, roleRes2 := th.SystemAdminClient.GetRoleByName(s1.DefaultTeamUserRole)
	CheckNoError(t, roleRes2)
	_, roleRes3 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelAdminRole)
	CheckNoError(t, roleRes3)
	_, roleRes4 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelUserRole)
	CheckNoError(t, roleRes4)
	_, roleRes5 := th.SystemAdminClient.GetRoleByName(s1.DefaultTeamGuestRole)
	CheckNoError(t, roleRes5)
	_, roleRes6 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelGuestRole)
	CheckNoError(t, roleRes6)

	// Basic Test of a Channel scheme.
	scheme2 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeChannel,
	}

	s2, r2 := th.SystemAdminClient.CreateScheme(scheme2)
	CheckNoError(t, r2)

	assert.Equal(t, s2.DisplayName, scheme2.DisplayName)
	assert.Equal(t, s2.Name, scheme2.Name)
	assert.Equal(t, s2.Description, scheme2.Description)
	assert.NotZero(t, s2.CreateAt)
	assert.Equal(t, s2.CreateAt, s2.UpdateAt)
	assert.Zero(t, s2.DeleteAt)
	assert.Equal(t, s2.Scope, scheme2.Scope)
	assert.Zero(t, len(s2.DefaultTeamAdminRole))
	assert.Zero(t, len(s2.DefaultTeamUserRole))
	assert.Zero(t, len(s2.DefaultTeamGuestRole))
	assert.NotZero(t, len(s2.DefaultChannelAdminRole))
	assert.NotZero(t, len(s2.DefaultChannelUserRole))
	assert.NotZero(t, len(s2.DefaultChannelGuestRole))

	// Check the default roles have been created.
	_, roleRes7 := th.SystemAdminClient.GetRoleByName(s2.DefaultChannelAdminRole)
	CheckNoError(t, roleRes7)
	_, roleRes8 := th.SystemAdminClient.GetRoleByName(s2.DefaultChannelUserRole)
	CheckNoError(t, roleRes8)
	_, roleRes9 := th.SystemAdminClient.GetRoleByName(s2.DefaultChannelGuestRole)
	CheckNoError(t, roleRes9)

	// Try and create a scheme with an invalid scope.
	scheme3 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.NewID(),
	}

	_, r3 := th.SystemAdminClient.CreateScheme(scheme3)
	CheckBadRequestStatus(t, r3)

	// Try and create a scheme with an invalid display name.
	scheme4 := &model.Scheme{
		DisplayName: strings.Repeat(model.NewID(), 100),
		Name:        "Name",
		Description: model.NewID(),
		Scope:       model.NewID(),
	}
	_, r4 := th.SystemAdminClient.CreateScheme(scheme4)
	CheckBadRequestStatus(t, r4)

	// Try and create a scheme with an invalid name.
	scheme8 := &model.Scheme{
		DisplayName: "DisplayName",
		Name:        strings.Repeat(model.NewID(), 100),
		Description: model.NewID(),
		Scope:       model.NewID(),
	}
	_, r8 := th.SystemAdminClient.CreateScheme(scheme8)
	CheckBadRequestStatus(t, r8)

	// Try and create a scheme without the appropriate permissions.
	scheme5 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}
	_, r5 := th.Client.CreateScheme(scheme5)
	CheckForbiddenStatus(t, r5)

	// Try and create a scheme without a license.
	th.App.Srv().SetLicense(nil)
	scheme6 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}
	_, r6 := th.SystemAdminClient.CreateScheme(scheme6)
	CheckNotImplementedStatus(t, r6)

	th.App.SetPhase2PermissionsMigrationStatus(false)

	th.LoginSystemAdmin()
	th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

	scheme7 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}
	_, r7 := th.SystemAdminClient.CreateScheme(scheme7)
	CheckNotImplementedStatus(t, r7)
}

func TestGetScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

	// Basic test of creating a team scheme.
	scheme1 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}

	th.App.SetPhase2PermissionsMigrationStatus(true)

	s1, r1 := th.SystemAdminClient.CreateScheme(scheme1)
	CheckNoError(t, r1)

	assert.Equal(t, s1.DisplayName, scheme1.DisplayName)
	assert.Equal(t, s1.Name, scheme1.Name)
	assert.Equal(t, s1.Description, scheme1.Description)
	assert.NotZero(t, s1.CreateAt)
	assert.Equal(t, s1.CreateAt, s1.UpdateAt)
	assert.Zero(t, s1.DeleteAt)
	assert.Equal(t, s1.Scope, scheme1.Scope)
	assert.NotZero(t, len(s1.DefaultTeamAdminRole))
	assert.NotZero(t, len(s1.DefaultTeamUserRole))
	assert.NotZero(t, len(s1.DefaultTeamGuestRole))
	assert.NotZero(t, len(s1.DefaultChannelAdminRole))
	assert.NotZero(t, len(s1.DefaultChannelUserRole))
	assert.NotZero(t, len(s1.DefaultChannelGuestRole))

	s2, r2 := th.SystemAdminClient.GetScheme(s1.ID)
	CheckNoError(t, r2)

	assert.Equal(t, s1, s2)

	_, r3 := th.SystemAdminClient.GetScheme(model.NewID())
	CheckNotFoundStatus(t, r3)

	_, r4 := th.SystemAdminClient.GetScheme("12345")
	CheckBadRequestStatus(t, r4)

	th.SystemAdminClient.Logout()
	_, r5 := th.SystemAdminClient.GetScheme(s1.ID)
	CheckUnauthorizedStatus(t, r5)

	th.SystemAdminClient.Login(th.SystemAdminUser.Username, th.SystemAdminUser.Password)
	th.App.Srv().SetLicense(nil)
	_, r6 := th.SystemAdminClient.GetScheme(s1.ID)
	CheckNoError(t, r6)

	_, r7 := th.Client.GetScheme(s1.ID)
	CheckForbiddenStatus(t, r7)

	th.App.SetPhase2PermissionsMigrationStatus(false)

	_, r8 := th.SystemAdminClient.GetScheme(s1.ID)
	CheckNotImplementedStatus(t, r8)
}

func TestGetSchemes(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

	scheme1 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}

	scheme2 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeChannel,
	}

	th.App.SetPhase2PermissionsMigrationStatus(true)

	_, r1 := th.SystemAdminClient.CreateScheme(scheme1)
	CheckNoError(t, r1)
	_, r2 := th.SystemAdminClient.CreateScheme(scheme2)
	CheckNoError(t, r2)

	l3, r3 := th.SystemAdminClient.GetSchemes("", 0, 100)
	CheckNoError(t, r3)

	assert.NotZero(t, len(l3))

	l4, r4 := th.SystemAdminClient.GetSchemes("team", 0, 100)
	CheckNoError(t, r4)

	for _, s := range l4 {
		assert.Equal(t, "team", s.Scope)
	}

	l5, r5 := th.SystemAdminClient.GetSchemes("channel", 0, 100)
	CheckNoError(t, r5)

	for _, s := range l5 {
		assert.Equal(t, "channel", s.Scope)
	}

	_, r6 := th.SystemAdminClient.GetSchemes("asdf", 0, 100)
	CheckBadRequestStatus(t, r6)

	th.Client.Logout()
	_, r7 := th.Client.GetSchemes("", 0, 100)
	CheckUnauthorizedStatus(t, r7)

	th.Client.Login(th.BasicUser.Username, th.BasicUser.Password)
	_, r8 := th.Client.GetSchemes("", 0, 100)
	CheckForbiddenStatus(t, r8)

	th.App.SetPhase2PermissionsMigrationStatus(false)

	_, r9 := th.SystemAdminClient.GetSchemes("", 0, 100)
	CheckNotImplementedStatus(t, r9)
}

func TestGetTeamsForScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

	th.App.SetPhase2PermissionsMigrationStatus(true)

	scheme1 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}
	scheme1, r1 := th.SystemAdminClient.CreateScheme(scheme1)
	CheckNoError(t, r1)

	team1 := &model.Team{
		Name:        GenerateTestUsername(),
		DisplayName: "A Test Team",
		Type:        model.TeamOpen,
	}

	team1, err := th.App.Srv().Store.Team().Save(team1)
	require.NoError(t, err)

	l2, r2 := th.SystemAdminClient.GetTeamsForScheme(scheme1.ID, 0, 100)
	CheckNoError(t, r2)
	assert.Zero(t, len(l2))

	team1.SchemeID = &scheme1.ID
	team1, err = th.App.Srv().Store.Team().Update(team1)
	assert.NoError(t, err)

	l3, r3 := th.SystemAdminClient.GetTeamsForScheme(scheme1.ID, 0, 100)
	CheckNoError(t, r3)
	assert.Len(t, l3, 1)
	assert.Equal(t, team1.ID, l3[0].ID)

	team2 := &model.Team{
		Name:        GenerateTestUsername(),
		DisplayName: "B Test Team",
		Type:        model.TeamOpen,
		SchemeID:    &scheme1.ID,
	}
	team2, err = th.App.Srv().Store.Team().Save(team2)
	require.NoError(t, err)

	l4, r4 := th.SystemAdminClient.GetTeamsForScheme(scheme1.ID, 0, 100)
	CheckNoError(t, r4)
	assert.Len(t, l4, 2)
	assert.Equal(t, team1.ID, l4[0].ID)
	assert.Equal(t, team2.ID, l4[1].ID)

	l5, r5 := th.SystemAdminClient.GetTeamsForScheme(scheme1.ID, 1, 1)
	CheckNoError(t, r5)
	assert.Len(t, l5, 1)
	assert.Equal(t, team2.ID, l5[0].ID)

	// Check various error cases.
	_, ri1 := th.SystemAdminClient.GetTeamsForScheme(model.NewID(), 0, 100)
	CheckNotFoundStatus(t, ri1)

	_, ri2 := th.SystemAdminClient.GetTeamsForScheme("", 0, 100)
	CheckBadRequestStatus(t, ri2)

	th.Client.Logout()
	_, ri3 := th.Client.GetTeamsForScheme(model.NewID(), 0, 100)
	CheckUnauthorizedStatus(t, ri3)

	th.Client.Login(th.BasicUser.Username, th.BasicUser.Password)
	_, ri4 := th.Client.GetTeamsForScheme(model.NewID(), 0, 100)
	CheckForbiddenStatus(t, ri4)

	scheme2 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeChannel,
	}
	scheme2, rs2 := th.SystemAdminClient.CreateScheme(scheme2)
	CheckNoError(t, rs2)

	_, ri5 := th.SystemAdminClient.GetTeamsForScheme(scheme2.ID, 0, 100)
	CheckBadRequestStatus(t, ri5)

	th.App.SetPhase2PermissionsMigrationStatus(false)

	_, ri6 := th.SystemAdminClient.GetTeamsForScheme(scheme1.ID, 0, 100)
	CheckNotImplementedStatus(t, ri6)
}

func TestGetChannelsForScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

	th.App.SetPhase2PermissionsMigrationStatus(true)

	scheme1 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeChannel,
	}
	scheme1, r1 := th.SystemAdminClient.CreateScheme(scheme1)
	CheckNoError(t, r1)

	channel1 := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "A Name",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}

	channel1, errCh := th.App.Srv().Store.Channel().Save(channel1, 1000000)
	assert.NoError(t, errCh)

	l2, r2 := th.SystemAdminClient.GetChannelsForScheme(scheme1.ID, 0, 100)
	CheckNoError(t, r2)
	assert.Zero(t, len(l2))

	channel1.SchemeID = &scheme1.ID
	channel1, err := th.App.Srv().Store.Channel().Update(channel1)
	assert.NoError(t, err)

	l3, r3 := th.SystemAdminClient.GetChannelsForScheme(scheme1.ID, 0, 100)
	CheckNoError(t, r3)
	assert.Len(t, l3, 1)
	assert.Equal(t, channel1.ID, l3[0].ID)

	channel2 := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "B Name",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
		SchemeID:    &scheme1.ID,
	}
	channel2, nErr := th.App.Srv().Store.Channel().Save(channel2, 1000000)
	assert.NoError(t, nErr)

	l4, r4 := th.SystemAdminClient.GetChannelsForScheme(scheme1.ID, 0, 100)
	CheckNoError(t, r4)
	assert.Len(t, l4, 2)
	assert.Equal(t, channel1.ID, l4[0].ID)
	assert.Equal(t, channel2.ID, l4[1].ID)

	l5, r5 := th.SystemAdminClient.GetChannelsForScheme(scheme1.ID, 1, 1)
	CheckNoError(t, r5)
	assert.Len(t, l5, 1)
	assert.Equal(t, channel2.ID, l5[0].ID)

	// Check various error cases.
	_, ri1 := th.SystemAdminClient.GetChannelsForScheme(model.NewID(), 0, 100)
	CheckNotFoundStatus(t, ri1)

	_, ri2 := th.SystemAdminClient.GetChannelsForScheme("", 0, 100)
	CheckBadRequestStatus(t, ri2)

	th.Client.Logout()
	_, ri3 := th.Client.GetChannelsForScheme(model.NewID(), 0, 100)
	CheckUnauthorizedStatus(t, ri3)

	th.Client.Login(th.BasicUser.Username, th.BasicUser.Password)
	_, ri4 := th.Client.GetChannelsForScheme(model.NewID(), 0, 100)
	CheckForbiddenStatus(t, ri4)

	scheme2 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}
	scheme2, rs2 := th.SystemAdminClient.CreateScheme(scheme2)
	CheckNoError(t, rs2)

	_, ri5 := th.SystemAdminClient.GetChannelsForScheme(scheme2.ID, 0, 100)
	CheckBadRequestStatus(t, ri5)

	th.App.SetPhase2PermissionsMigrationStatus(false)

	_, ri6 := th.SystemAdminClient.GetChannelsForScheme(scheme1.ID, 0, 100)
	CheckNotImplementedStatus(t, ri6)
}

func TestPatchScheme(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

	th.App.SetPhase2PermissionsMigrationStatus(true)

	// Basic test of creating a team scheme.
	scheme1 := &model.Scheme{
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Description: model.NewID(),
		Scope:       model.SchemeScopeTeam,
	}

	s1, r1 := th.SystemAdminClient.CreateScheme(scheme1)
	CheckNoError(t, r1)

	assert.Equal(t, s1.DisplayName, scheme1.DisplayName)
	assert.Equal(t, s1.Name, scheme1.Name)
	assert.Equal(t, s1.Description, scheme1.Description)
	assert.NotZero(t, s1.CreateAt)
	assert.Equal(t, s1.CreateAt, s1.UpdateAt)
	assert.Zero(t, s1.DeleteAt)
	assert.Equal(t, s1.Scope, scheme1.Scope)
	assert.NotZero(t, len(s1.DefaultTeamAdminRole))
	assert.NotZero(t, len(s1.DefaultTeamUserRole))
	assert.NotZero(t, len(s1.DefaultTeamGuestRole))
	assert.NotZero(t, len(s1.DefaultChannelAdminRole))
	assert.NotZero(t, len(s1.DefaultChannelUserRole))
	assert.NotZero(t, len(s1.DefaultChannelGuestRole))

	s2, r2 := th.SystemAdminClient.GetScheme(s1.ID)
	CheckNoError(t, r2)

	assert.Equal(t, s1, s2)

	// Test with a valid patch.
	schemePatch := &model.SchemePatch{
		DisplayName: new(string),
		Name:        new(string),
		Description: new(string),
	}
	*schemePatch.DisplayName = model.NewID()
	*schemePatch.Name = model.NewID()
	*schemePatch.Description = model.NewID()

	s3, r3 := th.SystemAdminClient.PatchScheme(s2.ID, schemePatch)
	CheckNoError(t, r3)
	assert.Equal(t, s3.ID, s2.ID)
	assert.Equal(t, s3.DisplayName, *schemePatch.DisplayName)
	assert.Equal(t, s3.Name, *schemePatch.Name)
	assert.Equal(t, s3.Description, *schemePatch.Description)

	s4, r4 := th.SystemAdminClient.GetScheme(s3.ID)
	CheckNoError(t, r4)
	assert.Equal(t, s3, s4)

	// Test with a partial patch.
	*schemePatch.Name = model.NewID()
	*schemePatch.DisplayName = model.NewID()
	schemePatch.Description = nil

	s5, r5 := th.SystemAdminClient.PatchScheme(s4.ID, schemePatch)
	CheckNoError(t, r5)
	assert.Equal(t, s5.ID, s4.ID)
	assert.Equal(t, s5.DisplayName, *schemePatch.DisplayName)
	assert.Equal(t, s5.Name, *schemePatch.Name)
	assert.Equal(t, s5.Description, s4.Description)

	s6, r6 := th.SystemAdminClient.GetScheme(s5.ID)
	CheckNoError(t, r6)
	assert.Equal(t, s5, s6)

	// Test with invalid patch.
	*schemePatch.Name = strings.Repeat(model.NewID(), 20)
	_, r7 := th.SystemAdminClient.PatchScheme(s6.ID, schemePatch)
	CheckBadRequestStatus(t, r7)

	// Test with unknown ID.
	*schemePatch.Name = model.NewID()
	_, r8 := th.SystemAdminClient.PatchScheme(model.NewID(), schemePatch)
	CheckNotFoundStatus(t, r8)

	// Test with invalid ID.
	_, r9 := th.SystemAdminClient.PatchScheme("12345", schemePatch)
	CheckBadRequestStatus(t, r9)

	// Test without required permissions.
	_, r10 := th.Client.PatchScheme(s6.ID, schemePatch)
	CheckForbiddenStatus(t, r10)

	// Test without license.
	th.App.Srv().SetLicense(nil)
	_, r11 := th.SystemAdminClient.PatchScheme(s6.ID, schemePatch)
	CheckNotImplementedStatus(t, r11)

	th.App.SetPhase2PermissionsMigrationStatus(false)

	th.LoginSystemAdmin()
	th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

	_, r12 := th.SystemAdminClient.PatchScheme(s6.ID, schemePatch)
	CheckNotImplementedStatus(t, r12)
}

func TestDeleteScheme(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("ValidTeamScheme", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

		th.App.SetPhase2PermissionsMigrationStatus(true)

		// Create a team scheme.
		scheme1 := &model.Scheme{
			DisplayName: model.NewID(),
			Name:        model.NewID(),
			Description: model.NewID(),
			Scope:       model.SchemeScopeTeam,
		}

		s1, r1 := th.SystemAdminClient.CreateScheme(scheme1)
		CheckNoError(t, r1)

		// Retrieve the roles and check they are not deleted.
		role1, roleRes1 := th.SystemAdminClient.GetRoleByName(s1.DefaultTeamAdminRole)
		CheckNoError(t, roleRes1)
		role2, roleRes2 := th.SystemAdminClient.GetRoleByName(s1.DefaultTeamUserRole)
		CheckNoError(t, roleRes2)
		role3, roleRes3 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelAdminRole)
		CheckNoError(t, roleRes3)
		role4, roleRes4 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelUserRole)
		CheckNoError(t, roleRes4)
		role5, roleRes5 := th.SystemAdminClient.GetRoleByName(s1.DefaultTeamGuestRole)
		CheckNoError(t, roleRes5)
		role6, roleRes6 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelGuestRole)
		CheckNoError(t, roleRes6)

		assert.Zero(t, role1.DeleteAt)
		assert.Zero(t, role2.DeleteAt)
		assert.Zero(t, role3.DeleteAt)
		assert.Zero(t, role4.DeleteAt)
		assert.Zero(t, role5.DeleteAt)
		assert.Zero(t, role6.DeleteAt)

		// Make sure this scheme is in use by a team.
		team, err := th.App.Srv().Store.Team().Save(&model.Team{
			Name:        "zz" + model.NewID(),
			DisplayName: model.NewID(),
			Email:       model.NewID() + "@nowhere.com",
			Type:        model.TeamOpen,
			SchemeID:    &s1.ID,
		})
		require.NoError(t, err)

		// Delete the Scheme.
		_, r3 := th.SystemAdminClient.DeleteScheme(s1.ID)
		CheckNoError(t, r3)

		// Check the roles were deleted.
		role1, roleRes1 = th.SystemAdminClient.GetRoleByName(s1.DefaultTeamAdminRole)
		CheckNoError(t, roleRes1)
		role2, roleRes2 = th.SystemAdminClient.GetRoleByName(s1.DefaultTeamUserRole)
		CheckNoError(t, roleRes2)
		role3, roleRes3 = th.SystemAdminClient.GetRoleByName(s1.DefaultChannelAdminRole)
		CheckNoError(t, roleRes3)
		role4, roleRes4 = th.SystemAdminClient.GetRoleByName(s1.DefaultChannelUserRole)
		CheckNoError(t, roleRes4)
		role5, roleRes5 = th.SystemAdminClient.GetRoleByName(s1.DefaultTeamGuestRole)
		CheckNoError(t, roleRes5)
		role6, roleRes6 = th.SystemAdminClient.GetRoleByName(s1.DefaultChannelGuestRole)
		CheckNoError(t, roleRes6)

		assert.NotZero(t, role1.DeleteAt)
		assert.NotZero(t, role2.DeleteAt)
		assert.NotZero(t, role3.DeleteAt)
		assert.NotZero(t, role4.DeleteAt)
		assert.NotZero(t, role5.DeleteAt)
		assert.NotZero(t, role6.DeleteAt)

		// Check the team now uses the default scheme
		c2, resp := th.SystemAdminClient.GetTeam(team.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "", *c2.SchemeID)
	})

	t.Run("ValidChannelScheme", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

		th.App.SetPhase2PermissionsMigrationStatus(true)

		// Create a channel scheme.
		scheme1 := &model.Scheme{
			DisplayName: model.NewID(),
			Name:        model.NewID(),
			Description: model.NewID(),
			Scope:       model.SchemeScopeChannel,
		}

		s1, r1 := th.SystemAdminClient.CreateScheme(scheme1)
		CheckNoError(t, r1)

		// Retrieve the roles and check they are not deleted.
		role3, roleRes3 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelAdminRole)
		CheckNoError(t, roleRes3)
		role4, roleRes4 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelUserRole)
		CheckNoError(t, roleRes4)
		role6, roleRes6 := th.SystemAdminClient.GetRoleByName(s1.DefaultChannelGuestRole)
		CheckNoError(t, roleRes6)

		assert.Zero(t, role3.DeleteAt)
		assert.Zero(t, role4.DeleteAt)
		assert.Zero(t, role6.DeleteAt)

		// Make sure this scheme is in use by a team.
		channel, err := th.App.Srv().Store.Channel().Save(&model.Channel{
			TeamID:      model.NewID(),
			DisplayName: model.NewID(),
			Name:        model.NewID(),
			Type:        model.ChannelTypeOpen,
			SchemeID:    &s1.ID,
		}, -1)
		assert.NoError(t, err)

		// Delete the Scheme.
		_, r3 := th.SystemAdminClient.DeleteScheme(s1.ID)
		CheckNoError(t, r3)

		// Check the roles were deleted.
		role3, roleRes3 = th.SystemAdminClient.GetRoleByName(s1.DefaultChannelAdminRole)
		CheckNoError(t, roleRes3)
		role4, roleRes4 = th.SystemAdminClient.GetRoleByName(s1.DefaultChannelUserRole)
		CheckNoError(t, roleRes4)
		role6, roleRes6 = th.SystemAdminClient.GetRoleByName(s1.DefaultChannelGuestRole)
		CheckNoError(t, roleRes6)

		assert.NotZero(t, role3.DeleteAt)
		assert.NotZero(t, role4.DeleteAt)
		assert.NotZero(t, role6.DeleteAt)

		// Check the channel now uses the default scheme
		c2, resp := th.SystemAdminClient.GetChannelByName(channel.Name, channel.TeamID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "", *c2.SchemeID)
	})

	t.Run("FailureCases", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

		th.App.SetPhase2PermissionsMigrationStatus(true)

		scheme1 := &model.Scheme{
			DisplayName: model.NewID(),
			Name:        model.NewID(),
			Description: model.NewID(),
			Scope:       model.SchemeScopeChannel,
		}

		s1, r1 := th.SystemAdminClient.CreateScheme(scheme1)
		CheckNoError(t, r1)

		// Test with unknown ID.
		_, r2 := th.SystemAdminClient.DeleteScheme(model.NewID())
		CheckNotFoundStatus(t, r2)

		// Test with invalid ID.
		_, r3 := th.SystemAdminClient.DeleteScheme("12345")
		CheckBadRequestStatus(t, r3)

		// Test without required permissions.
		_, r4 := th.Client.DeleteScheme(s1.ID)
		CheckForbiddenStatus(t, r4)

		// Test without license.
		th.App.Srv().SetLicense(nil)
		_, r5 := th.SystemAdminClient.DeleteScheme(s1.ID)
		CheckNotImplementedStatus(t, r5)

		th.App.SetPhase2PermissionsMigrationStatus(false)

		th.App.Srv().SetLicense(model.NewTestLicense("custom_permissions_schemes"))

		_, r6 := th.SystemAdminClient.DeleteScheme(s1.ID)
		CheckNotImplementedStatus(t, r6)
	})
}

func TestUpdateTeamSchemeWithTeamMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Correctly invalidates team member cache", func(t *testing.T) {
		th.App.SetPhase2PermissionsMigrationStatus(true)

		team := th.CreateTeam()
		_, _, err := th.App.AddUserToTeam(th.Context, team.ID, th.BasicUser.ID, th.SystemAdminUser.ID)
		require.Nil(t, err)

		teamScheme := th.SetupTeamScheme()

		teamUserRole, err := th.App.GetRoleByName(context.Background(), teamScheme.DefaultTeamUserRole)
		require.Nil(t, err)
		teamUserRole.Permissions = []string{}
		_, err = th.App.UpdateRole(teamUserRole)
		require.Nil(t, err)

		th.LoginBasic()

		_, resp := th.Client.CreateChannel(&model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamID: team.ID})
		require.Nil(t, resp.Error)

		team.SchemeID = &teamScheme.ID
		team, err = th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		_, resp = th.Client.CreateChannel(&model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamID: team.ID})
		require.NotNil(t, resp.Error)
	})
}
