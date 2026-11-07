// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Page} from '@playwright/test';
import {expect} from '@playwright/test';
import {waitUntil} from 'async-wait-until';

import type {
    ChannelNotificationPreferencesModal,
    ChannelsPost,
    SettingsModal,
    TeamSettingsModal,
    InvitePeopleModal,
    MembersInvitedModal,
} from '@/ui/components';
import {
    BrowseChannelsModal,
    ChannelsCenterView,
    ChannelSettingsModal,
    CreateTeamForm,
    NewChannelModal,
    components,
    ChannelsSidebarLeft,
} from '@/ui/components';
import {duration} from '@/util';

export default class ChannelsPage {
    readonly channels = 'Channels';

    readonly page: Page;

    readonly globalHeader;
    readonly mobileNavbar;
    readonly userAccountMenuButton;
    readonly searchBox;
    readonly centerView;
    readonly sidebarLeft;
    readonly sidebarRight;
    readonly appBar;
    readonly userProfilePopover;
    readonly messagePriority;

    readonly channelSettingsModal;
    readonly channelNotificationPreferencesModal;
    readonly editChannelHeaderModal;
    readonly createTeamForm;
    readonly deletePostModal;
    readonly findChannelsModal;
    readonly newChannelModal;
    readonly browseChannelsModal;
    readonly directChannelsModal;
    readonly keyboardShortcutsModal;
    readonly imagePreviewModal;
    public invitePeopleModal: InvitePeopleModal | undefined;
    public membersInvitedModal: MembersInvitedModal | undefined;
    readonly profileModal;
    readonly settingsModal;
    readonly teamSettingsModal;
    readonly scheduledDraftModal;
    readonly scheduleMessageModal;
    readonly burnOnReadConfirmationModal;
    readonly searchResultsPanel;
    readonly marketplaceModal;
    readonly channelBookmarksBar;
    readonly bookmarkCreateModal;
    readonly userGroupsModal;
    readonly leaveTeamModal;
    readonly archivedChannelMessage;
    readonly unarchiveChannelModal;
    readonly switchProductMenu;

    readonly postContainer;
    readonly channelMenu;
    readonly postDotMenu;
    readonly postReminderMenu;
    readonly userAccountMenu;
    readonly teamMenu;

    readonly emojiGifPickerPopup;
    readonly reactionEmojiPicker;
    readonly scheduleMessageMenu;

    readonly searchResultsContainer;
    readonly searchResultItems;

