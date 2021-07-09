// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package utils

import (
	"github.com/mattermost/mattermost-server/v5/model"
)

func SetRolePermissionsFromConfig(roles map[string]*model.Role, cfg *model.Config, isLicensed bool) map[string]*model.Role {
	if isLicensed {
		switch *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPublicChannelCreation {
		case model.PermissionsAll:
			roles[model.TeamUserRoleID].Permissions = append(
				roles[model.TeamUserRoleID].Permissions,
				model.PermissionCreatePublicChannel.ID,
			)
		case model.PermissionsTeamAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionCreatePublicChannel.ID,
			)
		}
	} else {
		roles[model.TeamUserRoleID].Permissions = append(
			roles[model.TeamUserRoleID].Permissions,
			model.PermissionCreatePublicChannel.ID,
		)
	}

	if isLicensed {
		switch *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPublicChannelManagement {
		case model.PermissionsAll:
			roles[model.ChannelUserRoleID].Permissions = append(
				roles[model.ChannelUserRoleID].Permissions,
				model.PermissionManagePublicChannelProperties.ID,
			)
		case model.PermissionsChannelAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionManagePublicChannelProperties.ID,
			)
			roles[model.ChannelAdminRoleID].Permissions = append(
				roles[model.ChannelAdminRoleID].Permissions,
				model.PermissionManagePublicChannelProperties.ID,
			)
		case model.PermissionsTeamAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionManagePublicChannelProperties.ID,
			)
		}
	} else {
		roles[model.ChannelUserRoleID].Permissions = append(
			roles[model.ChannelUserRoleID].Permissions,
			model.PermissionManagePublicChannelProperties.ID,
		)
	}

	if isLicensed {
		switch *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPublicChannelDeletion {
		case model.PermissionsAll:
			roles[model.ChannelUserRoleID].Permissions = append(
				roles[model.ChannelUserRoleID].Permissions,
				model.PermissionDeletePublicChannel.ID,
			)
		case model.PermissionsChannelAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionDeletePublicChannel.ID,
			)
			roles[model.ChannelAdminRoleID].Permissions = append(
				roles[model.ChannelAdminRoleID].Permissions,
				model.PermissionDeletePublicChannel.ID,
			)
		case model.PermissionsTeamAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionDeletePublicChannel.ID,
			)
		}
	} else {
		roles[model.ChannelUserRoleID].Permissions = append(
			roles[model.ChannelUserRoleID].Permissions,
			model.PermissionDeletePublicChannel.ID,
		)
	}

	if isLicensed {
		switch *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelCreation {
		case model.PermissionsAll:
			roles[model.TeamUserRoleID].Permissions = append(
				roles[model.TeamUserRoleID].Permissions,
				model.PermissionCreatePrivateChannel.ID,
			)
		case model.PermissionsTeamAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionCreatePrivateChannel.ID,
			)
		}
	} else {
		roles[model.TeamUserRoleID].Permissions = append(
			roles[model.TeamUserRoleID].Permissions,
			model.PermissionCreatePrivateChannel.ID,
		)
	}

	if isLicensed {
		switch *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManagement {
		case model.PermissionsAll:
			roles[model.ChannelUserRoleID].Permissions = append(
				roles[model.ChannelUserRoleID].Permissions,
				model.PermissionManagePrivateChannelProperties.ID,
			)
		case model.PermissionsChannelAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionManagePrivateChannelProperties.ID,
			)
			roles[model.ChannelAdminRoleID].Permissions = append(
				roles[model.ChannelAdminRoleID].Permissions,
				model.PermissionManagePrivateChannelProperties.ID,
			)
		case model.PermissionsTeamAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionManagePrivateChannelProperties.ID,
			)
		}
	} else {
		roles[model.ChannelUserRoleID].Permissions = append(
			roles[model.ChannelUserRoleID].Permissions,
			model.PermissionManagePrivateChannelProperties.ID,
		)
	}

	if isLicensed {
		switch *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelDeletion {
		case model.PermissionsAll:
			roles[model.ChannelUserRoleID].Permissions = append(
				roles[model.ChannelUserRoleID].Permissions,
				model.PermissionDeletePrivateChannel.ID,
			)
		case model.PermissionsChannelAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionDeletePrivateChannel.ID,
			)
			roles[model.ChannelAdminRoleID].Permissions = append(
				roles[model.ChannelAdminRoleID].Permissions,
				model.PermissionDeletePrivateChannel.ID,
			)
		case model.PermissionsTeamAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionDeletePrivateChannel.ID,
			)
		}
	} else {
		roles[model.ChannelUserRoleID].Permissions = append(
			roles[model.ChannelUserRoleID].Permissions,
			model.PermissionDeletePrivateChannel.ID,
		)
	}

	// Restrict permissions for Private Channel Manage Members
	if isLicensed {
		switch *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictPrivateChannelManageMembers {
		case model.PermissionsAll:
			roles[model.ChannelUserRoleID].Permissions = append(
				roles[model.ChannelUserRoleID].Permissions,
				model.PermissionManagePrivateChannelMembers.ID,
			)
		case model.PermissionsChannelAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionManagePrivateChannelMembers.ID,
			)
			roles[model.ChannelAdminRoleID].Permissions = append(
				roles[model.ChannelAdminRoleID].Permissions,
				model.PermissionManagePrivateChannelMembers.ID,
			)
		case model.PermissionsTeamAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionManagePrivateChannelMembers.ID,
			)
		}
	} else {
		roles[model.ChannelUserRoleID].Permissions = append(
			roles[model.ChannelUserRoleID].Permissions,
			model.PermissionManagePrivateChannelMembers.ID,
		)
	}

	if !*cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_EnableOnlyAdminIntegrations {
		roles[model.TeamUserRoleID].Permissions = append(
			roles[model.TeamUserRoleID].Permissions,
			model.PermissionManageIncomingWebhooks.ID,
			model.PermissionManageOutgoingWebhooks.ID,
			model.PermissionManageSlashCommands.ID,
		)
		roles[model.SystemUserRoleID].Permissions = append(
			roles[model.SystemUserRoleID].Permissions,
			model.PermissionManageOAuth.ID,
		)
	}

	// Grant permissions for inviting and adding users to a team.
	if isLicensed {
		if *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictTeamInvite == model.PermissionsTeamAdmin {
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionInviteUser.ID,
				model.PermissionAddUserToTeam.ID,
			)
		} else if *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_RestrictTeamInvite == model.PermissionsAll {
			roles[model.TeamUserRoleID].Permissions = append(
				roles[model.TeamUserRoleID].Permissions,
				model.PermissionInviteUser.ID,
				model.PermissionAddUserToTeam.ID,
			)
		}
	} else {
		roles[model.TeamUserRoleID].Permissions = append(
			roles[model.TeamUserRoleID].Permissions,
			model.PermissionInviteUser.ID,
			model.PermissionAddUserToTeam.ID,
		)
	}

	if isLicensed {
		switch *cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_RestrictPostDelete {
		case model.PermissionsDeletePostAll:
			roles[model.ChannelUserRoleID].Permissions = append(
				roles[model.ChannelUserRoleID].Permissions,
				model.PermissionDeletePost.ID,
			)
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionDeletePost.ID,
				model.PermissionDeleteOthersPosts.ID,
			)
		case model.PermissionsDeletePostTeamAdmin:
			roles[model.TeamAdminRoleID].Permissions = append(
				roles[model.TeamAdminRoleID].Permissions,
				model.PermissionDeletePost.ID,
				model.PermissionDeleteOthersPosts.ID,
			)
		}
	} else {
		roles[model.ChannelUserRoleID].Permissions = append(
			roles[model.ChannelUserRoleID].Permissions,
			model.PermissionDeletePost.ID,
		)
		roles[model.TeamAdminRoleID].Permissions = append(
			roles[model.TeamAdminRoleID].Permissions,
			model.PermissionDeletePost.ID,
			model.PermissionDeleteOthersPosts.ID,
		)
	}

	if *cfg.TeamSettings.DEPRECATED_DO_NOT_USE_EnableTeamCreation {
		roles[model.SystemUserRoleID].Permissions = append(
			roles[model.SystemUserRoleID].Permissions,
			model.PermissionCreateTeam.ID,
		)
	}

	if isLicensed {
		switch *cfg.ServiceSettings.DEPRECATED_DO_NOT_USE_AllowEditPost {
		case model.AllowEditPostAlways, model.AllowEditPostTimeLimit:
			roles[model.ChannelUserRoleID].Permissions = append(
				roles[model.ChannelUserRoleID].Permissions,
				model.PermissionEditPost.ID,
			)
			roles[model.SystemAdminRoleID].Permissions = append(
				roles[model.SystemAdminRoleID].Permissions,
				model.PermissionEditPost.ID,
			)
		}
	} else {
		roles[model.ChannelUserRoleID].Permissions = append(
			roles[model.ChannelUserRoleID].Permissions,
			model.PermissionEditPost.ID,
		)
		roles[model.SystemAdminRoleID].Permissions = append(
			roles[model.SystemAdminRoleID].Permissions,
			model.PermissionEditPost.ID,
		)
	}

	return roles
}
