// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"

	"math"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestChannelMemberHistoryStore(t *testing.T, ss store.Store) {
	t.Run("TestLogJoinEvent", func(t *testing.T) { testLogJoinEvent(t, ss) })
	t.Run("TestLogLeaveEvent", func(t *testing.T) { testLogLeaveEvent(t, ss) })
	t.Run("TestGetUsersInChannelAtChannelMemberHistory", func(t *testing.T) { testGetUsersInChannelAtChannelMemberHistory(t, ss) })
	t.Run("TestGetUsersInChannelAtChannelMembers", func(t *testing.T) { testGetUsersInChannelAtChannelMembers(t, ss) })
	t.Run("TestPermanentDeleteBatch", func(t *testing.T) { testPermanentDeleteBatch(t, ss) })
	t.Run("TestPermanentDeleteBatchForRetentionPolicies", func(t *testing.T) { testPermanentDeleteBatchForRetentionPolicies(t, ss) })
}

func testLogJoinEvent(t *testing.T, ss store.Store) {
	// create a test channel
	ch := model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Display " + model.NewID(),
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	channel, err := ss.Channel().Save(&ch, -1)
	require.NoError(t, err)

	// and a test user
	user := model.User{
		Email:    MakeEmail(),
		Nickname: model.NewID(),
		Username: model.NewID(),
	}
	userPtr, err := ss.User().Save(&user)
	require.NoError(t, err)
	user = *userPtr

	// log a join event
	err = ss.ChannelMemberHistory().LogJoinEvent(user.ID, channel.ID, model.GetMillis())
	assert.NoError(t, err)
}

func testLogLeaveEvent(t *testing.T, ss store.Store) {
	// create a test channel
	ch := model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Display " + model.NewID(),
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	channel, err := ss.Channel().Save(&ch, -1)
	require.NoError(t, err)

	// and a test user
	user := model.User{
		Email:    MakeEmail(),
		Nickname: model.NewID(),
		Username: model.NewID(),
	}
	userPtr, err := ss.User().Save(&user)
	require.NoError(t, err)
	user = *userPtr

	// log a join event, followed by a leave event
	err = ss.ChannelMemberHistory().LogJoinEvent(user.ID, channel.ID, model.GetMillis())
	assert.NoError(t, err)

	err = ss.ChannelMemberHistory().LogLeaveEvent(user.ID, channel.ID, model.GetMillis())
	assert.NoError(t, err)
}

func testGetUsersInChannelAtChannelMemberHistory(t *testing.T, ss store.Store) {
	// create a test channel
	ch := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Display " + model.NewID(),
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	channel, err := ss.Channel().Save(ch, -1)
	require.NoError(t, err)

	// and a test user
	user := model.User{
		Email:    MakeEmail(),
		Nickname: model.NewID(),
		Username: model.NewID(),
	}
	userPtr, err := ss.User().Save(&user)
	require.NoError(t, err)
	user = *userPtr

	// the user was previously in the channel a long time ago, before the export period starts
	// the existence of this record makes it look like the MessageExport feature has been active for awhile, and prevents
	// us from looking in the ChannelMembers table for data that isn't found in the ChannelMemberHistory table
	leaveTime := model.GetMillis() - 20000
	joinTime := leaveTime - 10000
	err = ss.ChannelMemberHistory().LogJoinEvent(user.ID, channel.ID, joinTime)
	require.NoError(t, err)
	err = ss.ChannelMemberHistory().LogLeaveEvent(user.ID, channel.ID, leaveTime)
	require.NoError(t, err)

	// log a join event
	leaveTime = model.GetMillis()
	joinTime = leaveTime - 10000
	err = ss.ChannelMemberHistory().LogJoinEvent(user.ID, channel.ID, joinTime)
	require.NoError(t, err)

	// case 1: user joins and leaves the channel before the export period begins
	channelMembers, err := ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime-500, joinTime-100, channel.ID)
	require.NoError(t, err)
	assert.Empty(t, channelMembers)

	// case 2: user joins the channel after the export period begins, but has not yet left the channel when the export period ends
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime-100, joinTime+500, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime, channelMembers[0].JoinTime)
	assert.Nil(t, channelMembers[0].LeaveTime)

	// case 3: user joins the channel before the export period begins, but has not yet left the channel when the export period ends
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime+100, joinTime+500, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime, channelMembers[0].JoinTime)
	assert.Nil(t, channelMembers[0].LeaveTime)

	// add a leave time for the user
	err = ss.ChannelMemberHistory().LogLeaveEvent(user.ID, channel.ID, leaveTime)
	require.NoError(t, err)

	// case 4: user joins the channel before the export period begins, but has not yet left the channel when the export period ends
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime+100, leaveTime-100, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime, channelMembers[0].JoinTime)
	assert.Equal(t, leaveTime, *channelMembers[0].LeaveTime)

	// case 5: user joins the channel after the export period begins, and leaves the channel before the export period ends
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime-100, leaveTime+100, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime, channelMembers[0].JoinTime)
	assert.Equal(t, leaveTime, *channelMembers[0].LeaveTime)

	// case 6: user has joined and left the channel long before the export period begins
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(leaveTime+100, leaveTime+200, channel.ID)
	require.NoError(t, err)
	assert.Empty(t, channelMembers)
}

