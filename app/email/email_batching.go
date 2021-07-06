// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package email

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	EmailBatchingTaskName = "Email Batching"
)

type postData struct {
	SenderName  string
	ChannelName string
	Message     template.HTML
	MessageURL  string
	SenderPhoto string
	PostPhoto   string
	Time        string
}

func (es *Service) InitEmailBatching() {
	if *es.config().EmailSettings.EnableEmailBatching {
		if es.EmailBatching == nil {
			es.EmailBatching = NewEmailBatchingJob(es, *es.config().EmailSettings.EmailBatchingBufferSize)
		}

		// note that we don't support changing EmailBatchingBufferSize without restarting the server

		es.EmailBatching.Start()
	}
}

func (es *Service) AddNotificationEmailToBatch(user *model.User, post *model.Post, team *model.Team) *model.AppError {
	if !*es.config().EmailSettings.EnableEmailBatching {
		return model.NewAppError("AddNotificationEmailToBatch", "api.email_batching.add_notification_email_to_batch.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	if !es.EmailBatching.Add(user, post, team) {
		mlog.Error("Email batching job's receiving channel was full. Please increase the EmailBatchingBufferSize.")
		return model.NewAppError("AddNotificationEmailToBatch", "api.email_batching.add_notification_email_to_batch.channel_full.app_error", nil, "", http.StatusInternalServerError)
	}

	return nil
}

type batchedNotification struct {
	userID   string
	post     *model.Post
	teamName string
}

type EmailBatchingJob struct {
	config  func() *model.Config
	service *Service

	newNotifications     chan *batchedNotification
	pendingNotifications map[string][]*batchedNotification
	task                 *model.ScheduledTask
	taskMutex            sync.Mutex
}

func NewEmailBatchingJob(es *Service, bufferSize int) *EmailBatchingJob {
	return &EmailBatchingJob{
		config:               es.config,
		service:              es,
		newNotifications:     make(chan *batchedNotification, bufferSize),
		pendingNotifications: make(map[string][]*batchedNotification),
	}
}

func (job *EmailBatchingJob) Start() {
	mlog.Debug("Email batching job starting. Checking for pending emails periodically.", mlog.Int("interval_in_seconds", *job.config().EmailSettings.EmailBatchingInterval))
	newTask := model.CreateRecurringTask(EmailBatchingTaskName, job.CheckPendingEmails, time.Duration(*job.config().EmailSettings.EmailBatchingInterval)*time.Second)

	job.taskMutex.Lock()
	oldTask := job.task
	job.task = newTask
	job.taskMutex.Unlock()

	if oldTask != nil {
		oldTask.Cancel()
	}
}

func (job *EmailBatchingJob) Add(user *model.User, post *model.Post, team *model.Team) bool {
	notification := &batchedNotification{
		userID:   user.Id,
		post:     post,
		teamName: team.Name,
	}

	select {
	case job.newNotifications <- notification:
		return true
	default:
		// return false if we couldn't queue the email notification so that we can send an immediate email
		return false
	}
}

func (job *EmailBatchingJob) CheckPendingEmails() {
	job.handleNewNotifications()

	// it's a bit weird to pass the send email function through here, but it makes it so that we can test
	// without actually sending emails
	job.checkPendingNotifications(time.Now(), job.service.sendBatchedEmailNotification)

	mlog.Debug("Email batching job ran. Some users still have notifications pending.", mlog.Int("number_of_users", len(job.pendingNotifications)))
}

func (job *EmailBatchingJob) handleNewNotifications() {
	receiving := true

	// read in new notifications to send
	for receiving {
		select {
		case notification := <-job.newNotifications:
			userID := notification.userID

			if _, ok := job.pendingNotifications[userID]; !ok {
				job.pendingNotifications[userID] = []*batchedNotification{notification}
			} else {
				job.pendingNotifications[userID] = append(job.pendingNotifications[userID], notification)
			}
		default:
			receiving = false
		}
	}
}

func (job *EmailBatchingJob) checkPendingNotifications(now time.Time, handler func(string, []*batchedNotification)) {
	for userID, notifications := range job.pendingNotifications {
		batchStartTime := notifications[0].post.CreateAt
		inspectedTeamNames := make(map[string]string)
		for _, notification := range notifications {
			// at most, we'll do one check for each team that notifications were sent for
			if inspectedTeamNames[notification.teamName] != "" {
				continue
			}

			team, nErr := job.service.store.Team().GetByName(notifications[0].teamName)
			if nErr != nil {
				mlog.Error("Unable to find Team id for notification", mlog.Err(nErr))
				continue
			}

			if team != nil {
				inspectedTeamNames[notification.teamName] = team.Id
			}

			// if the user has viewed any channels in this team since the notification was queued, delete
			// all queued notifications
			channelMembers, err := job.service.store.Channel().GetMembersForUser(inspectedTeamNames[notification.teamName], userID)
			if err != nil {
				mlog.Error("Unable to find ChannelMembers for user", mlog.Err(err))
				continue
			}

			for _, channelMember := range *channelMembers {
				if channelMember.LastViewedAt >= batchStartTime {
					mlog.Debug("Deleted notifications for user", mlog.String("user_id", userID))
					delete(job.pendingNotifications, userID)
					break
				}
			}
		}

		// get how long we need to wait to send notifications to the user
		var interval int64
		preference, err := job.service.store.Preference().Get(userID, model.PREFERENCE_CATEGORY_NOTIFICATIONS, model.PREFERENCE_NAME_EMAIL_INTERVAL)
		if err != nil {
			// use the default batching interval if an error ocurrs while fetching user preferences
			interval, _ = strconv.ParseInt(model.PREFERENCE_EMAIL_INTERVAL_BATCHING_SECONDS, 10, 64)
		} else {
			if value, err := strconv.ParseInt(preference.Value, 10, 64); err != nil {
				// // use the default batching interval if an error ocurrs while deserializing user preferences
				interval, _ = strconv.ParseInt(model.PREFERENCE_EMAIL_INTERVAL_BATCHING_SECONDS, 10, 64)
			} else {
				interval = value
			}
		}

		// send the email notification if there are notifications to send AND it's been long enough
		if len(job.pendingNotifications[userID]) > 0 && now.Sub(time.Unix(batchStartTime/1000, 0)) > time.Duration(interval)*time.Second {
			job.service.goFn(func(userID string, notifications []*batchedNotification) func() {
				return func() {
					handler(userID, notifications)
				}
			}(userID, job.pendingNotifications[userID]))
			delete(job.pendingNotifications, userID)
		}
	}
}

func (es *Service) sendBatchedEmailNotification(userID string, notifications []*batchedNotification) {
	user, err := es.userService.GetUser(userID)
	if err != nil {
		mlog.Warn("Unable to find recipient for batched email notification")
		return
	}

	translateFunc := i18n.GetUserTranslations(user.Locale)
	displayNameFormat := *es.config().TeamSettings.TeammateNameDisplay
	siteURL := *es.config().ServiceSettings.SiteURL

	postsData := make([]*postData, 0 /* len */, len(notifications) /* cap */)
	embeddedFiles := make(map[string]io.Reader)

	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	if license := es.license(); license != nil && *license.Features.EmailNotificationContents {
		emailNotificationContentsType = *es.config().EmailSettings.EmailNotificationContentsType
	}

	if emailNotificationContentsType == model.EMAIL_NOTIFICATION_CONTENTS_FULL {
		for i, notification := range notifications {
			sender, errSender := es.userService.GetUser(notification.post.UserId)
			if errSender != nil {
				mlog.Warn("Unable to find sender of post for batched email notification")
			}

			channel, errCh := es.store.Channel().Get(notification.post.ChannelId, true)
			if errCh != nil {
				mlog.Warn("Unable to find channel of post for batched email notification")
			}

			senderProfileImage, _, errProfileImage := es.userService.GetProfileImage(sender)
			if errProfileImage != nil {
				mlog.Warn("Unable to get the sender user profile image.", mlog.String("user_id", sender.Id), mlog.Err(errProfileImage))
			}

			senderPhoto := fmt.Sprintf("user-avatar-%d.png", i)
			if senderProfileImage != nil {
				embeddedFiles[senderPhoto] = bytes.NewReader(senderProfileImage)
			}

			tm := time.Unix(notification.post.CreateAt/1000, 0)
			timezone, _ := tm.Zone()

			t := translateFunc("api.email_batching.send_batched_email_notification.time", map[string]interface{}{
				"Hour":     tm.Hour(),
				"Minute":   fmt.Sprintf("%02d", tm.Minute()),
				"Month":    translateFunc(tm.Month().String()),
				"Day":      tm.Day(),
				"Year":     tm.Year(),
				"Timezone": timezone,
			})

			MessageURL := siteURL + "/" + notification.teamName + "/pl/" + notification.post.Id

			postsData = append(postsData, &postData{
				SenderPhoto: senderPhoto,
				SenderName:  sender.GetDisplayName(displayNameFormat),
				Time:        t,
				ChannelName: channel.DisplayName,
				Message:     template.HTML(es.GetMessageForNotification(notification.post, translateFunc)),
				MessageURL:  MessageURL,
			})
		}
	}

	tm := time.Unix(notifications[0].post.CreateAt/1000, 0)

	subject := translateFunc("api.email_batching.send_batched_email_notification.subject", len(notifications), map[string]interface{}{
		"SiteName": es.config().TeamSettings.SiteName,
		"Year":     tm.Year(),
		"Month":    translateFunc(tm.Month().String()),
		"Day":      tm.Day(),
	})

	firstSender, err := es.userService.GetUser(notifications[0].post.UserId)
	if err != nil {
		mlog.Warn("Unable to find sender of post for batched email notification")
	}

	data := es.NewEmailTemplateData(user.Locale)
	data.Props["SiteURL"] = siteURL
	data.Props["Title"] = translateFunc("api.email_batching.send_batched_email_notification.title", len(notifications)-1, map[string]interface{}{
		"SenderName": firstSender.GetDisplayName(displayNameFormat),
	})
	data.Props["SubTitle"] = translateFunc("api.email_batching.send_batched_email_notification.subTitle")
	data.Props["Button"] = translateFunc("api.email_batching.send_batched_email_notification.button")
	data.Props["ButtonURL"] = siteURL
	data.Props["Posts"] = postsData
	data.Props["MessageButton"] = translateFunc("api.email_batching.send_batched_email_notification.messageButton")
	data.Props["NotificationFooterTitle"] = translateFunc("app.notification.footer.title")
	data.Props["NotificationFooterInfoLogin"] = translateFunc("app.notification.footer.infoLogin")
	data.Props["NotificationFooterInfo"] = translateFunc("app.notification.footer.info")

	renderedPage, renderErr := es.templatesContainer.RenderToString("messages_notification", data)
	if renderErr != nil {
		mlog.Error("Unable to render email", mlog.Err(renderErr))
	}

	if nErr := es.SendNotificationMail(user.Email, subject, renderedPage); nErr != nil {
		mlog.Warn("Unable to send batched email notification", mlog.String("email", user.Email), mlog.Err(nErr))
	}
}
