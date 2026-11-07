// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React, {useCallback, useEffect, useMemo, useRef, useState, type JSX, type ReactNode} from 'react';
import type {IntlShape, MessageDescriptor} from 'react-intl';
import {defineMessages, FormattedMessage, useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';
import {useParams} from 'react-router-dom';
import type {Dispatch} from 'redux';

import type {ClientError} from '@mattermost/client';
import {ChevronLeftIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/compass-ui/components/button';
import {SectionNotice} from '@mattermost/compass-ui/components/section-notice';
import {WithTooltip} from '@mattermost/shared/components/tooltip';
import type {FieldVisibility, PropertyField, PropertyFieldOption, PropertyPermissionLevel} from '@mattermost/types/properties';
import {supportsHierarchy, supportsOptions} from '@mattermost/types/properties';
import type {PropertyFieldOwner} from '@mattermost/types/properties_user';

import {formatPropertyFieldLabel} from 'mattermost-redux/utils/property_utils';

import {setNavigationBlocked} from 'actions/admin_actions';

import BlockableLink from 'components/admin_console/blockable_link';
import {findRankCollision, isValidRank} from 'components/admin_console/system_properties/rank_utils';
import {useInstalledPluginIds, useIsFieldOrphaned, usePluginInventoryLoaded} from 'components/common/hooks/use_field_orphaned';
import useGetFeatureFlagValue from 'components/common/hooks/useGetFeatureFlagValue';
import LoadingScreen from 'components/loading_screen';
import {pageAllAccessControlFieldOptions} from 'components/property_fields/graph/page_all_access_control_field_options';
import SaveButton from 'components/save_button';
import AdminHeader from 'components/widgets/admin_console/admin_header';
import AdminPanel from 'components/widgets/admin_console/admin_panel';
import Input from 'components/widgets/inputs/input/input';

import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';
import {allOwnersAreUninstalledPlugins, CPA_FIELD_NAME_MAX_RUNES, filterCELIdentifier, slugifyForCEL, validateCPAFieldName} from 'utils/properties';
import type {CPAFieldNameValidationError} from 'utils/properties';

import AttributeAppliesTo from './attribute_applies_to';
import {ALL_RESOURCE_TYPES, ATTRIBUTE_APPLIES_TO_ADD_HEADER_TRIGGER_ID, resourceTypeLabels} from './attribute_applies_to_constants';
import type {ResourceObjectType, UserManagedValue} from './attribute_applies_to_constants';
import AttributeExternalSource from './attribute_external_source';
import AttributeOptionsRankValues from './attribute_options_rank_values';
import AttributeOptionsValues from './attribute_options_values';
import AttributeOwnersSource from './attribute_owners_source';
import AttributePluginSource from './attribute_plugin_source';
import {useConfirmRemoveAppliesTo} from './attribute_remove_applies_to_warning_modal';
import AttributeSelect from './attribute_select';
import type {ExternalSource, ExternalSourceLinks} from './external_source';
import {ALL_EXTERNAL_SOURCES, NO_EXTERNAL_SOURCE_LINKS, externalSourceLinksFromField, externalSourceMessages, hasExternalSourceLink, resolveExternalSource} from './external_source';
import {GraphValues, hasBlankTrimmedOptionName, hasCaseInsensitiveDuplicateNames} from './graph';

import {CHANNEL_VALUE_SETTER, DEFAULT_CHANNEL_RESOURCE_CONFIG, buildChannelFieldAttrs, buildChannelFieldPatch, isOrderedChangePolicy, parseChannelFieldConfig} from '../applies_to/channels';
import type {ChannelResourceConfig} from '../applies_to/channels';
import {ATTRIBUTE_TYPE_DESCRIPTOR, canSyncAttributeType, getAttributeTypeDescriptor, toServerFieldType} from '../attribute_type';
import {GLOBAL_ATTRIBUTES_LIST_ROUTE, GLOBAL_ATTRIBUTES_OBJECT_TYPE} from '../constants';
import {getSourceKind, getTypeIcon, getTypeLabel, isClassificationMarkingsField} from '../global_attributes_table';
import {findNameConflict, messages as nameConflictMessages, type NameConflict} from '../name_conflict';
import useAllowedResourceTypes from '../use_allowed_resource_types';
import {getFieldOwners, NO_OWNERS, useOwnersLabel} from '../use_owners_label';
import type {AttributeFieldType, AttributeTypeId, UpdateAttributeFieldPatch} from '../utils';
import {
    createAttributeField,
    createLinkedAttributeField,
    deleteAttributeField,
    deleteLinkedAttributeField,
    fetchAttributeField,
    fetchLinkedFieldsForTemplate,
    isAttributeFieldType,
    linkedFieldsByResourceType,
    listPropertyFields,
    patchLinkedAttributeField,
    syncUserAttributeFieldDelete,
    syncUserAttributeFieldUpsert,
    updateAttributeField,
} from '../utils';

import './attribute_details.scss';

const LIST_ROUTE = GLOBAL_ATTRIBUTES_LIST_ROUTE;

// Whether any option in `options` has a name equal to another option's name.
// Used defensively by canSave -- both options editors already block this
// interactively at add/rename time, but canSave re-derives it rather than
// trusting "the UI wouldn't let this happen" (matches the same defensive
// pattern used for rank validity below).
function hasDuplicateOptionNames(options: PropertyFieldOption[]): boolean {
    const seen = new Set<string>();
    for (const option of options) {
        if (seen.has(option.name)) {
            return true;
        }
        seen.add(option.name);
    }
    return false;
}

// Whether every option in `options` has a valid (positive, unique) rank.
// Only meaningful when the current type is 'rank'.
function hasValidRanks(options: PropertyFieldOption[]): boolean {
    return options.every((option, index) => isValidRank(option.rank) && !findRankCollision(options, option.rank as number, index));
}

function optionsFromField(field: PropertyField): PropertyFieldOption[] {
    const raw = field.attrs?.options;
    if (!Array.isArray(raw)) {
        return [];
    }
    return raw.filter((option): option is PropertyFieldOption => (
        typeof option === 'object' &&
        option !== null &&
        typeof (option as PropertyFieldOption).id === 'string' &&
        typeof (option as PropertyFieldOption).name === 'string'
    ));
}

// Mirrors CPA's own computeAutoFillSlug guard (user_properties_table.tsx): a
// derived slug of '_copy' (slugifyForCEL's empty-input sentinel) means there is
// nothing meaningful to show yet. Unlike CPA, a slug that resolves to a reserved
// word is NOT hidden here -- it is surfaced (with an inline error) rather than
// silently suppressed, so the admin can see and fix the collision before Save.
function computeAutoSlugDisplay(displayName: string): string | null {
    if (displayName.trim().length === 0) {
        return null;
    }
    const slug = slugifyForCEL(displayName);
    return slug === '_copy' ? null : slug;
}

// Same normalization load and create use: missing/empty display_name → ''.
function fieldDisplayName(field: PropertyField): string {
    return (typeof field.attrs?.display_name === 'string' ? field.attrs.display_name : '') || '';
}

type ErrorKind =
    | 'name_conflict' |
    'invalid_charset' |
    'reserved_word' |
    'limit_reached' |
    'invalid_options' |
    'generic' |
    'type_change_with_dependents' |
    'applies_to_failed' |
    'applies_to_remove_failed' |
    'applies_to_remove_partial_save' |
    'applies_to_partial_save' |
    'applies_to_config_save_failed' |
    'applies_to_display_name_save_failed' |
    'applies_to_rollback_failed' |
    'applies_to_name_conflict' |
    'applies_to_limit_reached';

// The error kinds whose banner copy interpolates {resources} -- share one
// flat formatMessage(errorMessages[errorKind], {resources}) call. Kept as a
// Set (module scope, not rebuilt per render) rather than a chain of ===
// comparisons at the call site.
const RESOURCE_INTERPOLATED_ERROR_KINDS = new Set<ErrorKind>([
    'applies_to_failed',
    'applies_to_remove_failed',
    'applies_to_remove_partial_save',
    'applies_to_partial_save',
    'applies_to_config_save_failed',
    'applies_to_display_name_save_failed',
]);

function errorKindFromError(error: unknown): ErrorKind {
    const serverErrorId = (error as ClientError | undefined)?.server_error_id;
    switch (serverErrorId) {
    case 'app.property_field.create.name_conflict.app_error':
    case 'app.property_field.update.name_conflict.app_error':
        return 'name_conflict';
    case 'model.cpa_field.name.invalid_charset.app_error':
        return 'invalid_charset';
    case 'model.cpa_field.name.reserved_word.app_error':
        return 'reserved_word';
    case 'app.property_field.create.limit_reached.app_error':
    case 'app.property_field.create.group_limit_reached.app_error':
        return 'limit_reached';
    case 'app.property_field.invalid_attrs.app_error':
        return 'invalid_options';
    case 'app.property_field.update.type_change_with_dependents.app_error':
        return 'type_change_with_dependents';
    default:
        return 'generic';
    }
}

// A linked-field creation failure gets its own mapping rather than reusing
// errorKindFromError's cases above: the server error ids are identical to the
// template's own name-conflict/limit-reached ids (both write into the same
// access_control group), but the actionable copy is different -- a linked
// 'user'-object-type field conflicts with a Custom Profile Attributes field,
// not with another Global Attribute. Returns null for anything else, so the
// caller falls back to the generic applies-to-failed message.
function appliesToErrorKindFromError(error: unknown): 'applies_to_name_conflict' | 'applies_to_limit_reached' | null {
    const serverErrorId = (error as ClientError | undefined)?.server_error_id;
    switch (serverErrorId) {
    case 'app.property_field.create.name_conflict.app_error':
        return 'applies_to_name_conflict';
    case 'app.property_field.create.limit_reached.app_error':
    case 'app.property_field.create.group_limit_reached.app_error':
        return 'applies_to_limit_reached';
    default:
        return null;
    }
}

// Formats a resource-type list for interpolation into an error banner, e.g.
// "Users, Channels" -- reuses the same labels the picker and rows already
// show, so the banner names resources the same way the UI does.
function resourceTypeListLabel(types: ResourceObjectType[], formatMessage: IntlShape['formatMessage']): string {
    return types.map((type) => formatMessage(resourceTypeLabels[type])).join(', ');
}

// The settled result of a handleSave attempt -- computed synchronously
// through the create-or-rollback sequence, then applied in one place
// (finalizeSave) behind the single isMountedRef check (see Decisions).
type SaveOutcome =
    | {success: true} |
    {success: false; errorKind: ErrorKind; serverErrorMessage: string | null; failedResourceTypes: ResourceObjectType[] | null};

// Rolls back everything created in a failed handleSave attempt: deletes every
// linked field already created so far (continuing through the full list even
// if one delete fails, so a single stuck field doesn't leave the rest
// orphaned too), then -- only if every one of those deletes succeeded --
// attempts to delete the template itself (deletion-order protection
// guarantees the template delete would 409 otherwise, so it's skipped rather
// than attempted for a failure this banner can't explain). Returns the
// settled failure outcome for handleSave to hand to finalizeSave.
async function rollbackLinkedFields(
    dispatch: Dispatch,
    createdLinkedFields: Array<{type: ResourceObjectType; field: PropertyField}>,
    templateFieldId: string,
    failedType: ResourceObjectType,
    creationError: unknown,
): Promise<SaveOutcome & {success: false}> {
    const survivingTypes: ResourceObjectType[] = [];
    for (const created of createdLinkedFields) {
        try {
            // eslint-disable-next-line no-await-in-loop
            await deleteLinkedAttributeField(created.type, created.field.id);

            // A user field was mirrored into the CPA slice as it was created;
            // drop it back out now that the rollback has removed it server-side.
            syncUserAttributeFieldDelete(dispatch, created.type, created.field.id);
        } catch {
            survivingTypes.push(created.type);
        }
    }

    if (survivingTypes.length > 0) {
        return {
            success: false,
            errorKind: 'applies_to_rollback_failed',
            serverErrorMessage: null,
            failedResourceTypes: survivingTypes,
        };
    }

    try {
        await deleteAttributeField(GLOBAL_ATTRIBUTES_OBJECT_TYPE, templateFieldId);
    } catch (deleteTemplateError) {
        // Linked fields are gone, but the template is still on the server.
        // A retry under the same Unique name will conflict with it, so this
        // is the same class of leftover as a linked-field that wouldn't
        // delete -- not "nothing was saved".
        // eslint-disable-next-line no-console
        console.error('Failed to delete orphaned attribute template after rolling back its linked fields', deleteTemplateError);
        return {
            success: false,
            errorKind: 'applies_to_rollback_failed',
            serverErrorMessage: null,
            failedResourceTypes: [],
        };
    }

    // CPA name-conflict / cap banners only make sense for a Users-linked
    // field (that namespace is shared with Custom Profile Attributes).
    // Channels/Posts use the generic applies-to-failed copy.
    const cpaErrorKind = failedType === 'user' ? appliesToErrorKindFromError(creationError) : null;
    return {
        success: false,
        errorKind: cpaErrorKind ?? 'applies_to_failed',
        serverErrorMessage: null,
        failedResourceTypes: [failedType],
    };
}

function nameErrorMessage(error: CPAFieldNameValidationError, formatMessage: IntlShape['formatMessage']): string {
    if (error.kind === 'reserved_word') {
        return formatMessage(nameErrorMessages.reservedWord, {word: error.word});
    }
    if (error.kind === 'too_long') {
        return formatMessage(nameErrorMessages.tooLong, {max: error.max});
    }
    return formatMessage(nameErrorMessages.invalidCharset);
}

// Returns the first reason whose condition is true, checked in the given order --
// shared by typeLockReason/nameLockReason below so a priority-ordered "which lock
// reason wins" chain exists in exactly one place, not once per lockable field.
function firstMatchingReason<T extends string>(...pairs: Array<[T, boolean]>): T | null {
    for (const [reason, matches] of pairs) {
        if (matches) {
            return reason;
        }
    }
    return null;
}

// The conflict catalogs are fetched once and held in a ref; only a change in
// the derived warning is committed to state. Putting the lists in state would
// re-render the whole form (open graph menus included) when the fetch lands,
// even if the typed name collides with nothing.
function sameNameConflict(a: NameConflict | undefined, b: NameConflict | undefined): boolean {
    if (a === b) {
        return true;
    }
    if (!a || !b) {
        return false;
    }
    return a.field.id === b.field.id && a.exact === b.exact && a.ownerTemplate?.id === b.ownerTemplate?.id;
}

type Props = {
    disabled?: boolean;
};

function AttributeDetails({disabled = false}: Props): JSX.Element {
    const dispatch = useDispatch();
    const {formatMessage} = useIntl();
    const {field_id: fieldId} = useParams<{field_id?: string}>();
    const isEditMode = Boolean(fieldId);

    const isGraphEnabled = useGetFeatureFlagValue('PropertyFieldGraph') === 'true';
    const ALL_TYPES = useMemo(
        () => Object.values(ATTRIBUTE_TYPE_DESCRIPTOR).filter((descriptor) => {
            if (descriptor.hidden) {
                return false;
            }
            return descriptor.id !== 'graph' || isGraphEnabled;
        }),
        [isGraphEnabled],
    );

    const [loading, setLoading] = useState(isEditMode);
    const [isDirty, setIsDirty] = useState(false);

    // Set once from the loaded field's attrs and never re-derived afterward -- a
    // plugin-owned field has nothing else on this page that can change it. Used
    // as the single source of truth for the plugin-owned lock (see
    // effectiveDisabled below) rather than recomputing getSourceKind(field) at
    // every render site.
    const [sourcePluginId, setSourcePluginId] = useState<string | undefined>(undefined);
    const isPluginOwned = Boolean(sourcePluginId);

    // Plugin-created attributes are excluded: they have their own, stricter lock and wording.
    const [owners, setOwners] = useState<PropertyFieldOwner[]>(NO_OWNERS);
    const isOwned = owners.length > 0 && !isPluginOwned;
    const ownersLabel = useOwnersLabel(owners);

    // Defaults to the template type because this page cannot create a
    // user/channel/post field; load() copies the fetched field's object_type
    // so Save PATCHes that type.
    const [objectType, setObjectType] = useState<string>(GLOBAL_ATTRIBUTES_OBJECT_TYPE);

    // Derived from objectType alone: create mode and templates both stay at
    // GLOBAL_ATTRIBUTES_OBJECT_TYPE, so they are false here without an extra
    // isEditMode check.
    const isNonTemplate = objectType !== GLOBAL_ATTRIBUTES_OBJECT_TYPE;

    // Only a post field's row stays fully locked; user and channel rows are editable.
    const standaloneRowLocked = isNonTemplate && objectType === 'post';

    // Substituted for the bare `disabled` prop everywhere else on this page --
    // one boolean, not a second parallel disabled path. Keeps the pre-existing
    // non-sysadmin `disabled` prop (schema-wired via isDisabled: it.not
    // (it.isSystemAdmin)) and the new plugin-owned lock behaving identically at
    // every call site.
    const effectiveDisabled = disabled || isPluginOwned;

    const [displayName, setDisplayName] = useState('');
    const [manualName, setManualName] = useState('');

    // Whether the admin currently has a committed manual override -- once true,
    // the Name stops re-deriving from Display name on further edits (persists
    // even if isEditingName below is later toggled back off). Can revert back
    // to false if Done is clicked while the field was left empty and there was
    // no prior commit -- see handleDoneClick. Also doubles as the pre-edit-
    // session snapshot handleDoneClick needs: it cannot change while
    // isEditingName is true (the Edit button that would flip it isn't even
    // rendered then), so reading it live at Done-click time is equivalent to,
    // and simpler than, capturing a separate ref at Edit-click time.
    const [isNameManuallyEdited, setIsNameManuallyEdited] = useState(false);

    // Whether the input is CURRENTLY shown/focused -- toggled independently by
    // the Edit/Done link.
    const [isEditingName, setIsEditingName] = useState(false);

    // The pre-edit-session Name, captured when "Edit" is clicked, so "Done" can
    // restore it if the field is left empty on a session that already had a
    // committed override (see handleDoneClick).
    const previousManualNameRef = useRef('');

    // Type and Options are independent for text/select/multiselect/rank — a
    // Text -> Select -> Text -> Select round-trip must restore names already
    // entered. Switching to or from graph clears options: DAG parents are
    // incompatible with a flat chip list. Phone/URL are text subtypes
    // (attrs.value_type) selected from this same Type menu.
    const [fieldType, setFieldType] = useState<AttributeTypeId>('text');
    const [options, setOptions] = useState<PropertyFieldOption[]>([]);

    // Independent of fieldType/options. AD/LDAP and SAML may be linked at once
    // (mirrors CPA's own dot-menu, which lets an admin link both on the same
    // field); OpenID Connect only on its own. See attribute_external_source.tsx.
    const [externalLinks, setExternalLinks] = useState<ExternalSourceLinks>(NO_EXTERNAL_SOURCE_LINKS);

    const allowedResourceTypes = useAllowedResourceTypes();

    // Pending Applies-to selection -- insertion order, not fixed Users/Channels/
    // Posts order (that fixed order only governs the picker's own offer list).
    const [appliesTo, setAppliesTo] = useState<ResourceObjectType[]>([]);

    // What the Channels row edits. Kept whether or not Channels is currently in
    // appliesTo, so removing the resource and adding it back does not silently
    // reset the configuration.
    const [channelResource, setChannelResource] = useState<ChannelResourceConfig>({...DEFAULT_CHANNEL_RESOURCE_CONFIG});

    // Loaded linked fields, keyed by resource type. Add/Remove only mutate
    // appliesTo; Save diffs against this snapshot. Updated in place after each
    // successful DELETE/POST so a retry does not repeat a completed step.
    const persistedLinkedFieldsRef = useRef<Partial<Record<ResourceObjectType, PropertyField>>>({});
    const originalNameRef = useRef('');

    // Template display_name as loaded (or last successfully saved). Used to decide
    // whether a linked field still shares that label and should be renamed with
    // the template on Save -- see shouldCascadeLinkedDisplayName below.
    const originalDisplayNameRef = useRef('');

    // Users row config -- Profile display / Who can set the value, both stored
    // as attrs.visibility/attrs.managed on the Users linked field, matching
    // CPA's own value domain exactly rather than the PSAv2 PermissionValues
    // field. Defaults match CPA's defaults for a brand-new field.
    const [userVisibility, setUserVisibility] = useState<FieldVisibility>('when_set');
    const [userManaged, setUserManaged] = useState<UserManagedValue>('');

    // Snapshot of what's actually persisted, to diff against at Save time --
    // populated in the load effect below. Not needed for a brand-new attribute
    // (both stay at the defaults above, matching what a new Users row would be
    // created with, so the diff below is trivially "unchanged").
    const originalUserVisibilityRef = useRef<FieldVisibility>('when_set');
    const originalUserManagedRef = useRef<UserManagedValue>('');

    // Template sync links as loaded (or last successfully saved). Sync reads the
    // linked Users field's own attrs, which are only copied from the template at
    // create time, so a change here must be patched onto that field on Save.
    const originalExternalLinksRef = useRef<ExternalSourceLinks>(NO_EXTERNAL_SOURCE_LINKS);

    // Compared against the live *server* field type at Save time to pick which
    // order DELETE/PATCH run in (see handleSave) -- the server rejects a type-
    // changing PATCH while linked fields of the old type still exist
    // (type_change_with_dependents), so that case must delete first, but doing
    // so unconditionally would delete linked values on a PATCH failure that has
    // nothing to do with type (e.g. a name conflict). Phone/URL keep type
    // 'text', so switching among text subtypes is not a type change.
    const originalFieldTypeRef = useRef<AttributeFieldType>('text');

    const [saving, setSaving] = useState(false);
    const [errorKind, setErrorKind] = useState<ErrorKind | null>(null);

    // Only populated for the applies_to_* error kinds -- interpolated into
    // their banners to name which resource(s) failed to create (generic
    // failure) or which survived a failed rollback (rollback-failed banner).
    const [failedResourceTypes, setFailedResourceTypes] = useState<ResourceObjectType[] | null>(null);

    // Only populated for 'name_conflict' -- the server's own message names the
    // specific conflicting field and the level it conflicts at (e.g. "system"),
    // which the canned copy below can't express since it doesn't know that detail.
    // Other kinds keep the canned copy: e.g. invalid_attrs's server message
    // ("Invalid property field attributes.") is less specific than ours.
    const [serverErrorMessage, setServerErrorMessage] = useState<string | null>(null);

    // Guards the post-await side effects in handleSave below against firing
    // after the component has unmounted (e.g. the admin confirmed "leave
    // without saving" via the BlockableLink prompt while a request was still
    // in flight) -- without this, a slow save that resolves after the user has
    // already navigated elsewhere would force-navigate them back to the list.
    const isMountedRef = useRef(true);
    useEffect(() => () => {
        isMountedRef.current = false;
    }, []);

    // The Source column's plugin-name resolution degrades to a raw plugin ID for
    // server-only plugins (no webapp bundle registered) until pluginStatuses has
    // been fetched -- mirrors global_attributes_table.tsx's own fetch-once
    // pattern (shared via usePluginInventoryLoaded) so a direct deep-link to
    // this page resolves the same name the listing would show. Also needed to
    // settle orphan detection below: an inventory that hasn't loaded yet reads
    // identically to "nothing installed" (see useIsFieldOrphaned's own doc
    // comment), so isOrphaned is only trusted once this is true.
    const pluginInventoryLoaded = usePluginInventoryLoaded(isPluginOwned || owners.some((owner) => owner.type === 'plugin'));

    // protected: true is safe to hardcode here (rather than threaded from the
    // loaded field) -- sourcePluginId is only ever set when getSourceKind(field)
    // === 'plugin', which itself requires attrs.protected === true on the real
    // field (see getSourceKind), so the invariant holds by construction.
    const isOrphanedRaw = useIsFieldOrphaned({attrs: {source_plugin_id: sourcePluginId, protected: true}});
    const isOrphaned = pluginInventoryLoaded && isOrphanedRaw;

    useEffect(() => {
        if (!fieldId) {
            return undefined;
        }

        let cancelled = false;

        const load = async () => {
            try {
                const field = await fetchAttributeField(fieldId, allowedResourceTypes);
                if (cancelled) {
                    return;
                }
                if (
                    !field ||
                    isClassificationMarkingsField(field, field.group_id) ||

                    // Plugin ownership is deliberately not part of this condition: a
                    // plugin-owned text/select/etc. field opens here read-only
                    // (effectiveDisabled); only an unrenderable type redirects.
                    !isAttributeFieldType(field.type)
                ) {
                    getHistory().push(LIST_ROUTE);
                    return;
                }

                setObjectType(field.object_type);

                // Field GET strips graph parents so a read-modify-write cannot
                // flatten the hierarchy. The options route is what reports them.
                const loadedOptions = field.type === 'graph' ? await pageAllAccessControlFieldOptions({id: field.id, object_type: field.object_type}) : optionsFromField(field);
                if (cancelled) {
                    return;
                }

                // Only a template has linked fields. An unlinked user/channel/post
                // field has none, so skip the fetch (there is also no Channels
                // child to parse config from).
                let linkedByType: Partial<Record<ResourceObjectType, PropertyField>> = {};
                if (field.object_type === GLOBAL_ATTRIBUTES_OBJECT_TYPE) {
                    const linkedFields = await fetchLinkedFieldsForTemplate(fieldId, allowedResourceTypes);
                    if (cancelled) {
                        return;
                    }
                    linkedByType = linkedFieldsByResourceType(linkedFields);
                }
                persistedLinkedFieldsRef.current = linkedByType;
                originalNameRef.current = field.name;
                originalFieldTypeRef.current = field.type;
                const loadedDisplayName = (field.attrs?.display_name as string | undefined) || '';
                originalDisplayNameRef.current = loadedDisplayName;

                setSourcePluginId(getSourceKind(field) === 'plugin' ? (field.attrs?.source_plugin_id as string | undefined) : undefined);
                setDisplayName(loadedDisplayName);
                setManualName(field.name);
                setIsNameManuallyEdited(true);
                setFieldType(getAttributeTypeDescriptor(field).id);
                setOptions(loadedOptions);
                const loadedLinks = externalSourceLinksFromField(field);
                originalExternalLinksRef.current = loadedLinks;
                setExternalLinks(loadedLinks);

                // A non-template field has no linked children: Applies-to is its
                // own object type, so there is no other resource to add or remove.
                setAppliesTo(
                    field.object_type === GLOBAL_ATTRIBUTES_OBJECT_TYPE ? ALL_RESOURCE_TYPES.filter((type) => Boolean(linkedByType[type])) : ALL_RESOURCE_TYPES.filter((type) => type === field.object_type),
                );

                // Seed from a Users field if one exists, not from create-vs-edit:
                // a template with no Users child still gets the same defaults as
                // create. A non-template user field is itself that Users field.
                const userField = field.object_type === 'user' ? field : linkedByType.user;
                setOwners(userField ? getFieldOwners(userField) : NO_OWNERS);
                const rawVisibility = userField?.attrs?.visibility;
                const loadedVisibility: FieldVisibility = rawVisibility === 'always' || rawVisibility === 'when_set' || rawVisibility === 'hidden' ? rawVisibility : 'when_set';
                const loadedManaged: UserManagedValue = userField?.attrs?.managed === 'admin' ? 'admin' : '';
                setUserVisibility(loadedVisibility);
                setUserManaged(loadedManaged);
                originalUserVisibilityRef.current = loadedVisibility;
                originalUserManagedRef.current = loadedManaged;

                // A non-template channel field is itself the channel field, so its
                // row settings parse from it directly; a template seeds them from
                // its linked channel child instead. Save writes every channel setting,
                // so without this seed a rename would overwrite them with defaults.
                const channelConfigSource = field.object_type === 'channel' ? field : linkedByType.channel;
                if (channelConfigSource) {
                    setChannelResource(parseChannelFieldConfig(channelConfigSource));
                }
                setLoading(false);
            } catch {
                if (!cancelled) {
                    getHistory().push(LIST_ROUTE);
                }
            }
        };

        load();

        return () => {
            cancelled = true;
        };
    }, [fieldId, allowedResourceTypes]);

    const autoSlugDisplay = useMemo(() => computeAutoSlugDisplay(displayName), [displayName]);
    const currentName = (isEditingName || isNameManuallyEdited) ? manualName : (autoSlugDisplay ?? '');
    const nameUnchanged = isEditMode && currentName === originalNameRef.current;
    const nameValidationError = (!nameUnchanged && currentName) ? validateCPAFieldName(currentName) : null;

    // Kept as a primitive rather than passing nameValidationError itself into
    // handleDoneClick's dep array -- the error object is rebuilt on every
    // render while invalid, which would churn the callback identity needlessly.
    const hasNameError = Boolean(nameValidationError);

    // Live user fields (what a CEL rule can collide with) and templates (to name
    // the owner of a colliding linked field). Loaded independently of the field
    // being edited, because the create form needs them too. A failure here only
    // costs the warning: the save path still reports the server's own 409.
    const conflictCatalogRef = useRef<{userFields: PropertyField[]; templates: PropertyField[]}>({
        userFields: [],
        templates: [],
    });
    const currentNameRef = useRef(currentName);
    currentNameRef.current = currentName;
    const fieldIdRef = useRef(fieldId);
    fieldIdRef.current = fieldId;
    const [nameConflict, setNameConflict] = useState<NameConflict | undefined>();

    useEffect(() => {
        let cancelled = false;

        (async () => {
            try {
                const [userFields, templates] = await Promise.all([
                    listPropertyFields('user'),
                    listPropertyFields(GLOBAL_ATTRIBUTES_OBJECT_TYPE),
                ]);
                if (cancelled) {
                    return;
                }
                conflictCatalogRef.current = {userFields, templates};
                const name = currentNameRef.current;
                setNameConflict((prev) => {
                    const next = name ? findNameConflict(name, userFields, templates, fieldIdRef.current) : undefined;
                    return sameNameConflict(prev, next) ? prev : next;
                });
            } catch (error) {
                console.error('AttributeDetails-load-name-conflicts: ', error); // eslint-disable-line no-console
            }
        })();

        return () => {
            cancelled = true;
        };
    }, []);

    useEffect(() => {
        const {userFields, templates} = conflictCatalogRef.current;
        setNameConflict((prev) => {
            const next = currentName ? findNameConflict(currentName, userFields, templates, fieldId) : undefined;
            return sameNameConflict(prev, next) ? prev : next;
        });
    }, [currentName, fieldId]);

    // Only an exact match would collide on the server, which compares names
    // case-sensitively. Applying to Users under that name creates a second user
    // field with it, which the server rejects with a 409.
    const usersBlockedByNameConflict = Boolean(nameConflict?.exact);

    // Display name was typed but auto-derivation produced nothing usable (e.g.
    // a non-Latin-script or symbol-only Display name normalizes to slugifyForCEL's
    // empty-input sentinel) -- distinct from nameValidationError (which requires
    // a non-empty currentName) and from the untouched-form empty state, so it
    // needs its own explanation rather than a silently-disabled Save button.
    const autoSlugCollapsedToEmpty = !isEditingName && !isNameManuallyEdited &&
        Boolean(displayName.trim()) && autoSlugDisplay === null;

    // Done refuses to commit an invalid Name (see handleDoneClick). Surfaced as
    // aria-disabled rather than the disabled attribute so the button stays
    // focusable -- a keyboard user who tabs to it still lands on it and hears
    // the reason via aria-describedby, instead of it silently vanishing from
    // the tab order with no explanation.
    const isDoneBlocked = isEditingName && hasNameError;

    const isServerNameError = errorKind === 'name_conflict' || errorKind === 'reserved_word' || errorKind === 'invalid_charset';
    let nameDescribedBy: string | undefined;
    if (nameValidationError) {
        nameDescribedBy = 'attribute-unique-name-error';
    } else if (isServerNameError) {
        nameDescribedBy = 'attribute-save-error';
    }

    // Plugin ownership wins over the post-field lock: it disables the whole page,
    // so it is the more specific reason to show.
    let appliesToLockedTooltip: string | undefined;
    if (isPluginOwned) {
        appliesToLockedTooltip = formatMessage(isOrphaned ? messages.appliesToLockedPluginOrphanedTooltip : messages.appliesToLockedPluginTooltip);
    } else if (standaloneRowLocked) {
        appliesToLockedTooltip = formatMessage(messages.appliesToLockedSingleResourceTooltip);
    }

    const ownedLockedTooltip = isOwned ? formatMessage(messages.ownedLockedTooltip, {owners: ownersLabel}) : undefined;

    // An uninstalled plugin can't manage the Users field, so once every owner is one,
    // Remove unlocks, matching the Delete action in the attributes table.
    const installedPluginIds = useInstalledPluginIds();
    const ownersGone = pluginInventoryLoaded && allOwnersAreUninstalledPlugins(owners, installedPluginIds);
    const userRemoveLocked = isOwned && !ownersGone;
    const ownedRemoveLockedTooltip = userRemoveLocked ? ownedLockedTooltip : undefined;

    let removeLockedTooltips: Partial<Record<ResourceObjectType, ReactNode>> | undefined;
    if (isNonTemplate) {
        removeLockedTooltips = {[objectType]: ownedRemoveLockedTooltip ?? formatMessage(messages.appliesToLockedSingleResourceTooltip)};
    } else if (ownedRemoveLockedTooltip) {
        removeLockedTooltips = {user: ownedRemoveLockedTooltip};
    }

    const markDirty = useCallback(() => {
        setIsDirty(true);
        dispatch(setNavigationBlocked(true));
        setErrorKind(null);
        setServerErrorMessage(null);
        setFailedResourceTypes(null);
    }, [dispatch]);

    // Defensive dedupe, independent of the picker's own filtering (which
    // already only offers not-yet-selected types) -- a genuine no-op if the
    // type is somehow already present, so it doesn't mark the page dirty or
    // clear an error banner for nothing.
    const handleAdd = useCallback((type: ResourceObjectType) => {
        if (appliesTo.includes(type)) {
            return;
        }
        setAppliesTo((prev) => [...prev, type]);
        markDirty();
    }, [appliesTo, markDirty]);

    const handleRemove = useCallback((type: ResourceObjectType) => {
        setAppliesTo((prev) => prev.filter((existing) => existing !== type));
        markDirty();
    }, [markDirty]);

    // Users row config -- no-op-guarded and markDirty()-wrapped, mirroring
    // handleTypeChange/handleLink above.
    const handleUserVisibilityChange = useCallback((visibility: FieldVisibility) => {
        if (visibility === userVisibility) {
            return;
        }
        setUserVisibility(visibility);
        markDirty();
    }, [userVisibility, markDirty]);

    const handleUserManagedChange = useCallback((managed: UserManagedValue) => {
        if (managed === userManaged) {
            return;
        }
        setUserManaged(managed);
        markDirty();
    }, [userManaged, markDirty]);

    // Moves focus to the header Add-resource trigger after a pre-save removal
    // -- via useEffect, not directly inside handleRemove, since the trigger may
    // have just been re-rendered into existence this same update (e.g. removing
    // the 3rd of 3 selected types un-hides both triggers), and a synchronous
    // focus() call in the handler would run before that re-render commits.
    // Mirrors the sibling external-source picker's own prevCountRef pattern
    // (attribute_external_source.tsx).
    //
    // Also handles the mirror-image case on add: picking the 3rd (last)
    // resource type unmounts BOTH "Add resource" triggers in this same
    // render, including whichever one the admin just clicked -- MUI's
    // Popover restores focus to that trigger once its close transition
    // finishes, but the trigger is gone from the DOM by then, so focus
    // silently drops to <body> with no fix here. Landing on the
    // just-added row's own toggle keeps focus on a real, newly-rendered
    // element instead. Looked up by data-testid (not id) since the row
    // doesn't otherwise need a stable element id.
    //
    // Create starts hydrated. Edit skips the first populated commit so
    // loading all three resource types does not look like "just added the
    // last type" and steal autoFocus from Display name.
    const prevAppliesToLengthRef = useRef(appliesTo.length);
    const appliesToHydratedRef = useRef(!isEditMode);
    useEffect(() => {
        if (loading) {
            prevAppliesToLengthRef.current = appliesTo.length;
            return;
        }
        if (!appliesToHydratedRef.current) {
            appliesToHydratedRef.current = true;
            prevAppliesToLengthRef.current = appliesTo.length;
            return;
        }
        const prevLength = prevAppliesToLengthRef.current;
        if (appliesTo.length < prevLength) {
            document.getElementById(ATTRIBUTE_APPLIES_TO_ADD_HEADER_TRIGGER_ID)?.focus();
        } else if (appliesTo.length > prevLength && appliesTo.length === ALL_RESOURCE_TYPES.length) {
            const lastAddedType = appliesTo[appliesTo.length - 1];
            document.querySelector<HTMLElement>(`[data-testid="attributeAppliesToRow-${lastAddedType}-toggle"]`)?.focus();
        }
        prevAppliesToLengthRef.current = appliesTo.length;
    }, [appliesTo, loading]);

    // Switching into Rank from any other type (re)assigns rank = index + 1 to
    // every current option, overwriting any stale rank values -- mirrors CPA's
    // own handleTypeChange (user_properties_type_menu.tsx) exactly, and is
    // idempotent. Switching away from Rank leaves rank values sitting inert in
    // state (not stripped) -- harmless, since Select/Multiselect ignore it.
    const handleTypeChange = useCallback((newType: AttributeTypeId) => {
        if (newType === fieldType) {
            return;
        }
        markDirty();

        // Must run before rank reassignment so graph → rank does not rank leftover DAG options.
        if ((newType === 'graph') !== (fieldType === 'graph')) {
            setOptions([]);
        }
        if (newType === 'rank' && fieldType !== 'rank') {
            setOptions((prevOptions) => (prevOptions.length > 0 ? prevOptions.map((option, index) => ({...option, rank: index + 1})) : prevOptions));
        }

        // The server strips every sync link (attrs.ldap, attrs.saml, attrs.openid)
        // from any field whose type an identity source cannot populate
        // (AccessControlAttributeValidationHook) -- clear them proactively here
        // so the UI never shows a link that's about to silently vanish.
        if (!canSyncAttributeType(newType)) {
            setExternalLinks(NO_EXTERNAL_SOURCE_LINKS);
        }

        // Raise-only and lower-only compare ranks, so they cannot survive a move off
        // Rank: left in place they would save a policy the server can never evaluate.
        if (newType !== 'rank') {
            setChannelResource((prev) => (isOrderedChangePolicy(prev.changePolicy) ? {...prev, changePolicy: 'any'} : prev));
        }
        setFieldType(newType);
    }, [fieldType, markDirty]);

    const handleOptionsChange = useCallback((newOptions: PropertyFieldOption[]) => {
        markDirty();
        setOptions(newOptions);
    }, [markDirty]);

    // Shared by both the "add" trigger (value: newly typed attribute name)
    // and a chip's remove action (value: ''). A no-op (unchanged value) skips
    // both the state update and markDirty(), mirroring handleTypeChange's own
    // no-op guard above.
    const handleLink = useCallback((source: ExternalSource, rawValue: string) => {
        const value = rawValue.trim();
        if (value === externalLinks[source]) {
            return;
        }
        markDirty();
        setExternalLinks((prev) => ({...prev, [source]: value}));
        if (value && !canSyncAttributeType(fieldType)) {
            setFieldType('text');
        }
    }, [externalLinks, fieldType, markDirty]);

    const handleDisplayNameChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        setDisplayName(e.target.value);
        markDirty();
    }, [markDirty]);

    const handleEditClick = useCallback(() => {
        previousManualNameRef.current = isNameManuallyEdited ? manualName : '';
        setManualName((current) => (isNameManuallyEdited ? current : (autoSlugDisplay ?? '')));
        setIsEditingName(true);
    }, [autoSlugDisplay, isNameManuallyEdited, manualName]);

    // Done, Enter, and blur (clicking away) share this path. Inert while the
    // typed Name is invalid, so a reserved word or bad charset can never be
    // committed -- the admin must fix it first. This is not a focus trap:
    // clearing the field makes Done live again (an empty name has no
    // validation error, and Done then applies the revert rules below), and
    // Escape still discards the whole edit outright.
    const handleDoneClick = useCallback(() => {
        if (hasNameError) {
            return;
        }
        if (manualName === '') {
            if (isNameManuallyEdited) {
                setManualName(previousManualNameRef.current);
            } else {
                setIsNameManuallyEdited(false);
            }
        } else {
            // In create mode, a committed value that happens to match the
            // current auto-slug keeps live derivation on (see the "no-op
            // keeps derivation" tests). In edit mode this must never flip
            // isNameManuallyEdited to false: the loaded Name is a persisted
            // identifier, not a derivation, and Done on an unchanged/no-op
            // edit session must not silently unpin it just because it
            // happens to coincide with what the current Display name would
            // slugify to (see the "does not re-slug on edit" regression).
            setIsNameManuallyEdited(isEditMode || manualName !== (autoSlugDisplay ?? ''));
        }
        setIsEditingName(false);
    }, [hasNameError, manualName, isNameManuallyEdited, autoSlugDisplay, isEditMode]);

    const handleNameChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        setManualName(filterCELIdentifier(e.target.value));
        markDirty();
    }, [markDirty]);

    // Escape discards whatever was typed this session and exits edit mode,
    // as distinct from Done (which commits it -- see handleDoneClick).
    // Deliberately NOT gated on hasNameError: Escape is the unconditional way
    // out of an edit session, including one left in an invalid state.
    const handleCancelEdit = useCallback(() => {
        setManualName(previousManualNameRef.current);
        setIsEditingName(false);
    }, []);

    const handleNameKeyDown = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            handleDoneClick();
        } else if (e.key === 'Escape') {
            e.preventDefault();
            handleCancelEdit();
        }
    }, [handleDoneClick, handleCancelEdit]);

    const hasExternalSource = hasExternalSourceLink(externalLinks);
    const linkedExternalSource = resolveExternalSource(externalLinks);
    const managedByExternalSource = isPluginOwned ? undefined : linkedExternalSource;

    // External source (LDAP/SAML) is a user-identity concept. A template's linked
    // children can include a user field, so every template keeps the editor
    // regardless of which resources it currently applies to; among non-template
    // fields only a user field qualifies.
    const showsExternalSource = !isNonTemplate || objectType === 'user';
    const typeLockedByAppliesTo = isEditMode && appliesTo.length > 0 && !isNonTemplate;

    // The server refuses to convert a field to or from the graph type
    // (app.property_field.update.graph_type_change.app_error) -- a flat option
    // list and a hierarchy do not describe the same values, so neither
    // conversion has an answer. A graph field therefore has nowhere to go and its
    // Type is locked outright; every other field keeps its menu but no longer
    // offers Hierarchical (see selectableTypes below).
    const typeLockedByGraph = isEditMode && originalFieldTypeRef.current === 'graph';
    const typeLocked = hasExternalSource || typeLockedByAppliesTo || isPluginOwned || isOwned || typeLockedByGraph;
    const serverFieldType = toServerFieldType(fieldType);
    const typeChanged = isEditMode && serverFieldType !== originalFieldTypeRef.current;
    const typeSupportsOptions = supportsOptions({type: serverFieldType});

    // A synced select or multiselect gets its options from the identity source:
    // the sync adds every value it is sent and removes those no user holds any
    // more, and the server refuses anyone else adding, renaming or removing one.
    const optionsOwnedBySync = typeSupportsOptions && hasExternalSource;

    // The sync provisions those options from the values it writes to users, so
    // a synced select or multiselect that applies to no Users would be left
    // with a list nobody can add to.
    const appliesToUsers = isNonTemplate ? objectType === 'user' : appliesTo.includes('user');
    const wasSyncedWhenLoaded = hasExternalSourceLink(originalExternalLinksRef.current);

    // Unique name is the identifier policies and integrations bind to, and the
    // server does not copy it onto linked fields. Renaming while any resource
    // application is persisted would leave those fields on the old identifier,
    // so the rename is locked until they are removed and saved. Keyed off the
    // persisted snapshot, not appliesTo, so a pending local remove doesn't
    // unlock a rename the dependents wouldn't receive.
    const nameLockedByAppliesTo = isEditMode && Object.keys(persistedLinkedFieldsRef.current).length > 0;
    const nameLocked = nameLockedByAppliesTo || isPluginOwned || isOwned;

    // Defensive re-check, not the primary guard: both options editors already
    // block duplicate names and invalid/duplicate ranks interactively at
    // add/rename time (see attribute_options_values.tsx /
    // attribute_options_rank_values.tsx), so 'duplicate'/'invalid_rank' should
    // never actually be the reason Save is disabled in normal use -- computed
    // here (and memoized against [typeSupportsOptions, options, fieldType], so
    // an unrelated re-render like a display-name keystroke doesn't re-walk the
    // options array) so there's still a specific inline reason to show if they
    // ever are.
    const optionsIssue = useMemo(() => {
        if (!typeSupportsOptions) {
            return null;
        }
        if (optionsOwnedBySync && !appliesToUsers) {
            return 'needs_users' as const;
        }
        if (options.length === 0) {
            return optionsOwnedBySync ? null : 'required' as const;
        }
        if (hasDuplicateOptionNames(options)) {
            return 'duplicate' as const;
        }
        if (fieldType === 'rank' && !hasValidRanks(options)) {
            return 'invalid_rank' as const;
        }
        return null;
    }, [typeSupportsOptions, optionsOwnedBySync, appliesToUsers, options, fieldType]);

    const isHierarchical = supportsHierarchy({type: serverFieldType});
    const graphOptionsValid = useMemo(() => {
        if (!isHierarchical) {
            return true;
        }
        return options.length > 0 &&
            !hasBlankTrimmedOptionName(options) &&
            !hasCaseInsensitiveDuplicateNames(options);
    }, [isHierarchical, options]);

    // Applying to Users under a name a user field already holds is the one
    // combination the server refuses. Saving it would create the template, fail
    // on the linked field, and roll the template back again, so Save waits for
    // Users to be dropped or the name changed. Every other resource still saves.
    const usersSaveBlocked = usersBlockedByNameConflict && appliesTo.includes('user');
    const canSave = !effectiveDisabled && Boolean(displayName.trim()) && Boolean(currentName) && !nameValidationError && !usersSaveBlocked && !saving && optionsIssue === null && graphOptionsValid && (!isEditMode || isDirty);

    const confirmRemoveAppliesTo = useConfirmRemoveAppliesTo();

    // Applies the fully-settled outcome of a save attempt -- the ONLY place in
    // handleSave that reads isMountedRef, checked once after the entire
    // create-or-rollback sequence below has finished. Inserting a
    // mounted-check earlier, e.g. right after the template create, would skip
    // the linked-field loop and its rollback entirely if the admin navigated
    // away mid-save, leaving an orphaned template with no cleanup attempted.
    const finalizeSave = useCallback((outcome: SaveOutcome) => {
        if (!isMountedRef.current) {
            return;
        }
        if (outcome.success) {
            dispatch(setNavigationBlocked(false));
            getHistory().push(LIST_ROUTE);
            return;
        }
        setErrorKind(outcome.errorKind);
        setServerErrorMessage(outcome.serverErrorMessage);
        setFailedResourceTypes(outcome.failedResourceTypes);
        setSaving(false);
    }, [dispatch]);

    const handleSave = useCallback(async () => {
        if (!canSave) {
            return;
        }
        setSaving(true);
        setErrorKind(null);
        setServerErrorMessage(null);
        setFailedResourceTypes(null);

        // Bundled into the Users linked field's create request below (never a
        // bare create followed by an immediate patch) -- undefined for Posts,
        // which has no config yet.
        const userConfigAttrs = {visibility: userVisibility, managed: userManaged};

        // The field's actual write-permission tier, kept in sync with managed --
        // sent as permission_values (top-level, not part of attrs) so the value
        // CPA's own attrs.managed toggle promises is the value that's actually
        // enforced.
        const userConfigPermissionValues: PropertyPermissionLevel = userManaged === 'admin' ? 'sysadmin' : 'member';

        // Dispatches each resource type's own config onto its create request --
        // Users' Profile display/Who can set the value, or Channels' own
        // required/change-policy attrs and pinned permission_values. Posts has
        // no config panel yet, so it always gets undefined (server defaults).
        const newResourceAttrsFor = (type: ResourceObjectType): Record<string, unknown> | undefined => {
            switch (type) {
            case 'user':
                return userConfigAttrs;
            case 'channel':
                return buildChannelFieldAttrs(channelResource);
            default:
                return undefined;
            }
        };
        const newResourcePermissionValuesFor = (type: ResourceObjectType): PropertyPermissionLevel | undefined => {
            switch (type) {
            case 'user':
                return userConfigPermissionValues;
            case 'channel':
                return CHANNEL_VALUE_SETTER;
            default:
                return undefined;
            }
        };

        if (isEditMode && fieldId) {
            const patch: UpdateAttributeFieldPatch = {
                ...(nameUnchanged ? {} : {name: currentName}),
                type: fieldType,
                displayName,

                // Sending the list back would race the sync: an option it added
                // since this page loaded would read as removed, which the server
                // refuses. Linking a source for the first time still saves the
                // options the admin seeded.
                ...(optionsOwnedBySync && wasSyncedWhenLoaded ? {} : {options}),
                externalLinks,
            };

            // An unlinked user/channel/post field owns its own row -- one PATCH
            // to that object type, skipping the template create-plus-link path
            // below entirely.
            if (objectType !== GLOBAL_ATTRIBUTES_OBJECT_TYPE) {
                const userConfigChanged = userVisibility !== originalUserVisibilityRef.current || userManaged !== originalUserManagedRef.current;
                if (objectType === 'user' && userConfigChanged) {
                    patch.resourceAttrs = userConfigAttrs;
                    patch.permissionValues = userConfigPermissionValues;
                } else if (objectType === 'channel') {
                    patch.resourceAttrs = buildChannelFieldPatch(channelResource).attrs;
                }
                try {
                    const updatedField = await updateAttributeField(objectType, fieldId, patch);
                    syncUserAttributeFieldUpsert(dispatch, updatedField, false);
                } catch (error) {
                    finalizeSave({
                        success: false,
                        errorKind: errorKindFromError(error),
                        serverErrorMessage: (error as ClientError | undefined)?.message ?? null,
                        failedResourceTypes: null,
                    });
                    return;
                }
                finalizeSave({success: true});
                return;
            }

            const persisted = persistedLinkedFieldsRef.current;
            const toDelete = (Object.keys(persisted) as ResourceObjectType[]).filter((type) => !appliesTo.includes(type) && !(userRemoveLocked && type === 'user'));
            const toCreate = appliesTo.filter((type) => !persisted[type]);

            // Removing a resource deletes every value stored under its linked
            // field -- unbounded and irreversible -- so confirm before doing
            // anything else. Declining aborts the whole save; nothing has been
            // deleted or patched yet at this point.
            if (toDelete.length > 0 && !(await confirmRemoveAppliesTo(toDelete))) {
                setSaving(false);
                return;
            }

            // Deletes toDelete's linked fields, stopping (and reporting) at the
            // first failure. Called from one of two positions below depending on
            // typeChanged, never both -- errorKind differs by position because
            // the two failures are not the same: before PATCH, nothing else has
            // been saved yet; after PATCH has already succeeded, the template
            // update is not lost, only the removal is.
            const deleteRemovedLinkedFields = async (errorKind: 'applies_to_remove_failed' | 'applies_to_remove_partial_save'): Promise<boolean> => {
                for (const type of toDelete) {
                    const existing = persisted[type];
                    if (!existing) {
                        continue;
                    }
                    try {
                        // eslint-disable-next-line no-await-in-loop
                        await deleteLinkedAttributeField(type, existing.id);
                        syncUserAttributeFieldDelete(dispatch, type, existing.id);
                        delete persistedLinkedFieldsRef.current[type];
                    } catch {
                        finalizeSave({
                            success: false,
                            errorKind,
                            serverErrorMessage: null,
                            failedResourceTypes: [type],
                        });
                        return false;
                    }
                }
                return true;
            };

            // A type-changing PATCH 409s server-side while linked fields of the
            // old type still exist (type_change_with_dependents), so that case
            // requires DELETE before PATCH. But applying that order
            // unconditionally means a PATCH failure that has nothing to do with
            // type (e.g. a name conflict) would still have already deleted every
            // linked value -- irreversibly, for a save that just failed. So when
            // the type isn't changing, PATCH runs first and the delete only
            // happens once it's known to succeed.
            if (typeChanged && !(await deleteRemovedLinkedFields('applies_to_remove_failed'))) {
                return;
            }

            try {
                await updateAttributeField(GLOBAL_ATTRIBUTES_OBJECT_TYPE, fieldId, patch);
            } catch (error) {
                finalizeSave({
                    success: false,
                    errorKind: errorKindFromError(error),
                    serverErrorMessage: (error as ClientError | undefined)?.message ?? null,
                    failedResourceTypes: null,
                });
                return;
            }

            // updatePropertyField returns only the template. The server still
            // rewrites linked fields in that same write (options, type), but
            // the originating tab never receives property_field_* for it.
            // Newly created Users rows and an explicit Users-config PATCH are
            // synced below; an already-persisted Users row that stays applied
            // is not, so read it back into the CPA slice. A fetch miss must
            // not fail the save -- the template update has already landed.
            if (appliesTo.includes('user') && !toCreate.includes('user') && persistedLinkedFieldsRef.current.user) {
                try {
                    const linkedFields = await fetchLinkedFieldsForTemplate(fieldId, allowedResourceTypes);
                    const userField = linkedFieldsByResourceType(linkedFields).user;
                    if (userField) {
                        persistedLinkedFieldsRef.current.user = userField;
                        syncUserAttributeFieldUpsert(dispatch, userField, false);
                    }
                } catch {
                    // Best-effort originating-tab cache refresh only.
                }
            }

            if (!typeChanged && !(await deleteRemovedLinkedFields('applies_to_remove_partial_save'))) {
                return;
            }

            for (const type of toCreate) {
                try {
                    // eslint-disable-next-line no-await-in-loop
                    const linkedField = await createLinkedAttributeField(
                        type,
                        currentName,
                        fieldType,
                        displayName,
                        fieldId,
                        newResourceAttrsFor(type),
                        newResourcePermissionValuesFor(type),
                    );
                    persistedLinkedFieldsRef.current[type] = linkedField;
                    syncUserAttributeFieldUpsert(dispatch, linkedField, true);
                    if (type === 'user') {
                        originalUserVisibilityRef.current = userVisibility;
                        originalUserManagedRef.current = userManaged;
                    }
                } catch (error) {
                    const cpaErrorKind = type === 'user' ? appliesToErrorKindFromError(error) : null;
                    finalizeSave({
                        success: false,
                        errorKind: cpaErrorKind ?? 'applies_to_partial_save',
                        serverErrorMessage: null,
                        failedResourceTypes: [type],
                    });
                    return;
                }
            }

            // Config-only patch for a Users row that's already persisted this
            // session. Guarded by all three of: `'user' ∈ appliesTo` (excludes a
            // row in `toDelete` -- already persisted and not in `toCreate` is also
            // true for a row being deleted, so this check must not rely on file
            // ordering relative to the delete loop above), already persisted, and
            // not in `toCreate` (that row's create call above already carries the
            // current values).
            //
            // Also PATCHes display_name when the linked field still shares the
            // template's previous label -- Channel Info / profile UIs read the
            // linked field's display_name, and the server does not cascade a
            // template rename onto dependents.
            const previousDisplayName = originalDisplayNameRef.current;
            const nextDisplayName = displayName.trim();
            const displayNameChanged = nextDisplayName !== previousDisplayName;
            const displayNamePatchAttrs = {display_name: nextDisplayName || undefined};
            const shouldCascadeLinkedDisplayName = (linked: PropertyField) => (
                displayNameChanged && fieldDisplayName(linked) === previousDisplayName
            );

            const userIsUpdateCandidate = appliesTo.includes('user') && !toCreate.includes('user') && Boolean(persistedLinkedFieldsRef.current.user);
            if (userIsUpdateCandidate) {
                const visibilityChanged = userVisibility !== originalUserVisibilityRef.current;
                const managedChanged = userManaged !== originalUserManagedRef.current;
                const changedSyncSources = ALL_EXTERNAL_SOURCES.filter((source) => externalLinks[source] !== originalExternalLinksRef.current[source]);
                const syncSourceChanged = changedSyncSources.length > 0;
                const existingUserField = persistedLinkedFieldsRef.current.user;
                const cascadeDisplayName = existingUserField ? shouldCascadeLinkedDisplayName(existingUserField) : false;
                if (existingUserField && (visibilityChanged || managedChanged || cascadeDisplayName || syncSourceChanged)) {
                    try {
                        const updatedUserField = await patchLinkedAttributeField(
                            'user',
                            existingUserField.id,
                            {
                                ...(visibilityChanged || managedChanged ? userConfigAttrs : {}),
                                ...(cascadeDisplayName ? displayNamePatchAttrs : {}),

                                // null unlinks, matching updateAttributeField's
                                // template patch — sync reads these attrs on the
                                // Users field, not the template.
                                ...Object.fromEntries(changedSyncSources.map((source) => [source, externalLinks[source] || null])),
                            },
                            (visibilityChanged || managedChanged) ? userConfigPermissionValues : undefined,
                        );
                        persistedLinkedFieldsRef.current.user = updatedUserField;
                        syncUserAttributeFieldUpsert(dispatch, updatedUserField, false);
                        if (visibilityChanged || managedChanged) {
                            originalUserVisibilityRef.current = userVisibility;
                            originalUserManagedRef.current = userManaged;
                        }
                        if (syncSourceChanged) {
                            originalExternalLinksRef.current = externalLinks;
                        }
                    } catch {
                        // Distinct from applies_to_partial_save -- the Users row is
                        // already linked and persisted here (userIsUpdateCandidate
                        // requires it), so "couldn't be applied to Users" would tell
                        // the admin the linkage itself failed and risk them removing
                        // and re-adding the row (a destructive, confirmation-gated
                        // action) when a simple Save retry is all that's needed.
                        // Display-name-only failures get their own copy: the config
                        // banner names Profile display / Who can set the value, which
                        // is wrong when neither control was part of this PATCH.
                        const configChanged = visibilityChanged || managedChanged || syncSourceChanged;
                        finalizeSave({
                            success: false,
                            errorKind: configChanged ? 'applies_to_config_save_failed' : 'applies_to_display_name_save_failed',
                            serverErrorMessage: null,
                            failedResourceTypes: ['user'],
                        });
                        return;
                    }
                }
            }

            // An existing Channels row's own settings (required, change policy,
            // display locations) are not part of the template patch above, so a
            // resource that was already applied still needs its own PATCH to
            // pick up an edit to those settings. Fold display_name into the same
            // PATCH when the linked label still matches the template's previous one.
            const persistedChannelField = persistedLinkedFieldsRef.current.channel;
            if (persistedChannelField && appliesTo.includes('channel') && !toCreate.includes('channel')) {
                try {
                    const patchedChannelField = await patchLinkedAttributeField(
                        'channel',
                        persistedChannelField.id,
                        {
                            ...buildChannelFieldPatch(channelResource).attrs,
                            ...(shouldCascadeLinkedDisplayName(persistedChannelField) ? displayNamePatchAttrs : {}),
                        },
                    );
                    persistedLinkedFieldsRef.current.channel = patchedChannelField;
                } catch {
                    finalizeSave({
                        success: false,
                        errorKind: 'applies_to_partial_save',
                        serverErrorMessage: null,
                        failedResourceTypes: ['channel'],
                    });
                    return;
                }
            }

            // Posts has no config panel yet; only cascade display_name when it still
            // matches the template's previous label.
            const persistedPostField = persistedLinkedFieldsRef.current.post;
            if (persistedPostField && appliesTo.includes('post') && !toCreate.includes('post') && shouldCascadeLinkedDisplayName(persistedPostField)) {
                try {
                    const patchedPostField = await patchLinkedAttributeField(
                        'post',
                        persistedPostField.id,
                        displayNamePatchAttrs,
                    );
                    persistedLinkedFieldsRef.current.post = patchedPostField;
                } catch {
                    finalizeSave({
                        success: false,
                        errorKind: 'applies_to_partial_save',
                        serverErrorMessage: null,
                        failedResourceTypes: ['post'],
                    });
                    return;
                }
            }

            originalDisplayNameRef.current = nextDisplayName;
            originalExternalLinksRef.current = externalLinks;
            finalizeSave({success: true});
            return;
        }

        let templateField: PropertyField;
        try {
            templateField = await createAttributeField(displayName, currentName, fieldType, options, externalLinks);
        } catch (error) {
            finalizeSave({
                success: false,
                errorKind: errorKindFromError(error),
                serverErrorMessage: (error as ClientError | undefined)?.message ?? null,
                failedResourceTypes: null,
            });
            return;
        }

        // Serial, not Promise.all -- costs nothing at N<=3 calls and is what
        // makes "which resource failed" deterministic (see Decisions).
        const createdLinkedFields: Array<{type: ResourceObjectType; field: PropertyField}> = [];
        let outcome: SaveOutcome = {success: true};

        for (const type of appliesTo) {
            try {
                // eslint-disable-next-line no-await-in-loop
                const linkedField = await createLinkedAttributeField(
                    type,
                    currentName,
                    fieldType,
                    displayName,
                    templateField.id,
                    newResourceAttrsFor(type),
                    newResourcePermissionValuesFor(type),
                );
                createdLinkedFields.push({type, field: linkedField});
                syncUserAttributeFieldUpsert(dispatch, linkedField, true);
            } catch (error) {
                // eslint-disable-next-line no-await-in-loop
                outcome = await rollbackLinkedFields(dispatch, createdLinkedFields, templateField.id, type, error);
                break;
            }
        }

        finalizeSave(outcome);
    }, [dispatch, canSave, isEditMode, fieldId, objectType, nameUnchanged, displayName, currentName, fieldType, typeChanged, options, optionsOwnedBySync, wasSyncedWhenLoaded, externalLinks, appliesTo, channelResource, finalizeSave, confirmRemoveAppliesTo, userVisibility, userManaged, userRemoveLocked, allowedResourceTypes]);

    const handleChannelResourceChange = useCallback((next: ChannelResourceConfig) => {
        setChannelResource(next);
        markDirty();
    }, [markDirty]);

    const TypeIcon = getTypeIcon(fieldType);

    // Hierarchical is create-only: an existing field cannot be converted into
    // one (see typeLockedByGraph), so offering it here would only produce a 400
    // at Save. A field that already is one never opens this menu.
    const selectableTypes = isEditMode ? ALL_TYPES.filter((descriptor) => descriptor.id !== 'graph') : ALL_TYPES;

    // Reason derived once, then looked up for both the tooltip and the
    // aria-label below -- pluginOrphaned/plugin checked first: a plugin-owned
    // field with zero applied resources has typeLockedByAppliesTo === false and
    // hasExternalSource === false, so it must not fall through to the
    // "applies to a resource" copy, which would be factually wrong when it
    // applies to nothing. pluginOrphaned (not plain "plugin") whenever the
    // field is also orphaned, so this tooltip doesn't contradict the
    // "(no longer installed)" copy the Managed-by panel shows one card up.
    // graph outranks externalSource/appliesTo because it is the only one of the
    // three the admin cannot undo: telling them to remove resources or unlink a
    // source would promise an unlock that never arrives. owned outranks graph
    // and the rest: an owned template always has a linked Users field, so a
    // lower-ranked reason would blame "applies to a resource" instead of the
    // integration that actually holds the attribute.
    const typeLockReason = firstMatchingReason<'pluginOrphaned' | 'plugin' | 'owned' | 'graph' | 'externalSource' | 'appliesTo'>(
        ['pluginOrphaned', isPluginOwned && isOrphaned],
        ['plugin', isPluginOwned],
        ['owned', isOwned],
        ['graph', typeLockedByGraph],
        ['externalSource', hasExternalSource],
        ['appliesTo', typeLockedByAppliesTo],
    );
    const typeLockTooltip = formatMessage(TYPE_LOCK_MESSAGES[typeLockReason ?? 'appliesTo'].tooltip, {owners: ownersLabel});
    const typeButtonAriaLabel = typeLockReason ? formatMessage(TYPE_LOCK_MESSAGES[typeLockReason].ariaLabel, {value: formatMessage(getTypeLabel(fieldType)), owners: ownersLabel}) : formatMessage(messages.typeFieldAriaLabel, {value: formatMessage(getTypeLabel(fieldType))});
    const typeMenu = (
        <AttributeSelect
            idPrefix='attribute-type'
            dataTestId='attributeTypeMenuButton'
            selected={{id: fieldType, icon: TypeIcon, label: getTypeLabel(fieldType)}}
            options={selectableTypes}
            ariaLabel={typeButtonAriaLabel}
            menuAriaLabel={formatMessage(messages.typeMenuAriaLabel)}
            onChange={handleTypeChange}
            disabled={saving || effectiveDisabled}
            locked={typeLocked}
        />
    );

    if (loading) {
        return <LoadingScreen/>;
    }

    const linkedSourceTitle = linkedExternalSource ? formatMessage(externalSourceMessages[linkedExternalSource].title) : '';

    let optionsEditor: JSX.Element | null = null;
    if (isHierarchical) {
        optionsEditor = (
            <GraphValues
                options={options}
                onOptionsChange={handleOptionsChange}
                disabled={saving || effectiveDisabled}
            />
        );
    } else if (optionsOwnedBySync) {
        optionsEditor = (
            <>
                <AttributeOptionsValues
                    options={options}
                    onOptionsChange={handleOptionsChange}
                    readOnly={true}
                />
                <p
                    className='AttributeDetails__optionsHelp'
                    data-testid='attributeOptionsSyncedHelp'
                >
                    <FormattedMessage
                        {...messages.optionsSyncedHelp}
                        values={{source: linkedSourceTitle}}
                    />
                </p>
                {optionsIssue === 'needs_users' && (
                    <div
                        className='AttributeDetails__uniqueNameError'
                        role='alert'
                        data-testid='attributeOptionsNeedUsersError'
                    >
                        <FormattedMessage
                            {...messages.optionsSyncedNeedUsers}
                            values={{source: linkedSourceTitle}}
                        />
                    </div>
                )}
            </>
        );
    } else if (typeSupportsOptions) {
        optionsEditor = (
            <>
                {fieldType === 'rank' ? (
                    <AttributeOptionsRankValues
                        options={options}
                        onOptionsChange={handleOptionsChange}
                        disabled={saving || effectiveDisabled}
                    />
                ) : (
                    <AttributeOptionsValues
                        options={options}
                        onOptionsChange={handleOptionsChange}
                        disabled={saving || effectiveDisabled}
                    />
                )}
                {/* Suppressed for plugin-owned fields -- this validation targets
                    admin-authored data entered through this form; a plugin-supplied
                    template's options came from the plugin API and aren't guaranteed
                    to satisfy this UI's own shape rules. Showing an actionable-looking
                    error on a page with no controls to act on it would be worse than
                    showing nothing. */}
                {optionsIssue && !isPluginOwned && (
                    <div
                        className='AttributeDetails__uniqueNameError'
                        role='alert'
                        data-testid='attributeOptionsRequiredError'
                    >
                        <FormattedMessage {...(optionsIssue === 'required' ? messages.optionsRequired : messages.optionsInvalid)}/>
                    </div>
                )}
            </>
        );
    } else if (!hasExternalSource) {
        optionsEditor = (
            <p
                className='AttributeDetails__optionsHelp'
                data-testid='attributeOptionsHelp'
            >
                <FormattedMessage {...messages.optionsHelp}/>
            </p>
        );
    }

    // The two applies_to_* kinds below that interpolate resource names need
    // their own copy path -- they can't go through the flat
    // formatMessage(errorMessages[errorKind]) call every other kind uses,
    // since that call takes no values. applies_to_name_conflict and
    // applies_to_limit_reached use canned copy naming the actual cause
    // ("already used by a User Attribute") rather than the server's raw
    // message -- unlike the template's own name_conflict case below (which
    // reuses the server's message because it already names the specific
    // conflicting field/level), the server's generic name-conflict message
    // has no notion of "User Attribute" to say, since that framing is
    // specific to this feature's CPA-namespace overlap.
    let errorContent: React.ReactNode = null;
    if (errorKind && RESOURCE_INTERPOLATED_ERROR_KINDS.has(errorKind)) {
        errorContent = formatMessage(errorMessages[errorKind], {resources: resourceTypeListLabel(failedResourceTypes ?? [], formatMessage)});
    } else if (errorKind === 'applies_to_rollback_failed') {
        const resources = resourceTypeListLabel(failedResourceTypes ?? [], formatMessage);
        errorContent = resources ? formatMessage(errorMessages.applies_to_rollback_failed, {
            name: displayName,
            resources,
        }) : formatMessage(errorMessages.applies_to_template_rollback_failed, {name: displayName});
    } else if (errorKind === 'name_conflict' && serverErrorMessage) {
        errorContent = serverErrorMessage;
    } else if (errorKind) {
        errorContent = formatMessage(errorMessages[errorKind]);
    }

    return (
        <div
            className='wrapper--fixed AttributeDetails'
            data-testid='attributeDetails'
        >
            <AdminHeader withBackButton={true}>
                <div>
                    <div className='AttributeDetails__back'>
                        <BlockableLink
                            to={LIST_ROUTE}
                            className='AttributeDetails__backButton'
                            aria-label={formatMessage(messages.backLink)}
                            data-testid='attributeDetailsBackLink'
                        >
                            <ChevronLeftIcon
                                size={28}
                                aria-hidden={true}
                            />
                        </BlockableLink>
                    </div>
                    <hgroup className='AttributeDetails__headerGroup'>
                        <FormattedMessage
                            tagName='h1'
                            {...(isEditMode ? messages.editTitle : messages.title)}
                            values={isEditMode ? {name: formatPropertyFieldLabel(displayName || currentName)} : undefined}
                        />
                        <FormattedMessage
                            tagName='p'
                            {...messages.subtitle}
                        />
                    </hgroup>
                </div>
            </AdminHeader>
            <div className='admin-console__wrapper'>
                <div className='admin-console__content'>
                    <AdminPanel
                        id='attribute_definition'
                        title={messages.definitionTitle}
                        subtitle={messages.definitionSubtitle}
                    >
                        <div className='AttributeDetails__row'>
                            <label
                                className='AttributeDetails__label'
                                htmlFor='input_display_name'
                            >
                                <FormattedMessage {...messages.displayNameLabel}/>
                            </label>
                            <div className='AttributeDetails__fieldControl'>
                                <Input
                                    name='display_name'
                                    type='text'
                                    useLegend={false}
                                    placeholder={formatMessage(messages.displayNamePlaceholder)}
                                    aria-label={formatMessage(messages.displayNameLabel)}
                                    value={displayName}
                                    onChange={handleDisplayNameChange}
                                    autoFocus={true}
                                    disabled={saving || effectiveDisabled}
                                    maxLength={Constants.MAX_CUSTOM_ATTRIBUTE_NAME_LENGTH}
                                    data-testid='attributeDisplayNameInput'
                                />
                                <div className='AttributeDetails__uniqueName'>
                                    <span
                                        className='AttributeDetails__uniqueNameCaption'
                                        aria-live='polite'
                                        data-testid='attributeUniqueNameCaption'
                                    >
                                        <span
                                            className='AttributeDetails__uniqueNamePrefix'
                                            id='attribute-unique-name-prefix'
                                        >
                                            <FormattedMessage {...messages.uniqueNamePrefix}/>
                                        </span>
                                        {isEditingName ? (
                                            <input
                                                type='text'
                                                className={classNames('AttributeDetails__uniqueNameInput', {
                                                    'AttributeDetails__uniqueNameInput--error': Boolean(nameValidationError) || isServerNameError,
                                                })}
                                                value={manualName}
                                                onChange={handleNameChange}
                                                onKeyDown={handleNameKeyDown}
                                                onBlur={handleDoneClick}
                                                autoFocus={true}
                                                disabled={saving || effectiveDisabled || nameLocked}
                                                maxLength={CPA_FIELD_NAME_MAX_RUNES}
                                                aria-labelledby='attribute-unique-name-prefix'
                                                aria-describedby={nameDescribedBy}
                                                aria-invalid={Boolean(nameValidationError) || isServerNameError}
                                                data-testid='attributeNameInput'
                                            />
                                        ) : (
                                            <span
                                                className={classNames({'AttributeDetails__uniqueNameValue--error': Boolean(nameValidationError) || isServerNameError})}
                                                aria-describedby={nameDescribedBy}
                                                data-testid='attributeUniqueNameValue'
                                            >
                                                {isNameManuallyEdited ? manualName : (autoSlugDisplay ?? '—')}
                                            </span>
                                        )}
                                        {(() => {
                                            // Reason derived once, then looked up for both the aria-label
                                            // and the tooltip below -- same priority reasoning as
                                            // typeLockReason above, including preferring pluginOrphaned
                                            // over plain "plugin" once the field is confirmed orphaned.
                                            const nameLockReason = firstMatchingReason<'pluginOrphaned' | 'plugin' | 'owned' | 'appliesTo'>(
                                                ['pluginOrphaned', isPluginOwned && isOrphaned],
                                                ['plugin', isPluginOwned],
                                                ['owned', isOwned],
                                                ['appliesTo', nameLockedByAppliesTo],
                                            );
                                            const nameEditLinkAriaLabel = nameLockReason ? formatMessage(NAME_LOCK_MESSAGES[nameLockReason].ariaLabel, {owners: ownersLabel}) : formatMessage(isEditingName ? messages.doneLinkAriaLabel : messages.editLinkAriaLabel);

                                            const editLinkButton = (
                                                <button
                                                    type='button'
                                                    className='AttributeDetails__editLink'
                                                    onClick={isEditingName ? handleDoneClick : handleEditClick}
                                                    onMouseDown={(e) => {
                                                        // Blur runs before click. Without this, Done would
                                                        // commit on blur and the same click would re-open Edit.
                                                        if (isEditingName) {
                                                            e.preventDefault();
                                                        }
                                                    }}
                                                    disabled={saving || effectiveDisabled || nameLocked}
                                                    aria-disabled={isDoneBlocked || undefined}
                                                    aria-describedby={isDoneBlocked ? 'attribute-unique-name-error' : undefined}
                                                    aria-label={nameEditLinkAriaLabel}
                                                    data-testid='attributeNameEditLink'
                                                >
                                                    <FormattedMessage {...(isEditingName ? messages.doneLink : messages.editLink)}/>
                                                </button>
                                            );

                                            const nameLockTooltip = formatMessage(NAME_LOCK_MESSAGES[nameLockReason ?? 'appliesTo'].tooltip, {owners: ownersLabel});
                                            return nameLocked ? (
                                                <WithTooltip title={nameLockTooltip}>
                                                    <span
                                                        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- WithTooltip's useFocus only fires on its cloned child; without this the disabled button inside is unreachable by keyboard, so the tooltip explaining the lock is mouse-only
                                                        tabIndex={0}
                                                        data-testid='attributeNameEditLinkLockWrap'
                                                    >
                                                        {editLinkButton}
                                                    </span>
                                                </WithTooltip>
                                            ) : editLinkButton;
                                        })()}
                                    </span>
                                    {nameValidationError && (
                                        <div
                                            id='attribute-unique-name-error'
                                            className='AttributeDetails__uniqueNameError'
                                            role='alert'
                                            data-testid='attributeUniqueNameError'
                                        >
                                            {nameErrorMessage(nameValidationError, formatMessage)}
                                        </div>
                                    )}
                                    {autoSlugCollapsedToEmpty && (
                                        <div
                                            id='attribute-unique-name-empty-warning'
                                            className='AttributeDetails__uniqueNameError'
                                            role='alert'
                                            data-testid='attributeUniqueNameEmptyWarning'
                                        >
                                            <FormattedMessage {...messages.couldNotGenerateName}/>
                                        </div>
                                    )}
                                    {nameConflict && (
                                        <div
                                            className='AttributeDetails__nameConflict'

                                            // Polite, not an alert: this updates on every
                                            // keystroke while the admin is still typing a name.
                                            role='status'
                                            data-testid='attributeNameConflictWarning'
                                        >
                                            <SectionNotice
                                                type={usersBlockedByNameConflict ? 'danger' : 'warning'}
                                                title={formatMessage(nameConflictMessages.detailsNotice)}
                                            />
                                        </div>
                                    )}
                                    <p className='AttributeDetails__helperText'>
                                        <FormattedMessage {...messages.helperText}/>
                                    </p>
                                </div>
                            </div>
                        </div>
                        <div className='AttributeDetails__row'>
                            <span
                                className='AttributeDetails__label'
                                data-testid='attributeTypeLabel'
                            >
                                <FormattedMessage {...messages.typeLabel}/>
                            </span>
                            <div className='AttributeDetails__fieldControl'>
                                {typeLocked ? (
                                    <WithTooltip title={typeLockTooltip}>
                                        <span
                                            // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- WithTooltip's useFocus only fires on its cloned child; without this the disabled Type menu button is unreachable by keyboard, so the tooltip explaining the lock is mouse-only
                                            tabIndex={0}
                                            className='AttributeDetails__typeLockWrap'
                                            data-testid='attributeTypeLockWrap'
                                        >
                                            {typeMenu}
                                        </span>
                                    </WithTooltip>
                                ) : typeMenu}
                            </div>
                        </div>
                        <div className='AttributeDetails__row'>
                            <span
                                className='AttributeDetails__label'
                                data-testid='attributeOptionsLabel'
                            >
                                <FormattedMessage {...messages.optionsLabel}/>
                            </span>
                            <div className='AttributeDetails__fieldControl'>
                                {optionsEditor}
                                {isPluginOwned ? (
                                    <AttributePluginSource
                                        pluginId={sourcePluginId!}
                                        isOrphaned={isOrphaned}
                                        pluginInventoryLoaded={pluginInventoryLoaded}
                                    />
                                ) : (
                                    <>
                                        {isOwned && (
                                            <AttributeOwnersSource
                                                owners={owners}
                                                pluginInventoryLoaded={pluginInventoryLoaded}
                                            />
                                        )}
                                        {showsExternalSource && fieldType !== 'graph' && (
                                            <AttributeExternalSource
                                                links={externalLinks}
                                                fieldType={fieldType}
                                                onLink={handleLink}
                                                disabled={saving || effectiveDisabled}
                                                disableAdding={typeLockedByAppliesTo && !canSyncAttributeType(fieldType)}
                                            />
                                        )}
                                    </>
                                )}
                            </div>
                        </div>
                    </AdminPanel>
                    <AttributeAppliesTo
                        appliesTo={appliesTo}
                        allowedTypes={allowedResourceTypes}
                        disabled={saving || effectiveDisabled || standaloneRowLocked}
                        hideAddResource={isPluginOwned || isNonTemplate}
                        blockedTypes={usersBlockedByNameConflict ? {user: formatMessage(nameConflictMessages.usersBlockedMenuLabel)} : undefined}
                        lockedTooltip={appliesToLockedTooltip}
                        onAdd={handleAdd}
                        onRemove={handleRemove}
                        userVisibility={userVisibility}
                        onUserVisibilityChange={handleUserVisibilityChange}
                        userManaged={userManaged}
                        onUserManagedChange={handleUserManagedChange}
                        externalSource={managedByExternalSource}
                        userWhoCanSetLockedTooltip={ownedLockedTooltip}
                        removeLockedTooltips={removeLockedTooltips}
                        channelResource={channelResource}
                        onChannelResourceChange={handleChannelResourceChange}
                        ordered={fieldType === 'rank'}
                        channelFieldId={objectType === 'channel' ? fieldId : persistedLinkedFieldsRef.current.channel?.id}
                        attributeDisplayName={displayName}
                    />
                </div>
            </div>
            <div className='admin-console-save'>
                <SaveButton
                    disabled={!canSave}
                    saving={saving}
                    onClick={handleSave}
                    defaultMessage={<FormattedMessage {...messages.save}/>}
                />
                <Button
                    asChild={true}
                    emphasis='tertiary'
                    data-testid='attributeCancelLink'
                >
                    <BlockableLink to={LIST_ROUTE}>
                        <FormattedMessage {...messages.cancel}/>
                    </BlockableLink>
                </Button>
                {errorKind && (
                    <span
                        id='attribute-save-error'
                        className='AttributeDetails__error'
                        role='alert'
                        data-testid='attributeSaveError'
                    >
                        <i className='icon icon-alert-outline'/>
                        {errorContent}
                    </span>
                )}
            </div>
        </div>
    );
}

