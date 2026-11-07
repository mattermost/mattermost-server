// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestSharedChannelStore(t *testing.T, rctx request.CTX, ss store.Store, s SqlStore) {
	t.Run("SaveSharedChannel", func(t *testing.T) { testSaveSharedChannel(t, rctx, ss) })
	t.Run("GetSharedChannel", func(t *testing.T) { testGetSharedChannel(t, rctx, ss) })
	t.Run("HasSharedChannel", func(t *testing.T) { testHasSharedChannel(t, rctx, ss) })
	t.Run("GetSharedChannels", func(t *testing.T) { testGetSharedChannels(t, rctx, ss) })
	t.Run("UpdateSharedChannel", func(t *testing.T) { testUpdateSharedChannel(t, rctx, ss) })
	t.Run("DeleteSharedChannel", func(t *testing.T) { testDeleteSharedChannel(t, rctx, ss) })

	t.Run("SaveSharedChannelRemote", func(t *testing.T) { testSaveSharedChannelRemote(t, rctx, ss) })
	t.Run("UpdateSharedChannelRemote", func(t *testing.T) { testUpdateSharedChannelRemote(t, rctx, ss) })
	t.Run("GetSharedChannelRemote", func(t *testing.T) { testGetSharedChannelRemote(t, rctx, ss) })
	t.Run("GetSharedChannelRemoteByIds", func(t *testing.T) { testGetSharedChannelRemoteByIds(t, rctx, ss) })
	t.Run("GetSharedChannelRemotes", func(t *testing.T) { testGetSharedChannelRemotes(t, rctx, ss) })
	t.Run("GetRemotesStatus", func(t *testing.T) { testGetRemotesStatus(t, rctx, ss) })
	t.Run("HasRemote", func(t *testing.T) { testHasRemote(t, rctx, ss) })
	t.Run("GetRemoteChannelIds", func(t *testing.T) { testGetRemoteChannelIds(t, rctx, ss) })
	t.Run("GetRemoteForUser", func(t *testing.T) { testGetRemoteForUser(t, rctx, ss) })
	t.Run("UpdateSharedChannelRemoteNextSyncAt", func(t *testing.T) { testUpdateSharedChannelRemoteCursor(t, rctx, ss) })
	t.Run("UpdateGlobalUserSyncCursor", func(t *testing.T) { testUpdateGlobalUserSyncCursor(t, rctx, ss) })
	t.Run("DeleteSharedChannelRemote", func(t *testing.T) { testDeleteSharedChannelRemote(t, rctx, ss) })

	t.Run("SaveSharedChannelUser", func(t *testing.T) { testSaveSharedChannelUser(t, rctx, ss) })
	t.Run("GetSharedChannelSingleUser", func(t *testing.T) { testGetSingleSharedChannelUser(t, rctx, ss) })
	t.Run("GetSharedChannelUser", func(t *testing.T) { testGetSharedChannelUser(t, rctx, ss) })
	t.Run("GetSharedChannelUsersForSync", func(t *testing.T) { testGetSharedChannelUsersForSync(t, rctx, ss) })
	t.Run("UpdateSharedChannelUserLastSyncAt", func(t *testing.T) { testUpdateSharedChannelUserLastSyncAt(t, rctx, ss) })

	t.Run("SaveSharedChannelAttachment", func(t *testing.T) { testSaveSharedChannelAttachment(t, rctx, ss) })
	t.Run("UpsertSharedChannelAttachment", func(t *testing.T) { testUpsertSharedChannelAttachment(t, rctx, ss) })
	t.Run("GetSharedChannelAttachment", func(t *testing.T) { testGetSharedChannelAttachment(t, rctx, ss) })
	t.Run("UpdateSharedChannelAttachmentLastSyncAt", func(t *testing.T) { testUpdateSharedChannelAttachmentLastSyncAt(t, rctx, ss) })
}

func testSaveSharedChannel(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("Save shared channel (home)", func(t *testing.T) {
		channel, err := createTestChannel(ss, rctx, "test_save")
		require.NoError(t, err)

		sc := &model.SharedChannel{
			ChannelId: channel.Id,
			TeamId:    channel.TeamId,
			CreatorId: model.NewId(),
			ShareName: "testshare",
			Home:      true,
		}

		scSaved, err := ss.SharedChannel().Save(sc)
		require.NoError(t, err, "couldn't save shared channel")

		require.Equal(t, sc.ChannelId, scSaved.ChannelId)
		require.Equal(t, sc.TeamId, scSaved.TeamId)
		require.Equal(t, sc.CreatorId, scSaved.CreatorId)

		// ensure channel's Shared flag is set
		channelMod, err := ss.Channel().Get(channel.Id, false)
		require.NoError(t, err)
		require.True(t, channelMod.IsShared())
	})

	t.Run("Save shared channel (remote)", func(t *testing.T) {
		channel, err := createTestChannel(ss, rctx, "test_save2")
		require.NoError(t, err)

		sc := &model.SharedChannel{
			ChannelId: channel.Id,
			TeamId:    channel.TeamId,
			CreatorId: model.NewId(),
			ShareName: "testshare",
			RemoteId:  model.NewId(),
		}

		scSaved, err := ss.SharedChannel().Save(sc)
		require.NoError(t, err, "couldn't save shared channel", err)

		require.Equal(t, sc.ChannelId, scSaved.ChannelId)
		require.Equal(t, sc.TeamId, scSaved.TeamId)
		require.Equal(t, sc.CreatorId, scSaved.CreatorId)

		// ensure channel's Shared flag is set
		channelMod, err := ss.Channel().Get(channel.Id, false)
		require.NoError(t, err)
		require.True(t, channelMod.IsShared())
	})

	t.Run("Save invalid shared channel", func(t *testing.T) {
		sc := &model.SharedChannel{
			ChannelId: "",
			TeamId:    model.NewId(),
			CreatorId: model.NewId(),
			ShareName: "testshare",
			Home:      true,
		}

		_, err := ss.SharedChannel().Save(sc)
		require.Error(t, err, "should error saving invalid shared channel", err)
	})

	t.Run("Save with invalid channel id", func(t *testing.T) {
		sc := &model.SharedChannel{
			ChannelId: model.NewId(),
			TeamId:    model.NewId(),
			CreatorId: model.NewId(),
			ShareName: "testshare",
			RemoteId:  model.NewId(),
		}

		_, err := ss.SharedChannel().Save(sc)
		require.Error(t, err, "expected error for invalid channel id")
	})
}

func testGetSharedChannel(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_get")
	require.NoError(t, err)

	sc := &model.SharedChannel{
		ChannelId: channel.Id,
		TeamId:    channel.TeamId,
		CreatorId: model.NewId(),
		ShareName: "testshare",
		Home:      true,
	}

	scSaved, err := ss.SharedChannel().Save(sc)
	require.NoError(t, err, "couldn't save shared channel", err)

	t.Run("Get existing shared channel", func(t *testing.T) {
		sc, err := ss.SharedChannel().Get(scSaved.ChannelId)
		require.NoError(t, err, "couldn't get shared channel", err)

		require.Equal(t, sc.ChannelId, scSaved.ChannelId)
		require.Equal(t, sc.TeamId, scSaved.TeamId)
		require.Equal(t, sc.CreatorId, scSaved.CreatorId)
	})

	t.Run("Get non-existent shared channel", func(t *testing.T) {
		sc, err := ss.SharedChannel().Get(model.NewId())
		require.Error(t, err)
		require.Nil(t, sc)
	})
}

