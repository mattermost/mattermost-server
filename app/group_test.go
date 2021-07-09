// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetGroup(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	group := th.CreateGroup()

	group, err := th.App.GetGroup(group.ID)
	require.Nil(t, err)
	require.NotNil(t, group)

	group, err = th.App.GetGroup(model.NewID())
	require.NotNil(t, err)
	require.Nil(t, group)
}

func TestGetGroupByRemoteID(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	group := th.CreateGroup()

	g, err := th.App.GetGroupByRemoteID(group.RemoteID, model.GroupSourceLdap)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.GetGroupByRemoteID(model.NewID(), model.GroupSourceLdap)
	require.NotNil(t, err)
	require.Nil(t, g)
}

func TestGetGroupsByType(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	th.CreateGroup()
	th.CreateGroup()
	th.CreateGroup()

	groups, err := th.App.GetGroupsBySource(model.GroupSourceLdap)
	require.Nil(t, err)
	require.NotEmpty(t, groups)

	groups, err = th.App.GetGroupsBySource(model.GroupSource("blah"))
	require.Nil(t, err)
	require.Empty(t, groups)
}

func TestCreateGroup(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	id := model.NewID()
	group := &model.Group{
		DisplayName: "dn_" + id,
		Name:        model.NewString("name" + id),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}

	g, err := th.App.CreateGroup(group)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.CreateGroup(group)
	require.NotNil(t, err)
	require.Nil(t, g)
}

func TestUpdateGroup(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	group := th.CreateGroup()
	group.DisplayName = model.NewID()

	g, err := th.App.UpdateGroup(group)
	require.Nil(t, err)
	require.NotNil(t, g)
}

func TestDeleteGroup(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	group := th.CreateGroup()

	g, err := th.App.DeleteGroup(group.ID)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.DeleteGroup(group.ID)
	require.NotNil(t, err)
	require.Nil(t, g)
}

func TestCreateOrRestoreGroupMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	g, err := th.App.UpsertGroupMember(group.ID, th.BasicUser.ID)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.UpsertGroupMember(group.ID, th.BasicUser.ID)
	require.Nil(t, err)
	require.NotNil(t, g)
}

func TestDeleteGroupMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	groupMember, err := th.App.UpsertGroupMember(group.ID, th.BasicUser.ID)
	require.Nil(t, err)
	require.NotNil(t, groupMember)

	groupMember, err = th.App.DeleteGroupMember(groupMember.GroupID, groupMember.UserID)
	require.Nil(t, err)
	require.NotNil(t, groupMember)

	groupMember, err = th.App.DeleteGroupMember(groupMember.GroupID, groupMember.UserID)
	require.NotNil(t, err)
	require.Nil(t, groupMember)
}

func TestUpsertGroupSyncable(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	groupSyncable := model.NewGroupTeam(group.ID, th.BasicTeam.ID, false)

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	// can update again without error
	gs, err = th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	gs, err = th.App.DeleteGroupSyncable(gs.GroupID, gs.SyncableID, gs.Type)
	require.Nil(t, err)
	require.NotEqual(t, int64(0), gs.DeleteAt)

	// Un-deleting works
	gs.DeleteAt = 0
	gs, err = th.App.UpsertGroupSyncable(gs)
	require.Nil(t, err)
	require.Equal(t, int64(0), gs.DeleteAt)
}

func TestUpsertGroupSyncableTeamGroupConstrained(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	group1 := th.CreateGroup()
	group2 := th.CreateGroup()

	team := th.CreateTeam()
	team.GroupConstrained = model.NewBool(true)
	team, err := th.App.UpdateTeam(team)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupSyncable(model.NewGroupTeam(group1.ID, team.ID, false))
	require.Nil(t, err)

	channel := th.CreateChannel(team)

	_, err = th.App.UpsertGroupSyncable(model.NewGroupChannel(group2.ID, channel.ID, false))
	require.NotNil(t, err)
	require.Equal(t, err.ID, "group_not_associated_to_synced_team")

	gs, err := th.App.GetGroupSyncable(group2.ID, channel.ID, model.GroupSyncableTypeChannel)
	require.Nil(t, gs)
	require.NotNil(t, err)

	_, err = th.App.UpsertGroupSyncable(model.NewGroupChannel(group1.ID, channel.ID, false))
	require.Nil(t, err)
}

