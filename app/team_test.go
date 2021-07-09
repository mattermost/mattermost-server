// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestCreateTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	team := &model.Team{
		DisplayName: "dn_" + id,
		Name:        "name" + id,
		Email:       "success+" + id + "@simulator.amazonses.com",
		Type:        model.TeamOpen,
	}

	_, err := th.App.CreateTeam(th.Context, team)
	require.Nil(t, err, "Should create a new team")

	_, err = th.App.CreateTeam(th.Context, th.BasicTeam)
	require.NotNil(t, err, "Should not create a new team - team already exist")
}

func TestCreateTeamWithUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	team := &model.Team{
		DisplayName: "dn_" + id,
		Name:        "name" + id,
		Email:       "success+" + id + "@simulator.amazonses.com",
		Type:        model.TeamOpen,
	}

	_, err := th.App.CreateTeamWithUser(th.Context, team, th.BasicUser.ID)
	require.Nil(t, err, "Should create a new team with existing user")

	_, err = th.App.CreateTeamWithUser(th.Context, team, model.NewID())
	require.NotNil(t, err, "Should not create a new team - user does not exist")
}

func TestUpdateTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.BasicTeam.DisplayName = "Testing 123"

	updatedTeam, err := th.App.UpdateTeam(th.BasicTeam)
	require.Nil(t, err, "Should update the team")
	require.Equal(t, "Testing 123", updatedTeam.DisplayName, "Wrong Team DisplayName")
}

func TestAddUserToTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("add user", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.Nil(t, err, "Should add user to the team")
	})

	t.Run("allow user by domain", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "example.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")

		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.Nil(t, err, "Should have allowed whitelisted user")
	})

	t.Run("block user by domain but allow bot", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "example.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")

		user := model.User{Email: strings.ToLower(model.NewID()) + "test@invalid.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, err := th.App.CreateUser(th.Context, &user)
		require.Nil(t, err, "Error creating user: %s", err)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.NotNil(t, err, "Should not add restricted user")
		require.Equal(t, "JoinUserToTeam", err.Where, "Error should be JoinUserToTeam")

		user = model.User{Email: strings.ToLower(model.NewID()) + "test@invalid.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), AuthService: "notnil", AuthData: model.NewString("notnil")}
		ruser, err = th.App.CreateUser(th.Context, &user)
		require.Nil(t, err, "Error creating authservice user: %s", err)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.NotNil(t, err, "Should not add authservice user")
		require.Equal(t, "JoinUserToTeam", err.Where, "Error should be JoinUserToTeam")

		bot, err := th.App.CreateBot(th.Context, &model.Bot{
			Username:    "somebot",
			Description: "a bot",
			OwnerID:     th.BasicUser.ID,
		})
		require.Nil(t, err)

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, bot.UserID, "")
		assert.Nil(t, err, "should be able to add bot to domain restricted team")
	})

	t.Run("block user with subdomain", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "example.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")

		user := model.User{Email: strings.ToLower(model.NewID()) + "test@invalid.example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.NotNil(t, err, "Should not add restricted user")
		require.Equal(t, "JoinUserToTeam", err.Where, "Error should be JoinUserToTeam")
	})

	t.Run("allow users by multiple domains", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "foo.com, bar.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")

		user1 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@foo.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser1, _ := th.App.CreateUser(th.Context, &user1)

		user2 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@bar.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser2, _ := th.App.CreateUser(th.Context, &user2)

		user3 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@invalid.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser3, _ := th.App.CreateUser(th.Context, &user3)

		defer th.App.PermanentDeleteUser(th.Context, &user1)
		defer th.App.PermanentDeleteUser(th.Context, &user2)
		defer th.App.PermanentDeleteUser(th.Context, &user3)

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser1.ID, "")
		require.Nil(t, err, "Should have allowed whitelisted user1")

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser2.ID, "")
		require.Nil(t, err, "Should have allowed whitelisted user2")

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser3.ID, "")
		require.NotNil(t, err, "Should not have allowed restricted user3")
		require.Equal(t, "JoinUserToTeam", err.Where, "Error should be JoinUserToTeam")
	})

	t.Run("should set up initial sidebar categories when joining a team", func(t *testing.T) {
		user := th.CreateUser()
		team := th.CreateTeam()

		_, _, err := th.App.AddUserToTeam(th.Context, team.ID, user.ID, "")
		require.Nil(t, err)

		res, err := th.App.GetSidebarCategories(user.ID, team.ID)
		require.Nil(t, err)
		assert.Len(t, res.Categories, 3)
		assert.Equal(t, model.SidebarCategoryFavorites, res.Categories[0].Type)
		assert.Equal(t, model.SidebarCategoryChannels, res.Categories[1].Type)
		assert.Equal(t, model.SidebarCategoryDirectMessages, res.Categories[2].Type)
	})
}

