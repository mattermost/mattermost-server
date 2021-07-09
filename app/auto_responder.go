// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"time"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
)

// check if there is any auto_response type post in channel by the user in a calender day
func (a *App) checkIfRespondedToday(createdAt int64, channelID, userID string) (bool, error) {
	y, m, d := time.Unix(int64(model.GetTimeForMillis(createdAt).Second()), 0).Date()
	since := model.GetMillisForTime(time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
	return a.Srv().Store.Post().HasAutoResponsePostByUserSince(
		model.GetPostsSinceOptions{ChannelID: channelID, Time: since},
		userID,
	)
}

func (a *App) SendAutoResponseIfNecessary(c *request.Context, channel *model.Channel, sender *model.User, post *model.Post) (bool, *model.AppError) {
	if channel.Type != model.ChannelTypeDirect {
		return false, nil
	}

	if sender.IsBot {
		return false, nil
	}

	receiverID := channel.GetOtherUserIDForDM(sender.ID)
	if receiverID == "" {
		// User direct messaged themself, let them test their auto-responder.
		receiverID = sender.ID
	}

	receiver, aErr := a.GetUser(receiverID)
	if aErr != nil {
		return false, aErr
	}

	autoResponded, err := a.checkIfRespondedToday(post.CreateAt, post.ChannelID, receiverID)
	if err != nil {
		return false, model.NewAppError("SendAutoResponseIfNecessary", "app.user.send_auto_response.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	if autoResponded {
		return false, nil
	}

	return a.SendAutoResponse(c, channel, receiver, post)
}

func (a *App) SendAutoResponse(c *request.Context, channel *model.Channel, receiver *model.User, post *model.Post) (bool, *model.AppError) {
	if receiver == nil || receiver.NotifyProps == nil {
		return false, nil
	}

	active := receiver.NotifyProps[model.AutoResponderActiveNotifyProp] == "true"
	message := receiver.NotifyProps[model.AutoResponderMessageNotifyProp]

	if !active || message == "" {
		return false, nil
	}

	rootID := post.ID
	if post.RootID != "" {
		rootID = post.RootID
	}

	autoResponderPost := &model.Post{
		ChannelID: channel.ID,
		Message:   message,
		RootID:    rootID,
		Type:      model.PostTypeAutoResponder,
		UserID:    receiver.ID,
	}

	if _, err := a.CreatePost(c, autoResponderPost, channel, false, false); err != nil {
		return false, err
	}

	return true, nil
}

func (a *App) SetAutoResponderStatus(user *model.User, oldNotifyProps model.StringMap) {
	active := user.NotifyProps[model.AutoResponderActiveNotifyProp] == "true"
	oldActive := oldNotifyProps[model.AutoResponderActiveNotifyProp] == "true"

	autoResponderEnabled := !oldActive && active
	autoResponderDisabled := oldActive && !active

	if autoResponderEnabled {
		a.SetStatusOutOfOffice(user.ID)
	} else if autoResponderDisabled {
		a.SetStatusOnline(user.ID, true)
	}
}

func (a *App) DisableAutoResponder(userID string, asAdmin bool) *model.AppError {
	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	active := user.NotifyProps[model.AutoResponderActiveNotifyProp] == "true"

	if active {
		patch := &model.UserPatch{}
		patch.NotifyProps = user.NotifyProps
		patch.NotifyProps[model.AutoResponderActiveNotifyProp] = "false"

		_, err := a.PatchUser(userID, patch, asAdmin)
		if err != nil {
			return err
		}
	}

	return nil
}
