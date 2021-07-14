// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dgryski/dgoogauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/einterfaces/mocks"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mail"
	"github.com/mattermost/mattermost-server/v5/utils/testutils"

	_ "github.com/mattermost/mattermost-server/v5/model/gitlab"
)

func TestCreateUser(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := model.User{
		Email:         th.GenerateTestEmail(),
		Nickname:      "Corey Hulen",
		Password:      "hello1",
		Username:      GenerateTestUsername(),
		Roles:         model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID,
		EmailVerified: true,
	}

	ruser, resp := th.Client.CreateUser(&user)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	// Creating a user as a regular user with verified flag should not verify the new user.
	require.False(t, ruser.EmailVerified)

	_, _ = th.Client.Login(user.Email, user.Password)

	require.Equal(t, user.Nickname, ruser.Nickname, "nickname didn't match")
	require.Equal(t, model.SYSTEM_USER_ROLE_ID, ruser.Roles, "did not clear roles")

	CheckUserSanitization(t, ruser)

	_, resp = th.Client.CreateUser(ruser)
	CheckBadRequestStatus(t, resp)

	ruser.Id = ""
	ruser.Username = GenerateTestUsername()
	ruser.Password = "passwd1"
	_, resp = th.Client.CreateUser(ruser)
	CheckErrorMessage(t, resp, "app.user.save.email_exists.app_error")
	CheckBadRequestStatus(t, resp)

	ruser.Email = th.GenerateTestEmail()
	ruser.Username = user.Username
	_, resp = th.Client.CreateUser(ruser)
	CheckErrorMessage(t, resp, "app.user.save.username_exists.app_error")
	CheckBadRequestStatus(t, resp)

	ruser.Email = ""
	_, resp = th.Client.CreateUser(ruser)
	CheckErrorMessage(t, resp, "model.user.is_valid.email.app_error")
	CheckBadRequestStatus(t, resp)

	ruser.Username = "testinvalid+++"
	_, resp = th.Client.CreateUser(ruser)
	CheckErrorMessage(t, resp, "model.user.is_valid.username.app_error")
	CheckBadRequestStatus(t, resp)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableOpenServer = false })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserCreation = false })

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		user2 := &model.User{Email: th.GenerateTestEmail(), Password: "Password1", Username: GenerateTestUsername(), EmailVerified: true}
		ruser2, resp := client.CreateUser(user2)
		CheckNoError(t, resp)
		// Creating a user as sysadmin should verify the user with the EmailVerified flag.
		require.True(t, ruser2.EmailVerified)

		r, err := client.DoApiPost("/users", "garbage")
		require.NotNil(t, err, "should have errored")
		assert.Equal(t, http.StatusBadRequest, r.StatusCode)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		email := th.GenerateTestEmail()
		user2 := &model.User{Email: email, Password: "Password1", Username: GenerateTestUsername(), EmailVerified: true}
		_, resp := client.CreateUser(user2)
		CheckNoError(t, resp)
		_, appErr := th.App.GetUserByUsername(user2.Username)
		require.Nil(t, appErr)

		user3 := &model.User{Email: fmt.Sprintf(" %s  ", email), Password: "Password1", Username: GenerateTestUsername(), EmailVerified: true}
		_, resp = client.CreateUser(user3)
		CheckBadRequestStatus(t, resp)
		_, appErr = th.App.GetUserByUsername(user3.Username)
		require.NotNil(t, appErr)
	}, "Should not be able to create two users with the same email but spaces in it")
}

func TestCreateUserInputFilter(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("DomainRestriction", func(t *testing.T) {

		enableAPIUserDeletion := th.App.Config().ServiceSettings.EnableAPIUserDeletion
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.EnableOpenServer = true
			*cfg.TeamSettings.EnableUserCreation = true
			*cfg.TeamSettings.RestrictCreationToDomains = "mattermost.com"
			*cfg.ServiceSettings.EnableAPIUserDeletion = true
		})

		defer th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.RestrictCreationToDomains = ""
			*cfg.ServiceSettings.EnableAPIUserDeletion = *enableAPIUserDeletion
		})

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			user := &model.User{Email: "foobar+testdomainrestriction@mattermost.com", Password: "Password1", Username: GenerateTestUsername()}
			u, resp := client.CreateUser(user) // we need the returned created user to use its Id for deletion.
			CheckNoError(t, resp)
			_, resp = client.PermanentDeleteUser(u.Id)
			CheckNoError(t, resp)
		}, "ValidUser")

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			user := &model.User{Email: "foobar+testdomainrestriction@mattermost.org", Password: "Password1", Username: GenerateTestUsername()}
			_, resp := client.CreateUser(user)
			CheckBadRequestStatus(t, resp)
		}, "InvalidEmail")

		t.Run("ValidAuthServiceFilter", func(t *testing.T) {
			t.Run("SystemAdminClient", func(t *testing.T) {
				user := &model.User{
					Email:       "foobar+testdomainrestriction@mattermost.org",
					Username:    GenerateTestUsername(),
					AuthService: "ldap",
					AuthData:    model.NewString("999099"),
				}
				u, resp := th.SystemAdminClient.CreateUser(user)
				CheckNoError(t, resp)
				_, resp = th.SystemAdminClient.PermanentDeleteUser(u.Id)
				CheckNoError(t, resp)
			})
			t.Run("LocalClient", func(t *testing.T) {
				user := &model.User{
					Email:       "foobar+testdomainrestrictionlocalclient@mattermost.org",
					Username:    GenerateTestUsername(),
					AuthService: "ldap",
					AuthData:    model.NewString("999100"),
				}
				u, resp := th.LocalClient.CreateUser(user)
				CheckNoError(t, resp)
				_, resp = th.LocalClient.PermanentDeleteUser(u.Id)
				CheckNoError(t, resp)
			})
		})

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			user := &model.User{Email: "foobar+testdomainrestriction@mattermost.org", Password: "Password1", Username: GenerateTestUsername(), AuthService: "ldap"}
			_, resp := th.Client.CreateUser(user)
			CheckBadRequestStatus(t, resp)
		}, "InvalidAuthServiceFilter")
	})

	t.Run("Roles", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.EnableOpenServer = true
			*cfg.TeamSettings.EnableUserCreation = true
			*cfg.TeamSettings.RestrictCreationToDomains = ""
			*cfg.ServiceSettings.EnableAPIUserDeletion = true
		})

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			emailAddr := "foobar+testinvalidrole@mattermost.com"
			user := &model.User{Email: emailAddr, Password: "Password1", Username: GenerateTestUsername(), Roles: "system_user system_admin"}
			_, resp := client.CreateUser(user)
			CheckNoError(t, resp)
			ruser, err := th.App.GetUserByEmail(emailAddr)
			require.Nil(t, err)
			assert.NotEqual(t, ruser.Roles, "system_user system_admin")
			_, resp = client.PermanentDeleteUser(ruser.Id)
			CheckNoError(t, resp)
		}, "InvalidRole")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.EnableOpenServer = true
			*cfg.TeamSettings.EnableUserCreation = true
		})
		user := &model.User{Id: "AAAAAAAAAAAAAAAAAAAAAAAAAA", Email: "foobar+testinvalidid@mattermost.com", Password: "Password1", Username: GenerateTestUsername(), Roles: "system_user system_admin"}
		_, resp := client.CreateUser(user)
		CheckBadRequestStatus(t, resp)
	}, "InvalidId")
}

func TestCreateUserWithToken(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("CreateWithTokenHappyPath", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}
		token := model.NewToken(
			app.TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": user.Email}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))

		ruser, resp := th.Client.CreateUserWithToken(&user, token.Token)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		th.Client.Login(user.Email, user.Password)
		require.Equal(t, user.Nickname, ruser.Nickname)
		require.Equal(t, model.SYSTEM_USER_ROLE_ID, ruser.Roles, "should clear roles")
		CheckUserSanitization(t, ruser)
		_, err := th.App.Srv().Store.Token().GetByToken(token.Token)
		require.Error(t, err, "The token must be deleted after being used")

		teams, appErr := th.App.GetTeamsForUser(ruser.Id)
		require.Nil(t, appErr)
		require.NotEmpty(t, teams, "The user must have teams")
		require.Equal(t, th.BasicTeam.Id, teams[0].Id, "The user joined team must be the team provided.")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}
		token := model.NewToken(
			app.TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": user.Email}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))

		ruser, resp := client.CreateUserWithToken(&user, token.Token)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		th.Client.Login(user.Email, user.Password)
		require.Equal(t, user.Nickname, ruser.Nickname)
		require.Equal(t, model.SYSTEM_USER_ROLE_ID, ruser.Roles, "should clear roles")
		CheckUserSanitization(t, ruser)
		_, err := th.App.Srv().Store.Token().GetByToken(token.Token)
		require.Error(t, err, "The token must be deleted after being used")

		teams, appErr := th.App.GetTeamsForUser(ruser.Id)
		require.Nil(t, appErr)
		require.NotEmpty(t, teams, "The user must have teams")
		require.Equal(t, th.BasicTeam.Id, teams[0].Id, "The user joined team must be the team provided.")
	}, "CreateWithTokenHappyPath")

	t.Run("NoToken", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}
		token := model.NewToken(
			app.TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": user.Email}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)

		_, resp := th.Client.CreateUserWithToken(&user, "")
		CheckBadRequestStatus(t, resp)
		CheckErrorMessage(t, resp, "api.user.create_user.missing_token.app_error")
	})

	t.Run("TokenExpired", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}
		timeNow := time.Now()
		past49Hours := timeNow.Add(-49*time.Hour).UnixNano() / int64(time.Millisecond)
		token := model.NewToken(
			app.TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": user.Email}),
		)
		token.CreateAt = past49Hours
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)

		_, resp := th.Client.CreateUserWithToken(&user, token.Token)
		CheckBadRequestStatus(t, resp)
		CheckErrorMessage(t, resp, "api.user.create_user.signup_link_expired.app_error")
	})

	t.Run("WrongToken", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		_, resp := th.Client.CreateUserWithToken(&user, "wrong")
		CheckNotFoundStatus(t, resp)
		CheckErrorMessage(t, resp, "api.user.create_user.signup_link_invalid.app_error")
	})

	t.Run("EnableUserCreationDisable", func(t *testing.T) {

		enableUserCreation := th.App.Config().TeamSettings.EnableUserCreation
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.EnableUserCreation = enableUserCreation })
		}()

		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		token := model.NewToken(
			app.TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": user.Email}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserCreation = false })

		_, resp := th.Client.CreateUserWithToken(&user, token.Token)
		CheckNotImplementedStatus(t, resp)
		CheckErrorMessage(t, resp, "api.user.create_user.signup_email_disabled.app_error")

	})
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		enableUserCreation := th.App.Config().TeamSettings.EnableUserCreation
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.EnableUserCreation = enableUserCreation })

		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		token := model.NewToken(
			app.TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": user.Email}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserCreation = false })

		_, resp := client.CreateUserWithToken(&user, token.Token)
		CheckNotImplementedStatus(t, resp)
		CheckErrorMessage(t, resp, "api.user.create_user.signup_email_disabled.app_error")
	}, "EnableUserCreationDisable")

	t.Run("EnableOpenServerDisable", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		token := model.NewToken(
			app.TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": user.Email}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))

		enableOpenServer := th.App.Config().TeamSettings.EnableOpenServer
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.EnableOpenServer = enableOpenServer })
		}()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableOpenServer = false })

		ruser, resp := th.Client.CreateUserWithToken(&user, token.Token)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		th.Client.Login(user.Email, user.Password)
		require.Equal(t, user.Nickname, ruser.Nickname)
		require.Equal(t, model.SYSTEM_USER_ROLE_ID, ruser.Roles, "should clear roles")
		CheckUserSanitization(t, ruser)
		_, err := th.App.Srv().Store.Token().GetByToken(token.Token)
		require.Error(t, err, "The token must be deleted after be used")
	})
}

func TestCreateUserWebSocketEvent(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("guest should not received new_user event but user should", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicense("guests"))
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.AllowEmailAccounts = true })

		id := model.NewId()
		guestPassword := "Pa$$word11"
		guest := &model.User{
			Email:         "success+" + id + "@simulator.amazonses.com",
			Username:      "un_" + id,
			Nickname:      "nn_" + id,
			Password:      guestPassword,
			EmailVerified: true,
		}

		guest, err := th.App.CreateGuest(th.Context, guest)
		require.Nil(t, err)

		_, _, err = th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, guest.Id, "")
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(guest, th.BasicChannel, false)
		require.Nil(t, err)

		guestClient := th.CreateClient()

		_, resp := guestClient.Login(guest.Email, guestPassword)
		require.Nil(t, resp.Error)

		guestWSClient, err := th.CreateWebSocketClientWithClient(guestClient)
		require.Nil(t, err)
		defer guestWSClient.Close()
		guestWSClient.Listen()

		userWSClient, err := th.CreateWebSocketClient()
		require.Nil(t, err)
		defer userWSClient.Close()
		userWSClient.Listen()

		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		inviteId := th.BasicTeam.InviteId

		_, resp = th.Client.CreateUserWithInviteId(&user, inviteId)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		var userHasReceived bool
		var guestHasReceived bool

		func() {
			for {
				select {
				case ev := <-userWSClient.EventChannel:
					if ev.EventType() == model.WEBSOCKET_EVENT_NEW_USER {
						userHasReceived = true
					}
				case ev := <-guestWSClient.EventChannel:
					if ev.EventType() == model.WEBSOCKET_EVENT_NEW_USER {
						guestHasReceived = true
					}
				case <-time.After(2 * time.Second):
					return
				}
			}
		}()

		require.Truef(t, userHasReceived, "User should have received %s event", model.WEBSOCKET_EVENT_NEW_USER)
		require.Falsef(t, guestHasReceived, "Guest should not have received %s event", model.WEBSOCKET_EVENT_NEW_USER)
	})
}

func TestCreateUserWithInviteId(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("CreateWithInviteIdHappyPath", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		inviteId := th.BasicTeam.InviteId

		ruser, resp := th.Client.CreateUserWithInviteId(&user, inviteId)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		th.Client.Login(user.Email, user.Password)
		require.Equal(t, user.Nickname, ruser.Nickname)
		require.Equal(t, model.SYSTEM_USER_ROLE_ID, ruser.Roles, "should clear roles")
		CheckUserSanitization(t, ruser)
	})
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		inviteId := th.BasicTeam.InviteId

		ruser, resp := client.CreateUserWithInviteId(&user, inviteId)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		th.Client.Login(user.Email, user.Password)
		require.Equal(t, user.Nickname, ruser.Nickname)
		require.Equal(t, model.SYSTEM_USER_ROLE_ID, ruser.Roles, "should clear roles")
		CheckUserSanitization(t, ruser)
	}, "CreateWithInviteIdHappyPath")

	t.Run("GroupConstrainedTeam", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		th.BasicTeam.GroupConstrained = model.NewBool(true)
		team, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err)

		defer func() {
			th.BasicTeam.GroupConstrained = model.NewBool(false)
			_, err = th.App.UpdateTeam(th.BasicTeam)
			require.Nil(t, err)
		}()

		inviteID := team.InviteId

		_, resp := th.Client.CreateUserWithInviteId(&user, inviteID)
		require.Equal(t, "app.team.invite_id.group_constrained.error", resp.Error.Id)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		th.BasicTeam.GroupConstrained = model.NewBool(true)
		team, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err)

		defer func() {
			th.BasicTeam.GroupConstrained = model.NewBool(false)
			_, err = th.App.UpdateTeam(th.BasicTeam)
			require.Nil(t, err)
		}()

		inviteID := team.InviteId

		_, resp := client.CreateUserWithInviteId(&user, inviteID)
		require.Equal(t, "app.team.invite_id.group_constrained.error", resp.Error.Id)
	}, "GroupConstrainedTeam")

	t.Run("WrongInviteId", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		inviteId := model.NewId()

		_, resp := th.Client.CreateUserWithInviteId(&user, inviteId)
		CheckNotFoundStatus(t, resp)
		CheckErrorMessage(t, resp, "app.team.get_by_invite_id.finding.app_error")
	})

	t.Run("NoInviteId", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		_, resp := th.Client.CreateUserWithInviteId(&user, "")
		CheckBadRequestStatus(t, resp)
		CheckErrorMessage(t, resp, "api.user.create_user.missing_invite_id.app_error")
	})

	t.Run("ExpiredInviteId", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		inviteId := th.BasicTeam.InviteId

		_, resp := th.SystemAdminClient.RegenerateTeamInviteId(th.BasicTeam.Id)
		CheckNoError(t, resp)

		_, resp = th.Client.CreateUserWithInviteId(&user, inviteId)
		CheckNotFoundStatus(t, resp)
		CheckErrorMessage(t, resp, "app.team.get_by_invite_id.finding.app_error")
	})

	t.Run("EnableUserCreationDisable", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		enableUserCreation := th.App.Config().TeamSettings.EnableUserCreation
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.EnableUserCreation = enableUserCreation })
		}()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserCreation = false })

		inviteId := th.BasicTeam.InviteId

		_, resp := th.Client.CreateUserWithInviteId(&user, inviteId)
		CheckNotImplementedStatus(t, resp)
		CheckErrorMessage(t, resp, "api.user.create_user.signup_email_disabled.app_error")
	})
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		enableUserCreation := th.App.Config().TeamSettings.EnableUserCreation
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.EnableUserCreation = enableUserCreation })

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserCreation = false })

		inviteId := th.BasicTeam.InviteId
		_, resp := client.CreateUserWithInviteId(&user, inviteId)
		CheckNotImplementedStatus(t, resp)
		CheckErrorMessage(t, resp, "api.user.create_user.signup_email_disabled.app_error")
	}, "EnableUserCreationDisable")

	t.Run("EnableOpenServerDisable", func(t *testing.T) {
		user := model.User{Email: th.GenerateTestEmail(), Nickname: "Corey Hulen", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		enableOpenServer := th.App.Config().TeamSettings.EnableOpenServer
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.EnableOpenServer = enableOpenServer })
		}()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableOpenServer = false })

		team, res := th.SystemAdminClient.RegenerateTeamInviteId(th.BasicTeam.Id)
		assert.Nil(t, res.Error)
		inviteId := team.InviteId

		ruser, resp := th.Client.CreateUserWithInviteId(&user, inviteId)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		th.Client.Login(user.Email, user.Password)
		require.Equal(t, user.Nickname, ruser.Nickname)
		require.Equal(t, model.SYSTEM_USER_ROLE_ID, ruser.Roles, "should clear roles")
		CheckUserSanitization(t, ruser)
	})
}

func TestGetMe(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	ruser, resp := th.Client.GetMe("")
	CheckNoError(t, resp)

	require.Equal(t, th.BasicUser.Id, ruser.Id)

	th.Client.Logout()
	_, resp = th.Client.GetMe("")
	CheckUnauthorizedStatus(t, resp)
}

func TestGetUser(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := th.CreateUser()
	user.Props = map[string]string{"testpropkey": "testpropvalue"}

	th.App.UpdateUser(user, false)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		ruser, resp := client.GetUser(user.Id, "")
		CheckNoError(t, resp)
		CheckUserSanitization(t, ruser)

		require.Equal(t, user.Email, ruser.Email)

		assert.NotNil(t, ruser.Props)
		assert.Equal(t, ruser.Props["testpropkey"], "testpropvalue")
		require.False(t, ruser.IsBot)

		ruser, resp = client.GetUser(user.Id, resp.Etag)
		CheckEtag(t, ruser, resp)

		_, resp = client.GetUser("junk", "")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetUser(model.NewId(), "")
		CheckNotFoundStatus(t, resp)
	})

	// Check against privacy config settings
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowEmailAddress = false })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = false })

	ruser, resp := th.Client.GetUser(user.Id, "")
	CheckNoError(t, resp)

	require.Empty(t, ruser.Email, "email should be blank")
	require.Empty(t, ruser.FirstName, "first name should be blank")
	require.Empty(t, ruser.LastName, "last name should be blank")

	th.Client.Logout()
	_, resp = th.Client.GetUser(user.Id, "")
	CheckUnauthorizedStatus(t, resp)

	// System admins should ignore privacy settings
	ruser, _ = th.SystemAdminClient.GetUser(user.Id, resp.Etag)
	require.NotEmpty(t, ruser.Email, "email should not be blank")
	require.NotEmpty(t, ruser.FirstName, "first name should not be blank")
	require.NotEmpty(t, ruser.LastName, "last name should not be blank")
}

func TestGetUserWithAcceptedTermsOfServiceForOtherUser(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := th.CreateUser()

	tos, _ := th.App.CreateTermsOfService("Dummy TOS", user.Id)

	th.App.UpdateUser(user, false)

	ruser, resp := th.Client.GetUser(user.Id, "")
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, user.Email, ruser.Email)

	assert.Empty(t, ruser.TermsOfServiceId)

	th.App.SaveUserTermsOfService(user.Id, tos.Id, true)

	ruser, resp = th.Client.GetUser(user.Id, "")
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, user.Email, ruser.Email)

	// user TOS data cannot be fetched for other users by non-admin users
	assert.Empty(t, ruser.TermsOfServiceId)
}

func TestGetUserWithAcceptedTermsOfService(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser

	tos, _ := th.App.CreateTermsOfService("Dummy TOS", user.Id)

	ruser, resp := th.Client.GetUser(user.Id, "")
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, user.Email, ruser.Email)

	assert.Empty(t, ruser.TermsOfServiceId)

	th.App.SaveUserTermsOfService(user.Id, tos.Id, true)

	ruser, resp = th.Client.GetUser(user.Id, "")
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, user.Email, ruser.Email)

	// a user can view their own TOS details
	assert.Equal(t, tos.Id, ruser.TermsOfServiceId)
}

func TestGetUserWithAcceptedTermsOfServiceWithAdminUser(t *testing.T) {
	th := Setup(t).InitBasic()
	th.LoginSystemAdmin()
	defer th.TearDown()

	user := th.BasicUser

	tos, _ := th.App.CreateTermsOfService("Dummy TOS", user.Id)

	ruser, resp := th.SystemAdminClient.GetUser(user.Id, "")
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, user.Email, ruser.Email)

	assert.Empty(t, ruser.TermsOfServiceId)

	th.App.SaveUserTermsOfService(user.Id, tos.Id, true)

	ruser, resp = th.SystemAdminClient.GetUser(user.Id, "")
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, user.Email, ruser.Email)

	// admin can view anyone's TOS details
	assert.Equal(t, tos.Id, ruser.TermsOfServiceId)
}

func TestGetBotUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

	th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
	th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.TEAM_USER_ROLE_ID, false)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	bot := &model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "a bot",
		Description: "bot",
	}

	createdBot, resp := th.Client.CreateBot(bot)
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

	botUser, resp := th.Client.GetUser(createdBot.UserId, "")
	CheckNoError(t, resp)
	require.Equal(t, bot.Username, botUser.Username)
	require.True(t, botUser.IsBot)
}

