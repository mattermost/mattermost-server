// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/users"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestPermanentDeleteChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableIncomingWebhooks = true
		*cfg.ServiceSettings.EnableOutgoingWebhooks = true
	})

	channel, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "deletion-test", Name: "deletion-test", Type: model.CHANNEL_OPEN, TeamId: th.BasicTeam.Id}, false)
	require.NotNil(t, channel, "Channel shouldn't be nil")
	require.Nil(t, err)
	defer func() {
		th.App.PermanentDeleteChannel(channel)
	}()

	incoming, err := th.App.CreateIncomingWebhookForChannel(th.BasicUser.Id, channel, &model.IncomingWebhook{ChannelId: channel.Id})
	require.NotNil(t, incoming, "incoming webhook should not be nil")
	require.Nil(t, err, "Unable to create Incoming Webhook for Channel")
	defer th.App.DeleteIncomingWebhook(incoming.Id)

	incoming, err = th.App.GetIncomingWebhook(incoming.Id)
	require.NotNil(t, incoming, "incoming webhook should not be nil")
	require.Nil(t, err, "Unable to get new incoming webhook")

	outgoing, err := th.App.CreateOutgoingWebhook(&model.OutgoingWebhook{
		ChannelId:    channel.Id,
		TeamId:       channel.TeamId,
		CreatorId:    th.BasicUser.Id,
		CallbackURLs: []string{"http://foo"},
	})
	require.Nil(t, err)
	defer th.App.DeleteOutgoingWebhook(outgoing.Id)

	outgoing, err = th.App.GetOutgoingWebhook(outgoing.Id)
	require.NotNil(t, outgoing, "Outgoing webhook should not be nil")
	require.Nil(t, err, "Unable to get new outgoing webhook")

	err = th.App.PermanentDeleteChannel(channel)
	require.Nil(t, err)

	incoming, err = th.App.GetIncomingWebhook(incoming.Id)
	require.Nil(t, incoming, "Incoming webhook should be nil")
	require.NotNil(t, err, "Incoming webhook wasn't deleted")

	outgoing, err = th.App.GetOutgoingWebhook(outgoing.Id)
	require.Nil(t, outgoing, "Outgoing webhook should be nil")
	require.NotNil(t, err, "Outgoing webhook wasn't deleted")
}

func TestRemoveAllDeactivatedMembersFromChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	var err *model.AppError

	team := th.CreateTeam()
	channel := th.CreateChannel(team)
	defer func() {
		th.App.PermanentDeleteChannel(channel)
		th.App.PermanentDeleteTeam(team)
	}()

	_, _, err = th.App.AddUserToTeam(th.Context, team.Id, th.BasicUser.Id, "")
	require.Nil(t, err)

	deacivatedUser := th.CreateUser()
	_, _, err = th.App.AddUserToTeam(th.Context, team.Id, deacivatedUser.Id, "")
	require.Nil(t, err)
	_, err = th.App.AddUserToChannel(deacivatedUser, channel, false)
	require.Nil(t, err)
	channelMembers, err := th.App.GetChannelMembersPage(channel.Id, 0, 10000000)
	require.Nil(t, err)
	require.Len(t, *channelMembers, 2)
	_, err = th.App.UpdateActive(th.Context, deacivatedUser, false)
	require.Nil(t, err)

	err = th.App.RemoveAllDeactivatedMembersFromChannel(channel)
	require.Nil(t, err)

	channelMembers, err = th.App.GetChannelMembersPage(channel.Id, 0, 10000000)
	require.Nil(t, err)
	require.Len(t, *channelMembers, 1)
}

func TestMoveChannel(t *testing.T) {
	t.Run("should move channels between teams", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		var err *model.AppError

		sourceTeam := th.CreateTeam()
		targetTeam := th.CreateTeam()
		channel1 := th.CreateChannel(sourceTeam)
		defer func() {
			th.App.PermanentDeleteChannel(channel1)
			th.App.PermanentDeleteTeam(sourceTeam)
			th.App.PermanentDeleteTeam(targetTeam)
		}()

		_, _, err = th.App.AddUserToTeam(th.Context, sourceTeam.Id, th.BasicUser.Id, "")
		require.Nil(t, err)

		_, _, err = th.App.AddUserToTeam(th.Context, sourceTeam.Id, th.BasicUser2.Id, "")
		require.Nil(t, err)

		_, _, err = th.App.AddUserToTeam(th.Context, targetTeam.Id, th.BasicUser.Id, "")
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(th.BasicUser, channel1, false)
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(th.BasicUser2, channel1, false)
		require.Nil(t, err)

		err = th.App.MoveChannel(th.Context, targetTeam, channel1, th.BasicUser)
		require.NotNil(t, err, "Should have failed due to mismatched members.")

		_, _, err = th.App.AddUserToTeam(th.Context, targetTeam.Id, th.BasicUser2.Id, "")
		require.Nil(t, err)

		err = th.App.MoveChannel(th.Context, targetTeam, channel1, th.BasicUser)
		require.Nil(t, err)

		// Test moving a channel with a deactivated user who isn't in the destination team.
		// It should fail, unless removeDeactivatedMembers is true.
		deacivatedUser := th.CreateUser()
		channel2 := th.CreateChannel(sourceTeam)
		defer th.App.PermanentDeleteChannel(channel2)

		_, _, err = th.App.AddUserToTeam(th.Context, sourceTeam.Id, deacivatedUser.Id, "")
		require.Nil(t, err)
		_, err = th.App.AddUserToChannel(th.BasicUser, channel2, false)
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(deacivatedUser, channel2, false)
		require.Nil(t, err)

		_, err = th.App.UpdateActive(th.Context, deacivatedUser, false)
		require.Nil(t, err)

		err = th.App.MoveChannel(th.Context, targetTeam, channel2, th.BasicUser)
		require.NotNil(t, err, "Should have failed due to mismatched deacivated member.")

		// Test moving a channel with no members.
		channel3 := &model.Channel{
			DisplayName: "dn_" + model.NewId(),
			Name:        "name_" + model.NewId(),
			Type:        model.CHANNEL_OPEN,
			TeamId:      sourceTeam.Id,
			CreatorId:   th.BasicUser.Id,
		}

		channel3, err = th.App.CreateChannel(th.Context, channel3, false)
		require.Nil(t, err)
		defer th.App.PermanentDeleteChannel(channel3)

		err = th.App.MoveChannel(th.Context, targetTeam, channel3, th.BasicUser)
		assert.Nil(t, err)
	})

	t.Run("should remove sidebar entries when moving channels from one team to another", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		sourceTeam := th.CreateTeam()
		targetTeam := th.CreateTeam()
		channel := th.CreateChannel(sourceTeam)

		th.LinkUserToTeam(th.BasicUser, sourceTeam)
		th.LinkUserToTeam(th.BasicUser, targetTeam)
		th.AddUserToChannel(th.BasicUser, channel)

		// Put the channel in a custom category so that it explicitly exists in SidebarChannels
		category, err := th.App.CreateSidebarCategory(th.BasicUser.Id, sourceTeam.Id, &model.SidebarCategoryWithChannels{
			SidebarCategory: model.SidebarCategory{
				DisplayName: "new category",
			},
			Channels: []string{channel.Id},
		})
		require.Nil(t, err)
		require.Equal(t, []string{channel.Id}, category.Channels)

		err = th.App.MoveChannel(th.Context, targetTeam, channel, th.BasicUser)
		require.Nil(t, err)

		moved, err := th.App.GetChannel(channel.Id)
		require.Nil(t, err)
		require.Equal(t, targetTeam.Id, moved.TeamId)

		// The channel should no longer be on the old team
		updatedCategory, err := th.App.GetSidebarCategory(category.Id)
		require.Nil(t, err)
		assert.Equal(t, []string{}, updatedCategory.Channels)

		// And it should be on the new team instead
		categories, err := th.App.GetSidebarCategories(th.BasicUser.Id, targetTeam.Id)
		require.Nil(t, err)
		require.Equal(t, model.SidebarCategoryChannels, categories.Categories[1].Type)
		assert.Contains(t, categories.Categories[1].Channels, channel.Id)
	})
}

func TestRemoveUsersFromChannelNotMemberOfTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team := th.CreateTeam()
	team2 := th.CreateTeam()
	channel1 := th.CreateChannel(team)
	defer func() {
		th.App.PermanentDeleteChannel(channel1)
		th.App.PermanentDeleteTeam(team)
		th.App.PermanentDeleteTeam(team2)
	}()

	_, _, err := th.App.AddUserToTeam(th.Context, team.Id, th.BasicUser.Id, "")
	require.Nil(t, err)
	_, _, err = th.App.AddUserToTeam(th.Context, team2.Id, th.BasicUser.Id, "")
	require.Nil(t, err)
	_, _, err = th.App.AddUserToTeam(th.Context, team.Id, th.BasicUser2.Id, "")
	require.Nil(t, err)

	_, err = th.App.AddUserToChannel(th.BasicUser, channel1, false)
	require.Nil(t, err)
	_, err = th.App.AddUserToChannel(th.BasicUser2, channel1, false)
	require.Nil(t, err)

	err = th.App.RemoveUsersFromChannelNotMemberOfTeam(th.Context, th.SystemAdminUser, channel1, team2)
	require.Nil(t, err)

	channelMembers, err := th.App.GetChannelMembersPage(channel1.Id, 0, 10000000)
	require.Nil(t, err)
	require.Len(t, *channelMembers, 1)
	members := make([]model.ChannelMember, len(*channelMembers))
	for i, m := range *channelMembers {
		members[i] = m
	}
	require.Equal(t, members[0].UserId, th.BasicUser.Id)
}

