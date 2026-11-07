// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"maps"
	"net/http"
	"reflect"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/properties"
)

// propertyFieldOptionsEqual reports whether two values from
// PropertyField.Attrs[options] are equivalent. Used to detect a no-op options
// patch on a linked field — see UpdatePropertyFields' linked-field invariants.
// Both nil/zero forms compare equal; otherwise reflect.DeepEqual handles the
// nested map/slice shape produced by JSON unmarshalling.
//
// A field read above model.PropertyFieldMaxHydratedOptions has no options key,
// so a read-modify-write of one compares nil against nil and is allowed through
// to leave its options untouched. A caller that supplies a real list for such a
// field is refused before reaching the store, by the option-list invariant in
// the property service's UpdatePropertyFields.
func propertyFieldOptionsEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// checkRequiredAttributeTransition blocks flipping a channel attribute to
// required while active, local channels still lack a value. Edge-triggered on
// false->true only: required can legitimately be true while channels are
// non-compliant (it is hard-enforced at channel creation only, and an
// unarchived channel can re-enter the population without a value -- see
// MM-70717), so a level-triggered check would trap admins on unrelated edits
// to an already-required field.
func (a *App) checkRequiredAttributeTransition(rctx request.CTX, callerName, groupID string, prev, next *model.PropertyField) *model.AppError {
	if !a.Config().FeatureFlags.IsChannelAttributesRequiredEnabled() || !model.MinimumEnterpriseAdvancedLicense(a.License()) {
		return nil
	}

	if next.ObjectType != model.PropertyFieldObjectTypeChannel {
		return nil
	}

	if model.IsPropertyFieldRequired(prev) || !model.IsPropertyFieldRequired(next) {
		return nil
	}

	accessControlGroup, appErr := a.GetPropertyGroup(rctx, model.AccessControlPropertyGroupName)
	if appErr != nil {
		return appErr
	}
	if accessControlGroup.ID != groupID {
		return nil
	}

	exists, storeErr := a.Srv().Store().Channel().ExistsChannelMissingPropertyValue(groupID, next.ID)
	if storeErr != nil {
		return model.NewAppError(callerName, "app.channel.missing_property_values.app_error", nil, "", http.StatusInternalServerError).Wrap(storeErr)
	}
	if exists {
		return model.NewAppError(callerName, "api.property_field.required.channels_missing_values.app_error", nil, "", http.StatusConflict)
	}

	return nil
}

func propertyFieldBroadcastParams(rctx request.CTX, field *model.PropertyField) (teamID, channelID string, ok bool) {
	switch field.TargetType {
	case "team":
		return field.TargetID, "", true
	case "channel":
		return "", field.TargetID, true
	case "system":
		return "", "", true
	default:
		rctx.Logger().Warn(
			"Unrecognized property field TargetType, skipping broadcast",
			mlog.String("target_type", field.TargetType),
			mlog.String("field_id", field.ID),
		)
		return "", "", false
	}
}

