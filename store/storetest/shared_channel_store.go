// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestSharedChannelStore(t *testing.T, ss store.Store, s SqlStore) {
	t.Run("SaveSharedChannel", func(t *testing.T) { testSaveSharedChannel(t, ss) })
	t.Run("GetSharedChannel", func(t *testing.T) { testGetSharedChannel(t, ss) })
	t.Run("HasSharedChannel", func(t *testing.T) { testHasSharedChannel(t, ss) })
	t.Run("GetSharedChannels", func(t *testing.T) { testGetSharedChannels(t, ss) })
	t.Run("UpdateSharedChannel", func(t *testing.T) { testUpdateSharedChannel(t, ss) })
	t.Run("DeleteSharedChannel", func(t *testing.T) { testDeleteSharedChannel(t, ss) })

	t.Run("SaveSharedChannelRemote", func(t *testing.T) { testSaveSharedChannelRemote(t, ss) })
	t.Run("UpdateSharedChannelRemote", func(t *testing.T) { testUpdateSharedChannelRemote(t, ss) })
	t.Run("GetSharedChannelRemote", func(t *testing.T) { testGetSharedChannelRemote(t, ss) })
	t.Run("GetSharedChannelRemoteByIds", func(t *testing.T) { testGetSharedChannelRemoteByIDs(t, ss) })
	t.Run("GetSharedChannelRemotes", func(t *testing.T) { testGetSharedChannelRemotes(t, ss) })
	t.Run("HasRemote", func(t *testing.T) { testHasRemote(t, ss) })
	t.Run("GetRemoteForUser", func(t *testing.T) { testGetRemoteForUser(t, ss) })
	t.Run("UpdateSharedChannelRemoteNextSyncAt", func(t *testing.T) { testUpdateSharedChannelRemoteCursor(t, ss) })
	t.Run("DeleteSharedChannelRemote", func(t *testing.T) { testDeleteSharedChannelRemote(t, ss) })

	t.Run("SaveSharedChannelUser", func(t *testing.T) { testSaveSharedChannelUser(t, ss) })
	t.Run("GetSharedChannelSingleUser", func(t *testing.T) { testGetSingleSharedChannelUser(t, ss) })
	t.Run("GetSharedChannelUser", func(t *testing.T) { testGetSharedChannelUser(t, ss) })
	t.Run("GetSharedChannelUsersForSync", func(t *testing.T) { testGetSharedChannelUsersForSync(t, ss) })
	t.Run("UpdateSharedChannelUserLastSyncAt", func(t *testing.T) { testUpdateSharedChannelUserLastSyncAt(t, ss) })

	t.Run("SaveSharedChannelAttachment", func(t *testing.T) { testSaveSharedChannelAttachment(t, ss) })
	t.Run("UpsertSharedChannelAttachment", func(t *testing.T) { testUpsertSharedChannelAttachment(t, ss) })
	t.Run("GetSharedChannelAttachment", func(t *testing.T) { testGetSharedChannelAttachment(t, ss) })
	t.Run("UpdateSharedChannelAttachmentLastSyncAt", func(t *testing.T) { testUpdateSharedChannelAttachmentLastSyncAt(t, ss) })
}

func testSaveSharedChannel(t *testing.T, ss store.Store) {
	t.Run("Save shared channel (home)", func(t *testing.T) {
		channel, err := createTestChannel(ss, "test_save")
		require.NoError(t, err)

		sc := &model.SharedChannel{
			ChannelID: channel.ID,
			TeamID:    channel.TeamID,
			CreatorID: model.NewID(),
			ShareName: "testshare",
			Home:      true,
		}

		scSaved, err := ss.SharedChannel().Save(sc)
		require.NoError(t, err, "couldn't save shared channel")

		require.Equal(t, sc.ChannelID, scSaved.ChannelID)
		require.Equal(t, sc.TeamID, scSaved.TeamID)
		require.Equal(t, sc.CreatorID, scSaved.CreatorID)

		// ensure channel's Shared flag is set
		channelMod, err := ss.Channel().Get(channel.ID, false)
		require.NoError(t, err)
		require.True(t, channelMod.IsShared())
	})

	t.Run("Save shared channel (remote)", func(t *testing.T) {
		channel, err := createTestChannel(ss, "test_save2")
		require.NoError(t, err)

		sc := &model.SharedChannel{
			ChannelID: channel.ID,
			TeamID:    channel.TeamID,
			CreatorID: model.NewID(),
			ShareName: "testshare",
			RemoteID:  model.NewID(),
		}

		scSaved, err := ss.SharedChannel().Save(sc)
		require.NoError(t, err, "couldn't save shared channel", err)

		require.Equal(t, sc.ChannelID, scSaved.ChannelID)
		require.Equal(t, sc.TeamID, scSaved.TeamID)
		require.Equal(t, sc.CreatorID, scSaved.CreatorID)

		// ensure channel's Shared flag is set
		channelMod, err := ss.Channel().Get(channel.ID, false)
		require.NoError(t, err)
		require.True(t, channelMod.IsShared())
	})

	t.Run("Save invalid shared channel", func(t *testing.T) {
		sc := &model.SharedChannel{
			ChannelID: "",
			TeamID:    model.NewID(),
			CreatorID: model.NewID(),
			ShareName: "testshare",
			Home:      true,
		}

		_, err := ss.SharedChannel().Save(sc)
		require.Error(t, err, "should error saving invalid shared channel", err)
	})

	t.Run("Save with invalid channel id", func(t *testing.T) {
		sc := &model.SharedChannel{
			ChannelID: model.NewID(),
			TeamID:    model.NewID(),
			CreatorID: model.NewID(),
			ShareName: "testshare",
			RemoteID:  model.NewID(),
		}

		_, err := ss.SharedChannel().Save(sc)
		require.Error(t, err, "expected error for invalid channel id")
	})
}

