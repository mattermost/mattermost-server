// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type LocalCacheReactionStore struct {
	store.ReactionStore
	rootStore *LocalCacheStore
}

func (s *LocalCacheReactionStore) handleClusterInvalidateReaction(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.reactionCache.Purge()
	} else {
		s.rootStore.reactionCache.Remove(msg.Data)
	}
}

func (s LocalCacheReactionStore) Save(reaction *model.Reaction) (*model.Reaction, error) {
	defer s.rootStore.doInvalidateCacheCluster(s.rootStore.reactionCache, reaction.PostID)
	return s.ReactionStore.Save(reaction)
}

func (s LocalCacheReactionStore) Delete(reaction *model.Reaction) (*model.Reaction, error) {
	defer s.rootStore.doInvalidateCacheCluster(s.rootStore.reactionCache, reaction.PostID)
	return s.ReactionStore.Delete(reaction)
}

func (s LocalCacheReactionStore) GetForPost(postID string, allowFromCache bool) ([]*model.Reaction, error) {
	if !allowFromCache {
		return s.ReactionStore.GetForPost(postID, false)
	}

	var reaction []*model.Reaction
	if err := s.rootStore.doStandardReadCache(s.rootStore.reactionCache, postID, &reaction); err == nil {
		return reaction, nil
	}

	reaction, err := s.ReactionStore.GetForPost(postID, false)
	if err != nil {
		return nil, err
	}

	s.rootStore.doStandardAddToCache(s.rootStore.reactionCache, postID, reaction)

	return reaction, nil
}

func (s LocalCacheReactionStore) DeleteAllWithEmojiName(emojiName string) error {
	// This could be improved. Right now we just clear the whole
	// cache because we don't have a way find what post Ids have this emoji name.
	defer s.rootStore.doClearCacheCluster(s.rootStore.reactionCache)
	return s.ReactionStore.DeleteAllWithEmojiName(emojiName)
}
