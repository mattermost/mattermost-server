// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestFileInfoStore(t *testing.T) {
	StoreTest(t, storetest.TestFileInfoStore)
}

func TestFileInfoStoreCache(t *testing.T) {
	fakeFileInfo := model.FileInfo{PostID: "123"}

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		fileInfos, err := cachedStore.FileInfo().GetForPost("123", true, true, true)
		require.NoError(t, err)
		assert.Equal(t, fileInfos, []*model.FileInfo{&fakeFileInfo})
		mockStore.FileInfo().(*mocks.FileInfoStore).AssertNumberOfCalls(t, "GetForPost", 1)
		assert.Equal(t, fileInfos, []*model.FileInfo{&fakeFileInfo})
		cachedStore.FileInfo().GetForPost("123", true, true, true)
		mockStore.FileInfo().(*mocks.FileInfoStore).AssertNumberOfCalls(t, "GetForPost", 1)
	})

	t.Run("first call not cached, second force no cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.FileInfo().GetForPost("123", true, true, true)
		mockStore.FileInfo().(*mocks.FileInfoStore).AssertNumberOfCalls(t, "GetForPost", 1)
		cachedStore.FileInfo().GetForPost("123", true, true, false)
		mockStore.FileInfo().(*mocks.FileInfoStore).AssertNumberOfCalls(t, "GetForPost", 2)
	})

	t.Run("first call not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.FileInfo().GetForPost("123", true, true, true)
		mockStore.FileInfo().(*mocks.FileInfoStore).AssertNumberOfCalls(t, "GetForPost", 1)
		cachedStore.FileInfo().InvalidateFileInfosForPostCache("123", true)
		cachedStore.FileInfo().GetForPost("123", true, true, true)
		mockStore.FileInfo().(*mocks.FileInfoStore).AssertNumberOfCalls(t, "GetForPost", 2)
	})
}
