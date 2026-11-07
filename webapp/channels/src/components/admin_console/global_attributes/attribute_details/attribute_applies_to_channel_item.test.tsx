// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import AttributeAppliesToChannelItem from './attribute_applies_to_channel_item';

import {DEFAULT_CHANNEL_RESOURCE_CONFIG} from '../applies_to/channels';
import useChannelMissingValues from '../applies_to/channels/use_channel_missing_values';

// Isolates this suite from the MM-70717 missing-values fetch -- covered by
// use_channel_missing_values.test.ts and channels_resource_settings.test.tsx.
jest.mock('../applies_to/channels/use_channel_missing_values');

const mockUseChannelMissingValues = jest.mocked(useChannelMissingValues);

describe('AttributeAppliesToChannelItem', () => {
    const onRemove = jest.fn();
    const onConfigChange = jest.fn();

    beforeEach(() => {
        jest.clearAllMocks();
        mockUseChannelMissingValues.mockReturnValue({loading: false, failed: false, summary: null, reload: jest.fn()});
    });

    const renderComponent = (props: Partial<React.ComponentProps<typeof AttributeAppliesToChannelItem>> = {}) => {
        return renderWithContext(
            <AttributeAppliesToChannelItem
                config={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onConfigChange={onConfigChange}
                onRemove={onRemove}
                {...props}
            />,
        );
    };

    it('renders the Channels label', () => {
        renderComponent();
        expect(screen.getByTestId('attributeAppliesToRow-channel')).toHaveTextContent('Channels');
    });

    it('starts collapsed, with no Remove button, and clicking the toggle reveals the channel settings and Remove', async () => {
        renderComponent();

        expect(screen.queryByTestId('attributeAppliesToRow-channel-body')).not.toBeInTheDocument();
        expect(screen.queryByTestId('attributeAppliesToRow-channel-remove')).not.toBeInTheDocument();

        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
        expect(screen.getByTestId('channelsResourceSettings')).toBeInTheDocument();
        expect(screen.getByTestId('attributeAppliesToRow-channel-remove')).toBeVisible();

        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
        expect(screen.queryByTestId('attributeAppliesToRow-channel-body')).not.toBeInTheDocument();
        expect(screen.queryByTestId('attributeAppliesToRow-channel-remove')).not.toBeInTheDocument();
    });

    it('calls onRemove exactly once when Remove is clicked, once expanded', async () => {
        renderComponent();

        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-remove'));
        expect(onRemove).toHaveBeenCalledTimes(1);
    });

    it('disables the toggle, and the Remove button once expanded', async () => {
        const {rerender} = renderComponent();

        // Expand while enabled, then disable -- isOpen is local state, so it survives
        // the prop change, letting Remove's own disabled state be asserted directly.
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
        const disabledProps = {config: DEFAULT_CHANNEL_RESOURCE_CONFIG, onConfigChange, onRemove, disabled: true};
        rerender(<AttributeAppliesToChannelItem {...disabledProps}/>);

        expect(screen.getByTestId('attributeAppliesToRow-channel-toggle')).toBeDisabled();
        expect(screen.getByTestId('attributeAppliesToRow-channel-remove')).toBeDisabled();
    });

    it('removeLockedTooltip locks only Remove behind a tooltip wrap', async () => {
        renderComponent({removeLockedTooltip: 'Locked'});
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));

        expect(screen.getByTestId('attributeAppliesToRow-channel-removeLockWrap')).toBeInTheDocument();
        expect(screen.getByTestId('attributeAppliesToRow-channel-remove')).toBeDisabled();
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-remove'));
        expect(onRemove).not.toHaveBeenCalled();

        expect(screen.getByTestId('attributeAppliesToRow-channel-toggle')).toBeEnabled();
        await userEvent.click(screen.getByTestId('channelsResourceLocation-display_label_header'));
        expect(onConfigChange).toHaveBeenCalled();
    });

    it('renders no Remove lock wrap and enables Remove without removeLockedTooltip', async () => {
        renderComponent();
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));

        expect(screen.queryByTestId('attributeAppliesToRow-channel-removeLockWrap')).not.toBeInTheDocument();
        expect(screen.getByTestId('attributeAppliesToRow-channel-remove')).toBeEnabled();
    });

    it('states the configuration in the row summary', async () => {
        renderComponent({config: {required: true, changePolicy: 'never', displayLocations: ['display_label_header']}});

        expect(screen.getByTestId('attributeAppliesToRow-channel-summary')).
            toHaveTextContent('Required · Display: Header · Locked once set');

        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
        expect(screen.getByTestId('attributeAppliesToRow-channel-summary')).
            toHaveTextContent('Required · Display: Header · Locked once set');
    });

    it('reports a settings change without holding it itself', async () => {
        renderComponent();

        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
        await userEvent.click(screen.getByTestId('channelsResourceLocation-display_label_header'));

        expect(onConfigChange).toHaveBeenCalledWith(expect.objectContaining({
            displayLocations: ['display_label_header'],
        }));
    });

    it('makes no Client4 calls and no data-mutating dispatch', async () => {
        const createPropertyField = jest.spyOn(Client4, 'createPropertyField');
        const deletePropertyField = jest.spyOn(Client4, 'deletePropertyField');

        renderComponent();
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-remove'));

        expect(createPropertyField).not.toHaveBeenCalled();
        expect(deletePropertyField).not.toHaveBeenCalled();
    });
});