func testHasSharedChannel(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_get")
	require.NoError(t, err)

	sc := &model.SharedChannel{
		ChannelId: channel.Id,
		TeamId:    channel.TeamId,
		CreatorId: model.NewId(),
		ShareName: "testshare",
		Home:      true,
	}

	scSaved, err := ss.SharedChannel().Save(sc)
	require.NoError(t, err, "couldn't save shared channel", err)

	t.Run("Get existing shared channel", func(t *testing.T) {
		exists, err := ss.SharedChannel().HasChannel(scSaved.ChannelId)
		require.NoError(t, err, "couldn't get shared channel", err)
		assert.True(t, exists)
	})

	t.Run("Get non-existent shared channel", func(t *testing.T) {
		exists, err := ss.SharedChannel().HasChannel(model.NewId())
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

// testGetRemoteChannelIds covers SharedChannelStore.GetRemoteChannelIds,
// added for MM-70717's shared/remote channel exclusion (D7).
func testGetRemoteChannelIds(t *testing.T, rctx request.CTX, ss store.Store) {
	homeChannel, err := createTestChannel(ss, rctx, "home_"+model.NewId())
	require.NoError(t, err)
	_, err = ss.SharedChannel().Save(&model.SharedChannel{
		ChannelId: homeChannel.Id,
		TeamId:    homeChannel.TeamId,
		CreatorId: model.NewId(),
		ShareName: "home_share_" + model.NewId(),
		Home:      true,
	})
	require.NoError(t, err)

	remoteChannel, err := createTestChannel(ss, rctx, "remote_"+model.NewId())
	require.NoError(t, err)
	_, err = ss.SharedChannel().Save(&model.SharedChannel{
		ChannelId: remoteChannel.Id,
		TeamId:    remoteChannel.TeamId,
		CreatorId: model.NewId(),
		ShareName: "remote_share_" + model.NewId(),
		Home:      false,
		RemoteId:  model.NewId(),
	})
	require.NoError(t, err)

	unsharedChannel, err := createTestChannel(ss, rctx, "unshared_"+model.NewId())
	require.NoError(t, err)

	t.Run("returns only the remote (non-home) channel", func(t *testing.T) {
		ids, err := ss.SharedChannel().GetRemoteChannelIds([]string{homeChannel.Id, remoteChannel.Id, unsharedChannel.Id})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{remoteChannel.Id}, ids)
	})

	t.Run("empty input returns no error", func(t *testing.T) {
		ids, err := ss.SharedChannel().GetRemoteChannelIds([]string{})
		require.NoError(t, err)
		assert.Empty(t, ids)
	})
}

func testGetSharedChannels(t *testing.T, rctx request.CTX, ss store.Store) {
	require.NoError(t, clearSharedChannels(ss))
	user, err := createTestUser(rctx, ss, "gary.goodspeed")
	require.NoError(t, err)

	creator := model.NewId()
	team1 := model.NewId()
	team2 := model.NewId()
	rid1 := model.NewId()
	rid2 := model.NewId()

	data := []model.SharedChannel{
		{CreatorId: creator, TeamId: team1, ShareName: "test1", Home: true},
		{CreatorId: creator, TeamId: team1, ShareName: "test2", Home: false, RemoteId: rid1},
		{CreatorId: creator, TeamId: team1, ShareName: "test3", Home: false, RemoteId: rid2},
		{CreatorId: creator, TeamId: team1, ShareName: "test4", Home: true},
		{CreatorId: creator, TeamId: team2, ShareName: "test5", Home: true},
		{CreatorId: creator, TeamId: team2, ShareName: "test6", Home: false, RemoteId: rid1},
		{CreatorId: creator, TeamId: team2, ShareName: "test7", Home: false, RemoteId: rid2},
		{CreatorId: creator, TeamId: team2, ShareName: "test8", Home: true},
		{CreatorId: creator, TeamId: team2, ShareName: "test9", Home: true},
	}

	for i, sc := range data {
		channel, err := createTestChannelWithUser(ss, rctx, "test_get2_"+strconv.Itoa(i), user)
		require.NoError(t, err)

		sc.ChannelId = channel.Id

		_, err = ss.SharedChannel().Save(&sc)
		require.NoError(t, err, "error saving shared channel")
	}

	t.Run("Get shared channels home only", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			ExcludeRemote: true,
			CreatorId:     creator,
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
			TeamId: team1,
		}

		count, err := ss.SharedChannel().GetAllCount(opts)
		require.NoError(t, err, "error getting shared channels count")

		remotes, err := ss.SharedChannel().GetAll(0, 100, opts)
		require.NoError(t, err, "error getting shared channels")

		require.Equal(t, int(count), len(remotes))
		require.Len(t, remotes, 4, "should be 4 matching channels")
		for _, sc := range remotes {
			require.Equal(t, team1, sc.TeamId)
		}
	})

	t.Run("Get shared channels invalid pagination", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			TeamId: team1,
		}

		_, err := ss.SharedChannel().GetAll(-1, 100, opts)
		require.Error(t, err)

		_, err = ss.SharedChannel().GetAll(0, -100, opts)
		require.Error(t, err)
	})

	t.Run("Get shared channels for member", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			TeamId:   team1,
			MemberId: user.Id,
		}

		count, err := ss.SharedChannel().GetAllCount(opts)
		require.NoError(t, err, "error getting shared channels count")

		remotes, err := ss.SharedChannel().GetAll(0, 100, opts)
		require.NoError(t, err, "error getting shared channels")

		require.Equal(t, int(count), len(remotes))
		require.Len(t, remotes, 4, "should be 4 matching channels")
		for _, sc := range remotes {
			require.Equal(t, team1, sc.TeamId)
		}
	})

	t.Run("Get shared channels for non-member", func(t *testing.T) {
		opts := model.SharedChannelFilterOpts{
			TeamId:   team1,
			MemberId: model.NewId(),
		}

		count, err := ss.SharedChannel().GetAllCount(opts)
		require.NoError(t, err, "error getting shared channels count")

		remotes, err := ss.SharedChannel().GetAll(0, 100, opts)
		require.NoError(t, err, "error getting shared channels")

		require.Equal(t, int(count), len(remotes))
		require.Len(t, remotes, 0, "should be 0 matching channels")
	})
}

func testUpdateSharedChannel(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_update")
	require.NoError(t, err)

	sc := &model.SharedChannel{
		ChannelId: channel.Id,
		TeamId:    channel.TeamId,
		CreatorId: model.NewId(),
		ShareName: "testshare",
		Home:      true,
	}

	scSaved, err := ss.SharedChannel().Save(sc)
	require.NoError(t, err, "couldn't save shared channel", err)

	t.Run("Update existing shared channel", func(t *testing.T) {
		id := model.NewId()
		scMod := scSaved // copy struct (contains basic types only)
		scMod.ShareName = "newname"
		scMod.ShareDisplayName = "For testing"
		scMod.ShareHeader = "This is a header."
		scMod.RemoteId = id

		scUpdated, err := ss.SharedChannel().Update(scMod)
		require.NoError(t, err, "couldn't update shared channel", err)

		require.Equal(t, "newname", scUpdated.ShareName)
		require.Equal(t, "For testing", scUpdated.ShareDisplayName)
		require.Equal(t, "This is a header.", scUpdated.ShareHeader)
		require.Equal(t, id, scUpdated.RemoteId)
	})

	t.Run("Update non-existent shared channel", func(t *testing.T) {
		sc := &model.SharedChannel{
			ChannelId: model.NewId(),
			TeamId:    model.NewId(),
			CreatorId: model.NewId(),
			ShareName: "missingshare",
		}
		_, err := ss.SharedChannel().Update(sc)
		require.Error(t, err, "should error when updating non-existent shared channel", err)
	})
}

func testDeleteSharedChannel(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_delete")
	require.NoError(t, err)

	sc := &model.SharedChannel{
		ChannelId: channel.Id,
		TeamId:    channel.TeamId,
		CreatorId: model.NewId(),
		ShareName: "testshare",
		RemoteId:  model.NewId(),
	}

	_, err = ss.SharedChannel().Save(sc)
	require.NoError(t, err, "couldn't save shared channel", err)

	// add some remotes
	for range 10 {
		remote := &model.SharedChannelRemote{
			ChannelId:         channel.Id,
			CreatorId:         model.NewId(),
			RemoteId:          model.NewId(),
			IsInviteConfirmed: true, // to avoid adding the IncludeUnconfirmed filter
		}
		_, err := ss.SharedChannel().SaveRemote(remote)
		require.NoError(t, err, "couldn't add remote", err)
	}

	t.Run("Delete existing shared channel", func(t *testing.T) {
		deleted, err := ss.SharedChannel().Delete(channel.Id)
		require.NoError(t, err, "delete existing shared channel should not error", err)
		require.True(t, deleted, "expected true from delete shared channel")

		sc, err := ss.SharedChannel().Get(channel.Id)
		require.Error(t, err)
		require.Nil(t, sc)

		// make sure the remotes were marked as deleted.
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, model.SharedChannelRemoteFilterOpts{ChannelId: channel.Id})
		require.NoError(t, err)
		require.Len(t, remotes, 0)

		deletedRemotes, err := ss.SharedChannel().GetRemotes(0, 999999, model.SharedChannelRemoteFilterOpts{ChannelId: channel.Id, IncludeDeleted: true})
		require.NoError(t, err)
		require.Len(t, deletedRemotes, 10)

		// ensure channel's Shared flag is unset
		channelMod, err := ss.Channel().Get(channel.Id, false)
		require.NoError(t, err)
		require.False(t, channelMod.IsShared())
	})

	t.Run("Delete non-existent shared channel", func(t *testing.T) {
		deleted, err := ss.SharedChannel().Delete(model.NewId())
		require.NoError(t, err, "delete non-existent shared channel should not error", err)
		require.False(t, deleted, "expected false from delete shared channel")
	})
}