func TestGetUserByUsername(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		ruser, resp := client.GetUserByUsername(user.Username, "")
		CheckNoError(t, resp)
		CheckUserSanitization(t, ruser)

		require.Equal(t, user.Email, ruser.Email)

		ruser, resp = client.GetUserByUsername(user.Username, resp.Etag)
		CheckEtag(t, ruser, resp)

		_, resp = client.GetUserByUsername(GenerateTestUsername(), "")
		CheckNotFoundStatus(t, resp)
	})

	// Check against privacy config settings
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowEmailAddress = false })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = false })

	ruser, resp := th.Client.GetUserByUsername(th.BasicUser2.Username, "")
	CheckNoError(t, resp)

	require.Empty(t, ruser.Email, "email should be blank")
	require.Empty(t, ruser.FirstName, "first name should be blank")
	require.Empty(t, ruser.LastName, "last name should be blank")

	ruser, resp = th.Client.GetUserByUsername(th.BasicUser.Username, "")
	CheckNoError(t, resp)
	require.NotEmpty(t, ruser.NotifyProps, "notify props should be sent")

	th.Client.Logout()
	_, resp = th.Client.GetUserByUsername(user.Username, "")
	CheckUnauthorizedStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		// System admins should ignore privacy settings
		ruser, _ = client.GetUserByUsername(user.Username, resp.Etag)
		require.NotEmpty(t, ruser.Email, "email should not be blank")
		require.NotEmpty(t, ruser.FirstName, "first name should not be blank")
		require.NotEmpty(t, ruser.LastName, "last name should not be blank")
	})
}

func TestGetUserByUsernameWithAcceptedTermsOfService(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser

	ruser, resp := th.Client.GetUserByUsername(user.Username, "")
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, user.Email, ruser.Email)

	tos, _ := th.App.CreateTermsOfService("Dummy TOS", user.Id)
	th.App.SaveUserTermsOfService(ruser.Id, tos.Id, true)

	ruser, resp = th.Client.GetUserByUsername(user.Username, "")
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, user.Email, ruser.Email)

	require.Equal(t, tos.Id, ruser.TermsOfServiceId, "Terms of service ID should match")
}

func TestSaveUserTermsOfService(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("Invalid data", func(t *testing.T) {
		resp, err := th.Client.DoApiPost("/users/"+th.BasicUser.Id+"/terms_of_service", "{}")
		require.NotNil(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestGetUserByEmail(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := th.CreateUser()
	userWithSlash, resp := th.SystemAdminClient.CreateUser(&model.User{
		Email:    "email/with/slashes@example.com",
		Username: GenerateTestUsername(),
		Password: "Pa$$word11",
	})
	require.Nil(t, resp.Error)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PrivacySettings.ShowEmailAddress = true
		*cfg.PrivacySettings.ShowFullName = true
	})

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		t.Run("should be able to get another user by email", func(t *testing.T) {
			ruser, resp := client.GetUserByEmail(user.Email, "")
			CheckNoError(t, resp)
			CheckUserSanitization(t, ruser)

			require.Equal(t, user.Email, ruser.Email)
		})

		t.Run("Get user with a / character in the email", func(t *testing.T) {
			ruser, resp := client.GetUserByEmail(userWithSlash.Email, "")
			require.Nil(t, resp.Error)
			require.Equal(t, ruser.Id, userWithSlash.Id)
		})

		t.Run("should return not modified when provided with a matching etag", func(t *testing.T) {
			_, resp := client.GetUserByEmail(user.Email, "")
			CheckNoError(t, resp)

			ruser, resp := client.GetUserByEmail(user.Email, resp.Etag)
			CheckEtag(t, ruser, resp)
		})

		t.Run("should return bad request when given an invalid email", func(t *testing.T) {
			_, resp := client.GetUserByEmail(GenerateTestUsername(), "")
			CheckBadRequestStatus(t, resp)
		})

		t.Run("should return 404 when given a non-existent email", func(t *testing.T) {
			_, resp := client.GetUserByEmail(th.GenerateTestEmail(), "")
			CheckNotFoundStatus(t, resp)
		})
	})

	t.Run("should sanitize full name for non-admin based on privacy settings", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.ShowEmailAddress = true
			*cfg.PrivacySettings.ShowFullName = false
		})

		ruser, resp := th.Client.GetUserByEmail(user.Email, "")
		CheckNoError(t, resp)
		assert.Equal(t, "", ruser.FirstName, "first name should be blank")
		assert.Equal(t, "", ruser.LastName, "last name should be blank")

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.ShowFullName = true
		})

		ruser, resp = th.Client.GetUserByEmail(user.Email, "")
		CheckNoError(t, resp)
		assert.NotEqual(t, "", ruser.FirstName, "first name should be set")
		assert.NotEqual(t, "", ruser.LastName, "last name should be set")
	})

	t.Run("should return forbidden for non-admin when privacy settings hide email", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.ShowEmailAddress = false
		})

		_, resp := th.Client.GetUserByEmail(user.Email, "")
		CheckForbiddenStatus(t, resp)

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.ShowEmailAddress = true
		})

		ruser, resp := th.Client.GetUserByEmail(user.Email, "")
		CheckNoError(t, resp)
		assert.Equal(t, user.Email, ruser.Email, "email should be set")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		t.Run("should not sanitize full name for admin, regardless of privacy settings", func(t *testing.T) {
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.PrivacySettings.ShowEmailAddress = true
				*cfg.PrivacySettings.ShowFullName = false
			})

			ruser, resp := client.GetUserByEmail(user.Email, "")
			CheckNoError(t, resp)
			assert.NotEqual(t, "", ruser.FirstName, "first name should be set")
			assert.NotEqual(t, "", ruser.LastName, "last name should be set")

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.PrivacySettings.ShowFullName = true
			})

			ruser, resp = client.GetUserByEmail(user.Email, "")
			CheckNoError(t, resp)
			assert.NotEqual(t, "", ruser.FirstName, "first name should be set")
			assert.NotEqual(t, "", ruser.LastName, "last name should be set")
		})

		t.Run("should always return email for admin, regardless of privacy settings", func(t *testing.T) {
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.PrivacySettings.ShowEmailAddress = false
			})

			ruser, resp := client.GetUserByEmail(user.Email, "")
			CheckNoError(t, resp)
			assert.Equal(t, user.Email, ruser.Email, "email should be set")

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.PrivacySettings.ShowEmailAddress = true
			})

			ruser, resp = client.GetUserByEmail(user.Email, "")
			CheckNoError(t, resp)
			assert.Equal(t, user.Email, ruser.Email, "email should be set")
		})
	})
}

func TestSearchUsers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	search := &model.UserSearch{Term: th.BasicUser.Username}

	users, resp := th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.True(t, findUserInList(th.BasicUser.Id, users), "should have found user")

	_, err := th.App.UpdateActive(th.Context, th.BasicUser2, false)
	require.Nil(t, err)

	search.Term = th.BasicUser2.Username
	search.AllowInactive = false

	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.False(t, findUserInList(th.BasicUser2.Id, users), "should not have found user")

	search.AllowInactive = true

	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.True(t, findUserInList(th.BasicUser2.Id, users), "should have found user")

	search.Term = th.BasicUser.Username
	search.AllowInactive = false
	search.TeamId = th.BasicTeam.Id

	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.True(t, findUserInList(th.BasicUser.Id, users), "should have found user")

	search.NotInChannelId = th.BasicChannel.Id

	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.False(t, findUserInList(th.BasicUser.Id, users), "should not have found user")

	search.TeamId = ""
	search.NotInChannelId = ""
	search.InChannelId = th.BasicChannel.Id

	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.True(t, findUserInList(th.BasicUser.Id, users), "should have found user")

	search.InChannelId = ""
	search.NotInChannelId = th.BasicChannel.Id
	_, resp = th.Client.SearchUsers(search)
	CheckBadRequestStatus(t, resp)

	search.NotInChannelId = model.NewId()
	search.TeamId = model.NewId()
	_, resp = th.Client.SearchUsers(search)
	CheckForbiddenStatus(t, resp)

	search.NotInChannelId = ""
	search.TeamId = model.NewId()
	_, resp = th.Client.SearchUsers(search)
	CheckForbiddenStatus(t, resp)

	search.InChannelId = model.NewId()
	search.TeamId = ""
	_, resp = th.Client.SearchUsers(search)
	CheckForbiddenStatus(t, resp)

	// Test search for users not in any team
	search.TeamId = ""
	search.NotInChannelId = ""
	search.InChannelId = ""
	search.NotInTeamId = th.BasicTeam.Id

	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.False(t, findUserInList(th.BasicUser.Id, users), "should not have found user")

	oddUser := th.CreateUser()
	search.Term = oddUser.Username

	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.True(t, findUserInList(oddUser.Id, users), "should have found user")

	_, resp = th.SystemAdminClient.AddTeamMember(th.BasicTeam.Id, oddUser.Id)
	CheckNoError(t, resp)

	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.False(t, findUserInList(oddUser.Id, users), "should not have found user")

	search.NotInTeamId = model.NewId()
	_, resp = th.Client.SearchUsers(search)
	CheckForbiddenStatus(t, resp)

	search.Term = th.BasicUser.Username

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowEmailAddress = false })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = false })

	_, err = th.App.UpdateActive(th.Context, th.BasicUser2, true)
	require.Nil(t, err)

	search.InChannelId = ""
	search.NotInTeamId = ""
	search.Term = th.BasicUser2.Email
	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.False(t, findUserInList(th.BasicUser2.Id, users), "should not have found user")

	search.Term = th.BasicUser2.FirstName
	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.False(t, findUserInList(th.BasicUser2.Id, users), "should not have found user")

	search.Term = th.BasicUser2.LastName
	users, resp = th.Client.SearchUsers(search)
	CheckNoError(t, resp)

	require.False(t, findUserInList(th.BasicUser2.Id, users), "should not have found user")

	search.Term = th.BasicUser.FirstName
	search.InChannelId = th.BasicChannel.Id
	search.NotInChannelId = th.BasicChannel.Id
	search.TeamId = th.BasicTeam.Id
	users, resp = th.SystemAdminClient.SearchUsers(search)
	CheckNoError(t, resp)

	require.True(t, findUserInList(th.BasicUser.Id, users), "should have found user")

	id := model.NewId()
	group, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn-foo_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteId:    model.NewId(),
	})
	assert.Nil(t, err)

	search = &model.UserSearch{Term: th.BasicUser.Username, InGroupId: group.Id}
	t.Run("Requires ldap license when searching in group", func(t *testing.T) {
		_, resp = th.SystemAdminClient.SearchUsers(search)
		CheckNotImplementedStatus(t, resp)
	})

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	t.Run("Requires manage system permission when searching for users in a group", func(t *testing.T) {
		_, resp = th.Client.SearchUsers(search)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Returns empty list when no users found searching for users in a group", func(t *testing.T) {
		users, resp = th.SystemAdminClient.SearchUsers(search)
		CheckNoError(t, resp)
		require.Empty(t, users)
	})

	_, err = th.App.UpsertGroupMember(group.Id, th.BasicUser.Id)
	assert.Nil(t, err)

	t.Run("Returns user in group user found in group", func(t *testing.T) {
		users, resp = th.SystemAdminClient.SearchUsers(search)
		CheckNoError(t, resp)
		require.Equal(t, users[0].Id, th.BasicUser.Id)
	})
}

func findUserInList(id string, users []*model.User) bool {
	for _, user := range users {
		if user.Id == id {
			return true
		}
	}
	return false
}

func TestAutocompleteUsersInChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	teamId := th.BasicTeam.Id
	channelId := th.BasicChannel.Id
	username := th.BasicUser.Username
	newUser := th.CreateUser()

	tt := []struct {
		Name            string
		TeamId          string
		ChannelId       string
		Username        string
		ExpectedResults int
		MoreThan        bool
		ShouldFail      bool
	}{
		{
			"Autocomplete in channel for specific username",
			teamId,
			channelId,
			username,
			1,
			false,
			false,
		},
		{
			"Search for not valid username",
			teamId,
			channelId,
			"amazonses",
			0,
			false,
			false,
		},
		{
			"Search for all users",
			teamId,
			channelId,
			"",
			2,
			true,
			false,
		},
		{
			"Fail when the teamId is not provided",
			"",
			channelId,
			"",
			2,
			true,
			true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			th.LoginBasic()
			rusers, resp := th.Client.AutocompleteUsersInChannel(tc.TeamId, tc.ChannelId, tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			if tc.ShouldFail {
				CheckErrorMessage(t, resp, "api.user.autocomplete_users.missing_team_id.app_error")
			} else {
				CheckNoError(t, resp)
				if tc.MoreThan {
					assert.True(t, len(rusers.Users) >= tc.ExpectedResults)
				} else {
					assert.Len(t, rusers.Users, tc.ExpectedResults)
				}
			}

			th.Client.Logout()
			_, resp = th.Client.AutocompleteUsersInChannel(tc.TeamId, tc.ChannelId, tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			CheckUnauthorizedStatus(t, resp)

			th.Client.Login(newUser.Email, newUser.Password)
			_, resp = th.Client.AutocompleteUsersInChannel(tc.TeamId, tc.ChannelId, tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			CheckForbiddenStatus(t, resp)
		})
	}

	t.Run("Check against privacy config settings", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = false })

		th.LoginBasic()
		rusers, resp := th.Client.AutocompleteUsersInChannel(teamId, channelId, username, model.USER_SEARCH_DEFAULT_LIMIT, "")
		CheckNoError(t, resp)

		assert.Equal(t, rusers.Users[0].FirstName, "", "should not show first/last name")
		assert.Equal(t, rusers.Users[0].LastName, "", "should not show first/last name")
	})

	t.Run("Check OutOfChannel results with/without VIEW_MEMBERS permissions", func(t *testing.T) {
		permissionsUser := th.CreateUser()
		th.SystemAdminClient.DemoteUserToGuest(permissionsUser.Id)
		permissionsUser.Roles = "system_guest"
		th.LinkUserToTeam(permissionsUser, th.BasicTeam)
		th.AddUserToChannel(permissionsUser, th.BasicChannel)

		otherUser := th.CreateUser()
		th.LinkUserToTeam(otherUser, th.BasicTeam)

		th.Client.Login(permissionsUser.Email, permissionsUser.Password)

		rusers, resp := th.Client.AutocompleteUsersInChannel(teamId, channelId, "", model.USER_SEARCH_DEFAULT_LIMIT, "")
		CheckNoError(t, resp)
		assert.Len(t, rusers.OutOfChannel, 1)

		defaultRolePermissions := th.SaveDefaultRolePermissions()
		defer func() {
			th.RestoreDefaultRolePermissions(defaultRolePermissions)
		}()

		th.RemovePermissionFromRole(model.PERMISSION_VIEW_MEMBERS.Id, model.SYSTEM_USER_ROLE_ID)
		th.RemovePermissionFromRole(model.PERMISSION_VIEW_MEMBERS.Id, model.TEAM_USER_ROLE_ID)

		rusers, resp = th.Client.AutocompleteUsersInChannel(teamId, channelId, "", model.USER_SEARCH_DEFAULT_LIMIT, "")
		CheckNoError(t, resp)
		assert.Empty(t, rusers.OutOfChannel)

		th.App.GetOrCreateDirectChannel(th.Context, permissionsUser.Id, otherUser.Id)

		rusers, resp = th.Client.AutocompleteUsersInChannel(teamId, channelId, "", model.USER_SEARCH_DEFAULT_LIMIT, "")
		CheckNoError(t, resp)
		assert.Len(t, rusers.OutOfChannel, 1)
	})

	t.Run("user must have access to team id, especially when it does not match channel's team id", func(t *testing.T) {
		_, resp := th.Client.AutocompleteUsersInChannel("otherTeamId", channelId, username, model.USER_SEARCH_DEFAULT_LIMIT, "")
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})
}

func TestAutocompleteUsersInTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	teamId := th.BasicTeam.Id
	username := th.BasicUser.Username
	newUser := th.CreateUser()

	tt := []struct {
		Name            string
		TeamId          string
		Username        string
		ExpectedResults int
		MoreThan        bool
	}{
		{
			"specific username",
			teamId,
			username,
			1,
			false,
		},
		{
			"not valid username",
			teamId,
			"amazonses",
			0,
			false,
		},
		{
			"all users in team",
			teamId,
			"",
			2,
			true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			th.LoginBasic()
			rusers, resp := th.Client.AutocompleteUsersInTeam(tc.TeamId, tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			CheckNoError(t, resp)
			if tc.MoreThan {
				assert.True(t, len(rusers.Users) >= tc.ExpectedResults)
			} else {
				assert.Len(t, rusers.Users, tc.ExpectedResults)
			}
			th.Client.Logout()
			_, resp = th.Client.AutocompleteUsersInTeam(tc.TeamId, tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			CheckUnauthorizedStatus(t, resp)

			th.Client.Login(newUser.Email, newUser.Password)
			_, resp = th.Client.AutocompleteUsersInTeam(tc.TeamId, tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			CheckForbiddenStatus(t, resp)
		})
	}

	t.Run("Check against privacy config settings", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = false })

		th.LoginBasic()
		rusers, resp := th.Client.AutocompleteUsersInTeam(teamId, username, model.USER_SEARCH_DEFAULT_LIMIT, "")
		CheckNoError(t, resp)

		assert.Equal(t, rusers.Users[0].FirstName, "", "should not show first/last name")
		assert.Equal(t, rusers.Users[0].LastName, "", "should not show first/last name")
	})
}

func TestAutocompleteUsers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	username := th.BasicUser.Username
	newUser := th.CreateUser()

	tt := []struct {
		Name            string
		Username        string
		ExpectedResults int
		MoreThan        bool
	}{
		{
			"specific username",
			username,
			1,
			false,
		},
		{
			"not valid username",
			"amazonses",
			0,
			false,
		},
		{
			"all users in team",
			"",
			2,
			true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			th.LoginBasic()
			rusers, resp := th.Client.AutocompleteUsers(tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			CheckNoError(t, resp)
			if tc.MoreThan {
				assert.True(t, len(rusers.Users) >= tc.ExpectedResults)
			} else {
				assert.Len(t, rusers.Users, tc.ExpectedResults)
			}

			th.Client.Logout()
			_, resp = th.Client.AutocompleteUsers(tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			CheckUnauthorizedStatus(t, resp)

			th.Client.Login(newUser.Email, newUser.Password)
			_, resp = th.Client.AutocompleteUsers(tc.Username, model.USER_SEARCH_DEFAULT_LIMIT, "")
			CheckNoError(t, resp)
		})
	}

	t.Run("Check against privacy config settings", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = false })

		th.LoginBasic()
		rusers, resp := th.Client.AutocompleteUsers(username, model.USER_SEARCH_DEFAULT_LIMIT, "")
		CheckNoError(t, resp)

		assert.Equal(t, rusers.Users[0].FirstName, "", "should not show first/last name")
		assert.Equal(t, rusers.Users[0].LastName, "", "should not show first/last name")
	})
}

func TestGetProfileImage(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// recreate basic user
	th.BasicUser = th.CreateUser()
	th.LoginBasic()
	user := th.BasicUser

	data, resp := th.Client.GetProfileImage(user.Id, "")
	CheckNoError(t, resp)
	require.NotEmpty(t, data, "should not be empty")

	_, resp = th.Client.GetProfileImage(user.Id, resp.Etag)
	require.NotEqual(t, http.StatusNotModified, resp.StatusCode, "should not hit etag")

	_, resp = th.Client.GetProfileImage("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.GetProfileImage(model.NewId(), "")
	CheckNotFoundStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.GetProfileImage(user.Id, "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetProfileImage(user.Id, "")
	CheckNoError(t, resp)

	info := &model.FileInfo{Path: "/users/" + user.Id + "/profile.png"}
	err := th.cleanupTestFile(info)
	require.NoError(t, err)
}

func TestGetUsersByIds(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		t.Run("should return the user", func(t *testing.T) {
			users, resp := client.GetUsersByIds([]string{th.BasicUser.Id})

			CheckNoError(t, resp)

			assert.Equal(t, th.BasicUser.Id, users[0].Id)
			CheckUserSanitization(t, users[0])
		})

		t.Run("should return error when no IDs are specified", func(t *testing.T) {
			_, resp := client.GetUsersByIds([]string{})

			CheckBadRequestStatus(t, resp)
		})

		t.Run("should not return an error for invalid IDs", func(t *testing.T) {
			users, resp := client.GetUsersByIds([]string{"junk"})

			CheckNoError(t, resp)
			require.Empty(t, users, "no users should be returned")
		})

		t.Run("should still return users for valid IDs when invalid IDs are specified", func(t *testing.T) {
			users, resp := client.GetUsersByIds([]string{"junk", th.BasicUser.Id})

			CheckNoError(t, resp)

			require.Len(t, users, 1, "1 user should be returned")
		})
	})

	t.Run("should return error when not logged in", func(t *testing.T) {
		th.Client.Logout()

		_, resp := th.Client.GetUsersByIds([]string{th.BasicUser.Id})
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestGetUsersByIdsWithOptions(t *testing.T) {
	t.Run("should only return specified users that have been updated since the given time", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()

		// Users before the timestamp shouldn't be returned
		user1, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Username: model.NewId(), Password: model.NewId()})
		require.Nil(t, err)

		user2, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Username: model.NewId(), Password: model.NewId()})
		require.Nil(t, err)

		// Users not in the list of IDs shouldn't be returned
		_, err = th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Username: model.NewId(), Password: model.NewId()})
		require.Nil(t, err)

		users, resp := th.Client.GetUsersByIdsWithOptions([]string{user1.Id, user2.Id}, &model.UserGetByIdsOptions{
			Since: user2.UpdateAt - 1,
		})

		assert.Nil(t, resp.Error)
		assert.Len(t, users, 1)
		assert.Equal(t, users[0].Id, user2.Id)
	})
}

func TestGetUsersByGroupChannelIds(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	gc1, err := th.App.CreateGroupChannel([]string{th.BasicUser.Id, th.SystemAdminUser.Id, th.TeamAdminUser.Id}, th.BasicUser.Id)
	require.Nil(t, err)

	usersByChannelId, resp := th.Client.GetUsersByGroupChannelIds([]string{gc1.Id})
	CheckNoError(t, resp)

	users, ok := usersByChannelId[gc1.Id]
	assert.True(t, ok)
	userIds := []string{}
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}

	require.ElementsMatch(t, []string{th.SystemAdminUser.Id, th.TeamAdminUser.Id}, userIds)

	th.LoginBasic2()
	usersByChannelId, resp = th.Client.GetUsersByGroupChannelIds([]string{gc1.Id})
	CheckNoError(t, resp)

	_, ok = usersByChannelId[gc1.Id]
	require.False(t, ok)

	th.Client.Logout()
	_, resp = th.Client.GetUsersByGroupChannelIds([]string{gc1.Id})
	CheckUnauthorizedStatus(t, resp)
}

func TestGetUsersByUsernames(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	users, resp := th.Client.GetUsersByUsernames([]string{th.BasicUser.Username})
	CheckNoError(t, resp)

	require.Equal(t, th.BasicUser.Id, users[0].Id)
	CheckUserSanitization(t, users[0])

	_, resp = th.Client.GetUsersByIds([]string{})
	CheckBadRequestStatus(t, resp)

	users, resp = th.Client.GetUsersByUsernames([]string{"junk"})
	CheckNoError(t, resp)
	require.Empty(t, users, "no users should be returned")

	users, resp = th.Client.GetUsersByUsernames([]string{"junk", th.BasicUser.Username})
	CheckNoError(t, resp)
	require.Len(t, users, 1, "1 user should be returned")

	th.Client.Logout()
	_, resp = th.Client.GetUsersByUsernames([]string{th.BasicUser.Username})
	CheckUnauthorizedStatus(t, resp)
}

