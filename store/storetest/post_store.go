// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func TestPostStore(t *testing.T, ss store.Store, s SqlStore) {
	t.Run("SaveMultiple", func(t *testing.T) { testPostStoreSaveMultiple(t, ss) })
	t.Run("Save", func(t *testing.T) { testPostStoreSave(t, ss) })
	t.Run("SaveAndUpdateChannelMsgCounts", func(t *testing.T) { testPostStoreSaveChannelMsgCounts(t, ss) })
	t.Run("Get", func(t *testing.T) { testPostStoreGet(t, ss) })
	t.Run("GetSingle", func(t *testing.T) { testPostStoreGetSingle(t, ss) })
	t.Run("Update", func(t *testing.T) { testPostStoreUpdate(t, ss) })
	t.Run("Delete", func(t *testing.T) { testPostStoreDelete(t, ss) })
	t.Run("Delete1Level", func(t *testing.T) { testPostStoreDelete1Level(t, ss) })
	t.Run("Delete2Level", func(t *testing.T) { testPostStoreDelete2Level(t, ss) })
	t.Run("PermDelete1Level", func(t *testing.T) { testPostStorePermDelete1Level(t, ss) })
	t.Run("PermDelete1Level2", func(t *testing.T) { testPostStorePermDelete1Level2(t, ss) })
	t.Run("GetWithChildren", func(t *testing.T) { testPostStoreGetWithChildren(t, ss) })
	t.Run("GetPostsWithDetails", func(t *testing.T) { testPostStoreGetPostsWithDetails(t, ss) })
	t.Run("GetPostsBeforeAfter", func(t *testing.T) { testPostStoreGetPostsBeforeAfter(t, ss) })
	t.Run("GetPostsSince", func(t *testing.T) { testPostStoreGetPostsSince(t, ss) })
	t.Run("GetPosts", func(t *testing.T) { testPostStoreGetPosts(t, ss) })
	t.Run("GetPostBeforeAfter", func(t *testing.T) { testPostStoreGetPostBeforeAfter(t, ss) })
	t.Run("UserCountsWithPostsByDay", func(t *testing.T) { testUserCountsWithPostsByDay(t, ss) })
	t.Run("PostCountsByDay", func(t *testing.T) { testPostCountsByDay(t, ss) })
	t.Run("GetFlaggedPostsForTeam", func(t *testing.T) { testPostStoreGetFlaggedPostsForTeam(t, ss, s) })
	t.Run("GetFlaggedPosts", func(t *testing.T) { testPostStoreGetFlaggedPosts(t, ss) })
	t.Run("GetFlaggedPostsForChannel", func(t *testing.T) { testPostStoreGetFlaggedPostsForChannel(t, ss) })
	t.Run("GetPostsCreatedAt", func(t *testing.T) { testPostStoreGetPostsCreatedAt(t, ss) })
	t.Run("Overwrite", func(t *testing.T) { testPostStoreOverwrite(t, ss) })
	t.Run("OverwriteMultiple", func(t *testing.T) { testPostStoreOverwriteMultiple(t, ss) })
	t.Run("GetPostsByIds", func(t *testing.T) { testPostStoreGetPostsByIDs(t, ss) })
	t.Run("GetPostsBatchForIndexing", func(t *testing.T) { testPostStoreGetPostsBatchForIndexing(t, ss) })
	t.Run("PermanentDeleteBatch", func(t *testing.T) { testPostStorePermanentDeleteBatch(t, ss) })
	t.Run("GetOldest", func(t *testing.T) { testPostStoreGetOldest(t, ss) })
	t.Run("TestGetMaxPostSize", func(t *testing.T) { testGetMaxPostSize(t, ss) })
	t.Run("GetParentsForExportAfter", func(t *testing.T) { testPostStoreGetParentsForExportAfter(t, ss) })
	t.Run("GetRepliesForExport", func(t *testing.T) { testPostStoreGetRepliesForExport(t, ss) })
	t.Run("GetDirectPostParentsForExportAfter", func(t *testing.T) { testPostStoreGetDirectPostParentsForExportAfter(t, ss, s) })
	t.Run("GetDirectPostParentsForExportAfterDeleted", func(t *testing.T) { testPostStoreGetDirectPostParentsForExportAfterDeleted(t, ss, s) })
	t.Run("GetDirectPostParentsForExportAfterBatched", func(t *testing.T) { testPostStoreGetDirectPostParentsForExportAfterBatched(t, ss, s) })
	t.Run("GetForThread", func(t *testing.T) { testPostStoreGetForThread(t, ss) })
	t.Run("HasAutoResponsePostByUserSince", func(t *testing.T) { testHasAutoResponsePostByUserSince(t, ss) })
	t.Run("GetPostsSinceForSync", func(t *testing.T) { testGetPostsSinceForSync(t, ss, s) })
}

func testPostStoreSave(t *testing.T, ss store.Store) {
	t.Run("Save post", func(t *testing.T) {
		o1 := model.Post{}
		o1.ChannelID = model.NewID()
		o1.UserID = model.NewID()
		o1.Message = "zz" + model.NewID() + "b"

		p, err := ss.Post().Save(&o1)
		require.NoError(t, err, "couldn't save item")
		assert.Equal(t, int64(0), p.ReplyCount)
	})

	t.Run("Save replies", func(t *testing.T) {
		o1 := model.Post{}
		o1.ChannelID = model.NewID()
		o1.UserID = model.NewID()
		o1.RootID = model.NewID()
		o1.Message = "zz" + model.NewID() + "b"

		o2 := model.Post{}
		o2.ChannelID = model.NewID()
		o2.UserID = model.NewID()
		o2.RootID = o1.RootID
		o2.Message = "zz" + model.NewID() + "b"

		o3 := model.Post{}
		o3.ChannelID = model.NewID()
		o3.UserID = model.NewID()
		o3.RootID = model.NewID()
		o3.Message = "zz" + model.NewID() + "b"

		p1, err := ss.Post().Save(&o1)
		require.NoError(t, err, "couldn't save item")
		assert.Equal(t, int64(1), p1.ReplyCount)

		p2, err := ss.Post().Save(&o2)
		require.NoError(t, err, "couldn't save item")
		assert.Equal(t, int64(2), p2.ReplyCount)

		p3, err := ss.Post().Save(&o3)
		require.NoError(t, err, "couldn't save item")
		assert.Equal(t, int64(1), p3.ReplyCount)
	})

	t.Run("Try to save existing post", func(t *testing.T) {
		o1 := model.Post{}
		o1.ChannelID = model.NewID()
		o1.UserID = model.NewID()
		o1.Message = "zz" + model.NewID() + "b"

		_, err := ss.Post().Save(&o1)
		require.NoError(t, err, "couldn't save item")

		_, err = ss.Post().Save(&o1)
		require.Error(t, err, "shouldn't be able to update from save")
	})

	t.Run("Update reply should update the UpdateAt of the root post", func(t *testing.T) {
		rootPost := model.Post{}
		rootPost.ChannelID = model.NewID()
		rootPost.UserID = model.NewID()
		rootPost.Message = "zz" + model.NewID() + "b"

		_, err := ss.Post().Save(&rootPost)
		require.NoError(t, err)

		time.Sleep(2 * time.Millisecond)

		replyPost := model.Post{}
		replyPost.ChannelID = rootPost.ChannelID
		replyPost.UserID = model.NewID()
		replyPost.Message = "zz" + model.NewID() + "b"
		replyPost.RootID = rootPost.ID

		// We need to sleep here to be sure the post is not created during the same millisecond
		time.Sleep(time.Millisecond)
		_, err = ss.Post().Save(&replyPost)
		require.NoError(t, err)

		rrootPost, err := ss.Post().GetSingle(rootPost.ID, false)
		require.NoError(t, err)
		assert.Greater(t, rrootPost.UpdateAt, rootPost.UpdateAt)
	})

	t.Run("Create a post should update the channel LastPostAt and the total messages count by one", func(t *testing.T) {
		channel := model.Channel{}
		channel.Name = "zz" + model.NewID() + "b"
		channel.DisplayName = "zz" + model.NewID() + "b"
		channel.Type = model.ChannelTypeOpen

		_, err := ss.Channel().Save(&channel, 100)
		require.NoError(t, err)

		post := model.Post{}
		post.ChannelID = channel.ID
		post.UserID = model.NewID()
		post.Message = "zz" + model.NewID() + "b"

		// We need to sleep here to be sure the post is not created during the same millisecond
		time.Sleep(time.Millisecond)
		_, err = ss.Post().Save(&post)
		require.NoError(t, err)

		rchannel, err := ss.Channel().Get(channel.ID, false)
		require.NoError(t, err)
		assert.Greater(t, rchannel.LastPostAt, channel.LastPostAt)
		assert.Equal(t, int64(1), rchannel.TotalMsgCount)

		post = model.Post{}
		post.ChannelID = channel.ID
		post.UserID = model.NewID()
		post.Message = "zz" + model.NewID() + "b"
		post.CreateAt = 5

		// We need to sleep here to be sure the post is not created during the same millisecond
		time.Sleep(time.Millisecond)
		_, err = ss.Post().Save(&post)
		require.NoError(t, err)

		rchannel2, err := ss.Channel().Get(channel.ID, false)
		require.NoError(t, err)
		assert.Equal(t, rchannel.LastPostAt, rchannel2.LastPostAt)
		assert.Equal(t, int64(2), rchannel2.TotalMsgCount)

		post = model.Post{}
		post.ChannelID = channel.ID
		post.UserID = model.NewID()
		post.Message = "zz" + model.NewID() + "b"

		// We need to sleep here to be sure the post is not created during the same millisecond
		time.Sleep(time.Millisecond)
		_, err = ss.Post().Save(&post)
		require.NoError(t, err)

		rchannel3, err := ss.Channel().Get(channel.ID, false)
		require.NoError(t, err)
		assert.Greater(t, rchannel3.LastPostAt, rchannel2.LastPostAt)
		assert.Equal(t, int64(3), rchannel3.TotalMsgCount)
	})
}

