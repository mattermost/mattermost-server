// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {defineMessages, FormattedMessage} from 'react-intl';

import {PlusIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/compass-ui/components/button';
import {Icon} from '@mattermost/compass-ui/components/icon';

import AdminPanel from 'components/widgets/admin_console/admin_panel';

import ChannelsResourceRow, {DEFAULT_CHANNEL_RESOURCE_CONFIG} from './channels';
import type {ChannelResourceConfig} from './channels';

import {resourceTypeLabels} from '../attribute_details/attribute_applies_to_constants';
import type {ResourceObjectType} from '../attribute_details/attribute_applies_to_constants';
import ResourceTypeIcon from '../attribute_details/resource_type_icon';

import './applies_to_card.scss';

type Props = {

    // null means the attribute does not apply to channels.
    channelResource: ChannelResourceConfig | null;
    onChannelResourceChange: (next: ChannelResourceConfig | null) => void;

    // Resources the attribute already applies to that this card does not edit,
    // each with the unique name its own field carries. That name can differ from
    // the template's -- Classification Markings applies `classification` to Users
    // as `clearance` -- and it is what a CEL rule spells, so without this the
    // attribute a policy author picks has no visible home here.
    readOnlyResources?: Array<{type: ResourceObjectType; name: string}>;

    // Removal is only a form action while the resource is unsaved. A host editing an
    // attribute that already exists has a field to delete, and values that go with
    // it, so it takes the removal over.
    onChannelResourceRemove?: () => void;

    // Whether the attribute's values are ranked; gates the raise/lower policies.
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
 * Temporary host for the Channels resource row.
 *
 * Global Attributes owns the real "Applies to" card, which does not exist yet.
 * Delete this file when it lands; ChannelsResourceRow is the part that survives.
 */
const AppliesToCard = ({channelResource, onChannelResourceChange, onChannelResourceRemove, readOnlyResources = [], ordered, disabled, channelFieldId, attributeDisplayName, channelFieldUpdateAt}: Props) => {
    return (
        <AdminPanel
            id='applies_to'
            className='AppliesToCard'
            title={messages.title}
            subtitle={messages.subtitle}
        >
            {readOnlyResources.length > 0 && (
                <ul
                    className='AppliesToCard__readOnlyResources'
                    data-testid='appliesToReadOnlyResources'
                >
                    {readOnlyResources.map((resource) => (
                        <li key={resource.type}>
                            <ResourceTypeIcon type={resource.type}/>
                            <FormattedMessage
                                {...messages.readOnlyResource}
                                values={{
                                    resource: <FormattedMessage {...resourceTypeLabels[resource.type]}/>,
                                    name: <code>{resource.name}</code>,
                                }}
                            />
                        </li>
                    ))}
                </ul>
            )}
            {channelResource && (
                <div className='AppliesToCard__rows'>
                    <ChannelsResourceRow
                        value={channelResource}
                        onChange={onChannelResourceChange}
                        onRemove={onChannelResourceRemove ?? (() => onChannelResourceChange(null))}
                        ordered={ordered}
                        disabled={disabled}
                        channelFieldId={channelFieldId}
                        attributeDisplayName={attributeDisplayName}
                        channelFieldUpdateAt={channelFieldUpdateAt}
                    />
                </div>
            )}
            {!channelResource && readOnlyResources.length === 0 && (
                <p
                    className='AppliesToCard__empty'
                    data-testid='appliesToEmpty'
                >
                    <FormattedMessage {...messages.empty}/>
                </p>
            )}
            {!channelResource && (
                <div className='AppliesToCard__footer'>
                    <Button
                        type='button'
                        emphasis='tertiary'
                        className='AppliesToCard__addResource'
                        onClick={() => onChannelResourceChange({...DEFAULT_CHANNEL_RESOURCE_CONFIG})}
                        disabled={disabled}
                        data-testid='appliesToAddResource'
                        leadingIcon={<Icon glyph={<PlusIcon/>}/>}
                    >
                        <FormattedMessage {...messages.addResource}/>
                    </Button>
                </div>
            )}
        </AdminPanel>
    );
};

const messages = defineMessages({
    title: {id: 'admin.global_attributes.applies_to.title', defaultMessage: 'Applies to'},
    subtitle: {id: 'admin.global_attributes.applies_to.subtitle', defaultMessage: 'Resources this attribute applies to, and how it behaves on each.'},
    addResource: {id: 'admin.global_attributes.applies_to.add_resource', defaultMessage: 'Add resource'},
    empty: {id: 'admin.global_attributes.applies_to.empty', defaultMessage: 'This attribute does not apply to any resource yet.'},
    readOnlyResource: {
        id: 'admin.global_attributes.applies_to.read_only_resource',
        defaultMessage: 'Applied to {resource} as {name}',
    },
});

export default AppliesToCard;
