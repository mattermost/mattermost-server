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
		userStatus, resp := Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "offline", userStatus.Status)
	})

	t.Run("online status", func(t *testing.T) {
		th.App.SetStatusOnline(th.BasicUser.Id, true)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "online", userStatus.Status)
	})

	t.Run("away status", func(t *testing.T) {
		th.App.SetStatusAwayIfNeeded(th.BasicUser.Id, true)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "away", userStatus.Status)
	})

	t.Run("dnd status", func(t *testing.T) {
		th.App.SetStatusDoNotDisturb(th.BasicUser.Id)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "dnd", userStatus.Status)
	})

	t.Run("dnd status timed", func(t *testing.T) {
		th.App.SetStatusDoNotDisturbTimed(th.BasicUser.Id, time.Now().Add(10*time.Minute).Unix())
		userStatus, resp := Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "dnd", userStatus.Status)
	})

	t.Run("dnd status timed restore after time interval", func(t *testing.T) {
		task := model.CreateRecurringTaskFromNextIntervalTime("Unset DND Statuses From Test", th.App.UpdateDNDStatusOfUsers, 1*time.Second)
		defer task.Cancel()
		th.App.SetStatusOnline(th.BasicUser.Id, true)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "online", userStatus.Status)
		th.App.SetStatusDoNotDisturbTimed(th.BasicUser.Id, time.Now().Add(2*time.Second).Unix())
		userStatus, resp = Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "dnd", userStatus.Status)
		time.Sleep(3 * time.Second)
		userStatus, resp = Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "online", userStatus.Status)
	})

	t.Run("update schedules", func(t *testing.T) {
		th.App.SetStatusSchedule(th.BasicUser.Id, "09:00", "18:00", "09:00", "18:00", "09:00", "18:00", "09:00", "18:00", "09:00", "18:00", "09:00", "18:00", "09:00", "18:00", 1)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "09:00", userStatus.MondayStart)
	})

	t.Run("back to offline status", func(t *testing.T) {
		th.App.SetStatusOffline(th.BasicUser.Id, true)
		userStatus, resp := Client.GetUserStatus(th.BasicUser.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "offline", userStatus.Status)
	})

	t.Run("get other user status", func(t *testing.T) {
		//Get user2 status logged as user1
		userStatus, resp := Client.GetUserStatus(th.BasicUser2.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "offline", userStatus.Status)
	})

	t.Run("get status from logged out user", func(t *testing.T) {
		Client.Logout()
		_, resp := Client.GetUserStatus(th.BasicUser2.Id, "")
		CheckUnauthorizedStatus(t, resp)
	})

	t.Run("get status from other user", func(t *testing.T) {
		th.LoginBasic2()
		userStatus, resp := Client.GetUserStatus(th.BasicUser2.Id, "")
		CheckNoError(t, resp)
		assert.Equal(t, "offline", userStatus.Status)
	})
}

