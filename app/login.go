// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/avct/uasurfer"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

const cwsTokenEnv = "CWS_CLOUD_TOKEN"

func (a *App) CheckForClientSideCert(r *http.Request) (string, string, string) {
	pem := r.Header.Get("X-SSL-Client-Cert")                // mapped to $ssl_client_cert from nginx
	subject := r.Header.Get("X-SSL-Client-Cert-Subject-DN") // mapped to $ssl_client_s_dn from nginx
	email := ""

	if subject != "" {
		for _, v := range strings.Split(subject, "/") {
			kv := strings.Split(v, "=")
			if len(kv) == 2 && kv[0] == "emailAddress" {
				email = kv[1]
			}
		}
	}

	return pem, subject, email
}

func (a *App) AuthenticateUserForLogin(c *request.Context, id, loginID, password, mfaToken, cwsToken string, ldapOnly bool) (user *model.User, err *model.AppError) {
	// Do statistics
	defer func() {
		if a.Metrics() != nil {
			if user == nil || err != nil {
				a.Metrics().IncrementLoginFail()
			} else {
				a.Metrics().IncrementLogin()
			}
		}
	}()

	if password == "" && !IsCWSLogin(a, cwsToken) {
		return nil, model.NewAppError("AuthenticateUserForLogin", "api.user.login.blank_pwd.app_error", nil, "", http.StatusBadRequest)
	}

	// Get the MM user we are trying to login
	if user, err = a.GetUserForLogin(id, loginID); err != nil {
		return nil, err
	}

	// CWS login allow to use the one-time token to login the users when they're redirected to their
	// installation for the first time
	if IsCWSLogin(a, cwsToken) {
		if err = checkUserNotBot(user); err != nil {
			return nil, err
		}
		token, err := a.Srv().Store.Token().GetByToken(cwsToken)
		if nfErr := new(store.ErrNotFound); err != nil && !errors.As(err, &nfErr) {
			mlog.Debug("Error retrieving the cws token from the store", mlog.Err(err))
			return nil, model.NewAppError("AuthenticateUserForLogin",
				"api.user.login_by_cws.invalid_token.app_error", nil, "", http.StatusInternalServerError)
		}
		// If token is stored in the database that means it was used
		if token != nil {
			return nil, model.NewAppError("AuthenticateUserForLogin",
				"api.user.login_by_cws.invalid_token.app_error", nil, "", http.StatusBadRequest)
		}
		envToken, ok := os.LookupEnv(cwsTokenEnv)
		if ok && subtle.ConstantTimeCompare([]byte(envToken), []byte(cwsToken)) == 1 {
			token = &model.Token{
				Token:    cwsToken,
				CreateAt: model.GetMillis(),
				Type:     TokenTypeCWSAccess,
			}
			err := a.Srv().Store.Token().Save(token)
			if err != nil {
				mlog.Debug("Error storing the cws token in the store", mlog.Err(err))
				return nil, model.NewAppError("AuthenticateUserForLogin",
					"api.user.login_by_cws.invalid_token.app_error", nil, "", http.StatusInternalServerError)
			}
			return user, nil
		}
		return nil, model.NewAppError("AuthenticateUserForLogin",
			"api.user.login_by_cws.invalid_token.app_error", nil, "", http.StatusBadRequest)
	}

	// If client side cert is enable and it's checking as a primary source
	// then trust the proxy and cert that the correct user is supplied and allow
	// them access
	if *a.Config().ExperimentalSettings.ClientSideCertEnable && *a.Config().ExperimentalSettings.ClientSideCertCheck == model.ClientSideCertCheckPrimaryAuth {
		// Unless the user is a bot.
		if err = checkUserNotBot(user); err != nil {
			return nil, err
		}

		return user, nil
	}

	// and then authenticate them
	if user, err = a.authenticateUser(c, user, password, mfaToken); err != nil {
		return nil, err
	}

	return user, nil
}

func (a *App) GetUserForLogin(id, loginID string) (*model.User, *model.AppError) {
	enableUsername := *a.Config().EmailSettings.EnableSignInWithUsername
	enableEmail := *a.Config().EmailSettings.EnableSignInWithEmail

	// If we are given a userID then fail if we can't find a user with that ID
	if id != "" {
		user, err := a.GetUser(id)
		if err != nil {
			if err.ID != MissingAccountError {
				err.StatusCode = http.StatusInternalServerError
				return nil, err
			}
			err.StatusCode = http.StatusBadRequest
			return nil, err
		}
		return user, nil
	}

	// Try to get the user by username/email
	if user, err := a.Srv().Store.User().GetForLogin(loginID, enableUsername, enableEmail); err == nil {
		return user, nil
	}

	// Try to get the user with LDAP if enabled
	if *a.Config().LdapSettings.Enable && a.Ldap() != nil {
		if ldapUser, err := a.Ldap().GetUser(loginID); err == nil {
			if user, err := a.GetUserByAuth(ldapUser.AuthData, model.UserAuthServiceLdap); err == nil {
				return user, nil
			}
			return ldapUser, nil
		}
	}

	return nil, model.NewAppError("GetUserForLogin", "store.sql_user.get_for_login.app_error", nil, "", http.StatusBadRequest)
}

