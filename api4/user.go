// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func (api *API) InitUser() {
	api.BaseRoutes.Users.Handle("", api.ApiHandler(createUser)).Methods("POST")
	api.BaseRoutes.Users.Handle("", api.ApiSessionRequired(getUsers)).Methods("GET")
	api.BaseRoutes.Users.Handle("/ids", api.ApiSessionRequired(getUsersByIds)).Methods("POST")
	api.BaseRoutes.Users.Handle("/usernames", api.ApiSessionRequired(getUsersByNames)).Methods("POST")
	api.BaseRoutes.Users.Handle("/known", api.ApiSessionRequired(getKnownUsers)).Methods("GET")
	api.BaseRoutes.Users.Handle("/search", api.ApiSessionRequiredDisableWhenBusy(searchUsers)).Methods("POST")
	api.BaseRoutes.Users.Handle("/autocomplete", api.ApiSessionRequired(autocompleteUsers)).Methods("GET")
	api.BaseRoutes.Users.Handle("/stats", api.ApiSessionRequired(getTotalUsersStats)).Methods("GET")
	api.BaseRoutes.Users.Handle("/stats/filtered", api.ApiSessionRequired(getFilteredUsersStats)).Methods("GET")
	api.BaseRoutes.Users.Handle("/group_channels", api.ApiSessionRequired(getUsersByGroupChannelIds)).Methods("POST")

	api.BaseRoutes.User.Handle("", api.ApiSessionRequired(getUser)).Methods("GET")
	api.BaseRoutes.User.Handle("/image/default", api.ApiSessionRequiredTrustRequester(getDefaultProfileImage)).Methods("GET")
	api.BaseRoutes.User.Handle("/image", api.ApiSessionRequiredTrustRequester(getProfileImage)).Methods("GET")
	api.BaseRoutes.User.Handle("/image", api.ApiSessionRequired(setProfileImage)).Methods("POST")
	api.BaseRoutes.User.Handle("/image", api.ApiSessionRequired(setDefaultProfileImage)).Methods("DELETE")
	api.BaseRoutes.User.Handle("", api.ApiSessionRequired(updateUser)).Methods("PUT")
	api.BaseRoutes.User.Handle("/patch", api.ApiSessionRequired(patchUser)).Methods("PUT")
	api.BaseRoutes.User.Handle("", api.ApiSessionRequired(deleteUser)).Methods("DELETE")
	api.BaseRoutes.User.Handle("/roles", api.ApiSessionRequired(updateUserRoles)).Methods("PUT")
	api.BaseRoutes.User.Handle("/active", api.ApiSessionRequired(updateUserActive)).Methods("PUT")
	api.BaseRoutes.User.Handle("/password", api.ApiSessionRequired(updatePassword)).Methods("PUT")
	api.BaseRoutes.User.Handle("/promote", api.ApiSessionRequired(promoteGuestToUser)).Methods("POST")
	api.BaseRoutes.User.Handle("/demote", api.ApiSessionRequired(demoteUserToGuest)).Methods("POST")
	api.BaseRoutes.User.Handle("/convert_to_bot", api.ApiSessionRequired(convertUserToBot)).Methods("POST")
	api.BaseRoutes.Users.Handle("/password/reset", api.ApiHandler(resetPassword)).Methods("POST")
	api.BaseRoutes.Users.Handle("/password/reset/send", api.ApiHandler(sendPasswordReset)).Methods("POST")
	api.BaseRoutes.Users.Handle("/email/verify", api.ApiHandler(verifyUserEmail)).Methods("POST")
	api.BaseRoutes.Users.Handle("/email/verify/send", api.ApiHandler(sendVerificationEmail)).Methods("POST")
	api.BaseRoutes.User.Handle("/email/verify/member", api.ApiSessionRequired(verifyUserEmailWithoutToken)).Methods("POST")
	api.BaseRoutes.User.Handle("/terms_of_service", api.ApiSessionRequired(saveUserTermsOfService)).Methods("POST")
	api.BaseRoutes.User.Handle("/terms_of_service", api.ApiSessionRequired(getUserTermsOfService)).Methods("GET")

	api.BaseRoutes.User.Handle("/auth", api.ApiSessionRequiredTrustRequester(updateUserAuth)).Methods("PUT")

	api.BaseRoutes.Users.Handle("/mfa", api.ApiHandler(checkUserMfa)).Methods("POST")
	api.BaseRoutes.User.Handle("/mfa", api.ApiSessionRequiredMfa(updateUserMfa)).Methods("PUT")
	api.BaseRoutes.User.Handle("/mfa/generate", api.ApiSessionRequiredMfa(generateMfaSecret)).Methods("POST")

	api.BaseRoutes.Users.Handle("/login", api.ApiHandler(login)).Methods("POST")
	api.BaseRoutes.Users.Handle("/login/switch", api.ApiHandler(switchAccountType)).Methods("POST")
	api.BaseRoutes.Users.Handle("/login/cws", api.ApiHandlerTrustRequester(loginCWS)).Methods("POST")
	api.BaseRoutes.Users.Handle("/logout", api.ApiHandler(logout)).Methods("POST")

	api.BaseRoutes.UserByUsername.Handle("", api.ApiSessionRequired(getUserByUsername)).Methods("GET")
	api.BaseRoutes.UserByEmail.Handle("", api.ApiSessionRequired(getUserByEmail)).Methods("GET")

	api.BaseRoutes.User.Handle("/sessions", api.ApiSessionRequired(getSessions)).Methods("GET")
	api.BaseRoutes.User.Handle("/sessions/revoke", api.ApiSessionRequired(revokeSession)).Methods("POST")
	api.BaseRoutes.User.Handle("/sessions/revoke/all", api.ApiSessionRequired(revokeAllSessionsForUser)).Methods("POST")
	api.BaseRoutes.Users.Handle("/sessions/revoke/all", api.ApiSessionRequired(revokeAllSessionsAllUsers)).Methods("POST")
	api.BaseRoutes.Users.Handle("/sessions/device", api.ApiSessionRequired(attachDeviceId)).Methods("PUT")
	api.BaseRoutes.User.Handle("/audits", api.ApiSessionRequired(getUserAudits)).Methods("GET")

	api.BaseRoutes.User.Handle("/tokens", api.ApiSessionRequired(createUserAccessToken)).Methods("POST")
	api.BaseRoutes.User.Handle("/tokens", api.ApiSessionRequired(getUserAccessTokensForUser)).Methods("GET")
	api.BaseRoutes.Users.Handle("/tokens", api.ApiSessionRequired(getUserAccessTokens)).Methods("GET")
	api.BaseRoutes.Users.Handle("/tokens/search", api.ApiSessionRequired(searchUserAccessTokens)).Methods("POST")
	api.BaseRoutes.Users.Handle("/tokens/{token_id:[A-Za-z0-9]+}", api.ApiSessionRequired(getUserAccessToken)).Methods("GET")
	api.BaseRoutes.Users.Handle("/tokens/revoke", api.ApiSessionRequired(revokeUserAccessToken)).Methods("POST")
	api.BaseRoutes.Users.Handle("/tokens/disable", api.ApiSessionRequired(disableUserAccessToken)).Methods("POST")
	api.BaseRoutes.Users.Handle("/tokens/enable", api.ApiSessionRequired(enableUserAccessToken)).Methods("POST")

	api.BaseRoutes.User.Handle("/typing", api.ApiSessionRequiredDisableWhenBusy(publishUserTyping)).Methods("POST")

	api.BaseRoutes.Users.Handle("/migrate_auth/ldap", api.ApiSessionRequired(migrateAuthToLDAP)).Methods("POST")
	api.BaseRoutes.Users.Handle("/migrate_auth/saml", api.ApiSessionRequired(migrateAuthToSaml)).Methods("POST")

	api.BaseRoutes.User.Handle("/uploads", api.ApiSessionRequired(getUploadsForUser)).Methods("GET")

	api.BaseRoutes.UserThreads.Handle("", api.ApiSessionRequired(getThreadsForUser)).Methods("GET")
	api.BaseRoutes.UserThreads.Handle("/read", api.ApiSessionRequired(updateReadStateAllThreadsByUser)).Methods("PUT")

	api.BaseRoutes.UserThread.Handle("", api.ApiSessionRequired(getThreadForUser)).Methods("GET")
	api.BaseRoutes.UserThread.Handle("/following", api.ApiSessionRequired(followThreadByUser)).Methods("PUT")
	api.BaseRoutes.UserThread.Handle("/following", api.ApiSessionRequired(unfollowThreadByUser)).Methods("DELETE")
	api.BaseRoutes.UserThread.Handle("/read/{timestamp:[0-9]+}", api.ApiSessionRequired(updateReadStateThreadByUser)).Methods("PUT")
}

