// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * Unarchive with ChannelAttributesRequired off (MM-70717).
 *
 * With the sub-flag off the server enforces Required on no channel attribute,
 * so a channel missing a value for a field still marked Required has nothing
 * to be warned about. Its own file because it needs the sub-flag off, unlike
 * unarchive_required_attribute_warning.spec.ts.
 *
 * This file calls ensureFeatureFlag with two combinations on purpose. The server
 * reads the sub-flag at runtime, but the Testcontainers server sets it through an
 * MM_FEATUREFLAGS_* env var, and the config store re-applies env overrides on every
 * config save, so patchConfig cannot turn it off. Only a restart can.
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
    purgeRequiredGateFields,
} from '../../system_console/global_attributes/required_gate_helpers';

test.describe(
    'Unarchive - required channel attribute warning (ChannelAttributesRequired off)',
    {tag: ['@channel_attributes']},
    () => {
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
         * @objective Verify unarchiving shows no missing-attribute warning and keeps the
         * plain "Unarchive" label while ChannelAttributesRequired is off.
         *
         * The field can only be marked Required while the sub-flag is on, so this test
         * sets it up first and then restarts the server with the sub-flag off. The
         * field stays Required across the restart; only enforcement is turned off.
         */
        test('shows no warning for a channel missing a required value', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            let {adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeRequiredGateFields(adminClient);

            const {team} = await pw.initSetup();
            const channel = await createComplianceChannel(adminClient, team, suffix, `Unarchive Flag Off ${suffix}`);
            await adminClient.deleteChannel(channel.id);

            const pair = await createChannelAttributeField(adminClient, fieldName('unarchive_flag_off', suffix), {
                options: ['Value A'],
                required: true,
            });

            try {
                // # Turn required enforcement off. The restart replaces the server, so fetch fresh clients.
                await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: false});
                const enabled = await requireGlobalAttributesEnabled(pw);
                adminClient = enabled.adminClient;

                const {channelsPage, page} = await pw.testBrowser.login(enabled.adminUser);
                await channelsPage.goto(team.name, channel.name);
                await expect(channelsPage.archivedChannelMessage).toBeVisible();

                const modal = await channelsPage.openUnarchiveChannelModal();
                await modal.toBeVisible();

                // Let any attribute value lookup the modal makes settle before asserting absence.
                await page.waitForLoadState('networkidle');

                // * Nothing to warn about while Required is not enforced
                await expect(modal.confirmButton).toHaveText('Unarchive');
                await expect(modal.missingAttributesBanner).toHaveCount(0);

                await modal.confirm();
                await expect(channelsPage.archivedChannelMessage).toHaveCount(0);
            } finally {
                await deleteChannelAttributeField(adminClient, pair);
            }
        });
    },
);
