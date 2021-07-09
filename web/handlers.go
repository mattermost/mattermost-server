// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/gziphandler"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	spanlog "github.com/opentracing/opentracing-go/log"

	"github.com/mattermost/mattermost-server/v5/app"
	app_opentracing "github.com/mattermost/mattermost-server/v5/app/opentracing"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/tracing"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store/opentracinglayer"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func GetHandlerName(h func(*Context, http.ResponseWriter, *http.Request)) string {
	handlerName := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	pos := strings.LastIndex(handlerName, ".")
	if pos != -1 && len(handlerName) > pos {
		handlerName = handlerName[pos+1:]
	}
	return handlerName
}

func (w *Web) NewHandler(h func(*Context, http.ResponseWriter, *http.Request)) http.Handler {
	return &Handler{
		App:            w.app,
		HandleFunc:     h,
		HandlerName:    GetHandlerName(h),
		RequireSession: false,
		TrustRequester: false,
		RequireMfa:     false,
		IsStatic:       false,
		IsLocal:        false,
	}
}

func (w *Web) NewStaticHandler(h func(*Context, http.ResponseWriter, *http.Request)) http.Handler {
	// Determine the CSP SHA directive needed for subpath support, if any. This value is fixed
	// on server start and intentionally requires a restart to take effect.
	subpath, _ := utils.GetSubpathFromConfig(w.app.Config())

	return &Handler{
		App:            w.app,
		HandleFunc:     h,
		HandlerName:    GetHandlerName(h),
		RequireSession: false,
		TrustRequester: false,
		RequireMfa:     false,
		IsStatic:       true,

		cspShaDirective: utils.GetSubpathScriptHash(subpath),
	}
}

