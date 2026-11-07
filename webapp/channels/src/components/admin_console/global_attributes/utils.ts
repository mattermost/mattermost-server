// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Dispatch} from 'redux';

import {supportsHierarchy, type ChannelAttributeComplianceSummary, type ChannelAttributeNotifyResult, type ChannelsMissingAttributeValueList, type PropertyField, type PropertyFieldOption, type PropertyPermissionLevel} from '@mattermost/types/properties';

import {GeneralTypes} from 'mattermost-redux/action_types';
import {Client4} from 'mattermost-redux/client';

import {clearGraphOptionNamesForField, commitGraphOptionNames} from 'components/property_fields/graph/use_graph_option_names';

import {ALL_RESOURCE_TYPES} from './attribute_details/attribute_applies_to_constants';
import type {ResourceObjectType} from './attribute_details/attribute_applies_to_constants';
import {ALL_EXTERNAL_SOURCES} from './attribute_details/external_source';
import type {ExternalSourceLinks} from './attribute_details/external_source';
import type {AttributeFieldType, AttributeTypeId} from './attribute_type';
import {toServerFieldType, toValueType} from './attribute_type';
import {GLOBAL_ATTRIBUTES_GROUP_NAME, GLOBAL_ATTRIBUTES_OBJECT_TYPE, GLOBAL_ATTRIBUTES_TARGET_TYPE} from './constants';

const USER_RESOURCE_OBJECT_TYPE: ResourceObjectType = 'user';

// Attribute Management writes user attributes through the generic property-fields
// API, which never touches the custom profile attribute slice that User
// Management, the profile popover and Account Settings → Profile all read. The
// server also omits the saving connection from property_field_* broadcasts, so
// the admin who made the change gets no websocket event of their own. Without
// mirroring the write here, every one of those surfaces in that admin's tab
// stays stale until a reload. A field returned from the create/patch call
// carries its authoritative option list (unlike the stripped broadcast), so it
// can be stored as-is.
//
// Graph option names live in a module-level cache that useGraphOptionNames
// prefers over the field's inline list. Remote property_field_* events already
// clear and recommit that cache; the originating tab has to do the same here
// or a renamed option keeps showing its old name until reload.
export function syncUserAttributeFieldUpsert(dispatch: Dispatch, field: PropertyField, created: boolean): void {
    if (field.object_type !== USER_RESOURCE_OBJECT_TYPE) {
        return;
    }
    if (supportsHierarchy(field)) {
        clearGraphOptionNamesForField(field.id);
        const optionsOmitted = field.attrs?.options_omitted === true;
        if (!optionsOmitted && Array.isArray(field.attrs?.options)) {
            const names: Record<string, string> = {};
            for (const option of field.attrs.options as PropertyFieldOption[]) {
                if (option.id) {
                    names[option.id] = option.name;
                }
            }
            commitGraphOptionNames(field.id, names);
        }

        // Withheld lists leave the cache empty. User Detail and the profile
        // popover pass {walk: true}; Account Settings shows a count until an
        // editable picker mounts and must not walk from this helper.
    }
    dispatch({
        type: created ? GeneralTypes.CUSTOM_PROFILE_ATTRIBUTE_FIELD_CREATED : GeneralTypes.CUSTOM_PROFILE_ATTRIBUTE_FIELD_PATCHED,
        data: field,
    });
}

export function syncUserAttributeFieldDelete(dispatch: Dispatch, objectType: string, fieldId: string): void {
    if (objectType !== USER_RESOURCE_OBJECT_TYPE) {
        return;
    }
    dispatch({type: GeneralTypes.CUSTOM_PROFILE_ATTRIBUTE_FIELD_DELETED, data: fieldId});
}

export type {AttributeFieldType, AttributeTypeId} from './attribute_type';

export const ATTRIBUTE_FIELD_TYPES: readonly AttributeFieldType[] = ['text', 'select', 'multiselect', 'rank', 'graph'];

export function isAttributeFieldType(value: string): value is AttributeFieldType {
    return (ATTRIBUTE_FIELD_TYPES as readonly string[]).includes(value);
}

// Server clamps per_page to this max (web.PerPageMaximum). Directory-mode
// listing with no cursor sorts CreateAt ASC, so a default 60-item page can
// miss a freshly created field.
const MAX_PROPERTY_FIELDS_PER_PAGE = 200;

