// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import NotifyChannelAdminsModal from './notify_channel_admins_modal';

describe('NotifyChannelAdminsModal', () => {
    const onConfirm = jest.fn();
    const onCancel = jest.fn();
    const onExited = jest.fn();

    beforeEach(() => {
        jest.clearAllMocks();
    });

    it('shows the no-admin bullet only when noAdminCount > 0', () => {
        const {rerender} = renderWithContext(
            <NotifyChannelAdminsModal
                totalCount={26}
                uniqueAdminCount={10}
                noAdminCount={2}
                messagePreview='Please set a value for Cost center.'
                attributeDisplayName='Cost center'
                onConfirm={onConfirm}
                onCancel={onCancel}
                onExited={onExited}
            />,
        );

        expect(screen.getByText(/no channel admin and won't be notified/)).toBeInTheDocument();

        rerender(
            <NotifyChannelAdminsModal
                totalCount={24}
                uniqueAdminCount={10}
                noAdminCount={0}
                messagePreview='Please set a value for Cost center.'
                attributeDisplayName='Cost center'
                onConfirm={onConfirm}
                onCancel={onCancel}
                onExited={onExited}
            />,
        );

        expect(screen.queryByText(/no channel admin and won't be notified/)).not.toBeInTheDocument();
    });

    it('renders the server-supplied message preview verbatim', () => {
        renderWithContext(
            <NotifyChannelAdminsModal
                totalCount={26}
                uniqueAdminCount={10}
                noAdminCount={2}
                messagePreview='Please set a value for Cost center on your channels.'
                attributeDisplayName='Cost center'
                onConfirm={onConfirm}
                onCancel={onCancel}
                onExited={onExited}
            />,
        );

        expect(screen.getByText('Please set a value for Cost center on your channels.')).toBeInTheDocument();
    });

    it('calls onConfirm when Send notification is clicked', async () => {
        renderWithContext(
            <NotifyChannelAdminsModal
                totalCount={26}
                uniqueAdminCount={10}
                noAdminCount={0}
                messagePreview='preview'
                onConfirm={onConfirm}
                onCancel={onCancel}
                onExited={onExited}
            />,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Send notification'}));

        expect(onConfirm).toHaveBeenCalled();
    });

    it('calls onCancel when Cancel is clicked', async () => {
        renderWithContext(
            <NotifyChannelAdminsModal
                totalCount={26}
                uniqueAdminCount={10}
                noAdminCount={0}
                messagePreview='preview'
                onConfirm={onConfirm}
                onCancel={onCancel}
                onExited={onExited}
            />,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));

        expect(onCancel).toHaveBeenCalled();
        expect(onConfirm).not.toHaveBeenCalled();
    });
});
