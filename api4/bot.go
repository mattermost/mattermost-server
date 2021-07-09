// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitBot() {
	api.BaseRoutes.Bots.Handle("", api.APISessionRequired(createBot)).Methods("POST")
	api.BaseRoutes.Bot.Handle("", api.APISessionRequired(patchBot)).Methods("PUT")
	api.BaseRoutes.Bot.Handle("", api.APISessionRequired(getBot)).Methods("GET")
	api.BaseRoutes.Bots.Handle("", api.APISessionRequired(getBots)).Methods("GET")
	api.BaseRoutes.Bot.Handle("/disable", api.APISessionRequired(disableBot)).Methods("POST")
	api.BaseRoutes.Bot.Handle("/enable", api.APISessionRequired(enableBot)).Methods("POST")
	api.BaseRoutes.Bot.Handle("/convert_to_user", api.APISessionRequired(convertBotToUser)).Methods("POST")
	api.BaseRoutes.Bot.Handle("/assign/{user_id:[A-Za-z0-9]+}", api.APISessionRequired(assignBot)).Methods("POST")

	api.BaseRoutes.Bot.Handle("/icon", api.APISessionRequiredTrustRequester(getBotIconImage)).Methods("GET")
	api.BaseRoutes.Bot.Handle("/icon", api.APISessionRequired(setBotIconImage)).Methods("POST")
	api.BaseRoutes.Bot.Handle("/icon", api.APISessionRequired(deleteBotIconImage)).Methods("DELETE")
}

func createBot(c *Context, w http.ResponseWriter, r *http.Request) {
	botPatch := model.BotPatchFromJSON(r.Body)
	if botPatch == nil {
		c.SetInvalidParam("bot")
		return
	}

	bot := &model.Bot{
		OwnerID: c.AppContext.Session().UserID,
	}
	bot.Patch(botPatch)

	auditRec := c.MakeAuditRecord("createBot", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("bot", bot)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionCreateBot) {
		c.SetPermissionError(model.PermissionCreateBot)
		return
	}

	if user, err := c.App.GetUser(c.AppContext.Session().UserID); err == nil {
		if user.IsBot {
			c.SetPermissionError(model.PermissionCreateBot)
			return
		}
	}

	if !*c.App.Config().ServiceSettings.EnableBotAccountCreation {
		c.Err = model.NewAppError("createBot", "api.bot.create_disabled", nil, "", http.StatusForbidden)
		return
	}

	createdBot, err := c.App.CreateBot(c.AppContext, bot)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("bot", createdBot) // overwrite meta

	w.WriteHeader(http.StatusCreated)
	w.Write(createdBot.ToJSON())
}

func patchBot(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireBotUserID()
	if c.Err != nil {
		return
	}
	botUserID := c.Params.BotUserID

	botPatch := model.BotPatchFromJSON(r.Body)
	if botPatch == nil {
		c.SetInvalidParam("bot")
		return
	}

	auditRec := c.MakeAuditRecord("patchBot", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("bot_id", botUserID)

	if err := c.App.SessionHasPermissionToManageBot(*c.AppContext.Session(), botUserID); err != nil {
		c.Err = err
		return
	}

	updatedBot, err := c.App.PatchBot(botUserID, botPatch)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("bot", updatedBot)

	w.Write(updatedBot.ToJSON())
}

func getBot(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireBotUserID()
	if c.Err != nil {
		return
	}
	botUserID := c.Params.BotUserID

	includeDeleted, _ := strconv.ParseBool(r.URL.Query().Get("include_deleted"))

	bot, appErr := c.App.GetBot(botUserID, includeDeleted)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionReadOthersBots) {
		// Allow access to any bot.
	} else if bot.OwnerID == c.AppContext.Session().UserID {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionReadBots) {
			// Pretend like the bot doesn't exist at all to avoid revealing that the
			// user is a bot. It's kind of silly in this case, sine we created the bot,
			// but we don't have read bot permissions.
			c.Err = model.MakeBotNotFoundError(botUserID)
			return
		}
	} else {
		// Pretend like the bot doesn't exist at all, to avoid revealing that the
		// user is a bot.
		c.Err = model.MakeBotNotFoundError(botUserID)
		return
	}

	if c.HandleEtag(bot.Etag(), "Get Bot", w, r) {
		return
	}

	w.Write(bot.ToJSON())
}

func getBots(c *Context, w http.ResponseWriter, r *http.Request) {
	includeDeleted, _ := strconv.ParseBool(r.URL.Query().Get("include_deleted"))
	onlyOrphaned, _ := strconv.ParseBool(r.URL.Query().Get("only_orphaned"))

	var OwnerID string
	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionReadOthersBots) {
		// Get bots created by any user.
		OwnerID = ""
	} else if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionReadBots) {
		// Only get bots created by this user.
		OwnerID = c.AppContext.Session().UserID
	} else {
		c.SetPermissionError(model.PermissionReadBots)
		return
	}

	bots, appErr := c.App.GetBots(&model.BotGetOptions{
		Page:           c.Params.Page,
		PerPage:        c.Params.PerPage,
		OwnerID:        OwnerID,
		IncludeDeleted: includeDeleted,
		OnlyOrphaned:   onlyOrphaned,
	})
	if appErr != nil {
		c.Err = appErr
		return
	}

	if c.HandleEtag(bots.Etag(), "Get Bots", w, r) {
		return
	}

	w.Write(bots.ToJSON())
}

func disableBot(c *Context, w http.ResponseWriter, _ *http.Request) {
	updateBotActive(c, w, false)
}

