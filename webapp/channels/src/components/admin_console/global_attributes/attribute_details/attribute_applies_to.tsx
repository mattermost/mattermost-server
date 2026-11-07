// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import type {ComponentType, JSX, ReactNode} from 'react';
import React, {useMemo} from 'react';
import {defineMessages, FormattedMessage, useIntl} from 'react-intl';

import {PlusIcon} from '@mattermost/compass-icons/components';
import {buttonClassNames, type ButtonEmphasis} from '@mattermost/shared/components/button';
import type {FieldVisibility} from '@mattermost/types/properties';

import * as Menu from 'components/menu';
import AdminPanel from 'components/widgets/admin_console/admin_panel';

import AttributeAppliesToChannelItem from './attribute_applies_to_channel_item';
import {ALL_RESOURCE_TYPES, ATTRIBUTE_APPLIES_TO_ADD_HEADER_TRIGGER_ID, resourceTypeLabels} from './attribute_applies_to_constants';
import type {AttributeAppliesToItemProps, ResourceObjectType, UserManagedValue} from './attribute_applies_to_constants';
import AttributeAppliesToPostItem from './attribute_applies_to_post_item';
import AttributeAppliesToUserItem from './attribute_applies_to_user_item';
import type {ExternalSource} from './external_source';
import ResourceTypeIcon from './resource_type_icon';

import type {ChannelResourceConfig} from '../applies_to/channels/types';

import './attribute_applies_to.scss';

type Props = {
    appliesTo: ResourceObjectType[];
    disabled?: boolean;

    // Distinct from `disabled` -- driven purely by plugin ownership, not by
    // saving/in-flight state, since hiding+reappearing the trigger on every
    // save would be a distracting layout shift for the normal editing path.
    // When true, both "Add resource" triggers are omitted entirely (not
    // merely disabled).
    hideAddResource?: boolean;

    // Resource types the picker still offers but cannot add right now, mapped
    // to a short reason shown under the option. Offered-and-disabled rather
    // than hidden: an option that silently vanishes is indistinguishable from
    // one this server does not have.
    blockedTypes?: Partial<Record<ResourceObjectType, string>>;

    // Explains WHY existing rows' toggle is disabled, when the reason isn't
    // the transient `saving` state -- threaded straight through to each row's
    // AttributeAppliesToItemProps.lockedTooltip. Undefined renders no tooltip.
    lockedTooltip?: ReactNode;

    // Locks Remove on the rows of the given types and explains why; the rest of
    // each row follows `disabled`.
    removeLockedTooltips?: Partial<Record<ResourceObjectType, ReactNode>>;
    onAdd: (type: ResourceObjectType) => void;
    onRemove: (type: ResourceObjectType) => void;

    // Users-only config, forwarded to AttributeAppliesToUserItem alone (see the
    // render loop below) -- Posts gets its own config props once it has
    // equivalent controls to expose.
    userVisibility?: FieldVisibility;
    onUserVisibilityChange?: (visibility: FieldVisibility) => void;
    userManaged?: UserManagedValue;
    onUserManagedChange?: (managed: UserManagedValue) => void;
    userWhoCanSetLockedTooltip?: ReactNode;

    // Set when the attribute's values are synced from AD/LDAP or SAML, which
    // the Users row surfaces as a read-only "Managed by" indicator.
    externalSource?: ExternalSource;

    // Channels only: the settings its row edits, held by the page because the
    // linked channel field is built from them on Save.
    channelResource: ChannelResourceConfig;
    onChannelResourceChange: (next: ChannelResourceConfig) => void;

    // Whether the attribute is rank-typed, which gates the directional change
    // policies on the Channels row.
    ordered?: boolean;

    // Resources this server may offer. Channels is dropped below Enterprise
    // Advanced, or with the ChannelAttributes flag off.
    allowedTypes?: ResourceObjectType[];

    // Channels only: the persisted linked channel field's ID (undefined in
    // create mode) and the attribute's own display name, both forwarded to
    // the Required toggle's missing-values banner (MM-70717).
    channelFieldId?: string;
    attributeDisplayName?: string;
};

