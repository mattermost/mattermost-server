// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/gorp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/timezones"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type SqlStore interface {
	GetMaster() *gorp.DbMap
	DriverName() string
}

func cleanupChannels(t *testing.T, ss store.Store) {
	list, err := ss.Channel().GetAllChannels(0, 100000, store.ChannelSearchOpts{IncludeDeleted: true})
	require.NoError(t, err, "error cleaning all channels", err)
	for _, channel := range *list {
		err = ss.Channel().PermanentDelete(channel.Id)
		assert.NoError(t, err)
	}
}

func TestChannelStore(t *testing.T, ss store.Store, s SqlStore) {
	createDefaultRoles(ss)

	t.Run("Save", func(t *testing.T) { testChannelStoreSave(t, ss) })
	t.Run("SaveDirectChannel", func(t *testing.T) { testChannelStoreSaveDirectChannel(t, ss, s) })
	t.Run("CreateDirectChannel", func(t *testing.T) { testChannelStoreCreateDirectChannel(t, ss) })
	t.Run("Update", func(t *testing.T) { testChannelStoreUpdate(t, ss) })
	t.Run("GetChannelUnread", func(t *testing.T) { testGetChannelUnread(t, ss) })
	t.Run("Get", func(t *testing.T) { testChannelStoreGet(t, ss, s) })
	t.Run("GetChannelsByIds", func(t *testing.T) { testChannelStoreGetChannelsByIds(t, ss) })
	t.Run("GetForPost", func(t *testing.T) { testChannelStoreGetForPost(t, ss) })
	t.Run("Restore", func(t *testing.T) { testChannelStoreRestore(t, ss) })
	t.Run("Delete", func(t *testing.T) { testChannelStoreDelete(t, ss) })
	t.Run("GetByName", func(t *testing.T) { testChannelStoreGetByName(t, ss) })
	t.Run("GetByNames", func(t *testing.T) { testChannelStoreGetByNames(t, ss) })
	t.Run("GetDeletedByName", func(t *testing.T) { testChannelStoreGetDeletedByName(t, ss) })
	t.Run("GetDeleted", func(t *testing.T) { testChannelStoreGetDeleted(t, ss) })
	t.Run("ChannelMemberStore", func(t *testing.T) { testChannelMemberStore(t, ss) })
	t.Run("SaveMember", func(t *testing.T) { testChannelSaveMember(t, ss) })
	t.Run("SaveMultipleMembers", func(t *testing.T) { testChannelSaveMultipleMembers(t, ss) })
	t.Run("UpdateMember", func(t *testing.T) { testChannelUpdateMember(t, ss) })
	t.Run("UpdateMultipleMembers", func(t *testing.T) { testChannelUpdateMultipleMembers(t, ss) })
	t.Run("RemoveMember", func(t *testing.T) { testChannelRemoveMember(t, ss) })
	t.Run("RemoveMembers", func(t *testing.T) { testChannelRemoveMembers(t, ss) })
	t.Run("ChannelDeleteMemberStore", func(t *testing.T) { testChannelDeleteMemberStore(t, ss) })
	t.Run("GetChannels", func(t *testing.T) { testChannelStoreGetChannels(t, ss) })
	t.Run("GetAllChannels", func(t *testing.T) { testChannelStoreGetAllChannels(t, ss, s) })
	t.Run("GetMoreChannels", func(t *testing.T) { testChannelStoreGetMoreChannels(t, ss) })
	t.Run("GetPrivateChannelsForTeam", func(t *testing.T) { testChannelStoreGetPrivateChannelsForTeam(t, ss) })
	t.Run("GetPublicChannelsForTeam", func(t *testing.T) { testChannelStoreGetPublicChannelsForTeam(t, ss) })
	t.Run("GetPublicChannelsByIdsForTeam", func(t *testing.T) { testChannelStoreGetPublicChannelsByIdsForTeam(t, ss) })
	t.Run("GetChannelCounts", func(t *testing.T) { testChannelStoreGetChannelCounts(t, ss) })
	t.Run("GetMembersForUser", func(t *testing.T) { testChannelStoreGetMembersForUser(t, ss) })
	t.Run("GetMembersForUserWithPagination", func(t *testing.T) { testChannelStoreGetMembersForUserWithPagination(t, ss) })
	t.Run("CountPostsAfter", func(t *testing.T) { testCountPostsAfter(t, ss) })
	t.Run("UpdateLastViewedAt", func(t *testing.T) { testChannelStoreUpdateLastViewedAt(t, ss) })
	t.Run("IncrementMentionCount", func(t *testing.T) { testChannelStoreIncrementMentionCount(t, ss) })
	t.Run("UpdateChannelMember", func(t *testing.T) { testUpdateChannelMember(t, ss) })
	t.Run("GetMember", func(t *testing.T) { testGetMember(t, ss) })
	t.Run("GetMemberForPost", func(t *testing.T) { testChannelStoreGetMemberForPost(t, ss) })
	t.Run("GetMemberCount", func(t *testing.T) { testGetMemberCount(t, ss) })
	t.Run("GetMemberCountsByGroup", func(t *testing.T) { testGetMemberCountsByGroup(t, ss) })
	t.Run("GetGuestCount", func(t *testing.T) { testGetGuestCount(t, ss) })
	t.Run("SearchMore", func(t *testing.T) { testChannelStoreSearchMore(t, ss) })
	t.Run("SearchInTeam", func(t *testing.T) { testChannelStoreSearchInTeam(t, ss) })
	t.Run("SearchArchivedInTeam", func(t *testing.T) { testChannelStoreSearchArchivedInTeam(t, ss, s) })
	t.Run("SearchForUserInTeam", func(t *testing.T) { testChannelStoreSearchForUserInTeam(t, ss) })
	t.Run("SearchAllChannels", func(t *testing.T) { testChannelStoreSearchAllChannels(t, ss) })
	t.Run("GetMembersByIds", func(t *testing.T) { testChannelStoreGetMembersByIds(t, ss) })
	t.Run("GetMembersByChannelIds", func(t *testing.T) { testChannelStoreGetMembersByChannelIds(t, ss) })
	t.Run("SearchGroupChannels", func(t *testing.T) { testChannelStoreSearchGroupChannels(t, ss) })
	t.Run("AnalyticsDeletedTypeCount", func(t *testing.T) { testChannelStoreAnalyticsDeletedTypeCount(t, ss) })
	t.Run("GetPinnedPosts", func(t *testing.T) { testChannelStoreGetPinnedPosts(t, ss) })
	t.Run("GetPinnedPostCount", func(t *testing.T) { testChannelStoreGetPinnedPostCount(t, ss) })
	t.Run("MaxChannelsPerTeam", func(t *testing.T) { testChannelStoreMaxChannelsPerTeam(t, ss) })
	t.Run("GetChannelsByScheme", func(t *testing.T) { testChannelStoreGetChannelsByScheme(t, ss) })
	t.Run("MigrateChannelMembers", func(t *testing.T) { testChannelStoreMigrateChannelMembers(t, ss) })
	t.Run("ResetAllChannelSchemes", func(t *testing.T) { testResetAllChannelSchemes(t, ss) })
	t.Run("ClearAllCustomRoleAssignments", func(t *testing.T) { testChannelStoreClearAllCustomRoleAssignments(t, ss) })
	t.Run("MaterializedPublicChannels", func(t *testing.T) { testMaterializedPublicChannels(t, ss, s) })
	t.Run("GetAllChannelsForExportAfter", func(t *testing.T) { testChannelStoreGetAllChannelsForExportAfter(t, ss) })
	t.Run("GetChannelMembersForExport", func(t *testing.T) { testChannelStoreGetChannelMembersForExport(t, ss) })
	t.Run("RemoveAllDeactivatedMembers", func(t *testing.T) { testChannelStoreRemoveAllDeactivatedMembers(t, ss, s) })
	t.Run("ExportAllDirectChannels", func(t *testing.T) { testChannelStoreExportAllDirectChannels(t, ss, s) })
	t.Run("ExportAllDirectChannelsExcludePrivateAndPublic", func(t *testing.T) { testChannelStoreExportAllDirectChannelsExcludePrivateAndPublic(t, ss, s) })
	t.Run("ExportAllDirectChannelsDeletedChannel", func(t *testing.T) { testChannelStoreExportAllDirectChannelsDeletedChannel(t, ss, s) })
	t.Run("GetChannelsBatchForIndexing", func(t *testing.T) { testChannelStoreGetChannelsBatchForIndexing(t, ss) })
	t.Run("GroupSyncedChannelCount", func(t *testing.T) { testGroupSyncedChannelCount(t, ss) })
	t.Run("CreateInitialSidebarCategories", func(t *testing.T) { testCreateInitialSidebarCategories(t, ss) })
	t.Run("CreateSidebarCategory", func(t *testing.T) { testCreateSidebarCategory(t, ss) })
	t.Run("GetSidebarCategory", func(t *testing.T) { testGetSidebarCategory(t, ss, s) })
	t.Run("GetSidebarCategories", func(t *testing.T) { testGetSidebarCategories(t, ss) })
	t.Run("UpdateSidebarCategories", func(t *testing.T) { testUpdateSidebarCategories(t, ss) })
	t.Run("DeleteSidebarCategory", func(t *testing.T) { testDeleteSidebarCategory(t, ss, s) })
	t.Run("UpdateSidebarChannelsByPreferences", func(t *testing.T) { testUpdateSidebarChannelsByPreferences(t, ss) })
	t.Run("SetShared", func(t *testing.T) { testSetShared(t, ss) })
	t.Run("GetTeamForChannel", func(t *testing.T) { testGetTeamForChannel(t, ss) })
}

func testChannelStoreSave(t *testing.T, ss store.Store) {
	teamId := model.NewId()

	o1 := model.Channel{}
	o1.TeamId = teamId
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN

	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr, "couldn't save item", nErr)

	_, nErr = ss.Channel().Save(&o1, -1)
	require.Error(t, nErr, "shouldn't be able to update from save")

	o1.Id = ""
	_, nErr = ss.Channel().Save(&o1, -1)
	require.Error(t, nErr, "should be unique name")

	o1.Id = ""
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_DIRECT
	_, nErr = ss.Channel().Save(&o1, -1)
	require.Error(t, nErr, "should not be able to save direct channel")

	o1 = model.Channel{}
	o1.TeamId = teamId
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN

	_, nErr = ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr, "should have saved channel")

	o2 := o1
	o2.Id = ""

	_, nErr = ss.Channel().Save(&o2, -1)
	require.Error(t, nErr, "should have failed to save a duplicate channel")
	var cErr *store.ErrConflict
	require.True(t, errors.As(nErr, &cErr))

	err := ss.Channel().Delete(o1.Id, 100)
	require.NoError(t, err, "should have deleted channel")

	o2.Id = ""
	_, nErr = ss.Channel().Save(&o2, -1)
	require.Error(t, nErr, "should have failed to save a duplicate of an archived channel")
	require.True(t, errors.As(nErr, &cErr))
}

