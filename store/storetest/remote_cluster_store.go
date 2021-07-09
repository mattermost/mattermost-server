// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteClusterStore(t *testing.T, ss store.Store) {
	t.Run("RemoteClusterGetAllInChannel", func(t *testing.T) { testRemoteClusterGetAllInChannel(t, ss) })
	t.Run("RemoteClusterGetAllNotInChannel", func(t *testing.T) { testRemoteClusterGetAllNotInChannel(t, ss) })
	t.Run("RemoteClusterSave", func(t *testing.T) { testRemoteClusterSave(t, ss) })
	t.Run("RemoteClusterDelete", func(t *testing.T) { testRemoteClusterDelete(t, ss) })
	t.Run("RemoteClusterGet", func(t *testing.T) { testRemoteClusterGet(t, ss) })
	t.Run("RemoteClusterGetAll", func(t *testing.T) { testRemoteClusterGetAll(t, ss) })
	t.Run("RemoteClusterGetByTopic", func(t *testing.T) { testRemoteClusterGetByTopic(t, ss) })
	t.Run("RemoteClusterUpdateTopics", func(t *testing.T) { testRemoteClusterUpdateTopics(t, ss) })
}

func testRemoteClusterSave(t *testing.T, ss store.Store) {

	t.Run("Save", func(t *testing.T) {
		rc := &model.RemoteCluster{
			Name:      "some_remote",
			SiteURL:   "somewhere.com",
			CreatorID: model.NewID(),
		}

		rcSaved, err := ss.RemoteCluster().Save(rc)
		require.NoError(t, err)
		require.Equal(t, rc.Name, rcSaved.Name)
		require.Equal(t, rc.SiteURL, rcSaved.SiteURL)
		require.Greater(t, rc.CreateAt, int64(0))
		require.Equal(t, rc.LastPingAt, int64(0))
	})

	t.Run("Save missing display name", func(t *testing.T) {
		rc := &model.RemoteCluster{
			SiteURL:   "somewhere.com",
			CreatorID: model.NewID(),
		}
		_, err := ss.RemoteCluster().Save(rc)
		require.Error(t, err)
	})

	t.Run("Save missing creator id", func(t *testing.T) {
		rc := &model.RemoteCluster{
			Name:    "some_remote_2",
			SiteURL: "somewhere.com",
		}
		_, err := ss.RemoteCluster().Save(rc)
		require.Error(t, err)
	})
}

func testRemoteClusterDelete(t *testing.T, ss store.Store) {
	t.Run("Delete", func(t *testing.T) {
		rc := &model.RemoteCluster{
			Name:      "shortlived_remote",
			SiteURL:   "nowhere.com",
			CreatorID: model.NewID(),
		}
		rcSaved, err := ss.RemoteCluster().Save(rc)
		require.NoError(t, err)

		deleted, err := ss.RemoteCluster().Delete(rcSaved.RemoteID)
		require.NoError(t, err)
		require.True(t, deleted)
	})

	t.Run("Delete nonexistent", func(t *testing.T) {
		deleted, err := ss.RemoteCluster().Delete(model.NewID())
		require.NoError(t, err)
		require.False(t, deleted)
	})
}

func testRemoteClusterGet(t *testing.T, ss store.Store) {
	t.Run("Get", func(t *testing.T) {
		rc := &model.RemoteCluster{
			Name:      "shortlived_remote_2",
			SiteURL:   "nowhere.com",
			CreatorID: model.NewID(),
		}
		rcSaved, err := ss.RemoteCluster().Save(rc)
		require.NoError(t, err)

		rcGet, err := ss.RemoteCluster().Get(rcSaved.RemoteID)
		require.NoError(t, err)
		require.Equal(t, rcSaved.RemoteID, rcGet.RemoteID)
	})

	t.Run("Get not found", func(t *testing.T) {
		_, err := ss.RemoteCluster().Get(model.NewID())
		require.Error(t, err)
	})
}

