// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type Context struct {
	App           app.AppIface
	AppContext    *request.Context
	Logger        *mlog.Logger
	Params        *Params
	Err           *model.AppError
	siteURLHeader string
}

// LogAuditRec logs an audit record using default LevelAPI.
func (c *Context) LogAuditRec(rec *audit.Record) {
	c.LogAuditRecWithLevel(rec, app.LevelAPI)
}

// LogAuditRec logs an audit record using specified Level.
// If the context is flagged with a permissions error then `level`
// is ignored and the audit record is emitted with `LevelPerms`.
func (c *Context) LogAuditRecWithLevel(rec *audit.Record, level mlog.LogLevel) {
	if rec == nil {
		return
	}
	if c.Err != nil {
		rec.AddMeta("err", c.Err.ID)
		rec.AddMeta("code", c.Err.StatusCode)
		if c.Err.ID == "api.context.permissions.app_error" {
			level = app.LevelPerms
		}
		rec.Fail()
	}
	c.App.Srv().Audit.LogRecord(level, *rec)
}

// MakeAuditRecord creates a audit record pre-populated with data from this context.
func (c *Context) MakeAuditRecord(event string, initialStatus string) *audit.Record {
	rec := &audit.Record{
		APIPath:   c.AppContext.Path(),
		Event:     event,
		Status:    initialStatus,
		UserID:    c.AppContext.Session().UserID,
		SessionID: c.AppContext.Session().ID,
		Client:    c.AppContext.UserAgent(),
		IPAddress: c.AppContext.IDAddress(),
		Meta:      audit.Meta{audit.KeyClusterID: c.App.GetClusterID()},
	}
	rec.AddMetaTypeConverter(model.AuditModelTypeConv)

	return rec
}

func (c *Context) LogAudit(extraInfo string) {
	audit := &model.Audit{UserID: c.AppContext.Session().UserID, IDAddress: c.AppContext.IDAddress(), Action: c.AppContext.Path(), ExtraInfo: extraInfo, SessionID: c.AppContext.Session().ID}
	if err := c.App.Srv().Store.Audit().Save(audit); err != nil {
		appErr := model.NewAppError("LogAudit", "app.audit.save.saving.app_error", nil, err.Error(), http.StatusInternalServerError)
		c.LogErrorByCode(appErr)
	}
}

func (c *Context) LogAuditWithUserID(userID, extraInfo string) {
	if c.AppContext.Session().UserID != "" {
		extraInfo = strings.TrimSpace(extraInfo + " session_user=" + c.AppContext.Session().UserID)
	}

	audit := &model.Audit{UserID: userID, IDAddress: c.AppContext.IDAddress(), Action: c.AppContext.Path(), ExtraInfo: extraInfo, SessionID: c.AppContext.Session().ID}
	if err := c.App.Srv().Store.Audit().Save(audit); err != nil {
		appErr := model.NewAppError("LogAuditWithUserId", "app.audit.save.saving.app_error", nil, err.Error(), http.StatusInternalServerError)
		c.LogErrorByCode(appErr)
	}
}

func (c *Context) LogErrorByCode(err *model.AppError) {
	code := err.StatusCode
	msg := err.SystemMessage(i18n.TDefault)
	fields := []mlog.Field{
		mlog.String("err_where", err.Where),
		mlog.Int("http_code", err.StatusCode),
		mlog.String("err_details", err.DetailedError),
	}
	switch {
	case (code >= http.StatusBadRequest && code < http.StatusInternalServerError) ||
		err.ID == "web.check_browser_compatibility.app_error":
		c.Logger.Debug(msg, fields...)
	case code == http.StatusNotImplemented:
		c.Logger.Info(msg, fields...)
	default:
		c.Logger.Error(msg, fields...)
	}
}

func (c *Context) IsSystemAdmin() bool {
	return c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem)
}

func (c *Context) SessionRequired() {
	if !*c.App.Config().ServiceSettings.EnableUserAccessTokens &&
		c.AppContext.Session().Props[model.SessionPropType] == model.SessionTypeUserAccessToken &&
		c.AppContext.Session().Props[model.SessionPropIsBot] != model.SessionPropIsBotValue {

		c.Err = model.NewAppError("", "api.context.session_expired.app_error", nil, "UserAccessToken", http.StatusUnauthorized)
		return
	}

	if c.AppContext.Session().UserID == "" {
		c.Err = model.NewAppError("", "api.context.session_expired.app_error", nil, "UserRequired", http.StatusUnauthorized)
		return
	}
}