func testChannelStoreSaveDirectChannel(t *testing.T, ss store.Store, s SqlStore) {
	teamId := model.NewId()

	o1 := model.Channel{}
	o1.TeamId = teamId
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_DIRECT

	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = u1.Id
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelId = o1.Id
	m2.UserId = u2.Id
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	_, nErr = ss.Channel().SaveDirectChannel(&o1, &m1, &m2)
	require.NoError(t, nErr, "couldn't save direct channel", nErr)

	members, nErr := ss.Channel().GetMembers(o1.Id, 0, 100)
	require.NoError(t, nErr)
	require.Len(t, *members, 2, "should have saved 2 members")

	_, nErr = ss.Channel().SaveDirectChannel(&o1, &m1, &m2)
	require.Error(t, nErr, "shoudn't be a able to update from save")

	// Attempt to save a direct channel that already exists
	o1a := model.Channel{
		TeamId:      o1.TeamId,
		DisplayName: o1.DisplayName,
		Name:        o1.Name,
		Type:        o1.Type,
	}

	returnedChannel, nErr := ss.Channel().SaveDirectChannel(&o1a, &m1, &m2)
	require.Error(t, nErr, "should've failed to save a duplicate direct channel")
	var cErr *store.ErrConflict
	require.Truef(t, errors.As(nErr, &cErr), "should've returned ChannelExistsError")
	require.Equal(t, o1.Id, returnedChannel.Id, "should've failed to save a duplicate direct channel")

	// Attempt to save a non-direct channel
	o1.Id = ""
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().SaveDirectChannel(&o1, &m1, &m2)
	require.Error(t, nErr, "Should not be able to save non-direct channel")

	// Save yourself Direct Message
	o1.Id = ""
	o1.DisplayName = "Myself"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_DIRECT
	_, nErr = ss.Channel().SaveDirectChannel(&o1, &m1, &m1)
	require.NoError(t, nErr, "couldn't save direct channel", nErr)

	members, nErr = ss.Channel().GetMembers(o1.Id, 0, 100)
	require.NoError(t, nErr)
	require.Len(t, *members, 1, "should have saved just 1 member")

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testChannelStoreCreateDirectChannel(t *testing.T, ss store.Store) {
	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	c1, nErr := ss.Channel().CreateDirectChannel(u1, u2)
	require.NoError(t, nErr, "couldn't create direct channel", nErr)
	defer func() {
		ss.Channel().PermanentDeleteMembersByChannel(c1.Id)
		ss.Channel().PermanentDelete(c1.Id)
	}()

	members, nErr := ss.Channel().GetMembers(c1.Id, 0, 100)
	require.NoError(t, nErr)
	require.Len(t, *members, 2, "should have saved 2 members")
}

func testChannelStoreUpdate(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN

	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{}
	o2.TeamId = o1.TeamId
	o2.DisplayName = "Name"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN

	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	time.Sleep(100 * time.Millisecond)

	_, err := ss.Channel().Update(&o1)
	require.NoError(t, err, err)

	o1.DeleteAt = 100
	_, err = ss.Channel().Update(&o1)
	require.Error(t, err, "update should have failed because channel is archived")

	o1.DeleteAt = 0
	o1.Id = "missing"
	_, err = ss.Channel().Update(&o1)
	require.Error(t, err, "Update should have failed because of missing key")

	o2.Name = o1.Name
	_, err = ss.Channel().Update(&o2)
	require.Error(t, err, "update should have failed because of existing name")
}

func testGetChannelUnread(t *testing.T, ss store.Store) {
	teamId1 := model.NewId()
	teamId2 := model.NewId()

	uid := model.NewId()
	m1 := &model.TeamMember{TeamId: teamId1, UserId: uid}
	m2 := &model.TeamMember{TeamId: teamId2, UserId: uid}
	_, nErr := ss.Team().SaveMember(m1, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(m2, -1)
	require.NoError(t, nErr)
	notifyPropsModel := model.GetDefaultChannelNotifyProps()

	// Setup Channel 1
	c1 := &model.Channel{TeamId: m1.TeamId, Name: model.NewId(), DisplayName: "Downtown", Type: model.CHANNEL_OPEN, TotalMsgCount: 100, TotalMsgCountRoot: 99}
	_, nErr = ss.Channel().Save(c1, -1)
	require.NoError(t, nErr)

	cm1 := &model.ChannelMember{ChannelId: c1.Id, UserId: m1.UserId, NotifyProps: notifyPropsModel, MsgCount: 90, MsgCountRoot: 80}
	_, err := ss.Channel().SaveMember(cm1)
	require.NoError(t, err)

	// Setup Channel 2
	c2 := &model.Channel{TeamId: m2.TeamId, Name: model.NewId(), DisplayName: "Cultural", Type: model.CHANNEL_OPEN, TotalMsgCount: 100, TotalMsgCountRoot: 100}
	_, nErr = ss.Channel().Save(c2, -1)
	require.NoError(t, nErr)

	cm2 := &model.ChannelMember{ChannelId: c2.Id, UserId: m2.UserId, NotifyProps: notifyPropsModel, MsgCount: 90, MsgCountRoot: 90, MentionCount: 5, MentionCountRoot: 1}
	_, err = ss.Channel().SaveMember(cm2)
	require.NoError(t, err)

	// Check for Channel 1
	ch, nErr := ss.Channel().GetChannelUnread(c1.Id, uid)

	require.NoError(t, nErr, nErr)
	require.Equal(t, c1.Id, ch.ChannelId, "Wrong channel id")
	require.Equal(t, teamId1, ch.TeamId, "Wrong team id for channel 1")
	require.NotNil(t, ch.NotifyProps, "wrong props for channel 1")
	require.EqualValues(t, 0, ch.MentionCount, "wrong MentionCount for channel 1")
	require.EqualValues(t, 10, ch.MsgCount, "wrong MsgCount for channel 1")
	require.EqualValues(t, 19, ch.MsgCountRoot, "wrong MsgCountRoot for channel 1")
	// Check for Channel 2
	ch2, nErr := ss.Channel().GetChannelUnread(c2.Id, uid)

	require.NoError(t, nErr, nErr)
	require.Equal(t, c2.Id, ch2.ChannelId, "Wrong channel id")
	require.Equal(t, teamId2, ch2.TeamId, "Wrong team id")
	require.EqualValues(t, 5, ch2.MentionCount, "wrong MentionCount for channel 2")
	require.EqualValues(t, 1, ch2.MentionCountRoot, "wrong MentionCountRoot for channel 2")
	require.EqualValues(t, 10, ch2.MsgCount, "wrong MsgCount for channel 2")
}

func testChannelStoreGet(t *testing.T, ss store.Store, s SqlStore) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	c1, err := ss.Channel().Get(o1.Id, false)
	require.NoError(t, err, err)
	require.Equal(t, o1.ToJson(), c1.ToJson(), "invalid returned channel")

	_, err = ss.Channel().Get("", false)
	require.Error(t, err, "missing id should have failed")

	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err = ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(&u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{}
	o2.TeamId = model.NewId()
	o2.DisplayName = "Direct Name"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_DIRECT

	m1 := model.ChannelMember{}
	m1.ChannelId = o2.Id
	m1.UserId = u1.Id
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelId = o2.Id
	m2.UserId = u2.Id
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	_, nErr = ss.Channel().SaveDirectChannel(&o2, &m1, &m2)
	require.NoError(t, nErr)

	c2, err := ss.Channel().Get(o2.Id, false)
	require.NoError(t, err, err)
	require.Equal(t, o2.ToJson(), c2.ToJson(), "invalid returned channel")

	c4, err := ss.Channel().Get(o2.Id, true)
	require.NoError(t, err, err)
	require.Equal(t, o2.ToJson(), c4.ToJson(), "invalid returned channel")

	channels, chanErr := ss.Channel().GetAll(o1.TeamId)
	require.NoError(t, chanErr, chanErr)
	require.Greater(t, len(channels), 0, "too little")

	channelsTeam, err := ss.Channel().GetTeamChannels(o1.TeamId)
	require.NoError(t, err, err)
	require.Greater(t, len(*channelsTeam), 0, "too little")

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testChannelStoreGetChannelsByIds(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Name"
	o1.Name = "aa" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(&u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{}
	o2.TeamId = model.NewId()
	o2.DisplayName = "Direct Name"
	o2.Name = "bb" + model.NewId() + "b"
	o2.Type = model.CHANNEL_DIRECT

	o3 := model.Channel{}
	o3.TeamId = model.NewId()
	o3.DisplayName = "Deleted channel"
	o3.Name = "cc" + model.NewId() + "b"
	o3.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)
	nErr = ss.Channel().Delete(o3.Id, 123)
	require.NoError(t, nErr)
	o3.DeleteAt = 123
	o3.UpdateAt = 123

	m1 := model.ChannelMember{}
	m1.ChannelId = o2.Id
	m1.UserId = u1.Id
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelId = o2.Id
	m2.UserId = u2.Id
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	_, nErr = ss.Channel().SaveDirectChannel(&o2, &m1, &m2)
	require.NoError(t, nErr)

	t.Run("Get 2 existing channels", func(t *testing.T) {
		r1, err := ss.Channel().GetChannelsByIds([]string{o1.Id, o2.Id}, false)
		require.NoError(t, err, err)
		require.Len(t, r1, 2, "invalid returned channels, exepected 2 and got "+strconv.Itoa(len(r1)))
		require.Equal(t, o1.ToJson(), r1[0].ToJson())
		require.Equal(t, o2.ToJson(), r1[1].ToJson())
	})

	t.Run("Get 1 existing and 1 not existing channel", func(t *testing.T) {
		nonexistentId := "abcd1234"
		r2, err := ss.Channel().GetChannelsByIds([]string{o1.Id, nonexistentId}, false)
		require.NoError(t, err, err)
		require.Len(t, r2, 1, "invalid returned channels, expected 1 and got "+strconv.Itoa(len(r2)))
		require.Equal(t, o1.ToJson(), r2[0].ToJson(), "invalid returned channel")
	})

	t.Run("Get 2 existing and 1 deleted channel", func(t *testing.T) {
		r1, err := ss.Channel().GetChannelsByIds([]string{o1.Id, o2.Id, o3.Id}, true)
		require.NoError(t, err, err)
		require.Len(t, r1, 3, "invalid returned channels, exepected 3 and got "+strconv.Itoa(len(r1)))
		require.Equal(t, o1.ToJson(), r1[0].ToJson())
		require.Equal(t, o2.ToJson(), r1[1].ToJson())
		require.Equal(t, o3.ToJson(), r1[2].ToJson())
	})
}

func testChannelStoreGetForPost(t *testing.T, ss store.Store) {

	ch := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	o1, nErr := ss.Channel().Save(ch, -1)
	require.NoError(t, nErr)

	p1, err := ss.Post().Save(&model.Post{
		UserId:    model.NewId(),
		ChannelId: o1.Id,
		Message:   "test",
	})
	require.NoError(t, err)

	channel, chanErr := ss.Channel().GetForPost(p1.Id)
	require.NoError(t, chanErr, chanErr)
	require.Equal(t, o1.Id, channel.Id, "incorrect channel returned")
}

func testChannelStoreRestore(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	err := ss.Channel().Delete(o1.Id, model.GetMillis())
	require.NoError(t, err, err)

	c, _ := ss.Channel().Get(o1.Id, false)
	require.NotEqual(t, 0, c.DeleteAt, "should have been deleted")

	err = ss.Channel().Restore(o1.Id, model.GetMillis())
	require.NoError(t, err, err)

	c, _ = ss.Channel().Get(o1.Id, false)
	require.EqualValues(t, 0, c.DeleteAt, "should have been restored")
}

func testChannelStoreDelete(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{}
	o2.TeamId = o1.TeamId
	o2.DisplayName = "Channel2"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	o3 := model.Channel{}
	o3.TeamId = o1.TeamId
	o3.DisplayName = "Channel3"
	o3.Name = "zz" + model.NewId() + "b"
	o3.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	o4 := model.Channel{}
	o4.TeamId = o1.TeamId
	o4.DisplayName = "Channel4"
	o4.Name = "zz" + model.NewId() + "b"
	o4.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o4, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = model.NewId()
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err := ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{}
	m2.ChannelId = o2.Id
	m2.UserId = m1.UserId
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	nErr = ss.Channel().Delete(o1.Id, model.GetMillis())
	require.NoError(t, nErr, nErr)

	c, _ := ss.Channel().Get(o1.Id, false)
	require.NotEqual(t, 0, c.DeleteAt, "should have been deleted")

	nErr = ss.Channel().Delete(o3.Id, model.GetMillis())
	require.NoError(t, nErr, nErr)

	list, nErr := ss.Channel().GetChannels(o1.TeamId, m1.UserId, false, 0)
	require.NoError(t, nErr)
	require.Len(t, *list, 1, "invalid number of channels")

	list, nErr = ss.Channel().GetMoreChannels(o1.TeamId, m1.UserId, 0, 100)
	require.NoError(t, nErr)
	require.Len(t, *list, 1, "invalid number of channels")

	cresult := ss.Channel().PermanentDelete(o2.Id)
	require.NoError(t, cresult)

	list, nErr = ss.Channel().GetChannels(o1.TeamId, m1.UserId, false, 0)
	if assert.Error(t, nErr) {
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(nErr, &nfErr))
	} else {
		require.Equal(t, &model.ChannelList{}, list)
	}

	nErr = ss.Channel().PermanentDeleteByTeam(o1.TeamId)
	require.NoError(t, nErr, nErr)
}

func testChannelStoreGetByName(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	result, err := ss.Channel().GetByName(o1.TeamId, o1.Name, true)
	require.NoError(t, err)
	require.Equal(t, o1.ToJson(), result.ToJson(), "invalid returned channel")

	channelID := result.Id

	result, err = ss.Channel().GetByName(o1.TeamId, "", true)
	require.Error(t, err, "Missing id should have failed")

	result, err = ss.Channel().GetByName(o1.TeamId, o1.Name, false)
	require.NoError(t, err)
	require.Equal(t, o1.ToJson(), result.ToJson(), "invalid returned channel")

	result, err = ss.Channel().GetByName(o1.TeamId, "", false)
	require.Error(t, err, "Missing id should have failed")

	nErr = ss.Channel().Delete(channelID, model.GetMillis())
	require.NoError(t, nErr, "channel should have been deleted")

	result, err = ss.Channel().GetByName(o1.TeamId, o1.Name, false)
	require.Error(t, err, "Deleted channel should not be returned by GetByName()")
}

func testChannelStoreGetByNames(t *testing.T, ss store.Store) {
	o1 := model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{
		TeamId:      o1.TeamId,
		DisplayName: "Name",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	for index, tc := range []struct {
		TeamId      string
		Names       []string
		ExpectedIds []string
	}{
		{o1.TeamId, []string{o1.Name}, []string{o1.Id}},
		{o1.TeamId, []string{o1.Name, o2.Name}, []string{o1.Id, o2.Id}},
		{o1.TeamId, nil, nil},
		{o1.TeamId, []string{"foo"}, nil},
		{o1.TeamId, []string{o1.Name, "foo", o2.Name, o2.Name}, []string{o1.Id, o2.Id}},
		{"", []string{o1.Name, "foo", o2.Name, o2.Name}, []string{o1.Id, o2.Id}},
		{"asd", []string{o1.Name, "foo", o2.Name, o2.Name}, nil},
	} {
		var channels []*model.Channel
		channels, err := ss.Channel().GetByNames(tc.TeamId, tc.Names, true)
		require.NoError(t, err)
		var ids []string
		for _, channel := range channels {
			ids = append(ids, channel.Id)
		}
		sort.Strings(ids)
		sort.Strings(tc.ExpectedIds)
		assert.Equal(t, tc.ExpectedIds, ids, "tc %v", index)
	}

	err := ss.Channel().Delete(o1.Id, model.GetMillis())
	require.NoError(t, err, "channel should have been deleted")

	err = ss.Channel().Delete(o2.Id, model.GetMillis())
	require.NoError(t, err, "channel should have been deleted")

	channels, nErr := ss.Channel().GetByNames(o1.TeamId, []string{o1.Name}, false)
	require.NoError(t, nErr)
	assert.Empty(t, channels)
}

func testChannelStoreGetDeletedByName(t *testing.T, ss store.Store) {
	o1 := &model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(o1, -1)
	require.NoError(t, nErr)

	now := model.GetMillis()
	err := ss.Channel().Delete(o1.Id, now)
	require.NoError(t, err, "channel should have been deleted")
	o1.DeleteAt = now
	o1.UpdateAt = now

	r1, nErr := ss.Channel().GetDeletedByName(o1.TeamId, o1.Name)
	require.NoError(t, nErr)
	require.Equal(t, o1, r1)

	_, nErr = ss.Channel().GetDeletedByName(o1.TeamId, "")
	require.Error(t, nErr, "missing id should have failed")
}

func testChannelStoreGetDeleted(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN

	userId := model.NewId()

	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	err := ss.Channel().Delete(o1.Id, model.GetMillis())
	require.NoError(t, err, "channel should have been deleted")

	list, nErr := ss.Channel().GetDeleted(o1.TeamId, 0, 100, userId)
	require.NoError(t, nErr, nErr)
	require.Len(t, *list, 1, "wrong list")
	require.Equal(t, o1.Name, (*list)[0].Name, "missing channel")

	o2 := model.Channel{}
	o2.TeamId = o1.TeamId
	o2.DisplayName = "Channel2"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	list, nErr = ss.Channel().GetDeleted(o1.TeamId, 0, 100, userId)
	require.NoError(t, nErr, nErr)
	require.Len(t, *list, 1, "wrong list")

	o3 := model.Channel{}
	o3.TeamId = o1.TeamId
	o3.DisplayName = "Channel3"
	o3.Name = "zz" + model.NewId() + "b"
	o3.Type = model.CHANNEL_OPEN

	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	err = ss.Channel().Delete(o3.Id, model.GetMillis())
	require.NoError(t, err, "channel should have been deleted")

	list, nErr = ss.Channel().GetDeleted(o1.TeamId, 0, 100, userId)
	require.NoError(t, nErr, nErr)
	require.Len(t, *list, 2, "wrong list length")

	list, nErr = ss.Channel().GetDeleted(o1.TeamId, 0, 1, userId)
	require.NoError(t, nErr, nErr)
	require.Len(t, *list, 1, "wrong list length")

	list, nErr = ss.Channel().GetDeleted(o1.TeamId, 1, 1, userId)
	require.NoError(t, nErr, nErr)
	require.Len(t, *list, 1, "wrong list length")

}

func testChannelMemberStore(t *testing.T, ss store.Store) {
	c1 := &model.Channel{}
	c1.TeamId = model.NewId()
	c1.DisplayName = "NameName"
	c1.Name = "zz" + model.NewId() + "b"
	c1.Type = model.CHANNEL_OPEN
	c1, nErr := ss.Channel().Save(c1, -1)
	require.NoError(t, nErr)

	c1t1, _ := ss.Channel().Get(c1.Id, false)
	assert.EqualValues(t, 0, c1t1.ExtraUpdateAt, "ExtraUpdateAt should be 0")

	u1 := model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(&u1)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(&u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	o1 := model.ChannelMember{}
	o1.ChannelId = c1.Id
	o1.UserId = u1.Id
	o1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, nErr = ss.Channel().SaveMember(&o1)
	require.NoError(t, nErr)

	o2 := model.ChannelMember{}
	o2.ChannelId = c1.Id
	o2.UserId = u2.Id
	o2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, nErr = ss.Channel().SaveMember(&o2)
	require.NoError(t, nErr)

	c1t2, _ := ss.Channel().Get(c1.Id, false)
	assert.EqualValues(t, 0, c1t2.ExtraUpdateAt, "ExtraUpdateAt should be 0")

	count, nErr := ss.Channel().GetMemberCount(o1.ChannelId, true)
	require.NoError(t, nErr)
	require.EqualValues(t, 2, count, "should have saved 2 members")

	count, nErr = ss.Channel().GetMemberCount(o1.ChannelId, true)
	require.NoError(t, nErr)
	require.EqualValues(t, 2, count, "should have saved 2 members")
	require.EqualValues(
		t,
		2,
		ss.Channel().GetMemberCountFromCache(o1.ChannelId),
		"should have saved 2 members")

	require.EqualValues(
		t,
		0,
		ss.Channel().GetMemberCountFromCache("junk"),
		"should have saved 0 members")

	count, nErr = ss.Channel().GetMemberCount(o1.ChannelId, false)
	require.NoError(t, nErr)
	require.EqualValues(t, 2, count, "should have saved 2 members")

	nErr = ss.Channel().RemoveMember(o2.ChannelId, o2.UserId)
	require.NoError(t, nErr)

	count, nErr = ss.Channel().GetMemberCount(o1.ChannelId, false)
	require.NoError(t, nErr)
	require.EqualValues(t, 1, count, "should have removed 1 member")

	c1t3, _ := ss.Channel().Get(c1.Id, false)
	assert.EqualValues(t, 0, c1t3.ExtraUpdateAt, "ExtraUpdateAt should be 0")

	member, _ := ss.Channel().GetMember(context.Background(), o1.ChannelId, o1.UserId)
	require.Equal(t, o1.ChannelId, member.ChannelId, "should have go member")

	_, nErr = ss.Channel().SaveMember(&o1)
	require.Error(t, nErr, "should have been a duplicate")

	c1t4, _ := ss.Channel().Get(c1.Id, false)
	assert.EqualValues(t, 0, c1t4.ExtraUpdateAt, "ExtraUpdateAt should be 0")
}

func testChannelSaveMember(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	defaultNotifyProps := model.GetDefaultChannelNotifyProps()

	t.Run("not valid channel member", func(t *testing.T) {
		member := &model.ChannelMember{ChannelId: "wrong", UserId: u1.Id, NotifyProps: defaultNotifyProps}
		_, nErr := ss.Channel().SaveMember(member)
		require.Error(t, nErr)
		var appErr *model.AppError
		require.True(t, errors.As(nErr, &appErr))
		require.Equal(t, "model.channel_member.is_valid.channel_id.app_error", appErr.Id)
	})

	t.Run("duplicated entries should fail", func(t *testing.T) {
		channelID1 := model.NewId()
		m1 := &model.ChannelMember{ChannelId: channelID1, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		_, nErr := ss.Channel().SaveMember(m1)
		require.NoError(t, nErr)
		m2 := &model.ChannelMember{ChannelId: channelID1, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		_, nErr = ss.Channel().SaveMember(m2)
		require.Error(t, nErr)
		require.IsType(t, &store.ErrConflict{}, nErr)
	})

	t.Run("insert member correctly (in channel without channel scheme and team without scheme)", func(t *testing.T) {
		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
		}

		team, nErr := ss.Team().Save(team)
		require.NoError(t, nErr)

		channel := &model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, nErr = ss.Channel().Save(channel, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      "channel_user",
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      "channel_user",
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       "channel_guest",
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       "channel_guest",
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       "channel_user channel_admin",
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       "channel_user channel_admin",
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_user",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test channel_user",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_guest",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test channel_guest",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_user channel_admin",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test channel_user channel_admin",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u1.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				member, nErr = ss.Channel().SaveMember(member)
				require.NoError(t, nErr)
				defer ss.Channel().RemoveMember(channel.Id, u1.Id)
				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})

	t.Run("insert member correctly (in channel without scheme and team with scheme)", func(t *testing.T) {
		ts := &model.Scheme{
			Name:        model.NewId(),
			DisplayName: model.NewId(),
			Description: model.NewId(),
			Scope:       model.SCHEME_SCOPE_TEAM,
		}
		ts, nErr := ss.Scheme().Save(ts)
		require.NoError(t, nErr)

		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
			SchemeId:    &ts.Id,
		}

		team, nErr = ss.Team().Save(team)
		require.NoError(t, nErr)

		channel := &model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, nErr = ss.Channel().Save(channel, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      ts.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      ts.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       ts.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       ts.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test " + ts.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u1.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				member, nErr = ss.Channel().SaveMember(member)
				require.NoError(t, nErr)
				defer ss.Channel().RemoveMember(channel.Id, u1.Id)
				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})

	t.Run("insert member correctly (in channel with channel scheme)", func(t *testing.T) {
		cs := &model.Scheme{
			Name:        model.NewId(),
			DisplayName: model.NewId(),
			Description: model.NewId(),
			Scope:       model.SCHEME_SCOPE_CHANNEL,
		}
		cs, nErr := ss.Scheme().Save(cs)
		require.NoError(t, nErr)

		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
		}

		team, nErr = ss.Team().Save(team)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
			SchemeId:    &cs.Id,
		}, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      cs.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      cs.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       cs.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       cs.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test " + cs.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u1.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				member, nErr = ss.Channel().SaveMember(member)
				require.NoError(t, nErr)
				defer ss.Channel().RemoveMember(channel.Id, u1.Id)
				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})
}

func testChannelSaveMultipleMembers(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	u2, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	defaultNotifyProps := model.GetDefaultChannelNotifyProps()

	t.Run("any not valid channel member", func(t *testing.T) {
		m1 := &model.ChannelMember{ChannelId: "wrong", UserId: u1.Id, NotifyProps: defaultNotifyProps}
		m2 := &model.ChannelMember{ChannelId: model.NewId(), UserId: u2.Id, NotifyProps: defaultNotifyProps}
		_, nErr := ss.Channel().SaveMultipleMembers([]*model.ChannelMember{m1, m2})
		require.Error(t, nErr)
		var appErr *model.AppError
		require.True(t, errors.As(nErr, &appErr))
		require.Equal(t, "model.channel_member.is_valid.channel_id.app_error", appErr.Id)
	})

	t.Run("duplicated entries should fail", func(t *testing.T) {
		channelID1 := model.NewId()
		m1 := &model.ChannelMember{ChannelId: channelID1, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		m2 := &model.ChannelMember{ChannelId: channelID1, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		_, nErr := ss.Channel().SaveMultipleMembers([]*model.ChannelMember{m1, m2})
		require.Error(t, nErr)
		require.IsType(t, &store.ErrConflict{}, nErr)
	})

	t.Run("insert members correctly (in channel without channel scheme and team without scheme)", func(t *testing.T) {
		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
		}

		team, nErr := ss.Team().Save(team)
		require.NoError(t, nErr)

		channel := &model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, nErr = ss.Channel().Save(channel, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      "channel_user",
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      "channel_user",
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       "channel_guest",
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       "channel_guest",
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       "channel_user channel_admin",
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       "channel_user channel_admin",
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_user",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test channel_user",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_guest",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test channel_guest",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_user channel_admin",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test channel_user channel_admin",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u1.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				otherMember := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u2.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				var members []*model.ChannelMember
				members, nErr = ss.Channel().SaveMultipleMembers([]*model.ChannelMember{member, otherMember})
				require.NoError(t, nErr)
				require.Len(t, members, 2)
				member = members[0]
				defer ss.Channel().RemoveMember(channel.Id, u1.Id)
				defer ss.Channel().RemoveMember(channel.Id, u2.Id)

				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})

	t.Run("insert members correctly (in channel without scheme and team with scheme)", func(t *testing.T) {
		ts := &model.Scheme{
			Name:        model.NewId(),
			DisplayName: model.NewId(),
			Description: model.NewId(),
			Scope:       model.SCHEME_SCOPE_TEAM,
		}
		ts, nErr := ss.Scheme().Save(ts)
		require.NoError(t, nErr)

		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
			SchemeId:    &ts.Id,
		}

		team, nErr = ss.Team().Save(team)
		require.NoError(t, nErr)

		channel := &model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, nErr = ss.Channel().Save(channel, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      ts.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      ts.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       ts.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       ts.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test " + ts.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u1.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				otherMember := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u2.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				var members []*model.ChannelMember
				members, nErr = ss.Channel().SaveMultipleMembers([]*model.ChannelMember{member, otherMember})
				require.NoError(t, nErr)
				require.Len(t, members, 2)
				member = members[0]
				defer ss.Channel().RemoveMember(channel.Id, u1.Id)
				defer ss.Channel().RemoveMember(channel.Id, u2.Id)

				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})

	t.Run("insert members correctly (in channel with channel scheme)", func(t *testing.T) {
		cs := &model.Scheme{
			Name:        model.NewId(),
			DisplayName: model.NewId(),
			Description: model.NewId(),
			Scope:       model.SCHEME_SCOPE_CHANNEL,
		}
		cs, nErr := ss.Scheme().Save(cs)
		require.NoError(t, nErr)

		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
		}

		team, nErr = ss.Team().Save(team)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
			SchemeId:    &cs.Id,
		}, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      cs.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      cs.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       cs.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       cs.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test " + cs.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u1.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				otherMember := &model.ChannelMember{
					ChannelId:     channel.Id,
					UserId:        u2.Id,
					SchemeGuest:   tc.SchemeGuest,
					SchemeUser:    tc.SchemeUser,
					SchemeAdmin:   tc.SchemeAdmin,
					ExplicitRoles: tc.ExplicitRoles,
					NotifyProps:   defaultNotifyProps,
				}
				members, err := ss.Channel().SaveMultipleMembers([]*model.ChannelMember{member, otherMember})
				require.NoError(t, err)
				require.Len(t, members, 2)
				member = members[0]
				defer ss.Channel().RemoveMember(channel.Id, u1.Id)
				defer ss.Channel().RemoveMember(channel.Id, u2.Id)

				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})
}

func testChannelUpdateMember(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	defaultNotifyProps := model.GetDefaultChannelNotifyProps()

	t.Run("not valid channel member", func(t *testing.T) {
		member := &model.ChannelMember{ChannelId: "wrong", UserId: u1.Id, NotifyProps: defaultNotifyProps}
		_, nErr := ss.Channel().UpdateMember(member)
		require.Error(t, nErr)
		var appErr *model.AppError
		require.True(t, errors.As(nErr, &appErr))
		require.Equal(t, "model.channel_member.is_valid.channel_id.app_error", appErr.Id)
	})

	t.Run("insert member correctly (in channel without channel scheme and team without scheme)", func(t *testing.T) {
		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
		}

		team, nErr := ss.Team().Save(team)
		require.NoError(t, nErr)

		channel := &model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, nErr = ss.Channel().Save(channel, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		member := &model.ChannelMember{
			ChannelId:   channel.Id,
			UserId:      u1.Id,
			NotifyProps: defaultNotifyProps,
		}
		member, nErr = ss.Channel().SaveMember(member)
		require.NoError(t, nErr)

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      "channel_user",
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      "channel_user",
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       "channel_guest",
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       "channel_guest",
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       "channel_user channel_admin",
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       "channel_user channel_admin",
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_user",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test channel_user",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_guest",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test channel_guest",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_user channel_admin",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test channel_user channel_admin",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member.SchemeGuest = tc.SchemeGuest
				member.SchemeUser = tc.SchemeUser
				member.SchemeAdmin = tc.SchemeAdmin
				member.ExplicitRoles = tc.ExplicitRoles
				member, nErr = ss.Channel().UpdateMember(member)
				require.NoError(t, nErr)
				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})

	t.Run("insert member correctly (in channel without scheme and team with scheme)", func(t *testing.T) {
		ts := &model.Scheme{
			Name:        model.NewId(),
			DisplayName: model.NewId(),
			Description: model.NewId(),
			Scope:       model.SCHEME_SCOPE_TEAM,
		}
		ts, nErr := ss.Scheme().Save(ts)
		require.NoError(t, nErr)

		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
			SchemeId:    &ts.Id,
		}

		team, nErr = ss.Team().Save(team)
		require.NoError(t, nErr)

		channel := &model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, nErr = ss.Channel().Save(channel, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		member := &model.ChannelMember{
			ChannelId:   channel.Id,
			UserId:      u1.Id,
			NotifyProps: defaultNotifyProps,
		}
		member, nErr = ss.Channel().SaveMember(member)
		require.NoError(t, nErr)

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      ts.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      ts.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       ts.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       ts.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test " + ts.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member.SchemeGuest = tc.SchemeGuest
				member.SchemeUser = tc.SchemeUser
				member.SchemeAdmin = tc.SchemeAdmin
				member.ExplicitRoles = tc.ExplicitRoles
				member, nErr = ss.Channel().UpdateMember(member)
				require.NoError(t, nErr)
				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})

	t.Run("insert member correctly (in channel with channel scheme)", func(t *testing.T) {
		cs := &model.Scheme{
			Name:        model.NewId(),
			DisplayName: model.NewId(),
			Description: model.NewId(),
			Scope:       model.SCHEME_SCOPE_CHANNEL,
		}
		cs, nErr := ss.Scheme().Save(cs)
		require.NoError(t, nErr)

		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
		}

		team, nErr = ss.Team().Save(team)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
			SchemeId:    &cs.Id,
		}, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		member := &model.ChannelMember{
			ChannelId:   channel.Id,
			UserId:      u1.Id,
			NotifyProps: defaultNotifyProps,
		}
		member, nErr = ss.Channel().SaveMember(member)
		require.NoError(t, nErr)

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      cs.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      cs.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       cs.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       cs.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test " + cs.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member.SchemeGuest = tc.SchemeGuest
				member.SchemeUser = tc.SchemeUser
				member.SchemeAdmin = tc.SchemeAdmin
				member.ExplicitRoles = tc.ExplicitRoles
				member, nErr = ss.Channel().UpdateMember(member)
				require.NoError(t, nErr)
				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})
}

func testChannelUpdateMultipleMembers(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	u2, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	defaultNotifyProps := model.GetDefaultChannelNotifyProps()

	t.Run("any not valid channel member", func(t *testing.T) {
		m1 := &model.ChannelMember{ChannelId: "wrong", UserId: u1.Id, NotifyProps: defaultNotifyProps}
		m2 := &model.ChannelMember{ChannelId: model.NewId(), UserId: u2.Id, NotifyProps: defaultNotifyProps}
		_, nErr := ss.Channel().SaveMultipleMembers([]*model.ChannelMember{m1, m2})
		require.Error(t, nErr)
		var appErr *model.AppError
		require.True(t, errors.As(nErr, &appErr))
		require.Equal(t, "model.channel_member.is_valid.channel_id.app_error", appErr.Id)
	})

	t.Run("duplicated entries should fail", func(t *testing.T) {
		channelID1 := model.NewId()
		m1 := &model.ChannelMember{ChannelId: channelID1, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		m2 := &model.ChannelMember{ChannelId: channelID1, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		_, nErr := ss.Channel().SaveMultipleMembers([]*model.ChannelMember{m1, m2})
		require.Error(t, nErr)
		require.IsType(t, &store.ErrConflict{}, nErr)
	})

	t.Run("insert members correctly (in channel without channel scheme and team without scheme)", func(t *testing.T) {
		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
		}

		team, nErr := ss.Team().Save(team)
		require.NoError(t, nErr)

		channel := &model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, nErr = ss.Channel().Save(channel, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		member := &model.ChannelMember{ChannelId: channel.Id, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		otherMember := &model.ChannelMember{ChannelId: channel.Id, UserId: u2.Id, NotifyProps: defaultNotifyProps}
		var members []*model.ChannelMember
		members, nErr = ss.Channel().SaveMultipleMembers([]*model.ChannelMember{member, otherMember})
		require.NoError(t, nErr)
		defer ss.Channel().RemoveMember(channel.Id, u1.Id)
		defer ss.Channel().RemoveMember(channel.Id, u2.Id)
		require.Len(t, members, 2)
		member = members[0]
		otherMember = members[1]

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      "channel_user",
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      "channel_user",
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       "channel_guest",
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       "channel_guest",
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       "channel_user channel_admin",
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       "channel_user channel_admin",
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_user",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test channel_user",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_guest",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test channel_guest",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test channel_user channel_admin",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test channel_user channel_admin",
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member.SchemeGuest = tc.SchemeGuest
				member.SchemeUser = tc.SchemeUser
				member.SchemeAdmin = tc.SchemeAdmin
				member.ExplicitRoles = tc.ExplicitRoles
				var members []*model.ChannelMember
				members, nErr = ss.Channel().UpdateMultipleMembers([]*model.ChannelMember{member, otherMember})
				require.NoError(t, nErr)
				require.Len(t, members, 2)
				member = members[0]

				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})

	t.Run("insert members correctly (in channel without scheme and team with scheme)", func(t *testing.T) {
		ts := &model.Scheme{
			Name:        model.NewId(),
			DisplayName: model.NewId(),
			Description: model.NewId(),
			Scope:       model.SCHEME_SCOPE_TEAM,
		}
		ts, nErr := ss.Scheme().Save(ts)
		require.NoError(t, nErr)

		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
			SchemeId:    &ts.Id,
		}

		team, nErr = ss.Team().Save(team)
		require.NoError(t, nErr)

		channel := &model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		channel, nErr = ss.Channel().Save(channel, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		member := &model.ChannelMember{ChannelId: channel.Id, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		otherMember := &model.ChannelMember{ChannelId: channel.Id, UserId: u2.Id, NotifyProps: defaultNotifyProps}
		var members []*model.ChannelMember
		members, nErr = ss.Channel().SaveMultipleMembers([]*model.ChannelMember{member, otherMember})
		require.NoError(t, nErr)
		defer ss.Channel().RemoveMember(channel.Id, u1.Id)
		defer ss.Channel().RemoveMember(channel.Id, u2.Id)
		require.Len(t, members, 2)
		member = members[0]
		otherMember = members[1]

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      ts.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      ts.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       ts.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       ts.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test " + ts.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test " + ts.DefaultChannelUserRole + " " + ts.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member.SchemeGuest = tc.SchemeGuest
				member.SchemeUser = tc.SchemeUser
				member.SchemeAdmin = tc.SchemeAdmin
				member.ExplicitRoles = tc.ExplicitRoles
				var members []*model.ChannelMember
				members, nErr = ss.Channel().UpdateMultipleMembers([]*model.ChannelMember{member, otherMember})
				require.NoError(t, nErr)
				require.Len(t, members, 2)
				member = members[0]

				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})

	t.Run("insert members correctly (in channel with channel scheme)", func(t *testing.T) {
		cs := &model.Scheme{
			Name:        model.NewId(),
			DisplayName: model.NewId(),
			Description: model.NewId(),
			Scope:       model.SCHEME_SCOPE_CHANNEL,
		}
		cs, nErr := ss.Scheme().Save(cs)
		require.NoError(t, nErr)

		team := &model.Team{
			DisplayName: "Name",
			Name:        "zz" + model.NewId(),
			Email:       MakeEmail(),
			Type:        model.TEAM_OPEN,
		}

		team, nErr = ss.Team().Save(team)
		require.NoError(t, nErr)

		channel, nErr := ss.Channel().Save(&model.Channel{
			DisplayName: "DisplayName",
			Name:        "z-z-z" + model.NewId() + "b",
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
			SchemeId:    &cs.Id,
		}, -1)
		require.NoError(t, nErr)
		defer func() { ss.Channel().PermanentDelete(channel.Id) }()

		member := &model.ChannelMember{ChannelId: channel.Id, UserId: u1.Id, NotifyProps: defaultNotifyProps}
		otherMember := &model.ChannelMember{ChannelId: channel.Id, UserId: u2.Id, NotifyProps: defaultNotifyProps}
		members, err := ss.Channel().SaveMultipleMembers([]*model.ChannelMember{member, otherMember})
		require.NoError(t, err)
		defer ss.Channel().RemoveMember(channel.Id, u1.Id)
		defer ss.Channel().RemoveMember(channel.Id, u2.Id)
		require.Len(t, members, 2)
		member = members[0]
		otherMember = members[1]

		testCases := []struct {
			Name                  string
			SchemeGuest           bool
			SchemeUser            bool
			SchemeAdmin           bool
			ExplicitRoles         string
			ExpectedRoles         string
			ExpectedExplicitRoles string
			ExpectedSchemeGuest   bool
			ExpectedSchemeUser    bool
			ExpectedSchemeAdmin   bool
		}{
			{
				Name:               "channel user implicit",
				SchemeUser:         true,
				ExpectedRoles:      cs.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:               "channel user explicit",
				ExplicitRoles:      "channel_user",
				ExpectedRoles:      cs.DefaultChannelUserRole,
				ExpectedSchemeUser: true,
			},
			{
				Name:                "channel guest implicit",
				SchemeGuest:         true,
				ExpectedRoles:       cs.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel guest explicit",
				ExplicitRoles:       "channel_guest",
				ExpectedRoles:       cs.DefaultChannelGuestRole,
				ExpectedSchemeGuest: true,
			},
			{
				Name:                "channel admin implicit",
				SchemeUser:          true,
				SchemeAdmin:         true,
				ExpectedRoles:       cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                "channel admin explicit",
				ExplicitRoles:       "channel_user channel_admin",
				ExpectedRoles:       cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedSchemeUser:  true,
				ExpectedSchemeAdmin: true,
			},
			{
				Name:                  "channel user implicit and explicit custom role",
				SchemeUser:            true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel user explicit and explicit custom role",
				ExplicitRoles:         "channel_user test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
			},
			{
				Name:                  "channel guest implicit and explicit custom role",
				SchemeGuest:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel guest explicit and explicit custom role",
				ExplicitRoles:         "channel_guest test",
				ExpectedRoles:         "test " + cs.DefaultChannelGuestRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeGuest:   true,
			},
			{
				Name:                  "channel admin implicit and explicit custom role",
				SchemeUser:            true,
				SchemeAdmin:           true,
				ExplicitRoles:         "test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel admin explicit and explicit custom role",
				ExplicitRoles:         "channel_user channel_admin test",
				ExpectedRoles:         "test " + cs.DefaultChannelUserRole + " " + cs.DefaultChannelAdminRole,
				ExpectedExplicitRoles: "test",
				ExpectedSchemeUser:    true,
				ExpectedSchemeAdmin:   true,
			},
			{
				Name:                  "channel member with only explicit custom roles",
				ExplicitRoles:         "test test2",
				ExpectedRoles:         "test test2",
				ExpectedExplicitRoles: "test test2",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				member.SchemeGuest = tc.SchemeGuest
				member.SchemeUser = tc.SchemeUser
				member.SchemeAdmin = tc.SchemeAdmin
				member.ExplicitRoles = tc.ExplicitRoles
				members, err := ss.Channel().UpdateMultipleMembers([]*model.ChannelMember{member, otherMember})
				require.NoError(t, err)
				require.Len(t, members, 2)
				member = members[0]

				assert.Equal(t, tc.ExpectedRoles, member.Roles)
				assert.Equal(t, tc.ExpectedExplicitRoles, member.ExplicitRoles)
				assert.Equal(t, tc.ExpectedSchemeGuest, member.SchemeGuest)
				assert.Equal(t, tc.ExpectedSchemeUser, member.SchemeUser)
				assert.Equal(t, tc.ExpectedSchemeAdmin, member.SchemeAdmin)
			})
		}
	})
}

func testChannelRemoveMember(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	u2, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	u3, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	u4, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	channelID := model.NewId()
	defaultNotifyProps := model.GetDefaultChannelNotifyProps()
	m1 := &model.ChannelMember{ChannelId: channelID, UserId: u1.Id, NotifyProps: defaultNotifyProps}
	m2 := &model.ChannelMember{ChannelId: channelID, UserId: u2.Id, NotifyProps: defaultNotifyProps}
	m3 := &model.ChannelMember{ChannelId: channelID, UserId: u3.Id, NotifyProps: defaultNotifyProps}
	m4 := &model.ChannelMember{ChannelId: channelID, UserId: u4.Id, NotifyProps: defaultNotifyProps}
	_, nErr := ss.Channel().SaveMultipleMembers([]*model.ChannelMember{m1, m2, m3, m4})
	require.NoError(t, nErr)

	t.Run("remove member from not existing channel", func(t *testing.T) {
		nErr = ss.Channel().RemoveMember("not-existing-channel", u1.Id)
		require.NoError(t, nErr)
		var membersCount int64
		membersCount, nErr = ss.Channel().GetMemberCount(channelID, false)
		require.NoError(t, nErr)
		require.Equal(t, int64(4), membersCount)
	})

	t.Run("remove not existing member from an existing channel", func(t *testing.T) {
		nErr = ss.Channel().RemoveMember(channelID, model.NewId())
		require.NoError(t, nErr)
		var membersCount int64
		membersCount, nErr = ss.Channel().GetMemberCount(channelID, false)
		require.NoError(t, nErr)
		require.Equal(t, int64(4), membersCount)
	})

	t.Run("remove existing member from an existing channel", func(t *testing.T) {
		nErr = ss.Channel().RemoveMember(channelID, u1.Id)
		require.NoError(t, nErr)
		defer ss.Channel().SaveMember(m1)
		var membersCount int64
		membersCount, nErr = ss.Channel().GetMemberCount(channelID, false)
		require.NoError(t, nErr)
		require.Equal(t, int64(3), membersCount)
	})
}

func testChannelRemoveMembers(t *testing.T, ss store.Store) {
	u1, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	u2, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	u3, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	u4, err := ss.User().Save(&model.User{Username: model.NewId(), Email: MakeEmail()})
	require.NoError(t, err)
	channelID := model.NewId()
	defaultNotifyProps := model.GetDefaultChannelNotifyProps()
	m1 := &model.ChannelMember{ChannelId: channelID, UserId: u1.Id, NotifyProps: defaultNotifyProps}
	m2 := &model.ChannelMember{ChannelId: channelID, UserId: u2.Id, NotifyProps: defaultNotifyProps}
	m3 := &model.ChannelMember{ChannelId: channelID, UserId: u3.Id, NotifyProps: defaultNotifyProps}
	m4 := &model.ChannelMember{ChannelId: channelID, UserId: u4.Id, NotifyProps: defaultNotifyProps}
	_, nErr := ss.Channel().SaveMultipleMembers([]*model.ChannelMember{m1, m2, m3, m4})
	require.NoError(t, nErr)

	t.Run("remove members from not existing channel", func(t *testing.T) {
		nErr = ss.Channel().RemoveMembers("not-existing-channel", []string{u1.Id, u2.Id, u3.Id, u4.Id})
		require.NoError(t, nErr)
		var membersCount int64
		membersCount, nErr = ss.Channel().GetMemberCount(channelID, false)
		require.NoError(t, nErr)
		require.Equal(t, int64(4), membersCount)
	})

	t.Run("remove not existing members from an existing channel", func(t *testing.T) {
		nErr = ss.Channel().RemoveMembers(channelID, []string{model.NewId(), model.NewId()})
		require.NoError(t, nErr)
		var membersCount int64
		membersCount, nErr = ss.Channel().GetMemberCount(channelID, false)
		require.NoError(t, nErr)
		require.Equal(t, int64(4), membersCount)
	})

	t.Run("remove not existing and not existing members from an existing channel", func(t *testing.T) {
		nErr = ss.Channel().RemoveMembers(channelID, []string{u1.Id, u2.Id, model.NewId(), model.NewId()})
		require.NoError(t, nErr)
		defer ss.Channel().SaveMultipleMembers([]*model.ChannelMember{m1, m2})
		var membersCount int64
		membersCount, nErr = ss.Channel().GetMemberCount(channelID, false)
		require.NoError(t, nErr)
		require.Equal(t, int64(2), membersCount)
	})
	t.Run("remove existing members from an existing channel", func(t *testing.T) {
		nErr = ss.Channel().RemoveMembers(channelID, []string{u1.Id, u2.Id, u3.Id})
		require.NoError(t, nErr)
		defer ss.Channel().SaveMultipleMembers([]*model.ChannelMember{m1, m2, m3})
		membersCount, err := ss.Channel().GetMemberCount(channelID, false)
		require.NoError(t, err)
		require.Equal(t, int64(1), membersCount)
	})
}

func testChannelDeleteMemberStore(t *testing.T, ss store.Store) {
	c1 := &model.Channel{}
	c1.TeamId = model.NewId()
	c1.DisplayName = "NameName"
	c1.Name = "zz" + model.NewId() + "b"
	c1.Type = model.CHANNEL_OPEN
	c1, nErr := ss.Channel().Save(c1, -1)
	require.NoError(t, nErr)

	c1t1, _ := ss.Channel().Get(c1.Id, false)
	assert.EqualValues(t, 0, c1t1.ExtraUpdateAt, "ExtraUpdateAt should be 0")

	u1 := model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(&u1)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(&u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	o1 := model.ChannelMember{}
	o1.ChannelId = c1.Id
	o1.UserId = u1.Id
	o1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, nErr = ss.Channel().SaveMember(&o1)
	require.NoError(t, nErr)

	o2 := model.ChannelMember{}
	o2.ChannelId = c1.Id
	o2.UserId = u2.Id
	o2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, nErr = ss.Channel().SaveMember(&o2)
	require.NoError(t, nErr)

	c1t2, _ := ss.Channel().Get(c1.Id, false)
	assert.EqualValues(t, 0, c1t2.ExtraUpdateAt, "ExtraUpdateAt should be 0")

	count, nErr := ss.Channel().GetMemberCount(o1.ChannelId, false)
	require.NoError(t, nErr)
	require.EqualValues(t, 2, count, "should have saved 2 members")

	nErr = ss.Channel().PermanentDeleteMembersByUser(o2.UserId)
	require.NoError(t, nErr)

	count, nErr = ss.Channel().GetMemberCount(o1.ChannelId, false)
	require.NoError(t, nErr)
	require.EqualValues(t, 1, count, "should have removed 1 member")

	nErr = ss.Channel().PermanentDeleteMembersByChannel(o1.ChannelId)
	require.NoError(t, nErr)

	count, nErr = ss.Channel().GetMemberCount(o1.ChannelId, false)
	require.NoError(t, nErr)
	require.EqualValues(t, 0, count, "should have removed all members")
}

func testChannelStoreGetChannels(t *testing.T, ss store.Store) {
	team := model.NewId()
	o1 := model.Channel{}
	o1.TeamId = team
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{}
	o2.TeamId = team
	o2.DisplayName = "Channel2"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	o3 := model.Channel{}
	o3.TeamId = team
	o3.DisplayName = "Channel3"
	o3.Name = "zz" + model.NewId() + "b"
	o3.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = model.NewId()
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err := ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{}
	m2.ChannelId = o1.Id
	m2.UserId = model.NewId()
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	m3 := model.ChannelMember{}
	m3.ChannelId = o2.Id
	m3.UserId = m1.UserId
	m3.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m3)
	require.NoError(t, err)

	m4 := model.ChannelMember{}
	m4.ChannelId = o3.Id
	m4.UserId = m1.UserId
	m4.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m4)
	require.NoError(t, err)

	list, nErr := ss.Channel().GetChannels(o1.TeamId, m1.UserId, false, 0)
	require.NoError(t, nErr)
	require.Len(t, *list, 3)
	require.Equal(t, o1.Id, (*list)[0].Id, "missing channel")
	require.Equal(t, o2.Id, (*list)[1].Id, "missing channel")
	require.Equal(t, o3.Id, (*list)[2].Id, "missing channel")

	ids, err := ss.Channel().GetAllChannelMembersForUser(m1.UserId, false, false)
	require.NoError(t, err)
	_, ok := ids[o1.Id]
	require.True(t, ok, "missing channel")

	ids2, err := ss.Channel().GetAllChannelMembersForUser(m1.UserId, true, false)
	require.NoError(t, err)
	_, ok = ids2[o1.Id]
	require.True(t, ok, "missing channel")

	ids3, err := ss.Channel().GetAllChannelMembersForUser(m1.UserId, true, false)
	require.NoError(t, err)
	_, ok = ids3[o1.Id]
	require.True(t, ok, "missing channel")

	ids4, err := ss.Channel().GetAllChannelMembersForUser(m1.UserId, true, true)
	require.NoError(t, err)
	_, ok = ids4[o1.Id]
	require.True(t, ok, "missing channel")

	nErr = ss.Channel().Delete(o2.Id, 10)
	require.NoError(t, nErr)

	nErr = ss.Channel().Delete(o3.Id, 20)
	require.NoError(t, nErr)

	// should return 1
	list, nErr = ss.Channel().GetChannels(o1.TeamId, m1.UserId, false, 0)
	require.NoError(t, nErr)
	require.Len(t, *list, 1)
	require.Equal(t, o1.Id, (*list)[0].Id, "missing channel")

	// Should return all
	list, nErr = ss.Channel().GetChannels(o1.TeamId, m1.UserId, true, 0)
	require.NoError(t, nErr)
	require.Len(t, *list, 3)
	require.Equal(t, o1.Id, (*list)[0].Id, "missing channel")
	require.Equal(t, o2.Id, (*list)[1].Id, "missing channel")
	require.Equal(t, o3.Id, (*list)[2].Id, "missing channel")

	// Should still return all
	list, nErr = ss.Channel().GetChannels(o1.TeamId, m1.UserId, true, 10)
	require.NoError(t, nErr)
	require.Len(t, *list, 3)
	require.Equal(t, o1.Id, (*list)[0].Id, "missing channel")
	require.Equal(t, o2.Id, (*list)[1].Id, "missing channel")
	require.Equal(t, o3.Id, (*list)[2].Id, "missing channel")

	// Should return 2
	list, nErr = ss.Channel().GetChannels(o1.TeamId, m1.UserId, true, 20)
	require.NoError(t, nErr)
	require.Len(t, *list, 2)
	require.Equal(t, o1.Id, (*list)[0].Id, "missing channel")
	require.Equal(t, o3.Id, (*list)[1].Id, "missing channel")

	require.True(
		t,
		ss.Channel().IsUserInChannelUseCache(m1.UserId, o1.Id),
		"missing channel")
	require.True(
		t,
		ss.Channel().IsUserInChannelUseCache(m1.UserId, o2.Id),
		"missing channel")

	require.False(
		t,
		ss.Channel().IsUserInChannelUseCache(m1.UserId, "blahblah"),
		"missing channel")

	require.False(
		t,
		ss.Channel().IsUserInChannelUseCache("blahblah", "blahblah"),
		"missing channel")

	ss.Channel().InvalidateAllChannelMembersForUser(m1.UserId)
}

func testChannelStoreGetAllChannels(t *testing.T, ss store.Store, s SqlStore) {
	cleanupChannels(t, ss)

	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewId()
	t1.Email = MakeEmail()
	t1.Type = model.TEAM_OPEN
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	t2 := model.Team{}
	t2.DisplayName = "Name2"
	t2.Name = "zz" + model.NewId()
	t2.Email = MakeEmail()
	t2.Type = model.TEAM_OPEN
	_, err = ss.Team().Save(&t2)
	require.NoError(t, err)

	c1 := model.Channel{}
	c1.TeamId = t1.Id
	c1.DisplayName = "Channel1" + model.NewId()
	c1.Name = "zz" + model.NewId() + "b"
	c1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	group := &model.Group{
		Name:        model.NewString(model.NewId()),
		DisplayName: model.NewId(),
		Source:      model.GroupSourceLdap,
		RemoteId:    model.NewId(),
	}
	_, err = ss.Group().Create(group)
	require.NoError(t, err)

	_, err = ss.Group().CreateGroupSyncable(model.NewGroupChannel(group.Id, c1.Id, true))
	require.NoError(t, err)

	c2 := model.Channel{}
	c2.TeamId = t1.Id
	c2.DisplayName = "Channel2" + model.NewId()
	c2.Name = "zz" + model.NewId() + "b"
	c2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&c2, -1)
	require.NoError(t, nErr)
	c2.DeleteAt = model.GetMillis()
	c2.UpdateAt = c2.DeleteAt
	nErr = ss.Channel().Delete(c2.Id, c2.DeleteAt)
	require.NoError(t, nErr, "channel should have been deleted")

	c3 := model.Channel{}
	c3.TeamId = t2.Id
	c3.DisplayName = "Channel3" + model.NewId()
	c3.Name = "zz" + model.NewId() + "b"
	c3.Type = model.CHANNEL_PRIVATE
	_, nErr = ss.Channel().Save(&c3, -1)
	require.NoError(t, nErr)

	u1 := model.User{Id: model.NewId()}
	u2 := model.User{Id: model.NewId()}
	_, nErr = ss.Channel().CreateDirectChannel(&u1, &u2)
	require.NoError(t, nErr)

	userIds := []string{model.NewId(), model.NewId(), model.NewId()}

	c5 := model.Channel{}
	c5.Name = model.GetGroupNameFromUserIds(userIds)
	c5.DisplayName = "GroupChannel" + model.NewId()
	c5.Name = "zz" + model.NewId() + "b"
	c5.Type = model.CHANNEL_GROUP
	_, nErr = ss.Channel().Save(&c5, -1)
	require.NoError(t, nErr)

	list, nErr := ss.Channel().GetAllChannels(0, 10, store.ChannelSearchOpts{})
	require.NoError(t, nErr)
	assert.Len(t, *list, 2)
	assert.Equal(t, c1.Id, (*list)[0].Id)
	assert.Equal(t, "Name", (*list)[0].TeamDisplayName)
	assert.Equal(t, c3.Id, (*list)[1].Id)
	assert.Equal(t, "Name2", (*list)[1].TeamDisplayName)

	count1, nErr := ss.Channel().GetAllChannelsCount(store.ChannelSearchOpts{})
	require.NoError(t, nErr)

	list, nErr = ss.Channel().GetAllChannels(0, 10, store.ChannelSearchOpts{IncludeDeleted: true})
	require.NoError(t, nErr)
	assert.Len(t, *list, 3)
	assert.Equal(t, c1.Id, (*list)[0].Id)
	assert.Equal(t, "Name", (*list)[0].TeamDisplayName)
	assert.Equal(t, c2.Id, (*list)[1].Id)
	assert.Equal(t, c3.Id, (*list)[2].Id)

	count2, nErr := ss.Channel().GetAllChannelsCount(store.ChannelSearchOpts{IncludeDeleted: true})
	require.NoError(t, nErr)
	require.True(t, func() bool {
		return count2 > count1
	}())

	list, nErr = ss.Channel().GetAllChannels(0, 1, store.ChannelSearchOpts{IncludeDeleted: true})
	require.NoError(t, nErr)
	assert.Len(t, *list, 1)
	assert.Equal(t, c1.Id, (*list)[0].Id)
	assert.Equal(t, "Name", (*list)[0].TeamDisplayName)

	// Not associated to group
	list, nErr = ss.Channel().GetAllChannels(0, 10, store.ChannelSearchOpts{NotAssociatedToGroup: group.Id})
	require.NoError(t, nErr)
	assert.Len(t, *list, 1)

	// Exclude channel names
	list, nErr = ss.Channel().GetAllChannels(0, 10, store.ChannelSearchOpts{ExcludeChannelNames: []string{c1.Name}})
	require.NoError(t, nErr)
	assert.Len(t, *list, 1)

	// Exclude policy constrained
	policy, nErr := ss.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:  "Policy 1",
			PostDuration: model.NewInt64(30),
		},
		ChannelIDs: []string{c1.Id},
	})
	require.NoError(t, nErr)
	list, nErr = ss.Channel().GetAllChannels(0, 10, store.ChannelSearchOpts{ExcludePolicyConstrained: true})
	require.NoError(t, nErr)
	assert.Len(t, *list, 1)
	assert.Equal(t, c3.Id, (*list)[0].Id)

	// Without the policy ID
	list, nErr = ss.Channel().GetAllChannels(0, 1, store.ChannelSearchOpts{})
	require.NoError(t, nErr)
	assert.Len(t, *list, 1)
	assert.Equal(t, c1.Id, (*list)[0].Id)
	assert.Nil(t, (*list)[0].PolicyID)
	// With the policy ID
	list, nErr = ss.Channel().GetAllChannels(0, 1, store.ChannelSearchOpts{IncludePolicyID: true})
	require.NoError(t, nErr)
	assert.Len(t, *list, 1)
	assert.Equal(t, c1.Id, (*list)[0].Id)
	assert.Equal(t, *(*list)[0].PolicyID, policy.ID)

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testChannelStoreGetMoreChannels(t *testing.T, ss store.Store) {
	teamId := model.NewId()
	otherTeamId := model.NewId()
	userId := model.NewId()
	otherUserId1 := model.NewId()
	otherUserId2 := model.NewId()

	// o1 is a channel on the team to which the user (and the other user 1) belongs
	o1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	_, err := ss.Channel().SaveMember(&model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      userId,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      otherUserId1,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	// o2 is a channel on the other team to which the user belongs
	o2 := model.Channel{
		TeamId:      otherTeamId,
		DisplayName: "Channel2",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelId:   o2.Id,
		UserId:      otherUserId2,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	// o3 is a channel on the team to which the user does not belong, and thus should show up
	// in "more channels"
	o3 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelA",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	// o4 is a private channel on the team to which the user does not belong
	o4 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelB",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o4, -1)
	require.NoError(t, nErr)

	// o5 is another private channel on the team to which the user does belong
	o5 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelC",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o5, -1)
	require.NoError(t, nErr)

	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelId:   o5.Id,
		UserId:      userId,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	t.Run("only o3 listed in more channels", func(t *testing.T) {
		list, channelErr := ss.Channel().GetMoreChannels(teamId, userId, 0, 100)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&o3}, list)
	})

	// o6 is another channel on the team to which the user does not belong, and would thus
	// start showing up in "more channels".
	o6 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelD",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o6, -1)
	require.NoError(t, nErr)

	// o7 is another channel on the team to which the user does not belong, but is deleted,
	// and thus would not start showing up in "more channels"
	o7 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelD",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o7, -1)
	require.NoError(t, nErr)

	nErr = ss.Channel().Delete(o7.Id, model.GetMillis())
	require.NoError(t, nErr, "channel should have been deleted")

	t.Run("both o3 and o6 listed in more channels", func(t *testing.T) {
		list, err := ss.Channel().GetMoreChannels(teamId, userId, 0, 100)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o3, &o6}, list)
	})

	t.Run("only o3 listed in more channels with offset 0, limit 1", func(t *testing.T) {
		list, err := ss.Channel().GetMoreChannels(teamId, userId, 0, 1)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o3}, list)
	})

	t.Run("only o6 listed in more channels with offset 1, limit 1", func(t *testing.T) {
		list, err := ss.Channel().GetMoreChannels(teamId, userId, 1, 1)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o6}, list)
	})

	t.Run("verify analytics for open channels", func(t *testing.T) {
		count, err := ss.Channel().AnalyticsTypeCount(teamId, model.CHANNEL_OPEN)
		require.NoError(t, err)
		require.EqualValues(t, 4, count)
	})

	t.Run("verify analytics for private channels", func(t *testing.T) {
		count, err := ss.Channel().AnalyticsTypeCount(teamId, model.CHANNEL_PRIVATE)
		require.NoError(t, err)
		require.EqualValues(t, 2, count)
	})
}

