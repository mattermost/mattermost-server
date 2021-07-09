// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
)

var (
	rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func TestGetAllSharedChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	const pages = 3
	const pageSize = 7

	mockService := app.NewMockRemoteClusterService(nil, app.MockOptionRemoteClusterServiceWithActive(true))
	th.App.Srv().SetRemoteClusterService(mockService)

	savedIDs := make([]string, 0, pages*pageSize)

	// make some shared channels
	for i := 0; i < pages*pageSize; i++ {
		channel := th.CreateChannelWithClientAndTeam(th.Client, model.ChannelTypeOpen, th.BasicTeam.ID)
		sc := &model.SharedChannel{
			ChannelID: channel.ID,
			TeamID:    channel.TeamID,
			Home:      randomBool(),
			ShareName: fmt.Sprintf("test_share_%d", i),
			CreatorID: th.BasicChannel.CreatorID,
			RemoteID:  model.NewID(),
		}
		_, err := th.App.SaveSharedChannel(sc)
		require.NoError(t, err)
		savedIDs = append(savedIDs, channel.ID)
	}
	sort.Strings(savedIDs)

	t.Run("get shared channels paginated", func(t *testing.T) {
		channelIDs := make([]string, 0, 21)
		for i := 0; i < pages; i++ {
			channels, resp := th.Client.GetAllSharedChannels(th.BasicTeam.ID, i, pageSize)
			CheckNoError(t, resp)
			channelIDs = append(channelIDs, getIDs(channels)...)
		}
		sort.Strings(channelIDs)

		// ids lists should now match
		assert.Equal(t, savedIDs, channelIDs, "id lists should match")
	})

	t.Run("get shared channels for invalid team", func(t *testing.T) {
		channels, resp := th.Client.GetAllSharedChannels(model.NewID(), 0, 100)
		CheckNoError(t, resp)
		assert.Empty(t, channels)
	})
}

func getIDs(channels []*model.SharedChannel) []string {
	ids := make([]string, 0, len(channels))
	for _, c := range channels {
		ids = append(ids, c.ChannelID)
	}
	return ids
}

func randomBool() bool {
	return rnd.Intn(2) != 0
}

func TestGetRemoteClusterByID(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	mockService := app.NewMockRemoteClusterService(nil, app.MockOptionRemoteClusterServiceWithActive(true))
	th.App.Srv().SetRemoteClusterService(mockService)

	// for this test we need a user that belongs to a channel that
	// is shared with the requested remote id.

	// create a remote cluster
	rc := &model.RemoteCluster{
		RemoteID:     model.NewID(),
		Name:         "Test1",
		RemoteTeamID: model.NewID(),
		SiteURL:      model.NewID(),
		CreatorID:    model.NewID(),
	}
	rc, appErr := th.App.AddRemoteCluster(rc)
	require.Nil(t, appErr)

	// create a shared channel
	sc := &model.SharedChannel{
		ChannelID: th.BasicChannel.ID,
		TeamID:    th.BasicChannel.TeamID,
		Home:      false,
		ShareName: "test_share",
		CreatorID: th.BasicChannel.CreatorID,
		RemoteID:  rc.RemoteID,
	}
	sc, err := th.App.SaveSharedChannel(sc)
	require.NoError(t, err)

	// create a shared channel remote to connect them
	scr := &model.SharedChannelRemote{
		ID:                model.NewID(),
		ChannelID:         sc.ChannelID,
		CreatorID:         sc.CreatorID,
		IsInviteAccepted:  true,
		IsInviteConfirmed: true,
		RemoteID:          sc.RemoteID,
	}
	_, err = th.App.SaveSharedChannelRemote(scr)
	require.NoError(t, err)

	t.Run("valid remote, user is member", func(t *testing.T) {
		rcInfo, resp := th.Client.GetRemoteClusterInfo(rc.RemoteID)
		CheckNoError(t, resp)
		assert.Equal(t, rc.Name, rcInfo.Name)
	})

	t.Run("invalid remote", func(t *testing.T) {
		_, resp := th.Client.GetRemoteClusterInfo(model.NewID())
		CheckNotFoundStatus(t, resp)
	})

}

func TestCreateDirectChannelWithRemoteUser(t *testing.T) {
	t.Run("creates a local DM channel that is shared", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		Client := th.Client
		defer Client.Logout()

		localUser := th.BasicUser
		remoteUser := th.CreateUser()
		remoteUser.RemoteID = model.NewString(model.NewID())
		remoteUser, err := th.App.UpdateUser(remoteUser, false)
		require.Nil(t, err)

		dm, resp := Client.CreateDirectChannel(localUser.ID, remoteUser.ID)
		CheckNoError(t, resp)

		channelName := model.GetDMNameFromIDs(localUser.ID, remoteUser.ID)
		require.Equal(t, channelName, dm.Name, "dm name didn't match")
		assert.True(t, dm.IsShared())
	})

	t.Run("sends a shared channel invitation to the remote", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		Client := th.Client
		defer Client.Logout()

		mockService := app.NewMockSharedChannelService(nil, app.MockOptionSharedChannelServiceWithActive(true))
		th.App.Srv().SetSharedChannelSyncService(mockService)

		localUser := th.BasicUser
		remoteUser := th.CreateUser()
		rc := &model.RemoteCluster{
			Name:      "test",
			Token:     model.NewID(),
			CreatorID: localUser.ID,
		}
		rc, err := th.App.AddRemoteCluster(rc)
		require.Nil(t, err)

		remoteUser.RemoteID = model.NewString(rc.RemoteID)
		remoteUser, err = th.App.UpdateUser(remoteUser, false)
		require.Nil(t, err)

		dm, resp := Client.CreateDirectChannel(localUser.ID, remoteUser.ID)
		CheckNoError(t, resp)

		channelName := model.GetDMNameFromIDs(localUser.ID, remoteUser.ID)
		require.Equal(t, channelName, dm.Name, "dm name didn't match")
		require.True(t, dm.IsShared())

		assert.Equal(t, 1, mockService.NumInvitations())
	})

	t.Run("does not send a shared channel invitation to the remote when creator is remote", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		Client := th.Client
		defer Client.Logout()

		mockService := app.NewMockSharedChannelService(nil, app.MockOptionSharedChannelServiceWithActive(true))
		th.App.Srv().SetSharedChannelSyncService(mockService)

		localUser := th.BasicUser
		remoteUser := th.CreateUser()
		rc := &model.RemoteCluster{
			Name:      "test",
			Token:     model.NewID(),
			CreatorID: localUser.ID,
		}
		rc, err := th.App.AddRemoteCluster(rc)
		require.Nil(t, err)

		remoteUser.RemoteID = model.NewString(rc.RemoteID)
		remoteUser, err = th.App.UpdateUser(remoteUser, false)
		require.Nil(t, err)

		dm, resp := Client.CreateDirectChannel(remoteUser.ID, localUser.ID)
		CheckNoError(t, resp)

		channelName := model.GetDMNameFromIDs(localUser.ID, remoteUser.ID)
		require.Equal(t, channelName, dm.Name, "dm name didn't match")
		require.True(t, dm.IsShared())

		assert.Zero(t, mockService.NumInvitations())
	})
}
