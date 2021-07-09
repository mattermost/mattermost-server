// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mail"
	"github.com/mattermost/mattermost-server/v5/utils/testutils"
)

func TestCreateTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		team := &model.Team{Name: GenerateTestUsername(), DisplayName: "Some Team", Type: model.TeamOpen}
		rteam, resp := client.CreateTeam(team)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		require.Equal(t, rteam.Name, team.Name, "names did not match")

		require.Equal(t, rteam.DisplayName, team.DisplayName, "display names did not match")

		require.Equal(t, rteam.Type, team.Type, "types did not match")

		_, resp = client.CreateTeam(rteam)
		CheckBadRequestStatus(t, resp)

		rteam.ID = ""
		_, resp = client.CreateTeam(rteam)
		CheckErrorMessage(t, resp, "app.team.save.existing.app_error")
		CheckBadRequestStatus(t, resp)

		rteam.Name = ""
		_, resp = client.CreateTeam(rteam)
		CheckErrorMessage(t, resp, "model.team.is_valid.characters.app_error")
		CheckBadRequestStatus(t, resp)

		r, err := client.DoApiPost("/teams", "garbage")
		require.NotNil(t, err, "should have errored")

		require.Equalf(t, r.StatusCode, http.StatusBadRequest, "wrong status code, actual: %s, expected: %s", strconv.Itoa(r.StatusCode), strconv.Itoa(http.StatusBadRequest))

		// Test GroupConstrained flag
		groupConstrainedTeam := &model.Team{Name: GenerateTestUsername(), DisplayName: "Some Team", Type: model.TeamOpen, GroupConstrained: model.NewBool(true)}
		rteam, resp = client.CreateTeam(groupConstrainedTeam)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		assert.Equal(t, *rteam.GroupConstrained, *groupConstrainedTeam.GroupConstrained, "GroupConstrained flags do not match")
	})

	th.Client.Logout()

	team := &model.Team{Name: GenerateTestUsername(), DisplayName: "Some Team", Type: model.TeamOpen}
	_, resp := th.Client.CreateTeam(team)
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic()

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.RemovePermissionFromRole(model.PermissionCreateTeam.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionCreateTeam.ID, model.SystemAdminRoleID)

	_, resp = th.Client.CreateTeam(team)
	CheckForbiddenStatus(t, resp)
}

func TestCreateTeamSanitization(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// Non-admin users can create a team, but they become a team admin by doing so

	t.Run("team admin", func(t *testing.T) {
		team := &model.Team{
			DisplayName:    t.Name() + "_1",
			Name:           GenerateTestTeamName(),
			Email:          th.GenerateTestEmail(),
			Type:           model.TeamOpen,
			AllowedDomains: "simulator.amazonses.com,localhost",
		}

		rteam, resp := th.Client.CreateTeam(team)
		CheckNoError(t, resp)
		require.NotEmpty(t, rteam.Email, "should not have sanitized email")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		team := &model.Team{
			DisplayName:    t.Name() + "_2",
			Name:           GenerateTestTeamName(),
			Email:          th.GenerateTestEmail(),
			Type:           model.TeamOpen,
			AllowedDomains: "simulator.amazonses.com,localhost",
		}

		rteam, resp := client.CreateTeam(team)
		CheckNoError(t, resp)
		require.NotEmpty(t, rteam.Email, "should not have sanitized email")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	}, "system admin")
}

func TestGetTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		rteam, resp := client.GetTeam(team.ID, "")
		CheckNoError(t, resp)

		require.Equal(t, rteam.ID, team.ID, "wrong team")

		_, resp = client.GetTeam("junk", "")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetTeam("", "")
		CheckNotFoundStatus(t, resp)

		_, resp = client.GetTeam(model.NewID(), "")
		CheckNotFoundStatus(t, resp)
	})

	th.LoginTeamAdmin()

	team2 := &model.Team{DisplayName: "Name", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen, AllowOpenInvite: false}
	rteam2, _ := Client.CreateTeam(team2)

	team3 := &model.Team{DisplayName: "Name", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamInvite, AllowOpenInvite: true}
	rteam3, _ := Client.CreateTeam(team3)

	th.LoginBasic()
	// AllowInviteOpen is false and team is open, and user is not on team
	_, resp := Client.GetTeam(rteam2.ID, "")
	CheckForbiddenStatus(t, resp)

	// AllowInviteOpen is true and team is invite, and user is not on team
	_, resp = Client.GetTeam(rteam3.ID, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetTeam(team.ID, "")
	CheckUnauthorizedStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.GetTeam(rteam2.ID, "")
		CheckNoError(t, resp)
	})
}

func TestGetTeamSanitization(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:    t.Name() + "_1",
		Name:           GenerateTestTeamName(),
		Email:          th.GenerateTestEmail(),
		Type:           model.TeamOpen,
		AllowedDomains: "simulator.amazonses.com,localhost",
	})
	CheckNoError(t, resp)

	t.Run("team user", func(t *testing.T) {
		th.LinkUserToTeam(th.BasicUser2, team)

		client := th.CreateClient()
		th.LoginBasic2WithClient(client)

		rteam, resp := client.GetTeam(team.ID, "")
		CheckNoError(t, resp)

		require.Empty(t, rteam.Email, "should have sanitized email")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	})

	t.Run("team user without invite permissions", func(t *testing.T) {
		th.RemovePermissionFromRole(model.PermissionInviteUser.ID, model.TeamUserRoleID)
		th.LinkUserToTeam(th.BasicUser2, team)

		client := th.CreateClient()
		th.LoginBasic2WithClient(client)

		rteam, resp := client.GetTeam(team.ID, "")
		CheckNoError(t, resp)

		require.Empty(t, rteam.Email, "should have sanitized email")
		require.Empty(t, rteam.InviteID, "should have sanitized inviteid")
	})

	t.Run("team admin", func(t *testing.T) {
		rteam, resp := th.Client.GetTeam(team.ID, "")
		CheckNoError(t, resp)

		require.NotEmpty(t, rteam.Email, "should not have sanitized email")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		rteam, resp := client.GetTeam(team.ID, "")
		CheckNoError(t, resp)

		require.NotEmpty(t, rteam.Email, "should not have sanitized email")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	}, "system admin")
}

func TestGetTeamUnread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	teamUnread, resp := Client.GetTeamUnread(th.BasicTeam.ID, th.BasicUser.ID)
	CheckNoError(t, resp)
	require.Equal(t, teamUnread.TeamID, th.BasicTeam.ID, "wrong team id returned for regular user call")

	_, resp = Client.GetTeamUnread("junk", th.BasicUser.ID)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamUnread(th.BasicTeam.ID, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamUnread(model.NewID(), th.BasicUser.ID)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetTeamUnread(th.BasicTeam.ID, model.NewID())
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetTeamUnread(th.BasicTeam.ID, th.BasicUser.ID)
	CheckUnauthorizedStatus(t, resp)

	teamUnread, resp = th.SystemAdminClient.GetTeamUnread(th.BasicTeam.ID, th.BasicUser.ID)
	CheckNoError(t, resp)
	require.Equal(t, teamUnread.TeamID, th.BasicTeam.ID, "wrong team id returned")
}

func TestUpdateTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		team := &model.Team{DisplayName: "Name", Description: "Some description", AllowOpenInvite: false, InviteID: "inviteid0", Name: "z-z-" + model.NewRandomTeamName() + "a", Email: "success+" + model.NewID() + "@simulator.amazonses.com", Type: model.TeamOpen}
		var resp *model.Response
		team, resp = th.Client.CreateTeam(team)
		CheckNoError(t, resp)

		team.Description = "updated description"
		uteam, resp := client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.Equal(t, uteam.Description, "updated description", "Update failed")

		team.DisplayName = "Updated Name"
		uteam, resp = client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.Equal(t, uteam.DisplayName, "Updated Name", "Update failed")

		// Test GroupConstrained flag
		team.GroupConstrained = model.NewBool(true)
		rteam, resp := client.UpdateTeam(team)
		CheckNoError(t, resp)
		CheckOKStatus(t, resp)

		require.Equal(t, *rteam.GroupConstrained, *team.GroupConstrained, "GroupConstrained flags do not match")

		team.GroupConstrained = nil

		team.AllowOpenInvite = true
		uteam, resp = client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.True(t, uteam.AllowOpenInvite, "Update failed")

		team.InviteID = "inviteid1"
		uteam, resp = client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.NotEqual(t, uteam.InviteID, "inviteid1", "InviteID should not be updated")

		team.AllowedDomains = "domain"
		uteam, resp = client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.Equal(t, uteam.AllowedDomains, "domain", "Update failed")

		team.Name = "Updated name"
		uteam, resp = client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.NotEqual(t, uteam.Name, "Updated name", "Should not update name")

		team.Email = "test@domain.com"
		uteam, resp = client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.NotEqual(t, uteam.Email, "test@domain.com", "Should not update email")

		team.Type = model.TeamInvite
		uteam, resp = client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.NotEqual(t, uteam.Type, model.TeamInvite, "Should not update type")

		originalTeamID := team.ID
		team.ID = model.NewID()

		r, _ := client.DoApiPut(client.GetTeamRoute(originalTeamID), team.ToJSON())
		assert.Equal(t, http.StatusBadRequest, r.StatusCode)

		require.Equal(t, uteam.ID, originalTeamID, "wrong team id")

		team.ID = "fake"
		_, resp = client.UpdateTeam(team)
		CheckBadRequestStatus(t, resp)

		th.Client.Logout() // for non-local clients
		_, resp = th.Client.UpdateTeam(team)
		CheckUnauthorizedStatus(t, resp)
		th.LoginBasic()
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		team := &model.Team{DisplayName: "New", Description: "Some description", AllowOpenInvite: false, InviteID: "inviteid0", Name: "z-z-" + model.NewRandomTeamName() + "a", Email: "success+" + model.NewID() + "@simulator.amazonses.com", Type: model.TeamOpen}
		var resp *model.Response
		team, resp = client.CreateTeam(team)
		CheckNoError(t, resp)

		team.Name = "new-name"
		_, resp = client.UpdateTeam(team)
		CheckNoError(t, resp)
	})
}

func TestUpdateTeamSanitization(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	team, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:    t.Name() + "_1",
		Name:           GenerateTestTeamName(),
		Email:          th.GenerateTestEmail(),
		Type:           model.TeamOpen,
		AllowedDomains: "simulator.amazonses.com,localhost",
	})
	CheckNoError(t, resp)

	// Non-admin users cannot update the team

	t.Run("team admin", func(t *testing.T) {
		rteam, resp := th.Client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.NotEmpty(t, rteam.Email, "should not have sanitized email for admin")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	})

	t.Run("system admin", func(t *testing.T) {
		rteam, resp := th.SystemAdminClient.UpdateTeam(team)
		CheckNoError(t, resp)

		require.NotEmpty(t, rteam.Email, "should not have sanitized email for admin")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	})
}

func TestPatchTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team := &model.Team{DisplayName: "Name", Description: "Some description", CompanyName: "Some company name", AllowOpenInvite: false, InviteID: "inviteid0", Name: "z-z-" + model.NewRandomTeamName() + "a", Email: "success+" + model.NewID() + "@simulator.amazonses.com", Type: model.TeamOpen}
	team, _ = th.Client.CreateTeam(team)

	patch := &model.TeamPatch{}
	patch.DisplayName = model.NewString("Other name")
	patch.Description = model.NewString("Other description")
	patch.CompanyName = model.NewString("Other company name")
	patch.AllowOpenInvite = model.NewBool(true)

	_, resp := th.Client.PatchTeam(GenerateTestID(), patch)
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.PatchTeam(team.ID, patch)
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic2()
	_, resp = th.Client.PatchTeam(team.ID, patch)
	CheckForbiddenStatus(t, resp)
	th.LoginBasic()

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		rteam, resp := client.PatchTeam(team.ID, patch)
		CheckNoError(t, resp)

		require.Equal(t, rteam.DisplayName, "Other name", "DisplayName did not update properly")
		require.Equal(t, rteam.Description, "Other description", "Description did not update properly")
		require.Equal(t, rteam.CompanyName, "Other company name", "CompanyName did not update properly")
		require.NotEqual(t, rteam.InviteID, "inviteid1", "InviteId should not update")
		require.True(t, rteam.AllowOpenInvite, "AllowOpenInvite did not update properly")

		t.Run("Changing AllowOpenInvite to false regenerates InviteID", func(t *testing.T) {
			team2 := &model.Team{DisplayName: "Name2", Description: "Some description", CompanyName: "Some company name", AllowOpenInvite: true, InviteID: model.NewID(), Name: "z-z-" + model.NewRandomTeamName() + "a", Email: "success+" + model.NewID() + "@simulator.amazonses.com", Type: model.TeamOpen}
			team2, _ = client.CreateTeam(team2)

			patch2 := &model.TeamPatch{
				AllowOpenInvite: model.NewBool(false),
			}

			rteam2, resp2 := client.PatchTeam(team2.ID, patch2)
			CheckNoError(t, resp2)
			require.Equal(t, team2.ID, rteam2.ID)
			require.False(t, rteam2.AllowOpenInvite)
			require.NotEqual(t, team2.InviteID, rteam2.InviteID)
		})

		t.Run("Changing AllowOpenInvite to true doesn't regenerate InviteID", func(t *testing.T) {
			team2 := &model.Team{DisplayName: "Name3", Description: "Some description", CompanyName: "Some company name", AllowOpenInvite: false, InviteID: model.NewID(), Name: "z-z-" + model.NewRandomTeamName() + "a", Email: "success+" + model.NewID() + "@simulator.amazonses.com", Type: model.TeamOpen}
			team2, _ = client.CreateTeam(team2)

			patch2 := &model.TeamPatch{
				AllowOpenInvite: model.NewBool(true),
			}

			rteam2, resp2 := client.PatchTeam(team2.ID, patch2)
			CheckNoError(t, resp2)
			require.Equal(t, team2.ID, rteam2.ID)
			require.True(t, rteam2.AllowOpenInvite)
			require.Equal(t, team2.InviteID, rteam2.InviteID)
		})

		// Test GroupConstrained flag
		patch.GroupConstrained = model.NewBool(true)
		rteam, resp = client.PatchTeam(team.ID, patch)
		CheckNoError(t, resp)
		CheckOKStatus(t, resp)
		require.Equal(t, *rteam.GroupConstrained, *patch.GroupConstrained, "GroupConstrained flags do not match")

		patch.GroupConstrained = nil
		_, resp = client.PatchTeam("junk", patch)
		CheckBadRequestStatus(t, resp)

		r, err := client.DoApiPut("/teams/"+team.ID+"/patch", "garbage")
		require.NotNil(t, err, "should have errored")
		require.Equalf(t, r.StatusCode, http.StatusBadRequest, "wrong status code, actual: %s, expected: %s", strconv.Itoa(r.StatusCode), strconv.Itoa(http.StatusBadRequest))
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.PatchTeam(th.BasicTeam.ID, patch)
		CheckNoError(t, resp)
	})
}

func TestRestoreTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	createTeam := func(t *testing.T, deleted bool, teamType string) *model.Team {
		t.Helper()
		team := &model.Team{
			DisplayName:     "Some Team",
			Description:     "Some description",
			CompanyName:     "Some company name",
			AllowOpenInvite: (teamType == model.TeamOpen),
			InviteID:        model.NewID(),
			Name:            "aa-" + model.NewRandomTeamName() + "zz",
			Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
			Type:            teamType,
		}
		team, _ = Client.CreateTeam(team)
		require.NotNil(t, team)
		if deleted {
			_, resp := th.SystemAdminClient.SoftDeleteTeam(team.ID)
			CheckOKStatus(t, resp)
		}
		return team
	}
	teamPublic := createTeam(t, true, model.TeamOpen)

	t.Run("invalid team", func(t *testing.T) {
		_, resp := Client.RestoreTeam(model.NewID())
		CheckForbiddenStatus(t, resp)
	})

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		team := createTeam(t, true, model.TeamOpen)
		team, resp := client.RestoreTeam(team.ID)
		CheckOKStatus(t, resp)
		require.Zero(t, team.DeleteAt)
		require.Equal(t, model.TeamOpen, team.Type)
	}, "restore archived public team")

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		team := createTeam(t, true, model.TeamInvite)
		team, resp := client.RestoreTeam(team.ID)
		CheckOKStatus(t, resp)
		require.Zero(t, team.DeleteAt)
		require.Equal(t, model.TeamInvite, team.Type)
	}, "restore archived private team")

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		team := createTeam(t, false, model.TeamOpen)
		team, resp := client.RestoreTeam(team.ID)
		CheckOKStatus(t, resp)
		require.Zero(t, team.DeleteAt)
		require.Equal(t, model.TeamOpen, team.Type)
	}, "restore active public team")

	t.Run("not logged in", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.RestoreTeam(teamPublic.ID)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("no permission to manage team", func(t *testing.T) {
		th.LoginBasic2()
		_, resp := Client.RestoreTeam(teamPublic.ID)
		CheckForbiddenStatus(t, resp)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.RestoreTeam(teamPublic.ID)
		CheckOKStatus(t, resp)
	})
}

func TestPatchTeamSanitization(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	team, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:    t.Name() + "_1",
		Name:           GenerateTestTeamName(),
		Email:          th.GenerateTestEmail(),
		Type:           model.TeamOpen,
		AllowedDomains: "simulator.amazonses.com,localhost",
	})
	CheckNoError(t, resp)

	// Non-admin users cannot update the team

	t.Run("team admin", func(t *testing.T) {
		rteam, resp := th.Client.PatchTeam(team.ID, &model.TeamPatch{})
		CheckNoError(t, resp)

		require.NotEmpty(t, rteam.Email, "should not have sanitized email for admin")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	})

	t.Run("system admin", func(t *testing.T) {
		rteam, resp := th.SystemAdminClient.PatchTeam(team.ID, &model.TeamPatch{})
		CheckNoError(t, resp)

		require.NotEmpty(t, rteam.Email, "should not have sanitized email for admin")
		require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
	})
}

func TestUpdateTeamPrivacy(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	createTeam := func(teamType string, allowOpenInvite bool) *model.Team {
		team := &model.Team{
			DisplayName:     teamType + " Team",
			Description:     "Some description",
			CompanyName:     "Some company name",
			AllowOpenInvite: allowOpenInvite,
			InviteID:        model.NewID(),
			Name:            "aa-" + model.NewRandomTeamName() + "zz",
			Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
			Type:            teamType,
		}
		team, _ = Client.CreateTeam(team)
		return team
	}

	teamPublic := createTeam(model.TeamOpen, true)
	teamPrivate := createTeam(model.TeamInvite, false)

	teamPublic2 := createTeam(model.TeamOpen, true)
	teamPrivate2 := createTeam(model.TeamInvite, false)

	tests := []struct {
		name                string
		team                *model.Team
		privacy             string
		errChecker          func(t *testing.T, resp *model.Response)
		wantType            string
		wantOpenInvite      bool
		wantInviteIDChanged bool
		originalInviteID    string
	}{
		{name: "bad privacy", team: teamPublic, privacy: "blap", errChecker: CheckBadRequestStatus, wantType: model.TeamOpen, wantOpenInvite: true},
		{name: "public to private", team: teamPublic, privacy: model.TeamInvite, errChecker: nil, wantType: model.TeamInvite, wantOpenInvite: false, originalInviteID: teamPublic.InviteID, wantInviteIDChanged: true},
		{name: "private to public", team: teamPrivate, privacy: model.TeamOpen, errChecker: nil, wantType: model.TeamOpen, wantOpenInvite: true, originalInviteID: teamPrivate.InviteID, wantInviteIDChanged: false},
		{name: "public to public", team: teamPublic2, privacy: model.TeamOpen, errChecker: nil, wantType: model.TeamOpen, wantOpenInvite: true, originalInviteID: teamPublic2.InviteID, wantInviteIDChanged: false},
		{name: "private to private", team: teamPrivate2, privacy: model.TeamInvite, errChecker: nil, wantType: model.TeamInvite, wantOpenInvite: false, originalInviteID: teamPrivate2.InviteID, wantInviteIDChanged: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
				team, resp := client.UpdateTeamPrivacy(test.team.ID, test.privacy)
				if test.errChecker != nil {
					test.errChecker(t, resp)
					return
				}
				CheckNoError(t, resp)
				CheckOKStatus(t, resp)
				require.Equal(t, test.wantType, team.Type)
				require.Equal(t, test.wantOpenInvite, team.AllowOpenInvite)
				if test.wantInviteIDChanged {
					require.NotEqual(t, test.originalInviteID, team.InviteID)
				} else {
					require.Equal(t, test.originalInviteID, team.InviteID)
				}
			})
		})
	}

	t.Run("non-existent team", func(t *testing.T) {
		_, resp := Client.UpdateTeamPrivacy(model.NewID(), model.TeamInvite)
		CheckForbiddenStatus(t, resp)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.UpdateTeamPrivacy(model.NewID(), model.TeamInvite)
		CheckNotFoundStatus(t, resp)
	}, "non-existent team for admins")

	t.Run("not logged in", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.UpdateTeamPrivacy(teamPublic.ID, model.TeamInvite)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("no permission to manage team", func(t *testing.T) {
		th.LoginBasic2()
		_, resp := Client.UpdateTeamPrivacy(teamPublic.ID, model.TeamInvite)
		CheckForbiddenStatus(t, resp)
	})
}

func TestTeamUnicodeNames(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	t.Run("create team unicode", func(t *testing.T) {
		team := &model.Team{
			Name:        GenerateTestUsername(),
			DisplayName: "Some\u206c Team",
			Description: "A \ufffatest\ufffb channel.",
			CompanyName: "\ufeffAcme Inc\ufffc",
			Type:        model.TeamOpen}
		rteam, resp := Client.CreateTeam(team)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		require.Equal(t, "Some Team", rteam.DisplayName, "bad unicode should be filtered from display name")
		require.Equal(t, "A test channel.", rteam.Description, "bad unicode should be filtered from description")
		require.Equal(t, "Acme Inc", rteam.CompanyName, "bad unicode should be filtered from company name")
	})

	t.Run("update team unicode", func(t *testing.T) {
		team := &model.Team{
			DisplayName: "Name",
			Description: "Some description",
			CompanyName: "Bad Company",
			Name:        model.NewRandomTeamName(),
			Email:       "success+" + model.NewID() + "@simulator.amazonses.com",
			Type:        model.TeamOpen}
		team, _ = Client.CreateTeam(team)

		team.DisplayName = "\u206eThe Team\u206f"
		team.Description = "A \u17a3great\u17d3 team."
		team.CompanyName = "\u206aAcme Inc"
		uteam, resp := Client.UpdateTeam(team)
		CheckNoError(t, resp)

		require.Equal(t, "The Team", uteam.DisplayName, "bad unicode should be filtered from display name")
		require.Equal(t, "A great team.", uteam.Description, "bad unicode should be filtered from description")
		require.Equal(t, "Acme Inc", uteam.CompanyName, "bad unicode should be filtered from company name")
	})

	t.Run("patch team unicode", func(t *testing.T) {
		team := &model.Team{
			DisplayName: "Name",
			Description: "Some description",
			CompanyName: "Some company name",
			Name:        model.NewRandomTeamName(),
			Email:       "success+" + model.NewID() + "@simulator.amazonses.com",
			Type:        model.TeamOpen}
		team, _ = Client.CreateTeam(team)

		patch := &model.TeamPatch{}

		patch.DisplayName = model.NewString("Goat\u206e Team")
		patch.Description = model.NewString("\ufffaGreat team.")
		patch.CompanyName = model.NewString("\u202bAcme Inc\u202c")

		rteam, resp := Client.PatchTeam(team.ID, patch)
		CheckNoError(t, resp)

		require.Equal(t, "Goat Team", rteam.DisplayName, "bad unicode should be filtered from display name")
		require.Equal(t, "Great team.", rteam.Description, "bad unicode should be filtered from description")
		require.Equal(t, "Acme Inc", rteam.CompanyName, "bad unicode should be filtered from company name")
	})
}

func TestRegenerateTeamInviteID(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	team := &model.Team{DisplayName: "Name", Description: "Some description", CompanyName: "Some company name", AllowOpenInvite: false, InviteID: "inviteid0", Name: "z-z-" + model.NewRandomTeamName() + "a", Email: "success+" + model.NewID() + "@simulator.amazonses.com", Type: model.TeamOpen}
	team, _ = Client.CreateTeam(team)

	assert.NotEqual(t, team.InviteID, "")
	assert.NotEqual(t, team.InviteID, "inviteid0")

	rteam, resp := Client.RegenerateTeamInviteID(team.ID)
	CheckNoError(t, resp)

	assert.NotEqual(t, team.InviteID, rteam.InviteID)
	assert.NotEqual(t, team.InviteID, "")
}

func TestSoftDeleteTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, resp := th.Client.SoftDeleteTeam(th.BasicTeam.ID)
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.SoftDeleteTeam(th.BasicTeam.ID)
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic()
	team := &model.Team{DisplayName: "DisplayName", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen}
	team, _ = th.Client.CreateTeam(team)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		ok, resp := client.SoftDeleteTeam(team.ID)
		CheckNoError(t, resp)

		require.True(t, ok, "should have returned true")

		rteam, err := th.App.GetTeam(team.ID)
		require.Nil(t, err, "should have returned archived team")
		require.NotEqual(t, rteam.DeleteAt, 0, "should have not set to zero")

		ok, resp = client.SoftDeleteTeam("junk")
		CheckBadRequestStatus(t, resp)

		require.False(t, ok, "should have returned false")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.SoftDeleteTeam(th.BasicTeam.ID)
		CheckNoError(t, resp)
	})
}

func TestPermanentDeleteTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	enableAPITeamDeletion := *th.App.Config().ServiceSettings.EnableAPITeamDeletion
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableAPITeamDeletion = &enableAPITeamDeletion })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPITeamDeletion = false })

	t.Run("Permanent deletion not available through API if EnableAPITeamDeletion is not set", func(t *testing.T) {
		team := &model.Team{DisplayName: "DisplayName", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen}
		team, _ = th.Client.CreateTeam(team)

		_, resp := th.Client.PermanentDeleteTeam(team.ID)
		CheckUnauthorizedStatus(t, resp)

		_, resp = th.SystemAdminClient.PermanentDeleteTeam(team.ID)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("Permanent deletion available through local mode even if EnableAPITeamDeletion is not set", func(t *testing.T) {
		team := &model.Team{DisplayName: "DisplayName", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen}
		team, _ = th.Client.CreateTeam(team)

		ok, resp := th.LocalClient.PermanentDeleteTeam(team.ID)
		CheckNoError(t, resp)
		assert.True(t, ok)
	})

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableAPITeamDeletion = &enableAPITeamDeletion })
		}()
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPITeamDeletion = true })

		team := &model.Team{DisplayName: "DisplayName", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen}
		team, _ = client.CreateTeam(team)
		ok, resp := client.PermanentDeleteTeam(team.ID)
		CheckNoError(t, resp)
		assert.True(t, ok)

		_, err := th.App.GetTeam(team.ID)
		assert.NotNil(t, err)

		ok, resp = client.PermanentDeleteTeam("junk")
		CheckBadRequestStatus(t, resp)

		require.False(t, ok, "should have returned false")
	}, "Permanent deletion with EnableAPITeamDeletion set")
}