    constructor(page: Page) {
        this.page = page;

        // The main areas of the app
        this.globalHeader = new components.GlobalHeader(this, page.locator('#global-header'));
        this.mobileNavbar = new components.ChannelsMobileNavbar(page.locator('#navbar'));
        this.searchBox = new components.SearchBox(page.locator('#searchBox'));
        this.centerView = new ChannelsCenterView(page.getByTestId('channel_view'), page);
        this.sidebarLeft = new ChannelsSidebarLeft(page.locator('#SidebarContainer'));
        this.sidebarRight = new components.ChannelsSidebarRight(page.locator('#sidebar-right'));
        this.appBar = new components.ChannelsAppBar(page.getByTestId('app-bar'));
        this.messagePriority = new components.MessagePriority(page.locator('body'));
        this.userAccountMenuButton = page.getByRole('button', {name: "User's account menu"});

        // Modals
        this.channelSettingsModal = new ChannelSettingsModal(page.getByRole('dialog', {name: 'Channel Settings'}));
        this.channelNotificationPreferencesModal = new components.ChannelNotificationPreferencesModal(
            page.getByRole('dialog', {name: 'Notification Preferences'}),
        );
        this.editChannelHeaderModal = new components.EditChannelHeaderModal(
            page.getByRole('dialog', {name: /Edit Header/}),
        );
        this.keyboardShortcutsModal = page.getByRole('dialog', {name: /Keyboard shortcuts/});
        this.imagePreviewModal = new components.ImagePreviewModal(
            page.getByRole('dialog').filter({has: page.getByAltText('preview url image')}),
        );
        this.createTeamForm = new CreateTeamForm(page.getByTestId('create-team-form'));
        this.deletePostModal = new components.DeletePostModal(page.locator('#deletePostModal'));
        this.findChannelsModal = new components.FindChannelsModal(page.getByRole('dialog', {name: 'Find Channels'}));
        this.newChannelModal = new NewChannelModal(page.getByRole('dialog', {name: 'Create a new channel'}));
        this.browseChannelsModal = new BrowseChannelsModal(page.getByRole('dialog', {name: 'Browse Channels'}));
        this.directChannelsModal = new components.DirectChannelsModal(
            page.getByRole('dialog', {name: 'Direct Messages'}),
        );
        this.profileModal = new components.ProfileModal(page.getByRole('dialog', {name: 'Profile'}));
        this.settingsModal = new components.SettingsModal(page.getByRole('dialog', {name: 'Settings'}));
        this.teamSettingsModal = new components.TeamSettingsModal(page.getByRole('dialog', {name: 'Team Settings'}));
        this.burnOnReadConfirmationModal = new components.BurnOnReadConfirmationModal(
            page.getByRole('dialog').filter({hasText: /burn|delete/i}),
        );
        this.searchResultsPanel = new components.SearchResultsPanel(page.locator('#searchContainer'));
        this.marketplaceModal = new components.MarketplaceModal(page.getByRole('dialog', {name: 'App Marketplace'}));
        this.channelBookmarksBar = new components.ChannelBookmarksBar(page.getByTestId('channel-bookmarks-container'));
        this.bookmarkCreateModal = new components.ChannelBookmarksCreateModal(
            page.getByRole('dialog', {name: 'Add a bookmark'}),
        );
        this.userGroupsModal = new components.UserGroupsModal(page.locator('#userGroupsModal'));
        this.leaveTeamModal = new components.LeaveTeamModal(page.getByRole('dialog', {name: 'Leave the team?'}));

        // Menus
        // The channel header dropdown menu's accessible name is "<channel> Channel Menu".
        this.channelMenu = new components.ChannelMenu(page.getByRole('menu', {name: /Channel Menu/i}));
        this.postDotMenu = new components.PostDotMenu(page.getByRole('menu', {name: 'Post extra options'}));
        this.postReminderMenu = new components.PostReminderMenu(page.getByRole('menu', {name: 'Set a reminder for:'}));
        this.userAccountMenu = new components.UserAccountMenu(page.locator('#userAccountMenu'));
        this.scheduleMessageMenu = new components.ScheduleMessageMenu(page.locator('#dropdown_send_post_options'));
        this.teamMenu = new components.TeamMenu(page.locator('#sidebarTeamMenu'));
        this.switchProductMenu = new components.SwitchProductMenu(page.locator('#switchProductMenu'));

        // Popovers
        this.emojiGifPickerPopup = new components.EmojiGifPicker(page.locator('#emojiGifPicker'));
        this.reactionEmojiPicker = new components.EmojiGifPicker(page.getByRole('dialog', {name: 'Emoji Picker'}));
        this.scheduledDraftModal = new components.ScheduledDraftModal(page.getByRole('dialog', {name: /scheduled/i}));
        this.scheduleMessageModal = new components.ScheduleMessageModal(
            page.getByRole('dialog', {name: 'Schedule message'}),
        );
        this.userProfilePopover = new components.UserProfilePopover(page.getByTestId('user-profile-popover'));

        // Posts
        this.postContainer = page.getByTestId('post-message-text');
        this.archivedChannelMessage = page.locator('#channelArchivedMessage');
        this.unarchiveChannelModal = new components.UnarchiveChannelModal(page.locator('#unarchiveChannelModal'));

        // Search results
        this.searchResultsContainer = page.locator('#search-items-container');
        this.searchResultItems = page.getByTestId('search-item-container');
    }

    /**
     * Locates a search result item containing the given text.
     * @param text
     */
    getSearchResultItem(text: string) {
        return this.page.getByTestId('search-item-container').filter({hasText: text});
    }

    async toBeVisible() {
        await this.centerView.toBeVisible();
    }

    /**
     * `toNotContainText` verifies if the page does not contain the specified text.
     * @param text Text to be verified not in the page
     */
    async toNotContainText(text: string) {
        await expect(this.page.locator('body')).not.toContainText(text);
    }

    async getLastPost() {
        return this.centerView.getLastPost();
    }

