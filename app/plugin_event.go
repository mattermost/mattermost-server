// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost-server/v5/model"
)

func (s *Server) notifyClusterPluginEvent(event string, data model.PluginEventData) {
	if s.Cluster != nil {
		s.Cluster.SendClusterMessage(&model.ClusterMessage{
			Event:            event,
			SendType:         model.ClusterSendReliable,
			WaitForAllToSend: true,
			Data:             data.ToJSON(),
		})
	}
}
