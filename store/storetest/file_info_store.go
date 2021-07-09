// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileInfoStore(t *testing.T, ss store.Store) {
	t.Run("FileInfoSaveGet", func(t *testing.T) { testFileInfoSaveGet(t, ss) })
	t.Run("FileInfoSaveGetByPath", func(t *testing.T) { testFileInfoSaveGetByPath(t, ss) })
	t.Run("FileInfoGetForPost", func(t *testing.T) { testFileInfoGetForPost(t, ss) })
	t.Run("FileInfoGetForUser", func(t *testing.T) { testFileInfoGetForUser(t, ss) })
	t.Run("FileInfoGetWithOptions", func(t *testing.T) { testFileInfoGetWithOptions(t, ss) })
	t.Run("FileInfoAttachToPost", func(t *testing.T) { testFileInfoAttachToPost(t, ss) })
	t.Run("FileInfoDeleteForPost", func(t *testing.T) { testFileInfoDeleteForPost(t, ss) })
	t.Run("FileInfoPermanentDelete", func(t *testing.T) { testFileInfoPermanentDelete(t, ss) })
	t.Run("FileInfoPermanentDeleteBatch", func(t *testing.T) { testFileInfoPermanentDeleteBatch(t, ss) })
	t.Run("FileInfoPermanentDeleteByUser", func(t *testing.T) { testFileInfoPermanentDeleteByUser(t, ss) })
	t.Run("GetFilesBatchForIndexing", func(t *testing.T) { testFileInfoStoreGetFilesBatchForIndexing(t, ss) })
	t.Run("CountAll", func(t *testing.T) { testFileInfoStoreCountAll(t, ss) })
}

func testFileInfoSaveGet(t *testing.T, ss store.Store) {
	info := &model.FileInfo{
		CreatorID: model.NewID(),
		Path:      "file.txt",
	}

	info, err := ss.FileInfo().Save(info)
	require.NoError(t, err)
	require.NotEqual(t, len(info.ID), 0)

	defer func() {
		ss.FileInfo().PermanentDelete(info.ID)
	}()

	rinfo, err := ss.FileInfo().Get(info.ID)
	require.NoError(t, err)
	require.Equal(t, info.ID, rinfo.ID)

	info2, err := ss.FileInfo().Save(&model.FileInfo{
		CreatorID: model.NewID(),
		Path:      "file.txt",
		DeleteAt:  123,
	})
	require.NoError(t, err)

	_, err = ss.FileInfo().Get(info2.ID)
	assert.Error(t, err)

	defer func() {
		ss.FileInfo().PermanentDelete(info2.ID)
	}()
}

func testFileInfoSaveGetByPath(t *testing.T, ss store.Store) {
	info := &model.FileInfo{
		CreatorID: model.NewID(),
		Path:      fmt.Sprintf("%v/file.txt", model.NewID()),
	}

	info, err := ss.FileInfo().Save(info)
	require.NoError(t, err)
	assert.NotEqual(t, len(info.ID), 0)
	defer func() {
		ss.FileInfo().PermanentDelete(info.ID)
	}()

	rinfo, err := ss.FileInfo().GetByPath(info.Path)
	require.NoError(t, err)
	assert.Equal(t, info.ID, rinfo.ID)

	info2, err := ss.FileInfo().Save(&model.FileInfo{
		CreatorID: model.NewID(),
		Path:      "file.txt",
		DeleteAt:  123,
	})
	require.NoError(t, err)

	_, err = ss.FileInfo().GetByPath(info2.ID)
	assert.Error(t, err)

	defer func() {
		ss.FileInfo().PermanentDelete(info2.ID)
	}()
}