// Every entry here must implement AttributeAppliesToItemProps exactly --
// TypeScript rejects the map itself if either row component's props drift
// from that shared signature, rather than only failing wherever they happen
// to get used. Channels is absent: it is the one row with settings of its
// own, so it takes props the other two have no use for.
const RESOURCE_TYPE_ITEM_COMPONENTS: Record<Exclude<ResourceObjectType, 'channel'>, ComponentType<AttributeAppliesToItemProps>> = {
    user: AttributeAppliesToUserItem,
    post: AttributeAppliesToPostItem,
};

// Owns only the AdminPanel chrome (header, "Add resource" triggers, empty state)
// and renders one per-type row component per entry in appliesTo (a dedicated
// component per resource type -- AttributeAppliesToUserItem/ChannelItem/
// PostItem -- rather than one generic item parameterized by resourceType).
// Holds no selection state of its own -- "available" picker options are
// derived purely from props on every render. Makes no data-mutating dispatch
// calls, no Client4/API calls (the page owns all of that).
function AttributeAppliesTo({
    appliesTo,
    disabled = false,
    hideAddResource = false,
    blockedTypes,
    lockedTooltip,
    removeLockedTooltips,
    onAdd,
    onRemove,
    userVisibility,
    onUserVisibilityChange,
    userManaged,
    onUserManagedChange,
    externalSource,
    userWhoCanSetLockedTooltip,
    channelResource,
    onChannelResourceChange,
    ordered,
    allowedTypes = ALL_RESOURCE_TYPES,
    channelFieldId,
    attributeDisplayName,
}: Props): JSX.Element {
    const {formatMessage} = useIntl();

    const availableTypes = useMemo(
        () => ALL_RESOURCE_TYPES.filter((type) => allowedTypes.includes(type) && !appliesTo.includes(type)),
        [appliesTo, allowedTypes],
    );

    const renderAddResourceMenu = (triggerId: string, dataTestId: string, label: string, emphasis: ButtonEmphasis) => (
        <Menu.Container
            menuButton={{
                id: triggerId,
                class: classNames(buttonClassNames({emphasis}), 'AttributeAppliesTo__trigger'),
                disabled,
                'aria-label': label,
                children: (
                    <>
                        <PlusIcon size={16}/>
                        {label}
                    </>
                ),
                dataTestId,
            }}
            menu={{
                id: `${triggerId}-menu`,
                'aria-label': label,
            }}
        >
            {availableTypes.map((type) => {
                const blockedReason = blockedTypes?.[type];
                return (
                    <Menu.Item
                        id={`${triggerId}-${type}`}
                        key={type}
                        disabled={Boolean(blockedReason)}
                        leadingElement={<ResourceTypeIcon type={type}/>}
                        onClick={() => onAdd(type)}
                        labels={(
                            <>
                                <span><FormattedMessage {...resourceTypeLabels[type]}/></span>
                                {blockedReason && <span>{blockedReason}</span>}
                            </>
                        )}
                    />
                );
            })}
        </Menu.Container>
    );

    const headerAddResource = availableTypes.length > 0 && !hideAddResource ? renderAddResourceMenu(
        ATTRIBUTE_APPLIES_TO_ADD_HEADER_TRIGGER_ID,
        'attributeAppliesToAddResourceButtonHeader',
        formatMessage(messages.addResourceHeader),
        'tertiary',
    ) : undefined;

    return (
        <div data-testid='attributeAppliesTo'>
            <AdminPanel
                id='attribute_applies_to'
                className='AttributeAppliesTo'
                title={messages.title}
                subtitle={messages.subtitle}
                button={headerAddResource}
            >
                {appliesTo.length === 0 ? (
                    <div
                        className='AttributeAppliesTo__emptyState'
                        data-testid='attributeAppliesToEmptyState'
                    >
                        <h5 className='AttributeAppliesTo__emptyStateHeading'>
                            <FormattedMessage {...messages.emptyStateHeading}/>
                        </h5>
                        <p className='AttributeAppliesTo__emptyStateHelperText'>
                            <FormattedMessage {...(hideAddResource ? messages.emptyStateHelperTextPluginOwned : messages.emptyStateHelperText)}/>
                        </p>
                        {availableTypes.length > 0 && !hideAddResource && renderAddResourceMenu(
                            'attribute-applies-to-add-inline',
                            'attributeAppliesToAddResourceButtonInline',
                            formatMessage(messages.addResourceHeader),
                            'primary',
                        )}
                    </div>
                ) : (
                    <>
                        <div className='AttributeAppliesTo__list'>
                            {appliesTo.map((type) => {
                                if (type === 'channel') {
                                    return (
                                        <AttributeAppliesToChannelItem
                                            key={type}
                                            config={channelResource}
                                            onConfigChange={onChannelResourceChange}
                                            ordered={ordered}
                                            disabled={disabled}
                                            lockedTooltip={lockedTooltip}
                                            removeLockedTooltip={removeLockedTooltips?.[type]}
                                            onRemove={() => onRemove(type)}
                                            channelFieldId={channelFieldId}
                                            attributeDisplayName={attributeDisplayName}
                                        />
                                    );
                                }

                                const Item = RESOURCE_TYPE_ITEM_COMPONENTS[type];
                                const userProps = type === 'user' ? {
                                    visibility: userVisibility,
                                    onVisibilityChange: onUserVisibilityChange,
                                    managed: userManaged,
                                    onManagedChange: onUserManagedChange,
                                    externalSource,
                                    whoCanSetLockedTooltip: userWhoCanSetLockedTooltip,
                                } : {};
                                return (
                                    <Item
                                        key={type}
                                        disabled={disabled}
                                        lockedTooltip={lockedTooltip}
                                        removeLockedTooltip={removeLockedTooltips?.[type]}
                                        onRemove={() => onRemove(type)}
                                        {...userProps}
                                    />
                                );
                            })}
                        </div>
                        {availableTypes.length > 0 && !hideAddResource && renderAddResourceMenu(
                            'attribute-applies-to-add-inline',
                            'attributeAppliesToAddResourceButtonInline',
                            formatMessage(messages.addResourceInline),
                            'tertiary',
                        )}
                    </>
                )}
            </AdminPanel>
        </div>
    );
}

export default AttributeAppliesTo;

const messages = defineMessages({
    title: {id: 'admin.global_attributes.attribute_details.applies_to.title', defaultMessage: 'Applies to'},
    subtitle: {id: 'admin.global_attributes.attribute_details.applies_to.subtitle', defaultMessage: 'Resources this attribute applies to, and who can set the value on each.'},
    emptyStateHeading: {id: 'admin.global_attributes.attribute_details.applies_to.empty_state.heading', defaultMessage: 'No resources yet'},
    emptyStateHelperText: {
        id: 'admin.global_attributes.attribute_details.applies_to.empty_state.helper_text',
        defaultMessage: 'Add a resource to apply this attribute to users, channels, or posts.',
    },
    emptyStateHelperTextPluginOwned: {
        id: 'admin.global_attributes.attribute_details.applies_to.empty_state.helper_text_plugin_owned',
        defaultMessage: "Resources for plugin-managed attributes can't be added here.",
    },
    addResourceHeader: {id: 'admin.global_attributes.attribute_details.applies_to.add_resource_header', defaultMessage: 'Add resource'},
    addResourceInline: {id: 'admin.global_attributes.attribute_details.applies_to.add_resource_inline', defaultMessage: 'Add another resource'},
});
