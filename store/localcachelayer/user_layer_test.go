// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestUserStore(t *testing.T) {
	StoreTestWithSQLStore(t, storetest.TestUserStore)
}

func TestUserStoreCache(t *testing.T) {
	fakeUserIDs := []string{"123"}
	fakeUser := []*model.User{{
		ID:          "123",
		AuthData:    model.NewString("authData"),
		AuthService: "authService",
	}}

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUser, err := cachedStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, true)
		require.NoError(t, err)
		assert.Equal(t, fakeUser, gotUser)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetProfileByIds", 1)

		_, _ = cachedStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, true)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetProfileByIds", 1)
	})

	t.Run("first call not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUser, err := cachedStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, true)
		require.NoError(t, err)
		assert.Equal(t, fakeUser, gotUser)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetProfileByIds", 1)

		_, _ = cachedStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, false)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetProfileByIds", 2)
	})

	t.Run("first call not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUser, err := cachedStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, true)
		require.NoError(t, err)
		assert.Equal(t, fakeUser, gotUser)

		cachedStore.User().InvalidateProfileCacheForUser("123")

		_, _ = cachedStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, true)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetProfileByIds", 2)
	})

	t.Run("should always return a copy of the stored data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		storedUsers, err := mockStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, false)
		require.NoError(t, err)

		originalProps := make([]model.StringMap, len(storedUsers))

		for i := 0; i < len(storedUsers); i++ {
			originalProps[i] = storedUsers[i].NotifyProps
			storedUsers[i].NotifyProps = map[string]string{}
			storedUsers[i].NotifyProps["key"] = "somevalue"
		}

		cachedUsers, err := cachedStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, true)
		require.NoError(t, err)

		for i := 0; i < len(storedUsers); i++ {
			assert.Equal(t, storedUsers[i].ID, cachedUsers[i].ID)
		}

		cachedUsers, err = cachedStore.User().GetProfileByIDs(context.Background(), fakeUserIDs, &store.UserGetByIDsOpts{}, true)
		require.NoError(t, err)
		for i := 0; i < len(storedUsers); i++ {
			storedUsers[i].Props = model.StringMap{}
			storedUsers[i].Timezone = model.StringMap{}
			assert.Equal(t, storedUsers[i], cachedUsers[i])
			if storedUsers[i] == cachedUsers[i] {
				assert.Fail(t, "should be different pointers")
			}
			cachedUsers[i].NotifyProps["key"] = "othervalue"
			assert.NotEqual(t, storedUsers[i], cachedUsers[i])
		}

		for i := 0; i < len(storedUsers); i++ {
			storedUsers[i].NotifyProps = originalProps[i]
		}
	})
}

func TestUserStoreProfilesInChannelCache(t *testing.T) {
	fakeChannelID := "123"
	fakeUserID := "456"
	fakeMap := map[string]*model.User{
		fakeUserID: {ID: "456"},
	}

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotMap, err := cachedStore.User().GetAllProfilesInChannel(context.Background(), fakeChannelID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeMap, gotMap)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetAllProfilesInChannel", 1)

		_, _ = cachedStore.User().GetAllProfilesInChannel(context.Background(), fakeChannelID, true)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetAllProfilesInChannel", 1)
	})

	t.Run("first call not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotMap, err := cachedStore.User().GetAllProfilesInChannel(context.Background(), fakeChannelID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeMap, gotMap)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetAllProfilesInChannel", 1)

		_, _ = cachedStore.User().GetAllProfilesInChannel(context.Background(), fakeChannelID, false)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetAllProfilesInChannel", 2)
	})

	t.Run("first call not cached, invalidate by channel, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotMap, err := cachedStore.User().GetAllProfilesInChannel(context.Background(), fakeChannelID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeMap, gotMap)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetAllProfilesInChannel", 1)

		cachedStore.User().InvalidateProfilesInChannelCache("123")

		_, _ = cachedStore.User().GetAllProfilesInChannel(context.Background(), fakeChannelID, true)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetAllProfilesInChannel", 2)
	})

	t.Run("first call not cached, invalidate by user, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotMap, err := cachedStore.User().GetAllProfilesInChannel(context.Background(), fakeChannelID, true)
		require.NoError(t, err)
		assert.Equal(t, fakeMap, gotMap)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetAllProfilesInChannel", 1)

		cachedStore.User().InvalidateProfilesInChannelCacheByUser("456")

		_, _ = cachedStore.User().GetAllProfilesInChannel(context.Background(), fakeChannelID, true)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetAllProfilesInChannel", 2)
	})
}

