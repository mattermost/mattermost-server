// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/users"
	"github.com/mattermost/mattermost-server/v5/shared/mfa"
)

type TokenLocation int

const (
	TokenLocationNotFound TokenLocation = iota
	TokenLocationHeader
	TokenLocationCookie
	TokenLocationQueryString
	TokenLocationCloudHeader
	TokenLocationRemoteClusterHeader
)

func (tl TokenLocation) String() string {
	switch tl {
	case TokenLocationNotFound:
		return "Not Found"
	case TokenLocationHeader:
		return "Header"
	case TokenLocationCookie:
		return "Cookie"
	case TokenLocationQueryString:
		return "QueryString"
	case TokenLocationCloudHeader:
		return "CloudHeader"
	case TokenLocationRemoteClusterHeader:
		return "RemoteClusterHeader"
	default:
		return "Unknown"
	}
}

func (a *App) IsPasswordValid(password string) *model.AppError {

	if *a.Config().ServiceSettings.EnableDeveloper {
		return nil
	}

	if err := users.IsPasswordValidWithSettings(password, &a.Config().PasswordSettings); err != nil {
		var invErr *users.ErrInvalidPassword
		switch {
		case errors.As(err, &invErr):
			return model.NewAppError("User.IsValid", invErr.ID(), map[string]interface{}{"Min": *a.Config().PasswordSettings.MinimumLength}, "", http.StatusBadRequest)
		default:
			return model.NewAppError("User.IsValid", "app.valid_password_generic.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return nil
}

func (a *App) CheckPasswordAndAllCriteria(user *model.User, password string, mfaToken string) *model.AppError {
	if err := a.CheckUserPreflightAuthenticationCriteria(user, mfaToken); err != nil {
		return err
	}

	if err := users.CheckUserPassword(user, password); err != nil {
		if passErr := a.Srv().Store.User().UpdateFailedPasswordAttempts(user.ID, user.FailedAttempts+1); passErr != nil {
			return model.NewAppError("CheckPasswordAndAllCriteria", "app.user.update_failed_pwd_attempts.app_error", nil, passErr.Error(), http.StatusInternalServerError)
		}

		a.InvalidateCacheForUser(user.ID)

		var invErr *users.ErrInvalidPassword
		switch {
		case errors.As(err, &invErr):
			return model.NewAppError("checkUserPassword", "api.user.check_user_password.invalid.app_error", nil, "user_id="+user.ID, http.StatusUnauthorized)
		default:
			return model.NewAppError("checkUserPassword", "app.valid_password_generic.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	if err := a.CheckUserMfa(user, mfaToken); err != nil {
		// If the mfaToken is not set, we assume the client used this as a pre-flight request to query the server
		// about the MFA state of the user in question
		if mfaToken != "" {
			if passErr := a.Srv().Store.User().UpdateFailedPasswordAttempts(user.ID, user.FailedAttempts+1); passErr != nil {
				return model.NewAppError("CheckPasswordAndAllCriteria", "app.user.update_failed_pwd_attempts.app_error", nil, passErr.Error(), http.StatusInternalServerError)
			}
		}

		a.InvalidateCacheForUser(user.ID)

		return err
	}

	if passErr := a.Srv().Store.User().UpdateFailedPasswordAttempts(user.ID, 0); passErr != nil {
		return model.NewAppError("CheckPasswordAndAllCriteria", "app.user.update_failed_pwd_attempts.app_error", nil, passErr.Error(), http.StatusInternalServerError)
	}

	a.InvalidateCacheForUser(user.ID)

	if err := a.CheckUserPostflightAuthenticationCriteria(user); err != nil {
		return err
	}

	return nil
}

// This to be used for places we check the users password when they are already logged in
func (a *App) DoubleCheckPassword(user *model.User, password string) *model.AppError {
	if err := checkUserLoginAttempts(user, *a.Config().ServiceSettings.MaximumLoginAttempts); err != nil {
		return err
	}

	if err := users.CheckUserPassword(user, password); err != nil {
		if passErr := a.Srv().Store.User().UpdateFailedPasswordAttempts(user.ID, user.FailedAttempts+1); passErr != nil {
			return model.NewAppError("DoubleCheckPassword", "app.user.update_failed_pwd_attempts.app_error", nil, passErr.Error(), http.StatusInternalServerError)
		}

		a.InvalidateCacheForUser(user.ID)

		var invErr *users.ErrInvalidPassword
		switch {
		case errors.As(err, &invErr):
			return model.NewAppError("DoubleCheckPassword", "api.user.check_user_password.invalid.app_error", nil, "user_id="+user.ID, http.StatusUnauthorized)
		default:
			return model.NewAppError("DoubleCheckPassword", "app.valid_password_generic.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	if passErr := a.Srv().Store.User().UpdateFailedPasswordAttempts(user.ID, 0); passErr != nil {
		return model.NewAppError("DoubleCheckPassword", "app.user.update_failed_pwd_attempts.app_error", nil, passErr.Error(), http.StatusInternalServerError)
	}

	a.InvalidateCacheForUser(user.ID)

	return nil
}

func (a *App) checkLdapUserPasswordAndAllCriteria(c *request.Context, ldapID *string, password string, mfaToken string) (*model.User, *model.AppError) {
	if a.Ldap() == nil || ldapID == nil {
		err := model.NewAppError("doLdapAuthentication", "api.user.login_ldap.not_available.app_error", nil, "", http.StatusNotImplemented)
		return nil, err
	}

	ldapUser, err := a.Ldap().DoLogin(c, *ldapID, password)
	if err != nil {
		err.StatusCode = http.StatusUnauthorized
		return nil, err
	}

	if err := a.CheckUserMfa(ldapUser, mfaToken); err != nil {
		return nil, err
	}

	if err := checkUserNotDisabled(ldapUser); err != nil {
		return nil, err
	}

	// user successfully authenticated
	return ldapUser, nil
}

func (a *App) CheckUserAllAuthenticationCriteria(user *model.User, mfaToken string) *model.AppError {
	if err := a.CheckUserPreflightAuthenticationCriteria(user, mfaToken); err != nil {
		return err
	}

	if err := a.CheckUserPostflightAuthenticationCriteria(user); err != nil {
		return err
	}

	return nil
}

func (a *App) CheckUserPreflightAuthenticationCriteria(user *model.User, mfaToken string) *model.AppError {
	if err := checkUserNotDisabled(user); err != nil {
		return err
	}

	if err := checkUserNotBot(user); err != nil {
		return err
	}

	if err := checkUserLoginAttempts(user, *a.Config().ServiceSettings.MaximumLoginAttempts); err != nil {
		return err
	}

	return nil
}

func (a *App) CheckUserPostflightAuthenticationCriteria(user *model.User) *model.AppError {
	if !user.EmailVerified && *a.Config().EmailSettings.RequireEmailVerification {
		return model.NewAppError("Login", "api.user.login.not_verified.app_error", nil, "user_id="+user.ID, http.StatusUnauthorized)
	}

	return nil
}

func (a *App) CheckUserMfa(user *model.User, token string) *model.AppError {
	if !user.MfaActive || !*a.Config().ServiceSettings.EnableMultifactorAuthentication {
		return nil
	}

	if !*a.Config().ServiceSettings.EnableMultifactorAuthentication {
		return model.NewAppError("CheckUserMfa", "mfa.mfa_disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	ok, err := mfa.New(a.Srv().Store.User()).ValidateToken(user.MfaSecret, token)
	if err != nil {
		return model.NewAppError("CheckUserMfa", "mfa.validate_token.authenticate.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	if !ok {
		return model.NewAppError("checkUserMfa", "api.user.check_user_mfa.bad_code.app_error", nil, "", http.StatusUnauthorized)
	}

	return nil
}

func checkUserLoginAttempts(user *model.User, max int) *model.AppError {
	if user.FailedAttempts >= max {
		return model.NewAppError("checkUserLoginAttempts", "api.user.check_user_login_attempts.too_many.app_error", nil, "user_id="+user.ID, http.StatusUnauthorized)
	}

	return nil
}

func checkUserNotDisabled(user *model.User) *model.AppError {
	if user.DeleteAt > 0 {
		return model.NewAppError("Login", "api.user.login.inactive.app_error", nil, "user_id="+user.ID, http.StatusUnauthorized)
	}
	return nil
}

func checkUserNotBot(user *model.User) *model.AppError {
	if user.IsBot {
		return model.NewAppError("Login", "api.user.login.bot_login_forbidden.app_error", nil, "user_id="+user.ID, http.StatusUnauthorized)
	}
	return nil
}

func (a *App) authenticateUser(c *request.Context, user *model.User, password, mfaToken string) (*model.User, *model.AppError) {
	license := a.Srv().License()
	ldapAvailable := *a.Config().LdapSettings.Enable && a.Ldap() != nil && license != nil && *license.Features.LDAP

	if user.AuthService == model.UserAuthServiceLdap {
		if !ldapAvailable {
			err := model.NewAppError("login", "api.user.login_ldap.not_available.app_error", nil, "", http.StatusNotImplemented)
			return user, err
		}

		ldapUser, err := a.checkLdapUserPasswordAndAllCriteria(c, user.AuthData, password, mfaToken)
		if err != nil {
			err.StatusCode = http.StatusUnauthorized
			return user, err
		}

		// slightly redundant to get the user again, but we need to get it from the LDAP server
		return ldapUser, nil
	}

	if user.AuthService != "" {
		authService := user.AuthService
		if authService == model.UserAuthServiceSaml {
			authService = strings.ToUpper(authService)
		}
		err := model.NewAppError("login", "api.user.login.use_auth_service.app_error", map[string]interface{}{"AuthService": authService}, "", http.StatusBadRequest)
		return user, err
	}

	if err := a.CheckPasswordAndAllCriteria(user, password, mfaToken); err != nil {
		err.StatusCode = http.StatusUnauthorized
		return user, err
	}

	return user, nil
}

func ParseAuthTokenFromRequest(r *http.Request) (string, TokenLocation) {
	authHeader := r.Header.Get(model.HeaderAuth)

	// Attempt to parse the token from the cookie
	if cookie, err := r.Cookie(model.SessionCookieToken); err == nil {
		return cookie.Value, TokenLocationCookie
	}

	// Parse the token from the header
	if len(authHeader) > 6 && strings.ToUpper(authHeader[0:6]) == model.HeaderBearer {
		// Default session token
		return authHeader[7:], TokenLocationHeader
	}

	if len(authHeader) > 5 && strings.ToLower(authHeader[0:5]) == model.HeaderToken {
		// OAuth token
		return authHeader[6:], TokenLocationHeader
	}

	// Attempt to parse token out of the query string
	if token := r.URL.Query().Get("access_token"); token != "" {
		return token, TokenLocationQueryString
	}

	if token := r.Header.Get(model.HeaderCloudToken); token != "" {
		return token, TokenLocationCloudHeader
	}

	if token := r.Header.Get(model.HeaderRemoteclusterToken); token != "" {
		return token, TokenLocationRemoteClusterHeader
	}

	return "", TokenLocationNotFound
}
