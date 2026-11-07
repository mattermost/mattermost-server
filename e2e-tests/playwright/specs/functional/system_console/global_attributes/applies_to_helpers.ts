// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';
import type {PropertyField} from '@mattermost/types/properties';

import type {SystemConsolePage} from '@mattermost/playwright-lib';

import {backfillAndRequireChannelAttribute} from '../../channels/channel_attributes/helpers';

const PROPERTY_GROUP = 'access_control';

export type ChannelDisplayLocation = 'display_label_header' | 'display_banner_top';

export type ChannelAttributeConfig = {
    displayName: string;
    type?: 'Select' | 'Multiselect' | 'Text';
    options?: string[];
    required?: boolean;

    // The menu label, e.g. 'Cannot be changed once set'. Left alone the row keeps
    // its default, 'Can be changed at any time'.
    changePolicy?: string;

    displayLocations?: ChannelDisplayLocation[];
};

/**
 * Mirrors slugifyForCEL in webapp utils/properties.ts, which is what the console
 * derives the unique name from as the display name is typed.
 */
export function derivedAttributeName(displayName: string): string {
    let slug = displayName
        .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
        .replace(/([A-Z]+)([A-Z][a-z])/g, '$1_$2')
        .toLowerCase()
        .replace(/[^a-z0-9_]/g, '_');
    if (/^[0-9]/.test(slug)) {
        slug = '_' + slug;
    }
    return slug.replace(/_+/g, '_').replace(/_+$/, '') || '_copy';
}

/**
 * Creates an attribute and its Channels resource entirely through the System
 * Console UI, and returns the name the console derived for it.
 */
export async function configureChannelAttribute(
    systemConsolePage: SystemConsolePage,
    {displayName, type, options = [], required = false, changePolicy, displayLocations = []}: ChannelAttributeConfig,
    adminClient?: Client4,
): Promise<string> {
    const {globalAttributes} = systemConsolePage;
    const {attributeAppliesToChannels} = globalAttributes;

    await globalAttributes.gotoNewAttribute();
    await globalAttributes.setDisplayName(displayName);

    if (type && type !== 'Text') {
        await globalAttributes.selectType(type);
    }
    if (options.length) {
        await globalAttributes.addOptions(options);
    }

    await attributeAppliesToChannels.addResource();
    if (!required) {
        await attributeAppliesToChannels.setRequired(false);
    }
    if (changePolicy) {
        await attributeAppliesToChannels.setChangePolicy(changePolicy);
    }
    await attributeAppliesToChannels.setDisplayLocations(displayLocations);

    await globalAttributes.save();

    const name = derivedAttributeName(displayName);
    if (required) {
        if (!adminClient) {
            throw new Error('adminClient is required to backfill a required channel attribute');
        }
        const field = await findChannelField(adminClient, name);
        if (!field) {
            throw new Error(`channel property field ${name} not found`);
        }
        const fieldOptions = (field.attrs?.options ?? []) as Array<{id: string}>;
        let backfillValue: unknown = 'e2e backfill';
        if (field.type === 'select') {
            backfillValue = fieldOptions[0].id;
        } else if (field.type === 'multiselect') {
            backfillValue = [fieldOptions[0].id];
        }
        await backfillAndRequireChannelAttribute(adminClient, field, backfillValue);
    }

    return name;
}

export async function findChannelField(adminClient: Client4, name: string): Promise<PropertyField | undefined> {
    const fields = await adminClient.getPropertyFields(PROPERTY_GROUP, 'channel', {targetType: 'system', perPage: 200});
    return fields.find((field) => field.name === name && field.delete_at === 0);
}

export async function deleteChannelFieldIfExists(adminClient: Client4, name: string) {
    try {
        const field = await findChannelField(adminClient, name);
        if (field) {
            await adminClient.deletePropertyField(PROPERTY_GROUP, 'channel', field.id);
        }
    } catch {
        // May not exist, or the property routes may be unavailable; ignore.
    }
}