func TestJoinDefaultChannelsCreatesChannelMemberHistoryRecordTownSquare(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// figure out the initial number of users in town square
	channel, err := th.App.Srv().Store.Channel().GetByName(th.BasicTeam.Id, "town-square", true)
	require.NoError(t, err)
	townSquareChannelId := channel.Id
	users, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, townSquareChannelId)
	require.NoError(t, nErr)
	initialNumTownSquareUsers := len(users)

	// create a new user that joins the default channels
	user := th.CreateUser()
	th.App.JoinDefaultChannels(th.Context, th.BasicTeam.Id, user, false, "")

	// there should be a ChannelMemberHistory record for the user
	histories, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, townSquareChannelId)
	require.NoError(t, nErr)
	assert.Len(t, histories, initialNumTownSquareUsers+1)

	found := false
	for _, history := range histories {
		if user.Id == history.UserId && townSquareChannelId == history.ChannelId {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestJoinDefaultChannelsCreatesChannelMemberHistoryRecordOffTopic(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// figure out the initial number of users in off-topic
	channel, err := th.App.Srv().Store.Channel().GetByName(th.BasicTeam.Id, "off-topic", true)
	require.NoError(t, err)
	offTopicChannelId := channel.Id
	users, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, offTopicChannelId)
	require.NoError(t, nErr)
	initialNumTownSquareUsers := len(users)

	// create a new user that joins the default channels
	user := th.CreateUser()
	th.App.JoinDefaultChannels(th.Context, th.BasicTeam.Id, user, false, "")

	// there should be a ChannelMemberHistory record for the user
	histories, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, offTopicChannelId)
	require.NoError(t, nErr)
	assert.Len(t, histories, initialNumTownSquareUsers+1)

	found := false
	for _, history := range histories {
		if user.Id == history.UserId && offTopicChannelId == history.ChannelId {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestJoinDefaultChannelsExperimentalDefaultChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	basicChannel2 := th.CreateChannel(th.BasicTeam)
	defer th.App.PermanentDeleteChannel(basicChannel2)
	defaultChannelList := []string{th.BasicChannel.Name, basicChannel2.Name, basicChannel2.Name}
	th.App.Config().TeamSettings.ExperimentalDefaultChannels = defaultChannelList

	user := th.CreateUser()
	th.App.JoinDefaultChannels(th.Context, th.BasicTeam.Id, user, false, "")

	for _, channelName := range defaultChannelList {
		channel, err := th.App.GetChannelByName(channelName, th.BasicTeam.Id, false)
		require.Nil(t, err, "Expected nil, didn't receive nil")

		member, err := th.App.GetChannelMember(context.Background(), channel.Id, user.Id)

		require.NotNil(t, member, "Expected member object, got nil")
		require.Nil(t, err, "Expected nil object, didn't receive nil")
	}
}

func TestCreateChannelPublicCreatesChannelMemberHistoryRecord(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// creates a public channel and adds basic user to it
	publicChannel := th.createChannel(th.BasicTeam, model.CHANNEL_OPEN)

	// there should be a ChannelMemberHistory record for the user
	histories, err := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, publicChannel.Id)
	require.NoError(t, err)
	assert.Len(t, histories, 1)
	assert.Equal(t, th.BasicUser.Id, histories[0].UserId)
	assert.Equal(t, publicChannel.Id, histories[0].ChannelId)
}

func TestCreateChannelPrivateCreatesChannelMemberHistoryRecord(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// creates a private channel and adds basic user to it
	privateChannel := th.createChannel(th.BasicTeam, model.CHANNEL_PRIVATE)

	// there should be a ChannelMemberHistory record for the user
	histories, err := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, privateChannel.Id)
	require.NoError(t, err)
	assert.Len(t, histories, 1)
	assert.Equal(t, th.BasicUser.Id, histories[0].UserId)
	assert.Equal(t, privateChannel.Id, histories[0].ChannelId)
}
func TestCreateChannelDisplayNameTrimsWhitespace(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "  Public 1  ", Name: "public1", Type: model.CHANNEL_OPEN, TeamId: th.BasicTeam.Id}, false)
	defer th.App.PermanentDeleteChannel(channel)
	require.Nil(t, err)
	require.Equal(t, channel.DisplayName, "Public 1")
}

func TestUpdateChannelPrivacy(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	privateChannel := th.createChannel(th.BasicTeam, model.CHANNEL_PRIVATE)
	privateChannel.Type = model.CHANNEL_OPEN

	publicChannel, err := th.App.UpdateChannelPrivacy(th.Context, privateChannel, th.BasicUser)
	require.Nil(t, err, "Failed to update channel privacy.")
	assert.Equal(t, publicChannel.Id, privateChannel.Id)
	assert.Equal(t, publicChannel.Type, model.CHANNEL_OPEN)
}

func TestCreateGroupChannelCreatesChannelMemberHistoryRecord(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user1 := th.CreateUser()
	user2 := th.CreateUser()

	groupUserIds := make([]string, 0)
	groupUserIds = append(groupUserIds, user1.Id)
	groupUserIds = append(groupUserIds, user2.Id)
	groupUserIds = append(groupUserIds, th.BasicUser.Id)

	channel, err := th.App.CreateGroupChannel(groupUserIds, th.BasicUser.Id)

	require.Nil(t, err, "Failed to create group channel.")
	histories, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, channel.Id)
	require.NoError(t, nErr)
	assert.Len(t, histories, 3)

	channelMemberHistoryUserIds := make([]string, 0)
	for _, history := range histories {
		assert.Equal(t, channel.Id, history.ChannelId)
		channelMemberHistoryUserIds = append(channelMemberHistoryUserIds, history.UserId)
	}

	sort.Strings(groupUserIds)
	sort.Strings(channelMemberHistoryUserIds)
	assert.Equal(t, groupUserIds, channelMemberHistoryUserIds)
}

func TestCreateDirectChannelCreatesChannelMemberHistoryRecord(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user1 := th.CreateUser()
	user2 := th.CreateUser()

	channel, err := th.App.GetOrCreateDirectChannel(th.Context, user1.Id, user2.Id)
	require.Nil(t, err, "Failed to create direct channel.")

	histories, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, channel.Id)
	require.NoError(t, nErr)
	assert.Len(t, histories, 2)

	historyId0 := histories[0].UserId
	historyId1 := histories[1].UserId
	switch historyId0 {
	case user1.Id:
		assert.Equal(t, user2.Id, historyId1)
	case user2.Id:
		assert.Equal(t, user1.Id, historyId1)
	default:
		require.Fail(t, "Unexpected user id in ChannelMemberHistory table", historyId0)
	}
}

func TestGetDirectChannelCreatesChannelMemberHistoryRecord(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user1 := th.CreateUser()
	user2 := th.CreateUser()

	// this function call implicitly creates a direct channel between the two users if one doesn't already exist
	channel, err := th.App.GetOrCreateDirectChannel(th.Context, user1.Id, user2.Id)
	require.Nil(t, err, "Failed to create direct channel.")

	// there should be a ChannelMemberHistory record for both users
	histories, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, channel.Id)
	require.NoError(t, nErr)
	assert.Len(t, histories, 2)

	historyId0 := histories[0].UserId
	historyId1 := histories[1].UserId
	switch historyId0 {
	case user1.Id:
		assert.Equal(t, user2.Id, historyId1)
	case user2.Id:
		assert.Equal(t, user1.Id, historyId1)
	default:
		require.Fail(t, "Unexpected user id in ChannelMemberHistory table", historyId0)
	}
}

func TestAddUserToChannelCreatesChannelMemberHistoryRecord(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// create a user and add it to a channel
	user := th.CreateUser()
	_, err := th.App.AddTeamMember(th.Context, th.BasicTeam.Id, user.Id)
	require.Nil(t, err, "Failed to add user to team.")

	groupUserIds := make([]string, 0)
	groupUserIds = append(groupUserIds, th.BasicUser.Id)
	groupUserIds = append(groupUserIds, user.Id)

	channel := th.createChannel(th.BasicTeam, model.CHANNEL_OPEN)

	_, err = th.App.AddUserToChannel(user, channel, false)
	require.Nil(t, err, "Failed to add user to channel.")

	// there should be a ChannelMemberHistory record for the user
	histories, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, channel.Id)
	require.NoError(t, nErr)
	assert.Len(t, histories, 2)
	channelMemberHistoryUserIds := make([]string, 0)
	for _, history := range histories {
		assert.Equal(t, channel.Id, history.ChannelId)
		channelMemberHistoryUserIds = append(channelMemberHistoryUserIds, history.UserId)
	}
	assert.Equal(t, groupUserIds, channelMemberHistoryUserIds)
}

func TestLeaveDefaultChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	guest := th.CreateGuest()
	th.LinkUserToTeam(guest, th.BasicTeam)

	townSquare, err := th.App.GetChannelByName("town-square", th.BasicTeam.Id, false)
	require.Nil(t, err)
	th.AddUserToChannel(guest, townSquare)
	th.AddUserToChannel(th.BasicUser, townSquare)

	t.Run("User tries to leave the default channel", func(t *testing.T) {
		err = th.App.LeaveChannel(th.Context, townSquare.Id, th.BasicUser.Id)
		assert.NotNil(t, err, "It should fail to remove a regular user from the default channel")
		assert.Equal(t, err.Id, "api.channel.remove.default.app_error")
		_, err = th.App.GetChannelMember(context.Background(), townSquare.Id, th.BasicUser.Id)
		assert.Nil(t, err)
	})

	t.Run("Guest leaves the default channel", func(t *testing.T) {
		err = th.App.LeaveChannel(th.Context, townSquare.Id, guest.Id)
		assert.Nil(t, err, "It should allow to remove a guest user from the default channel")
		_, err = th.App.GetChannelMember(context.Background(), townSquare.Id, guest.Id)
		assert.NotNil(t, err)
	})
}

func TestLeaveLastChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	guest := th.CreateGuest()
	th.LinkUserToTeam(guest, th.BasicTeam)

	townSquare, err := th.App.GetChannelByName("town-square", th.BasicTeam.Id, false)
	require.Nil(t, err)
	th.AddUserToChannel(guest, townSquare)
	th.AddUserToChannel(guest, th.BasicChannel)

	t.Run("Guest leaves not last channel", func(t *testing.T) {
		err = th.App.LeaveChannel(th.Context, townSquare.Id, guest.Id)
		require.Nil(t, err)
		_, err = th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		assert.Nil(t, err, "It should maintain the team membership")
	})

	t.Run("Guest leaves last channel", func(t *testing.T) {
		err = th.App.LeaveChannel(th.Context, th.BasicChannel.Id, guest.Id)
		assert.Nil(t, err, "It should allow to remove a guest user from the default channel")
		_, err = th.App.GetChannelMember(context.Background(), th.BasicChannel.Id, guest.Id)
		assert.NotNil(t, err)
		_, err = th.App.GetTeamMember(th.BasicTeam.Id, guest.Id)
		assert.Nil(t, err, "It should remove the team membership")
	})
}