func TestGetGroupSyncable(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	groupSyncable := model.NewGroupTeam(group.ID, th.BasicTeam.ID, false)

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	gs, err = th.App.GetGroupSyncable(group.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	require.Nil(t, err)
	require.NotNil(t, gs)
}

func TestGetGroupSyncables(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	// Create a group team
	groupSyncable := model.NewGroupTeam(group.ID, th.BasicTeam.ID, false)

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	groupTeams, err := th.App.GetGroupSyncables(group.ID, model.GroupSyncableTypeTeam)
	require.Nil(t, err)

	require.NotEmpty(t, groupTeams)
}

func TestDeleteGroupSyncable(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	groupChannel := model.NewGroupChannel(group.ID, th.BasicChannel.ID, false)

	gs, err := th.App.UpsertGroupSyncable(groupChannel)
	require.Nil(t, err)
	require.NotNil(t, gs)

	gs, err = th.App.DeleteGroupSyncable(group.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel)
	require.Nil(t, err)
	require.NotNil(t, gs)

	gs, err = th.App.DeleteGroupSyncable(group.ID, th.BasicChannel.ID, model.GroupSyncableTypeChannel)
	require.NotNil(t, err)
	require.Nil(t, gs)
}

func TestGetGroupsByChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	// Create a group channel
	groupSyncable := &model.GroupSyncable{
		GroupID:    group.ID,
		AutoAdd:    false,
		SyncableID: th.BasicChannel.ID,
		Type:       model.GroupSyncableTypeChannel,
	}

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	opts := model.GroupSearchOpts{
		PageOpts: &model.PageOpts{
			Page:    0,
			PerPage: 60,
		},
	}

	groups, _, err := th.App.GetGroupsByChannel(th.BasicChannel.ID, opts)
	require.Nil(t, err)
	require.ElementsMatch(t, []*model.GroupWithSchemeAdmin{{Group: *group, SchemeAdmin: model.NewBool(false)}}, groups)
	require.NotNil(t, groups[0].SchemeAdmin)

	groups, _, err = th.App.GetGroupsByChannel(model.NewID(), opts)
	require.Nil(t, err)
	require.Empty(t, groups)
}

func TestGetGroupsAssociatedToChannelsByTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	// Create a group channel
	groupSyncable := &model.GroupSyncable{
		GroupID:    group.ID,
		AutoAdd:    false,
		SyncableID: th.BasicChannel.ID,
		Type:       model.GroupSyncableTypeChannel,
	}

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	opts := model.GroupSearchOpts{
		PageOpts: &model.PageOpts{
			Page:    0,
			PerPage: 60,
		},
	}

	groups, err := th.App.GetGroupsAssociatedToChannelsByTeam(th.BasicTeam.ID, opts)
	require.Nil(t, err)

	assert.Equal(t, map[string][]*model.GroupWithSchemeAdmin{
		th.BasicChannel.ID: {
			{Group: *group, SchemeAdmin: model.NewBool(false)},
		},
	}, groups)
	require.NotNil(t, groups[th.BasicChannel.ID][0].SchemeAdmin)

	groups, err = th.App.GetGroupsAssociatedToChannelsByTeam(model.NewID(), opts)
	require.Nil(t, err)
	require.Empty(t, groups)
}

func TestGetGroupsByTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	// Create a group team
	groupSyncable := &model.GroupSyncable{
		GroupID:    group.ID,
		AutoAdd:    false,
		SyncableID: th.BasicTeam.ID,
		Type:       model.GroupSyncableTypeTeam,
	}

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	groups, _, err := th.App.GetGroupsByTeam(th.BasicTeam.ID, model.GroupSearchOpts{})
	require.Nil(t, err)
	require.ElementsMatch(t, []*model.GroupWithSchemeAdmin{{Group: *group, SchemeAdmin: model.NewBool(false)}}, groups)
	require.NotNil(t, groups[0].SchemeAdmin)

	groups, _, err = th.App.GetGroupsByTeam(model.NewID(), model.GroupSearchOpts{})
	require.Nil(t, err)
	require.Empty(t, groups)
}

func TestGetGroups(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	group := th.CreateGroup()

	groups, err := th.App.GetGroups(0, 60, model.GroupSearchOpts{})
	require.Nil(t, err)
	require.ElementsMatch(t, []*model.Group{group}, groups)
}

func TestUserIsInAdminRoleGroup(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group1 := th.CreateGroup()
	group2 := th.CreateGroup()

	g, err := th.App.UpsertGroupMember(group1.ID, th.BasicUser.ID)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.UpsertGroupMember(group2.ID, th.BasicUser.ID)
	require.Nil(t, err)
	require.NotNil(t, g)

	_, err = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupID:    group1.ID,
		AutoAdd:    false,
		SyncableID: th.BasicTeam.ID,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.Nil(t, err)

	groupSyncable2, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupID:    group2.ID,
		AutoAdd:    false,
		SyncableID: th.BasicTeam.ID,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.Nil(t, err)

	// no syncables are set to scheme admin true, so this returns false
	actual, err := th.App.UserIsInAdminRoleGroup(th.BasicUser.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	require.Nil(t, err)
	require.False(t, actual)

	// set a syncable to be scheme admins
	groupSyncable2.SchemeAdmin = true
	_, err = th.App.UpdateGroupSyncable(groupSyncable2)
	require.Nil(t, err)

	// a syncable is set to scheme admin true, so this returns true
	actual, err = th.App.UserIsInAdminRoleGroup(th.BasicUser.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	require.Nil(t, err)
	require.True(t, actual)

	// delete the syncable, should be false again
	th.App.DeleteGroupSyncable(group2.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	actual, err = th.App.UserIsInAdminRoleGroup(th.BasicUser.ID, th.BasicTeam.ID, model.GroupSyncableTypeTeam)
	require.Nil(t, err)
	require.False(t, actual)
}
