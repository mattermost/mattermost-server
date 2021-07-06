// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/einterfaces/mocks"
	"github.com/mattermost/mattermost-server/v5/model"
	oauthgitlab "github.com/mattermost/mattermost-server/v5/model/gitlab"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils/testutils"
)

func TestCreateOAuthUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.GitLabSettings.Enable = true
	})

	t.Run("create user successfully", func(t *testing.T) {
		glUser := oauthgitlab.GitLabUser{Id: 42, Username: "o" + model.NewId(), Email: model.NewId() + "@simulator.amazonses.com", Name: "Joram Wilander"}
		json := glUser.ToJson()

		user, err := th.App.CreateOAuthUser(th.Context, model.USER_AUTH_SERVICE_GITLAB, strings.NewReader(json), th.BasicTeam.Id, nil)
		require.Nil(t, err)

		require.Equal(t, glUser.Username, user.Username, "usernames didn't match")

		th.App.PermanentDeleteUser(th.Context, user)
	})

	t.Run("user exists, update authdata successfully", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.Office365Settings.Enable = true
		})

		dbUser := th.BasicUser

		// mock oAuth Provider, return data
		mockUser := &model.User{Id: "abcdef", AuthData: model.NewString("e7110007-64be-43d8-9840-4a7e9c26b710"), Email: dbUser.Email}
		providerMock := &mocks.OauthProvider{}
		providerMock.On("IsSameUser", mock.Anything, mock.Anything).Return(true)
		providerMock.On("GetUserFromJson", mock.Anything, mock.Anything).Return(mockUser, nil)
		einterfaces.RegisterOauthProvider(model.SERVICE_OFFICE365, providerMock)

		// Update user to be OAuth, formatting to match Office365 OAuth data
		s, er2 := th.App.Srv().Store.User().UpdateAuthData(dbUser.Id, model.SERVICE_OFFICE365, model.NewString("e711000764be43d898404a7e9c26b710"), "", false)
		assert.NoError(t, er2)
		assert.Equal(t, dbUser.Id, s)

		// data passed doesn't matter as return is mocked
		_, err := th.App.CreateOAuthUser(th.Context, model.SERVICE_OFFICE365, strings.NewReader("{}"), th.BasicTeam.Id, nil)
		assert.Nil(t, err)
		u, er := th.App.Srv().Store.User().GetByEmail(dbUser.Email)
		assert.NoError(t, er)
		// make sure authdata is updated
		assert.Equal(t, "e7110007-64be-43d8-9840-4a7e9c26b710", *u.AuthData)
	})

	t.Run("user creation disabled", func(t *testing.T) {
		*th.App.Config().TeamSettings.EnableUserCreation = false
		_, err := th.App.CreateOAuthUser(th.Context, model.USER_AUTH_SERVICE_GITLAB, strings.NewReader("{}"), th.BasicTeam.Id, nil)
		require.NotNil(t, err, "should have failed - user creation disabled")
	})
}

func TestSetDefaultProfileImage(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	err := th.App.SetDefaultProfileImage(&model.User{
		Id:       model.NewId(),
		Username: "notvaliduser",
	})
	// It doesn't fail, but it does nothing
	require.Nil(t, err)

	user := th.BasicUser

	err = th.App.SetDefaultProfileImage(user)
	require.Nil(t, err)

	user = getUserFromDB(th.App, user.Id, t)
	assert.Equal(t, int64(0), user.LastPictureUpdate)
}

func TestAdjustProfileImage(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, err := th.App.AdjustImage(bytes.NewReader([]byte{}))
	require.NotNil(t, err)

	// test image isn't the correct dimensions
	// it should be adjusted
	testjpg, error := testutils.ReadTestFile("testjpg.jpg")
	require.NoError(t, error)
	adjusted, err := th.App.AdjustImage(bytes.NewReader(testjpg))
	require.Nil(t, err)
	assert.True(t, adjusted.Len() > 0)
	assert.NotEqual(t, testjpg, adjusted)

	// default image should require adjustement
	user := th.BasicUser
	image, err := th.App.GetDefaultProfileImage(user)
	require.Nil(t, err)
	image2, err := th.App.AdjustImage(bytes.NewReader(image))
	require.Nil(t, err)
	assert.Equal(t, image, image2.Bytes())
}

func TestUpdateUserToRestrictedDomain(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := th.CreateUser()
	defer th.App.PermanentDeleteUser(th.Context, user)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.RestrictCreationToDomains = "foo.com"
	})

	_, err := th.App.UpdateUser(user, false)
	assert.Nil(t, err)

	user.Email = "asdf@ghjk.l"
	_, err = th.App.UpdateUser(user, false)
	assert.NotNil(t, err)

	t.Run("Restricted Domains must be ignored for guest users", func(t *testing.T) {
		guest := th.CreateGuest()
		defer th.App.PermanentDeleteUser(th.Context, guest)

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.RestrictCreationToDomains = "foo.com"
		})

		guest.Email = "asdf@bar.com"
		updatedGuest, err := th.App.UpdateUser(guest, false)
		require.Nil(t, err)
		require.Equal(t, guest.Email, updatedGuest.Email)
	})

	t.Run("Guest users should be affected by guest restricted domains", func(t *testing.T) {
		guest := th.CreateGuest()
		defer th.App.PermanentDeleteUser(th.Context, guest)

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.GuestAccountsSettings.RestrictCreationToDomains = "foo.com"
		})

		guest.Email = "asdf@bar.com"
		_, err := th.App.UpdateUser(guest, false)
		require.NotNil(t, err)

		guest.Email = "asdf@foo.com"
		updatedGuest, err := th.App.UpdateUser(guest, false)
		require.Nil(t, err)
		require.Equal(t, guest.Email, updatedGuest.Email)
	})
}

func TestUpdateUserActive(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := th.CreateUser()

	EnableUserDeactivation := th.App.Config().TeamSettings.EnableUserDeactivation
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.EnableUserDeactivation = EnableUserDeactivation })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.EnableUserDeactivation = true
	})
	err := th.App.UpdateUserActive(th.Context, user.Id, false)
	assert.Nil(t, err)
}