// Builds the attrs.options payload for the given type: {id: '', name} for
// Select/Multiselect (id is a required string on PropertyFieldOption, but the
// server always generates the real one -- EnsureOptionIDs /
// sanitizeAndValidateOptions); {id: '', name, rank} for Rank, with rank always
// explicitly present (never inferred from array position -- the server's
// validateRankOptions hard-errors on create if it's missing); {id: '', name,
// parents} for Graph, with parents always present (roots send [] — omitting
// the key is a server no-op). Text has no options key at all.
export function buildOptionsAttr(fieldType: AttributeFieldType, options: PropertyFieldOption[]): PropertyFieldOption[] | undefined {
    switch (fieldType) {
    case 'select':
    case 'multiselect':
        return options.map(({name}) => ({id: '', name}));
    case 'rank':
        return options.map(({name, rank}) => ({id: '', name, rank}));
    case 'graph':
        return options.map(({name, parents}) => ({id: '', name, parents: parents ?? []}));
    default:
        return undefined;
    }
}

// Patch keeps existing option IDs so stored values stay attached. New options
// still send an empty id for the server to mint. Other option properties
// (color, rank, …) are spread through: mergeAttrs replaces the whole options
// array, so omitting them would drop chip colors on a standalone channel
// select. Graph always sends parents (roots send [] — omitting the key is a
// server no-op). Text sends null so mergeAttrs drops a leftover options key
// when switching away from Select/Multiselect/Rank/Graph.
function buildPatchOptionsAttr(fieldType: AttributeFieldType, options: PropertyFieldOption[]): PropertyFieldOption[] | null {
    switch (fieldType) {
    case 'select':
    case 'multiselect':
    case 'rank':
        return options.map((option) => ({...option, id: option.id || ''}));
    case 'graph':
        return options.map((option) => ({...option, id: option.id || '', parents: option.parents ?? []}));
    case 'text':
        return null;
    default: {
        const exhaustive: never = fieldType;
        return exhaustive;
    }
    }
}

export async function listPropertyFields(objectType: string): Promise<PropertyField[]> {
    const fields: PropertyField[] = [];
    let cursorId: string | undefined;
    let cursorCreateAt: number | undefined;

    while (true) {
        // eslint-disable-next-line no-await-in-loop
        const page = await Client4.getPropertyFields(
            GLOBAL_ATTRIBUTES_GROUP_NAME,
            objectType,
            {
                targetType: GLOBAL_ATTRIBUTES_TARGET_TYPE,
                perPage: MAX_PROPERTY_FIELDS_PER_PAGE,
                cursorId,
                cursorCreateAt,
            },
        );
        fields.push(...page);
        if (page.length === 0) {
            break;
        }
        const last = page[page.length - 1];
        cursorId = last.id;
        cursorCreateAt = last.create_at;
        if (page.length < MAX_PROPERTY_FIELDS_PER_PAGE) {
            break;
        }
    }

    return fields;
}

// There is no GET-by-id property-fields HTTP handler. List every object type a
// field can live in and find the one whose id matches. A user/channel/post
// field only counts when it isn't a template's linked child (linked_field_id
// set) -- those aren't listed or edited on their own, so an id that only
// resolves to one returns undefined and the details page redirects to the list.
//
// allowedTypes is what this server actually offers (see useAllowedResourceTypes):
// a resource behind an off feature flag is not listed, and channel in particular
// must not be, since the server 501s a channel-scoped access_control GET below
// Enterprise Advanced -- fetching it unconditionally would reject the whole
// Promise.all and bounce every details page to the list, even one editing a user
// or template field.
export async function fetchAttributeField(fieldId: string, allowedTypes: readonly ResourceObjectType[]): Promise<PropertyField | undefined> {
    const objectTypes = [GLOBAL_ATTRIBUTES_OBJECT_TYPE, ...ALL_RESOURCE_TYPES.filter((type) => allowedTypes.includes(type))];
    const pages = await Promise.all(
        objectTypes.map((objectType) => listPropertyFields(objectType)),
    );
    return pages.flat().find((field) => (
        field.id === fieldId &&
        field.delete_at === 0 &&
        (field.object_type === GLOBAL_ATTRIBUTES_OBJECT_TYPE || !field.linked_field_id)
    ));
}

export function isResourceObjectType(value: string): value is ResourceObjectType {
    return (ALL_RESOURCE_TYPES as string[]).includes(value);
}

// Lists user/channel/post fields and keeps those pointing at the template.
// There is no cross-object-type listing endpoint.
//
// allowedTypes matches fetchAttributeField. That earlier fetch's own skip does
// not protect this later Promise.all; a 501 on the channel scope would reject
// the whole load and bounce the template details page to the list.
export async function fetchLinkedFieldsForTemplate(templateFieldId: string, allowedTypes: readonly ResourceObjectType[]): Promise<PropertyField[]> {
    const objectTypes = ALL_RESOURCE_TYPES.filter((type) => allowedTypes.includes(type));
    const pages = await Promise.all(
        objectTypes.map((objectType) => listPropertyFields(objectType)),
    );
    return pages.flat().filter((field) => (
        field.linked_field_id === templateFieldId &&
        field.delete_at === 0 &&
        isResourceObjectType(field.object_type)
    ));
}

