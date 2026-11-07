// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * System Console — classification's own attribute page.
 *
 * Classification's fields are created by the Classification Markings page, so the
 * create-only card on the New attribute page can never reach them. This page is the
 * only way to configure which resources classification applies to without the API,
 * which is what these tests exercise.
 */

import {expect, test, getAdminClient} from '@mattermost/playwright-lib';

import {
    deleteClassificationFieldsIfExist,
    setupClassificationWithChannelField,
} from '../../channels/channel_classification/helpers';

import {findChannelField} from './applies_to_helpers';
import {requireGlobalAttributesEnabled} from './global_attributes_helpers';

const FEATURE_FLAGS = {ChannelAttributes: true, ChannelAttributesRequired: true, ClassificationMarkings: true};

test.describe(
    'System Console - the classification attribute page',
    {tag: ['@system_console', '@channel_attributes']},
    () => {
        test.describe.configure({mode: 'serial'});

        test.afterAll(async () => {
            const {adminClient} = await getAdminClient();
            if (!adminClient) {
                return;
            }
            await deleteClassificationFieldsIfExist(adminClient);
        });

        /**
         * @objective Verify the definition is shown read-only, with the levels editor a link away.
         */
        test('shows the definition read-only and links to Classification Markings', async ({pw}) => {
            await pw.ensureFeatureFlag(FEATURE_FLAGS);
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const {levels} = await setupClassificationWithChannelField(adminClient);

            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {globalAttributes} = systemConsolePage;

            await globalAttributes.gotoClassificationAttribute();

            // * Every level is listed, and none of them is editable here
            for (const level of levels) {
                await expect(globalAttributes.classificationLevels).toContainText(level.name);
            }
            // The display name is shown, but as a disabled field.
            const displayName = systemConsolePage.page.getByTestId('classificationAttribute').getByRole('textbox');
            await expect(displayName).toHaveCount(1);
            await expect(displayName).not.toBeEditable();

            // * The one place levels can be changed is a link away
            await expect(globalAttributes.classificationMarkingsLink).toHaveHrefPath(
                '/admin_console/site_config/classification_markings',
            );
        });

        /**
         * @objective Verify a display location chosen here reaches the channel field and the channel header.
         */
        test('applies a header chip to channels once Header is chosen', async ({pw}) => {
            await pw.ensureFeatureFlag(FEATURE_FLAGS);
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const {levels} = await setupClassificationWithChannelField(adminClient);
            const {team, user} = await pw.initSetup();

            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {globalAttributes} = systemConsolePage;

            await globalAttributes.gotoClassificationAttribute();
            await globalAttributes.appliesToChannels.setDisplayLocations(['display_label_header']);
            await globalAttributes.saveInPlace();

            // * The choice landed on the linked channel field
            const channelField = await findChannelField(adminClient, 'classification');
            expect(channelField?.attrs?.actions).toEqual(['display_label_header']);

            // # Give a channel a classification, then look at it as a member
            const channel = await adminClient.createChannel({
                team_id: team.id,
                name: `classification-attr-${pw.random.id()}`,
                display_name: 'Classification Attr',
                type: 'O',
            } as Parameters<typeof adminClient.createChannel>[0]);
            await adminClient.addToChannel(user.id, channel.id);
            await adminClient.patchPropertyValues('access_control', 'channel', channel.id, [
                {field_id: channelField!.id, value: levels[0].id},
            ] as Parameters<typeof adminClient.patchPropertyValues>[3]);

            const {channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name, channel.name);
            await channelsPage.toBeVisible();

            // * The header carries the chip the console asked for
            await expect(channelsPage.centerView.header.attributes.chip(levels[0].name)).toBeVisible();
        });

        /**
         * @objective Verify classification is asked for at channel creation only once Required is on.
         *
         * Classification is a channel attribute like any other: the create dialog asks
         * only for required attributes, and an optional one is added later from Channel Info.
         */
        test('asks for classification at channel creation only once Required is on', async ({pw}) => {
            // The Required toggle this test flips only renders in the System Console
            // while this kill switch is on (model.IsChannelAttributesRequiredEnabled).
            await pw.ensureFeatureFlag(FEATURE_FLAGS);
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            await setupClassificationWithChannelField(adminClient);
            const {team} = await pw.initSetup();

            const {channelsPage} = await pw.testBrowser.login(adminUser);
            await channelsPage.goto(team.name);
            await channelsPage.toBeVisible();

            // * Optional attributes are not asked for at creation, so Create is free
            const modal = await channelsPage.openNewChannelModal();
            await modal.fillDisplayName(`Optional Classification ${pw.random.id()}`);
            await expect(channelsPage.page.getByTestId('channelAttribute-classification')).toHaveCount(0);
            await expect(modal.createButton).toBeEnabled();
            await modal.cancel();
        });

        /**
         * @objective Verify a channel classification field already marked required
         * demands a level before Create is enabled. The console toggle flow itself is
         * covered by required_gate.spec.ts.
         *
         * The helper creates the field optional, backfills every active channel with a
         * level, then PATCHes it to required. The server rejects that transition while
         * any active channel lacks a value, so the backfill is what lets it through on a
         * shared e2e server.
         */
        test('demands a classification level at creation once the field is required', async ({pw}) => {
            await pw.ensureFeatureFlag(FEATURE_FLAGS);
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const {levels} = await setupClassificationWithChannelField(adminClient, undefined, true);
            const {team} = await pw.initSetup();

            const {channelsPage} = await pw.testBrowser.login(adminUser);
            await channelsPage.goto(team.name);
            await channelsPage.toBeVisible();

            const modal = await channelsPage.openNewChannelModal();
            await modal.fillDisplayName(`Required Classification ${pw.random.id()}`);

            // * Now it is asked for, and holds up Create until a level is chosen
            await expect(channelsPage.page.getByTestId('channelAttribute-classification')).toBeVisible();
            await expect(modal.createButton).toBeDisabled();
            await channelsPage.page.getByTestId('channelAttribute-classification').click();
            await channelsPage.page.getByText(levels[0].name, {exact: true}).click();
            await expect(modal.createButton).toBeEnabled();
        });

        /**
         * @objective Verify unticking Banner actually stops the banner.
         *
         * Classification banners on a field carrying no display locations at all, which is
         * what keeps a server that predates this page behaving as it did. A configured
         * field has to be obeyed instead, or the Banner checkbox would do nothing.
         */
        test('stops bannering when the display locations exclude the banner', async ({pw}) => {
            await pw.ensureFeatureFlag(FEATURE_FLAGS);
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const {levels} = await setupClassificationWithChannelField(adminClient);
            const {team, user} = await pw.initSetup();

            const channelFieldBefore = await findChannelField(adminClient, 'classification');
            const channel = await adminClient.createChannel({
                team_id: team.id,
                name: `classification-banner-${pw.random.id()}`,
                display_name: 'Classification Banner',
                type: 'O',
            } as Parameters<typeof adminClient.createChannel>[0]);
            await adminClient.addToChannel(user.id, channel.id);
            await adminClient.patchPropertyValues('access_control', 'channel', channel.id, [
                {field_id: channelFieldBefore!.id, value: levels[0].id},
            ] as Parameters<typeof adminClient.patchPropertyValues>[3]);

            // # Configure Header only — deliberately not the banner
            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {globalAttributes} = systemConsolePage;
            await globalAttributes.gotoClassificationAttribute();
            await globalAttributes.appliesToChannels.setDisplayLocations(['display_label_header']);
            await globalAttributes.saveInPlace();

            const {page, channelsPage} = await pw.testBrowser.login(user);
            await channelsPage.goto(team.name, channel.name);
            await channelsPage.toBeVisible();

            // * The Header location took effect, and there is no banner even though
            // * the channel has a classification
            await expect(channelsPage.centerView.header.attributes.chip(levels[0].name)).toBeVisible();
            await expect(page.getByTestId('channel_banner_container')).toHaveCount(0);
        });

        /**
         * @objective Verify removing the resource asks first, and that a removal is not undone by an unrelated save.
         */
        test('removes the Channels resource only after confirming, and keeps it removed', async ({pw}) => {
            await pw.ensureFeatureFlag(FEATURE_FLAGS);
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            await setupClassificationWithChannelField(adminClient);

            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {globalAttributes} = systemConsolePage;
            const page = systemConsolePage.page;

            await globalAttributes.gotoClassificationAttribute();

            // # Ask to remove, then back out
            await globalAttributes.appliesToChannels.removeResource();
            await page.getByRole('button', {name: 'Cancel'}).click();

            // * Nothing was deleted
            await expect(globalAttributes.appliesToChannels.row).toBeVisible();
            expect(await findChannelField(adminClient, 'classification')).toBeDefined();

            // # Ask again and confirm
            await globalAttributes.appliesToChannels.removeResource();
            await page.getByRole('button', {name: 'Remove and delete values'}).click();

            // * The field is gone, and the card offers to add it back
            await expect(globalAttributes.appliesToChannels.addResourceButton).toBeVisible();
            expect(await findChannelField(adminClient, 'classification')).toBeUndefined();

            // * Saving the Classification Markings page does not reinstate it: the field is
            // * created on the transition into enabled, not on every save
            await page.goto('/admin_console/site_config/classification_markings');

            // Save only enables once something changes, so the save under test needs an
            // edit -- one with nothing to do with the Channels resource. The cell keeps the
            // typing local and only reports it on blur, so the edit has to be committed.
            const levelName = page.getByRole('textbox', {name: 'Classification level name'}).first();
            await levelName.fill('UNCLASSIFIED EDITED');
            await levelName.blur();

            await page.getByTestId('saveSetting').click();
            await expect(page.getByTestId('saveSetting')).toBeDisabled();

            expect(await findChannelField(adminClient, 'classification')).toBeUndefined();
        });

        /**
         * @objective Verify the missing-values banner and the notify preview call the
         * attribute Classification, as the page does, rather than "this attribute" or
         * its internal name.
         */
        test('names Classification in the missing-values banner and the notify preview', async ({pw}) => {
            await pw.ensureFeatureFlag(FEATURE_FLAGS);
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            await setupClassificationWithChannelField(adminClient);
            const {team} = await pw.initSetup();

            // A channel with no classification, so the banner has something to count.
            await adminClient.createChannel({
                team_id: team.id,
                name: `classification-missing-${pw.random.id()}`,
                display_name: 'Classification Missing',
                type: 'O',
            } as Parameters<typeof adminClient.createChannel>[0]);

            const {systemConsolePage} = await pw.testBrowser.login(adminUser);
            const {globalAttributes} = systemConsolePage;
            const {appliesToChannels} = globalAttributes;

            await globalAttributes.gotoClassificationAttribute();

            // * The banner names Classification
            await expect(appliesToChannels.missingValuesBanner).toContainText('a Classification value');
            await expect(appliesToChannels.missingValuesBanner).not.toContainText('this attribute');

            // * So does the notify preview, formatted rather than as markdown
            const modal = await appliesToChannels.openNotifyChannelAdminsModal();
            await expect(modal.container).toContainText('a Classification value');
            await expect(modal.messagePreview.locator('strong')).toHaveText('Classification');
            await expect(modal.messagePreview).not.toContainText('**');
            await modal.cancel();
        });
    },
);
