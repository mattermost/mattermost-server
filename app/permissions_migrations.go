// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type permissionTransformation struct {
	On     func(*model.Role, map[string]map[string]bool) bool
	Add    []string
	Remove []string
}
type permissionsMap []permissionTransformation

const (
	PermissionManageSystem                   = "manage_system"
	PermissionManageTeam                     = "manage_team"
	PermissionManageEmojis                   = "manage_emojis"
	PermissionManageOthersEmojis             = "manage_others_emojis"
	PermissionCreateEmojis                   = "create_emojis"
	PermissionDeleteEmojis                   = "delete_emojis"
	PermissionDeleteOthersEmojis             = "delete_others_emojis"
	PermissionManageWebhooks                 = "manage_webhooks"
	PermissionManageOthersWebhooks           = "manage_others_webhooks"
	PermissionManageIncomingWebhooks         = "manage_incoming_webhooks"
	PermissionManageOthersIncomingWebhooks   = "manage_others_incoming_webhooks"
	PermissionManageOutgoingWebhooks         = "manage_outgoing_webhooks"
	PermissionManageOthersOutgoingWebhooks   = "manage_others_outgoing_webhooks"
	PermissionListPublicTeams                = "list_public_teams"
	PermissionListPrivateTeams               = "list_private_teams"
	PermissionJoinPublicTeams                = "join_public_teams"
	PermissionJoinPrivateTeams               = "join_private_teams"
	PermissionPermanentDeleteUser            = "permanent_delete_user"
	PermissionCreateBot                      = "create_bot"
	PermissionReadBots                       = "read_bots"
	PermissionReadOthersBots                 = "read_others_bots"
	PermissionManageBots                     = "manage_bots"
	PermissionManageOthersBots               = "manage_others_bots"
	PermissionDeletePublicChannel            = "delete_public_channel"
	PermissionDeletePrivateChannel           = "delete_private_channel"
	PermissionManagePublicChannelProperties  = "manage_public_channel_properties"
	PermissionManagePrivateChannelProperties = "manage_private_channel_properties"
	PermissionConvertPublicChannelToPrivate  = "convert_public_channel_to_private"
	PermissionConvertPrivateChannelToPublic  = "convert_private_channel_to_public"
	PermissionViewMembers                    = "view_members"
	PermissionInviteUser                     = "invite_user"
	PermissionInviteGuest                    = "invite_guest"
	PermissionPromoteGuest                   = "promote_guest"
	PermissionDemoteToGuest                  = "demote_to_guest"
	PermissionUseChannelMentions             = "use_channel_mentions"
	PermissionCreatePost                     = "create_post"
	PermissionCreatePost_PUBLIC              = "create_post_public"
	PermissionUseGroupMentions               = "use_group_mentions"
	PermissionAddReaction                    = "add_reaction"
	PermissionRemoveReaction                 = "remove_reaction"
	PermissionManagePublicChannelMembers     = "manage_public_channel_members"
	PermissionManagePrivateChannelMembers    = "manage_private_channel_members"
	PermissionReadJobs                       = "read_jobs"
	PermissionManageJobs                     = "manage_jobs"
	PermissionReadOtherUsersTeams            = "read_other_users_teams"
	PermissionEditOtherUsers                 = "edit_other_users"
	PermissionReadPublicChannelGroups        = "read_public_channel_groups"
	PermissionReadPrivateChannelGroups       = "read_private_channel_groups"
	PermissionEditBrand                      = "edit_brand"
	PermissionManageSharedChannels           = "manage_shared_channels"
	PermissionManageSecureConnections        = "manage_secure_connections"
	PermissionManageRemoteClusters           = "manage_remote_clusters" // deprecated; use `manage_secure_connections`
)

func isRole(roleName string) func(*model.Role, map[string]map[string]bool) bool {
	return func(role *model.Role, permissionsMap map[string]map[string]bool) bool {
		return role.Name == roleName
	}
}

func isNotRole(roleName string) func(*model.Role, map[string]map[string]bool) bool {
	return func(role *model.Role, permissionsMap map[string]map[string]bool) bool {
		return role.Name != roleName
	}
}

func isNotSchemeRole(roleName string) func(*model.Role, map[string]map[string]bool) bool {
	return func(role *model.Role, permissionsMap map[string]map[string]bool) bool {
		return !strings.Contains(role.DisplayName, roleName)
	}
}

func permissionExists(permission string) func(*model.Role, map[string]map[string]bool) bool {
	return func(role *model.Role, permissionsMap map[string]map[string]bool) bool {
		val, ok := permissionsMap[role.Name][permission]
		return ok && val
	}
}

func permissionNotExists(permission string) func(*model.Role, map[string]map[string]bool) bool {
	return func(role *model.Role, permissionsMap map[string]map[string]bool) bool {
		val, ok := permissionsMap[role.Name][permission]
		return !(ok && val)
	}
}

func onOtherRole(otherRole string, function func(*model.Role, map[string]map[string]bool) bool) func(*model.Role, map[string]map[string]bool) bool {
	return func(role *model.Role, permissionsMap map[string]map[string]bool) bool {
		return function(&model.Role{Name: otherRole}, permissionsMap)
	}
}

