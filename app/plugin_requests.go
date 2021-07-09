// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func (s *Server) ServePluginRequest(w http.ResponseWriter, r *http.Request) {
	pluginsEnvironment := s.GetPluginsEnvironment()
	if pluginsEnvironment == nil {
		err := model.NewAppError("ServePluginRequest", "app.plugin.disabled.app_error", nil, "Enable plugins to serve plugin requests", http.StatusNotImplemented)
		s.Log.Error(err.Error())
		w.WriteHeader(err.StatusCode)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(err.ToJSON()))
		return
	}

	params := mux.Vars(r)
	hooks, err := pluginsEnvironment.HooksForPlugin(params["plugin_id"])
	if err != nil {
		s.Log.Error("Access to route for non-existent plugin",
			mlog.String("missing_plugin_id", params["plugin_id"]),
			mlog.String("url", r.URL.String()),
			mlog.Err(err))
		http.NotFound(w, r)
		return
	}

	s.servePluginRequest(w, r, hooks.ServeHTTP)
}

func (a *App) ServeInterPluginRequest(w http.ResponseWriter, r *http.Request, sourcePluginID, destinationPluginID string) {
	pluginsEnvironment := a.GetPluginsEnvironment()
	if pluginsEnvironment == nil {
		err := model.NewAppError("ServeInterPluginRequest", "app.plugin.disabled.app_error", nil, "Plugin environment not found.", http.StatusNotImplemented)
		a.Log().Error(err.Error())
		w.WriteHeader(err.StatusCode)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(err.ToJSON()))
		return
	}

	hooks, err := pluginsEnvironment.HooksForPlugin(destinationPluginID)
	if err != nil {
		a.Log().Error("Access to route for non-existent plugin in inter plugin request",
			mlog.String("source_plugin_id", sourcePluginID),
			mlog.String("destination_plugin_id", destinationPluginID),
			mlog.String("url", r.URL.String()),
			mlog.Err(err),
		)
		http.NotFound(w, r)
		return
	}

	context := &plugin.Context{
		RequestID:      model.NewID(),
		UserAgent:      r.UserAgent(),
		SourcePluginID: sourcePluginID,
	}

	r.Header.Set("Mattermost-Plugin-ID", sourcePluginID)

	hooks.ServeHTTP(context, w, r)
}

// ServePluginPublicRequest serves public plugin files
// at the URL http(s)://$SITE_URL/plugins/$PLUGIN_ID/public/{anything}
func (s *Server) ServePluginPublicRequest(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}

	// Should be in the form of /$PLUGIN_ID/public/{anything} by the time we get here
	vars := mux.Vars(r)
	pluginID := vars["plugin_id"]

	pluginsEnv := s.GetPluginsEnvironment()

	// Check if someone has nullified the pluginsEnv in the meantime
	if pluginsEnv == nil {
		http.NotFound(w, r)
		return
	}

	publicFilesPath, err := pluginsEnv.PublicFilesPath(pluginID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	publicFilePath := path.Clean(r.URL.Path)
	prefix := fmt.Sprintf("/plugins/%s/public/", pluginID)
	if !strings.HasPrefix(publicFilePath, prefix) {
		http.NotFound(w, r)
		return
	}
	publicFile := filepath.Join(publicFilesPath, strings.TrimPrefix(publicFilePath, prefix))
	http.ServeFile(w, r, publicFile)
}

func (s *Server) servePluginRequest(w http.ResponseWriter, r *http.Request, handler func(*plugin.Context, http.ResponseWriter, *http.Request)) {
	token := ""
	context := &plugin.Context{
		RequestID:      model.NewID(),
		IDAddress:      utils.GetIPAddress(r, s.Config().ServiceSettings.TrustedProxyIPHeader),
		AcceptLanguage: r.Header.Get("Accept-Language"),
		UserAgent:      r.UserAgent(),
	}
	cookieAuth := false

	authHeader := r.Header.Get(model.HeaderAuth)
	if strings.HasPrefix(strings.ToUpper(authHeader), model.HeaderBearer+" ") {
		token = authHeader[len(model.HeaderBearer)+1:]
	} else if strings.HasPrefix(strings.ToLower(authHeader), model.HeaderToken+" ") {
		token = authHeader[len(model.HeaderToken)+1:]
	} else if cookie, _ := r.Cookie(model.SessionCookieToken); cookie != nil {
		token = cookie.Value
		cookieAuth = true
	} else {
		token = r.URL.Query().Get("access_token")
	}

	// Mattermost-Plugin-ID can only be set by inter-plugin requests
	r.Header.Del("Mattermost-Plugin-ID")

	r.Header.Del("Mattermost-User-Id")
	if token != "" {
		session, err := New(ServerConnector(s)).GetSession(token)
		defer s.userService.ReturnSessionToPool(session)

		csrfCheckPassed := false

		if session != nil && err == nil && cookieAuth && r.Method != "GET" {
			sentToken := ""

			if r.Header.Get(model.HeaderCsrfToken) == "" {
				bodyBytes, _ := ioutil.ReadAll(r.Body)
				r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
				r.ParseForm()
				sentToken = r.FormValue("csrf")
				r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			} else {
				sentToken = r.Header.Get(model.HeaderCsrfToken)
			}

			expectedToken := session.GetCSRF()

			if sentToken == expectedToken {
				csrfCheckPassed = true
			}

			// ToDo(DSchalla) 2019/01/04: Remove after deprecation period and only allow CSRF Header (MM-13657)
			if r.Header.Get(model.HeaderRequestedWith) == model.HeaderRequestedWithXml && !csrfCheckPassed {
				csrfErrorMessage := "CSRF Check failed for request - Please migrate your plugin to either send a CSRF Header or Form Field, XMLHttpRequest is deprecated"
				sid := ""
				userID := ""

				if session.ID != "" {
					sid = session.ID
					userID = session.UserID
				}

				fields := []mlog.Field{
					mlog.String("path", r.URL.Path),
					mlog.String("ip", r.RemoteAddr),
					mlog.String("session_id", sid),
					mlog.String("user_id", userID),
				}

				if *s.Config().ServiceSettings.ExperimentalStrictCSRFEnforcement {
					s.Log.Warn(csrfErrorMessage, fields...)
				} else {
					s.Log.Debug(csrfErrorMessage, fields...)
					csrfCheckPassed = true
				}
			}
		} else {
			csrfCheckPassed = true
		}

		if (session != nil && session.ID != "") && err == nil && csrfCheckPassed {
			r.Header.Set("Mattermost-User-Id", session.UserID)
			context.SessionID = session.ID
		}
	}

	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range cookies {
		if c.Name != model.SessionCookieToken {
			r.AddCookie(c)
		}
	}
	r.Header.Del(model.HeaderAuth)
	r.Header.Del("Referer")

	params := mux.Vars(r)

	subpath, _ := utils.GetSubpathFromConfig(s.Config())

	newQuery := r.URL.Query()
	newQuery.Del("access_token")
	r.URL.RawQuery = newQuery.Encode()
	r.URL.Path = strings.TrimPrefix(r.URL.Path, path.Join(subpath, "plugins", params["plugin_id"]))

	handler(context, w, r)
}