export default AttributeDetails;

const messages = defineMessages({
    backLink: {id: 'admin.global_attributes.attribute_details.back_link', defaultMessage: 'Back to Attribute Management'},
    title: {id: 'admin.global_attributes.attribute_details.title', defaultMessage: 'New attribute'},
    editTitle: {id: 'admin.global_attributes.attribute_details.edit_title', defaultMessage: 'Edit {name} Attribute'},
    subtitle: {id: 'admin.global_attributes.attribute_details.subtitle', defaultMessage: 'Add a display name, choose a type, and pick where it applies.'},
    definitionTitle: {id: 'admin.global_attributes.attribute_details.definition.title', defaultMessage: 'Definition'},
    definitionSubtitle: {id: 'admin.global_attributes.attribute_details.definition.subtitle', defaultMessage: 'Display name, type, and options.'},
    displayNameLabel: {id: 'admin.global_attributes.attribute_details.display_name.label', defaultMessage: 'Display name'},
    displayNamePlaceholder: {id: 'admin.global_attributes.attribute_details.display_name.placeholder', defaultMessage: 'Add a display name'},
    uniqueNamePrefix: {id: 'admin.global_attributes.attribute_details.unique_name.prefix', defaultMessage: 'Unique name:'},
    editLink: {id: 'admin.global_attributes.attribute_details.unique_name.edit', defaultMessage: 'Edit'},
    editLinkAriaLabel: {id: 'admin.global_attributes.attribute_details.unique_name.edit_aria_label', defaultMessage: 'Edit unique name'},
    doneLink: {id: 'admin.global_attributes.attribute_details.unique_name.done', defaultMessage: 'Done'},
    doneLinkAriaLabel: {id: 'admin.global_attributes.attribute_details.unique_name.done_aria_label', defaultMessage: 'Done editing unique name'},
    nameLockedAppliesToAriaLabel: {
        id: 'admin.global_attributes.attribute_details.unique_name.locked_applies_to_aria_label',
        defaultMessage: 'Edit unique name. Locked while this attribute applies to a resource.',
    },
    nameLockedAppliesToTooltip: {
        id: 'admin.global_attributes.attribute_details.unique_name.locked_applies_to_tooltip',
        defaultMessage: 'Name cannot be changed while this attribute applies to a resource. Remove and save first.',
    },
    nameLockedPluginAriaLabel: {
        id: 'admin.global_attributes.attribute_details.unique_name.locked_plugin_aria_label',
        defaultMessage: 'Edit unique name. Locked because this attribute is managed by a plugin.',
    },
    nameLockedPluginTooltip: {
        id: 'admin.global_attributes.attribute_details.unique_name.locked_plugin_tooltip',
        defaultMessage: 'Name cannot be changed — this attribute is managed by a plugin.',
    },
    nameLockedOwnedAriaLabel: {
        id: 'admin.global_attributes.attribute_details.unique_name.locked_owned_aria_label',
        defaultMessage: 'Edit unique name. Locked because this attribute is managed by {owners}.',
    },
    nameLockedOwnedTooltip: {
        id: 'admin.global_attributes.attribute_details.unique_name.locked_owned_tooltip',
        defaultMessage: 'Name cannot be changed — this attribute is managed by {owners}.',
    },
    nameLockedPluginOrphanedAriaLabel: {
        id: 'admin.global_attributes.attribute_details.unique_name.locked_plugin_orphaned_aria_label',
        defaultMessage: "Edit unique name. Locked because this attribute was managed by a plugin that's no longer installed.",
    },
    nameLockedPluginOrphanedTooltip: {
        id: 'admin.global_attributes.attribute_details.unique_name.locked_plugin_orphaned_tooltip',
        defaultMessage: "Name cannot be changed — this attribute was managed by a plugin that's no longer installed.",
    },
    helperText: {
        id: 'admin.global_attributes.attribute_details.unique_name.helper_text',
        defaultMessage: 'Name is the internal identifier for policies and integrations. Display name is what admins and users see.',
    },
    couldNotGenerateName: {
        id: 'admin.global_attributes.attribute_details.unique_name.could_not_generate',
        defaultMessage: "Couldn't generate a unique name from this display name. Click Edit to set one manually.",
    },
    typeLabel: {id: 'admin.global_attributes.attribute_details.type.label', defaultMessage: 'Type'},
    typeMenuAriaLabel: {id: 'admin.global_attributes.attribute_details.type.menu_label', defaultMessage: 'Select type'},
    typeFieldAriaLabel: {id: 'admin.global_attributes.attribute_details.type.field_aria_label', defaultMessage: 'Type: {value}'},
    typeFieldLockedAriaLabel: {id: 'admin.global_attributes.attribute_details.type.field_locked_external_source_aria_label', defaultMessage: 'Type: {value}. Locked while linked to an external source.'},
    typeFieldLockedAppliesToAriaLabel: {
        id: 'admin.global_attributes.attribute_details.type.field_locked_applies_to_aria_label',
        defaultMessage: 'Type: {value}. Locked while this attribute applies to a resource.',
    },
    typeFieldLockedPluginAriaLabel: {
        id: 'admin.global_attributes.attribute_details.type.field_locked_plugin_aria_label',
        defaultMessage: 'Type: {value}. Locked because this attribute is managed by a plugin.',
    },
    typeFieldLockedOwnedAriaLabel: {
        id: 'admin.global_attributes.attribute_details.type.field_locked_owned_aria_label',
        defaultMessage: 'Type: {value}. Locked because this attribute is managed by {owners}.',
    },
    typeFieldLockedPluginOrphanedAriaLabel: {
        id: 'admin.global_attributes.attribute_details.type.field_locked_plugin_orphaned_aria_label',
        defaultMessage: "Type: {value}. Locked because this attribute was managed by a plugin that's no longer installed.",
    },
    typeFieldLockedGraphAriaLabel: {
        id: 'admin.global_attributes.attribute_details.type.field_locked_graph_aria_label',
        defaultMessage: 'Type: {value}. Locked because a hierarchical attribute cannot be converted to another type.',
    },
    typeLockedAppliesToTooltip: {
        id: 'admin.global_attributes.attribute_details.type.locked_applies_to_tooltip',
        defaultMessage: 'Type cannot be changed while this attribute applies to a resource.',
    },
    typeLockedExternalSourceTooltip: {
        id: 'admin.global_attributes.attribute_details.type.locked_external_source_tooltip',
        defaultMessage: 'Type cannot be changed while this attribute is linked to an external source.',
    },
    typeLockedPluginTooltip: {
        id: 'admin.global_attributes.attribute_details.type.locked_plugin_tooltip',
        defaultMessage: 'Type cannot be changed — this attribute is managed by a plugin.',
    },
    typeLockedOwnedTooltip: {
        id: 'admin.global_attributes.attribute_details.type.locked_owned_tooltip',
        defaultMessage: 'Type cannot be changed — this attribute is managed by {owners}.',
    },
    typeLockedPluginOrphanedTooltip: {
        id: 'admin.global_attributes.attribute_details.type.locked_plugin_orphaned_tooltip',
        defaultMessage: "Type cannot be changed — this attribute was managed by a plugin that's no longer installed.",
    },
    typeLockedGraphTooltip: {
        id: 'admin.global_attributes.attribute_details.type.locked_graph_tooltip',
        defaultMessage: 'Type cannot be changed — a hierarchical attribute cannot be converted to another type. Create a new attribute of the type you need instead.',
    },
    appliesToLockedPluginTooltip: {
        id: 'admin.global_attributes.attribute_details.applies_to.locked_plugin_tooltip',
        defaultMessage: 'This resource cannot be changed — this attribute is managed by a plugin.',
    },
    appliesToLockedPluginOrphanedTooltip: {
        id: 'admin.global_attributes.attribute_details.applies_to.locked_plugin_orphaned_tooltip',
        defaultMessage: "This resource cannot be changed — this attribute was managed by a plugin that's no longer installed.",
    },
    appliesToLockedSingleResourceTooltip: {
        id: 'admin.global_attributes.attribute_details.applies_to.locked_single_resource_tooltip',
        defaultMessage: 'This resource cannot be changed — this attribute applies to only one resource.',
    },
    ownedLockedTooltip: {
        id: 'admin.global_attributes.attribute_details.applies_to.owned_locked_tooltip',
        defaultMessage: 'This attribute is managed by {owners}.',
    },
    optionsLabel: {id: 'admin.global_attributes.attribute_details.options.label', defaultMessage: 'Options'},
    optionsSyncedHelp: {
        id: 'admin.global_attributes.attribute_details.options.synced_help',
        defaultMessage: 'Options are managed by the {source} sync. Each value it sends is added as an option, and options that no user holds any more are removed.',
    },
    optionsSyncedNeedUsers: {
        id: 'admin.global_attributes.attribute_details.options.synced_need_users',
        defaultMessage: 'Add Users under Applies to. The {source} sync creates these options from the values it syncs to users.',
    },
    optionsHelp: {
        id: 'admin.global_attributes.attribute_details.options.help',
        defaultMessage: 'Text attributes have no preset values — a value is typed in per resource.',
    },
    optionsRequired: {
        id: 'admin.global_attributes.attribute_details.options.required',
        defaultMessage: 'At least one option is required.',
    },
    optionsInvalid: {
        id: 'admin.global_attributes.attribute_details.options.invalid',
        defaultMessage: "There's a problem with one or more options — check for a duplicate or overly long name, or a missing/duplicate rank.",
    },
    save: {id: 'admin.global_attributes.attribute_details.save', defaultMessage: 'Save'},
    cancel: {id: 'admin.global_attributes.attribute_details.cancel', defaultMessage: 'Cancel'},
});

