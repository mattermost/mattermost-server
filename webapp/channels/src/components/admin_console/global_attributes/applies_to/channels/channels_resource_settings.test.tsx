// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {DISPLAY_BANNER_TOP, DISPLAY_LABEL_HEADER} from 'mattermost-redux/constants/properties';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';

import ChannelsResourceSettings from './channels_resource_settings';
import {useChannelsWithoutValueModal} from './channels_without_value_modal';
import {useNotifyChannelAdmins} from './notify_channel_admins_modal';
import {DEFAULT_CHANNEL_RESOURCE_CONFIG} from './types';
import useChannelMissingValues from './use_channel_missing_values';
import type {ChannelMissingValuesState} from './use_channel_missing_values';

import {notifyChannelAdminsOfMissingValue} from '../../utils';

jest.mock('./use_channel_missing_values');
jest.mock('./channels_without_value_modal');
jest.mock('./notify_channel_admins_modal');
jest.mock('../../utils', () => ({
    __esModule: true,
    notifyChannelAdminsOfMissingValue: jest.fn(),
}));

const mockUseChannelMissingValues = jest.mocked(useChannelMissingValues);
const mockUseChannelsWithoutValueModal = jest.mocked(useChannelsWithoutValueModal);
const mockUseNotifyChannelAdmins = jest.mocked(useNotifyChannelAdmins);
const mockNotify = jest.mocked(notifyChannelAdminsOfMissingValue);

function missingValuesState(overrides: Partial<ChannelMissingValuesState> = {}): ChannelMissingValuesState {
    return {
        loading: false,
        failed: false,
        summary: null,
        reload: jest.fn(),
        ...overrides,
    };
}

