// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Locator} from '@playwright/test';
import {expect} from '@playwright/test';

/**
 * The Unarchive Channel confirmation modal. Warn-only for a required
 * channel attribute missing its value (MM-70717): the warning banner never
 * blocks the restore, it only relabels the confirm button "Unarchive anyway".
 */
export default class UnarchiveChannelModal {
    readonly container: Locator;
    readonly missingAttributesBanner: Locator;
    readonly cancelButton: Locator;
    readonly confirmButton: Locator;

    constructor(container: Locator) {
        this.container = container;
        this.missingAttributesBanner = container.getByTestId('unarchiveChannelModalMissingAttributes');
        this.cancelButton = container.getByRole('button', {name: 'Cancel', exact: true});
        this.confirmButton = container.getByRole('button', {name: /^Unarchive/});
    }

    async toBeVisible() {
        await expect(this.container).toBeVisible();
    }

    async confirm() {
        await this.confirmButton.click();
        await expect(this.container).not.toBeVisible();
    }

    async cancel() {
        await this.cancelButton.click();
        await expect(this.container).not.toBeVisible();
    }
}
