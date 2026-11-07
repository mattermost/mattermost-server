// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// Access Control & Security
const (
	AuditEventApplyIPFilters            = "applyIPFilters"            // apply IP address filtering
	AuditEventAssignAccessPolicy        = "assignAccessPolicy"        // assign access control policy to channels and/or teams
	AuditEventCreateAccessControlPolicy = "createAccessControlPolicy" // create access control policy
	AuditEventDeleteAccessControlPolicy = "deleteAccessControlPolicy" // delete access control policy
	AuditEventUnassignAccessPolicy      = "unassignAccessPolicy"      // remove access control policy from channels and/or teams
	AuditEventUpdateActiveStatus        = "updateActiveStatus"        // update active/inactive status of access control policy
	AuditEventSetActiveStatus           = "setActiveStatus"           // set active/inactive status of multiple access control policies

	AuditEventSavePluginAccessControlPolicy   = "savePluginAccessControlPolicy"   // create/update plugin-owned access control policy (activation implicit)
	AuditEventDeletePluginAccessControlPolicy = "deletePluginAccessControlPolicy" // delete plugin-owned access control policy

	AuditEventCreateTeamAccessPolicy   = "createTeamAccessPolicy"   // create team-scoped access control policy
	AuditEventUpdateTeamAccessPolicy   = "updateTeamAccessPolicy"   // update team-scoped access control policy
	AuditEventDeleteTeamAccessPolicy   = "deleteTeamAccessPolicy"   // delete team-scoped access control policy
	AuditEventAssignTeamAccessPolicy   = "assignTeamAccessPolicy"   // assign channels to team-scoped access control policy
	AuditEventUnassignTeamAccessPolicy = "unassignTeamAccessPolicy" // remove channels from team-scoped access control policy
	AuditEventTriggerTeamPolicySync    = "triggerTeamPolicySync"    // trigger sync for team-scoped access control policies

	AuditEventTeamMembershipAdded        = "teamMembershipAdded"        // user auto-added to a team by its membership policy
	AuditEventTeamMembershipRemoved      = "teamMembershipRemoved"      // user removed from a team by its membership policy
	AuditEventTeamCascadedChannelRemoval = "teamCascadedChannelRemoval" // channel membership dropped as a cascade of a policy-driven team removal

	AuditEventChannelMembershipAdded   = "channelMembershipAdded"   // user auto-added to a channel by its membership policy
	AuditEventChannelMembershipRemoved = "channelMembershipRemoved" // user removed from a channel by its membership policy
)

// Audit & Certificates
const (
	AuditEventAddAuditLogCertificate    = "addAuditLogCertificate"    // add certificate for secure audit log transmission
	AuditEventGetAudits                 = "getAudits"                 // get audit log entries
	AuditEventGetUserAudits             = "getUserAudits"             // get audit log entries for specific user
	AuditEventRemoveAuditLogCertificate = "removeAuditLogCertificate" // remove certificate used for audit log transmission
)

// Bots
const (
	AuditEventAssignBot        = "assignBot"        // assign bot to user
	AuditEventConvertBotToUser = "convertBotToUser" // convert bot account to regular user account
	AuditEventConvertUserToBot = "convertUserToBot" // convert regular user account to bot account
	AuditEventCreateBot        = "createBot"        // create bot account
	AuditEventPatchBot         = "patchBot"         // update bot properties
	AuditEventUpdateBotActive  = "updateBotActive"  // enable or disable bot account
)

// Branding
const (
	AuditEventDeleteBrandImage = "deleteBrandImage" // delete brand image
	AuditEventUploadBrandImage = "uploadBrandImage" // upload brand image
)

// Channel Bookmarks
const (
	AuditEventCreateChannelBookmark          = "createChannelBookmark"          // create bookmark in channels
	AuditEventDeleteChannelBookmark          = "deleteChannelBookmark"          // delete bookmark
	AuditEventUpdateChannelBookmark          = "updateChannelBookmark"          // update bookmark
	AuditEventUpdateChannelBookmarkSortOrder = "updateChannelBookmarkSortOrder" // update display order of bookmarks
	AuditEventListChannelBookmarksForChannel = "listChannelBookmarksForChannel" // list bookmarks for channel
)

// Boards
const (
	AuditEventCreateBoard = "createBoard" // create board channel
)

// Views
const (
	AuditEventCreateView          = "createView"          // create view in channel
	AuditEventGetView             = "getView"             // get view by ID
	AuditEventUpdateView          = "updateView"          // update view
	AuditEventDeleteView          = "deleteView"          // delete view
	AuditEventListViewsForChannel = "listViewsForChannel" // list views for channel
	AuditEventUpdateViewSortOrder = "updateViewSortOrder" // update view sort order
	AuditEventGetPostsForView     = "getPostsForView"     // get posts for view
)

