// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {waitFor} from '@testing-library/react';
import {useSelector} from 'react-redux';

import type {PropertyField, PropertyValue} from '@mattermost/types/properties';

import {Client4} from 'mattermost-redux/client';
import {getPropertyValueForTargetField} from 'mattermost-redux/selectors/entities/properties';

import {renderHookWithContext} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';

import useChannelAttributes from './useChannelAttributes';
import useMissingRequiredChannelAttributes from './useMissingRequiredChannelAttributes';

jest.mock('./useChannelAttributes');
jest.mock('mattermost-redux/client', () => ({
    Client4: {
        getPropertyValues: jest.fn(),
    },
}));

const mockUseChannelAttributes = jest.mocked(useChannelAttributes);
const mockGetPropertyValues = jest.mocked(Client4.getPropertyValues);

const requiredField: PropertyField = {
    id: 'field1',
    group_id: 'group1',
    name: 'cost_center',
    type: 'text',
    target_id: '',
    target_type: 'system',
    object_type: 'channel',
    create_at: 0,
    update_at: 0,
    delete_at: 0,
    created_by: '',
    updated_by: '',
    attrs: {required: true, display_name: 'Cost center'},
};

function propertyValue(fieldId: string, value: unknown): PropertyValue<unknown> {
    return {
        id: 'v_' + fieldId,
        target_id: 'channel1',
        target_type: 'channel',
        group_id: 'group1',
        field_id: fieldId,
        value,
        create_at: 0,
        update_at: 0,
        delete_at: 0,
        created_by: '',
        updated_by: '',
    };
}

const requiredEnforcedState = {
    entities: {
        general: {
            config: {
                FeatureFlagChannelAttributes: 'true',
                FeatureFlagChannelAttributesRequired: 'true',
            },
        },
    },
};

describe('useMissingRequiredChannelAttributes', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockUseChannelAttributes.mockReturnValue({
            enabled: true,
            loading: false,
            failed: false,
            fields: [requiredField],
        });
    });

    test('reports loading with no missing fields while the value fetch is pending', async () => {
        let resolveFetch: (values: Array<PropertyValue<unknown>>) => void = () => {};
        mockGetPropertyValues.mockReturnValue(new Promise((resolve) => {
            resolveFetch = resolve;
        }));

        const {result} = renderHookWithContext(() => useMissingRequiredChannelAttributes('channel1'), requiredEnforcedState);

        expect(result.current.loading).toBe(true);
        expect(result.current.missing).toEqual([]);

        resolveFetch([propertyValue('field1', 'set')]);
        await waitFor(() => expect(result.current.loading).toBe(false));
    });

    test('reports only the required fields that genuinely lack a value once the fetch succeeds', async () => {
        mockGetPropertyValues.mockResolvedValue([propertyValue('field1', '')]);

        const {result} = renderHookWithContext(() => useMissingRequiredChannelAttributes('channel1'), requiredEnforcedState);

        await waitFor(() => expect(result.current.loading).toBe(false));
        expect(result.current.missing).toEqual([requiredField]);
    });

    test('does not report the field as missing once a real value is set', async () => {
        mockGetPropertyValues.mockResolvedValue([propertyValue('field1', 'set')]);

        const {result} = renderHookWithContext(() => useMissingRequiredChannelAttributes('channel1'), requiredEnforcedState);

        await waitFor(() => expect(result.current.loading).toBe(false));
        expect(result.current.missing).toEqual([]);
    });

    test('drops a cached value for a required field the server response omits', async () => {
        mockGetPropertyValues.mockResolvedValue([]);

        const {result} = renderHookWithContext(
            () => ({
                missing: useMissingRequiredChannelAttributes('channel1'),
                cached: useSelector((state: GlobalState) => getPropertyValueForTargetField(state, 'channel1', 'field1')),
            }),
            {
                ...requiredEnforcedState,
                entities: {
                    ...requiredEnforcedState.entities,
                    properties: {
                        values: {
                            byTargetId: {channel1: {field1: propertyValue('field1', 'stale')}},
                            byFieldId: {field1: {channel1: propertyValue('field1', 'stale')}},
                        },
                    },
                },
            },
        );

        expect(result.current.cached?.value).toBe('stale');

        await waitFor(() => expect(result.current.missing.loading).toBe(false));
        expect(result.current.cached).toBeUndefined();
        expect(result.current.missing.missing).toEqual([requiredField]);
    });

    test('keeps the cached value when the server response includes the field', async () => {
        mockGetPropertyValues.mockResolvedValue([propertyValue('field1', 'fresh')]);

        const {result} = renderHookWithContext(
            () => ({
                missing: useMissingRequiredChannelAttributes('channel1'),
                cached: useSelector((state: GlobalState) => getPropertyValueForTargetField(state, 'channel1', 'field1')),
            }),
            requiredEnforcedState,
        );

        await waitFor(() => expect(result.current.missing.loading).toBe(false));
        expect(result.current.cached?.value).toBe('fresh');
        expect(result.current.missing.missing).toEqual([]);
    });

    test('fails open (missing: []) when the value fetch rejects', async () => {
        mockGetPropertyValues.mockRejectedValue(new Error('network'));

        const {result} = renderHookWithContext(() => useMissingRequiredChannelAttributes('channel1'), requiredEnforcedState);

        await waitFor(() => expect(result.current.loading).toBe(false));
        expect(result.current.missing).toEqual([]);
    });

    test('a stale response for a previous channel cannot affect the current channel\'s result', async () => {
        let resolveFirst: (values: Array<PropertyValue<unknown>>) => void = () => {};
        mockGetPropertyValues.mockReturnValueOnce(new Promise((resolve) => {
            resolveFirst = resolve;
        }));

        const {result, rerender} = renderHookWithContext(
            (channelId: string = 'channel1') => useMissingRequiredChannelAttributes(channelId),
            requiredEnforcedState,
        );

        // Switch to a second channel before the first channel's request settles.
        mockGetPropertyValues.mockResolvedValueOnce([propertyValue('field1', 'set')]);
        rerender('channel2');

        await waitFor(() => expect(result.current.loading).toBe(false));
        expect(result.current.missing).toEqual([]);

        // Channel1's stale request resolving afterward must not overwrite
        // channel2's already-settled result.
        resolveFirst([propertyValue('field1', '')]);
        await new Promise((resolve) => setTimeout(resolve, 0));

        expect(result.current.missing).toEqual([]);
    });

    test('reports nothing and fetches nothing while ChannelAttributesRequired is off', async () => {
        mockGetPropertyValues.mockResolvedValue([propertyValue('field1', '')]);

        const {result} = renderHookWithContext(() => useMissingRequiredChannelAttributes('channel1'), {
            entities: {general: {config: {FeatureFlagChannelAttributes: 'true', FeatureFlagChannelAttributesRequired: 'false'}}},
        });

        await waitFor(() => expect(result.current.loading).toBe(false));
        expect(result.current.missing).toEqual([]);
        expect(mockGetPropertyValues).not.toHaveBeenCalled();
    });
});