func testFileInfoGetForPost(t *testing.T, ss store.Store) {
	userID := model.NewID()
	postID := model.NewID()

	infos := []*model.FileInfo{
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
		},
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
		},
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
			DeleteAt:  123,
		},
		{
			PostID:    model.NewID(),
			CreatorID: userID,
			Path:      "file.txt",
		},
	}

	for i, info := range infos {
		newInfo, err := ss.FileInfo().Save(info)
		require.NoError(t, err)
		infos[i] = newInfo
		defer func(id string) {
			ss.FileInfo().PermanentDelete(id)
		}(newInfo.ID)
	}

	testCases := []struct {
		Name           string
		PostID         string
		ReadFromMaster bool
		IncludeDeleted bool
		AllowFromCache bool
		ExpectedPosts  int
	}{
		{
			Name:           "Fetch from master, without deleted and without cache",
			PostID:         postID,
			ReadFromMaster: true,
			IncludeDeleted: false,
			AllowFromCache: false,
			ExpectedPosts:  2,
		},
		{
			Name:           "Fetch from master, with deleted and without cache",
			PostID:         postID,
			ReadFromMaster: true,
			IncludeDeleted: true,
			AllowFromCache: false,
			ExpectedPosts:  3,
		},
		{
			Name:           "Fetch from master, with deleted and with cache",
			PostID:         postID,
			ReadFromMaster: true,
			IncludeDeleted: true,
			AllowFromCache: true,
			ExpectedPosts:  3,
		},
		{
			Name:           "Fetch from replica, without deleted and without cache",
			PostID:         postID,
			ReadFromMaster: false,
			IncludeDeleted: false,
			AllowFromCache: false,
			ExpectedPosts:  2,
		},
		{
			Name:           "Fetch from replica, with deleted and without cache",
			PostID:         postID,
			ReadFromMaster: false,
			IncludeDeleted: true,
			AllowFromCache: false,
			ExpectedPosts:  3,
		},
		{
			Name:           "Fetch from replica, with deleted and without cache",
			PostID:         postID,
			ReadFromMaster: false,
			IncludeDeleted: true,
			AllowFromCache: true,
			ExpectedPosts:  3,
		},
		{
			Name:           "Fetch from replica, without deleted and with cache",
			PostID:         postID,
			ReadFromMaster: true,
			IncludeDeleted: false,
			AllowFromCache: true,
			ExpectedPosts:  2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			postInfos, err := ss.FileInfo().GetForPost(
				tc.PostID,
				tc.ReadFromMaster,
				tc.IncludeDeleted,
				tc.AllowFromCache,
			)
			require.NoError(t, err)
			assert.Len(t, postInfos, tc.ExpectedPosts)

		})
	}
}

func testFileInfoGetForUser(t *testing.T, ss store.Store) {
	userID := model.NewID()
	userID2 := model.NewID()
	postID := model.NewID()

	infos := []*model.FileInfo{
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
		},
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
		},
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
		},
		{
			PostID:    model.NewID(),
			CreatorID: userID2,
			Path:      "file.txt",
		},
	}

	for i, info := range infos {
		newInfo, err := ss.FileInfo().Save(info)
		require.NoError(t, err)
		infos[i] = newInfo
		defer func(id string) {
			ss.FileInfo().PermanentDelete(id)
		}(newInfo.ID)
	}

	userPosts, err := ss.FileInfo().GetForUser(userID)
	require.NoError(t, err)
	assert.Len(t, userPosts, 3)

	userPosts, err = ss.FileInfo().GetForUser(userID2)
	require.NoError(t, err)
	assert.Len(t, userPosts, 1)
}

