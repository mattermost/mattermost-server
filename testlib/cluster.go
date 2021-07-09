// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package testlib

import (
	"sync"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
)

type FakeClusterInterface struct {
	clusterMessageHandler einterfaces.ClusterMessageHandler
	mut                   sync.RWMutex
	messages              []*model.ClusterMessage
}

func (c *FakeClusterInterface) StartInterNodeCommunication() {}

func (c *FakeClusterInterface) StopInterNodeCommunication() {}

func (c *FakeClusterInterface) RegisterClusterMessageHandler(event string, crm einterfaces.ClusterMessageHandler) {
	c.clusterMessageHandler = crm
}

func (c *FakeClusterInterface) HealthScore() int {
	return 0
}

func (c *FakeClusterInterface) GetClusterID() string { return "" }

func (c *FakeClusterInterface) IsLeader() bool { return false }

func (c *FakeClusterInterface) GetMyClusterInfo() *model.ClusterInfo { return nil }

func (c *FakeClusterInterface) GetClusterInfos() []*model.ClusterInfo { return nil }

func (c *FakeClusterInterface) SendClusterMessage(message *model.ClusterMessage) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.messages = append(c.messages, message)
}

func (c *FakeClusterInterface) SendClusterMessageToNode(nodeID string, message *model.ClusterMessage) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.messages = append(c.messages, message)
	return nil
}

func (c *FakeClusterInterface) NotifyMsg(buf []byte) {}

func (c *FakeClusterInterface) GetClusterStats() ([]*model.ClusterStats, *model.AppError) {
	return nil, nil
}

func (c *FakeClusterInterface) GetLogs(page, perPage int) ([]string, *model.AppError) {
	return []string{}, nil
}

func (c *FakeClusterInterface) ConfigChanged(previousConfig *model.Config, newConfig *model.Config, sendToOtherServer bool) *model.AppError {
	return nil
}

func (c *FakeClusterInterface) SendClearRoleCacheMessage() {
	if c.clusterMessageHandler != nil {
		c.clusterMessageHandler(&model.ClusterMessage{
			Event: model.ClusterEventInvalidateCacheForRoles,
		})
	}
}

func (c *FakeClusterInterface) GetPluginStatuses() (model.PluginStatuses, *model.AppError) {
	return nil, nil
}

func (c *FakeClusterInterface) GetMessages() []*model.ClusterMessage {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.messages
}

func (c *FakeClusterInterface) SelectMessages(filterCond func(message *model.ClusterMessage) bool) []*model.ClusterMessage {
	c.mut.RLock()
	defer c.mut.RUnlock()

	filteredMessages := []*model.ClusterMessage{}
	for _, msg := range c.messages {
		if filterCond(msg) {
			filteredMessages = append(filteredMessages, msg)
		}
	}
	return filteredMessages
}

func (c *FakeClusterInterface) ClearMessages() {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.messages = nil
}