func createUser(c *Context, w http.ResponseWriter, r *http.Request) {
	user := model.UserFromJson(r.Body)
	if user == nil {
		c.SetInvalidParam("user")
		return
	}

	user.SanitizeInput(c.IsSystemAdmin())

	tokenId := r.URL.Query().Get("t")
	inviteId := r.URL.Query().Get("iid")
	redirect := r.URL.Query().Get("r")

	auditRec := c.MakeAuditRecord("createUser", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("invite_id", inviteId)
	auditRec.AddMeta("user", user)

	// No permission check required

	var ruser *model.User
	var err *model.AppError
	if tokenId != "" {
		token, nErr := c.App.Srv().Store.Token().GetByToken(tokenId)
		if nErr != nil {
			var status int
			switch nErr.(type) {
			case *store.ErrNotFound:
				status = http.StatusNotFound
			default:
				status = http.StatusInternalServerError
			}
			c.Err = model.NewAppError("CreateUserWithToken", "api.user.create_user.signup_link_invalid.app_error", nil, nErr.Error(), status)
			return
		}
		auditRec.AddMeta("token_type", token.Type)

		if token.Type == app.TokenTypeGuestInvitation {
			if c.App.Srv().License() == nil {
				c.Err = model.NewAppError("CreateUserWithToken", "api.user.create_user.guest_accounts.license.app_error", nil, "", http.StatusBadRequest)
				return
			}
			if !*c.App.Config().GuestAccountsSettings.Enable {
				c.Err = model.NewAppError("CreateUserWithToken", "api.user.create_user.guest_accounts.disabled.app_error", nil, "", http.StatusBadRequest)
				return
			}
		}
		ruser, err = c.App.CreateUserWithToken(c.AppContext, user, token)
	} else if inviteId != "" {
		ruser, err = c.App.CreateUserWithInviteId(c.AppContext, user, inviteId, redirect)
	} else if c.IsSystemAdmin() {
		ruser, err = c.App.CreateUserAsAdmin(c.AppContext, user, redirect)
		auditRec.AddMeta("admin", true)
	} else {
		ruser, err = c.App.CreateUserFromSignup(c.AppContext, user, redirect)
	}

	if err != nil {
		c.Err = err
		return
	}

	// New user created, check cloud limits and send emails if needed
	// Soft fail on error since user is already created
	if ruser != nil {
		err = c.App.CheckAndSendUserLimitWarningEmails(c.AppContext)
		if err != nil {
			c.LogErrorByCode(err)
		}
	}

	auditRec.Success()
	auditRec.AddMeta("user", ruser) // overwrite meta

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(ruser.ToJson()))
}

func getUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserId, c.Params.UserId)
	if err != nil {
		c.SetPermissionError(model.PERMISSION_VIEW_MEMBERS)
		return
	}

	if !canSee {
		c.SetPermissionError(model.PERMISSION_VIEW_MEMBERS)
		return
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	if c.IsSystemAdmin() || c.AppContext.Session().UserId == user.Id {
		userTermsOfService, err := c.App.GetUserTermsOfService(user.Id)
		if err != nil && err.StatusCode != http.StatusNotFound {
			c.Err = err
			return
		}

		if userTermsOfService != nil {
			user.TermsOfServiceId = userTermsOfService.TermsOfServiceId
			user.TermsOfServiceCreateAt = userTermsOfService.CreateAt
		}
	}

	etag := user.Etag(*c.App.Config().PrivacySettings.ShowFullName, *c.App.Config().PrivacySettings.ShowEmailAddress)

	if c.HandleEtag(etag, "Get User", w, r) {
		return
	}

	if c.AppContext.Session().UserId == user.Id {
		user.Sanitize(map[string]bool{})
	} else {
		c.App.SanitizeProfile(user, c.IsSystemAdmin())
	}
	c.App.UpdateLastActivityAtIfNeeded(*c.AppContext.Session())
	w.Header().Set(model.HEADER_ETAG_SERVER, etag)
	w.Write([]byte(user.ToJson()))
}

func getUserByUsername(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUsername()
	if c.Err != nil {
		return
	}

	user, err := c.App.GetUserByUsername(c.Params.Username)
	if err != nil {
		restrictions, err2 := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserId)
		if err2 != nil {
			c.Err = err2
			return
		}
		if restrictions != nil {
			c.SetPermissionError(model.PERMISSION_VIEW_MEMBERS)
			return
		}
		c.Err = err
		return
	}

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserId, user.Id)
	if err != nil {
		c.Err = err
		return
	}

	if !canSee {
		c.SetPermissionError(model.PERMISSION_VIEW_MEMBERS)
		return
	}

	if c.IsSystemAdmin() || c.AppContext.Session().UserId == user.Id {
		userTermsOfService, err := c.App.GetUserTermsOfService(user.Id)
		if err != nil && err.StatusCode != http.StatusNotFound {
			c.Err = err
			return
		}

		if userTermsOfService != nil {
			user.TermsOfServiceId = userTermsOfService.TermsOfServiceId
			user.TermsOfServiceCreateAt = userTermsOfService.CreateAt
		}
	}

	etag := user.Etag(*c.App.Config().PrivacySettings.ShowFullName, *c.App.Config().PrivacySettings.ShowEmailAddress)

	if c.HandleEtag(etag, "Get User", w, r) {
		return
	}

	if c.AppContext.Session().UserId == user.Id {
		user.Sanitize(map[string]bool{})
	} else {
		c.App.SanitizeProfile(user, c.IsSystemAdmin())
	}
	w.Header().Set(model.HEADER_ETAG_SERVER, etag)
	w.Write([]byte(user.ToJson()))
}

func getUserByEmail(c *Context, w http.ResponseWriter, r *http.Request) {
	c.SanitizeEmail()
	if c.Err != nil {
		return
	}

	sanitizeOptions := c.App.GetSanitizeOptions(c.IsSystemAdmin())
	if !sanitizeOptions["email"] {
		c.Err = model.NewAppError("getUserByEmail", "api.user.get_user_by_email.permissions.app_error", nil, "userId="+c.AppContext.Session().UserId, http.StatusForbidden)
		return
	}

	user, err := c.App.GetUserByEmail(c.Params.Email)
	if err != nil {
		restrictions, err2 := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserId)
		if err2 != nil {
			c.Err = err2
			return
		}
		if restrictions != nil {
			c.SetPermissionError(model.PERMISSION_VIEW_MEMBERS)
			return
		}
		c.Err = err
		return
	}

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserId, user.Id)
	if err != nil {
		c.Err = err
		return
	}

	if !canSee {
		c.SetPermissionError(model.PERMISSION_VIEW_MEMBERS)
		return
	}

	etag := user.Etag(*c.App.Config().PrivacySettings.ShowFullName, *c.App.Config().PrivacySettings.ShowEmailAddress)

	if c.HandleEtag(etag, "Get User", w, r) {
		return
	}

	c.App.SanitizeProfile(user, c.IsSystemAdmin())
	w.Header().Set(model.HEADER_ETAG_SERVER, etag)
	w.Write([]byte(user.ToJson()))
}