func TestUpdateActiveBotsSideEffect(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	bot, err := th.App.CreateBot(th.Context, &model.Bot{
		Username:    "username",
		Description: "a bot",
		OwnerId:     th.BasicUser.Id,
	})
	require.Nil(t, err)
	defer th.App.PermanentDeleteBot(bot.UserId)

	// Automatic deactivation disabled
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.DisableBotsWhenOwnerIsDeactivated = false
	})

	th.App.UpdateActive(th.Context, th.BasicUser, false)

	retbot1, err := th.App.GetBot(bot.UserId, true)
	require.Nil(t, err)
	require.Zero(t, retbot1.DeleteAt)
	user1, err := th.App.GetUser(bot.UserId)
	require.Nil(t, err)
	require.Zero(t, user1.DeleteAt)

	th.App.UpdateActive(th.Context, th.BasicUser, true)

	// Automatic deactivation enabled
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.DisableBotsWhenOwnerIsDeactivated = true
	})

	th.App.UpdateActive(th.Context, th.BasicUser, false)

	retbot2, err := th.App.GetBot(bot.UserId, true)
	require.Nil(t, err)
	require.NotZero(t, retbot2.DeleteAt)
	user2, err := th.App.GetUser(bot.UserId)
	require.Nil(t, err)
	require.NotZero(t, user2.DeleteAt)

	th.App.UpdateActive(th.Context, th.BasicUser, true)
}

func TestUpdateOAuthUserAttrs(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	id := model.NewId()
	id2 := model.NewId()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.GitLabSettings.Enable = true
	})
	gitlabProvider := einterfaces.GetOauthProvider("gitlab")

	username := "user" + id
	username2 := "user" + id2

	email := "user" + id + "@nowhere.com"
	email2 := "user" + id2 + "@nowhere.com"

	var user, user2 *model.User
	var gitlabUserObj oauthgitlab.GitLabUser
	user, gitlabUserObj = createGitlabUser(t, th.App, th.Context, 1, username, email)
	user2, _ = createGitlabUser(t, th.App, th.Context, 2, username2, email2)

	t.Run("UpdateUsername", func(t *testing.T) {
		t.Run("NoExistingUserWithSameUsername", func(t *testing.T) {
			gitlabUserObj.Username = "updateduser" + model.NewId()
			gitlabUser := getGitlabUserPayload(gitlabUserObj, t)
			data := bytes.NewReader(gitlabUser)

			user = getUserFromDB(th.App, user.Id, t)
			th.App.UpdateOAuthUserAttrs(data, user, gitlabProvider, "gitlab", nil)
			user = getUserFromDB(th.App, user.Id, t)

			require.Equal(t, gitlabUserObj.Username, user.Username, "user's username is not updated")
		})

		t.Run("ExistinguserWithSameUsername", func(t *testing.T) {
			gitlabUserObj.Username = user2.Username

			gitlabUser := getGitlabUserPayload(gitlabUserObj, t)
			data := bytes.NewReader(gitlabUser)

			user = getUserFromDB(th.App, user.Id, t)
			th.App.UpdateOAuthUserAttrs(data, user, gitlabProvider, "gitlab", nil)
			user = getUserFromDB(th.App, user.Id, t)

			require.NotEqual(t, gitlabUserObj.Username, user.Username, "user's username is updated though there already exists another user with the same username")
		})
	})

	t.Run("UpdateEmail", func(t *testing.T) {
		t.Run("NoExistingUserWithSameEmail", func(t *testing.T) {
			gitlabUserObj.Email = "newuser" + model.NewId() + "@nowhere.com"
			gitlabUser := getGitlabUserPayload(gitlabUserObj, t)
			data := bytes.NewReader(gitlabUser)

			user = getUserFromDB(th.App, user.Id, t)
			th.App.UpdateOAuthUserAttrs(data, user, gitlabProvider, "gitlab", nil)
			user = getUserFromDB(th.App, user.Id, t)

			require.Equal(t, gitlabUserObj.Email, user.Email, "user's email is not updated")

			require.True(t, user.EmailVerified, "user's email should have been verified")
		})

		t.Run("ExistingUserWithSameEmail", func(t *testing.T) {
			gitlabUserObj.Email = user2.Email

			gitlabUser := getGitlabUserPayload(gitlabUserObj, t)
			data := bytes.NewReader(gitlabUser)

			user = getUserFromDB(th.App, user.Id, t)
			th.App.UpdateOAuthUserAttrs(data, user, gitlabProvider, "gitlab", nil)
			user = getUserFromDB(th.App, user.Id, t)

			require.NotEqual(t, gitlabUserObj.Email, user.Email, "user's email is updated though there already exists another user with the same email")
		})
	})

	t.Run("UpdateFirstName", func(t *testing.T) {
		gitlabUserObj.Name = "Updated User"
		gitlabUser := getGitlabUserPayload(gitlabUserObj, t)
		data := bytes.NewReader(gitlabUser)

		user = getUserFromDB(th.App, user.Id, t)
		th.App.UpdateOAuthUserAttrs(data, user, gitlabProvider, "gitlab", nil)
		user = getUserFromDB(th.App, user.Id, t)

		require.Equal(t, "Updated", user.FirstName, "user's first name is not updated")
	})

	t.Run("UpdateLastName", func(t *testing.T) {
		gitlabUserObj.Name = "Updated Lastname"
		gitlabUser := getGitlabUserPayload(gitlabUserObj, t)
		data := bytes.NewReader(gitlabUser)

		user = getUserFromDB(th.App, user.Id, t)
		th.App.UpdateOAuthUserAttrs(data, user, gitlabProvider, "gitlab", nil)
		user = getUserFromDB(th.App, user.Id, t)

		require.Equal(t, "Lastname", user.LastName, "user's last name is not updated")
	})
}

func TestCreateUserConflict(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := &model.User{
		Email:    "test@localhost",
		Username: model.NewId(),
	}
	user, err := th.App.Srv().Store.User().Save(user)
	require.NoError(t, err)
	username := user.Username

	var invErr *store.ErrInvalidInput
	// Same id
	_, err = th.App.Srv().Store.User().Save(user)
	require.Error(t, err)
	require.True(t, errors.As(err, &invErr))
	assert.Equal(t, "id", invErr.Field)

	// Same email
	user = &model.User{
		Email:    "test@localhost",
		Username: model.NewId(),
	}
	_, err = th.App.Srv().Store.User().Save(user)
	require.Error(t, err)
	require.True(t, errors.As(err, &invErr))
	assert.Equal(t, "email", invErr.Field)

	// Same username
	user = &model.User{
		Email:    "test2@localhost",
		Username: username,
	}
	_, err = th.App.Srv().Store.User().Save(user)
	require.Error(t, err)
	require.True(t, errors.As(err, &invErr))
	assert.Equal(t, "username", invErr.Field)
}

