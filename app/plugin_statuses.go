// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
)

// GetPluginStatus returns the status for a plugin installed on this server.
func (s *Server) GetPluginStatus(id string) (*model.PluginStatus, *model.AppError) {
	pluginsEnvironment := s.GetPluginsEnvironment()
	if pluginsEnvironment == nil {
		return nil, model.NewAppError("GetPluginStatus", "app.plugin.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	pluginStatuses, err := pluginsEnvironment.Statuses()
	if err != nil {
		return nil, model.NewAppError("GetPluginStatus", "app.plugin.get_statuses.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	for _, status := range pluginStatuses {
		if status.PluginID == id {
			// Add our cluster ID
			if s.Cluster != nil {
				status.ClusterID = s.Cluster.GetClusterID()
			}

			return status, nil
		}
	}

	return nil, model.NewAppError("GetPluginStatus", "app.plugin.not_installed.app_error", nil, "", http.StatusNotFound)
}

// GetPluginStatus returns the status for a plugin installed on this server.
func (a *App) GetPluginStatus(id string) (*model.PluginStatus, *model.AppError) {
	return a.Srv().GetPluginStatus(id)
}

// GetPluginStatuses returns the status for plugins installed on this server.
func (s *Server) GetPluginStatuses() (model.PluginStatuses, *model.AppError) {
	pluginsEnvironment := s.GetPluginsEnvironment()
	if pluginsEnvironment == nil {
		return nil, model.NewAppError("GetPluginStatuses", "app.plugin.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	pluginStatuses, err := pluginsEnvironment.Statuses()
	if err != nil {
		return nil, model.NewAppError("GetPluginStatuses", "app.plugin.get_statuses.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	// Add our cluster ID
	for _, status := range pluginStatuses {
		if s.Cluster != nil {
			status.ClusterID = s.Cluster.GetClusterID()
		} else {
			status.ClusterID = ""
		}
	}

	return pluginStatuses, nil
}

// GetPluginStatuses returns the status for plugins installed on this server.
func (a *App) GetPluginStatuses() (model.PluginStatuses, *model.AppError) {
	return a.Srv().GetPluginStatuses()
}

// GetClusterPluginStatuses returns the status for plugins installed anywhere in the cluster.
func (a *App) GetClusterPluginStatuses() (model.PluginStatuses, *model.AppError) {
	return a.Srv().getClusterPluginStatuses()
}

func (s *Server) getClusterPluginStatuses() (model.PluginStatuses, *model.AppError) {
	pluginStatuses, err := s.GetPluginStatuses()
	if err != nil {
		return nil, err
	}

	if s.Cluster != nil && *s.Config().ClusterSettings.Enable {
		clusterPluginStatuses, err := s.Cluster.GetPluginStatuses()
		if err != nil {
			return nil, model.NewAppError("GetClusterPluginStatuses", "app.plugin.get_cluster_plugin_statuses.app_error", nil, err.Error(), http.StatusInternalServerError)
		}

		pluginStatuses = append(pluginStatuses, clusterPluginStatuses...)
	}

	return pluginStatuses, nil
}

func (s *Server) notifyPluginStatusesChanged() error {
	pluginStatuses, err := s.getClusterPluginStatuses()
	if err != nil {
		return err
	}

	// Notify any system admins.
	message := model.NewWebSocketEvent(model.WebsocketEventPluginStatusesChanged, "", "", "", nil)
	message.Add("plugin_statuses", pluginStatuses)
	message.GetBroadcast().ContainsSensitiveData = true
	s.Publish(message)

	return nil
}
