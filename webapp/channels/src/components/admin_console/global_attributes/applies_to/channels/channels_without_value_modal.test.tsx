// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {ChannelsMissingAttributeValueList} from '@mattermost/types/properties';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import ChannelsWithoutValueModal from './channels_without_value_modal';

import {fetchChannelsMissingValue} from '../../utils';

jest.mock('../../utils', () => ({
    __esModule: true,
    fetchChannelsMissingValue: jest.fn(),
}));

const mockedFetch = jest.mocked(fetchChannelsMissingValue);

function page(overrides: Partial<ChannelsMissingAttributeValueList> = {}): ChannelsMissingAttributeValueList {
    return {
        channels: [],
        total_count: 0,
        ...overrides,
    };
}

describe('ChannelsWithoutValueModal', () => {
    const onExited = jest.fn();

    beforeEach(() => {
        jest.clearAllMocks();
    });

    it('renders rows with a team-scoped link and comma-joined admins', async () => {
        mockedFetch.mockResolvedValue(page({
            total_count: 2,
            channels: [
                {
                    channel_id: 'c1',
                    channel_name: 'ops-planning',
                    channel_display_name: 'ops-planning',
                    channel_type: 'O',
                    team_id: 't1',
                    team_display_name: 'Team 1',
                    is_local: true,
                    channel_admins: [
                        {id: 'u1', username: 'maya', first_name: 'Maya', last_name: 'Chen', nickname: ''},
                        {id: 'u2', username: 'jonah', first_name: 'Jonah', last_name: 'Blake', nickname: ''},
                    ],
                },
                {
                    channel_id: 'c2',
                    channel_name: 'logistics-sync',
                    channel_display_name: 'logistics-sync',
                    channel_type: 'P',
                    team_id: 't1',
                    team_display_name: 'Team 1',
                    is_local: true,
                    channel_admins: [],
                },
            ],
        }));

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                attributeDisplayName='Cost center'
                totalCount={2}
                onExited={onExited}
            />,
        );

        await waitFor(() => expect(screen.getByText('ops-planning')).toBeInTheDocument());

        expect(screen.getByRole('link', {name: /ops-planning/})).toHaveAttribute('href', '/admin_console/user_management/channels/c1');
        expect(screen.getByText('Maya Chen, Jonah Blake')).toBeInTheDocument();
        expect(screen.getByText('No channel admin')).toBeInTheDocument();
    });

    it('shows the subtitle count from props before the first page resolves', () => {
        mockedFetch.mockReturnValue(new Promise(() => {})); // never resolves in this test

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                attributeDisplayName='Cost center'
                totalCount={26}
                onExited={onExited}
            />,
        );

        expect(screen.getByText('26 channels still need a Cost center value')).toBeInTheDocument();
    });

    it('shows an empty state when nothing is missing', async () => {
        mockedFetch.mockResolvedValue(page());

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                totalCount={0}
                onExited={onExited}
            />,
        );

        await waitFor(() => expect(screen.getByText('Every channel has a value')).toBeInTheDocument());
    });

    it('shows an error state with a retry that refetches', async () => {
        mockedFetch.mockRejectedValueOnce(new Error('boom'));
        mockedFetch.mockResolvedValueOnce(page({
            total_count: 1,
            channels: [{
                channel_id: 'c1',
                channel_name: 'ops-planning',
                channel_display_name: 'ops-planning',
                channel_type: 'O',
                team_id: 't1',
                team_display_name: 'Team 1',
                is_local: true,
                channel_admins: [],
            }],
        }));

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                totalCount={1}
                onExited={onExited}
            />,
        );

        await waitFor(() => expect(screen.getByText('The channel list couldn\'t be loaded.')).toBeInTheDocument());

        await userEvent.click(screen.getByRole('button', {name: 'Try again'}));

        await waitFor(() => expect(screen.getByText('ops-planning')).toBeInTheDocument());
    });

    it('drives the filter tabs from whole-list totals, not the current page', async () => {
        mockedFetch.mockResolvedValue(page({
            total_count: 30,
            channels: [{
                channel_id: 'c1',
                channel_name: 'ops-planning',
                channel_display_name: 'ops-planning',
                channel_type: 'O',
                team_id: 't1',
                team_display_name: 'Team 1',
                is_local: true,
                channel_admins: [],
            }],
        }));

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                totalCount={30}
                sharedCount={3}
                onExited={onExited}
            />,
        );

        await waitFor(() => expect(screen.getByText('ops-planning')).toBeInTheDocument());

        // The only row on this page is local, yet shared channels exist further on.
        expect(screen.getByRole('button', {name: 'All (30)'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Local (27)'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Shared (3)'})).toBeInTheDocument();
    });

    it('shows a page-level empty message, not the all-clear, when the filter matches nothing on this page', async () => {
        mockedFetch.mockResolvedValue(page({
            total_count: 30,
            channels: [{
                channel_id: 'c1',
                channel_name: 'ops-planning',
                channel_display_name: 'ops-planning',
                channel_type: 'O',
                team_id: 't1',
                team_display_name: 'Team 1',
                is_local: true,
                channel_admins: [],
            }],
        }));

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                totalCount={30}
                sharedCount={3}
                onExited={onExited}
            />,
        );

        await waitFor(() => expect(screen.getByText('ops-planning')).toBeInTheDocument());

        await userEvent.click(screen.getByRole('button', {name: 'Shared (3)'}));

        expect(screen.getByText('No matching channels on this page')).toBeInTheDocument();
        expect(screen.queryByText('Every channel has a value')).not.toBeInTheDocument();
    });

    it('calls onExited when Close is clicked', async () => {
        mockedFetch.mockResolvedValue(page());

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                totalCount={0}
                onExited={onExited}
            />,
        );

        await userEvent.click(screen.getByTestId('channelsWithoutValueModalClose'));

        expect(onExited).toHaveBeenCalled();
    });

    function rows(count: number, offset = 0) {
        return Array.from({length: count}, (_, i) => ({
            channel_id: `c${offset + i}`,
            channel_name: `ch-${offset + i}`,
            channel_display_name: `Channel ${offset + i}`,
            channel_type: 'O' as const,
            team_id: 't1',
            team_display_name: 'Team 1',
            is_local: true,
            channel_admins: [],
        }));
    }

    it('renders Previous and Next alongside Close, and pages to the 21st channel', async () => {
        mockedFetch.mockResolvedValueOnce(page({total_count: 21, channels: rows(20)}));
        mockedFetch.mockResolvedValueOnce(page({total_count: 21, channels: rows(1, 20)}));

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                totalCount={21}
                onExited={onExited}
            />,
        );

        await waitFor(() => expect(screen.getByText('Channel 19')).toBeInTheDocument());
        expect(screen.getByTestId('channelsWithoutValueModalClose')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Previous'})).toBeDisabled();

        await userEvent.click(screen.getByRole('button', {name: 'Next'}));

        await waitFor(() => expect(screen.getByText('Channel 20')).toBeInTheDocument());
        expect(mockedFetch).toHaveBeenLastCalledWith('field1', 1, 20);
        expect(screen.getByRole('button', {name: 'Next'})).toBeDisabled();
        expect(screen.getByRole('button', {name: 'Previous'})).toBeEnabled();
    });

    it('disables Next when exactly one full page is missing a value', async () => {
        mockedFetch.mockResolvedValue(page({total_count: 20, channels: rows(20)}));

        renderWithContext(
            <ChannelsWithoutValueModal
                fieldId='field1'
                totalCount={20}
                onExited={onExited}
            />,
        );

        await waitFor(() => expect(screen.getByText('Channel 19')).toBeInTheDocument());
        expect(screen.getByRole('button', {name: 'Next'})).toBeDisabled();
    });
});
