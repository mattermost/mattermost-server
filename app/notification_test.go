// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func TestSendNotifications(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.AddUserToChannel(th.BasicUser2, th.BasicChannel, false)

	post1, appErr := th.App.CreatePostMissingChannel(th.Context, &model.Post{
		UserID:    th.BasicUser.ID,
		ChannelID: th.BasicChannel.ID,
		Message:   "@" + th.BasicUser2.Username,
		Type:      model.PostTypeAddToChannel,
		Props:     map[string]interface{}{model.PostPropsAddedUserID: "junk"},
	}, true)
	require.Nil(t, appErr)

	mentions, err := th.App.SendNotifications(post1, th.BasicTeam, th.BasicChannel, th.BasicUser, nil, true)
	require.NoError(t, err)
	require.NotNil(t, mentions)
	require.True(t, utils.StringInSlice(th.BasicUser2.ID, mentions), "mentions", mentions)

	dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.ID, th.BasicUser2.ID)
	require.Nil(t, appErr)

	post2, appErr := th.App.CreatePostMissingChannel(th.Context, &model.Post{
		UserID:    th.BasicUser.ID,
		ChannelID: dm.ID,
		Message:   "dm message",
	}, true)
	require.Nil(t, appErr)

	mentions, err = th.App.SendNotifications(post2, th.BasicTeam, dm, th.BasicUser, nil, true)
	require.NoError(t, err)
	require.NotNil(t, mentions)

	_, appErr = th.App.UpdateActive(th.Context, th.BasicUser2, false)
	require.Nil(t, appErr)
	appErr = th.App.Srv().InvalidateAllCaches()
	require.Nil(t, appErr)

	post3, appErr := th.App.CreatePostMissingChannel(th.Context, &model.Post{
		UserID:    th.BasicUser.ID,
		ChannelID: dm.ID,
		Message:   "dm message",
	}, true)
	require.Nil(t, appErr)

	mentions, err = th.App.SendNotifications(post3, th.BasicTeam, dm, th.BasicUser, nil, true)
	require.NoError(t, err)
	require.NotNil(t, mentions)

	th.BasicChannel.DeleteAt = 1
	mentions, err = th.App.SendNotifications(post1, th.BasicTeam, th.BasicChannel, th.BasicUser, nil, true)
	require.NoError(t, err)
	require.Empty(t, mentions)

	t.Run("replies to post created by OAuth bot should not notify user", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		testUserNotNotified := func(t *testing.T, user *model.User) {
			rootPost := &model.Post{
				UserID:    user.ID,
				ChannelID: th.BasicChannel.ID,
				Message:   "a message",
				Props:     model.StringInterface{"from_webhook": "true", "override_username": "a bot"},
			}

			rootPost, appErr = th.App.CreatePostMissingChannel(th.Context, rootPost, false)
			require.Nil(t, appErr)

			childPost := &model.Post{
				UserID:    th.BasicUser2.ID,
				ChannelID: th.BasicChannel.ID,
				RootID:    rootPost.ID,
				Message:   "a reply",
			}
			childPost, appErr = th.App.CreatePostMissingChannel(th.Context, childPost, false)
			require.Nil(t, appErr)

			postList := model.PostList{
				Order: []string{rootPost.ID, childPost.ID},
				Posts: map[string]*model.Post{rootPost.ID: rootPost, childPost.ID: childPost},
			}
			mentions, err = th.App.SendNotifications(childPost, th.BasicTeam, th.BasicChannel, th.BasicUser2, &postList, true)
			require.NoError(t, err)
			require.False(t, utils.StringInSlice(user.ID, mentions))
		}

		th.BasicUser.NotifyProps[model.CommentsNotifyProp] = model.CommentsNotifyAny
		th.BasicUser, appErr = th.App.UpdateUser(th.BasicUser, false)
		require.Nil(t, appErr)
		t.Run("user wants notifications on all comments", func(t *testing.T) {
			testUserNotNotified(t, th.BasicUser)
		})

		th.BasicUser.NotifyProps[model.CommentsNotifyProp] = model.CommentsNotifyRoot
		th.BasicUser, appErr = th.App.UpdateUser(th.BasicUser, false)
		require.Nil(t, appErr)
		t.Run("user wants notifications on root comment", func(t *testing.T) {
			testUserNotNotified(t, th.BasicUser)
		})
	})
}

func TestSendNotificationsWithManyUsers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	users := []*model.User{}
	for i := 0; i < 10; i++ {
		user := th.CreateUser()
		th.LinkUserToTeam(user, th.BasicTeam)
		th.App.AddUserToChannel(user, th.BasicChannel, false)
		users = append(users, user)
	}

	_, appErr1 := th.App.CreatePostMissingChannel(th.Context, &model.Post{
		UserID:    th.BasicUser.ID,
		ChannelID: th.BasicChannel.ID,
		Message:   "@channel",
		Type:      model.PostTypeAddToChannel,
		Props:     map[string]interface{}{model.PostPropsAddedUserID: "junk"},
	}, true)
	require.Nil(t, appErr1)

	// Each user should have a mention count of exactly 1 in the DB at this point.
	t.Run("1-mention", func(t *testing.T) {
		for i, user := range users {
			t.Run(fmt.Sprintf("user-%d", i+1), func(t *testing.T) {
				channelUnread, appErr2 := th.Server.Store.Channel().GetChannelUnread(th.BasicChannel.ID, user.ID)
				require.NoError(t, appErr2)
				assert.Equal(t, int64(1), channelUnread.MentionCount)
			})
		}
	})

	_, appErr1 = th.App.CreatePostMissingChannel(th.Context, &model.Post{
		UserID:    th.BasicUser.ID,
		ChannelID: th.BasicChannel.ID,
		Message:   "@channel",
		Type:      model.PostTypeAddToChannel,
		Props:     map[string]interface{}{model.PostPropsAddedUserID: "junk"},
	}, true)
	require.Nil(t, appErr1)

	// Now each user should have a mention count of exactly 2 in the DB.
	t.Run("2-mentions", func(t *testing.T) {
		for i, user := range users {
			t.Run(fmt.Sprintf("user-%d", i+1), func(t *testing.T) {
				channelUnread, appErr2 := th.Server.Store.Channel().GetChannelUnread(th.BasicChannel.ID, user.ID)
				require.NoError(t, appErr2)
				assert.Equal(t, int64(2), channelUnread.MentionCount)
			})
		}
	})
}

func TestSendOutOfChannelMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel

	user1 := th.BasicUser
	user2 := th.BasicUser2

	t.Run("should send ephemeral post when there is an out of channel mention", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{user2.Username}

		sent, err := th.App.sendOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.NoError(t, err)
		assert.True(t, sent)
	})

	t.Run("should not send ephemeral post when there are no out of channel mentions", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{"not a user"}

		sent, err := th.App.sendOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.NoError(t, err)
		assert.False(t, sent)
	})
}

func TestFilterOutOfChannelMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel

	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()
	guest := th.CreateGuest()
	user4 := th.CreateUser()
	guestAndUser4Channel := th.CreateChannel(th.BasicTeam)
	defer th.App.PermanentDeleteUser(th.Context, guest)
	th.LinkUserToTeam(user3, th.BasicTeam)
	th.LinkUserToTeam(user4, th.BasicTeam)
	th.LinkUserToTeam(guest, th.BasicTeam)
	th.App.AddUserToChannel(guest, channel, false)
	th.App.AddUserToChannel(user4, guestAndUser4Channel, false)
	th.App.AddUserToChannel(guest, guestAndUser4Channel, false)

	t.Run("should return users not in the channel", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{user2.Username, user3.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.NoError(t, err)
		assert.Len(t, outOfChannelUsers, 2)
		assert.True(t, (outOfChannelUsers[0].ID == user2.ID || outOfChannelUsers[1].ID == user2.ID))
		assert.True(t, (outOfChannelUsers[0].ID == user3.ID || outOfChannelUsers[1].ID == user3.ID))
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should return only visible users not in the channel (for guests)", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{user2.Username, user3.Username, user4.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(guest, post, channel, potentialMentions)

		require.NoError(t, err)
		require.Len(t, outOfChannelUsers, 1)
		assert.Equal(t, user4.ID, outOfChannelUsers[0].ID)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return results for a system message", func(t *testing.T) {
		post := &model.Post{
			Type: model.PostTypeAddRemove,
		}
		potentialMentions := []string{user2.Username, user3.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.NoError(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return results for a direct message", func(t *testing.T) {
		post := &model.Post{}
		directChannel := &model.Channel{
			Type: model.ChannelTypeDirect,
		}
		potentialMentions := []string{user2.Username, user3.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, directChannel, potentialMentions)

		assert.NoError(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return results for a group message", func(t *testing.T) {
		post := &model.Post{}
		groupChannel := &model.Channel{
			Type: model.ChannelTypeGroup,
		}
		potentialMentions := []string{user2.Username, user3.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, groupChannel, potentialMentions)

		assert.NoError(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return inactive users", func(t *testing.T) {
		inactiveUser := th.CreateUser()
		inactiveUser, appErr := th.App.UpdateActive(th.Context, inactiveUser, false)
		require.Nil(t, appErr)

		post := &model.Post{}
		potentialMentions := []string{inactiveUser.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.NoError(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return bot users", func(t *testing.T) {
		botUser := th.CreateUser()
		botUser.IsBot = true

		post := &model.Post{}
		potentialMentions := []string{botUser.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.NoError(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return results for non-existent users", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{"foo", "bar"}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.NoError(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should separate users not in the channel from users not in the group", func(t *testing.T) {
		nonChannelMember := th.CreateUser()
		th.LinkUserToTeam(nonChannelMember, th.BasicTeam)
		nonGroupMember := th.CreateUser()
		th.LinkUserToTeam(nonGroupMember, th.BasicTeam)

		group := th.CreateGroup()
		_, appErr := th.App.UpsertGroupMember(group.ID, th.BasicUser.ID)
		require.Nil(t, appErr)
		_, appErr = th.App.UpsertGroupMember(group.ID, nonChannelMember.ID)
		require.Nil(t, appErr)

		constrainedChannel := th.CreateChannel(th.BasicTeam)
		constrainedChannel.GroupConstrained = model.NewBool(true)
		constrainedChannel, appErr = th.App.UpdateChannel(constrainedChannel)
		require.Nil(t, appErr)

		_, appErr = th.App.UpsertGroupSyncable(&model.GroupSyncable{
			GroupID:    group.ID,
			Type:       model.GroupSyncableTypeChannel,
			SyncableID: constrainedChannel.ID,
		})
		require.Nil(t, appErr)

		post := &model.Post{}
		potentialMentions := []string{nonChannelMember.Username, nonGroupMember.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, constrainedChannel, potentialMentions)

		assert.NoError(t, err)
		assert.Len(t, outOfChannelUsers, 1)
		assert.Equal(t, nonChannelMember.ID, outOfChannelUsers[0].ID)
		assert.Len(t, outOfGroupUsers, 1)
		assert.Equal(t, nonGroupMember.ID, outOfGroupUsers[0].ID)
	})
}

func TestGetExplicitMentions(t *testing.T) {
	id1 := model.NewID()
	id2 := model.NewID()
	id3 := model.NewID()

	for name, tc := range map[string]struct {
		Message     string
		Attachments []*model.SlackAttachment
		Keywords    map[string][]string
		Groups      map[string]*model.Group
		Expected    *ExplicitMentions
	}{
		"Nobody": {
			Message:  "this is a message",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"NonexistentUser": {
			Message: "this is a message for @user",
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"user"},
			},
		},
		"OnePerson": {
			Message:  "this is a message for @user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithPeriodAtEndOfUsername": {
			Message:  "this is a message for @user.name.",
			Keywords: map[string][]string{"@user.name.": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithPeriodAtEndOfUsernameButNotSimilarName": {
			Message:  "this is a message for @user.name.",
			Keywords: map[string][]string{"@user.name.": {id1}, "@user.name": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonAtEndOfSentence": {
			Message:  "this is a message for @user.",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithoutAtMention": {
			Message:  "this is a message for @user",
			Keywords: map[string][]string{"this": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
				OtherPotentialMentions: []string{"user"},
			},
		},
		"OnePersonWithPeriodAfter": {
			Message:  "this is a message for @user.",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithPeriodBefore": {
			Message:  "this is a message for .@user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithColonAfter": {
			Message:  "this is a message for @user:",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithColonBefore": {
			Message:  "this is a message for :@user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithHyphenAfter": {
			Message:  "this is a message for @user.",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithHyphenBefore": {
			Message:  "this is a message for -@user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultiplePeopleWithOneWord": {
			Message:  "this is a message for @user",
			Keywords: map[string][]string{"@user": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
				},
			},
		},
		"OneOfMultiplePeople": {
			Message:  "this is a message for @user",
			Keywords: map[string][]string{"@user": {id1}, "@mention": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultiplePeopleWithMultipleWords": {
			Message:  "this is an @mention for @user",
			Keywords: map[string][]string{"@user": {id1}, "@mention": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
				},
			},
		},
		"Channel": {
			Message:  "this is an message for @channel",
			Keywords: map[string][]string{"@channel": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				ChannelMentioned: true,
			},
		},

		"ChannelWithColonAtEnd": {
			Message:  "this is a message for @channel:",
			Keywords: map[string][]string{"@channel": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				ChannelMentioned: true,
			},
		},
		"CapitalizedChannel": {
			Message:  "this is an message for @cHaNNeL",
			Keywords: map[string][]string{"@channel": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				ChannelMentioned: true,
			},
		},
		"All": {
			Message:  "this is an message for @all",
			Keywords: map[string][]string{"@all": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				AllMentioned: true,
			},
		},
		"AllWithColonAtEnd": {
			Message:  "this is a message for @all:",
			Keywords: map[string][]string{"@all": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				AllMentioned: true,
			},
		},
		"CapitalizedAll": {
			Message:  "this is an message for @ALL",
			Keywords: map[string][]string{"@all": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				AllMentioned: true,
			},
		},
		"UserWithPeriod": {
			Message:  "user.period doesn't complicate things at all by including periods in their username",
			Keywords: map[string][]string{"user.period": {id1}, "user": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"AtUserWithColonAtEnd": {
			Message:  "this is a message for @user:",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"AtUserWithPeriodAtEndOfSentence": {
			Message:  "this is a message for @user.period.",
			Keywords: map[string][]string{"@user.period": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"UserWithPeriodAtEndOfSentence": {
			Message:  "this is a message for user.period.",
			Keywords: map[string][]string{"user.period": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"UserWithColonAtEnd": {
			Message:  "this is a message for user:",
			Keywords: map[string][]string{"user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"PotentialOutOfChannelUser": {
			Message:  "this is an message for @potential and @user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
				OtherPotentialMentions: []string{"potential"},
			},
		},
		"PotentialOutOfChannelUserWithPeriod": {
			Message: "this is an message for @potential.user",
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"potential.user"},
			},
		},
		"InlineCode": {
			Message:  "`this shouldn't mention @channel at all`",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"FencedCodeBlock": {
			Message:  "```\nthis shouldn't mention @channel at all\n```",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"Emphasis": {
			Message:  "*@aaa @bbb @ccc*",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
					id3: KeywordMention,
				},
			},
		},
		"StrongEmphasis": {
			Message:  "**@aaa @bbb @ccc**",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
					id3: KeywordMention,
				},
			},
		},
		"Strikethrough": {
			Message:  "~~@aaa @bbb @ccc~~",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
					id3: KeywordMention,
				},
			},
		},
		"Heading": {
			Message:  "### @aaa",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"BlockQuote": {
			Message:  "> @aaa",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Emoji": {
			Message:  ":smile:",
			Keywords: map[string][]string{"smile": {id1}, "smiley": {id2}, "smiley_cat": {id3}},
			Expected: &ExplicitMentions{},
		},
		"NotEmoji": {
			Message:  "smile",
			Keywords: map[string][]string{"smile": {id1}, "smiley": {id2}, "smiley_cat": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"UnclosedEmoji": {
			Message:  ":smile",
			Keywords: map[string][]string{"smile": {id1}, "smiley": {id2}, "smiley_cat": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"UnopenedEmoji": {
			Message:  "smile:",
			Keywords: map[string][]string{"smile": {id1}, "smiley": {id2}, "smiley_cat": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"IndentedCodeBlock": {
			Message:  "    this shouldn't mention @channel at all",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"LinkTitle": {
			Message:  `[foo](this "shouldn't mention @channel at all")`,
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"MalformedInlineCode": {
			Message:  "`this should mention @channel``",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{
				ChannelMentioned: true,
			},
		},
		"MultibyteCharacter": {
			Message:  "My name is 萌",
			Keywords: map[string][]string{"萌": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterAtBeginningOfSentence": {
			Message:  "이메일을 보내다.",
			Keywords: map[string][]string{"이메일": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterInPartOfSentence": {
			Message:  "我爱吃番茄炒饭",
			Keywords: map[string][]string{"番茄": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterAtEndOfSentence": {
			Message:  "こんにちは、世界",
			Keywords: map[string][]string{"世界": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterTwiceInSentence": {
			Message:  "石橋さんが石橋を渡る",
			Keywords: map[string][]string{"石橋": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},

		// The following tests cover cases where the message mentions @user.name, so we shouldn't assume that
		// the user might be intending to mention some @user that isn't in the channel.
		"Don't include potential mention that's part of an actual mention (without trailing period)": {
			Message:  "this is an message for @user.name",
			Keywords: map[string][]string{"@user.name": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Don't include potential mention that's part of an actual mention (with trailing period)": {
			Message:  "this is an message for @user.name.",
			Keywords: map[string][]string{"@user.name": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Don't include potential mention that's part of an actual mention (with multiple trailing periods)": {
			Message:  "this is an message for @user.name...",
			Keywords: map[string][]string{"@user.name": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Don't include potential mention that's part of an actual mention (containing and followed by multiple periods)": {
			Message:  "this is an message for @user...name...",
			Keywords: map[string][]string{"@user...name": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"should include the mentions from attachment text and preText": {
			Message: "this is an message for @user1",
			Attachments: []*model.SlackAttachment{
				{
					Text:    "this is a message For @user2",
					Pretext: "this is a message for @here",
				},
			},
			Keywords: map[string][]string{"@user1": {id1}, "@user2": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
				},
				HereMentioned: true,
			},
		},
		"Name on keywords is a prefix of a mention": {
			Message:  "@other @test-two",
			Keywords: map[string][]string{"@test": {model.NewID()}},
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"other", "test-two"},
			},
		},
		"Name on mentions is a prefix of other mention": {
			Message:  "@other-one @other @other-two",
			Keywords: nil,
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"other-one", "other", "other-two"},
			},
		},
		"No groups": {
			Message: "@nothing",
			Groups:  map[string]*model.Group{},
			Expected: &ExplicitMentions{
				Mentions:               nil,
				OtherPotentialMentions: []string{"nothing"},
			},
		},
		"No matching groups": {
			Message: "@nothing",
			Groups:  map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}},
			Expected: &ExplicitMentions{
				Mentions:               nil,
				GroupMentions:          nil,
				OtherPotentialMentions: []string{"nothing"},
			},
		},
		"matching group with no @": {
			Message: "engineering",
			Groups:  map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}},
			Expected: &ExplicitMentions{
				Mentions:               nil,
				GroupMentions:          nil,
				OtherPotentialMentions: nil,
			},
		},
		"matching group with preceding @": {
			Message: "@engineering",
			Groups:  map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}},
			Expected: &ExplicitMentions{
				Mentions: nil,
				GroupMentions: map[string]*model.Group{
					"engineering": {Name: model.NewString("engineering")},
				},
				OtherPotentialMentions: []string{"engineering"},
			},
		},
		"matching upper case group with preceding @": {
			Message: "@Engineering",
			Groups:  map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}},
			Expected: &ExplicitMentions{
				Mentions: nil,
				GroupMentions: map[string]*model.Group{
					"engineering": {Name: model.NewString("engineering")},
				},
				OtherPotentialMentions: []string{"Engineering"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			post := &model.Post{
				Message: tc.Message,
				Props: model.StringInterface{
					"attachments": tc.Attachments,
				},
			}

			m := getExplicitMentions(post, tc.Keywords, tc.Groups)

			assert.EqualValues(t, tc.Expected, m)
		})
	}
}

func TestGetExplicitMentionsAtHere(t *testing.T) {
	t.Run("Boundary cases", func(t *testing.T) {
		// test all the boundary cases that we know can break up terms (and those that we know won't)
		cases := map[string]bool{
			"":          false,
			"here":      false,
			"@here":     true,
			" @here ":   true,
			"\n@here\n": true,
			"!@here!":   true,
			"#@here#":   true,
			"$@here$":   true,
			"%@here%":   true,
			"^@here^":   true,
			"&@here&":   true,
			"*@here*":   true,
			"(@here(":   true,
			")@here)":   true,
			"-@here-":   true,
			"_@here_":   true,
			"=@here=":   true,
			"+@here+":   true,
			"[@here[":   true,
			"{@here{":   true,
			"]@here]":   true,
			"}@here}":   true,
			"\\@here\\": true,
			"|@here|":   true,
			";@here;":   true,
			"@here:":    true,
			":@here:":   false, // This case shouldn't trigger a mention since it follows the format of reactions e.g. :word:
			"'@here'":   true,
			"\"@here\"": true,
			",@here,":   true,
			"<@here<":   true,
			".@here.":   true,
			">@here>":   true,
			"/@here/":   true,
			"?@here?":   true,
			"`@here`":   false, // This case shouldn't mention since it's a code block
			"~@here~":   true,
			"@HERE":     true,
			"@hERe":     true,
		}
		for message, shouldMention := range cases {
			post := &model.Post{Message: message}
			m := getExplicitMentions(post, nil, nil)
			require.False(t, m.HereMentioned && !shouldMention, "shouldn't have mentioned @here with \"%v\"")
			require.False(t, !m.HereMentioned && shouldMention, "should've mentioned @here with \"%v\"")
		}
	})

	t.Run("Mention @here and someone", func(t *testing.T) {
		id := model.NewID()
		m := getExplicitMentions(&model.Post{Message: "@here @user @potential"}, map[string][]string{"@user": {id}}, nil)
		require.True(t, m.HereMentioned, "should've mentioned @here with \"@here @user\"")
		require.Len(t, m.Mentions, 1)
		require.Equal(t, KeywordMention, m.Mentions[id], "should've mentioned @user with \"@here @user\"")
		require.Equal(t, len(m.OtherPotentialMentions), 1, "should've potential mentions for @potential")
		assert.Equal(t, "potential", m.OtherPotentialMentions[0])
	})

	t.Run("Username ending with period", func(t *testing.T) {
		id := model.NewID()
		m := getExplicitMentions(&model.Post{Message: "@potential. test"}, map[string][]string{"@user": {id}}, nil)
		require.Equal(t, len(m.OtherPotentialMentions), 1, "should've potential mentions for @potential")
		assert.Equal(t, "potential", m.OtherPotentialMentions[0])
	})
}

func TestAllowChannelMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	post := &model.Post{ChannelID: th.BasicChannel.ID, UserID: th.BasicUser.ID}

	t.Run("should return true for a regular post with few channel members", func(t *testing.T) {
		allowChannelMentions := th.App.allowChannelMentions(post, 5)
		assert.True(t, allowChannelMentions)
	})

	t.Run("should return false for a channel header post", func(t *testing.T) {
		headerChangePost := &model.Post{ChannelID: th.BasicChannel.ID, UserID: th.BasicUser.ID, Type: model.PostTypeHeaderChange}
		allowChannelMentions := th.App.allowChannelMentions(headerChangePost, 5)
		assert.False(t, allowChannelMentions)
	})

	t.Run("should return false for a channel purpose post", func(t *testing.T) {
		purposeChangePost := &model.Post{ChannelID: th.BasicChannel.ID, UserID: th.BasicUser.ID, Type: model.PostTypePurposeChange}
		allowChannelMentions := th.App.allowChannelMentions(purposeChangePost, 5)
		assert.False(t, allowChannelMentions)
	})

	t.Run("should return false for a regular post with many channel members", func(t *testing.T) {
		allowChannelMentions := th.App.allowChannelMentions(post, int(*th.App.Config().TeamSettings.MaxNotificationsPerChannel)+1)
		assert.False(t, allowChannelMentions)
	})

	t.Run("should return false for a post where the post user does not have USE_CHANNEL_MENTIONS permission", func(t *testing.T) {
		defer th.AddPermissionToRole(model.PermissionUseChannelMentions.ID, model.ChannelUserRoleID)
		defer th.AddPermissionToRole(model.PermissionUseChannelMentions.ID, model.ChannelAdminRoleID)
		th.RemovePermissionFromRole(model.PermissionUseChannelMentions.ID, model.ChannelUserRoleID)
		th.RemovePermissionFromRole(model.PermissionUseChannelMentions.ID, model.ChannelAdminRoleID)
		allowChannelMentions := th.App.allowChannelMentions(post, 5)
		assert.False(t, allowChannelMentions)
	})
}

func TestAllowGroupMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	post := &model.Post{ChannelID: th.BasicChannel.ID, UserID: th.BasicUser.ID}

	t.Run("should return false without ldap groups license", func(t *testing.T) {
		allowGroupMentions := th.App.allowGroupMentions(post)
		assert.False(t, allowGroupMentions)
	})

	th.App.Srv().SetLicense(model.NewTestLicense("ldap_groups"))

	t.Run("should return true for a regular post with few channel members", func(t *testing.T) {
		allowGroupMentions := th.App.allowGroupMentions(post)
		assert.True(t, allowGroupMentions)
	})

	t.Run("should return false for a channel header post", func(t *testing.T) {
		headerChangePost := &model.Post{ChannelID: th.BasicChannel.ID, UserID: th.BasicUser.ID, Type: model.PostTypeHeaderChange}
		allowGroupMentions := th.App.allowGroupMentions(headerChangePost)
		assert.False(t, allowGroupMentions)
	})

	t.Run("should return false for a channel purpose post", func(t *testing.T) {
		purposeChangePost := &model.Post{ChannelID: th.BasicChannel.ID, UserID: th.BasicUser.ID, Type: model.PostTypePurposeChange}
		allowGroupMentions := th.App.allowGroupMentions(purposeChangePost)
		assert.False(t, allowGroupMentions)
	})

	t.Run("should return false for a post where the post user does not have USE_GROUP_MENTIONS permission", func(t *testing.T) {
		defer func() {
			th.AddPermissionToRole(model.PermissionUseGroupMentions.ID, model.ChannelUserRoleID)
			th.AddPermissionToRole(model.PermissionUseGroupMentions.ID, model.ChannelAdminRoleID)
		}()
		th.RemovePermissionFromRole(model.PermissionUseGroupMentions.ID, model.ChannelUserRoleID)
		th.RemovePermissionFromRole(model.PermissionUseGroupMentions.ID, model.ChannelAdminRoleID)
		allowGroupMentions := th.App.allowGroupMentions(post)
		assert.False(t, allowGroupMentions)
	})
}

func TestGetMentionKeywords(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// user with username or custom mentions enabled
	user1 := &model.User{
		ID:        model.NewID(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"mention_keys": "User,@User,MENTION",
		},
	}

	channelMemberNotifyPropsMap1Off := map[string]model.StringMap{
		user1.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
	}

	profiles := map[string]*model.User{user1.ID: user1}
	mentions := th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap1Off)
	require.Len(t, mentions, 3, "should've returned three mention keywords")

	ids, ok := mentions["user"]
	require.True(t, ok)
	require.Equal(t, user1.ID, ids[0], "should've returned mention key of user")
	ids, ok = mentions["@user"]
	require.True(t, ok)
	require.Equal(t, user1.ID, ids[0], "should've returned mention key of @user")
	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Equal(t, user1.ID, ids[0], "should've returned mention key of mention")

	// user with first name mention enabled
	user2 := &model.User{
		ID:        model.NewID(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"first_name": "true",
		},
	}

	channelMemberNotifyPropsMap2Off := map[string]model.StringMap{
		user2.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
	}

	profiles = map[string]*model.User{user2.ID: user2}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap2Off)
	require.Len(t, mentions, 2, "should've returned two mention keyword")

	ids, ok = mentions["First"]
	require.True(t, ok)
	require.Equal(t, user2.ID, ids[0], "should've returned mention key of First")

	// user with @channel/@all mentions enabled
	user3 := &model.User{
		ID:        model.NewID(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"channel": "true",
		},
	}

	// Channel-wide mentions are not ignored on channel level
	channelMemberNotifyPropsMap3Off := map[string]model.StringMap{
		user3.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
	}
	profiles = map[string]*model.User{user3.ID: user3}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap3Off)
	require.Len(t, mentions, 3, "should've returned three mention keywords")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Equal(t, user3.ID, ids[0], "should've returned mention key of @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Equal(t, user3.ID, ids[0], "should've returned mention key of @all")

	// Channel member notify props is set to default
	channelMemberNotifyPropsMapDefault := map[string]model.StringMap{
		user3.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsDefault,
		},
	}
	profiles = map[string]*model.User{user3.ID: user3}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMapDefault)
	require.Len(t, mentions, 3, "should've returned three mention keywords")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Equal(t, user3.ID, ids[0], "should've returned mention key of @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Equal(t, user3.ID, ids[0], "should've returned mention key of @all")

	// Channel member notify props is empty
	channelMemberNotifyPropsMapEmpty := map[string]model.StringMap{}
	profiles = map[string]*model.User{user3.ID: user3}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMapEmpty)
	require.Len(t, mentions, 3, "should've returned three mention keywords")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Equal(t, user3.ID, ids[0], "should've returned mention key of @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Equal(t, user3.ID, ids[0], "should've returned mention key of @all")

	// Channel-wide mentions are ignored channel level
	channelMemberNotifyPropsMap3On := map[string]model.StringMap{
		user3.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOn,
		},
	}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap3On)
	require.NotEmpty(t, mentions, "should've not returned any keywords")

	// user with all types of mentions enabled
	user4 := &model.User{
		ID:        model.NewID(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"mention_keys": "User,@User,MENTION",
			"first_name":   "true",
			"channel":      "true",
		},
	}

	// Channel-wide mentions are not ignored on channel level
	channelMemberNotifyPropsMap4Off := map[string]model.StringMap{
		user4.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
	}

	profiles = map[string]*model.User{user4.ID: user4}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap4Off)
	require.Len(t, mentions, 6, "should've returned six mention keywords")
	ids, ok = mentions["user"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of user")
	ids, ok = mentions["@user"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of @user")
	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of mention")
	ids, ok = mentions["First"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of First")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of @all")

	// Channel-wide mentions are ignored on channel level
	channelMemberNotifyPropsMap4On := map[string]model.StringMap{
		user4.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOn,
		},
	}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap4On)
	require.Len(t, mentions, 4, "should've returned four mention keywords")
	ids, ok = mentions["user"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of user")
	ids, ok = mentions["@user"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of @user")
	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of mention")
	ids, ok = mentions["First"]
	require.True(t, ok)
	require.Equal(t, user4.ID, ids[0], "should've returned mention key of First")
	dup_count := func(list []string) map[string]int {

		duplicate_frequency := make(map[string]int)

		for _, item := range list {
			// check if the item/element exist in the duplicate_frequency map

			_, exist := duplicate_frequency[item]

			if exist {
				duplicate_frequency[item] += 1 // increase counter by 1 if already in the map
			} else {
				duplicate_frequency[item] = 1 // else start counting from 1
			}
		}
		return duplicate_frequency
	}

	// multiple users but no more than MaxNotificationsPerChannel
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.MaxNotificationsPerChannel = 4 })
	profiles = map[string]*model.User{
		user1.ID: user1,
		user2.ID: user2,
		user3.ID: user3,
		user4.ID: user4,
	}
	// Channel-wide mentions are not ignored on channel level for all users
	channelMemberNotifyPropsMap5Off := map[string]model.StringMap{
		user1.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
		user2.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
		user3.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
		user4.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
	}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap5Off)
	require.Len(t, mentions, 6, "should've returned six mention keywords")
	ids, ok = mentions["user"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user1.ID && ids[1] != user1.ID, "should've mentioned user1  with user")
	require.False(t, ids[0] != user4.ID && ids[1] != user4.ID, "should've mentioned user4  with user")
	idsMap := dup_count(mentions["@user"])
	require.True(t, ok)
	require.Len(t, idsMap, 4)
	require.Equal(t, idsMap[user1.ID], 2, "should've mentioned user1 with @user")
	require.Equal(t, idsMap[user4.ID], 2, "should've mentioned user4 with @user")

	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user1.ID && ids[1] != user1.ID, "should've mentioned user1 with mention")
	require.False(t, ids[0] != user4.ID && ids[1] != user4.ID, "should've mentioned user4 with mention")
	ids, ok = mentions["First"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user2.ID && ids[1] != user2.ID, "should've mentioned user2 with First")
	require.False(t, ids[0] != user4.ID && ids[1] != user4.ID, "should've mentioned user4 with First")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user3.ID && ids[1] != user3.ID, "should've mentioned user3 with @channel")
	require.False(t, ids[0] != user4.ID && ids[1] != user4.ID, "should've mentioned user4 with @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user3.ID && ids[1] != user3.ID, "should've mentioned user3 with @all")
	require.False(t, ids[0] != user4.ID && ids[1] != user4.ID, "should've mentioned user4 with @all")

	// multiple users and more than MaxNotificationsPerChannel
	mentions = th.App.getMentionKeywordsInChannel(profiles, false, channelMemberNotifyPropsMap4Off)
	require.Len(t, mentions, 4, "should've returned four mention keywords")
	_, ok = mentions["@channel"]
	require.False(t, ok, "should not have mentioned any user with @channel")
	_, ok = mentions["@all"]
	require.False(t, ok, "should not have mentioned any user with @all")
	_, ok = mentions["@here"]
	require.False(t, ok, "should not have mentioned any user with @here")
	// no special mentions
	profiles = map[string]*model.User{
		user1.ID: user1,
	}
	mentions = th.App.getMentionKeywordsInChannel(profiles, false, channelMemberNotifyPropsMap4Off)
	require.Len(t, mentions, 3, "should've returned three mention keywords")
	ids, ok = mentions["user"]
	require.True(t, ok)
	require.Len(t, ids, 1)
	require.Equal(t, user1.ID, ids[0], "should've mentioned user1 with user")
	ids, ok = mentions["@user"]

	require.True(t, ok)
	require.Len(t, ids, 2)
	require.Equal(t, user1.ID, ids[0], "should've mentioned user1 twice with @user")
	require.Equal(t, user1.ID, ids[1], "should've mentioned user1 twice with @user")

	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Len(t, ids, 1)
	require.Equal(t, user1.ID, ids[0], "should've mentioned user1 with user")

	_, ok = mentions["First"]
	require.False(t, ok, "should not have mentioned user1 with First")
	_, ok = mentions["@channel"]
	require.False(t, ok, "should not have mentioned any user with @channel")
	_, ok = mentions["@all"]
	require.False(t, ok, "should not have mentioned any user with @all")
	_, ok = mentions["@here"]
	require.False(t, ok, "should not have mentioned any user with @here")

	// user with empty mention keys
	userNoMentionKeys := &model.User{
		ID:        model.NewID(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"mention_keys": ",",
		},
	}

	channelMemberNotifyPropsMapEmptyOff := map[string]model.StringMap{
		userNoMentionKeys.ID: {
			"ignore_channel_mentions": model.IgnoreChannelMentionsOff,
		},
	}

	profiles = map[string]*model.User{userNoMentionKeys.ID: userNoMentionKeys}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMapEmptyOff)
	assert.Equal(t, 1, len(mentions), "should've returned one metion keyword")
	ids, ok = mentions["@user"]
	assert.True(t, ok)
	assert.Equal(t, userNoMentionKeys.ID, ids[0], "should've returned mention key of @user")
}

func TestAddMentionKeywordsForUser(t *testing.T) {
	t.Run("should add @user", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.Contains(t, keywords["@user"], user.ID)
	})

	t.Run("should add custom mention keywords", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
			NotifyProps: map[string]string{
				model.MentionKeysNotifyProp: "apple,BANANA,OrAnGe",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.Contains(t, keywords["apple"], user.ID)
		assert.Contains(t, keywords["banana"], user.ID)
		assert.Contains(t, keywords["orange"], user.ID)
	})

	t.Run("should not add empty custom keywords", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
			NotifyProps: map[string]string{
				model.MentionKeysNotifyProp: ",,",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.Nil(t, keywords[""])
	})

	t.Run("should add case sensitive first name if enabled", func(t *testing.T) {
		user := &model.User{
			ID:        model.NewID(),
			Username:  "user",
			FirstName: "William",
			LastName:  "Robert",
			NotifyProps: map[string]string{
				model.FirstNameNotifyProp: "true",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.Contains(t, keywords["William"], user.ID)
		assert.NotContains(t, keywords["william"], user.ID)
		assert.NotContains(t, keywords["Robert"], user.ID)
	})

	t.Run("should not add case sensitive first name if enabled but empty First Name", func(t *testing.T) {
		user := &model.User{
			ID:        model.NewID(),
			Username:  "user",
			FirstName: "",
			LastName:  "Robert",
			NotifyProps: map[string]string{
				model.FirstNameNotifyProp: "true",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.NotContains(t, keywords[""], user.ID)
	})

	t.Run("should not add case sensitive first name if disabled", func(t *testing.T) {
		user := &model.User{
			ID:        model.NewID(),
			Username:  "user",
			FirstName: "William",
			LastName:  "Robert",
			NotifyProps: map[string]string{
				model.FirstNameNotifyProp: "false",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.NotContains(t, keywords["William"], user.ID)
		assert.NotContains(t, keywords["william"], user.ID)
		assert.NotContains(t, keywords["Robert"], user.ID)
	})

	t.Run("should add @channel/@all/@here when allowed", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
			NotifyProps: map[string]string{
				model.ChannelMentionsNotifyProp: "true",
			},
		}
		channelNotifyProps := map[string]string{}
		status := &model.Status{
			Status: model.StatusOnline,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.Contains(t, keywords["@channel"], user.ID)
		assert.Contains(t, keywords["@all"], user.ID)
		assert.Contains(t, keywords["@here"], user.ID)
	})

	t.Run("should not add @channel/@all/@here when not allowed", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
			NotifyProps: map[string]string{
				model.ChannelMentionsNotifyProp: "true",
			},
		}
		channelNotifyProps := map[string]string{}
		status := &model.Status{
			Status: model.StatusOnline,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, false)

		assert.NotContains(t, keywords["@channel"], user.ID)
		assert.NotContains(t, keywords["@all"], user.ID)
		assert.NotContains(t, keywords["@here"], user.ID)
	})

	t.Run("should not add @channel/@all/@here when disabled for user", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
			NotifyProps: map[string]string{
				model.ChannelMentionsNotifyProp: "false",
			},
		}
		channelNotifyProps := map[string]string{}
		status := &model.Status{
			Status: model.StatusOnline,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.NotContains(t, keywords["@channel"], user.ID)
		assert.NotContains(t, keywords["@all"], user.ID)
		assert.NotContains(t, keywords["@here"], user.ID)
	})

	t.Run("should not add @channel/@all/@here when disabled for channel", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
			NotifyProps: map[string]string{
				model.ChannelMentionsNotifyProp: "true",
			},
		}
		channelNotifyProps := map[string]string{
			model.IgnoreChannelMentionsNotifyProp: model.IgnoreChannelMentionsOn,
		}
		status := &model.Status{
			Status: model.StatusOnline,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.NotContains(t, keywords["@channel"], user.ID)
		assert.NotContains(t, keywords["@all"], user.ID)
		assert.NotContains(t, keywords["@here"], user.ID)
	})

	t.Run("should not add @channel/@all/@here when channel is muted and channel mention setting is not updated by user", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
			NotifyProps: map[string]string{
				model.ChannelMentionsNotifyProp: "true",
			},
		}
		channelNotifyProps := map[string]string{
			model.MarkUnreadNotifyProp:            model.UserNotifyMention,
			model.IgnoreChannelMentionsNotifyProp: model.IgnoreChannelMentionsDefault,
		}
		status := &model.Status{
			Status: model.StatusOnline,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.NotContains(t, keywords["@channel"], user.ID)
		assert.NotContains(t, keywords["@all"], user.ID)
		assert.NotContains(t, keywords["@here"], user.ID)
	})

	t.Run("should not add @here when when user is not online", func(t *testing.T) {
		user := &model.User{
			ID:       model.NewID(),
			Username: "user",
			NotifyProps: map[string]string{
				model.ChannelMentionsNotifyProp: "true",
			},
		}
		channelNotifyProps := map[string]string{}
		status := &model.Status{
			Status: model.StatusAway,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.Contains(t, keywords["@channel"], user.ID)
		assert.Contains(t, keywords["@all"], user.ID)
		assert.NotContains(t, keywords["@here"], user.ID)
	})

	t.Run("should add for multiple users", func(t *testing.T) {
		user1 := &model.User{
			ID:       model.NewID(),
			Username: "user1",
			NotifyProps: map[string]string{
				model.ChannelMentionsNotifyProp: "true",
			},
		}
		user2 := &model.User{
			ID:       model.NewID(),
			Username: "user2",
			NotifyProps: map[string]string{
				model.ChannelMentionsNotifyProp: "true",
			},
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user1, map[string]string{}, nil, true)
		addMentionKeywordsForUser(keywords, user2, map[string]string{}, nil, true)

		assert.Contains(t, keywords["@user1"], user1.ID)
		assert.Contains(t, keywords["@user2"], user2.ID)
		assert.Contains(t, keywords["@all"], user1.ID)
		assert.Contains(t, keywords["@all"], user2.ID)
	})
}

func TestGetMentionsEnabledFields(t *testing.T) {

	attachmentWithTextAndPreText := model.SlackAttachment{
		Text:    "@here with mentions",
		Pretext: "@Channel some comment for the channel",
	}

	attachmentWithOutPreText := model.SlackAttachment{
		Text: "some text",
	}
	attachments := []*model.SlackAttachment{
		&attachmentWithTextAndPreText,
		&attachmentWithOutPreText,
	}

	post := &model.Post{
		Message: "This is the message",
		Props: model.StringInterface{
			"attachments": attachments,
		},
	}
	expectedFields := []string{
		"This is the message",
		"@Channel some comment for the channel",
		"@here with mentions",
		"some text"}

	mentionEnabledFields := getMentionsEnabledFields(post)

	assert.EqualValues(t, 4, len(mentionEnabledFields))
	assert.EqualValues(t, expectedFields, mentionEnabledFields)
}

func TestPostNotificationGetChannelName(t *testing.T) {
	sender := &model.User{ID: model.NewID(), Username: "sender", FirstName: "Sender", LastName: "Sender", Nickname: "Sender"}
	recipient := &model.User{ID: model.NewID(), Username: "recipient", FirstName: "Recipient", LastName: "Recipient", Nickname: "Recipient"}
	otherUser := &model.User{ID: model.NewID(), Username: "other", FirstName: "Other", LastName: "Other", Nickname: "Other"}
	profileMap := map[string]*model.User{
		sender.ID:    sender,
		recipient.ID: recipient,
		otherUser.ID: otherUser,
	}

	for name, testCase := range map[string]struct {
		channel     *model.Channel
		nameFormat  string
		recipientID string
		expected    string
	}{
		"regular channel": {
			channel:  &model.Channel{Type: model.ChannelTypeOpen, Name: "channel", DisplayName: "My Channel"},
			expected: "My Channel",
		},
		"direct channel, unspecified": {
			channel:  &model.Channel{Type: model.ChannelTypeDirect},
			expected: "@sender",
		},
		"direct channel, username": {
			channel:    &model.Channel{Type: model.ChannelTypeDirect},
			nameFormat: model.ShowUsername,
			expected:   "@sender",
		},
		"direct channel, full name": {
			channel:    &model.Channel{Type: model.ChannelTypeDirect},
			nameFormat: model.ShowFullName,
			expected:   "Sender Sender",
		},
		"direct channel, nickname": {
			channel:    &model.Channel{Type: model.ChannelTypeDirect},
			nameFormat: model.ShowNicknameFullName,
			expected:   "Sender",
		},
		"group channel, unspecified": {
			channel:  &model.Channel{Type: model.ChannelTypeGroup},
			expected: "other, sender",
		},
		"group channel, username": {
			channel:    &model.Channel{Type: model.ChannelTypeGroup},
			nameFormat: model.ShowUsername,
			expected:   "other, sender",
		},
		"group channel, full name": {
			channel:    &model.Channel{Type: model.ChannelTypeGroup},
			nameFormat: model.ShowFullName,
			expected:   "Other Other, Sender Sender",
		},
		"group channel, nickname": {
			channel:    &model.Channel{Type: model.ChannelTypeGroup},
			nameFormat: model.ShowNicknameFullName,
			expected:   "Other, Sender",
		},
		"group channel, not excluding current user": {
			channel:     &model.Channel{Type: model.ChannelTypeGroup},
			nameFormat:  model.ShowNicknameFullName,
			expected:    "Other, Sender",
			recipientID: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			notification := &PostNotification{
				Channel:    testCase.channel,
				Sender:     sender,
				ProfileMap: profileMap,
			}

			recipientID := recipient.ID
			if testCase.recipientID != "" {
				recipientID = testCase.recipientID
			}

			assert.Equal(t, testCase.expected, notification.GetChannelName(testCase.nameFormat, recipientID))
		})
	}
}

func TestPostNotificationGetSenderName(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	defaultChannel := &model.Channel{Type: model.ChannelTypeOpen}
	defaultPost := &model.Post{Props: model.StringInterface{}}
	sender := &model.User{ID: model.NewID(), Username: "sender", FirstName: "Sender", LastName: "Sender", Nickname: "Sender"}

	overriddenPost := &model.Post{
		Props: model.StringInterface{
			"override_username": "Overridden",
			"from_webhook":      "true",
		},
	}

	for name, testCase := range map[string]struct {
		channel        *model.Channel
		post           *model.Post
		nameFormat     string
		allowOverrides bool
		expected       string
	}{
		"name format unspecified": {
			expected: "@" + sender.Username,
		},
		"name format username": {
			nameFormat: model.ShowUsername,
			expected:   "@" + sender.Username,
		},
		"name format full name": {
			nameFormat: model.ShowFullName,
			expected:   sender.FirstName + " " + sender.LastName,
		},
		"name format nickname": {
			nameFormat: model.ShowNicknameFullName,
			expected:   sender.Nickname,
		},
		"system message": {
			post:     &model.Post{Type: model.PostSystemMessagePrefix + "custom"},
			expected: i18n.T("system.message.name"),
		},
		"overridden username": {
			post:           overriddenPost,
			allowOverrides: true,
			expected:       overriddenPost.GetProp("override_username").(string),
		},
		"overridden username, direct channel": {
			channel:        &model.Channel{Type: model.ChannelTypeDirect},
			post:           overriddenPost,
			allowOverrides: true,
			expected:       "@" + sender.Username,
		},
		"overridden username, overrides disabled": {
			post:           overriddenPost,
			allowOverrides: false,
			expected:       "@" + sender.Username,
		},
	} {
		t.Run(name, func(t *testing.T) {
			channel := defaultChannel
			if testCase.channel != nil {
				channel = testCase.channel
			}

			post := defaultPost
			if testCase.post != nil {
				post = testCase.post
			}

			notification := &PostNotification{
				Channel: channel,
				Post:    post,
				Sender:  sender,
			}

			assert.Equal(t, testCase.expected, notification.GetSenderName(testCase.nameFormat, testCase.allowOverrides))
		})
	}
}

func TestIsKeywordMultibyte(t *testing.T) {
	id1 := model.NewID()

	for name, tc := range map[string]struct {
		Message     string
		Attachments []*model.SlackAttachment
		Keywords    map[string][]string
		Groups      map[string]*model.Group
		Expected    *ExplicitMentions
	}{
		"MultibyteCharacter": {
			Message:  "My name is 萌",
			Keywords: map[string][]string{"萌": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterWithNoUser": {
			Message:  "My name is 萌",
			Keywords: map[string][]string{"萌": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
		"MultibyteCharacterAtBeginningOfSentence": {
			Message:  "이메일을 보내다.",
			Keywords: map[string][]string{"이메일": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterAtBeginningOfSentenceWithNoUser": {
			Message:  "이메일을 보내다.",
			Keywords: map[string][]string{"이메일": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
		"MultibyteCharacterInPartOfSentence": {
			Message:  "我爱吃番茄炒饭",
			Keywords: map[string][]string{"番茄": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterInPartOfSentenceWithNoUser": {
			Message:  "我爱吃番茄炒饭",
			Keywords: map[string][]string{"番茄": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
		"MultibyteCharacterAtEndOfSentence": {
			Message:  "こんにちは、世界",
			Keywords: map[string][]string{"世界": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterAtEndOfSentenceWithNoUser": {
			Message:  "こんにちは、世界",
			Keywords: map[string][]string{"世界": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
		"MultibyteCharacterTwiceInSentence": {
			Message:  "石橋さんが石橋を渡る",
			Keywords: map[string][]string{"石橋": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterTwiceInSentenceWithNoUser": {
			Message:  "石橋さんが石橋を渡る",
			Keywords: map[string][]string{"石橋": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			post := &model.Post{
				Message: tc.Message,
				Props: model.StringInterface{
					"attachments": tc.Attachments,
				},
			}

			m := getExplicitMentions(post, tc.Keywords, tc.Groups)
			assert.EqualValues(t, tc.Expected, m)
		})
	}
}

func TestAddMention(t *testing.T) {
	t.Run("should initialize Mentions and store new mentions", func(t *testing.T) {
		m := &ExplicitMentions{}

		userID1 := model.NewID()
		userID2 := model.NewID()

		m.addMention(userID1, KeywordMention)
		m.addMention(userID2, CommentMention)

		assert.Equal(t, map[string]MentionType{
			userID1: KeywordMention,
			userID2: CommentMention,
		}, m.Mentions)
	})

	t.Run("should replace existing mentions with higher priority ones", func(t *testing.T) {
		m := &ExplicitMentions{}

		userID1 := model.NewID()
		userID2 := model.NewID()

		m.addMention(userID1, ThreadMention)
		m.addMention(userID2, DMMention)

		m.addMention(userID1, ChannelMention)
		m.addMention(userID2, KeywordMention)

		assert.Equal(t, map[string]MentionType{
			userID1: ChannelMention,
			userID2: KeywordMention,
		}, m.Mentions)
	})

	t.Run("should not replace high priority mentions with low priority ones", func(t *testing.T) {
		m := &ExplicitMentions{}

		userID1 := model.NewID()
		userID2 := model.NewID()

		m.addMention(userID1, KeywordMention)
		m.addMention(userID2, CommentMention)

		m.addMention(userID1, DMMention)
		m.addMention(userID2, ThreadMention)

		assert.Equal(t, map[string]MentionType{
			userID1: KeywordMention,
			userID2: CommentMention,
		}, m.Mentions)
	})
}

func TestCheckForMentionUsers(t *testing.T) {
	id1 := model.NewID()
	id2 := model.NewID()

	for name, tc := range map[string]struct {
		Word        string
		Attachments []*model.SlackAttachment
		Keywords    map[string][]string
		Expected    *ExplicitMentions
	}{
		"Nobody": {
			Word:     "nothing",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"UppercaseUser1": {
			Word:     "@User",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"LowercaseUser1": {
			Word:     "@user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"LowercaseUser2": {
			Word:     "@user2",
			Keywords: map[string][]string{"@user2": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id2: KeywordMention,
				},
			},
		},
		"UppercaseUser2": {
			Word:     "@UsEr2",
			Keywords: map[string][]string{"@user2": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id2: KeywordMention,
				},
			},
		},
		"HereMention": {
			Word: "@here",
			Expected: &ExplicitMentions{
				HereMentioned: true,
			},
		},
		"ChannelMention": {
			Word: "@channel",
			Expected: &ExplicitMentions{
				ChannelMentioned: true,
			},
		},
		"AllMention": {
			Word: "@all",
			Expected: &ExplicitMentions{
				AllMentioned: true,
			},
		},
		"UppercaseHere": {
			Word: "@HeRe",
			Expected: &ExplicitMentions{
				HereMentioned: true,
			},
		},
		"UppercaseChannel": {
			Word: "@ChaNNel",
			Expected: &ExplicitMentions{
				ChannelMentioned: true,
			},
		},
		"UppercaseAll": {
			Word: "@ALL",
			Expected: &ExplicitMentions{
				AllMentioned: true,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {

			e := &ExplicitMentions{}
			e.checkForMention(tc.Word, tc.Keywords, nil)

			assert.EqualValues(t, tc.Expected, e)
		})
	}
}

func TestAddGroupMention(t *testing.T) {
	for name, tc := range map[string]struct {
		Word     string
		Groups   map[string]*model.Group
		Expected bool
	}{
		"No groups": {
			Word:     "nothing",
			Groups:   map[string]*model.Group{},
			Expected: false,
		},
		"No matching groups": {
			Word:     "nothing",
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: false,
		},
		"matching group with no @": {
			Word:     "engineering",
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: false,
		},
		"matching group with preceding @": {
			Word:     "@engineering",
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: true,
		},
		"matching upper case group with preceding @": {
			Word:     "@Engineering",
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := &ExplicitMentions{}
			groupFound := e.addGroupMention(tc.Word, tc.Groups)

			if groupFound {
				require.Equal(t, len(e.GroupMentions), 1)
			}

			require.Equal(t, tc.Expected, groupFound)
		})
	}
}

func TestProcessText(t *testing.T) {
	id1 := model.NewID()

	for name, tc := range map[string]struct {
		Text     string
		Keywords map[string][]string
		Groups   map[string]*model.Group
		Expected *ExplicitMentions
	}{
		"Mention user in text": {
			Text:     "hello user @user1",
			Keywords: map[string][]string{"@user1": {id1}},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Mention user after ending a sentence with full stop": {
			Text:     "hello user.@user1",
			Keywords: map[string][]string{"@user1": {id1}},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Mention user after hyphen": {
			Text:     "hello user-@user1",
			Keywords: map[string][]string{"@user1": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Mention user after colon": {
			Text:     "hello user:@user1",
			Keywords: map[string][]string{"@user1": {id1}},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Mention here after colon": {
			Text:     "hello all:@here",
			Keywords: map[string][]string{},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				HereMentioned: true,
			},
		},
		"Mention all after hyphen": {
			Text:     "hello all-@all",
			Keywords: map[string][]string{},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				AllMentioned: true,
			},
		},
		"Mention channel after full stop": {
			Text:     "hello channel.@channel",
			Keywords: map[string][]string{},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				ChannelMentioned: true,
			},
		},
		"Mention other pontential users or system calls": {
			Text:     "hello @potentialuser and @otherpotentialuser",
			Keywords: map[string][]string{},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"potentialuser", "otherpotentialuser"},
			},
		},
		"Mention a real user and another potential user": {
			Text:     "@user1, you can use @systembot to get help",
			Keywords: map[string][]string{"@user1": {id1}},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
				OtherPotentialMentions: []string{"systembot"},
			},
		},
		"Mention a group": {
			Text:     "@engineering",
			Keywords: map[string][]string{"@user1": {id1}},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				GroupMentions:          map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}},
				OtherPotentialMentions: []string{"engineering"},
			},
		},
		"Mention a real user and another potential user and a group": {
			Text:     "@engineering @user1, you can use @systembot to get help from",
			Keywords: map[string][]string{"@user1": {id1}},
			Groups:   map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}, "developers": {Name: model.NewString("developers")}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
				GroupMentions:          map[string]*model.Group{"engineering": {Name: model.NewString("engineering")}},
				OtherPotentialMentions: []string{"engineering", "systembot"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := &ExplicitMentions{}
			e.processText(tc.Text, tc.Keywords, tc.Groups)

			assert.EqualValues(t, tc.Expected, e)
		})
	}
}

func TestGetNotificationNameFormat(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("show full name on", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.ShowFullName = true
			*cfg.TeamSettings.TeammateNameDisplay = model.ShowFullName
		})

		assert.Equal(t, model.ShowFullName, th.App.GetNotificationNameFormat(th.BasicUser))
	})

	t.Run("show full name off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.ShowFullName = false
			*cfg.TeamSettings.TeammateNameDisplay = model.ShowFullName
		})

		assert.Equal(t, model.ShowUsername, th.App.GetNotificationNameFormat(th.BasicUser))
	})
}

func TestUserAllowsEmail(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("should return true", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOffline(user.ID, true)

		channelMemberNotificationProps := model.StringMap{
			model.EmailNotifyProp:      model.ChannelNotifyDefault,
			model.MarkUnreadNotifyProp: model.ChannelMarkUnreadAll,
		}

		assert.True(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: "some-post-type"}))
	})

	t.Run("should return false in case the status is ONLINE", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOnline(user.ID, true)

		channelMemberNotificationProps := model.StringMap{
			model.EmailNotifyProp:      model.ChannelNotifyDefault,
			model.MarkUnreadNotifyProp: model.ChannelMarkUnreadAll,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: "some-post-type"}))
	})

	t.Run("should return false in case the EMAIL_NOTIFY_PROP is false", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOffline(user.ID, true)

		channelMemberNotificationProps := model.StringMap{
			model.EmailNotifyProp:      "false",
			model.MarkUnreadNotifyProp: model.ChannelMarkUnreadAll,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: "some-post-type"}))
	})

	t.Run("should return false in case the MARK_UNREAD_NOTIFY_PROP is CHANNEL_MARK_UNREAD_MENTION", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOffline(user.ID, true)

		channelMemberNotificationProps := model.StringMap{
			model.EmailNotifyProp:      model.ChannelNotifyDefault,
			model.MarkUnreadNotifyProp: model.ChannelMarkUnreadMention,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: "some-post-type"}))
	})

	t.Run("should return false in case the Post type is POST_AUTO_RESPONDER", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOffline(user.ID, true)

		channelMemberNotificationProps := model.StringMap{
			model.EmailNotifyProp:      model.ChannelNotifyDefault,
			model.MarkUnreadNotifyProp: model.ChannelMarkUnreadAll,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: model.PostTypeAutoResponder}))
	})

	t.Run("should return false in case the status is STATUS_OUT_OF_OFFICE", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOutOfOffice(user.ID)

		channelMemberNotificationProps := model.StringMap{
			model.EmailNotifyProp:      model.ChannelNotifyDefault,
			model.MarkUnreadNotifyProp: model.ChannelMarkUnreadAll,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: model.PostTypeAutoResponder}))
	})

	t.Run("should return false in case the status is STATUS_ONLINE", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusDoNotDisturb(user.ID)

		channelMemberNotificationProps := model.StringMap{
			model.EmailNotifyProp:      model.ChannelNotifyDefault,
			model.MarkUnreadNotifyProp: model.ChannelMarkUnreadAll,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: model.PostTypeAutoResponder}))
	})

}

func TestInsertGroupMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team := th.BasicTeam
	channel := th.BasicChannel
	group := th.CreateGroup()
	group.DisplayName = "engineering"
	group.Name = model.NewString("engineering")
	group, err := th.App.UpdateGroup(group)
	require.Nil(t, err)

	groupChannelMember := th.CreateUser()
	th.LinkUserToTeam(groupChannelMember, team)
	th.App.AddUserToChannel(groupChannelMember, channel, false)
	_, err = th.App.UpsertGroupMember(group.ID, groupChannelMember.ID)
	require.Nil(t, err)

	nonGroupChannelMember := th.CreateUser()
	th.LinkUserToTeam(nonGroupChannelMember, team)
	th.App.AddUserToChannel(nonGroupChannelMember, channel, false)

	nonChannelGroupMember := th.CreateUser()
	th.LinkUserToTeam(nonChannelGroupMember, team)
	_, err = th.App.UpsertGroupMember(group.ID, nonChannelGroupMember.ID)
	require.Nil(t, err)

	groupWithNoMembers := th.CreateGroup()
	groupWithNoMembers.DisplayName = "marketing"
	groupWithNoMembers.Name = model.NewString("marketing")
	groupWithNoMembers, err = th.App.UpdateGroup(groupWithNoMembers)
	require.Nil(t, err)

	profileMap := map[string]*model.User{groupChannelMember.ID: groupChannelMember, nonGroupChannelMember.ID: nonGroupChannelMember}

	t.Run("should add expected mentions for users part of the mentioned group", func(t *testing.T) {
		mentions := &ExplicitMentions{}
		usersMentioned, err := th.App.insertGroupMentions(group, channel, profileMap, mentions)
		require.Nil(t, err)
		require.Equal(t, usersMentioned, true)

		// Ensure group member that is also a channel member is added to the mentions list.
		require.Equal(t, len(mentions.Mentions), 1)
		_, found := mentions.Mentions[groupChannelMember.ID]
		require.Equal(t, found, true)

		// Ensure group member that is not a channel member is added to the other potential mentions list.
		require.Equal(t, len(mentions.OtherPotentialMentions), 1)
		require.Equal(t, mentions.OtherPotentialMentions[0], nonChannelGroupMember.Username)
	})

	t.Run("should add no expected or potential mentions if the group has no users ", func(t *testing.T) {
		mentions := &ExplicitMentions{}
		usersMentioned, err := th.App.insertGroupMentions(groupWithNoMembers, channel, profileMap, mentions)
		require.Nil(t, err)
		require.Equal(t, usersMentioned, false)

		// Ensure no mentions are added for a group with no users
		require.Equal(t, len(mentions.Mentions), 0)
		require.Equal(t, len(mentions.OtherPotentialMentions), 0)
	})

	t.Run("should keep existing mentions", func(t *testing.T) {
		mentions := &ExplicitMentions{}
		th.App.insertGroupMentions(group, channel, profileMap, mentions)
		th.App.insertGroupMentions(groupWithNoMembers, channel, profileMap, mentions)

		// Ensure mentions from group are kept after running with groupWithNoMembers
		require.Equal(t, len(mentions.Mentions), 1)
		require.Equal(t, len(mentions.OtherPotentialMentions), 1)
	})

	t.Run("should return true if no members mentioned while in group or direct message channel", func(t *testing.T) {
		mentions := &ExplicitMentions{}
		emptyProfileMap := make(map[string]*model.User)

		groupChannel := &model.Channel{Type: model.ChannelTypeGroup}
		usersMentioned, _ := th.App.insertGroupMentions(group, groupChannel, emptyProfileMap, mentions)
		// Ensure group channel with no group members mentioned always returns true
		require.Equal(t, usersMentioned, true)
		require.Equal(t, len(mentions.Mentions), 0)

		directChannel := &model.Channel{Type: model.ChannelTypeDirect}
		usersMentioned, _ = th.App.insertGroupMentions(group, directChannel, emptyProfileMap, mentions)
		// Ensure direct channel with no group members mentioned always returns true
		require.Equal(t, usersMentioned, true)
		require.Equal(t, len(mentions.Mentions), 0)
	})

	t.Run("should add mentions for members while in group channel", func(t *testing.T) {
		groupChannel, err := th.App.CreateGroupChannel([]string{groupChannelMember.ID, nonGroupChannelMember.ID, th.BasicUser.ID}, groupChannelMember.ID)
		require.Nil(t, err)

		mentions := &ExplicitMentions{}
		th.App.insertGroupMentions(group, groupChannel, profileMap, mentions)

		require.Equal(t, len(mentions.Mentions), 1)
		_, found := mentions.Mentions[groupChannelMember.ID]
		require.Equal(t, found, true)
	})
}

func TestGetGroupsAllowedForReferenceInChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	var err *model.AppError

	team := th.BasicTeam
	channel := th.BasicChannel
	group1 := th.CreateGroup()

	t.Run("should return empty map when no groups with allow reference", func(t *testing.T) {
		groupsMap, nErr := th.App.getGroupsAllowedForReferenceInChannel(channel, team)
		require.NoError(t, nErr)
		require.Len(t, groupsMap, 0)
	})

	group1.AllowReference = true
	group1, err = th.App.UpdateGroup(group1)
	require.Nil(t, err)

	group2 := th.CreateGroup()
	t.Run("should only return groups with allow reference", func(t *testing.T) {
		groupsMap, nErr := th.App.getGroupsAllowedForReferenceInChannel(channel, team)
		require.NoError(t, nErr)
		require.Len(t, groupsMap, 1)
		require.Nil(t, groupsMap[*group2.Name])
		require.Equal(t, groupsMap[*group1.Name], group1)
	})

	group2.AllowReference = true
	group2, err = th.App.UpdateGroup(group2)
	require.Nil(t, err)

	// Sync first group to constrained channel
	constrainedChannel := th.CreateChannel(th.BasicTeam)
	constrainedChannel.GroupConstrained = model.NewBool(true)
	constrainedChannel, err = th.App.UpdateChannel(constrainedChannel)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupID:    group1.ID,
		Type:       model.GroupSyncableTypeChannel,
		SyncableID: constrainedChannel.ID,
	})
	require.Nil(t, err)

	t.Run("should return only groups synced to channel if channel is group constrained", func(t *testing.T) {
		groupsMap, nErr := th.App.getGroupsAllowedForReferenceInChannel(constrainedChannel, team)
		require.NoError(t, nErr)
		require.Len(t, groupsMap, 1)
		require.Nil(t, groupsMap[*group2.Name])
		require.Equal(t, groupsMap[*group1.Name], group1)
	})

	// Create a third group not synced with a team or channel
	group3 := th.CreateGroup()
	group3.AllowReference = true
	group3, err = th.App.UpdateGroup(group3)
	require.Nil(t, err)

	// Sync group2 to the team
	team.GroupConstrained = model.NewBool(true)
	team, err = th.App.UpdateTeam(team)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupID:    group2.ID,
		Type:       model.GroupSyncableTypeTeam,
		SyncableID: team.ID,
	})
	require.Nil(t, err)

	t.Run("should return union of groups synced to team and any channels if team is group constrained", func(t *testing.T) {
		groupsMap, nErr := th.App.getGroupsAllowedForReferenceInChannel(channel, team)
		require.NoError(t, nErr)
		require.Len(t, groupsMap, 2)
		require.Nil(t, groupsMap[*group3.Name])
		require.Equal(t, groupsMap[*group2.Name], group2)
		require.Equal(t, groupsMap[*group1.Name], group1)
	})

	t.Run("should return only subset of groups synced to channel for group constrained channel when team is also group constrained", func(t *testing.T) {
		groupsMap, nErr := th.App.getGroupsAllowedForReferenceInChannel(constrainedChannel, team)
		require.NoError(t, nErr)
		require.Len(t, groupsMap, 1)
		require.Nil(t, groupsMap[*group3.Name])
		require.Nil(t, groupsMap[*group2.Name])
		require.Equal(t, groupsMap[*group1.Name], group1)
	})

	team.GroupConstrained = model.NewBool(false)
	team, err = th.App.UpdateTeam(team)
	require.Nil(t, err)

	t.Run("should return all groups when team and channel are not group constrained", func(t *testing.T) {
		groupsMap, nErr := th.App.getGroupsAllowedForReferenceInChannel(channel, team)
		require.NoError(t, nErr)
		require.Len(t, groupsMap, 3)
		require.Equal(t, groupsMap[*group1.Name], group1)
		require.Equal(t, groupsMap[*group2.Name], group2)
		require.Equal(t, groupsMap[*group3.Name], group3)
	})
}

