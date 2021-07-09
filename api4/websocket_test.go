// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestWebSocketTrailingSlash(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	url := fmt.Sprintf("ws://localhost:%v", th.App.Srv().ListenAddr.Port)
	_, _, err := websocket.DefaultDialer.Dial(url+model.APIURLSuffix+"/websocket/", nil)
	require.NoError(t, err)
}

func TestWebSocketEvent(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	defer WebSocketClient.Close()

	WebSocketClient.Listen()

	resp := <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Status, model.StatusOk, "should have responded OK to authentication challenge")

	omitUser := make(map[string]bool, 1)
	omitUser["somerandomid"] = true
	evt1 := model.NewWebSocketEvent(model.WebsocketEventTyping, "", th.BasicChannel.ID, "", omitUser)
	evt1.Add("user_id", "somerandomid")
	th.App.Publish(evt1)

	time.Sleep(300 * time.Millisecond)

	stop := make(chan bool)
	eventHit := false

	go func() {
		for {
			select {
			case resp := <-WebSocketClient.EventChannel:
				if resp.EventType() == model.WebsocketEventTyping && resp.GetData()["user_id"].(string) == "somerandomid" {
					eventHit = true
				}
			case <-stop:
				return
			}
		}
	}()

	time.Sleep(400 * time.Millisecond)

	stop <- true

	require.True(t, eventHit, "did not receive typing event")

	evt2 := model.NewWebSocketEvent(model.WebsocketEventTyping, "", "somerandomid", "", nil)
	th.App.Publish(evt2)
	time.Sleep(300 * time.Millisecond)

	eventHit = false

	go func() {
		for {
			select {
			case resp := <-WebSocketClient.EventChannel:
				if resp.EventType() == model.WebsocketEventTyping {
					eventHit = true
				}
			case <-stop:
				return
			}
		}
	}()

	time.Sleep(400 * time.Millisecond)

	stop <- true

	require.False(t, eventHit, "got typing event for bad channel id")
}

func TestCreateDirectChannelWithSocket(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	user2 := th.BasicUser2

	users := make([]*model.User, 0)
	users = append(users, user2)

	for i := 0; i < 10; i++ {
		users = append(users, th.CreateUser())
	}

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	defer WebSocketClient.Close()
	WebSocketClient.Listen()

	resp := <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Status, model.StatusOk, "should have responded OK to authentication challenge")

	wsr := <-WebSocketClient.EventChannel
	require.Equal(t, wsr.EventType(), model.WebsocketEventHello, "missing hello")

	stop := make(chan bool)
	count := 0

	go func() {
		for {
			select {
			case wsr := <-WebSocketClient.EventChannel:
				if wsr != nil && wsr.EventType() == model.WebsocketEventDirectAdded {
					count = count + 1
				}

			case <-stop:
				return
			}
		}
	}()

	for _, user := range users {
		time.Sleep(100 * time.Millisecond)
		_, resp := Client.CreateDirectChannel(th.BasicUser.ID, user.ID)
		require.Nil(t, resp.Error, "failed to create DM channel")
	}

	time.Sleep(5000 * time.Millisecond)

	stop <- true

	require.Equal(t, count, len(users), "We didn't get the proper amount of direct_added messages")
}

func TestWebsocketOriginSecurity(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	url := fmt.Sprintf("ws://localhost:%v", th.App.Srv().ListenAddr.Port)

	// Should fail because origin doesn't match
	_, _, err := websocket.DefaultDialer.Dial(url+model.APIURLSuffix+"/websocket", http.Header{
		"Origin": []string{"http://www.evil.com"},
	})

	require.Error(t, err, "Should have errored because Origin does not match host! SECURITY ISSUE!")

	// We are not a browser so we can spoof this just fine
	_, _, err = websocket.DefaultDialer.Dial(url+model.APIURLSuffix+"/websocket", http.Header{
		"Origin": []string{fmt.Sprintf("http://localhost:%v", th.App.Srv().ListenAddr.Port)},
	})
	require.NoError(t, err, err)

	// Should succeed now because open CORS
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "*" })
	_, _, err = websocket.DefaultDialer.Dial(url+model.APIURLSuffix+"/websocket", http.Header{
		"Origin": []string{"http://www.evil.com"},
	})
	require.NoError(t, err, err)

	// Should succeed now because matching CORS
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "http://www.evil.com" })
	_, _, err = websocket.DefaultDialer.Dial(url+model.APIURLSuffix+"/websocket", http.Header{
		"Origin": []string{"http://www.evil.com"},
	})
	require.NoError(t, err, err)

	// Should fail because non-matching CORS
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "http://www.good.com" })
	_, _, err = websocket.DefaultDialer.Dial(url+model.APIURLSuffix+"/websocket", http.Header{
		"Origin": []string{"http://www.evil.com"},
	})
	require.Error(t, err, "Should have errored because Origin contain AllowCorsFrom")

	// Should fail because non-matching CORS
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "http://www.good.com" })
	_, _, err = websocket.DefaultDialer.Dial(url+model.APIURLSuffix+"/websocket", http.Header{
		"Origin": []string{"http://www.good.co"},
	})
	require.Error(t, err, "Should have errored because Origin does not match host! SECURITY ISSUE!")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "" })
}

