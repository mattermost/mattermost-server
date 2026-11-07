// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';
import type {PropertyField, PropertyFieldOptionPage} from '@mattermost/types/properties';

import {getAdminClient, licenseTier, test} from '@mattermost/playwright-lib';
import type {PlaywrightExtended} from '@mattermost/playwright-lib';

export const GLOBAL_ATTRIBUTES_ADMIN_PATH = '/admin_console/system_attributes/manage_attributes';

// Canonical values: webapp/channels/src/components/admin_console/global_attributes/constants.ts
// (cross-package import not feasible between e2e-tests and webapp)
const PROPERTY_GROUP = 'access_control';
const OBJECT_TYPE = 'template';
const TARGET_TYPE = 'system';

// The three resource object types an Applies-to linked field can use. Also the
// only object types (besides 'template') PropertyField.IsValid allows a linked
// field to carry -- a template field itself is rejected for having a
// linked_field_id ("template fields cannot have a linked field").
// Canonical values: webapp/channels/.../attribute_details/attribute_applies_to_constants.ts
export type ResourceObjectType = 'user' | 'channel' | 'post';
const ALL_RESOURCE_OBJECT_TYPES: ResourceObjectType[] = ['user', 'channel', 'post'];

// Server clamps per_page to this max (see web.PerPageMaximum in server/channels/web/params.go).
// Directory-mode search with no cursor sorts CreateAt ASC, so the default 60-item page only
// returns the oldest fields — request the max to reduce the risk of missing newer ones.
const MAX_PROPERTY_FIELDS_PER_PAGE = 200;

/**
 * Shared precondition for every test that needs the Attribute Management page actually
 * reachable: skips on a sub-Enterprise license. Returns the admin session.
 */
export async function requireGlobalAttributesEnabled(pw: PlaywrightExtended) {
    await pw.skipIfNoLicense();
    const {adminUser, adminClient} = await getAdminClient();

    if (!adminUser || !adminClient) {
        throw new Error('Failed to get admin user');
    }

    const license = await adminClient.getClientLicenseOld();
    test.skip(
        licenseTier(license.SkuShortName) < 20,
        'Attribute Management requires Enterprise-tier license (SkuShortName enterprise, entry, or advanced). ' +
            'Professional is not sufficient—the admin route is hidden and redirects away.',
    );

    return {adminUser, adminClient};
}

export async function setGlobalAttributesFeatureFlag(adminClient: Client4, enabled: boolean) {
    await adminClient.patchConfig({
        FeatureFlags: {
            GlobalAttributes: enabled,
        },
    } as Parameters<Client4['patchConfig']>[0]);
}

/**
 * Precondition for hierarchical (graph) authoring, on top of
 * {@link requireGlobalAttributesEnabled}'s license check. Does not enable
 * PropertyFieldGraph itself -- callers must include it in their own
 * pw.ensureFeatureFlag() call.
 */
export async function requireHierarchicalAttributesEnabled(pw: PlaywrightExtended) {
    return requireGlobalAttributesEnabled(pw);
}

/**
 * Removes any access_control/template field with the given name (clean slate for E2E),
 * ignoring failures — the property routes may be unavailable below Enterprise tier, or
 * the field may simply not exist yet.
 */
export async function deleteGlobalAttributeFieldIfExists(adminClient: Client4, name: string) {
    try {
        const fields = await adminClient.getPropertyFields(PROPERTY_GROUP, OBJECT_TYPE, {
            targetType: TARGET_TYPE,
            perPage: MAX_PROPERTY_FIELDS_PER_PAGE,
        });
        for (const field of fields.filter((f) => f.name === name && f.delete_at === 0)) {
            await adminClient.deletePropertyField(PROPERTY_GROUP, OBJECT_TYPE, field.id);
        }
    } catch {
        // May not exist, or routes unavailable; ignore.
    }
}

/**
 * Returns the live access_control/template field with the given name, or undefined
 * if it is missing. Used to inspect the saved graph payload after a UI save.
 */
export async function getGlobalAttributeFieldByName(adminClient: Client4, name: string) {
    const fields = await adminClient.getPropertyFields(PROPERTY_GROUP, OBJECT_TYPE, {
        targetType: TARGET_TYPE,
        perPage: MAX_PROPERTY_FIELDS_PER_PAGE,
    });
    return fields.find((f) => f.name === name && f.delete_at === 0);
}

/**
 * Lists a field's options including graph parents. Field GET omits parents from
 * the inlined attrs.options list on purpose (a field read-modify-write must not
 * flatten the hierarchy); the dedicated options route is what reports them.
 *
 * The options route returns a page object. has_more is the only stop signal,
 * and the cursor names the last candidate examined — not the last option
 * returned — so this helper forwards the server cursor rather than deriving one.
 */
