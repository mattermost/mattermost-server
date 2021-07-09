// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

const (
	TriggerwordsExactMatch = 0
	TriggerwordsStartsWith = 1

	MaxIntegrationResponseSize = 1024 * 1024 // Posts can be <100KB at most, so this is likely more than enough
)

func (a *App) handleWebhookEvents(c *request.Context, post *model.Post, team *model.Team, channel *model.Channel, user *model.User) *model.AppError {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil
	}

	if channel.Type != model.ChannelTypeOpen {
		return nil
	}

	hooks, err := a.Srv().Store.Webhook().GetOutgoingByTeam(team.ID, -1, -1)
	if err != nil {
		return model.NewAppError("handleWebhookEvents", "app.webhooks.get_outgoing_by_team.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if len(hooks) == 0 {
		return nil
	}

	var firstWord, triggerWord string

	splitWords := strings.Fields(post.Message)
	if len(splitWords) > 0 {
		firstWord = splitWords[0]
	}

	relevantHooks := []*model.OutgoingWebhook{}
	for _, hook := range hooks {
		if hook.ChannelID == post.ChannelID || hook.ChannelID == "" {
			if hook.ChannelID == post.ChannelID && len(hook.TriggerWords) == 0 {
				relevantHooks = append(relevantHooks, hook)
				triggerWord = ""
			} else if hook.TriggerWhen == TriggerwordsExactMatch && hook.TriggerWordExactMatch(firstWord) {
				relevantHooks = append(relevantHooks, hook)
				triggerWord = hook.GetTriggerWord(firstWord, true)
			} else if hook.TriggerWhen == TriggerwordsStartsWith && hook.TriggerWordStartsWith(firstWord) {
				relevantHooks = append(relevantHooks, hook)
				triggerWord = hook.GetTriggerWord(firstWord, false)
			}
		}
	}

	for _, hook := range relevantHooks {
		payload := &model.OutgoingWebhookPayload{
			Token:       hook.Token,
			TeamID:      hook.TeamID,
			TeamDomain:  team.Name,
			ChannelID:   post.ChannelID,
			ChannelName: channel.Name,
			Timestamp:   post.CreateAt,
			UserID:      post.UserID,
			UserName:    user.Username,
			PostID:      post.ID,
			Text:        post.Message,
			TriggerWord: triggerWord,
			FileIDs:     strings.Join(post.FileIDs, ","),
		}
		a.Srv().Go(func(hook *model.OutgoingWebhook) func() {
			return func() {
				a.TriggerWebhook(c, payload, hook, post, channel)
			}
		}(hook))
	}

	return nil
}

func (a *App) TriggerWebhook(c *request.Context, payload *model.OutgoingWebhookPayload, hook *model.OutgoingWebhook, post *model.Post, channel *model.Channel) {
	var body io.Reader
	var contentType string
	if hook.ContentType == "application/json" {
		body = strings.NewReader(payload.ToJSON())
		contentType = "application/json"
	} else {
		body = strings.NewReader(payload.ToFormValues())
		contentType = "application/x-www-form-urlencoded"
	}

	for i := range hook.CallbackURLs {
		// Get the callback URL by index to properly capture it for the go func
		url := hook.CallbackURLs[i]

		a.Srv().Go(func() {
			webhookResp, err := a.doOutgoingWebhookRequest(url, body, contentType)
			if err != nil {
				mlog.Error("Event POST failed.", mlog.Err(err))
				return
			}

			if webhookResp != nil && (webhookResp.Text != nil || len(webhookResp.Attachments) > 0) {
				postRootID := ""
				if webhookResp.ResponseType == model.OutgoingHookResponseTypeComment {
					postRootID = post.ID
				}
				if len(webhookResp.Props) == 0 {
					webhookResp.Props = make(model.StringInterface)
				}
				webhookResp.Props["webhook_display_name"] = hook.DisplayName

				text := ""
				if webhookResp.Text != nil {
					text = a.ProcessSlackText(*webhookResp.Text)
				}
				webhookResp.Attachments = a.ProcessSlackAttachments(webhookResp.Attachments)
				// attachments is in here for slack compatibility
				if len(webhookResp.Attachments) > 0 {
					webhookResp.Props["attachments"] = webhookResp.Attachments
				}
				if *a.Config().ServiceSettings.EnablePostUsernameOverride && hook.Username != "" && webhookResp.Username == "" {
					webhookResp.Username = hook.Username
				}

				if *a.Config().ServiceSettings.EnablePostIconOverride && hook.IconURL != "" && webhookResp.IconURL == "" {
					webhookResp.IconURL = hook.IconURL
				}
				if _, err := a.CreateWebhookPost(c, hook.CreatorID, channel, text, webhookResp.Username, webhookResp.IconURL, "", webhookResp.Props, webhookResp.Type, postRootID); err != nil {
					mlog.Error("Failed to create response post.", mlog.Err(err))
				}
			}
		})
	}
}