describe('ChannelsResourceSettings', () => {
    // The Required field (toggle + missing-values banner) is gated entirely on
    // this flag -- see requiredEnforcementEnabled in the component. Every test
    // below that exercises the toggle or the banner needs it on; the kill-switch
    // tests explicitly turn it off to prove the opposite.
    const requiredEnabledState: DeepPartial<GlobalState> = {
        entities: {
            general: {
                config: {
                    FeatureFlagChannelAttributes: 'true',
                    FeatureFlagChannelAttributesRequired: 'true',
                },
            },
        },
    };

    const openViewList = jest.fn();
    const promptNotify = jest.fn();

    // jsdom does not implement scrollIntoView at all; the blocked-click handler
    // calls it on the banner ref to bring attention to it.
    beforeAll(() => {
        Element.prototype.scrollIntoView = jest.fn();
    });

    beforeEach(() => {
        jest.clearAllMocks();
        mockUseChannelMissingValues.mockReturnValue(missingValuesState());
        mockUseChannelsWithoutValueModal.mockReturnValue(openViewList);
        mockUseNotifyChannelAdmins.mockReturnValue(promptNotify);
    });

    // Pass {} as state for the flag-off baseline: an explicit undefined would
    // fall back to the default and silently render with the flag on.
    const renderComponent = (
        props: Partial<React.ComponentProps<typeof ChannelsResourceSettings>> = {},
        state: DeepPartial<GlobalState> = requiredEnabledState,
    ) => {
        const onChange = jest.fn();
        renderWithContext(
            <ChannelsResourceSettings
                value={{...DEFAULT_CHANNEL_RESOURCE_CONFIG, ...props.value}}
                onChange={onChange}
                channelFieldId={props.channelFieldId}
                attributeDisplayName={props.attributeDisplayName}
                disabled={props.disabled}
                ordered={props.ordered}
            />,
            state,
        );
        return {onChange};
    };

    it('hides the Required toggle by default (ChannelAttributesRequired off)', () => {
        renderComponent({}, {});

        expect(screen.queryByTestId('channelsResourceRequired-button')).not.toBeInTheDocument();
        expect(screen.queryByText('Required')).not.toBeInTheDocument();
    });

    it('shows the Required toggle when ChannelAttributesRequired is on', () => {
        renderComponent();

        expect(screen.getByTestId('channelsResourceRequired-button')).toBeInTheDocument();
        expect(screen.getByText('Required')).toBeInTheDocument();
    });

    it('still shows every other setting when the Required toggle is hidden', () => {
        renderComponent({}, {});

        expect(screen.getByText('Display location')).toBeInTheDocument();
        expect(screen.getByText('Changing the value')).toBeInTheDocument();
    });

    it('renders Header and Banner location checkboxes', () => {
        renderComponent({value: {...DEFAULT_CHANNEL_RESOURCE_CONFIG, displayLocations: [DISPLAY_LABEL_HEADER]}}, {});

        const header = screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`);
        const banner = screen.getByTestId(`channelsResourceLocation-${DISPLAY_BANNER_TOP}`);

        expect(header).toBeChecked();
        expect(banner).not.toBeChecked();
        expect(screen.getByText('Header')).toBeInTheDocument();
        expect(screen.getByText('Banner')).toBeInTheDocument();
    });

    it('toggles a display location via Compass Checkbox', async () => {
        const {onChange} = renderComponent({value: {...DEFAULT_CHANNEL_RESOURCE_CONFIG, displayLocations: []}}, {});

        await userEvent.click(screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`));

        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({
            displayLocations: [DISPLAY_LABEL_HEADER],
        }));
    });

    it('disables location checkboxes when disabled', () => {
        renderComponent({disabled: true}, {});

        expect(screen.getByTestId(`channelsResourceLocation-${DISPLAY_LABEL_HEADER}`)).toBeDisabled();
        expect(screen.getByTestId(`channelsResourceLocation-${DISPLAY_BANNER_TOP}`)).toBeDisabled();
    });

    it('keeps the Required control as a switch, not a Compass Checkbox', () => {
        renderComponent();

        expect(screen.getByTestId('channelsResourceRequired-button')).toHaveAttribute('role', 'switch');
    });

    it('renders no banner when nothing is missing', () => {
        renderComponent();

        expect(screen.queryByText('Set channel attribute values')).not.toBeInTheDocument();
    });

    it('does not call onChange when the toggle is clicked while blocked, but the Toggle stays enabled', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 26,
            sharedCount: 0,
            uniqueAdminCount: 10,
            noAdminCount: 2,
            messagePreview: 'preview',
            required: false,
        }}));

        const {onChange} = renderComponent({value: {...DEFAULT_CHANNEL_RESOURCE_CONFIG, required: false}});

        const toggle = screen.getByTestId('channelsResourceRequired-button');
        expect(toggle).not.toBeDisabled();

        await userEvent.click(toggle);

        expect(onChange).not.toHaveBeenCalled();
        expect(screen.getByText('Set channel attribute values')).toBeInTheDocument();
    });

    it('opens as an info banner and escalates to warning only after a blocked click', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 26,
            sharedCount: 0,
            uniqueAdminCount: 10,
            noAdminCount: 2,
            messagePreview: 'preview',
            required: false,
        }}));

        renderComponent({value: {...DEFAULT_CHANNEL_RESOURCE_CONFIG, required: false}, channelFieldId: 'field1'});

        const banner = screen.getByTestId('channelsMissingValuesBanner');
        expect(banner).toHaveClass('info');
        expect(banner).not.toHaveClass('warning');

        await userEvent.click(screen.getByTestId('channelsResourceRequired-button'));

        expect(banner).toHaveClass('warning');
        expect(banner).not.toHaveClass('info');
    });

    it('shows create-mode copy and hides both Notify and View channel list when there is no field id yet', () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 157,
            sharedCount: 0,
            uniqueAdminCount: 0,
            noAdminCount: 0,
            messagePreview: 'preview',
            required: false,
        }}));

        renderComponent({value: {...DEFAULT_CHANNEL_RESOURCE_CONFIG, required: false}, channelFieldId: undefined});

        expect(screen.getByText(/Save this attribute first/)).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Notify all channel admins'})).not.toBeInTheDocument();

        // Nothing can have a value for a field that doesn't exist yet, so the
        // list would just be every active channel -- not a targeted list.
        expect(screen.queryByRole('button', {name: 'View channel list'})).not.toBeInTheDocument();
    });

    it('still calls onChange to turn Required OFF while channels are missing values (the escape hatch)', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 26,
            sharedCount: 0,
            uniqueAdminCount: 10,
            noAdminCount: 2,
            messagePreview: 'preview',
            required: false,
        }}));

        const {onChange} = renderComponent({value: {...DEFAULT_CHANNEL_RESOURCE_CONFIG, required: true}});

        await userEvent.click(screen.getByTestId('channelsResourceRequired-button'));

        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({required: false}));
    });

    it('lets an unsaved Off be turned back On on an attribute whose saved state is already Required', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 1,
            sharedCount: 0,
            uniqueAdminCount: 1,
            noAdminCount: 0,
            messagePreview: 'preview',
            required: true,
        }}));

        // The admin has already flipped it Off without saving; the host
        // re-renders with the unsaved value.
        const {onChange} = renderComponent({value: {...DEFAULT_CHANNEL_RESOURCE_CONFIG, required: false}, channelFieldId: 'field1'});

        await userEvent.click(screen.getByTestId('channelsResourceRequired-button'));

        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({required: true}));
        expect(screen.getByTestId('channelsMissingValuesBanner')).not.toHaveClass('warning');
        expect(screen.getByText(/New channels must set one/)).toBeInTheDocument();
    });

    it('flips normally when nothing is missing', async () => {
        const {onChange} = renderComponent({value: {...DEFAULT_CHANNEL_RESOURCE_CONFIG, required: false}});

        await userEvent.click(screen.getByTestId('channelsResourceRequired-button'));

        expect(onChange).toHaveBeenCalledWith(expect.objectContaining({required: true}));
    });

    it('hides both Notify and View list in create mode (no channelFieldId)', () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 5,
            sharedCount: 0,
            uniqueAdminCount: 3,
            noAdminCount: 0,
            messagePreview: 'preview',
            required: false,
        }}));

        renderComponent({channelFieldId: undefined});

        expect(screen.queryByRole('button', {name: 'Notify all channel admins'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'View channel list'})).not.toBeInTheDocument();
    });

    it('shows both buttons in edit mode (channelFieldId set)', () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 5,
            sharedCount: 0,
            uniqueAdminCount: 3,
            noAdminCount: 0,
            messagePreview: 'preview',
            required: false,
        }}));

        renderComponent({channelFieldId: 'field1'});

        expect(screen.getByRole('button', {name: 'Notify all channel admins'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'View channel list'})).toBeInTheDocument();
    });

    it('opens the channel list modal with the current field id, display name and counts', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 7,
            sharedCount: 2,
            uniqueAdminCount: 3,
            noAdminCount: 0,
            messagePreview: 'preview',
            required: false,
        }}));

        renderComponent({channelFieldId: 'field1', attributeDisplayName: 'Cost center'});

        await userEvent.click(screen.getByRole('button', {name: 'View channel list'}));

        expect(openViewList).toHaveBeenCalledWith({
            fieldId: 'field1',
            attributeDisplayName: 'Cost center',
            totalCount: 9,
            sharedCount: 2,
        });
    });

    it('posts the notify request only after the confirmation modal resolves true, then reloads the summary', async () => {
        const reload = jest.fn();
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({
            summary: {totalCount: 7, sharedCount: 0, uniqueAdminCount: 3, noAdminCount: 0, messagePreview: 'preview', required: false},
            reload,
        }));
        promptNotify.mockResolvedValue(true);
        mockNotify.mockResolvedValue({queued_admin_count: 3, queued_channel_count: 7, channels_without_admin_count: 0, truncated: false});

        renderComponent({channelFieldId: 'field1', attributeDisplayName: 'Cost center'});

        await userEvent.click(screen.getByRole('button', {name: 'Notify all channel admins'}));

        expect(promptNotify).toHaveBeenCalledWith({
            totalCount: 7,
            uniqueAdminCount: 3,
            noAdminCount: 0,
            messagePreview: 'preview',
            attributeDisplayName: 'Cost center',
        });

        await waitFor(() => expect(mockNotify).toHaveBeenCalledWith('field1'));
        await waitFor(() => expect(reload).toHaveBeenCalled());
        expect(screen.getByText('Notification queued for 3 channel admins.')).toBeInTheDocument();
    });

    it('tells the admin the notify run was truncated instead of implying every admin was reached', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({
            summary: {totalCount: 30000, sharedCount: 0, uniqueAdminCount: 20000, noAdminCount: 0, messagePreview: 'preview', required: false},
        }));
        promptNotify.mockResolvedValue(true);
        mockNotify.mockResolvedValue({queued_admin_count: 20000, queued_channel_count: 20000, channels_without_admin_count: 0, truncated: true});

        renderComponent({channelFieldId: 'field1'});

        await userEvent.click(screen.getByRole('button', {name: 'Notify all channel admins'}));

        await waitFor(() => expect(screen.getByText(/more admins than one run can notify/)).toBeInTheDocument());
        expect(screen.getByText(/Use View channel list to see the channels still missing a value/)).toBeInTheDocument();
        expect(screen.queryByText(/click Notify again/i)).not.toBeInTheDocument();
        expect(screen.queryByText('Notification queued for 20000 channel admins.')).not.toBeInTheDocument();
    });

    it('does not carry a field\'s queued notification result over to a different field', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({
            summary: {totalCount: 30000, sharedCount: 0, uniqueAdminCount: 20000, noAdminCount: 0, messagePreview: 'preview', required: false},
        }));
        promptNotify.mockResolvedValue(true);
        mockNotify.mockResolvedValue({queued_admin_count: 20000, queued_channel_count: 20000, channels_without_admin_count: 0, truncated: true});

        const onChange = jest.fn();
        const {rerender} = renderWithContext(
            <ChannelsResourceSettings
                value={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChange={onChange}
                channelFieldId='field1'
            />,
            requiredEnabledState,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Notify all channel admins'}));
        await waitFor(() => expect(screen.getByText(/more admins than one run can notify/)).toBeInTheDocument());

        rerender(
            <ChannelsResourceSettings
                value={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChange={onChange}
                channelFieldId='field2'
            />,
        );

        expect(screen.queryByText(/Notification queued/)).not.toBeInTheDocument();
        expect(screen.getByText(/have a this attribute value yet/)).toBeInTheDocument();

        // Returning to the original field must not resurrect the stale result.
        rerender(
            <ChannelsResourceSettings
                value={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChange={onChange}
                channelFieldId='field1'
            />,
        );
        expect(screen.queryByText(/Notification queued/)).not.toBeInTheDocument();
    });

    it('does not act on a confirmation that resolves after the admin has switched to a different field', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({
            summary: {totalCount: 7, sharedCount: 0, uniqueAdminCount: 3, noAdminCount: 0, messagePreview: 'preview', required: false},
        }));

        let resolveConfirm: (value: boolean) => void = () => {};
        promptNotify.mockReturnValueOnce(new Promise((resolve) => {
            resolveConfirm = resolve;
        }));

        const onChange = jest.fn();
        const {rerender} = renderWithContext(
            <ChannelsResourceSettings
                value={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChange={onChange}
                channelFieldId='field1'
            />,
            requiredEnabledState,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Notify all channel admins'}));
        expect(promptNotify).toHaveBeenCalledTimes(1);

        // The admin switches to a different attribute's Channels row while
        // the confirmation modal for field1 is still open.
        rerender(
            <ChannelsResourceSettings
                value={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChange={onChange}
                channelFieldId='field2'
            />,
        );

        resolveConfirm(true);
        await new Promise((resolve) => setTimeout(resolve, 0));

        // field1's confirmation resolving must not POST for field1, nor mark
        // field2's (now-current) banner as "notifying".
        expect(mockNotify).not.toHaveBeenCalled();
        expect(screen.queryByText('Notification queued for 3 channel admins.')).not.toBeInTheDocument();
    });

    it('does not POST when the confirmation is cancelled', async () => {
        mockUseChannelMissingValues.mockReturnValue(missingValuesState({summary: {
            totalCount: 7, sharedCount: 0, uniqueAdminCount: 3, noAdminCount: 0, messagePreview: 'preview', required: false,
        }}));
        promptNotify.mockResolvedValue(false);

        renderComponent({channelFieldId: 'field1'});

        await userEvent.click(screen.getByRole('button', {name: 'Notify all channel admins'}));

        expect(mockNotify).not.toHaveBeenCalled();
    });
});
