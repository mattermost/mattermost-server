// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type notificationType string

const (
	notificationTypeClear       notificationType = "clear"
	notificationTypeMessage     notificationType = "message"
	notificationTypeUpdateBadge notificationType = "update_badge"
	notificationTypeDummy       notificationType = "dummy"
)

type PushNotificationsHub struct {
	notificationsChan chan PushNotification
	app               *App // XXX: This will go away once push notifications move to their own package.
	sema              chan struct{}
	stopChan          chan struct{}
	wg                *sync.WaitGroup
	semaWg            *sync.WaitGroup
	buffer            int
}

type PushNotification struct {
	notificationType   notificationType
	currentSessionID   string
	userID             string
	channelID          string
	post               *model.Post
	user               *model.User
	channel            *model.Channel
	senderName         string
	channelName        string
	explicitMention    bool
	channelWideMention bool
	replyToThreadType  string
}

func (a *App) sendPushNotificationSync(post *model.Post, user *model.User, channel *model.Channel, channelName string, senderName string,
	explicitMention bool, channelWideMention bool, replyToThreadType string) *model.AppError {
	cfg := a.Config()
	message, err := utils.StripMarkdown(post.Message)
	if err != nil {
		mlog.Warn("Failed parse to markdown", mlog.String("post_id", post.ID), mlog.Err(err))
	} else {
		post.Message = message
	}
	msg, appErr := a.BuildPushNotificationMessage(
		*cfg.EmailSettings.PushNotificationContents,
		post,
		user,
		channel,
		channelName,
		senderName,
		explicitMention,
		channelWideMention,
		replyToThreadType,
	)
	if appErr != nil {
		return appErr
	}

	return a.sendPushNotificationToAllSessions(msg, user.ID, "")
}

func (a *App) sendPushNotificationToAllSessions(msg *model.PushNotification, userID string, skipSessionID string) *model.AppError {
	sessions, err := a.getMobileAppSessions(userID)
	if err != nil {
		return err
	}

	if msg == nil {
		return model.NewAppError(
			"pushNotification",
			"api.push_notifications.message.parse.app_error",
			nil,
			"",
			http.StatusBadRequest,
		)
	}

	for _, session := range sessions {
		// Don't send notifications to this session if it's expired or we want to skip it
		if session.IsExpired() || (skipSessionID != "" && skipSessionID == session.ID) {
			continue
		}

		// We made a copy to avoid decoding and parsing all the time
		tmpMessage := msg.DeepCopy()
		tmpMessage.SetDeviceIDAndPlatform(session.DeviceID)
		tmpMessage.AckID = model.NewID()

		err := a.sendToPushProxy(tmpMessage, session)
		if err != nil {
			a.NotificationsLog().Error("Notification error",
				mlog.String("ackId", tmpMessage.AckID),
				mlog.String("type", tmpMessage.Type),
				mlog.String("userId", session.UserID),
				mlog.String("postId", tmpMessage.PostID),
				mlog.String("channelId", tmpMessage.ChannelID),
				mlog.String("deviceId", tmpMessage.DeviceID),
				mlog.String("status", err.Error()),
			)
			continue
		}

		a.NotificationsLog().Info("Notification sent",
			mlog.String("ackId", tmpMessage.AckID),
			mlog.String("type", tmpMessage.Type),
			mlog.String("userId", session.UserID),
			mlog.String("postId", tmpMessage.PostID),
			mlog.String("channelId", tmpMessage.ChannelID),
			mlog.String("deviceId", tmpMessage.DeviceID),
			mlog.String("status", model.PushSendSuccess),
		)

		if a.Metrics() != nil {
			a.Metrics().IncrementPostSentPush()
		}
	}

	return nil
}

