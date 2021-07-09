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

func TestRoleStore(t *testing.T) {
	StoreTestWithSQLStore(t, storetest.TestRoleStore)
}

func TestRoleStoreCache(t *testing.T) {
	fakeRole := model.Role{ID: "123", Name: "role-name"}

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		role, err := cachedStore.Role().GetByName(context.Background(), "role-name")
		require.NoError(t, err)
		assert.Equal(t, role, &fakeRole)
		mockStore.Role().(*mocks.RoleStore).AssertNumberOfCalls(t, "GetByName", 1)
		require.NoError(t, err)
		assert.Equal(t, role, &fakeRole)
		cachedStore.Role().GetByName(context.Background(), "role-name")
		mockStore.Role().(*mocks.RoleStore).AssertNumberOfCalls(t, "GetByName", 1)
	})

	t.Run("first call not cached, save, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Role().GetByName(context.Background(), "role-name")
		mockStore.Role().(*mocks.RoleStore).AssertNumberOfCalls(t, "GetByName", 1)
		cachedStore.Role().Save(&fakeRole)
		cachedStore.Role().GetByName(context.Background(), "role-name")
		mockStore.Role().(*mocks.RoleStore).AssertNumberOfCalls(t, "GetByName", 2)
	})

	t.Run("first call not cached, delete, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Role().GetByName(context.Background(), "role-name")
		mockStore.Role().(*mocks.RoleStore).AssertNumberOfCalls(t, "GetByName", 1)
		cachedStore.Role().Delete("123")
		cachedStore.Role().GetByName(context.Background(), "role-name")
		mockStore.Role().(*mocks.RoleStore).AssertNumberOfCalls(t, "GetByName", 2)
	})

	t.Run("first call not cached, permanent delete all, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Role().GetByName(context.Background(), "role-name")
		mockStore.Role().(*mocks.RoleStore).AssertNumberOfCalls(t, "GetByName", 1)
		cachedStore.Role().PermanentDeleteAll()
		cachedStore.Role().GetByName(context.Background(), "role-name")
		mockStore.Role().(*mocks.RoleStore).AssertNumberOfCalls(t, "GetByName", 2)
	})
}
