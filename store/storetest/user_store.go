// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	DayMilliseconds   = 24 * 60 * 60 * 1000
	MonthMilliseconds = 31 * DayMilliseconds
)

func cleanupStatusStore(t *testing.T, s SqlStore) {
	_, execerr := s.GetMaster().ExecNoTimeout(` DELETE FROM Status `)
	require.NoError(t, execerr)
}

func TestUserStore(t *testing.T, ss store.Store, s SqlStore) {
	users, err := ss.User().GetAll()
	require.NoError(t, err, "failed cleaning up test users")

	for _, u := range users {
		err := ss.User().PermanentDelete(u.ID)
		require.NoError(t, err, "failed cleaning up test user %s", u.Username)
	}

	t.Run("Count", func(t *testing.T) { testCount(t, ss) })
	t.Run("AnalyticsActiveCount", func(t *testing.T) { testUserStoreAnalyticsActiveCount(t, ss, s) })
	t.Run("AnalyticsActiveCountForPeriod", func(t *testing.T) { testUserStoreAnalyticsActiveCountForPeriod(t, ss, s) })
	t.Run("AnalyticsGetInactiveUsersCount", func(t *testing.T) { testUserStoreAnalyticsGetInactiveUsersCount(t, ss) })
	t.Run("AnalyticsGetSystemAdminCount", func(t *testing.T) { testUserStoreAnalyticsGetSystemAdminCount(t, ss) })
	t.Run("AnalyticsGetGuestCount", func(t *testing.T) { testUserStoreAnalyticsGetGuestCount(t, ss) })
	t.Run("AnalyticsGetExternalUsers", func(t *testing.T) { testUserStoreAnalyticsGetExternalUsers(t, ss) })
	t.Run("Save", func(t *testing.T) { testUserStoreSave(t, ss) })
	t.Run("Update", func(t *testing.T) { testUserStoreUpdate(t, ss) })
	t.Run("UpdateUpdateAt", func(t *testing.T) { testUserStoreUpdateUpdateAt(t, ss) })
	t.Run("UpdateFailedPasswordAttempts", func(t *testing.T) { testUserStoreUpdateFailedPasswordAttempts(t, ss) })
	t.Run("Get", func(t *testing.T) { testUserStoreGet(t, ss) })
	t.Run("GetAllUsingAuthService", func(t *testing.T) { testGetAllUsingAuthService(t, ss) })
	t.Run("GetAllProfiles", func(t *testing.T) { testUserStoreGetAllProfiles(t, ss) })
	t.Run("GetProfiles", func(t *testing.T) { testUserStoreGetProfiles(t, ss) })
	t.Run("GetProfilesInChannel", func(t *testing.T) { testUserStoreGetProfilesInChannel(t, ss) })
	t.Run("GetProfilesInChannelByStatus", func(t *testing.T) { testUserStoreGetProfilesInChannelByStatus(t, ss, s) })
	t.Run("GetProfilesWithoutTeam", func(t *testing.T) { testUserStoreGetProfilesWithoutTeam(t, ss) })
	t.Run("GetAllProfilesInChannel", func(t *testing.T) { testUserStoreGetAllProfilesInChannel(t, ss) })
	t.Run("GetProfilesNotInChannel", func(t *testing.T) { testUserStoreGetProfilesNotInChannel(t, ss) })
	t.Run("GetProfilesByIds", func(t *testing.T) { testUserStoreGetProfilesByIDs(t, ss) })
	t.Run("GetProfileByGroupChannelIdsForUser", func(t *testing.T) { testUserStoreGetProfileByGroupChannelIDsForUser(t, ss) })
	t.Run("GetProfilesByUsernames", func(t *testing.T) { testUserStoreGetProfilesByUsernames(t, ss) })
	t.Run("GetSystemAdminProfiles", func(t *testing.T) { testUserStoreGetSystemAdminProfiles(t, ss) })
	t.Run("GetByEmail", func(t *testing.T) { testUserStoreGetByEmail(t, ss) })
	t.Run("GetByAuthData", func(t *testing.T) { testUserStoreGetByAuthData(t, ss) })
	t.Run("GetByUsername", func(t *testing.T) { testUserStoreGetByUsername(t, ss) })
	t.Run("GetForLogin", func(t *testing.T) { testUserStoreGetForLogin(t, ss) })
	t.Run("UpdatePassword", func(t *testing.T) { testUserStoreUpdatePassword(t, ss) })
	t.Run("Delete", func(t *testing.T) { testUserStoreDelete(t, ss) })
	t.Run("UpdateAuthData", func(t *testing.T) { testUserStoreUpdateAuthData(t, ss) })
	t.Run("ResetAuthDataToEmailForUsers", func(t *testing.T) { testUserStoreResetAuthDataToEmailForUsers(t, ss) })
	t.Run("UserUnreadCount", func(t *testing.T) { testUserUnreadCount(t, ss) })
	t.Run("UpdateMfaSecret", func(t *testing.T) { testUserStoreUpdateMfaSecret(t, ss) })
	t.Run("UpdateMfaActive", func(t *testing.T) { testUserStoreUpdateMfaActive(t, ss) })
	t.Run("GetRecentlyActiveUsersForTeam", func(t *testing.T) { testUserStoreGetRecentlyActiveUsersForTeam(t, ss, s) })
	t.Run("GetNewUsersForTeam", func(t *testing.T) { testUserStoreGetNewUsersForTeam(t, ss) })
	t.Run("Search", func(t *testing.T) { testUserStoreSearch(t, ss) })
	t.Run("SearchNotInChannel", func(t *testing.T) { testUserStoreSearchNotInChannel(t, ss) })
	t.Run("SearchInChannel", func(t *testing.T) { testUserStoreSearchInChannel(t, ss) })
	t.Run("SearchNotInTeam", func(t *testing.T) { testUserStoreSearchNotInTeam(t, ss) })
	t.Run("SearchWithoutTeam", func(t *testing.T) { testUserStoreSearchWithoutTeam(t, ss) })
	t.Run("SearchInGroup", func(t *testing.T) { testUserStoreSearchInGroup(t, ss) })
	t.Run("GetProfilesNotInTeam", func(t *testing.T) { testUserStoreGetProfilesNotInTeam(t, ss) })
	t.Run("ClearAllCustomRoleAssignments", func(t *testing.T) { testUserStoreClearAllCustomRoleAssignments(t, ss) })
	t.Run("GetAllAfter", func(t *testing.T) { testUserStoreGetAllAfter(t, ss) })
	t.Run("GetUsersBatchForIndexing", func(t *testing.T) { testUserStoreGetUsersBatchForIndexing(t, ss) })
	t.Run("GetTeamGroupUsers", func(t *testing.T) { testUserStoreGetTeamGroupUsers(t, ss) })
	t.Run("GetChannelGroupUsers", func(t *testing.T) { testUserStoreGetChannelGroupUsers(t, ss) })
	t.Run("PromoteGuestToUser", func(t *testing.T) { testUserStorePromoteGuestToUser(t, ss) })
	t.Run("DemoteUserToGuest", func(t *testing.T) { testUserStoreDemoteUserToGuest(t, ss) })
	t.Run("DeactivateGuests", func(t *testing.T) { testDeactivateGuests(t, ss) })
	t.Run("ResetLastPictureUpdate", func(t *testing.T) { testUserStoreResetLastPictureUpdate(t, ss) })
	t.Run("GetKnownUsers", func(t *testing.T) { testGetKnownUsers(t, ss) })
}

func testUserStoreSave(t *testing.T, ss store.Store) {
	teamID := model.NewID()
	maxUsersPerTeam := 50

	u1 := model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}

	_, err := ss.User().Save(&u1)
	require.NoError(t, err, "couldn't save user")

	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, maxUsersPerTeam)
	require.NoError(t, nErr)

	_, err = ss.User().Save(&u1)
	require.Error(t, err, "shouldn't be able to update user from save")

	u2 := model.User{
		Email:    u1.Email,
		Username: model.NewID(),
	}
	_, err = ss.User().Save(&u2)
	require.Error(t, err, "should be unique email")

	u2.Email = MakeEmail()
	u2.Username = u1.Username
	_, err = ss.User().Save(&u1)
	require.Error(t, err, "should be unique username")

	u2.Username = ""
	_, err = ss.User().Save(&u1)
	require.Error(t, err, "should be unique username")

	for i := 0; i < 49; i++ {
		u := model.User{
			Email:    MakeEmail(),
			Username: model.NewID(),
		}
		_, err = ss.User().Save(&u)
		require.NoError(t, err, "couldn't save item")

		defer func() { require.NoError(t, ss.User().PermanentDelete(u.ID)) }()

		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u.ID}, maxUsersPerTeam)
		require.NoError(t, nErr)
	}

	u2.ID = ""
	u2.Email = MakeEmail()
	u2.Username = model.NewID()
	_, err = ss.User().Save(&u2)
	require.NoError(t, err, "couldn't save item")

	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, maxUsersPerTeam)
	require.Error(t, nErr, "should be the limit")
}

func testUserStoreUpdate(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Email: MakeEmail(),
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{
		Email:       MakeEmail(),
		AuthService: "ldap",
	}
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	_, err = ss.User().Update(u1, false)
	require.NoError(t, err)

	missing := &model.User{}
	_, err = ss.User().Update(missing, false)
	require.Error(t, err, "Update should have failed because of missing key")

	newID := &model.User{
		ID: model.NewID(),
	}
	_, err = ss.User().Update(newID, false)
	require.Error(t, err, "Update should have failed because id change")

	u2.Email = MakeEmail()
	_, err = ss.User().Update(u2, false)
	require.Error(t, err, "Update should have failed because you can't modify AD/LDAP fields")

	u3 := &model.User{
		Email:       MakeEmail(),
		AuthService: "gitlab",
	}
	oldEmail := u3.Email
	_, err = ss.User().Save(u3)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u3.ID}, -1)
	require.NoError(t, nErr)

	u3.Email = MakeEmail()
	userUpdate, err := ss.User().Update(u3, false)
	require.NoError(t, err, "Update should not have failed")
	assert.Equal(t, oldEmail, userUpdate.New.Email, "Email should not have been updated as the update is not trusted")

	u3.Email = MakeEmail()
	userUpdate, err = ss.User().Update(u3, true)
	require.NoError(t, err, "Update should not have failed")
	assert.NotEqual(t, oldEmail, userUpdate.New.Email, "Email should have been updated as the update is trusted")

	err = ss.User().UpdateLastPictureUpdate(u1.ID)
	require.NoError(t, err, "Update should not have failed")
}

func testUserStoreUpdateUpdateAt(t *testing.T, ss store.Store) {
	u1 := &model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	_, err = ss.User().UpdateUpdateAt(u1.ID)
	require.NoError(t, err)

	user, err := ss.User().Get(context.Background(), u1.ID)
	require.NoError(t, err)
	require.Less(t, u1.UpdateAt, user.UpdateAt, "UpdateAt not updated correctly")
}

func testUserStoreUpdateFailedPasswordAttempts(t *testing.T, ss store.Store) {
	u1 := &model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	err = ss.User().UpdateFailedPasswordAttempts(u1.ID, 3)
	require.NoError(t, err)

	user, err := ss.User().Get(context.Background(), u1.ID)
	require.NoError(t, err)
	require.Equal(t, 3, user.FailedAttempts, "FailedAttempts not updated correctly")
}

func testUserStoreGet(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Email: MakeEmail(),
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2, _ := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	})
	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:      u2.ID,
		Username:    u2.Username,
		Description: "bot description",
		OwnerID:     u1.ID,
	})
	require.NoError(t, nErr)
	u2.IsBot = true
	u2.BotDescription = "bot description"
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u2.ID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	t.Run("fetch empty id", func(t *testing.T) {
		_, err := ss.User().Get(context.Background(), "")
		require.Error(t, err)
	})

	t.Run("fetch user 1", func(t *testing.T) {
		actual, err := ss.User().Get(context.Background(), u1.ID)
		require.NoError(t, err)
		require.Equal(t, u1, actual)
		require.False(t, actual.IsBot)
	})

	t.Run("fetch user 2, also a bot", func(t *testing.T) {
		actual, err := ss.User().Get(context.Background(), u2.ID)
		require.NoError(t, err)
		require.Equal(t, u2, actual)
		require.True(t, actual.IsBot)
		require.Equal(t, "bot description", actual.BotDescription)
	})
}

func testGetAllUsingAuthService(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:       MakeEmail(),
		Username:    "u1" + model.NewID(),
		AuthService: "service",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:       MakeEmail(),
		Username:    "u2" + model.NewID(),
		AuthService: "service",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:       MakeEmail(),
		Username:    "u3" + model.NewID(),
		AuthService: "service2",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()

	t.Run("get by unknown auth service", func(t *testing.T) {
		users, err := ss.User().GetAllUsingAuthService("unknown")
		require.NoError(t, err)
		assert.Equal(t, []*model.User{}, users)
	})

	t.Run("get by auth service", func(t *testing.T) {
		users, err := ss.User().GetAllUsingAuthService("service")
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1, u2}, users)
	})

	t.Run("get by other auth service", func(t *testing.T) {
		users, err := ss.User().GetAllUsingAuthService("service2")
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u3}, users)
	})
}

func sanitized(user *model.User) *model.User {
	clonedUser := user.DeepCopy()
	clonedUser.Sanitize(map[string]bool{})

	return clonedUser
}

