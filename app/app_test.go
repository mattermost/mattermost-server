// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

/* Temporarily comment out until MM-11108
func TestAppRace(t *testing.T) {
	for i := 0; i < 10; i++ {
		a, err := New()
		require.NoError(t, err)
		a.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.ListenAddress = ":0" })
		serverErr := a.StartServer()
		require.NoError(t, serverErr)
		a.Srv().Shutdown()
	}
}
*/

var allPermissionIDs []string

func init() {
	for _, perm := range model.AllPermissions {
		allPermissionIDs = append(allPermissionIDs, perm.ID)
	}
}

func TestUnitUpdateConfig(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	mockStore := th.App.Srv().Store.(*mocks.Store)
	mockUserStore := mocks.UserStore{}
	mockUserStore.On("Count", mock.Anything).Return(int64(10), nil)
	mockPostStore := mocks.PostStore{}
	mockPostStore.On("GetMaxPostSize").Return(65535, nil)
	mockSystemStore := mocks.SystemStore{}
	mockSystemStore.On("GetByName", "UpgradedFromTE").Return(&model.System{Name: "UpgradedFromTE", Value: "false"}, nil)
	mockSystemStore.On("GetByName", "InstallationDate").Return(&model.System{Name: "InstallationDate", Value: "10"}, nil)
	mockSystemStore.On("GetByName", "FirstServerRunTimestamp").Return(&model.System{Name: "FirstServerRunTimestamp", Value: "10"}, nil)
	mockSystemStore.On("Get").Return(make(model.StringMap), nil)
	mockLicenseStore := mocks.LicenseStore{}
	mockLicenseStore.On("Get", "").Return(&model.LicenseRecord{}, nil)
	mockStore.On("User").Return(&mockUserStore)
	mockStore.On("Post").Return(&mockPostStore)
	mockStore.On("System").Return(&mockSystemStore)
	mockStore.On("License").Return(&mockLicenseStore)

	prev := *th.App.Config().ServiceSettings.SiteURL

	th.App.AddConfigListener(func(old, current *model.Config) {
		assert.Equal(t, prev, *old.ServiceSettings.SiteURL)
		assert.Equal(t, "http://foo.com", *current.ServiceSettings.SiteURL)
	})

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.SiteURL = "http://foo.com"
	})
}