func (a *App) doOutgoingWebhookRequest(url string, body io.Reader, contentType string) (*model.OutgoingWebhookResponse, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := a.HTTPService().MakeClient(false).Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return model.OutgoingWebhookResponseFromJSON(io.LimitReader(resp.Body, MaxIntegrationResponseSize))
}

func SplitWebhookPost(post *model.Post, maxPostSize int) ([]*model.Post, *model.AppError) {
	splits := make([]*model.Post, 0)
	remainingText := post.Message

	base := post.Clone()
	base.Message = ""
	base.SetProps(make(map[string]interface{}))
	for k, v := range post.GetProps() {
		if k != "attachments" {
			base.AddProp(k, v)
		}
	}

	if utf8.RuneCountInString(model.StringInterfaceToJSON(base.GetProps())) > model.PostPropsMaxUserRunes {
		return nil, model.NewAppError("SplitWebhookPost", "web.incoming_webhook.split_props_length.app_error", map[string]interface{}{"Max": model.PostPropsMaxUserRunes}, "", http.StatusBadRequest)
	}

	for utf8.RuneCountInString(remainingText) > maxPostSize {
		split := base.Clone()
		x := 0
		for index := range remainingText {
			x++
			if x > maxPostSize {
				split.Message = remainingText[:index]
				remainingText = remainingText[index:]
				break
			}
		}
		splits = append(splits, split)
	}

	split := base.Clone()
	split.Message = remainingText
	splits = append(splits, split)

	attachments, _ := post.GetProp("attachments").([]*model.SlackAttachment)
	for _, attachment := range attachments {
		newAttachment := *attachment
		for {
			lastSplit := splits[len(splits)-1]
			newProps := make(map[string]interface{})
			for k, v := range lastSplit.GetProps() {
				newProps[k] = v
			}
			origAttachments, _ := newProps["attachments"].([]*model.SlackAttachment)
			newProps["attachments"] = append(origAttachments, &newAttachment)
			newPropsString := model.StringInterfaceToJSON(newProps)
			runeCount := utf8.RuneCountInString(newPropsString)

			if runeCount <= model.PostPropsMaxUserRunes {
				lastSplit.SetProps(newProps)
				break
			}

			if len(origAttachments) > 0 {
				newSplit := base.Clone()
				splits = append(splits, newSplit)
				continue
			}

			truncationNeeded := runeCount - model.PostPropsMaxUserRunes
			textRuneCount := utf8.RuneCountInString(attachment.Text)
			if textRuneCount < truncationNeeded {
				return nil, model.NewAppError("SplitWebhookPost", "web.incoming_webhook.split_props_length.app_error", map[string]interface{}{"Max": model.PostPropsMaxUserRunes}, "", http.StatusBadRequest)
			}
			x := 0
			for index := range attachment.Text {
				x++
				if x > textRuneCount-truncationNeeded {
					newAttachment.Text = newAttachment.Text[:index]
					break
				}
			}
			lastSplit.SetProps(newProps)
			break
		}
	}

	return splits, nil
}

