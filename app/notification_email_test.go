// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/timezones"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestGetDirectMessageNotificationEmailSubject(t *testing.T) {
	expectedPrefix := "[http://localhost:8065] New Direct Message from @sender on"
	user := &model.User{}
	post := &model.Post{
		CreateAt: 1501804801000,
	}
	translateFunc := i18n.GetUserTranslations("en")
	subject := getDirectMessageNotificationEmailSubject(user, post, translateFunc, "http://localhost:8065", "@sender", true)
	require.Regexp(t, regexp.MustCompile("^"+regexp.QuoteMeta(expectedPrefix)), subject, fmt.Sprintf("Expected subject line prefix '%s', got %s", expectedPrefix, subject))
}

func TestGetGroupMessageNotificationEmailSubjectFull(t *testing.T) {
	expectedPrefix := "[http://localhost:8065] New Group Message in sender on"
	user := &model.User{}
	post := &model.Post{
		CreateAt: 1501804801000,
	}
	translateFunc := i18n.GetUserTranslations("en")
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	subject := getGroupMessageNotificationEmailSubject(user, post, translateFunc, "http://localhost:8065", "sender", emailNotificationContentsType, true)
	require.Regexp(t, regexp.MustCompile("^"+regexp.QuoteMeta(expectedPrefix)), subject, fmt.Sprintf("Expected subject line prefix '%s', got %s", expectedPrefix, subject))
}

func TestGetGroupMessageNotificationEmailSubjectGeneric(t *testing.T) {
	expectedPrefix := "[http://localhost:8065] New Group Message on"
	user := &model.User{}
	post := &model.Post{
		CreateAt: 1501804801000,
	}
	translateFunc := i18n.GetUserTranslations("en")
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_GENERIC
	subject := getGroupMessageNotificationEmailSubject(user, post, translateFunc, "http://localhost:8065", "sender", emailNotificationContentsType, true)
	require.Regexp(t, regexp.MustCompile("^"+regexp.QuoteMeta(expectedPrefix)), subject, fmt.Sprintf("Expected subject line prefix '%s', got %s", expectedPrefix, subject))
}

func TestGetNotificationEmailSubject(t *testing.T) {
	expectedPrefix := "[http://localhost:8065] Notification in team on"
	user := &model.User{}
	post := &model.Post{
		CreateAt: 1501804801000,
	}
	translateFunc := i18n.GetUserTranslations("en")
	subject := getNotificationEmailSubject(user, post, translateFunc, "http://localhost:8065", "team", true)
	require.Regexp(t, regexp.MustCompile("^"+regexp.QuoteMeta(expectedPrefix)), subject, fmt.Sprintf("Expected subject line prefix '%s', got %s", expectedPrefix, subject))
}

