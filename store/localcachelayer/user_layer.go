// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"context"
	"sort"
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/sqlstore"
)

type LocalCacheUserStore struct {
	store.UserStore
	rootStore                     *LocalCacheStore
	userProfileByIDsMut           sync.Mutex
	userProfileByIDsInvalidations map[string]bool
}

func (s *LocalCacheUserStore) handleClusterInvalidateScheme(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.userProfileByIDsCache.Purge()
	} else {
		s.userProfileByIDsMut.Lock()
		s.userProfileByIDsInvalidations[msg.Data] = true
		s.userProfileByIDsMut.Unlock()
		s.rootStore.userProfileByIDsCache.Remove(msg.Data)
	}
}

func (s *LocalCacheUserStore) handleClusterInvalidateProfilesInChannel(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.profilesInChannelCache.Purge()
	} else {
		s.rootStore.profilesInChannelCache.Remove(msg.Data)
	}
}

func (s *LocalCacheUserStore) ClearCaches() {
	s.rootStore.userProfileByIDsCache.Purge()
	s.rootStore.profilesInChannelCache.Purge()

	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Profile By Ids - Purge")
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Profiles in Channel - Purge")
	}
}

func (s *LocalCacheUserStore) InvalidateProfileCacheForUser(userID string) {
	s.userProfileByIDsMut.Lock()
	s.userProfileByIDsInvalidations[userID] = true
	s.userProfileByIDsMut.Unlock()
	s.rootStore.doInvalidateCacheCluster(s.rootStore.userProfileByIDsCache, userID)

	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Profile By Ids - Remove")
	}
}

func (s *LocalCacheUserStore) InvalidateProfilesInChannelCacheByUser(userID string) {
	keys, err := s.rootStore.profilesInChannelCache.Keys()
	if err == nil {
		for _, key := range keys {
			var userMap map[string]*model.User
			if err = s.rootStore.profilesInChannelCache.Get(key, &userMap); err == nil {
				if _, userInCache := userMap[userID]; userInCache {
					s.rootStore.doInvalidateCacheCluster(s.rootStore.profilesInChannelCache, key)
					if s.rootStore.metrics != nil {
						s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Profiles in Channel - Remove by User")
					}
				}
			}
		}
	}
}

func (s *LocalCacheUserStore) InvalidateProfilesInChannelCache(channelID string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.profilesInChannelCache, channelID)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Profiles in Channel - Remove by Channel")
	}
}

func (s *LocalCacheUserStore) GetAllProfilesInChannel(ctx context.Context, channelID string, allowFromCache bool) (map[string]*model.User, error) {
	if allowFromCache {
		var cachedMap map[string]*model.User
		if err := s.rootStore.doStandardReadCache(s.rootStore.profilesInChannelCache, channelID, &cachedMap); err == nil {
			return cachedMap, nil
		}
	}

	userMap, err := s.UserStore.GetAllProfilesInChannel(ctx, channelID, allowFromCache)
	if err != nil {
		return nil, err
	}

	if allowFromCache {
		s.rootStore.doStandardAddToCache(s.rootStore.profilesInChannelCache, channelID, model.UserMap(userMap))
	}

	return userMap, nil
}

func (s *LocalCacheUserStore) GetProfileByIDs(ctx context.Context, userIDs []string, options *store.UserGetByIDsOpts, allowFromCache bool) ([]*model.User, error) {
	if !allowFromCache {
		return s.UserStore.GetProfileByIDs(ctx, userIDs, options, false)
	}

	if options == nil {
		options = &store.UserGetByIDsOpts{}
	}

	users := []*model.User{}
	remainingUserIDs := make([]string, 0)

	fromMaster := false
	for _, userID := range userIDs {
		var cacheItem *model.User
		if err := s.rootStore.doStandardReadCache(s.rootStore.userProfileByIDsCache, userID, &cacheItem); err == nil {
			if options.Since == 0 || cacheItem.UpdateAt > options.Since {
				users = append(users, cacheItem)
			}
		} else {
			// If it was invalidated, then we need to query master.
			s.userProfileByIDsMut.Lock()
			if s.userProfileByIDsInvalidations[userID] {
				fromMaster = true
				// And then remove the key from the map.
				delete(s.userProfileByIDsInvalidations, userID)
			}
			s.userProfileByIDsMut.Unlock()
			remainingUserIDs = append(remainingUserIDs, userID)
		}
	}

	if len(remainingUserIDs) > 0 {
		if fromMaster {
			ctx = sqlstore.WithMaster(ctx)
		}
		remainingUsers, err := s.UserStore.GetProfileByIDs(ctx, remainingUserIDs, options, false)
		if err != nil {
			return nil, err
		}
		for _, user := range remainingUsers {
			s.rootStore.doStandardAddToCache(s.rootStore.userProfileByIDsCache, user.ID, user)
			users = append(users, user)
		}
	}

	return users, nil
}

