// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"io"
	"strings"
)

// SysconsoleAncillaryPermissions maps the non-sysconsole permissions required by each sysconsole view.
var SysconsoleAncillaryPermissions map[string][]*Permission
var SystemManagerDefaultPermissions []string
var SystemUserManagerDefaultPermissions []string
var SystemReadOnlyAdminDefaultPermissions []string

var BuiltInSchemeManagedRoleIDs []string

var NewSystemRoleIDs []string

func init() {
	NewSystemRoleIDs = []string{
		SystemUserManagerRoleID,
		SystemReadOnlyAdminRoleID,
		SystemManagerRoleID,
	}

	BuiltInSchemeManagedRoleIDs = append([]string{
		SystemGuestRoleID,
		SystemUserRoleID,
		SystemAdminRoleID,
		SystemPostAllRoleID,
		SystemPostAllPublicRoleID,
		SystemUserAccessTokenRoleID,

		TeamGuestRoleID,
		TeamUserRoleID,
		TeamAdminRoleID,
		TeamPostAllRoleID,
		TeamPostAllPublicRoleID,

		ChannelGuestRoleID,
		ChannelUserRoleID,
		ChannelAdminRoleID,
	}, NewSystemRoleIDs...)

	// When updating the values here, the values in mattermost-redux must also be updated.
	SysconsoleAncillaryPermissions = map[string][]*Permission{
		PermissionSysconsoleReadAboutEditionAndLicense.ID: {
			PermissionReadLicenseInformation,
		},
		PermissionSysconsoleWriteAboutEditionAndLicense.ID: {
			PermissionManageLicenseInformation,
		},
		PermissionSysconsoleReadUserManagementChannels.ID: {
			PermissionReadPublicChannel,
			PermissionReadChannel,
			PermissionReadPublicChannelGroups,
			PermissionReadPrivateChannelGroups,
		},
		PermissionSysconsoleReadUserManagementUsers.ID: {
			PermissionReadOtherUsersTeams,
			PermissionGetAnalytics,
		},
		PermissionSysconsoleReadUserManagementTeams.ID: {
			PermissionListPrivateTeams,
			PermissionListPublicTeams,
			PermissionViewTeam,
		},
		PermissionSysconsoleReadEnvironmentElasticsearch.ID: {
			PermissionReadElasticsearchPostIndexingJob,
			PermissionReadElasticsearchPostAggregationJob,
		},
		PermissionSysconsoleWriteEnvironmentWebServer.ID: {
			PermissionTestSiteURL,
			PermissionReloadConfig,
			PermissionInvalidateCaches,
		},
		PermissionSysconsoleWriteEnvironmentDatabase.ID: {
			PermissionRecycleDatabaseConnections,
		},
		PermissionSysconsoleWriteEnvironmentElasticsearch.ID: {
			PermissionTestElasticsearch,
			PermissionCreateElasticsearchPostIndexingJob,
			PermissionCreateElasticsearchPostAggregationJob,
			PermissionPurgeElasticsearchIndexes,
		},
		PermissionSysconsoleWriteEnvironmentFileStorage.ID: {
			PermissionTestS3,
		},
		PermissionSysconsoleWriteEnvironmentSMTP.ID: {
			PermissionTestEmail,
		},
		PermissionSysconsoleReadReportingServerLogs.ID: {
			PermissionGetLogs,
		},
		PermissionSysconsoleReadReportingSiteStatistics.ID: {
			PermissionGetAnalytics,
		},
		PermissionSysconsoleReadReportingTeamStatistics.ID: {
			PermissionViewTeam,
		},
		PermissionSysconsoleWriteUserManagementUsers.ID: {
			PermissionEditOtherUsers,
			PermissionDemoteToGuest,
			PermissionPromoteGuest,
		},
		PermissionSysconsoleWriteUserManagementChannels.ID: {
			PermissionManageTeam,
			PermissionManagePublicChannelProperties,
			PermissionManagePrivateChannelProperties,
			PermissionManagePrivateChannelMembers,
			PermissionManagePublicChannelMembers,
			PermissionDeletePrivateChannel,
			PermissionDeletePublicChannel,
			PermissionManageChannelRoles,
			PermissionConvertPublicChannelToPrivate,
			PermissionConvertPrivateChannelToPublic,
		},
		PermissionSysconsoleWriteUserManagementTeams.ID: {
			PermissionManageTeam,
			PermissionManageTeamRoles,
			PermissionRemoveUserFromTeam,
			PermissionJoinPrivateTeams,
			PermissionJoinPublicTeams,
			PermissionAddUserToTeam,
		},
		PermissionSysconsoleWriteUserManagementGroups.ID: {
			PermissionManageTeam,
			PermissionManagePrivateChannelMembers,
			PermissionManagePublicChannelMembers,
			PermissionConvertPublicChannelToPrivate,
			PermissionConvertPrivateChannelToPublic,
		},
		PermissionSysconsoleWriteSiteCustomization.ID: {
			PermissionEditBrand,
		},
		PermissionSysconsoleWriteComplianceDataRetentionPolicy.ID: {
			PermissionCreateDataRetentionJob,
		},
		PermissionSysconsoleReadComplianceDataRetentionPolicy.ID: {
			PermissionReadDataRetentionJob,
		},
		PermissionSysconsoleWriteComplianceComplianceExport.ID: {
			PermissionCreateComplianceExportJob,
			PermissionDownloadComplianceExportResult,
		},
		PermissionSysconsoleReadComplianceComplianceExport.ID: {
			PermissionReadComplianceExportJob,
			PermissionDownloadComplianceExportResult,
		},
		PermissionSysconsoleReadComplianceCustomTermsOfService.ID: {
			PermissionReadAudits,
		},
		PermissionSysconsoleWriteExperimentalBleve.ID: {
			PermissionCreatePostBleveIndexesJob,
			PermissionPurgeBleveIndexes,
		},
		PermissionSysconsoleWriteAuthenticationLdap.ID: {
			PermissionCreateLdapSyncJob,
			PermissionAddLdapPublicCert,
			PermissionRemoveLdapPublicCert,
			PermissionAddLdapPrivateCert,
			PermissionRemoveLdapPrivateCert,
		},
		PermissionSysconsoleReadAuthenticationLdap.ID: {
			PermissionTestLdap,
			PermissionReadLdapSyncJob,
		},
		PermissionSysconsoleWriteAuthenticationEmail.ID: {
			PermissionInvalidateEmailInvite,
		},
		PermissionSysconsoleWriteAuthenticationSaml.ID: {
			PermissionGetSamlMetadataFromIDp,
			PermissionAddSamlPublicCert,
			PermissionAddSamlPrivateCert,
			PermissionAddSamlIDpCert,
			PermissionRemoveSamlPublicCert,
			PermissionRemoveSamlPrivateCert,
			PermissionRemoveSamlIDpCert,
			PermissionGetSamlCertStatus,
		},
	}

	SystemUserManagerDefaultPermissions = []string{
		PermissionSysconsoleReadUserManagementGroups.ID,
		PermissionSysconsoleReadUserManagementTeams.ID,
		PermissionSysconsoleReadUserManagementChannels.ID,
		PermissionSysconsoleReadUserManagementPermissions.ID,
		PermissionSysconsoleWriteUserManagementGroups.ID,
		PermissionSysconsoleWriteUserManagementTeams.ID,
		PermissionSysconsoleWriteUserManagementChannels.ID,
		PermissionSysconsoleReadAuthenticationSignup.ID,
		PermissionSysconsoleReadAuthenticationEmail.ID,
		PermissionSysconsoleReadAuthenticationPassword.ID,
		PermissionSysconsoleReadAuthenticationMfa.ID,
		PermissionSysconsoleReadAuthenticationLdap.ID,
		PermissionSysconsoleReadAuthenticationSaml.ID,
		PermissionSysconsoleReadAuthenticationOpenid.ID,
		PermissionSysconsoleReadAuthenticationGuestAccess.ID,
	}

	SystemReadOnlyAdminDefaultPermissions = []string{
		PermissionSysconsoleReadAboutEditionAndLicense.ID,
		PermissionSysconsoleReadReportingSiteStatistics.ID,
		PermissionSysconsoleReadReportingTeamStatistics.ID,
		PermissionSysconsoleReadReportingServerLogs.ID,
		PermissionSysconsoleReadUserManagementUsers.ID,
		PermissionSysconsoleReadUserManagementGroups.ID,
		PermissionSysconsoleReadUserManagementTeams.ID,
		PermissionSysconsoleReadUserManagementChannels.ID,
		PermissionSysconsoleReadUserManagementPermissions.ID,
		PermissionSysconsoleReadEnvironmentWebServer.ID,
		PermissionSysconsoleReadEnvironmentDatabase.ID,
		PermissionSysconsoleReadEnvironmentElasticsearch.ID,
		PermissionSysconsoleReadEnvironmentFileStorage.ID,
		PermissionSysconsoleReadEnvironmentImageProxy.ID,
		PermissionSysconsoleReadEnvironmentSMTP.ID,
		PermissionSysconsoleReadEnvironmentPushNotificationServer.ID,
		PermissionSysconsoleReadEnvironmentHighAvailability.ID,
		PermissionSysconsoleReadEnvironmentRateLimiting.ID,
		PermissionSysconsoleReadEnvironmentLogging.ID,
		PermissionSysconsoleReadEnvironmentSessionLengths.ID,
		PermissionSysconsoleReadEnvironmentPerformanceMonitoring.ID,
		PermissionSysconsoleReadEnvironmentDeveloper.ID,
		PermissionSysconsoleReadSiteCustomization.ID,
		PermissionSysconsoleReadSiteLocalization.ID,
		PermissionSysconsoleReadSiteUsersAndTeams.ID,
		PermissionSysconsoleReadSiteNotifications.ID,
		PermissionSysconsoleReadSiteAnnouncementBanner.ID,
		PermissionSysconsoleReadSiteEmoji.ID,
		PermissionSysconsoleReadSitePosts.ID,
		PermissionSysconsoleReadSiteFileSharingAndDownloads.ID,
		PermissionSysconsoleReadSitePublicLinks.ID,
		PermissionSysconsoleReadSiteNotices.ID,
		PermissionSysconsoleReadAuthenticationSignup.ID,
		PermissionSysconsoleReadAuthenticationEmail.ID,
		PermissionSysconsoleReadAuthenticationPassword.ID,
		PermissionSysconsoleReadAuthenticationMfa.ID,
		PermissionSysconsoleReadAuthenticationLdap.ID,
		PermissionSysconsoleReadAuthenticationSaml.ID,
		PermissionSysconsoleReadAuthenticationOpenid.ID,
		PermissionSysconsoleReadAuthenticationGuestAccess.ID,
		PermissionSysconsoleReadPlugins.ID,
		PermissionSysconsoleReadIntegrationsIntegrationManagement.ID,
		PermissionSysconsoleReadIntegrationsBotAccounts.ID,
		PermissionSysconsoleReadIntegrationsGif.ID,
		PermissionSysconsoleReadIntegrationsCors.ID,
		PermissionSysconsoleReadComplianceDataRetentionPolicy.ID,
		PermissionSysconsoleReadComplianceComplianceExport.ID,
		PermissionSysconsoleReadComplianceComplianceMonitoring.ID,
		PermissionSysconsoleReadComplianceCustomTermsOfService.ID,
		PermissionSysconsoleReadExperimentalFeatures.ID,
		PermissionSysconsoleReadExperimentalFeatureFlags.ID,
		PermissionSysconsoleReadExperimentalBleve.ID,
	}

	SystemManagerDefaultPermissions = []string{
		PermissionSysconsoleReadAboutEditionAndLicense.ID,
		PermissionSysconsoleReadReportingSiteStatistics.ID,
		PermissionSysconsoleReadReportingTeamStatistics.ID,
		PermissionSysconsoleReadReportingServerLogs.ID,
		PermissionSysconsoleReadUserManagementGroups.ID,
		PermissionSysconsoleReadUserManagementTeams.ID,
		PermissionSysconsoleReadUserManagementChannels.ID,
		PermissionSysconsoleReadUserManagementPermissions.ID,
		PermissionSysconsoleWriteUserManagementGroups.ID,
		PermissionSysconsoleWriteUserManagementTeams.ID,
		PermissionSysconsoleWriteUserManagementChannels.ID,
		PermissionSysconsoleWriteUserManagementPermissions.ID,
		PermissionSysconsoleReadEnvironmentWebServer.ID,
		PermissionSysconsoleReadEnvironmentDatabase.ID,
		PermissionSysconsoleReadEnvironmentElasticsearch.ID,
		PermissionSysconsoleReadEnvironmentFileStorage.ID,
		PermissionSysconsoleReadEnvironmentImageProxy.ID,
		PermissionSysconsoleReadEnvironmentSMTP.ID,
		PermissionSysconsoleReadEnvironmentPushNotificationServer.ID,
		PermissionSysconsoleReadEnvironmentHighAvailability.ID,
		PermissionSysconsoleReadEnvironmentRateLimiting.ID,
		PermissionSysconsoleReadEnvironmentLogging.ID,
		PermissionSysconsoleReadEnvironmentSessionLengths.ID,
		PermissionSysconsoleReadEnvironmentPerformanceMonitoring.ID,
		PermissionSysconsoleReadEnvironmentDeveloper.ID,
		PermissionSysconsoleWriteEnvironmentWebServer.ID,
		PermissionSysconsoleWriteEnvironmentDatabase.ID,
		PermissionSysconsoleWriteEnvironmentElasticsearch.ID,
		PermissionSysconsoleWriteEnvironmentFileStorage.ID,
		PermissionSysconsoleWriteEnvironmentImageProxy.ID,
		PermissionSysconsoleWriteEnvironmentSMTP.ID,
		PermissionSysconsoleWriteEnvironmentPushNotificationServer.ID,
		PermissionSysconsoleWriteEnvironmentHighAvailability.ID,
		PermissionSysconsoleWriteEnvironmentRateLimiting.ID,
		PermissionSysconsoleWriteEnvironmentLogging.ID,
		PermissionSysconsoleWriteEnvironmentSessionLengths.ID,
		PermissionSysconsoleWriteEnvironmentPerformanceMonitoring.ID,
		PermissionSysconsoleWriteEnvironmentDeveloper.ID,
		PermissionSysconsoleReadSiteCustomization.ID,
		PermissionSysconsoleWriteSiteCustomization.ID,
		PermissionSysconsoleReadSiteLocalization.ID,
		PermissionSysconsoleWriteSiteLocalization.ID,
		PermissionSysconsoleReadSiteUsersAndTeams.ID,
		PermissionSysconsoleWriteSiteUsersAndTeams.ID,
		PermissionSysconsoleReadSiteNotifications.ID,
		PermissionSysconsoleWriteSiteNotifications.ID,
		PermissionSysconsoleReadSiteAnnouncementBanner.ID,
		PermissionSysconsoleWriteSiteAnnouncementBanner.ID,
		PermissionSysconsoleReadSiteEmoji.ID,
		PermissionSysconsoleWriteSiteEmoji.ID,
		PermissionSysconsoleReadSitePosts.ID,
		PermissionSysconsoleWriteSitePosts.ID,
		PermissionSysconsoleReadSiteFileSharingAndDownloads.ID,
		PermissionSysconsoleWriteSiteFileSharingAndDownloads.ID,
		PermissionSysconsoleReadSitePublicLinks.ID,
		PermissionSysconsoleWriteSitePublicLinks.ID,
		PermissionSysconsoleReadSiteNotices.ID,
		PermissionSysconsoleWriteSiteNotices.ID,
		PermissionSysconsoleReadAuthenticationSignup.ID,
		PermissionSysconsoleReadAuthenticationEmail.ID,
		PermissionSysconsoleReadAuthenticationPassword.ID,
		PermissionSysconsoleReadAuthenticationMfa.ID,
		PermissionSysconsoleReadAuthenticationLdap.ID,
		PermissionSysconsoleReadAuthenticationSaml.ID,
		PermissionSysconsoleReadAuthenticationOpenid.ID,
		PermissionSysconsoleReadAuthenticationGuestAccess.ID,
		PermissionSysconsoleReadPlugins.ID,
		PermissionSysconsoleReadIntegrationsIntegrationManagement.ID,
		PermissionSysconsoleReadIntegrationsBotAccounts.ID,
		PermissionSysconsoleReadIntegrationsGif.ID,
		PermissionSysconsoleReadIntegrationsCors.ID,
		PermissionSysconsoleWriteIntegrationsIntegrationManagement.ID,
		PermissionSysconsoleWriteIntegrationsBotAccounts.ID,
		PermissionSysconsoleWriteIntegrationsGif.ID,
		PermissionSysconsoleWriteIntegrationsCors.ID,
	}

	// Add the ancillary permissions to each system role
	SystemUserManagerDefaultPermissions = AddAncillaryPermissions(SystemUserManagerDefaultPermissions)
	SystemReadOnlyAdminDefaultPermissions = AddAncillaryPermissions(SystemReadOnlyAdminDefaultPermissions)
	SystemManagerDefaultPermissions = AddAncillaryPermissions(SystemManagerDefaultPermissions)
}