func (a *App) publishPropertyFieldEvent(rctx request.CTX, eventType model.WebsocketEventType, field *model.PropertyField, connectionID string) {
	if field == nil || field.IsPSAv1() {
		return
	}
	teamID, channelID, ok := propertyFieldBroadcastParams(rctx, field)
	if !ok {
		return
	}
	// A broadcast has no single caller to shape permissions for, and no target
	// narrow enough to make a partial payload safe (a system-scoped field
	// broadcasts to every connected client) -- so the event drops permissions
	// entirely rather than shaping them, the same answer the CPA payload and
	// the access-control autocomplete give. Copy first: field is the object the
	// caller who triggered this event is about to get back themselves.
	broadcastField := *field
	broadcastField.Permissions = nil

	// It has no recipient to filter options against either, so a field whose
	// option list not everyone may read must go out with none at all — a
	// caller reads the field back afterward to get the copy filtered for them.
	// readableByEveryone follows a linked field to its template: a linked
	// field's own Masking is always nil, even when the template whose option
	// list it is broadcasting is masked.
	if field.Type.SupportsOptions() && !a.readableByEveryone(rctx, field.GroupID, field, model.PropertyActionOptionRead) {
		broadcastField.Attrs = make(model.StringInterface, len(field.Attrs))
		maps.Copy(broadcastField.Attrs, field.Attrs)
		broadcastField.HideOptions()
		// The broadcast withheld the list whatever the source field carried, so
		// restore the withheld-options marker HideOptions deletes — a client
		// must be able to tell "options withheld" from "field has no options".
		// options_count stays deleted: on a field not everyone may read the
		// count is controlled information too.
		broadcastField.Attrs[model.PropertyFieldAttributeOptionsOmitted] = true
	}

	fieldJSON, err := json.Marshal(&broadcastField)
	if err != nil {
		rctx.Logger().Warn("Failed to encode property field to JSON", mlog.Err(err))
		return
	}
	message := model.NewWebSocketEvent(eventType, teamID, channelID, "", nil, connectionID)
	message.Add("property_field", string(fieldJSON))
	message.Add("object_type", field.ObjectType)
	a.Publish(message)
}

// rankPropertyFieldGate blocks the user-facing "rank" custom profile attribute
// type while the PropertyFieldRank feature flag is disabled. Consolidated here
// so that both CreatePropertyField (which blocks creating a rank field) and
// UpdatePropertyFields (which blocks converting an existing field to rank)
// share a single check.
//
// The gate is scoped to user-object fields, because that is the only place the
// user-facing rank type can originate: createCPAField forces ObjectType=user
// before reaching CreatePropertyField. Rank fields on other object types are
// intentionally exempt — in particular the classification-markings fields
// (template/system/channel in the access_control group) legitimately use the
// rank type and ship behind the separate, GA-by-default ClassificationMarkings
// flag. Gating those here would break the classification admin panel (create
// and edit alike) whenever PropertyFieldRank is off.
func (a *App) rankPropertyFieldGate(where string, field *model.PropertyField) *model.AppError {
	if field == nil || field.Type != model.PropertyFieldTypeRank {
		return nil
	}
	if field.ObjectType != model.PropertyFieldObjectTypeUser {
		return nil
	}
	if a.Config().FeatureFlags.PropertyFieldRank {
		return nil
	}
	return model.NewAppError(
		where,
		"app.property_field.rank_disabled.app_error",
		nil,
		"rank property fields are not enabled",
		http.StatusBadRequest,
	)
}

// graphPropertyFieldGate blocks creating a "graph" property field while the
// PropertyFieldGraph feature flag is disabled. It runs only on the create
// path — nothing on the update path reads the flag, because
// PropertyService.updatePropertyFields already rejects any conversion to or
// from the graph type unconditionally, so a create-only gate loses no
// coverage.
//
// A graph field that already exists therefore stays fully operable with the
// flag off — its definition and options stay editable, and every path that
// merely reads one is untouched. That half is the one a reader will not
// assume, and it is deliberate: the flag exists to stop new graph fields
// appearing, not to freeze the ones already in use. A flag switched off
// underneath a field somebody is halfway through populating would leave that
// field uncorrectable, with no way out but deleting it — a worse outcome than
// the type shipping with no flag at all.
//
// Unlike rankPropertyFieldGate this is not narrowed to user-object fields.
// Nothing shipped uses the graph type yet, so there is no existing feature to
// exempt, and a hierarchy is normally defined on a template field that user and
// channel fields link to — narrowing to user objects would leave the definition
// itself ungated.
func (a *App) graphPropertyFieldGate(field *model.PropertyField) *model.AppError {
	if field == nil || field.Type != model.PropertyFieldTypeGraph {
		return nil
	}
	if a.Config().FeatureFlags.PropertyFieldGraph {
		return nil
	}
	return model.NewAppError(
		"CreatePropertyField",
		"app.property_field.graph_disabled.app_error",
		nil,
		"graph property fields are not enabled",
		http.StatusBadRequest,
	)
}

