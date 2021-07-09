// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestCreateTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	name := "name" + id
	displayName := "Name " + id

	th.CheckCommand(t, "team", "create", "--name", name, "--display_name", displayName)

	found := th.SystemAdminClient.Must(th.SystemAdminClient.TeamExists(name, "")).(bool)

	require.True(t, found, "Failed to create Team")
}

func TestJoinTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.CheckCommand(t, "team", "add", th.BasicTeam.Name, th.BasicUser.Email)

	profiles := th.SystemAdminClient.Must(th.SystemAdminClient.GetUsersInTeam(th.BasicTeam.ID, 0, 1000, "")).([]*model.User)

	found := false

	for _, user := range profiles {
		if user.Email == th.BasicUser.Email {
			found = true
		}

	}

	require.True(t, found, "Failed to create User")
}

func TestLeaveTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.CheckCommand(t, "team", "remove", th.BasicTeam.Name, th.BasicUser.Email)

	profiles := th.Client.Must(th.Client.GetUsersInTeam(th.BasicTeam.ID, 0, 1000, "")).([]*model.User)

	found := false

	for _, user := range profiles {
		if user.Email == th.BasicUser.Email {
			found = true
		}

	}

	require.False(t, found, "profile should not be on team")

	teams, err := th.App.Srv().Store.Team().GetTeamsByUserID(th.BasicUser.ID)
	require.NoError(t, err)
	require.Equal(t, 0, len(teams), "Shouldn't be in team")
}

func TestListTeams(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	name := "name" + id
	displayName := "Name " + id

	th.CheckCommand(t, "team", "create", "--name", name, "--display_name", displayName)

	output := th.CheckCommand(t, "team", "list", th.BasicTeam.Name, th.BasicUser.Email)

	assert.Contains(t, output, name, "should have the created team")
}

func TestListArchivedTeams(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	name := "name" + id
	displayName := "Name " + id

	th.CheckCommand(t, "team", "create", "--name", name, "--display_name", displayName)

	th.CheckCommand(t, "team", "archive", name)

	output := th.CheckCommand(t, "team", "list", th.BasicTeam.Name, th.BasicUser.Email)

	assert.Contains(t, output, name+" (archived)", "should have archived team")
}

func TestSearchTeamsByName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	name := "name" + id
	displayName := "Name " + id

	th.CheckCommand(t, "team", "create", "--name", name, "--display_name", displayName)

	output := th.CheckCommand(t, "team", "search", name)

	assert.Contains(t, output, name, "should have the created team")
}

func TestSearchTeamsByDisplayName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	name := "name" + id
	displayName := "Name " + id

	th.CheckCommand(t, "team", "create", "--name", name, "--display_name", displayName)

	output := th.CheckCommand(t, "team", "search", displayName)

	assert.Contains(t, output, name, "should have the created team")
}

func TestSearchArchivedTeamsByName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	name := "name" + id
	displayName := "Name " + id

	th.CheckCommand(t, "team", "create", "--name", name, "--display_name", displayName)

	th.CheckCommand(t, "team", "archive", name)

	output := th.CheckCommand(t, "team", "search", name)

	assert.Contains(t, output, "(archived)", "should have archived team")
}

func TestArchiveTeams(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	name := "name" + id
	displayName := "Name " + id

	th.CheckCommand(t, "team", "create", "--name", name, "--display_name", displayName)

	th.CheckCommand(t, "team", "archive", name)

	output := th.CheckCommand(t, "team", "list")

	assert.Contains(t, output, name+" (archived)", "should have archived team")
}

func TestRestoreTeams(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	name := "name" + id
	displayName := "Name " + id

	th.CheckCommand(t, "team", "create", "--name", name, "--display_name", displayName)

	th.CheckCommand(t, "team", "archive", name)

	th.CheckCommand(t, "team", "restore", name)

	found := th.SystemAdminClient.Must(th.SystemAdminClient.TeamExists(name, "")).(bool)

	require.True(t, found)
}

func TestRenameTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team := th.CreateTeam()

	newTeamName := "newteamnamex3"
	newDisplayName := "New Display NameX"

	th.CheckCommand(t, "team", "rename", team.Name, newTeamName, "--display_name", newDisplayName)

	// Get the team from the DB
	updatedTeam, _ := th.App.GetTeam(team.ID)

	require.Equal(t, updatedTeam.Name, newTeamName, "failed renaming team")
	require.Equal(t, updatedTeam.DisplayName, newDisplayName, "failed updating team display name")

	// Try to rename to occupied name
	team2 := th.CreateTeam()
	n := team2.Name
	dn := team2.DisplayName

	th.CheckCommand(t, "team", "rename", team2.Name, newTeamName, "--display_name", newDisplayName)

	// No renaming should have occurred
	require.Equal(t, team2.Name, n, "team was renamed when it should have not been")
	require.Equal(t, team2.DisplayName, dn, "team display name was changed when it should have not been")

	// Try to change only Display Name
	team3 := th.CreateTeam()

	// trying to change only Display Name (using "-" as a new team name)
	th.CheckCommand(t, "team", "rename", team3.Name, "-", "--display_name", newDisplayName)

	// Get the team from the DB
	updatedTeam, _ = th.App.GetTeam(team3.ID)

	require.NotEqual(t, updatedTeam.Name, "-", "team was renamed to `-` but only display name should have been changed")
	require.Equal(t, updatedTeam.DisplayName, newDisplayName, "team Display Name was not properly updated")

	// now try to change Display Name using old team name
	th.CheckCommand(t, "team", "rename", team3.Name, team3.Name, "--display_name", "Brand New DName")

	// Get the team from the DB
	updatedTeam, _ = th.App.GetTeam(team3.ID)

	require.Equal(t, updatedTeam.DisplayName, "Brand New DName", "team Display Name was not properly updated")
}

func TestModifyTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team := th.CreateTeam()

	th.CheckCommand(t, "team", "modify", team.Name, "--private")

	updatedTeam, _ := th.App.GetTeam(team.ID)

	require.False(t, !updatedTeam.AllowOpenInvite && team.Type == model.TeamInvite, "Failed modifying team's privacy to private")

	th.CheckCommand(t, "team", "modify", team.Name, "--public")

	require.False(t, updatedTeam.AllowOpenInvite && team.Type == model.TeamOpen, "Failed modifying team's privacy to private")
}