func testSaveSharedChannelRemote(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("Save shared channel remote", func(t *testing.T) {
		channel, err := createTestChannel(ss, rctx, "test_save_remote")
		require.NoError(t, err)

		remote := &model.SharedChannelRemote{
			ChannelId: channel.Id,
			CreatorId: model.NewId(),
			RemoteId:  model.NewId(),
		}

		remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
		require.NoError(t, err, "couldn't save shared channel remote", err)

		require.Equal(t, remote.ChannelId, remoteSaved.ChannelId)
		require.Equal(t, remote.CreatorId, remoteSaved.CreatorId)
	})

	t.Run("Save invalid shared channel remote", func(t *testing.T) {
		remote := &model.SharedChannelRemote{
			ChannelId: "",
			CreatorId: model.NewId(),
			RemoteId:  model.NewId(),
		}

		_, err := ss.SharedChannel().SaveRemote(remote)
		require.Error(t, err, "should error saving invalid remote", err)
	})

	t.Run("Save shared channel remote with invalid channel id", func(t *testing.T) {
		remote := &model.SharedChannelRemote{
			ChannelId: model.NewId(),
			CreatorId: model.NewId(),
			RemoteId:  model.NewId(),
		}

		_, err := ss.SharedChannel().SaveRemote(remote)
		require.Error(t, err, "expected error for invalid channel id")
	})
}

func testUpdateSharedChannelRemote(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("Update shared channel remote", func(t *testing.T) {
		channel, err := createTestChannel(ss, rctx, "test_update_remote")
		require.NoError(t, err)

		remote := &model.SharedChannelRemote{
			ChannelId: channel.Id,
			CreatorId: model.NewId(),
			RemoteId:  model.NewId(),
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
			ChannelId: "",
			CreatorId: model.NewId(),
			RemoteId:  model.NewId(),
		}

		_, err := ss.SharedChannel().UpdateRemote(remote)
		require.Error(t, err, "should error updating invalid remote", err)
	})

	t.Run("Update shared channel remote with invalid channel id", func(t *testing.T) {
		remote := &model.SharedChannelRemote{
			ChannelId: model.NewId(),
			CreatorId: model.NewId(),
			RemoteId:  model.NewId(),
		}

		_, err := ss.SharedChannel().UpdateRemote(remote)
		require.Error(t, err, "expected error for invalid channel id")
	})
}

func testGetSharedChannelRemote(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_remote_get")
	require.NoError(t, err)

	remote := &model.SharedChannelRemote{
		ChannelId: channel.Id,
		CreatorId: model.NewId(),
		RemoteId:  model.NewId(),
	}

	remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
	require.NoError(t, err, "couldn't save remote", err)

	t.Run("Get existing shared channel remote", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemote(remoteSaved.Id)
		require.NoError(t, err, "could not get shared channel remote", err)

		require.Equal(t, remoteSaved.Id, r.Id)
		require.Equal(t, remoteSaved.ChannelId, r.ChannelId)
		require.Equal(t, remoteSaved.CreatorId, r.CreatorId)
		require.Equal(t, remoteSaved.RemoteId, r.RemoteId)
	})

	t.Run("Get non-existent shared channel remote", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemote(model.NewId())
		require.Error(t, err)
		require.Nil(t, r)
	})
}

func testGetSharedChannelRemoteByIds(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_remote_get_by_ids")
	require.NoError(t, err)

	remote := &model.SharedChannelRemote{
		ChannelId: channel.Id,
		CreatorId: model.NewId(),
		RemoteId:  model.NewId(),
	}

	remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
	require.NoError(t, err, "could not save remote", err)

	t.Run("Get existing shared channel remote by ids", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemoteByIds(remoteSaved.ChannelId, remoteSaved.RemoteId)
		require.NoError(t, err, "couldn't get shared channel remote by ids", err)

		require.Equal(t, remoteSaved.Id, r.Id)
		require.Equal(t, remoteSaved.ChannelId, r.ChannelId)
		require.Equal(t, remoteSaved.CreatorId, r.CreatorId)
		require.Equal(t, remoteSaved.RemoteId, r.RemoteId)
	})

	t.Run("Get non-existent shared channel remote by ids", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemoteByIds(model.NewId(), model.NewId())
		require.Error(t, err)
		require.Nil(t, r)
	})

	t.Run("Get existing shared channel remote by ids from master", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemoteByIdsFromMaster(remoteSaved.ChannelId, remoteSaved.RemoteId)
		require.NoError(t, err)
		require.Equal(t, remoteSaved.Id, r.Id)
		require.Equal(t, remoteSaved.ChannelId, r.ChannelId)
		require.Equal(t, remoteSaved.RemoteId, r.RemoteId)
	})

	t.Run("Get non-existent shared channel remote by ids from master", func(t *testing.T) {
		r, err := ss.SharedChannel().GetRemoteByIdsFromMaster(model.NewId(), model.NewId())
		var nfErr *store.ErrNotFound
		require.ErrorAs(t, err, &nfErr)
		require.Nil(t, r)
	})

	t.Run("Get shared channel remote by ids from master for permanently deleted channel", func(t *testing.T) {
		deletedChannel, err := createTestChannel(ss, rctx, "test_remote_get_by_ids_deleted")
		require.NoError(t, err)
		orphan, err := ss.SharedChannel().SaveRemote(&model.SharedChannelRemote{
			ChannelId: deletedChannel.Id,
			CreatorId: model.NewId(),
			RemoteId:  model.NewId(),
		})
		require.NoError(t, err)
		require.NoError(t, ss.Channel().PermanentDelete(rctx, deletedChannel.Id))

		r, err := ss.SharedChannel().GetRemoteByIds(orphan.ChannelId, orphan.RemoteId)
		require.NoError(t, err)
		require.Equal(t, orphan.Id, r.Id)

		r, err = ss.SharedChannel().GetRemoteByIdsFromMaster(orphan.ChannelId, orphan.RemoteId)
		var nfErr *store.ErrNotFound
		require.ErrorAs(t, err, &nfErr)
		require.Nil(t, r)
	})

	t.Run("Get shared channel remote by ids from master for non-message channel type", func(t *testing.T) {
		spaceChannel, err := createTestChannel(ss, rctx, "test_remote_get_by_ids_space")
		require.NoError(t, err)
		spaceRemote, err := ss.SharedChannel().SaveRemote(&model.SharedChannelRemote{
			ChannelId: spaceChannel.Id,
			CreatorId: model.NewId(),
			RemoteId:  model.NewId(),
		})
		require.NoError(t, err)
		spaceChannel.Type = model.ChannelTypeSpace
		_, err = ss.Channel().Update(rctx, spaceChannel)
		require.NoError(t, err)

		r, err := ss.SharedChannel().GetRemoteByIds(spaceRemote.ChannelId, spaceRemote.RemoteId)
		require.NoError(t, err)
		require.Equal(t, spaceRemote.Id, r.Id)

		r, err = ss.SharedChannel().GetRemoteByIdsFromMaster(spaceRemote.ChannelId, spaceRemote.RemoteId)
		var nfErr *store.ErrNotFound
		require.ErrorAs(t, err, &nfErr)
		require.Nil(t, r)
	})

	t.Run("Get deleted shared channel remote by ids", func(t *testing.T) {
		deleted, err := ss.SharedChannel().DeleteRemote(remoteSaved.Id)
		require.NoError(t, err)
		require.True(t, deleted)

		r, err := ss.SharedChannel().GetRemoteByIds(remoteSaved.ChannelId, remoteSaved.RemoteId)
		require.NoError(t, err)
		require.Equal(t, remoteSaved.Id, r.Id)
		require.NotZero(t, r.DeleteAt)
	})
}