func testFileInfoGetWithOptions(t *testing.T, ss store.Store) {
	makePost := func(chID string, user string) *model.Post {
		post := model.Post{}
		post.ChannelID = chID
		post.UserID = user
		_, err := ss.Post().Save(&post)
		require.NoError(t, err)
		return &post
	}

	makeFile := func(post *model.Post, user string, createAt int64, idPrefix string) model.FileInfo {
		id := model.NewID()
		id = idPrefix + id[1:] // hacky way to get sortable Ids to confirm secondary Id sort works
		fileInfo := model.FileInfo{
			ID:        id,
			CreatorID: user,
			Path:      "file.txt",
			CreateAt:  createAt,
		}
		if post.ID != "" {
			fileInfo.PostID = post.ID
		}
		_, err := ss.FileInfo().Save(&fileInfo)
		require.NoError(t, err)
		return fileInfo
	}

	userID1 := model.NewID()
	userID2 := model.NewID()

	channelID1 := model.NewID()
	channelID2 := model.NewID()
	channelID3 := model.NewID()

	post1_1 := makePost(channelID1, userID1) // post 1 by user 1
	post1_2 := makePost(channelID3, userID1) // post 2 by user 1
	post2_1 := makePost(channelID2, userID2)
	post2_2 := makePost(channelID3, userID2)

	epoch := time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC)
	file1_1 := makeFile(post1_1, userID1, epoch.AddDate(0, 0, 1).Unix(), "a")       // file 1 by user 1
	file1_2 := makeFile(post1_2, userID1, epoch.AddDate(0, 0, 2).Unix(), "b")       // file 2 by user 1
	file1_3 := makeFile(&model.Post{}, userID1, epoch.AddDate(0, 0, 3).Unix(), "c") // file that is not attached to a post
	file2_1 := makeFile(post2_1, userID2, epoch.AddDate(0, 0, 4).Unix(), "d")       // file 2 by user 1
	file2_2 := makeFile(post2_2, userID2, epoch.AddDate(0, 0, 5).Unix(), "e")

	// delete a file
	_, err := ss.FileInfo().DeleteForPost(file2_2.PostID)
	require.NoError(t, err)

	testCases := []struct {
		Name            string
		Page, PerPage   int
		Opt             *model.GetFileInfosOptions
		ExpectedFileIDs []string
	}{
		{
			Name:            "Get files with nil option",
			Page:            0,
			PerPage:         10,
			Opt:             nil,
			ExpectedFileIDs: []string{file1_1.ID, file1_2.ID, file1_3.ID, file2_1.ID},
		},
		{
			Name:            "Get files including deleted",
			Page:            0,
			PerPage:         10,
			Opt:             &model.GetFileInfosOptions{IncludeDeleted: true},
			ExpectedFileIDs: []string{file1_1.ID, file1_2.ID, file1_3.ID, file2_1.ID, file2_2.ID},
		},
		{
			Name:    "Get files including deleted filtered by channel",
			Page:    0,
			PerPage: 10,
			Opt: &model.GetFileInfosOptions{
				IncludeDeleted: true,
				ChannelIDs:     []string{channelID3},
			},
			ExpectedFileIDs: []string{file1_2.ID, file2_2.ID},
		},
		{
			Name:    "Get files including deleted filtered by channel and user",
			Page:    0,
			PerPage: 10,
			Opt: &model.GetFileInfosOptions{
				IncludeDeleted: true,
				UserIDs:        []string{userID1},
				ChannelIDs:     []string{channelID3},
			},
			ExpectedFileIDs: []string{file1_2.ID},
		},
		{
			Name:    "Get files including deleted sorted by created at",
			Page:    0,
			PerPage: 10,
			Opt: &model.GetFileInfosOptions{
				IncludeDeleted: true,
				SortBy:         model.FileinfoSortByCreated,
			},
			ExpectedFileIDs: []string{file1_1.ID, file1_2.ID, file1_3.ID, file2_1.ID, file2_2.ID},
		},
		{
			Name:    "Get files filtered by user ordered by created at descending",
			Page:    0,
			PerPage: 10,
			Opt: &model.GetFileInfosOptions{
				UserIDs:        []string{userID1},
				SortBy:         model.FileinfoSortByCreated,
				SortDescending: true,
			},
			ExpectedFileIDs: []string{file1_3.ID, file1_2.ID, file1_1.ID},
		},
		{
			Name:    "Get all files including deleted ordered by created descending 2nd page of 3 per page ",
			Page:    1,
			PerPage: 3,
			Opt: &model.GetFileInfosOptions{
				IncludeDeleted: true,
				SortBy:         model.FileinfoSortByCreated,
				SortDescending: true,
			},
			ExpectedFileIDs: []string{file1_2.ID, file1_1.ID},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			fileInfos, err := ss.FileInfo().GetWithOptions(tc.Page, tc.PerPage, tc.Opt)
			require.NoError(t, err)
			require.Len(t, fileInfos, len(tc.ExpectedFileIDs))
			for i := range tc.ExpectedFileIDs {
				assert.Equal(t, tc.ExpectedFileIDs[i], fileInfos[i].ID)
			}
		})
	}
}

