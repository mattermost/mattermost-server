// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/web"
)

const maxPropertyValuePatchItems = 50

// targetAccessPath names the rule that let hasTargetAccess grant access.
type targetAccessPath string

const (
	targetAccessSelf           targetAccessPath = "self"
	targetAccessLocal          targetAccessPath = "local"
	targetAccessPermission     targetAccessPath = "permission"
	targetAccessEditOtherUsers targetAccessPath = "edit_other_users"
	targetAccessBotManager     targetAccessPath = "bot_manager"
)

func (api *API) InitProperties() {
	if api.srv.Config().FeatureFlags.IntegratedBoards ||
		api.srv.Config().FeatureFlags.ManagedChannelCategories ||
		api.srv.Config().FeatureFlags.ClassificationMarkings ||
		api.srv.Config().FeatureFlags.SessionAttributes ||
		api.srv.Config().FeatureFlags.PostAttributes ||
		api.srv.Config().FeatureFlags.ChannelAttributes {
		api.BaseRoutes.PropertyFields.Handle("", api.APISessionRequired(getPropertyFields)).Methods(http.MethodGet)
		api.BaseRoutes.PropertyFieldsSearch.Handle("", api.APISessionRequired(searchPropertyFields)).Methods(http.MethodPost)
		api.BaseRoutes.GlobalAttributesSearch.Handle("", api.APISessionRequired(searchGlobalAttributeListing)).Methods(http.MethodPost)
		api.BaseRoutes.PropertyValues.Handle("", api.APISessionRequired(getPropertyValues)).Methods(http.MethodGet)
		api.BaseRoutes.PropertySystemValues.Handle("", api.APISessionRequired(getSystemPropertyValues)).Methods(http.MethodGet)

		api.BaseRoutes.PropertyFields.Handle("", api.APISessionRequired(createPropertyField)).Methods(http.MethodPost)
		api.BaseRoutes.PropertyField.Handle("", api.APISessionRequired(patchPropertyField)).Methods(http.MethodPatch)
		api.BaseRoutes.PropertyField.Handle("", api.APISessionRequired(deletePropertyField)).Methods(http.MethodDelete)

		api.BaseRoutes.PropertyFieldOptions.Handle("", api.APISessionRequired(getPropertyFieldOptions)).Methods(http.MethodGet)
		api.BaseRoutes.PropertyFieldOptions.Handle("", api.APISessionRequired(createPropertyFieldOptions)).Methods(http.MethodPost)
		api.BaseRoutes.PropertyFieldOptions.Handle("", api.APISessionRequired(patchPropertyFieldOptions)).Methods(http.MethodPatch)
		api.BaseRoutes.PropertyFieldOptions.Handle("", api.APISessionRequired(deletePropertyFieldOptions)).Methods(http.MethodDelete)

		api.BaseRoutes.PropertyValues.Handle("", api.APISessionRequired(patchPropertyValues)).Methods(http.MethodPatch)
		api.BaseRoutes.PropertySystemValues.Handle("", api.APISessionRequired(patchSystemPropertyValues)).Methods(http.MethodPatch)
	}

	// Required-channel-attribute compliance: reporting and remediation for the
	// Global Attributes "Required" toggle (MM-70717). Channel-attributes-only,
	// so gated on its own flag rather than the broader OR-of-six-flags block
	// above.
	if api.srv.Config().FeatureFlags.ChannelAttributes {
		api.BaseRoutes.PropertyFields.Handle("/missing_values/summary", api.APISessionRequired(getPropertyFieldsComplianceSummary)).Methods(http.MethodGet)
		api.BaseRoutes.PropertyField.Handle("/missing_values", api.APISessionRequired(getPropertyFieldMissingValues)).Methods(http.MethodGet)
		api.BaseRoutes.PropertyField.Handle("/missing_values/summary", api.APISessionRequired(getPropertyFieldComplianceSummary)).Methods(http.MethodGet)
		api.BaseRoutes.PropertyField.Handle("/missing_values/notify", api.APISessionRequired(notifyPropertyFieldMissingValues)).Methods(http.MethodPost)
	}
}

// getPropertyAPIGroup resolves c.Params.GroupName to a PSAv2/PSAv3 property group.
// On any error (not found, or not a v2/v3 group) it sets c.Err and returns nil.
func getPropertyAPIGroup(c *Context, callerName string) *model.PropertyGroup {
	group, appErr := c.App.GetPropertyGroup(c.AppContext, c.Params.GroupName)
	if appErr != nil {
		c.Err = appErr
		return nil
	}
	if !group.IsPSAv2() && !group.IsPSAv3() {
		c.Err = model.NewAppError(callerName, "api.property.v2_group_not_found.app_error", nil, "", http.StatusNotFound)
		return nil
	}
	// Session attributes require both the feature flag and Enterprise
	// Advanced. This mirrors the dedicated manifest endpoint's gate so the
	// generic Properties API cannot expose the schema when the feature is off.
	if group.Name == model.SessionAttributesPropertyGroupName &&
		(!c.App.Config().FeatureFlags.SessionAttributes || !model.MinimumEnterpriseAdvancedLicense(c.App.License())) {
		c.Err = model.NewAppError(callerName, "api.property.session_attributes.license.app_error", nil, "", http.StatusNotImplemented)
		return nil
	}
	return group
}

// requireChannelAttributeLicense gates channel-scoped objects in the
// access_control group. They are the storage behind channel attributes and
// classification's channel banner, which are Enterprise Advanced. The same
// group also holds user attributes, which are not, so the tier is checked per
// object type rather than on the group as a whole.
func requireChannelAttributeLicense(c *Context, group *model.PropertyGroup, callerName string, objectTypes ...string) bool {
	if group.Name != model.AccessControlPropertyGroupName {
		return true
	}

	if !slices.Contains(objectTypes, model.PropertyFieldObjectTypeChannel) {
		return true
	}

	if !model.MinimumEnterpriseAdvancedLicense(c.App.License()) {
		c.Err = model.NewAppError(callerName, "api.property.channel_attributes.license.app_error", nil, "", http.StatusNotImplemented)
		return false
	}

	return true
}

// requireChannelAttributeLicenseForTemplate refuses a request against an
// access_control template that a channel field links to, unless the license is
// Enterprise Advanced. requireChannelAttributeLicense cannot catch this case:
// it runs before any field is read and keys on the object type in the URL,
// which is "template" here. Sets c.Err and returns false on refusal.
//
// Best-effort: this catches a template a channel field links to directly.
// A linked field may not itself be a link target, so a chain cannot hide
// the template; dependents still come and go, so the answer is only as
// good as this moment.
func requireChannelAttributeLicenseForTemplate(c *Context, rctx request.CTX, group *model.PropertyGroup, field *model.PropertyField, callerName string) bool {
	if group.Name != model.AccessControlPropertyGroupName ||
		field.ObjectType != model.PropertyFieldObjectTypeTemplate ||
		model.MinimumEnterpriseAdvancedLicense(c.App.License()) {
		return true
	}

	dependents, searchErr := c.App.SearchPropertyFields(rctx, group.ID, model.PropertyFieldSearchOpts{
		ObjectTypes:   []string{model.PropertyFieldObjectTypeChannel},
		LinkedFieldID: field.ID,
		PerPage:       1,
	})
	if searchErr != nil {
		c.Err = searchErr
		return false
	}
	if len(dependents) > 0 {
		c.Err = model.NewAppError(callerName, "api.property.channel_attributes.license.app_error", nil, "", http.StatusNotImplemented)
		return false
	}

	return true
}