type Handler struct {
	App                       app.AppIface
	HandleFunc                func(*Context, http.ResponseWriter, *http.Request)
	HandlerName               string
	RequireSession            bool
	RequireCloudKey           bool
	RequireRemoteClusterToken bool
	TrustRequester            bool
	RequireMfa                bool
	IsStatic                  bool
	IsLocal                   bool
	DisableWhenBusy           bool

	cspShaDirective string
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w = newWrappedWriter(w)
	now := time.Now()

	requestID := model.NewID()
	var statusCode string
	defer func() {
		responseLogFields := []mlog.Field{
			mlog.String("method", r.Method),
			mlog.String("url", r.URL.Path),
			mlog.String("request_id", requestID),
		}
		// Websockets are returning status code 0 to requests after closing the socket
		if statusCode != "0" {
			responseLogFields = append(responseLogFields, mlog.String("status_code", statusCode))
		}
		mlog.Debug("Received HTTP request", responseLogFields...)
	}()

	c := &Context{
		AppContext: &request.Context{},
		App:        h.App,
	}

	t, _ := i18n.GetTranslationsAndLocaleFromRequest(r)
	c.AppContext.SetT(t)
	c.AppContext.SetRequestID(requestID)
	c.AppContext.SetIDAddress(utils.GetIPAddress(r, c.App.Config().ServiceSettings.TrustedProxyIPHeader))
	c.AppContext.SetUserAgent(r.UserAgent())
	c.AppContext.SetAcceptLanguage(r.Header.Get("Accept-Language"))
	c.AppContext.SetPath(r.URL.Path)
	c.Params = ParamsFromRequest(r)
	c.Logger = c.App.Log()

	if *c.App.Config().ServiceSettings.EnableOpenTracing {
		span, ctx := tracing.StartRootSpanByContext(context.Background(), "web:ServeHTTP")
		carrier := opentracing.HTTPHeadersCarrier(r.Header)
		_ = opentracing.GlobalTracer().Inject(span.Context(), opentracing.HTTPHeaders, carrier)
		ext.HTTPMethod.Set(span, r.Method)
		ext.HTTPUrl.Set(span, c.AppContext.Path())
		ext.PeerAddress.Set(span, c.AppContext.IDAddress())
		span.SetTag("request_id", c.AppContext.RequestID())
		span.SetTag("user_agent", c.AppContext.UserAgent())

		defer func() {
			if c.Err != nil {
				span.LogFields(spanlog.Error(c.Err))
				ext.HTTPStatusCode.Set(span, uint16(c.Err.StatusCode))
				ext.Error.Set(span, true)
			}
			span.Finish()
		}()
		c.AppContext.SetContext(ctx)

		tmpSrv := *c.App.Srv()
		tmpSrv.Store = opentracinglayer.New(c.App.Srv().Store, ctx)
		c.App.SetServer(&tmpSrv)
		c.App = app_opentracing.NewOpenTracingAppLayer(c.App, ctx)
	}

	// Set the max request body size to be equal to MaxFileSize.
	// Ideally, non-file request bodies should be smaller than file request bodies,
	// but we don't have a clean way to identify all file upload handlers.
	// So to keep it simple, we clamp it to the max file size.
	// We add a buffer of bytes.MinRead so that file sizes close to max file size
	// do not get cut off.
	r.Body = http.MaxBytesReader(w, r.Body, *c.App.Config().FileSettings.MaxFileSize+bytes.MinRead)

	subpath, _ := utils.GetSubpathFromConfig(c.App.Config())
	siteURLHeader := app.GetProtocol(r) + "://" + r.Host + subpath
	c.SetSiteURLHeader(siteURLHeader)

	w.Header().Set(model.HeaderRequestID, c.AppContext.RequestID())
	w.Header().Set(model.HeaderVersionID, fmt.Sprintf("%v.%v.%v.%v", model.CurrentVersion, model.BuildNumber, c.App.ClientConfigHash(), c.App.Srv().License() != nil))

	if *c.App.Config().ServiceSettings.TLSStrictTransport {
		w.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d", *c.App.Config().ServiceSettings.TLSStrictTransportMaxAge))
	}

	cloudCSP := ""
	if c.App.Srv().License() != nil && *c.App.Srv().License().Features.Cloud {
		cloudCSP = " js.stripe.com/v3"
	}

	if h.IsStatic {
		// Instruct the browser not to display us in an iframe unless is the same origin for anti-clickjacking
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")

		// Add unsafe-eval to the content security policy for faster source maps in development mode
		devCSP := ""
		if model.BuildNumber == "dev" {
			devCSP += " 'unsafe-eval'"
		}

		// Add unsafe-inline to unlock extensions like React & Redux DevTools in Firefox
		// see https://github.com/reduxjs/redux-devtools/issues/380
		if model.BuildNumber == "dev" {
			devCSP += " 'unsafe-inline'"
		}

		// Set content security policy. This is also specified in the root.html of the webapp in a meta tag.
		w.Header().Set("Content-Security-Policy", fmt.Sprintf(
			"frame-ancestors 'self'; script-src 'self' cdn.rudderlabs.com%s%s%s",
			cloudCSP,
			h.cspShaDirective,
			devCSP,
		))
	} else {
		// All api response bodies will be JSON formatted by default
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" {
			w.Header().Set("Expires", "0")
		}
	}

	token, tokenLocation := app.ParseAuthTokenFromRequest(r)

	if token != "" && tokenLocation != app.TokenLocationCloudHeader && tokenLocation != app.TokenLocationRemoteClusterHeader {
		session, err := c.App.GetSession(token)
		defer c.App.ReturnSessionToPool(session)

		if err != nil {
			c.Logger.Info("Invalid session", mlog.Err(err))
			if err.StatusCode == http.StatusInternalServerError {
				c.Err = err
			} else if h.RequireSession {
				c.RemoveSessionCookie(w, r)
				c.Err = model.NewAppError("ServeHTTP", "api.context.session_expired.app_error", nil, "token="+token, http.StatusUnauthorized)
			}
		} else if !session.IsOAuth && tokenLocation == app.TokenLocationQueryString {
			c.Err = model.NewAppError("ServeHTTP", "api.context.token_provided.app_error", nil, "token="+token, http.StatusUnauthorized)
		} else {
			c.AppContext.SetSession(session)
		}

		// Rate limit by UserID
		if c.App.Srv().RateLimiter != nil && c.App.Srv().RateLimiter.UserIDRateLimit(c.AppContext.Session().UserID, w) {
			return
		}

		h.checkCSRFToken(c, r, token, tokenLocation, session)
	} else if token != "" && c.App.Srv().License() != nil && *c.App.Srv().License().Features.Cloud && tokenLocation == app.TokenLocationCloudHeader {
		// Check to see if this provided token matches our CWS Token
		session, err := c.App.GetCloudSession(token)
		if err != nil {
			c.Logger.Warn("Invalid CWS token", mlog.Err(err))
			c.Err = err
		} else {
			c.AppContext.SetSession(session)
		}
	} else if token != "" && c.App.Srv().License() != nil && *c.App.Srv().License().Features.RemoteClusterService && tokenLocation == app.TokenLocationRemoteClusterHeader {
		// Get the remote cluster
		if remoteID := c.GetRemoteID(r); remoteID == "" {
			c.Logger.Warn("Missing remote cluster id") //
			c.Err = model.NewAppError("ServeHTTP", "api.context.remote_id_missing.app_error", nil, "", http.StatusUnauthorized)
		} else {
			// Check the token is correct for the remote cluster id.
			session, err := c.App.GetRemoteClusterSession(token, remoteID)
			if err != nil {
				c.Logger.Warn("Invalid remote cluster token", mlog.Err(err))
				c.Err = err
			} else {
				c.AppContext.SetSession(session)
			}
		}
	}

	c.Logger = c.App.Log().With(
		mlog.String("path", c.AppContext.Path()),
		mlog.String("request_id", c.AppContext.RequestID()),
		mlog.String("ip_addr", c.AppContext.IDAddress()),
		mlog.String("user_id", c.AppContext.Session().UserID),
		mlog.String("method", r.Method),
	)

	if c.Err == nil && h.RequireSession {
		c.SessionRequired()
	}

	if c.Err == nil && h.RequireMfa {
		c.MfaRequired()
	}

	if c.Err == nil && h.DisableWhenBusy && c.App.Srv().Busy.IsBusy() {
		c.SetServerBusyError()
	}

	if c.Err == nil && h.RequireCloudKey {
		c.CloudKeyRequired()
	}

	if c.Err == nil && h.RequireRemoteClusterToken {
		c.RemoteClusterTokenRequired()
	}

	if c.Err == nil && h.IsLocal {
		// if the connection is local, RemoteAddr shouldn't have the
		// shape IP:PORT (it will be "@" in Linux, for example)
		isLocalOrigin := !strings.Contains(r.RemoteAddr, ":")
		if *c.App.Config().ServiceSettings.EnableLocalMode && isLocalOrigin {
			c.AppContext.SetSession(&model.Session{Local: true})
		} else if !isLocalOrigin {
			c.Err = model.NewAppError("", "api.context.local_origin_required.app_error", nil, "LocalOriginRequired", http.StatusUnauthorized)
		}
	}

	if c.Err == nil {
		h.HandleFunc(c, w, r)
	}

	// Handle errors that have occurred
	if c.Err != nil {
		c.Err.Translate(c.AppContext.T)
		c.Err.RequestID = c.AppContext.RequestID()
		c.LogErrorByCode(c.Err)

		c.Err.Where = r.URL.Path

		// Block out detailed error when not in developer mode
		if !*c.App.Config().ServiceSettings.EnableDeveloper {
			c.Err.DetailedError = ""
		}

		// Sanitize all 5xx error messages in hardened mode
		if *c.App.Config().ServiceSettings.ExperimentalEnableHardenedMode && c.Err.StatusCode >= 500 {
			c.Err.ID = ""
			c.Err.Message = "Internal Server Error"
			c.Err.DetailedError = ""
			c.Err.StatusCode = 500
			c.Err.Where = ""
			c.Err.IsOAuth = false
		}

		if IsAPICall(c.App, r) || IsWebhookCall(c.App, r) || IsOAuthAPICall(c.App, r) || r.Header.Get("X-Mobile-App") != "" {
			w.WriteHeader(c.Err.StatusCode)
			w.Write([]byte(c.Err.ToJSON()))
		} else {
			utils.RenderWebAppError(c.App.Config(), w, r, c.Err, c.App.AsymmetricSigningKey())
		}

		if c.App.Metrics() != nil {
			c.App.Metrics().IncrementHTTPError()
		}
	}

	statusCode = strconv.Itoa(w.(*responseWriterWrapper).StatusCode())
	if c.App.Metrics() != nil {
		c.App.Metrics().IncrementHTTPRequest()

		if r.URL.Path != model.APIURLSuffix+"/websocket" {
			elapsed := float64(time.Since(now)) / float64(time.Second)
			c.App.Metrics().ObserveAPIEndpointDuration(h.HandlerName, r.Method, statusCode, elapsed)
		}
	}
}

