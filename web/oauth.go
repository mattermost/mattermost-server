// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"html"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func (w *Web) InitOAuth() {
	// API version independent OAuth 2.0 as a service provider endpoints
	w.MainRouter.Handle("/oauth/authorize", w.APIHandlerTrustRequester(authorizeOAuthPage)).Methods("GET")
	w.MainRouter.Handle("/oauth/authorize", w.APISessionRequired(authorizeOAuthApp)).Methods("POST")
	w.MainRouter.Handle("/oauth/deauthorize", w.APISessionRequired(deauthorizeOAuthApp)).Methods("POST")
	w.MainRouter.Handle("/oauth/access_token", w.APIHandlerTrustRequester(getAccessToken)).Methods("POST")

	// API version independent OAuth as a client endpoints
	w.MainRouter.Handle("/oauth/{service:[A-Za-z0-9]+}/complete", w.APIHandler(completeOAuth)).Methods("GET")
	w.MainRouter.Handle("/oauth/{service:[A-Za-z0-9]+}/login", w.APIHandler(loginWithOAuth)).Methods("GET")
	w.MainRouter.Handle("/oauth/{service:[A-Za-z0-9]+}/mobile_login", w.APIHandler(mobileLoginWithOAuth)).Methods("GET")
	w.MainRouter.Handle("/oauth/{service:[A-Za-z0-9]+}/signup", w.APIHandler(signupWithOAuth)).Methods("GET")

	// Old endpoints for backwards compatibility, needed to not break SSO for any old setups
	w.MainRouter.Handle("/api/v3/oauth/{service:[A-Za-z0-9]+}/complete", w.APIHandler(completeOAuth)).Methods("GET")
	w.MainRouter.Handle("/signup/{service:[A-Za-z0-9]+}/complete", w.APIHandler(completeOAuth)).Methods("GET")
	w.MainRouter.Handle("/login/{service:[A-Za-z0-9]+}/complete", w.APIHandler(completeOAuth)).Methods("GET")
	w.MainRouter.Handle("/api/v4/oauth_test", w.APISessionRequired(testHandler)).Methods("GET")
}

func testHandler(c *Context, w http.ResponseWriter, r *http.Request) {
	ReturnStatusOK(w)
}