func permissionOr(funcs ...func(*model.Role, map[string]map[string]bool) bool) func(*model.Role, map[string]map[string]bool) bool {
	return func(role *model.Role, permissionsMap map[string]map[string]bool) bool {
		for _, f := range funcs {
			if f(role, permissionsMap) {
				return true
			}
		}
		return false
	}
}

func permissionAnd(funcs ...func(*model.Role, map[string]map[string]bool) bool) func(*model.Role, map[string]map[string]bool) bool {
	return func(role *model.Role, permissionsMap map[string]map[string]bool) bool {
		for _, f := range funcs {
			if !f(role, permissionsMap) {
				return false
			}
		}
		return true
	}
}

func applyPermissionsMap(role *model.Role, roleMap map[string]map[string]bool, migrationMap permissionsMap) []string {
	var result []string

	roleName := role.Name
	for _, transformation := range migrationMap {
		if transformation.On(role, roleMap) {
			for _, permission := range transformation.Add {
				roleMap[roleName][permission] = true
			}
			for _, permission := range transformation.Remove {
				roleMap[roleName][permission] = false
			}
		}
	}

	for key, active := range roleMap[roleName] {
		if active {
			result = append(result, key)
		}
	}
	return result
}

func (s *Server) doPermissionsMigration(key string, migrationMap permissionsMap, roles []*model.Role) *model.AppError {
	if _, err := s.Store.System().GetByName(key); err == nil {
		return nil
	}

	roleMap := make(map[string]map[string]bool)
	for _, role := range roles {
		roleMap[role.Name] = make(map[string]bool)
		for _, permission := range role.Permissions {
			roleMap[role.Name][permission] = true
		}
	}

	for _, role := range roles {
		role.Permissions = applyPermissionsMap(role, roleMap, migrationMap)
		if _, err := s.Store.Role().Save(role); err != nil {
			var invErr *store.ErrInvalidInput
			switch {
			case errors.As(err, &invErr):
				return model.NewAppError("doPermissionsMigration", "app.role.save.invalid_role.app_error", nil, invErr.Error(), http.StatusBadRequest)
			default:
				return model.NewAppError("doPermissionsMigration", "app.role.save.insert.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	if err := s.Store.System().Save(&model.System{Name: key, Value: "true"}); err != nil {
		return model.NewAppError("doPermissionsMigration", "app.system.save.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (a *App) getEmojisPermissionsSplitMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:     permissionExists(PermissionManageEmojis),
			Add:    []string{PermissionCreateEmojis, PermissionDeleteEmojis},
			Remove: []string{PermissionManageEmojis},
		},
		permissionTransformation{
			On:     permissionExists(PermissionManageOthersEmojis),
			Add:    []string{PermissionDeleteOthersEmojis},
			Remove: []string{PermissionManageOthersEmojis},
		},
	}, nil
}

func (a *App) getWebhooksPermissionsSplitMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:     permissionExists(PermissionManageWebhooks),
			Add:    []string{PermissionManageIncomingWebhooks, PermissionManageOutgoingWebhooks},
			Remove: []string{PermissionManageWebhooks},
		},
		permissionTransformation{
			On:     permissionExists(PermissionManageOthersWebhooks),
			Add:    []string{PermissionManageOthersIncomingWebhooks, PermissionManageOthersOutgoingWebhooks},
			Remove: []string{PermissionManageOthersWebhooks},
		},
	}, nil
}

func (a *App) getListJoinPublicPrivateTeamsPermissionsMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:     isRole(model.SystemAdminRoleID),
			Add:    []string{PermissionListPrivateTeams, PermissionJoinPrivateTeams},
			Remove: []string{},
		},
		permissionTransformation{
			On:     isRole(model.SystemUserRoleID),
			Add:    []string{PermissionListPublicTeams, PermissionJoinPublicTeams},
			Remove: []string{},
		},
	}, nil
}

func (a *App) removePermanentDeleteUserMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:     permissionExists(PermissionPermanentDeleteUser),
			Remove: []string{PermissionPermanentDeleteUser},
		},
	}, nil
}

func (a *App) getAddBotPermissionsMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:     isRole(model.SystemAdminRoleID),
			Add:    []string{PermissionCreateBot, PermissionReadBots, PermissionReadOthersBots, PermissionManageBots, PermissionManageOthersBots},
			Remove: []string{},
		},
	}, nil
}

func (a *App) applyChannelManageDeleteToChannelUser() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:  permissionAnd(isRole(model.ChannelUserRoleID), onOtherRole(model.TeamUserRoleID, permissionExists(PermissionManagePrivateChannelProperties))),
			Add: []string{PermissionManagePrivateChannelProperties},
		},
		permissionTransformation{
			On:  permissionAnd(isRole(model.ChannelUserRoleID), onOtherRole(model.TeamUserRoleID, permissionExists(PermissionDeletePrivateChannel))),
			Add: []string{PermissionDeletePrivateChannel},
		},
		permissionTransformation{
			On:  permissionAnd(isRole(model.ChannelUserRoleID), onOtherRole(model.TeamUserRoleID, permissionExists(PermissionManagePublicChannelProperties))),
			Add: []string{PermissionManagePublicChannelProperties},
		},
		permissionTransformation{
			On:  permissionAnd(isRole(model.ChannelUserRoleID), onOtherRole(model.TeamUserRoleID, permissionExists(PermissionDeletePublicChannel))),
			Add: []string{PermissionDeletePublicChannel},
		},
	}, nil
}

