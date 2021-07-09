// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitWebhook() {
	api.BaseRoutes.IncomingHooks.Handle("", api.ApiSessionRequired(createIncomingHook)).Methods("POST")
	api.BaseRoutes.IncomingHooks.Handle("", api.ApiSessionRequired(getIncomingHooks)).Methods("GET")
	api.BaseRoutes.IncomingHook.Handle("", api.ApiSessionRequired(getIncomingHook)).Methods("GET")
	api.BaseRoutes.IncomingHook.Handle("", api.ApiSessionRequired(updateIncomingHook)).Methods("PUT")
	api.BaseRoutes.IncomingHook.Handle("", api.ApiSessionRequired(deleteIncomingHook)).Methods("DELETE")

	api.BaseRoutes.OutgoingHooks.Handle("", api.ApiSessionRequired(createOutgoingHook)).Methods("POST")
	api.BaseRoutes.OutgoingHooks.Handle("", api.ApiSessionRequired(getOutgoingHooks)).Methods("GET")
	api.BaseRoutes.OutgoingHook.Handle("", api.ApiSessionRequired(getOutgoingHook)).Methods("GET")
	api.BaseRoutes.OutgoingHook.Handle("", api.ApiSessionRequired(updateOutgoingHook)).Methods("PUT")
	api.BaseRoutes.OutgoingHook.Handle("", api.ApiSessionRequired(deleteOutgoingHook)).Methods("DELETE")
	api.BaseRoutes.OutgoingHook.Handle("/regen_token", api.ApiSessionRequired(regenOutgoingHookToken)).Methods("POST")
}

func createIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	hook := model.IncomingWebhookFromJSON(r.Body)
	if hook == nil {
		c.SetInvalidParam("incoming_webhook")
		return
	}

	channel, err := c.App.GetChannel(hook.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("createIncomingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionManageIncomingWebhooks) {
		c.SetPermissionError(model.PermissionManageIncomingWebhooks)
		return
	}

	if channel.Type != model.ChannelTypeOpen && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionReadChannel) {
		c.LogAudit("fail - bad channel permissions")
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	userID := c.AppContext.Session().UserID
	if hook.UserID != "" && hook.UserID != userID {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionManageOthersIncomingWebhooks) {
			c.LogAudit("fail - innapropriate permissions")
			c.SetPermissionError(model.PermissionManageOthersIncomingWebhooks)
			return
		}

		if _, err = c.App.GetUser(hook.UserID); err != nil {
			c.Err = err
			return
		}

		userID = hook.UserID
	}

	incomingHook, err := c.App.CreateIncomingWebhookForChannel(userID, channel, hook)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("hook", incomingHook)
	c.LogAudit("success")

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(incomingHook.ToJSON()))
}

func updateIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookID()
	if c.Err != nil {
		return
	}

	updatedHook := model.IncomingWebhookFromJSON(r.Body)
	if updatedHook == nil {
		c.SetInvalidParam("incoming_webhook")
		return
	}

	// The hook being updated in the payload must be the same one as indicated in the URL.
	if updatedHook.ID != c.Params.HookID {
		c.SetInvalidParam("hook_id")
		return
	}

	auditRec := c.MakeAuditRecord("updateIncomingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", c.Params.HookID)
	c.LogAudit("attempt")

	oldHook, err := c.App.GetIncomingWebhook(c.Params.HookID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("team_id", oldHook.TeamID)

	if updatedHook.TeamID == "" {
		updatedHook.TeamID = oldHook.TeamID
	}

	if updatedHook.TeamID != oldHook.TeamID {
		c.Err = model.NewAppError("updateIncomingHook", "api.webhook.team_mismatch.app_error", nil, "user_id="+c.AppContext.Session().UserID, http.StatusBadRequest)
		return
	}

	channel, err := c.App.GetChannel(updatedHook.ChannelID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("channel_id", channel.ID)
	auditRec.AddMeta("channel_name", channel.Name)

	if channel.TeamID != updatedHook.TeamID {
		c.SetInvalidParam("channel_id")
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionManageIncomingWebhooks) {
		c.SetPermissionError(model.PermissionManageIncomingWebhooks)
		return
	}

	if c.AppContext.Session().UserID != oldHook.UserID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionManageOthersIncomingWebhooks) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersIncomingWebhooks)
		return
	}

	if channel.Type != model.ChannelTypeOpen && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionReadChannel) {
		c.LogAudit("fail - bad channel permissions")
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	incomingHook, err := c.App.UpdateIncomingWebhook(oldHook, updatedHook)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(incomingHook.ToJSON()))
}