func testGetSharedChannelRemotes(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_remotes_get2")
	require.NoError(t, err)

	creator := model.NewId()
	remoteId := model.NewId()
	remoteId2 := model.NewId()

	data := []model.SharedChannelRemote{
		{ChannelId: channel.Id, CreatorId: creator, RemoteId: model.NewId(), IsInviteConfirmed: true},
		{ChannelId: channel.Id, CreatorId: creator, RemoteId: model.NewId(), IsInviteConfirmed: true},
		{ChannelId: channel.Id, CreatorId: creator, RemoteId: remoteId2, IsInviteConfirmed: true},
		{CreatorId: creator, RemoteId: remoteId, IsInviteConfirmed: true},
		{CreatorId: creator, RemoteId: remoteId, IsInviteConfirmed: true},
		{CreatorId: creator, RemoteId: remoteId, IsInviteConfirmed: true, DeleteAt: 123},
		{CreatorId: creator, RemoteId: remoteId},
	}

	// first three remotes are homed locally
	_, scErr := shareChannel(ss, channel, true, "")
	require.NoError(t, scErr)

	for i, r := range data {
		if r.ChannelId == "" {
			c, err := createTestChannel(ss, rctx, "test_remotes_get2_"+strconv.Itoa(i))
			require.NoError(t, err)
			r.ChannelId = c.Id

			// next three remotes are homed outside
			shareChannel(ss, c, false, r.RemoteId)
		}
		_, err := ss.SharedChannel().SaveRemote(&r)
		require.NoError(t, err, "error saving shared channel remote")
	}

	t.Run("Get shared channel remotes by channel_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ChannelId: channel.Id,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 3)
		for _, r := range remotes {
			require.Equal(t, channel.Id, r.ChannelId)
		}
	})

	t.Run("Get shared channel remotes by invalid channel_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ChannelId: model.NewId(),
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 0)
	})

	t.Run("Get shared channel remotes by remote_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			RemoteId: remoteId,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 2) // only confirmed invitations
		for _, r := range remotes {
			require.Equal(t, remoteId, r.RemoteId)
			require.True(t, r.IsInviteConfirmed)
		}
	})

	t.Run("Get shared channel remotes by remote_id including deleted", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			RemoteId:       remoteId,
			IncludeDeleted: true,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 3) // only confirmed invitations
		deletedCount := 0
		for _, r := range remotes {
			require.Equal(t, remoteId, r.RemoteId)
			require.True(t, r.IsInviteConfirmed)
			if r.DeleteAt != 0 {
				deletedCount++
			}
		}
		require.Equal(t, 1, deletedCount)
	})

	t.Run("Get shared channel remotes by invalid remote_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			RemoteId: model.NewId(),
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 0)
	})

	t.Run("Get shared channel remotes by remote_id including unconfirmed", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			RemoteId:           remoteId,
			IncludeUnconfirmed: true,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 3)
		for _, r := range remotes {
			require.Equal(t, remoteId, r.RemoteId)
		}
	})

	t.Run("Get only unconfirmed shared channel remotes for remote", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			RemoteId:         remoteId,
			ExcludeConfirmed: true,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 1)
		for _, r := range remotes {
			require.False(t, r.IsInviteConfirmed)
		}
	})

	t.Run("Get shared channel remotes with bad options", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ExcludeHome:   true,
			ExcludeRemote: true,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.Error(t, err, "error expected")
		require.Empty(t, remotes)
	})

	t.Run("Get shared channel remotes excluding shared from outside", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ExcludeRemote: true,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 3)
	})

	t.Run("Get shared channel remotes excluding shared from home", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ExcludeHome: true,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 2)
	})

	t.Run("Get shared channel remotes excluding shared from outside and by remote_id", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ExcludeRemote: true,
			RemoteId:      remoteId2,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 1)
	})

	t.Run("Get shared channel remotes excluding shared from home including unconfirmed", func(t *testing.T) {
		opts := model.SharedChannelRemoteFilterOpts{
			ExcludeHome:        true,
			IncludeUnconfirmed: true,
		}
		remotes, err := ss.SharedChannel().GetRemotes(0, 999999, opts)
		require.NoError(t, err, "should not error", err)
		require.Len(t, remotes, 3)
	})
}

