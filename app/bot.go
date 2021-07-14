// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/app/imaging"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

// CreateBot creates the given bot and corresponding user.
func (a *App) CreateBot(c *request.Context, bot *model.Bot) (*model.Bot, *model.AppError) {
	user, nErr := a.Srv().Store.User().Save(model.UserFromBot(bot))
	if nErr != nil {
		var appErr *model.AppError
		var invErr *store.ErrInvalidInput
		switch {
		case errors.As(nErr, &appErr):
			return nil, appErr
		case errors.As(nErr, &invErr):
			code := ""
			switch invErr.Field {
			case "email":
				code = "app.user.save.email_exists.app_error"
			case "username":
				code = "app.user.save.username_exists.app_error"
			default:
				code = "app.user.save.existing.app_error"
			}
			return nil, model.NewAppError("CreateBot", code, nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("CreateBot", "app.user.save.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}
	bot.UserId = user.Id

	savedBot, nErr := a.Srv().Store.Bot().Save(bot)
	if nErr != nil {
		a.Srv().Store.User().PermanentDelete(bot.UserId)
		var appErr *model.AppError
		switch {
		case errors.As(nErr, &appErr): // in case we haven't converted to plain error.
			return nil, appErr
		default: // last fallback in case it doesn't map to an existing app error.
			return nil, model.NewAppError("CreateBot", "app.bot.createbot.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	// Get the owner of the bot, if one exists. If not, don't send a message
	ownerUser, err := a.Srv().Store.User().Get(context.Background(), bot.OwnerId)
	var nfErr *store.ErrNotFound
	if err != nil && !errors.As(err, &nfErr) {
		return nil, model.NewAppError("CreateBot", "app.user.get.app_error", nil, err.Error(), http.StatusInternalServerError)
	} else if ownerUser != nil {
		// Send a message to the bot's creator to inform them that the bot needs to be added
		// to a team and channel after it's created
		botOwner, err := a.GetUser(bot.OwnerId)
		if err != nil {
			return nil, err
		}
		channel, err := a.getOrCreateDirectChannelWithUser(c, user, botOwner)
		if err != nil {
			return nil, err
		}

		T := i18n.GetUserTranslations(ownerUser.Locale)
		botAddPost := &model.Post{
			Type:      model.POST_ADD_BOT_TEAMS_CHANNELS,
			UserId:    savedBot.UserId,
			ChannelId: channel.Id,
			Message:   T("api.bot.teams_channels.add_message_mobile"),
		}

		if _, err := a.CreatePostAsUser(c, botAddPost, c.Session().Id, true); err != nil {
			return nil, err
		}
	}

	return savedBot, nil
}

//nolint:golint,unused,deadcode
func (a *App) GetWarnMetricsBot() (*model.Bot, *model.AppError) {
	perPage := 1
	userOptions := &model.UserGetOptions{
		Page:     0,
		PerPage:  perPage,
		Role:     model.SYSTEM_ADMIN_ROLE_ID,
		Inactive: false,
	}

	sysAdminList, err := a.GetUsers(userOptions)
	if err != nil {
		return nil, err
	}

	if len(sysAdminList) == 0 {
		return nil, model.NewAppError("GetWarnMetricsBot", "app.bot.get_warn_metrics_bot.empty_admin_list.app_error", nil, "", http.StatusInternalServerError)
	}

	T := i18n.GetUserTranslations(sysAdminList[0].Locale)
	warnMetricsBot := &model.Bot{
		Username:    model.BOT_WARN_METRIC_BOT_USERNAME,
		DisplayName: T("app.system.warn_metric.bot_displayname"),
		Description: "",
		OwnerId:     sysAdminList[0].Id,
	}

	return a.getOrCreateBot(warnMetricsBot)
}

func (a *App) GetSystemBot() (*model.Bot, *model.AppError) {
	perPage := 1
	userOptions := &model.UserGetOptions{
		Page:     0,
		PerPage:  perPage,
		Role:     model.SYSTEM_ADMIN_ROLE_ID,
		Inactive: false,
	}

	sysAdminList, err := a.GetUsers(userOptions)
	if err != nil {
		return nil, err
	}

	if len(sysAdminList) == 0 {
		return nil, model.NewAppError("GetSystemBot", "app.bot.get_system_bot.empty_admin_list.app_error", nil, "", http.StatusInternalServerError)
	}

	T := i18n.GetUserTranslations(sysAdminList[0].Locale)
	systemBot := &model.Bot{
		Username:    model.BOT_SYSTEM_BOT_USERNAME,
		DisplayName: T("app.system.system_bot.bot_displayname"),
		Description: "",
		OwnerId:     sysAdminList[0].Id,
	}

	return a.getOrCreateBot(systemBot)
}

func (a *App) getOrCreateBot(botDef *model.Bot) (*model.Bot, *model.AppError) {
	botUser, appErr := a.GetUserByUsername(botDef.Username)
	if appErr != nil {
		if appErr.StatusCode != http.StatusNotFound {
			return nil, appErr
		}

		// cannot find this bot user, save the user
		user, nErr := a.Srv().Store.User().Save(model.UserFromBot(botDef))
		if nErr != nil {
			var appError *model.AppError
			var invErr *store.ErrInvalidInput
			switch {
			case errors.As(nErr, &appError):
				return nil, appError
			case errors.As(nErr, &invErr):
				code := ""
				switch invErr.Field {
				case "email":
					code = "app.user.save.email_exists.app_error"
				case "username":
					code = "app.user.save.username_exists.app_error"
				default:
					code = "app.user.save.existing.app_error"
				}
				return nil, model.NewAppError("getOrCreateBot", code, nil, invErr.Error(), http.StatusBadRequest)
			default:
				return nil, model.NewAppError("getOrCreateBot", "app.user.save.app_error", nil, nErr.Error(), http.StatusInternalServerError)
			}
		}
		botDef.UserId = user.Id

		//save the bot
		savedBot, nErr := a.Srv().Store.Bot().Save(botDef)
		if nErr != nil {
			a.Srv().Store.User().PermanentDelete(savedBot.UserId)
			var nAppErr *model.AppError
			switch {
			case errors.As(nErr, &nAppErr): // in case we haven't converted to plain error.
				return nil, nAppErr
			default: // last fallback in case it doesn't map to an existing app error.
				return nil, model.NewAppError("getOrCreateBot", "app.bot.createbot.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
			}
		}
		return savedBot, nil
	}

	if botUser == nil {
		return nil, model.NewAppError("getOrCreateBot", "app.bot.createbot.internal_error", nil, "", http.StatusInternalServerError)
	}

	//return the bot for this user
	savedBot, appErr := a.GetBot(botUser.Id, false)
	if appErr != nil {
		return nil, appErr
	}

	return savedBot, nil
}

// PatchBot applies the given patch to the bot and corresponding user.
func (a *App) PatchBot(botUserId string, botPatch *model.BotPatch) (*model.Bot, *model.AppError) {
	bot, err := a.GetBot(botUserId, true)
	if err != nil {
		return nil, err
	}

	if !bot.WouldPatch(botPatch) {
		return bot, nil
	}

	bot.Patch(botPatch)

	user, nErr := a.Srv().Store.User().Get(context.Background(), botUserId)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(nErr, &nfErr):
			return nil, model.NewAppError("PatchBot", MissingAccountError, nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("PatchBot", "app.user.get.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	patchedUser := model.UserFromBot(bot)
	user.Id = patchedUser.Id
	user.Username = patchedUser.Username
	user.Email = patchedUser.Email
	user.FirstName = patchedUser.FirstName

	userUpdate, nErr := a.Srv().Store.User().Update(user, true)
	if nErr != nil {
		var appErr *model.AppError
		var invErr *store.ErrInvalidInput
		var conErr *store.ErrConflict
		switch {
		case errors.As(nErr, &appErr):
			return nil, appErr
		case errors.As(nErr, &invErr):
			return nil, model.NewAppError("PatchBot", "app.user.update.find.app_error", nil, nErr.Error(), http.StatusBadRequest)
		case errors.As(nErr, &conErr):
			if cErr, ok := nErr.(*store.ErrConflict); ok && cErr.Resource == "Username" {
				return nil, model.NewAppError("PatchBot", "app.user.save.username_exists.app_error", nil, nErr.Error(), http.StatusBadRequest)
			}
			return nil, model.NewAppError("PatchBot", "app.user.save.email_exists.app_error", nil, nErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("PatchBot", "app.user.update.finding.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}
	a.InvalidateCacheForUser(user.Id)

	ruser := userUpdate.New
	a.sendUpdatedUserEvent(*ruser)

	bot, nErr = a.Srv().Store.Bot().Update(bot)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(nErr, &nfErr):
			return nil, model.MakeBotNotFoundError(nfErr.ID)
		case errors.As(nErr, &appErr): // in case we haven't converted to plain error.
			return nil, appErr
		default: // last fallback in case it doesn't map to an existing app error.
			return nil, model.NewAppError("PatchBot", "app.bot.patchbot.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}
	return bot, nil
}

// GetBot returns the given bot.
func (a *App) GetBot(botUserId string, includeDeleted bool) (*model.Bot, *model.AppError) {
	bot, err := a.Srv().Store.Bot().Get(botUserId, includeDeleted)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.MakeBotNotFoundError(nfErr.ID)
		default: // last fallback in case it doesn't map to an existing app error.
			return nil, model.NewAppError("GetBot", "app.bot.getbot.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return bot, nil
}

// GetBots returns the requested page of bots.
func (a *App) GetBots(options *model.BotGetOptions) (model.BotList, *model.AppError) {
	bots, err := a.Srv().Store.Bot().GetAll(options)
	if err != nil {
		return nil, model.NewAppError("GetBots", "app.bot.getbots.internal_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return bots, nil
}

// UpdateBotActive marks a bot as active or inactive, along with its corresponding user.
func (a *App) UpdateBotActive(c *request.Context, botUserId string, active bool) (*model.Bot, *model.AppError) {
	user, nErr := a.Srv().Store.User().Get(context.Background(), botUserId)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(nErr, &nfErr):
			return nil, model.NewAppError("PatchBot", MissingAccountError, nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("PatchBot", "app.user.get.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	if _, err := a.UpdateActive(c, user, active); err != nil {
		return nil, err
	}

	bot, nErr := a.Srv().Store.Bot().Get(botUserId, true)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(nErr, &nfErr):
			return nil, model.MakeBotNotFoundError(nfErr.ID)
		default: // last fallback in case it doesn't map to an existing app error.
			return nil, model.NewAppError("UpdateBotActive", "app.bot.getbot.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	changed := true
	if active && bot.DeleteAt != 0 {
		bot.DeleteAt = 0
	} else if !active && bot.DeleteAt == 0 {
		bot.DeleteAt = model.GetMillis()
	} else {
		changed = false
	}

	if changed {
		bot, nErr = a.Srv().Store.Bot().Update(bot)
		if nErr != nil {
			var nfErr *store.ErrNotFound
			var appErr *model.AppError
			switch {
			case errors.As(nErr, &nfErr):
				return nil, model.MakeBotNotFoundError(nfErr.ID)
			case errors.As(nErr, &appErr): // in case we haven't converted to plain error.
				return nil, appErr
			default: // last fallback in case it doesn't map to an existing app error.
				return nil, model.NewAppError("PatchBot", "app.bot.patchbot.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
			}
		}
	}

	return bot, nil
}

// PermanentDeleteBot permanently deletes a bot and its corresponding user.
func (a *App) PermanentDeleteBot(c *request.Context, botUserId string) *model.AppError {
	user, err := a.GetUser(botUserId)
	if err != nil {
		return err
	}

	if !user.IsBot {
		return model.NewAppError("PermanentDeleteBot", "app.bot.permenent_delete.bad_id", map[string]interface{}{"user_id": botUserId}, "invalid bot id", http.StatusBadRequest)
	}

	if err := a.PermanentDeleteUser(c, user); err != nil {
		return model.NewAppError("PermanentDeleteBot", "app.user.permanent_delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

// UpdateBotOwner changes a bot's owner to the given value.
func (a *App) UpdateBotOwner(botUserId, newOwnerId string) (*model.Bot, *model.AppError) {
	bot, err := a.Srv().Store.Bot().Get(botUserId, true)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.MakeBotNotFoundError(nfErr.ID)
		default: // last fallback in case it doesn't map to an existing app error.
			return nil, model.NewAppError("UpdateBotOwner", "app.bot.getbot.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	bot.OwnerId = newOwnerId

	bot, err = a.Srv().Store.Bot().Update(bot)
	if err != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(err, &nfErr):
			return nil, model.MakeBotNotFoundError(nfErr.ID)
		case errors.As(err, &appErr): // in case we haven't converted to plain error.
			return nil, appErr
		default: // last fallback in case it doesn't map to an existing app error.
			return nil, model.NewAppError("PatchBot", "app.bot.patchbot.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return bot, nil
}

// disableUserBots disables all bots owned by the given user.
func (a *App) disableUserBots(c *request.Context, userID string) *model.AppError {
	perPage := 20
	for {
		options := &model.BotGetOptions{
			OwnerId:        userID,
			IncludeDeleted: false,
			OnlyOrphaned:   false,
			Page:           0,
			PerPage:        perPage,
		}
		userBots, err := a.GetBots(options)
		if err != nil {
			return err
		}

		for _, bot := range userBots {
			_, err := a.UpdateBotActive(c, bot.UserId, false)
			if err != nil {
				mlog.Warn("Unable to deactivate bot.", mlog.String("bot_user_id", bot.UserId), mlog.Err(err))
			}
		}

		// Get next set of bots if we got the max number of bots
		if len(userBots) == perPage {
			options.Page += 1
			continue
		}
		break
	}

	return nil
}

func (a *App) notifySysadminsBotOwnerDeactivated(c *request.Context, userID string) *model.AppError {
	perPage := 25
	botOptions := &model.BotGetOptions{
		OwnerId:        userID,
		IncludeDeleted: false,
		OnlyOrphaned:   false,
		Page:           0,
		PerPage:        perPage,
	}
	// get owner bots
	var userBots []*model.Bot
	for {
		bots, err := a.GetBots(botOptions)
		if err != nil {
			return err
		}

		userBots = append(userBots, bots...)

		if len(bots) < perPage {
			break
		}

		botOptions.Page += 1
	}

	// user does not own bots
	if len(userBots) == 0 {
		return nil
	}

	userOptions := &model.UserGetOptions{
		Page:     0,
		PerPage:  perPage,
		Role:     model.SYSTEM_ADMIN_ROLE_ID,
		Inactive: false,
	}
	// get sysadmins
	var sysAdmins []*model.User
	for {
		sysAdminsList, err := a.GetUsers(userOptions)
		if err != nil {
			return err
		}

		sysAdmins = append(sysAdmins, sysAdminsList...)

		if len(sysAdminsList) < perPage {
			break
		}

		userOptions.Page += 1
	}

	// user being disabled
	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	// for each sysadmin, notify user that owns bots was disabled
	for _, sysAdmin := range sysAdmins {
		channel, appErr := a.GetOrCreateDirectChannel(c, sysAdmin.Id, sysAdmin.Id)
		if appErr != nil {
			return appErr
		}

		post := &model.Post{
			UserId:    sysAdmin.Id,
			ChannelId: channel.Id,
			Message:   a.getDisableBotSysadminMessage(user, userBots),
			Type:      model.POST_SYSTEM_GENERIC,
		}

		_, appErr = a.CreatePost(c, post, channel, false, true)
		if appErr != nil {
			return appErr
		}
	}
	return nil
}

func (a *App) getDisableBotSysadminMessage(user *model.User, userBots model.BotList) string {
	disableBotsSetting := *a.Config().ServiceSettings.DisableBotsWhenOwnerIsDeactivated

	var printAllBots = true
	numBotsToPrint := len(userBots)

	if numBotsToPrint > 10 {
		numBotsToPrint = 10
		printAllBots = false
	}

	var message, botList string
	for _, bot := range userBots[:numBotsToPrint] {
		botList += fmt.Sprintf("* %v\n", bot.Username)
	}

	T := i18n.GetUserTranslations(user.Locale)
	message = T("app.bot.get_disable_bot_sysadmin_message",
		map[string]interface{}{
			"UserName":           user.Username,
			"NumBots":            len(userBots),
			"BotNames":           botList,
			"disableBotsSetting": disableBotsSetting,
			"printAllBots":       printAllBots,
		})

	return message
}

// ConvertUserToBot converts a user to bot.
func (a *App) ConvertUserToBot(user *model.User) (*model.Bot, *model.AppError) {
	bot, err := a.Srv().Store.Bot().Save(model.BotFromUser(user))
	if err != nil {
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr): // in case we haven't converted to plain error.
			return nil, appErr
		default: // last fallback in case it doesn't map to an existing app error.
			return nil, model.NewAppError("CreateBot", "app.bot.createbot.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return bot, nil
}

// SetBotIconImageFromMultiPartFile sets LHS icon for a bot.
func (a *App) SetBotIconImageFromMultiPartFile(botUserId string, imageData *multipart.FileHeader) *model.AppError {
	file, err := imageData.Open()
	if err != nil {
		return model.NewAppError("SetBotIconImage", "api.bot.set_bot_icon_image.open.app_error", nil, err.Error(), http.StatusBadRequest)
	}
	defer file.Close()

	file.Seek(0, 0)
	return a.SetBotIconImage(botUserId, file)
}

// SetBotIconImage sets LHS icon for a bot.
func (a *App) SetBotIconImage(botUserId string, file io.ReadSeeker) *model.AppError {
	bot, err := a.GetBot(botUserId, true)
	if err != nil {
		return err
	}

	if _, err := imaging.ParseSVG(file); err != nil {
		return model.NewAppError("SetBotIconImage", "api.bot.set_bot_icon_image.parse.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	// Set icon
	file.Seek(0, 0)
	if _, err = a.WriteFile(file, getBotIconPath(botUserId)); err != nil {
		return model.NewAppError("SetBotIconImage", "api.bot.set_bot_icon_image.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	bot.LastIconUpdate = model.GetMillis()
	if _, err := a.Srv().Store.Bot().Update(bot); err != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(err, &nfErr):
			return model.MakeBotNotFoundError(nfErr.ID)
		case errors.As(err, &appErr): // in case we haven't converted to plain error.
			return appErr
		default: // last fallback in case it doesn't map to an existing app error.
			return model.NewAppError("SetBotIconImage", "app.bot.patchbot.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	a.invalidateUserCacheAndPublish(botUserId)

	return nil
}

// DeleteBotIconImage deletes LHS icon for a bot.
func (a *App) DeleteBotIconImage(botUserId string) *model.AppError {
	bot, err := a.GetBot(botUserId, true)
	if err != nil {
		return err
	}

	// Delete icon
	if err = a.RemoveFile(getBotIconPath(botUserId)); err != nil {
		return model.NewAppError("DeleteBotIconImage", "api.bot.delete_bot_icon_image.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if nErr := a.Srv().Store.User().UpdateLastPictureUpdate(botUserId); nErr != nil {
		mlog.Warn(nErr.Error())
	}

	bot.LastIconUpdate = int64(0)
	if _, err := a.Srv().Store.Bot().Update(bot); err != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(err, &nfErr):
			return model.MakeBotNotFoundError(nfErr.ID)
		case errors.As(err, &appErr): // in case we haven't converted to plain error.
			return appErr
		default: // last fallback in case it doesn't map to an existing app error.
			return model.NewAppError("DeleteBotIconImage", "app.bot.patchbot.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	a.invalidateUserCacheAndPublish(botUserId)

	return nil
}

// GetBotIconImage retrieves LHS icon for a bot.
func (a *App) GetBotIconImage(botUserId string) ([]byte, *model.AppError) {
	if _, err := a.GetBot(botUserId, true); err != nil {
		return nil, err
	}

	data, err := a.ReadFile(getBotIconPath(botUserId))
	if err != nil {
		return nil, model.NewAppError("GetBotIconImage", "api.bot.get_bot_icon_image.read.app_error", nil, err.Error(), http.StatusNotFound)
	}

	return data, nil
}

func getBotIconPath(botUserId string) string {
	return fmt.Sprintf("bots/%v/icon.svg", botUserId)
}