func TestGetTotalUsersStat(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	total, _ := th.Server.Store.User().Count(model.UserCountOptions{
		IncludeDeleted:     false,
		IncludeBotAccounts: true,
	})

	rstats, resp := th.Client.GetTotalUsersStats("")
	CheckNoError(t, resp)

	require.Equal(t, total, rstats.TotalUsersCount)
}

func TestUpdateUser(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := th.CreateUser()
	th.Client.Login(user.Email, user.Password)

	user.Nickname = "Joram Wilander"
	user.Roles = model.SYSTEM_USER_ROLE_ID
	user.LastPasswordUpdate = 123

	ruser, resp := th.Client.UpdateUser(user)
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, "Joram Wilander", ruser.Nickname, "Nickname should update properly")
	require.Equal(t, model.SYSTEM_USER_ROLE_ID, ruser.Roles, "Roles should not update")
	require.NotEqual(t, 123, ruser.LastPasswordUpdate, "LastPasswordUpdate should not update")

	ruser.Email = th.GenerateTestEmail()
	_, resp = th.Client.UpdateUser(ruser)
	CheckBadRequestStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		ruser.Email = th.GenerateTestEmail()
		_, resp = client.UpdateUser(user)
		CheckNoError(t, resp)
	})

	ruser.Password = user.Password
	ruser, resp = th.Client.UpdateUser(ruser)
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	ruser.Id = "junk"
	_, resp = th.Client.UpdateUser(ruser)
	CheckBadRequestStatus(t, resp)

	ruser.Id = model.NewId()
	_, resp = th.Client.UpdateUser(ruser)
	CheckForbiddenStatus(t, resp)

	r, err := th.Client.DoApiPut("/users/"+ruser.Id, "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	session, _ := th.App.GetSession(th.Client.AuthToken)
	session.IsOAuth = true
	th.App.AddSessionToCache(session)

	ruser.Id = user.Id
	ruser.Email = th.GenerateTestEmail()
	_, resp = th.Client.UpdateUser(ruser)
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.UpdateUser(user)
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic()
	_, resp = th.Client.UpdateUser(user)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.UpdateUser(user)
		CheckNoError(t, resp)
	})
}

func TestPatchUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.CreateUser()
	th.Client.Login(user.Email, user.Password)

	t.Run("Timezone limit error", func(t *testing.T) {
		patch := &model.UserPatch{}
		patch.Timezone = model.StringMap{}
		patch.Timezone["manualTimezone"] = string(make([]byte, model.USER_TIMEZONE_MAX_RUNES))
		ruser, resp := th.Client.PatchUser(user.Id, patch)
		CheckBadRequestStatus(t, resp)
		require.Equal(t, "model.user.is_valid.timezone_limit.app_error", resp.Error.Id)
		require.Nil(t, ruser)
	})

	patch := &model.UserPatch{}
	patch.Password = model.NewString("testpassword")
	patch.Nickname = model.NewString("Joram Wilander")
	patch.FirstName = model.NewString("Joram")
	patch.LastName = model.NewString("Wilander")
	patch.Position = new(string)
	patch.NotifyProps = model.StringMap{}
	patch.NotifyProps["comment"] = "somethingrandom"
	patch.Timezone = model.StringMap{}
	patch.Timezone["useAutomaticTimezone"] = "true"
	patch.Timezone["automaticTimezone"] = "America/New_York"
	patch.Timezone["manualTimezone"] = ""

	ruser, resp := th.Client.PatchUser(user.Id, patch)
	CheckNoError(t, resp)
	CheckUserSanitization(t, ruser)

	require.Equal(t, "Joram Wilander", ruser.Nickname, "Nickname should update properly")
	require.Equal(t, "Joram", ruser.FirstName, "FirstName should update properly")
	require.Equal(t, "Wilander", ruser.LastName, "LastName should update properly")
	require.Empty(t, ruser.Position, "Position should update properly")
	require.Equal(t, user.Username, ruser.Username, "Username should not update")
	require.Empty(t, ruser.Password, "Password should not be returned")
	require.Equal(t, "somethingrandom", ruser.NotifyProps["comment"], "NotifyProps should update properly")
	require.Equal(t, "true", ruser.Timezone["useAutomaticTimezone"], "useAutomaticTimezone should update properly")
	require.Equal(t, "America/New_York", ruser.Timezone["automaticTimezone"], "automaticTimezone should update properly")
	require.Empty(t, ruser.Timezone["manualTimezone"], "manualTimezone should update properly")

	err := th.App.CheckPasswordAndAllCriteria(ruser, *patch.Password, "")
	require.NotNil(t, err, "Password should not match")

	currentPassword := user.Password
	user, err = th.App.GetUser(ruser.Id)
	require.Nil(t, err)

	err = th.App.CheckPasswordAndAllCriteria(user, currentPassword, "")
	require.Nil(t, err, "Password should still match")

	patch = &model.UserPatch{}
	patch.Email = model.NewString(th.GenerateTestEmail())

	_, resp = th.Client.PatchUser(user.Id, patch)
	CheckBadRequestStatus(t, resp)

	patch.Password = model.NewString(currentPassword)
	ruser, resp = th.Client.PatchUser(user.Id, patch)
	CheckNoError(t, resp)

	require.Equal(t, *patch.Email, ruser.Email, "Email should update properly")

	patch.Username = model.NewString(th.BasicUser2.Username)
	_, resp = th.Client.PatchUser(user.Id, patch)
	CheckBadRequestStatus(t, resp)

	patch.Username = nil

	_, resp = th.Client.PatchUser("junk", patch)
	CheckBadRequestStatus(t, resp)

	ruser.Id = model.NewId()
	_, resp = th.Client.PatchUser(model.NewId(), patch)
	CheckForbiddenStatus(t, resp)

	r, err := th.Client.DoApiPut("/users/"+user.Id+"/patch", "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	session, _ := th.App.GetSession(th.Client.AuthToken)
	session.IsOAuth = true
	th.App.AddSessionToCache(session)

	patch.Email = model.NewString(th.GenerateTestEmail())
	_, resp = th.Client.PatchUser(user.Id, patch)
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.PatchUser(user.Id, patch)
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic()
	_, resp = th.Client.PatchUser(user.Id, patch)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.PatchUser(user.Id, patch)
	CheckNoError(t, resp)
}

func TestUserUnicodeNames(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	t.Run("create user unicode", func(t *testing.T) {
		user := model.User{
			Email:     th.GenerateTestEmail(),
			FirstName: "Andrew\u202e",
			LastName:  "\ufeffWiggin",
			Nickname:  "Ender\u2028 Wiggin",
			Password:  "hello1",
			Username:  "\ufeffwiggin77",
			Roles:     model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

		ruser, resp := Client.CreateUser(&user)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		_, _ = Client.Login(user.Email, user.Password)

		require.Equal(t, "wiggin77", ruser.Username, "Bad Unicode not filtered from username")
		require.Equal(t, "Andrew Wiggin", ruser.GetDisplayName(model.SHOW_FULLNAME), "Bad Unicode not filtered from displayname")
		require.Equal(t, "Ender Wiggin", ruser.Nickname, "Bad Unicode not filtered from nickname")
	})

	t.Run("update user unicode", func(t *testing.T) {
		user := th.CreateUser()
		Client.Login(user.Email, user.Password)

		user.Username = "wiggin\ufff9"
		user.Nickname = "Ender\u0340 \ufffcWiggin"
		user.FirstName = "Andrew\ufff9"
		user.LastName = "Wig\u206fgin"

		ruser, resp := Client.UpdateUser(user)
		CheckNoError(t, resp)

		require.Equal(t, "wiggin", ruser.Username, "bad unicode should be filtered from username")
		require.Equal(t, "Ender Wiggin", ruser.Nickname, "bad unicode should be filtered from nickname")
		require.Equal(t, "Andrew Wiggin", ruser.GetDisplayName(model.SHOW_FULLNAME), "bad unicode should be filtered from display name")
	})

	t.Run("patch user unicode", func(t *testing.T) {
		user := th.CreateUser()
		Client.Login(user.Email, user.Password)

		patch := &model.UserPatch{}
		patch.Nickname = model.NewString("\U000E0000Ender\u206d Wiggin\U000E007F")
		patch.FirstName = model.NewString("\U0001d173Andrew\U0001d17a")
		patch.LastName = model.NewString("\u2028Wiggin\u2029")

		ruser, resp := Client.PatchUser(user.Id, patch)
		CheckNoError(t, resp)
		CheckUserSanitization(t, ruser)

		require.Equal(t, "Ender Wiggin", ruser.Nickname, "Bad unicode should be filtered from nickname")
		require.Equal(t, "Andrew", ruser.FirstName, "Bad unicode should be filtered from first name")
		require.Equal(t, "Wiggin", ruser.LastName, "Bad unicode should be filtered from last name")
		require.Equal(t, "Andrew Wiggin", ruser.GetDisplayName(model.SHOW_FULLNAME), "Bad unicode should be filtered from display name")
	})
}

func TestUpdateUserAuth(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	team := th.CreateTeamWithClient(th.SystemAdminClient)

	user := th.CreateUser()

	th.LinkUserToTeam(user, team)
	_, err := th.App.Srv().Store.User().VerifyEmail(user.Id, user.Email)
	require.NoError(t, err)

	userAuth := &model.UserAuth{}
	userAuth.AuthData = user.AuthData
	userAuth.AuthService = user.AuthService
	userAuth.Password = user.Password

	// Regular user can not use endpoint
	_, respErr := th.SystemAdminClient.UpdateUserAuth(user.Id, userAuth)
	require.NotNil(t, respErr, "Shouldn't have permissions. Only Admins")

	userAuth.AuthData = model.NewString("test@test.com")
	userAuth.AuthService = model.USER_AUTH_SERVICE_SAML
	userAuth.Password = "newpassword"
	ruser, resp := th.SystemAdminClient.UpdateUserAuth(user.Id, userAuth)
	CheckNoError(t, resp)

	// AuthData and AuthService are set, password is set to empty
	require.Equal(t, *userAuth.AuthData, *ruser.AuthData)
	require.Equal(t, model.USER_AUTH_SERVICE_SAML, ruser.AuthService)
	require.Empty(t, ruser.Password)

	// When AuthData or AuthService are empty, password must be valid
	userAuth.AuthData = user.AuthData
	userAuth.AuthService = ""
	userAuth.Password = "1"
	_, respErr = th.SystemAdminClient.UpdateUserAuth(user.Id, userAuth)
	require.NotNil(t, respErr)

	// Regular user can not use endpoint
	user2 := th.CreateUser()
	th.LinkUserToTeam(user2, team)
	_, err = th.App.Srv().Store.User().VerifyEmail(user2.Id, user2.Email)
	require.NoError(t, err)

	th.SystemAdminClient.Login(user2.Email, "passwd1")

	userAuth.AuthData = user.AuthData
	userAuth.AuthService = user.AuthService
	userAuth.Password = user.Password
	_, respErr = th.SystemAdminClient.UpdateUserAuth(user.Id, userAuth)
	require.NotNil(t, respErr, "Should have errored")
}

func TestDeleteUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.LoginBasic()
	_, resp := th.Client.DeleteUser(th.SystemAdminUser.Id)
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.DeleteUser(th.BasicUser.Id)
	CheckUnauthorizedStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		_, resp = c.DeleteUser(model.NewId())
		CheckNotFoundStatus(t, resp)

		_, resp = c.DeleteUser("junk")
		CheckBadRequestStatus(t, resp)

		userToDelete := th.CreateUser()
		_, resp = c.DeleteUser(userToDelete.Id)
		CheckNoError(t, resp)
	})

	selfDeleteUser := th.CreateUser()
	th.LoginBasic()
	_, resp = th.Client.DeleteUser(selfDeleteUser.Id)
	CheckForbiddenStatus(t, resp)

	th.Client.Login(selfDeleteUser.Email, selfDeleteUser.Password)
	th.App.UpdateConfig(func(c *model.Config) {
		*c.TeamSettings.EnableUserDeactivation = false
	})
	_, resp = th.Client.DeleteUser(selfDeleteUser.Id)
	CheckUnauthorizedStatus(t, resp)

	th.App.UpdateConfig(func(c *model.Config) {
		*c.TeamSettings.EnableUserDeactivation = true
	})
	_, resp = th.Client.DeleteUser(selfDeleteUser.Id)
	CheckNoError(t, resp)
}

func TestPermanentDeleteUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	enableAPIUserDeletion := *th.App.Config().ServiceSettings.EnableAPIUserDeletion
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableAPIUserDeletion = &enableAPIUserDeletion })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPIUserDeletion = false })

	userToDelete := th.CreateUser()

	t.Run("Permanent deletion not available through API if EnableAPIUserDeletion is not set", func(t *testing.T) {
		_, resp := th.SystemAdminClient.PermanentDeleteUser(userToDelete.Id)
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("Permanent deletion available through local mode even if EnableAPIUserDeletion is not set", func(t *testing.T) {
		ok, resp := th.LocalClient.PermanentDeleteUser(userToDelete.Id)
		CheckNoError(t, resp)
		assert.True(t, ok)
	})

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableAPIUserDeletion = true })
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		userToDelete = th.CreateUser()
		ok, resp := c.PermanentDeleteUser(userToDelete.Id)
		CheckNoError(t, resp)
		assert.True(t, ok)

		_, err := th.App.GetTeam(userToDelete.Id)
		assert.NotNil(t, err)

		ok, resp = c.PermanentDeleteUser("junk")
		CheckBadRequestStatus(t, resp)
		require.False(t, ok, "should have returned false")
	}, "Permanent deletion with EnableAPIUserDeletion set")
}

func TestPermanentDeleteAllUsers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("The endpoint should not be available for neither normal nor sysadmin users", func(t *testing.T) {
		_, resp := th.Client.PermanentDeleteAllUsers()
		CheckNotFoundStatus(t, resp)

		_, resp = th.SystemAdminClient.PermanentDeleteAllUsers()
		CheckNotFoundStatus(t, resp)
	})

	t.Run("The endpoint should permanently delete all users", func(t *testing.T) {
		// Basic user creates a team and a channel
		team, err := th.App.CreateTeamWithUser(th.Context, &model.Team{
			DisplayName: "User Created Team",
			Name:        "user-created-team",
			Email:       "usercreatedteam@test.com",
			Type:        model.TEAM_OPEN,
		}, th.BasicUser.Id)
		require.Nil(t, err)

		channel, err := th.App.CreateChannelWithUser(th.Context, &model.Channel{
			DisplayName: "User Created Channel",
			Name:        "user-created-channel",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}, th.BasicUser.Id)
		require.Nil(t, err)

		// Check that we have users and posts in the database
		users, nErr := th.App.Srv().Store.User().GetAll()
		require.NoError(t, nErr)
		require.Greater(t, len(users), 0)

		postCount, nErr := th.App.Srv().Store.Post().AnalyticsPostCount("", false, false)
		require.NoError(t, nErr)
		require.Greater(t, postCount, int64(0))

		// Delete all users and their posts
		_, resp := th.LocalClient.PermanentDeleteAllUsers()
		require.Nil(t, resp.Error)

		// Check that both user and post tables are empty
		users, nErr = th.App.Srv().Store.User().GetAll()
		require.NoError(t, nErr)
		require.Len(t, users, 0)

		postCount, nErr = th.App.Srv().Store.Post().AnalyticsPostCount("", false, false)
		require.NoError(t, nErr)
		require.Equal(t, postCount, int64(0))

		// Check that the channel and team created by the user were not deleted
		rTeam, err := th.App.GetTeam(team.Id)
		require.Nil(t, err)
		require.NotNil(t, rTeam)

		rChannel, err := th.App.GetChannel(channel.Id)
		require.Nil(t, err)
		require.NotNil(t, rChannel)
	})
}

func TestUpdateUserRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, resp := th.Client.UpdateUserRoles(th.SystemAdminUser.Id, model.SYSTEM_USER_ROLE_ID)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID)
		CheckNoError(t, resp)

		_, resp = client.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_ADMIN_ROLE_ID)
		CheckNoError(t, resp)

		_, resp = client.UpdateUserRoles(th.BasicUser.Id, "junk")
		CheckBadRequestStatus(t, resp)

		_, resp = client.UpdateUserRoles("junk", model.SYSTEM_USER_ROLE_ID)
		CheckBadRequestStatus(t, resp)

		_, resp = client.UpdateUserRoles(model.NewId(), model.SYSTEM_USER_ROLE_ID)
		CheckBadRequestStatus(t, resp)
	})
}

func assertExpectedWebsocketEvent(t *testing.T, client *model.WebSocketClient, event string, test func(*model.WebSocketEvent)) {
	for {
		select {
		case resp, ok := <-client.EventChannel:
			require.Truef(t, ok, "channel closed before receiving expected event %s", event)
			if resp.EventType() == event {
				test(resp)
				return
			}
		case <-time.After(5 * time.Second):
			require.Failf(t, "failed to receive expected event %s", event)
		}
	}
}

func assertWebsocketEventUserUpdatedWithEmail(t *testing.T, client *model.WebSocketClient, email string) {
	assertExpectedWebsocketEvent(t, client, model.WEBSOCKET_EVENT_USER_UPDATED, func(event *model.WebSocketEvent) {
		eventUser, ok := event.GetData()["user"].(*model.User)
		require.True(t, ok, "expected user")
		assert.Equal(t, email, eventUser.Email)
	})
}

func TestUpdateUserActive(t *testing.T) {
	t.Run("not activating more users when cloud license users at limit", func(t *testing.T) {
		// create 5 active users
		th := Setup(t).InitBasic()
		defer th.TearDown()

		cloudMock := &mocks.CloudInterface{}
		cloudMock.Mock.On(
			"GetSubscription", mock.Anything,
		).Return(&model.Subscription{
			ID:         "MySubscriptionID",
			CustomerID: "MyCustomer",
			ProductID:  "SomeProductId",
			AddOns:     []string{},
			StartAt:    1000000000,
			EndAt:      2000000000,
			CreateAt:   1000000000,
			Seats:      100,
			DNS:        "some.dns.server",
			IsPaidTier: "false",
		}, nil)

		th.App.Srv().SetLicense(model.NewTestLicense("cloud"))
		th.App.Srv().Cloud = cloudMock

		user := th.BasicUser

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.EnableUserDeactivation = true
			*cfg.ExperimentalSettings.CloudUserLimit = 4
		})

		// deactivate 5th user, now we have 4 active users and are at limit
		pass, resp := th.SystemAdminClient.UpdateUserActive(user.Id, false)
		CheckNoError(t, resp)
		require.True(t, pass)

		// try and reactivate 5th user, not allowed because it exceeds the set cloud user limit
		pass, resp = th.SystemAdminClient.UpdateUserActive(user.Id, true)
		CheckBadRequestStatus(t, resp)
		require.False(t, pass)
		require.Equal(t, resp.Error.Message, "Unable to activate more users as the cloud account is over capacity.")
	})
	t.Run("basic tests", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user := th.BasicUser

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserDeactivation = true })
		pass, resp := th.Client.UpdateUserActive(user.Id, false)
		CheckNoError(t, resp)

		require.True(t, pass)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserDeactivation = false })
		pass, resp = th.Client.UpdateUserActive(user.Id, false)
		CheckUnauthorizedStatus(t, resp)

		require.False(t, pass)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserDeactivation = true })
		pass, resp = th.Client.UpdateUserActive(user.Id, false)
		CheckUnauthorizedStatus(t, resp)

		require.False(t, pass)

		th.LoginBasic2()

		_, resp = th.Client.UpdateUserActive(user.Id, true)
		CheckForbiddenStatus(t, resp)

		_, resp = th.Client.UpdateUserActive(GenerateTestId(), true)
		CheckForbiddenStatus(t, resp)

		_, resp = th.Client.UpdateUserActive("junk", true)
		CheckBadRequestStatus(t, resp)

		th.Client.Logout()

		_, resp = th.Client.UpdateUserActive(user.Id, true)
		CheckUnauthorizedStatus(t, resp)

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.UpdateUserActive(user.Id, true)
			CheckNoError(t, resp)

			_, resp = client.UpdateUserActive(user.Id, false)
			CheckNoError(t, resp)

			authData := model.NewId()
			_, err := th.App.Srv().Store.User().UpdateAuthData(user.Id, "random", &authData, "", true)
			require.NoError(t, err)

			_, resp = client.UpdateUserActive(user.Id, false)
			CheckNoError(t, resp)
		})
	})

	t.Run("websocket events", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		user := th.BasicUser2

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableUserDeactivation = true })

		webSocketClient, err := th.CreateWebSocketClient()
		assert.Nil(t, err)
		defer webSocketClient.Close()

		webSocketClient.Listen()

		time.Sleep(300 * time.Millisecond)
		resp := <-webSocketClient.ResponseChannel
		require.Equal(t, model.STATUS_OK, resp.Status)

		adminWebSocketClient, err := th.CreateWebSocketSystemAdminClient()
		assert.Nil(t, err)
		defer adminWebSocketClient.Close()

		adminWebSocketClient.Listen()

		time.Sleep(300 * time.Millisecond)
		resp = <-adminWebSocketClient.ResponseChannel
		require.Equal(t, model.STATUS_OK, resp.Status)

		// Verify that both admins and regular users see the email when privacy settings allow same,
		// and confirm event is fired for SystemAdmin and Local mode
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowEmailAddress = true })
		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			_, respErr := client.UpdateUserActive(user.Id, false)
			CheckNoError(t, respErr)

			assertWebsocketEventUserUpdatedWithEmail(t, webSocketClient, user.Email)
			assertWebsocketEventUserUpdatedWithEmail(t, adminWebSocketClient, user.Email)
		})

		// Verify that only admins see the email when privacy settings hide emails,
		// and confirm event is fired for SystemAdmin and Local mode
		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowEmailAddress = false })
			_, respErr := client.UpdateUserActive(user.Id, true)
			CheckNoError(t, respErr)

			assertWebsocketEventUserUpdatedWithEmail(t, webSocketClient, "")
			assertWebsocketEventUserUpdatedWithEmail(t, adminWebSocketClient, user.Email)
		})
	})

	t.Run("activate guest should fail when guests feature is disable", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		id := model.NewId()
		guest := &model.User{
			Email:         "success+" + id + "@simulator.amazonses.com",
			Username:      "un_" + id,
			Nickname:      "nn_" + id,
			Password:      "Password1",
			EmailVerified: true,
		}
		user, err := th.App.CreateGuest(th.Context, guest)
		require.Nil(t, err)
		th.App.UpdateActive(th.Context, user, false)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.UpdateUserActive(user.Id, true)
			CheckUnauthorizedStatus(t, resp)
		})
	})

	t.Run("activate guest should work when guests feature is enabled", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		id := model.NewId()
		guest := &model.User{
			Email:         "success+" + id + "@simulator.amazonses.com",
			Username:      "un_" + id,
			Nickname:      "nn_" + id,
			Password:      "Password1",
			EmailVerified: true,
		}
		user, err := th.App.CreateGuest(th.Context, guest)
		require.Nil(t, err)
		th.App.UpdateActive(th.Context, user, false)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.UpdateUserActive(user.Id, true)
			CheckNoError(t, resp)
		})
	})
}