// checkCSRFToken performs a CSRF check on the provided request with the given CSRF token. Returns whether or not
// a CSRF check occurred and whether or not it succeeded.
func (h *Handler) checkCSRFToken(c *Context, r *http.Request, token string, tokenLocation app.TokenLocation, session *model.Session) (checked bool, passed bool) {
	csrfCheckNeeded := session != nil && c.Err == nil && tokenLocation == app.TokenLocationCookie && !h.TrustRequester && r.Method != "GET"
	csrfCheckPassed := false

	if csrfCheckNeeded {
		csrfHeader := r.Header.Get(model.HeaderCsrfToken)

		if csrfHeader == session.GetCSRF() {
			csrfCheckPassed = true
		} else if r.Header.Get(model.HeaderRequestedWith) == model.HeaderRequestedWithXml {
			// ToDo(DSchalla) 2019/01/04: Remove after deprecation period and only allow CSRF Header (MM-13657)
			csrfErrorMessage := "CSRF Header check failed for request - Please upgrade your web application or custom app to set a CSRF Header"

			sid := ""
			userID := ""

			if session != nil {
				sid = session.ID
				userID = session.UserID
			}

			fields := []mlog.Field{
				mlog.String("path", r.URL.Path),
				mlog.String("ip", r.RemoteAddr),
				mlog.String("session_id", sid),
				mlog.String("user_id", userID),
			}

			if *c.App.Config().ServiceSettings.ExperimentalStrictCSRFEnforcement {
				c.Logger.Warn(csrfErrorMessage, fields...)
			} else {
				c.Logger.Debug(csrfErrorMessage, fields...)
				csrfCheckPassed = true
			}
		}

		if !csrfCheckPassed {
			c.AppContext.SetSession(&model.Session{})
			c.Err = model.NewAppError("ServeHTTP", "api.context.session_expired.app_error", nil, "token="+token+" Appears to be a CSRF attempt", http.StatusUnauthorized)
		}
	}

	return csrfCheckNeeded, csrfCheckPassed
}

