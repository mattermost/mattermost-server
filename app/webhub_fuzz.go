// +build gofuzz

// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
package app

import (
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/testlib"
)

// This is a file used to fuzz test the web_hub code.
// It performs a high-level fuzzing of the web_hub by spawning a hub
// and creating connections to it with a fixed concurrency.
//
// During the fuzz test, we create the server just once, and we send
// the random byte slice through a channel and perform some actions depending
// on the random data.
// The actions are decided in the getActionData function which decides
// which user, team, channel should the message go to and some other stuff too.
//
// Since this requires help of the testing library, we have to duplicate some code
// over here because go-fuzz cannot take code from _test.go files. It won't affect
// the main build because it's behind a build tag.
//
// To run this:
// 1. go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build
// 2. mv app/helper_test.go app/helper.go
// (Also reduce the number of push notification workers to 1 to debug stack traces easily.)
// 3. go-fuzz-build github.com/mattermost/mattermost-server/v5/app
// 4. Generate a corpus dir. It's just a directory with files containing random data
// for go-fuzz to use as an initial seed. Use the generateInitialCorpus function for that.
// 5. go-fuzz -bin=app-fuzz.zip -workdir=./workdir
var mainHelper *testlib.MainHelper

func init() {
	testing.Init()
	var options = testlib.HelperOptions{
		EnableStore:     true,
		EnableResources: true,
	}

	mainHelper = testlib.NewMainHelperWithOptions(&options)
}

func dummyWebsocketHandler() http.HandlerFunc {
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
			panic(err)
		}
	}
}

func registerDummyWebConn(a *App, addr net.Addr, userID string) *WebConn {
	session, appErr := a.CreateSession(&model.Session{
		UserID: userID,
	})
	if appErr != nil {
		panic(appErr)
	}

	d := websocket.Dialer{}
	c, _, err := d.Dial("ws://"+addr.String()+"/ws", nil)
	if err != nil {
		panic(err)
	}

	wc := a.NewWebConn(c, *session, i18n.IdentityTfunc(), "en")
	a.HubRegister(wc)
	go wc.Pump()
	return wc
}

type actionData struct {
	event                string
	createUserID         string
	selectChannelID      string
	selectTeamID         string
	invalidateConnUserID string
	updateConnUserID     string
	attachment           map[string]interface{}
}

func getActionData(data []byte, userIDs, teamIDs, channelIDs []string) *actionData {
	// Some sample events
	events := []string{
		model.WebsocketEventChannelCreated,
		model.WebsocketEventChannelDeleted,
		model.WebsocketEventUserAdded,
		model.WebsocketEventUserUpdated,
		model.WebsocketEventStatusChange,
		model.WebsocketEventHello,
		model.WebsocketAuthenticationChallenge,
		model.WebsocketEventReactionAdded,
		model.WebsocketEventReactionRemoved,
		model.WebsocketEventResponse,
	}
	// We need atleast 10 bytes to get all the data we need
	if len(data) < 10 {
		return nil
	}
	input := &actionData{}
	//	Assign userID, channelID, teamID randomly from respective byte indices
	input.createUserID = userIDs[int(data[0])%len(userIDs)]
	input.selectChannelID = channelIDs[int(data[1])%len(channelIDs)]
	input.selectTeamID = teamIDs[int(data[2])%len(teamIDs)]
	input.invalidateConnUserID = userIDs[int(data[3])%len(userIDs)]
	input.updateConnUserID = userIDs[int(data[4])%len(userIDs)]
	input.event = events[int(data[5])%len(events)]
	data = data[6:]
	input.attachment = make(map[string]interface{})
	for len(data) >= 4 { // 2 bytes key, 2 bytes value
		k := data[:2]
		v := data[2:4]
		input.attachment[string(k)] = v
		data = data[4:]
	}

	return input
}

var startServerOnce sync.Once
var dataChan chan []byte
var resChan = make(chan int, 4) // buffer of 4 to keep reading results.

func Fuzz(data []byte) int {
	// We don't want to close anything down as the fuzzer will keep on running forever.
	startServerOnce.Do(func() {
		t := &testing.T{}
		th := Setup(t).InitBasic()

		s := httptest.NewServer(dummyWebsocketHandler())

		th.App.HubStart()

		u1 := th.CreateUser()
		u2 := th.CreateUser()
		u3 := th.CreateUser()

		t1 := th.CreateTeam()
		t2 := th.CreateTeam()

		ch1 := th.CreateDmChannel(u1)
		ch2 := th.CreateChannel(t1)
		ch3 := th.CreateChannel(t2)

		th.LinkUserToTeam(u1, t1)
		th.LinkUserToTeam(u1, t2)
		th.LinkUserToTeam(u2, t1)
		th.LinkUserToTeam(u2, t2)
		th.LinkUserToTeam(u3, t1)
		th.LinkUserToTeam(u3, t2)

		th.AddUserToChannel(u1, ch2)
		th.AddUserToChannel(u2, ch2)
		th.AddUserToChannel(u3, ch2)
		th.AddUserToChannel(u1, ch3)
		th.AddUserToChannel(u2, ch3)
		th.AddUserToChannel(u3, ch3)

		sema := make(chan struct{}, 4) // A counting semaphore with concurrency of 4.
		dataChan = make(chan []byte)

		go func() {
			for {
				// get data
				data, ok := <-dataChan
				if !ok {
					return
				}
				// acquire semaphore
				sema <- struct{}{}
				go func(data []byte) {
					defer func() {
						// release semaphore
						<-sema
					}()
					var returnCode int
					defer func() {
						resChan <- returnCode
					}()
					// assign data randomly
					// 3 users, 2 teams, 3 channels
					input := getActionData(data,
						[]string{u1.ID, u2.ID, u3.ID, ""},
						[]string{t1.ID, t2.ID, ""},
						[]string{ch1.ID, ch2.ID, ""})
					if input == nil {
						returnCode = 0
						return
					}
					// We get the input from the random data.
					// Now we perform some actions based on that.

					conn := registerDummyWebConn(th.App, s.Listener.Addr(), input.createUserID)
					defer func() {
						conn.Close()
						// A sleep of 2 seconds to allow other connections
						// from the same user to be created, before unregistering them.
						// This hits some additional code paths.
						go func() {
							time.Sleep(2 * time.Second)
							th.App.HubUnregister(conn)
						}()
					}()

					msg := model.NewWebSocketEvent(input.event,
						input.selectTeamID,
						input.selectChannelID,
						input.createUserID, nil)
					for k, v := range input.attachment {
						msg.Add(k, v)
					}
					th.App.Publish(msg)

					th.App.InvalidateWebConnSessionCacheForUser(input.invalidateConnUserID)

					sessions, err := th.App.GetSessions(input.updateConnUserID)
					if err != nil {
						panic(err)
					}
					if len(sessions) > 0 {
						th.App.UpdateWebConnUserActivity(*sessions[0], model.GetMillis())
					}
					returnCode = 1
				}(data)
			}
		}()
	})

	// send data to dataChan
	dataChan <- data

	// get data from res chan
	result := <-resChan
	return result
}

// generateInitialCorpus generates the corpus for go-fuzz.
// Place this function in any main.go file and run it.
// Use the generated directory as the corpus.
func generateInitialCorpus() error {
	err := os.MkdirAll("workdir/corpus", 0755)
	if err != nil {
		return err
	}
	for i := 0; i < 100; i++ {
		data := make([]byte, 25)
		_, err = rand.Read(data)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile("./workdir/corpus"+strconv.Itoa(i), data, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