func testGetSharedChannel(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_get")
	require.NoError(t, err)

	sc := &model.SharedChannel{
		ChannelID: channel.ID,
		TeamID:    channel.TeamID,
		CreatorID: model.NewID(),
		ShareName: "testshare",
		Home:      true,
	}

	scSaved, err := ss.SharedChannel().Save(sc)
	require.NoError(t, err, "couldn't save shared channel", err)

	t.Run("Get existing shared channel", func(t *testing.T) {
		sc, err := ss.SharedChannel().Get(scSaved.ChannelID)
		require.NoError(t, err, "couldn't get shared channel", err)

		require.Equal(t, sc.ChannelID, scSaved.ChannelID)
		require.Equal(t, sc.TeamID, scSaved.TeamID)
		require.Equal(t, sc.CreatorID, scSaved.CreatorID)
	})

	t.Run("Get non-existent shared channel", func(t *testing.T) {
		sc, err := ss.SharedChannel().Get(model.NewID())
		require.Error(t, err)
		require.Nil(t, sc)
	})
}

func testHasSharedChannel(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_get")
	require.NoError(t, err)

	sc := &model.SharedChannel{
		ChannelID: channel.ID,
		TeamID:    channel.TeamID,
		CreatorID: model.NewID(),
		ShareName: "testshare",
		Home:      true,
	}

	scSaved, err := ss.SharedChannel().Save(sc)
	require.NoError(t, err, "couldn't save shared channel", err)

	t.Run("Get existing shared channel", func(t *testing.T) {
		exists, err := ss.SharedChannel().HasChannel(scSaved.ChannelID)
		require.NoError(t, err, "couldn't get shared channel", err)
		assert.True(t, exists)
	})

	t.Run("Get non-existent shared channel", func(t *testing.T) {
		exists, err := ss.SharedChannel().HasChannel(model.NewID())
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func testGetSharedChannels(t *testing.T, ss store.Store) {
	require.NoError(t, clearSharedChannels(ss))

	creator := model.NewID()
	team1 := model.NewID()
	team2 := model.NewID()
	rid := model.NewID()

	data := []model.SharedChannel{
		{CreatorID: creator, TeamID: team1, ShareName: "test1", Home: true},
		{CreatorID: creator, TeamID: team1, ShareName: "test2", Home: false, RemoteID: rid},
		{CreatorID: creator, TeamID: team1, ShareName: "test3", Home: false, RemoteID: rid},
		{CreatorID: creator, TeamID: team1, ShareName: "test4", Home: true},
		{CreatorID: creator, TeamID: team2, ShareName: "test5", Home: true},
		{CreatorID: creator, TeamID: team2, ShareName: "test6", Home: false, RemoteID: rid},
		{CreatorID: creator, TeamID: team2, ShareName: "test7", Home: false, RemoteID: rid},
		{CreatorID: creator, TeamID: team2, ShareName: "test8", Home: true},
		{CreatorID: creator, TeamID: team2, ShareName: "test9", Home: true},
	}

	for i, sc := range data {
		channel, err := createTestChannel(ss, "test_get2_"+strconv.Itoa(i))
		require.NoError(t, err)

		sc.ChannelID = channel.ID

		_, err = ss.SharedChannel().Save(&sc)
		require.NoError(t, err, "error saving shared channel")
	}

	t.Run("Get shared channels home only", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			ExcludeRemote: true,
			CreatorID:     creator,
		}

		count, err := ss.SharedChannel().GetAllCount(opts)
		require.NoError(t, err, "error getting shared channels count")

		home, err := ss.SharedChannel().GetAll(0, 100, opts)
		require.NoError(t, err, "error getting shared channels")

		require.Equal(t, int(count), len(home))
		require.Len(t, home, 5, "should be 5 home channels")
		for _, sc := range home {
			require.True(t, sc.Home, "should be home channel")
		}
	})

	t.Run("Get shared channels remote only", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			ExcludeHome: true,
		}

		count, err := ss.SharedChannel().GetAllCount(opts)
		require.NoError(t, err, "error getting shared channels count")

		remotes, err := ss.SharedChannel().GetAll(0, 100, opts)
		require.NoError(t, err, "error getting shared channels")

		require.Equal(t, int(count), len(remotes))
		require.Len(t, remotes, 4, "should be 4 remote channels")
		for _, sc := range remotes {
			require.False(t, sc.Home, "should be remote channel")
		}
	})

	t.Run("Get shared channels bad opts", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			ExcludeHome:   true,
			ExcludeRemote: true,
		}
		_, err := ss.SharedChannel().GetAll(0, 100, opts)
		require.Error(t, err, "error expected")
	})

	t.Run("Get shared channels by team", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			TeamID: team1,
		}

		count, err := ss.SharedChannel().GetAllCount(opts)
		require.NoError(t, err, "error getting shared channels count")

		remotes, err := ss.SharedChannel().GetAll(0, 100, opts)
		require.NoError(t, err, "error getting shared channels")

		require.Equal(t, int(count), len(remotes))
		require.Len(t, remotes, 4, "should be 4 matching channels")
		for _, sc := range remotes {
			require.Equal(t, team1, sc.TeamID)
		}
	})

	t.Run("Get shared channels invalid pagnation", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			TeamID: team1,
		}

		_, err := ss.SharedChannel().GetAll(-1, 100, opts)
		require.Error(t, err)

		_, err = ss.SharedChannel().GetAll(0, -100, opts)
		require.Error(t, err)
	})
}

