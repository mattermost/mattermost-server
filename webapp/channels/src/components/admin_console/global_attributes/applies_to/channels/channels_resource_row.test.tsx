// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {DISPLAY_BANNER_TOP, DISPLAY_LABEL_HEADER, DISPLAY_LABEL_INFO} from 'mattermost-redux/constants/properties';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';

import ChannelsResourceRow from './channels_resource_row';
import type {ChannelResourceConfig} from './types';
import {DEFAULT_CHANNEL_RESOURCE_CONFIG} from './types';
import useChannelMissingValues from './use_channel_missing_values';

// Isolates this suite from the MM-70717 missing-values fetch -- it is covered
// by use_channel_missing_values.test.ts and channels_resource_settings.test.tsx.
// Idle/empty by default so every existing assertion here (in particular
// "toggles required") behaves exactly as before that feature landed.
jest.mock('./use_channel_missing_values');

const mockUseChannelMissingValues = jest.mocked(useChannelMissingValues);

describe('ChannelsResourceRow', () => {
    // The Required toggle itself is gated on this flag; its own on/off behavior
    // is covered in channels_resource_settings.test.tsx, so it is left on here.
    const requiredEnabledState: DeepPartial<GlobalState> = {
        entities: {
            general: {
                config: {
                    FeatureFlagChannelAttributes: 'true',
                    FeatureFlagChannelAttributesRequired: 'true',
                },
            },
        },
    };

    beforeEach(() => {
        mockUseChannelMissingValues.mockReturnValue({loading: false, failed: false, summary: null, reload: jest.fn()});
    });

    const renderRow = (overrides: Partial<ChannelResourceConfig> = {}, props: {disabled?: boolean; ordered?: boolean} = {}) => {
        const onChange = jest.fn();
        const onRemove = jest.fn();

        const row = (config: Partial<ChannelResourceConfig>) => (
            <ChannelsResourceRow
                value={{...DEFAULT_CHANNEL_RESOURCE_CONFIG, ...config}}
                onChange={onChange}
                onRemove={onRemove}
                ordered={props.ordered}
                disabled={props.disabled}
            />
        );

        const view = renderWithContext(row(overrides), requiredEnabledState);

        return {
            onChange,
            onRemove,
            rerender: (next: Partial<ChannelResourceConfig>) => view.rerender(row(next)),
        };
    };

    it('offers the display locations the server renders, and not the ones it does not', () => {
        renderRow();

        expect(screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`)).toBeInTheDocument();
        expect(screen.getByTestId(`channelsResourceLocation-${DISPLAY_BANNER_TOP}`)).toBeInTheDocument();

        // Channel Info lists every attribute, so a checkbox would not change where it appears.
        expect(screen.queryByTestId('channelsResourceLocation-display_label_info')).not.toBeInTheDocument();

        // Validates server-side, but always renders at the top.
        expect(screen.queryByTestId('channelsResourceLocation-display_banner_bottom')).not.toBeInTheDocument();

        expect(screen.getByText(/You can always view and set the attribute value in the Channel Info sidebar/)).toBeInTheDocument();
        expect(screen.getByText(/Unset values do not appear in the channel/)).toBeInTheDocument();
    });

    it('toggles required', async () => {
        const {onChange} = renderRow({required: false});

        await userEvent.click(screen.getByTestId('channelsResourceRequired-button'));

        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({required: true}));
    });

    it('states the required toggle in words as well as position', () => {
        const {rerender} = renderRow({required: false});
        expect(screen.getByText('Off')).toBeInTheDocument();
        expect(screen.getByText(/^Optional —/)).toBeInTheDocument();

        rerender({required: true});
        expect(screen.getByText('On')).toBeInTheDocument();
        expect(screen.getByText(/^Required —/)).toBeInTheDocument();
    });

    // Every menu-driven case here asserts the selected label, never the open menu:
    // MUI's Popover rejects a jsdom anchor as having no layout. Playwright covers
    // opening them.
    it('shows which change policy is selected', () => {
        const {rerender} = renderRow({changePolicy: 'any'});
        expect(screen.getByTestId('channelsResourceChangePolicyButton')).toHaveTextContent('Can be changed at any time');

        rerender({changePolicy: 'never'});
        expect(screen.getByTestId('channelsResourceChangePolicyButton')).toHaveTextContent('Cannot be changed once set');
    });

    it('explains why raising and lowering are unavailable on an unranked attribute', () => {
        renderRow({}, {ordered: false});

        expect(screen.getByText(/only offered on a Rank attribute/)).toBeInTheDocument();
    });

    it('drops that explanation once the values are ranked', () => {
        renderRow({}, {ordered: true});

        expect(screen.queryByText(/only offered on a Rank attribute/)).not.toBeInTheDocument();
    });

    it('describes a directional policy the attribute can no longer take', () => {
        // Set while the attribute was ranked, then the type changed underneath it.
        renderRow({changePolicy: 'raise_only'}, {ordered: false});

        expect(screen.getByTestId('channelsResourceChangePolicyButton')).toHaveTextContent('Can only be raised, never lowered');
    });

    it('adds a display location', async () => {
        const {onChange} = renderRow({displayLocations: []});

        await userEvent.click(screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`));

        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({displayLocations: [DISPLAY_LABEL_HEADER]}));
    });

    it('removes a display location', async () => {
        const {onChange} = renderRow({displayLocations: [DISPLAY_LABEL_HEADER, DISPLAY_BANNER_TOP]});

        await userEvent.click(screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`));

        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({displayLocations: [DISPLAY_BANNER_TOP]}));
    });

    it('keeps a UI-hidden display_label_info action when toggling another location', async () => {
        const {onChange} = renderRow({displayLocations: [DISPLAY_LABEL_INFO, DISPLAY_BANNER_TOP]});

        await userEvent.click(screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`));

        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({
            displayLocations: [DISPLAY_LABEL_HEADER, DISPLAY_BANNER_TOP, DISPLAY_LABEL_INFO],
        }));
    });

    it('stores display locations in a stable order regardless of tick order', async () => {
        const {onChange} = renderRow({displayLocations: [DISPLAY_BANNER_TOP]});

        await userEvent.click(screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`));

        // Header first, even though Banner was ticked first.
        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({
            displayLocations: [DISPLAY_LABEL_HEADER, DISPLAY_BANNER_TOP],
        }));
    });

    it('offers no control over who can set the value', () => {
        // Held back until the permissions work lands. The tier itself is still
        // written -- see channel_field_payload.
        renderRow();

        expect(screen.queryByTestId('channelsResourceSetterButton')).not.toBeInTheDocument();
        expect(screen.queryByText('Who can set the value')).not.toBeInTheDocument();
    });

    it('names its disclosure control', async () => {
        renderRow();

        expect(screen.getByRole('button', {name: 'Collapse channel settings'})).toBeInTheDocument();

        await userEvent.click(screen.getByTestId('channelsResourceRowDisclosure'));

        expect(screen.getByRole('button', {name: 'Expand channel settings'})).toBeInTheDocument();
    });

    it('groups the display locations under their own label', () => {
        renderRow();

        expect(screen.getByRole('group', {name: 'Display location'})).toBeInTheDocument();
    });

    it('summarises its own state for the collapsed view', () => {
        renderRow({
            required: true,
            displayLocations: [DISPLAY_LABEL_HEADER, DISPLAY_BANNER_TOP],
        });

        expect(screen.getByTestId('channelsResourceRowSummary')).toHaveTextContent('Required · Display: Header + Banner');
    });

    it('says so in the summary when the attribute is displayed nowhere or locked', () => {
        renderRow({displayLocations: [], changePolicy: 'never'});

        expect(screen.getByTestId('channelsResourceRowSummary')).toHaveTextContent('Optional · Channel Info only · Locked once set');
    });

    it('summarises a directional policy too', () => {
        renderRow({changePolicy: 'raise_only', displayLocations: [DISPLAY_LABEL_HEADER]}, {ordered: true});

        expect(screen.getByTestId('channelsResourceRowSummary')).toHaveTextContent('Optional · Display: Header · Raise only');
    });

    it('collapses and expands its body', async () => {
        renderRow();

        expect(screen.getByTestId('channelsResourceRequired-button')).toBeInTheDocument();

        await userEvent.click(screen.getByTestId('channelsResourceRowDisclosure'));

        expect(screen.queryByTestId('channelsResourceRequired-button')).not.toBeInTheDocument();
        expect(screen.getByTestId('channelsResourceRowSummary')).toBeInTheDocument();
    });

    it('removes the resource', async () => {
        const {onRemove} = renderRow();

        await userEvent.click(screen.getByTestId('channelsResourceRowRemove'));

        expect(onRemove).toHaveBeenCalled();
    });

    it('does not accept changes while disabled', async () => {
        const {onChange, onRemove} = renderRow({}, {disabled: true});

        await userEvent.click(screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`));
        await userEvent.click(screen.getByTestId('channelsResourceRowRemove'));

        expect(onChange).not.toHaveBeenCalled();
        expect(onRemove).not.toHaveBeenCalled();
    });
});
