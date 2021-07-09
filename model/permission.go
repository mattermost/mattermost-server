// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

const (
	PermissionScopeSystem  = "system_scope"
	PermissionScopeTeam    = "team_scope"
	PermissionScopeChannel = "channel_scope"
)

type Permission struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
}

var PermissionInviteUser *Permission
var PermissionAddUserToTeam *Permission
var PermissionUseSlashCommands *Permission
var PermissionManageSlashCommands *Permission
var PermissionManageOthersSlashCommands *Permission
var PermissionCreatePublicChannel *Permission
var PermissionCreatePrivateChannel *Permission
var PermissionManagePublicChannelMembers *Permission
var PermissionManagePrivateChannelMembers *Permission
var PermissionConvertPublicChannelToPrivate *Permission
var PermissionConvertPrivateChannelToPublic *Permission
var PermissionAssignSystemAdminRole *Permission
var PermissionManageRoles *Permission
var PermissionManageTeamRoles *Permission
var PermissionManageChannelRoles *Permission
var PermissionCreateDirectChannel *Permission
var PermissionCreateGroupChannel *Permission
var PermissionManagePublicChannelProperties *Permission
var PermissionManagePrivateChannelProperties *Permission
var PermissionListPublicTeams *Permission
var PermissionJoinPublicTeams *Permission
var PermissionListPrivateTeams *Permission
var PermissionJoinPrivateTeams *Permission
var PermissionListTeamChannels *Permission
var PermissionJoinPublicChannels *Permission
var PermissionDeletePublicChannel *Permission
var PermissionDeletePrivateChannel *Permission
var PermissionEditOtherUsers *Permission
var PermissionReadChannel *Permission
var PermissionReadPublicChannelGroups *Permission
var PermissionReadPrivateChannelGroups *Permission
var PermissionReadPublicChannel *Permission
var PermissionAddReaction *Permission
var PermissionRemoveReaction *Permission
var PermissionRemoveOthersReactions *Permission
var PermissionPermanentDeleteUser *Permission
var PermissionUploadFile *Permission
var PermissionGetPublicLink *Permission
var PermissionManageWebhooks *Permission
var PermissionManageOthersWebhooks *Permission
var PermissionManageIncomingWebhooks *Permission
var PermissionManageOutgoingWebhooks *Permission
var PermissionManageOthersIncomingWebhooks *Permission
var PermissionManageOthersOutgoingWebhooks *Permission
var PermissionManageOAuth *Permission
var PermissionManageSystemWideOAuth *Permission
var PermissionManageEmojis *Permission
var PermissionManageOthersEmojis *Permission
var PermissionCreateEmojis *Permission
var PermissionDeleteEmojis *Permission
var PermissionDeleteOthersEmojis *Permission
var PermissionCreatePost *Permission
var PermissionCreatePostPublic *Permission
var PermissionCreatePostEphemeral *Permission
var PermissionEditPost *Permission
var PermissionEditOthersPosts *Permission
var PermissionDeletePost *Permission
var PermissionDeleteOthersPosts *Permission
var PermissionRemoveUserFromTeam *Permission
var PermissionCreateTeam *Permission
var PermissionManageTeam *Permission
var PermissionImportTeam *Permission
var PermissionViewTeam *Permission
var PermissionListUsersWithoutTeam *Permission
var PermissionReadJobs *Permission
var PermissionManageJobs *Permission
var PermissionCreateUserAccessToken *Permission
var PermissionReadUserAccessToken *Permission
var PermissionRevokeUserAccessToken *Permission
var PermissionCreateBot *Permission
var PermissionAssignBot *Permission
var PermissionReadBots *Permission
var PermissionReadOthersBots *Permission
var PermissionManageBots *Permission
var PermissionManageOthersBots *Permission
var PermissionViewMembers *Permission
var PermissionInviteGuest *Permission
var PermissionPromoteGuest *Permission
var PermissionDemoteToGuest *Permission
var PermissionUseChannelMentions *Permission
var PermissionUseGroupMentions *Permission
var PermissionReadOtherUsersTeams *Permission
var PermissionEditBrand *Permission
var PermissionManageSharedChannels *Permission
var PermissionManageSecureConnections *Permission
var PermissionDownloadComplianceExportResult *Permission
var PermissionCreateDataRetentionJob *Permission
var PermissionReadDataRetentionJob *Permission
var PermissionCreateComplianceExportJob *Permission
var PermissionReadComplianceExportJob *Permission
var PermissionReadAudits *Permission
var PermissionTestElasticsearch *Permission
var PermissionTestSiteURL *Permission
var PermissionTestS3 *Permission
var PermissionReloadConfig *Permission
var PermissionInvalidateCaches *Permission
var PermissionRecycleDatabaseConnections *Permission
var PermissionPurgeElasticsearchIndexes *Permission
var PermissionTestEmail *Permission
var PermissionCreateElasticsearchPostIndexingJob *Permission
var PermissionCreateElasticsearchPostAggregationJob *Permission
var PermissionReadElasticsearchPostIndexingJob *Permission
var PermissionReadElasticsearchPostAggregationJob *Permission
var PermissionPurgeBleveIndexes *Permission
var PermissionCreatePostBleveIndexesJob *Permission
var PermissionCreateLdapSyncJob *Permission
var PermissionReadLdapSyncJob *Permission
var PermissionTestLdap *Permission
var PermissionInvalidateEmailInvite *Permission
var PermissionGetSamlMetadataFromIDp *Permission
var PermissionAddSamlPublicCert *Permission
var PermissionAddSamlPrivateCert *Permission
var PermissionAddSamlIDpCert *Permission
var PermissionRemoveSamlPublicCert *Permission
var PermissionRemoveSamlPrivateCert *Permission
var PermissionRemoveSamlIDpCert *Permission
var PermissionGetSamlCertStatus *Permission
var PermissionAddLdapPublicCert *Permission
var PermissionAddLdapPrivateCert *Permission
var PermissionRemoveLdapPublicCert *Permission
var PermissionRemoveLdapPrivateCert *Permission
var PermissionGetLogs *Permission
var PermissionGetAnalytics *Permission
var PermissionReadLicenseInformation *Permission
var PermissionManageLicenseInformation *Permission

var PermissionSysconsoleReadAbout *Permission
var PermissionSysconsoleWriteAbout *Permission

var PermissionSysconsoleReadAboutEditionAndLicense *Permission
var PermissionSysconsoleWriteAboutEditionAndLicense *Permission

var PermissionSysconsoleReadBilling *Permission
var PermissionSysconsoleWriteBilling *Permission

var PermissionSysconsoleReadReporting *Permission
var PermissionSysconsoleWriteReporting *Permission

var PermissionSysconsoleReadReportingSiteStatistics *Permission
var PermissionSysconsoleWriteReportingSiteStatistics *Permission

var PermissionSysconsoleReadReportingTeamStatistics *Permission
var PermissionSysconsoleWriteReportingTeamStatistics *Permission

var PermissionSysconsoleReadReportingServerLogs *Permission
var PermissionSysconsoleWriteReportingServerLogs *Permission

var PermissionSysconsoleReadUserManagementUsers *Permission
var PermissionSysconsoleWriteUserManagementUsers *Permission

var PermissionSysconsoleReadUserManagementGroups *Permission
var PermissionSysconsoleWriteUserManagementGroups *Permission

var PermissionSysconsoleReadUserManagementTeams *Permission
var PermissionSysconsoleWriteUserManagementTeams *Permission

var PermissionSysconsoleReadUserManagementChannels *Permission
var PermissionSysconsoleWriteUserManagementChannels *Permission

var PermissionSysconsoleReadUserManagementPermissions *Permission
var PermissionSysconsoleWriteUserManagementPermissions *Permission

var PermissionSysconsoleReadUserManagementSystemRoles *Permission
var PermissionSysconsoleWriteUserManagementSystemRoles *Permission

