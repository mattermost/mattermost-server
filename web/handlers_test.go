// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func handlerForHTTPErrors(c *Context, w http.ResponseWriter, r *http.Request) {
	c.Err = model.NewAppError("loginWithSaml", "api.user.saml.not_available.app_error", nil, "", http.StatusFound)
}

func TestHandlerServeHTTPErrors(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	web := New(th.App, th.Server.Router)
	handler := web.NewHandler(handlerForHTTPErrors)

	var flagtests = []struct {
		name     string
		url      string
		mobile   bool
		redirect bool
	}{
		{"redirect on desktop non-api endpoint", "/login/sso/saml", false, true},
		{"not redirect on desktop api endpoint", "/api/v4/test", false, false},
		{"not redirect on mobile non-api endpoint", "/login/sso/saml", true, false},
		{"not redirect on mobile api endpoint", "/api/v4/test", true, false},
	}

	for _, tt := range flagtests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", tt.url, nil)
			if tt.mobile {
				request.Header.Add("X-Mobile-App", "mattermost")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if tt.redirect {
				assert.Equal(t, response.Code, http.StatusFound)
			} else {
				assert.NotContains(t, response.Body.String(), "/error?message=")
			}
		})
	}
}

func handlerForHTTPSecureTransport(c *Context, w http.ResponseWriter, r *http.Request) {
}

func TestHandlerServeHTTPSecureTransport(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	mockStore := th.App.Srv().Store.(*mocks.Store)
	mockUserStore := mocks.UserStore{}
	mockUserStore.On("Count", mock.Anything).Return(int64(10), nil)
	mockPostStore := mocks.PostStore{}
	mockPostStore.On("GetMaxPostSize").Return(65535, nil)
	mockSystemStore := mocks.SystemStore{}
	mockSystemStore.On("GetByName", "UpgradedFromTE").Return(&model.System{Name: "UpgradedFromTE", Value: "false"}, nil)
	mockSystemStore.On("GetByName", "InstallationDate").Return(&model.System{Name: "InstallationDate", Value: "10"}, nil)
	mockSystemStore.On("GetByName", "FirstServerRunTimestamp").Return(&model.System{Name: "FirstServerRunTimestamp", Value: "10"}, nil)

	mockStore.On("User").Return(&mockUserStore)
	mockStore.On("Post").Return(&mockPostStore)
	mockStore.On("System").Return(&mockSystemStore)

	th.App.UpdateConfig(func(config *model.Config) {
		*config.ServiceSettings.TLSStrictTransport = true
		*config.ServiceSettings.TLSStrictTransportMaxAge = 6000
	})

	web := New(th.App, th.Server.Router)
	handler := web.NewHandler(handlerForHTTPSecureTransport)

	request := httptest.NewRequest("GET", "/api/v4/test", nil)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	header := response.Header().Get("Strict-Transport-Security")

	if header == "" {
		t.Errorf("Strict-Transport-Security expected but not existent")
	}

	if header != "max-age=6000" {
		t.Errorf("Expected max-age=6000, got %s", header)
	}

	th.App.UpdateConfig(func(config *model.Config) {
		*config.ServiceSettings.TLSStrictTransport = false
	})

	request = httptest.NewRequest("GET", "/api/v4/test", nil)

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	header = response.Header().Get("Strict-Transport-Security")

	if header != "" {
		t.Errorf("Strict-Transport-Security header is not expected, but returned")
	}
}

func handlerForCSRFToken(c *Context, w http.ResponseWriter, r *http.Request) {
}

