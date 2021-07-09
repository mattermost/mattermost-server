// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/users"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

//
// -- Bulk Import Functions --
// These functions import data directly into the database. Security and permission checks are bypassed but validity is
// still enforced.
//

func (a *App) importScheme(data *SchemeImportData, dryRun bool) *model.AppError {
	if err := validateSchemeImportData(data); err != nil {
		return err
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return nil
	}

	scheme, err := a.GetSchemeByName(*data.Name)
	if err != nil {
		scheme = new(model.Scheme)
	} else if scheme.Scope != *data.Scope {
		return model.NewAppError("BulkImport", "app.import.import_scheme.scope_change.error", map[string]interface{}{"SchemeName": scheme.Name}, "", http.StatusBadRequest)
	}

	scheme.Name = *data.Name
	scheme.DisplayName = *data.DisplayName
	scheme.Scope = *data.Scope

	if data.Description != nil {
		scheme.Description = *data.Description
	}

	if scheme.ID == "" {
		scheme, err = a.CreateScheme(scheme)
	} else {
		scheme, err = a.UpdateScheme(scheme)
	}

	if err != nil {
		return err
	}

	if scheme.Scope == model.SchemeScopeTeam {
		data.DefaultTeamAdminRole.Name = &scheme.DefaultTeamAdminRole
		if err := a.importRole(data.DefaultTeamAdminRole, dryRun, true); err != nil {
			return err
		}

		data.DefaultTeamUserRole.Name = &scheme.DefaultTeamUserRole
		if err := a.importRole(data.DefaultTeamUserRole, dryRun, true); err != nil {
			return err
		}

		if data.DefaultTeamGuestRole == nil {
			data.DefaultTeamGuestRole = &RoleImportData{
				DisplayName: model.NewString("Team Guest Role for Scheme"),
			}
		}
		data.DefaultTeamGuestRole.Name = &scheme.DefaultTeamGuestRole
		if err := a.importRole(data.DefaultTeamGuestRole, dryRun, true); err != nil {
			return err
		}
	}

	if scheme.Scope == model.SchemeScopeTeam || scheme.Scope == model.SchemeScopeChannel {
		data.DefaultChannelAdminRole.Name = &scheme.DefaultChannelAdminRole
		if err := a.importRole(data.DefaultChannelAdminRole, dryRun, true); err != nil {
			return err
		}

		data.DefaultChannelUserRole.Name = &scheme.DefaultChannelUserRole
		if err := a.importRole(data.DefaultChannelUserRole, dryRun, true); err != nil {
			return err
		}

		if data.DefaultChannelGuestRole == nil {
			data.DefaultChannelGuestRole = &RoleImportData{
				DisplayName: model.NewString("Channel Guest Role for Scheme"),
			}
		}
		data.DefaultChannelGuestRole.Name = &scheme.DefaultChannelGuestRole
		if err := a.importRole(data.DefaultChannelGuestRole, dryRun, true); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) importRole(data *RoleImportData, dryRun bool, isSchemeRole bool) *model.AppError {
	if !isSchemeRole {
		if err := validateRoleImportData(data); err != nil {
			return err
		}
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return nil
	}

	role, err := a.GetRoleByName(context.Background(), *data.Name)
	if err != nil {
		role = new(model.Role)
	}

	role.Name = *data.Name

	if data.DisplayName != nil {
		role.DisplayName = *data.DisplayName
	}

	if data.Description != nil {
		role.Description = *data.Description
	}

	if data.Permissions != nil {
		role.Permissions = *data.Permissions
	}

	if isSchemeRole {
		role.SchemeManaged = true
	} else {
		role.SchemeManaged = false
	}

	if role.ID == "" {
		_, err = a.CreateRole(role)
	} else {
		_, err = a.UpdateRole(role)
	}

	return err
}

func (a *App) importTeam(c *request.Context, data *TeamImportData, dryRun bool) *model.AppError {
	if err := validateTeamImportData(data); err != nil {
		return err
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return nil
	}

	var team *model.Team
	team, err := a.Srv().Store.Team().GetByName(*data.Name)

	if err != nil {
		team = &model.Team{}
	}

	team.Name = *data.Name
	team.DisplayName = *data.DisplayName
	team.Type = *data.Type

	if data.Description != nil {
		team.Description = *data.Description
	}

	if data.AllowOpenInvite != nil {
		team.AllowOpenInvite = *data.AllowOpenInvite
	}

	if data.Scheme != nil {
		scheme, err := a.GetSchemeByName(*data.Scheme)
		if err != nil {
			return err
		}

		if scheme.DeleteAt != 0 {
			return model.NewAppError("BulkImport", "app.import.import_team.scheme_deleted.error", nil, "", http.StatusBadRequest)
		}

		if scheme.Scope != model.SchemeScopeTeam {
			return model.NewAppError("BulkImport", "app.import.import_team.scheme_wrong_scope.error", nil, "", http.StatusBadRequest)
		}

		team.SchemeID = &scheme.ID
	}

	if team.ID == "" {
		if _, err := a.CreateTeam(c, team); err != nil {
			return err
		}
	} else {
		if _, err := a.updateTeamUnsanitized(team); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) importChannel(c *request.Context, data *ChannelImportData, dryRun bool) *model.AppError {
	if err := validateChannelImportData(data); err != nil {
		return err
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return nil
	}

	team, err := a.Srv().Store.Team().GetByName(*data.Team)
	if err != nil {
		return model.NewAppError("BulkImport", "app.import.import_channel.team_not_found.error", map[string]interface{}{"TeamName": *data.Team}, err.Error(), http.StatusBadRequest)
	}

	var channel *model.Channel
	if result, err := a.Srv().Store.Channel().GetByNameIncludeDeleted(team.ID, *data.Name, true); err == nil {
		channel = result
	} else {
		channel = &model.Channel{}
	}

	channel.TeamID = team.ID
	channel.Name = *data.Name
	channel.DisplayName = *data.DisplayName
	channel.Type = *data.Type

	if data.Header != nil {
		channel.Header = *data.Header
	}

	if data.Purpose != nil {
		channel.Purpose = *data.Purpose
	}

	if data.Scheme != nil {
		scheme, err := a.GetSchemeByName(*data.Scheme)
		if err != nil {
			return err
		}

		if scheme.DeleteAt != 0 {
			return model.NewAppError("BulkImport", "app.import.import_channel.scheme_deleted.error", nil, "", http.StatusBadRequest)
		}

		if scheme.Scope != model.SchemeScopeChannel {
			return model.NewAppError("BulkImport", "app.import.import_channel.scheme_wrong_scope.error", nil, "", http.StatusBadRequest)
		}

		channel.SchemeID = &scheme.ID
	}

	if channel.ID == "" {
		if _, err := a.CreateChannel(c, channel, false); err != nil {
			return err
		}
	} else {
		if _, err := a.UpdateChannel(channel); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) importUser(data *UserImportData, dryRun bool) *model.AppError {
	if err := validateUserImportData(data); err != nil {
		return err
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return nil
	}

	// We want to avoid database writes if nothing has changed.
	hasUserChanged := false
	hasNotifyPropsChanged := false
	hasUserRolesChanged := false
	hasUserAuthDataChanged := false
	hasUserEmailVerifiedChanged := false

	var user *model.User
	var nErr error
	user, nErr = a.Srv().Store.User().GetByUsername(*data.Username)
	if nErr != nil {
		user = &model.User{}
		user.MakeNonNil()
		user.SetDefaultNotifications()
		hasUserChanged = true
	}

	user.Username = *data.Username

	if user.Email != *data.Email {
		hasUserChanged = true
		hasUserEmailVerifiedChanged = true // Changing the email resets email verified to false by default.
		user.Email = *data.Email
		user.Email = strings.ToLower(user.Email)
	}

	var password string
	var authService string
	var authData *string

	if data.AuthService != nil {
		if user.AuthService != *data.AuthService {
			hasUserAuthDataChanged = true
		}
		authService = *data.AuthService
	}

	// AuthData and Password are mutually exclusive.
	if data.AuthData != nil {
		if user.AuthData == nil || *user.AuthData != *data.AuthData {
			hasUserAuthDataChanged = true
		}
		authData = data.AuthData
		password = ""
	} else if data.Password != nil {
		password = *data.Password
		authData = nil
	} else {
		var err error
		// If no AuthData or Password is specified, we must generate a password.
		password, err = generatePassword(*a.Config().PasswordSettings.MinimumLength)
		if err != nil {
			return model.NewAppError("importUser", "app.import.generate_password.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
		authData = nil
	}

	user.Password = password
	user.AuthService = authService
	user.AuthData = authData

	// Automatically assume all emails are verified.
	emailVerified := true
	if user.EmailVerified != emailVerified {
		user.EmailVerified = emailVerified
		hasUserEmailVerifiedChanged = true
	}

	if data.Nickname != nil {
		if user.Nickname != *data.Nickname {
			user.Nickname = *data.Nickname
			hasUserChanged = true
		}
	}

	if data.FirstName != nil {
		if user.FirstName != *data.FirstName {
			user.FirstName = *data.FirstName
			hasUserChanged = true
		}
	}

	if data.LastName != nil {
		if user.LastName != *data.LastName {
			user.LastName = *data.LastName
			hasUserChanged = true
		}
	}

	if data.Position != nil {
		if user.Position != *data.Position {
			user.Position = *data.Position
			hasUserChanged = true
		}
	}

	if data.Locale != nil {
		if user.Locale != *data.Locale {
			user.Locale = *data.Locale
			hasUserChanged = true
		}
	} else {
		if user.Locale != *a.Config().LocalizationSettings.DefaultClientLocale {
			user.Locale = *a.Config().LocalizationSettings.DefaultClientLocale
			hasUserChanged = true
		}
	}

	if data.DeleteAt != nil {
		if user.DeleteAt != *data.DeleteAt {
			user.DeleteAt = *data.DeleteAt
			hasUserChanged = true
		}
	}

	var roles string
	if data.Roles != nil {
		if user.Roles != *data.Roles {
			roles = *data.Roles
			hasUserRolesChanged = true
		}
	} else if user.Roles == "" {
		// Set SYSTEM_USER roles on newly created users by default.
		if user.Roles != model.SystemUserRoleID {
			roles = model.SystemUserRoleID
			hasUserRolesChanged = true
		}
	}
	user.Roles = roles

	if data.NotifyProps != nil {
		if data.NotifyProps.Desktop != nil {
			if value, ok := user.NotifyProps[model.DesktopNotifyProp]; !ok || value != *data.NotifyProps.Desktop {
				user.AddNotifyProp(model.DesktopNotifyProp, *data.NotifyProps.Desktop)
				hasNotifyPropsChanged = true
			}
		}

		if data.NotifyProps.DesktopSound != nil {
			if value, ok := user.NotifyProps[model.DesktopSoundNotifyProp]; !ok || value != *data.NotifyProps.DesktopSound {
				user.AddNotifyProp(model.DesktopSoundNotifyProp, *data.NotifyProps.DesktopSound)
				hasNotifyPropsChanged = true
			}
		}

		if data.NotifyProps.Email != nil {
			if value, ok := user.NotifyProps[model.EmailNotifyProp]; !ok || value != *data.NotifyProps.Email {
				user.AddNotifyProp(model.EmailNotifyProp, *data.NotifyProps.Email)
				hasNotifyPropsChanged = true
			}
		}

		if data.NotifyProps.Mobile != nil {
			if value, ok := user.NotifyProps[model.PushNotifyProp]; !ok || value != *data.NotifyProps.Mobile {
				user.AddNotifyProp(model.PushNotifyProp, *data.NotifyProps.Mobile)
				hasNotifyPropsChanged = true
			}
		}

		if data.NotifyProps.MobilePushStatus != nil {
			if value, ok := user.NotifyProps[model.PushStatusNotifyProp]; !ok || value != *data.NotifyProps.MobilePushStatus {
				user.AddNotifyProp(model.PushStatusNotifyProp, *data.NotifyProps.MobilePushStatus)
				hasNotifyPropsChanged = true
			}
		}

		if data.NotifyProps.ChannelTrigger != nil {
			if value, ok := user.NotifyProps[model.ChannelMentionsNotifyProp]; !ok || value != *data.NotifyProps.ChannelTrigger {
				user.AddNotifyProp(model.ChannelMentionsNotifyProp, *data.NotifyProps.ChannelTrigger)
				hasNotifyPropsChanged = true
			}
		}

		if data.NotifyProps.CommentsTrigger != nil {
			if value, ok := user.NotifyProps[model.CommentsNotifyProp]; !ok || value != *data.NotifyProps.CommentsTrigger {
				user.AddNotifyProp(model.CommentsNotifyProp, *data.NotifyProps.CommentsTrigger)
				hasNotifyPropsChanged = true
			}
		}

		if data.NotifyProps.MentionKeys != nil {
			if value, ok := user.NotifyProps[model.MentionKeysNotifyProp]; !ok || value != *data.NotifyProps.MentionKeys {
				user.AddNotifyProp(model.MentionKeysNotifyProp, *data.NotifyProps.MentionKeys)
				hasNotifyPropsChanged = true
			}
		} else {
			user.UpdateMentionKeysFromUsername("")
		}
	}

	var savedUser *model.User
	var err error
	if user.ID == "" {
		if savedUser, err = a.srv.userService.CreateUser(user, users.UserCreateOptions{FromImport: true}); err != nil {
			var appErr *model.AppError
			var invErr *store.ErrInvalidInput
			switch {
			case errors.As(err, &appErr):
				return appErr
			case errors.Is(err, users.AcceptedDomainError):
				return model.NewAppError("importUser", "api.user.create_user.accepted_domain.app_error", nil, "", http.StatusBadRequest)
			case errors.Is(err, users.UserCountError):
				return model.NewAppError("importUser", "app.user.get_total_users_count.app_error", nil, nErr.Error(), http.StatusInternalServerError)
			case errors.As(err, &invErr):
				switch invErr.Field {
				case "email":
					return model.NewAppError("importUser", "app.user.save.email_exists.app_error", nil, invErr.Error(), http.StatusBadRequest)
				case "username":
					return model.NewAppError("importUser", "app.user.save.username_exists.app_error", nil, invErr.Error(), http.StatusBadRequest)
				default:
					return model.NewAppError("importUser", "app.user.save.existing.app_error", nil, invErr.Error(), http.StatusBadRequest)
				}
			default:
				return model.NewAppError("importUser", "app.user.save.app_error", nil, nErr.Error(), http.StatusInternalServerError)
			}
		}

		pref := model.Preference{UserID: savedUser.ID, Category: model.PreferenceCategoryTutorialSteps, Name: savedUser.ID, Value: "0"}
		if err := a.Srv().Store.Preference().Save(&model.Preferences{pref}); err != nil {
			mlog.Warn("Encountered error saving tutorial preference", mlog.Err(err))
		}

	} else {
		var appErr *model.AppError
		if hasUserChanged {
			if savedUser, appErr = a.UpdateUser(user, false); appErr != nil {
				return appErr
			}
		}
		if hasUserRolesChanged {
			if savedUser, appErr = a.UpdateUserRoles(user.ID, roles, false); appErr != nil {
				return appErr
			}
		}
		if hasNotifyPropsChanged {
			if savedUser, appErr = a.UpdateUserNotifyProps(user.ID, user.NotifyProps, false); appErr != nil {
				return appErr
			}
		}
		if password != "" {
			if appErr = a.UpdatePassword(user, password); appErr != nil {
				return appErr
			}
		} else {
			if hasUserAuthDataChanged {
				if _, nErr := a.Srv().Store.User().UpdateAuthData(user.ID, authService, authData, user.Email, false); nErr != nil {
					var invErr *store.ErrInvalidInput
					switch {
					case errors.As(nErr, &invErr):
						return model.NewAppError("importUser", "app.user.update_auth_data.email_exists.app_error", nil, invErr.Error(), http.StatusBadRequest)
					default:
						return model.NewAppError("importUser", "app.user.update_auth_data.app_error", nil, nErr.Error(), http.StatusInternalServerError)
					}
				}
			}
		}
		if emailVerified {
			if hasUserEmailVerifiedChanged {
				if err := a.VerifyUserEmail(user.ID, user.Email); err != nil {
					return err
				}
			}
		}
	}

	if savedUser == nil {
		savedUser = user
	}

	if data.ProfileImage != nil {
		file, err := os.Open(*data.ProfileImage)
		if err != nil {
			mlog.Warn("Unable to open the profile image.", mlog.Err(err))
		}
		defer file.Close()
		if err == nil {
			if err := a.SetProfileImageFromMultiPartFile(savedUser.ID, file); err != nil {
				mlog.Warn("Unable to set the profile image from a file.", mlog.Err(err))
			}
		}
	}

	// Preferences.
	var preferences model.Preferences

	if data.Theme != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategoryTheme,
			Name:     "",
			Value:    *data.Theme,
		})
	}

	if data.UseMilitaryTime != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategoryDisplaySettings,
			Name:     model.PreferenceNameUseMilitaryTime,
			Value:    *data.UseMilitaryTime,
		})
	}

	if data.CollapsePreviews != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategoryDisplaySettings,
			Name:     model.PreferenceNameCollapseSetting,
			Value:    *data.CollapsePreviews,
		})
	}

	if data.MessageDisplay != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategoryDisplaySettings,
			Name:     model.PreferenceNameMessageDisplay,
			Value:    *data.MessageDisplay,
		})
	}

	if data.ChannelDisplayMode != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategoryDisplaySettings,
			Name:     "channel_display_mode",
			Value:    *data.ChannelDisplayMode,
		})
	}

	if data.TutorialStep != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategoryTutorialSteps,
			Name:     savedUser.ID,
			Value:    *data.TutorialStep,
		})
	}

	if data.UseMarkdownPreview != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategoryAdvancedSettings,
			Name:     "feature_enabled_markdown_preview",
			Value:    *data.UseMarkdownPreview,
		})
	}

	if data.UseFormatting != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategoryAdvancedSettings,
			Name:     "formatting",
			Value:    *data.UseFormatting,
		})
	}

	if data.ShowUnreadSection != nil {
		preferences = append(preferences, model.Preference{
			UserID:   savedUser.ID,
			Category: model.PreferenceCategorySidebarSettings,
			Name:     "show_unread_section",
			Value:    *data.ShowUnreadSection,
		})
	}

	if data.EmailInterval != nil || savedUser.NotifyProps[model.EmailNotifyProp] == "false" {
		var intervalSeconds string
		if value := savedUser.NotifyProps[model.EmailNotifyProp]; value == "false" {
			intervalSeconds = "0"
		} else {
			switch *data.EmailInterval {
			case model.PreferenceEmailIntervalImmediately:
				intervalSeconds = model.PreferenceEmailIntervalNoBatchingSeconds
			case model.PreferenceEmailIntervalFifteen:
				intervalSeconds = model.PreferenceEmailIntervalFifteenAsSeconds
			case model.PreferenceEmailIntervalHour:
				intervalSeconds = model.PreferenceEmailIntervalHourAsSeconds
			}
		}
		if intervalSeconds != "" {
			preferences = append(preferences, model.Preference{
				UserID:   savedUser.ID,
				Category: model.PreferenceCategoryNotifications,
				Name:     model.PreferenceNameEmailInterval,
				Value:    intervalSeconds,
			})
		}
	}

	if len(preferences) > 0 {
		if err := a.Srv().Store.Preference().Save(&preferences); err != nil {
			return model.NewAppError("BulkImport", "app.import.import_user.save_preferences.error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return a.importUserTeams(savedUser, data.Teams)
}

func (a *App) importUserTeams(user *model.User, data *[]UserTeamImportData) *model.AppError {
	if data == nil {
		return nil
	}

	teamNames := []string{}
	for _, tdata := range *data {
		teamNames = append(teamNames, *tdata.Name)
	}
	allTeams, err := a.getTeamsByNames(teamNames)
	if err != nil {
		return err
	}

	teamThemePreferencesByID := map[string]model.Preferences{}
	channels := map[string][]UserChannelImportData{}
	teamsByID := map[string]*model.Team{}
	teamMemberByTeamID := map[string]*model.TeamMember{}
	newTeamMembers := []*model.TeamMember{}
	oldTeamMembers := []*model.TeamMember{}
	rolesByTeamID := map[string]string{}
	isGuestByTeamID := map[string]bool{}
	isUserByTeamID := map[string]bool{}
	isAdminByTeamID := map[string]bool{}
	existingMemberships, nErr := a.Srv().Store.Team().GetTeamsForUser(context.Background(), user.ID)
	if nErr != nil {
		return model.NewAppError("importUserTeams", "app.team.get_members.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	existingMembershipsByTeamID := map[string]*model.TeamMember{}
	for _, teamMembership := range existingMemberships {
		existingMembershipsByTeamID[teamMembership.TeamID] = teamMembership
	}
	for _, tdata := range *data {
		team := allTeams[*tdata.Name]

		// Team-specific theme Preferences.
		if tdata.Theme != nil {
			teamThemePreferencesByID[team.ID] = append(teamThemePreferencesByID[team.ID], model.Preference{
				UserID:   user.ID,
				Category: model.PreferenceCategoryTheme,
				Name:     team.ID,
				Value:    *tdata.Theme,
			})
		}

		isGuestByTeamID[team.ID] = false
		isUserByTeamID[team.ID] = true
		isAdminByTeamID[team.ID] = false

		if tdata.Roles == nil {
			isUserByTeamID[team.ID] = true
		} else {
			rawRoles := *tdata.Roles
			explicitRoles := []string{}
			for _, role := range strings.Fields(rawRoles) {
				if role == model.TeamGuestRoleID {
					isGuestByTeamID[team.ID] = true
					isUserByTeamID[team.ID] = false
				} else if role == model.TeamUserRoleID {
					isUserByTeamID[team.ID] = true
				} else if role == model.TeamAdminRoleID {
					isAdminByTeamID[team.ID] = true
				} else {
					explicitRoles = append(explicitRoles, role)
				}
			}
			rolesByTeamID[team.ID] = strings.Join(explicitRoles, " ")
		}

		member := &model.TeamMember{
			TeamID:      team.ID,
			UserID:      user.ID,
			SchemeGuest: user.IsGuest(),
			SchemeUser:  !user.IsGuest(),
			SchemeAdmin: team.Email == user.Email && !user.IsGuest(),
		}
		if !user.IsGuest() {
			var userShouldBeAdmin bool
			userShouldBeAdmin, err = a.UserIsInAdminRoleGroup(user.ID, team.ID, model.GroupSyncableTypeTeam)
			if err != nil {
				return err
			}
			member.SchemeAdmin = userShouldBeAdmin
		}

		if tdata.Channels != nil {
			channels[team.ID] = append(channels[team.ID], *tdata.Channels...)
		}
		if !user.IsGuest() {
			channels[team.ID] = append(channels[team.ID], UserChannelImportData{Name: model.NewString(model.DefaultChannelName)})
		}

		teamsByID[team.ID] = team
		teamMemberByTeamID[team.ID] = member
		if _, ok := existingMembershipsByTeamID[team.ID]; !ok {
			newTeamMembers = append(newTeamMembers, member)
		} else {
			oldTeamMembers = append(oldTeamMembers, member)
		}
	}

	oldMembers, nErr := a.Srv().Store.Team().UpdateMultipleMembers(oldTeamMembers)
	if nErr != nil {
		var appErr *model.AppError
		switch {
		case errors.As(nErr, &appErr):
			return appErr
		default:
			return model.NewAppError("importUserTeams", "app.team.save_member.save.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	newMembers := []*model.TeamMember{}
	if len(newTeamMembers) > 0 {
		var nErr error
		newMembers, nErr = a.Srv().Store.Team().SaveMultipleMembers(newTeamMembers, *a.Config().TeamSettings.MaxUsersPerTeam)
		if nErr != nil {
			var appErr *model.AppError
			var conflictErr *store.ErrConflict
			var limitExeededErr *store.ErrLimitExceeded
			switch {
			case errors.As(nErr, &appErr): // in case we haven't converted to plain error.
				return appErr
			case errors.As(nErr, &conflictErr):
				return model.NewAppError("BulkImport", "app.import.import_user_teams.save_members.conflict.app_error", nil, nErr.Error(), http.StatusBadRequest)
			case errors.As(nErr, &limitExeededErr):
				return model.NewAppError("BulkImport", "app.import.import_user_teams.save_members.max_accounts.app_error", nil, nErr.Error(), http.StatusBadRequest)
			default: // last fallback in case it doesn't map to an existing app error.
				return model.NewAppError("BulkImport", "app.import.import_user_teams.save_members.error", nil, nErr.Error(), http.StatusInternalServerError)
			}
		}
	}

	for _, member := range append(newMembers, oldMembers...) {
		if member.ExplicitRoles != rolesByTeamID[member.TeamID] {
			if _, err = a.UpdateTeamMemberRoles(member.TeamID, user.ID, rolesByTeamID[member.TeamID]); err != nil {
				return err
			}
		}

		a.UpdateTeamMemberSchemeRoles(member.TeamID, user.ID, isGuestByTeamID[member.TeamID], isUserByTeamID[member.TeamID], isAdminByTeamID[member.TeamID])
	}

	for _, team := range allTeams {
		if len(teamThemePreferencesByID[team.ID]) > 0 {
			pref := teamThemePreferencesByID[team.ID]
			if err := a.Srv().Store.Preference().Save(&pref); err != nil {
				return model.NewAppError("BulkImport", "app.import.import_user_teams.save_preferences.error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
		channelsToImport := channels[team.ID]
		if err := a.importUserChannels(user, team, &channelsToImport); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) importUserChannels(user *model.User, team *model.Team, data *[]UserChannelImportData) *model.AppError {
	if data == nil {
		return nil
	}

	channelNames := []string{}
	for _, tdata := range *data {
		channelNames = append(channelNames, *tdata.Name)
	}
	allChannels, err := a.getChannelsByNames(channelNames, team.ID)
	if err != nil {
		return err
	}

	channelsByID := map[string]*model.Channel{}
	channelMemberByChannelID := map[string]*model.ChannelMember{}
	newChannelMembers := []*model.ChannelMember{}
	oldChannelMembers := []*model.ChannelMember{}
	rolesByChannelID := map[string]string{}
	channelPreferencesByID := map[string]model.Preferences{}
	isGuestByChannelID := map[string]bool{}
	isUserByChannelID := map[string]bool{}
	isAdminByChannelID := map[string]bool{}
	existingMemberships, nErr := a.Srv().Store.Channel().GetMembersForUser(team.ID, user.ID)
	if nErr != nil {
		return model.NewAppError("importUserChannels", "app.channel.get_members.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	existingMembershipsByChannelID := map[string]model.ChannelMember{}
	for _, channelMembership := range *existingMemberships {
		existingMembershipsByChannelID[channelMembership.ChannelID] = channelMembership
	}
	for _, cdata := range *data {
		channel, ok := allChannels[*cdata.Name]
		if !ok {
			return model.NewAppError("BulkImport", "app.import.import_user_channels.channel_not_found.error", nil, "", http.StatusInternalServerError)
		}
		if _, ok = channelsByID[channel.ID]; ok && *cdata.Name == model.DefaultChannelName {
			// town-square membership was in the import and added by the importer (skip the added by the importer)
			continue
		}

		isGuestByChannelID[channel.ID] = false
		isUserByChannelID[channel.ID] = true
		isAdminByChannelID[channel.ID] = false

		if cdata.Roles == nil {
			isUserByChannelID[channel.ID] = true
		} else {
			rawRoles := *cdata.Roles
			explicitRoles := []string{}
			for _, role := range strings.Fields(rawRoles) {
				if role == model.ChannelGuestRoleID {
					isGuestByChannelID[channel.ID] = true
					isUserByChannelID[channel.ID] = false
				} else if role == model.ChannelUserRoleID {
					isUserByChannelID[channel.ID] = true
				} else if role == model.ChannelAdminRoleID {
					isAdminByChannelID[channel.ID] = true
				} else {
					explicitRoles = append(explicitRoles, role)
				}
			}
			rolesByChannelID[channel.ID] = strings.Join(explicitRoles, " ")
		}

		if cdata.Favorite != nil && *cdata.Favorite {
			channelPreferencesByID[channel.ID] = append(channelPreferencesByID[channel.ID], model.Preference{
				UserID:   user.ID,
				Category: model.PreferenceCategoryFavoriteChannel,
				Name:     channel.ID,
				Value:    "true",
			})
		}

		member := &model.ChannelMember{
			ChannelID:   channel.ID,
			UserID:      user.ID,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
			SchemeGuest: user.IsGuest(),
			SchemeUser:  !user.IsGuest(),
			SchemeAdmin: false,
		}
		if !user.IsGuest() {
			var userShouldBeAdmin bool
			userShouldBeAdmin, err = a.UserIsInAdminRoleGroup(user.ID, team.ID, model.GroupSyncableTypeTeam)
			if err != nil {
				return err
			}
			member.SchemeAdmin = userShouldBeAdmin
		}

		if cdata.NotifyProps != nil {
			if cdata.NotifyProps.Desktop != nil {
				member.NotifyProps[model.DesktopNotifyProp] = *cdata.NotifyProps.Desktop
			}

			if cdata.NotifyProps.Mobile != nil {
				member.NotifyProps[model.PushNotifyProp] = *cdata.NotifyProps.Mobile
			}

			if cdata.NotifyProps.MarkUnread != nil {
				member.NotifyProps[model.MarkUnreadNotifyProp] = *cdata.NotifyProps.MarkUnread
			}
		}

		channelsByID[channel.ID] = channel
		channelMemberByChannelID[channel.ID] = member
		if _, ok := existingMembershipsByChannelID[channel.ID]; !ok {
			newChannelMembers = append(newChannelMembers, member)
		} else {
			oldChannelMembers = append(oldChannelMembers, member)
		}
	}

	oldMembers, nErr := a.Srv().Store.Channel().UpdateMultipleMembers(oldChannelMembers)
	if nErr != nil {
		var nfErr *store.ErrNotFound
		var appErr *model.AppError
		switch {
		case errors.As(nErr, &appErr):
			return appErr
		case errors.As(nErr, &nfErr):
			return model.NewAppError("importUserChannels", MissingChannelMemberError, nil, nfErr.Error(), http.StatusNotFound)
		default:
			return model.NewAppError("importUserChannels", "app.channel.get_member.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	newMembers := []*model.ChannelMember{}
	if len(newChannelMembers) > 0 {
		newMembers, nErr = a.Srv().Store.Channel().SaveMultipleMembers(newChannelMembers)
		if nErr != nil {
			var cErr *store.ErrConflict
			var appErr *model.AppError
			switch {
			case errors.As(nErr, &cErr):
				switch cErr.Resource {
				case "ChannelMembers":
					return model.NewAppError("importUserChannels", "app.channel.save_member.exists.app_error", nil, cErr.Error(), http.StatusBadRequest)
				}
			case errors.As(nErr, &appErr):
				return appErr
			default:
				return model.NewAppError("importUserChannels", "app.channel.create_direct_channel.internal_error", nil, nErr.Error(), http.StatusInternalServerError)
			}
		}
	}

	for _, member := range append(newMembers, oldMembers...) {
		if member.ExplicitRoles != rolesByChannelID[member.ChannelID] {
			if _, err = a.UpdateChannelMemberRoles(member.ChannelID, user.ID, rolesByChannelID[member.ChannelID]); err != nil {
				return err
			}
		}

		a.UpdateChannelMemberSchemeRoles(member.ChannelID, user.ID, isGuestByChannelID[member.ChannelID], isUserByChannelID[member.ChannelID], isAdminByChannelID[member.ChannelID])
	}

	for _, channel := range allChannels {
		if len(channelPreferencesByID[channel.ID]) > 0 {
			pref := channelPreferencesByID[channel.ID]
			if err := a.Srv().Store.Preference().Save(&pref); err != nil {
				return model.NewAppError("BulkImport", "app.import.import_user_channels.save_preferences.error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	return nil
}

func (a *App) importReaction(data *ReactionImportData, post *model.Post) *model.AppError {
	if err := validateReactionImportData(data, post.CreateAt); err != nil {
		return err
	}

	var user *model.User
	var nErr error
	if user, nErr = a.Srv().Store.User().GetByUsername(*data.User); nErr != nil {
		return model.NewAppError("BulkImport", "app.import.import_post.user_not_found.error", map[string]interface{}{"Username": data.User}, nErr.Error(), http.StatusBadRequest)
	}

	reaction := &model.Reaction{
		UserID:    user.ID,
		PostID:    post.ID,
		EmojiName: *data.EmojiName,
		CreateAt:  *data.CreateAt,
	}
	if _, nErr = a.Srv().Store.Reaction().Save(reaction); nErr != nil {
		var appErr *model.AppError
		switch {
		case errors.As(nErr, &appErr):
			return appErr
		default:
			return model.NewAppError("importReaction", "app.reaction.save.save.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	return nil
}

func (a *App) importReplies(c *request.Context, data []ReplyImportData, post *model.Post, teamID string) *model.AppError {
	var err *model.AppError
	usernames := []string{}
	for _, replyData := range data {
		replyData := replyData
		if err = validateReplyImportData(&replyData, post.CreateAt, a.MaxPostSize()); err != nil {
			return err
		}
		usernames = append(usernames, *replyData.User)
	}

	users, err := a.getUsersByUsernames(usernames)
	if err != nil {
		return err
	}

	postsWithData := []postAndData{}
	postsForCreateList := []*model.Post{}
	postsForOverwriteList := []*model.Post{}

	for _, replyData := range data {
		replyData := replyData
		user := users[*replyData.User]

		// Check if this post already exists.
		replies, nErr := a.Srv().Store.Post().GetPostsCreatedAt(post.ChannelID, *replyData.CreateAt)
		if nErr != nil {
			return model.NewAppError("importReplies", "app.post.get_posts_created_at.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}

		var reply *model.Post
		for _, r := range replies {
			if r.Message == *replyData.Message && r.RootID == post.ID {
				reply = r
				break
			}
		}

		if reply == nil {
			reply = &model.Post{}
		}
		reply.UserID = user.ID
		reply.ChannelID = post.ChannelID
		reply.ParentID = post.ID
		reply.RootID = post.ID
		reply.Message = *replyData.Message
		reply.CreateAt = *replyData.CreateAt

		fileIDs, err := a.uploadAttachments(c, replyData.Attachments, reply, teamID)
		if err != nil {
			return err
		}
		for _, fileID := range reply.FileIDs {
			if _, ok := fileIDs[fileID]; !ok {
				a.Srv().Store.FileInfo().PermanentDelete(fileID)
			}
		}
		reply.FileIDs = make([]string, 0)
		for fileID := range fileIDs {
			reply.FileIDs = append(reply.FileIDs, fileID)
		}

		if reply.ID == "" {
			postsForCreateList = append(postsForCreateList, reply)
		} else {
			postsForOverwriteList = append(postsForOverwriteList, reply)
		}
		postsWithData = append(postsWithData, postAndData{post: reply, replyData: &replyData})
	}

	if len(postsForCreateList) > 0 {
		if _, _, err := a.Srv().Store.Post().SaveMultiple(postsForCreateList); err != nil {
			var appErr *model.AppError
			var invErr *store.ErrInvalidInput
			switch {
			case errors.As(err, &appErr):
				return appErr
			case errors.As(err, &invErr):
				return model.NewAppError("importReplies", "app.post.save.existing.app_error", nil, invErr.Error(), http.StatusBadRequest)
			default:
				return model.NewAppError("importReplies", "app.post.save.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	if _, _, nErr := a.Srv().Store.Post().OverwriteMultiple(postsForOverwriteList); nErr != nil {
		return model.NewAppError("importReplies", "app.post.overwrite.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	for _, postWithData := range postsWithData {
		a.updateFileInfoWithPostID(postWithData.post)
	}

	return nil
}

func (a *App) importAttachment(c *request.Context, data *AttachmentImportData, post *model.Post, teamID string) (*model.FileInfo, *model.AppError) {
	var (
		name string
		file io.Reader
	)
	if data.Data != nil {
		zipFile, err := data.Data.Open()
		if err != nil {
			return nil, model.NewAppError("BulkImport", "app.import.attachment.bad_file.error", map[string]interface{}{"FilePath": *data.Path}, err.Error(), http.StatusBadRequest)
		}
		defer zipFile.Close()
		name = data.Data.Name
		file = zipFile.(io.Reader)
	} else {
		realFile, err := os.Open(*data.Path)
		if err != nil {
			return nil, model.NewAppError("BulkImport", "app.import.attachment.bad_file.error", map[string]interface{}{"FilePath": *data.Path}, err.Error(), http.StatusBadRequest)
		}
		defer realFile.Close()
		name = realFile.Name()
		file = realFile
	}

	timestamp := utils.TimeFromMillis(post.CreateAt)

	fileData, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, model.NewAppError("BulkImport", "app.import.attachment.read_file_data.error", map[string]interface{}{"FilePath": *data.Path}, "", http.StatusBadRequest)
	}

	// Go over existing files in the post and see if there already exists a file with the same name, size and hash. If so - skip it
	if post.ID != "" {
		oldFiles, err := a.GetFileInfosForPost(post.ID, true)
		if err != nil {
			return nil, model.NewAppError("BulkImport", "app.import.attachment.file_upload.error", map[string]interface{}{"FilePath": *data.Path}, "", http.StatusBadRequest)
		}
		for _, oldFile := range oldFiles {
			if oldFile.Name != path.Base(name) || oldFile.Size != int64(len(fileData)) {
				continue
			}
			// check md5
			newHash := sha1.Sum(fileData)
			oldFileData, err := a.GetFile(oldFile.ID)
			if err != nil {
				return nil, model.NewAppError("BulkImport", "app.import.attachment.file_upload.error", map[string]interface{}{"FilePath": *data.Path}, "", http.StatusBadRequest)
			}
			oldHash := sha1.Sum(oldFileData)

			if bytes.Equal(oldHash[:], newHash[:]) {
				mlog.Info("Skipping uploading of file because name already exists", mlog.Any("file_name", name))
				return oldFile, nil
			}
		}
	}

	mlog.Info("Uploading file with name", mlog.String("file_name", name))

	fileInfo, appErr := a.DoUploadFile(c, timestamp, teamID, post.ChannelID, post.UserID, name, fileData)
	if appErr != nil {
		mlog.Error("Failed to upload file:", mlog.Err(appErr))
		return nil, appErr
	}

	if fileInfo.IsImage() {
		a.HandleImages([]string{fileInfo.PreviewPath}, []string{fileInfo.ThumbnailPath}, [][]byte{fileData})
	}

	return fileInfo, nil
}

type postAndData struct {
	post           *model.Post
	postData       *PostImportData
	directPostData *DirectPostImportData
	replyData      *ReplyImportData
	team           *model.Team
	lineNumber     int
}

func (a *App) getUsersByUsernames(usernames []string) (map[string]*model.User, *model.AppError) {
	uniqueUsernames := utils.RemoveDuplicatesFromStringArray(usernames)
	allUsers, err := a.Srv().Store.User().GetProfilesByUsernames(uniqueUsernames, nil)
	if err != nil {
		return nil, model.NewAppError("BulkImport", "app.import.get_users_by_username.some_users_not_found.error", nil, err.Error(), http.StatusBadRequest)
	}

	if len(allUsers) != len(uniqueUsernames) {
		return nil, model.NewAppError("BulkImport", "app.import.get_users_by_username.some_users_not_found.error", nil, "", http.StatusBadRequest)
	}

	users := make(map[string]*model.User)
	for _, user := range allUsers {
		users[user.Username] = user
	}
	return users, nil
}

func (a *App) getTeamsByNames(names []string) (map[string]*model.Team, *model.AppError) {
	allTeams, err := a.Srv().Store.Team().GetByNames(names)
	if err != nil {
		return nil, model.NewAppError("BulkImport", "app.import.get_teams_by_names.some_teams_not_found.error", nil, err.Error(), http.StatusBadRequest)
	}

	teams := make(map[string]*model.Team)
	for _, team := range allTeams {
		teams[team.Name] = team
	}
	return teams, nil
}

func (a *App) getChannelsByNames(names []string, teamID string) (map[string]*model.Channel, *model.AppError) {
	allChannels, err := a.Srv().Store.Channel().GetByNames(teamID, names, true)
	if err != nil {
		return nil, model.NewAppError("BulkImport", "app.import.get_teams_by_names.some_teams_not_found.error", nil, err.Error(), http.StatusBadRequest)
	}

	channels := make(map[string]*model.Channel)
	for _, channel := range allChannels {
		channels[channel.Name] = channel
	}
	return channels, nil
}

// getChannelsForPosts returns map[teamName]map[channelName]*model.Channel
func (a *App) getChannelsForPosts(teams map[string]*model.Team, data []*PostImportData) (map[string]map[string]*model.Channel, *model.AppError) {
	teamChannels := make(map[string]map[string]*model.Channel)
	for _, postData := range data {
		teamName := *postData.Team
		if _, ok := teamChannels[teamName]; !ok {
			teamChannels[teamName] = make(map[string]*model.Channel)
		}
		if channel, ok := teamChannels[teamName][*postData.Channel]; !ok || channel == nil {
			var err error
			channel, err = a.Srv().Store.Channel().GetByName(teams[teamName].ID, *postData.Channel, true)
			if err != nil {
				return nil, model.NewAppError("BulkImport", "app.import.import_post.channel_not_found.error", map[string]interface{}{"ChannelName": *postData.Channel}, err.Error(), http.StatusBadRequest)
			}
			teamChannels[teamName][*postData.Channel] = channel
		}
	}
	return teamChannels, nil
}

// getPostStrID returns a string ID composed of several post fields to
// uniquely identify a post before it's imported, so it has no ID yet
func getPostStrID(post *model.Post) string {
	return fmt.Sprintf("%d%s%s", post.CreateAt, post.ChannelID, post.Message)
}

// importMultiplePostLines will return an error and the line that
// caused it whenever possible
func (a *App) importMultiplePostLines(c *request.Context, lines []LineImportWorkerData, dryRun bool) (int, *model.AppError) {
	if len(lines) == 0 {
		return 0, nil
	}

	for _, line := range lines {
		if err := validatePostImportData(line.Post, a.MaxPostSize()); err != nil {
			return line.LineNumber, err
		}
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return 0, nil
	}

	usernames := []string{}
	teamNames := make([]string, len(lines))
	postsData := make([]*PostImportData, len(lines))
	for i, line := range lines {
		usernames = append(usernames, *line.Post.User)
		if line.Post.FlaggedBy != nil {
			usernames = append(usernames, *line.Post.FlaggedBy...)
		}
		teamNames[i] = *line.Post.Team
		postsData[i] = line.Post
	}

	users, err := a.getUsersByUsernames(usernames)
	if err != nil {
		return 0, err
	}

	teams, err := a.getTeamsByNames(teamNames)
	if err != nil {
		return 0, err
	}

	channels, err := a.getChannelsForPosts(teams, postsData)
	if err != nil {
		return 0, err
	}
	postsWithData := []postAndData{}
	postsForCreateList := []*model.Post{}
	postsForCreateMap := map[string]int{}
	postsForOverwriteList := []*model.Post{}
	postsForOverwriteMap := map[string]int{}

	for _, line := range lines {
		team := teams[*line.Post.Team]
		channel := channels[*line.Post.Team][*line.Post.Channel]
		user := users[*line.Post.User]

		// Check if this post already exists.
		posts, nErr := a.Srv().Store.Post().GetPostsCreatedAt(channel.ID, *line.Post.CreateAt)
		if nErr != nil {
			return line.LineNumber, model.NewAppError("importMultiplePostLines", "app.post.get_posts_created_at.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}

		var post *model.Post
		for _, p := range posts {
			if p.Message == *line.Post.Message {
				post = p
				break
			}
		}

		if post == nil {
			post = &model.Post{}
		}

		post.ChannelID = channel.ID
		post.Message = *line.Post.Message
		post.UserID = user.ID
		post.CreateAt = *line.Post.CreateAt
		post.Hashtags, _ = model.ParseHashtags(post.Message)

		if line.Post.Props != nil {
			post.Props = *line.Post.Props
		}

		fileIDs, appErr := a.uploadAttachments(c, line.Post.Attachments, post, team.ID)
		if appErr != nil {
			return line.LineNumber, appErr
		}
		for _, fileID := range post.FileIDs {
			if _, ok := fileIDs[fileID]; !ok {
				a.Srv().Store.FileInfo().PermanentDelete(fileID)
			}
		}
		post.FileIDs = make([]string, 0)
		for fileID := range fileIDs {
			post.FileIDs = append(post.FileIDs, fileID)
		}

		if post.ID == "" {
			postsForCreateList = append(postsForCreateList, post)
			postsForCreateMap[getPostStrID(post)] = line.LineNumber
		} else {
			postsForOverwriteList = append(postsForOverwriteList, post)
			postsForOverwriteMap[getPostStrID(post)] = line.LineNumber
		}
		postsWithData = append(postsWithData, postAndData{post: post, postData: line.Post, team: team, lineNumber: line.LineNumber})
	}

	if len(postsForCreateList) > 0 {
		if _, idx, nErr := a.Srv().Store.Post().SaveMultiple(postsForCreateList); nErr != nil {
			var appErr *model.AppError
			var invErr *store.ErrInvalidInput
			var retErr *model.AppError
			switch {
			case errors.As(nErr, &appErr):
				retErr = appErr
			case errors.As(nErr, &invErr):
				retErr = model.NewAppError("importMultiplePostLines", "app.post.save.existing.app_error", nil, invErr.Error(), http.StatusBadRequest)
			default:
				retErr = model.NewAppError("importMultiplePostLines", "app.post.save.app_error", nil, nErr.Error(), http.StatusInternalServerError)
			}

			if idx != -1 && idx < len(postsForCreateList) {
				post := postsForCreateList[idx]
				if lineNumber, ok := postsForCreateMap[getPostStrID(post)]; ok {
					return lineNumber, retErr
				}
			}
			return 0, retErr
		}
	}

	if _, idx, err := a.Srv().Store.Post().OverwriteMultiple(postsForOverwriteList); err != nil {
		if idx != -1 && idx < len(postsForOverwriteList) {
			post := postsForOverwriteList[idx]
			if lineNumber, ok := postsForOverwriteMap[getPostStrID(post)]; ok {
				return lineNumber, model.NewAppError("importMultiplePostLines", "app.post.overwrite.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
		return 0, model.NewAppError("importMultiplePostLines", "app.post.overwrite.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, postWithData := range postsWithData {
		postWithData := postWithData
		if postWithData.postData.FlaggedBy != nil {
			var preferences model.Preferences

			for _, username := range *postWithData.postData.FlaggedBy {
				user := users[username]

				preferences = append(preferences, model.Preference{
					UserID:   user.ID,
					Category: model.PreferenceCategoryFlaggedPost,
					Name:     postWithData.post.ID,
					Value:    "true",
				})
			}

			if len(preferences) > 0 {
				if err := a.Srv().Store.Preference().Save(&preferences); err != nil {
					return postWithData.lineNumber, model.NewAppError("BulkImport", "app.import.import_post.save_preferences.error", nil, err.Error(), http.StatusInternalServerError)
				}
			}
		}

		if postWithData.postData.Reactions != nil {
			for _, reaction := range *postWithData.postData.Reactions {
				reaction := reaction
				if err := a.importReaction(&reaction, postWithData.post); err != nil {
					return postWithData.lineNumber, err
				}
			}
		}

		if postWithData.postData.Replies != nil && len(*postWithData.postData.Replies) > 0 {
			err := a.importReplies(c, *postWithData.postData.Replies, postWithData.post, postWithData.team.ID)
			if err != nil {
				return postWithData.lineNumber, err
			}
		}
		a.updateFileInfoWithPostID(postWithData.post)
	}
	return 0, nil
}

// uploadAttachments imports new attachments and returns current attachments of the post as a map
func (a *App) uploadAttachments(c *request.Context, attachments *[]AttachmentImportData, post *model.Post, teamID string) (map[string]bool, *model.AppError) {
	if attachments == nil {
		return nil, nil
	}
	fileIDs := make(map[string]bool)
	for _, attachment := range *attachments {
		attachment := attachment
		fileInfo, err := a.importAttachment(c, &attachment, post, teamID)
		if err != nil {
			return nil, err
		}
		fileIDs[fileInfo.ID] = true
	}
	return fileIDs, nil
}

func (a *App) updateFileInfoWithPostID(post *model.Post) {
	for _, fileID := range post.FileIDs {
		if err := a.Srv().Store.FileInfo().AttachToPost(fileID, post.ID, post.UserID); err != nil {
			mlog.Error("Error attaching files to post.", mlog.String("post_id", post.ID), mlog.Any("post_file_ids", post.FileIDs), mlog.Err(err))
		}
	}
}
func (a *App) importDirectChannel(data *DirectChannelImportData, dryRun bool) *model.AppError {
	var err *model.AppError
	if err = validateDirectChannelImportData(data); err != nil {
		return err
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return nil
	}

	var userIDs []string
	userMap, err := a.getUsersByUsernames(*data.Members)
	if err != nil {
		return err
	}
	for _, user := range *data.Members {
		userIDs = append(userIDs, userMap[user].ID)
	}

	var channel *model.Channel

	if len(userIDs) == 2 {
		ch, err := a.createDirectChannel(userIDs[0], userIDs[1])
		if err != nil && err.ID != store.ChannelExistsError {
			return model.NewAppError("BulkImport", "app.import.import_direct_channel.create_direct_channel.error", nil, err.Error(), http.StatusBadRequest)
		}
		channel = ch
	} else {
		ch, err := a.createGroupChannel(userIDs)
		if err != nil && err.ID != store.ChannelExistsError {
			return model.NewAppError("BulkImport", "app.import.import_direct_channel.create_group_channel.error", nil, err.Error(), http.StatusBadRequest)
		}
		channel = ch
	}

	var preferences model.Preferences

	for _, userID := range userIDs {
		preferences = append(preferences, model.Preference{
			UserID:   userID,
			Category: model.PreferenceCategoryDirectChannelShow,
			Name:     channel.ID,
			Value:    "true",
		})
	}

	if data.FavoritedBy != nil {
		for _, favoriter := range *data.FavoritedBy {
			preferences = append(preferences, model.Preference{
				UserID:   userMap[favoriter].ID,
				Category: model.PreferenceCategoryFavoriteChannel,
				Name:     channel.ID,
				Value:    "true",
			})
		}
	}

	if err := a.Srv().Store.Preference().Save(&preferences); err != nil {
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			appErr.StatusCode = http.StatusBadRequest
			return appErr
		default:
			return model.NewAppError("importDirectChannel", "app.preference.save.updating.app_error", nil, err.Error(), http.StatusBadRequest)
		}
	}

	if data.Header != nil {
		channel.Header = *data.Header
		if _, appErr := a.Srv().Store.Channel().Update(channel); appErr != nil {
			return model.NewAppError("BulkImport", "app.import.import_direct_channel.update_header_failed.error", nil, appErr.Error(), http.StatusBadRequest)
		}
	}

	return nil
}

// importMultipleDirectPostLines will return an error and the line
// that caused it whenever possible
func (a *App) importMultipleDirectPostLines(c *request.Context, lines []LineImportWorkerData, dryRun bool) (int, *model.AppError) {
	if len(lines) == 0 {
		return 0, nil
	}

	for _, line := range lines {
		if err := validateDirectPostImportData(line.DirectPost, a.MaxPostSize()); err != nil {
			return line.LineNumber, err
		}
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return 0, nil
	}

	usernames := []string{}
	for _, line := range lines {
		usernames = append(usernames, *line.DirectPost.User)
		if line.DirectPost.FlaggedBy != nil {
			usernames = append(usernames, *line.DirectPost.FlaggedBy...)
		}
		usernames = append(usernames, *line.DirectPost.ChannelMembers...)
	}

	users, err := a.getUsersByUsernames(usernames)
	if err != nil {
		return 0, err
	}

	postsWithData := []postAndData{}
	postsForCreateList := []*model.Post{}
	postsForCreateMap := map[string]int{}
	postsForOverwriteList := []*model.Post{}
	postsForOverwriteMap := map[string]int{}

	for _, line := range lines {
		var userIDs []string
		var err *model.AppError
		for _, username := range *line.DirectPost.ChannelMembers {
			user := users[username]
			userIDs = append(userIDs, user.ID)
		}

		var channel *model.Channel
		var ch *model.Channel
		if len(userIDs) == 2 {
			ch, err = a.GetOrCreateDirectChannel(c, userIDs[0], userIDs[1])
			if err != nil && err.ID != store.ChannelExistsError {
				return line.LineNumber, model.NewAppError("BulkImport", "app.import.import_direct_post.create_direct_channel.error", nil, err.Error(), http.StatusBadRequest)
			}
			channel = ch
		} else {
			ch, err = a.createGroupChannel(userIDs)
			if err != nil && err.ID != store.ChannelExistsError {
				return line.LineNumber, model.NewAppError("BulkImport", "app.import.import_direct_post.create_group_channel.error", nil, err.Error(), http.StatusBadRequest)
			}
			channel = ch
		}

		user := users[*line.DirectPost.User]

		// Check if this post already exists.
		posts, nErr := a.Srv().Store.Post().GetPostsCreatedAt(channel.ID, *line.DirectPost.CreateAt)
		if nErr != nil {
			return line.LineNumber, model.NewAppError("BulkImport", "app.post.get_posts_created_at.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}

		var post *model.Post
		for _, p := range posts {
			if p.Message == *line.DirectPost.Message {
				post = p
				break
			}
		}

		if post == nil {
			post = &model.Post{}
		}

		post.ChannelID = channel.ID
		post.Message = *line.DirectPost.Message
		post.UserID = user.ID
		post.CreateAt = *line.DirectPost.CreateAt
		post.Hashtags, _ = model.ParseHashtags(post.Message)

		if line.DirectPost.Props != nil {
			post.Props = *line.DirectPost.Props
		}

		fileIDs, err := a.uploadAttachments(c, line.DirectPost.Attachments, post, "noteam")
		if err != nil {
			return line.LineNumber, err
		}
		for _, fileID := range post.FileIDs {
			if _, ok := fileIDs[fileID]; !ok {
				a.Srv().Store.FileInfo().PermanentDelete(fileID)
			}
		}
		post.FileIDs = make([]string, 0)
		for fileID := range fileIDs {
			post.FileIDs = append(post.FileIDs, fileID)
		}

		if post.ID == "" {
			postsForCreateList = append(postsForCreateList, post)
			postsForCreateMap[getPostStrID(post)] = line.LineNumber
		} else {
			postsForOverwriteList = append(postsForOverwriteList, post)
			postsForOverwriteMap[getPostStrID(post)] = line.LineNumber
		}
		postsWithData = append(postsWithData, postAndData{post: post, directPostData: line.DirectPost, lineNumber: line.LineNumber})
	}

	if len(postsForCreateList) > 0 {
		if _, idx, err := a.Srv().Store.Post().SaveMultiple(postsForCreateList); err != nil {
			var appErr *model.AppError
			var invErr *store.ErrInvalidInput
			var retErr *model.AppError
			switch {
			case errors.As(err, &appErr):
				retErr = appErr
			case errors.As(err, &invErr):
				retErr = model.NewAppError("importMultiplePostLines", "app.post.save.existing.app_error", nil, invErr.Error(), http.StatusBadRequest)
			default:
				retErr = model.NewAppError("importMultiplePostLines", "app.post.save.app_error", nil, err.Error(), http.StatusInternalServerError)
			}

			if idx != -1 && idx < len(postsForCreateList) {
				post := postsForCreateList[idx]
				if lineNumber, ok := postsForCreateMap[getPostStrID(post)]; ok {
					return lineNumber, retErr
				}
			}
			return 0, retErr
		}
	}
	if _, idx, err := a.Srv().Store.Post().OverwriteMultiple(postsForOverwriteList); err != nil {
		if idx != -1 && idx < len(postsForOverwriteList) {
			post := postsForOverwriteList[idx]
			if lineNumber, ok := postsForOverwriteMap[getPostStrID(post)]; ok {
				return lineNumber, model.NewAppError("importMultiplePostLines", "app.post.overwrite.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
		return 0, model.NewAppError("importMultiplePostLines", "app.post.overwrite.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, postWithData := range postsWithData {
		if postWithData.directPostData.FlaggedBy != nil {
			var preferences model.Preferences

			for _, username := range *postWithData.directPostData.FlaggedBy {
				user := users[username]

				preferences = append(preferences, model.Preference{
					UserID:   user.ID,
					Category: model.PreferenceCategoryFlaggedPost,
					Name:     postWithData.post.ID,
					Value:    "true",
				})
			}

			if len(preferences) > 0 {
				if err := a.Srv().Store.Preference().Save(&preferences); err != nil {
					return postWithData.lineNumber, model.NewAppError("BulkImport", "app.import.import_post.save_preferences.error", nil, err.Error(), http.StatusInternalServerError)
				}
			}
		}

		if postWithData.directPostData.Reactions != nil {
			for _, reaction := range *postWithData.directPostData.Reactions {
				reaction := reaction
				if err := a.importReaction(&reaction, postWithData.post); err != nil {
					return postWithData.lineNumber, err
				}
			}
		}

		if postWithData.directPostData.Replies != nil {
			if err := a.importReplies(c, *postWithData.directPostData.Replies, postWithData.post, "noteam"); err != nil {
				return postWithData.lineNumber, err
			}
		}

		a.updateFileInfoWithPostID(postWithData.post)
	}
	return 0, nil
}

func (a *App) importEmoji(data *EmojiImportData, dryRun bool) *model.AppError {
	if err := validateEmojiImportData(data); err != nil {
		return err
	}

	// If this is a Dry Run, do not continue any further.
	if dryRun {
		return nil
	}

	var emoji *model.Emoji

	emoji, err := a.Srv().Store.Emoji().GetByName(context.Background(), *data.Name, true)
	if err != nil {
		var nfErr *store.ErrNotFound
		if !errors.As(err, &nfErr) {
			return model.NewAppError("importEmoji", "app.emoji.get_by_name.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	alreadyExists := emoji != nil

	if !alreadyExists {
		emoji = &model.Emoji{
			Name: *data.Name,
		}
		emoji.PreSave()
	}

	file, err := os.Open(*data.Image)
	if err != nil {
		return model.NewAppError("BulkImport", "app.import.emoji.bad_file.error", map[string]interface{}{"EmojiName": *data.Name}, "", http.StatusBadRequest)
	}
	defer file.Close()

	if _, err := a.WriteFile(file, getEmojiImagePath(emoji.ID)); err != nil {
		return err
	}

	if !alreadyExists {
		if _, err := a.Srv().Store.Emoji().Save(emoji); err != nil {
			return model.NewAppError("importEmoji", "api.emoji.create.internal_error", nil, err.Error(), http.StatusBadRequest)
		}
	}

	return nil
}