func TestUpdateUserEmail(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user := th.CreateUser()

	t.Run("RequireVerification", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.EmailSettings.RequireEmailVerification = true
		})

		currentEmail := user.Email
		newEmail := th.MakeEmail()

		user.Email = newEmail
		user2, appErr := th.App.UpdateUser(user, false)
		assert.Nil(t, appErr)
		assert.Equal(t, currentEmail, user2.Email)
		assert.True(t, user2.EmailVerified)

		token, err := th.App.Srv().EmailService.CreateVerifyEmailToken(user2.Id, newEmail)
		assert.NoError(t, err)

		appErr = th.App.VerifyEmailFromToken(token.Token)
		assert.Nil(t, appErr)

		user2, appErr = th.App.GetUser(user2.Id)
		assert.Nil(t, appErr)
		assert.Equal(t, newEmail, user2.Email)
		assert.True(t, user2.EmailVerified)

		// Create bot user
		botuser := model.User{
			Email:    "botuser@localhost",
			Username: model.NewId(),
			IsBot:    true,
		}
		_, nErr := th.App.Srv().Store.User().Save(&botuser)
		assert.NoError(t, nErr)

		newBotEmail := th.MakeEmail()
		botuser.Email = newBotEmail
		botuser2, appErr := th.App.UpdateUser(&botuser, false)
		assert.Nil(t, appErr)
		assert.Equal(t, botuser2.Email, newBotEmail)

	})

	t.Run("RequireVerificationAlreadyUsedEmail", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.EmailSettings.RequireEmailVerification = true
		})

		user2 := th.CreateUser()
		newEmail := user2.Email

		user.Email = newEmail
		user3, err := th.App.UpdateUser(user, false)
		require.NotNil(t, err)
		assert.Equal(t, err.Id, "app.user.save.email_exists.app_error")
		assert.Nil(t, user3)
	})

	t.Run("NoVerification", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.EmailSettings.RequireEmailVerification = false
		})

		newEmail := th.MakeEmail()

		user.Email = newEmail
		user2, err := th.App.UpdateUser(user, false)
		assert.Nil(t, err)
		assert.Equal(t, newEmail, user2.Email)

		// Create bot user
		botuser := model.User{
			Email:    "botuser@localhost",
			Username: model.NewId(),
			IsBot:    true,
		}
		_, nErr := th.App.Srv().Store.User().Save(&botuser)
		assert.NoError(t, nErr)

		newBotEmail := th.MakeEmail()
		botuser.Email = newBotEmail
		botuser2, err := th.App.UpdateUser(&botuser, false)
		assert.Nil(t, err)
		assert.Equal(t, botuser2.Email, newBotEmail)
	})

	t.Run("NoVerificationAlreadyUsedEmail", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.EmailSettings.RequireEmailVerification = false
		})

		user2 := th.CreateUser()
		newEmail := user2.Email

		user.Email = newEmail
		user3, err := th.App.UpdateUser(user, false)
		require.NotNil(t, err)
		assert.Equal(t, err.Id, "app.user.save.email_exists.app_error")
		assert.Nil(t, user3)
	})
}

func getUserFromDB(a *App, id string, t *testing.T) *model.User {
	user, err := a.GetUser(id)
	require.Nil(t, err, "user is not found", err)
	return user
}

func getGitlabUserPayload(gitlabUser oauthgitlab.GitLabUser, t *testing.T) []byte {
	var payload []byte
	var err error
	payload, err = json.Marshal(gitlabUser)
	require.NoError(t, err, "Serialization of gitlab user to json failed", err)

	return payload
}

func createGitlabUser(t *testing.T, a *App, c *request.Context, id int64, username string, email string) (*model.User, oauthgitlab.GitLabUser) {
	gitlabUserObj := oauthgitlab.GitLabUser{Id: id, Username: username, Login: "user1", Email: email, Name: "Test User"}
	gitlabUser := getGitlabUserPayload(gitlabUserObj, t)

	var user *model.User
	var err *model.AppError

	user, err = a.CreateOAuthUser(c, "gitlab", bytes.NewReader(gitlabUser), "", nil)
	require.Nil(t, err, "unable to create the user", err)

	return user, gitlabUserObj
}

func TestGetUsersByStatus(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	team := th.CreateTeam()
	channel, err := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "dn_" + model.NewId(),
		Name:        "name_" + model.NewId(),
		Type:        model.CHANNEL_OPEN,
		TeamId:      team.Id,
		CreatorId:   model.NewId(),
	}, false)
	require.Nil(t, err, "failed to create channel: %v", err)

	createUserWithStatus := func(username string, status string) *model.User {
		id := model.NewId()

		user, err := th.App.CreateUser(th.Context, &model.User{
			Email:    "success+" + id + "@simulator.amazonses.com",
			Username: "un_" + username + "_" + id,
			Nickname: "nn_" + id,
			Password: "Password1",
		})
		require.Nil(t, err, "failed to create user: %v", err)

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
	awayUser1 := createUserWithStatus("away1", model.STATUS_AWAY)
	awayUser2 := createUserWithStatus("away2", model.STATUS_AWAY)
	dndUser1 := createUserWithStatus("dnd1", model.STATUS_DND)
	dndUser2 := createUserWithStatus("dnd2", model.STATUS_DND)
	offlineUser1 := createUserWithStatus("offline1", model.STATUS_OFFLINE)
	offlineUser2 := createUserWithStatus("offline2", model.STATUS_OFFLINE)
	onlineUser1 := createUserWithStatus("online1", model.STATUS_ONLINE)
	onlineUser2 := createUserWithStatus("online2", model.STATUS_ONLINE)

	t.Run("sorting by status then alphabetical", func(t *testing.T) {
		usersByStatus, err := th.App.GetUsersInChannelPageByStatus(&model.UserGetOptions{
			InChannelId: channel.Id,
			Page:        0,
			PerPage:     8,
		}, true)
		require.Nil(t, err)

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

		require.Equalf(t, len(expectedUsersByStatus), len(usersByStatus), "received only %v users, expected %v", len(usersByStatus), len(expectedUsersByStatus))

		for i := range usersByStatus {
			require.Equalf(t, expectedUsersByStatus[i].Id, usersByStatus[i].Id, "received user %v at index %v, expected %v", usersByStatus[i].Username, i, expectedUsersByStatus[i].Username)
		}
	})

	t.Run("paging", func(t *testing.T) {
		usersByStatus, err := th.App.GetUsersInChannelPageByStatus(&model.UserGetOptions{
			InChannelId: channel.Id,
			Page:        0,
			PerPage:     3,
		}, true)
		require.Nil(t, err)

		require.Equal(t, 3, len(usersByStatus), "received too many users")

		require.False(
			t,
			usersByStatus[0].Id != onlineUser1.Id && usersByStatus[1].Id != onlineUser2.Id,
			"expected to receive online users first",
		)

		require.Equal(t, awayUser1.Id, usersByStatus[2].Id, "expected to receive away users second")

		usersByStatus, err = th.App.GetUsersInChannelPageByStatus(&model.UserGetOptions{
			InChannelId: channel.Id,
			Page:        1,
			PerPage:     3,
		}, true)
		require.Nil(t, err)

		require.NotEmpty(t, usersByStatus, "at least some users are expected")
		require.Equal(t, awayUser2.Id, usersByStatus[0].Id, "expected to receive away users second")

		require.False(
			t,
			usersByStatus[1].Id != dndUser1.Id && usersByStatus[2].Id != dndUser2.Id,
			"expected to receive dnd users third",
		)

		usersByStatus, err = th.App.GetUsersInChannelPageByStatus(&model.UserGetOptions{
			InChannelId: channel.Id,
			Page:        1,
			PerPage:     4,
		}, true)
		require.Nil(t, err)

		require.Equal(t, 4, len(usersByStatus), "received too many users")

		require.False(
			t,
			usersByStatus[0].Id != dndUser1.Id && usersByStatus[1].Id != dndUser2.Id,
			"expected to receive dnd users third",
		)

		require.False(
			t,
			usersByStatus[2].Id != offlineUser1.Id && usersByStatus[3].Id != offlineUser2.Id,
			"expected to receive offline users last",
		)
	})
}