func testUpdateSharedChannel(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_update")
	require.NoError(t, err)

	sc := &model.SharedChannel{
		ChannelID: channel.ID,
		TeamID:    channel.TeamID,
		CreatorID: model.NewID(),
		ShareName: "testshare",
		Home:      true,
	}

	scSaved, err := ss.SharedChannel().Save(sc)
	require.NoError(t, err, "couldn't save shared channel", err)

	t.Run("Update existing shared channel", func(t *testing.T) {
		id := model.NewID()
		scMod := scSaved // copy struct (contains basic types only)
		scMod.ShareName = "newname"
		scMod.ShareDisplayName = "For testing"
		scMod.ShareHeader = "This is a header."
		scMod.RemoteID = id

		scUpdated, err := ss.SharedChannel().Update(scMod)
		require.NoError(t, err, "couldn't update shared channel", err)

		require.Equal(t, "newname", scUpdated.ShareName)
		require.Equal(t, "For testing", scUpdated.ShareDisplayName)
		require.Equal(t, "This is a header.", scUpdated.ShareHeader)
		require.Equal(t, id, scUpdated.RemoteID)
	})

	t.Run("Update non-existent shared channel", func(t *testing.T) {
		sc := &model.SharedChannel{
			ChannelID: model.NewID(),
			TeamID:    model.NewID(),
			CreatorID: model.NewID(),
			ShareName: "missingshare",
		}
		_, err := ss.SharedChannel().Update(sc)
		require.Error(t, err, "should error when updating non-existent shared channel", err)
	})
}

func testDeleteSharedChannel(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_delete")
	require.NoError(t, err)

	sc := &model.SharedChannel{
		ChannelID: channel.ID,
		TeamID:    channel.TeamID,
		CreatorID: model.NewID(),
		ShareName: "testshare",
		RemoteID:  model.NewID(),
	}

	_, err = ss.SharedChannel().Save(sc)
	require.NoError(t, err, "couldn't save shared channel", err)

	// add some remotes
	for i := 0; i < 10; i++ {
		remote := &model.SharedChannelRemote{
			ChannelID: channel.ID,
			CreatorID: model.NewID(),
			RemoteID:  model.NewID(),
		}
		_, err := ss.SharedChannel().SaveRemote(remote)
		require.NoError(t, err, "couldn't add remote", err)
	}

	t.Run("Delete existing shared channel", func(t *testing.T) {
		deleted, err := ss.SharedChannel().Delete(channel.ID)
		require.NoError(t, err, "delete existing shared channel should not error", err)
		require.True(t, deleted, "expected true from delete shared channel")

		sc, err := ss.SharedChannel().Get(channel.ID)
		require.Error(t, err)
		require.Nil(t, sc)

		// make sure the remotes were deleted.
		remotes, err := ss.SharedChannel().GetRemotes(model.SharedChannelRemoteFilterOpts{ChannelID: channel.ID})
		require.NoError(t, err)
		require.Len(t, remotes, 0, "expected empty remotes list")

		// ensure channel's Shared flag is unset
		channelMod, err := ss.Channel().Get(channel.ID, false)
		require.NoError(t, err)
		require.False(t, channelMod.IsShared())
	})

	t.Run("Delete non-existent shared channel", func(t *testing.T) {
		deleted, err := ss.SharedChannel().Delete(model.NewID())
		require.NoError(t, err, "delete non-existent shared channel should not error", err)
		require.False(t, deleted, "expected false from delete shared channel")
	})
}

func testSaveSharedChannelRemote(t *testing.T, ss store.Store) {
	t.Run("Save shared channel remote", func(t *testing.T) {
		channel, err := createTestChannel(ss, "test_save_remote")
		require.NoError(t, err)

		remote := &model.SharedChannelRemote{
			ChannelID: channel.ID,
			CreatorID: model.NewID(),
			RemoteID:  model.NewID(),
		}

		remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
		require.NoError(t, err, "couldn't save shared channel remote", err)

		require.Equal(t, remote.ChannelID, remoteSaved.ChannelID)
		require.Equal(t, remote.CreatorID, remoteSaved.CreatorID)
	})

	t.Run("Save invalid shared channel remote", func(t *testing.T) {
		remote := &model.SharedChannelRemote{
			ChannelID: "",
			CreatorID: model.NewID(),
			RemoteID:  model.NewID(),
		}

		_, err := ss.SharedChannel().SaveRemote(remote)
		require.Error(t, err, "should error saving invalid remote", err)
	})

	t.Run("Save shared channel remote with invalid channel id", func(t *testing.T) {
		remote := &model.SharedChannelRemote{
			ChannelID: model.NewID(),
			CreatorID: model.NewID(),
			RemoteID:  model.NewID(),
		}

		_, err := ss.SharedChannel().SaveRemote(remote)
		require.Error(t, err, "expected error for invalid channel id")
	})
}