func TestDoAdvancedPermissionsMigration(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.ResetRoleMigration()

	th.App.DoAdvancedPermissionsMigration()

	roleNames := []string{
		"system_user",
		"system_admin",
		"team_user",
		"team_admin",
		"channel_user",
		"channel_admin",
		"system_post_all",
		"system_post_all_public",
		"system_user_access_token",
		"team_post_all",
		"team_post_all_public",
	}

	roles1, err1 := th.App.GetRolesByNames(roleNames)
	assert.Nil(t, err1)
	assert.Equal(t, len(roles1), len(roleNames))

	expected1 := map[string][]string{
		"channel_user": {
			model.PermissionReadChannel.ID,
			model.PermissionAddReaction.ID,
			model.PermissionRemoveReaction.ID,
			model.PermissionManagePublicChannelMembers.ID,
			model.PermissionUploadFile.ID,
			model.PermissionGetPublicLink.ID,
			model.PermissionCreatePost.ID,
			model.PermissionUseChannelMentions.ID,
			model.PermissionUseSlashCommands.ID,
			model.PermissionManagePublicChannelProperties.ID,
			model.PermissionDeletePublicChannel.ID,
			model.PermissionManagePrivateChannelProperties.ID,
			model.PermissionDeletePrivateChannel.ID,
			model.PermissionManagePrivateChannelMembers.ID,
			model.PermissionDeletePost.ID,
			model.PermissionEditPost.ID,
		},
		"channel_admin": {
			model.PermissionManageChannelRoles.ID,
			model.PermissionUseGroupMentions.ID,
		},
		"team_user": {
			model.PermissionListTeamChannels.ID,
			model.PermissionJoinPublicChannels.ID,
			model.PermissionReadPublicChannel.ID,
			model.PermissionViewTeam.ID,
			model.PermissionCreatePublicChannel.ID,
			model.PermissionCreatePrivateChannel.ID,
			model.PermissionInviteUser.ID,
			model.PermissionAddUserToTeam.ID,
		},
		"team_post_all": {
			model.PermissionCreatePost.ID,
			model.PermissionUseChannelMentions.ID,
		},
		"team_post_all_public": {
			model.PermissionCreatePostPublic.ID,
			model.PermissionUseChannelMentions.ID,
		},
		"team_admin": {
			model.PermissionRemoveUserFromTeam.ID,
			model.PermissionManageTeam.ID,
			model.PermissionImportTeam.ID,
			model.PermissionManageTeamRoles.ID,
			model.PermissionManageChannelRoles.ID,
			model.PermissionManageOthersIncomingWebhooks.ID,
			model.PermissionManageOthersOutgoingWebhooks.ID,
			model.PermissionManageSlashCommands.ID,
			model.PermissionManageOthersSlashCommands.ID,
			model.PermissionManageIncomingWebhooks.ID,
			model.PermissionManageOutgoingWebhooks.ID,
			model.PermissionConvertPublicChannelToPrivate.ID,
			model.PermissionConvertPrivateChannelToPublic.ID,
			model.PermissionDeletePost.ID,
			model.PermissionDeleteOthersPosts.ID,
		},
		"system_user": {
			model.PermissionListPublicTeams.ID,
			model.PermissionJoinPublicTeams.ID,
			model.PermissionCreateDirectChannel.ID,
			model.PermissionCreateGroupChannel.ID,
			model.PermissionViewMembers.ID,
			model.PermissionCreateTeam.ID,
		},
		"system_post_all": {
			model.PermissionCreatePost.ID,
			model.PermissionUseChannelMentions.ID,
		},
		"system_post_all_public": {
			model.PermissionCreatePostPublic.ID,
			model.PermissionUseChannelMentions.ID,
		},
		"system_user_access_token": {
			model.PermissionCreateUserAccessToken.ID,
			model.PermissionReadUserAccessToken.ID,
			model.PermissionRevokeUserAccessToken.ID,
		},
		"system_admin": allPermissionIDs,
	}
	assert.Contains(t, allPermissionIDs, model.PermissionManageSharedChannels.ID, "manage_shared_channels permission not found")
	assert.Contains(t, allPermissionIDs, model.PermissionManageSecureConnections.ID, "manage_secure_connections permission not found")

	// Check the migration matches what's expected.
	for name, permissions := range expected1 {
		role, err := th.App.GetRoleByName(context.Background(), name)
		assert.Nil(t, err)
		assert.Equal(t, role.Permissions, permissions, fmt.Sprintf("role %q didn't match", name))
	}
	// Add a license and change the policy config.
	restrictPublicChannel := *th.App.Config().TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement
	restrictPrivateChannel := *th.App.Config().TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement

	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement = restrictPublicChannel
		})
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement = restrictPrivateChannel
		})
	}()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement = model.PermissionsTeamAdmin
	})
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement = model.PermissionsTeamAdmin
	})
	th.App.Srv().SetLicense(model.NewTestLicense())

	// Check the migration doesn't change anything if run again.
	th.App.DoAdvancedPermissionsMigration()

	roles2, err2 := th.App.GetRolesByNames(roleNames)
	assert.Nil(t, err2)
	assert.Equal(t, len(roles2), len(roleNames))

	for name, permissions := range expected1 {
		role, err := th.App.GetRoleByName(context.Background(), name)
		assert.Nil(t, err)
		assert.Equal(t, permissions, role.Permissions)
	}

	// Reset the database
	th.ResetRoleMigration()

	// Do the migration again with different policy config settings and a license.
	th.App.DoAdvancedPermissionsMigration()

	// Check the role permissions.
	expected2 := map[string][]string{
		"channel_user": {
			model.PermissionReadChannel.ID,
			model.PermissionAddReaction.ID,
			model.PermissionRemoveReaction.ID,
			model.PermissionManagePublicChannelMembers.ID,
			model.PermissionUploadFile.ID,
			model.PermissionGetPublicLink.ID,
			model.PermissionCreatePost.ID,
			model.PermissionUseChannelMentions.ID,
			model.PermissionUseSlashCommands.ID,
			model.PermissionDeletePublicChannel.ID,
			model.PermissionDeletePrivateChannel.ID,
			model.PermissionManagePrivateChannelMembers.ID,
			model.PermissionDeletePost.ID,
			model.PermissionEditPost.ID,
		},
		"channel_admin": {
			model.PermissionManageChannelRoles.ID,
			model.PermissionUseGroupMentions.ID,
		},
		"team_user": {
			model.PermissionListTeamChannels.ID,
			model.PermissionJoinPublicChannels.ID,
			model.PermissionReadPublicChannel.ID,
			model.PermissionViewTeam.ID,
			model.PermissionCreatePublicChannel.ID,
			model.PermissionCreatePrivateChannel.ID,
			model.PermissionInviteUser.ID,
			model.PermissionAddUserToTeam.ID,
		},
		"team_post_all": {
			model.PermissionCreatePost.ID,
			model.PermissionUseChannelMentions.ID,
		},
		"team_post_all_public": {
			model.PermissionCreatePostPublic.ID,
			model.PermissionUseChannelMentions.ID,
		},
		"team_admin": {
			model.PermissionRemoveUserFromTeam.ID,
			model.PermissionManageTeam.ID,
			model.PermissionImportTeam.ID,
			model.PermissionManageTeamRoles.ID,
			model.PermissionManageChannelRoles.ID,
			model.PermissionManageOthersIncomingWebhooks.ID,
			model.PermissionManageOthersOutgoingWebhooks.ID,
			model.PermissionManageSlashCommands.ID,
			model.PermissionManageOthersSlashCommands.ID,
			model.PermissionManageIncomingWebhooks.ID,
			model.PermissionManageOutgoingWebhooks.ID,
			model.PermissionConvertPublicChannelToPrivate.ID,
			model.PermissionConvertPrivateChannelToPublic.ID,
			model.PermissionManagePublicChannelProperties.ID,
			model.PermissionManagePrivateChannelProperties.ID,
			model.PermissionDeletePost.ID,
			model.PermissionDeleteOthersPosts.ID,
		},
		"system_user": {
			model.PermissionListPublicTeams.ID,
			model.PermissionJoinPublicTeams.ID,
			model.PermissionCreateDirectChannel.ID,
			model.PermissionCreateGroupChannel.ID,
			model.PermissionViewMembers.ID,
			model.PermissionCreateTeam.ID,
		},
		"system_post_all": {
			model.PermissionCreatePost.ID,
			model.PermissionUseChannelMentions.ID,
		},
		"system_post_all_public": {
			model.PermissionCreatePostPublic.ID,
			model.PermissionUseChannelMentions.ID,
		},
		"system_user_access_token": {
			model.PermissionCreateUserAccessToken.ID,
			model.PermissionReadUserAccessToken.ID,
			model.PermissionRevokeUserAccessToken.ID,
		},
		"system_admin": allPermissionIDs,
	}

	roles3, err3 := th.App.GetRolesByNames(roleNames)
	assert.Nil(t, err3)
	assert.Equal(t, len(roles3), len(roleNames))

	for name, permissions := range expected2 {
		role, err := th.App.GetRoleByName(context.Background(), name)
		assert.Nil(t, err)
		assert.Equal(t, permissions, role.Permissions, fmt.Sprintf("'%v' did not have expected permissions", name))
	}

	// Remove the license.
	th.App.Srv().SetLicense(nil)

	// Do the migration again.
	th.ResetRoleMigration()
	th.App.DoAdvancedPermissionsMigration()

	// Check the role permissions.
	roles4, err4 := th.App.GetRolesByNames(roleNames)
	assert.Nil(t, err4)
	assert.Equal(t, len(roles4), len(roleNames))

	for name, permissions := range expected1 {
		role, err := th.App.GetRoleByName(context.Background(), name)
		assert.Nil(t, err)
		assert.Equal(t, permissions, role.Permissions)
	}

	// Check that the config setting for "always" and "time_limit" edit posts is updated correctly.
	th.ResetRoleMigration()

	allowEditPost := *th.App.Config().ServiceSettings.DEPRECATED_DO_NOT_USE_AllowEditPost
	postEditTimeLimit := *th.App.Config().ServiceSettings.PostEditTimeLimit

	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_AllowEditPost = allowEditPost })
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.PostEditTimeLimit = postEditTimeLimit })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_AllowEditPost = "always"
		*cfg.ServiceSettings.PostEditTimeLimit = 300
	})

	th.App.DoAdvancedPermissionsMigration()

	config := th.App.Config()
	assert.Equal(t, -1, *config.ServiceSettings.PostEditTimeLimit)

	th.ResetRoleMigration()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_AllowEditPost = "time_limit"
		*cfg.ServiceSettings.PostEditTimeLimit = 300
	})

	th.App.DoAdvancedPermissionsMigration()
	config = th.App.Config()
	assert.Equal(t, 300, *config.ServiceSettings.PostEditTimeLimit)
}