func TestGetAllTeams(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	team1 := &model.Team{DisplayName: "Name", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen, AllowOpenInvite: true}
	team1, resp := Client.CreateTeam(team1)
	CheckNoError(t, resp)

	team2 := &model.Team{DisplayName: "Name2", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen, AllowOpenInvite: true}
	team2, resp = Client.CreateTeam(team2)
	CheckNoError(t, resp)

	team3 := &model.Team{DisplayName: "Name3", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen, AllowOpenInvite: false}
	team3, resp = Client.CreateTeam(team3)
	CheckNoError(t, resp)

	team4 := &model.Team{DisplayName: "Name4", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen, AllowOpenInvite: false}
	team4, resp = Client.CreateTeam(team4)
	CheckNoError(t, resp)

	testCases := []struct {
		Name               string
		Page               int
		PerPage            int
		Permissions        []string
		ExpectedTeams      []string
		WithCount          bool
		ExpectedCount      int64
		ExpectedError      bool
		ErrorID            string
		ExpectedStatusCode int
	}{
		{
			Name:          "Get 1 team per page",
			Page:          0,
			PerPage:       1,
			Permissions:   []string{model.PermissionListPublicTeams.ID},
			ExpectedTeams: []string{team1.ID},
		},
		{
			Name:          "Get second page with 1 team per page",
			Page:          1,
			PerPage:       1,
			Permissions:   []string{model.PermissionListPublicTeams.ID},
			ExpectedTeams: []string{team2.ID},
		},
		{
			Name:          "Get no items per page",
			Page:          1,
			PerPage:       0,
			Permissions:   []string{model.PermissionListPublicTeams.ID},
			ExpectedTeams: []string{},
		},
		{
			Name:          "Get all open teams",
			Page:          0,
			PerPage:       10,
			Permissions:   []string{model.PermissionListPublicTeams.ID},
			ExpectedTeams: []string{team1.ID, team2.ID},
		},
		{
			Name:          "Get all private teams",
			Page:          0,
			PerPage:       10,
			Permissions:   []string{model.PermissionListPrivateTeams.ID},
			ExpectedTeams: []string{th.BasicTeam.ID, team3.ID, team4.ID},
		},
		{
			Name:          "Get all teams",
			Page:          0,
			PerPage:       10,
			Permissions:   []string{model.PermissionListPublicTeams.ID, model.PermissionListPrivateTeams.ID},
			ExpectedTeams: []string{th.BasicTeam.ID, team1.ID, team2.ID, team3.ID, team4.ID},
		},
		{
			Name:               "Get no teams because permissions",
			Page:               0,
			PerPage:            10,
			Permissions:        []string{},
			ExpectedError:      true,
			ExpectedStatusCode: http.StatusForbidden,
			ErrorID:            "api.team.get_all_teams.insufficient_permissions",
		},
		{
			Name:               "Get no teams because permissions with count",
			Page:               0,
			PerPage:            10,
			Permissions:        []string{},
			WithCount:          true,
			ExpectedError:      true,
			ExpectedStatusCode: http.StatusForbidden,
			ErrorID:            "api.team.get_all_teams.insufficient_permissions",
		},
		{
			Name:          "Get all teams with count",
			Page:          0,
			PerPage:       10,
			Permissions:   []string{model.PermissionListPublicTeams.ID, model.PermissionListPrivateTeams.ID},
			ExpectedTeams: []string{th.BasicTeam.ID, team1.ID, team2.ID, team3.ID, team4.ID},
			WithCount:     true,
			ExpectedCount: 5,
		},
		{
			Name:          "Get all public teams with count",
			Page:          0,
			PerPage:       10,
			Permissions:   []string{model.PermissionListPublicTeams.ID},
			ExpectedTeams: []string{team1.ID, team2.ID},
			WithCount:     true,
			ExpectedCount: 2,
		},
		{
			Name:          "Get all private teams with count",
			Page:          0,
			PerPage:       10,
			Permissions:   []string{model.PermissionListPrivateTeams.ID},
			ExpectedTeams: []string{th.BasicTeam.ID, team3.ID, team4.ID},
			WithCount:     true,
			ExpectedCount: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			defaultRolePermissions := th.SaveDefaultRolePermissions()
			defer func() {
				th.RestoreDefaultRolePermissions(defaultRolePermissions)
			}()
			th.RemovePermissionFromRole(model.PermissionListPublicTeams.ID, model.SystemUserRoleID)
			th.RemovePermissionFromRole(model.PermissionJoinPublicTeams.ID, model.SystemUserRoleID)
			th.RemovePermissionFromRole(model.PermissionListPrivateTeams.ID, model.SystemUserRoleID)
			th.RemovePermissionFromRole(model.PermissionJoinPrivateTeams.ID, model.SystemUserRoleID)
			for _, permission := range tc.Permissions {
				th.AddPermissionToRole(permission, model.SystemUserRoleID)
			}

			var teams []*model.Team
			var count int64
			if tc.WithCount {
				teams, count, resp = Client.GetAllTeamsWithTotalCount("", tc.Page, tc.PerPage)
			} else {
				teams, resp = Client.GetAllTeams("", tc.Page, tc.PerPage)
			}
			if tc.ExpectedError {
				CheckErrorMessage(t, resp, tc.ErrorID)
				checkHTTPStatus(t, resp, tc.ExpectedStatusCode, true)
				return
			}
			CheckNoError(t, resp)
			require.Equal(t, len(tc.ExpectedTeams), len(teams))
			for idx, team := range teams {
				assert.Equal(t, tc.ExpectedTeams[idx], team.ID)
			}
			require.Equal(t, tc.ExpectedCount, count)
		})
	}

	t.Run("Local mode", func(t *testing.T) {
		teams, res := th.LocalClient.GetAllTeams("", 0, 10)
		CheckNoError(t, res)
		require.Len(t, teams, 5)
	})

	// Choose a team which the system manager can access
	sysManagerTeams, resp := th.SystemManagerClient.GetAllTeams("", 0, 10000)
	CheckOKStatus(t, resp)
	policyTeam := sysManagerTeams[0]
	// If no policies exist, GetAllTeamsExcludePolicyConstrained should return everything
	t.Run("exclude policy constrained, without policy", func(t *testing.T) {
		_, excludeConstrainedResp := Client.GetAllTeamsExcludePolicyConstrained("", 0, 100)
		CheckForbiddenStatus(t, excludeConstrainedResp)
		teams, excludeConstrainedResp := th.SystemAdminClient.GetAllTeamsExcludePolicyConstrained("", 0, 100)
		CheckOKStatus(t, excludeConstrainedResp)
		found := false
		for _, team := range teams {
			if team.ID == policyTeam.ID {
				found = true
				break
			}
		}
		require.True(t, found)
	})
	// Now actually create the policy and assign the team to it
	policy, savePolicyErr := th.App.Srv().Store.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:  "Policy 1",
			PostDuration: model.NewInt64(30),
		},
		TeamIDs: []string{policyTeam.ID},
	})
	require.NoError(t, savePolicyErr)
	// This time, the team shouldn't be returned
	t.Run("exclude policy constrained, with policy", func(t *testing.T) {
		teams, excludeConstrainedResp := th.SystemAdminClient.GetAllTeamsExcludePolicyConstrained("", 0, 100)
		CheckOKStatus(t, excludeConstrainedResp)
		found := false
		for _, team := range teams {
			if team.ID == policyTeam.ID {
				found = true
				break
			}
		}
		require.False(t, found)
	})

	t.Run("does not return policy ID", func(t *testing.T) {
		teams, sysManagerResp := th.SystemManagerClient.GetAllTeams("", 0, 100)
		CheckOKStatus(t, sysManagerResp)
		found := false
		for _, team := range teams {
			if team.ID == policyTeam.ID {
				found = true
				require.Nil(t, team.PolicyID)
				break
			}
		}
		require.True(t, found)
	})

	t.Run("returns policy ID", func(t *testing.T) {
		teams, sysAdminResp := th.SystemAdminClient.GetAllTeams("", 0, 100)
		CheckOKStatus(t, sysAdminResp)
		found := false
		for _, team := range teams {
			if team.ID == policyTeam.ID {
				found = true
				require.Equal(t, *team.PolicyID, policy.ID)
				break
			}
		}
		require.True(t, found)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		Client.Logout()
		_, resp = Client.GetAllTeams("", 1, 10)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestGetAllTeamsSanitization(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	team, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:     t.Name() + "_1",
		Name:            GenerateTestTeamName(),
		Email:           th.GenerateTestEmail(),
		Type:            model.TeamOpen,
		AllowedDomains:  "simulator.amazonses.com,localhost",
		AllowOpenInvite: true,
	})
	CheckNoError(t, resp)
	team2, resp := th.SystemAdminClient.CreateTeam(&model.Team{
		DisplayName:     t.Name() + "_2",
		Name:            GenerateTestTeamName(),
		Email:           th.GenerateTestEmail(),
		Type:            model.TeamOpen,
		AllowedDomains:  "simulator.amazonses.com,localhost",
		AllowOpenInvite: true,
	})
	CheckNoError(t, resp)

	// This may not work if the server has over 1000 open teams on it

	t.Run("team admin/non-admin", func(t *testing.T) {
		teamFound := false
		team2Found := false

		rteams, resp := th.Client.GetAllTeams("", 0, 1000)
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			if rteam.ID == team.ID {
				teamFound = true
				require.NotEmpty(t, rteam.Email, "should not have sanitized email for team admin")
				require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
			} else if rteam.ID == team2.ID {
				team2Found = true
				require.Empty(t, rteam.Email, "should have sanitized email for team admin")
				require.Empty(t, rteam.InviteID, "should have sanitized inviteid")
			}
		}

		require.True(t, teamFound, "wasn't returned the expected teams so the test wasn't run correctly")
		require.True(t, team2Found, "wasn't returned the expected teams so the test wasn't run correctly")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		rteams, resp := client.GetAllTeams("", 0, 1000)
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			if rteam.ID != team.ID && rteam.ID != team2.ID {
				continue
			}

			require.NotEmpty(t, rteam.Email, "should not have sanitized email")
			require.NotEmpty(t, rteam.InviteID, "should not have sanitized inviteid")
		}
	}, "system admin")
}

func TestGetTeamByName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	team := th.BasicTeam

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		rteam, resp := client.GetTeamByName(team.Name, "")
		CheckNoError(t, resp)

		require.Equal(t, rteam.Name, team.Name, "wrong team")

		_, resp = client.GetTeamByName("junk", "")
		CheckNotFoundStatus(t, resp)

		_, resp = client.GetTeamByName("", "")
		CheckNotFoundStatus(t, resp)

	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.GetTeamByName(strings.ToUpper(team.Name), "")
		CheckNoError(t, resp)
	})

	th.Client.Logout()
	_, resp := th.Client.GetTeamByName(team.Name, "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetTeamByName(team.Name, "")
	CheckNoError(t, resp)

	th.LoginTeamAdmin()

	team2 := &model.Team{DisplayName: "Name", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen, AllowOpenInvite: false}
	rteam2, _ := th.Client.CreateTeam(team2)

	team3 := &model.Team{DisplayName: "Name", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamInvite, AllowOpenInvite: true}
	rteam3, _ := th.Client.CreateTeam(team3)

	th.LoginBasic()
	// AllowInviteOpen is false and team is open, and user is not on team
	_, resp = th.Client.GetTeamByName(rteam2.Name, "")
	CheckForbiddenStatus(t, resp)

	// AllowInviteOpen is true and team is invite only, and user is not on team
	_, resp = th.Client.GetTeamByName(rteam3.Name, "")
	CheckForbiddenStatus(t, resp)
}

func TestGetTeamByNameSanitization(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:    t.Name() + "_1",
		Name:           GenerateTestTeamName(),
		Email:          th.GenerateTestEmail(),
		Type:           model.TeamOpen,
		AllowedDomains: "simulator.amazonses.com,localhost",
	})
	CheckNoError(t, resp)

	t.Run("team user", func(t *testing.T) {
		th.LinkUserToTeam(th.BasicUser2, team)

		client := th.CreateClient()
		th.LoginBasic2WithClient(client)

		rteam, resp := client.GetTeamByName(team.Name, "")
		CheckNoError(t, resp)

		require.Empty(t, rteam.Email, "should've sanitized email")
		require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
	})

	t.Run("team user without invite permissions", func(t *testing.T) {
		th.RemovePermissionFromRole(model.PermissionInviteUser.ID, model.TeamUserRoleID)
		th.LinkUserToTeam(th.BasicUser2, team)

		client := th.CreateClient()

		th.LoginBasic2WithClient(client)

		rteam, resp := client.GetTeam(team.ID, "")
		CheckNoError(t, resp)

		require.Empty(t, rteam.Email, "should have sanitized email")
		require.Empty(t, rteam.InviteID, "should have sanitized inviteid")
	})

	t.Run("team admin/non-admin", func(t *testing.T) {
		rteam, resp := th.Client.GetTeamByName(team.Name, "")
		CheckNoError(t, resp)

		require.NotEmpty(t, rteam.Email, "should not have sanitized email")
		require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
	})

	t.Run("system admin", func(t *testing.T) {
		rteam, resp := th.SystemAdminClient.GetTeamByName(team.Name, "")
		CheckNoError(t, resp)

		require.NotEmpty(t, rteam.Email, "should not have sanitized email")
		require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
	})
}

func TestSearchAllTeams(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	oTeam := th.BasicTeam
	oTeam.AllowOpenInvite = true

	updatedTeam, err := th.App.UpdateTeam(oTeam)
	require.Nil(t, err, err)
	oTeam.UpdateAt = updatedTeam.UpdateAt

	pTeam := &model.Team{DisplayName: "PName", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamInvite}
	th.Client.CreateTeam(pTeam)

	rteams, resp := th.Client.SearchTeams(&model.TeamSearch{Term: pTeam.Name})
	CheckNoError(t, resp)
	require.Empty(t, rteams, "should have not returned team")

	rteams, resp = th.Client.SearchTeams(&model.TeamSearch{Term: pTeam.DisplayName})
	CheckNoError(t, resp)
	require.Empty(t, rteams, "should have not returned team")

	th.Client.Logout()

	_, resp = th.Client.SearchTeams(&model.TeamSearch{Term: pTeam.Name})
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.Client.SearchTeams(&model.TeamSearch{Term: pTeam.DisplayName})
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic()

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		rteams, resp = client.SearchTeams(&model.TeamSearch{Term: oTeam.Name})
		CheckNoError(t, resp)
		require.Len(t, rteams, 1, "should have returned 1 team")
		require.Equal(t, oTeam.ID, rteams[0].ID, "invalid team")

		rteams, resp = client.SearchTeams(&model.TeamSearch{Term: oTeam.DisplayName})
		CheckNoError(t, resp)
		require.Len(t, rteams, 1, "should have returned 1 team")
		require.Equal(t, oTeam.ID, rteams[0].ID, "invalid team")

		rteams, resp = client.SearchTeams(&model.TeamSearch{Term: "junk"})
		CheckNoError(t, resp)
		require.Empty(t, rteams, "should have not returned team")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		rteams, resp = client.SearchTeams(&model.TeamSearch{Term: oTeam.Name})
		CheckNoError(t, resp)
		require.Len(t, rteams, 1, "should have returned 1 team")

		rteams, resp = client.SearchTeams(&model.TeamSearch{Term: pTeam.DisplayName})
		CheckNoError(t, resp)
		require.Len(t, rteams, 1, "should have returned 1 team")
	})

	// Choose a team which the system manager can access
	sysManagerTeams, resp := th.SystemManagerClient.GetAllTeams("", 0, 10000)
	CheckOKStatus(t, resp)
	policyTeam := sysManagerTeams[0]
	// Now actually create the policy and assign the team to it
	policy, savePolicyErr := th.App.Srv().Store.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:  "Policy 1",
			PostDuration: model.NewInt64(30),
		},
		TeamIDs: []string{policyTeam.ID},
	})
	require.NoError(t, savePolicyErr)
	t.Run("does not return policy ID", func(t *testing.T) {
		teams, sysManagerResp := th.SystemManagerClient.SearchTeams(&model.TeamSearch{Term: policyTeam.Name})
		CheckOKStatus(t, sysManagerResp)
		found := false
		for _, team := range teams {
			if team.ID == policyTeam.ID {
				found = true
				require.Nil(t, team.PolicyID)
				break
			}
		}
		require.True(t, found)
	})
	t.Run("returns policy ID", func(t *testing.T) {
		teams, sysAdminResp := th.SystemAdminClient.SearchTeams(&model.TeamSearch{Term: policyTeam.Name})
		CheckOKStatus(t, sysAdminResp)
		found := false
		for _, team := range teams {
			if team.ID == policyTeam.ID {
				found = true
				require.Equal(t, *team.PolicyID, policy.ID)
				break
			}
		}
		require.True(t, found)
	})
}

