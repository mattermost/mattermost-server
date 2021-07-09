// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitStatus() {
	api.BaseRoutes.User.Handle("/status", api.APISessionRequired(getUserStatus)).Methods("GET")
	api.BaseRoutes.Users.Handle("/status/ids", api.APISessionRequired(getUserStatusesByIDs)).Methods("POST")
	api.BaseRoutes.User.Handle("/status", api.APISessionRequired(updateUserStatus)).Methods("PUT")
	api.BaseRoutes.User.Handle("/status/custom", api.APISessionRequired(updateUserCustomStatus)).Methods("PUT")
	api.BaseRoutes.User.Handle("/status/custom", api.APISessionRequired(removeUserCustomStatus)).Methods("DELETE")

	// Both these handlers are for removing the recent custom status but the one with the POST method should be preferred
	// as DELETE method doesn't support request body in the mobile app.
	api.BaseRoutes.User.Handle("/status/custom/recent", api.APISessionRequired(removeUserRecentCustomStatus)).Methods("DELETE")
	api.BaseRoutes.User.Handle("/status/custom/recent/delete", api.APISessionRequired(removeUserRecentCustomStatus)).Methods("POST")
}

func getUserStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	// No permission check required

	statusMap, err := c.App.GetUserStatusesByIDs([]string{c.Params.UserID})
	if err != nil {
		c.Err = err
		return
	}

	if len(statusMap) == 0 {
		c.Err = model.NewAppError("UserStatus", "api.status.user_not_found.app_error", nil, "", http.StatusNotFound)
		return
	}

	w.Write([]byte(statusMap[0].ToJSON()))
}

func getUserStatusesByIDs(c *Context, w http.ResponseWriter, r *http.Request) {
	userIDs := model.ArrayFromJSON(r.Body)

	if len(userIDs) == 0 {
		c.SetInvalidParam("user_ids")
		return
	}

	for _, userID := range userIDs {
		if len(userID) != 26 {
			c.SetInvalidParam("user_ids")
			return
		}
	}

	// No permission check required

	statusMap, err := c.App.GetUserStatusesByIDs(userIDs)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.StatusListToJSON(statusMap)))
}

func updateUserStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	status := model.StatusFromJSON(r.Body)
	if status == nil {
		c.SetInvalidParam("status")
		return
	}

	// The user being updated in the payload must be the same one as indicated in the URL.
	if status.UserID != c.Params.UserID {
		c.SetInvalidParam("user_id")
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	currentStatus, err := c.App.GetStatus(c.Params.UserID)
	if err == nil && currentStatus.Status == model.StatusOutOfOffice && status.Status != model.StatusOutOfOffice {
		c.App.DisableAutoResponder(c.Params.UserID, c.IsSystemAdmin())
	}

	switch status.Status {
	case "online":
		c.App.SetStatusOnline(c.Params.UserID, true)
	case "offline":
		c.App.SetStatusOffline(c.Params.UserID, true)
	case "away":
		c.App.SetStatusAwayIfNeeded(c.Params.UserID, true)
	case "dnd":
		if c.App.Config().FeatureFlags.TimedDND {
			c.App.SetStatusDoNotDisturbTimed(c.Params.UserID, status.DNDEndTime)
		} else {
			c.App.SetStatusDoNotDisturb(c.Params.UserID)
		}
	default:
		c.SetInvalidParam("status")
		return
	}

	getUserStatus(c, w, r)
}

func updateUserCustomStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().TeamSettings.EnableCustomUserStatuses {
		c.Err = model.NewAppError("updateUserCustomStatus", "api.custom_status.disabled", nil, "", http.StatusNotImplemented)
		return
	}

	customStatus := model.CustomStatusFromJSON(r.Body)
	if customStatus == nil || (customStatus.Emoji == "" && customStatus.Text == "") || !customStatus.AreDurationAndExpirationTimeValid() {
		c.SetInvalidParam("custom_status")
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	customStatus.PreSave()
	err := c.App.SetCustomStatus(c.Params.UserID, customStatus)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func removeUserCustomStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().TeamSettings.EnableCustomUserStatuses {
		c.Err = model.NewAppError("removeUserCustomStatus", "api.custom_status.disabled", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	if err := c.App.RemoveCustomStatus(c.Params.UserID); err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func removeUserRecentCustomStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().TeamSettings.EnableCustomUserStatuses {
		c.Err = model.NewAppError("removeUserRecentCustomStatus", "api.custom_status.disabled", nil, "", http.StatusNotImplemented)
		return
	}

	recentCustomStatus := model.CustomStatusFromJSON(r.Body)
	if recentCustomStatus == nil {
		c.SetInvalidParam("recent_custom_status")
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	if err := c.App.RemoveRecentCustomStatus(c.Params.UserID, recentCustomStatus); err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}
