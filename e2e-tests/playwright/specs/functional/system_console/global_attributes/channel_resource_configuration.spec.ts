// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * System Console — applying an attribute to channels.
 *
 * The Channels resource row writes the channel keys onto a linked channel field,
 * and the channel surfaces honour them. Every test here configures through the
 * console UI and then checks what a user sees, with no API seeding in between.
 */

import {expect, test} from '@mattermost/playwright-lib';

import {configureChannelAttribute, deleteChannelFieldIfExists, findChannelField} from './applies_to_helpers';
import {
    createGlobalAttributeField,
    createLinkedDependentField,
    deleteGlobalAttributeFieldIfExists,
    deleteLinkedDependentField,
    requireGlobalAttributesEnabled,
} from './global_attributes_helpers';

test.describe(
    'System Console - applying an attribute to channels',
    {tag: ['@system_console', '@channel_attributes']},
    () => {
        test.describe.configure({mode: 'serial'});

        /**
         * @objective Ensure the Channels row writes every channel key onto a linked channel field.
         */
        test('creates a linked channel field carrying the configured keys', async ({pw}) => {
            // The Required toggle this test flips only renders in the System Console
            // while this kill switch is on (model.IsChannelAttributesRequiredEnabled).
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            const displayName = `Program ${suffix}`;
            let name = '';

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);

                // # Apply it to channels, required, shown in the header and the banner
                name = await configureChannelAttribute(
                    systemConsolePage,
                    {
                        displayName,
                        required: true,
                        displayLocations: ['display_label_header', 'display_banner_top'],
                    },
                    adminClient,
                );

                // * The linked channel field carries every configured key
                const channelField = await findChannelField(adminClient, name);
                expect(channelField).toBeDefined();
                expect(channelField?.linked_field_id).toBeTruthy();
                expect(channelField?.object_type).toBe('channel');

                // The console has no control for this, so the payload has to pin it: the
                // server would otherwise default a channel field to any member.
                expect(channelField?.permission_values).toBe('admin');
                expect(channelField?.attrs?.required).toBe(true);
                expect(channelField?.attrs?.actions).toEqual(['display_label_header', 'display_banner_top']);

                // * Options stay on the template; a linked field never carries its own
                expect(channelField?.attrs?.options).toBeUndefined();
            } finally {
                await deleteChannelFieldIfExists(adminClient, name);
                await deleteGlobalAttributeFieldIfExists(adminClient, name);
            }
        });

        /**
         * @objective Ensure an attribute configured through the System Console is asked for at
         * channel creation and rendered afterwards, with no API seeding anywhere in the path.
         */
        test('an attribute configured here is required when creating a channel and shown in its header', async ({
            pw,
        }) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            let name = '';

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);

                name = await configureChannelAttribute(
                    systemConsolePage,
                    {
                        displayName: `Caveat ${suffix}`,
                        type: 'Select',
                        options: ['NOFORN'],
                        required: true,
                        displayLocations: ['display_label_header'],
                    },
                    adminClient,
                );

                // # Create a channel as the same admin, who is a member of a team
                const {team} = await pw.initSetup();
                const {channelsPage} = await pw.testBrowser.login(adminUser);
                await channelsPage.goto(team.name);
                await channelsPage.toBeVisible();

                const modal = await channelsPage.openNewChannelModal();

                // * The attribute is asked for, because it was marked required here
                await expect(channelsPage.page.getByTestId(`channelAttributeRow-${name}`)).toBeVisible();

                await modal.fillDisplayName(`Attr Console ${suffix}`);
                await channelsPage.page.getByTestId(`channelAttribute-${name}`).click();
                await channelsPage.page.getByText('NOFORN', {exact: true}).click();
                await modal.create();
                await expect(modal.container).not.toBeVisible();

                // * The value renders as a header chip, because it was designated here
                await expect(channelsPage.centerView.header.attributes.chip('NOFORN')).toBeVisible();
            } finally {
                await deleteChannelFieldIfExists(adminClient, name);
                await deleteGlobalAttributeFieldIfExists(adminClient, name);
            }
        });

        /**
         * @objective Ensure the Banner display location renders a banner and nothing else.
         */
        test('renders a Banner-only attribute as a banner, with no chip', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            let name = '';

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);

                name = await configureChannelAttribute(
                    systemConsolePage,
                    {
                        displayName: `Marking ${suffix}`,
                        type: 'Select',
                        options: ['RESTRICTED'],
                        required: true,
                        displayLocations: ['display_banner_top'],
                    },
                    adminClient,
                );

                const {team} = await pw.initSetup();
                const {channelsPage} = await pw.testBrowser.login(adminUser);
                await channelsPage.goto(team.name);
                await channelsPage.toBeVisible();

                const modal = await channelsPage.openNewChannelModal();
                await modal.fillDisplayName(`Attr Banner ${suffix}`);
                await channelsPage.page.getByTestId(`channelAttribute-${name}`).click();
                await channelsPage.page.getByText('RESTRICTED', {exact: true}).click();
                await modal.create();
                await expect(modal.container).not.toBeVisible();

                // * The value drives the banner and stays out of the header
                await expect(channelsPage.page.getByTestId('channel_banner_text')).toContainText('RESTRICTED');
                await expect(channelsPage.centerView.header.attributes.chip('RESTRICTED')).toHaveCount(0);
            } finally {
                await deleteChannelFieldIfExists(adminClient, name);
                await deleteGlobalAttributeFieldIfExists(adminClient, name);
            }
        });

        /**
         * @objective Ensure an attribute with no display location is stored and shown nowhere.
         */
        test('stores a value for an attribute with no display location and renders it nowhere', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            let name = '';

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);

                name = await configureChannelAttribute(
                    systemConsolePage,
                    {
                        displayName: `Quiet ${suffix}`,
                        type: 'Select',
                        options: ['QUIET'],
                        required: true,
                    },
                    adminClient,
                );

                const {team} = await pw.initSetup();
                const {channelsPage} = await pw.testBrowser.login(adminUser);
                await channelsPage.goto(team.name);
                await channelsPage.toBeVisible();

                const displayName = `Attr Quiet ${suffix}`;
                const modal = await channelsPage.openNewChannelModal();
                await modal.fillDisplayName(displayName);
                await channelsPage.page.getByTestId(`channelAttribute-${name}`).click();
                await channelsPage.page.getByText('QUIET', {exact: true}).click();
                await modal.create();
                await expect(modal.container).not.toBeVisible();

                // * The value reached the store
                const channel = await adminClient.getChannelByName(
                    team.id,
                    displayName.toLowerCase().replace(/\s+/g, '-'),
                );
                const values = await adminClient.getPropertyValues('access_control', 'channel', channel.id);
                expect(values?.length).toBeGreaterThan(0);

                // * And is rendered on no display surface
                await expect(channelsPage.centerView.header.attributes.chip('QUIET')).toHaveCount(0);
                await expect(channelsPage.page.getByTestId('channel_banner_container')).toHaveCount(0);

                // * Channel Info still lists it: the display locations govern the
                // header and banner, never whether an admin can reach a value
                const info = await channelsPage.openChannelInfo();
                await expect(info.attributes.chip(name)).toHaveText('QUIET');
                await expect(info.attributes.editButton(name)).toBeVisible();
            } finally {
                await deleteChannelFieldIfExists(adminClient, name);
                await deleteGlobalAttributeFieldIfExists(adminClient, name);
            }
        });

        /**
         * @objective Ensure "Cannot be changed once set" locks the value in Channel Info.
         */
        test('locks the value in Channel Info when the change policy forbids changes', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            let name = '';

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);

                name = await configureChannelAttribute(
                    systemConsolePage,
                    {
                        displayName: `Locked ${suffix}`,
                        type: 'Select',
                        options: ['FINAL'],
                        required: true,
                        changePolicy: 'Cannot be changed once set',
                    },
                    adminClient,
                );

                // * The console wrote both keys: change_policy, and the editable key the
                // * channel UI still reads
                const channelField = await findChannelField(adminClient, name);
                expect(channelField?.attrs?.change_policy).toBe('never');
                expect(channelField?.attrs?.editable).toBe(false);

                const {team} = await pw.initSetup();
                const {channelsPage} = await pw.testBrowser.login(adminUser);
                await channelsPage.goto(team.name);
                await channelsPage.toBeVisible();

                const modal = await channelsPage.openNewChannelModal();
                await modal.fillDisplayName(`Attr Locked ${suffix}`);
                await channelsPage.page.getByTestId(`channelAttribute-${name}`).click();
                await channelsPage.page.getByText('FINAL', {exact: true}).click();
                await modal.create();
                await expect(modal.container).not.toBeVisible();

                // * Once set at creation the value is read-only, even to a system admin
                const info = await channelsPage.openChannelInfo();
                await expect(info.attributes.lock(name)).toBeVisible();
                await expect(info.attributes.editButton(name)).toHaveCount(0);
            } finally {
                await deleteChannelFieldIfExists(adminClient, name);
                await deleteGlobalAttributeFieldIfExists(adminClient, name);
            }
        });

        /**
         * @objective Ensure the channel-admin tier the console pins reaches the channel
         * field, so an attribute configured here is not editable by an ordinary member.
         *
         * There is no control for this, which is what makes the assertion worth keeping:
         * the tier is only correct as long as the payload keeps pinning it.
         */
        test('keeps a console-configured attribute out of a plain member reach', async ({pw}) => {
            await pw.ensureFeatureFlag({ChannelAttributes: true, ChannelAttributesRequired: true});
            const {adminUser, adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            let name = '';

            try {
                const {systemConsolePage} = await pw.testBrowser.login(adminUser);

                name = await configureChannelAttribute(
                    systemConsolePage,
                    {
                        displayName: `Restricted ${suffix}`,
                        type: 'Select',
                        options: ['SET'],
                        required: true,
                    },
                    adminClient,
                );

                const channelField = await findChannelField(adminClient, name);
                expect(channelField?.permission_values).toBe('admin');

                const {team, user} = await pw.initSetup();
                const {channelsPage} = await pw.testBrowser.login(adminUser);
                await channelsPage.goto(team.name);
                await channelsPage.toBeVisible();

                const channelDisplayName = `Attr Restricted ${suffix}`;
                const modal = await channelsPage.openNewChannelModal();
                await modal.fillDisplayName(channelDisplayName);
                await channelsPage.page.getByTestId(`channelAttribute-${name}`).click();
                await channelsPage.page.getByText('SET', {exact: true}).click();
                await modal.create();
                await expect(modal.container).not.toBeVisible();

                const channel = await adminClient.getChannelByName(
                    team.id,
                    channelDisplayName.toLowerCase().replace(/\s+/g, '-'),
                );
                await adminClient.addToChannel(user.id, channel.id);

                // # Look at the same attribute as an ordinary member
                const asMember = await pw.testBrowser.login(user);
                await asMember.channelsPage.goto(team.name, channel.name);
                await asMember.channelsPage.toBeVisible();

                // * The member reads the value but is offered no way to change it
                const info = await asMember.channelsPage.openChannelInfo();
                await expect(info.attributes.chip(name)).toHaveText('SET');
                await expect(info.attributes.editButton(name)).toHaveCount(0);
            } finally {
                await deleteChannelFieldIfExists(adminClient, name);
                await deleteGlobalAttributeFieldIfExists(adminClient, name);
            }
        });
        /**
         * @objective Regression for MM-70890: a template with ldap/saml sync attrs applied to
         * a channel must NOT copy those attrs onto the channel-linked field. Without the fix,
         * checkSyncLock blocks all writes to the channel field with 403 because the field
         * carries attrs.ldap/attrs.saml even though SAML/LDAP sync never writes channel values.
         */
        test('channel-linked field does not inherit ldap/saml sync attrs from the template', async ({pw}) => {
            const {adminClient} = await requireGlobalAttributesEnabled(pw);

            const suffix = pw.random.id();
            const name = `sync_attr_regression_${suffix}`;
            let templateId = '';
            let channelFieldId = '';

            try {
                // # Create a template with ldap and saml attrs via API
                const template = await createGlobalAttributeField(adminClient, name, {
                    type: 'text',
                    attrs: {ldap: 'sAMAccountName', saml: 'employeeID'},
                });
                templateId = template.id;

                // # Link a channel field to that template
                const channelField = await createLinkedDependentField(
                    adminClient,
                    name,
                    template.id,
                    'text',
                    'channel',
                );
                channelFieldId = channelField.id;

                // * The channel field must not carry ldap or saml attrs
                expect(channelField.attrs?.ldap).toBeUndefined();
                expect(channelField.attrs?.saml).toBeUndefined();
            } finally {
                if (channelFieldId) {
                    await deleteLinkedDependentField(adminClient, channelFieldId, 'channel');
                }
                if (templateId) {
                    await deleteGlobalAttributeFieldIfExists(adminClient, name);
                }
            }
        });
    },
);
