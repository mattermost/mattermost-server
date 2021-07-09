// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetUserStatus(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	t.Run("offline status", func(t *testing.T) {
		userStatus, resp := Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "offline", userStatus.Status)
	})

	t.Run("online status", func(t *testing.T) {
		th.App.SetStatusOnline(th.BasicUser.ID, true)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "online", userStatus.Status)
	})

	t.Run("away status", func(t *testing.T) {
		th.App.SetStatusAwayIfNeeded(th.BasicUser.ID, true)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "away", userStatus.Status)
	})

	t.Run("dnd status", func(t *testing.T) {
		th.App.SetStatusDoNotDisturb(th.BasicUser.ID)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "dnd", userStatus.Status)
	})

	t.Run("dnd status timed", func(t *testing.T) {
		th.App.SetStatusDoNotDisturbTimed(th.BasicUser.ID, time.Now().Add(10*time.Minute).Unix())
		userStatus, resp := Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "dnd", userStatus.Status)
	})

	t.Run("dnd status timed restore after time interval", func(t *testing.T) {
		task := model.CreateRecurringTaskFromNextIntervalTime("Unset DND Statuses From Test", th.App.UpdateDNDStatusOfUsers, 1*time.Second)
		defer task.Cancel()
		th.App.SetStatusOnline(th.BasicUser.ID, true)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "online", userStatus.Status)
		th.App.SetStatusDoNotDisturbTimed(th.BasicUser.ID, time.Now().Add(2*time.Second).Unix())
		userStatus, resp = Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "dnd", userStatus.Status)
		time.Sleep(3 * time.Second)
		userStatus, resp = Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "online", userStatus.Status)
	})

	t.Run("back to offline status", func(t *testing.T) {
		th.App.SetStatusOffline(th.BasicUser.ID, true)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "offline", userStatus.Status)
	})

	t.Run("get other user status", func(t *testing.T) {
		//Get user2 status logged as user1
		userStatus, resp := Client.GetUserStatus(th.BasicUser2.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "offline", userStatus.Status)
	})

	t.Run("get status from logged out user", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.GetUserStatus(th.BasicUser2.ID, "")
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("get status from other user", func(t *testing.T) {
		th.LoginBasic2()
		userStatus, resp := Client.GetUserStatus(th.BasicUser2.ID, "")
		CheckNoError(t, resp)
		assert.Equal(t, "offline", userStatus.Status)
	})
}

func TestGetUsersStatusesByIDs(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	usersIDs := []string{th.BasicUser.ID, th.BasicUser2.ID}

	t.Run("empty userIds list", func(t *testing.T) {
		_, resp := Client.GetUsersStatusesByIDs([]string{})
		CheckBadRequestStatus(t, resp)
	})

	t.Run("completely invalid userIds list", func(t *testing.T) {
		_, resp := Client.GetUsersStatusesByIDs([]string{"invalid_user_id", "invalid_user_id"})
		CheckBadRequestStatus(t, resp)
	})

	t.Run("partly invalid userIds list", func(t *testing.T) {
		_, resp := Client.GetUsersStatusesByIDs([]string{th.BasicUser.ID, "invalid_user_id"})
		CheckBadRequestStatus(t, resp)
	})

	t.Run("offline status", func(t *testing.T) {
		usersStatuses, resp := Client.GetUsersStatusesByIDs(usersIDs)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "offline", userStatus.Status)
		}
	})

	t.Run("online status", func(t *testing.T) {
		th.App.SetStatusOnline(th.BasicUser.ID, true)
		th.App.SetStatusOnline(th.BasicUser2.ID, true)
		usersStatuses, resp := Client.GetUsersStatusesByIDs(usersIDs)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "online", userStatus.Status)
		}
	})

	t.Run("away status", func(t *testing.T) {
		th.App.SetStatusAwayIfNeeded(th.BasicUser.ID, true)
		th.App.SetStatusAwayIfNeeded(th.BasicUser2.ID, true)
		usersStatuses, resp := Client.GetUsersStatusesByIDs(usersIDs)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "away", userStatus.Status)
		}
	})

	t.Run("dnd status", func(t *testing.T) {
		th.App.SetStatusDoNotDisturb(th.BasicUser.ID)
		th.App.SetStatusDoNotDisturb(th.BasicUser2.ID)
		usersStatuses, resp := Client.GetUsersStatusesByIDs(usersIDs)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "dnd", userStatus.Status)
		}
	})

	t.Run("dnd status", func(t *testing.T) {
		th.App.SetStatusDoNotDisturbTimed(th.BasicUser.ID, time.Now().Add(10*time.Minute).Unix())
		th.App.SetStatusDoNotDisturbTimed(th.BasicUser2.ID, time.Now().Add(15*time.Minute).Unix())
		usersStatuses, resp := Client.GetUsersStatusesByIDs(usersIDs)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "dnd", userStatus.Status)
		}
	})

	t.Run("get statuses from logged out user", func(t *testing.T) {
		Client.Logout()

		_, resp := Client.GetUsersStatusesByIDs(usersIDs)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestUpdateUserStatus(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	t.Run("set online status", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserID: th.BasicUser.ID}
		updateUserStatus, resp := Client.UpdateUserStatus(th.BasicUser.ID, toUpdateUserStatus)
		CheckNoError(t, resp)
		assert.Equal(t, "online", updateUserStatus.Status)
	})

	t.Run("set away status", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "away", UserID: th.BasicUser.ID}
		updateUserStatus, resp := Client.UpdateUserStatus(th.BasicUser.ID, toUpdateUserStatus)
		CheckNoError(t, resp)
		assert.Equal(t, "away", updateUserStatus.Status)
	})

	t.Run("set dnd status timed", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "dnd", UserID: th.BasicUser.ID, DNDEndTime: time.Now().Add(10 * time.Minute).Unix()}
		updateUserStatus, resp := Client.UpdateUserStatus(th.BasicUser.ID, toUpdateUserStatus)
		CheckNoError(t, resp)
		assert.Equal(t, "dnd", updateUserStatus.Status)
	})

	t.Run("set offline status", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "offline", UserID: th.BasicUser.ID}
		updateUserStatus, resp := Client.UpdateUserStatus(th.BasicUser.ID, toUpdateUserStatus)
		CheckNoError(t, resp)
		assert.Equal(t, "offline", updateUserStatus.Status)
	})

	t.Run("set status for other user as regular user", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserID: th.BasicUser2.ID}
		_, resp := Client.UpdateUserStatus(th.BasicUser2.ID, toUpdateUserStatus)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("set status for other user as admin user", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserID: th.BasicUser2.ID}
		updateUserStatus, _ := th.SystemAdminClient.UpdateUserStatus(th.BasicUser2.ID, toUpdateUserStatus)
		assert.Equal(t, "online", updateUserStatus.Status)
	})

	t.Run("not matching status user id and the user id passed in the function", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserID: th.BasicUser2.ID}
		_, resp := Client.UpdateUserStatus(th.BasicUser.ID, toUpdateUserStatus)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("get statuses from logged out user", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserID: th.BasicUser2.ID}
		Client.Logout()

		_, resp := Client.UpdateUserStatus(th.BasicUser2.ID, toUpdateUserStatus)
		CheckUnauthorizedStatus(t, resp)
	})
}