func (a *App) CreateWebhookPost(c *request.Context, userID string, channel *model.Channel, text, overrideUsername, overrideIconURL, overrideIconEmoji string, props model.StringInterface, postType string, postRootID string) (*model.Post, *model.AppError) {
	// parse links into Markdown format
	linkWithTextRegex := regexp.MustCompile(`<([^\n<\|>]+)\|([^\n>]+)>`)
	text = linkWithTextRegex.ReplaceAllString(text, "[${2}](${1})")

	post := &model.Post{UserID: userID, ChannelID: channel.ID, Message: text, Type: postType, RootID: postRootID}
	post.AddProp("from_webhook", "true")

	if strings.HasPrefix(post.Type, model.PostSystemMessagePrefix) {
		err := model.NewAppError("CreateWebhookPost", "api.context.invalid_param.app_error", map[string]interface{}{"Name": "post.type"}, "", http.StatusBadRequest)
		return nil, err
	}

	if metrics := a.Metrics(); metrics != nil {
		metrics.IncrementWebhookPost()
	}

	if *a.Config().ServiceSettings.EnablePostUsernameOverride {
		if overrideUsername != "" {
			post.AddProp("override_username", overrideUsername)
		} else {
			post.AddProp("override_username", model.DefaultWebhookUsername)
		}
	}

	if *a.Config().ServiceSettings.EnablePostIconOverride {
		if overrideIconURL != "" {
			post.AddProp("override_icon_url", overrideIconURL)
		}
		if overrideIconEmoji != "" {
			post.AddProp("override_icon_emoji", overrideIconEmoji)
		}
	}

	if len(props) > 0 {
		for key, val := range props {
			if key == "attachments" {
				if attachments, success := val.([]*model.SlackAttachment); success {
					model.ParseSlackAttachment(post, attachments)
				}
			} else if key != "override_icon_url" && key != "override_username" && key != "from_webhook" {
				post.AddProp(key, val)
			}
		}
	}

	splits, err := SplitWebhookPost(post, a.MaxPostSize())
	if err != nil {
		return nil, err
	}

	for _, split := range splits {
		if _, err := a.CreatePostMissingChannel(c, split, false); err != nil {
			return nil, model.NewAppError("CreateWebhookPost", "api.post.create_webhook_post.creating.app_error", nil, "err="+err.Message, http.StatusInternalServerError)
		}
	}

	return splits[0], nil
}

