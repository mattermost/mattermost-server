// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {PropertyField} from '@mattermost/types/properties';

import {expect, test} from '@mattermost/playwright-lib';

import {
    DISPLAY_BANNER_TOP,
    DISPLAY_LABEL_HEADER,
    DISPLAY_LABEL_INFO,
    assertNoForeignRequiredAttributes,
    attributeName,
    createAttribute,
    deleteAttributes,
    optionId,
    purgeAttributes,
    setChannelValue,
} from './helpers';

test.describe('Channel attribute lifecycle', {tag: ['@channel_attributes']}, () => {
    test.describe.configure({mode: 'serial'});

    /**
     * @objective Verify a required attribute is asked for at creation and blocks it until filled.
     */
    test('blocks channel creation until a required attribute is filled', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        await pw.skipIfNoLicense();

        const {adminClient, user, team} = await pw.initSetup();
        const suffix = pw.random.id();
        const created: PropertyField[] = [];

        try {
            await purgeAttributes(adminClient);
            await assertNoForeignRequiredAttributes(adminClient);

            const required = await createAttribute(adminClient, attributeName('mandatory', suffix), {
                options: ['ALPHA'],
                actions: [DISPLAY_LABEL_HEADER, DISPLAY_LABEL_INFO],
                required: true,
            });
            const optional = await createAttribute(adminClient, attributeName('discretionary', suffix), {
                options: ['BETA'],
                actions: [DISPLAY_LABEL_INFO],
            });
            created.push(required, optional);

            const {page, channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name);
            await channelsPage.toBeVisible();

            const modal = await channelsPage.openNewChannelModal();

            // Required is asked for; optional is not.
            await expect(page.getByTestId(`channelAttributeRow-${required.name}`)).toBeVisible();
            await expect(page.getByTestId(`channelAttributeRow-${optional.name}`)).toHaveCount(0);

            const displayName = `Attr Required ${suffix}`;
            await modal.fillDisplayName(displayName);

            // The dialog blocks submission so the user finds out here; the server
            // refuses the same create independently.
            await expect(modal.createButton).toBeDisabled();

            await page.getByTestId(`channelAttribute-${required.name}`).click();
            await page.getByTestId('dropdownMenu').getByText('ALPHA', {exact: true}).click();

            await expect(modal.createButton).toBeEnabled();
            await modal.create();
            await expect(modal.container).not.toBeVisible();

            const channel = await adminClient.getChannelByName(team.id, displayName.toLowerCase().replace(/\s+/g, '-'));
            const values = await adminClient.getPropertyValues('access_control', 'channel', channel.id);
            expect(values?.find((value) => value.field_id === required.id)?.value).toBe(optionId(required, 'ALPHA'));
        } finally {
            await deleteAttributes(adminClient, created);
        }
    });

    /**
     * @objective Verify a value changed elsewhere reaches an open session without a reload.
     */
    test('updates an open session when a value changes, and when it is cleared', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        await pw.skipIfNoLicense();
        const {adminClient, user, team} = await pw.initSetup();
        const suffix = pw.random.id();
        const created: PropertyField[] = [];

        try {
            await purgeAttributes(adminClient);
            await assertNoForeignRequiredAttributes(adminClient);

            const program = await createAttribute(adminClient, attributeName('live', suffix), {
                options: ['BEFORE', 'AFTER'],
                actions: [DISPLAY_LABEL_HEADER, DISPLAY_LABEL_INFO],
            });
            created.push(program);

            const channel = await adminClient.createChannel({
                team_id: team.id,
                name: `attr-live-${suffix}`,
                display_name: `Attr Live ${suffix}`,
                type: 'O',
            } as Parameters<typeof adminClient.createChannel>[0]);
            await adminClient.addToChannel(user.id, channel.id);
            await setChannelValue(adminClient, channel.id, program, optionId(program, 'BEFORE'));

            const {page, channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name, channel.name);
            await channelsPage.toBeVisible();

            const chips = page.getByTestId('attributeChip');
            await expect(chips.filter({hasText: 'BEFORE'}).first()).toBeVisible();

            // Changed elsewhere, this session untouched.
            await setChannelValue(adminClient, channel.id, program, optionId(program, 'AFTER'));

            await expect(chips.filter({hasText: 'AFTER'}).first()).toBeVisible();
            await expect(chips.filter({hasText: 'BEFORE'})).toHaveCount(0);

            // The shape most easily got wrong: a clear returns a null-valued row,
            // not a delete event.
            await setChannelValue(adminClient, channel.id, program, null);

            await expect(page.getByTestId('attributeChip')).toHaveCount(0);
        } finally {
            await deleteAttributes(adminClient, created);
        }
    });

    /**
     * @objective Verify a required attribute left unset after creation is visible and recoverable from Channel Info.
     */
    test('shows a required attribute as unset and lets it be filled from Channel Info', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        await pw.skipIfNoLicense();
        const {adminClient, user, team} = await pw.initSetup();
        const suffix = pw.random.id();
        const created: PropertyField[] = [];

        try {
            await purgeAttributes(adminClient);
            await assertNoForeignRequiredAttributes(adminClient);

            // The channel comes first, because creating one that misses a required
            // attribute is refused now. What remains reachable is an attribute that
            // becomes required after the fact, which is how an existing channel ends up
            // short of its own requirement.
            const channel = await adminClient.createChannel({
                team_id: team.id,
                name: `attr-unset-${suffix}`,
                display_name: `Attr Unset ${suffix}`,
                type: 'O',
            } as Parameters<typeof adminClient.createChannel>[0]);
            await adminClient.addToChannel(user.id, channel.id);

            // The unset-required row and its edit path are admin-only in Channel Info,
            // so recovering it needs a channel admin, not a plain member.
            await adminClient.updateChannelMemberSchemeRoles(channel.id, user.id, true, true);
            await adminClient.deleteChannel(channel.id);

            const required = await createAttribute(adminClient, attributeName('unfilled', suffix), {
                options: ['RECOVERED'],
                actions: [DISPLAY_LABEL_INFO],
                required: true,
            });
            created.push(required);
            await adminClient.unarchiveChannel(channel.id);

            const {page, channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name, channel.name);
            await channelsPage.toBeVisible();
            await page.locator('#channel-info-btn').click();

            // The empty row is both the signal and the retry path.
            const row = page.getByTestId(`channelInfoAttributeRow-${required.name}`);
            await expect(row).toBeVisible();
            await expect(row).toContainText('Not set');

            // No chip while unset.
            await expect(page.getByTestId('attributeChip')).toHaveCount(0);

            await page.getByTestId(`channelInfoAttributeEdit-${required.name}`).click();
            await page.getByRole('menuitem', {name: 'RECOVERED', exact: true}).click();

            await expect(row).toContainText('RECOVERED');

            await expect
                .poll(async () => {
                    const values = await adminClient.getPropertyValues('access_control', 'channel', channel.id);
                    return values?.find((value) => value.field_id === required.id)?.value;
                })
                .toBe(optionId(required, 'RECOVERED'));
        } finally {
            await deleteAttributes(adminClient, created);
        }
    });

    /**
     * @objective Verify the banner renders attribute tokens, and that a manual banner text still wins.
     */
    test('renders a token banner and honours a manual override', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        await pw.skipIfNoLicense();
        const {adminClient, user, team} = await pw.initSetup();
        const suffix = pw.random.id();
        const created: PropertyField[] = [];

        try {
            await purgeAttributes(adminClient);
            await assertNoForeignRequiredAttributes(adminClient);

            const marking = await createAttribute(adminClient, attributeName('bannertoken', suffix), {
                options: ['RESTRICTED'],
                actions: [DISPLAY_BANNER_TOP],
                optionColors: {RESTRICTED: '#c8102e'},
                sortOrder: 0,
            });
            const program = await createAttribute(adminClient, attributeName('bannerprogram', suffix), {
                options: ['AURORA'],
                actions: [DISPLAY_LABEL_INFO],
                sortOrder: 1,
            });
            created.push(marking, program);

            const channel = await adminClient.createChannel({
                team_id: team.id,
                name: `attr-token-${suffix}`,
                display_name: `Attr Token ${suffix}`,
                type: 'O',
            } as Parameters<typeof adminClient.createChannel>[0]);
            await adminClient.addToChannel(user.id, channel.id);
            await setChannelValue(adminClient, channel.id, marking, optionId(marking, 'RESTRICTED'));
            await setChannelValue(adminClient, channel.id, program, optionId(program, 'AURORA'));

            const {page, channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name, channel.name);
            await channelsPage.toBeVisible();

            // No manual text: falls back to the value name.
            await expect(page.getByTestId('channel_banner_text')).toContainText('RESTRICTED');

            // A two-attribute template resolves against this channel.
            await adminClient.patchChannel(channel.id, {
                banner_info: {
                    enabled: true,
                    text: `{{${marking.name}}} · {{${program.name}}}`,
                    background_color: '#c8102e',
                },
            } as never);

            await expect(page.getByTestId('channel_banner_text')).toContainText('RESTRICTED · AURORA');

            // A literal passes through untouched.
            await adminClient.patchChannel(channel.id, {
                banner_info: {enabled: true, text: 'Handle with care', background_color: '#c8102e'},
            } as never);

            await expect(page.getByTestId('channel_banner_text')).toContainText('Handle with care');
        } finally {
            await deleteAttributes(adminClient, created);
        }
    });

    /**
     * @objective Verify a direct message shows no attribute chips even when values exist through the API.
     */
    test('shows no chips on a direct message', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        await pw.skipIfNoLicense();
        const {adminClient, user, team} = await pw.initSetup();
        const suffix = pw.random.id();
        const created: PropertyField[] = [];

        try {
            await purgeAttributes(adminClient);
            await assertNoForeignRequiredAttributes(adminClient);

            const marking = await createAttribute(adminClient, attributeName('dm', suffix), {
                options: ['PRIVATE'],
                actions: [DISPLAY_LABEL_HEADER, DISPLAY_LABEL_INFO],
            });
            created.push(marking);

            const other = await pw.createNewUserProfile(adminClient);
            await adminClient.addToTeam(team.id, other.id);

            const dm = await adminClient.createDirectChannel([user.id, other.id]);

            // Not blocked at the API level, but no DM surface displays them.
            await setChannelValue(adminClient, dm.id, marking, optionId(marking, 'PRIVATE'));

            const {page, channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name);
            await channelsPage.toBeVisible();
            await page.goto(`/${team.name}/messages/@${other.username}`);

            await expect(page.getByTestId('channelAttributeLabels-info-header')).toHaveCount(0);
            await expect(page.getByText('PRIVATE')).toHaveCount(0);
        } finally {
            await deleteAttributes(adminClient, created);
        }
    });

    /**
     * @objective Verify the configured colour reaches the chip, since a marking's colour is part of how it is read.
     */
    test('applies the configured colour to a chip', async ({pw}) => {
        await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
        await pw.skipIfNoLicense();
        const {adminClient, user, team} = await pw.initSetup();
        const suffix = pw.random.id();
        const created: PropertyField[] = [];

        try {
            await purgeAttributes(adminClient);
            await assertNoForeignRequiredAttributes(adminClient);

            const marking = await createAttribute(adminClient, attributeName('coloured', suffix), {
                options: ['DARKBG'],
                actions: [DISPLAY_LABEL_HEADER],
                optionColors: {DARKBG: '#1e325c'},
            });
            created.push(marking);

            const channel = await adminClient.createChannel({
                team_id: team.id,
                name: `attr-colour-${suffix}`,
                display_name: `Attr Colour ${suffix}`,
                type: 'O',
            } as Parameters<typeof adminClient.createChannel>[0]);
            await adminClient.addToChannel(user.id, channel.id);
            await setChannelValue(adminClient, channel.id, marking, optionId(marking, 'DARKBG'));

            const {page, channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name, channel.name);
            await channelsPage.toBeVisible();

            const chip = page.getByTestId('attributeChip').filter({hasText: 'DARKBG'}).first();
            await expect(chip).toBeVisible();
            await expect(chip).toHaveCSS('background-color', 'rgb(30, 50, 92)');

            // Contrast is derived, so a dark background must produce light text.
            await expect(chip).toHaveCSS('color', 'rgb(255, 255, 255)');
        } finally {
            await deleteAttributes(adminClient, created);
        }
    });
});