export async function getGlobalAttributeFieldOptions(adminClient: Client4, fieldId: string) {
    const all: Array<{id: string; name: string; parents?: string[] | null}> = [];
    let cursorId: string | undefined;
    let cursorCreateAt: number | undefined;

    for (;;) {
        const params = new URLSearchParams({per_page: String(MAX_PROPERTY_FIELDS_PER_PAGE)});
        if (cursorId) {
            params.set('cursor_id', cursorId);
        }
        if (cursorCreateAt) {
            params.set('cursor_create_at', String(cursorCreateAt));
        }

        const url = `${adminClient.getPropertyFieldRoute(PROPERTY_GROUP, OBJECT_TYPE, fieldId)}/options?${params.toString()}`;
        const response = await fetch(url, {
            headers: {Authorization: `Bearer ${adminClient.getToken()}`},
        });
        if (!response.ok) {
            throw new Error(`Failed to list property field options: ${response.status}`);
        }

        const page = (await response.json()) as PropertyFieldOptionPage;
        all.push(...page.options);

        if (!page.has_more) {
            return all;
        }
        if (!page.next_cursor_id || !page.next_cursor_create_at) {
            throw new Error('Failed to list property field options: has_more without a cursor');
        }

        cursorId = page.next_cursor_id;
        cursorCreateAt = page.next_cursor_create_at;
    }
}

/**
 * Best-effort cleanup for an Applies-to save: delete linked fields first
 * (looked up from the live template, not from the test's success-path locals)
 * so the template delete is not 409'd by leftover dependents.
 */
export async function deleteAppliesToAttributeAndLinkedFieldsIfExists(adminClient: Client4, name: string) {
    try {
        const templates = await adminClient.getPropertyFields(PROPERTY_GROUP, OBJECT_TYPE, {
            targetType: TARGET_TYPE,
            perPage: MAX_PROPERTY_FIELDS_PER_PAGE,
        });
        for (const template of templates.filter((field) => field.name === name && field.delete_at === 0)) {
            const linked = await fetchLinkedFieldsForTemplate(adminClient, template.id);
            for (const field of linked) {
                await deleteLinkedDependentField(adminClient, field.id, field.object_type as ResourceObjectType);
            }
        }
    } catch {
        // Listing may fail below Enterprise tier; still try the template delete below.
    }
    await deleteGlobalAttributeFieldIfExists(adminClient, name);
}

/**
 * Creates an access_control/template property field for E2E seeding. Ensures a clean
 * slate first so reruns don't collide with a field left over from a prior failed run.
 */
export async function createGlobalAttributeField(
    adminClient: Client4,
    name: string,
    field: Partial<Parameters<Client4['createPropertyField']>[2]>,
) {
    await deleteGlobalAttributeFieldIfExists(adminClient, name);

    return adminClient.createPropertyField(PROPERTY_GROUP, OBJECT_TYPE, {
        name,
        target_type: TARGET_TYPE,
        target_id: '',
        ...field,
    } as Parameters<Client4['createPropertyField']>[2]);
}

/**
 * Creates a field that links to `sourceFieldId` (e.g. the template). Two uses:
 * seeding an already-saved Applies-to resource for a given `objectType`
 * (defaulting to 'user', the one pre-existing call site's shape), and making
 * the server refuse to delete the source field -- deletePropertyField counts
 * live linked dependents and returns 409 `has_linked_dependents` when any
 * exist (server/channels/app/properties/property_field.go), which this is
 * the only way to exercise against a real server response. The create flow
 * for a fresh Applies-to resource itself is exercised through the UI, not
 * this helper.
 */
export async function createLinkedDependentField(
    adminClient: Client4,
    name: string,
    sourceFieldId: string,
    type: string,
    objectType: ResourceObjectType = 'user',
    attrs?: Record<string, unknown>,
) {
    return adminClient.createPropertyField(PROPERTY_GROUP, objectType, {
        name,
        type,
        target_type: TARGET_TYPE,
        target_id: '',
        linked_field_id: sourceFieldId,
        ...(attrs ? {attrs} : {}),
    } as Parameters<Client4['createPropertyField']>[2]);
}

/**
 * Deletes a linked property field, ignoring failures -- mirrors deleteGlobalAttributeFieldIfExists'
 * best-effort cleanup style, since a test's own save/rollback assertions may have already deleted it.
 * Must run BEFORE deleting the field it points at -- the source delete stays blocked with a 409
 * for as long as a live dependent exists.
 */
export async function deleteLinkedDependentField(
    adminClient: Client4,
    fieldId: string,
    objectType: ResourceObjectType = 'user',
) {
    try {
        await adminClient.deletePropertyField(PROPERTY_GROUP, objectType, fieldId);
    } catch {
        // May already be gone (e.g. a prior rollback already deleted it); ignore.
    }
}

/**
 * Finds every linked field (across all three resource object types) pointing at `templateFieldId`.
 * Queries user/channel/post separately -- there is no single "all object types" listing endpoint --
 * and requests the max page size per call, since the `user` object type's result page is shared
 * with every Custom Profile Attributes field on the server (see deleteGlobalAttributeFieldIfExists'
 * own MAX_PROPERTY_FIELDS_PER_PAGE comment) and a freshly-created linked field is exactly the kind
 * of newest-row a default ascending-CreateAt page can drop.
 */
export async function fetchLinkedFieldsForTemplate(
    adminClient: Client4,
    templateFieldId: string,
): Promise<PropertyField[]> {
    const results = await Promise.all(
        ALL_RESOURCE_OBJECT_TYPES.map((objectType) =>
            adminClient.getPropertyFields(PROPERTY_GROUP, objectType, {
                targetType: TARGET_TYPE,
                perPage: MAX_PROPERTY_FIELDS_PER_PAGE,
            }),
        ),
    );

    return results.flat().filter((field) => field.linked_field_id === templateFieldId && field.delete_at === 0);
}
