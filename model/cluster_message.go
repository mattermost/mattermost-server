// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"io"
)

const (
	ClusterEventPublish                                     = "publish"
	ClusterEventUpdateStatus                                = "update_status"
	ClusterEventInvalidateAllCaches                         = "inv_all_caches"
	ClusterEventInvalidateCacheForReactions                 = "inv_reactions"
	ClusterEventInvalidateCacheForChannelMembersNotifyProps = "inv_channel_members_notify_props"
	ClusterEventInvalidateCacheForChannelByName             = "inv_channel_name"
	ClusterEventInvalidateCacheForChannel                   = "inv_channel"
	ClusterEventInvalidateCacheForChannelGuestCount         = "inv_channel_guest_count"
	ClusterEventInvalidateCacheForUser                      = "inv_user"
	ClusterEventInvalidateCacheForUserTeams                 = "inv_user_teams"
	ClusterEventClearSessionCacheForUser                    = "clear_session_user"
	ClusterEventInvalidateCacheForRoles                     = "inv_roles"
	ClusterEventInvalidateCacheForRolePermissions           = "inv_role_permissions"
	ClusterEventInvalidateCacheForProfileByIds              = "inv_profile_ids"
	ClusterEventInvalidateCacheForProfileInChannel          = "inv_profile_in_channel"
	ClusterEventInvalidateCacheForSchemes                   = "inv_schemes"
	ClusterEventInvalidateCacheForFileInfos                 = "inv_file_infos"
	ClusterEventInvalidateCacheForWebhooks                  = "inv_webhooks"
	ClusterEventInvalidateCacheForEmojisById                = "inv_emojis_by_id"
	ClusterEventInvalidateCacheForEmojisIdByName            = "inv_emojis_id_by_name"
	ClusterEventInvalidateCacheForChannelPinnedpostsCounts  = "inv_channel_pinnedposts_counts"
	ClusterEventInvalidateCacheForChannelMemberCounts       = "inv_channel_member_counts"
	ClusterEventInvalidateCacheForLastPosts                 = "inv_last_posts"
	ClusterEventInvalidateCacheForLastPostTime              = "inv_last_post_time"
	ClusterEventInvalidateCacheForTeams                     = "inv_teams"
	ClusterEventClearSessionCacheForAllUsers                = "inv_all_user_sessions"
	ClusterEventInstallPlugin                               = "install_plugin"
	ClusterEventRemovePlugin                                = "remove_plugin"
	ClusterEventPluginEvent                                 = "plugin_event"
	ClusterEventInvalidateCacheForTermsOfService            = "inv_terms_of_service"
	ClusterEventBusyStateChanged                            = "busy_state_change"

	// Gossip communication
	ClusterGossipEventRequestGetLogs            = "gossip_request_get_logs"
	ClusterGossipEventResponseGetLogs           = "gossip_response_get_logs"
	ClusterGossipEventRequestGetClusterStats    = "gossip_request_cluster_stats"
	ClusterGossipEventResponseGetClusterStats   = "gossip_response_cluster_stats"
	ClusterGossipEventRequestGetPluginStatuses  = "gossip_request_plugin_statuses"
	ClusterGossipEventResponseGetPluginStatuses = "gossip_response_plugin_statuses"
	ClusterGossipEventRequestSaveConfig         = "gossip_request_save_config"
	ClusterGossipEventResponseSaveConfig        = "gossip_response_save_config"

	// SendTypes for ClusterMessage.
	ClusterSendBestEffort = "best_effort"
	ClusterSendReliable   = "reliable"
)

type ClusterMessage struct {
	Event            string            `json:"event"`
	SendType         string            `json:"-"`
	WaitForAllToSend bool              `json:"-"`
	Data             string            `json:"data,omitempty"`
	Props            map[string]string `json:"props,omitempty"`
}

func (o *ClusterMessage) ToJson() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func ClusterMessageFromJson(data io.Reader) *ClusterMessage {
	var o *ClusterMessage
	json.NewDecoder(data).Decode(&o)
	return o
}