func TestGetUsers(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		rusers, resp := client.GetUsers(0, 60, "")
		CheckNoError(t, resp)
		for _, u := range rusers {
			CheckUserSanitization(t, u)
		}

		rusers, resp = client.GetUsers(0, 1, "")
		CheckNoError(t, resp)
		require.Len(t, rusers, 1, "should be 1 per page")

		rusers, resp = client.GetUsers(1, 1, "")
		CheckNoError(t, resp)
		require.Len(t, rusers, 1, "should be 1 per page")

		rusers, resp = client.GetUsers(10000, 100, "")
		CheckNoError(t, resp)
		require.Empty(t, rusers, "should be no users")

		// Check default params for page and per_page
		_, err := client.DoApiGet("/users", "")
		require.Nil(t, err)
	})

	th.Client.Logout()
	_, resp := th.Client.GetUsers(0, 60, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestGetNewUsersInTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	teamId := th.BasicTeam.Id

	rusers, resp := th.Client.GetNewUsersInTeam(teamId, 0, 60, "")
	CheckNoError(t, resp)

	lastCreateAt := model.GetMillis()
	for _, u := range rusers {
		require.LessOrEqual(t, u.CreateAt, lastCreateAt, "right sorting")
		lastCreateAt = u.CreateAt
		CheckUserSanitization(t, u)
	}

	rusers, resp = th.Client.GetNewUsersInTeam(teamId, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	th.Client.Logout()
	_, resp = th.Client.GetNewUsersInTeam(teamId, 1, 1, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestGetRecentlyActiveUsersInTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	teamId := th.BasicTeam.Id

	th.App.SetStatusOnline(th.BasicUser.Id, true)

	rusers, resp := th.Client.GetRecentlyActiveUsersInTeam(teamId, 0, 60, "")
	CheckNoError(t, resp)

	for _, u := range rusers {
		require.NotZero(t, u.LastActivityAt, "should return last activity at")
		CheckUserSanitization(t, u)
	}

	rusers, resp = th.Client.GetRecentlyActiveUsersInTeam(teamId, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	th.Client.Logout()
	_, resp = th.Client.GetRecentlyActiveUsersInTeam(teamId, 0, 1, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestGetActiveUsersInTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	teamId := th.BasicTeam.Id

	th.SystemAdminClient.UpdateUserActive(th.BasicUser2.Id, false)
	rusers, resp := th.Client.GetActiveUsersInTeam(teamId, 0, 60, "")
	CheckNoError(t, resp)

	require.NotZero(t, len(rusers))
	for _, u := range rusers {
		require.Zero(t, u.DeleteAt, "should not be deleted")
		require.NotEqual(t, th.BasicUser2.Id, "should not include deactivated user")
		CheckUserSanitization(t, u)
	}

	rusers, resp = th.Client.GetActiveUsersInTeam(teamId, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	// Check case where we have supplied both active and inactive flags
	_, err := th.Client.DoApiGet("/users?inactive=true&active=true", "")
	require.NotNil(t, err)

	th.Client.Logout()
	_, resp = th.Client.GetActiveUsersInTeam(teamId, 0, 1, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestGetUsersWithoutTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, resp := th.Client.GetUsersWithoutTeam(0, 100, "")
	require.NotNil(t, resp.Error, "should prevent non-admin user from getting users without a team")

	// These usernames need to appear in the first 100 users for this to work

	user, resp := th.Client.CreateUser(&model.User{
		Username: "a000000000" + model.NewId(),
		Email:    "success+" + model.NewId() + "@simulator.amazonses.com",
		Password: "Password1",
	})
	CheckNoError(t, resp)
	th.LinkUserToTeam(user, th.BasicTeam)
	defer th.App.Srv().Store.User().PermanentDelete(user.Id)

	user2, resp := th.Client.CreateUser(&model.User{
		Username: "a000000001" + model.NewId(),
		Email:    "success+" + model.NewId() + "@simulator.amazonses.com",
		Password: "Password1",
	})
	CheckNoError(t, resp)
	defer th.App.Srv().Store.User().PermanentDelete(user2.Id)

	rusers, resp := th.SystemAdminClient.GetUsersWithoutTeam(0, 100, "")
	CheckNoError(t, resp)

	found1 := false
	found2 := false

	for _, u := range rusers {
		if u.Id == user.Id {
			found1 = true
		} else if u.Id == user2.Id {
			found2 = true
		}
	}

	require.False(t, found1, "should not return user that as a team")
	require.True(t, found2, "should return user that has no teams")
}

func TestGetUsersInTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	teamId := th.BasicTeam.Id

	rusers, resp := th.Client.GetUsersInTeam(teamId, 0, 60, "")
	CheckNoError(t, resp)
	for _, u := range rusers {
		CheckUserSanitization(t, u)
	}

	rusers, resp = th.Client.GetUsersInTeam(teamId, 0, 60, resp.Etag)
	CheckEtag(t, rusers, resp)

	rusers, resp = th.Client.GetUsersInTeam(teamId, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	rusers, resp = th.Client.GetUsersInTeam(teamId, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	rusers, resp = th.Client.GetUsersInTeam(teamId, 10000, 100, "")
	CheckNoError(t, resp)
	require.Empty(t, rusers, "should be no users")

	th.Client.Logout()
	_, resp = th.Client.GetUsersInTeam(teamId, 0, 60, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	th.Client.Login(user.Email, user.Password)
	_, resp = th.Client.GetUsersInTeam(teamId, 0, 60, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetUsersInTeam(teamId, 0, 60, "")
	CheckNoError(t, resp)
}

func TestGetUsersNotInTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	teamId := th.BasicTeam.Id

	rusers, resp := th.Client.GetUsersNotInTeam(teamId, 0, 60, "")
	CheckNoError(t, resp)
	for _, u := range rusers {
		CheckUserSanitization(t, u)
	}
	require.Len(t, rusers, 2, "should be 2 users in total")

	rusers, resp = th.Client.GetUsersNotInTeam(teamId, 0, 60, resp.Etag)
	CheckEtag(t, rusers, resp)

	rusers, resp = th.Client.GetUsersNotInTeam(teamId, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	rusers, resp = th.Client.GetUsersNotInTeam(teamId, 2, 1, "")
	CheckNoError(t, resp)
	require.Empty(t, rusers, "should be no users")

	rusers, resp = th.Client.GetUsersNotInTeam(teamId, 10000, 100, "")
	CheckNoError(t, resp)
	require.Empty(t, rusers, "should be no users")

	th.Client.Logout()
	_, resp = th.Client.GetUsersNotInTeam(teamId, 0, 60, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	th.Client.Login(user.Email, user.Password)
	_, resp = th.Client.GetUsersNotInTeam(teamId, 0, 60, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetUsersNotInTeam(teamId, 0, 60, "")
	CheckNoError(t, resp)
}

func TestGetUsersInChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	channelId := th.BasicChannel.Id

	rusers, resp := th.Client.GetUsersInChannel(channelId, 0, 60, "")
	CheckNoError(t, resp)
	for _, u := range rusers {
		CheckUserSanitization(t, u)
	}

	rusers, resp = th.Client.GetUsersInChannel(channelId, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	rusers, resp = th.Client.GetUsersInChannel(channelId, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	rusers, resp = th.Client.GetUsersInChannel(channelId, 10000, 100, "")
	CheckNoError(t, resp)
	require.Empty(t, rusers, "should be no users")

	th.Client.Logout()
	_, resp = th.Client.GetUsersInChannel(channelId, 0, 60, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	th.Client.Login(user.Email, user.Password)
	_, resp = th.Client.GetUsersInChannel(channelId, 0, 60, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetUsersInChannel(channelId, 0, 60, "")
	CheckNoError(t, resp)
}

func TestGetUsersNotInChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	teamId := th.BasicTeam.Id
	channelId := th.BasicChannel.Id

	user := th.CreateUser()
	th.LinkUserToTeam(user, th.BasicTeam)

	rusers, resp := th.Client.GetUsersNotInChannel(teamId, channelId, 0, 60, "")
	CheckNoError(t, resp)
	for _, u := range rusers {
		CheckUserSanitization(t, u)
	}

	rusers, resp = th.Client.GetUsersNotInChannel(teamId, channelId, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, rusers, 1, "should be 1 per page")

	rusers, resp = th.Client.GetUsersNotInChannel(teamId, channelId, 10000, 100, "")
	CheckNoError(t, resp)
	require.Empty(t, rusers, "should be no users")

	th.Client.Logout()
	_, resp = th.Client.GetUsersNotInChannel(teamId, channelId, 0, 60, "")
	CheckUnauthorizedStatus(t, resp)

	th.Client.Login(user.Email, user.Password)
	_, resp = th.Client.GetUsersNotInChannel(teamId, channelId, 0, 60, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetUsersNotInChannel(teamId, channelId, 0, 60, "")
	CheckNoError(t, resp)
}

func TestGetUsersInGroup(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewId()
	group, err := th.App.CreateGroup(&model.Group{
		DisplayName: "dn-foo_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		Description: "description_" + id,
		RemoteId:    model.NewId(),
	})
	assert.Nil(t, err)

	var response *model.Response
	var users []*model.User

	t.Run("Requires ldap license", func(t *testing.T) {
		_, response = th.SystemAdminClient.GetUsersInGroup(group.Id, 0, 60, "")
		CheckNotImplementedStatus(t, response)
	})

	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))

	t.Run("Requires manage system permission to access users in group", func(t *testing.T) {
		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
		_, response = th.Client.GetUsersInGroup(group.Id, 0, 60, "")
		CheckForbiddenStatus(t, response)
	})

	user1, err := th.App.CreateUser(th.Context, &model.User{Email: th.GenerateTestEmail(), Nickname: "test user1", Password: "test-password-1", Username: "test-user-1", Roles: model.SYSTEM_USER_ROLE_ID})
	assert.Nil(t, err)
	_, err = th.App.UpsertGroupMember(group.Id, user1.Id)
	assert.Nil(t, err)

	t.Run("Returns users in group when called by system admin", func(t *testing.T) {
		users, response = th.SystemAdminClient.GetUsersInGroup(group.Id, 0, 60, "")
		CheckNoError(t, response)
		assert.Equal(t, users[0].Id, user1.Id)
	})

	t.Run("Returns no users when pagination out of range", func(t *testing.T) {
		users, response = th.SystemAdminClient.GetUsersInGroup(group.Id, 5, 60, "")
		CheckNoError(t, response)
		assert.Empty(t, users)
	})
}

func TestUpdateUserMfa(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().SetLicense(model.NewTestLicense("mfa"))
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableMultifactorAuthentication = true })

	session, _ := th.App.GetSession(th.Client.AuthToken)
	session.IsOAuth = true
	th.App.AddSessionToCache(session)

	_, resp := th.Client.UpdateUserMfa(th.BasicUser.Id, "12345", false)
	CheckForbiddenStatus(t, resp)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.UpdateUserMfa(th.BasicUser.Id, "12345", false)
		CheckNoError(t, resp)
	})
}

// CheckUserMfa is deprecated and should not be used anymore, it will be disabled by default in version 6.0
func TestCheckUserMfa(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(c *model.Config) {
		*c.ServiceSettings.DisableLegacyMFA = false
	})

	required, resp := th.Client.CheckUserMfa(th.BasicUser.Email)
	CheckNoError(t, resp)

	require.False(t, required, "mfa not active")

	_, resp = th.Client.CheckUserMfa("")
	CheckBadRequestStatus(t, resp)

	th.Client.Logout()

	required, resp = th.Client.CheckUserMfa(th.BasicUser.Email)
	CheckNoError(t, resp)

	require.False(t, required, "mfa not active")

	th.App.Srv().SetLicense(model.NewTestLicense("mfa"))
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableMultifactorAuthentication = true })

	th.LoginBasic()

	required, resp = th.Client.CheckUserMfa(th.BasicUser.Email)
	CheckNoError(t, resp)

	require.False(t, required, "mfa not active")

	th.Client.Logout()

	required, resp = th.Client.CheckUserMfa(th.BasicUser.Email)
	CheckNoError(t, resp)

	require.False(t, required, "mfa not active")

	th.App.UpdateConfig(func(c *model.Config) {
		*c.ServiceSettings.DisableLegacyMFA = true
	})

	_, resp = th.Client.CheckUserMfa(th.BasicUser.Email)
	CheckNotFoundStatus(t, resp)
}

func TestUserLoginMFAFlow(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(c *model.Config) {
		*c.ServiceSettings.DisableLegacyMFA = true
		*c.ServiceSettings.EnableMultifactorAuthentication = true
	})

	t.Run("WithoutMFA", func(t *testing.T) {
		_, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
		CheckNoError(t, resp)
	})

	t.Run("WithInvalidMFA", func(t *testing.T) {
		secret, err := th.App.GenerateMfaSecret(th.BasicUser.Id)
		assert.Nil(t, err)

		// Fake user has MFA enabled
		nErr := th.Server.Store.User().UpdateMfaActive(th.BasicUser.Id, true)
		require.NoError(t, nErr)

		nErr = th.Server.Store.User().UpdateMfaActive(th.BasicUser.Id, true)
		require.NoError(t, nErr)

		nErr = th.Server.Store.User().UpdateMfaSecret(th.BasicUser.Id, secret.Secret)
		require.NoError(t, nErr)

		user, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
		CheckErrorMessage(t, resp, "mfa.validate_token.authenticate.app_error")
		assert.Nil(t, user)

		user, resp = th.Client.LoginWithMFA(th.BasicUser.Email, th.BasicUser.Password, "")
		CheckErrorMessage(t, resp, "mfa.validate_token.authenticate.app_error")
		assert.Nil(t, user)

		user, resp = th.Client.LoginWithMFA(th.BasicUser.Email, th.BasicUser.Password, "abcdefgh")
		CheckErrorMessage(t, resp, "mfa.validate_token.authenticate.app_error")
		assert.Nil(t, user)

		secret2, err := th.App.GenerateMfaSecret(th.BasicUser2.Id)
		assert.Nil(t, err)
		user, resp = th.Client.LoginWithMFA(th.BasicUser.Email, th.BasicUser.Password, secret2.Secret)
		CheckErrorMessage(t, resp, "mfa.validate_token.authenticate.app_error")
		assert.Nil(t, user)
	})

	t.Run("WithCorrectMFA", func(t *testing.T) {
		secret, err := th.App.GenerateMfaSecret(th.BasicUser.Id)
		assert.Nil(t, err)

		// Fake user has MFA enabled
		nErr := th.Server.Store.User().UpdateMfaActive(th.BasicUser.Id, true)
		require.NoError(t, nErr)

		nErr = th.Server.Store.User().UpdateMfaSecret(th.BasicUser.Id, secret.Secret)
		require.NoError(t, nErr)

		code := dgoogauth.ComputeCode(secret.Secret, time.Now().UTC().Unix()/30)

		user, resp := th.Client.LoginWithMFA(th.BasicUser.Email, th.BasicUser.Password, fmt.Sprintf("%06d", code))
		CheckNoError(t, resp)
		assert.NotNil(t, user)
	})
}

func TestGenerateMfaSecret(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableMultifactorAuthentication = false })

	_, resp := th.Client.GenerateMfaSecret(th.BasicUser.Id)
	CheckNotImplementedStatus(t, resp)

	_, resp = th.SystemAdminClient.GenerateMfaSecret(th.BasicUser.Id)
	CheckNotImplementedStatus(t, resp)

	_, resp = th.Client.GenerateMfaSecret("junk")
	CheckBadRequestStatus(t, resp)

	th.App.Srv().SetLicense(model.NewTestLicense("mfa"))
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableMultifactorAuthentication = true })

	_, resp = th.Client.GenerateMfaSecret(model.NewId())
	CheckForbiddenStatus(t, resp)

	session, _ := th.App.GetSession(th.Client.AuthToken)
	session.IsOAuth = true
	th.App.AddSessionToCache(session)

	_, resp = th.Client.GenerateMfaSecret(th.BasicUser.Id)
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()

	_, resp = th.Client.GenerateMfaSecret(th.BasicUser.Id)
	CheckUnauthorizedStatus(t, resp)
}

func TestUpdateUserPassword(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	password := "newpassword1"
	pass, resp := th.Client.UpdateUserPassword(th.BasicUser.Id, th.BasicUser.Password, password)
	CheckNoError(t, resp)

	require.True(t, pass)

	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, password, "")
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, password, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.UpdateUserPassword("junk", password, password)
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, "", password)
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, "junk", password)
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, password, th.BasicUser.Password)
	CheckNoError(t, resp)

	th.Client.Logout()
	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, password, password)
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic2()
	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, password, password)
	CheckForbiddenStatus(t, resp)

	th.LoginBasic()

	// Test lockout
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.MaximumLoginAttempts = 2 })

	// Fail twice
	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, "badpwd", "newpwd")
	CheckBadRequestStatus(t, resp)
	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, "badpwd", "newpwd")
	CheckBadRequestStatus(t, resp)

	// Should fail because account is locked out
	_, resp = th.Client.UpdateUserPassword(th.BasicUser.Id, th.BasicUser.Password, "newpwd")
	CheckErrorMessage(t, resp, "api.user.check_user_login_attempts.too_many.app_error")
	CheckUnauthorizedStatus(t, resp)

	// System admin can update another user's password
	adminSetPassword := "pwdsetbyadmin"
	pass, resp = th.SystemAdminClient.UpdateUserPassword(th.BasicUser.Id, "", adminSetPassword)
	CheckNoError(t, resp)

	require.True(t, pass)

	_, resp = th.Client.Login(th.BasicUser.Email, adminSetPassword)
	CheckNoError(t, resp)
}

func TestUpdateUserHashedPassword(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	client := th.Client

	password := "SuperSecurePass23!"
	passwordHash := "$2a$10$CiS1iWVPUj7rQNdY6XW53.DmaPLsETIvmW2p0asp4Dqpofs10UL5W"
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		pass, resp := client.UpdateUserHashedPassword(th.BasicUser.Id, passwordHash)
		CheckNoError(t, resp)
		require.True(t, pass)
	})

	_, resp := client.Login(th.BasicUser.Email, password)
	CheckNoError(t, resp)

	// Standard users should never be updating their passwords with already-
	// hashed passwords.
	pass, resp := client.UpdateUserHashedPassword(th.BasicUser.Id, passwordHash)
	CheckUnauthorizedStatus(t, resp)
	require.False(t, pass)
}

func TestResetPassword(t *testing.T) {
	t.Skip("test disabled during old build server changes, should be investigated")

	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.Client.Logout()
	user := th.BasicUser
	// Delete all the messages before check the reset password
	mail.DeleteMailBox(user.Email)
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		success, resp := client.SendPasswordResetEmail(user.Email)
		CheckNoError(t, resp)
		require.True(t, success, "should succeed")
		_, resp = client.SendPasswordResetEmail("")
		CheckBadRequestStatus(t, resp)
		// Should not leak whether the email is attached to an account or not
		success, resp = client.SendPasswordResetEmail("notreal@example.com")
		CheckNoError(t, resp)
		require.True(t, success, "should succeed")
	})
	// Check if the email was send to the right email address and the recovery key match
	var resultsMailbox mail.JSONMessageHeaderInbucket
	err := mail.RetryInbucket(5, func() error {
		var err error
		resultsMailbox, err = mail.GetMailBox(user.Email)
		return err
	})
	if err != nil {
		t.Log(err)
		t.Log("No email was received, maybe due load on the server. Disabling this verification")
	}
	var recoveryTokenString string
	if err == nil && len(resultsMailbox) > 0 {
		require.Contains(t, resultsMailbox[0].To[0], user.Email, "Correct To recipient")
		resultsEmail, mailErr := mail.GetMessageFromMailbox(user.Email, resultsMailbox[0].ID)
		require.NoError(t, mailErr)
		loc := strings.Index(resultsEmail.Body.Text, "token=")
		require.NotEqual(t, -1, loc, "Code should be found in email")
		loc += 6
		recoveryTokenString = resultsEmail.Body.Text[loc : loc+model.TOKEN_SIZE]
	}
	recoveryToken, err := th.App.Srv().Store.Token().GetByToken(recoveryTokenString)
	require.NoError(t, err, "Recovery token not found (%s)", recoveryTokenString)

	_, resp := th.Client.ResetPassword(recoveryToken.Token, "")
	CheckBadRequestStatus(t, resp)
	_, resp = th.Client.ResetPassword(recoveryToken.Token, "newp")
	CheckBadRequestStatus(t, resp)
	_, resp = th.Client.ResetPassword("", "newpwd")
	CheckBadRequestStatus(t, resp)
	_, resp = th.Client.ResetPassword("junk", "newpwd")
	CheckBadRequestStatus(t, resp)
	code := ""
	for i := 0; i < model.TOKEN_SIZE; i++ {
		code += "a"
	}
	_, resp = th.Client.ResetPassword(code, "newpwd")
	CheckBadRequestStatus(t, resp)
	success, resp := th.Client.ResetPassword(recoveryToken.Token, "newpwd")
	CheckNoError(t, resp)
	require.True(t, success)
	th.Client.Login(user.Email, "newpwd")
	th.Client.Logout()
	_, resp = th.Client.ResetPassword(recoveryToken.Token, "newpwd")
	CheckBadRequestStatus(t, resp)
	authData := model.NewId()
	_, err = th.App.Srv().Store.User().UpdateAuthData(user.Id, "random", &authData, "", true)
	require.NoError(t, err)
	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		_, resp = client.SendPasswordResetEmail(user.Email)
		CheckBadRequestStatus(t, resp)
	})
}

func TestGetSessions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser

	th.Client.Login(user.Email, user.Password)

	sessions, resp := th.Client.GetSessions(user.Id, "")
	for _, session := range sessions {
		require.Equal(t, user.Id, session.UserId, "user id should match session user id")
	}
	CheckNoError(t, resp)

	_, resp = th.Client.RevokeSession("junk", model.NewId())
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.GetSessions(th.BasicUser2.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.Client.GetSessions(model.NewId(), "")
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.GetSessions(th.BasicUser2.Id, "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetSessions(user.Id, "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetSessions(th.BasicUser2.Id, "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetSessions(model.NewId(), "")
	CheckNoError(t, resp)
}

func TestRevokeSessions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	th.Client.Login(user.Email, user.Password)
	sessions, _ := th.Client.GetSessions(user.Id, "")
	require.NotZero(t, len(sessions), "sessions should exist")
	for _, session := range sessions {
		require.Equal(t, user.Id, session.UserId, "user id does not match session user id")
	}
	session := sessions[0]

	_, resp := th.Client.RevokeSession(user.Id, model.NewId())
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.RevokeSession(th.BasicUser2.Id, model.NewId())
	CheckForbiddenStatus(t, resp)

	_, resp = th.Client.RevokeSession("junk", model.NewId())
	CheckBadRequestStatus(t, resp)

	status, resp := th.Client.RevokeSession(user.Id, session.Id)
	require.True(t, status, "user session revoke successfully")
	CheckNoError(t, resp)

	th.LoginBasic()

	sessions, _ = th.App.GetSessions(th.SystemAdminUser.Id)
	session = sessions[0]

	_, resp = th.Client.RevokeSession(user.Id, session.Id)
	CheckBadRequestStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.RevokeSession(user.Id, model.NewId())
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.RevokeSession(user.Id, model.NewId())
	CheckBadRequestStatus(t, resp)

	sessions, _ = th.SystemAdminClient.GetSessions(th.SystemAdminUser.Id, "")
	require.NotEmpty(t, sessions, "sessions should exist")
	for _, session := range sessions {
		require.Equal(t, th.SystemAdminUser.Id, session.UserId, "user id should match session user id")
	}
	session = sessions[0]

	_, resp = th.SystemAdminClient.RevokeSession(th.SystemAdminUser.Id, session.Id)
	CheckNoError(t, resp)
}

func TestRevokeAllSessions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	th.Client.Login(user.Email, user.Password)

	_, resp := th.Client.RevokeAllSessions(th.BasicUser2.Id)
	CheckForbiddenStatus(t, resp)

	_, resp = th.Client.RevokeAllSessions("junk" + user.Id)
	CheckBadRequestStatus(t, resp)

	status, resp := th.Client.RevokeAllSessions(user.Id)
	require.True(t, status, "user all sessions revoke unsuccessful")
	CheckNoError(t, resp)

	th.Client.Logout()
	_, resp = th.Client.RevokeAllSessions(user.Id)
	CheckUnauthorizedStatus(t, resp)

	th.Client.Login(user.Email, user.Password)

	sessions, _ := th.Client.GetSessions(user.Id, "")
	require.NotEmpty(t, sessions, "session should exist")

	_, resp = th.Client.RevokeAllSessions(user.Id)
	CheckNoError(t, resp)

	sessions, _ = th.SystemAdminClient.GetSessions(user.Id, "")
	require.Empty(t, sessions, "no sessions should exist for user")

	_, resp = th.Client.RevokeAllSessions(user.Id)
	CheckUnauthorizedStatus(t, resp)
}