func (a *App) CreateIncomingWebhookForChannel(creatorID string, channel *model.Channel, hook *model.IncomingWebhook) (*model.IncomingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableIncomingWebhooks {
		return nil, model.NewAppError("CreateIncomingWebhookForChannel", "api.incoming_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	hook.UserID = creatorID
	hook.TeamID = channel.TeamID

	if !*a.Config().ServiceSettings.EnablePostUsernameOverride {
		hook.Username = ""
	}
	if !*a.Config().ServiceSettings.EnablePostIconOverride {
		hook.IconURL = ""
	}

	if hook.Username != "" && !model.IsValidUsername(hook.Username) {
		return nil, model.NewAppError("CreateIncomingWebhookForChannel", "api.incoming_webhook.invalid_username.app_error", nil, "", http.StatusBadRequest)
	}

	webhook, err := a.Srv().Store.Webhook().SaveIncoming(hook)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &invErr):
			return nil, model.NewAppError("CreateIncomingWebhookForChannel", "app.webhooks.save_incoming.existing.app_error", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("CreateIncomingWebhookForChannel", "app.webhooks.save_incoming.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return webhook, nil
}

func (a *App) UpdateIncomingWebhook(oldHook, updatedHook *model.IncomingWebhook) (*model.IncomingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableIncomingWebhooks {
		return nil, model.NewAppError("UpdateIncomingWebhook", "api.incoming_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	if !*a.Config().ServiceSettings.EnablePostUsernameOverride {
		updatedHook.Username = oldHook.Username
	}
	if !*a.Config().ServiceSettings.EnablePostIconOverride {
		updatedHook.IconURL = oldHook.IconURL
	}

	if updatedHook.Username != "" && !model.IsValidUsername(updatedHook.Username) {
		return nil, model.NewAppError("UpdateIncomingWebhook", "api.incoming_webhook.invalid_username.app_error", nil, "", http.StatusBadRequest)
	}

	updatedHook.ID = oldHook.ID
	updatedHook.UserID = oldHook.UserID
	updatedHook.CreateAt = oldHook.CreateAt
	updatedHook.UpdateAt = model.GetMillis()
	updatedHook.TeamID = oldHook.TeamID
	updatedHook.DeleteAt = oldHook.DeleteAt

	newWebhook, err := a.Srv().Store.Webhook().UpdateIncoming(updatedHook)
	if err != nil {
		return nil, model.NewAppError("UpdateIncomingWebhook", "app.webhooks.update_incoming.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	a.invalidateCacheForWebhook(oldHook.ID)
	return newWebhook, nil
}

func (a *App) DeleteIncomingWebhook(hookID string) *model.AppError {
	if !*a.Config().ServiceSettings.EnableIncomingWebhooks {
		return model.NewAppError("DeleteIncomingWebhook", "api.incoming_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	if err := a.Srv().Store.Webhook().DeleteIncoming(hookID, model.GetMillis()); err != nil {
		return model.NewAppError("DeleteIncomingWebhook", "app.webhooks.delete_incoming.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	a.invalidateCacheForWebhook(hookID)

	return nil
}

func (a *App) GetIncomingWebhook(hookID string) (*model.IncomingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableIncomingWebhooks {
		return nil, model.NewAppError("GetIncomingWebhook", "api.incoming_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	webhook, err := a.Srv().Store.Webhook().GetIncoming(hookID, true)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetIncomingWebhook", "app.webhooks.get_incoming.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetIncomingWebhook", "app.webhooks.get_incoming.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return webhook, nil
}

func (a *App) GetIncomingWebhooksForTeamPage(teamID string, page, perPage int) ([]*model.IncomingWebhook, *model.AppError) {
	return a.GetIncomingWebhooksForTeamPageByUser(teamID, "", page, perPage)
}

func (a *App) GetIncomingWebhooksForTeamPageByUser(teamID string, userID string, page, perPage int) ([]*model.IncomingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableIncomingWebhooks {
		return nil, model.NewAppError("GetIncomingWebhooksForTeamPage", "api.incoming_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	webhooks, err := a.Srv().Store.Webhook().GetIncomingByTeamByUser(teamID, userID, page*perPage, perPage)
	if err != nil {
		return nil, model.NewAppError("GetIncomingWebhooksForTeamPage", "app.webhooks.get_incoming_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return webhooks, nil
}

func (a *App) GetIncomingWebhooksPageByUser(userID string, page, perPage int) ([]*model.IncomingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableIncomingWebhooks {
		return nil, model.NewAppError("GetIncomingWebhooksPageByUser", "api.incoming_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	webhooks, err := a.Srv().Store.Webhook().GetIncomingListByUser(userID, page*perPage, perPage)
	if err != nil {
		return nil, model.NewAppError("GetIncomingWebhooksPageByUser", "app.webhooks.get_incoming_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return webhooks, nil
}

func (a *App) GetIncomingWebhooksPage(page, perPage int) ([]*model.IncomingWebhook, *model.AppError) {
	return a.GetIncomingWebhooksPageByUser("", page, perPage)
}

func (a *App) CreateOutgoingWebhook(hook *model.OutgoingWebhook) (*model.OutgoingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil, model.NewAppError("CreateOutgoingWebhook", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	if hook.ChannelID != "" {
		channel, errCh := a.Srv().Store.Channel().Get(hook.ChannelID, true)
		if errCh != nil {
			var nfErr *store.ErrNotFound
			switch {
			case errors.As(errCh, &nfErr):
				return nil, model.NewAppError("CreateOutgoingWebhook", "app.channel.get.existing.app_error", nil, nfErr.Error(), http.StatusNotFound)
			default:
				return nil, model.NewAppError("CreateOutgoingWebhook", "app.channel.get.find.app_error", nil, errCh.Error(), http.StatusInternalServerError)
			}
		}

		if channel.Type != model.ChannelTypeOpen {
			return nil, model.NewAppError("CreateOutgoingWebhook", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusForbidden)
		}

		if channel.Type != model.ChannelTypeOpen || channel.TeamID != hook.TeamID {
			return nil, model.NewAppError("CreateOutgoingWebhook", "api.webhook.create_outgoing.permissions.app_error", nil, "", http.StatusForbidden)
		}
	} else if len(hook.TriggerWords) == 0 {
		return nil, model.NewAppError("CreateOutgoingWebhook", "api.webhook.create_outgoing.triggers.app_error", nil, "", http.StatusBadRequest)
	}

	allHooks, err := a.Srv().Store.Webhook().GetOutgoingByTeam(hook.TeamID, -1, -1)
	if err != nil {
		return nil, model.NewAppError("CreateOutgoingWebhook", "app.webhooks.get_outgoing_by_team.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	for _, existingOutHook := range allHooks {
		urlIntersect := utils.StringArrayIntersection(existingOutHook.CallbackURLs, hook.CallbackURLs)
		triggerIntersect := utils.StringArrayIntersection(existingOutHook.TriggerWords, hook.TriggerWords)

		if existingOutHook.ChannelID == hook.ChannelID && len(urlIntersect) != 0 && len(triggerIntersect) != 0 {
			return nil, model.NewAppError("CreateOutgoingWebhook", "api.webhook.create_outgoing.intersect.app_error", nil, "", http.StatusInternalServerError)
		}
	}

	webhook, err := a.Srv().Store.Webhook().SaveOutgoing(hook)
	if err != nil {
		var appErr *model.AppError
		var invErr *store.ErrInvalidInput
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &invErr):
			return nil, model.NewAppError("CreateOutgoingWebhook", "app.webhooks.save_outgoing.override.app_error", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("CreateOutgoingWebhook", "app.webhooks.save_outgoing.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return webhook, nil
}

func (a *App) UpdateOutgoingWebhook(oldHook, updatedHook *model.OutgoingWebhook) (*model.OutgoingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil, model.NewAppError("UpdateOutgoingWebhook", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	if updatedHook.ChannelID != "" {
		channel, err := a.GetChannel(updatedHook.ChannelID)
		if err != nil {
			return nil, err
		}

		if channel.Type != model.ChannelTypeOpen {
			return nil, model.NewAppError("UpdateOutgoingWebhook", "api.webhook.create_outgoing.not_open.app_error", nil, "", http.StatusForbidden)
		}

		if channel.TeamID != oldHook.TeamID {
			return nil, model.NewAppError("UpdateOutgoingWebhook", "api.webhook.create_outgoing.permissions.app_error", nil, "", http.StatusForbidden)
		}
	} else if len(updatedHook.TriggerWords) == 0 {
		return nil, model.NewAppError("UpdateOutgoingWebhook", "api.webhook.create_outgoing.triggers.app_error", nil, "", http.StatusInternalServerError)
	}

	allHooks, err := a.Srv().Store.Webhook().GetOutgoingByTeam(oldHook.TeamID, -1, -1)
	if err != nil {
		return nil, model.NewAppError("UpdateOutgoingWebhook", "app.webhooks.get_outgoing_by_team.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, existingOutHook := range allHooks {
		urlIntersect := utils.StringArrayIntersection(existingOutHook.CallbackURLs, updatedHook.CallbackURLs)
		triggerIntersect := utils.StringArrayIntersection(existingOutHook.TriggerWords, updatedHook.TriggerWords)

		if existingOutHook.ChannelID == updatedHook.ChannelID && len(urlIntersect) != 0 && len(triggerIntersect) != 0 && existingOutHook.ID != updatedHook.ID {
			return nil, model.NewAppError("UpdateOutgoingWebhook", "api.webhook.update_outgoing.intersect.app_error", nil, "", http.StatusBadRequest)
		}
	}

	updatedHook.CreatorID = oldHook.CreatorID
	updatedHook.CreateAt = oldHook.CreateAt
	updatedHook.DeleteAt = oldHook.DeleteAt
	updatedHook.TeamID = oldHook.TeamID
	updatedHook.UpdateAt = model.GetMillis()

	webhook, err := a.Srv().Store.Webhook().UpdateOutgoing(updatedHook)
	if err != nil {
		return nil, model.NewAppError("UpdateOutgoingWebhook", "app.webhooks.update_outgoing.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return webhook, nil
}

func (a *App) GetOutgoingWebhook(hookID string) (*model.OutgoingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil, model.NewAppError("GetOutgoingWebhook", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	webhook, err := a.Srv().Store.Webhook().GetOutgoing(hookID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetOutgoingWebhook", "app.webhooks.get_outgoing.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetOutgoingWebhook", "app.webhooks.get_outgoing.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return webhook, nil
}

func (a *App) GetOutgoingWebhooksPage(page, perPage int) ([]*model.OutgoingWebhook, *model.AppError) {
	return a.GetOutgoingWebhooksPageByUser("", page, perPage)
}

func (a *App) GetOutgoingWebhooksPageByUser(userID string, page, perPage int) ([]*model.OutgoingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil, model.NewAppError("GetOutgoingWebhooksPageByUser", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	webhooks, err := a.Srv().Store.Webhook().GetOutgoingListByUser(userID, page*perPage, perPage)
	if err != nil {
		return nil, model.NewAppError("GetOutgoingWebhooksPageByUser", "app.webhooks.get_outgoing_by_channel.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return webhooks, nil
}

func (a *App) GetOutgoingWebhooksForChannelPageByUser(channelID string, userID string, page, perPage int) ([]*model.OutgoingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil, model.NewAppError("GetOutgoingWebhooksForChannelPage", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	webhooks, err := a.Srv().Store.Webhook().GetOutgoingByChannelByUser(channelID, userID, page*perPage, perPage)
	if err != nil {
		return nil, model.NewAppError("GetOutgoingWebhooksForChannelPage", "app.webhooks.get_outgoing_by_channel.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return webhooks, nil
}

func (a *App) GetOutgoingWebhooksForTeamPage(teamID string, page, perPage int) ([]*model.OutgoingWebhook, *model.AppError) {
	return a.GetOutgoingWebhooksForTeamPageByUser(teamID, "", page, perPage)
}

func (a *App) GetOutgoingWebhooksForTeamPageByUser(teamID string, userID string, page, perPage int) ([]*model.OutgoingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil, model.NewAppError("GetOutgoingWebhooksForTeamPageByUser", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	webhooks, err := a.Srv().Store.Webhook().GetOutgoingByTeamByUser(teamID, userID, page*perPage, perPage)
	if err != nil {
		return nil, model.NewAppError("GetOutgoingWebhooksForTeamPageByUser", "app.webhooks.get_outgoing_by_team.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return webhooks, nil
}

func (a *App) DeleteOutgoingWebhook(hookID string) *model.AppError {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return model.NewAppError("DeleteOutgoingWebhook", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	if err := a.Srv().Store.Webhook().DeleteOutgoing(hookID, model.GetMillis()); err != nil {
		return model.NewAppError("DeleteOutgoingWebhook", "app.webhooks.delete_outgoing.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (a *App) RegenOutgoingWebhookToken(hook *model.OutgoingWebhook) (*model.OutgoingWebhook, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableOutgoingWebhooks {
		return nil, model.NewAppError("RegenOutgoingWebhookToken", "api.outgoing_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	hook.Token = model.NewID()

	webhook, err := a.Srv().Store.Webhook().UpdateOutgoing(hook)
	if err != nil {
		return nil, model.NewAppError("RegenOutgoingWebhookToken", "app.webhooks.update_outgoing.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return webhook, nil
}

func (a *App) HandleIncomingWebhook(c *request.Context, hookID string, req *model.IncomingWebhookRequest) *model.AppError {
	if !*a.Config().ServiceSettings.EnableIncomingWebhooks {
		return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	hchan := make(chan store.StoreResult, 1)
	go func() {
		webhook, err := a.Srv().Store.Webhook().GetIncoming(hookID, true)
		hchan <- store.StoreResult{Data: webhook, NErr: err}
		close(hchan)
	}()

	if req == nil {
		return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.parse.app_error", nil, "", http.StatusBadRequest)
	}

	text := req.Text
	if text == "" && req.Attachments == nil {
		return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.text.app_error", nil, "", http.StatusBadRequest)
	}

	channelName := req.ChannelName
	webhookType := req.Type

	var hook *model.IncomingWebhook
	result := <-hchan
	if result.NErr != nil {
		return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.invalid.app_error", nil, result.NErr.Error(), http.StatusBadRequest)
	}
	hook = result.Data.(*model.IncomingWebhook)

	uchan := make(chan store.StoreResult, 1)
	go func() {
		user, err := a.Srv().Store.User().Get(context.Background(), hook.UserID)
		uchan <- store.StoreResult{Data: user, NErr: err}
		close(uchan)
	}()

	if len(req.Props) == 0 {
		req.Props = make(model.StringInterface)
	}

	req.Props["webhook_display_name"] = hook.DisplayName

	text = a.ProcessSlackText(text)
	req.Attachments = a.ProcessSlackAttachments(req.Attachments)
	// attachments is in here for slack compatibility
	if len(req.Attachments) > 0 {
		req.Props["attachments"] = req.Attachments
		webhookType = model.PostTypeSlackAttachment
	}

	var channel *model.Channel
	var cchan chan store.StoreResult

	if channelName != "" {
		if channelName[0] == '@' {
			result, nErr := a.Srv().Store.User().GetByUsername(channelName[1:])
			if nErr != nil {
				return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.user.app_error", nil, nErr.Error(), http.StatusBadRequest)
			}
			ch, err := a.GetOrCreateDirectChannel(c, hook.UserID, result.ID)
			if err != nil {
				return err
			}
			channel = ch
		} else if channelName[0] == '#' {
			cchan = make(chan store.StoreResult, 1)
			go func() {
				chnn, chnnErr := a.Srv().Store.Channel().GetByName(hook.TeamID, channelName[1:], true)
				cchan <- store.StoreResult{Data: chnn, NErr: chnnErr}
				close(cchan)
			}()
		} else {
			cchan = make(chan store.StoreResult, 1)
			go func() {
				chnn, chnnErr := a.Srv().Store.Channel().GetByName(hook.TeamID, channelName, true)
				cchan <- store.StoreResult{Data: chnn, NErr: chnnErr}
				close(cchan)
			}()
		}
	} else {
		var err error
		channel, err = a.Srv().Store.Channel().Get(hook.ChannelID, true)
		if err != nil {
			var nfErr *store.ErrNotFound
			switch {
			case errors.As(err, &nfErr):
				return model.NewAppError("HandleIncomingWebhook", "app.channel.get.existing.app_error", nil, nfErr.Error(), http.StatusNotFound)
			default:
				return model.NewAppError("HandleIncomingWebhook", "app.channel.get.find.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	if channel == nil {
		result2 := <-cchan
		if result2.NErr != nil {
			var nfErr *store.ErrNotFound
			switch {
			case errors.As(result2.NErr, &nfErr):
				return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.channel.app_error", nil, nfErr.Error(), http.StatusNotFound)
			default:
				return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.channel.app_error", nil, result2.NErr.Error(), http.StatusInternalServerError)
			}
		}
		channel = result2.Data.(*model.Channel)
	}

	if hook.ChannelLocked && hook.ChannelID != channel.ID {
		return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.channel_locked.app_error", nil, "", http.StatusForbidden)
	}

	var user *model.User
	result = <-uchan
	if result.NErr != nil {
		return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.user.app_error", nil, result.NErr.Error(), http.StatusForbidden)
	}
	user = result.Data.(*model.User)

	if a.Srv().License() != nil && *a.Config().TeamSettings.ExperimentalTownSquareIsReadOnly &&
		channel.Name == model.DefaultChannelName && !a.RolesGrantPermission(user.GetRoles(), model.PermissionManageSystem.ID) {
		return model.NewAppError("HandleIncomingWebhook", "api.post.create_post.town_square_read_only", nil, "", http.StatusForbidden)
	}

	if channel.Type != model.ChannelTypeOpen && !a.HasPermissionToChannel(hook.UserID, channel.ID, model.PermissionReadChannel) {
		return model.NewAppError("HandleIncomingWebhook", "web.incoming_webhook.permissions.app_error", nil, "", http.StatusForbidden)
	}

	overrideUsername := hook.Username
	if req.Username != "" {
		overrideUsername = req.Username
	}

	overrideIconURL := hook.IconURL
	if req.IconURL != "" {
		overrideIconURL = req.IconURL
	}

	_, err := a.CreateWebhookPost(c, hook.UserID, channel, text, overrideUsername, overrideIconURL, req.IconEmoji, req.Props, webhookType, "")
	return err
}

func (a *App) CreateCommandWebhook(commandID string, args *model.CommandArgs) (*model.CommandWebhook, *model.AppError) {
	hook := &model.CommandWebhook{
		CommandID: commandID,
		UserID:    args.UserID,
		ChannelID: args.ChannelID,
		RootID:    args.RootID,
		ParentID:  args.ParentID,
	}

	savedHook, err := a.Srv().Store.CommandWebhook().Save(hook)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var appErr *model.AppError
		switch {
		case errors.As(err, &invErr):
			return nil, model.NewAppError("CreateCommandWebhook", "app.command_webhook.create_command_webhook.existing", nil, invErr.Error(), http.StatusBadRequest)
		case errors.As(err, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("CreateCommandWebhook", "app.command_webhook.create_command_webhook.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}

	}
	return savedHook, nil
}

func (a *App) HandleCommandWebhook(c *request.Context, hookID string, response *model.CommandResponse) *model.AppError {
	if response == nil {
		return model.NewAppError("HandleCommandWebhook", "app.command_webhook.handle_command_webhook.parse", nil, "", http.StatusBadRequest)
	}

	hook, nErr := a.Srv().Store.CommandWebhook().Get(hookID)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(nErr, &nfErr):
			return model.NewAppError("HandleCommandWebhook", "app.command_webhook.get.missing", map[string]interface{}{"hook_id": hookID}, nfErr.Error(), http.StatusNotFound)
		default:
			return model.NewAppError("HandleCommandWebhook", "app.command_webhook.get.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	cmd, cmdErr := a.Srv().Store.Command().Get(hook.CommandID)
	if cmdErr != nil {
		var appErr *model.AppError
		switch {
		case errors.As(cmdErr, &appErr):
			return appErr
		default:
			return model.NewAppError("HandleCommandWebhook", "web.command_webhook.command.app_error", nil, "err="+cmdErr.Error(), http.StatusBadRequest)
		}
	}

	args := &model.CommandArgs{
		UserID:    hook.UserID,
		ChannelID: hook.ChannelID,
		TeamID:    cmd.TeamID,
		RootID:    hook.RootID,
		ParentID:  hook.ParentID,
	}

	if nErr := a.Srv().Store.CommandWebhook().TryUse(hook.ID, 5); nErr != nil {
		var invErr *store.ErrInvalidInput
		switch {
		case errors.As(nErr, &invErr):
			return model.NewAppError("HandleCommandWebhook", "app.command_webhook.try_use.invalid", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return model.NewAppError("HandleCommandWebhook", "app.command_webhook.try_use.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	_, err := a.HandleCommandResponse(c, cmd, args, response, false)
	return err
}