func testChannelStoreGetPrivateChannelsForTeam(t *testing.T, ss store.Store) {
	teamId := model.NewId()

	// p1 is a private channel on the team
	p1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "PrivateChannel1Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr := ss.Channel().Save(&p1, -1)
	require.NoError(t, nErr)

	// p2 is a private channel on another team
	p2 := model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "PrivateChannel1Team2",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&p2, -1)
	require.NoError(t, nErr)

	// o1 is a public channel on the team
	o1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "OpenChannel1Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	t.Run("only p1 initially listed in private channels", func(t *testing.T) {
		list, channelErr := ss.Channel().GetPrivateChannelsForTeam(teamId, 0, 100)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&p1}, list)
	})

	// p3 is another private channel on the team
	p3 := model.Channel{
		TeamId:      teamId,
		DisplayName: "PrivateChannel2Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&p3, -1)
	require.NoError(t, nErr)

	// p4 is another private, but deleted channel on the team
	p4 := model.Channel{
		TeamId:      teamId,
		DisplayName: "PrivateChannel3Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&p4, -1)
	require.NoError(t, nErr)
	err := ss.Channel().Delete(p4.Id, model.GetMillis())
	require.NoError(t, err, "channel should have been deleted")

	t.Run("both p1 and p3 listed in private channels", func(t *testing.T) {
		list, err := ss.Channel().GetPrivateChannelsForTeam(teamId, 0, 100)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&p1, &p3}, list)
	})

	t.Run("only p1 listed in private channels with offset 0, limit 1", func(t *testing.T) {
		list, err := ss.Channel().GetPrivateChannelsForTeam(teamId, 0, 1)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&p1}, list)
	})

	t.Run("only p3 listed in private channels with offset 1, limit 1", func(t *testing.T) {
		list, err := ss.Channel().GetPrivateChannelsForTeam(teamId, 1, 1)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&p3}, list)
	})

	t.Run("verify analytics for private channels", func(t *testing.T) {
		count, err := ss.Channel().AnalyticsTypeCount(teamId, model.CHANNEL_PRIVATE)
		require.NoError(t, err)
		require.EqualValues(t, 3, count)
	})

	t.Run("verify analytics for open open channels", func(t *testing.T) {
		count, err := ss.Channel().AnalyticsTypeCount(teamId, model.CHANNEL_OPEN)
		require.NoError(t, err)
		require.EqualValues(t, 1, count)
	})
}