func TestRevokeSessionsFromAllUsers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	th.Client.Login(user.Email, user.Password)
	_, resp := th.Client.RevokeSessionsFromAllUsers()
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.RevokeSessionsFromAllUsers()
	CheckUnauthorizedStatus(t, resp)

	th.Client.Login(user.Email, user.Password)
	admin := th.SystemAdminUser
	th.Client.Login(admin.Email, admin.Password)
	sessions, err := th.Server.Store.Session().GetSessions(user.Id)
	require.NotEmpty(t, sessions)
	require.NoError(t, err)
	sessions, err = th.Server.Store.Session().GetSessions(admin.Id)
	require.NotEmpty(t, sessions)
	require.NoError(t, err)
	_, resp = th.Client.RevokeSessionsFromAllUsers()
	CheckNoError(t, resp)

	// All sessions were revoked, so making the same call
	// again will fail due to lack of a session.
	_, resp = th.Client.RevokeSessionsFromAllUsers()
	CheckUnauthorizedStatus(t, resp)

	sessions, err = th.Server.Store.Session().GetSessions(user.Id)
	require.Empty(t, sessions)
	require.NoError(t, err)

	sessions, err = th.Server.Store.Session().GetSessions(admin.Id)
	require.Empty(t, sessions)
	require.NoError(t, err)

}

func TestAttachDeviceId(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	deviceId := model.PUSH_NOTIFY_APPLE + ":1234567890"

	t.Run("success", func(t *testing.T) {
		testCases := []struct {
			Description                   string
			SiteURL                       string
			ExpectedSetCookieHeaderRegexp string
		}{
			{"no subpath", "http://localhost:8065", "^MMAUTHTOKEN=[a-z0-9]+; Path=/"},
			{"subpath", "http://localhost:8065/subpath", "^MMAUTHTOKEN=[a-z0-9]+; Path=/subpath"},
		}

		for _, tc := range testCases {
			t.Run(tc.Description, func(t *testing.T) {

				th.App.UpdateConfig(func(cfg *model.Config) {
					*cfg.ServiceSettings.SiteURL = tc.SiteURL
				})

				pass, resp := th.Client.AttachDeviceId(deviceId)
				CheckNoError(t, resp)

				cookies := resp.Header.Get("Set-Cookie")
				assert.Regexp(t, tc.ExpectedSetCookieHeaderRegexp, cookies)
				assert.True(t, pass)

				sessions, err := th.App.GetSessions(th.BasicUser.Id)
				require.Nil(t, err)
				assert.Equal(t, deviceId, sessions[0].DeviceId, "Missing device Id")
			})
		}
	})

	t.Run("invalid device id", func(t *testing.T) {
		_, resp := th.Client.AttachDeviceId("")
		CheckBadRequestStatus(t, resp)
	})

	t.Run("not logged in", func(t *testing.T) {
		th.Client.Logout()

		_, resp := th.Client.AttachDeviceId("")
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestGetUserAudits(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.BasicUser

	audits, resp := th.Client.GetUserAudits(user.Id, 0, 100, "")
	for _, audit := range audits {
		require.Equal(t, user.Id, audit.UserId, "user id should match audit user id")
	}
	CheckNoError(t, resp)

	_, resp = th.Client.GetUserAudits(th.BasicUser2.Id, 0, 100, "")
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.GetUserAudits(user.Id, 0, 100, "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetUserAudits(user.Id, 0, 100, "")
	CheckNoError(t, resp)
}

func TestVerifyUserEmail(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	email := th.GenerateTestEmail()
	user := model.User{Email: email, Nickname: "Darth Vader", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_ADMIN_ROLE_ID + " " + model.SYSTEM_USER_ROLE_ID}

	ruser, _ := th.Client.CreateUser(&user)

	token, err := th.App.Srv().EmailService.CreateVerifyEmailToken(ruser.Id, email)
	require.Nil(t, err, "Unable to create email verify token")

	_, resp := th.Client.VerifyUserEmail(token.Token)
	CheckNoError(t, resp)

	_, resp = th.Client.VerifyUserEmail(GenerateTestId())
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.VerifyUserEmail("")
	CheckBadRequestStatus(t, resp)
}

func TestSendVerificationEmail(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	pass, resp := th.Client.SendVerificationEmail(th.BasicUser.Email)
	CheckNoError(t, resp)

	require.True(t, pass, "should have passed")

	_, resp = th.Client.SendVerificationEmail("")
	CheckBadRequestStatus(t, resp)

	// Even non-existent emails should return 200 OK
	_, resp = th.Client.SendVerificationEmail(th.GenerateTestEmail())
	CheckNoError(t, resp)

	th.Client.Logout()
	_, resp = th.Client.SendVerificationEmail(th.BasicUser.Email)
	CheckNoError(t, resp)
}

func TestSetProfileImage(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.BasicUser

	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	ok, resp := th.Client.SetProfileImage(user.Id, data)
	require.Truef(t, ok, "%v", resp.Error)
	CheckNoError(t, resp)

	ok, resp = th.Client.SetProfileImage(model.NewId(), data)
	require.False(t, ok, "Should return false, set profile image not allowed")
	CheckForbiddenStatus(t, resp)

	// status code returns either forbidden or unauthorized
	// note: forbidden is set as default at Client4.SetProfileImage when request is terminated early by server
	th.Client.Logout()
	_, resp = th.Client.SetProfileImage(user.Id, data)
	if resp.StatusCode == http.StatusForbidden {
		CheckForbiddenStatus(t, resp)
	} else if resp.StatusCode == http.StatusUnauthorized {
		CheckUnauthorizedStatus(t, resp)
	} else {
		require.Fail(t, "Should have failed either forbidden or unauthorized")
	}

	buser, appErr := th.App.GetUser(user.Id)
	require.Nil(t, appErr)

	_, resp = th.SystemAdminClient.SetProfileImage(user.Id, data)
	CheckNoError(t, resp)

	ruser, appErr := th.App.GetUser(user.Id)
	require.Nil(t, appErr)
	assert.True(t, buser.LastPictureUpdate < ruser.LastPictureUpdate, "Picture should have updated for user")

	info := &model.FileInfo{Path: "users/" + user.Id + "/profile.png"}
	err = th.cleanupTestFile(info)
	require.NoError(t, err)
}

func TestSetDefaultProfileImage(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.BasicUser

	ok, resp := th.Client.SetDefaultProfileImage(user.Id)
	require.True(t, ok)
	CheckNoError(t, resp)

	ok, resp = th.Client.SetDefaultProfileImage(model.NewId())
	require.False(t, ok, "Should return false, set profile image not allowed")
	CheckForbiddenStatus(t, resp)

	// status code returns either forbidden or unauthorized
	// note: forbidden is set as default at Client4.SetDefaultProfileImage when request is terminated early by server
	th.Client.Logout()
	_, resp = th.Client.SetDefaultProfileImage(user.Id)
	if resp.StatusCode == http.StatusForbidden {
		CheckForbiddenStatus(t, resp)
	} else if resp.StatusCode == http.StatusUnauthorized {
		CheckUnauthorizedStatus(t, resp)
	} else {
		require.Fail(t, "Should have failed either forbidden or unauthorized")
	}

	_, resp = th.SystemAdminClient.SetDefaultProfileImage(user.Id)
	CheckNoError(t, resp)

	ruser, err := th.App.GetUser(user.Id)
	require.Nil(t, err)
	assert.Equal(t, int64(0), ruser.LastPictureUpdate, "Picture should have resetted to default")

	info := &model.FileInfo{Path: "users/" + user.Id + "/profile.png"}
	cleanupErr := th.cleanupTestFile(info)
	require.NoError(t, cleanupErr)
}

func TestLogin(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.Client.Logout()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	t.Run("missing password", func(t *testing.T) {
		_, resp := th.Client.Login(th.BasicUser.Email, "")
		CheckErrorMessage(t, resp, "api.user.login.blank_pwd.app_error")
	})

	t.Run("unknown user", func(t *testing.T) {
		_, resp := th.Client.Login("unknown", th.BasicUser.Password)
		CheckErrorMessage(t, resp, "api.user.login.invalid_credentials_email_username")
	})

	t.Run("valid login", func(t *testing.T) {
		user, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
		CheckNoError(t, resp)
		assert.Equal(t, user.Id, th.BasicUser.Id)
	})

	t.Run("bot login rejected", func(t *testing.T) {
		bot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username: "bot",
		})
		CheckNoError(t, resp)

		botUser, resp := th.SystemAdminClient.GetUser(bot.UserId, "")
		CheckNoError(t, resp)

		changed, resp := th.SystemAdminClient.UpdateUserPassword(bot.UserId, "", "password")
		CheckNoError(t, resp)
		require.True(t, changed)

		_, resp = th.Client.Login(botUser.Email, "password")
		CheckErrorMessage(t, resp, "api.user.login.bot_login_forbidden.app_error")
	})

	t.Run("login with terms_of_service set", func(t *testing.T) {
		termsOfService, err := th.App.CreateTermsOfService("terms of service", th.BasicUser.Id)
		require.Nil(t, err)

		success, resp := th.Client.RegisterTermsOfServiceAction(th.BasicUser.Id, termsOfService.Id, true)
		CheckNoError(t, resp)
		assert.True(t, *success)

		userTermsOfService, resp := th.Client.GetUserTermsOfService(th.BasicUser.Id, "")
		CheckNoError(t, resp)

		user, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
		CheckNoError(t, resp)
		assert.Equal(t, user.Id, th.BasicUser.Id)
		assert.Equal(t, user.TermsOfServiceId, userTermsOfService.TermsOfServiceId)
		assert.Equal(t, user.TermsOfServiceCreateAt, userTermsOfService.CreateAt)
	})
}

func TestLoginWithLag(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.Client.Logout()

	t.Run("with replication lag, caches cleared", func(t *testing.T) {
		if !replicaFlag {
			t.Skipf("requires test flag: -mysql-replica")
		}

		if *th.App.Srv().Config().SqlSettings.DriverName != model.DATABASE_DRIVER_MYSQL {
			t.Skipf("requires %q database driver", model.DATABASE_DRIVER_MYSQL)
		}

		mainHelper.SQLStore.UpdateLicense(model.NewTestLicense("ldap"))
		mainHelper.ToggleReplicasOff()

		err := th.App.RevokeAllSessions(th.BasicUser.Id)
		require.Nil(t, err)

		mainHelper.ToggleReplicasOn()
		defer mainHelper.ToggleReplicasOff()

		cmdErr := mainHelper.SetReplicationLagForTesting(5)
		require.NoError(t, cmdErr)
		defer mainHelper.SetReplicationLagForTesting(0)

		_, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
		CheckNoError(t, resp)

		err = th.App.Srv().InvalidateAllCaches()
		require.Nil(t, err)

		session, err := th.App.GetSession(th.Client.AuthToken)
		require.Nil(t, err)
		require.NotNil(t, session)
	})
}

func TestLoginCookies(t *testing.T) {
	t.Run("should return cookies with X-Requested-With header", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.Client.HttpHeader[model.HEADER_REQUESTED_WITH] = model.HEADER_REQUESTED_WITH_XML

		user, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		sessionCookie := ""
		userCookie := ""
		csrfCookie := ""

		for _, cookie := range resp.Header["Set-Cookie"] {
			if match := regexp.MustCompile("^" + model.SESSION_COOKIE_TOKEN + "=([a-z0-9]+)").FindStringSubmatch(cookie); match != nil {
				sessionCookie = match[1]
			} else if match := regexp.MustCompile("^" + model.SESSION_COOKIE_USER + "=([a-z0-9]+)").FindStringSubmatch(cookie); match != nil {
				userCookie = match[1]
			} else if match := regexp.MustCompile("^" + model.SESSION_COOKIE_CSRF + "=([a-z0-9]+)").FindStringSubmatch(cookie); match != nil {
				csrfCookie = match[1]
			}
		}

		session, _ := th.App.GetSession(th.Client.AuthToken)

		assert.Equal(t, th.Client.AuthToken, sessionCookie)
		assert.Equal(t, user.Id, userCookie)
		assert.Equal(t, session.GetCSRF(), csrfCookie)
	})

	t.Run("should not return cookies without X-Requested-With header", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		_, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		assert.Empty(t, resp.Header.Get("Set-Cookie"))
	})

	t.Run("should include subpath in path", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.Client.HttpHeader[model.HEADER_REQUESTED_WITH] = model.HEADER_REQUESTED_WITH_XML

		testCases := []struct {
			Description                   string
			SiteURL                       string
			ExpectedSetCookieHeaderRegexp string
		}{
			{"no subpath", "http://localhost:8065", "^MMAUTHTOKEN=[a-z0-9]+; Path=/"},
			{"subpath", "http://localhost:8065/subpath", "^MMAUTHTOKEN=[a-z0-9]+; Path=/subpath"},
		}

		for _, tc := range testCases {
			t.Run(tc.Description, func(t *testing.T) {
				th.App.UpdateConfig(func(cfg *model.Config) {
					*cfg.ServiceSettings.SiteURL = tc.SiteURL
				})

				user, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
				CheckNoError(t, resp)
				assert.Equal(t, user.Id, th.BasicUser.Id)

				cookies := resp.Header.Get("Set-Cookie")
				assert.Regexp(t, tc.ExpectedSetCookieHeaderRegexp, cookies)
			})
		}
	})
}

func TestCBALogin(t *testing.T) {
	t.Run("primary", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		th.App.Srv().SetLicense(model.NewTestLicense("saml"))

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ExperimentalSettings.ClientSideCertEnable = true
			*cfg.ExperimentalSettings.ClientSideCertCheck = model.CLIENT_SIDE_CERT_CHECK_PRIMARY_AUTH
		})

		t.Run("missing cert header", func(t *testing.T) {
			th.Client.Logout()
			_, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
			CheckBadRequestStatus(t, resp)
		})

		t.Run("missing cert subject", func(t *testing.T) {
			th.Client.Logout()
			th.Client.HttpHeader["X-SSL-Client-Cert"] = "valid_cert_fake"
			_, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
			CheckBadRequestStatus(t, resp)
		})

		t.Run("emails mismatch", func(t *testing.T) {
			th.Client.Logout()
			th.Client.HttpHeader["X-SSL-Client-Cert-Subject-DN"] = "C=US, ST=Maryland, L=Pasadena, O=Brent Baccala, OU=FreeSoft, CN=www.freesoft.org/emailAddress=mis_match" + th.BasicUser.Email
			_, resp := th.Client.Login(th.BasicUser.Email, "")
			CheckUnauthorizedStatus(t, resp)
		})

		t.Run("successful cba login", func(t *testing.T) {
			th.Client.HttpHeader["X-SSL-Client-Cert-Subject-DN"] = "C=US, ST=Maryland, L=Pasadena, O=Brent Baccala, OU=FreeSoft, CN=www.freesoft.org/emailAddress=" + th.BasicUser.Email
			user, resp := th.Client.Login(th.BasicUser.Email, "")
			CheckNoError(t, resp)
			require.NotNil(t, user)
			require.Equal(t, th.BasicUser.Id, user.Id)
		})

		t.Run("bot login rejected", func(t *testing.T) {
			bot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
				Username: "bot",
			})
			CheckNoError(t, resp)

			botUser, resp := th.SystemAdminClient.GetUser(bot.UserId, "")
			CheckNoError(t, resp)

			th.Client.HttpHeader["X-SSL-Client-Cert-Subject-DN"] = "C=US, ST=Maryland, L=Pasadena, O=Brent Baccala, OU=FreeSoft, CN=www.freesoft.org/emailAddress=" + botUser.Email

			_, resp = th.Client.Login(botUser.Email, "")
			CheckErrorMessage(t, resp, "api.user.login.bot_login_forbidden.app_error")
		})
	})

	t.Run("secondary", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		th.App.Srv().SetLicense(model.NewTestLicense("saml"))

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		th.Client.HttpHeader["X-SSL-Client-Cert"] = "valid_cert_fake"

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ExperimentalSettings.ClientSideCertEnable = true
			*cfg.ExperimentalSettings.ClientSideCertCheck = model.CLIENT_SIDE_CERT_CHECK_SECONDARY_AUTH
		})

		t.Run("password required", func(t *testing.T) {
			th.Client.HttpHeader["X-SSL-Client-Cert-Subject-DN"] = "C=US, ST=Maryland, L=Pasadena, O=Brent Baccala, OU=FreeSoft, CN=www.freesoft.org/emailAddress=" + th.BasicUser.Email
			_, resp := th.Client.Login(th.BasicUser.Email, "")
			CheckBadRequestStatus(t, resp)
		})

		t.Run("successful cba login with password", func(t *testing.T) {
			th.Client.HttpHeader["X-SSL-Client-Cert-Subject-DN"] = "C=US, ST=Maryland, L=Pasadena, O=Brent Baccala, OU=FreeSoft, CN=www.freesoft.org/emailAddress=" + th.BasicUser.Email
			user, resp := th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
			CheckNoError(t, resp)
			require.NotNil(t, user)
			require.Equal(t, th.BasicUser.Id, user.Id)
		})

		t.Run("bot login rejected", func(t *testing.T) {
			bot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
				Username: "bot",
			})
			CheckNoError(t, resp)

			botUser, resp := th.SystemAdminClient.GetUser(bot.UserId, "")
			CheckNoError(t, resp)

			changed, resp := th.SystemAdminClient.UpdateUserPassword(bot.UserId, "", "password")
			CheckNoError(t, resp)
			require.True(t, changed)

			th.Client.HttpHeader["X-SSL-Client-Cert-Subject-DN"] = "C=US, ST=Maryland, L=Pasadena, O=Brent Baccala, OU=FreeSoft, CN=www.freesoft.org/emailAddress=" + botUser.Email

			_, resp = th.Client.Login(botUser.Email, "password")
			CheckErrorMessage(t, resp, "api.user.login.bot_login_forbidden.app_error")
		})
	})
}

func TestSwitchAccount(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.Enable = true })

	th.Client.Logout()

	sr := &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_EMAIL,
		NewService:     model.USER_AUTH_SERVICE_GITLAB,
		Email:          th.BasicUser.Email,
		Password:       th.BasicUser.Password,
	}

	link, resp := th.Client.SwitchAccountType(sr)
	CheckNoError(t, resp)

	require.NotEmpty(t, link, "bad link")

	th.App.Srv().SetLicense(model.NewTestLicense())
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.ExperimentalEnableAuthenticationTransfer = false })

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_EMAIL,
		NewService:     model.USER_AUTH_SERVICE_GITLAB,
	}

	_, resp = th.Client.SwitchAccountType(sr)
	CheckForbiddenStatus(t, resp)

	th.LoginBasic()

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_SAML,
		NewService:     model.USER_AUTH_SERVICE_EMAIL,
		Email:          th.BasicUser.Email,
		NewPassword:    th.BasicUser.Password,
	}

	_, resp = th.Client.SwitchAccountType(sr)
	CheckForbiddenStatus(t, resp)

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_EMAIL,
		NewService:     model.USER_AUTH_SERVICE_LDAP,
	}

	_, resp = th.Client.SwitchAccountType(sr)
	CheckForbiddenStatus(t, resp)

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_LDAP,
		NewService:     model.USER_AUTH_SERVICE_EMAIL,
	}

	_, resp = th.Client.SwitchAccountType(sr)
	CheckForbiddenStatus(t, resp)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.ExperimentalEnableAuthenticationTransfer = true })

	th.LoginBasic()

	fakeAuthData := model.NewId()
	_, err := th.App.Srv().Store.User().UpdateAuthData(th.BasicUser.Id, model.USER_AUTH_SERVICE_GITLAB, &fakeAuthData, th.BasicUser.Email, true)
	require.NoError(t, err)

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_GITLAB,
		NewService:     model.USER_AUTH_SERVICE_EMAIL,
		Email:          th.BasicUser.Email,
		NewPassword:    th.BasicUser.Password,
	}

	link, resp = th.Client.SwitchAccountType(sr)
	CheckNoError(t, resp)

	require.Equal(t, "/login?extra=signin_change", link)

	th.Client.Logout()
	_, resp = th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
	CheckNoError(t, resp)
	th.Client.Logout()

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_GITLAB,
		NewService:     model.SERVICE_GOOGLE,
	}

	_, resp = th.Client.SwitchAccountType(sr)
	CheckBadRequestStatus(t, resp)

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_EMAIL,
		NewService:     model.USER_AUTH_SERVICE_GITLAB,
		Password:       th.BasicUser.Password,
	}

	_, resp = th.Client.SwitchAccountType(sr)
	CheckNotFoundStatus(t, resp)

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_EMAIL,
		NewService:     model.USER_AUTH_SERVICE_GITLAB,
		Email:          th.BasicUser.Email,
	}

	_, resp = th.Client.SwitchAccountType(sr)
	CheckUnauthorizedStatus(t, resp)

	sr = &model.SwitchRequest{
		CurrentService: model.USER_AUTH_SERVICE_GITLAB,
		NewService:     model.USER_AUTH_SERVICE_EMAIL,
		Email:          th.BasicUser.Email,
		NewPassword:    th.BasicUser.Password,
	}

	_, resp = th.Client.SwitchAccountType(sr)
	CheckUnauthorizedStatus(t, resp)
}

func assertToken(t *testing.T, th *TestHelper, token *model.UserAccessToken, expectedUserId string) {
	t.Helper()

	oldSessionToken := th.Client.AuthToken
	defer func() { th.Client.AuthToken = oldSessionToken }()

	th.Client.AuthToken = token.Token
	ruser, resp := th.Client.GetMe("")
	CheckNoError(t, resp)

	assert.Equal(t, expectedUserId, ruser.Id, "returned wrong user")
}

func assertInvalidToken(t *testing.T, th *TestHelper, token *model.UserAccessToken) {
	t.Helper()

	oldSessionToken := th.Client.AuthToken
	defer func() { th.Client.AuthToken = oldSessionToken }()

	th.Client.AuthToken = token.Token
	_, resp := th.Client.GetMe("")
	CheckUnauthorizedStatus(t, resp)
}

