// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage, defineMessages} from 'react-intl';

import AlertBanner from 'components/alert_banner';

export type NotifyStatus = 'idle' | 'notifying' | 'queued' | 'notify_failed';

type Props = {
    count: number;
    attributeDisplayName?: string;

    // False in create mode: the field has no ID yet. Gates both actions, not
    // just Notify -- View channel list would otherwise show literally every
    // active channel (nothing can have a value for a field that does not
    // exist), which is not a targeted list, just noise.
    canNotify: boolean;

    // required is already true and channels are still missing a value (e.g.
    // after an unarchive). Nothing is being blocked in this state -- only the
    // copy changes.
    alreadyRequired: boolean;

    // Off -> on is currently refused. Combined with attemptedEnable this is
    // what turns the banner from info to warning -- see the mode comment
    // below.
    blocked: boolean;

    // The admin has clicked Required at least once while blocked. Starts
    // false: the banner opens as a plain informational notice (nothing has
    // been refused yet), and only escalates once a real attempt was made and
    // refused, so it does not read as an error before the admin has done
    // anything.
    attemptedEnable: boolean;

    status: NotifyStatus;
    queuedAdminCount?: number;

    // True when the last notify run hit the per-run assignment cap: some
    // affected admins were not part of this batch and were not queued
    // for anything. Distinct from ChannelsMissingValuesBanner's other counts,
    // which stay exact regardless -- this is specifically about the DMs
    // just queued, not the underlying missing-value tally.
    queuedTruncated?: boolean;

    bodyId: string;
    onNotify: () => void;
    onViewList: () => void;
};

const messages = defineMessages({
    title: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.title',
        defaultMessage: 'Set channel attribute values',
    },
    body: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.body',
        defaultMessage: '{count, plural, one {# channel doesn\'t} other {# channels don\'t}} have a {attribute} value yet. Set a value on every existing channel before turning Required on.',
    },
    bodyAlreadyRequired: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.body_already_required',
        defaultMessage: '{count, plural, one {# channel doesn\'t} other {# channels don\'t}} have a {attribute} value yet. New channels must set one; existing channels keep working until a value is set.',
    },
    bodyCreateMode: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.body_create_mode',
        defaultMessage: 'Save this attribute first, then set a value on every existing channel before turning Required on.',
    },
    attributeFallback: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.attribute_fallback',
        defaultMessage: 'this attribute',
    },
    notify: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.notify',
        defaultMessage: 'Notify all channel admins',
    },
    viewList: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.view_list',
        defaultMessage: 'View channel list',
    },
    queued: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.queued',
        defaultMessage: 'Notification queued for {count, plural, one {# channel admin} other {# channel admins}}.',
    },
    queuedTruncated: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.queued_truncated',
        defaultMessage: 'Notification queued for {count, plural, one {# channel admin} other {# channel admins}}. Some channel admins were not reached because this attribute affects more admins than one run can notify. Use View channel list to see the channels still missing a value.',
    },
    notifyFailed: {
        id: 'admin.global_attributes.applies_to.channels.missing_values.notify_failed',
        defaultMessage: 'The notifications couldn\'t be sent. Please try again.',
    },
});

export default function ChannelsMissingValuesBanner({
    count,
    attributeDisplayName,
    canNotify,
    alreadyRequired,
    blocked,
    attemptedEnable,
    status,
    queuedAdminCount,
    queuedTruncated,
    bodyId,
    onNotify,
    onViewList,
}: Props) {
    if (count <= 0) {
        return null;
    }

    const attribute = attributeDisplayName || <FormattedMessage {...messages.attributeFallback}/>;

    let bodyMessage = messages.body;
    if (alreadyRequired) {
        bodyMessage = messages.bodyAlreadyRequired;
    } else if (canNotify === false) {
        bodyMessage = messages.bodyCreateMode;
    }

    // Info until a refused attempt makes it a warning worth calling out --
    // see the attemptedEnable prop doc.
    let mode: 'info' | 'warning' | 'success' = blocked && attemptedEnable ? 'warning' : 'info';
    let message: React.ReactNode = (
        <span id={bodyId}>
            <FormattedMessage
                {...bodyMessage}
                values={{count, attribute}}
            />
        </span>
    );

    if (status === 'notify_failed') {
        mode = 'warning';
        message = (
            <span id={bodyId}>
                <FormattedMessage {...messages.notifyFailed}/>
            </span>
        );
    } else if (status === 'queued') {
        // Still a success -- every admin this run could reach was queued for
        // a DM -- but the copy has to be honest that this run did not cover
        // everyone, since "Notify all channel admins" otherwise reads as a
        // completeness promise this run could not keep. Delivery happens in
        // the background, so the copy must not claim the DMs were delivered.
        mode = queuedTruncated ? 'warning' : 'success';
        message = (
            <span id={bodyId}>
                <FormattedMessage
                    {...(queuedTruncated ? messages.queuedTruncated : messages.queued)}
                    values={{count: queuedAdminCount ?? 0}}
                />
            </span>
        );
    }

    return (
        <AlertBanner
            id='channelsMissingValuesBanner'
            mode={mode}
            title={<FormattedMessage {...messages.title}/>}
            message={message}
            actionButtonLeft={canNotify ? (
                <button
                    type='button'
                    className='AlertBanner__buttonLeft ChannelsMissingValuesBanner__notify'
                    disabled={status === 'notifying'}
                    onClick={onNotify}
                >
                    <FormattedMessage {...messages.notify}/>
                </button>
            ) : undefined}
            actionButtonRight={canNotify ? (
                <button
                    type='button'
                    className='AlertBanner__buttonRight ChannelsMissingValuesBanner__viewList'
                    onClick={onViewList}
                >
                    <FormattedMessage {...messages.viewList}/>
                </button>
            ) : undefined}
        />
    );
}
