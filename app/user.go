// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app/imaging"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/services/users"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mfa"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	TokenTypePasswordRecovery  = "password_recovery"
	TokenTypeVerifyEmail       = "verify_email"
	TokenTypeTeamInvitation    = "team_invitation"
	TokenTypeGuestInvitation   = "guest_invitation"
	TokenTypeCWSAccess         = "cws_access_token"
	PasswordRecoverExpiryTime  = 1000 * 60 * 60      // 1 hour
	InvitationExpiryTime       = 1000 * 60 * 60 * 48 // 48 hours
	ImageProfilePixelDimension = 128
)

func (a *App) CreateUserWithToken(c *request.Context, user *model.User, token *model.Token) (*model.User, *model.AppError) {
	if err := a.IsUserSignUpAllowed(); err != nil {
		return nil, err
	}

	if token.Type != TokenTypeTeamInvitation && token.Type != TokenTypeGuestInvitation {
		return nil, model.NewAppError("CreateUserWithToken", "api.user.create_user.signup_link_invalid.app_error", nil, "", http.StatusBadRequest)
	}

	if model.GetMillis()-token.CreateAt >= InvitationExpiryTime {
		a.DeleteToken(token)
		return nil, model.NewAppError("CreateUserWithToken", "api.user.create_user.signup_link_expired.app_error", nil, "", http.StatusBadRequest)
	}

	tokenData := model.MapFromJSON(strings.NewReader(token.Extra))

	team, nErr := a.Srv().Store.Team().Get(tokenData["teamId"])
	if nErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(nErr, &nfErr):
			return nil, model.NewAppError("CreateUserWithToken", "app.team.get.find.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("CreateUserWithToken", "app.team.get.finding.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	channels, nErr := a.Srv().Store.Channel().GetChannelsByIDs(strings.Split(tokenData["channels"], " "), false)
	if nErr != nil {
		return nil, model.NewAppError("CreateUserWithToken", "app.channel.get_channels_by_ids.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	user.Email = tokenData["email"]
	user.EmailVerified = true

	var ruser *model.User
	var err *model.AppError
	if token.Type == TokenTypeTeamInvitation {
		ruser, err = a.CreateUser(c, user)
	} else {
		ruser, err = a.CreateGuest(c, user)
	}
	if err != nil {
		return nil, err
	}

	if _, err := a.JoinUserToTeam(c, team, ruser, ""); err != nil {
		return nil, err
	}

	a.AddDirectChannels(team.ID, ruser)

	if token.Type == TokenTypeGuestInvitation {
		for _, channel := range channels {
			_, err := a.AddChannelMember(c, ruser.ID, channel, ChannelMemberOpts{})
			if err != nil {
				mlog.Warn("Failed to add channel member", mlog.Err(err))
			}
		}
	}

	if err := a.DeleteToken(token); err != nil {
		mlog.Warn("Error while deleting token", mlog.Err(err))
	}

	return ruser, nil
}

func (a *App) CreateUserWithInviteID(c *request.Context, user *model.User, inviteID, redirect string) (*model.User, *model.AppError) {
	if err := a.IsUserSignUpAllowed(); err != nil {
		return nil, err
	}

	team, nErr := a.Srv().Store.Team().GetByInviteID(inviteID)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(nErr, &nfErr):
			return nil, model.NewAppError("CreateUserWithInviteId", "app.team.get_by_invite_id.finding.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("CreateUserWithInviteId", "app.team.get_by_invite_id.finding.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	if team.IsGroupConstrained() {
		return nil, model.NewAppError("CreateUserWithInviteId", "app.team.invite_id.group_constrained.error", nil, "", http.StatusForbidden)
	}

	if !users.CheckUserDomain(user, team.AllowedDomains) {
		return nil, model.NewAppError("CreateUserWithInviteId", "api.team.invite_members.invalid_email.app_error", map[string]interface{}{"Addresses": team.AllowedDomains}, "", http.StatusForbidden)
	}

	user.EmailVerified = false

	ruser, err := a.CreateUser(c, user)
	if err != nil {
		return nil, err
	}

	if _, err := a.JoinUserToTeam(c, team, ruser, ""); err != nil {
		return nil, err
	}

	a.AddDirectChannels(team.ID, ruser)

	if err := a.Srv().EmailService.sendWelcomeEmail(ruser.ID, ruser.Email, ruser.EmailVerified, ruser.DisableWelcomeEmail, ruser.Locale, a.GetSiteURL(), redirect); err != nil {
		mlog.Warn("Failed to send welcome email on create user with inviteId", mlog.Err(err))
	}

	return ruser, nil
}

func (a *App) CreateUserAsAdmin(c *request.Context, user *model.User, redirect string) (*model.User, *model.AppError) {
	ruser, err := a.CreateUser(c, user)
	if err != nil {
		return nil, err
	}

	if err := a.Srv().EmailService.sendWelcomeEmail(ruser.ID, ruser.Email, ruser.EmailVerified, ruser.DisableWelcomeEmail, ruser.Locale, a.GetSiteURL(), redirect); err != nil {
		mlog.Warn("Failed to send welcome email to the new user, created by system admin", mlog.Err(err))
	}

	return ruser, nil
}

func (a *App) CreateUserFromSignup(c *request.Context, user *model.User, redirect string) (*model.User, *model.AppError) {
	if err := a.IsUserSignUpAllowed(); err != nil {
		return nil, err
	}

	if !a.IsFirstUserAccount() && !*a.Config().TeamSettings.EnableOpenServer {
		err := model.NewAppError("CreateUserFromSignup", "api.user.create_user.no_open_server", nil, "email="+user.Email, http.StatusForbidden)
		return nil, err
	}

	user.EmailVerified = false

	ruser, err := a.CreateUser(c, user)
	if err != nil {
		return nil, err
	}

	if err := a.Srv().EmailService.sendWelcomeEmail(ruser.ID, ruser.Email, ruser.EmailVerified, ruser.DisableWelcomeEmail, ruser.Locale, a.GetSiteURL(), redirect); err != nil {
		mlog.Warn("Failed to send welcome email on create user from signup", mlog.Err(err))
	}

	return ruser, nil
}

func (a *App) IsUserSignUpAllowed() *model.AppError {
	if !*a.Config().EmailSettings.EnableSignUpWithEmail || !*a.Config().TeamSettings.EnableUserCreation {
		err := model.NewAppError("IsUserSignUpAllowed", "api.user.create_user.signup_email_disabled.app_error", nil, "", http.StatusNotImplemented)
		return err
	}
	return nil
}

func (a *App) IsFirstUserAccount() bool {
	return a.srv.userService.IsFirstUserAccount()
}

// CreateUser creates a user and sets several fields of the returned User struct to
// their zero values.
func (a *App) CreateUser(c *request.Context, user *model.User) (*model.User, *model.AppError) {
	return a.createUserOrGuest(c, user, false)
}

// CreateGuest creates a guest and sets several fields of the returned User struct to
// their zero values.
func (a *App) CreateGuest(c *request.Context, user *model.User) (*model.User, *model.AppError) {
	return a.createUserOrGuest(c, user, true)
}

func (a *App) createUserOrGuest(c *request.Context, user *model.User, guest bool) (*model.User, *model.AppError) {
	ruser, nErr := a.srv.userService.CreateUser(user, users.UserCreateOptions{Guest: guest})
	if nErr != nil {
		var appErr *model.AppError
		var invErr *store.ErrInvalidInput
		var nfErr *users.ErrInvalidPassword
		switch {
		case errors.As(nErr, &appErr):
			return nil, appErr
		case errors.Is(nErr, users.AcceptedDomainError):
			return nil, model.NewAppError("createUserOrGuest", "api.user.create_user.accepted_domain.app_error", nil, "", http.StatusBadRequest)
		case errors.As(nErr, &nfErr):
			return nil, model.NewAppError("createUserOrGuest", "api.user.check_user_password.invalid.app_error", nil, "", http.StatusBadRequest)
		case errors.Is(nErr, users.UserCountError):
			return nil, model.NewAppError("createUserOrGuest", "app.user.get_total_users_count.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		case errors.As(nErr, &invErr):
			switch invErr.Field {
			case "email":
				return nil, model.NewAppError("createUserOrGuest", "app.user.save.email_exists.app_error", nil, invErr.Error(), http.StatusBadRequest)
			case "username":
				return nil, model.NewAppError("createUserOrGuest", "app.user.save.username_exists.app_error", nil, invErr.Error(), http.StatusBadRequest)
			default:
				return nil, model.NewAppError("createUserOrGuest", "app.user.save.existing.app_error", nil, invErr.Error(), http.StatusBadRequest)
			}
		default:
			return nil, model.NewAppError("createUserOrGuest", "app.user.save.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	if user.EmailVerified {
		a.InvalidateCacheForUser(ruser.ID)

		nUser, err := a.srv.userService.GetUser(ruser.ID)
		if err != nil {
			var nfErr *store.ErrNotFound
			switch {
			case errors.As(err, &nfErr):
				return nil, model.NewAppError("createUserOrGuest", MissingAccountError, nil, nfErr.Error(), http.StatusNotFound)
			default:
				return nil, model.NewAppError("createUserOrGuest", "app.user.get.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}

		a.sendUpdatedUserEvent(*nUser)
	}

	pref := model.Preference{UserID: ruser.ID, Category: model.PreferenceCategoryTutorialSteps, Name: ruser.ID, Value: "0"}
	if err := a.Srv().Store.Preference().Save(&model.Preferences{pref}); err != nil {
		mlog.Warn("Encountered error saving tutorial preference", mlog.Err(err))
	}

	go a.UpdateViewedProductNoticesForNewUser(ruser.ID)

	// This message goes to everyone, so the teamID, channelID and userID are irrelevant
	message := model.NewWebSocketEvent(model.WebsocketEventNewUser, "", "", "", nil)
	message.Add("user_id", ruser.ID)
	a.Publish(message)

	if pluginsEnvironment := a.GetPluginsEnvironment(); pluginsEnvironment != nil {
		a.Srv().Go(func() {
			pluginContext := pluginContext(c)
			pluginsEnvironment.RunMultiPluginHook(func(hooks plugin.Hooks) bool {
				hooks.UserHasBeenCreated(pluginContext, user)
				return true
			}, plugin.UserHasBeenCreatedID)
		})
	}

	return ruser, nil
}

func (a *App) CreateOAuthUser(c *request.Context, service string, userData io.Reader, teamID string, tokenUser *model.User) (*model.User, *model.AppError) {
	if !*a.Config().TeamSettings.EnableUserCreation {
		return nil, model.NewAppError("CreateOAuthUser", "api.user.create_user.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	provider, e := a.getSSOProvider(service)
	if e != nil {
		return nil, e
	}
	user, err1 := provider.GetUserFromJSON(userData, tokenUser)
	if err1 != nil {
		return nil, model.NewAppError("CreateOAuthUser", "api.user.create_oauth_user.create.app_error", map[string]interface{}{"Service": service}, err1.Error(), http.StatusInternalServerError)
	}
	if user.AuthService == "" {
		user.AuthService = service
	}

	found := true
	count := 0
	for found {
		if found = a.srv.userService.IsUsernameTaken(user.Username); found {
			user.Username = user.Username + strconv.Itoa(count)
			count++
		}
	}

	userByAuth, _ := a.srv.userService.GetUserByAuth(user.AuthData, service)
	if userByAuth != nil {
		return userByAuth, nil
	}

	userByEmail, _ := a.srv.userService.GetUserByEmail(user.Email)
	if userByEmail != nil {
		if userByEmail.AuthService == "" {
			return nil, model.NewAppError("CreateOAuthUser", "api.user.create_oauth_user.already_attached.app_error", map[string]interface{}{"Service": service, "Auth": model.UserAuthServiceEmail}, "email="+user.Email, http.StatusBadRequest)
		}
		if provider.IsSameUser(userByEmail, user) {
			if _, err := a.Srv().Store.User().UpdateAuthData(userByEmail.ID, user.AuthService, user.AuthData, "", false); err != nil {
				// if the user is not updated, write a warning to the log, but don't prevent user login
				mlog.Warn("Error attempting to update user AuthData", mlog.Err(err))
			}
			return userByEmail, nil
		}
		return nil, model.NewAppError("CreateOAuthUser", "api.user.create_oauth_user.already_attached.app_error", map[string]interface{}{"Service": service, "Auth": userByEmail.AuthService}, "email="+user.Email+" authData="+*user.AuthData, http.StatusBadRequest)
	}

	user.EmailVerified = true

	ruser, err := a.CreateUser(c, user)
	if err != nil {
		return nil, err
	}

	if teamID != "" {
		err = a.AddUserToTeamByTeamID(c, teamID, user)
		if err != nil {
			return nil, err
		}

		err = a.AddDirectChannels(teamID, user)
		if err != nil {
			mlog.Warn("Failed to add direct channels", mlog.Err(err))
		}
	}

	return ruser, nil
}

func (a *App) GetUser(userID string) (*model.User, *model.AppError) {
	user, err := a.srv.userService.GetUser(userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetUser", MissingAccountError, nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetUser", "app.user.get_by_username.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return user, nil
}

func (a *App) GetUserByUsername(username string) (*model.User, *model.AppError) {
	result, err := a.srv.userService.GetUserByUsername(username)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetUserByUsername", "app.user.get_by_username.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetUserByUsername", "app.user.get_by_username.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return result, nil
}

func (a *App) GetUserByEmail(email string) (*model.User, *model.AppError) {
	user, err := a.srv.userService.GetUserByEmail(email)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetUserByEmail", MissingAccountError, nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetUserByEmail", MissingAccountError, nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return user, nil
}

func (a *App) GetUserByAuth(authData *string, authService string) (*model.User, *model.AppError) {
	user, err := a.srv.userService.GetUserByAuth(authData, authService)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &invErr):
			return nil, model.NewAppError("GetUserByAuth", MissingAuthAccountError, nil, invErr.Error(), http.StatusBadRequest)
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetUserByAuth", MissingAuthAccountError, nil, nfErr.Error(), http.StatusInternalServerError)
		default:
			return nil, model.NewAppError("GetUserByAuth", "app.user.get_by_auth.other.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return user, nil
}

func (a *App) GetUsers(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsers(options)
	if err != nil {
		return nil, model.NewAppError("GetUsers", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersPage(options *model.UserGetOptions, asAdmin bool) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersPage(options, asAdmin)
	if err != nil {
		return nil, model.NewAppError("GetUsersPage", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersEtag(restrictionsHash string) string {
	return a.srv.userService.GetUsersEtag(restrictionsHash)
}

func (a *App) GetUsersInTeam(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersInTeam(options)
	if err != nil {
		return nil, model.NewAppError("GetUsersInTeam", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersNotInTeam(teamID string, groupConstrained bool, offset int, limit int, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersNotInTeam(teamID, groupConstrained, offset, limit, viewRestrictions)
	if err != nil {
		return nil, model.NewAppError("GetUsersNotInTeam", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersInTeamPage(options *model.UserGetOptions, asAdmin bool) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersInTeamPage(options, asAdmin)
	if err != nil {
		return nil, model.NewAppError("GetUsersInTeamPage", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return a.sanitizeProfiles(users, asAdmin), nil
}

func (a *App) GetUsersNotInTeamPage(teamID string, groupConstrained bool, page int, perPage int, asAdmin bool, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersNotInTeamPage(teamID, groupConstrained, page*perPage, perPage, asAdmin, viewRestrictions)
	if err != nil {
		return nil, model.NewAppError("GetUsersNotInTeamPage", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return a.sanitizeProfiles(users, asAdmin), nil
}

func (a *App) GetUsersInTeamEtag(teamID string, restrictionsHash string) string {
	return a.srv.userService.GetUsersInTeamEtag(teamID, restrictionsHash)
}

func (a *App) GetUsersNotInTeamEtag(teamID string, restrictionsHash string) string {
	return a.srv.userService.GetUsersNotInTeamEtag(teamID, restrictionsHash)
}

func (a *App) GetUsersInChannel(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
	users, err := a.Srv().Store.User().GetProfilesInChannel(options)
	if err != nil {
		return nil, model.NewAppError("GetUsersInChannel", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersInChannelByStatus(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
	users, err := a.Srv().Store.User().GetProfilesInChannelByStatus(options)
	if err != nil {
		return nil, model.NewAppError("GetUsersInChannelByStatus", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersInChannelMap(options *model.UserGetOptions, asAdmin bool) (map[string]*model.User, *model.AppError) {
	users, err := a.GetUsersInChannel(options)
	if err != nil {
		return nil, err
	}

	userMap := make(map[string]*model.User, len(users))

	for _, user := range users {
		a.SanitizeProfile(user, asAdmin)
		userMap[user.ID] = user
	}

	return userMap, nil
}

func (a *App) GetUsersInChannelPage(options *model.UserGetOptions, asAdmin bool) ([]*model.User, *model.AppError) {
	users, err := a.GetUsersInChannel(options)
	if err != nil {
		return nil, err
	}
	return a.sanitizeProfiles(users, asAdmin), nil
}

func (a *App) GetUsersInChannelPageByStatus(options *model.UserGetOptions, asAdmin bool) ([]*model.User, *model.AppError) {
	users, err := a.GetUsersInChannelByStatus(options)
	if err != nil {
		return nil, err
	}
	return a.sanitizeProfiles(users, asAdmin), nil
}

func (a *App) GetUsersNotInChannel(teamID string, channelID string, groupConstrained bool, offset int, limit int, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, *model.AppError) {
	users, err := a.Srv().Store.User().GetProfilesNotInChannel(teamID, channelID, groupConstrained, offset, limit, viewRestrictions)
	if err != nil {
		return nil, model.NewAppError("GetUsersNotInChannel", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersNotInChannelMap(teamID string, channelID string, groupConstrained bool, offset int, limit int, asAdmin bool, viewRestrictions *model.ViewUsersRestrictions) (map[string]*model.User, *model.AppError) {
	users, err := a.GetUsersNotInChannel(teamID, channelID, groupConstrained, offset, limit, viewRestrictions)
	if err != nil {
		return nil, err
	}

	userMap := make(map[string]*model.User, len(users))

	for _, user := range users {
		a.SanitizeProfile(user, asAdmin)
		userMap[user.ID] = user
	}

	return userMap, nil
}

func (a *App) GetUsersNotInChannelPage(teamID string, channelID string, groupConstrained bool, page int, perPage int, asAdmin bool, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, *model.AppError) {
	users, err := a.GetUsersNotInChannel(teamID, channelID, groupConstrained, page*perPage, perPage, viewRestrictions)
	if err != nil {
		return nil, err
	}

	return a.sanitizeProfiles(users, asAdmin), nil
}

func (a *App) GetUsersWithoutTeamPage(options *model.UserGetOptions, asAdmin bool) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersWithoutTeamPage(options, asAdmin)
	if err != nil {
		return nil, model.NewAppError("GetUsersWithoutTeamPage", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return a.sanitizeProfiles(users, asAdmin), nil
}

func (a *App) GetUsersWithoutTeam(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersWithoutTeam(options)
	if err != nil {
		return nil, model.NewAppError("GetUsersWithoutTeam", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

// GetTeamGroupUsers returns the users who are associated to the team via GroupTeams and GroupMembers.
func (a *App) GetTeamGroupUsers(teamID string) ([]*model.User, *model.AppError) {
	users, err := a.Srv().Store.User().GetTeamGroupUsers(teamID)
	if err != nil {
		return nil, model.NewAppError("GetTeamGroupUsers", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

// GetChannelGroupUsers returns the users who are associated to the channel via GroupChannels and GroupMembers.
func (a *App) GetChannelGroupUsers(channelID string) ([]*model.User, *model.AppError) {
	users, err := a.Srv().Store.User().GetChannelGroupUsers(channelID)
	if err != nil {
		return nil, model.NewAppError("GetChannelGroupUsers", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersByIDs(userIDs []string, options *store.UserGetByIDsOpts) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersByIDs(userIDs, options)
	if err != nil {
		return nil, model.NewAppError("GetUsersByIds", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

func (a *App) GetUsersByGroupChannelIDs(c *request.Context, channelIDs []string, asAdmin bool) (map[string][]*model.User, *model.AppError) {
	usersByChannelID, err := a.Srv().Store.User().GetProfileByGroupChannelIDsForUser(c.Session().UserID, channelIDs)
	if err != nil {
		return nil, model.NewAppError("GetUsersByGroupChannelIds", "app.user.get_profile_by_group_channel_ids_for_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	for channelID, userList := range usersByChannelID {
		usersByChannelID[channelID] = a.sanitizeProfiles(userList, asAdmin)
	}

	return usersByChannelID, nil
}

func (a *App) GetUsersByUsernames(usernames []string, asAdmin bool, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, *model.AppError) {
	users, err := a.srv.userService.GetUsersByUsernames(usernames, &model.UserGetOptions{ViewRestrictions: viewRestrictions})
	if err != nil {
		return nil, model.NewAppError("GetUsersByUsernames", "app.user.get_profiles.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return a.sanitizeProfiles(users, asAdmin), nil
}

func (a *App) sanitizeProfiles(users []*model.User, asAdmin bool) []*model.User {
	for _, u := range users {
		a.SanitizeProfile(u, asAdmin)
	}

	return users
}

func (a *App) GenerateMfaSecret(userID string) (*model.MfaSecret, *model.AppError) {
	user, appErr := a.GetUser(userID)
	if appErr != nil {
		return nil, appErr
	}

	if !*a.Config().ServiceSettings.EnableMultifactorAuthentication {
		return nil, model.NewAppError("GenerateMfaSecret", "mfa.mfa_disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	mfaSecret, err := a.srv.userService.GenerateMfaSecret(user)
	if err != nil {
		return nil, model.NewAppError("GenerateMfaSecret", "mfa.generate_qr_code.create_code.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return mfaSecret, nil
}

func (a *App) ActivateMfa(userID, token string) *model.AppError {
	user, appErr := a.GetUser(userID)
	if appErr != nil {
		return appErr
	}

	if user.AuthService != "" && user.AuthService != model.UserAuthServiceLdap {
		return model.NewAppError("ActivateMfa", "api.user.activate_mfa.email_and_ldap_only.app_error", nil, "", http.StatusBadRequest)
	}

	if !*a.Config().ServiceSettings.EnableMultifactorAuthentication {
		return model.NewAppError("ActivateMfa", "mfa.mfa_disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	if err := a.srv.userService.ActivateMfa(user, token); err != nil {
		switch {
		case errors.Is(err, mfa.InvalidToken):
			return model.NewAppError("ActivateMfa", "mfa.activate.bad_token.app_error", nil, "", http.StatusUnauthorized)
		default:
			return model.NewAppError("ActivateMfa", "mfa.activate.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	// Make sure old MFA status is not cached locally or in cluster nodes.
	a.InvalidateCacheForUser(userID)

	return nil
}

func (a *App) DeactivateMfa(userID string) *model.AppError {
	user, appErr := a.GetUser(userID)
	if appErr != nil {
		return appErr
	}

	if err := a.srv.userService.DeactivateMfa(user); err != nil {
		return model.NewAppError("DeactivateMfa", "mfa.deactivate.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	// Make sure old MFA status is not cached locally or in cluster nodes.
	a.InvalidateCacheForUser(userID)

	return nil
}

func (a *App) GetProfileImage(user *model.User) ([]byte, bool, *model.AppError) {
	return a.srv.GetProfileImage(user)
}

func (a *App) GetDefaultProfileImage(user *model.User) ([]byte, *model.AppError) {
	return a.srv.GetDefaultProfileImage(user)
}

func (a *App) SetDefaultProfileImage(user *model.User) *model.AppError {
	img, appErr := a.GetDefaultProfileImage(user)
	if appErr != nil {
		return appErr
	}

	path := "users/" + user.ID + "/profile.png"

	if _, err := a.WriteFile(bytes.NewReader(img), path); err != nil {
		return err
	}

	if err := a.Srv().Store.User().ResetLastPictureUpdate(user.ID); err != nil {
		mlog.Warn("Failed to reset last picture update", mlog.Err(err))
	}

	a.InvalidateCacheForUser(user.ID)

	updatedUser, appErr := a.GetUser(user.ID)
	if appErr != nil {
		mlog.Warn("Error in getting users profile forcing logout", mlog.String("user_id", user.ID), mlog.Err(appErr))
		return nil
	}

	options := a.Config().GetSanitizeOptions()
	updatedUser.SanitizeProfile(options)

	message := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", "", nil)
	message.Add("user", updatedUser)
	a.Publish(message)

	return nil
}

func (a *App) SetProfileImage(userID string, imageData *multipart.FileHeader) *model.AppError {
	file, err := imageData.Open()
	if err != nil {
		return model.NewAppError("SetProfileImage", "api.user.upload_profile_user.open.app_error", nil, err.Error(), http.StatusBadRequest)
	}
	defer file.Close()
	return a.SetProfileImageFromMultiPartFile(userID, file)
}

func (a *App) SetProfileImageFromMultiPartFile(userID string, file multipart.File) *model.AppError {
	if limitErr := checkImageLimits(file); limitErr != nil {
		return model.NewAppError("SetProfileImage", "api.user.upload_profile_user.check_image_limits.app_error", nil, "", http.StatusBadRequest)
	}

	return a.SetProfileImageFromFile(userID, file)
}

func (a *App) AdjustImage(file io.Reader) (*bytes.Buffer, *model.AppError) {
	// Decode image into Image object
	img, _, err := a.srv.imgDecoder.Decode(file)
	if err != nil {
		return nil, model.NewAppError("SetProfileImage", "api.user.upload_profile_user.decode.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	orientation, _ := imaging.GetImageOrientation(file)
	img = imaging.MakeImageUpright(img, orientation)

	// Scale profile image
	profileWidthAndHeight := 128
	img = imaging.FillCenter(img, profileWidthAndHeight, profileWidthAndHeight)

	buf := new(bytes.Buffer)
	err = a.srv.imgEncoder.EncodePNG(buf, img)
	if err != nil {
		return nil, model.NewAppError("SetProfileImage", "api.user.upload_profile_user.encode.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return buf, nil
}

func (a *App) SetProfileImageFromFile(userID string, file io.Reader) *model.AppError {
	buf, err := a.AdjustImage(file)
	if err != nil {
		return err
	}
	path := "users/" + userID + "/profile.png"

	if _, err := a.WriteFile(buf, path); err != nil {
		return model.NewAppError("SetProfileImage", "api.user.upload_profile_user.upload_profile.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.User().UpdateLastPictureUpdate(userID); err != nil {
		mlog.Warn("Error with updating last picture update", mlog.Err(err))
	}
	a.invalidateUserCacheAndPublish(userID)
	a.onUserProfileChange(userID)

	return nil
}

func (a *App) UpdatePasswordAsUser(userID, currentPassword, newPassword string) *model.AppError {
	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	if user == nil {
		err = model.NewAppError("updatePassword", "api.user.update_password.valid_account.app_error", nil, "", http.StatusBadRequest)
		return err
	}

	if user.AuthData != nil && *user.AuthData != "" {
		err = model.NewAppError("updatePassword", "api.user.update_password.oauth.app_error", nil, "auth_service="+user.AuthService, http.StatusBadRequest)
		return err
	}

	if err := a.DoubleCheckPassword(user, currentPassword); err != nil {
		if err.ID == "api.user.check_user_password.invalid.app_error" {
			err = model.NewAppError("updatePassword", "api.user.update_password.incorrect.app_error", nil, "", http.StatusBadRequest)
		}
		return err
	}

	T := i18n.GetUserTranslations(user.Locale)

	return a.UpdatePasswordSendEmail(user, newPassword, T("api.user.update_password.menu"))
}

func (a *App) userDeactivated(c *request.Context, userID string) *model.AppError {
	a.SetStatusOffline(userID, false)

	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	// when disable a user, userDeactivated is called for the user and the
	// bots the user owns. Only notify once, when the user is the owner, not the
	// owners bots
	if !user.IsBot {
		a.notifySysadminsBotOwnerDeactivated(c, userID)
	}

	if *a.Config().ServiceSettings.DisableBotsWhenOwnerIsDeactivated {
		a.disableUserBots(c, userID)
	}

	return nil
}

func (a *App) invalidateUserChannelMembersCaches(userID string) *model.AppError {
	teamsForUser, err := a.GetTeamsForUser(userID)
	if err != nil {
		return err
	}

	for _, team := range teamsForUser {
		channelsForUser, err := a.GetChannelsForUser(team.ID, userID, false, 0)
		if err != nil {
			return err
		}

		for _, channel := range *channelsForUser {
			a.invalidateCacheForChannelMembers(channel.ID)
		}
	}

	return nil
}

func (a *App) UpdateActive(c *request.Context, user *model.User, active bool) (*model.User, *model.AppError) {
	user.UpdateAt = model.GetMillis()
	if active {
		user.DeleteAt = 0
	} else {
		user.DeleteAt = user.UpdateAt
	}

	userUpdate, err := a.srv.userService.UpdateUser(user, true)
	if err != nil {
		var appErr *model.AppError
		var invErr *store.ErrInvalidInput
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &invErr):
			return nil, model.NewAppError("UpdateActive", "app.user.update.find.app_error", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("UpdateActive", "app.user.update.finding.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	ruser := userUpdate.New

	if !active {
		if err := a.RevokeAllSessions(ruser.ID); err != nil {
			return nil, err
		}
		if err := a.userDeactivated(c, ruser.ID); err != nil {
			return nil, err
		}
	}

	a.invalidateUserChannelMembersCaches(user.ID)
	a.InvalidateCacheForUser(user.ID)

	a.sendUpdatedUserEvent(*ruser)

	return ruser, nil
}

func (a *App) DeactivateGuests(c *request.Context) *model.AppError {
	userIDs, err := a.srv.userService.DeactivateAllGuests()
	if err != nil {
		return model.NewAppError("DeactivateGuests", "app.user.update_active_for_multiple_users.updating.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, userID := range userIDs {
		if err := a.userDeactivated(c, userID); err != nil {
			return err
		}
	}

	a.Srv().Store.Channel().ClearCaches()
	a.Srv().Store.User().ClearCaches()

	message := model.NewWebSocketEvent(model.WebsocketEventGuestsDeactivated, "", "", "", nil)
	a.Publish(message)

	return nil
}

func (a *App) GetSanitizeOptions(asAdmin bool) map[string]bool {
	return a.srv.userService.GetSanitizeOptions(asAdmin)
}

func (a *App) SanitizeProfile(user *model.User, asAdmin bool) {
	options := a.srv.userService.GetSanitizeOptions(asAdmin)

	user.SanitizeProfile(options)
}

func (a *App) UpdateUserAsUser(user *model.User, asAdmin bool) (*model.User, *model.AppError) {
	updatedUser, err := a.UpdateUser(user, true)
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

// CheckProviderAttributes returns the empty string if the patch can be applied without
// overriding attributes set by the user's login provider; otherwise, the name of the offending
// field is returned.
func (a *App) CheckProviderAttributes(user *model.User, patch *model.UserPatch) string {
	tryingToChange := func(userValue *string, patchValue *string) bool {
		return patchValue != nil && *patchValue != *userValue
	}

	// If any login provider is used, then the username may not be changed
	if user.AuthService != "" && tryingToChange(&user.Username, patch.Username) {
		return "username"
	}

	LdapSettings := &a.Config().LdapSettings
	SamlSettings := &a.Config().SamlSettings

	conflictField := ""
	if a.Ldap() != nil &&
		(user.IsLDAPUser() || (user.IsSAMLUser() && *SamlSettings.EnableSyncWithLdap)) {
		conflictField = a.Ldap().CheckProviderAttributes(LdapSettings, user, patch)
	} else if a.Saml() != nil && user.IsSAMLUser() {
		conflictField = a.Saml().CheckProviderAttributes(SamlSettings, user, patch)
	} else if user.IsOAuthUser() {
		if tryingToChange(&user.FirstName, patch.FirstName) || tryingToChange(&user.LastName, patch.LastName) {
			conflictField = "full name"
		}
	}

	return conflictField
}

func (a *App) PatchUser(userID string, patch *model.UserPatch, asAdmin bool) (*model.User, *model.AppError) {
	user, err := a.GetUser(userID)
	if err != nil {
		return nil, err
	}

	user.Patch(patch)

	updatedUser, err := a.UpdateUser(user, true)
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

func (a *App) UpdateUserAuth(userID string, userAuth *model.UserAuth) (*model.UserAuth, *model.AppError) {
	userAuth.Password = ""
	if _, err := a.Srv().Store.User().UpdateAuthData(userID, userAuth.AuthService, userAuth.AuthData, "", false); err != nil {
		var invErr *store.ErrInvalidInput
		switch {
		case errors.As(err, &invErr):
			return nil, model.NewAppError("UpdateUserAuth", "app.user.update_auth_data.email_exists.app_error", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("UpdateUserAuth", "app.user.update_auth_data.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return userAuth, nil
}

func (a *App) sendUpdatedUserEvent(user model.User) {
	adminCopyOfUser := user.DeepCopy()
	a.SanitizeProfile(adminCopyOfUser, true)
	adminMessage := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", "", nil)
	adminMessage.Add("user", adminCopyOfUser)
	adminMessage.GetBroadcast().ContainsSensitiveData = true
	a.Publish(adminMessage)

	a.SanitizeProfile(&user, false)
	message := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", "", nil)
	message.Add("user", &user)
	message.GetBroadcast().ContainsSanitizedData = true
	a.Publish(message)
}

func (a *App) UpdateUser(user *model.User, sendNotifications bool) (*model.User, *model.AppError) {
	prev, err := a.srv.userService.GetUser(user.ID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("UpdateUser", MissingAccountError, nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("UpdateUser", "app.user.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	var newEmail string
	if user.Email != prev.Email {
		if !users.CheckUserDomain(user, *a.Config().TeamSettings.RestrictCreationToDomains) {
			if !prev.IsGuest() && !prev.IsLDAPUser() && !prev.IsSAMLUser() {
				return nil, model.NewAppError("UpdateUser", "api.user.update_user.accepted_domain.app_error", nil, "", http.StatusBadRequest)
			}
		}

		if !users.CheckUserDomain(user, *a.Config().GuestAccountsSettings.RestrictCreationToDomains) {
			if prev.IsGuest() && !prev.IsLDAPUser() && !prev.IsSAMLUser() {
				return nil, model.NewAppError("UpdateUser", "api.user.update_user.accepted_guest_domain.app_error", nil, "", http.StatusBadRequest)
			}
		}

		if *a.Config().EmailSettings.RequireEmailVerification {
			newEmail = user.Email
			// Don't set new eMail on user account if email verification is required, this will be done as a post-verification action
			// to avoid users being able to set non-controlled eMails as their account email
			if _, appErr := a.GetUserByEmail(newEmail); appErr == nil {
				return nil, model.NewAppError("UpdateUser", "app.user.save.email_exists.app_error", nil, "user_id="+user.ID, http.StatusBadRequest)
			}

			//  When a bot is created, prev.Email will be an autogenerated faked email,
			//  which will not match a CLI email input during bot to user conversions.
			//  To update a bot users email, do not set the email to the faked email
			//  stored in prev.Email.  Allow using the email defined in the CLI
			if !user.IsBot {
				user.Email = prev.Email
			}
		}
	}

	userUpdate, err := a.srv.userService.UpdateUser(user, false)
	if err != nil {
		var appErr *model.AppError
		var invErr *store.ErrInvalidInput
		var conErr *store.ErrConflict
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &invErr):
			return nil, model.NewAppError("UpdateUser", "app.user.update.find.app_error", nil, invErr.Error(), http.StatusBadRequest)
		case errors.As(err, &conErr):
			if cErr, ok := err.(*store.ErrConflict); ok && cErr.Resource == "Username" {
				return nil, model.NewAppError("UpdateUser", "app.user.save.username_exists.app_error", nil, "", http.StatusBadRequest)
			}
			return nil, model.NewAppError("UpdateUser", "app.user.save.email_exists.app_error", nil, "", http.StatusBadRequest)
		default:
			return nil, model.NewAppError("UpdateUser", "app.user.update.finding.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	if sendNotifications {
		if userUpdate.New.Email != userUpdate.Old.Email || newEmail != "" {
			if *a.Config().EmailSettings.RequireEmailVerification {
				a.Srv().Go(func() {
					if err := a.SendEmailVerification(userUpdate.New, newEmail, ""); err != nil {
						mlog.Error("Failed to send email verification", mlog.Err(err))
					}
				})
			} else {
				a.Srv().Go(func() {
					if err := a.Srv().EmailService.sendEmailChangeEmail(userUpdate.Old.Email, userUpdate.New.Email, userUpdate.New.Locale, a.GetSiteURL()); err != nil {
						mlog.Error("Failed to send email change email", mlog.Err(err))
					}
				})
			}
		}

		if userUpdate.New.Username != userUpdate.Old.Username {
			a.Srv().Go(func() {
				if err := a.Srv().EmailService.sendChangeUsernameEmail(userUpdate.New.Username, userUpdate.New.Email, userUpdate.New.Locale, a.GetSiteURL()); err != nil {
					mlog.Error("Failed to send change username email", mlog.Err(err))
				}
			})
		}
		a.sendUpdatedUserEvent(*userUpdate.New)
	}

	a.InvalidateCacheForUser(user.ID)
	a.onUserProfileChange(user.ID)

	return userUpdate.New, nil
}

func (a *App) UpdateUserActive(c *request.Context, userID string, active bool) *model.AppError {
	user, err := a.GetUser(userID)

	if err != nil {
		return err
	}
	if _, err = a.UpdateActive(c, user, active); err != nil {
		return err
	}

	return nil
}

func (a *App) UpdateUserNotifyProps(userID string, props map[string]string, sendNotifications bool) (*model.User, *model.AppError) {
	user, err := a.GetUser(userID)
	if err != nil {
		return nil, err
	}

	user.NotifyProps = props

	ruser, err := a.UpdateUser(user, sendNotifications)
	if err != nil {
		return nil, err
	}

	return ruser, nil
}

func (a *App) UpdateMfa(activate bool, userID, token string) *model.AppError {
	if activate {
		if err := a.ActivateMfa(userID, token); err != nil {
			return err
		}
	} else {
		if err := a.DeactivateMfa(userID); err != nil {
			return err
		}
	}

	a.Srv().Go(func() {
		user, err := a.GetUser(userID)
		if err != nil {
			mlog.Error("Failed to get user", mlog.Err(err))
			return
		}

		if err := a.Srv().EmailService.sendMfaChangeEmail(user.Email, activate, user.Locale, a.GetSiteURL()); err != nil {
			mlog.Error("Failed to send mfa change email", mlog.Err(err))
		}
	})

	return nil
}

func (a *App) UpdatePasswordByUserIDSendEmail(userID, newPassword, method string) *model.AppError {
	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	return a.UpdatePasswordSendEmail(user, newPassword, method)
}

func (a *App) UpdatePassword(user *model.User, newPassword string) *model.AppError {
	if err := a.IsPasswordValid(newPassword); err != nil {
		return err
	}

	hashedPassword := model.HashPassword(newPassword)

	if err := a.Srv().Store.User().UpdatePassword(user.ID, hashedPassword); err != nil {
		return model.NewAppError("UpdatePassword", "api.user.update_password.failed.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	a.InvalidateCacheForUser(user.ID)

	return nil
}

func (a *App) UpdatePasswordSendEmail(user *model.User, newPassword, method string) *model.AppError {
	if err := a.UpdatePassword(user, newPassword); err != nil {
		return err
	}

	a.Srv().Go(func() {
		if err := a.Srv().EmailService.sendPasswordChangeEmail(user.Email, method, user.Locale, a.GetSiteURL()); err != nil {
			mlog.Error("Failed to send password change email", mlog.Err(err))
		}
	})

	return nil
}

func (a *App) UpdateHashedPasswordByUserID(userID, newHashedPassword string) *model.AppError {
	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	return a.UpdateHashedPassword(user, newHashedPassword)
}

func (a *App) UpdateHashedPassword(user *model.User, newHashedPassword string) *model.AppError {
	if err := a.Srv().Store.User().UpdatePassword(user.ID, newHashedPassword); err != nil {
		return model.NewAppError("UpdatePassword", "api.user.update_password.failed.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	a.InvalidateCacheForUser(user.ID)

	return nil
}

func (a *App) ResetPasswordFromToken(userSuppliedTokenString, newPassword string) *model.AppError {
	token, err := a.GetPasswordRecoveryToken(userSuppliedTokenString)
	if err != nil {
		return err
	}
	if model.GetMillis()-token.CreateAt >= PasswordRecoverExpiryTime {
		return model.NewAppError("resetPassword", "api.user.reset_password.link_expired.app_error", nil, "", http.StatusBadRequest)
	}

	tokenData := struct {
		UserID string
		Email  string
	}{}

	err2 := json.Unmarshal([]byte(token.Extra), &tokenData)
	if err2 != nil {
		return model.NewAppError("resetPassword", "api.user.reset_password.token_parse.error", nil, "", http.StatusInternalServerError)
	}

	user, err := a.GetUser(tokenData.UserID)
	if err != nil {
		return err
	}

	if user.Email != tokenData.Email {
		return model.NewAppError("resetPassword", "api.user.reset_password.link_expired.app_error", nil, "", http.StatusBadRequest)
	}

	if user.IsSSOUser() {
		return model.NewAppError("ResetPasswordFromCode", "api.user.reset_password.sso.app_error", nil, "userId="+user.ID, http.StatusBadRequest)
	}

	T := i18n.GetUserTranslations(user.Locale)

	if err := a.UpdatePasswordSendEmail(user, newPassword, T("api.user.reset_password.method")); err != nil {
		return err
	}

	if err := a.DeleteToken(token); err != nil {
		mlog.Warn("Failed to delete token", mlog.Err(err))
	}

	return nil
}

func (a *App) SendPasswordReset(email string, siteURL string) (bool, *model.AppError) {
	user, err := a.GetUserByEmail(email)
	if err != nil {
		return false, nil
	}

	if user.AuthData != nil && *user.AuthData != "" {
		return false, model.NewAppError("SendPasswordReset", "api.user.send_password_reset.sso.app_error", nil, "userId="+user.ID, http.StatusBadRequest)
	}

	token, err := a.CreatePasswordRecoveryToken(user.ID, user.Email)
	if err != nil {
		return false, err
	}

	return a.Srv().EmailService.SendPasswordResetEmail(user.Email, token, user.Locale, siteURL)
}

func (a *App) CreatePasswordRecoveryToken(userID, email string) (*model.Token, *model.AppError) {

	tokenExtra := struct {
		UserID string
		Email  string
	}{
		userID,
		email,
	}
	jsonData, err := json.Marshal(tokenExtra)

	if err != nil {
		return nil, model.NewAppError("CreatePasswordRecoveryToken", "api.user.create_password_token.error", nil, "", http.StatusInternalServerError)
	}

	token := model.NewToken(TokenTypePasswordRecovery, string(jsonData))

	if err := a.Srv().Store.Token().Save(token); err != nil {
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("CreatePasswordRecoveryToken", "app.recover.save.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return token, nil
}

func (a *App) GetPasswordRecoveryToken(token string) (*model.Token, *model.AppError) {
	rtoken, err := a.Srv().Store.Token().GetByToken(token)
	if err != nil {
		return nil, model.NewAppError("GetPasswordRecoveryToken", "api.user.reset_password.invalid_link.app_error", nil, err.Error(), http.StatusBadRequest)
	}
	if rtoken.Type != TokenTypePasswordRecovery {
		return nil, model.NewAppError("GetPasswordRecoveryToken", "api.user.reset_password.broken_token.app_error", nil, "", http.StatusBadRequest)
	}
	return rtoken, nil
}

func (a *App) DeleteToken(token *model.Token) *model.AppError {
	err := a.Srv().Store.Token().Delete(token.Token)
	if err != nil {
		return model.NewAppError("DeleteToken", "app.recover.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (a *App) UpdateUserRoles(userID string, newRoles string, sendWebSocketEvent bool) (*model.User, *model.AppError) {
	user, err := a.GetUser(userID)
	if err != nil {
		err.StatusCode = http.StatusBadRequest
		return nil, err
	}

	return a.UpdateUserRolesWithUser(user, newRoles, sendWebSocketEvent)
}

func (a *App) UpdateUserRolesWithUser(user *model.User, newRoles string, sendWebSocketEvent bool) (*model.User, *model.AppError) {

	if err := a.CheckRolesExist(strings.Fields(newRoles)); err != nil {
		return nil, err
	}

	user.Roles = newRoles
	uchan := make(chan store.StoreResult, 1)
	go func() {
		userUpdate, err := a.Srv().Store.User().Update(user, true)
		uchan <- store.StoreResult{Data: userUpdate, NErr: err}
		close(uchan)
	}()

	schan := make(chan store.StoreResult, 1)
	go func() {
		id, err := a.Srv().Store.Session().UpdateRoles(user.ID, newRoles)
		schan <- store.StoreResult{Data: id, NErr: err}
		close(schan)
	}()

	result := <-uchan
	if result.NErr != nil {
		var appErr *model.AppError
		var invErr *store.ErrInvalidInput
		switch {
		case errors.As(result.NErr, &appErr):
			return nil, appErr
		case errors.As(result.NErr, &invErr):
			return nil, model.NewAppError("UpdateUserRoles", "app.user.update.find.app_error", nil, invErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("UpdateUserRoles", "app.user.update.finding.app_error", nil, result.NErr.Error(), http.StatusInternalServerError)
		}
	}
	ruser := result.Data.(*model.UserUpdate).New

	if result := <-schan; result.NErr != nil {
		// soft error since the user roles were still updated
		mlog.Warn("Failed during updating user roles", mlog.Err(result.NErr))
	}

	a.InvalidateCacheForUser(user.ID)
	a.ClearSessionCacheForUser(user.ID)

	if sendWebSocketEvent {
		message := model.NewWebSocketEvent(model.WebsocketEventUserRoleUpdated, "", "", user.ID, nil)
		message.Add("user_id", user.ID)
		message.Add("roles", newRoles)
		a.Publish(message)
	}

	return ruser, nil
}

func (a *App) PermanentDeleteUser(c *request.Context, user *model.User) *model.AppError {
	mlog.Warn("Attempting to permanently delete account", mlog.String("user_id", user.ID), mlog.String("user_email", user.Email))
	if user.IsInRole(model.SystemAdminRoleID) {
		mlog.Warn("You are deleting a user that is a system administrator.  You may need to set another account as the system administrator using the command line tools.", mlog.String("user_email", user.Email))
	}

	if _, err := a.UpdateActive(c, user, false); err != nil {
		return err
	}

	if err := a.Srv().Store.Session().PermanentDeleteSessionsByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.session.permanent_delete_sessions_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.UserAccessToken().DeleteAllForUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.user_access_token.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.OAuth().PermanentDeleteAuthDataByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.oauth.permanent_delete_auth_data_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Webhook().PermanentDeleteIncomingByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.webhooks.permanent_delete_incoming_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Webhook().PermanentDeleteOutgoingByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.webhooks.permanent_delete_outgoing_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Command().PermanentDeleteByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.user.permanentdeleteuser.internal_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Preference().PermanentDeleteByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.preference.permanent_delete_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Channel().PermanentDeleteMembersByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.channel.permanent_delete_members_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Group().PermanentDeleteMembersByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.group.permanent_delete_members_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Post().PermanentDeleteByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.post.permanent_delete_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Bot().PermanentDelete(user.ID); err != nil {
		var invErr *store.ErrInvalidInput
		switch {
		case errors.As(err, &invErr):
			return model.NewAppError("PermanentDeleteUser", "app.bot.permenent_delete.bad_id", map[string]interface{}{"user_id": invErr.Value}, invErr.Error(), http.StatusBadRequest)
		default: // last fallback in case it doesn't map to an existing app error.
			return model.NewAppError("PermanentDeleteUser", "app.bot.permanent_delete.internal_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	infos, err := a.Srv().Store.FileInfo().GetForUser(user.ID)
	if err != nil {
		mlog.Warn("Error getting file list for user from FileInfoStore", mlog.Err(err))
	}

	for _, info := range infos {
		res, err := a.FileExists(info.Path)
		if err != nil {
			mlog.Warn(
				"Error checking existence of file",
				mlog.String("path", info.Path),
				mlog.Err(err),
			)
			continue
		}

		if !res {
			mlog.Warn("File not found", mlog.String("path", info.Path))
			continue
		}

		err = a.RemoveFile(info.Path)

		if err != nil {
			mlog.Warn(
				"Unable to remove file",
				mlog.String("path", info.Path),
				mlog.Err(err),
			)
		}
	}

	if _, err := a.Srv().Store.FileInfo().PermanentDeleteByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.file_info.permanent_delete_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.User().PermanentDelete(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.user.permanent_delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Audit().PermanentDeleteByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.audit.permanent_delete_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := a.Srv().Store.Team().RemoveAllMembersByUser(user.ID); err != nil {
		return model.NewAppError("PermanentDeleteUser", "app.team.remove_member.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	mlog.Warn("Permanently deleted account", mlog.String("user_email", user.Email), mlog.String("user_id", user.ID))

	return nil
}

func (a *App) PermanentDeleteAllUsers(c *request.Context) *model.AppError {
	users, err := a.Srv().Store.User().GetAll()
	if err != nil {
		return model.NewAppError("PermanentDeleteAllUsers", "app.user.get.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	for _, user := range users {
		a.PermanentDeleteUser(c, user)
	}

	return nil
}

func (a *App) SendEmailVerification(user *model.User, newEmail, redirect string) *model.AppError {
	token, err := a.Srv().EmailService.CreateVerifyEmailToken(user.ID, newEmail)
	if err != nil {
		return err
	}

	if _, err := a.GetStatus(user.ID); err != nil {
		return a.Srv().EmailService.sendVerifyEmail(newEmail, user.Locale, a.GetSiteURL(), token.Token, redirect)
	}
	return a.Srv().EmailService.sendEmailChangeVerifyEmail(newEmail, user.Locale, a.GetSiteURL(), token.Token)
}

func (a *App) VerifyEmailFromToken(userSuppliedTokenString string) *model.AppError {
	token, err := a.GetVerifyEmailToken(userSuppliedTokenString)
	if err != nil {
		return err
	}
	if model.GetMillis()-token.CreateAt >= PasswordRecoverExpiryTime {
		return model.NewAppError("VerifyEmailFromToken", "api.user.verify_email.link_expired.app_error", nil, "", http.StatusBadRequest)
	}

	tokenData := struct {
		UserID string
		Email  string
	}{}

	err2 := json.Unmarshal([]byte(token.Extra), &tokenData)
	if err2 != nil {
		return model.NewAppError("VerifyEmailFromToken", "api.user.verify_email.token_parse.error", nil, "", http.StatusInternalServerError)
	}

	user, err := a.GetUser(tokenData.UserID)
	if err != nil {
		return err
	}

	tokenData.Email = strings.ToLower(tokenData.Email)
	if err := a.VerifyUserEmail(tokenData.UserID, tokenData.Email); err != nil {
		return err
	}

	if user.Email != tokenData.Email {
		a.Srv().Go(func() {
			if err := a.Srv().EmailService.sendEmailChangeEmail(user.Email, tokenData.Email, user.Locale, a.GetSiteURL()); err != nil {
				mlog.Error("Failed to send email change email", mlog.Err(err))
			}
		})
	}

	if err := a.DeleteToken(token); err != nil {
		mlog.Warn("Failed to delete token", mlog.Err(err))
	}

	return nil
}

func (a *App) GetVerifyEmailToken(token string) (*model.Token, *model.AppError) {
	rtoken, err := a.Srv().Store.Token().GetByToken(token)
	if err != nil {
		return nil, model.NewAppError("GetVerifyEmailToken", "api.user.verify_email.bad_link.app_error", nil, err.Error(), http.StatusBadRequest)
	}
	if rtoken.Type != TokenTypeVerifyEmail {
		return nil, model.NewAppError("GetVerifyEmailToken", "api.user.verify_email.broken_token.app_error", nil, "", http.StatusBadRequest)
	}
	return rtoken, nil
}

// GetTotalUsersStats is used for the DM list total
func (a *App) GetTotalUsersStats(viewRestrictions *model.ViewUsersRestrictions) (*model.UsersStats, *model.AppError) {
	count, err := a.Srv().Store.User().Count(model.UserCountOptions{
		IncludeBotAccounts: true,
		ViewRestrictions:   viewRestrictions,
	})
	if err != nil {
		return nil, model.NewAppError("GetTotalUsersStats", "app.user.get_total_users_count.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	stats := &model.UsersStats{
		TotalUsersCount: count,
	}
	return stats, nil
}

// GetFilteredUsersStats is used to get a count of users based on the set of filters supported by UserCountOptions.
func (a *App) GetFilteredUsersStats(options *model.UserCountOptions) (*model.UsersStats, *model.AppError) {
	count, err := a.Srv().Store.User().Count(*options)
	if err != nil {
		return nil, model.NewAppError("GetFilteredUsersStats", "app.user.get_total_users_count.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	stats := &model.UsersStats{
		TotalUsersCount: count,
	}
	return stats, nil
}

func (a *App) VerifyUserEmail(userID, email string) *model.AppError {
	if _, err := a.Srv().Store.User().VerifyEmail(userID, email); err != nil {
		return model.NewAppError("VerifyUserEmail", "app.user.verify_email.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	a.InvalidateCacheForUser(userID)

	user, err := a.GetUser(userID)

	if err != nil {
		return err
	}

	a.sendUpdatedUserEvent(*user)

	return nil
}

func (a *App) SearchUsers(props *model.UserSearch, options *model.UserSearchOptions) ([]*model.User, *model.AppError) {
	if props.WithoutTeam {
		return a.SearchUsersWithoutTeam(props.Term, options)
	}
	if props.InChannelID != "" {
		return a.SearchUsersInChannel(props.InChannelID, props.Term, options)
	}
	if props.NotInChannelID != "" {
		return a.SearchUsersNotInChannel(props.TeamID, props.NotInChannelID, props.Term, options)
	}
	if props.NotInTeamID != "" {
		return a.SearchUsersNotInTeam(props.NotInTeamID, props.Term, options)
	}
	if props.InGroupID != "" {
		return a.SearchUsersInGroup(props.InGroupID, props.Term, options)
	}
	return a.SearchUsersInTeam(props.TeamID, props.Term, options)
}

func (a *App) SearchUsersInChannel(channelID string, term string, options *model.UserSearchOptions) ([]*model.User, *model.AppError) {
	term = strings.TrimSpace(term)
	users, err := a.Srv().Store.User().SearchInChannel(channelID, term, options)
	if err != nil {
		return nil, model.NewAppError("SearchUsersInChannel", "app.user.search.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	for _, user := range users {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	return users, nil
}

func (a *App) SearchUsersNotInChannel(teamID string, channelID string, term string, options *model.UserSearchOptions) ([]*model.User, *model.AppError) {
	term = strings.TrimSpace(term)
	users, err := a.Srv().Store.User().SearchNotInChannel(teamID, channelID, term, options)
	if err != nil {
		return nil, model.NewAppError("SearchUsersNotInChannel", "app.user.search.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, user := range users {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	return users, nil
}

func (a *App) SearchUsersInTeam(teamID, term string, options *model.UserSearchOptions) ([]*model.User, *model.AppError) {
	term = strings.TrimSpace(term)

	users, err := a.Srv().Store.User().Search(teamID, term, options)
	if err != nil {
		return nil, model.NewAppError("SearchUsersInTeam", "app.user.search.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, user := range users {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	return users, nil
}

func (a *App) SearchUsersNotInTeam(notInTeamID string, term string, options *model.UserSearchOptions) ([]*model.User, *model.AppError) {
	term = strings.TrimSpace(term)
	users, err := a.Srv().Store.User().SearchNotInTeam(notInTeamID, term, options)
	if err != nil {
		return nil, model.NewAppError("SearchUsersNotInTeam", "app.user.search.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, user := range users {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	return users, nil
}

func (a *App) SearchUsersWithoutTeam(term string, options *model.UserSearchOptions) ([]*model.User, *model.AppError) {
	term = strings.TrimSpace(term)
	users, err := a.Srv().Store.User().SearchWithoutTeam(term, options)
	if err != nil {
		return nil, model.NewAppError("SearchUsersWithoutTeam", "app.user.search.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, user := range users {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	return users, nil
}

func (a *App) SearchUsersInGroup(groupID string, term string, options *model.UserSearchOptions) ([]*model.User, *model.AppError) {
	term = strings.TrimSpace(term)
	users, err := a.Srv().Store.User().SearchInGroup(groupID, term, options)
	if err != nil {
		return nil, model.NewAppError("SearchUsersInGroup", "app.user.search.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, user := range users {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	return users, nil
}

func (a *App) AutocompleteUsersInChannel(teamID string, channelID string, term string, options *model.UserSearchOptions) (*model.UserAutocompleteInChannel, *model.AppError) {
	term = strings.TrimSpace(term)

	autocomplete, err := a.Srv().Store.User().AutocompleteUsersInChannel(teamID, channelID, term, options)
	if err != nil {
		return nil, model.NewAppError("AutocompleteUsersInChannel", "app.user.search.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, user := range autocomplete.InChannel {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	for _, user := range autocomplete.OutOfChannel {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	return autocomplete, nil
}

func (a *App) AutocompleteUsersInTeam(teamID string, term string, options *model.UserSearchOptions) (*model.UserAutocompleteInTeam, *model.AppError) {
	term = strings.TrimSpace(term)

	users, err := a.Srv().Store.User().Search(teamID, term, options)
	if err != nil {
		return nil, model.NewAppError("AutocompleteUsersInTeam", "app.user.search.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, user := range users {
		a.SanitizeProfile(user, options.IsAdmin)
	}

	autocomplete := &model.UserAutocompleteInTeam{}
	autocomplete.InTeam = users
	return autocomplete, nil
}

func (a *App) UpdateOAuthUserAttrs(userData io.Reader, user *model.User, provider einterfaces.OAuthProvider, service string, tokenUser *model.User) *model.AppError {
	oauthUser, err1 := provider.GetUserFromJSON(userData, tokenUser)
	if err1 != nil {
		return model.NewAppError("UpdateOAuthUserAttrs", "api.user.update_oauth_user_attrs.get_user.app_error", map[string]interface{}{"Service": service}, err1.Error(), http.StatusBadRequest)
	}

	userAttrsChanged := false

	if oauthUser.Username != user.Username {
		if existingUser, _ := a.GetUserByUsername(oauthUser.Username); existingUser == nil {
			user.Username = oauthUser.Username
			userAttrsChanged = true
		}
	}

	if oauthUser.GetFullName() != user.GetFullName() {
		user.FirstName = oauthUser.FirstName
		user.LastName = oauthUser.LastName
		userAttrsChanged = true
	}

	if oauthUser.Email != user.Email {
		if existingUser, _ := a.GetUserByEmail(oauthUser.Email); existingUser == nil {
			user.Email = oauthUser.Email
			userAttrsChanged = true
		}
	}

	if user.DeleteAt > 0 {
		// Make sure they are not disabled
		user.DeleteAt = 0
		userAttrsChanged = true
	}

	if userAttrsChanged {
		users, err := a.Srv().Store.User().Update(user, true)
		if err != nil {
			var appErr *model.AppError
			var invErr *store.ErrInvalidInput
			switch {
			case errors.As(err, &appErr):
				return appErr
			case errors.As(err, &invErr):
				return model.NewAppError("UpdateOAuthUserAttrs", "app.user.update.find.app_error", nil, invErr.Error(), http.StatusBadRequest)
			default:
				return model.NewAppError("UpdateOAuthUserAttrs", "app.user.update.finding.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}

		user = users.New
		a.InvalidateCacheForUser(user.ID)
	}

	return nil
}

func (a *App) RestrictUsersGetByPermissions(userID string, options *model.UserGetOptions) (*model.UserGetOptions, *model.AppError) {
	restrictions, err := a.GetViewUsersRestrictions(userID)
	if err != nil {
		return nil, err
	}

	options.ViewRestrictions = restrictions
	return options, nil
}

// FilterNonGroupTeamMembers returns the subset of the given user IDs of the users who are not members of groups
// associated to the team excluding bots.
func (a *App) FilterNonGroupTeamMembers(userIDs []string, team *model.Team) ([]string, error) {
	teamGroupUsers, err := a.GetTeamGroupUsers(team.ID)
	if err != nil {
		return nil, err
	}
	return a.filterNonGroupUsers(userIDs, teamGroupUsers)
}

// FilterNonGroupChannelMembers returns the subset of the given user IDs of the users who are not members of groups
// associated to the channel excluding bots
func (a *App) FilterNonGroupChannelMembers(userIDs []string, channel *model.Channel) ([]string, error) {
	channelGroupUsers, err := a.GetChannelGroupUsers(channel.ID)
	if err != nil {
		return nil, err
	}
	return a.filterNonGroupUsers(userIDs, channelGroupUsers)
}

// filterNonGroupUsers is a helper function that takes a list of user ids and a list of users
// and returns the list of normal users present in userIDs but not in groupUsers.
func (a *App) filterNonGroupUsers(userIDs []string, groupUsers []*model.User) ([]string, error) {
	nonMemberIDs := []string{}
	users, err := a.Srv().Store.User().GetProfileByIDs(context.Background(), userIDs, nil, false)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		userIsMember := user.IsBot

		for _, pu := range groupUsers {
			if pu.ID == user.ID {
				userIsMember = true
				break
			}
		}
		if !userIsMember {
			nonMemberIDs = append(nonMemberIDs, user.ID)
		}
	}

	return nonMemberIDs, nil
}

func (a *App) RestrictUsersSearchByPermissions(userID string, options *model.UserSearchOptions) (*model.UserSearchOptions, *model.AppError) {
	restrictions, err := a.GetViewUsersRestrictions(userID)
	if err != nil {
		return nil, err
	}

	options.ViewRestrictions = restrictions
	return options, nil
}

func (a *App) UserCanSeeOtherUser(userID string, otherUserID string) (bool, *model.AppError) {
	if userID == otherUserID {
		return true, nil
	}

	restrictions, err := a.GetViewUsersRestrictions(userID)
	if err != nil {
		return false, err
	}

	if restrictions == nil {
		return true, nil
	}

	if len(restrictions.Teams) > 0 {
		result, err := a.Srv().Store.Team().UserBelongsToTeams(otherUserID, restrictions.Teams)
		if err != nil {
			return false, model.NewAppError("UserCanSeeOtherUser", "app.team.user_belongs_to_teams.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
		if result {
			return true, nil
		}
	}

	if len(restrictions.Channels) > 0 {
		result, err := a.userBelongsToChannels(otherUserID, restrictions.Channels)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}
	}

	return false, nil
}

func (a *App) userBelongsToChannels(userID string, channelIDs []string) (bool, *model.AppError) {
	belongs, err := a.Srv().Store.Channel().UserBelongsToChannels(userID, channelIDs)
	if err != nil {
		return false, model.NewAppError("userBelongsToChannels", "app.channel.user_belongs_to_channels.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return belongs, nil
}

func (a *App) GetViewUsersRestrictions(userID string) (*model.ViewUsersRestrictions, *model.AppError) {
	if a.HasPermissionTo(userID, model.PermissionViewMembers) {
		return nil, nil
	}

	teamIDs, nErr := a.Srv().Store.Team().GetUserTeamIDs(userID, true)
	if nErr != nil {
		return nil, model.NewAppError("GetViewUsersRestrictions", "app.team.get_user_team_ids.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	teamIDsWithPermission := []string{}
	for _, teamID := range teamIDs {
		if a.HasPermissionToTeam(userID, teamID, model.PermissionViewMembers) {
			teamIDsWithPermission = append(teamIDsWithPermission, teamID)
		}
	}

	userChannelMembers, err := a.Srv().Store.Channel().GetAllChannelMembersForUser(userID, true, true)
	if err != nil {
		return nil, model.NewAppError("GetViewUsersRestrictions", "app.channel.get_channels.get.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	channelIDs := []string{}
	for channelID := range userChannelMembers {
		channelIDs = append(channelIDs, channelID)
	}

	return &model.ViewUsersRestrictions{Teams: teamIDsWithPermission, Channels: channelIDs}, nil
}

// PromoteGuestToUser Convert user's roles and all his mermbership's roles from
// guest roles to regular user roles.
func (a *App) PromoteGuestToUser(c *request.Context, user *model.User, requestorID string) *model.AppError {
	nErr := a.Srv().Store.User().PromoteGuestToUser(user.ID)
	a.InvalidateCacheForUser(user.ID)
	if nErr != nil {
		return model.NewAppError("PromoteGuestToUser", "app.user.promote_guest.user_update.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	userTeams, nErr := a.Srv().Store.Team().GetTeamsByUserID(user.ID)
	if nErr != nil {
		return model.NewAppError("PromoteGuestToUser", "app.team.get_all.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	for _, team := range userTeams {
		// Soft error if there is an issue joining the default channels
		if err := a.JoinDefaultChannels(c, team.ID, user, false, requestorID); err != nil {
			mlog.Warn("Failed to join default channels", mlog.String("user_id", user.ID), mlog.String("team_id", team.ID), mlog.String("requestor_id", requestorID), mlog.Err(err))
		}
	}

	promotedUser, err := a.GetUser(user.ID)
	if err != nil {
		mlog.Warn("Failed to get user on promote guest to user", mlog.Err(err))
	} else {
		a.sendUpdatedUserEvent(*promotedUser)
		if uErr := a.srv.userService.UpdateSessionsIsGuest(promotedUser.ID, promotedUser.IsGuest()); uErr != nil {
			mlog.Warn("Unable to update user sessions", mlog.String("user_id", promotedUser.ID), mlog.Err(uErr))
		}
	}

	teamMembers, err := a.GetTeamMembersForUser(user.ID)
	if err != nil {
		mlog.Warn("Failed to get team members for user on promote guest to user", mlog.Err(err))
	}

	for _, member := range teamMembers {
		a.sendUpdatedMemberRoleEvent(user.ID, member)

		channelMembers, err := a.GetChannelMembersForUser(member.TeamID, user.ID)
		if err != nil {
			mlog.Warn("Failed to get channel members for user on promote guest to user", mlog.Err(err))
		}

		for _, member := range *channelMembers {
			a.invalidateCacheForChannelMembers(member.ChannelID)

			evt := model.NewWebSocketEvent(model.WebsocketEventChannelMemberUpdated, "", "", user.ID, nil)
			evt.Add("channelMember", member.ToJSON())
			a.Publish(evt)
		}
	}

	a.ClearSessionCacheForUser(user.ID)
	return nil
}

// DemoteUserToGuest Convert user's roles and all his mermbership's roles from
// regular user roles to guest roles.
func (a *App) DemoteUserToGuest(user *model.User) *model.AppError {
	demotedUser, nErr := a.Srv().Store.User().DemoteUserToGuest(user.ID)
	a.InvalidateCacheForUser(user.ID)
	if nErr != nil {
		return model.NewAppError("DemoteUserToGuest", "app.user.demote_user_to_guest.user_update.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	a.sendUpdatedUserEvent(*demotedUser)
	if uErr := a.srv.userService.UpdateSessionsIsGuest(demotedUser.ID, demotedUser.IsGuest()); uErr != nil {
		mlog.Warn("Unable to update user sessions", mlog.String("user_id", demotedUser.ID), mlog.Err(uErr))
	}

	teamMembers, err := a.GetTeamMembersForUser(user.ID)
	if err != nil {
		mlog.Warn("Failed to get team members for users on demote user to guest", mlog.Err(err))
	}

	for _, member := range teamMembers {
		a.sendUpdatedMemberRoleEvent(user.ID, member)

		channelMembers, err := a.GetChannelMembersForUser(member.TeamID, user.ID)
		if err != nil {
			mlog.Warn("Failed to get channel members for users on demote user to guest", mlog.Err(err))
			continue
		}

		for _, member := range *channelMembers {
			a.invalidateCacheForChannelMembers(member.ChannelID)

			evt := model.NewWebSocketEvent(model.WebsocketEventChannelMemberUpdated, "", "", user.ID, nil)
			evt.Add("channelMember", member.ToJSON())
			a.Publish(evt)
		}
	}

	a.ClearSessionCacheForUser(user.ID)
	return nil
}

func (a *App) PublishUserTyping(userID, channelID, parentID string) *model.AppError {
	omitUsers := make(map[string]bool, 1)
	omitUsers[userID] = true

	event := model.NewWebSocketEvent(model.WebsocketEventTyping, "", channelID, "", omitUsers)
	event.Add("parent_id", parentID)
	event.Add("user_id", userID)
	a.Publish(event)

	return nil
}

// invalidateUserCacheAndPublish Invalidates cache for a user and publishes user updated event
func (a *App) invalidateUserCacheAndPublish(userID string) {
	a.InvalidateCacheForUser(userID)

	user, userErr := a.GetUser(userID)
	if userErr != nil {
		mlog.Error("Error in getting users profile", mlog.String("user_id", userID), mlog.Err(userErr))
		return
	}

	options := a.Config().GetSanitizeOptions()
	user.SanitizeProfile(options)

	message := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", "", nil)
	message.Add("user", user)
	a.Publish(message)
}

// GetKnownUsers returns the list of user ids of users with any direct
// relationship with a user. That means any user sharing any channel, including
// direct and group channels.
func (a *App) GetKnownUsers(userID string) ([]string, *model.AppError) {
	users, err := a.Srv().Store.User().GetKnownUsers(userID)
	if err != nil {
		return nil, model.NewAppError("GetKnownUsers", "app.user.get_known_users.get_users.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return users, nil
}

// ConvertBotToUser converts a bot to user.
func (a *App) ConvertBotToUser(bot *model.Bot, userPatch *model.UserPatch, sysadmin bool) (*model.User, *model.AppError) {
	user, nErr := a.Srv().Store.User().Get(context.Background(), bot.UserID)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(nErr, &nfErr):
			return nil, model.NewAppError("ConvertBotToUser", MissingAccountError, nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("ConvertBotToUser", "app.user.get.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	if sysadmin && !user.IsInRole(model.SystemAdminRoleID) {
		_, appErr := a.UpdateUserRoles(
			user.ID,
			fmt.Sprintf("%s %s", user.Roles, model.SystemAdminRoleID),
			false)
		if appErr != nil {
			return nil, appErr
		}
	}

	user.Patch(userPatch)

	user, err := a.UpdateUser(user, false)
	if err != nil {
		return nil, err
	}

	err = a.UpdatePassword(user, *userPatch.Password)
	if err != nil {
		return nil, err
	}

	appErr := a.Srv().Store.Bot().PermanentDelete(bot.UserID)
	if appErr != nil {
		return nil, model.NewAppError("ConvertBotToUser", "app.user.convert_bot_to_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return user, nil
}

func (a *App) GetThreadsForUser(userID, teamID string, options model.GetUserThreadsOpts) (*model.Threads, *model.AppError) {
	threads, err := a.Srv().Store.Thread().GetThreadsForUser(userID, teamID, options)
	if err != nil {
		return nil, model.NewAppError("GetThreadsForUser", "app.user.get_threads_for_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	for _, thread := range threads.Threads {
		a.sanitizeProfiles(thread.Participants, false)
		thread.Post.SanitizeProps()
	}
	return threads, nil
}

func (a *App) GetThreadMembershipForUser(userID, threadID string) (*model.ThreadMembership, *model.AppError) {
	threadMembership, err := a.Srv().Store.Thread().GetMembershipForUser(userID, threadID)
	if err != nil {
		return nil, model.NewAppError("GetThreadMembershipForUser", "app.user.get_thread_membership_for_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	if threadMembership == nil {
		return nil, model.NewAppError("GetThreadMembershipForUser", "app.user.get_thread_membership_for_user.not_found", nil, "thread membership not found/followed", http.StatusNotFound)
	}
	return threadMembership, nil
}

func (a *App) GetThreadForUser(teamID string, threadMembership *model.ThreadMembership, extended bool) (*model.ThreadResponse, *model.AppError) {
	thread, err := a.Srv().Store.Thread().GetThreadForUser(teamID, threadMembership, extended)
	if err != nil {
		return nil, model.NewAppError("GetThreadForUser", "app.user.get_threads_for_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	if thread == nil {
		return nil, model.NewAppError("GetThreadForUser", "app.user.get_threads_for_user.not_found", nil, "thread not found/followed", http.StatusNotFound)
	}
	a.sanitizeProfiles(thread.Participants, false)
	thread.Post.SanitizeProps()
	return thread, nil
}

func (a *App) UpdateThreadsReadForUser(userID, teamID string) *model.AppError {
	nErr := a.Srv().Store.Thread().MarkAllAsRead(userID, teamID)
	if nErr != nil {
		return model.NewAppError("UpdateThreadsReadForUser", "app.user.update_threads_read_for_user.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	message := model.NewWebSocketEvent(model.WebsocketEventThreadReadChanged, teamID, "", userID, nil)
	a.Publish(message)
	return nil
}

func (a *App) UpdateThreadFollowForUser(userID, teamID, threadID string, state bool) *model.AppError {
	opts := store.ThreadMembershipOpts{
		Following:             state,
		IncrementMentions:     false,
		UpdateFollowing:       true,
		UpdateViewedTimestamp: state,
		UpdateParticipants:    false,
	}
	_, err := a.Srv().Store.Thread().MaintainMembership(userID, threadID, opts)
	if err != nil {
		return model.NewAppError("UpdateThreadFollowForUser", "app.user.update_thread_follow_for_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	thread, err := a.Srv().Store.Thread().Get(threadID)
	if err != nil {
		return model.NewAppError("UpdateThreadFollowForUser", "app.user.update_thread_follow_for_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	replyCount := int64(0)
	if thread != nil {
		replyCount = thread.ReplyCount
	}
	message := model.NewWebSocketEvent(model.WebsocketEventThreadFollowChanged, teamID, "", userID, nil)
	message.Add("thread_id", threadID)
	message.Add("state", state)
	message.Add("reply_count", replyCount)
	a.Publish(message)
	return nil
}

func (a *App) UpdateThreadReadForUser(userID, teamID, threadID string, timestamp int64) (*model.ThreadResponse, *model.AppError) {
	user, err := a.GetUser(userID)
	if err != nil {
		return nil, err
	}

	opts := store.ThreadMembershipOpts{
		Following:       true,
		UpdateFollowing: false,
	}
	membership, storeErr := a.Srv().Store.Thread().MaintainMembership(userID, threadID, opts)
	if storeErr != nil {
		return nil, model.NewAppError("UpdateThreadReadForUser", "app.user.update_thread_read_for_user.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	post, err := a.GetSinglePost(threadID)
	if err != nil {
		return nil, err
	}
	membership.UnreadMentions, err = a.countThreadMentions(user, post, teamID, timestamp)
	if err != nil {
		return nil, err
	}
	membership.Following = true
	_, nErr := a.Srv().Store.Thread().UpdateMembership(membership)
	if nErr != nil {
		return nil, model.NewAppError("UpdateThreadReadForUser", "app.user.update_thread_read_for_user.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	membership.LastViewed = timestamp
	nErr = a.Srv().Store.Thread().MarkAsRead(userID, threadID, timestamp)
	if nErr != nil {
		return nil, model.NewAppError("UpdateThreadReadForUser", "app.user.update_thread_read_for_user.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	thread, err := a.GetThreadForUser(teamID, membership, false)
	if err != nil {
		return nil, err
	}

	message := model.NewWebSocketEvent(model.WebsocketEventThreadReadChanged, teamID, "", userID, nil)
	message.Add("thread_id", threadID)
	message.Add("timestamp", timestamp)
	message.Add("unread_mentions", membership.UnreadMentions)
	message.Add("unread_replies", thread.UnreadReplies)
	message.Add("channel_id", post.ChannelID)
	a.Publish(message)
	return thread, nil
}