func (a *App) removeChannelManageDeleteFromTeamUser() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:     permissionAnd(isRole(model.TeamUserRoleID), permissionExists(PermissionManagePrivateChannelProperties)),
			Remove: []string{PermissionManagePrivateChannelProperties},
		},
		permissionTransformation{
			On:     permissionAnd(isRole(model.TeamUserRoleID), permissionExists(PermissionDeletePrivateChannel)),
			Remove: []string{model.PermissionDeletePrivateChannel.ID},
		},
		permissionTransformation{
			On:     permissionAnd(isRole(model.TeamUserRoleID), permissionExists(PermissionManagePublicChannelProperties)),
			Remove: []string{PermissionManagePublicChannelProperties},
		},
		permissionTransformation{
			On:     permissionAnd(isRole(model.TeamUserRoleID), permissionExists(PermissionDeletePublicChannel)),
			Remove: []string{PermissionDeletePublicChannel},
		},
	}, nil
}

func (a *App) getViewMembersPermissionMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:  isRole(model.SystemUserRoleID),
			Add: []string{PermissionViewMembers},
		},
		permissionTransformation{
			On:  isRole(model.SystemAdminRoleID),
			Add: []string{PermissionViewMembers},
		},
	}, nil
}

func (a *App) getAddManageGuestsPermissionsMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:  isRole(model.SystemAdminRoleID),
			Add: []string{PermissionPromoteGuest, PermissionDemoteToGuest, PermissionInviteGuest},
		},
	}, nil
}

func (a *App) channelModerationPermissionsMigration() (permissionsMap, error) {
	transformations := permissionsMap{}

	var allTeamSchemes []*model.Scheme
	next := a.SchemesIterator(model.SchemeScopeTeam, 100)
	var schemeBatch []*model.Scheme
	for schemeBatch = next(); len(schemeBatch) > 0; schemeBatch = next() {
		allTeamSchemes = append(allTeamSchemes, schemeBatch...)
	}

	moderatedPermissionsMinusCreatePost := []string{
		PermissionAddReaction,
		PermissionRemoveReaction,
		PermissionManagePublicChannelMembers,
		PermissionManagePrivateChannelMembers,
		PermissionUseChannelMentions,
	}

	teamAndChannelAdminConditionalTransformations := func(teamAdminID, channelAdminID, channelUserID, channelGuestID string) []permissionTransformation {
		transformations := []permissionTransformation{}

		for _, perm := range moderatedPermissionsMinusCreatePost {
			// add each moderated permission to the channel admin if channel user or guest has the permission
			trans := permissionTransformation{
				On: permissionAnd(
					isRole(channelAdminID),
					permissionOr(
						onOtherRole(channelUserID, permissionExists(perm)),
						onOtherRole(channelGuestID, permissionExists(perm)),
					),
				),
				Add: []string{perm},
			}
			transformations = append(transformations, trans)

			// add each moderated permission to the team admin if channel admin, user, or guest has the permission
			trans = permissionTransformation{
				On: permissionAnd(
					isRole(teamAdminID),
					permissionOr(
						onOtherRole(channelAdminID, permissionExists(perm)),
						onOtherRole(channelUserID, permissionExists(perm)),
						onOtherRole(channelGuestID, permissionExists(perm)),
					),
				),
				Add: []string{perm},
			}
			transformations = append(transformations, trans)
		}

		return transformations
	}

	for _, ts := range allTeamSchemes {
		// ensure all team scheme channel admins have create_post because it's not exposed via the UI
		trans := permissionTransformation{
			On:  isRole(ts.DefaultChannelAdminRole),
			Add: []string{PermissionCreatePost},
		}
		transformations = append(transformations, trans)

		// ensure all team scheme team admins have create_post because it's not exposed via the UI
		trans = permissionTransformation{
			On:  isRole(ts.DefaultTeamAdminRole),
			Add: []string{PermissionCreatePost},
		}
		transformations = append(transformations, trans)

		// conditionally add all other moderated permissions to team and channel admins
		transformations = append(transformations, teamAndChannelAdminConditionalTransformations(
			ts.DefaultTeamAdminRole,
			ts.DefaultChannelAdminRole,
			ts.DefaultChannelUserRole,
			ts.DefaultChannelGuestRole,
		)...)
	}

	// ensure team admins have create_post
	transformations = append(transformations, permissionTransformation{
		On:  isRole(model.TeamAdminRoleID),
		Add: []string{PermissionCreatePost},
	})

	// ensure channel admins have create_post
	transformations = append(transformations, permissionTransformation{
		On:  isRole(model.ChannelAdminRoleID),
		Add: []string{PermissionCreatePost},
	})

	// conditionally add all other moderated permissions to team and channel admins
	transformations = append(transformations, teamAndChannelAdminConditionalTransformations(
		model.TeamAdminRoleID,
		model.ChannelAdminRoleID,
		model.ChannelUserRoleID,
		model.ChannelGuestRoleID,
	)...)

	// ensure system admin has all of the moderated permissions
	transformations = append(transformations, permissionTransformation{
		On:  isRole(model.SystemAdminRoleID),
		Add: append(moderatedPermissionsMinusCreatePost, PermissionCreatePost),
	})

	// add the new use_channel_mentions permission to everyone who has create_post
	transformations = append(transformations, permissionTransformation{
		On:  permissionOr(permissionExists(PermissionCreatePost), permissionExists(PermissionCreatePost_PUBLIC)),
		Add: []string{PermissionUseChannelMentions},
	})

	return transformations, nil
}

