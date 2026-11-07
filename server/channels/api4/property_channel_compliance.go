// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// resolveChannelAttributeComplianceGroup validates the URL's group/object_type
// against the access_control + channel bucket these endpoints are scoped to,
// mirroring patchPropertyField's preamble order. On any failure it sets c.Err
// and returns nil.
func resolveChannelAttributeComplianceGroup(c *Context, callerName string) *model.PropertyGroup {
	group := getPropertyAPIGroup(c, callerName)
	if c.Err != nil {
		return nil
	}

	if group.Name != model.AccessControlPropertyGroupName {
		c.Err = model.NewAppError(callerName, "api.property_field.object_type_mismatch.app_error", nil, "", http.StatusNotFound)
		return nil
	}

	if !requireChannelAttributeLicense(c, group, callerName, c.Params.ObjectType) {
		return nil
	}

	if c.Params.ObjectType != model.PropertyFieldObjectTypeChannel {
		c.Err = model.NewAppError(callerName, "api.property_field.object_type_mismatch.app_error", nil, "", http.StatusNotFound)
		return nil
	}

	return group
}

// resolveChannelAttributeComplianceField additionally loads and validates the
// field for the field-scoped routes, then checks edit permission on it. This
// payload -- every channel on the server plus its admins -- is strictly more
// sensitive than the field definition itself, so it uses the same permission
// bar as editing the field, not the weaker options-only check.
func resolveChannelAttributeComplianceField(c *Context, callerName string, group *model.PropertyGroup) *model.PropertyField {
	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	field, appErr := c.App.GetPropertyField(rctx, group.ID, c.Params.FieldId)
	if appErr != nil {
		c.Err = appErr
		return nil
	}

	if field.IsPSAv1() {
		c.Err = model.NewAppError(callerName, "api.property_field.missing_values.legacy_field.app_error", nil, "", http.StatusBadRequest)
		return nil
	}

	if field.ObjectType != c.Params.ObjectType {
		c.Err = model.NewAppError(callerName, "api.property_field.object_type_mismatch.app_error", nil, "", http.StatusNotFound)
		return nil
	}

	if !c.App.SessionHasPermissionToEditPropertyField(rctx, *c.AppContext.Session(), field) {
		c.Err = model.NewAppError(callerName, "api.property_field.missing_values.no_permission.app_error", nil, "", http.StatusForbidden)
		return nil
	}

	return field
}

func getPropertyFieldMissingValues(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType().RequireFieldId()
	if c.Err != nil {
		return
	}

	// Audited as early as the route params allow -- this endpoint's payload
	// (every channel on the server plus its admins) is PII-bearing, so even a
	// denied attempt against a bogus group/object_type/field_id should leave
	// a trail, not just a denial past the group/license/field checks below.
	auditRec := c.MakeAuditRecord(model.AuditEventGetPropertyFieldMissingValues, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "group_name", c.Params.GroupName)
	model.AddEventParameterToAuditRec(auditRec, "field_id", c.Params.FieldId)

	group := resolveChannelAttributeComplianceGroup(c, "getPropertyFieldMissingValues")
	if c.Err != nil {
		return
	}

	field := resolveChannelAttributeComplianceField(c, "getPropertyFieldMissingValues", group)
	if c.Err != nil {
		return
	}

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	list, appErr := c.App.GetChannelsMissingAttributeValue(rctx, group.ID, field, c.Params.Page, c.Params.PerPage)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()

	if err := writeChannelAttributeComplianceJSON(w, list); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getPropertyFieldComplianceSummary(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType().RequireFieldId()
	if c.Err != nil {
		return
	}

	// Audited as early as the route params allow -- see the identical
	// comment in getPropertyFieldMissingValues.
	auditRec := c.MakeAuditRecord(model.AuditEventGetPropertyFieldComplianceSummary, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "group_name", c.Params.GroupName)
	model.AddEventParameterToAuditRec(auditRec, "field_id", c.Params.FieldId)

	group := resolveChannelAttributeComplianceGroup(c, "getPropertyFieldComplianceSummary")
	if c.Err != nil {
		return
	}

	field := resolveChannelAttributeComplianceField(c, "getPropertyFieldComplianceSummary", group)
	if c.Err != nil {
		return
	}

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	summary, appErr := c.App.GetChannelAttributeComplianceSummary(rctx, group.ID, field, channelAttributeViewerLocale(c, rctx))
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()

	if err := writeChannelAttributeComplianceJSON(w, summary); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// getPropertyFieldsComplianceSummary is the create-mode form of
// getPropertyFieldComplianceSummary.
func getPropertyFieldsComplianceSummary(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventGetPropertyFieldsComplianceSummary, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "group_name", c.Params.GroupName)

	group := resolveChannelAttributeComplianceGroup(c, "getPropertyFieldsComplianceSummary")
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	summary, appErr := c.App.GetChannelAttributeComplianceSummary(rctx, group.ID, nil, channelAttributeViewerLocale(c, rctx))
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()

	if err := writeChannelAttributeComplianceJSON(w, summary); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func notifyPropertyFieldMissingValues(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupName().RequireObjectType().RequireFieldId()
	if c.Err != nil {
		return
	}

	// Audited as early as the route params allow -- see the identical
	// comment in getPropertyFieldMissingValues.
	auditRec := c.MakeAuditRecord(model.AuditEventNotifyPropertyFieldMissingValues, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "group_name", c.Params.GroupName)
	model.AddEventParameterToAuditRec(auditRec, "field_id", c.Params.FieldId)

	group := resolveChannelAttributeComplianceGroup(c, "notifyPropertyFieldMissingValues")
	if c.Err != nil {
		return
	}

	field := resolveChannelAttributeComplianceField(c, "notifyPropertyFieldMissingValues", group)
	if c.Err != nil {
		return
	}

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))

	result, appErr := c.App.NotifyChannelAdminsOfMissingAttribute(rctx, group.ID, field)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()

	w.WriteHeader(http.StatusAccepted)
	if err := writeChannelAttributeComplianceJSON(w, result); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// channelAttributeViewerLocale resolves the requesting sysadmin's own locale
// for the notify-message preview. Falls back to the server default on lookup
// failure -- the preview is advisory, never worth failing the request over.
func channelAttributeViewerLocale(c *Context, rctx request.CTX) string {
	user, appErr := c.App.GetUser(rctx, c.AppContext.Session().UserId)
	if appErr != nil {
		return ""
	}
	return user.Locale
}

func writeChannelAttributeComplianceJSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}