type RoleType string
type RoleScope string

const (
	SystemGuestRoleID           = "system_guest"
	SystemUserRoleID            = "system_user"
	SystemAdminRoleID           = "system_admin"
	SystemPostAllRoleID         = "system_post_all"
	SystemPostAllPublicRoleID   = "system_post_all_public"
	SystemUserAccessTokenRoleID = "system_user_access_token"
	SystemUserManagerRoleID     = "system_user_manager"
	SystemReadOnlyAdminRoleID   = "system_read_only_admin"
	SystemManagerRoleID         = "system_manager"

	TeamGuestRoleID         = "team_guest"
	TeamUserRoleID          = "team_user"
	TeamAdminRoleID         = "team_admin"
	TeamPostAllRoleID       = "team_post_all"
	TeamPostAllPublicRoleID = "team_post_all_public"

	ChannelGuestRoleID = "channel_guest"
	ChannelUserRoleID  = "channel_user"
	ChannelAdminRoleID = "channel_admin"

	RoleNameMaxLength        = 64
	RoleDisplayNameMaxLength = 128
	RoleDescriptionMaxLength = 1024

	RoleScopeSystem  RoleScope = "System"
	RoleScopeTeam    RoleScope = "Team"
	RoleScopeChannel RoleScope = "Channel"

	RoleTypeGuest RoleType = "Guest"
	RoleTypeUser  RoleType = "User"
	RoleTypeAdmin RoleType = "Admin"
)