// CreatePropertyField creates a new property field.
func (a *App) CreatePropertyField(rctx request.CTX, field *model.PropertyField, bypassProtectedCheck bool, connectionID string) (*model.PropertyField, *model.AppError) {
	if field == nil {
		return nil, model.NewAppError("CreatePropertyField", "app.property_field.invalid_input.app_error", nil, "property field is required", http.StatusBadRequest)
	}

	// Intrinsic invariants (apply to every caller — HTTP, plugin, internal).
	CanonicalizeSystemObjectField(field)
	field.Name = strings.TrimSpace(field.Name)

	// A linked field takes its levels from its template; a default here would
	// read to the property service as the caller pinning PermissionValues.
	if !field.IsPSAv1() && field.Permissions == nil && !field.Protected && field.LinkSourceID() == "" {
		defaultLevel := DefaultPropertyFieldPermissionLevel(field)
		if field.PermissionField == nil {
			field.PermissionField = &defaultLevel
		}
		if field.PermissionValues == nil {
			defaultValuesLevel := DefaultPropertyFieldValuesPermissionLevel(field)
			field.PermissionValues = &defaultValuesLevel
		}
		if field.PermissionOptions == nil {
			field.PermissionOptions = &defaultLevel
		}
	}

	if appErr := a.rankPropertyFieldGate("CreatePropertyField", field); appErr != nil {
		return nil, appErr
	}

	if appErr := a.graphPropertyFieldGate(field); appErr != nil {
		return nil, appErr
	}

	// A field created with a linked_field_id takes its type from the template it
	// links to — the property service copies the source's type over whatever the
	// request said — so the check above cannot see what is about to be created.
	// Gate on the source's type as well, or a graph template hands out new graph
	// fields while the flag is off. Skipped once the flag is on: the gate below
	// is then a no-op regardless of source, so the read would only fetch an
	// argument nothing needs.
	if field.LinkedFieldID != nil && *field.LinkedFieldID != "" && !a.Config().FeatureFlags.PropertyFieldGraph {
		source, appErr := a.GetPropertyField(rctx, field.GroupID, *field.LinkedFieldID)
		switch {
		case appErr != nil && appErr.StatusCode == http.StatusNotFound:
			// No such source in this group. Nothing to gate, and the service has
			// the specific errors for an unusable link target.
		case appErr != nil:
			return nil, appErr
		default:
			if appErr := a.graphPropertyFieldGate(source); appErr != nil {
				return nil, appErr
			}
		}
	}

	// Apply the same false-to-true gate as updates so API, plugin and internal
	// callers cannot bypass the required-attribute rollout by creating a field
	// already marked required. Since a new field has no ID yet, no existing
	// channel can hold a value for it; installations with active local channels
	// must therefore create it optional, backfill values, then mark it required.
	if appErr := a.checkRequiredAttributeTransition(rctx, "CreatePropertyField", field.GroupID, nil, field); appErr != nil {
		return nil, appErr
	}

	if !bypassProtectedCheck && field.Protected {
		return nil, model.NewAppError(
			"CreatePropertyField",
			"app.property_field.create.protected.app_error",
			nil,
			"cannot create protected field",
			http.StatusBadRequest,
		)
	}

	createdField, err := a.Srv().propertyService.CreatePropertyField(rctx, field)
	if err != nil {
		if appErr := mapPropertyServiceError("CreatePropertyField", err); appErr != nil {
			return nil, appErr
		}
		return nil, model.NewAppError("CreatePropertyField", "app.property_field.create.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	a.publishPropertyFieldEvent(rctx, model.WebsocketEventPropertyFieldCreated, createdField, connectionID)

	return createdField, nil
}

// GetPropertyField retrieves a property field by group ID and field ID.
func (a *App) GetPropertyField(rctx request.CTX, groupID, fieldID string) (*model.PropertyField, *model.AppError) {
	field, err := a.Srv().propertyService.GetPropertyField(rctx, groupID, fieldID)
	if err != nil {
		if appErr := mapPropertyServiceError("GetPropertyField", err); appErr != nil {
			return nil, appErr
		}
		return nil, model.NewAppError("GetPropertyField", "app.property_field.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return field, nil
}

// GetPropertyFields retrieves multiple property fields by their IDs.
func (a *App) GetPropertyFields(rctx request.CTX, groupID string, ids []string) ([]*model.PropertyField, *model.AppError) {
	fields, err := a.Srv().propertyService.GetPropertyFields(rctx, groupID, ids)
	if err != nil {
		if appErr := mapPropertyServiceError("GetPropertyFields", err); appErr != nil {
			return nil, appErr
		}
		return nil, model.NewAppError("GetPropertyFields", "app.property_field.get_many.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return fields, nil
}

// GetPropertyFieldByName retrieves a property field by name within a group and target.
//
// Deprecated: name is not unique within a group when fields of different object
// types share a name. Use GetPropertyFieldByNameForObjectType to scope the
// lookup to a single object type and get a deterministic result.
func (a *App) GetPropertyFieldByName(rctx request.CTX, groupID, targetID, name string) (*model.PropertyField, *model.AppError) {
	field, err := a.Srv().propertyService.GetPropertyFieldByName(rctx, groupID, targetID, name)
	if err != nil {
		if appErr := mapPropertyServiceError("GetPropertyFieldByName", err); appErr != nil {
			return nil, appErr
		}
		return nil, model.NewAppError("GetPropertyFieldByName", "app.property_field.get_by_name.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return field, nil
}

// GetPropertyFieldByNameForObjectType retrieves a property field by name within
// a group and target, scoped to a single object type. Prefer this over
// GetPropertyFieldByName when a name may be shared across object types within a
// group (e.g. the access_control "classification" fields).
func (a *App) GetPropertyFieldByNameForObjectType(rctx request.CTX, groupID, targetID, objectType, name string) (*model.PropertyField, *model.AppError) {
	field, err := a.Srv().propertyService.GetPropertyFieldByNameForObjectType(rctx, groupID, targetID, objectType, name)
	if err != nil {
		if appErr := mapPropertyServiceError("GetPropertyFieldByNameForObjectType", err); appErr != nil {
			return nil, appErr
		}
		return nil, model.NewAppError("GetPropertyFieldByNameForObjectType", "app.property_field.get_by_name.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return field, nil
}

// SearchPropertyFields searches for property fields matching the given options.
func (a *App) SearchPropertyFields(rctx request.CTX, groupID string, opts model.PropertyFieldSearchOpts) ([]*model.PropertyField, *model.AppError) {
	fields, err := a.Srv().propertyService.SearchPropertyFields(rctx, groupID, opts)
	if err != nil {
		if appErr := mapPropertyServiceError("SearchPropertyFields", err); appErr != nil {
			return nil, appErr
		}
		return nil, model.NewAppError("SearchPropertyFields", "app.property_field.search.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return fields, nil
}

// GetPropertyFieldsForGroup retrieves all active property fields for a group.
func (a *App) GetPropertyFieldsForGroup(rctx request.CTX, groupID string) ([]*model.PropertyField, *model.AppError) {
	fields, err := a.Srv().propertyService.GetPropertyFieldsForGroup(rctx, groupID)
	if err != nil {
		if appErr := mapPropertyServiceError("GetPropertyFieldsForGroup", err); appErr != nil {
			return nil, appErr
		}
		return nil, model.NewAppError("GetPropertyFieldsForGroup", "app.property_field.get_for_group.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return fields, nil
}

// CountPropertyFieldsForGroup counts property fields for a group.
func (a *App) CountPropertyFieldsForGroup(rctx request.CTX, groupID string, includeDeleted bool) (int64, *model.AppError) {
	var count int64
	var err error
	if includeDeleted {
		count, err = a.Srv().propertyService.CountAllPropertyFieldsForGroup(rctx, groupID)
	} else {
		count, err = a.Srv().propertyService.CountActivePropertyFieldsForGroup(rctx, groupID)
	}

	if err != nil {
		if appErr := mapPropertyServiceError("CountPropertyFieldsForGroup", err); appErr != nil {
			return 0, appErr
		}
		return 0, model.NewAppError("CountPropertyFieldsForGroup", "app.property_field.count_for_group.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return count, nil
}

// CountPropertyFieldsForTarget counts property fields for a specific target.
func (a *App) CountPropertyFieldsForTarget(rctx request.CTX, groupID, targetType, targetID string, includeDeleted bool) (int64, *model.AppError) {
	var count int64
	var err error
	if includeDeleted {
		count, err = a.Srv().propertyService.CountAllPropertyFieldsForTarget(rctx, groupID, targetType, targetID)
	} else {
		count, err = a.Srv().propertyService.CountActivePropertyFieldsForTarget(rctx, groupID, targetType, targetID)
	}

	if err != nil {
		if appErr := mapPropertyServiceError("CountPropertyFieldsForTarget", err); appErr != nil {
			return 0, appErr
		}
		return 0, model.NewAppError("CountPropertyFieldsForTarget", "app.property_field.count_for_target.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return count, nil
}

// UpdatePropertyField updates an existing property field. The second return
// value lists the IDs of fields whose dependent property values were cleared
// as a side effect (e.g. by TypeChangeValueCleanupHook on a type change).
// Hooks may cascade clears to other fields, so the slice is not necessarily
// limited to the updated field's own ID.
func (a *App) UpdatePropertyField(rctx request.CTX, groupID string, field *model.PropertyField, bypassProtectedCheck bool, connectionID string) (*model.PropertyField, []string, *model.AppError) {
	fields, clearedIDs, err := a.UpdatePropertyFields(rctx, groupID, []*model.PropertyField{field}, bypassProtectedCheck, connectionID)
	if err != nil {
		return nil, nil, err
	}

	return fields[0], clearedIDs, nil
}

// UpdatePropertyFields updates multiple property fields. The second return
// value lists the IDs of fields whose dependent property values were cleared
// as a side effect.
func (a *App) UpdatePropertyFields(rctx request.CTX, groupID string, fields []*model.PropertyField, bypassProtectedCheck bool, connectionID string) ([]*model.PropertyField, []string, *model.AppError) {
	if len(fields) == 0 {
		return nil, nil, model.NewAppError("UpdatePropertyFields", "app.property_field.invalid_input.app_error", nil, "property fields are required", http.StatusBadRequest)
	}

	// Intrinsic invariants — apply to every caller (HTTP, plugin, internal).
	// Service returns DB-order, not input-order, so we'll build a lookup map
	// keyed by ID below; collect IDs in this same pass.
	ids := make([]string, len(fields))
	for i, f := range fields {
		f.Name = strings.TrimSpace(f.Name)
		ids[i] = f.ID
	}

	// Load existing fields once. Used for: protected-check (gated by
	// bypassProtectedCheck), linked-field diff invariants (always-on).

	existingFields, err := a.Srv().propertyService.GetPropertyFields(rctx, groupID, ids)
	if err != nil {
		if appErr := mapPropertyServiceError("UpdatePropertyFields", err); appErr != nil {
			return nil, nil, appErr
		}
		return nil, nil, model.NewAppError("UpdatePropertyFields", "app.property_field.update.get_existing.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	existingByID := make(map[string]*model.PropertyField, len(existingFields))
	for _, ex := range existingFields {
		existingByID[ex.ID] = ex
	}

	for _, f := range fields {
		existing, ok := existingByID[f.ID]
		if !ok {
			// Service-level GetPropertyFields returns an ErrResultsMismatch when
			// any input ID is missing, so this branch is defensive.
			continue
		}

		// Rank-type gate: block converting a field to rank while the feature
		// flag is off (shares the create-path check).
		if appErr := a.rankPropertyFieldGate("UpdatePropertyFields", f); appErr != nil {
			return nil, nil, appErr
		}

		// Linked-field diff invariants. "Linked" = LinkedFieldID != nil &&
		// *LinkedFieldID != "". Unlink (nil or "") is always allowed when
		// existing was linked. This is an early refusal for a clean 400; the
		// property service (property_field.go, updatePropertyFields) enforces
		// the same four rules on every caller, so removing one here must never
		// turn a refusal into a write.
		existingLinked := existing.LinkedFieldID != nil && *existing.LinkedFieldID != ""
		incomingLinked := f.LinkedFieldID != nil && *f.LinkedFieldID != ""

		if existingLinked {
			if f.Type != existing.Type {
				return nil, nil, model.NewAppError(
					"UpdatePropertyFields",
					"app.property_field.update.linked_type_change.app_error",
					map[string]any{"FieldID": existing.ID},
					"cannot modify type of a linked field",
					http.StatusBadRequest,
				)
			}
			// Compare the options portion of Attrs.
			var existingOpts, incomingOpts any
			if existing.Attrs != nil {
				existingOpts = existing.Attrs[model.PropertyFieldAttributeOptions]
			}
			if f.Attrs != nil {
				incomingOpts = f.Attrs[model.PropertyFieldAttributeOptions]
			}
			if !propertyFieldOptionsEqual(existingOpts, incomingOpts) {
				return nil, nil, model.NewAppError(
					"UpdatePropertyFields",
					"app.property_field.update.linked_options_change.app_error",
					map[string]any{"FieldID": existing.ID},
					"cannot modify options of a linked field",
					http.StatusBadRequest,
				)
			}
			if incomingLinked && *f.LinkedFieldID != *existing.LinkedFieldID {
				return nil, nil, model.NewAppError(
					"UpdatePropertyFields",
					"app.property_field.update.cannot_change_link_target.app_error",
					map[string]any{"FieldID": existing.ID},
					"cannot change link target",
					http.StatusBadRequest,
				)
			}
		} else if incomingLinked {
			return nil, nil, model.NewAppError(
				"UpdatePropertyFields",
				"app.property_field.update.cannot_link_existing.app_error",
				map[string]any{"FieldID": existing.ID},
				"linked_field_id can only be set at creation time",
				http.StatusBadRequest,
			)
		}

		// Protected-check is the only invariant gated on the caller's opt-out.
		if !bypassProtectedCheck && model.ProjectLegacyPermissions(existing).Protected && !a.isAccessControlGroup(groupID) {
			return nil, nil, model.NewAppError(
				"UpdatePropertyFields",
				"app.property_field.update.protected.app_error",
				map[string]any{"FieldID": existing.ID},
				"cannot update protected field",
				http.StatusForbidden,
			)
		}

		if appErr := a.checkRequiredAttributeTransition(rctx, "UpdatePropertyFields", groupID, existing, f); appErr != nil {
			return nil, nil, appErr
		}
	}

	updated, propagated, clearedFieldIDs, err := a.Srv().propertyService.UpdatePropertyFields(rctx, groupID, fields)
	if err != nil {
		if appErr := mapPropertyServiceError("UpdatePropertyFields", err); appErr != nil {
			return nil, nil, appErr
		}
		return nil, nil, model.NewAppError("UpdatePropertyFields", "app.property_field.update.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// This runs before the websocket broadcast so a client reacting to the
	// event never re-reads stale cached metadata (mirrors the ordering in
	// DeletePropertyField).
	for _, field := range updated {
		a.invalidatePolicyCachesForOptionChange(rctx, field.ID)
	}
	for _, field := range propagated {
		a.invalidatePolicyCachesForOptionChange(rctx, field.ID)
	}

	// Renaming or re-ranking an option rewrites what the matview materializes, and a type change
	// drops dependent values, none of it touching the rows the per-user epoch is computed from.
	if anyUserObjectType(updated) || anyUserObjectType(propagated) {
		a.invalidateAllUserAttributeCaches()
	}

	// Broadcast websocket events for the requested fields and for the linked
	// fields whose derived option list changed with them
	for _, field := range updated {
		a.publishPropertyFieldEvent(rctx, model.WebsocketEventPropertyFieldUpdated, field, connectionID)
	}
	for _, field := range propagated {
		a.publishPropertyFieldEvent(rctx, model.WebsocketEventPropertyFieldUpdated, field, "")
	}

	// For each field whose dependent values were cleared as a side effect of
	// the update (e.g. a type change handled by TypeChangeValueCleanupHook),
	// publish the generic property_values_updated event so subscribers refresh
	// their local caches. Mirrors App.DeletePropertyValuesForField's wire shape.
	for _, fieldID := range clearedFieldIDs {
		message := model.NewWebSocketEvent(model.WebsocketEventPropertyValuesUpdated, "", "", "", nil, "")
		message.Add("field_id", fieldID)
		message.Add("values", "[]")
		a.Publish(message)
	}

	return updated, clearedFieldIDs, nil
}

// anyUserObjectType reports whether any field is one the AttributeView materializes, and that ABAC
// policies can therefore match on.
func anyUserObjectType(fields []*model.PropertyField) bool {
	for _, f := range fields {
		if f != nil && f.ObjectType == model.PropertyFieldObjectTypeUser {
			return true
		}
	}
	return false
}

// isAccessControlGroup reports whether groupID is the access_control group.
// There the AccessControlHook decides a protected field's writes from its
// Permissions, so the app layer's blanket protected guard must not run first.
// A failed lookup reports false, which keeps the guard.
func (a *App) isAccessControlGroup(groupID string) bool {
	group, err := a.Srv().propertyService.GroupByID(groupID)
	return err == nil && group.Name == model.AccessControlPropertyGroupName
}

// OrphanedPropertyFieldForDelete returns the field a session's delete of field
// should be judged on; see properties.OrphanedFieldForDelete.
func (a *App) OrphanedPropertyFieldForDelete(field *model.PropertyField) (*model.PropertyField, bool) {
	return properties.OrphanedFieldForDelete(field, a.IsInstalledPlugin)
}

// DeletePropertyField deletes a property field.
func (a *App) DeletePropertyField(rctx request.CTX, groupID, id string, bypassProtectedCheck bool, connectionID string) *model.AppError {
	existing, err := a.Srv().propertyService.GetPropertyField(rctx, groupID, id)
	if err != nil {
		if appErr := mapPropertyServiceError("DeletePropertyField", err); appErr != nil {
			return appErr
		}
		return model.NewAppError("DeletePropertyField", "app.property_field.delete.get_existing.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if existing == nil {
		return model.NewAppError("DeletePropertyField", "app.property_field.delete.not_found.app_error", nil, "", http.StatusNotFound)
	}

	if !bypassProtectedCheck && model.ProjectLegacyPermissions(existing).Protected && !a.isAccessControlGroup(groupID) {
		return model.NewAppError(
			"DeletePropertyField",
			"app.property_field.delete.protected.app_error",
			nil,
			"cannot delete protected field",
			http.StatusForbidden,
		)
	}

	if err := a.Srv().propertyService.DeletePropertyField(rctx, groupID, id); err != nil {
		if appErr := mapPropertyServiceError("DeletePropertyField", err); appErr != nil {
			return appErr
		}
		return model.NewAppError("DeletePropertyField", "app.property_field.delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// The matview filters out soft-deleted fields, so this attribute disappears from every subject
	// while PropertyValues stays untouched and the per-user epoch cannot see it.
	if existing.ObjectType == model.PropertyFieldObjectTypeUser {
		a.invalidateAllUserAttributeCaches()
	}

	// Without this a deleted rank field's stale options would linger in the
	// per-node cache until restart.
	a.invalidatePolicyCachesForOptionChange(rctx, existing.ID)

	if existing.IsPSAv2() {
		teamID, channelID, ok := propertyFieldBroadcastParams(rctx, existing)
		if ok {
			message := model.NewWebSocketEvent(model.WebsocketEventPropertyFieldDeleted, teamID, channelID, "", nil, connectionID)
			message.Add("field_id", existing.ID)
			message.Add("object_type", existing.ObjectType)
			a.Publish(message)
		}
	}

	return nil
}