func (a *App) getAddUseGroupMentionsPermissionMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On: permissionAnd(
				isNotRole(model.ChannelGuestRoleID),
				isNotSchemeRole("Channel Guest Role for Scheme"),
				permissionOr(permissionExists(PermissionCreatePost), permissionExists(PermissionCreatePost_PUBLIC)),
			),
			Add: []string{PermissionUseGroupMentions},
		},
	}, nil
}

func (a *App) getAddSystemConsolePermissionsMigration() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsToAdd := []string{}
	for _, permission := range append(model.SysconsoleReadPermissions, model.SysconsoleWritePermissions...) {
		permissionsToAdd = append(permissionsToAdd, permission.ID)
	}

	// add the new permissions to system admin
	transformations = append(transformations,
		permissionTransformation{
			On:  isRole(model.SystemAdminRoleID),
			Add: permissionsToAdd,
		})

	// add read_jobs to all roles with manage_jobs
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(PermissionManageJobs),
		Add: []string{PermissionReadJobs},
	})

	// add read_other_users_teams to all roles with edit_other_users
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(PermissionEditOtherUsers),
		Add: []string{PermissionReadOtherUsersTeams},
	})

	// add read_public_channel_groups to all roles with manage_public_channel_members
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(PermissionManagePublicChannelMembers),
		Add: []string{PermissionReadPublicChannelGroups},
	})

	// add read_private_channel_groups to all roles with manage_private_channel_members
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(PermissionManagePrivateChannelMembers),
		Add: []string{PermissionReadPrivateChannelGroups},
	})

	// add edit_brand to all roles with manage_system
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(PermissionManageSystem),
		Add: []string{PermissionEditBrand},
	})

	return transformations, nil
}

func (a *App) getAddConvertChannelPermissionsMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:  permissionExists(PermissionManageTeam),
			Add: []string{PermissionConvertPublicChannelToPrivate, PermissionConvertPrivateChannelToPublic},
		},
	}, nil
}

func (a *App) getSystemRolesPermissionsMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:  isRole(model.SystemAdminRoleID),
			Add: []string{model.PermissionSysconsoleReadUserManagementSystemRoles.ID, model.PermissionSysconsoleWriteUserManagementSystemRoles.ID},
		},
	}, nil
}

func (a *App) getAddManageSharedChannelsPermissionsMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:  isRole(model.SystemAdminRoleID),
			Add: []string{PermissionManageSharedChannels},
		},
	}, nil
}

func (a *App) getBillingPermissionsMigration() (permissionsMap, error) {
	return permissionsMap{
		permissionTransformation{
			On:  isRole(model.SystemAdminRoleID),
			Add: []string{model.PermissionSysconsoleReadBilling.ID, model.PermissionSysconsoleWriteBilling.ID},
		},
	}, nil
}

func (a *App) getAddManageSecureConnectionsPermissionsMigration() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	// add the new permission to system admin
	transformations = append(transformations,
		permissionTransformation{
			On:  isRole(model.SystemAdminRoleID),
			Add: []string{PermissionManageSecureConnections},
		})

	// remote the decprecated permission from system admin
	transformations = append(transformations,
		permissionTransformation{
			On:     isRole(model.SystemAdminRoleID),
			Remove: []string{PermissionManageRemoteClusters},
		})

	return transformations, nil
}

func (a *App) getAddDownloadComplianceExportResult() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsToAddComplianceRead := []string{model.PermissionDownloadComplianceExportResult.ID, model.PermissionReadDataRetentionJob.ID}
	permissionsToAddComplianceWrite := []string{model.PermissionManageJobs.ID}

	// add the new permissions to system admin
	transformations = append(transformations,
		permissionTransformation{
			On:  isRole(model.SystemAdminRoleID),
			Add: []string{model.PermissionDownloadComplianceExportResult.ID},
		})

	// add Download Compliance Export Result and Read Jobs to all roles with sysconsole_read_compliance
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadCompliance.ID),
		Add: permissionsToAddComplianceRead,
	})

	// add manage_jobs to all roles with sysconsole_write_compliance
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteCompliance.ID),
		Add: permissionsToAddComplianceWrite,
	})

	return transformations, nil
}