// Channel Categories
const (
	AuditEventCreateCategoryForTeamForUser      = "createCategoryForTeamForUser"      // create channel category for user
	AuditEventDeleteCategoryForTeamForUser      = "deleteCategoryForTeamForUser"      // delete channel category
	AuditEventUpdateCategoriesForTeamForUser    = "updateCategoriesForTeamForUser"    // update multiple channel categories
	AuditEventUpdateCategoryForTeamForUser      = "updateCategoryForTeamForUser"      // update single channel category
	AuditEventUpdateCategoryOrderForTeamForUser = "updateCategoryOrderForTeamForUser" // update display order of the categories
)

// Channels
const (
	AuditEventAddChannelMember                   = "addChannelMember"                   // add member to channel
	AuditEventConvertGroupMessageToChannel       = "convertGroupMessageToChannel"       // convert group message to private channel
	AuditEventCreateChannel                      = "createChannel"                      // create public or private channel
	AuditEventCreateChannelJoinRequest           = "createChannelJoinRequest"           // request to join a discoverable private channel
	AuditEventUpdateChannelJoinRequest           = "updateChannelJoinRequest"           // approve or deny a channel join request
	AuditEventWithdrawChannelJoinRequest         = "withdrawChannelJoinRequest"         // requester cancels their channel join request
	AuditEventCreateDirectChannel                = "createDirectChannel"                // create direct message channel between two users
	AuditEventCreateGroupChannel                 = "createGroupChannel"                 // create group message channel with multiple users
	AuditEventDeleteChannel                      = "deleteChannel"                      // delete channel
	AuditEventGetPinnedPosts                     = "getPinnedPosts"                     // get pinned posts
	AuditEventLocalAddChannelMember              = "localAddChannelMember"              // add channel member locally
	AuditEventLocalCreateChannel                 = "localCreateChannel"                 // create channel locally
	AuditEventLocalDeleteChannel                 = "localDeleteChannel"                 // delete channel locally
	AuditEventLocalMoveChannel                   = "localMoveChannel"                   // move channel locally
	AuditEventLocalPatchChannel                  = "localPatchChannel"                  // patch channel locally
	AuditEventLocalRemoveChannelMember           = "localRemoveChannelMember"           // remove channel member locally
	AuditEventLocalRestoreChannel                = "localRestoreChannel"                // restore channel locally
	AuditEventLocalUpdateChannelPrivacy          = "localUpdateChannelPrivacy"          // update channel privacy locally
	AuditEventMoveChannel                        = "moveChannel"                        // move channel to different team
	AuditEventPatchChannel                       = "patchChannel"                       // update channel properties
	AuditEventPatchChannelModerations            = "patchChannelModerations"            // update channel moderation settings
	AuditEventRemoveChannelMember                = "removeChannelMember"                // remove member from channel
	AuditEventRestoreChannel                     = "restoreChannel"                     // restore previously deleted channel
	AuditEventSetChannelMembers                  = "setChannelMembers"                  // bulk set (replace) channel memberships
	AuditEventUpdateChannel                      = "updateChannel"                      // update channel properties
	AuditEventUpdateChannelMemberNotifyProps     = "updateChannelMemberNotifyProps"     // update notification preferences
	AuditEventUpdateChannelMemberAutotranslation = "updateChannelMemberAutotranslation" // update autotranslation setting
	AuditEventUpdateChannelMemberRoles           = "updateChannelMemberRoles"           // update roles and permissions
	AuditEventUpdateChannelMemberSchemeRoles     = "updateChannelMemberSchemeRoles"     // update scheme-based roles
	AuditEventUpdateChannelPrivacy               = "updateChannelPrivacy"               // change channel privacy settings
	AuditEventUpdateChannelScheme                = "updateChannelScheme"                // update permission scheme applied to channel
)

// Commands
const (
	AuditEventCreateCommand      = "createCommand"      // create slash command
	AuditEventDeleteCommand      = "deleteCommand"      // delete command
	AuditEventExecuteCommand     = "executeCommand"     // execute command
	AuditEventLocalCreateCommand = "localCreateCommand" // create command locally
	AuditEventMoveCommand        = "moveCommand"        // move command to another team
	AuditEventRegenCommandToken  = "regenCommandToken"  // regenerate authentication token for command
	AuditEventUpdateCommand      = "updateCommand"      // update command
)

// Compliance
const (
	AuditEventCreateComplianceReport   = "createComplianceReport"   // create compliance report
	AuditEventDownloadComplianceReport = "downloadComplianceReport" // download compliance report
	AuditEventGetComplianceReport      = "getComplianceReport"      // get specific compliance report
	AuditEventGetComplianceReports     = "getComplianceReports"     // get all compliance reports
)

