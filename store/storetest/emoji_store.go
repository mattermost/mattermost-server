// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"context"
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmojiStore(t *testing.T, ss store.Store) {
	t.Run("EmojiSaveDelete", func(t *testing.T) { testEmojiSaveDelete(t, ss) })
	t.Run("EmojiGet", func(t *testing.T) { testEmojiGet(t, ss) })
	t.Run("EmojiGetByName", func(t *testing.T) { testEmojiGetByName(t, ss) })
	t.Run("EmojiGetMultipleByName", func(t *testing.T) { testEmojiGetMultipleByName(t, ss) })
	t.Run("EmojiGetList", func(t *testing.T) { testEmojiGetList(t, ss) })
	t.Run("EmojiSearch", func(t *testing.T) { testEmojiSearch(t, ss) })
}

func testEmojiSaveDelete(t *testing.T, ss store.Store) {
	emoji1 := &model.Emoji{
		CreatorID: model.NewID(),
		Name:      model.NewID(),
	}

	_, err := ss.Emoji().Save(emoji1)
	require.NoError(t, err)

	assert.Len(t, emoji1.ID, 26, "should've set id for emoji")

	emoji2 := model.Emoji{
		CreatorID: model.NewID(),
		Name:      emoji1.Name,
	}
	_, err = ss.Emoji().Save(&emoji2)
	require.Error(t, err, "shouldn't be able to save emoji with duplicate name")

	err = ss.Emoji().Delete(emoji1, time.Now().Unix())
	require.NoError(t, err)

	_, err = ss.Emoji().Save(&emoji2)
	require.NoError(t, err, "should be able to save emoji with duplicate name now that original has been deleted")

	err = ss.Emoji().Delete(&emoji2, time.Now().Unix()+1)
	require.NoError(t, err)
}

func testEmojiGet(t *testing.T, ss store.Store) {
	emojis := []model.Emoji{
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
	}

	for i, emoji := range emojis {
		data, err := ss.Emoji().Save(&emoji)
		require.NoError(t, err)
		emojis[i] = *data
	}
	defer func() {
		for _, emoji := range emojis {
			err := ss.Emoji().Delete(&emoji, time.Now().Unix())
			require.NoError(t, err)
		}
	}()

	for _, emoji := range emojis {
		_, err := ss.Emoji().Get(context.Background(), emoji.ID, false)
		require.NoErrorf(t, err, "failed to get emoji with id %v", emoji.ID)
	}

	for _, emoji := range emojis {
		_, err := ss.Emoji().Get(context.Background(), emoji.ID, true)
		require.NoErrorf(t, err, "failed to get emoji with id %v", emoji.ID)
	}
}

func testEmojiGetByName(t *testing.T, ss store.Store) {
	emojis := []model.Emoji{
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
	}

	for i, emoji := range emojis {
		data, err := ss.Emoji().Save(&emoji)
		require.NoError(t, err)
		emojis[i] = *data
	}
	defer func() {
		for _, emoji := range emojis {
			err := ss.Emoji().Delete(&emoji, time.Now().Unix())
			require.NoError(t, err)
		}
	}()

	for _, emoji := range emojis {
		_, err := ss.Emoji().GetByName(context.Background(), emoji.Name, true)
		require.NoErrorf(t, err, "failed to get emoji with name %v", emoji.Name)
	}
}

