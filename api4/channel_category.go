// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func getCategoriesForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	categories, err := c.App.GetSidebarCategories(c.Params.UserID, c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write(categories.ToJSON())
}

func createCategoryForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	auditRec := c.MakeAuditRecord("createCategoryForTeamForUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	categoryCreateRequest, err := model.SidebarCategoryFromJSON(r.Body)
	if err != nil || c.Params.UserID != categoryCreateRequest.UserID || c.Params.TeamID != categoryCreateRequest.TeamID {
		c.SetInvalidParam("category")
		return
	}

	if appErr := validateSidebarCategory(c, c.Params.TeamID, c.Params.UserID, categoryCreateRequest); appErr != nil {
		c.Err = appErr
		return
	}

	category, appErr := c.App.CreateSidebarCategory(c.Params.UserID, c.Params.TeamID, categoryCreateRequest)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	w.Write(category.ToJSON())
}

func getCategoryOrderForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	order, err := c.App.GetSidebarCategoryOrder(c.Params.UserID, c.Params.TeamID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.ArrayToJSON(order)))
}

func updateCategoryOrderForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	auditRec := c.MakeAuditRecord("updateCategoryOrderForTeamForUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	categoryOrder := model.ArrayFromJSON(r.Body)

	for _, categoryID := range categoryOrder {
		if !c.App.SessionHasPermissionToCategory(*c.AppContext.Session(), c.Params.UserID, c.Params.TeamID, categoryID) {
			c.SetInvalidParam("category")
			return
		}
	}

	err := c.App.UpdateSidebarCategoryOrder(c.Params.UserID, c.Params.TeamID, categoryOrder)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	w.Write([]byte(model.ArrayToJSON(categoryOrder)))
}

func getCategoryForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID().RequireCategoryID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToCategory(*c.AppContext.Session(), c.Params.UserID, c.Params.TeamID, c.Params.CategoryID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	categories, err := c.App.GetSidebarCategory(c.Params.CategoryID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write(categories.ToJSON())
}

func updateCategoriesForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	auditRec := c.MakeAuditRecord("updateCategoriesForTeamForUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	categoriesUpdateRequest, err := model.SidebarCategoriesFromJSON(r.Body)
	if err != nil {
		c.SetInvalidParam("category")
		return
	}

	for _, category := range categoriesUpdateRequest {
		if !c.App.SessionHasPermissionToCategory(*c.AppContext.Session(), c.Params.UserID, c.Params.TeamID, category.ID) {
			c.SetInvalidParam("category")
			return
		}
	}

	if appErr := validateSidebarCategories(c, c.Params.TeamID, c.Params.UserID, categoriesUpdateRequest); appErr != nil {
		c.Err = appErr
		return
	}

	categories, appErr := c.App.UpdateSidebarCategories(c.Params.UserID, c.Params.TeamID, categoriesUpdateRequest)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	w.Write(model.SidebarCategoriesWithChannelsToJSON(categories))
}

func validateSidebarCategory(c *Context, teamID, userID string, category *model.SidebarCategoryWithChannels) *model.AppError {
	channels, err := c.App.GetChannelsForUser(teamID, userID, true, 0)
	if err != nil {
		return model.NewAppError("validateSidebarCategory", "api.invalid_channel", nil, err.Error(), http.StatusBadRequest)
	}

	category.Channels = validateSidebarCategoryChannels(userID, category.Channels, channels)

	return nil
}

func validateSidebarCategories(c *Context, teamID, userID string, categories []*model.SidebarCategoryWithChannels) *model.AppError {
	channels, err := c.App.GetChannelsForUser(teamID, userID, true, 0)
	if err != nil {
		return model.NewAppError("validateSidebarCategory", "api.invalid_channel", nil, err.Error(), http.StatusBadRequest)
	}

	for _, category := range categories {
		category.Channels = validateSidebarCategoryChannels(userID, category.Channels, channels)
	}

	return nil
}

func validateSidebarCategoryChannels(userID string, channelIDs []string, channels *model.ChannelList) []string {
	var filtered []string

	for _, channelID := range channelIDs {
		found := false
		for _, channel := range *channels {
			if channel.ID == channelID {
				found = true
				break
			}
		}

		if found {
			filtered = append(filtered, channelID)
		} else {
			mlog.Info("Stopping user from adding channel to their sidebar when they are not a member", mlog.String("user_id", userID), mlog.String("channel_id", channelID))
		}
	}

	return filtered
}

func updateCategoryForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID().RequireCategoryID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToCategory(*c.AppContext.Session(), c.Params.UserID, c.Params.TeamID, c.Params.CategoryID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	auditRec := c.MakeAuditRecord("updateCategoryForTeamForUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	categoryUpdateRequest, err := model.SidebarCategoryFromJSON(r.Body)
	if err != nil || categoryUpdateRequest.TeamID != c.Params.TeamID || categoryUpdateRequest.UserID != c.Params.UserID {
		c.SetInvalidParam("category")
		return
	}

	if appErr := validateSidebarCategory(c, c.Params.TeamID, c.Params.UserID, categoryUpdateRequest); appErr != nil {
		c.Err = appErr
		return
	}

	categoryUpdateRequest.ID = c.Params.CategoryID

	categories, appErr := c.App.UpdateSidebarCategories(c.Params.UserID, c.Params.TeamID, []*model.SidebarCategoryWithChannels{categoryUpdateRequest})
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	w.Write(categories[0].ToJSON())
}

func deleteCategoryForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID().RequireCategoryID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToCategory(*c.AppContext.Session(), c.Params.UserID, c.Params.TeamID, c.Params.CategoryID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	auditRec := c.MakeAuditRecord("deleteCategoryForTeamForUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	appErr := c.App.DeleteSidebarCategory(c.Params.UserID, c.Params.TeamID, c.Params.CategoryID)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}
