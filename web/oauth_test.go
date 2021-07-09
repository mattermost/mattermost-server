// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func TestOAuthComplete_AccessDenied(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	c := &Context{
		App: th.App,
		Params: &Params{
			Service: "TestService",
		},
	}
	responseWriter := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, th.App.GetSiteURL()+"/signup/TestService/complete?error=access_denied", nil)

	completeOAuth(c, responseWriter, request)

	response := responseWriter.Result()

	assert.Equal(t, http.StatusTemporaryRedirect, response.StatusCode)

	location, _ := url.Parse(response.Header.Get("Location"))
	assert.Equal(t, "oauth_access_denied", location.Query().Get("type"))
	assert.Equal(t, "TestService", location.Query().Get("service"))
}

func TestAuthorizeOAuthApp(t *testing.T) {
	th := Setup(t).InitBasic()
	th.Login(APIClient, th.SystemAdminUser)
	defer th.TearDown()

	enableOAuth := *th.App.Config().ServiceSettings.EnableOAuthServiceProvider
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOAuthServiceProvider = enableOAuth })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOAuthServiceProvider = true })

	oapp := &model.OAuthApp{
		Name:         GenerateTestAppName(),
		Homepage:     "https://nowhere.com",
		Description:  "test",
		CallbackURLs: []string{"https://nowhere.com"},
		CreatorID:    th.SystemAdminUser.ID,
	}

	rapp, appErr := th.App.CreateOAuthApp(oapp)
	require.Nil(t, appErr)

	authRequest := &model.AuthorizeRequest{
		ResponseType: model.AuthCodeResponseType,
		ClientID:     rapp.ID,
		RedirectURI:  rapp.CallbackURLs[0],
		Scope:        "",
		State:        "123",
	}

	// Test auth code flow
	ruri, resp := APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)

	require.NotEmpty(t, ruri, "redirect url should be set")

	ru, _ := url.Parse(ruri)
	require.NotNil(t, ru, "redirect url unparseable")
	require.NotEmpty(t, ru.Query().Get("code"), "authorization code not returned")
	require.Equal(t, ru.Query().Get("state"), authRequest.State, "returned state doesn't match")

	// Test implicit flow
	authRequest.ResponseType = model.ImplicitResponseType
	ruri, resp = APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)
	require.False(t, ruri == "", "redirect url should be set")

	ru, _ = url.Parse(ruri)
	require.NotNil(t, ru, "redirect url unparseable")
	values, err := url.ParseQuery(ru.Fragment)
	require.NoError(t, err)
	assert.False(t, values.Get("access_token") == "", "access_token not returned")
	assert.Equal(t, authRequest.State, values.Get("state"), "returned state doesn't match")

	oldToken := APIClient.AuthToken
	APIClient.AuthToken = values.Get("access_token")
	_, resp = APIClient.AuthorizeOAuthApp(authRequest)
	CheckForbiddenStatus(t, resp)

	APIClient.AuthToken = oldToken

	authRequest.RedirectURI = ""
	_, resp = APIClient.AuthorizeOAuthApp(authRequest)
	CheckBadRequestStatus(t, resp)

	authRequest.RedirectURI = "http://somewhereelse.com"
	_, resp = APIClient.AuthorizeOAuthApp(authRequest)
	CheckBadRequestStatus(t, resp)

	authRequest.RedirectURI = rapp.CallbackURLs[0]
	authRequest.ResponseType = ""
	_, resp = APIClient.AuthorizeOAuthApp(authRequest)
	CheckBadRequestStatus(t, resp)

	authRequest.ResponseType = model.AuthCodeResponseType
	authRequest.ClientID = ""
	_, resp = APIClient.AuthorizeOAuthApp(authRequest)
	CheckBadRequestStatus(t, resp)

	authRequest.ClientID = model.NewID()
	_, resp = APIClient.AuthorizeOAuthApp(authRequest)
	CheckNotFoundStatus(t, resp)
}

func TestNilAuthorizeOAuthApp(t *testing.T) {
	th := Setup(t).InitBasic()
	th.Login(APIClient, th.SystemAdminUser)
	defer th.TearDown()

	_, resp := APIClient.AuthorizeOAuthApp(nil)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "api.context.invalid_body_param.app_error", resp.Error.ID)
}