type Role struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name"`
	Description   string   `json:"description"`
	CreateAt      int64    `json:"create_at"`
	UpdateAt      int64    `json:"update_at"`
	DeleteAt      int64    `json:"delete_at"`
	Permissions   []string `json:"permissions"`
	SchemeManaged bool     `json:"scheme_managed"`
	BuiltIn       bool     `json:"built_in"`
}

type RolePatch struct {
	Permissions *[]string `json:"permissions"`
}

type RolePermissions struct {
	RoleID      string
	Permissions []string
}

func (r *Role) ToJSON() string {
	b, _ := json.Marshal(r)
	return string(b)
}

func RoleFromJSON(data io.Reader) *Role {
	var r *Role
	json.NewDecoder(data).Decode(&r)
	return r
}

func RoleListToJSON(r []*Role) string {
	b, _ := json.Marshal(r)
	return string(b)
}

func RoleListFromJSON(data io.Reader) []*Role {
	var roles []*Role
	json.NewDecoder(data).Decode(&roles)
	return roles
}

func (r *RolePatch) ToJSON() string {
	b, _ := json.Marshal(r)
	return string(b)
}

func RolePatchFromJSON(data io.Reader) *RolePatch {
	var rolePatch *RolePatch
	json.NewDecoder(data).Decode(&rolePatch)
	return rolePatch
}

