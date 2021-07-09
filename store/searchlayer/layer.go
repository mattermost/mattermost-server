// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchlayer

import (
	"context"
	"sync/atomic"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SearchStore struct {
	store.Store
	searchEngine *searchengine.Broker
	user         *SearchUserStore
	team         *SearchTeamStore
	channel      *SearchChannelStore
	post         *SearchPostStore
	fileInfo     *SearchFileInfoStore
	configValue  atomic.Value
}

func NewSearchLayer(baseStore store.Store, searchEngine *searchengine.Broker, cfg *model.Config) *SearchStore {
	searchStore := &SearchStore{
		Store:        baseStore,
		searchEngine: searchEngine,
	}
	searchStore.configValue.Store(cfg)
	searchStore.channel = &SearchChannelStore{ChannelStore: baseStore.Channel(), rootStore: searchStore}
	searchStore.post = &SearchPostStore{PostStore: baseStore.Post(), rootStore: searchStore}
	searchStore.team = &SearchTeamStore{TeamStore: baseStore.Team(), rootStore: searchStore}
	searchStore.user = &SearchUserStore{UserStore: baseStore.User(), rootStore: searchStore}
	searchStore.fileInfo = &SearchFileInfoStore{FileInfoStore: baseStore.FileInfo(), rootStore: searchStore}

	return searchStore
}

func (s *SearchStore) UpdateConfig(cfg *model.Config) {
	s.configValue.Store(cfg)
}

func (s *SearchStore) getConfig() *model.Config {
	return s.configValue.Load().(*model.Config)
}

func (s *SearchStore) Channel() store.ChannelStore {
	return s.channel
}

func (s *SearchStore) Post() store.PostStore {
	return s.post
}

func (s *SearchStore) FileInfo() store.FileInfoStore {
	return s.fileInfo
}

func (s *SearchStore) Team() store.TeamStore {
	return s.team
}

func (s *SearchStore) User() store.UserStore {
	return s.user
}

func (s *SearchStore) indexUserFromID(userID string) {
	user, err := s.User().Get(context.Background(), userID)
	if err != nil {
		return
	}
	s.indexUser(user)
}

func (s *SearchStore) indexUser(user *model.User) {
	for _, engine := range s.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				userTeams, nErr := s.Team().GetTeamsByUserID(user.ID)
				if nErr != nil {
					mlog.Error("Encountered error indexing user", mlog.String("user_id", user.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(nErr))
					return
				}

				userTeamsIDs := []string{}
				for _, team := range userTeams {
					userTeamsIDs = append(userTeamsIDs, team.ID)
				}

				userChannelMembers, err := s.Channel().GetAllChannelMembersForUser(user.ID, false, true)
				if err != nil {
					mlog.Error("Encountered error indexing user", mlog.String("user_id", user.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}

				userChannelsIDs := []string{}
				for channelID := range userChannelMembers {
					userChannelsIDs = append(userChannelsIDs, channelID)
				}

				if err := engineCopy.IndexUser(user, userTeamsIDs, userChannelsIDs); err != nil {
					mlog.Error("Encountered error indexing user", mlog.String("user_id", user.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Indexed user in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("user_id", user.ID))
			})
		}
	}
}

// Runs an indexing function synchronously or asynchronously depending on the engine
func runIndexFn(engine searchengine.SearchEngineInterface, indexFn func(searchengine.SearchEngineInterface)) {
	if engine.IsIndexingSync() {
		indexFn(engine)
		if err := engine.RefreshIndexes(); err != nil {
			mlog.Error("Encountered error refresh the indexes", mlog.Err(err))
		}
	} else {
		go (func(engineCopy searchengine.SearchEngineInterface) {
			indexFn(engineCopy)
		})(engine)
	}
}