// Configuration
const (
	AuditEventAutoSelectPushNotificationServer = "autoSelectPushNotificationServer" // automatically switch push notification server based on license entitlement
	AuditEventConfigReload                     = "configReload"                     // reload server configuration
	AuditEventGetConfig                        = "getConfig"                        // get current server configuration
	AuditEventLocalGetClientConfig             = "localGetClientConfig"             // get client configuration locally
	AuditEventLocalGetConfig                   = "localGetConfig"                   // get server configuration locally
	AuditEventLocalPatchConfig                 = "localPatchConfig"                 // update server configuration locally
	AuditEventLocalUpdateConfig                = "localUpdateConfig"                // update server configuration locally
	AuditEventMigrateConfig                    = "migrateConfig"                    // migrate configs with file values from one store to another
	AuditEventPatchConfig                      = "patchConfig"                      // update server configuration
	AuditEventUpdateConfig                     = "updateConfig"                     // update server configuration
)

// Custom Profile Attributes
const (
	AuditEventCreateCPAField = "createCPAField" // create custom profile attribute
	AuditEventDeleteCPAField = "deleteCPAField" // delete custom profile attribute
	AuditEventPatchCPAField  = "patchCPAField"  // update custom profile attribute field
	AuditEventPatchCPAValues = "patchCPAValues" // update custom profile attribute values
	// AuditEventCPAValueChange is emitted from the common app-layer value write
	// path for every effective CPA value change, regardless of caller (session,
	// local admin, plugin owner, or LDAP/SAML sync). It is the single place all
	// owners converge, capturing the caller ID and acting-as scope.
	AuditEventCPAValueChange = "cpaValueChange"
)

// Property Fields
const (
	AuditEventCreatePropertyField = "createPropertyField" // create property field
	AuditEventDeletePropertyField = "deletePropertyField" // delete property field
	AuditEventGetPropertyFields   = "getPropertyFields"   // list property fields
	AuditEventPatchPropertyField  = "patchPropertyField"  // update property field

	AuditEventSearchGlobalAttributeListing = "searchGlobalAttributeListing" // search the Global Attributes listing
)

// Property Field Options
const (
	AuditEventCreatePropertyFieldOptions = "createPropertyFieldOptions" // add options to a property field
	AuditEventDeletePropertyFieldOptions = "deletePropertyFieldOptions" // delete options of a property field
	AuditEventGetPropertyFieldOptions    = "getPropertyFieldOptions"    // list the options of a property field
	AuditEventPatchPropertyFieldOptions  = "patchPropertyFieldOptions"  // update options of a property field
)

// Property Values
const (
	AuditEventGetPropertyValues   = "getPropertyValues"   // get property values for target
	AuditEventPatchPropertyValues = "patchPropertyValues" // update property values for target
)

// Channel Attribute Compliance
const (
	AuditEventGetPropertyFieldMissingValues      = "getPropertyFieldMissingValues"      // list channels missing a value for a channel attribute
	AuditEventGetPropertyFieldComplianceSummary  = "getPropertyFieldComplianceSummary"  // get channel attribute compliance summary
	AuditEventGetPropertyFieldsComplianceSummary = "getPropertyFieldsComplianceSummary" // get compliance summary for a not-yet-created channel attribute
	AuditEventNotifyPropertyFieldMissingValues   = "notifyPropertyFieldMissingValues"   // notify channel admins of a missing channel attribute value
)

// Data Retention Policies
const (
	AuditEventAddChannelsToPolicy      = "addChannelsToPolicy"      // add channels to data retention policy
	AuditEventAddTeamsToPolicy         = "addTeamsToPolicy"         // add teams to data retention policy
	AuditEventCreatePolicy             = "createPolicy"             // create data retention policy
	AuditEventDeletePolicy             = "deletePolicy"             // delete data retention policy
	AuditEventPatchPolicy              = "patchPolicy"              // update data retention policy
	AuditEventRemoveChannelsFromPolicy = "removeChannelsFromPolicy" // remove channels from data retention policy
	AuditEventRemoveTeamsFromPolicy    = "removeTeamsFromPolicy"    // remove teams from data retention policy
)

// Ephemeral Mode
const (
	AuditEventAutoCacheCleanupRun = "autoCacheCleanupRun" // automatic cache cleanup run
	AuditEventOfflinePurge        = "offlinePurge"        // offline purge
	AuditEventSessionWipe         = "sessionWipe"         // session wipe
)

// Emojis
const (
	AuditEventCreateEmoji = "createEmoji" // create emoji
	AuditEventDeleteEmoji = "deleteEmoji" // delete emoji
)

// Exports
const (
	AuditEventBulkExport               = "bulkExport"               // bulk export data to a file
	AuditEventDeleteExport             = "deleteExport"             // delete exported file
	AuditEventGeneratePresignURLExport = "generatePresignURLExport" // generate presigned URL to download the exported file
	AuditEventScheduleExport           = "scheduleExport"           // schedule export job
)

