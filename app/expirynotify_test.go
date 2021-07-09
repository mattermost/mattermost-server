// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestNotifySessionsExpired(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	handler := &testPushNotificationHandler{t: t}
	pushServer := httptest.NewServer(
		http.HandlerFunc(handler.handleReq),
	)
	defer pushServer.Close()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.EmailSettings.PushNotificationServer = pushServer.URL
	})

	t.Run("push notifications disabled", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.EmailSettings.SendPushNotifications = false
		})

		err := th.App.NotifySessionsExpired()
		// no error, but also no requests sent
		require.Nil(t, err)
		require.Equal(t, 0, handler.numReqs())
	})

	t.Run("two sessions expired", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.EmailSettings.SendPushNotifications = true
		})

		data := []struct {
			deviceID  string
			expiresAt int64
			notified  bool
		}{
			{deviceID: "android:11111", expiresAt: model.GetMillis() + 100000, notified: false},
			{deviceID: "android:22222", expiresAt: model.GetMillis() - 1000, notified: false},
			{deviceID: "android:33333", expiresAt: model.GetMillis() - 2000, notified: false},
			{deviceID: "android:44444", expiresAt: model.GetMillis() - 3000, notified: true},
		}

		for _, d := range data {
			_, err := th.App.CreateSession(&model.Session{
				UserID:        th.BasicUser.ID,
				DeviceID:      d.deviceID,
				ExpiresAt:     d.expiresAt,
				ExpiredNotify: d.notified,
			})
			require.Nil(t, err)
		}

		err := th.App.NotifySessionsExpired()

		require.Nil(t, err)
		require.Equal(t, 2, handler.numReqs())

		expected := []string{"22222", "33333"}
		require.Equal(t, model.PushTypeSession, handler.notifications()[0].Type)
		require.Contains(t, expected, handler.notifications()[0].DeviceID)
		require.Contains(t, handler.notifications()[0].Message, "Session Expired")

		require.Equal(t, model.PushTypeSession, handler.notifications()[1].Type)
		require.Contains(t, expected, handler.notifications()[1].DeviceID)
		require.Contains(t, handler.notifications()[1].Message, "Session Expired")
	})
}
