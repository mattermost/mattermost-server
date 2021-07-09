// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func TestGroupStore(t *testing.T, ss store.Store) {
	t.Run("Create", func(t *testing.T) { testGroupStoreCreate(t, ss) })
	t.Run("Get", func(t *testing.T) { testGroupStoreGet(t, ss) })
	t.Run("GetByName", func(t *testing.T) { testGroupStoreGetByName(t, ss) })
	t.Run("GetByIDs", func(t *testing.T) { testGroupStoreGetByIDs(t, ss) })
	t.Run("GetByRemoteID", func(t *testing.T) { testGroupStoreGetByRemoteID(t, ss) })
	t.Run("GetAllBySource", func(t *testing.T) { testGroupStoreGetAllByType(t, ss) })
	t.Run("GetByUser", func(t *testing.T) { testGroupStoreGetByUser(t, ss) })
	t.Run("Update", func(t *testing.T) { testGroupStoreUpdate(t, ss) })
	t.Run("Delete", func(t *testing.T) { testGroupStoreDelete(t, ss) })

	t.Run("GetMemberUsers", func(t *testing.T) { testGroupGetMemberUsers(t, ss) })
	t.Run("GetMemberUsersPage", func(t *testing.T) { testGroupGetMemberUsersPage(t, ss) })

	t.Run("GetMemberUsersInTeam", func(t *testing.T) { testGroupGetMemberUsersInTeam(t, ss) })
	t.Run("GetMemberUsersNotInChannel", func(t *testing.T) { testGroupGetMemberUsersNotInChannel(t, ss) })

	t.Run("UpsertMember", func(t *testing.T) { testUpsertMember(t, ss) })
	t.Run("DeleteMember", func(t *testing.T) { testGroupDeleteMember(t, ss) })
	t.Run("PermanentDeleteMembersByUser", func(t *testing.T) { testGroupPermanentDeleteMembersByUser(t, ss) })

	t.Run("CreateGroupSyncable", func(t *testing.T) { testCreateGroupSyncable(t, ss) })
	t.Run("GetGroupSyncable", func(t *testing.T) { testGetGroupSyncable(t, ss) })
	t.Run("GetAllGroupSyncablesByGroupId", func(t *testing.T) { testGetAllGroupSyncablesByGroup(t, ss) })
	t.Run("UpdateGroupSyncable", func(t *testing.T) { testUpdateGroupSyncable(t, ss) })
	t.Run("DeleteGroupSyncable", func(t *testing.T) { testDeleteGroupSyncable(t, ss) })

	t.Run("TeamMembersToAdd", func(t *testing.T) { testTeamMembersToAdd(t, ss) })
	t.Run("TeamMembersToAdd_SingleTeam", func(t *testing.T) { testTeamMembersToAddSingleTeam(t, ss) })

	t.Run("ChannelMembersToAdd", func(t *testing.T) { testChannelMembersToAdd(t, ss) })
	t.Run("ChannelMembersToAdd_SingleChannel", func(t *testing.T) { testChannelMembersToAddSingleChannel(t, ss) })

	t.Run("TeamMembersToRemove", func(t *testing.T) { testTeamMembersToRemove(t, ss) })
	t.Run("TeamMembersToRemove_SingleTeam", func(t *testing.T) { testTeamMembersToRemoveSingleTeam(t, ss) })

	t.Run("ChannelMembersToRemove", func(t *testing.T) { testChannelMembersToRemove(t, ss) })
	t.Run("ChannelMembersToRemove_SingleChannel", func(t *testing.T) { testChannelMembersToRemoveSingleChannel(t, ss) })

	t.Run("GetGroupsByChannel", func(t *testing.T) { testGetGroupsByChannel(t, ss) })
	t.Run("GetGroupsAssociatedToChannelsByTeam", func(t *testing.T) { testGetGroupsAssociatedToChannelsByTeam(t, ss) })
	t.Run("GetGroupsByTeam", func(t *testing.T) { testGetGroupsByTeam(t, ss) })

	t.Run("GetGroups", func(t *testing.T) { testGetGroups(t, ss) })

	t.Run("TeamMembersMinusGroupMembers", func(t *testing.T) { testTeamMembersMinusGroupMembers(t, ss) })
	t.Run("ChannelMembersMinusGroupMembers", func(t *testing.T) { testChannelMembersMinusGroupMembers(t, ss) })

	t.Run("GetMemberCount", func(t *testing.T) { groupTestGetMemberCount(t, ss) })

	t.Run("AdminRoleGroupsForSyncableMember_Channel", func(t *testing.T) { groupTestAdminRoleGroupsForSyncableMemberChannel(t, ss) })
	t.Run("AdminRoleGroupsForSyncableMember_Team", func(t *testing.T) { groupTestAdminRoleGroupsForSyncableMemberTeam(t, ss) })
	t.Run("PermittedSyncableAdmins_Team", func(t *testing.T) { groupTestPermittedSyncableAdminsTeam(t, ss) })
	t.Run("PermittedSyncableAdmins_Channel", func(t *testing.T) { groupTestPermittedSyncableAdminsChannel(t, ss) })
	t.Run("UpdateMembersRole_Team", func(t *testing.T) { groupTestpUpdateMembersRoleTeam(t, ss) })
	t.Run("UpdateMembersRole_Channel", func(t *testing.T) { groupTestpUpdateMembersRoleChannel(t, ss) })

	t.Run("GroupCount", func(t *testing.T) { groupTestGroupCount(t, ss) })
	t.Run("GroupTeamCount", func(t *testing.T) { groupTestGroupTeamCount(t, ss) })
	t.Run("GroupChannelCount", func(t *testing.T) { groupTestGroupChannelCount(t, ss) })
	t.Run("GroupMemberCount", func(t *testing.T) { groupTestGroupMemberCount(t, ss) })
	t.Run("DistinctGroupMemberCount", func(t *testing.T) { groupTestDistinctGroupMemberCount(t, ss) })
	t.Run("GroupCountWithAllowReference", func(t *testing.T) { groupTestGroupCountWithAllowReference(t, ss) })
}

func testGroupStoreCreate(t *testing.T, ss store.Store) {
	// Save a new group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}

	// Happy path
	d1, err := ss.Group().Create(g1)
	require.NoError(t, err)
	require.Len(t, d1.ID, 26)
	require.Equal(t, *g1.Name, *d1.Name)
	require.Equal(t, g1.DisplayName, d1.DisplayName)
	require.Equal(t, g1.Description, d1.Description)
	require.Equal(t, g1.RemoteID, d1.RemoteID)
	require.NotZero(t, d1.CreateAt)
	require.NotZero(t, d1.UpdateAt)
	require.Zero(t, d1.DeleteAt)

	// Requires display name
	g2 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "",
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	data, err := ss.Group().Create(g2)
	require.Nil(t, data)
	require.Error(t, err)
	var appErr *model.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, appErr.ID, "model.group.display_name.app_error")

	// Won't accept a duplicate name
	g4 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	_, err = ss.Group().Create(g4)
	require.NoError(t, err)
	g4b := &model.Group{
		Name:        g4.Name,
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	data, err = ss.Group().Create(g4b)
	require.Nil(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("Group with name %s already exists", *g4b.Name))

	// Fields cannot be greater than max values
	g5 := &model.Group{
		Name:        model.NewString(strings.Repeat("x", model.GroupNameMaxLength)),
		DisplayName: strings.Repeat("x", model.GroupDisplayNameMaxLength),
		Description: strings.Repeat("x", model.GroupDescriptionMaxLength),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	require.Nil(t, g5.IsValidForCreate())

	g5.Name = model.NewString(*g5.Name + "x")
	require.Equal(t, g5.IsValidForCreate().ID, "model.group.name.invalid_length.app_error")
	g5.Name = model.NewString(model.NewID())
	require.Nil(t, g5.IsValidForCreate())

	g5.DisplayName = g5.DisplayName + "x"
	require.Equal(t, g5.IsValidForCreate().ID, "model.group.display_name.app_error")
	g5.DisplayName = model.NewID()
	require.Nil(t, g5.IsValidForCreate())

	g5.Description = g5.Description + "x"
	require.Equal(t, g5.IsValidForCreate().ID, "model.group.description.app_error")
	g5.Description = model.NewID()
	require.Nil(t, g5.IsValidForCreate())

	// Must use a valid type
	g6 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSource("fake"),
		RemoteID:    model.NewID(),
	}
	require.Equal(t, g6.IsValidForCreate().ID, "model.group.source.app_error")

	//must use valid characters
	g7 := &model.Group{
		Name:        model.NewString("%^#@$$"),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	require.Equal(t, g7.IsValidForCreate().ID, "model.group.name.invalid_chars.app_error")
}

func testGroupStoreGet(t *testing.T, ss store.Store) {
	// Create a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	d1, err := ss.Group().Create(g1)
	require.NoError(t, err)
	require.Len(t, d1.ID, 26)

	// Get the group
	d2, err := ss.Group().Get(d1.ID)
	require.NoError(t, err)
	require.Equal(t, d1.ID, d2.ID)
	require.Equal(t, *d1.Name, *d2.Name)
	require.Equal(t, d1.DisplayName, d2.DisplayName)
	require.Equal(t, d1.Description, d2.Description)
	require.Equal(t, d1.RemoteID, d2.RemoteID)
	require.Equal(t, d1.CreateAt, d2.CreateAt)
	require.Equal(t, d1.UpdateAt, d2.UpdateAt)
	require.Equal(t, d1.DeleteAt, d2.DeleteAt)

	// Get an invalid group
	_, err = ss.Group().Get(model.NewID())
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}

func testGroupStoreGetByName(t *testing.T, ss store.Store) {
	// Create a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	g1Opts := model.GroupSearchOpts{
		FilterAllowReference: false,
	}

	d1, err := ss.Group().Create(g1)
	require.NoError(t, err)
	require.Len(t, d1.ID, 26)

	// Get the group
	d2, err := ss.Group().GetByName(*d1.Name, g1Opts)
	require.NoError(t, err)
	require.Equal(t, d1.ID, d2.ID)
	require.Equal(t, *d1.Name, *d2.Name)
	require.Equal(t, d1.DisplayName, d2.DisplayName)
	require.Equal(t, d1.Description, d2.Description)
	require.Equal(t, d1.RemoteID, d2.RemoteID)
	require.Equal(t, d1.CreateAt, d2.CreateAt)
	require.Equal(t, d1.UpdateAt, d2.UpdateAt)
	require.Equal(t, d1.DeleteAt, d2.DeleteAt)

	// Get an invalid group
	_, err = ss.Group().GetByName(model.NewID(), g1Opts)
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}

func testGroupStoreGetByIDs(t *testing.T, ss store.Store) {
	var group1 *model.Group
	var group2 *model.Group

	for i := 0; i < 2; i++ {
		group := &model.Group{
			Name:        model.NewString(model.NewID()),
			DisplayName: model.NewID(),
			Description: model.NewID(),
			Source:      model.GroupSourceLdap,
			RemoteID:    model.NewID(),
		}
		group, err := ss.Group().Create(group)
		require.NoError(t, err)
		switch i {
		case 0:
			group1 = group
		case 1:
			group2 = group
		}
	}

	groups, err := ss.Group().GetByIDs([]string{group1.ID, group2.ID})
	require.NoError(t, err)
	require.Len(t, groups, 2)

	for i := 0; i < 2; i++ {
		require.True(t, (groups[i].ID == group1.ID || groups[i].ID == group2.ID))
	}

	require.True(t, groups[0].ID != groups[1].ID)
}

func testGroupStoreGetByRemoteID(t *testing.T, ss store.Store) {
	// Create a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	d1, err := ss.Group().Create(g1)
	require.NoError(t, err)
	require.Len(t, d1.ID, 26)

	// Get the group
	d2, err := ss.Group().GetByRemoteID(d1.RemoteID, model.GroupSourceLdap)
	require.NoError(t, err)
	require.Equal(t, d1.ID, d2.ID)
	require.Equal(t, *d1.Name, *d2.Name)
	require.Equal(t, d1.DisplayName, d2.DisplayName)
	require.Equal(t, d1.Description, d2.Description)
	require.Equal(t, d1.RemoteID, d2.RemoteID)
	require.Equal(t, d1.CreateAt, d2.CreateAt)
	require.Equal(t, d1.UpdateAt, d2.UpdateAt)
	require.Equal(t, d1.DeleteAt, d2.DeleteAt)

	// Get an invalid group
	_, err = ss.Group().GetByRemoteID(model.NewID(), model.GroupSource("fake"))
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}

func testGroupStoreGetAllByType(t *testing.T, ss store.Store) {
	numGroups := 10

	groups := []*model.Group{}

	// Create groups
	for i := 0; i < numGroups; i++ {
		g := &model.Group{
			Name:        model.NewString(model.NewID()),
			DisplayName: model.NewID(),
			Description: model.NewID(),
			Source:      model.GroupSourceLdap,
			RemoteID:    model.NewID(),
		}
		groups = append(groups, g)
		_, err := ss.Group().Create(g)
		require.NoError(t, err)
	}

	// Returns all the groups
	d1, err := ss.Group().GetAllBySource(model.GroupSourceLdap)
	require.NoError(t, err)
	require.Condition(t, func() bool { return len(d1) >= numGroups })
	for _, expectedGroup := range groups {
		present := false
		for _, dbGroup := range d1 {
			if dbGroup.ID == expectedGroup.ID {
				present = true
				break
			}
		}
		require.True(t, present)
	}
}

func testGroupStoreGetByUser(t *testing.T, ss store.Store) {
	// Save a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	g1, err := ss.Group().Create(g1)
	require.NoError(t, err)

	g2 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	g2, err = ss.Group().Create(g2)
	require.NoError(t, err)

	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	u1, nErr := ss.User().Save(u1)
	require.NoError(t, nErr)

	_, err = ss.Group().UpsertMember(g1.ID, u1.ID)
	require.NoError(t, err)
	_, err = ss.Group().UpsertMember(g2.ID, u1.ID)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	u2, nErr = ss.User().Save(u2)
	require.NoError(t, nErr)

	_, err = ss.Group().UpsertMember(g2.ID, u2.ID)
	require.NoError(t, err)

	groups, err := ss.Group().GetByUser(u1.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(groups))
	found1 := false
	found2 := false
	for _, g := range groups {
		if g.ID == g1.ID {
			found1 = true
		}
		if g.ID == g2.ID {
			found2 = true
		}
	}
	assert.True(t, found1)
	assert.True(t, found2)

	groups, err = ss.Group().GetByUser(u2.ID)
	require.NoError(t, err)
	require.Equal(t, 1, len(groups))
	assert.Equal(t, g2.ID, groups[0].ID)

	groups, err = ss.Group().GetByUser(model.NewID())
	require.NoError(t, err)
	assert.Equal(t, 0, len(groups))
}