export function linkedFieldsByResourceType(fields: PropertyField[]): Partial<Record<ResourceObjectType, PropertyField>> {
    const byType: Partial<Record<ResourceObjectType, PropertyField>> = {};
    for (const field of fields) {
        if (isResourceObjectType(field.object_type) && !byType[field.object_type]) {
            byType[field.object_type] = field;
        }
    }
    return byType;
}

// Live fields in `fields` that an admin would read as the same attribute as
// `name`, ignoring `templateId` itself and anything linked to it.
//
// Name, not display name: a CEL rule refers to a field by its name
// (user.attributes.clearance), so two fields sharing one are indistinguishable
// to whoever writes the rule. The comparison ignores case even though the
// server's uniqueness check does not (buildConflictSubquery in
// sqlstore/property_field_store.go compares Name exactly) -- a template named
// `Clearance` never collides on the server with a user field named
// `clearance`, yet an admin sees the two as one attribute.
export function findNameConflicts(name: string, fields: PropertyField[], templateId?: string): PropertyField[] {
    const target = name.toLowerCase();
    return fields.filter((field) => (
        field.delete_at === 0 &&
        field.name.toLowerCase() === target &&
        (!templateId || (field.id !== templateId && field.linked_field_id !== templateId))
    ));
}

// Creates a template field in the access_control group. target_type/target_id
// are set explicitly because CanonicalizeSystemObjectField only auto-corrects
// ObjectType=system fields, not ObjectType=template ones.
//
// `links` is a trailing optional parameter (not folded into the existing 4
// positional args), keyed by source so its same-typed strings can't be swapped
// with each other or with displayName/name, and every existing call site keeps
// compiling unchanged.
export function createAttributeField(
    displayName: string,
    name: string,
    typeId: AttributeTypeId,
    options: PropertyFieldOption[],
    links?: Partial<ExternalSourceLinks>,
): Promise<PropertyField> {
    const fieldType = toServerFieldType(typeId);
    const valueType = toValueType(typeId);
    const optionsAttr = buildOptionsAttr(fieldType, options);
    return Client4.createPropertyField(GLOBAL_ATTRIBUTES_GROUP_NAME, GLOBAL_ATTRIBUTES_OBJECT_TYPE, {
        name,
        type: fieldType as PropertyField['type'],
        target_type: GLOBAL_ATTRIBUTES_TARGET_TYPE,
        target_id: '',
        attrs: {
            display_name: displayName.trim() || undefined,
            ...(optionsAttr ? {options: optionsAttr} : {}),
            ...(valueType ? {value_type: valueType} : {}),
            ...Object.fromEntries(ALL_EXTERNAL_SOURCES.filter((source) => links?.[source]).map((source) => [source, links?.[source]])),
        },
    });
}

export type UpdateAttributeFieldPatch = {
    name?: string;
    type: AttributeTypeId;
    displayName: string;

    // Omitted to leave the options as the server has them: those of an
    // attribute synced from an identity source belong to its sync, which may
    // have added some since this editor loaded them.
    options?: PropertyFieldOption[];
    externalLinks: ExternalSourceLinks;
    resourceAttrs?: Record<string, unknown>;
    permissionValues?: PropertyPermissionLevel;
};

// Attrs are merge-patched (mergeAttrs=true on the server): each sync source
// (ldap, saml, openid) sends null to unlink, Text sends options: null so a leftover options array is
// dropped, and value_type sends null so a leftover phone/url/email subtype
// is dropped when switching away. name is omitted when unchanged so the
// server skips uniqueness re-validation, and options when the patch carries
// none.
export function updateAttributeField(
    objectType: string,
    fieldId: string,
    patch: UpdateAttributeFieldPatch,
): Promise<PropertyField> {
    const fieldType = toServerFieldType(patch.type);
    const valueType = toValueType(patch.type);
    return Client4.patchPropertyField(GLOBAL_ATTRIBUTES_GROUP_NAME, objectType, fieldId, {
        ...(patch.name === undefined ? {} : {name: patch.name}),
        type: fieldType as PropertyField['type'],
        attrs: {
            display_name: patch.displayName.trim() || undefined,
            ...(patch.options === undefined ? {} : {options: buildPatchOptionsAttr(fieldType, patch.options)}),
            ...Object.fromEntries(ALL_EXTERNAL_SOURCES.map((source) => [source, patch.externalLinks[source] || null])),
            value_type: valueType || null,
            ...patch.resourceAttrs,
        },
        ...(patch.permissionValues ? {permission_values: patch.permissionValues} : {}),
    });
}

// The server returns 409 when the field still has active linked dependents
// (CountLinkedFields > 0); callers are expected to surface that case distinctly
// (or, for a save-time rollback, to only delete linked fields first -- see
// createLinkedAttributeField).
export function deleteAttributeField(objectType: string, fieldId: string): Promise<unknown> {
    return Client4.deletePropertyField(GLOBAL_ATTRIBUTES_GROUP_NAME, objectType, fieldId);
}

