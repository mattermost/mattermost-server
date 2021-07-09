// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestPostStore(t *testing.T) {
	StoreTestWithSQLStore(t, storetest.TestPostStore)
}

func TestPostStoreLastPostTimeCache(t *testing.T) {
	var fakeLastTime int64 = 1
	channelID := "channelId"
	fakeOptions := model.GetPostsSinceOptions{
		ChannelID:        channelID,
		Time:             fakeLastTime,
		SkipFetchThreads: false,
	}

	t.Run("GetEtag: first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		expectedResult := fmt.Sprintf("%v.%v", model.CurrentVersion, fakeLastTime)

		etag := cachedStore.Post().GetEtag(channelID, true, false)
		assert.Equal(t, etag, expectedResult)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetEtag", 1)

		etag = cachedStore.Post().GetEtag(channelID, true, false)
		assert.Equal(t, etag, expectedResult)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetEtag", 1)
	})

	t.Run("GetEtag: first call not cached, second force no cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Post().GetEtag(channelID, true, false)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetEtag", 1)
		cachedStore.Post().GetEtag(channelID, false, false)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetEtag", 2)
	})

	t.Run("GetEtag: first call not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Post().GetEtag(channelID, true, false)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetEtag", 1)
		cachedStore.Post().InvalidateLastPostTimeCache(channelID)
		cachedStore.Post().GetEtag(channelID, true, false)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetEtag", 2)
	})

	t.Run("GetEtag: first call not cached, clear caches, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Post().GetEtag(channelID, true, false)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetEtag", 1)
		cachedStore.Post().ClearCaches()
		cachedStore.Post().GetEtag(channelID, true, false)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetEtag", 2)
	})

	t.Run("GetPostsSince: first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		expectedResult := model.NewPostList()

		list, err := cachedStore.Post().GetPostsSince(fakeOptions, true)
		require.NoError(t, err)
		assert.Equal(t, list, expectedResult)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPostsSince", 1)

		list, err = cachedStore.Post().GetPostsSince(fakeOptions, true)
		require.NoError(t, err)
		assert.Equal(t, list, expectedResult)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPostsSince", 1)
	})

	t.Run("GetPostsSince: first call not cached, second force no cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Post().GetPostsSince(fakeOptions, true)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPostsSince", 1)
		cachedStore.Post().GetPostsSince(fakeOptions, false)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPostsSince", 2)
	})

	t.Run("GetPostsSince: first call not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Post().GetPostsSince(fakeOptions, true)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPostsSince", 1)
		cachedStore.Post().InvalidateLastPostTimeCache(channelID)
		cachedStore.Post().GetPostsSince(fakeOptions, true)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPostsSince", 2)
	})

	t.Run("GetPostsSince: first call not cached, clear caches, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Post().GetPostsSince(fakeOptions, true)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPostsSince", 1)
		cachedStore.Post().ClearCaches()
		cachedStore.Post().GetPostsSince(fakeOptions, true)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPostsSince", 2)
	})
}

func TestPostStoreCache(t *testing.T) {
	fakePosts := &model.PostList{}
	fakeOptions := model.GetPostsOptions{ChannelID: "123", PerPage: 30}

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotPosts, err := cachedStore.Post().GetPosts(fakeOptions, true)
		require.NoError(t, err)
		assert.Equal(t, fakePosts, gotPosts)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPosts", 1)

		_, _ = cachedStore.Post().GetPosts(fakeOptions, true)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPosts", 1)
	})

	t.Run("first call not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotPosts, err := cachedStore.Post().GetPosts(fakeOptions, true)
		require.NoError(t, err)
		assert.Equal(t, fakePosts, gotPosts)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPosts", 1)

		_, _ = cachedStore.Post().GetPosts(fakeOptions, false)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPosts", 2)
	})

	t.Run("first call not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotPosts, err := cachedStore.Post().GetPosts(fakeOptions, true)
		require.NoError(t, err)
		assert.Equal(t, fakePosts, gotPosts)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPosts", 1)

		cachedStore.Post().InvalidateLastPostTimeCache("12360")

		_, _ = cachedStore.Post().GetPosts(fakeOptions, true)
		mockStore.Post().(*mocks.PostStore).AssertNumberOfCalls(t, "GetPosts", 1)

	})
}