func (r *Role) Patch(patch *RolePatch) {
	if patch.Permissions != nil {
		r.Permissions = *patch.Permissions
	}
}

// MergeChannelHigherScopedPermissions is meant to be invoked on a channel scheme's role and merges the higher-scoped
// channel role's permissions.
func (r *Role) MergeChannelHigherScopedPermissions(higherScopedPermissions *RolePermissions) {
	mergedPermissions := []string{}

	higherScopedPermissionsMap := AsStringBoolMap(higherScopedPermissions.Permissions)
	rolePermissionsMap := AsStringBoolMap(r.Permissions)

	for _, cp := range AllPermissions {
		if cp.Scope != PermissionScopeChannel {
			continue
		}

		_, presentOnHigherScope := higherScopedPermissionsMap[cp.ID]

		// For the channel admin role always look to the higher scope to determine if the role has their permission.
		// The channel admin is a special case because they're not part of the UI to be "channel moderated", only
		// channel members and channel guests are.
		if higherScopedPermissions.RoleID == ChannelAdminRoleID && presentOnHigherScope {
			mergedPermissions = append(mergedPermissions, cp.ID)
			continue
		}

		_, permissionIsModerated := ChannelModeratedPermissionsMap[cp.ID]
		if permissionIsModerated {
			_, presentOnRole := rolePermissionsMap[cp.ID]
			if presentOnRole && presentOnHigherScope {
				mergedPermissions = append(mergedPermissions, cp.ID)
			}
		} else {
			if presentOnHigherScope {
				mergedPermissions = append(mergedPermissions, cp.ID)
			}
		}
	}

	r.Permissions = mergedPermissions
}

