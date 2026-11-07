// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React, {useMemo, useState} from 'react';
import {defineMessages, FormattedMessage, useIntl} from 'react-intl';

import {ChevronDownIcon, ChevronRightIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/compass-ui/components/button';

import ChannelsResourceSettings from './channels_resource_settings';
import {summarizeChannelResource} from './summary';
import type {ChannelResourceConfig} from './types';

import ResourceTypeIcon from '../../attribute_details/resource_type_icon';

import './channels_resource_row.scss';

// A constant, not generated per instance: the card offers Channels once, so only
// one row can exist per attribute.
const BODY_ID = 'channelsResourceRowBody';

type Props = {
    value: ChannelResourceConfig;
    onChange: (next: ChannelResourceConfig) => void;
    onRemove: () => void;

    // Whether the attribute's values have a defined order, i.e. it is rank-typed.
    // Raise-only and lower-only are meaningless without one, so they are not offered.
    ordered?: boolean;

    disabled?: boolean;

    // The persisted linked channel field's ID (undefined in create mode) and
    // the attribute's display name, forwarded to the Required toggle's
    // missing-values banner (MM-70717).
    channelFieldId?: string;
    attributeDisplayName?: string;

    // The persisted channel field's update_at. A host that stays on the page
    // after Save changes it, which refetches the saved Required state and
    // the missing-values count.
    channelFieldUpdateAt?: number;
};

/**
 * The Channels entry in an attribute's "Applies to" card.
 *
 * Never dispatches and never saves: the hosting page owns the form and its Save,
 * so this drops into a card owned by another team without either side owning
 * half a save.
 */
const ChannelsResourceRow = ({value, onChange, onRemove, ordered, disabled, channelFieldId, attributeDisplayName, channelFieldUpdateAt}: Props) => {
    const intl = useIntl();
    const {formatMessage} = intl;
    const [expanded, setExpanded] = useState(true);

    const summary = useMemo(() => summarizeChannelResource(value, intl), [value, intl]);

    return (
        <div
            className={classNames('ChannelsResourceRow', {'ChannelsResourceRow--open': expanded})}
            data-testid='channelsResourceRow'
        >
            <div className='ChannelsResourceRow__header'>
                <button
                    type='button'
                    className='ChannelsResourceRow__disclosure'
                    aria-expanded={expanded}
                    aria-controls={BODY_ID}
                    aria-label={formatMessage(expanded ? messages.collapse : messages.expand)}
                    onClick={() => setExpanded((current) => !current)}
                    data-testid='channelsResourceRowDisclosure'
                >
                    {expanded ? <ChevronDownIcon size={16}/> : <ChevronRightIcon size={16}/>}
                    <span className='ChannelsResourceRow__heading'>
                        <span className='ChannelsResourceRow__title'>
                            <ResourceTypeIcon type='channel'/>
                            <FormattedMessage {...messages.title}/>
                        </span>
                        <span
                            className='ChannelsResourceRow__summary'
                            data-testid='channelsResourceRowSummary'
                        >
                            {summary}
                        </span>
                    </span>
                </button>
                {expanded && (
                    <Button
                        type='button'
                        emphasis='tertiary'
                        destructive={true}
                        size='small'
                        className='ChannelsResourceRow__remove'
                        onClick={onRemove}
                        disabled={disabled}
                        data-testid='channelsResourceRowRemove'
                    >
                        <FormattedMessage {...messages.remove}/>
                    </Button>
                )}
            </div>

            {expanded && (
                <div
                    className='ChannelsResourceRow__body'
                    id={BODY_ID}
                >
                    <ChannelsResourceSettings
                        value={value}
                        onChange={onChange}
                        ordered={ordered}
                        disabled={disabled}
                        channelFieldId={channelFieldId}
                        attributeDisplayName={attributeDisplayName}
                        channelFieldUpdateAt={channelFieldUpdateAt}
                    />
                </div>
            )}
        </div>
    );
};

const messages = defineMessages({
    title: {id: 'admin.global_attributes.applies_to.channels.title', defaultMessage: 'Channels'},
    expand: {id: 'admin.global_attributes.applies_to.channels.expand', defaultMessage: 'Expand channel settings'},
    collapse: {id: 'admin.global_attributes.applies_to.channels.collapse', defaultMessage: 'Collapse channel settings'},
    remove: {id: 'admin.global_attributes.applies_to.channels.remove', defaultMessage: 'Remove resource'},
});

export default ChannelsResourceRow;
