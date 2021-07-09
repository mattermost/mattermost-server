// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestSidebarCategory(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	basicChannel2 := th.CreateChannel(th.BasicTeam)
	defer th.App.PermanentDeleteChannel(basicChannel2)
	user := th.CreateUser()
	defer th.App.Srv().Store.User().PermanentDelete(user.ID)
	th.LinkUserToTeam(user, th.BasicTeam)
	th.AddUserToChannel(user, basicChannel2)

	var createdCategory *model.SidebarCategoryWithChannels
	t.Run("CreateSidebarCategory", func(t *testing.T) {
		catData := model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				DisplayName: "TEST",
			},
			Channels: []string{th.BasicChannel.ID, basicChannel2.ID, basicChannel2.ID},
		}
		_, err := th.App.CreateSidebarCategory(user.ID, th.BasicTeam.ID, &catData)
		require.NotNil(t, err, "Should return error due to duplicate IDs")
		catData.Channels = []string{th.BasicChannel.ID, basicChannel2.ID}
		cat, err := th.App.CreateSidebarCategory(user.ID, th.BasicTeam.ID, &catData)
		require.Nil(t, err, "Expected no error")
		require.NotNil(t, cat, "Expected category object, got nil")
		createdCategory = cat
	})

	t.Run("UpdateSidebarCategories", func(t *testing.T) {
		require.NotNil(t, createdCategory)
		createdCategory.Channels = []string{th.BasicChannel.ID}
		updatedCat, err := th.App.UpdateSidebarCategories(user.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{createdCategory})
		require.Nil(t, err, "Expected no error")
		require.NotNil(t, updatedCat, "Expected category object, got nil")
		require.Len(t, updatedCat, 1)
		require.Len(t, updatedCat[0].Channels, 1)
		require.Equal(t, updatedCat[0].Channels[0], th.BasicChannel.ID)
	})

	t.Run("UpdateSidebarCategoryOrder", func(t *testing.T) {
		err := th.App.UpdateSidebarCategoryOrder(user.ID, th.BasicTeam.ID, []string{th.BasicChannel.ID, basicChannel2.ID})
		require.NotNil(t, err, "Should return error due to invalid order")

		actualOrder, err := th.App.GetSidebarCategoryOrder(user.ID, th.BasicTeam.ID)
		require.Nil(t, err, "Should fetch order successfully")

		actualOrder[2], actualOrder[3] = actualOrder[3], actualOrder[2]
		err = th.App.UpdateSidebarCategoryOrder(user.ID, th.BasicTeam.ID, actualOrder)
		require.Nil(t, err, "Should update order successfully")

		// We create a copy of actualOrder to prevent racy read
		// of the slice when the broadcast message is sent from webhub.
		newOrder := make([]string, len(actualOrder))
		copy(newOrder, actualOrder)
		newOrder[2] = "asd"
		err = th.App.UpdateSidebarCategoryOrder(user.ID, th.BasicTeam.ID, newOrder)
		require.NotNil(t, err, "Should return error due to invalid id")
	})

	t.Run("GetSidebarCategoryOrder", func(t *testing.T) {
		catOrder, err := th.App.GetSidebarCategoryOrder(user.ID, th.BasicTeam.ID)
		require.Nil(t, err, "Expected no error")
		require.Len(t, catOrder, 4)
		require.Equal(t, catOrder[1], createdCategory.ID, "the newly created category should be after favorites")
	})
}