func (a *App) getAddExperimentalSubsectionPermissions() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsExperimentalRead := []string{model.PermissionSysconsoleReadExperimentalBleve.ID, model.PermissionSysconsoleReadExperimentalFeatures.ID, model.PermissionSysconsoleReadExperimentalFeatureFlags.ID}
	permissionsExperimentalWrite := []string{model.PermissionSysconsoleWriteExperimentalBleve.ID, model.PermissionSysconsoleWriteExperimentalFeatures.ID, model.PermissionSysconsoleWriteExperimentalFeatureFlags.ID}

	// Give the new subsection READ permissions to any user with READ_EXPERIMENTAL
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadExperimental.ID),
		Add: permissionsExperimentalRead,
	})

	// Give the new subsection WRITE permissions to any user with WRITE_EXPERIMENTAL
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteExperimental.ID),
		Add: permissionsExperimentalWrite,
	})

	// Give the ancillary permissions MANAGE_JOBS and PURGE_BLEVE_INDEXES to anyone with WRITE_EXPERIMENTAL_BLEVE
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteExperimentalBleve.ID),
		Add: []string{model.PermissionCreatePostBleveIndexesJob.ID, model.PermissionPurgeBleveIndexes.ID},
	})

	return transformations, nil
}

func (a *App) getAddIntegrationsSubsectionPermissions() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsIntegrationsRead := []string{model.PermissionSysconsoleReadIntegrationsIntegrationManagement.ID, model.PermissionSysconsoleReadIntegrationsBotAccounts.ID, model.PermissionSysconsoleReadIntegrationsGif.ID, model.PermissionSysconsoleReadIntegrationsCors.ID}
	permissionsIntegrationsWrite := []string{model.PermissionSysconsoleWriteIntegrationsIntegrationManagement.ID, model.PermissionSysconsoleWriteIntegrationsBotAccounts.ID, model.PermissionSysconsoleWriteIntegrationsGif.ID, model.PermissionSysconsoleWriteIntegrationsCors.ID}

	// Give the new subsection READ permissions to any user with READ_INTEGRATIONS
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadIntegrations.ID),
		Add: permissionsIntegrationsRead,
	})

	// Give the new subsection WRITE permissions to any user with WRITE_EXPERIMENTAL
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteIntegrations.ID),
		Add: permissionsIntegrationsWrite,
	})

	return transformations, nil
}

func (a *App) getAddSiteSubsectionPermissions() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsSiteRead := []string{model.PermissionSysconsoleReadSiteCustomization.ID, model.PermissionSysconsoleReadSiteLocalization.ID, model.PermissionSysconsoleReadSiteUsersAndTeams.ID, model.PermissionSysconsoleReadSiteNotifications.ID, model.PermissionSysconsoleReadSiteAnnouncementBanner.ID, model.PermissionSysconsoleReadSiteEmoji.ID, model.PermissionSysconsoleReadSitePosts.ID, model.PermissionSysconsoleReadSiteFileSharingAndDownloads.ID, model.PermissionSysconsoleReadSitePublicLinks.ID, model.PermissionSysconsoleReadSiteNotices.ID}
	permissionsSiteWrite := []string{model.PermissionSysconsoleWriteSiteCustomization.ID, model.PermissionSysconsoleWriteSiteLocalization.ID, model.PermissionSysconsoleWriteSiteUsersAndTeams.ID, model.PermissionSysconsoleWriteSiteNotifications.ID, model.PermissionSysconsoleWriteSiteAnnouncementBanner.ID, model.PermissionSysconsoleWriteSiteEmoji.ID, model.PermissionSysconsoleWriteSitePosts.ID, model.PermissionSysconsoleWriteSiteFileSharingAndDownloads.ID, model.PermissionSysconsoleWriteSitePublicLinks.ID, model.PermissionSysconsoleWriteSiteNotices.ID}

	// Give the new subsection READ permissions to any user with READ_SITE
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadSite.ID),
		Add: permissionsSiteRead,
	})

	// Give the new subsection WRITE permissions to any user with WRITE_SITE
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteSite.ID),
		Add: permissionsSiteWrite,
	})

	// Give the ancillary permissions EDIT_BRAND to anyone with WRITE_SITE_CUSTOMIZATION
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteSiteCustomization.ID),
		Add: []string{model.PermissionEditBrand.ID},
	})

	return transformations, nil
}

func (a *App) getAddComplianceSubsectionPermissions() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsComplianceRead := []string{model.PermissionSysconsoleReadComplianceDataRetentionPolicy.ID, model.PermissionSysconsoleReadComplianceComplianceExport.ID, model.PermissionSysconsoleReadComplianceComplianceMonitoring.ID, model.PermissionSysconsoleReadComplianceCustomTermsOfService.ID}
	permissionsComplianceWrite := []string{model.PermissionSysconsoleWriteComplianceDataRetentionPolicy.ID, model.PermissionSysconsoleWriteComplianceComplianceExport.ID, model.PermissionSysconsoleWriteComplianceComplianceMonitoring.ID, model.PermissionSysconsoleWriteComplianceCustomTermsOfService.ID}

	// Give the new subsection READ permissions to any user with READ_COMPLIANCE
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadCompliance.ID),
		Add: permissionsComplianceRead,
	})

	// Give the new subsection WRITE permissions to any user with WRITE_COMPLIANCE
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteCompliance.ID),
		Add: permissionsComplianceWrite,
	})

	// Ancilary permissions
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteComplianceDataRetentionPolicy.ID),
		Add: []string{model.PermissionCreateDataRetentionJob.ID},
	})

	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadComplianceDataRetentionPolicy.ID),
		Add: []string{model.PermissionReadDataRetentionJob.ID},
	})

	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteComplianceComplianceExport.ID),
		Add: []string{model.PermissionCreateComplianceExportJob.ID, model.PermissionDownloadComplianceExportResult.ID},
	})

	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadComplianceComplianceExport.ID),
		Add: []string{model.PermissionReadComplianceExportJob.ID, model.PermissionDownloadComplianceExportResult.ID},
	})

	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadComplianceCustomTermsOfService.ID),
		Add: []string{model.PermissionReadAudits.ID},
	})

	return transformations, nil
}

