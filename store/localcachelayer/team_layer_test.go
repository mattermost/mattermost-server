// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/store/storetest"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestTeamStore(t *testing.T) {
	StoreTest(t, storetest.TestTeamStore)
}

func TestTeamStoreCache(t *testing.T) {
	fakeUserID := "123"
	fakeUserTeamIDs := []string{"1", "2", "3"}

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUserTeamIDs, err := cachedStore.Team().GetUserTeamIDs(fakeUserID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeUserTeamIDs, gotUserTeamIDs)
		mockStore.Team().(*mocks.TeamStore).AssertNumberOfCalls(t, "GetUserTeamIds", 1)

		gotUserTeamIDs, err = cachedStore.Team().GetUserTeamIDs(fakeUserID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeUserTeamIDs, gotUserTeamIDs)
		mockStore.Team().(*mocks.TeamStore).AssertNumberOfCalls(t, "GetUserTeamIds", 1)
	})

	t.Run("first call not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUserTeamIDs, err := cachedStore.Team().GetUserTeamIDs(fakeUserID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeUserTeamIDs, gotUserTeamIDs)
		mockStore.Team().(*mocks.TeamStore).AssertNumberOfCalls(t, "GetUserTeamIds", 1)

		gotUserTeamIDs, err = cachedStore.Team().GetUserTeamIDs(fakeUserID, false)
		require.NoError(t, err)
		assert.Equal(t, fakeUserTeamIDs, gotUserTeamIDs)
		mockStore.Team().(*mocks.TeamStore).AssertNumberOfCalls(t, "GetUserTeamIds", 2)
	})

	t.Run("first call not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUserTeamIDs, err := cachedStore.Team().GetUserTeamIDs(fakeUserID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeUserTeamIDs, gotUserTeamIDs)
		mockStore.Team().(*mocks.TeamStore).AssertNumberOfCalls(t, "GetUserTeamIds", 1)

		cachedStore.Team().InvalidateAllTeamIDsForUser(fakeUserID)

		gotUserTeamIDs, err = cachedStore.Team().GetUserTeamIDs(fakeUserID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeUserTeamIDs, gotUserTeamIDs)
		mockStore.Team().(*mocks.TeamStore).AssertNumberOfCalls(t, "GetUserTeamIds", 2)
	})

}
