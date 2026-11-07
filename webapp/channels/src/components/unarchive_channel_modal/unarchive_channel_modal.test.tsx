// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {PropertyField} from '@mattermost/types/properties';

import useMissingRequiredChannelAttributes from 'components/common/hooks/useMissingRequiredChannelAttributes';
import ConnectedUnarchiveChannelModal from 'components/unarchive_channel_modal';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {boardChannelsState} from 'utils/channel_kind_test_utils';
import {TestHelper} from 'utils/test_helper';

import UnarchiveChannelModal from './unarchive_channel_modal';

jest.mock('components/common/hooks/useMissingRequiredChannelAttributes');

function propertyField(overrides: Partial<PropertyField>): PropertyField {
    return {
        id: '',
        group_id: 'group1',
        name: '',
        type: 'text',
        target_id: '',
        target_type: 'system',
        object_type: 'channel',
        create_at: 0,
        update_at: 0,
        delete_at: 0,
        created_by: '',
        updated_by: '',
        attrs: {},
        ...overrides,
    };
}

const mockUseMissingRequiredChannelAttributes = jest.mocked(useMissingRequiredChannelAttributes);

describe('components/unarchive_channel_modal', () => {
    const channel = TestHelper.getChannelMock({
        create_at: 1508265709607,
        creator_id: 'zaktnt8bpbgu8mb6ez9k64r7sa',
        delete_at: 0,
        display_name: 'testing',
        header: 'test',
        id: 'owsyt8n43jfxjpzh9np93mx1wa',
        last_post_at: 1508265709635,
        name: 'testing',
        purpose: 'test',
        team_id: 'eatxocwc3bg9ffo9xyybnj4omr',
        type: 'O',
        update_at: 1508265709607,
    });

    const baseProps = {
        channel,
        actions: {
            unarchiveChannel: jest.fn(),
        },
        onExited: jest.fn(),
    };

    beforeEach(() => {
        mockUseMissingRequiredChannelAttributes.mockReturnValue({loading: false, missing: []});
    });

    test('should have called actions.unarchiveChannel when Unarchive is clicked', async () => {
        const unarchiveChannel = jest.fn().mockResolvedValue({data: true});
        const props = {...baseProps, actions: {unarchiveChannel}};
        renderWithContext(
            <UnarchiveChannelModal {...props}/>,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Unarchive'}));

        expect(unarchiveChannel).toHaveBeenCalledTimes(1);
        expect(unarchiveChannel).toHaveBeenCalledWith(props.channel.id);

        await waitFor(() => {
            expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        });
    });

    test('focuses the confirm button so Enter restores the channel', async () => {
        const unarchiveChannel = jest.fn().mockResolvedValue({data: true});
        renderWithContext(
            <UnarchiveChannelModal
                {...baseProps}
                actions={{unarchiveChannel}}
            />,
        );

        await waitFor(() => expect(screen.getByRole('button', {name: 'Unarchive'})).toHaveFocus());
        await userEvent.keyboard('{Enter}');

        expect(unarchiveChannel).toHaveBeenCalledWith(channel.id);
    });

    test('names a missing attribute by its label, capitalizing classification', () => {
        mockUseMissingRequiredChannelAttributes.mockReturnValue({
            loading: false,
            missing: [
                propertyField({id: 'f1', name: 'cost_center', attrs: {required: true, display_name: 'Cost Center'}}),
                propertyField({id: 'f2', name: 'classification', linked_field_id: 't1', attrs: {required: true}}),
            ],
        });

        renderWithContext(<UnarchiveChannelModal {...baseProps}/>);

        expect(screen.getByText(/This channel has no value for Cost Center, Classification\./)).toBeInTheDocument();
    });

    test('should have called props.onExited on Cancel', async () => {
        const onExited = jest.fn();
        renderWithContext(
            <UnarchiveChannelModal
                {...baseProps}
                onExited={onExited}
            />,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));

        await waitFor(() => {
            expect(onExited).toHaveBeenCalledTimes(1);
        });
    });

    test('shows no warning and the plain "Unarchive" label when no required attribute is missing', () => {
        renderWithContext(
            <UnarchiveChannelModal {...baseProps}/>,
        );

        expect(screen.getByRole('button', {name: 'Unarchive'})).toBeInTheDocument();
        expect(screen.queryByText('Missing required attribute values')).not.toBeInTheDocument();
    });

    test('warns and relabels the confirm button when a required attribute is missing, but does not block the restore', async () => {
        mockUseMissingRequiredChannelAttributes.mockReturnValue({
            loading: false,
            missing: [{
                id: 'field1',
                group_id: 'group1',
                name: 'cost_center',
                type: 'text',
                target_id: '',
                target_type: 'channel',
                object_type: 'channel',
                create_at: 0,
                update_at: 0,
                delete_at: 0,
                created_by: '',
                updated_by: '',
                attrs: {required: true, display_name: 'Cost center'},
            }],
        });

        const unarchiveChannel = jest.fn().mockResolvedValue({data: true});
        const props = {...baseProps, actions: {unarchiveChannel}};
        renderWithContext(
            <UnarchiveChannelModal {...props}/>,
        );

        expect(screen.getByText('Missing required attribute values')).toBeInTheDocument();
        const confirmButton = screen.getByRole('button', {name: 'Unarchive anyway'});
        expect(confirmButton).toBeInTheDocument();

        await userEvent.click(confirmButton);

        expect(unarchiveChannel).toHaveBeenCalledWith(props.channel.id);
    });

    test('stays open and shows the server error when unarchiveChannel fails', async () => {
        const unarchiveChannel = jest.fn().mockResolvedValue({error: {message: 'Something went wrong'}});
        const props = {...baseProps, actions: {unarchiveChannel}};
        renderWithContext(
            <UnarchiveChannelModal {...props}/>,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Unarchive'}));

        await waitFor(() => {
            expect(screen.getByText('Something went wrong')).toBeInTheDocument();
        });
        expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    test('says board for a board', () => {
        renderWithContext(
            <ConnectedUnarchiveChannelModal
                channel={channel}
                onExited={jest.fn()}
            />,
            boardChannelsState(channel.id),
        );

        expect(screen.getByText('Confirm UNARCHIVE Board')).toBeInTheDocument();
        expect(screen.getByText(/Are you sure you wish to unarchive the/)).toHaveTextContent('Are you sure you wish to unarchive the testing board?');
        expect(screen.queryByText(/channel/i)).not.toBeInTheDocument();
    });
});