func testGetRemotesStatus(t *testing.T, rctx request.CTX, ss store.Store) {
	// Create a channel and share it
	channel, err := createTestChannel(ss, rctx, "test_remotes_status")
	require.NoError(t, err)

	_, scErr := shareChannel(ss, channel, true, "")
	require.NoError(t, scErr)

	// Create remote clusters
	rc1 := &model.RemoteCluster{
		RemoteId:    model.NewId(),
		Name:        "remote_status_1",
		DisplayName: "Remote Status 1",
		SiteURL:     "http://example1.com",
		CreatorId:   model.NewId(),
	}
	rc1, err = ss.RemoteCluster().Save(rc1)
	require.NoError(t, err)

	rc2 := &model.RemoteCluster{
		RemoteId:    model.NewId(),
		Name:        "remote_status_2",
		DisplayName: "Remote Status 2",
		SiteURL:     "http://example2.com",
		CreatorId:   model.NewId(),
	}
	rc2, err = ss.RemoteCluster().Save(rc2)
	require.NoError(t, err)

	// Add remotes to the shared channel
	scr1 := &model.SharedChannelRemote{
		Id:                model.NewId(),
		ChannelId:         channel.Id,
		RemoteId:          rc1.RemoteId,
		CreatorId:         rc1.CreatorId,
		IsInviteAccepted:  true,
		IsInviteConfirmed: true,
	}
	_, err = ss.SharedChannel().SaveRemote(scr1)
	require.NoError(t, err)

	scr2 := &model.SharedChannelRemote{
		Id:                model.NewId(),
		ChannelId:         channel.Id,
		RemoteId:          rc2.RemoteId,
		CreatorId:         rc2.CreatorId,
		IsInviteAccepted:  false,
		IsInviteConfirmed: true,
	}
	_, err = ss.SharedChannel().SaveRemote(scr2)
	require.NoError(t, err)

	// Advance the sync cursors so the reported LastSyncAt reflects real activity.
	// scr1's members sync is newer than its post sync, so LastSyncAt must track the
	// members cursor. scr2's post-create cursor is the newest of all three cursors,
	// so LastSyncAt must track it. The post-create cursor advances independently of
	// the post-update cursor (see GetPostsSinceForSync), so it can be the most recent
	// sync activity and must be one of the GREATEST operands.
	now := model.GetMillis()
	scr1PostSyncAt := now - 5000
	scr1MembersSyncAt := now - 1000
	scr2PostCreateAt := now - 500
	scr2PostUpdateAt := now - 3000
	scr2MembersSyncAt := now - 7000

	err = ss.SharedChannel().UpdateRemoteCursor(scr1.Id, model.GetPostsSinceForSyncCursor{
		LastPostUpdateAt: scr1PostSyncAt,
		LastPostUpdateID: model.NewId(),
	})
	require.NoError(t, err)
	err = ss.SharedChannel().UpdateRemoteMembershipCursor(scr1.Id, scr1MembersSyncAt)
	require.NoError(t, err)

	err = ss.SharedChannel().UpdateRemoteCursor(scr2.Id, model.GetPostsSinceForSyncCursor{
		LastPostCreateAt: scr2PostCreateAt,
		LastPostCreateID: model.NewId(),
		LastPostUpdateAt: scr2PostUpdateAt,
		LastPostUpdateID: model.NewId(),
	})
	require.NoError(t, err)
	err = ss.SharedChannel().UpdateRemoteMembershipCursor(scr2.Id, scr2MembersSyncAt)
	require.NoError(t, err)

	t.Run("Get remotes status for channel", func(t *testing.T) {
		statuses, err := ss.SharedChannel().GetRemotesStatus(channel.Id)
		require.NoError(t, err)
		require.Len(t, statuses, 2)

		// Build a map by RemoteId for easier assertion
		statusMap := make(map[string]*model.SharedChannelRemoteStatus)
		for _, s := range statuses {
			statusMap[s.RemoteId] = s
		}

		s1 := statusMap[rc1.RemoteId]
		require.NotNil(t, s1)
		assert.Equal(t, channel.Id, s1.ChannelId)
		assert.Equal(t, rc1.RemoteId, s1.RemoteId)
		assert.Equal(t, rc1.DisplayName, s1.DisplayName)
		assert.Equal(t, rc1.SiteURL, s1.SiteURL)
		assert.True(t, s1.IsInviteAccepted)
		// LastSyncAt is the greater of the post and members cursors.
		assert.Equal(t, scr1MembersSyncAt, s1.LastSyncAt)

		s2 := statusMap[rc2.RemoteId]
		require.NotNil(t, s2)
		assert.Equal(t, channel.Id, s2.ChannelId)
		assert.Equal(t, rc2.RemoteId, s2.RemoteId)
		assert.Equal(t, rc2.DisplayName, s2.DisplayName)
		assert.Equal(t, rc2.SiteURL, s2.SiteURL)
		assert.False(t, s2.IsInviteAccepted)
		// The post-create cursor is the newest cursor, so LastSyncAt tracks it. If the
		// post-create cursor were excluded from GREATEST, this would report the older
		// post-update cursor instead.
		assert.Equal(t, scr2PostCreateAt, s2.LastSyncAt)
	})

	t.Run("Reports zero last sync when the remote has never synced", func(t *testing.T) {
		unsyncedChannel, err := createTestChannel(ss, rctx, "test_remotes_status_unsynced")
		require.NoError(t, err)

		_, scErr := shareChannel(ss, unsyncedChannel, true, "")
		require.NoError(t, scErr)

		scrUnsynced := &model.SharedChannelRemote{
			Id:                model.NewId(),
			ChannelId:         unsyncedChannel.Id,
			RemoteId:          rc1.RemoteId,
			CreatorId:         rc1.CreatorId,
			IsInviteAccepted:  true,
			IsInviteConfirmed: true,
		}
		_, err = ss.SharedChannel().SaveRemote(scrUnsynced)
		require.NoError(t, err)

		statuses, err := ss.SharedChannel().GetRemotesStatus(unsyncedChannel.Id)
		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Zero(t, statuses[0].LastSyncAt)
	})

	t.Run("Get remotes status for channel with no remotes", func(t *testing.T) {
		emptyChannel, err := createTestChannel(ss, rctx, "test_remotes_status_empty")
		require.NoError(t, err)

		_, scErr := shareChannel(ss, emptyChannel, true, "")
		require.NoError(t, scErr)

		statuses, err := ss.SharedChannel().GetRemotesStatus(emptyChannel.Id)
		require.NoError(t, err)
		require.Empty(t, statuses)
	})

	t.Run("Get remotes status excludes deleted remotes", func(t *testing.T) {
		// Create another channel with a deleted remote
		channel2, err := createTestChannel(ss, rctx, "test_remotes_status_deleted")
		require.NoError(t, err)

		_, scErr := shareChannel(ss, channel2, true, "")
		require.NoError(t, scErr)

		scrDeleted := &model.SharedChannelRemote{
			Id:                model.NewId(),
			ChannelId:         channel2.Id,
			RemoteId:          rc1.RemoteId,
			CreatorId:         rc1.CreatorId,
			IsInviteAccepted:  true,
			IsInviteConfirmed: true,
			DeleteAt:          model.GetMillis(),
		}
		_, err = ss.SharedChannel().SaveRemote(scrDeleted)
		require.NoError(t, err)

		statuses, err := ss.SharedChannel().GetRemotesStatus(channel2.Id)
		require.NoError(t, err)
		require.Empty(t, statuses)
	})
}