func (a *App) getAddEnvironmentSubsectionPermissions() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsEnvironmentRead := []string{
		model.PermissionSysconsoleReadEnvironmentWebServer.ID,
		model.PermissionSysconsoleReadEnvironmentDatabase.ID,
		model.PermissionSysconsoleReadEnvironmentElasticsearch.ID,
		model.PermissionSysconsoleReadEnvironmentFileStorage.ID,
		model.PermissionSysconsoleReadEnvironmentImageProxy.ID,
		model.PermissionSysconsoleReadEnvironmentSMTP.ID,
		model.PermissionSysconsoleReadEnvironmentPushNotificationServer.ID,
		model.PermissionSysconsoleReadEnvironmentHighAvailability.ID,
		model.PermissionSysconsoleReadEnvironmentRateLimiting.ID,
		model.PermissionSysconsoleReadEnvironmentLogging.ID,
		model.PermissionSysconsoleReadEnvironmentSessionLengths.ID,
		model.PermissionSysconsoleReadEnvironmentPerformanceMonitoring.ID,
		model.PermissionSysconsoleReadEnvironmentDeveloper.ID,
	}
	permissionsEnvironmentWrite := []string{
		model.PermissionSysconsoleWriteEnvironmentWebServer.ID,
		model.PermissionSysconsoleWriteEnvironmentDatabase.ID,
		model.PermissionSysconsoleWriteEnvironmentElasticsearch.ID,
		model.PermissionSysconsoleWriteEnvironmentFileStorage.ID,
		model.PermissionSysconsoleWriteEnvironmentImageProxy.ID,
		model.PermissionSysconsoleWriteEnvironmentSMTP.ID,
		model.PermissionSysconsoleWriteEnvironmentPushNotificationServer.ID,
		model.PermissionSysconsoleWriteEnvironmentHighAvailability.ID,
		model.PermissionSysconsoleWriteEnvironmentRateLimiting.ID,
		model.PermissionSysconsoleWriteEnvironmentLogging.ID,
		model.PermissionSysconsoleWriteEnvironmentSessionLengths.ID,
		model.PermissionSysconsoleWriteEnvironmentPerformanceMonitoring.ID,
		model.PermissionSysconsoleWriteEnvironmentDeveloper.ID,
	}

	// Give the new subsection READ permissions to any user with READ_ENVIRONMENT
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadEnvironment.ID),
		Add: permissionsEnvironmentRead,
	})

	// Give the new subsection WRITE permissions to any user with WRITE_ENVIRONMENT
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteEnvironment.ID),
		Add: permissionsEnvironmentWrite,
	})

	// Give these ancillary permissions to anyone with READ_ENVIRONMENT_ELASTICSEARCH
	transformations = append(transformations, permissionTransformation{
		On: permissionExists(model.PermissionSysconsoleReadEnvironmentElasticsearch.ID),
		Add: []string{
			model.PermissionReadElasticsearchPostIndexingJob.ID,
			model.PermissionReadElasticsearchPostAggregationJob.ID,
		},
	})

	// Give these ancillary permissions to anyone with WRITE_ENVIRONMENT_WEB_SERVER
	transformations = append(transformations, permissionTransformation{
		On: permissionExists(model.PermissionSysconsoleWriteEnvironmentWebServer.ID),
		Add: []string{
			model.PermissionTestSiteURL.ID,
			model.PermissionReloadConfig.ID,
			model.PermissionInvalidateCaches.ID,
		},
	})

	// Give these ancillary permissions to anyone with WRITE_ENVIRONMENT_DATABASE
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteEnvironmentDatabase.ID),
		Add: []string{model.PermissionRecycleDatabaseConnections.ID},
	})

	// Give these ancillary permissions to anyone with WRITE_ENVIRONMENT_ELASTICSEARCH
	transformations = append(transformations, permissionTransformation{
		On: permissionExists(model.PermissionSysconsoleWriteEnvironmentElasticsearch.ID),
		Add: []string{
			model.PermissionTestElasticsearch.ID,
			model.PermissionCreateElasticsearchPostIndexingJob.ID,
			model.PermissionCreateElasticsearchPostAggregationJob.ID,
			model.PermissionPurgeElasticsearchIndexes.ID,
		},
	})

	// Give these ancillary permissions to anyone with WRITE_ENVIRONMENT_FILE_STORAGE
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteEnvironmentFileStorage.ID),
		Add: []string{model.PermissionTestS3.ID},
	})

	return transformations, nil
}