func TestAddChannelMemberNoUserRequestor(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// create a user and add it to a channel
	user := th.CreateUser()
	_, err := th.App.AddTeamMember(th.Context, th.BasicTeam.Id, user.Id)
	require.Nil(t, err)

	groupUserIds := make([]string, 0)
	groupUserIds = append(groupUserIds, th.BasicUser.Id)
	groupUserIds = append(groupUserIds, user.Id)

	channel := th.createChannel(th.BasicTeam, model.CHANNEL_OPEN)

	_, err = th.App.AddChannelMember(th.Context, user.Id, channel, ChannelMemberOpts{})
	require.Nil(t, err, "Failed to add user to channel.")

	// there should be a ChannelMemberHistory record for the user
	histories, nErr := th.App.Srv().Store.ChannelMemberHistory().GetUsersInChannelDuring(model.GetMillis()-100, model.GetMillis()+100, channel.Id)
	require.NoError(t, nErr)
	assert.Len(t, histories, 2)
	channelMemberHistoryUserIds := make([]string, 0)
	for _, history := range histories {
		assert.Equal(t, channel.Id, history.ChannelId)
		channelMemberHistoryUserIds = append(channelMemberHistoryUserIds, history.UserId)
	}
	assert.Equal(t, groupUserIds, channelMemberHistoryUserIds)

	postList, nErr := th.App.Srv().Store.Post().GetPosts(model.GetPostsOptions{ChannelId: channel.Id, Page: 0, PerPage: 1}, false)
	require.NoError(t, nErr)

	if assert.Len(t, postList.Order, 1) {
		post := postList.Posts[postList.Order[0]]

		assert.Equal(t, model.POST_JOIN_CHANNEL, post.Type)
		assert.Equal(t, user.Id, post.UserId)
		assert.Equal(t, user.Username, post.GetProp("username"))
	}
}

func TestAppUpdateChannelScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel
	mockID := model.NewString("x")
	channel.SchemeId = mockID

	updatedChannel, err := th.App.UpdateChannelScheme(channel)
	require.Nil(t, err)

	if updatedChannel.SchemeId != mockID {
		require.Fail(t, "Wrong Channel SchemeId")
	}
}

func TestSetChannelsMuted(t *testing.T) {
	t.Run("should mute and unmute the given channels", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		channel1 := th.BasicChannel

		channel2 := th.CreateChannel(th.BasicTeam)
		th.AddUserToChannel(th.BasicUser, channel2)

		// Ensure that both channels start unmuted
		member1, err := th.App.GetChannelMember(context.Background(), channel1.Id, th.BasicUser.Id)
		require.Nil(t, err)
		require.False(t, member1.IsChannelMuted())

		member2, err := th.App.GetChannelMember(context.Background(), channel2.Id, th.BasicUser.Id)
		require.Nil(t, err)
		require.False(t, member2.IsChannelMuted())

		// Mute both channels
		updated, err := th.App.setChannelsMuted([]string{channel1.Id, channel2.Id}, th.BasicUser.Id, true)
		require.Nil(t, err)
		assert.True(t, updated[0].IsChannelMuted())
		assert.True(t, updated[1].IsChannelMuted())

		// Verify that the channels are muted in the database
		member1, err = th.App.GetChannelMember(context.Background(), channel1.Id, th.BasicUser.Id)
		require.Nil(t, err)
		require.True(t, member1.IsChannelMuted())

		member2, err = th.App.GetChannelMember(context.Background(), channel2.Id, th.BasicUser.Id)
		require.Nil(t, err)
		require.True(t, member2.IsChannelMuted())

		// Unm both channels
		updated, err = th.App.setChannelsMuted([]string{channel1.Id, channel2.Id}, th.BasicUser.Id, false)
		require.Nil(t, err)
		assert.False(t, updated[0].IsChannelMuted())
		assert.False(t, updated[1].IsChannelMuted())

		// Verify that the channels are muted in the database
		member1, err = th.App.GetChannelMember(context.Background(), channel1.Id, th.BasicUser.Id)
		require.Nil(t, err)
		require.False(t, member1.IsChannelMuted())

		member2, err = th.App.GetChannelMember(context.Background(), channel2.Id, th.BasicUser.Id)
		require.Nil(t, err)
		require.False(t, member2.IsChannelMuted())
	})
}

func TestFillInChannelProps(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channelPublic1, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "Public 1", Name: "public1", Type: model.CHANNEL_OPEN, TeamId: th.BasicTeam.Id}, false)
	require.Nil(t, err)
	defer th.App.PermanentDeleteChannel(channelPublic1)

	channelPublic2, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "Public 2", Name: "public2", Type: model.CHANNEL_OPEN, TeamId: th.BasicTeam.Id}, false)
	require.Nil(t, err)
	defer th.App.PermanentDeleteChannel(channelPublic2)

	channelPrivate, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "Private", Name: "private", Type: model.CHANNEL_PRIVATE, TeamId: th.BasicTeam.Id}, false)
	require.Nil(t, err)
	defer th.App.PermanentDeleteChannel(channelPrivate)

	otherTeamId := model.NewId()
	otherTeam := &model.Team{
		DisplayName: "dn_" + otherTeamId,
		Name:        "name" + otherTeamId,
		Email:       "success+" + otherTeamId + "@simulator.amazonses.com",
		Type:        model.TEAM_OPEN,
	}
	otherTeam, err = th.App.CreateTeam(th.Context, otherTeam)
	require.Nil(t, err)
	defer th.App.PermanentDeleteTeam(otherTeam)

	channelOtherTeam, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "Other Team Channel", Name: "other-team", Type: model.CHANNEL_OPEN, TeamId: otherTeam.Id}, false)
	require.Nil(t, err)
	defer th.App.PermanentDeleteChannel(channelOtherTeam)

	// Note that purpose is intentionally plaintext below.

	t.Run("single channels", func(t *testing.T) {
		testCases := []struct {
			Description          string
			Channel              *model.Channel
			ExpectedChannelProps map[string]interface{}
		}{
			{
				"channel on basic team without references",
				&model.Channel{
					TeamId:  th.BasicTeam.Id,
					Header:  "No references",
					Purpose: "No references",
				},
				nil,
			},
			{
				"channel on basic team",
				&model.Channel{
					TeamId:  th.BasicTeam.Id,
					Header:  "~public1, ~private, ~other-team",
					Purpose: "~public2, ~private, ~other-team",
				},
				map[string]interface{}{
					"channel_mentions": map[string]interface{}{
						"public1": map[string]interface{}{
							"display_name": "Public 1",
						},
					},
				},
			},
			{
				"channel on other team",
				&model.Channel{
					TeamId:  otherTeam.Id,
					Header:  "~public1, ~private, ~other-team",
					Purpose: "~public2, ~private, ~other-team",
				},
				map[string]interface{}{
					"channel_mentions": map[string]interface{}{
						"other-team": map[string]interface{}{
							"display_name": "Other Team Channel",
						},
					},
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.Description, func(t *testing.T) {
				err = th.App.FillInChannelProps(testCase.Channel)
				require.Nil(t, err)

				assert.Equal(t, testCase.ExpectedChannelProps, testCase.Channel.Props)
			})
		}
	})

	t.Run("multiple channels", func(t *testing.T) {
		testCases := []struct {
			Description          string
			Channels             *model.ChannelList
			ExpectedChannelProps map[string]interface{}
		}{
			{
				"single channel on basic team",
				&model.ChannelList{
					{
						Name:    "test",
						TeamId:  th.BasicTeam.Id,
						Header:  "~public1, ~private, ~other-team",
						Purpose: "~public2, ~private, ~other-team",
					},
				},
				map[string]interface{}{
					"test": map[string]interface{}{
						"channel_mentions": map[string]interface{}{
							"public1": map[string]interface{}{
								"display_name": "Public 1",
							},
						},
					},
				},
			},
			{
				"multiple channels on basic team",
				&model.ChannelList{
					{
						Name:    "test",
						TeamId:  th.BasicTeam.Id,
						Header:  "~public1, ~private, ~other-team",
						Purpose: "~public2, ~private, ~other-team",
					},
					{
						Name:    "test2",
						TeamId:  th.BasicTeam.Id,
						Header:  "~private, ~other-team",
						Purpose: "~public2, ~private, ~other-team",
					},
					{
						Name:    "test3",
						TeamId:  th.BasicTeam.Id,
						Header:  "No references",
						Purpose: "No references",
					},
				},
				map[string]interface{}{
					"test": map[string]interface{}{
						"channel_mentions": map[string]interface{}{
							"public1": map[string]interface{}{
								"display_name": "Public 1",
							},
						},
					},
					"test2": map[string]interface{}(nil),
					"test3": map[string]interface{}(nil),
				},
			},
			{
				"multiple channels across teams",
				&model.ChannelList{
					{
						Name:    "test",
						TeamId:  th.BasicTeam.Id,
						Header:  "~public1, ~private, ~other-team",
						Purpose: "~public2, ~private, ~other-team",
					},
					{
						Name:    "test2",
						TeamId:  otherTeam.Id,
						Header:  "~private, ~other-team",
						Purpose: "~public2, ~private, ~other-team",
					},
					{
						Name:    "test3",
						TeamId:  th.BasicTeam.Id,
						Header:  "No references",
						Purpose: "No references",
					},
				},
				map[string]interface{}{
					"test": map[string]interface{}{
						"channel_mentions": map[string]interface{}{
							"public1": map[string]interface{}{
								"display_name": "Public 1",
							},
						},
					},
					"test2": map[string]interface{}{
						"channel_mentions": map[string]interface{}{
							"other-team": map[string]interface{}{
								"display_name": "Other Team Channel",
							},
						},
					},
					"test3": map[string]interface{}(nil),
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.Description, func(t *testing.T) {
				err = th.App.FillInChannelsProps(testCase.Channels)
				require.Nil(t, err)

				for _, channel := range *testCase.Channels {
					assert.Equal(t, testCase.ExpectedChannelProps[channel.Name], channel.Props)
				}
			})
		}
	})
}

func TestRenameChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	testCases := []struct {
		Name                string
		Channel             *model.Channel
		ExpectError         bool
		ChannelName         string
		ExpectedName        string
		ExpectedDisplayName string
	}{
		{
			"Rename open channel",
			th.createChannel(th.BasicTeam, model.CHANNEL_OPEN),
			false,
			"newchannelname",
			"newchannelname",
			"New Display Name",
		},
		{
			"Fail on rename open channel with bad name",
			th.createChannel(th.BasicTeam, model.CHANNEL_OPEN),
			true,
			"6zii9a9g6pruzj451x3esok54h__wr4j4g8zqtnhmkw771pfpynqwo",
			"",
			"",
		},
		{
			"Success on rename open channel with consecutive underscores in name",
			th.createChannel(th.BasicTeam, model.CHANNEL_OPEN),
			false,
			"foo__bar",
			"foo__bar",
			"New Display Name",
		},
		{
			"Fail on rename direct message channel",
			th.CreateDmChannel(th.BasicUser2),
			true,
			"newchannelname",
			"",
			"",
		},
		{
			"Fail on rename group message channel",
			th.CreateGroupChannel(th.BasicUser2, th.CreateUser()),
			true,
			"newchannelname",
			"",
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			channel, err := th.App.RenameChannel(tc.Channel, tc.ChannelName, "New Display Name")
			if tc.ExpectError {
				assert.NotNil(t, err)
			} else {
				assert.Equal(t, tc.ExpectedName, channel.Name)
				assert.Equal(t, tc.ExpectedDisplayName, channel.DisplayName)
			}
		})
	}
}