func TestCreateUserAccessToken(t *testing.T) {
	t.Run("create token without permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		_, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckForbiddenStatus(t, resp)
	})

	t.Run("system admin and local mode can create access token", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			rtoken, resp := client.CreateUserAccessToken(th.BasicUser.Id, "test token")
			CheckNoError(t, resp)

			assert.Equal(t, th.BasicUser.Id, rtoken.UserId, "wrong user id")
			assert.NotEmpty(t, rtoken.Token, "token should not be empty")
			assert.NotEmpty(t, rtoken.Id, "id should not be empty")
			assert.Equal(t, "test token", rtoken.Description, "description did not match")
			assert.True(t, rtoken.IsActive, "token should be active")
			assertToken(t, th, rtoken, th.BasicUser.Id)
		})
	})

	t.Run("create token for invalid user id", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.CreateUserAccessToken("notarealuserid", "test token")
			CheckBadRequestStatus(t, resp)
		})
	})

	t.Run("create token with invalid value", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.CreateUserAccessToken(th.BasicUser.Id, "")
			CheckBadRequestStatus(t, resp)
		})
	})

	t.Run("create token with user access tokens disabled", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = false })
		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.CreateUserAccessToken(th.BasicUser.Id, "test token")
			CheckNotImplementedStatus(t, resp)
		})
	})

	t.Run("create user access token", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })
		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		rtoken, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckNoError(t, resp)

		assert.Equal(t, th.BasicUser.Id, rtoken.UserId, "wrong user id")
		assert.NotEmpty(t, rtoken.Token, "token should not be empty")
		assert.NotEmpty(t, rtoken.Id, "id should not be empty")
		assert.Equal(t, "test token", rtoken.Description, "description did not match")
		assert.True(t, rtoken.IsActive, "token should be active")

		assertToken(t, th, rtoken, th.BasicUser.Id)
	})

	t.Run("create user access token as second user, without permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		_, resp := th.Client.CreateUserAccessToken(th.BasicUser2.Id, "test token")
		CheckForbiddenStatus(t, resp)
	})

	t.Run("create user access token for basic user as as system admin", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		rtoken, resp := th.SystemAdminClient.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckNoError(t, resp)
		assert.Equal(t, th.BasicUser.Id, rtoken.UserId)

		oldSessionToken := th.Client.AuthToken
		defer func() { th.Client.AuthToken = oldSessionToken }()

		assertToken(t, th, rtoken, th.BasicUser.Id)
	})

	t.Run("create access token as oauth session", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		session, _ := th.App.GetSession(th.Client.AuthToken)
		session.IsOAuth = true
		th.App.AddSessionToCache(session)

		_, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckForbiddenStatus(t, resp)
	})

	t.Run("create access token for bot created by user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		t.Run("without MANAGE_BOT permission", func(t *testing.T) {
			th.RemovePermissionFromRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			_, resp = th.Client.CreateUserAccessToken(createdBot.UserId, "test token")
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			token, resp := th.Client.CreateUserAccessToken(createdBot.UserId, "test token")
			CheckNoError(t, resp)
			assert.Equal(t, createdBot.UserId, token.UserId)
			assertToken(t, th, token, createdBot.UserId)
		})
	})

	t.Run("create access token for bot created by another user, only having MANAGE_BOTS permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		t.Run("only having MANAGE_BOTS permission", func(t *testing.T) {
			_, resp = th.Client.CreateUserAccessToken(createdBot.UserId, "test token")
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_OTHERS_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_OTHERS_BOTS.Id, model.TEAM_USER_ROLE_ID)

			rtoken, resp := th.Client.CreateUserAccessToken(createdBot.UserId, "test token")
			CheckNoError(t, resp)
			assert.Equal(t, createdBot.UserId, rtoken.UserId)

			assertToken(t, th, rtoken, createdBot.UserId)
		})
	})
}

func TestGetUserAccessToken(t *testing.T) {
	t.Run("get for invalid user id", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		_, resp := th.Client.GetUserAccessToken("123")
		CheckBadRequestStatus(t, resp)
	})

	t.Run("get for unknown user id", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		_, resp := th.Client.GetUserAccessToken(model.NewId())
		CheckForbiddenStatus(t, resp)
	})

	t.Run("get my token", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })
		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		token, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckNoError(t, resp)

		rtoken, resp := th.Client.GetUserAccessToken(token.Id)
		CheckNoError(t, resp)

		assert.Equal(t, th.BasicUser.Id, rtoken.UserId, "wrong user id")
		assert.Empty(t, rtoken.Token, "token should be blank")
		assert.NotEmpty(t, rtoken.Id, "id should not be empty")
		assert.Equal(t, "test token", rtoken.Description, "description did not match")
	})

	t.Run("get user token as system admin", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		token, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckNoError(t, resp)

		rtoken, resp := th.SystemAdminClient.GetUserAccessToken(token.Id)
		CheckNoError(t, resp)

		assert.Equal(t, th.BasicUser.Id, rtoken.UserId, "wrong user id")
		assert.Empty(t, rtoken.Token, "token should be blank")
		assert.NotEmpty(t, rtoken.Id, "id should not be empty")
		assert.Equal(t, "test token", rtoken.Description, "description did not match")
	})

	t.Run("get token for bot created by user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_READ_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		token, resp := th.Client.CreateUserAccessToken(createdBot.UserId, "test token")
		CheckNoError(t, resp)

		t.Run("without MANAGE_BOTS permission", func(t *testing.T) {
			th.RemovePermissionFromRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			_, resp := th.Client.GetUserAccessToken(token.Id)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			returnedToken, resp := th.Client.GetUserAccessToken(token.Id)
			CheckNoError(t, resp)

			// Actual token won't be returned.
			returnedToken.Token = token.Token
			assert.Equal(t, token, returnedToken)
		})
	})

	t.Run("get token for bot created by another user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_READ_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		token, resp := th.SystemAdminClient.CreateUserAccessToken(createdBot.UserId, "test token")
		CheckNoError(t, resp)

		t.Run("only having MANAGE_BOTS permission", func(t *testing.T) {
			_, resp = th.Client.GetUserAccessToken(token.Id)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_OTHERS_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_OTHERS_BOTS.Id, model.TEAM_USER_ROLE_ID)

			returnedToken, resp := th.Client.GetUserAccessToken(token.Id)
			CheckNoError(t, resp)

			// Actual token won't be returned.
			returnedToken.Token = token.Token
			assert.Equal(t, token, returnedToken)
		})
	})
}

func TestGetUserAccessTokensForUser(t *testing.T) {
	t.Run("multiple tokens, offset 0, limit 100", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		_, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckNoError(t, resp)

		_, resp = th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token 2")
		CheckNoError(t, resp)

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			rtokens, resp := client.GetUserAccessTokensForUser(th.BasicUser.Id, 0, 100)
			CheckNoError(t, resp)

			assert.Len(t, rtokens, 2, "should have 2 tokens")
			for _, uat := range rtokens {
				assert.Equal(t, th.BasicUser.Id, uat.UserId, "wrong user id")
			}
		})
	})

	t.Run("multiple tokens, offset 1, limit 1", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		_, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckNoError(t, resp)

		_, resp = th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token 2")
		CheckNoError(t, resp)

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			rtokens, resp := client.GetUserAccessTokensForUser(th.BasicUser.Id, 1, 1)
			CheckNoError(t, resp)

			assert.Len(t, rtokens, 1, "should have 1 tokens")
			for _, uat := range rtokens {
				assert.Equal(t, th.BasicUser.Id, uat.UserId, "wrong user id")
			}
		})
	})
}

func TestGetUserAccessTokens(t *testing.T) {
	t.Run("GetUserAccessTokens, not a system admin", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		_, resp := th.Client.GetUserAccessTokens(0, 100)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("GetUserAccessTokens, as a system admin, page 1, perPage 1", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		_, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token 2")
		CheckNoError(t, resp)

		_, resp = th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token 2")
		CheckNoError(t, resp)

		rtokens, resp := th.SystemAdminClient.GetUserAccessTokens(1, 1)
		CheckNoError(t, resp)

		assert.Len(t, rtokens, 1, "should have 1 token")
	})

	t.Run("GetUserAccessTokens, as a system admin, page 0, perPage 2", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)

		_, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token 2")
		CheckNoError(t, resp)

		_, resp = th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token 2")
		CheckNoError(t, resp)

		rtokens, resp := th.SystemAdminClient.GetUserAccessTokens(0, 2)
		CheckNoError(t, resp)

		assert.Len(t, rtokens, 2, "should have 2 tokens")
	})
}

func TestSearchUserAccessToken(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	testDescription := "test token"

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

	th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)
	token, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, testDescription)
	CheckNoError(t, resp)

	_, resp = th.Client.SearchUserAccessTokens(&model.UserAccessTokenSearch{Term: token.Id})
	CheckForbiddenStatus(t, resp)

	rtokens, resp := th.SystemAdminClient.SearchUserAccessTokens(&model.UserAccessTokenSearch{Term: th.BasicUser.Id})
	CheckNoError(t, resp)

	require.Len(t, rtokens, 1, "should have 1 token")

	rtokens, resp = th.SystemAdminClient.SearchUserAccessTokens(&model.UserAccessTokenSearch{Term: token.Id})
	CheckNoError(t, resp)

	require.Len(t, rtokens, 1, "should have 1 token")

	rtokens, resp = th.SystemAdminClient.SearchUserAccessTokens(&model.UserAccessTokenSearch{Term: th.BasicUser.Username})
	CheckNoError(t, resp)

	require.Len(t, rtokens, 1, "should have 1 token")

	rtokens, resp = th.SystemAdminClient.SearchUserAccessTokens(&model.UserAccessTokenSearch{Term: "not found"})
	CheckNoError(t, resp)

	require.Empty(t, rtokens, "should have 1 tokens")
}

func TestRevokeUserAccessToken(t *testing.T) {
	t.Run("revoke user token", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			token, resp := client.CreateUserAccessToken(th.BasicUser.Id, "test token")
			CheckNoError(t, resp)
			assertToken(t, th, token, th.BasicUser.Id)

			ok, resp := client.RevokeUserAccessToken(token.Id)
			CheckNoError(t, resp)
			assert.True(t, ok, "should have passed")

			assertInvalidToken(t, th, token)
		})
	})

	t.Run("revoke token belonging to another user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		token, resp := th.SystemAdminClient.CreateUserAccessToken(th.BasicUser2.Id, "test token")
		CheckNoError(t, resp)

		ok, resp := th.Client.RevokeUserAccessToken(token.Id)
		CheckForbiddenStatus(t, resp)
		assert.False(t, ok, "should have failed")
	})

	t.Run("revoke token for bot created by user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_REVOKE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		token, resp := th.Client.CreateUserAccessToken(createdBot.UserId, "test token")
		CheckNoError(t, resp)

		t.Run("without MANAGE_BOTS permission", func(t *testing.T) {
			th.RemovePermissionFromRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			_, resp := th.Client.RevokeUserAccessToken(token.Id)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			ok, resp := th.Client.RevokeUserAccessToken(token.Id)
			CheckNoError(t, resp)
			assert.True(t, ok, "should have passed")
		})
	})

	t.Run("revoke token for bot created by another user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_REVOKE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		token, resp := th.SystemAdminClient.CreateUserAccessToken(createdBot.UserId, "test token")
		CheckNoError(t, resp)

		t.Run("only having MANAGE_BOTS permission", func(t *testing.T) {
			_, resp = th.Client.RevokeUserAccessToken(token.Id)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_OTHERS_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_OTHERS_BOTS.Id, model.TEAM_USER_ROLE_ID)

			ok, resp := th.Client.RevokeUserAccessToken(token.Id)
			CheckNoError(t, resp)
			assert.True(t, ok, "should have passed")
		})
	})
}

func TestDisableUserAccessToken(t *testing.T) {
	t.Run("disable user token", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)
		token, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckNoError(t, resp)
		assertToken(t, th, token, th.BasicUser.Id)

		ok, resp := th.Client.DisableUserAccessToken(token.Id)
		CheckNoError(t, resp)
		assert.True(t, ok, "should have passed")

		assertInvalidToken(t, th, token)
	})

	t.Run("disable token belonging to another user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		token, resp := th.SystemAdminClient.CreateUserAccessToken(th.BasicUser2.Id, "test token")
		CheckNoError(t, resp)

		ok, resp := th.Client.DisableUserAccessToken(token.Id)
		CheckForbiddenStatus(t, resp)
		assert.False(t, ok, "should have failed")
	})

	t.Run("disable token for bot created by user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_REVOKE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		token, resp := th.Client.CreateUserAccessToken(createdBot.UserId, "test token")
		CheckNoError(t, resp)

		t.Run("without MANAGE_BOTS permission", func(t *testing.T) {
			th.RemovePermissionFromRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			_, resp := th.Client.DisableUserAccessToken(token.Id)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			ok, resp := th.Client.DisableUserAccessToken(token.Id)
			CheckNoError(t, resp)
			assert.True(t, ok, "should have passed")
		})
	})

	t.Run("disable token for bot created by another user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_REVOKE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		token, resp := th.SystemAdminClient.CreateUserAccessToken(createdBot.UserId, "test token")
		CheckNoError(t, resp)

		t.Run("only having MANAGE_BOTS permission", func(t *testing.T) {
			_, resp = th.Client.DisableUserAccessToken(token.Id)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_OTHERS_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_OTHERS_BOTS.Id, model.TEAM_USER_ROLE_ID)

			ok, resp := th.Client.DisableUserAccessToken(token.Id)
			CheckNoError(t, resp)
			assert.True(t, ok, "should have passed")
		})
	})
}

func TestEnableUserAccessToken(t *testing.T) {
	t.Run("enable user token", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)
		token, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, "test token")
		CheckNoError(t, resp)
		assertToken(t, th, token, th.BasicUser.Id)

		ok, resp := th.Client.DisableUserAccessToken(token.Id)
		CheckNoError(t, resp)
		assert.True(t, ok, "should have passed")

		assertInvalidToken(t, th, token)

		ok, resp = th.Client.EnableUserAccessToken(token.Id)
		CheckNoError(t, resp)
		assert.True(t, ok, "should have passed")

		assertToken(t, th, token, th.BasicUser.Id)
	})

	t.Run("enable token belonging to another user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		token, resp := th.SystemAdminClient.CreateUserAccessToken(th.BasicUser2.Id, "test token")
		CheckNoError(t, resp)

		ok, resp := th.SystemAdminClient.DisableUserAccessToken(token.Id)
		CheckNoError(t, resp)
		assert.True(t, ok, "should have passed")

		ok, resp = th.Client.DisableUserAccessToken(token.Id)
		CheckForbiddenStatus(t, resp)
		assert.False(t, ok, "should have failed")
	})

	t.Run("enable token for bot created by user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_REVOKE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		token, resp := th.Client.CreateUserAccessToken(createdBot.UserId, "test token")
		CheckNoError(t, resp)

		ok, resp := th.Client.DisableUserAccessToken(token.Id)
		CheckNoError(t, resp)
		assert.True(t, ok, "should have passed")

		t.Run("without MANAGE_BOTS permission", func(t *testing.T) {
			th.RemovePermissionFromRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			_, resp := th.Client.EnableUserAccessToken(token.Id)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)

			ok, resp := th.Client.EnableUserAccessToken(token.Id)
			CheckNoError(t, resp)
			assert.True(t, ok, "should have passed")
		})
	})

	t.Run("enable token for bot created by another user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())
		th.AddPermissionToRole(model.PERMISSION_CREATE_BOT.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_MANAGE_BOTS.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_CREATE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.AddPermissionToRole(model.PERMISSION_REVOKE_USER_ACCESS_TOKEN.Id, model.TEAM_USER_ROLE_ID)
		th.App.UpdateUserRoles(th.BasicUser.Id, model.TEAM_USER_ROLE_ID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(th.Context, createdBot.UserId)

		token, resp := th.SystemAdminClient.CreateUserAccessToken(createdBot.UserId, "test token")
		CheckNoError(t, resp)

		ok, resp := th.SystemAdminClient.DisableUserAccessToken(token.Id)
		CheckNoError(t, resp)
		assert.True(t, ok, "should have passed")

		t.Run("only having MANAGE_BOTS permission", func(t *testing.T) {
			_, resp := th.Client.EnableUserAccessToken(token.Id)
			CheckForbiddenStatus(t, resp)
		})

		t.Run("with MANAGE_OTHERS_BOTS permission", func(t *testing.T) {
			th.AddPermissionToRole(model.PERMISSION_MANAGE_OTHERS_BOTS.Id, model.TEAM_USER_ROLE_ID)

			ok, resp := th.Client.EnableUserAccessToken(token.Id)
			CheckNoError(t, resp)
			assert.True(t, ok, "should have passed")
		})
	})
}

func TestUserAccessTokenInactiveUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	testDescription := "test token"

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

	th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)
	token, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, testDescription)
	CheckNoError(t, resp)

	th.Client.AuthToken = token.Token
	_, resp = th.Client.GetMe("")
	CheckNoError(t, resp)

	th.App.UpdateActive(th.Context, th.BasicUser, false)

	_, resp = th.Client.GetMe("")
	CheckUnauthorizedStatus(t, resp)
}

func TestUserAccessTokenDisableConfig(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	testDescription := "test token"

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })

	th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_USER_ACCESS_TOKEN_ROLE_ID, false)
	token, resp := th.Client.CreateUserAccessToken(th.BasicUser.Id, testDescription)
	CheckNoError(t, resp)

	oldSessionToken := th.Client.AuthToken
	th.Client.AuthToken = token.Token
	_, resp = th.Client.GetMe("")
	CheckNoError(t, resp)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = false })

	_, resp = th.Client.GetMe("")
	CheckUnauthorizedStatus(t, resp)

	th.Client.AuthToken = oldSessionToken
	_, resp = th.Client.GetMe("")
	CheckNoError(t, resp)
}

func TestUserAccessTokenDisableConfigBotsExcluded(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
		*cfg.ServiceSettings.EnableUserAccessTokens = false
	})

	bot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "a bot",
		Description: "bot",
	})
	CheckCreatedStatus(t, resp)

	rtoken, resp := th.SystemAdminClient.CreateUserAccessToken(bot.UserId, "test token")
	th.Client.AuthToken = rtoken.Token
	CheckNoError(t, resp)

	_, resp = th.Client.GetMe("")
	CheckNoError(t, resp)
}

func TestGetUsersByStatus(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	team, err := th.App.CreateTeam(th.Context, &model.Team{
		DisplayName: "dn_" + model.NewId(),
		Name:        GenerateTestTeamName(),
		Email:       th.GenerateTestEmail(),
		Type:        model.TEAM_OPEN,
	})

	require.Nil(t, err, "failed to create team")

	channel, err := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "dn_" + model.NewId(),
		Name:        "name_" + model.NewId(),
		Type:        model.CHANNEL_OPEN,
		TeamId:      team.Id,
		CreatorId:   model.NewId(),
	}, false)
	require.Nil(t, err, "failed to create channel")

	createUserWithStatus := func(username string, status string) *model.User {
		id := model.NewId()

		user, err := th.App.CreateUser(th.Context, &model.User{
			Email:    "success+" + id + "@simulator.amazonses.com",
			Username: "un_" + username + "_" + id,
			Nickname: "nn_" + id,
			Password: "Password1",
		})
		require.Nil(t, err, "failed to create user")

		th.LinkUserToTeam(user, team)
		th.AddUserToChannel(user, channel)

		th.App.SaveAndBroadcastStatus(&model.Status{
			UserId: user.Id,
			Status: status,
			Manual: true,
		})

		return user
	}

	// Creating these out of order in case that affects results
	offlineUser1 := createUserWithStatus("offline1", model.STATUS_OFFLINE)
	offlineUser2 := createUserWithStatus("offline2", model.STATUS_OFFLINE)
	awayUser1 := createUserWithStatus("away1", model.STATUS_AWAY)
	awayUser2 := createUserWithStatus("away2", model.STATUS_AWAY)
	onlineUser1 := createUserWithStatus("online1", model.STATUS_ONLINE)
	onlineUser2 := createUserWithStatus("online2", model.STATUS_ONLINE)
	dndUser1 := createUserWithStatus("dnd1", model.STATUS_DND)
	dndUser2 := createUserWithStatus("dnd2", model.STATUS_DND)

	client := th.CreateClient()
	_, resp := client.Login(onlineUser2.Username, "Password1")
	require.Nil(t, resp.Error)

	t.Run("sorting by status then alphabetical", func(t *testing.T) {
		usersByStatus, resp := client.GetUsersInChannelByStatus(channel.Id, 0, 8, "")
		require.Nil(t, resp.Error)

		expectedUsersByStatus := []*model.User{
			onlineUser1,
			onlineUser2,
			awayUser1,
			awayUser2,
			dndUser1,
			dndUser2,
			offlineUser1,
			offlineUser2,
		}
		require.Equal(t, len(expectedUsersByStatus), len(usersByStatus))

		for i := range usersByStatus {
			require.Equal(t, expectedUsersByStatus[i].Id, usersByStatus[i].Id)
		}
	})

	t.Run("paging", func(t *testing.T) {
		usersByStatus, resp := client.GetUsersInChannelByStatus(channel.Id, 0, 3, "")
		require.Nil(t, resp.Error)
		require.Len(t, usersByStatus, 3)
		require.Equal(t, onlineUser1.Id, usersByStatus[0].Id, "online users first")
		require.Equal(t, onlineUser2.Id, usersByStatus[1].Id, "online users first")
		require.Equal(t, awayUser1.Id, usersByStatus[2].Id, "expected to receive away users second")

		usersByStatus, resp = client.GetUsersInChannelByStatus(channel.Id, 1, 3, "")
		require.Nil(t, resp.Error)

		require.Equal(t, awayUser2.Id, usersByStatus[0].Id, "expected to receive away users second")
		require.Equal(t, dndUser1.Id, usersByStatus[1].Id, "expected to receive dnd users third")
		require.Equal(t, dndUser2.Id, usersByStatus[2].Id, "expected to receive dnd users third")

		usersByStatus, resp = client.GetUsersInChannelByStatus(channel.Id, 1, 4, "")
		require.Nil(t, resp.Error)

		require.Len(t, usersByStatus, 4)
		require.Equal(t, dndUser1.Id, usersByStatus[0].Id, "expected to receive dnd users third")
		require.Equal(t, dndUser2.Id, usersByStatus[1].Id, "expected to receive dnd users third")

		require.Equal(t, offlineUser1.Id, usersByStatus[2].Id, "expected to receive offline users last")
		require.Equal(t, offlineUser2.Id, usersByStatus[3].Id, "expected to receive offline users last")
	})
}

func TestRegisterTermsOfServiceAction(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	success, resp := th.Client.RegisterTermsOfServiceAction(th.BasicUser.Id, "st_1", true)
	CheckErrorMessage(t, resp, "app.terms_of_service.get.no_rows.app_error")
	assert.Nil(t, success)

	termsOfService, err := th.App.CreateTermsOfService("terms of service", th.BasicUser.Id)
	require.Nil(t, err)

	success, resp = th.Client.RegisterTermsOfServiceAction(th.BasicUser.Id, termsOfService.Id, true)
	CheckNoError(t, resp)

	assert.True(t, *success)
	_, err = th.App.GetUser(th.BasicUser.Id)
	require.Nil(t, err)
}

