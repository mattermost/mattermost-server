// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package remotecluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wiggin77/merror"

	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	TestTopics  = " share incident "
	TestTopic   = "share"
	NumRemotes  = 50
	NoteContent = "Woot!!"
)

type testPayload struct {
	Note string `json:"note"`
}

func TestBroadcastMsg(t *testing.T) {
	msgID := model.NewID()
	disablePing = true

	t.Run("No error", func(t *testing.T) {
		var countCallbacks int32
		var countWebReq int32
		merr := merror.New()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				w.WriteHeader(200)
				var resp Response
				b, errMarshall := json.Marshal(&resp)
				if errMarshall != nil {
					merr.Append(errMarshall)
					return
				}
				w.Write(b)
			}()

			atomic.AddInt32(&countWebReq, 1)

			frame, appErr := model.RemoteClusterFrameFromJSON(r.Body)
			if appErr != nil {
				merr.Append(appErr)
				return
			}
			if len(frame.Msg.Payload) == 0 {
				merr.Append(fmt.Errorf("webrequest missing Msg.Payload"))
			}
			if msgID != frame.Msg.ID {
				merr.Append(fmt.Errorf("webrequest msgId expected %s, got %s", msgID, frame.Msg.ID))
				return
			}

			note := testPayload{}
			err := json.Unmarshal(frame.Msg.Payload, &note)
			if err != nil {
				merr.Append(err)
				return
			}
			if note.Note != NoteContent {
				merr.Append(fmt.Errorf("webrequest payload expected %s, got %s", NoteContent, note.Note))
				return
			}
		}))
		defer ts.Close()

		mockServer := newMockServer(t, makeRemoteClusters(NumRemotes, ts.URL))
		defer mockServer.Shutdown()

		service, err := NewRemoteClusterService(mockServer)
		require.NoError(t, err)

		err = service.Start()
		require.NoError(t, err)
		defer service.Shutdown()

		wg := &sync.WaitGroup{}
		wg.Add(NumRemotes)

		msg := makeRemoteClusterMsg(msgID, NoteContent)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
		defer cancel()

		err = service.BroadcastMsg(ctx, msg, func(msg model.RemoteClusterMsg, remote *model.RemoteCluster, resp *Response, err error) {
			defer wg.Done()
			atomic.AddInt32(&countCallbacks, 1)

			if err != nil {
				merr.Append(err)
			}
			if msgID != msg.ID {
				merr.Append(fmt.Errorf("result callback msgId expected %s, got %s", msgID, msg.ID))
			}

			var note testPayload
			err2 := json.Unmarshal(msg.Payload, &note)
			if err2 != nil {
				merr.Append(fmt.Errorf("unmarshal payload error: %w", err2))
				return
			}
			if note.Note != NoteContent {
				merr.Append(fmt.Errorf("compare payload failed: expected '%s', got '%s'", NoteContent, note))
			}
		})
		assert.NoError(t, err)

		wg.Wait()

		assert.NoError(t, merr.ErrorOrNil())

		assert.Equal(t, int32(NumRemotes), atomic.LoadInt32(&countCallbacks))
		assert.Equal(t, int32(NumRemotes), atomic.LoadInt32(&countWebReq))
		t.Log(fmt.Sprintf("%d callbacks counted;  %d web requests counted;  %d expected",
			atomic.LoadInt32(&countCallbacks), atomic.LoadInt32(&countWebReq), NumRemotes))
	})

	t.Run("HTTP error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))
		defer ts.Close()

		mockServer := newMockServer(t, makeRemoteClusters(NumRemotes, ts.URL))
		defer mockServer.Shutdown()

		service, err := NewRemoteClusterService(mockServer)
		require.NoError(t, err)

		err = service.Start()
		require.NoError(t, err)
		defer service.Shutdown()

		msg := makeRemoteClusterMsg(msgID, NoteContent)
		var countCallbacks int32
		var countErrors int32
		wg := &sync.WaitGroup{}
		wg.Add(NumRemotes)

		err = service.BroadcastMsg(context.Background(), msg, func(msg model.RemoteClusterMsg, remote *model.RemoteCluster, resp *Response, err error) {
			defer wg.Done()
			atomic.AddInt32(&countCallbacks, 1)
			if err != nil {
				atomic.AddInt32(&countErrors, 1)
			}
		})
		assert.NoError(t, err)

		wg.Wait()

		assert.Equal(t, int32(NumRemotes), atomic.LoadInt32(&countCallbacks))
		assert.Equal(t, int32(NumRemotes), atomic.LoadInt32(&countErrors))
	})
}

func makeRemoteClusters(num int, siteURL string) []*model.RemoteCluster {
	var remotes []*model.RemoteCluster
	for i := 0; i < num; i++ {
		rc := makeRemoteCluster(fmt.Sprintf("test cluster %d", i+1), siteURL, TestTopics)
		remotes = append(remotes, rc)
	}
	return remotes
}

func makeRemoteCluster(name string, siteURL string, topics string) *model.RemoteCluster {
	return &model.RemoteCluster{
		RemoteID:   model.NewID(),
		Name:       name,
		SiteURL:    siteURL,
		Token:      model.NewID(),
		Topics:     topics,
		CreateAt:   model.GetMillis(),
		LastPingAt: model.GetMillis(),
		CreatorID:  model.NewID(),
	}
}

func makeRemoteClusterMsg(id string, note string) model.RemoteClusterMsg {
	payload := testPayload{Note: note}
	raw, _ := json.Marshal(payload)

	return model.RemoteClusterMsg{
		ID:       id,
		Topic:    TestTopic,
		CreateAt: model.GetMillis(),
		Payload:  raw}
}
