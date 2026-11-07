// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, renderHook, waitFor} from '@testing-library/react';

import useChannelMissingValues from './use_channel_missing_values';

import {fetchChannelsMissingValueSummary} from '../../utils';

jest.mock('../../utils', () => ({
    __esModule: true,
    fetchChannelsMissingValueSummary: jest.fn(),
}));

const mockedFetch = jest.mocked(fetchChannelsMissingValueSummary);

function summary(overrides: Partial<Awaited<ReturnType<typeof fetchChannelsMissingValueSummary>>> = {}) {
    return {
        required: false,
        missing_channel_count: 3,
        shared_channel_count: 1,
        unique_admin_count: 2,
        channels_without_admin_count: 0,
        message_preview: 'preview text',
        ...overrides,
    };
}

describe('useChannelMissingValues', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    it('does not fetch when disabled', () => {
        renderHook(() => useChannelMissingValues({fieldId: 'field1', enabled: false}));

        expect(mockedFetch).not.toHaveBeenCalled();
    });

    it('fetches with the given field id in edit mode', async () => {
        mockedFetch.mockResolvedValue(summary());

        renderHook(() => useChannelMissingValues({fieldId: 'field1', enabled: true}));

        await waitFor(() => expect(mockedFetch).toHaveBeenCalledWith('field1'));
    });

    it('fetches with undefined in create mode', async () => {
        mockedFetch.mockResolvedValue(summary());

        renderHook(() => useChannelMissingValues({fieldId: undefined, enabled: true}));

        await waitFor(() => expect(mockedFetch).toHaveBeenCalledWith(undefined));
    });

    it('maps the response onto the summary shape', async () => {
        mockedFetch.mockResolvedValue(summary({missing_channel_count: 5, unique_admin_count: 4, channels_without_admin_count: 1, required: true}));

        const {result} = renderHook(() => useChannelMissingValues({fieldId: 'field1', enabled: true}));

        await waitFor(() => expect(result.current.summary).not.toBeNull());
        expect(result.current.summary).toEqual({
            totalCount: 5,
            sharedCount: 1,
            uniqueAdminCount: 4,
            noAdminCount: 1,
            messagePreview: 'preview text',
            required: true,
        });
        expect(result.current.loading).toBe(false);
        expect(result.current.failed).toBe(false);
    });

    it('refetches when the revision changes, so a save on the same page is picked up', async () => {
        mockedFetch.mockResolvedValueOnce(summary({required: true}));

        const {result, rerender} = renderHook(
            ({revision}: {revision: number}) => useChannelMissingValues({fieldId: 'field1', enabled: true, revision}),
            {initialProps: {revision: 1}},
        );
        await waitFor(() => expect(result.current.summary?.required).toBe(true));

        mockedFetch.mockResolvedValueOnce(summary({required: false}));
        rerender({revision: 2});

        await waitFor(() => expect(result.current.summary?.required).toBe(false));
        expect(mockedFetch).toHaveBeenCalledTimes(2);
    });

    it('reports failed on rejection, leaving summary null', async () => {
        mockedFetch.mockRejectedValue(new Error('boom'));

        const {result} = renderHook(() => useChannelMissingValues({fieldId: 'field1', enabled: true}));

        await waitFor(() => expect(result.current.failed).toBe(true));
        expect(result.current.summary).toBeNull();
        expect(result.current.loading).toBe(false);
    });

    it('ignores a stale response that resolves after a newer fetch', async () => {
        let resolveFirst: (value: ReturnType<typeof summary>) => void = () => {};
        const first = new Promise<ReturnType<typeof summary>>((resolve) => {
            resolveFirst = resolve;
        });
        mockedFetch.mockReturnValueOnce(first);
        mockedFetch.mockResolvedValueOnce(summary({missing_channel_count: 9}));

        const {result, rerender} = renderHook(
            ({fieldId}) => useChannelMissingValues({fieldId, enabled: true}),
            {initialProps: {fieldId: 'field1'}},
        );

        rerender({fieldId: 'field2'});

        await waitFor(() => expect(result.current.summary?.totalCount).toBe(9));

        // The first (stale) fetch resolving afterward must not overwrite the
        // second (current) fetch's result.
        resolveFirst(summary({missing_channel_count: 1}));
        await new Promise((resolve) => setTimeout(resolve, 0));

        expect(result.current.summary?.totalCount).toBe(9);
    });

    it('ignores a response that resolves after the hook has been disabled', async () => {
        let resolveFetch: (value: ReturnType<typeof summary>) => void = () => {};
        mockedFetch.mockReturnValueOnce(new Promise((resolve) => {
            resolveFetch = resolve;
        }));

        const {result, rerender} = renderHook(
            ({enabled}) => useChannelMissingValues({fieldId: 'field1', enabled}),
            {initialProps: {enabled: true}},
        );

        await waitFor(() => expect(mockedFetch).toHaveBeenCalledTimes(1));

        rerender({enabled: false});
        expect(result.current.loading).toBe(false);

        // The disabled hook must not react to a response for the fetch it
        // fired while still enabled.
        resolveFetch(summary({missing_channel_count: 42}));
        await new Promise((resolve) => setTimeout(resolve, 0));

        expect(result.current.summary).toBeNull();
        expect(result.current.loading).toBe(false);
    });

    it('reload() triggers a new fetch', async () => {
        mockedFetch.mockResolvedValue(summary());

        const {result} = renderHook(() => useChannelMissingValues({fieldId: 'field1', enabled: true}));

        await waitFor(() => expect(mockedFetch).toHaveBeenCalledTimes(1));

        act(() => {
            result.current.reload();
        });

        await waitFor(() => expect(mockedFetch).toHaveBeenCalledTimes(2));
    });
});
