// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestPreferenceStore(t *testing.T, ss store.Store) {
	t.Run("PreferenceSave", func(t *testing.T) { testPreferenceSave(t, ss) })
	t.Run("PreferenceGet", func(t *testing.T) { testPreferenceGet(t, ss) })
	t.Run("PreferenceGetCategory", func(t *testing.T) { testPreferenceGetCategory(t, ss) })
	t.Run("PreferenceGetAll", func(t *testing.T) { testPreferenceGetAll(t, ss) })
	t.Run("PreferenceDeleteByUser", func(t *testing.T) { testPreferenceDeleteByUser(t, ss) })
	t.Run("PreferenceDelete", func(t *testing.T) { testPreferenceDelete(t, ss) })
	t.Run("PreferenceDeleteCategory", func(t *testing.T) { testPreferenceDeleteCategory(t, ss) })
	t.Run("PreferenceDeleteCategoryAndName", func(t *testing.T) { testPreferenceDeleteCategoryAndName(t, ss) })
	t.Run("PreferenceDeleteOrphanedRows", func(t *testing.T) { testPreferenceDeleteOrphanedRows(t, ss) })
}

func testPreferenceSave(t *testing.T, ss store.Store) {
	id := model.NewID()

	preferences := model.Preferences{
		{
			UserID:   id,
			Category: model.PreferenceCategoryDirectChannelShow,
			Name:     model.NewID(),
			Value:    "value1a",
		},
		{
			UserID:   id,
			Category: model.PreferenceCategoryDirectChannelShow,
			Name:     model.NewID(),
			Value:    "value1b",
		},
	}
	err := ss.Preference().Save(&preferences)
	require.NoError(t, err, "saving preference returned error")

	for _, preference := range preferences {
		data, _ := ss.Preference().Get(preference.UserID, preference.Category, preference.Name)
		require.Equal(t, data.ToJSON(), preference.ToJSON(), "got incorrect preference after first Save")
	}

	preferences[0].Value = "value2a"
	preferences[1].Value = "value2b"
	err = ss.Preference().Save(&preferences)
	require.NoError(t, err, "saving preference returned error")

	for _, preference := range preferences {
		data, _ := ss.Preference().Get(preference.UserID, preference.Category, preference.Name)
		require.Equal(t, data.ToJSON(), preference.ToJSON(), "got incorrect preference after second Save")
	}
}

func testPreferenceGet(t *testing.T, ss store.Store) {
	userID := model.NewID()
	category := model.PreferenceCategoryDirectChannelShow
	name := model.NewID()

	preferences := model.Preferences{
		{
			UserID:   userID,
			Category: category,
			Name:     name,
		},
		{
			UserID:   userID,
			Category: category,
			Name:     model.NewID(),
		},
		{
			UserID:   userID,
			Category: model.NewID(),
			Name:     name,
		},
		{
			UserID:   model.NewID(),
			Category: category,
			Name:     name,
		},
	}

	err := ss.Preference().Save(&preferences)
	require.NoError(t, err)

	data, err := ss.Preference().Get(userID, category, name)
	require.NoError(t, err)
	require.Equal(t, preferences[0].ToJSON(), data.ToJSON(), "got incorrect preference")

	// make sure getting a missing preference fails
	_, err = ss.Preference().Get(model.NewID(), model.NewID(), model.NewID())
	require.Error(t, err, "no error on getting a missing preference")
}