func TestDoEmojisPermissionsMigration(t *testing.T) {
	th := SetupWithoutPreloadMigrations(t)
	defer th.TearDown()

	// Add a license and change the policy config.
	restrictCustomEmojiCreation := *th.App.Config().ServiceSettings.DEPRECATED_DO_NOT_USE_RestrictCustomEmojiCreation

	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_RestrictCustomEmojiCreation = restrictCustomEmojiCreation
		})
	}()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_RestrictCustomEmojiCreation = model.RestrictEmojiCreationSystemAdmin
	})

	th.ResetEmojisMigration()
	th.App.DoEmojisPermissionsMigration()

	expectedSystemAdmin := allPermissionIDs
	sort.Strings(expectedSystemAdmin)

	role1, err1 := th.App.GetRoleByName(context.Background(), model.SystemAdminRoleID)
	assert.Nil(t, err1)
	sort.Strings(role1.Permissions)
	assert.Equal(t, expectedSystemAdmin, role1.Permissions, fmt.Sprintf("'%v' did not have expected permissions", model.SystemAdminRoleID))

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_RestrictCustomEmojiCreation = model.RestrictEmojiCreationAdmin
	})

	th.ResetEmojisMigration()
	th.App.DoEmojisPermissionsMigration()

	role2, err2 := th.App.GetRoleByName(context.Background(), model.TeamAdminRoleID)
	assert.Nil(t, err2)
	expected2 := []string{
		model.PermissionRemoveUserFromTeam.ID,
		model.PermissionManageTeam.ID,
		model.PermissionImportTeam.ID,
		model.PermissionManageTeamRoles.ID,
		model.PermissionReadPublicChannelGroups.ID,
		model.PermissionReadPrivateChannelGroups.ID,
		model.PermissionManageChannelRoles.ID,
		model.PermissionManageOthersIncomingWebhooks.ID,
		model.PermissionManageOthersOutgoingWebhooks.ID,
		model.PermissionManageSlashCommands.ID,
		model.PermissionManageOthersSlashCommands.ID,
		model.PermissionManageIncomingWebhooks.ID,
		model.PermissionManageOutgoingWebhooks.ID,
		model.PermissionDeletePost.ID,
		model.PermissionDeleteOthersPosts.ID,
		model.PermissionCreateEmojis.ID,
		model.PermissionDeleteEmojis.ID,
		model.PermissionAddReaction.ID,
		model.PermissionCreatePost.ID,
		model.PermissionManagePublicChannelMembers.ID,
		model.PermissionManagePrivateChannelMembers.ID,
		model.PermissionRemoveReaction.ID,
		model.PermissionUseChannelMentions.ID,
		model.PermissionUseGroupMentions.ID,
		model.PermissionConvertPublicChannelToPrivate.ID,
		model.PermissionConvertPrivateChannelToPublic.ID,
	}
	sort.Strings(expected2)
	sort.Strings(role2.Permissions)
	assert.Equal(t, expected2, role2.Permissions, fmt.Sprintf("'%v' did not have expected permissions", model.TeamAdminRoleID))

	systemAdmin1, systemAdminErr1 := th.App.GetRoleByName(context.Background(), model.SystemAdminRoleID)
	assert.Nil(t, systemAdminErr1)
	sort.Strings(systemAdmin1.Permissions)
	assert.Equal(t, expectedSystemAdmin, systemAdmin1.Permissions, fmt.Sprintf("'%v' did not have expected permissions", model.SystemAdminRoleID))

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_RestrictCustomEmojiCreation = model.RestrictEmojiCreationAll
	})

	th.ResetEmojisMigration()
	th.App.DoEmojisPermissionsMigration()

	role3, err3 := th.App.GetRoleByName(context.Background(), model.SystemUserRoleID)
	assert.Nil(t, err3)
	expected3 := []string{
		model.PermissionListPublicTeams.ID,
		model.PermissionJoinPublicTeams.ID,
		model.PermissionCreateDirectChannel.ID,
		model.PermissionCreateGroupChannel.ID,
		model.PermissionCreateTeam.ID,
		model.PermissionCreateEmojis.ID,
		model.PermissionDeleteEmojis.ID,
		model.PermissionViewMembers.ID,
	}
	sort.Strings(expected3)
	sort.Strings(role3.Permissions)
	assert.Equal(t, expected3, role3.Permissions, fmt.Sprintf("'%v' did not have expected permissions", model.SystemUserRoleID))

	systemAdmin2, systemAdminErr2 := th.App.GetRoleByName(context.Background(), model.SystemAdminRoleID)
	assert.Nil(t, systemAdminErr2)
	sort.Strings(systemAdmin2.Permissions)
	assert.Equal(t, expectedSystemAdmin, systemAdmin2.Permissions, fmt.Sprintf("'%v' did not have expected permissions", model.SystemAdminRoleID))
}

func TestDBHealthCheckWriteAndDelete(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	expectedKey := "health_check_" + th.App.GetClusterID()
	assert.Equal(t, expectedKey, th.App.dbHealthCheckKey())

	_, err := th.App.Srv().Store.System().GetByName(expectedKey)
	assert.Error(t, err)

	err = th.App.DBHealthCheckWrite()
	assert.NoError(t, err)

	systemVal, err := th.App.Srv().Store.System().GetByName(expectedKey)
	assert.NoError(t, err)
	assert.NotNil(t, systemVal)

	err = th.App.DBHealthCheckDelete()
	assert.NoError(t, err)

	_, err = th.App.Srv().Store.System().GetByName(expectedKey)
	assert.Error(t, err)
}
