// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitCommand() {
	api.BaseRoutes.Commands.Handle("", api.ApiSessionRequired(createCommand)).Methods("POST")
	api.BaseRoutes.Commands.Handle("", api.ApiSessionRequired(listCommands)).Methods("GET")
	api.BaseRoutes.Commands.Handle("/execute", api.ApiSessionRequired(executeCommand)).Methods("POST")

	api.BaseRoutes.Command.Handle("", api.ApiSessionRequired(getCommand)).Methods("GET")
	api.BaseRoutes.Command.Handle("", api.ApiSessionRequired(updateCommand)).Methods("PUT")
	api.BaseRoutes.Command.Handle("/move", api.ApiSessionRequired(moveCommand)).Methods("PUT")
	api.BaseRoutes.Command.Handle("", api.ApiSessionRequired(deleteCommand)).Methods("DELETE")

	api.BaseRoutes.Team.Handle("/commands/autocomplete", api.ApiSessionRequired(listAutocompleteCommands)).Methods("GET")
	api.BaseRoutes.Team.Handle("/commands/autocomplete_suggestions", api.ApiSessionRequired(listCommandAutocompleteSuggestions)).Methods("GET")
	api.BaseRoutes.Command.Handle("/regen_token", api.ApiSessionRequired(regenCommandToken)).Methods("PUT")
}

func createCommand(c *Context, w http.ResponseWriter, r *http.Request) {
	cmd := model.CommandFromJSON(r.Body)
	if cmd == nil {
		c.SetInvalidParam("command")
		return
	}

	auditRec := c.MakeAuditRecord("createCommand", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), cmd.TeamID, model.PermissionManageSlashCommands) {
		c.SetPermissionError(model.PermissionManageSlashCommands)
		return
	}

	cmd.CreatorID = c.AppContext.Session().UserID

	rcmd, err := c.App.CreateCommand(cmd)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")
	auditRec.AddMeta("command", rcmd)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(rcmd.ToJSON()))
}

func updateCommand(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireCommandID()
	if c.Err != nil {
		return
	}

	cmd := model.CommandFromJSON(r.Body)
	if cmd == nil || cmd.ID != c.Params.CommandID {
		c.SetInvalidParam("command")
		return
	}

	auditRec := c.MakeAuditRecord("updateCommand", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	oldCmd, err := c.App.GetCommand(c.Params.CommandID)
	if err != nil {
		auditRec.AddMeta("command_id", c.Params.CommandID)
		c.SetCommandNotFoundError()
		return
	}
	auditRec.AddMeta("command", oldCmd)

	if cmd.TeamID != oldCmd.TeamID {
		c.Err = model.NewAppError("updateCommand", "api.command.team_mismatch.app_error", nil, "user_id="+c.AppContext.Session().UserID, http.StatusBadRequest)
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), oldCmd.TeamID, model.PermissionManageSlashCommands) {
		c.LogAudit("fail - inappropriate permissions")
		// here we return Not_found instead of a permissions error so we don't leak the existence of
		// a command to someone without permissions for the team it belongs to.
		c.SetCommandNotFoundError()
		return
	}

	if c.AppContext.Session().UserID != oldCmd.CreatorID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), oldCmd.TeamID, model.PermissionManageOthersSlashCommands) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersSlashCommands)
		return
	}

	rcmd, err := c.App.UpdateCommand(oldCmd, cmd)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	w.Write([]byte(rcmd.ToJSON()))
}