func TestDeauthorizeOAuthApp(t *testing.T) {
	th := Setup(t).InitBasic()
	th.Login(APIClient, th.SystemAdminUser)
	defer th.TearDown()

	enableOAuth := th.App.Config().ServiceSettings.EnableOAuthServiceProvider
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableOAuthServiceProvider = enableOAuth })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOAuthServiceProvider = true })

	oapp := &model.OAuthApp{
		Name:         GenerateTestAppName(),
		Homepage:     "https://nowhere.com",
		Description:  "test",
		CallbackURLs: []string{"https://nowhere.com"},
		CreatorID:    th.SystemAdminUser.ID,
	}

	rapp, appErr := th.App.CreateOAuthApp(oapp)
	require.Nil(t, appErr)

	authRequest := &model.AuthorizeRequest{
		ResponseType: model.AuthCodeResponseType,
		ClientID:     rapp.ID,
		RedirectURI:  rapp.CallbackURLs[0],
		Scope:        "",
		State:        "123",
	}

	_, resp := APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)

	pass, resp := APIClient.DeauthorizeOAuthApp(rapp.ID)
	require.Nil(t, resp.Error)

	require.True(t, pass, "should have passed")

	_, resp = APIClient.DeauthorizeOAuthApp("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = APIClient.DeauthorizeOAuthApp(model.NewID())
	require.Nil(t, resp.Error)

	th.Logout(APIClient)
	_, resp = APIClient.DeauthorizeOAuthApp(rapp.ID)
	CheckUnauthorizedStatus(t, resp)
}

