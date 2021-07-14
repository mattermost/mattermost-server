// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestEmojiStore(t *testing.T) {
	StoreTest(t, storetest.TestEmojiStore)
}

func TestEmojiStoreCache(t *testing.T) {
	fakeEmoji := model.Emoji{Id: "123", Name: "name123"}
	ctxEmoji := model.Emoji{Id: "master", Name: "name123"}

	t.Run("first call by id not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		emoji, err := cachedStore.Emoji().Get(context.Background(), "123", true)
		require.NoError(t, err)
		assert.Equal(t, emoji, &fakeEmoji)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 1)
		emoji, err = cachedStore.Emoji().Get(context.Background(), "123", true)
		require.NoError(t, err)
		assert.Equal(t, emoji, &fakeEmoji)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 1)
	})

	t.Run("first call by name not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		emoji, err := cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		require.NoError(t, err)
		assert.Equal(t, emoji, &fakeEmoji)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 1)
		emoji, err = cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		require.NoError(t, err)
		assert.Equal(t, emoji, &fakeEmoji)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 1)
	})

	t.Run("first call by id not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().Get(context.Background(), "123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Emoji().Get(context.Background(), "123", false)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 2)
	})

	t.Run("first call by name not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 1)
		cachedStore.Emoji().GetByName(context.Background(), "name123", false)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 2)
	})

	t.Run("first call by id force not cached, second not cached, third cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().Get(context.Background(), "123", false)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Emoji().Get(context.Background(), "123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 2)
		cachedStore.Emoji().Get(context.Background(), "123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 2)
	})

	t.Run("first call by name force not cached, second not cached, third cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().GetByName(context.Background(), "name123", false)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 1)
		cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 2)
		cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 2)
	})

	t.Run("first call by id, second call by name cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().Get(context.Background(), "123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 0)
	})

	t.Run("first call by name, second call by id cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 1)
		cachedStore.Emoji().Get(context.Background(), "123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 0)
	})

	t.Run("first call by id not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().Get(context.Background(), "123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Emoji().Delete(&fakeEmoji, 0)
		cachedStore.Emoji().Get(context.Background(), "123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 2)
	})

	t.Run("call by id, use master", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().Get(context.Background(), "master", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Emoji().Delete(&ctxEmoji, 0)
		cachedStore.Emoji().Get(context.Background(), "master", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "Get", 2)
	})

	t.Run("first call by name not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 1)
		cachedStore.Emoji().Delete(&fakeEmoji, 0)
		cachedStore.Emoji().GetByName(context.Background(), "name123", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 2)
	})

	t.Run("call by name, use master", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().GetByName(context.Background(), "master", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 1)
		cachedStore.Emoji().Delete(&ctxEmoji, 0)
		cachedStore.Emoji().GetByName(context.Background(), "master", true)
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetByName", 2)
	})

	t.Run("call multiple by name", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Emoji().GetMultipleByName([]string{"name123"})
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetMultipleByName", 1)
		mockStore.Emoji().(*mocks.EmojiStore).AssertCalled(t, "GetMultipleByName", []string{"name123"})
		cachedStore.Emoji().GetMultipleByName([]string{"name123"})
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetMultipleByName", 1)
		cachedStore.Emoji().GetMultipleByName([]string{"name123", "name321"})
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetMultipleByName", 2)
		mockStore.Emoji().(*mocks.EmojiStore).AssertCalled(t, "GetMultipleByName", []string{"name321"})
		cachedStore.Emoji().GetMultipleByName([]string{"name321", "name123"})
		mockStore.Emoji().(*mocks.EmojiStore).AssertNumberOfCalls(t, "GetMultipleByName", 2)
	})
}