func TestHandlerServeCSRFToken(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	session := &model.Session{
		UserID:   th.BasicUser.ID,
		CreateAt: model.GetMillis(),
		Roles:    model.SystemUserRoleID,
		IsOAuth:  false,
	}
	session.GenerateCSRF()
	th.App.SetSessionExpireInDays(session, 1)
	session, err := th.App.CreateSession(session)
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}

	web := New(th.App, th.Server.Router)

	handler := Handler{
		App:            web.app,
		HandleFunc:     handlerForCSRFToken,
		RequireSession: true,
		TrustRequester: false,
		RequireMfa:     false,
		IsStatic:       false,
	}

	cookie := &http.Cookie{
		Name:  model.SessionCookieUser,
		Value: th.BasicUser.Username,
	}
	cookie2 := &http.Cookie{
		Name:  model.SessionCookieToken,
		Value: session.Token,
	}
	cookie3 := &http.Cookie{
		Name:  model.SessionCookieCsrf,
		Value: session.GetCSRF(),
	}

	// CSRF Token Used - Success Expected

	request := httptest.NewRequest("POST", "/api/v4/test", nil)
	request.AddCookie(cookie)
	request.AddCookie(cookie2)
	request.AddCookie(cookie3)
	request.Header.Add(model.HeaderCsrfToken, session.GetCSRF())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != 200 {
		t.Errorf("Expected status 200, got %d", response.Code)
	}

	// No CSRF Token Used - Failure Expected

	request = httptest.NewRequest("POST", "/api/v4/test", nil)
	request.AddCookie(cookie)
	request.AddCookie(cookie2)
	request.AddCookie(cookie3)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != 401 {
		t.Errorf("Expected status 401, got %d", response.Code)
	}

	// Fallback Behavior Used - Success expected
	// ToDo (DSchalla) 2019/01/04: Remove once legacy CSRF Handling is removed
	th.App.UpdateConfig(func(config *model.Config) {
		*config.ServiceSettings.ExperimentalStrictCSRFEnforcement = false
	})
	request = httptest.NewRequest("POST", "/api/v4/test", nil)
	request.AddCookie(cookie)
	request.AddCookie(cookie2)
	request.AddCookie(cookie3)
	request.Header.Add(model.HeaderRequestedWith, model.HeaderRequestedWithXml)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != 200 {
		t.Errorf("Expected status 200, got %d", response.Code)
	}

	// Fallback Behavior Used with Strict Enforcement - Failure Expected
	// ToDo (DSchalla) 2019/01/04: Remove once legacy CSRF Handling is removed
	th.App.UpdateConfig(func(config *model.Config) {
		*config.ServiceSettings.ExperimentalStrictCSRFEnforcement = true
	})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != 401 {
		t.Errorf("Expected status 200, got %d", response.Code)
	}

	// Handler with RequireSession set to false

	handlerNoSession := Handler{
		App:            th.App,
		HandleFunc:     handlerForCSRFToken,
		RequireSession: false,
		TrustRequester: false,
		RequireMfa:     false,
		IsStatic:       false,
	}

	// CSRF Token Used - Success Expected

	request = httptest.NewRequest("POST", "/api/v4/test", nil)
	request.AddCookie(cookie)
	request.AddCookie(cookie2)
	request.AddCookie(cookie3)
	request.Header.Add(model.HeaderCsrfToken, session.GetCSRF())
	response = httptest.NewRecorder()
	handlerNoSession.ServeHTTP(response, request)

	if response.Code != 200 {
		t.Errorf("Expected status 200, got %d", response.Code)
	}

	// No CSRF Token Used - Failure Expected

	request = httptest.NewRequest("POST", "/api/v4/test", nil)
	request.AddCookie(cookie)
	request.AddCookie(cookie2)
	request.AddCookie(cookie3)
	response = httptest.NewRecorder()
	handlerNoSession.ServeHTTP(response, request)

	if response.Code != 401 {
		t.Errorf("Expected status 401, got %d", response.Code)
	}
}

func handlerForCSPHeader(c *Context, w http.ResponseWriter, r *http.Request) {
}

func TestHandlerServeCSPHeader(t *testing.T) {
	t.Run("non-static", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		web := New(th.App, th.Server.Router)

		handler := Handler{
			App:            web.app,
			HandleFunc:     handlerForCSPHeader,
			RequireSession: false,
			TrustRequester: false,
			RequireMfa:     false,
			IsStatic:       false,
		}

		request := httptest.NewRequest("POST", "/api/v4/test", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, 200, response.Code)
		assert.Empty(t, response.Header()["Content-Security-Policy"])
	})

	t.Run("static, without subpath", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		web := New(th.App, th.Server.Router)

		handler := Handler{
			App:            web.app,
			HandleFunc:     handlerForCSPHeader,
			RequireSession: false,
			TrustRequester: false,
			RequireMfa:     false,
			IsStatic:       true,
		}

		request := httptest.NewRequest("POST", "/", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, 200, response.Code)
		assert.Equal(t, response.Header()["Content-Security-Policy"], []string{"frame-ancestors 'self'; script-src 'self' cdn.rudderlabs.com"})
	})

	t.Run("static, with subpath", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		mockStore := th.App.Srv().Store.(*mocks.Store)
		mockUserStore := mocks.UserStore{}
		mockUserStore.On("Count", mock.Anything).Return(int64(10), nil)
		mockPostStore := mocks.PostStore{}
		mockPostStore.On("GetMaxPostSize").Return(65535, nil)
		mockSystemStore := mocks.SystemStore{}
		mockSystemStore.On("GetByName", "UpgradedFromTE").Return(&model.System{Name: "UpgradedFromTE", Value: "false"}, nil)
		mockSystemStore.On("GetByName", "InstallationDate").Return(&model.System{Name: "InstallationDate", Value: "10"}, nil)
		mockSystemStore.On("GetByName", "FirstServerRunTimestamp").Return(&model.System{Name: "FirstServerRunTimestamp", Value: "10"}, nil)

		mockStore.On("User").Return(&mockUserStore)
		mockStore.On("Post").Return(&mockPostStore)
		mockStore.On("System").Return(&mockSystemStore)

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.SiteURL = *cfg.ServiceSettings.SiteURL + "/subpath"
		})

		web := New(th.App, th.Server.Router)

		handler := Handler{
			App:            web.app,
			HandleFunc:     handlerForCSPHeader,
			RequireSession: false,
			TrustRequester: false,
			RequireMfa:     false,
			IsStatic:       true,
		}

		request := httptest.NewRequest("POST", "/", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, 200, response.Code)
		assert.Equal(t, response.Header()["Content-Security-Policy"], []string{"frame-ancestors 'self'; script-src 'self' cdn.rudderlabs.com"})

		// TODO: It's hard to unit test this now that the CSP directive is effectively
		// decided in Setup(). Circle back to this in master once the memory store is
		// merged, allowing us to mock the desired initial config to take effect in Setup().
		// assert.Contains(t, response.Header()["Content-Security-Policy"], "frame-ancestors 'self'; script-src 'self' cdn.rudderlabs.com 'sha256-tPOjw+tkVs9axL78ZwGtYl975dtyPHB6LYKAO2R3gR4='")

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.SiteURL = *cfg.ServiceSettings.SiteURL + "/subpath2"
		})

		request = httptest.NewRequest("POST", "/", nil)
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, 200, response.Code)
		assert.Equal(t, response.Header()["Content-Security-Policy"], []string{"frame-ancestors 'self'; script-src 'self' cdn.rudderlabs.com"})
		// TODO: See above.
		// assert.Contains(t, response.Header()["Content-Security-Policy"], "frame-ancestors 'self'; script-src 'self' cdn.rudderlabs.com 'sha256-tPOjw+tkVs9axL78ZwGtYl975dtyPHB6LYKAO2R3gR4='", "csp header incorrectly changed after subpath changed")
	})
}

