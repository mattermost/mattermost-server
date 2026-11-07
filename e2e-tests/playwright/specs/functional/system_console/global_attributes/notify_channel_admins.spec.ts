// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * System Console — "Notify all channel admins" (MM-70717).
 *
 * The one part of this feature not worth much as a unit test and everything
 * as an end-to-end one: does a real message land in a real inbox. Delivery is
 * asynchronous (fired via a background goroutine on the server), so
 * assertions on the resulting DM poll rather than check immediately.
 */

import {expect, test, getAdminClient} from '@mattermost/playwright-lib';

import {requireGlobalAttributesEnabled, setGlobalAttributesFeatureFlag} from './global_attributes_helpers';
import {
    ATTRIBUTE_DETAILS_PATH,
    createChannelAttributeField,
    createComplianceChannel,
    deleteChannelAttributeField,
    fieldName,
    purgeRequiredGateFields,
} from './required_gate_helpers';

const SYSTEM_BOT_USERNAME = 'system-bot';

test.describe('System Console - notify all channel admins', {tag: ['@system_console', '@channel_attributes']}, () => {
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
     * @objective Verify confirming Notify actually delivers a DM from the System
     * Bot to the channel's admin, naming the channel.
     */
    test('confirming Notify sends a real DM to the channel admin', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

        const suffix = pw.random.id();
        await purgeRequiredGateFields(adminClient);

        const {team, user} = await pw.initSetup();
        const pair = await createChannelAttributeField(adminClient, fieldName('notify_happy', suffix), {
            options: ['Value A'],
        });
        const channel = await createComplianceChannel(adminClient, team, suffix, `Notify Channel ${suffix}`);
        await adminClient.addToChannel(user.id, channel.id);
        await adminClient.updateChannelMemberSchemeRoles(channel.id, user.id, true, true);

        try {
            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {page, globalAttributes} = systemConsolePage;
            const {attributeAppliesToChannels} = globalAttributes;

            await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
            await expect(globalAttributes.displayNameInput).toBeVisible();
            await attributeAppliesToChannels.expand();

            const modal = await attributeAppliesToChannels.openNotifyChannelAdminsModal();
            await expect(modal.messagePreview).not.toHaveText('');
            await modal.confirm();

            // * The banner reflects a successful send
            await expect(attributeAppliesToChannels.missingValuesBanner).toContainText('Notification queued');

            // # Switch to the channel admin's own session and check their DM inbox
            const {channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name, `@${SYSTEM_BOT_USERNAME}`);

            // * A real DM arrived from the System Bot, naming the channel
            await expect(channelsPage.centerView.postViews.filter({hasText: channel.display_name})).toBeVisible({
                timeout: 15000,
            });
        } finally {
            await deleteChannelAttributeField(adminClient, pair);
        }
    });

    /**
     * @objective Verify cancelling the confirmation sends nothing.
     */
    test('cancelling the confirmation sends nothing', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

        const suffix = pw.random.id();
        await purgeRequiredGateFields(adminClient);

        const {team, user} = await pw.initSetup();
        const pair = await createChannelAttributeField(adminClient, fieldName('notify_cancel', suffix), {
            options: ['Value A'],
        });
        const channel = await createComplianceChannel(adminClient, team, suffix, `Notify Cancel Channel ${suffix}`);
        await adminClient.addToChannel(user.id, channel.id);
        await adminClient.updateChannelMemberSchemeRoles(channel.id, user.id, true, true);

        try {
            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {page, globalAttributes} = systemConsolePage;
            const {attributeAppliesToChannels} = globalAttributes;

            // The DM send only ever starts server-side in response to this POST, so
            // its absence is what proves nothing can still be in flight.
            const notifyRequests: string[] = [];
            page.on('request', (request) => {
                if (request.method() === 'POST' && request.url().endsWith('/missing_values/notify')) {
                    notifyRequests.push(request.url());
                }
            });

            await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
            await expect(globalAttributes.displayNameInput).toBeVisible();
            await attributeAppliesToChannels.expand();

            const modal = await attributeAppliesToChannels.openNotifyChannelAdminsModal();
            await modal.cancel();

            // * No notify request left the browser
            expect(notifyRequests).toHaveLength(0);

            // * No DM exists. The system bot is created lazily on first use, so if it
            // does not exist yet nothing can have been sent by it.
            const systemBot = await adminClient.getUserByUsername(SYSTEM_BOT_USERNAME).catch(() => null);
            if (systemBot) {
                const dm = await adminClient.createDirectChannel([user.id, systemBot.id]);
                const {posts} = await adminClient.getPosts(dm.id);
                const matching = Object.values(posts).filter((post) => post.message.includes(channel.display_name));
                expect(matching).toHaveLength(0);
            }
        } finally {
            await deleteChannelAttributeField(adminClient, pair);
        }
    });

    /**
     * @objective Verify the confirmation calls out channels with no channel admin
     * instead of silently dropping them.
     */
    test('the confirmation calls out channels with no channel admin', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

        const suffix = pw.random.id();
        await purgeRequiredGateFields(adminClient);

        const {team, user} = await pw.initSetup();
        const pair = await createChannelAttributeField(adminClient, fieldName('notify_no_admin', suffix), {
            options: ['Value A'],
        });

        const channelWithAdmin = await createComplianceChannel(adminClient, team, `${suffix}a`, `Has Admin ${suffix}`);
        await adminClient.addToChannel(user.id, channelWithAdmin.id);
        await adminClient.updateChannelMemberSchemeRoles(channelWithAdmin.id, user.id, true, true);

        // The creator (adminUser) is always made a channel admin on creation --
        // demote them so this channel genuinely has none.
        const channelNoAdmin = await createComplianceChannel(adminClient, team, `${suffix}b`, `No Admin ${suffix}`);
        await adminClient.updateChannelMemberSchemeRoles(channelNoAdmin.id, adminUser.id, true, false);

        try {
            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {page, globalAttributes} = systemConsolePage;
            const {attributeAppliesToChannels} = globalAttributes;

            await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
            await expect(globalAttributes.displayNameInput).toBeVisible();
            await attributeAppliesToChannels.expand();

            const modal = await attributeAppliesToChannels.openNotifyChannelAdminsModal();

            // * The no-admin channel is called out, not silently dropped
            await expect(modal.container).toContainText("no channel admin and won't be notified");

            await modal.confirm();
        } finally {
            await deleteChannelAttributeField(adminClient, pair);
        }
    });

    /**
     * @objective Verify the notify preview and the DM name the attribute by its display
     * name -- taken from the template when the linked channel field has none of its
     * own -- and that the preview shows the formatted message, not its markdown.
     */
    test('names the attribute by its display name in the preview and the DM', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

        const suffix = pw.random.id();
        await purgeRequiredGateFields(adminClient);

        const displayName = `Data Owner ${suffix}`;
        const {team, user} = await pw.initSetup();
        const pair = await createChannelAttributeField(adminClient, fieldName('notify_name', suffix), {
            options: ['Value A'],
            displayName,
        });
        const channel = await createComplianceChannel(adminClient, team, suffix, `Notify Name Channel ${suffix}`);
        await adminClient.addToChannel(user.id, channel.id);
        await adminClient.updateChannelMemberSchemeRoles(channel.id, user.id, true, true);

        try {
            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {page, globalAttributes} = systemConsolePage;
            const {attributeAppliesToChannels} = globalAttributes;

            await page.goto(`${ATTRIBUTE_DETAILS_PATH}/${pair.templateField.id}`);
            await expect(globalAttributes.displayNameInput).toBeVisible();
            await attributeAppliesToChannels.expand();

            const modal = await attributeAppliesToChannels.openNotifyChannelAdminsModal();

            // * The preview names the display name, rendered bold rather than as **markdown**
            await expect(modal.messagePreview.locator('strong')).toHaveText(displayName);
            await expect(modal.messagePreview).not.toContainText('**');
            await expect(modal.messagePreview).not.toContainText(pair.channelField.name);

            await modal.confirm();
            await expect(attributeAppliesToChannels.missingValuesBanner).toContainText('Notification queued');

            // * The DM the channel admin receives names the display name too
            await expect
                .poll(
                    async () => {
                        // The bot is created lazily by the background send, so it may not exist yet.
                        const systemBot = await adminClient.getUserByUsername(SYSTEM_BOT_USERNAME).catch(() => null);
                        if (!systemBot) {
                            return '';
                        }
                        const dm = await adminClient.createDirectChannel([user.id, systemBot.id]);
                        const {posts} = await adminClient.getPosts(dm.id);
                        return (
                            Object.values(posts).find((post) => post.message.includes(channel.display_name))?.message ??
                            ''
                        );
                    },
                    {timeout: 15000},
                )
                .toContain(`**${displayName}**`);
        } finally {
            await deleteChannelAttributeField(adminClient, pair);
        }
    });
});