func (a *App) getAddAboutSubsectionPermissions() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsAboutRead := []string{model.PermissionSysconsoleReadAboutEditionAndLicense.ID}
	permissionsAboutWrite := []string{model.PermissionSysconsoleWriteAboutEditionAndLicense.ID}

	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadAbout.ID),
		Add: permissionsAboutRead,
	})

	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteAbout.ID),
		Add: permissionsAboutWrite,
	})

	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadAboutEditionAndLicense.ID),
		Add: []string{model.PermissionReadLicenseInformation.ID},
	})

	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteAboutEditionAndLicense.ID),
		Add: []string{model.PermissionManageLicenseInformation.ID},
	})

	return transformations, nil
}

func (a *App) getAddReportingSubsectionPermissions() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsReportingRead := []string{
		model.PermissionSysconsoleReadReportingSiteStatistics.ID,
		model.PermissionSysconsoleReadReportingTeamStatistics.ID,
		model.PermissionSysconsoleReadReportingServerLogs.ID,
	}
	permissionsReportingWrite := []string{
		model.PermissionSysconsoleWriteReportingSiteStatistics.ID,
		model.PermissionSysconsoleWriteReportingTeamStatistics.ID,
		model.PermissionSysconsoleWriteReportingServerLogs.ID,
	}

	// Give the new subsection READ permissions to any user with READ_REPORTING
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadReporting.ID),
		Add: permissionsReportingRead,
	})

	// Give the new subsection WRITE permissions to any user with WRITE_REPORTING
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteReporting.ID),
		Add: permissionsReportingWrite,
	})

	// Give the ancillary permissions PERMISSION_GET_ANALYTICS to anyone with PERMISSION_SYSCONSOLE_READ_USERMANAGEMENT_USERS or PERMISSION_SYSCONSOLE_READ_REPORTING_SITE_STATISTICS
	transformations = append(transformations, permissionTransformation{
		On:  permissionOr(permissionExists(model.PermissionSysconsoleReadUserManagementUsers.ID), permissionExists(model.PermissionSysconsoleReadReportingSiteStatistics.ID)),
		Add: []string{model.PermissionGetAnalytics.ID},
	})

	// Give the ancillary permissions PERMISSION_GET_LOGS to anyone with PERMISSION_SYSCONSOLE_READ_REPORTING_SERVER_LOGS
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadReportingServerLogs.ID),
		Add: []string{model.PermissionGetLogs.ID},
	})

	return transformations, nil
}

func (a *App) getAddAuthenticationSubsectionPermissions() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	permissionsAuthenticationRead := []string{model.PermissionSysconsoleReadAuthenticationSignup.ID, model.PermissionSysconsoleReadAuthenticationEmail.ID, model.PermissionSysconsoleReadAuthenticationPassword.ID, model.PermissionSysconsoleReadAuthenticationMfa.ID, model.PermissionSysconsoleReadAuthenticationLdap.ID, model.PermissionSysconsoleReadAuthenticationSaml.ID, model.PermissionSysconsoleReadAuthenticationOpenid.ID, model.PermissionSysconsoleReadAuthenticationGuestAccess.ID}
	permissionsAuthenticationWrite := []string{model.PermissionSysconsoleWriteAuthenticationSignup.ID, model.PermissionSysconsoleWriteAuthenticationEmail.ID, model.PermissionSysconsoleWriteAuthenticationPassword.ID, model.PermissionSysconsoleWriteAuthenticationMfa.ID, model.PermissionSysconsoleWriteAuthenticationLdap.ID, model.PermissionSysconsoleWriteAuthenticationSaml.ID, model.PermissionSysconsoleWriteAuthenticationOpenid.ID, model.PermissionSysconsoleWriteAuthenticationGuestAccess.ID}

	// Give the new subsection READ permissions to any user with READ_AUTHENTICATION
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadAuthentication.ID),
		Add: permissionsAuthenticationRead,
	})

	// Give the new subsection WRITE permissions to any user with WRITE_AUTHENTICATION
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteAuthentication.ID),
		Add: permissionsAuthenticationWrite,
	})

	// Give the ancillary permissions for LDAP to anyone with WRITE_AUTHENTICATION_LDAP
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteAuthenticationLdap.ID),
		Add: []string{model.PermissionCreateLdapSyncJob.ID, model.PermissionTestLdap.ID, model.PermissionAddLdapPublicCert.ID, model.PermissionAddLdapPrivateCert.ID, model.PermissionRemoveLdapPublicCert.ID, model.PermissionRemoveLdapPrivateCert.ID},
	})

	// Give the ancillary permissions PERMISSION_TEST_LDAP to anyone with READ_AUTHENTICATION_LDAP
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleReadAuthenticationLdap.ID),
		Add: []string{model.PermissionReadLdapSyncJob.ID},
	})

	// Give the ancillary permissions PERMISSION_INVALIDATE_EMAIL_INVITE to anyone with WRITE_AUTHENTICATION_EMAIL
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteAuthenticationEmail.ID),
		Add: []string{model.PermissionInvalidateEmailInvite.ID},
	})

	// Give the ancillary permissions for SAML to anyone with WRITE_AUTHENTICATION_SAML
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteAuthenticationSaml.ID),
		Add: []string{model.PermissionGetSamlMetadataFromIDp.ID, model.PermissionAddSamlPublicCert.ID, model.PermissionAddSamlPrivateCert.ID, model.PermissionAddSamlIDpCert.ID, model.PermissionRemoveSamlPublicCert.ID, model.PermissionRemoveSamlPrivateCert.ID, model.PermissionRemoveSamlIDpCert.ID, model.PermissionGetSamlCertStatus.ID},
	})

	return transformations, nil
}