func testUserStoreGetAllProfiles(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
		Roles:    model.SystemUserRoleID,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
		Roles:    model.SystemUserRoleID,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
		Roles:    "system_user some-other-role",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()

	u5, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u5" + model.NewID(),
		Roles:    "system_admin",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u5.ID)) }()

	u6, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u6" + model.NewID(),
		DeleteAt: model.GetMillis(),
		Roles:    "system_admin",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u6.ID)) }()

	u7, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u7" + model.NewID(),
		DeleteAt: model.GetMillis(),
		Roles:    model.SystemUserRoleID,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u7.ID)) }()

	t.Run("get offset 0, limit 100", func(t *testing.T) {
		options := &model.UserGetOptions{Page: 0, PerPage: 100}
		actual, userErr := ss.User().GetAllProfiles(options)
		require.NoError(t, userErr)

		require.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
			sanitized(u3),
			sanitized(u4),
			sanitized(u5),
			sanitized(u6),
			sanitized(u7),
		}, actual)
	})

	t.Run("get offset 0, limit 1", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:    0,
			PerPage: 1,
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u1),
		}, actual)
	})

	t.Run("get all", func(t *testing.T) {
		actual, userErr := ss.User().GetAll()
		require.NoError(t, userErr)

		require.Equal(t, []*model.User{
			u1,
			u2,
			u3,
			u4,
			u5,
			u6,
			u7,
		}, actual)
	})

	t.Run("etag changes for all after user creation", func(t *testing.T) {
		etag := ss.User().GetEtagForAllProfiles()

		uNew := &model.User{}
		uNew.Email = MakeEmail()
		_, userErr := ss.User().Save(uNew)
		require.NoError(t, userErr)
		defer func() { require.NoError(t, ss.User().PermanentDelete(uNew.ID)) }()

		updatedEtag := ss.User().GetEtagForAllProfiles()
		require.NotEqual(t, etag, updatedEtag)
	})

	t.Run("filter to system_admin role", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:    0,
			PerPage: 10,
			Role:    "system_admin",
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u5),
			sanitized(u6),
		}, actual)
	})

	t.Run("filter to system_admin role, inactive", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:     0,
			PerPage:  10,
			Role:     "system_admin",
			Inactive: true,
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u6),
		}, actual)
	})

	t.Run("filter to inactive", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:     0,
			PerPage:  10,
			Inactive: true,
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u6),
			sanitized(u7),
		}, actual)
	})

	t.Run("filter to active", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:    0,
			PerPage: 10,
			Active:  true,
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
			sanitized(u3),
			sanitized(u4),
			sanitized(u5),
		}, actual)
	})

	t.Run("try to filter to active and inactive", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:     0,
			PerPage:  10,
			Inactive: true,
			Active:   true,
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u6),
			sanitized(u7),
		}, actual)
	})

	u8, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u8" + model.NewID(),
		DeleteAt: model.GetMillis(),
		Roles:    "system_user_manager system_user",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u8.ID)) }()

	u9, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u9" + model.NewID(),
		DeleteAt: model.GetMillis(),
		Roles:    "system_manager system_user",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u9.ID)) }()

	u10, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u10" + model.NewID(),
		DeleteAt: model.GetMillis(),
		Roles:    "system_read_only_admin system_user",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u10.ID)) }()

	t.Run("filter by system_user_manager role", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:    0,
			PerPage: 10,
			Roles:   []string{"system_user_manager"},
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u8),
		}, actual)
	})

	t.Run("filter by multiple system roles", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:    0,
			PerPage: 10,
			Roles:   []string{"system_manager", "system_user_manager", "system_read_only_admin", "system_admin"},
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u10),
			sanitized(u5),
			sanitized(u6),
			sanitized(u8),
			sanitized(u9),
		}, actual)
	})

	t.Run("filter by system_user only", func(t *testing.T) {
		actual, userErr := ss.User().GetAllProfiles(&model.UserGetOptions{
			Page:    0,
			PerPage: 10,
			Roles:   []string{"system_user"},
		})
		require.NoError(t, userErr)
		require.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
			sanitized(u7),
		}, actual)
	})
}

func testUserStoreGetProfiles(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
		Roles:    "system_admin",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u4.ID}, -1)
	require.NoError(t, nErr)

	u5, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u5" + model.NewID(),
		DeleteAt: model.GetMillis(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u5.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u5.ID}, -1)
	require.NoError(t, nErr)

	t.Run("get page 0, perPage 100", func(t *testing.T) {
		actual, err := ss.User().GetProfiles(&model.UserGetOptions{
			InTeamID: teamID,
			Page:     0,
			PerPage:  100,
		})
		require.NoError(t, err)

		require.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
			sanitized(u3),
			sanitized(u4),
			sanitized(u5),
		}, actual)
	})

	t.Run("get page 0, perPage 1", func(t *testing.T) {
		actual, err := ss.User().GetProfiles(&model.UserGetOptions{
			InTeamID: teamID,
			Page:     0,
			PerPage:  1,
		})
		require.NoError(t, err)

		require.Equal(t, []*model.User{sanitized(u1)}, actual)
	})

	t.Run("get unknown team id", func(t *testing.T) {
		actual, err := ss.User().GetProfiles(&model.UserGetOptions{
			InTeamID: "123",
			Page:     0,
			PerPage:  100,
		})
		require.NoError(t, err)

		require.Equal(t, []*model.User{}, actual)
	})

	t.Run("etag changes for all after user creation", func(t *testing.T) {
		etag := ss.User().GetEtagForProfiles(teamID)

		uNew := &model.User{}
		uNew.Email = MakeEmail()
		_, err := ss.User().Save(uNew)
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(uNew.ID)) }()
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: uNew.ID}, -1)
		require.NoError(t, nErr)

		updatedEtag := ss.User().GetEtagForProfiles(teamID)
		require.NotEqual(t, etag, updatedEtag)
	})

	t.Run("filter to system_admin role", func(t *testing.T) {
		actual, err := ss.User().GetProfiles(&model.UserGetOptions{
			InTeamID: teamID,
			Page:     0,
			PerPage:  10,
			Role:     "system_admin",
		})
		require.NoError(t, err)
		require.Equal(t, []*model.User{
			sanitized(u4),
		}, actual)
	})

	t.Run("filter to inactive", func(t *testing.T) {
		actual, err := ss.User().GetProfiles(&model.UserGetOptions{
			InTeamID: teamID,
			Page:     0,
			PerPage:  10,
			Inactive: true,
		})
		require.NoError(t, err)
		require.Equal(t, []*model.User{
			sanitized(u5),
		}, actual)
	})

	t.Run("filter to active", func(t *testing.T) {
		actual, err := ss.User().GetProfiles(&model.UserGetOptions{
			InTeamID: teamID,
			Page:     0,
			PerPage:  10,
			Active:   true,
		})
		require.NoError(t, err)
		require.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
			sanitized(u3),
			sanitized(u4),
		}, actual)
	})

	t.Run("try to filter to active and inactive", func(t *testing.T) {
		actual, err := ss.User().GetProfiles(&model.UserGetOptions{
			InTeamID: teamID,
			Page:     0,
			PerPage:  10,
			Inactive: true,
			Active:   true,
		})
		require.NoError(t, err)
		require.Equal(t, []*model.User{
			sanitized(u5),
		}, actual)
	})
}

func testUserStoreGetProfilesInChannel(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u4.ID}, -1)
	require.NoError(t, nErr)

	ch1 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in channel",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	c1, nErr := ss.Channel().Save(ch1, -1)
	require.NoError(t, nErr)

	ch2 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in private",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	c2, nErr := ss.Channel().Save(ch2, -1)
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u3.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u4.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	u4.DeleteAt = 1
	_, err = ss.User().Update(u4, true)
	require.NoError(t, err)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	t.Run("get all users in channel 1, offset 0, limit 100", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannel(&model.UserGetOptions{
			InChannelID: c1.ID,
			Page:        0,
			PerPage:     100,
		})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u1), sanitized(u2), sanitized(u3), sanitized(u4)}, users)
	})

	t.Run("get only active users in channel 1, offset 0, limit 100", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannel(&model.UserGetOptions{
			InChannelID: c1.ID,
			Page:        0,
			PerPage:     100,
			Active:      true,
		})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u1), sanitized(u2), sanitized(u3)}, users)
	})

	t.Run("get inactive users in channel 1, offset 0, limit 100", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannel(&model.UserGetOptions{
			InChannelID: c1.ID,
			Page:        0,
			PerPage:     100,
			Inactive:    true,
		})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u4)}, users)
	})

	t.Run("get in channel 1, offset 1, limit 2", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannel(&model.UserGetOptions{
			InChannelID: c1.ID,
			Page:        1,
			PerPage:     1,
		})
		require.NoError(t, err)
		users_p2, err2 := ss.User().GetProfilesInChannel(&model.UserGetOptions{
			InChannelID: c1.ID,
			Page:        2,
			PerPage:     1,
		})
		require.NoError(t, err2)
		users = append(users, users_p2...)
		assert.Equal(t, []*model.User{sanitized(u2), sanitized(u3)}, users)
	})

	t.Run("get in channel 2, offset 0, limit 1", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannel(&model.UserGetOptions{
			InChannelID: c2.ID,
			Page:        0,
			PerPage:     1,
		})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u1)}, users)
	})
}

func testUserStoreGetProfilesInChannelByStatus(t *testing.T, ss store.Store, s SqlStore) {

	cleanupStatusStore(t, s)

	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u4.ID}, -1)
	require.NoError(t, nErr)

	ch1 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in channel",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	c1, nErr := ss.Channel().Save(ch1, -1)
	require.NoError(t, nErr)

	ch2 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in private",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	c2, nErr := ss.Channel().Save(ch2, -1)
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u3.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u4.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	u4.DeleteAt = 1
	_, err = ss.User().Update(u4, true)
	require.NoError(t, err)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{
		UserID: u1.ID,
		Status: model.StatusDnd,
	}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{
		UserID: u2.ID,
		Status: model.StatusAway,
	}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{
		UserID: u3.ID,
		Status: model.StatusOnline,
	}))

	t.Run("get all users in channel 1, offset 0, limit 100", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannel(&model.UserGetOptions{
			InChannelID: c1.ID,
			Page:        0,
			PerPage:     100,
		})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u1), sanitized(u2), sanitized(u3), sanitized(u4)}, users)
	})

	t.Run("get active in channel 1 by status, offset 0, limit 100", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannelByStatus(&model.UserGetOptions{
			InChannelID: c1.ID,
			Page:        0,
			PerPage:     100,
			Active:      true,
		})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u3), sanitized(u2), sanitized(u1)}, users)
	})

	t.Run("get inactive users in channel 1, offset 0, limit 100", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannel(&model.UserGetOptions{
			InChannelID: c1.ID,
			Page:        0,
			PerPage:     100,
			Inactive:    true,
		})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u4)}, users)
	})

	t.Run("get in channel 2 by status, offset 0, limit 1", func(t *testing.T) {
		users, err := ss.User().GetProfilesInChannelByStatus(&model.UserGetOptions{
			InChannelID: c2.ID,
			Page:        0,
			PerPage:     1,
		})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u1)}, users)
	})
}

func testUserStoreGetProfilesWithoutTeam(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
		DeleteAt: 1,
		Roles:    "system_admin",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	t.Run("get, page 0, per_page 100", func(t *testing.T) {
		users, err := ss.User().GetProfilesWithoutTeam(&model.UserGetOptions{Page: 0, PerPage: 100})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u2), sanitized(u3)}, users)
	})

	t.Run("get, page 1, per_page 1", func(t *testing.T) {
		users, err := ss.User().GetProfilesWithoutTeam(&model.UserGetOptions{Page: 1, PerPage: 1})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u3)}, users)
	})

	t.Run("get, page 2, per_page 1", func(t *testing.T) {
		users, err := ss.User().GetProfilesWithoutTeam(&model.UserGetOptions{Page: 2, PerPage: 1})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{}, users)
	})

	t.Run("get, page 0, per_page 100, inactive", func(t *testing.T) {
		users, err := ss.User().GetProfilesWithoutTeam(&model.UserGetOptions{Page: 0, PerPage: 100, Inactive: true})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u3)}, users)
	})

	t.Run("get, page 0, per_page 100, role", func(t *testing.T) {
		users, err := ss.User().GetProfilesWithoutTeam(&model.UserGetOptions{Page: 0, PerPage: 100, Role: "system_admin"})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{sanitized(u3)}, users)
	})
}

func testUserStoreGetAllProfilesInChannel(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	ch1 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in channel",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	c1, nErr := ss.Channel().Save(ch1, -1)
	require.NoError(t, nErr)

	ch2 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in private",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	c2, nErr := ss.Channel().Save(ch2, -1)
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u3.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	t.Run("all profiles in channel 1, no caching", func(t *testing.T) {
		var profiles map[string]*model.User
		profiles, err = ss.User().GetAllProfilesInChannel(context.Background(), c1.ID, false)
		require.NoError(t, err)
		assert.Equal(t, map[string]*model.User{
			u1.ID: sanitized(u1),
			u2.ID: sanitized(u2),
			u3.ID: sanitized(u3),
		}, profiles)
	})

	t.Run("all profiles in channel 2, no caching", func(t *testing.T) {
		var profiles map[string]*model.User
		profiles, err = ss.User().GetAllProfilesInChannel(context.Background(), c2.ID, false)
		require.NoError(t, err)
		assert.Equal(t, map[string]*model.User{
			u1.ID: sanitized(u1),
		}, profiles)
	})

	t.Run("all profiles in channel 2, caching", func(t *testing.T) {
		var profiles map[string]*model.User
		profiles, err = ss.User().GetAllProfilesInChannel(context.Background(), c2.ID, true)
		require.NoError(t, err)
		assert.Equal(t, map[string]*model.User{
			u1.ID: sanitized(u1),
		}, profiles)
	})

	t.Run("all profiles in channel 2, caching [repeated]", func(t *testing.T) {
		var profiles map[string]*model.User
		profiles, err = ss.User().GetAllProfilesInChannel(context.Background(), c2.ID, true)
		require.NoError(t, err)
		assert.Equal(t, map[string]*model.User{
			u1.ID: sanitized(u1),
		}, profiles)
	})

	ss.User().InvalidateProfilesInChannelCacheByUser(u1.ID)
	ss.User().InvalidateProfilesInChannelCache(c2.ID)
}

func testUserStoreGetProfilesNotInChannel(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	ch1 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in channel",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	c1, nErr := ss.Channel().Save(ch1, -1)
	require.NoError(t, nErr)

	ch2 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in private",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	c2, nErr := ss.Channel().Save(ch2, -1)
	require.NoError(t, nErr)

	t.Run("get team 1, channel 1, offset 0, limit 100", func(t *testing.T) {
		var profiles []*model.User
		profiles, err = ss.User().GetProfilesNotInChannel(teamID, c1.ID, false, 0, 100, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
			sanitized(u3),
		}, profiles)
	})

	t.Run("get team 1, channel 2, offset 0, limit 100", func(t *testing.T) {
		var profiles []*model.User
		profiles, err = ss.User().GetProfilesNotInChannel(teamID, c2.ID, false, 0, 100, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
			sanitized(u3),
		}, profiles)
	})

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u3.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	t.Run("get team 1, channel 1, offset 0, limit 100, after update", func(t *testing.T) {
		var profiles []*model.User
		profiles, err = ss.User().GetProfilesNotInChannel(teamID, c1.ID, false, 0, 100, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{}, profiles)
	})

	t.Run("get team 1, channel 2, offset 0, limit 100, after update", func(t *testing.T) {
		var profiles []*model.User
		profiles, err = ss.User().GetProfilesNotInChannel(teamID, c2.ID, false, 0, 100, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u2),
			sanitized(u3),
		}, profiles)
	})

	t.Run("get team 1, channel 2, offset 0, limit 0, setting group constrained when it's not", func(t *testing.T) {
		var profiles []*model.User
		profiles, err = ss.User().GetProfilesNotInChannel(teamID, c2.ID, true, 0, 100, nil)
		require.NoError(t, err)
		assert.Empty(t, profiles)
	})

	// create a group
	group, err := ss.Group().Create(&model.Group{
		Name:        model.NewString("n_" + model.NewID()),
		DisplayName: "dn_" + model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    "ri_" + model.NewID(),
	})
	require.NoError(t, err)

	// add two members to the group
	for _, u := range []*model.User{u1, u2} {
		_, err = ss.Group().UpsertMember(group.ID, u.ID)
		require.NoError(t, err)
	}

	// associate the group with the channel
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		GroupID:    group.ID,
		SyncableID: c2.ID,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.NoError(t, err)

	t.Run("get team 1, channel 2, offset 0, limit 0, setting group constrained", func(t *testing.T) {
		profiles, err := ss.User().GetProfilesNotInChannel(teamID, c2.ID, true, 0, 100, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u2),
		}, profiles)
	})
}