func testPostStoreSaveMultiple(t *testing.T, ss store.Store) {
	p1 := model.Post{}
	p1.ChannelID = model.NewID()
	p1.UserID = model.NewID()
	p1.Message = "zz" + model.NewID() + "b"

	p2 := model.Post{}
	p2.ChannelID = model.NewID()
	p2.UserID = model.NewID()
	p2.Message = "zz" + model.NewID() + "b"

	p3 := model.Post{}
	p3.ChannelID = model.NewID()
	p3.UserID = model.NewID()
	p3.Message = "zz" + model.NewID() + "b"

	p4 := model.Post{}
	p4.ChannelID = model.NewID()
	p4.UserID = model.NewID()
	p4.Message = "zz" + model.NewID() + "b"

	t.Run("Save correctly a new set of posts", func(t *testing.T) {
		newPosts, errIDx, err := ss.Post().SaveMultiple([]*model.Post{&p1, &p2, &p3})
		require.NoError(t, err)
		require.Equal(t, -1, errIDx)
		for _, post := range newPosts {
			storedPost, err := ss.Post().GetSingle(post.ID, false)
			assert.NoError(t, err)
			assert.Equal(t, post.ChannelID, storedPost.ChannelID)
			assert.Equal(t, post.Message, storedPost.Message)
			assert.Equal(t, post.UserID, storedPost.UserID)
		}
	})

	t.Run("Save replies", func(t *testing.T) {
		o1 := model.Post{}
		o1.ChannelID = model.NewID()
		o1.UserID = model.NewID()
		o1.RootID = model.NewID()
		o1.Message = "zz" + model.NewID() + "b"

		o2 := model.Post{}
		o2.ChannelID = model.NewID()
		o2.UserID = model.NewID()
		o2.RootID = o1.RootID
		o2.Message = "zz" + model.NewID() + "b"

		o3 := model.Post{}
		o3.ChannelID = model.NewID()
		o3.UserID = model.NewID()
		o3.RootID = model.NewID()
		o3.Message = "zz" + model.NewID() + "b"

		o4 := model.Post{}
		o4.ChannelID = model.NewID()
		o4.UserID = model.NewID()
		o4.Message = "zz" + model.NewID() + "b"

		newPosts, errIDx, err := ss.Post().SaveMultiple([]*model.Post{&o1, &o2, &o3, &o4})
		require.NoError(t, err, "couldn't save item")
		require.Equal(t, -1, errIDx)
		assert.Len(t, newPosts, 4)
		assert.Equal(t, int64(2), newPosts[0].ReplyCount)
		assert.Equal(t, int64(2), newPosts[1].ReplyCount)
		assert.Equal(t, int64(1), newPosts[2].ReplyCount)
		assert.Equal(t, int64(0), newPosts[3].ReplyCount)
	})

	t.Run("Try to save mixed, already saved and not saved posts", func(t *testing.T) {
		newPosts, errIDx, err := ss.Post().SaveMultiple([]*model.Post{&p4, &p3})
		require.Error(t, err)
		require.Equal(t, 1, errIDx)
		require.Nil(t, newPosts)
		storedPost, err := ss.Post().GetSingle(p3.ID, false)
		assert.NoError(t, err)
		assert.Equal(t, p3.ChannelID, storedPost.ChannelID)
		assert.Equal(t, p3.Message, storedPost.Message)
		assert.Equal(t, p3.UserID, storedPost.UserID)

		storedPost, err = ss.Post().GetSingle(p4.ID, false)
		assert.Error(t, err)
		assert.Nil(t, storedPost)
	})

	t.Run("Update reply should update the UpdateAt of the root post", func(t *testing.T) {
		rootPost := model.Post{}
		rootPost.ChannelID = model.NewID()
		rootPost.UserID = model.NewID()
		rootPost.Message = "zz" + model.NewID() + "b"

		replyPost := model.Post{}
		replyPost.ChannelID = rootPost.ChannelID
		replyPost.UserID = model.NewID()
		replyPost.Message = "zz" + model.NewID() + "b"
		replyPost.RootID = rootPost.ID

		_, _, err := ss.Post().SaveMultiple([]*model.Post{&rootPost, &replyPost})
		require.NoError(t, err)

		rrootPost, err := ss.Post().GetSingle(rootPost.ID, false)
		require.NoError(t, err)
		assert.Equal(t, rrootPost.UpdateAt, rootPost.UpdateAt)

		replyPost2 := model.Post{}
		replyPost2.ChannelID = rootPost.ChannelID
		replyPost2.UserID = model.NewID()
		replyPost2.Message = "zz" + model.NewID() + "b"
		replyPost2.RootID = rootPost.ID

		replyPost3 := model.Post{}
		replyPost3.ChannelID = rootPost.ChannelID
		replyPost3.UserID = model.NewID()
		replyPost3.Message = "zz" + model.NewID() + "b"
		replyPost3.RootID = rootPost.ID

		_, _, err = ss.Post().SaveMultiple([]*model.Post{&replyPost2, &replyPost3})
		require.NoError(t, err)

		rrootPost2, err := ss.Post().GetSingle(rootPost.ID, false)
		require.NoError(t, err)
		assert.Greater(t, rrootPost2.UpdateAt, rrootPost.UpdateAt)
	})

	t.Run("Create a post should update the channel LastPostAt and the total messages count by one", func(t *testing.T) {
		channel := model.Channel{}
		channel.Name = "zz" + model.NewID() + "b"
		channel.DisplayName = "zz" + model.NewID() + "b"
		channel.Type = model.ChannelTypeOpen

		_, err := ss.Channel().Save(&channel, 100)
		require.NoError(t, err)

		post1 := model.Post{}
		post1.ChannelID = channel.ID
		post1.UserID = model.NewID()
		post1.Message = "zz" + model.NewID() + "b"

		post2 := model.Post{}
		post2.ChannelID = channel.ID
		post2.UserID = model.NewID()
		post2.Message = "zz" + model.NewID() + "b"
		post2.CreateAt = 5

		post3 := model.Post{}
		post3.ChannelID = channel.ID
		post3.UserID = model.NewID()
		post3.Message = "zz" + model.NewID() + "b"

		_, _, err = ss.Post().SaveMultiple([]*model.Post{&post1, &post2, &post3})
		require.NoError(t, err)

		rchannel, err := ss.Channel().Get(channel.ID, false)
		require.NoError(t, err)
		assert.Greater(t, rchannel.LastPostAt, channel.LastPostAt)
		assert.Equal(t, int64(3), rchannel.TotalMsgCount)
	})
}

func testPostStoreSaveChannelMsgCounts(t *testing.T, ss store.Store) {
	c1 := &model.Channel{Name: model.NewID(), DisplayName: "posttestchannel", Type: model.ChannelTypeOpen}
	_, err := ss.Channel().Save(c1, 1000000)
	require.NoError(t, err)

	o1 := model.Post{}
	o1.ChannelID = c1.ID
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"

	_, err = ss.Post().Save(&o1)
	require.NoError(t, err)

	c1, err = ss.Channel().Get(c1.ID, false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), c1.TotalMsgCount, "Message count should update by 1")

	o1.ID = ""
	o1.Type = model.PostTypeAddToTeam
	_, err = ss.Post().Save(&o1)
	require.NoError(t, err)

	o1.ID = ""
	o1.Type = model.PostTypeRemoveFromTeam
	_, err = ss.Post().Save(&o1)
	require.NoError(t, err)

	c1, err = ss.Channel().Get(c1.ID, false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), c1.TotalMsgCount, "Message count should not update for team add/removed message")

	oldLastPostAt := c1.LastPostAt

	o2 := model.Post{}
	o2.ChannelID = c1.ID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.CreateAt = int64(7)
	_, err = ss.Post().Save(&o2)
	require.NoError(t, err)

	c1, err = ss.Channel().Get(c1.ID, false)
	require.NoError(t, err)
	assert.Equal(t, oldLastPostAt, c1.LastPostAt, "LastPostAt should not update for old message save")
}

func testPostStoreGet(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"

	etag1 := ss.Post().GetEtag(o1.ChannelID, false, false)
	require.Equal(t, 0, strings.Index(etag1, model.CurrentVersion+"."), "Invalid Etag")

	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	etag2 := ss.Post().GetEtag(o1.ChannelID, false, false)
	require.Equal(t, 0, strings.Index(etag2, fmt.Sprintf("%v.%v", model.CurrentVersion, o1.UpdateAt)), "Invalid Etag")

	r1, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)
	require.Equal(t, r1.Posts[o1.ID].CreateAt, o1.CreateAt, "invalid returned post")

	_, err = ss.Post().Get(context.Background(), "123", false, false, false, "")
	require.Error(t, err, "Missing id should have failed")

	_, err = ss.Post().Get(context.Background(), "", false, false, false, "")
	require.Error(t, err, "should fail for blank post ids")
}

func testPostStoreGetForThread(t *testing.T, ss store.Store) {
	o1 := &model.Post{ChannelID: model.NewID(), UserID: model.NewID(), Message: "zz" + model.NewID() + "b"}
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)
	_, err = ss.Post().Save(&model.Post{ChannelID: o1.ChannelID, UserID: model.NewID(), Message: "zz" + model.NewID() + "b", RootID: o1.ID})
	require.NoError(t, err)

	threadMembership := &model.ThreadMembership{
		PostID:         o1.ID,
		UserID:         o1.UserID,
		Following:      true,
		LastViewed:     0,
		LastUpdated:    0,
		UnreadMentions: 0,
	}
	_, err = ss.Thread().SaveMembership(threadMembership)
	require.NoError(t, err)
	r1, err := ss.Post().Get(context.Background(), o1.ID, false, true, false, o1.UserID)
	require.NoError(t, err)
	require.Equal(t, r1.Posts[o1.ID].CreateAt, o1.CreateAt, "invalid returned post")
	require.True(t, *r1.Posts[o1.ID].IsFollowing)
}

func testPostStoreGetSingle(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = o1.UserID
	o2.Message = "zz" + model.NewID() + "c"

	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	err = ss.Post().Delete(o2.ID, model.GetMillis(), o2.UserID)
	require.NoError(t, err)

	post, err := ss.Post().GetSingle(o1.ID, false)
	require.NoError(t, err)
	require.Equal(t, post.CreateAt, o1.CreateAt, "invalid returned post")

	post, err = ss.Post().GetSingle(o2.ID, false)
	require.Error(t, err, "should not return deleted post")

	post, err = ss.Post().GetSingle(o2.ID, true)
	require.NoError(t, err)
	require.Equal(t, post.CreateAt, o2.CreateAt, "invalid returned post")
	require.NotZero(t, post.DeleteAt, "DeleteAt should be non-zero")

	_, err = ss.Post().GetSingle("123", false)
	require.Error(t, err, "Missing id should have failed")
}

func testPostStoreUpdate(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "CCCCCCCCC"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "QQQQQQQQQQ"
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	r1, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)
	ro1 := r1.Posts[o1.ID]

	r2, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)
	ro2 := r2.Posts[o2.ID]

	r3, err := ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.NoError(t, err)
	ro3 := r3.Posts[o3.ID]

	require.Equal(t, ro1.Message, o1.Message, "Failed to save/get")

	o1a := ro1.Clone()
	o1a.Message = ro1.Message + "BBBBBBBBBB"
	_, err = ss.Post().Update(o1a, ro1)
	require.NoError(t, err)

	r1, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)

	ro1a := r1.Posts[o1.ID]
	require.Equal(t, ro1a.Message, o1a.Message, "Failed to update/get")

	o2a := ro2.Clone()
	o2a.Message = ro2.Message + "DDDDDDD"
	_, err = ss.Post().Update(o2a, ro2)
	require.NoError(t, err)

	r2, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)
	ro2a := r2.Posts[o2.ID]

	require.Equal(t, ro2a.Message, o2a.Message, "Failed to update/get")

	o3a := ro3.Clone()
	o3a.Message = ro3.Message + "WWWWWWW"
	_, err = ss.Post().Update(o3a, ro3)
	require.NoError(t, err)

	r3, err = ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.NoError(t, err)
	ro3a := r3.Posts[o3.ID]

	if ro3a.Message != o3a.Message {
		require.Equal(t, ro3a.Hashtags, o3a.Hashtags, "Failed to update/get")
	}

	o4, err := ss.Post().Save(&model.Post{
		ChannelID: model.NewID(),
		UserID:    model.NewID(),
		Message:   model.NewID(),
		Filenames: []string{"test"},
	})
	require.NoError(t, err)

	r4, err := ss.Post().Get(context.Background(), o4.ID, false, false, false, "")
	require.NoError(t, err)
	ro4 := r4.Posts[o4.ID]

	o4a := ro4.Clone()
	o4a.Filenames = []string{}
	o4a.FileIDs = []string{model.NewID()}
	_, err = ss.Post().Update(o4a, ro4)
	require.NoError(t, err)

	r4, err = ss.Post().Get(context.Background(), o4.ID, false, false, false, "")
	require.NoError(t, err)

	ro4a := r4.Posts[o4.ID]
	require.Empty(t, ro4a.Filenames, "Failed to clear Filenames")
	require.Len(t, ro4a.FileIDs, 1, "Failed to set FileIds")
}

func testPostStoreDelete(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	deleteByID := model.NewID()

	etag1 := ss.Post().GetEtag(o1.ChannelID, false, false)
	require.Equal(t, 0, strings.Index(etag1, model.CurrentVersion+"."), "Invalid Etag")

	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	r1, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)
	require.Equal(t, r1.Posts[o1.ID].CreateAt, o1.CreateAt, "invalid returned post")

	err = ss.Post().Delete(o1.ID, model.GetMillis(), deleteByID)
	require.NoError(t, err)

	posts, _ := ss.Post().GetPostsCreatedAt(o1.ChannelID, o1.CreateAt)
	post := posts[0]
	actual := post.GetProp(model.PostPropsDeleteBy)

	assert.Equal(t, deleteByID, actual, "Expected (*Post).Props[model.PostPropsDeleteBy] to be %v but got %v.", deleteByID, actual)

	r3, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.Error(t, err, "Missing id should have failed - PostList %v", r3)

	etag2 := ss.Post().GetEtag(o1.ChannelID, false, false)
	require.Equal(t, 0, strings.Index(etag2, model.CurrentVersion+"."), "Invalid Etag")
}

func testPostStoreDelete1Level(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	err = ss.Post().Delete(o1.ID, model.GetMillis(), "")
	require.NoError(t, err)

	_, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")

	_, err = ss.Post().Get(context.Background(), o2.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")
}

func testPostStoreDelete2Level(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "b"
	o3.ParentID = o2.ID
	o3.RootID = o1.ID
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	o4 := &model.Post{}
	o4.ChannelID = model.NewID()
	o4.UserID = model.NewID()
	o4.Message = "zz" + model.NewID() + "b"
	o4, err = ss.Post().Save(o4)
	require.NoError(t, err)

	err = ss.Post().Delete(o1.ID, model.GetMillis(), "")
	require.NoError(t, err)

	_, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")

	_, err = ss.Post().Get(context.Background(), o2.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")

	_, err = ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")

	_, err = ss.Post().Get(context.Background(), o4.ID, false, false, false, "")
	require.NoError(t, err)
}

func testPostStorePermDelete1Level(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = model.NewID()
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "b"
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	err2 := ss.Post().PermanentDeleteByUser(o2.UserID)
	require.NoError(t, err2)

	_, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err, "Deleted id shouldn't have failed")

	_, err = ss.Post().Get(context.Background(), o2.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")

	err = ss.Post().PermanentDeleteByChannel(o3.ChannelID)
	require.NoError(t, err)

	_, err = ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")
}

func testPostStorePermDelete1Level2(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = model.NewID()
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "b"
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	err2 := ss.Post().PermanentDeleteByUser(o1.UserID)
	require.NoError(t, err2)

	_, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")

	_, err = ss.Post().Get(context.Background(), o2.ID, false, false, false, "")
	require.Error(t, err, "Deleted id should have failed")

	_, err = ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.NoError(t, err, "Deleted id should have failed")
}

func testPostStoreGetWithChildren(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "b"
	o3.ParentID = o2.ID
	o3.RootID = o1.ID
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	pl, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)

	require.Len(t, pl.Posts, 3, "invalid returned post")

	dErr := ss.Post().Delete(o3.ID, model.GetMillis(), "")
	require.NoError(t, dErr)

	pl, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)

	require.Len(t, pl.Posts, 2, "invalid returned post")

	dErr = ss.Post().Delete(o2.ID, model.GetMillis(), "")
	require.NoError(t, dErr)

	pl, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)

	require.Len(t, pl.Posts, 1, "invalid returned post")
}