func TestGetUserTermsOfService(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, resp := th.Client.GetUserTermsOfService(th.BasicUser.Id, "")
	CheckErrorMessage(t, resp, "app.user_terms_of_service.get_by_user.no_rows.app_error")

	termsOfService, err := th.App.CreateTermsOfService("terms of service", th.BasicUser.Id)
	require.Nil(t, err)

	success, resp := th.Client.RegisterTermsOfServiceAction(th.BasicUser.Id, termsOfService.Id, true)
	CheckNoError(t, resp)
	assert.True(t, *success)

	userTermsOfService, resp := th.Client.GetUserTermsOfService(th.BasicUser.Id, "")
	CheckNoError(t, resp)

	assert.Equal(t, th.BasicUser.Id, userTermsOfService.UserId)
	assert.Equal(t, termsOfService.Id, userTermsOfService.TermsOfServiceId)
	assert.NotEmpty(t, userTermsOfService.CreateAt)
}

func TestLoginErrorMessage(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, resp := th.Client.Logout()
	CheckNoError(t, resp)

	// Email and Username enabled
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.EmailSettings.EnableSignInWithEmail = true
		*cfg.EmailSettings.EnableSignInWithUsername = true
	})
	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.login.invalid_credentials_email_username")

	// Email enabled
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.EmailSettings.EnableSignInWithEmail = true
		*cfg.EmailSettings.EnableSignInWithUsername = false
	})
	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.login.invalid_credentials_email")

	// Username enabled
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.EmailSettings.EnableSignInWithEmail = false
		*cfg.EmailSettings.EnableSignInWithUsername = true
	})
	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.login.invalid_credentials_username")

	// SAML/SSO enabled
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.SamlSettings.Enable = true
		*cfg.SamlSettings.Verify = false
		*cfg.SamlSettings.Encrypt = false
		*cfg.SamlSettings.IdpUrl = "https://localhost/adfs/ls"
		*cfg.SamlSettings.IdpDescriptorUrl = "https://localhost/adfs/services/trust"
		*cfg.SamlSettings.IdpMetadataUrl = "https://localhost/adfs/metadata"
		*cfg.SamlSettings.ServiceProviderIdentifier = "https://localhost/login/sso/saml"
		*cfg.SamlSettings.AssertionConsumerServiceURL = "https://localhost/login/sso/saml"
		*cfg.SamlSettings.IdpCertificateFile = app.SamlIdpCertificateName
		*cfg.SamlSettings.PrivateKeyFile = app.SamlPrivateKeyName
		*cfg.SamlSettings.PublicCertificateFile = app.SamlPublicCertificateName
		*cfg.SamlSettings.EmailAttribute = "Email"
		*cfg.SamlSettings.UsernameAttribute = "Username"
		*cfg.SamlSettings.FirstNameAttribute = "FirstName"
		*cfg.SamlSettings.LastNameAttribute = "LastName"
		*cfg.SamlSettings.NicknameAttribute = ""
		*cfg.SamlSettings.PositionAttribute = ""
		*cfg.SamlSettings.LocaleAttribute = ""
	})
	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.login.invalid_credentials_sso")
}

func TestLoginLockout(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, resp := th.Client.Logout()
	CheckNoError(t, resp)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.MaximumLoginAttempts = 3 })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableMultifactorAuthentication = true })

	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.login.invalid_credentials_email_username")
	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.login.invalid_credentials_email_username")
	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.login.invalid_credentials_email_username")
	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.check_user_login_attempts.too_many.app_error")
	_, resp = th.Client.Login(th.BasicUser.Email, "wrong")
	CheckErrorMessage(t, resp, "api.user.check_user_login_attempts.too_many.app_error")

	//Check if lock is active
	_, resp = th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)
	CheckErrorMessage(t, resp, "api.user.check_user_login_attempts.too_many.app_error")

	// Fake user has MFA enabled
	err := th.Server.Store.User().UpdateMfaActive(th.BasicUser2.Id, true)
	require.NoError(t, err)
	_, resp = th.Client.LoginWithMFA(th.BasicUser2.Email, th.BasicUser2.Password, "000000")
	CheckErrorMessage(t, resp, "api.user.check_user_mfa.bad_code.app_error")
	_, resp = th.Client.LoginWithMFA(th.BasicUser2.Email, th.BasicUser2.Password, "000000")
	CheckErrorMessage(t, resp, "api.user.check_user_mfa.bad_code.app_error")
	_, resp = th.Client.LoginWithMFA(th.BasicUser2.Email, th.BasicUser2.Password, "000000")
	CheckErrorMessage(t, resp, "api.user.check_user_mfa.bad_code.app_error")
	_, resp = th.Client.LoginWithMFA(th.BasicUser2.Email, th.BasicUser2.Password, "000000")
	CheckErrorMessage(t, resp, "api.user.check_user_login_attempts.too_many.app_error")
	_, resp = th.Client.LoginWithMFA(th.BasicUser2.Email, th.BasicUser2.Password, "000000")
	CheckErrorMessage(t, resp, "api.user.check_user_login_attempts.too_many.app_error")

	// Fake user has MFA disabled
	err = th.Server.Store.User().UpdateMfaActive(th.BasicUser2.Id, false)
	require.NoError(t, err)

	//Check if lock is active
	_, resp = th.Client.Login(th.BasicUser2.Email, th.BasicUser2.Password)
	CheckErrorMessage(t, resp, "api.user.check_user_login_attempts.too_many.app_error")
}

func TestDemoteUserToGuest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		th.App.Srv().RemoveLicense()
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	user := th.BasicUser

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		_, respErr := c.GetUser(user.Id, "")
		CheckNoError(t, respErr)

		_, respErr = c.DemoteUserToGuest(user.Id)
		CheckNoError(t, respErr)

		defer require.Nil(t, th.App.PromoteGuestToUser(th.Context, user, ""))
	}, "demote a user to guest")

	t.Run("websocket update user event", func(t *testing.T) {
		webSocketClient, err := th.CreateWebSocketClient()
		assert.Nil(t, err)
		defer webSocketClient.Close()

		webSocketClient.Listen()

		time.Sleep(300 * time.Millisecond)
		resp := <-webSocketClient.ResponseChannel
		require.Equal(t, model.STATUS_OK, resp.Status)

		adminWebSocketClient, err := th.CreateWebSocketSystemAdminClient()
		assert.Nil(t, err)
		defer adminWebSocketClient.Close()

		adminWebSocketClient.Listen()

		time.Sleep(300 * time.Millisecond)
		resp = <-adminWebSocketClient.ResponseChannel
		require.Equal(t, model.STATUS_OK, resp.Status)

		_, respErr := th.SystemAdminClient.GetUser(user.Id, "")
		CheckNoError(t, respErr)
		_, respErr = th.SystemAdminClient.DemoteUserToGuest(user.Id)
		CheckNoError(t, respErr)
		defer th.SystemAdminClient.PromoteGuestToUser(user.Id)

		assertExpectedWebsocketEvent(t, webSocketClient, model.WEBSOCKET_EVENT_USER_UPDATED, func(event *model.WebSocketEvent) {
			eventUser, ok := event.GetData()["user"].(*model.User)
			require.True(t, ok, "expected user")
			assert.Equal(t, "system_guest", eventUser.Roles)
		})
		assertExpectedWebsocketEvent(t, adminWebSocketClient, model.WEBSOCKET_EVENT_USER_UPDATED, func(event *model.WebSocketEvent) {
			eventUser, ok := event.GetData()["user"].(*model.User)
			require.True(t, ok, "expected user")
			assert.Equal(t, "system_guest", eventUser.Roles)
		})
	})
}

func TestPromoteGuestToUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		th.App.Srv().RemoveLicense()
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.Srv().SetLicense(model.NewTestLicense())

	user := th.BasicUser
	th.App.UpdateUserRoles(user.Id, model.SYSTEM_GUEST_ROLE_ID, false)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		_, respErr := c.GetUser(user.Id, "")
		CheckNoError(t, respErr)

		_, respErr = c.PromoteGuestToUser(user.Id)
		CheckNoError(t, respErr)

		defer require.Nil(t, th.App.DemoteUserToGuest(user))
	}, "promete a guest to user")

	t.Run("websocket update user event", func(t *testing.T) {
		webSocketClient, err := th.CreateWebSocketClient()
		assert.Nil(t, err)
		defer webSocketClient.Close()

		webSocketClient.Listen()

		time.Sleep(300 * time.Millisecond)
		resp := <-webSocketClient.ResponseChannel
		require.Equal(t, model.STATUS_OK, resp.Status)

		adminWebSocketClient, err := th.CreateWebSocketSystemAdminClient()
		assert.Nil(t, err)
		defer adminWebSocketClient.Close()

		adminWebSocketClient.Listen()

		time.Sleep(300 * time.Millisecond)
		resp = <-adminWebSocketClient.ResponseChannel
		require.Equal(t, model.STATUS_OK, resp.Status)

		_, respErr := th.SystemAdminClient.GetUser(user.Id, "")
		CheckNoError(t, respErr)
		_, respErr = th.SystemAdminClient.PromoteGuestToUser(user.Id)
		CheckNoError(t, respErr)
		defer th.SystemAdminClient.DemoteUserToGuest(user.Id)

		assertExpectedWebsocketEvent(t, webSocketClient, model.WEBSOCKET_EVENT_USER_UPDATED, func(event *model.WebSocketEvent) {
			eventUser, ok := event.GetData()["user"].(*model.User)
			require.True(t, ok, "expected user")
			assert.Equal(t, "system_user", eventUser.Roles)
		})
		assertExpectedWebsocketEvent(t, adminWebSocketClient, model.WEBSOCKET_EVENT_USER_UPDATED, func(event *model.WebSocketEvent) {
			eventUser, ok := event.GetData()["user"].(*model.User)
			require.True(t, ok, "expected user")
			assert.Equal(t, "system_user", eventUser.Roles)
		})
	})
}

func TestVerifyUserEmailWithoutToken(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		email := th.GenerateTestEmail()
		user := model.User{Email: email, Nickname: "Darth Vader", Password: "hello1", Username: GenerateTestUsername(), Roles: model.SYSTEM_USER_ROLE_ID}
		ruser, _ := th.Client.CreateUser(&user)

		vuser, resp := client.VerifyUserEmailWithoutToken(ruser.Id)
		require.Nil(t, resp.Error)
		require.Equal(t, ruser.Id, vuser.Id)
	}, "Should verify a new user")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		vuser, resp := client.VerifyUserEmailWithoutToken("randomId")
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "api.context.invalid_url_param.app_error")
		require.Nil(t, vuser)
	}, "Should not be able to find user")

	t.Run("Should not be able to verify user due to permissions", func(t *testing.T) {
		user := th.CreateUser()
		vuser, resp := th.Client.VerifyUserEmailWithoutToken(user.Id)
		require.NotNil(t, resp.Error)
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
		require.Nil(t, vuser)
	})
}

func TestGetKnownUsers(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t1, err := th.App.CreateTeam(th.Context, &model.Team{
		DisplayName: "dn_" + model.NewId(),
		Name:        GenerateTestTeamName(),
		Email:       th.GenerateTestEmail(),
		Type:        model.TEAM_OPEN,
	})
	require.Nil(t, err, "failed to create team")

	t2, err := th.App.CreateTeam(th.Context, &model.Team{
		DisplayName: "dn_" + model.NewId(),
		Name:        GenerateTestTeamName(),
		Email:       th.GenerateTestEmail(),
		Type:        model.TEAM_OPEN,
	})
	require.Nil(t, err, "failed to create team")

	t3, err := th.App.CreateTeam(th.Context, &model.Team{
		DisplayName: "dn_" + model.NewId(),
		Name:        GenerateTestTeamName(),
		Email:       th.GenerateTestEmail(),
		Type:        model.TEAM_OPEN,
	})
	require.Nil(t, err, "failed to create team")

	c1, err := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "dn_" + model.NewId(),
		Name:        "name_" + model.NewId(),
		Type:        model.CHANNEL_OPEN,
		TeamId:      t1.Id,
		CreatorId:   model.NewId(),
	}, false)
	require.Nil(t, err, "failed to create channel")

	c2, err := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "dn_" + model.NewId(),
		Name:        "name_" + model.NewId(),
		Type:        model.CHANNEL_OPEN,
		TeamId:      t2.Id,
		CreatorId:   model.NewId(),
	}, false)
	require.Nil(t, err, "failed to create channel")

	c3, err := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "dn_" + model.NewId(),
		Name:        "name_" + model.NewId(),
		Type:        model.CHANNEL_OPEN,
		TeamId:      t3.Id,
		CreatorId:   model.NewId(),
	}, false)
	require.Nil(t, err, "failed to create channel")

	u1 := th.CreateUser()
	defer th.App.PermanentDeleteUser(th.Context, u1)
	u2 := th.CreateUser()
	defer th.App.PermanentDeleteUser(th.Context, u2)
	u3 := th.CreateUser()
	defer th.App.PermanentDeleteUser(th.Context, u3)
	u4 := th.CreateUser()
	defer th.App.PermanentDeleteUser(th.Context, u4)

	th.LinkUserToTeam(u1, t1)
	th.LinkUserToTeam(u1, t2)
	th.LinkUserToTeam(u2, t1)
	th.LinkUserToTeam(u3, t2)
	th.LinkUserToTeam(u4, t3)

	th.App.AddUserToChannel(u1, c1, false)
	th.App.AddUserToChannel(u1, c2, false)
	th.App.AddUserToChannel(u2, c1, false)
	th.App.AddUserToChannel(u3, c2, false)
	th.App.AddUserToChannel(u4, c3, false)

	t.Run("get know users sharing no channels", func(t *testing.T) {
		_, _ = th.Client.Login(u4.Email, u4.Password)
		userIds, resp := th.Client.GetKnownUsers()
		CheckNoError(t, resp)
		assert.Empty(t, userIds)
	})

	t.Run("get know users sharing one channel", func(t *testing.T) {
		_, _ = th.Client.Login(u3.Email, u3.Password)
		userIds, resp := th.Client.GetKnownUsers()
		CheckNoError(t, resp)
		assert.Len(t, userIds, 1)
		assert.Equal(t, userIds[0], u1.Id)
	})

	t.Run("get know users sharing multiple channels", func(t *testing.T) {
		_, _ = th.Client.Login(u1.Email, u1.Password)
		userIds, resp := th.Client.GetKnownUsers()
		CheckNoError(t, resp)
		assert.Len(t, userIds, 2)
		assert.ElementsMatch(t, userIds, []string{u2.Id, u3.Id})
	})
}

func TestPublishUserTyping(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	tr := model.TypingRequest{
		ChannelId: th.BasicChannel.Id,
		ParentId:  "randomparentid",
	}

	t.Run("should return ok for non-system admin when triggering typing event for own user", func(t *testing.T) {
		_, resp := th.Client.PublishUserTyping(th.BasicUser.Id, tr)
		CheckNoError(t, resp)
	})

	t.Run("should return ok for system admin when triggering typing event for own user", func(t *testing.T) {
		th.LinkUserToTeam(th.SystemAdminUser, th.BasicTeam)
		th.AddUserToChannel(th.SystemAdminUser, th.BasicChannel)

		_, resp := th.SystemAdminClient.PublishUserTyping(th.SystemAdminUser.Id, tr)
		CheckNoError(t, resp)
	})

	t.Run("should return forbidden for non-system admin when triggering a typing event for a different user", func(t *testing.T) {
		_, resp := th.Client.PublishUserTyping(th.BasicUser2.Id, tr)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("should return bad request when triggering a typing event for an invalid user id", func(t *testing.T) {
		_, resp := th.Client.PublishUserTyping("invalid", tr)
		CheckErrorMessage(t, resp, "api.context.invalid_url_param.app_error")
		CheckBadRequestStatus(t, resp)
	})

	t.Run("should send typing event via websocket when triggering a typing event for a user with a common channel", func(t *testing.T) {
		webSocketClient, err := th.CreateWebSocketClient()
		assert.Nil(t, err)
		defer webSocketClient.Close()

		webSocketClient.Listen()

		time.Sleep(300 * time.Millisecond)
		wsResp := <-webSocketClient.ResponseChannel
		require.Equal(t, model.STATUS_OK, wsResp.Status)

		_, resp := th.SystemAdminClient.PublishUserTyping(th.BasicUser2.Id, tr)
		CheckNoError(t, resp)

		assertExpectedWebsocketEvent(t, webSocketClient, model.WEBSOCKET_EVENT_TYPING, func(resp *model.WebSocketEvent) {
			assert.Equal(t, th.BasicChannel.Id, resp.GetBroadcast().ChannelId)

			eventUserId, ok := resp.GetData()["user_id"].(string)
			require.True(t, ok, "expected user_id")
			assert.Equal(t, th.BasicUser2.Id, eventUserId)

			eventParentId, ok := resp.GetData()["parent_id"].(string)
			require.True(t, ok, "expected parent_id")
			assert.Equal(t, "randomparentid", eventParentId)
		})
	})

	th.Server.Busy.Set(time.Second * 10)

	t.Run("should return service unavailable for non-system admin user when triggering a typing event and server busy", func(t *testing.T) {
		_, resp := th.Client.PublishUserTyping("invalid", tr)
		CheckErrorMessage(t, resp, "api.context.server_busy.app_error")
		CheckServiceUnavailableStatus(t, resp)
	})

	t.Run("should return service unavailable for system admin user when triggering a typing event and server busy", func(t *testing.T) {
		_, resp := th.SystemAdminClient.PublishUserTyping(th.SystemAdminUser.Id, tr)
		CheckErrorMessage(t, resp, "api.context.server_busy.app_error")
		CheckServiceUnavailableStatus(t, resp)
	})
}

func TestConvertUserToBot(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	bot, resp := th.Client.ConvertUserToBot(th.BasicUser.Id)
	CheckForbiddenStatus(t, resp)
	require.Nil(t, bot)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		user := model.User{Email: th.GenerateTestEmail(), Username: GenerateTestUsername(), Password: "password"}

		ruser, resp := client.CreateUser(&user)
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		bot, resp = client.ConvertUserToBot(ruser.Id)
		CheckNoError(t, resp)
		require.NotNil(t, bot)
		require.Equal(t, bot.UserId, ruser.Id)

		bot, resp = client.GetBot(bot.UserId, "")
		CheckNoError(t, resp)
		require.NotNil(t, bot)
	})
}

func TestMigrateAuthToLDAP(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, err := th.Client.MigrateAuthToLdap("email", "a", false)
	CheckForbiddenStatus(t, err)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, err = client.MigrateAuthToLdap("email", "a", false)
		CheckNotImplementedStatus(t, err)
	})
}

func TestMigrateAuthToSAML(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, err := th.Client.MigrateAuthToSaml("email", map[string]string{"1": "a"}, true)
	CheckForbiddenStatus(t, err)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, err = client.MigrateAuthToSaml("email", map[string]string{"1": "a"}, true)
		CheckNotImplementedStatus(t, err)
	})
}
func TestUpdatePassword(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("Forbidden when request performed by system user on a system admin", func(t *testing.T) {
		res := th.Client.UpdatePassword(th.SystemAdminUser.Id, "Pa$$word11", "foobar")
		CheckForbiddenStatus(t, res)
	})

	t.Run("OK when request performed by system user with requisite system permission, except if requested user is system admin", func(t *testing.T) {
		th.AddPermissionToRole(model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_USERS.Id, model.SYSTEM_USER_ROLE_ID)
		defer th.RemovePermissionFromRole(model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_USERS.Id, model.SYSTEM_USER_ROLE_ID)

		res := th.Client.UpdatePassword(th.TeamAdminUser.Id, "Pa$$word11", "foobar")
		CheckOKStatus(t, res)

		res = th.Client.UpdatePassword(th.SystemAdminUser.Id, "Pa$$word11", "foobar")
		CheckForbiddenStatus(t, res)
	})

	t.Run("OK when request performed by system admin, even if requested user is system admin", func(t *testing.T) {
		res := th.SystemAdminClient.UpdatePassword(th.SystemAdminUser.Id, "Pa$$word11", "foobar")
		CheckOKStatus(t, res)
	})
}

func TestGetThreadsForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})
	t.Run("empty", func(t *testing.T) {
		Client := th.Client

		_, resp := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)

		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)

		uss, resp := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
		require.Nil(t, resp.Error)
		require.Len(t, uss.Threads, 0)
	})

	t.Run("no params, 1 thread", func(t *testing.T) {
		Client := th.Client

		rpost, resp := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)
		_, resp2 := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})
		CheckNoError(t, resp2)
		CheckCreatedStatus(t, resp2)

		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)

		uss, resp := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
		require.Nil(t, resp.Error)
		require.Len(t, uss.Threads, 1)
		require.Equal(t, uss.Threads[0].PostId, rpost.Id)
		require.Equal(t, uss.Threads[0].ReplyCount, int64(1))
	})

	t.Run("extended, 1 thread", func(t *testing.T) {
		Client := th.Client

		rpost, resp := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)
		_, resp2 := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})
		CheckNoError(t, resp2)
		CheckCreatedStatus(t, resp2)

		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)

		uss, resp := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Extended: true,
		})
		require.Nil(t, resp.Error)
		require.Len(t, uss.Threads, 1)
		require.Equal(t, uss.Threads[0].PostId, rpost.Id)
		require.Equal(t, uss.Threads[0].ReplyCount, int64(1))
		require.Equal(t, uss.Threads[0].Participants[0].Id, th.BasicUser.Id)
	})

	t.Run("deleted, 1 thread", func(t *testing.T) {
		Client := th.Client

		rpost, resp := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)
		_, resp2 := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})
		CheckNoError(t, resp2)
		CheckCreatedStatus(t, resp2)

		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)

		uss, resp := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		require.Nil(t, resp.Error)
		require.Len(t, uss.Threads, 1)
		require.Equal(t, uss.Threads[0].PostId, rpost.Id)
		require.Equal(t, uss.Threads[0].ReplyCount, int64(1))
		require.Equal(t, uss.Threads[0].Participants[0].Id, th.BasicUser.Id)

		res, resp2 := th.Client.DeletePost(rpost.Id)
		require.True(t, res)
		require.Nil(t, resp2.Error)

		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		require.Nil(t, resp.Error)
		require.Len(t, uss.Threads, 0)

		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: true,
		})
		require.Nil(t, resp.Error)
		require.Len(t, uss.Threads, 1)
		require.Greater(t, uss.Threads[0].Post.DeleteAt, int64(0))

	})

	t.Run("paged, 30 threads", func(t *testing.T) {
		Client := th.Client

		var rootIds []*model.Post
		for i := 0; i < 30; i++ {
			time.Sleep(1)
			rpost, resp := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
			CheckNoError(t, resp)
			CheckCreatedStatus(t, resp)
			rootIds = append(rootIds, rpost)
			time.Sleep(1)
			_, resp2 := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})
			CheckNoError(t, resp2)
			CheckCreatedStatus(t, resp2)
		}

		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)

		uss, resp := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		require.Nil(t, resp.Error)
		require.Len(t, uss.Threads, 30)
		require.Len(t, rootIds, 30)
		require.Equal(t, uss.Threads[0].PostId, rootIds[29].Id)
		require.Equal(t, uss.Threads[0].ReplyCount, int64(1))
		require.Equal(t, uss.Threads[0].Participants[0].Id, th.BasicUser.Id)
	})

	t.Run("paged, 10 threads before/after", func(t *testing.T) {
		Client := th.Client

		var rootIds []*model.Post
		for i := 0; i < 30; i++ {
			time.Sleep(1)
			rpost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: fmt.Sprintf("testMsg-%d", i)})
			rootIds = append(rootIds, rpost)
			time.Sleep(1)
			postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: fmt.Sprintf("testReply-%d", i), RootId: rpost.Id})
		}
		rootId := rootIds[15].Id // middle point
		rootIdBefore := rootIds[14].Id
		rootIdAfter := rootIds[16].Id

		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)

		uss, resp := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted:  false,
			PageSize: 10,
			Before:   rootId,
		})

		require.Nil(t, resp.Error)
		require.Len(t, uss.Threads, 10)
		require.Equal(t, uss.Threads[0].PostId, rootIdBefore)

		uss2, resp2 := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted:  false,
			PageSize: 10,
			After:    rootId,
		})
		require.Nil(t, resp2.Error)
		require.Len(t, uss2.Threads, 10)

		require.Equal(t, uss2.Threads[0].PostId, rootIdAfter)

		uss3, resp3 := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted:  false,
			PageSize: 10,
			After:    rootId + "__bad",
		})
		require.Nil(t, resp3.Error)
		require.NotNil(t, uss3.Threads)
		require.Len(t, uss3.Threads, 0)
	})

	t.Run("editing or reacting to reply post does not make thread unread", func(t *testing.T) {
		Client := th.Client

		rootPost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "root post"})
		replyPost, _ := postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel.Id, Message: "reply post", RootId: rootPost.Id})
		uss, resp := th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Equal(t, uss.TotalUnreadThreads, int64(1))
		require.Equal(t, uss.Threads[0].PostId, rootPost.Id)

		_, resp = th.Client.UpdateThreadReadForUser(th.BasicUser.Id, th.BasicChannel.TeamId, rootPost.Id, model.GetMillis())
		CheckNoError(t, resp)
		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Equal(t, uss.TotalUnreadThreads, int64(0))

		// edit post
		editedReplyPostMessage := "edited " + replyPost.Message
		_, resp = th.SystemAdminClient.PatchPost(replyPost.Id, &model.PostPatch{Message: &editedReplyPostMessage})
		CheckNoError(t, resp)
		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Equal(t, uss.TotalUnreadThreads, int64(0))

		// react to post
		reaction := &model.Reaction{
			UserId:    th.SystemAdminUser.Id,
			PostId:    replyPost.Id,
			EmojiName: "smile",
		}
		_, resp = th.SystemAdminClient.SaveReaction(reaction)
		CheckNoError(t, resp)
		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Equal(t, uss.TotalUnreadThreads, int64(0))
	})
}