func testUserStoreGetProfilesByIDs(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	time.Sleep(time.Millisecond)
	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()

	t.Run("get u1 by id, no caching", func(t *testing.T) {
		users, err := ss.User().GetProfileByIDs(context.Background(), []string{u1.ID}, nil, false)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1}, users)
	})

	t.Run("get u1 by id, caching", func(t *testing.T) {
		users, err := ss.User().GetProfileByIDs(context.Background(), []string{u1.ID}, nil, true)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1}, users)
	})

	t.Run("get u1, u2, u3 by id, no caching", func(t *testing.T) {
		users, err := ss.User().GetProfileByIDs(context.Background(), []string{u1.ID, u2.ID, u3.ID}, nil, false)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1, u2, u3}, users)
	})

	t.Run("get u1, u2, u3 by id, caching", func(t *testing.T) {
		users, err := ss.User().GetProfileByIDs(context.Background(), []string{u1.ID, u2.ID, u3.ID}, nil, true)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1, u2, u3}, users)
	})

	t.Run("get unknown id, caching", func(t *testing.T) {
		users, err := ss.User().GetProfileByIDs(context.Background(), []string{"123"}, nil, true)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{}, users)
	})

	t.Run("should only return users with UpdateAt greater than the since time", func(t *testing.T) {
		users, err := ss.User().GetProfileByIDs(context.Background(), []string{u1.ID, u2.ID, u3.ID, u4.ID}, &store.UserGetByIDsOpts{
			Since: u2.CreateAt,
		}, true)
		require.NoError(t, err)

		// u3 comes from the cache, and u4 does not
		assert.Equal(t, []*model.User{u3, u4}, users)
	})
}

func testUserStoreGetProfileByGroupChannelIDsForUser(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()

	gc1, nErr := ss.Channel().Save(&model.Channel{
		DisplayName: "Profiles in private",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypeGroup,
	}, -1)
	require.NoError(t, nErr)

	for _, uID := range []string{u1.ID, u2.ID, u3.ID} {
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
			ChannelID:   gc1.ID,
			UserID:      uID,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, nErr)
	}

	gc2, nErr := ss.Channel().Save(&model.Channel{
		DisplayName: "Profiles in private",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypeGroup,
	}, -1)
	require.NoError(t, nErr)

	for _, uID := range []string{u1.ID, u3.ID, u4.ID} {
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
			ChannelID:   gc2.ID,
			UserID:      uID,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, nErr)
	}

	testCases := []struct {
		Name                       string
		UserID                     string
		ChannelIDs                 []string
		ExpectedUserIDsByChannel   map[string][]string
		EnsureChannelsNotInResults []string
	}{
		{
			Name:       "Get group 1 as user 1",
			UserID:     u1.ID,
			ChannelIDs: []string{gc1.ID},
			ExpectedUserIDsByChannel: map[string][]string{
				gc1.ID: {u2.ID, u3.ID},
			},
			EnsureChannelsNotInResults: []string{},
		},
		{
			Name:       "Get groups 1 and 2 as user 1",
			UserID:     u1.ID,
			ChannelIDs: []string{gc1.ID, gc2.ID},
			ExpectedUserIDsByChannel: map[string][]string{
				gc1.ID: {u2.ID, u3.ID},
				gc2.ID: {u3.ID, u4.ID},
			},
			EnsureChannelsNotInResults: []string{},
		},
		{
			Name:       "Get groups 1 and 2 as user 2",
			UserID:     u2.ID,
			ChannelIDs: []string{gc1.ID, gc2.ID},
			ExpectedUserIDsByChannel: map[string][]string{
				gc1.ID: {u1.ID, u3.ID},
			},
			EnsureChannelsNotInResults: []string{gc2.ID},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			res, err := ss.User().GetProfileByGroupChannelIDsForUser(tc.UserID, tc.ChannelIDs)
			require.NoError(t, err)

			for channelID, expectedUsers := range tc.ExpectedUserIDsByChannel {
				users, ok := res[channelID]
				require.True(t, ok)

				var userIDs []string
				for _, user := range users {
					userIDs = append(userIDs, user.ID)
				}
				require.ElementsMatch(t, expectedUsers, userIDs)
			}

			for _, channelID := range tc.EnsureChannelsNotInResults {
				_, ok := res[channelID]
				require.False(t, ok)
			}
		})
	}
}

func testUserStoreGetProfilesByUsernames(t *testing.T, ss store.Store) {
	teamID := model.NewID()
	team2ID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: team2ID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	t.Run("get by u1 and u2 usernames, team id 1", func(t *testing.T) {
		users, err := ss.User().GetProfilesByUsernames([]string{u1.Username, u2.Username}, &model.ViewUsersRestrictions{Teams: []string{teamID}})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1, u2}, users)
	})

	t.Run("get by u1 username, team id 1", func(t *testing.T) {
		users, err := ss.User().GetProfilesByUsernames([]string{u1.Username}, &model.ViewUsersRestrictions{Teams: []string{teamID}})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1}, users)
	})

	t.Run("get by u1 and u3 usernames, no team id", func(t *testing.T) {
		users, err := ss.User().GetProfilesByUsernames([]string{u1.Username, u3.Username}, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1, u3}, users)
	})

	t.Run("get by u1 and u3 usernames, team id 1", func(t *testing.T) {
		users, err := ss.User().GetProfilesByUsernames([]string{u1.Username, u3.Username}, &model.ViewUsersRestrictions{Teams: []string{teamID}})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u1}, users)
	})

	t.Run("get by u1 and u3 usernames, team id 2", func(t *testing.T) {
		users, err := ss.User().GetProfilesByUsernames([]string{u1.Username, u3.Username}, &model.ViewUsersRestrictions{Teams: []string{team2ID}})
		require.NoError(t, err)
		assert.Equal(t, []*model.User{u3}, users)
	})
}

func testUserStoreGetSystemAdminProfiles(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Roles:    model.SystemUserRoleID + " " + model.SystemAdminRoleID,
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Roles:    model.SystemUserRoleID + " " + model.SystemAdminRoleID,
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	t.Run("all system admin profiles", func(t *testing.T) {
		result, userError := ss.User().GetSystemAdminProfiles()
		require.NoError(t, userError)
		assert.Equal(t, map[string]*model.User{
			u1.ID: sanitized(u1),
			u3.ID: sanitized(u3),
		}, result)
	})
}

func testUserStoreGetByEmail(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	t.Run("get u1 by email", func(t *testing.T) {
		u, err := ss.User().GetByEmail(u1.Email)
		require.NoError(t, err)
		assert.Equal(t, u1, u)
	})

	t.Run("get u2 by email", func(t *testing.T) {
		u, err := ss.User().GetByEmail(u2.Email)
		require.NoError(t, err)
		assert.Equal(t, u2, u)
	})

	t.Run("get u3 by email", func(t *testing.T) {
		u, err := ss.User().GetByEmail(u3.Email)
		require.NoError(t, err)
		assert.Equal(t, u3, u)
	})

	t.Run("get by empty email", func(t *testing.T) {
		_, err := ss.User().GetByEmail("")
		require.Error(t, err)
	})

	t.Run("get by unknown", func(t *testing.T) {
		_, err := ss.User().GetByEmail("unknown")
		require.Error(t, err)
	})
}

func testUserStoreGetByAuthData(t *testing.T, ss store.Store) {
	teamID := model.NewID()
	auth1 := model.NewID()
	auth3 := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:       MakeEmail(),
		Username:    "u1" + model.NewID(),
		AuthData:    &auth1,
		AuthService: "service",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:       MakeEmail(),
		Username:    "u3" + model.NewID(),
		AuthData:    &auth3,
		AuthService: "service2",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	t.Run("get by u1 auth", func(t *testing.T) {
		u, err := ss.User().GetByAuth(u1.AuthData, u1.AuthService)
		require.NoError(t, err)
		assert.Equal(t, u1, u)
	})

	t.Run("get by u3 auth", func(t *testing.T) {
		u, err := ss.User().GetByAuth(u3.AuthData, u3.AuthService)
		require.NoError(t, err)
		assert.Equal(t, u3, u)
	})

	t.Run("get by u1 auth, unknown service", func(t *testing.T) {
		_, err := ss.User().GetByAuth(u1.AuthData, "unknown")
		require.Error(t, err)
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(err, &nfErr))
	})

	t.Run("get by unknown auth, u1 service", func(t *testing.T) {
		unknownAuth := ""
		_, err := ss.User().GetByAuth(&unknownAuth, u1.AuthService)
		require.Error(t, err)
		var invErr *store.ErrInvalidInput
		require.True(t, errors.As(err, &invErr))
	})

	t.Run("get by unknown auth, unknown service", func(t *testing.T) {
		unknownAuth := ""
		_, err := ss.User().GetByAuth(&unknownAuth, "unknown")
		require.Error(t, err)
		var invErr *store.ErrInvalidInput
		require.True(t, errors.As(err, &invErr))
	})
}

func testUserStoreGetByUsername(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	t.Run("get u1 by username", func(t *testing.T) {
		result, err := ss.User().GetByUsername(u1.Username)
		require.NoError(t, err)
		assert.Equal(t, u1, result)
	})

	t.Run("get u2 by username", func(t *testing.T) {
		result, err := ss.User().GetByUsername(u2.Username)
		require.NoError(t, err)
		assert.Equal(t, u2, result)
	})

	t.Run("get u3 by username", func(t *testing.T) {
		result, err := ss.User().GetByUsername(u3.Username)
		require.NoError(t, err)
		assert.Equal(t, u3, result)
	})

	t.Run("get by empty username", func(t *testing.T) {
		_, err := ss.User().GetByUsername("")
		require.Error(t, err)
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(err, &nfErr))
	})

	t.Run("get by unknown", func(t *testing.T) {
		_, err := ss.User().GetByUsername("unknown")
		require.Error(t, err)
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(err, &nfErr))
	})
}

func testUserStoreGetForLogin(t *testing.T, ss store.Store) {
	teamID := model.NewID()
	auth := model.NewID()
	auth2 := model.NewID()
	auth3 := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:       MakeEmail(),
		Username:    "u1" + model.NewID(),
		AuthService: model.UserAuthServiceGitlab,
		AuthData:    &auth,
	})

	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:       MakeEmail(),
		Username:    "u2" + model.NewID(),
		AuthService: model.UserAuthServiceLdap,
		AuthData:    &auth2,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:       MakeEmail(),
		Username:    "u3" + model.NewID(),
		AuthService: model.UserAuthServiceLdap,
		AuthData:    &auth3,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	t.Run("get u1 by username, allow both", func(t *testing.T) {
		user, err := ss.User().GetForLogin(u1.Username, true, true)
		require.NoError(t, err)
		assert.Equal(t, u1, user)
	})

	t.Run("get u1 by username, check for case issues", func(t *testing.T) {
		user, err := ss.User().GetForLogin(strings.ToUpper(u1.Username), true, true)
		require.NoError(t, err)
		assert.Equal(t, u1, user)
	})

	t.Run("get u1 by username, allow only email", func(t *testing.T) {
		_, err := ss.User().GetForLogin(u1.Username, false, true)
		require.Error(t, err)
		require.Equal(t, "user not found", err.Error())
	})

	t.Run("get u1 by email, allow both", func(t *testing.T) {
		user, err := ss.User().GetForLogin(u1.Email, true, true)
		require.NoError(t, err)
		assert.Equal(t, u1, user)
	})

	t.Run("get u1 by email, check for case issues", func(t *testing.T) {
		user, err := ss.User().GetForLogin(strings.ToUpper(u1.Email), true, true)
		require.NoError(t, err)
		assert.Equal(t, u1, user)
	})

	t.Run("get u1 by email, allow only username", func(t *testing.T) {
		_, err := ss.User().GetForLogin(u1.Email, true, false)
		require.Error(t, err)
		require.Equal(t, "user not found", err.Error())
	})

	t.Run("get u2 by username, allow both", func(t *testing.T) {
		user, err := ss.User().GetForLogin(u2.Username, true, true)
		require.NoError(t, err)
		assert.Equal(t, u2, user)
	})

	t.Run("get u2 by email, allow both", func(t *testing.T) {
		user, err := ss.User().GetForLogin(u2.Email, true, true)
		require.NoError(t, err)
		assert.Equal(t, u2, user)
	})

	t.Run("get u2 by username, allow neither", func(t *testing.T) {
		_, err := ss.User().GetForLogin(u2.Username, false, false)
		require.Error(t, err)
		require.Equal(t, "sign in with username and email are disabled", err.Error())
	})
}

func testUserStoreUpdatePassword(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1 := &model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	hashedPassword := model.HashPassword("newpwd")

	err = ss.User().UpdatePassword(u1.ID, hashedPassword)
	require.NoError(t, err)

	user, err := ss.User().GetByEmail(u1.Email)
	require.NoError(t, err)
	require.Equal(t, user.Password, hashedPassword, "Password was not updated correctly")
}

func testUserStoreDelete(t *testing.T, ss store.Store) {
	u1 := &model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	err = ss.User().PermanentDelete(u1.ID)
	require.NoError(t, err)
}

func testUserStoreUpdateAuthData(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1 := &model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	service := "someservice"
	authData := model.NewID()

	_, err = ss.User().UpdateAuthData(u1.ID, service, &authData, "", true)
	require.NoError(t, err)

	user, err := ss.User().GetByEmail(u1.Email)
	require.NoError(t, err)
	require.Equal(t, service, user.AuthService, "AuthService was not updated correctly")
	require.Equal(t, authData, *user.AuthData, "AuthData was not updated correctly")
	require.Equal(t, "", user.Password, "Password was not cleared properly")
}