// Returns an array of permissions that are in either role.Permissions
// or patch.Permissions, but not both.
func PermissionsChangedByPatch(role *Role, patch *RolePatch) []string {
	var result []string

	if patch.Permissions == nil {
		return result
	}

	roleMap := make(map[string]bool)
	patchMap := make(map[string]bool)

	for _, permission := range role.Permissions {
		roleMap[permission] = true
	}

	for _, permission := range *patch.Permissions {
		patchMap[permission] = true
	}

	for _, permission := range role.Permissions {
		if !patchMap[permission] {
			result = append(result, permission)
		}
	}

	for _, permission := range *patch.Permissions {
		if !roleMap[permission] {
			result = append(result, permission)
		}
	}

	return result
}

func ChannelModeratedPermissionsChangedByPatch(role *Role, patch *RolePatch) []string {
	var result []string

	if role == nil {
		return result
	}

	if patch.Permissions == nil {
		return result
	}

	roleMap := make(map[string]bool)
	patchMap := make(map[string]bool)

	for _, permission := range role.Permissions {
		if channelModeratedPermissionName, found := ChannelModeratedPermissionsMap[permission]; found {
			roleMap[channelModeratedPermissionName] = true
		}
	}

	for _, permission := range *patch.Permissions {
		if channelModeratedPermissionName, found := ChannelModeratedPermissionsMap[permission]; found {
			patchMap[channelModeratedPermissionName] = true
		}
	}

	for permissionKey := range roleMap {
		if !patchMap[permissionKey] {
			result = append(result, permissionKey)
		}
	}

	for permissionKey := range patchMap {
		if !roleMap[permissionKey] {
			result = append(result, permissionKey)
		}
	}

	return result
}

