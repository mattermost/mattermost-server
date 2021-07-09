// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitOAuth() {
	api.BaseRoutes.OAuthApps.Handle("", api.APISessionRequired(createOAuthApp)).Methods("POST")
	api.BaseRoutes.OAuthApp.Handle("", api.APISessionRequired(updateOAuthApp)).Methods("PUT")
	api.BaseRoutes.OAuthApps.Handle("", api.APISessionRequired(getOAuthApps)).Methods("GET")
	api.BaseRoutes.OAuthApp.Handle("", api.APISessionRequired(getOAuthApp)).Methods("GET")
	api.BaseRoutes.OAuthApp.Handle("/info", api.APISessionRequired(getOAuthAppInfo)).Methods("GET")
	api.BaseRoutes.OAuthApp.Handle("", api.APISessionRequired(deleteOAuthApp)).Methods("DELETE")
	api.BaseRoutes.OAuthApp.Handle("/regen_secret", api.APISessionRequired(regenerateOAuthAppSecret)).Methods("POST")

	api.BaseRoutes.User.Handle("/oauth/apps/authorized", api.APISessionRequired(getAuthorizedOAuthApps)).Methods("GET")
}

func createOAuthApp(c *Context, w http.ResponseWriter, r *http.Request) {
	oauthApp := model.OAuthAppFromJSON(r.Body)

	if oauthApp == nil {
		c.SetInvalidParam("oauth_app")
		return
	}

	auditRec := c.MakeAuditRecord("createOAuthApp", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOAuth) {
		c.SetPermissionError(model.PermissionManageOAuth)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		oauthApp.IsTrusted = false
	}

	oauthApp.CreatorID = c.AppContext.Session().UserID

	rapp, err := c.App.CreateOAuthApp(oauthApp)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("oauth_app", rapp)
	c.LogAudit("client_id=" + rapp.ID)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(rapp.ToJSON()))
}

func updateOAuthApp(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAppID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("updateOAuthApp", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("oauth_app_id", c.Params.AppID)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOAuth) {
		c.SetPermissionError(model.PermissionManageOAuth)
		return
	}

	oauthApp := model.OAuthAppFromJSON(r.Body)
	if oauthApp == nil {
		c.SetInvalidParam("oauth_app")
		return
	}

	// The app being updated in the payload must be the same one as indicated in the URL.
	if oauthApp.ID != c.Params.AppID {
		c.SetInvalidParam("app_id")
		return
	}

	oldOAuthApp, err := c.App.GetOAuthApp(c.Params.AppID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("oauth_app", oldOAuthApp)

	if c.AppContext.Session().UserID != oldOAuthApp.CreatorID && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystemWideOAuth) {
		c.SetPermissionError(model.PermissionManageSystemWideOAuth)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		oauthApp.IsTrusted = oldOAuthApp.IsTrusted
	}

	updatedOAuthApp, err := c.App.UpdateOAuthApp(oldOAuthApp, oauthApp)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("update", updatedOAuthApp)
	c.LogAudit("success")

	w.Write([]byte(updatedOAuthApp.ToJSON()))
}

func getOAuthApps(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOAuth) {
		c.Err = model.NewAppError("getOAuthApps", "api.command.admin_only.app_error", nil, "", http.StatusForbidden)
		return
	}

	var apps []*model.OAuthApp
	var err *model.AppError
	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystemWideOAuth) {
		apps, err = c.App.GetOAuthApps(c.Params.Page, c.Params.PerPage)
	} else if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOAuth) {
		apps, err = c.App.GetOAuthAppsByCreator(c.AppContext.Session().UserID, c.Params.Page, c.Params.PerPage)
	} else {
		c.SetPermissionError(model.PermissionManageOAuth)
		return
	}

	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.OAuthAppListToJSON(apps)))
}

func getOAuthApp(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAppID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOAuth) {
		c.SetPermissionError(model.PermissionManageOAuth)
		return
	}

	oauthApp, err := c.App.GetOAuthApp(c.Params.AppID)
	if err != nil {
		c.Err = err
		return
	}

	if oauthApp.CreatorID != c.AppContext.Session().UserID && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystemWideOAuth) {
		c.SetPermissionError(model.PermissionManageSystemWideOAuth)
		return
	}

	w.Write([]byte(oauthApp.ToJSON()))
}

func getOAuthAppInfo(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAppID()
	if c.Err != nil {
		return
	}

	oauthApp, err := c.App.GetOAuthApp(c.Params.AppID)
	if err != nil {
		c.Err = err
		return
	}

	oauthApp.Sanitize()
	w.Write([]byte(oauthApp.ToJSON()))
}

func deleteOAuthApp(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAppID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("deleteOAuthApp", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("oauth_app_id", c.Params.AppID)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOAuth) {
		c.SetPermissionError(model.PermissionManageOAuth)
		return
	}

	oauthApp, err := c.App.GetOAuthApp(c.Params.AppID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("oauth_app", oauthApp)

	if c.AppContext.Session().UserID != oauthApp.CreatorID && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystemWideOAuth) {
		c.SetPermissionError(model.PermissionManageSystemWideOAuth)
		return
	}

	err = c.App.DeleteOAuthApp(oauthApp.ID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	ReturnStatusOK(w)
}

func regenerateOAuthAppSecret(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAppID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("regenerateOAuthAppSecret", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("oauth_app_id", c.Params.AppID)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageOAuth) {
		c.SetPermissionError(model.PermissionManageOAuth)
		return
	}

	oauthApp, err := c.App.GetOAuthApp(c.Params.AppID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("oauth_app", oauthApp)

	if oauthApp.CreatorID != c.AppContext.Session().UserID && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystemWideOAuth) {
		c.SetPermissionError(model.PermissionManageSystemWideOAuth)
		return
	}

	oauthApp, err = c.App.RegenerateOAuthAppSecret(oauthApp)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	w.Write([]byte(oauthApp.ToJSON()))
}

func getAuthorizedOAuthApps(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	apps, err := c.App.GetAuthorizedAppsForUser(c.Params.UserID, c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.OAuthAppListToJSON(apps)))
}
