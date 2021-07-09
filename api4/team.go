// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	MaxAddMembersBatch    = 256
	MaximumBulkImportSize = 10 * 1024 * 1024
	groupIDsParamPattern  = "[^a-zA-Z0-9,]*"
)

var groupIDsQueryParamRegex *regexp.Regexp

func init() {
	groupIDsQueryParamRegex = regexp.MustCompile(groupIDsParamPattern)
}

func (api *API) InitTeam() {
	api.BaseRoutes.Teams.Handle("", api.APISessionRequired(createTeam)).Methods("POST")
	api.BaseRoutes.Teams.Handle("", api.APISessionRequired(getAllTeams)).Methods("GET")
	api.BaseRoutes.Teams.Handle("/{team_id:[A-Za-z0-9]+}/scheme", api.APISessionRequired(updateTeamScheme)).Methods("PUT")
	api.BaseRoutes.Teams.Handle("/search", api.APISessionRequiredDisableWhenBusy(searchTeams)).Methods("POST")
	api.BaseRoutes.TeamsForUser.Handle("", api.APISessionRequired(getTeamsForUser)).Methods("GET")
	api.BaseRoutes.TeamsForUser.Handle("/unread", api.APISessionRequired(getTeamsUnreadForUser)).Methods("GET")

	api.BaseRoutes.Team.Handle("", api.APISessionRequired(getTeam)).Methods("GET")
	api.BaseRoutes.Team.Handle("", api.APISessionRequired(updateTeam)).Methods("PUT")
	api.BaseRoutes.Team.Handle("", api.APISessionRequired(deleteTeam)).Methods("DELETE")
	api.BaseRoutes.Team.Handle("/patch", api.APISessionRequired(patchTeam)).Methods("PUT")
	api.BaseRoutes.Team.Handle("/restore", api.APISessionRequired(restoreTeam)).Methods("POST")
	api.BaseRoutes.Team.Handle("/privacy", api.APISessionRequired(updateTeamPrivacy)).Methods("PUT")
	api.BaseRoutes.Team.Handle("/stats", api.APISessionRequired(getTeamStats)).Methods("GET")
	api.BaseRoutes.Team.Handle("/regenerate_invite_id", api.APISessionRequired(regenerateTeamInviteID)).Methods("POST")

	api.BaseRoutes.Team.Handle("/image", api.APISessionRequiredTrustRequester(getTeamIcon)).Methods("GET")
	api.BaseRoutes.Team.Handle("/image", api.APISessionRequired(setTeamIcon)).Methods("POST")
	api.BaseRoutes.Team.Handle("/image", api.APISessionRequired(removeTeamIcon)).Methods("DELETE")

	api.BaseRoutes.TeamMembers.Handle("", api.APISessionRequired(getTeamMembers)).Methods("GET")
	api.BaseRoutes.TeamMembers.Handle("/ids", api.APISessionRequired(getTeamMembersByIDs)).Methods("POST")
	api.BaseRoutes.TeamMembersForUser.Handle("", api.APISessionRequired(getTeamMembersForUser)).Methods("GET")
	api.BaseRoutes.TeamMembers.Handle("", api.APISessionRequired(addTeamMember)).Methods("POST")
	api.BaseRoutes.Teams.Handle("/members/invite", api.APISessionRequired(addUserToTeamFromInvite)).Methods("POST")
	api.BaseRoutes.TeamMembers.Handle("/batch", api.APISessionRequired(addTeamMembers)).Methods("POST")
	api.BaseRoutes.TeamMember.Handle("", api.APISessionRequired(removeTeamMember)).Methods("DELETE")

	api.BaseRoutes.TeamForUser.Handle("/unread", api.APISessionRequired(getTeamUnread)).Methods("GET")

	api.BaseRoutes.TeamByName.Handle("", api.APISessionRequired(getTeamByName)).Methods("GET")
	api.BaseRoutes.TeamMember.Handle("", api.APISessionRequired(getTeamMember)).Methods("GET")
	api.BaseRoutes.TeamByName.Handle("/exists", api.APISessionRequired(teamExists)).Methods("GET")
	api.BaseRoutes.TeamMember.Handle("/roles", api.APISessionRequired(updateTeamMemberRoles)).Methods("PUT")
	api.BaseRoutes.TeamMember.Handle("/schemeRoles", api.APISessionRequired(updateTeamMemberSchemeRoles)).Methods("PUT")
	api.BaseRoutes.Team.Handle("/import", api.APISessionRequired(importTeam)).Methods("POST")
	api.BaseRoutes.Team.Handle("/invite/email", api.APISessionRequired(inviteUsersToTeam)).Methods("POST")
	api.BaseRoutes.Team.Handle("/invite-guests/email", api.APISessionRequired(inviteGuestsToChannels)).Methods("POST")
	api.BaseRoutes.Teams.Handle("/invites/email", api.APISessionRequired(invalidateAllEmailInvites)).Methods("DELETE")
	api.BaseRoutes.Teams.Handle("/invite/{invite_id:[A-Za-z0-9]+}", api.APIHandler(getInviteInfo)).Methods("GET")

	api.BaseRoutes.Teams.Handle("/{team_id:[A-Za-z0-9]+}/members_minus_group_members", api.APISessionRequired(teamMembersMinusGroupMembers)).Methods("GET")
}

func createTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	team := model.TeamFromJSON(r.Body)
	if team == nil {
		c.SetInvalidParam("team")
		return
	}
	team.Email = strings.ToLower(team.Email)

	auditRec := c.MakeAuditRecord("createTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team", team)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionCreateTeam) {
		c.Err = model.NewAppError("createTeam", "api.team.is_team_creation_allowed.disabled.app_error", nil, "", http.StatusForbidden)
		return
	}

	rteam, err := c.App.CreateTeamWithUser(c.AppContext, team, c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	// Don't sanitize the team here since the user will be a team admin and their session won't reflect that yet

	auditRec.Success()
	auditRec.AddMeta("team", team) // overwrite meta

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(rteam.ToJSON()))
}

func getTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	team, err := c.App.GetTeam(c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}

	if (!team.AllowOpenInvite || team.Type != model.TeamOpen) && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), team.ID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	c.App.SanitizeTeam(*c.AppContext.Session(), team)
	w.Write([]byte(team.ToJSON()))
}

func getTeamByName(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamName()
	if c.Err != nil {
		return
	}

	team, err := c.App.GetTeamByName(c.Params.TeamName)
	if err != nil {
		c.Err = err
		return
	}

	if (!team.AllowOpenInvite || team.Type != model.TeamOpen) && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), team.ID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	c.App.SanitizeTeam(*c.AppContext.Session(), team)
	w.Write([]byte(team.ToJSON()))
}

func updateTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	team := model.TeamFromJSON(r.Body)

	if team == nil {
		c.SetInvalidParam("team")
		return
	}
	team.Email = strings.ToLower(team.Email)

	// The team being updated in the payload must be the same one as indicated in the URL.
	if team.ID != c.Params.TeamID {
		c.SetInvalidParam("id")
		return
	}

	auditRec := c.MakeAuditRecord("updateTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team", team)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	updatedTeam, err := c.App.UpdateTeam(team)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("update", updatedTeam)

	c.App.SanitizeTeam(*c.AppContext.Session(), updatedTeam)
	w.Write([]byte(updatedTeam.ToJSON()))
}

func patchTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	team := model.TeamPatchFromJSON(r.Body)

	if team == nil {
		c.SetInvalidParam("team")
		return
	}

	auditRec := c.MakeAuditRecord("patchTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	if oldTeam, err := c.App.GetTeam(c.Params.TeamID); err == nil {
		auditRec.AddMeta("team", oldTeam)
	}

	patchedTeam, err := c.App.PatchTeam(c.Params.TeamID, team)

	if err != nil {
		c.Err = err
		return
	}

	c.App.SanitizeTeam(*c.AppContext.Session(), patchedTeam)

	auditRec.Success()
	auditRec.AddMeta("patched", patchedTeam)
	c.LogAudit("")

	w.Write([]byte(patchedTeam.ToJSON()))
}

func restoreTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("restoreTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team_id", c.Params.TeamID)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	err := c.App.RestoreTeam(c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}

	// Return the restored team to be consistent with RestoreChannel.
	team, err := c.App.GetTeam(c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.AddMeta("team", team)
	auditRec.Success()

	w.Write([]byte(team.ToJSON()))
}

func updateTeamPrivacy(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	props := model.StringInterfaceFromJSON(r.Body)
	privacy, ok := props["privacy"].(string)
	if !ok {
		c.SetInvalidParam("privacy")
		return
	}

	var openInvite bool
	switch privacy {
	case model.TeamOpen:
		openInvite = true
	case model.TeamInvite:
		openInvite = false
	default:
		c.SetInvalidParam("privacy")
		return
	}

	auditRec := c.MakeAuditRecord("updateTeamPrivacy", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("privacy", privacy)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeam) {
		auditRec.AddMeta("team_id", c.Params.TeamID)
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	if err := c.App.UpdateTeamPrivacy(c.Params.TeamID, privacy, openInvite); err != nil {
		c.Err = err
		return
	}

	// Return the updated team to be consistent with UpdateChannelPrivacy
	team, err := c.App.GetTeam(c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.AddMeta("team", team)
	auditRec.Success()

	w.Write([]byte(team.ToJSON()))
}

func regenerateTeamInviteID(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	auditRec := c.MakeAuditRecord("regenerateTeamInviteId", audit.Fail)
	defer c.LogAuditRec(auditRec)

	patchedTeam, err := c.App.RegenerateTeamInviteID(c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}

	c.App.SanitizeTeam(*c.AppContext.Session(), patchedTeam)

	auditRec.Success()
	auditRec.AddMeta("team", patchedTeam)
	c.LogAudit("")

	w.Write([]byte(patchedTeam.ToJSON()))
}

func deleteTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	auditRec := c.MakeAuditRecord("deleteTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if team, err := c.App.GetTeam(c.Params.TeamID); err == nil {
		auditRec.AddMeta("team", team)
	}

	var err *model.AppError
	if c.Params.Permanent {
		if *c.App.Config().ServiceSettings.EnableAPITeamDeletion {
			err = c.App.PermanentDeleteTeamID(c.Params.TeamID)
		} else {
			err = model.NewAppError("deleteTeam", "api.user.delete_team.not_enabled.app_error", nil, "teamId="+c.Params.TeamID, http.StatusUnauthorized)
		}
	} else {
		err = c.App.SoftDeleteTeam(c.Params.TeamID)
	}

	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func getTeamsForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if c.AppContext.Session().UserID != c.Params.UserID && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementUsers) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementUsers)
		return
	}

	teams, err := c.App.GetTeamsForUser(c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	c.App.SanitizeTeams(*c.AppContext.Session(), teams)
	w.Write([]byte(model.TeamListToJSON(teams)))
}

func getTeamsUnreadForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if c.AppContext.Session().UserID != c.Params.UserID && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	// optional team id to be excluded from the result
	teamID := r.URL.Query().Get("exclude_team")

	unreadTeamsList, err := c.App.GetTeamsUnreadForUser(teamID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.TeamsUnreadToJSON(unreadTeamsList)))
}

func getTeamMember(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID().RequireUserID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	if !canSee {
		c.SetPermissionError(model.PermissionViewMembers)
		return
	}

	team, err := c.App.GetTeamMember(c.Params.TeamID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(team.ToJSON()))
}

func getTeamMembers(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	sort := r.URL.Query().Get("sort")
	excludeDeletedUsers := r.URL.Query().Get("exclude_deleted_users")
	excludeDeletedUsersBool, _ := strconv.ParseBool(excludeDeletedUsers)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	restrictions, err := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	teamMembersGetOptions := &model.TeamMembersGetOptions{
		Sort:                sort,
		ExcludeDeletedUsers: excludeDeletedUsersBool,
		ViewRestrictions:    restrictions,
	}

	members, err := c.App.GetTeamMembers(c.Params.TeamID, c.Params.Page*c.Params.PerPage, c.Params.PerPage, teamMembersGetOptions)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.TeamMembersToJSON(members)))
}

func getTeamMembersForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionReadOtherUsersTeams) {
		c.SetPermissionError(model.PermissionReadOtherUsersTeams)
		return
	}

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	if !canSee {
		c.SetPermissionError(model.PermissionViewMembers)
		return
	}

	members, err := c.App.GetTeamMembersForUser(c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.TeamMembersToJSON(members)))
}

func getTeamMembersByIDs(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	userIDs := model.ArrayFromJSON(r.Body)

	if len(userIDs) == 0 {
		c.SetInvalidParam("user_ids")
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	restrictions, err := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	members, err := c.App.GetTeamMembersByIDs(c.Params.TeamID, userIDs, restrictions)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.TeamMembersToJSON(members)))
}

func addTeamMember(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	var err *model.AppError
	member := model.TeamMemberFromJSON(r.Body)
	if member == nil {
		c.Err = model.NewAppError("addTeamMember", "api.team.add_team_member.invalid_body.app_error", nil, "Error in model.TeamMemberFromJson()", http.StatusBadRequest)
		return
	}
	if member.TeamID != c.Params.TeamID {
		c.SetInvalidParam("team_id")
		return
	}

	if !model.IsValidID(member.UserID) {
		c.SetInvalidParam("user_id")
		return
	}

	auditRec := c.MakeAuditRecord("addTeamMember", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("member", member)

	if member.UserID == c.AppContext.Session().UserID {
		var team *model.Team
		team, err = c.App.GetTeam(member.TeamID)
		if err != nil {
			c.Err = err
			return
		}

		if team.AllowOpenInvite && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionJoinPublicTeams) {
			c.SetPermissionError(model.PermissionJoinPublicTeams)
			return
		}
		if !team.AllowOpenInvite && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionJoinPrivateTeams) {
			c.SetPermissionError(model.PermissionJoinPrivateTeams)
			return
		}
	} else {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), member.TeamID, model.PermissionAddUserToTeam) {
			c.SetPermissionError(model.PermissionAddUserToTeam)
			return
		}
	}

	team, err := c.App.GetTeam(member.TeamID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("team", team)

	if team.IsGroupConstrained() {
		nonMembers, err := c.App.FilterNonGroupTeamMembers([]string{member.UserID}, team)
		if err != nil {
			if v, ok := err.(*model.AppError); ok {
				c.Err = v
			} else {
				c.Err = model.NewAppError("addTeamMember", "api.team.add_members.error", nil, err.Error(), http.StatusBadRequest)
			}
			return
		}
		if len(nonMembers) > 0 {
			c.Err = model.NewAppError("addTeamMember", "api.team.add_members.user_denied", map[string]interface{}{"UserIDs": nonMembers}, "", http.StatusBadRequest)
			return
		}
	}

	member, err = c.App.AddTeamMember(c.AppContext, member.TeamID, member.UserID)

	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(member.ToJSON()))
}

