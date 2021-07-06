// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/users"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func dummyWebsocketHandler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		upgrader := &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		conn, err := upgrader.Upgrade(w, req, nil)
		for err == nil {
			_, _, err = conn.ReadMessage()
		}
		if _, ok := err.(*websocket.CloseError); !ok {
			require.NoError(t, err)
		}
	}
}

func registerDummyWebConn(t *testing.T, a *App, addr net.Addr, userID string) *WebConn {
	session, appErr := a.CreateSession(&model.Session{
		UserId: userID,
	})
	require.Nil(t, appErr)

	d := websocket.Dialer{}
	c, _, err := d.Dial("ws://"+addr.String()+"/ws", nil)
	require.NoError(t, err)

	cfg := &WebConnConfig{
		WebSocket: c,
		Session:   *session,
		TFunc:     i18n.IdentityTfunc(),
		Locale:    "en",
	}
	wc := a.NewWebConn(cfg)
	a.HubRegister(wc)
	go wc.Pump()
	return wc
}

func TestHubStopWithMultipleConnections(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	s := httptest.NewServer(dummyWebsocketHandler(t))
	defer s.Close()

	th.App.HubStart()
	wc1 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
	wc2 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
	wc3 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
	defer wc1.Close()
	defer wc2.Close()
	defer wc3.Close()
}

// TestHubStopRaceCondition verifies that attempts to use the hub after it has shutdown does not
// block the caller indefinitely.
func TestHubStopRaceCondition(t *testing.T) {
	th := Setup(t).InitBasic()
	// We do not call TearDown because th.TearDown shuts down the hub again. And hub close is not idempotent.
	// Making it idempotent is not really important to the server because close only happens once.
	// So we just use this quick hack for the test.
	s := httptest.NewServer(dummyWebsocketHandler(t))

	th.App.HubStart()
	wc1 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
	defer wc1.Close()

	hub := th.App.Srv().hubs[0]
	th.App.HubStop()

	done := make(chan bool)
	go func() {
		wc4 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
		wc5 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
		hub.Register(wc4)
		hub.Register(wc5)

		hub.UpdateActivity("userId", "sessionToken", 0)

		for i := 0; i <= broadcastQueueSize; i++ {
			hub.Broadcast(model.NewWebSocketEvent("", "", "", "", nil))
		}

		hub.InvalidateUser("userId")
		hub.Unregister(wc4)
		hub.Unregister(wc5)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		require.FailNow(t, "hub call did not return within 15 seconds after stop")
	}
}

func TestHubSessionRevokeRace(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	sess1 := &model.Session{
		Id:             "id1",
		UserId:         "user1",
		DeviceId:       "",
		Token:          "sesstoken",
		ExpiresAt:      model.GetMillis() + 300000,
		LastActivityAt: 10000,
	}

	mockStore := th.App.Srv().Store.(*mocks.Store)

	mockUserStore := mocks.UserStore{}
	mockUserStore.On("Count", mock.Anything).Return(int64(10), nil)
	mockUserStore.On("GetUnreadCount", mock.AnythingOfType("string")).Return(int64(1), nil)
	mockPostStore := mocks.PostStore{}
	mockPostStore.On("GetMaxPostSize").Return(65535, nil)
	mockSystemStore := mocks.SystemStore{}
	mockSystemStore.On("GetByName", "UpgradedFromTE").Return(&model.System{Name: "UpgradedFromTE", Value: "false"}, nil)
	mockSystemStore.On("GetByName", "InstallationDate").Return(&model.System{Name: "InstallationDate", Value: "10"}, nil)
	mockSystemStore.On("GetByName", "FirstServerRunTimestamp").Return(&model.System{Name: "FirstServerRunTimestamp", Value: "10"}, nil)

	mockSessionStore := mocks.SessionStore{}
	mockSessionStore.On("UpdateLastActivityAt", "id1", mock.Anything).Return(nil)
	mockSessionStore.On("Save", mock.AnythingOfType("*model.Session")).Return(sess1, nil)
	mockSessionStore.On("Get", mock.Anything, "id1").Return(sess1, nil)
	mockSessionStore.On("Remove", "id1").Return(nil)

	mockStatusStore := mocks.StatusStore{}
	mockStatusStore.On("Get", "user1").Return(&model.Status{UserId: "user1", Status: model.STATUS_ONLINE}, nil)
	mockStatusStore.On("UpdateLastActivityAt", "user1", mock.Anything).Return(nil)
	mockStatusStore.On("SaveOrUpdate", mock.AnythingOfType("*model.Status")).Return(nil)

	mockOAuthStore := mocks.OAuthStore{}
	mockStore.On("Session").Return(&mockSessionStore)
	mockStore.On("OAuth").Return(&mockOAuthStore)
	mockStore.On("Status").Return(&mockStatusStore)
	mockStore.On("User").Return(&mockUserStore)
	mockStore.On("Post").Return(&mockPostStore)
	mockStore.On("System").Return(&mockSystemStore)

	userService, err := users.New(users.ServiceConfig{
		UserStore:    &mockUserStore,
		SessionStore: &mockSessionStore,
		OAuthStore:   &mockOAuthStore,
		ConfigFn:     th.App.srv.Config,
		Metrics:      th.App.Metrics(),
		Cluster:      th.App.Cluster(),
		LicenseFn:    th.App.srv.License,
	})
	require.NoError(t, err)
	th.App.srv.userService = userService

	// This needs to be false for the condition to trigger
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ExtendSessionLengthWithActivity = false
	})

	s := httptest.NewServer(dummyWebsocketHandler(t))
	defer s.Close()

	wc1 := registerDummyWebConn(t, th.App, s.Listener.Addr(), "testid")
	hub := th.App.GetHubForUserId(wc1.UserId)

	done := make(chan bool)

	time.Sleep(time.Second)
	// We override the LastActivityAt which happens in NewWebConn.
	// This is needed to call RevokeSessionById which triggers the race.
	th.App.srv.userService.AddSessionToCache(sess1)

	go func() {
		for i := 0; i <= broadcastQueueSize; i++ {
			hub.Broadcast(model.NewWebSocketEvent("", "teamID", "", "", nil))
		}
		close(done)
	}()

	// This call should happen _after_ !wc.IsAuthenticated() and _before_wc.isMemberOfTeam().
	// There's no guarantee this will happen. But that's out best bet to trigger this race.
	wc1.InvalidateCache()

	for i := 0; i < 10; i++ {
		// If broadcast buffer has not emptied,
		// we sleep for a second and check again
		if len(hub.broadcast) > 0 {
			time.Sleep(time.Second)
			continue
		}
	}
	if len(hub.broadcast) > 0 {
		require.Fail(t, "hub is deadlocked")
	}
}