func testChannelStoreGetPublicChannelsForTeam(t *testing.T, ss store.Store) {
	teamId := model.NewId()

	// o1 is a public channel on the team
	o1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "OpenChannel1Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	// o2 is a public channel on another team
	o2 := model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "OpenChannel1Team2",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	// o3 is a private channel on the team
	o3 := model.Channel{
		TeamId:      teamId,
		DisplayName: "PrivateChannel1Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	t.Run("only o1 initially listed in public channels", func(t *testing.T) {
		list, channelErr := ss.Channel().GetPublicChannelsForTeam(teamId, 0, 100)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&o1}, list)
	})

	// o4 is another public channel on the team
	o4 := model.Channel{
		TeamId:      teamId,
		DisplayName: "OpenChannel2Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o4, -1)
	require.NoError(t, nErr)

	// o5 is another public, but deleted channel on the team
	o5 := model.Channel{
		TeamId:      teamId,
		DisplayName: "OpenChannel3Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o5, -1)
	require.NoError(t, nErr)
	err := ss.Channel().Delete(o5.Id, model.GetMillis())
	require.NoError(t, err, "channel should have been deleted")

	t.Run("both o1 and o4 listed in public channels", func(t *testing.T) {
		list, err := ss.Channel().GetPublicChannelsForTeam(teamId, 0, 100)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o1, &o4}, list)
	})

	t.Run("only o1 listed in public channels with offset 0, limit 1", func(t *testing.T) {
		list, err := ss.Channel().GetPublicChannelsForTeam(teamId, 0, 1)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o1}, list)
	})

	t.Run("only o4 listed in public channels with offset 1, limit 1", func(t *testing.T) {
		list, err := ss.Channel().GetPublicChannelsForTeam(teamId, 1, 1)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o4}, list)
	})

	t.Run("verify analytics for open channels", func(t *testing.T) {
		count, err := ss.Channel().AnalyticsTypeCount(teamId, model.CHANNEL_OPEN)
		require.NoError(t, err)
		require.EqualValues(t, 3, count)
	})

	t.Run("verify analytics for private channels", func(t *testing.T) {
		count, err := ss.Channel().AnalyticsTypeCount(teamId, model.CHANNEL_PRIVATE)
		require.NoError(t, err)
		require.EqualValues(t, 1, count)
	})
}

func testChannelStoreGetPublicChannelsByIdsForTeam(t *testing.T, ss store.Store) {
	teamId := model.NewId()

	// oc1 is a public channel on the team
	oc1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "OpenChannel1Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&oc1, -1)
	require.NoError(t, nErr)

	// oc2 is a public channel on another team
	oc2 := model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "OpenChannel2TeamOther",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&oc2, -1)
	require.NoError(t, nErr)

	// pc3 is a private channel on the team
	pc3 := model.Channel{
		TeamId:      teamId,
		DisplayName: "PrivateChannel3Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&pc3, -1)
	require.NoError(t, nErr)

	t.Run("oc1 by itself should be found as a public channel in the team", func(t *testing.T) {
		list, channelErr := ss.Channel().GetPublicChannelsByIdsForTeam(teamId, []string{oc1.Id})
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&oc1}, list)
	})

	t.Run("only oc1, among others, should be found as a public channel in the team", func(t *testing.T) {
		list, channelErr := ss.Channel().GetPublicChannelsByIdsForTeam(teamId, []string{oc1.Id, oc2.Id, model.NewId(), pc3.Id})
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&oc1}, list)
	})

	// oc4 is another public channel on the team
	oc4 := model.Channel{
		TeamId:      teamId,
		DisplayName: "OpenChannel4Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&oc4, -1)
	require.NoError(t, nErr)

	// oc4 is another public, but deleted channel on the team
	oc5 := model.Channel{
		TeamId:      teamId,
		DisplayName: "OpenChannel4Team1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&oc5, -1)
	require.NoError(t, nErr)

	err := ss.Channel().Delete(oc5.Id, model.GetMillis())
	require.NoError(t, err, "channel should have been deleted")

	t.Run("only oc1 and oc4, among others, should be found as a public channel in the team", func(t *testing.T) {
		list, err := ss.Channel().GetPublicChannelsByIdsForTeam(teamId, []string{oc1.Id, oc2.Id, model.NewId(), pc3.Id, oc4.Id})
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&oc1, &oc4}, list)
	})

	t.Run("random channel id should not be found as a public channel in the team", func(t *testing.T) {
		_, err := ss.Channel().GetPublicChannelsByIdsForTeam(teamId, []string{model.NewId()})
		require.Error(t, err)
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(err, &nfErr))
	})
}

func testChannelStoreGetChannelCounts(t *testing.T, ss store.Store) {
	o2 := model.Channel{}
	o2.TeamId = model.NewId()
	o2.DisplayName = "Channel2"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = model.NewId()
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err := ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{}
	m2.ChannelId = o1.Id
	m2.UserId = model.NewId()
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	m3 := model.ChannelMember{}
	m3.ChannelId = o2.Id
	m3.UserId = model.NewId()
	m3.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m3)
	require.NoError(t, err)

	counts, _ := ss.Channel().GetChannelCounts(o1.TeamId, m1.UserId)

	require.Len(t, counts.Counts, 1, "wrong number of counts")
	require.Len(t, counts.UpdateTimes, 1, "wrong number of update times")
}

func testChannelStoreGetMembersForUser(t *testing.T, ss store.Store) {
	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewId()
	t1.Email = MakeEmail()
	t1.Type = model.TEAM_OPEN
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	o1 := model.Channel{}
	o1.TeamId = t1.Id
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{}
	o2.TeamId = o1.TeamId
	o2.DisplayName = "Channel2"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = model.NewId()
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{}
	m2.ChannelId = o2.Id
	m2.UserId = m1.UserId
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	t.Run("with channels", func(t *testing.T) {
		var members *model.ChannelMembers
		members, err = ss.Channel().GetMembersForUser(o1.TeamId, m1.UserId)
		require.NoError(t, err)

		assert.Len(t, *members, 2)
	})

	t.Run("with channels and direct messages", func(t *testing.T) {
		user := model.User{Id: m1.UserId}
		u1 := model.User{Id: model.NewId()}
		u2 := model.User{Id: model.NewId()}
		u3 := model.User{Id: model.NewId()}
		u4 := model.User{Id: model.NewId()}
		_, nErr = ss.Channel().CreateDirectChannel(&u1, &user)
		require.NoError(t, nErr)
		_, nErr = ss.Channel().CreateDirectChannel(&u2, &user)
		require.NoError(t, nErr)
		// other user direct message
		_, nErr = ss.Channel().CreateDirectChannel(&u3, &u4)
		require.NoError(t, nErr)

		var members *model.ChannelMembers
		members, err = ss.Channel().GetMembersForUser(o1.TeamId, m1.UserId)
		require.NoError(t, err)

		assert.Len(t, *members, 4)
	})

	t.Run("with channels, direct channels and group messages", func(t *testing.T) {
		userIds := []string{model.NewId(), model.NewId(), model.NewId(), m1.UserId}
		group := &model.Channel{
			Name:        model.GetGroupNameFromUserIds(userIds),
			DisplayName: "test",
			Type:        model.CHANNEL_GROUP,
		}
		var channel *model.Channel
		channel, nErr = ss.Channel().Save(group, 10000)
		require.NoError(t, nErr)
		for _, userId := range userIds {
			cm := &model.ChannelMember{
				UserId:      userId,
				ChannelId:   channel.Id,
				NotifyProps: model.GetDefaultChannelNotifyProps(),
				SchemeUser:  true,
			}

			_, err = ss.Channel().SaveMember(cm)
			require.NoError(t, err)
		}
		var members *model.ChannelMembers
		members, err = ss.Channel().GetMembersForUser(o1.TeamId, m1.UserId)
		require.NoError(t, err)

		assert.Len(t, *members, 5)
	})
}