func (a *App) sendPushNotification(notification *PostNotification, user *model.User, explicitMention, channelWideMention bool, replyToThreadType string) {
	cfg := a.Config()
	channel := notification.Channel
	post := notification.Post

	nameFormat := a.GetNotificationNameFormat(user)

	channelName := notification.GetChannelName(nameFormat, user.ID)
	senderName := notification.GetSenderName(nameFormat, *cfg.ServiceSettings.EnablePostUsernameOverride)

	select {
	case a.Srv().PushNotificationsHub.notificationsChan <- PushNotification{
		notificationType:   notificationTypeMessage,
		post:               post,
		user:               user,
		channel:            channel,
		senderName:         senderName,
		channelName:        channelName,
		explicitMention:    explicitMention,
		channelWideMention: channelWideMention,
		replyToThreadType:  replyToThreadType,
	}:
	case <-a.Srv().PushNotificationsHub.stopChan:
		return
	}
}

func (a *App) getPushNotificationMessage(contentsConfig, postMessage string, explicitMention, channelWideMention,
	hasFiles bool, senderName, channelType, replyToThreadType string, userLocale i18n.TranslateFunc) string {

	// If the post only has images then push an appropriate message
	if postMessage == "" && hasFiles {
		if channelType == model.ChannelTypeDirect {
			return strings.Trim(userLocale("api.post.send_notifications_and_forget.push_image_only"), " ")
		}
		return senderName + userLocale("api.post.send_notifications_and_forget.push_image_only")
	}

	if contentsConfig == model.FullNotification {
		if channelType == model.ChannelTypeDirect {
			return model.ClearMentionTags(postMessage)
		}
		return senderName + ": " + model.ClearMentionTags(postMessage)
	}

	if channelType == model.ChannelTypeDirect {
		return userLocale("api.post.send_notifications_and_forget.push_message")
	}

	if channelWideMention {
		return senderName + userLocale("api.post.send_notification_and_forget.push_channel_mention")
	}

	if explicitMention {
		return senderName + userLocale("api.post.send_notifications_and_forget.push_explicit_mention")
	}

	if replyToThreadType == model.CommentsNotifyRoot {
		return senderName + userLocale("api.post.send_notification_and_forget.push_comment_on_post")
	}

	if replyToThreadType == model.CommentsNotifyAny {
		return senderName + userLocale("api.post.send_notification_and_forget.push_comment_on_thread")
	}

	return senderName + userLocale("api.post.send_notifications_and_forget.push_general_message")
}

