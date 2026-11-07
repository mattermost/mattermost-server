// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Shared / Global Components
import AboutBuildModal from './about_build_modal';
import Footer from './footer';
import GlobalHeader from './global_header';
import MainHeader from './main_header';
import UserAccountMenu from './user_account_menu';
// Channels Components
import BrowseChannelsModal from './channels/browse_channels_modal';
import ChannelsAppBar from './channels/app_bar';
import ChannelsCenterView from './channels/center_view';
import {ChannelAttributeLabels} from './channels/channel_attributes';
import CreateTeamForm from './channels/create_team_form';
import ChannelsHeader from './channels/header';
import ChannelsPost from './channels/post';
import ChannelsPostCreate from './channels/post_create';
import ChannelsPostEdit from './channels/post_edit';
import ChannelNotificationPreferencesModal from './channels/channel_notification_preferences_modal';
import ChannelSettingsModal from './channels/channel_settings/channel_settings_modal';
import ChannelsSidebarLeft from './channels/sidebar_left';
import ChannelsSidebarRight from './channels/sidebar_right';
import AddPeopleToChannelModal from './channels/add_people_to_channel_modal';
import ChannelBookmarksBar from './channels/channel_bookmarks_bar';
import ChannelBookmarksCreateModal from './channels/channel_bookmarks_create_modal';
import LeaveTeamModal from './channels/leave_team_modal';
import MarketplaceModal from './channels/marketplace_modal';
import UserGroupsModal from './channels/user_groups_modal';
import ViewUserGroupModal from './channels/view_user_group_modal';
import DeletePostConfirmationDialog from './channels/delete_post_confirmation_dialog';
import DeletePostModal from './channels/delete_post_modal';
import DeleteScheduledPostModal from './channels/delete_scheduled_post_modal';
import DirectChannelsModal from './channels/direct_channels_modal';
import ChannelMenu from './channels/channel_menu';
import DraftPost from './channels/draft_post';
import EditChannelHeaderModal from './channels/edit_channel_header_modal';
import EmojiGifPicker from './channels/emoji_gif_picker';
import FindChannelsModal from './channels/find_channels_modal';
import NewChannelModal from './channels/new_channel_modal';
import FlagPostConfirmationDialog from './channels/flag_post_confirmation_dialog';
import GenericConfirmModal from './channels/generic_confirm_modal';
import InvitePeopleModal from './channels/invite_people_modal';
import ImagePreviewModal from './channels/image_preview_modal';
import LinkPreview from './channels/link_preview';
import MembersInvitedModal from './channels/members_invited_modal';
import MessagePriority from './channels/message_priority';
import ChannelsMobileNavbar from './channels/mobile_navbar';
import PostDotMenu from './channels/post_dot_menu';
import PostMenu from './channels/post_menu';
import PostReminderMenu from './channels/post_reminder_menu';
import ProfileModal from './channels/profile_modal';
import RestorePostConfirmationDialog from './channels/restore_post_confirmation_dialog';
import ScheduledDraftModal from './channels/scheduled_draft_modal';
import ScheduledPost from './channels/scheduled_post';
import ScheduledPostIndicator from './channels/scheduled_post_indicator';
import ScheduleMessageMenu from './channels/schedule_message_menu';
import ScheduleMessageModal from './channels/schedule_message_modal';
import SearchBox from './channels/search_box';
import SearchResultsPanel from './channels/search_results_panel';
import SendMessageNowModal from './channels/send_message_now_modal';
import SettingsModal from './channels/settings/settings_modal';
import SwitchProductMenu from './channels/switch_product_menu';
import TeamMenu from './channels/team_menu';
import TeamSettingsModal from './channels/team_settings/team_settings_modal';
import ThreadFooter from './channels/thread_footer';
import UnarchiveChannelModal from './channels/unarchive_channel_modal';
import UserProfilePopover from './channels/user_profile_popover';
import WysiwygEditor from './channels/wysiwyg_editor';
// Burn-on-Read Components
import BurnOnReadBadge from './channels/burn_on_read_badge';
import BurnOnReadTimerChip from './channels/burn_on_read_timer_chip';
import BurnOnReadConcealedPlaceholder from './channels/burn_on_read_concealed_placeholder';
import BurnOnReadConfirmationModal from './channels/burn_on_read_confirmation_modal';
// System Console Components
import {
    AdminSectionPanel,
    DropdownSetting,
    NumberInputSetting,
    RadioSetting,
    TextInputSetting,
} from './system_console/base_components';
import DelegatedGranularAdministration from './system_console/sections/user_management/delegated_granular_administration';
import UserDetail from './system_console/sections/user_management/user_detail';
import EditionAndLicense from './system_console/sections/about/edition_and_license';
import MobileSecurity from './system_console/sections/environment/mobile_security';
import Notifications from './system_console/sections/site_configuration/notifications';
import UsersAndTeams from './system_console/sections/site_configuration/users_and_teams';
import SystemConsoleFeatureDiscovery from './system_console/sections/system_users/feature_discovery';
import SystemConsoleHeader from './system_console/header';
import SystemConsoleNavbar from './system_console/navbar';
import SystemConsoleSidebar from './system_console/sidebar';
import SystemConsoleSidebarHeader from './system_console/sidebar_header';
import TeamStatistics from './system_console/sections/reporting/team_statistics';
import Users from './system_console/sections/user_management/users';