func TestSearchAllTeamsPaged(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	commonRandom := model.NewID()
	teams := [3]*model.Team{}

	for i := 0; i < 3; i++ {
		uid := model.NewID()
		newTeam, err := th.App.CreateTeam(th.Context, &model.Team{
			DisplayName: fmt.Sprintf("%s %d %s", commonRandom, i, uid),
			Name:        fmt.Sprintf("%s-%d-%s", commonRandom, i, uid),
			Type:        model.TeamOpen,
			Email:       th.GenerateTestEmail(),
		})
		require.Nil(t, err)
		teams[i] = newTeam
	}

	foobarTeam, err := th.App.CreateTeam(th.Context, &model.Team{
		DisplayName: "FOOBARDISPLAYNAME",
		Name:        "whatever",
		Type:        model.TeamOpen,
		Email:       th.GenerateTestEmail(),
	})
	require.Nil(t, err)

	testCases := []struct {
		Name               string
		Search             *model.TeamSearch
		ExpectedTeams      []string
		ExpectedTotalCount int64
	}{
		{
			Name:               "Retrieve foobar team using partial term search",
			Search:             &model.TeamSearch{Term: "oobardisplay", Page: model.NewInt(0), PerPage: model.NewInt(100)},
			ExpectedTeams:      []string{foobarTeam.ID},
			ExpectedTotalCount: 1,
		},
		{
			Name:               "Retrieve foobar team using the beginning of the display name as search text",
			Search:             &model.TeamSearch{Term: "foobar", Page: model.NewInt(0), PerPage: model.NewInt(100)},
			ExpectedTeams:      []string{foobarTeam.ID},
			ExpectedTotalCount: 1,
		},
		{
			Name:               "Retrieve foobar team using the ending of the term of the display name",
			Search:             &model.TeamSearch{Term: "bardisplayname", Page: model.NewInt(0), PerPage: model.NewInt(100)},
			ExpectedTeams:      []string{foobarTeam.ID},
			ExpectedTotalCount: 1,
		},
		{
			Name:               "Retrieve foobar team using partial term search on the name property of team",
			Search:             &model.TeamSearch{Term: "what", Page: model.NewInt(0), PerPage: model.NewInt(100)},
			ExpectedTeams:      []string{foobarTeam.ID},
			ExpectedTotalCount: 1,
		},
		{
			Name:               "Retrieve foobar team using partial term search on the name property of team #2",
			Search:             &model.TeamSearch{Term: "ever", Page: model.NewInt(0), PerPage: model.NewInt(100)},
			ExpectedTeams:      []string{foobarTeam.ID},
			ExpectedTotalCount: 1,
		},
		{
			Name:               "Get all teams on one page",
			Search:             &model.TeamSearch{Term: commonRandom, Page: model.NewInt(0), PerPage: model.NewInt(100)},
			ExpectedTeams:      []string{teams[0].ID, teams[1].ID, teams[2].ID},
			ExpectedTotalCount: 3,
		},
		{
			Name:               "Get all teams on one page with partial word",
			Search:             &model.TeamSearch{Term: commonRandom[11:18]},
			ExpectedTeams:      []string{teams[0].ID, teams[1].ID, teams[2].ID},
			ExpectedTotalCount: 3,
		},
		{
			Name:               "Get all teams on one page with term upper cased",
			Search:             &model.TeamSearch{Term: strings.ToUpper(commonRandom)},
			ExpectedTeams:      []string{teams[0].ID, teams[1].ID, teams[2].ID},
			ExpectedTotalCount: 3,
		},
		{
			Name:               "Get all teams on one page with some of term upper and some lower",
			Search:             &model.TeamSearch{Term: commonRandom[0:11] + strings.ToUpper(commonRandom[11:18]+commonRandom[18:])},
			ExpectedTeams:      []string{teams[0].ID, teams[1].ID, teams[2].ID},
			ExpectedTotalCount: 3,
		},
		{
			Name:               "Get 2 teams on the first page",
			Search:             &model.TeamSearch{Term: commonRandom, Page: model.NewInt(0), PerPage: model.NewInt(2)},
			ExpectedTeams:      []string{teams[0].ID, teams[1].ID},
			ExpectedTotalCount: 3,
		},
		{
			Name:               "Get 1 team on the second page",
			Search:             &model.TeamSearch{Term: commonRandom, Page: model.NewInt(1), PerPage: model.NewInt(2)},
			ExpectedTeams:      []string{teams[2].ID},
			ExpectedTotalCount: 3,
		},
		{
			Name:               "SearchTeamsPaged paginates results by default",
			Search:             &model.TeamSearch{Term: commonRandom},
			ExpectedTeams:      []string{teams[0].ID, teams[1].ID, teams[2].ID},
			ExpectedTotalCount: 3,
		},
		{
			Name:               "No results",
			Search:             &model.TeamSearch{Term: model.NewID()},
			ExpectedTeams:      []string{},
			ExpectedTotalCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			teams, count, resp := th.SystemAdminClient.SearchTeamsPaged(tc.Search)
			require.Nil(t, resp.Error)
			require.Equal(t, tc.ExpectedTotalCount, count)
			require.Equal(t, len(tc.ExpectedTeams), len(teams))
			for i, team := range teams {
				require.Equal(t, tc.ExpectedTeams[i], team.ID)
			}
		})
	}

	_, _, resp := th.Client.SearchTeamsPaged(&model.TeamSearch{Term: commonRandom, PerPage: model.NewInt(100)})
	require.Equal(t, "api.team.search_teams.pagination_not_implemented.public_team_search", resp.Error.ID)
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
}

func TestSearchAllTeamsSanitization(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:    t.Name() + "_1",
		Name:           GenerateTestTeamName(),
		Email:          th.GenerateTestEmail(),
		Type:           model.TeamOpen,
		AllowedDomains: "simulator.amazonses.com,localhost",
	})
	CheckNoError(t, resp)
	team2, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:    t.Name() + "_2",
		Name:           GenerateTestTeamName(),
		Email:          th.GenerateTestEmail(),
		Type:           model.TeamOpen,
		AllowedDomains: "simulator.amazonses.com,localhost",
	})
	CheckNoError(t, resp)

	t.Run("non-team user", func(t *testing.T) {
		client := th.CreateClient()
		th.LoginBasic2WithClient(client)

		rteams, resp := client.SearchTeams(&model.TeamSearch{Term: t.Name()})
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			require.Empty(t, rteam.Email, "should've sanitized email")
			require.Empty(t, rteam.AllowedDomains, "should've sanitized allowed domains")
			require.Empty(t, rteam.InviteID, "should have sanitized inviteid")
		}
	})

	t.Run("team user", func(t *testing.T) {
		th.LinkUserToTeam(th.BasicUser2, team)

		client := th.CreateClient()
		th.LoginBasic2WithClient(client)

		rteams, resp := client.SearchTeams(&model.TeamSearch{Term: t.Name()})
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			require.Empty(t, rteam.Email, "should've sanitized email")
			require.Empty(t, rteam.AllowedDomains, "should've sanitized allowed domains")
			require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
		}
	})

	t.Run("team admin", func(t *testing.T) {
		rteams, resp := th.Client.SearchTeams(&model.TeamSearch{Term: t.Name()})
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			if rteam.ID == team.ID || rteam.ID == team2.ID || rteam.ID == th.BasicTeam.ID {
				require.NotEmpty(t, rteam.Email, "should not have sanitized email")
				require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
			}
		}
	})

	t.Run("system admin", func(t *testing.T) {
		rteams, resp := th.SystemAdminClient.SearchTeams(&model.TeamSearch{Term: t.Name()})
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			require.NotEmpty(t, rteam.Email, "should not have sanitized email")
			require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
		}
	})
}

func TestGetTeamsForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	team2 := &model.Team{DisplayName: "Name", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamInvite}
	rteam2, _ := Client.CreateTeam(team2)

	teams, resp := Client.GetTeamsForUser(th.BasicUser.ID, "")
	CheckNoError(t, resp)

	require.Len(t, teams, 2, "wrong number of teams")

	found1 := false
	found2 := false
	for _, t := range teams {
		if t.ID == th.BasicTeam.ID {
			found1 = true
		} else if t.ID == rteam2.ID {
			found2 = true
		}
	}

	require.True(t, found1, "missing team")
	require.True(t, found2, "missing team")

	_, resp = Client.GetTeamsForUser("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamsForUser(model.NewID(), "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetTeamsForUser(th.BasicUser2.ID, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetTeamsForUser(th.BasicUser2.ID, "")
	CheckNoError(t, resp)
}

func TestGetTeamsForUserSanitization(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:    t.Name() + "_1",
		Name:           GenerateTestTeamName(),
		Email:          th.GenerateTestEmail(),
		Type:           model.TeamOpen,
		AllowedDomains: "simulator.amazonses.com,localhost",
	})
	CheckNoError(t, resp)
	team2, resp := th.Client.CreateTeam(&model.Team{
		DisplayName:    t.Name() + "_2",
		Name:           GenerateTestTeamName(),
		Email:          th.GenerateTestEmail(),
		Type:           model.TeamOpen,
		AllowedDomains: "simulator.amazonses.com,localhost",
	})
	CheckNoError(t, resp)

	t.Run("team user", func(t *testing.T) {
		th.LinkUserToTeam(th.BasicUser2, team)
		th.LinkUserToTeam(th.BasicUser2, team2)

		client := th.CreateClient()
		th.LoginBasic2WithClient(client)

		rteams, resp := client.GetTeamsForUser(th.BasicUser2.ID, "")
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			if rteam.ID != team.ID && rteam.ID != team2.ID {
				continue
			}

			require.Empty(t, rteam.Email, "should've sanitized email")
			require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
		}
	})

	t.Run("team user without invite permissions", func(t *testing.T) {
		th.LinkUserToTeam(th.BasicUser2, team)
		th.LinkUserToTeam(th.BasicUser2, team2)

		client := th.CreateClient()
		th.RemovePermissionFromRole(model.PermissionInviteUser.ID, model.TeamUserRoleID)
		th.LoginBasic2WithClient(client)

		rteams, resp := client.GetTeamsForUser(th.BasicUser2.ID, "")
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			if rteam.ID != team.ID && rteam.ID != team2.ID {
				continue
			}

			require.Empty(t, rteam.Email, "should have sanitized email")
			require.Empty(t, rteam.InviteID, "should have sanitized inviteid")
		}
	})

	t.Run("team admin", func(t *testing.T) {
		rteams, resp := th.Client.GetTeamsForUser(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			if rteam.ID != team.ID && rteam.ID != team2.ID {
				continue
			}

			require.NotEmpty(t, rteam.Email, "should not have sanitized email")
			require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
		}
	})

	t.Run("system admin", func(t *testing.T) {
		rteams, resp := th.SystemAdminClient.GetTeamsForUser(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		for _, rteam := range rteams {
			if rteam.ID != team.ID && rteam.ID != team2.ID {
				continue
			}

			require.NotEmpty(t, rteam.Email, "should not have sanitized email")
			require.NotEmpty(t, rteam.InviteID, "should have not sanitized inviteid")
		}
	})
}

func TestGetTeamMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam
	user := th.BasicUser

	rmember, resp := Client.GetTeamMember(team.ID, user.ID, "")
	CheckNoError(t, resp)

	require.Equal(t, rmember.TeamID, team.ID, "wrong team id")

	require.Equal(t, rmember.UserID, user.ID, "wrong user id")

	_, resp = Client.GetTeamMember("junk", user.ID, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamMember(team.ID, "junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamMember("junk", "junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamMember(team.ID, model.NewID(), "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetTeamMember(model.NewID(), user.ID, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetTeamMember(team.ID, user.ID, "")
	CheckNoError(t, resp)
}

func TestGetTeamMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam
	userNotMember := th.CreateUser()

	rmembers, resp := Client.GetTeamMembers(team.ID, 0, 100, "")
	CheckNoError(t, resp)

	t.Logf("rmembers count %v\n", len(rmembers))

	require.NotEqual(t, len(rmembers), 0, "should have results")

	for _, rmember := range rmembers {
		require.Equal(t, rmember.TeamID, team.ID, "user should be a member of team")
		require.NotEqual(t, rmember.UserID, userNotMember.ID, "user should be a member of team")
	}

	rmembers, resp = Client.GetTeamMembers(team.ID, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rmembers, 1, "should be 1 per page")

	rmembers, resp = Client.GetTeamMembers(team.ID, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rmembers, 1, "should be 1 per page")

	rmembers, resp = Client.GetTeamMembers(team.ID, 10000, 100, "")
	CheckNoError(t, resp)
	require.Empty(t, rmembers, "should be no member")

	rmembers, resp = Client.GetTeamMembers(team.ID, 0, 2, "")
	CheckNoError(t, resp)
	rmembers2, resp := Client.GetTeamMembers(team.ID, 1, 2, "")
	CheckNoError(t, resp)

	for _, tm1 := range rmembers {
		for _, tm2 := range rmembers2 {
			assert.NotEqual(t, tm1.UserID+tm1.TeamID, tm2.UserID+tm2.TeamID, "different pages should not have the same members")
		}
	}

	_, resp = Client.GetTeamMembers("junk", 0, 100, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamMembers(model.NewID(), 0, 100, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetTeamMembers(team.ID, 0, 1, "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetTeamMembersSortAndWithoutDeletedUsers(team.ID, 0, 100, "", false, "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetTeamMembersSortAndWithoutDeletedUsers(team.ID, 0, 100, model.USERNAME, false, "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetTeamMembersSortAndWithoutDeletedUsers(team.ID, 0, 100, model.USERNAME, true, "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetTeamMembersSortAndWithoutDeletedUsers(team.ID, 0, 100, "", true, "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetTeamMembersSortAndWithoutDeletedUsers(team.ID, 0, 100, model.USERNAME, false, "")
	CheckNoError(t, resp)
}

func TestGetTeamMembersForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	members, resp := Client.GetTeamMembersForUser(th.BasicUser.ID, "")
	CheckNoError(t, resp)

	found := false
	for _, m := range members {
		if m.TeamID == th.BasicTeam.ID {
			found = true
		}
	}

	require.True(t, found, "missing team member")

	_, resp = Client.GetTeamMembersForUser("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamMembersForUser(model.NewID(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetTeamMembersForUser(th.BasicUser.ID, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetTeamMembersForUser(th.BasicUser.ID, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetTeamMembersForUser(th.BasicUser.ID, "")
	CheckNoError(t, resp)
}

func TestGetTeamMembersByIDs(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	tm, resp := Client.GetTeamMembersByIDs(th.BasicTeam.ID, []string{th.BasicUser.ID})
	CheckNoError(t, resp)

	require.Equal(t, tm[0].UserID, th.BasicUser.ID, "returned wrong user")

	_, resp = Client.GetTeamMembersByIDs(th.BasicTeam.ID, []string{})
	CheckBadRequestStatus(t, resp)

	tm1, resp := Client.GetTeamMembersByIDs(th.BasicTeam.ID, []string{"junk"})
	CheckNoError(t, resp)
	require.False(t, len(tm1) > 0, "no users should be returned")

	tm1, resp = Client.GetTeamMembersByIDs(th.BasicTeam.ID, []string{"junk", th.BasicUser.ID})
	CheckNoError(t, resp)
	require.Len(t, tm1, 1, "1 user should be returned")

	_, resp = Client.GetTeamMembersByIDs("junk", []string{th.BasicUser.ID})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamMembersByIDs(model.NewID(), []string{th.BasicUser.ID})
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetTeamMembersByIDs(th.BasicTeam.ID, []string{th.BasicUser.ID})
	CheckUnauthorizedStatus(t, resp)
}

func TestAddTeamMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam
	otherUser := th.CreateUser()

	th.App.Srv().SetLicense(model.NewTestLicense(""))
	defer th.App.Srv().SetLicense(nil)

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.GuestAccountsSettings.Enable = &enableGuestAccounts })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })

	guest := th.CreateUser()
	_, resp := th.SystemAdminClient.DemoteUserToGuest(guest.ID)
	CheckNoError(t, resp)

	err := th.App.RemoveUserFromTeam(th.Context, th.BasicTeam.ID, th.BasicUser2.ID, "")
	if err != nil {
		require.FailNow(t, err.Error())
	}

	// Regular user can't add a member to a team they don't belong to.
	th.LoginBasic2()
	_, resp = Client.AddTeamMember(team.ID, otherUser.ID)
	CheckForbiddenStatus(t, resp)
	require.NotNil(t, resp.Error, "Error is nil")
	Client.Logout()

	// SystemAdmin and mode can add member to a team
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		tm, r := client.AddTeamMember(team.ID, otherUser.ID)
		CheckNoError(t, r)
		CheckCreatedStatus(t, r)
		require.Equal(t, tm.UserID, otherUser.ID, "user ids should have matched")
		require.Equal(t, tm.TeamID, team.ID, "team ids should have matched")
	})

	// Regular user can add a member to a team they belong to.
	th.LoginBasic()
	tm, resp := Client.AddTeamMember(team.ID, otherUser.ID)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	// Check all the returned data.
	require.NotNil(t, tm, "should have returned team member")

	require.Equal(t, tm.UserID, otherUser.ID, "user ids should have matched")

	require.Equal(t, tm.TeamID, team.ID, "team ids should have matched")

	// Check with various invalid requests.
	tm, resp = Client.AddTeamMember(team.ID, "junk")
	CheckBadRequestStatus(t, resp)

	require.Nil(t, tm, "should have not returned team member")

	_, resp = Client.AddTeamMember("junk", otherUser.ID)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.AddTeamMember(GenerateTestID(), otherUser.ID)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.AddTeamMember(team.ID, GenerateTestID())
	CheckNotFoundStatus(t, resp)

	Client.Logout()

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// Set the config so that only team admins can add a user to a team.
	th.AddPermissionToRole(model.PermissionInviteUser.ID, model.TeamAdminRoleID)
	th.AddPermissionToRole(model.PermissionAddUserToTeam.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionInviteUser.ID, model.TeamUserRoleID)
	th.RemovePermissionFromRole(model.PermissionAddUserToTeam.ID, model.TeamUserRoleID)

	th.LoginBasic()

	// Check that a regular user can't add someone to the team.
	_, resp = Client.AddTeamMember(team.ID, otherUser.ID)
	CheckForbiddenStatus(t, resp)

	// Update user to team admin
	th.UpdateUserToTeamAdmin(th.BasicUser, th.BasicTeam)
	th.App.Srv().InvalidateAllCaches()
	th.LoginBasic()

	// Should work as a team admin.
	_, resp = Client.AddTeamMember(team.ID, otherUser.ID)
	CheckNoError(t, resp)

	// Change permission level to team user
	th.AddPermissionToRole(model.PermissionInviteUser.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionAddUserToTeam.ID, model.TeamUserRoleID)
	th.RemovePermissionFromRole(model.PermissionInviteUser.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionAddUserToTeam.ID, model.TeamAdminRoleID)

	th.UpdateUserToNonTeamAdmin(th.BasicUser, th.BasicTeam)
	th.App.Srv().InvalidateAllCaches()
	th.LoginBasic()

	// Should work as a regular user.
	_, resp = Client.AddTeamMember(team.ID, otherUser.ID)
	CheckNoError(t, resp)

	// Should return error with invalid JSON in body.
	_, err = Client.DoApiPost("/teams/"+team.ID+"/members", "invalid")
	require.NotNil(t, err)
	require.Equal(t, "api.team.add_team_member.invalid_body.app_error", err.ID)

	// by token
	Client.Login(otherUser.Email, otherUser.Password)

	token := model.NewToken(
		app.TokenTypeTeamInvitation,
		model.MapToJSON(map[string]string{"teamId": team.ID}),
	)
	require.NoError(t, th.App.Srv().Store.Token().Save(token))

	tm, resp = Client.AddTeamMemberFromInvite(token.Token, "")
	CheckNoError(t, resp)

	require.NotNil(t, tm, "should have returned team member")

	require.Equal(t, tm.UserID, otherUser.ID, "user ids should have matched")

	require.Equal(t, tm.TeamID, team.ID, "team ids should have matched")

	_, nErr := th.App.Srv().Store.Token().GetByToken(token.Token)
	require.Error(t, nErr, "The token must be deleted after be used")

	tm, resp = Client.AddTeamMemberFromInvite("junk", "")
	CheckBadRequestStatus(t, resp)

	require.Nil(t, tm, "should have not returned team member")

	// expired token of more than 50 hours
	token = model.NewToken(app.TokenTypeTeamInvitation, "")
	token.CreateAt = model.GetMillis() - 1000*60*60*50
	require.NoError(t, th.App.Srv().Store.Token().Save(token))

	_, resp = Client.AddTeamMemberFromInvite(token.Token, "")
	CheckBadRequestStatus(t, resp)
	th.App.DeleteToken(token)

	// invalid team id
	testID := GenerateTestID()
	token = model.NewToken(
		app.TokenTypeTeamInvitation,
		model.MapToJSON(map[string]string{"teamId": testID}),
	)
	require.NoError(t, th.App.Srv().Store.Token().Save(token))

	_, resp = Client.AddTeamMemberFromInvite(token.Token, "")
	CheckNotFoundStatus(t, resp)
	th.App.DeleteToken(token)

	// by invite_id
	th.App.Srv().SetLicense(model.NewTestLicense(""))
	defer th.App.Srv().SetLicense(nil)
	_, resp = Client.Login(guest.Email, guest.Password)
	CheckNoError(t, resp)

	tm, resp = Client.AddTeamMemberFromInvite("", team.InviteID)
	CheckForbiddenStatus(t, resp)

	// by invite_id
	Client.Login(otherUser.Email, otherUser.Password)

	tm, resp = Client.AddTeamMemberFromInvite("", team.InviteID)
	CheckNoError(t, resp)

	require.NotNil(t, tm, "should have returned team member")

	require.Equal(t, tm.UserID, otherUser.ID, "user ids should have matched")

	require.Equal(t, tm.TeamID, team.ID, "team ids should have matched")

	tm, resp = Client.AddTeamMemberFromInvite("", "junk")
	CheckNotFoundStatus(t, resp)

	require.Nil(t, tm, "should have not returned team member")

	// Set a team to group-constrained
	team.GroupConstrained = model.NewBool(true)
	_, err = th.App.UpdateTeam(team)
	require.Nil(t, err)

	// Attempt to use a token on a group-constrained team
	token = model.NewToken(
		app.TokenTypeTeamInvitation,
		model.MapToJSON(map[string]string{"teamId": team.ID}),
	)
	require.NoError(t, th.App.Srv().Store.Token().Save(token))
	tm, resp = Client.AddTeamMemberFromInvite(token.Token, "")
	require.Equal(t, "app.team.invite_token.group_constrained.error", resp.Error.ID)

	// Attempt to use an invite id
	tm, resp = Client.AddTeamMemberFromInvite("", team.InviteID)
	require.Equal(t, "app.team.invite_id.group_constrained.error", resp.Error.ID)

	// User is not in associated groups so shouldn't be allowed
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.AddTeamMember(team.ID, otherUser.ID)
		CheckErrorMessage(t, resp, "api.team.add_members.user_denied")
	})

	// Associate group to team
	_, err = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupID:    th.Group.ID,
		SyncableID: team.ID,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.Nil(t, err)

	// Add user to group
	_, err = th.App.UpsertGroupMember(th.Group.ID, otherUser.ID)
	require.Nil(t, err)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.AddTeamMember(team.ID, otherUser.ID)
		CheckNoError(t, resp)
	})
}

func TestAddTeamMemberMyself(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.LoginBasic()

	testCases := []struct {
		Name              string
		Public            bool
		PublicPermission  bool
		PrivatePermission bool
		ExpectedSuccess   bool
	}{
		{
			Name:              "Try to join an open team without the permissions",
			Public:            true,
			PublicPermission:  false,
			PrivatePermission: false,
			ExpectedSuccess:   false,
		},
		{
			Name:              "Try to join a private team without the permissions",
			Public:            false,
			PublicPermission:  false,
			PrivatePermission: false,
			ExpectedSuccess:   false,
		},
		{
			Name:              "Try to join an open team without public permission but with private permissions",
			Public:            true,
			PublicPermission:  false,
			PrivatePermission: true,
			ExpectedSuccess:   false,
		},
		{
			Name:              "Try to join a private team without private permission but with public permission",
			Public:            false,
			PublicPermission:  true,
			PrivatePermission: false,
			ExpectedSuccess:   false,
		},
		{
			Name:              "Join an open team with the permissions",
			Public:            true,
			PublicPermission:  true,
			PrivatePermission: false,
			ExpectedSuccess:   true,
		},
		{
			Name:              "Join a private team with the permissions",
			Public:            false,
			PublicPermission:  false,
			PrivatePermission: true,
			ExpectedSuccess:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			team := th.CreateTeam()
			team.AllowOpenInvite = tc.Public
			th.App.UpdateTeam(team)
			if tc.PublicPermission {
				th.AddPermissionToRole(model.PermissionJoinPublicTeams.ID, model.SystemUserRoleID)
			} else {
				th.RemovePermissionFromRole(model.PermissionJoinPublicTeams.ID, model.SystemUserRoleID)
			}
			if tc.PrivatePermission {
				th.AddPermissionToRole(model.PermissionJoinPrivateTeams.ID, model.SystemUserRoleID)
			} else {
				th.RemovePermissionFromRole(model.PermissionJoinPrivateTeams.ID, model.SystemUserRoleID)
			}
			_, resp := Client.AddTeamMember(team.ID, th.BasicUser.ID)
			if tc.ExpectedSuccess {
				CheckNoError(t, resp)
			} else {
				CheckForbiddenStatus(t, resp)
			}
		})
	}

}

func TestAddTeamMembersDomainConstrained(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	client := th.SystemAdminClient
	team := th.BasicTeam
	team.AllowedDomains = "domain1.com, domain2.com"
	_, response := client.UpdateTeam(team)
	require.Nil(t, response.Error)

	// create two users on allowed domains
	user1, response := client.CreateUser(&model.User{
		Email:    "user@domain1.com",
		Password: "Pa$$word11",
		Username: GenerateTestUsername(),
	})
	require.Nil(t, response.Error)
	user2, response := client.CreateUser(&model.User{
		Email:    "user@domain2.com",
		Password: "Pa$$word11",
		Username: GenerateTestUsername(),
	})
	require.Nil(t, response.Error)

	userList := []string{
		user1.ID,
		user2.ID,
	}

	// validate that they can be added
	tm, response := client.AddTeamMembers(team.ID, userList)
	require.Nil(t, response.Error)
	require.Len(t, tm, 2)

	// cleanup
	_, response = client.RemoveTeamMember(team.ID, user1.ID)
	require.Nil(t, response.Error)
	_, response = client.RemoveTeamMember(team.ID, user2.ID)
	require.Nil(t, response.Error)

	// disable one of the allowed domains
	team.AllowedDomains = "domain1.com"
	_, response = client.UpdateTeam(team)
	require.Nil(t, response.Error)

	// validate that they cannot be added
	_, response = client.AddTeamMembers(team.ID, userList)
	require.NotNil(t, response.Error)

	// validate that one user can be added gracefully
	members, response := client.AddTeamMembersGracefully(team.ID, userList)
	require.Nil(t, response.Error)
	require.Len(t, members, 2)
	require.NotNil(t, members[0].Member)
	require.NotNil(t, members[1].Error)
	require.Equal(t, members[0].UserID, user1.ID)
	require.Equal(t, members[1].UserID, user2.ID)
	require.Nil(t, members[0].Error)
	require.Nil(t, members[1].Member)
}

func TestAddTeamMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam
	otherUser := th.CreateUser()
	userList := []string{
		otherUser.ID,
	}

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})
	bot := th.CreateBotWithSystemAdminClient()

	err := th.App.RemoveUserFromTeam(th.Context, th.BasicTeam.ID, th.BasicUser2.ID, "")
	require.Nil(t, err)

	// Regular user can't add a member to a team they don't belong to.
	th.LoginBasic2()
	_, resp := Client.AddTeamMembers(team.ID, userList)
	CheckForbiddenStatus(t, resp)
	Client.Logout()

	// Regular user can add a member to a team they belong to.
	th.LoginBasic()
	tm, resp := Client.AddTeamMembers(team.ID, userList)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	// Check all the returned data.
	require.NotNil(t, tm[0], "should have returned team member")

	require.Equal(t, tm[0].UserID, otherUser.ID, "user ids should have matched")

	require.Equal(t, tm[0].TeamID, team.ID, "team ids should have matched")

	// Check with various invalid requests.
	_, resp = Client.AddTeamMembers("junk", userList)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.AddTeamMembers(GenerateTestID(), userList)
	CheckNotFoundStatus(t, resp)

	testUserList := append(userList, GenerateTestID())
	_, resp = Client.AddTeamMembers(team.ID, testUserList)
	CheckNotFoundStatus(t, resp)

	// Test with many users.
	for i := 0; i < 260; i++ {
		testUserList = append(testUserList, GenerateTestID())
	}
	_, resp = Client.AddTeamMembers(team.ID, testUserList)
	CheckBadRequestStatus(t, resp)

	Client.Logout()

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// Set the config so that only team admins can add a user to a team.
	th.AddPermissionToRole(model.PermissionInviteUser.ID, model.TeamAdminRoleID)
	th.AddPermissionToRole(model.PermissionAddUserToTeam.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionInviteUser.ID, model.TeamUserRoleID)
	th.RemovePermissionFromRole(model.PermissionAddUserToTeam.ID, model.TeamUserRoleID)

	th.LoginBasic()

	// Check that a regular user can't add someone to the team.
	_, resp = Client.AddTeamMembers(team.ID, userList)
	CheckForbiddenStatus(t, resp)

	// Update user to team admin
	th.UpdateUserToTeamAdmin(th.BasicUser, th.BasicTeam)
	th.App.Srv().InvalidateAllCaches()
	th.LoginBasic()

	// Should work as a team admin.
	_, resp = Client.AddTeamMembers(team.ID, userList)
	CheckNoError(t, resp)

	// Change permission level to team user
	th.AddPermissionToRole(model.PermissionInviteUser.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionAddUserToTeam.ID, model.TeamUserRoleID)
	th.RemovePermissionFromRole(model.PermissionInviteUser.ID, model.TeamAdminRoleID)
	th.RemovePermissionFromRole(model.PermissionAddUserToTeam.ID, model.TeamAdminRoleID)

	th.UpdateUserToNonTeamAdmin(th.BasicUser, th.BasicTeam)
	th.App.Srv().InvalidateAllCaches()
	th.LoginBasic()

	// Should work as a regular user.
	_, resp = Client.AddTeamMembers(team.ID, userList)
	CheckNoError(t, resp)

	// Set a team to group-constrained
	team.GroupConstrained = model.NewBool(true)
	_, err = th.App.UpdateTeam(team)
	require.Nil(t, err)

	// User is not in associated groups so shouldn't be allowed
	_, resp = Client.AddTeamMembers(team.ID, userList)
	CheckErrorMessage(t, resp, "api.team.add_members.user_denied")

	// Ensure that a group synced team can still add bots
	_, resp = Client.AddTeamMembers(team.ID, []string{bot.UserID})
	CheckNoError(t, resp)

	// Associate group to team
	_, err = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupID:    th.Group.ID,
		SyncableID: team.ID,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.Nil(t, err)

	// Add user to group
	_, err = th.App.UpsertGroupMember(th.Group.ID, userList[0])
	require.Nil(t, err)

	_, resp = Client.AddTeamMembers(team.ID, userList)
	CheckNoError(t, resp)
}

func TestRemoveTeamMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})
	bot := th.CreateBotWithSystemAdminClient()

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		pass, resp := client.RemoveTeamMember(th.BasicTeam.ID, th.BasicUser.ID)
		CheckNoError(t, resp)

		require.True(t, pass, "should have passed")

		_, resp = th.SystemAdminClient.AddTeamMember(th.BasicTeam.ID, th.BasicUser.ID)
		CheckNoError(t, resp)
	})

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.RemoveTeamMember(th.BasicTeam.ID, "junk")
		CheckBadRequestStatus(t, resp)

		_, resp = client.RemoveTeamMember("junk", th.BasicUser2.ID)
		CheckBadRequestStatus(t, resp)
	})

	_, resp := Client.RemoveTeamMember(th.BasicTeam.ID, th.BasicUser2.ID)
	CheckForbiddenStatus(t, resp)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.RemoveTeamMember(model.NewID(), th.BasicUser.ID)
		CheckNotFoundStatus(t, resp)
	})

	_, resp = th.SystemAdminClient.AddTeamMember(th.BasicTeam.ID, th.SystemAdminUser.ID)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.AddTeamMember(th.BasicTeam.ID, bot.UserID)
	CheckNoError(t, resp)

	// If the team is group-constrained the user cannot be removed
	th.BasicTeam.GroupConstrained = model.NewBool(true)
	_, err := th.App.UpdateTeam(th.BasicTeam)
	require.Nil(t, err)
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.RemoveTeamMember(th.BasicTeam.ID, th.BasicUser.ID)
		require.Equal(t, "api.team.remove_member.group_constrained.app_error", resp.Error.ID)
	})

	// Can remove a bot even if team is group-constrained

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.RemoveTeamMember(th.BasicTeam.ID, bot.UserID)
		CheckNoError(t, resp)
		_, resp = client.AddTeamMember(th.BasicTeam.ID, bot.UserID)
		CheckNoError(t, resp)
	})

	// Can remove self even if team is group-constrained
	_, resp = th.SystemAdminClient.RemoveTeamMember(th.BasicTeam.ID, th.SystemAdminUser.ID)
	CheckNoError(t, resp)
}

func TestGetTeamStats(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	rstats, resp := Client.GetTeamStats(team.ID, "")
	CheckNoError(t, resp)

	require.Equal(t, rstats.TeamID, team.ID, "wrong team id")

	require.Equal(t, rstats.TotalMemberCount, int64(3), "wrong count")

	require.Equal(t, rstats.ActiveMemberCount, int64(3), "wrong count")

	_, resp = Client.GetTeamStats("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamStats(model.NewID(), "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetTeamStats(team.ID, "")
	CheckNoError(t, resp)

	// deactivate BasicUser2
	th.UpdateActiveUser(th.BasicUser2, false)

	rstats, resp = th.SystemAdminClient.GetTeamStats(team.ID, "")
	CheckNoError(t, resp)

	require.Equal(t, rstats.TotalMemberCount, int64(3), "wrong count")

	require.Equal(t, rstats.ActiveMemberCount, int64(2), "wrong count")

	// login with different user and test if forbidden
	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetTeamStats(th.BasicTeam.ID, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetTeamStats(th.BasicTeam.ID, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestUpdateTeamMemberRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	SystemAdminClient := th.SystemAdminClient

	const TeamMember = "team_user"
	const TeamAdmin = "team_user team_admin"

	// user 1 tries to promote user 2
	ok, resp := Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser2.ID, TeamAdmin)
	CheckForbiddenStatus(t, resp)
	require.False(t, ok, "should have returned false")

	// user 1 tries to promote himself
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser.ID, TeamAdmin)
	CheckForbiddenStatus(t, resp)

	// user 1 tries to demote someone
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.SystemAdminUser.ID, TeamMember)
	CheckForbiddenStatus(t, resp)

	// system admin promotes user 1
	ok, resp = SystemAdminClient.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser.ID, TeamAdmin)
	CheckNoError(t, resp)
	require.True(t, ok, "should have returned true")

	// user 1 (team admin) promotes user 2
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser2.ID, TeamAdmin)
	CheckNoError(t, resp)

	// user 1 (team admin) demotes user 2 (team admin)
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser2.ID, TeamMember)
	CheckNoError(t, resp)

	// user 1 (team admin) tries to demote system admin (not member of a team)
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.SystemAdminUser.ID, TeamMember)
	CheckNotFoundStatus(t, resp)

	// user 1 (team admin) demotes system admin (member of a team)
	th.LinkUserToTeam(th.SystemAdminUser, th.BasicTeam)
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.SystemAdminUser.ID, TeamMember)
	CheckNoError(t, resp)
	// Note from API v3
	// Note to anyone who thinks this (above) test is wrong:
	// This operation will not affect the system admin's permissions because they have global access to all teams.
	// Their team level permissions are irrelevant. A team admin should be able to manage team level permissions.

	// System admins should be able to manipulate permission no matter what their team level permissions are.
	// system admin promotes user 2
	_, resp = SystemAdminClient.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser2.ID, TeamAdmin)
	CheckNoError(t, resp)

	// system admin demotes user 2 (team admin)
	_, resp = SystemAdminClient.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser2.ID, TeamMember)
	CheckNoError(t, resp)

	// user 1 (team admin) tries to promote himself to a random team
	_, resp = Client.UpdateTeamMemberRoles(model.NewID(), th.BasicUser.ID, TeamAdmin)
	CheckForbiddenStatus(t, resp)

	// user 1 (team admin) tries to promote a random user
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, model.NewID(), TeamAdmin)
	CheckNotFoundStatus(t, resp)

	// user 1 (team admin) tries to promote invalid team permission
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser.ID, "junk")
	CheckBadRequestStatus(t, resp)

	// user 1 (team admin) demotes himself
	_, resp = Client.UpdateTeamMemberRoles(th.BasicTeam.ID, th.BasicUser.ID, TeamMember)
	CheckNoError(t, resp)
}

func TestUpdateTeamMemberSchemeRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	SystemAdminClient := th.SystemAdminClient
	th.LoginBasic()

	s1 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: false,
	}
	_, r1 := SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, th.BasicUser.ID, s1)
	CheckNoError(t, r1)

	tm1, rtm1 := SystemAdminClient.GetTeamMember(th.BasicTeam.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm1)
	assert.Equal(t, false, tm1.SchemeGuest)
	assert.Equal(t, false, tm1.SchemeUser)
	assert.Equal(t, false, tm1.SchemeAdmin)

	s2 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, r2 := SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, th.BasicUser.ID, s2)
	CheckNoError(t, r2)

	tm2, rtm2 := SystemAdminClient.GetTeamMember(th.BasicTeam.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm2)
	assert.Equal(t, false, tm2.SchemeGuest)
	assert.Equal(t, true, tm2.SchemeUser)
	assert.Equal(t, false, tm2.SchemeAdmin)

	s3 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  false,
		SchemeGuest: false,
	}
	_, r3 := SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, th.BasicUser.ID, s3)
	CheckNoError(t, r3)

	tm3, rtm3 := SystemAdminClient.GetTeamMember(th.BasicTeam.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm3)
	assert.Equal(t, false, tm3.SchemeGuest)
	assert.Equal(t, false, tm3.SchemeUser)
	assert.Equal(t, true, tm3.SchemeAdmin)

	s4 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, r4 := SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, th.BasicUser.ID, s4)
	CheckNoError(t, r4)

	tm4, rtm4 := SystemAdminClient.GetTeamMember(th.BasicTeam.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm4)
	assert.Equal(t, false, tm4.SchemeGuest)
	assert.Equal(t, true, tm4.SchemeUser)
	assert.Equal(t, true, tm4.SchemeAdmin)

	s5 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: true,
	}
	_, r5 := SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, th.BasicUser.ID, s5)
	CheckNoError(t, r5)

	tm5, rtm5 := SystemAdminClient.GetTeamMember(th.BasicTeam.ID, th.BasicUser.ID, "")
	CheckNoError(t, rtm5)
	assert.Equal(t, true, tm5.SchemeGuest)
	assert.Equal(t, false, tm5.SchemeUser)
	assert.Equal(t, false, tm5.SchemeAdmin)

	s6 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: true,
	}
	_, resp := SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, th.BasicUser.ID, s6)
	CheckBadRequestStatus(t, resp)

	_, resp = SystemAdminClient.UpdateTeamMemberSchemeRoles(model.NewID(), th.BasicUser.ID, s4)
	CheckNotFoundStatus(t, resp)

	_, resp = SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, model.NewID(), s4)
	CheckNotFoundStatus(t, resp)

	_, resp = SystemAdminClient.UpdateTeamMemberSchemeRoles("ASDF", th.BasicUser.ID, s4)
	CheckBadRequestStatus(t, resp)

	_, resp = SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, "ASDF", s4)
	CheckBadRequestStatus(t, resp)

	th.LoginBasic2()
	_, resp = th.Client.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, th.BasicUser.ID, s4)
	CheckForbiddenStatus(t, resp)

	SystemAdminClient.Logout()
	_, resp = SystemAdminClient.UpdateTeamMemberSchemeRoles(th.BasicTeam.ID, th.SystemAdminUser.ID, s4)
	CheckUnauthorizedStatus(t, resp)
}