func moveCommand(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireCommandID()
	if c.Err != nil {
		return
	}

	cmr, err := model.CommandMoveRequestFromJSON(r.Body)
	if err != nil {
		c.SetInvalidParam("team_id")
		return
	}

	auditRec := c.MakeAuditRecord("moveCommand", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	newTeam, appErr := c.App.GetTeam(cmr.TeamID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddMeta("team", newTeam)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), newTeam.ID, model.PermissionManageSlashCommands) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageSlashCommands)
		return
	}

	cmd, appErr := c.App.GetCommand(c.Params.CommandID)
	if appErr != nil {
		c.SetCommandNotFoundError()
		return
	}
	auditRec.AddMeta("command", cmd)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), cmd.TeamID, model.PermissionManageSlashCommands) {
		c.LogAudit("fail - inappropriate permissions")
		// here we return Not_found instead of a permissions error so we don't leak the existence of
		// a command to someone without permissions for the team it belongs to.
		c.SetCommandNotFoundError()
		return
	}

	if appErr = c.App.MoveCommand(newTeam, cmd); appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	ReturnStatusOK(w)
}

func deleteCommand(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireCommandID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("deleteCommand", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	cmd, err := c.App.GetCommand(c.Params.CommandID)
	if err != nil {
		c.SetCommandNotFoundError()
		return
	}
	auditRec.AddMeta("command", cmd)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), cmd.TeamID, model.PermissionManageSlashCommands) {
		c.LogAudit("fail - inappropriate permissions")
		// here we return Not_found instead of a permissions error so we don't leak the existence of
		// a command to someone without permissions for the team it belongs to.
		c.SetCommandNotFoundError()
		return
	}

	if c.AppContext.Session().UserID != cmd.CreatorID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), cmd.TeamID, model.PermissionManageOthersSlashCommands) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersSlashCommands)
		return
	}

	err = c.App.DeleteCommand(cmd.ID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	ReturnStatusOK(w)
}

func listCommands(c *Context, w http.ResponseWriter, r *http.Request) {
	customOnly, _ := strconv.ParseBool(r.URL.Query().Get("custom_only"))

	teamID := r.URL.Query().Get("team_id")
	if teamID == "" {
		c.SetInvalidParam("team_id")
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	var commands []*model.Command
	var err *model.AppError
	if customOnly {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamID, model.PermissionManageSlashCommands) {
			c.SetPermissionError(model.PermissionManageSlashCommands)
			return
		}
		commands, err = c.App.ListTeamCommands(teamID)
		if err != nil {
			c.Err = err
			return
		}
	} else {
		//User with no permission should see only system commands
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamID, model.PermissionManageSlashCommands) {
			commands, err = c.App.ListAutocompleteCommands(teamID, c.AppContext.T)
			if err != nil {
				c.Err = err
				return
			}
		} else {
			commands, err = c.App.ListAllCommands(teamID, c.AppContext.T)
			if err != nil {
				c.Err = err
				return
			}
		}
	}

	w.Write([]byte(model.CommandListToJSON(commands)))
}

func getCommand(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireCommandID()
	if c.Err != nil {
		return
	}

	cmd, err := c.App.GetCommand(c.Params.CommandID)
	if err != nil {
		c.SetCommandNotFoundError()
		return
	}

	// check for permissions to view this command; must have perms to view team and
	// PERMISSION_MANAGE_SLASH_COMMANDS for the team the command belongs to.

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), cmd.TeamID, model.PermissionViewTeam) {
		// here we return Not_found instead of a permissions error so we don't leak the existence of
		// a command to someone without permissions for the team it belongs to.
		c.SetCommandNotFoundError()
		return
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), cmd.TeamID, model.PermissionManageSlashCommands) {
		// again, return not_found to ensure id existence does not leak.
		c.SetCommandNotFoundError()
		return
	}
	w.Write([]byte(cmd.ToJSON()))
}