func testPostStoreGetPostsWithDetails(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	_, err = ss.Post().Save(o2)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o2a := &model.Post{}
	o2a.ChannelID = o1.ChannelID
	o2a.UserID = model.NewID()
	o2a.Message = "zz" + model.NewID() + "b"
	o2a.ParentID = o1.ID
	o2a.RootID = o1.ID
	o2a, err = ss.Post().Save(o2a)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "b"
	o3.ParentID = o1.ID
	o3.RootID = o1.ID
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o4 := &model.Post{}
	o4.ChannelID = o1.ChannelID
	o4.UserID = model.NewID()
	o4.Message = "zz" + model.NewID() + "b"
	o4, err = ss.Post().Save(o4)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o5 := &model.Post{}
	o5.ChannelID = o1.ChannelID
	o5.UserID = model.NewID()
	o5.Message = "zz" + model.NewID() + "b"
	o5.ParentID = o4.ID
	o5.RootID = o4.ID
	o5, err = ss.Post().Save(o5)
	require.NoError(t, err)

	r1, err := ss.Post().GetPosts(model.GetPostsOptions{ChannelID: o1.ChannelID, Page: 0, PerPage: 4}, false)
	require.NoError(t, err)

	require.Equal(t, r1.Order[0], o5.ID, "invalid order")
	require.Equal(t, r1.Order[1], o4.ID, "invalid order")
	require.Equal(t, r1.Order[2], o3.ID, "invalid order")
	require.Equal(t, r1.Order[3], o2a.ID, "invalid order")

	//the last 4, + o1 (o2a and o3's parent) + o2 (in same thread as o2a and o3)
	require.Len(t, r1.Posts, 6, "wrong size")

	require.Equal(t, r1.Posts[o1.ID].Message, o1.Message, "Missing parent")

	r2, err := ss.Post().GetPosts(model.GetPostsOptions{ChannelID: o1.ChannelID, Page: 0, PerPage: 4}, false)
	require.NoError(t, err)

	require.Equal(t, r2.Order[0], o5.ID, "invalid order")
	require.Equal(t, r2.Order[1], o4.ID, "invalid order")
	require.Equal(t, r2.Order[2], o3.ID, "invalid order")
	require.Equal(t, r2.Order[3], o2a.ID, "invalid order")

	//the last 4, + o1 (o2a and o3's parent) + o2 (in same thread as o2a and o3)
	require.Len(t, r2.Posts, 6, "wrong size")

	require.Equal(t, r2.Posts[o1.ID].Message, o1.Message, "Missing parent")

	// Run once to fill cache
	_, err = ss.Post().GetPosts(model.GetPostsOptions{ChannelID: o1.ChannelID, Page: 0, PerPage: 30}, false)
	require.NoError(t, err)

	o6 := &model.Post{}
	o6.ChannelID = o1.ChannelID
	o6.UserID = model.NewID()
	o6.Message = "zz" + model.NewID() + "b"
	_, err = ss.Post().Save(o6)
	require.NoError(t, err)

	r3, err := ss.Post().GetPosts(model.GetPostsOptions{ChannelID: o1.ChannelID, Page: 0, PerPage: 30}, false)
	require.NoError(t, err)
	assert.Equal(t, 7, len(r3.Order))
}

func testPostStoreGetPostsBeforeAfter(t *testing.T, ss store.Store) {
	t.Run("without threads", func(t *testing.T) {
		channelID := model.NewID()
		userID := model.NewID()

		var posts []*model.Post
		for i := 0; i < 10; i++ {
			post, err := ss.Post().Save(&model.Post{
				ChannelID: channelID,
				UserID:    userID,
				Message:   "message",
			})
			require.NoError(t, err)

			posts = append(posts, post)

			time.Sleep(time.Millisecond)
		}

		t.Run("should return error if negative Page/PerPage options are passed", func(t *testing.T) {
			postList, err := ss.Post().GetPostsAfter(model.GetPostsOptions{ChannelID: channelID, PostID: posts[0].ID, Page: 0, PerPage: -1})
			assert.Nil(t, postList)
			assert.Error(t, err)
			assert.IsType(t, &store.ErrInvalidInput{}, err)

			postList, err = ss.Post().GetPostsAfter(model.GetPostsOptions{ChannelID: channelID, PostID: posts[0].ID, Page: -1, PerPage: 10})
			assert.Nil(t, postList)
			assert.Error(t, err)
			assert.IsType(t, &store.ErrInvalidInput{}, err)
		})

		t.Run("should not return anything before the first post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsBefore(model.GetPostsOptions{ChannelID: channelID, PostID: posts[0].ID, Page: 0, PerPage: 10})
			assert.NoError(t, err)

			assert.Equal(t, []string{}, postList.Order)
			assert.Equal(t, map[string]*model.Post{}, postList.Posts)
		})

		t.Run("should return posts before a post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsBefore(model.GetPostsOptions{ChannelID: channelID, PostID: posts[5].ID, Page: 0, PerPage: 10})
			assert.NoError(t, err)

			assert.Equal(t, []string{posts[4].ID, posts[3].ID, posts[2].ID, posts[1].ID, posts[0].ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				posts[0].ID: posts[0],
				posts[1].ID: posts[1],
				posts[2].ID: posts[2],
				posts[3].ID: posts[3],
				posts[4].ID: posts[4],
			}, postList.Posts)
		})

		t.Run("should limit posts before", func(t *testing.T) {
			postList, err := ss.Post().GetPostsBefore(model.GetPostsOptions{ChannelID: channelID, PostID: posts[5].ID, PerPage: 2})
			assert.NoError(t, err)

			assert.Equal(t, []string{posts[4].ID, posts[3].ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				posts[3].ID: posts[3],
				posts[4].ID: posts[4],
			}, postList.Posts)
		})

		t.Run("should not return anything after the last post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsAfter(model.GetPostsOptions{ChannelID: channelID, PostID: posts[len(posts)-1].ID, PerPage: 10})
			assert.NoError(t, err)

			assert.Equal(t, []string{}, postList.Order)
			assert.Equal(t, map[string]*model.Post{}, postList.Posts)
		})

		t.Run("should return posts after a post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsAfter(model.GetPostsOptions{ChannelID: channelID, PostID: posts[5].ID, PerPage: 10})
			assert.NoError(t, err)

			assert.Equal(t, []string{posts[9].ID, posts[8].ID, posts[7].ID, posts[6].ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				posts[6].ID: posts[6],
				posts[7].ID: posts[7],
				posts[8].ID: posts[8],
				posts[9].ID: posts[9],
			}, postList.Posts)
		})

		t.Run("should limit posts after", func(t *testing.T) {
			postList, err := ss.Post().GetPostsAfter(model.GetPostsOptions{ChannelID: channelID, PostID: posts[5].ID, PerPage: 2})
			assert.NoError(t, err)

			assert.Equal(t, []string{posts[7].ID, posts[6].ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				posts[6].ID: posts[6],
				posts[7].ID: posts[7],
			}, postList.Posts)
		})
	})
	t.Run("with threads", func(t *testing.T) {
		channelID := model.NewID()
		userID := model.NewID()

		// This creates a series of posts that looks like:
		// post1
		// post2
		// post3 (in response to post1)
		// post4 (in response to post2)
		// post5
		// post6 (in response to post2)

		post1, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		post1.ReplyCount = 1
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post2, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		post2.ReplyCount = 2
		time.Sleep(time.Millisecond)

		post3, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			ParentID:  post1.ID,
			RootID:    post1.ID,
			Message:   "message",
		})
		require.NoError(t, err)
		post3.ReplyCount = 1
		time.Sleep(time.Millisecond)

		post4, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			RootID:    post2.ID,
			ParentID:  post2.ID,
			Message:   "message",
		})
		require.NoError(t, err)
		post4.ReplyCount = 2
		time.Sleep(time.Millisecond)

		post5, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post6, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			ParentID:  post2.ID,
			RootID:    post2.ID,
			Message:   "message",
		})
		post6.ReplyCount = 2
		require.NoError(t, err)

		// Adding a post to a thread changes the UpdateAt timestamp of the parent post
		post1.UpdateAt = post3.UpdateAt
		post2.UpdateAt = post6.UpdateAt

		t.Run("should return each post and thread before a post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsBefore(model.GetPostsOptions{ChannelID: channelID, PostID: post4.ID, PerPage: 2})
			assert.NoError(t, err)

			assert.Equal(t, []string{post3.ID, post2.ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				post1.ID: post1,
				post2.ID: post2,
				post3.ID: post3,
				post4.ID: post4,
				post6.ID: post6,
			}, postList.Posts)
		})

		t.Run("should return each post and the root of each thread after a post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsAfter(model.GetPostsOptions{ChannelID: channelID, PostID: post4.ID, PerPage: 2})
			assert.NoError(t, err)

			assert.Equal(t, []string{post6.ID, post5.ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				post2.ID: post2,
				post4.ID: post4,
				post5.ID: post5,
				post6.ID: post6,
			}, postList.Posts)
		})
	})
	t.Run("with threads (skipFetchThreads)", func(t *testing.T) {
		channelID := model.NewID()
		userID := model.NewID()

		// This creates a series of posts that looks like:
		// post1
		// post2
		// post3 (in response to post1)
		// post4 (in response to post2)
		// post5
		// post6 (in response to post2)

		post1, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "post1",
		})
		require.NoError(t, err)
		post1.ReplyCount = 1
		time.Sleep(time.Millisecond)

		post2, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "post2",
		})
		require.NoError(t, err)
		post2.ReplyCount = 2
		time.Sleep(time.Millisecond)

		post3, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			ParentID:  post1.ID,
			RootID:    post1.ID,
			Message:   "post3",
		})
		require.NoError(t, err)
		post3.ReplyCount = 1
		time.Sleep(time.Millisecond)

		post4, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			RootID:    post2.ID,
			ParentID:  post2.ID,
			Message:   "post4",
		})
		require.NoError(t, err)
		post4.ReplyCount = 2
		time.Sleep(time.Millisecond)

		post5, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "post5",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post6, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			ParentID:  post2.ID,
			RootID:    post2.ID,
			Message:   "post6",
		})
		post6.ReplyCount = 2
		require.NoError(t, err)

		// Adding a post to a thread changes the UpdateAt timestamp of the parent post
		post1.UpdateAt = post3.UpdateAt
		post2.UpdateAt = post6.UpdateAt

		t.Run("should return each post and thread before a post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsBefore(model.GetPostsOptions{ChannelID: channelID, PostID: post4.ID, PerPage: 2, SkipFetchThreads: true})
			assert.NoError(t, err)

			assert.Equal(t, []string{post3.ID, post2.ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				post1.ID: post1,
				post2.ID: post2,
				post3.ID: post3,
			}, postList.Posts)
		})

		t.Run("should return each post and thread before a post with limit", func(t *testing.T) {
			postList, err := ss.Post().GetPostsBefore(model.GetPostsOptions{ChannelID: channelID, PostID: post4.ID, PerPage: 1, SkipFetchThreads: true})
			assert.NoError(t, err)

			assert.Equal(t, []string{post3.ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				post1.ID: post1,
				post3.ID: post3,
			}, postList.Posts)
		})

		t.Run("should return each post and the root of each thread after a post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsAfter(model.GetPostsOptions{ChannelID: channelID, PostID: post4.ID, PerPage: 2, SkipFetchThreads: true})
			assert.NoError(t, err)

			assert.Equal(t, []string{post6.ID, post5.ID}, postList.Order)
			assert.Equal(t, map[string]*model.Post{
				post2.ID: post2,
				post5.ID: post5,
				post6.ID: post6,
			}, postList.Posts)
		})
	})
	t.Run("with threads (collapsedThreads)", func(t *testing.T) {
		channelID := model.NewID()
		userID := model.NewID()

		// This creates a series of posts that looks like:
		// post1
		// post2
		// post3 (in response to post1)
		// post4 (in response to post2)
		// post5
		// post6 (in response to post2)

		post1, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "post1",
		})
		require.NoError(t, err)
		post1.ReplyCount = 1
		time.Sleep(time.Millisecond)

		post2, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "post2",
		})
		require.NoError(t, err)
		post2.ReplyCount = 2
		time.Sleep(time.Millisecond)

		post3, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			ParentID:  post1.ID,
			RootID:    post1.ID,
			Message:   "post3",
		})
		require.NoError(t, err)
		post3.ReplyCount = 1
		time.Sleep(time.Millisecond)

		post4, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			RootID:    post2.ID,
			ParentID:  post2.ID,
			Message:   "post4",
		})
		require.NoError(t, err)
		post4.ReplyCount = 2
		time.Sleep(time.Millisecond)

		post5, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "post5",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post6, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			ParentID:  post2.ID,
			RootID:    post2.ID,
			Message:   "post6",
		})
		post6.ReplyCount = 2
		require.NoError(t, err)

		// Adding a post to a thread changes the UpdateAt timestamp of the parent post
		post1.UpdateAt = post3.UpdateAt
		post2.UpdateAt = post6.UpdateAt

		t.Run("should return each root post before a post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsBefore(model.GetPostsOptions{ChannelID: channelID, PostID: post4.ID, PerPage: 2, CollapsedThreads: true})
			assert.NoError(t, err)

			assert.Equal(t, []string{post2.ID, post1.ID}, postList.Order)
		})

		t.Run("should return each root post before a post with limit", func(t *testing.T) {
			postList, err := ss.Post().GetPostsBefore(model.GetPostsOptions{ChannelID: channelID, PostID: post4.ID, PerPage: 1, CollapsedThreads: true})
			assert.NoError(t, err)

			assert.Equal(t, []string{post2.ID}, postList.Order)
		})

		t.Run("should return each root after a post", func(t *testing.T) {
			postList, err := ss.Post().GetPostsAfter(model.GetPostsOptions{ChannelID: channelID, PostID: post4.ID, PerPage: 2, CollapsedThreads: true})
			require.NoError(t, err)

			assert.Equal(t, []string{post5.ID}, postList.Order)
		})
	})
}