func TestGetNotificationEmailBodyFullNotificationPublicChannel(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "mentioned you in a message", fmt.Sprintf("Expected email text 'mentioned you in a message. Got %s", body))
	require.Contains(t, body, post.Message, fmt.Sprintf("Expected email text '%s'. Got %s", post.Message, body))
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestGetNotificationEmailBodyFullNotificationGroupChannel(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_GROUP,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "sent you a new message", fmt.Sprintf("Expected email text 'sent you a new message. Got "+body))
	require.Contains(t, body, post.Message, fmt.Sprintf("Expected email text '%s'. Got %s", post.Message, body))
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestGetNotificationEmailBodyFullNotificationPrivateChannel(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_PRIVATE,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "mentioned you in a message", fmt.Sprintf("Expected email text 'mentioned you in a message. Got "+body))
	require.Contains(t, body, post.Message, fmt.Sprintf("Expected email text '%s'. Got %s", post.Message, body))
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestGetNotificationEmailBodyFullNotificationDirectChannel(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_DIRECT,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "sent you a new message", fmt.Sprintf("Expected email text 'sent you a new message. Got "+body))
	require.Contains(t, body, post.Message, fmt.Sprintf("Expected email text '%s'. Got %s", post.Message, body))
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestGetNotificationEmailBodyFullNotificationLocaleTimeWithTimezone(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{
		Timezone: timezones.DefaultUserTimezone(),
	}
	recipient.Timezone["automaticTimezone"] = "America/New_York"
	post := &model.Post{
		CreateAt: 1524663790000,
		Message:  "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_DIRECT,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, false, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	r, _ := regexp.Compile("E([S|D]+)T")
	zone := r.FindString(body)
	require.Contains(t, body, "9:43 AM "+zone, fmt.Sprintf("Expected email text '9:43 AM %s'. Got %s", zone, body))
}

func TestGetNotificationEmailBodyFullNotificationLocaleTimeNoTimezone(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{
		Timezone: timezones.DefaultUserTimezone(),
	}
	post := &model.Post{
		CreateAt: 1524681000000,
		Message:  "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_DIRECT,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	tm := time.Unix(post.CreateAt/1000, 0)
	zone, _ := tm.Zone()

	formattedTime := formattedPostTime{
		Hour:     fmt.Sprintf("%02d", tm.Hour()),
		Minute:   fmt.Sprintf("%02d", tm.Minute()),
		TimeZone: zone,
	}

	tmp, err := template.New("foo").Parse(`{{.}}`)
	require.NoError(t, err)
	var text bytes.Buffer
	err = tmp.Execute(&text, fmt.Sprintf("%s:%s %s", formattedTime.Hour, formattedTime.Minute, formattedTime.TimeZone))
	require.NoError(t, err)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	postTimeLine := text.String()
	require.Contains(t, body, postTimeLine, fmt.Sprintf("Expected email text '%s'. Got %s", postTimeLine, body))
}

func TestGetNotificationEmailBodyFullNotificationLocaleTime12Hour(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{
		Timezone: timezones.DefaultUserTimezone(),
	}
	recipient.Timezone["automaticTimezone"] = "America/New_York"
	post := &model.Post{
		CreateAt: 1524681000000, // 1524681000 // 1524681000000
		Message:  "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_DIRECT,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, false, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "2:30 PM", fmt.Sprintf("Expected email text '2:30 PM'. Got %s", body))
}

func TestGetNotificationEmailBodyFullNotificationLocaleTime24Hour(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{
		Timezone: timezones.DefaultUserTimezone(),
	}
	recipient.Timezone["automaticTimezone"] = "America/New_York"
	post := &model.Post{
		CreateAt: 1524681000000,
		Message:  "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_DIRECT,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "14:30", fmt.Sprintf("Expected email text '14:30'. Got %s", body))
}

// from here
func TestGetNotificationEmailBodyGenericNotificationPublicChannel(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_GENERIC
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "mentioned you in a message", fmt.Sprintf("Expected email text 'mentioned you in a message. Got %s", body))
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestGetNotificationEmailBodyGenericNotificationGroupChannel(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_GROUP,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_GENERIC
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "sent you a new message", fmt.Sprintf("Expected email text 'sent you a new message. Got "+body))
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestGetNotificationEmailBodyGenericNotificationPrivateChannel(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_PRIVATE,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_GENERIC
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "mentioned you in a message", fmt.Sprintf("Expected email text 'mentioned you in a message. Got %s", body))
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestGetNotificationEmailBodyGenericNotificationDirectChannel(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_DIRECT,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_GENERIC
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, "sent you a new message", fmt.Sprintf("Expected email text 'sent you a new message. Got "+body))
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestGetNotificationEmailEscapingChars(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	ch := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	channelName := "ChannelName"
	recipient := &model.User{}
	message := "<b>Bold Test</b>"
	post := &model.Post{
		Message: message,
	}

	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, ch,
		channelName, senderName, teamName, teamURL,
		emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)

	assert.NotContains(t, body, message)
}

func TestGetNotificationEmailBodyPublicChannelMention(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	ch := &model.Channel{
		Name:        "channelname",
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	id := model.NewId()
	recipient := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}
	post := &model.Post{
		Message: "This is the message ~" + ch.Name,
	}

	senderName := "user1"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Id: "test", Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	channelStoreMock := mocks.ChannelStore{}
	channelStoreMock.On("GetByNames", "test", []string{ch.Name}, true).Return([]*model.Channel{ch}, nil)
	storeMock.On("Channel").Return(&channelStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, ch,
		ch.Name, senderName, teamName, teamURL,
		emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	channelURL := teamURL + "/channels/" + ch.Name
	mention := "~" + ch.Name
	assert.Contains(t, body, "<a href='"+channelURL+"'>"+mention+"</a>")
}

func TestGetNotificationEmailBodyMultiPublicChannelMention(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	ch := &model.Channel{
		Id:          model.NewId(),
		Name:        "channelnameone",
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	mention := "~" + ch.Name

	ch2 := &model.Channel{
		Id:          model.NewId(),
		Name:        "channelnametwo",
		DisplayName: "ChannelName2",
		Type:        model.CHANNEL_OPEN,
	}
	mention2 := "~" + ch2.Name

	ch3 := &model.Channel{
		Id:          model.NewId(),
		Name:        "channelnamethree",
		DisplayName: "ChannelName3",
		Type:        model.CHANNEL_OPEN,
	}
	mention3 := "~" + ch3.Name

	message := fmt.Sprintf("This is the message Channel1: %s; Channel2: %s;"+
		" Channel3: %s", mention, mention2, mention3)
	id := model.NewId()
	recipient := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}
	post := &model.Post{
		Message: message,
	}

	senderName := "user1"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Id: "test", Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	channelStoreMock := mocks.ChannelStore{}
	channelStoreMock.On("GetByNames", "test", []string{ch.Name, ch2.Name, ch3.Name}, true).Return([]*model.Channel{ch, ch2, ch3}, nil)
	storeMock.On("Channel").Return(&channelStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, ch,
		ch.Name, senderName, teamName, teamURL,
		emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	channelURL := teamURL + "/channels/" + ch.Name
	channelURL2 := teamURL + "/channels/" + ch2.Name
	channelURL3 := teamURL + "/channels/" + ch3.Name
	expMessage := fmt.Sprintf("This is the message Channel1: <a href='%s'>%s</a>;"+
		" Channel2: <a href='%s'>%s</a>; Channel3: <a href='%s'>%s</a>",
		channelURL, mention, channelURL2, mention2, channelURL3, mention3)
	assert.Contains(t, body, expMessage)
}

func TestGetNotificationEmailBodyPrivateChannelMention(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	ch := &model.Channel{
		Name:        "channelname",
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_PRIVATE,
	}
	id := model.NewId()
	recipient := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}
	post := &model.Post{
		Message: "This is the message ~" + ch.Name,
	}

	senderName := "user1"
	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Id: "test", Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	channelStoreMock := mocks.ChannelStore{}
	channelStoreMock.On("GetByNames", "test", []string{ch.Name}, true).Return([]*model.Channel{ch}, nil)
	storeMock.On("Channel").Return(&channelStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, ch,
		ch.Name, senderName, teamName, teamURL,
		emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	channelURL := teamURL + "/channels/" + ch.Name
	mention := "~" + ch.Name
	assert.NotContains(t, body, "<a href='"+channelURL+"'>"+mention+"</a>")
}

func TestGenerateHyperlinkForChannelsPublic(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	ch := &model.Channel{
		Name:        "channelname",
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	message := "This is the message "
	mention := "~" + ch.Name

	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Id: "test", Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	channelStoreMock := mocks.ChannelStore{}
	channelStoreMock.On("GetByNames", "test", []string{ch.Name}, true).Return([]*model.Channel{ch}, nil)
	storeMock.On("Channel").Return(&channelStoreMock)

	outMessage, err := th.App.generateHyperlinkForChannels(message+mention, teamName, teamURL)
	require.Nil(t, err)
	channelURL := teamURL + "/channels/" + ch.Name
	assert.Equal(t, message+"<a href='"+channelURL+"'>"+mention+"</a>", outMessage)
}

func TestGenerateHyperlinkForChannelsMultiPublic(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	// TODO: Fix the case where the first channel name contains the other channel names (for example here channelnameone)"
	ch := &model.Channel{
		Id:          model.NewId(),
		Name:        "channelnameone",
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	mention := "~" + ch.Name

	ch2 := &model.Channel{
		Id:          model.NewId(),
		Name:        "channelnametwo",
		DisplayName: "ChannelName2",
		Type:        model.CHANNEL_OPEN,
	}
	mention2 := "~" + ch2.Name

	ch3 := &model.Channel{
		Id:          model.NewId(),
		Name:        "channelnamethree",
		DisplayName: "ChannelName3",
		Type:        model.CHANNEL_OPEN,
	}
	mention3 := "~" + ch3.Name

	message := fmt.Sprintf("This is the message Channel1: %s; Channel2: %s;"+
		" Channel3: %s", mention, mention2, mention3)

	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Id: "test", Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	channelStoreMock := mocks.ChannelStore{}
	channelStoreMock.On("GetByNames", "test", []string{ch.Name, ch2.Name, ch3.Name}, true).Return([]*model.Channel{ch, ch2, ch3}, nil)
	storeMock.On("Channel").Return(&channelStoreMock)

	outMessage, err := th.App.generateHyperlinkForChannels(message, teamName, teamURL)
	require.Nil(t, err)
	channelURL := teamURL + "/channels/" + ch.Name
	channelURL2 := teamURL + "/channels/" + ch2.Name
	channelURL3 := teamURL + "/channels/" + ch3.Name
	expMessage := fmt.Sprintf("This is the message Channel1: <a href='%s'>%s</a>;"+
		" Channel2: <a href='%s'>%s</a>; Channel3: <a href='%s'>%s</a>",
		channelURL, mention, channelURL2, mention2, channelURL3, mention3)
	assert.Equal(t, expMessage, outMessage)
}

func TestGenerateHyperlinkForChannelsPrivate(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	ch := &model.Channel{
		Name:        "channelname",
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_PRIVATE,
	}
	message := "This is the message ~" + ch.Name

	teamName := "testteam"
	teamURL := "http://localhost:8065/testteam"

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Id: "test", Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	channelStoreMock := mocks.ChannelStore{}
	channelStoreMock.On("GetByNames", "test", []string{ch.Name}, true).Return([]*model.Channel{ch}, nil)
	storeMock.On("Channel").Return(&channelStoreMock)

	outMessage, err := th.App.generateHyperlinkForChannels(message, teamName, teamURL)
	require.Nil(t, err)
	assert.Equal(t, message, outMessage)
}

func TestLandingLink(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/landing#/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, teamURL, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}

func TestLandingLinkPermalink(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	recipient := &model.User{}
	post := &model.Post{
		Id:      "Test_id",
		Message: "This is the message",
	}
	channel := &model.Channel{
		DisplayName: "ChannelName",
		Type:        model.CHANNEL_OPEN,
	}
	channelName := "ChannelName"
	senderName := "sender"
	teamName := "testteam"
	teamURL := "http://localhost:8065/landing#/testteam"
	emailNotificationContentsType := model.EMAIL_NOTIFICATION_CONTENTS_FULL
	translateFunc := i18n.GetUserTranslations("en")

	storeMock := th.App.Srv().Store.(*mocks.Store)
	teamStoreMock := mocks.TeamStore{}
	teamStoreMock.On("GetByName", "testteam").Return(&model.Team{Name: "testteam"}, nil)
	storeMock.On("Team").Return(&teamStoreMock)

	body, err := th.App.getNotificationEmailBody(recipient, post, channel, channelName, senderName, teamName, teamURL, emailNotificationContentsType, true, translateFunc, "user-avatar.png")
	require.NoError(t, err)
	require.Contains(t, body, teamURL+"/pl/"+post.Id, fmt.Sprintf("Expected email text '%s'. Got %s", teamURL, body))
}