type byFileInfoID []*model.FileInfo

func (a byFileInfoID) Len() int           { return len(a) }
func (a byFileInfoID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byFileInfoID) Less(i, j int) bool { return a[i].ID < a[j].ID }

func testFileInfoAttachToPost(t *testing.T, ss store.Store) {
	t.Run("should attach files", func(t *testing.T) {
		userID := model.NewID()
		postID := model.NewID()

		info1, err := ss.FileInfo().Save(&model.FileInfo{
			CreatorID: userID,
			Path:      "file.txt",
		})
		require.NoError(t, err)
		info2, err := ss.FileInfo().Save(&model.FileInfo{
			CreatorID: userID,
			Path:      "file2.txt",
		})
		require.NoError(t, err)

		require.Equal(t, "", info1.PostID)
		require.Equal(t, "", info2.PostID)

		err = ss.FileInfo().AttachToPost(info1.ID, postID, userID)
		assert.NoError(t, err)
		info1.PostID = postID

		err = ss.FileInfo().AttachToPost(info2.ID, postID, userID)
		assert.NoError(t, err)
		info2.PostID = postID

		data, err := ss.FileInfo().GetForPost(postID, true, false, false)
		require.NoError(t, err)

		expected := []*model.FileInfo{info1, info2}
		sort.Sort(byFileInfoID(expected))
		sort.Sort(byFileInfoID(data))
		assert.EqualValues(t, expected, data)
	})

	t.Run("should not attach files to multiple posts", func(t *testing.T) {
		userID := model.NewID()
		postID := model.NewID()

		info, err := ss.FileInfo().Save(&model.FileInfo{
			CreatorID: userID,
			Path:      "file.txt",
		})
		require.NoError(t, err)

		require.Equal(t, "", info.PostID)

		err = ss.FileInfo().AttachToPost(info.ID, model.NewID(), userID)
		require.NoError(t, err)

		err = ss.FileInfo().AttachToPost(info.ID, postID, userID)
		require.Error(t, err)
	})

	t.Run("should not attach files owned from a different user", func(t *testing.T) {
		userID := model.NewID()
		postID := model.NewID()

		info, err := ss.FileInfo().Save(&model.FileInfo{
			CreatorID: model.NewID(),
			Path:      "file.txt",
		})
		require.NoError(t, err)

		require.Equal(t, "", info.PostID)

		err = ss.FileInfo().AttachToPost(info.ID, postID, userID)
		assert.Error(t, err)
	})

	t.Run("should attach files uploaded by nouser", func(t *testing.T) {
		postID := model.NewID()

		info, err := ss.FileInfo().Save(&model.FileInfo{
			CreatorID: "nouser",
			Path:      "file.txt",
		})
		require.NoError(t, err)
		assert.Equal(t, "", info.PostID)

		err = ss.FileInfo().AttachToPost(info.ID, postID, model.NewID())
		require.NoError(t, err)

		data, err := ss.FileInfo().GetForPost(postID, true, false, false)
		require.NoError(t, err)
		info.PostID = postID
		assert.EqualValues(t, []*model.FileInfo{info}, data)
	})
}