func TestAddUserToTeamByToken(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
	ruser, _ := th.App.CreateUser(th.Context, &user)
	rguest := th.CreateGuest()

	t.Run("invalid token", func(t *testing.T) {
		_, _, err := th.App.AddUserToTeamByToken(th.Context, ruser.ID, "123")
		require.NotNil(t, err, "Should fail on unexisting token")
	})

	t.Run("invalid token type", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeVerifyEmail,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID}),
		)

		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)

		_, _, err := th.App.AddUserToTeamByToken(th.Context, ruser.ID, token.Token)
		require.NotNil(t, err, "Should fail on bad token type")
	})

	t.Run("expired token", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID}),
		)

		token.CreateAt = model.GetMillis() - InvitationExpiryTime - 1
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)

		_, _, err := th.App.AddUserToTeamByToken(th.Context, ruser.ID, token.Token)
		require.NotNil(t, err, "Should fail on expired token")
	})

	t.Run("invalid team id", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJSON(map[string]string{"teamId": model.NewID()}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)

		_, _, err := th.App.AddUserToTeamByToken(th.Context, ruser.ID, token.Token)
		require.NotNil(t, err, "Should fail on bad team id")
	})

	t.Run("invalid user id", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)

		_, _, err := th.App.AddUserToTeamByToken(th.Context, model.NewID(), token.Token)
		require.NotNil(t, err, "Should fail on bad user id")
	})

	t.Run("valid request", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		_, _, err := th.App.AddUserToTeamByToken(th.Context, ruser.ID, token.Token)
		require.Nil(t, err, "Should add user to the team")

		_, nErr := th.App.Srv().Store.Token().GetByToken(token.Token)
		require.Error(t, nErr, "The token must be deleted after be used")

		members, err := th.App.GetChannelMembersForUser(th.BasicTeam.ID, ruser.ID)
		require.Nil(t, err)
		assert.Len(t, *members, 2)
	})

	t.Run("invalid add a guest using a regular invite", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		_, _, err := th.App.AddUserToTeamByToken(th.Context, rguest.ID, token.Token)
		assert.NotNil(t, err)
	})

	t.Run("invalid add a regular user using a guest invite", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID, "channels": th.BasicChannel.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		_, _, err := th.App.AddUserToTeamByToken(th.Context, ruser.ID, token.Token)
		assert.NotNil(t, err)
	})

	t.Run("invalid add a guest user with a non-granted email domain", func(t *testing.T) {
		restrictedDomain := *th.App.Config().GuestAccountsSettings.RestrictCreationToDomains
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.GuestAccountsSettings.RestrictCreationToDomains = &restrictedDomain })
		}()
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.RestrictCreationToDomains = "restricted.com" })
		token := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID, "channels": th.BasicChannel.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		_, _, err := th.App.AddUserToTeamByToken(th.Context, rguest.ID, token.Token)
		require.NotNil(t, err)
		assert.Equal(t, "api.team.join_user_to_team.allowed_domains.app_error", err.ID)
	})

	t.Run("add a guest user with a granted email domain", func(t *testing.T) {
		restrictedDomain := *th.App.Config().GuestAccountsSettings.RestrictCreationToDomains
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.GuestAccountsSettings.RestrictCreationToDomains = &restrictedDomain })
		}()
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.RestrictCreationToDomains = "restricted.com" })
		token := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID, "channels": th.BasicChannel.ID}),
		)
		guestEmail := rguest.Email
		rguest.Email = "test@restricted.com"
		_, err := th.App.Srv().Store.User().Update(rguest, false)
		th.App.InvalidateCacheForUser(rguest.ID)
		require.NoError(t, err)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		_, _, appErr := th.App.AddUserToTeamByToken(th.Context, rguest.ID, token.Token)
		require.Nil(t, appErr)
		rguest.Email = guestEmail
		_, err = th.App.Srv().Store.User().Update(rguest, false)
		require.NoError(t, err)
	})

	t.Run("add a guest user even though there are team and system domain restrictions", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "restricted-team.com"
		_, err := th.Server.Store.Team().Update(th.BasicTeam)
		require.NoError(t, err)
		restrictedDomain := *th.App.Config().TeamSettings.RestrictCreationToDomains
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.RestrictCreationToDomains = &restrictedDomain })
		}()
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.RestrictCreationToDomains = "restricted.com" })
		token := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID, "channels": th.BasicChannel.ID}),
		)
		_, err = th.App.Srv().Store.User().Update(rguest, false)
		require.NoError(t, err)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		_, _, appErr := th.App.AddUserToTeamByToken(th.Context, rguest.ID, token.Token)
		require.Nil(t, appErr)
		th.BasicTeam.AllowedDomains = ""
		_, err = th.Server.Store.Team().Update(th.BasicTeam)
		require.NoError(t, err)
	})

	t.Run("valid request from guest invite", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID, "channels": th.BasicChannel.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))

		_, _, err := th.App.AddUserToTeamByToken(th.Context, rguest.ID, token.Token)
		require.Nil(t, err, "Should add user to the team")

		_, nErr := th.App.Srv().Store.Token().GetByToken(token.Token)
		require.Error(t, nErr, "The token must be deleted after be used")

		members, err := th.App.GetChannelMembersForUser(th.BasicTeam.ID, rguest.ID)
		require.Nil(t, err)
		require.Len(t, *members, 1)
		assert.Equal(t, (*members)[0].ChannelID, th.BasicChannel.ID)
	})

	t.Run("group-constrained team", func(t *testing.T) {
		th.BasicTeam.GroupConstrained = model.NewBool(true)
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")

		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))

		_, _, err = th.App.AddUserToTeamByToken(th.Context, ruser.ID, token.Token)
		require.NotNil(t, err, "Should return an error when trying to join a group-constrained team.")
		require.Equal(t, "app.team.invite_token.group_constrained.error", err.ID)

		th.BasicTeam.GroupConstrained = model.NewBool(false)
		_, err = th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")
	})

	t.Run("block user", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "example.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")

		user := model.User{Email: strings.ToLower(model.NewID()) + "test@invalid.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJSON(map[string]string{"teamId": th.BasicTeam.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))

		_, _, err = th.App.AddUserToTeamByToken(th.Context, ruser.ID, token.Token)
		require.NotNil(t, err, "Should not add restricted user")
		require.Equal(t, "JoinUserToTeam", err.Where, "Error should be JoinUserToTeam")
	})

	t.Run("should set up initial sidebar categories when joining a team by token", func(t *testing.T) {
		user := th.CreateUser()
		team := th.CreateTeam()

		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJSON(map[string]string{"teamId": team.ID}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))

		_, _, err := th.App.AddUserToTeamByToken(th.Context, user.ID, token.Token)
		require.Nil(t, err)

		res, err := th.App.GetSidebarCategories(user.ID, team.ID)
		require.Nil(t, err)
		assert.Len(t, res.Categories, 3)
		assert.Equal(t, model.SidebarCategoryFavorites, res.Categories[0].Type)
		assert.Equal(t, model.SidebarCategoryChannels, res.Categories[1].Type)
		assert.Equal(t, model.SidebarCategoryDirectMessages, res.Categories[2].Type)
	})
}