func testRemoteClusterGetAll(t *testing.T, ss store.Store) {
	require.NoError(t, clearRemoteClusters(ss))

	userID := model.NewID()
	now := model.GetMillis()
	pingLongAgo := model.GetMillis() - (model.RemoteOfflineAfterMillis * 3)

	data := []*model.RemoteCluster{
		{Name: "offline_remote", CreatorID: userID, SiteURL: "somewhere.com", LastPingAt: pingLongAgo, Topics: " shared incident "},
		{Name: "some_online_remote", CreatorID: userID, SiteURL: "nowhere.com", LastPingAt: now, Topics: " shared incident "},
		{Name: "another_online_remote", CreatorID: model.NewID(), SiteURL: "underwhere.com", LastPingAt: now, Topics: ""},
		{Name: "another_offline_remote", CreatorID: model.NewID(), SiteURL: "knowhere.com", LastPingAt: pingLongAgo, Topics: " shared "},
		{Name: "brand_new_offline_remote", CreatorID: userID, SiteURL: "", LastPingAt: 0, Topics: " bogus shared stuff "},
	}

	idsAll := make([]string, 0)
	idsOnline := make([]string, 0)
	idsOffline := make([]string, 0)
	idsShareTopic := make([]string, 0)

	for _, item := range data {
		online := item.LastPingAt == now
		saved, err := ss.RemoteCluster().Save(item)
		require.NoError(t, err)
		idsAll = append(idsAll, saved.RemoteID)
		if online {
			idsOnline = append(idsOnline, saved.RemoteID)
		} else {
			idsOffline = append(idsOffline, saved.RemoteID)
		}
		if strings.Contains(saved.Topics, " shared ") {
			idsShareTopic = append(idsShareTopic, saved.RemoteID)
		}
	}

	t.Run("GetAll", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{}
		remotes, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		// make sure all the test data remotes were returned.
		ids := getIDs(remotes)
		assert.ElementsMatch(t, ids, idsAll)
	})

	t.Run("GetAll online only", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			ExcludeOffline: true,
		}
		remotes, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		// make sure all the online remotes were returned.
		ids := getIDs(remotes)
		assert.ElementsMatch(t, ids, idsOnline)
	})

	t.Run("GetAll by topic", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			Topic: "shared",
		}
		remotes, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		// make sure only correct topic returned
		ids := getIDs(remotes)
		assert.ElementsMatch(t, ids, idsShareTopic)
	})

	t.Run("GetAll online by topic", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			ExcludeOffline: true,
			Topic:          "shared",
		}
		remotes, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		// make sure only online remotes were returned.
		ids := getIDs(remotes)
		assert.Subset(t, idsOnline, ids)
		// make sure correct topic returned
		assert.Subset(t, idsShareTopic, ids)
		assert.Len(t, ids, 1)
	})

	t.Run("GetAll by Creator", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			CreatorID: userID,
		}
		remotes, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		// make sure only correct creator returned
		assert.Len(t, remotes, 3)
		for _, rc := range remotes {
			assert.Equal(t, userID, rc.CreatorID)
		}
	})

	t.Run("GetAll by Confirmed", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			OnlyConfirmed: true,
		}
		remotes, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		// make sure only confirmed returned
		assert.Len(t, remotes, 4)
		for _, rc := range remotes {
			assert.NotEmpty(t, rc.SiteURL)
		}
	})
}

