// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * System Console — the required-channel-attribute compliance gate (MM-70717).
 *
 * Marking a channel attribute Required is blocked while any active channel on
 * the server lacks a value for it. The toggle itself never visually disables:
 * a blocked click reveals the missing-values banner instead of flipping it,
 * and turning Required back off always works regardless of channel state.
 *
 * Every scenario here seeds its own template+linked-channel-field pair via
 * the API (see required_gate_helpers.ts) rather than driving the "New
 * attribute" create flow through the UI for setup — that flow is already
 * covered by channel_resource_configuration.spec.ts. This file is only about
 * what happens once such a field already exists and is edited.
 */

import type {Client4} from '@mattermost/client';
import type {Channel} from '@mattermost/types/channels';

import {expect, test, getAdminClient} from '@mattermost/playwright-lib';

import {
    attributeName,
    backfillChannelAttribute,
    createAttribute,
    deleteAttributes,
    optionId as attributeOptionId,
    purgeAttributes,
} from '../../channels/channel_attributes/helpers';

import {requireGlobalAttributesEnabled, setGlobalAttributesFeatureFlag} from './global_attributes_helpers';
import {
    ATTRIBUTE_DETAILS_PATH,
    GLOBAL_ATTRIBUTES_PATH,
    createChannelAttributeField,
    createComplianceChannel,
    deleteChannelAttributeField,
    fieldName,
    getChannelPropertyField,
    optionId,
    purgeRequiredGateFields,
} from './required_gate_helpers';

// Archived channels are not counted by the gate, so archiving keeps the
// server-wide missing-values count from leaking into later tests and specs.
async function archiveChannels(adminClient: Client4, channels: Channel[]) {
    for (const channel of channels) {
        await adminClient.deleteChannel(channel.id).catch(() => undefined);
    }
}

