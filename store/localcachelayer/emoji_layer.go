// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"context"
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/sqlstore"
)

type LocalCacheEmojiStore struct {
	store.EmojiStore
	rootStore                *LocalCacheStore
	emojiByIDMut             sync.Mutex
	emojiByIDInvalidations   map[string]bool
	emojiByNameMut           sync.Mutex
	emojiByNameInvalidations map[string]bool
}

func (es *LocalCacheEmojiStore) handleClusterInvalidateEmojiByID(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		es.rootStore.emojiCacheByID.Purge()
	} else {
		es.emojiByIDMut.Lock()
		es.emojiByIDInvalidations[msg.Data] = true
		es.emojiByIDMut.Unlock()
		es.rootStore.emojiCacheByID.Remove(msg.Data)
	}
}

func (es *LocalCacheEmojiStore) handleClusterInvalidateEmojiIDByName(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		es.rootStore.emojiIDCacheByName.Purge()
	} else {
		es.emojiByNameMut.Lock()
		es.emojiByNameInvalidations[msg.Data] = true
		es.emojiByNameMut.Unlock()
		es.rootStore.emojiIDCacheByName.Remove(msg.Data)
	}
}

func (es *LocalCacheEmojiStore) Get(ctx context.Context, id string, allowFromCache bool) (*model.Emoji, error) {
	if allowFromCache {
		if emoji, ok := es.getFromCacheByID(id); ok {
			return emoji, nil
		}
	}

	// If it was invalidated, then we need to query master.
	es.emojiByIDMut.Lock()
	if es.emojiByIDInvalidations[id] {
		// And then remove the key from the map.
		ctx = sqlstore.WithMaster(ctx)
		delete(es.emojiByIDInvalidations, id)
	}
	es.emojiByIDMut.Unlock()

	emoji, err := es.EmojiStore.Get(ctx, id, allowFromCache)

	if allowFromCache && err == nil {
		es.addToCache(emoji)
	}

	return emoji, err
}

func (es *LocalCacheEmojiStore) GetByName(ctx context.Context, name string, allowFromCache bool) (*model.Emoji, error) {
	if id, ok := model.GetSystemEmojiID(name); ok {
		return es.Get(ctx, id, allowFromCache)
	}

	if allowFromCache {
		if emoji, ok := es.getFromCacheByName(name); ok {
			return emoji, nil
		}
	}

	// If it was invalidated, then we need to query master.
	es.emojiByNameMut.Lock()
	if es.emojiByNameInvalidations[name] {
		ctx = sqlstore.WithMaster(ctx)
		// And then remove the key from the map.
		delete(es.emojiByNameInvalidations, name)
	}
	es.emojiByNameMut.Unlock()

	emoji, err := es.EmojiStore.GetByName(ctx, name, allowFromCache)

	if allowFromCache && err == nil {
		es.addToCache(emoji)
	}

	return emoji, err
}

func (es *LocalCacheEmojiStore) Delete(emoji *model.Emoji, time int64) error {
	err := es.EmojiStore.Delete(emoji, time)

	if err == nil {
		es.removeFromCache(emoji)
	}

	return err
}

func (es *LocalCacheEmojiStore) addToCache(emoji *model.Emoji) {
	es.rootStore.doStandardAddToCache(es.rootStore.emojiCacheByID, emoji.ID, emoji)
	es.rootStore.doStandardAddToCache(es.rootStore.emojiIDCacheByName, emoji.Name, emoji.ID)
}

func (es *LocalCacheEmojiStore) getFromCacheByID(id string) (*model.Emoji, bool) {
	var emoji *model.Emoji
	if err := es.rootStore.doStandardReadCache(es.rootStore.emojiCacheByID, id, &emoji); err == nil {
		return emoji, true
	}
	return nil, false
}

func (es *LocalCacheEmojiStore) getFromCacheByName(name string) (*model.Emoji, bool) {
	var emojiID string
	if err := es.rootStore.doStandardReadCache(es.rootStore.emojiIDCacheByName, name, &emojiID); err == nil {
		return es.getFromCacheByID(emojiID)
	}
	return nil, false
}

func (es *LocalCacheEmojiStore) removeFromCache(emoji *model.Emoji) {
	es.emojiByIDMut.Lock()
	es.emojiByIDInvalidations[emoji.ID] = true
	es.emojiByIDMut.Unlock()
	es.rootStore.doInvalidateCacheCluster(es.rootStore.emojiCacheByID, emoji.ID)

	es.emojiByNameMut.Lock()
	es.emojiByNameInvalidations[emoji.Name] = true
	es.emojiByNameMut.Unlock()
	es.rootStore.doInvalidateCacheCluster(es.rootStore.emojiIDCacheByName, emoji.Name)
}