// Files
const (
	AuditEventGetFile                   = "getFile"                   // get or download file
	AuditEventGetFileLink               = "getFileLink"               // generate link for file sharing
	AuditEventUploadFileMultipart       = "uploadFileMultipart"       // upload file using multipart form data
	AuditEventUploadFileMultipartLegacy = "uploadFileMultipartLegacy" // upload file using legacy multipart method
	AuditEventUploadFileSimple          = "uploadFileSimple"          // upload file using simple direct upload method
	AuditEventGetFileThumbnail          = "getFileThumbnail"          // get file thumbnail
	AuditEventGetFileInfosForPost       = "getFileInfosForPost"       // get file infos for post
	AuditEventGetFileInfo               = "getFileInfo"               // get file info
	AuditEventGetFilePreview            = "getFilePreview"            // get file preview
	AuditEventSearchFiles               = "searchFiles"               // search for files
)

// Groups
const (
	AuditEventAddGroupMembers         = "addGroupMembers"         // add members to group
	AuditEventAddUserToGroupSyncables = "addUserToGroupSyncables" // add user to group-synchronized teams and channels
	AuditEventCreateGroup             = "createGroup"             // create group
	AuditEventDeleteGroup             = "deleteGroup"             // delete group
	AuditEventDeleteGroupMembers      = "deleteGroupMembers"      // remove members from group
	AuditEventLinkGroupSyncable       = "linkGroupSyncable"       // link group to team or channel for synchronization
	AuditEventPatchGroup              = "patchGroup"              // update group
	AuditEventPatchGroupSyncable      = "patchGroupSyncable"      // update group synchronization settings
	AuditEventRestoreGroup            = "restoreGroup"            // restore previously deleted group
	AuditEventUnlinkGroupSyncable     = "unlinkGroupSyncable"     // unlink group from team or channel synchronization
)

// Health Dashboard
const (
	AuditEventMuteHealthFinding   = "muteHealthFinding"   // mute health dashboard finding
	AuditEventUnmuteHealthFinding = "unmuteHealthFinding" // unmute health dashboard finding
)

// Imports
const (
	AuditEventBulkImport   = "bulkImport"   // bulk import data from a file
	AuditEventDeleteImport = "deleteImport" // delete import file
)

// Jobs
const (
	AuditEventCancelJob       = "cancelJob"       // cancel a job
	AuditEventCreateJob       = "createJob"       // create a job
	AuditEventJobServer       = "jobServer"       // start job server
	AuditEventUpdateJobStatus = "updateJobStatus" // update status of a job
)

// LDAP
const (
	AuditEventAddLdapPrivateCertificate    = "addLdapPrivateCertificate"    // add private certificate for LDAP
	AuditEventAddLdapPublicCertificate     = "addLdapPublicCertificate"     // add public certificate for LDAP
	AuditEventIdMigrateLdap                = "idMigrateLdap"                // migrate user ID mapping to another attribute
	AuditEventLinkLdapGroup                = "linkLdapGroup"                // link LDAP group to Mattermost team or channel
	AuditEventRemoveLdapPrivateCertificate = "removeLdapPrivateCertificate" // remove private certificate for LDAP
	AuditEventRemoveLdapPublicCertificate  = "removeLdapPublicCertificate"  // remove public certificate for LDAP
	AuditEventSyncLdap                     = "syncLdap"                     // synchronize users and groups from LDAP
	AuditEventUnlinkLdapGroup              = "unlinkLdapGroup"              // unlink LDAP group from Mattermost team or channel
)

// Licensing
const (
	AuditEventAddLicense          = "addLicense"          // add license
	AuditEventLocalAddLicense     = "localAddLicense"     // add license locally
	AuditEventLocalRemoveLicense  = "localRemoveLicense"  // remove license locally
	AuditEventRemoveLicense       = "removeLicense"       // remove license
	AuditEventRequestTrialLicense = "requestTrialLicense" // request trial license
)

// OAuth
const (
	AuditEventAuthorizeOAuthApp                          = "authorizeOAuthApp"                          // authorize OAuth app
	AuditEventAuthorizeOAuthPage                         = "authorizeOAuthPage"                         // authorize OAuth page
	AuditEventCompleteOAuth                              = "completeOAuth"                              // complete OAuth authorization flow
	AuditEventCreateOAuthApp                             = "createOAuthApp"                             // create OAuth app
	AuditEventCreateOutgoingOauthConnection              = "createOutgoingOauthConnection"              // create outgoing OAuth connection
	AuditEventDeauthorizeOAuthApp                        = "deauthorizeOAuthApp"                        // revoke OAuth app authorization
	AuditEventDeleteOAuthApp                             = "deleteOAuthApp"                             // delete OAuth app
	AuditEventDeleteOutgoingOAuthConnection              = "deleteOutgoingOAuthConnection"              // delete outgoing OAuth connection
	AuditEventGetAccessToken                             = "getAccessToken"                             // get OAuth access token
	AuditEventLoginWithOAuth                             = "loginWithOAuth"                             // login using OAuth authentication provider
	AuditEventMobileLoginWithOAuth                       = "mobileLoginWithOAuth"                       // mobile application login using OAuth authentication provider
	AuditEventRegenerateOAuthAppSecret                   = "regenerateOAuthAppSecret"                   // regenerate secret key for OAuth app
	AuditEventRegisterOAuthClient                        = "registerOAuthClient"                        // register OAuth client via dynamic client registration (RFC 7591)
	AuditEventSignupWithOAuth                            = "signupWithOAuth"                            // create account using OAuth authentication provider
	AuditEventUpdateOAuthApp                             = "updateOAuthApp"                             // update OAuth app
	AuditEventUpdateOutgoingOAuthConnection              = "updateOutgoingOAuthConnection"              // update outgoing OAuth connection
	AuditEventValidateOutgoingOAuthConnectionCredentials = "validateOutgoingOAuthConnectionCredentials" // validate credentials for outgoing OAuth connection

)