func addUserToTeamFromInvite(c *Context, w http.ResponseWriter, r *http.Request) {
	tokenID := r.URL.Query().Get("token")
	inviteID := r.URL.Query().Get("invite_id")

	var member *model.TeamMember
	var err *model.AppError

	auditRec := c.MakeAuditRecord("addUserToTeamFromInvite", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("invite_id", inviteID)

	if tokenID != "" {
		member, err = c.App.AddTeamMemberByToken(c.AppContext, c.AppContext.Session().UserID, tokenID)
	} else if inviteID != "" {
		if c.AppContext.Session().Props[model.SessionPropIsGuest] == "true" {
			c.Err = model.NewAppError("addUserToTeamFromInvite", "api.team.add_user_to_team_from_invite.guest.app_error", nil, "", http.StatusForbidden)
			return
		}

		member, err = c.App.AddTeamMemberByInviteID(c.AppContext, inviteID, c.AppContext.Session().UserID)
	} else {
		err = model.NewAppError("addTeamMember", "api.team.add_user_to_team.missing_parameter.app_error", nil, "", http.StatusBadRequest)
	}

	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	if member != nil {
		auditRec.AddMeta("member", member)
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(member.ToJSON()))
}

func addTeamMembers(c *Context, w http.ResponseWriter, r *http.Request) {
	graceful := r.URL.Query().Get("graceful") != ""

	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	var err *model.AppError
	members := model.TeamMembersFromJSON(r.Body)

	if len(members) > MaxAddMembersBatch {
		c.SetInvalidParam("too many members in batch")
		return
	}

	if len(members) == 0 {
		c.SetInvalidParam("no members in batch")
		return
	}

	auditRec := c.MakeAuditRecord("addTeamMembers", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("count", len(members))

	var memberIDs []string
	for _, member := range members {
		memberIDs = append(memberIDs, member.UserID)
	}
	auditRec.AddMeta("user_ids", memberIDs)

	team, err := c.App.GetTeam(c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("team", team)

	if team.IsGroupConstrained() {
		nonMembers, err := c.App.FilterNonGroupTeamMembers(memberIDs, team)
		if err != nil {
			if v, ok := err.(*model.AppError); ok {
				c.Err = v
			} else {
				c.Err = model.NewAppError("addTeamMembers", "api.team.add_members.error", nil, err.Error(), http.StatusBadRequest)
			}
			return
		}
		if len(nonMembers) > 0 {
			c.Err = model.NewAppError("addTeamMembers", "api.team.add_members.user_denied", map[string]interface{}{"UserIDs": nonMembers}, "", http.StatusBadRequest)
			return
		}
	}

	var userIDs []string
	for _, member := range members {
		if member.TeamID != c.Params.TeamID {
			c.SetInvalidParam("team_id for member with user_id=" + member.UserID)
			return
		}

		if !model.IsValidID(member.UserID) {
			c.SetInvalidParam("user_id")
			return
		}

		userIDs = append(userIDs, member.UserID)
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionAddUserToTeam) {
		c.SetPermissionError(model.PermissionAddUserToTeam)
		return
	}

	membersWithErrors, err := c.App.AddTeamMembers(c.AppContext, c.Params.TeamID, userIDs, c.AppContext.Session().UserID, graceful)

	if membersWithErrors != nil {
		errList := make([]string, 0, len(membersWithErrors))
		for _, m := range membersWithErrors {
			if m.Error != nil {
				errList = append(errList, model.TeamMemberWithErrorToString(m))
			}
		}
		auditRec.AddMeta("errors", errList)
	}
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	w.WriteHeader(http.StatusCreated)

	if graceful {
		// in 'graceful' mode we allow a different return value, notifying the client which users were not added
		w.Write([]byte(model.TeamMembersWithErrorToJSON(membersWithErrors)))
	} else {
		w.Write([]byte(model.TeamMembersToJSON(model.TeamMembersWithErrorToTeamMembers(membersWithErrors))))
	}

}

func removeTeamMember(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID().RequireUserID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("removeTeamMember", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if c.AppContext.Session().UserID != c.Params.UserID {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionRemoveUserFromTeam) {
			c.SetPermissionError(model.PermissionRemoveUserFromTeam)
			return
		}
	}

	team, err := c.App.GetTeam(c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("team", team)

	user, err := c.App.GetUser(c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("user", user)

	if team.IsGroupConstrained() && (c.Params.UserID != c.AppContext.Session().UserID) && !user.IsBot {
		c.Err = model.NewAppError("removeTeamMember", "api.team.remove_member.group_constrained.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if err := c.App.RemoveUserFromTeam(c.AppContext, c.Params.TeamID, c.Params.UserID, c.AppContext.Session().UserID); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func getTeamUnread(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID().RequireUserID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	unreadTeam, err := c.App.GetTeamUnread(c.Params.TeamID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(unreadTeam.ToJSON()))
}

func getTeamStats(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	restrictions, err := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	stats, err := c.App.GetTeamStats(c.Params.TeamID, restrictions)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(stats.ToJSON()))
}

func updateTeamMemberRoles(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID().RequireUserID()
	if c.Err != nil {
		return
	}

	props := model.MapFromJSON(r.Body)

	newRoles := props["roles"]
	if !model.IsValidUserRoles(newRoles) {
		c.SetInvalidParam("team_member_roles")
		return
	}

	auditRec := c.MakeAuditRecord("updateTeamMemberRoles", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("roles", newRoles)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeamRoles) {
		c.SetPermissionError(model.PermissionManageTeamRoles)
		return
	}

	teamMember, err := c.App.UpdateTeamMemberRoles(c.Params.TeamID, c.Params.UserID, newRoles)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("member", teamMember)

	ReturnStatusOK(w)
}

func updateTeamMemberSchemeRoles(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID().RequireUserID()
	if c.Err != nil {
		return
	}

	schemeRoles := model.SchemeRolesFromJSON(r.Body)
	if schemeRoles == nil {
		c.SetInvalidParam("scheme_roles")
		return
	}

	auditRec := c.MakeAuditRecord("updateTeamMemberSchemeRoles", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("roles", schemeRoles)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeamRoles) {
		c.SetPermissionError(model.PermissionManageTeamRoles)
		return
	}

	teamMember, err := c.App.UpdateTeamMemberSchemeRoles(c.Params.TeamID, c.Params.UserID, schemeRoles.SchemeGuest, schemeRoles.SchemeUser, schemeRoles.SchemeAdmin)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("member", teamMember)

	ReturnStatusOK(w)
}

func getAllTeams(c *Context, w http.ResponseWriter, r *http.Request) {
	teams := []*model.Team{}
	var err *model.AppError
	var teamsWithCount *model.TeamsWithCount

	opts := &model.TeamSearch{}
	if c.Params.ExcludePolicyConstrained {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadComplianceDataRetentionPolicy) {
			c.SetPermissionError(model.PermissionSysconsoleReadComplianceDataRetentionPolicy)
			return
		}
		opts.ExcludePolicyConstrained = model.NewBool(true)
	}
	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadComplianceDataRetentionPolicy) {
		opts.IncludePolicyID = model.NewBool(true)
	}

	listPrivate := c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionListPrivateTeams)
	listPublic := c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionListPublicTeams)
	limit := c.Params.PerPage
	offset := limit * c.Params.Page
	if listPrivate && listPublic {
	} else if listPrivate {
		opts.AllowOpenInvite = model.NewBool(false)
	} else if listPublic {
		opts.AllowOpenInvite = model.NewBool(true)
	} else {
		// The user doesn't have permissions to list private as well as public teams.
		c.Err = model.NewAppError("getAllTeams", "api.team.get_all_teams.insufficient_permissions", nil, "", http.StatusForbidden)
		return
	}

	if c.Params.IncludeTotalCount {
		teamsWithCount, err = c.App.GetAllTeamsPageWithCount(offset, limit, opts)
	} else {
		teams, err = c.App.GetAllTeamsPage(offset, limit, opts)
	}
	if err != nil {
		c.Err = err
		return
	}

	c.App.SanitizeTeams(*c.AppContext.Session(), teams)

	var resBody []byte

	if c.Params.IncludeTotalCount {
		resBody = model.TeamsWithCountToJSON(teamsWithCount)
	} else {
		resBody = model.ToJSON(teams)
	}

	w.Write(resBody)
}

func searchTeams(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.TeamSearchFromJSON(r.Body)
	if props == nil {
		c.SetInvalidParam("team_search")
		return
	}
	// Only system managers may use the ExcludePolicyConstrained field
	if props.ExcludePolicyConstrained != nil && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadComplianceDataRetentionPolicy) {
		c.SetPermissionError(model.PermissionSysconsoleReadComplianceDataRetentionPolicy)
		return
	}
	// policy ID may only be used through the /data_retention/policies endpoint
	props.PolicyID = nil
	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadComplianceDataRetentionPolicy) {
		props.IncludePolicyID = model.NewBool(true)
	}

	var teams []*model.Team
	var totalCount int64
	var err *model.AppError

	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionListPrivateTeams) && c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionListPublicTeams) {
		teams, totalCount, err = c.App.SearchAllTeams(props)
	} else if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionListPrivateTeams) {
		if props.Page != nil || props.PerPage != nil {
			c.Err = model.NewAppError("searchTeams", "api.team.search_teams.pagination_not_implemented.private_team_search", nil, "", http.StatusNotImplemented)
			return
		}
		teams, err = c.App.SearchPrivateTeams(props)
	} else if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionListPublicTeams) {
		if props.Page != nil || props.PerPage != nil {
			c.Err = model.NewAppError("searchTeams", "api.team.search_teams.pagination_not_implemented.public_team_search", nil, "", http.StatusNotImplemented)
			return
		}
		teams, err = c.App.SearchPublicTeams(props)
	} else {
		teams = []*model.Team{}
	}

	if err != nil {
		c.Err = err
		return
	}

	c.App.SanitizeTeams(*c.AppContext.Session(), teams)

	var payload []byte
	if props.Page != nil && props.PerPage != nil {
		twc := map[string]interface{}{"teams": teams, "total_count": totalCount}
		payload = model.ToJSON(twc)
	} else {
		payload = []byte(model.TeamListToJSON(teams))
	}

	w.Write(payload)
}