func TestAddUserToTeamByTeamID(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("add user", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)

		err := th.App.AddUserToTeamByTeamID(th.Context, th.BasicTeam.ID, ruser)
		require.Nil(t, err, "Should add user to the team")
	})

	t.Run("block user", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "example.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")

		user := model.User{Email: strings.ToLower(model.NewID()) + "test@invalid.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		err = th.App.AddUserToTeamByTeamID(th.Context, th.BasicTeam.ID, ruser)
		require.NotNil(t, err, "Should not add restricted user")
		require.Equal(t, "JoinUserToTeam", err.Where, "Error should be JoinUserToTeam")
	})

}

func TestPermanentDeleteTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team, err := th.App.CreateTeam(th.Context, &model.Team{
		DisplayName: "deletion-test",
		Name:        "deletion-test",
		Email:       "foo@foo.com",
		Type:        model.TeamOpen,
	})
	require.Nil(t, err, "Should create a team")

	defer func() {
		th.App.PermanentDeleteTeam(team)
	}()

	command, err := th.App.CreateCommand(&model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    team.ID,
		Trigger:   "foo",
		URL:       "http://foo",
		Method:    model.CommandMethodPost,
	})
	require.Nil(t, err, "Should create a command")
	defer th.App.DeleteCommand(command.ID)

	command, err = th.App.GetCommand(command.ID)
	require.NotNil(t, command, "command should not be nil")
	require.Nil(t, err, "unable to get new command")

	err = th.App.PermanentDeleteTeam(team)
	require.Nil(t, err)

	command, err = th.App.GetCommand(command.ID)
	require.Nil(t, command, "command wasn't deleted")
	require.NotNil(t, err, "should not return an error")

	// Test deleting a team with no channels.
	team = th.CreateTeam()
	defer func() {
		th.App.PermanentDeleteTeam(team)
	}()

	channels, err := th.App.GetPublicChannelsForTeam(team.ID, 0, 1000)
	require.Nil(t, err)

	for _, channel := range *channels {
		err2 := th.App.PermanentDeleteChannel(channel)
		require.Nil(t, err2)
	}

	err = th.App.PermanentDeleteTeam(team)
	require.Nil(t, err)
}

func TestSanitizeTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	team := &model.Team{
		ID:             model.NewID(),
		Email:          th.MakeEmail(),
		InviteID:       model.NewID(),
		AllowedDomains: "example.com",
	}

	copyTeam := func() *model.Team {
		copy := &model.Team{}
		*copy = *team
		return copy
	}

	t.Run("not a user of the team", func(t *testing.T) {
		userID := model.NewID()
		session := model.Session{
			Roles: model.SystemUserRoleID,
			TeamMembers: []*model.TeamMember{
				{
					UserID: userID,
					TeamID: model.NewID(),
					Roles:  model.TeamUserRoleID,
				},
			},
		}

		sanitized := th.App.SanitizeTeam(session, copyTeam())
		require.Empty(t, sanitized.Email, "should've sanitized team")
		require.Empty(t, sanitized.InviteID, "should've sanitized inviteid")
	})

	t.Run("user of the team", func(t *testing.T) {
		userID := model.NewID()
		session := model.Session{
			Roles: model.SystemUserRoleID,
			TeamMembers: []*model.TeamMember{
				{
					UserID: userID,
					TeamID: team.ID,
					Roles:  model.TeamUserRoleID,
				},
			},
		}

		sanitized := th.App.SanitizeTeam(session, copyTeam())
		require.Empty(t, sanitized.Email, "should've sanitized team")
		require.NotEmpty(t, sanitized.InviteID, "should have not sanitized inviteid")
	})

	t.Run("team admin", func(t *testing.T) {
		userID := model.NewID()
		session := model.Session{
			Roles: model.SystemUserRoleID,
			TeamMembers: []*model.TeamMember{
				{
					UserID: userID,
					TeamID: team.ID,
					Roles:  model.TeamUserRoleID + " " + model.TeamAdminRoleID,
				},
			},
		}

		sanitized := th.App.SanitizeTeam(session, copyTeam())
		require.NotEmpty(t, sanitized.Email, "shouldn't have sanitized team")
		require.NotEmpty(t, sanitized.InviteID, "shouldn't have sanitized inviteid")
	})

	t.Run("team admin of another team", func(t *testing.T) {
		userID := model.NewID()
		session := model.Session{
			Roles: model.SystemUserRoleID,
			TeamMembers: []*model.TeamMember{
				{
					UserID: userID,
					TeamID: model.NewID(),
					Roles:  model.TeamUserRoleID + " " + model.TeamAdminRoleID,
				},
			},
		}

		sanitized := th.App.SanitizeTeam(session, copyTeam())
		require.Empty(t, sanitized.Email, "should've sanitized team")
		require.Empty(t, sanitized.InviteID, "should've sanitized inviteid")
	})

	t.Run("system admin, not a user of team", func(t *testing.T) {
		userID := model.NewID()
		session := model.Session{
			Roles: model.SystemUserRoleID + " " + model.SystemAdminRoleID,
			TeamMembers: []*model.TeamMember{
				{
					UserID: userID,
					TeamID: model.NewID(),
					Roles:  model.TeamUserRoleID,
				},
			},
		}

		sanitized := th.App.SanitizeTeam(session, copyTeam())
		require.NotEmpty(t, sanitized.Email, "shouldn't have sanitized team")
		require.NotEmpty(t, sanitized.InviteID, "shouldn't have sanitized inviteid")
	})

	t.Run("system admin, user of team", func(t *testing.T) {
		userID := model.NewID()
		session := model.Session{
			Roles: model.SystemUserRoleID + " " + model.SystemAdminRoleID,
			TeamMembers: []*model.TeamMember{
				{
					UserID: userID,
					TeamID: team.ID,
					Roles:  model.TeamUserRoleID,
				},
			},
		}

		sanitized := th.App.SanitizeTeam(session, copyTeam())
		require.NotEmpty(t, sanitized.Email, "shouldn't have sanitized team")
		require.NotEmpty(t, sanitized.InviteID, "shouldn't have sanitized inviteid")
	})
}

