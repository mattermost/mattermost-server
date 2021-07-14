// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitStatus() {
	api.BaseRoutes.User.Handle("/status", api.ApiSessionRequired(getUserStatus)).Methods("GET")
	api.BaseRoutes.Users.Handle("/status/ids", api.ApiSessionRequired(getUserStatusesByIds)).Methods("POST")
	api.BaseRoutes.User.Handle("/status", api.ApiSessionRequired(updateUserStatus)).Methods("PUT")
	// DND Schedule update
	api.BaseRoutes.User.Handle("/status/schedule", api.ApiSessionRequired(getUserStatusSchedule)).Methods("PUT")
	api.BaseRoutes.User.Handle("/status/schedule/periods", api.ApiSessionRequired(updateStatusSchedule)).Methods("PUT")
	api.BaseRoutes.User.Handle("/status/custom", api.ApiSessionRequired(updateUserCustomStatus)).Methods("PUT")
	api.BaseRoutes.User.Handle("/status/custom", api.ApiSessionRequired(removeUserCustomStatus)).Methods("DELETE")

	// Both these handlers are for removing the recent custom status but the one with the POST method should be preferred
	// as DELETE method doesn't support request body in the mobile app.
	api.BaseRoutes.User.Handle("/status/custom/recent", api.ApiSessionRequired(removeUserRecentCustomStatus)).Methods("DELETE")
	api.BaseRoutes.User.Handle("/status/custom/recent/delete", api.ApiSessionRequired(removeUserRecentCustomStatus)).Methods("POST")
}

func getUserStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	// No permission check required

	statusMap, err := c.App.GetUserStatusesByIds([]string{c.Params.UserId})
	if err != nil {
		c.Err = err
		return
	}

	if len(statusMap) == 0 {
		c.Err = model.NewAppError("UserStatus", "api.status.user_not_found.app_error", nil, "", http.StatusNotFound)
		return
	}

	w.Write([]byte(statusMap[0].ToJson()))
}

func getUserStatusesByIds(c *Context, w http.ResponseWriter, r *http.Request) {
	userIds := model.ArrayFromJson(r.Body)

	if len(userIds) == 0 {
		c.SetInvalidParam("user_ids")
		return
	}

	for _, userId := range userIds {
		if len(userId) != 26 {
			c.SetInvalidParam("user_ids")
			return
		}
	}

	// No permission check required

	statusMap, err := c.App.GetUserStatusesByIds(userIds)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.StatusListToJson(statusMap)))
}

func updateUserStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	status := model.StatusFromJson(r.Body)
	if status == nil {
		c.SetInvalidParam("status")
		return
	}

	// The user being updated in the payload must be the same one as indicated in the URL.
	if status.UserId != c.Params.UserId {
		c.SetInvalidParam("user_id")
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	currentStatus, err := c.App.GetStatus(c.Params.UserId)
	if err == nil && currentStatus.Status == model.STATUS_OUT_OF_OFFICE && status.Status != model.STATUS_OUT_OF_OFFICE {
		c.App.DisableAutoResponder(c.Params.UserId, c.IsSystemAdmin())
	}

	switch status.Status {
	case "online":
		c.App.SetStatusOnline(c.Params.UserId, true)
	case "offline":
		c.App.SetStatusOffline(c.Params.UserId, true)
	case "away":
		c.App.SetStatusAwayIfNeeded(c.Params.UserId, true)
	case "dnd":
		if c.App.Config().FeatureFlags.TimedDND {
			c.App.SetStatusDoNotDisturbTimed(c.Params.UserId, status.DNDEndTime)
		} else {
			c.App.SetStatusDoNotDisturb(c.Params.UserId)
		}
	default:
		c.SetInvalidParam("status")
		return
	}

	getUserStatus(c, w, r)
}

func getUserStatusSchedule(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	status := model.StatusFromJson(r.Body)
	if status == nil {
		c.SetInvalidParam("status")
		return
	}

	// The user being updated in the payload must be the same one as indicated in the URL.
	if status.UserId != c.Params.UserId {
		c.SetInvalidParam("user_id")
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	c.App.SetStatusDoNotDisturbSchedule(status.UserId, status.DayOfTheWeek, status.CurrentTime)

	ReturnStatusOK(w)
}

func updateStatusSchedule(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	status := model.StatusFromJson(r.Body)
	if status == nil {
		c.SetInvalidParam("status")
		return
	}

	// The user being updated in the payload must be the same one as indicated in the URL.
	if status.UserId != c.Params.UserId {
		c.SetInvalidParam("user_id")
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	c.App.SetStatusSchedule(c.Params.UserId, status.MondayStart, status.MondayEnd, status.TuesdayStart, status.TuesdayEnd, status.WednesdayStart, status.WednesdayEnd, status.ThursdayStart, status.ThursdayEnd, status.FridayStart, status.FridayEnd, status.SaturdayStart, status.SaturdayEnd, status.SundayStart, status.SundayEnd, status.Mode)

	ReturnStatusOK(w)
}

func updateUserCustomStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().TeamSettings.EnableCustomUserStatuses {
		c.Err = model.NewAppError("updateUserCustomStatus", "api.custom_status.disabled", nil, "", http.StatusNotImplemented)
		return
	}

	customStatus := model.CustomStatusFromJson(r.Body)
	if customStatus == nil || (customStatus.Emoji == "" && customStatus.Text == "") || !customStatus.AreDurationAndExpirationTimeValid() {
		c.SetInvalidParam("custom_status")
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	customStatus.PreSave()
	err := c.App.SetCustomStatus(c.Params.UserId, customStatus)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func removeUserCustomStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().TeamSettings.EnableCustomUserStatuses {
		c.Err = model.NewAppError("removeUserCustomStatus", "api.custom_status.disabled", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if err := c.App.RemoveCustomStatus(c.Params.UserId); err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func removeUserRecentCustomStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().TeamSettings.EnableCustomUserStatuses {
		c.Err = model.NewAppError("removeUserRecentCustomStatus", "api.custom_status.disabled", nil, "", http.StatusNotImplemented)
		return
	}

	recentCustomStatus := model.CustomStatusFromJson(r.Body)
	if recentCustomStatus == nil {
		c.SetInvalidParam("recent_custom_status")
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if err := c.App.RemoveRecentCustomStatus(c.Params.UserId, recentCustomStatus); err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}
