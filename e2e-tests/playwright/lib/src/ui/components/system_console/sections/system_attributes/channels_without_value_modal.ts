// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect} from '@playwright/test';

import BaseModal from '@/ui/components/system_console/base_modal';

/**
 * The "Channels without a value" modal (MM-70717), opened from the Required
 * toggle's missing-values banner. Its footer holds Previous/Next and Close;
 * the header's X is also named "Close", so the footer button is found by id.
 */
export default class ChannelsWithoutValueModal extends BaseModal {
    get footerCloseButton() {
        return this.container.getByTestId('channelsWithoutValueModalClose');
    }

    async close() {
        await this.footerCloseButton.click();
        await expect(this.container).not.toBeVisible();
    }

    get rows() {
        return this.container.locator('tbody tr');
    }

    /**
     * @param channelDisplayName exact display name of the channel row to find.
     */
    row(channelDisplayName: string) {
        return this.container.locator('tr', {hasText: channelDisplayName});
    }

    get nextButton() {
        return this.container.getByRole('button', {name: 'Next'});
    }

    get previousButton() {
        return this.container.getByRole('button', {name: 'Previous'});
    }

    /**
     * The Local/Shared/All filter tabs. Only rendered at all once at least one
     * shared (remote) channel is present in the list.
     */
    filterTab(label: 'All' | 'Local' | 'Shared') {
        return this.container.getByRole('button', {name: new RegExp(`^${label} \\(\\d+\\)$`)});
    }

    get emptyState() {
        return this.container.getByText('Every channel has a value');
    }
}