    async getInvitePeopleModal(teamDisplayName: string) {
        this.invitePeopleModal = new components.InvitePeopleModal(
            this.page.getByRole('dialog', {name: `Invite people to ${teamDisplayName}`}),
        );
        return this.invitePeopleModal;
    }

    getAddPeopleToChannelModal() {
        return new components.AddPeopleToChannelModal(this.page.getByRole('dialog', {name: /Add people to/}));
    }

    getViewUserGroupModal(groupDisplayName: string) {
        return new components.ViewUserGroupModal(this.page.getByRole('dialog', {name: groupDisplayName, exact: true}));
    }

    getBookmarkEditModal() {
        return new components.ChannelBookmarksCreateModal(this.page.getByRole('dialog', {name: 'Edit bookmark'}));
    }

    async getMembersInvitedModal(teamDisplayName: string) {
        this.membersInvitedModal = new components.MembersInvitedModal(
            this.page.getByRole('dialog', {name: `invited to ${teamDisplayName}`}),
        );
        return this.membersInvitedModal;
    }

    async goto(teamName = '', channelName = '') {
        let channelsUrl = '/';
        if (teamName) {
            channelsUrl += `${teamName}`;
            if (channelName) {
                const prefix = channelName.startsWith('@') ? '/messages' : '/channels';
                channelsUrl += `${prefix}/${channelName}`;
            }
        }
        await this.page.goto(channelsUrl);

        return channelsUrl;
    }

    async expectOnTeamChannel(teamName: string, channelName = 'town-square') {
        const escapedTeam = teamName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const escapedChannel = channelName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        await expect(this.page).toHaveURL(new RegExp(`/${escapedTeam}/channels/${escapedChannel}(?:/|\\?|#|$)`));
        await this.toBeVisible();
    }

    // Force the /messages route for group-message slugs that do not start with '@'.
    async gotoMessage(teamName: string, channelName: string) {
        const channelsUrl = `/${teamName}/messages/${channelName}`;
        await this.page.goto(channelsUrl);

        return channelsUrl;
    }

    /**
     * `postMessage` posts a message in the current channel
     * @param message Message to post
     * @param files Files to attach to the message
     */
    async postMessage(message: string, files?: string[]) {
        await this.centerView.postMessage(message, files);
    }

    async replyToLastPost(message: string) {
        const rootPost = await this.getLastPost();
        await rootPost.reply();

        const sidebarRight = this.sidebarRight;
        await sidebarRight.toBeVisible();
        await sidebarRight.postMessage('Replying to a thread');

        // * Verify the message has been sent
        await waitUntil(
            async () => {
                const post = await this.sidebarRight.getLastPost();
                const content = await post.container.textContent();

                return content?.includes(message);
            },
            {timeout: duration.ten_sec},
        );

        const lastPost = await sidebarRight.getLastPost();

        return {rootPost, sidebarRight, lastPost};
    }

    async openTeamSettings(): Promise<TeamSettingsModal> {
        await this.page.locator('#sidebarTeamMenuButton').click();
        await this.page.getByText('Team settings').first().click();
        await this.teamSettingsModal.toBeVisible();

        return this.teamSettingsModal;
    }

    /**
     * Returns a confirm-modal page object scoped to the dialog with the given title (its accessible name).
     * @param title The modal's title text.
     * @param confirmLabel The confirm button label (defaults to "Confirm").
     */
    getConfirmModal(title: string, confirmLabel = 'Confirm') {
        return new components.GenericConfirmModal(this.page.getByRole('dialog', {name: title}), confirmLabel);
    }

    /**
     * Opens the channel header dropdown menu and returns it.
     */
    async openChannelMenu() {
        await this.centerView.header.openChannelMenu();
        await this.channelMenu.toBeVisible();

        return this.channelMenu;
    }

    /**
     * Archives the current channel via the channel header menu and confirms the
     * archive dialog. Waits for the archived-channel footer to appear.
     */
    async archiveChannel() {
        const channelMenu = await this.openChannelMenu();
        await channelMenu.archiveToggle.click();
        await this.page.getByRole('button', {name: 'Archive', exact: true}).click();
        await expect(this.archivedChannelMessage).toBeVisible();
    }

