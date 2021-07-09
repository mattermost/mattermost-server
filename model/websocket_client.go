// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	SocketMaxMessageSizeKb   = 8 * 1024 // 8KB
	PingTimeoutBufferSeconds = 5
)

type msgType int

const (
	msgTypeJSON msgType = iota + 1
	msgTypePong
)

type writeMessage struct {
	msgType msgType
	data    interface{}
}

const avgReadMsgSizeBytes = 1024

// WebSocketClient stores the necessary information required to
// communicate with a WebSocket endpoint.
// A client must read from PingTimeoutChannel, EventChannel and ResponseChannel to prevent
// deadlocks from occurring in the program.
type WebSocketClient struct {
	URL                string                  // The location of the server like "ws://localhost:8065"
	APIURL             string                  // The API location of the server like "ws://localhost:8065/api/v3"
	ConnectURL         string                  // The WebSocket URL to connect to like "ws://localhost:8065/api/v3/path/to/websocket"
	Conn               *websocket.Conn         // The WebSocket connection
	AuthToken          string                  // The token used to open the WebSocket connection
	Sequence           int64                   // The ever-incrementing sequence attached to each WebSocket action
	PingTimeoutChannel chan bool               // The channel used to signal ping timeouts
	EventChannel       chan *WebSocketEvent    // The channel used to receive various events pushed from the server. For example: typing, posted
	ResponseChannel    chan *WebSocketResponse // The channel used to receive responses for requests made to the server
	ListenError        *AppError               // A field that is set if there was an abnormal closure of the WebSocket connection
	writeChan          chan writeMessage

	pingTimeoutTimer *time.Timer
	quitPingWatchdog chan struct{}

	quitWriterChan chan struct{}
	resetTimerChan chan struct{}
	closed         int32
}

// NewWebSocketClient constructs a new WebSocket client with convenience
// methods for talking to the server.
func NewWebSocketClient(url, authToken string) (*WebSocketClient, *AppError) {
	return NewWebSocketClientWithDialer(websocket.DefaultDialer, url, authToken)
}

