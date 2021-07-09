// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestCreateUserWithTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewID()
	email := "success+" + id + "@simulator.amazonses.com"
	username := "name" + id

	th.CheckCommand(t, "user", "create", "--email", email, "--password", "mypassword1", "--username", username)

	th.CheckCommand(t, "team", "add", th.BasicTeam.ID, email)

	profiles := th.SystemAdminClient.Must(th.SystemAdminClient.GetUsersInTeam(th.BasicTeam.ID, 0, 1000, "")).([]*model.User)

	found := false

	for _, user := range profiles {
		if user.Email == email {
			found = true
		}

	}

	require.True(t, found, "Failed to create User")
}

func TestCreateUserWithoutTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	id := model.NewID()
	email := "success+" + id + "@simulator.amazonses.com"
	username := "name" + id

	th.CheckCommand(t, "user", "create", "--email", email, "--password", "mypassword1", "--username", username)

	user, err := th.App.Srv().Store.User().GetByEmail(email)
	require.NoError(t, err)

	require.Equal(t, email, user.Email)
}

func TestResetPassword(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.CheckCommand(t, "user", "password", th.BasicUser.Email, "password2")

	th.Client.Logout()
	th.BasicUser.Password = "password2"
	th.LoginBasic()
}

func TestMakeUserActiveAndInactive(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// first inactivate the user
	th.CheckCommand(t, "user", "deactivate", th.BasicUser.Email)

	// activate the inactive user
	th.CheckCommand(t, "user", "activate", th.BasicUser.Email)
}

func TestChangeUserEmail(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	newEmail := model.NewID() + "@mattermost-test.com"

	th.CheckCommand(t, "user", "email", th.BasicUser.Username, newEmail)
	_, err := th.App.Srv().Store.User().GetByEmail(th.BasicUser.Email)
	require.Error(t, err, "should've updated to the new email")

	user, err := th.App.Srv().Store.User().GetByEmail(newEmail)
	require.NoError(t, err)

	require.Equal(t, user.Email, newEmail, "should've updated to the new email")

	// should fail because using an invalid email
	require.Error(t, th.RunCommand(t, "user", "email", th.BasicUser.Username, "wrong$email.com"))

	// should fail because missing one parameter
	require.Error(t, th.RunCommand(t, "user", "email", th.BasicUser.Username))

	// should fail because missing both parameters
	require.Error(t, th.RunCommand(t, "user", "email"))

	// should fail because have more than 2  parameters
	require.Error(t, th.RunCommand(t, "user", "email", th.BasicUser.Username, "new@email.com", "extra!"))

	// should fail because user not found
	require.Error(t, th.RunCommand(t, "user", "email", "invalidUser", newEmail))

	// should fail because email already in use
	require.Error(t, th.RunCommand(t, "user", "email", th.BasicUser.Username, th.BasicUser2.Email))

}

func TestDeleteUserBotUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.CheckCommand(t, "user", "delete", th.BasicUser.Username, "--confirm")
	_, err := th.App.Srv().Store.User().Get(context.Background(), th.BasicUser.ID)
	require.Error(t, err)

	// Make a bot
	bot := &model.Bot{
		Username:    "bottodelete",
		Description: "Delete me!",
		OwnerID:     model.NewID(),
	}
	user, err := th.App.Srv().Store.User().Save(model.UserFromBot(bot))
	require.NoError(t, err)
	bot.UserID = user.ID
	bot, nErr := th.App.Srv().Store.Bot().Save(bot)
	require.NoError(t, nErr)

	th.CheckCommand(t, "user", "delete", bot.Username, "--confirm")
	_, err = th.App.Srv().Store.User().Get(context.Background(), user.ID)
	require.Error(t, err)
	_, nErr = th.App.Srv().Store.Bot().Get(user.ID, true)
	require.Error(t, nErr)
}

func TestConvertUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Invalid command line input", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser.Username)
		require.Error(t, err)

		err = th.RunCommand(t, "user", "convert", th.BasicUser.Username, "--user", "--bot")
		require.Error(t, err)

		err = th.RunCommand(t, "user", "convert", "--bot")
		require.Error(t, err)
	})

	t.Run("Convert to bot from username", func(t *testing.T) {
		th.CheckCommand(t, "user", "convert", th.BasicUser.Username, "anotherinvaliduser", "--bot")
		_, err := th.App.Srv().Store.Bot().Get(th.BasicUser.ID, false)
		require.NoError(t, err)
	})

	t.Run("Unable to convert to user with missing password", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser.Username, "--user")
		require.Error(t, err)
	})

	t.Run("Unable to convert to user with invalid email", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser.Username, "--user",
			"--password", "password",
			"--email", "invalidEmail")
		require.Error(t, err)
	})

	t.Run("Convert to user with minimum flags", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser.Username, "--user",
			"--password", "password")
		require.NoError(t, err)
		_, err = th.App.Srv().Store.Bot().Get(th.BasicUser.ID, false)
		require.Error(t, err)
	})

	t.Run("Convert to bot from email", func(t *testing.T) {
		th.CheckCommand(t, "user", "convert", th.BasicUser2.Email, "--bot")
		_, err := th.App.Srv().Store.Bot().Get(th.BasicUser2.ID, false)
		require.NoError(t, err)
	})

	t.Run("Convert to user with all flags", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser2.Username, "--user",
			"--password", "password",
			"--username", "newusername",
			"--email", "valid@email.com",
			"--nickname", "newNickname",
			"--firstname", "newFirstName",
			"--lastname", "newLastName",
			"--locale", "en_CA",
			"--system_admin")
		require.NoError(t, err)

		_, err = th.App.Srv().Store.Bot().Get(th.BasicUser2.ID, false)
		require.Error(t, err)

		user, appErr := th.App.Srv().Store.User().Get(context.Background(), th.BasicUser2.ID)
		require.NoError(t, appErr)
		require.Equal(t, "newusername", user.Username)
		require.Equal(t, "valid@email.com", user.Email)
		require.Equal(t, "newNickname", user.Nickname)
		require.Equal(t, "newFirstName", user.FirstName)
		require.Equal(t, "newLastName", user.LastName)
		require.Equal(t, "en_CA", user.Locale)
		require.True(t, user.IsInRole("system_admin"))
	})

}