func TestGetSidebarCategories(t *testing.T) {
	t.Run("should return the sidebar categories for the given user/team", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		_, err := th.App.CreateSidebarCategory(th.BasicUser.ID, th.BasicTeam.ID, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				UserID:      th.BasicUser.ID,
				TeamID:      th.BasicTeam.ID,
				DisplayName: "new category",
			},
		})
		require.Nil(t, err)

		categories, err := th.App.GetSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID)
		assert.Nil(t, err)
		assert.Len(t, categories.Categories, 4)
	})

	t.Run("should create the initial categories even if migration hasn't ran yet", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		// Manually add the user to the team without going through the app layer to simulate a pre-existing user/team
		// relationship that hasn't been migrated yet
		team := th.CreateTeam()
		_, err := th.App.Srv().Store.Team().SaveMember(&model.TeamMember{
			TeamID:     team.ID,
			UserID:     th.BasicUser.ID,
			SchemeUser: true,
		}, 100)
		require.NoError(t, err)

		categories, appErr := th.App.GetSidebarCategories(th.BasicUser.ID, team.ID)
		assert.Nil(t, appErr)
		assert.Len(t, categories.Categories, 3)
	})

	t.Run("should return a store error if a db table is missing", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		// Temporarily renaming a table to force a DB error.
		sqlStore := mainHelper.GetSQLStore()
		_, err := sqlStore.GetMaster().Exec("ALTER TABLE SidebarCategories RENAME TO SidebarCategoriesTest")
		require.NoError(t, err)
		defer func() {
			_, err := sqlStore.GetMaster().Exec("ALTER TABLE SidebarCategoriesTest RENAME TO SidebarCategories")
			require.NoError(t, err)
		}()

		categories, appErr := th.App.GetSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID)
		assert.Nil(t, categories)
		assert.NotNil(t, appErr)
		assert.Equal(t, "app.channel.sidebar_categories.app_error", appErr.ID)
	})
}

