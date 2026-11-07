// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useState} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import type {PropertyField, PropertyValue} from '@mattermost/types/properties';

import {PropertyTypes} from 'mattermost-redux/action_types';
import {Client4} from 'mattermost-redux/client';
import {ACCESS_CONTROL_PROPERTY_GROUP, CHANNEL_OBJECT_TYPE} from 'mattermost-redux/constants/properties';
import {isChannelAttributesRequiredEnabled} from 'mattermost-redux/selectors/entities/general';
import {isPropertyFieldRequired, isPropertyValueSet} from 'mattermost-redux/utils/property_utils';

import useChannelAttributes from './useChannelAttributes';

export type MissingRequiredChannelAttributesState = {
    loading: boolean;
    missing: PropertyField[];
};

type RequestState = {
    channelId: string;
    status: 'loading' | 'loaded' | 'failed';
    missing: PropertyField[];
};

const IDLE: RequestState = {channelId: '', status: 'loaded', missing: []};

/**
 * Reports which required channel attributes lack a value on this channel --
 * the advisory check behind the unarchive warning (MM-70717; unarchive itself
 * is warn-only and never blocked on this).
 *
 * Fails open: while the value fetch is in flight, and if it errors, `missing`
 * is `[]` rather than every required field. `missing` is derived from the
 * fetch response directly, not from a Redux selector -- Redux may not have
 * this channel's values cached yet (or ever, if this channel was never
 * mounted elsewhere), and reading it as authoritative would read "no cached
 * value" as "unset" on every cold load, well before the fetch below settles.
 *
 * Reports nothing while ChannelAttributesRequired is off: the server enforces
 * Required on no channel attribute then, so there is nothing to warn about.
 */
export default function useMissingRequiredChannelAttributes(channelId: string): MissingRequiredChannelAttributesState {
    const dispatch = useDispatch();

    const {enabled: attributesEnabled, loading: fieldsLoading, fields} = useChannelAttributes();
    const requiredEnforced = useSelector(isChannelAttributesRequiredEnabled);
    const enabled = attributesEnabled && requiredEnforced;
    const requiredFields = fields.filter(isPropertyFieldRequired);

    const [request, setRequest] = useState<RequestState>(IDLE);

    useEffect(() => {
        if (!enabled || requiredFields.length === 0) {
            setRequest({channelId, status: 'loaded', missing: []});
            return undefined;
        }

        // Guards against a slower request for a previous channelId settling
        // after a newer one has already started -- without this, switching
        // channels quickly could let a stale response overwrite the current
        // channel's result.
        let current = true;
        setRequest({channelId, status: 'loading', missing: []});

        Client4.getPropertyValues(ACCESS_CONTROL_PROPERTY_GROUP, CHANNEL_OBJECT_TYPE, channelId).then((values) => {
            if (!current) {
                return;
            }

            // RECEIVED_PROPERTY_VALUES only merges, so a cached value for a
            // field the server now omits must be dropped explicitly.
            const returnedFieldIds = new Set((values ?? []).map((v) => v.field_id));
            for (const field of requiredFields) {
                if (!returnedFieldIds.has(field.id)) {
                    dispatch({
                        type: PropertyTypes.PROPERTY_VALUE_DELETED,
                        data: {targetId: channelId, fieldId: field.id},
                    });
                }
            }
            if (values && values.length > 0) {
                dispatch({
                    type: PropertyTypes.RECEIVED_PROPERTY_VALUES,
                    data: {values},
                });
            }
            const missing = requiredFields.filter((field) => {
                const value = (values as Array<PropertyValue<unknown>> | undefined)?.find((v) => v.field_id === field.id);
                return !isPropertyValueSet(value?.value);
            });
            setRequest({channelId, status: 'loaded', missing});
        }).catch(() => {
            if (!current) {
                return;
            }

            // Fail open -- see doc comment above.
            setRequest({channelId, status: 'failed', missing: []});
        });

        return () => {
            current = false;
        };

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [enabled, channelId, requiredFields.length]);

    // A request still in flight for this channelId (or one that hasn't
    // started yet after channelId just changed) reports loading with an
    // empty `missing`, not the previous channel's result.
    const forThisChannel = request.channelId === channelId;
    const valuesLoading = !forThisChannel || request.status === 'loading';

    return {
        loading: fieldsLoading || (enabled && requiredFields.length > 0 && valuesLoading),
        missing: forThisChannel ? request.missing : [],
    };
}