const components = {
    // Shared / Global
    AboutBuildModal,
    Footer,
    GlobalHeader,
    MainHeader,
    UserAccountMenu,

    // Channels
    ChannelAttributeLabels,
    ChannelsAppBar,
    ChannelsCenterView,
    CreateTeamForm,
    ChannelNotificationPreferencesModal,
    ChannelsHeader,
    ChannelsPost,
    ChannelsPostCreate,
    ChannelsPostEdit,
    ChannelSettingsModal,
    ChannelsSidebarLeft,
    ChannelsSidebarRight,
    AddPeopleToChannelModal,
    ChannelBookmarksBar,
    ChannelBookmarksCreateModal,
    LeaveTeamModal,
    MarketplaceModal,
    UserGroupsModal,
    ViewUserGroupModal,
    DeletePostConfirmationDialog,
    DeletePostModal,
    DeleteScheduledPostModal,
    ChannelMenu,
    DirectChannelsModal,
    DraftPost,
    EditChannelHeaderModal,
    EmojiGifPicker,
    FindChannelsModal,
    FlagPostConfirmationDialog,
    NewChannelModal,
    BrowseChannelsModal,
    GenericConfirmModal,
    InvitePeopleModal,
    ImagePreviewModal,
    LinkPreview,
    MembersInvitedModal,
    MessagePriority,
    ChannelsMobileNavbar,
    PostDotMenu,
    PostMenu,
    PostReminderMenu,
    ProfileModal,
    RestorePostConfirmationDialog,
    ScheduledDraftModal,
    ScheduledPost,
    ScheduledPostIndicator,
    ScheduleMessageMenu,
    ScheduleMessageModal,
    SearchBox,
    SearchResultsPanel,
    SendMessageNowModal,
    SettingsModal,
    SwitchProductMenu,
    TeamMenu,
    TeamSettingsModal,
    ThreadFooter,
    UnarchiveChannelModal,
    UserProfilePopover,
    WysiwygEditor,

    // Burn-on-Read
    BurnOnReadBadge,
    BurnOnReadTimerChip,
    BurnOnReadConcealedPlaceholder,
    BurnOnReadConfirmationModal,

    // System Console
    AdminSectionPanel,
    DelegatedGranularAdministration,
    DropdownSetting,
    EditionAndLicense,
    MobileSecurity,
    Notifications,
    NumberInputSetting,
    RadioSetting,
    UsersAndTeams,
    SystemConsoleFeatureDiscovery,
    SystemConsoleHeader,
    SystemConsoleNavbar,
    SystemConsoleSidebar,
    SystemConsoleSidebarHeader,
    TeamStatistics,
    TextInputSetting,
    UserDetail,
    Users,
};

export {
    components,

    // Shared / Global
    AboutBuildModal,
    Footer,
    GlobalHeader,
    MainHeader,
    UserAccountMenu,

    // Channels Page
    ChannelAttributeLabels,
    ChannelsAppBar,
    ChannelsCenterView,
    CreateTeamForm,
    ChannelNotificationPreferencesModal,
    ChannelsHeader,
    ChannelsPost,
    ChannelsPostCreate,
    ChannelsPostEdit,
    ChannelSettingsModal,
    ChannelsSidebarLeft,
    ChannelsSidebarRight,
    AddPeopleToChannelModal,
    ChannelBookmarksBar,
    ChannelBookmarksCreateModal,
    LeaveTeamModal,
    MarketplaceModal,
    UserGroupsModal,
    ViewUserGroupModal,
    DeletePostConfirmationDialog,
    DeletePostModal,
    DeleteScheduledPostModal,
    DraftPost,
    ChannelMenu,
    EditChannelHeaderModal,
    EmojiGifPicker,
    FindChannelsModal,
    FlagPostConfirmationDialog,
    NewChannelModal,
    BrowseChannelsModal,
    DirectChannelsModal,
    GenericConfirmModal,
    InvitePeopleModal,
    ImagePreviewModal,
    LinkPreview,
    MembersInvitedModal,
    MessagePriority,
    ChannelsMobileNavbar,
    PostDotMenu,
    PostMenu,
    PostReminderMenu,
    ProfileModal,
    RestorePostConfirmationDialog,
    ScheduledDraftModal,
    ScheduledPost,
    ScheduledPostIndicator,
    ScheduleMessageMenu,
    ScheduleMessageModal,
    SearchBox,
    SearchResultsPanel,
    SendMessageNowModal,
    SettingsModal,
    SwitchProductMenu,
    TeamMenu,
    TeamSettingsModal,
    ThreadFooter,
    UnarchiveChannelModal,
    UserProfilePopover,
    WysiwygEditor,

    // Burn-on-Read
    BurnOnReadBadge,
    BurnOnReadTimerChip,
    BurnOnReadConcealedPlaceholder,
    BurnOnReadConfirmationModal,

    // System Console
    AdminSectionPanel,
    DelegatedGranularAdministration,
    DropdownSetting,
    EditionAndLicense,
    MobileSecurity,
    Notifications,
    NumberInputSetting,
    RadioSetting,
    UsersAndTeams,
    SystemConsoleFeatureDiscovery,
    SystemConsoleHeader,
    SystemConsoleNavbar,
    SystemConsoleSidebar,
    SystemConsoleSidebarHeader,
    TeamStatistics,
    TextInputSetting,
    UserDetail,
    Users,
};

export type {PriorityOption} from './channels/message_priority';
