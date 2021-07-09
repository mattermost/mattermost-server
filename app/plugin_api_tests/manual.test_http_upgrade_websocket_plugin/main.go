// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer ws.Close()

	for {
		mt, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		req := model.WebSocketRequestFromJSON(bytes.NewReader(msg))
		resp := model.NewWebSocketResponse("OK", req.Seq, map[string]interface{}{"action": req.Action, "value": req.Data["value"]})
		if err = ws.WriteMessage(mt, []byte(resp.ToJSON())); err != nil {
			break
		}
	}
}

func main() {
	plugin.ClientMain(&Plugin{})
}
