// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type webSocketHandler interface {
	ServeWebSocket(*WebConn, *model.WebSocketRequest)
}

type WebSocketRouter struct {
	app      *App
	handlers map[string]webSocketHandler
}

func (wr *WebSocketRouter) Handle(action string, handler webSocketHandler) {
	wr.handlers[action] = handler
}

func (wr *WebSocketRouter) ServeWebSocket(conn *WebConn, r *model.WebSocketRequest) {
	if r.Action == "" {
		err := model.NewAppError("ServeWebSocket", "api.web_socket_router.no_action.app_error", nil, "", http.StatusBadRequest)
		returnWebSocketError(wr.app, conn, r, err)
		return
	}

	if r.Seq <= 0 {
		err := model.NewAppError("ServeWebSocket", "api.web_socket_router.bad_seq.app_error", nil, "", http.StatusBadRequest)
		returnWebSocketError(wr.app, conn, r, err)
		return
	}

	if r.Action == model.WebsocketAuthenticationChallenge {
		if conn.GetSessionToken() != "" {
			return
		}

		token, ok := r.Data["token"].(string)
		if !ok {
			conn.WebSocket.Close()
			return
		}

		session, err := wr.app.GetSession(token)
		if err != nil {
			conn.WebSocket.Close()
			return
		}
		conn.SetSession(session)
		conn.SetSessionToken(session.Token)
		conn.UserID = session.UserID

		// TODO: Same logic to reconnect queue as api4/websocket.go

		wr.app.HubRegister(conn)

		wr.app.Srv().Go(func() {
			wr.app.SetStatusOnline(session.UserID, false)
			wr.app.UpdateLastActivityAtIfNeeded(*session)
		})

		resp := model.NewWebSocketResponse(model.StatusOk, r.Seq, nil)
		hub := wr.app.GetHubForUserID(conn.UserID)
		if hub == nil {
			return
		}
		hub.SendMessage(conn, resp)

		return
	}

	if !conn.IsAuthenticated() {
		err := model.NewAppError("ServeWebSocket", "api.web_socket_router.not_authenticated.app_error", nil, "", http.StatusUnauthorized)
		returnWebSocketError(wr.app, conn, r, err)
		return
	}

	handler, ok := wr.handlers[r.Action]
	if !ok {
		err := model.NewAppError("ServeWebSocket", "api.web_socket_router.bad_action.app_error", nil, "", http.StatusInternalServerError)
		returnWebSocketError(wr.app, conn, r, err)
		return
	}

	handler.ServeWebSocket(conn, r)
}

func returnWebSocketError(app *App, conn *WebConn, r *model.WebSocketRequest, err *model.AppError) {
	logF := mlog.Error
	if err.StatusCode >= http.StatusBadRequest && err.StatusCode < http.StatusInternalServerError {
		logF = mlog.Debug
	}
	logF(
		"websocket routing error.",
		mlog.Int64("seq", r.Seq),
		mlog.String("user_id", conn.UserID),
		mlog.String("system_message", err.SystemMessage(i18n.T)),
		mlog.Err(err),
	)

	hub := app.GetHubForUserID(conn.UserID)
	if hub == nil {
		return
	}

	err.DetailedError = ""
	errorResp := model.NewWebSocketError(r.Seq, err)
	hub.SendMessage(conn, errorResp)
}