func testGroupStoreUpdate(t *testing.T, ss store.Store) {
	// Save a new group
	g1 := &model.Group{
		Name:        model.NewString("g1-test"),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}

	// Create a group
	d1, err := ss.Group().Create(g1)
	require.NoError(t, err)

	// Update happy path
	g1Update := &model.Group{}
	*g1Update = *g1
	g1Update.Name = model.NewString(model.NewID())
	g1Update.DisplayName = model.NewID()
	g1Update.Description = model.NewID()
	g1Update.RemoteID = model.NewID()

	ud1, err := ss.Group().Update(g1Update)
	require.NoError(t, err)
	// Not changed...
	require.Equal(t, d1.ID, ud1.ID)
	require.Equal(t, d1.CreateAt, ud1.CreateAt)
	require.Equal(t, d1.Source, ud1.Source)
	// Still zero...
	require.Zero(t, ud1.DeleteAt)
	// Updated...
	require.Equal(t, *g1Update.Name, *ud1.Name)
	require.Equal(t, g1Update.DisplayName, ud1.DisplayName)
	require.Equal(t, g1Update.Description, ud1.Description)
	require.Equal(t, g1Update.RemoteID, ud1.RemoteID)

	// Requires display name
	data, err := ss.Group().Update(&model.Group{
		ID:          d1.ID,
		Name:        model.NewString(model.NewID()),
		DisplayName: "",
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.Nil(t, data)
	require.Error(t, err)
	var appErr *model.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, appErr.ID, "model.group.display_name.app_error")

	// Create another Group
	g2 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	d2, err := ss.Group().Create(g2)
	require.NoError(t, err)

	// Can't update the name to be a duplicate of an existing group's name
	_, err = ss.Group().Update(&model.Group{
		ID:          d2.ID,
		Name:        g1Update.Name,
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("Group with name %s already exists", *g1Update.Name))

	// Cannot update CreateAt
	someVal := model.GetMillis()
	d1.CreateAt = someVal
	d3, err := ss.Group().Update(d1)
	require.NoError(t, err)
	require.NotEqual(t, someVal, d3.CreateAt)

	// Cannot update DeleteAt to non-zero
	d1.DeleteAt = 1
	_, err = ss.Group().Update(d1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "DeleteAt should be 0 when updating")

	//...except for 0 for DeleteAt
	d1.DeleteAt = 0
	d4, err := ss.Group().Update(d1)
	require.NoError(t, err)
	require.Zero(t, d4.DeleteAt)
}

func testGroupStoreDelete(t *testing.T, ss store.Store) {
	// Save a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}

	d1, err := ss.Group().Create(g1)
	require.NoError(t, err)
	require.Len(t, d1.ID, 26)

	// Check the group is retrievable
	_, err = ss.Group().Get(d1.ID)
	require.NoError(t, err)

	// Get the before count
	d7, err := ss.Group().GetAllBySource(model.GroupSourceLdap)
	require.NoError(t, err)
	beforeCount := len(d7)

	// Delete the group
	_, err = ss.Group().Delete(d1.ID)
	require.NoError(t, err)

	// Check the group is deleted
	d4, err := ss.Group().Get(d1.ID)
	require.NoError(t, err)
	require.NotZero(t, d4.DeleteAt)

	// Check the after count
	d5, err := ss.Group().GetAllBySource(model.GroupSourceLdap)
	require.NoError(t, err)
	afterCount := len(d5)
	require.Condition(t, func() bool { return beforeCount == afterCount+1 })

	// Try and delete a nonexistent group
	_, err = ss.Group().Delete(model.NewID())
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))

	// Cannot delete again
	_, err = ss.Group().Delete(d1.ID)
	require.True(t, errors.As(err, &nfErr))
}

func testGroupGetMemberUsers(t *testing.T, ss store.Store) {
	// Save a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, nErr := ss.User().Save(u1)
	require.NoError(t, nErr)

	_, err = ss.Group().UpsertMember(group.ID, user1.ID)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, nErr := ss.User().Save(u2)
	require.NoError(t, nErr)

	_, err = ss.Group().UpsertMember(group.ID, user2.ID)
	require.NoError(t, err)

	// Check returns members
	groupMembers, err := ss.Group().GetMemberUsers(group.ID)
	require.NoError(t, err)
	require.Equal(t, 2, len(groupMembers))

	// Check madeup id
	groupMembers, err = ss.Group().GetMemberUsers(model.NewID())
	require.NoError(t, err)
	require.Equal(t, 0, len(groupMembers))

	// Delete a member
	_, err = ss.Group().DeleteMember(group.ID, user1.ID)
	require.NoError(t, err)

	// Should not return deleted members
	groupMembers, err = ss.Group().GetMemberUsers(group.ID)
	require.NoError(t, err)
	require.Equal(t, 1, len(groupMembers))
}

func testGroupGetMemberUsersPage(t *testing.T, ss store.Store) {
	// Save a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, nErr := ss.User().Save(u1)
	require.NoError(t, nErr)

	_, err = ss.Group().UpsertMember(group.ID, user1.ID)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, nErr := ss.User().Save(u2)
	require.NoError(t, nErr)

	_, err = ss.Group().UpsertMember(group.ID, user2.ID)
	require.NoError(t, err)

	u3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, nErr := ss.User().Save(u3)
	require.NoError(t, nErr)

	_, err = ss.Group().UpsertMember(group.ID, user3.ID)
	require.NoError(t, err)

	// Check returns members
	groupMembers, err := ss.Group().GetMemberUsersPage(group.ID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, 3, len(groupMembers))

	// Check page 1
	groupMembers, err = ss.Group().GetMemberUsersPage(group.ID, 0, 2)
	require.NoError(t, err)
	require.Equal(t, 2, len(groupMembers))
	require.Equal(t, user3.ID, groupMembers[0].ID)
	require.Equal(t, user2.ID, groupMembers[1].ID)

	// Check page 2
	groupMembers, err = ss.Group().GetMemberUsersPage(group.ID, 1, 2)
	require.NoError(t, err)
	require.Equal(t, 1, len(groupMembers))
	require.Equal(t, user1.ID, groupMembers[0].ID)

	// Check madeup id
	groupMembers, err = ss.Group().GetMemberUsersPage(model.NewID(), 0, 100)
	require.NoError(t, err)
	require.Equal(t, 0, len(groupMembers))

	// Delete a member
	_, err = ss.Group().DeleteMember(group.ID, user1.ID)
	require.NoError(t, err)

	// Should not return deleted members
	groupMembers, err = ss.Group().GetMemberUsersPage(group.ID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, 2, len(groupMembers))
}

func testGroupGetMemberUsersInTeam(t *testing.T, ss store.Store) {
	// Save a team
	team := &model.Team{
		DisplayName: "Name",
		Description: "Some description",
		CompanyName: "Some company name",
		Name:        "z-z-" + model.NewID() + "a",
		Email:       "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:        model.TeamOpen,
	}
	team, err := ss.Team().Save(team)
	require.NoError(t, err)

	// Save a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(u1)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group.ID, user1.ID)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err := ss.User().Save(u2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group.ID, user2.ID)
	require.NoError(t, err)

	u3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, err := ss.User().Save(u3)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group.ID, user3.ID)
	require.NoError(t, err)

	// returns no members when team does not exist
	groupMembers, err := ss.Group().GetMemberUsersInTeam(group.ID, "non-existent-channel-id")
	require.NoError(t, err)
	require.Equal(t, 0, len(groupMembers))

	// returns no members when group has no members in the team
	groupMembers, err = ss.Group().GetMemberUsersInTeam(group.ID, team.ID)
	require.NoError(t, err)
	require.Equal(t, 0, len(groupMembers))

	m1 := &model.TeamMember{TeamID: team.ID, UserID: user1.ID}
	_, nErr := ss.Team().SaveMember(m1, -1)
	require.NoError(t, nErr)

	// returns single member in team
	groupMembers, err = ss.Group().GetMemberUsersInTeam(group.ID, team.ID)
	require.NoError(t, err)
	require.Equal(t, 1, len(groupMembers))

	m2 := &model.TeamMember{TeamID: team.ID, UserID: user2.ID}
	m3 := &model.TeamMember{TeamID: team.ID, UserID: user3.ID}
	_, nErr = ss.Team().SaveMember(m2, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(m3, -1)
	require.NoError(t, nErr)

	// returns all members when all members are in team
	groupMembers, err = ss.Group().GetMemberUsersInTeam(group.ID, team.ID)
	require.NoError(t, err)
	require.Equal(t, 3, len(groupMembers))
}

func testGroupGetMemberUsersNotInChannel(t *testing.T, ss store.Store) {
	// Save a team
	team := &model.Team{
		DisplayName: "Name",
		Description: "Some description",
		CompanyName: "Some company name",
		Name:        "z-z-" + model.NewID() + "a",
		Email:       "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:        model.TeamOpen,
	}
	team, err := ss.Team().Save(team)
	require.NoError(t, err)

	// Save a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(u1)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group.ID, user1.ID)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err := ss.User().Save(u2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group.ID, user2.ID)
	require.NoError(t, err)

	u3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, err := ss.User().Save(u3)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group.ID, user3.ID)
	require.NoError(t, err)

	// Create Channel
	channel := &model.Channel{
		TeamID:      team.ID,
		DisplayName: "Channel",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen, // Query does not look at type so this shouldn't matter.
	}
	channel, nErr := ss.Channel().Save(channel, 9999)
	require.NoError(t, nErr)

	// returns no members when channel does not exist
	groupMembers, err := ss.Group().GetMemberUsersNotInChannel(group.ID, "non-existent-channel-id")
	require.NoError(t, err)
	require.Equal(t, 0, len(groupMembers))

	// returns no members when group has no members in the team that the channel belongs to
	groupMembers, err = ss.Group().GetMemberUsersNotInChannel(group.ID, channel.ID)
	require.NoError(t, err)
	require.Equal(t, 0, len(groupMembers))

	m1 := &model.TeamMember{TeamID: team.ID, UserID: user1.ID}
	_, nErr = ss.Team().SaveMember(m1, -1)
	require.NoError(t, nErr)

	// returns single member in team and not in channel
	groupMembers, err = ss.Group().GetMemberUsersNotInChannel(group.ID, channel.ID)
	require.NoError(t, err)
	require.Equal(t, 1, len(groupMembers))

	m2 := &model.TeamMember{TeamID: team.ID, UserID: user2.ID}
	m3 := &model.TeamMember{TeamID: team.ID, UserID: user3.ID}
	_, nErr = ss.Team().SaveMember(m2, -1)
	require.NoError(t, nErr)
	_, nErr = ss.Team().SaveMember(m3, -1)
	require.NoError(t, nErr)

	// returns all members when all members are in team and not in channel
	groupMembers, err = ss.Group().GetMemberUsersNotInChannel(group.ID, channel.ID)
	require.NoError(t, err)
	require.Equal(t, 3, len(groupMembers))

	cm1 := &model.ChannelMember{
		ChannelID:   channel.ID,
		UserID:      user1.ID,
		SchemeGuest: false,
		SchemeUser:  true,
		SchemeAdmin: false,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	_, err = ss.Channel().SaveMember(cm1)
	require.NoError(t, err)

	// returns both members not yet added to channel
	groupMembers, err = ss.Group().GetMemberUsersNotInChannel(group.ID, channel.ID)
	require.NoError(t, err)
	require.Equal(t, 2, len(groupMembers))

	cm2 := &model.ChannelMember{
		ChannelID:   channel.ID,
		UserID:      user2.ID,
		SchemeGuest: false,
		SchemeUser:  true,
		SchemeAdmin: false,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}
	cm3 := &model.ChannelMember{
		ChannelID:   channel.ID,
		UserID:      user3.ID,
		SchemeGuest: false,
		SchemeUser:  true,
		SchemeAdmin: false,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	}

	_, err = ss.Channel().SaveMember(cm2)
	require.NoError(t, err)
	_, err = ss.Channel().SaveMember(cm3)
	require.NoError(t, err)

	// returns none when all members have been added to team and channel
	groupMembers, err = ss.Group().GetMemberUsersNotInChannel(group.ID, channel.ID)
	require.NoError(t, err)
	require.Equal(t, 0, len(groupMembers))
}

func testUpsertMember(t *testing.T, ss store.Store) {
	// Create group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	// Create user
	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user, nErr := ss.User().Save(u1)
	require.NoError(t, nErr)

	// Happy path
	d2, err := ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, d2.GroupID, group.ID)
	require.Equal(t, d2.UserID, user.ID)
	require.NotZero(t, d2.CreateAt)
	require.Zero(t, d2.DeleteAt)

	// Duplicate composite key (GroupId, UserId)
	// Ensure new CreateAt > previous CreateAt for the same (groupId, userId)
	time.Sleep(1 * time.Millisecond)
	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)

	// Invalid GroupId
	_, err = ss.Group().UpsertMember(model.NewID(), user.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get UserGroup with")

	// Restores a deleted member
	// Ensure new CreateAt > previous CreateAt for the same (groupId, userId)
	time.Sleep(1 * time.Millisecond)
	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)

	_, err = ss.Group().DeleteMember(group.ID, user.ID)
	require.NoError(t, err)

	groupMembers, err := ss.Group().GetMemberUsers(group.ID)
	beforeRestoreCount := len(groupMembers)

	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)

	groupMembers, err = ss.Group().GetMemberUsers(group.ID)
	afterRestoreCount := len(groupMembers)

	require.Equal(t, beforeRestoreCount+1, afterRestoreCount)
}

func testGroupDeleteMember(t *testing.T, ss store.Store) {
	// Create group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	// Create user
	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user, nErr := ss.User().Save(u1)
	require.NoError(t, nErr)

	// Create member
	d1, err := ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)

	// Happy path
	d2, err := ss.Group().DeleteMember(group.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, d2.GroupID, group.ID)
	require.Equal(t, d2.UserID, user.ID)
	require.Equal(t, d2.CreateAt, d1.CreateAt)
	require.NotZero(t, d2.DeleteAt)

	// Delete an already deleted member
	_, err = ss.Group().DeleteMember(group.ID, user.ID)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))

	// Delete with non-existent User
	_, err = ss.Group().DeleteMember(group.ID, model.NewID())
	require.True(t, errors.As(err, &nfErr))

	// Delete non-existent Group
	_, err = ss.Group().DeleteMember(model.NewID(), group.ID)
	require.True(t, errors.As(err, &nfErr))
}

func testGroupPermanentDeleteMembersByUser(t *testing.T, ss store.Store) {
	var g *model.Group
	var groups []*model.Group
	numberOfGroups := 5

	for i := 0; i < numberOfGroups; i++ {
		g = &model.Group{
			Name:        model.NewString(model.NewID()),
			DisplayName: model.NewID(),
			Source:      model.GroupSourceLdap,
			RemoteID:    model.NewID(),
		}
		group, err := ss.Group().Create(g)
		groups = append(groups, group)
		require.NoError(t, err)
	}

	// Create user
	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user, err := ss.User().Save(u1)
	require.NoError(t, err)

	// Create members
	for _, group := range groups {
		_, err = ss.Group().UpsertMember(group.ID, user.ID)
		require.NoError(t, err)
	}

	// Happy path
	err = ss.Group().PermanentDeleteMembersByUser(user.ID)
	require.NoError(t, err)
}

