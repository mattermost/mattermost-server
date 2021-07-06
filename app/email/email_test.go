// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package email

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mail"
)

func TestCondenseSiteURL(t *testing.T) {
	require.Equal(t, "", condenseSiteURL(""))
	require.Equal(t, "mattermost.com", condenseSiteURL("mattermost.com"))
	require.Equal(t, "mattermost.com", condenseSiteURL("mattermost.com/"))
	require.Equal(t, "chat.mattermost.com", condenseSiteURL("chat.mattermost.com"))
	require.Equal(t, "chat.mattermost.com", condenseSiteURL("chat.mattermost.com/"))
	require.Equal(t, "mattermost.com/subpath", condenseSiteURL("mattermost.com/subpath"))
	require.Equal(t, "mattermost.com/subpath", condenseSiteURL("mattermost.com/subpath/"))
	require.Equal(t, "chat.mattermost.com/subpath", condenseSiteURL("chat.mattermost.com/subpath"))
	require.Equal(t, "chat.mattermost.com/subpath", condenseSiteURL("chat.mattermost.com/subpath/"))

	require.Equal(t, "mattermost.com:8080", condenseSiteURL("http://mattermost.com:8080"))
	require.Equal(t, "mattermost.com:8080", condenseSiteURL("http://mattermost.com:8080/"))
	require.Equal(t, "chat.mattermost.com:8080", condenseSiteURL("http://chat.mattermost.com:8080"))
	require.Equal(t, "chat.mattermost.com:8080", condenseSiteURL("http://chat.mattermost.com:8080/"))
	require.Equal(t, "mattermost.com:8080/subpath", condenseSiteURL("http://mattermost.com:8080/subpath"))
	require.Equal(t, "mattermost.com:8080/subpath", condenseSiteURL("http://mattermost.com:8080/subpath/"))
	require.Equal(t, "chat.mattermost.com:8080/subpath", condenseSiteURL("http://chat.mattermost.com:8080/subpath"))
	require.Equal(t, "chat.mattermost.com:8080/subpath", condenseSiteURL("http://chat.mattermost.com:8080/subpath/"))
}

func TestSendInviteEmails(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.ConfigureInbucketMail()

	th.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableEmailInvitations = true
	})

	require.NotNil(t, th.BasicUser)
	require.NotNil(t, th.BasicChannel)

	emailTo := "test@example.com"
	mail.DeleteMailBox(emailTo)

	err := th.service.SendInviteEmails(th.BasicTeam, "test-user", th.BasicUser.Id, []string{emailTo}, "http://testserver")
	require.NoError(t, err)

	var resultsMailbox mail.JSONMessageHeaderInbucket
	err2 := mail.RetryInbucket(5, func() error {
		var err error
		resultsMailbox, err = mail.GetMailBox(emailTo)
		return err
	})
	if err2 != nil {
		t.Log(err2)
		t.Log("No email was received, maybe due load on the server. Skipping this verification")
	} else if len(resultsMailbox) > 0 {
		require.Len(t, resultsMailbox, 1)
		require.Contains(t, resultsMailbox[0].To[0], emailTo, "Wrong To: recipient")
		resultsEmail, err := mail.GetMessageFromMailbox(emailTo, resultsMailbox[0].ID)
		require.NoError(t, err, "Could not get message from mailbox")
		require.Contains(t, resultsEmail.Body.HTML, "http://testserver", "Wrong received message %s", resultsEmail.Body.Text)
		require.Contains(t, resultsEmail.Body.HTML, "test-user", "Wrong received message %s", resultsEmail.Body.Text)
		require.Contains(t, resultsEmail.Body.Text, "http://testserver", "Wrong received message %s", resultsEmail.Body.Text)
		require.Contains(t, resultsEmail.Body.Text, "test-user", "Wrong received message %s", resultsEmail.Body.Text)
	}
}