// This migration fixes https://github.com/mattermost/mattermost-server/issues/17642 where this particular ancillary permission was forgotten during the initial migrations
func (a *App) getAddTestEmailAncillaryPermission() (permissionsMap, error) {
	transformations := []permissionTransformation{}

	// Give these ancillary permissions to anyone with WRITE_ENVIRONMENT_SMTP
	transformations = append(transformations, permissionTransformation{
		On:  permissionExists(model.PermissionSysconsoleWriteEnvironmentSMTP.ID),
		Add: []string{model.PermissionTestEmail.ID},
	})

	return transformations, nil
}

// DoPermissionsMigrations execute all the permissions migrations need by the current version.
func (a *App) DoPermissionsMigrations() error {
	return a.Srv().doPermissionsMigrations()
}

func (s *Server) doPermissionsMigrations() error {
	a := New(ServerConnector(s))
	PermissionsMigrations := []struct {
		Key       string
		Migration func() (permissionsMap, error)
	}{
		{Key: model.MigrationKeyEmojiPermissionsSplit, Migration: a.getEmojisPermissionsSplitMigration},
		{Key: model.MigrationKeyWebhookPermissionsSplit, Migration: a.getWebhooksPermissionsSplitMigration},
		{Key: model.MigrationKeyListJoinPublicPrivateTeams, Migration: a.getListJoinPublicPrivateTeamsPermissionsMigration},
		{Key: model.MigrationKeyRemovePermanentDeleteUser, Migration: a.removePermanentDeleteUserMigration},
		{Key: model.MigrationKeyAddBotPermissions, Migration: a.getAddBotPermissionsMigration},
		{Key: model.MigrationKeyApplyChannelManageDeleteToChannelUser, Migration: a.applyChannelManageDeleteToChannelUser},
		{Key: model.MigrationKeyRemoveChannelManageDeleteFromTeamUser, Migration: a.removeChannelManageDeleteFromTeamUser},
		{Key: model.MigrationKeyViewMembersNewPermission, Migration: a.getViewMembersPermissionMigration},
		{Key: model.MigrationKeyAddManageGuestsPermissions, Migration: a.getAddManageGuestsPermissionsMigration},
		{Key: model.MigrationKeyChannelModerationsPermissions, Migration: a.channelModerationPermissionsMigration},
		{Key: model.MigrationKeyAddUseGroupMentionsPermission, Migration: a.getAddUseGroupMentionsPermissionMigration},
		{Key: model.MigrationKeyAddSystemConsolePermissions, Migration: a.getAddSystemConsolePermissionsMigration},
		{Key: model.MigrationKeyAddConvertChannelPermissions, Migration: a.getAddConvertChannelPermissionsMigration},
		{Key: model.MigrationKeyAddManageSharedChannelPermissions, Migration: a.getAddManageSharedChannelsPermissionsMigration},
		{Key: model.MigrationKeyAddManageSecureConnectionsPermissions, Migration: a.getAddManageSecureConnectionsPermissionsMigration},
		{Key: model.MigrationKeyAddSystemRolesPermissions, Migration: a.getSystemRolesPermissionsMigration},
		{Key: model.MigrationKeyAddBillingPermissions, Migration: a.getBillingPermissionsMigration},
		{Key: model.MigrationKeyAddDownloadComplianceExportResults, Migration: a.getAddDownloadComplianceExportResult},
		{Key: model.MigrationKeyAddExperimentalSubsectionPermissions, Migration: a.getAddExperimentalSubsectionPermissions},
		{Key: model.MigrationKeyAddAuthenticationSubsectionPermissions, Migration: a.getAddAuthenticationSubsectionPermissions},
		{Key: model.MigrationKeyAddIntegrationsSubsectionPermissions, Migration: a.getAddIntegrationsSubsectionPermissions},
		{Key: model.MigrationKeyAddSiteSubsectionPermissions, Migration: a.getAddSiteSubsectionPermissions},
		{Key: model.MigrationKeyAddComplianceSubsectionPermissions, Migration: a.getAddComplianceSubsectionPermissions},
		{Key: model.MigrationKeyAddEnvironmentSubsectionPermissions, Migration: a.getAddEnvironmentSubsectionPermissions},
		{Key: model.MigrationKeyAddAboutSubsectionPermissions, Migration: a.getAddAboutSubsectionPermissions},
		{Key: model.MigrationKeyAddReportingSubsectionPermissions, Migration: a.getAddReportingSubsectionPermissions},
		{Key: model.MigrationKeyAddTestEmailAncillaryPermission, Migration: a.getAddTestEmailAncillaryPermission},
	}

	roles, err := s.Store.Role().GetAll()
	if err != nil {
		return err
	}

	for _, migration := range PermissionsMigrations {
		migMap, err := migration.Migration()
		if err != nil {
			return err
		}
		if err := s.doPermissionsMigration(migration.Key, migMap, roles); err != nil {
			return err
		}
	}
	return nil
}
