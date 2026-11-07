// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {ClientError} from '@mattermost/client';
import type {PropertyField} from '@mattermost/types/properties';

import {Client4} from 'mattermost-redux/client';

import ModalController from 'components/modal_controller';

import {act, renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';

import ClassificationAttribute from './classification_attribute';

const mockSetNavigationBlocked = jest.fn();
jest.mock('actions/admin_actions', () => ({
    setNavigationBlocked: (blocked: boolean) => {
        mockSetNavigationBlocked(blocked);
        return {type: 'SET_NAVIGATION_BLOCKED', blocked};
    },
}));

const TEMPLATE: PropertyField = {
    id: 'template_field_id_123456789',
    group_id: 'group_id_1234567890123456',
    name: 'classification',
    type: 'rank',
    target_type: 'system',
    target_id: '',
    object_type: 'template',
    attrs: {
        options: [
            {id: 'lvl1', name: 'UNCLASSIFIED', color: '#007A33', rank: 1},
            {id: 'lvl2', name: 'SECRET', color: '#C8102E', rank: 2},
        ],
    },
    create_at: 1,
    update_at: 1,
    delete_at: 0,
} as unknown as PropertyField;

function channelField(attrs: Record<string, unknown> = {}): PropertyField {
    return {
        ...TEMPLATE,
        id: 'channel_field_id_1234567890',
        object_type: 'channel',
        linked_field_id: TEMPLATE.id,
        permission_values: 'admin',
        attrs,
    } as unknown as PropertyField;
}

// The `clearance` user field Classification Markings links to the template. Its
// name is not the template's, which is the only reason this page has to show it.
function userField(overrides: Partial<PropertyField> = {}): PropertyField {
    return {
        ...TEMPLATE,
        id: 'user_field_id_123456789012',
        name: 'clearance',
        object_type: 'user',
        linked_field_id: TEMPLATE.id,
        attrs: {},
        ...overrides,
    } as unknown as PropertyField;
}

// The page loads the template, the channel field, and the template's other linked
// fields with one paged call per object type.
function mockLoad(existingChannelField?: PropertyField, linkedUserFields: PropertyField[] = []) {
    return jest.spyOn(Client4, 'getPropertyFields').mockImplementation(async (_group, objectType) => {
        if (objectType === 'template') {
            return [TEMPLATE];
        }
        if (objectType === 'user') {
            return linkedUserFields;
        }
        return existingChannelField ? [existingChannelField] : [];
    });
}

// Channels needs an Enterprise Advanced licence as well as its flag, or
// useAllowedResourceTypes leaves the channel scope out of the linked-field
// lookup altogether -- which would make every "no channel row" assertion below
// true for the wrong reason.
function render() {
    return renderWithContext(
        <>
            <ClassificationAttribute/>
            <ModalController/>
        </>,
        {entities: {general: {
            config: {FeatureFlagChannelAttributes: 'true', FeatureFlagChannelAttributesRequired: 'true'},
            license: {IsLicensed: 'true', SkuShortName: 'advanced'},
        }}},
    );
}

describe('ClassificationAttribute', () => {
    beforeEach(() => {
        jest.clearAllMocks();

        // The Channels row's missing-values banner (MM-70717) fetches on
        // every render; stub it to zero so it never blocks the Required
        // toggle in the tests below, none of which are about that fetch.
        jest.spyOn(Client4, 'getChannelAttributeComplianceSummary').mockResolvedValue({
            required: false,
            missing_channel_count: 0,
            shared_channel_count: 0,
            unique_admin_count: 0,
            channels_without_admin_count: 0,
            message_preview: '',
        });
    });

    it('shows the definition without offering any way to edit it', async () => {
        mockLoad();

        render();

        expect(await screen.findByRole('heading', {name: 'Edit Classification Attribute'})).toBeInTheDocument();
        expect(await screen.findByTestId('classificationAttributeName')).toHaveValue('Classification');
        expect(screen.getByTestId('classificationAttributeName')).toBeDisabled();
        expect(screen.getByTestId('classificationAttributeUniqueName')).toHaveTextContent('classification');
        expect(screen.getByTestId('classificationAttributeType')).toHaveTextContent('Ranked');
        expect(screen.getByTestId('classificationAttributeType')).toBeDisabled();
        expect(screen.getByTestId('classificationAttributeLevels')).toHaveTextContent('1');
        expect(screen.getByTestId('classificationAttributeLevels')).toHaveTextContent('UNCLASSIFIED');
        expect(screen.getByTestId('classificationAttributeLevels')).toHaveTextContent('2');
        expect(screen.getByTestId('classificationAttributeLevels')).toHaveTextContent('SECRET');
        expect(screen.getByText('Presets and marking colors are configured on Classification Markings.')).toBeInTheDocument();
        expect(screen.getByTestId('classificationAttributeMarkingsLink')).toHaveAttribute(
            'href',
            '/admin_console/site_config/classification_markings',
        );
        expect(screen.getByTestId('classificationAttributeMarkingsLink')).toHaveTextContent('Open');
    });

    it('paints option chips with contrasting text so a bright marking stays readable', async () => {
        jest.spyOn(Client4, 'getPropertyFields').mockImplementation(async (_group, objectType) => {
            if (objectType === 'template') {
                return [{
                    ...TEMPLATE,
                    attrs: {
                        options: [
                            {id: 'lvl1', name: 'UNCLASSIFIED', color: '#007A33', rank: 1},
                            {id: 'lvl2', name: 'TOP SECRET//SCI', color: '#FFCC00', rank: 2},
                        ],
                    },
                } as PropertyField];
            }
            return [];
        });

        render();

        const chips = await screen.findAllByText(/UNCLASSIFIED|TOP SECRET\/\/SCI/);
        expect(chips[0].closest('.ClassificationAttribute__optionChip')).toHaveStyle({color: '#FFFFFF'});
        expect(chips[1].closest('.ClassificationAttribute__optionChip')).toHaveStyle({color: '#000000'});
    });

    it('says so when classification has not been set up yet', async () => {
        jest.spyOn(Client4, 'getPropertyFields').mockResolvedValue([]);

        render();

        expect(await screen.findByTestId('classificationAttributeMissing')).toBeInTheDocument();
        expect(screen.queryByTestId('classificationAttributeName')).not.toBeInTheDocument();
    });

    it('refuses to describe a wrong-typed template named classification', async () => {
        // * The name alone is not this feature's: a text attribute called
        // `classification` belongs to Attribute Management, and rendering it here
        // would label it "Ranked" and let Save attach channel settings to it.
        jest.spyOn(Client4, 'getPropertyFields').mockImplementation(async (_group, objectType) => {
            return objectType === 'template' ? [{...TEMPLATE, type: 'text', attrs: {}} as PropertyField] : [];
        });

        render();

        expect(await screen.findByTestId('classificationAttributeConflict')).toHaveTextContent(
            'An attribute named "classification" already exists but is not part of classification.',
        );
        expect(screen.queryByTestId('classificationAttributeType')).not.toBeInTheDocument();
        expect(screen.queryByTestId('saveSetting')).not.toBeInTheDocument();
    });

    it('refuses to adopt a channel field named classification that is linked elsewhere', async () => {
        // * Its options come from whatever template it is linked to, so the Applies-to
        // card would be editing another attribute's channel behaviour.
        mockLoad({...channelField(), linked_field_id: 'some_other_template_id'});

        render();

        expect(await screen.findByTestId('classificationAttributeConflict')).toBeInTheDocument();
        expect(screen.queryByTestId('channelsResourceRow')).not.toBeInTheDocument();
        expect(screen.queryByTestId('saveSetting')).not.toBeInTheDocument();
    });

    it('treats a 404 from the property routes as absent rather than broken', async () => {
        // How those routes say "no such field", and both loads have legitimate reasons
        // to hit it: classification may not be set up, or set up without applying to
        // channels. Surfacing an error here told an admin the page was broken.
        const notFound = new ClientError('https://example.com', {
            message: 'Not found',
            status_code: 404,
            url: '/api/v4/properties/groups/access_control/template/fields',
        });
        jest.spyOn(Client4, 'getPropertyFields').mockRejectedValue(notFound);

        render();

        expect(await screen.findByTestId('classificationAttributeMissing')).toBeInTheDocument();
        expect(screen.queryByTestId('classificationAttributeLoadError')).not.toBeInTheDocument();
    });

    it('shows the channel field as absent when only that lookup 404s', async () => {
        const notFound = new ClientError('https://example.com', {
            message: 'Not found',
            status_code: 404,
            url: '/api/v4/properties/groups/access_control/channel/fields',
        });
        jest.spyOn(Client4, 'getPropertyFields').mockImplementation(async (_group, objectType) => {
            if (objectType === 'template') {
                return [TEMPLATE];
            }
            throw notFound;
        });

        render();

        // Classification exists, it just does not apply to channels yet.
        expect(await screen.findByTestId('appliesToAddResource')).toBeInTheDocument();
        expect(screen.queryByTestId('classificationAttributeLoadError')).not.toBeInTheDocument();
    });

    it('surfaces the server message when a load genuinely fails', async () => {
        // Suppress the expected console.error from the load failure this test triggers.
        const consoleSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        jest.spyOn(Client4, 'getPropertyFields').mockRejectedValue(new ClientError('https://example.com', {
            message: 'Property group not found.',
            status_code: 500,
            url: '/api/v4/properties/groups/access_control/template/fields',
        }));

        render();

        // The canned copy cannot say which call failed, so the reason is shown.
        expect(await screen.findByTestId('classificationAttributeLoadError')).toHaveTextContent('Property group not found.');

        consoleSpy.mockRestore();
    });

    it('reads an existing channel field back into the row', async () => {
        mockLoad(channelField({required: true, actions: ['display_label_header']}));

        render();

        expect(await screen.findByTestId('channelsResourceRow')).toBeInTheDocument();
        expect(screen.getByTestId('channelsResourceRowSummary')).toHaveTextContent('Required · Display: Header');
        expect(screen.getByTestId('channelsResourceLocation-display_label_header')).toBeChecked();
    });

    it('offers Add resource when classification does not apply to channels', async () => {
        mockLoad();

        render();

        expect(await screen.findByTestId('appliesToAddResource')).toBeInTheDocument();
        expect(screen.queryByTestId('channelsResourceRow')).not.toBeInTheDocument();
    });

    it('gives the linked user field a home under the name it actually carries', async () => {
        // Attribute Management lists no row for a linked field, so the `clearance`
        // name Classification Markings created is otherwise nowhere to be found.
        mockLoad(undefined, [userField()]);

        render();

        const rows = within(await screen.findByTestId('appliesToReadOnlyResources')).getAllByRole('listitem');
        expect(rows).toHaveLength(1);
        expect(rows[0]).toHaveTextContent('Applied to Users as clearance');

        // Not the template's own name: the two differ, which is the point of the row.
        expect(rows[0]).not.toHaveTextContent('classification');
    });

    it('leaves the channel field out of the read-only list, since its own editable row already represents it', async () => {
        const getPropertyFields = mockLoad(channelField({required: true, actions: ['display_label_header']}), [userField()]);

        render();

        expect(await screen.findByTestId('channelsResourceRow')).toBeInTheDocument();

        // A check on the fixture rather than on behaviour: channel is among the
        // scopes the linked-field lookup lists (that call passes no target id,
        // unlike the by-name channel lookup), so the template's channel field
        // really does come back and really is being excluded below.
        expect(getPropertyFields).toHaveBeenCalledWith('access_control', 'channel', expect.objectContaining({targetType: 'system'}));

        const rows = within(screen.getByTestId('appliesToReadOnlyResources')).getAllByRole('listitem');
        expect(rows).toHaveLength(1);
        expect(rows[0]).toHaveTextContent('Applied to Users as clearance');
    });

    it('keeps the empty state when the template applies to nothing yet', async () => {
        mockLoad();

        render();

        expect(await screen.findByTestId('appliesToEmpty')).toBeInTheDocument();
        expect(screen.queryByTestId('appliesToReadOnlyResources')).not.toBeInTheDocument();
    });

    it('treats a 404 from the linked-field listing as nothing linked', async () => {
        // Same convention as the two lookups above: 404 is how the property routes
        // say "no such field", not that the page is broken. The empty state alone
        // would also hold without that convention, so the spy is what separates
        // "handled" from "logged as a failure and rendered the same way".
        const consoleSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        const notFound = new ClientError('https://example.com', {
            message: 'Not found',
            status_code: 404,
            url: '/api/v4/properties/groups/access_control/user/fields',
        });
        jest.spyOn(Client4, 'getPropertyFields').mockImplementation(async (_group, objectType) => {
            if (objectType === 'template') {
                return [TEMPLATE];
            }
            if (objectType === 'user') {
                throw notFound;
            }
            return [];
        });

        render();

        expect(await screen.findByTestId('appliesToEmpty')).toBeInTheDocument();
        expect(screen.queryByTestId('appliesToReadOnlyResources')).not.toBeInTheDocument();
        expect(screen.queryByTestId('classificationAttributeLoadError')).not.toBeInTheDocument();
        expect(consoleSpy).not.toHaveBeenCalled();
    });

    it('keeps the page usable when the linked-field listing genuinely fails, and loses only its rows', async () => {
        // That listing decides one read-only sentence and nothing else on the page,
        // so it runs outside the load the page waits on: inside it, any non-404 from
        // the user or post scope would replace the levels, the channel row and Save
        // with the load-error screen.
        const consoleSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        const existingChannelField = channelField({required: true, actions: ['display_label_header']});
        const getPropertyFields = jest.spyOn(Client4, 'getPropertyFields').mockImplementation(async (_group, objectType) => {
            if (objectType === 'template') {
                return [TEMPLATE];
            }
            if (objectType === 'user') {
                throw new ClientError('https://example.com', {
                    message: 'Internal Server Error',
                    status_code: 500,
                    url: '/api/v4/properties/groups/access_control/user/fields',
                });
            }
            return [existingChannelField];
        });

        render();

        // The listing runs in its own effect, so the page can be on screen before it
        // has failed -- waiting on the page alone would make the "no read-only rows"
        // assertion below a statement about the state the page starts in.
        await waitFor(() => {
            expect(getPropertyFields).toHaveBeenCalledWith('access_control', 'user', expect.objectContaining({targetType: 'system'}));
        });
        await act(async () => {});

        // Everything the page is for, none of which the failed listing has any say
        // over. Spelled out rather than left to the load-error check alone, because
        // the error screen is not the only way this could go wrong.
        expect(screen.getByTestId('classificationAttributeLevels')).toHaveTextContent('UNCLASSIFIED');
        expect(screen.getByTestId('classificationAttributeLevels')).toHaveTextContent('SECRET');
        expect(screen.getByTestId('channelsResourceRow')).toBeInTheDocument();
        expect(screen.getByTestId('channelsResourceRowSummary')).toHaveTextContent('Required · Display: Header');
        expect(screen.getByTestId('saveSetting')).toBeInTheDocument();
        expect(screen.getByTestId('classificationAttributeCancelLink')).toBeInTheDocument();
        expect(screen.queryByTestId('classificationAttributeLoadError')).not.toBeInTheDocument();

        // The only casualty. The same fixture plus a listed user field produces the
        // row in the test above, so its absence here is the failure and nothing else.
        expect(screen.queryByTestId('appliesToReadOnlyResources')).not.toBeInTheDocument();
        expect(screen.queryByText(/Applied to Users as/)).not.toBeInTheDocument();

        consoleSpy.mockRestore();
    });

    it('creates the channel field when the resource is added and saved', async () => {
        mockLoad();
        const createSpy = jest.spyOn(Client4, 'createPropertyField').mockResolvedValue(channelField());

        render();

        await userEvent.click(await screen.findByTestId('appliesToAddResource'));
        await userEvent.click(screen.getByTestId('channelsResourceLocation-display_label_header'));
        await userEvent.click(screen.getByTestId('saveSetting'));

        await waitFor(() => {
            expect(createSpy).toHaveBeenCalledWith(
                'access_control',
                'channel',
                expect.objectContaining({
                    linked_field_id: TEMPLATE.id,
                    permission_values: 'admin',
                    attrs: {actions: ['display_label_header']},
                }),
            );
        });
    });

    it('keeps Save inert until something actually changes', async () => {
        // Every save is a full write of the channel keys, so an idle click would
        // rewrite the field with what it already holds.
        mockLoad(channelField({required: true, actions: ['display_label_header']}));
        const patchSpy = jest.spyOn(Client4, 'patchPropertyField').mockResolvedValue(channelField());

        render();

        await waitFor(() => expect(screen.getByTestId('channelsResourceRow')).toBeInTheDocument());
        expect(screen.getByTestId('saveSetting')).toBeDisabled();

        await userEvent.click(screen.getByTestId('channelsResourceRequired-button'));
        expect(screen.getByTestId('saveSetting')).toBeEnabled();

        await userEvent.click(screen.getByTestId('saveSetting'));

        // Back to inert once the write lands, which is what the e2e helper waits on.
        await waitFor(() => expect(patchSpy).toHaveBeenCalled());
        await waitFor(() => expect(screen.getByTestId('saveSetting')).toBeDisabled());
    });

    it('patches an existing field rather than creating a second one', async () => {
        // The off states are written explicitly because the server merges attrs; the
        // shape of that patch is covered in channel_field_payload.test.
        mockLoad(channelField({required: true, actions: ['display_label_header']}));
        const patchSpy = jest.spyOn(Client4, 'patchPropertyField').mockResolvedValue(channelField());
        const createSpy = jest.spyOn(Client4, 'createPropertyField');

        render();

        await userEvent.click(await screen.findByTestId('channelsResourceRequired-button'));
        await userEvent.click(screen.getByTestId('channelsResourceLocation-display_label_header'));
        await userEvent.click(screen.getByTestId('saveSetting'));

        await waitFor(() => {
            expect(patchSpy).toHaveBeenCalledWith(
                'access_control',
                'channel',
                'channel_field_id_1234567890',
                {attrs: {required: false, change_policy: 'any', editable: null, actions: []}},
            );
        });
        expect(createSpy).not.toHaveBeenCalled();
    });

    it('asks before removing the resource, and does nothing when cancelled', async () => {
        mockLoad(channelField({actions: ['display_label_header']}));
        const deleteSpy = jest.spyOn(Client4, 'deletePropertyField').mockResolvedValue({status: 'OK'});

        render();

        await userEvent.click(await screen.findByTestId('channelsResourceRowRemove'));

        expect(await screen.findByText('Stop applying Classification to channels?')).toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));

        expect(deleteSpy).not.toHaveBeenCalled();
        expect(screen.getByTestId('channelsResourceRow')).toBeInTheDocument();
    });

    it('deletes the channel field once the removal is confirmed', async () => {
        mockLoad(channelField({actions: ['display_label_header']}));
        const deleteSpy = jest.spyOn(Client4, 'deletePropertyField').mockResolvedValue({status: 'OK'});

        render();

        await userEvent.click(await screen.findByTestId('channelsResourceRowRemove'));
        await userEvent.click(await screen.findByRole('button', {name: 'Remove and delete values'}));

        await waitFor(() => {
            expect(deleteSpy).toHaveBeenCalledWith('access_control', 'channel', 'channel_field_id_1234567890');
        });
        await waitFor(() => {
            expect(screen.queryByTestId('channelsResourceRow')).not.toBeInTheDocument();
        });
    });

    it('surfaces a failed save and keeps the form as it was', async () => {
        mockLoad(channelField({actions: ['display_label_header']}));
        jest.spyOn(Client4, 'patchPropertyField').mockRejectedValue(new Error('nope'));

        render();

        await userEvent.click(await screen.findByTestId('channelsResourceRequired-button'));
        await userEvent.click(screen.getByTestId('saveSetting'));

        expect(await screen.findByTestId('classificationAttributeSaveError')).toBeInTheDocument();
        expect(screen.getByTestId('channelsResourceRow')).toBeInTheDocument();
    });
});