func testPostStoreGetPostsSince(t *testing.T, ss store.Store) {
	t.Run("should return posts created after the given time", func(t *testing.T) {
		channelID := model.NewID()
		userID := model.NewID()

		post1, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		_, err = ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post3, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post4, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post5, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
			RootID:    post3.ID,
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post6, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
			RootID:    post1.ID,
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		postList, err := ss.Post().GetPostsSince(model.GetPostsSinceOptions{ChannelID: channelID, Time: post3.CreateAt}, false)
		require.NoError(t, err)

		assert.Equal(t, []string{
			post6.ID,
			post5.ID,
			post4.ID,
			post3.ID,
			post1.ID,
		}, postList.Order)

		assert.Len(t, postList.Posts, 5)
		assert.NotNil(t, postList.Posts[post1.ID], "should return the parent post")
		assert.NotNil(t, postList.Posts[post3.ID])
		assert.NotNil(t, postList.Posts[post4.ID])
		assert.NotNil(t, postList.Posts[post5.ID])
		assert.NotNil(t, postList.Posts[post6.ID])
	})

	t.Run("should return empty list when nothing has changed", func(t *testing.T) {
		channelID := model.NewID()
		userID := model.NewID()

		post1, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		postList, err := ss.Post().GetPostsSince(model.GetPostsSinceOptions{ChannelID: channelID, Time: post1.CreateAt}, false)
		assert.NoError(t, err)

		assert.Equal(t, []string{}, postList.Order)
		assert.Empty(t, postList.Posts)
	})

	t.Run("should not cache a timestamp of 0 when nothing has changed", func(t *testing.T) {
		ss.Post().ClearCaches()

		channelID := model.NewID()
		userID := model.NewID()

		post1, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		// Make a request that returns no results
		postList, err := ss.Post().GetPostsSince(model.GetPostsSinceOptions{ChannelID: channelID, Time: post1.CreateAt}, true)
		require.NoError(t, err)
		require.Equal(t, model.NewPostList(), postList)

		// And then ensure that it doesn't cause future requests to also return no results
		postList, err = ss.Post().GetPostsSince(model.GetPostsSinceOptions{ChannelID: channelID, Time: post1.CreateAt - 1}, true)
		require.NoError(t, err)

		assert.Equal(t, []string{post1.ID}, postList.Order)

		assert.Len(t, postList.Posts, 1)
		assert.NotNil(t, postList.Posts[post1.ID])
	})
}

func testPostStoreGetPosts(t *testing.T, ss store.Store) {
	channelID := model.NewID()
	userID := model.NewID()

	post1, err := ss.Post().Save(&model.Post{
		ChannelID: channelID,
		UserID:    userID,
		Message:   "message",
	})
	require.NoError(t, err)
	time.Sleep(time.Millisecond)

	post2, err := ss.Post().Save(&model.Post{
		ChannelID: channelID,
		UserID:    userID,
		Message:   "message",
	})
	require.NoError(t, err)
	time.Sleep(time.Millisecond)

	post3, err := ss.Post().Save(&model.Post{
		ChannelID: channelID,
		UserID:    userID,
		Message:   "message",
	})
	require.NoError(t, err)
	time.Sleep(time.Millisecond)

	post4, err := ss.Post().Save(&model.Post{
		ChannelID: channelID,
		UserID:    userID,
		Message:   "message",
	})
	require.NoError(t, err)
	time.Sleep(time.Millisecond)

	post5, err := ss.Post().Save(&model.Post{
		ChannelID: channelID,
		UserID:    userID,
		Message:   "message",
		RootID:    post3.ID,
	})
	require.NoError(t, err)
	time.Sleep(time.Millisecond)

	post6, err := ss.Post().Save(&model.Post{
		ChannelID: channelID,
		UserID:    userID,
		Message:   "message",
		RootID:    post1.ID,
	})
	require.NoError(t, err)

	t.Run("should return the last posts created in a channel", func(t *testing.T) {
		postList, err := ss.Post().GetPosts(model.GetPostsOptions{ChannelID: channelID, Page: 0, PerPage: 30, SkipFetchThreads: false}, false)
		assert.NoError(t, err)

		assert.Equal(t, []string{
			post6.ID,
			post5.ID,
			post4.ID,
			post3.ID,
			post2.ID,
			post1.ID,
		}, postList.Order)

		assert.Len(t, postList.Posts, 6)
		assert.NotNil(t, postList.Posts[post1.ID])
		assert.NotNil(t, postList.Posts[post2.ID])
		assert.NotNil(t, postList.Posts[post3.ID])
		assert.NotNil(t, postList.Posts[post4.ID])
		assert.NotNil(t, postList.Posts[post5.ID])
		assert.NotNil(t, postList.Posts[post6.ID])
	})

	t.Run("should return the last posts created in a channel and the threads and the reply count must be 0", func(t *testing.T) {
		postList, err := ss.Post().GetPosts(model.GetPostsOptions{ChannelID: channelID, Page: 0, PerPage: 2, SkipFetchThreads: false}, false)
		assert.NoError(t, err)

		assert.Equal(t, []string{
			post6.ID,
			post5.ID,
		}, postList.Order)

		assert.Len(t, postList.Posts, 4)
		require.NotNil(t, postList.Posts[post1.ID])
		require.NotNil(t, postList.Posts[post3.ID])
		require.NotNil(t, postList.Posts[post5.ID])
		require.NotNil(t, postList.Posts[post6.ID])
		assert.Equal(t, int64(0), postList.Posts[post1.ID].ReplyCount)
		assert.Equal(t, int64(0), postList.Posts[post3.ID].ReplyCount)
		assert.Equal(t, int64(0), postList.Posts[post5.ID].ReplyCount)
		assert.Equal(t, int64(0), postList.Posts[post6.ID].ReplyCount)
	})

	t.Run("should return the last posts created in a channel without the threads and the reply count must be correct", func(t *testing.T) {
		postList, err := ss.Post().GetPosts(model.GetPostsOptions{ChannelID: channelID, Page: 0, PerPage: 2, SkipFetchThreads: true}, false)
		require.NoError(t, err)

		assert.Equal(t, []string{
			post6.ID,
			post5.ID,
		}, postList.Order)

		assert.Len(t, postList.Posts, 4)
		assert.NotNil(t, postList.Posts[post5.ID])
		assert.NotNil(t, postList.Posts[post6.ID])
		assert.Equal(t, int64(1), postList.Posts[post5.ID].ReplyCount)
		assert.Equal(t, int64(1), postList.Posts[post6.ID].ReplyCount)
	})
}

func testPostStoreGetPostBeforeAfter(t *testing.T, ss store.Store) {
	channelID := model.NewID()

	o0 := &model.Post{}
	o0.ChannelID = channelID
	o0.UserID = model.NewID()
	o0.Message = "zz" + model.NewID() + "b"
	_, err := ss.Post().Save(o0)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o1 := &model.Post{}
	o1.ChannelID = channelID
	o1.Type = model.PostTypeJoinChannel
	o1.UserID = model.NewID()
	o1.Message = "system_join_channel message"
	_, err = ss.Post().Save(o1)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o0a := &model.Post{}
	o0a.ChannelID = channelID
	o0a.UserID = model.NewID()
	o0a.Message = "zz" + model.NewID() + "b"
	o0a.ParentID = o1.ID
	o0a.RootID = o1.ID
	_, err = ss.Post().Save(o0a)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o0b := &model.Post{}
	o0b.ChannelID = channelID
	o0b.UserID = model.NewID()
	o0b.Message = "deleted message"
	o0b.ParentID = o1.ID
	o0b.RootID = o1.ID
	o0b.DeleteAt = 1
	_, err = ss.Post().Save(o0b)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	otherChannelPost := &model.Post{}
	otherChannelPost.ChannelID = model.NewID()
	otherChannelPost.UserID = model.NewID()
	otherChannelPost.Message = "zz" + model.NewID() + "b"
	_, err = ss.Post().Save(otherChannelPost)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o2 := &model.Post{}
	o2.ChannelID = channelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	_, err = ss.Post().Save(o2)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o2a := &model.Post{}
	o2a.ChannelID = channelID
	o2a.UserID = model.NewID()
	o2a.Message = "zz" + model.NewID() + "b"
	o2a.ParentID = o2.ID
	o2a.RootID = o2.ID
	_, err = ss.Post().Save(o2a)
	require.NoError(t, err)

	rPostID1, err := ss.Post().GetPostIDBeforeTime(channelID, o0a.CreateAt, false)
	require.Equal(t, rPostID1, o1.ID, "should return before post o1")
	require.NoError(t, err)

	rPostID1, err = ss.Post().GetPostIDAfterTime(channelID, o0b.CreateAt, false)
	require.Equal(t, rPostID1, o2.ID, "should return before post o2")
	require.NoError(t, err)

	rPost1, err := ss.Post().GetPostAfterTime(channelID, o0b.CreateAt, false)
	require.Equal(t, rPost1.ID, o2.ID, "should return before post o2")
	require.NoError(t, err)

	rPostID2, err := ss.Post().GetPostIDBeforeTime(channelID, o0.CreateAt, false)
	require.Empty(t, rPostID2, "should return no post")
	require.NoError(t, err)

	rPostID2, err = ss.Post().GetPostIDAfterTime(channelID, o0.CreateAt, false)
	require.Equal(t, rPostID2, o1.ID, "should return before post o1")
	require.NoError(t, err)

	rPost2, err := ss.Post().GetPostAfterTime(channelID, o0.CreateAt, false)
	require.Equal(t, rPost2.ID, o1.ID, "should return before post o1")
	require.NoError(t, err)

	rPostID3, err := ss.Post().GetPostIDBeforeTime(channelID, o2a.CreateAt, false)
	require.Equal(t, rPostID3, o2.ID, "should return before post o2")
	require.NoError(t, err)

	rPostID3, err = ss.Post().GetPostIDAfterTime(channelID, o2a.CreateAt, false)
	require.Empty(t, rPostID3, "should return no post")
	require.NoError(t, err)

	rPost3, err := ss.Post().GetPostAfterTime(channelID, o2a.CreateAt, false)
	require.Empty(t, rPost3, "should return no post")
	require.NoError(t, err)
}

func testUserCountsWithPostsByDay(t *testing.T, ss store.Store) {
	t1 := &model.Team{}
	t1.DisplayName = "DisplayName"
	t1.Name = "zz" + model.NewID() + "b"
	t1.Email = MakeEmail()
	t1.Type = model.TeamOpen
	t1, err := ss.Team().Save(t1)
	require.NoError(t, err)

	c1 := &model.Channel{}
	c1.TeamID = t1.ID
	c1.DisplayName = "Channel2"
	c1.Name = "zz" + model.NewID() + "b"
	c1.Type = model.ChannelTypeOpen
	c1, nErr := ss.Channel().Save(c1, -1)
	require.NoError(t, nErr)

	o1 := &model.Post{}
	o1.ChannelID = c1.ID
	o1.UserID = model.NewID()
	o1.CreateAt = utils.MillisFromTime(utils.Yesterday())
	o1.Message = "zz" + model.NewID() + "b"
	o1, nErr = ss.Post().Save(o1)
	require.NoError(t, nErr)

	o1a := &model.Post{}
	o1a.ChannelID = c1.ID
	o1a.UserID = model.NewID()
	o1a.CreateAt = o1.CreateAt
	o1a.Message = "zz" + model.NewID() + "b"
	_, nErr = ss.Post().Save(o1a)
	require.NoError(t, nErr)

	o2 := &model.Post{}
	o2.ChannelID = c1.ID
	o2.UserID = model.NewID()
	o2.CreateAt = o1.CreateAt - (1000 * 60 * 60 * 24)
	o2.Message = "zz" + model.NewID() + "b"
	o2, nErr = ss.Post().Save(o2)
	require.NoError(t, nErr)

	o2a := &model.Post{}
	o2a.ChannelID = c1.ID
	o2a.UserID = o2.UserID
	o2a.CreateAt = o1.CreateAt - (1000 * 60 * 60 * 24)
	o2a.Message = "zz" + model.NewID() + "b"
	_, nErr = ss.Post().Save(o2a)
	require.NoError(t, nErr)

	r1, err := ss.Post().AnalyticsUserCountsWithPostsByDay(t1.ID)
	require.NoError(t, err)

	row1 := r1[0]
	require.Equal(t, float64(2), row1.Value, "wrong value")

	row2 := r1[1]
	require.Equal(t, float64(1), row2.Value, "wrong value")
}