func testEmojiGetMultipleByName(t *testing.T, ss store.Store) {
	emojis := []model.Emoji{
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
	}

	for i, emoji := range emojis {
		data, err := ss.Emoji().Save(&emoji)
		require.NoError(t, err)
		emojis[i] = *data
	}
	defer func() {
		for _, emoji := range emojis {
			err := ss.Emoji().Delete(&emoji, time.Now().Unix())
			require.NoError(t, err)
		}
	}()

	t.Run("one emoji", func(t *testing.T) {
		received, err := ss.Emoji().GetMultipleByName([]string{emojis[0].Name})
		require.NoError(t, err, "could not get emoji")
		require.Len(t, received, 1, "got incorrect emoji")
		require.Equal(t, *received[0], emojis[0], "got incorrect emoji")
	})

	t.Run("multiple emojis", func(t *testing.T) {
		received, err := ss.Emoji().GetMultipleByName([]string{emojis[0].Name, emojis[1].Name, emojis[2].Name})
		require.NoError(t, err, "could not get emojis")
		require.Len(t, received, 3, "got incorrect emojis")
	})

	t.Run("one nonexistent emoji", func(t *testing.T) {
		received, err := ss.Emoji().GetMultipleByName([]string{"ab"})
		require.NoError(t, err, "could not get emoji", err)
		require.Empty(t, received, "got incorrect emoji")
	})

	t.Run("multiple emojis with nonexistent names", func(t *testing.T) {
		received, err := ss.Emoji().GetMultipleByName([]string{emojis[0].Name, emojis[1].Name, emojis[2].Name, "abcd", "1234"})
		require.NoError(t, err, "could not get emojis")
		require.Len(t, received, 3, "got incorrect emojis")
	})
}

func testEmojiGetList(t *testing.T, ss store.Store) {
	emojis := []model.Emoji{
		{
			CreatorID: model.NewID(),
			Name:      "00000000000000000000000000a" + model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      "00000000000000000000000000b" + model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      "00000000000000000000000000c" + model.NewID(),
		},
	}

	for i, emoji := range emojis {
		data, err := ss.Emoji().Save(&emoji)
		require.NoError(t, err)
		emojis[i] = *data
	}
	defer func() {
		for _, emoji := range emojis {
			err := ss.Emoji().Delete(&emoji, time.Now().Unix())
			require.NoError(t, err)
		}
	}()

	result, err := ss.Emoji().GetList(0, 100, "")
	require.NoError(t, err)

	for _, emoji := range emojis {
		found := false

		for _, savedEmoji := range result {
			if emoji.ID == savedEmoji.ID {
				found = true
				break
			}
		}

		require.Truef(t, found, "failed to get emoji with id %v", emoji.ID)
	}

	remojis, err := ss.Emoji().GetList(0, 3, model.EmojiSortByName)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(remojis))
	assert.Equal(t, emojis[0].Name, remojis[0].Name)
	assert.Equal(t, emojis[1].Name, remojis[1].Name)
	assert.Equal(t, emojis[2].Name, remojis[2].Name)

	remojis, err = ss.Emoji().GetList(1, 2, model.EmojiSortByName)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(remojis))
	assert.Equal(t, emojis[1].Name, remojis[0].Name)
	assert.Equal(t, emojis[2].Name, remojis[1].Name)

}

func testEmojiSearch(t *testing.T, ss store.Store) {
	emojis := []model.Emoji{
		{
			CreatorID: model.NewID(),
			Name:      "blargh_" + model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID() + "_blargh",
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID() + "_blargh_" + model.NewID(),
		},
		{
			CreatorID: model.NewID(),
			Name:      model.NewID(),
		},
	}

	for i, emoji := range emojis {
		data, err := ss.Emoji().Save(&emoji)
		require.NoError(t, err)
		emojis[i] = *data
	}
	defer func() {
		for _, emoji := range emojis {
			err := ss.Emoji().Delete(&emoji, time.Now().Unix())
			require.NoError(t, err)
		}
	}()

	shouldFind := []bool{true, false, false, false}

	result, err := ss.Emoji().Search("blargh", true, 100)
	require.NoError(t, err)
	for i, emoji := range emojis {
		found := false

		for _, savedEmoji := range result {
			if emoji.ID == savedEmoji.ID {
				found = true
				break
			}
		}

		assert.Equal(t, shouldFind[i], found, emoji.Name)
	}

	shouldFind = []bool{true, true, true, false}
	result, err = ss.Emoji().Search("blargh", false, 100)
	require.NoError(t, err)
	for i, emoji := range emojis {
		found := false

		for _, savedEmoji := range result {
			if emoji.ID == savedEmoji.ID {
				found = true
				break
			}
		}

		assert.Equal(t, shouldFind[i], found, emoji.Name)
	}
}