func getDefaultProfileImage(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserId, c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	if !canSee {
		c.SetPermissionError(model.PERMISSION_VIEW_MEMBERS)
		return
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	img, err := c.App.GetDefaultProfileImage(user)
	if err != nil {
		c.Err = err
		return
	}

	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%v, private", 24*60*60)) // 24 hrs
	w.Header().Set("Content-Type", "image/png")
	w.Write(img)
}

func getProfileImage(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserId, c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	if !canSee {
		c.SetPermissionError(model.PERMISSION_VIEW_MEMBERS)
		return
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	etag := strconv.FormatInt(user.LastPictureUpdate, 10)
	if c.HandleEtag(etag, "Get Profile Image", w, r) {
		return
	}

	img, readFailed, err := c.App.GetProfileImage(user)
	if err != nil {
		c.Err = err
		return
	}

	if readFailed {
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%v, private", 5*60)) // 5 mins
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%v, private", 24*60*60)) // 24 hrs
		w.Header().Set(model.HEADER_ETAG_SERVER, etag)
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(img)
}

func setProfileImage(c *Context, w http.ResponseWriter, r *http.Request) {
	defer io.Copy(ioutil.Discard, r.Body)

	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if *c.App.Config().FileSettings.DriverName == "" {
		c.Err = model.NewAppError("uploadProfileImage", "api.user.upload_profile_user.storage.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	if r.ContentLength > *c.App.Config().FileSettings.MaxFileSize {
		c.Err = model.NewAppError("uploadProfileImage", "api.user.upload_profile_user.too_large.app_error", nil, "", http.StatusRequestEntityTooLarge)
		return
	}

	if err := r.ParseMultipartForm(*c.App.Config().FileSettings.MaxFileSize); err != nil {
		c.Err = model.NewAppError("uploadProfileImage", "api.user.upload_profile_user.parse.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	m := r.MultipartForm
	imageArray, ok := m.File["image"]
	if !ok {
		c.Err = model.NewAppError("uploadProfileImage", "api.user.upload_profile_user.no_file.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if len(imageArray) <= 0 {
		c.Err = model.NewAppError("uploadProfileImage", "api.user.upload_profile_user.array.app_error", nil, "", http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord("setProfileImage", audit.Fail)
	defer c.LogAuditRec(auditRec)
	if imageArray[0] != nil {
		auditRec.AddMeta("filename", imageArray[0].Filename)
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.SetInvalidUrlParam("user_id")
		return
	}
	auditRec.AddMeta("user", user)

	if (user.IsLDAPUser() || (user.IsSAMLUser() && *c.App.Config().SamlSettings.EnableSyncWithLdap)) &&
		*c.App.Config().LdapSettings.PictureAttribute != "" {
		c.Err = model.NewAppError(
			"uploadProfileImage", "api.user.upload_profile_user.login_provider_attribute_set.app_error",
			nil, "", http.StatusConflict)
		return
	}

	imageData := imageArray[0]
	if err := c.App.SetProfileImage(c.Params.UserId, imageData); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func setDefaultProfileImage(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if *c.App.Config().FileSettings.DriverName == "" {
		c.Err = model.NewAppError("setDefaultProfileImage", "api.user.upload_profile_user.storage.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	auditRec := c.MakeAuditRecord("setDefaultProfileImage", audit.Fail)
	defer c.LogAuditRec(auditRec)

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("user", user)

	if err := c.App.SetDefaultProfileImage(user); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func getTotalUsersStats(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.Err != nil {
		return
	}

	restrictions, err := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = err
		return
	}

	stats, err := c.App.GetTotalUsersStats(restrictions)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(stats.ToJson()))
}

func getFilteredUsersStats(c *Context, w http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("in_team")
	channelID := r.URL.Query().Get("in_channel")
	includeDeleted := r.URL.Query().Get("include_deleted")
	includeBotAccounts := r.URL.Query().Get("include_bots")
	rolesString := r.URL.Query().Get("roles")
	channelRolesString := r.URL.Query().Get("channel_roles")
	teamRolesString := r.URL.Query().Get("team_roles")

	includeDeletedBool, _ := strconv.ParseBool(includeDeleted)
	includeBotAccountsBool, _ := strconv.ParseBool(includeBotAccounts)

	roles := []string{}
	var rolesValid bool
	if rolesString != "" {
		roles, rolesValid = model.CleanRoleNames(strings.Split(rolesString, ","))
		if !rolesValid {
			c.SetInvalidParam("roles")
			return
		}
	}
	channelRoles := []string{}
	if channelRolesString != "" && channelID != "" {
		channelRoles, rolesValid = model.CleanRoleNames(strings.Split(channelRolesString, ","))
		if !rolesValid {
			c.SetInvalidParam("channelRoles")
			return
		}
	}
	teamRoles := []string{}
	if teamRolesString != "" && teamID != "" {
		teamRoles, rolesValid = model.CleanRoleNames(strings.Split(teamRolesString, ","))
		if !rolesValid {
			c.SetInvalidParam("teamRoles")
			return
		}
	}

	options := &model.UserCountOptions{
		IncludeDeleted:     includeDeletedBool,
		IncludeBotAccounts: includeBotAccountsBool,
		TeamId:             teamID,
		ChannelId:          channelID,
		Roles:              roles,
		ChannelRoles:       channelRoles,
		TeamRoles:          teamRoles,
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_READ_USERMANAGEMENT_USERS) {
		c.SetPermissionError(model.PERMISSION_SYSCONSOLE_READ_USERMANAGEMENT_USERS)
		return
	}

	stats, err := c.App.GetFilteredUsersStats(options)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(stats.ToJson()))
}

func getUsersByGroupChannelIds(c *Context, w http.ResponseWriter, r *http.Request) {
	channelIds := model.ArrayFromJson(r.Body)

	if len(channelIds) == 0 {
		c.SetInvalidParam("channel_ids")
		return
	}

	usersByChannelId, err := c.App.GetUsersByGroupChannelIds(c.AppContext, channelIds, c.IsSystemAdmin())
	if err != nil {
		c.Err = err
		return
	}

	b, _ := json.Marshal(usersByChannelId)
	w.Write(b)
}

func getUsers(c *Context, w http.ResponseWriter, r *http.Request) {
	inTeamId := r.URL.Query().Get("in_team")
	notInTeamId := r.URL.Query().Get("not_in_team")
	inChannelId := r.URL.Query().Get("in_channel")
	inGroupId := r.URL.Query().Get("in_group")
	notInChannelId := r.URL.Query().Get("not_in_channel")
	groupConstrained := r.URL.Query().Get("group_constrained")
	withoutTeam := r.URL.Query().Get("without_team")
	inactive := r.URL.Query().Get("inactive")
	active := r.URL.Query().Get("active")
	role := r.URL.Query().Get("role")
	sort := r.URL.Query().Get("sort")
	rolesString := r.URL.Query().Get("roles")
	channelRolesString := r.URL.Query().Get("channel_roles")
	teamRolesString := r.URL.Query().Get("team_roles")

	if notInChannelId != "" && inTeamId == "" {
		c.SetInvalidUrlParam("team_id")
		return
	}

	if sort != "" && sort != "last_activity_at" && sort != "create_at" && sort != "status" {
		c.SetInvalidUrlParam("sort")
		return
	}

	// Currently only supports sorting on a team
	// or sort="status" on inChannelId
	if (sort == "last_activity_at" || sort == "create_at") && (inTeamId == "" || notInTeamId != "" || inChannelId != "" || notInChannelId != "" || withoutTeam != "" || inGroupId != "") {
		c.SetInvalidUrlParam("sort")
		return
	}
	if sort == "status" && inChannelId == "" {
		c.SetInvalidUrlParam("sort")
		return
	}

	withoutTeamBool, _ := strconv.ParseBool(withoutTeam)
	groupConstrainedBool, _ := strconv.ParseBool(groupConstrained)
	inactiveBool, _ := strconv.ParseBool(inactive)
	activeBool, _ := strconv.ParseBool(active)

	if inactiveBool && activeBool {
		c.SetInvalidUrlParam("inactive")
	}

	roles := []string{}
	var rolesValid bool
	if rolesString != "" {
		roles, rolesValid = model.CleanRoleNames(strings.Split(rolesString, ","))
		if !rolesValid {
			c.SetInvalidParam("roles")
			return
		}
	}
	channelRoles := []string{}
	if channelRolesString != "" && inChannelId != "" {
		channelRoles, rolesValid = model.CleanRoleNames(strings.Split(channelRolesString, ","))
		if !rolesValid {
			c.SetInvalidParam("channelRoles")
			return
		}
	}
	teamRoles := []string{}
	if teamRolesString != "" && inTeamId != "" {
		teamRoles, rolesValid = model.CleanRoleNames(strings.Split(teamRolesString, ","))
		if !rolesValid {
			c.SetInvalidParam("teamRoles")
			return
		}
	}

	restrictions, err := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = err
		return
	}

	userGetOptions := &model.UserGetOptions{
		InTeamId:         inTeamId,
		InChannelId:      inChannelId,
		NotInTeamId:      notInTeamId,
		NotInChannelId:   notInChannelId,
		InGroupId:        inGroupId,
		GroupConstrained: groupConstrainedBool,
		WithoutTeam:      withoutTeamBool,
		Inactive:         inactiveBool,
		Active:           activeBool,
		Role:             role,
		Roles:            roles,
		ChannelRoles:     channelRoles,
		TeamRoles:        teamRoles,
		Sort:             sort,
		Page:             c.Params.Page,
		PerPage:          c.Params.PerPage,
		ViewRestrictions: restrictions,
	}

	var profiles []*model.User
	etag := ""

	if withoutTeamBool, _ := strconv.ParseBool(withoutTeam); withoutTeamBool {
		// Use a special permission for now
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_LIST_USERS_WITHOUT_TEAM) {
			c.SetPermissionError(model.PERMISSION_LIST_USERS_WITHOUT_TEAM)
			return
		}

		profiles, err = c.App.GetUsersWithoutTeamPage(userGetOptions, c.IsSystemAdmin())
	} else if notInChannelId != "" {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), notInChannelId, model.PERMISSION_READ_CHANNEL) {
			c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
			return
		}

		profiles, err = c.App.GetUsersNotInChannelPage(inTeamId, notInChannelId, groupConstrainedBool, c.Params.Page, c.Params.PerPage, c.IsSystemAdmin(), restrictions)
	} else if notInTeamId != "" {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), notInTeamId, model.PERMISSION_VIEW_TEAM) {
			c.SetPermissionError(model.PERMISSION_VIEW_TEAM)
			return
		}

		etag = c.App.GetUsersNotInTeamEtag(inTeamId, restrictions.Hash())
		if c.HandleEtag(etag, "Get Users Not in Team", w, r) {
			return
		}

		profiles, err = c.App.GetUsersNotInTeamPage(notInTeamId, groupConstrainedBool, c.Params.Page, c.Params.PerPage, c.IsSystemAdmin(), restrictions)
	} else if inTeamId != "" {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), inTeamId, model.PERMISSION_VIEW_TEAM) {
			c.SetPermissionError(model.PERMISSION_VIEW_TEAM)
			return
		}

		if sort == "last_activity_at" {
			profiles, err = c.App.GetRecentlyActiveUsersForTeamPage(inTeamId, c.Params.Page, c.Params.PerPage, c.IsSystemAdmin(), restrictions)
		} else if sort == "create_at" {
			profiles, err = c.App.GetNewUsersForTeamPage(inTeamId, c.Params.Page, c.Params.PerPage, c.IsSystemAdmin(), restrictions)
		} else {
			etag = c.App.GetUsersInTeamEtag(inTeamId, restrictions.Hash())
			if c.HandleEtag(etag, "Get Users in Team", w, r) {
				return
			}
			profiles, err = c.App.GetUsersInTeamPage(userGetOptions, c.IsSystemAdmin())
		}
	} else if inChannelId != "" {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), inChannelId, model.PERMISSION_READ_CHANNEL) {
			c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
			return
		}
		if sort == "status" {
			profiles, err = c.App.GetUsersInChannelPageByStatus(userGetOptions, c.IsSystemAdmin())
		} else {
			profiles, err = c.App.GetUsersInChannelPage(userGetOptions, c.IsSystemAdmin())
		}
	} else if inGroupId != "" {
		if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.LDAPGroups {
			c.Err = model.NewAppError("Api4.getUsersInGroup", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
			return
		}

		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_READ_USERMANAGEMENT_GROUPS) {
			c.SetPermissionError(model.PERMISSION_SYSCONSOLE_READ_USERMANAGEMENT_GROUPS)
			return
		}

		profiles, _, err = c.App.GetGroupMemberUsersPage(inGroupId, c.Params.Page, c.Params.PerPage)
		if err != nil {
			c.Err = err
			return
		}
	} else {
		userGetOptions, err = c.App.RestrictUsersGetByPermissions(c.AppContext.Session().UserId, userGetOptions)
		if err != nil {
			c.Err = err
			return
		}
		profiles, err = c.App.GetUsersPage(userGetOptions, c.IsSystemAdmin())
	}

	if err != nil {
		c.Err = err
		return
	}

	if etag != "" {
		w.Header().Set(model.HEADER_ETAG_SERVER, etag)
	}
	c.App.UpdateLastActivityAtIfNeeded(*c.AppContext.Session())
	w.Write([]byte(model.UserListToJson(profiles)))
}

func getUsersByIds(c *Context, w http.ResponseWriter, r *http.Request) {
	userIds := model.ArrayFromJson(r.Body)

	if len(userIds) == 0 {
		c.SetInvalidParam("user_ids")
		return
	}

	sinceString := r.URL.Query().Get("since")

	options := &store.UserGetByIdsOpts{
		IsAdmin: c.IsSystemAdmin(),
	}

	if sinceString != "" {
		since, parseError := strconv.ParseInt(sinceString, 10, 64)
		if parseError != nil {
			c.SetInvalidParam("since")
			return
		}
		options.Since = since
	}

	restrictions, err := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = err
		return
	}
	options.ViewRestrictions = restrictions

	users, err := c.App.GetUsersByIds(userIds, options)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.UserListToJson(users)))
}

