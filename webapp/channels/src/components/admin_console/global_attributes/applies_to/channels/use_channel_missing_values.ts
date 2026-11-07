// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useCallback, useEffect, useRef, useState} from 'react';

import {fetchChannelsMissingValueSummary} from '../../utils';

export type ChannelMissingValuesSummary = {
    totalCount: number;
    sharedCount: number;
    uniqueAdminCount: number;
    noAdminCount: number;
    messagePreview: string;

    // Whether the saved field is required. The toggle's own value is the
    // unsaved form state; the server only refuses a saved false -> true.
    required: boolean;
};

export type ChannelMissingValuesState = {
    loading: boolean;
    failed: boolean;

    // null until the first successful load; 0 is a meaningful, distinct value
    // (every channel has a value).
    summary: ChannelMissingValuesSummary | null;
    reload: () => void;
};

// useChannelMissingValues fetches the count-only compliance summary behind
// the Required toggle's banner. fieldId is undefined in create mode -- the
// channel-linked field has no ID yet, so the server reports every active
// channel as missing a value. enabled=false never fetches (used to defer the
// call until the Channels row is actually expanded). A change to revision
// refetches, for a host that saves the field without leaving the page.
export default function useChannelMissingValues(args: {fieldId?: string; enabled: boolean; revision?: number}): ChannelMissingValuesState {
    const {fieldId, enabled, revision} = args;

    const [loading, setLoading] = useState(false);
    const [failed, setFailed] = useState(false);
    const [summary, setSummary] = useState<ChannelMissingValuesSummary | null>(null);
    const [reloadNonce, setReloadNonce] = useState(0);

    // Guards against an out-of-order response: reload() or a fieldId change
    // can fire a new fetch while an older one is still in flight.
    const seqRef = useRef(0);

    const reload = useCallback(() => {
        setReloadNonce((n) => n + 1);
    }, []);

    useEffect(() => {
        if (!enabled) {
            setLoading(false);
            setFailed(false);

            // Still bumps the sequence on the way out (see the cleanup
            // below): a fetch started while enabled was true can otherwise
            // resolve after enabled flips to false without ever seeing a
            // seq mismatch, since nothing else would have incremented it.
            return () => {
                seqRef.current += 1;
            };
        }

        const mySeq = ++seqRef.current;
        setLoading(true);
        setFailed(false);

        fetchChannelsMissingValueSummary(fieldId).then((data) => {
            if (seqRef.current !== mySeq) {
                return;
            }
            setSummary({
                totalCount: data.missing_channel_count,
                sharedCount: data.shared_channel_count,
                uniqueAdminCount: data.unique_admin_count,
                noAdminCount: data.channels_without_admin_count,
                messagePreview: data.message_preview,
                required: data.required,
            });
            setLoading(false);
        }).catch(() => {
            if (seqRef.current !== mySeq) {
                return;
            }
            setFailed(true);
            setLoading(false);
        });

        // Invalidates mySeq on unmount or on any dependency change (fieldId,
        // enabled, or a reload), so a response that lands after either can
        // never land on stale/disabled state.
        return () => {
            seqRef.current += 1;
        };
    }, [enabled, fieldId, revision, reloadNonce]);

    return {loading, failed, summary, reload};
}