func TestGetUsersStatusesByIds(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	usersIds := []string{th.BasicUser.Id, th.BasicUser2.Id}

	t.Run("empty userIds list", func(t *testing.T) {
		_, resp := Client.GetUsersStatusesByIds([]string{})
		CheckBadRequestStatus(t, resp)
	})

	t.Run("completely invalid userIds list", func(t *testing.T) {
		_, resp := Client.GetUsersStatusesByIds([]string{"invalid_user_id", "invalid_user_id"})
		CheckBadRequestStatus(t, resp)
	})

	t.Run("partly invalid userIds list", func(t *testing.T) {
		_, resp := Client.GetUsersStatusesByIds([]string{th.BasicUser.Id, "invalid_user_id"})
		CheckBadRequestStatus(t, resp)
	})

	t.Run("offline status", func(t *testing.T) {
		usersStatuses, resp := Client.GetUsersStatusesByIds(usersIds)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "offline", userStatus.Status)
		}
	})

	t.Run("online status", func(t *testing.T) {
		th.App.SetStatusOnline(th.BasicUser.Id, true)
		th.App.SetStatusOnline(th.BasicUser2.Id, true)
		usersStatuses, resp := Client.GetUsersStatusesByIds(usersIds)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "online", userStatus.Status)
		}
	})

	t.Run("away status", func(t *testing.T) {
		th.App.SetStatusAwayIfNeeded(th.BasicUser.Id, true)
		th.App.SetStatusAwayIfNeeded(th.BasicUser2.Id, true)
		usersStatuses, resp := Client.GetUsersStatusesByIds(usersIds)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "away", userStatus.Status)
		}
	})

	t.Run("dnd status", func(t *testing.T) {
		th.App.SetStatusDoNotDisturb(th.BasicUser.Id)
		th.App.SetStatusDoNotDisturb(th.BasicUser2.Id)
		usersStatuses, resp := Client.GetUsersStatusesByIds(usersIds)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "dnd", userStatus.Status)
		}
	})

	t.Run("dnd status", func(t *testing.T) {
		th.App.SetStatusDoNotDisturbTimed(th.BasicUser.Id, time.Now().Add(10*time.Minute).Unix())
		th.App.SetStatusDoNotDisturbTimed(th.BasicUser2.Id, time.Now().Add(15*time.Minute).Unix())
		usersStatuses, resp := Client.GetUsersStatusesByIds(usersIds)
		CheckNoError(t, resp)
		for _, userStatus := range usersStatuses {
			assert.Equal(t, "dnd", userStatus.Status)
		}
	})

	t.Run("get statuses from logged out user", func(t *testing.T) {
		Client.Logout()

		_, resp := Client.GetUsersStatusesByIds(usersIds)
		CheckUnauthorizedStatus(t, resp)
	})
}

func TestUpdateUserStatus(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	t.Run("set online status", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserId: th.BasicUser.Id}
		updateUserStatus, resp := Client.UpdateUserStatus(th.BasicUser.Id, toUpdateUserStatus)
		CheckNoError(t, resp)
		assert.Equal(t, "online", updateUserStatus.Status)
	})

	t.Run("set away status", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "away", UserId: th.BasicUser.Id}
		updateUserStatus, resp := Client.UpdateUserStatus(th.BasicUser.Id, toUpdateUserStatus)
		CheckNoError(t, resp)
		assert.Equal(t, "away", updateUserStatus.Status)
	})

	t.Run("set dnd status timed", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "dnd", UserId: th.BasicUser.Id, DNDEndTime: time.Now().Add(10 * time.Minute).Unix()}
		updateUserStatus, resp := Client.UpdateUserStatus(th.BasicUser.Id, toUpdateUserStatus)
		CheckNoError(t, resp)
		assert.Equal(t, "dnd", updateUserStatus.Status)
	})

	t.Run("set offline status", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "offline", UserId: th.BasicUser.Id}
		updateUserStatus, resp := Client.UpdateUserStatus(th.BasicUser.Id, toUpdateUserStatus)
		CheckNoError(t, resp)
		assert.Equal(t, "offline", updateUserStatus.Status)
	})

	t.Run("set status for other user as regular user", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserId: th.BasicUser2.Id}
		_, resp := Client.UpdateUserStatus(th.BasicUser2.Id, toUpdateUserStatus)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("set status for other user as admin user", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserId: th.BasicUser2.Id}
		updateUserStatus, _ := th.SystemAdminClient.UpdateUserStatus(th.BasicUser2.Id, toUpdateUserStatus)
		assert.Equal(t, "online", updateUserStatus.Status)
	})

	t.Run("not matching status user id and the user id passed in the function", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserId: th.BasicUser2.Id}
		_, resp := Client.UpdateUserStatus(th.BasicUser.Id, toUpdateUserStatus)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("get statuses from logged out user", func(t *testing.T) {
		toUpdateUserStatus := &model.Status{Status: "online", UserId: th.BasicUser2.Id}
		Client.Logout()

		_, resp := Client.UpdateUserStatus(th.BasicUser2.Id, toUpdateUserStatus)
		CheckUnauthorizedStatus(t, resp)
	})
}