func testCreateGroupSyncable(t *testing.T, ss store.Store) {
	// Invalid GroupID
	_, err := ss.Group().CreateGroupSyncable(model.NewGroupTeam("x", model.NewID(), false))
	var appErr *model.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, appErr.ID, "model.group_syncable.group_id.app_error")

	// Create Group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	// Create Team
	t1 := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team, nErr := ss.Team().Save(t1)
	require.NoError(t, nErr)

	// New GroupSyncable, happy path
	gt1 := model.NewGroupTeam(group.ID, team.ID, false)
	d1, err := ss.Group().CreateGroupSyncable(gt1)
	require.NoError(t, err)
	require.Equal(t, gt1.SyncableID, d1.SyncableID)
	require.Equal(t, gt1.GroupID, d1.GroupID)
	require.Equal(t, gt1.AutoAdd, d1.AutoAdd)
	require.NotZero(t, d1.CreateAt)
	require.Zero(t, d1.DeleteAt)
}

func testGetGroupSyncable(t *testing.T, ss store.Store) {
	// Create a group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	// Create Team
	t1 := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team, nErr := ss.Team().Save(t1)
	require.NoError(t, nErr)

	// Create GroupSyncable
	gt1 := model.NewGroupTeam(group.ID, team.ID, false)
	groupTeam, err := ss.Group().CreateGroupSyncable(gt1)
	require.NoError(t, err)

	// Get GroupSyncable
	dgt, err := ss.Group().GetGroupSyncable(groupTeam.GroupID, groupTeam.SyncableID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.Equal(t, gt1.GroupID, dgt.GroupID)
	require.Equal(t, gt1.SyncableID, dgt.SyncableID)
	require.Equal(t, gt1.AutoAdd, dgt.AutoAdd)
	require.NotZero(t, gt1.CreateAt)
	require.NotZero(t, gt1.UpdateAt)
	require.Zero(t, gt1.DeleteAt)
}

func testGetAllGroupSyncablesByGroup(t *testing.T, ss store.Store) {
	numGroupSyncables := 10

	// Create group
	g := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Description: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g)
	require.NoError(t, err)

	groupTeams := []*model.GroupSyncable{}

	// Create groupTeams
	for i := 0; i < numGroupSyncables; i++ {
		// Create Team
		t1 := &model.Team{
			DisplayName:     "Name",
			Description:     "Some description",
			CompanyName:     "Some company name",
			AllowOpenInvite: false,
			InviteID:        "inviteid0",
			Name:            "z-z-" + model.NewID() + "a",
			Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
			Type:            model.TeamOpen,
		}
		var team *model.Team
		team, nErr := ss.Team().Save(t1)
		require.NoError(t, nErr)

		// create groupteam
		var groupTeam *model.GroupSyncable
		gt := model.NewGroupTeam(group.ID, team.ID, false)
		gt.SchemeAdmin = true
		groupTeam, err = ss.Group().CreateGroupSyncable(gt)
		require.NoError(t, err)
		groupTeams = append(groupTeams, groupTeam)
	}

	// Returns all the group teams
	d1, err := ss.Group().GetAllGroupSyncablesByGroupID(group.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.Condition(t, func() bool { return len(d1) >= numGroupSyncables })
	for _, expectedGroupTeam := range groupTeams {
		present := false
		for _, dbGroupTeam := range d1 {
			if dbGroupTeam.GroupID == expectedGroupTeam.GroupID && dbGroupTeam.SyncableID == expectedGroupTeam.SyncableID {
				require.True(t, dbGroupTeam.SchemeAdmin)
				present = true
				break
			}
		}
		require.True(t, present)
	}
}

func testUpdateGroupSyncable(t *testing.T, ss store.Store) {
	// Create Group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	// Create Team
	t1 := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team, nErr := ss.Team().Save(t1)
	require.NoError(t, nErr)

	// New GroupSyncable, happy path
	gt1 := model.NewGroupTeam(group.ID, team.ID, false)
	d1, err := ss.Group().CreateGroupSyncable(gt1)
	require.NoError(t, err)

	// Update existing group team
	gt1.AutoAdd = true
	d2, err := ss.Group().UpdateGroupSyncable(gt1)
	require.NoError(t, err)
	require.True(t, d2.AutoAdd)

	// Non-existent Group
	gt2 := model.NewGroupTeam(model.NewID(), team.ID, false)
	_, err = ss.Group().UpdateGroupSyncable(gt2)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))

	// Non-existent Team
	gt3 := model.NewGroupTeam(group.ID, model.NewID(), false)
	_, err = ss.Group().UpdateGroupSyncable(gt3)
	require.True(t, errors.As(err, &nfErr))

	// Cannot update CreateAt or DeleteAt
	origCreateAt := d1.CreateAt
	d1.CreateAt = model.GetMillis()
	d1.AutoAdd = true
	d3, err := ss.Group().UpdateGroupSyncable(d1)
	require.NoError(t, err)
	require.Equal(t, origCreateAt, d3.CreateAt)

	// Cannot update DeleteAt to arbitrary value
	d1.DeleteAt = 1
	_, err = ss.Group().UpdateGroupSyncable(d1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "DeleteAt should be 0 when updating")

	// Can update DeleteAt to 0
	d1.DeleteAt = 0
	d4, err := ss.Group().UpdateGroupSyncable(d1)
	require.NoError(t, err)
	require.Zero(t, d4.DeleteAt)
}

func testDeleteGroupSyncable(t *testing.T, ss store.Store) {
	// Create Group
	g1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(g1)
	require.NoError(t, err)

	// Create Team
	t1 := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team, nErr := ss.Team().Save(t1)
	require.NoError(t, nErr)

	// Create GroupSyncable
	gt1 := model.NewGroupTeam(group.ID, team.ID, false)
	groupTeam, err := ss.Group().CreateGroupSyncable(gt1)
	require.NoError(t, err)

	// Non-existent Group
	_, err = ss.Group().DeleteGroupSyncable(model.NewID(), groupTeam.SyncableID, model.GroupSyncableTypeTeam)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))

	// Non-existent Team
	_, err = ss.Group().DeleteGroupSyncable(groupTeam.GroupID, model.NewID(), model.GroupSyncableTypeTeam)
	require.True(t, errors.As(err, &nfErr))

	// Happy path...
	d1, err := ss.Group().DeleteGroupSyncable(groupTeam.GroupID, groupTeam.SyncableID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.NotZero(t, d1.DeleteAt)
	require.Equal(t, d1.GroupID, groupTeam.GroupID)
	require.Equal(t, d1.SyncableID, groupTeam.SyncableID)
	require.Equal(t, d1.AutoAdd, groupTeam.AutoAdd)
	require.Equal(t, d1.CreateAt, groupTeam.CreateAt)
	require.Condition(t, func() bool { return d1.UpdateAt > groupTeam.UpdateAt })

	// Record already deleted
	_, err = ss.Group().DeleteGroupSyncable(d1.GroupID, d1.SyncableID, d1.Type)
	require.Error(t, err)
	var invErr *store.ErrInvalidInput
	require.True(t, errors.As(err, &invErr))
}

func testTeamMembersToAdd(t *testing.T, ss store.Store) {
	// Create Group
	group, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "TeamMembersToAdd Test Group",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	require.NoError(t, err)

	// Create User
	user := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user, nErr := ss.User().Save(user)
	require.NoError(t, nErr)

	// Create GroupMember
	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)

	// Create Team
	team := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team, nErr = ss.Team().Save(team)
	require.NoError(t, nErr)

	// Create GroupTeam
	syncable, err := ss.Group().CreateGroupSyncable(model.NewGroupTeam(group.ID, team.ID, true))
	require.NoError(t, err)

	// Time before syncable was created
	teamMembers, err := ss.Group().TeamMembersToAdd(syncable.CreateAt-1, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)
	require.Equal(t, user.ID, teamMembers[0].UserID)
	require.Equal(t, team.ID, teamMembers[0].TeamID)

	// Time after syncable was created
	teamMembers, err = ss.Group().TeamMembersToAdd(syncable.CreateAt+1, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// Delete and restore GroupMember should return result
	_, err = ss.Group().DeleteMember(group.ID, user.ID)
	require.NoError(t, err)
	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(syncable.CreateAt+1, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)

	pristineSyncable := *syncable

	_, err = ss.Group().UpdateGroupSyncable(syncable)
	require.NoError(t, err)

	// Time before syncable was updated
	teamMembers, err = ss.Group().TeamMembersToAdd(syncable.UpdateAt-1, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)
	require.Equal(t, user.ID, teamMembers[0].UserID)
	require.Equal(t, team.ID, teamMembers[0].TeamID)

	// Time after syncable was updated
	teamMembers, err = ss.Group().TeamMembersToAdd(syncable.UpdateAt+1, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// Only includes if auto-add
	syncable.AutoAdd = false
	_, err = ss.Group().UpdateGroupSyncable(syncable)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// reset state of syncable and verify
	_, err = ss.Group().UpdateGroupSyncable(&pristineSyncable)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)

	// No result if Group deleted
	_, err = ss.Group().Delete(group.ID)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// reset state of group and verify
	group.DeleteAt = 0
	_, err = ss.Group().Update(group)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)

	// No result if Team deleted
	team.DeleteAt = model.GetMillis()
	team, nErr = ss.Team().Update(team)
	require.NoError(t, nErr)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// reset state of team and verify
	team.DeleteAt = 0
	team, nErr = ss.Team().Update(team)
	require.NoError(t, nErr)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)

	// No result if GroupTeam deleted
	_, err = ss.Group().DeleteGroupSyncable(group.ID, team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// reset GroupTeam and verify
	_, err = ss.Group().UpdateGroupSyncable(&pristineSyncable)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)

	// No result if GroupMember deleted
	_, err = ss.Group().DeleteMember(group.ID, user.ID)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// restore group member and verify
	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)

	// adding team membership stops returning result
	_, nErr = ss.Team().SaveMember(&model.TeamMember{
		TeamID: team.ID,
		UserID: user.ID,
	}, 999)
	require.NoError(t, nErr)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// Leaving Team should still not return result
	_, nErr = ss.Team().UpdateMember(&model.TeamMember{
		TeamID:   team.ID,
		UserID:   user.ID,
		DeleteAt: model.GetMillis(),
	})
	require.NoError(t, nErr)
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, teamMembers)

	// If includeRemovedMembers is set to true, removed members should be added back in
	teamMembers, err = ss.Group().TeamMembersToAdd(0, nil, true)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)
}

func testTeamMembersToAddSingleTeam(t *testing.T, ss store.Store) {
	group1, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "TeamMembersToAdd Test Group",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	require.NoError(t, err)

	group2, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "TeamMembersToAdd Test Group",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	require.NoError(t, err)

	user1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, nErr := ss.User().Save(user1)
	require.NoError(t, nErr)

	user2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, nErr = ss.User().Save(user2)
	require.NoError(t, nErr)

	user3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, nErr = ss.User().Save(user3)
	require.NoError(t, nErr)

	for _, user := range []*model.User{user1, user2} {
		_, err = ss.Group().UpsertMember(group1.ID, user.ID)
		require.NoError(t, err)
	}
	_, err = ss.Group().UpsertMember(group2.ID, user3.ID)
	require.NoError(t, err)

	team1 := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team1, nErr = ss.Team().Save(team1)
	require.NoError(t, nErr)

	team2 := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team2, nErr = ss.Team().Save(team2)
	require.NoError(t, nErr)

	_, err = ss.Group().CreateGroupSyncable(model.NewGroupTeam(group1.ID, team1.ID, true))
	require.NoError(t, err)

	_, err = ss.Group().CreateGroupSyncable(model.NewGroupTeam(group2.ID, team2.ID, true))
	require.NoError(t, err)

	teamMembers, err := ss.Group().TeamMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 3)

	teamMembers, err = ss.Group().TeamMembersToAdd(0, &team1.ID, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 2)

	teamMembers, err = ss.Group().TeamMembersToAdd(0, &team2.ID, false)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)
}

func testChannelMembersToAdd(t *testing.T, ss store.Store) {
	// Create Group
	group, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "ChannelMembersToAdd Test Group",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	require.NoError(t, err)

	// Create User
	user := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user, nErr := ss.User().Save(user)
	require.NoError(t, nErr)

	// Create GroupMember
	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)

	// Create Channel
	channel := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "A Name",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen, // Query does not look at type so this shouldn't matter.
	}
	channel, nErr = ss.Channel().Save(channel, 9999)
	require.NoError(t, nErr)

	// Create GroupChannel
	syncable, err := ss.Group().CreateGroupSyncable(model.NewGroupChannel(group.ID, channel.ID, true))
	require.NoError(t, err)

	// Time before syncable was created
	channelMembers, err := ss.Group().ChannelMembersToAdd(syncable.CreateAt-1, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)
	require.Equal(t, user.ID, channelMembers[0].UserID)
	require.Equal(t, channel.ID, channelMembers[0].ChannelID)

	// Time after syncable was created
	channelMembers, err = ss.Group().ChannelMembersToAdd(syncable.CreateAt+1, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// Delete and restore GroupMember should return result
	_, err = ss.Group().DeleteMember(group.ID, user.ID)
	require.NoError(t, err)
	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(syncable.CreateAt+1, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)

	pristineSyncable := *syncable

	_, err = ss.Group().UpdateGroupSyncable(syncable)
	require.NoError(t, err)

	// Time before syncable was updated
	channelMembers, err = ss.Group().ChannelMembersToAdd(syncable.UpdateAt-1, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)
	require.Equal(t, user.ID, channelMembers[0].UserID)
	require.Equal(t, channel.ID, channelMembers[0].ChannelID)

	// Time after syncable was updated
	channelMembers, err = ss.Group().ChannelMembersToAdd(syncable.UpdateAt+1, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// Only includes if auto-add
	syncable.AutoAdd = false
	_, err = ss.Group().UpdateGroupSyncable(syncable)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// reset state of syncable and verify
	_, err = ss.Group().UpdateGroupSyncable(&pristineSyncable)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)

	// No result if Group deleted
	_, err = ss.Group().Delete(group.ID)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// reset state of group and verify
	group.DeleteAt = 0
	_, err = ss.Group().Update(group)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)

	// No result if Channel deleted
	nErr = ss.Channel().Delete(channel.ID, model.GetMillis())
	require.NoError(t, nErr)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// reset state of channel and verify
	channel.DeleteAt = 0
	_, nErr = ss.Channel().Update(channel)
	require.NoError(t, nErr)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)

	// No result if GroupChannel deleted
	_, err = ss.Group().DeleteGroupSyncable(group.ID, channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// reset GroupChannel and verify
	_, err = ss.Group().UpdateGroupSyncable(&pristineSyncable)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)

	// No result if GroupMember deleted
	_, err = ss.Group().DeleteMember(group.ID, user.ID)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// restore group member and verify
	_, err = ss.Group().UpsertMember(group.ID, user.ID)
	require.NoError(t, err)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)

	// Adding Channel (ChannelMemberHistory) should stop returning result
	nErr = ss.ChannelMemberHistory().LogJoinEvent(user.ID, channel.ID, model.GetMillis())
	require.NoError(t, nErr)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// Leaving Channel (ChannelMemberHistory) should still not return result
	nErr = ss.ChannelMemberHistory().LogLeaveEvent(user.ID, channel.ID, model.GetMillis())
	require.NoError(t, nErr)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Empty(t, channelMembers)

	// Purging ChannelMemberHistory re-returns the result
	_, _, nErr = ss.ChannelMemberHistory().PermanentDeleteBatchForRetentionPolicies(
		0, model.GetMillis()+1, 100, model.RetentionPolicyCursor{})
	require.NoError(t, nErr)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)

	// If includeRemovedMembers is set to true, removed members should be added back in
	nErr = ss.ChannelMemberHistory().LogLeaveEvent(user.ID, channel.ID, model.GetMillis())
	require.NoError(t, nErr)
	channelMembers, err = ss.Group().ChannelMembersToAdd(0, nil, true)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)
}

