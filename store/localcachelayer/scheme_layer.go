// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type LocalCacheSchemeStore struct {
	store.SchemeStore
	rootStore *LocalCacheStore
}

func (s *LocalCacheSchemeStore) handleClusterInvalidateScheme(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.schemeCache.Purge()
	} else {
		s.rootStore.schemeCache.Remove(msg.Data)
	}
}

func (s LocalCacheSchemeStore) Save(scheme *model.Scheme) (*model.Scheme, error) {
	if scheme.ID != "" {
		defer s.rootStore.doInvalidateCacheCluster(s.rootStore.schemeCache, scheme.ID)
	}
	return s.SchemeStore.Save(scheme)
}

func (s LocalCacheSchemeStore) Get(schemeID string) (*model.Scheme, error) {
	var scheme *model.Scheme
	if err := s.rootStore.doStandardReadCache(s.rootStore.schemeCache, schemeID, &scheme); err == nil {
		return scheme, nil
	}

	scheme, err := s.SchemeStore.Get(schemeID)
	if err != nil {
		return nil, err
	}

	s.rootStore.doStandardAddToCache(s.rootStore.schemeCache, schemeID, scheme)

	return scheme, nil
}

func (s LocalCacheSchemeStore) Delete(schemeID string) (*model.Scheme, error) {
	defer s.rootStore.doInvalidateCacheCluster(s.rootStore.schemeCache, schemeID)
	defer s.rootStore.doClearCacheCluster(s.rootStore.roleCache)
	defer s.rootStore.doClearCacheCluster(s.rootStore.rolePermissionsCache)
	return s.SchemeStore.Delete(schemeID)
}

func (s LocalCacheSchemeStore) PermanentDeleteAll() error {
	defer s.rootStore.doClearCacheCluster(s.rootStore.schemeCache)
	defer s.rootStore.doClearCacheCluster(s.rootStore.roleCache)
	defer s.rootStore.doClearCacheCluster(s.rootStore.rolePermissionsCache)
	return s.SchemeStore.PermanentDeleteAll()
}
