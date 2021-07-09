// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	LatestKey = "latest"
)

type LocalCacheTermsOfServiceStore struct {
	store.TermsOfServiceStore
	rootStore *LocalCacheStore
}

func (s *LocalCacheTermsOfServiceStore) handleClusterInvalidateTermsOfService(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.termsOfServiceCache.Purge()
	} else {
		s.rootStore.termsOfServiceCache.Remove(msg.Data)
	}
}

func (s LocalCacheTermsOfServiceStore) ClearCaches() {
	s.rootStore.doClearCacheCluster(s.rootStore.termsOfServiceCache)

	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Terms Of Service - Purge")
	}
}

func (s LocalCacheTermsOfServiceStore) Save(termsOfService *model.TermsOfService) (*model.TermsOfService, error) {
	tos, err := s.TermsOfServiceStore.Save(termsOfService)

	if err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.termsOfServiceCache, tos.ID, tos)
		s.rootStore.doInvalidateCacheCluster(s.rootStore.termsOfServiceCache, LatestKey)
	}
	return tos, err
}

func (s LocalCacheTermsOfServiceStore) GetLatest(allowFromCache bool) (*model.TermsOfService, error) {
	if allowFromCache {
		if len, err := s.rootStore.termsOfServiceCache.Len(); err == nil && len != 0 {
			var cacheItem *model.TermsOfService
			if err := s.rootStore.doStandardReadCache(s.rootStore.termsOfServiceCache, LatestKey, &cacheItem); err == nil {
				return cacheItem, nil
			}
		}
	}

	termsOfService, err := s.TermsOfServiceStore.GetLatest(allowFromCache)

	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.termsOfServiceCache, termsOfService.ID, termsOfService)
		s.rootStore.doStandardAddToCache(s.rootStore.termsOfServiceCache, LatestKey, termsOfService)
	}

	return termsOfService, err
}

func (s LocalCacheTermsOfServiceStore) Get(id string, allowFromCache bool) (*model.TermsOfService, error) {
	if allowFromCache {
		var cacheItem *model.TermsOfService
		if err := s.rootStore.doStandardReadCache(s.rootStore.termsOfServiceCache, id, &cacheItem); err == nil {
			return cacheItem, nil
		}
	}

	termsOfService, err := s.TermsOfServiceStore.Get(id, allowFromCache)

	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.termsOfServiceCache, termsOfService.ID, termsOfService)
	}

	return termsOfService, err
}