func testChannelMembersToAddSingleChannel(t *testing.T, ss store.Store) {
	group1, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "TeamMembersToAdd Test Group",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	require.NoError(t, err)

	group2, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "TeamMembersToAdd Test Group",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	require.NoError(t, err)

	user1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, nErr := ss.User().Save(user1)
	require.NoError(t, nErr)

	user2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, nErr = ss.User().Save(user2)
	require.NoError(t, nErr)

	user3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, nErr = ss.User().Save(user3)
	require.NoError(t, nErr)

	for _, user := range []*model.User{user1, user2} {
		_, err = ss.Group().UpsertMember(group1.ID, user.ID)
		require.NoError(t, err)
	}
	_, err = ss.Group().UpsertMember(group2.ID, user3.ID)
	require.NoError(t, err)

	channel1 := &model.Channel{
		DisplayName: "Name",
		Name:        "z-z-" + model.NewID() + "a",
		Type:        model.ChannelTypeOpen,
	}
	channel1, nErr = ss.Channel().Save(channel1, 999)
	require.NoError(t, nErr)

	channel2 := &model.Channel{
		DisplayName: "Name",
		Name:        "z-z-" + model.NewID() + "a",
		Type:        model.ChannelTypeOpen,
	}
	channel2, nErr = ss.Channel().Save(channel2, 999)
	require.NoError(t, nErr)

	_, err = ss.Group().CreateGroupSyncable(model.NewGroupChannel(group1.ID, channel1.ID, true))
	require.NoError(t, err)

	_, err = ss.Group().CreateGroupSyncable(model.NewGroupChannel(group2.ID, channel2.ID, true))
	require.NoError(t, err)

	channelMembers, err := ss.Group().ChannelMembersToAdd(0, nil, false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(channelMembers), 3)

	channelMembers, err = ss.Group().ChannelMembersToAdd(0, &channel1.ID, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 2)

	channelMembers, err = ss.Group().ChannelMembersToAdd(0, &channel2.ID, false)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)
}

func testTeamMembersToRemove(t *testing.T, ss store.Store) {
	data := pendingMemberRemovalsDataSetup(t, ss)

	// one result when both users are in the group (for user C)
	teamMembers, err := ss.Group().TeamMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)
	require.Equal(t, data.UserC.ID, teamMembers[0].UserID)

	_, err = ss.Group().DeleteMember(data.Group.ID, data.UserB.ID)
	require.NoError(t, err)

	// user b and c should now be returned
	teamMembers, err = ss.Group().TeamMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, teamMembers, 2)

	var userIDs []string
	for _, item := range teamMembers {
		userIDs = append(userIDs, item.UserID)
	}
	require.Contains(t, userIDs, data.UserB.ID)
	require.Contains(t, userIDs, data.UserC.ID)
	require.Equal(t, data.ConstrainedTeam.ID, teamMembers[0].TeamID)
	require.Equal(t, data.ConstrainedTeam.ID, teamMembers[1].TeamID)

	_, err = ss.Group().DeleteMember(data.Group.ID, data.UserA.ID)
	require.NoError(t, err)

	teamMembers, err = ss.Group().TeamMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, teamMembers, 3)

	// Make one of them a bot
	teamMembers, err = ss.Group().TeamMembersToRemove(nil)
	require.NoError(t, err)
	teamMember := teamMembers[0]
	bot := &model.Bot{
		UserID:      teamMember.UserID,
		Username:    "un_" + model.NewID(),
		DisplayName: "dn_" + model.NewID(),
		OwnerID:     teamMember.UserID,
	}
	bot, nErr := ss.Bot().Save(bot)
	require.NoError(t, nErr)

	// verify that bot is not returned in results
	teamMembers, err = ss.Group().TeamMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, teamMembers, 2)

	// delete the bot
	nErr = ss.Bot().PermanentDelete(bot.UserID)
	require.NoError(t, nErr)

	// Should be back to 3 users
	teamMembers, err = ss.Group().TeamMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, teamMembers, 3)

	// add users back to groups
	res := ss.Team().RemoveMember(data.ConstrainedTeam.ID, data.UserA.ID)
	require.NoError(t, res)
	res = ss.Team().RemoveMember(data.ConstrainedTeam.ID, data.UserB.ID)
	require.NoError(t, res)
	res = ss.Team().RemoveMember(data.ConstrainedTeam.ID, data.UserC.ID)
	require.NoError(t, res)
	nErr = ss.Channel().RemoveMember(data.ConstrainedChannel.ID, data.UserA.ID)
	require.NoError(t, nErr)
	nErr = ss.Channel().RemoveMember(data.ConstrainedChannel.ID, data.UserB.ID)
	require.NoError(t, nErr)
	nErr = ss.Channel().RemoveMember(data.ConstrainedChannel.ID, data.UserC.ID)
	require.NoError(t, nErr)
}

func testTeamMembersToRemoveSingleTeam(t *testing.T, ss store.Store) {
	user1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(user1)
	require.NoError(t, err)

	user2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err = ss.User().Save(user2)
	require.NoError(t, err)

	user3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, err = ss.User().Save(user3)
	require.NoError(t, err)

	team1 := &model.Team{
		DisplayName:      "Name",
		Description:      "Some description",
		CompanyName:      "Some company name",
		AllowOpenInvite:  false,
		InviteID:         "inviteid0",
		Name:             "z-z-" + model.NewID() + "a",
		Email:            "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:             model.TeamOpen,
		GroupConstrained: model.NewBool(true),
	}
	team1, nErr := ss.Team().Save(team1)
	require.NoError(t, nErr)

	team2 := &model.Team{
		DisplayName:      "Name",
		Description:      "Some description",
		CompanyName:      "Some company name",
		AllowOpenInvite:  false,
		InviteID:         "inviteid0",
		Name:             "z-z-" + model.NewID() + "a",
		Email:            "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:             model.TeamOpen,
		GroupConstrained: model.NewBool(true),
	}
	team2, nErr = ss.Team().Save(team2)
	require.NoError(t, nErr)

	for _, user := range []*model.User{user1, user2} {
		_, nErr = ss.Team().SaveMember(&model.TeamMember{
			TeamID: team1.ID,
			UserID: user.ID,
		}, 999)
		require.NoError(t, nErr)
	}

	_, nErr = ss.Team().SaveMember(&model.TeamMember{
		TeamID: team2.ID,
		UserID: user3.ID,
	}, 999)
	require.NoError(t, nErr)

	teamMembers, err := ss.Group().TeamMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, teamMembers, 3)

	teamMembers, err = ss.Group().TeamMembersToRemove(&team1.ID)
	require.NoError(t, err)
	require.Len(t, teamMembers, 2)

	teamMembers, err = ss.Group().TeamMembersToRemove(&team2.ID)
	require.NoError(t, err)
	require.Len(t, teamMembers, 1)
}

func testChannelMembersToRemove(t *testing.T, ss store.Store) {
	data := pendingMemberRemovalsDataSetup(t, ss)

	// one result when both users are in the group (for user C)
	channelMembers, err := ss.Group().ChannelMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)
	require.Equal(t, data.UserC.ID, channelMembers[0].UserID)

	_, err = ss.Group().DeleteMember(data.Group.ID, data.UserB.ID)
	require.NoError(t, err)

	// user b and c should now be returned
	channelMembers, err = ss.Group().ChannelMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, channelMembers, 2)

	var userIDs []string
	for _, item := range channelMembers {
		userIDs = append(userIDs, item.UserID)
	}
	require.Contains(t, userIDs, data.UserB.ID)
	require.Contains(t, userIDs, data.UserC.ID)
	require.Equal(t, data.ConstrainedChannel.ID, channelMembers[0].ChannelID)
	require.Equal(t, data.ConstrainedChannel.ID, channelMembers[1].ChannelID)

	_, err = ss.Group().DeleteMember(data.Group.ID, data.UserA.ID)
	require.NoError(t, err)

	channelMembers, err = ss.Group().ChannelMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, channelMembers, 3)

	// Make one of them a bot
	channelMembers, err = ss.Group().ChannelMembersToRemove(nil)
	require.NoError(t, err)
	channelMember := channelMembers[0]
	bot := &model.Bot{
		UserID:      channelMember.UserID,
		Username:    "un_" + model.NewID(),
		DisplayName: "dn_" + model.NewID(),
		OwnerID:     channelMember.UserID,
	}
	bot, nErr := ss.Bot().Save(bot)
	require.NoError(t, nErr)

	// verify that bot is not returned in results
	channelMembers, err = ss.Group().ChannelMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, channelMembers, 2)

	// delete the bot
	nErr = ss.Bot().PermanentDelete(bot.UserID)
	require.NoError(t, nErr)

	// Should be back to 3 users
	channelMembers, err = ss.Group().ChannelMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, channelMembers, 3)

	// add users back to groups
	res := ss.Team().RemoveMember(data.ConstrainedTeam.ID, data.UserA.ID)
	require.NoError(t, res)
	res = ss.Team().RemoveMember(data.ConstrainedTeam.ID, data.UserB.ID)
	require.NoError(t, res)
	res = ss.Team().RemoveMember(data.ConstrainedTeam.ID, data.UserC.ID)
	require.NoError(t, res)
	nErr = ss.Channel().RemoveMember(data.ConstrainedChannel.ID, data.UserA.ID)
	require.NoError(t, nErr)
	nErr = ss.Channel().RemoveMember(data.ConstrainedChannel.ID, data.UserB.ID)
	require.NoError(t, nErr)
	nErr = ss.Channel().RemoveMember(data.ConstrainedChannel.ID, data.UserC.ID)
	require.NoError(t, nErr)
}

func testChannelMembersToRemoveSingleChannel(t *testing.T, ss store.Store) {
	user1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(user1)
	require.NoError(t, err)

	user2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err = ss.User().Save(user2)
	require.NoError(t, err)

	user3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, err = ss.User().Save(user3)
	require.NoError(t, err)

	channel1 := &model.Channel{
		DisplayName:      "Name",
		Name:             "z-z-" + model.NewID() + "a",
		Type:             model.ChannelTypeOpen,
		GroupConstrained: model.NewBool(true),
	}
	channel1, nErr := ss.Channel().Save(channel1, 999)
	require.NoError(t, nErr)

	channel2 := &model.Channel{
		DisplayName:      "Name",
		Name:             "z-z-" + model.NewID() + "a",
		Type:             model.ChannelTypeOpen,
		GroupConstrained: model.NewBool(true),
	}
	channel2, nErr = ss.Channel().Save(channel2, 999)
	require.NoError(t, nErr)

	for _, user := range []*model.User{user1, user2} {
		_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
			ChannelID:   channel1.ID,
			UserID:      user.ID,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, nErr)
	}

	_, nErr = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   channel2.ID,
		UserID:      user3.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, nErr)

	channelMembers, err := ss.Group().ChannelMembersToRemove(nil)
	require.NoError(t, err)
	require.Len(t, channelMembers, 3)

	channelMembers, err = ss.Group().ChannelMembersToRemove(&channel1.ID)
	require.NoError(t, err)
	require.Len(t, channelMembers, 2)

	channelMembers, err = ss.Group().ChannelMembersToRemove(&channel2.ID)
	require.NoError(t, err)
	require.Len(t, channelMembers, 1)
}

type removalsData struct {
	UserA                *model.User
	UserB                *model.User
	UserC                *model.User
	ConstrainedChannel   *model.Channel
	UnconstrainedChannel *model.Channel
	ConstrainedTeam      *model.Team
	UnconstrainedTeam    *model.Team
	Group                *model.Group
}

