// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func (s *Server) clusterInstallPluginHandler(msg *model.ClusterMessage) {
	s.installPluginFromData(model.PluginEventDataFromJSON(strings.NewReader(msg.Data)))
}

func (s *Server) clusterRemovePluginHandler(msg *model.ClusterMessage) {
	s.removePluginFromData(model.PluginEventDataFromJSON(strings.NewReader(msg.Data)))
}

func (s *Server) clusterPluginEventHandler(msg *model.ClusterMessage) {
	env := s.GetPluginsEnvironment()
	if env == nil {
		return
	}
	if msg.Props == nil {
		mlog.Warn("ClusterMessage.Props for plugin event should not be nil")
		return
	}
	pluginID := msg.Props["PluginID"]
	eventID := msg.Props["EventID"]
	if pluginID == "" || eventID == "" {
		mlog.Warn("Invalid ClusterMessage.Props values for plugin event",
			mlog.String("plugin_id", pluginID), mlog.String("event_id", eventID))
		return
	}

	hooks, err := env.HooksForPlugin(pluginID)
	if err != nil {
		mlog.Warn("Getting hooks for plugin failed", mlog.String("plugin_id", pluginID), mlog.Err(err))
		return
	}

	hooks.OnPluginClusterEvent(&plugin.Context{}, model.PluginClusterEvent{
		ID:   eventID,
		Data: []byte(msg.Data),
	})
}

// registerClusterHandlers registers the cluster message handlers that are handled by the server.
//
// The cluster event handlers are spread across this function and NewLocalCacheLayer.
// Be careful to not have duplicated handlers here and there.
func (s *Server) registerClusterHandlers() {
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventPublish, s.clusterPublishHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventUpdateStatus, s.clusterUpdateStatusHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventInvalidateAllCaches, s.clusterInvalidateAllCachesHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventInvalidateCacheForChannelMembersNotifyProps, s.clusterInvalidateCacheForChannelMembersNotifyPropHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventInvalidateCacheForChannelByName, s.clusterInvalidateCacheForChannelByNameHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventInvalidateCacheForUser, s.clusterInvalidateCacheForUserHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventInvalidateCacheForUserTeams, s.clusterInvalidateCacheForUserTeamsHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventBusyStateChanged, s.clusterBusyStateChgHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventClearSessionCacheForUser, s.clusterClearSessionCacheForUserHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventClearSessionCacheForAllUsers, s.clusterClearSessionCacheForAllUsersHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventInstallPlugin, s.clusterInstallPluginHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventRemovePlugin, s.clusterRemovePluginHandler)
	s.Cluster.RegisterClusterMessageHandler(model.ClusterEventPluginEvent, s.clusterPluginEventHandler)
}

func (s *Server) clusterPublishHandler(msg *model.ClusterMessage) {
	event := model.WebSocketEventFromJSON(strings.NewReader(msg.Data))
	if event == nil {
		return
	}
	s.PublishSkipClusterSend(event)
}

func (s *Server) clusterUpdateStatusHandler(msg *model.ClusterMessage) {
	status := model.StatusFromJSON(strings.NewReader(msg.Data))
	s.statusCache.Set(status.UserID, status)
}

func (s *Server) clusterInvalidateAllCachesHandler(msg *model.ClusterMessage) {
	s.InvalidateAllCachesSkipSend()
}

func (s *Server) clusterInvalidateCacheForChannelMembersNotifyPropHandler(msg *model.ClusterMessage) {
	s.invalidateCacheForChannelMembersNotifyPropsSkipClusterSend(msg.Data)
}

func (s *Server) clusterInvalidateCacheForChannelByNameHandler(msg *model.ClusterMessage) {
	s.invalidateCacheForChannelByNameSkipClusterSend(msg.Props["id"], msg.Props["name"])
}

func (s *Server) clusterInvalidateCacheForUserHandler(msg *model.ClusterMessage) {
	s.invalidateCacheForUserSkipClusterSend(msg.Data)
}

func (s *Server) clusterInvalidateCacheForUserTeamsHandler(msg *model.ClusterMessage) {
	s.invalidateWebConnSessionCacheForUser(msg.Data)
}

func (s *Server) clearSessionCacheForUserSkipClusterSend(userID string) {
	s.userService.ClearUserSessionCacheLocal(userID)
	s.invalidateWebConnSessionCacheForUser(userID)
}

func (s *Server) clearSessionCacheForAllUsersSkipClusterSend() {
	mlog.Info("Purging sessions cache")
	s.userService.ClearAllUsersSessionCacheLocal()
}

func (s *Server) clusterClearSessionCacheForUserHandler(msg *model.ClusterMessage) {
	s.clearSessionCacheForUserSkipClusterSend(msg.Data)
}

func (s *Server) clusterClearSessionCacheForAllUsersHandler(msg *model.ClusterMessage) {
	s.clearSessionCacheForAllUsersSkipClusterSend()
}

func (s *Server) clusterBusyStateChgHandler(msg *model.ClusterMessage) {
	s.serverBusyStateChanged(model.ServerBusyStateFromJSON(strings.NewReader(msg.Data)))
}

func (s *Server) invalidateCacheForChannelMembersNotifyPropsSkipClusterSend(channelID string) {
	s.Store.Channel().InvalidateCacheForChannelMembersNotifyProps(channelID)
}

func (s *Server) invalidateCacheForChannelByNameSkipClusterSend(teamID, name string) {
	if teamID == "" {
		teamID = "dm"
	}

	s.Store.Channel().InvalidateChannelByName(teamID, name)
}

func (s *Server) invalidateCacheForUserSkipClusterSend(userID string) {
	s.Store.Channel().InvalidateAllChannelMembersForUser(userID)
	s.invalidateWebConnSessionCacheForUser(userID)
}

func (s *Server) invalidateWebConnSessionCacheForUser(userID string) {
	hub := s.GetHubForUserID(userID)
	if hub != nil {
		hub.InvalidateUser(userID)
	}
}