func testUpdateSharedChannelRemote(t *testing.T, ss store.Store) {
	t.Run("Update shared channel remote", func(t *testing.T) {
		channel, err := createTestChannel(ss, "test_update_remote")
		require.NoError(t, err)

		remote := &model.SharedChannelRemote{
			ChannelID: channel.ID,
			CreatorID: model.NewID(),
			RemoteID:  model.NewID(),
		}

		remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
		require.NoError(t, err, "couldn't save shared channel remote", err)

		remoteSaved.IsInviteAccepted = true
		remoteSaved.IsInviteConfirmed = true

		remoteUpdated, err := ss.SharedChannel().UpdateRemote(remoteSaved)
		require.NoError(t, err, "couldn't update shared channel remote", err)

		require.Equal(t, true, remoteUpdated.IsInviteAccepted)
		require.Equal(t, true, remoteUpdated.IsInviteConfirmed)
	})

	t.Run("Update invalid shared channel remote", func(t *testing.T) {
		remote := &model.SharedChannelRemote{
			ChannelID: "",
			CreatorID: model.NewID(),
			RemoteID:  model.NewID(),
		}

		_, err := ss.SharedChannel().UpdateRemote(remote)
		require.Error(t, err, "should error updating invalid remote", err)
	})

	t.Run("Update shared channel remote with invalid channel id", func(t *testing.T) {
		remote := &model.SharedChannelRemote{
			ChannelID: model.NewID(),
			CreatorID: model.NewID(),
			RemoteID:  model.NewID(),
		}

		_, err := ss.SharedChannel().UpdateRemote(remote)
		require.Error(t, err, "expected error for invalid channel id")
	})
}

func testGetSharedChannelRemote(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_remote_get")
	require.NoError(t, err)

	remote := &model.SharedChannelRemote{
		ChannelID: channel.ID,
		CreatorID: model.NewID(),
		RemoteID:  model.NewID(),
	}

	remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
	require.NoError(t, err, "couldn't save remote", err)

	t.Run("Get existing shared channel remote", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemote(remoteSaved.ID)
		require.NoError(t, err, "could not get shared channel remote", err)

		require.Equal(t, remoteSaved.ID, r.ID)
		require.Equal(t, remoteSaved.ChannelID, r.ChannelID)
		require.Equal(t, remoteSaved.CreatorID, r.CreatorID)
		require.Equal(t, remoteSaved.RemoteID, r.RemoteID)
	})

	t.Run("Get non-existent shared channel remote", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemote(model.NewID())
		require.Error(t, err)
		require.Nil(t, r)
	})
}

func testGetSharedChannelRemoteByIDs(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_remote_get_by_ids")
	require.NoError(t, err)

	remote := &model.SharedChannelRemote{
		ChannelID: channel.ID,
		CreatorID: model.NewID(),
		RemoteID:  model.NewID(),
	}

	remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
	require.NoError(t, err, "could not save remote", err)

	t.Run("Get existing shared channel remote by ids", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemoteByIDs(remoteSaved.ChannelID, remoteSaved.RemoteID)
		require.NoError(t, err, "couldn't get shared channel remote by ids", err)

		require.Equal(t, remoteSaved.ID, r.ID)
		require.Equal(t, remoteSaved.ChannelID, r.ChannelID)
		require.Equal(t, remoteSaved.CreatorID, r.CreatorID)
		require.Equal(t, remoteSaved.RemoteID, r.RemoteID)
	})

	t.Run("Get non-existent shared channel remote by ids", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemoteByIDs(model.NewID(), model.NewID())
		require.Error(t, err)
		require.Nil(t, r)
	})
}

func testGetSharedChannelRemotes(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_remotes_get2")
	require.NoError(t, err)

	creator := model.NewID()
	remoteID := model.NewID()

	data := []model.SharedChannelRemote{
		{ChannelID: channel.ID, CreatorID: creator, RemoteID: model.NewID(), IsInviteConfirmed: true},
		{ChannelID: channel.ID, CreatorID: creator, RemoteID: model.NewID(), IsInviteConfirmed: true},
		{ChannelID: channel.ID, CreatorID: creator, RemoteID: model.NewID(), IsInviteConfirmed: true},
		{CreatorID: creator, RemoteID: remoteID, IsInviteConfirmed: true},
		{CreatorID: creator, RemoteID: remoteID, IsInviteConfirmed: true},
		{CreatorID: creator, RemoteID: remoteID},
	}

	for i, r := range data {
		if r.ChannelID == "" {
			c, err := createTestChannel(ss, "test_remotes_get2_"+strconv.Itoa(i))
			require.NoError(t, err)
			r.ChannelID = c.ID
		}
		_, err := ss.SharedChannel().SaveRemote(&r)
		require.NoError(t, err, "error saving shared channel remote")
	}

	t.Run("Get shared channel remotes by channel_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ChannelID: channel.ID,
		}
		remotes, err := ss.SharedChannel().GetRemotes(opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 3)
		for _, r := range remotes {
			require.Equal(t, channel.ID, r.ChannelID)
		}
	})

	t.Run("Get shared channel remotes by invalid channel_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ChannelID: model.NewID(),
		}
		remotes, err := ss.SharedChannel().GetRemotes(opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 0)
	})

	t.Run("Get shared channel remotes by remote_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			RemoteID: remoteID,
		}
		remotes, err := ss.SharedChannel().GetRemotes(opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 2) // only confirmed invitations
		for _, r := range remotes {
			require.Equal(t, remoteID, r.RemoteID)
			require.True(t, r.IsInviteConfirmed)
		}
	})

	t.Run("Get shared channel remotes by invalid remote_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			RemoteID: model.NewID(),
		}
		remotes, err := ss.SharedChannel().GetRemotes(opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 0)
	})

	t.Run("Get shared channel remotes by remote_id including unconfirmed", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			RemoteID:        remoteID,
			InclUnconfirmed: true,
		}
		remotes, err := ss.SharedChannel().GetRemotes(opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 3)
		for _, r := range remotes {
			require.Equal(t, remoteID, r.RemoteID)
		}
	})
}