    /**
     * Opens the Unarchive confirmation from the channel header menu. The menu
     * item's accessible name flips between "Archive Channel" and "Unarchive
     * Channel" depending on the channel's current state -- this only makes
     * sense to call while viewing an already-archived channel.
     */
    async openUnarchiveChannelModal() {
        const channelMenu = await this.openChannelMenu();
        await channelMenu.archiveToggle.click();
        await this.unarchiveChannelModal.toBeVisible();

        return this.unarchiveChannelModal;
    }

    /**
     * Opens the search UI, runs a search for the given term, and waits for the
     * results panel to appear.
     */
    async searchFor(term: string) {
        await this.globalHeader.openSearch();
        await this.searchBox.toBeVisible();
        await this.searchBox.search(term);
        await this.searchResultsPanel.toBeVisible();
    }

    async openChannelSettings(): Promise<ChannelSettingsModal> {
        const channelMenu = await this.openChannelMenu();

        const channelSettingsMenuItem = channelMenu.channelSettings;
        const moreActionsMenuItem = channelMenu.item(/More actions/i);

        const channelSettingsVisible = await channelSettingsMenuItem.isVisible({timeout: 1500}).catch(() => false);
        if (!channelSettingsVisible) {
            const moreActionsVisible = await moreActionsMenuItem.isVisible({timeout: 1500}).catch(() => false);
            if (moreActionsVisible) {
                await moreActionsMenuItem.click();
            }
        }

        await expect(channelSettingsMenuItem).toBeVisible();
        await channelSettingsMenuItem.click();
        await this.channelSettingsModal.toBeVisible();

        return this.channelSettingsModal;
    }

    /**
     * Opens the Channel Info panel in the RHS, toggling it open if a different
     * panel is already showing there.
     */
    async openChannelInfo() {
        const infoButton = this.page.locator('#channel-info-btn');
        await expect(infoButton).toBeVisible();

        // The button toggles, so clicking it while Info is already showing would close it.
        const infoHeading = this.sidebarRight.container.getByText('Info', {exact: true});
        const alreadyOpen = await infoHeading.isVisible().catch(() => false);
        if (!alreadyOpen) {
            await infoButton.click();
            await expect(infoHeading).toBeVisible();
        }

        await this.sidebarRight.toBeVisible();

        return this.sidebarRight;
    }

    async openChannelNotificationPreferences(): Promise<ChannelNotificationPreferencesModal> {
        const channelMenu = await this.openChannelMenu();
        await channelMenu.notificationPreferences.click();
        await this.channelNotificationPreferencesModal.toBeVisible();

        return this.channelNotificationPreferencesModal;
    }

    async closeGroupMessage() {
        const channelMenu = await this.openChannelMenu();
        await channelMenu.closeConversation.click();
    }

    async openSettings(): Promise<SettingsModal> {
        await this.globalHeader.openSettings();
        await this.settingsModal.toBeVisible();
        return this.settingsModal;
    }

    async openNewChannelModal(): Promise<NewChannelModal> {
        await this.sidebarLeft.browseOrCreateChannelButton.click();
        await this.page.getByText('Create new channel').click();
        await this.newChannelModal.toBeVisible();

        return this.newChannelModal;
    }

    async openBrowseChannelsModal(): Promise<BrowseChannelsModal> {
        await this.sidebarLeft.browseOrCreateChannelButton.click();
        await this.page.getByText('Browse channels').click();
        await this.browseChannelsModal.toBeVisible();

        return this.browseChannelsModal;
    }

    async openDirectChannelsModal() {
        await this.sidebarLeft.openDirectMessageButton.click();
        await this.directChannelsModal.toBeVisible();

        return this.directChannelsModal;
    }

    async openCreateTeamForm(): Promise<CreateTeamForm> {
        await this.sidebarLeft.teamMenuButton.click();
        await this.teamMenu.toBeVisible();
        await this.teamMenu.clickCreateTeam();
        await this.createTeamForm.toBeVisible();

        return this.createTeamForm;
    }

    async newChannel(name: string, channelType: string) {
        const newChannelModal = await this.openNewChannelModal();
        await newChannelModal.displayNameInput.fill(name);

        if (channelType === 'P') {
            await newChannelModal.privateTypeButton.click();
        } else {
            await newChannelModal.publicTypeButton.click();
        }

        await newChannelModal.create();
    }