func testFileInfoDeleteForPost(t *testing.T, ss store.Store) {
	userID := model.NewID()
	postID := model.NewID()

	infos := []*model.FileInfo{
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
		},
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
		},
		{
			PostID:    postID,
			CreatorID: userID,
			Path:      "file.txt",
			DeleteAt:  123,
		},
		{
			PostID:    model.NewID(),
			CreatorID: userID,
			Path:      "file.txt",
		},
	}

	for i, info := range infos {
		newInfo, err := ss.FileInfo().Save(info)
		require.NoError(t, err)
		infos[i] = newInfo
		defer func(id string) {
			ss.FileInfo().PermanentDelete(id)
		}(newInfo.ID)
	}

	_, err := ss.FileInfo().DeleteForPost(postID)
	require.NoError(t, err)

	infos, err = ss.FileInfo().GetForPost(postID, true, false, false)
	require.NoError(t, err)
	assert.Empty(t, infos)
}

func testFileInfoPermanentDelete(t *testing.T, ss store.Store) {
	info, err := ss.FileInfo().Save(&model.FileInfo{
		PostID:    model.NewID(),
		CreatorID: model.NewID(),
		Path:      "file.txt",
	})
	require.NoError(t, err)

	err = ss.FileInfo().PermanentDelete(info.ID)
	require.NoError(t, err)
}

func testFileInfoPermanentDeleteBatch(t *testing.T, ss store.Store) {
	postID := model.NewID()

	_, err := ss.FileInfo().Save(&model.FileInfo{
		PostID:    postID,
		CreatorID: model.NewID(),
		Path:      "file.txt",
		CreateAt:  1000,
	})
	require.NoError(t, err)

	_, err = ss.FileInfo().Save(&model.FileInfo{
		PostID:    postID,
		CreatorID: model.NewID(),
		Path:      "file.txt",
		CreateAt:  1200,
	})
	require.NoError(t, err)

	_, err = ss.FileInfo().Save(&model.FileInfo{
		PostID:    postID,
		CreatorID: model.NewID(),
		Path:      "file.txt",
		CreateAt:  2000,
	})
	require.NoError(t, err)

	postFiles, err := ss.FileInfo().GetForPost(postID, true, false, false)
	require.NoError(t, err)
	assert.Len(t, postFiles, 3)

	_, err = ss.FileInfo().PermanentDeleteBatch(1500, 1000)
	require.NoError(t, err)

	postFiles, err = ss.FileInfo().GetForPost(postID, true, false, false)
	require.NoError(t, err)
	assert.Len(t, postFiles, 1)
}

func testFileInfoPermanentDeleteByUser(t *testing.T, ss store.Store) {
	userID := model.NewID()
	postID := model.NewID()

	_, err := ss.FileInfo().Save(&model.FileInfo{
		PostID:    postID,
		CreatorID: userID,
		Path:      "file.txt",
	})
	require.NoError(t, err)

	_, err = ss.FileInfo().PermanentDeleteByUser(userID)
	require.NoError(t, err)
}

