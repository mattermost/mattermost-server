// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestCreateDefaultMemberships(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	singersTeam, err := th.App.CreateTeam(th.Context, &model.Team{
		DisplayName: "Singers",
		Name:        "zz" + model.NewID(),
		Email:       "singers@test.com",
		Type:        model.TeamOpen,
	})
	if err != nil {
		t.Errorf("test team not created: %s", err.Error())
	}

	nerdsTeam, err := th.App.CreateTeam(th.Context, &model.Team{
		DisplayName: "Nerds",
		Name:        "zz" + model.NewID(),
		Email:       "nerds@test.com",
		Type:        model.TeamInvite,
	})
	if err != nil {
		t.Errorf("test team not created: %s", err.Error())
	}

	practiceChannel, err := th.App.CreateChannel(th.Context, &model.Channel{
		TeamID:      singersTeam.ID,
		DisplayName: "Practices",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}, false)
	if err != nil {
		t.Errorf("test channel not created: %s", err.Error())
	}

	experimentsChannel, err := th.App.CreateChannel(th.Context, &model.Channel{
		TeamID:      singersTeam.ID,
		DisplayName: "Experiments",
		Name:        model.NewID(),
		Type:        model.ChannelTypePrivate,
	}, false)
	if err != nil {
		t.Errorf("test channel not created: %s", err.Error())
	}

	gleeGroup, err := th.App.CreateGroup(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "Glee Club",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	if err != nil {
		t.Errorf("test group not created: %s", err.Error())
	}

	scienceGroup, err := th.App.CreateGroup(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "Science Club",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	if err != nil {
		t.Errorf("test group not created: %s", err.Error())
	}

	_, err = th.App.UpsertGroupSyncable(model.NewGroupChannel(gleeGroup.ID, practiceChannel.ID, true))
	if err != nil {
		t.Errorf("test groupchannel not created: %s", err.Error())
	}

	scienceTeamGroupSyncable, err := th.App.UpsertGroupSyncable(model.NewGroupTeam(scienceGroup.ID, nerdsTeam.ID, false))
	if err != nil {
		t.Errorf("test groupteam not created: %s", err.Error())
	}

	scienceChannelGroupSyncable, err := th.App.UpsertGroupSyncable(model.NewGroupChannel(scienceGroup.ID, experimentsChannel.ID, false))
	if err != nil {
		t.Errorf("test groupchannel not created: %s", err.Error())
	}

	singer1 := th.BasicUser
	scientist1 := th.BasicUser2

	_, err = th.App.UpsertGroupMember(gleeGroup.ID, singer1.ID)
	if err != nil {
		t.Errorf("test groupmember not created: %s", err.Error())
	}

	scientistGroupMember, err := th.App.UpsertGroupMember(scienceGroup.ID, scientist1.ID)
	if err != nil {
		t.Errorf("test groupmember not created: %s", err.Error())
	}

	pErr := th.App.CreateDefaultMemberships(th.Context, 0, false)
	if pErr != nil {
		t.Errorf("faild to populate syncables: %s", pErr.Error())
	}

	// Singer should be in team and channel
	_, err = th.App.GetTeamMember(singersTeam.ID, singer1.ID)
	if err != nil {
		t.Errorf("error retrieving team member: %s", err.Error())
	}
	_, err = th.App.GetChannelMember(context.Background(), practiceChannel.ID, singer1.ID)
	if err != nil {
		t.Errorf("error retrieving channel member: %s", err.Error())
	}

	tMembers, err := th.App.GetTeamMembers(singersTeam.ID, 0, 999, nil)
	if err != nil {
		t.Errorf("error retrieving team members: %s", err.Error())
	}
	expected := 1
	actual := len(tMembers)
	if actual != expected {
		t.Errorf("expected %d team members but got %d", expected, actual)
	}

	cMembersCount, err := th.App.GetChannelMemberCount(practiceChannel.ID)
	if err != nil {
		t.Errorf("error retrieving team members: %s", err.Error())
	}
	if cMembersCount != int64(expected) {
		t.Errorf("expected %d team member but got %d", expected, cMembersCount)
	}

	// Scientist should not be in team or channel
	_, err = th.App.GetTeamMember(nerdsTeam.ID, scientist1.ID)
	if err.ID != "app.team.get_member.missing.app_error" {
		t.Errorf("wrong error: %s", err.ID)
	}

	_, err = th.App.GetChannelMember(context.Background(), experimentsChannel.ID, scientist1.ID)
	if err.ID != "app.channel.get_member.missing.app_error" {
		t.Errorf("wrong error: %s", err.ID)
	}

	tMembers, err = th.App.GetTeamMembers(nerdsTeam.ID, 0, 999, nil)
	if err != nil {
		t.Errorf("error retrieving team members: %s", err.Error())
	}
	expected = 0
	actual = len(tMembers)
	if actual != expected {
		t.Errorf("expected %d team members but got %d", expected, actual)
	}

	cMembersCount, err = th.App.GetChannelMemberCount(experimentsChannel.ID)
	if err != nil {
		t.Errorf("error retrieving team members: %s", err.Error())
	}
	if cMembersCount != int64(expected) {
		t.Errorf("expected %d team members but got %d", expected, cMembersCount)
	}

	// update AutoAdd to true
	scienceTeamGroupSyncable.AutoAdd = true
	scienceTeamGroupSyncable, err = th.App.UpdateGroupSyncable(scienceTeamGroupSyncable)
	if err != nil {
		t.Errorf("error updating group syncable: %s", err.Error())
	}

	// Sync everything after syncable was created (proving that team updates trigger re-sync)
	pErr = th.App.CreateDefaultMemberships(th.Context, scientistGroupMember.CreateAt+1, false)
	if pErr != nil {
		t.Errorf("faild to populate syncables: %s", pErr.Error())
	}

	// Scientist should be in team but not the channel
	_, err = th.App.GetTeamMember(nerdsTeam.ID, scientist1.ID)
	if err != nil {
		t.Errorf("error retrieving team member: %s", err.Error())
	}

	_, err = th.App.GetChannelMember(context.Background(), experimentsChannel.ID, scientist1.ID)
	if err.ID != "app.channel.get_member.missing.app_error" {
		t.Errorf("wrong error: %s", err.ID)
	}

	tMembers, err = th.App.GetTeamMembers(nerdsTeam.ID, 0, 999, nil)
	if err != nil {
		t.Errorf("error retrieving team members: %s", err.Error())
	}
	expected = 1
	actual = len(tMembers)
	if actual != expected {
		t.Errorf("expected %d team members but got %d", expected, actual)
	}

	expected = 0
	cMembersCount, err = th.App.GetChannelMemberCount(experimentsChannel.ID)
	if err != nil {
		t.Errorf("error retrieving team members: %s", err.Error())
	}
	if cMembersCount != int64(expected) {
		t.Errorf("expected %d team members but got %d", expected, cMembersCount)
	}

	// Update the channel syncable
	scienceChannelGroupSyncable.AutoAdd = true
	scienceChannelGroupSyncable, err = th.App.UpdateGroupSyncable(scienceChannelGroupSyncable)
	if err != nil {
		t.Errorf("error updating group syncable: %s", err.Error())
	}

	// Sync everything after syncable was created (proving that channel updates trigger re-sync)
	pErr = th.App.CreateDefaultMemberships(th.Context, scientistGroupMember.CreateAt+1, false)
	if pErr != nil {
		t.Errorf("faild to populate syncables: %s", pErr.Error())
	}

	expected = 1
	cMembersCount, err = th.App.GetChannelMemberCount(experimentsChannel.ID)
	if err != nil {
		t.Errorf("error retrieving team members: %s", err.Error())
	}
	if cMembersCount != int64(expected) {
		t.Errorf("expected %d team members but got %d", expected, cMembersCount)
	}

	// singer leaves team and channel
	err = th.App.LeaveChannel(th.Context, practiceChannel.ID, singer1.ID)
	if err != nil {
		t.Errorf("error leaving channel: %s", err.Error())
	}
	err = th.App.LeaveTeam(th.Context, singersTeam, singer1, "")
	if err != nil {
		t.Errorf("error leaving team: %s", err.Error())
	}

	// Even re-syncing from the beginning doesn't re-add to channel or team
	pErr = th.App.CreateDefaultMemberships(th.Context, 0, false)
	if pErr != nil {
		t.Errorf("faild to populate syncables: %s", pErr.Error())
	}

	// Singer should not be in team or channel
	tMember, err := th.App.GetTeamMember(singersTeam.ID, singer1.ID)
	if err != nil {
		t.Errorf("error retrieving team member: %s", err.Error())
	}
	if tMember.DeleteAt == 0 {
		t.Error("expected team member to remain deleted")
	}

	_, err = th.App.GetChannelMember(context.Background(), practiceChannel.ID, singer1.ID)
	if err == nil {
		t.Error("Expected channel member to remain deleted")
	}

	// Ensure members are in channel
	_, err = th.App.AddChannelMember(th.Context, scientist1.ID, experimentsChannel, ChannelMemberOpts{})
	if err != nil {
		t.Errorf("unable to add user to channel: %s", err.Error())
	}

	// Add other user so that user can leave channel
	_, err = th.App.AddTeamMember(th.Context, singersTeam.ID, singer1.ID)
	if err != nil {
		t.Errorf("unable to add user to team: %s", err.Error())
	}
	_, err = th.App.AddChannelMember(th.Context, singer1.ID, experimentsChannel, ChannelMemberOpts{})
	if err != nil {
		t.Errorf("unable to add user to channel: %s", err.Error())
	}

	// the channel syncable is updated
	scienceChannelGroupSyncable, err = th.App.UpdateGroupSyncable(scienceChannelGroupSyncable)
	if err != nil {
		t.Errorf("error updating group syncable: %s", err.Error())
	}

	pErr = th.App.CreateDefaultMemberships(th.Context, 0, false)
	if pErr != nil {
		t.Errorf("faild to populate syncables: %s", pErr.Error())
	}

	timeBeforeLeaving := model.GetMillis()

	// User leaves channel
	err = th.App.LeaveChannel(th.Context, experimentsChannel.ID, scientist1.ID)
	if err != nil {
		t.Errorf("unable to add user to channel: %s", err.Error())
	}

	timeAfterLeaving := model.GetMillis()

	// Purging channelmemberhistory doesn't re-add user to channel
	_, _, nErr := th.App.Srv().Store.ChannelMemberHistory().PermanentDeleteBatchForRetentionPolicies(
		0, timeBeforeLeaving, 1000, model.RetentionPolicyCursor{})
	if nErr != nil {
		t.Errorf("error permanently deleting channelmemberhistory: %s", nErr.Error())
	}

	pErr = th.App.CreateDefaultMemberships(th.Context, scienceChannelGroupSyncable.UpdateAt, false)
	if pErr != nil {
		t.Errorf("failed to populate syncables: %s", pErr.Error())
	}

	_, err = th.App.GetChannelMember(context.Background(), experimentsChannel.ID, scientist1.ID)
	if err == nil {
		t.Error("Expected channel member to remain deleted")
	}

	// Purging channelmemberhistory doesn't re-add user to channel
	_, _, nErr = th.App.Srv().Store.ChannelMemberHistory().PermanentDeleteBatchForRetentionPolicies(
		0, timeAfterLeaving, 1000, model.RetentionPolicyCursor{})
	if nErr != nil {
		t.Errorf("error permanently deleting channelmemberhistory: %s", nErr.Error())
	}

	pErr = th.App.CreateDefaultMemberships(th.Context, scienceChannelGroupSyncable.UpdateAt, false)
	if pErr != nil {
		t.Errorf("failed to populate syncables: %s", pErr.Error())
	}

	// Channel member is re-added.
	_, err = th.App.GetChannelMember(context.Background(), experimentsChannel.ID, scientist1.ID)
	if err != nil {
		t.Errorf("expected channel member: %s", err.Error())
	}

	t.Run("Team with restricted domains skips over members that do not match the allowed domains", func(t *testing.T) {
		restrictedUser := th.CreateUser()
		restrictedUser.Email = "restricted@mattermost.org"
		_, err = th.App.UpdateUser(restrictedUser, false)
		require.Nil(t, err)
		_, err = th.App.UpsertGroupMember(scienceGroup.ID, restrictedUser.ID)
		require.Nil(t, err)

		restrictedTeam, err := th.App.CreateTeam(th.Context, &model.Team{
			DisplayName:    "Restricted",
			Name:           "restricted" + model.NewID(),
			Email:          "restricted@mattermost.org",
			AllowedDomains: "mattermost.org",
			Type:           model.TeamOpen,
		})
		require.Nil(t, err)
		_, err = th.App.UpsertGroupSyncable(model.NewGroupTeam(scienceGroup.ID, restrictedTeam.ID, true))
		require.Nil(t, err)

		restrictedChannel, err := th.App.CreateChannel(th.Context, &model.Channel{
			TeamID:      restrictedTeam.ID,
			DisplayName: "Restricted",
			Name:        "restricted" + model.NewID(),
			Type:        model.ChannelTypeOpen,
		}, false)
		require.Nil(t, err)
		_, err = th.App.UpsertGroupSyncable(model.NewGroupChannel(scienceGroup.ID, restrictedChannel.ID, true))
		require.Nil(t, err)

		pErr = th.App.CreateDefaultMemberships(th.Context, 0, false)
		require.NoError(t, pErr)

		// Ensure only the restricted user was added to both the team and channel
		cMembersCount, err = th.App.GetChannelMemberCount(restrictedChannel.ID)
		require.Nil(t, err)
		require.Equal(t, cMembersCount, int64(1))
		tmembers, err := th.App.GetTeamMembers(restrictedTeam.ID, 0, 100, nil)
		require.Nil(t, err)
		require.Len(t, tmembers, 1)
		require.Equal(t, tmembers[0].UserID, restrictedUser.ID)
	})
}

func TestDeleteGroupMemberships(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	group := th.CreateGroup()

	userIDs := []string{th.BasicUser.ID, th.BasicUser2.ID, th.SystemAdminUser.ID}

	var err *model.AppError
	// add users to teams and channels
	for _, userID := range userIDs {
		_, err = th.App.AddTeamMember(th.Context, th.BasicTeam.ID, userID)
		require.Nil(t, err)

		_, err = th.App.AddChannelMember(th.Context, userID, th.BasicChannel, ChannelMemberOpts{})
		require.Nil(t, err)
	}

	// make team group-constrained
	team := th.BasicTeam
	team.GroupConstrained = model.NewBool(true)
	team, err = th.App.UpdateTeam(team)
	require.Nil(t, err)
	require.True(t, *team.GroupConstrained)

	// make channel group-constrained
	channel := th.BasicChannel
	channel.GroupConstrained = model.NewBool(true)
	channel, err = th.App.UpdateChannel(channel)
	require.Nil(t, err)
	require.True(t, *channel.GroupConstrained)

	// create groupteam and groupchannel
	_, err = th.App.UpsertGroupSyncable(model.NewGroupTeam(group.ID, team.ID, true))
	require.Nil(t, err)
	_, err = th.App.UpsertGroupSyncable(model.NewGroupChannel(group.ID, channel.ID, true))
	require.Nil(t, err)

	// verify the member count
	tmembers, err := th.App.GetTeamMembers(th.BasicTeam.ID, 0, 100, nil)
	require.Nil(t, err)
	require.Len(t, tmembers, 3)

	cmemberCount, err := th.App.GetChannelMemberCount(th.BasicChannel.ID)
	require.Nil(t, err)
	require.Equal(t, 3, int(cmemberCount))

	// add a user to the group
	_, err = th.App.UpsertGroupMember(group.ID, th.SystemAdminUser.ID)
	require.Nil(t, err)

	// run the delete
	appErr := th.App.DeleteGroupConstrainedMemberships(th.Context)
	require.NoError(t, appErr)

	// verify the new member counts
	tmembers, err = th.App.GetTeamMembers(th.BasicTeam.ID, 0, 100, nil)
	require.Nil(t, err)
	require.Len(t, tmembers, 1)
	require.Equal(t, th.SystemAdminUser.ID, tmembers[0].UserID)

	cmembers, err := th.App.GetChannelMembersPage(channel.ID, 0, 99)
	require.Nil(t, err)
	require.Len(t, (*cmembers), 1)
	require.Equal(t, th.SystemAdminUser.ID, (*cmembers)[0].UserID)
}

func TestSyncSyncableRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team := th.CreateTeam()

	channel := th.CreateChannel(team)
	channel.GroupConstrained = model.NewBool(true)
	channel, err := th.App.UpdateChannel(channel)
	require.Nil(t, err)

	user1 := th.CreateUser()
	user2 := th.CreateUser()
	group := th.CreateGroup()

	teamSyncable, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
		SyncableID: team.ID,
		Type:       model.GroupSyncableTypeTeam,
		GroupID:    group.ID,
	})
	require.Nil(t, err)

	channelSyncable, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
		SyncableID: channel.ID,
		Type:       model.GroupSyncableTypeChannel,
		GroupID:    group.ID,
	})
	require.Nil(t, err)

	for _, user := range []*model.User{user1, user2} {
		_, err = th.App.UpsertGroupMember(group.ID, user.ID)
		require.Nil(t, err)

		var tm *model.TeamMember
		tm, err = th.App.AddTeamMember(th.Context, team.ID, user.ID)
		require.Nil(t, err)
		require.False(t, tm.SchemeAdmin)

		cm := th.AddUserToChannel(user, channel)
		require.False(t, cm.SchemeAdmin)
	}

	teamSyncable.SchemeAdmin = true
	_, err = th.App.UpdateGroupSyncable(teamSyncable)
	require.Nil(t, err)

	channelSyncable.SchemeAdmin = true
	_, err = th.App.UpdateGroupSyncable(channelSyncable)
	require.Nil(t, err)

	err = th.App.SyncSyncableRoles(channel.ID, model.GroupSyncableTypeChannel)
	require.Nil(t, err)

	err = th.App.SyncSyncableRoles(team.ID, model.GroupSyncableTypeTeam)
	require.Nil(t, err)

	for _, user := range []*model.User{user1, user2} {
		tm, err := th.App.GetTeamMember(team.ID, user.ID)
		require.Nil(t, err)
		require.True(t, tm.SchemeAdmin)

		cm, err := th.App.GetChannelMember(context.Background(), channel.ID, user.ID)
		require.Nil(t, err)
		require.True(t, cm.SchemeAdmin)
	}
}