func TestThreadSocketEvents(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")

	th.ConfigStore.SetReadOnlyFF(false)
	defer th.ConfigStore.SetReadOnlyFF(true)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	userWSClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	defer userWSClient.Close()
	userWSClient.Listen()

	Client := th.Client

	rpost, resp := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	_, err = th.App.CreatePostAsUser(th.Context, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", UserId: th.BasicUser2.Id, RootId: rpost.Id}, th.Context.Session().Id, false)
	require.Nil(t, err)
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser2.Id)

	t.Run("Listed for update event", func(t *testing.T) {
		var caught bool
		func() {
			for {
				select {
				case ev := <-userWSClient.EventChannel:
					if ev.EventType() == model.WEBSOCKET_EVENT_THREAD_UPDATED {
						caught = true
						thread, err := model.ThreadResponseFromJson(ev.GetData()["thread"].(string))
						require.NoError(t, err)
						for _, p := range thread.Participants {
							if p.Id != th.BasicUser.Id && p.Id != th.BasicUser2.Id {
								require.Fail(t, "invalid participants")
							}
						}
					}
				case <-time.After(1 * time.Second):
					return
				}
			}
		}()
		require.Truef(t, caught, "User should have received %s event", model.WEBSOCKET_EVENT_THREAD_UPDATED)
	})

	resp = th.Client.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rpost.Id, false)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	t.Run("Listed for follow event", func(t *testing.T) {
		var caught bool
		func() {
			for {
				select {
				case ev := <-userWSClient.EventChannel:
					if ev.EventType() == model.WEBSOCKET_EVENT_THREAD_FOLLOW_CHANGED {
						caught = true
						require.Equal(t, ev.GetData()["state"], false)
						require.Equal(t, ev.GetData()["reply_count"], float64(1))
					}
				case <-time.After(1 * time.Second):
					return
				}
			}
		}()
		require.Truef(t, caught, "User should have received %s event", model.WEBSOCKET_EVENT_THREAD_FOLLOW_CHANGED)
	})

	_, resp = th.Client.UpdateThreadReadForUser(th.BasicUser.Id, th.BasicTeam.Id, rpost.Id, 123)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	t.Run("Listed for read event", func(t *testing.T) {
		var caught bool
		func() {
			for {
				select {
				case ev := <-userWSClient.EventChannel:
					if ev.EventType() == model.WEBSOCKET_EVENT_THREAD_READ_CHANGED {
						caught = true
						require.EqualValues(t, ev.GetData()["timestamp"], 123)
					}
				case <-time.After(1 * time.Second):
					return
				}
			}
		}()

		require.Truef(t, caught, "User should have received %s event", model.WEBSOCKET_EVENT_THREAD_READ_CHANGED)
	})

}

func TestFollowThreads(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("1 thread", func(t *testing.T) {
		Client := th.Client

		rpost, resp := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)
		_, resp2 := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})
		CheckNoError(t, resp2)
		CheckCreatedStatus(t, resp2)

		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)
		var uss *model.Threads
		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Len(t, uss.Threads, 1)

		resp = th.Client.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rpost.Id, false)
		CheckNoError(t, resp)
		CheckOKStatus(t, resp)

		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Len(t, uss.Threads, 0)

		resp = th.Client.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rpost.Id, true)
		CheckNoError(t, resp)
		CheckOKStatus(t, resp)

		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Len(t, uss.Threads, 1)
		require.GreaterOrEqual(t, uss.Threads[0].LastViewedAt, uss.Threads[0].LastReplyAt)

	})
}

func checkThreadListReplies(t *testing.T, th *TestHelper, client *model.Client4, userId string, expectedReplies, expectedThreads int, options *model.GetUserThreadsOpts) (*model.Threads, *model.Response) {
	opts := model.GetUserThreadsOpts{}
	if options != nil {
		opts = *options
	}
	u, r := client.GetUserThreads(userId, th.BasicTeam.Id, opts)
	CheckNoError(t, r)
	require.Len(t, u.Threads, expectedThreads)

	count := int64(0)
	sum := int64(0)
	for _, thr := range u.Threads {
		if thr.UnreadReplies > 0 {
			count += 1
		}
		sum += thr.UnreadReplies
	}
	require.EqualValues(t, expectedReplies, sum, "expectedReplies don't match")
	require.Equal(t, count, u.TotalUnreadThreads, "TotalUnreadThreads don't match")

	return u, r
}

func postAndCheck(t *testing.T, client *model.Client4, post *model.Post) (*model.Post, *model.Response) {
	p, resp := client.CreatePost(post)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	return p, resp
}

func TestMaintainUnreadRepliesInThread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	Client := th.Client
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.SystemAdminUser.Id)

	// create a post by regular user
	rpost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
	// reply with another
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})

	// regular user should have one thread with one reply
	checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 1, 1, nil)

	// add another reply by regular user
	postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply2", RootId: rpost.Id})

	// replying to the thread clears reply count, so it should be 0
	checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 0, 1, nil)

	// the other user should have 1 reply - the reply from the regular user
	checkThreadListReplies(t, th, th.SystemAdminClient, th.SystemAdminUser.Id, 1, 1, nil)

	// mark all as read for user
	resp := th.Client.UpdateThreadsReadForUser(th.BasicUser.Id, th.BasicTeam.Id)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	// reply count should be 0
	checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 0, 1, nil)

	// mark other user's read state
	_, resp = th.SystemAdminClient.UpdateThreadReadForUser(th.SystemAdminUser.Id, th.BasicTeam.Id, rpost.Id, model.GetMillis())
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	// get unread only, should return nothing
	checkThreadListReplies(t, th, th.SystemAdminClient, th.SystemAdminUser.Id, 0, 0, &model.GetUserThreadsOpts{Unread: true})

	// restore unread to an old date
	_, resp = th.SystemAdminClient.UpdateThreadReadForUser(th.SystemAdminUser.Id, th.BasicTeam.Id, rpost.Id, 123)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	// should have 2 unread replies now
	checkThreadListReplies(t, th, th.SystemAdminClient, th.SystemAdminUser.Id, 2, 1, &model.GetUserThreadsOpts{Unread: true})

}

func TestThreadCounts(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	Client := th.Client
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.SystemAdminUser.Id)

	// create a post by regular user
	rpost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
	// reply with another
	time.Sleep(1)
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})

	// create another post by regular user
	time.Sleep(1)
	rpost2, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel2.Id, Message: "testMsg1"})
	// reply with another 2 times
	time.Sleep(1)
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel2.Id, Message: "testReply2", RootId: rpost2.Id})
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel2.Id, Message: "testReply22", RootId: rpost2.Id})

	// regular user should have two threads with 3 replies total
	checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 3, 2, &model.GetUserThreadsOpts{
		Deleted: false,
	})

	// delete first thread
	th.App.Srv().Store.Post().Delete(rpost.Id, model.GetMillis(), th.BasicUser.Id)

	// we should now have 1 thread with 2 replies
	checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 2, 1, &model.GetUserThreadsOpts{
		Deleted: false,
	})
	// with Deleted we should get the same as before deleting
	checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 3, 2, &model.GetUserThreadsOpts{
		Deleted: true,
	})
}

func TestSingleThreadGet(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	Client := th.Client
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.SystemAdminUser.Id)

	// create a post by regular user
	rpost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
	// reply with another
	time.Sleep(1)
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})

	// create another thread to check that we are not returning it by mistake
	rpost2, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel2.Id, Message: "testMsg2"})
	time.Sleep(1)
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel2.Id, Message: "testReply", RootId: rpost2.Id})

	// regular user should have two threads with 3 replies total
	threads, _ := checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 2, 2, nil)

	tr, resp := th.Client.GetUserThread(th.BasicUser.Id, th.BasicTeam.Id, threads.Threads[0].PostId, false)
	CheckNoError(t, resp)
	require.NotNil(t, tr)
	require.Equal(t, threads.Threads[0].PostId, tr.PostId)
	require.Empty(t, tr.Participants[0].Username)

	tr, resp = th.Client.GetUserThread(th.BasicUser.Id, th.BasicTeam.Id, threads.Threads[0].PostId, true)
	CheckNoError(t, resp)
	require.NotEmpty(t, tr.Participants[0].Username)
}

func TestMaintainUnreadMentionsInThread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})
	checkThreadList := func(client *model.Client4, userId string, expectedMentions, expectedThreads int) (*model.Threads, *model.Response) {
		uss, resp := client.GetUserThreads(userId, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)

		require.Len(t, uss.Threads, expectedThreads)
		sum := int64(0)
		for _, thr := range uss.Threads {
			sum += thr.UnreadMentions
		}
		require.Equal(t, sum, uss.TotalUnreadMentions)
		require.EqualValues(t, expectedMentions, uss.TotalUnreadMentions)

		return uss, resp
	}

	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)
	defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.SystemAdminUser.Id)

	// create regular post
	rpost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
	// create reply and mention the original poster and another user
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply @" + th.BasicUser.Username + " and @" + th.BasicUser2.Username, RootId: rpost.Id})

	// basic user 1 was mentioned 1 time
	checkThreadList(th.Client, th.BasicUser.Id, 1, 1)
	// basic user 2 was mentioned 1 time
	checkThreadList(th.SystemAdminClient, th.BasicUser2.Id, 1, 1)

	// test self mention, shouldn't increase mention count
	postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply @" + th.BasicUser.Username, RootId: rpost.Id})
	// count shouldn't increase
	checkThreadList(th.Client, th.BasicUser.Id, 1, 1)

	// test DM
	dm := th.CreateDmChannel(th.SystemAdminUser)
	dm_root_post, _ := postAndCheck(t, Client, &model.Post{ChannelId: dm.Id, Message: "hi @" + th.SystemAdminUser.Username})

	// no changes
	checkThreadList(th.Client, th.BasicUser.Id, 1, 1)

	// post reply by the same user
	postAndCheck(t, Client, &model.Post{ChannelId: dm.Id, Message: "how are you", RootId: dm_root_post.Id})

	// thread created
	checkThreadList(th.Client, th.BasicUser.Id, 1, 2)

	// post two replies by another user, without mentions. mention count should still increase since this is a DM
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: dm.Id, Message: "msg1", RootId: dm_root_post.Id})
	postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: dm.Id, Message: "msg2", RootId: dm_root_post.Id})
	// expect increment by two mentions
	checkThreadList(th.Client, th.BasicUser.Id, 3, 2)
}

func TestReadThreads(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})
	Client := th.Client
	t.Run("all threads", func(t *testing.T) {

		rpost, resp := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg"})
		CheckNoError(t, resp)
		CheckCreatedStatus(t, resp)
		_, resp2 := Client.CreatePost(&model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply", RootId: rpost.Id})
		CheckNoError(t, resp2)
		CheckCreatedStatus(t, resp2)
		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)

		var uss, uss2 *model.Threads
		uss, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Len(t, uss.Threads, 1)

		time.Sleep(1)
		resp = th.Client.UpdateThreadsReadForUser(th.BasicUser.Id, th.BasicTeam.Id)
		CheckNoError(t, resp)
		CheckOKStatus(t, resp)

		uss2, resp = th.Client.GetUserThreads(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{
			Deleted: false,
		})
		CheckNoError(t, resp)
		require.Len(t, uss2.Threads, 1)
		require.Greater(t, uss2.Threads[0].LastViewedAt, uss.Threads[0].LastViewedAt)
	})

	t.Run("1 thread", func(t *testing.T) {
		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.BasicUser.Id)
		defer th.App.Srv().Store.Post().PermanentDeleteByUser(th.SystemAdminUser.Id)

		rpost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsgC1"})
		postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReplyC1", RootId: rpost.Id})

		rrpost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel2.Id, Message: "testMsgC2"})
		postAndCheck(t, th.SystemAdminClient, &model.Post{ChannelId: th.BasicChannel2.Id, Message: "testReplyC2", RootId: rrpost.Id})

		uss, _ := checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 2, 2, nil)

		_, resp := th.Client.UpdateThreadReadForUser(th.BasicUser.Id, th.BasicTeam.Id, rrpost.Id, model.GetMillis()+10)
		CheckNoError(t, resp)
		CheckOKStatus(t, resp)

		uss2, _ := checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 1, 2, nil)
		require.Greater(t, uss2.Threads[0].LastViewedAt, uss.Threads[0].LastViewedAt)

		timestamp := model.GetMillis()
		_, resp = th.Client.UpdateThreadReadForUser(th.BasicUser.Id, th.BasicTeam.Id, rrpost.Id, timestamp)
		CheckNoError(t, resp)
		CheckOKStatus(t, resp)

		uss3, _ := checkThreadListReplies(t, th, th.Client, th.BasicUser.Id, 1, 2, nil)
		require.Equal(t, uss3.Threads[0].LastViewedAt, timestamp)
	})
}

func TestMarkThreadUnreadMentionCount(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})
	Client := th.Client

	channel := th.BasicChannel
	user := th.BasicUser
	user2 := th.BasicUser2
	appErr := th.App.JoinChannel(th.Context, channel, user.Id)
	require.Nil(t, appErr)
	appErr = th.App.JoinChannel(th.Context, channel, user2.Id)
	require.Nil(t, appErr)

	rpost, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testMsg @" + th.BasicUser2.Username})
	time.Sleep(1)
	reply, _ := postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply1", RootId: rpost.Id})
	time.Sleep(1)
	postAndCheck(t, Client, &model.Post{ChannelId: th.BasicChannel.Id, Message: "testReply2", RootId: rpost.Id})

	th.SystemAdminClient.UpdateThreadReadForUser(th.BasicUser2.Id, th.BasicTeam.Id, rpost.Id, model.GetMillis())

	u, _ := th.SystemAdminClient.GetUserThreads(th.BasicUser2.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
	require.EqualValues(t, 0, u.TotalUnreadMentions)

	th.SystemAdminClient.UpdateThreadReadForUser(th.BasicUser2.Id, th.BasicTeam.Id, rpost.Id, rpost.CreateAt)

	u, _ = th.SystemAdminClient.GetUserThreads(th.BasicUser2.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
	require.EqualValues(t, 1, u.TotalUnreadMentions)

	th.SystemAdminClient.UpdateThreadReadForUser(th.BasicUser2.Id, th.BasicTeam.Id, rpost.Id, reply.CreateAt)

	u, _ = th.SystemAdminClient.GetUserThreads(th.BasicUser2.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
	require.EqualValues(t, 0, u.TotalUnreadMentions)
}

func TestPatchAndUpdateWithProviderAttributes(t *testing.T) {
	t.Run("LDAP user", func(t *testing.T) {
		th := SetupEnterprise(t).InitBasic()
		defer th.TearDown()
		user := th.CreateUserWithAuth(model.USER_AUTH_SERVICE_LDAP)
		ldapMock := &mocks.LdapInterface{}
		ldapMock.Mock.On(
			"CheckProviderAttributes",
			mock.Anything, // app.AppIface
			mock.Anything, // *model.User
			mock.Anything, // *model.Patch
		).Return("")
		th.App.Srv().Ldap = ldapMock
		// CheckProviderAttributes should be called for both Patch and Update
		th.SystemAdminClient.PatchUser(user.Id, &model.UserPatch{})
		ldapMock.AssertNumberOfCalls(t, "CheckProviderAttributes", 1)
		th.SystemAdminClient.UpdateUser(user)
		ldapMock.AssertNumberOfCalls(t, "CheckProviderAttributes", 2)
	})
	t.Run("SAML user", func(t *testing.T) {
		t.Run("with LDAP sync", func(t *testing.T) {
			th := SetupEnterprise(t).InitBasic()
			defer th.TearDown()
			th.SetupLdapConfig()
			th.SetupSamlConfig()
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.SamlSettings.EnableSyncWithLdap = true
			})
			user := th.CreateUserWithAuth(model.USER_AUTH_SERVICE_SAML)
			ldapMock := &mocks.LdapInterface{}
			ldapMock.Mock.On(
				"CheckProviderAttributes", mock.Anything, mock.Anything, mock.Anything,
			).Return("")
			th.App.Srv().Ldap = ldapMock
			th.SystemAdminClient.PatchUser(user.Id, &model.UserPatch{})
			ldapMock.AssertNumberOfCalls(t, "CheckProviderAttributes", 1)
			th.SystemAdminClient.UpdateUser(user)
			ldapMock.AssertNumberOfCalls(t, "CheckProviderAttributes", 2)
		})
		t.Run("without LDAP sync", func(t *testing.T) {
			th := SetupEnterprise(t).InitBasic()
			defer th.TearDown()
			user := th.CreateUserWithAuth(model.USER_AUTH_SERVICE_SAML)
			samlMock := &mocks.SamlInterface{}
			samlMock.Mock.On(
				"CheckProviderAttributes", mock.Anything, mock.Anything, mock.Anything,
			).Return("")
			th.App.Srv().Saml = samlMock
			th.SystemAdminClient.PatchUser(user.Id, &model.UserPatch{})
			samlMock.AssertNumberOfCalls(t, "CheckProviderAttributes", 1)
			th.SystemAdminClient.UpdateUser(user)
			samlMock.AssertNumberOfCalls(t, "CheckProviderAttributes", 2)
		})
	})
	t.Run("OpenID user", func(t *testing.T) {
		th := SetupEnterprise(t).InitBasic()
		defer th.TearDown()
		user := th.CreateUserWithAuth(model.SERVICE_OPENID)
		// OAUTH users cannot change these fields
		for _, fieldName := range []string{
			"FirstName",
			"LastName",
		} {
			patch := user.ToPatch()
			patch.SetField(fieldName, "something new")
			conflictField := th.App.CheckProviderAttributes(user, patch)
			require.NotEqual(t, "", conflictField)
		}
	})
	t.Run("Patch username", func(t *testing.T) {
		th := SetupEnterprise(t).InitBasic()
		defer th.TearDown()
		// For non-email users, the username must be changed through the provider
		for _, authService := range []string{
			model.USER_AUTH_SERVICE_LDAP,
			model.USER_AUTH_SERVICE_SAML,
			model.SERVICE_OPENID,
		} {
			user := th.CreateUserWithAuth(authService)
			patch := &model.UserPatch{Username: model.NewString("something new")}
			conflictField := th.App.CheckProviderAttributes(user, patch)
			require.NotEqual(t, "", conflictField)
		}
	})
}

func TestSetProfileImageWithProviderAttributes(t *testing.T) {
	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	type imageTestCase struct {
		testName      string
		ldapAttrIsSet bool
		shouldPass    bool
	}

	doImageTest := func(t *testing.T, th *TestHelper, user *model.User, testCase imageTestCase) {
		client := th.SystemAdminClient
		t.Run(testCase.testName, func(t *testing.T) {
			th.App.UpdateConfig(func(cfg *model.Config) {
				if testCase.ldapAttrIsSet {
					*cfg.LdapSettings.PictureAttribute = "jpegPhoto"
				} else {
					*cfg.LdapSettings.PictureAttribute = ""
				}
			})
			ok, resp := client.SetProfileImage(user.Id, data)
			if testCase.shouldPass {
				require.True(t, ok)
				CheckNoError(t, resp)
			} else {
				require.False(t, ok)
				checkHTTPStatus(t, resp, http.StatusConflict, true)
			}
		})
	}
	doCleanup := func(t *testing.T, th *TestHelper, user *model.User) {
		info := &model.FileInfo{Path: "users/" + user.Id + "/profile.png"}
		err = th.cleanupTestFile(info)
		require.NoError(t, err)
	}

	t.Run("LDAP user", func(t *testing.T) {
		testCases := []imageTestCase{
			{"profile picture attribute is set", true, false},
			{"profile picture attribute is not set", false, true},
		}
		th := SetupEnterprise(t).InitBasic()
		defer th.TearDown()
		th.SetupLdapConfig()
		user := th.CreateUserWithAuth(model.USER_AUTH_SERVICE_LDAP)
		for _, testCase := range testCases {
			doImageTest(t, th, user, testCase)
		}
		doCleanup(t, th, user)
	})

	t.Run("SAML user", func(t *testing.T) {
		th := SetupEnterprise(t).InitBasic()
		defer th.TearDown()
		th.SetupLdapConfig()
		th.SetupSamlConfig()
		user := th.CreateUserWithAuth(model.USER_AUTH_SERVICE_SAML)

		t.Run("with LDAP sync", func(t *testing.T) {
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.SamlSettings.EnableSyncWithLdap = true
			})
			testCases := []imageTestCase{
				{"profile picture attribute is set", true, false},
				{"profile picture attribute is not set", false, true},
			}
			for _, testCase := range testCases {
				doImageTest(t, th, user, testCase)
			}
		})
		t.Run("without LDAP sync", func(t *testing.T) {
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.SamlSettings.EnableSyncWithLdap = false
			})
			testCases := []imageTestCase{
				{"profile picture attribute is set", true, true},
				{"profile picture attribute is not set", false, true},
			}
			for _, testCase := range testCases {
				doImageTest(t, th, user, testCase)
			}
		})
		doCleanup(t, th, user)
	})
}
