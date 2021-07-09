// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/testlib"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/services/remotecluster"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestShareProviderDoCommand(t *testing.T) {
	t.Run("share command sends a websocket channel converted event", func(t *testing.T) {
		th := setup(t).initBasic()
		defer th.tearDown()

		th.addPermissionToRole(model.PermissionManageSharedChannels.ID, th.BasicUser.Roles)

		mockSyncService := app.NewMockSharedChannelService(nil)
		th.Server.SetSharedChannelSyncService(mockSyncService)
		mockRemoteCluster, err := remotecluster.NewRemoteClusterService(th.Server)
		require.NoError(t, err)

		th.Server.SetRemoteClusterService(mockRemoteCluster)
		testCluster := &testlib.FakeClusterInterface{}
		th.Server.Cluster = testCluster

		commandProvider := ShareProvider{}
		channel := th.CreateChannel(th.BasicTeam, WithShared(false))

		args := &model.CommandArgs{
			T:         func(s string, args ...interface{}) string { return s },
			ChannelID: channel.ID,
			UserID:    th.BasicUser.ID,
			TeamID:    th.BasicTeam.ID,
			Command:   "/share-channel share",
		}

		response := commandProvider.DoCommand(th.App, th.Context, args, "")
		require.Equal(t, "##### "+args.T("api.command_share.channel_shared"), response.Text)

		channelConvertedMessages := testCluster.SelectMessages(func(msg *model.ClusterMessage) bool {
			event := model.WebSocketEventFromJSON(strings.NewReader(msg.Data))
			return event != nil && event.EventType() == model.WebsocketEventChannelConverted
		})
		assert.Len(t, channelConvertedMessages, 1)
	})

	t.Run("unshare command sends a websocket channel converted event", func(t *testing.T) {
		th := setup(t).initBasic()
		defer th.tearDown()

		th.addPermissionToRole(model.PermissionManageSharedChannels.ID, th.BasicUser.Roles)

		mockSyncService := app.NewMockSharedChannelService(nil)
		th.Server.SetSharedChannelSyncService(mockSyncService)
		mockRemoteCluster, err := remotecluster.NewRemoteClusterService(th.Server)
		require.NoError(t, err)

		th.Server.SetRemoteClusterService(mockRemoteCluster)
		testCluster := &testlib.FakeClusterInterface{}
		th.Server.Cluster = testCluster

		commandProvider := ShareProvider{}
		channel := th.CreateChannel(th.BasicTeam, WithShared(true))
		args := &model.CommandArgs{
			T:         func(s string, args ...interface{}) string { return s },
			ChannelID: channel.ID,
			UserID:    th.BasicUser.ID,
			TeamID:    th.BasicTeam.ID,
			Command:   "/share-channel unshare",
		}

		response := commandProvider.DoCommand(th.App, th.Context, args, "")
		require.Equal(t, "##### "+args.T("api.command_share.shared_channel_unavailable"), response.Text)

		channelConvertedMessages := testCluster.SelectMessages(func(msg *model.ClusterMessage) bool {
			event := model.WebSocketEventFromJSON(strings.NewReader(msg.Data))
			return event != nil && event.EventType() == model.WebsocketEventChannelConverted
		})
		require.Len(t, channelConvertedMessages, 1)
	})
}