func testChannelStoreGetMembersForUserWithPagination(t *testing.T, ss store.Store) {
	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewId()
	t1.Email = MakeEmail()
	t1.Type = model.TEAM_OPEN
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	o1 := model.Channel{}
	o1.TeamId = t1.Id
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{}
	o2.TeamId = o1.TeamId
	o2.DisplayName = "Channel2"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = model.NewId()
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{}
	m2.ChannelId = o2.Id
	m2.UserId = m1.UserId
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	members, err := ss.Channel().GetMembersForUserWithPagination(o1.TeamId, m1.UserId, 0, 1)
	require.NoError(t, err)
	assert.Len(t, *members, 1)

	members, err = ss.Channel().GetMembersForUserWithPagination(o1.TeamId, m1.UserId, 1, 1)
	require.NoError(t, err)
	assert.Len(t, *members, 1)
}

func testCountPostsAfter(t *testing.T, ss store.Store) {
	t.Run("should count all posts with or without the given user ID", func(t *testing.T) {
		userId1 := model.NewId()
		userId2 := model.NewId()

		channelId := model.NewId()

		p1, err := ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1000,
		})
		require.NoError(t, err)

		_, err = ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1001,
		})
		require.NoError(t, err)

		_, err = ss.Post().Save(&model.Post{
			UserId:    userId2,
			ChannelId: channelId,
			CreateAt:  1002,
		})
		require.NoError(t, err)

		count, _, err := ss.Channel().CountPostsAfter(channelId, p1.CreateAt-1, "")
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		count, _, err = ss.Channel().CountPostsAfter(channelId, p1.CreateAt, "")
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		count, _, err = ss.Channel().CountPostsAfter(channelId, p1.CreateAt-1, userId1)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		count, _, err = ss.Channel().CountPostsAfter(channelId, p1.CreateAt, userId1)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("should not count deleted posts", func(t *testing.T) {
		userId1 := model.NewId()

		channelId := model.NewId()

		p1, err := ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1000,
		})
		require.NoError(t, err)

		_, err = ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1001,
			DeleteAt:  1001,
		})
		require.NoError(t, err)

		count, _, err := ss.Channel().CountPostsAfter(channelId, p1.CreateAt-1, "")
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		count, _, err = ss.Channel().CountPostsAfter(channelId, p1.CreateAt, "")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("should count system/bot messages, but not join/leave messages", func(t *testing.T) {
		userId1 := model.NewId()

		channelId := model.NewId()

		p1, err := ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1000,
		})
		require.NoError(t, err)

		_, err = ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1001,
			Type:      model.POST_JOIN_CHANNEL,
		})
		require.NoError(t, err)

		_, err = ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1002,
			Type:      model.POST_REMOVE_FROM_CHANNEL,
		})
		require.NoError(t, err)

		_, err = ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1003,
			Type:      model.POST_LEAVE_TEAM,
		})
		require.NoError(t, err)

		p5, err := ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1004,
			Type:      model.POST_HEADER_CHANGE,
		})
		require.NoError(t, err)

		_, err = ss.Post().Save(&model.Post{
			UserId:    userId1,
			ChannelId: channelId,
			CreateAt:  1005,
			Type:      "custom_nps_survey",
		})
		require.NoError(t, err)

		count, _, err := ss.Channel().CountPostsAfter(channelId, p1.CreateAt-1, "")
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		count, _, err = ss.Channel().CountPostsAfter(channelId, p1.CreateAt, "")
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		count, _, err = ss.Channel().CountPostsAfter(channelId, p5.CreateAt-1, "")
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		count, _, err = ss.Channel().CountPostsAfter(channelId, p5.CreateAt, "")
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func testChannelStoreUpdateLastViewedAt(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	o1.TotalMsgCount = 25
	o1.LastPostAt = 12345
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = model.NewId()
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err := ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	o2 := model.Channel{}
	o2.TeamId = model.NewId()
	o2.DisplayName = "Channel1"
	o2.Name = "zz" + model.NewId() + "c"
	o2.Type = model.CHANNEL_OPEN
	o2.TotalMsgCount = 26
	o2.LastPostAt = 123456
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	m2 := model.ChannelMember{}
	m2.ChannelId = o2.Id
	m2.UserId = m1.UserId
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	var times map[string]int64
	times, err = ss.Channel().UpdateLastViewedAt([]string{m1.ChannelId}, m1.UserId, false)
	require.NoError(t, err, "failed to update ", err)
	require.Equal(t, o1.LastPostAt, times[o1.Id], "last viewed at time incorrect")

	times, err = ss.Channel().UpdateLastViewedAt([]string{m1.ChannelId, m2.ChannelId}, m1.UserId, false)
	require.NoError(t, err, "failed to update ", err)
	require.Equal(t, o2.LastPostAt, times[o2.Id], "last viewed at time incorrect")

	rm1, err := ss.Channel().GetMember(context.Background(), m1.ChannelId, m1.UserId)
	assert.NoError(t, err)
	assert.Equal(t, o1.LastPostAt, rm1.LastViewedAt)
	assert.Equal(t, o1.LastPostAt, rm1.LastUpdateAt)
	assert.Equal(t, o1.TotalMsgCount, rm1.MsgCount)

	rm2, err := ss.Channel().GetMember(context.Background(), m2.ChannelId, m2.UserId)
	assert.NoError(t, err)
	assert.Equal(t, o2.LastPostAt, rm2.LastViewedAt)
	assert.Equal(t, o2.LastPostAt, rm2.LastUpdateAt)
	assert.Equal(t, o2.TotalMsgCount, rm2.MsgCount)

	_, err = ss.Channel().UpdateLastViewedAt([]string{m1.ChannelId}, "missing id", false)
	require.NoError(t, err, "failed to update")
}

func testChannelStoreIncrementMentionCount(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "Channel1"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	o1.TotalMsgCount = 25
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = model.NewId()
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err := ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	err = ss.Channel().IncrementMentionCount(m1.ChannelId, m1.UserId, false, false)
	require.NoError(t, err, "failed to update")

	err = ss.Channel().IncrementMentionCount(m1.ChannelId, "missing id", false, false)
	require.NoError(t, err, "failed to update")

	err = ss.Channel().IncrementMentionCount("missing id", m1.UserId, false, false)
	require.NoError(t, err, "failed to update")

	err = ss.Channel().IncrementMentionCount("missing id", "missing id", false, false)
	require.NoError(t, err, "failed to update")
}

func testUpdateChannelMember(t *testing.T, ss store.Store) {
	userId := model.NewId()

	c1 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: model.NewId(),
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(c1, -1)
	require.NoError(t, nErr)

	m1 := &model.ChannelMember{
		ChannelId:   c1.Id,
		UserId:      userId,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err := ss.Channel().SaveMember(m1)
	require.NoError(t, err)

	m1.NotifyProps["test"] = "sometext"
	_, err = ss.Channel().UpdateMember(m1)
	require.NoError(t, err, err)

	m1.UserId = ""
	_, err = ss.Channel().UpdateMember(m1)
	require.Error(t, err, "bad user id - should fail")
}

func testGetMember(t *testing.T, ss store.Store) {
	userId := model.NewId()

	c1 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: model.NewId(),
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(c1, -1)
	require.NoError(t, nErr)

	c2 := &model.Channel{
		TeamId:      c1.TeamId,
		DisplayName: model.NewId(),
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(c2, -1)
	require.NoError(t, nErr)

	m1 := &model.ChannelMember{
		ChannelId:   c1.Id,
		UserId:      userId,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err := ss.Channel().SaveMember(m1)
	require.NoError(t, err)

	m2 := &model.ChannelMember{
		ChannelId:   c2.Id,
		UserId:      userId,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(m2)
	require.NoError(t, err)

	_, err = ss.Channel().GetMember(context.Background(), model.NewId(), userId)
	require.Error(t, err, "should've failed to get member for non-existent channel")

	_, err = ss.Channel().GetMember(context.Background(), c1.Id, model.NewId())
	require.Error(t, err, "should've failed to get member for non-existent user")

	member, err := ss.Channel().GetMember(context.Background(), c1.Id, userId)
	require.NoError(t, err, "shouldn't have errored when getting member", err)
	require.Equal(t, c1.Id, member.ChannelId, "should've gotten member of channel 1")
	require.Equal(t, userId, member.UserId, "should've have gotten member for user")

	member, err = ss.Channel().GetMember(context.Background(), c2.Id, userId)
	require.NoError(t, err, "should'nt have errored when getting member", err)
	require.Equal(t, c2.Id, member.ChannelId, "should've gotten member of channel 2")
	require.Equal(t, userId, member.UserId, "should've gotten member for user")

	props, err := ss.Channel().GetAllChannelMembersNotifyPropsForChannel(c2.Id, false)
	require.NoError(t, err, err)
	require.NotEqual(t, 0, len(props), "should not be empty")

	props, err = ss.Channel().GetAllChannelMembersNotifyPropsForChannel(c2.Id, true)
	require.NoError(t, err, err)
	require.NotEqual(t, 0, len(props), "should not be empty")

	ss.Channel().InvalidateCacheForChannelMembersNotifyProps(c2.Id)
}

func testChannelStoreGetMemberForPost(t *testing.T, ss store.Store) {
	ch := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}

	o1, nErr := ss.Channel().Save(ch, -1)
	require.NoError(t, nErr)

	m1, err := ss.Channel().SaveMember(&model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	p1, nErr := ss.Post().Save(&model.Post{
		UserId:    model.NewId(),
		ChannelId: o1.Id,
		Message:   "test",
	})
	require.NoError(t, nErr)

	r1, err := ss.Channel().GetMemberForPost(p1.Id, m1.UserId)
	require.NoError(t, err, err)
	require.Equal(t, m1.ToJson(), r1.ToJson(), "invalid returned channel member")

	_, err = ss.Channel().GetMemberForPost(p1.Id, model.NewId())
	require.Error(t, err, "shouldn't have returned a member")
}

func testGetMemberCount(t *testing.T, ss store.Store) {
	teamId := model.NewId()

	c1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	c2 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel2",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&c2, -1)
	require.NoError(t, nErr)

	u1 := &model.User{
		Email:    MakeEmail(),
		DeleteAt: 0,
	}
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{
		ChannelId:   c1.Id,
		UserId:      u1.Id,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, nErr = ss.Channel().SaveMember(&m1)
	require.NoError(t, nErr)

	count, channelErr := ss.Channel().GetMemberCount(c1.Id, false)
	require.NoError(t, channelErr, "failed to get member count", channelErr)
	require.EqualValuesf(t, 1, count, "got incorrect member count %v", count)

	u2 := model.User{
		Email:    MakeEmail(),
		DeleteAt: 0,
	}
	_, err = ss.User().Save(&u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	m2 := model.ChannelMember{
		ChannelId:   c1.Id,
		UserId:      u2.Id,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, nErr = ss.Channel().SaveMember(&m2)
	require.NoError(t, nErr)

	count, channelErr = ss.Channel().GetMemberCount(c1.Id, false)
	require.NoErrorf(t, channelErr, "failed to get member count: %v", channelErr)
	require.EqualValuesf(t, 2, count, "got incorrect member count %v", count)

	// make sure members of other channels aren't counted
	u3 := model.User{
		Email:    MakeEmail(),
		DeleteAt: 0,
	}
	_, err = ss.User().Save(&u3)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u3.Id}, -1)
	require.NoError(t, nErr)

	m3 := model.ChannelMember{
		ChannelId:   c2.Id,
		UserId:      u3.Id,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, nErr = ss.Channel().SaveMember(&m3)
	require.NoError(t, nErr)

	count, channelErr = ss.Channel().GetMemberCount(c1.Id, false)
	require.NoErrorf(t, channelErr, "failed to get member count: %v", channelErr)
	require.EqualValuesf(t, 2, count, "got incorrect member count %v", count)

	// make sure inactive users aren't counted
	u4 := &model.User{
		Email:    MakeEmail(),
		DeleteAt: 10000,
	}
	_, err = ss.User().Save(u4)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u4.Id}, -1)
	require.NoError(t, nErr)

	m4 := model.ChannelMember{
		ChannelId:   c1.Id,
		UserId:      u4.Id,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, nErr = ss.Channel().SaveMember(&m4)
	require.NoError(t, nErr)

	count, nErr = ss.Channel().GetMemberCount(c1.Id, false)
	require.NoError(t, nErr, "failed to get member count", nErr)
	require.EqualValuesf(t, 2, count, "got incorrect member count %v", count)
}

func testGetMemberCountsByGroup(t *testing.T, ss store.Store) {
	var memberCounts []*model.ChannelMemberCountByGroup
	teamId := model.NewId()
	g1 := &model.Group{
		Name:        model.NewString(model.NewId()),
		DisplayName: model.NewId(),
		Source:      model.GroupSourceLdap,
		RemoteId:    model.NewId(),
	}
	_, err := ss.Group().Create(g1)
	require.NoError(t, err)

	c1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	u1 := &model.User{
		Timezone: timezones.DefaultUserTimezone(),
		Email:    MakeEmail(),
		DeleteAt: 0,
	}
	_, nErr = ss.User().Save(u1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{
		ChannelId:   c1.Id,
		UserId:      u1.Id,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, nErr = ss.Channel().SaveMember(&m1)
	require.NoError(t, nErr)

	t.Run("empty slice for channel with no groups", func(t *testing.T) {
		memberCounts, nErr = ss.Channel().GetMemberCountsByGroup(context.Background(), c1.Id, false)
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{}
		require.NoError(t, nErr)
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	_, err = ss.Group().UpsertMember(g1.Id, u1.Id)
	require.NoError(t, err)

	t.Run("returns memberCountsByGroup without timezones", func(t *testing.T) {
		memberCounts, nErr = ss.Channel().GetMemberCountsByGroup(context.Background(), c1.Id, false)
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     g1.Id,
				ChannelMemberCount:          1,
				ChannelMemberTimezonesCount: 0,
			},
		}
		require.NoError(t, nErr)
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	t.Run("returns memberCountsByGroup with timezones when no timezones set", func(t *testing.T) {
		memberCounts, nErr = ss.Channel().GetMemberCountsByGroup(context.Background(), c1.Id, true)
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     g1.Id,
				ChannelMemberCount:          1,
				ChannelMemberTimezonesCount: 0,
			},
		}
		require.NoError(t, nErr)
		require.Equal(t, expectedMemberCounts, memberCounts)
	})

	g2 := &model.Group{
		Name:        model.NewString(model.NewId()),
		DisplayName: model.NewId(),
		Source:      model.GroupSourceLdap,
		RemoteId:    model.NewId(),
	}
	_, err = ss.Group().Create(g2)
	require.NoError(t, err)

	// create 5 different users with 2 different timezones for group 2
	for i := 1; i <= 5; i++ {
		timeZone := timezones.DefaultUserTimezone()
		if i == 1 {
			timeZone["manualTimezone"] = "EDT"
			timeZone["useAutomaticTimezone"] = "false"
		}

		u := &model.User{
			Timezone: timeZone,
			Email:    MakeEmail(),
			DeleteAt: 0,
		}
		_, nErr = ss.User().Save(u)
		require.NoError(t, nErr)
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u.Id}, -1)
		require.NoError(t, nErr)

		m := model.ChannelMember{
			ChannelId:   c1.Id,
			UserId:      u.Id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		}
		_, nErr = ss.Channel().SaveMember(&m)
		require.NoError(t, nErr)

		_, err = ss.Group().UpsertMember(g2.Id, u.Id)
		require.NoError(t, err)
	}

	g3 := &model.Group{
		Name:        model.NewString(model.NewId()),
		DisplayName: model.NewId(),
		Source:      model.GroupSourceLdap,
		RemoteId:    model.NewId(),
	}

	_, err = ss.Group().Create(g3)
	require.NoError(t, err)

	// create 10 different users with 3 different timezones for group 3
	for i := 1; i <= 10; i++ {
		timeZone := timezones.DefaultUserTimezone()

		if i == 1 || i == 2 {
			timeZone["manualTimezone"] = "EDT"
			timeZone["useAutomaticTimezone"] = "false"
		} else if i == 3 || i == 4 {
			timeZone["manualTimezone"] = "PST"
			timeZone["useAutomaticTimezone"] = "false"
		} else if i == 5 || i == 6 {
			timeZone["autoTimezone"] = "PST"
			timeZone["useAutomaticTimezone"] = "true"
		} else {
			// Give every user with auto timezone set to true a random manual timezone to ensure that manual timezone is not looked at if auto is set
			timeZone["useAutomaticTimezone"] = "true"
			timeZone["manualTimezone"] = "PST" + utils.RandomName(utils.Range{Begin: 5, End: 5}, utils.ALPHANUMERIC)
		}

		u := &model.User{
			Timezone: timeZone,
			Email:    MakeEmail(),
			DeleteAt: 0,
		}
		_, nErr = ss.User().Save(u)
		require.NoError(t, nErr)
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u.Id}, -1)
		require.NoError(t, nErr)

		m := model.ChannelMember{
			ChannelId:   c1.Id,
			UserId:      u.Id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		}
		_, nErr = ss.Channel().SaveMember(&m)
		require.NoError(t, nErr)

		_, err = ss.Group().UpsertMember(g3.Id, u.Id)
		require.NoError(t, err)
	}

	t.Run("returns memberCountsByGroup for multiple groups with lots of users without timezones", func(t *testing.T) {
		memberCounts, nErr = ss.Channel().GetMemberCountsByGroup(context.Background(), c1.Id, false)
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     g1.Id,
				ChannelMemberCount:          1,
				ChannelMemberTimezonesCount: 0,
			},
			{
				GroupId:                     g2.Id,
				ChannelMemberCount:          5,
				ChannelMemberTimezonesCount: 0,
			},
			{
				GroupId:                     g3.Id,
				ChannelMemberCount:          10,
				ChannelMemberTimezonesCount: 0,
			},
		}
		require.NoError(t, nErr)
		require.ElementsMatch(t, expectedMemberCounts, memberCounts)
	})

	t.Run("returns memberCountsByGroup for multiple groups with lots of users with timezones", func(t *testing.T) {
		memberCounts, nErr = ss.Channel().GetMemberCountsByGroup(context.Background(), c1.Id, true)
		expectedMemberCounts := []*model.ChannelMemberCountByGroup{
			{
				GroupId:                     g1.Id,
				ChannelMemberCount:          1,
				ChannelMemberTimezonesCount: 0,
			},
			{
				GroupId:                     g2.Id,
				ChannelMemberCount:          5,
				ChannelMemberTimezonesCount: 1,
			},
			{
				GroupId:                     g3.Id,
				ChannelMemberCount:          10,
				ChannelMemberTimezonesCount: 3,
			},
		}
		require.NoError(t, nErr)
		require.ElementsMatch(t, expectedMemberCounts, memberCounts)
	})
}

func testGetGuestCount(t *testing.T, ss store.Store) {
	teamId := model.NewId()

	c1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	c2 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel2",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&c2, -1)
	require.NoError(t, nErr)

	t.Run("Regular member doesn't count", func(t *testing.T) {
		u1 := &model.User{
			Email:    MakeEmail(),
			DeleteAt: 0,
			Roles:    model.SYSTEM_USER_ROLE_ID,
		}
		_, err := ss.User().Save(u1)
		require.NoError(t, err)
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u1.Id}, -1)
		require.NoError(t, nErr)

		m1 := model.ChannelMember{
			ChannelId:   c1.Id,
			UserId:      u1.Id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
			SchemeGuest: false,
		}
		_, nErr = ss.Channel().SaveMember(&m1)
		require.NoError(t, nErr)

		count, channelErr := ss.Channel().GetGuestCount(c1.Id, false)
		require.NoError(t, channelErr)
		require.Equal(t, int64(0), count)
	})

	t.Run("Guest member does count", func(t *testing.T) {
		u2 := model.User{
			Email:    MakeEmail(),
			DeleteAt: 0,
			Roles:    model.SYSTEM_GUEST_ROLE_ID,
		}
		_, err := ss.User().Save(&u2)
		require.NoError(t, err)
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u2.Id}, -1)
		require.NoError(t, nErr)

		m2 := model.ChannelMember{
			ChannelId:   c1.Id,
			UserId:      u2.Id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
			SchemeGuest: true,
		}
		_, nErr = ss.Channel().SaveMember(&m2)
		require.NoError(t, nErr)

		count, channelErr := ss.Channel().GetGuestCount(c1.Id, false)
		require.NoError(t, channelErr)
		require.Equal(t, int64(1), count)
	})

	t.Run("make sure members of other channels aren't counted", func(t *testing.T) {
		u3 := model.User{
			Email:    MakeEmail(),
			DeleteAt: 0,
			Roles:    model.SYSTEM_GUEST_ROLE_ID,
		}
		_, err := ss.User().Save(&u3)
		require.NoError(t, err)
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u3.Id}, -1)
		require.NoError(t, nErr)

		m3 := model.ChannelMember{
			ChannelId:   c2.Id,
			UserId:      u3.Id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
			SchemeGuest: true,
		}
		_, nErr = ss.Channel().SaveMember(&m3)
		require.NoError(t, nErr)

		count, channelErr := ss.Channel().GetGuestCount(c1.Id, false)
		require.NoError(t, channelErr)
		require.Equal(t, int64(1), count)
	})

	t.Run("make sure inactive users aren't counted", func(t *testing.T) {
		u4 := &model.User{
			Email:    MakeEmail(),
			DeleteAt: 10000,
			Roles:    model.SYSTEM_GUEST_ROLE_ID,
		}
		_, err := ss.User().Save(u4)
		require.NoError(t, err)
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: teamId, UserId: u4.Id}, -1)
		require.NoError(t, nErr)

		m4 := model.ChannelMember{
			ChannelId:   c1.Id,
			UserId:      u4.Id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
			SchemeGuest: true,
		}
		_, nErr = ss.Channel().SaveMember(&m4)
		require.NoError(t, nErr)

		count, channelErr := ss.Channel().GetGuestCount(c1.Id, false)
		require.NoError(t, channelErr)
		require.Equal(t, int64(1), count)
	})
}