func TestCreateUserWithInviteId(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}

	t.Run("should create a user", func(t *testing.T) {
		u, err := th.App.CreateUserWithInviteId(th.Context, &user, th.BasicTeam.InviteId, "")
		require.Nil(t, err)
		require.Equal(t, u.Id, user.Id)
	})

	t.Run("invalid invite id", func(t *testing.T) {
		_, err := th.App.CreateUserWithInviteId(th.Context, &user, "", "")
		require.NotNil(t, err)
		require.Contains(t, err.Id, "app.team.get_by_invite_id")
	})

	t.Run("invalid domain", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "mattermost.com"
		_, nErr := th.App.Srv().Store.Team().Update(th.BasicTeam)
		require.NoError(t, nErr)
		_, err := th.App.CreateUserWithInviteId(th.Context, &user, th.BasicTeam.InviteId, "")
		require.NotNil(t, err)
		require.Equal(t, "api.team.invite_members.invalid_email.app_error", err.Id)
	})
}

func TestCreateUserWithToken(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}

	t.Run("invalid token", func(t *testing.T) {
		_, err := th.App.CreateUserWithToken(th.Context, &user, &model.Token{Token: "123"})
		require.NotNil(t, err, "Should fail on unexisting token")
	})

	t.Run("invalid token type", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeVerifyEmail,
			model.MapToJson(map[string]string{"teamID": th.BasicTeam.Id, "email": user.Email}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)
		_, err := th.App.CreateUserWithToken(th.Context, &user, token)
		require.NotNil(t, err, "Should fail on bad token type")
	})

	t.Run("expired token", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": user.Email}),
		)
		token.CreateAt = model.GetMillis() - InvitationExpiryTime - 1
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)
		_, err := th.App.CreateUserWithToken(th.Context, &user, token)
		require.NotNil(t, err, "Should fail on expired token")
	})

	t.Run("invalid team id", func(t *testing.T) {
		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": model.NewId(), "email": user.Email}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		defer th.App.DeleteToken(token)
		_, err := th.App.CreateUserWithToken(th.Context, &user, token)
		require.NotNil(t, err, "Should fail on bad team id")
	})

	t.Run("valid regular user request", func(t *testing.T) {
		invitationEmail := model.NewId() + "other-email@test.com"
		token := model.NewToken(
			TokenTypeTeamInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": invitationEmail}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		newUser, err := th.App.CreateUserWithToken(th.Context, &user, token)
		require.Nil(t, err, "Should add user to the team. err=%v", err)
		assert.False(t, newUser.IsGuest())
		require.Equal(t, invitationEmail, newUser.Email, "The user email must be the invitation one")

		_, nErr := th.App.Srv().Store.Token().GetByToken(token.Token)
		require.Error(t, nErr, "The token must be deleted after be used")

		members, err := th.App.GetChannelMembersForUser(th.BasicTeam.Id, newUser.Id)
		require.Nil(t, err)
		assert.Len(t, *members, 2)
	})

	t.Run("valid guest request", func(t *testing.T) {
		invitationEmail := model.NewId() + "other-email@test.com"
		token := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": invitationEmail, "channels": th.BasicChannel.Id}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		guest := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
		newGuest, err := th.App.CreateUserWithToken(th.Context, &guest, token)
		require.Nil(t, err, "Should add user to the team. err=%v", err)

		assert.True(t, newGuest.IsGuest())
		require.Equal(t, invitationEmail, newGuest.Email, "The user email must be the invitation one")
		_, nErr := th.App.Srv().Store.Token().GetByToken(token.Token)
		require.Error(t, nErr, "The token must be deleted after be used")

		members, err := th.App.GetChannelMembersForUser(th.BasicTeam.Id, newGuest.Id)
		require.Nil(t, err)
		require.Len(t, *members, 1)
		assert.Equal(t, (*members)[0].ChannelId, th.BasicChannel.Id)
	})

	t.Run("create guest having email domain restrictions", func(t *testing.T) {
		enableGuestDomainRestricions := *th.App.Config().GuestAccountsSettings.RestrictCreationToDomains
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) {
				cfg.GuestAccountsSettings.RestrictCreationToDomains = &enableGuestDomainRestricions
			})
		}()
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.RestrictCreationToDomains = "restricted.com" })
		forbiddenInvitationEmail := model.NewId() + "other-email@test.com"
		grantedInvitationEmail := model.NewId() + "other-email@restricted.com"
		forbiddenDomainToken := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": forbiddenInvitationEmail, "channels": th.BasicChannel.Id}),
		)
		grantedDomainToken := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": grantedInvitationEmail, "channels": th.BasicChannel.Id}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(forbiddenDomainToken))
		require.NoError(t, th.App.Srv().Store.Token().Save(grantedDomainToken))
		guest := model.User{
			Email:       strings.ToLower(model.NewId()) + "+test@example.com",
			Nickname:    "Darth Vader",
			Username:    "vader" + model.NewId(),
			Password:    "passwd1",
			AuthService: "",
		}
		newGuest, err := th.App.CreateUserWithToken(th.Context, &guest, forbiddenDomainToken)
		require.NotNil(t, err)
		require.Nil(t, newGuest)
		assert.Equal(t, "api.user.create_user.accepted_domain.app_error", err.Id)

		newGuest, err = th.App.CreateUserWithToken(th.Context, &guest, grantedDomainToken)
		require.Nil(t, err)
		assert.True(t, newGuest.IsGuest())
		require.Equal(t, grantedInvitationEmail, newGuest.Email)
		_, nErr := th.App.Srv().Store.Token().GetByToken(grantedDomainToken.Token)
		require.Error(t, nErr)

		members, err := th.App.GetChannelMembersForUser(th.BasicTeam.Id, newGuest.Id)
		require.Nil(t, err)
		require.Len(t, *members, 1)
		assert.Equal(t, (*members)[0].ChannelId, th.BasicChannel.Id)
	})

	t.Run("create guest having team and system email domain restrictions", func(t *testing.T) {
		th.BasicTeam.AllowedDomains = "restricted-team.com"
		_, err := th.App.UpdateTeam(th.BasicTeam)
		require.Nil(t, err, "Should update the team")
		enableGuestDomainRestricions := *th.App.Config().TeamSettings.RestrictCreationToDomains
		defer func() {
			th.App.UpdateConfig(func(cfg *model.Config) {
				cfg.TeamSettings.RestrictCreationToDomains = &enableGuestDomainRestricions
			})
		}()
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.RestrictCreationToDomains = "restricted.com" })
		invitationEmail := model.NewId() + "other-email@test.com"
		token := model.NewToken(
			TokenTypeGuestInvitation,
			model.MapToJson(map[string]string{"teamId": th.BasicTeam.Id, "email": invitationEmail, "channels": th.BasicChannel.Id}),
		)
		require.NoError(t, th.App.Srv().Store.Token().Save(token))
		guest := model.User{
			Email:       strings.ToLower(model.NewId()) + "+test@example.com",
			Nickname:    "Darth Vader",
			Username:    "vader" + model.NewId(),
			Password:    "passwd1",
			AuthService: "",
		}
		newGuest, err := th.App.CreateUserWithToken(th.Context, &guest, token)
		require.Nil(t, err)
		assert.True(t, newGuest.IsGuest())
		assert.Equal(t, invitationEmail, newGuest.Email, "The user email must be the invitation one")
		_, nErr := th.App.Srv().Store.Token().GetByToken(token.Token)
		require.Error(t, nErr)

		members, err := th.App.GetChannelMembersForUser(th.BasicTeam.Id, newGuest.Id)
		require.Nil(t, err)
		require.Len(t, *members, 1)
		assert.Equal(t, (*members)[0].ChannelId, th.BasicChannel.Id)
	})
}

func TestPermanentDeleteUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	b := []byte("testimage")

	finfo, err := th.App.DoUploadFile(th.Context, time.Now(), th.BasicTeam.Id, th.BasicChannel.Id, th.BasicUser.Id, "testfile.txt", b)

	require.Nil(t, err, "Unable to upload file. err=%v", err)

	bot, err := th.App.CreateBot(th.Context, &model.Bot{
		Username:    "botname",
		Description: "a bot",
		OwnerId:     model.NewId(),
	})
	assert.Nil(t, err)

	var bots1 []*model.Bot
	var bots2 []*model.Bot

	sqlStore := mainHelper.GetSQLStore()
	_, err1 := sqlStore.GetMaster().Select(&bots1, "SELECT * FROM Bots")
	assert.NoError(t, err1)
	assert.Equal(t, 1, len(bots1))

	// test that bot is deleted from bots table
	retUser1, err := th.App.GetUser(bot.UserId)
	assert.Nil(t, err)

	err = th.App.PermanentDeleteUser(th.Context, retUser1)
	assert.Nil(t, err)

	_, err1 = sqlStore.GetMaster().Select(&bots2, "SELECT * FROM Bots")
	assert.NoError(t, err1)
	assert.Equal(t, 0, len(bots2))

	err = th.App.PermanentDeleteUser(th.Context, th.BasicUser)
	require.Nil(t, err, "Unable to delete user. err=%v", err)

	res, err := th.App.FileExists(finfo.Path)

	require.Nil(t, err, "Unable to check whether file exists. err=%v", err)

	require.False(t, res, "File was not deleted on FS. err=%v", err)

	finfo, err = th.App.GetFileInfo(finfo.Id)

	require.Nil(t, finfo, "Unable to find finfo. err=%v", err)

	require.NotNil(t, err, "GetFileInfo after DeleteUser is nil. err=%v", err)
}

func TestPasswordRecovery(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	token, err := th.App.CreatePasswordRecoveryToken(th.BasicUser.Id, th.BasicUser.Email)
	assert.Nil(t, err)

	tokenData := struct {
		UserId string
		Email  string
	}{}

	err2 := json.Unmarshal([]byte(token.Extra), &tokenData)
	assert.NoError(t, err2)
	assert.Equal(t, th.BasicUser.Id, tokenData.UserId)
	assert.Equal(t, th.BasicUser.Email, tokenData.Email)

	// Password token with same eMail as during creation
	err = th.App.ResetPasswordFromToken(token.Token, "abcdefgh")
	assert.Nil(t, err)

	// Password token with modified eMail after creation
	token, err = th.App.CreatePasswordRecoveryToken(th.BasicUser.Id, th.BasicUser.Email)
	assert.Nil(t, err)

	th.App.UpdateConfig(func(c *model.Config) {
		*c.EmailSettings.RequireEmailVerification = false
	})

	th.BasicUser.Email = th.MakeEmail()
	_, err = th.App.UpdateUser(th.BasicUser, false)
	assert.Nil(t, err)

	err = th.App.ResetPasswordFromToken(token.Token, "abcdefgh")
	assert.NotNil(t, err)
}