func TestReplyPostNotificationsWithCRT(t *testing.T) {
	t.Run("Reply posts only shows badges for explicit mentions in collapsed threads", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		u1 := th.BasicUser
		u2 := th.BasicUser2
		c1 := th.BasicChannel
		th.AddUserToChannel(u2, c1)

		// Enable "Trigger notifications on messages in
		// reply threads that I start or participate in"
		// for the second user
		oldValue := th.BasicUser2.NotifyProps[model.CommentsNotifyProp]
		newNotifyProps := th.BasicUser2.NotifyProps
		newNotifyProps[model.CommentsNotifyProp] = model.CommentsNotifyAny
		u2, appErr := th.App.PatchUser(th.BasicUser2.ID, &model.UserPatch{NotifyProps: newNotifyProps}, false)
		require.Nil(t, appErr)
		require.Equal(t, model.CommentsNotifyAny, u2.NotifyProps[model.CommentsNotifyProp])
		defer func() {
			newNotifyProps := th.BasicUser2.NotifyProps
			newNotifyProps[model.CommentsNotifyProp] = oldValue
			_, nAppErr := th.App.PatchUser(th.BasicUser2.ID, &model.UserPatch{NotifyProps: newNotifyProps}, false)
			require.Nil(t, nAppErr)
		}()

		// Enable CRT
		os.Setenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS", "true")
		defer os.Unsetenv("MM_FEATUREFLAGS_COLLAPSEDTHREADS")
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.ThreadAutoFollow = true
			*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDefaultOn
		})

		rootPost := &model.Post{
			ChannelID: c1.ID,
			Message:   "root post by user1",
			UserID:    u1.ID,
		}
		rpost, appErr := th.App.CreatePost(th.Context, rootPost, c1, false, true)
		require.Nil(t, appErr)

		replyPost1 := &model.Post{
			ChannelID: c1.ID,
			Message:   "reply post by user2",
			UserID:    u2.ID,
			RootID:    rpost.ID,
		}
		_, appErr = th.App.CreatePost(th.Context, replyPost1, c1, false, true)
		require.Nil(t, appErr)

		replyPost2 := &model.Post{
			ChannelID: c1.ID,
			Message:   "reply post by user1",
			UserID:    u1.ID,
			RootID:    rpost.ID,
		}
		_, appErr = th.App.CreatePost(th.Context, replyPost2, c1, false, true)
		require.Nil(t, appErr)

		threadMembership, appErr := th.App.GetThreadMembershipForUser(u2.ID, rpost.ID)
		require.Nil(t, appErr)
		thread, appErr := th.App.GetThreadForUser(c1.TeamID, threadMembership, false)
		require.Nil(t, appErr)
		// Then: with notifications set to "all" we should
		// not see a mention badge
		require.Equal(t, int64(0), thread.UnreadMentions)
		// Then: last post is still marked as unread
		require.Equal(t, int64(1), thread.UnreadReplies)
	})
}