func TestUpdateSidebarCategories(t *testing.T) {
	t.Run("should mute and unmute all channels in a category when it is muted or unmuted", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		categories, err := th.App.GetSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID)
		require.Nil(t, err)

		channelsCategory := categories.Categories[1]

		// Create some channels to be part of the channels category
		channel1 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel1)

		channel2 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel2)

		// Mute the category
		updated, err := th.App.UpdateSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:    channelsCategory.ID,
					Muted: true,
				},
				Channels: []string{channel1.ID, channel2.ID},
			},
		})
		require.Nil(t, err)
		assert.True(t, updated[0].Muted)

		// Confirm that the channels are now muted
		member1, err := th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.True(t, member1.IsChannelMuted())
		member2, err := th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.True(t, member2.IsChannelMuted())

		// Unmute the category
		updated, err = th.App.UpdateSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:    channelsCategory.ID,
					Muted: false,
				},
				Channels: []string{channel1.ID, channel2.ID},
			},
		})
		require.Nil(t, err)
		assert.False(t, updated[0].Muted)

		// Confirm that the channels are now unmuted
		member1, err = th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.False(t, member1.IsChannelMuted())
		member2, err = th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.False(t, member2.IsChannelMuted())
	})

	t.Run("should mute and unmute channels moved from an unmuted category to a muted one and back", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		// Create some channels
		channel1 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel1)

		channel2 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel2)

		// And some categories
		mutedCategory, err := th.App.CreateSidebarCategory(th.BasicUser.ID, th.BasicTeam.ID, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				DisplayName: "muted",
				Muted:       true,
			},
		})
		require.Nil(t, err)
		require.True(t, mutedCategory.Muted)

		unmutedCategory, err := th.App.CreateSidebarCategory(th.BasicUser.ID, th.BasicTeam.ID, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				DisplayName: "unmuted",
				Muted:       false,
			},
			Channels: []string{channel1.ID, channel2.ID},
		})
		require.Nil(t, err)
		require.False(t, unmutedCategory.Muted)

		// Move the channels
		_, err = th.App.UpdateSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          mutedCategory.ID,
					DisplayName: mutedCategory.DisplayName,
					Muted:       mutedCategory.Muted,
				},
				Channels: []string{channel1.ID, channel2.ID},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          unmutedCategory.ID,
					DisplayName: unmutedCategory.DisplayName,
					Muted:       unmutedCategory.Muted,
				},
				Channels: []string{},
			},
		})
		require.Nil(t, err)

		// Confirm that the channels are now muted
		member1, err := th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.True(t, member1.IsChannelMuted())
		member2, err := th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.True(t, member2.IsChannelMuted())

		// Move the channels back
		_, err = th.App.UpdateSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          mutedCategory.ID,
					DisplayName: mutedCategory.DisplayName,
					Muted:       mutedCategory.Muted,
				},
				Channels: []string{},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          unmutedCategory.ID,
					DisplayName: unmutedCategory.DisplayName,
					Muted:       unmutedCategory.Muted,
				},
				Channels: []string{channel1.ID, channel2.ID},
			},
		})
		require.Nil(t, err)

		// Confirm that the channels are now unmuted
		member1, err = th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.False(t, member1.IsChannelMuted())
		member2, err = th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.False(t, member2.IsChannelMuted())
	})

	t.Run("should not mute or unmute channels moved between muted categories", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		// Create some channels
		channel1 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel1)

		channel2 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel2)

		// And some categories
		category1, err := th.App.CreateSidebarCategory(th.BasicUser.ID, th.BasicTeam.ID, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				DisplayName: "category1",
				Muted:       true,
			},
		})
		require.Nil(t, err)
		require.True(t, category1.Muted)

		category2, err := th.App.CreateSidebarCategory(th.BasicUser.ID, th.BasicTeam.ID, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				DisplayName: "category2",
				Muted:       true,
			},
			Channels: []string{channel1.ID, channel2.ID},
		})
		require.Nil(t, err)
		require.True(t, category2.Muted)

		// Move the unmuted channels
		_, err = th.App.UpdateSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          category1.ID,
					DisplayName: category1.DisplayName,
					Muted:       category1.Muted,
				},
				Channels: []string{channel1.ID, channel2.ID},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          category2.ID,
					DisplayName: category2.DisplayName,
					Muted:       category2.Muted,
				},
				Channels: []string{},
			},
		})
		require.Nil(t, err)

		// Confirm that the channels are still unmuted
		member1, err := th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.False(t, member1.IsChannelMuted())
		member2, err := th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.False(t, member2.IsChannelMuted())

		// Mute the channels manually
		_, err = th.App.ToggleMuteChannel(channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		_, err = th.App.ToggleMuteChannel(channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)

		// Move the muted channels back
		_, err = th.App.UpdateSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          category1.ID,
					DisplayName: category1.DisplayName,
					Muted:       category1.Muted,
				},
				Channels: []string{},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          category2.ID,
					DisplayName: category2.DisplayName,
					Muted:       category2.Muted,
				},
				Channels: []string{channel1.ID, channel2.ID},
			},
		})
		require.Nil(t, err)

		// Confirm that the channels are still muted
		member1, err = th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.True(t, member1.IsChannelMuted())
		member2, err = th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.True(t, member2.IsChannelMuted())
	})

	t.Run("should not mute or unmute channels moved between unmuted categories", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		// Create some channels
		channel1 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel1)

		channel2 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel2)

		// And some categories
		category1, err := th.App.CreateSidebarCategory(th.BasicUser.ID, th.BasicTeam.ID, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				DisplayName: "category1",
				Muted:       false,
			},
		})
		require.Nil(t, err)
		require.False(t, category1.Muted)

		category2, err := th.App.CreateSidebarCategory(th.BasicUser.ID, th.BasicTeam.ID, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				DisplayName: "category2",
				Muted:       false,
			},
			Channels: []string{channel1.ID, channel2.ID},
		})
		require.Nil(t, err)
		require.False(t, category2.Muted)

		// Move the unmuted channels
		_, err = th.App.UpdateSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          category1.ID,
					DisplayName: category1.DisplayName,
					Muted:       category1.Muted,
				},
				Channels: []string{channel1.ID, channel2.ID},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          category2.ID,
					DisplayName: category2.DisplayName,
					Muted:       category2.Muted,
				},
				Channels: []string{},
			},
		})
		require.Nil(t, err)

		// Confirm that the channels are still unmuted
		member1, err := th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.False(t, member1.IsChannelMuted())
		member2, err := th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.False(t, member2.IsChannelMuted())

		// Mute the channels manually
		_, err = th.App.ToggleMuteChannel(channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		_, err = th.App.ToggleMuteChannel(channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)

		// Move the muted channels back
		_, err = th.App.UpdateSidebarCategories(th.BasicUser.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          category1.ID,
					DisplayName: category1.DisplayName,
					Muted:       category1.Muted,
				},
				Channels: []string{},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          category2.ID,
					DisplayName: category2.DisplayName,
					Muted:       category2.Muted,
				},
				Channels: []string{channel1.ID, channel2.ID},
			},
		})
		require.Nil(t, err)

		// Confirm that the channels are still muted
		member1, err = th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.True(t, member1.IsChannelMuted())
		member2, err = th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
		require.Nil(t, err)
		assert.True(t, member2.IsChannelMuted())
	})
}