test.describe(
    'System Console - the required-channel-attribute gate',
    {tag: ['@system_console', '@channel_attributes']},
    () => {
        // Shares the server-wide GlobalAttributes flag with the sibling specs.
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
         * @objective Verify a blocked click on Required does not flip it, reveals the
         * missing-values banner instead, and leaves the rest of the page usable.
         */
        test('does not flip Required while a channel lacks a value, and the page stays usable', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeRequiredGateFields(adminClient);

            const {team} = await pw.initSetup();
            const pair = await createChannelAttributeField(adminClient, fieldName('blocked', suffix), {
                options: ['Value A'],
                displayName: `Blocked ${suffix}`,
            });
            const channel = await createComplianceChannel(adminClient, team, suffix);

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);
                const {page, globalAttributes} = systemConsolePage;
                const {attributeAppliesToChannels} = globalAttributes;

                await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
                await expect(globalAttributes.displayNameInput).toBeVisible();
                await attributeAppliesToChannels.expand();

                // * The banner explains why, before any click
                await expect(attributeAppliesToChannels.missingValuesBanner).toBeVisible();
                await expect(attributeAppliesToChannels.missingValuesBanner).toContainText(
                    'Set channel attribute values',
                );
                await expect(attributeAppliesToChannels.viewChannelListButton).toBeVisible();
                await expect(attributeAppliesToChannels.notifyChannelAdminsButton).toBeVisible();

                // # Click Required while blocked
                await attributeAppliesToChannels.clickRequired();

                // * It did not flip
                await expect(attributeAppliesToChannels.requiredToggle).not.toBeChecked();
                // * The banner is still there to explain why
                await expect(attributeAppliesToChannels.missingValuesBanner).toBeVisible();

                // # The blocked click did not break the page: an unrelated edit still saves
                await attributeAppliesToChannels.setDisplayLocations(['display_label_header']);
                await globalAttributes.save();
                await expect(page).toHaveURL(new RegExp(`${GLOBAL_ATTRIBUTES_PATH}$`));

                const reloaded = await getChannelPropertyField(adminClient, pair.channelField.id);
                expect(reloaded.attrs?.required).not.toBe(true);
                expect(reloaded.attrs?.actions).toEqual(['display_label_header']);
            } finally {
                await archiveChannels(adminClient, [channel]);
                await deleteChannelAttributeField(adminClient, pair);
            }
        });

        /**
         * @objective Verify turning Required off always works while channels are
         * missing values -- the escape hatch for a field that started required.
         */
        test('always allows turning Required off, even while channels are missing values', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeRequiredGateFields(adminClient);

            const {team} = await pw.initSetup();
            const channel = await createComplianceChannel(adminClient, team, suffix);
            await adminClient.deleteChannel(channel.id);

            const pair = await createChannelAttributeField(adminClient, fieldName('escape', suffix), {
                options: ['Value A'],
                required: true,
                displayName: `Escape ${suffix}`,
            });
            await adminClient.unarchiveChannel(channel.id);

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);
                const {page, globalAttributes} = systemConsolePage;
                const {attributeAppliesToChannels} = globalAttributes;

                await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
                await expect(globalAttributes.displayNameInput).toBeVisible();
                await attributeAppliesToChannels.expand();

                await expect(attributeAppliesToChannels.requiredToggle).toBeChecked();
                await expect(attributeAppliesToChannels.missingValuesBanner).toBeVisible();

                await attributeAppliesToChannels.setRequired(false);
                await expect(attributeAppliesToChannels.requiredToggle).not.toBeChecked();

                await globalAttributes.save();
                await expect(page).toHaveURL(new RegExp(`${GLOBAL_ATTRIBUTES_PATH}$`));

                const reloaded = await getChannelPropertyField(adminClient, pair.channelField.id);
                expect(reloaded.attrs?.required).not.toBe(true);
            } finally {
                await archiveChannels(adminClient, [channel]);
                await deleteChannelAttributeField(adminClient, pair);
            }
        });

        /**
         * @objective Verify create mode (no field ID yet) hides both Notify and View
         * channel list -- nothing can have a value for a field that doesn't exist
         * yet, so a notified admin would have nothing to set, and the list would
         * just be every active channel rather than a targeted one.
         */
        test('create mode hides both Notify and View channel list', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeRequiredGateFields(adminClient);
            await pw.initSetup();

            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {globalAttributes} = systemConsolePage;
            const {attributeAppliesToChannels} = globalAttributes;

            await globalAttributes.gotoNewAttribute();
            await globalAttributes.setDisplayName(fieldName('create_mode', suffix));
            await attributeAppliesToChannels.addResource();

            await expect(attributeAppliesToChannels.missingValuesBanner).toBeVisible();
            await expect(attributeAppliesToChannels.viewChannelListButton).toHaveCount(0);
            await expect(attributeAppliesToChannels.notifyChannelAdminsButton).toHaveCount(0);
        });

        /**
         * @objective Verify View channel list surfaces the specific channel and admin
         * missing the value, links to its System Console detail page, and drops the
         * channel from the list once it is no longer missing.
         */
        test('View channel list finds the channel and its admin, and drops it once compliant', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeRequiredGateFields(adminClient);

            const {team, user} = await pw.initSetup();
            const pair = await createChannelAttributeField(adminClient, fieldName('list', suffix), {
                options: ['Value A'],
            });

            // The modal pages 20 channels sorted by name, and channels left by earlier
            // specs would push this one off the first page, so backfill them first.
            await backfillChannelAttribute(adminClient, pair.channelField, optionId(pair.templateField, 'Value A'));

            const channel = await createComplianceChannel(adminClient, team, suffix, `Missing Value ${suffix}`);
            await adminClient.addToChannel(user.id, channel.id);
            await adminClient.updateChannelMemberSchemeRoles(channel.id, user.id, true, true);

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);
                const {page, globalAttributes} = systemConsolePage;
                const {attributeAppliesToChannels} = globalAttributes;

                await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
                await expect(globalAttributes.displayNameInput).toBeVisible();
                await attributeAppliesToChannels.expand();

                let modal = await attributeAppliesToChannels.openChannelsWithoutValueModal();
                const row = modal.row(channel.display_name);
                await expect(row).toBeVisible();
                await expect(row).toContainText(user.username);
                await expect(row.getByRole('link', {name: channel.display_name})).toHaveAttribute(
                    'href',
                    `/admin_console/user_management/channels/${channel.id}`,
                );
                await modal.close();

                // # Set the value directly via the API, then reopen the modal (a fresh
                // # mount, so it refetches rather than reusing a stale page).
                await adminClient.patchPropertyValues('access_control', 'channel', channel.id, [
                    {field_id: pair.channelField.id, value: optionId(pair.templateField, 'Value A')},
                ] as Parameters<typeof adminClient.patchPropertyValues>[3]);

                modal = await attributeAppliesToChannels.openChannelsWithoutValueModal();
                await expect(modal.row(channel.display_name)).toHaveCount(0);
                await modal.close();
            } finally {
                await archiveChannels(adminClient, [channel]);
                await deleteChannelAttributeField(adminClient, pair);
            }
        });

        /**
         * @objective Verify Required becomes settable once every channel has a value.
         *
         * The missing-values scan is server-wide, so the test first backfills every
         * active local channel (without setting Required) to make the table fully
         * compliant regardless of what other specs left on the server.
         */
        test('once every channel has a value, Required can be turned on', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeRequiredGateFields(adminClient);

            const {team} = await pw.initSetup();
            const pair = await createChannelAttributeField(adminClient, fieldName('compliant', suffix), {
                options: ['Value A'],
                displayName: `Compliant ${suffix}`,
            });
            const channel = await createComplianceChannel(adminClient, team, suffix);
            const value = optionId(pair.templateField, 'Value A');
            await adminClient.patchPropertyValues('access_control', 'channel', channel.id, [
                {field_id: pair.channelField.id, value},
            ] as Parameters<typeof adminClient.patchPropertyValues>[3]);

            try {
                // Default channels (town-square, off-topic) from this and earlier specs'
                // initSetup teams stay active and have no value for a brand-new field,
                // so backfill them without setting Required.
                await backfillChannelAttribute(adminClient, pair.channelField, value);

                const {systemConsolePage} = await pw.testBrowser.login(adminUser);
                const {page, globalAttributes} = systemConsolePage;
                const {attributeAppliesToChannels} = globalAttributes;

                await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
                await expect(globalAttributes.displayNameInput).toBeVisible();
                await attributeAppliesToChannels.expand();

                const summary = await adminClient.getChannelAttributeComplianceSummary(
                    'access_control',
                    pair.channelField.id,
                );
                expect(summary.missing_channel_count).toBe(0);
                await expect(attributeAppliesToChannels.missingValuesBanner).toBeHidden();

                await attributeAppliesToChannels.setRequired(true);
                await expect(attributeAppliesToChannels.requiredToggle).toBeChecked();

                await globalAttributes.save();
                await expect(page).toHaveURL(new RegExp(`${GLOBAL_ATTRIBUTES_PATH}$`));

                const reloaded = await getChannelPropertyField(adminClient, pair.channelField.id);
                expect(reloaded.attrs?.required).toBe(true);
            } finally {
                await archiveChannels(adminClient, [channel]);
                await deleteChannelAttributeField(adminClient, pair);
            }
        });

        /**
         * @objective Verify a channel attribute created on channels directly (no
         * template) is edited as the saved attribute it is: the banner counts its
         * own channels and offers Notify and View channel list, rather than the
         * create-mode copy that asks to save it first.
         */
        test('a standalone channel attribute is edited as itself, not as an unsaved one', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeAttributes(adminClient);

            const {team} = await pw.initSetup();
            const field = await createAttribute(adminClient, attributeName('standalone', suffix), {
                type: 'select',
                options: ['Value A'],
            });

            // Keeps this channel on the modal's first page; see the View channel list test.
            await backfillChannelAttribute(adminClient, field, attributeOptionId(field, 'Value A'));

            const channel = await createComplianceChannel(adminClient, team, suffix, `Standalone Missing ${suffix}`);

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);
                const {page, globalAttributes} = systemConsolePage;
                const {attributeAppliesToChannels} = globalAttributes;

                await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${field.id}`);
                await expect(globalAttributes.displayNameInput).toBeVisible();
                await attributeAppliesToChannels.expand();

                // * The banner treats the attribute as saved: no create-mode copy, both actions offered
                await expect(attributeAppliesToChannels.missingValuesBanner).toBeVisible();
                await expect(attributeAppliesToChannels.missingValuesBanner).not.toContainText(
                    'Save this attribute first',
                );
                await expect(attributeAppliesToChannels.notifyChannelAdminsButton).toBeVisible();
                await expect(attributeAppliesToChannels.viewChannelListButton).toBeVisible();

                // * The list is this attribute's own: the channel is in it until it has a value
                let modal = await attributeAppliesToChannels.openChannelsWithoutValueModal();
                await expect(modal.row(channel.display_name)).toBeVisible();
                await modal.close();

                await adminClient.patchPropertyValues('access_control', 'channel', channel.id, [
                    {field_id: field.id, value: attributeOptionId(field, 'Value A')},
                ] as Parameters<typeof adminClient.patchPropertyValues>[3]);

                modal = await attributeAppliesToChannels.openChannelsWithoutValueModal();
                await expect(modal.row(channel.display_name)).toHaveCount(0);
                await modal.close();
            } finally {
                await archiveChannels(adminClient, [channel]);
                await deleteAttributes(adminClient, [field]);
            }
        });

        /**
         * @objective Verify an attribute that is already Required can be switched off
         * and back on before saving: only a saved off -> on is refused, so undoing an
         * unsaved change is not blocked and does not raise the warning.
         */
        test('an already-required attribute can be turned off and back on before saving', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeRequiredGateFields(adminClient);

            const {team} = await pw.initSetup();
            const channel = await createComplianceChannel(adminClient, team, suffix);
            await adminClient.deleteChannel(channel.id);

            const pair = await createChannelAttributeField(adminClient, fieldName('undo', suffix), {
                options: ['Value A'],
                required: true,
            });
            await adminClient.unarchiveChannel(channel.id);

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);
                const {page, globalAttributes} = systemConsolePage;
                const {attributeAppliesToChannels} = globalAttributes;

                await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
                await expect(globalAttributes.displayNameInput).toBeVisible();
                await attributeAppliesToChannels.expand();

                // * Saved Required, with the already-required copy
                await expect(attributeAppliesToChannels.requiredToggle).toBeChecked();
                await expect(attributeAppliesToChannels.missingValuesBanner).toContainText('New channels must set one');

                // # Off, then back on, without saving
                await attributeAppliesToChannels.clickRequired();
                await expect(attributeAppliesToChannels.requiredToggle).not.toBeChecked();
                await attributeAppliesToChannels.clickRequired();

                // * Back on, and nothing was refused
                await expect(attributeAppliesToChannels.requiredToggle).toBeChecked();
                await expect(attributeAppliesToChannels.missingValuesBanner).not.toHaveClass(/warning/);
                await expect(attributeAppliesToChannels.missingValuesBanner).toContainText('New channels must set one');
            } finally {
                await archiveChannels(adminClient, [channel]);
                await deleteChannelAttributeField(adminClient, pair);
            }
        });

        /**
         * @objective Verify View channel list pages through every channel missing a
         * value: Previous and Next are shown with Close, Next reaches past the first
         * 20, and on the last page Next is disabled rather than opening an empty page.
         */
        test('View channel list pages past the first 20 channels', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            await purgeRequiredGateFields(adminClient);

            const {team} = await pw.initSetup();
            const pair = await createChannelAttributeField(adminClient, fieldName('paging', suffix), {
                options: ['Value A'],
            });

            // The count is server-wide; these 21 guarantee more than one page.
            const pagingChannels: Channel[] = [];

            try {
                for (let i = 0; i < 21; i++) {
                    pagingChannels.push(
                        await createComplianceChannel(adminClient, team, `${suffix}p${i}`, `Paging ${suffix} ${i}`),
                    );
                }

                const {systemConsolePage} = await pw.testBrowser.login(adminUser);
                const {page, globalAttributes} = systemConsolePage;
                const {attributeAppliesToChannels} = globalAttributes;

                await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
                await expect(globalAttributes.displayNameInput).toBeVisible();
                await attributeAppliesToChannels.expand();

                const modal = await attributeAppliesToChannels.openChannelsWithoutValueModal();

                // * The pager is shown alongside Close
                await expect(modal.footerCloseButton).toBeVisible();
                await expect(modal.rows).toHaveCount(20);
                await expect(modal.previousButton).toBeDisabled();
                await expect(modal.nextButton).toBeEnabled();

                const header = await modal.container.getByText(/channels? still need/).innerText();
                const total = Number(header.match(/(\d+) channels? still need/)?.[1]);
                expect(total).toBeGreaterThan(20);
                const lastPage = Math.ceil(total / 20) - 1;
                const firstRow = await modal.rows.first().innerText();

                // # Page forward to the last page. Previous enabling marks each page loaded.
                for (let p = 1; p <= lastPage; p++) {
                    await modal.nextButton.click();
                    await expect(modal.previousButton).toBeEnabled();
                }

                // * The last page has its remaining rows, and Next stops there
                await expect(modal.rows).toHaveCount(total - lastPage * 20);
                await expect(modal.rows.first()).not.toHaveText(firstRow);
                await expect(modal.nextButton).toBeDisabled();

                await modal.close();
            } finally {
                await archiveChannels(adminClient, pagingChannels);
                await deleteChannelAttributeField(adminClient, pair);
            }
        });
    },
);