func TestGetViewUsersRestrictions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team1 := th.CreateTeam()
	team2 := th.CreateTeam()
	th.CreateTeam() // Another team

	user1 := th.CreateUser()

	th.LinkUserToTeam(user1, team1)
	th.LinkUserToTeam(user1, team2)

	th.App.UpdateTeamMemberRoles(team1.Id, user1.Id, "team_user team_admin")

	team1channel1 := th.CreateChannel(team1)
	team1channel2 := th.CreateChannel(team1)
	th.CreateChannel(team1) // Another channel
	team1offtopic, err := th.App.GetChannelByName("off-topic", team1.Id, false)
	require.Nil(t, err)
	team1townsquare, err := th.App.GetChannelByName("town-square", team1.Id, false)
	require.Nil(t, err)

	team2channel1 := th.CreateChannel(team2)
	th.CreateChannel(team2) // Another channel
	team2offtopic, err := th.App.GetChannelByName("off-topic", team2.Id, false)
	require.Nil(t, err)
	team2townsquare, err := th.App.GetChannelByName("town-square", team2.Id, false)
	require.Nil(t, err)

	th.App.AddUserToChannel(user1, team1channel1, false)
	th.App.AddUserToChannel(user1, team1channel2, false)
	th.App.AddUserToChannel(user1, team2channel1, false)

	addPermission := func(role *model.Role, permission string) *model.AppError {
		newPermissions := append(role.Permissions, permission)
		_, err := th.App.PatchRole(role, &model.RolePatch{Permissions: &newPermissions})
		return err
	}

	removePermission := func(role *model.Role, permission string) *model.AppError {
		newPermissions := []string{}
		for _, oldPermission := range role.Permissions {
			if permission != oldPermission {
				newPermissions = append(newPermissions, oldPermission)
			}
		}
		_, err := th.App.PatchRole(role, &model.RolePatch{Permissions: &newPermissions})
		return err
	}

	t.Run("VIEW_MEMBERS permission granted at system level", func(t *testing.T) {
		restrictions, err := th.App.GetViewUsersRestrictions(user1.Id)
		require.Nil(t, err)

		assert.Nil(t, restrictions)
	})

	t.Run("VIEW_MEMBERS permission granted at team level", func(t *testing.T) {
		systemUserRole, err := th.App.GetRoleByName(context.Background(), model.SYSTEM_USER_ROLE_ID)
		require.Nil(t, err)
		teamUserRole, err := th.App.GetRoleByName(context.Background(), model.TEAM_USER_ROLE_ID)
		require.Nil(t, err)

		require.Nil(t, removePermission(systemUserRole, model.PERMISSION_VIEW_MEMBERS.Id))
		defer addPermission(systemUserRole, model.PERMISSION_VIEW_MEMBERS.Id)
		require.Nil(t, addPermission(teamUserRole, model.PERMISSION_VIEW_MEMBERS.Id))
		defer removePermission(teamUserRole, model.PERMISSION_VIEW_MEMBERS.Id)

		restrictions, err := th.App.GetViewUsersRestrictions(user1.Id)
		require.Nil(t, err)

		assert.NotNil(t, restrictions)
		assert.NotNil(t, restrictions.Teams)
		assert.NotNil(t, restrictions.Channels)
		assert.ElementsMatch(t, []string{team1townsquare.Id, team1offtopic.Id, team1channel1.Id, team1channel2.Id, team2townsquare.Id, team2offtopic.Id, team2channel1.Id}, restrictions.Channels)
		assert.ElementsMatch(t, []string{team1.Id, team2.Id}, restrictions.Teams)
	})

	t.Run("VIEW_MEMBERS permission not granted at any level", func(t *testing.T) {
		systemUserRole, err := th.App.GetRoleByName(context.Background(), model.SYSTEM_USER_ROLE_ID)
		require.Nil(t, err)
		require.Nil(t, removePermission(systemUserRole, model.PERMISSION_VIEW_MEMBERS.Id))
		defer addPermission(systemUserRole, model.PERMISSION_VIEW_MEMBERS.Id)

		restrictions, err := th.App.GetViewUsersRestrictions(user1.Id)
		require.Nil(t, err)

		assert.NotNil(t, restrictions)
		assert.Empty(t, restrictions.Teams)
		assert.NotNil(t, restrictions.Channels)
		assert.ElementsMatch(t, []string{team1townsquare.Id, team1offtopic.Id, team1channel1.Id, team1channel2.Id, team2townsquare.Id, team2offtopic.Id, team2channel1.Id}, restrictions.Channels)
	})

	t.Run("VIEW_MEMBERS permission for some teams but not for others", func(t *testing.T) {
		systemUserRole, err := th.App.GetRoleByName(context.Background(), model.SYSTEM_USER_ROLE_ID)
		require.Nil(t, err)
		teamAdminRole, err := th.App.GetRoleByName(context.Background(), model.TEAM_ADMIN_ROLE_ID)
		require.Nil(t, err)

		require.Nil(t, removePermission(systemUserRole, model.PERMISSION_VIEW_MEMBERS.Id))
		defer addPermission(systemUserRole, model.PERMISSION_VIEW_MEMBERS.Id)
		require.Nil(t, addPermission(teamAdminRole, model.PERMISSION_VIEW_MEMBERS.Id))
		defer removePermission(teamAdminRole, model.PERMISSION_VIEW_MEMBERS.Id)

		restrictions, err := th.App.GetViewUsersRestrictions(user1.Id)
		require.Nil(t, err)

		assert.NotNil(t, restrictions)
		assert.NotNil(t, restrictions.Teams)
		assert.NotNil(t, restrictions.Channels)
		assert.ElementsMatch(t, restrictions.Teams, []string{team1.Id})
		assert.ElementsMatch(t, []string{team1townsquare.Id, team1offtopic.Id, team1channel1.Id, team1channel2.Id, team2townsquare.Id, team2offtopic.Id, team2channel1.Id}, restrictions.Channels)
	})
}

func TestPromoteGuestToUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Must fail with regular user", func(t *testing.T) {
		require.Equal(t, "system_user", th.BasicUser.Roles)
		err := th.App.PromoteGuestToUser(th.Context, th.BasicUser, th.BasicUser.Id)
		require.Nil(t, err)

		user, err := th.App.GetUser(th.BasicUser.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_user", user.Roles)
	})

	t.Run("Must work with guest user without teams or channels", func(t *testing.T) {
		guest := th.CreateGuest()
		require.Equal(t, "system_guest", guest.Roles)

		err := th.App.PromoteGuestToUser(th.Context, guest, th.BasicUser.Id)
		require.Nil(t, err)
		guest, err = th.App.GetUser(guest.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_user", guest.Roles)
	})

	t.Run("Must work with guest user with teams but no channels", func(t *testing.T) {
		guest := th.CreateGuest()
		require.Equal(t, "system_guest", guest.Roles)
		th.LinkUserToTeam(guest, th.BasicTeam)
		teamMember, err := th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeGuest)
		require.False(t, teamMember.SchemeUser)

		err = th.App.PromoteGuestToUser(th.Context, guest, th.BasicUser.Id)
		require.Nil(t, err)
		guest, err = th.App.GetUser(guest.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_user", guest.Roles)
		teamMember, err = th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeGuest)
		assert.True(t, teamMember.SchemeUser)
	})

	t.Run("Must work with guest user with teams and channels", func(t *testing.T) {
		guest := th.CreateGuest()
		require.Equal(t, "system_guest", guest.Roles)
		th.LinkUserToTeam(guest, th.BasicTeam)
		teamMember, err := th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeGuest)
		require.False(t, teamMember.SchemeUser)

		channelMember := th.AddUserToChannel(guest, th.BasicChannel)
		require.True(t, channelMember.SchemeGuest)
		require.False(t, channelMember.SchemeUser)

		err = th.App.PromoteGuestToUser(th.Context, guest, th.BasicUser.Id)
		require.Nil(t, err)
		guest, err = th.App.GetUser(guest.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_user", guest.Roles)
		teamMember, err = th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeGuest)
		assert.True(t, teamMember.SchemeUser)
		channelMember, err = th.App.GetChannelMember(context.Background(), th.BasicChannel.Id, guest.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeGuest)
		assert.True(t, teamMember.SchemeUser)
	})

	t.Run("Must add the default channels", func(t *testing.T) {
		guest := th.CreateGuest()
		require.Equal(t, "system_guest", guest.Roles)
		th.LinkUserToTeam(guest, th.BasicTeam)
		teamMember, err := th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeGuest)
		require.False(t, teamMember.SchemeUser)

		channelMember := th.AddUserToChannel(guest, th.BasicChannel)
		require.True(t, channelMember.SchemeGuest)
		require.False(t, channelMember.SchemeUser)

		channelMembers, err := th.App.GetChannelMembersForUser(th.BasicTeam.Id, guest.Id)
		require.Nil(t, err)
		require.Len(t, *channelMembers, 1)

		err = th.App.PromoteGuestToUser(th.Context, guest, th.BasicUser.Id)
		require.Nil(t, err)
		guest, err = th.App.GetUser(guest.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_user", guest.Roles)
		teamMember, err = th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeGuest)
		assert.True(t, teamMember.SchemeUser)
		channelMember, err = th.App.GetChannelMember(context.Background(), th.BasicChannel.Id, guest.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeGuest)
		assert.True(t, teamMember.SchemeUser)

		channelMembers, err = th.App.GetChannelMembersForUser(th.BasicTeam.Id, guest.Id)
		require.Nil(t, err)
		assert.Len(t, *channelMembers, 3)
	})

	t.Run("Must invalidate channel stats cache when promoting a guest", func(t *testing.T) {
		guest := th.CreateGuest()
		require.Equal(t, "system_guest", guest.Roles)
		th.LinkUserToTeam(guest, th.BasicTeam)
		teamMember, err := th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeGuest)
		require.False(t, teamMember.SchemeUser)

		guestCount, _ := th.App.GetChannelGuestCount(th.BasicChannel.Id)
		require.Equal(t, int64(0), guestCount)

		channelMember := th.AddUserToChannel(guest, th.BasicChannel)
		require.True(t, channelMember.SchemeGuest)
		require.False(t, channelMember.SchemeUser)

		guestCount, _ = th.App.GetChannelGuestCount(th.BasicChannel.Id)
		require.Equal(t, int64(1), guestCount)

		err = th.App.PromoteGuestToUser(th.Context, guest, th.BasicUser.Id)
		require.Nil(t, err)

		guestCount, _ = th.App.GetChannelGuestCount(th.BasicChannel.Id)
		require.Equal(t, int64(0), guestCount)
	})
}

