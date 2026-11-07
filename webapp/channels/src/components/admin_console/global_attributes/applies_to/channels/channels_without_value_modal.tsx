// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef, useState} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import {GlobeIcon, LockOutlineIcon, OpenInNewIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/compass-ui/components/button';
import {GenericModal} from '@mattermost/components';
import type {ChannelMissingAttributeValue} from '@mattermost/types/properties';

import {openModal} from 'actions/views/modals';

import ExternalLink from 'components/external_link';
import LoadingScreen from 'components/loading_screen';

import {ModalIdentifiers} from 'utils/constants';

import {fetchChannelsMissingValue} from '../../utils';

import './channels_without_value_modal.scss';

const PER_PAGE = 20;

type Filter = 'all' | 'local' | 'shared';

type Props = {
    fieldId: string;
    attributeDisplayName?: string;

    // Seeds the subtitle count before the first page resolves. Includes shared
    // channels, matching the list endpoint's total_count.
    totalCount: number;

    // Whole-list count of shared channels, from the compliance summary. The
    // list itself is paginated, so per-page rows cannot drive the filter tabs.
    sharedCount?: number;
    onExited: () => void;
};

export const useChannelsWithoutValueModal = () => {
    const dispatch = useDispatch();

    return (args: {fieldId: string; attributeDisplayName?: string; totalCount: number; sharedCount?: number}) => {
        dispatch(openModal({
            modalId: ModalIdentifiers.GLOBAL_ATTRIBUTE_CHANNELS_WITHOUT_VALUE,
            dialogType: ChannelsWithoutValueModal,
            dialogProps: args,
        }));
    };
};

