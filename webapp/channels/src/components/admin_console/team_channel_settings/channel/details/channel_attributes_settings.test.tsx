// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fireEvent, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';

import type {PropertyField} from '@mattermost/types/properties';

import type {ResolvedChannelAttribute} from 'mattermost-redux/selectors/entities/properties';

import {renderWithContext, waitFor} from 'tests/react_testing_utils';

import ChannelAttributesSettings from './channel_attributes_settings';

const mockUseChannelAttributes = jest.fn();
jest.mock('components/common/hooks/useChannelAttributes', () => ({
    __esModule: true,
    default: () => mockUseChannelAttributes(),
}));

const mockUseResolvedChannelAttributes = jest.fn();
jest.mock('components/common/hooks/useResolvedChannelAttributes', () => ({
    __esModule: true,
    default: () => mockUseResolvedChannelAttributes(),
}));

jest.mock('mattermost-redux/client', () => ({
    Client4: {
        getPropertyValues: jest.fn().mockResolvedValue([]),
    },
}));

function makeField(overrides: Partial<PropertyField> = {}): PropertyField {
    return {
        id: 'field1',
        group_id: 'group1',
        name: 'program',
        type: 'select',
        create_at: 0,
        update_at: 0,
        delete_at: 0,
        attrs: {
            display_name: 'Program',
            options: [
                {id: 'opt1', name: 'AURORA'},
                {id: 'opt2', name: 'ZEPHYR'},
            ],
        },
        target_id: '',
        target_type: '',
        ...overrides,
    } as PropertyField;
}

function makeResolved(field: PropertyField, rawValue?: unknown): ResolvedChannelAttribute {
    return {
        field,
        value: rawValue === undefined ? undefined : {
            id: 'value1',
            field_id: field.id,
            target_id: 'channel1',
            target_type: 'channel',
            group_id: field.group_id,
            value: rawValue,
            create_at: 0,
            update_at: 0,
            delete_at: 0,
        },
    } as ResolvedChannelAttribute;
}

describe('ChannelAttributesSettings', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('renders nothing when the feature is disabled', () => {
        mockUseChannelAttributes.mockReturnValue({enabled: false, loading: false, failed: false, fields: []});
        mockUseResolvedChannelAttributes.mockReturnValue([]);

        const {container} = renderWithContext(
            <ChannelAttributesSettings
                channelId='channel1'
                onChange={jest.fn()}
            />,
        );

        expect(container).toBeEmptyDOMElement();
    });

    test('renders a bordered option picker and reports the pending change', async () => {
        const field = makeField();
        mockUseChannelAttributes.mockReturnValue({enabled: true, loading: false, failed: false, fields: [field]});
        mockUseResolvedChannelAttributes.mockReturnValue([makeResolved(field)]);

        const onChange = jest.fn();
        renderWithContext(
            <ChannelAttributesSettings
                channelId='channel1'
                onChange={onChange}
            />,
        );

        await waitFor(() => expect(screen.getByTestId('channelAttributesSettings-program')).toBeInTheDocument());

        await userEvent.click(screen.getByTestId('channelAttributesSettings-program'));
        await userEvent.click(await screen.findByText('ZEPHYR'));

        await waitFor(() => expect(onChange).toHaveBeenCalledWith({[field.id]: 'opt2'}));

        // MUI's Popover logs a spurious "anchorEl" warning the first time a jsdom
        // test opens one -- reproducible on unrelated, already-shipped tests too
        // (e.g. channel_info_attributes.test.tsx run in isolation), not something
        // this menu usage introduces. setup_jest.ts fails the test on any console
        // warning, so drop just this known-noisy one.
        const warnCalls = (console.warn as jest.Mock).mock.calls;
        for (let i = warnCalls.length - 1; i >= 0; i--) {
            if (typeof warnCalls[i][0] === 'string' && warnCalls[i][0].includes('anchorEl')) {
                warnCalls.splice(i, 1);
            }
        }
    });

    test('a multiselect field toggles a value on and off as separate chips', async () => {
        const field = makeField({
            name: 'labels',
            type: 'multiselect',
            attrs: {
                display_name: 'Labels',
                options: [
                    {id: 'opt1', name: 'AURORA'},
                    {id: 'opt2', name: 'ZEPHYR'},
                ],
            },
        });
        mockUseChannelAttributes.mockReturnValue({enabled: true, loading: false, failed: false, fields: [field]});
        mockUseResolvedChannelAttributes.mockReturnValue([makeResolved(field, ['opt1'])]);

        const onChange = jest.fn();
        renderWithContext(
            <ChannelAttributesSettings
                channelId='channel1'
                onChange={onChange}
            />,
        );

        await waitFor(() => expect(screen.getByTestId('channelAttributesSettings-labels')).toBeInTheDocument());
        expect(screen.getByText('AURORA')).toBeInTheDocument();

        await userEvent.click(screen.getByTestId('channelAttributesSettings-labels'));
        await userEvent.click(await screen.findByText('ZEPHYR'));

        await waitFor(() => expect(onChange).toHaveBeenCalledWith({[field.id]: ['opt1', 'opt2']}));

        const warnCalls = (console.warn as jest.Mock).mock.calls;
        for (let i = warnCalls.length - 1; i >= 0; i--) {
            if (typeof warnCalls[i][0] === 'string' && warnCalls[i][0].includes('anchorEl')) {
                warnCalls.splice(i, 1);
            }
        }
    });

    test('a field locked by permission_values="none" is disabled', async () => {
        const field = makeField({permission_values: 'none'});
        mockUseChannelAttributes.mockReturnValue({enabled: true, loading: false, failed: false, fields: [field]});
        mockUseResolvedChannelAttributes.mockReturnValue([makeResolved(field)]);

        renderWithContext(
            <ChannelAttributesSettings
                channelId='channel1'
                onChange={jest.fn()}
            />,
        );

        await waitFor(() => expect(screen.getByTestId('channelAttributesSettings-program')).toBeDisabled());
    });

    test('a text field renders as an input and reports typed values', async () => {
        const field = makeField({name: 'notes', type: 'text', attrs: {display_name: 'Notes'}});
        mockUseChannelAttributes.mockReturnValue({enabled: true, loading: false, failed: false, fields: [field]});
        mockUseResolvedChannelAttributes.mockReturnValue([makeResolved(field)]);

        const onChange = jest.fn();
        renderWithContext(
            <ChannelAttributesSettings
                channelId='channel1'
                onChange={onChange}
            />,
        );

        const input = await waitFor(() => screen.getByTestId('channelAttributesSettings-notes'));
        fireEvent.change(input, {target: {value: 'hello'}});

        expect(onChange).toHaveBeenCalledWith({[field.id]: 'hello'});
    });
});
