// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type LocalCacheChannelStore struct {
	store.ChannelStore
	rootStore *LocalCacheStore
}

func (s *LocalCacheChannelStore) handleClusterInvalidateChannelMemberCounts(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.channelMemberCountsCache.Purge()
	} else {
		s.rootStore.channelMemberCountsCache.Remove(msg.Data)
	}
}

func (s *LocalCacheChannelStore) handleClusterInvalidateChannelPinnedPostCount(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.channelPinnedPostCountsCache.Purge()
	} else {
		s.rootStore.channelPinnedPostCountsCache.Remove(msg.Data)
	}
}

func (s *LocalCacheChannelStore) handleClusterInvalidateChannelGuestCounts(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.channelGuestCountCache.Purge()
	} else {
		s.rootStore.channelGuestCountCache.Remove(msg.Data)
	}
}

func (s *LocalCacheChannelStore) handleClusterInvalidateChannelByID(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.channelByIDCache.Purge()
	} else {
		s.rootStore.channelByIDCache.Remove(msg.Data)
	}
}

func (s LocalCacheChannelStore) ClearCaches() {
	s.rootStore.doClearCacheCluster(s.rootStore.channelMemberCountsCache)
	s.rootStore.doClearCacheCluster(s.rootStore.channelPinnedPostCountsCache)
	s.rootStore.doClearCacheCluster(s.rootStore.channelGuestCountCache)
	s.rootStore.doClearCacheCluster(s.rootStore.channelByIDCache)
	s.ChannelStore.ClearCaches()
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Pinned Post Counts - Purge")
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Member Counts - Purge")
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Guest Count - Purge")
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel - Purge")
	}
}

func (s LocalCacheChannelStore) InvalidatePinnedPostCount(channelID string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.channelPinnedPostCountsCache, channelID)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Pinned Post Counts - Remove by ChannelId")
	}
}

func (s LocalCacheChannelStore) InvalidateMemberCount(channelID string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.channelMemberCountsCache, channelID)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Member Counts - Remove by ChannelId")
	}
}

func (s LocalCacheChannelStore) InvalidateGuestCount(channelID string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.channelGuestCountCache, channelID)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Guests Count - Remove by channelId")
	}
}

func (s LocalCacheChannelStore) InvalidateChannel(channelID string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.channelByIDCache, channelID)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel - Remove by ChannelId")
	}
}

func (s LocalCacheChannelStore) GetMemberCount(channelID string, allowFromCache bool) (int64, error) {
	if allowFromCache {
		var count int64
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelMemberCountsCache, channelID, &count); err == nil {
			return count, nil
		}
	}
	count, err := s.ChannelStore.GetMemberCount(channelID, allowFromCache)

	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.channelMemberCountsCache, channelID, count)
	}

	return count, err
}

func (s LocalCacheChannelStore) GetGuestCount(channelID string, allowFromCache bool) (int64, error) {
	if allowFromCache {
		var count int64
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelGuestCountCache, channelID, &count); err == nil {
			return count, nil
		}
	}
	count, err := s.ChannelStore.GetGuestCount(channelID, allowFromCache)

	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.channelGuestCountCache, channelID, count)
	}

	return count, err
}

func (s LocalCacheChannelStore) GetMemberCountFromCache(channelID string) int64 {
	var count int64
	if err := s.rootStore.doStandardReadCache(s.rootStore.channelMemberCountsCache, channelID, &count); err == nil {
		return count
	}

	count, err := s.GetMemberCount(channelID, true)
	if err != nil {
		return 0
	}

	return count
}

func (s LocalCacheChannelStore) GetPinnedPostCount(channelID string, allowFromCache bool) (int64, error) {
	if allowFromCache {
		var count int64
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelPinnedPostCountsCache, channelID, &count); err == nil {
			return count, nil
		}
	}

	count, err := s.ChannelStore.GetPinnedPostCount(channelID, allowFromCache)

	if err != nil {
		return 0, err
	}

	if allowFromCache {
		s.rootStore.doStandardAddToCache(s.rootStore.channelPinnedPostCountsCache, channelID, count)
	}

	return count, nil
}

func (s LocalCacheChannelStore) Get(id string, allowFromCache bool) (*model.Channel, error) {

	if allowFromCache {
		var cacheItem *model.Channel
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelByIDCache, id, &cacheItem); err == nil {
			return cacheItem, nil
		}
	}

	ch, err := s.ChannelStore.Get(id, allowFromCache)

	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.channelByIDCache, id, ch)
	}

	return ch, err
}

func (s LocalCacheChannelStore) SaveMember(member *model.ChannelMember) (*model.ChannelMember, error) {
	member, err := s.ChannelStore.SaveMember(member)
	if err != nil {
		return nil, err
	}
	s.InvalidateMemberCount(member.ChannelID)
	return member, nil
}

func (s LocalCacheChannelStore) SaveMultipleMembers(members []*model.ChannelMember) ([]*model.ChannelMember, error) {
	members, err := s.ChannelStore.SaveMultipleMembers(members)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		s.InvalidateMemberCount(member.ChannelID)
	}
	return members, nil
}

func (s LocalCacheChannelStore) UpdateMember(member *model.ChannelMember) (*model.ChannelMember, error) {
	member, err := s.ChannelStore.UpdateMember(member)
	if err != nil {
		return nil, err
	}
	s.InvalidateMemberCount(member.ChannelID)
	return member, nil
}

func (s LocalCacheChannelStore) UpdateMultipleMembers(members []*model.ChannelMember) ([]*model.ChannelMember, error) {
	members, err := s.ChannelStore.UpdateMultipleMembers(members)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		s.InvalidateMemberCount(member.ChannelID)
	}
	return members, nil
}

func (s LocalCacheChannelStore) RemoveMember(channelID, userID string) error {
	err := s.ChannelStore.RemoveMember(channelID, userID)
	if err != nil {
		return err
	}
	s.InvalidateMemberCount(channelID)
	return nil
}

func (s LocalCacheChannelStore) RemoveMembers(channelID string, userIDs []string) error {
	err := s.ChannelStore.RemoveMembers(channelID, userIDs)
	if err != nil {
		return err
	}
	s.InvalidateMemberCount(channelID)
	return nil
}