func TestSanitizeTeams(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("not a system admin", func(t *testing.T) {
		teams := []*model.Team{
			{
				ID:             model.NewID(),
				Email:          th.MakeEmail(),
				AllowedDomains: "example.com",
			},
			{
				ID:             model.NewID(),
				Email:          th.MakeEmail(),
				AllowedDomains: "example.com",
			},
		}

		userID := model.NewID()
		session := model.Session{
			Roles: model.SystemUserRoleID,
			TeamMembers: []*model.TeamMember{
				{
					UserID: userID,
					TeamID: teams[0].ID,
					Roles:  model.TeamUserRoleID,
				},
				{
					UserID: userID,
					TeamID: teams[1].ID,
					Roles:  model.TeamUserRoleID + " " + model.TeamAdminRoleID,
				},
			},
		}

		sanitized := th.App.SanitizeTeams(session, teams)

		require.Empty(t, sanitized[0].Email, "should've sanitized first team")
		require.NotEmpty(t, sanitized[1].Email, "shouldn't have sanitized second team")
	})

	t.Run("system admin", func(t *testing.T) {
		teams := []*model.Team{
			{
				ID:             model.NewID(),
				Email:          th.MakeEmail(),
				AllowedDomains: "example.com",
			},
			{
				ID:             model.NewID(),
				Email:          th.MakeEmail(),
				AllowedDomains: "example.com",
			},
		}

		userID := model.NewID()
		session := model.Session{
			Roles: model.SystemUserRoleID + " " + model.SystemAdminRoleID,
			TeamMembers: []*model.TeamMember{
				{
					UserID: userID,
					TeamID: teams[0].ID,
					Roles:  model.TeamUserRoleID,
				},
			},
		}

		sanitized := th.App.SanitizeTeams(session, teams)
		assert.NotEmpty(t, sanitized[0].Email, "shouldn't have sanitized first team")
		assert.NotEmpty(t, sanitized[1].Email, "shouldn't have sanitized second team")
	})
}

func TestJoinUserToTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	id := model.NewID()
	team := &model.Team{
		DisplayName: "dn_" + id,
		Name:        "name" + id,
		Email:       "success+" + id + "@simulator.amazonses.com",
		Type:        model.TeamOpen,
	}

	_, err := th.App.CreateTeam(th.Context, team)
	require.Nil(t, err, "Should create a new team")

	maxUsersPerTeam := th.App.Config().TeamSettings.MaxUsersPerTeam
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.MaxUsersPerTeam = maxUsersPerTeam })
		th.App.PermanentDeleteTeam(team)
	}()
	one := 1
	th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.MaxUsersPerTeam = &one })

	t.Run("new join", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		var alreadyAdded bool
		_, alreadyAdded, err = th.App.joinUserToTeam(team, ruser)
		require.False(t, alreadyAdded, "Should return already added equal to false")
		require.Nil(t, err, "Should return no error")
	})

	t.Run("join when you are a member", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		th.App.joinUserToTeam(team, ruser)

		var alreadyAdded bool
		_, alreadyAdded, err = th.App.joinUserToTeam(team, ruser)
		require.True(t, alreadyAdded, "Should return already added")
		require.Nil(t, err, "Should return no error")
	})

	t.Run("re-join after leaving", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		th.App.joinUserToTeam(team, ruser)
		th.App.LeaveTeam(th.Context, team, ruser, ruser.ID)

		var alreadyAdded bool
		_, alreadyAdded, err = th.App.joinUserToTeam(team, ruser)
		require.False(t, alreadyAdded, "Should return already added equal to false")
		require.Nil(t, err, "Should return no error")
	})

	t.Run("new join with limit problem", func(t *testing.T) {
		user1 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser1, _ := th.App.CreateUser(th.Context, &user1)
		user2 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser2, _ := th.App.CreateUser(th.Context, &user2)

		defer th.App.PermanentDeleteUser(th.Context, &user1)
		defer th.App.PermanentDeleteUser(th.Context, &user2)
		th.App.joinUserToTeam(team, ruser1)

		_, _, err = th.App.joinUserToTeam(team, ruser2)
		require.NotNil(t, err, "Should fail")
	})

	t.Run("re-join alfter leaving with limit problem", func(t *testing.T) {
		user1 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser1, _ := th.App.CreateUser(th.Context, &user1)

		user2 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser2, _ := th.App.CreateUser(th.Context, &user2)

		defer th.App.PermanentDeleteUser(th.Context, &user1)
		defer th.App.PermanentDeleteUser(th.Context, &user2)

		th.App.joinUserToTeam(team, ruser1)
		th.App.LeaveTeam(th.Context, team, ruser1, ruser1.ID)
		th.App.joinUserToTeam(team, ruser2)

		_, _, err = th.App.joinUserToTeam(team, ruser1)
		require.NotNil(t, err, "Should fail")
	})

	t.Run("new join with correct scheme_admin value from group syncable", func(t *testing.T) {
		user1 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser1, _ := th.App.CreateUser(th.Context, &user1)
		defer th.App.PermanentDeleteUser(th.Context, &user1)

		group := th.CreateGroup()

		_, err = th.App.UpsertGroupMember(group.ID, user1.ID)
		require.Nil(t, err)

		gs, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
			AutoAdd:     true,
			SyncableID:  team.ID,
			Type:        model.GroupSyncableTypeTeam,
			GroupID:     group.ID,
			SchemeAdmin: false,
		})
		require.Nil(t, err)

		th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.MaxUsersPerTeam = model.NewInt(999) })

		tm1, _, err := th.App.joinUserToTeam(team, ruser1)
		require.Nil(t, err)
		require.False(t, tm1.SchemeAdmin)

		user2 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser2, _ := th.App.CreateUser(th.Context, &user2)
		defer th.App.PermanentDeleteUser(th.Context, &user2)

		_, err = th.App.UpsertGroupMember(group.ID, user2.ID)
		require.Nil(t, err)

		gs.SchemeAdmin = true
		_, err = th.App.UpdateGroupSyncable(gs)
		require.Nil(t, err)

		tm2, _, err := th.App.joinUserToTeam(team, ruser2)
		require.Nil(t, err)
		require.True(t, tm2.SchemeAdmin)
	})
}

func TestAppUpdateTeamScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team := th.BasicTeam
	mockID := model.NewString("x")
	team.SchemeID = mockID

	updatedTeam, err := th.App.UpdateTeamScheme(th.BasicTeam)
	require.Nil(t, err)
	require.Equal(t, mockID, updatedTeam.SchemeID, "Wrong Team SchemeId")
}

func TestGetTeamMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	var users []model.User
	users = append(users, *th.BasicUser)
	users = append(users, *th.BasicUser2)

	for i := 0; i < 8; i++ {
		user := model.User{
			Email:    strings.ToLower(model.NewID()) + "success+test@example.com",
			Username: fmt.Sprintf("user%v", i),
			Password: "passwd1",
			DeleteAt: int64(rand.Intn(2)),
		}
		ruser, err := th.App.CreateUser(th.Context, &user)
		require.Nil(t, err)
		require.NotNil(t, ruser)
		defer th.App.PermanentDeleteUser(th.Context, &user)

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.Nil(t, err)

		// Store the users for comparison later
		users = append(users, *ruser)
	}

	t.Run("Ensure Sorted By Username when TeamMemberGet options is passed", func(t *testing.T) {
		members, err := th.App.GetTeamMembers(th.BasicTeam.ID, 0, 100, &model.TeamMembersGetOptions{Sort: model.USERNAME})
		require.Nil(t, err)

		// Sort the users array by username
		sort.Slice(users, func(i, j int) bool {
			return users[i].Username < users[j].Username
		})

		// We should have the same number of users in both users and members array as we have not excluded any deleted members
		require.Equal(t, len(users), len(members))
		for i, member := range members {
			assert.Equal(t, users[i].ID, member.UserID)
		}
	})

	t.Run("Ensure ExcludedDeletedUsers when TeamMemberGetOptions is passed", func(t *testing.T) {
		members, err := th.App.GetTeamMembers(th.BasicTeam.ID, 0, 100, &model.TeamMembersGetOptions{ExcludeDeletedUsers: true})
		require.Nil(t, err)

		// Choose all users who aren't deleted from our users array
		var usersNotDeletedIDs []string
		var membersIDs []string
		for _, u := range users {
			if u.DeleteAt == 0 {
				usersNotDeletedIDs = append(usersNotDeletedIDs, u.ID)
			}
		}

		for _, m := range members {
			membersIDs = append(membersIDs, m.UserID)
		}

		require.Equal(t, len(usersNotDeletedIDs), len(membersIDs))
		require.ElementsMatch(t, usersNotDeletedIDs, membersIDs)
	})

	t.Run("Ensure Sorted By Username and ExcludedDeletedUsers when TeamMemberGetOptions is passed", func(t *testing.T) {
		members, err := th.App.GetTeamMembers(th.BasicTeam.ID, 0, 100, &model.TeamMembersGetOptions{Sort: model.USERNAME, ExcludeDeletedUsers: true})
		require.Nil(t, err)

		var usersNotDeleted []model.User
		for _, u := range users {
			if u.DeleteAt == 0 {
				usersNotDeleted = append(usersNotDeleted, u)
			}
		}

		// Sort our non deleted members by username
		sort.Slice(usersNotDeleted, func(i, j int) bool {
			return usersNotDeleted[i].Username < usersNotDeleted[j].Username
		})

		require.Equal(t, len(usersNotDeleted), len(members))
		for i, member := range members {
			assert.Equal(t, usersNotDeleted[i].ID, member.UserID)
		}
	})

	t.Run("Ensure Sorted By User ID when no TeamMemberGetOptions is passed", func(t *testing.T) {

		// Sort them by UserID because the result of GetTeamMembers() is also sorted
		sort.Slice(users, func(i, j int) bool {
			return users[i].ID < users[j].ID
		})

		// Fetch team members multipile times
		members, err := th.App.GetTeamMembers(th.BasicTeam.ID, 0, 5, nil)
		require.Nil(t, err)

		// This should return 5 members
		members2, err := th.App.GetTeamMembers(th.BasicTeam.ID, 5, 6, nil)
		require.Nil(t, err)
		members = append(members, members2...)

		require.Equal(t, len(users), len(members))
		for i, member := range members {
			assert.Equal(t, users[i].ID, member.UserID)
		}
	})
}

func TestGetTeamStats(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("without view restrictions", func(t *testing.T) {
		teamStats, err := th.App.GetTeamStats(th.BasicTeam.ID, nil)
		require.Nil(t, err)
		require.NotNil(t, teamStats)
		members, err := th.App.GetTeamMembers(th.BasicTeam.ID, 0, 5, nil)
		require.Nil(t, err)
		assert.Equal(t, int64(len(members)), teamStats.TotalMemberCount)
		assert.Equal(t, int64(len(members)), teamStats.ActiveMemberCount)
	})

	t.Run("with view restrictions by this team", func(t *testing.T) {
		restrictions := &model.ViewUsersRestrictions{Teams: []string{th.BasicTeam.ID}}
		teamStats, err := th.App.GetTeamStats(th.BasicTeam.ID, restrictions)
		require.Nil(t, err)
		require.NotNil(t, teamStats)
		members, err := th.App.GetTeamMembers(th.BasicTeam.ID, 0, 5, nil)
		require.Nil(t, err)
		assert.Equal(t, int64(len(members)), teamStats.TotalMemberCount)
		assert.Equal(t, int64(len(members)), teamStats.ActiveMemberCount)
	})

	t.Run("with view restrictions by valid channel", func(t *testing.T) {
		restrictions := &model.ViewUsersRestrictions{Teams: []string{}, Channels: []string{th.BasicChannel.ID}}
		teamStats, err := th.App.GetTeamStats(th.BasicTeam.ID, restrictions)
		require.Nil(t, err)
		require.NotNil(t, teamStats)
		members, err := th.App.GetChannelMembersPage(th.BasicChannel.ID, 0, 5)
		require.Nil(t, err)
		assert.Equal(t, int64(len(*members)), teamStats.TotalMemberCount)
		assert.Equal(t, int64(len(*members)), teamStats.ActiveMemberCount)
	})

	t.Run("with view restrictions to not see anything", func(t *testing.T) {
		restrictions := &model.ViewUsersRestrictions{Teams: []string{}, Channels: []string{}}
		teamStats, err := th.App.GetTeamStats(th.BasicTeam.ID, restrictions)
		require.Nil(t, err)
		require.NotNil(t, teamStats)
		assert.Equal(t, int64(0), teamStats.TotalMemberCount)
		assert.Equal(t, int64(0), teamStats.ActiveMemberCount)
	})

	t.Run("with view restrictions by other team", func(t *testing.T) {
		restrictions := &model.ViewUsersRestrictions{Teams: []string{"other-team-id"}}
		teamStats, err := th.App.GetTeamStats(th.BasicTeam.ID, restrictions)
		require.Nil(t, err)
		require.NotNil(t, teamStats)
		assert.Equal(t, int64(0), teamStats.TotalMemberCount)
		assert.Equal(t, int64(0), teamStats.ActiveMemberCount)
	})

	t.Run("with view restrictions by not-existing channel", func(t *testing.T) {
		restrictions := &model.ViewUsersRestrictions{Teams: []string{}, Channels: []string{"test"}}
		teamStats, err := th.App.GetTeamStats(th.BasicTeam.ID, restrictions)
		require.Nil(t, err)
		require.NotNil(t, teamStats)
		assert.Equal(t, int64(0), teamStats.TotalMemberCount)
		assert.Equal(t, int64(0), teamStats.ActiveMemberCount)
	})
}

