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

func TestWebhookStore(t *testing.T) {
	StoreTest(t, storetest.TestWebhookStore)
}

func TestWebhookStoreCache(t *testing.T) {
	fakeWebhook := model.IncomingWebhook{ID: "123"}

	t.Run("first call not cached, second cached and returning same data", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		incomingWebhook, err := cachedStore.Webhook().GetIncoming("123", true)
		require.NoError(t, err)
		assert.Equal(t, incomingWebhook, &fakeWebhook)
		mockStore.Webhook().(*mocks.WebhookStore).AssertNumberOfCalls(t, "GetIncoming", 1)

		assert.Equal(t, incomingWebhook, &fakeWebhook)
		cachedStore.Webhook().GetIncoming("123", true)
		mockStore.Webhook().(*mocks.WebhookStore).AssertNumberOfCalls(t, "GetIncoming", 1)
	})

	t.Run("first call not cached, second force not cached", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Webhook().GetIncoming("123", true)
		mockStore.Webhook().(*mocks.WebhookStore).AssertNumberOfCalls(t, "GetIncoming", 1)
		cachedStore.Webhook().GetIncoming("123", false)
		mockStore.Webhook().(*mocks.WebhookStore).AssertNumberOfCalls(t, "GetIncoming", 2)
	})

	t.Run("first call not cached, invalidate, and then not cached again", func(t *testing.T) {
		mockStore := getMockStore()
		mockCacheProvider := getMockCacheProvider()
		cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, mockCacheProvider)
		require.NoError(t, err)

		cachedStore.Webhook().GetIncoming("123", true)
		mockStore.Webhook().(*mocks.WebhookStore).AssertNumberOfCalls(t, "GetIncoming", 1)
		cachedStore.Webhook().InvalidateWebhookCache("123")
		cachedStore.Webhook().GetIncoming("123", true)
		mockStore.Webhook().(*mocks.WebhookStore).AssertNumberOfCalls(t, "GetIncoming", 2)
	})
}