func TestHubConnIndex(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	connIndex := newHubConnectionIndex(1 * time.Second)

	// User1
	wc1 := &WebConn{
		App:    th.App,
		UserId: model.NewId(),
	}

	// User2
	wc2 := &WebConn{
		App:    th.App,
		UserId: model.NewId(),
	}
	wc3 := &WebConn{
		App:    th.App,
		UserId: wc2.UserId,
	}
	wc4 := &WebConn{
		App:    th.App,
		UserId: wc2.UserId,
	}

	connIndex.Add(wc1)
	connIndex.Add(wc2)
	connIndex.Add(wc3)
	connIndex.Add(wc4)

	t.Run("Basic", func(t *testing.T) {
		assert.True(t, connIndex.Has(wc1))
		assert.True(t, connIndex.Has(wc2))

		assert.ElementsMatch(t, connIndex.ForUser(wc2.UserId), []*WebConn{wc2, wc3, wc4})
		assert.ElementsMatch(t, connIndex.ForUser(wc1.UserId), []*WebConn{wc1})
		assert.True(t, connIndex.Has(wc2))
		assert.True(t, connIndex.Has(wc1))
		assert.Len(t, connIndex.All(), 4)
	})

	t.Run("RemoveMiddleUser2", func(t *testing.T) {
		connIndex.Remove(wc3) // Remove from middle from user2

		assert.ElementsMatch(t, connIndex.ForUser(wc2.UserId), []*WebConn{wc2, wc4})
		assert.ElementsMatch(t, connIndex.ForUser(wc1.UserId), []*WebConn{wc1})
		assert.True(t, connIndex.Has(wc2))
		assert.False(t, connIndex.Has(wc3))
		assert.True(t, connIndex.Has(wc4))
		assert.Len(t, connIndex.All(), 3)
	})

	t.Run("RemoveUser1", func(t *testing.T) {
		connIndex.Remove(wc1) // Remove sole connection from user1

		assert.ElementsMatch(t, connIndex.ForUser(wc2.UserId), []*WebConn{wc2, wc4})
		assert.ElementsMatch(t, connIndex.ForUser(wc1.UserId), []*WebConn{})
		assert.Len(t, connIndex.All(), 2)
		assert.False(t, connIndex.Has(wc1))
		assert.True(t, connIndex.Has(wc2))
	})

	t.Run("RemoveEndUser2", func(t *testing.T) {
		connIndex.Remove(wc4) // Remove from end from user2

		assert.ElementsMatch(t, connIndex.ForUser(wc2.UserId), []*WebConn{wc4})
		assert.ElementsMatch(t, connIndex.ForUser(wc1.UserId), []*WebConn{})
		assert.True(t, connIndex.Has(wc2))
		assert.False(t, connIndex.Has(wc3))
		assert.False(t, connIndex.Has(wc4))
		assert.Len(t, connIndex.All(), 1)
	})
}