func testRemoteClusterGetAllInChannel(t *testing.T, ss store.Store) {
	require.NoError(t, clearRemoteClusters(ss))
	now := model.GetMillis()

	userID := model.NewID()

	channel1, err := createTestChannel(ss, "channel_1")
	require.NoError(t, err)

	channel2, err := createTestChannel(ss, "channel_2")
	require.NoError(t, err)

	channel3, err := createTestChannel(ss, "channel_3")
	require.NoError(t, err)

	// Create shared channels
	scData := []*model.SharedChannel{
		{ChannelID: channel1.ID, TeamID: model.NewID(), Home: true, ShareName: "test_chan_1", CreatorID: model.NewID()},
		{ChannelID: channel2.ID, TeamID: model.NewID(), Home: true, ShareName: "test_chan_2", CreatorID: model.NewID()},
		{ChannelID: channel3.ID, TeamID: model.NewID(), Home: true, ShareName: "test_chan_3", CreatorID: model.NewID()},
	}
	for _, item := range scData {
		_, err := ss.SharedChannel().Save(item)
		require.NoError(t, err)
	}

	// Create some remote clusters
	rcData := []*model.RemoteCluster{
		{Name: "AAAA_Inc", CreatorID: userID, SiteURL: "aaaa.com", RemoteID: model.NewID(), LastPingAt: now},
		{Name: "BBBB_Inc", CreatorID: userID, SiteURL: "bbbb.com", RemoteID: model.NewID(), LastPingAt: 0},
		{Name: "CCCC_Inc", CreatorID: userID, SiteURL: "cccc.com", RemoteID: model.NewID(), LastPingAt: now},
		{Name: "DDDD_Inc", CreatorID: userID, SiteURL: "dddd.com", RemoteID: model.NewID(), LastPingAt: now},
		{Name: "EEEE_Inc", CreatorID: userID, SiteURL: "eeee.com", RemoteID: model.NewID(), LastPingAt: 0},
	}
	for _, item := range rcData {
		_, err := ss.RemoteCluster().Save(item)
		require.NoError(t, err)
	}

	// Create some shared channel remotes
	scrData := []*model.SharedChannelRemote{
		{ChannelID: channel1.ID, RemoteID: rcData[0].RemoteID, CreatorID: model.NewID()},
		{ChannelID: channel1.ID, RemoteID: rcData[1].RemoteID, CreatorID: model.NewID()},
		{ChannelID: channel2.ID, RemoteID: rcData[2].RemoteID, CreatorID: model.NewID()},
		{ChannelID: channel2.ID, RemoteID: rcData[3].RemoteID, CreatorID: model.NewID()},
		{ChannelID: channel2.ID, RemoteID: rcData[4].RemoteID, CreatorID: model.NewID()},
	}
	for _, item := range scrData {
		_, err := ss.SharedChannel().SaveRemote(item)
		require.NoError(t, err)
	}

	t.Run("Channel 1", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			InChannel: channel1.ID,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Len(t, list, 2, "channel 1 should have 2 remote clusters")
		ids := getIDs(list)
		require.ElementsMatch(t, []string{rcData[0].RemoteID, rcData[1].RemoteID}, ids)
	})

	t.Run("Channel 1 online only", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			ExcludeOffline: true,
			InChannel:      channel1.ID,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Len(t, list, 1, "channel 1 should have 1 online remote clusters")
		ids := getIDs(list)
		require.ElementsMatch(t, []string{rcData[0].RemoteID}, ids)
	})

	t.Run("Channel 2", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			InChannel: channel2.ID,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Len(t, list, 3, "channel 2 should have 3 remote clusters")
		ids := getIDs(list)
		require.ElementsMatch(t, []string{rcData[2].RemoteID, rcData[3].RemoteID, rcData[4].RemoteID}, ids)
	})

	t.Run("Channel 2 online only", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			ExcludeOffline: true,
			InChannel:      channel2.ID,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Len(t, list, 2, "channel 2 should have 2 online remote clusters")
		ids := getIDs(list)
		require.ElementsMatch(t, []string{rcData[2].RemoteID, rcData[3].RemoteID}, ids)
	})

	t.Run("Channel 3", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			InChannel: channel3.ID,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Empty(t, list, "channel 3 should have 0 remote clusters")
	})
}