func pendingMemberRemovalsDataSetup(t *testing.T, ss store.Store) *removalsData {
	// create group
	group, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: "Pending[Channel|Team]MemberRemovals Test Group",
		RemoteID:    model.NewID(),
		Source:      model.GroupSourceLdap,
	})
	require.NoError(t, err)

	// create users
	// userA will get removed from the group
	userA := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	userA, nErr := ss.User().Save(userA)
	require.NoError(t, nErr)

	// userB will not get removed from the group
	userB := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	userB, nErr = ss.User().Save(userB)
	require.NoError(t, nErr)

	// userC was never in the group
	userC := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	userC, nErr = ss.User().Save(userC)
	require.NoError(t, nErr)

	// add users to group (but not userC)
	_, err = ss.Group().UpsertMember(group.ID, userA.ID)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group.ID, userB.ID)
	require.NoError(t, err)

	// create channels
	channelConstrained := &model.Channel{
		TeamID:           model.NewID(),
		DisplayName:      "A Name",
		Name:             model.NewID(),
		Type:             model.ChannelTypePrivate,
		GroupConstrained: model.NewBool(true),
	}
	channelConstrained, nErr = ss.Channel().Save(channelConstrained, 9999)
	require.NoError(t, nErr)

	channelUnconstrained := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "A Name",
		Name:        model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	channelUnconstrained, nErr = ss.Channel().Save(channelUnconstrained, 9999)
	require.NoError(t, nErr)

	// create teams
	teamConstrained := &model.Team{
		DisplayName:      "Name",
		Description:      "Some description",
		CompanyName:      "Some company name",
		AllowOpenInvite:  false,
		InviteID:         "inviteid0",
		Name:             "z-z-" + model.NewID() + "a",
		Email:            "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:             model.TeamInvite,
		GroupConstrained: model.NewBool(true),
	}
	teamConstrained, nErr = ss.Team().Save(teamConstrained)
	require.NoError(t, nErr)

	teamUnconstrained := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid1",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamInvite,
	}
	teamUnconstrained, nErr = ss.Team().Save(teamUnconstrained)
	require.NoError(t, nErr)

	// create groupteams
	_, err = ss.Group().CreateGroupSyncable(model.NewGroupTeam(group.ID, teamConstrained.ID, true))
	require.NoError(t, err)

	_, err = ss.Group().CreateGroupSyncable(model.NewGroupTeam(group.ID, teamUnconstrained.ID, true))
	require.NoError(t, err)

	// create groupchannels
	_, err = ss.Group().CreateGroupSyncable(model.NewGroupChannel(group.ID, channelConstrained.ID, true))
	require.NoError(t, err)

	_, err = ss.Group().CreateGroupSyncable(model.NewGroupChannel(group.ID, channelUnconstrained.ID, true))
	require.NoError(t, err)

	// add users to teams
	userIDTeamIDs := [][]string{
		{userA.ID, teamConstrained.ID},
		{userB.ID, teamConstrained.ID},
		{userC.ID, teamConstrained.ID},
		{userA.ID, teamUnconstrained.ID},
		{userB.ID, teamUnconstrained.ID},
		{userC.ID, teamUnconstrained.ID},
	}

	for _, item := range userIDTeamIDs {
		_, nErr = ss.Team().SaveMember(&model.TeamMember{
			UserID: item[0],
			TeamID: item[1],
		}, 99)
		require.NoError(t, nErr)
	}

	// add users to channels
	userIDChannelIDs := [][]string{
		{userA.ID, channelConstrained.ID},
		{userB.ID, channelConstrained.ID},
		{userC.ID, channelConstrained.ID},
		{userA.ID, channelUnconstrained.ID},
		{userB.ID, channelUnconstrained.ID},
		{userC.ID, channelUnconstrained.ID},
	}

	for _, item := range userIDChannelIDs {
		_, err := ss.Channel().SaveMember(&model.ChannelMember{
			UserID:      item[0],
			ChannelID:   item[1],
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, err)
	}

	return &removalsData{
		UserA:                userA,
		UserB:                userB,
		UserC:                userC,
		ConstrainedChannel:   channelConstrained,
		UnconstrainedChannel: channelUnconstrained,
		ConstrainedTeam:      teamConstrained,
		UnconstrainedTeam:    teamUnconstrained,
		Group:                group,
	}
}

func testGetGroupsByChannel(t *testing.T, ss store.Store) {
	// Create Channel1
	channel1 := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Channel1",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	channel1, err := ss.Channel().Save(channel1, 9999)
	require.NoError(t, err)

	// Create Groups 1, 2 and a deleted group
	group1, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-1",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
	})
	require.NoError(t, err)

	group2, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-2",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: false,
	})
	require.NoError(t, err)

	deletedGroup, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-deleted",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
		DeleteAt:       1,
	})
	require.NoError(t, err)

	// And associate them with Channel1
	for _, g := range []*model.Group{group1, group2, deletedGroup} {
		_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
			AutoAdd:    true,
			SyncableID: channel1.ID,
			Type:       model.GroupSyncableTypeChannel,
			GroupID:    g.ID,
		})
		require.NoError(t, err)
	}

	// Create Channel2
	channel2 := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Channel2",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	channel2, nErr := ss.Channel().Save(channel2, 9999)
	require.NoError(t, nErr)

	// Create Group3
	group3, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-3",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
	})
	require.NoError(t, err)

	// And associate it to Channel2
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: channel2.ID,
		Type:       model.GroupSyncableTypeChannel,
		GroupID:    group3.ID,
	})
	require.NoError(t, err)

	// add members
	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(u1)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err := ss.User().Save(u2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user1.ID)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user2.ID)
	require.NoError(t, err)

	user2.DeleteAt = 1
	_, err = ss.User().Update(user2, true)
	require.NoError(t, err)

	group1WithMemberCount := *group1
	group1WithMemberCount.MemberCount = model.NewInt(1)

	group2WithMemberCount := *group2
	group2WithMemberCount.MemberCount = model.NewInt(0)

	group1WSA := &model.GroupWithSchemeAdmin{Group: *group1, SchemeAdmin: model.NewBool(false)}
	group2WSA := &model.GroupWithSchemeAdmin{Group: *group2, SchemeAdmin: model.NewBool(false)}
	group3WSA := &model.GroupWithSchemeAdmin{Group: *group3, SchemeAdmin: model.NewBool(false)}

	testCases := []struct {
		Name       string
		ChannelID  string
		Page       int
		PerPage    int
		Result     []*model.GroupWithSchemeAdmin
		Opts       model.GroupSearchOpts
		TotalCount *int64
	}{
		{
			Name:       "Get the two Groups for Channel1",
			ChannelID:  channel1.ID,
			Opts:       model.GroupSearchOpts{},
			Page:       0,
			PerPage:    60,
			Result:     []*model.GroupWithSchemeAdmin{group1WSA, group2WSA},
			TotalCount: model.NewInt64(2),
		},
		{
			Name:      "Get first Group for Channel1 with page 0 with 1 element",
			ChannelID: channel1.ID,
			Opts:      model.GroupSearchOpts{},
			Page:      0,
			PerPage:   1,
			Result:    []*model.GroupWithSchemeAdmin{group1WSA},
		},
		{
			Name:      "Get second Group for Channel1 with page 1 with 1 element",
			ChannelID: channel1.ID,
			Opts:      model.GroupSearchOpts{},
			Page:      1,
			PerPage:   1,
			Result:    []*model.GroupWithSchemeAdmin{group2WSA},
		},
		{
			Name:      "Get third Group for Channel2",
			ChannelID: channel2.ID,
			Opts:      model.GroupSearchOpts{},
			Page:      0,
			PerPage:   60,
			Result:    []*model.GroupWithSchemeAdmin{group3WSA},
		},
		{
			Name:       "Get empty Groups for a fake id",
			ChannelID:  model.NewID(),
			Opts:       model.GroupSearchOpts{},
			Page:       0,
			PerPage:    60,
			Result:     []*model.GroupWithSchemeAdmin{},
			TotalCount: model.NewInt64(0),
		},
		{
			Name:       "Get group matching name",
			ChannelID:  channel1.ID,
			Opts:       model.GroupSearchOpts{Q: string([]rune(*group1.Name)[2:10])}, // very low change of a name collision
			Page:       0,
			PerPage:    100,
			Result:     []*model.GroupWithSchemeAdmin{group1WSA},
			TotalCount: model.NewInt64(1),
		},
		{
			Name:       "Get group matching display name",
			ChannelID:  channel1.ID,
			Opts:       model.GroupSearchOpts{Q: "rouP-1"},
			Page:       0,
			PerPage:    100,
			Result:     []*model.GroupWithSchemeAdmin{group1WSA},
			TotalCount: model.NewInt64(1),
		},
		{
			Name:       "Get group matching multiple display names",
			ChannelID:  channel1.ID,
			Opts:       model.GroupSearchOpts{Q: "roUp-"},
			Page:       0,
			PerPage:    100,
			Result:     []*model.GroupWithSchemeAdmin{group1WSA, group2WSA},
			TotalCount: model.NewInt64(2),
		},
		{
			Name:      "Include member counts",
			ChannelID: channel1.ID,
			Opts:      model.GroupSearchOpts{IncludeMemberCount: true},
			Page:      0,
			PerPage:   2,
			Result: []*model.GroupWithSchemeAdmin{
				{Group: group1WithMemberCount, SchemeAdmin: model.NewBool(false)},
				{Group: group2WithMemberCount, SchemeAdmin: model.NewBool(false)},
			},
		},
		{
			Name:      "Include allow reference",
			ChannelID: channel1.ID,
			Opts:      model.GroupSearchOpts{FilterAllowReference: true},
			Page:      0,
			PerPage:   100,
			Result:    []*model.GroupWithSchemeAdmin{group1WSA},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Opts.PageOpts == nil {
				tc.Opts.PageOpts = &model.PageOpts{}
			}
			tc.Opts.PageOpts.Page = tc.Page
			tc.Opts.PageOpts.PerPage = tc.PerPage
			groups, err := ss.Group().GetGroupsByChannel(tc.ChannelID, tc.Opts)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.Result, groups)
			if tc.TotalCount != nil {
				var count int64
				count, err = ss.Group().CountGroupsByChannel(tc.ChannelID, tc.Opts)
				require.NoError(t, err)
				require.Equal(t, *tc.TotalCount, count)
			}
		})
	}
}

func testGetGroupsAssociatedToChannelsByTeam(t *testing.T, ss store.Store) {
	// Create Team1
	team1 := &model.Team{
		DisplayName:     "Team1",
		Description:     model.NewID(),
		CompanyName:     model.NewID(),
		AllowOpenInvite: false,
		InviteID:        model.NewID(),
		Name:            "zz" + model.NewID(),
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team1, errt := ss.Team().Save(team1)
	require.NoError(t, errt)

	// Create Channel1
	channel1 := &model.Channel{
		TeamID:      team1.ID,
		DisplayName: "Channel1",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	channel1, err := ss.Channel().Save(channel1, 9999)
	require.NoError(t, err)

	// Create Groups 1, 2 and a deleted group
	group1, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-1",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: false,
	})
	require.NoError(t, err)

	group2, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-2",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
	})
	require.NoError(t, err)

	deletedGroup, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-deleted",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
		DeleteAt:       1,
	})
	require.NoError(t, err)

	// And associate them with Channel1
	for _, g := range []*model.Group{group1, group2, deletedGroup} {
		_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
			AutoAdd:    true,
			SyncableID: channel1.ID,
			Type:       model.GroupSyncableTypeChannel,
			GroupID:    g.ID,
		})
		require.NoError(t, err)
	}

	// Create Channel2
	channel2 := &model.Channel{
		TeamID:      team1.ID,
		DisplayName: "Channel2",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	channel2, err = ss.Channel().Save(channel2, 9999)
	require.NoError(t, err)

	// Create Group3
	group3, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-3",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
	})
	require.NoError(t, err)

	// And associate it to Channel2
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: channel2.ID,
		Type:       model.GroupSyncableTypeChannel,
		GroupID:    group3.ID,
	})
	require.NoError(t, err)

	// add members
	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(u1)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err := ss.User().Save(u2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user1.ID)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user2.ID)
	require.NoError(t, err)

	user2.DeleteAt = 1
	_, err = ss.User().Update(user2, true)
	require.NoError(t, err)

	group1WithMemberCount := *group1
	group1WithMemberCount.MemberCount = model.NewInt(1)

	group2WithMemberCount := *group2
	group2WithMemberCount.MemberCount = model.NewInt(0)

	group3WithMemberCount := *group3
	group3WithMemberCount.MemberCount = model.NewInt(0)

	group1WSA := &model.GroupWithSchemeAdmin{Group: *group1, SchemeAdmin: model.NewBool(false)}
	group2WSA := &model.GroupWithSchemeAdmin{Group: *group2, SchemeAdmin: model.NewBool(false)}
	group3WSA := &model.GroupWithSchemeAdmin{Group: *group3, SchemeAdmin: model.NewBool(false)}

	testCases := []struct {
		Name    string
		TeamID  string
		Page    int
		PerPage int
		Result  map[string][]*model.GroupWithSchemeAdmin
		Opts    model.GroupSearchOpts
	}{
		{
			Name:    "Get the groups for Channel1 and Channel2",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{},
			Page:    0,
			PerPage: 60,
			Result:  map[string][]*model.GroupWithSchemeAdmin{channel1.ID: {group1WSA, group2WSA}, channel2.ID: {group3WSA}},
		},
		{
			Name:    "Get first Group for Channel1 with page 0 with 1 element",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{},
			Page:    0,
			PerPage: 1,
			Result:  map[string][]*model.GroupWithSchemeAdmin{channel1.ID: {group1WSA}},
		},
		{
			Name:    "Get second Group for Channel1 with page 1 with 1 element",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{},
			Page:    1,
			PerPage: 1,
			Result:  map[string][]*model.GroupWithSchemeAdmin{channel1.ID: {group2WSA}},
		},
		{
			Name:    "Get empty Groups for a fake id",
			TeamID:  model.NewID(),
			Opts:    model.GroupSearchOpts{},
			Page:    0,
			PerPage: 60,
			Result:  map[string][]*model.GroupWithSchemeAdmin{},
		},
		{
			Name:    "Get group matching name",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{Q: string([]rune(*group1.Name)[2:10])}, // very low chance of a name collision
			Page:    0,
			PerPage: 100,
			Result:  map[string][]*model.GroupWithSchemeAdmin{channel1.ID: {group1WSA}},
		},
		{
			Name:    "Get group matching display name",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{Q: "rouP-1"},
			Page:    0,
			PerPage: 100,
			Result:  map[string][]*model.GroupWithSchemeAdmin{channel1.ID: {group1WSA}},
		},
		{
			Name:    "Get group matching multiple display names",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{Q: "roUp-"},
			Page:    0,
			PerPage: 100,
			Result:  map[string][]*model.GroupWithSchemeAdmin{channel1.ID: {group1WSA, group2WSA}, channel2.ID: {group3WSA}},
		},
		{
			Name:    "Include member counts",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{IncludeMemberCount: true},
			Page:    0,
			PerPage: 10,
			Result: map[string][]*model.GroupWithSchemeAdmin{
				channel1.ID: {
					{Group: group1WithMemberCount, SchemeAdmin: model.NewBool(false)},
					{Group: group2WithMemberCount, SchemeAdmin: model.NewBool(false)},
				},
				channel2.ID: {
					{Group: group3WithMemberCount, SchemeAdmin: model.NewBool(false)},
				},
			},
		},
		{
			Name:    "Include allow reference",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{FilterAllowReference: true},
			Page:    0,
			PerPage: 2,
			Result: map[string][]*model.GroupWithSchemeAdmin{
				channel1.ID: {
					group2WSA,
				},
				channel2.ID: {
					group3WSA,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Opts.PageOpts == nil {
				tc.Opts.PageOpts = &model.PageOpts{}
			}
			tc.Opts.PageOpts.Page = tc.Page
			tc.Opts.PageOpts.PerPage = tc.PerPage
			groups, err := ss.Group().GetGroupsAssociatedToChannelsByTeam(tc.TeamID, tc.Opts)
			require.NoError(t, err)
			assert.Equal(t, tc.Result, groups)
		})
	}
}