// Plugins
const (
	AuditEventDisablePlugin                       = "disablePlugin"                       // disable installed plugin
	AuditEventEnablePlugin                        = "enablePlugin"                        // enable installed plugin
	AuditEventGetFirstAdminVisitMarketplaceStatus = "getFirstAdminVisitMarketplaceStatus" // get first admin visit status
	AuditEventInstallMarketplacePlugin            = "installMarketplacePlugin"            // install plugin from official marketplace
	AuditEventInstallPluginFromURL                = "installPluginFromURL"                // install plugin from external URL
	AuditEventLocalAddPluginPublicKey             = "localAddPluginPublicKey"             // add plugin signature public key locally
	AuditEventLocalDeletePluginPublicKey          = "localDeletePluginPublicKey"          // delete plugin signature public key locally
	AuditEventRemovePlugin                        = "removePlugin"                        // delete plugin
	AuditEventSetFirstAdminVisitMarketplaceStatus = "setFirstAdminVisitMarketplaceStatus" // set first admin visit status
	AuditEventUploadPlugin                        = "uploadPlugin"                        // upload plugin file to server for installation
)

// Posts
const (
	AuditEventCreateEphemeralPost                = "createEphemeralPost"                // create ephemeral post
	AuditEventCreatePost                         = "createPost"                         // create post
	AuditEventDeletePost                         = "deletePost"                         // delete post
	AuditEventGetEditHistoryForPost              = "getEditHistoryForPost"              // get edit history for post
	AuditEventGetFlaggedPosts                    = "getFlaggedPosts"                    // get flagged posts
	AuditEventGetPostsForChannel                 = "getPostsForChannel"                 // get posts for channel
	AuditEventGetPostsForChannelAroundLastUnread = "getPostsForChannelAroundLastUnread" // get posts for channel around last unread
	AuditEventGetPost                            = "getPost"                            // get post
	AuditEventGetPostThread                      = "getPostThread"                      // get post thread
	AuditEventGetPostsByIds                      = "getPostsByIds"                      // get posts by ids
	AuditEventGetThreadForUser                   = "getThreadForUser"                   // get thread for user
	AuditEventLocalDeletePost                    = "localDeletePost"                    // delete post locally
	AuditEventMoveThread                         = "moveThread"                         // move thread and replies to different channel
	AuditEventNotificationAck                    = "notificationAck"                    // notification ack
	AuditEventPatchPost                          = "patchPost"                          // update post meta properties
	AuditEventRestorePostVersion                 = "restorePostVersion"                 // restore post to previous version
	AuditEventSaveIsPinnedPost                   = "saveIsPinnedPost"                   // pin or unpin post
	AuditEventSearchPosts                        = "searchPosts"                        // search for posts
	AuditEventUpdatePost                         = "updatePost"                         // update post content
	AuditEventRevealPost                         = "revealPost"                         // reveal a post that was hidden due to burn on read
	AuditEventBurnPost                           = "burnPost"                           // burn a post that was hidden due to burn on read
	AuditEventWebsocketPost                      = "websocketPost"                      // post received via websocket
)

// Recaps
const (
	AuditEventCreateRecap        = "createRecap"        // create recap summarizing channel content
	AuditEventGetRecap           = "getRecap"           // view a single recap
	AuditEventGetRecaps          = "getRecaps"          // list user's recaps
	AuditEventMarkRecapAsRead    = "markRecapAsRead"    // mark recap as read
	AuditEventMarkRecapsAsViewed = "markRecapsAsViewed" // bulk mark user's finished recaps as viewed
	AuditEventRegenerateRecap    = "regenerateRecap"    // regenerate recap with updated channel content
	AuditEventDeleteRecap        = "deleteRecap"        // delete recap
)