func testHasRemote(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_remotes_get2")
	require.NoError(t, err)

	remote1 := model.NewID()
	remote2 := model.NewID()

	creator := model.NewID()
	data := []model.SharedChannelRemote{
		{ChannelID: channel.ID, CreatorID: creator, RemoteID: remote1},
		{ChannelID: channel.ID, CreatorID: creator, RemoteID: remote2},
	}

	for _, r := range data {
		_, err := ss.SharedChannel().SaveRemote(&r)
		require.NoError(t, err, "error saving shared channel remote")
	}

	t.Run("has remote", func(t *testing.T) {
		has, err := ss.SharedChannel().HasRemote(channel.ID, remote1)
		require.NoError(t, err)
		assert.True(t, has)

		has, err = ss.SharedChannel().HasRemote(channel.ID, remote2)
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("wrong channel id ", func(t *testing.T) {
		has, err := ss.SharedChannel().HasRemote(model.NewID(), remote1)
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("wrong remote id", func(t *testing.T) {
		has, err := ss.SharedChannel().HasRemote(channel.ID, model.NewID())
		require.NoError(t, err)
		assert.False(t, has)
	})
}

func testGetRemoteForUser(t *testing.T, ss store.Store) {
	// add remotes, and users to simulated shared channels.
	teamID := model.NewID()
	channel, err := createSharedTestChannel(ss, "share_test_channel", true)
	require.NoError(t, err)
	remotes := []*model.RemoteCluster{
		{RemoteID: model.NewID(), SiteURL: model.NewID(), CreatorID: model.NewID(), RemoteTeamID: teamID, Name: "Test_Remote_1"},
		{RemoteID: model.NewID(), SiteURL: model.NewID(), CreatorID: model.NewID(), RemoteTeamID: teamID, Name: "Test_Remote_2"},
		{RemoteID: model.NewID(), SiteURL: model.NewID(), CreatorID: model.NewID(), RemoteTeamID: teamID, Name: "Test_Remote_3"},
	}
	var channelRemotes []*model.SharedChannelRemote
	for _, rc := range remotes {
		_, err := ss.RemoteCluster().Save(rc)
		require.NoError(t, err)

		scr := &model.SharedChannelRemote{ID: model.NewID(), CreatorID: rc.CreatorID, ChannelID: channel.ID, RemoteID: rc.RemoteID}
		scr, err = ss.SharedChannel().SaveRemote(scr)
		require.NoError(t, err)
		channelRemotes = append(channelRemotes, scr)
	}
	users := []string{model.NewID(), model.NewID(), model.NewID()}
	for _, id := range users {
		member := &model.ChannelMember{
			ChannelID:   channel.ID,
			UserID:      id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
			SchemeGuest: false,
			SchemeUser:  true,
		}
		_, err := ss.Channel().SaveMember(member)
		require.NoError(t, err)
	}

	t.Run("user is member", func(t *testing.T) {
		for _, rc := range remotes {
			for _, userID := range users {
				rcFound, err := ss.SharedChannel().GetRemoteForUser(rc.RemoteID, userID)
				assert.NoError(t, err, "remote should be found for user")
				assert.Equal(t, rc.RemoteID, rcFound.RemoteID, "remoteIds should match")
			}
		}
	})

	t.Run("user is not a member", func(t *testing.T) {
		for _, rc := range remotes {
			rcFound, err := ss.SharedChannel().GetRemoteForUser(rc.RemoteID, model.NewID())
			assert.Error(t, err, "remote should not be found for user")
			assert.Nil(t, rcFound)
		}
	})

	t.Run("unknown remote id", func(t *testing.T) {
		rcFound, err := ss.SharedChannel().GetRemoteForUser(model.NewID(), users[0])
		assert.Error(t, err, "remote should not be found for unknown remote id")
		assert.Nil(t, rcFound)
	})
}

func testUpdateSharedChannelRemoteCursor(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_remote_update_next_sync_at")
	require.NoError(t, err)

	remote := &model.SharedChannelRemote{
		ChannelID: channel.ID,
		CreatorID: model.NewID(),
		RemoteID:  model.NewID(),
	}

	remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
	require.NoError(t, err, "couldn't save remote", err)

	future := model.GetMillis() + 3600000 // 1 hour in the future
	postID := model.NewID()

	cursor := model.GetPostsSinceForSyncCursor{
		LastPostUpdateAt: future,
		LastPostID:       postID,
	}

	t.Run("Update NextSyncAt for remote", func(t *testing.T) {
		err := ss.SharedChannel().UpdateRemoteCursor(remoteSaved.ID, cursor)
		require.NoError(t, err, "update NextSyncAt should not error", err)

		r, err := ss.SharedChannel().GetRemote(remoteSaved.ID)
		require.NoError(t, err)
		require.Equal(t, future, r.LastPostUpdateAt)
		require.Equal(t, postID, r.LastPostID)
	})

	t.Run("Update NextSyncAt for non-existent shared channel remote", func(t *testing.T) {
		err := ss.SharedChannel().UpdateRemoteCursor(model.NewID(), cursor)
		require.Error(t, err, "update non-existent remote should error", err)
	})
}

func testDeleteSharedChannelRemote(t *testing.T, ss store.Store) {
	channel, err := createTestChannel(ss, "test_remote_delete")
	require.NoError(t, err)

	remote := &model.SharedChannelRemote{
		ChannelID: channel.ID,
		CreatorID: model.NewID(),
		RemoteID:  model.NewID(),
	}

	remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
	require.NoError(t, err, "couldn't save remote", err)

	t.Run("Delete existing shared channel remote", func(t *testing.T) {
		deleted, err := ss.SharedChannel().DeleteRemote(remoteSaved.ID)
		require.NoError(t, err, "delete existing remote should not error", err)
		require.True(t, deleted, "expected true from delete remote")

		r, err := ss.SharedChannel().GetRemote(remoteSaved.ID)
		require.Error(t, err)
		require.Nil(t, r)
	})

	t.Run("Delete non-existent shared channel remote", func(t *testing.T) {
		deleted, err := ss.SharedChannel().DeleteRemote(model.NewID())
		require.NoError(t, err, "delete non-existent remote should not error", err)
		require.False(t, deleted, "expected false from delete remote")
	})
}

func createTestChannel(ss store.Store, name string) (*model.Channel, error) {
	channel, err := createSharedTestChannel(ss, name, false)
	return channel, err
}

func createSharedTestChannel(ss store.Store, name string, shared bool) (*model.Channel, error) {
	channel := &model.Channel{
		TeamID:      model.NewID(),
		Type:        model.ChannelTypeOpen,
		Name:        name,
		DisplayName: name + " display name",
		Header:      name + " header",
		Purpose:     name + "purpose",
		CreatorID:   model.NewID(),
		Shared:      model.NewBool(shared),
	}
	channel, err := ss.Channel().Save(channel, 10000)
	if err != nil {
		return nil, err
	}

	if shared {
		sc := &model.SharedChannel{
			ChannelID: channel.ID,
			TeamID:    channel.TeamID,
			CreatorID: channel.CreatorID,
			ShareName: channel.Name,
			Home:      true,
		}
		_, err = ss.SharedChannel().Save(sc)
		if err != nil {
			return nil, err
		}
	}
	return channel, nil
}

func clearSharedChannels(ss store.Store) error {
	opts := model.SharedChannelFilterOpts{}
	all, err := ss.SharedChannel().GetAll(0, 1000, opts)
	if err != nil {
		return err
	}

	for _, sc := range all {
		if _, err := ss.SharedChannel().Delete(sc.ChannelID); err != nil {
			return err
		}
	}
	return nil
}

func testSaveSharedChannelUser(t *testing.T, ss store.Store) {
	t.Run("Save shared channel user", func(t *testing.T) {
		scUser := &model.SharedChannelUser{
			UserID:    model.NewID(),
			RemoteID:  model.NewID(),
			ChannelID: model.NewID(),
		}

		userSaved, err := ss.SharedChannel().SaveUser(scUser)
		require.NoError(t, err, "couldn't save shared channel user", err)

		require.Equal(t, scUser.UserID, userSaved.UserID)
		require.Equal(t, scUser.RemoteID, userSaved.RemoteID)
	})

	t.Run("Save invalid shared channel user", func(t *testing.T) {
		scUser := &model.SharedChannelUser{
			UserID:   "",
			RemoteID: model.NewID(),
		}

		_, err := ss.SharedChannel().SaveUser(scUser)
		require.Error(t, err, "should error saving invalid user", err)
	})

	t.Run("Save shared channel user with invalid remote id", func(t *testing.T) {
		scUser := &model.SharedChannelUser{
			UserID:   model.NewID(),
			RemoteID: "bogus",
		}

		_, err := ss.SharedChannel().SaveUser(scUser)
		require.Error(t, err, "expected error for invalid remote id")
	})
}

func testGetSingleSharedChannelUser(t *testing.T, ss store.Store) {
	scUser := &model.SharedChannelUser{
		UserID:    model.NewID(),
		RemoteID:  model.NewID(),
		ChannelID: model.NewID(),
	}

	userSaved, err := ss.SharedChannel().SaveUser(scUser)
	require.NoError(t, err, "could not save user", err)

	t.Run("Get existing shared channel user", func(t *testing.T) {
		r, err := ss.SharedChannel().GetSingleUser(userSaved.UserID, userSaved.ChannelID, userSaved.RemoteID)
		require.NoError(t, err, "couldn't get shared channel user", err)

		require.Equal(t, userSaved.ID, r.ID)
		require.Equal(t, userSaved.UserID, r.UserID)
		require.Equal(t, userSaved.RemoteID, r.RemoteID)
		require.Equal(t, userSaved.CreateAt, r.CreateAt)
	})

	t.Run("Get non-existent shared channel user", func(t *testing.T) {
		u, err := ss.SharedChannel().GetSingleUser(model.NewID(), model.NewID(), model.NewID())
		require.Error(t, err)
		require.Nil(t, u)
	})
}

func testGetSharedChannelUser(t *testing.T, ss store.Store) {
	userID := model.NewID()
	for i := 0; i < 10; i++ {
		scUser := &model.SharedChannelUser{
			UserID:    userID,
			RemoteID:  model.NewID(),
			ChannelID: model.NewID(),
		}
		_, err := ss.SharedChannel().SaveUser(scUser)
		require.NoError(t, err, "could not save user", err)
	}

	t.Run("Get existing shared channel user", func(t *testing.T) {
		scus, err := ss.SharedChannel().GetUsersForUser(userID)
		require.NoError(t, err, "couldn't get shared channel user", err)

		require.Len(t, scus, 10, "should be 10 shared channel user records")
		require.Equal(t, userID, scus[0].UserID)
	})

	t.Run("Get non-existent shared channel user", func(t *testing.T) {
		scus, err := ss.SharedChannel().GetUsersForUser(model.NewID())
		require.NoError(t, err, "should not error when not found")
		require.Empty(t, scus, "should be empty")
	})
}

func testGetSharedChannelUsersForSync(t *testing.T, ss store.Store) {
	channelID := model.NewID()
	remoteID := model.NewID()
	earlier := model.GetMillis() - 300000
	later := model.GetMillis() + 300000

	var users []*model.User
	for i := 0; i < 10; i++ { // need real users
		u := &model.User{
			Username:          model.NewID(),
			Email:             model.NewID() + "@example.com",
			LastPictureUpdate: model.GetMillis(),
		}
		u, err := ss.User().Save(u)
		require.NoError(t, err)
		users = append(users, u)
	}

	data := []model.SharedChannelUser{
		{UserID: users[0].ID, ChannelID: model.NewID(), RemoteID: model.NewID(), LastSyncAt: later},
		{UserID: users[1].ID, ChannelID: model.NewID(), RemoteID: model.NewID(), LastSyncAt: earlier},
		{UserID: users[1].ID, ChannelID: model.NewID(), RemoteID: model.NewID(), LastSyncAt: earlier},
		{UserID: users[1].ID, ChannelID: channelID, RemoteID: remoteID, LastSyncAt: later},
		{UserID: users[2].ID, ChannelID: channelID, RemoteID: model.NewID(), LastSyncAt: later},
		{UserID: users[3].ID, ChannelID: channelID, RemoteID: model.NewID(), LastSyncAt: earlier},
		{UserID: users[4].ID, ChannelID: channelID, RemoteID: model.NewID(), LastSyncAt: later},
		{UserID: users[5].ID, ChannelID: channelID, RemoteID: remoteID, LastSyncAt: earlier},
		{UserID: users[6].ID, ChannelID: channelID, RemoteID: remoteID, LastSyncAt: later},
	}

	for i, u := range data {
		scu := &model.SharedChannelUser{
			UserID:     u.UserID,
			ChannelID:  u.ChannelID,
			RemoteID:   u.RemoteID,
			LastSyncAt: u.LastSyncAt,
		}
		_, err := ss.SharedChannel().SaveUser(scu)
		require.NoError(t, err, "could not save user #", i, err)
	}

	t.Run("Filter by channelId", func(t *testing.T) {
		filter := model.GetUsersForSyncFilter{
			CheckProfileImage: false,
			ChannelID:         channelID,
		}
		usersFound, err := ss.SharedChannel().GetUsersForSync(filter)
		require.NoError(t, err, "shouldn't error getting users", err)
		require.Len(t, usersFound, 2)
		for _, user := range usersFound {
			require.Contains(t, []string{users[3].ID, users[5].ID}, user.ID)
		}
	})

	t.Run("Filter by channelId for profile image", func(t *testing.T) {
		filter := model.GetUsersForSyncFilter{
			CheckProfileImage: true,
			ChannelID:         channelID,
		}
		usersFound, err := ss.SharedChannel().GetUsersForSync(filter)
		require.NoError(t, err, "shouldn't error getting users", err)
		require.Len(t, usersFound, 2)
		for _, user := range usersFound {
			require.Contains(t, []string{users[3].ID, users[5].ID}, user.ID)
		}
	})

	t.Run("Filter by channelId with Limit", func(t *testing.T) {
		filter := model.GetUsersForSyncFilter{
			CheckProfileImage: true,
			ChannelID:         channelID,
			Limit:             1,
		}
		usersFound, err := ss.SharedChannel().GetUsersForSync(filter)
		require.NoError(t, err, "shouldn't error getting users", err)
		require.Len(t, usersFound, 1)
	})
}

func testUpdateSharedChannelUserLastSyncAt(t *testing.T, ss store.Store) {
	u1 := &model.User{
		Username:          model.NewID(),
		Email:             model.NewID() + "@example.com",
		LastPictureUpdate: model.GetMillis() - 300000, // 5 mins
	}
	u1, err := ss.User().Save(u1)
	require.NoError(t, err)

	u2 := &model.User{
		Username:          model.NewID(),
		Email:             model.NewID() + "@example.com",
		LastPictureUpdate: model.GetMillis() + 300000,
	}
	u2, err = ss.User().Save(u2)
	require.NoError(t, err)

	channelID := model.NewID()
	remoteID := model.NewID()

	scUser1 := &model.SharedChannelUser{
		UserID:    u1.ID,
		RemoteID:  remoteID,
		ChannelID: channelID,
	}
	_, err = ss.SharedChannel().SaveUser(scUser1)
	require.NoError(t, err, "couldn't save user", err)

	scUser2 := &model.SharedChannelUser{
		UserID:    u2.ID,
		RemoteID:  remoteID,
		ChannelID: channelID,
	}
	_, err = ss.SharedChannel().SaveUser(scUser2)
	require.NoError(t, err, "couldn't save user", err)

	t.Run("Update LastSyncAt for user via UpdateAt", func(t *testing.T) {
		err := ss.SharedChannel().UpdateUserLastSyncAt(u1.ID, channelID, remoteID)
		require.NoError(t, err, "updateLastSyncAt should not error", err)

		scu, err := ss.SharedChannel().GetSingleUser(u1.ID, channelID, remoteID)
		require.NoError(t, err)
		require.Equal(t, u1.UpdateAt, scu.LastSyncAt)
	})

	t.Run("Update LastSyncAt for user via LastPictureUpdate", func(t *testing.T) {
		err := ss.SharedChannel().UpdateUserLastSyncAt(u2.ID, channelID, remoteID)
		require.NoError(t, err, "updateLastSyncAt should not error", err)

		scu, err := ss.SharedChannel().GetSingleUser(u2.ID, channelID, remoteID)
		require.NoError(t, err)
		require.Equal(t, u2.LastPictureUpdate, scu.LastSyncAt)
	})

	t.Run("Update LastSyncAt for non-existent shared channel user", func(t *testing.T) {
		err := ss.SharedChannel().UpdateUserLastSyncAt(model.NewID(), channelID, remoteID)
		require.Error(t, err, "update non-existent user should error", err)
	})
}

func testSaveSharedChannelAttachment(t *testing.T, ss store.Store) {
	t.Run("Save shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileID:   model.NewID(),
			RemoteID: model.NewID(),
		}

		saved, err := ss.SharedChannel().SaveAttachment(attachment)
		require.NoError(t, err, "couldn't save shared channel attachment", err)

		require.Equal(t, attachment.FileID, saved.FileID)
		require.Equal(t, attachment.RemoteID, saved.RemoteID)
	})

	t.Run("Save invalid shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileID:   "",
			RemoteID: model.NewID(),
		}

		_, err := ss.SharedChannel().SaveAttachment(attachment)
		require.Error(t, err, "should error saving invalid attachment", err)
	})

	t.Run("Save shared channel attachment with invalid remote id", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileID:   model.NewID(),
			RemoteID: "bogus",
		}

		_, err := ss.SharedChannel().SaveAttachment(attachment)
		require.Error(t, err, "expected error for invalid remote id")
	})
}

