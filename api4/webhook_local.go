// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitWebhookLocal() {
	api.BaseRoutes.IncomingHooks.Handle("", api.APILocal(localCreateIncomingHook)).Methods("POST")
	api.BaseRoutes.IncomingHooks.Handle("", api.APILocal(getIncomingHooks)).Methods("GET")
	api.BaseRoutes.IncomingHook.Handle("", api.APILocal(getIncomingHook)).Methods("GET")
	api.BaseRoutes.IncomingHook.Handle("", api.APILocal(updateIncomingHook)).Methods("PUT")
	api.BaseRoutes.IncomingHook.Handle("", api.APILocal(deleteIncomingHook)).Methods("DELETE")

	api.BaseRoutes.OutgoingHooks.Handle("", api.APILocal(localCreateOutgoingHook)).Methods("POST")
	api.BaseRoutes.OutgoingHooks.Handle("", api.APILocal(getOutgoingHooks)).Methods("GET")
	api.BaseRoutes.OutgoingHook.Handle("", api.APILocal(getOutgoingHook)).Methods("GET")
	api.BaseRoutes.OutgoingHook.Handle("", api.APILocal(updateOutgoingHook)).Methods("PUT")
	api.BaseRoutes.OutgoingHook.Handle("", api.APILocal(deleteOutgoingHook)).Methods("DELETE")
}

func localCreateIncomingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	hook := model.IncomingWebhookFromJSON(r.Body)
	if hook == nil {
		c.SetInvalidParam("incoming_webhook")
		return
	}

	if hook.UserID == "" {
		c.SetInvalidParam("user_id")
		return
	}

	channel, err := c.App.GetChannel(hook.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	if _, err = c.App.GetUser(hook.UserID); err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("localCreateIncomingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)
	c.LogAudit("attempt")

	incomingHook, err := c.App.CreateIncomingWebhookForChannel(hook.UserID, channel, hook)
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

func localCreateOutgoingHook(c *Context, w http.ResponseWriter, r *http.Request) {
	hook := model.OutgoingWebhookFromJSON(r.Body)
	if hook == nil {
		c.SetInvalidParam("outgoing_webhook")
		return
	}

	auditRec := c.MakeAuditRecord("createOutgoingHook", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", hook.ID)
	c.LogAudit("attempt")

	if hook.CreatorID == "" {
		c.SetInvalidParam("creator_id")
		return
	}

	_, err := c.App.GetUser(hook.CreatorID)
	if err != nil {
		c.Err = err
		return
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