func testUserStoreResetAuthDataToEmailForUsers(t *testing.T, ss store.Store) {
	user := &model.User{}
	user.Username = "user1" + model.NewID()
	user.Email = MakeEmail()
	_, err := ss.User().Save(user)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

	resetAuthDataToID := func() {
		_, err = ss.User().UpdateAuthData(
			user.ID, model.UserAuthServiceSaml, model.NewString("some-id"), "", false)
		require.NoError(t, err)
	}
	resetAuthDataToID()

	// dry run
	numAffected, err := ss.User().ResetAuthDataToEmailForUsers(model.UserAuthServiceSaml, nil, false, true)
	require.NoError(t, err)
	require.Equal(t, 1, numAffected)
	// real run
	numAffected, err = ss.User().ResetAuthDataToEmailForUsers(model.UserAuthServiceSaml, nil, false, false)
	require.NoError(t, err)
	require.Equal(t, 1, numAffected)
	user, appErr := ss.User().Get(context.Background(), user.ID)
	require.NoError(t, appErr)
	require.Equal(t, *user.AuthData, user.Email)

	resetAuthDataToID()
	// with specific user IDs
	numAffected, err = ss.User().ResetAuthDataToEmailForUsers(model.UserAuthServiceSaml, []string{model.NewID()}, false, true)
	require.NoError(t, err)
	require.Equal(t, 0, numAffected)
	numAffected, err = ss.User().ResetAuthDataToEmailForUsers(model.UserAuthServiceSaml, []string{user.ID}, false, true)
	require.NoError(t, err)
	require.Equal(t, 1, numAffected)

	// delete user
	user.DeleteAt = model.GetMillisForTime(time.Now())
	ss.User().Update(user, true)
	// without deleted user
	numAffected, err = ss.User().ResetAuthDataToEmailForUsers(model.UserAuthServiceSaml, nil, false, true)
	require.NoError(t, err)
	require.Equal(t, 0, numAffected)
	// with deleted user
	numAffected, err = ss.User().ResetAuthDataToEmailForUsers(model.UserAuthServiceSaml, nil, true, true)
	require.NoError(t, err)
	require.Equal(t, 1, numAffected)
}

func testUserUnreadCount(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	c1 := model.Channel{}
	c1.TeamID = teamID
	c1.DisplayName = "Unread Messages"
	c1.Name = "unread-messages-" + model.NewID()
	c1.Type = model.ChannelTypeOpen

	c2 := model.Channel{}
	c2.TeamID = teamID
	c2.DisplayName = "Unread Direct"
	c2.Name = "unread-direct-" + model.NewID()
	c2.Type = model.ChannelTypeDirect

	u1 := &model.User{}
	u1.Username = "user1" + model.NewID()
	u1.Email = MakeEmail()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.Username = "user2" + model.NewID()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	_, nErr = ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr, "couldn't save item")

	m1 := model.ChannelMember{}
	m1.ChannelID = c1.ID
	m1.UserID = u1.ID
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelID = c1.ID
	m2.UserID = u2.ID
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	_, nErr = ss.Channel().SaveMember(&m2)
	require.NoError(t, nErr)

	m1.ChannelID = c2.ID
	m2.ChannelID = c2.ID

	_, nErr = ss.Channel().SaveDirectChannel(&c2, &m1, &m2)
	require.NoError(t, nErr, "couldn't save direct channel")

	p1 := model.Post{}
	p1.ChannelID = c1.ID
	p1.UserID = u1.ID
	p1.Message = "this is a message for @" + u2.Username

	// Post one message with mention to open channel
	_, nErr = ss.Post().Save(&p1)
	require.NoError(t, nErr)
	nErr = ss.Channel().IncrementMentionCount(c1.ID, u2.ID, false, false)
	require.NoError(t, nErr)

	// Post 2 messages without mention to direct channel
	p2 := model.Post{}
	p2.ChannelID = c2.ID
	p2.UserID = u1.ID
	p2.Message = "first message"

	_, nErr = ss.Post().Save(&p2)
	require.NoError(t, nErr)
	nErr = ss.Channel().IncrementMentionCount(c2.ID, u2.ID, false, false)
	require.NoError(t, nErr)

	p3 := model.Post{}
	p3.ChannelID = c2.ID
	p3.UserID = u1.ID
	p3.Message = "second message"
	_, nErr = ss.Post().Save(&p3)
	require.NoError(t, nErr)

	nErr = ss.Channel().IncrementMentionCount(c2.ID, u2.ID, false, false)
	require.NoError(t, nErr)

	badge, unreadCountErr := ss.User().GetUnreadCount(u2.ID)
	require.NoError(t, unreadCountErr)
	require.Equal(t, int64(3), badge, "should have 3 unread messages")

	badge, unreadCountErr = ss.User().GetUnreadCountForChannel(u2.ID, c1.ID)
	require.NoError(t, unreadCountErr)
	require.Equal(t, int64(1), badge, "should have 1 unread messages for that channel")

	badge, unreadCountErr = ss.User().GetUnreadCountForChannel(u2.ID, c2.ID)
	require.NoError(t, unreadCountErr)
	require.Equal(t, int64(2), badge, "should have 2 unread messages for that channel")
}

func testUserStoreUpdateMfaSecret(t *testing.T, ss store.Store) {
	u1 := model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(&u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	err = ss.User().UpdateMfaSecret(u1.ID, "12345")
	require.NoError(t, err)

	// should pass, no update will occur though
	err = ss.User().UpdateMfaSecret("junk", "12345")
	require.NoError(t, err)
}

func testUserStoreUpdateMfaActive(t *testing.T, ss store.Store) {
	u1 := model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(&u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	time.Sleep(time.Millisecond)

	err = ss.User().UpdateMfaActive(u1.ID, true)
	require.NoError(t, err)

	err = ss.User().UpdateMfaActive(u1.ID, false)
	require.NoError(t, err)

	// should pass, no update will occur though
	err = ss.User().UpdateMfaActive("junk", true)
	require.NoError(t, err)
}

func testUserStoreGetRecentlyActiveUsersForTeam(t *testing.T, ss store.Store, s SqlStore) {

	cleanupStatusStore(t, s)

	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	millis := model.GetMillis()
	u3.LastActivityAt = millis
	u2.LastActivityAt = millis - 1
	u1.LastActivityAt = millis - 1

	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u1.ID, Status: model.StatusOnline, Manual: false, LastActivityAt: u1.LastActivityAt, ActiveChannel: ""}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u2.ID, Status: model.StatusOnline, Manual: false, LastActivityAt: u2.LastActivityAt, ActiveChannel: ""}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u3.ID, Status: model.StatusOnline, Manual: false, LastActivityAt: u3.LastActivityAt, ActiveChannel: ""}))

	t.Run("get team 1, offset 0, limit 100", func(t *testing.T) {
		users, err := ss.User().GetRecentlyActiveUsersForTeam(teamID, 0, 100, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u3),
			sanitized(u1),
			sanitized(u2),
		}, users)
	})

	t.Run("get team 1, offset 0, limit 1", func(t *testing.T) {
		users, err := ss.User().GetRecentlyActiveUsersForTeam(teamID, 0, 1, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u3),
		}, users)
	})

	t.Run("get team 1, offset 2, limit 1", func(t *testing.T) {
		users, err := ss.User().GetRecentlyActiveUsersForTeam(teamID, 2, 1, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u2),
		}, users)
	})
}

func testUserStoreGetNewUsersForTeam(t *testing.T, ss store.Store) {
	teamID := model.NewID()
	teamID2 := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "Yuka",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "Leia",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "Ali",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID2, UserID: u4.ID}, -1)
	require.NoError(t, nErr)

	t.Run("get team 1, offset 0, limit 100", func(t *testing.T) {
		result, err := ss.User().GetNewUsersForTeam(teamID, 0, 100, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u3),
			sanitized(u2),
			sanitized(u1),
		}, result)
	})

	t.Run("get team 1, offset 0, limit 1", func(t *testing.T) {
		result, err := ss.User().GetNewUsersForTeam(teamID, 0, 1, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u3),
		}, result)
	})

	t.Run("get team 1, offset 2, limit 1", func(t *testing.T) {
		result, err := ss.User().GetNewUsersForTeam(teamID, 2, 1, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u1),
		}, result)
	})

	t.Run("get team 2, offset 0, limit 100", func(t *testing.T) {
		result, err := ss.User().GetNewUsersForTeam(teamID2, 0, 100, nil)
		require.NoError(t, err)
		assert.Equal(t, []*model.User{
			sanitized(u4),
		}, result)
	})
}

func assertUsers(t *testing.T, expected, actual []*model.User) {
	expectedUsernames := make([]string, 0, len(expected))
	for _, user := range expected {
		expectedUsernames = append(expectedUsernames, user.Username)
	}

	actualUsernames := make([]string, 0, len(actual))
	for _, user := range actual {
		actualUsernames = append(actualUsernames, user.Username)
	}

	if assert.Equal(t, expectedUsernames, actualUsernames) {
		assert.Equal(t, expected, actual)
	}
}

func testUserStoreSearch(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Username:  "jimbo1" + model.NewID(),
		FirstName: "Tim",
		LastName:  "Bill",
		Nickname:  "Rob",
		Email:     "harold" + model.NewID() + "@simulator.amazonses.com",
		Roles:     "system_user system_admin",
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2 := &model.User{
		Username: "jim2-bobby" + model.NewID(),
		Email:    MakeEmail(),
		Roles:    "system_user system_user_manager",
	}
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3 := &model.User{
		Username: "jimbo3" + model.NewID(),
		Email:    MakeEmail(),
		Roles:    "system_guest",
	}
	_, err = ss.User().Save(u3)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()

	// The users returned from the database will have AuthData as an empty string.
	nilAuthData := new(string)
	*nilAuthData = ""
	u1.AuthData = nilAuthData
	u2.AuthData = nilAuthData
	u3.AuthData = nilAuthData

	t1id := model.NewID()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: t1id, UserID: u1.ID, SchemeAdmin: true, SchemeUser: true}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: t1id, UserID: u2.ID, SchemeAdmin: true, SchemeUser: true}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: t1id, UserID: u3.ID, SchemeAdmin: false, SchemeUser: false, SchemeGuest: true}, -1)
	require.NoError(t, nErr)

	testCases := []struct {
		Description string
		TeamID      string
		Term        string
		Options     *model.UserSearchOptions
		Expected    []*model.User
	}{
		{
			"search jimb, team 1",
			t1id,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1, u3},
		},
		{
			"search jimb, team 1 with team guest and team admin filters without sys admin filter",
			t1id,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
				TeamRoles:      []string{model.TeamGuestRoleID, model.TeamAdminRoleID},
			},
			[]*model.User{u3},
		},
		{
			"search jimb, team 1 with team admin filter and sys admin filter",
			t1id,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
				Roles:          []string{model.SystemAdminRoleID},
				TeamRoles:      []string{model.TeamAdminRoleID},
			},
			[]*model.User{u1},
		},
		{
			"search jim, team 1 with team admin filter",
			t1id,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
				TeamRoles:      []string{model.TeamAdminRoleID},
			},
			[]*model.User{u2},
		},
		{
			"search jim, team 1 with team admin and team guest filter",
			t1id,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
				TeamRoles:      []string{model.TeamAdminRoleID, model.TeamGuestRoleID},
			},
			[]*model.User{u2, u3},
		},
		{
			"search jim, team 1 with team admin and system admin filters",
			t1id,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
				Roles:          []string{model.SystemAdminRoleID},
				TeamRoles:      []string{model.TeamAdminRoleID},
			},
			[]*model.User{u2, u1},
		},
		{
			"search jim, team 1 with system guest filter",
			t1id,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
				Roles:          []string{model.SystemGuestRoleID},
				TeamRoles:      []string{},
			},
			[]*model.User{u3},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			users, err := ss.User().Search(
				testCase.TeamID,
				testCase.Term,
				testCase.Options,
			)
			require.NoError(t, err)
			assertUsers(t, testCase.Expected, users)
		})
	}
}

func testUserStoreSearchNotInChannel(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Username:  "jimbo1" + model.NewID(),
		FirstName: "Tim",
		LastName:  "Bill",
		Nickname:  "Rob",
		Email:     "harold" + model.NewID() + "@simulator.amazonses.com",
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2 := &model.User{
		Username: "jim2-bobby" + model.NewID(),
		Email:    MakeEmail(),
	}
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3 := &model.User{
		Username: "jimbo3" + model.NewID(),
		Email:    MakeEmail(),
		DeleteAt: 1,
	}
	_, err = ss.User().Save(u3)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	tid := model.NewID()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: tid, UserID: u1.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: tid, UserID: u2.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: tid, UserID: u3.ID}, -1)
	require.NoError(t, nErr)

	// The users returned from the database will have AuthData as an empty string.
	nilAuthData := new(string)
	*nilAuthData = ""

	u1.AuthData = nilAuthData
	u2.AuthData = nilAuthData
	u3.AuthData = nilAuthData

	ch1 := model.Channel{
		TeamID:      tid,
		DisplayName: "NameName",
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	c1, nErr := ss.Channel().Save(&ch1, -1)
	require.NoError(t, nErr)

	ch2 := model.Channel{
		TeamID:      tid,
		DisplayName: "NameName",
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	c2, nErr := ss.Channel().Save(&ch2, -1)
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)
	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u3.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)
	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	testCases := []struct {
		Description string
		TeamID      string
		ChannelID   string
		Term        string
		Options     *model.UserSearchOptions
		Expected    []*model.User
	}{
		{
			"search jimb, channel 1",
			tid,
			c1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1},
		},
		{
			"search jimb, allow inactive, channel 1",
			tid,
			c1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1},
		},
		{
			"search jimb, channel 1, no team id",
			"",
			c1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1},
		},
		{
			"search jimb, channel 1, junk team id",
			"junk",
			c1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jimb, channel 2",
			tid,
			c2.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jimb, allow inactive, channel 2",
			tid,
			c2.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u3},
		},
		{
			"search jimb, channel 2, no team id",
			"",
			c2.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jimb, channel 2, junk team id",
			"junk",
			c2.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jim, channel 1",
			tid,
			c1.ID,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u2, u1},
		},
		{
			"search jim, channel 1, limit 1",
			tid,
			c1.ID,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          1,
			},
			[]*model.User{u2},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			users, err := ss.User().SearchNotInChannel(
				testCase.TeamID,
				testCase.ChannelID,
				testCase.Term,
				testCase.Options,
			)
			require.NoError(t, err)
			assertUsers(t, testCase.Expected, users)
		})
	}
}