func testChannelStoreSearchMore(t *testing.T, ss store.Store) {
	teamId := model.NewId()
	otherTeamId := model.NewId()

	o1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelA",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err := ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	o2 := model.Channel{
		TeamId:      otherTeamId,
		DisplayName: "Channel2",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	m3 := model.ChannelMember{
		ChannelId:   o2.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m3)
	require.NoError(t, err)

	o3 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelA",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	o4 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelB",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o4, -1)
	require.NoError(t, nErr)

	o5 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelC",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o5, -1)
	require.NoError(t, nErr)

	o6 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Off-Topic",
		Name:        "off-topic",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o6, -1)
	require.NoError(t, nErr)

	o7 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Off-Set",
		Name:        "off-set",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o7, -1)
	require.NoError(t, nErr)

	o8 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Off-Limit",
		Name:        "off-limit",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o8, -1)
	require.NoError(t, nErr)

	o9 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel With Purpose",
		Purpose:     "This can now be searchable!",
		Name:        "with-purpose",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o9, -1)
	require.NoError(t, nErr)

	o10 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelA",
		Name:        "channel-a-deleted",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o10, -1)
	require.NoError(t, nErr)

	o10.DeleteAt = model.GetMillis()
	o10.UpdateAt = o10.DeleteAt
	nErr = ss.Channel().Delete(o10.Id, o10.DeleteAt)
	require.NoError(t, nErr, "channel should have been deleted")

	t.Run("three public channels matching 'ChannelA', but already a member of one and one deleted", func(t *testing.T) {
		channels, err := ss.Channel().SearchMore(m1.UserId, teamId, "ChannelA")
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o3}, channels)
	})

	t.Run("one public channels, but already a member", func(t *testing.T) {
		channels, err := ss.Channel().SearchMore(m1.UserId, teamId, o4.Name)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{}, channels)
	})

	t.Run("three matching channels, but only two public", func(t *testing.T) {
		channels, err := ss.Channel().SearchMore(m1.UserId, teamId, "off-")
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o7, &o6}, channels)
	})

	t.Run("one channel matching 'off-topic'", func(t *testing.T) {
		channels, err := ss.Channel().SearchMore(m1.UserId, teamId, "off-topic")
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o6}, channels)
	})

	t.Run("search purpose", func(t *testing.T) {
		channels, err := ss.Channel().SearchMore(m1.UserId, teamId, "now searchable")
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o9}, channels)
	})
}

type ByChannelDisplayName model.ChannelList

func (s ByChannelDisplayName) Len() int { return len(s) }
func (s ByChannelDisplayName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ByChannelDisplayName) Less(i, j int) bool {
	if s[i].DisplayName != s[j].DisplayName {
		return s[i].DisplayName < s[j].DisplayName
	}

	return s[i].Id < s[j].Id
}

func testChannelStoreSearchArchivedInTeam(t *testing.T, ss store.Store, s SqlStore) {
	teamId := model.NewId()
	userId := model.NewId()

	t.Run("empty result", func(t *testing.T) {
		list, err := ss.Channel().SearchArchivedInTeam(teamId, "term", userId)
		require.NoError(t, err)
		require.NotNil(t, list)
		require.Empty(t, list)
	})

	t.Run("error", func(t *testing.T) {
		// trigger a SQL error
		s.GetMaster().Exec("ALTER TABLE Channels RENAME TO Channels_renamed")
		defer s.GetMaster().Exec("ALTER TABLE Channels_renamed RENAME TO Channels")

		list, err := ss.Channel().SearchArchivedInTeam(teamId, "term", userId)
		require.Error(t, err)
		require.Nil(t, list)
	})
}

func testChannelStoreSearchInTeam(t *testing.T, ss store.Store) {
	teamId := model.NewId()
	otherTeamId := model.NewId()

	o1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelA",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{
		TeamId:      otherTeamId,
		DisplayName: "ChannelA",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err := ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	m3 := model.ChannelMember{
		ChannelId:   o2.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m3)
	require.NoError(t, err)

	o3 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelA (alternate)",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	o4 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel B",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o4, -1)
	require.NoError(t, nErr)

	m4 := model.ChannelMember{
		ChannelId:   o4.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m4)
	require.NoError(t, err)

	o5 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Channel C",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o5, -1)
	require.NoError(t, nErr)

	o6 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Off-Topic",
		Name:        "off-topic",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o6, -1)
	require.NoError(t, nErr)

	o7 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Off-Set",
		Name:        "off-set",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o7, -1)
	require.NoError(t, nErr)

	o8 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Off-Limit",
		Name:        "off-limit",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o8, -1)
	require.NoError(t, nErr)

	m5 := model.ChannelMember{
		ChannelId:   o8.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m5)
	require.NoError(t, err)

	o9 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Town Square",
		Name:        "town-square",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o9, -1)
	require.NoError(t, nErr)

	o10 := model.Channel{
		TeamId:      teamId,
		DisplayName: "The",
		Name:        "thename",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o10, -1)
	require.NoError(t, nErr)

	o11 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Native Mobile Apps",
		Name:        "native-mobile-apps",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o11, -1)
	require.NoError(t, nErr)

	o12 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelZ",
		Purpose:     "This can now be searchable!",
		Name:        "with-purpose",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o12, -1)
	require.NoError(t, nErr)

	o13 := model.Channel{
		TeamId:      teamId,
		DisplayName: "ChannelA (deleted)",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o13, -1)
	require.NoError(t, nErr)
	o13.DeleteAt = model.GetMillis()
	o13.UpdateAt = o13.DeleteAt
	nErr = ss.Channel().Delete(o13.Id, o13.DeleteAt)
	require.NoError(t, nErr, "channel should have been deleted")

	testCasesSearchInTeam := []struct {
		Description     string
		TeamId          string
		Term            string
		IncludeDeleted  bool
		ExpectedResults *model.ChannelList
	}{
		{"ChannelA", teamId, "ChannelA", false, &model.ChannelList{&o1, &o3}},
		{"ChannelA, include deleted", teamId, "ChannelA", true, &model.ChannelList{&o1, &o3, &o13}},
		{"ChannelA, other team", otherTeamId, "ChannelA", false, &model.ChannelList{&o2}},
		{"empty string", teamId, "", false, &model.ChannelList{&o1, &o3, &o12, &o11, &o7, &o6, &o10, &o9}},
		{"no matches", teamId, "blargh", false, &model.ChannelList{}},
		{"prefix", teamId, "off-", false, &model.ChannelList{&o7, &o6}},
		{"full match with dash", teamId, "off-topic", false, &model.ChannelList{&o6}},
		{"town square", teamId, "town square", false, &model.ChannelList{&o9}},
		{"the in name", teamId, "thename", false, &model.ChannelList{&o10}},
		{"Mobile", teamId, "Mobile", false, &model.ChannelList{&o11}},
		{"search purpose", teamId, "now searchable", false, &model.ChannelList{&o12}},
		{"pipe ignored", teamId, "town square |", false, &model.ChannelList{&o9}},
	}

	for _, testCase := range testCasesSearchInTeam {
		t.Run("SearchInTeam/"+testCase.Description, func(t *testing.T) {
			channels, err := ss.Channel().SearchInTeam(testCase.TeamId, testCase.Term, testCase.IncludeDeleted)
			require.NoError(t, err)

			require.Equal(t, testCase.ExpectedResults, channels)
		})
	}

	testCasesAutocompleteInTeam := []struct {
		Description     string
		TeamId          string
		UserId          string
		Term            string
		IncludeDeleted  bool
		ExpectedResults *model.ChannelList
	}{
		{"ChannelA", teamId, m1.UserId, "ChannelA", false, &model.ChannelList{&o1, &o3}},
		{"ChannelA, include deleted", teamId, m1.UserId, "ChannelA", true, &model.ChannelList{&o1, &o3, &o13}},
		{"ChannelA, other team", otherTeamId, m3.UserId, "ChannelA", false, &model.ChannelList{&o2}},
		{"empty string", teamId, m1.UserId, "", false, &model.ChannelList{&o1, &o3, &o12, &o11, &o7, &o6, &o10, &o9}},
		{"no matches", teamId, m1.UserId, "blargh", false, &model.ChannelList{}},
		{"prefix", teamId, m1.UserId, "off-", false, &model.ChannelList{&o7, &o6}},
		{"full match with dash", teamId, m1.UserId, "off-topic", false, &model.ChannelList{&o6}},
		{"town square", teamId, m1.UserId, "town square", false, &model.ChannelList{&o9}},
		{"the in name", teamId, m1.UserId, "thename", false, &model.ChannelList{&o10}},
		{"Mobile", teamId, m1.UserId, "Mobile", false, &model.ChannelList{&o11}},
		{"search purpose", teamId, m1.UserId, "now searchable", false, &model.ChannelList{&o12}},
		{"pipe ignored", teamId, m1.UserId, "town square |", false, &model.ChannelList{&o9}},
		{"Channel B (private)", teamId, m4.UserId, "Channel B", false, &model.ChannelList{&o4}},
		{"off limit (private)", teamId, m5.UserId, "off limit", false, &model.ChannelList{&o8}},
	}

	for _, testCase := range testCasesAutocompleteInTeam {
		t.Run("AutocompleteInTeam/"+testCase.Description, func(t *testing.T) {
			channels, err := ss.Channel().AutocompleteInTeam(testCase.TeamId, testCase.UserId, testCase.Term, testCase.IncludeDeleted)
			require.NoError(t, err)

			require.Equal(t, testCase.ExpectedResults, channels)
		})
	}

}

func testChannelStoreSearchForUserInTeam(t *testing.T, ss store.Store) {
	userId := model.NewId()
	teamId := model.NewId()
	otherTeamId := model.NewId()

	// create 4 channels for the same team and one for other team
	o1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "test-dev-1",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{
		TeamId:      teamId,
		DisplayName: "test-dev-2",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	o3 := model.Channel{
		TeamId:      teamId,
		DisplayName: "dev-3",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	o4 := model.Channel{
		TeamId:      teamId,
		DisplayName: "dev-4",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o4, -1)
	require.NoError(t, nErr)

	o5 := model.Channel{
		TeamId:      otherTeamId,
		DisplayName: "other-team-dev-5",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o5, -1)
	require.NoError(t, nErr)

	// add the user to the first 3 channels and the other team channel
	for _, c := range []model.Channel{o1, o2, o3, o5} {
		_, err := ss.Channel().SaveMember(&model.ChannelMember{
			ChannelId:   c.Id,
			UserId:      userId,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, err)
	}

	searchAndCheck := func(t *testing.T, term string, includeDeleted bool, expectedDisplayNames []string) {
		res, searchErr := ss.Channel().SearchForUserInTeam(userId, teamId, term, includeDeleted)
		require.NoError(t, searchErr)
		require.Len(t, *res, len(expectedDisplayNames))

		resultDisplayNames := []string{}
		for _, c := range *res {
			resultDisplayNames = append(resultDisplayNames, c.DisplayName)
		}
		require.ElementsMatch(t, expectedDisplayNames, resultDisplayNames)
	}

	t.Run("Search for test, get channels 1 and 2", func(t *testing.T) {
		searchAndCheck(t, "test", false, []string{o1.DisplayName, o2.DisplayName})
	})

	t.Run("Search for dev, get channels 1, 2 and 3", func(t *testing.T) {
		searchAndCheck(t, "dev", false, []string{o1.DisplayName, o2.DisplayName, o3.DisplayName})
	})

	t.Run("After adding user to channel 4, search for dev, get channels 1, 2, 3 and 4", func(t *testing.T) {
		_, err := ss.Channel().SaveMember(&model.ChannelMember{
			ChannelId:   o4.Id,
			UserId:      userId,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, err)

		searchAndCheck(t, "dev", false, []string{o1.DisplayName, o2.DisplayName, o3.DisplayName, o4.DisplayName})
	})

	t.Run("Mark channel 1 as deleted, search for dev, get channels 2, 3 and 4", func(t *testing.T) {
		o1.DeleteAt = model.GetMillis()
		o1.UpdateAt = o1.DeleteAt
		err := ss.Channel().Delete(o1.Id, o1.DeleteAt)
		require.NoError(t, err)

		searchAndCheck(t, "dev", false, []string{o2.DisplayName, o3.DisplayName, o4.DisplayName})
	})

	t.Run("With includeDeleted, search for dev, get channels 1, 2, 3 and 4", func(t *testing.T) {
		searchAndCheck(t, "dev", true, []string{o1.DisplayName, o2.DisplayName, o3.DisplayName, o4.DisplayName})
	})
}

func testChannelStoreSearchAllChannels(t *testing.T, ss store.Store) {
	cleanupChannels(t, ss)

	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewId()
	t1.Email = MakeEmail()
	t1.Type = model.TEAM_OPEN
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	t2 := model.Team{}
	t2.DisplayName = "Name2"
	t2.Name = "zz" + model.NewId()
	t2.Email = MakeEmail()
	t2.Type = model.TEAM_OPEN
	_, err = ss.Team().Save(&t2)
	require.NoError(t, err)

	o1 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "A1 ChannelA",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{
		TeamId:      t2.Id,
		DisplayName: "A2 ChannelA",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{
		ChannelId:   o1.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	m3 := model.ChannelMember{
		ChannelId:   o2.Id,
		UserId:      model.NewId(),
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(&m3)
	require.NoError(t, err)

	o3 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "A3 ChannelA (alternate)",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	o4 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "A4 ChannelB",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o4, -1)
	require.NoError(t, nErr)

	o5 := model.Channel{
		TeamId:           t1.Id,
		DisplayName:      "A5 ChannelC",
		Name:             "zz" + model.NewId() + "b",
		Type:             model.CHANNEL_PRIVATE,
		GroupConstrained: model.NewBool(true),
	}
	_, nErr = ss.Channel().Save(&o5, -1)
	require.NoError(t, nErr)

	o6 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "A6 Off-Topic",
		Name:        "off-topic",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o6, -1)
	require.NoError(t, nErr)

	o7 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "A7 Off-Set",
		Name:        "off-set",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o7, -1)
	require.NoError(t, nErr)

	group := &model.Group{
		Name:        model.NewString(model.NewId()),
		DisplayName: model.NewId(),
		Source:      model.GroupSourceLdap,
		RemoteId:    model.NewId(),
	}
	_, err = ss.Group().Create(group)
	require.NoError(t, err)

	_, err = ss.Group().CreateGroupSyncable(model.NewGroupChannel(group.Id, o7.Id, true))
	require.NoError(t, err)

	o8 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "A8 Off-Limit",
		Name:        "off-limit",
		Type:        model.CHANNEL_PRIVATE,
	}
	_, nErr = ss.Channel().Save(&o8, -1)
	require.NoError(t, nErr)

	o9 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "A9 Town Square",
		Name:        "town-square",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o9, -1)
	require.NoError(t, nErr)

	o10 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "B10 Which",
		Name:        "which",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o10, -1)
	require.NoError(t, nErr)

	o11 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "B11 Native Mobile Apps",
		Name:        "native-mobile-apps",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o11, -1)
	require.NoError(t, nErr)

	o12 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "B12 ChannelZ",
		Purpose:     "This can now be searchable!",
		Name:        "with-purpose",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o12, -1)
	require.NoError(t, nErr)

	o13 := model.Channel{
		TeamId:      t1.Id,
		DisplayName: "B13 ChannelA (deleted)",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o13, -1)
	require.NoError(t, nErr)

	o13.DeleteAt = model.GetMillis()
	o13.UpdateAt = o13.DeleteAt
	nErr = ss.Channel().Delete(o13.Id, o13.DeleteAt)
	require.NoError(t, nErr, "channel should have been deleted")

	o14 := model.Channel{
		TeamId:      t2.Id,
		DisplayName: "B14 FOOBARDISPLAYNAME",
		Name:        "whatever",
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o14, -1)
	require.NoError(t, nErr)

	_, nErr = ss.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:  "Policy 1",
			PostDuration: model.NewInt64(30),
		},
		ChannelIDs: []string{o14.Id},
	})
	require.NoError(t, nErr)

	testCases := []struct {
		Description     string
		Term            string
		Opts            store.ChannelSearchOpts
		ExpectedResults *model.ChannelList
		TotalCount      int
	}{
		{"Search FooBar by display name", "bardisplay", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o14}, 1},
		{"Search FooBar by display name2", "foobar", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o14}, 1},
		{"Search FooBar by display name3", "displayname", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o14}, 1},
		{"Search FooBar by name", "what", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o14}, 1},
		{"Search FooBar by name2", "ever", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o14}, 1},
		{"ChannelA", "ChannelA", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o1, &o2, &o3}, 0},
		{"ChannelA, include deleted", "ChannelA", store.ChannelSearchOpts{IncludeDeleted: true}, &model.ChannelList{&o1, &o2, &o3, &o13}, 0},
		{"empty string", "", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o1, &o2, &o3, &o4, &o5, &o6, &o7, &o8, &o9, &o10, &o11, &o12, &o14}, 0},
		{"no matches", "blargh", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{}, 0},
		{"prefix", "off-", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o6, &o7, &o8}, 0},
		{"full match with dash", "off-topic", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o6}, 0},
		{"town square", "town square", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o9}, 0},
		{"which in name", "which", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o10}, 0},
		{"Mobile", "Mobile", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o11}, 0},
		{"search purpose", "now searchable", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o12}, 0},
		{"pipe ignored", "town square |", store.ChannelSearchOpts{IncludeDeleted: false}, &model.ChannelList{&o9}, 0},
		{"exclude defaults search 'off'", "off-", store.ChannelSearchOpts{IncludeDeleted: false, ExcludeChannelNames: []string{"off-topic"}}, &model.ChannelList{&o7, &o8}, 0},
		{"exclude defaults search 'town'", "town", store.ChannelSearchOpts{IncludeDeleted: false, ExcludeChannelNames: []string{"town-square"}}, &model.ChannelList{}, 0},
		{"exclude by group association", "off-", store.ChannelSearchOpts{IncludeDeleted: false, NotAssociatedToGroup: group.Id}, &model.ChannelList{&o6, &o8}, 0},
		{"paginate includes count", "off-", store.ChannelSearchOpts{IncludeDeleted: false, PerPage: model.NewInt(100)}, &model.ChannelList{&o6, &o7, &o8}, 3},
		{"paginate, page 2 correct entries and count", "off-", store.ChannelSearchOpts{IncludeDeleted: false, PerPage: model.NewInt(2), Page: model.NewInt(1)}, &model.ChannelList{&o8}, 3},
		{"Filter private", "", store.ChannelSearchOpts{IncludeDeleted: false, Private: true}, &model.ChannelList{&o4, &o5, &o8}, 3},
		{"Filter public", "", store.ChannelSearchOpts{IncludeDeleted: false, Public: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o1, &o2, &o3, &o6, &o7}, 10},
		{"Filter public and private", "", store.ChannelSearchOpts{IncludeDeleted: false, Public: true, Private: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o1, &o2, &o3, &o4, &o5}, 13},
		{"Filter public and private and include deleted", "", store.ChannelSearchOpts{IncludeDeleted: true, Public: true, Private: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o1, &o2, &o3, &o4, &o5}, 14},
		{"Filter group constrained", "", store.ChannelSearchOpts{IncludeDeleted: false, GroupConstrained: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o5}, 1},
		{"Filter exclude group constrained and include deleted", "", store.ChannelSearchOpts{IncludeDeleted: true, ExcludeGroupConstrained: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o1, &o2, &o3, &o4, &o6}, 13},
		{"Filter private and exclude group constrained", "", store.ChannelSearchOpts{IncludeDeleted: false, ExcludeGroupConstrained: true, Private: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o4, &o8}, 2},
		{"Exclude policy constrained", "", store.ChannelSearchOpts{ExcludePolicyConstrained: true}, &model.ChannelList{&o1, &o2, &o3, &o4, &o5, &o6, &o7, &o8, &o9, &o10, &o11, &o12}, 0},
		{"Filter team 2", "", store.ChannelSearchOpts{IncludeDeleted: false, TeamIds: []string{t2.Id}, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o2, &o14}, 2},
		{"Filter team 2, private", "", store.ChannelSearchOpts{IncludeDeleted: false, TeamIds: []string{t2.Id}, Private: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{}, 0},
		{"Filter team 1 and team 2, private", "", store.ChannelSearchOpts{IncludeDeleted: false, TeamIds: []string{t1.Id, t2.Id}, Private: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o4, &o5, &o8}, 3},
		{"Filter team 1 and team 2, public and private", "", store.ChannelSearchOpts{IncludeDeleted: false, TeamIds: []string{t1.Id, t2.Id}, Public: true, Private: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o1, &o2, &o3, &o4, &o5}, 13},
		{"Filter team 1 and team 2, public and private and group constrained", "", store.ChannelSearchOpts{IncludeDeleted: false, TeamIds: []string{t1.Id, t2.Id}, Public: true, Private: true, GroupConstrained: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o5}, 1},
		{"Filter team 1 and team 2, public and private and exclude group constrained", "", store.ChannelSearchOpts{IncludeDeleted: false, TeamIds: []string{t1.Id, t2.Id}, Public: true, Private: true, ExcludeGroupConstrained: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o1, &o2, &o3, &o4, &o6}, 12},
		{"Filter deleted returns only deleted channels", "", store.ChannelSearchOpts{Deleted: true, Page: model.NewInt(0), PerPage: model.NewInt(5)}, &model.ChannelList{&o13}, 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			channels, count, err := ss.Channel().SearchAllChannels(testCase.Term, testCase.Opts)
			require.NoError(t, err)
			require.Equal(t, len(*testCase.ExpectedResults), len(*channels))
			for i, expected := range *testCase.ExpectedResults {
				require.Equal(t, expected.Id, (*channels)[i].Id)
			}
			if testCase.Opts.Page != nil || testCase.Opts.PerPage != nil {
				require.Equal(t, int64(testCase.TotalCount), count)
			}
		})
	}
}