// Scheduled Recaps
const (
	AuditEventCreateScheduledRecap = "createScheduledRecap" // create scheduled recap configuration
	AuditEventGetScheduledRecap    = "getScheduledRecap"    // view a single scheduled recap
	AuditEventGetScheduledRecaps   = "getScheduledRecaps"   // list user's scheduled recaps
	AuditEventUpdateScheduledRecap = "updateScheduledRecap" // update scheduled recap configuration
	AuditEventDeleteScheduledRecap = "deleteScheduledRecap" // delete scheduled recap
	AuditEventPauseScheduledRecap  = "pauseScheduledRecap"  // pause scheduled recap execution
	AuditEventResumeScheduledRecap = "resumeScheduledRecap" // resume paused scheduled recap
)

// Preferences
const (
	AuditEventDeletePreferences = "deletePreferences" // delete user preferences
	AuditEventUpdatePreferences = "updatePreferences" // update user preferences
)

// Remote Clusters
const (
	AuditEventCreateRemoteCluster            = "createRemoteCluster"            // create connection to remote Mattermost cluster
	AuditEventDeleteRemoteCluster            = "deleteRemoteCluster"            // delete connection to remote Mattermost cluster
	AuditEventGenerateRemoteClusterInvite    = "generateRemoteClusterInvite"    // generate invitation token for remote cluster connection
	AuditEventInviteRemoteClusterToChannel   = "inviteRemoteClusterToChannel"   // invite remote cluster users to shared channel
	AuditEventPatchRemoteCluster             = "patchRemoteCluster"             // update remote cluster connection settings
	AuditEventRemoteClusterAcceptInvite      = "remoteClusterAcceptInvite"      // accept invitation from remote cluster
	AuditEventRemoteClusterAcceptMessage     = "remoteClusterAcceptMessage"     // accept message from remote cluster
	AuditEventRemoteUploadProfileImage       = "remoteUploadProfileImage"       // upload profile image from remote cluster
	AuditEventUninviteRemoteClusterToChannel = "uninviteRemoteClusterToChannel" // remove remote cluster access from shared channel
	AuditEventUploadRemoteData               = "uploadRemoteData"               // upload data to remote cluster
)

// Roles
const (
	AuditEventPatchRole = "patchRole" // update role permissions
)

// SAML
const (
	AuditEventAddSamlIdpCertificate        = "addSamlIdpCertificate"        // add SAML identity provider certificate
	AuditEventAddSamlPrivateCertificate    = "addSamlPrivateCertificate"    // add SAML private certificate
	AuditEventAddSamlPublicCertificate     = "addSamlPublicCertificate"     // add SAML public certificate
	AuditEventCompleteSaml                 = "completeSaml"                 // complete SAML authentication flow
	AuditEventRemoveSamlIdpCertificate     = "removeSamlIdpCertificate"     // remove SAML identity provider certificate
	AuditEventRemoveSamlPrivateCertificate = "removeSamlPrivateCertificate" // remove SAML private certificate
	AuditEventRemoveSamlPublicCertificate  = "removeSamlPublicCertificate"  // remove SAML public certificate
)

// Scheduled Posts
const (
	AuditEventCreateSchedulePost  = "createSchedulePost"  // create post scheduled for future delivery
	AuditEventDeleteScheduledPost = "deleteScheduledPost" // delete scheduled post before delivery
	AuditEventUpdateScheduledPost = "updateScheduledPost" // update scheduled post
)

// Schemes
const (
	AuditEventCreateScheme = "createScheme" // create permission scheme with role definitions
	AuditEventDeleteScheme = "deleteScheme" // delete scheme
	AuditEventPatchScheme  = "patchScheme"  // update scheme
)

// Search Indexes
const (
	AuditEventPurgeBleveIndexes         = "purgeBleveIndexes"         // purge Bleve search indexes
	AuditEventPurgeElasticsearchIndexes = "purgeElasticsearchIndexes" // purge Elasticsearch search indexes
)

// Server Administration
const (
	AuditEventClearServerBusy            = "clearServerBusy"            // clear server busy status to allow normal operations
	AuditEventCompleteOnboarding         = "completeOnboarding"         // complete system onboarding process
	AuditEventDatabaseRecycle            = "databaseRecycle"            // closes active connections
	AuditEventDownloadLogs               = "downloadLogs"               // download server log files
	AuditEventGenerateSupportPacket      = "generateSupportPacket"      // generate support packet with server diagnostics and logs
	AuditEventGetAppliedSchemaMigrations = "getAppliedSchemaMigrations" // get list of applied database schema migrations
	AuditEventGetLogs                    = "getLogs"                    // get server log entries
	AuditEventGetOnboarding              = "getOnboarding"              // get system onboarding status
	AuditEventInvalidateCaches           = "invalidateCaches"           // clear server caches
	AuditEventLocalCheckIntegrity        = "localCheckIntegrity"        // check database integrity locally
	AuditEventQueryLogs                  = "queryLogs"                  // search server log entries
	AuditEventRestartServer              = "restartServer"              // restart Mattermost server process
	AuditEventSetServerBusy              = "setServerBusy"              // set server busy status to disallow any operations
	AuditEventUpdateViewedProductNotices = "updateViewedProductNotices" // update viewed status of product notices
	AuditEventUpgradeToEnterprise        = "upgradeToEnterprise"        // upgrade server to Enterprise edition
)