func TestHandlerServeInvalidToken(t *testing.T) {
	testCases := []struct {
		Description                   string
		SiteURL                       string
		ExpectedSetCookieHeaderRegexp string
	}{
		{"no subpath", "http://localhost:8065", "^MMAUTHTOKEN=; Path=/"},
		{"subpath", "http://localhost:8065/subpath", "^MMAUTHTOKEN=; Path=/subpath"},
	}

	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			th := Setup(t)
			defer th.TearDown()

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ServiceSettings.SiteURL = tc.SiteURL
			})

			web := New(th.App, th.Server.Router)

			handler := Handler{
				App:            web.app,
				HandleFunc:     handlerForCSRFToken,
				RequireSession: true,
				TrustRequester: false,
				RequireMfa:     false,
				IsStatic:       false,
			}

			cookie := &http.Cookie{
				Name:  model.SessionCookieToken,
				Value: "invalid",
			}

			request := httptest.NewRequest("POST", "/api/v4/test", nil)
			request.AddCookie(cookie)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			require.Equal(t, http.StatusUnauthorized, response.Code)

			cookies := response.Header().Get("Set-Cookie")
			assert.Regexp(t, tc.ExpectedSetCookieHeaderRegexp, cookies)
		})
	}
}

func TestCheckCSRFToken(t *testing.T) {
	t.Run("should allow a POST request with a valid CSRF token header", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		h := &Handler{
			RequireSession: true,
			TrustRequester: false,
		}

		token := "token"
		tokenLocation := app.TokenLocationCookie

		c := &Context{
			App:        th.App,
			AppContext: th.Context,
		}
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		r.Header.Set(model.HeaderCsrfToken, token)
		session := &model.Session{
			Props: map[string]string{
				"csrf": token,
			},
		}

		checked, passed := h.checkCSRFToken(c, r, token, tokenLocation, session)

		assert.True(t, checked)
		assert.True(t, passed)
		assert.Nil(t, c.Err)
	})

	t.Run("should allow a POST request with an X-Requested-With header", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		h := &Handler{
			RequireSession: true,
			TrustRequester: false,
		}

		token := "token"
		tokenLocation := app.TokenLocationCookie

		c := &Context{
			App:        th.App,
			Logger:     th.App.Log(),
			AppContext: th.Context,
		}
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		r.Header.Set(model.HeaderRequestedWith, model.HeaderRequestedWithXml)
		session := &model.Session{
			Props: map[string]string{
				"csrf": token,
			},
		}

		checked, passed := h.checkCSRFToken(c, r, token, tokenLocation, session)

		assert.True(t, checked)
		assert.True(t, passed)
		assert.Nil(t, c.Err)
	})

	t.Run("should not allow a POST request with an X-Requested-With header with strict CSRF enforcement enabled", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		mockStore := th.App.Srv().Store.(*mocks.Store)
		mockUserStore := mocks.UserStore{}
		mockUserStore.On("Count", mock.Anything).Return(int64(10), nil)
		mockPostStore := mocks.PostStore{}
		mockPostStore.On("GetMaxPostSize").Return(65535, nil)
		mockSystemStore := mocks.SystemStore{}
		mockSystemStore.On("GetByName", "UpgradedFromTE").Return(&model.System{Name: "UpgradedFromTE", Value: "false"}, nil)
		mockSystemStore.On("GetByName", "InstallationDate").Return(&model.System{Name: "InstallationDate", Value: "10"}, nil)
		mockSystemStore.On("GetByName", "FirstServerRunTimestamp").Return(&model.System{Name: "FirstServerRunTimestamp", Value: "10"}, nil)

		mockStore.On("User").Return(&mockUserStore)
		mockStore.On("Post").Return(&mockPostStore)
		mockStore.On("System").Return(&mockSystemStore)

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.ExperimentalStrictCSRFEnforcement = true
		})

		h := &Handler{
			RequireSession: true,
			TrustRequester: false,
		}

		token := "token"
		tokenLocation := app.TokenLocationCookie

		c := &Context{
			App:        th.App,
			Logger:     th.App.Log(),
			AppContext: th.Context,
		}
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		r.Header.Set(model.HeaderRequestedWith, model.HeaderRequestedWithXml)
		session := &model.Session{
			Props: map[string]string{
				"csrf": token,
			},
		}

		checked, passed := h.checkCSRFToken(c, r, token, tokenLocation, session)

		assert.True(t, checked)
		assert.False(t, passed)
		assert.NotNil(t, c.Err)
	})

	t.Run("should not allow a POST request without either header", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		h := &Handler{
			RequireSession: true,
			TrustRequester: false,
		}

		token := "token"
		tokenLocation := app.TokenLocationCookie

		c := &Context{
			App:        th.App,
			AppContext: th.Context,
		}
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		session := &model.Session{
			Props: map[string]string{
				"csrf": token,
			},
		}

		checked, passed := h.checkCSRFToken(c, r, token, tokenLocation, session)

		assert.True(t, checked)
		assert.False(t, passed)
		assert.NotNil(t, c.Err)
	})

	t.Run("should not check GET requests", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		h := &Handler{
			RequireSession: true,
			TrustRequester: false,
		}

		token := "token"
		tokenLocation := app.TokenLocationCookie

		c := &Context{
			App:        th.App,
			AppContext: th.Context,
		}
		r, _ := http.NewRequest(http.MethodGet, "", nil)
		session := &model.Session{
			Props: map[string]string{
				"csrf": token,
			},
		}

		checked, passed := h.checkCSRFToken(c, r, token, tokenLocation, session)

		assert.False(t, checked)
		assert.False(t, passed)
		assert.Nil(t, c.Err)
	})

	t.Run("should not check a request passing the auth token in a header", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		h := &Handler{
			RequireSession: true,
			TrustRequester: false,
		}

		token := "token"
		tokenLocation := app.TokenLocationHeader

		c := &Context{
			App:        th.App,
			AppContext: th.Context,
		}
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		session := &model.Session{
			Props: map[string]string{
				"csrf": token,
			},
		}

		checked, passed := h.checkCSRFToken(c, r, token, tokenLocation, session)

		assert.False(t, checked)
		assert.False(t, passed)
		assert.Nil(t, c.Err)
	})

	t.Run("should not check a request passing a nil session", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		h := &Handler{
			RequireSession: false,
			TrustRequester: false,
		}

		token := "token"
		tokenLocation := app.TokenLocationCookie

		c := &Context{
			App:        th.App,
			AppContext: th.Context,
		}
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		r.Header.Set(model.HeaderCsrfToken, token)

		checked, passed := h.checkCSRFToken(c, r, token, tokenLocation, nil)

		assert.False(t, checked)
		assert.False(t, passed)
		assert.Nil(t, c.Err)
	})

	t.Run("should check requests for handlers that don't require a session but have one", func(t *testing.T) {
		th := SetupWithStoreMock(t)
		defer th.TearDown()

		h := &Handler{
			RequireSession: false,
			TrustRequester: false,
		}

		token := "token"
		tokenLocation := app.TokenLocationCookie

		c := &Context{
			App:        th.App,
			AppContext: th.Context,
		}
		r, _ := http.NewRequest(http.MethodPost, "", nil)
		r.Header.Set(model.HeaderCsrfToken, token)
		session := &model.Session{
			Props: map[string]string{
				"csrf": token,
			},
		}

		checked, passed := h.checkCSRFToken(c, r, token, tokenLocation, session)

		assert.True(t, checked)
		assert.True(t, passed)
		assert.Nil(t, c.Err)
	})
}