func testGetGroupsByTeam(t *testing.T, ss store.Store) {
	// Create Team1
	team1 := &model.Team{
		DisplayName:     "Team1",
		Description:     model.NewID(),
		CompanyName:     model.NewID(),
		AllowOpenInvite: false,
		InviteID:        model.NewID(),
		Name:            "zz" + model.NewID(),
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team1, err := ss.Team().Save(team1)
	require.NoError(t, err)

	// Create Groups 1, 2 and a deleted group
	group1, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-1",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: false,
	})
	require.NoError(t, err)

	group2, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-2",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
	})
	require.NoError(t, err)

	deletedGroup, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-deleted",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
		DeleteAt:       1,
	})
	require.NoError(t, err)

	// And associate them with Team1
	for _, g := range []*model.Group{group1, group2, deletedGroup} {
		_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
			AutoAdd:    true,
			SyncableID: team1.ID,
			Type:       model.GroupSyncableTypeTeam,
			GroupID:    g.ID,
		})
		require.NoError(t, err)
	}

	// Create Team2
	team2 := &model.Team{
		DisplayName:     "Team2",
		Description:     model.NewID(),
		CompanyName:     model.NewID(),
		AllowOpenInvite: false,
		InviteID:        model.NewID(),
		Name:            "zz" + model.NewID(),
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamInvite,
	}
	team2, err = ss.Team().Save(team2)
	require.NoError(t, err)

	// Create Group3
	group3, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-3",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
	})
	require.NoError(t, err)

	// And associate it to Team2
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: team2.ID,
		Type:       model.GroupSyncableTypeTeam,
		GroupID:    group3.ID,
	})
	require.NoError(t, err)

	// add members
	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(u1)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err := ss.User().Save(u2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user1.ID)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user2.ID)
	require.NoError(t, err)

	user2.DeleteAt = 1
	_, err = ss.User().Update(user2, true)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(deletedGroup.ID, user1.ID)
	require.NoError(t, err)

	group1WithMemberCount := *group1
	group1WithMemberCount.MemberCount = model.NewInt(1)

	group2WithMemberCount := *group2
	group2WithMemberCount.MemberCount = model.NewInt(0)

	group1WSA := &model.GroupWithSchemeAdmin{Group: *group1, SchemeAdmin: model.NewBool(false)}
	group2WSA := &model.GroupWithSchemeAdmin{Group: *group2, SchemeAdmin: model.NewBool(false)}
	group3WSA := &model.GroupWithSchemeAdmin{Group: *group3, SchemeAdmin: model.NewBool(false)}

	testCases := []struct {
		Name       string
		TeamID     string
		Page       int
		PerPage    int
		Opts       model.GroupSearchOpts
		Result     []*model.GroupWithSchemeAdmin
		TotalCount *int64
	}{
		{
			Name:       "Get the two Groups for Team1",
			TeamID:     team1.ID,
			Opts:       model.GroupSearchOpts{},
			Page:       0,
			PerPage:    60,
			Result:     []*model.GroupWithSchemeAdmin{group1WSA, group2WSA},
			TotalCount: model.NewInt64(2),
		},
		{
			Name:    "Get first Group for Team1 with page 0 with 1 element",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{},
			Page:    0,
			PerPage: 1,
			Result:  []*model.GroupWithSchemeAdmin{group1WSA},
		},
		{
			Name:    "Get second Group for Team1 with page 1 with 1 element",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{},
			Page:    1,
			PerPage: 1,
			Result:  []*model.GroupWithSchemeAdmin{group2WSA},
		},
		{
			Name:       "Get third Group for Team2",
			TeamID:     team2.ID,
			Opts:       model.GroupSearchOpts{},
			Page:       0,
			PerPage:    60,
			Result:     []*model.GroupWithSchemeAdmin{group3WSA},
			TotalCount: model.NewInt64(1),
		},
		{
			Name:       "Get empty Groups for a fake id",
			TeamID:     model.NewID(),
			Opts:       model.GroupSearchOpts{},
			Page:       0,
			PerPage:    60,
			Result:     []*model.GroupWithSchemeAdmin{},
			TotalCount: model.NewInt64(0),
		},
		{
			Name:       "Get group matching name",
			TeamID:     team1.ID,
			Opts:       model.GroupSearchOpts{Q: string([]rune(*group1.Name)[2:10])}, // very low change of a name collision
			Page:       0,
			PerPage:    100,
			Result:     []*model.GroupWithSchemeAdmin{group1WSA},
			TotalCount: model.NewInt64(1),
		},
		{
			Name:       "Get group matching display name",
			TeamID:     team1.ID,
			Opts:       model.GroupSearchOpts{Q: "rouP-1"},
			Page:       0,
			PerPage:    100,
			Result:     []*model.GroupWithSchemeAdmin{group1WSA},
			TotalCount: model.NewInt64(1),
		},
		{
			Name:       "Get group matching multiple display names",
			TeamID:     team1.ID,
			Opts:       model.GroupSearchOpts{Q: "roUp-"},
			Page:       0,
			PerPage:    100,
			Result:     []*model.GroupWithSchemeAdmin{group1WSA, group2WSA},
			TotalCount: model.NewInt64(2),
		},
		{
			Name:    "Include member counts",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{IncludeMemberCount: true},
			Page:    0,
			PerPage: 2,
			Result: []*model.GroupWithSchemeAdmin{
				{Group: group1WithMemberCount, SchemeAdmin: model.NewBool(false)},
				{Group: group2WithMemberCount, SchemeAdmin: model.NewBool(false)},
			},
		},
		{
			Name:    "Include allow reference",
			TeamID:  team1.ID,
			Opts:    model.GroupSearchOpts{FilterAllowReference: true},
			Page:    0,
			PerPage: 100,
			Result:  []*model.GroupWithSchemeAdmin{group2WSA},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Opts.PageOpts == nil {
				tc.Opts.PageOpts = &model.PageOpts{}
			}
			tc.Opts.PageOpts.Page = tc.Page
			tc.Opts.PageOpts.PerPage = tc.PerPage
			groups, err := ss.Group().GetGroupsByTeam(tc.TeamID, tc.Opts)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.Result, groups)
			if tc.TotalCount != nil {
				var count int64
				count, err = ss.Group().CountGroupsByTeam(tc.TeamID, tc.Opts)
				require.NoError(t, err)
				require.Equal(t, *tc.TotalCount, count)
			}
		})
	}
}

func testGetGroups(t *testing.T, ss store.Store) {
	// Create Team1
	team1 := &model.Team{
		DisplayName:      "Team1",
		Description:      model.NewID(),
		CompanyName:      model.NewID(),
		AllowOpenInvite:  false,
		InviteID:         model.NewID(),
		Name:             "zz" + model.NewID(),
		Email:            "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:             model.TeamOpen,
		GroupConstrained: model.NewBool(true),
	}
	team1, err := ss.Team().Save(team1)
	require.NoError(t, err)

	startCreateTime := team1.UpdateAt - 1

	// Create Channel1
	channel1 := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Channel1",
		Name:        model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	channel1, nErr := ss.Channel().Save(channel1, 9999)
	require.NoError(t, nErr)

	// Create Groups 1 and 2
	group1, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    "group-1",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
	})
	require.NoError(t, err)

	group2, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID() + "-group-2"),
		DisplayName:    "group-2",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: false,
	})
	require.NoError(t, err)

	deletedGroup, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID() + "-group-deleted"),
		DisplayName:    "group-deleted",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: false,
		DeleteAt:       1,
	})
	require.NoError(t, err)

	// And associate them with Team1
	for _, g := range []*model.Group{group1, group2, deletedGroup} {
		_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
			AutoAdd:    true,
			SyncableID: team1.ID,
			Type:       model.GroupSyncableTypeTeam,
			GroupID:    g.ID,
		})
		require.NoError(t, err)
	}

	// Create Team2
	team2 := &model.Team{
		DisplayName:     "Team2",
		Description:     model.NewID(),
		CompanyName:     model.NewID(),
		AllowOpenInvite: false,
		InviteID:        model.NewID(),
		Name:            "zz" + model.NewID(),
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamInvite,
	}
	team2, err = ss.Team().Save(team2)
	require.NoError(t, err)

	// Create Channel2
	channel2 := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "Channel2",
		Name:        model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	channel2, nErr = ss.Channel().Save(channel2, 9999)
	require.NoError(t, nErr)

	// Create Channel3
	channel3 := &model.Channel{
		TeamID:      team1.ID,
		DisplayName: "Channel3",
		Name:        model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	channel3, nErr = ss.Channel().Save(channel3, 9999)
	require.NoError(t, nErr)

	// Create Group3
	group3, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID() + "-group-3"),
		DisplayName:    "group-3",
		RemoteID:       model.NewID(),
		Source:         model.GroupSourceLdap,
		AllowReference: true,
	})
	require.NoError(t, err)

	// And associate it to Team2
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: team2.ID,
		Type:       model.GroupSyncableTypeTeam,
		GroupID:    group3.ID,
	})
	require.NoError(t, err)

	// And associate Group1 to Channel2
	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: channel2.ID,
		Type:       model.GroupSyncableTypeChannel,
		GroupID:    group1.ID,
	})
	require.NoError(t, err)

	// And associate Group2 and Group3 to Channel1
	for _, g := range []*model.Group{group2, group3} {
		_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
			AutoAdd:    true,
			SyncableID: channel1.ID,
			Type:       model.GroupSyncableTypeChannel,
			GroupID:    g.ID,
		})
		require.NoError(t, err)
	}

	// add members
	u1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(u1)
	require.NoError(t, err)

	u2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err := ss.User().Save(u2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user1.ID)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user2.ID)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(deletedGroup.ID, user1.ID)
	require.NoError(t, err)

	user2.DeleteAt = 1
	u2Update, _ := ss.User().Update(user2, true)

	group2NameSubstring := "group-2"

	endCreateTime := u2Update.New.UpdateAt + 1

	// Create Team3
	team3 := &model.Team{
		DisplayName:     "Team3",
		Description:     model.NewID(),
		CompanyName:     model.NewID(),
		AllowOpenInvite: false,
		InviteID:        model.NewID(),
		Name:            "zz" + model.NewID(),
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamInvite,
	}
	team3, err = ss.Team().Save(team3)
	require.NoError(t, err)

	channel4 := &model.Channel{
		TeamID:      team3.ID,
		DisplayName: "Channel4",
		Name:        model.NewID(),
		Type:        model.ChannelTypePrivate,
	}
	channel4, nErr = ss.Channel().Save(channel4, 9999)
	require.NoError(t, nErr)

	testCases := []struct {
		Name    string
		Page    int
		PerPage int
		Opts    model.GroupSearchOpts
		Resultf func([]*model.Group) bool
	}{
		{
			Name:    "Get all the Groups",
			Opts:    model.GroupSearchOpts{},
			Page:    0,
			PerPage: 3,
			Resultf: func(groups []*model.Group) bool { return len(groups) == 3 },
		},
		{
			Name:    "Get first Group with page 0 with 1 element",
			Opts:    model.GroupSearchOpts{},
			Page:    0,
			PerPage: 1,
			Resultf: func(groups []*model.Group) bool { return len(groups) == 1 },
		},
		{
			Name:    "Get single result from page 1",
			Opts:    model.GroupSearchOpts{},
			Page:    1,
			PerPage: 1,
			Resultf: func(groups []*model.Group) bool { return len(groups) == 1 },
		},
		{
			Name:    "Get multiple results from page 1",
			Opts:    model.GroupSearchOpts{},
			Page:    1,
			PerPage: 2,
			Resultf: func(groups []*model.Group) bool { return len(groups) == 2 },
		},
		{
			Name:    "Get group matching name",
			Opts:    model.GroupSearchOpts{Q: group2NameSubstring},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				for _, g := range groups {
					if !strings.Contains(*g.Name, group2NameSubstring) && !strings.Contains(g.DisplayName, group2NameSubstring) {
						return false
					}
				}
				return true
			},
		},
		{
			Name:    "Get group matching display name",
			Opts:    model.GroupSearchOpts{Q: "rouP-3"},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				for _, g := range groups {
					if !strings.Contains(strings.ToLower(g.DisplayName), "roup-3") {
						return false
					}
				}
				return true
			},
		},
		{
			Name:    "Get group matching multiple display names",
			Opts:    model.GroupSearchOpts{Q: "groUp"},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				for _, g := range groups {
					if !strings.Contains(strings.ToLower(g.DisplayName), "group") {
						return false
					}
				}
				return true
			},
		},
		{
			Name:    "Include member counts",
			Opts:    model.GroupSearchOpts{IncludeMemberCount: true},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				for _, g := range groups {
					if g.MemberCount == nil {
						return false
					}
					if g.ID == group1.ID && *g.MemberCount != 1 {
						return false
					}
					if g.DeleteAt != 0 {
						return false
					}
				}
				return true
			},
		},
		{
			Name:    "Not associated to team",
			Opts:    model.GroupSearchOpts{NotAssociatedToTeam: team2.ID},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				if len(groups) == 0 {
					return false
				}
				for _, g := range groups {
					if g.ID == group3.ID {
						return false
					}
					if g.DeleteAt != 0 {
						return false
					}
				}
				return true
			},
		},
		{
			Name:    "Not associated to other team",
			Opts:    model.GroupSearchOpts{NotAssociatedToTeam: team1.ID},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				if len(groups) == 0 {
					return false
				}
				for _, g := range groups {
					if g.ID == group1.ID || g.ID == group2.ID {
						return false
					}
					if g.DeleteAt != 0 {
						return false
					}
				}
				return true
			},
		},
		{
			Name:    "Include allow reference",
			Opts:    model.GroupSearchOpts{FilterAllowReference: true},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				if len(groups) == 0 {
					return false
				}
				for _, g := range groups {
					if !g.AllowReference {
						return false
					}
					if g.DeleteAt != 0 {
						return false
					}
				}
				return true
			},
		},
		{
			Name:    "Use Since return all",
			Opts:    model.GroupSearchOpts{FilterAllowReference: true, Since: startCreateTime},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				if len(groups) == 0 {
					return false
				}
				for _, g := range groups {
					if g.DeleteAt != 0 {
						return false
					}
				}
				return true
			},
		},
		{
			Name:    "Use Since return none",
			Opts:    model.GroupSearchOpts{FilterAllowReference: true, Since: endCreateTime},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				return len(groups) == 0
			},
		},
		{
			Name:    "Filter groups from group-constrained teams",
			Opts:    model.GroupSearchOpts{NotAssociatedToChannel: channel3.ID, FilterParentTeamPermitted: true},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				return len(groups) == 2 && groups[0].ID == group1.ID && groups[1].ID == group2.ID
			},
		},
		{
			Name:    "Filter groups from group-constrained page 0",
			Opts:    model.GroupSearchOpts{NotAssociatedToChannel: channel3.ID, FilterParentTeamPermitted: true},
			Page:    0,
			PerPage: 1,
			Resultf: func(groups []*model.Group) bool {
				return groups[0].ID == group1.ID
			},
		},
		{
			Name:    "Filter groups from group-constrained page 1",
			Opts:    model.GroupSearchOpts{NotAssociatedToChannel: channel3.ID, FilterParentTeamPermitted: true},
			Page:    1,
			PerPage: 1,
			Resultf: func(groups []*model.Group) bool {
				return groups[0].ID == group2.ID
			},
		},
		{
			Name:    "Non-group constrained team with no associated groups still returns groups for the child channel",
			Opts:    model.GroupSearchOpts{NotAssociatedToChannel: channel4.ID, FilterParentTeamPermitted: true},
			Page:    0,
			PerPage: 100,
			Resultf: func(groups []*model.Group) bool {
				return len(groups) > 0
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			groups, err := ss.Group().GetGroups(tc.Page, tc.PerPage, tc.Opts)
			require.NoError(t, err)
			require.True(t, tc.Resultf(groups))
		})
	}
}