func enableBot(c *Context, w http.ResponseWriter, _ *http.Request) {
	updateBotActive(c, w, true)
}

func updateBotActive(c *Context, w http.ResponseWriter, active bool) {
	c.RequireBotUserID()
	if c.Err != nil {
		return
	}
	botUserID := c.Params.BotUserID

	auditRec := c.MakeAuditRecord("updateBotActive", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("bot_id", botUserID)
	auditRec.AddMeta("enable", active)

	if err := c.App.SessionHasPermissionToManageBot(*c.AppContext.Session(), botUserID); err != nil {
		c.Err = err
		return
	}

	bot, err := c.App.UpdateBotActive(c.AppContext, botUserID, active)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("bot", bot)

	w.Write(bot.ToJSON())
}

func assignBot(c *Context, w http.ResponseWriter, _ *http.Request) {
	c.RequireUserID()
	c.RequireBotUserID()
	if c.Err != nil {
		return
	}
	botUserID := c.Params.BotUserID
	userID := c.Params.UserID

	auditRec := c.MakeAuditRecord("assignBot", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("bot_id", botUserID)
	auditRec.AddMeta("assign_user_id", userID)

	if err := c.App.SessionHasPermissionToManageBot(*c.AppContext.Session(), botUserID); err != nil {
		c.Err = err
		return
	}

	if user, err := c.App.GetUser(userID); err == nil {
		if user.IsBot {
			c.SetPermissionError(model.PermissionAssignBot)
			return
		}
	}

	bot, err := c.App.UpdateBotOwner(botUserID, userID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("bot", bot)

	w.Write(bot.ToJSON())
}

func getBotIconImage(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireBotUserID()
	if c.Err != nil {
		return
	}
	botUserID := c.Params.BotUserID

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserID, botUserID)
	if err != nil {
		c.Err = err
		return
	}

	if !canSee {
		c.SetPermissionError(model.PermissionViewMembers)
		return
	}

	img, err := c.App.GetBotIconImage(botUserID)
	if err != nil {
		c.Err = err
		return
	}

	user, err := c.App.GetUser(botUserID)
	if err != nil {
		c.Err = err
		return
	}

	etag := strconv.FormatInt(user.LastPictureUpdate, 10)
	if c.HandleEtag(etag, "Get Icon Image", w, r) {
		return
	}

	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%v, private", 24*60*60)) // 24 hrs
	w.Header().Set(model.HeaderEtagServer, etag)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Write(img)
}

func setBotIconImage(c *Context, w http.ResponseWriter, r *http.Request) {
	defer io.Copy(ioutil.Discard, r.Body)

	c.RequireBotUserID()
	if c.Err != nil {
		return
	}
	botUserID := c.Params.BotUserID

	auditRec := c.MakeAuditRecord("setBotIconImage", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("bot_id", botUserID)

	if err := c.App.SessionHasPermissionToManageBot(*c.AppContext.Session(), botUserID); err != nil {
		c.Err = err
		return
	}

	if r.ContentLength > *c.App.Config().FileSettings.MaxFileSize {
		c.Err = model.NewAppError("setBotIconImage", "api.bot.set_bot_icon_image.too_large.app_error", nil, "", http.StatusRequestEntityTooLarge)
		return
	}

	if err := r.ParseMultipartForm(*c.App.Config().FileSettings.MaxFileSize); err != nil {
		c.Err = model.NewAppError("setBotIconImage", "api.bot.set_bot_icon_image.parse.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	m := r.MultipartForm
	imageArray, ok := m.File["image"]
	if !ok {
		c.Err = model.NewAppError("setBotIconImage", "api.bot.set_bot_icon_image.no_file.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if len(imageArray) <= 0 {
		c.Err = model.NewAppError("setBotIconImage", "api.bot.set_bot_icon_image.array.app_error", nil, "", http.StatusBadRequest)
		return
	}

	imageData := imageArray[0]
	if err := c.App.SetBotIconImageFromMultiPartFile(botUserID, imageData); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func deleteBotIconImage(c *Context, w http.ResponseWriter, r *http.Request) {
	defer io.Copy(ioutil.Discard, r.Body)

	c.RequireBotUserID()
	if c.Err != nil {
		return
	}
	botUserID := c.Params.BotUserID

	auditRec := c.MakeAuditRecord("deleteBotIconImage", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("bot_id", botUserID)

	if err := c.App.SessionHasPermissionToManageBot(*c.AppContext.Session(), botUserID); err != nil {
		c.Err = err
		return
	}

	if err := c.App.DeleteBotIconImage(botUserID); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func convertBotToUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireBotUserID()
	if c.Err != nil {
		return
	}

	bot, err := c.App.GetBot(c.Params.BotUserID, false)
	if err != nil {
		c.Err = err
		return
	}

	userPatch := model.UserPatchFromJSON(r.Body)
	if userPatch == nil || userPatch.Password == nil || *userPatch.Password == "" {
		c.SetInvalidParam("userPatch")
		return
	}

	systemAdmin, _ := strconv.ParseBool(r.URL.Query().Get("set_system_admin"))

	auditRec := c.MakeAuditRecord("convertBotToUser", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("bot", bot)
	auditRec.AddMeta("userPatch", userPatch)
	auditRec.AddMeta("set_system_admin", systemAdmin)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	user, err := c.App.ConvertBotToUser(bot, userPatch, systemAdmin)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("convertedTo", user)

	w.Write([]byte(user.ToJSON()))
}