func getUsersByNames(c *Context, w http.ResponseWriter, r *http.Request) {
	usernames := model.ArrayFromJson(r.Body)

	if len(usernames) == 0 {
		c.SetInvalidParam("usernames")
		return
	}

	restrictions, err := c.App.GetViewUsersRestrictions(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = err
		return
	}

	users, err := c.App.GetUsersByUsernames(usernames, c.IsSystemAdmin(), restrictions)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.UserListToJson(users)))
}

func getKnownUsers(c *Context, w http.ResponseWriter, r *http.Request) {
	userIds, err := c.App.GetKnownUsers(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = err
		return
	}

	data, _ := json.Marshal(userIds)

	w.Write(data)
}

func searchUsers(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.UserSearchFromJson(r.Body)
	if props == nil {
		c.SetInvalidParam("")
		return
	}

	if props.Term == "" {
		c.SetInvalidParam("term")
		return
	}

	if props.TeamId == "" && props.NotInChannelId != "" {
		c.SetInvalidParam("team_id")
		return
	}

	if props.InGroupId != "" {
		if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.LDAPGroups {
			c.Err = model.NewAppError("Api4.searchUsers", "api.ldap_groups.license_error", nil, "", http.StatusNotImplemented)
			return
		}

		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
			c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
			return
		}
	}

	if props.InChannelId != "" && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), props.InChannelId, model.PERMISSION_READ_CHANNEL) {
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	if props.NotInChannelId != "" && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), props.NotInChannelId, model.PERMISSION_READ_CHANNEL) {
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	if props.TeamId != "" && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), props.TeamId, model.PERMISSION_VIEW_TEAM) {
		c.SetPermissionError(model.PERMISSION_VIEW_TEAM)
		return
	}

	if props.NotInTeamId != "" && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), props.NotInTeamId, model.PERMISSION_VIEW_TEAM) {
		c.SetPermissionError(model.PERMISSION_VIEW_TEAM)
		return
	}

	if props.Limit <= 0 || props.Limit > model.USER_SEARCH_MAX_LIMIT {
		c.SetInvalidParam("limit")
		return
	}

	options := &model.UserSearchOptions{
		IsAdmin:          c.IsSystemAdmin(),
		AllowInactive:    props.AllowInactive,
		GroupConstrained: props.GroupConstrained,
		Limit:            props.Limit,
		Role:             props.Role,
		Roles:            props.Roles,
		ChannelRoles:     props.ChannelRoles,
		TeamRoles:        props.TeamRoles,
	}

	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		options.AllowEmails = true
		options.AllowFullNames = true
	} else {
		options.AllowEmails = *c.App.Config().PrivacySettings.ShowEmailAddress
		options.AllowFullNames = *c.App.Config().PrivacySettings.ShowFullName
	}

	options, err := c.App.RestrictUsersSearchByPermissions(c.AppContext.Session().UserId, options)
	if err != nil {
		c.Err = err
		return
	}

	profiles, err := c.App.SearchUsers(props, options)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.UserListToJson(profiles)))
}

func autocompleteUsers(c *Context, w http.ResponseWriter, r *http.Request) {
	channelId := r.URL.Query().Get("in_channel")
	teamId := r.URL.Query().Get("in_team")
	name := r.URL.Query().Get("name")
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limitStr == "" {
		limit = model.USER_SEARCH_DEFAULT_LIMIT
	} else if limit > model.USER_SEARCH_MAX_LIMIT {
		limit = model.USER_SEARCH_MAX_LIMIT
	}

	options := &model.UserSearchOptions{
		IsAdmin: c.IsSystemAdmin(),
		// Never autocomplete on emails.
		AllowEmails: false,
		Limit:       limit,
	}

	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		options.AllowFullNames = true
	} else {
		options.AllowFullNames = *c.App.Config().PrivacySettings.ShowFullName
	}

	if channelId != "" {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channelId, model.PERMISSION_READ_CHANNEL) {
			c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
			return
		}
	}

	if teamId != "" {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamId, model.PERMISSION_VIEW_TEAM) {
			c.SetPermissionError(model.PERMISSION_VIEW_TEAM)
			return
		}
	}

	var autocomplete model.UserAutocomplete

	var err *model.AppError
	options, err = c.App.RestrictUsersSearchByPermissions(c.AppContext.Session().UserId, options)
	if err != nil {
		c.Err = err
		return
	}

	if channelId != "" {
		// We're using the channelId to search for users inside that channel and the team
		// to get the not in channel list. Also we want to include the DM and GM users for
		// that team which could only be obtained having the team id.
		if teamId == "" {
			c.Err = model.NewAppError("autocompleteUser",
				"api.user.autocomplete_users.missing_team_id.app_error",
				nil,
				"channelId="+channelId,
				http.StatusInternalServerError,
			)
			return
		}
		result, err := c.App.AutocompleteUsersInChannel(teamId, channelId, name, options)
		if err != nil {
			c.Err = err
			return
		}

		autocomplete.Users = result.InChannel
		autocomplete.OutOfChannel = result.OutOfChannel
	} else if teamId != "" {
		result, err := c.App.AutocompleteUsersInTeam(teamId, name, options)
		if err != nil {
			c.Err = err
			return
		}

		autocomplete.Users = result.InTeam
	} else {
		result, err := c.App.SearchUsersInTeam("", name, options)
		if err != nil {
			c.Err = err
			return
		}
		autocomplete.Users = result
	}

	w.Write([]byte((autocomplete.ToJson())))
}

func updateUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	user := model.UserFromJson(r.Body)
	if user == nil {
		c.SetInvalidParam("user")
		return
	}

	// The user being updated in the payload must be the same one as indicated in the URL.
	if user.Id != c.Params.UserId {
		c.SetInvalidParam("user_id")
		return
	}

	auditRec := c.MakeAuditRecord("updateUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	// Cannot update a system admin unless user making request is a systemadmin also.
	if user.IsSystemAdmin() && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), user.Id) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	ouser, err := c.App.GetUser(user.Id)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("user", ouser)

	if c.AppContext.Session().IsOAuth {
		if ouser.Email != user.Email {
			c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
			c.Err.DetailedError += ", attempted email update by oauth app"
			return
		}
	}

	// Check that the fields being updated are not set by the login provider
	conflictField := c.App.CheckProviderAttributes(ouser, user.ToPatch())
	if conflictField != "" {
		c.Err = model.NewAppError(
			"updateUser", "api.user.update_user.login_provider_attribute_set.app_error",
			map[string]interface{}{"Field": conflictField}, "", http.StatusConflict)
		return
	}

	// If eMail update is attempted by the currently logged in user, check if correct password was provided
	if user.Email != "" && ouser.Email != user.Email && c.AppContext.Session().UserId == c.Params.UserId {
		err = c.App.DoubleCheckPassword(ouser, user.Password)
		if err != nil {
			c.SetInvalidParam("password")
			return
		}
	}

	ruser, err := c.App.UpdateUserAsUser(user, c.IsSystemAdmin())
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("update", ruser)
	c.LogAudit("")

	w.Write([]byte(ruser.ToJson()))
}

func patchUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	patch := model.UserPatchFromJson(r.Body)
	if patch == nil {
		c.SetInvalidParam("user")
		return
	}

	auditRec := c.MakeAuditRecord("patchUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	ouser, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.SetInvalidParam("user_id")
		return
	}
	auditRec.AddMeta("user", ouser)

	// Cannot update a system admin unless user making request is a systemadmin also
	if ouser.IsSystemAdmin() && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	if c.AppContext.Session().IsOAuth && patch.Email != nil {
		if ouser.Email != *patch.Email {
			c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
			c.Err.DetailedError += ", attempted email update by oauth app"
			return
		}
	}

	conflictField := c.App.CheckProviderAttributes(ouser, patch)
	if conflictField != "" {
		c.Err = model.NewAppError(
			"patchUser", "api.user.patch_user.login_provider_attribute_set.app_error",
			map[string]interface{}{"Field": conflictField}, "", http.StatusConflict)
		return
	}

	// If eMail update is attempted by the currently logged in user, check if correct password was provided
	if patch.Email != nil && ouser.Email != *patch.Email && c.AppContext.Session().UserId == c.Params.UserId {
		if patch.Password == nil {
			c.SetInvalidParam("password")
			return
		}

		if err = c.App.DoubleCheckPassword(ouser, *patch.Password); err != nil {
			c.Err = err
			return
		}
	}

	ruser, err := c.App.PatchUser(c.Params.UserId, patch, c.IsSystemAdmin())
	if err != nil {
		c.Err = err
		return
	}

	c.App.SetAutoResponderStatus(ruser, ouser.NotifyProps)

	auditRec.Success()
	auditRec.AddMeta("patch", ruser)
	c.LogAudit("")

	w.Write([]byte(ruser.ToJson()))
}

func deleteUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	userId := c.Params.UserId

	auditRec := c.MakeAuditRecord("deleteUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), userId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	// if EnableUserDeactivation flag is disabled the user cannot deactivate himself.
	if c.Params.UserId == c.AppContext.Session().UserId && !*c.App.Config().TeamSettings.EnableUserDeactivation && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.Err = model.NewAppError("deleteUser", "api.user.update_active.not_enable.app_error", nil, "userId="+c.Params.UserId, http.StatusUnauthorized)
		return
	}

	user, err := c.App.GetUser(userId)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("user", user)

	// Cannot update a system admin unless user making request is a systemadmin also
	if user.IsSystemAdmin() && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	if c.Params.Permanent {
		if *c.App.Config().ServiceSettings.EnableAPIUserDeletion {
			err = c.App.PermanentDeleteUser(c.AppContext, user)
		} else {
			err = model.NewAppError("deleteUser", "api.user.delete_user.not_enabled.app_error", nil, "userId="+c.Params.UserId, http.StatusUnauthorized)
		}
	} else {
		_, err = c.App.UpdateActive(c.AppContext, user, false)
	}
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func updateUserRoles(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	props := model.MapFromJson(r.Body)

	newRoles := props["roles"]
	if !model.IsValidUserRoles(newRoles) {
		c.SetInvalidParam("roles")
		return
	}

	// require license feature to assign "new system roles"
	for _, roleName := range strings.Fields(newRoles) {
		for _, id := range model.NewSystemRoleIDs {
			if roleName == id {
				if license := c.App.Srv().License(); license == nil || !*license.Features.CustomPermissionsSchemes {
					c.Err = model.NewAppError("updateUserRoles", "api.user.update_user_roles.license.app_error", nil, "", http.StatusBadRequest)
					return
				}
			}
		}
	}

	auditRec := c.MakeAuditRecord("updateUserRoles", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("roles", newRoles)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_ROLES) {
		c.SetPermissionError(model.PERMISSION_MANAGE_ROLES)
		return
	}

	user, err := c.App.UpdateUserRoles(c.Params.UserId, newRoles, true)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("user", user)
	c.LogAudit(fmt.Sprintf("user=%s roles=%s", c.Params.UserId, newRoles))

	ReturnStatusOK(w)
}

func updateUserActive(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	props := model.StringInterfaceFromJson(r.Body)

	active, ok := props["active"].(bool)
	if !ok {
		c.SetInvalidParam("active")
		return
	}

	auditRec := c.MakeAuditRecord("updateUserActive", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("active", active)

	// true when you're trying to de-activate yourself
	isSelfDeactive := !active && c.Params.UserId == c.AppContext.Session().UserId

	if !isSelfDeactive && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_USERS) {
		c.Err = model.NewAppError("updateUserActive", "api.user.update_active.permissions.app_error", nil, "userId="+c.Params.UserId, http.StatusForbidden)
		return
	}

	// if EnableUserDeactivation flag is disabled the user cannot deactivate himself.
	if isSelfDeactive && !*c.App.Config().TeamSettings.EnableUserDeactivation {
		c.Err = model.NewAppError("updateUserActive", "api.user.update_active.not_enable.app_error", nil, "userId="+c.Params.UserId, http.StatusUnauthorized)
		return
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("user", user)

	if user.IsSystemAdmin() && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	if active && user.IsGuest() && !*c.App.Config().GuestAccountsSettings.Enable {
		c.Err = model.NewAppError("updateUserActive", "api.user.update_active.cannot_enable_guest_when_guest_feature_is_disabled.app_error", nil, "userId="+c.Params.UserId, http.StatusUnauthorized)
		return
	}

	// if non cloud instances, isOverLimit is false and no error
	isAtLimit, err := c.App.CheckCloudAccountAtLimit()
	if err != nil {
		c.Err = model.NewAppError("updateUserActive", "api.user.update_active.cloud_at_limit_check_error", nil, "userId="+c.Params.UserId, http.StatusInternalServerError)
		return
	}

	if active && isAtLimit {
		c.Err = model.NewAppError("updateUserActive", "api.user.update_active.cloud_at_or_over_limit_check_overcapacity", nil, "userId="+c.Params.UserId, http.StatusBadRequest)
		return
	}

	if _, err = c.App.UpdateActive(c.AppContext, user, active); err != nil {
		c.Err = err
	}

	auditRec.Success()
	c.LogAudit(fmt.Sprintf("user_id=%s active=%v", user.Id, active))

	if isSelfDeactive {
		c.App.Srv().Go(func() {
			if err := c.App.Srv().EmailService.SendDeactivateAccountEmail(user.Email, user.Locale, c.App.GetSiteURL()); err != nil {
				c.LogErrorByCode(model.NewAppError("SendDeactivateEmail", "api.user.send_deactivate_email_and_forget.failed.error", nil, err.Error(), http.StatusInternalServerError))
			}
		})
	}

	message := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_USER_ACTIVATION_STATUS_CHANGE, "", "", "", nil)
	c.App.Publish(message)

	// If activating, run cloud check for limit overages
	if active {
		emailErr := c.App.CheckAndSendUserLimitWarningEmails(c.AppContext)
		if emailErr != nil {
			c.Err = emailErr
			return
		}
	}

	ReturnStatusOK(w)
}

