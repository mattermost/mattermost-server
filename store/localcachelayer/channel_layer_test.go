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

func TestChannelStore(t *testing.T) {
	StoreTestWithSQLStore(t, storetest.TestReactionStore)
}

func TestChannelStoreChannelMemberCountsCache(t *testing.T) {
	countResult := int64(10)

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		count, err := cachedStore.Channel().GetMemberCount("id", true)
		require.NoError(t, err)
		assert.Equal(t, count, countResult)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 1)
		count, err = cachedStore.Channel().GetMemberCount("id", true)
		require.NoError(t, err)
		assert.Equal(t, count, countResult)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 1)
	})

	t.Run("first call not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetMemberCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 1)
		cachedStore.Channel().GetMemberCount("id", false)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 2)
	})

	t.Run("first call force not cached, second not cached, third cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetMemberCount("id", false)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 1)
		cachedStore.Channel().GetMemberCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 2)
		cachedStore.Channel().GetMemberCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 2)
	})

	t.Run("first call with GetMemberCountFromCache not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		count := cachedStore.Channel().GetMemberCountFromCache("id")
		assert.Equal(t, count, countResult)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 1)
		count = cachedStore.Channel().GetMemberCountFromCache("id")
		assert.Equal(t, count, countResult)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 1)
	})

	t.Run("first call not cached, clear cache, second call not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetMemberCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 1)
		cachedStore.Channel().ClearCaches()
		cachedStore.Channel().GetMemberCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 2)
	})

	t.Run("first call not cached, invalidate cache, second call not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetMemberCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 1)
		cachedStore.Channel().InvalidateMemberCount("id")
		cachedStore.Channel().GetMemberCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetMemberCount", 2)
	})
}

func TestChannelStoreChannelPinnedPostsCountsCache(t *testing.T) {
	countResult := int64(10)

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		count, err := cachedStore.Channel().GetPinnedPostCount("id", true)
		require.NoError(t, err)
		assert.Equal(t, count, countResult)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 1)
		count, err = cachedStore.Channel().GetPinnedPostCount("id", true)
		require.NoError(t, err)
		assert.Equal(t, count, countResult)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 1)
	})

	t.Run("first call not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetPinnedPostCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 1)
		cachedStore.Channel().GetPinnedPostCount("id", false)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 2)
	})

	t.Run("first call force not cached, second not cached, third cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetPinnedPostCount("id", false)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 1)
		cachedStore.Channel().GetPinnedPostCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 2)
		cachedStore.Channel().GetPinnedPostCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 2)
	})

	t.Run("first call not cached, clear cache, second call not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetPinnedPostCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 1)
		cachedStore.Channel().ClearCaches()
		cachedStore.Channel().GetPinnedPostCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 2)
	})

	t.Run("first call not cached, invalidate cache, second call not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetPinnedPostCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 1)
		cachedStore.Channel().InvalidatePinnedPostCount("id")
		cachedStore.Channel().GetPinnedPostCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetPinnedPostCount", 2)
	})
}

func TestChannelStoreGuestCountCache(t *testing.T) {
	countResult := int64(12)

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		count, err := cachedStore.Channel().GetGuestCount("id", true)
		require.NoError(t, err)
		assert.Equal(t, count, countResult)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 1)
		count, err = cachedStore.Channel().GetGuestCount("id", true)
		require.NoError(t, err)
		assert.Equal(t, count, countResult)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 1)
	})

	t.Run("first call not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetGuestCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 1)
		cachedStore.Channel().GetGuestCount("id", false)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 2)
	})

	t.Run("first call force not cached, second not cached, third cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetGuestCount("id", false)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 1)
		cachedStore.Channel().GetGuestCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 2)
		cachedStore.Channel().GetGuestCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 2)
	})

	t.Run("first call not cached, clear cache, second call not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetGuestCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 1)
		cachedStore.Channel().ClearCaches()
		cachedStore.Channel().GetGuestCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 2)
	})

	t.Run("first call not cached, invalidate cache, second call not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().GetGuestCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 1)
		cachedStore.Channel().InvalidateGuestCount("id")
		cachedStore.Channel().GetGuestCount("id", true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "GetGuestCount", 2)
	})
}

func TestChannelStoreChannel(t *testing.T) {
	channelID := "channel1"
	fakeChannel := model.Channel{ID: channelID}
	t.Run("first call by id not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		channel, err := cachedStore.Channel().Get(channelID, true)
		require.NoError(t, err)
		assert.Equal(t, channel, &fakeChannel)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 1)
		channel, err = cachedStore.Channel().Get(channelID, true)
		require.NoError(t, err)
		assert.Equal(t, channel, &fakeChannel)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 1)
	})

	t.Run("first call not cached, second force no cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().Get(channelID, true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Channel().Get(channelID, false)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 2)
	})

	t.Run("first call force no cached, second not cached, third cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)
		cachedStore.Channel().Get(channelID, false)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Channel().Get(channelID, true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 2)
		cachedStore.Channel().Get(channelID, true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 2)
	})

	t.Run("first call not cached, clear cache, second call not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Channel().Get(channelID, true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Channel().ClearCaches()
		cachedStore.Channel().Get(channelID, true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 2)
	})

	t.Run("first call not cached, invalidate cache, second call not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)
		cachedStore.Channel().Get(channelID, true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 1)
		cachedStore.Channel().InvalidateChannel(channelID)
		cachedStore.Channel().Get(channelID, true)
		mockStore.Channel().(*mocks.ChannelStore).AssertNumberOfCalls(t, "Get", 2)
	})
}