func TestUserStoreGetCache(t *testing.T) {
	fakeUserID := "123"
	fakeUser := &model.User{
		ID:          "123",
		AuthData:    model.NewString("authData"),
		AuthService: "authService",
	}
	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUser, err := cachedStore.User().Get(context.Background(), fakeUserID)
		require.NoError(t, err)
		assert.Equal(t, fakeUser, gotUser)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "Get", 1)

		_, _ = cachedStore.User().Get(context.Background(), fakeUserID)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "Get", 1)
	})

	t.Run("first call not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUser, err := cachedStore.User().Get(context.Background(), fakeUserID)
		require.NoError(t, err)
		assert.Equal(t, fakeUser, gotUser)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "Get", 1)

		cachedStore.User().InvalidateProfileCacheForUser("123")

		_, _ = cachedStore.User().Get(context.Background(), fakeUserID)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "Get", 2)
	})

	t.Run("should always return a copy of the stored data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		storedUser, err := mockStore.User().Get(context.Background(), fakeUserID)
		require.NoError(t, err)
		originalProps := storedUser.NotifyProps

		storedUser.NotifyProps = map[string]string{}
		storedUser.NotifyProps["key"] = "somevalue"

		cachedUser, err := cachedStore.User().Get(context.Background(), fakeUserID)
		require.NoError(t, err)
		assert.Equal(t, storedUser, cachedUser)

		storedUser.Props = model.StringMap{}
		storedUser.Timezone = model.StringMap{}
		cachedUser, err = cachedStore.User().Get(context.Background(), fakeUserID)
		require.NoError(t, err)
		assert.Equal(t, storedUser, cachedUser)
		if storedUser == cachedUser {
			assert.Fail(t, "should be different pointers")
		}
		cachedUser.NotifyProps["key"] = "othervalue"
		assert.NotEqual(t, storedUser, cachedUser)

		storedUser.NotifyProps = originalProps
	})
}

func TestUserStoreGetManyCache(t *testing.T) {
	fakeUser := &model.User{
		ID:          "123",
		AuthData:    model.NewString("authData"),
		AuthService: "authService",
	}
	otherFakeUser := &model.User{
		ID:          "456",
		AuthData:    model.NewString("authData"),
		AuthService: "authService",
	}
	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUsers, err := cachedStore.User().GetMany(context.Background(), []string{fakeUser.ID, otherFakeUser.ID})
		require.NoError(t, err)
		assert.Len(t, gotUsers, 2)
		assert.Contains(t, gotUsers, fakeUser)
		assert.Contains(t, gotUsers, otherFakeUser)

		gotUsers, err = cachedStore.User().GetMany(context.Background(), []string{fakeUser.ID, otherFakeUser.ID})
		require.NoError(t, err)
		assert.Len(t, gotUsers, 2)
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetMany", 1)
	})

	t.Run("first call not cached, invalidate one user, and then check that one is cached and one is fetched from db", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		gotUsers, err := cachedStore.User().GetMany(context.Background(), []string{fakeUser.ID, otherFakeUser.ID})
		require.NoError(t, err)
		assert.Len(t, gotUsers, 2)
		assert.Contains(t, gotUsers, fakeUser)
		assert.Contains(t, gotUsers, otherFakeUser)

		cachedStore.User().InvalidateProfileCacheForUser("123")

		gotUsers, err = cachedStore.User().GetMany(context.Background(), []string{fakeUser.ID, otherFakeUser.ID})
		require.NoError(t, err)
		assert.Len(t, gotUsers, 2)
		mockStore.User().(*mocks.UserStore).AssertCalled(t, "GetMany", mock.Anything, []string{"123"})
		mockStore.User().(*mocks.UserStore).AssertNumberOfCalls(t, "GetMany", 2)
	})
}