func testChannelStoreGetMembersByIds(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "ChannelA"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	m1 := &model.ChannelMember{ChannelId: o1.Id, UserId: model.NewId(), NotifyProps: model.GetDefaultChannelNotifyProps()}
	_, err := ss.Channel().SaveMember(m1)
	require.NoError(t, err)

	var members *model.ChannelMembers
	members, nErr = ss.Channel().GetMembersByIds(m1.ChannelId, []string{m1.UserId})
	require.NoError(t, nErr, nErr)
	rm1 := (*members)[0]

	require.Equal(t, m1.ChannelId, rm1.ChannelId, "bad team id")
	require.Equal(t, m1.UserId, rm1.UserId, "bad user id")

	m2 := &model.ChannelMember{ChannelId: o1.Id, UserId: model.NewId(), NotifyProps: model.GetDefaultChannelNotifyProps()}
	_, err = ss.Channel().SaveMember(m2)
	require.NoError(t, err)

	members, nErr = ss.Channel().GetMembersByIds(m1.ChannelId, []string{m1.UserId, m2.UserId, model.NewId()})
	require.NoError(t, nErr, nErr)
	require.Len(t, *members, 2, "return wrong number of results")

	_, nErr = ss.Channel().GetMembersByIds(m1.ChannelId, []string{})
	require.Error(t, nErr, "empty user ids - should have failed")
}

func testChannelStoreGetMembersByChannelIds(t *testing.T, ss store.Store) {
	userId := model.NewId()

	// Create a couple channels and add the user to them
	channel1, err := ss.Channel().Save(&model.Channel{
		TeamId:      model.NewId(),
		DisplayName: model.NewId(),
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}, -1)
	require.NoError(t, err)

	channel2, err := ss.Channel().Save(&model.Channel{
		TeamId:      model.NewId(),
		DisplayName: model.NewId(),
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}, -1)
	require.NoError(t, err)

	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelId:   channel1.Id,
		UserId:      userId,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelId:   channel2.Id,
		UserId:      userId,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	t.Run("should return the user's members for the given channels", func(t *testing.T) {
		result, nErr := ss.Channel().GetMembersByChannelIds([]string{channel1.Id, channel2.Id}, userId)
		require.NoError(t, nErr)
		assert.Len(t, *result, 2)

		assert.Equal(t, userId, (*result)[0].UserId)
		assert.True(t, (*result)[0].ChannelId == channel1.Id || (*result)[1].ChannelId == channel1.Id)
		assert.Equal(t, userId, (*result)[1].UserId)
		assert.True(t, (*result)[0].ChannelId == channel2.Id || (*result)[1].ChannelId == channel2.Id)
	})

	t.Run("should not error or return anything for invalid channel IDs", func(t *testing.T) {
		result, nErr := ss.Channel().GetMembersByChannelIds([]string{model.NewId(), model.NewId()}, userId)
		require.NoError(t, nErr)
		assert.Len(t, *result, 0)
	})

	t.Run("should not error or return anything for invalid user IDs", func(t *testing.T) {
		result, nErr := ss.Channel().GetMembersByChannelIds([]string{channel1.Id, channel2.Id}, model.NewId())
		require.NoError(t, nErr)
		assert.Len(t, *result, 0)
	})
}

func testChannelStoreSearchGroupChannels(t *testing.T, ss store.Store) {
	// Users
	u1 := &model.User{}
	u1.Username = "user.one"
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)

	u2 := &model.User{}
	u2.Username = "user.two"
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)

	u3 := &model.User{}
	u3.Username = "user.three"
	u3.Email = MakeEmail()
	u3.Nickname = model.NewId()
	_, err = ss.User().Save(u3)
	require.NoError(t, err)

	u4 := &model.User{}
	u4.Username = "user.four"
	u4.Email = MakeEmail()
	u4.Nickname = model.NewId()
	_, err = ss.User().Save(u4)
	require.NoError(t, err)

	// Group channels
	userIds := []string{u1.Id, u2.Id, u3.Id}
	gc1 := model.Channel{}
	gc1.Name = model.GetGroupNameFromUserIds(userIds)
	gc1.DisplayName = "GroupChannel" + model.NewId()
	gc1.Type = model.CHANNEL_GROUP
	_, nErr := ss.Channel().Save(&gc1, -1)
	require.NoError(t, nErr)

	for _, userId := range userIds {
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
			ChannelId:   gc1.Id,
			UserId:      userId,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, nErr)
	}

	userIds = []string{u1.Id, u4.Id}
	gc2 := model.Channel{}
	gc2.Name = model.GetGroupNameFromUserIds(userIds)
	gc2.DisplayName = "GroupChannel" + model.NewId()
	gc2.Type = model.CHANNEL_GROUP
	_, nErr = ss.Channel().Save(&gc2, -1)
	require.NoError(t, nErr)

	for _, userId := range userIds {
		_, err := ss.Channel().SaveMember(&model.ChannelMember{
			ChannelId:   gc2.Id,
			UserId:      userId,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, err)
	}

	userIds = []string{u1.Id, u2.Id, u3.Id, u4.Id}
	gc3 := model.Channel{}
	gc3.Name = model.GetGroupNameFromUserIds(userIds)
	gc3.DisplayName = "GroupChannel" + model.NewId()
	gc3.Type = model.CHANNEL_GROUP
	_, nErr = ss.Channel().Save(&gc3, -1)
	require.NoError(t, nErr)

	for _, userId := range userIds {
		_, err := ss.Channel().SaveMember(&model.ChannelMember{
			ChannelId:   gc3.Id,
			UserId:      userId,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, err)
	}

	defer func() {
		for _, gc := range []model.Channel{gc1, gc2, gc3} {
			ss.Channel().PermanentDeleteMembersByChannel(gc3.Id)
			ss.Channel().PermanentDelete(gc.Id)
		}
	}()

	testCases := []struct {
		Name           string
		UserId         string
		Term           string
		ExpectedResult []string
	}{
		{
			Name:           "Get all group channels for user1",
			UserId:         u1.Id,
			Term:           "",
			ExpectedResult: []string{gc1.Id, gc2.Id, gc3.Id},
		},
		{
			Name:           "Get group channels for user1 and term 'three'",
			UserId:         u1.Id,
			Term:           "three",
			ExpectedResult: []string{gc1.Id, gc3.Id},
		},
		{
			Name:           "Get group channels for user1 and term 'four two'",
			UserId:         u1.Id,
			Term:           "four two",
			ExpectedResult: []string{gc3.Id},
		},
		{
			Name:           "Get all group channels for user2",
			UserId:         u2.Id,
			Term:           "",
			ExpectedResult: []string{gc1.Id, gc3.Id},
		},
		{
			Name:           "Get group channels for user2 and term 'four'",
			UserId:         u2.Id,
			Term:           "four",
			ExpectedResult: []string{gc3.Id},
		},
		{
			Name:           "Get all group channels for user4",
			UserId:         u4.Id,
			Term:           "",
			ExpectedResult: []string{gc2.Id, gc3.Id},
		},
		{
			Name:           "Get group channels for user4 and term 'one five'",
			UserId:         u4.Id,
			Term:           "one five",
			ExpectedResult: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := ss.Channel().SearchGroupChannels(tc.UserId, tc.Term)
			require.NoError(t, err)

			resultIds := []string{}
			for _, gc := range *result {
				resultIds = append(resultIds, gc.Id)
			}

			require.ElementsMatch(t, tc.ExpectedResult, resultIds)
		})
	}
}

func testChannelStoreAnalyticsDeletedTypeCount(t *testing.T, ss store.Store) {
	o1 := model.Channel{}
	o1.TeamId = model.NewId()
	o1.DisplayName = "ChannelA"
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	o2 := model.Channel{}
	o2.TeamId = model.NewId()
	o2.DisplayName = "Channel2"
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	p3 := model.Channel{}
	p3.TeamId = model.NewId()
	p3.DisplayName = "Channel3"
	p3.Name = "zz" + model.NewId() + "b"
	p3.Type = model.CHANNEL_PRIVATE
	_, nErr = ss.Channel().Save(&p3, -1)
	require.NoError(t, nErr)

	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)

	d4, nErr := ss.Channel().CreateDirectChannel(u1, u2)
	require.NoError(t, nErr)
	defer func() {
		ss.Channel().PermanentDeleteMembersByChannel(d4.Id)
		ss.Channel().PermanentDelete(d4.Id)
	}()

	var openStartCount int64
	openStartCount, nErr = ss.Channel().AnalyticsDeletedTypeCount("", "O")
	require.NoError(t, nErr, nErr)

	var privateStartCount int64
	privateStartCount, nErr = ss.Channel().AnalyticsDeletedTypeCount("", "P")
	require.NoError(t, nErr, nErr)

	var directStartCount int64
	directStartCount, nErr = ss.Channel().AnalyticsDeletedTypeCount("", "D")
	require.NoError(t, nErr, nErr)

	nErr = ss.Channel().Delete(o1.Id, model.GetMillis())
	require.NoError(t, nErr, "channel should have been deleted")
	nErr = ss.Channel().Delete(o2.Id, model.GetMillis())
	require.NoError(t, nErr, "channel should have been deleted")
	nErr = ss.Channel().Delete(p3.Id, model.GetMillis())
	require.NoError(t, nErr, "channel should have been deleted")
	nErr = ss.Channel().Delete(d4.Id, model.GetMillis())
	require.NoError(t, nErr, "channel should have been deleted")

	var count int64

	count, nErr = ss.Channel().AnalyticsDeletedTypeCount("", "O")
	require.NoError(t, err, nErr)
	assert.Equal(t, openStartCount+2, count, "Wrong open channel deleted count.")

	count, nErr = ss.Channel().AnalyticsDeletedTypeCount("", "P")
	require.NoError(t, nErr, nErr)
	assert.Equal(t, privateStartCount+1, count, "Wrong private channel deleted count.")

	count, nErr = ss.Channel().AnalyticsDeletedTypeCount("", "D")
	require.NoError(t, nErr, nErr)
	assert.Equal(t, directStartCount+1, count, "Wrong direct channel deleted count.")
}

func testChannelStoreGetPinnedPosts(t *testing.T, ss store.Store) {
	ch1 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}

	o1, nErr := ss.Channel().Save(ch1, -1)
	require.NoError(t, nErr)

	p1, err := ss.Post().Save(&model.Post{
		UserId:    model.NewId(),
		ChannelId: o1.Id,
		Message:   "test",
		IsPinned:  true,
	})
	require.NoError(t, err)

	pl, errGet := ss.Channel().GetPinnedPosts(o1.Id)
	require.NoError(t, errGet, errGet)
	require.NotNil(t, pl.Posts[p1.Id], "didn't return relevant pinned posts")

	ch2 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}

	o2, nErr := ss.Channel().Save(ch2, -1)
	require.NoError(t, nErr)

	_, err = ss.Post().Save(&model.Post{
		UserId:    model.NewId(),
		ChannelId: o2.Id,
		Message:   "test",
	})
	require.NoError(t, err)

	pl, errGet = ss.Channel().GetPinnedPosts(o2.Id)
	require.NoError(t, errGet, errGet)
	require.Empty(t, pl.Posts, "wasn't supposed to return posts")

	t.Run("with correct ReplyCount", func(t *testing.T) {
		channelId := model.NewId()
		userId := model.NewId()

		post1, err := ss.Post().Save(&model.Post{
			ChannelId: channelId,
			UserId:    userId,
			Message:   "message",
			IsPinned:  true,
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post2, err := ss.Post().Save(&model.Post{
			ChannelId: channelId,
			UserId:    userId,
			Message:   "message",
			IsPinned:  true,
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post3, err := ss.Post().Save(&model.Post{
			ChannelId: channelId,
			UserId:    userId,
			ParentId:  post1.Id,
			RootId:    post1.Id,
			Message:   "message",
			IsPinned:  true,
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		posts, err := ss.Channel().GetPinnedPosts(channelId)
		require.NoError(t, err)
		require.Len(t, posts.Posts, 3)
		require.Equal(t, posts.Posts[post1.Id].ReplyCount, int64(1))
		require.Equal(t, posts.Posts[post2.Id].ReplyCount, int64(0))
		require.Equal(t, posts.Posts[post3.Id].ReplyCount, int64(1))
	})
}

func testChannelStoreGetPinnedPostCount(t *testing.T, ss store.Store) {
	ch1 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}

	o1, nErr := ss.Channel().Save(ch1, -1)
	require.NoError(t, nErr)

	_, err := ss.Post().Save(&model.Post{
		UserId:    model.NewId(),
		ChannelId: o1.Id,
		Message:   "test",
		IsPinned:  true,
	})
	require.NoError(t, err)

	_, err = ss.Post().Save(&model.Post{
		UserId:    model.NewId(),
		ChannelId: o1.Id,
		Message:   "test",
		IsPinned:  true,
	})
	require.NoError(t, err)

	count, errGet := ss.Channel().GetPinnedPostCount(o1.Id, true)
	require.NoError(t, errGet, errGet)
	require.EqualValues(t, 2, count, "didn't return right count")

	ch2 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        "zz" + model.NewId() + "b",
		Type:        model.CHANNEL_OPEN,
	}

	o2, nErr := ss.Channel().Save(ch2, -1)
	require.NoError(t, nErr)

	_, err = ss.Post().Save(&model.Post{
		UserId:    model.NewId(),
		ChannelId: o2.Id,
		Message:   "test",
	})
	require.NoError(t, err)

	_, err = ss.Post().Save(&model.Post{
		UserId:    model.NewId(),
		ChannelId: o2.Id,
		Message:   "test",
	})
	require.NoError(t, err)

	count, errGet = ss.Channel().GetPinnedPostCount(o2.Id, true)
	require.NoError(t, errGet, errGet)
	require.EqualValues(t, 0, count, "should return 0")
}

func testChannelStoreMaxChannelsPerTeam(t *testing.T, ss store.Store) {
	channel := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Channel",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(channel, 0)
	assert.Error(t, nErr)
	var ltErr *store.ErrLimitExceeded
	assert.True(t, errors.As(nErr, &ltErr))

	channel.Id = ""
	_, nErr = ss.Channel().Save(channel, 1)
	assert.NoError(t, nErr)
}

func testChannelStoreGetChannelsByScheme(t *testing.T, ss store.Store) {
	// Create some schemes.
	s1 := &model.Scheme{
		DisplayName: model.NewId(),
		Name:        model.NewId(),
		Description: model.NewId(),
		Scope:       model.SCHEME_SCOPE_CHANNEL,
	}

	s2 := &model.Scheme{
		DisplayName: model.NewId(),
		Name:        model.NewId(),
		Description: model.NewId(),
		Scope:       model.SCHEME_SCOPE_CHANNEL,
	}

	s1, err := ss.Scheme().Save(s1)
	require.NoError(t, err)
	s2, err = ss.Scheme().Save(s2)
	require.NoError(t, err)

	// Create and save some teams.
	c1 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
		SchemeId:    &s1.Id,
	}

	c2 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
		SchemeId:    &s1.Id,
	}

	c3 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}

	_, _ = ss.Channel().Save(c1, 100)
	_, _ = ss.Channel().Save(c2, 100)
	_, _ = ss.Channel().Save(c3, 100)

	// Get the channels by a valid Scheme ID.
	d1, err := ss.Channel().GetChannelsByScheme(s1.Id, 0, 100)
	assert.NoError(t, err)
	assert.Len(t, d1, 2)

	// Get the channels by a valid Scheme ID where there aren't any matching Channel.
	d2, err := ss.Channel().GetChannelsByScheme(s2.Id, 0, 100)
	assert.NoError(t, err)
	assert.Empty(t, d2)

	// Get the channels by an invalid Scheme ID.
	d3, err := ss.Channel().GetChannelsByScheme(model.NewId(), 0, 100)
	assert.NoError(t, err)
	assert.Empty(t, d3)
}

func testChannelStoreMigrateChannelMembers(t *testing.T, ss store.Store) {
	s1 := model.NewId()
	c1 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
		SchemeId:    &s1,
	}
	c1, _ = ss.Channel().Save(c1, 100)

	cm1 := &model.ChannelMember{
		ChannelId:     c1.Id,
		UserId:        model.NewId(),
		ExplicitRoles: "channel_admin channel_user",
		NotifyProps:   model.GetDefaultChannelNotifyProps(),
	}
	cm2 := &model.ChannelMember{
		ChannelId:     c1.Id,
		UserId:        model.NewId(),
		ExplicitRoles: "channel_user",
		NotifyProps:   model.GetDefaultChannelNotifyProps(),
	}
	cm3 := &model.ChannelMember{
		ChannelId:     c1.Id,
		UserId:        model.NewId(),
		ExplicitRoles: "something_else",
		NotifyProps:   model.GetDefaultChannelNotifyProps(),
	}

	cm1, _ = ss.Channel().SaveMember(cm1)
	cm2, _ = ss.Channel().SaveMember(cm2)
	cm3, _ = ss.Channel().SaveMember(cm3)

	lastDoneChannelId := strings.Repeat("0", 26)
	lastDoneUserId := strings.Repeat("0", 26)

	for {
		data, err := ss.Channel().MigrateChannelMembers(lastDoneChannelId, lastDoneUserId)
		if assert.NoError(t, err) {
			if data == nil {
				break
			}
			lastDoneChannelId = data["ChannelId"]
			lastDoneUserId = data["UserId"]
		}
	}

	ss.Channel().ClearCaches()

	cm1b, err := ss.Channel().GetMember(context.Background(), cm1.ChannelId, cm1.UserId)
	assert.NoError(t, err)
	assert.Equal(t, "", cm1b.ExplicitRoles)
	assert.False(t, cm1b.SchemeGuest)
	assert.True(t, cm1b.SchemeUser)
	assert.True(t, cm1b.SchemeAdmin)

	cm2b, err := ss.Channel().GetMember(context.Background(), cm2.ChannelId, cm2.UserId)
	assert.NoError(t, err)
	assert.Equal(t, "", cm2b.ExplicitRoles)
	assert.False(t, cm1b.SchemeGuest)
	assert.True(t, cm2b.SchemeUser)
	assert.False(t, cm2b.SchemeAdmin)

	cm3b, err := ss.Channel().GetMember(context.Background(), cm3.ChannelId, cm3.UserId)
	assert.NoError(t, err)
	assert.Equal(t, "something_else", cm3b.ExplicitRoles)
	assert.False(t, cm1b.SchemeGuest)
	assert.False(t, cm3b.SchemeUser)
	assert.False(t, cm3b.SchemeAdmin)
}

func testResetAllChannelSchemes(t *testing.T, ss store.Store) {
	s1 := &model.Scheme{
		Name:        model.NewId(),
		DisplayName: model.NewId(),
		Description: model.NewId(),
		Scope:       model.SCHEME_SCOPE_CHANNEL,
	}
	s1, err := ss.Scheme().Save(s1)
	require.NoError(t, err)

	c1 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
		SchemeId:    &s1.Id,
	}

	c2 := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
		SchemeId:    &s1.Id,
	}

	c1, _ = ss.Channel().Save(c1, 100)
	c2, _ = ss.Channel().Save(c2, 100)

	assert.Equal(t, s1.Id, *c1.SchemeId)
	assert.Equal(t, s1.Id, *c2.SchemeId)

	err = ss.Channel().ResetAllChannelSchemes()
	assert.NoError(t, err)

	c1, _ = ss.Channel().Get(c1.Id, true)
	c2, _ = ss.Channel().Get(c2.Id, true)

	assert.Equal(t, "", *c1.SchemeId)
	assert.Equal(t, "", *c2.SchemeId)
}