func (a *App) clearPushNotificationSync(currentSessionID, userID, channelID string) *model.AppError {
	msg := &model.PushNotification{
		Type:             model.PushTypeClear,
		Version:          model.PushMessageV2,
		ChannelID:        channelID,
		ContentAvailable: 1,
	}

	unreadCount, err := a.Srv().Store.User().GetUnreadCount(userID)
	if err != nil {
		return model.NewAppError("clearPushNotificationSync", "app.user.get_unread_count.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	msg.Badge = int(unreadCount)

	return a.sendPushNotificationToAllSessions(msg, userID, currentSessionID)
}

func (a *App) clearPushNotification(currentSessionID, userID, channelID string) {
	select {
	case a.Srv().PushNotificationsHub.notificationsChan <- PushNotification{
		notificationType: notificationTypeClear,
		currentSessionID: currentSessionID,
		userID:           userID,
		channelID:        channelID,
	}:
	case <-a.Srv().PushNotificationsHub.stopChan:
		return
	}
}

func (a *App) updateMobileAppBadgeSync(userID string) *model.AppError {
	msg := &model.PushNotification{
		Type:             model.PushTypeUpdateBadge,
		Version:          model.PushMessageV2,
		Sound:            "none",
		ContentAvailable: 1,
	}

	unreadCount, err := a.Srv().Store.User().GetUnreadCount(userID)
	if err != nil {
		return model.NewAppError("updateMobileAppBadgeSync", "app.user.get_unread_count.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	msg.Badge = int(unreadCount)

	return a.sendPushNotificationToAllSessions(msg, userID, "")
}

func (a *App) UpdateMobileAppBadge(userID string) {
	select {
	case a.Srv().PushNotificationsHub.notificationsChan <- PushNotification{
		notificationType: notificationTypeUpdateBadge,
		userID:           userID,
	}:
	case <-a.Srv().PushNotificationsHub.stopChan:
		return
	}
}

func (s *Server) createPushNotificationsHub() {
	buffer := *s.Config().EmailSettings.PushNotificationBuffer
	hub := PushNotificationsHub{
		notificationsChan: make(chan PushNotification, buffer),
		app:               New(ServerConnector(s)),
		wg:                new(sync.WaitGroup),
		semaWg:            new(sync.WaitGroup),
		sema:              make(chan struct{}, runtime.NumCPU()*8), // numCPU * 8 is a good amount of concurrency.
		stopChan:          make(chan struct{}),
		buffer:            buffer,
	}
	go hub.start()
	s.PushNotificationsHub = hub
}

func (hub *PushNotificationsHub) start() {
	hub.wg.Add(1)
	defer hub.wg.Done()
	for {
		select {
		case notification := <-hub.notificationsChan:
			// We just ignore dummy notifications.
			// These are used to pump out any remaining notifications
			// before we stop the hub.
			if notification.notificationType == notificationTypeDummy {
				continue
			}
			// Adding to the waitgroup first.
			hub.semaWg.Add(1)
			// Get token.
			hub.sema <- struct{}{}
			go func(notification PushNotification) {
				defer func() {
					// Release token.
					<-hub.sema
					// Now marking waitgroup as done.
					hub.semaWg.Done()
				}()

				var err *model.AppError
				switch notification.notificationType {
				case notificationTypeClear:
					err = hub.app.clearPushNotificationSync(notification.currentSessionID, notification.userID, notification.channelID)
				case notificationTypeMessage:
					err = hub.app.sendPushNotificationSync(
						notification.post,
						notification.user,
						notification.channel,
						notification.channelName,
						notification.senderName,
						notification.explicitMention,
						notification.channelWideMention,
						notification.replyToThreadType,
					)
				case notificationTypeUpdateBadge:
					err = hub.app.updateMobileAppBadgeSync(notification.userID)
				default:
					mlog.Debug("Invalid notification type", mlog.String("notification_type", string(notification.notificationType)))
				}

				if err != nil {
					mlog.Error("Unable to send push notification", mlog.String("notification_type", string(notification.notificationType)), mlog.Err(err))
				}
			}(notification)
		case <-hub.stopChan:
			return
		}
	}
}

func (hub *PushNotificationsHub) stop() {
	// Drain the channel.
	for i := 0; i < hub.buffer+1; i++ {
		hub.notificationsChan <- PushNotification{
			notificationType: notificationTypeDummy,
		}
	}
	close(hub.stopChan)
	// We need to wait for the outer for loop to exit first.
	// We cannot just send struct{}{} to stopChan because there are
	// other listeners to the channel. And sending just once
	// will cause a race.
	hub.wg.Wait()
	// And then we wait for the semaphore to finish.
	hub.semaWg.Wait()
}

func (s *Server) StopPushNotificationsHubWorkers() {
	s.PushNotificationsHub.stop()
}

func (a *App) sendToPushProxy(msg *model.PushNotification, session *model.Session) error {
	msg.ServerID = a.TelemetryID()

	a.NotificationsLog().Info("Notification will be sent",
		mlog.String("ackId", msg.AckID),
		mlog.String("type", msg.Type),
		mlog.String("userId", session.UserID),
		mlog.String("postId", msg.PostID),
		mlog.String("status", model.PushSendPrepare),
	)

	url := strings.TrimRight(*a.Config().EmailSettings.PushNotificationServer, "/") + model.ApiUrlSuffixV1 + "/send_push"
	request, err := http.NewRequest("POST", url, strings.NewReader(msg.ToJSON()))
	if err != nil {
		return err
	}

	resp, err := a.Srv().pushNotificationClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	pushResponse := model.PushResponseFromJSON(resp.Body)

	switch pushResponse[model.PushStatus] {
	case model.PushStatusRemove:
		a.AttachDeviceID(session.ID, "", session.ExpiresAt)
		a.ClearSessionCacheForUser(session.UserID)
		return errors.New("Device was reported as removed")
	case model.PushStatusFail:
		return errors.New(pushResponse[model.PushStatusErrorMsg])
	}
	return nil
}

func (a *App) SendAckToPushProxy(ack *model.PushNotificationAck) error {
	if ack == nil {
		return nil
	}

	a.NotificationsLog().Info("Notification received",
		mlog.String("ackId", ack.ID),
		mlog.String("type", ack.NotificationType),
		mlog.String("deviceType", ack.ClientPlatform),
		mlog.Int64("receivedAt", ack.ClientReceivedAt),
		mlog.String("status", model.PushReceived),
	)

	request, err := http.NewRequest(
		"POST",
		strings.TrimRight(*a.Config().EmailSettings.PushNotificationServer, "/")+model.ApiUrlSuffixV1+"/ack",
		strings.NewReader(ack.ToJSON()),
	)

	if err != nil {
		return err
	}

	resp, err := a.Srv().pushNotificationClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Reading the body to completion.
	_, err = io.Copy(ioutil.Discard, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (a *App) getMobileAppSessions(userID string) ([]*model.Session, *model.AppError) {
	sessions, err := a.Srv().Store.Session().GetSessionsWithActiveDeviceIDs(userID)
	if err != nil {
		return nil, model.NewAppError("getMobileAppSessions", "app.session.get_sessions.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return sessions, nil
}

func ShouldSendPushNotification(user *model.User, channelNotifyProps model.StringMap, wasMentioned bool, status *model.Status, post *model.Post) bool {
	return DoesNotifyPropsAllowPushNotification(user, channelNotifyProps, post, wasMentioned) &&
		DoesStatusAllowPushNotification(user.NotifyProps, status, post.ChannelID)
}

func DoesNotifyPropsAllowPushNotification(user *model.User, channelNotifyProps model.StringMap, post *model.Post, wasMentioned bool) bool {
	userNotifyProps := user.NotifyProps
	userNotify := userNotifyProps[model.PushNotifyProp]
	channelNotify, ok := channelNotifyProps[model.PushNotifyProp]
	if !ok || channelNotify == "" {
		channelNotify = model.ChannelNotifyDefault
	}

	// If the channel is muted do not send push notifications
	if channelNotifyProps[model.MarkUnreadNotifyProp] == model.ChannelMarkUnreadMention {
		return false
	}

	if post.IsSystemMessage() {
		return false
	}

	if channelNotify == model.UserNotifyNone {
		return false
	}

	if channelNotify == model.ChannelNotifyMention && !wasMentioned {
		return false
	}

	if userNotify == model.UserNotifyMention && channelNotify == model.ChannelNotifyDefault && !wasMentioned {
		return false
	}

	if (userNotify == model.UserNotifyAll || channelNotify == model.ChannelNotifyAll) &&
		(post.UserID != user.ID || post.GetProp("from_webhook") == "true") {
		return true
	}

	if userNotify == model.UserNotifyNone &&
		channelNotify == model.ChannelNotifyDefault {
		return false
	}

	return true
}

func DoesStatusAllowPushNotification(userNotifyProps model.StringMap, status *model.Status, channelID string) bool {
	// If User status is DND or OOO return false right away
	if status.Status == model.StatusDnd || status.Status == model.StatusOutOfOffice {
		return false
	}

	pushStatus, ok := userNotifyProps[model.PushStatusNotifyProp]
	if (pushStatus == model.StatusOnline || !ok) && (status.ActiveChannel != channelID || model.GetMillis()-status.LastActivityAt > model.StatusChannelTimeout) {
		return true
	}

	if pushStatus == model.StatusAway && (status.Status == model.StatusAway || status.Status == model.StatusOffline) {
		return true
	}

	if pushStatus == model.StatusOffline && status.Status == model.StatusOffline {
		return true
	}

	return false
}

func (a *App) BuildPushNotificationMessage(contentsConfig string, post *model.Post, user *model.User, channel *model.Channel, channelName string, senderName string,
	explicitMention bool, channelWideMention bool, replyToThreadType string) (*model.PushNotification, *model.AppError) {

	var msg *model.PushNotification

	notificationInterface := a.Srv().Notification
	if (notificationInterface == nil || notificationInterface.CheckLicense() != nil) && contentsConfig == model.IDLoadedNotification {
		contentsConfig = model.GenericNotification
	}

	if contentsConfig == model.IDLoadedNotification {
		msg = a.buildIDLoadedPushNotificationMessage(post, user)
	} else {
		msg = a.buildFullPushNotificationMessage(contentsConfig, post, user, channel, channelName, senderName, explicitMention, channelWideMention, replyToThreadType)
	}

	unreadCount, err := a.Srv().Store.User().GetUnreadCount(user.ID)
	if err != nil {
		return nil, model.NewAppError("BuildPushNotificationMessage", "app.user.get_unread_count.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	msg.Badge = int(unreadCount)

	return msg, nil
}

func (a *App) buildIDLoadedPushNotificationMessage(post *model.Post, user *model.User) *model.PushNotification {
	userLocale := i18n.GetUserTranslations(user.Locale)
	msg := &model.PushNotification{
		PostID:     post.ID,
		ChannelID:  post.ChannelID,
		Category:   model.CategoryCanReply,
		Version:    model.PushMessageV2,
		Type:       model.PushTypeMessage,
		IsIDLoaded: true,
		SenderID:   user.ID,
		Message:    userLocale("api.push_notification.id_loaded.default_message"),
	}

	return msg
}

func (a *App) buildFullPushNotificationMessage(contentsConfig string, post *model.Post, user *model.User, channel *model.Channel, channelName string, senderName string,
	explicitMention bool, channelWideMention bool, replyToThreadType string) *model.PushNotification {

	msg := &model.PushNotification{
		Category:   model.CategoryCanReply,
		Version:    model.PushMessageV2,
		Type:       model.PushTypeMessage,
		TeamID:     channel.TeamID,
		ChannelID:  channel.ID,
		PostID:     post.ID,
		RootID:     post.RootID,
		SenderID:   post.UserID,
		IsIDLoaded: false,
	}

	cfg := a.Config()
	if contentsConfig != model.GenericNoChannelNotification || channel.Type == model.ChannelTypeDirect {
		msg.ChannelName = channelName
	}

	msg.SenderName = senderName
	if ou, ok := post.GetProp("override_username").(string); ok && *cfg.ServiceSettings.EnablePostUsernameOverride {
		msg.OverrideUsername = ou
		msg.SenderName = ou
	}

	if oi, ok := post.GetProp("override_icon_url").(string); ok && *cfg.ServiceSettings.EnablePostIconOverride {
		msg.OverrideIconUrl = oi
	}

	if fw, ok := post.GetProp("from_webhook").(string); ok {
		msg.FromWebhook = fw
	}

	postMessage := post.Message
	for _, attachment := range post.Attachments() {
		if attachment.Fallback != "" {
			postMessage += "\n" + attachment.Fallback
		}
	}

	userLocale := i18n.GetUserTranslations(user.Locale)
	hasFiles := post.FileIDs != nil && len(post.FileIDs) > 0

	msg.Message = a.getPushNotificationMessage(
		contentsConfig,
		postMessage,
		explicitMention,
		channelWideMention,
		hasFiles,
		msg.SenderName,
		channel.Type,
		replyToThreadType,
		userLocale,
	)

	return msg
}