func TestDiffChannelsBetweenCategories(t *testing.T) {
	t.Run("should return nothing when the categories contain identical channels", func(t *testing.T) {
		originalCategories := []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category1",
					DisplayName: "Category One",
				},
				Channels: []string{"channel1", "channel2", "channel3"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category2",
					DisplayName: "Category Two",
				},
				Channels: []string{"channel4", "channel5"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category3",
					DisplayName: "Category Three",
				},
				Channels: []string{},
			},
		}

		updatedCategories := []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category1",
					DisplayName: "Category Won",
				},
				Channels: []string{"channel1", "channel2", "channel3"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category2",
					DisplayName: "Category Too",
				},
				Channels: []string{"channel4", "channel5"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category3",
					DisplayName: "Category 🌲",
				},
				Channels: []string{},
			},
		}

		channelsDiff := diffChannelsBetweenCategories(updatedCategories, originalCategories)
		assert.Equal(t, map[string]*categoryChannelDiff{}, channelsDiff)
	})

	t.Run("should return nothing when the categories contain identical channels", func(t *testing.T) {
		originalCategories := []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category1",
					DisplayName: "Category One",
				},
				Channels: []string{"channel1", "channel2", "channel3"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category2",
					DisplayName: "Category Two",
				},
				Channels: []string{"channel4", "channel5"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category3",
					DisplayName: "Category Three",
				},
				Channels: []string{},
			},
		}

		updatedCategories := []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category1",
					DisplayName: "Category Won",
				},
				Channels: []string{},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category2",
					DisplayName: "Category Too",
				},
				Channels: []string{"channel5", "channel2"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category3",
					DisplayName: "Category 🌲",
				},
				Channels: []string{"channel4", "channel1", "channel3"},
			},
		}

		channelsDiff := diffChannelsBetweenCategories(updatedCategories, originalCategories)
		assert.Equal(
			t,
			map[string]*categoryChannelDiff{
				"channel1": {
					fromCategoryID: "category1",
					toCategoryID:   "category3",
				},
				"channel2": {
					fromCategoryID: "category1",
					toCategoryID:   "category2",
				},
				"channel3": {
					fromCategoryID: "category1",
					toCategoryID:   "category3",
				},
				"channel4": {
					fromCategoryID: "category2",
					toCategoryID:   "category3",
				},
			},
			channelsDiff,
		)
	})

	t.Run("should not return channels that are moved in our out of the categories implicitly", func(t *testing.T) {
		// This case could change to actually return the channels in the future, but we don't need to handle it right now
		originalCategories := []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category1",
					DisplayName: "Category One",
				},
				Channels: []string{"channel1", "channel2"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category2",
					DisplayName: "Category Two",
				},
				Channels: []string{"channel3"},
			},
		}

		updatedCategories := []*model.SidebarCategoryWithChannels{
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category1",
					DisplayName: "Category Won",
				},
				Channels: []string{"channel1", "channel3"},
			},
			{
				SidebarCategory: model.SidebarCategory{
					ID:          "category2",
					DisplayName: "Category Too",
				},
				Channels: []string{"channel4"},
			},
		}

		channelsDiff := diffChannelsBetweenCategories(updatedCategories, originalCategories)
		assert.Equal(
			t,
			map[string]*categoryChannelDiff{
				"channel3": {
					fromCategoryID: "category2",
					toCategoryID:   "category1",
				},
			},
			channelsDiff,
		)
	})
}