func TestOAuthAccessToken(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	th := Setup(t).InitBasic()
	th.Login(APIClient, th.SystemAdminUser)
	defer th.TearDown()

	enableOAuth := th.App.Config().ServiceSettings.EnableOAuthServiceProvider
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableOAuthServiceProvider = enableOAuth })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOAuthServiceProvider = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageOAuth.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionManageOAuth.ID, model.SystemUserRoleID)

	oauthApp := &model.OAuthApp{
		Name:         "TestApp5" + model.NewID(),
		Homepage:     "https://nowhere.com",
		Description:  "test",
		CallbackURLs: []string{"https://nowhere.com"},
		CreatorID:    th.SystemAdminUser.ID,
	}
	oauthApp, appErr := th.App.CreateOAuthApp(oauthApp)
	require.Nil(t, appErr)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOAuthServiceProvider = false })
	data := url.Values{"grant_type": []string{"junk"}, "client_id": []string{"12345678901234567890123456"}, "client_secret": []string{"12345678901234567890123456"}, "code": []string{"junk"}, "redirect_uri": []string{oauthApp.CallbackURLs[0]}}

	_, resp := APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - oauth providing turned off - response status code: %v", resp.StatusCode)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOAuthServiceProvider = true })

	authRequest := &model.AuthorizeRequest{
		ResponseType: model.AuthCodeResponseType,
		ClientID:     oauthApp.ID,
		RedirectURI:  oauthApp.CallbackURLs[0],
		Scope:        "all",
		State:        "123",
	}

	redirect, resp := APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)
	rurl, _ := url.Parse(redirect)

	APIClient.Logout()

	data = url.Values{"grant_type": []string{"junk"}, "client_id": []string{oauthApp.ID}, "client_secret": []string{oauthApp.ClientSecret}, "code": []string{rurl.Query().Get("code")}, "redirect_uri": []string{oauthApp.CallbackURLs[0]}}

	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - bad grant type")

	data.Set("grant_type", model.AccessTokenGrantType)
	data.Set("client_id", "")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - missing client id")

	data.Set("client_id", "junk")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - bad client id")

	data.Set("client_id", oauthApp.ID)
	data.Set("client_secret", "")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - missing client secret")

	data.Set("client_secret", "junk")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - bad client secret")

	data.Set("client_secret", oauthApp.ClientSecret)
	data.Set("code", "")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - missing code")

	data.Set("code", "junk")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - bad code")

	data.Set("code", rurl.Query().Get("code"))
	data.Set("redirect_uri", "junk")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - non-matching redirect uri")

	// reset data for successful request
	data.Set("grant_type", model.AccessTokenGrantType)
	data.Set("client_id", oauthApp.ID)
	data.Set("client_secret", oauthApp.ClientSecret)
	data.Set("code", rurl.Query().Get("code"))
	data.Set("redirect_uri", oauthApp.CallbackURLs[0])

	token := ""
	refreshToken := ""
	rsp, resp := APIClient.GetOAuthAccessToken(data)
	require.Nil(t, resp.Error)
	require.NotEmpty(t, rsp.AccessToken, "access token not returned")
	require.NotEmpty(t, rsp.RefreshToken, "refresh token not returned")
	token, refreshToken = rsp.AccessToken, rsp.RefreshToken
	require.Equal(t, rsp.TokenType, model.AccessTokenType, "access token type incorrect")

	_, err := APIClient.DoAPIGet("/oauth_test", "")
	require.Nil(t, err)

	APIClient.SetOAuthToken("")
	_, err = APIClient.DoAPIGet("/oauth_test", "")
	require.NotNil(t, err, "should have failed - no access token provided")

	APIClient.SetOAuthToken("badtoken")
	_, err = APIClient.DoAPIGet("/oauth_test", "")
	require.NotNil(t, err, "should have failed - bad token provided")

	APIClient.SetOAuthToken(token)
	_, err = APIClient.DoAPIGet("/oauth_test", "")
	require.Nil(t, err)

	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "should have failed - tried to reuse auth code")

	data.Set("grant_type", model.RefreshTokenGrantType)
	data.Set("client_id", oauthApp.ID)
	data.Set("client_secret", oauthApp.ClientSecret)
	data.Set("refresh_token", "")
	data.Set("redirect_uri", oauthApp.CallbackURLs[0])
	data.Del("code")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "Should have failed - refresh token empty")

	data.Set("refresh_token", refreshToken)
	rsp, resp = APIClient.GetOAuthAccessToken(data)
	require.Nil(t, resp.Error)
	require.NotEmpty(t, rsp.AccessToken, "access token not returned")
	require.NotEmpty(t, rsp.RefreshToken, "refresh token not returned")
	require.NotEqual(t, rsp.RefreshToken, refreshToken, "refresh token did not update")
	require.Equal(t, rsp.TokenType, model.AccessTokenType, "access token type incorrect")

	APIClient.SetOAuthToken(rsp.AccessToken)
	_, err = APIClient.DoAPIGet("/oauth_test", "")
	require.Nil(t, err)

	data.Set("refresh_token", rsp.RefreshToken)
	rsp, resp = APIClient.GetOAuthAccessToken(data)
	require.Nil(t, resp.Error)
	require.NotEmpty(t, rsp.AccessToken, "access token not returned")
	require.NotEmpty(t, rsp.RefreshToken, "refresh token not returned")
	require.NotEqual(t, rsp.RefreshToken, refreshToken, "refresh token did not update")
	require.Equal(t, rsp.TokenType, model.AccessTokenType, "access token type incorrect")

	APIClient.SetOAuthToken(rsp.AccessToken)
	_, err = APIClient.DoAPIGet("/oauth_test", "")
	require.Nil(t, err)

	authData := &model.AuthData{ClientID: oauthApp.ID, RedirectURI: oauthApp.CallbackURLs[0], UserID: th.BasicUser.ID, Code: model.NewID(), ExpiresIn: -1}
	_, nErr := th.App.Srv().Store.OAuth().SaveAuthData(authData)
	require.NoError(t, nErr)

	data.Set("grant_type", model.AccessTokenGrantType)
	data.Set("client_id", oauthApp.ID)
	data.Set("client_secret", oauthApp.ClientSecret)
	data.Set("redirect_uri", oauthApp.CallbackURLs[0])
	data.Set("code", authData.Code)
	data.Del("refresh_token")
	_, resp = APIClient.GetOAuthAccessToken(data)
	require.NotNil(t, resp.Error, "Should have failed - code is expired")

	APIClient.ClearOAuthToken()
}