func (c *Context) CloudKeyRequired() {
	if license := c.App.Srv().License(); license == nil || !*license.Features.Cloud || c.AppContext.Session().Props[model.SessionPropType] != model.SessionTypeCloudKey {
		c.Err = model.NewAppError("", "api.context.session_expired.app_error", nil, "TokenRequired", http.StatusUnauthorized)
		return
	}
}

func (c *Context) RemoteClusterTokenRequired() {
	if license := c.App.Srv().License(); license == nil || !*license.Features.RemoteClusterService || c.AppContext.Session().Props[model.SessionPropType] != model.SessionTypeRemoteclusterToken {
		c.Err = model.NewAppError("", "api.context.session_expired.app_error", nil, "TokenRequired", http.StatusUnauthorized)
		return
	}
}

func (c *Context) MfaRequired() {
	// Must be licensed for MFA and have it configured for enforcement
	if license := c.App.Srv().License(); license == nil || !*license.Features.MFA || !*c.App.Config().ServiceSettings.EnableMultifactorAuthentication || !*c.App.Config().ServiceSettings.EnforceMultifactorAuthentication {
		return
	}

	// OAuth integrations are excepted
	if c.AppContext.Session().IsOAuth {
		return
	}

	user, err := c.App.GetUser(c.AppContext.Session().UserID)
	if err != nil {
		c.Err = model.NewAppError("MfaRequired", "api.context.get_user.app_error", nil, err.Error(), http.StatusUnauthorized)
		return
	}

	if user.IsGuest() && !*c.App.Config().GuestAccountsSettings.EnforceMultifactorAuthentication {
		return
	}
	// Only required for email and ldap accounts
	if user.AuthService != "" &&
		user.AuthService != model.UserAuthServiceEmail &&
		user.AuthService != model.UserAuthServiceLdap {
		return
	}

	// Special case to let user get themself
	subpath, _ := utils.GetSubpathFromConfig(c.App.Config())
	if c.AppContext.Path() == path.Join(subpath, "/api/v4/users/me") {
		return
	}

	// Bots are exempt
	if user.IsBot {
		return
	}

	if !user.MfaActive {
		c.Err = model.NewAppError("MfaRequired", "api.context.mfa_required.app_error", nil, "", http.StatusForbidden)
		return
	}
}

// ExtendSessionExpiryIfNeeded will update Session.ExpiresAt based on session lengths in config.
// Session cookies will be resent to the client with updated max age.
func (c *Context) ExtendSessionExpiryIfNeeded(w http.ResponseWriter, r *http.Request) {
	if ok := c.App.ExtendSessionExpiryIfNeeded(c.AppContext.Session()); ok {
		c.App.AttachSessionCookies(c.AppContext, w, r)
	}
}

func (c *Context) RemoveSessionCookie(w http.ResponseWriter, r *http.Request) {
	subpath, _ := utils.GetSubpathFromConfig(c.App.Config())

	cookie := &http.Cookie{
		Name:     model.SessionCookieToken,
		Value:    "",
		Path:     subpath,
		MaxAge:   -1,
		HttpOnly: true,
	}

	http.SetCookie(w, cookie)
}

func (c *Context) SetInvalidParam(parameter string) {
	c.Err = NewInvalidParamError(parameter)
}

func (c *Context) SetInvalidUrlParam(parameter string) {
	c.Err = NewInvalidUrlParamError(parameter)
}

func (c *Context) SetServerBusyError() {
	c.Err = NewServerBusyError()
}

func (c *Context) SetInvalidRemoteIDError(id string) {
	c.Err = NewInvalidRemoteIDError(id)
}

func (c *Context) SetInvalidRemoteClusterTokenError() {
	c.Err = NewInvalidRemoteClusterTokenError()
}

func (c *Context) SetJSONEncodingError() {
	c.Err = NewJSONEncodingError()
}

func (c *Context) SetCommandNotFoundError() {
	c.Err = model.NewAppError("GetCommand", "store.sql_command.save.get.app_error", nil, "", http.StatusNotFound)
}

