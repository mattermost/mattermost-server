// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * Unarchiving a channel that lacks a value for a required channel attribute
 * (MM-70717). Warn-only by design: the warning never blocks the restore, it
 * only explains what is missing and relabels the confirm button "Unarchive
 * anyway". There was no unarchive spec in the suite before this file.
 */

import {expect, test, getAdminClient} from '@mattermost/playwright-lib';

import {
    requireGlobalAttributesEnabled,
    setGlobalAttributesFeatureFlag,
} from '../../system_console/global_attributes/global_attributes_helpers';
import {
    createChannelAttributeField,
    createComplianceChannel,
    deleteChannelAttributeField,
    fieldName,
    optionId,
    purgeRequiredGateFields,
} from '../../system_console/global_attributes/required_gate_helpers';

test.describe('Unarchive - required channel attribute warning', {tag: ['@channel_attributes']}, () => {
    test.describe.configure({mode: 'serial'});

    let originalGlobalAttributes: boolean | undefined;

    test.beforeAll(async () => {
        const {adminClient} = await getAdminClient();
        const {FeatureFlags} = await adminClient.getConfig();
        originalGlobalAttributes = FeatureFlags.GlobalAttributes === true;
    });

    test.afterAll(async () => {
        const {adminClient} = await getAdminClient();
        if (adminClient && originalGlobalAttributes !== undefined) {
            await setGlobalAttributesFeatureFlag(adminClient, originalGlobalAttributes);
        }
    });

    /**
     * @objective Verify unarchiving a channel that predates a now-required
     * attribute warns, but still restores the channel successfully.
     *
     * The channel is archived before the field is created, so the helper can
     * create the field optional, backfill every active channel, and mark it
     * Required while this channel remains outside the gate population.
     */
    test('warns and offers "Unarchive anyway" for a channel missing a required value, and restores it anyway', async ({
        pw,
    }) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

        const suffix = pw.random.id();
        await purgeRequiredGateFields(adminClient);

        const {team} = await pw.initSetup();
        const channel = await createComplianceChannel(adminClient, team, suffix, `Unarchive Warn ${suffix}`);
        await adminClient.deleteChannel(channel.id);

        const pair = await createChannelAttributeField(adminClient, fieldName('unarchive_warn', suffix), {
            options: ['Value A'],
            required: true,
        });

        try {
            const {channelsPage} = await pw.testBrowser.login(adminUser);
            await channelsPage.goto(team.name, channel.name);
            await expect(channelsPage.archivedChannelMessage).toBeVisible();

            const modal = await channelsPage.openUnarchiveChannelModal();
            await modal.toBeVisible();

            // * Warns, naming this being about a required attribute
            await expect(modal.missingAttributesBanner).toBeVisible();
            await expect(modal.missingAttributesBanner).toContainText('Missing required attribute values');

            // * The confirm button reflects the warning
            await expect(modal.confirmButton).toHaveText('Unarchive anyway');

            // # Confirming still restores the channel -- never a dead end
            await modal.confirm();
            await expect(channelsPage.archivedChannelMessage).toHaveCount(0);
        } finally {
            await deleteChannelAttributeField(adminClient, pair);
        }
    });

    /**
     * @objective Verify no warning appears, and the plain "Unarchive" label is
     * kept, when the channel already has a value for the required attribute.
     */
    test('shows no warning and the plain "Unarchive" label when the channel already has a value', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

        const suffix = pw.random.id();
        await purgeRequiredGateFields(adminClient);

        const {team} = await pw.initSetup();
        const pair = await createChannelAttributeField(adminClient, fieldName('unarchive_ok', suffix), {
            options: ['Value A'],
            required: true,
        });

        // Creation-time enforcement (pre-existing, unrelated to MM-70717) requires
        // a value up front since the field is already required.
        const channel = await adminClient.createChannel({
            team_id: team.id,
            name: `unarchive-ok-${suffix}`.toLowerCase(),
            display_name: `Unarchive OK ${suffix}`,
            type: 'O',
            property_values: [{field_id: pair.channelField.id, value: optionId(pair.templateField, 'Value A')}],
        } as Parameters<typeof adminClient.createChannel>[0]);
        await adminClient.deleteChannel(channel.id);

        try {
            const {channelsPage} = await pw.testBrowser.login(adminUser);
            await channelsPage.goto(team.name, channel.name);
            await expect(channelsPage.archivedChannelMessage).toBeVisible();

            const modal = await channelsPage.openUnarchiveChannelModal();
            await modal.toBeVisible();

            await expect(modal.missingAttributesBanner).toHaveCount(0);
            await expect(modal.confirmButton).toHaveText('Unarchive');

            await modal.confirm();
            await expect(channelsPage.archivedChannelMessage).toHaveCount(0);
        } finally {
            await deleteChannelAttributeField(adminClient, pair);
        }
    });

    /**
     * @objective Verify the confirm button has focus when the warning is shown, so
     * Enter restores the channel as it did before the warning was added.
     */
    test('Enter confirms the unarchive while the warning is shown', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

        const suffix = pw.random.id();
        await purgeRequiredGateFields(adminClient);

        const {team} = await pw.initSetup();
        const channel = await createComplianceChannel(adminClient, team, suffix, `Unarchive Enter ${suffix}`);
        await adminClient.deleteChannel(channel.id);

        const pair = await createChannelAttributeField(adminClient, fieldName('unarchive_enter', suffix), {
            options: ['Value A'],
            required: true,
        });

        try {
            const {channelsPage, page} = await pw.testBrowser.login(adminUser);
            await channelsPage.goto(team.name, channel.name);
            await expect(channelsPage.archivedChannelMessage).toBeVisible();

            const modal = await channelsPage.openUnarchiveChannelModal();
            await expect(modal.missingAttributesBanner).toBeVisible();

            // * Focus is on the confirm button
            await expect(modal.confirmButton).toBeFocused();

            // # Enter restores the channel
            await page.keyboard.press('Enter');
            await expect(modal.container).not.toBeVisible();
            await expect(channelsPage.archivedChannelMessage).toHaveCount(0);
        } finally {
            await deleteChannelAttributeField(adminClient, pair);
        }
    });
});
