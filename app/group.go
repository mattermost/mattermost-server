// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func (a *App) GetGroup(id string) (*model.Group, *model.AppError) {
	group, err := a.Srv().Store.Group().Get(id)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetGroup", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetGroup", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return group, nil
}

func (a *App) GetGroupByName(name string, opts model.GroupSearchOpts) (*model.Group, *model.AppError) {
	group, err := a.Srv().Store.Group().GetByName(name, opts)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetGroupByName", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetGroupByName", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return group, nil
}

func (a *App) GetGroupByRemoteID(remoteID string, groupSource model.GroupSource) (*model.Group, *model.AppError) {
	group, err := a.Srv().Store.Group().GetByRemoteID(remoteID, groupSource)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetGroupByRemoteID", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetGroupByRemoteID", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return group, nil
}

func (a *App) GetGroupsBySource(groupSource model.GroupSource) ([]*model.Group, *model.AppError) {
	groups, err := a.Srv().Store.Group().GetAllBySource(groupSource)
	if err != nil {
		return nil, model.NewAppError("GetGroupsBySource", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return groups, nil
}

func (a *App) GetGroupsByUserID(userID string) ([]*model.Group, *model.AppError) {
	groups, err := a.Srv().Store.Group().GetByUser(userID)
	if err != nil {
		return nil, model.NewAppError("GetGroupsByUserId", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return groups, nil
}

func (a *App) CreateGroup(group *model.Group) (*model.Group, *model.AppError) {
	group, err := a.Srv().Store.Group().Create(group)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &invErr):
			return nil, model.NewAppError("CreateGroup", "app.group.id.app_error", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("CreateGroup", "app.insert_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return group, nil
}

func (a *App) UpdateGroup(group *model.Group) (*model.Group, *model.AppError) {
	updatedGroup, err := a.Srv().Store.Group().Update(group)

	if err == nil {
		messageWs := model.NewWebSocketEvent(model.WebsocketEventReceivedGroup, "", "", "", nil)
		messageWs.Add("group", updatedGroup.ToJSON())
		a.Publish(messageWs)
	}

	if err != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("UpdateGroup", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("UpdateGroup", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return updatedGroup, nil
}

func (a *App) DeleteGroup(groupID string) (*model.Group, *model.AppError) {
	deletedGroup, err := a.Srv().Store.Group().Delete(groupID)

	if err == nil {
		messageWs := model.NewWebSocketEvent(model.WebsocketEventReceivedGroup, "", "", "", nil)
		messageWs.Add("group", deletedGroup.ToJSON())
		a.Publish(messageWs)
	}

	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("DeleteGroup", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("DeleteGroup", "app.update_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return deletedGroup, nil
}

func (a *App) GetGroupMemberCount(groupID string) (int64, *model.AppError) {
	count, err := a.Srv().Store.Group().GetMemberCount(groupID)
	if err != nil {
		return 0, model.NewAppError("GetGroupMemberCount", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return count, nil
}

func (a *App) GetGroupMemberUsers(groupID string) ([]*model.User, *model.AppError) {
	users, err := a.Srv().Store.Group().GetMemberUsers(groupID)
	if err != nil {
		return nil, model.NewAppError("GetGroupMemberUsers", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetGroupMemberUsersPage(groupID string, page int, perPage int) ([]*model.User, int, *model.AppError) {
	members, err := a.Srv().Store.Group().GetMemberUsersPage(groupID, page, perPage)
	if err != nil {
		return nil, 0, model.NewAppError("GetGroupMemberUsersPage", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	count, appErr := a.GetGroupMemberCount(groupID)
	if appErr != nil {
		return nil, 0, appErr
	}
	return members, int(count), nil
}

func (a *App) UpsertGroupMember(groupID string, userID string) (*model.GroupMember, *model.AppError) {
	groupMember, err := a.Srv().Store.Group().UpsertMember(groupID, userID)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &invErr):
			return nil, model.NewAppError("UpsertGroupMember", "app.group.uniqueness_error", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("UpsertGroupMember", "app.update_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return groupMember, nil
}

func (a *App) DeleteGroupMember(groupID string, userID string) (*model.GroupMember, *model.AppError) {
	groupMember, err := a.Srv().Store.Group().DeleteMember(groupID, userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("DeleteGroupMember", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("DeleteGroupMember", "app.update_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return groupMember, nil
}

func (a *App) UpsertGroupSyncable(groupSyncable *model.GroupSyncable) (*model.GroupSyncable, *model.AppError) {
	gs, err := a.Srv().Store.Group().GetGroupSyncable(groupSyncable.GroupID, groupSyncable.SyncableID, groupSyncable.Type)
	var notFoundErr *store.ErrNotFound
	if err != nil && !errors.As(err, &notFoundErr) {
		return nil, model.NewAppError("UpsertGroupSyncable", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	// reject the syncable creation if the group isn't already associated to the parent team
	if groupSyncable.Type == model.GroupSyncableTypeChannel {
		channel, nErr := a.Srv().Store.Channel().Get(groupSyncable.SyncableID, true)
		if nErr != nil {
			var nfErr *store.ErrNotFound
			switch {
			case errors.As(nErr, &nfErr):
				return nil, model.NewAppError("UpsertGroupSyncable", "app.channel.get.existing.app_error", nil, nfErr.Error(), http.StatusNotFound)
			default:
				return nil, model.NewAppError("UpsertGroupSyncable", "app.channel.get.find.app_error", nil, nErr.Error(), http.StatusInternalServerError)
			}
		}

		var team *model.Team
		team, nErr = a.Srv().Store.Team().Get(channel.TeamID)
		if nErr != nil {
			var nfErr *store.ErrNotFound
			switch {
			case errors.As(nErr, &nfErr):
				return nil, model.NewAppError("UpsertGroupSyncable", "app.team.get.find.app_error", nil, nfErr.Error(), http.StatusNotFound)
			default:
				return nil, model.NewAppError("UpsertGroupSyncable", "app.team.get.finding.app_error", nil, nErr.Error(), http.StatusInternalServerError)
			}
		}
		if team.IsGroupConstrained() {
			var teamGroups []*model.GroupWithSchemeAdmin
			teamGroups, err = a.Srv().Store.Group().GetGroupsByTeam(channel.TeamID, model.GroupSearchOpts{})
			if err != nil {
				return nil, model.NewAppError("UpsertGroupSyncable", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
			}
			var permittedGroup bool
			for _, teamGroup := range teamGroups {
				if teamGroup.Group.ID == groupSyncable.GroupID {
					permittedGroup = true
					break
				}
			}
			if !permittedGroup {
				return nil, model.NewAppError("UpsertGroupSyncable", "group_not_associated_to_synced_team", nil, "", http.StatusBadRequest)
			}
		} else {
			_, appErr := a.UpsertGroupSyncable(model.NewGroupTeam(groupSyncable.GroupID, team.ID, groupSyncable.AutoAdd))
			if appErr != nil {
				return nil, appErr
			}
		}
	}

	if gs == nil {
		gs, err = a.Srv().Store.Group().CreateGroupSyncable(groupSyncable)
		if err != nil {
			var nfErr *store.ErrNotFound
			var appErr *model.AppError
			switch {
			case errors.As(err, &appErr):
				return nil, appErr
			case errors.As(err, &nfErr):
				return nil, model.NewAppError("UpsertGroupSyncable", "store.sql_channel.get.existing.app_error", nil, nfErr.Error(), http.StatusNotFound)
			default:
				return nil, model.NewAppError("UpsertGroupSyncable", "app.insert_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
	} else {
		gs, err = a.Srv().Store.Group().UpdateGroupSyncable(groupSyncable)
		if err != nil {
			var appErr *model.AppError
			switch {
			case errors.As(err, &appErr):
				return nil, appErr
			default:
				return nil, model.NewAppError("UpsertGroupSyncable", "app.update_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	var messageWs *model.WebSocketEvent
	if gs.Type == model.GroupSyncableTypeTeam {
		messageWs = model.NewWebSocketEvent(model.WebsocketEventReceivedGroupAssociatedToTeam, gs.SyncableID, "", "", nil)
	} else {
		messageWs = model.NewWebSocketEvent(model.WebsocketEventReceivedGroupAssociatedToChannel, "", gs.SyncableID, "", nil)
	}
	messageWs.Add("group_id", gs.GroupID)
	a.Publish(messageWs)

	return gs, nil
}

func (a *App) GetGroupSyncable(groupID string, syncableID string, syncableType model.GroupSyncableType) (*model.GroupSyncable, *model.AppError) {
	group, err := a.Srv().Store.Group().GetGroupSyncable(groupID, syncableID, syncableType)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetGroupSyncable", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetGroupSyncable", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return group, nil
}

func (a *App) GetGroupSyncables(groupID string, syncableType model.GroupSyncableType) ([]*model.GroupSyncable, *model.AppError) {
	groups, err := a.Srv().Store.Group().GetAllGroupSyncablesByGroupID(groupID, syncableType)
	if err != nil {
		return nil, model.NewAppError("GetGroupSyncables", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return groups, nil
}

func (a *App) UpdateGroupSyncable(groupSyncable *model.GroupSyncable) (*model.GroupSyncable, *model.AppError) {
	if groupSyncable.DeleteAt == 0 {
		// updating a *deleted* GroupSyncable, so no need to ensure the GroupTeam is present (as done in the upsert)
		gs, err := a.Srv().Store.Group().UpdateGroupSyncable(groupSyncable)
		if err != nil {
			var appErr *model.AppError
			switch {
			case errors.As(err, &appErr):
				return nil, appErr
			default:
				return nil, model.NewAppError("UpdateGroupSyncable", "app.update_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}

		return gs, nil
	}

	// do an upsert to ensure that there's an associated GroupTeam
	gs, err := a.UpsertGroupSyncable(groupSyncable)
	if err != nil {
		return nil, err
	}

	return gs, nil
}

func (a *App) DeleteGroupSyncable(groupID string, syncableID string, syncableType model.GroupSyncableType) (*model.GroupSyncable, *model.AppError) {
	gs, err := a.Srv().Store.Group().DeleteGroupSyncable(groupID, syncableID, syncableType)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("DeleteGroupSyncable", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
		case errors.As(err, &invErr):
			return nil, model.NewAppError("DeleteGroupSyncable", "app.group.group_syncable_already_deleted", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("DeleteGroupSyncable", "app.update_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	// if a GroupTeam is being deleted delete all associated GroupChannels
	if gs.Type == model.GroupSyncableTypeTeam {
		allGroupChannels, err := a.Srv().Store.Group().GetAllGroupSyncablesByGroupID(gs.GroupID, model.GroupSyncableTypeChannel)
		if err != nil {
			return nil, model.NewAppError("DeleteGroupSyncable", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
		}

		for _, groupChannel := range allGroupChannels {
			_, err = a.Srv().Store.Group().DeleteGroupSyncable(groupChannel.GroupID, groupChannel.SyncableID, groupChannel.Type)
			if err != nil {
				var invErr *store.ErrInvalidInput
				var nfErr *store.ErrNotFound
				switch {
				case errors.As(err, &nfErr):
					return nil, model.NewAppError("DeleteGroupSyncable", "app.group.no_rows", nil, nfErr.Error(), http.StatusNotFound)
				case errors.As(err, &invErr):
					return nil, model.NewAppError("DeleteGroupSyncable", "app.group.group_syncable_already_deleted", nil, invErr.Error(), http.StatusBadRequest)
				default:
					return nil, model.NewAppError("DeleteGroupSyncable", "app.update_error", nil, err.Error(), http.StatusInternalServerError)
				}
			}
		}
	}

	var messageWs *model.WebSocketEvent
	if gs.Type == model.GroupSyncableTypeTeam {
		messageWs = model.NewWebSocketEvent(model.WebsocketEventReceivedGroupNotAssociatedToTeam, gs.SyncableID, "", "", nil)
	} else {
		messageWs = model.NewWebSocketEvent(model.WebsocketEventReceivedGroupNotAssociatedToChannel, "", gs.SyncableID, "", nil)
	}

	messageWs.Add("group_id", gs.GroupID)
	a.Publish(messageWs)

	return gs, nil
}

// TeamMembersToAdd returns a slice of UserTeamIDPair that need newly created memberships
// based on the groups configurations. The returned list can be optionally scoped to a single given team.
//
// Typically since will be the last successful group sync time.
// If includeRemovedMembers is true, then team members who left or were removed from the team will
// be included; otherwise, they will be excluded.
func (a *App) TeamMembersToAdd(since int64, teamID *string, includeRemovedMembers bool) ([]*model.UserTeamIDPair, *model.AppError) {
	userTeams, err := a.Srv().Store.Group().TeamMembersToAdd(since, teamID, includeRemovedMembers)
	if err != nil {
		return nil, model.NewAppError("TeamMembersToAdd", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return userTeams, nil
}

// ChannelMembersToAdd returns a slice of UserChannelIDPair that need newly created memberships
// based on the groups configurations. The returned list can be optionally scoped to a single given channel.
//
// Typically since will be the last successful group sync time.
// If includeRemovedMembers is true, then channel members who left or were removed from the channel will
// be included; otherwise, they will be excluded.
func (a *App) ChannelMembersToAdd(since int64, channelID *string, includeRemovedMembers bool) ([]*model.UserChannelIDPair, *model.AppError) {
	userChannels, err := a.Srv().Store.Group().ChannelMembersToAdd(since, channelID, includeRemovedMembers)
	if err != nil {
		return nil, model.NewAppError("ChannelMembersToAdd", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return userChannels, nil
}

func (a *App) TeamMembersToRemove(teamID *string) ([]*model.TeamMember, *model.AppError) {
	teamMembers, err := a.Srv().Store.Group().TeamMembersToRemove(teamID)
	if err != nil {
		return nil, model.NewAppError("TeamMembersToRemove", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return teamMembers, nil
}

func (a *App) ChannelMembersToRemove(teamID *string) ([]*model.ChannelMember, *model.AppError) {
	channelMembers, err := a.Srv().Store.Group().ChannelMembersToRemove(teamID)
	if err != nil {
		return nil, model.NewAppError("ChannelMembersToRemove", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return channelMembers, nil
}

func (a *App) GetGroupsByChannel(channelID string, opts model.GroupSearchOpts) ([]*model.GroupWithSchemeAdmin, int, *model.AppError) {
	groups, err := a.Srv().Store.Group().GetGroupsByChannel(channelID, opts)
	if err != nil {
		return nil, 0, model.NewAppError("GetGroupsByChannel", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	count, err := a.Srv().Store.Group().CountGroupsByChannel(channelID, opts)
	if err != nil {
		return nil, 0, model.NewAppError("GetGroupsByChannel", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return groups, int(count), nil
}

// GetGroupsByTeam returns the paged list and the total count of group associated to the given team.
func (a *App) GetGroupsByTeam(teamID string, opts model.GroupSearchOpts) ([]*model.GroupWithSchemeAdmin, int, *model.AppError) {
	groups, err := a.Srv().Store.Group().GetGroupsByTeam(teamID, opts)
	if err != nil {
		return nil, 0, model.NewAppError("GetGroupsByTeam", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	count, err := a.Srv().Store.Group().CountGroupsByTeam(teamID, opts)
	if err != nil {
		return nil, 0, model.NewAppError("GetGroupsByTeam", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return groups, int(count), nil
}

func (a *App) GetGroupsAssociatedToChannelsByTeam(teamID string, opts model.GroupSearchOpts) (map[string][]*model.GroupWithSchemeAdmin, *model.AppError) {
	groupsAssociatedByChannelID, err := a.Srv().Store.Group().GetGroupsAssociatedToChannelsByTeam(teamID, opts)
	if err != nil {
		return nil, model.NewAppError("GetGroupsAssociatedToChannelsByTeam", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return groupsAssociatedByChannelID, nil
}

func (a *App) GetGroups(page, perPage int, opts model.GroupSearchOpts) ([]*model.Group, *model.AppError) {
	groups, err := a.Srv().Store.Group().GetGroups(page, perPage, opts)
	if err != nil {
		return nil, model.NewAppError("GetGroups", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return groups, nil
}

// TeamMembersMinusGroupMembers returns the set of users on the given team minus the set of users in the given
// groups.
//
// The result can be used, for example, to determine the set of users who would be removed from a team if the team
// were group-constrained with the given groups.
func (a *App) TeamMembersMinusGroupMembers(teamID string, groupIDs []string, page, perPage int) ([]*model.UserWithGroups, int64, *model.AppError) {
	users, err := a.Srv().Store.Group().TeamMembersMinusGroupMembers(teamID, groupIDs, page, perPage)
	if err != nil {
		return nil, 0, model.NewAppError("TeamMembersMinusGroupMembers", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	// parse all group ids of all users
	allUsersGroupIDMap := map[string]bool{}
	for _, user := range users {
		for _, groupID := range user.GetGroupIDs() {
			allUsersGroupIDMap[groupID] = true
		}
	}

	// create a slice of distinct group ids
	var allUsersGroupIDSlice []string
	for key := range allUsersGroupIDMap {
		allUsersGroupIDSlice = append(allUsersGroupIDSlice, key)
	}

	// retrieve groups from DB
	groups, appErr := a.GetGroupsByIDs(allUsersGroupIDSlice)
	if appErr != nil {
		return nil, 0, appErr
	}

	// map groups by id
	groupMap := map[string]*model.Group{}
	for _, group := range groups {
		groupMap[group.ID] = group
	}

	// populate each instance's groups field
	for _, user := range users {
		user.Groups = []*model.Group{}
		for _, groupID := range user.GetGroupIDs() {
			group, ok := groupMap[groupID]
			if ok {
				user.Groups = append(user.Groups, group)
			}
		}
	}

	totalCount, err := a.Srv().Store.Group().CountTeamMembersMinusGroupMembers(teamID, groupIDs)
	if err != nil {
		return nil, 0, model.NewAppError("TeamMembersMinusGroupMembers", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return users, totalCount, nil
}

func (a *App) GetGroupsByIDs(groupIDs []string) ([]*model.Group, *model.AppError) {
	groups, err := a.Srv().Store.Group().GetByIDs(groupIDs)
	if err != nil {
		return nil, model.NewAppError("GetGroupsByIDs", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return groups, nil
}

// ChannelMembersMinusGroupMembers returns the set of users in the given channel minus the set of users in the given
// groups.
//
// The result can be used, for example, to determine the set of users who would be removed from a channel if the
// channel were group-constrained with the given groups.
func (a *App) ChannelMembersMinusGroupMembers(channelID string, groupIDs []string, page, perPage int) ([]*model.UserWithGroups, int64, *model.AppError) {
	users, err := a.Srv().Store.Group().ChannelMembersMinusGroupMembers(channelID, groupIDs, page, perPage)
	if err != nil {
		return nil, 0, model.NewAppError("ChannelMembersMinusGroupMembers", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	// parse all group ids of all users
	allUsersGroupIDMap := map[string]bool{}
	for _, user := range users {
		for _, groupID := range user.GetGroupIDs() {
			allUsersGroupIDMap[groupID] = true
		}
	}

	// create a slice of distinct group ids
	var allUsersGroupIDSlice []string
	for key := range allUsersGroupIDMap {
		allUsersGroupIDSlice = append(allUsersGroupIDSlice, key)
	}

	// retrieve groups from DB
	groups, appErr := a.GetGroupsByIDs(allUsersGroupIDSlice)
	if appErr != nil {
		return nil, 0, appErr
	}

	// map groups by id
	groupMap := map[string]*model.Group{}
	for _, group := range groups {
		groupMap[group.ID] = group
	}

	// populate each instance's groups field
	for _, user := range users {
		user.Groups = []*model.Group{}
		for _, groupID := range user.GetGroupIDs() {
			group, ok := groupMap[groupID]
			if ok {
				user.Groups = append(user.Groups, group)
			}
		}
	}

	totalCount, err := a.Srv().Store.Group().CountChannelMembersMinusGroupMembers(channelID, groupIDs)
	if err != nil {
		return nil, 0, model.NewAppError("ChannelMembersMinusGroupMembers", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return users, totalCount, nil
}

// UserIsInAdminRoleGroup returns true at least one of the user's groups are configured to set the members as
// admins in the given syncable.
func (a *App) UserIsInAdminRoleGroup(userID, syncableID string, syncableType model.GroupSyncableType) (bool, *model.AppError) {
	groupIDs, err := a.Srv().Store.Group().AdminRoleGroupsForSyncableMember(userID, syncableID, syncableType)
	if err != nil {
		return false, model.NewAppError("UserIsInAdminRoleGroup", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if len(groupIDs) == 0 {
		return false, nil
	}

	return true, nil
}