func TestMobileLoginWithOAuth(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	c := &Context{
		App:        th.App,
		AppContext: &request.Context{},
		Params: &Params{
			Service: "gitlab",
		},
	}

	var siteURL = "http://localhost:8065"
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.SiteURL = siteURL })

	translationFunc := i18n.GetUserTranslations("en")
	c.AppContext.SetT(translationFunc)
	buffer := &bytes.Buffer{}
	c.Logger = mlog.NewTestingLogger(t, buffer)
	provider := &MattermostTestProvider{}
	einterfaces.RegisterOAuthProvider(model.ServiceGitlab, provider)

	t.Run("Should include redirect URL in the output when valid URL Scheme is passed", func(t *testing.T) {
		responseWriter := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, th.App.GetSiteURL()+"/oauth/gitlab/mobile_login?redirect_to="+url.QueryEscape("randomScheme://"), nil)
		mobileLoginWithOAuth(c, responseWriter, request)
		assert.Contains(t, responseWriter.Body.String(), "randomScheme://")
		assert.NotContains(t, responseWriter.Body.String(), siteURL)
	})

	t.Run("Should not include the redirect URL consisting of javascript protocol", func(t *testing.T) {
		responseWriter := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, th.App.GetSiteURL()+"/oauth/gitlab/mobile_login?redirect_to="+url.QueryEscape("javascript:alert('hello')"), nil)
		mobileLoginWithOAuth(c, responseWriter, request)
		assert.NotContains(t, responseWriter.Body.String(), "javascript:alert('hello')")
		assert.Contains(t, responseWriter.Body.String(), siteURL)
	})

	t.Run("Should not include the redirect URL consisting of javascript protocol in mixed case", func(t *testing.T) {
		responseWriter := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, th.App.GetSiteURL()+"/oauth/gitlab/mobile_login?redirect_to="+url.QueryEscape("JaVasCript:alert('hello')"), nil)
		mobileLoginWithOAuth(c, responseWriter, request)
		assert.NotContains(t, responseWriter.Body.String(), "JaVasCript:alert('hello')")
		assert.Contains(t, responseWriter.Body.String(), siteURL)
	})
}