// ApiHandler provides a handler for API endpoints which do not require the user to be logged in order for access to be
// granted.
func (w *Web) APIHandler(h func(*Context, http.ResponseWriter, *http.Request)) http.Handler {
	handler := &Handler{
		App:            w.app,
		HandleFunc:     h,
		HandlerName:    GetHandlerName(h),
		RequireSession: false,
		TrustRequester: false,
		RequireMfa:     false,
		IsStatic:       false,
		IsLocal:        false,
	}
	if *w.app.Config().ServiceSettings.WebserverMode == "gzip" {
		return gziphandler.GzipHandler(handler)
	}
	return handler
}

// ApiHandlerTrustRequester provides a handler for API endpoints which do not require the user to be logged in and are
// allowed to be requested directly rather than via javascript/XMLHttpRequest, such as site branding images or the
// websocket.
func (w *Web) APIHandlerTrustRequester(h func(*Context, http.ResponseWriter, *http.Request)) http.Handler {
	handler := &Handler{
		App:            w.app,
		HandleFunc:     h,
		HandlerName:    GetHandlerName(h),
		RequireSession: false,
		TrustRequester: true,
		RequireMfa:     false,
		IsStatic:       false,
		IsLocal:        false,
	}
	if *w.app.Config().ServiceSettings.WebserverMode == "gzip" {
		return gziphandler.GzipHandler(handler)
	}
	return handler
}

// ApiSessionRequired provides a handler for API endpoints which require the user to be logged in in order for access to
// be granted.
func (w *Web) APISessionRequired(h func(*Context, http.ResponseWriter, *http.Request)) http.Handler {
	handler := &Handler{
		App:            w.app,
		HandleFunc:     h,
		HandlerName:    GetHandlerName(h),
		RequireSession: true,
		TrustRequester: false,
		RequireMfa:     true,
		IsStatic:       false,
		IsLocal:        false,
	}
	if *w.app.Config().ServiceSettings.WebserverMode == "gzip" {
		return gziphandler.GzipHandler(handler)
	}
	return handler
}