func testPreferenceGetCategory(t *testing.T, ss store.Store) {
	userID := model.NewID()
	category := model.PreferenceCategoryDirectChannelShow
	name := model.NewID()

	preferences := model.Preferences{
		{
			UserID:   userID,
			Category: category,
			Name:     name,
		},
		// same user/category, different name
		{
			UserID:   userID,
			Category: category,
			Name:     model.NewID(),
		},
		// same user/name, different category
		{
			UserID:   userID,
			Category: model.NewID(),
			Name:     name,
		},
		// same name/category, different user
		{
			UserID:   model.NewID(),
			Category: category,
			Name:     name,
		},
	}

	err := ss.Preference().Save(&preferences)
	require.NoError(t, err)

	preferencesByCategory, err := ss.Preference().GetCategory(userID, category)
	require.NoError(t, err)
	require.Equal(t, 2, len(preferencesByCategory), "got the wrong number of preferences")
	require.True(
		t,
		((preferencesByCategory[0] == preferences[0] && preferencesByCategory[1] == preferences[1]) || (preferencesByCategory[0] == preferences[1] && preferencesByCategory[1] == preferences[0])),
		"got incorrect preferences",
	)

	// make sure getting a missing preference category doesn't fail
	preferencesByCategory, err = ss.Preference().GetCategory(model.NewID(), model.NewID())
	require.NoError(t, err)
	require.Equal(t, 0, len(preferencesByCategory), "shouldn't have got any preferences")
}

func testPreferenceGetAll(t *testing.T, ss store.Store) {
	userID := model.NewID()
	category := model.PreferenceCategoryDirectChannelShow
	name := model.NewID()

	preferences := model.Preferences{
		{
			UserID:   userID,
			Category: category,
			Name:     name,
		},
		// same user/category, different name
		{
			UserID:   userID,
			Category: category,
			Name:     model.NewID(),
		},
		// same user/name, different category
		{
			UserID:   userID,
			Category: model.NewID(),
			Name:     name,
		},
		// same name/category, different user
		{
			UserID:   model.NewID(),
			Category: category,
			Name:     name,
		},
	}

	err := ss.Preference().Save(&preferences)
	require.NoError(t, err)

	result, err := ss.Preference().GetAll(userID)
	require.NoError(t, err)
	require.Equal(t, 3, len(result), "got the wrong number of preferences")

	for i := 0; i < 3; i++ {
		assert.Falsef(t, result[0] != preferences[i] && result[1] != preferences[i] && result[2] != preferences[i], "got incorrect preferences")
	}

}

func testPreferenceDeleteByUser(t *testing.T, ss store.Store) {
	userID := model.NewID()
	category := model.PreferenceCategoryDirectChannelShow
	name := model.NewID()

	preferences := model.Preferences{
		{
			UserID:   userID,
			Category: category,
			Name:     name,
		},
		// same user/category, different name
		{
			UserID:   userID,
			Category: category,
			Name:     model.NewID(),
		},
		// same user/name, different category
		{
			UserID:   userID,
			Category: model.NewID(),
			Name:     name,
		},
		// same name/category, different user
		{
			UserID:   model.NewID(),
			Category: category,
			Name:     name,
		},
	}

	err := ss.Preference().Save(&preferences)
	require.NoError(t, err)

	err = ss.Preference().PermanentDeleteByUser(userID)
	require.NoError(t, err)
}

func testPreferenceDelete(t *testing.T, ss store.Store) {
	preference := model.Preference{
		UserID:   model.NewID(),
		Category: model.PreferenceCategoryDirectChannelShow,
		Name:     model.NewID(),
		Value:    "value1a",
	}

	err := ss.Preference().Save(&model.Preferences{preference})
	require.NoError(t, err)

	preferences, err := ss.Preference().GetAll(preference.UserID)
	require.NoError(t, err)
	assert.Len(t, preferences, 1, "should've returned 1 preference")

	err = ss.Preference().Delete(preference.UserID, preference.Category, preference.Name)
	require.NoError(t, err)
	preferences, err = ss.Preference().GetAll(preference.UserID)
	require.NoError(t, err)
	assert.Empty(t, preferences, "should've returned no preferences")
}