func testUserStoreSearchInChannel(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Username:  "jimbo1" + model.NewID(),
		FirstName: "Tim",
		LastName:  "Bill",
		Nickname:  "Rob",
		Email:     "harold" + model.NewID() + "@simulator.amazonses.com",
		Roles:     "system_user system_admin",
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2 := &model.User{
		Username: "jim-bobby" + model.NewID(),
		Email:    MakeEmail(),
		Roles:    "system_user",
	}
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3 := &model.User{
		Username: "jimbo3" + model.NewID(),
		Email:    MakeEmail(),
		DeleteAt: 1,
		Roles:    "system_user",
	}
	_, err = ss.User().Save(u3)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	tid := model.NewID()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: tid, UserID: u1.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: tid, UserID: u2.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: tid, UserID: u3.ID}, -1)
	require.NoError(t, nErr)

	// The users returned from the database will have AuthData as an empty string.
	nilAuthData := new(string)
	*nilAuthData = ""

	u1.AuthData = nilAuthData
	u2.AuthData = nilAuthData
	u3.AuthData = nilAuthData

	ch1 := model.Channel{
		TeamID:      tid,
		DisplayName: "NameName",
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	c1, nErr := ss.Channel().Save(&ch1, -1)
	require.NoError(t, nErr)

	ch2 := model.Channel{
		TeamID:      tid,
		DisplayName: "NameName",
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	c2, nErr := ss.Channel().Save(&ch2, -1)
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
		SchemeAdmin: true,
		SchemeUser:  true,
	})
	require.NoError(t, nErr)
	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
		SchemeAdmin: false,
		SchemeUser:  true,
	})
	require.NoError(t, nErr)
	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u3.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
		SchemeAdmin: false,
		SchemeUser:  true,
	})
	require.NoError(t, nErr)

	testCases := []struct {
		Description string
		ChannelID   string
		Term        string
		Options     *model.UserSearchOptions
		Expected    []*model.User
	}{
		{
			"search jimb, channel 1",
			c1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1},
		},
		{
			"search jimb, allow inactive, channel 1",
			c1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1, u3},
		},
		{
			"search jimb, allow inactive, channel 1, limit 1",
			c1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          1,
			},
			[]*model.User{u1},
		},
		{
			"search jimb, channel 2",
			c2.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jimb, allow inactive, channel 2",
			c2.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jim, allow inactive, channel 1 with system admin filter",
			c1.ID,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
				Roles:          []string{model.SystemAdminRoleID},
			},
			[]*model.User{u1},
		},
		{
			"search jim, allow inactive, channel 1 with system admin and system user filter",
			c1.ID,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
				Roles:          []string{model.SystemAdminRoleID, model.SystemUserRoleID},
			},
			[]*model.User{u1, u3},
		},
		{
			"search jim, allow inactive, channel 1 with channel user filter",
			c1.ID,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
				ChannelRoles:   []string{model.ChannelUserRoleID},
			},
			[]*model.User{u3},
		},
		{
			"search jim, allow inactive, channel 1 with channel user and channel admin filter",
			c1.ID,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
				ChannelRoles:   []string{model.ChannelUserRoleID, model.ChannelAdminRoleID},
			},
			[]*model.User{u3},
		},
		{
			"search jim, allow inactive, channel 2 with channel user filter",
			c2.ID,
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
				ChannelRoles:   []string{model.ChannelUserRoleID},
			},
			[]*model.User{u2},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			users, err := ss.User().SearchInChannel(
				testCase.ChannelID,
				testCase.Term,
				testCase.Options,
			)
			require.NoError(t, err)
			assertUsers(t, testCase.Expected, users)
		})
	}
}

func testUserStoreSearchNotInTeam(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Username:  "jimbo1" + model.NewID(),
		FirstName: "Tim",
		LastName:  "Bill",
		Nickname:  "Rob",
		Email:     "harold" + model.NewID() + "@simulator.amazonses.com",
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2 := &model.User{
		Username: "jim-bobby" + model.NewID(),
		Email:    MakeEmail(),
	}
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3 := &model.User{
		Username: "jimbo3" + model.NewID(),
		Email:    MakeEmail(),
		DeleteAt: 1,
	}
	_, err = ss.User().Save(u3)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	u4 := &model.User{
		Username: "simon" + model.NewID(),
		Email:    MakeEmail(),
		DeleteAt: 0,
	}
	_, err = ss.User().Save(u4)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()

	u5 := &model.User{
		Username:  "yu" + model.NewID(),
		FirstName: "En",
		LastName:  "Yu",
		Nickname:  "enyu",
		Email:     MakeEmail(),
	}
	_, err = ss.User().Save(u5)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u5.ID)) }()

	u6 := &model.User{
		Username:  "underscore" + model.NewID(),
		FirstName: "Du_",
		LastName:  "_DE",
		Nickname:  "lodash",
		Email:     MakeEmail(),
	}
	_, err = ss.User().Save(u6)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u6.ID)) }()

	teamID1 := model.NewID()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID1, UserID: u1.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID1, UserID: u2.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID1, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	// u4 is not in team 1
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID1, UserID: u5.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID1, UserID: u6.ID}, -1)
	require.NoError(t, nErr)

	teamID2 := model.NewID()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID2, UserID: u4.ID}, -1)
	require.NoError(t, nErr)

	// The users returned from the database will have AuthData as an empty string.
	nilAuthData := new(string)
	*nilAuthData = ""

	u1.AuthData = nilAuthData
	u2.AuthData = nilAuthData
	u3.AuthData = nilAuthData
	u4.AuthData = nilAuthData
	u5.AuthData = nilAuthData
	u6.AuthData = nilAuthData

	testCases := []struct {
		Description string
		TeamID      string
		Term        string
		Options     *model.UserSearchOptions
		Expected    []*model.User
	}{
		{
			"search simo, team 1",
			teamID1,
			"simo",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u4},
		},

		{
			"search jimb, team 1",
			teamID1,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jimb, allow inactive, team 1",
			teamID1,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search simo, team 2",
			teamID2,
			"simo",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jimb, team2",
			teamID2,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1},
		},
		{
			"search jimb, allow inactive, team 2",
			teamID2,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1, u3},
		},
		{
			"search jimb, allow inactive, team 2, limit 1",
			teamID2,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          1,
			},
			[]*model.User{u1},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			users, err := ss.User().SearchNotInTeam(
				testCase.TeamID,
				testCase.Term,
				testCase.Options,
			)
			require.NoError(t, err)
			assertUsers(t, testCase.Expected, users)
		})
	}
}

func testUserStoreSearchWithoutTeam(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Username:  "jimbo1" + model.NewID(),
		FirstName: "Tim",
		LastName:  "Bill",
		Nickname:  "Rob",
		Email:     "harold" + model.NewID() + "@simulator.amazonses.com",
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2 := &model.User{
		Username: "jim2-bobby" + model.NewID(),
		Email:    MakeEmail(),
	}
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3 := &model.User{
		Username: "jimbo3" + model.NewID(),
		Email:    MakeEmail(),
		DeleteAt: 1,
	}
	_, err = ss.User().Save(u3)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	tid := model.NewID()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: tid, UserID: u3.ID}, -1)
	require.NoError(t, nErr)

	// The users returned from the database will have AuthData as an empty string.
	nilAuthData := new(string)
	*nilAuthData = ""

	u1.AuthData = nilAuthData
	u2.AuthData = nilAuthData
	u3.AuthData = nilAuthData

	testCases := []struct {
		Description string
		Term        string
		Options     *model.UserSearchOptions
		Expected    []*model.User
	}{
		{
			"empty string",
			"",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u2, u1},
		},
		{
			"jim",
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u2, u1},
		},
		{
			"PLT-8354",
			"* ",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u2, u1},
		},
		{
			"jim, limit 1",
			"jim",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          1,
			},
			[]*model.User{u2},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			users, err := ss.User().SearchWithoutTeam(
				testCase.Term,
				testCase.Options,
			)
			require.NoError(t, err)
			assertUsers(t, testCase.Expected, users)
		})
	}
}

func testUserStoreSearchInGroup(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Username:  "jimbo1" + model.NewID(),
		FirstName: "Tim",
		LastName:  "Bill",
		Nickname:  "Rob",
		Email:     "harold" + model.NewID() + "@simulator.amazonses.com",
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2 := &model.User{
		Username: "jim-bobby" + model.NewID(),
		Email:    MakeEmail(),
	}
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	u3 := &model.User{
		Username: "jimbo3" + model.NewID(),
		Email:    MakeEmail(),
		DeleteAt: 1,
	}
	_, err = ss.User().Save(u3)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()

	// The users returned from the database will have AuthData as an empty string.
	nilAuthData := model.NewString("")

	u1.AuthData = nilAuthData
	u2.AuthData = nilAuthData
	u3.AuthData = nilAuthData

	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	_, err = ss.Group().Create(g1)
	require.NoError(t, err)

	g2 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	_, err = ss.Group().Create(g2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(g1.ID, u1.ID)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(g2.ID, u2.ID)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(g1.ID, u3.ID)
	require.NoError(t, err)

	testCases := []struct {
		Description string
		GroupID     string
		Term        string
		Options     *model.UserSearchOptions
		Expected    []*model.User
	}{
		{
			"search jimb, group 1",
			g1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1},
		},
		{
			"search jimb, group 1, allow inactive",
			g1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{u1, u3},
		},
		{
			"search jimb, group 1, limit 1",
			g1.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          1,
			},
			[]*model.User{u1},
		},
		{
			"search jimb, group 2",
			g2.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
		{
			"search jimb, allow inactive, group 2",
			g2.ID,
			"jimb",
			&model.UserSearchOptions{
				AllowFullNames: true,
				AllowInactive:  true,
				Limit:          model.UserSearchDefaultLimit,
			},
			[]*model.User{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			users, err := ss.User().SearchInGroup(
				testCase.GroupID,
				testCase.Term,
				testCase.Options,
			)
			require.NoError(t, err)
			assertUsers(t, testCase.Expected, users)
		})
	}
}

func testCount(t *testing.T, ss store.Store) {
	// Regular
	teamID := model.NewID()
	channelID := model.NewID()
	regularUser := &model.User{}
	regularUser.Email = MakeEmail()
	regularUser.Roles = model.SystemUserRoleID
	_, err := ss.User().Save(regularUser)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(regularUser.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: regularUser.ID, SchemeAdmin: false, SchemeUser: true}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{UserID: regularUser.ID, ChannelID: channelID, SchemeAdmin: false, SchemeUser: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
	require.NoError(t, nErr)

	guestUser := &model.User{}
	guestUser.Email = MakeEmail()
	guestUser.Roles = model.SystemGuestRoleID
	_, err = ss.User().Save(guestUser)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(guestUser.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: guestUser.ID, SchemeAdmin: false, SchemeUser: false, SchemeGuest: true}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{UserID: guestUser.ID, ChannelID: channelID, SchemeAdmin: false, SchemeUser: false, SchemeGuest: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
	require.NoError(t, nErr)

	teamAdmin := &model.User{}
	teamAdmin.Email = MakeEmail()
	teamAdmin.Roles = model.SystemUserRoleID
	_, err = ss.User().Save(teamAdmin)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(teamAdmin.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: teamAdmin.ID, SchemeAdmin: true, SchemeUser: true}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{UserID: teamAdmin.ID, ChannelID: channelID, SchemeAdmin: true, SchemeUser: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
	require.NoError(t, nErr)

	sysAdmin := &model.User{}
	sysAdmin.Email = MakeEmail()
	sysAdmin.Roles = model.SystemAdminRoleID + " " + model.SystemUserRoleID
	_, err = ss.User().Save(sysAdmin)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(sysAdmin.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: sysAdmin.ID, SchemeAdmin: false, SchemeUser: true}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{UserID: sysAdmin.ID, ChannelID: channelID, SchemeAdmin: true, SchemeUser: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
	require.NoError(t, nErr)

	// Deleted
	deletedUser := &model.User{}
	deletedUser.Email = MakeEmail()
	deletedUser.DeleteAt = model.GetMillis()
	_, err = ss.User().Save(deletedUser)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(deletedUser.ID)) }()

	// Bot
	botUser, err := ss.User().Save(&model.User{
		Email: MakeEmail(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(botUser.ID)) }()
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   botUser.ID,
		Username: botUser.Username,
		OwnerID:  regularUser.ID,
	})
	require.NoError(t, nErr)
	botUser.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(botUser.ID)) }()

	testCases := []struct {
		Description string
		Options     model.UserCountOptions
		Expected    int64
	}{
		{
			"No bot accounts no deleted accounts and no team id",
			model.UserCountOptions{
				IncludeBotAccounts: false,
				IncludeDeleted:     false,
				TeamID:             "",
			},
			4,
		},
		{
			"Include bot accounts no deleted accounts and no team id",
			model.UserCountOptions{
				IncludeBotAccounts: true,
				IncludeDeleted:     false,
				TeamID:             "",
			},
			5,
		},
		{
			"Include delete accounts no bots and no team id",
			model.UserCountOptions{
				IncludeBotAccounts: false,
				IncludeDeleted:     true,
				TeamID:             "",
			},
			5,
		},
		{
			"Include bot accounts and deleted accounts and no team id",
			model.UserCountOptions{
				IncludeBotAccounts: true,
				IncludeDeleted:     true,
				TeamID:             "",
			},
			6,
		},
		{
			"Include bot accounts, deleted accounts, exclude regular users with no team id",
			model.UserCountOptions{
				IncludeBotAccounts:  true,
				IncludeDeleted:      true,
				ExcludeRegularUsers: true,
				TeamID:              "",
			},
			1,
		},
		{
			"Include bot accounts and deleted accounts with existing team id",
			model.UserCountOptions{
				IncludeBotAccounts: true,
				IncludeDeleted:     true,
				TeamID:             teamID,
			},
			4,
		},
		{
			"Include bot accounts and deleted accounts with fake team id",
			model.UserCountOptions{
				IncludeBotAccounts: true,
				IncludeDeleted:     true,
				TeamID:             model.NewID(),
			},
			0,
		},
		{
			"Include bot accounts and deleted accounts with existing team id and view restrictions allowing team",
			model.UserCountOptions{
				IncludeBotAccounts: true,
				IncludeDeleted:     true,
				TeamID:             teamID,
				ViewRestrictions:   &model.ViewUsersRestrictions{Teams: []string{teamID}},
			},
			4,
		},
		{
			"Include bot accounts and deleted accounts with existing team id and view restrictions not allowing current team",
			model.UserCountOptions{
				IncludeBotAccounts: true,
				IncludeDeleted:     true,
				TeamID:             teamID,
				ViewRestrictions:   &model.ViewUsersRestrictions{Teams: []string{model.NewID()}},
			},
			0,
		},
		{
			"Filter by system admins only",
			model.UserCountOptions{
				TeamID: teamID,
				Roles:  []string{model.SystemAdminRoleID},
			},
			1,
		},
		{
			"Filter by system users only",
			model.UserCountOptions{
				TeamID: teamID,
				Roles:  []string{model.SystemUserRoleID},
			},
			2,
		},
		{
			"Filter by system guests only",
			model.UserCountOptions{
				TeamID: teamID,
				Roles:  []string{model.SystemGuestRoleID},
			},
			1,
		},
		{
			"Filter by system admins and system users",
			model.UserCountOptions{
				TeamID: teamID,
				Roles:  []string{model.SystemAdminRoleID, model.SystemUserRoleID},
			},
			3,
		},
		{
			"Filter by system admins, system user and system guests",
			model.UserCountOptions{
				TeamID: teamID,
				Roles:  []string{model.SystemAdminRoleID, model.SystemUserRoleID, model.SystemGuestRoleID},
			},
			4,
		},
		{
			"Filter by team admins",
			model.UserCountOptions{
				TeamID:    teamID,
				TeamRoles: []string{model.TeamAdminRoleID},
			},
			1,
		},
		{
			"Filter by team members",
			model.UserCountOptions{
				TeamID:    teamID,
				TeamRoles: []string{model.TeamUserRoleID},
			},
			1,
		},
		{
			"Filter by team guests",
			model.UserCountOptions{
				TeamID:    teamID,
				TeamRoles: []string{model.TeamGuestRoleID},
			},
			1,
		},
		{
			"Filter by team guests and any system role",
			model.UserCountOptions{
				TeamID:    teamID,
				TeamRoles: []string{model.TeamGuestRoleID},
				Roles:     []string{model.SystemAdminRoleID},
			},
			2,
		},
		{
			"Filter by channel members",
			model.UserCountOptions{
				ChannelID:    channelID,
				ChannelRoles: []string{model.ChannelUserRoleID},
			},
			1,
		},
		{
			"Filter by channel members and system admins",
			model.UserCountOptions{
				ChannelID:    channelID,
				Roles:        []string{model.SystemAdminRoleID},
				ChannelRoles: []string{model.ChannelUserRoleID},
			},
			2,
		},
		{
			"Filter by channel members and system admins and channel admins",
			model.UserCountOptions{
				ChannelID:    channelID,
				Roles:        []string{model.SystemAdminRoleID},
				ChannelRoles: []string{model.ChannelUserRoleID, model.ChannelAdminRoleID},
			},
			3,
		},
		{
			"Filter by channel guests",
			model.UserCountOptions{
				ChannelID:    channelID,
				ChannelRoles: []string{model.ChannelGuestRoleID},
			},
			1,
		},
		{
			"Filter by channel guests and any system role",
			model.UserCountOptions{
				ChannelID:    channelID,
				ChannelRoles: []string{model.ChannelGuestRoleID},
				Roles:        []string{model.SystemAdminRoleID},
			},
			2,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			count, err := ss.User().Count(testCase.Options)
			require.NoError(t, err)
			require.Equal(t, testCase.Expected, count)
		})
	}
}

