// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import AttributeAppliesTo from './attribute_applies_to';
import type {ResourceObjectType} from './attribute_applies_to_constants';

import {DEFAULT_CHANNEL_RESOURCE_CONFIG} from '../applies_to/channels';
import useChannelMissingValues from '../applies_to/channels/use_channel_missing_values';

// Isolates this suite from the MM-70717 missing-values fetch -- covered by
// use_channel_missing_values.test.ts and channels_resource_settings.test.tsx.
jest.mock('../applies_to/channels/use_channel_missing_values');

const mockUseChannelMissingValues = jest.mocked(useChannelMissingValues);

describe('AttributeAppliesTo', () => {
    const onAdd = jest.fn();
    const onRemove = jest.fn();

    beforeEach(() => {
        jest.clearAllMocks();
        mockUseChannelMissingValues.mockReturnValue({loading: false, failed: false, summary: null, reload: jest.fn()});
    });

    const renderComponent = (props: Partial<React.ComponentProps<typeof AttributeAppliesTo>> = {}) => {
        return renderWithContext(
            <AttributeAppliesTo
                appliesTo={[]}
                onAdd={onAdd}
                onRemove={onRemove}
                channelResource={DEFAULT_CHANNEL_RESOURCE_CONFIG}
                onChannelResourceChange={jest.fn()}
                {...props}
            />,
        );
    };

    it('renders as a Compass AdminPanel with title and subtitle', () => {
        const {container} = renderComponent();

        expect(container.querySelector('#attribute_applies_to.AdminPanel')).toBeInTheDocument();
        expect(screen.getByRole('heading', {name: 'Applies to'})).toBeInTheDocument();
        expect(screen.getByText('Resources this attribute applies to, and who can set the value on each.')).toBeInTheDocument();
    });

    it('renders the empty state when appliesTo is empty', () => {
        renderComponent();
        expect(screen.getByTestId('attributeAppliesToEmptyState')).toBeInTheDocument();
        expect(screen.queryByTestId(/attributeAppliesToRow-/)).not.toBeInTheDocument();
    });

    it('shows both Add-resource triggers when the empty state is showing', () => {
        renderComponent();
        const header = screen.getByTestId('attributeAppliesToAddResourceButtonHeader');
        const inline = screen.getByTestId('attributeAppliesToAddResourceButtonInline');
        expect(header).toBeVisible();
        expect(inline).toBeVisible();
        expect(header).toHaveAccessibleName('Add resource');
        expect(inline).toHaveAccessibleName('Add resource');
        expect(header).toHaveClass('btn-tertiary');
        expect(inline).toHaveClass('btn-primary');
    });

    it('offers exactly the not-yet-selected types, in Users -> Channels -> Posts order', async () => {
        renderComponent({appliesTo: ['channel']});

        expect(screen.getByTestId('attributeAppliesToAddResourceButtonHeader')).toHaveClass('btn-tertiary');
        expect(screen.getByTestId('attributeAppliesToAddResourceButtonHeader')).toHaveAccessibleName('Add resource');
        expect(screen.getByTestId('attributeAppliesToAddResourceButtonInline')).toHaveClass('btn-tertiary');
        expect(screen.getByTestId('attributeAppliesToAddResourceButtonInline')).toHaveAccessibleName('Add another resource');

        await userEvent.click(screen.getByTestId('attributeAppliesToAddResourceButtonHeader'));
        const items = screen.getAllByRole('menuitem');
        expect(items.map((item) => item.textContent)).toEqual(['Users', 'Posts']);
    });

    it('calls onAdd with the correct type when a menu item is selected', async () => {
        renderComponent();

        await userEvent.click(screen.getByTestId('attributeAppliesToAddResourceButtonHeader'));
        await userEvent.click(screen.getByRole('menuitem', {name: 'Channels'}));

        // A non-checkbox/radio Menu.Item defers onClick until after the
        // menu's close transition completes (menu_item.tsx's addOnClosedListener).
        await waitFor(() => expect(onAdd).toHaveBeenCalledWith('channel'));
    });

    it('renders one row per entry in appliesTo, in insertion order', () => {
        renderComponent({appliesTo: ['post', 'user']});

        const rows = screen.getAllByTestId(/^attributeAppliesToRow-(user|channel|post)$/);
        expect(rows.map((row) => row.getAttribute('data-testid'))).toEqual(['attributeAppliesToRow-post', 'attributeAppliesToRow-user']);
    });

    it('hides both Add-resource triggers entirely once all three types are present', () => {
        renderComponent({appliesTo: ['user', 'channel', 'post'] as ResourceObjectType[]});

        expect(screen.queryByTestId('attributeAppliesToAddResourceButtonHeader')).not.toBeInTheDocument();
        expect(screen.queryByTestId('attributeAppliesToAddResourceButtonInline')).not.toBeInTheDocument();
    });

    it('calls onRemove with each row\'s own type, once expanded', async () => {
        renderComponent({appliesTo: ['user', 'channel']});

        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-remove'));
        expect(onRemove).toHaveBeenCalledWith('channel');

        await userEvent.click(screen.getByTestId('attributeAppliesToRow-user-toggle'));
        await userEvent.click(screen.getByTestId('attributeAppliesToRow-user-remove'));
        expect(onRemove).toHaveBeenCalledWith('user');
    });

    describe('hideAddResource', () => {
        it('hides both Add-resource triggers even when types are still available', () => {
            renderComponent({hideAddResource: true});

            expect(screen.queryByTestId('attributeAppliesToAddResourceButtonHeader')).not.toBeInTheDocument();
            expect(screen.queryByTestId('attributeAppliesToAddResourceButtonInline')).not.toBeInTheDocument();
        });

        it('hides the inline trigger next to already-applied rows too, not just the empty state', () => {
            renderComponent({appliesTo: ['user'], hideAddResource: true});

            expect(screen.queryByTestId('attributeAppliesToAddResourceButtonInline')).not.toBeInTheDocument();
            expect(screen.getByTestId('attributeAppliesToRow-user')).toBeInTheDocument();
        });

        it('shows plugin-owned empty-state copy instead of the normal Add-a-resource copy', () => {
            renderComponent({hideAddResource: true});

            expect(screen.getByTestId('attributeAppliesToEmptyState')).toHaveTextContent("Resources for plugin-managed attributes can't be added here.");
            expect(screen.getByTestId('attributeAppliesToEmptyState')).not.toHaveTextContent('Add a resource to apply this attribute');
        });

        it('preserves all existing behavior when hideAddResource is false (default)', () => {
            renderComponent();

            expect(screen.getByTestId('attributeAppliesToAddResourceButtonHeader')).toBeInTheDocument();
            expect(screen.getByTestId('attributeAppliesToAddResourceButtonInline')).toBeInTheDocument();
            expect(screen.getByTestId('attributeAppliesToEmptyState')).toHaveTextContent('Add a resource to apply this attribute');
        });
    });

    describe('userWhoCanSetLockedTooltip on the Users row', () => {
        it('locks Who can set the value once the Users row is expanded', async () => {
            renderComponent({appliesTo: ['user'], userWhoCanSetLockedTooltip: 'Managed by SCIM'});
            await userEvent.click(screen.getByTestId('attributeAppliesToRow-user-toggle'));

            expect(screen.getByTestId('attributeAppliesToUserWhoCanSet-lockWrap')).toBeInTheDocument();
        });
    });

    describe('blockedTypes', () => {
        it('offers a blocked type as a disabled option carrying its reason, and adds nothing when it is clicked', async () => {
            renderComponent({blockedTypes: {user: 'Name already in use'}});

            await userEvent.click(screen.getByTestId('attributeAppliesToAddResourceButtonHeader'));

            const users = screen.getByRole('menuitem', {name: /^Users/});
            expect(users).toHaveAttribute('aria-disabled', 'true');
            expect(users).toHaveTextContent('Name already in use');

            // pointerEventsCheck off so userEvent dispatches the click rather than
            // refusing it over the item's pointer-events: none. The component
            // passes onAdd to every option unconditionally and relies on MUI's
            // `disabled` to swallow the click, so what follows is a statement
            // about the outcome only — choosing a blocked option adds nothing —
            // and not about any guard of this component's own.
            await userEvent.click(users, {pointerEventsCheck: 0});

            // A live Menu.Item defers its onClick until the menu has finished
            // closing, so close the menu before concluding nothing was added.
            await userEvent.keyboard('{Escape}');
            await waitFor(() => expect(screen.queryByRole('menuitem')).not.toBeInTheDocument());
            expect(onAdd).not.toHaveBeenCalled();
        });

        it('leaves a type with no entry enabled, and still adds it', async () => {
            renderComponent({blockedTypes: {user: 'Name already in use'}});

            await userEvent.click(screen.getByTestId('attributeAppliesToAddResourceButtonHeader'));

            const channels = screen.getByRole('menuitem', {name: 'Channels'});
            expect(channels).not.toHaveAttribute('aria-disabled', 'true');
            expect(channels).not.toHaveTextContent('Name already in use');

            await userEvent.click(channels);

            await waitFor(() => expect(onAdd).toHaveBeenCalledWith('channel'));
        });
    });

    describe('lockedTooltip on the Channels row', () => {
        it('wraps the toggle in the lock tooltip when lockedTooltip is given', () => {
            renderComponent({appliesTo: ['channel'], lockedTooltip: 'Locked'});

            expect(screen.getByTestId('attributeAppliesToRow-channel-toggleLockWrap')).toBeInTheDocument();
        });

        it('renders no lock wrapper when lockedTooltip is not given', () => {
            renderComponent({appliesTo: ['channel']});

            expect(screen.queryByTestId('attributeAppliesToRow-channel-toggleLockWrap')).not.toBeInTheDocument();
        });
    });

    describe('removeLockedTooltips', () => {
        it('locks Remove only on the row whose type has a tooltip', async () => {
            renderComponent({appliesTo: ['user', 'channel'], removeLockedTooltips: {user: 'Locked'}});

            await userEvent.click(screen.getByTestId('attributeAppliesToRow-user-toggle'));
            expect(screen.getByTestId('attributeAppliesToRow-user-removeLockWrap')).toBeInTheDocument();

            await userEvent.click(screen.getByTestId('attributeAppliesToRow-channel-toggle'));
            expect(screen.getByTestId('attributeAppliesToRow-channel-remove')).toBeEnabled();
        });
    });
});
