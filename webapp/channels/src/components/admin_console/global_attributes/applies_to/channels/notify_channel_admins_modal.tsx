// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage, useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import {GenericModal} from '@mattermost/components';

import {openModal} from 'actions/views/modals';

import Markdown from 'components/markdown';

import {ModalIdentifiers} from 'utils/constants';

export type NotifyChannelAdminsStats = {
    totalCount: number;
    uniqueAdminCount: number;
    noAdminCount: number;
    messagePreview: string;
    attributeDisplayName?: string;
};

type Props = NotifyChannelAdminsStats & {
    onConfirm: () => void;
    onCancel: () => void;
    onExited: () => void;
};

/**
 * Confirms sending a batched system-bot DM to every unique channel admin
 * missing a value for this attribute. Resolves false on both explicit Cancel
 * and any other dismissal (backdrop click, Esc) -- there is no third outcome
 * for the caller to distinguish.
 *
 * The modal only resolves the confirmation; it does not perform the POST
 * itself. The caller (ChannelsResourceSettings) owns the request and the
 * resulting banner state, since the modal closes immediately on confirm and
 * the send result needs somewhere to land after it's gone.
 */
export const useNotifyChannelAdmins = () => {
    const dispatch = useDispatch();

    return (stats: NotifyChannelAdminsStats): Promise<boolean> => {
        return new Promise<boolean>((resolve) => {
            let resolved = false;
            const resolveOnce = (value: boolean) => {
                if (!resolved) {
                    resolved = true;
                    resolve(value);
                }
            };

            dispatch(openModal({
                modalId: ModalIdentifiers.GLOBAL_ATTRIBUTE_NOTIFY_CHANNEL_ADMINS,
                dialogType: NotifyChannelAdminsModal,
                dialogProps: {
                    ...stats,
                    onConfirm: () => resolveOnce(true),
                    onCancel: () => resolveOnce(false),
                    onExited: () => resolveOnce(false),
                },
            }));
        });
    };
};

function NotifyChannelAdminsModal({
    totalCount,
    uniqueAdminCount,
    noAdminCount,
    messagePreview,
    attributeDisplayName,
    onConfirm,
    onCancel,
    onExited,
}: Props) {
    const {formatMessage} = useIntl();

    const attribute = attributeDisplayName || formatMessage({
        id: 'admin.global_attributes.applies_to.channels.missing_values.attribute_fallback',
        defaultMessage: 'this attribute',
    });

    const title = formatMessage({
        id: 'admin.global_attributes.confirm.notify_channel_admins.title',
        defaultMessage: 'Notify all channel admins?',
    });

    const confirmButtonText = formatMessage({
        id: 'admin.global_attributes.confirm.notify_channel_admins.confirm',
        defaultMessage: 'Send notification',
    });

    const notifiedChannelCount = totalCount - noAdminCount;

    return (
        <GenericModal
            id='notifyChannelAdminsModal'
            dataTestId='notifyChannelAdminsModal'
            confirmButtonText={confirmButtonText}
            handleCancel={onCancel}
            handleConfirm={onConfirm}
            modalHeaderText={title}
            onExited={onExited}
            compassDesign={true}
        >
            <p>
                <strong>
                    <FormattedMessage
                        id='admin.global_attributes.confirm.notify_channel_admins.summary'
                        defaultMessage='{count, plural, one {# channel needs} other {# channels need}} a {attribute} value.'
                        values={{count: totalCount, attribute}}
                    />
                </strong>
            </p>
            <ul>
                <li>
                    <FormattedMessage
                        id='admin.global_attributes.confirm.notify_channel_admins.bullet.unique_admins'
                        defaultMessage='Notifications go to each unique channel admin — not one message per channel — so {adminCount, plural, one {# admin} other {# admins}} will be contacted for {channelCount, plural, one {# channel} other {# channels}}.'
                        values={{adminCount: uniqueAdminCount, channelCount: notifiedChannelCount}}
                    />
                </li>
                {noAdminCount > 0 && (
                    <li>
                        <FormattedMessage
                            id='admin.global_attributes.confirm.notify_channel_admins.bullet.no_admin'
                            defaultMessage="{count, plural, one {# channel has} other {# channels have}} no channel admin and won't be notified."
                            values={{count: noAdminCount}}
                        />
                    </li>
                )}
            </ul>
            <div className='NotifyChannelAdminsModal__previewLabel'>
                <FormattedMessage
                    id='admin.global_attributes.confirm.notify_channel_admins.message_label'
                    defaultMessage='Message to be sent'
                />
            </div>
            {/* Rendered as the DM will be, so the admin previews the message,
                not its markdown source. */}
            <blockquote className='NotifyChannelAdminsModal__preview'>
                <Markdown message={messagePreview}/>
            </blockquote>
        </GenericModal>
    );
}

export default NotifyChannelAdminsModal;