// One reason derived once (see typeLockReason above), looked up here for both
// the tooltip and the aria-label -- a new lock reason becomes one entry in
// this map instead of a fourth branch across two separate if/else chains.
const TYPE_LOCK_MESSAGES: Record<'pluginOrphaned' | 'plugin' | 'owned' | 'graph' | 'externalSource' | 'appliesTo', {tooltip: MessageDescriptor; ariaLabel: MessageDescriptor}> = {
    pluginOrphaned: {tooltip: messages.typeLockedPluginOrphanedTooltip, ariaLabel: messages.typeFieldLockedPluginOrphanedAriaLabel},
    plugin: {tooltip: messages.typeLockedPluginTooltip, ariaLabel: messages.typeFieldLockedPluginAriaLabel},
    owned: {tooltip: messages.typeLockedOwnedTooltip, ariaLabel: messages.typeFieldLockedOwnedAriaLabel},
    graph: {tooltip: messages.typeLockedGraphTooltip, ariaLabel: messages.typeFieldLockedGraphAriaLabel},
    externalSource: {tooltip: messages.typeLockedExternalSourceTooltip, ariaLabel: messages.typeFieldLockedAriaLabel},
    appliesTo: {tooltip: messages.typeLockedAppliesToTooltip, ariaLabel: messages.typeFieldLockedAppliesToAriaLabel},
};