func (c *Context) HandleEtag(etag string, routeName string, w http.ResponseWriter, r *http.Request) bool {
	metrics := c.App.Metrics()
	if et := r.Header.Get(model.HeaderEtagClient); etag != "" {
		if et == etag {
			w.Header().Set(model.HeaderEtagServer, etag)
			w.WriteHeader(http.StatusNotModified)
			if metrics != nil {
				metrics.IncrementEtagHitCounter(routeName)
			}
			return true
		}
	}

	if metrics != nil {
		metrics.IncrementEtagMissCounter(routeName)
	}

	return false
}

func NewInvalidParamError(parameter string) *model.AppError {
	err := model.NewAppError("Context", "api.context.invalid_body_param.app_error", map[string]interface{}{"Name": parameter}, "", http.StatusBadRequest)
	return err
}
func NewInvalidUrlParamError(parameter string) *model.AppError {
	err := model.NewAppError("Context", "api.context.invalid_url_param.app_error", map[string]interface{}{"Name": parameter}, "", http.StatusBadRequest)
	return err
}
func NewServerBusyError() *model.AppError {
	err := model.NewAppError("Context", "api.context.server_busy.app_error", nil, "", http.StatusServiceUnavailable)
	return err
}

func NewInvalidRemoteIDError(parameter string) *model.AppError {
	err := model.NewAppError("Context", "api.context.remote_id_invalid.app_error", map[string]interface{}{"RemoteId": parameter}, "", http.StatusBadRequest)
	return err
}

func NewInvalidRemoteClusterTokenError() *model.AppError {
	err := model.NewAppError("Context", "api.context.remote_id_invalid.app_error", nil, "", http.StatusUnauthorized)
	return err
}

func NewJSONEncodingError() *model.AppError {
	err := model.NewAppError("Context", "api.context.json_encoding.app_error", nil, "", http.StatusInternalServerError)
	return err
}

func (c *Context) SetPermissionError(permissions ...*model.Permission) {
	c.Err = c.App.MakePermissionError(c.AppContext.Session(), permissions)
}

func (c *Context) SetSiteURLHeader(url string) {
	c.siteURLHeader = strings.TrimRight(url, "/")
}

func (c *Context) GetSiteURLHeader() string {
	return c.siteURLHeader
}

func (c *Context) RequireUserID() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.UserID == model.Me {
		c.Params.UserID = c.AppContext.Session().UserID
	}

	if !model.IsValidID(c.Params.UserID) {
		c.SetInvalidUrlParam("user_id")
	}
	return c
}

func (c *Context) RequireTeamID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.TeamID) {
		c.SetInvalidUrlParam("team_id")
	}
	return c
}

func (c *Context) RequireCategoryID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidCategoryID(c.Params.CategoryID) {
		c.SetInvalidUrlParam("category_id")
	}
	return c
}

func (c *Context) RequireInviteID() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.InviteID == "" {
		c.SetInvalidUrlParam("invite_id")
	}
	return c
}

func (c *Context) RequireTokenID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.TokenID) {
		c.SetInvalidUrlParam("token_id")
	}
	return c
}

func (c *Context) RequireThreadID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.ThreadID) {
		c.SetInvalidUrlParam("thread_id")
	}
	return c
}

func (c *Context) RequireTimestamp() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.Timestamp == 0 {
		c.SetInvalidUrlParam("timestamp")
	}
	return c
}

func (c *Context) RequireChannelID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.ChannelID) {
		c.SetInvalidUrlParam("channel_id")
	}
	return c
}

func (c *Context) RequireUsername() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidUsername(c.Params.Username) {
		c.SetInvalidParam("username")
	}

	return c
}

func (c *Context) RequirePostID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.PostID) {
		c.SetInvalidUrlParam("post_id")
	}
	return c
}

func (c *Context) RequirePolicyID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.PolicyID) {
		c.SetInvalidUrlParam("policy_id")
	}
	return c
}

func (c *Context) RequireAppID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.AppID) {
		c.SetInvalidUrlParam("app_id")
	}
	return c
}

func (c *Context) RequireFileID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.FileID) {
		c.SetInvalidUrlParam("file_id")
	}

	return c
}

func (c *Context) RequireUploadID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.UploadID) {
		c.SetInvalidUrlParam("upload_id")
	}

	return c
}

func (c *Context) RequireFilename() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.Filename == "" {
		c.SetInvalidUrlParam("filename")
	}

	return c
}