func getIncomingHooks(c *Context, w http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("team_id")
	userID := c.AppContext.Session().UserID

	var hooks []*model.IncomingWebhook
	var err *model.AppError

	if teamID != "" {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamID, model.PermissionManageIncomingWebhooks) {
			c.SetPermissionError(model.PermissionManageIncomingWebhooks)
			return
		}

		// Remove userId as a filter if they have permission to manage others.
		if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamID, model.PermissionManageOthersIncomingWebhooks) {
			userID = ""
		}

		hooks, err = c.App.GetIncomingWebhooksForTeamPageByUser(teamID, userID, c.Params.Page, c.Params.PerPage)
	} else {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageIncomingWebhooks) {
			c.SetPermissionError(model.PermissionManageIncomingWebhooks)
			return
		}

		// Remove userId as a filter if they have permission to manage others.
		if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOthersIncomingWebhooks) {
			userID = ""
		}

		hooks, err = c.App.GetIncomingWebhooksPageByUser(userID, c.Params.Page, c.Params.PerPage)
	}

	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.IncomingWebhookListToJSON(hooks)))
}

func getIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookID()
	if c.Err != nil {
		return
	}

	hookID := c.Params.HookID

	var err *model.AppError
	var hook *model.IncomingWebhook
	var channel *model.Channel

	hook, err = c.App.GetIncomingWebhook(hookID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("getIncomingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", hook.ID)
	auditRec.AddMeta("hook_display", hook.DisplayName)
	auditRec.AddMeta("channel_id", hook.ChannelID)
	auditRec.AddMeta("team_id", hook.TeamID)
	c.LogAudit("attempt")

	channel, err = c.App.GetChannel(hook.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageIncomingWebhooks) ||
		(channel.Type != model.ChannelTypeOpen && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), hook.ChannelID, model.PermissionReadChannel)) {
		c.LogAudit("fail - bad permissions")
		c.SetPermissionError(model.PermissionManageIncomingWebhooks)
		return
	}

	if c.AppContext.Session().UserID != hook.UserID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOthersIncomingWebhooks) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersIncomingWebhooks)
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	w.Write([]byte(hook.ToJSON()))
}

func deleteIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookID()
	if c.Err != nil {
		return
	}

	hookID := c.Params.HookID

	var err *model.AppError
	var hook *model.IncomingWebhook
	var channel *model.Channel

	hook, err = c.App.GetIncomingWebhook(hookID)
	if err != nil {
		c.Err = err
		return
	}

	channel, err = c.App.GetChannel(hook.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("deleteIncomingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", hook.ID)
	auditRec.AddMeta("hook_display", hook.DisplayName)
	auditRec.AddMeta("channel_id", channel.ID)
	auditRec.AddMeta("channel_name", channel.Name)
	auditRec.AddMeta("team_id", hook.TeamID)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageIncomingWebhooks) ||
		(channel.Type != model.ChannelTypeOpen && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), hook.ChannelID, model.PermissionReadChannel)) {
		c.LogAudit("fail - bad permissions")
		c.SetPermissionError(model.PermissionManageIncomingWebhooks)
		return
	}

	if c.AppContext.Session().UserID != hook.UserID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOthersIncomingWebhooks) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersIncomingWebhooks)
		return
	}

	if err = c.App.DeleteIncomingWebhook(hookID); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func updateOutgoingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookID()
	if c.Err != nil {
		return
	}

	updatedHook := model.OutgoingWebhookFromJSON(r.Body)
	if updatedHook == nil {
		c.SetInvalidParam("outgoing_webhook")
		return
	}

	// The hook being updated in the payload must be the same one as indicated in the URL.
	if updatedHook.ID != c.Params.HookID {
		c.SetInvalidParam("hook_id")
		return
	}

	auditRec := c.MakeAuditRecord("updateOutgoingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", updatedHook.ID)
	auditRec.AddMeta("hook_display", updatedHook.DisplayName)
	auditRec.AddMeta("channel_id", updatedHook.ChannelID)
	auditRec.AddMeta("team_id", updatedHook.TeamID)
	c.LogAudit("attempt")

	oldHook, err := c.App.GetOutgoingWebhook(c.Params.HookID)
	if err != nil {
		c.Err = err
		return
	}

	if updatedHook.TeamID == "" {
		updatedHook.TeamID = oldHook.TeamID
	}

	if updatedHook.TeamID != oldHook.TeamID {
		c.Err = model.NewAppError("updateOutgoingHook", "api.webhook.team_mismatch.app_error", nil, "user_id="+c.AppContext.Session().UserID, http.StatusBadRequest)
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), updatedHook.TeamID, model.PermissionManageOutgoingWebhooks) {
		c.SetPermissionError(model.PermissionManageOutgoingWebhooks)
		return
	}

	if c.AppContext.Session().UserID != oldHook.CreatorID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), updatedHook.TeamID, model.PermissionManageOthersOutgoingWebhooks) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersOutgoingWebhooks)
		return
	}

	updatedHook.CreatorID = c.AppContext.Session().UserID

	rhook, err := c.App.UpdateOutgoingWebhook(oldHook, updatedHook)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	w.Write([]byte(rhook.ToJSON()))
}

func createOutgoingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	hook := model.OutgoingWebhookFromJSON(r.Body)
	if hook == nil {
		c.SetInvalidParam("outgoing_webhook")
		return
	}

	auditRec := c.MakeAuditRecord("createOutgoingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", hook.ID)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOutgoingWebhooks) {
		c.SetPermissionError(model.PermissionManageOutgoingWebhooks)
		return
	}

	if hook.CreatorID == "" {
		hook.CreatorID = c.AppContext.Session().UserID
	} else {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOthersOutgoingWebhooks) {
			c.LogAudit("fail - innapropriate permissions")
			c.SetPermissionError(model.PermissionManageOthersOutgoingWebhooks)
			return
		}

		_, err := c.App.GetUser(hook.CreatorID)
		if err != nil {
			c.Err = err
			return
		}
	}

	rhook, err := c.App.CreateOutgoingWebhook(hook)
	if err != nil {
		c.LogAudit("fail")
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("hook_display", rhook.DisplayName)
	auditRec.AddMeta("channel_id", rhook.ChannelID)
	auditRec.AddMeta("team_id", rhook.TeamID)
	c.LogAudit("success")

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(rhook.ToJSON()))
}

func getOutgoingHooks(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channel_id")
	teamID := r.URL.Query().Get("team_id")
	userID := c.AppContext.Session().UserID

	var hooks []*model.OutgoingWebhook
	var err *model.AppError

	if channelID != "" {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channelID, model.PermissionManageOutgoingWebhooks) {
			c.SetPermissionError(model.PermissionManageOutgoingWebhooks)
			return
		}

		// Remove userId as a filter if they have permission to manage others.
		if c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channelID, model.PermissionManageOthersOutgoingWebhooks) {
			userID = ""
		}

		hooks, err = c.App.GetOutgoingWebhooksForChannelPageByUser(channelID, userID, c.Params.Page, c.Params.PerPage)
	} else if teamID != "" {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamID, model.PermissionManageOutgoingWebhooks) {
			c.SetPermissionError(model.PermissionManageOutgoingWebhooks)
			return
		}

		// Remove userId as a filter if they have permission to manage others.
		if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamID, model.PermissionManageOthersOutgoingWebhooks) {
			userID = ""
		}

		hooks, err = c.App.GetOutgoingWebhooksForTeamPageByUser(teamID, userID, c.Params.Page, c.Params.PerPage)
	} else {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOutgoingWebhooks) {
			c.SetPermissionError(model.PermissionManageOutgoingWebhooks)
			return
		}

		// Remove userId as a filter if they have permission to manage others.
		if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOthersOutgoingWebhooks) {
			userID = ""
		}

		hooks, err = c.App.GetOutgoingWebhooksPageByUser(userID, c.Params.Page, c.Params.PerPage)
	}

	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.OutgoingWebhookListToJSON(hooks)))
}

func getOutgoingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookID()
	if c.Err != nil {
		return
	}

	hook, err := c.App.GetOutgoingWebhook(c.Params.HookID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("getOutgoingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", hook.ID)
	auditRec.AddMeta("hook_display", hook.DisplayName)
	auditRec.AddMeta("channel_id", hook.ChannelID)
	auditRec.AddMeta("team_id", hook.TeamID)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOutgoingWebhooks) {
		c.SetPermissionError(model.PermissionManageOutgoingWebhooks)
		return
	}

	if c.AppContext.Session().UserID != hook.CreatorID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOthersOutgoingWebhooks) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersOutgoingWebhooks)
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	w.Write([]byte(hook.ToJSON()))
}

func regenOutgoingHookToken(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookID()
	if c.Err != nil {
		return
	}

	hook, err := c.App.GetOutgoingWebhook(c.Params.HookID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("regenOutgoingHookToken", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", hook.ID)
	auditRec.AddMeta("hook_display", hook.DisplayName)
	auditRec.AddMeta("channel_id", hook.ChannelID)
	auditRec.AddMeta("team_id", hook.TeamID)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOutgoingWebhooks) {
		c.SetPermissionError(model.PermissionManageOutgoingWebhooks)
		return
	}

	if c.AppContext.Session().UserID != hook.CreatorID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOthersOutgoingWebhooks) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersOutgoingWebhooks)
		return
	}

	rhook, err := c.App.RegenOutgoingWebhookToken(hook)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	w.Write([]byte(rhook.ToJSON()))
}

func deleteOutgoingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookID()
	if c.Err != nil {
		return
	}

	hook, err := c.App.GetOutgoingWebhook(c.Params.HookID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("deleteOutgoingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", hook.ID)
	auditRec.AddMeta("hook_display", hook.DisplayName)
	auditRec.AddMeta("channel_id", hook.ChannelID)
	auditRec.AddMeta("team_id", hook.TeamID)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOutgoingWebhooks) {
		c.SetPermissionError(model.PermissionManageOutgoingWebhooks)
		return
	}

	if c.AppContext.Session().UserID != hook.CreatorID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamID, model.PermissionManageOthersOutgoingWebhooks) {
		c.LogAudit("fail - inappropriate permissions")
		c.SetPermissionError(model.PermissionManageOthersOutgoingWebhooks)
		return
	}

	if err := c.App.DeleteOutgoingWebhook(hook.ID); err != nil {
		c.LogAudit("fail")
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	ReturnStatusOK(w)
}