const NAME_LOCK_MESSAGES: Record<'pluginOrphaned' | 'plugin' | 'owned' | 'appliesTo', {tooltip: MessageDescriptor; ariaLabel: MessageDescriptor}> = {
    pluginOrphaned: {tooltip: messages.nameLockedPluginOrphanedTooltip, ariaLabel: messages.nameLockedPluginOrphanedAriaLabel},
    plugin: {tooltip: messages.nameLockedPluginTooltip, ariaLabel: messages.nameLockedPluginAriaLabel},
    owned: {tooltip: messages.nameLockedOwnedTooltip, ariaLabel: messages.nameLockedOwnedAriaLabel},
    appliesTo: {tooltip: messages.nameLockedAppliesToTooltip, ariaLabel: messages.nameLockedAppliesToAriaLabel},
};

const nameErrorMessages = defineMessages({
    invalidCharset: {
        id: 'admin.global_attributes.attribute_details.name_error.invalid_charset',
        defaultMessage: 'Name must start with a letter or underscore, and contain only letters, numbers, and underscores.',
    },
    reservedWord: {
        id: 'admin.global_attributes.attribute_details.name_error.reserved_word',
        defaultMessage: '"{word}" is a reserved word and cannot be used as a name.',
    },
    tooLong: {
        id: 'admin.global_attributes.attribute_details.name_error.too_long',
        defaultMessage: 'Name must be {max} characters or fewer.',
    },
});