func testPostCountsByDay(t *testing.T, ss store.Store) {
	t1 := &model.Team{}
	t1.DisplayName = "DisplayName"
	t1.Name = "zz" + model.NewID() + "b"
	t1.Email = MakeEmail()
	t1.Type = model.TeamOpen
	t1, err := ss.Team().Save(t1)
	require.NoError(t, err)

	c1 := &model.Channel{}
	c1.TeamID = t1.ID
	c1.DisplayName = "Channel2"
	c1.Name = "zz" + model.NewID() + "b"
	c1.Type = model.ChannelTypeOpen
	c1, nErr := ss.Channel().Save(c1, -1)
	require.NoError(t, nErr)

	o1 := &model.Post{}
	o1.ChannelID = c1.ID
	o1.UserID = model.NewID()
	o1.CreateAt = utils.MillisFromTime(utils.Yesterday())
	o1.Message = "zz" + model.NewID() + "b"
	o1.Hashtags = "hashtag"
	o1, nErr = ss.Post().Save(o1)
	require.NoError(t, nErr)

	o1a := &model.Post{}
	o1a.ChannelID = c1.ID
	o1a.UserID = model.NewID()
	o1a.CreateAt = o1.CreateAt
	o1a.Message = "zz" + model.NewID() + "b"
	o1a.FileIDs = []string{"fileId1"}
	_, nErr = ss.Post().Save(o1a)
	require.NoError(t, nErr)

	o2 := &model.Post{}
	o2.ChannelID = c1.ID
	o2.UserID = model.NewID()
	o2.CreateAt = o1.CreateAt - (1000 * 60 * 60 * 24 * 2)
	o2.Message = "zz" + model.NewID() + "b"
	o2.Filenames = []string{"filename1"}
	o2, nErr = ss.Post().Save(o2)
	require.NoError(t, nErr)

	o2a := &model.Post{}
	o2a.ChannelID = c1.ID
	o2a.UserID = o2.UserID
	o2a.CreateAt = o1.CreateAt - (1000 * 60 * 60 * 24 * 2)
	o2a.Message = "zz" + model.NewID() + "b"
	o2a.Hashtags = "hashtag"
	o2a.FileIDs = []string{"fileId2"}
	_, nErr = ss.Post().Save(o2a)
	require.NoError(t, nErr)

	bot1 := &model.Bot{
		Username:    "username",
		Description: "a bot",
		OwnerID:     model.NewID(),
		UserID:      model.NewID(),
	}
	_, nErr = ss.Bot().Save(bot1)
	require.NoError(t, nErr)

	b1 := &model.Post{}
	b1.Message = "bot message one"
	b1.ChannelID = c1.ID
	b1.UserID = bot1.UserID
	b1.CreateAt = utils.MillisFromTime(utils.Yesterday())
	_, nErr = ss.Post().Save(b1)
	require.NoError(t, nErr)

	b1a := &model.Post{}
	b1a.Message = "bot message two"
	b1a.ChannelID = c1.ID
	b1a.UserID = bot1.UserID
	b1a.CreateAt = utils.MillisFromTime(utils.Yesterday()) - (1000 * 60 * 60 * 24 * 2)
	_, nErr = ss.Post().Save(b1a)
	require.NoError(t, nErr)

	time.Sleep(1 * time.Second)

	// summary of posts
	// yesterday - 2 non-bot user posts, 1 bot user post
	// 3 days ago - 2 non-bot user posts, 1 bot user post

	// last 31 days, all users (including bots)
	postCountsOptions := &model.AnalyticsPostCountsOptions{TeamID: t1.ID, BotsOnly: false, YesterdayOnly: false}
	r1, err := ss.Post().AnalyticsPostCountsByDay(postCountsOptions)
	require.NoError(t, err)
	assert.Equal(t, float64(3), r1[0].Value)
	assert.Equal(t, float64(3), r1[1].Value)

	// last 31 days, bots only
	postCountsOptions = &model.AnalyticsPostCountsOptions{TeamID: t1.ID, BotsOnly: true, YesterdayOnly: false}
	r1, err = ss.Post().AnalyticsPostCountsByDay(postCountsOptions)
	require.NoError(t, err)
	assert.Equal(t, float64(1), r1[0].Value)
	assert.Equal(t, float64(1), r1[1].Value)

	// yesterday only, all users (including bots)
	postCountsOptions = &model.AnalyticsPostCountsOptions{TeamID: t1.ID, BotsOnly: false, YesterdayOnly: true}
	r1, err = ss.Post().AnalyticsPostCountsByDay(postCountsOptions)
	require.NoError(t, err)
	assert.Equal(t, float64(3), r1[0].Value)

	// yesterday only, bots only
	postCountsOptions = &model.AnalyticsPostCountsOptions{TeamID: t1.ID, BotsOnly: true, YesterdayOnly: true}
	r1, err = ss.Post().AnalyticsPostCountsByDay(postCountsOptions)
	require.NoError(t, err)
	assert.Equal(t, float64(1), r1[0].Value)

	// total
	r2, err := ss.Post().AnalyticsPostCount(t1.ID, false, false)
	require.NoError(t, err)
	assert.Equal(t, int64(6), r2)

	// total across teams
	r2, err = ss.Post().AnalyticsPostCount("", false, false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, r2, int64(6))

	// total across teams with files
	r2, err = ss.Post().AnalyticsPostCount("", true, false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, r2, int64(3))

	// total across teams with hastags
	r2, err = ss.Post().AnalyticsPostCount("", false, true)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, r2, int64(2))

	// total across teams with hastags and files
	r2, err = ss.Post().AnalyticsPostCount("", true, true)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, r2, int64(1))
}