// resolvePropertyGroupParam reads the propertyGroup query parameter and resolves it to a
// property group ID. Returns "" when the parameter is absent, which callers pass straight through to
// PreparePostForClientOpts as "do not hydrate". On any validation failure it sets c.Err, so callers
// check that rather than the returned value.
func resolvePropertyGroupParam(c *Context, r *http.Request) string {
	raw := strings.TrimSpace(r.URL.Query().Get("propertyGroup"))
	if raw == "" {
		return ""
	}

	c.Params.GroupName = raw
	group := getPropertyAPIGroup(c, "resolvePropertyGroupParam")
	if group == nil {
		return ""
	}

	return group.ID
}

// servesV3 reports whether group's payload should carry the shaped
// Permissions object -- both the group's own version and the rollout flag
// have to agree, so a v3 group with the flag off still serves the v2 shape.
func servesV3(c *Context, group *model.PropertyGroup) bool {
	return group.IsPSAv3() && c.App.Config().FeatureFlags.PropertyFieldPermissionsV3
}

func createPropertyField(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType()
	if c.Err != nil {
		return
	}

	group := getPropertyAPIGroup(c, "createPropertyField")
	if c.Err != nil {
		return
	}

	if !requireChannelAttributeLicense(c, group, "createPropertyField", c.Params.ObjectType) {
		return
	}

	var field *model.PropertyField
	if err := json.NewDecoder(r.Body).Decode(&field); err != nil || field == nil {
		c.SetInvalidParamWithErr("property_field", err)
		return
	}

	field.ObjectType = c.Params.ObjectType
	field.GroupID = group.ID

	auditRec := c.MakeAuditRecord(model.AuditEventCreatePropertyField, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterAuditableToAuditRec(auditRec, "property_field", field)

	// There is no existing field to decide a basis against here -- creation
	// is its own question, not a field.write -- so only the caller identity
	// and whether it bypassed the decision are recorded.
	callerID := sessionCallerID(c)
	model.AddEventParameterToAuditRec(auditRec, "caller_id", callerID)
	if callerID == model.CallerIDLocalAdmin {
		model.AddEventParameterToAuditRec(auditRec, "basis_unrestricted", true)
	}

	rctx := app.RequestContextWithCallerID(c.AppContext, callerID)

	if field.Protected {
		c.Err = model.NewAppError("createPropertyField", "api.property_field.create.protected_via_api.app_error", nil, "", http.StatusBadRequest)
		return
	}

	// A group not serving the v3 payload has no place for a permissions
	// object in its shape, so accepting one would store something the
	// caller was never shown. Who may submit one is decided below, with the
	// permission levels it replaces.
	if field.Permissions != nil && !servesV3(c, group) {
		c.Err = model.NewAppError("createPropertyField", "api.property_field.patch.permissions_not_supported.app_error", nil, "", http.StatusBadRequest)
		return
	}

	// Pre-canonicalize system objects so the scope check below cannot be
	// bypassed by submitting ObjectType=system with TargetType=channel. The
	// App layer re-canonicalizes defensively for plugin/internal callers.
	app.CanonicalizeSystemObjectField(field)

	// Templates are always sysadmin-only, regardless of TargetType.
	if field.ObjectType == model.PropertyFieldObjectTypeTemplate &&
		!c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	// Scope-based create permission.
	switch field.TargetType {
	case "channel":
		if field.TargetID == "" {
			c.Err = model.NewAppError("createPropertyField", "api.property_field.create.target_id_required.app_error", nil, "", http.StatusBadRequest)
			return
		}
		hasPermission, _ := c.App.SessionHasPermissionToChannel(c.AppContext, *c.AppContext.Session(), field.TargetID, model.PermissionCreatePost)
		if !hasPermission {
			c.SetPermissionError(model.PermissionCreatePost)
			return
		}
	case "team":
		if field.TargetID == "" {
			c.Err = model.NewAppError("createPropertyField", "api.property_field.create.target_id_required.app_error", nil, "", http.StatusBadRequest)
			return
		}
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), field.TargetID, model.PermissionManageTeam) {
			c.SetPermissionError(model.PermissionManageTeam)
			return
		}
	case "system":
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
			c.SetPermissionError(model.PermissionManageSystem)
			return
		}
	default:
		c.Err = model.NewAppError("createPropertyField", "api.property_field.create.invalid_target_type.app_error", nil, "", http.StatusBadRequest)
		return
	}

	// The access_control group's hook pins field.write and option.write to
	// sysadmin on every create, but a submitted permissions object is stored
	// as sent and decides alone, so it would slip past that pin. Hold it to
	// the same bar here: only a system administrator may submit one. Plugins
	// reach this group through the plugin API, not this handler.
	if field.Permissions != nil && group.Name == model.AccessControlPropertyGroupName &&
		!c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.Err = model.NewAppError("createPropertyField", "api.property_field.create.permissions_require_system_admin.app_error", nil, "", http.StatusForbidden)
		return
	}

	// Set before the permission levels are resolved.
	field.CreatedBy = c.AppContext.Session().UserId
	field.UpdatedBy = c.AppContext.Session().UserId

	// Default permission levels: whoever administers the field's scope may
	// configure them, everyone else has all three pinned to the default.
	canPin := c.App.SessionHasPermissionToAdministerPropertyFieldScope(c.AppContext, *c.AppContext.Session(), field)
	defaultLevel := app.DefaultPropertyFieldPermissionLevel(field)
	defaultValuesLevel := app.DefaultPropertyFieldValuesPermissionLevel(field)
	if !canPin {
		// A permissions object is the same configuration the levels are, and
		// once stored it decides alone, so it takes the same authority. Refused
		// rather than replaced by the defaults, since the levels' silent pin
		// would leave the caller believing their object was applied. Without
		// this, anyone able to create a field could set field.write none with
		// a grant naming only themselves, and no administrator could then edit
		// or delete it.
		if field.Permissions != nil {
			c.Err = model.NewAppError("createPropertyField", "api.property_field.create.permissions_require_scope_admin.app_error", nil, "", http.StatusForbidden)
			return
		}
		field.PermissionField = &defaultLevel
		field.PermissionValues = &defaultValuesLevel
		field.PermissionOptions = &defaultLevel
	} else {
		if field.PermissionField == nil {
			field.PermissionField = &defaultLevel
		}
		if field.PermissionValues == nil {
			field.PermissionValues = &defaultValuesLevel
		}
		if field.PermissionOptions == nil {
			field.PermissionOptions = &defaultLevel
		}

		// Scope admin or not, a submitted object may not put the field
		// beyond every system admin's reach.
		if field.Permissions != nil && permissionsLockOutSystemAdmins(*c.AppContext.Session(), field.Permissions) {
			c.Err = model.NewAppError("createPropertyField", "api.property_field.permissions.field_write_none.app_error", nil, "", http.StatusBadRequest)
			return
		}

		// Anti-lockout: an unlinked field must be editable by whoever creates
		// it, or they could pin a level that leaves them unable to patch or
		// delete their own field.
		// The app layer converts the levels into Permissions only on create, and
		// the decision denies every human on a field without one, so judge the
		// object the field will be stored with.
		isLinked := field.LinkedFieldID != nil && *field.LinkedFieldID != ""
		asStored := *field
		if asStored.Permissions == nil {
			asStored.Permissions = model.PermissionsFromLegacy(field, model.LegacyConversionOpts{})
		}
		if !isLinked && !c.App.SessionHasPermissionToEditPropertyField(c.AppContext, *c.AppContext.Session(), &asStored) {
			c.Err = model.NewAppError("createPropertyField", "api.property_field.create.creator_cannot_edit.app_error", nil, "", http.StatusForbidden)
			return
		}
	}

	connectionID := r.Header.Get(model.ConnectionId)

	createdField, appErr := c.App.CreatePropertyField(rctx, field, false, connectionID)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	auditRec.AddEventResultState(createdField)
	auditRec.AddEventObjectType("property_field")

	shapedField := c.App.ShapePropertyFieldForCaller(rctx, *c.AppContext.Session(), createdField, servesV3(c, group))

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(shapedField); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getPropertyFields(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType()
	if c.Err != nil {
		return
	}

	group := getPropertyAPIGroup(c, "getPropertyFields")
	if c.Err != nil {
		return
	}

	opts := model.PropertyFieldSearchOpts{
		GroupID:     group.ID,
		ObjectTypes: []string{c.Params.ObjectType},
		PerPage:     c.Params.PerPage,
	}

	query := r.URL.Query()

	if s := query.Get("since"); s != "" {
		since, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			c.SetInvalidParamWithErr("since", err)
			return
		}
		opts.SinceUpdateAt = since
	}

	// Cursor: directory mode uses CreateAt, delta mode (since>0) uses
	// UpdateAt.
	if cursorID := query.Get("cursor_id"); cursorID != "" {
		cur := model.PropertyFieldSearchCursor{PropertyFieldID: cursorID}
		if v := query.Get("cursor_update_at"); v != "" {
			ua, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				c.SetInvalidParamWithErr("cursor_update_at", err)
				return
			}
			cur.UpdateAt = ua
		}
		if v := query.Get("cursor_create_at"); v != "" {
			ca, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				c.SetInvalidParamWithErr("cursor_create_at", err)
				return
			}
			cur.CreateAt = ca
		}
		if err := cur.IsValid(); err != nil {
			c.SetInvalidParamWithErr("cursor", err)
			return
		}
		opts.Cursor = cur
	}

	opts.ChannelID = query.Get("channel_id")
	opts.TeamID = query.Get("team_id")
	opts.TargetType = query.Get("target_type")
	if t := query.Get("target_id"); t != "" {
		opts.TargetIDs = []string{t}
	}

	searchPropertyFieldsCore(c, w, group, opts, "getPropertyFields")
}