func TestOAuthComplete(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	th := Setup(t).InitBasic()
	th.Login(APIClient, th.SystemAdminUser)
	defer th.TearDown()

	gitLabSettingsEnable := th.App.Config().GitLabSettings.Enable
	gitLabSettingsAuthEndpoint := th.App.Config().GitLabSettings.AuthEndpoint
	gitLabSettingsID := th.App.Config().GitLabSettings.ID
	gitLabSettingsSecret := th.App.Config().GitLabSettings.Secret
	gitLabSettingsTokenEndpoint := th.App.Config().GitLabSettings.TokenEndpoint
	gitLabSettingsUserAPIEndpoint := th.App.Config().GitLabSettings.UserAPIEndpoint
	enableOAuthServiceProvider := th.App.Config().ServiceSettings.EnableOAuthServiceProvider
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.GitLabSettings.Enable = gitLabSettingsEnable })
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.GitLabSettings.AuthEndpoint = gitLabSettingsAuthEndpoint })
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.GitLabSettings.ID = gitLabSettingsID })
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.GitLabSettings.Secret = gitLabSettingsSecret })
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.GitLabSettings.TokenEndpoint = gitLabSettingsTokenEndpoint })
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.GitLabSettings.UserAPIEndpoint = gitLabSettingsUserAPIEndpoint })
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.EnableOAuthServiceProvider = enableOAuthServiceProvider })
	}()

	r, err := HTTPGet(APIClient.URL+"/login/gitlab/complete?code=123", APIClient.HTTPClient, "", true)
	assert.NotNil(t, err)
	closeBody(r)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.Enable = true })
	r, err = HTTPGet(APIClient.URL+"/login/gitlab/complete?code=123&state=!#$#F@#Yˆ&~ñ", APIClient.HTTPClient, "", true)
	assert.NotNil(t, err)
	closeBody(r)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.AuthEndpoint = APIClient.URL + "/oauth/authorize" })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.ID = model.NewID() })

	stateProps := map[string]string{}
	stateProps["action"] = model.OAuthActionLogin
	stateProps["team_id"] = th.BasicTeam.ID
	stateProps["redirect_to"] = *th.App.Config().GitLabSettings.AuthEndpoint

	state := base64.StdEncoding.EncodeToString([]byte(model.MapToJSON(stateProps)))
	r, err = HTTPGet(APIClient.URL+"/login/gitlab/complete?code=123&state="+url.QueryEscape(state), APIClient.HTTPClient, "", true)
	assert.NotNil(t, err)
	closeBody(r)

	stateProps["hash"] = utils.HashSha256(*th.App.Config().GitLabSettings.ID)
	state = base64.StdEncoding.EncodeToString([]byte(model.MapToJSON(stateProps)))
	r, err = HTTPGet(APIClient.URL+"/login/gitlab/complete?code=123&state="+url.QueryEscape(state), APIClient.HTTPClient, "", true)
	assert.NotNil(t, err)
	closeBody(r)

	// We are going to use mattermost as the provider emulating gitlab
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOAuthServiceProvider = true })

	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()
	th.AddPermissionToRole(model.PermissionManageOAuth.ID, model.TeamUserRoleID)
	th.AddPermissionToRole(model.PermissionManageOAuth.ID, model.SystemUserRoleID)

	oauthApp := &model.OAuthApp{
		Name:        "TestApp5" + model.NewID(),
		Homepage:    "https://nowhere.com",
		Description: "test",
		CallbackURLs: []string{
			APIClient.URL + "/signup/" + model.ServiceGitlab + "/complete",
			APIClient.URL + "/login/" + model.ServiceGitlab + "/complete",
		},
		CreatorID: th.SystemAdminUser.ID,
		IsTrusted: true,
	}
	oauthApp, appErr := th.App.CreateOAuthApp(oauthApp)
	require.Nil(t, appErr)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.ID = oauthApp.ID })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.Secret = oauthApp.ClientSecret })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.AuthEndpoint = APIClient.URL + "/oauth/authorize" })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.TokenEndpoint = APIClient.URL + "/oauth/access_token" })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.UserAPIEndpoint = APIClient.APIURL + "/users/me" })

	provider := &MattermostTestProvider{}

	authRequest := &model.AuthorizeRequest{
		ResponseType: model.AuthCodeResponseType,
		ClientID:     oauthApp.ID,
		RedirectURI:  oauthApp.CallbackURLs[0],
		Scope:        "all",
		State:        "123",
	}

	redirect, resp := APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)
	rurl, _ := url.Parse(redirect)

	code := rurl.Query().Get("code")
	stateProps["action"] = model.OAuthActionEmailToSSO
	delete(stateProps, "team_id")
	stateProps["redirect_to"] = *th.App.Config().GitLabSettings.AuthEndpoint
	stateProps["hash"] = utils.HashSha256(*th.App.Config().GitLabSettings.ID)
	stateProps["redirect_to"] = "/oauth/authorize"
	state = base64.StdEncoding.EncodeToString([]byte(model.MapToJSON(stateProps)))
	r, err = HTTPGet(APIClient.URL+"/login/"+model.ServiceGitlab+"/complete?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), APIClient.HTTPClient, "", false)
	if err == nil {
		closeBody(r)
	}

	einterfaces.RegisterOAuthProvider(model.ServiceGitlab, provider)

	redirect, resp = APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)
	rurl, _ = url.Parse(redirect)

	code = rurl.Query().Get("code")
	r, err = HTTPGet(APIClient.URL+"/login/"+model.ServiceGitlab+"/complete?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), APIClient.HTTPClient, "", false)
	if err == nil {
		closeBody(r)
	}

	_, nErr := th.App.Srv().Store.User().UpdateAuthData(
		th.BasicUser.ID, model.ServiceGitlab, &th.BasicUser.Email, th.BasicUser.Email, true)
	require.NoError(t, nErr)

	redirect, resp = APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)
	rurl, _ = url.Parse(redirect)

	code = rurl.Query().Get("code")
	stateProps["action"] = model.OAuthActionLogin
	state = base64.StdEncoding.EncodeToString([]byte(model.MapToJSON(stateProps)))
	if r, err := HTTPGet(APIClient.URL+"/login/"+model.ServiceGitlab+"/complete?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), APIClient.HTTPClient, "", false); err == nil {
		closeBody(r)
	}

	redirect, resp = APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)
	rurl, _ = url.Parse(redirect)

	code = rurl.Query().Get("code")
	delete(stateProps, "action")
	state = base64.StdEncoding.EncodeToString([]byte(model.MapToJSON(stateProps)))
	if r, err := HTTPGet(APIClient.URL+"/login/"+model.ServiceGitlab+"/complete?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), APIClient.HTTPClient, "", false); err == nil {
		closeBody(r)
	}

	redirect, resp = APIClient.AuthorizeOAuthApp(authRequest)
	require.Nil(t, resp.Error)
	rurl, _ = url.Parse(redirect)

	code = rurl.Query().Get("code")
	stateProps["action"] = model.OAuthActionSignup
	state = base64.StdEncoding.EncodeToString([]byte(model.MapToJSON(stateProps)))
	if r, err := HTTPGet(APIClient.URL+"/login/"+model.ServiceGitlab+"/complete?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), APIClient.HTTPClient, "", false); err == nil {
		closeBody(r)
	}
}