// Teams
const (
	AuditEventAddTeamMember               = "addTeamMember"               // add member to team
	AuditEventAddTeamMembers              = "addTeamMembers"              // add multiple members to team
	AuditEventAddUserToTeamFromInvite     = "addUserToTeamFromInvite"     // add user to team using invitation link
	AuditEventCreateTeam                  = "createTeam"                  // create team
	AuditEventDeleteTeam                  = "deleteTeam"                  // delete team
	AuditEventInvalidateAllEmailInvites   = "invalidateAllEmailInvites"   // invalidate all pending email invitations
	AuditEventInviteGuestsToChannels      = "inviteGuestsToChannels"      // invite guest users to specific channels
	AuditEventInviteUsersToTeam           = "inviteUsersToTeam"           // invite users to team
	AuditEventLocalCreateTeam             = "localCreateTeam"             // create team locally
	AuditEventLocalDeleteTeam             = "localDeleteTeam"             // delete team locally
	AuditEventLocalInviteUsersToTeam      = "localInviteUsersToTeam"      // invite users to team locally
	AuditEventPatchTeam                   = "patchTeam"                   // update team properties
	AuditEventRegenerateTeamInviteId      = "regenerateTeamInviteId"      // regenerate team invitation ID
	AuditEventRemoveTeamIcon              = "removeTeamIcon"              // remove custom icon from team
	AuditEventRemoveTeamMember            = "removeTeamMember"            // remove member from team
	AuditEventRestoreTeam                 = "restoreTeam"                 // restore previously deleted team
	AuditEventSetTeamIcon                 = "setTeamIcon"                 // set custom icon for team
	AuditEventUpdateTeam                  = "updateTeam"                  // update team properties
	AuditEventUpdateTeamMemberRoles       = "updateTeamMemberRoles"       // update roles of team members
	AuditEventUpdateTeamMemberSchemeRoles = "updateTeamMemberSchemeRoles" // update scheme-based roles of team members
	AuditEventUpdateTeamPrivacy           = "updateTeamPrivacy"           // change team privacy settings
	AuditEventUpdateTeamScheme            = "updateTeamScheme"            // update scheme applied to team
)

// Terms of Service
const (
	AuditEventCreateTermsOfService   = "createTermsOfService"   // create terms of service
	AuditEventSaveUserTermsOfService = "saveUserTermsOfService" // save user acceptance of terms of service
)

// Threads
const (
	AuditEventFollowThreadByUser              = "followThreadByUser"              // follow thread to receive notifications about replies
	AuditEventSetUnreadThreadByPostId         = "setUnreadThreadByPostId"         // mark thread as unread for user by post ID
	AuditEventUnfollowThreadByUser            = "unfollowThreadByUser"            // unfollow thread to stop receiving notifications about replies
	AuditEventUpdateReadStateAllThreadsByUser = "updateReadStateAllThreadsByUser" // update read status for all threads for user
	AuditEventUpdateReadStateThreadByUser     = "updateReadStateThreadByUser"     // update read status for specific thread for user
)

// Uploads
const (
	AuditEventCreateUpload = "createUpload" // create file upload session
	AuditEventUploadData   = "uploadData"   // upload file data to server storage
)