func searchPropertyFields(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName()
	if c.Err != nil {
		return
	}

	group := getPropertyAPIGroup(c, "searchPropertyFields")
	if c.Err != nil {
		return
	}

	var search model.PropertyFieldSearch
	if err := json.NewDecoder(r.Body).Decode(&search); err != nil {
		c.SetInvalidParamWithErr("property_field_search", err)
		return
	}

	if len(search.ObjectTypes) == 0 {
		c.SetInvalidParam("object_types")
		return
	}
	for _, ot := range search.ObjectTypes {
		if !model.IsValidPropertyFieldObjectType(ot) {
			c.SetInvalidParam("object_types")
			return
		}
	}

	opts := model.PropertyFieldSearchOpts{
		GroupID:       group.ID,
		ObjectTypes:   search.ObjectTypes,
		ChannelID:     search.ChannelID,
		TeamID:        search.TeamID,
		TargetType:    search.TargetType,
		SinceUpdateAt: search.SinceUpdateAt,
		PerPage:       search.PerPage,
	}
	if search.TargetID != "" {
		opts.TargetIDs = []string{search.TargetID}
	}
	if search.CursorID != "" {
		opts.Cursor = model.PropertyFieldSearchCursor{
			PropertyFieldID: search.CursorID,
			CreateAt:        search.CursorCreateAt,
			UpdateAt:        search.CursorUpdateAt,
		}
	}
	if opts.PerPage <= 0 {
		opts.PerPage = web.PerPageDefault
	} else if opts.PerPage > web.PerPageMaximum {
		opts.PerPage = web.PerPageMaximum
	}

	searchPropertyFieldsCore(c, w, group, opts, "searchPropertyFields")
}