    async openUserAccountMenu() {
        await this.userAccountMenuButton.click();
        await expect(this.userAccountMenu.container).toBeVisible();
        return this.userAccountMenu;
    }

    /**
     * Switches to another team via its button in the team sidebar.
     */
    async switchToTeam(teamName: string) {
        const teamButton = this.page.locator(`#${teamName}TeamButton`);
        await teamButton.waitFor();
        await teamButton.click();
    }

    getTeamButton(teamDisplayName: string) {
        const escapedName = teamDisplayName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        return this.page.getByRole('button', {name: new RegExp(`^${escapedName} team`, 'i')});
    }

    async switchToTeamByDisplayName(teamDisplayName: string) {
        await this.getTeamButton(teamDisplayName).click();
    }

    async toHaveTeamMentionCount(teamDisplayName: string, count: number) {
        await expect(this.getTeamButton(teamDisplayName)).toHaveAccessibleName(
            `${teamDisplayName.toLowerCase()} team, ${count} mentions`,
        );
    }

    async toHaveTeamNoUnread(teamDisplayName: string) {
        await expect(this.getTeamButton(teamDisplayName)).toHaveAccessibleName(`${teamDisplayName.toLowerCase()} team`);
    }

    async toHaveTeamUnread(teamDisplayName: string, teamId: string) {
        const teamButton = this.getTeamButton(teamDisplayName);
        await expect(teamButton).toHaveAccessibleName(`${teamDisplayName.toLowerCase()} team unread`);
        await expect(teamButton.getByTestId(`team-badge-${teamId}`)).toBeVisible();
        await expect(teamButton.getByTestId(`team-badge-${teamId}`)).toHaveText('');
    }

    /**
     * Switches to the given team and leaves it via the team menu, confirming the modal.
     */
    async leaveTeam(teamName: string) {
        await this.switchToTeam(teamName);
        await this.sidebarLeft.teamMenuButton.click();
        await this.teamMenu.clickLeaveTeam();
        await this.leaveTeamModal.confirm();
    }

    /**
     * Logs the current user out via the user account menu.
     */
    async logout() {
        const menu = await this.openUserAccountMenu();
        await menu.logout.click();
    }

    async openProfileModal() {
        await this.openUserAccountMenu();
        await this.userAccountMenu.profile.click();
        await expect(this.profileModal.container).toBeVisible();
        return this.profileModal;
    }

    async openProfilePopover(post: ChannelsPost) {
        // Find and click the post's user avatar to open the profile popover
        await post.hover();
        await post.profileIcon.click();

        // Wait for the profile popover to be visible
        const popover = this.userProfilePopover;
        await expect(popover.container).toBeVisible();

        return popover;
    }

    async openScheduleMessageModal() {
        await expect(this.centerView.postCreate.scheduleMessageButton).toBeVisible();
        await this.centerView.postCreate.scheduleMessageButton.click();

        await this.scheduleMessageMenu.toBeVisible();
        await this.scheduleMessageMenu.selectCustomTime();

        await this.scheduleMessageModal.toBeVisible();

        return this.scheduleMessageModal;
    }

    async scheduleMessage(
        message: string,
        dayFromToday: number = 0,
        timeOptionIndex: number = 0,
        repeatWeekly?: boolean,
    ) {
        await this.centerView.postCreate.writeMessage(message);

        const scheduleMessageModal = await this.openScheduleMessageModal();

        return scheduleMessageModal.scheduleMessage(dayFromToday, timeOptionIndex, repeatWeekly);
    }

    async scheduleMessageFromThread(
        message: string,
        dayFromToday: number = 0,
        timeOptionIndex: number = 0,
        repeatWeekly?: boolean,
    ) {
        await this.sidebarRight.postCreate.writeMessage(message);

        await expect(this.sidebarRight.postCreate.scheduleMessageButton).toBeVisible();
        await this.sidebarRight.postCreate.scheduleMessageButton.click();

        await this.scheduleMessageMenu.toBeVisible();
        await this.scheduleMessageMenu.selectCustomTime();

        return this.scheduleMessageModal.scheduleMessage(dayFromToday, timeOptionIndex, repeatWeekly);
    }

    async getFlaggedPostViewDetailButton(flaggedPostId: string) {
        return this.page.getByTestId(`data-spillage-action-view-details_${flaggedPostId}`);
    }
}
