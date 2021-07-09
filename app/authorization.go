// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func (a *App) MakePermissionError(s *model.Session, permissions []*model.Permission) *model.AppError {
	permissionsStr := "permission="
	for _, permission := range permissions {
		permissionsStr += permission.ID
		permissionsStr += ","
	}
	return model.NewAppError("Permissions", "api.context.permissions.app_error", nil, "userId="+s.UserID+", "+permissionsStr, http.StatusForbidden)
}

func (a *App) SessionHasPermissionTo(session model.Session, permission *model.Permission) bool {
	if session.IsUnrestricted() {
		return true
	}
	return a.RolesGrantPermission(session.GetUserRoles(), permission.ID)
}

func (a *App) SessionHasPermissionToAny(session model.Session, permissions []*model.Permission) bool {
	for _, perm := range permissions {
		if a.SessionHasPermissionTo(session, perm) {
			return true
		}
	}
	return false
}

func (a *App) SessionHasPermissionToTeam(session model.Session, teamID string, permission *model.Permission) bool {
	if teamID == "" {
		return false
	}
	if session.IsUnrestricted() {
		return true
	}

	teamMember := session.GetTeamByTeamID(teamID)
	if teamMember != nil {
		if a.RolesGrantPermission(teamMember.GetRoles(), permission.ID) {
			return true
		}
	}

	return a.RolesGrantPermission(session.GetUserRoles(), permission.ID)
}

func (a *App) SessionHasPermissionToChannel(session model.Session, channelID string, permission *model.Permission) bool {
	if channelID == "" {
		return false
	}

	ids, err := a.Srv().Store.Channel().GetAllChannelMembersForUser(session.UserID, true, true)

	var channelRoles []string
	if err == nil {
		if roles, ok := ids[channelID]; ok {
			channelRoles = strings.Fields(roles)
			if a.RolesGrantPermission(channelRoles, permission.ID) {
				return true
			}
		}
	}

	channel, appErr := a.GetChannel(channelID)
	if appErr != nil && appErr.StatusCode == http.StatusNotFound {
		return false
	}

	if session.IsUnrestricted() {
		return true
	}

	if appErr == nil && channel.TeamID != "" {
		return a.SessionHasPermissionToTeam(session, channel.TeamID, permission)
	}

	return a.SessionHasPermissionTo(session, permission)
}

func (a *App) SessionHasPermissionToChannelByPost(session model.Session, postID string, permission *model.Permission) bool {
	if channelMember, err := a.Srv().Store.Channel().GetMemberForPost(postID, session.UserID); err == nil {

		if a.RolesGrantPermission(channelMember.GetRoles(), permission.ID) {
			return true
		}
	}

	if channel, err := a.Srv().Store.Channel().GetForPost(postID); err == nil {
		if channel.TeamID != "" {
			return a.SessionHasPermissionToTeam(session, channel.TeamID, permission)
		}
	}

	return a.SessionHasPermissionTo(session, permission)
}

func (a *App) SessionHasPermissionToCategory(session model.Session, userID, teamID, categoryID string) bool {
	if a.SessionHasPermissionTo(session, model.PermissionEditOtherUsers) {
		return true
	}
	category, err := a.GetSidebarCategory(categoryID)
	return err == nil && category != nil && category.UserID == session.UserID && category.UserID == userID && category.TeamID == teamID
}

func (a *App) SessionHasPermissionToUser(session model.Session, userID string) bool {
	if userID == "" {
		return false
	}
	if session.IsUnrestricted() {
		return true
	}

	if session.UserID == userID {
		return true
	}

	if a.SessionHasPermissionTo(session, model.PermissionEditOtherUsers) {
		return true
	}

	return false
}

func (a *App) SessionHasPermissionToUserOrBot(session model.Session, userID string) bool {
	if session.IsUnrestricted() {
		return true
	}
	if a.SessionHasPermissionToUser(session, userID) {
		return true
	}

	if err := a.SessionHasPermissionToManageBot(session, userID); err == nil {
		return true
	}

	return false
}