// searchPropertyFieldsCore is the shared pipeline both list-style endpoints
// share once opts has been populated from query string or request body:
// scope resolution + permission checks, opts validation, audit, search,
// response encoding.
func searchPropertyFieldsCore(c *Context, w http.ResponseWriter, group *model.PropertyGroup, opts model.PropertyFieldSearchOpts, callerName string) {
	auditRec := c.MakeAuditRecord(model.AuditEventGetPropertyFields, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "group_name", c.Params.GroupName)
	model.AddEventParameterToAuditRec(auditRec, "object_types", opts.ObjectTypes)
	model.AddEventParameterToAuditRec(auditRec, "since", opts.SinceUpdateAt)
	model.AddEventParameterToAuditRec(auditRec, "channel_id", opts.ChannelID)
	model.AddEventParameterToAuditRec(auditRec, "team_id", opts.TeamID)

	// System-object fields can only live at the system scope by
	// invariant (enforced at create time). When the caller asks
	// exclusively for system-object fields, any channel/team/target
	// filter is a semantic no-op — we collapse to target_type=system
	// regardless of what was passed so legacy callers don't get a
	// confusing scope_conflict on otherwise valid requests. The
	// shortcut only applies when object_types is exactly [system]:
	// mixing with other types would silently drop the non-system rows.
	if len(opts.ObjectTypes) == 1 && opts.ObjectTypes[0] == model.PropertyFieldObjectTypeSystem {
		opts.ChannelID = ""
		opts.TeamID = ""
		opts.TargetIDs = nil
		opts.TargetType = string(model.PropertyFieldTargetLevelSystem)
	}

	if !requireChannelAttributeLicense(c, group, callerName, opts.ObjectTypes...) {
		return
	}

	if !resolveScopeAndCheckPermissions(c, &opts, callerName) {
		return
	}

	if err := opts.IsValid(); err != nil {
		c.Err = model.NewAppError(callerName, "api.property_field.get.invalid_opts.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	fields, err := c.App.SearchPropertyFields(rctx, group.ID, opts)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	shapedFields := c.App.ShapePropertyFieldsForCaller(rctx, *c.AppContext.Session(), fields, servesV3(c, group))

	if err := json.NewEncoder(w).Encode(shapedFields); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// resolveScopeAndCheckPermissions enforces the two scope modes (hierarchical
// vs single-target) and runs the per-scope permission checks. It operates on
// already-populated opts.
func resolveScopeAndCheckPermissions(c *Context, opts *model.PropertyFieldSearchOpts, callerName string) bool {
	scopeByChanTeam := opts.ChannelID != "" || opts.TeamID != ""
	scopeByTarget := opts.TargetType != "" || len(opts.TargetIDs) > 0
	if scopeByChanTeam && scopeByTarget {
		c.Err = model.NewAppError(callerName, "api.property_field.get.scope_conflict.app_error", nil, "", http.StatusBadRequest)
		return false
	}

	switch {
	case opts.ChannelID != "":
		hasPermission, _ := c.App.SessionHasPermissionToChannel(c.AppContext, *c.AppContext.Session(), opts.ChannelID, model.PermissionReadChannel)
		if !hasPermission {
			c.SetPermissionError(model.PermissionReadChannel)
			return false
		}
		channel, appErr := c.App.GetChannel(c.AppContext, opts.ChannelID)
		if appErr != nil {
			c.Err = appErr
			return false
		}
		opts.ChannelID = channel.Id
		opts.TeamID = channel.TeamId
	case opts.TeamID != "":
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), opts.TeamID, model.PermissionViewTeam) {
			c.SetPermissionError(model.PermissionViewTeam)
			return false
		}
	case opts.TargetType != "":
		if !model.IsValidPSAv2PropertyFieldTargetType(opts.TargetType) {
			c.Err = model.NewAppError(callerName, "api.property_field.get.invalid_target_type.app_error", nil, "", http.StatusBadRequest)
			return false
		}
		switch model.PropertyFieldTargetLevel(opts.TargetType) {
		case model.PropertyFieldTargetLevelChannel:
			if len(opts.TargetIDs) == 0 {
				c.Err = model.NewAppError(callerName, "api.property_field.get.target_id_required.app_error", nil, "", http.StatusBadRequest)
				return false
			}
			hasPermission, _ := c.App.SessionHasPermissionToChannel(c.AppContext, *c.AppContext.Session(), opts.TargetIDs[0], model.PermissionReadChannel)
			if !hasPermission {
				c.SetPermissionError(model.PermissionReadChannel)
				return false
			}
		case model.PropertyFieldTargetLevelTeam:
			if len(opts.TargetIDs) == 0 {
				c.Err = model.NewAppError(callerName, "api.property_field.get.target_id_required.app_error", nil, "", http.StatusBadRequest)
				return false
			}
			if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), opts.TargetIDs[0], model.PermissionViewTeam) {
				c.SetPermissionError(model.PermissionViewTeam)
				return false
			}
		case model.PropertyFieldTargetLevelSystem:
			// System-level fields are visible to all authenticated users.
		}
	case len(opts.TargetIDs) > 0:
		// target_id without target_type is malformed.
		c.Err = model.NewAppError(callerName, "api.property_field.get.target_type_required.app_error", nil, "", http.StatusBadRequest)
		return false
	default:
		c.Err = model.NewAppError(callerName, "api.property_field.get.scope_required.app_error", nil, "", http.StatusBadRequest)
		return false
	}
	return true
}