// DEPRECATED
var PermissionSysconsoleReadEnvironment *Permission

// DEPRECATED
var PermissionSysconsoleWriteEnvironment *Permission

var PermissionSysconsoleReadEnvironmentWebServer *Permission
var PermissionSysconsoleWriteEnvironmentWebServer *Permission

var PermissionSysconsoleReadEnvironmentDatabase *Permission
var PermissionSysconsoleWriteEnvironmentDatabase *Permission

var PermissionSysconsoleReadEnvironmentElasticsearch *Permission
var PermissionSysconsoleWriteEnvironmentElasticsearch *Permission

var PermissionSysconsoleReadEnvironmentFileStorage *Permission
var PermissionSysconsoleWriteEnvironmentFileStorage *Permission

var PermissionSysconsoleReadEnvironmentImageProxy *Permission
var PermissionSysconsoleWriteEnvironmentImageProxy *Permission

var PermissionSysconsoleReadEnvironmentSMTP *Permission
var PermissionSysconsoleWriteEnvironmentSMTP *Permission

var PermissionSysconsoleReadEnvironmentPushNotificationServer *Permission
var PermissionSysconsoleWriteEnvironmentPushNotificationServer *Permission

var PermissionSysconsoleReadEnvironmentHighAvailability *Permission
var PermissionSysconsoleWriteEnvironmentHighAvailability *Permission

var PermissionSysconsoleReadEnvironmentRateLimiting *Permission
var PermissionSysconsoleWriteEnvironmentRateLimiting *Permission

var PermissionSysconsoleReadEnvironmentLogging *Permission
var PermissionSysconsoleWriteEnvironmentLogging *Permission

var PermissionSysconsoleReadEnvironmentSessionLengths *Permission
var PermissionSysconsoleWriteEnvironmentSessionLengths *Permission

var PermissionSysconsoleReadEnvironmentPerformanceMonitoring *Permission
var PermissionSysconsoleWriteEnvironmentPerformanceMonitoring *Permission

var PermissionSysconsoleReadEnvironmentDeveloper *Permission
var PermissionSysconsoleWriteEnvironmentDeveloper *Permission

var PermissionSysconsoleReadSite *Permission
var PermissionSysconsoleWriteSite *Permission

var PermissionSysconsoleReadSiteCustomization *Permission
var PermissionSysconsoleWriteSiteCustomization *Permission

var PermissionSysconsoleReadSiteLocalization *Permission
var PermissionSysconsoleWriteSiteLocalization *Permission

var PermissionSysconsoleReadSiteUsersAndTeams *Permission
var PermissionSysconsoleWriteSiteUsersAndTeams *Permission

var PermissionSysconsoleReadSiteNotifications *Permission
var PermissionSysconsoleWriteSiteNotifications *Permission

var PermissionSysconsoleReadSiteAnnouncementBanner *Permission
var PermissionSysconsoleWriteSiteAnnouncementBanner *Permission

var PermissionSysconsoleReadSiteEmoji *Permission
var PermissionSysconsoleWriteSiteEmoji *Permission

var PermissionSysconsoleReadSitePosts *Permission
var PermissionSysconsoleWriteSitePosts *Permission

var PermissionSysconsoleReadSiteFileSharingAndDownloads *Permission
var PermissionSysconsoleWriteSiteFileSharingAndDownloads *Permission

var PermissionSysconsoleReadSitePublicLinks *Permission
var PermissionSysconsoleWriteSitePublicLinks *Permission

var PermissionSysconsoleReadSiteNotices *Permission
var PermissionSysconsoleWriteSiteNotices *Permission

var PermissionSysconsoleReadAuthentication *Permission
var PermissionSysconsoleWriteAuthentication *Permission

var PermissionSysconsoleReadAuthenticationSignup *Permission
var PermissionSysconsoleWriteAuthenticationSignup *Permission

var PermissionSysconsoleReadAuthenticationEmail *Permission
var PermissionSysconsoleWriteAuthenticationEmail *Permission

var PermissionSysconsoleReadAuthenticationPassword *Permission
var PermissionSysconsoleWriteAuthenticationPassword *Permission

var PermissionSysconsoleReadAuthenticationMfa *Permission
var PermissionSysconsoleWriteAuthenticationMfa *Permission

var PermissionSysconsoleReadAuthenticationLdap *Permission
var PermissionSysconsoleWriteAuthenticationLdap *Permission

var PermissionSysconsoleReadAuthenticationSaml *Permission
var PermissionSysconsoleWriteAuthenticationSaml *Permission

var PermissionSysconsoleReadAuthenticationOpenid *Permission
var PermissionSysconsoleWriteAuthenticationOpenid *Permission

var PermissionSysconsoleReadAuthenticationGuestAccess *Permission
var PermissionSysconsoleWriteAuthenticationGuestAccess *Permission

var PermissionSysconsoleReadPlugins *Permission
var PermissionSysconsoleWritePlugins *Permission

var PermissionSysconsoleReadIntegrations *Permission
var PermissionSysconsoleWriteIntegrations *Permission

var PermissionSysconsoleReadIntegrationsIntegrationManagement *Permission
var PermissionSysconsoleWriteIntegrationsIntegrationManagement *Permission

var PermissionSysconsoleReadIntegrationsBotAccounts *Permission
var PermissionSysconsoleWriteIntegrationsBotAccounts *Permission

var PermissionSysconsoleReadIntegrationsGif *Permission
var PermissionSysconsoleWriteIntegrationsGif *Permission

var PermissionSysconsoleReadIntegrationsCors *Permission
var PermissionSysconsoleWriteIntegrationsCors *Permission

var PermissionSysconsoleReadCompliance *Permission
var PermissionSysconsoleWriteCompliance *Permission

var PermissionSysconsoleReadComplianceDataRetentionPolicy *Permission
var PermissionSysconsoleWriteComplianceDataRetentionPolicy *Permission

var PermissionSysconsoleReadComplianceComplianceExport *Permission
var PermissionSysconsoleWriteComplianceComplianceExport *Permission

var PermissionSysconsoleReadComplianceComplianceMonitoring *Permission
var PermissionSysconsoleWriteComplianceComplianceMonitoring *Permission

var PermissionSysconsoleReadComplianceCustomTermsOfService *Permission
var PermissionSysconsoleWriteComplianceCustomTermsOfService *Permission

var PermissionSysconsoleReadExperimental *Permission
var PermissionSysconsoleWriteExperimental *Permission

var PermissionSysconsoleReadExperimentalFeatures *Permission
var PermissionSysconsoleWriteExperimentalFeatures *Permission

var PermissionSysconsoleReadExperimentalFeatureFlags *Permission
var PermissionSysconsoleWriteExperimentalFeatureFlags *Permission

var PermissionSysconsoleReadExperimentalBleve *Permission
var PermissionSysconsoleWriteExperimentalBleve *Permission

// General permission that encompasses all system admin functions
// in the future this could be broken up to allow access to some
// admin functions but not others
var PermissionManageSystem *Permission

var AllPermissions []*Permission
var DeprecatedPermissions []*Permission

var ChannelModeratedPermissions []string
var ChannelModeratedPermissionsMap map[string]string

var SysconsoleReadPermissions []*Permission
var SysconsoleWritePermissions []*Permission