func testPostStoreGetFlaggedPostsForTeam(t *testing.T, ss store.Store, s SqlStore) {
	c1 := &model.Channel{}
	c1.TeamID = model.NewID()
	c1.DisplayName = "Channel1"
	c1.Name = "zz" + model.NewID() + "b"
	c1.Type = model.ChannelTypeOpen
	c1, err := ss.Channel().Save(c1, -1)
	require.NoError(t, err)

	o1 := &model.Post{}
	o1.ChannelID = c1.ID
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err = ss.Post().Save(o1)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "b"
	o3.DeleteAt = 1
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o4 := &model.Post{}
	o4.ChannelID = model.NewID()
	o4.UserID = model.NewID()
	o4.Message = "zz" + model.NewID() + "b"
	o4, err = ss.Post().Save(o4)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	c2 := &model.Channel{}
	c2.DisplayName = "DMChannel1"
	c2.Name = "zz" + model.NewID() + "b"
	c2.Type = model.ChannelTypeDirect

	m1 := &model.ChannelMember{}
	m1.ChannelID = c2.ID
	m1.UserID = o1.UserID
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := &model.ChannelMember{}
	m2.ChannelID = c2.ID
	m2.UserID = model.NewID()
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	c2, err = ss.Channel().SaveDirectChannel(c2, m1, m2)
	require.NoError(t, err)

	o5 := &model.Post{}
	o5.ChannelID = c2.ID
	o5.UserID = m2.UserID
	o5.Message = "zz" + model.NewID() + "b"
	o5, err = ss.Post().Save(o5)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	r1, err := ss.Post().GetFlaggedPosts(o1.ChannelID, 0, 2)
	require.NoError(t, err)

	require.Empty(t, r1.Order, "should be empty")

	preferences := model.Preferences{
		{
			UserID:   o1.UserID,
			Category: model.PreferenceCategoryFlaggedPost,
			Name:     o1.ID,
			Value:    "true",
		},
	}

	err = ss.Preference().Save(&preferences)
	require.NoError(t, err)

	r2, err := ss.Post().GetFlaggedPostsForTeam(o1.UserID, c1.TeamID, 0, 2)
	require.NoError(t, err)
	require.Len(t, r2.Order, 1, "should have 1 post")

	preferences = model.Preferences{
		{
			UserID:   o1.UserID,
			Category: model.PreferenceCategoryFlaggedPost,
			Name:     o2.ID,
			Value:    "true",
		},
	}

	err = ss.Preference().Save(&preferences)
	require.NoError(t, err)

	r3, err := ss.Post().GetFlaggedPostsForTeam(o1.UserID, c1.TeamID, 0, 1)
	require.NoError(t, err)
	require.Len(t, r3.Order, 1, "should have 1 post")

	r3, err = ss.Post().GetFlaggedPostsForTeam(o1.UserID, c1.TeamID, 1, 1)
	require.NoError(t, err)
	require.Len(t, r3.Order, 1, "should have 1 post")

	r3, err = ss.Post().GetFlaggedPostsForTeam(o1.UserID, c1.TeamID, 1000, 10)
	require.NoError(t, err)
	require.Empty(t, r3.Order, "should be empty")

	r4, err := ss.Post().GetFlaggedPostsForTeam(o1.UserID, c1.TeamID, 0, 2)
	require.NoError(t, err)
	require.Len(t, r4.Order, 2, "should have 2 posts")

	preferences = model.Preferences{
		{
			UserID:   o1.UserID,
			Category: model.PreferenceCategoryFlaggedPost,
			Name:     o3.ID,
			Value:    "true",
		},
	}

	err = ss.Preference().Save(&preferences)
	require.NoError(t, err)

	r4, err = ss.Post().GetFlaggedPostsForTeam(o1.UserID, c1.TeamID, 0, 2)
	require.NoError(t, err)
	require.Len(t, r4.Order, 2, "should have 2 posts")

	preferences = model.Preferences{
		{
			UserID:   o1.UserID,
			Category: model.PreferenceCategoryFlaggedPost,
			Name:     o4.ID,
			Value:    "true",
		},
	}
	err = ss.Preference().Save(&preferences)
	require.NoError(t, err)

	r4, err = ss.Post().GetFlaggedPostsForTeam(o1.UserID, c1.TeamID, 0, 2)
	require.NoError(t, err)
	require.Len(t, r4.Order, 2, "should have 2 posts")

	r4, err = ss.Post().GetFlaggedPostsForTeam(o1.UserID, model.NewID(), 0, 2)
	require.NoError(t, err)
	require.Empty(t, r4.Order, "should have 0 posts")

	preferences = model.Preferences{
		{
			UserID:   o1.UserID,
			Category: model.PreferenceCategoryFlaggedPost,
			Name:     o5.ID,
			Value:    "true",
		},
	}
	err = ss.Preference().Save(&preferences)
	require.NoError(t, err)

	r4, err = ss.Post().GetFlaggedPostsForTeam(o1.UserID, c1.TeamID, 0, 10)
	require.NoError(t, err)
	require.Len(t, r4.Order, 3, "should have 3 posts")

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testPostStoreGetFlaggedPosts(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "b"
	o3.DeleteAt = 1
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	r1, err := ss.Post().GetFlaggedPosts(o1.UserID, 0, 2)
	require.NoError(t, err)
	require.Empty(t, r1.Order, "should be empty")

	preferences := model.Preferences{
		{
			UserID:   o1.UserID,
			Category: model.PreferenceCategoryFlaggedPost,
			Name:     o1.ID,
			Value:    "true",
		},
	}

	nErr := ss.Preference().Save(&preferences)
	require.NoError(t, nErr)

	r2, err := ss.Post().GetFlaggedPosts(o1.UserID, 0, 2)
	require.NoError(t, err)
	require.Len(t, r2.Order, 1, "should have 1 post")

	preferences = model.Preferences{
		{
			UserID:   o1.UserID,
			Category: model.PreferenceCategoryFlaggedPost,
			Name:     o2.ID,
			Value:    "true",
		},
	}

	nErr = ss.Preference().Save(&preferences)
	require.NoError(t, nErr)

	r3, err := ss.Post().GetFlaggedPosts(o1.UserID, 0, 1)
	require.NoError(t, err)
	require.Len(t, r3.Order, 1, "should have 1 post")

	r3, err = ss.Post().GetFlaggedPosts(o1.UserID, 1, 1)
	require.NoError(t, err)
	require.Len(t, r3.Order, 1, "should have 1 post")

	r3, err = ss.Post().GetFlaggedPosts(o1.UserID, 1000, 10)
	require.NoError(t, err)
	require.Empty(t, r3.Order, "should be empty")

	r4, err := ss.Post().GetFlaggedPosts(o1.UserID, 0, 2)
	require.NoError(t, err)
	require.Len(t, r4.Order, 2, "should have 2 posts")

	preferences = model.Preferences{
		{
			UserID:   o1.UserID,
			Category: model.PreferenceCategoryFlaggedPost,
			Name:     o3.ID,
			Value:    "true",
		},
	}

	nErr = ss.Preference().Save(&preferences)
	require.NoError(t, nErr)

	r4, err = ss.Post().GetFlaggedPosts(o1.UserID, 0, 2)
	require.NoError(t, err)
	require.Len(t, r4.Order, 2, "should have 2 posts")
}

func testPostStoreGetFlaggedPostsForChannel(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	// deleted post
	o3 := &model.Post{}
	o3.ChannelID = model.NewID()
	o3.UserID = o1.ChannelID
	o3.Message = "zz" + model.NewID() + "b"
	o3.DeleteAt = 1
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	o4 := &model.Post{}
	o4.ChannelID = model.NewID()
	o4.UserID = model.NewID()
	o4.Message = "zz" + model.NewID() + "b"
	o4, err = ss.Post().Save(o4)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	r, err := ss.Post().GetFlaggedPostsForChannel(o1.UserID, o1.ChannelID, 0, 10)
	require.NoError(t, err)
	require.Empty(t, r.Order, "should be empty")

	preference := model.Preference{
		UserID:   o1.UserID,
		Category: model.PreferenceCategoryFlaggedPost,
		Name:     o1.ID,
		Value:    "true",
	}

	nErr := ss.Preference().Save(&model.Preferences{preference})
	require.NoError(t, nErr)

	r, err = ss.Post().GetFlaggedPostsForChannel(o1.UserID, o1.ChannelID, 0, 10)
	require.NoError(t, err)
	require.Len(t, r.Order, 1, "should have 1 post")

	preference.Name = o2.ID
	nErr = ss.Preference().Save(&model.Preferences{preference})
	require.NoError(t, nErr)

	preference.Name = o3.ID
	nErr = ss.Preference().Save(&model.Preferences{preference})
	require.NoError(t, nErr)

	r, err = ss.Post().GetFlaggedPostsForChannel(o1.UserID, o1.ChannelID, 0, 1)
	require.NoError(t, err)
	require.Len(t, r.Order, 1, "should have 1 post")

	r, err = ss.Post().GetFlaggedPostsForChannel(o1.UserID, o1.ChannelID, 1, 1)
	require.NoError(t, err)
	require.Len(t, r.Order, 1, "should have 1 post")

	r, err = ss.Post().GetFlaggedPostsForChannel(o1.UserID, o1.ChannelID, 1000, 10)
	require.NoError(t, err)
	require.Empty(t, r.Order, "should be empty")

	r, err = ss.Post().GetFlaggedPostsForChannel(o1.UserID, o1.ChannelID, 0, 10)
	require.NoError(t, err)
	require.Len(t, r.Order, 2, "should have 2 posts")

	preference.Name = o4.ID
	nErr = ss.Preference().Save(&model.Preferences{preference})
	require.NoError(t, nErr)

	r, err = ss.Post().GetFlaggedPostsForChannel(o1.UserID, o4.ChannelID, 0, 10)
	require.NoError(t, err)
	require.Len(t, r.Order, 1, "should have 1 posts")
}

func testPostStoreGetPostsCreatedAt(t *testing.T, ss store.Store) {
	createTime := model.GetMillis() + 1

	o0 := &model.Post{}
	o0.ChannelID = model.NewID()
	o0.UserID = model.NewID()
	o0.Message = "zz" + model.NewID() + "b"
	o0.CreateAt = createTime
	o0, err := ss.Post().Save(o0)
	require.NoError(t, err)

	o1 := &model.Post{}
	o1.ChannelID = o0.ChannelID
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1.CreateAt = createTime
	o1, err = ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2.CreateAt = createTime + 1
	_, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = model.NewID()
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "b"
	o3.CreateAt = createTime
	_, err = ss.Post().Save(o3)
	require.NoError(t, err)

	r1, _ := ss.Post().GetPostsCreatedAt(o1.ChannelID, createTime)
	assert.Equal(t, 2, len(r1))
}

func testPostStoreOverwriteMultiple(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "CCCCCCCCC"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "QQQQQQQQQQ"
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	o4, err := ss.Post().Save(&model.Post{
		ChannelID: model.NewID(),
		UserID:    model.NewID(),
		Message:   model.NewID(),
		Filenames: []string{"test"},
	})
	require.NoError(t, err)

	o5, err := ss.Post().Save(&model.Post{
		ChannelID: model.NewID(),
		UserID:    model.NewID(),
		Message:   model.NewID(),
		Filenames: []string{"test2", "test3"},
	})
	require.NoError(t, err)

	r1, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)
	ro1 := r1.Posts[o1.ID]

	r2, err := ss.Post().Get(context.Background(), o2.ID, false, false, false, "")
	require.NoError(t, err)
	ro2 := r2.Posts[o2.ID]

	r3, err := ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.NoError(t, err)
	ro3 := r3.Posts[o3.ID]

	r4, err := ss.Post().Get(context.Background(), o4.ID, false, false, false, "")
	require.NoError(t, err)
	ro4 := r4.Posts[o4.ID]

	r5, err := ss.Post().Get(context.Background(), o5.ID, false, false, false, "")
	require.NoError(t, err)
	ro5 := r5.Posts[o5.ID]

	require.Equal(t, ro1.Message, o1.Message, "Failed to save/get")
	require.Equal(t, ro2.Message, o2.Message, "Failed to save/get")
	require.Equal(t, ro3.Message, o3.Message, "Failed to save/get")
	require.Equal(t, ro4.Message, o4.Message, "Failed to save/get")
	require.Equal(t, ro4.Filenames, o4.Filenames, "Failed to save/get")
	require.Equal(t, ro5.Message, o5.Message, "Failed to save/get")
	require.Equal(t, ro5.Filenames, o5.Filenames, "Failed to save/get")

	t.Run("overwrite changing message", func(t *testing.T) {
		o1a := ro1.Clone()
		o1a.Message = ro1.Message + "BBBBBBBBBB"

		o2a := ro2.Clone()
		o2a.Message = ro2.Message + "DDDDDDD"

		o3a := ro3.Clone()
		o3a.Message = ro3.Message + "WWWWWWW"

		_, errIDx, err := ss.Post().OverwriteMultiple([]*model.Post{o1a, o2a, o3a})
		require.NoError(t, err)
		require.Equal(t, -1, errIDx)

		r1, nErr := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
		require.NoError(t, nErr)
		ro1a := r1.Posts[o1.ID]

		r2, nErr = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
		require.NoError(t, nErr)
		ro2a := r2.Posts[o2.ID]

		r3, nErr = ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
		require.NoError(t, nErr)
		ro3a := r3.Posts[o3.ID]

		assert.Equal(t, ro1a.Message, o1a.Message, "Failed to overwrite/get")
		assert.Equal(t, ro2a.Message, o2a.Message, "Failed to overwrite/get")
		assert.Equal(t, ro3a.Message, o3a.Message, "Failed to overwrite/get")
	})

	t.Run("overwrite clearing filenames", func(t *testing.T) {
		o4a := ro4.Clone()
		o4a.Filenames = []string{}
		o4a.FileIDs = []string{model.NewID()}

		o5a := ro5.Clone()
		o5a.Filenames = []string{}
		o5a.FileIDs = []string{}

		_, errIDx, err := ss.Post().OverwriteMultiple([]*model.Post{o4a, o5a})
		require.NoError(t, err)
		require.Equal(t, -1, errIDx)

		r4, nErr := ss.Post().Get(context.Background(), o4.ID, false, false, false, "")
		require.NoError(t, nErr)
		ro4a := r4.Posts[o4.ID]

		r5, nErr = ss.Post().Get(context.Background(), o5.ID, false, false, false, "")
		require.NoError(t, nErr)
		ro5a := r5.Posts[o5.ID]

		require.Empty(t, ro4a.Filenames, "Failed to clear Filenames")
		require.Len(t, ro4a.FileIDs, 1, "Failed to set FileIds")
		require.Empty(t, ro5a.Filenames, "Failed to clear Filenames")
		require.Empty(t, ro5a.FileIDs, "Failed to set FileIds")
	})
}

func testPostStoreOverwrite(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "CCCCCCCCC"
	o2.ParentID = o1.ID
	o2.RootID = o1.ID
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "QQQQQQQQQQ"
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	o4, err := ss.Post().Save(&model.Post{
		ChannelID: model.NewID(),
		UserID:    model.NewID(),
		Message:   model.NewID(),
		Filenames: []string{"test"},
	})
	require.NoError(t, err)

	r1, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)
	ro1 := r1.Posts[o1.ID]

	r2, err := ss.Post().Get(context.Background(), o2.ID, false, false, false, "")
	require.NoError(t, err)
	ro2 := r2.Posts[o2.ID]

	r3, err := ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.NoError(t, err)
	ro3 := r3.Posts[o3.ID]

	r4, err := ss.Post().Get(context.Background(), o4.ID, false, false, false, "")
	require.NoError(t, err)
	ro4 := r4.Posts[o4.ID]

	require.Equal(t, ro1.Message, o1.Message, "Failed to save/get")
	require.Equal(t, ro2.Message, o2.Message, "Failed to save/get")
	require.Equal(t, ro3.Message, o3.Message, "Failed to save/get")
	require.Equal(t, ro4.Message, o4.Message, "Failed to save/get")

	t.Run("overwrite changing message", func(t *testing.T) {
		o1a := ro1.Clone()
		o1a.Message = ro1.Message + "BBBBBBBBBB"
		_, err = ss.Post().Overwrite(o1a)
		require.NoError(t, err)

		o2a := ro2.Clone()
		o2a.Message = ro2.Message + "DDDDDDD"
		_, err = ss.Post().Overwrite(o2a)
		require.NoError(t, err)

		o3a := ro3.Clone()
		o3a.Message = ro3.Message + "WWWWWWW"
		_, err = ss.Post().Overwrite(o3a)
		require.NoError(t, err)

		r1, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
		require.NoError(t, err)
		ro1a := r1.Posts[o1.ID]

		r2, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
		require.NoError(t, err)
		ro2a := r2.Posts[o2.ID]

		r3, err = ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
		require.NoError(t, err)
		ro3a := r3.Posts[o3.ID]

		assert.Equal(t, ro1a.Message, o1a.Message, "Failed to overwrite/get")
		assert.Equal(t, ro2a.Message, o2a.Message, "Failed to overwrite/get")
		assert.Equal(t, ro3a.Message, o3a.Message, "Failed to overwrite/get")
	})

	t.Run("overwrite clearing filenames", func(t *testing.T) {
		o4a := ro4.Clone()
		o4a.Filenames = []string{}
		o4a.FileIDs = []string{model.NewID()}
		_, err = ss.Post().Overwrite(o4a)
		require.NoError(t, err)

		r4, err = ss.Post().Get(context.Background(), o4.ID, false, false, false, "")
		require.NoError(t, err)

		ro4a := r4.Posts[o4.ID]
		require.Empty(t, ro4a.Filenames, "Failed to clear Filenames")
		require.Len(t, ro4a.FileIDs, 1, "Failed to set FileIds")
	})
}