func TestGetChannelMembersTimezones(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	_, err := th.App.AddChannelMember(th.Context, th.BasicUser2.Id, th.BasicChannel, ChannelMemberOpts{})
	require.Nil(t, err, "Failed to add user to channel.")

	user := th.BasicUser
	user.Timezone["useAutomaticTimezone"] = "false"
	user.Timezone["manualTimezone"] = "XOXO/BLABLA"
	th.App.UpdateUser(user, false)

	user2 := th.BasicUser2
	user2.Timezone["automaticTimezone"] = "NoWhere/Island"
	th.App.UpdateUser(user2, false)

	user3 := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
	ruser, _ := th.App.CreateUser(th.Context, &user3)
	th.App.AddUserToChannel(ruser, th.BasicChannel, false)

	ruser.Timezone["automaticTimezone"] = "NoWhere/Island"
	th.App.UpdateUser(ruser, false)

	user4 := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
	ruser, _ = th.App.CreateUser(th.Context, &user4)
	th.App.AddUserToChannel(ruser, th.BasicChannel, false)

	timezones, err := th.App.GetChannelMembersTimezones(th.BasicChannel.Id)
	require.Nil(t, err, "Failed to get the timezones for a channel.")

	assert.Equal(t, 2, len(timezones))
}

func TestGetChannelsForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	channel := &model.Channel{
		DisplayName: "Public",
		Name:        "public",
		Type:        model.CHANNEL_OPEN,
		CreatorId:   th.BasicUser.Id,
		TeamId:      th.BasicTeam.Id,
	}
	th.App.CreateChannel(th.Context, channel, true)
	defer th.App.PermanentDeleteChannel(channel)
	defer th.TearDown()

	channelList, err := th.App.GetChannelsForUser(th.BasicTeam.Id, th.BasicUser.Id, false, 0)
	require.Nil(t, err)
	require.Len(t, *channelList, 4)

	th.App.DeleteChannel(th.Context, channel, th.BasicUser.Id)

	// Now we get all the non-archived channels for the user
	channelList, err = th.App.GetChannelsForUser(th.BasicTeam.Id, th.BasicUser.Id, false, 0)
	require.Nil(t, err)
	require.Len(t, *channelList, 3)

	// Now we get all the channels, even though are archived, for the user
	channelList, err = th.App.GetChannelsForUser(th.BasicTeam.Id, th.BasicUser.Id, true, 0)
	require.Nil(t, err)
	require.Len(t, *channelList, 4)
}

func TestGetPublicChannelsForTeam(t *testing.T) {
	th := Setup(t)
	team := th.CreateTeam()
	defer th.TearDown()

	var expectedChannels []*model.Channel

	townSquare, err := th.App.GetChannelByName("town-square", team.Id, false)
	require.Nil(t, err)
	require.NotNil(t, townSquare)
	expectedChannels = append(expectedChannels, townSquare)

	offTopic, err := th.App.GetChannelByName("off-topic", team.Id, false)
	require.Nil(t, err)
	require.NotNil(t, offTopic)
	expectedChannels = append(expectedChannels, offTopic)

	for i := 0; i < 8; i++ {
		channel := model.Channel{
			DisplayName: fmt.Sprintf("Public %v", i),
			Name:        fmt.Sprintf("public_%v", i),
			Type:        model.CHANNEL_OPEN,
			TeamId:      team.Id,
		}
		var rchannel *model.Channel
		rchannel, err = th.App.CreateChannel(th.Context, &channel, false)
		require.Nil(t, err)
		require.NotNil(t, rchannel)
		defer th.App.PermanentDeleteChannel(rchannel)

		// Store the user ids for comparison later
		expectedChannels = append(expectedChannels, rchannel)
	}

	// Fetch public channels multipile times
	channelList, err := th.App.GetPublicChannelsForTeam(team.Id, 0, 5)
	require.Nil(t, err)
	channelList2, err := th.App.GetPublicChannelsForTeam(team.Id, 5, 5)
	require.Nil(t, err)

	channels := append(*channelList, *channelList2...)
	assert.ElementsMatch(t, expectedChannels, channels)
}

func TestGetPrivateChannelsForTeam(t *testing.T) {
	th := Setup(t)
	team := th.CreateTeam()
	defer th.TearDown()

	var expectedChannels []*model.Channel
	for i := 0; i < 8; i++ {
		channel := model.Channel{
			DisplayName: fmt.Sprintf("Private %v", i),
			Name:        fmt.Sprintf("private_%v", i),
			Type:        model.CHANNEL_PRIVATE,
			TeamId:      team.Id,
		}
		var rchannel *model.Channel
		rchannel, err := th.App.CreateChannel(th.Context, &channel, false)
		require.Nil(t, err)
		require.NotNil(t, rchannel)
		defer th.App.PermanentDeleteChannel(rchannel)

		// Store the user ids for comparison later
		expectedChannels = append(expectedChannels, rchannel)
	}

	// Fetch private channels multipile times
	channelList, err := th.App.GetPrivateChannelsForTeam(team.Id, 0, 5)
	require.Nil(t, err)
	channelList2, err := th.App.GetPrivateChannelsForTeam(team.Id, 5, 5)
	require.Nil(t, err)

	channels := append(*channelList, *channelList2...)
	assert.ElementsMatch(t, expectedChannels, channels)
}

func TestUpdateChannelMemberRolesChangingGuest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("from guest to user", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateGuest(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, ruser.Id, "")
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(ruser, th.BasicChannel, false)
		require.Nil(t, err)

		_, err = th.App.UpdateChannelMemberRoles(th.BasicChannel.Id, ruser.Id, "channel_user")
		require.NotNil(t, err, "Should fail when try to modify the guest role")
	})

	t.Run("from user to guest", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, ruser.Id, "")
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(ruser, th.BasicChannel, false)
		require.Nil(t, err)

		_, err = th.App.UpdateChannelMemberRoles(th.BasicChannel.Id, ruser.Id, "channel_guest")
		require.NotNil(t, err, "Should fail when try to modify the guest role")
	})

	t.Run("from user to admin", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateUser(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, ruser.Id, "")
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(ruser, th.BasicChannel, false)
		require.Nil(t, err)

		_, err = th.App.UpdateChannelMemberRoles(th.BasicChannel.Id, ruser.Id, "channel_user channel_admin")
		require.Nil(t, err, "Should work when you not modify guest role")
	})

	t.Run("from guest to guest plus custom", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateGuest(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, ruser.Id, "")
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(ruser, th.BasicChannel, false)
		require.Nil(t, err)

		_, err = th.App.CreateRole(&model.Role{Name: "custom", DisplayName: "custom", Description: "custom"})
		require.Nil(t, err)

		_, err = th.App.UpdateChannelMemberRoles(th.BasicChannel.Id, ruser.Id, "channel_guest custom")
		require.Nil(t, err, "Should work when you not modify guest role")
	})

	t.Run("a guest cant have user role", func(t *testing.T) {
		user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
		ruser, _ := th.App.CreateGuest(th.Context, &user)

		_, _, err := th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, ruser.Id, "")
		require.Nil(t, err)

		_, err = th.App.AddUserToChannel(ruser, th.BasicChannel, false)
		require.Nil(t, err)

		_, err = th.App.UpdateChannelMemberRoles(th.BasicChannel.Id, ruser.Id, "channel_guest channel_user")
		require.NotNil(t, err, "Should work when you not modify guest role")
	})
}

func TestDefaultChannelNames(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	actual := th.App.DefaultChannelNames()
	expect := []string{"town-square", "off-topic"}
	require.ElementsMatch(t, expect, actual)

	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.TeamSettings.ExperimentalDefaultChannels = []string{"foo", "bar"}
	})

	actual = th.App.DefaultChannelNames()
	expect = []string{"town-square", "foo", "bar"}
	require.ElementsMatch(t, expect, actual)
}

func TestSearchChannelsForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	c1, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "test-dev-1", Name: "test-dev-1", Type: model.CHANNEL_OPEN, TeamId: th.BasicTeam.Id}, false)
	require.Nil(t, err)

	c2, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "test-dev-2", Name: "test-dev-2", Type: model.CHANNEL_OPEN, TeamId: th.BasicTeam.Id}, false)
	require.Nil(t, err)

	c3, err := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "dev-3", Name: "dev-3", Type: model.CHANNEL_OPEN, TeamId: th.BasicTeam.Id}, false)
	require.Nil(t, err)

	defer func() {
		th.App.PermanentDeleteChannel(c1)
		th.App.PermanentDeleteChannel(c2)
		th.App.PermanentDeleteChannel(c3)
	}()

	// add user to test-dev-1 and dev3
	_, err = th.App.AddUserToChannel(th.BasicUser, c1, false)
	require.Nil(t, err)
	_, err = th.App.AddUserToChannel(th.BasicUser, c3, false)
	require.Nil(t, err)

	searchAndCheck := func(t *testing.T, term string, expectedDisplayNames []string) {
		res, searchErr := th.App.SearchChannelsForUser(th.BasicUser.Id, th.BasicTeam.Id, term)
		require.Nil(t, searchErr)
		require.Len(t, *res, len(expectedDisplayNames))

		resultDisplayNames := []string{}
		for _, c := range *res {
			resultDisplayNames = append(resultDisplayNames, c.Name)
		}
		require.ElementsMatch(t, expectedDisplayNames, resultDisplayNames)
	}

	t.Run("Search for test, only test-dev-1 should be returned", func(t *testing.T) {
		searchAndCheck(t, "test", []string{"test-dev-1"})
	})

	t.Run("Search for dev, both test-dev-1 and dev-3 should be returned", func(t *testing.T) {
		searchAndCheck(t, "dev", []string{"test-dev-1", "dev-3"})
	})

	t.Run("After adding user to test-dev-2, search for dev, the three channels should be returned", func(t *testing.T) {
		_, err = th.App.AddUserToChannel(th.BasicUser, c2, false)
		require.Nil(t, err)

		searchAndCheck(t, "dev", []string{"test-dev-1", "test-dev-2", "dev-3"})
	})
}

func TestMarkChannelAsUnreadFromPost(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	u1 := th.BasicUser
	u2 := th.BasicUser2
	c1 := th.BasicChannel
	pc1 := th.CreatePrivateChannel(th.BasicTeam)
	th.AddUserToChannel(u2, c1)
	th.AddUserToChannel(u1, pc1)
	th.AddUserToChannel(u2, pc1)

	p1 := th.CreatePost(c1)
	p2 := th.CreatePost(c1)
	p3 := th.CreatePost(c1)

	pp1 := th.CreatePost(pc1)
	require.NotNil(t, pp1)
	pp2 := th.CreatePost(pc1)

	unread, err := th.App.GetChannelUnread(c1.Id, u1.Id)
	require.Nil(t, err)
	require.Equal(t, int64(4), unread.MsgCount)
	unread, err = th.App.GetChannelUnread(c1.Id, u2.Id)
	require.Nil(t, err)
	require.Equal(t, int64(4), unread.MsgCount)
	err = th.App.UpdateChannelLastViewedAt([]string{c1.Id, pc1.Id}, u1.Id)
	require.Nil(t, err)
	err = th.App.UpdateChannelLastViewedAt([]string{c1.Id, pc1.Id}, u2.Id)
	require.Nil(t, err)
	unread, err = th.App.GetChannelUnread(c1.Id, u2.Id)
	require.Nil(t, err)
	require.Equal(t, int64(0), unread.MsgCount)

	t.Run("Unread but last one", func(t *testing.T) {
		response, err := th.App.MarkChannelAsUnreadFromPost(p2.Id, u1.Id, true, true)
		require.Nil(t, err)
		require.NotNil(t, response)
		assert.Equal(t, int64(2), response.MsgCount)
		unread, err := th.App.GetChannelUnread(c1.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(2), unread.MsgCount)
		assert.Equal(t, p2.CreateAt-1, response.LastViewedAt)
	})

	t.Run("Unread last one", func(t *testing.T) {
		response, err := th.App.MarkChannelAsUnreadFromPost(p3.Id, u1.Id, true, true)
		require.Nil(t, err)
		require.NotNil(t, response)
		assert.Equal(t, int64(3), response.MsgCount)
		unread, err := th.App.GetChannelUnread(c1.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(1), unread.MsgCount)
		assert.Equal(t, p3.CreateAt-1, response.LastViewedAt)
	})

	t.Run("Unread first one", func(t *testing.T) {
		response, err := th.App.MarkChannelAsUnreadFromPost(p1.Id, u1.Id, true, true)
		require.Nil(t, err)
		require.NotNil(t, response)
		assert.Equal(t, int64(1), response.MsgCount)
		unread, err := th.App.GetChannelUnread(c1.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(3), unread.MsgCount)
		assert.Equal(t, p1.CreateAt-1, response.LastViewedAt)
	})

	t.Run("Other users are unaffected", func(t *testing.T) {
		unread, err := th.App.GetChannelUnread(c1.Id, u2.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(0), unread.MsgCount)
	})

	t.Run("Unread on a private channel", func(t *testing.T) {
		response, err := th.App.MarkChannelAsUnreadFromPost(pp1.Id, u1.Id, true, true)
		require.Nil(t, err)
		require.NotNil(t, response)
		assert.Equal(t, int64(0), response.MsgCount)
		unread, err := th.App.GetChannelUnread(pc1.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(2), unread.MsgCount)
		assert.Equal(t, pp1.CreateAt-1, response.LastViewedAt)

		response, err = th.App.MarkChannelAsUnreadFromPost(pp2.Id, u1.Id, true, true)
		assert.Nil(t, err)
		assert.Equal(t, int64(1), response.MsgCount)
		unread, err = th.App.GetChannelUnread(pc1.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(1), unread.MsgCount)
		assert.Equal(t, pp2.CreateAt-1, response.LastViewedAt)
	})

	t.Run("Unread with mentions", func(t *testing.T) {
		c2 := th.CreateChannel(th.BasicTeam)
		_, err := th.App.AddUserToChannel(u2, c2, false)
		require.Nil(t, err)

		p4, err := th.App.CreatePost(th.Context, &model.Post{
			UserId:    u2.Id,
			ChannelId: c2.Id,
			Message:   "@" + u1.Username,
		}, c2, false, true)
		require.Nil(t, err)
		th.CreatePost(c2)

		th.App.CreatePost(th.Context, &model.Post{
			UserId:    u2.Id,
			ChannelId: c2.Id,
			RootId:    p4.Id,
			Message:   "@" + u1.Username,
		}, c2, false, true)

		response, err := th.App.MarkChannelAsUnreadFromPost(p4.Id, u1.Id, true, true)
		assert.Nil(t, err)
		assert.Equal(t, int64(1), response.MsgCount)
		assert.Equal(t, int64(2), response.MentionCount)
		assert.Equal(t, int64(1), response.MentionCountRoot)

		unread, err := th.App.GetChannelUnread(c2.Id, u1.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(2), unread.MsgCount)
		assert.Equal(t, int64(2), unread.MentionCount)
		assert.Equal(t, int64(1), unread.MentionCountRoot)
	})

	t.Run("Unread on a DM channel", func(t *testing.T) {
		dc := th.CreateDmChannel(u2)

		dm1 := th.CreatePost(dc)
		th.CreatePost(dc)
		th.CreatePost(dc)

		_, err := th.App.CreatePost(th.Context, &model.Post{ChannelId: dc.Id, UserId: th.BasicUser.Id, Message: "testReply", RootId: dm1.Id}, dc, false, false)
		assert.Nil(t, err)

		response, err := th.App.MarkChannelAsUnreadFromPost(dm1.Id, u2.Id, true, true)
		assert.Nil(t, err)
		assert.Equal(t, int64(0), response.MsgCount)
		assert.Equal(t, int64(4), response.MentionCount)
		assert.Equal(t, int64(3), response.MentionCountRoot)

		unread, err := th.App.GetChannelUnread(dc.Id, u2.Id)
		require.Nil(t, err)
		assert.Equal(t, int64(4), unread.MsgCount)
		assert.Equal(t, int64(4), unread.MentionCount)
		assert.Equal(t, int64(3), unread.MentionCountRoot)
	})

	t.Run("Can't unread an imaginary post", func(t *testing.T) {
		response, err := th.App.MarkChannelAsUnreadFromPost("invalid4ofngungryquinj976y", u1.Id, true, true)
		assert.NotNil(t, err)
		assert.Nil(t, response)
	})
}

func TestAddUserToChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user1 := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
	ruser1, _ := th.App.CreateUser(th.Context, &user1)
	defer th.App.PermanentDeleteUser(th.Context, &user1)
	bot := th.CreateBot()
	botUser, _ := th.App.GetUser(bot.UserId)
	defer th.App.PermanentDeleteBot(th.Context, botUser.Id)

	th.App.AddTeamMember(th.Context, th.BasicTeam.Id, ruser1.Id)
	th.App.AddTeamMember(th.Context, th.BasicTeam.Id, bot.UserId)

	group := th.CreateGroup()

	_, err := th.App.UpsertGroupMember(group.Id, user1.Id)
	require.Nil(t, err)

	gs, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
		AutoAdd:     true,
		SyncableId:  th.BasicChannel.Id,
		Type:        model.GroupSyncableTypeChannel,
		GroupId:     group.Id,
		SchemeAdmin: false,
	})
	require.Nil(t, err)

	err = th.App.JoinChannel(th.Context, th.BasicChannel, ruser1.Id)
	require.Nil(t, err)

	// verify user was added as a non-admin
	cm1, err := th.App.GetChannelMember(context.Background(), th.BasicChannel.Id, ruser1.Id)
	require.Nil(t, err)
	require.False(t, cm1.SchemeAdmin)

	user2 := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
	ruser2, _ := th.App.CreateUser(th.Context, &user2)
	defer th.App.PermanentDeleteUser(th.Context, &user2)
	th.App.AddTeamMember(th.Context, th.BasicTeam.Id, ruser2.Id)

	_, err = th.App.UpsertGroupMember(group.Id, user2.Id)
	require.Nil(t, err)

	gs.SchemeAdmin = true
	_, err = th.App.UpdateGroupSyncable(gs)
	require.Nil(t, err)

	err = th.App.JoinChannel(th.Context, th.BasicChannel, ruser2.Id)
	require.Nil(t, err)

	// Should allow a bot to be added to a public group synced channel
	_, err = th.App.AddUserToChannel(botUser, th.BasicChannel, false)
	require.Nil(t, err)

	// verify user was added as an admin
	cm2, err := th.App.GetChannelMember(context.Background(), th.BasicChannel.Id, ruser2.Id)
	require.Nil(t, err)
	require.True(t, cm2.SchemeAdmin)

	privateChannel := th.CreatePrivateChannel(th.BasicTeam)
	privateChannel.GroupConstrained = model.NewBool(true)
	_, err = th.App.UpdateChannel(privateChannel)
	require.Nil(t, err)

	_, err = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupId:    group.Id,
		SyncableId: privateChannel.Id,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.Nil(t, err)

	// Should allow a group synced user to be added to a group synced private channel
	_, err = th.App.AddUserToChannel(ruser1, privateChannel, false)
	require.Nil(t, err)

	// Should allow a bot to be added to a private group synced channel
	_, err = th.App.AddUserToChannel(botUser, privateChannel, false)
	require.Nil(t, err)
}

func TestRemoveUserFromChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@example.com", Nickname: "Darth Vader", Username: "vader" + model.NewId(), Password: "passwd1", AuthService: ""}
	ruser, _ := th.App.CreateUser(th.Context, &user)
	defer th.App.PermanentDeleteUser(th.Context, ruser)

	bot := th.CreateBot()
	botUser, _ := th.App.GetUser(bot.UserId)
	defer th.App.PermanentDeleteBot(th.Context, botUser.Id)

	th.App.AddTeamMember(th.Context, th.BasicTeam.Id, ruser.Id)
	th.App.AddTeamMember(th.Context, th.BasicTeam.Id, bot.UserId)

	privateChannel := th.CreatePrivateChannel(th.BasicTeam)

	_, err := th.App.AddUserToChannel(ruser, privateChannel, false)
	require.Nil(t, err)
	_, err = th.App.AddUserToChannel(botUser, privateChannel, false)
	require.Nil(t, err)

	group := th.CreateGroup()
	_, err = th.App.UpsertGroupMember(group.Id, ruser.Id)
	require.Nil(t, err)

	_, err = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupId:    group.Id,
		SyncableId: privateChannel.Id,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.Nil(t, err)

	privateChannel.GroupConstrained = model.NewBool(true)
	_, err = th.App.UpdateChannel(privateChannel)
	require.Nil(t, err)

	// Should not allow a group synced user to be removed from channel
	err = th.App.RemoveUserFromChannel(th.Context, ruser.Id, th.SystemAdminUser.Id, privateChannel)
	assert.Equal(t, err.Id, "api.channel.remove_members.denied")

	// Should allow a user to remove themselves from group synced channel
	err = th.App.RemoveUserFromChannel(th.Context, ruser.Id, ruser.Id, privateChannel)
	require.Nil(t, err)

	// Should allow a bot to be removed from a group synced channel
	err = th.App.RemoveUserFromChannel(th.Context, botUser.Id, th.SystemAdminUser.Id, privateChannel)
	require.Nil(t, err)
}

func TestPatchChannelModerationsForChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.SetPhase2PermissionsMigrationStatus(true)
	channel := th.BasicChannel

	user := th.BasicUser
	th.AddUserToChannel(user, channel)

	createPosts := model.ChannelModeratedPermissions[0]
	createReactions := model.ChannelModeratedPermissions[1]
	manageMembers := model.ChannelModeratedPermissions[2]
	channelMentions := model.ChannelModeratedPermissions[3]

	nonChannelModeratedPermission := model.PERMISSION_CREATE_BOT.Id

	testCases := []struct {
		Name                          string
		ChannelModerationsPatch       []*model.ChannelModerationPatch
		PermissionsModeratedByPatch   map[string]*model.ChannelModeratedRoles
		RevertChannelModerationsPatch []*model.ChannelModerationPatch
		HigherScopedMemberPermissions []string
		HigherScopedGuestPermissions  []string
		ShouldError                   bool
		ShouldHaveNoChannelScheme     bool
	}{
		{
			Name: "Removing create posts from members role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &createPosts,
					Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(false)},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{
				createPosts: {
					Members: &model.ChannelModeratedRole{Value: false, Enabled: true},
				},
			},
			RevertChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &createPosts,
					Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(true)},
				},
			},
		},
		{
			Name: "Removing create reactions from members role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &createReactions,
					Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(false)},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{
				createReactions: {
					Members: &model.ChannelModeratedRole{Value: false, Enabled: true},
				},
			},
			RevertChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &createReactions,
					Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(true)},
				},
			},
		},
		{
			Name: "Removing channel mentions from members role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &channelMentions,
					Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(false)},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{
				channelMentions: {
					Members: &model.ChannelModeratedRole{Value: false, Enabled: true},
				},
			},
			RevertChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &channelMentions,
					Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(true)},
				},
			},
		},
		{
			Name: "Removing manage members from members role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &manageMembers,
					Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(false)},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{
				manageMembers: {
					Members: &model.ChannelModeratedRole{Value: false, Enabled: true},
				},
			},
			RevertChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &manageMembers,
					Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(true)},
				},
			},
		},
		{
			Name: "Removing create posts from guests role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &createPosts,
					Roles: &model.ChannelModeratedRolesPatch{Guests: model.NewBool(false)},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{
				createPosts: {
					Guests: &model.ChannelModeratedRole{Value: false, Enabled: true},
				},
			},
			RevertChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &createPosts,
					Roles: &model.ChannelModeratedRolesPatch{Guests: model.NewBool(true)},
				},
			},
		},
		{
			Name: "Removing create reactions from guests role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &createReactions,
					Roles: &model.ChannelModeratedRolesPatch{Guests: model.NewBool(false)},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{
				createReactions: {
					Guests: &model.ChannelModeratedRole{Value: false, Enabled: true},
				},
			},
			RevertChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &createReactions,
					Roles: &model.ChannelModeratedRolesPatch{Guests: model.NewBool(true)},
				},
			},
		},
		{
			Name: "Removing channel mentions from guests role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &channelMentions,
					Roles: &model.ChannelModeratedRolesPatch{Guests: model.NewBool(false)},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{
				channelMentions: {
					Guests: &model.ChannelModeratedRole{Value: false, Enabled: true},
				},
			},
			RevertChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &channelMentions,
					Roles: &model.ChannelModeratedRolesPatch{Guests: model.NewBool(true)},
				},
			},
		},
		{
			Name: "Removing manage members from guests role should not error",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name:  &manageMembers,
					Roles: &model.ChannelModeratedRolesPatch{Guests: model.NewBool(false)},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{},
			ShouldError:                 false,
			ShouldHaveNoChannelScheme:   true,
		},
		{
			Name: "Removing a permission that is not channel moderated should not error",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name: &nonChannelModeratedPermission,
					Roles: &model.ChannelModeratedRolesPatch{
						Members: model.NewBool(false),
						Guests:  model.NewBool(false),
					},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{},
			ShouldError:                 false,
			ShouldHaveNoChannelScheme:   true,
		},
		{
			Name: "Error when adding a permission that is disabled in the parent member role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name: &createPosts,
					Roles: &model.ChannelModeratedRolesPatch{
						Members: model.NewBool(true),
						Guests:  model.NewBool(false),
					},
				},
			},
			PermissionsModeratedByPatch:   map[string]*model.ChannelModeratedRoles{},
			HigherScopedMemberPermissions: []string{},
			ShouldError:                   true,
		},
		{
			Name: "Error when adding a permission that is disabled in the parent guest role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name: &createPosts,
					Roles: &model.ChannelModeratedRolesPatch{
						Members: model.NewBool(false),
						Guests:  model.NewBool(true),
					},
				},
			},
			PermissionsModeratedByPatch:  map[string]*model.ChannelModeratedRoles{},
			HigherScopedGuestPermissions: []string{},
			ShouldError:                  true,
		},
		{
			Name: "Removing a permission from the member role that is disabled in the parent guest role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name: &createPosts,
					Roles: &model.ChannelModeratedRolesPatch{
						Members: model.NewBool(false),
					},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{
				createPosts: {
					Members: &model.ChannelModeratedRole{Value: false, Enabled: true},
					Guests:  &model.ChannelModeratedRole{Value: false, Enabled: false},
				},
				createReactions: {
					Guests: &model.ChannelModeratedRole{Value: false, Enabled: false},
				},
				channelMentions: {
					Guests: &model.ChannelModeratedRole{Value: false, Enabled: false},
				},
			},
			HigherScopedGuestPermissions: []string{},
			ShouldError:                  false,
		},
		{
			Name: "Channel should have no scheme when all moderated permissions are equivalent to higher scoped role",
			ChannelModerationsPatch: []*model.ChannelModerationPatch{
				{
					Name: &createPosts,
					Roles: &model.ChannelModeratedRolesPatch{
						Members: model.NewBool(true),
						Guests:  model.NewBool(true),
					},
				},
				{
					Name: &createReactions,
					Roles: &model.ChannelModeratedRolesPatch{
						Members: model.NewBool(true),
						Guests:  model.NewBool(true),
					},
				},
				{
					Name: &channelMentions,
					Roles: &model.ChannelModeratedRolesPatch{
						Members: model.NewBool(true),
						Guests:  model.NewBool(true),
					},
				},
				{
					Name: &manageMembers,
					Roles: &model.ChannelModeratedRolesPatch{
						Members: model.NewBool(true),
					},
				},
			},
			PermissionsModeratedByPatch: map[string]*model.ChannelModeratedRoles{},
			ShouldHaveNoChannelScheme:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			higherScopedPermissionsOverriden := tc.HigherScopedMemberPermissions != nil || tc.HigherScopedGuestPermissions != nil
			// If the test case restricts higher scoped permissions.
			if higherScopedPermissionsOverriden {
				higherScopedGuestRoleName, higherScopedMemberRoleName, _, _ := th.App.GetTeamSchemeChannelRoles(channel.TeamId)
				if tc.HigherScopedMemberPermissions != nil {
					higherScopedMemberRole, err := th.App.GetRoleByName(context.Background(), higherScopedMemberRoleName)
					require.Nil(t, err)
					originalPermissions := higherScopedMemberRole.Permissions

					th.App.PatchRole(higherScopedMemberRole, &model.RolePatch{Permissions: &tc.HigherScopedMemberPermissions})
					defer th.App.PatchRole(higherScopedMemberRole, &model.RolePatch{Permissions: &originalPermissions})
				}

				if tc.HigherScopedGuestPermissions != nil {
					higherScopedGuestRole, err := th.App.GetRoleByName(context.Background(), higherScopedGuestRoleName)
					require.Nil(t, err)
					originalPermissions := higherScopedGuestRole.Permissions

					th.App.PatchRole(higherScopedGuestRole, &model.RolePatch{Permissions: &tc.HigherScopedGuestPermissions})
					defer th.App.PatchRole(higherScopedGuestRole, &model.RolePatch{Permissions: &originalPermissions})
				}
			}

			moderations, appErr := th.App.PatchChannelModerationsForChannel(channel, tc.ChannelModerationsPatch)
			if tc.ShouldError {
				require.NotNil(t, appErr)
				return
			}
			require.Nil(t, appErr)

			updatedChannel, _ := th.App.GetChannel(channel.Id)
			if tc.ShouldHaveNoChannelScheme {
				require.Nil(t, updatedChannel.SchemeId)
			} else {
				require.NotNil(t, updatedChannel.SchemeId)
			}

			for _, moderation := range moderations {
				// If the permission is not found in the expected modified permissions table then require it to be true
				if permission, found := tc.PermissionsModeratedByPatch[moderation.Name]; found && permission.Members != nil {
					require.Equal(t, moderation.Roles.Members.Value, permission.Members.Value)
					require.Equal(t, moderation.Roles.Members.Enabled, permission.Members.Enabled)
				} else {
					require.Equal(t, moderation.Roles.Members.Value, true)
					require.Equal(t, moderation.Roles.Members.Enabled, true)
				}

				if permission, found := tc.PermissionsModeratedByPatch[moderation.Name]; found && permission.Guests != nil {
					require.Equal(t, moderation.Roles.Guests.Value, permission.Guests.Value)
					require.Equal(t, moderation.Roles.Guests.Enabled, permission.Guests.Enabled)
				} else if moderation.Name == manageMembers {
					require.Empty(t, moderation.Roles.Guests)
				} else {
					require.Equal(t, moderation.Roles.Guests.Value, true)
					require.Equal(t, moderation.Roles.Guests.Enabled, true)
				}
			}

			if tc.RevertChannelModerationsPatch != nil {
				th.App.PatchChannelModerationsForChannel(channel, tc.RevertChannelModerationsPatch)
			}
		})
	}

	t.Run("Handles concurrent patch requests gracefully", func(t *testing.T) {
		addCreatePosts := []*model.ChannelModerationPatch{
			{
				Name: &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{
					Members: model.NewBool(false),
					Guests:  model.NewBool(false),
				},
			},
		}
		removeCreatePosts := []*model.ChannelModerationPatch{
			{
				Name: &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{
					Members: model.NewBool(false),
					Guests:  model.NewBool(false),
				},
			},
		}

		wg := sync.WaitGroup{}
		wg.Add(20)
		for i := 0; i < 10; i++ {
			go func() {
				th.App.PatchChannelModerationsForChannel(channel.DeepCopy(), addCreatePosts)
				th.App.PatchChannelModerationsForChannel(channel.DeepCopy(), removeCreatePosts)
				wg.Done()
			}()
		}
		for i := 0; i < 10; i++ {
			go func() {
				th.App.PatchChannelModerationsForChannel(channel.DeepCopy(), addCreatePosts)
				th.App.PatchChannelModerationsForChannel(channel.DeepCopy(), removeCreatePosts)
				wg.Done()
			}()
		}
		wg.Wait()

		higherScopedGuestRoleName, higherScopedMemberRoleName, _, _ := th.App.GetTeamSchemeChannelRoles(channel.TeamId)
		higherScopedMemberRole, _ := th.App.GetRoleByName(context.Background(), higherScopedMemberRoleName)
		higherScopedGuestRole, _ := th.App.GetRoleByName(context.Background(), higherScopedGuestRoleName)
		assert.Contains(t, higherScopedMemberRole.Permissions, createPosts)
		assert.Contains(t, higherScopedGuestRole.Permissions, createPosts)
	})

	t.Run("Updates the authorization to create post", func(t *testing.T) {
		addCreatePosts := []*model.ChannelModerationPatch{
			{
				Name: &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{
					Members: model.NewBool(true),
				},
			},
		}
		removeCreatePosts := []*model.ChannelModerationPatch{
			{
				Name: &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{
					Members: model.NewBool(false),
				},
			},
		}

		mockSession := model.Session{UserId: user.Id}

		_, err := th.App.PatchChannelModerationsForChannel(channel.DeepCopy(), addCreatePosts)
		require.Nil(t, err)
		require.True(t, th.App.SessionHasPermissionToChannel(mockSession, channel.Id, model.PERMISSION_CREATE_POST))

		_, err = th.App.PatchChannelModerationsForChannel(channel.DeepCopy(), removeCreatePosts)
		require.Nil(t, err)
		require.False(t, th.App.SessionHasPermissionToChannel(mockSession, channel.Id, model.PERMISSION_CREATE_POST))
	})
}