func (c *Context) RequirePluginID() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.PluginID == "" {
		c.SetInvalidUrlParam("plugin_id")
	}

	return c
}

func (c *Context) RequireReportID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.ReportID) {
		c.SetInvalidUrlParam("report_id")
	}
	return c
}

func (c *Context) RequireEmojiID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.EmojiID) {
		c.SetInvalidUrlParam("emoji_id")
	}
	return c
}

func (c *Context) RequireTeamName() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidTeamName(c.Params.TeamName) {
		c.SetInvalidUrlParam("team_name")
	}

	return c
}

func (c *Context) RequireChannelName() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidChannelIdentifier(c.Params.ChannelName) {
		c.SetInvalidUrlParam("channel_name")
	}

	return c
}

func (c *Context) SanitizeEmail() *Context {
	if c.Err != nil {
		return c
	}
	c.Params.Email = strings.ToLower(c.Params.Email)
	if !model.IsValidEmail(c.Params.Email) {
		c.SetInvalidUrlParam("email")
	}

	return c
}

func (c *Context) RequireCategory() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidAlphaNumHyphenUnderscore(c.Params.Category, true) {
		c.SetInvalidUrlParam("category")
	}

	return c
}

func (c *Context) RequireService() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.Service == "" {
		c.SetInvalidUrlParam("service")
	}

	return c
}

func (c *Context) RequirePreferenceName() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidAlphaNumHyphenUnderscore(c.Params.PreferenceName, true) {
		c.SetInvalidUrlParam("preference_name")
	}

	return c
}

func (c *Context) RequireEmojiName() *Context {
	if c.Err != nil {
		return c
	}

	validName := regexp.MustCompile(`^[a-zA-Z0-9\-\+_]+$`)

	if c.Params.EmojiName == "" || len(c.Params.EmojiName) > model.EmojiNameMaxLength || !validName.MatchString(c.Params.EmojiName) {
		c.SetInvalidUrlParam("emoji_name")
	}

	return c
}

func (c *Context) RequireHookID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.HookID) {
		c.SetInvalidUrlParam("hook_id")
	}

	return c
}

func (c *Context) RequireCommandID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.CommandID) {
		c.SetInvalidUrlParam("command_id")
	}
	return c
}

func (c *Context) RequireJobID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.JobID) {
		c.SetInvalidUrlParam("job_id")
	}
	return c
}

func (c *Context) RequireJobType() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.JobType == "" || len(c.Params.JobType) > 32 {
		c.SetInvalidUrlParam("job_type")
	}
	return c
}

func (c *Context) RequireRoleID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.RoleID) {
		c.SetInvalidUrlParam("role_id")
	}
	return c
}

func (c *Context) RequireSchemeID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.SchemeID) {
		c.SetInvalidUrlParam("scheme_id")
	}
	return c
}

func (c *Context) RequireRoleName() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidRoleName(c.Params.RoleName) {
		c.SetInvalidUrlParam("role_name")
	}

	return c
}

func (c *Context) RequireGroupID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.GroupID) {
		c.SetInvalidUrlParam("group_id")
	}
	return c
}

func (c *Context) RequireRemoteID() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.RemoteID == "" {
		c.SetInvalidUrlParam("remote_id")
	}
	return c
}

func (c *Context) RequireSyncableID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.SyncableID) {
		c.SetInvalidUrlParam("syncable_id")
	}
	return c
}

func (c *Context) RequireSyncableType() *Context {
	if c.Err != nil {
		return c
	}

	if c.Params.SyncableType != model.GroupSyncableTypeTeam && c.Params.SyncableType != model.GroupSyncableTypeChannel {
		c.SetInvalidUrlParam("syncable_type")
	}
	return c
}

func (c *Context) RequireBotUserID() *Context {
	if c.Err != nil {
		return c
	}

	if !model.IsValidID(c.Params.BotUserID) {
		c.SetInvalidUrlParam("bot_user_id")
	}
	return c
}

func (c *Context) RequireInvoiceID() *Context {
	if c.Err != nil {
		return c
	}

	if len(c.Params.InvoiceID) != 27 {
		c.SetInvalidUrlParam("invoice_id")
	}

	return c
}

func (c *Context) GetRemoteID(r *http.Request) string {
	return r.Header.Get(model.HeaderRemoteclusterID)
}