func testGetUsersInChannelAtChannelMembers(t *testing.T, ss store.Store) {
	// create a test channel
	channel := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Display " + model.NewID(),
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	channel, err := ss.Channel().Save(channel, -1)
	require.NoError(t, err)

	// and a test user
	user := model.User{
		Email:    MakeEmail(),
		Nickname: model.NewID(),
		Username: model.NewID(),
	}
	userPtr, err := ss.User().Save(&user)
	require.NoError(t, err)
	user = *userPtr

	// clear any existing ChannelMemberHistory data that might interfere with our test
	var tableDataTruncated = false
	for !tableDataTruncated {
		var count int64
		count, _, err = ss.ChannelMemberHistory().PermanentDeleteBatchForRetentionPolicies(
			0, model.GetMillis(), 1000, model.RetentionPolicyCursor{})
		require.NoError(t, err, "Failed to truncate ChannelMemberHistory contents")
		tableDataTruncated = count == int64(0)
	}

	// in this test, we're pretending that Message Export was not activated during the export period, so there's no data
	// available in the ChannelMemberHistory table. Instead, we'll fall back to the ChannelMembers table for a rough approximation
	joinTime := int64(1000)
	leaveTime := joinTime + 5000
	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   channel.ID,
		UserID:      user.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	// in every single case, the user will be included in the export, because ChannelMembers says they were in the channel at some point in
	// the past, even though the time that they were actually in the channel doesn't necessarily overlap with the export period

	// case 1: user joins and leaves the channel before the export period begins
	channelMembers, err := ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime-500, joinTime-100, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime-500, channelMembers[0].JoinTime)
	assert.Equal(t, joinTime-100, *channelMembers[0].LeaveTime)

	// case 2: user joins the channel after the export period begins, but has not yet left the channel when the export period ends
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime-100, joinTime+500, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime-100, channelMembers[0].JoinTime)
	assert.Equal(t, joinTime+500, *channelMembers[0].LeaveTime)

	// case 3: user joins the channel before the export period begins, but has not yet left the channel when the export period ends
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime+100, joinTime+500, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime+100, channelMembers[0].JoinTime)
	assert.Equal(t, joinTime+500, *channelMembers[0].LeaveTime)

	// case 4: user joins the channel before the export period begins, but has not yet left the channel when the export period ends
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime+100, leaveTime-100, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime+100, channelMembers[0].JoinTime)
	assert.Equal(t, leaveTime-100, *channelMembers[0].LeaveTime)

	// case 5: user joins the channel after the export period begins, and leaves the channel before the export period ends
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime-100, leaveTime+100, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, joinTime-100, channelMembers[0].JoinTime)
	assert.Equal(t, leaveTime+100, *channelMembers[0].LeaveTime)

	// case 6: user has joined and left the channel long before the export period begins
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(leaveTime+100, leaveTime+200, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, channel.ID, channelMembers[0].ChannelID)
	assert.Equal(t, user.ID, channelMembers[0].UserID)
	assert.Equal(t, user.Email, channelMembers[0].UserEmail)
	assert.Equal(t, user.Username, channelMembers[0].Username)
	assert.Equal(t, leaveTime+100, channelMembers[0].JoinTime)
	assert.Equal(t, leaveTime+200, *channelMembers[0].LeaveTime)
}