// TestMarkChannelsAsViewedPanic verifies that returning an error from a.GetUser
// does not cause a panic.
func TestMarkChannelsAsViewedPanic(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	mockStore := th.App.Srv().Store.(*mocks.Store)
	mockUserStore := mocks.UserStore{}
	mockUserStore.On("Get", context.Background(), "userID").Return(nil, model.NewAppError("SqlUserStore.Get", "app.user.get.app_error", nil, "user_id=userID", http.StatusInternalServerError))
	mockChannelStore := mocks.ChannelStore{}
	mockChannelStore.On("Get", "channelID", true).Return(&model.Channel{}, nil)
	mockChannelStore.On("GetMember", context.Background(), "channelID", "userID").Return(&model.ChannelMember{
		NotifyProps: model.StringMap{
			model.PUSH_NOTIFY_PROP: model.CHANNEL_NOTIFY_DEFAULT,
		}}, nil)
	times := map[string]int64{
		"userID": 1,
	}
	mockChannelStore.On("UpdateLastViewedAt", []string{"channelID"}, "userID", false).Return(times, nil)
	mockSessionStore := mocks.SessionStore{}
	mockOAuthStore := mocks.OAuthStore{}
	var err error
	th.App.srv.userService, err = users.New(users.ServiceConfig{
		UserStore:    &mockUserStore,
		SessionStore: &mockSessionStore,
		OAuthStore:   &mockOAuthStore,
		ConfigFn:     th.App.srv.Config,
	})
	require.NoError(t, err)
	mockPreferenceStore := mocks.PreferenceStore{}
	mockPreferenceStore.On("Get", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&model.Preference{Value: "test"}, nil)
	mockThreadStore := mocks.ThreadStore{}
	mockThreadStore.On("MarkAllAsReadInChannels", "userID", []string{"channelID"}).Return(nil)
	mockStore.On("Channel").Return(&mockChannelStore)
	mockStore.On("Preference").Return(&mockPreferenceStore)
	mockStore.On("Thread").Return(&mockThreadStore)

	_, appErr := th.App.MarkChannelsAsViewed([]string{"channelID"}, "userID", th.Context.Session().Id, false)
	require.Nil(t, appErr)
}

func TestClearChannelMembersCache(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	mockStore := th.App.Srv().Store.(*mocks.Store)
	mockChannelStore := mocks.ChannelStore{}
	cms := model.ChannelMembers{}
	for i := 0; i < 200; i++ {
		cms = append(cms, model.ChannelMember{
			ChannelId: "1",
		})
	}
	mockChannelStore.On("GetMembers", "channelID", 0, 100).Return(&cms, nil)
	mockChannelStore.On("GetMembers", "channelID", 100, 100).Return(&model.ChannelMembers{
		model.ChannelMember{
			ChannelId: "1",
		}}, nil)
	mockStore.On("Channel").Return(&mockChannelStore)

	th.App.ClearChannelMembersCache("channelID")
}

func TestGetMemberCountsByGroup(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	mockStore := th.App.Srv().Store.(*mocks.Store)
	mockChannelStore := mocks.ChannelStore{}
	cmc := []*model.ChannelMemberCountByGroup{}
	for i := 0; i < 5; i++ {
		cmc = append(cmc, &model.ChannelMemberCountByGroup{
			GroupId:                     model.NewId(),
			ChannelMemberCount:          int64(i),
			ChannelMemberTimezonesCount: int64(i),
		})
	}
	mockChannelStore.On("GetMemberCountsByGroup", context.Background(), "channelID", true).Return(cmc, nil)
	mockStore.On("Channel").Return(&mockChannelStore)
	resp, err := th.App.GetMemberCountsByGroup(context.Background(), "channelID", true)
	require.Nil(t, err)
	require.ElementsMatch(t, cmc, resp)
}

func TestViewChannelCollapsedThreadsTurnedOff(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	u1 := th.BasicUser
	u2 := th.BasicUser2
	c1 := th.BasicChannel
	th.AddUserToChannel(u2, c1)

	// Enable CRT
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	// Turn off CRT for user
	preference := model.Preference{
		UserId:   u1.Id,
		Category: model.PREFERENCE_CATEGORY_DISPLAY_SETTINGS,
		Name:     model.PREFERENCE_NAME_COLLAPSED_THREADS_ENABLED,
		Value:    "off",
	}
	var preferences model.Preferences
	preferences = append(preferences, preference)
	err := th.App.Srv().Store.Preference().Save(&preferences)
	require.NoError(t, err)

	// mention the user in a root post
	post1 := &model.Post{
		ChannelId: c1.Id,
		Message:   "root post @" + u1.Username,
		UserId:    u2.Id,
	}
	rpost1, appErr := th.App.CreatePost(th.Context, post1, c1, false, true)
	require.Nil(t, appErr)

	// mention the user in a reply post
	post2 := &model.Post{
		ChannelId: c1.Id,
		Message:   "reply post @" + u1.Username,
		UserId:    u2.Id,
		RootId:    rpost1.Id,
	}
	_, appErr = th.App.CreatePost(th.Context, post2, c1, false, true)
	require.Nil(t, appErr)

	// Check we have unread mention in the thread
	threads, appErr := th.App.GetThreadsForUser(u1.Id, c1.TeamId, model.GetUserThreadsOpts{})
	require.Nil(t, appErr)
	found := false
	for _, thread := range threads.Threads {
		if thread.PostId == rpost1.Id {
			require.EqualValues(t, int64(1), thread.UnreadMentions)
			found = true
			break
		}
	}
	require.Truef(t, found, "did not find created thread in user's threads")

	// Mark channel as read from a client that supports CRT
	_, appErr = th.App.MarkChannelsAsViewed([]string{c1.Id}, u1.Id, th.Context.Session().Id, true)
	require.Nil(t, appErr)

	// Thread should be marked as read because CRT has been turned off by user
	threads, appErr = th.App.GetThreadsForUser(u1.Id, c1.TeamId, model.GetUserThreadsOpts{})
	require.Nil(t, appErr)
	found = false
	for _, thread := range threads.Threads {
		if thread.PostId == rpost1.Id {
			require.Zero(t, thread.UnreadMentions)
			found = true
			break
		}
	}
	require.Truef(t, found, "did not find created thread in user's threads")
}