func testUpsertSharedChannelAttachment(t *testing.T, ss store.Store) {
	t.Run("Upsert new shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileID:   model.NewID(),
			RemoteID: model.NewID(),
		}

		_, err := ss.SharedChannel().UpsertAttachment(attachment)
		require.NoError(t, err, "couldn't upsert shared channel attachment", err)

		saved, err := ss.SharedChannel().GetAttachment(attachment.FileID, attachment.RemoteID)
		require.NoError(t, err, "couldn't get shared channel attachment", err)

		require.NotZero(t, saved.CreateAt)
		require.Equal(t, saved.CreateAt, saved.LastSyncAt)
	})

	t.Run("Upsert existing shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileID:   model.NewID(),
			RemoteID: model.NewID(),
		}

		saved, err := ss.SharedChannel().SaveAttachment(attachment)
		require.NoError(t, err, "couldn't save shared channel attachment", err)

		// make sure enough time passed that GetMillis returns a different value
		time.Sleep(1 * time.Millisecond)

		_, err = ss.SharedChannel().UpsertAttachment(saved)
		require.NoError(t, err, "couldn't upsert shared channel attachment", err)

		updated, err := ss.SharedChannel().GetAttachment(attachment.FileID, attachment.RemoteID)
		require.NoError(t, err, "couldn't get shared channel attachment", err)

		require.NotZero(t, updated.CreateAt)
		require.Greater(t, updated.LastSyncAt, updated.CreateAt)
	})

	t.Run("Upsert invalid shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileID:   "",
			RemoteID: model.NewID(),
		}

		id, err := ss.SharedChannel().UpsertAttachment(attachment)
		require.Error(t, err, "should error upserting invalid attachment", err)
		require.Empty(t, id)
	})

	t.Run("Upsert shared channel attachment with invalid remote id", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileID:   model.NewID(),
			RemoteID: "bogus",
		}

		id, err := ss.SharedChannel().UpsertAttachment(attachment)
		require.Error(t, err, "expected error for invalid remote id")
		require.Empty(t, id)
	})
}