// GetChannelModeratedPermissions returns a map of channel moderated permissions that the role has access to
func (r *Role) GetChannelModeratedPermissions(channelType string) map[string]bool {
	moderatedPermissions := make(map[string]bool)
	for _, permission := range r.Permissions {
		if _, found := ChannelModeratedPermissionsMap[permission]; !found {
			continue
		}

		for moderated, moderatedPermissionValue := range ChannelModeratedPermissionsMap {
			// the moderated permission has already been found to be true so skip this iteration
			if moderatedPermissions[moderatedPermissionValue] {
				continue
			}

			if moderated == permission {
				// Special case where the channel moderated permission for `manage_members` is different depending on whether the channel is private or public
				if moderated == PermissionManagePublicChannelMembers.ID || moderated == PermissionManagePrivateChannelMembers.ID {
					canManagePublic := channelType == ChannelTypeOpen && moderated == PermissionManagePublicChannelMembers.ID
					canManagePrivate := channelType == ChannelTypePrivate && moderated == PermissionManagePrivateChannelMembers.ID
					moderatedPermissions[moderatedPermissionValue] = canManagePublic || canManagePrivate
				} else {
					moderatedPermissions[moderatedPermissionValue] = true
				}
			}
		}
	}

	return moderatedPermissions
}

// RolePatchFromChannelModerationsPatch Creates and returns a RolePatch based on a slice of ChannelModerationPatchs, roleName is expected to be either "members" or "guests".
func (r *Role) RolePatchFromChannelModerationsPatch(channelModerationsPatch []*ChannelModerationPatch, roleName string) *RolePatch {
	permissionsToAddToPatch := make(map[string]bool)

	// Iterate through the list of existing permissions on the role and append permissions that we want to keep.
	for _, permission := range r.Permissions {
		// Permission is not moderated so dont add it to the patch and skip the channelModerationsPatch
		if _, isModerated := ChannelModeratedPermissionsMap[permission]; !isModerated {
			continue
		}

		permissionEnabled := true
		// Check if permission has a matching moderated permission name inside the channel moderation patch
		for _, channelModerationPatch := range channelModerationsPatch {
			if *channelModerationPatch.Name == ChannelModeratedPermissionsMap[permission] {
				// Permission key exists in patch with a value of false so skip over it
				if roleName == "members" {
					if channelModerationPatch.Roles.Members != nil && !*channelModerationPatch.Roles.Members {
						permissionEnabled = false
					}
				} else if roleName == "guests" {
					if channelModerationPatch.Roles.Guests != nil && !*channelModerationPatch.Roles.Guests {
						permissionEnabled = false
					}
				}
			}
		}

		if permissionEnabled {
			permissionsToAddToPatch[permission] = true
		}
	}

	// Iterate through the patch and add any permissions that dont already exist on the role
	for _, channelModerationPatch := range channelModerationsPatch {
		for permission, moderatedPermissionName := range ChannelModeratedPermissionsMap {
			if roleName == "members" && channelModerationPatch.Roles.Members != nil && *channelModerationPatch.Roles.Members && *channelModerationPatch.Name == moderatedPermissionName {
				permissionsToAddToPatch[permission] = true
			}

			if roleName == "guests" && channelModerationPatch.Roles.Guests != nil && *channelModerationPatch.Roles.Guests && *channelModerationPatch.Name == moderatedPermissionName {
				permissionsToAddToPatch[permission] = true
			}
		}
	}

	patchPermissions := make([]string, 0, len(permissionsToAddToPatch))
	for permission := range permissionsToAddToPatch {
		patchPermissions = append(patchPermissions, permission)
	}

	return &RolePatch{Permissions: &patchPermissions}
}

func (r *Role) IsValid() bool {
	if !IsValidID(r.ID) {
		return false
	}

	return r.IsValidWithoutID()
}

func (r *Role) IsValidWithoutID() bool {
	if !IsValidRoleName(r.Name) {
		return false
	}

	if r.DisplayName == "" || len(r.DisplayName) > RoleDisplayNameMaxLength {
		return false
	}

	if len(r.Description) > RoleDescriptionMaxLength {
		return false
	}

	check := func(perms []*Permission, permission string) bool {
		for _, p := range perms {
			if permission == p.ID {
				return true
			}
		}
		return false
	}
	for _, permission := range r.Permissions {
		permissionValidated := check(AllPermissions, permission) || check(DeprecatedPermissions, permission)
		if !permissionValidated {
			return false
		}
	}

	return true
}