func testUserStoreAnalyticsActiveCount(t *testing.T, ss store.Store, s SqlStore) {

	cleanupStatusStore(t, s)

	// Create 5 users statuses u0, u1, u2, u3, u4.
	// u4 is also a bot
	u0, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u0" + model.NewID(),
	})
	require.NoError(t, err)
	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, ss.User().PermanentDelete(u0.ID))
		require.NoError(t, ss.User().PermanentDelete(u1.ID))
		require.NoError(t, ss.User().PermanentDelete(u2.ID))
		require.NoError(t, ss.User().PermanentDelete(u3.ID))
		require.NoError(t, ss.User().PermanentDelete(u4.ID))
	}()

	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:   u4.ID,
		Username: u4.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)

	millis := model.GetMillis()
	millisTwoDaysAgo := model.GetMillis() - (2 * DayMilliseconds)
	millisTwoMonthsAgo := model.GetMillis() - (2 * MonthMilliseconds)

	// u0 last activity status is two months ago.
	// u1 last activity status is two days ago.
	// u2, u3, u4 last activity is within last day
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u0.ID, Status: model.StatusOffline, LastActivityAt: millisTwoMonthsAgo}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u1.ID, Status: model.StatusOffline, LastActivityAt: millisTwoDaysAgo}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u2.ID, Status: model.StatusOffline, LastActivityAt: millis}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u3.ID, Status: model.StatusOffline, LastActivityAt: millis}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u4.ID, Status: model.StatusOffline, LastActivityAt: millis}))

	// Daily counts (without bots)
	count, err := ss.User().AnalyticsActiveCount(DayMilliseconds, model.UserCountOptions{IncludeBotAccounts: false, IncludeDeleted: true})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Daily counts (with bots)
	count, err = ss.User().AnalyticsActiveCount(DayMilliseconds, model.UserCountOptions{IncludeBotAccounts: true, IncludeDeleted: true})
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Monthly counts (without bots)
	count, err = ss.User().AnalyticsActiveCount(MonthMilliseconds, model.UserCountOptions{IncludeBotAccounts: false, IncludeDeleted: true})
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Monthly counts - (with bots)
	count, err = ss.User().AnalyticsActiveCount(MonthMilliseconds, model.UserCountOptions{IncludeBotAccounts: true, IncludeDeleted: true})
	require.NoError(t, err)
	assert.Equal(t, int64(4), count)

	// Monthly counts - (with bots, excluding deleted)
	count, err = ss.User().AnalyticsActiveCount(MonthMilliseconds, model.UserCountOptions{IncludeBotAccounts: true, IncludeDeleted: false})
	require.NoError(t, err)
	assert.Equal(t, int64(4), count)
}

func testUserStoreAnalyticsActiveCountForPeriod(t *testing.T, ss store.Store, s SqlStore) {

	cleanupStatusStore(t, s)

	// Create 5 users statuses u0, u1, u2, u3, u4.
	// u4 is also a bot
	u0, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u0" + model.NewID(),
	})
	require.NoError(t, err)
	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, ss.User().PermanentDelete(u0.ID))
		require.NoError(t, ss.User().PermanentDelete(u1.ID))
		require.NoError(t, ss.User().PermanentDelete(u2.ID))
		require.NoError(t, ss.User().PermanentDelete(u3.ID))
		require.NoError(t, ss.User().PermanentDelete(u4.ID))
	}()

	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:   u4.ID,
		Username: u4.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)

	millis := model.GetMillis()
	millisTwoDaysAgo := model.GetMillis() - (2 * DayMilliseconds)
	millisTwoMonthsAgo := model.GetMillis() - (2 * MonthMilliseconds)

	// u0 last activity status is two months ago.
	// u1 last activity status is one month ago
	// u2 last activiy is two days ago
	// u2 last activity is one day ago
	// u3 last activity is within last day
	// u4 last activity is within last day
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u0.ID, Status: model.StatusOffline, LastActivityAt: millisTwoMonthsAgo}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u1.ID, Status: model.StatusOffline, LastActivityAt: millisTwoMonthsAgo + MonthMilliseconds}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u2.ID, Status: model.StatusOffline, LastActivityAt: millisTwoDaysAgo}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u3.ID, Status: model.StatusOffline, LastActivityAt: millisTwoDaysAgo + DayMilliseconds}))
	require.NoError(t, ss.Status().SaveOrUpdate(&model.Status{UserID: u4.ID, Status: model.StatusOffline, LastActivityAt: millis}))

	// Two months to two days (without bots)
	count, nerr := ss.User().AnalyticsActiveCountForPeriod(millisTwoMonthsAgo, millisTwoDaysAgo, model.UserCountOptions{IncludeBotAccounts: false, IncludeDeleted: false})
	require.NoError(t, nerr)
	assert.Equal(t, int64(2), count)

	// Two months to two days (without bots)
	count, nerr = ss.User().AnalyticsActiveCountForPeriod(millisTwoMonthsAgo, millisTwoDaysAgo, model.UserCountOptions{IncludeBotAccounts: false, IncludeDeleted: true})
	require.NoError(t, nerr)
	assert.Equal(t, int64(2), count)

	// Two days to present - (with bots)
	count, nerr = ss.User().AnalyticsActiveCountForPeriod(millisTwoDaysAgo, millis, model.UserCountOptions{IncludeBotAccounts: true, IncludeDeleted: false})
	require.NoError(t, nerr)
	assert.Equal(t, int64(2), count)

	// Two days to present - (with bots, excluding deleted)
	count, nerr = ss.User().AnalyticsActiveCountForPeriod(millisTwoDaysAgo, millis, model.UserCountOptions{IncludeBotAccounts: true, IncludeDeleted: true})
	require.NoError(t, nerr)
	assert.Equal(t, int64(2), count)
}

func testUserStoreAnalyticsGetInactiveUsersCount(t *testing.T, ss store.Store) {
	u1 := &model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	count, err := ss.User().AnalyticsGetInactiveUsersCount()
	require.NoError(t, err)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.DeleteAt = model.GetMillis()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	newCount, err := ss.User().AnalyticsGetInactiveUsersCount()
	require.NoError(t, err)
	require.Equal(t, count, newCount-1, "Expected 1 more inactive users but found otherwise.")
}

func testUserStoreAnalyticsGetSystemAdminCount(t *testing.T, ss store.Store) {
	countBefore, err := ss.User().AnalyticsGetSystemAdminCount()
	require.NoError(t, err)

	u1 := model.User{}
	u1.Email = MakeEmail()
	u1.Username = model.NewID()
	u1.Roles = "system_user system_admin"

	u2 := model.User{}
	u2.Email = MakeEmail()
	u2.Username = model.NewID()

	_, nErr := ss.User().Save(&u1)
	require.NoError(t, nErr, "couldn't save user")
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	_, nErr = ss.User().Save(&u2)
	require.NoError(t, nErr, "couldn't save user")

	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	result, err := ss.User().AnalyticsGetSystemAdminCount()
	require.NoError(t, err)
	require.Equal(t, countBefore+1, result, "Did not get the expected number of system admins.")

}

func testUserStoreAnalyticsGetGuestCount(t *testing.T, ss store.Store) {
	countBefore, err := ss.User().AnalyticsGetGuestCount()
	require.NoError(t, err)

	u1 := model.User{}
	u1.Email = MakeEmail()
	u1.Username = model.NewID()
	u1.Roles = "system_user system_admin"

	u2 := model.User{}
	u2.Email = MakeEmail()
	u2.Username = model.NewID()
	u2.Roles = "system_user"

	u3 := model.User{}
	u3.Email = MakeEmail()
	u3.Username = model.NewID()
	u3.Roles = "system_guest"

	_, nErr := ss.User().Save(&u1)
	require.NoError(t, nErr, "couldn't save user")
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	_, nErr = ss.User().Save(&u2)
	require.NoError(t, nErr, "couldn't save user")
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	_, nErr = ss.User().Save(&u3)
	require.NoError(t, nErr, "couldn't save user")
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()

	result, err := ss.User().AnalyticsGetGuestCount()
	require.NoError(t, err)
	require.Equal(t, countBefore+1, result, "Did not get the expected number of guests.")
}

func testUserStoreAnalyticsGetExternalUsers(t *testing.T, ss store.Store) {
	localHostDomain := "mattermost.com"
	result, err := ss.User().AnalyticsGetExternalUsers(localHostDomain)
	require.NoError(t, err)
	assert.False(t, result)

	u1 := model.User{}
	u1.Email = "a@mattermost.com"
	u1.Username = model.NewID()
	u1.Roles = "system_user system_admin"

	u2 := model.User{}
	u2.Email = "b@example.com"
	u2.Username = model.NewID()
	u2.Roles = "system_user"

	u3 := model.User{}
	u3.Email = "c@test.com"
	u3.Username = model.NewID()
	u3.Roles = "system_guest"

	_, err = ss.User().Save(&u1)
	require.NoError(t, err, "couldn't save user")
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	_, err = ss.User().Save(&u2)
	require.NoError(t, err, "couldn't save user")
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()

	_, err = ss.User().Save(&u3)
	require.NoError(t, err, "couldn't save user")
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()

	result, err = ss.User().AnalyticsGetExternalUsers(localHostDomain)
	require.NoError(t, err)
	assert.True(t, result)
}

func testUserStoreGetProfilesNotInTeam(t *testing.T, ss store.Store) {
	team, err := ss.Team().Save(&model.Team{
		DisplayName: "Team",
		Name:        "zz" + model.NewID(),
		Type:        model.TeamOpen,
	})
	require.NoError(t, err)

	teamID := team.ID
	teamID2 := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	// Ensure update at timestamp changes
	time.Sleep(time.Millisecond)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID2, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	// Ensure update at timestamp changes
	time.Sleep(time.Millisecond)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	var etag1, etag2, etag3 string

	t.Run("etag for profiles not in team 1", func(t *testing.T) {
		etag1 = ss.User().GetEtagForProfilesNotInTeam(teamID)
	})

	t.Run("get not in team 1, offset 0, limit 100000", func(t *testing.T) {
		users, userErr := ss.User().GetProfilesNotInTeam(teamID, false, 0, 100000, nil)
		require.NoError(t, userErr)
		assert.Equal(t, []*model.User{
			sanitized(u2),
			sanitized(u3),
		}, users)
	})

	t.Run("get not in team 1, offset 1, limit 1", func(t *testing.T) {
		users, userErr := ss.User().GetProfilesNotInTeam(teamID, false, 1, 1, nil)
		require.NoError(t, userErr)
		assert.Equal(t, []*model.User{
			sanitized(u3),
		}, users)
	})

	t.Run("get not in team 2, offset 0, limit 100", func(t *testing.T) {
		users, userErr := ss.User().GetProfilesNotInTeam(teamID2, false, 0, 100, nil)
		require.NoError(t, userErr)
		assert.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u3),
		}, users)
	})

	// Ensure update at timestamp changes
	time.Sleep(time.Millisecond)

	// Add u2 to team 1
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)
	u2.UpdateAt, err = ss.User().UpdateUpdateAt(u2.ID)
	require.NoError(t, err)

	t.Run("etag for profiles not in team 1 after update", func(t *testing.T) {
		etag2 = ss.User().GetEtagForProfilesNotInTeam(teamID)
		require.NotEqual(t, etag2, etag1, "etag should have changed")
	})

	t.Run("get not in team 1, offset 0, limit 100000 after update", func(t *testing.T) {
		users, userErr := ss.User().GetProfilesNotInTeam(teamID, false, 0, 100000, nil)
		require.NoError(t, userErr)
		assert.Equal(t, []*model.User{
			sanitized(u3),
		}, users)
	})

	// Ensure update at timestamp changes
	time.Sleep(time.Millisecond)

	e := ss.Team().RemoveMember(teamID, u1.ID)
	require.NoError(t, e)
	e = ss.Team().RemoveMember(teamID, u2.ID)
	require.NoError(t, e)

	u1.UpdateAt, err = ss.User().UpdateUpdateAt(u1.ID)
	require.NoError(t, err)
	u2.UpdateAt, err = ss.User().UpdateUpdateAt(u2.ID)
	require.NoError(t, err)

	t.Run("etag for profiles not in team 1 after second update", func(t *testing.T) {
		etag3 = ss.User().GetEtagForProfilesNotInTeam(teamID)
		require.NotEqual(t, etag1, etag3, "etag should have changed")
		require.NotEqual(t, etag2, etag3, "etag should have changed")
	})

	t.Run("get not in team 1, offset 0, limit 100000 after second update", func(t *testing.T) {
		users, userErr := ss.User().GetProfilesNotInTeam(teamID, false, 0, 100000, nil)
		require.NoError(t, userErr)
		assert.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
			sanitized(u3),
		}, users)
	})

	// Ensure update at timestamp changes
	time.Sleep(time.Millisecond)

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u4.ID}, -1)
	require.NoError(t, nErr)

	t.Run("etag for profiles not in team 1 after addition to team", func(t *testing.T) {
		etag4 := ss.User().GetEtagForProfilesNotInTeam(teamID)
		require.Equal(t, etag3, etag4, "etag should not have changed")
	})

	// Add u3 to team 2
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID2, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	u3.UpdateAt, err = ss.User().UpdateUpdateAt(u3.ID)
	require.NoError(t, err)

	// GetEtagForProfilesNotInTeam produces a new etag every time a member, not
	// in the team, gets a new UpdateAt value. In the case that an older member
	// in the set joins a different team, their UpdateAt value changes, thus
	// creating a new etag (even though the user set doesn't change). A hashing
	// solution, which only uses UserIds, would solve this issue.
	t.Run("etag for profiles not in team 1 after u3 added to team 2", func(t *testing.T) {
		t.Skip()
		etag4 := ss.User().GetEtagForProfilesNotInTeam(teamID)
		require.Equal(t, etag3, etag4, "etag should not have changed")
	})

	t.Run("get not in team 1, offset 0, limit 100000 after second update, setting group constrained when it's not", func(t *testing.T) {
		users, userErr := ss.User().GetProfilesNotInTeam(teamID, true, 0, 100000, nil)
		require.NoError(t, userErr)
		assert.Empty(t, users)
	})

	// create a group
	group, err := ss.Group().Create(&model.Group{
		Name:        model.NewString("n_" + model.NewID()),
		DisplayName: "dn_" + model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    "ri_" + model.NewID(),
	})
	require.NoError(t, err)

	// add two members to the group
	for _, u := range []*model.User{u1, u2} {
		_, err = ss.Group().UpsertMember(group.ID, u.ID)
		require.NoError(t, err)
	}

	// associate the group with the team
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		GroupID:    group.ID,
		SyncableID: teamID,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.NoError(t, err)

	t.Run("get not in team 1, offset 0, limit 100000 after second update, setting group constrained", func(t *testing.T) {
		users, userErr := ss.User().GetProfilesNotInTeam(teamID, true, 0, 100000, nil)
		require.NoError(t, userErr)
		assert.Equal(t, []*model.User{
			sanitized(u1),
			sanitized(u2),
		}, users)
	})
}

