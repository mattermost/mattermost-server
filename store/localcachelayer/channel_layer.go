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

func (s *LocalCacheChannelStore) handleClusterInvalidateChannelById(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.channelByIdCache.Purge()
	} else {
		s.rootStore.channelByIdCache.Remove(msg.Data)
	}
}

func (s LocalCacheChannelStore) ClearCaches() {
	s.rootStore.doClearCacheCluster(s.rootStore.channelMemberCountsCache)
	s.rootStore.doClearCacheCluster(s.rootStore.channelPinnedPostCountsCache)
	s.rootStore.doClearCacheCluster(s.rootStore.channelGuestCountCache)
	s.rootStore.doClearCacheCluster(s.rootStore.channelByIdCache)
	s.rootStore.doClearCacheCluster(s.rootStore.channelMembersNotifyPropsForChannelCache)
	s.ChannelStore.ClearCaches()
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Pinned Post Counts - Purge")
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Member Counts - Purge")
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Guest Count - Purge")
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel - Purge")
	}
}

func (s LocalCacheChannelStore) InvalidatePinnedPostCount(channelId string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.channelPinnedPostCountsCache, channelId)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Pinned Post Counts - Remove by ChannelId")
	}
}

func (s LocalCacheChannelStore) InvalidateMemberCount(channelId string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.channelMemberCountsCache, channelId)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Member Counts - Remove by ChannelId")
	}
}

func (s LocalCacheChannelStore) InvalidateGuestCount(channelId string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.channelGuestCountCache, channelId)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel Guests Count - Remove by channelId")
	}
}

func (s LocalCacheChannelStore) InvalidateChannel(channelId string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.channelByIdCache, channelId)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("Channel - Remove by ChannelId")
	}
}

func (s LocalCacheChannelStore) GetMemberCount(channelId string, allowFromCache bool) (int64, error) {
	if allowFromCache {
		var count int64
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelMemberCountsCache, channelId, &count); err == nil {
			return count, nil
		}
	}
	count, err := s.ChannelStore.GetMemberCount(channelId, allowFromCache)

	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.channelMemberCountsCache, channelId, count)
	}

	return count, err
}

func (s LocalCacheChannelStore) GetGuestCount(channelId string, allowFromCache bool) (int64, error) {
	if allowFromCache {
		var count int64
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelGuestCountCache, channelId, &count); err == nil {
			return count, nil
		}
	}
	count, err := s.ChannelStore.GetGuestCount(channelId, allowFromCache)

	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.channelGuestCountCache, channelId, count)
	}

	return count, err
}

func (s LocalCacheChannelStore) GetAllChannelMembersNotifyPropsForChannel(channelId string, allowFromCache bool) (map[string]model.StringMap, error) {
	if allowFromCache {
		var cacheItem map[string]model.StringMap
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelMembersNotifyPropsForChannelCache, channelId, &cacheItem); err == nil {
			return cacheItem, nil
		}
	}

	propsForChannel, err := s.ChannelStore.GetAllChannelMembersNotifyPropsForChannel(channelId, allowFromCache)
	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.channelMembersNotifyPropsForChannelCache, channelId, propsForChannel)
	}

	return propsForChannel, err
}

func (s LocalCacheChannelStore) GetMemberCountFromCache(channelId string) int64 {
	var count int64
	if err := s.rootStore.doStandardReadCache(s.rootStore.channelMemberCountsCache, channelId, &count); err == nil {
		return count
	}

	count, err := s.GetMemberCount(channelId, true)
	if err != nil {
		return 0
	}

	return count
}

func (s LocalCacheChannelStore) GetPinnedPostCount(channelId string, allowFromCache bool) (int64, error) {
	if allowFromCache {
		var count int64
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelPinnedPostCountsCache, channelId, &count); err == nil {
			return count, nil
		}
	}

	count, err := s.ChannelStore.GetPinnedPostCount(channelId, allowFromCache)

	if err != nil {
		return 0, err
	}

	if allowFromCache {
		s.rootStore.doStandardAddToCache(s.rootStore.channelPinnedPostCountsCache, channelId, count)
	}

	return count, nil
}

func (s LocalCacheChannelStore) Get(id string, allowFromCache bool) (*model.Channel, error) {

	if allowFromCache {
		var cacheItem *model.Channel
		if err := s.rootStore.doStandardReadCache(s.rootStore.channelByIdCache, id, &cacheItem); err == nil {
			return cacheItem, nil
		}
	}

	ch, err := s.ChannelStore.Get(id, allowFromCache)

	if allowFromCache && err == nil {
		s.rootStore.doStandardAddToCache(s.rootStore.channelByIdCache, id, ch)
	}

	return ch, err
}

func (s LocalCacheChannelStore) SaveMember(member *model.ChannelMember) (*model.ChannelMember, error) {
	member, err := s.ChannelStore.SaveMember(member)
	if err != nil {
		return nil, err
	}
	s.InvalidateMemberCount(member.ChannelId)
	return member, nil
}

func (s LocalCacheChannelStore) SaveMultipleMembers(members []*model.ChannelMember) ([]*model.ChannelMember, error) {
	members, err := s.ChannelStore.SaveMultipleMembers(members)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		s.InvalidateMemberCount(member.ChannelId)
	}
	return members, nil
}

func (s LocalCacheChannelStore) UpdateMember(member *model.ChannelMember) (*model.ChannelMember, error) {
	member, err := s.ChannelStore.UpdateMember(member)
	if err != nil {
		return nil, err
	}
	s.InvalidateMemberCount(member.ChannelId)
	return member, nil
}

func (s LocalCacheChannelStore) UpdateMultipleMembers(members []*model.ChannelMember) ([]*model.ChannelMember, error) {
	members, err := s.ChannelStore.UpdateMultipleMembers(members)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		s.InvalidateMemberCount(member.ChannelId)
	}
	return members, nil
}

func (s LocalCacheChannelStore) RemoveMember(channelId, userId string) error {
	err := s.ChannelStore.RemoveMember(channelId, userId)
	if err != nil {
		return err
	}
	s.InvalidateMemberCount(channelId)
	return nil
}

func (s LocalCacheChannelStore) RemoveMembers(channelId string, userIds []string) error {
	err := s.ChannelStore.RemoveMembers(channelId, userIds)
	if err != nil {
		return err
	}
	s.InvalidateMemberCount(channelId)
	return nil
}