func testTeamMembersMinusGroupMembers(t *testing.T, ss store.Store) {
	const numberOfGroups = 3
	const numberOfUsers = 4

	groups := []*model.Group{}
	users := []*model.User{}

	team := &model.Team{
		DisplayName:      model.NewID(),
		Description:      model.NewID(),
		CompanyName:      model.NewID(),
		AllowOpenInvite:  false,
		InviteID:         model.NewID(),
		Name:             "zz" + model.NewID(),
		Email:            model.NewID() + "@simulator.amazonses.com",
		Type:             model.TeamOpen,
		GroupConstrained: model.NewBool(true),
	}
	team, err := ss.Team().Save(team)
	require.NoError(t, err)

	for i := 0; i < numberOfUsers; i++ {
		user := &model.User{
			Email:    MakeEmail(),
			Username: fmt.Sprintf("%d_%s", i, model.NewID()),
		}
		user, err = ss.User().Save(user)
		require.NoError(t, err)
		users = append(users, user)

		trueOrFalse := int(math.Mod(float64(i), 2)) == 0
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: team.ID, UserID: user.ID, SchemeUser: trueOrFalse, SchemeAdmin: !trueOrFalse}, 999)
		require.NoError(t, nErr)
	}

	// Extra user outside of the group member users.
	user := &model.User{
		Email:    MakeEmail(),
		Username: "99_" + model.NewID(),
	}
	user, err = ss.User().Save(user)
	require.NoError(t, err)
	users = append(users, user)
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: team.ID, UserID: user.ID, SchemeUser: true, SchemeAdmin: false}, 999)
	require.NoError(t, nErr)

	for i := 0; i < numberOfGroups; i++ {
		group := &model.Group{
			Name:        model.NewString(fmt.Sprintf("n_%d_%s", i, model.NewID())),
			DisplayName: model.NewID(),
			Source:      model.GroupSourceLdap,
			Description: model.NewID(),
			RemoteID:    model.NewID(),
		}
		group, err := ss.Group().Create(group)
		require.NoError(t, err)
		groups = append(groups, group)
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})

	// Add even users to even group, and the inverse
	for i := 0; i < numberOfUsers; i++ {
		groupIndex := int(math.Mod(float64(i), 2))
		_, err := ss.Group().UpsertMember(groups[groupIndex].ID, users[i].ID)
		require.NoError(t, err)

		// Add everyone to group 2
		_, err = ss.Group().UpsertMember(groups[numberOfGroups-1].ID, users[i].ID)
		require.NoError(t, err)
	}

	testCases := map[string]struct {
		expectedUserIDs    []string
		expectedTotalCount int64
		groupIDs           []string
		page               int
		perPage            int
		setup              func()
		teardown           func()
	}{
		"No group IDs, all members": {
			expectedUserIDs:    []string{users[0].ID, users[1].ID, users[2].ID, users[3].ID, user.ID},
			expectedTotalCount: numberOfUsers + 1,
			groupIDs:           []string{},
			page:               0,
			perPage:            100,
		},
		"All members, page 1": {
			expectedUserIDs:    []string{users[0].ID, users[1].ID, users[2].ID},
			expectedTotalCount: numberOfUsers + 1,
			groupIDs:           []string{},
			page:               0,
			perPage:            3,
		},
		"All members, page 2": {
			expectedUserIDs:    []string{users[3].ID, users[4].ID},
			expectedTotalCount: numberOfUsers + 1,
			groupIDs:           []string{},
			page:               1,
			perPage:            3,
		},
		"Group 1, even users would be removed": {
			expectedUserIDs:    []string{users[0].ID, users[2].ID, users[4].ID},
			expectedTotalCount: 3,
			groupIDs:           []string{groups[1].ID},
			page:               0,
			perPage:            100,
		},
		"Group 0, odd users would be removed": {
			expectedUserIDs:    []string{users[1].ID, users[3].ID, users[4].ID},
			expectedTotalCount: 3,
			groupIDs:           []string{groups[0].ID},
			page:               0,
			perPage:            100,
		},
		"All groups, no users would be removed": {
			expectedUserIDs:    []string{users[4].ID},
			expectedTotalCount: 1,
			groupIDs:           []string{groups[0].ID, groups[1].ID},
			page:               0,
			perPage:            100,
		},
	}

	mapUserIDs := func(users []*model.UserWithGroups) []string {
		ids := []string{}
		for _, user := range users {
			ids = append(ids, user.ID)
		}
		return ids
	}

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}

			if tc.teardown != nil {
				defer tc.teardown()
			}

			actual, err := ss.Group().TeamMembersMinusGroupMembers(team.ID, tc.groupIDs, tc.page, tc.perPage)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.expectedUserIDs, mapUserIDs(actual))

			actualCount, err := ss.Group().CountTeamMembersMinusGroupMembers(team.ID, tc.groupIDs)
			require.NoError(t, err)
			require.Equal(t, tc.expectedTotalCount, actualCount)
		})
	}
}

func testChannelMembersMinusGroupMembers(t *testing.T, ss store.Store) {
	const numberOfGroups = 3
	const numberOfUsers = 4

	groups := []*model.Group{}
	users := []*model.User{}

	channel := &model.Channel{
		TeamID:           model.NewID(),
		DisplayName:      "A Name",
		Name:             model.NewID(),
		Type:             model.ChannelTypePrivate,
		GroupConstrained: model.NewBool(true),
	}
	channel, err := ss.Channel().Save(channel, 9999)
	require.NoError(t, err)

	for i := 0; i < numberOfUsers; i++ {
		user := &model.User{
			Email:    MakeEmail(),
			Username: fmt.Sprintf("%d_%s", i, model.NewID()),
		}
		user, err = ss.User().Save(user)
		require.NoError(t, err)
		users = append(users, user)

		trueOrFalse := int(math.Mod(float64(i), 2)) == 0
		_, err = ss.Channel().SaveMember(&model.ChannelMember{
			ChannelID:   channel.ID,
			UserID:      user.ID,
			SchemeUser:  trueOrFalse,
			SchemeAdmin: !trueOrFalse,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, err)
	}

	// Extra user outside of the group member users.
	user, err := ss.User().Save(&model.User{
		Email:    MakeEmail(),
		Username: "99_" + model.NewID(),
	})
	require.NoError(t, err)
	users = append(users, user)
	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   channel.ID,
		UserID:      user.ID,
		SchemeUser:  true,
		SchemeAdmin: false,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
	})
	require.NoError(t, err)

	for i := 0; i < numberOfGroups; i++ {
		group := &model.Group{
			Name:        model.NewString(fmt.Sprintf("n_%d_%s", i, model.NewID())),
			DisplayName: model.NewID(),
			Source:      model.GroupSourceLdap,
			Description: model.NewID(),
			RemoteID:    model.NewID(),
		}
		group, err := ss.Group().Create(group)
		require.NoError(t, err)
		groups = append(groups, group)
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})

	// Add even users to even group, and the inverse
	for i := 0; i < numberOfUsers; i++ {
		groupIndex := int(math.Mod(float64(i), 2))
		_, err := ss.Group().UpsertMember(groups[groupIndex].ID, users[i].ID)
		require.NoError(t, err)

		// Add everyone to group 2
		_, err = ss.Group().UpsertMember(groups[numberOfGroups-1].ID, users[i].ID)
		require.NoError(t, err)
	}

	testCases := map[string]struct {
		expectedUserIDs    []string
		expectedTotalCount int64
		groupIDs           []string
		page               int
		perPage            int
		setup              func()
		teardown           func()
	}{
		"No group IDs, all members": {
			expectedUserIDs:    []string{users[0].ID, users[1].ID, users[2].ID, users[3].ID, users[4].ID},
			expectedTotalCount: numberOfUsers + 1,
			groupIDs:           []string{},
			page:               0,
			perPage:            100,
		},
		"All members, page 1": {
			expectedUserIDs:    []string{users[0].ID, users[1].ID, users[2].ID},
			expectedTotalCount: numberOfUsers + 1,
			groupIDs:           []string{},
			page:               0,
			perPage:            3,
		},
		"All members, page 2": {
			expectedUserIDs:    []string{users[3].ID, users[4].ID},
			expectedTotalCount: numberOfUsers + 1,
			groupIDs:           []string{},
			page:               1,
			perPage:            3,
		},
		"Group 1, even users would be removed": {
			expectedUserIDs:    []string{users[0].ID, users[2].ID, users[4].ID},
			expectedTotalCount: 3,
			groupIDs:           []string{groups[1].ID},
			page:               0,
			perPage:            100,
		},
		"Group 0, odd users would be removed": {
			expectedUserIDs:    []string{users[1].ID, users[3].ID, users[4].ID},
			expectedTotalCount: 3,
			groupIDs:           []string{groups[0].ID},
			page:               0,
			perPage:            100,
		},
		"All groups, no users would be removed": {
			expectedUserIDs:    []string{users[4].ID},
			expectedTotalCount: 1,
			groupIDs:           []string{groups[0].ID, groups[1].ID},
			page:               0,
			perPage:            100,
		},
	}

	mapUserIDs := func(users []*model.UserWithGroups) []string {
		ids := []string{}
		for _, user := range users {
			ids = append(ids, user.ID)
		}
		return ids
	}

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}

			if tc.teardown != nil {
				defer tc.teardown()
			}

			actual, err := ss.Group().ChannelMembersMinusGroupMembers(channel.ID, tc.groupIDs, tc.page, tc.perPage)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.expectedUserIDs, mapUserIDs(actual))

			actualCount, err := ss.Group().CountChannelMembersMinusGroupMembers(channel.ID, tc.groupIDs)
			require.NoError(t, err)
			require.Equal(t, tc.expectedTotalCount, actualCount)
		})
	}
}

func groupTestGetMemberCount(t *testing.T, ss store.Store) {
	group := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group, err := ss.Group().Create(group)
	require.NoError(t, err)

	var user *model.User
	var nErr error
	for i := 0; i < 2; i++ {
		user = &model.User{
			Email:    MakeEmail(),
			Username: fmt.Sprintf("%d_%s", i, model.NewID()),
		}
		user, nErr = ss.User().Save(user)
		require.NoError(t, nErr)

		_, err = ss.Group().UpsertMember(group.ID, user.ID)
		require.NoError(t, err)
	}

	count, err := ss.Group().GetMemberCount(group.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	user.DeleteAt = 1
	_, nErr = ss.User().Update(user, true)
	require.NoError(t, nErr)

	count, err = ss.Group().GetMemberCount(group.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func groupTestAdminRoleGroupsForSyncableMemberChannel(t *testing.T, ss store.Store) {
	user := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user, err := ss.User().Save(user)
	require.NoError(t, err)

	group1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group1, err = ss.Group().Create(group1)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user.ID)
	require.NoError(t, err)

	group2 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group2, err = ss.Group().Create(group2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group2.ID, user.ID)
	require.NoError(t, err)

	channel := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "A Name",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	channel, nErr := ss.Channel().Save(channel, 9999)
	require.NoError(t, nErr)

	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:     true,
		SyncableID:  channel.ID,
		Type:        model.GroupSyncableTypeChannel,
		GroupID:     group1.ID,
		SchemeAdmin: true,
	})
	require.NoError(t, err)

	groupSyncable2, err := ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: channel.ID,
		Type:       model.GroupSyncableTypeChannel,
		GroupID:    group2.ID,
	})
	require.NoError(t, err)

	// User is a member of both groups but only one is SchmeAdmin: true
	actualGroupIDs, err := ss.Group().AdminRoleGroupsForSyncableMember(user.ID, channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{group1.ID}, actualGroupIDs)

	// Update the second group syncable to be SchemeAdmin: true and both groups should be returned
	groupSyncable2.SchemeAdmin = true
	_, err = ss.Group().UpdateGroupSyncable(groupSyncable2)
	require.NoError(t, err)
	actualGroupIDs, err = ss.Group().AdminRoleGroupsForSyncableMember(user.ID, channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{group1.ID, group2.ID}, actualGroupIDs)

	// Deleting membership from group should stop the group from being returned
	_, err = ss.Group().DeleteMember(group1.ID, user.ID)
	require.NoError(t, err)
	actualGroupIDs, err = ss.Group().AdminRoleGroupsForSyncableMember(user.ID, channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{group2.ID}, actualGroupIDs)

	// Deleting group syncable should stop it being returned
	_, err = ss.Group().DeleteGroupSyncable(group2.ID, channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	actualGroupIDs, err = ss.Group().AdminRoleGroupsForSyncableMember(user.ID, channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{}, actualGroupIDs)
}

func groupTestAdminRoleGroupsForSyncableMemberTeam(t *testing.T, ss store.Store) {
	user := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user, err := ss.User().Save(user)
	require.NoError(t, err)

	group1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group1, err = ss.Group().Create(group1)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user.ID)
	require.NoError(t, err)

	group2 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group2, err = ss.Group().Create(group2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group2.ID, user.ID)
	require.NoError(t, err)

	team := &model.Team{
		DisplayName: "A Name",
		Name:        "zz" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	team, nErr := ss.Team().Save(team)
	require.NoError(t, nErr)

	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:     true,
		SyncableID:  team.ID,
		Type:        model.GroupSyncableTypeTeam,
		GroupID:     group1.ID,
		SchemeAdmin: true,
	})
	require.NoError(t, err)

	groupSyncable2, err := ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:    true,
		SyncableID: team.ID,
		Type:       model.GroupSyncableTypeTeam,
		GroupID:    group2.ID,
	})
	require.NoError(t, err)

	// User is a member of both groups but only one is SchmeAdmin: true
	actualGroupIDs, err := ss.Group().AdminRoleGroupsForSyncableMember(user.ID, team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{group1.ID}, actualGroupIDs)

	// Update the second group syncable to be SchemeAdmin: true and both groups should be returned
	groupSyncable2.SchemeAdmin = true
	_, err = ss.Group().UpdateGroupSyncable(groupSyncable2)
	require.NoError(t, err)
	actualGroupIDs, err = ss.Group().AdminRoleGroupsForSyncableMember(user.ID, team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{group1.ID, group2.ID}, actualGroupIDs)

	// Deleting membership from group should stop the group from being returned
	_, err = ss.Group().DeleteMember(group1.ID, user.ID)
	require.NoError(t, err)
	actualGroupIDs, err = ss.Group().AdminRoleGroupsForSyncableMember(user.ID, team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{group2.ID}, actualGroupIDs)

	// Deleting group syncable should stop it being returned
	_, err = ss.Group().DeleteGroupSyncable(group2.ID, team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	actualGroupIDs, err = ss.Group().AdminRoleGroupsForSyncableMember(user.ID, team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{}, actualGroupIDs)
}