func testRemoteClusterGetAllNotInChannel(t *testing.T, ss store.Store) {
	require.NoError(t, clearRemoteClusters(ss))

	userID := model.NewID()

	channel1, err := createTestChannel(ss, "channel_1")
	require.NoError(t, err)

	channel2, err := createTestChannel(ss, "channel_2")
	require.NoError(t, err)

	channel3, err := createTestChannel(ss, "channel_3")
	require.NoError(t, err)

	// Create shared channels
	scData := []*model.SharedChannel{
		{ChannelID: channel1.ID, TeamID: model.NewID(), Home: true, ShareName: "test_chan_1", CreatorID: model.NewID()},
		{ChannelID: channel2.ID, TeamID: model.NewID(), Home: true, ShareName: "test_chan_2", CreatorID: model.NewID()},
		{ChannelID: channel3.ID, TeamID: model.NewID(), Home: true, ShareName: "test_chan_3", CreatorID: model.NewID()},
	}
	for _, item := range scData {
		_, err := ss.SharedChannel().Save(item)
		require.NoError(t, err)
	}

	// Create some remote clusters
	rcData := []*model.RemoteCluster{
		{Name: "AAAA_Inc", CreatorID: userID, SiteURL: "aaaa.com", RemoteID: model.NewID()},
		{Name: "BBBB_Inc", CreatorID: userID, SiteURL: "bbbb.com", RemoteID: model.NewID()},
		{Name: "CCCC_Inc", CreatorID: userID, SiteURL: "cccc.com", RemoteID: model.NewID()},
		{Name: "DDDD_Inc", CreatorID: userID, SiteURL: "dddd.com", RemoteID: model.NewID()},
		{Name: "EEEE_Inc", CreatorID: userID, SiteURL: "eeee.com", RemoteID: model.NewID()},
	}
	for _, item := range rcData {
		_, err := ss.RemoteCluster().Save(item)
		require.NoError(t, err)
	}

	// Create some shared channel remotes
	scrData := []*model.SharedChannelRemote{
		{ChannelID: channel1.ID, RemoteID: rcData[0].RemoteID, CreatorID: model.NewID()},
		{ChannelID: channel1.ID, RemoteID: rcData[1].RemoteID, CreatorID: model.NewID()},
		{ChannelID: channel2.ID, RemoteID: rcData[2].RemoteID, CreatorID: model.NewID()},
		{ChannelID: channel2.ID, RemoteID: rcData[3].RemoteID, CreatorID: model.NewID()},
		{ChannelID: channel3.ID, RemoteID: rcData[4].RemoteID, CreatorID: model.NewID()},
	}
	for _, item := range scrData {
		_, err := ss.SharedChannel().SaveRemote(item)
		require.NoError(t, err)
	}

	t.Run("Channel 1", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			NotInChannel: channel1.ID,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Len(t, list, 3, "channel 1 should have 3 remote clusters that are not already members")
		ids := getIDs(list)
		require.ElementsMatch(t, []string{rcData[2].RemoteID, rcData[3].RemoteID, rcData[4].RemoteID}, ids)
	})

	t.Run("Channel 2", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			NotInChannel: channel2.ID,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Len(t, list, 3, "channel 2 should have 3 remote clusters that are not already members")
		ids := getIDs(list)
		require.ElementsMatch(t, []string{rcData[0].RemoteID, rcData[1].RemoteID, rcData[4].RemoteID}, ids)
	})

	t.Run("Channel 3", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			NotInChannel: channel3.ID,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Len(t, list, 4, "channel 3 should have 4 remote clusters that are not already members")
		ids := getIDs(list)
		require.ElementsMatch(t, []string{rcData[0].RemoteID, rcData[1].RemoteID, rcData[2].RemoteID, rcData[3].RemoteID}, ids)
	})

	t.Run("Channel with no share remotes", func(t *testing.T) {
		filter := model.RemoteClusterQueryFilter{
			NotInChannel: model.NewID(),
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		require.NoError(t, err)
		require.Len(t, list, 5, "should have 5 remote clusters that are not already members")
		ids := getIDs(list)
		require.ElementsMatch(t, []string{rcData[0].RemoteID, rcData[1].RemoteID, rcData[2].RemoteID, rcData[3].RemoteID,
			rcData[4].RemoteID}, ids)
	})
}

func getIDs(remotes []*model.RemoteCluster) []string {
	ids := make([]string, 0, len(remotes))
	for _, r := range remotes {
		ids = append(ids, r.RemoteID)
	}
	return ids
}