func TestMarkChannelAsUnreadFromPostCollapsedThreadsTurnedOff(t *testing.T) {
	// Enable CRT
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")

	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	th.AddUserToChannel(th.BasicUser2, th.BasicChannel)

	// Turn off CRT for user
	preference := model.Preference{
		UserId:   th.BasicUser.Id,
		Category: model.PREFERENCE_CATEGORY_DISPLAY_SETTINGS,
		Name:     model.PREFERENCE_NAME_COLLAPSED_THREADS_ENABLED,
		Value:    "off",
	}
	var preferences model.Preferences
	preferences = append(preferences, preference)
	err := th.App.Srv().Store.Preference().Save(&preferences)
	require.NoError(t, err)

	// user2: first root mention @user1
	//   - user1: hello
	//   - user2: mention @u1
	//   - user1: another repoy
	//   - user2: another mention @u1
	// user1: a root post
	// user2: Another root mention @u1
	user1Mention := " @" + th.BasicUser.Username
	rootPost1, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "first root mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost1.Id, UserId: th.BasicUser.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hello"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	replyPost1, appErr := th.App.CreatePost(th.Context, &model.Post{RootId: rootPost1.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost1.Id, UserId: th.BasicUser.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "another reply"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost1.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "another mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "a root post"}, th.BasicChannel, false, false)
	require.Nil(t, appErr)
	_, appErr = th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "another root mention" + user1Mention}, th.BasicChannel, false, false)
	require.Nil(t, appErr)

	t.Run("Mark reply post as unread", func(t *testing.T) {
		_, err := th.App.MarkChannelAsUnreadFromPost(replyPost1.Id, th.BasicUser.Id, true, true)
		require.Nil(t, err)
		// Get channel unreads
		// Easier to reason with ChannelUnread now, than channelUnreadAt from the previous call
		channelUnread, err := th.App.GetChannelUnread(th.BasicChannel.Id, th.BasicUser.Id)
		require.Nil(t, err)

		require.Equal(t, int64(3), channelUnread.MentionCount)
		//  MentionCountRoot should be zero for a user that has CRT turned off
		require.Equal(t, int64(0), channelUnread.MentionCountRoot)

		require.Equal(t, int64(5), channelUnread.MsgCount)
		//  MentionCountRoot should be zero for a user that has CRT turned off
		require.Equal(t, channelUnread.MsgCountRoot, int64(0))

		threadMembership, err := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost1.Id)
		require.Nil(t, err)
		thread, err := th.App.GetThreadForUser(th.BasicTeam.Id, threadMembership, false)
		require.Nil(t, err)
		require.Equal(t, int64(2), thread.UnreadMentions)
		require.Equal(t, int64(3), thread.UnreadReplies)
	})

	t.Run("Mark root post as unread", func(t *testing.T) {
		_, err := th.App.MarkChannelAsUnreadFromPost(rootPost1.Id, th.BasicUser.Id, true, true)
		require.Nil(t, err)
		// Get channel unreads
		// Easier to reason with ChannelUnread now, than channelUnreadAt from the previous call
		channelUnread, err := th.App.GetChannelUnread(th.BasicChannel.Id, th.BasicUser.Id)
		require.Nil(t, err)

		require.Equal(t, int64(4), channelUnread.MentionCount)
		require.Equal(t, int64(2), channelUnread.MentionCountRoot)

		require.Equal(t, int64(7), channelUnread.MsgCount)
		require.Equal(t, int64(3), channelUnread.MsgCountRoot)
	})
}

func TestMarkUnreadWithThreads(t *testing.T) {
	os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
	defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ThreadAutoFollow = true
		*cfg.ServiceSettings.CollapsedThreads = model.COLLAPSED_THREADS_DEFAULT_ON
	})

	t.Run("Follow threads only if specified", func(t *testing.T) {
		rootPost, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
		require.Nil(t, appErr)
		replyPost, appErr := th.App.CreatePost(th.Context, &model.Post{RootId: rootPost.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
		require.Nil(t, appErr)
		threads, appErr := th.App.GetThreadsForUser(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
		require.Nil(t, appErr)
		require.Zero(t, threads.Total)

		_, appErr = th.App.MarkChannelAsUnreadFromPost(replyPost.Id, th.BasicUser.Id, true, true)
		require.Nil(t, appErr)

		threads, appErr = th.App.GetThreadsForUser(th.BasicUser.Id, th.BasicTeam.Id, model.GetUserThreadsOpts{})
		require.Nil(t, appErr)
		require.NotZero(t, threads.Total)

		threadMembership, appErr := th.App.GetThreadMembershipForUser(th.BasicUser.Id, replyPost.RootId)
		require.Nil(t, appErr)
		require.NotNil(t, threadMembership)
		assert.True(t, threadMembership.Following)

		// Create a new thread
		rootPost, appErr = th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi2"}, th.BasicChannel, false, false)
		require.Nil(t, appErr)
		replyPost, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi2"}, th.BasicChannel, false, false)
		require.Nil(t, appErr)

		_, appErr = th.App.MarkChannelAsUnreadFromPost(replyPost.Id, th.BasicUser.Id, true, false)
		require.Nil(t, appErr)

		threadMembership, appErr = th.App.GetThreadMembershipForUser(th.BasicUser.Id, replyPost.RootId)
		require.Nil(t, appErr)
		require.NotNil(t, threadMembership)
		assert.False(t, threadMembership.Following)
	})

	t.Run("Set unread mentions correctly", func(t *testing.T) {
		t.Run("Never followed root post with no replies or mentions", func(t *testing.T) {
			rootPost, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			_, appErr = th.App.MarkChannelAsUnreadFromPost(rootPost.Id, th.BasicUser.Id, true, true)
			require.Nil(t, appErr)

			threadMembership, appErr := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost.Id)
			require.Nil(t, appErr)
			require.NotNil(t, threadMembership)
			assert.Zero(t, threadMembership.UnreadMentions)
		})

		t.Run("Never followed root post with replies and no mentions", func(t *testing.T) {
			rootPost, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			_, appErr = th.App.MarkChannelAsUnreadFromPost(rootPost.Id, th.BasicUser.Id, true, true)
			require.Nil(t, appErr)

			threadMembership, appErr := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost.Id)
			require.Nil(t, appErr)
			require.NotNil(t, threadMembership)
			assert.Zero(t, threadMembership.UnreadMentions)
		})

		t.Run("Never followed root post with replies and mentions", func(t *testing.T) {
			rootPost, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi @" + th.BasicUser.Username}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			_, appErr = th.App.MarkChannelAsUnreadFromPost(rootPost.Id, th.BasicUser.Id, true, true)
			require.Nil(t, appErr)

			threadMembership, appErr := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost.Id)
			require.Nil(t, appErr)
			require.NotNil(t, threadMembership)
			assert.Equal(t, int64(1), threadMembership.UnreadMentions)
		})

		t.Run("Previously followed root post with no replies or mentions", func(t *testing.T) {
			rootPost, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			appErr = th.App.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rootPost.Id, true)
			require.Nil(t, appErr)
			appErr = th.App.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rootPost.Id, false)
			require.Nil(t, appErr)

			_, appErr = th.App.MarkChannelAsUnreadFromPost(rootPost.Id, th.BasicUser.Id, true, true)
			require.Nil(t, appErr)

			threadMembership, appErr := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost.Id)
			require.Nil(t, appErr)
			require.NotNil(t, threadMembership)
			assert.Zero(t, threadMembership.UnreadMentions)
		})

		t.Run("Previously followed root post with replies and no mentions", func(t *testing.T) {
			rootPost, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			appErr = th.App.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rootPost.Id, true)
			require.Nil(t, appErr)
			appErr = th.App.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rootPost.Id, false)
			require.Nil(t, appErr)

			_, appErr = th.App.MarkChannelAsUnreadFromPost(rootPost.Id, th.BasicUser.Id, true, true)
			require.Nil(t, appErr)

			threadMembership, appErr := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost.Id)
			require.Nil(t, appErr)
			require.NotNil(t, threadMembership)
			assert.Zero(t, threadMembership.UnreadMentions)
		})

		t.Run("Previously followed root post with replies and mentions", func(t *testing.T) {
			rootPost, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi"}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			_, appErr = th.App.CreatePost(th.Context, &model.Post{RootId: rootPost.Id, UserId: th.BasicUser2.Id, CreateAt: model.GetMillis(), ChannelId: th.BasicChannel.Id, Message: "hi @" + th.BasicUser.Username}, th.BasicChannel, false, false)
			require.Nil(t, appErr)
			appErr = th.App.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rootPost.Id, true)
			require.Nil(t, appErr)
			appErr = th.App.UpdateThreadFollowForUser(th.BasicUser.Id, th.BasicTeam.Id, rootPost.Id, false)
			require.Nil(t, appErr)

			_, appErr = th.App.MarkChannelAsUnreadFromPost(rootPost.Id, th.BasicUser.Id, true, true)
			require.Nil(t, appErr)

			threadMembership, appErr := th.App.GetThreadMembershipForUser(th.BasicUser.Id, rootPost.Id)
			require.Nil(t, appErr)
			require.NotNil(t, threadMembership)
			assert.Equal(t, int64(1), threadMembership.UnreadMentions)
		})
	})
}