// NewWebSocketClientWithDialer constructs a new WebSocket client with convenience
// methods for talking to the server using a custom dialer.
func NewWebSocketClientWithDialer(dialer *websocket.Dialer, url, authToken string) (*WebSocketClient, *AppError) {
	conn, _, err := dialer.Dial(url+APIURLSuffix+"/websocket", nil)
	if err != nil {
		return nil, NewAppError("NewWebSocketClient", "model.websocket_client.connect_fail.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	client := &WebSocketClient{
		URL:                url,
		APIURL:             url + APIURLSuffix,
		ConnectURL:         url + APIURLSuffix + "/websocket",
		Conn:               conn,
		AuthToken:          authToken,
		Sequence:           1,
		PingTimeoutChannel: make(chan bool, 1),
		EventChannel:       make(chan *WebSocketEvent, 100),
		ResponseChannel:    make(chan *WebSocketResponse, 100),
		writeChan:          make(chan writeMessage),
		quitPingWatchdog:   make(chan struct{}),
		quitWriterChan:     make(chan struct{}),
		resetTimerChan:     make(chan struct{}),
	}

	client.configurePingHandling()
	go client.writer()

	client.SendMessage(WebsocketAuthenticationChallenge, map[string]interface{}{"token": authToken})

	return client, nil
}

// NewWebSocketClient4 constructs a new WebSocket client with convenience
// methods for talking to the server. Uses the v4 endpoint.
func NewWebSocketClient4(url, authToken string) (*WebSocketClient, *AppError) {
	return NewWebSocketClient4WithDialer(websocket.DefaultDialer, url, authToken)
}

// NewWebSocketClient4WithDialer constructs a new WebSocket client with convenience
// methods for talking to the server using a custom dialer. Uses the v4 endpoint.
func NewWebSocketClient4WithDialer(dialer *websocket.Dialer, url, authToken string) (*WebSocketClient, *AppError) {
	return NewWebSocketClientWithDialer(dialer, url, authToken)
}

// Connect creates a websocket connection with the given ConnectUrl.
// This is racy and error-prone should not be used. Use any of the New* functions to create a websocket.
func (wsc *WebSocketClient) Connect() *AppError {
	return wsc.ConnectWithDialer(websocket.DefaultDialer)
}

// ConnectWithDialer creates a websocket connection with the given ConnectUrl using the dialer.
// This is racy and error-prone and should not be used. Use any of the New* functions to create a websocket.
func (wsc *WebSocketClient) ConnectWithDialer(dialer *websocket.Dialer) *AppError {
	var err error
	wsc.Conn, _, err = dialer.Dial(wsc.ConnectURL, nil)
	if err != nil {
		return NewAppError("Connect", "model.websocket_client.connect_fail.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	// Super racy and should not be done anyways.
	// All of this needs to be redesigned for v6.
	wsc.configurePingHandling()
	// If it has been closed before, we just restart the writer.
	if atomic.CompareAndSwapInt32(&wsc.closed, 1, 0) {
		wsc.writeChan = make(chan writeMessage)
		wsc.quitWriterChan = make(chan struct{})
		go wsc.writer()
		wsc.resetTimerChan = make(chan struct{})
		wsc.quitPingWatchdog = make(chan struct{})
	}

	wsc.EventChannel = make(chan *WebSocketEvent, 100)
	wsc.ResponseChannel = make(chan *WebSocketResponse, 100)

	wsc.SendMessage(WebsocketAuthenticationChallenge, map[string]interface{}{"token": wsc.AuthToken})

	return nil
}

// Close closes the websocket client. It is recommended that a closed client should not be
// reused again. Rather a new client should be created anew.
func (wsc *WebSocketClient) Close() {
	// CAS to 1 and proceed. Return if already 1.
	if !atomic.CompareAndSwapInt32(&wsc.closed, 0, 1) {
		return
	}
	wsc.quitWriterChan <- struct{}{}
	close(wsc.writeChan)
	// We close the connection, which breaks the reader loop.
	// Then we let the defer block in the reader do further cleanup.
	wsc.Conn.Close()
}

// TODO: un-export the Conn so that Write methods go through the writer
func (wsc *WebSocketClient) writer() {
	for {
		select {
		case msg := <-wsc.writeChan:
			switch msg.msgType {
			case msgTypeJSON:
				wsc.Conn.WriteJSON(msg.data)
			case msgTypePong:
				wsc.Conn.WriteMessage(websocket.PongMessage, []byte{})
			}
		case <-wsc.quitWriterChan:
			return
		}
	}
}

// Listen starts the read loop of the websocket client.
func (wsc *WebSocketClient) Listen() {
	// This loop can exit in 2 conditions:
	// 1. Either the connection breaks naturally.
	// 2. Close was explicitly called, which closes the connection manually.
	//
	// Due to the way the API is written, there is a requirement that a client may NOT
	// call Listen at all and can still call Close and Connect.
	// Therefore, we let the cleanup of the reader stuff rely on closing the connection
	// and then we do the cleanup in the defer block.
	//
	// First, we close some channels and then CAS to 1 and proceed to close the writer chan also.
	// This is needed because then the defer clause does not double-close the writer when (2) happens.
	// But if (1) happens, we set the closed bit, and close the rest of the stuff.
	go func() {
		defer func() {
			close(wsc.EventChannel)
			close(wsc.ResponseChannel)
			close(wsc.quitPingWatchdog)
			close(wsc.resetTimerChan)
			// We CAS to 1 and proceed.
			if !atomic.CompareAndSwapInt32(&wsc.closed, 0, 1) {
				return
			}
			wsc.quitWriterChan <- struct{}{}
			close(wsc.writeChan)
			wsc.Conn.Close() // This can most likely be removed. Needs to be checked.
		}()

		var buf bytes.Buffer
		buf.Grow(avgReadMsgSizeBytes)

		for {
			// Reset buffer.
			buf.Reset()
			_, r, err := wsc.Conn.NextReader()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					wsc.ListenError = NewAppError("NewWebSocketClient", "model.websocket_client.connect_fail.app_error", nil, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			// Use pre-allocated buffer.
			_, err = buf.ReadFrom(r)
			if err != nil {
				// This should use a different error ID, but en.json is not imported anyways.
				// It's a different bug altogether but we let it be for now.
				// See MM-24520.
				wsc.ListenError = NewAppError("NewWebSocketClient", "model.websocket_client.connect_fail.app_error", nil, err.Error(), http.StatusInternalServerError)
				return
			}

			event := WebSocketEventFromJSON(bytes.NewReader(buf.Bytes()))
			if event == nil {
				continue
			}
			if event.IsValid() {
				wsc.EventChannel <- event
				continue
			}

			var response WebSocketResponse
			if err := json.Unmarshal(buf.Bytes(), &response); err == nil && response.IsValid() {
				wsc.ResponseChannel <- &response
				continue
			}
		}
	}()
}

func (wsc *WebSocketClient) SendMessage(action string, data map[string]interface{}) {
	req := &WebSocketRequest{}
	req.Seq = wsc.Sequence
	req.Action = action
	req.Data = data

	wsc.Sequence++
	wsc.writeChan <- writeMessage{
		msgType: msgTypeJSON,
		data:    req,
	}
}

// UserTyping will push a user_typing event out to all connected users
// who are in the specified channel
func (wsc *WebSocketClient) UserTyping(channelID, parentID string) {
	data := map[string]interface{}{
		"channel_id": channelID,
		"parent_id":  parentID,
	}

	wsc.SendMessage("user_typing", data)
}

// GetStatuses will return a map of string statuses using user id as the key
func (wsc *WebSocketClient) GetStatuses() {
	wsc.SendMessage("get_statuses", nil)
}

// GetStatusesByIds will fetch certain user statuses based on ids and return
// a map of string statuses using user id as the key
func (wsc *WebSocketClient) GetStatusesByIDs(userIDs []string) {
	data := map[string]interface{}{
		"user_ids": userIDs,
	}
	wsc.SendMessage("get_statuses_by_ids", data)
}

func (wsc *WebSocketClient) configurePingHandling() {
	wsc.Conn.SetPingHandler(wsc.pingHandler)
	wsc.pingTimeoutTimer = time.NewTimer(time.Second * (60 + PingTimeoutBufferSeconds))
	go wsc.pingWatchdog()
}

func (wsc *WebSocketClient) pingHandler(appData string) error {
	if atomic.LoadInt32(&wsc.closed) == 1 {
		return nil
	}
	wsc.resetTimerChan <- struct{}{}
	wsc.writeChan <- writeMessage{
		msgType: msgTypePong,
	}
	return nil
}

// pingWatchdog is used to send values to the PingTimeoutChannel whenever a timeout occurs.
// We use the resetTimerChan from the pingHandler to pass the signal, and then reset the timer
// after draining it. And if the timer naturally expires, we also extend it to prevent it from
// being deadlocked when the resetTimerChan case runs. Because timer.Stop would return false,
// and the code would be forever stuck trying to read from C.
func (wsc *WebSocketClient) pingWatchdog() {
	for {
		select {
		case <-wsc.resetTimerChan:
			if !wsc.pingTimeoutTimer.Stop() {
				<-wsc.pingTimeoutTimer.C
			}
			wsc.pingTimeoutTimer.Reset(time.Second * (60 + PingTimeoutBufferSeconds))

		case <-wsc.pingTimeoutTimer.C:
			wsc.PingTimeoutChannel <- true
			wsc.pingTimeoutTimer.Reset(time.Second * (60 + PingTimeoutBufferSeconds))
		case <-wsc.quitPingWatchdog:
			return
		}
	}
}