const errorMessages = defineMessages({
    name_conflict: {
        id: 'admin.global_attributes.attribute_details.save_error.name_conflict',
        defaultMessage: 'An attribute with this name already exists. Please choose a different name.',
    },
    invalid_charset: {
        id: 'admin.global_attributes.attribute_details.save_error.invalid_charset',
        defaultMessage: 'Name must start with a letter or underscore, and contain only letters, numbers, and underscores.',
    },
    reserved_word: {
        id: 'admin.global_attributes.attribute_details.save_error.reserved_word',
        defaultMessage: 'This name is a reserved word and cannot be used. Please choose a different name.',
    },
    channel_resource: {
        id: 'admin.global_attributes.attribute_details.save_error.channel_resource',
        defaultMessage: 'The attribute was created, but it could not be applied to channels. Its definition can no longer be changed here; select Save to try applying it again, or remove the Channels resource to finish without it.',
    },
    limit_reached: {
        id: 'admin.global_attributes.attribute_details.save_error.limit_reached',
        defaultMessage: 'You have reached the maximum number of attributes for this server. Delete an existing attribute before creating a new one.',
    },
    invalid_options: {
        id: 'admin.global_attributes.attribute_details.save_error.invalid_options',
        defaultMessage: "There's a problem with one or more options — check for a duplicate or overly long name, or a missing/duplicate rank, then try again.",
    },
    generic: {
        id: 'admin.global_attributes.attribute_details.save_error.generic',
        defaultMessage: 'Something went wrong while saving this attribute. Please try again.',
    },
    type_change_with_dependents: {
        id: 'admin.global_attributes.attribute_details.save_error.type_change_with_dependents',
        defaultMessage: "This attribute's type can't be changed while it applies to a resource. Remove those resources first, then try again.",
    },
    applies_to_failed: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_failed',
        defaultMessage: "Couldn't apply this attribute to {resources}. Nothing was saved — please try again.",
    },
    applies_to_remove_failed: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_remove_failed',
        defaultMessage: "Couldn't remove this attribute from {resources}. Nothing else was saved — please try again.",
    },
    applies_to_remove_partial_save: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_remove_partial_save',
        defaultMessage: "The attribute was saved, but couldn't be removed from {resources}. Please try again.",
    },
    applies_to_partial_save: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_partial_save',
        defaultMessage: 'The attribute was saved, but couldn\'t be applied to {resources}. Please try again.',
    },
    applies_to_config_save_failed: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_config_save_failed',
        defaultMessage: "The attribute was saved, but its {resources} settings (Profile display, Who can set the value) couldn't be updated. Please try again.",
    },
    applies_to_display_name_save_failed: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_display_name_save_failed',
        defaultMessage: "The attribute was saved, but its {resources} display name couldn't be updated. Please try again.",
    },
    applies_to_rollback_failed: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_rollback_failed',
        defaultMessage: '"{name}" may have been partially created for {resources}. A retry under the same name will likely fail until those are cleaned up.',
    },
    applies_to_template_rollback_failed: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_template_rollback_failed',
        defaultMessage: '"{name}" was created but could not be cleaned up after a failed apply. A retry under the same name will likely fail until it is deleted.',
    },
    applies_to_name_conflict: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_name_conflict',
        defaultMessage: 'This name is already used by a User Attribute. Please choose a different name.',
    },
    applies_to_limit_reached: {
        id: 'admin.global_attributes.attribute_details.save_error.applies_to_limit_reached',
        defaultMessage: 'The maximum number of User Attributes has been reached. Delete an existing one before applying this attribute to Users.',
    },
});
