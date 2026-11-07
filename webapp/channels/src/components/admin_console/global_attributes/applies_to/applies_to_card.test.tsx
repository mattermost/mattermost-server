// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';

import AppliesToCard from './applies_to_card';
import {DEFAULT_CHANNEL_RESOURCE_CONFIG} from './channels';
import useChannelMissingValues from './channels/use_channel_missing_values';

// Isolates this suite from the MM-70717 missing-values fetch -- covered by
// use_channel_missing_values.test.ts and channels_resource_settings.test.tsx.
jest.mock('./channels/use_channel_missing_values');

const mockUseChannelMissingValues = jest.mocked(useChannelMissingValues);

describe('AppliesToCard', () => {
    beforeEach(() => {
        mockUseChannelMissingValues.mockReturnValue({loading: false, failed: false, summary: null, reload: jest.fn()});
    });

    it('starts with no resource and offers to add one', () => {
        renderWithContext(
            <AppliesToCard
                channelResource={null}
                onChannelResourceChange={jest.fn()}
            />,
        );

        expect(screen.getByTestId('appliesToEmpty')).toBeInTheDocument();
        expect(screen.getByTestId('appliesToAddResource')).toBeInTheDocument();
        expect(screen.queryByTestId('channelsResourceRow')).not.toBeInTheDocument();
    });

    it('adds a Channels resource with the server defaults', async () => {
        const onChannelResourceChange = jest.fn();
        renderWithContext(
            <AppliesToCard
                channelResource={null}
                onChannelResourceChange={onChannelResourceChange}
            />,
        );

        await userEvent.click(screen.getByTestId('appliesToAddResource'));

        expect(onChannelResourceChange).toHaveBeenCalledWith(DEFAULT_CHANNEL_RESOURCE_CONFIG);
    });

    it('renders the row and hides Add resource once channels are covered', () => {
        renderWithContext(
            <AppliesToCard
                channelResource={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChannelResourceChange={jest.fn()}
            />,
        );

        expect(screen.getByTestId('channelsResourceRow')).toBeInTheDocument();

        // Channels is the only resource type this card offers.
        expect(screen.queryByTestId('appliesToAddResource')).not.toBeInTheDocument();
    });

    it('clears the resource when the row is removed', async () => {
        const onChannelResourceChange = jest.fn();
        renderWithContext(
            <AppliesToCard
                channelResource={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChannelResourceChange={onChannelResourceChange}
            />,
        );

        await userEvent.click(screen.getByTestId('channelsResourceRowRemove'));

        expect(onChannelResourceChange).toHaveBeenCalledWith(null);
    });

    it('hands removal to the host when it has a saved field to delete', async () => {
        // Clearing the form is right while the resource is unsaved. A host editing an
        // attribute that already exists owns the removal instead, because it deletes a
        // field and the values on it.
        const onChannelResourceChange = jest.fn();
        const onChannelResourceRemove = jest.fn();
        renderWithContext(
            <AppliesToCard
                channelResource={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChannelResourceChange={onChannelResourceChange}
                onChannelResourceRemove={onChannelResourceRemove}
            />,
        );

        await userEvent.click(screen.getByTestId('channelsResourceRowRemove'));

        expect(onChannelResourceRemove).toHaveBeenCalled();
        expect(onChannelResourceChange).not.toHaveBeenCalled();
    });

    it('does not offer to add a resource while disabled', async () => {
        const onChannelResourceChange = jest.fn();
        renderWithContext(
            <AppliesToCard
                channelResource={null}
                onChannelResourceChange={onChannelResourceChange}
                disabled={true}
            />,
        );

        await userEvent.click(screen.getByTestId('appliesToAddResource'));

        expect(onChannelResourceChange).not.toHaveBeenCalled();
    });

    it('names the resource and the field name for each read-only resource', () => {
        // The name is the point: it can differ from the attribute's own, and it is
        // what a policy spells, so a row that only said "Users" would not help.
        renderWithContext(
            <AppliesToCard
                channelResource={null}
                onChannelResourceChange={jest.fn()}
                readOnlyResources={[
                    {type: 'user', name: 'clearance'},
                    {type: 'post', name: 'post_marking'},
                ]}
            />,
        );

        const rows = within(screen.getByTestId('appliesToReadOnlyResources')).getAllByRole('listitem');
        expect(rows).toHaveLength(2);
        expect(rows[0]).toHaveTextContent('Applied to Users as clearance');
        expect(rows[1]).toHaveTextContent('Applied to Posts as post_marking');

        // Each row also carries its resource's icon. Asserted structurally
        // rather than by role or name, because it is decoration: it is
        // aria-hidden, repeats the resource the sentence beside it already
        // names, and has nothing a query by role or accessible name could find.
        // Without this, deleting it from the card would change nothing any test
        // here can see.
        for (const row of rows) {
            const icon = row.querySelector('.GlobalAttributesResourceIcon');
            expect(icon).not.toBeNull();
            expect(icon).toHaveAttribute('aria-hidden', 'true');
        }
    });

    it('does not call itself empty while a read-only resource is listed', () => {
        renderWithContext(
            <AppliesToCard
                channelResource={null}
                onChannelResourceChange={jest.fn()}
                readOnlyResources={[{type: 'user', name: 'clearance'}]}
            />,
        );

        expect(screen.queryByText('This attribute does not apply to any resource yet.')).not.toBeInTheDocument();

        // Channels is still unclaimed, so it can still be added.
        expect(screen.getByTestId('appliesToAddResource')).toBeInTheDocument();
    });

    it('still calls itself empty when there is no resource of either kind', () => {
        renderWithContext(
            <AppliesToCard
                channelResource={null}
                onChannelResourceChange={jest.fn()}
                readOnlyResources={[]}
            />,
        );

        expect(screen.getByText('This attribute does not apply to any resource yet.')).toBeInTheDocument();
        expect(screen.queryByTestId('appliesToReadOnlyResources')).not.toBeInTheDocument();
    });

    it('shows the channels row alongside the read-only rows', () => {
        renderWithContext(
            <AppliesToCard
                channelResource={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChannelResourceChange={jest.fn()}
                readOnlyResources={[{type: 'user', name: 'clearance'}]}
            />,
        );

        expect(within(screen.getByTestId('appliesToReadOnlyResources')).getByRole('listitem')).
            toHaveTextContent('Applied to Users as clearance');
        expect(screen.getByTestId('channelsResourceRow')).toBeInTheDocument();
        expect(screen.queryByText('This attribute does not apply to any resource yet.')).not.toBeInTheDocument();
    });
});