func testPermanentDeleteBatch(t *testing.T, ss store.Store) {
	// create a test channel
	channel := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Display " + model.NewID(),
		Name:        "zz" + model.NewID() + "b",
		Type:        model.ChannelTypeOpen,
	}
	channel, err := ss.Channel().Save(channel, -1)
	require.NoError(t, err)

	// and two test users
	user := model.User{
		Email:    MakeEmail(),
		Nickname: model.NewID(),
		Username: model.NewID(),
	}
	userPtr, err := ss.User().Save(&user)
	require.NoError(t, err)
	user = *userPtr

	user2 := model.User{
		Email:    MakeEmail(),
		Nickname: model.NewID(),
		Username: model.NewID(),
	}
	user2Ptr, err := ss.User().Save(&user2)
	require.NoError(t, err)
	user2 = *user2Ptr

	// user1 joins and leaves the channel
	leaveTime := model.GetMillis()
	joinTime := leaveTime - 10000
	err = ss.ChannelMemberHistory().LogJoinEvent(user.ID, channel.ID, joinTime)
	require.NoError(t, err)
	err = ss.ChannelMemberHistory().LogLeaveEvent(user.ID, channel.ID, leaveTime)
	require.NoError(t, err)

	// user2 joins the channel but never leaves
	err = ss.ChannelMemberHistory().LogJoinEvent(user2.ID, channel.ID, joinTime)
	require.NoError(t, err)

	// in between the join time and the leave time, both users were members of the channel
	channelMembers, err := ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime+10, leaveTime-10, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 2)

	// the permanent delete should delete at least one record
	rowsDeleted, _, err := ss.ChannelMemberHistory().PermanentDeleteBatchForRetentionPolicies(
		0, leaveTime+1, math.MaxInt64, model.RetentionPolicyCursor{})
	require.NoError(t, err)
	assert.NotEqual(t, int64(0), rowsDeleted)

	// after the delete, there should be one less member in the channel
	channelMembers, err = ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime+10, leaveTime-10, channel.ID)
	require.NoError(t, err)
	assert.Len(t, channelMembers, 1)
	assert.Equal(t, user2.ID, channelMembers[0].UserID)
}

func testPermanentDeleteBatchForRetentionPolicies(t *testing.T, ss store.Store) {
	const limit = 1000
	team, err := ss.Team().Save(&model.Team{
		DisplayName: "DisplayName",
		Name:        "team" + model.NewID(),
		Email:       MakeEmail(),
		Type:        model.TeamOpen,
	})
	require.NoError(t, err)
	channel, err := ss.Channel().Save(&model.Channel{
		TeamID:      team.ID,
		DisplayName: "DisplayName",
		Name:        "channel" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}, -1)
	require.NoError(t, err)
	userID := model.NewID()

	joinTime := int64(1000)
	leaveTime := int64(1500)
	err = ss.ChannelMemberHistory().LogJoinEvent(userID, channel.ID, joinTime)
	require.NoError(t, err)
	err = ss.ChannelMemberHistory().LogLeaveEvent(userID, channel.ID, leaveTime)
	require.NoError(t, err)

	channelPolicy, err := ss.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
		RetentionPolicy: model.RetentionPolicy{
			DisplayName:  "DisplayName",
			PostDuration: model.NewInt64(30),
		},
		ChannelIDs: []string{channel.ID},
	})
	require.NoError(t, err)

	nowMillis := leaveTime + *channelPolicy.PostDuration*24*60*60*1000 + 1
	_, _, err = ss.ChannelMemberHistory().PermanentDeleteBatchForRetentionPolicies(
		nowMillis, 0, limit, model.RetentionPolicyCursor{})
	require.NoError(t, err)
	result, err := ss.ChannelMemberHistory().GetUsersInChannelDuring(joinTime, leaveTime, channel.ID)
	require.NoError(t, err)
	require.Empty(t, result, "history should have been deleted by channel policy")
}