// Users
const (
	AuditEventAttachDeviceId                     = "attachDeviceId"                     // attach device IDs (standard or VoIP) to user session for mobile app
	AuditEventCreateUser                         = "createUser"                         // create user account
	AuditEventCreateUserAccessToken              = "createUserAccessToken"              // create personal access token for user API access
	AuditEventDeleteUser                         = "deleteUser"                         // delete user account
	AuditEventDemoteUserToGuest                  = "demoteUserToGuest"                  // demote regular user to guest account with limited permissions
	AuditEventDisableUserAccessToken             = "disableUserAccessToken"             // disable user personal access token
	AuditEventEnableUserAccessToken              = "enableUserAccessToken"              // enable user personal access token
	AuditEventExtendSessionExpiry                = "extendSessionExpiry"                // extend user session expiration time
	AuditEventLocalDeleteUser                    = "localDeleteUser"                    // delete user locally
	AuditEventLocalPermanentDeleteAllUsers       = "localPermanentDeleteAllUsers"       // permanently delete all users locally
	AuditEventLogin                              = "login"                              // user login to system
	AuditEventLoginWithDesktopToken              = "loginWithDesktopToken"              // user login to system with desktop token
	AuditEventLogout                             = "logout"                             // user logout from system
	AuditEventMarkMessagesRead                   = "markAllMessagesRead"                // user marked all direct and group messages as read
	AuditEventMarkTeamRead                       = "markFullTeamRead"                   // user marked an entire team as read
	AuditEventMigrateAuthToLdap                  = "migrateAuthToLdap"                  // migrate user authentication method to LDAP
	AuditEventMigrateAuthToSaml                  = "migrateAuthToSaml"                  // migrate user authentication method to SAML
	AuditEventPatchUser                          = "patchUser"                          // update user properties
	AuditEventPromoteGuestToUser                 = "promoteGuestToUser"                 // promote guest account to regular user
	AuditEventResetPassword                      = "resetPassword"                      // reset user password
	AuditEventResetPasswordFailedAttempts        = "resetPasswordFailedAttempts"        // reset failed password attempt counter
	AuditEventRevokeAllSessionsAllUsers          = "revokeAllSessionsAllUsers"          // revoke all active sessions for all users
	AuditEventRevokeAllSessionsForUser           = "revokeAllSessionsForUser"           // revoke all active sessions for specific user
	AuditEventRevokeSession                      = "revokeSession"                      // revoke specific user session
	AuditEventRejectExpiredUserAccessToken       = "rejectExpiredUserAccessToken"       // rejected an API request because the personal access token has expired
	AuditEventRevokeUserAccessToken              = "revokeUserAccessToken"              // revoke user personal access token
	AuditEventRevokeNonCompliantUserAccessTokens = "revokeNonCompliantUserAccessTokens" // revoke all personal access tokens that violate the maximum lifetime policy
	AuditEventRotateUserAccessToken              = "rotateUserAccessToken"              // rotate (regenerate secret for) user personal access token
	AuditEventSendPasswordReset                  = "sendPasswordReset"                  // send password reset email to user
	AuditEventSendVerificationEmail              = "sendVerificationEmail"              // send email verification link to user
	AuditEventSetDefaultProfileImage             = "setDefaultProfileImage"             // set user profile image to default avatar
	AuditEventSetProfileImage                    = "setProfileImage"                    // set custom profile image for user
	AuditEventSwitchAccountType                  = "switchAccountType"                  // switch user authentication method from one to another
	AuditEventUpdatePassword                     = "updatePassword"                     // update user password
	AuditEventUpdateUser                         = "updateUser"                         // update user account properties
	AuditEventUpdateUserActive                   = "updateUserActive"                   // update user active status
	AuditEventUpdateUserAuth                     = "updateUserAuth"                     // update user authentication method
	AuditEventUpdateUserMfa                      = "updateUserMfa"                      // update user multi-factor authentication settings
	AuditEventUpdateUserRoles                    = "updateUserRoles"                    // update user roles
	AuditEventVerifyUserEmail                    = "verifyUserEmail"                    // verify user email address using verification token
	AuditEventVerifyUserEmailWithoutToken        = "verifyUserEmailWithoutToken"        // verify user email address without verification token
)

// Webhooks
const (
	AuditEventCreateIncomingHook      = "createIncomingHook"      // create incoming webhook
	AuditEventCreateOutgoingHook      = "createOutgoingHook"      // create outgoing webhook
	AuditEventDeleteIncomingHook      = "deleteIncomingHook"      // delete incoming webhook
	AuditEventDeleteOutgoingHook      = "deleteOutgoingHook"      // delete outgoing webhook
	AuditEventGetIncomingHook         = "getIncomingHook"         // get incoming webhook details
	AuditEventGetOutgoingHook         = "getOutgoingHook"         // get outgoing webhook details
	AuditEventLocalCreateIncomingHook = "localCreateIncomingHook" // create incoming webhook locally
	AuditEventRegenOutgoingHookToken  = "regenOutgoingHookToken"  // regenerate authentication token
	AuditEventUpdateIncomingHook      = "updateIncomingHook"      // update incoming webhook
	AuditEventUpdateOutgoingHook      = "updateOutgoingHook"      // update outgoing webhook
)

// Content Flagging
const (
	AuditEventFlagPost                     = "flagPost"                     // flag post for review
	AuditEventGetFlaggedPost               = "getFlaggedPost"               // get flagged post details
	AuditEventPermanentlyRemoveFlaggedPost = "permanentlyRemoveFlaggedPost" // permanently remove flagged post
	AuditEventKeepFlaggedPost              = "keepFlaggedPost"              // keep flagged post
	AuditEventUpdateContentFlaggingConfig  = "updateContentFlaggingConfig"  // update content flagging configuration
	AuditEventSetReviewer                  = "setFlaggedPostReviewer"       // assign reviewer for flagged post
	AuditEventGenerateFlaggedPostReport    = "generateFlaggedPostReport"    // generate flagged post data report
	AuditEventGeneratePostExposureReport   = "generatePostExposureReport"   // generate flagged post exposure report
)

// Post Delivery Tracking
const (
	AuditEventUpdateDeliveryTrackingConfig = "updateDeliveryTrackingConfig" // update post delivery tracking configuration
	AuditEventPostDelivered                = "postDelivered"                // a post's content was delivered to a user or integration
)