func groupTestPermittedSyncableAdminsTeam(t *testing.T, ss store.Store) {
	user1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(user1)
	require.NoError(t, err)

	user2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err = ss.User().Save(user2)
	require.NoError(t, err)

	user3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, err = ss.User().Save(user3)
	require.NoError(t, err)

	group1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group1, err = ss.Group().Create(group1)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user1.ID)
	require.NoError(t, err)
	_, err = ss.Group().UpsertMember(group1.ID, user2.ID)
	require.NoError(t, err)

	group2 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group2, err = ss.Group().Create(group2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group2.ID, user3.ID)
	require.NoError(t, err)

	team := &model.Team{
		DisplayName: "A Name",
		Name:        "zz" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	team, nErr := ss.Team().Save(team)
	require.NoError(t, nErr)

	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:     true,
		SyncableID:  team.ID,
		Type:        model.GroupSyncableTypeTeam,
		GroupID:     group1.ID,
		SchemeAdmin: true,
	})
	require.NoError(t, err)

	groupSyncable2, err := ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:     true,
		SyncableID:  team.ID,
		Type:        model.GroupSyncableTypeTeam,
		GroupID:     group2.ID,
		SchemeAdmin: false,
	})
	require.NoError(t, err)

	// group 1's users are returned because groupsyncable 2 has SchemeAdmin false.
	actualUserIDs, err := ss.Group().PermittedSyncableAdmins(team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{user1.ID, user2.ID}, actualUserIDs)

	// update groupsyncable 2 to be SchemeAdmin true
	groupSyncable2.SchemeAdmin = true
	_, err = ss.Group().UpdateGroupSyncable(groupSyncable2)
	require.NoError(t, err)

	// group 2's users are now included in return value
	actualUserIDs, err = ss.Group().PermittedSyncableAdmins(team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{user1.ID, user2.ID, user3.ID}, actualUserIDs)

	// deleted group member should not be included
	ss.Group().DeleteMember(group1.ID, user2.ID)
	require.NoError(t, err)
	actualUserIDs, err = ss.Group().PermittedSyncableAdmins(team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{user1.ID, user3.ID}, actualUserIDs)

	// deleted group syncable no longer includes group members
	_, err = ss.Group().DeleteGroupSyncable(group1.ID, team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	actualUserIDs, err = ss.Group().PermittedSyncableAdmins(team.ID, model.GroupSyncableTypeTeam)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{user3.ID}, actualUserIDs)
}

func groupTestPermittedSyncableAdminsChannel(t *testing.T, ss store.Store) {
	user1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err := ss.User().Save(user1)
	require.NoError(t, err)

	user2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err = ss.User().Save(user2)
	require.NoError(t, err)

	user3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, err = ss.User().Save(user3)
	require.NoError(t, err)

	group1 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group1, err = ss.Group().Create(group1)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group1.ID, user1.ID)
	require.NoError(t, err)
	_, err = ss.Group().UpsertMember(group1.ID, user2.ID)
	require.NoError(t, err)

	group2 := &model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		Description: model.NewID(),
		RemoteID:    model.NewID(),
	}
	group2, err = ss.Group().Create(group2)
	require.NoError(t, err)

	_, err = ss.Group().UpsertMember(group2.ID, user3.ID)
	require.NoError(t, err)

	channel := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "A Name",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}
	channel, nErr := ss.Channel().Save(channel, 9999)
	require.NoError(t, nErr)

	_, err = ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:     true,
		SyncableID:  channel.ID,
		Type:        model.GroupSyncableTypeChannel,
		GroupID:     group1.ID,
		SchemeAdmin: true,
	})
	require.NoError(t, err)

	groupSyncable2, err := ss.Group().CreateGroupSyncable(&model.GroupSyncable{
		AutoAdd:     true,
		SyncableID:  channel.ID,
		Type:        model.GroupSyncableTypeChannel,
		GroupID:     group2.ID,
		SchemeAdmin: false,
	})
	require.NoError(t, err)

	// group 1's users are returned because groupsyncable 2 has SchemeAdmin false.
	actualUserIDs, err := ss.Group().PermittedSyncableAdmins(channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{user1.ID, user2.ID}, actualUserIDs)

	// update groupsyncable 2 to be SchemeAdmin true
	groupSyncable2.SchemeAdmin = true
	_, err = ss.Group().UpdateGroupSyncable(groupSyncable2)
	require.NoError(t, err)

	// group 2's users are now included in return value
	actualUserIDs, err = ss.Group().PermittedSyncableAdmins(channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{user1.ID, user2.ID, user3.ID}, actualUserIDs)

	// deleted group member should not be included
	ss.Group().DeleteMember(group1.ID, user2.ID)
	require.NoError(t, err)
	actualUserIDs, err = ss.Group().PermittedSyncableAdmins(channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{user1.ID, user3.ID}, actualUserIDs)

	// deleted group syncable no longer includes group members
	_, err = ss.Group().DeleteGroupSyncable(group1.ID, channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	actualUserIDs, err = ss.Group().PermittedSyncableAdmins(channel.ID, model.GroupSyncableTypeChannel)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{user3.ID}, actualUserIDs)
}

func groupTestpUpdateMembersRoleTeam(t *testing.T, ss store.Store) {
	team := &model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteID:        "inviteid0",
		Name:            "z-z-" + model.NewID() + "a",
		Email:           "success+" + model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	}
	team, err := ss.Team().Save(team)
	require.NoError(t, err)

	user1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err = ss.User().Save(user1)
	require.NoError(t, err)

	user2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err = ss.User().Save(user2)
	require.NoError(t, err)

	user3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, err = ss.User().Save(user3)
	require.NoError(t, err)

	user4 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user4, err = ss.User().Save(user4)
	require.NoError(t, err)

	for _, user := range []*model.User{user1, user2, user3} {
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: team.ID, UserID: user.ID}, 9999)
		require.NoError(t, nErr)
	}

	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: team.ID, UserID: user4.ID, SchemeGuest: true}, 9999)
	require.NoError(t, nErr)

	tests := []struct {
		testName               string
		inUserIDs              []string
		targetSchemeAdminValue bool
	}{
		{
			"Given users are admins",
			[]string{user1.ID, user2.ID},
			true,
		},
		{
			"Given users are members",
			[]string{user2.ID},
			false,
		},
		{
			"Non-given users are admins",
			[]string{user2.ID},
			false,
		},
		{
			"Non-given users are members",
			[]string{user2.ID},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err = ss.Team().UpdateMembersRole(team.ID, tt.inUserIDs)
			require.NoError(t, err)

			members, err := ss.Team().GetMembers(team.ID, 0, 100, nil)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(members), 4) // sanity check for team membership

			for _, member := range members {
				if utils.StringInSlice(member.UserID, tt.inUserIDs) {
					require.True(t, member.SchemeAdmin)
				} else {
					require.False(t, member.SchemeAdmin)
				}

				// Ensure guest account never changes.
				if member.UserID == user4.ID {
					require.False(t, member.SchemeUser)
					require.False(t, member.SchemeAdmin)
					require.True(t, member.SchemeGuest)
				}
			}
		})
	}
}

func groupTestpUpdateMembersRoleChannel(t *testing.T, ss store.Store) {
	channel := &model.Channel{
		TeamID:      model.NewID(),
		DisplayName: "A Name",
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen, // Query does not look at type so this shouldn't matter.
	}
	channel, err := ss.Channel().Save(channel, 9999)
	require.NoError(t, err)

	user1 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user1, err = ss.User().Save(user1)
	require.NoError(t, err)

	user2 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user2, err = ss.User().Save(user2)
	require.NoError(t, err)

	user3 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user3, err = ss.User().Save(user3)
	require.NoError(t, err)

	user4 := &model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	user4, err = ss.User().Save(user4)
	require.NoError(t, err)

	for _, user := range []*model.User{user1, user2, user3} {
		_, err = ss.Channel().SaveMember(&model.ChannelMember{
			ChannelID:   channel.ID,
			UserID:      user.ID,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		require.NoError(t, err)
	}

	_, err = ss.Channel().SaveMember(&model.ChannelMember{
		ChannelID:   channel.ID,
		UserID:      user4.ID,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
		SchemeGuest: true,
	})
	require.NoError(t, err)

	tests := []struct {
		testName               string
		inUserIDs              []string
		targetSchemeAdminValue bool
	}{
		{
			"Given users are admins",
			[]string{user1.ID, user2.ID},
			true,
		},
		{
			"Given users are members",
			[]string{user2.ID},
			false,
		},
		{
			"Non-given users are admins",
			[]string{user2.ID},
			false,
		},
		{
			"Non-given users are members",
			[]string{user2.ID},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err = ss.Channel().UpdateMembersRole(channel.ID, tt.inUserIDs)
			require.NoError(t, err)

			members, err := ss.Channel().GetMembers(channel.ID, 0, 100)
			require.NoError(t, err)

			require.GreaterOrEqual(t, len(*members), 4) // sanity check for channel membership

			for _, member := range *members {
				if utils.StringInSlice(member.UserID, tt.inUserIDs) {
					require.True(t, member.SchemeAdmin)
				} else {
					require.False(t, member.SchemeAdmin)
				}

				// Ensure guest account never changes.
				if member.UserID == user4.ID {
					require.False(t, member.SchemeUser)
					require.False(t, member.SchemeAdmin)
					require.True(t, member.SchemeGuest)
				}
			}
		})
	}
}

func groupTestGroupCount(t *testing.T, ss store.Store) {
	group1, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group1.ID)

	count, err := ss.Group().GroupCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))

	group2, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group2.ID)

	countAfter, err := ss.Group().GroupCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, countAfter, count+1)
}

func groupTestGroupTeamCount(t *testing.T, ss store.Store) {
	team, err := ss.Team().Save(&model.Team{
		DisplayName:     model.NewID(),
		Description:     model.NewID(),
		AllowOpenInvite: false,
		InviteID:        model.NewID(),
		Name:            "zz" + model.NewID(),
		Email:           model.NewID() + "@simulator.amazonses.com",
		Type:            model.TeamOpen,
	})
	require.NoError(t, err)
	defer ss.Team().PermanentDelete(team.ID)

	group1, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group1.ID)

	group2, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group2.ID)

	groupSyncable1, err := ss.Group().CreateGroupSyncable(model.NewGroupTeam(group1.ID, team.ID, false))
	require.NoError(t, err)
	defer ss.Group().DeleteGroupSyncable(groupSyncable1.GroupID, groupSyncable1.SyncableID, groupSyncable1.Type)

	count, err := ss.Group().GroupTeamCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))

	groupSyncable2, err := ss.Group().CreateGroupSyncable(model.NewGroupTeam(group2.ID, team.ID, false))
	require.NoError(t, err)
	defer ss.Group().DeleteGroupSyncable(groupSyncable2.GroupID, groupSyncable2.SyncableID, groupSyncable2.Type)

	countAfter, err := ss.Group().GroupTeamCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, countAfter, count+1)
}

func groupTestGroupChannelCount(t *testing.T, ss store.Store) {
	channel, err := ss.Channel().Save(&model.Channel{
		TeamID:      model.NewID(),
		DisplayName: model.NewID(),
		Name:        model.NewID(),
		Type:        model.ChannelTypeOpen,
	}, 9999)
	require.NoError(t, err)
	defer ss.Channel().Delete(channel.ID, 0)

	group1, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group1.ID)

	group2, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group2.ID)

	groupSyncable1, err := ss.Group().CreateGroupSyncable(model.NewGroupChannel(group1.ID, channel.ID, false))
	require.NoError(t, err)
	defer ss.Group().DeleteGroupSyncable(groupSyncable1.GroupID, groupSyncable1.SyncableID, groupSyncable1.Type)

	count, err := ss.Group().GroupChannelCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))

	groupSyncable2, err := ss.Group().CreateGroupSyncable(model.NewGroupChannel(group2.ID, channel.ID, false))
	require.NoError(t, err)
	defer ss.Group().DeleteGroupSyncable(groupSyncable2.GroupID, groupSyncable2.SyncableID, groupSyncable2.Type)

	countAfter, err := ss.Group().GroupChannelCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, countAfter, count+1)
}

func groupTestGroupMemberCount(t *testing.T, ss store.Store) {
	group, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group.ID)

	member1, err := ss.Group().UpsertMember(group.ID, model.NewID())
	require.NoError(t, err)
	defer ss.Group().DeleteMember(group.ID, member1.UserID)

	count, err := ss.Group().GroupMemberCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))

	member2, err := ss.Group().UpsertMember(group.ID, model.NewID())
	require.NoError(t, err)
	defer ss.Group().DeleteMember(group.ID, member2.UserID)

	countAfter, err := ss.Group().GroupMemberCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, countAfter, count+1)
}

func groupTestDistinctGroupMemberCount(t *testing.T, ss store.Store) {
	group1, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group1.ID)

	group2, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group2.ID)

	member1, err := ss.Group().UpsertMember(group1.ID, model.NewID())
	require.NoError(t, err)
	defer ss.Group().DeleteMember(group1.ID, member1.UserID)

	count, err := ss.Group().GroupMemberCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))

	member2, err := ss.Group().UpsertMember(group1.ID, model.NewID())
	require.NoError(t, err)
	defer ss.Group().DeleteMember(group1.ID, member2.UserID)

	countAfter1, err := ss.Group().GroupMemberCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, countAfter1, count+1)

	member3, err := ss.Group().UpsertMember(group1.ID, member1.UserID)
	require.NoError(t, err)
	defer ss.Group().DeleteMember(group1.ID, member3.UserID)

	countAfter2, err := ss.Group().GroupMemberCount()
	require.NoError(t, err)
	require.GreaterOrEqual(t, countAfter2, countAfter1)
}

func groupTestGroupCountWithAllowReference(t *testing.T, ss store.Store) {
	initialCount, err := ss.Group().GroupCountWithAllowReference()
	require.NoError(t, err)

	group1, err := ss.Group().Create(&model.Group{
		Name:        model.NewString(model.NewID()),
		DisplayName: model.NewID(),
		Source:      model.GroupSourceLdap,
		RemoteID:    model.NewID(),
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group1.ID)

	count, err := ss.Group().GroupCountWithAllowReference()
	require.NoError(t, err)
	require.Equal(t, count, initialCount)

	group2, err := ss.Group().Create(&model.Group{
		Name:           model.NewString(model.NewID()),
		DisplayName:    model.NewID(),
		Source:         model.GroupSourceLdap,
		RemoteID:       model.NewID(),
		AllowReference: true,
	})
	require.NoError(t, err)
	defer ss.Group().Delete(group2.ID)

	countAfter, err := ss.Group().GroupCountWithAllowReference()
	require.NoError(t, err)
	require.Greater(t, countAfter, count)
}