func testHasRemote(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_remotes_get2")
	require.NoError(t, err)

	remote1 := model.NewId()
	remote2 := model.NewId()

	creator := model.NewId()
	data := []model.SharedChannelRemote{
		{ChannelId: channel.Id, CreatorId: creator, RemoteId: remote1},
		{ChannelId: channel.Id, CreatorId: creator, RemoteId: remote2},
	}

	for _, r := range data {
		_, err := ss.SharedChannel().SaveRemote(&r)
		require.NoError(t, err, "error saving shared channel remote")
	}

	t.Run("has remote", func(t *testing.T) {
		has, err := ss.SharedChannel().HasRemote(channel.Id, remote1)
		require.NoError(t, err)
		assert.True(t, has)

		has, err = ss.SharedChannel().HasRemote(channel.Id, remote2)
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("wrong channel id ", func(t *testing.T) {
		has, err := ss.SharedChannel().HasRemote(model.NewId(), remote1)
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("wrong remote id", func(t *testing.T) {
		has, err := ss.SharedChannel().HasRemote(channel.Id, model.NewId())
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("deleted remote", func(t *testing.T) {
		scr, err := ss.SharedChannel().GetRemoteByIds(channel.Id, remote1)
		require.NoError(t, err)
		require.NotEmpty(t, scr.Id)

		deleted, err := ss.SharedChannel().DeleteRemote(scr.Id)
		require.NoError(t, err)
		require.True(t, deleted)

		has, err := ss.SharedChannel().HasRemote(channel.Id, remote1)
		require.NoError(t, err)
		assert.False(t, has)
	})
}

func testGetRemoteForUser(t *testing.T, rctx request.CTX, ss store.Store) {
	// add remotes, and users to simulated shared channels.
	channel, err := createSharedTestChannel(ss, rctx, "share_test_channel", true, nil)
	require.NoError(t, err)
	remotes := []*model.RemoteCluster{
		{RemoteId: model.NewId(), SiteURL: model.NewId(), CreatorId: model.NewId(), Name: "Test_Remote_1"},
		{RemoteId: model.NewId(), SiteURL: model.NewId(), CreatorId: model.NewId(), Name: "Test_Remote_2"},
		{RemoteId: model.NewId(), SiteURL: model.NewId(), CreatorId: model.NewId(), Name: "Test_Remote_3"},
	}
	for _, rc := range remotes {
		_, err := ss.RemoteCluster().Save(rc)
		require.NoError(t, err)

		scr := &model.SharedChannelRemote{Id: model.NewId(), CreatorId: rc.CreatorId, ChannelId: channel.Id, RemoteId: rc.RemoteId}
		_, err = ss.SharedChannel().SaveRemote(scr)
		require.NoError(t, err)
	}
	users := []string{model.NewId(), model.NewId(), model.NewId()}
	for _, id := range users {
		member := &model.ChannelMember{
			ChannelId:   channel.Id,
			UserId:      id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
			SchemeGuest: false,
			SchemeUser:  true,
		}
		_, err := ss.Channel().SaveMember(rctx, member)
		require.NoError(t, err)
	}

	t.Run("user is member", func(t *testing.T) {
		for _, rc := range remotes {
			for _, userId := range users {
				rcFound, err := ss.SharedChannel().GetRemoteForUser(rc.RemoteId, userId, false)
				assert.NoError(t, err, "remote should be found for user")
				assert.Equal(t, rc.RemoteId, rcFound.RemoteId, "remoteIds should match")
			}
		}
	})

	t.Run("user is not a member", func(t *testing.T) {
		for _, rc := range remotes {
			rcFound, err := ss.SharedChannel().GetRemoteForUser(rc.RemoteId, model.NewId(), false)
			assert.Error(t, err, "remote should not be found for user")
			assert.Nil(t, rcFound)
		}
	})

	t.Run("unknown remote id", func(t *testing.T) {
		rcFound, err := ss.SharedChannel().GetRemoteForUser(model.NewId(), users[0], false)
		assert.Error(t, err, "remote should not be found for unknown remote id")
		assert.Nil(t, rcFound)
	})

	t.Run("deleted remote with includeDeleted false", func(t *testing.T) {
		// Get the shared channel remote to delete
		scr, err := ss.SharedChannel().GetRemoteByIds(channel.Id, remotes[0].RemoteId)
		require.NoError(t, err)
		require.NotEmpty(t, scr.Id)

		// Delete the shared channel remote
		deleted, err := ss.SharedChannel().DeleteRemote(scr.Id)
		require.NoError(t, err)
		require.True(t, deleted)

		// Should not find the remote without includeDeleted
		rcFound, err := ss.SharedChannel().GetRemoteForUser(remotes[0].RemoteId, users[0], false)
		assert.Error(t, err, "deleted remote should not be found without includeDeleted")
		assert.Nil(t, rcFound)
	})

	t.Run("deleted remote with includeDeleted true", func(t *testing.T) {
		// Should find the remote with includeDeleted
		rcFound, err := ss.SharedChannel().GetRemoteForUser(remotes[0].RemoteId, users[0], true)
		assert.NoError(t, err, "deleted remote should be found with includeDeleted")
		assert.NotNil(t, rcFound)
		assert.Equal(t, remotes[0].RemoteId, rcFound.RemoteId, "remoteIds should match")
	})

	t.Run("deleted remote with includeDeleted true but user not a member", func(t *testing.T) {
		// A user who is not a member of the channel should not be able to see
		// the deleted remote, even with includeDeleted=true
		nonMemberUserId := model.NewId()
		rcFound, err := ss.SharedChannel().GetRemoteForUser(remotes[0].RemoteId, nonMemberUserId, true)
		assert.Error(t, err, "deleted remote should not be found for non-member user even with includeDeleted")
		assert.Nil(t, rcFound)
	})
}

func testUpdateSharedChannelRemoteCursor(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_remote_update_next_sync_at")
	require.NoError(t, err)

	remote := &model.SharedChannelRemote{
		ChannelId: channel.Id,
		CreatorId: model.NewId(),
		RemoteId:  model.NewId(),
	}

	remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
	require.NoError(t, err, "couldn't save remote", err)

	futureCreateAt := model.GetMillis() + 3600000 // 1 hour in the future
	postCreateID := model.NewId()

	futureUpdateAt := model.GetMillis() + (3600000 * 2) // 2 hours in the future
	postUpdateID := model.NewId()

	cursorCreate := model.GetPostsSinceForSyncCursor{
		LastPostCreateAt: futureCreateAt,
		LastPostCreateID: postCreateID,
	}

	cursorUpdate := model.GetPostsSinceForSyncCursor{
		LastPostUpdateAt: futureUpdateAt,
		LastPostUpdateID: postUpdateID,
	}

	t.Run("Update cursor CreateAt for remote", func(t *testing.T) {
		err := ss.SharedChannel().UpdateRemoteCursor(remoteSaved.Id, cursorCreate)
		require.NoError(t, err, "update cursor should not error", err)

		r, err := ss.SharedChannel().GetRemote(remoteSaved.Id)
		require.NoError(t, err)
		require.Equal(t, futureCreateAt, r.LastPostCreateAt)
		require.Equal(t, postCreateID, r.LastPostCreateID)
	})

	t.Run("Update cursor UpdateAt for remote", func(t *testing.T) {
		err := ss.SharedChannel().UpdateRemoteCursor(remoteSaved.Id, cursorUpdate)
		require.NoError(t, err, "update cursor should not error", err)

		r, err := ss.SharedChannel().GetRemote(remoteSaved.Id)
		require.NoError(t, err)
		require.Equal(t, futureUpdateAt, r.LastPostUpdateAt)
		require.Equal(t, postUpdateID, r.LastPostUpdateID)
	})

	t.Run("Update cursor for non-existent shared channel remote", func(t *testing.T) {
		err := ss.SharedChannel().UpdateRemoteCursor(model.NewId(), cursorUpdate)
		require.Error(t, err, "update non-existent remote should error", err)
	})

	t.Run("Update with empty cursor", func(t *testing.T) {
		emptyCursor := model.GetPostsSinceForSyncCursor{}
		err := ss.SharedChannel().UpdateRemoteCursor(remoteSaved.Id, emptyCursor)
		require.Error(t, err, "update with empty cursor should error", err)
	})
}

func testUpdateGlobalUserSyncCursor(t *testing.T, rctx request.CTX, ss store.Store) {
	// Create a remote cluster first
	rc := &model.RemoteCluster{
		RemoteId:  model.NewId(),
		SiteURL:   "http://example.com",
		CreatorId: model.NewId(),
		Name:      "test",
	}
	rcSaved, err := ss.RemoteCluster().Save(rc)
	require.NoError(t, err, "couldn't save remote cluster", err)

	futureTimestamp := model.GetMillis() + 3600000 // 1 hour in the future

	t.Run("Update global user sync cursor for remote", func(t *testing.T) {
		err := ss.RemoteCluster().UpdateLastGlobalUserSyncAt(rcSaved.RemoteId, futureTimestamp)
		require.NoError(t, err, "update global user sync cursor should not error", err)

		// Verify that the LastGlobalUserSyncAt field was updated in the RemoteCluster table
		// Small sleep to ensure the transaction is committed
		time.Sleep(10 * time.Millisecond)
		updatedRC, err := ss.RemoteCluster().Get(rcSaved.RemoteId, false)
		require.NoError(t, err)
		require.NotZero(t, updatedRC.LastGlobalUserSyncAt, "LastGlobalUserSyncAt should not be zero")
		require.Equal(t, futureTimestamp, updatedRC.LastGlobalUserSyncAt)
	})

	t.Run("Update global user sync cursor for non-existent remote", func(t *testing.T) {
		err := ss.RemoteCluster().UpdateLastGlobalUserSyncAt(model.NewId(), futureTimestamp)
		require.Error(t, err, "update non-existent remote should error", err)
	})
}

func testDeleteSharedChannelRemote(t *testing.T, rctx request.CTX, ss store.Store) {
	channel, err := createTestChannel(ss, rctx, "test_remote_delete")
	require.NoError(t, err)

	remote := &model.SharedChannelRemote{
		ChannelId: channel.Id,
		CreatorId: model.NewId(),
		RemoteId:  model.NewId(),
	}

	remoteSaved, err := ss.SharedChannel().SaveRemote(remote)
	require.NoError(t, err, "couldn't save remote", err)

	t.Run("Delete existing shared channel remote", func(t *testing.T) {
		deleted, err := ss.SharedChannel().DeleteRemote(remoteSaved.Id)
		require.NoError(t, err, "delete existing remote should not error", err)
		require.True(t, deleted, "expected true from delete remote")

		r, err := ss.SharedChannel().GetRemote(remoteSaved.Id)
		require.NoError(t, err)
		require.NotZero(t, r.DeleteAt)
	})

	t.Run("Delete non-existent shared channel remote", func(t *testing.T) {
		deleted, err := ss.SharedChannel().DeleteRemote(model.NewId())
		require.NoError(t, err, "delete non-existent remote should not error", err)
		require.False(t, deleted, "expected false from delete remote")
	})
}

func createTestUser(rctx request.CTX, ss store.Store, username string) (*model.User, error) {
	user := &model.User{
		Username: username,
		Email:    "gary@example.com",
	}
	return ss.User().Save(rctx, user)
}

func createTestChannel(ss store.Store, rctx request.CTX, name string) (*model.Channel, error) {
	channel, err := createSharedTestChannel(ss, rctx, name, false, nil)
	return channel, err
}

func createTestChannelWithUser(ss store.Store, rctx request.CTX, name string, member *model.User) (*model.Channel, error) {
	channel, err := createSharedTestChannel(ss, rctx, name, false, member)
	return channel, err
}

func createSharedTestChannel(ss store.Store, rctx request.CTX, name string, shared bool, member *model.User) (*model.Channel, error) {
	channel := &model.Channel{
		TeamId:      model.NewId(),
		Type:        model.ChannelTypeOpen,
		Name:        name,
		DisplayName: name + " display name",
		Header:      name + " header",
		Purpose:     name + "purpose",
		CreatorId:   model.NewId(),
		Shared:      new(shared),
	}
	channel, err := ss.Channel().Save(rctx, channel, 10000)
	if err != nil {
		return nil, err
	}

	if member != nil {
		newMember := &model.ChannelMember{
			ChannelId:   channel.Id,
			UserId:      member.Id,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
			SchemeGuest: member.IsGuest(),
			SchemeUser:  !member.IsGuest(),
		}

		_, err = ss.Channel().SaveMember(rctx, newMember)
		if err != nil {
			return nil, err
		}
	}

	if shared {
		if _, err := shareChannel(ss, channel, true, ""); err != nil {
			return nil, err
		}
	}
	return channel, nil
}

func shareChannel(ss store.Store, channel *model.Channel, home bool, remoteId string) (*model.SharedChannel, error) {
	sc := &model.SharedChannel{
		ChannelId: channel.Id,
		TeamId:    channel.TeamId,
		CreatorId: channel.CreatorId,
		ShareName: channel.Name,
		Home:      home,
		RemoteId:  remoteId,
	}

	return ss.SharedChannel().Save(sc)
}

func clearSharedChannels(ss store.Store) error {
	opts := model.SharedChannelFilterOpts{}
	all, err := ss.SharedChannel().GetAll(0, 1000, opts)
	if err != nil {
		return err
	}

	for _, sc := range all {
		if _, err := ss.SharedChannel().Delete(sc.ChannelId); err != nil {
			return err
		}
	}
	return nil
}

func testSaveSharedChannelUser(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("Save shared channel user", func(t *testing.T) {
		scUser := &model.SharedChannelUser{
			UserId:    model.NewId(),
			RemoteId:  model.NewId(),
			ChannelId: model.NewId(),
		}

		userSaved, err := ss.SharedChannel().SaveUser(scUser)
		require.NoError(t, err, "couldn't save shared channel user", err)

		require.Equal(t, scUser.UserId, userSaved.UserId)
		require.Equal(t, scUser.RemoteId, userSaved.RemoteId)
	})

	t.Run("Save invalid shared channel user", func(t *testing.T) {
		scUser := &model.SharedChannelUser{
			UserId:   "",
			RemoteId: model.NewId(),
		}

		_, err := ss.SharedChannel().SaveUser(scUser)
		require.Error(t, err, "should error saving invalid user", err)
	})

	t.Run("Save shared channel user with invalid remote id", func(t *testing.T) {
		scUser := &model.SharedChannelUser{
			UserId:   model.NewId(),
			RemoteId: "bogus",
		}

		_, err := ss.SharedChannel().SaveUser(scUser)
		require.Error(t, err, "expected error for invalid remote id")
	})
}

func testGetSingleSharedChannelUser(t *testing.T, rctx request.CTX, ss store.Store) {
	scUser := &model.SharedChannelUser{
		UserId:    model.NewId(),
		RemoteId:  model.NewId(),
		ChannelId: model.NewId(),
	}

	userSaved, err := ss.SharedChannel().SaveUser(scUser)
	require.NoError(t, err, "could not save user", err)

	t.Run("Get existing shared channel user", func(t *testing.T) {
		r, err := ss.SharedChannel().GetSingleUser(userSaved.UserId, userSaved.ChannelId, userSaved.RemoteId)
		require.NoError(t, err, "couldn't get shared channel user", err)

		require.Equal(t, userSaved.Id, r.Id)
		require.Equal(t, userSaved.UserId, r.UserId)
		require.Equal(t, userSaved.RemoteId, r.RemoteId)
		require.Equal(t, userSaved.CreateAt, r.CreateAt)
	})

	t.Run("Get non-existent shared channel user", func(t *testing.T) {
		u, err := ss.SharedChannel().GetSingleUser(model.NewId(), model.NewId(), model.NewId())
		require.Error(t, err)
		require.Nil(t, u)
	})
}

func testGetSharedChannelUser(t *testing.T, rctx request.CTX, ss store.Store) {
	// GetUsersForUser only returns rows whose remote cluster still exists and
	// is not deleted, so point the rows at a live remote.
	liveRemote := &model.RemoteCluster{
		RemoteId:  model.NewId(),
		SiteURL:   "http://example.com",
		CreatorId: model.NewId(),
		Name:      "live-remote",
	}
	_, err := ss.RemoteCluster().Save(liveRemote)
	require.NoError(t, err, "couldn't save remote cluster", err)

	userId := model.NewId()
	for range 10 {
		scUser := &model.SharedChannelUser{
			UserId:    userId,
			RemoteId:  liveRemote.RemoteId,
			ChannelId: model.NewId(),
		}
		_, err := ss.SharedChannel().SaveUser(scUser)
		require.NoError(t, err, "could not save user", err)
	}

	t.Run("Get existing shared channel user", func(t *testing.T) {
		scus, err := ss.SharedChannel().GetUsersForUser(userId)
		require.NoError(t, err, "couldn't get shared channel user", err)

		require.Len(t, scus, 10, "should be 10 shared channel user records")
		require.Equal(t, userId, scus[0].UserId)
	})

	t.Run("Get non-existent shared channel user", func(t *testing.T) {
		scus, err := ss.SharedChannel().GetUsersForUser(model.NewId())
		require.NoError(t, err, "should not error when not found")
		require.Empty(t, scus, "should be empty")
	})

	t.Run("Excludes rows for deleted or missing remote clusters", func(t *testing.T) {
		deletedRemote := &model.RemoteCluster{
			RemoteId:  model.NewId(),
			SiteURL:   "http://example.com",
			CreatorId: model.NewId(),
			Name:      "deleted-remote",
		}
		_, err := ss.RemoteCluster().Save(deletedRemote)
		require.NoError(t, err)

		mixedUserId := model.NewId()
		// Two rows for the live remote: these should be returned.
		for range 2 {
			_, err = ss.SharedChannel().SaveUser(&model.SharedChannelUser{
				UserId:    mixedUserId,
				RemoteId:  liveRemote.RemoteId,
				ChannelId: model.NewId(),
			})
			require.NoError(t, err)
		}
		// One row for a soft-deleted remote: should be excluded.
		_, err = ss.SharedChannel().SaveUser(&model.SharedChannelUser{
			UserId:    mixedUserId,
			RemoteId:  deletedRemote.RemoteId,
			ChannelId: model.NewId(),
		})
		require.NoError(t, err)
		// One row for a remote that never existed: should be excluded.
		_, err = ss.SharedChannel().SaveUser(&model.SharedChannelUser{
			UserId:    mixedUserId,
			RemoteId:  model.NewId(),
			ChannelId: model.NewId(),
		})
		require.NoError(t, err)

		deleted, err := ss.RemoteCluster().Delete(deletedRemote.RemoteId)
		require.NoError(t, err)
		require.True(t, deleted)

		scus, err := ss.SharedChannel().GetUsersForUser(mixedUserId)
		require.NoError(t, err)
		require.Len(t, scus, 2, "should only return rows pointing at the live remote")
		for _, scu := range scus {
			require.Equal(t, liveRemote.RemoteId, scu.RemoteId)
		}
	})
}

func testGetSharedChannelUsersForSync(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID := model.NewId()
	remoteID := model.NewId()
	earlier := model.GetMillis() - 300000
	later := model.GetMillis() + 300000

	var users []*model.User
	for range 10 { // need real users
		u := &model.User{
			Username:          model.NewUsername(),
			Email:             model.NewId() + "@example.com",
			LastPictureUpdate: model.GetMillis(),
		}
		u, err := ss.User().Save(rctx, u)
		require.NoError(t, err)
		users = append(users, u)
	}

	data := []model.SharedChannelUser{
		{UserId: users[0].Id, ChannelId: model.NewId(), RemoteId: model.NewId(), LastSyncAt: later},
		{UserId: users[1].Id, ChannelId: model.NewId(), RemoteId: model.NewId(), LastSyncAt: earlier},
		{UserId: users[1].Id, ChannelId: model.NewId(), RemoteId: model.NewId(), LastSyncAt: earlier},
		{UserId: users[1].Id, ChannelId: channelID, RemoteId: remoteID, LastSyncAt: later},
		{UserId: users[2].Id, ChannelId: channelID, RemoteId: model.NewId(), LastSyncAt: later},
		{UserId: users[3].Id, ChannelId: channelID, RemoteId: model.NewId(), LastSyncAt: earlier},
		{UserId: users[4].Id, ChannelId: channelID, RemoteId: model.NewId(), LastSyncAt: later},
		{UserId: users[5].Id, ChannelId: channelID, RemoteId: remoteID, LastSyncAt: earlier},
		{UserId: users[6].Id, ChannelId: channelID, RemoteId: remoteID, LastSyncAt: later},
	}

	for i, u := range data {
		scu := &model.SharedChannelUser{
			UserId:     u.UserId,
			ChannelId:  u.ChannelId,
			RemoteId:   u.RemoteId,
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
			require.Contains(t, []string{users[3].Id, users[5].Id}, user.Id)
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
			require.Contains(t, []string{users[3].Id, users[5].Id}, user.Id)
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

func testUpdateSharedChannelUserLastSyncAt(t *testing.T, rctx request.CTX, ss store.Store) {
	u1 := &model.User{
		Username:          model.NewUsername(),
		Email:             model.NewId() + "@example.com",
		LastPictureUpdate: model.GetMillis() - 300000, // 5 mins
	}
	u1, err := ss.User().Save(rctx, u1)
	require.NoError(t, err)

	u2 := &model.User{
		Username:          model.NewUsername(),
		Email:             model.NewId() + "@example.com",
		LastPictureUpdate: model.GetMillis() + 300000,
	}
	u2, err = ss.User().Save(rctx, u2)
	require.NoError(t, err)

	channelID := model.NewId()
	remoteID := model.NewId()

	scUser1 := &model.SharedChannelUser{
		UserId:    u1.Id,
		RemoteId:  remoteID,
		ChannelId: channelID,
	}
	_, err = ss.SharedChannel().SaveUser(scUser1)
	require.NoError(t, err, "couldn't save user", err)

	scUser2 := &model.SharedChannelUser{
		UserId:    u2.Id,
		RemoteId:  remoteID,
		ChannelId: channelID,
	}
	_, err = ss.SharedChannel().SaveUser(scUser2)
	require.NoError(t, err, "couldn't save user", err)

	t.Run("Update LastSyncAt for user via UpdateAt", func(t *testing.T) {
		err := ss.SharedChannel().UpdateUserLastSyncAt(u1.Id, channelID, remoteID)
		require.NoError(t, err, "updateLastSyncAt should not error", err)

		scu, err := ss.SharedChannel().GetSingleUser(u1.Id, channelID, remoteID)
		require.NoError(t, err)
		require.Equal(t, u1.UpdateAt, scu.LastSyncAt)
	})

	t.Run("Update LastSyncAt for user via LastPictureUpdate", func(t *testing.T) {
		err := ss.SharedChannel().UpdateUserLastSyncAt(u2.Id, channelID, remoteID)
		require.NoError(t, err, "updateLastSyncAt should not error", err)

		scu, err := ss.SharedChannel().GetSingleUser(u2.Id, channelID, remoteID)
		require.NoError(t, err)
		require.Equal(t, u2.LastPictureUpdate, scu.LastSyncAt)
	})

	t.Run("Update LastSyncAt for non-existent shared channel user", func(t *testing.T) {
		err := ss.SharedChannel().UpdateUserLastSyncAt(model.NewId(), channelID, remoteID)
		require.Error(t, err, "update non-existent user should error", err)
	})
}

func testSaveSharedChannelAttachment(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("Save shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileId:   model.NewId(),
			RemoteId: model.NewId(),
		}

		saved, err := ss.SharedChannel().SaveAttachment(attachment)
		require.NoError(t, err, "couldn't save shared channel attachment", err)

		require.Equal(t, attachment.FileId, saved.FileId)
		require.Equal(t, attachment.RemoteId, saved.RemoteId)
	})

	t.Run("Save invalid shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileId:   "",
			RemoteId: model.NewId(),
		}

		_, err := ss.SharedChannel().SaveAttachment(attachment)
		require.Error(t, err, "should error saving invalid attachment", err)
	})

	t.Run("Save shared channel attachment with invalid remote id", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileId:   model.NewId(),
			RemoteId: "bogus",
		}

		_, err := ss.SharedChannel().SaveAttachment(attachment)
		require.Error(t, err, "expected error for invalid remote id")
	})
}

func testUpsertSharedChannelAttachment(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("Upsert new shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileId:   model.NewId(),
			RemoteId: model.NewId(),
		}

		_, err := ss.SharedChannel().UpsertAttachment(attachment)
		require.NoError(t, err, "couldn't upsert shared channel attachment", err)

		saved, err := ss.SharedChannel().GetAttachment(attachment.FileId, attachment.RemoteId)
		require.NoError(t, err, "couldn't get shared channel attachment", err)

		require.NotZero(t, saved.CreateAt)
		require.Equal(t, saved.CreateAt, saved.LastSyncAt)
	})

	t.Run("Upsert existing shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileId:   model.NewId(),
			RemoteId: model.NewId(),
		}

		saved, err := ss.SharedChannel().SaveAttachment(attachment)
		require.NoError(t, err, "couldn't save shared channel attachment", err)

		// make sure enough time passed that GetMillis returns a different value
		time.Sleep(2 * time.Millisecond)

		_, err = ss.SharedChannel().UpsertAttachment(saved)
		require.NoError(t, err, "couldn't upsert shared channel attachment", err)

		updated, err := ss.SharedChannel().GetAttachment(attachment.FileId, attachment.RemoteId)
		require.NoError(t, err, "couldn't get shared channel attachment", err)

		require.NotZero(t, updated.CreateAt)
		require.Greater(t, updated.LastSyncAt, updated.CreateAt)
	})

	t.Run("Upsert invalid shared channel attachment", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileId:   "",
			RemoteId: model.NewId(),
		}

		id, err := ss.SharedChannel().UpsertAttachment(attachment)
		require.Error(t, err, "should error upserting invalid attachment", err)
		require.Empty(t, id)
	})

	t.Run("Upsert shared channel attachment with invalid remote id", func(t *testing.T) {
		attachment := &model.SharedChannelAttachment{
			FileId:   model.NewId(),
			RemoteId: "bogus",
		}

		id, err := ss.SharedChannel().UpsertAttachment(attachment)
		require.Error(t, err, "expected error for invalid remote id")
		require.Empty(t, id)
	})
}