func updateUserAuth(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.IsSystemAdmin() {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	c.RequireUserId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("updateUserAuth", audit.Fail)
	defer c.LogAuditRec(auditRec)

	userAuth := model.UserAuthFromJson(r.Body)
	if userAuth == nil {
		c.SetInvalidParam("user")
		return
	}

	if userAuth.AuthData == nil || *userAuth.AuthData == "" || userAuth.AuthService == "" {
		c.Err = model.NewAppError("updateUserAuth", "api.user.update_user_auth.invalid_request", nil, "", http.StatusBadRequest)
		return
	}

	if user, err := c.App.GetUser(c.Params.UserId); err == nil {
		auditRec.AddMeta("user", user)
	}

	user, err := c.App.UpdateUserAuth(c.Params.UserId, userAuth)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("auth_service", user.AuthService)
	c.LogAudit(fmt.Sprintf("updated user %s auth to service=%v", c.Params.UserId, user.AuthService))

	w.Write([]byte(user.ToJson()))
}

// Deprecated: checkUserMfa is deprecated and should not be used anymore, starting with version 6.0 it will be disabled.
//			   Clients should attempt a login without MFA and will receive a MFA error when it's required.
func checkUserMfa(c *Context, w http.ResponseWriter, r *http.Request) {

	if *c.App.Config().ServiceSettings.DisableLegacyMFA {
		http.NotFound(w, r)
		return
	}

	props := model.MapFromJson(r.Body)

	loginId := props["login_id"]
	if loginId == "" {
		c.SetInvalidParam("login_id")
		return
	}

	resp := map[string]interface{}{}
	resp["mfa_required"] = false

	if !*c.App.Config().ServiceSettings.EnableMultifactorAuthentication {
		w.Write([]byte(model.StringInterfaceToJson(resp)))
		return
	}

	if *c.App.Config().ServiceSettings.ExperimentalEnableHardenedMode {
		resp["mfa_required"] = true
	} else if user, err := c.App.GetUserForLogin("", loginId); err == nil {
		resp["mfa_required"] = user.MfaActive
	}

	w.Write([]byte(model.StringInterfaceToJson(resp)))
}

func updateUserMfa(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("updateUserMfa", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if c.AppContext.Session().IsOAuth {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		c.Err.DetailedError += ", attempted access by oauth app"
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if user, err := c.App.GetUser(c.Params.UserId); err == nil {
		auditRec.AddMeta("user", user)
	}

	props := model.StringInterfaceFromJson(r.Body)
	activate, ok := props["activate"].(bool)
	if !ok {
		c.SetInvalidParam("activate")
		return
	}

	code := ""
	if activate {
		code, ok = props["code"].(string)
		if !ok || code == "" {
			c.SetInvalidParam("code")
			return
		}
	}

	c.LogAudit("attempt")

	if err := c.App.UpdateMfa(activate, c.Params.UserId, code); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("activate", activate)
	c.LogAudit("success - mfa updated")

	ReturnStatusOK(w)
}

func generateMfaSecret(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if c.AppContext.Session().IsOAuth {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		c.Err.DetailedError += ", attempted access by oauth app"
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	secret, err := c.App.GenerateMfaSecret(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write([]byte(secret.ToJson()))
}

func updatePassword(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	props := model.MapFromJson(r.Body)
	newPassword := props["new_password"]

	auditRec := c.MakeAuditRecord("updatePassword", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempted")

	var canUpdatePassword bool
	if user, err := c.App.GetUser(c.Params.UserId); err == nil {
		auditRec.AddMeta("user", user)

		if user.IsSystemAdmin() {
			canUpdatePassword = c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM)
		} else {
			canUpdatePassword = c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_USERS)
		}
	}

	var err *model.AppError

	// There are two main update flows depending on whether the provided password
	// is already hashed or not.
	if props["already_hashed"] == "true" {
		if canUpdatePassword {
			err = c.App.UpdateHashedPasswordByUserId(c.Params.UserId, newPassword)
		} else if c.Params.UserId == c.AppContext.Session().UserId {
			err = model.NewAppError("updatePassword", "api.user.update_password.user_and_hashed.app_error", nil, "", http.StatusUnauthorized)
		} else {
			err = model.NewAppError("updatePassword", "api.user.update_password.context.app_error", nil, "", http.StatusForbidden)
		}
	} else {
		if c.Params.UserId == c.AppContext.Session().UserId {
			currentPassword := props["current_password"]
			if currentPassword == "" {
				c.SetInvalidParam("current_password")
				return
			}

			err = c.App.UpdatePasswordAsUser(c.Params.UserId, currentPassword, newPassword)
		} else if canUpdatePassword {
			err = c.App.UpdatePasswordByUserIdSendEmail(c.Params.UserId, newPassword, c.AppContext.T("api.user.reset_password.method"))
		} else {
			err = model.NewAppError("updatePassword", "api.user.update_password.context.app_error", nil, "", http.StatusForbidden)
		}
	}

	if err != nil {
		c.LogAudit("failed")
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("completed")

	ReturnStatusOK(w)
}

func resetPassword(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.MapFromJson(r.Body)

	token := props["token"]
	if len(token) != model.TOKEN_SIZE {
		c.SetInvalidParam("token")
		return
	}

	newPassword := props["new_password"]

	auditRec := c.MakeAuditRecord("resetPassword", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("token", token)
	c.LogAudit("attempt - token=" + token)

	if err := c.App.ResetPasswordFromToken(token, newPassword); err != nil {
		c.LogAudit("fail - token=" + token)
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success - token=" + token)

	ReturnStatusOK(w)
}

func sendPasswordReset(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.MapFromJson(r.Body)

	email := props["email"]
	email = strings.ToLower(email)
	if email == "" {
		c.SetInvalidParam("email")
		return
	}

	auditRec := c.MakeAuditRecord("sendPasswordReset", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("email", email)

	sent, err := c.App.SendPasswordReset(email, c.App.GetSiteURL())
	if err != nil {
		if *c.App.Config().ServiceSettings.ExperimentalEnableHardenedMode {
			ReturnStatusOK(w)
		} else {
			c.Err = err
		}
		return
	}

	if sent {
		auditRec.Success()
		c.LogAudit("sent=" + email)
	}
	ReturnStatusOK(w)
}

func login(c *Context, w http.ResponseWriter, r *http.Request) {
	// Mask all sensitive errors, with the exception of the following
	defer func() {
		if c.Err == nil {
			return
		}

		unmaskedErrors := []string{
			"mfa.validate_token.authenticate.app_error",
			"api.user.check_user_mfa.bad_code.app_error",
			"api.user.login.blank_pwd.app_error",
			"api.user.login.bot_login_forbidden.app_error",
			"api.user.login.client_side_cert.certificate.app_error",
			"api.user.login.inactive.app_error",
			"api.user.login.not_verified.app_error",
			"api.user.check_user_login_attempts.too_many.app_error",
			"app.team.join_user_to_team.max_accounts.app_error",
			"store.sql_user.save.max_accounts.app_error",
		}

		maskError := true

		for _, unmaskedError := range unmaskedErrors {
			if c.Err.Id == unmaskedError {
				maskError = false
			}
		}

		if !maskError {
			return
		}

		config := c.App.Config()
		enableUsername := *config.EmailSettings.EnableSignInWithUsername
		enableEmail := *config.EmailSettings.EnableSignInWithEmail
		samlEnabled := *config.SamlSettings.Enable
		gitlabEnabled := *config.GitLabSettings.Enable
		openidEnabled := *config.OpenIdSettings.Enable
		googleEnabled := *config.GoogleSettings.Enable
		office365Enabled := *config.Office365Settings.Enable

		if samlEnabled || gitlabEnabled || googleEnabled || office365Enabled || openidEnabled {
			c.Err = model.NewAppError("login", "api.user.login.invalid_credentials_sso", nil, "", http.StatusUnauthorized)
			return
		}

		if enableUsername && !enableEmail {
			c.Err = model.NewAppError("login", "api.user.login.invalid_credentials_username", nil, "", http.StatusUnauthorized)
			return
		}

		if !enableUsername && enableEmail {
			c.Err = model.NewAppError("login", "api.user.login.invalid_credentials_email", nil, "", http.StatusUnauthorized)
			return
		}

		c.Err = model.NewAppError("login", "api.user.login.invalid_credentials_email_username", nil, "", http.StatusUnauthorized)
	}()

	props := model.MapFromJson(r.Body)
	id := props["id"]
	loginId := props["login_id"]
	password := props["password"]
	mfaToken := props["token"]
	deviceId := props["device_id"]
	ldapOnly := props["ldap_only"] == "true"

	if *c.App.Config().ExperimentalSettings.ClientSideCertEnable {
		if license := c.App.Srv().License(); license == nil || !*license.Features.SAML {
			c.Err = model.NewAppError("ClientSideCertNotAllowed", "api.user.login.client_side_cert.license.app_error", nil, "", http.StatusBadRequest)
			return
		}
		certPem, certSubject, certEmail := c.App.CheckForClientSideCert(r)
		mlog.Debug("Client Cert", mlog.String("cert_subject", certSubject), mlog.String("cert_email", certEmail))

		if certPem == "" || certEmail == "" {
			c.Err = model.NewAppError("ClientSideCertMissing", "api.user.login.client_side_cert.certificate.app_error", nil, "", http.StatusBadRequest)
			return
		}

		if *c.App.Config().ExperimentalSettings.ClientSideCertCheck == model.CLIENT_SIDE_CERT_CHECK_PRIMARY_AUTH {
			loginId = certEmail
			password = "certificate"
		}
	}

	auditRec := c.MakeAuditRecord("login", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("login_id", loginId)
	auditRec.AddMeta("device_id", deviceId)

	c.LogAuditWithUserId(id, "attempt - login_id="+loginId)

	user, err := c.App.AuthenticateUserForLogin(c.AppContext, id, loginId, password, mfaToken, "", ldapOnly)
	if err != nil {
		c.LogAuditWithUserId(id, "failure - login_id="+loginId)
		c.Err = err
		return
	}
	auditRec.AddMeta("user", user)

	if user.IsGuest() {
		if c.App.Srv().License() == nil {
			c.Err = model.NewAppError("login", "api.user.login.guest_accounts.license.error", nil, "", http.StatusUnauthorized)
			return
		}
		if !*c.App.Config().GuestAccountsSettings.Enable {
			c.Err = model.NewAppError("login", "api.user.login.guest_accounts.disabled.error", nil, "", http.StatusUnauthorized)
			return
		}
	}

	c.LogAuditWithUserId(user.Id, "authenticated")

	err = c.App.DoLogin(c.AppContext, w, r, user, deviceId, false, false, false)
	if err != nil {
		c.Err = err
		return
	}

	c.LogAuditWithUserId(user.Id, "success")

	if r.Header.Get(model.HEADER_REQUESTED_WITH) == model.HEADER_REQUESTED_WITH_XML {
		c.App.AttachSessionCookies(c.AppContext, w, r)
	}

	userTermsOfService, err := c.App.GetUserTermsOfService(user.Id)
	if err != nil && err.StatusCode != http.StatusNotFound {
		c.Err = err
		return
	}

	if userTermsOfService != nil {
		user.TermsOfServiceId = userTermsOfService.TermsOfServiceId
		user.TermsOfServiceCreateAt = userTermsOfService.CreateAt
	}

	user.Sanitize(map[string]bool{})

	auditRec.Success()
	w.Write([]byte(user.ToJson()))
}

func loginCWS(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.Cloud {
		c.Err = model.NewAppError("loginCWS", "api.user.login_cws.license.error", nil, "", http.StatusUnauthorized)
		return
	}
	r.ParseForm()
	var loginID string
	var token string
	if len(r.Form) > 0 {
		for key, value := range r.Form {
			if key == "login_id" {
				loginID = value[0]
			}
			if key == "cws_token" {
				token = value[0]
			}
		}
	}

	auditRec := c.MakeAuditRecord("login", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("login_id", loginID)
	user, err := c.App.AuthenticateUserForLogin(c.AppContext, "", loginID, "", "", token, false)
	if err != nil {
		c.LogAuditWithUserId("", "failure - login_id="+loginID)
		c.LogErrorByCode(err)
		http.Redirect(w, r, *c.App.Config().ServiceSettings.SiteURL, 302)
		return
	}
	auditRec.AddMeta("user", user)
	c.LogAuditWithUserId(user.Id, "authenticated")
	err = c.App.DoLogin(c.AppContext, w, r, user, "", false, false, false)
	if err != nil {
		c.LogErrorByCode(err)
		http.Redirect(w, r, *c.App.Config().ServiceSettings.SiteURL, 302)
		return
	}
	c.LogAuditWithUserId(user.Id, "success")
	c.App.AttachSessionCookies(c.AppContext, w, r)
	http.Redirect(w, r, *c.App.Config().ServiceSettings.SiteURL, 302)
}

func logout(c *Context, w http.ResponseWriter, r *http.Request) {
	Logout(c, w, r)
}

func Logout(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("Logout", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("")

	c.RemoveSessionCookie(w, r)
	if c.AppContext.Session().Id != "" {
		if err := c.App.RevokeSessionById(c.AppContext.Session().Id); err != nil {
			c.Err = err
			return
		}
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func getSessions(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	sessions, err := c.App.GetSessions(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	for _, session := range sessions {
		session.Sanitize()
	}

	w.Write([]byte(model.SessionsToJson(sessions)))
}

func revokeSession(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("revokeSession", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	props := model.MapFromJson(r.Body)
	sessionId := props["session_id"]
	if sessionId == "" {
		c.SetInvalidParam("session_id")
		return
	}

	session, err := c.App.GetSessionById(sessionId)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("session", session)

	if session.UserId != c.Params.UserId {
		c.SetInvalidUrlParam("user_id")
		return
	}

	if err := c.App.RevokeSession(session); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func revokeAllSessionsForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("revokeAllSessionsForUser", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("user_id", c.Params.UserId)

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if err := c.App.RevokeAllSessions(c.Params.UserId); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func revokeAllSessionsAllUsers(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	auditRec := c.MakeAuditRecord("revokeAllSessionsAllUsers", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if err := c.App.RevokeSessionsFromAllUsers(); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func attachDeviceId(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.MapFromJson(r.Body)

	deviceId := props["device_id"]
	if deviceId == "" {
		c.SetInvalidParam("device_id")
		return
	}

	auditRec := c.MakeAuditRecord("attachDeviceId", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("device_id", deviceId)

	// A special case where we logout of all other sessions with the same device id
	if err := c.App.RevokeSessionsForDeviceId(c.AppContext.Session().UserId, deviceId, c.AppContext.Session().Id); err != nil {
		c.Err = err
		return
	}

	c.App.ClearSessionCacheForUser(c.AppContext.Session().UserId)
	c.App.SetSessionExpireInDays(c.AppContext.Session(), *c.App.Config().ServiceSettings.SessionLengthMobileInDays)

	maxAge := *c.App.Config().ServiceSettings.SessionLengthMobileInDays * 60 * 60 * 24

	secure := false
	if app.GetProtocol(r) == "https" {
		secure = true
	}

	subpath, _ := utils.GetSubpathFromConfig(c.App.Config())

	expiresAt := time.Unix(model.GetMillis()/1000+int64(maxAge), 0)
	sessionCookie := &http.Cookie{
		Name:     model.SESSION_COOKIE_TOKEN,
		Value:    c.AppContext.Session().Token,
		Path:     subpath,
		MaxAge:   maxAge,
		Expires:  expiresAt,
		HttpOnly: true,
		Domain:   c.App.GetCookieDomain(),
		Secure:   secure,
	}

	http.SetCookie(w, sessionCookie)

	if err := c.App.AttachDeviceId(c.AppContext.Session().Id, deviceId, c.AppContext.Session().ExpiresAt); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}

func getUserAudits(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("getUserAudits", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if user, err := c.App.GetUser(c.Params.UserId); err == nil {
		auditRec.AddMeta("user", user)
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	audits, err := c.App.GetAuditsPage(c.Params.UserId, c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("page", c.Params.Page)
	auditRec.AddMeta("audits_per_page", c.Params.LogsPerPage)

	w.Write([]byte(audits.ToJson()))
}

func verifyUserEmail(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.MapFromJson(r.Body)

	token := props["token"]
	if len(token) != model.TOKEN_SIZE {
		c.SetInvalidParam("token")
		return
	}

	auditRec := c.MakeAuditRecord("verifyUserEmail", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if err := c.App.VerifyEmailFromToken(token); err != nil {
		c.Err = model.NewAppError("verifyUserEmail", "api.user.verify_email.bad_link.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	auditRec.Success()
	c.LogAudit("Email Verified")

	ReturnStatusOK(w)
}

func sendVerificationEmail(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.MapFromJson(r.Body)

	email := props["email"]
	email = strings.ToLower(email)
	if email == "" {
		c.SetInvalidParam("email")
		return
	}
	redirect := r.URL.Query().Get("r")

	auditRec := c.MakeAuditRecord("sendVerificationEmail", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("email", email)

	user, err := c.App.GetUserForLogin("", email)
	if err != nil {
		// Don't want to leak whether the email is valid or not
		ReturnStatusOK(w)
		return
	}
	auditRec.AddMeta("user", user)

	if err = c.App.SendEmailVerification(user, user.Email, redirect); err != nil {
		// Don't want to leak whether the email is valid or not
		c.LogErrorByCode(err)
		ReturnStatusOK(w)
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func switchAccountType(c *Context, w http.ResponseWriter, r *http.Request) {
	switchRequest := model.SwitchRequestFromJson(r.Body)
	if switchRequest == nil {
		c.SetInvalidParam("switch_request")
		return
	}

	auditRec := c.MakeAuditRecord("switchAccountType", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("email", switchRequest.Email)
	auditRec.AddMeta("new_service", switchRequest.NewService)
	auditRec.AddMeta("old_service", switchRequest.CurrentService)

	link := ""
	var err *model.AppError

	if switchRequest.EmailToOAuth() {
		link, err = c.App.SwitchEmailToOAuth(w, r, switchRequest.Email, switchRequest.Password, switchRequest.MfaCode, switchRequest.NewService)
	} else if switchRequest.OAuthToEmail() {
		c.SessionRequired()
		if c.Err != nil {
			return
		}

		link, err = c.App.SwitchOAuthToEmail(switchRequest.Email, switchRequest.NewPassword, c.AppContext.Session().UserId)
	} else if switchRequest.EmailToLdap() {
		link, err = c.App.SwitchEmailToLdap(switchRequest.Email, switchRequest.Password, switchRequest.MfaCode, switchRequest.LdapLoginId, switchRequest.NewPassword)
	} else if switchRequest.LdapToEmail() {
		link, err = c.App.SwitchLdapToEmail(switchRequest.Password, switchRequest.MfaCode, switchRequest.Email, switchRequest.NewPassword)
	} else {
		c.SetInvalidParam("switch_request")
		return
	}

	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success")

	w.Write([]byte(model.MapToJson(map[string]string{"follow_link": link})))
}

func createUserAccessToken(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("createUserAccessToken", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if user, err := c.App.GetUser(c.Params.UserId); err == nil {
		auditRec.AddMeta("user", user)
	}

	if c.AppContext.Session().IsOAuth {
		c.SetPermissionError(model.PERMISSION_CREATE_USER_ACCESS_TOKEN)
		c.Err.DetailedError += ", attempted access by oauth app"
		return
	}

	accessToken := model.UserAccessTokenFromJson(r.Body)
	if accessToken == nil {
		c.SetInvalidParam("user_access_token")
		return
	}

	if accessToken.Description == "" {
		c.SetInvalidParam("description")
		return
	}

	c.LogAudit("")

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_CREATE_USER_ACCESS_TOKEN) {
		c.SetPermissionError(model.PERMISSION_CREATE_USER_ACCESS_TOKEN)
		return
	}

	if !c.App.SessionHasPermissionToUserOrBot(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	accessToken.UserId = c.Params.UserId
	accessToken.Token = ""

	accessToken, err := c.App.CreateUserAccessToken(accessToken)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("token_id", accessToken.Id)
	c.LogAudit("success - token_id=" + accessToken.Id)

	w.Write([]byte(accessToken.ToJson()))
}

func searchUserAccessTokens(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}
	props := model.UserAccessTokenSearchFromJson(r.Body)
	if props == nil {
		c.SetInvalidParam("user_access_token_search")
		return
	}

	if props.Term == "" {
		c.SetInvalidParam("term")
		return
	}

	accessTokens, err := c.App.SearchUserAccessTokens(props.Term)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.UserAccessTokenListToJson(accessTokens)))
}

func getUserAccessTokens(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	accessTokens, err := c.App.GetUserAccessTokens(c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.UserAccessTokenListToJson(accessTokens)))
}

func getUserAccessTokensForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_READ_USER_ACCESS_TOKEN) {
		c.SetPermissionError(model.PERMISSION_READ_USER_ACCESS_TOKEN)
		return
	}

	if !c.App.SessionHasPermissionToUserOrBot(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	accessTokens, err := c.App.GetUserAccessTokensForUser(c.Params.UserId, c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.UserAccessTokenListToJson(accessTokens)))
}

func getUserAccessToken(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTokenId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_READ_USER_ACCESS_TOKEN) {
		c.SetPermissionError(model.PERMISSION_READ_USER_ACCESS_TOKEN)
		return
	}

	accessToken, err := c.App.GetUserAccessToken(c.Params.TokenId, true)
	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionToUserOrBot(*c.AppContext.Session(), accessToken.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	w.Write([]byte(accessToken.ToJson()))
}

func revokeUserAccessToken(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.MapFromJson(r.Body)

	tokenId := props["token_id"]
	if tokenId == "" {
		c.SetInvalidParam("token_id")
	}

	auditRec := c.MakeAuditRecord("revokeUserAccessToken", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("token_id", tokenId)
	c.LogAudit("")

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_REVOKE_USER_ACCESS_TOKEN) {
		c.SetPermissionError(model.PERMISSION_REVOKE_USER_ACCESS_TOKEN)
		return
	}

	accessToken, err := c.App.GetUserAccessToken(tokenId, false)
	if err != nil {
		c.Err = err
		return
	}

	if user, errGet := c.App.GetUser(accessToken.UserId); errGet == nil {
		auditRec.AddMeta("user", user)
	}

	if !c.App.SessionHasPermissionToUserOrBot(*c.AppContext.Session(), accessToken.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if err = c.App.RevokeUserAccessToken(accessToken); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success - token_id=" + accessToken.Id)

	ReturnStatusOK(w)
}

func disableUserAccessToken(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.MapFromJson(r.Body)
	tokenId := props["token_id"]

	if tokenId == "" {
		c.SetInvalidParam("token_id")
	}

	auditRec := c.MakeAuditRecord("disableUserAccessToken", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("token_id", tokenId)
	c.LogAudit("")

	// No separate permission for this action for now
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_REVOKE_USER_ACCESS_TOKEN) {
		c.SetPermissionError(model.PERMISSION_REVOKE_USER_ACCESS_TOKEN)
		return
	}

	accessToken, err := c.App.GetUserAccessToken(tokenId, false)
	if err != nil {
		c.Err = err
		return
	}

	if user, errGet := c.App.GetUser(accessToken.UserId); errGet == nil {
		auditRec.AddMeta("user", user)
	}

	if !c.App.SessionHasPermissionToUserOrBot(*c.AppContext.Session(), accessToken.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if err = c.App.DisableUserAccessToken(accessToken); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success - token_id=" + accessToken.Id)

	ReturnStatusOK(w)
}

func enableUserAccessToken(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.MapFromJson(r.Body)

	tokenId := props["token_id"]
	if tokenId == "" {
		c.SetInvalidParam("token_id")
	}

	auditRec := c.MakeAuditRecord("enableUserAccessToken", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("token_id", tokenId)
	c.LogAudit("")

	// No separate permission for this action for now
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_CREATE_USER_ACCESS_TOKEN) {
		c.SetPermissionError(model.PERMISSION_CREATE_USER_ACCESS_TOKEN)
		return
	}

	accessToken, err := c.App.GetUserAccessToken(tokenId, false)
	if err != nil {
		c.Err = err
		return
	}

	if user, errGet := c.App.GetUser(accessToken.UserId); errGet == nil {
		auditRec.AddMeta("user", user)
	}

	if !c.App.SessionHasPermissionToUserOrBot(*c.AppContext.Session(), accessToken.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	if err = c.App.EnableUserAccessToken(accessToken); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("success - token_id=" + accessToken.Id)

	ReturnStatusOK(w)
}

func saveUserTermsOfService(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.StringInterfaceFromJson(r.Body)

	userId := c.AppContext.Session().UserId
	termsOfServiceId, ok := props["termsOfServiceId"].(string)
	if !ok {
		c.SetInvalidParam("termsOfServiceId")
		return
	}
	accepted, ok := props["accepted"].(bool)
	if !ok {
		c.SetInvalidParam("accepted")
		return
	}

	auditRec := c.MakeAuditRecord("saveUserTermsOfService", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("terms_id", termsOfServiceId)
	auditRec.AddMeta("accepted", accepted)

	if user, err := c.App.GetUser(userId); err == nil {
		auditRec.AddMeta("user", user)
	}

	if _, err := c.App.GetTermsOfService(termsOfServiceId); err != nil {
		c.Err = err
		return
	}

	if err := c.App.SaveUserTermsOfService(userId, termsOfServiceId, accepted); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("TermsOfServiceId=" + termsOfServiceId + ", accepted=" + strconv.FormatBool(accepted))

	ReturnStatusOK(w)
}

func getUserTermsOfService(c *Context, w http.ResponseWriter, r *http.Request) {
	userId := c.AppContext.Session().UserId
	result, err := c.App.GetUserTermsOfService(userId)
	if err != nil {
		c.Err = err
		return
	}
	w.Write([]byte(result.ToJson()))
}

func promoteGuestToUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("promoteGuestToUser", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_PROMOTE_GUEST) {
		c.SetPermissionError(model.PERMISSION_PROMOTE_GUEST)
		return
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("user", user)

	if !user.IsGuest() {
		c.Err = model.NewAppError("Api4.promoteGuestToUser", "api.user.promote_guest_to_user.no_guest.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	if err := c.App.PromoteGuestToUser(c.AppContext, user, c.AppContext.Session().UserId); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func demoteUserToGuest(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if c.App.Srv().License() == nil {
		c.Err = model.NewAppError("Api4.demoteUserToGuest", "api.team.demote_user_to_guest.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	if !*c.App.Config().GuestAccountsSettings.Enable {
		c.Err = model.NewAppError("Api4.demoteUserToGuest", "api.team.demote_user_to_guest.disabled.error", nil, "", http.StatusNotImplemented)
		return
	}

	auditRec := c.MakeAuditRecord("demoteUserToGuest", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_DEMOTE_TO_GUEST) {
		c.SetPermissionError(model.PERMISSION_DEMOTE_TO_GUEST)
		return
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	if user.IsSystemAdmin() && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	auditRec.AddMeta("user", user)

	if user.IsGuest() {
		c.Err = model.NewAppError("Api4.demoteUserToGuest", "api.user.demote_user_to_guest.already_guest.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	if err := c.App.DemoteUserToGuest(user); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func publishUserTyping(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	typingRequest := model.TypingRequestFromJson(r.Body)
	if typingRequest == nil {
		c.SetInvalidParam("typing_request")
		return
	}

	if c.Params.UserId != c.AppContext.Session().UserId && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	if !c.App.HasPermissionToChannel(c.Params.UserId, typingRequest.ChannelId, model.PERMISSION_CREATE_POST) {
		c.SetPermissionError(model.PERMISSION_CREATE_POST)
		return
	}

	if err := c.App.PublishUserTyping(c.Params.UserId, typingRequest.ChannelId, typingRequest.ParentId); err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func verifyUserEmailWithoutToken(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("verifyUserEmailWithoutToken", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("user_id", user.Id)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	if err := c.App.VerifyUserEmail(user.Id, user.Email); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("user verified")

	w.Write([]byte(user.ToJson()))
}

func convertUserToBot(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	user, err := c.App.GetUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("convertUserToBot", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("user", user)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	bot, err := c.App.ConvertUserToBot(user)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("convertedTo", bot)

	w.Write(bot.ToJson())
}

func getUploadsForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if c.Params.UserId != c.AppContext.Session().UserId {
		c.Err = model.NewAppError("getUploadsForUser", "api.user.get_uploads_for_user.forbidden.app_error", nil, "", http.StatusForbidden)
		return
	}

	uss, err := c.App.GetUploadSessionsForUser(c.Params.UserId)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.UploadSessionsToJson(uss)))
}

func migrateAuthToLDAP(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.StringInterfaceFromJson(r.Body)
	from, ok := props["from"].(string)
	if !ok {
		c.SetInvalidParam("from")
		return
	}
	if from == "" || (from != "email" && from != "gitlab" && from != "saml" && from != "google" && from != "office365") {
		c.SetInvalidParam("from")
		return
	}

	force, ok := props["force"].(bool)
	if !ok {
		c.SetInvalidParam("force")
		return
	}

	matchField, ok := props["match_field"].(string)
	if !ok {
		c.SetInvalidParam("match_field")
		return
	}

	auditRec := c.MakeAuditRecord("migrateAuthToLdap", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("from", from)
	auditRec.AddMeta("match_field", matchField)
	auditRec.AddMeta("force", force)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.LDAP {
		c.Err = model.NewAppError("api.migrateAuthToLDAP", "api.admin.ldap.not_available.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	// Email auth in Mattermost system is represented by ""
	if from == "email" {
		from = ""
	}

	if migrate := c.App.AccountMigration(); migrate != nil {
		if err := migrate.MigrateToLdap(from, matchField, force, false); err != nil {
			c.Err = model.NewAppError("api.migrateAuthToLdap", "api.migrate_to_saml.error", nil, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		c.Err = model.NewAppError("api.migrateAuthToLdap", "api.admin.ldap.not_available.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func migrateAuthToSaml(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.StringInterfaceFromJson(r.Body)
	from, ok := props["from"].(string)
	if !ok {
		c.SetInvalidParam("from")
		return
	}
	if from == "" || (from != "email" && from != "gitlab" && from != "ldap" && from != "google" && from != "office365") {
		c.SetInvalidParam("from")
		return
	}

	auto, ok := props["auto"].(bool)
	if !ok {
		c.SetInvalidParam("auto")
		return
	}
	matches, ok := props["matches"].(map[string]interface{})
	if !ok {
		c.SetInvalidParam("matches")
		return
	}
	usersMap := model.MapFromJson(strings.NewReader(model.StringInterfaceToJson(matches)))

	auditRec := c.MakeAuditRecord("migrateAuthToSaml", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("from", from)
	auditRec.AddMeta("matches", matches)
	auditRec.AddMeta("auto", auto)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.SAML {
		c.Err = model.NewAppError("api.migrateAuthToSaml", "api.admin.saml.not_available.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	// Email auth in Mattermost system is represented by ""
	if from == "email" {
		from = ""
	}

	if migrate := c.App.AccountMigration(); migrate != nil {
		if err := migrate.MigrateToSaml(from, usersMap, auto, false); err != nil {
			c.Err = model.NewAppError("api.migrateAuthToSaml", "api.migrate_to_saml.error", nil, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		c.Err = model.NewAppError("api.migrateAuthToSaml", "api.admin.saml.not_available.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func getThreadForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId().RequireTeamId().RequireThreadId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}
	extendedStr := r.URL.Query().Get("extended")
	extended, _ := strconv.ParseBool(extendedStr)

	threadMembership, err := c.App.GetThreadMembershipForUser(c.Params.UserId, c.Params.ThreadId)
	if err != nil {
		c.Err = err
		return
	}

	thread, err := c.App.GetThreadForUser(c.Params.TeamId, threadMembership, extended)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(thread.ToJson()))
}

func getThreadsForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId().RequireTeamId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	options := model.GetUserThreadsOpts{
		Since:    0,
		Before:   "",
		After:    "",
		PageSize: 30,
		Unread:   false,
		Extended: false,
		Deleted:  false,
	}

	sinceString := r.URL.Query().Get("since")
	if sinceString != "" {
		since, parseError := strconv.ParseUint(sinceString, 10, 64)
		if parseError != nil {
			c.SetInvalidParam("since")
			return
		}
		options.Since = since
	}

	options.Before = r.URL.Query().Get("before")
	options.After = r.URL.Query().Get("after")
	// parameters are mutually exclusive
	if options.Before != "" && options.After != "" {
		c.Err = model.NewAppError("api.getThreadsForUser", "api.getThreadsForUser.bad_params", nil, "", http.StatusBadRequest)
		return
	}
	pageSizeString := r.URL.Query().Get("pageSize")
	if pageSizeString != "" {
		pageSize, parseError := strconv.ParseUint(pageSizeString, 10, 64)
		if parseError != nil {
			c.SetInvalidParam("pageSize")
			return
		}
		options.PageSize = pageSize
	}

	deletedStr := r.URL.Query().Get("deleted")
	unreadStr := r.URL.Query().Get("unread")
	extendedStr := r.URL.Query().Get("extended")

	options.Deleted, _ = strconv.ParseBool(deletedStr)
	options.Unread, _ = strconv.ParseBool(unreadStr)
	options.Extended, _ = strconv.ParseBool(extendedStr)

	threads, err := c.App.GetThreadsForUser(c.Params.UserId, c.Params.TeamId, options)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(threads.ToJson()))
}

func updateReadStateThreadByUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId().RequireThreadId().RequireTimestamp().RequireTeamId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("updateReadStateThreadByUser", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("user_id", c.Params.UserId)
	auditRec.AddMeta("thread_id", c.Params.ThreadId)
	auditRec.AddMeta("team_id", c.Params.TeamId)
	auditRec.AddMeta("timestamp", c.Params.Timestamp)
	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	thread, err := c.App.UpdateThreadReadForUser(c.Params.UserId, c.Params.TeamId, c.Params.ThreadId, c.Params.Timestamp)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(thread.ToJson()))

	auditRec.Success()
}

func unfollowThreadByUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId().RequireThreadId().RequireTeamId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("unfollowThreadByUser", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("user_id", c.Params.UserId)
	auditRec.AddMeta("thread_id", c.Params.ThreadId)
	auditRec.AddMeta("team_id", c.Params.TeamId)

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	err := c.App.UpdateThreadFollowForUser(c.Params.UserId, c.Params.TeamId, c.Params.ThreadId, false)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)

	auditRec.Success()
}

func followThreadByUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId().RequireThreadId().RequireTeamId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("followThreadByUser", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("user_id", c.Params.UserId)
	auditRec.AddMeta("thread_id", c.Params.ThreadId)
	auditRec.AddMeta("team_id", c.Params.TeamId)

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	err := c.App.UpdateThreadFollowForUser(c.Params.UserId, c.Params.TeamId, c.Params.ThreadId, true)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
	auditRec.Success()
}

func updateReadStateAllThreadsByUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId().RequireTeamId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("updateReadStateAllThreadsByUser", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("user_id", c.Params.UserId)
	auditRec.AddMeta("team_id", c.Params.TeamId)

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	err := c.App.UpdateThreadsReadForUser(c.Params.UserId, c.Params.TeamId)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
	auditRec.Success()
}