func testUserStoreClearAllCustomRoleAssignments(t *testing.T, ss store.Store) {
	u1 := model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
		Roles:    "system_user system_admin system_post_all",
	}
	u2 := model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
		Roles:    "system_user custom_role system_admin another_custom_role",
	}
	u3 := model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
		Roles:    "system_user",
	}
	u4 := model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
		Roles:    "custom_only",
	}

	_, err := ss.User().Save(&u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, err = ss.User().Save(&u2)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, err = ss.User().Save(&u3)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, err = ss.User().Save(&u4)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()

	require.NoError(t, ss.User().ClearAllCustomRoleAssignments())

	r1, err := ss.User().GetByUsername(u1.Username)
	require.NoError(t, err)
	assert.Equal(t, u1.Roles, r1.Roles)

	r2, err1 := ss.User().GetByUsername(u2.Username)
	require.NoError(t, err1)
	assert.Equal(t, "system_user system_admin", r2.Roles)

	r3, err2 := ss.User().GetByUsername(u3.Username)
	require.NoError(t, err2)
	assert.Equal(t, u3.Roles, r3.Roles)

	r4, err3 := ss.User().GetByUsername(u4.Username)
	require.NoError(t, err3)
	assert.Equal(t, "", r4.Roles)
}

func testUserStoreGetAllAfter(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
		Roles:    "system_user system_admin system_post_all",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr := ss.Bot().Save(&model.Bot{
		UserID:   u2.ID,
		Username: u2.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u2.IsBot = true
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u2.ID)) }()

	expected := []*model.User{u1, u2}
	if strings.Compare(u2.ID, u1.ID) < 0 {
		expected = []*model.User{u2, u1}
	}

	t.Run("get after lowest possible id", func(t *testing.T) {
		actual, err := ss.User().GetAllAfter(10000, strings.Repeat("0", 26))
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})

	t.Run("get after first user", func(t *testing.T) {
		actual, err := ss.User().GetAllAfter(10000, expected[0].ID)
		require.NoError(t, err)

		assert.Equal(t, []*model.User{expected[1]}, actual)
	})

	t.Run("get after second user", func(t *testing.T) {
		actual, err := ss.User().GetAllAfter(10000, expected[1].ID)
		require.NoError(t, err)

		assert.Equal(t, []*model.User{}, actual)
	})
}

func testUserStoreGetUsersBatchForIndexing(t *testing.T, ss store.Store) {
	// Set up all the objects needed
	t1, err := ss.Team().Save(&model.Team{
		DisplayName: "Team1",
		Name:        "zz" + model.NewID(),
		Type:        model.TeamOpen,
	})
	require.NoError(t, err)

	ch1 := &model.Channel{
		Name: model.NewID(),
		Type: model.ChannelTypeOpen,
	}
	cPub1, nErr := ss.Channel().Save(ch1, -1)
	require.NoError(t, nErr)

	ch2 := &model.Channel{
		Name: model.NewID(),
		Type: model.ChannelTypeOpen,
	}
	cPub2, nErr := ss.Channel().Save(ch2, -1)
	require.NoError(t, nErr)

	ch3 := &model.Channel{
		Name: model.NewID(),
		Type: model.ChannelTypePrivate,
	}

	cPriv, nErr := ss.Channel().Save(ch3, -1)
	require.NoError(t, nErr)

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
		CreateAt: model.GetMillis(),
	})
	require.NoError(t, err)

	time.Sleep(time.Millisecond)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
		CreateAt: model.GetMillis(),
	})
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{
		UserID: u2.ID,
		TeamID: t1.ID,
	}, 100)
	require.NoError(t, nErr)
	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		UserID:      u2.ID,
		ChannelID:   cPub1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)
	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		UserID:      u2.ID,
		ChannelID:   cPub2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	startTime := u2.CreateAt
	time.Sleep(time.Millisecond)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
		CreateAt: model.GetMillis(),
	})
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{
		UserID:   u3.ID,
		TeamID:   t1.ID,
		DeleteAt: model.GetMillis(),
	}, 100)
	require.NoError(t, nErr)
	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		UserID:      u3.ID,
		ChannelID:   cPub2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)
	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		UserID:      u3.ID,
		ChannelID:   cPriv.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	endTime := u3.CreateAt

	// First and last user should be outside the range
	res1List, err := ss.User().GetUsersBatchForIndexing(startTime, endTime, 100)
	require.NoError(t, err)

	assert.Len(t, res1List, 1)
	assert.Equal(t, res1List[0].Username, u2.Username)
	assert.ElementsMatch(t, res1List[0].TeamsIDs, []string{t1.ID})
	assert.ElementsMatch(t, res1List[0].ChannelsIDs, []string{cPub1.ID, cPub2.ID})

	// Update startTime to include first user
	startTime = u1.CreateAt
	res2List, err := ss.User().GetUsersBatchForIndexing(startTime, endTime, 100)
	require.NoError(t, err)

	assert.Len(t, res2List, 2)
	assert.Equal(t, res2List[0].Username, u1.Username)
	assert.Equal(t, res2List[0].ChannelsIDs, []string{})
	assert.Equal(t, res2List[0].TeamsIDs, []string{})
	assert.Equal(t, res2List[1].Username, u2.Username)

	// Update endTime to include last user
	endTime = model.GetMillis()
	res3List, err := ss.User().GetUsersBatchForIndexing(startTime, endTime, 100)
	require.NoError(t, err)

	assert.Len(t, res3List, 3)
	assert.Equal(t, res3List[0].Username, u1.Username)
	assert.Equal(t, res3List[1].Username, u2.Username)
	assert.Equal(t, res3List[2].Username, u3.Username)
	assert.ElementsMatch(t, res3List[2].TeamsIDs, []string{})
	assert.ElementsMatch(t, res3List[2].ChannelsIDs, []string{cPub2.ID})

	// Testing the limit
	res4List, err := ss.User().GetUsersBatchForIndexing(startTime, endTime, 2)
	require.NoError(t, err)

	assert.Len(t, res4List, 2)
	assert.Equal(t, res4List[0].Username, u1.Username)
	assert.Equal(t, res4List[1].Username, u2.Username)
}

func testUserStoreGetTeamGroupUsers(t *testing.T, ss store.Store) {
	// create team
	id := model.NewID()
	team, err := ss.Team().Save(&model.Team{
		DisplayName: "dn_" + id,
		Name:        "n-" + id,
		Email:       id + "@test.com",
		Type:        model.TeamInvite,
	})
	require.NoError(t, err)
	require.NotNil(t, team)

	// create users
	var testUsers []*model.User
	for i := 0; i < 3; i++ {
		id = model.NewID()
		user, userErr := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
		})
		require.NoError(t, userErr)
		require.NotNil(t, user)
		testUsers = append(testUsers, user)
	}
	require.Len(t, testUsers, 3, "testUsers length doesn't meet required length")
	userGroupA, userGroupB, userNoGroup := testUsers[0], testUsers[1], testUsers[2]

	// add non-group-member to the team (to prove that the query isn't just returning all members)
	_, nErr := ss.Team().SaveMember(&model.TeamMember{
		TeamID: team.ID,
		UserID: userNoGroup.ID,
	}, 999)
	require.NoError(t, nErr)

	// create groups
	var testGroups []*model.Group
	for i := 0; i < 2; i++ {
		id = model.NewID()

		var group *model.Group
		group, err = ss.Group().Create(&model.Group{
			Name:        model.NewString("n_" + id),
			DisplayName: "dn_" + id,
			Source:      model.GroupSourceLdap,
			RemoteID:    "ri_" + id,
		})
		require.NoError(t, err)
		require.NotNil(t, group)
		testGroups = append(testGroups, group)
	}
	require.Len(t, testGroups, 2, "testGroups length doesn't meet required length")
	groupA, groupB := testGroups[0], testGroups[1]

	// add members to groups
	_, err = ss.Group().UpsertMember(groupA.ID, userGroupA.ID)
	require.NoError(t, err)
	_, err = ss.Group().UpsertMember(groupB.ID, userGroupB.ID)
	require.NoError(t, err)

	// association one group to team
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		GroupID:    groupA.ID,
		SyncableID: team.ID,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.NoError(t, err)

	var users []*model.User

	requireNUsers := func(n int) {
		users, err = ss.User().GetTeamGroupUsers(team.ID)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.Len(t, users, n)
	}

	// team not group constrained returns users
	requireNUsers(1)

	// update team to be group-constrained
	team.GroupConstrained = model.NewBool(true)
	team, err = ss.Team().Update(team)
	require.NoError(t, err)

	// still returns user (being group-constrained has no effect)
	requireNUsers(1)

	// associate other group to team
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		GroupID:    groupB.ID,
		SyncableID: team.ID,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.NoError(t, err)

	// should return users from all groups
	// 2 users now that both groups have been associated to the team
	requireNUsers(2)

	// add team membership of allowed user
	_, nErr = ss.Team().SaveMember(&model.TeamMember{
		TeamID: team.ID,
		UserID: userGroupA.ID,
	}, 999)
	require.NoError(t, nErr)

	// ensure allowed member still returned by query
	requireNUsers(2)

	// delete team membership of allowed user
	err = ss.Team().RemoveMember(team.ID, userGroupA.ID)
	require.NoError(t, err)

	// ensure removed allowed member still returned by query
	requireNUsers(2)
}

func testUserStoreGetChannelGroupUsers(t *testing.T, ss store.Store) {
	// create channel
	id := model.NewID()
	channel, nErr := ss.Channel().Save(&model.Channel{
		DisplayName: "dn_" + id,
		Name:        "n-" + id,
		Type:        model.ChannelTypePrivate,
	}, 999)
	require.NoError(t, nErr)
	require.NotNil(t, channel)

	// create users
	var testUsers []*model.User
	for i := 0; i < 3; i++ {
		id = model.NewID()
		user, userErr := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
		})
		require.NoError(t, userErr)
		require.NotNil(t, user)
		testUsers = append(testUsers, user)
	}
	require.Len(t, testUsers, 3, "testUsers length doesn't meet required length")
	userGroupA, userGroupB, userNoGroup := testUsers[0], testUsers[1], testUsers[2]

	// add non-group-member to the channel (to prove that the query isn't just returning all members)
	_, err := ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   channel.ID,
		UserID:      userNoGroup.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	// create groups
	var testGroups []*model.Group
	for i := 0; i < 2; i++ {
		id = model.NewID()
		var group *model.Group
		group, err = ss.Group().Create(&model.Group{
			Name:        model.NewString("n_" + id),
			DisplayName: "dn_" + id,
			Source:      model.GroupSourceLdap,
			RemoteID:    "ri_" + id,
		})
		require.NoError(t, err)
		require.NotNil(t, group)
		testGroups = append(testGroups, group)
	}
	require.Len(t, testGroups, 2, "testGroups length doesn't meet required length")
	groupA, groupB := testGroups[0], testGroups[1]

	// add members to groups
	_, err = ss.Group().UpsertMember(groupA.ID, userGroupA.ID)
	require.NoError(t, err)
	_, err = ss.Group().UpsertMember(groupB.ID, userGroupB.ID)
	require.NoError(t, err)

	// association one group to channel
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		GroupID:    groupA.ID,
		SyncableID: channel.ID,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.NoError(t, err)

	var users []*model.User

	requireNUsers := func(n int) {
		users, err = ss.User().GetChannelGroupUsers(channel.ID)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.Len(t, users, n)
	}

	// channel not group constrained returns users
	requireNUsers(1)

	// update team to be group-constrained
	channel.GroupConstrained = model.NewBool(true)
	_, nErr = ss.Channel().Update(channel)
	require.NoError(t, nErr)

	// still returns user (being group-constrained has no effect)
	requireNUsers(1)

	// associate other group to team
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		GroupID:    groupB.ID,
		SyncableID: channel.ID,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.NoError(t, err)

	// should return users from all groups
	// 2 users now that both groups have been associated to the team
	requireNUsers(2)

	// add team membership of allowed user
	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   channel.ID,
		UserID:      userGroupA.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	// ensure allowed member still returned by query
	requireNUsers(2)

	// delete team membership of allowed user
	err = ss.Channel().RemoveMember(channel.ID, userGroupA.ID)
	require.NoError(t, err)

	// ensure removed allowed member still returned by query
	requireNUsers(2)
}

