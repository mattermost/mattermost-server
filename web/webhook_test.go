// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestIncomingWebhook(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	if !*th.App.Config().ServiceSettings.EnableIncomingWebhooks {
		_, err := http.Post(ApiClient.Url+"/hooks/123", "", strings.NewReader("123"))
		assert.Error(t, err, "should have errored - webhooks turned off")
		return
	}

	hook, err := th.App.CreateIncomingWebhookForChannel(th.BasicUser.ID, th.BasicChannel, &model.IncomingWebhook{ChannelID: th.BasicChannel.ID})
	require.Nil(t, err)

	url := ApiClient.Url + "/hooks/" + hook.ID

	tooLongText := ""
	for i := 0; i < 8200; i++ {
		tooLongText += "a"
	}

	t.Run("WebhookBasics", func(t *testing.T) {
		payload := "payload={\"text\": \"test text\"}"
		resp, err := http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(payload))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		payload = "payload={\"text\": \"\"}"
		resp, err = http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(payload))
		require.NoError(t, err)
		assert.NotEqual(t, http.StatusOK, resp.StatusCode, "should have errored - no text post")

		payload = "payload={\"text\": \"test text\", \"channel\": \"junk\"}"
		resp, err = http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(payload))
		require.NoError(t, err)
		assert.NotEqual(t, http.StatusOK, resp.StatusCode, "should have errored - bad channel")

		payload = "payload={\"text\": \"test text\"}"
		resp, err = http.Post(ApiClient.Url+"/hooks/abc123", "application/x-www-form-urlencoded", strings.NewReader(payload))
		require.NoError(t, err)
		assert.NotEqual(t, http.StatusOK, resp.StatusCode, "should have errored - bad hook")

		resp, err = http.Post(url, "application/json", strings.NewReader("{\"text\":\"this is a test\"}"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		text := `this is a \"test\"
	that contains a newline and a tab`
		resp, err = http.Post(url, "application/json", strings.NewReader("{\"text\":\""+text+"\"}"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/json", strings.NewReader(fmt.Sprintf("{\"text\":\"this is a test\", \"channel\":\"%s\"}", th.BasicChannel.Name)))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/json", strings.NewReader(fmt.Sprintf("{\"text\":\"this is a test\", \"channel\":\"#%s\"}", th.BasicChannel.Name)))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/json", strings.NewReader(fmt.Sprintf("{\"text\":\"this is a test\", \"channel\":\"@%s\"}", th.BasicUser.Username)))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/x-www-form-urlencoded", strings.NewReader("payload={\"text\":\"this is a test\"}"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/x-www-form-urlencoded", strings.NewReader("payload={\"text\":\""+text+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "AppLicaTion/x-www-Form-urlencoded", strings.NewReader("payload={\"text\":\""+text+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/x-www-form-urlencoded;charset=utf-8", strings.NewReader("payload={\"text\":\""+text+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/x-www-form-urlencoded; charset=utf-8", strings.NewReader("payload={\"text\":\""+text+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/x-www-form-urlencoded wrongtext", strings.NewReader("payload={\"text\":\""+text+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		resp, err = http.Post(url, "application/json", strings.NewReader("{\"text\":\""+tooLongText+"\"}"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "application/x-www-form-urlencoded", strings.NewReader("{\"text\":\""+tooLongText+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		resp, err = http.Post(url, "application/json", strings.NewReader("payload={\"text\":\""+text+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		payloadMultiPart := "------WebKitFormBoundary7MA4YWxkTrZu0gW\r\nContent-Disposition: form-data; name=\"username\"\r\n\r\nwebhook-bot\r\n------WebKitFormBoundary7MA4YWxkTrZu0gW\r\nContent-Disposition: form-data; name=\"text\"\r\n\r\nthis is a test :tada:\r\n------WebKitFormBoundary7MA4YWxkTrZu0gW--"
		resp, err = http.Post(ApiClient.Url+"/hooks/"+hook.ID, "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW", strings.NewReader(payloadMultiPart))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = http.Post(url, "mimetype/wrong", strings.NewReader("payload={\"text\":\""+text+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		resp, err = http.Post(url, "", strings.NewReader("{\"text\":\""+text+"\"}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("WebhookExperimentalReadOnly", func(t *testing.T) {
		th.App.Srv().SetLicense(model.NewTestLicense())
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.ExperimentalTownSquareIsReadOnly = true })

		// Read only default channel should fail.
		resp, err := http.Post(url, "application/json", strings.NewReader(fmt.Sprintf("{\"text\":\"this is a test\", \"channel\":\"%s\"}", model.DefaultChannelName)))
		require.NoError(t, err)
		assert.True(t, resp.StatusCode != http.StatusOK)

		// None-default channel should still work.
		resp, err = http.Post(url, "application/json", strings.NewReader(fmt.Sprintf("{\"text\":\"this is a test\", \"channel\":\"%s\"}", th.BasicChannel.Name)))
		require.NoError(t, err)
		assert.True(t, resp.StatusCode == http.StatusOK)

		// System-Admin Owned Hook
		adminHook, appErr := th.App.CreateIncomingWebhookForChannel(th.SystemAdminUser.ID, th.BasicChannel, &model.IncomingWebhook{ChannelID: th.BasicChannel.ID})
		require.Nil(t, appErr)
		adminUrl := ApiClient.Url + "/hooks/" + adminHook.ID

		resp, err = http.Post(adminUrl, "application/json", strings.NewReader(fmt.Sprintf("{\"text\":\"this is a test\", \"channel\":\"%s\"}", model.DefaultChannelName)))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.ExperimentalTownSquareIsReadOnly = false })
	})

	t.Run("WebhookAttachments", func(t *testing.T) {
		attachmentPayload := `{
	       "text": "this is a test",
	       "attachments": [
	           {
	               "fallback": "Required plain-text summary of the attachment.",

	               "color": "#36a64f",

	               "pretext": "Optional text that appears above the attachment block",

	               "author_name": "Bobby Tables",
	               "author_link": "http://flickr.com/bobby/",
	               "author_icon": "http://flickr.com/icons/bobby.jpg",

	               "title": "Slack API Documentation",
	               "title_link": "https://api.slack.com/",

	               "text": "Optional text that appears within the attachment",

	               "fields": [
	                   {
	                       "title": "Priority",
	                       "value": "High",
	                       "short": false
	                   }
	               ],

	               "image_url": "http://my-website.com/path/to/image.jpg",
	               "thumb_url": "http://example.com/path/to/thumb.png"
	           }
	       ]
	   }`

		resp, err := http.Post(url, "application/json", strings.NewReader(attachmentPayload))
		require.NoError(t, err)
		assert.True(t, resp.StatusCode == http.StatusOK)

		attachmentPayload = `{
	       "text": "this is a test",
	       "attachments": [
	           {
	               "fallback": "Required plain-text summary of the attachment.",

	               "color": "#36a64f",

	               "pretext": "Optional text that appears above the attachment block",

	               "author_name": "Bobby Tables",
	               "author_link": "http://flickr.com/bobby/",
	               "author_icon": "http://flickr.com/icons/bobby.jpg",

	               "title": "Slack API Documentation",
	               "title_link": "https://api.slack.com/",

	               "text": "` + tooLongText + `",

	               "fields": [
	                   {
	                       "title": "Priority",
	                       "value": "High",
	                       "short": false
	                   }
	               ],

	               "image_url": "http://my-website.com/path/to/image.jpg",
	               "thumb_url": "http://example.com/path/to/thumb.png"
	           }
	       ]
	   }`

		resp, err = http.Post(url, "application/json", strings.NewReader(attachmentPayload))
		require.NoError(t, err)
		assert.True(t, resp.StatusCode == http.StatusOK)
	})

	t.Run("ChannelLockedWebhook", func(t *testing.T) {
		channel, err := th.App.CreateChannel(th.Context, &model.Channel{TeamID: th.BasicTeam.ID, Name: model.NewID(), DisplayName: model.NewID(), Type: model.ChannelTypeOpen, CreatorID: th.BasicUser.ID}, true)
		require.Nil(t, err)

		hook, err := th.App.CreateIncomingWebhookForChannel(th.BasicUser.ID, th.BasicChannel, &model.IncomingWebhook{ChannelID: th.BasicChannel.ID, ChannelLocked: true})
		require.Nil(t, err)
		require.NotNil(t, hook)

		apiHookUrl := ApiClient.Url + "/hooks/" + hook.ID

		payload := "payload={\"text\": \"test text\"}"
		resp, err2 := http.Post(apiHookUrl, "application/x-www-form-urlencoded", strings.NewReader(payload))
		require.NoError(t, err2)
		assert.True(t, resp.StatusCode == http.StatusOK)

		resp, err2 = http.Post(apiHookUrl, "application/json", strings.NewReader(fmt.Sprintf("{\"text\":\"this is a test\", \"channel\":\"%s\"}", th.BasicChannel.Name)))
		require.NoError(t, err2)
		assert.True(t, resp.StatusCode == http.StatusOK)

		resp, err2 = http.Post(apiHookUrl, "application/json", strings.NewReader(fmt.Sprintf("{\"text\":\"this is a test\", \"channel\":\"%s\"}", channel.Name)))
		require.NoError(t, err2)
		assert.True(t, resp.StatusCode == http.StatusForbidden)
	})

	t.Run("DisableWebhooks", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = false })
		resp, err := http.Post(url, "application/json", strings.NewReader("{\"text\":\"this is a test\"}"))
		require.NoError(t, err)
		assert.True(t, resp.StatusCode == http.StatusNotImplemented)
	})
}

func TestCommandWebhooks(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	cmd, appErr := th.App.CreateCommand(&model.Command{
		CreatorID: th.BasicUser.ID,
		TeamID:    th.BasicTeam.ID,
		URL:       "http://nowhere.com",
		Method:    model.CommandMethodPost,
		Trigger:   "delayed"})
	require.Nil(t, appErr)

	args := &model.CommandArgs{
		TeamID:    th.BasicTeam.ID,
		UserID:    th.BasicUser.ID,
		ChannelID: th.BasicChannel.ID,
	}

	hook, appErr := th.App.CreateCommandWebhook(cmd.ID, args)
	require.Nil(t, appErr)

	resp, err := http.Post(ApiClient.Url+"/hooks/commands/123123123123", "application/json", bytes.NewBufferString(`{"text":"this is a test"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "expected not-found for non-existent hook")

	resp, err = http.Post(ApiClient.Url+"/hooks/commands/"+hook.ID, "application/json", bytes.NewBufferString(`{"text":"invalid`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	for i := 0; i < 5; i++ {
		response, err2 := http.Post(ApiClient.Url+"/hooks/commands/"+hook.ID, "application/json", bytes.NewBufferString(`{"text":"this is a test"}`))
		require.NoError(t, err2)
		require.Equal(t, http.StatusOK, response.StatusCode)
	}

	resp, _ = http.Post(ApiClient.Url+"/hooks/commands/"+hook.ID, "application/json", bytes.NewBufferString(`{"text":"this is a test"}`))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