func testFileInfoStoreGetFilesBatchForIndexing(t *testing.T, ss store.Store) {
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
	f1, err := ss.FileInfo().Save(&model.FileInfo{
		PostID:    o1.ID,
		CreatorID: model.NewID(),
		Path:      "file1.txt",
	})
	require.NoError(t, err)
	defer func() {
		ss.FileInfo().PermanentDelete(f1.ID)
	}()
	time.Sleep(1 * time.Millisecond)

	o2 := &model.Post{}
	o2.ChannelID = c2.ID
	o2.UserID = model.NewID()
	o2.Message = "zz" + model.NewID() + "CCCCCCCCC"
	o2, err = ss.Post().Save(o2)
	require.NoError(t, err)

	f2, err := ss.FileInfo().Save(&model.FileInfo{
		PostID:    o2.ID,
		CreatorID: model.NewID(),
		Path:      "file2.txt",
	})
	require.NoError(t, err)
	defer func() {
		ss.FileInfo().PermanentDelete(f2.ID)
	}()
	time.Sleep(1 * time.Millisecond)

	o3 := &model.Post{}
	o3.ChannelID = c1.ID
	o3.UserID = model.NewID()
	o3.ParentID = o1.ID
	o3.RootID = o1.ID
	o3.Message = "zz" + model.NewID() + "QQQQQQQQQQ"
	o3, err = ss.Post().Save(o3)
	require.NoError(t, err)

	f3, err := ss.FileInfo().Save(&model.FileInfo{
		PostID:    o3.ID,
		CreatorID: model.NewID(),
		Path:      "file3.txt",
	})
	require.NoError(t, err)
	defer func() {
		ss.FileInfo().PermanentDelete(f3.ID)
	}()

	t.Run("get all files", func(t *testing.T) {
		r, err := ss.FileInfo().GetFilesBatchForIndexing(f1.CreateAt, model.GetMillis()+100000, 100)
		require.NoError(t, err)
		require.Len(t, r, 3, "Expected 3 posts in results. Got %v", len(r))
		for _, f := range r {
			if f.ID == f1.ID {
				require.Equal(t, f.ChannelID, o1.ChannelID, "Unexpected channel ID")
				require.Equal(t, f.Path, "file1.txt", "Unexpected filename")
			} else if f.ID == f2.ID {
				require.Equal(t, f.ChannelID, o2.ChannelID, "Unexpected channel ID")
				require.Equal(t, f.Path, "file2.txt", "Unexpected filename")
			} else if f.ID == f3.ID {
				require.Equal(t, f.ChannelID, o3.ChannelID, "Unexpected channel ID")
				require.Equal(t, f.Path, "file3.txt", "Unexpected filename")
			} else {
				require.Fail(t, "unexpected file returned")
			}
		}
	})

	t.Run("get files after certain date", func(t *testing.T) {
		r, err := ss.FileInfo().GetFilesBatchForIndexing(f1.CreateAt+1, model.GetMillis()+100000, 100)
		require.NoError(t, err)
		require.Len(t, r, 2, "Expected 2 posts in results. Got %v", len(r))
		for _, f := range r {
			if f.ID == f2.ID {
				require.Equal(t, f.ChannelID, o2.ChannelID, "Unexpected channel ID")
				require.Equal(t, f.Path, "file2.txt", "Unexpected filename")
			} else if f.ID == f3.ID {
				require.Equal(t, f.ChannelID, o3.ChannelID, "Unexpected channel ID")
				require.Equal(t, f.Path, "file3.txt", "Unexpected filename")
			} else {
				require.Fail(t, "unexpected file returned")
			}
		}
	})
}

func testFileInfoStoreCountAll(t *testing.T, ss store.Store) {
	_, err := ss.FileInfo().PermanentDeleteBatch(model.GetMillis(), 100000)
	require.NoError(t, err)
	f1, err := ss.FileInfo().Save(&model.FileInfo{
		PostID:    model.NewID(),
		CreatorID: model.NewID(),
		Path:      "file1.txt",
	})
	require.NoError(t, err)

	_, err = ss.FileInfo().Save(&model.FileInfo{
		PostID:    model.NewID(),
		CreatorID: model.NewID(),
		Path:      "file2.txt",
	})
	require.NoError(t, err)
	_, err = ss.FileInfo().Save(&model.FileInfo{
		PostID:    model.NewID(),
		CreatorID: model.NewID(),
		Path:      "file3.txt",
	})
	require.NoError(t, err)

	count, err := ss.FileInfo().CountAll()
	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	_, err = ss.FileInfo().DeleteForPost(f1.PostID)
	require.NoError(t, err)
	count, err = ss.FileInfo().CountAll()
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}