func CleanRoleNames(roleNames []string) ([]string, bool) {
	var cleanedRoleNames []string
	for _, roleName := range roleNames {
		if strings.TrimSpace(roleName) == "" {
			continue
		}

		if !IsValidRoleName(roleName) {
			return roleNames, false
		}

		cleanedRoleNames = append(cleanedRoleNames, roleName)
	}

	return cleanedRoleNames, true
}

func IsValidRoleName(roleName string) bool {
	if roleName == "" || len(roleName) > RoleNameMaxLength {
		return false
	}

	if strings.TrimLeft(roleName, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" {
		return false
	}

	return true
}

func MakeDefaultRoles() map[string]*Role {
	roles := make(map[string]*Role)

	roles[ChannelGuestRoleID] = &Role{
		Name:        "channel_guest",
		DisplayName: "authentication.roles.channel_guest.name",
		Description: "authentication.roles.channel_guest.description",
		Permissions: []string{
			PermissionReadChannel.ID,
			PermissionAddReaction.ID,
			PermissionRemoveReaction.ID,
			PermissionUploadFile.ID,
			PermissionEditPost.ID,
			PermissionCreatePost.ID,
			PermissionUseChannelMentions.ID,
			PermissionUseSlashCommands.ID,
		},
		SchemeManaged: true,
		BuiltIn:       true,
	}

	roles[ChannelUserRoleID] = &Role{
		Name:        "channel_user",
		DisplayName: "authentication.roles.channel_user.name",
		Description: "authentication.roles.channel_user.description",
		Permissions: []string{
			PermissionReadChannel.ID,
			PermissionAddReaction.ID,
			PermissionRemoveReaction.ID,
			PermissionManagePublicChannelMembers.ID,
			PermissionUploadFile.ID,
			PermissionGetPublicLink.ID,
			PermissionCreatePost.ID,
			PermissionUseChannelMentions.ID,
			PermissionUseSlashCommands.ID,
		},
		SchemeManaged: true,
		BuiltIn:       true,
	}

	roles[ChannelAdminRoleID] = &Role{
		Name:        "channel_admin",
		DisplayName: "authentication.roles.channel_admin.name",
		Description: "authentication.roles.channel_admin.description",
		Permissions: []string{
			PermissionManageChannelRoles.ID,
			PermissionUseGroupMentions.ID,
		},
		SchemeManaged: true,
		BuiltIn:       true,
	}

	roles[TeamGuestRoleID] = &Role{
		Name:        "team_guest",
		DisplayName: "authentication.roles.team_guest.name",
		Description: "authentication.roles.team_guest.description",
		Permissions: []string{
			PermissionViewTeam.ID,
		},
		SchemeManaged: true,
		BuiltIn:       true,
	}

	roles[TeamUserRoleID] = &Role{
		Name:        "team_user",
		DisplayName: "authentication.roles.team_user.name",
		Description: "authentication.roles.team_user.description",
		Permissions: []string{
			PermissionListTeamChannels.ID,
			PermissionJoinPublicChannels.ID,
			PermissionReadPublicChannel.ID,
			PermissionViewTeam.ID,
		},
		SchemeManaged: true,
		BuiltIn:       true,
	}

	roles[TeamPostAllRoleID] = &Role{
		Name:        "team_post_all",
		DisplayName: "authentication.roles.team_post_all.name",
		Description: "authentication.roles.team_post_all.description",
		Permissions: []string{
			PermissionCreatePost.ID,
			PermissionUseChannelMentions.ID,
		},
		SchemeManaged: false,
		BuiltIn:       true,
	}

	roles[TeamPostAllPublicRoleID] = &Role{
		Name:        "team_post_all_public",
		DisplayName: "authentication.roles.team_post_all_public.name",
		Description: "authentication.roles.team_post_all_public.description",
		Permissions: []string{
			PermissionCreatePostPublic.ID,
			PermissionUseChannelMentions.ID,
		},
		SchemeManaged: false,
		BuiltIn:       true,
	}

	roles[TeamAdminRoleID] = &Role{
		Name:        "team_admin",
		DisplayName: "authentication.roles.team_admin.name",
		Description: "authentication.roles.team_admin.description",
		Permissions: []string{
			PermissionRemoveUserFromTeam.ID,
			PermissionManageTeam.ID,
			PermissionImportTeam.ID,
			PermissionManageTeamRoles.ID,
			PermissionManageChannelRoles.ID,
			PermissionManageOthersIncomingWebhooks.ID,
			PermissionManageOthersOutgoingWebhooks.ID,
			PermissionManageSlashCommands.ID,
			PermissionManageOthersSlashCommands.ID,
			PermissionManageIncomingWebhooks.ID,
			PermissionManageOutgoingWebhooks.ID,
			PermissionConvertPublicChannelToPrivate.ID,
			PermissionConvertPrivateChannelToPublic.ID,
		},
		SchemeManaged: true,
		BuiltIn:       true,
	}

	roles[SystemGuestRoleID] = &Role{
		Name:        "system_guest",
		DisplayName: "authentication.roles.global_guest.name",
		Description: "authentication.roles.global_guest.description",
		Permissions: []string{
			PermissionCreateDirectChannel.ID,
			PermissionCreateGroupChannel.ID,
		},
		SchemeManaged: true,
		BuiltIn:       true,
	}

	roles[SystemUserRoleID] = &Role{
		Name:        "system_user",
		DisplayName: "authentication.roles.global_user.name",
		Description: "authentication.roles.global_user.description",
		Permissions: []string{
			PermissionListPublicTeams.ID,
			PermissionJoinPublicTeams.ID,
			PermissionCreateDirectChannel.ID,
			PermissionCreateGroupChannel.ID,
			PermissionViewMembers.ID,
		},
		SchemeManaged: true,
		BuiltIn:       true,
	}

	roles[SystemPostAllRoleID] = &Role{
		Name:        "system_post_all",
		DisplayName: "authentication.roles.system_post_all.name",
		Description: "authentication.roles.system_post_all.description",
		Permissions: []string{
			PermissionCreatePost.ID,
			PermissionUseChannelMentions.ID,
		},
		SchemeManaged: false,
		BuiltIn:       true,
	}

	roles[SystemPostAllPublicRoleID] = &Role{
		Name:        "system_post_all_public",
		DisplayName: "authentication.roles.system_post_all_public.name",
		Description: "authentication.roles.system_post_all_public.description",
		Permissions: []string{
			PermissionCreatePostPublic.ID,
			PermissionUseChannelMentions.ID,
		},
		SchemeManaged: false,
		BuiltIn:       true,
	}

	roles[SystemUserAccessTokenRoleID] = &Role{
		Name:        "system_user_access_token",
		DisplayName: "authentication.roles.system_user_access_token.name",
		Description: "authentication.roles.system_user_access_token.description",
		Permissions: []string{
			PermissionCreateUserAccessToken.ID,
			PermissionReadUserAccessToken.ID,
			PermissionRevokeUserAccessToken.ID,
		},
		SchemeManaged: false,
		BuiltIn:       true,
	}

	roles[SystemUserManagerRoleID] = &Role{
		Name:          "system_user_manager",
		DisplayName:   "authentication.roles.system_user_manager.name",
		Description:   "authentication.roles.system_user_manager.description",
		Permissions:   SystemUserManagerDefaultPermissions,
		SchemeManaged: false,
		BuiltIn:       true,
	}

	roles[SystemReadOnlyAdminRoleID] = &Role{
		Name:          "system_read_only_admin",
		DisplayName:   "authentication.roles.system_read_only_admin.name",
		Description:   "authentication.roles.system_read_only_admin.description",
		Permissions:   SystemReadOnlyAdminDefaultPermissions,
		SchemeManaged: false,
		BuiltIn:       true,
	}

	roles[SystemManagerRoleID] = &Role{
		Name:          "system_manager",
		DisplayName:   "authentication.roles.system_manager.name",
		Description:   "authentication.roles.system_manager.description",
		Permissions:   SystemManagerDefaultPermissions,
		SchemeManaged: false,
		BuiltIn:       true,
	}

	allPermissionIDs := []string{}
	for _, permission := range AllPermissions {
		allPermissionIDs = append(allPermissionIDs, permission.ID)
	}

	roles[SystemAdminRoleID] = &Role{
		Name:        "system_admin",
		DisplayName: "authentication.roles.global_admin.name",
		Description: "authentication.roles.global_admin.description",
		// System admins can do anything channel and team admins can do
		// plus everything members of teams and channels can do to all teams
		// and channels on the system
		Permissions:   allPermissionIDs,
		SchemeManaged: true,
		BuiltIn:       true,
	}

	return roles
}

func AddAncillaryPermissions(permissions []string) []string {
	for _, permission := range permissions {
		if ancillaryPermissions, ok := SysconsoleAncillaryPermissions[permission]; ok {
			for _, ancillaryPermission := range ancillaryPermissions {
				permissions = append(permissions, ancillaryPermission.ID)
			}
		}
	}
	return permissions
}