function ChannelsWithoutValueModal({fieldId, attributeDisplayName, totalCount: seedTotalCount, sharedCount = 0, onExited}: Props) {
    const {formatMessage} = useIntl();

    const [channels, setChannels] = useState<ChannelMissingAttributeValue[]>([]);
    const [totalCount, setTotalCount] = useState(seedTotalCount);
    const [loading, setLoading] = useState(true);
    const [failed, setFailed] = useState(false);
    const [page, setPage] = useState(0);
    const [filter, setFilter] = useState<Filter>('all');

    // Guards against an out-of-order response overwriting a newer page/filter's
    // results.
    const seqRef = useRef(0);

    const load = useCallback((targetPage: number) => {
        const mySeq = ++seqRef.current;
        setLoading(true);
        setFailed(false);

        fetchChannelsMissingValue(fieldId, targetPage, PER_PAGE).then((data) => {
            if (seqRef.current !== mySeq) {
                return;
            }
            setChannels(data.channels);
            setTotalCount(data.total_count);
            setLoading(false);
        }).catch(() => {
            if (seqRef.current !== mySeq) {
                return;
            }
            setFailed(true);
            setLoading(false);
        });
    }, [fieldId]);

    useEffect(() => {
        load(0);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [fieldId]);

    const attribute = attributeDisplayName || formatMessage({
        id: 'admin.global_attributes.applies_to.channels.missing_values.attribute_fallback',
        defaultMessage: 'this attribute',
    });

    const title = formatMessage({
        id: 'admin.global_attributes.applies_to.channels.without_value.title',
        defaultMessage: 'Channels without a value',
    });

    const subtitle = formatMessage({
        id: 'admin.global_attributes.applies_to.channels.without_value.subtitle',
        defaultMessage: '{count, plural, one {# channel} other {# channels}} still need a {attribute} value',
    }, {count: totalCount, attribute});

    const headerNode = (
        <>
            {title}
            <span
                className='ChannelsWithoutValueModal__header-divider'
                aria-hidden='true'
            />
            <span className='ChannelsWithoutValueModal__header-count'>
                {subtitle}
            </span>
        </>
    );

    const visibleChannels = channels.filter((ch) => {
        if (filter === 'local') {
            return ch.is_local;
        }
        if (filter === 'shared') {
            return !ch.is_local;
        }
        return true;
    });

    const localCount = Math.max(0, totalCount - sharedCount);

    const handlePrevious = () => {
        const next = Math.max(0, page - 1);
        setPage(next);
        load(next);
    };

    // Paged by the server's total, not by this page's row count: a full last
    // page would otherwise leave Next enabled onto an empty page.
    const hasNextPage = (page + 1) * PER_PAGE < totalCount;

    const handleNext = () => {
        const next = page + 1;
        setPage(next);
        load(next);
    };

    return (
        <GenericModal
            id='channelsWithoutValueModal'
            className='ChannelsWithoutValueModal a11y__modal'
            dataTestId='channelsWithoutValueModal'
            compassDesign={true}
            show={true}
            onHide={onExited}
            onExited={onExited}
            modalHeaderText={headerNode}
            showCloseButton={true}
            bodyPadding={true}
            bodyDivider={true}
            footerDivider={true}

            // GenericModal renders either its own confirm/cancel buttons or
            // footerContent, never both, so Close lives here with the pager.
            footerContent={(
                <div className='ChannelsWithoutValueModal__footer'>
                    <div className='ChannelsWithoutValueModal__pagination'>
                        <Button
                            emphasis='tertiary'
                            disabled={page === 0 || loading}
                            onClick={handlePrevious}
                        >
                            <FormattedMessage
                                id='admin.global_attributes.applies_to.channels.without_value.previous'
                                defaultMessage='Previous'
                            />
                        </Button>
                        <Button
                            emphasis='tertiary'
                            disabled={loading || !hasNextPage}
                            onClick={handleNext}
                        >
                            <FormattedMessage
                                id='admin.global_attributes.applies_to.channels.without_value.next'
                                defaultMessage='Next'
                            />
                        </Button>
                    </div>
                    <Button
                        onClick={onExited}
                        data-testid='channelsWithoutValueModalClose'
                    >
                        <FormattedMessage
                            id='admin.global_attributes.applies_to.channels.without_value.close'
                            defaultMessage='Close'
                        />
                    </Button>
                </div>
            )}
        >
            {sharedCount > 0 && (
                <div className='ChannelsWithoutValueModal__filters'>
                    <button
                        type='button'
                        className={filter === 'all' ? 'active' : ''}
                        onClick={() => setFilter('all')}
                    >
                        {formatMessage({id: 'admin.global_attributes.applies_to.channels.without_value.filter.all', defaultMessage: 'All ({count})'}, {count: totalCount})}
                    </button>
                    <button
                        type='button'
                        className={filter === 'local' ? 'active' : ''}
                        onClick={() => setFilter('local')}
                    >
                        {formatMessage({id: 'admin.global_attributes.applies_to.channels.without_value.filter.local', defaultMessage: 'Local ({count})'}, {count: localCount})}
                    </button>
                    <button
                        type='button'
                        className={filter === 'shared' ? 'active' : ''}
                        onClick={() => setFilter('shared')}
                    >
                        {formatMessage({id: 'admin.global_attributes.applies_to.channels.without_value.filter.shared', defaultMessage: 'Shared ({count})'}, {count: sharedCount})}
                    </button>
                </div>
            )}
            {loading && channels.length === 0 && <LoadingScreen/>}
            {failed && (
                <div className='ChannelsWithoutValueModal__error'>
                    <FormattedMessage
                        id='admin.global_attributes.applies_to.channels.without_value.error'
                        defaultMessage="The channel list couldn't be loaded."
                    />
                    <button
                        type='button'
                        onClick={() => load(page)}
                    >
                        <FormattedMessage
                            id='admin.global_attributes.applies_to.channels.without_value.retry'
                            defaultMessage='Try again'
                        />
                    </button>
                </div>
            )}
            {!loading && !failed && totalCount === 0 && (
                <div className='ChannelsWithoutValueModal__empty'>
                    <FormattedMessage
                        id='admin.global_attributes.applies_to.channels.without_value.empty'
                        defaultMessage='Every channel has a value'
                    />
                </div>
            )}
            {!loading && !failed && totalCount > 0 && visibleChannels.length === 0 && (
                <div className='ChannelsWithoutValueModal__empty'>
                    <FormattedMessage
                        id='admin.global_attributes.applies_to.channels.without_value.empty_filtered_page'
                        defaultMessage='No matching channels on this page'
                    />
                </div>
            )}
            {!failed && visibleChannels.length > 0 && (
                <table className='ChannelsWithoutValueModal__table'>
                    <thead>
                        <tr>
                            <th>
                                <FormattedMessage
                                    id='admin.global_attributes.applies_to.channels.without_value.column.channel'
                                    defaultMessage='Channel'
                                />
                            </th>
                            <th>
                                <FormattedMessage
                                    id='admin.global_attributes.applies_to.channels.without_value.column.admin'
                                    defaultMessage='Channel admin'
                                />
                            </th>
                        </tr>
                    </thead>
                    <tbody>
                        {visibleChannels.map((ch) => (
                            <tr key={ch.channel_id}>
                                <td>
                                    <div className='ChannelsWithoutValueModal__channel-cell'>
                                        {ch.channel_type === 'P' ? <LockOutlineIcon size={16}/> : <GlobeIcon size={16}/>}
                                        <ExternalLink
                                            href={`/admin_console/user_management/channels/${ch.channel_id}`}
                                            location='channels_without_value_modal'
                                            aria-label={formatMessage({
                                                id: 'admin.global_attributes.applies_to.channels.without_value.open_channel_aria',
                                                defaultMessage: 'Open {channel} in a new tab',
                                            }, {channel: ch.channel_display_name})}
                                        >
                                            <span title={ch.channel_display_name}>{ch.channel_display_name}</span>
                                            <OpenInNewIcon size={14}/>
                                        </ExternalLink>
                                    </div>
                                </td>
                                <td>
                                    {ch.channel_admins.length > 0 ? (
                                        ch.channel_admins.map((a) => (a.first_name || a.last_name ? `${a.first_name} ${a.last_name}`.trim() : a.username)).join(', ')
                                    ) : (
                                        <em>
                                            <FormattedMessage
                                                id='admin.global_attributes.applies_to.channels.without_value.no_admin'
                                                defaultMessage='No channel admin'
                                            />
                                        </em>
                                    )}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            )}
        </GenericModal>
    );
}

export default ChannelsWithoutValueModal;