func TestHubConnIndexInactive(t *testing.T) {
	connIndex := newHubConnectionIndex(2 * time.Second)

	// User1
	wc1 := &WebConn{
		UserId: model.NewId(),
		active: true,
	}
	wc1.SetConnectionID("conn1")

	// User2
	wc2 := &WebConn{
		UserId: model.NewId(),
		active: true,
	}
	wc2.SetConnectionID("conn2")
	wc3 := &WebConn{
		UserId: wc2.UserId,
		active: false,
	}
	wc3.SetConnectionID("conn3")

	connIndex.Add(wc1)
	connIndex.Add(wc2)
	connIndex.Add(wc3)

	assert.Nil(t, connIndex.GetInactiveByConnectionID(wc2.UserId, "conn2"))
	assert.NotNil(t, connIndex.GetInactiveByConnectionID(wc2.UserId, "conn3"))
	assert.Nil(t, connIndex.GetInactiveByConnectionID(wc1.UserId, "conn3"))

	assert.Nil(t, connIndex.RemoveInactiveByConnectionID(wc2.UserId, "conn2"))
	assert.NotNil(t, connIndex.RemoveInactiveByConnectionID(wc2.UserId, "conn3"))
	assert.Nil(t, connIndex.RemoveInactiveByConnectionID(wc1.UserId, "conn3"))
	assert.False(t, connIndex.Has(wc3))
	assert.Len(t, connIndex.ForUser(wc2.UserId), 1)

	wc3.lastUserActivityAt = model.GetMillis()
	connIndex.Add(wc3)
	connIndex.RemoveInactiveConnections()
	assert.True(t, connIndex.Has(wc3))
	assert.Len(t, connIndex.ForUser(wc2.UserId), 2)
	assert.Len(t, connIndex.All(), 3)

	wc3.lastUserActivityAt = model.GetMillis() - (time.Minute).Milliseconds()
	connIndex.RemoveInactiveConnections()
	assert.False(t, connIndex.Has(wc3))
	assert.Len(t, connIndex.ForUser(wc2.UserId), 1)
	assert.Len(t, connIndex.All(), 2)
}

func TestHubIsRegistered(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	s := httptest.NewServer(dummyWebsocketHandler(t))
	defer s.Close()

	th.App.HubStart()
	wc1 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
	wc2 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
	wc3 := registerDummyWebConn(t, th.App, s.Listener.Addr(), th.BasicUser.Id)
	defer wc1.Close()
	defer wc2.Close()
	defer wc3.Close()

	session1 := wc1.session.Load().(*model.Session)

	assert.True(t, th.App.SessionIsRegistered(*session1))
	assert.True(t, th.App.SessionIsRegistered(*wc2.session.Load().(*model.Session)))
	assert.True(t, th.App.SessionIsRegistered(*wc3.session.Load().(*model.Session)))

	session4, appErr := th.App.CreateSession(&model.Session{
		UserId: th.BasicUser2.Id,
	})
	require.Nil(t, appErr)
	assert.False(t, th.App.SessionIsRegistered(*session4))
}

// Always run this with -benchtime=0.1s
// See: https://github.com/golang/go/issues/27217.
func BenchmarkHubConnIndex(b *testing.B) {
	th := Setup(b).InitBasic()
	defer th.TearDown()
	connIndex := newHubConnectionIndex(1 * time.Second)

	// User1
	wc1 := &WebConn{
		App:    th.App,
		UserId: model.NewId(),
	}

	// User2
	wc2 := &WebConn{
		App:    th.App,
		UserId: model.NewId(),
	}
	b.ResetTimer()
	b.Run("Add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			connIndex.Add(wc1)
			connIndex.Add(wc2)

			b.StopTimer()
			connIndex.Remove(wc1)
			connIndex.Remove(wc2)
			b.StartTimer()
		}
	})

	b.Run("Remove", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			connIndex.Add(wc1)
			connIndex.Add(wc2)
			b.StartTimer()

			connIndex.Remove(wc1)
			connIndex.Remove(wc2)
		}
	})
}

var hubSink *Hub

func BenchmarkGetHubForUserId(b *testing.B) {
	th := Setup(b).InitBasic()
	defer th.TearDown()

	th.App.HubStart()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hubSink = th.Server.GetHubForUserId(th.BasicUser.Id)
	}
}
