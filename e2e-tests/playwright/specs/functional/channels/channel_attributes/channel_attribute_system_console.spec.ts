// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * System Console — Channel Configuration's "Channel attributes" section
 * (MM-70717 follow-up). Lets a sysadmin set a channel's attribute values
 * directly, without visiting the channel itself. Edits batch under the
 * page's own Save Changes bar rather than saving per field.
 */

import type {PropertyField} from '@mattermost/types/properties';

import {expect, getAdminClient, test} from '@mattermost/playwright-lib';

import {
    attributeName,
    createAttribute,
    createChannelForAttributes,
    deleteAttributes,
    optionId,
    purgeAttributes,
    readChannelValues,
    valueFor,
} from './helpers';

test.describe('System Console - Channel attributes section', {tag: ['@channel_attributes', '@system_console']}, () => {
    test.describe.configure({mode: 'serial'});

    /**
     * @objective Verify a select attribute's value can be set from the channel
     * details page, that it does not persist until the page Save is clicked,
     * and that it is stored correctly once saved.
     */
    test('sets a channel attribute value from Channel Configuration, batched under Save', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        await pw.skipIfNoLicense();

        const {adminUser, adminClient} = await getAdminClient();
        if (!adminUser || !adminClient) {
            throw new Error('Admin user/client not available');
        }
        const suffix = pw.random.id();
        const created: PropertyField[] = [];

        try {
            await purgeAttributes(adminClient);

            const program = await createAttribute(adminClient, attributeName('program', suffix), {
                type: 'select',
                options: ['AURORA', 'ZEPHYR'],
            });
            created.push(program);

            const {team} = await pw.initSetup();
            const channel = await createChannelForAttributes(adminClient, team, `sysconsole-set-${suffix}`);

            const {page} = await pw.testBrowser.login(adminUser);
            await page.goto(`/admin_console/user_management/channels/${channel.id}`);

            const trigger = page.getByTestId(`channelAttributesSettings-${program.name}`);
            await expect(trigger).toBeVisible();

            // # Pick a value, but do not save yet
            await trigger.click();
            await page.getByText('AURORA', {exact: true}).click();

            // * The pending edit has not reached the server
            expect(valueFor(await readChannelValues(adminClient, channel.id), program)).toBeUndefined();

            // # Save the page
            const saveButton = page.getByTestId('saveSetting');
            await expect(saveButton).toBeEnabled();
            await saveButton.click();
            await expect(page).toHaveURL(/\/admin_console\/user_management\/channels$/);

            // * The value is now stored
            await expect
                .poll(async () => valueFor(await readChannelValues(adminClient, channel.id), program))
                .toBe(optionId(program, 'AURORA'));
        } finally {
            await deleteAttributes(adminClient, created);
        }
    });

    /**
     * @objective Verify an attribute locked to permission_values="none" cannot be
     * edited from the channel details page, even by a sysadmin.
     */
    test('renders a permission_values="none" attribute as disabled', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        await pw.skipIfNoLicense();

        const {adminUser, adminClient} = await getAdminClient();
        if (!adminUser || !adminClient) {
            throw new Error('Admin user/client not available');
        }
        const suffix = pw.random.id();
        const created: PropertyField[] = [];

        try {
            await purgeAttributes(adminClient);

            const locked = await createAttribute(adminClient, attributeName('locked', suffix), {
                type: 'select',
                options: ['ONE'],
                permissionValues: 'none',
            });
            created.push(locked);

            const {team} = await pw.initSetup();
            const channel = await createChannelForAttributes(adminClient, team, `sysconsole-locked-${suffix}`);

            const {page} = await pw.testBrowser.login(adminUser);
            await page.goto(`/admin_console/user_management/channels/${channel.id}`);

            const trigger = page.getByTestId(`channelAttributesSettings-${locked.name}`);
            await expect(trigger).toBeVisible();
            await expect(trigger).toBeDisabled();
        } finally {
            await deleteAttributes(adminClient, created);
        }
    });
});