func teamExists(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamName()
	if c.Err != nil {
		return
	}

	team, err := c.App.GetTeamByName(c.Params.TeamName)
	if err != nil && err.StatusCode != http.StatusNotFound {
		c.Err = err
		return
	}

	exists := false

	if team != nil {
		var teamMember *model.TeamMember
		teamMember, err = c.App.GetTeamMember(team.ID, c.AppContext.Session().UserID)
		if err != nil && err.StatusCode != http.StatusNotFound {
			c.Err = err
			return
		}

		// Verify that the user can see the team (be a member or have the permission to list the team)
		if (teamMember != nil && teamMember.DeleteAt == 0) ||
			(team.AllowOpenInvite && c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionListPublicTeams)) ||
			(!team.AllowOpenInvite && c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionListPrivateTeams)) {
			exists = true
		}
	}

	resp := map[string]bool{"exists": exists}
	w.Write([]byte(model.MapBoolToJSON(resp)))
}

func importTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() != nil && *c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("importTeam", "api.restricted_system_admin", nil, "", http.StatusForbidden)
		return
	}

	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionImportTeam) {
		c.SetPermissionError(model.PermissionImportTeam)
		return
	}

	if err := r.ParseMultipartForm(MaximumBulkImportSize); err != nil {
		c.Err = model.NewAppError("importTeam", "api.team.import_team.parse.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	importFromArray, ok := r.MultipartForm.Value["importFrom"]
	if !ok || len(importFromArray) < 1 {
		c.Err = model.NewAppError("importTeam", "api.team.import_team.no_import_from.app_error", nil, "", http.StatusBadRequest)
		return
	}
	importFrom := importFromArray[0]

	fileSizeStr, ok := r.MultipartForm.Value["filesize"]
	if !ok || len(fileSizeStr) < 1 {
		c.Err = model.NewAppError("importTeam", "api.team.import_team.unavailable.app_error", nil, "", http.StatusBadRequest)
		return
	}

	fileSize, err := strconv.ParseInt(fileSizeStr[0], 10, 64)
	if err != nil {
		c.Err = model.NewAppError("importTeam", "api.team.import_team.integer.app_error", nil, "", http.StatusBadRequest)
		return
	}

	fileInfoArray, ok := r.MultipartForm.File["file"]
	if !ok {
		c.Err = model.NewAppError("importTeam", "api.team.import_team.no_file.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if len(fileInfoArray) <= 0 {
		c.Err = model.NewAppError("importTeam", "api.team.import_team.array.app_error", nil, "", http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord("importTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team_id", c.Params.TeamID)

	fileInfo := fileInfoArray[0]

	fileData, err := fileInfo.Open()
	if err != nil {
		c.Err = model.NewAppError("importTeam", "api.team.import_team.open.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	}
	defer fileData.Close()
	auditRec.AddMeta("filename", fileInfo.Filename)
	auditRec.AddMeta("filesize", fileSize)
	auditRec.AddMeta("from", importFrom)

	var log *bytes.Buffer
	data := map[string]string{}
	switch importFrom {
	case "slack":
		var err *model.AppError
		if err, log = c.App.SlackImport(c.AppContext, fileData, fileSize, c.Params.TeamID); err != nil {
			c.Err = err
			c.Err.StatusCode = http.StatusBadRequest
		}
		data["results"] = base64.StdEncoding.EncodeToString(log.Bytes())
	default:
		c.Err = model.NewAppError("importTeam", "api.team.import_team.unknown_import_from.app_error", nil, "", http.StatusBadRequest)
	}

	if c.Err != nil {
		w.WriteHeader(c.Err.StatusCode)
		return
	}
	auditRec.Success()
	w.Write([]byte(model.MapToJSON(data)))
}

func inviteUsersToTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	graceful := r.URL.Query().Get("graceful") != ""

	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionInviteUser) {
		c.SetPermissionError(model.PermissionInviteUser)
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionAddUserToTeam) {
		c.SetPermissionError(model.PermissionInviteUser)
		return
	}

	emailList := model.ArrayFromJSON(r.Body)

	for i := range emailList {
		emailList[i] = strings.ToLower(emailList[i])
	}

	if len(emailList) == 0 {
		c.SetInvalidParam("user_email")
		return
	}

	auditRec := c.MakeAuditRecord("inviteUsersToTeam", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team_id", c.Params.TeamID)
	auditRec.AddMeta("count", len(emailList))
	auditRec.AddMeta("emails", emailList)

	if graceful {
		cloudUserLimit := *c.App.Config().ExperimentalSettings.CloudUserLimit
		var invitesOverLimit []*model.EmailInviteWithError
		if c.App.Srv().License() != nil && *c.App.Srv().License().Features.Cloud && cloudUserLimit > 0 {
			subscription, subErr := c.App.Cloud().GetSubscription(c.AppContext.Session().UserID)
			if subErr != nil {
				c.Err = model.NewAppError(
					"Api4.inviteUsersToTeam",
					"api.team.cloud.subscription.error",
					nil,
					subErr.Error(),
					http.StatusInternalServerError)
				return
			}
			if subscription == nil || subscription.IsPaidTier != "true" {
				emailList, invitesOverLimit, _ = c.App.GetErrorListForEmailsOverLimit(emailList, cloudUserLimit)
			}
		}

		// we get the emailList after it has finished checks like the emails over the list

		scheduledAt := model.GetMillis()
		jobData := map[string]string{
			"emailList":   model.ArrayToJSON(emailList),
			"teamID":      c.Params.TeamID,
			"senderID":    c.AppContext.Session().UserID,
			"scheduledAt": strconv.FormatInt(scheduledAt, 10),
		}

		// we then manually schedule the job
		_, e := c.App.Srv().Jobs.CreateJob(model.JobTypeResendInvitationEmail, jobData)
		if e != nil {
			c.Err = model.NewAppError("Api4.inviteUsersToTeam", e.ID, nil, e.Error(), e.StatusCode)
			return
		}

		var invitesWithError []*model.EmailInviteWithError
		var err *model.AppError
		if emailList != nil {
			invitesWithError, err = c.App.InviteNewUsersToTeamGracefully(emailList, c.Params.TeamID, c.AppContext.Session().UserID)
		}

		if len(invitesOverLimit) > 0 {
			invitesWithError = append(invitesWithError, invitesOverLimit...)
		}

		if invitesWithError != nil {
			errList := make([]string, 0, len(invitesWithError))
			for _, inv := range invitesWithError {
				if inv.Error != nil {
					errList = append(errList, model.EmailInviteWithErrorToString(inv))
				}
			}
			auditRec.AddMeta("errors", errList)
		}
		if err != nil {
			c.Err = err
			return
		}
		// in graceful mode we return both the successful ones and the failed ones
		w.Write([]byte(model.EmailInviteWithErrorToJSON(invitesWithError)))
	} else {
		err := c.App.InviteNewUsersToTeam(emailList, c.Params.TeamID, c.AppContext.Session().UserID)
		if err != nil {
			c.Err = err
			return
		}
		ReturnStatusOK(w)
	}
	auditRec.Success()
}

func inviteGuestsToChannels(c *Context, w http.ResponseWriter, r *http.Request) {
	graceful := r.URL.Query().Get("graceful") != ""
	if c.App.Srv().License() == nil {
		c.Err = model.NewAppError("Api4.InviteGuestsToChannels", "api.team.invate_guests_to_channels.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	if !*c.App.Config().GuestAccountsSettings.Enable {
		c.Err = model.NewAppError("Api4.InviteGuestsToChannels", "api.team.invate_guests_to_channels.disabled.error", nil, "", http.StatusNotImplemented)
		return
	}

	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("inviteGuestsToChannels", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team_id", c.Params.TeamID)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionInviteGuest) {
		c.SetPermissionError(model.PermissionInviteGuest)
		return
	}

	guestsInvite := model.GuestsInviteFromJSON(r.Body)
	if guestsInvite == nil {
		c.Err = model.NewAppError("Api4.inviteGuestsToChannels", "api.team.invite_guests_to_channels.invalid_body.app_error", nil, "", http.StatusBadRequest)
		return
	}

	for i, email := range guestsInvite.Emails {
		guestsInvite.Emails[i] = strings.ToLower(email)
	}
	if err := guestsInvite.IsValid(); err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("email_count", len(guestsInvite.Emails))
	auditRec.AddMeta("emails", guestsInvite.Emails)
	auditRec.AddMeta("channel_count", len(guestsInvite.Channels))
	auditRec.AddMeta("channels", guestsInvite.Channels)

	if graceful {
		cloudUserLimit := *c.App.Config().ExperimentalSettings.CloudUserLimit
		var invitesOverLimit []*model.EmailInviteWithError
		if c.App.Srv().License() != nil && *c.App.Srv().License().Features.Cloud && cloudUserLimit > 0 && c.IsSystemAdmin() {
			subscription, err := c.App.Cloud().GetSubscription(c.AppContext.Session().UserID)
			if err != nil {
				c.Err = model.NewAppError(
					"Api4.inviteGuestsToChannel",
					"api.team.cloud.subscription.error",
					nil,
					err.Error(),
					http.StatusInternalServerError)
				return
			}
			if subscription == nil || subscription.IsPaidTier != "true" {
				guestsInvite.Emails, invitesOverLimit, _ = c.App.GetErrorListForEmailsOverLimit(guestsInvite.Emails, cloudUserLimit)
			}
		}

		var invitesWithError []*model.EmailInviteWithError
		var err *model.AppError

		if guestsInvite.Emails != nil {
			invitesWithError, err = c.App.InviteGuestsToChannelsGracefully(c.Params.TeamID, guestsInvite, c.AppContext.Session().UserID)
		}

		if len(invitesOverLimit) > 0 {
			invitesWithError = append(invitesWithError, invitesOverLimit...)
		}

		if err != nil {
			errList := make([]string, 0, len(invitesWithError))
			for _, inv := range invitesWithError {
				errList = append(errList, model.EmailInviteWithErrorToString(inv))
			}
			auditRec.AddMeta("errors", errList)
			c.Err = err
			return
		}
		// in graceful mode we return both the successful ones and the failed ones
		w.Write([]byte(model.EmailInviteWithErrorToJSON(invitesWithError)))
	} else {
		err := c.App.InviteGuestsToChannels(c.Params.TeamID, guestsInvite, c.AppContext.Session().UserID)
		if err != nil {
			c.Err = err
			return
		}
		ReturnStatusOK(w)
	}
	auditRec.Success()
}

func getInviteInfo(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireInviteID()
	if c.Err != nil {
		return
	}

	team, err := c.App.GetTeamByInviteID(c.Params.InviteID)
	if err != nil {
		c.Err = err
		return
	}

	if team.Type != model.TeamOpen {
		c.Err = model.NewAppError("getInviteInfo", "api.team.get_invite_info.not_open_team", nil, "id="+c.Params.InviteID, http.StatusForbidden)
		return
	}

	result := map[string]string{}
	result["display_name"] = team.DisplayName
	result["description"] = team.Description
	result["name"] = team.Name
	result["id"] = team.ID
	w.Write([]byte(model.MapToJSON(result)))
}

func invalidateAllEmailInvites(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionInvalidateEmailInvite) {
		c.SetPermissionError(model.PermissionInvalidateEmailInvite)
		return
	}

	auditRec := c.MakeAuditRecord("invalidateAllEmailInvites", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if err := c.App.InvalidateAllEmailInvites(); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func getTeamIcon(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	team, err := c.App.GetTeam(c.Params.TeamID)

	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) &&
		(team.Type != model.TeamOpen || !team.AllowOpenInvite) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	etag := strconv.FormatInt(team.LastTeamIconUpdate, 10)

	if c.HandleEtag(etag, "Get Team Icon", w, r) {
		return
	}

	img, err := c.App.GetTeamIcon(team)
	if err != nil {
		c.Err = err
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%v, private", 24*60*60)) // 24 hrs
	w.Header().Set(model.HeaderEtagServer, etag)
	w.Write(img)
}

func setTeamIcon(c *Context, w http.ResponseWriter, r *http.Request) {
	defer io.Copy(ioutil.Discard, r.Body)

	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("setTeamIcon", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team_id", c.Params.TeamID)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	if r.ContentLength > *c.App.Config().FileSettings.MaxFileSize {
		c.Err = model.NewAppError("setTeamIcon", "api.team.set_team_icon.too_large.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(*c.App.Config().FileSettings.MaxFileSize); err != nil {
		c.Err = model.NewAppError("setTeamIcon", "api.team.set_team_icon.parse.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	m := r.MultipartForm

	imageArray, ok := m.File["image"]
	if !ok {
		c.Err = model.NewAppError("setTeamIcon", "api.team.set_team_icon.no_file.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if len(imageArray) <= 0 {
		c.Err = model.NewAppError("setTeamIcon", "api.team.set_team_icon.array.app_error", nil, "", http.StatusBadRequest)
		return
	}

	imageData := imageArray[0]

	if err := c.App.SetTeamIcon(c.Params.TeamID, imageData); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func removeTeamIcon(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("removeTeamIcon", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("team_id", c.Params.TeamID)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	if err := c.App.RemoveTeamIcon(c.Params.TeamID); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func updateTeamScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	schemeID := model.SchemeIDFromJSON(r.Body)
	if schemeID == nil || (!model.IsValidID(*schemeID) && *schemeID != "") {
		c.SetInvalidParam("scheme_id")
		return
	}

	auditRec := c.MakeAuditRecord("updateTeamScheme", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if c.App.Srv().License() == nil {
		c.Err = model.NewAppError("Api4.UpdateTeamScheme", "api.team.update_team_scheme.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementPermissions) {
		c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementPermissions)
		return
	}

	if *schemeID != "" {
		scheme, err := c.App.GetScheme(*schemeID)
		if err != nil {
			c.Err = err
			return
		}
		auditRec.AddMeta("scheme", scheme)

		if scheme.Scope != model.SchemeScopeTeam {
			c.Err = model.NewAppError("Api4.UpdateTeamScheme", "api.team.update_team_scheme.scheme_scope.error", nil, "", http.StatusBadRequest)
			return
		}
	}

	team, err := c.App.GetTeam(c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("team", team)

	team.SchemeID = schemeID

	_, err = c.App.UpdateTeamScheme(team)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func teamMembersMinusGroupMembers(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	groupIDsParam := groupIDsQueryParamRegex.ReplaceAllString(c.Params.GroupIDs, "")

	if len(groupIDsParam) < 26 {
		c.SetInvalidParam("group_ids")
		return
	}

	groupIDs := []string{}
	for _, gid := range strings.Split(c.Params.GroupIDs, ",") {
		if !model.IsValidID(gid) {
			c.SetInvalidParam("group_ids")
			return
		}
		groupIDs = append(groupIDs, gid)
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementGroups) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementGroups)
		return
	}

	users, totalCount, err := c.App.TeamMembersMinusGroupMembers(
		c.Params.TeamID,
		groupIDs,
		c.Params.Page,
		c.Params.PerPage,
	)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(&model.UsersWithGroupsAndCount{
		Users: users,
		Count: totalCount,
	})
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.teamMembersMinusGroupMembers", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}