func patchPropertyField(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType().RequireFieldId()
	if c.Err != nil {
		return
	}

	group := getPropertyAPIGroup(c, "patchPropertyField")
	if c.Err != nil {
		return
	}

	if !requireChannelAttributeLicense(c, group, "patchPropertyField", c.Params.ObjectType) {
		return
	}

	var patch *model.PropertyFieldPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil || patch == nil {
		c.SetInvalidParamWithErr("property_field_patch", err)
		return
	}

	if patch.Name != nil {
		*patch.Name = strings.TrimSpace(*patch.Name)
	}

	patch.TargetID = nil
	patch.TargetType = nil

	if err := patch.IsValid(); err != nil {
		if appErr, ok := errors.AsType[*model.AppError](err); ok {
			c.Err = appErr
		} else {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.invalid_patch.app_error", nil, "", http.StatusBadRequest)
		}
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventPatchPropertyField, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterAuditableToAuditRec(auditRec, "property_field_patch", patch)

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	existingField, appErr := c.App.GetPropertyField(rctx, group.ID, c.Params.FieldId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	// PSAv2 routes only operate on PSAv2 fields. Reject legacy fields.
	if existingField.IsPSAv1() {
		c.Err = model.NewAppError("patchPropertyField", "api.property_field.patch.legacy_field.app_error", nil, "", http.StatusBadRequest)
		return
	}

	// HTTP-routing: a 404 indistinguishable from "no such field" lets us
	// bucket fields by URL ObjectType without leaking cross-bucket existence.
	if existingField.ObjectType != c.Params.ObjectType {
		c.Err = model.NewAppError("patchPropertyField", "api.property_field.object_type_mismatch.app_error", nil, "", http.StatusNotFound)
		return
	}

	if !requireChannelAttributeLicenseForTemplate(c, rctx, group, existingField, "patchPropertyField") {
		return
	}

	// PermissionValues is only patchable on a linked field (any object type),
	// and only to Member or Sysadmin. Requiring LinkedFieldID keeps this off
	// standalone fields, which can have a weaker (Member-level) PermissionField
	// than a linked field's always-sysadmin one -- without this, a caller who
	// can already edit a weaker-gated field's definition could use this
	// capability to escalate its PermissionValues to sysadmin, or loosen
	// another field's to member.
	if patch.PermissionValues != nil {
		if existingField.LinkedFieldID == nil || *existingField.LinkedFieldID == "" {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.patch.permission_values_not_linked.app_error", nil, "", http.StatusBadRequest)
			return
		}
		if *patch.PermissionValues != model.PermissionLevelMember && *patch.PermissionValues != model.PermissionLevelSysadmin {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.patch.permission_values_invalid.app_error", nil, "", http.StatusBadRequest)
			return
		}
	}

	// Permissions rides its own patch type (PermissionsPatch) rather than
	// Patch's field-by-field copy, so it is resolved here -- before the
	// permission branching below, which needs to know whether it changed
	// anything. An absent "permissions" key and a top-level null both decode
	// to nil and leave the stored object alone, so the whole object cannot
	// be cleared. Inside the object, ApplyTo resolves restrictions, grants
	// and masking each on their own: absent keeps the stored value, null
	// clears it, and a value replaces it outright. The result is held back
	// and only assigned once the caller has been gated.
	patchedPermissions := existingField.Permissions
	permissionsChanged := false
	if patch.Permissions != nil {
		if !servesV3(c, group) {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.patch.permissions_not_supported.app_error", nil, "", http.StatusBadRequest)
			return
		}

		applied, err := applyPermissionsPatch(existingField, patch)
		if err != nil {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.patch.invalid_permissions.app_error", nil, err.Error(), http.StatusBadRequest)
			return
		}
		patchedPermissions = applied
		// Keyed on effect, not on the key being present. An inbound key is
		// applied only where it differs from what the caller was shown, so a
		// permissions object that round-trips unchanged is an echo and gates
		// nothing. This also keeps this decision in step with the
		// enforcement hook's, which sees only the merged field and so can only
		// judge by effect (model.PropertyFieldChangeIsOptionsOnly) -- if the two
		// disagree, one layer allows what the other refuses. Both compare with
		// model.PermissionsEqual, which reads null grants and the [] they are
		// served as the same.
		permissionsChanged = !model.PermissionsEqual(existingField.Permissions, applied)
	}

	// Mirrors permissionsChanged above: an options-only patch is gated on
	// option.write only when the submitted option list actually differs from
	// what is stored, not merely because it is the only attr present.
	optionsChanged := false
	if patch.Attrs != nil {
		if newOptions, ok := (*patch.Attrs)[model.PropertyFieldAttributeOptions]; ok {
			optionsChanged = !reflect.DeepEqual(existingField.Attrs[model.PropertyFieldAttributeOptions], newOptions)
		}
	}

	// Permission branching (session-bound): options-only patches use a
	// narrower permission than full edits.
	isOptionsOnly := isOptionsOnlyPatch(patch, permissionsChanged, optionsChanged)
	if isOptionsOnly && !existingField.Type.SupportsOptions() {
		isOptionsOnly = false
	}
	// basis_action distinguishes the two paths through this handler, since
	// otherwise they log near-identical meta.
	patchAction := model.PropertyActionFieldWrite
	var basis app.PropertyPermissionBasis
	if isOptionsOnly {
		patchAction = model.PropertyActionOptionWrite
		basis = c.App.SessionPropertyFieldOptionsBasis(rctx, *c.AppContext.Session(), existingField)
		if !basis.Allowed {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.update.no_options_permission.app_error", nil, "", http.StatusForbidden)
			return
		}
	} else {
		basis = c.App.SessionPropertyFieldEditBasis(rctx, *c.AppContext.Session(), existingField)
		if !basis.Allowed {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.update.no_field_permission.app_error", nil, "", http.StatusForbidden)
			return
		}
	}

	model.AddEventParameterToAuditRec(auditRec, "basis_action", patchAction)
	addPropertyPermissionBasisMeta(auditRec, basis)

	// Capture original state for audit before the in-place patch. Attrs is
	// shallow-copied because Patch mutates it.
	orig := *existingField
	if existingField.Attrs != nil {
		orig.Attrs = make(model.StringInterface, len(existingField.Attrs))
		maps.Copy(orig.Attrs, existingField.Attrs)
	}
	auditRec.AddEventPriorState(&orig)

	if patch.Permissions != nil {
		// A leaf a submitted restrictions object omits would be stored as
		// none, so a caller who sends only the leaves they mean to change
		// would silently revoke the rest, field.write included. The spec's
		// answer to a half-written block is a denial, not a half-updated
		// policy. Sending back the restrictions the server served is
		// unaffected, since every stored object carries all five leaves.
		if patch.Permissions.Restrictions != nil && patchedPermissions.Restrictions != nil &&
			!restrictionsSetEveryLeaf(patchedPermissions.Restrictions) {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.patch.incomplete_restrictions.app_error", nil, "", http.StatusBadRequest)
			return
		}

		// Keyed on effect, like the gating above: an unchanged echo of a
		// field a plugin keeps at field.write none takes nothing away, and
		// may ride along on an options patch the caller holds only
		// option.write for.
		if permissionsChanged && permissionsLockOutSystemAdmins(*c.AppContext.Session(), patchedPermissions) {
			c.Err = model.NewAppError("patchPropertyField", "api.property_field.permissions.field_write_none.app_error", nil, "", http.StatusBadRequest)
			return
		}
	}

	existingField.Permissions = patchedPermissions
	existingField.Patch(patch, true)

	// Anti-lockout, as on create: field.write was checked against the stored
	// field, so without this a caller could patch in permissions that leave
	// them unable to edit or delete the field they just changed. An unchanged
	// echo changes nothing and was gated as such, so it is not judged here.
	if permissionsChanged && !c.App.SessionHasPermissionToEditPropertyField(rctx, *c.AppContext.Session(), existingField) {
		c.Err = model.NewAppError("patchPropertyField", "api.property_field.patch.caller_cannot_edit.app_error", nil, "", http.StatusForbidden)
		return
	}

	existingField.UpdatedBy = c.AppContext.Session().UserId
	connectionID := r.Header.Get(model.ConnectionId)

	updatedField, _, updateErr := c.App.UpdatePropertyField(rctx, group.ID, existingField, false, connectionID)
	if updateErr != nil {
		c.Err = updateErr
		return
	}

	auditRec.Success()
	auditRec.AddEventResultState(updatedField)
	auditRec.AddEventObjectType("property_field")

	shapedField := c.App.ShapePropertyFieldForCaller(rctx, *c.AppContext.Session(), updatedField, servesV3(c, group))

	if err := json.NewEncoder(w).Encode(shapedField); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func deletePropertyField(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType().RequireFieldId()
	if c.Err != nil {
		return
	}

	group := getPropertyAPIGroup(c, "deletePropertyField")
	if c.Err != nil {
		return
	}

	if !requireChannelAttributeLicense(c, group, "deletePropertyField", c.Params.ObjectType) {
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventDeletePropertyField, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "field_id", c.Params.FieldId)

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	existingField, appErr := c.App.GetPropertyField(rctx, group.ID, c.Params.FieldId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if existingField.ObjectType != c.Params.ObjectType {
		c.Err = model.NewAppError("deletePropertyField", "api.property_field.object_type_mismatch.app_error", nil, "", http.StatusNotFound)
		return
	}

	if !requireChannelAttributeLicenseForTemplate(c, rctx, group, existingField, "deletePropertyField") {
		return
	}

	// Deleting a definition is a field.write -- there is no separate delete
	// cell in the permission grid.
	decideOn, _ := c.App.OrphanedPropertyFieldForDelete(existingField)
	basis := c.App.SessionPropertyFieldEditBasis(rctx, *c.AppContext.Session(), decideOn)
	if !basis.Allowed {
		c.Err = model.NewAppError("deletePropertyField", "api.property_field.delete.no_permission.app_error", nil, "", http.StatusForbidden)
		return
	}
	addPropertyPermissionBasisMeta(auditRec, basis)

	auditRec.AddEventPriorState(existingField)

	connectionID := r.Header.Get(model.ConnectionId)
	if deleteErr := c.App.DeletePropertyField(rctx, group.ID, c.Params.FieldId, false, connectionID); deleteErr != nil {
		c.Err = deleteErr
		return
	}

	auditRec.Success()
	auditRec.AddEventResultState(existingField)
	auditRec.AddEventObjectType("property_field")

	ReturnStatusOK(w)
}

func getPropertyValues(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType()
	if c.Err != nil {
		return
	}

	if c.Params.ObjectType == model.PropertyFieldObjectTypeTemplate {
		c.Err = model.NewAppError("getPropertyValues", "api.property_value.template_no_values.app_error", nil, "template fields cannot have values", http.StatusBadRequest)
		return
	}

	if c.Params.ObjectType == model.PropertyFieldObjectTypeSystem {
		c.Err = model.NewAppError("getPropertyValues", "api.property_value.system_use_dedicated_route.app_error", nil, "system values must use the dedicated system values endpoint", http.StatusBadRequest)
		return
	}

	c.RequireTargetId()
	if c.Err != nil {
		return
	}

	getPropertyValuesCore(c, w, r, c.Params.ObjectType, c.Params.TargetId)
}

func getSystemPropertyValues(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName()
	if c.Err != nil {
		return
	}

	getPropertyValuesCore(c, w, r, model.PropertyFieldObjectTypeSystem, model.PropertyValueSystemTargetID)
}

func getPropertyValuesCore(c *Context, w http.ResponseWriter, r *http.Request, objectType, targetID string) {
	group := getPropertyAPIGroup(c, "getPropertyValues")
	if c.Err != nil {
		return
	}

	if !requireChannelAttributeLicense(c, group, "getPropertyValues", objectType) {
		return
	}

	// Check target access based on object type
	if _, ok := hasTargetAccess(c, group.Name, objectType, targetID, false); !ok {
		return
	}

	query := r.URL.Query()

	opts := model.PropertyValueSearchOpts{
		TargetIDs:  []string{targetID},
		TargetType: objectType,
		PerPage:    c.Params.PerPage,
	}

	if s := query.Get("since"); s != "" {
		since, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			c.SetInvalidParamWithErr("since", err)
			return
		}
		opts.SinceUpdateAt = since
	}

	// Cursor: directory mode uses CreateAt, delta mode (since>0) uses
	// UpdateAt. opts.IsValid below rejects the mismatched combinations.
	if cursorID := query.Get("cursor_id"); cursorID != "" {
		cur := model.PropertyValueSearchCursor{PropertyValueID: cursorID}
		if v := query.Get("cursor_update_at"); v != "" {
			ua, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				c.SetInvalidParamWithErr("cursor_update_at", err)
				return
			}
			cur.UpdateAt = ua
		}
		if v := query.Get("cursor_create_at"); v != "" {
			ca, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				c.SetInvalidParamWithErr("cursor_create_at", err)
				return
			}
			cur.CreateAt = ca
		}
		opts.Cursor = cur
	}

	if err := opts.IsValid(); err != nil {
		c.Err = model.NewAppError("getPropertyValues", "api.property_value.get.invalid_opts.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventGetPropertyValues, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "group_name", c.Params.GroupName)
	model.AddEventParameterToAuditRec(auditRec, "object_type", objectType)
	model.AddEventParameterToAuditRec(auditRec, "target_id", targetID)
	model.AddEventParameterToAuditRec(auditRec, "since", opts.SinceUpdateAt)

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	values, err := c.App.SearchPropertyValues(rctx, group.ID, opts)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	if err := json.NewEncoder(w).Encode(values); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func patchPropertyValues(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType()
	if c.Err != nil {
		return
	}

	if c.Params.ObjectType == model.PropertyFieldObjectTypeTemplate {
		c.Err = model.NewAppError("patchPropertyValues", "api.property_value.template_no_values.app_error", nil, "template fields cannot have values", http.StatusBadRequest)
		return
	}

	if c.Params.ObjectType == model.PropertyFieldObjectTypeSystem {
		c.Err = model.NewAppError("patchPropertyValues", "api.property_value.system_use_dedicated_route.app_error", nil, "system values must use the dedicated system values endpoint", http.StatusBadRequest)
		return
	}

	c.RequireTargetId()
	if c.Err != nil {
		return
	}

	patchPropertyValuesCore(c, w, r, c.Params.ObjectType, c.Params.TargetId)
}

func patchSystemPropertyValues(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName()
	if c.Err != nil {
		return
	}

	patchPropertyValuesCore(c, w, r, model.PropertyFieldObjectTypeSystem, model.PropertyValueSystemTargetID)
}

func patchPropertyValuesCore(c *Context, w http.ResponseWriter, r *http.Request, objectType, targetID string) {
	group := getPropertyAPIGroup(c, "patchPropertyValues")
	if c.Err != nil {
		return
	}

	if !requireChannelAttributeLicense(c, group, "patchPropertyValues", objectType) {
		return
	}

	var items []model.PropertyValuePatchItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		c.SetInvalidParamWithErr("property_values", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventPatchPropertyValues, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "group_name", c.Params.GroupName)
	model.AddEventParameterToAuditRec(auditRec, "object_type", objectType)
	model.AddEventParameterToAuditRec(auditRec, "target_id", targetID)

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	path, ok := hasTargetAccess(c, group.Name, objectType, targetID, true)
	if !ok {
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "target_access", string(path))

	if len(items) == 0 {
		c.Err = model.NewAppError("patchPropertyValues", "api.property_value.patch.empty_body.app_error", nil, "", http.StatusBadRequest)
		return
	}
	if len(items) > maxPropertyValuePatchItems {
		c.Err = model.NewAppError("patchPropertyValues", "api.property_value.patch.too_many_items.request_error", map[string]any{
			"Max": maxPropertyValuePatchItems,
		}, "", http.StatusBadRequest)
		return
	}

	// Pre-validate IDs and de-dup so we can bulk-load fields for the
	// session-bound permission check below. The App layer re-validates these
	// invariants (defense for plugin/internal callers).
	seen := map[string]bool{}
	fieldIDs := make([]string, 0, len(items))
	for _, item := range items {
		if !model.IsValidId(item.FieldID) {
			c.Err = model.NewAppError("patchPropertyValues", "api.property_value.patch.invalid_field_id.app_error", nil, "", http.StatusBadRequest)
			return
		}
		if seen[item.FieldID] {
			c.Err = model.NewAppError("patchPropertyValues", "api.property_value.patch.duplicate_field_id.app_error", nil, "", http.StatusBadRequest)
			return
		}
		seen[item.FieldID] = true
		fieldIDs = append(fieldIDs, item.FieldID)
	}

	fields, fieldsErr := c.App.GetPropertyFields(rctx, group.ID, fieldIDs)
	if fieldsErr != nil {
		c.Err = fieldsErr
		return
	}
	fieldByID := make(map[string]*model.PropertyField, len(fields))
	for _, f := range fields {
		fieldByID[f.ID] = f
	}
	for _, item := range items {
		f, ok := fieldByID[item.FieldID]
		if !ok {
			c.Err = model.NewAppError("patchPropertyValues", "api.property_value.patch.field_not_found.app_error",
				map[string]any{"FieldID": item.FieldID}, "", http.StatusNotFound)
			return
		}
		if f.ObjectType != objectType {
			c.Err = model.NewAppError("patchPropertyValues", "api.property_field.object_type_mismatch.app_error", nil, "", http.StatusNotFound)
			return
		}
		if !c.App.SessionHasPermissionToSetPropertyFieldValues(rctx, *c.AppContext.Session(), f, targetID) {
			c.Err = model.NewAppError("patchPropertyValues", "api.property_value.patch.no_values_permission.app_error", nil, "", http.StatusForbidden)
			return
		}
	}

	userID := c.AppContext.Session().UserId
	values := make([]*model.PropertyValue, len(items))
	for i, item := range items {
		values[i] = &model.PropertyValue{
			TargetID:   targetID,
			TargetType: objectType,
			GroupID:    group.ID,
			FieldID:    item.FieldID,
			Value:      item.Value,
			CreatedBy:  userID,
			UpdatedBy:  userID,
		}
	}
	connectionID := r.Header.Get(model.ConnectionId)

	upserted, upsertErr := c.App.UpsertPropertyValues(rctx, values, objectType, targetID, connectionID)
	if upsertErr != nil {
		c.Err = upsertErr
		return
	}

	auditRec.Success()
	auditRec.AddEventObjectType("property_value")

	if err := json.NewEncoder(w).Encode(upserted); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// hasTargetAccess checks that the caller has access to the target entity
// identified by objectType and targetID. For reads (write=false) it checks
// read-level permissions; for writes it checks management-level permissions.
// It sets c.Err and returns false when access is denied. groupName is the
// resolved property group; it scopes the bot-manager write rule. The returned
// path names the rule that granted access, for auditing.
func hasTargetAccess(c *Context, groupName, objectType, targetID string, write bool) (targetAccessPath, bool) {
	switch objectType {
	case model.PropertyFieldObjectTypeChannel:
		if !write {
			hasPermission, _ := c.App.SessionHasPermissionToChannel(c.AppContext, *c.AppContext.Session(), targetID, model.PermissionReadChannel)
			if !hasPermission {
				c.SetPermissionError(model.PermissionReadChannel)
				return "", false
			}
		} else {
			channel, appErr := c.App.GetChannel(c.AppContext, targetID)
			if appErr != nil {
				c.Err = appErr
				return "", false
			}
			var perm *model.Permission
			switch channel.Type {
			case model.ChannelTypeOpen:
				perm = model.PermissionManagePublicChannelProperties
			case model.ChannelTypePrivate:
				perm = model.PermissionManagePrivateChannelProperties
			default:
				// DM/GM channels have no manage_*_channel_properties permission, so
				// this outer gate only checks membership, and the per-field tier check
				// in SessionHasPermissionToSetPropertyFieldValues resolves participation
				// as membership too. Groups that need DM/GM values kept out of users'
				// hands enforce that in their own PropertyHook — see
				// AccessControlAttributeValidationHook for the access_control group.
				perm = model.PermissionReadChannel
			}
			hasPermission, _ := c.App.SessionHasPermissionToChannel(c.AppContext, *c.AppContext.Session(), targetID, perm)
			if !hasPermission {
				c.SetPermissionError(perm)
				return "", false
			}
		}
	case model.PropertyFieldObjectTypePost:
		post, appErr := c.App.GetSinglePost(c.AppContext, targetID, false)
		if appErr != nil {
			c.Err = appErr
			return "", false
		}
		perm := model.PermissionReadChannel
		if write {
			perm = model.PermissionCreatePost
		}
		hasPermission, _ := c.App.SessionHasPermissionToChannel(c.AppContext, *c.AppContext.Session(), post.ChannelId, perm)
		if !hasPermission {
			c.SetPermissionError(perm)
			return "", false
		}
	case model.PropertyFieldObjectTypeUser:
		return userTargetAccess(c, groupName, targetID, write)
	case model.PropertyFieldObjectTypeSystem:
		// Any authenticated user can read system-scoped property values.
		// Only a system administrator can write them.
		if write && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
			c.SetPermissionError(model.PermissionManageSystem)
			return "", false
		}
	case model.PropertyFieldObjectTypeTemplate:
		// Templates don't have value targets — this should not be reached
		// if value endpoints properly reject template object type
		c.Err = model.NewAppError("hasTargetAccess", "api.property_value.template_no_values.app_error", nil, "template fields cannot have values", http.StatusBadRequest)
		return "", false
	default:
		c.Err = model.NewAppError("hasTargetAccess", "api.property_value.invalid_object_type.app_error", nil, "", http.StatusBadRequest)
		return "", false
	}
	return targetAccessPermission, true
}

// userTargetAccess applies the user-object rules of hasTargetAccess.
func userTargetAccess(c *Context, groupName, targetID string, write bool) (targetAccessPath, bool) {
	session := c.AppContext.Session()
	if targetID == session.UserId {
		return targetAccessSelf, true
	}
	if session.IsUnrestricted() {
		return targetAccessLocal, true
	}

	if !write {
		canSee, appErr := c.App.UserCanSeeOtherUser(c.AppContext, session.UserId, targetID)
		if appErr != nil {
			c.Err = appErr
			return "", false
		}
		if !canSee {
			c.SetPermissionError(model.PermissionViewMembers)
			return "", false
		}
		return targetAccessPermission, true
	}

	var path targetAccessPath
	switch {
	case c.App.SessionHasPermissionTo(*session, model.PermissionEditOtherUsers):
		path = targetAccessEditOtherUsers
	// Bot managers may write their bots' access_control values.
	case groupName == model.AccessControlPropertyGroupName && canManageBotTarget(c, targetID):
		path = targetAccessBotManager
	default:
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return "", false
	}

	// As in patchUser, only a system admin may modify a system admin, bots included.
	if !c.App.SessionHasPermissionTo(*session, model.PermissionManageSystem) {
		target, appErr := c.App.GetUser(c.AppContext, targetID)
		if appErr != nil {
			c.Err = appErr
			return "", false
		}
		if target.IsSystemAdmin() {
			c.SetPermissionError(model.PermissionManageSystem)
			return "", false
		}
	}
	return path, true
}

// canManageBotTarget reports whether the session can manage the bot with the
// given user ID. Any failure, including "not a bot" and "no such bot", is
// false so the caller can deny without revealing whether the bot exists.
func canManageBotTarget(c *Context, targetID string) bool {
	appErr := c.App.SessionHasPermissionToManageBot(c.AppContext, *c.AppContext.Session(), targetID)
	if appErr != nil && appErr.StatusCode >= http.StatusInternalServerError {
		c.Logger.Warn("Failed to resolve bot for target access", mlog.String("target_id", targetID), mlog.Err(appErr))
	}
	return appErr == nil
}

// sessionCallerID returns the caller ID to attach to a request-derived rctx
// for property-service hook identification. Local-mode (unrestricted)
// sessions have an empty Session.UserId but full admin privileges, so they
// are tagged with CallerIDLocalAdmin instead.
//
// EVERY handler that reads or writes property fields, options or values must
// pass an rctx built with this -- reads no less than writes. An access mode of
// shared_only masks a field's options and values against what the caller
// themselves holds, so a read that does not say who is asking is answered as a
// caller who holds nothing and is shown nothing. That fails closed, which is
// correct, but it is also the exact answer a legitimate caller who shares
// nothing with the target gets, so a handler that forgets looks like masking
// doing its job in the response.
//
// There is no structural guard. The access control hook logs a warning
// (properties.NoCallerIDWarning) when a call with no caller ID has a write
// refused or values hidden, but not when a field read only has its options
// hidden. The app package's tests fail on any such call; this package's tests
// check only when they opt in through observeUntaggedPropertyCalls. Reviewing
// a new property handler means checking this by eye.
func sessionCallerID(c *Context) string {
	session := c.AppContext.Session()
	if session.IsUnrestricted() {
		return model.CallerIDLocalAdmin
	}
	return session.UserId
}

// permissionsLockOutSystemAdmins reports whether permissions, submitted through
// the API, would leave field.write at none, the one tier a system
// administrator does not pass. Every other tier admits one, so holding a
// submitted object above none means no API caller can take a field away from
// the system admins, whatever grants it names. A field only a plugin or
// service may redefine is set up through the plugin API, which does not come
// through here; a field that does end up at none is recovered through the
// plugin API too. The local-mode exemption is defensive only, matching the
// field.write decision itself: these routes are registered for session
// callers alone, so local mode never reaches this check.
func permissionsLockOutSystemAdmins(session model.Session, permissions *model.Permissions) bool {
	if session.IsUnrestricted() || permissions == nil {
		return false
	}
	return permissions.Restrictions.TierFor(model.PropertyActionFieldWrite) == model.PermissionLevelNone
}

// restrictionsSetEveryLeaf reports whether r, as decoded from a request and
// before validation fills omitted leaves with none, sets all five enforced
// leaves.
func restrictionsSetEveryLeaf(r *model.Restrictions) bool {
	return r.Value.Read != "" && r.Value.Write != "" &&
		r.Option.Read != "" && r.Option.Write != "" &&
		r.Field.Write != ""
}

// addPropertyPermissionBasisMeta records which rule allowed a definition
// write, the same keys the value-write audit sink records for a value write,
// so one query over the audit log answers "what allowed this" for either.
// Empty fields are omitted so a record says only what applied.
func addPropertyPermissionBasisMeta(auditRec *model.AuditRecord, basis app.PropertyPermissionBasis) {
	if basis.Tier != "" {
		model.AddEventParameterToAuditRec(auditRec, "basis_tier", string(basis.Tier))
	}
	if basis.GrantID != "" {
		model.AddEventParameterToAuditRec(auditRec, "basis_grant_id", basis.GrantID)
	}
	if basis.GrantScope != "" {
		model.AddEventParameterToAuditRec(auditRec, "basis_grant_scope", basis.GrantScope)
	}
	if basis.GrantWildcard {
		model.AddEventParameterToAuditRec(auditRec, "basis_grant_wildcard", true)
	}
	if basis.Unrestricted {
		model.AddEventParameterToAuditRec(auditRec, "basis_unrestricted", true)
	}
}

// applyPermissionsPatch resolves patch.Permissions, which must be set,
// against existingField's stored object and returns the result; neither is
// modified.
//
// A linked field is served its template's masking as presence only
// (masking: {}) and may not declare masking of its own, so that is read back
// as unchanged: sending back the permissions object a linked field was served
// then succeeds. Not when the same patch unlinks the field, where masking: {}
// means what it says.
func applyPermissionsPatch(existingField *model.PropertyField, patch *model.PropertyFieldPatch) (*model.Permissions, error) {
	permissionsPatch := patch.Permissions
	if existingField.LinkSourceID() != "" && patch.LinkedFieldID == nil && permissionsPatch.MaskingIsPresenceOnly() {
		withoutMasking := *permissionsPatch
		withoutMasking.Masking = nil
		permissionsPatch = &withoutMasking
	}
	return permissionsPatch.ApplyTo(existingField.Permissions)
}

// permissionsChanged and optionsChanged say whether the patch's permissions
// object and option list, once applied, actually differ from what is stored.
// Both must be keyed on effect rather than the key's presence: a permissions
// or options value that round-trips unchanged decides nothing and is judged
// like any other unchanged key, and keying on presence instead would disagree
// with the enforcement hook, which only ever sees the merged field and so can
// only judge by effect (model.PropertyFieldChangeIsOptionsOnly).
func isOptionsOnlyPatch(patch *model.PropertyFieldPatch, permissionsChanged, optionsChanged bool) bool {
	// If any field property (besides attrs) is being updated, it's not options-only.
	// PermissionValues in particular must never ride through on the weaker
	// options-permission check (SessionHasPermissionToManagePropertyFieldOptions,
	// keyed on PermissionOptions) -- it requires the full-edit permission tier.
	if patch.Name != nil || patch.Type != nil || patch.TargetID != nil || patch.TargetType != nil || patch.LinkedFieldID != nil || patch.PermissionValues != nil || permissionsChanged {
		return false
	}

	// If attrs is not being updated at all, it's not an options update
	if patch.Attrs == nil {
		return false
	}

	// Check if attrs only contains "options" key
	attrs := *patch.Attrs
	if len(attrs) == 0 {
		return false
	}

	// If attrs has only the "options" key, and the option list actually
	// changed, it's an options-only update. An unchanged options echo decides
	// nothing, same as an unchanged permissions echo above -- without
	// optionsChanged here, a resubmission of the stored option list under the
	// "options" key alone would be classified option-only by presence while
	// the hook's field comparison finds nothing changed and falls back to
	// field.write, and the two layers would disagree.
	_, hasOptions := attrs[model.PropertyFieldAttributeOptions]
	return len(attrs) == 1 && hasOptions && optionsChanged
}