func testPostStoreGetPostsByIDs(t *testing.T, ss store.Store) {
	o1 := &model.Post{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "CCCCCCCCC"
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = o1.ChannelID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "QQQQQQQQQQ"
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	r1, err := ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.NoError(t, err)
	ro1 := r1.Posts[o1.ID]

	r2, err := ss.Post().Get(context.Background(), o2.ID, false, false, false, "")
	require.NoError(t, err)
	ro2 := r2.Posts[o2.ID]

	r3, err := ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.NoError(t, err)
	ro3 := r3.Posts[o3.ID]

	postIDs := []string{
		ro1.ID,
		ro2.ID,
		ro3.ID,
	}

	posts, err := ss.Post().GetPostsByIDs(postIDs)
	require.NoError(t, err)
	require.Len(t, posts, 3, "Expected 3 posts in results. Got %v", len(posts))

	err = ss.Post().Delete(ro1.ID, model.GetMillis(), "")
	require.NoError(t, err)

	posts, err = ss.Post().GetPostsByIDs(postIDs)
	require.NoError(t, err)
	require.Len(t, posts, 3, "Expected 3 posts in results. Got %v", len(posts))
}

func testPostStoreGetPostsBatchForIndexing(t *testing.T, ss store.Store) {
	c1 := &model.Channel{}
	c1.TeamID = model.NewID()
	c1.DisplayName = "Channel1"
	c1.Name = "zz" + model.NewID() + "b"
	c1.Type = model.ChannelTypeOpen
	c1, _ = ss.Channel().Save(c1, -1)

	c2 := &model.Channel{}
	c2.TeamID = model.NewID()
	c2.DisplayName = "Channel2"
	c2.Name = "zz" + model.NewID() + "b"
	c2.Type = model.ChannelTypeOpen
	c2, _ = ss.Channel().Save(c2, -1)

	o1 := &model.Post{}
	o1.ChannelID = c1.ID
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	o1, err := ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = c2.ID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "CCCCCCCCC"
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = c1.ID
	o3.UserID = model.NewID()
	o3.ParentID = o1.ID
	o3.RootID = o1.ID
	o3.Message = "zz" + model.NewID() + "QQQQQQQQQQ"
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	r, err := ss.Post().GetPostsBatchForIndexing(o1.CreateAt, model.GetMillis()+100000, 100)
	require.NoError(t, err)
	require.Len(t, r, 3, "Expected 3 posts in results. Got %v", len(r))
	for _, p := range r {
		if p.ID == o1.ID {
			require.Equal(t, p.TeamID, c1.TeamID, "Unexpected team ID")
			require.Nil(t, p.ParentCreateAt, "Unexpected parent create at")
		} else if p.ID == o2.ID {
			require.Equal(t, p.TeamID, c2.TeamID, "Unexpected team ID")
			require.Nil(t, p.ParentCreateAt, "Unexpected parent create at")
		} else if p.ID == o3.ID {
			require.Equal(t, p.TeamID, c1.TeamID, "Unexpected team ID")
			require.Equal(t, *p.ParentCreateAt, o1.CreateAt, "Unexpected parent create at")
		} else {
			require.Fail(t, "unexpected post returned")
		}
	}
}

func testPostStorePermanentDeleteBatch(t *testing.T, ss store.Store) {
	team, err := ss.Team().Save(&model.Team{
		DisplayName: "DisplayName",
		Name:        "team" + model.NewID(),
		Email:       MakeEmail(),
		Type:        model.TeamOpen,
	})
	require.NoError(t, err)
	channel, err := ss.Channel().Save(&model.Channel{
		TeamID:      team.ID,
		DisplayName: "DisplayName",
		Name:        "channel" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}, -1)
	require.NoError(t, err)

	o1 := &model.Post{}
	o1.ChannelID = channel.ID
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	o1.CreateAt = 1000
	o1, err = ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = channel.ID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	o2.CreateAt = 1000
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	o3 := &model.Post{}
	o3.ChannelID = channel.ID
	o3.UserID = model.NewID()
	o3.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	o3.CreateAt = 100000
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	_, _, err = ss.Post().PermanentDeleteBatchForRetentionPolicies(0, 2000, 1000, model.RetentionPolicyCursor{})
	require.NoError(t, err)

	_, err = ss.Post().Get(context.Background(), o1.ID, false, false, false, "")
	require.Error(t, err, "Should have not found post 1 after purge")

	_, err = ss.Post().Get(context.Background(), o2.ID, false, false, false, "")
	require.Error(t, err, "Should have not found post 2 after purge")

	_, err = ss.Post().Get(context.Background(), o3.ID, false, false, false, "")
	require.NoError(t, err, "Should have found post 3 after purge")

	t.Run("with pagination", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			_, err = ss.Post().Save(&model.Post{
				ChannelID: channel.ID,
				UserID:    model.NewID(),
				Message:   "message",
				CreateAt:  1,
			})
			require.NoError(t, err)
		}
		cursor := model.RetentionPolicyCursor{}

		deleted, cursor, err := ss.Post().PermanentDeleteBatchForRetentionPolicies(0, 2, 2, cursor)
		require.NoError(t, err)
		require.Equal(t, int64(2), deleted)

		deleted, _, err = ss.Post().PermanentDeleteBatchForRetentionPolicies(0, 2, 2, cursor)
		require.NoError(t, err)
		require.Equal(t, int64(1), deleted)
	})

	t.Run("with data retention policies", func(t *testing.T) {
		channelPolicy, err2 := ss.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
			RetentionPolicy: model.RetentionPolicy{
				DisplayName:  "DisplayName",
				PostDuration: model.NewInt64(30),
			},
			ChannelIDs: []string{channel.ID},
		})
		require.NoError(t, err2)
		post := &model.Post{
			ChannelID: channel.ID,
			UserID:    model.NewID(),
			Message:   "message",
			CreateAt:  1,
		}
		post, err2 = ss.Post().Save(post)
		require.NoError(t, err2)

		_, _, err2 = ss.Post().PermanentDeleteBatchForRetentionPolicies(0, 2000, 1000, model.RetentionPolicyCursor{})
		require.NoError(t, err2)
		_, err2 = ss.Post().Get(context.Background(), post.ID, false, false, false, "")
		require.NoError(t, err2, "global policy should have been ignored due to granular policy")

		nowMillis := post.CreateAt + *channelPolicy.PostDuration*24*60*60*1000 + 1
		_, _, err2 = ss.Post().PermanentDeleteBatchForRetentionPolicies(nowMillis, 0, 1000, model.RetentionPolicyCursor{})
		require.NoError(t, err2)
		_, err2 = ss.Post().Get(context.Background(), post.ID, false, false, false, "")
		require.Error(t, err2, "post should have been deleted by channel policy")

		// Create a team policy which is stricter than the channel policy
		teamPolicy, err2 := ss.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
			RetentionPolicy: model.RetentionPolicy{
				DisplayName:  "DisplayName",
				PostDuration: model.NewInt64(20),
			},
			TeamIDs: []string{team.ID},
		})
		require.NoError(t, err2)
		post.ID = ""
		post, err2 = ss.Post().Save(post)
		require.NoError(t, err2)

		nowMillis = post.CreateAt + *teamPolicy.PostDuration*24*60*60*1000 + 1
		_, _, err2 = ss.Post().PermanentDeleteBatchForRetentionPolicies(nowMillis, 0, 1000, model.RetentionPolicyCursor{})
		require.NoError(t, err2)
		_, err2 = ss.Post().Get(context.Background(), post.ID, false, false, false, "")
		require.NoError(t, err2, "channel policy should have overridden team policy")

		// Delete channel policy and re-run team policy
		err2 = ss.RetentionPolicy().RemoveChannels(channelPolicy.ID, []string{channel.ID})
		require.NoError(t, err2)

		err2 = ss.RetentionPolicy().Delete(channelPolicy.ID)
		require.NoError(t, err2)

		_, _, err2 = ss.Post().PermanentDeleteBatchForRetentionPolicies(nowMillis, 0, 1000, model.RetentionPolicyCursor{})
		require.NoError(t, err2)
		_, err2 = ss.Post().Get(context.Background(), post.ID, false, false, false, "")
		require.Error(t, err2, "post should have been deleted by team policy")

		err2 = ss.RetentionPolicy().RemoveTeams(teamPolicy.ID, []string{team.ID})
		require.NoError(t, err2)

		err2 = ss.RetentionPolicy().Delete(teamPolicy.ID)
		require.NoError(t, err2)
	})

	t.Run("with channel, team and global policies", func(t *testing.T) {
		c1 := &model.Channel{}
		c1.TeamID = model.NewID()
		c1.DisplayName = "Channel1"
		c1.Name = "zz" + model.NewID() + "b"
		c1.Type = model.ChannelTypeOpen
		c1, _ = ss.Channel().Save(c1, -1)

		c2 := &model.Channel{}
		c2.TeamID = model.NewID()
		c2.DisplayName = "Channel2"
		c2.Name = "zz" + model.NewID() + "b"
		c2.Type = model.ChannelTypeOpen
		c2, _ = ss.Channel().Save(c2, -1)

		channelPolicy, err2 := ss.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
			RetentionPolicy: model.RetentionPolicy{
				DisplayName:  "DisplayName",
				PostDuration: model.NewInt64(30),
			},
			ChannelIDs: []string{c1.ID},
		})
		require.NoError(t, err2)
		defer ss.RetentionPolicy().Delete(channelPolicy.ID)
		teamPolicy, err2 := ss.RetentionPolicy().Save(&model.RetentionPolicyWithTeamAndChannelIDs{
			RetentionPolicy: model.RetentionPolicy{
				DisplayName:  "DisplayName",
				PostDuration: model.NewInt64(30),
			},
			TeamIDs: []string{team.ID},
		})
		require.NoError(t, err2)
		defer ss.RetentionPolicy().Delete(teamPolicy.ID)

		// This one should be deleted by the channel policy
		_, err2 = ss.Post().Save(&model.Post{
			ChannelID: c1.ID,
			UserID:    model.NewID(),
			Message:   "message",
			CreateAt:  1,
		})
		require.NoError(t, err2)
		// This one, by the team policy
		_, err2 = ss.Post().Save(&model.Post{
			ChannelID: channel.ID,
			UserID:    model.NewID(),
			Message:   "message",
			CreateAt:  1,
		})
		require.NoError(t, err2)
		// This one, by the global policy
		_, err2 = ss.Post().Save(&model.Post{
			ChannelID: c2.ID,
			UserID:    model.NewID(),
			Message:   "message",
			CreateAt:  1,
		})
		require.NoError(t, err2)

		nowMillis := int64(1 + 30*24*60*60*1000 + 1)
		deleted, _, err2 := ss.Post().PermanentDeleteBatchForRetentionPolicies(nowMillis, 2, 1000, model.RetentionPolicyCursor{})
		require.NoError(t, err2)
		require.Equal(t, int64(3), deleted)
	})
}

func testPostStoreGetOldest(t *testing.T, ss store.Store) {
	o0 := &model.Post{}
	o0.ChannelID = model.NewID()
	o0.UserID = model.NewID()
	o0.Message = "zz" + model.NewID() + "b"
	o0.CreateAt = 3
	o0, err := ss.Post().Save(o0)
	require.NoError(t, err)

	o1 := &model.Post{}
	o1.ChannelID = o0.ID
	o1.UserID = model.NewID()
	o1.Message = "zz" + model.NewID() + "b"
	o1.CreateAt = 2
	o1, err = ss.Post().Save(o1)
	require.NoError(t, err)

	o2 := &model.Post{}
	o2.ChannelID = o1.ChannelID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "b"
	o2.CreateAt = 1
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	r1, err := ss.Post().GetOldest()

	require.NoError(t, err)
	assert.EqualValues(t, o2.ID, r1.ID)
}

func testGetMaxPostSize(t *testing.T, ss store.Store) {
	assert.Equal(t, model.PostMessageMaxRunesV2, ss.Post().GetMaxPostSize())
	assert.Equal(t, model.PostMessageMaxRunesV2, ss.Post().GetMaxPostSize())
}

func testPostStoreGetParentsForExportAfter(t *testing.T, ss store.Store) {
	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewID()
	t1.Email = MakeEmail()
	t1.Type = model.TeamOpen
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	c1 := model.Channel{}
	c1.TeamID = t1.ID
	c1.DisplayName = "Channel1"
	c1.Name = "zz" + model.NewID() + "b"
	c1.Type = model.ChannelTypeOpen
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	u1 := model.User{}
	u1.Username = model.NewID()
	u1.Email = MakeEmail()
	u1.Nickname = model.NewID()
	_, err = ss.User().Save(&u1)
	require.NoError(t, err)

	p1 := &model.Post{}
	p1.ChannelID = c1.ID
	p1.UserID = u1.ID
	p1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	p1.CreateAt = 1000
	p1, nErr = ss.Post().Save(p1)
	require.NoError(t, nErr)

	posts, err := ss.Post().GetParentsForExportAfter(10000, strings.Repeat("0", 26))
	assert.NoError(t, err)

	found := false
	for _, p := range posts {
		if p.ID == p1.ID {
			found = true
			assert.Equal(t, p.ID, p1.ID)
			assert.Equal(t, p.Message, p1.Message)
			assert.Equal(t, p.Username, u1.Username)
			assert.Equal(t, p.TeamName, t1.Name)
			assert.Equal(t, p.ChannelName, c1.Name)
		}
	}
	assert.True(t, found)
}