func testUserStorePromoteGuestToUser(t *testing.T, ss store.Store) {
	// create users
	t.Run("Must do nothing with regular user", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: true, SchemeUser: false}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user.ID, SchemeGuest: true, SchemeUser: false, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		err = ss.User().PromoteGuestToUser(user.ID)
		require.NoError(t, err)
		updatedUser, err := ss.User().Get(context.Background(), user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_user", updatedUser.Roles)
		require.True(t, user.UpdateAt < updatedUser.UpdateAt)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedTeamMember.SchemeGuest)
		require.True(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedChannelMember.SchemeGuest)
		require.True(t, updatedChannelMember.SchemeUser)
	})

	t.Run("Must do nothing with admin user", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user system_admin",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: true, SchemeUser: false}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user.ID, SchemeGuest: true, SchemeUser: false, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		err = ss.User().PromoteGuestToUser(user.ID)
		require.NoError(t, err)
		updatedUser, err := ss.User().Get(context.Background(), user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_user system_admin", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedTeamMember.SchemeGuest)
		require.True(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedChannelMember.SchemeGuest)
		require.True(t, updatedChannelMember.SchemeUser)
	})

	t.Run("Must work with guest user without teams or channels", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_guest",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		err = ss.User().PromoteGuestToUser(user.ID)
		require.NoError(t, err)
		updatedUser, err := ss.User().Get(context.Background(), user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_user", updatedUser.Roles)
	})

	t.Run("Must work with guest user with teams but no channels", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_guest",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: true, SchemeUser: false}, 999)
		require.NoError(t, nErr)

		err = ss.User().PromoteGuestToUser(user.ID)
		require.NoError(t, err)
		updatedUser, err := ss.User().Get(context.Background(), user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_user", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedTeamMember.SchemeGuest)
		require.True(t, updatedTeamMember.SchemeUser)
	})

	t.Run("Must work with guest user with teams and channels", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_guest",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: true, SchemeUser: false}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user.ID, SchemeGuest: true, SchemeUser: false, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		err = ss.User().PromoteGuestToUser(user.ID)
		require.NoError(t, err)
		updatedUser, err := ss.User().Get(context.Background(), user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_user", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedTeamMember.SchemeGuest)
		require.True(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedChannelMember.SchemeGuest)
		require.True(t, updatedChannelMember.SchemeUser)
	})

	t.Run("Must work with guest user with teams and channels and custom role", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_guest custom_role",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: true, SchemeUser: false}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user.ID, SchemeGuest: true, SchemeUser: false, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		err = ss.User().PromoteGuestToUser(user.ID)
		require.NoError(t, err)
		updatedUser, err := ss.User().Get(context.Background(), user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_user custom_role", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedTeamMember.SchemeGuest)
		require.True(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user.ID)
		require.NoError(t, nErr)
		require.False(t, updatedChannelMember.SchemeGuest)
		require.True(t, updatedChannelMember.SchemeUser)
	})

	t.Run("Must no change any other user guest role", func(t *testing.T) {
		id := model.NewID()
		user1, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_guest",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user1.ID)) }()

		teamID1 := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID1, UserID: user1.ID, SchemeGuest: true, SchemeUser: false}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID1,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)

		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user1.ID, SchemeGuest: true, SchemeUser: false, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		id = model.NewID()
		user2, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_guest",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user2.ID)) }()

		teamID2 := model.NewID()
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID2, UserID: user2.ID, SchemeGuest: true, SchemeUser: false}, 999)
		require.NoError(t, nErr)

		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user2.ID, SchemeGuest: true, SchemeUser: false, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		err = ss.User().PromoteGuestToUser(user1.ID)
		require.NoError(t, err)
		updatedUser, err := ss.User().Get(context.Background(), user1.ID)
		require.NoError(t, err)
		require.Equal(t, "system_user", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID1, user1.ID)
		require.NoError(t, nErr)
		require.False(t, updatedTeamMember.SchemeGuest)
		require.True(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user1.ID)
		require.NoError(t, nErr)
		require.False(t, updatedChannelMember.SchemeGuest)
		require.True(t, updatedChannelMember.SchemeUser)

		notUpdatedUser, err := ss.User().Get(context.Background(), user2.ID)
		require.NoError(t, err)
		require.Equal(t, "system_guest", notUpdatedUser.Roles)

		notUpdatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID2, user2.ID)
		require.NoError(t, nErr)
		require.True(t, notUpdatedTeamMember.SchemeGuest)
		require.False(t, notUpdatedTeamMember.SchemeUser)

		notUpdatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user2.ID)
		require.NoError(t, nErr)
		require.True(t, notUpdatedChannelMember.SchemeGuest)
		require.False(t, notUpdatedChannelMember.SchemeUser)
	})
}

func testUserStoreDemoteUserToGuest(t *testing.T, ss store.Store) {
	// create users
	t.Run("Must do nothing with guest", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_guest",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: false, SchemeUser: true}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user.ID, SchemeGuest: false, SchemeUser: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		updatedUser, err := ss.User().DemoteUserToGuest(user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_guest", updatedUser.Roles)
		require.True(t, user.UpdateAt < updatedUser.UpdateAt)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, updatedUser.ID)
		require.NoError(t, nErr)
		require.True(t, updatedTeamMember.SchemeGuest)
		require.False(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, updatedUser.ID)
		require.NoError(t, nErr)
		require.True(t, updatedChannelMember.SchemeGuest)
		require.False(t, updatedChannelMember.SchemeUser)
	})

	t.Run("Must demote properly an admin user", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user system_admin",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: true, SchemeUser: false}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user.ID, SchemeGuest: true, SchemeUser: false, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		updatedUser, err := ss.User().DemoteUserToGuest(user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_guest", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.True(t, updatedTeamMember.SchemeGuest)
		require.False(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user.ID)
		require.NoError(t, nErr)
		require.True(t, updatedChannelMember.SchemeGuest)
		require.False(t, updatedChannelMember.SchemeUser)
	})

	t.Run("Must work with user without teams or channels", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		updatedUser, err := ss.User().DemoteUserToGuest(user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_guest", updatedUser.Roles)
	})

	t.Run("Must work with user with teams but no channels", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: false, SchemeUser: true}, 999)
		require.NoError(t, nErr)

		updatedUser, err := ss.User().DemoteUserToGuest(user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_guest", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.True(t, updatedTeamMember.SchemeGuest)
		require.False(t, updatedTeamMember.SchemeUser)
	})

	t.Run("Must work with user with teams and channels", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: false, SchemeUser: true}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user.ID, SchemeGuest: false, SchemeUser: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		updatedUser, err := ss.User().DemoteUserToGuest(user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_guest", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.True(t, updatedTeamMember.SchemeGuest)
		require.False(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user.ID)
		require.NoError(t, nErr)
		require.True(t, updatedChannelMember.SchemeGuest)
		require.False(t, updatedChannelMember.SchemeUser)
	})

	t.Run("Must work with user with teams and channels and custom role", func(t *testing.T) {
		id := model.NewID()
		user, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user custom_role",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()

		teamID := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID, SchemeGuest: false, SchemeUser: true}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user.ID, SchemeGuest: false, SchemeUser: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		updatedUser, err := ss.User().DemoteUserToGuest(user.ID)
		require.NoError(t, err)
		require.Equal(t, "system_guest custom_role", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID, user.ID)
		require.NoError(t, nErr)
		require.True(t, updatedTeamMember.SchemeGuest)
		require.False(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user.ID)
		require.NoError(t, nErr)
		require.True(t, updatedChannelMember.SchemeGuest)
		require.False(t, updatedChannelMember.SchemeUser)
	})

	t.Run("Must no change any other user role", func(t *testing.T) {
		id := model.NewID()
		user1, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user1.ID)) }()

		teamID1 := model.NewID()
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID1, UserID: user1.ID, SchemeGuest: false, SchemeUser: true}, 999)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			TeamID:      teamID1,
			DisplayName: "Channel name",
			Name:        "channel-" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, -1)
		require.NoError(t, nErr)

		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user1.ID, SchemeGuest: false, SchemeUser: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		id = model.NewID()
		user2, err := ss.User().Save(&model.User{
			Email:     id + "@test.com",
			Username:  "un_" + id,
			Nickname:  "nn_" + id,
			FirstName: "f_" + id,
			LastName:  "l_" + id,
			Password:  "Password1",
			Roles:     "system_user",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user2.ID)) }()

		teamID2 := model.NewID()
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID2, UserID: user2.ID, SchemeGuest: false, SchemeUser: true}, 999)
		require.NoError(t, nErr)

		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{ChannelID: channel.ID, UserID: user2.ID, SchemeGuest: false, SchemeUser: true, NotifyProps: model.GetDefaultChannelNotifyProps()})
		require.NoError(t, nErr)

		updatedUser, err := ss.User().DemoteUserToGuest(user1.ID)
		require.NoError(t, err)
		require.Equal(t, "system_guest", updatedUser.Roles)

		updatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID1, user1.ID)
		require.NoError(t, nErr)
		require.True(t, updatedTeamMember.SchemeGuest)
		require.False(t, updatedTeamMember.SchemeUser)

		updatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user1.ID)
		require.NoError(t, nErr)
		require.True(t, updatedChannelMember.SchemeGuest)
		require.False(t, updatedChannelMember.SchemeUser)

		notUpdatedUser, err := ss.User().Get(context.Background(), user2.ID)
		require.NoError(t, err)
		require.Equal(t, "system_user", notUpdatedUser.Roles)

		notUpdatedTeamMember, nErr := ss.Team().GetMember(context.Background(), teamID2, user2.ID)
		require.NoError(t, nErr)
		require.False(t, notUpdatedTeamMember.SchemeGuest)
		require.True(t, notUpdatedTeamMember.SchemeUser)

		notUpdatedChannelMember, nErr := ss.Channel().GetMember(context.Background(), channel.ID, user2.ID)
		require.NoError(t, nErr)
		require.False(t, notUpdatedChannelMember.SchemeGuest)
		require.True(t, notUpdatedChannelMember.SchemeUser)
	})
}

func testDeactivateGuests(t *testing.T, ss store.Store) {
	// create users
	t.Run("Must disable all guests and no regular user or already deactivated users", func(t *testing.T) {
		guest1Random := model.NewID()
		guest1, err := ss.User().Save(&model.User{
			Email:     guest1Random + "@test.com",
			Username:  "un_" + guest1Random,
			Nickname:  "nn_" + guest1Random,
			FirstName: "f_" + guest1Random,
			LastName:  "l_" + guest1Random,
			Password:  "Password1",
			Roles:     "system_guest",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(guest1.ID)) }()

		guest2Random := model.NewID()
		guest2, err := ss.User().Save(&model.User{
			Email:     guest2Random + "@test.com",
			Username:  "un_" + guest2Random,
			Nickname:  "nn_" + guest2Random,
			FirstName: "f_" + guest2Random,
			LastName:  "l_" + guest2Random,
			Password:  "Password1",
			Roles:     "system_guest",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(guest2.ID)) }()

		guest3Random := model.NewID()
		guest3, err := ss.User().Save(&model.User{
			Email:     guest3Random + "@test.com",
			Username:  "un_" + guest3Random,
			Nickname:  "nn_" + guest3Random,
			FirstName: "f_" + guest3Random,
			LastName:  "l_" + guest3Random,
			Password:  "Password1",
			Roles:     "system_guest",
			DeleteAt:  10,
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(guest3.ID)) }()

		regularUserRandom := model.NewID()
		regularUser, err := ss.User().Save(&model.User{
			Email:     regularUserRandom + "@test.com",
			Username:  "un_" + regularUserRandom,
			Nickname:  "nn_" + regularUserRandom,
			FirstName: "f_" + regularUserRandom,
			LastName:  "l_" + regularUserRandom,
			Password:  "Password1",
			Roles:     "system_user",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(regularUser.ID)) }()

		ids, err := ss.User().DeactivateGuests()
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{guest1.ID, guest2.ID}, ids)

		u, err := ss.User().Get(context.Background(), guest1.ID)
		require.NoError(t, err)
		assert.NotEqual(t, u.DeleteAt, int64(0))

		u, err = ss.User().Get(context.Background(), guest2.ID)
		require.NoError(t, err)
		assert.NotEqual(t, u.DeleteAt, int64(0))

		u, err = ss.User().Get(context.Background(), guest3.ID)
		require.NoError(t, err)
		assert.Equal(t, u.DeleteAt, int64(10))

		u, err = ss.User().Get(context.Background(), regularUser.ID)
		require.NoError(t, err)
		assert.Equal(t, u.DeleteAt, int64(0))
	})
}

func testUserStoreResetLastPictureUpdate(t *testing.T, ss store.Store) {
	u1 := &model.User{}
	u1.Email = MakeEmail()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	err = ss.User().UpdateLastPictureUpdate(u1.ID)
	require.NoError(t, err)

	user, err := ss.User().Get(context.Background(), u1.ID)
	require.NoError(t, err)

	assert.NotZero(t, user.LastPictureUpdate)
	assert.NotZero(t, user.UpdateAt)

	// Ensure update at timestamp changes
	time.Sleep(time.Millisecond)

	err = ss.User().ResetLastPictureUpdate(u1.ID)
	require.NoError(t, err)

	ss.User().InvalidateProfileCacheForUser(u1.ID)

	user2, err := ss.User().Get(context.Background(), u1.ID)
	require.NoError(t, err)

	assert.True(t, user2.UpdateAt > user.UpdateAt)
	assert.Zero(t, user2.LastPictureUpdate)
}

func testGetKnownUsers(t *testing.T, ss store.Store) {
	teamID := model.NewID()

	u1, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u1" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u1.ID)) }()
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u2" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u2.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	u3, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u3" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u3.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u3.ID}, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Bot().Save(&model.Bot{
		UserID:   u3.ID,
		Username: u3.Username,
		OwnerID:  u1.ID,
	})
	require.NoError(t, nErr)
	u3.IsBot = true

	defer func() { require.NoError(t, ss.Bot().PermanentDelete(u3.ID)) }()

	u4, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "u4" + model.NewID(),
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.User().PermanentDelete(u4.ID)) }()
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: u4.ID}, -1)
	require.NoError(t, nErr)

	ch1 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in channel",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	c1, nErr := ss.Channel().Save(ch1, -1)
	require.NoError(t, nErr)

	ch2 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in private",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	c2, nErr := ss.Channel().Save(ch2, -1)
	require.NoError(t, nErr)

	ch3 := &model.Channel{
		TeamID:      teamID,
		DisplayName: "Profiles in private",
		Name:        "profiles-" + model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	c3, nErr := ss.Channel().Save(ch3, -1)
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c1.ID,
		UserID:      u2.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u3.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c2.ID,
		UserID:      u1.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   c3.ID,
		UserID:      u4.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	t.Run("get know users sharing no channels", func(t *testing.T) {
		userIDs, err := ss.User().GetKnownUsers(u4.ID)
		require.NoError(t, err)
		assert.Empty(t, userIDs)
	})

	t.Run("get know users sharing one channel", func(t *testing.T) {
		userIDs, err := ss.User().GetKnownUsers(u3.ID)
		require.NoError(t, err)
		assert.Len(t, userIDs, 1)
		assert.Equal(t, userIDs[0], u1.ID)
	})

	t.Run("get know users sharing multiple channels", func(t *testing.T) {
		userIDs, err := ss.User().GetKnownUsers(u1.ID)
		require.NoError(t, err)
		assert.Len(t, userIDs, 2)
		assert.ElementsMatch(t, userIDs, []string{u2.ID, u3.ID})
	})
}