func testRemoteClusterGetByTopic(t *testing.T, ss store.Store) {
	require.NoError(t, clearRemoteClusters(ss))

	rcData := []*model.RemoteCluster{
		{Name: "AAAA_Inc", CreatorID: model.NewID(), SiteURL: "aaaa.com", RemoteID: model.NewID(), Topics: ""},
		{Name: "BBBB_Inc", CreatorID: model.NewID(), SiteURL: "bbbb.com", RemoteID: model.NewID(), Topics: " share "},
		{Name: "CCCC_Inc", CreatorID: model.NewID(), SiteURL: "cccc.com", RemoteID: model.NewID(), Topics: " incident share "},
		{Name: "DDDD_Inc", CreatorID: model.NewID(), SiteURL: "dddd.com", RemoteID: model.NewID(), Topics: " bogus "},
		{Name: "EEEE_Inc", CreatorID: model.NewID(), SiteURL: "eeee.com", RemoteID: model.NewID(), Topics: " logs share incident "},
		{Name: "FFFF_Inc", CreatorID: model.NewID(), SiteURL: "ffff.com", RemoteID: model.NewID(), Topics: " bogus incident "},
		{Name: "GGGG_Inc", CreatorID: model.NewID(), SiteURL: "gggg.com", RemoteID: model.NewID(), Topics: "*"},
	}
	for _, item := range rcData {
		_, err := ss.RemoteCluster().Save(item)
		require.NoError(t, err)
	}

	testData := []struct {
		topic         string
		expectedCount int
		expectError   bool
	}{
		{topic: "", expectedCount: 7, expectError: false},
		{topic: " ", expectedCount: 0, expectError: true},
		{topic: "share", expectedCount: 4},
		{topic: " share ", expectedCount: 4},
		{topic: "bogus", expectedCount: 3},
		{topic: "non-existent", expectedCount: 1},
		{topic: "*", expectedCount: 0, expectError: true}, // can't query with wildcard
	}

	for _, tt := range testData {
		filter := model.RemoteClusterQueryFilter{
			Topic: tt.topic,
		}
		list, err := ss.RemoteCluster().GetAll(filter)
		if tt.expectError {
			assert.Errorf(t, err, "expected error for topic=%s", tt.topic)
		} else {
			assert.NoErrorf(t, err, "expected no error for topic=%s", tt.topic)
		}
		assert.Lenf(t, list, tt.expectedCount, "topic=%s", tt.topic)
	}
}

func testRemoteClusterUpdateTopics(t *testing.T, ss store.Store) {
	remoteID := model.NewID()
	rc := &model.RemoteCluster{
		DisplayName: "Blap Inc",
		Name:        "blap",
		SiteURL:     "blap.com",
		RemoteID:    remoteID,
		Topics:      "",
		CreatorID:   model.NewID(),
	}

	_, err := ss.RemoteCluster().Save(rc)
	require.NoError(t, err)

	testData := []struct {
		topics   string
		expected string
	}{
		{topics: "", expected: ""},
		{topics: " ", expected: ""},
		{topics: "share", expected: " share "},
		{topics: " share ", expected: " share "},
		{topics: "share incident", expected: " share incident "},
		{topics: "  share    incident   ", expected: " share incident "},
	}

	for _, tt := range testData {
		_, err = ss.RemoteCluster().UpdateTopics(remoteID, tt.topics)
		require.NoError(t, err)

		rcUpdated, err := ss.RemoteCluster().Get(remoteID)
		require.NoError(t, err)

		require.Equal(t, tt.expected, rcUpdated.Topics)
	}
}

func clearRemoteClusters(ss store.Store) error {
	list, err := ss.RemoteCluster().GetAll(model.RemoteClusterQueryFilter{})
	if err != nil {
		return err
	}

	for _, rc := range list {
		if _, err := ss.RemoteCluster().Delete(rc.RemoteID); err != nil {
			return err
		}
	}
	return nil
}