func executeCommand(c *Context, w http.ResponseWriter, r *http.Request) {
	commandArgs := model.CommandArgsFromJSON(r.Body)
	if commandArgs == nil {
		c.SetInvalidParam("command_args")
		return
	}

	if len(commandArgs.Command) <= 1 || strings.Index(commandArgs.Command, "/") != 0 || !model.IsValidID(commandArgs.ChannelID) {
		c.Err = model.NewAppError("executeCommand", "api.command.execute_command.start.app_error", nil, "", http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord("executeCommand", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("commandargs", commandArgs)

	// checks that user is a member of the specified channel, and that they have permission to use slash commands in it
	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), commandArgs.ChannelID, model.PermissionUseSlashCommands) {
		c.SetPermissionError(model.PermissionUseSlashCommands)
		return
	}

	channel, err := c.App.GetChannel(commandArgs.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	if channel.Type != model.ChannelTypeDirect && channel.Type != model.ChannelTypeGroup {
		// if this isn't a DM or GM, the team id is implicitly taken from the channel so that slash commands created on
		// some other team can't be run against this one
		commandArgs.TeamID = channel.TeamID
	} else {
		// if the slash command was used in a DM or GM, ensure that the user is a member of the specified team, so that
		// they can't just execute slash commands against arbitrary teams
		if c.AppContext.Session().GetTeamByTeamID(commandArgs.TeamID) == nil {
			if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionUseSlashCommands) {
				c.SetPermissionError(model.PermissionUseSlashCommands)
				return
			}
		}
	}

	commandArgs.UserID = c.AppContext.Session().UserID
	commandArgs.T = c.AppContext.T
	commandArgs.SiteURL = c.GetSiteURLHeader()
	commandArgs.Session = *c.AppContext.Session()

	auditRec.AddMeta("commandargs", commandArgs) // overwrite in case teamid changed

	response, err := c.App.ExecuteCommand(c.AppContext, commandArgs)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	w.Write([]byte(response.ToJSON()))
}

func listAutocompleteCommands(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	commands, err := c.App.ListAutocompleteCommands(c.Params.TeamID, c.AppContext.T)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.CommandListToJSON(commands)))
}

func listCommandAutocompleteSuggestions(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	roleID := model.SystemUserRoleID
	if c.IsSystemAdmin() {
		roleID = model.SystemAdminRoleID
	}

	query := r.URL.Query()
	userInput := query.Get("user_input")
	if userInput == "" {
		c.SetInvalidParam("userInput")
		return
	}
	userInput = strings.TrimPrefix(userInput, "/")

	commands, err := c.App.ListAutocompleteCommands(c.Params.TeamID, c.AppContext.T)
	if err != nil {
		c.Err = err
		return
	}

	commandArgs := &model.CommandArgs{
		ChannelID: query.Get("channel_id"),
		TeamID:    c.Params.TeamID,
		RootID:    query.Get("root_id"),
		ParentID:  query.Get("parent_id"),
		UserID:    c.AppContext.Session().UserID,
		T:         c.AppContext.T,
		Session:   *c.AppContext.Session(),
		SiteURL:   c.GetSiteURLHeader(),
		Command:   userInput,
	}

	suggestions := c.App.GetSuggestions(c.AppContext, commandArgs, commands, roleID)

	w.Write(model.AutocompleteSuggestionsToJSON(suggestions))
}

func regenCommandToken(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireCommandID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("regenCommandToken", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	cmd, err := c.App.GetCommand(c.Params.CommandID)
	if err != nil {
		auditRec.AddMeta("command_id", c.Params.CommandID)
		c.SetCommandNotFoundError()
		return
	}
	auditRec.AddMeta("command", cmd)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), cmd.TeamID, model.PermissionManageSlashCommands) {
		c.LogAudit("fail - inappropriate permissions")
		// here we return Not_found instead of a permissions error so we don't leak the existence of
		// a command to someone without permissions for the team it belongs to.
		c.SetCommandNotFoundError()
		return
	}

	if c.AppContext.Session().UserID != cmd.CreatorID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), cmd.TeamID, model.PermissionManageOthersSlashCommands) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersSlashCommands)
		return
	}

	rcmd, err := c.App.RegenCommandToken(cmd)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	resp := make(map[string]string)
	resp["token"] = rcmd.Token

	w.Write([]byte(model.MapToJSON(resp)))
}
