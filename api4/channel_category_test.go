// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestCreateCategoryForTeamForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should silently prevent the user from creating a category with an invalid channel ID", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		// Attempt to create the category
		category := &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				UserID:      user.ID,
				TeamID:      th.BasicTeam.ID,
				DisplayName: "test",
			},
			Channels: []string{th.BasicChannel.ID, "notachannel", th.BasicChannel2.ID},
		}

		received, resp := client.CreateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, category)
		require.Nil(t, resp.Error)
		assert.NotContains(t, received.Channels, "notachannel")
		assert.Equal(t, []string{th.BasicChannel.ID, th.BasicChannel2.ID}, received.Channels)
	})

	t.Run("should silently prevent the user from creating a category with a channel that they're not a member of", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		// Have another user create a channel that user isn't a part of
		channel, resp := th.SystemAdminClient.CreateChannel(&model.Channel{
			TeamID: th.BasicTeam.ID,
			Type:   model.ChannelTypeOpen,
			Name:   "testchannel",
		})
		require.Nil(t, resp.Error)

		// Attempt to create the category
		category := &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				UserID:      user.ID,
				TeamID:      th.BasicTeam.ID,
				DisplayName: "test",
			},
			Channels: []string{th.BasicChannel.ID, channel.ID},
		}

		received, resp := client.CreateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, category)
		require.Nil(t, resp.Error)
		assert.NotContains(t, received.Channels, channel.ID)
		assert.Equal(t, []string{th.BasicChannel.ID}, received.Channels)
	})
}

func TestUpdateCategoryForTeamForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should update the channel order of the Channels category", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory := categories.Categories[1]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)
		require.Len(t, channelsCategory.Channels, 5) // Town Square, Off Topic, and the 3 channels created by InitBasic

		// Should return the correct values from the API
		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: channelsCategory.SidebarCategory,
			Channels:        []string{channelsCategory.Channels[1], channelsCategory.Channels[0], channelsCategory.Channels[4], channelsCategory.Channels[3], channelsCategory.Channels[2]},
		}

		received, resp := client.UpdateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, channelsCategory.ID, updatedCategory)
		assert.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received.ID)
		assert.Equal(t, updatedCategory.Channels, received.Channels)

		// And when requesting the category later
		received, resp = client.GetSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, channelsCategory.ID, "")
		assert.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received.ID)
		assert.Equal(t, updatedCategory.Channels, received.Channels)
	})

	t.Run("should update the sort order of the DM category", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		dmsCategory := categories.Categories[2]
		require.Equal(t, model.SidebarCategoryDirectMessages, dmsCategory.Type)
		require.Equal(t, model.SidebarCategorySortRecent, dmsCategory.Sorting)

		// Should return the correct values from the API
		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: dmsCategory.SidebarCategory,
			Channels:        dmsCategory.Channels,
		}
		updatedCategory.Sorting = model.SidebarCategorySortAlphabetical

		received, resp := client.UpdateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, dmsCategory.ID, updatedCategory)
		assert.Nil(t, resp.Error)
		assert.Equal(t, dmsCategory.ID, received.ID)
		assert.Equal(t, model.SidebarCategorySortAlphabetical, received.Sorting)

		// And when requesting the category later
		received, resp = client.GetSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, dmsCategory.ID, "")
		assert.Nil(t, resp.Error)
		assert.Equal(t, dmsCategory.ID, received.ID)
		assert.Equal(t, model.SidebarCategorySortAlphabetical, received.Sorting)
	})

	t.Run("should update the display name of a custom category", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		customCategory, resp := client.CreateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				UserID:      user.ID,
				TeamID:      th.BasicTeam.ID,
				DisplayName: "custom123",
			},
		})
		require.Nil(t, resp.Error)
		require.Equal(t, "custom123", customCategory.DisplayName)

		// Should return the correct values from the API
		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: customCategory.SidebarCategory,
			Channels:        customCategory.Channels,
		}
		updatedCategory.DisplayName = "abcCustom"

		received, resp := client.UpdateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, customCategory.ID, updatedCategory)
		assert.Nil(t, resp.Error)
		assert.Equal(t, customCategory.ID, received.ID)
		assert.Equal(t, updatedCategory.DisplayName, received.DisplayName)

		// And when requesting the category later
		received, resp = client.GetSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, customCategory.ID, "")
		assert.Nil(t, resp.Error)
		assert.Equal(t, customCategory.ID, received.ID)
		assert.Equal(t, updatedCategory.DisplayName, received.DisplayName)
	})

	t.Run("should update the channel order of the category even if it contains archived channels", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory := categories.Categories[1]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)
		require.Len(t, channelsCategory.Channels, 5) // Town Square, Off Topic, and the 3 channels created by InitBasic

		// Delete one of the channels
		_, resp = client.DeleteChannel(th.BasicChannel.ID)
		require.Nil(t, resp.Error)

		// Should still be able to reorder the channels
		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: channelsCategory.SidebarCategory,
			Channels:        []string{channelsCategory.Channels[1], channelsCategory.Channels[0], channelsCategory.Channels[4], channelsCategory.Channels[3], channelsCategory.Channels[2]},
		}

		received, resp := client.UpdateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, channelsCategory.ID, updatedCategory)
		require.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received.ID)
		assert.Equal(t, updatedCategory.Channels, received.Channels)
	})

	t.Run("should silently prevent the user from adding an invalid channel ID", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory := categories.Categories[1]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)

		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: channelsCategory.SidebarCategory,
			Channels:        append(channelsCategory.Channels, "notachannel"),
		}

		received, resp := client.UpdateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, channelsCategory.ID, updatedCategory)
		require.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received.ID)
		assert.NotContains(t, received.Channels, "notachannel")
		assert.Equal(t, channelsCategory.Channels, received.Channels)
	})

	t.Run("should silently prevent the user from adding a channel that they're not a member of", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory := categories.Categories[1]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)

		// Have another user create a channel that user isn't a part of
		channel, resp := th.SystemAdminClient.CreateChannel(&model.Channel{
			TeamID: th.BasicTeam.ID,
			Type:   model.ChannelTypeOpen,
			Name:   "testchannel",
		})
		require.Nil(t, resp.Error)

		// Attempt to update the category
		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: channelsCategory.SidebarCategory,
			Channels:        append(channelsCategory.Channels, channel.ID),
		}

		received, resp := client.UpdateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, channelsCategory.ID, updatedCategory)
		require.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received.ID)
		assert.NotContains(t, received.Channels, channel.ID)
		assert.Equal(t, channelsCategory.Channels, received.Channels)
	})

	t.Run("muting a category should mute all of its channels", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory := categories.Categories[1]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)
		require.True(t, len(channelsCategory.Channels) > 0)

		// Mute the category
		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				ID:      channelsCategory.ID,
				UserID:  user.ID,
				TeamID:  th.BasicTeam.ID,
				Sorting: channelsCategory.Sorting,
				Muted:   true,
			},
			Channels: channelsCategory.Channels,
		}

		received, resp := client.UpdateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, channelsCategory.ID, updatedCategory)
		require.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received.ID)
		assert.True(t, received.Muted)

		// Check that the muted category was saved in the database
		received, resp = client.GetSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, channelsCategory.ID, "")
		require.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received.ID)
		assert.True(t, received.Muted)

		// Confirm that the channels in the category were muted
		member, resp := client.GetChannelMember(channelsCategory.Channels[0], user.ID, "")
		require.Nil(t, resp.Error)
		assert.True(t, member.IsChannelMuted())
	})

	t.Run("should not be able to mute DM category", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		dmsCategory := categories.Categories[2]
		require.Equal(t, model.SidebarCategoryDirectMessages, dmsCategory.Type)
		require.Len(t, dmsCategory.Channels, 0)

		// Ensure a DM channel exists
		dmChannel, resp := client.CreateDirectChannel(user.ID, th.BasicUser.ID)
		require.Nil(t, resp.Error)

		// Attempt to mute the category
		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				ID:      dmsCategory.ID,
				UserID:  user.ID,
				TeamID:  th.BasicTeam.ID,
				Sorting: dmsCategory.Sorting,
				Muted:   true,
			},
			Channels: []string{dmChannel.ID},
		}

		received, resp := client.UpdateSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, dmsCategory.ID, updatedCategory)
		require.Nil(t, resp.Error)
		assert.Equal(t, dmsCategory.ID, received.ID)
		assert.False(t, received.Muted)

		// Check that the muted category was not saved in the database
		received, resp = client.GetSidebarCategoryForTeamForUser(user.ID, th.BasicTeam.ID, dmsCategory.ID, "")
		require.Nil(t, resp.Error)
		assert.Equal(t, dmsCategory.ID, received.ID)
		assert.False(t, received.Muted)

		// Confirm that the channels in the category were not muted
		member, resp := client.GetChannelMember(dmChannel.ID, user.ID, "")
		require.Nil(t, resp.Error)
		assert.False(t, member.IsChannelMuted())
	})
}

func TestUpdateCategoriesForTeamForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should silently prevent the user from adding an invalid channel ID", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory := categories.Categories[1]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)

		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: channelsCategory.SidebarCategory,
			Channels:        append(channelsCategory.Channels, "notachannel"),
		}

		received, resp := client.UpdateSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{updatedCategory})
		require.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received[0].ID)
		assert.NotContains(t, received[0].Channels, "notachannel")
		assert.Equal(t, channelsCategory.Channels, received[0].Channels)
	})

	t.Run("should silently prevent the user from adding a channel that they're not a member of", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory := categories.Categories[1]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)

		// Have another user create a channel that user isn't a part of
		channel, resp := th.SystemAdminClient.CreateChannel(&model.Channel{
			TeamID: th.BasicTeam.ID,
			Type:   model.ChannelTypeOpen,
			Name:   "testchannel",
		})
		require.Nil(t, resp.Error)

		// Attempt to update the category
		updatedCategory := &model.SidebarCategoryWithChannels{
			SidebarCategory: channelsCategory.SidebarCategory,
			Channels:        append(channelsCategory.Channels, channel.ID),
		}

		received, resp := client.UpdateSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, []*model.SidebarCategoryWithChannels{updatedCategory})
		require.Nil(t, resp.Error)
		assert.Equal(t, channelsCategory.ID, received[0].ID)
		assert.NotContains(t, received[0].Channels, channel.ID)
		assert.Equal(t, channelsCategory.Channels, received[0].Channels)
	})

	t.Run("should update order", func(t *testing.T) {
		user, client := setupUserForSubtest(t, th)

		categories, resp := client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory := categories.Categories[1]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)

		_, resp = client.UpdateSidebarCategoryOrderForTeamForUser(user.ID, th.BasicTeam.ID, []string{categories.Order[1], categories.Order[0], categories.Order[2]})
		require.Nil(t, resp.Error)

		categories, resp = client.GetSidebarCategoriesForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.Len(t, categories.Categories, 3)
		require.Len(t, categories.Order, 3)

		channelsCategory = categories.Categories[0]
		require.Equal(t, model.SidebarCategoryChannels, channelsCategory.Type)

		// validate order
		newOrder, resp := client.GetSidebarCategoryOrderForTeamForUser(user.ID, th.BasicTeam.ID, "")
		require.Nil(t, resp.Error)
		require.EqualValues(t, newOrder, categories.Order)

		// try to update with missing category
		_, resp = client.UpdateSidebarCategoryOrderForTeamForUser(user.ID, th.BasicTeam.ID, []string{categories.Order[1], categories.Order[0]})
		require.NotNil(t, resp.Error)

		// try to update with invalid category
		_, resp = client.UpdateSidebarCategoryOrderForTeamForUser(user.ID, th.BasicTeam.ID, []string{categories.Order[1], categories.Order[0], "asd"})
		require.NotNil(t, resp.Error)
	})
}

func setupUserForSubtest(t *testing.T, th *TestHelper) (*model.User, *model.Client4) {
	password := "password"
	user, err := th.App.CreateUser(th.Context, &model.User{
		Email:    th.GenerateTestEmail(),
		Username: "user_" + model.NewID(),
		Password: password,
	})
	require.Nil(t, err)

	th.LinkUserToTeam(user, th.BasicTeam)
	th.AddUserToChannel(user, th.BasicChannel)
	th.AddUserToChannel(user, th.BasicChannel2)
	th.AddUserToChannel(user, th.BasicPrivateChannel)

	client := th.CreateClient()
	user, resp := client.Login(user.Email, password)
	require.Nil(t, resp.Error)

	return user, client
}