func testPreferenceDeleteCategory(t *testing.T, ss store.Store) {
	category := model.NewID()
	userID := model.NewID()

	preference1 := model.Preference{
		UserID:   userID,
		Category: category,
		Name:     model.NewID(),
		Value:    "value1a",
	}

	preference2 := model.Preference{
		UserID:   userID,
		Category: category,
		Name:     model.NewID(),
		Value:    "value1a",
	}

	err := ss.Preference().Save(&model.Preferences{preference1, preference2})
	require.NoError(t, err)

	preferences, err := ss.Preference().GetAll(userID)
	require.NoError(t, err)
	assert.Len(t, preferences, 2, "should've returned 2 preferences")

	err = ss.Preference().DeleteCategory(userID, category)
	require.NoError(t, err)

	preferences, err = ss.Preference().GetAll(userID)
	require.NoError(t, err)
	assert.Empty(t, preferences, "should've returned no preferences")
}

func testPreferenceDeleteCategoryAndName(t *testing.T, ss store.Store) {
	category := model.NewID()
	name := model.NewID()
	userID := model.NewID()
	userID2 := model.NewID()

	preference1 := model.Preference{
		UserID:   userID,
		Category: category,
		Name:     name,
		Value:    "value1a",
	}

	preference2 := model.Preference{
		UserID:   userID2,
		Category: category,
		Name:     name,
		Value:    "value1a",
	}

	err := ss.Preference().Save(&model.Preferences{preference1, preference2})
	require.NoError(t, err)

	preferences, err := ss.Preference().GetAll(userID)
	require.NoError(t, err)
	assert.Len(t, preferences, 1, "should've returned 1 preference")

	preferences, err = ss.Preference().GetAll(userID2)
	require.NoError(t, err)
	assert.Len(t, preferences, 1, "should've returned 1 preference")

	err = ss.Preference().DeleteCategoryAndName(category, name)
	require.NoError(t, err)

	preferences, err = ss.Preference().GetAll(userID)
	require.NoError(t, err)
	assert.Empty(t, preferences, "should've returned no preference")

	preferences, err = ss.Preference().GetAll(userID2)
	require.NoError(t, err)
	assert.Empty(t, preferences, "should've returned no preference")
}

func testPreferenceDeleteOrphanedRows(t *testing.T, ss store.Store) {
	const limit = 1000
	team, err := ss.Team().Save(&model.Team{
		DisplayName: "DisplayName",
		Name:        "team" + model.NewID(),
		Email:       MakeEmail(),
		Type:        model.TeamOpen,
	})
	require.NoError(t, err)
	channel, err := ss.Channel().Save(&model.Channel{
		TeamID:      team.ID,
		DisplayName: "DisplayName",
		Name:        "channel" + model.NewID(),
		Type:        model.ChannelTypeOpen,
	}, -1)
	require.NoError(t, err)
	category := model.PreferenceCategoryFlaggedPost
	userID := model.NewID()

	olderPost, err := ss.Post().Save(&model.Post{
		ChannelID: channel.ID,
		UserID:    userID,
		Message:   "message",
		CreateAt:  1000,
	})
	require.NoError(t, err)
	newerPost, err := ss.Post().Save(&model.Post{
		ChannelID: channel.ID,
		UserID:    userID,
		Message:   "message",
		CreateAt:  3000,
	})
	require.NoError(t, err)

	preference1 := model.Preference{
		UserID:   userID,
		Category: category,
		Name:     olderPost.ID,
		Value:    "true",
	}

	preference2 := model.Preference{
		UserID:   userID,
		Category: category,
		Name:     newerPost.ID,
		Value:    "true",
	}

	nErr := ss.Preference().Save(&model.Preferences{preference1, preference2})
	require.NoError(t, nErr)

	_, _, nErr = ss.Post().PermanentDeleteBatchForRetentionPolicies(0, 2000, limit, model.RetentionPolicyCursor{})
	assert.NoError(t, nErr)

	_, nErr = ss.Preference().DeleteOrphanedRows(limit)
	assert.NoError(t, nErr)

	_, nErr = ss.Preference().Get(userID, category, preference1.Name)
	assert.Error(t, nErr, "older preference should have been deleted")

	_, nErr = ss.Preference().Get(userID, category, preference2.Name)
	assert.NoError(t, nErr, "newer preference should not have been deleted")
}