func TestGetMyTeamsUnread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	user := th.BasicUser
	Client.Login(user.Email, user.Password)

	teams, resp := Client.GetTeamsUnreadForUser(user.ID, "")
	CheckNoError(t, resp)
	require.NotEqual(t, len(teams), 0, "should have results")

	teams, resp = Client.GetTeamsUnreadForUser(user.ID, th.BasicTeam.ID)
	CheckNoError(t, resp)
	require.Empty(t, teams, "should not have results")

	_, resp = Client.GetTeamsUnreadForUser("fail", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetTeamsUnreadForUser(model.NewID(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetTeamsUnreadForUser(user.ID, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestTeamExists(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	public_member_team := th.BasicTeam
	err := th.App.UpdateTeamPrivacy(public_member_team.ID, model.TeamOpen, true)
	require.Nil(t, err)

	public_not_member_team := th.CreateTeamWithClient(th.SystemAdminClient)
	err = th.App.UpdateTeamPrivacy(public_not_member_team.ID, model.TeamOpen, true)
	require.Nil(t, err)

	private_member_team := th.CreateTeamWithClient(th.SystemAdminClient)
	th.LinkUserToTeam(th.BasicUser, private_member_team)
	err = th.App.UpdateTeamPrivacy(private_member_team.ID, model.TeamInvite, false)
	require.Nil(t, err)

	private_not_member_team := th.CreateTeamWithClient(th.SystemAdminClient)
	err = th.App.UpdateTeamPrivacy(private_not_member_team.ID, model.TeamInvite, false)
	require.Nil(t, err)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PermissionListPublicTeams.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionListPrivateTeams.ID, model.SystemUserRoleID)

	t.Run("Logged user with permissions and valid public team", func(t *testing.T) {
		th.LoginBasic()
		exists, resp := Client.TeamExists(public_not_member_team.Name, "")
		CheckNoError(t, resp)
		assert.True(t, exists, "team should exist")
	})

	t.Run("Logged user with permissions and valid private team", func(t *testing.T) {
		th.LoginBasic()
		exists, resp := Client.TeamExists(private_not_member_team.Name, "")
		CheckNoError(t, resp)
		assert.True(t, exists, "team should exist")
	})

	t.Run("Logged user and invalid team", func(t *testing.T) {
		th.LoginBasic()
		exists, resp := Client.TeamExists("testingteam", "")
		CheckNoError(t, resp)
		assert.False(t, exists, "team should not exist")
	})

	t.Run("Logged out user", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.TeamExists(public_not_member_team.Name, "")
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("Logged without LIST_PUBLIC_TEAMS permissions and member public team", func(t *testing.T) {
		th.LoginBasic()
		th.RemovePermissionFromRole(model.PermissionListPublicTeams.ID, model.SystemUserRoleID)

		exists, resp := Client.TeamExists(public_member_team.Name, "")
		CheckNoError(t, resp)
		assert.True(t, exists, "team should be visible")
	})

	t.Run("Logged without LIST_PUBLIC_TEAMS permissions and not member public team", func(t *testing.T) {
		th.LoginBasic()
		th.RemovePermissionFromRole(model.PermissionListPublicTeams.ID, model.SystemUserRoleID)

		exists, resp := Client.TeamExists(public_not_member_team.Name, "")
		CheckNoError(t, resp)
		assert.False(t, exists, "team should not be visible")
	})

	t.Run("Logged without LIST_PRIVATE_TEAMS permissions and member private team", func(t *testing.T) {
		th.LoginBasic()
		th.RemovePermissionFromRole(model.PermissionListPrivateTeams.ID, model.SystemUserRoleID)

		exists, resp := Client.TeamExists(private_member_team.Name, "")
		CheckNoError(t, resp)
		assert.True(t, exists, "team should be visible")
	})

	t.Run("Logged without LIST_PRIVATE_TEAMS permissions and not member private team", func(t *testing.T) {
		th.LoginBasic()
		th.RemovePermissionFromRole(model.PermissionListPrivateTeams.ID, model.SystemUserRoleID)

		exists, resp := Client.TeamExists(private_not_member_team.Name, "")
		CheckNoError(t, resp)
		assert.False(t, exists, "team should not be visible")
	})
}

func TestImportTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.TestForAllClients(t, func(T *testing.T, c *model.Client4) {
		data, err := testutils.ReadTestFile("Fake_Team_Import.zip")

		require.False(t, err != nil && len(data) == 0, "Error while reading the test file.")
		_, resp := th.SystemAdminClient.ImportTeam(data, binary.Size(data), "XYZ", "Fake_Team_Import.zip", th.BasicTeam.ID)
		CheckBadRequestStatus(t, resp)

		_, resp = th.SystemAdminClient.ImportTeam(data, binary.Size(data), "", "Fake_Team_Import.zip", th.BasicTeam.ID)
		CheckBadRequestStatus(t, resp)
	}, "Import from unknown and source")

	t.Run("ImportTeam", func(t *testing.T) {
		var data []byte
		var err error
		data, err = testutils.ReadTestFile("Fake_Team_Import.zip")

		require.False(t, err != nil && len(data) == 0, "Error while reading the test file.")

		// Import the channels/users/posts
		fileResp, resp := th.SystemAdminClient.ImportTeam(data, binary.Size(data), "slack", "Fake_Team_Import.zip", th.BasicTeam.ID)
		CheckNoError(t, resp)

		fileData, err := base64.StdEncoding.DecodeString(fileResp["results"])
		require.NoError(t, err, "failed to decode base64 results data")

		fileReturned := fmt.Sprintf("%s", fileData)
		require.Truef(t, strings.Contains(fileReturned, "darth.vader@stardeath.com"), "failed to report the user was imported, fileReturned: %s", fileReturned)

		// Checking the imported users
		importedUser, resp := th.SystemAdminClient.GetUserByUsername("bot_test", "")
		CheckNoError(t, resp)
		require.Equal(t, importedUser.Username, "bot_test", "username should match with the imported user")

		importedUser, resp = th.SystemAdminClient.GetUserByUsername("lordvader", "")
		CheckNoError(t, resp)
		require.Equal(t, importedUser.Username, "lordvader", "username should match with the imported user")

		// Checking the imported Channels
		importedChannel, resp := th.SystemAdminClient.GetChannelByName("testchannel", th.BasicTeam.ID, "")
		CheckNoError(t, resp)
		require.Equal(t, importedChannel.Name, "testchannel", "names did not match expected: testchannel")

		importedChannel, resp = th.SystemAdminClient.GetChannelByName("general", th.BasicTeam.ID, "")
		CheckNoError(t, resp)
		require.Equal(t, importedChannel.Name, "general", "names did not match expected: general")

		posts, resp := th.SystemAdminClient.GetPostsForChannel(importedChannel.ID, 0, 60, "", false)
		CheckNoError(t, resp)
		require.Equal(t, posts.Posts[posts.Order[3]].Message, "This is a test post to test the import process", "missing posts in the import process")
	})

	t.Run("Cloud Forbidden", func(t *testing.T) {
		var data []byte
		var err error
		data, err = testutils.ReadTestFile("Fake_Team_Import.zip")

		require.False(t, err != nil && len(data) == 0, "Error while reading the test file.")
		th.App.Srv().SetLicense(model.NewTestLicense("cloud"))

		// Import the channels/users/posts
		_, resp := th.SystemAdminClient.ImportTeam(data, binary.Size(data), "slack", "Fake_Team_Import.zip", th.BasicTeam.ID)
		CheckForbiddenStatus(t, resp)
		th.App.Srv().SetLicense(nil)
	})

	t.Run("MissingFile", func(t *testing.T) {
		_, resp := th.SystemAdminClient.ImportTeam(nil, 4343, "slack", "Fake_Team_Import.zip", th.BasicTeam.ID)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("WrongPermission", func(t *testing.T) {
		var data []byte
		var err error
		data, err = testutils.ReadTestFile("Fake_Team_Import.zip")
		require.False(t, err != nil && len(data) == 0, "Error while reading the test file.")

		// Import the channels/users/posts
		_, resp := th.Client.ImportTeam(data, binary.Size(data), "slack", "Fake_Team_Import.zip", th.BasicTeam.ID)
		CheckForbiddenStatus(t, resp)
	})
}

func TestInviteUsersToTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user1 := th.GenerateTestEmail()
	user2 := th.GenerateTestEmail()

	emailList := []string{user1, user2}

	//Delete all the messages before check the sample email
	mail.DeleteMailBox(user1)
	mail.DeleteMailBox(user2)

	enableEmailInvitations := *th.App.Config().ServiceSettings.EnableEmailInvitations
	restrictCreationToDomains := th.App.Config().TeamSettings.RestrictCreationToDomains
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableEmailInvitations = &enableEmailInvitations })
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.RestrictCreationToDomains = restrictCreationToDomains })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableEmailInvitations = false })
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.InviteUsersToTeam(th.BasicTeam.ID, emailList)
		require.NotNil(t, resp.Error, "Should be disabled")
	})

	checkEmail := func(t *testing.T, expectedSubject string) {
		//Check if the email was sent to the right email address
		for _, email := range emailList {
			var resultsMailbox mail.JSONMessageHeaderInbucket
			err := mail.RetryInbucket(5, func() error {
				var err error
				resultsMailbox, err = mail.GetMailBox(email)
				return err
			})
			if err != nil {
				t.Log(err)
				t.Log("No email was received, maybe due load on the server. Disabling this verification")
			}
			if err == nil && len(resultsMailbox) > 0 {
				require.True(t, strings.ContainsAny(resultsMailbox[len(resultsMailbox)-1].To[0], email), "Wrong To recipient")
				resultsEmail, err := mail.GetMessageFromMailbox(email, resultsMailbox[len(resultsMailbox)-1].ID)
				if err == nil {
					require.Equalf(t, resultsEmail.Subject, expectedSubject, "Wrong Subject, actual: %s, expected: %s", resultsEmail.Subject, expectedSubject)
				}
			}
		}
	}

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableEmailInvitations = true })
	okMsg, resp := th.SystemAdminClient.InviteUsersToTeam(th.BasicTeam.ID, emailList)
	CheckNoError(t, resp)
	require.True(t, okMsg, "should return true")
	nameFormat := *th.App.Config().TeamSettings.TeammateNameDisplay
	expectedSubject := i18n.T("api.templates.invite_subject",
		map[string]interface{}{"SenderName": th.SystemAdminUser.GetDisplayName(nameFormat),
			"TeamDisplayName": th.BasicTeam.DisplayName,
			"SiteName":        th.App.ClientConfig()["SiteName"]})
	checkEmail(t, expectedSubject)

	mail.DeleteMailBox(user1)
	mail.DeleteMailBox(user2)
	okMsg, resp = th.LocalClient.InviteUsersToTeam(th.BasicTeam.ID, emailList)
	CheckNoError(t, resp)
	require.True(t, okMsg, "should return true")
	expectedSubject = i18n.T("api.templates.invite_subject",
		map[string]interface{}{"SenderName": "Administrator",
			"TeamDisplayName": th.BasicTeam.DisplayName,
			"SiteName":        th.App.ClientConfig()["SiteName"]})
	checkEmail(t, expectedSubject)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.RestrictCreationToDomains = "@global.com,@common.com" })

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		okMsg, resp := client.InviteUsersToTeam(th.BasicTeam.ID, emailList)
		require.False(t, okMsg, "should return false")
		require.NotNil(t, resp.Error, "Adding users with non-restricted domains was allowed")

		invitesWithErrors, resp := client.InviteUsersToTeamGracefully(th.BasicTeam.ID, emailList)
		CheckNoError(t, resp)
		require.Len(t, invitesWithErrors, 2)
		require.NotNil(t, invitesWithErrors[0].Error)
		require.NotNil(t, invitesWithErrors[1].Error)
	}, "restricted domains")

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		th.BasicTeam.AllowedDomains = "invalid.com,common.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.NotNil(t, err, "Should not update the team")

		th.BasicTeam.AllowedDomains = "common.com"
		_, err = th.App.UpdateTeam(th.BasicTeam)
		require.Nilf(t, err, "%v, Should update the team", err)

		okMsg, resp := client.InviteUsersToTeam(th.BasicTeam.ID, []string{"test@global.com"})
		require.False(t, okMsg, "should return false")
		require.NotNilf(t, resp.Error, "%v, Per team restriction should take precedence over the globally allowed domains", err)

		okMsg, resp = client.InviteUsersToTeam(th.BasicTeam.ID, []string{"test@common.com"})
		require.True(t, okMsg, "should return true")
		require.Nilf(t, resp.Error, "%v, Failed to invite user which was common between team and global domain restriction", err)

		okMsg, resp = client.InviteUsersToTeam(th.BasicTeam.ID, []string{"test@invalid.com"})
		require.False(t, okMsg, "should return false")
		require.NotNilf(t, resp.Error, "%v, Should not invite user", err)

		invitesWithErrors, resp := client.InviteUsersToTeamGracefully(th.BasicTeam.ID, []string{"test@invalid.com", "test@common.com"})
		CheckNoError(t, resp)
		require.Len(t, invitesWithErrors, 2)
		require.NotNil(t, invitesWithErrors[0].Error)
		require.Nil(t, invitesWithErrors[1].Error)
	}, "override restricted domains")

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		th.BasicTeam.AllowedDomains = "common.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nilf(t, err, "%v, Should update the team", err)

		emailList := make([]string, 22)
		for i := 0; i < 22; i++ {
			emailList[i] = "test-" + strconv.Itoa(i) + "@common.com"
		}
		okMsg, resp := client.InviteUsersToTeam(th.BasicTeam.ID, emailList)
		require.False(t, okMsg, "should return false")
		CheckRequestEntityTooLargeStatus(t, resp)
		CheckErrorMessage(t, resp, "app.email.rate_limit_exceeded.app_error")

		_, resp = client.InviteUsersToTeamGracefully(th.BasicTeam.ID, emailList)
		CheckRequestEntityTooLargeStatus(t, resp)
		CheckErrorMessage(t, resp, "app.email.rate_limit_exceeded.app_error")
	}, "rate limits")
}

func TestInviteGuestsToTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	guest1 := th.GenerateTestEmail()
	guest2 := th.GenerateTestEmail()

	emailList := []string{guest1, guest2}

	//Delete all the messages before check the sample email
	mail.DeleteMailBox(guest1)
	mail.DeleteMailBox(guest2)

	enableEmailInvitations := *th.App.Config().ServiceSettings.EnableEmailInvitations
	restrictCreationToDomains := th.App.Config().TeamSettings.RestrictCreationToDomains
	guestRestrictCreationToDomains := th.App.Config().GuestAccountsSettings.RestrictCreationToDomains
	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableEmailInvitations = &enableEmailInvitations })
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.RestrictCreationToDomains = restrictCreationToDomains })
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.GuestAccountsSettings.RestrictCreationToDomains = guestRestrictCreationToDomains
		})
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.GuestAccountsSettings.Enable = &enableGuestAccounts })
	}()

	th.App.Srv().SetLicense(model.NewTestLicense(""))

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = false })
	_, resp := th.SystemAdminClient.InviteGuestsToTeam(th.BasicTeam.ID, emailList, []string{th.BasicChannel.ID}, "test-message")
	assert.NotNil(t, resp.Error, "Should be disabled")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableEmailInvitations = false })
	_, resp = th.SystemAdminClient.InviteGuestsToTeam(th.BasicTeam.ID, emailList, []string{th.BasicChannel.ID}, "test-message")
	require.NotNil(t, resp.Error, "Should be disabled")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableEmailInvitations = true })

	th.App.Srv().SetLicense(nil)

	_, resp = th.SystemAdminClient.InviteGuestsToTeam(th.BasicTeam.ID, emailList, []string{th.BasicChannel.ID}, "test-message")
	require.NotNil(t, resp.Error, "Should be disabled")

	th.App.Srv().SetLicense(model.NewTestLicense(""))
	defer th.App.Srv().SetLicense(nil)

	okMsg, resp := th.SystemAdminClient.InviteGuestsToTeam(th.BasicTeam.ID, emailList, []string{th.BasicChannel.ID}, "test-message")
	CheckNoError(t, resp)
	require.True(t, okMsg, "should return true")

	t.Run("invalid data in request body", func(t *testing.T) {
		res, err := th.SystemAdminClient.DoApiPost(th.SystemAdminClient.GetTeamRoute(th.BasicTeam.ID)+"/invite-guests/email", "bad data")
		require.NotNil(t, err)
		require.Equal(t, "api.team.invite_guests_to_channels.invalid_body.app_error", err.ID)
		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	nameFormat := *th.App.Config().TeamSettings.TeammateNameDisplay
	expectedSubject := i18n.T("api.templates.invite_guest_subject",
		map[string]interface{}{"SenderName": th.SystemAdminUser.GetDisplayName(nameFormat),
			"TeamDisplayName": th.BasicTeam.DisplayName,
			"SiteName":        th.App.ClientConfig()["SiteName"]})

	//Check if the email was send to the right email address
	for _, email := range emailList {
		var resultsMailbox mail.JSONMessageHeaderInbucket
		err := mail.RetryInbucket(5, func() error {
			var err error
			resultsMailbox, err = mail.GetMailBox(email)
			return err
		})
		if err != nil {
			t.Log(err)
			t.Log("No email was received, maybe due load on the server. Disabling this verification")
		}
		if err == nil && len(resultsMailbox) > 0 {
			require.True(t, strings.ContainsAny(resultsMailbox[len(resultsMailbox)-1].To[0], email), "Wrong To recipient")
			resultsEmail, err := mail.GetMessageFromMailbox(email, resultsMailbox[len(resultsMailbox)-1].ID)
			if err == nil {
				require.Equalf(t, resultsEmail.Subject, expectedSubject, "Wrong Subject, actual: %s, expected: %s", resultsEmail.Subject, expectedSubject)
			}
		}
	}

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.RestrictCreationToDomains = "@global.com,@common.com" })

	t.Run("team domain restrictions should not affect inviting guests", func(t *testing.T) {
		err := th.App.InviteGuestsToChannels(th.BasicTeam.ID, &model.GuestsInvite{Emails: emailList, Channels: []string{th.BasicChannel.ID}, Message: "test message"}, th.BasicUser.ID)
		require.Nil(t, err, "guest user invites should not be affected by team restrictions")
	})

	t.Run("guest restrictions should affect guest users", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.RestrictCreationToDomains = "@guest.com" })

		err := th.App.InviteGuestsToChannels(th.BasicTeam.ID, &model.GuestsInvite{Emails: []string{"guest1@invalid.com"}, Channels: []string{th.BasicChannel.ID}, Message: "test message"}, th.BasicUser.ID)
		require.NotNil(t, err, "guest user invites should be affected by the guest domain restrictions")

		res, err := th.App.InviteGuestsToChannelsGracefully(th.BasicTeam.ID, &model.GuestsInvite{Emails: []string{"guest1@invalid.com", "guest1@guest.com"}, Channels: []string{th.BasicChannel.ID}, Message: "test message"}, th.BasicUser.ID)
		require.Nil(t, err)
		require.Len(t, res, 2)
		require.NotNil(t, res[0].Error)
		require.Nil(t, res[1].Error)

		err = th.App.InviteGuestsToChannels(th.BasicTeam.ID, &model.GuestsInvite{Emails: []string{"guest1@guest.com"}, Channels: []string{th.BasicChannel.ID}, Message: "test message"}, th.BasicUser.ID)
		require.Nil(t, err, "whitelisted guest user email should be allowed by the guest domain restrictions")
	})

	t.Run("guest restrictions should not affect inviting new team members", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.RestrictCreationToDomains = "@guest.com" })

		err := th.App.InviteNewUsersToTeam([]string{"user@global.com"}, th.BasicTeam.ID, th.BasicUser.ID)
		require.Nil(t, err, "non guest user invites should not be affected by the guest domain restrictions")
	})

	t.Run("rate limit", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.RestrictCreationToDomains = "@guest.com" })

		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nilf(t, err, "%v, Should update the team", err)

		emailList := make([]string, 22)
		for i := 0; i < 22; i++ {
			emailList[i] = "test-" + strconv.Itoa(i) + "@guest.com"
		}
		invite := &model.GuestsInvite{
			Emails:   emailList,
			Channels: []string{th.BasicChannel.ID},
			Message:  "test message",
		}
		err = th.App.InviteGuestsToChannels(th.BasicTeam.ID, invite, th.BasicUser.ID)
		require.NotNil(t, err)
		assert.Equal(t, "app.email.rate_limit_exceeded.app_error", err.ID)
		assert.Equal(t, http.StatusRequestEntityTooLarge, err.StatusCode)

		_, err = th.App.InviteGuestsToChannelsGracefully(th.BasicTeam.ID, invite, th.BasicUser.ID)
		require.NotNil(t, err)
		assert.Equal(t, "app.email.rate_limit_exceeded.app_error", err.ID)
		assert.Equal(t, http.StatusRequestEntityTooLarge, err.StatusCode)
	})
}

func TestGetTeamInviteInfo(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	team, resp := Client.GetTeamInviteInfo(team.InviteID)
	CheckNoError(t, resp)

	require.NotEmpty(t, team.DisplayName, "should not be empty")

	require.Empty(t, team.Email, "should be empty")

	team.InviteID = "12345678901234567890123456789012"
	team, resp = th.SystemAdminClient.UpdateTeam(team)
	CheckNoError(t, resp)

	_, resp = Client.GetTeamInviteInfo(team.InviteID)
	CheckNoError(t, resp)

	_, resp = Client.GetTeamInviteInfo("junk")
	CheckNotFoundStatus(t, resp)
}

func TestSetTeamIcon(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err, err)

	th.LoginTeamAdmin()

	ok, resp := Client.SetTeamIcon(team.ID, data)
	require.True(t, ok, resp.Error)

	CheckNoError(t, resp)

	ok, resp = Client.SetTeamIcon(model.NewID(), data)
	require.False(t, ok, "Should return false, set team icon not allowed")

	CheckForbiddenStatus(t, resp)

	th.LoginBasic()

	_, resp = Client.SetTeamIcon(team.ID, data)
	if resp.StatusCode == http.StatusForbidden {
		CheckForbiddenStatus(t, resp)
	} else if resp.StatusCode == http.StatusUnauthorized {
		CheckUnauthorizedStatus(t, resp)
	} else {
		require.Fail(t, "Should have failed either forbidden or unauthorized")
	}

	Client.Logout()

	_, resp = Client.SetTeamIcon(team.ID, data)
	if resp.StatusCode == http.StatusForbidden {
		CheckForbiddenStatus(t, resp)
	} else if resp.StatusCode == http.StatusUnauthorized {
		CheckUnauthorizedStatus(t, resp)
	} else {
		require.Fail(t, "Should have failed either forbidden or unauthorized")
	}

	teamBefore, appErr := th.App.GetTeam(team.ID)
	require.Nil(t, appErr)

	_, resp = th.SystemAdminClient.SetTeamIcon(team.ID, data)
	CheckNoError(t, resp)

	teamAfter, appErr := th.App.GetTeam(team.ID)
	require.Nil(t, appErr)
	assert.True(t, teamBefore.LastTeamIconUpdate < teamAfter.LastTeamIconUpdate, "LastTeamIconUpdate should have been updated for team")

	info := &model.FileInfo{Path: "teams/" + team.ID + "/teamIcon.png"}
	err = th.cleanupTestFile(info)
	require.NoError(t, err)
}

func TestGetTeamIcon(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	// should always fail because no initial image and no auto creation
	_, resp := Client.GetTeamIcon(team.ID, "")
	CheckNotFoundStatus(t, resp)

	Client.Logout()

	_, resp = Client.GetTeamIcon(team.ID, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestRemoveTeamIcon(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	th.LoginTeamAdmin()
	data, _ := testutils.ReadTestFile("test.png")
	Client.SetTeamIcon(team.ID, data)

	_, resp := Client.RemoveTeamIcon(team.ID)
	CheckNoError(t, resp)
	teamAfter, _ := th.App.GetTeam(team.ID)
	require.Equal(t, teamAfter.LastTeamIconUpdate, int64(0), "should update LastTeamIconUpdate to 0")

	Client.SetTeamIcon(team.ID, data)

	_, resp = th.SystemAdminClient.RemoveTeamIcon(team.ID)
	CheckNoError(t, resp)
	teamAfter, _ = th.App.GetTeam(team.ID)
	require.Equal(t, teamAfter.LastTeamIconUpdate, int64(0), "should update LastTeamIconUpdate to 0")

	Client.SetTeamIcon(team.ID, data)
	Client.Logout()

	_, resp = Client.RemoveTeamIcon(team.ID)
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic()
	_, resp = Client.RemoveTeamIcon(team.ID)
	CheckForbiddenStatus(t, resp)
}

func TestUpdateTeamScheme(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense(""))

	th.App.SetPhase2PermissionsMigrationStatus(true)

	team := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team, _ = th.SystemAdminClient.CreateTeam(team)

	teamScheme := &model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewID(),
		Description: "Some description",
		Scope:       model.SchemeScopeTeam,
	}
	teamScheme, _ = th.SystemAdminClient.CreateScheme(teamScheme)
	channelScheme := &model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewID(),
		Description: "Some description",
		Scope:       model.SchemeScopeChannel,
	}
	channelScheme, _ = th.SystemAdminClient.CreateScheme(channelScheme)

	// Test the setup/base case.
	_, resp := th.SystemAdminClient.UpdateTeamScheme(team.ID, teamScheme.ID)
	CheckNoError(t, resp)

	// Test the return to default scheme
	_, resp = th.SystemAdminClient.UpdateTeamScheme(team.ID, "")
	CheckNoError(t, resp)

	// Test various invalid team and scheme id combinations.
	_, resp = th.SystemAdminClient.UpdateTeamScheme(team.ID, "x")
	CheckBadRequestStatus(t, resp)
	_, resp = th.SystemAdminClient.UpdateTeamScheme("x", teamScheme.ID)
	CheckBadRequestStatus(t, resp)
	_, resp = th.SystemAdminClient.UpdateTeamScheme("x", "x")
	CheckBadRequestStatus(t, resp)

	// Test that permissions are required.
	_, resp = th.Client.UpdateTeamScheme(team.ID, teamScheme.ID)
	CheckForbiddenStatus(t, resp)

	// Test that a license is required.
	th.App.Srv().SetLicense(nil)
	_, resp = th.SystemAdminClient.UpdateTeamScheme(team.ID, teamScheme.ID)
	CheckNotImplementedStatus(t, resp)
	th.App.Srv().SetLicense(model.NewTestLicense(""))

	// Test an invalid scheme scope.
	_, resp = th.SystemAdminClient.UpdateTeamScheme(team.ID, channelScheme.ID)
	CheckBadRequestStatus(t, resp)

	// Test that an unauthenticated user gets rejected.
	th.SystemAdminClient.Logout()
	_, resp = th.SystemAdminClient.UpdateTeamScheme(team.ID, teamScheme.ID)
	CheckUnauthorizedStatus(t, resp)
}

func TestTeamMembersMinusGroupMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user1 := th.BasicUser
	user2 := th.BasicUser2

	team := th.CreateTeam()
	team.GroupConstrained = model.NewBool(true)
	team, err := th.App.UpdateTeam(team)
	require.Nil(t, err)

	_, err = th.App.AddTeamMember(th.Context, team.ID, user1.ID)
	require.Nil(t, err)
	_, err = th.App.AddTeamMember(th.Context, team.ID, user2.ID)
	require.Nil(t, err)

	group1 := th.CreateGroup()
	group2 := th.CreateGroup()

	_, err = th.App.UpsertGroupMember(group1.ID, user1.ID)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupMember(group2.ID, user2.ID)
	require.Nil(t, err)

	// No permissions
	_, _, res := th.Client.TeamMembersMinusGroupMembers(team.ID, []string{group1.ID, group2.ID}, 0, 100, "")
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
			uwg, count, res := th.SystemAdminClient.TeamMembersMinusGroupMembers(team.ID, tc.groupIDs, tc.page, tc.perPage, "")
			require.Nil(t, res.Error)
			require.Len(t, uwg, tc.length)
			require.Equal(t, tc.count, int(count))
			if tc.otherAssertions != nil {
				tc.otherAssertions(uwg)
			}
		})
	}
}

func TestInvalidateAllEmailInvites(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("Forbidden when request performed by system user", func(t *testing.T) {
		ok, res := th.Client.InvalidateEmailInvites()
		require.Equal(t, false, ok)
		CheckForbiddenStatus(t, res)
	})

	t.Run("OK when request performed by system user with requisite system permission", func(t *testing.T) {
		th.AddPermissionToRole(model.PermissionInvalidateEmailInvite.ID, model.SystemUserRoleID)
		defer th.RemovePermissionFromRole(model.PermissionInvalidateEmailInvite.ID, model.SystemUserRoleID)
		ok, res := th.Client.InvalidateEmailInvites()
		require.Equal(t, true, ok)
		CheckOKStatus(t, res)
	})

	t.Run("OK when request performed by system admin", func(t *testing.T) {
		ok, res := th.SystemAdminClient.InvalidateEmailInvites()
		require.Equal(t, true, ok)
		CheckOKStatus(t, res)
	})
}