func testGetSharedChannelAttachment(t *testing.T, ss store.Store) {
	attachment := &model.SharedChannelAttachment{
		FileID:   model.NewID(),
		RemoteID: model.NewID(),
	}

	saved, err := ss.SharedChannel().SaveAttachment(attachment)
	require.NoError(t, err, "could not save attachment", err)

	t.Run("Get existing shared channel attachment", func(t *testing.T) {
		r, err := ss.SharedChannel().GetAttachment(saved.FileID, saved.RemoteID)
		require.NoError(t, err, "couldn't get shared channel attachment", err)

		require.Equal(t, saved.ID, r.ID)
		require.Equal(t, saved.FileID, r.FileID)
		require.Equal(t, saved.RemoteID, r.RemoteID)
		require.Equal(t, saved.CreateAt, r.CreateAt)
	})

	t.Run("Get non-existent shared channel attachment", func(t *testing.T) {
		u, err := ss.SharedChannel().GetAttachment(model.NewID(), model.NewID())
		require.Error(t, err)
		require.Nil(t, u)
	})
}

func testUpdateSharedChannelAttachmentLastSyncAt(t *testing.T, ss store.Store) {
	attachment := &model.SharedChannelAttachment{
		FileID:   model.NewID(),
		RemoteID: model.NewID(),
	}

	saved, err := ss.SharedChannel().SaveAttachment(attachment)
	require.NoError(t, err, "couldn't save attachment", err)

	future := model.GetMillis() + 3600000 // 1 hour in the future

	t.Run("Update LastSyncAt for attachment", func(t *testing.T) {
		err := ss.SharedChannel().UpdateAttachmentLastSyncAt(saved.ID, future)
		require.NoError(t, err, "updateLastSyncAt should not error", err)

		f, err := ss.SharedChannel().GetAttachment(saved.FileID, saved.RemoteID)
		require.NoError(t, err)
		require.Equal(t, future, f.LastSyncAt)
	})

	t.Run("Update LastSyncAt for non-existent shared channel attachment", func(t *testing.T) {
		err := ss.SharedChannel().UpdateAttachmentLastSyncAt(model.NewID(), future)
		require.Error(t, err, "update non-existent attachment should error", err)
	})
}