func TestOAuthComplete_ErrorMessages(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	c := &Context{
		App:        th.App,
		AppContext: &request.Context{},
		Params: &Params{
			Service: "gitlab",
		},
	}

	translationFunc := i18n.GetUserTranslations("en")
	c.AppContext.SetT(translationFunc)
	buffer := &bytes.Buffer{}
	c.Logger = mlog.NewTestingLogger(t, buffer)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GitLabSettings.Enable = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableOAuthServiceProvider = true })
	provider := &MattermostTestProvider{}
	einterfaces.RegisterOAuthProvider(model.ServiceGitlab, provider)

	responseWriter := httptest.NewRecorder()

	// Renders for web & mobile app with webview
	request, _ := http.NewRequest(http.MethodGet, th.App.GetSiteURL()+"/signup/gitlab/complete?code=1234", nil)

	completeOAuth(c, responseWriter, request)
	assert.Contains(t, responseWriter.Body.String(), "<!-- web error message -->")

	// Renders for mobile app with redirect url
	stateProps := map[string]string{}
	stateProps["action"] = model.OAuthActionMobile
	stateProps["redirect_to"] = th.App.Config().NativeAppSettings.AppCustomURLSchemes[0]
	state := base64.StdEncoding.EncodeToString([]byte(model.MapToJSON(stateProps)))
	request2, _ := http.NewRequest(http.MethodGet, th.App.GetSiteURL()+"/signup/gitlab/complete?code=1234&state="+url.QueryEscape(state), nil)

	completeOAuth(c, responseWriter, request2)
	assert.Contains(t, responseWriter.Body.String(), "<!-- mobile app message -->")
}

func HTTPGet(url string, httpClient *http.Client, authToken string, followRedirect bool) (*http.Response, *model.AppError) {
	rq, _ := http.NewRequest("GET", url, nil)
	rq.Close = true

	if authToken != "" {
		rq.Header.Set(model.HeaderAuth, authToken)
	}

	if !followRedirect {
		httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	if rp, err := httpClient.Do(rq); err != nil {
		return nil, model.NewAppError(url, "model.client.connecting.app_error", nil, err.Error(), 0)
	} else if rp.StatusCode == 304 {
		return rp, nil
	} else if rp.StatusCode == 307 {
		return rp, nil
	} else if rp.StatusCode >= 300 {
		defer closeBody(rp)
		return rp, model.AppErrorFromJSON(rp.Body)
	} else {
		return rp, nil
	}
}

func closeBody(r *http.Response) {
	if r != nil && r.Body != nil {
		ioutil.ReadAll(r.Body)
		r.Body.Close()
	}
}

type MattermostTestProvider struct {
}

func (m *MattermostTestProvider) GetUserFromJSON(data io.Reader, tokenUser *model.User) (*model.User, error) {
	user := model.UserFromJSON(data)
	user.AuthData = &user.Email
	return user, nil
}

func (m *MattermostTestProvider) GetSSOSettings(config *model.Config, service string) (*model.SSOSettings, error) {
	return &config.GitLabSettings, nil
}

func (m *MattermostTestProvider) GetUserFromIDToken(token string) (*model.User, error) {
	return nil, nil
}

func (m *MattermostTestProvider) IsSameUser(dbUser, oauthUser *model.User) bool {
	return dbUser.AuthData == oauthUser.AuthData
}

func GenerateTestAppName() string {
	return "fakeoauthapp" + model.NewRandomString(10)
}

func checkHTTPStatus(t *testing.T, resp *model.Response, expectedStatus int) {
	t.Helper()

	require.NotNil(t, resp, "Unexpected nil response, expected http:%v, expectError:%v)", expectedStatus, true)

	require.NotNil(t, resp.Error, "Expected a non-nil error and http status:%v, got nil, %v", expectedStatus, resp.StatusCode)

	require.Equal(t, resp.StatusCode, expectedStatus, "Expected http status:%v, got %v (err: %q)", expectedStatus, resp.StatusCode, resp.Error)
}

func CheckForbiddenStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusForbidden)
}

func CheckUnauthorizedStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusUnauthorized)
}

func CheckNotFoundStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusNotFound)
}

func CheckBadRequestStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusBadRequest)
}

func (th *TestHelper) Login(client *model.Client4, user *model.User) {
	session := &model.Session{
		UserID:  user.ID,
		Roles:   user.GetRawRoles(),
		IsOAuth: false,
	}
	session, _ = th.App.CreateSession(session)
	client.AuthToken = session.Token
	client.AuthType = model.HeaderBearer
}

func (th *TestHelper) Logout(client *model.Client4) {
	client.AuthToken = ""
}

func (th *TestHelper) SaveDefaultRolePermissions() map[string][]string {
	utils.DisableDebugLogForTest()

	results := make(map[string][]string)

	for _, roleName := range []string{
		"system_user",
		"system_admin",
		"team_user",
		"team_admin",
		"channel_user",
		"channel_admin",
	} {
		role, err1 := th.App.GetRoleByName(context.Background(), roleName)
		if err1 != nil {
			utils.EnableDebugLogForTest()
			panic(err1)
		}

		results[roleName] = role.Permissions
	}

	utils.EnableDebugLogForTest()
	return results
}

func (th *TestHelper) RestoreDefaultRolePermissions(data map[string][]string) {
	utils.DisableDebugLogForTest()

	for roleName, permissions := range data {
		role, err1 := th.App.GetRoleByName(context.Background(), roleName)
		if err1 != nil {
			utils.EnableDebugLogForTest()
			panic(err1)
		}

		if strings.Join(role.Permissions, " ") == strings.Join(permissions, " ") {
			continue
		}

		role.Permissions = permissions

		_, err2 := th.App.UpdateRole(role)
		if err2 != nil {
			utils.EnableDebugLogForTest()
			panic(err2)
		}
	}

	utils.EnableDebugLogForTest()
}

// func (th *TestHelper) RemovePermissionFromRole(permission string, roleName string) {
// 	utils.DisableDebugLogForTest()

// 	role, err1 := th.App.GetRoleByName(roleName)
// 	if err1 != nil {
// 		utils.EnableDebugLogForTest()
// 		panic(err1)
// 	}

// 	var newPermissions []string
// 	for _, p := range role.Permissions {
// 		if p != permission {
// 			newPermissions = append(newPermissions, p)
// 		}
// 	}

// 	if strings.Join(role.Permissions, " ") == strings.Join(newPermissions, " ") {
// 		utils.EnableDebugLogForTest()
// 		return
// 	}

// 	role.Permissions = newPermissions

// 	_, err2 := th.App.UpdateRole(role)
// 	if err2 != nil {
// 		utils.EnableDebugLogForTest()
// 		panic(err2)
// 	}

// 	utils.EnableDebugLogForTest()
// }

func (th *TestHelper) AddPermissionToRole(permission string, roleName string) {
	utils.DisableDebugLogForTest()

	role, err1 := th.App.GetRoleByName(context.Background(), roleName)
	if err1 != nil {
		utils.EnableDebugLogForTest()
		panic(err1)
	}

	for _, existingPermission := range role.Permissions {
		if existingPermission == permission {
			utils.EnableDebugLogForTest()
			return
		}
	}

	role.Permissions = append(role.Permissions, permission)

	_, err2 := th.App.UpdateRole(role)
	if err2 != nil {
		utils.EnableDebugLogForTest()
		panic(err2)
	}

	utils.EnableDebugLogForTest()
}