func testChannelStoreClearAllCustomRoleAssignments(t *testing.T, ss store.Store) {
	c := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "Name",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}

	c, _ = ss.Channel().Save(c, 100)

	m1 := &model.ChannelMember{
		ChannelId:     c.Id,
		UserId:        model.NewId(),
		NotifyProps:   model.GetDefaultChannelNotifyProps(),
		ExplicitRoles: "system_user_access_token channel_user channel_admin",
	}
	m2 := &model.ChannelMember{
		ChannelId:     c.Id,
		UserId:        model.NewId(),
		NotifyProps:   model.GetDefaultChannelNotifyProps(),
		ExplicitRoles: "channel_user custom_role channel_admin another_custom_role",
	}
	m3 := &model.ChannelMember{
		ChannelId:     c.Id,
		UserId:        model.NewId(),
		NotifyProps:   model.GetDefaultChannelNotifyProps(),
		ExplicitRoles: "channel_user",
	}
	m4 := &model.ChannelMember{
		ChannelId:     c.Id,
		UserId:        model.NewId(),
		NotifyProps:   model.GetDefaultChannelNotifyProps(),
		ExplicitRoles: "custom_only",
	}

	_, err := ss.Channel().SaveMember(m1)
	require.NoError(t, err)
	_, err = ss.Channel().SaveMember(m2)
	require.NoError(t, err)
	_, err = ss.Channel().SaveMember(m3)
	require.NoError(t, err)
	_, err = ss.Channel().SaveMember(m4)
	require.NoError(t, err)

	require.NoError(t, ss.Channel().ClearAllCustomRoleAssignments())

	member, err := ss.Channel().GetMember(context.Background(), m1.ChannelId, m1.UserId)
	require.NoError(t, err)
	assert.Equal(t, m1.ExplicitRoles, member.Roles)

	member, err = ss.Channel().GetMember(context.Background(), m2.ChannelId, m2.UserId)
	require.NoError(t, err)
	assert.Equal(t, "channel_user channel_admin", member.Roles)

	member, err = ss.Channel().GetMember(context.Background(), m3.ChannelId, m3.UserId)
	require.NoError(t, err)
	assert.Equal(t, m3.ExplicitRoles, member.Roles)

	member, err = ss.Channel().GetMember(context.Background(), m4.ChannelId, m4.UserId)
	require.NoError(t, err)
	assert.Equal(t, "", member.Roles)
}

// testMaterializedPublicChannels tests edge cases involving the triggers and stored procedures
// that materialize the PublicChannels table.
func testMaterializedPublicChannels(t *testing.T, ss store.Store, s SqlStore) {
	teamId := model.NewId()

	// o1 is a public channel on the team
	o1 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Open Channel",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr := ss.Channel().Save(&o1, -1)
	require.NoError(t, nErr)

	// o2 is another public channel on the team
	o2 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Open Channel 2",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}
	_, nErr = ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	t.Run("o1 and o2 initially listed in public channels", func(t *testing.T) {
		channels, channelErr := ss.Channel().SearchInTeam(teamId, "", true)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&o1, &o2}, channels)
	})

	o1.DeleteAt = model.GetMillis()
	o1.UpdateAt = o1.DeleteAt

	e := ss.Channel().Delete(o1.Id, o1.DeleteAt)
	require.NoError(t, e, "channel should have been deleted")

	t.Run("o1 still listed in public channels when marked as deleted", func(t *testing.T) {
		channels, channelErr := ss.Channel().SearchInTeam(teamId, "", true)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&o1, &o2}, channels)
	})

	ss.Channel().PermanentDelete(o1.Id)

	t.Run("o1 no longer listed in public channels when permanently deleted", func(t *testing.T) {
		channels, channelErr := ss.Channel().SearchInTeam(teamId, "", true)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&o2}, channels)
	})

	o2.Type = model.CHANNEL_PRIVATE
	_, err := ss.Channel().Update(&o2)
	require.NoError(t, err)

	t.Run("o2 no longer listed since now private", func(t *testing.T) {
		channels, channelErr := ss.Channel().SearchInTeam(teamId, "", true)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{}, channels)
	})

	o2.Type = model.CHANNEL_OPEN
	_, err = ss.Channel().Update(&o2)
	require.NoError(t, err)

	t.Run("o2 listed once again since now public", func(t *testing.T) {
		channels, channelErr := ss.Channel().SearchInTeam(teamId, "", true)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&o2}, channels)
	})

	// o3 is a public channel on the team that already existed in the PublicChannels table.
	o3 := model.Channel{
		Id:          model.NewId(),
		TeamId:      teamId,
		DisplayName: "Open Channel 3",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}

	_, execerr := s.GetMaster().ExecNoTimeout(`
		INSERT INTO
		    PublicChannels(Id, DeleteAt, TeamId, DisplayName, Name, Header, Purpose)
		VALUES
		    (:Id, :DeleteAt, :TeamId, :DisplayName, :Name, :Header, :Purpose);
	`, map[string]interface{}{
		"Id":          o3.Id,
		"DeleteAt":    o3.DeleteAt,
		"TeamId":      o3.TeamId,
		"DisplayName": o3.DisplayName,
		"Name":        o3.Name,
		"Header":      o3.Header,
		"Purpose":     o3.Purpose,
	})
	require.NoError(t, execerr)

	o3.DisplayName = "Open Channel 3 - Modified"

	_, execerr = s.GetMaster().ExecNoTimeout(`
		INSERT INTO
		    Channels(Id, CreateAt, UpdateAt, DeleteAt, TeamId, Type, DisplayName, Name, Header, Purpose, LastPostAt, TotalMsgCount, ExtraUpdateAt, CreatorId, TotalMsgCountRoot)
		VALUES
		    (:Id, :CreateAt, :UpdateAt, :DeleteAt, :TeamId, :Type, :DisplayName, :Name, :Header, :Purpose, :LastPostAt, :TotalMsgCount, :ExtraUpdateAt, :CreatorId, 0);
	`, map[string]interface{}{
		"Id":            o3.Id,
		"CreateAt":      o3.CreateAt,
		"UpdateAt":      o3.UpdateAt,
		"DeleteAt":      o3.DeleteAt,
		"TeamId":        o3.TeamId,
		"Type":          o3.Type,
		"DisplayName":   o3.DisplayName,
		"Name":          o3.Name,
		"Header":        o3.Header,
		"Purpose":       o3.Purpose,
		"LastPostAt":    o3.LastPostAt,
		"TotalMsgCount": o3.TotalMsgCount,
		"ExtraUpdateAt": o3.ExtraUpdateAt,
		"CreatorId":     o3.CreatorId,
	})
	require.NoError(t, execerr)

	t.Run("verify o3 INSERT converted to UPDATE", func(t *testing.T) {
		channels, channelErr := ss.Channel().SearchInTeam(teamId, "", true)
		require.NoError(t, channelErr)
		require.Equal(t, &model.ChannelList{&o2, &o3}, channels)
	})

	// o4 is a public channel on the team that existed in the Channels table but was omitted from the PublicChannels table.
	o4 := model.Channel{
		TeamId:      teamId,
		DisplayName: "Open Channel 4",
		Name:        model.NewId(),
		Type:        model.CHANNEL_OPEN,
	}

	_, nErr = ss.Channel().Save(&o4, -1)
	require.NoError(t, nErr)

	_, execerr = s.GetMaster().ExecNoTimeout(`
		DELETE FROM
		    PublicChannels
		WHERE
		    Id = :Id
	`, map[string]interface{}{
		"Id": o4.Id,
	})
	require.NoError(t, execerr)

	o4.DisplayName += " - Modified"
	_, err = ss.Channel().Update(&o4)
	require.NoError(t, err)

	t.Run("verify o4 UPDATE converted to INSERT", func(t *testing.T) {
		channels, err := ss.Channel().SearchInTeam(teamId, "", true)
		require.NoError(t, err)
		require.Equal(t, &model.ChannelList{&o2, &o3, &o4}, channels)
	})
}

func testChannelStoreGetAllChannelsForExportAfter(t *testing.T, ss store.Store) {
	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewId()
	t1.Email = MakeEmail()
	t1.Type = model.TEAM_OPEN
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	c1 := model.Channel{}
	c1.TeamId = t1.Id
	c1.DisplayName = "Channel1"
	c1.Name = "zz" + model.NewId() + "b"
	c1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	d1, err := ss.Channel().GetAllChannelsForExportAfter(10000, strings.Repeat("0", 26))
	assert.NoError(t, err)

	found := false
	for _, c := range d1 {
		if c.Id == c1.Id {
			found = true
			assert.Equal(t, t1.Id, c.TeamId)
			assert.Nil(t, c.SchemeId)
			assert.Equal(t, t1.Name, c.TeamName)
		}
	}
	assert.True(t, found)
}

func testChannelStoreGetChannelMembersForExport(t *testing.T, ss store.Store) {
	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewId()
	t1.Email = MakeEmail()
	t1.Type = model.TEAM_OPEN
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	c1 := model.Channel{}
	c1.TeamId = t1.Id
	c1.DisplayName = "Channel1"
	c1.Name = "zz" + model.NewId() + "b"
	c1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	c2 := model.Channel{}
	c2.TeamId = model.NewId()
	c2.DisplayName = "Channel2"
	c2.Name = "zz" + model.NewId() + "b"
	c2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(&c2, -1)
	require.NoError(t, nErr)

	u1 := model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err = ss.User().Save(&u1)
	require.NoError(t, err)

	m1 := model.ChannelMember{}
	m1.ChannelId = c1.Id
	m1.UserId = u1.Id
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{}
	m2.ChannelId = c2.Id
	m2.UserId = u1.Id
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	d1, err := ss.Channel().GetChannelMembersForExport(u1.Id, t1.Id)
	assert.NoError(t, err)

	assert.Len(t, d1, 1)

	cmfe1 := d1[0]
	assert.Equal(t, c1.Name, cmfe1.ChannelName)
	assert.Equal(t, c1.Id, cmfe1.ChannelId)
	assert.Equal(t, u1.Id, cmfe1.UserId)
}

func testChannelStoreRemoveAllDeactivatedMembers(t *testing.T, ss store.Store, s SqlStore) {
	// Set up all the objects needed in the store.
	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewId()
	t1.Email = MakeEmail()
	t1.Type = model.TEAM_OPEN
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	c1 := model.Channel{}
	c1.TeamId = t1.Id
	c1.DisplayName = "Channel1"
	c1.Name = "zz" + model.NewId() + "b"
	c1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	u1 := model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err = ss.User().Save(&u1)
	require.NoError(t, err)

	u2 := model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(&u2)
	require.NoError(t, err)

	u3 := model.User{}
	u3.Email = MakeEmail()
	u3.Nickname = model.NewId()
	_, err = ss.User().Save(&u3)
	require.NoError(t, err)

	m1 := model.ChannelMember{}
	m1.ChannelId = c1.Id
	m1.UserId = u1.Id
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m1)
	require.NoError(t, err)

	m2 := model.ChannelMember{}
	m2.ChannelId = c1.Id
	m2.UserId = u2.Id
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m2)
	require.NoError(t, err)

	m3 := model.ChannelMember{}
	m3.ChannelId = c1.Id
	m3.UserId = u3.Id
	m3.NotifyProps = model.GetDefaultChannelNotifyProps()
	_, err = ss.Channel().SaveMember(&m3)
	require.NoError(t, err)

	// Get all the channel members. Check there are 3.
	d1, err := ss.Channel().GetMembers(c1.Id, 0, 1000)
	assert.NoError(t, err)
	assert.Len(t, *d1, 3)

	// Deactivate users 1 & 2.
	u1.DeleteAt = model.GetMillis()
	u2.DeleteAt = model.GetMillis()
	_, err = ss.User().Update(&u1, true)
	require.NoError(t, err)
	_, err = ss.User().Update(&u2, true)
	require.NoError(t, err)

	// Remove all deactivated users from the channel.
	assert.NoError(t, ss.Channel().RemoveAllDeactivatedMembers(c1.Id))

	// Get all the channel members. Check there is now only 1: m3.
	d2, err := ss.Channel().GetMembers(c1.Id, 0, 1000)
	assert.NoError(t, err)
	assert.Len(t, *d2, 1)
	assert.Equal(t, u3.Id, (*d2)[0].UserId)

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testChannelStoreExportAllDirectChannels(t *testing.T, ss store.Store, s SqlStore) {
	teamId := model.NewId()

	o1 := model.Channel{}
	o1.TeamId = teamId
	o1.DisplayName = "Name" + model.NewId()
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_DIRECT

	userIds := []string{model.NewId(), model.NewId(), model.NewId()}

	o2 := model.Channel{}
	o2.Name = model.GetGroupNameFromUserIds(userIds)
	o2.DisplayName = "GroupChannel" + model.NewId()
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_GROUP
	_, nErr := ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = u1.Id
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelId = o1.Id
	m2.UserId = u2.Id
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	ss.Channel().SaveDirectChannel(&o1, &m1, &m2)

	d1, nErr := ss.Channel().GetAllDirectChannelsForExportAfter(10000, strings.Repeat("0", 26))
	assert.NoError(t, nErr)

	assert.Len(t, d1, 2)
	assert.ElementsMatch(t, []string{o1.DisplayName, o2.DisplayName}, []string{d1[0].DisplayName, d1[1].DisplayName})

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testChannelStoreExportAllDirectChannelsExcludePrivateAndPublic(t *testing.T, ss store.Store, s SqlStore) {
	teamId := model.NewId()

	o1 := model.Channel{}
	o1.TeamId = teamId
	o1.DisplayName = "The Direct Channel" + model.NewId()
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_DIRECT

	o2 := model.Channel{}
	o2.TeamId = teamId
	o2.DisplayName = "Channel2" + model.NewId()
	o2.Name = "zz" + model.NewId() + "b"
	o2.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(&o2, -1)
	require.NoError(t, nErr)

	o3 := model.Channel{}
	o3.TeamId = teamId
	o3.DisplayName = "Channel3" + model.NewId()
	o3.Name = "zz" + model.NewId() + "b"
	o3.Type = model.CHANNEL_PRIVATE
	_, nErr = ss.Channel().Save(&o3, -1)
	require.NoError(t, nErr)

	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = u1.Id
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelId = o1.Id
	m2.UserId = u2.Id
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	ss.Channel().SaveDirectChannel(&o1, &m1, &m2)

	d1, nErr := ss.Channel().GetAllDirectChannelsForExportAfter(10000, strings.Repeat("0", 26))
	assert.NoError(t, nErr)
	assert.Len(t, d1, 1)
	assert.Equal(t, o1.DisplayName, d1[0].DisplayName)

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testChannelStoreExportAllDirectChannelsDeletedChannel(t *testing.T, ss store.Store, s SqlStore) {
	teamId := model.NewId()

	o1 := model.Channel{}
	o1.TeamId = teamId
	o1.DisplayName = "Different Name" + model.NewId()
	o1.Name = "zz" + model.NewId() + "b"
	o1.Type = model.CHANNEL_DIRECT

	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewId()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u1.Id}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewId()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamId: model.NewId(), UserId: u2.Id}, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelId = o1.Id
	m1.UserId = u1.Id
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelId = o1.Id
	m2.UserId = u2.Id
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	ss.Channel().SaveDirectChannel(&o1, &m1, &m2)

	o1.DeleteAt = 1
	nErr = ss.Channel().SetDeleteAt(o1.Id, 1, 1)
	require.NoError(t, nErr, "channel should have been deleted")

	d1, nErr := ss.Channel().GetAllDirectChannelsForExportAfter(10000, strings.Repeat("0", 26))
	assert.NoError(t, nErr)

	assert.Equal(t, 0, len(d1))

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testChannelStoreGetChannelsBatchForIndexing(t *testing.T, ss store.Store) {
	// Set up all the objects needed
	c1 := &model.Channel{}
	c1.DisplayName = "Channel1"
	c1.Name = "zz" + model.NewId() + "b"
	c1.Type = model.CHANNEL_OPEN
	_, nErr := ss.Channel().Save(c1, -1)
	require.NoError(t, nErr)

	time.Sleep(10 * time.Millisecond)

	c2 := &model.Channel{}
	c2.DisplayName = "Channel2"
	c2.Name = "zz" + model.NewId() + "b"
	c2.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(c2, -1)
	require.NoError(t, nErr)

	time.Sleep(10 * time.Millisecond)
	startTime := c2.CreateAt

	c3 := &model.Channel{}
	c3.DisplayName = "Channel3"
	c3.Name = "zz" + model.NewId() + "b"
	c3.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(c3, -1)
	require.NoError(t, nErr)

	c4 := &model.Channel{}
	c4.DisplayName = "Channel4"
	c4.Name = "zz" + model.NewId() + "b"
	c4.Type = model.CHANNEL_PRIVATE
	_, nErr = ss.Channel().Save(c4, -1)
	require.NoError(t, nErr)

	c5 := &model.Channel{}
	c5.DisplayName = "Channel5"
	c5.Name = "zz" + model.NewId() + "b"
	c5.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(c5, -1)
	require.NoError(t, nErr)

	time.Sleep(10 * time.Millisecond)

	c6 := &model.Channel{}
	c6.DisplayName = "Channel6"
	c6.Name = "zz" + model.NewId() + "b"
	c6.Type = model.CHANNEL_OPEN
	_, nErr = ss.Channel().Save(c6, -1)
	require.NoError(t, nErr)

	endTime := c6.CreateAt

	// First and last channel should be outside the range
	channels, err := ss.Channel().GetChannelsBatchForIndexing(startTime, endTime, 1000)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []*model.Channel{c2, c3, c5}, channels)

	// Update the endTime, last channel should be in
	endTime = model.GetMillis()
	channels, err = ss.Channel().GetChannelsBatchForIndexing(startTime, endTime, 1000)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []*model.Channel{c2, c3, c5, c6}, channels)

	// Testing the limit
	channels, err = ss.Channel().GetChannelsBatchForIndexing(startTime, endTime, 2)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []*model.Channel{c2, c3}, channels)
}

func testGroupSyncedChannelCount(t *testing.T, ss store.Store) {
	channel1, nErr := ss.Channel().Save(&model.Channel{
		DisplayName:      model.NewId(),
		Name:             model.NewId(),
		Type:             model.CHANNEL_PRIVATE,
		GroupConstrained: model.NewBool(true),
	}, 999)
	require.NoError(t, nErr)
	require.True(t, channel1.IsGroupConstrained())
	defer ss.Channel().PermanentDelete(channel1.Id)

	channel2, nErr := ss.Channel().Save(&model.Channel{
		DisplayName: model.NewId(),
		Name:        model.NewId(),
		Type:        model.CHANNEL_PRIVATE,
	}, 999)
	require.NoError(t, nErr)
	require.False(t, channel2.IsGroupConstrained())
	defer ss.Channel().PermanentDelete(channel2.Id)

	count, err := ss.Channel().GroupSyncedChannelCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))

	channel2.GroupConstrained = model.NewBool(true)
	channel2, err = ss.Channel().Update(channel2)
	require.NoError(t, err)
	require.True(t, channel2.IsGroupConstrained())

	countAfter, err := ss.Channel().GroupSyncedChannelCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, countAfter, count+1)
}

func testSetShared(t *testing.T, ss store.Store) {
	channel := &model.Channel{
		TeamId:      model.NewId(),
		DisplayName: "test_share_flag",
		Name:        "test_share_flag",
		Type:        model.CHANNEL_OPEN,
	}
	channelSaved, err := ss.Channel().Save(channel, 999)
	require.NoError(t, err)

	t.Run("Check default", func(t *testing.T) {
		assert.False(t, channelSaved.IsShared())
	})

	t.Run("Set Shared flag", func(t *testing.T) {
		err := ss.Channel().SetShared(channelSaved.Id, true)
		require.NoError(t, err)

		channelMod, err := ss.Channel().Get(channelSaved.Id, false)
		require.NoError(t, err)

		assert.True(t, channelMod.IsShared())
	})

	t.Run("Set Shared for invalid id", func(t *testing.T) {
		err := ss.Channel().SetShared(model.NewId(), true)
		require.Error(t, err)
	})
}

func testGetTeamForChannel(t *testing.T, ss store.Store) {
	team, err := ss.Team().Save(&model.Team{
		Name:        "myteam",
		DisplayName: "DisplayName",
		Email:       MakeEmail(),
		Type:        model.TEAM_OPEN,
	})
	require.NoError(t, err)

	channel := &model.Channel{
		TeamId:      team.Id,
		DisplayName: "test_share_flag",
		Name:        "test_share_flag",
		Type:        model.CHANNEL_OPEN,
	}
	channelSaved, err := ss.Channel().Save(channel, 999)
	require.NoError(t, err)

	got, err := ss.Channel().GetTeamForChannel(channelSaved.Id)
	require.NoError(t, err)
	assert.Equal(t, team.Id, got.Id)

	_, err = ss.Channel().GetTeamForChannel("notfound")
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}