func TestUpdateTeamMemberRolesChangingGuest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("from guest to user", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateGuest(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.Nil(t, err)

		_, err = th.App.UpdateTeamMemberRoles(th.BasicTeam.ID, ruser.ID, "team_user")
		require.NotNil(t, err, "Should fail when try to modify the guest role")
	})

	t.Run("from user to guest", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.Nil(t, err)

		_, err = th.App.UpdateTeamMemberRoles(th.BasicTeam.ID, ruser.ID, "team_guest")
		require.NotNil(t, err, "Should fail when try to modify the guest role")
	})

	t.Run("from user to admin", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.Nil(t, err)

		_, err = th.App.UpdateTeamMemberRoles(th.BasicTeam.ID, ruser.ID, "team_user team_admin")
		require.Nil(t, err, "Should work when you not modify guest role")
	})

	t.Run("from guest to guest plus custom", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateGuest(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.Nil(t, err)

		_, err = th.App.CreateRole(&model.Role{Name: "custom", DisplayName: "custom", Description: "custom"})
		require.Nil(t, err)

		_, err = th.App.UpdateTeamMemberRoles(th.BasicTeam.ID, ruser.ID, "team_guest custom")
		require.Nil(t, err, "Should work when you not modify guest role")
	})

	t.Run("a guest cant have user role", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewID(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateGuest(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.ID, ruser.ID, "")
		require.Nil(t, err)

		_, err = th.App.UpdateTeamMemberRoles(th.BasicTeam.ID, ruser.ID, "team_guest team_user")
		require.NotNil(t, err, "Should work when you not modify guest role")
	})
}

func TestInvalidateAllEmailInvites(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t1 := model.Token{
		Token:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		CreateAt: model.GetMillis(),
		Type:     TokenTypeGuestInvitation,
		Extra:    "",
	}
	err := th.App.Srv().Store.Token().Save(&t1)
	require.NoError(t, err)

	t2 := model.Token{
		Token:    "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy",
		CreateAt: model.GetMillis(),
		Type:     TokenTypeTeamInvitation,
		Extra:    "",
	}
	err = th.App.Srv().Store.Token().Save(&t2)
	require.NoError(t, err)

	t3 := model.Token{
		Token:    "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		CreateAt: model.GetMillis(),
		Type:     "other",
		Extra:    "",
	}
	err = th.App.Srv().Store.Token().Save(&t3)
	require.NoError(t, err)

	appErr := th.App.InvalidateAllEmailInvites()
	require.Nil(t, appErr)

	_, err = th.App.Srv().Store.Token().GetByToken(t1.Token)
	require.Error(t, err)

	_, err = th.App.Srv().Store.Token().GetByToken(t2.Token)
	require.Error(t, err)

	_, err = th.App.Srv().Store.Token().GetByToken(t3.Token)
	require.NoError(t, err)
}

func TestClearTeamMembersCache(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	mockStore := th.App.Srv().Store.(*mocks.Store)
	mockTeamStore := mocks.TeamStore{}
	tms := []*model.TeamMember{}
	for i := 0; i < 200; i++ {
		tms = append(tms, &model.TeamMember{
			TeamID: "1",
		})
	}
	mockTeamStore.On("GetMembers", "teamID", 0, 100, mock.Anything).Return(tms, nil)
	mockTeamStore.On("GetMembers", "teamID", 100, 100, mock.Anything).Return([]*model.TeamMember{{
		TeamID: "1",
	}}, nil)
	mockStore.On("Team").Return(&mockTeamStore)

	th.App.ClearTeamMembersCache("teamID")
}