func testPostStoreGetRepliesForExport(t *testing.T, ss store.Store) {
	t1 := model.Team{}
	t1.DisplayName = "Name"
	t1.Name = "zz" + model.NewID()
	t1.Email = MakeEmail()
	t1.Type = model.TeamOpen
	_, err := ss.Team().Save(&t1)
	require.NoError(t, err)

	c1 := model.Channel{}
	c1.TeamID = t1.ID
	c1.DisplayName = "Channel1"
	c1.Name = "zz" + model.NewID() + "b"
	c1.Type = model.ChannelTypeOpen
	_, nErr := ss.Channel().Save(&c1, -1)
	require.NoError(t, nErr)

	u1 := model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewID()
	_, err = ss.User().Save(&u1)
	require.NoError(t, err)

	p1 := &model.Post{}
	p1.ChannelID = c1.ID
	p1.UserID = u1.ID
	p1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	p1.CreateAt = 1000
	p1, nErr = ss.Post().Save(p1)
	require.NoError(t, nErr)

	p2 := &model.Post{}
	p2.ChannelID = c1.ID
	p2.UserID = u1.ID
	p2.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	p2.CreateAt = 1001
	p2.ParentID = p1.ID
	p2.RootID = p1.ID
	p2, nErr = ss.Post().Save(p2)
	require.NoError(t, nErr)

	r1, err := ss.Post().GetRepliesForExport(p1.ID)
	assert.NoError(t, err)

	assert.Len(t, r1, 1)

	reply1 := r1[0]
	assert.Equal(t, reply1.ID, p2.ID)
	assert.Equal(t, reply1.Message, p2.Message)
	assert.Equal(t, reply1.Username, u1.Username)

	// Checking whether replies by deleted user are exported
	u1.DeleteAt = 1002
	_, err = ss.User().Update(&u1, false)
	require.NoError(t, err)

	r1, err = ss.Post().GetRepliesForExport(p1.ID)
	assert.NoError(t, err)

	assert.Len(t, r1, 1)

	reply1 = r1[0]
	assert.Equal(t, reply1.ID, p2.ID)
	assert.Equal(t, reply1.Message, p2.Message)
	assert.Equal(t, reply1.Username, u1.Username)

}

func testPostStoreGetDirectPostParentsForExportAfter(t *testing.T, ss store.Store, s SqlStore) {
	teamID := model.NewID()

	o1 := model.Channel{}
	o1.TeamID = teamID
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewID() + "b"
	o1.Type = model.ChannelTypeDirect

	u1 := &model.User{}
	u1.Email = MakeEmail()
	u1.Nickname = model.NewID()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{}
	u2.Email = MakeEmail()
	u2.Nickname = model.NewID()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelID = o1.ID
	m1.UserID = u1.ID
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelID = o1.ID
	m2.UserID = u2.ID
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	ss.Channel().SaveDirectChannel(&o1, &m1, &m2)

	p1 := &model.Post{}
	p1.ChannelID = o1.ID
	p1.UserID = u1.ID
	p1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
	p1.CreateAt = 1000
	p1, nErr = ss.Post().Save(p1)
	require.NoError(t, nErr)

	r1, nErr := ss.Post().GetDirectPostParentsForExportAfter(10000, strings.Repeat("0", 26))
	assert.NoError(t, nErr)

	assert.Equal(t, p1.Message, r1[0].Message)

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testPostStoreGetDirectPostParentsForExportAfterDeleted(t *testing.T, ss store.Store, s SqlStore) {
	teamID := model.NewID()

	o1 := model.Channel{}
	o1.TeamID = teamID
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewID() + "b"
	o1.Type = model.ChannelTypeDirect

	u1 := &model.User{}
	u1.DeleteAt = 1
	u1.Email = MakeEmail()
	u1.Nickname = model.NewID()
	_, err := ss.User().Save(u1)
	require.NoError(t, err)
	_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
	require.NoError(t, nErr)

	u2 := &model.User{}
	u2.DeleteAt = 1
	u2.Email = MakeEmail()
	u2.Nickname = model.NewID()
	_, err = ss.User().Save(u2)
	require.NoError(t, err)
	_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u2.ID}, -1)
	require.NoError(t, nErr)

	m1 := model.ChannelMember{}
	m1.ChannelID = o1.ID
	m1.UserID = u1.ID
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := model.ChannelMember{}
	m2.ChannelID = o1.ID
	m2.UserID = u2.ID
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	ss.Channel().SaveDirectChannel(&o1, &m1, &m2)

	o1.DeleteAt = 1
	nErr = ss.Channel().SetDeleteAt(o1.ID, 1, 1)
	assert.NoError(t, nErr)

	p1 := &model.Post{}
	p1.ChannelID = o1.ID
	p1.UserID = u1.ID
	p1.Message = "zz" + model.NewID() + "BBBBBBBBBBBB"
	p1.CreateAt = 1000
	p1, nErr = ss.Post().Save(p1)
	require.NoError(t, nErr)

	o1a := p1.Clone()
	o1a.DeleteAt = 1
	o1a.Message = p1.Message + "BBBBBBBBBB"
	_, nErr = ss.Post().Update(o1a, p1)
	require.NoError(t, nErr)

	r1, nErr := ss.Post().GetDirectPostParentsForExportAfter(10000, strings.Repeat("0", 26))
	assert.NoError(t, nErr)

	assert.Equal(t, 0, len(r1))

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testPostStoreGetDirectPostParentsForExportAfterBatched(t *testing.T, ss store.Store, s SqlStore) {
	teamID := model.NewID()

	o1 := model.Channel{}
	o1.TeamID = teamID
	o1.DisplayName = "Name"
	o1.Name = "zz" + model.NewID() + "b"
	o1.Type = model.ChannelTypeDirect

	var postIDs []string
	for i := 0; i < 150; i++ {
		u1 := &model.User{}
		u1.Email = MakeEmail()
		u1.Nickname = model.NewID()
		_, err := ss.User().Save(u1)
		require.NoError(t, err)
		_, nErr := ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u1.ID}, -1)
		require.NoError(t, nErr)

		u2 := &model.User{}
		u2.Email = MakeEmail()
		u2.Nickname = model.NewID()
		_, err = ss.User().Save(u2)
		require.NoError(t, err)
		_, nErr = ss.Team().SaveMember(&model.TeamMember{TeamID: model.NewID(), UserID: u2.ID}, -1)
		require.NoError(t, nErr)

		m1 := model.ChannelMember{}
		m1.ChannelID = o1.ID
		m1.UserID = u1.ID
		m1.NotifyProps = model.GetDefaultChannelNotifyProps()

		m2 := model.ChannelMember{}
		m2.ChannelID = o1.ID
		m2.UserID = u2.ID
		m2.NotifyProps = model.GetDefaultChannelNotifyProps()

		ss.Channel().SaveDirectChannel(&o1, &m1, &m2)

		p1 := &model.Post{}
		p1.ChannelID = o1.ID
		p1.UserID = u1.ID
		p1.Message = "zz" + model.NewID() + "AAAAAAAAAAA"
		p1.CreateAt = 1000
		p1, nErr = ss.Post().Save(p1)
		require.NoError(t, nErr)
		postIDs = append(postIDs, p1.ID)
	}
	sort.Slice(postIDs, func(i, j int) bool { return postIDs[i] < postIDs[j] })

	// Get all posts
	r1, err := ss.Post().GetDirectPostParentsForExportAfter(10000, strings.Repeat("0", 26))
	assert.NoError(t, err)
	assert.Equal(t, len(postIDs), len(r1))
	var exportedPostIDs []string
	for i := range r1 {
		exportedPostIDs = append(exportedPostIDs, r1[i].ID)
	}
	sort.Slice(exportedPostIDs, func(i, j int) bool { return exportedPostIDs[i] < exportedPostIDs[j] })
	assert.ElementsMatch(t, postIDs, exportedPostIDs)

	// Get 100
	r1, err = ss.Post().GetDirectPostParentsForExportAfter(100, strings.Repeat("0", 26))
	assert.NoError(t, err)
	assert.Equal(t, 100, len(r1))
	exportedPostIDs = []string{}
	for i := range r1 {
		exportedPostIDs = append(exportedPostIDs, r1[i].ID)
	}
	sort.Slice(exportedPostIDs, func(i, j int) bool { return exportedPostIDs[i] < exportedPostIDs[j] })
	assert.ElementsMatch(t, postIDs[:100], exportedPostIDs)

	// Manually truncate Channels table until testlib can handle cleanups
	s.GetMaster().Exec("TRUNCATE Channels")
}

func testHasAutoResponsePostByUserSince(t *testing.T, ss store.Store) {
	t.Run("should return posts created after the given time", func(t *testing.T) {
		channelID := model.NewID()
		userID := model.NewID()

		_, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post2, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "message",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		post3, err := ss.Post().Save(&model.Post{
			ChannelID: channelID,
			UserID:    userID,
			Message:   "auto response message",
			Type:      model.PostTypeAutoResponder,
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)

		exists, err := ss.Post().HasAutoResponsePostByUserSince(model.GetPostsSinceOptions{ChannelID: channelID, Time: post2.CreateAt}, userID)
		require.NoError(t, err)
		assert.True(t, exists)

		err = ss.Post().Delete(post3.ID, time.Now().Unix(), userID)
		require.NoError(t, err)

		exists, err = ss.Post().HasAutoResponsePostByUserSince(model.GetPostsSinceOptions{ChannelID: channelID, Time: post2.CreateAt}, userID)
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func testGetPostsSinceForSync(t *testing.T, ss store.Store, s SqlStore) {
	// create some posts.
	channelID := model.NewID()
	remoteID := model.NewString(model.NewID())
	first := model.GetMillis()

	data := []*model.Post{
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 0"},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 1"},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 2"},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 3", RemoteID: remoteID},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 4", RemoteID: remoteID},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 5", RemoteID: remoteID},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 6", RemoteID: remoteID},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 7"},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 8", DeleteAt: model.GetMillis()},
		{ID: model.NewID(), ChannelID: channelID, UserID: model.NewID(), Message: "test post 9", DeleteAt: model.GetMillis()},
	}

	for i, p := range data {
		p.UpdateAt = first + (int64(i) * 300000)
		if p.RemoteID == nil {
			p.RemoteID = model.NewString(model.NewID())
		}
		_, err := ss.Post().Save(p)
		require.NoError(t, err, "couldn't save post")
	}

	t.Run("Invalid channel id", func(t *testing.T) {
		opt := model.GetPostsSinceForSyncOptions{
			ChannelID: model.NewID(),
		}
		cursor := model.GetPostsSinceForSyncCursor{}
		posts, cursorOut, err := ss.Post().GetPostsSinceForSync(opt, cursor, 100)
		require.NoError(t, err)
		require.Empty(t, posts, "should return zero posts")
		require.Equal(t, cursor, cursorOut)
	})

	t.Run("Get by channel, exclude remotes, exclude deleted", func(t *testing.T) {
		opt := model.GetPostsSinceForSyncOptions{
			ChannelID:       channelID,
			ExcludeRemoteID: *remoteID,
		}
		cursor := model.GetPostsSinceForSyncCursor{}
		posts, _, err := ss.Post().GetPostsSinceForSync(opt, cursor, 100)
		require.NoError(t, err)

		require.ElementsMatch(t, getPostIDs(data[0:3], data[7]), getPostIDs(posts))
	})

	t.Run("Include deleted", func(t *testing.T) {
		opt := model.GetPostsSinceForSyncOptions{
			ChannelID:      channelID,
			IncludeDeleted: true,
		}
		cursor := model.GetPostsSinceForSyncCursor{}
		posts, _, err := ss.Post().GetPostsSinceForSync(opt, cursor, 100)
		require.NoError(t, err)

		require.ElementsMatch(t, getPostIDs(data), getPostIDs(posts))
	})

	t.Run("Limit and cursor", func(t *testing.T) {
		opt := model.GetPostsSinceForSyncOptions{
			ChannelID: channelID,
		}
		cursor := model.GetPostsSinceForSyncCursor{}
		posts1, cursor, err := ss.Post().GetPostsSinceForSync(opt, cursor, 5)
		require.NoError(t, err)
		require.Len(t, posts1, 5, "should get 5 posts")

		posts2, _, err := ss.Post().GetPostsSinceForSync(opt, cursor, 5)
		require.NoError(t, err)
		require.Len(t, posts2, 3, "should get 3 posts")

		require.ElementsMatch(t, getPostIDs(data[0:8]), getPostIDs(posts1, posts2...))
	})

	t.Run("UpdateAt collisions", func(t *testing.T) {
		// this test requires all the UpdateAt timestamps to be the same.
		args := map[string]interface{}{"UpdateAt": model.GetMillis()}
		result, err := s.GetMaster().Exec("UPDATE Posts SET UpdateAt = :UpdateAt", args)
		require.NoError(t, err)
		rows, err := result.RowsAffected()
		require.NoError(t, err)
		require.Greater(t, rows, int64(0))

		opt := model.GetPostsSinceForSyncOptions{
			ChannelID: channelID,
		}
		cursor := model.GetPostsSinceForSyncCursor{}
		posts1, cursor, err := ss.Post().GetPostsSinceForSync(opt, cursor, 5)
		require.NoError(t, err)
		require.Len(t, posts1, 5, "should get 5 posts")

		posts2, _, err := ss.Post().GetPostsSinceForSync(opt, cursor, 5)
		require.NoError(t, err)
		require.Len(t, posts2, 3, "should get 3 posts")

		require.ElementsMatch(t, getPostIDs(data[0:8]), getPostIDs(posts1, posts2...))
	})
}

func getPostIDs(posts []*model.Post, morePosts ...*model.Post) []string {
	ids := make([]string, 0, len(posts)+len(morePosts))
	for _, p := range posts {
		ids = append(ids, p.ID)
	}
	for _, p := range morePosts {
		ids = append(ids, p.ID)
	}
	return ids
}