// Get is a cache wrapper around the SqlStore method to get a user profile by id.
// It checks if the user entry is present in the cache, returning the entry from cache
// if it is present. Otherwise, it fetches the entry from the store and stores it in the
// cache.
func (s *LocalCacheUserStore) Get(ctx context.Context, id string) (*model.User, error) {
	var cacheItem *model.User
	if err := s.rootStore.doStandardReadCache(s.rootStore.userProfileByIDsCache, id, &cacheItem); err == nil {
		if s.rootStore.metrics != nil {
			s.rootStore.metrics.AddMemCacheHitCounter("Profile By Id", float64(1))
		}
		return cacheItem, nil
	}
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.AddMemCacheMissCounter("Profile By Id", float64(1))
	}

	// If it was invalidated, then we need to query master.
	s.userProfileByIDsMut.Lock()
	if s.userProfileByIDsInvalidations[id] {
		ctx = sqlstore.WithMaster(ctx)
		// And then remove the key from the map.
		delete(s.userProfileByIDsInvalidations, id)
	}
	s.userProfileByIDsMut.Unlock()

	user, err := s.UserStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.rootStore.doStandardAddToCache(s.rootStore.userProfileByIDsCache, id, user)
	return user, nil
}

// GetMany is a cache wrapper around the SqlStore method to get a user profiles by ids.
// It checks if the user entries are present in the cache, returning the entries from cache
// if it is present. Otherwise, it fetches the entries from the store and stores it in the
// cache.
func (s *LocalCacheUserStore) GetMany(ctx context.Context, ids []string) ([]*model.User, error) {
	// we are doing a loop instead of caching the full set in the cache because the number of permutations that we can have
	// in this func is making caching of the total set not beneficial.
	var cachedUsers []*model.User
	var notCachedUserIDs []string
	uniqIDs := dedup(ids)

	fromMaster := false
	for _, id := range uniqIDs {
		var cachedUser *model.User
		if err := s.rootStore.doStandardReadCache(s.rootStore.userProfileByIDsCache, id, &cachedUser); err == nil {
			if s.rootStore.metrics != nil {
				s.rootStore.metrics.AddMemCacheHitCounter("Profile By Id", float64(1))
			}
			cachedUsers = append(cachedUsers, cachedUser)
		} else {
			if s.rootStore.metrics != nil {
				s.rootStore.metrics.AddMemCacheMissCounter("Profile By Id", float64(1))
			}
			// If it was invalidated, then we need to query master.
			s.userProfileByIDsMut.Lock()
			if s.userProfileByIDsInvalidations[id] {
				fromMaster = true
				// And then remove the key from the map.
				delete(s.userProfileByIDsInvalidations, id)
			}
			s.userProfileByIDsMut.Unlock()

			notCachedUserIDs = append(notCachedUserIDs, id)
		}
	}

	if len(notCachedUserIDs) > 0 {
		if fromMaster {
			ctx = sqlstore.WithMaster(ctx)
		}
		dbUsers, err := s.UserStore.GetMany(ctx, notCachedUserIDs)
		if err != nil {
			return nil, err
		}
		for _, user := range dbUsers {
			s.rootStore.doStandardAddToCache(s.rootStore.userProfileByIDsCache, user.ID, user)
			cachedUsers = append(cachedUsers, user)
		}
	}

	return cachedUsers, nil
}

func dedup(elements []string) []string {
	if len(elements) == 0 {
		return elements
	}

	sort.Strings(elements)

	j := 0
	for i := 1; i < len(elements); i++ {
		if elements[j] == elements[i] {
			continue
		}
		j++
		// preserve the original data
		// in[i], in[j] = in[j], in[i]
		// only set what is required
		elements[j] = elements[i]
	}

	return elements[:j+1]
}
