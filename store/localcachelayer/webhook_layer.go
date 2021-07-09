// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type LocalCacheWebhookStore struct {
	store.WebhookStore
	rootStore *LocalCacheStore
}

func (s *LocalCacheWebhookStore) handleClusterInvalidateWebhook(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.webhookCache.Purge()
	} else {
		s.rootStore.webhookCache.Remove(msg.Data)
	}
}

func (s LocalCacheWebhookStore) ClearCaches() {
	s.rootStore.doClearCacheCluster(s.rootStore.webhookCache)

	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Webhook - Purge")
	}
}

func (s LocalCacheWebhookStore) InvalidateWebhookCache(webhookID string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.webhookCache, webhookID)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Webhook - Remove by WebhookId")
	}
}

func (s LocalCacheWebhookStore) GetIncoming(id string, allowFromCache bool) (*model.IncomingWebhook, error) {
	if !allowFromCache {
		return s.WebhookStore.GetIncoming(id, allowFromCache)
	}

	var incomingWebhook *model.IncomingWebhook
	if err := s.rootStore.doStandardReadCache(s.rootStore.webhookCache, id, &incomingWebhook); err == nil {
		return incomingWebhook, nil
	}

	incomingWebhook, err := s.WebhookStore.GetIncoming(id, allowFromCache)
	if err != nil {
		return nil, err
	}

	s.rootStore.doStandardAddToCache(s.rootStore.webhookCache, id, incomingWebhook)

	return incomingWebhook, nil
}

func (s LocalCacheWebhookStore) DeleteIncoming(webhookID string, time int64) error {
	err := s.WebhookStore.DeleteIncoming(webhookID, time)
	if err != nil {
		return err
	}

	s.InvalidateWebhookCache(webhookID)
	return nil
}

func (s LocalCacheWebhookStore) PermanentDeleteIncomingByUser(userID string) error {
	err := s.WebhookStore.PermanentDeleteIncomingByUser(userID)
	if err != nil {
		return err
	}

	s.ClearCaches()
	return nil
}

func (s LocalCacheWebhookStore) PermanentDeleteIncomingByChannel(channelID string) error {
	err := s.WebhookStore.PermanentDeleteIncomingByChannel(channelID)
	if err != nil {
		return err
	}

	s.ClearCaches()
	return nil
}
