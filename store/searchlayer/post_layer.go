// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchlayer

import (
	"context"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SearchPostStore struct {
	store.PostStore
	rootStore *SearchStore
}

func (s SearchPostStore) indexPost(post *model.Post) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				channel, chanErr := s.rootStore.Channel().Get(post.ChannelID, true)
				if chanErr != nil {
					mlog.Error("Couldn't get channel for post for SearchEngine indexing.", mlog.String("channel_id", post.ChannelID), mlog.String("search_engine", engineCopy.GetName()), mlog.String("post_id", post.ID), mlog.Err(chanErr))
					return
				}
				if err := engineCopy.IndexPost(post, channel.TeamID); err != nil {
					mlog.Warn("Encountered error indexing post", mlog.String("post_id", post.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Indexed post in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("post_id", post.ID))
			})
		}
	}
}

func (s SearchPostStore) deletePostIndex(post *model.Post) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if err := engineCopy.DeletePost(post); err != nil {
					mlog.Warn("Encountered error deleting post", mlog.String("post_id", post.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Removed post from the index in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("post_id", post.ID))
			})
		}
	}
}

func (s SearchPostStore) deleteChannelPostsIndex(channelID string) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if err := engineCopy.DeleteChannelPosts(channelID); err != nil {
					mlog.Warn("Encountered error deleting channel posts", mlog.String("channel_id", channelID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Removed all channel posts from the index in search engine", mlog.String("channel_id", channelID), mlog.String("search_engine", engineCopy.GetName()))
			})
		}
	}
}

func (s SearchPostStore) deleteUserPostsIndex(userID string) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if err := engineCopy.DeleteUserPosts(userID); err != nil {
					mlog.Warn("Encountered error deleting user posts", mlog.String("user_id", userID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Removed all user posts from the index in search engine", mlog.String("user_id", userID), mlog.String("search_engine", engineCopy.GetName()))
			})
		}
	}
}

func (s SearchPostStore) Update(newPost, oldPost *model.Post) (*model.Post, error) {
	post, err := s.PostStore.Update(newPost, oldPost)

	if err == nil {
		s.indexPost(post)
	}
	return post, err
}

func (s *SearchPostStore) Overwrite(post *model.Post) (*model.Post, error) {
	post, err := s.PostStore.Overwrite(post)
	if err == nil {
		s.indexPost(post)
	}
	return post, err
}

func (s SearchPostStore) Save(post *model.Post) (*model.Post, error) {
	npost, err := s.PostStore.Save(post)

	if err == nil {
		s.indexPost(npost)
	}
	return npost, err
}

func (s SearchPostStore) Delete(postID string, date int64, deletedByID string) error {
	err := s.PostStore.Delete(postID, date, deletedByID)

	if err == nil {
		postList, err2 := s.PostStore.Get(context.Background(), postID, true, false, false, "")
		if postList != nil && len(postList.Order) > 0 {
			if err2 != nil {
				s.deletePostIndex(postList.Posts[postList.Order[0]])
			}
		}
	}
	return err
}

func (s SearchPostStore) PermanentDeleteByUser(userID string) error {
	err := s.PostStore.PermanentDeleteByUser(userID)
	if err == nil {
		s.deleteUserPostsIndex(userID)
	}
	return err
}

func (s SearchPostStore) PermanentDeleteByChannel(channelID string) error {
	err := s.PostStore.PermanentDeleteByChannel(channelID)
	if err == nil {
		s.deleteChannelPostsIndex(channelID)
	}
	return err
}

func (s SearchPostStore) searchPostsInTeamForUserByEngine(engine searchengine.SearchEngineInterface, paramsList []*model.SearchParams, userID, teamID string, page, perPage int) (*model.PostSearchResults, error) {
	if err := model.IsSearchParamsListValid(paramsList); err != nil {
		return nil, err
	}

	// We only allow the user to search in channels they are a member of.
	userChannels, err2 := s.rootStore.Channel().GetChannels(teamID, userID, paramsList[0].IncludeDeletedChannels, 0)
	if err2 != nil {
		return nil, errors.Wrap(err2, "error getting channel for user")
	}

	postIDs, matches, err := engine.SearchPosts(userChannels, paramsList, page, perPage)
	if err != nil {
		return nil, err
	}

	// Get the posts
	postList := model.NewPostList()
	if len(postIDs) > 0 {
		posts, err := s.PostStore.GetPostsByIDs(postIDs)
		if err != nil {
			return nil, err
		}
		for _, p := range posts {
			if p.DeleteAt == 0 {
				postList.AddPost(p)
				postList.AddOrder(p.ID)
			}
		}
	}

	return model.MakePostSearchResults(postList, matches), nil
}

func (s SearchPostStore) SearchPostsInTeamForUser(paramsList []*model.SearchParams, userID, teamID string, page, perPage int) (*model.PostSearchResults, error) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsSearchEnabled() {
			results, err := s.searchPostsInTeamForUserByEngine(engine, paramsList, userID, teamID, page, perPage)
			if err != nil {
				mlog.Warn("Encountered error on SearchPostsInTeamForUser.", mlog.String("search_engine", engine.GetName()), mlog.Err(err))
				continue
			}
			mlog.Debug("Using the first available search engine", mlog.String("search_engine", engine.GetName()))
			return results, err
		}
	}

	if *s.rootStore.getConfig().SqlSettings.DisableDatabaseSearch {
		mlog.Debug("Returning empty results for post SearchPostsInTeam as the database search is disabled")
		return &model.PostSearchResults{PostList: model.NewPostList(), Matches: model.PostSearchMatches{}}, nil
	}

	mlog.Debug("Using database search because no other search engine is available")
	return s.PostStore.SearchPostsInTeamForUser(paramsList, userID, teamID, page, perPage)
}