// Creates a linked field for one Applies-to resource. The server validates
// linked_field_id against the template and copies its Type, attrs.options,
// and ldap/saml onto the new field (server/channels/app/properties/property_field.go)
// -- display_name and value_type are NOT copied, so both are sent explicitly
// here. objectType is the resource type ('user'/'channel'/'post'), a URL path
// segment on the generic property-fields endpoint, not a separate route.
//
// `attrs` is what the resource's own settings contribute -- e.g. a Users row's
// Profile display config, or a Channels row's required/change-policy attrs
// (buildChannelFieldAttrs) -- bundled directly into the create request so the
// row is never created bare and then immediately patched. Trailing and
// optional so a resource type with no config panel yet keeps calling this
// unchanged.
//
// `permissionValues` sets the field's actual write-permission tier -- a
// top-level PropertyField field, not part of attrs. The server would
// otherwise inherit this from the template (always sysadmin), overriding
// whatever the caller picked; passing it explicitly here is honored for a
// linked field (see server/channels/app/properties/property_field.go).
// Omitting it takes the server's own per-object-type default.
export function createLinkedAttributeField(
    objectType: ResourceObjectType,
    name: string,
    typeId: AttributeTypeId,
    displayName: string,
    linkedFieldId: string,
    attrs?: Record<string, unknown>,
    permissionValues?: PropertyPermissionLevel,
): Promise<PropertyField> {
    const fieldType = toServerFieldType(typeId);
    const valueType = toValueType(typeId);
    return Client4.createPropertyField(GLOBAL_ATTRIBUTES_GROUP_NAME, objectType, {
        name,
        type: fieldType as PropertyField['type'],
        target_type: GLOBAL_ATTRIBUTES_TARGET_TYPE,
        target_id: '',
        linked_field_id: linkedFieldId,
        ...(permissionValues ? {permission_values: permissionValues} : {}),
        attrs: {
            display_name: displayName.trim() || undefined,
            ...(valueType ? {value_type: valueType} : {}),
            ...attrs,
        },
    });
}

// Deletes a linked field for one Applies-to resource. Must be called before
// deleteAttributeField on the template it points at -- the server blocks
// deleting a template with active linked dependents.
export function deleteLinkedAttributeField(objectType: ResourceObjectType, fieldId: string): Promise<unknown> {
    return Client4.deletePropertyField(GLOBAL_ATTRIBUTES_GROUP_NAME, objectType, fieldId);
}

// PATCHes a linked field's config for an already-persisted Applies-to
// resource (e.g. a Users row's Profile display, or a Channels row's own
// settings, changed after the row itself was already saved). Attrs are
// merge-patched (mergeAttrs=true on the server, same as updateAttributeField
// above) -- only the keys present in `attrs` are updated, everything else on
// the field is left untouched.
//
// permissionValues patches the field's actual write-permission tier -- see
// createLinkedAttributeField above for why this is a top-level field, not
// part of attrs.
export function patchLinkedAttributeField(
    objectType: ResourceObjectType,
    fieldId: string,
    attrs?: Record<string, unknown>,
    permissionValues?: PropertyPermissionLevel,
): Promise<PropertyField> {
    return Client4.patchPropertyField(GLOBAL_ATTRIBUTES_GROUP_NAME, objectType, fieldId, {
        ...(attrs ? {attrs} : {}),
        ...(permissionValues ? {permission_values: permissionValues} : {}),
    });
}

// --- Required-channel-attribute compliance (MM-70717) ---
//
// fieldId is undefined in create mode: the channel-linked field has no ID yet
// (nothing has been saved), so the server treats every active channel as
// missing a value.

const CHANNEL_MISSING_VALUES_PER_PAGE = 20;

export function fetchChannelsMissingValueSummary(fieldId?: string): Promise<ChannelAttributeComplianceSummary> {
    return Client4.getChannelAttributeComplianceSummary(GLOBAL_ATTRIBUTES_GROUP_NAME, fieldId);
}

export function fetchChannelsMissingValue(fieldId: string, page: number, perPage: number = CHANNEL_MISSING_VALUES_PER_PAGE): Promise<ChannelsMissingAttributeValueList> {
    return Client4.getChannelsMissingAttributeValue(GLOBAL_ATTRIBUTES_GROUP_NAME, fieldId, page, perPage);
}

export function notifyChannelAdminsOfMissingValue(fieldId: string): Promise<ChannelAttributeNotifyResult> {
    return Client4.notifyChannelAdminsOfMissingAttributeValue(GLOBAL_ATTRIBUTES_GROUP_NAME, fieldId);
}