func (a *App) HasPermissionTo(askingUserID string, permission *model.Permission) bool {
	user, err := a.GetUser(askingUserID)
	if err != nil {
		return false
	}

	roles := user.GetRoles()

	return a.RolesGrantPermission(roles, permission.ID)
}

func (a *App) HasPermissionToTeam(askingUserID string, teamID string, permission *model.Permission) bool {
	if teamID == "" || askingUserID == "" {
		return false
	}
	teamMember, _ := a.GetTeamMember(teamID, askingUserID)
	if teamMember != nil && teamMember.DeleteAt == 0 {
		if a.RolesGrantPermission(teamMember.GetRoles(), permission.ID) {
			return true
		}
	}
	return a.HasPermissionTo(askingUserID, permission)
}

func (a *App) HasPermissionToChannel(askingUserID string, channelID string, permission *model.Permission) bool {
	if channelID == "" || askingUserID == "" {
		return false
	}

	channelMember, err := a.GetChannelMember(context.Background(), channelID, askingUserID)
	if err == nil {
		roles := channelMember.GetRoles()
		if a.RolesGrantPermission(roles, permission.ID) {
			return true
		}
	}

	var channel *model.Channel
	channel, err = a.GetChannel(channelID)
	if err == nil {
		return a.HasPermissionToTeam(askingUserID, channel.TeamID, permission)
	}

	return a.HasPermissionTo(askingUserID, permission)
}

func (a *App) HasPermissionToChannelByPost(askingUserID string, postID string, permission *model.Permission) bool {
	if channelMember, err := a.Srv().Store.Channel().GetMemberForPost(postID, askingUserID); err == nil {
		if a.RolesGrantPermission(channelMember.GetRoles(), permission.ID) {
			return true
		}
	}

	if channel, err := a.Srv().Store.Channel().GetForPost(postID); err == nil {
		return a.HasPermissionToTeam(askingUserID, channel.TeamID, permission)
	}

	return a.HasPermissionTo(askingUserID, permission)
}

func (a *App) HasPermissionToUser(askingUserID string, userID string) bool {
	if askingUserID == userID {
		return true
	}

	if a.HasPermissionTo(askingUserID, model.PermissionEditOtherUsers) {
		return true
	}

	return false
}

func (a *App) RolesGrantPermission(roleNames []string, permissionID string) bool {
	roles, err := a.GetRolesByNames(roleNames)
	if err != nil {
		// This should only happen if something is very broken. We can't realistically
		// recover the situation, so deny permission and log an error.
		mlog.Error("Failed to get roles from database with role names: "+strings.Join(roleNames, ",")+" ", mlog.Err(err))
		return false
	}

	for _, role := range roles {
		if role.DeleteAt != 0 {
			continue
		}

		permissions := role.Permissions
		for _, permission := range permissions {
			if permission == permissionID {
				return true
			}
		}
	}

	return false
}

// SessionHasPermissionToManageBot returns nil if the session has access to manage the given bot.
// This function deviates from other authorization checks in returning an error instead of just
// a boolean, allowing the permission failure to be exposed with more granularity.
func (a *App) SessionHasPermissionToManageBot(session model.Session, botUserID string) *model.AppError {
	existingBot, err := a.GetBot(botUserID, true)
	if err != nil {
		return err
	}
	if session.IsUnrestricted() {
		return nil
	}

	if existingBot.OwnerID == session.UserID {
		if !a.SessionHasPermissionTo(session, model.PermissionManageBots) {
			if !a.SessionHasPermissionTo(session, model.PermissionReadBots) {
				// If the user doesn't have permission to read bots, pretend as if
				// the bot doesn't exist at all.
				return model.MakeBotNotFoundError(botUserID)
			}
			return a.MakePermissionError(&session, []*model.Permission{model.PermissionManageBots})
		}
	} else {
		if !a.SessionHasPermissionTo(session, model.PermissionManageOthersBots) {
			if !a.SessionHasPermissionTo(session, model.PermissionReadOthersBots) {
				// If the user doesn't have permission to read others' bots,
				// pretend as if the bot doesn't exist at all.
				return model.MakeBotNotFoundError(botUserID)
			}
			return a.MakePermissionError(&session, []*model.Permission{model.PermissionManageOthersBots})
		}
	}

	return nil
}
