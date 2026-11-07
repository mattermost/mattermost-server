// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ComponentType, ReactNode} from 'react';
import type {MessageDescriptor} from 'react-intl';
import {defineMessages} from 'react-intl';

import {AccountOutlineIcon, MessageTextOutlineIcon, ProductChannelsIcon} from '@mattermost/compass-icons/components';
import type IconProps from '@mattermost/compass-icons/components/props';
import type {FieldVisibility} from '@mattermost/types/properties';

import type {ExternalSource} from './external_source';

import type {ChannelResourceConfig} from '../applies_to/channels/types';

export type ResourceObjectType = 'user' | 'channel' | 'post';

// Mirrors CPA's attrs.managed value domain exactly (see user_properties_dot_menu.tsx) --
// '' means member-editable, 'admin' means locked to System Administrator. Deliberately
// not the richer PSAv2 PropertyPermissionLevel type: this writes the same value CPA already
// writes, not a parallel permission mechanism.
export type UserManagedValue = '' | 'admin';

// Fixed Users -> Channels -> Posts order used everywhere a resource list is
// rendered (the picker menu, and used to derive "available" options) -- not
// the insertion order of a saved appliesTo array, which is separate.
export const ALL_RESOURCE_TYPES: ResourceObjectType[] = ['user', 'channel', 'post'];

// Every per-resource-type row component (AttributeAppliesToUserItem/
// ChannelItem/PostItem) implements exactly this prop signature -- there's no
// resourceType prop, since each component already knows its own type. Sharing
// one type here (rather than each component declaring an identical local
// `Props`) is what AttributeAppliesTo relies on to treat all three
// interchangeably in its render switch.
export type AttributeAppliesToItemProps = {
    disabled?: boolean;

    // Explains WHY the row's toggle is disabled, when the reason isn't the
    // transient in-flight `saving` state -- mirrors the Type/Unique-Name
    // lock tooltip convention on the parent page. Undefined (the `saving`
    // case) renders no tooltip, matching today's existing behavior.
    lockedTooltip?: ReactNode;

    // Locks only Remove and explains why; the rest of the row follows `disabled`.
    removeLockedTooltip?: ReactNode;
    onRemove: () => void;

    // Users-only config. Optional so this shared prop type still fits
    // AttributeAppliesToChannelItem/AttributeAppliesToPostItem, which don't have
    // a config panel yet and simply don't destructure these -- Channels/Posts
    // should define their own config shape once they need one, not inherit this one.
    visibility?: FieldVisibility;
    onVisibilityChange?: (visibility: FieldVisibility) => void;
    managed?: UserManagedValue;
    onManagedChange?: (managed: UserManagedValue) => void;

    // The external system this attribute's values are synced from, when there
    // is one. Undefined for attributes managed in Mattermost or by a plugin.
    externalSource?: ExternalSource;

    // Locks only "Who can set the value" at its stored value and explains why;
    // Profile display keeps following `disabled`; Remove has its own lock.
    whoCanSetLockedTooltip?: ReactNode;
};

// Channels is the one resource with settings of its own, so its row takes the
// shared props plus the configuration it edits. The page owns that state: it is
// what the linked channel field is built from at save time.
export type AttributeAppliesToChannelItemProps = AttributeAppliesToItemProps & {
    config: ChannelResourceConfig;
    onConfigChange: (next: ChannelResourceConfig) => void;

    // Whether the attribute is rank-typed, which is what makes the directional
    // change policies meaningful.
    ordered?: boolean;

    // The persisted linked channel field's ID, for the Required toggle's
    // missing-values banner (MM-70717). Undefined in create mode.
    channelFieldId?: string;
    attributeDisplayName?: string;
};

// Shared between AttributeAppliesTo (which owns the button) and AttributeDetails
// (which moves focus back to it after a pre-save resource removal).
export const ATTRIBUTE_APPLIES_TO_ADD_HEADER_TRIGGER_ID = 'attribute-applies-to-add-header';

export const RESOURCE_TYPE_ICONS: Record<ResourceObjectType, ComponentType<IconProps>> = {
    user: AccountOutlineIcon,
    channel: ProductChannelsIcon,
    post: MessageTextOutlineIcon,
};

export const resourceTypeLabels: Record<ResourceObjectType, MessageDescriptor> = defineMessages({
    user: {id: 'admin.global_attributes.attribute_details.applies_to.resource_type.user', defaultMessage: 'Users'},
    channel: {id: 'admin.global_attributes.attribute_details.applies_to.resource_type.channel', defaultMessage: 'Channels'},
    post: {id: 'admin.global_attributes.attribute_details.applies_to.resource_type.post', defaultMessage: 'Posts'},
});