func (a *App) DoLogin(c *request.Context, w http.ResponseWriter, r *http.Request, user *model.User, deviceID string, isMobile, isOAuthUser, isSaml bool) *model.AppError {
	if pluginsEnvironment := a.GetPluginsEnvironment(); pluginsEnvironment != nil {
		var rejectionReason string
		pluginContext := pluginContext(c)
		pluginsEnvironment.RunMultiPluginHook(func(hooks plugin.Hooks) bool {
			rejectionReason = hooks.UserWillLogIn(pluginContext, user)
			return rejectionReason == ""
		}, plugin.UserWillLogInID)

		if rejectionReason != "" {
			return model.NewAppError("DoLogin", "Login rejected by plugin: "+rejectionReason, nil, "", http.StatusBadRequest)
		}
	}

	session := &model.Session{UserID: user.ID, Roles: user.GetRawRoles(), DeviceID: deviceID, IsOAuth: false, Props: map[string]string{
		model.UserAuthServiceIsMobile: strconv.FormatBool(isMobile),
		model.UserAuthServiceIsSaml:   strconv.FormatBool(isSaml),
		model.UserAuthServiceIsOAuth:  strconv.FormatBool(isOAuthUser),
	}}
	session.GenerateCSRF()

	if deviceID != "" {
		a.srv.userService.SetSessionExpireInDays(session, *a.Config().ServiceSettings.SessionLengthMobileInDays)

		// A special case where we logout of all other sessions with the same Id
		if err := a.RevokeSessionsForDeviceID(user.ID, deviceID, ""); err != nil {
			err.StatusCode = http.StatusInternalServerError
			return err
		}
	} else if isMobile {
		a.srv.userService.SetSessionExpireInDays(session, *a.Config().ServiceSettings.SessionLengthMobileInDays)
	} else if isOAuthUser || isSaml {
		a.srv.userService.SetSessionExpireInDays(session, *a.Config().ServiceSettings.SessionLengthSSOInDays)
	} else {
		a.srv.userService.SetSessionExpireInDays(session, *a.Config().ServiceSettings.SessionLengthWebInDays)
	}

	ua := uasurfer.Parse(r.UserAgent())

	plat := getPlatformName(ua)
	os := getOSName(ua)
	bname := getBrowserName(ua, r.UserAgent())
	bversion := getBrowserVersion(ua, r.UserAgent())

	session.AddProp(model.SessionPropPlatform, plat)
	session.AddProp(model.SessionPropOs, os)
	session.AddProp(model.SessionPropBrowser, fmt.Sprintf("%v/%v", bname, bversion))
	if user.IsGuest() {
		session.AddProp(model.SessionPropIsGuest, "true")
	} else {
		session.AddProp(model.SessionPropIsGuest, "false")
	}

	var err *model.AppError
	if session, err = a.CreateSession(session); err != nil {
		err.StatusCode = http.StatusInternalServerError
		return err
	}

	w.Header().Set(model.HeaderToken, session.Token)

	c.SetSession(session)
	if a.Srv().License() != nil && *a.Srv().License().Features.LDAP && a.Ldap() != nil {
		userVal := *user
		sessionVal := *session
		a.Srv().Go(func() {
			a.Ldap().UpdateProfilePictureIfNecessary(userVal, sessionVal)
		})
	}

	if pluginsEnvironment := a.GetPluginsEnvironment(); pluginsEnvironment != nil {
		a.Srv().Go(func() {
			pluginContext := pluginContext(c)
			pluginsEnvironment.RunMultiPluginHook(func(hooks plugin.Hooks) bool {
				hooks.UserHasLoggedIn(pluginContext, user)
				return true
			}, plugin.UserHasLoggedInID)
		})
	}

	return nil
}

func (a *App) AttachSessionCookies(c *request.Context, w http.ResponseWriter, r *http.Request) {
	secure := false
	if GetProtocol(r) == "https" {
		secure = true
	}

	maxAge := *a.Config().ServiceSettings.SessionLengthWebInDays * 60 * 60 * 24
	domain := a.GetCookieDomain()
	subpath, _ := utils.GetSubpathFromConfig(a.Config())

	expiresAt := time.Unix(model.GetMillis()/1000+int64(maxAge), 0)
	sessionCookie := &http.Cookie{
		Name:     model.SessionCookieToken,
		Value:    c.Session().Token,
		Path:     subpath,
		MaxAge:   maxAge,
		Expires:  expiresAt,
		HttpOnly: true,
		Domain:   domain,
		Secure:   secure,
	}

	userCookie := &http.Cookie{
		Name:    model.SessionCookieUser,
		Value:   c.Session().UserID,
		Path:    subpath,
		MaxAge:  maxAge,
		Expires: expiresAt,
		Domain:  domain,
		Secure:  secure,
	}

	csrfCookie := &http.Cookie{
		Name:    model.SessionCookieCsrf,
		Value:   c.Session().GetCSRF(),
		Path:    subpath,
		MaxAge:  maxAge,
		Expires: expiresAt,
		Domain:  domain,
		Secure:  secure,
	}

	http.SetCookie(w, sessionCookie)
	http.SetCookie(w, userCookie)
	http.SetCookie(w, csrfCookie)
}

func GetProtocol(r *http.Request) string {
	if r.Header.Get(model.HeaderForwardedProto) == "https" || r.TLS != nil {
		return "https"
	}
	return "http"
}

func IsCWSLogin(a *App, token string) bool {
	return a.Srv().License() != nil && *a.Srv().License().Features.Cloud && token != ""
}
