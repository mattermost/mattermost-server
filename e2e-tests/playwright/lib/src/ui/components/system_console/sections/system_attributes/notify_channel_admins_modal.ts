// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Locator} from '@playwright/test';
import {expect} from '@playwright/test';

import BaseModal from '@/ui/components/system_console/base_modal';

/**
 * The "Notify all channel admins?" confirmation modal (MM-70717), opened from
 * the Required toggle's missing-values banner. Dumb by design: it only
 * resolves the confirmation, the POST and the resulting banner state are
 * owned by the page behind it (see channels_resource_settings.tsx).
 */
export default class NotifyChannelAdminsModal extends BaseModal {
    readonly confirmButton: Locator;

    constructor(container: Locator) {
        super(container);
        this.confirmButton = container.getByRole('button', {name: 'Send notification'});
    }

    get messagePreview() {
        return this.container.locator('blockquote');
    }

    async confirm() {
        await this.confirmButton.click();
        await expect(this.container).not.toBeVisible();
    }
}