func initializePermissions() {
	PermissionInviteUser = &Permission{
		"invite_user",
		"authentication.permissions.team_invite_user.name",
		"authentication.permissions.team_invite_user.description",
		PermissionScopeTeam,
	}
	PermissionAddUserToTeam = &Permission{
		"add_user_to_team",
		"authentication.permissions.add_user_to_team.name",
		"authentication.permissions.add_user_to_team.description",
		PermissionScopeTeam,
	}
	PermissionUseSlashCommands = &Permission{
		"use_slash_commands",
		"authentication.permissions.team_use_slash_commands.name",
		"authentication.permissions.team_use_slash_commands.description",
		PermissionScopeChannel,
	}
	PermissionManageSlashCommands = &Permission{
		"manage_slash_commands",
		"authentication.permissions.manage_slash_commands.name",
		"authentication.permissions.manage_slash_commands.description",
		PermissionScopeTeam,
	}
	PermissionManageOthersSlashCommands = &Permission{
		"manage_others_slash_commands",
		"authentication.permissions.manage_others_slash_commands.name",
		"authentication.permissions.manage_others_slash_commands.description",
		PermissionScopeTeam,
	}
	PermissionCreatePublicChannel = &Permission{
		"create_public_channel",
		"authentication.permissions.create_public_channel.name",
		"authentication.permissions.create_public_channel.description",
		PermissionScopeTeam,
	}
	PermissionCreatePrivateChannel = &Permission{
		"create_private_channel",
		"authentication.permissions.create_private_channel.name",
		"authentication.permissions.create_private_channel.description",
		PermissionScopeTeam,
	}
	PermissionManagePublicChannelMembers = &Permission{
		"manage_public_channel_members",
		"authentication.permissions.manage_public_channel_members.name",
		"authentication.permissions.manage_public_channel_members.description",
		PermissionScopeChannel,
	}
	PermissionManagePrivateChannelMembers = &Permission{
		"manage_private_channel_members",
		"authentication.permissions.manage_private_channel_members.name",
		"authentication.permissions.manage_private_channel_members.description",
		PermissionScopeChannel,
	}
	PermissionConvertPublicChannelToPrivate = &Permission{
		"convert_public_channel_to_private",
		"authentication.permissions.convert_public_channel_to_private.name",
		"authentication.permissions.convert_public_channel_to_private.description",
		PermissionScopeChannel,
	}
	PermissionConvertPrivateChannelToPublic = &Permission{
		"convert_private_channel_to_public",
		"authentication.permissions.convert_private_channel_to_public.name",
		"authentication.permissions.convert_private_channel_to_public.description",
		PermissionScopeChannel,
	}
	PermissionAssignSystemAdminRole = &Permission{
		"assign_system_admin_role",
		"authentication.permissions.assign_system_admin_role.name",
		"authentication.permissions.assign_system_admin_role.description",
		PermissionScopeSystem,
	}
	PermissionManageRoles = &Permission{
		"manage_roles",
		"authentication.permissions.manage_roles.name",
		"authentication.permissions.manage_roles.description",
		PermissionScopeSystem,
	}
	PermissionManageTeamRoles = &Permission{
		"manage_team_roles",
		"authentication.permissions.manage_team_roles.name",
		"authentication.permissions.manage_team_roles.description",
		PermissionScopeTeam,
	}
	PermissionManageChannelRoles = &Permission{
		"manage_channel_roles",
		"authentication.permissions.manage_channel_roles.name",
		"authentication.permissions.manage_channel_roles.description",
		PermissionScopeChannel,
	}
	PermissionManageSystem = &Permission{
		"manage_system",
		"authentication.permissions.manage_system.name",
		"authentication.permissions.manage_system.description",
		PermissionScopeSystem,
	}
	PermissionCreateDirectChannel = &Permission{
		"create_direct_channel",
		"authentication.permissions.create_direct_channel.name",
		"authentication.permissions.create_direct_channel.description",
		PermissionScopeSystem,
	}
	PermissionCreateGroupChannel = &Permission{
		"create_group_channel",
		"authentication.permissions.create_group_channel.name",
		"authentication.permissions.create_group_channel.description",
		PermissionScopeSystem,
	}
	PermissionManagePublicChannelProperties = &Permission{
		"manage_public_channel_properties",
		"authentication.permissions.manage_public_channel_properties.name",
		"authentication.permissions.manage_public_channel_properties.description",
		PermissionScopeChannel,
	}
	PermissionManagePrivateChannelProperties = &Permission{
		"manage_private_channel_properties",
		"authentication.permissions.manage_private_channel_properties.name",
		"authentication.permissions.manage_private_channel_properties.description",
		PermissionScopeChannel,
	}
	PermissionListPublicTeams = &Permission{
		"list_public_teams",
		"authentication.permissions.list_public_teams.name",
		"authentication.permissions.list_public_teams.description",
		PermissionScopeSystem,
	}
	PermissionJoinPublicTeams = &Permission{
		"join_public_teams",
		"authentication.permissions.join_public_teams.name",
		"authentication.permissions.join_public_teams.description",
		PermissionScopeSystem,
	}
	PermissionListPrivateTeams = &Permission{
		"list_private_teams",
		"authentication.permissions.list_private_teams.name",
		"authentication.permissions.list_private_teams.description",
		PermissionScopeSystem,
	}
	PermissionJoinPrivateTeams = &Permission{
		"join_private_teams",
		"authentication.permissions.join_private_teams.name",
		"authentication.permissions.join_private_teams.description",
		PermissionScopeSystem,
	}
	PermissionListTeamChannels = &Permission{
		"list_team_channels",
		"authentication.permissions.list_team_channels.name",
		"authentication.permissions.list_team_channels.description",
		PermissionScopeTeam,
	}
	PermissionJoinPublicChannels = &Permission{
		"join_public_channels",
		"authentication.permissions.join_public_channels.name",
		"authentication.permissions.join_public_channels.description",
		PermissionScopeTeam,
	}
	PermissionDeletePublicChannel = &Permission{
		"delete_public_channel",
		"authentication.permissions.delete_public_channel.name",
		"authentication.permissions.delete_public_channel.description",
		PermissionScopeChannel,
	}
	PermissionDeletePrivateChannel = &Permission{
		"delete_private_channel",
		"authentication.permissions.delete_private_channel.name",
		"authentication.permissions.delete_private_channel.description",
		PermissionScopeChannel,
	}
	PermissionEditOtherUsers = &Permission{
		"edit_other_users",
		"authentication.permissions.edit_other_users.name",
		"authentication.permissions.edit_other_users.description",
		PermissionScopeSystem,
	}
	PermissionReadChannel = &Permission{
		"read_channel",
		"authentication.permissions.read_channel.name",
		"authentication.permissions.read_channel.description",
		PermissionScopeChannel,
	}
	PermissionReadPublicChannelGroups = &Permission{
		"read_public_channel_groups",
		"authentication.permissions.read_public_channel_groups.name",
		"authentication.permissions.read_public_channel_groups.description",
		PermissionScopeChannel,
	}
	PermissionReadPrivateChannelGroups = &Permission{
		"read_private_channel_groups",
		"authentication.permissions.read_private_channel_groups.name",
		"authentication.permissions.read_private_channel_groups.description",
		PermissionScopeChannel,
	}
	PermissionReadPublicChannel = &Permission{
		"read_public_channel",
		"authentication.permissions.read_public_channel.name",
		"authentication.permissions.read_public_channel.description",
		PermissionScopeTeam,
	}
	PermissionAddReaction = &Permission{
		"add_reaction",
		"authentication.permissions.add_reaction.name",
		"authentication.permissions.add_reaction.description",
		PermissionScopeChannel,
	}
	PermissionRemoveReaction = &Permission{
		"remove_reaction",
		"authentication.permissions.remove_reaction.name",
		"authentication.permissions.remove_reaction.description",
		PermissionScopeChannel,
	}
	PermissionRemoveOthersReactions = &Permission{
		"remove_others_reactions",
		"authentication.permissions.remove_others_reactions.name",
		"authentication.permissions.remove_others_reactions.description",
		PermissionScopeChannel,
	}
	// DEPRECATED
	PermissionPermanentDeleteUser = &Permission{
		"permanent_delete_user",
		"authentication.permissions.permanent_delete_user.name",
		"authentication.permissions.permanent_delete_user.description",
		PermissionScopeSystem,
	}
	PermissionUploadFile = &Permission{
		"upload_file",
		"authentication.permissions.upload_file.name",
		"authentication.permissions.upload_file.description",
		PermissionScopeChannel,
	}
	PermissionGetPublicLink = &Permission{
		"get_public_link",
		"authentication.permissions.get_public_link.name",
		"authentication.permissions.get_public_link.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionManageWebhooks = &Permission{
		"manage_webhooks",
		"authentication.permissions.manage_webhooks.name",
		"authentication.permissions.manage_webhooks.description",
		PermissionScopeTeam,
	}
	// DEPRECATED
	PermissionManageOthersWebhooks = &Permission{
		"manage_others_webhooks",
		"authentication.permissions.manage_others_webhooks.name",
		"authentication.permissions.manage_others_webhooks.description",
		PermissionScopeTeam,
	}
	PermissionManageIncomingWebhooks = &Permission{
		"manage_incoming_webhooks",
		"authentication.permissions.manage_incoming_webhooks.name",
		"authentication.permissions.manage_incoming_webhooks.description",
		PermissionScopeTeam,
	}
	PermissionManageOutgoingWebhooks = &Permission{
		"manage_outgoing_webhooks",
		"authentication.permissions.manage_outgoing_webhooks.name",
		"authentication.permissions.manage_outgoing_webhooks.description",
		PermissionScopeTeam,
	}
	PermissionManageOthersIncomingWebhooks = &Permission{
		"manage_others_incoming_webhooks",
		"authentication.permissions.manage_others_incoming_webhooks.name",
		"authentication.permissions.manage_others_incoming_webhooks.description",
		PermissionScopeTeam,
	}
	PermissionManageOthersOutgoingWebhooks = &Permission{
		"manage_others_outgoing_webhooks",
		"authentication.permissions.manage_others_outgoing_webhooks.name",
		"authentication.permissions.manage_others_outgoing_webhooks.description",
		PermissionScopeTeam,
	}
	PermissionManageOAuth = &Permission{
		"manage_oauth",
		"authentication.permissions.manage_oauth.name",
		"authentication.permissions.manage_oauth.description",
		PermissionScopeSystem,
	}
	PermissionManageSystemWideOAuth = &Permission{
		"manage_system_wide_oauth",
		"authentication.permissions.manage_system_wide_oauth.name",
		"authentication.permissions.manage_system_wide_oauth.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionManageEmojis = &Permission{
		"manage_emojis",
		"authentication.permissions.manage_emojis.name",
		"authentication.permissions.manage_emojis.description",
		PermissionScopeTeam,
	}
	// DEPRECATED
	PermissionManageOthersEmojis = &Permission{
		"manage_others_emojis",
		"authentication.permissions.manage_others_emojis.name",
		"authentication.permissions.manage_others_emojis.description",
		PermissionScopeTeam,
	}
	PermissionCreateEmojis = &Permission{
		"create_emojis",
		"authentication.permissions.create_emojis.name",
		"authentication.permissions.create_emojis.description",
		PermissionScopeTeam,
	}
	PermissionDeleteEmojis = &Permission{
		"delete_emojis",
		"authentication.permissions.delete_emojis.name",
		"authentication.permissions.delete_emojis.description",
		PermissionScopeTeam,
	}
	PermissionDeleteOthersEmojis = &Permission{
		"delete_others_emojis",
		"authentication.permissions.delete_others_emojis.name",
		"authentication.permissions.delete_others_emojis.description",
		PermissionScopeTeam,
	}
	PermissionCreatePost = &Permission{
		"create_post",
		"authentication.permissions.create_post.name",
		"authentication.permissions.create_post.description",
		PermissionScopeChannel,
	}
	PermissionCreatePostPublic = &Permission{
		"create_post_public",
		"authentication.permissions.create_post_public.name",
		"authentication.permissions.create_post_public.description",
		PermissionScopeChannel,
	}
	PermissionCreatePostEphemeral = &Permission{
		"create_post_ephemeral",
		"authentication.permissions.create_post_ephemeral.name",
		"authentication.permissions.create_post_ephemeral.description",
		PermissionScopeChannel,
	}
	PermissionEditPost = &Permission{
		"edit_post",
		"authentication.permissions.edit_post.name",
		"authentication.permissions.edit_post.description",
		PermissionScopeChannel,
	}
	PermissionEditOthersPosts = &Permission{
		"edit_others_posts",
		"authentication.permissions.edit_others_posts.name",
		"authentication.permissions.edit_others_posts.description",
		PermissionScopeChannel,
	}
	PermissionDeletePost = &Permission{
		"delete_post",
		"authentication.permissions.delete_post.name",
		"authentication.permissions.delete_post.description",
		PermissionScopeChannel,
	}
	PermissionDeleteOthersPosts = &Permission{
		"delete_others_posts",
		"authentication.permissions.delete_others_posts.name",
		"authentication.permissions.delete_others_posts.description",
		PermissionScopeChannel,
	}
	PermissionManageSharedChannels = &Permission{
		"manage_shared_channels",
		"authentication.permissions.manage_shared_channels.name",
		"authentication.permissions.manage_shared_channels.description",
		PermissionScopeSystem,
	}
	PermissionManageSecureConnections = &Permission{
		"manage_secure_connections",
		"authentication.permissions.manage_secure_connections.name",
		"authentication.permissions.manage_secure_connections.description",
		PermissionScopeSystem,
	}

	PermissionCreateDataRetentionJob = &Permission{
		"create_data_retention_job",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionReadDataRetentionJob = &Permission{
		"read_data_retention_job",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionCreateComplianceExportJob = &Permission{
		"create_compliance_export_job",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionReadComplianceExportJob = &Permission{
		"read_compliance_export_job",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionReadAudits = &Permission{
		"read_audits",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionPurgeBleveIndexes = &Permission{
		"purge_bleve_indexes",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionCreatePostBleveIndexesJob = &Permission{
		"create_post_bleve_indexes_job",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionCreateLdapSyncJob = &Permission{
		"create_ldap_sync_job",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionReadLdapSyncJob = &Permission{
		"read_ldap_sync_job",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionTestLdap = &Permission{
		"test_ldap",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionInvalidateEmailInvite = &Permission{
		"invalidate_email_invite",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionGetSamlMetadataFromIDp = &Permission{
		"get_saml_metadata_from_idp",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionAddSamlPublicCert = &Permission{
		"add_saml_public_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionAddSamlPrivateCert = &Permission{
		"add_saml_private_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionAddSamlIDpCert = &Permission{
		"add_saml_idp_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionRemoveSamlPublicCert = &Permission{
		"remove_saml_public_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionRemoveSamlPrivateCert = &Permission{
		"remove_saml_private_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionRemoveSamlIDpCert = &Permission{
		"remove_saml_idp_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionGetSamlCertStatus = &Permission{
		"get_saml_cert_status",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionAddLdapPublicCert = &Permission{
		"add_ldap_public_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionAddLdapPrivateCert = &Permission{
		"add_ldap_private_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionRemoveLdapPublicCert = &Permission{
		"remove_ldap_public_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionRemoveLdapPrivateCert = &Permission{
		"remove_ldap_private_cert",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionGetLogs = &Permission{
		"get_logs",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionReadLicenseInformation = &Permission{
		"read_license_information",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionGetAnalytics = &Permission{
		"get_analytics",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionManageLicenseInformation = &Permission{
		"manage_license_information",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionDownloadComplianceExportResult = &Permission{
		"download_compliance_export_result",
		"authentication.permissions.download_compliance_export_result.name",
		"authentication.permissions.download_compliance_export_result.description",
		PermissionScopeSystem,
	}

	PermissionTestSiteURL = &Permission{
		"test_site_url",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionTestElasticsearch = &Permission{
		"test_elasticsearch",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionTestS3 = &Permission{
		"test_s3",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionReloadConfig = &Permission{
		"reload_config",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionInvalidateCaches = &Permission{
		"invalidate_caches",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionRecycleDatabaseConnections = &Permission{
		"recycle_database_connections",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionPurgeElasticsearchIndexes = &Permission{
		"purge_elasticsearch_indexes",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionTestEmail = &Permission{
		"test_email",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionCreateElasticsearchPostIndexingJob = &Permission{
		"create_elasticsearch_post_indexing_job",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionCreateElasticsearchPostAggregationJob = &Permission{
		"create_elasticsearch_post_aggregation_job",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionReadElasticsearchPostIndexingJob = &Permission{
		"read_elasticsearch_post_indexing_job",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionReadElasticsearchPostAggregationJob = &Permission{
		"read_elasticsearch_post_aggregation_job",
		"",
		"",
		PermissionScopeSystem,
	}

	PermissionRemoveUserFromTeam = &Permission{
		"remove_user_from_team",
		"authentication.permissions.remove_user_from_team.name",
		"authentication.permissions.remove_user_from_team.description",
		PermissionScopeTeam,
	}
	PermissionCreateTeam = &Permission{
		"create_team",
		"authentication.permissions.create_team.name",
		"authentication.permissions.create_team.description",
		PermissionScopeSystem,
	}
	PermissionManageTeam = &Permission{
		"manage_team",
		"authentication.permissions.manage_team.name",
		"authentication.permissions.manage_team.description",
		PermissionScopeTeam,
	}
	PermissionImportTeam = &Permission{
		"import_team",
		"authentication.permissions.import_team.name",
		"authentication.permissions.import_team.description",
		PermissionScopeTeam,
	}
	PermissionViewTeam = &Permission{
		"view_team",
		"authentication.permissions.view_team.name",
		"authentication.permissions.view_team.description",
		PermissionScopeTeam,
	}
	PermissionListUsersWithoutTeam = &Permission{
		"list_users_without_team",
		"authentication.permissions.list_users_without_team.name",
		"authentication.permissions.list_users_without_team.description",
		PermissionScopeSystem,
	}
	PermissionCreateUserAccessToken = &Permission{
		"create_user_access_token",
		"authentication.permissions.create_user_access_token.name",
		"authentication.permissions.create_user_access_token.description",
		PermissionScopeSystem,
	}
	PermissionReadUserAccessToken = &Permission{
		"read_user_access_token",
		"authentication.permissions.read_user_access_token.name",
		"authentication.permissions.read_user_access_token.description",
		PermissionScopeSystem,
	}
	PermissionRevokeUserAccessToken = &Permission{
		"revoke_user_access_token",
		"authentication.permissions.revoke_user_access_token.name",
		"authentication.permissions.revoke_user_access_token.description",
		PermissionScopeSystem,
	}
	PermissionCreateBot = &Permission{
		"create_bot",
		"authentication.permissions.create_bot.name",
		"authentication.permissions.create_bot.description",
		PermissionScopeSystem,
	}
	PermissionAssignBot = &Permission{
		"assign_bot",
		"authentication.permissions.assign_bot.name",
		"authentication.permissions.assign_bot.description",
		PermissionScopeSystem,
	}
	PermissionReadBots = &Permission{
		"read_bots",
		"authentication.permissions.read_bots.name",
		"authentication.permissions.read_bots.description",
		PermissionScopeSystem,
	}
	PermissionReadOthersBots = &Permission{
		"read_others_bots",
		"authentication.permissions.read_others_bots.name",
		"authentication.permissions.read_others_bots.description",
		PermissionScopeSystem,
	}
	PermissionManageBots = &Permission{
		"manage_bots",
		"authentication.permissions.manage_bots.name",
		"authentication.permissions.manage_bots.description",
		PermissionScopeSystem,
	}
	PermissionManageOthersBots = &Permission{
		"manage_others_bots",
		"authentication.permissions.manage_others_bots.name",
		"authentication.permissions.manage_others_bots.description",
		PermissionScopeSystem,
	}
	PermissionReadJobs = &Permission{
		"read_jobs",
		"authentication.permisssions.read_jobs.name",
		"authentication.permisssions.read_jobs.description",
		PermissionScopeSystem,
	}
	PermissionManageJobs = &Permission{
		"manage_jobs",
		"authentication.permisssions.manage_jobs.name",
		"authentication.permisssions.manage_jobs.description",
		PermissionScopeSystem,
	}
	PermissionViewMembers = &Permission{
		"view_members",
		"authentication.permisssions.view_members.name",
		"authentication.permisssions.view_members.description",
		PermissionScopeTeam,
	}
	PermissionInviteGuest = &Permission{
		"invite_guest",
		"authentication.permissions.invite_guest.name",
		"authentication.permissions.invite_guest.description",
		PermissionScopeTeam,
	}
	PermissionPromoteGuest = &Permission{
		"promote_guest",
		"authentication.permissions.promote_guest.name",
		"authentication.permissions.promote_guest.description",
		PermissionScopeSystem,
	}
	PermissionDemoteToGuest = &Permission{
		"demote_to_guest",
		"authentication.permissions.demote_to_guest.name",
		"authentication.permissions.demote_to_guest.description",
		PermissionScopeSystem,
	}
	PermissionUseChannelMentions = &Permission{
		"use_channel_mentions",
		"authentication.permissions.use_channel_mentions.name",
		"authentication.permissions.use_channel_mentions.description",
		PermissionScopeChannel,
	}
	PermissionUseGroupMentions = &Permission{
		"use_group_mentions",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeChannel,
	}
	PermissionReadOtherUsersTeams = &Permission{
		"read_other_users_teams",
		"authentication.permissions.read_other_users_teams.name",
		"authentication.permissions.read_other_users_teams.description",
		PermissionScopeSystem,
	}
	PermissionEditBrand = &Permission{
		"edit_brand",
		"authentication.permissions.edit_brand.name",
		"authentication.permissions.edit_brand.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleReadAbout = &Permission{
		"sysconsole_read_about",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleWriteAbout = &Permission{
		"sysconsole_write_about",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAboutEditionAndLicense = &Permission{
		"sysconsole_read_about_edition_and_license",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAboutEditionAndLicense = &Permission{
		"sysconsole_write_about_edition_and_license",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadBilling = &Permission{
		"sysconsole_read_billing",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteBilling = &Permission{
		"sysconsole_write_billing",
		"",
		"",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleReadReporting = &Permission{
		"sysconsole_read_reporting",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleWriteReporting = &Permission{
		"sysconsole_write_reporting",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadReportingSiteStatistics = &Permission{
		"sysconsole_read_reporting_site_statistics",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteReportingSiteStatistics = &Permission{
		"sysconsole_write_reporting_site_statistics",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadReportingTeamStatistics = &Permission{
		"sysconsole_read_reporting_team_statistics",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteReportingTeamStatistics = &Permission{
		"sysconsole_write_reporting_team_statistics",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadReportingServerLogs = &Permission{
		"sysconsole_read_reporting_server_logs",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteReportingServerLogs = &Permission{
		"sysconsole_write_reporting_server_logs",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadUserManagementUsers = &Permission{
		"sysconsole_read_user_management_users",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteUserManagementUsers = &Permission{
		"sysconsole_write_user_management_users",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadUserManagementGroups = &Permission{
		"sysconsole_read_user_management_groups",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteUserManagementGroups = &Permission{
		"sysconsole_write_user_management_groups",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadUserManagementTeams = &Permission{
		"sysconsole_read_user_management_teams",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteUserManagementTeams = &Permission{
		"sysconsole_write_user_management_teams",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadUserManagementChannels = &Permission{
		"sysconsole_read_user_management_channels",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteUserManagementChannels = &Permission{
		"sysconsole_write_user_management_channels",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadUserManagementPermissions = &Permission{
		"sysconsole_read_user_management_permissions",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteUserManagementPermissions = &Permission{
		"sysconsole_write_user_management_permissions",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadUserManagementSystemRoles = &Permission{
		"sysconsole_read_user_management_system_roles",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteUserManagementSystemRoles = &Permission{
		"sysconsole_write_user_management_system_roles",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleReadEnvironment = &Permission{
		"sysconsole_read_environment",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleWriteEnvironment = &Permission{
		"sysconsole_write_environment",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentWebServer = &Permission{
		"sysconsole_read_environment_web_server",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentWebServer = &Permission{
		"sysconsole_write_environment_web_server",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentDatabase = &Permission{
		"sysconsole_read_environment_database",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentDatabase = &Permission{
		"sysconsole_write_environment_database",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentElasticsearch = &Permission{
		"sysconsole_read_environment_elasticsearch",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentElasticsearch = &Permission{
		"sysconsole_write_environment_elasticsearch",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentFileStorage = &Permission{
		"sysconsole_read_environment_file_storage",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentFileStorage = &Permission{
		"sysconsole_write_environment_file_storage",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentImageProxy = &Permission{
		"sysconsole_read_environment_image_proxy",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentImageProxy = &Permission{
		"sysconsole_write_environment_image_proxy",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentSMTP = &Permission{
		"sysconsole_read_environment_smtp",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentSMTP = &Permission{
		"sysconsole_write_environment_smtp",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentPushNotificationServer = &Permission{
		"sysconsole_read_environment_push_notification_server",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentPushNotificationServer = &Permission{
		"sysconsole_write_environment_push_notification_server",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentHighAvailability = &Permission{
		"sysconsole_read_environment_high_availability",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentHighAvailability = &Permission{
		"sysconsole_write_environment_high_availability",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentRateLimiting = &Permission{
		"sysconsole_read_environment_rate_limiting",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentRateLimiting = &Permission{
		"sysconsole_write_environment_rate_limiting",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentLogging = &Permission{
		"sysconsole_read_environment_logging",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentLogging = &Permission{
		"sysconsole_write_environment_logging",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentSessionLengths = &Permission{
		"sysconsole_read_environment_session_lengths",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentSessionLengths = &Permission{
		"sysconsole_write_environment_session_lengths",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentPerformanceMonitoring = &Permission{
		"sysconsole_read_environment_performance_monitoring",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentPerformanceMonitoring = &Permission{
		"sysconsole_write_environment_performance_monitoring",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadEnvironmentDeveloper = &Permission{
		"sysconsole_read_environment_developer",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteEnvironmentDeveloper = &Permission{
		"sysconsole_write_environment_developer",
		"",
		"",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleReadSite = &Permission{
		"sysconsole_read_site",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleWriteSite = &Permission{
		"sysconsole_write_site",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}

	PermissionSysconsoleReadSiteCustomization = &Permission{
		"sysconsole_read_site_customization",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSiteCustomization = &Permission{
		"sysconsole_write_site_customization",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSiteLocalization = &Permission{
		"sysconsole_read_site_localization",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSiteLocalization = &Permission{
		"sysconsole_write_site_localization",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSiteUsersAndTeams = &Permission{
		"sysconsole_read_site_users_and_teams",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSiteUsersAndTeams = &Permission{
		"sysconsole_write_site_users_and_teams",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSiteNotifications = &Permission{
		"sysconsole_read_site_notifications",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSiteNotifications = &Permission{
		"sysconsole_write_site_notifications",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSiteAnnouncementBanner = &Permission{
		"sysconsole_read_site_announcement_banner",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSiteAnnouncementBanner = &Permission{
		"sysconsole_write_site_announcement_banner",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSiteEmoji = &Permission{
		"sysconsole_read_site_emoji",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSiteEmoji = &Permission{
		"sysconsole_write_site_emoji",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSitePosts = &Permission{
		"sysconsole_read_site_posts",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSitePosts = &Permission{
		"sysconsole_write_site_posts",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSiteFileSharingAndDownloads = &Permission{
		"sysconsole_read_site_file_sharing_and_downloads",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSiteFileSharingAndDownloads = &Permission{
		"sysconsole_write_site_file_sharing_and_downloads",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSitePublicLinks = &Permission{
		"sysconsole_read_site_public_links",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSitePublicLinks = &Permission{
		"sysconsole_write_site_public_links",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadSiteNotices = &Permission{
		"sysconsole_read_site_notices",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteSiteNotices = &Permission{
		"sysconsole_write_site_notices",
		"",
		"",
		PermissionScopeSystem,
	}

	// Deprecated
	PermissionSysconsoleReadAuthentication = &Permission{
		"sysconsole_read_authentication",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// Deprecated
	PermissionSysconsoleWriteAuthentication = &Permission{
		"sysconsole_write_authentication",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAuthenticationSignup = &Permission{
		"sysconsole_read_authentication_signup",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAuthenticationSignup = &Permission{
		"sysconsole_write_authentication_signup",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAuthenticationEmail = &Permission{
		"sysconsole_read_authentication_email",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAuthenticationEmail = &Permission{
		"sysconsole_write_authentication_email",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAuthenticationPassword = &Permission{
		"sysconsole_read_authentication_password",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAuthenticationPassword = &Permission{
		"sysconsole_write_authentication_password",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAuthenticationMfa = &Permission{
		"sysconsole_read_authentication_mfa",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAuthenticationMfa = &Permission{
		"sysconsole_write_authentication_mfa",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAuthenticationLdap = &Permission{
		"sysconsole_read_authentication_ldap",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAuthenticationLdap = &Permission{
		"sysconsole_write_authentication_ldap",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAuthenticationSaml = &Permission{
		"sysconsole_read_authentication_saml",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAuthenticationSaml = &Permission{
		"sysconsole_write_authentication_saml",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAuthenticationOpenid = &Permission{
		"sysconsole_read_authentication_openid",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAuthenticationOpenid = &Permission{
		"sysconsole_write_authentication_openid",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadAuthenticationGuestAccess = &Permission{
		"sysconsole_read_authentication_guest_access",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteAuthenticationGuestAccess = &Permission{
		"sysconsole_write_authentication_guest_access",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadPlugins = &Permission{
		"sysconsole_read_plugins",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWritePlugins = &Permission{
		"sysconsole_write_plugins",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleReadIntegrations = &Permission{
		"sysconsole_read_integrations",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleWriteIntegrations = &Permission{
		"sysconsole_write_integrations",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadIntegrationsIntegrationManagement = &Permission{
		"sysconsole_read_integrations_integration_management",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteIntegrationsIntegrationManagement = &Permission{
		"sysconsole_write_integrations_integration_management",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadIntegrationsBotAccounts = &Permission{
		"sysconsole_read_integrations_bot_accounts",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteIntegrationsBotAccounts = &Permission{
		"sysconsole_write_integrations_bot_accounts",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadIntegrationsGif = &Permission{
		"sysconsole_read_integrations_gif",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteIntegrationsGif = &Permission{
		"sysconsole_write_integrations_gif",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadIntegrationsCors = &Permission{
		"sysconsole_read_integrations_cors",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteIntegrationsCors = &Permission{
		"sysconsole_write_integrations_cors",
		"",
		"",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleReadCompliance = &Permission{
		"sysconsole_read_compliance",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleWriteCompliance = &Permission{
		"sysconsole_write_compliance",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadComplianceDataRetentionPolicy = &Permission{
		"sysconsole_read_compliance_data_retention_policy",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteComplianceDataRetentionPolicy = &Permission{
		"sysconsole_write_compliance_data_retention_policy",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadComplianceComplianceExport = &Permission{
		"sysconsole_read_compliance_compliance_export",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteComplianceComplianceExport = &Permission{
		"sysconsole_write_compliance_compliance_export",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadComplianceComplianceMonitoring = &Permission{
		"sysconsole_read_compliance_compliance_monitoring",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteComplianceComplianceMonitoring = &Permission{
		"sysconsole_write_compliance_compliance_monitoring",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadComplianceCustomTermsOfService = &Permission{
		"sysconsole_read_compliance_custom_terms_of_service",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteComplianceCustomTermsOfService = &Permission{
		"sysconsole_write_compliance_custom_terms_of_service",
		"",
		"",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleReadExperimental = &Permission{
		"sysconsole_read_experimental",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	// DEPRECATED
	PermissionSysconsoleWriteExperimental = &Permission{
		"sysconsole_write_experimental",
		"authentication.permissions.use_group_mentions.name",
		"authentication.permissions.use_group_mentions.description",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadExperimentalFeatures = &Permission{
		"sysconsole_read_experimental_features",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteExperimentalFeatures = &Permission{
		"sysconsole_write_experimental_features",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadExperimentalFeatureFlags = &Permission{
		"sysconsole_read_experimental_feature_flags",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteExperimentalFeatureFlags = &Permission{
		"sysconsole_write_experimental_feature_flags",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleReadExperimentalBleve = &Permission{
		"sysconsole_read_experimental_bleve",
		"",
		"",
		PermissionScopeSystem,
	}
	PermissionSysconsoleWriteExperimentalBleve = &Permission{
		"sysconsole_write_experimental_bleve",
		"",
		"",
		PermissionScopeSystem,
	}

	SysconsoleReadPermissions = []*Permission{
		PermissionSysconsoleReadAboutEditionAndLicense,
		PermissionSysconsoleReadBilling,
		PermissionSysconsoleReadReportingSiteStatistics,
		PermissionSysconsoleReadReportingTeamStatistics,
		PermissionSysconsoleReadReportingServerLogs,
		PermissionSysconsoleReadUserManagementUsers,
		PermissionSysconsoleReadUserManagementGroups,
		PermissionSysconsoleReadUserManagementTeams,
		PermissionSysconsoleReadUserManagementChannels,
		PermissionSysconsoleReadUserManagementPermissions,
		PermissionSysconsoleReadUserManagementSystemRoles,
		PermissionSysconsoleReadEnvironmentWebServer,
		PermissionSysconsoleReadEnvironmentDatabase,
		PermissionSysconsoleReadEnvironmentElasticsearch,
		PermissionSysconsoleReadEnvironmentFileStorage,
		PermissionSysconsoleReadEnvironmentImageProxy,
		PermissionSysconsoleReadEnvironmentSMTP,
		PermissionSysconsoleReadEnvironmentPushNotificationServer,
		PermissionSysconsoleReadEnvironmentHighAvailability,
		PermissionSysconsoleReadEnvironmentRateLimiting,
		PermissionSysconsoleReadEnvironmentLogging,
		PermissionSysconsoleReadEnvironmentSessionLengths,
		PermissionSysconsoleReadEnvironmentPerformanceMonitoring,
		PermissionSysconsoleReadEnvironmentDeveloper,
		PermissionSysconsoleReadSiteCustomization,
		PermissionSysconsoleReadSiteLocalization,
		PermissionSysconsoleReadSiteUsersAndTeams,
		PermissionSysconsoleReadSiteNotifications,
		PermissionSysconsoleReadSiteAnnouncementBanner,
		PermissionSysconsoleReadSiteEmoji,
		PermissionSysconsoleReadSitePosts,
		PermissionSysconsoleReadSiteFileSharingAndDownloads,
		PermissionSysconsoleReadSitePublicLinks,
		PermissionSysconsoleReadSiteNotices,
		PermissionSysconsoleReadAuthenticationSignup,
		PermissionSysconsoleReadAuthenticationEmail,
		PermissionSysconsoleReadAuthenticationPassword,
		PermissionSysconsoleReadAuthenticationMfa,
		PermissionSysconsoleReadAuthenticationLdap,
		PermissionSysconsoleReadAuthenticationSaml,
		PermissionSysconsoleReadAuthenticationOpenid,
		PermissionSysconsoleReadAuthenticationGuestAccess,
		PermissionSysconsoleReadPlugins,
		PermissionSysconsoleReadIntegrationsIntegrationManagement,
		PermissionSysconsoleReadIntegrationsBotAccounts,
		PermissionSysconsoleReadIntegrationsGif,
		PermissionSysconsoleReadIntegrationsCors,
		PermissionSysconsoleReadComplianceDataRetentionPolicy,
		PermissionSysconsoleReadComplianceComplianceExport,
		PermissionSysconsoleReadComplianceComplianceMonitoring,
		PermissionSysconsoleReadComplianceCustomTermsOfService,
		PermissionSysconsoleReadExperimentalFeatures,
		PermissionSysconsoleReadExperimentalFeatureFlags,
		PermissionSysconsoleReadExperimentalBleve,
	}

	SysconsoleWritePermissions = []*Permission{
		PermissionSysconsoleWriteAboutEditionAndLicense,
		PermissionSysconsoleWriteBilling,
		PermissionSysconsoleWriteReportingSiteStatistics,
		PermissionSysconsoleWriteReportingTeamStatistics,
		PermissionSysconsoleWriteReportingServerLogs,
		PermissionSysconsoleWriteUserManagementUsers,
		PermissionSysconsoleWriteUserManagementGroups,
		PermissionSysconsoleWriteUserManagementTeams,
		PermissionSysconsoleWriteUserManagementChannels,
		PermissionSysconsoleWriteUserManagementPermissions,
		PermissionSysconsoleWriteUserManagementSystemRoles,
		PermissionSysconsoleWriteEnvironmentWebServer,
		PermissionSysconsoleWriteEnvironmentDatabase,
		PermissionSysconsoleWriteEnvironmentElasticsearch,
		PermissionSysconsoleWriteEnvironmentFileStorage,
		PermissionSysconsoleWriteEnvironmentImageProxy,
		PermissionSysconsoleWriteEnvironmentSMTP,
		PermissionSysconsoleWriteEnvironmentPushNotificationServer,
		PermissionSysconsoleWriteEnvironmentHighAvailability,
		PermissionSysconsoleWriteEnvironmentRateLimiting,
		PermissionSysconsoleWriteEnvironmentLogging,
		PermissionSysconsoleWriteEnvironmentSessionLengths,
		PermissionSysconsoleWriteEnvironmentPerformanceMonitoring,
		PermissionSysconsoleWriteEnvironmentDeveloper,
		PermissionSysconsoleWriteSiteCustomization,
		PermissionSysconsoleWriteSiteLocalization,
		PermissionSysconsoleWriteSiteUsersAndTeams,
		PermissionSysconsoleWriteSiteNotifications,
		PermissionSysconsoleWriteSiteAnnouncementBanner,
		PermissionSysconsoleWriteSiteEmoji,
		PermissionSysconsoleWriteSitePosts,
		PermissionSysconsoleWriteSiteFileSharingAndDownloads,
		PermissionSysconsoleWriteSitePublicLinks,
		PermissionSysconsoleWriteSiteNotices,
		PermissionSysconsoleWriteAuthenticationSignup,
		PermissionSysconsoleWriteAuthenticationEmail,
		PermissionSysconsoleWriteAuthenticationPassword,
		PermissionSysconsoleWriteAuthenticationMfa,
		PermissionSysconsoleWriteAuthenticationLdap,
		PermissionSysconsoleWriteAuthenticationSaml,
		PermissionSysconsoleWriteAuthenticationOpenid,
		PermissionSysconsoleWriteAuthenticationGuestAccess,
		PermissionSysconsoleWritePlugins,
		PermissionSysconsoleWriteIntegrationsIntegrationManagement,
		PermissionSysconsoleWriteIntegrationsBotAccounts,
		PermissionSysconsoleWriteIntegrationsGif,
		PermissionSysconsoleWriteIntegrationsCors,
		PermissionSysconsoleWriteComplianceDataRetentionPolicy,
		PermissionSysconsoleWriteComplianceComplianceExport,
		PermissionSysconsoleWriteComplianceComplianceMonitoring,
		PermissionSysconsoleWriteComplianceCustomTermsOfService,
		PermissionSysconsoleWriteExperimentalFeatures,
		PermissionSysconsoleWriteExperimentalFeatureFlags,
		PermissionSysconsoleWriteExperimentalBleve,
	}

	SystemScopedPermissionsMinusSysconsole := []*Permission{
		PermissionAssignSystemAdminRole,
		PermissionManageRoles,
		PermissionManageSystem,
		PermissionCreateDirectChannel,
		PermissionCreateGroupChannel,
		PermissionListPublicTeams,
		PermissionJoinPublicTeams,
		PermissionListPrivateTeams,
		PermissionJoinPrivateTeams,
		PermissionEditOtherUsers,
		PermissionReadOtherUsersTeams,
		PermissionGetPublicLink,
		PermissionManageOAuth,
		PermissionManageSystemWideOAuth,
		PermissionCreateTeam,
		PermissionListUsersWithoutTeam,
		PermissionCreateUserAccessToken,
		PermissionReadUserAccessToken,
		PermissionRevokeUserAccessToken,
		PermissionCreateBot,
		PermissionAssignBot,
		PermissionReadBots,
		PermissionReadOthersBots,
		PermissionManageBots,
		PermissionManageOthersBots,
		PermissionReadJobs,
		PermissionManageJobs,
		PermissionPromoteGuest,
		PermissionDemoteToGuest,
		PermissionEditBrand,
		PermissionManageSharedChannels,
		PermissionManageSecureConnections,
		PermissionDownloadComplianceExportResult,
		PermissionCreateDataRetentionJob,
		PermissionReadDataRetentionJob,
		PermissionCreateComplianceExportJob,
		PermissionReadComplianceExportJob,
		PermissionReadAudits,
		PermissionTestSiteURL,
		PermissionTestElasticsearch,
		PermissionTestS3,
		PermissionReloadConfig,
		PermissionInvalidateCaches,
		PermissionRecycleDatabaseConnections,
		PermissionPurgeElasticsearchIndexes,
		PermissionTestEmail,
		PermissionCreateElasticsearchPostIndexingJob,
		PermissionCreateElasticsearchPostAggregationJob,
		PermissionReadElasticsearchPostIndexingJob,
		PermissionReadElasticsearchPostAggregationJob,
		PermissionPurgeBleveIndexes,
		PermissionCreatePostBleveIndexesJob,
		PermissionCreateLdapSyncJob,
		PermissionReadLdapSyncJob,
		PermissionTestLdap,
		PermissionInvalidateEmailInvite,
		PermissionGetSamlMetadataFromIDp,
		PermissionAddSamlPublicCert,
		PermissionAddSamlPrivateCert,
		PermissionAddSamlIDpCert,
		PermissionRemoveSamlPublicCert,
		PermissionRemoveSamlPrivateCert,
		PermissionRemoveSamlIDpCert,
		PermissionGetSamlCertStatus,
		PermissionAddLdapPublicCert,
		PermissionAddLdapPrivateCert,
		PermissionRemoveLdapPublicCert,
		PermissionRemoveLdapPrivateCert,
		PermissionGetAnalytics,
		PermissionGetLogs,
		PermissionReadLicenseInformation,
		PermissionManageLicenseInformation,
	}

	TeamScopedPermissions := []*Permission{
		PermissionInviteUser,
		PermissionAddUserToTeam,
		PermissionManageSlashCommands,
		PermissionManageOthersSlashCommands,
		PermissionCreatePublicChannel,
		PermissionCreatePrivateChannel,
		PermissionManageTeamRoles,
		PermissionListTeamChannels,
		PermissionJoinPublicChannels,
		PermissionReadPublicChannel,
		PermissionManageIncomingWebhooks,
		PermissionManageOutgoingWebhooks,
		PermissionManageOthersIncomingWebhooks,
		PermissionManageOthersOutgoingWebhooks,
		PermissionCreateEmojis,
		PermissionDeleteEmojis,
		PermissionDeleteOthersEmojis,
		PermissionRemoveUserFromTeam,
		PermissionManageTeam,
		PermissionImportTeam,
		PermissionViewTeam,
		PermissionViewMembers,
		PermissionInviteGuest,
	}

	ChannelScopedPermissions := []*Permission{
		PermissionUseSlashCommands,
		PermissionManagePublicChannelMembers,
		PermissionManagePrivateChannelMembers,
		PermissionManageChannelRoles,
		PermissionManagePublicChannelProperties,
		PermissionManagePrivateChannelProperties,
		PermissionConvertPublicChannelToPrivate,
		PermissionConvertPrivateChannelToPublic,
		PermissionDeletePublicChannel,
		PermissionDeletePrivateChannel,
		PermissionReadChannel,
		PermissionReadPublicChannelGroups,
		PermissionReadPrivateChannelGroups,
		PermissionAddReaction,
		PermissionRemoveReaction,
		PermissionRemoveOthersReactions,
		PermissionUploadFile,
		PermissionCreatePost,
		PermissionCreatePostPublic,
		PermissionCreatePostEphemeral,
		PermissionEditPost,
		PermissionEditOthersPosts,
		PermissionDeletePost,
		PermissionDeleteOthersPosts,
		PermissionUseChannelMentions,
		PermissionUseGroupMentions,
	}

	DeprecatedPermissions = []*Permission{
		PermissionPermanentDeleteUser,
		PermissionManageWebhooks,
		PermissionManageOthersWebhooks,
		PermissionManageEmojis,
		PermissionManageOthersEmojis,
		PermissionSysconsoleReadAuthentication,
		PermissionSysconsoleWriteAuthentication,
		PermissionSysconsoleReadSite,
		PermissionSysconsoleWriteSite,
		PermissionSysconsoleReadEnvironment,
		PermissionSysconsoleWriteEnvironment,
		PermissionSysconsoleReadReporting,
		PermissionSysconsoleWriteReporting,
		PermissionSysconsoleReadAbout,
		PermissionSysconsoleWriteAbout,
		PermissionSysconsoleReadExperimental,
		PermissionSysconsoleWriteExperimental,
		PermissionSysconsoleReadIntegrations,
		PermissionSysconsoleWriteIntegrations,
		PermissionSysconsoleReadCompliance,
		PermissionSysconsoleWriteCompliance,
	}

	AllPermissions = []*Permission{}
	AllPermissions = append(AllPermissions, SystemScopedPermissionsMinusSysconsole...)
	AllPermissions = append(AllPermissions, TeamScopedPermissions...)
	AllPermissions = append(AllPermissions, ChannelScopedPermissions...)
	AllPermissions = append(AllPermissions, SysconsoleReadPermissions...)
	AllPermissions = append(AllPermissions, SysconsoleWritePermissions...)

	ChannelModeratedPermissions = []string{
		PermissionCreatePost.ID,
		"create_reactions",
		"manage_members",
		PermissionUseChannelMentions.ID,
	}

	ChannelModeratedPermissionsMap = map[string]string{
		PermissionCreatePost.ID:                  ChannelModeratedPermissions[0],
		PermissionAddReaction.ID:                 ChannelModeratedPermissions[1],
		PermissionRemoveReaction.ID:              ChannelModeratedPermissions[1],
		PermissionManagePublicChannelMembers.ID:  ChannelModeratedPermissions[2],
		PermissionManagePrivateChannelMembers.ID: ChannelModeratedPermissions[2],
		PermissionUseChannelMentions.ID:          ChannelModeratedPermissions[3],
	}
}

func init() {
	initializePermissions()
}