func authorizeOAuthApp(c *Context, w http.ResponseWriter, r *http.Request) {
	authRequest := model.AuthorizeRequestFromJSON(r.Body)
	if authRequest == nil {
		c.SetInvalidParam("authorize_request")
		return
	}

	if err := authRequest.IsValid(); err != nil {
		c.Err = err
		return
	}

	if c.AppContext.Session().IsOAuth {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		c.Err.DetailedError += ", attempted access by oauth app"
		return
	}

	auditRec := c.MakeAuditRecord("authorizeOAuthApp", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	redirectURL, err := c.App.AllowOAuthAppAccessToUser(c.AppContext.Session().UserID, authRequest)

	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	w.Write([]byte(model.MapToJSON(map[string]string{"redirect": redirectURL})))
}

func deauthorizeOAuthApp(c *Context, w http.ResponseWriter, r *http.Request) {
	requestData := model.MapFromJSON(r.Body)
	clientID := requestData["client_id"]

	if !model.IsValidID(clientID) {
		c.SetInvalidParam("client_id")
		return
	}

	auditRec := c.MakeAuditRecord("deauthorizeOAuthApp", audit.Fail)
	defer c.LogAuditRec(auditRec)

	err := c.App.DeauthorizeOAuthAppForUser(c.AppContext.Session().UserID, clientID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	ReturnStatusOK(w)
}

func authorizeOAuthPage(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().ServiceSettings.EnableOAuthServiceProvider {
		err := model.NewAppError("authorizeOAuth", "api.oauth.authorize_oauth.disabled.app_error", nil, "", http.StatusNotImplemented)
		utils.RenderWebAppError(c.App.Config(), w, r, err, c.App.AsymmetricSigningKey())
		return
	}

	authRequest := &model.AuthorizeRequest{
		ResponseType: r.URL.Query().Get("response_type"),
		ClientID:     r.URL.Query().Get("client_id"),
		RedirectURI:  r.URL.Query().Get("redirect_uri"),
		Scope:        r.URL.Query().Get("scope"),
		State:        r.URL.Query().Get("state"),
	}

	loginHint := r.URL.Query().Get("login_hint")

	if err := authRequest.IsValid(); err != nil {
		utils.RenderWebError(c.App.Config(), w, r, err.StatusCode,
			url.Values{
				"type":    []string{"oauth_invalid_param"},
				"message": []string{err.Message},
			}, c.App.AsymmetricSigningKey())
		return
	}

	oauthApp, err := c.App.GetOAuthApp(authRequest.ClientID)
	if err != nil {
		utils.RenderWebAppError(c.App.Config(), w, r, err, c.App.AsymmetricSigningKey())
		return
	}

	// here we should check if the user is logged in
	if c.AppContext.Session().UserID == "" {
		if loginHint == model.UserAuthServiceSaml {
			http.Redirect(w, r, c.GetSiteURLHeader()+"/login/sso/saml?redirect_to="+url.QueryEscape(r.RequestURI), http.StatusFound)
		} else {
			http.Redirect(w, r, c.GetSiteURLHeader()+"/login?redirect_to="+url.QueryEscape(r.RequestURI), http.StatusFound)
		}
		return
	}

	if !oauthApp.IsValidRedirectURL(authRequest.RedirectURI) {
		err := model.NewAppError("authorizeOAuthPage", "api.oauth.allow_oauth.redirect_callback.app_error", nil, "", http.StatusBadRequest)
		utils.RenderWebError(c.App.Config(), w, r, err.StatusCode,
			url.Values{
				"type":    []string{"oauth_invalid_redirect_url"},
				"message": []string{i18n.T("api.oauth.allow_oauth.redirect_callback.app_error")},
			}, c.App.AsymmetricSigningKey())
		return
	}

	isAuthorized := false

	if _, err := c.App.GetPreferenceByCategoryAndNameForUser(c.AppContext.Session().UserID, model.PreferenceCategoryAuthorizedOAuthApp, authRequest.ClientID); err == nil {
		// when we support scopes we should check if the scopes match
		isAuthorized = true
	}

	// Automatically allow if the app is trusted
	if oauthApp.IsTrusted || isAuthorized {
		redirectURL, err := c.App.AllowOAuthAppAccessToUser(c.AppContext.Session().UserID, authRequest)

		if err != nil {
			utils.RenderWebAppError(c.App.Config(), w, r, err, c.App.AsymmetricSigningKey())
			return
		}

		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, max-age=31556926")

	staticDir, _ := fileutils.FindDir(model.ClientDir)
	http.ServeFile(w, r, filepath.Join(staticDir, "root.html"))
}

func getAccessToken(c *Context, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	code := r.FormValue("code")
	refreshToken := r.FormValue("refresh_token")

	grantType := r.FormValue("grant_type")
	switch grantType {
	case model.AccessTokenGrantType:
		if code == "" {
			c.Err = model.NewAppError("getAccessToken", "api.oauth.get_access_token.missing_code.app_error", nil, "", http.StatusBadRequest)
			return
		}
	case model.RefreshTokenGrantType:
		if refreshToken == "" {
			c.Err = model.NewAppError("getAccessToken", "api.oauth.get_access_token.missing_refresh_token.app_error", nil, "", http.StatusBadRequest)
			return
		}
	default:
		c.Err = model.NewAppError("getAccessToken", "api.oauth.get_access_token.bad_grant.app_error", nil, "", http.StatusBadRequest)
		return
	}

	clientID := r.FormValue("client_id")
	if !model.IsValidID(clientID) {
		c.Err = model.NewAppError("getAccessToken", "api.oauth.get_access_token.bad_client_id.app_error", nil, "", http.StatusBadRequest)
		return
	}

	secret := r.FormValue("client_secret")
	if secret == "" {
		c.Err = model.NewAppError("getAccessToken", "api.oauth.get_access_token.bad_client_secret.app_error", nil, "", http.StatusBadRequest)
		return
	}

	redirectURI := r.FormValue("redirect_uri")

	auditRec := c.MakeAuditRecord("getAccessToken", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("grant_type", grantType)
	auditRec.AddMeta("client_id", clientID)
	c.LogAudit("attempt")

	accessRsp, err := c.App.GetOAuthAccessTokenForCodeFlow(clientID, grantType, redirectURI, code, secret, refreshToken)
	if err != nil {
		c.Err = err
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	auditRec.Success()
	c.LogAudit("success")

	w.Write([]byte(accessRsp.ToJSON()))
}

func completeOAuth(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireService()
	if c.Err != nil {
		return
	}

	service := c.Params.Service

	oauthError := r.URL.Query().Get("error")
	if oauthError == "access_denied" {
		utils.RenderWebError(c.App.Config(), w, r, http.StatusTemporaryRedirect, url.Values{
			"type":    []string{"oauth_access_denied"},
			"service": []string{strings.Title(service)},
		}, c.App.AsymmetricSigningKey())
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		utils.RenderWebError(c.App.Config(), w, r, http.StatusTemporaryRedirect, url.Values{
			"type":    []string{"oauth_missing_code"},
			"service": []string{strings.Title(service)},
		}, c.App.AsymmetricSigningKey())
		return
	}

	state := r.URL.Query().Get("state")

	uri := c.GetSiteURLHeader() + "/signup/" + service + "/complete"

	body, teamID, props, tokenUser, err := c.App.AuthorizeOAuthUser(w, r, service, code, state, uri)

	action := ""
	hasRedirectURL := false
	isMobile := false
	redirectURL := ""
	if props != nil {
		action = props["action"]
		isMobile = action == model.OAuthActionMobile
		if val, ok := props["redirect_to"]; ok {
			redirectURL = val
			hasRedirectURL = redirectURL != ""
		}
	}

	renderError := func(err *model.AppError) {
		if isMobile && hasRedirectURL {
			utils.RenderMobileError(c.App.Config(), w, err, redirectURL)
		} else {
			utils.RenderWebAppError(c.App.Config(), w, r, err, c.App.AsymmetricSigningKey())
		}
	}

	if err != nil {
		err.Translate(c.AppContext.T)
		c.LogErrorByCode(err)
		renderError(err)
		return
	}

	user, err := c.App.CompleteOAuth(c.AppContext, service, body, teamID, props, tokenUser)
	if err != nil {
		err.Translate(c.AppContext.T)
		c.LogErrorByCode(err)
		renderError(err)
		return
	}

	if action == model.OAuthActionEmailToSSO {
		redirectURL = c.GetSiteURLHeader() + "/login?extra=signin_change"
	} else if action == model.OAuthActionSSOToEmail {
		redirectURL = app.GetProtocol(r) + "://" + r.Host + "/claim?email=" + url.QueryEscape(props["email"])
	} else {
		err = c.App.DoLogin(c.AppContext, w, r, user, "", isMobile, false, false)
		if err != nil {
			err.Translate(c.AppContext.T)
			mlog.Error(err.Error())
			renderError(err)
			return
		}

		// Old mobile version
		if isMobile && !hasRedirectURL {
			c.App.AttachSessionCookies(c.AppContext, w, r)
			return
		} else
		// New mobile version
		if isMobile && hasRedirectURL {
			redirectURL = utils.AppendQueryParamsToURL(redirectURL, map[string]string{
				model.SessionCookieToken: c.AppContext.Session().Token,
				model.SessionCookieCsrf:  c.AppContext.Session().GetCSRF(),
			})
			utils.RenderMobileAuthComplete(w, redirectURL)
			return
		} else { // For web
			c.App.AttachSessionCookies(c.AppContext, w, r)

			// If no redirect url is passed, get the default one
			if !hasRedirectURL {
				redirectURL = c.GetSiteURLHeader()
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func loginWithOAuth(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireService()
	if c.Err != nil {
		return
	}

	loginHint := r.URL.Query().Get("login_hint")
	redirectURL := r.URL.Query().Get("redirect_to")

	if redirectURL != "" && !utils.IsValidWebAuthRedirectURL(c.App.Config(), redirectURL) {
		c.Err = model.NewAppError("loginWithOAuth", "api.invalid_redirect_url", nil, "", http.StatusBadRequest)
		return
	}

	teamID, err := c.App.GetTeamIDFromQuery(r.URL.Query())
	if err != nil {
		c.Err = err
		return
	}

	authURL, err := c.App.GetOAuthLoginEndpoint(w, r, c.Params.Service, teamID, model.OAuthActionLogin, redirectURL, loginHint, false)
	if err != nil {
		c.Err = err
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func mobileLoginWithOAuth(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireService()
	if c.Err != nil {
		return
	}

	redirectURL := html.EscapeString(r.URL.Query().Get("redirect_to"))

	if redirectURL != "" && !utils.IsValidMobileAuthRedirectURL(c.App.Config(), redirectURL) {
		err := model.NewAppError("mobileLoginWithOAuth", "api.invalid_custom_url_scheme", nil, "", http.StatusBadRequest)
		utils.RenderMobileError(c.App.Config(), w, err, redirectURL)
		return
	}

	teamID, err := c.App.GetTeamIDFromQuery(r.URL.Query())
	if err != nil {
		c.Err = err
		return
	}

	authURL, err := c.App.GetOAuthLoginEndpoint(w, r, c.Params.Service, teamID, model.OAuthActionMobile, redirectURL, "", true)
	if err != nil {
		c.Err = err
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func signupWithOAuth(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireService()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().TeamSettings.EnableUserCreation {
		utils.RenderWebError(c.App.Config(), w, r, http.StatusBadRequest, url.Values{
			"message": []string{i18n.T("api.oauth.singup_with_oauth.disabled.app_error")},
		}, c.App.AsymmetricSigningKey())
		return
	}

	teamID, err := c.App.GetTeamIDFromQuery(r.URL.Query())
	if err != nil {
		c.Err = err
		return
	}

	authURL, err := c.App.GetOAuthSignupEndpoint(w, r, c.Params.Service, teamID)
	if err != nil {
		c.Err = err
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}
