// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type LocalCacheTeamStore struct {
	store.TeamStore
	rootStore *LocalCacheStore
}

func (s *LocalCacheTeamStore) handleClusterInvalidateTeam(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.teamAllTeamIDsForUserCache.Purge()
	} else {
		s.rootStore.teamAllTeamIDsForUserCache.Remove(msg.Data)
	}
}

func (s LocalCacheTeamStore) ClearCaches() {
	s.rootStore.teamAllTeamIDsForUserCache.Purge()
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("All Team Ids for User - Purge")
	}
}

func (s LocalCacheTeamStore) InvalidateAllTeamIDsForUser(userID string) {
	s.rootStore.doInvalidateCacheCluster(s.rootStore.teamAllTeamIDsForUserCache, userID)
	if s.rootStore.metrics != nil {
		s.rootStore.metrics.IncrementMemCacheInvalidationCounter("All Team Ids for User - Remove by UserId")
	}
}

func (s LocalCacheTeamStore) GetUserTeamIDs(userID string, allowFromCache bool) ([]string, error) {
	if !allowFromCache {
		return s.TeamStore.GetUserTeamIDs(userID, allowFromCache)
	}

	var userTeamIDs []string
	if err := s.rootStore.doStandardReadCache(s.rootStore.teamAllTeamIDsForUserCache, userID, &userTeamIDs); err == nil {
		return userTeamIDs, nil
	}

	userTeamIDs, err := s.TeamStore.GetUserTeamIDs(userID, allowFromCache)
	if err != nil {
		return nil, err
	}

	if len(userTeamIDs) > 0 {
		s.rootStore.doStandardAddToCache(s.rootStore.teamAllTeamIDsForUserCache, userID, userTeamIDs)
	}

	return userTeamIDs, nil
}

func (s LocalCacheTeamStore) Update(team *model.Team) (*model.Team, error) {
	var oldTeam *model.Team
	var err error
	if team.DeleteAt != 0 {
		oldTeam, err = s.TeamStore.Get(team.ID)
		if err != nil {
			return nil, err
		}
	}

	tm, err := s.TeamStore.Update(team)
	if err != nil {
		return nil, err
	}
	defer s.rootStore.doClearCacheCluster(s.rootStore.rolePermissionsCache)

	if oldTeam != nil && oldTeam.DeleteAt == 0 {
		s.rootStore.doClearCacheCluster(s.rootStore.teamAllTeamIDsForUserCache)
	}

	return tm, err
}