func TestWebSocketStatuses(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err, err)
	defer WebSocketClient.Close()
	WebSocketClient.Listen()

	resp := <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Status, model.StatusOk, "should have responded OK to authentication challenge")

	team := model.Team{DisplayName: "Name", Name: "z-z-" + model.NewRandomTeamName() + "a", Email: "test@nowhere.com", Type: model.TeamOpen}
	rteam, _ := Client.CreateTeam(&team)

	user := model.User{Email: strings.ToLower(model.NewID()) + "success+test@simulator.amazonses.com", Nickname: "Corey Hulen", Password: "passwd1"}
	ruser := Client.Must(Client.CreateUser(&user)).(*model.User)
	th.LinkUserToTeam(ruser, rteam)
	_, nErr := th.App.Srv().Store.User().VerifyEmail(ruser.ID, ruser.Email)
	require.NoError(t, nErr)

	user2 := model.User{Email: strings.ToLower(model.NewID()) + "success+test@simulator.amazonses.com", Nickname: "Corey Hulen", Password: "passwd1"}
	ruser2 := Client.Must(Client.CreateUser(&user2)).(*model.User)
	th.LinkUserToTeam(ruser2, rteam)
	_, nErr = th.App.Srv().Store.User().VerifyEmail(ruser2.ID, ruser2.Email)
	require.NoError(t, nErr)

	Client.Login(user.Email, user.Password)

	th.LoginBasic2()

	WebSocketClient2, err2 := th.CreateWebSocketClient()
	require.Nil(t, err2, err2)

	time.Sleep(1000 * time.Millisecond)

	WebSocketClient.GetStatuses()
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error, resp.Error)

	require.Equal(t, resp.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")

	allowedValues := [4]string{model.StatusOffline, model.StatusAway, model.StatusOnline, model.StatusDnd}
	for _, status := range resp.Data {
		require.Containsf(t, allowedValues, status, "one of the statuses had an invalid value status=%v", status)
	}

	status, ok := resp.Data[th.BasicUser2.ID]
	require.True(t, ok, "should have had user status")

	require.Equal(t, status, model.StatusOnline, "status should have been online status=%v", status)

	WebSocketClient.GetStatusesByIDs([]string{th.BasicUser2.ID})
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error, resp.Error)

	require.Equal(t, resp.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")

	allowedValues = [4]string{model.StatusOffline, model.StatusAway, model.StatusOnline}
	for _, status := range resp.Data {
		require.Containsf(t, allowedValues, status, "one of the statuses had an invalid value status")
	}

	status, ok = resp.Data[th.BasicUser2.ID]
	require.True(t, ok, "should have had user status")

	require.Equal(t, status, model.StatusOnline, "status should have been online status=%v", status)
	require.Equal(t, len(resp.Data), 1, "only 1 status should be returned")

	WebSocketClient.GetStatusesByIDs([]string{ruser2.ID, "junk"})
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error, resp.Error)
	require.Equal(t, resp.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")
	require.Equal(t, len(resp.Data), 2, "2 statuses should be returned")

	WebSocketClient.GetStatusesByIDs([]string{})
	if resp2 := <-WebSocketClient.ResponseChannel; resp2.Error == nil {
		require.Equal(t, resp2.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")
		require.NotNil(t, resp2.Error, "should have errored - empty user ids")
	}

	WebSocketClient2.Close()

	th.App.SetStatusAwayIfNeeded(th.BasicUser.ID, false)

	awayTimeout := *th.App.Config().TeamSettings.UserStatusAwayTimeout
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.UserStatusAwayTimeout = awayTimeout })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.UserStatusAwayTimeout = 1 })

	time.Sleep(1500 * time.Millisecond)

	th.App.SetStatusAwayIfNeeded(th.BasicUser.ID, false)
	th.App.SetStatusOnline(th.BasicUser.ID, false)

	time.Sleep(1500 * time.Millisecond)

	WebSocketClient.GetStatuses()
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error)

	require.Equal(t, resp.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")
	_, ok = resp.Data[th.BasicUser2.ID]
	require.False(t, ok, "should not have had user status")

	stop := make(chan bool)
	onlineHit := false
	awayHit := false

	go func() {
		for {
			select {
			case resp := <-WebSocketClient.EventChannel:
				if resp.EventType() == model.WebsocketEventStatusChange && resp.GetData()["user_id"].(string) == th.BasicUser.ID {
					status := resp.GetData()["status"].(string)
					if status == model.StatusOnline {
						onlineHit = true
					} else if status == model.StatusAway {
						awayHit = true
					}
				}
			case <-stop:
				return
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)

	stop <- true

	require.True(t, onlineHit, "didn't get online event")
	require.True(t, awayHit, "didn't get away event")

	time.Sleep(500 * time.Millisecond)

	WebSocketClient.Close()
}