func testGetSharedChannelAttachment(t *testing.T, rctx request.CTX, ss store.Store) {
	attachment := &model.SharedChannelAttachment{
		FileId:   model.NewId(),
		RemoteId: model.NewId(),
	}

	saved, err := ss.SharedChannel().SaveAttachment(attachment)
	require.NoError(t, err, "could not save attachment", err)

	t.Run("Get existing shared channel attachment", func(t *testing.T) {
		r, err := ss.SharedChannel().GetAttachment(saved.FileId, saved.RemoteId)
		require.NoError(t, err, "couldn't get shared channel attachment", err)

		require.Equal(t, saved.Id, r.Id)
		require.Equal(t, saved.FileId, r.FileId)
		require.Equal(t, saved.RemoteId, r.RemoteId)
		require.Equal(t, saved.CreateAt, r.CreateAt)
	})

	t.Run("Get non-existent shared channel attachment", func(t *testing.T) {
		u, err := ss.SharedChannel().GetAttachment(model.NewId(), model.NewId())
		require.Error(t, err)
		require.Nil(t, u)
	})
}

func testUpdateSharedChannelAttachmentLastSyncAt(t *testing.T, rctx request.CTX, ss store.Store) {
	attachment := &model.SharedChannelAttachment{
		FileId:   model.NewId(),
		RemoteId: model.NewId(),
	}

	saved, err := ss.SharedChannel().SaveAttachment(attachment)
	require.NoError(t, err, "couldn't save attachment", err)

	future := model.GetMillis() + 3600000 // 1 hour in the future

	t.Run("Update LastSyncAt for attachment", func(t *testing.T) {
		err := ss.SharedChannel().UpdateAttachmentLastSyncAt(saved.Id, future)
		require.NoError(t, err, "updateLastSyncAt should not error", err)

		f, err := ss.SharedChannel().GetAttachment(saved.FileId, saved.RemoteId)
		require.NoError(t, err)
		require.Equal(t, future, f.LastSyncAt)
	})

	t.Run("Update LastSyncAt for non-existent shared channel attachment", func(t *testing.T) {
		err := ss.SharedChannel().UpdateAttachmentLastSyncAt(model.NewId(), future)
		require.Error(t, err, "update non-existent attachment should error", err)
	})
}