func TestDemoteUserToGuest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Must invalidate channel stats cache when demoting a user", func(t *testing.T) {
		user := th.CreateUser()
		require.Equal(t, "system_user", user.Roles)
		th.LinkUserToTeam(user, th.BasicTeam)
		teamMember, err := th.App.GetTeamMember(th.BasicTeam.Id, user.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeUser)
		require.False(t, teamMember.SchemeGuest)

		guestCount, _ := th.App.GetChannelGuestCount(th.BasicChannel.Id)
		require.Equal(t, int64(0), guestCount)

		channelMember := th.AddUserToChannel(user, th.BasicChannel)
		require.True(t, channelMember.SchemeUser)
		require.False(t, channelMember.SchemeGuest)

		guestCount, _ = th.App.GetChannelGuestCount(th.BasicChannel.Id)
		require.Equal(t, int64(0), guestCount)

		err = th.App.DemoteUserToGuest(user)
		require.Nil(t, err)

		guestCount, _ = th.App.GetChannelGuestCount(th.BasicChannel.Id)
		require.Equal(t, int64(1), guestCount)
	})

	t.Run("Must fail with guest user", func(t *testing.T) {
		guest := th.CreateGuest()
		require.Equal(t, "system_guest", guest.Roles)
		err := th.App.DemoteUserToGuest(guest)
		require.Nil(t, err)

		user, err := th.App.GetUser(guest.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_guest", user.Roles)
	})

	t.Run("Must work with user without teams or channels", func(t *testing.T) {
		user := th.CreateUser()
		require.Equal(t, "system_user", user.Roles)

		err := th.App.DemoteUserToGuest(user)
		require.Nil(t, err)
		user, err = th.App.GetUser(user.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_guest", user.Roles)
	})

	t.Run("Must work with user with teams but no channels", func(t *testing.T) {
		user := th.CreateUser()
		require.Equal(t, "system_user", user.Roles)
		th.LinkUserToTeam(user, th.BasicTeam)
		teamMember, err := th.App.GetTeamMember(th.BasicTeam.Id, user.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeUser)
		require.False(t, teamMember.SchemeGuest)

		err = th.App.DemoteUserToGuest(user)
		require.Nil(t, err)
		user, err = th.App.GetUser(user.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_guest", user.Roles)
		teamMember, err = th.App.GetTeamMember(th.BasicTeam.Id, user.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeUser)
		assert.True(t, teamMember.SchemeGuest)
	})

	t.Run("Must work with user with teams and channels", func(t *testing.T) {
		user := th.CreateUser()
		require.Equal(t, "system_user", user.Roles)
		th.LinkUserToTeam(user, th.BasicTeam)
		teamMember, err := th.App.GetTeamMember(th.BasicTeam.Id, user.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeUser)
		require.False(t, teamMember.SchemeGuest)

		channelMember := th.AddUserToChannel(user, th.BasicChannel)
		require.True(t, channelMember.SchemeUser)
		require.False(t, channelMember.SchemeGuest)

		err = th.App.DemoteUserToGuest(user)
		require.Nil(t, err)
		user, err = th.App.GetUser(user.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_guest", user.Roles)
		teamMember, err = th.App.GetTeamMember(th.BasicTeam.Id, user.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeUser)
		assert.True(t, teamMember.SchemeGuest)
		channelMember, err = th.App.GetChannelMember(context.Background(), th.BasicChannel.Id, user.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeUser)
		assert.True(t, teamMember.SchemeGuest)
	})

	t.Run("Must respect the current channels not removing defaults", func(t *testing.T) {
		user := th.CreateUser()
		require.Equal(t, "system_user", user.Roles)
		th.LinkUserToTeam(user, th.BasicTeam)
		teamMember, err := th.App.GetTeamMember(th.BasicTeam.Id, user.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeUser)
		require.False(t, teamMember.SchemeGuest)

		channelMember := th.AddUserToChannel(user, th.BasicChannel)
		require.True(t, channelMember.SchemeUser)
		require.False(t, channelMember.SchemeGuest)

		channelMembers, err := th.App.GetChannelMembersForUser(th.BasicTeam.Id, user.Id)
		require.Nil(t, err)
		require.Len(t, *channelMembers, 3)

		err = th.App.DemoteUserToGuest(user)
		require.Nil(t, err)
		user, err = th.App.GetUser(user.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_guest", user.Roles)
		teamMember, err = th.App.GetTeamMember(th.BasicTeam.Id, user.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeUser)
		assert.True(t, teamMember.SchemeGuest)
		channelMember, err = th.App.GetChannelMember(context.Background(), th.BasicChannel.Id, user.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeUser)
		assert.True(t, teamMember.SchemeGuest)

		channelMembers, err = th.App.GetChannelMembersForUser(th.BasicTeam.Id, user.Id)
		require.Nil(t, err)
		assert.Len(t, *channelMembers, 3)
	})

	t.Run("Must be removed as team and channel admin", func(t *testing.T) {
		user := th.CreateUser()
		require.Equal(t, "system_user", user.Roles)

		team := th.CreateTeam()

		th.LinkUserToTeam(user, team)
		th.App.UpdateTeamMemberRoles(team.Id, user.Id, "team_user team_admin")

		teamMember, err := th.App.GetTeamMember(team.Id, user.Id)
		require.Nil(t, err)
		require.True(t, teamMember.SchemeUser)
		require.True(t, teamMember.SchemeAdmin)
		require.False(t, teamMember.SchemeGuest)

		channel := th.CreateChannel(team)

		th.AddUserToChannel(user, channel)
		th.App.UpdateChannelMemberSchemeRoles(channel.Id, user.Id, false, true, true)

		channelMember, err := th.App.GetChannelMember(context.Background(), channel.Id, user.Id)
		assert.Nil(t, err)
		assert.True(t, channelMember.SchemeUser)
		assert.True(t, channelMember.SchemeAdmin)
		assert.False(t, channelMember.SchemeGuest)

		err = th.App.DemoteUserToGuest(user)
		require.Nil(t, err)

		user, err = th.App.GetUser(user.Id)
		assert.Nil(t, err)
		assert.Equal(t, "system_guest", user.Roles)

		teamMember, err = th.App.GetTeamMember(team.Id, user.Id)
		assert.Nil(t, err)
		assert.False(t, teamMember.SchemeUser)
		assert.False(t, teamMember.SchemeAdmin)
		assert.True(t, teamMember.SchemeGuest)

		channelMember, err = th.App.GetChannelMember(context.Background(), channel.Id, user.Id)
		assert.Nil(t, err)
		assert.False(t, channelMember.SchemeUser)
		assert.False(t, channelMember.SchemeAdmin)
		assert.True(t, channelMember.SchemeGuest)
	})
}

func TestDeactivateGuests(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	guest1 := th.CreateGuest()
	guest2 := th.CreateGuest()
	user := th.CreateUser()

	err := th.App.DeactivateGuests(th.Context)
	require.Nil(t, err)

	guest1, err = th.App.GetUser(guest1.Id)
	assert.Nil(t, err)
	assert.NotEqual(t, int64(0), guest1.DeleteAt)

	guest2, err = th.App.GetUser(guest2.Id)
	assert.Nil(t, err)
	assert.NotEqual(t, int64(0), guest2.DeleteAt)

	user, err = th.App.GetUser(user.Id)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), user.DeleteAt)
}

func TestUpdateUserRolesWithUser(t *testing.T) {
	// InitBasic is used to let the first CreateUser call not be
	// a system_admin
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create normal user.
	user := th.CreateUser()
	assert.Equal(t, user.Roles, model.SYSTEM_USER_ROLE_ID)

	// Upgrade to sysadmin.
	user, err := th.App.UpdateUserRolesWithUser(user, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_ADMIN_ROLE_ID, false)
	require.Nil(t, err)
	assert.Equal(t, user.Roles, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_ADMIN_ROLE_ID)

	// Test bad role.
	_, err = th.App.UpdateUserRolesWithUser(user, "does not exist", false)
	require.NotNil(t, err)
}

func TestDeactivateMfa(t *testing.T) {
	t.Run("MFA is disabled", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableMultifactorAuthentication = false
		})

		user := th.BasicUser
		err := th.App.DeactivateMfa(user.Id)
		require.Nil(t, err)
	})
}

func TestPatchUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	testUser := th.CreateUser()
	defer th.App.PermanentDeleteUser(th.Context, testUser)

	t.Run("Patch with a username already exists", func(t *testing.T) {
		_, err := th.App.PatchUser(testUser.Id, &model.UserPatch{
			Username: model.NewString(th.BasicUser.Username),
		}, true)

		require.NotNil(t, err)
		require.Equal(t, "app.user.save.username_exists.app_error", err.Id)
	})

	t.Run("Patch with a email already exists", func(t *testing.T) {
		_, err := th.App.PatchUser(testUser.Id, &model.UserPatch{
			Email: model.NewString(th.BasicUser.Email),
		}, true)

		require.NotNil(t, err)
		require.Equal(t, "app.user.save.email_exists.app_error", err.Id)
	})

	t.Run("Patch username with a new username", func(t *testing.T) {
		_, err := th.App.PatchUser(testUser.Id, &model.UserPatch{
			Username: model.NewString(model.NewId()),
		}, true)

		require.Nil(t, err)
	})
}

func TestUpdateThreadReadForUser(t *testing.T) {
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	t.Run("Ensure thread membership is created and followed", func(t *testing.T) {
		rootPost, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
		require.Nil(t, appErr)
		replyPost, appErr := th.App.CreatePost(th.Context, &model.Post{RootId: rootPost.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
		require.Nil(t, appErr)
		threads, appErr := th.App.GetThreadsForUser(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
		require.Nil(t, appErr)
		require.Zero(t, threads.Total)

		_, appErr = th.App.UpdateThreadReadForUser(th.BasicUser.Id, th.BasicChannel.TeamId, rootPost.Id, replyPost.CreateAt)
		require.Nil(t, appErr)

		threads, appErr = th.App.GetThreadsForUser(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
		require.Nil(t, appErr)
		assert.NotZero(t, threads.Total)

		threadMembership, appErr := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost.Id)
		require.Nil(t, appErr)
		require.NotNil(t, threadMembership)
		assert.True(t, threadMembership.Following)
	})
}
