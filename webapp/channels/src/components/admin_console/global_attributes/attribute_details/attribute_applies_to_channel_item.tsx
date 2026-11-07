// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React, {useMemo, useState, type JSX} from 'react';
import {defineMessages, FormattedMessage, useIntl} from 'react-intl';

import {ChevronDownIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/compass-ui/components/button';
import {WithTooltip} from '@mattermost/shared/components/tooltip';

import {resourceTypeLabels, type AttributeAppliesToChannelItemProps} from './attribute_applies_to_constants';
import ResourceTypeIcon from './resource_type_icon';

import ChannelsResourceSettings from '../applies_to/channels/channels_resource_settings';
import {summarizeChannelResource} from '../applies_to/channels/summary';

import './attribute_applies_to_item.scss';

const BODY_ID = 'attribute-applies-to-channel-panel';

// The Channels row of the Applies-to list -- owns its own expand/collapse
// state (deliberately not the shared Accordion component: AccordionCard
// renders the row itself from plain data with no slot for a child component to
// own it, and its open-row tracking is by array index, which misattributes
// state when a row is removed from the middle of the list). Remove is only
// reachable once expanded -- there is no collapsed-row remove affordance.
function AttributeAppliesToChannelItem({config, onConfigChange, ordered, disabled = false, lockedTooltip, removeLockedTooltip, onRemove, channelFieldId, attributeDisplayName}: AttributeAppliesToChannelItemProps): JSX.Element {
    const intl = useIntl();
    const {formatMessage} = intl;
    const [isOpen, setIsOpen] = useState(false);

    const label = formatMessage(resourceTypeLabels.channel);

    // Collapsed, the row is the only place the configuration is visible, so it
    // states it rather than leaving the admin to expand every row to find out.
    const summary = useMemo(() => summarizeChannelResource(config, intl), [config, intl]);
    const toggleLabel = formatMessage(isOpen ? messages.collapseLabel : messages.expandLabel, {label});

    const toggleButton = (
        <button
            type='button'
            className='AttributeAppliesToItem__toggle'
            onClick={() => setIsOpen((prev) => !prev)}
            disabled={disabled}
            aria-expanded={isOpen}
            aria-controls={BODY_ID}
            aria-label={toggleLabel}
            data-testid='attributeAppliesToRow-channel-toggle'
        >
            <ChevronDownIcon
                size={16}
                className={classNames('AttributeAppliesToItem__chevron', {'AttributeAppliesToItem__chevron--open': isOpen})}
            />
            <span className='AttributeAppliesToItem__heading'>
                <span className='AttributeAppliesToItem__name'>
                    <ResourceTypeIcon type='channel'/>
                    <span className='AttributeAppliesToItem__label'>{label}</span>
                </span>
                <span
                    className='AttributeAppliesToItem__summary'
                    data-testid='attributeAppliesToRow-channel-summary'
                >
                    {summary}
                </span>
            </span>
        </button>
    );

    const removeButton = (
        <Button
            type='button'
            emphasis='tertiary'
            destructive={true}
            size='small'
            className='AttributeAppliesToItem__remove'
            onClick={onRemove}
            disabled={disabled || Boolean(removeLockedTooltip)}
            data-testid='attributeAppliesToRow-channel-remove'
        >
            <FormattedMessage {...messages.removeLabel}/>
        </Button>
    );

    return (
        <div
            className={classNames('AttributeAppliesToItem', {'AttributeAppliesToItem--open': isOpen})}
            data-testid='attributeAppliesToRow-channel'
        >
            <div className='AttributeAppliesToItem__header'>
                {lockedTooltip ? (
                    <WithTooltip title={lockedTooltip}>
                        <span
                            // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- WithTooltip's useFocus only fires on its cloned child; without this the disabled toggle is unreachable by keyboard, so the tooltip explaining the lock is mouse-only
                            tabIndex={0}
                            data-testid='attributeAppliesToRow-channel-toggleLockWrap'
                        >
                            {toggleButton}
                        </span>
                    </WithTooltip>
                ) : toggleButton}
                {isOpen && (removeLockedTooltip ? (
                    <WithTooltip title={removeLockedTooltip}>
                        <span
                            // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- WithTooltip's useFocus only fires on its cloned child; without this the disabled Remove is unreachable by keyboard, so the tooltip explaining the lock is mouse-only
                            tabIndex={0}
                            data-testid='attributeAppliesToRow-channel-removeLockWrap'
                        >
                            {removeButton}
                        </span>
                    </WithTooltip>
                ) : removeButton)}
            </div>
            {isOpen && (
                <div
                    id={BODY_ID}
                    role='region'
                    aria-label={label}
                    className='AttributeAppliesToItem__body'
                    data-testid='attributeAppliesToRow-channel-body'
                >
                    <ChannelsResourceSettings
                        value={config}
                        onChange={onConfigChange}
                        ordered={ordered}
                        disabled={disabled}
                        channelFieldId={channelFieldId}
                        attributeDisplayName={attributeDisplayName}
                    />
                </div>
            )}
        </div>
    );
}

export default AttributeAppliesToChannelItem;

const messages = defineMessages({
    expandLabel: {id: 'admin.global_attributes.attribute_details.applies_to.item.expand', defaultMessage: 'Expand {label}'},
    collapseLabel: {id: 'admin.global_attributes.attribute_details.applies_to.item.collapse', defaultMessage: 'Collapse {label}'},
    removeLabel: {id: 'admin.global_attributes.attribute_details.applies_to.item.remove', defaultMessage: 'Remove resource'},
});
