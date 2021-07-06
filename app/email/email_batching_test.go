// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package email

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestHandleNewNotifications(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	id1 := model.NewId()
	id2 := model.NewId()
	id3 := model.NewId()

	// test queueing of received posts by user
	job := NewEmailBatchingJob(th.service, 128)

	job.handleNewNotifications()

	require.Empty(t, job.pendingNotifications, "shouldn't have added any pending notifications")

	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test"}, &model.Team{Name: "team"})
	require.Empty(t, job.pendingNotifications, "shouldn't have added any pending notifications")

	job.handleNewNotifications()
	require.Len(t, job.pendingNotifications, 1, "should have received posts for 1 user")
	require.Len(t, job.pendingNotifications[id1], 1, "should have received 1 post for user")

	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test"}, &model.Team{Name: "team"})
	job.handleNewNotifications()
	require.Len(t, job.pendingNotifications, 1, "should have received posts for 1 user")
	require.Len(t, job.pendingNotifications[id1], 2, "should have received 2 posts for user1")

	job.Add(&model.User{Id: id2}, &model.Post{UserId: id1, Message: "test"}, &model.Team{Name: "team"})
	job.handleNewNotifications()
	require.Len(t, job.pendingNotifications, 2, "should have received posts for 2 users")
	require.Len(t, job.pendingNotifications[id1], 2, "should have received 2 posts for user1")
	require.Len(t, job.pendingNotifications[id2], 1, "should have received 1 post for user2")

	job.Add(&model.User{Id: id2}, &model.Post{UserId: id2, Message: "test"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id1}, &model.Post{UserId: id3, Message: "test"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id3}, &model.Post{UserId: id3, Message: "test"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id2}, &model.Post{UserId: id2, Message: "test"}, &model.Team{Name: "team"})
	job.handleNewNotifications()
	require.Len(t, job.pendingNotifications, 3, "should have received posts for 3 users")
	require.Len(t, job.pendingNotifications[id1], 3, "should have received 3 posts for user1")
	require.Len(t, job.pendingNotifications[id2], 3, "should have received 3 posts for user2")
	require.Len(t, job.pendingNotifications[id3], 1, "should have received 1 post for user3")

	// test ordering of received posts
	job = NewEmailBatchingJob(th.service, 128)

	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test1"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test2"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id2}, &model.Post{UserId: id1, Message: "test3"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test4"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id2}, &model.Post{UserId: id1, Message: "test5"}, &model.Team{Name: "team"})
	job.handleNewNotifications()
	assert.Equal(t, job.pendingNotifications[id1][0].post.Message, "test1", "incorrect order of received posts for user1")
	assert.Equal(t, job.pendingNotifications[id1][1].post.Message, "test2", "incorrect order of received posts for user1")
	assert.Equal(t, job.pendingNotifications[id1][2].post.Message, "test4", "incorrect order of received posts for user1")
	assert.Equal(t, job.pendingNotifications[id2][0].post.Message, "test3", "incorrect order of received posts for user2")
	assert.Equal(t, job.pendingNotifications[id2][1].post.Message, "test5", "incorrect order of received posts for user2")
}

func TestCheckPendingNotifications(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	job := NewEmailBatchingJob(th.service, 128)
	job.pendingNotifications[th.BasicUser.Id] = []*batchedNotification{
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10000000,
			},
			teamName: th.BasicTeam.Name,
		},
	}

	channelMember, err := th.store.Channel().GetMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	channelMember.LastViewedAt = 9999999
	_, err = th.store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)

	nErr := th.store.Preference().Save(&model.Preferences{{
		UserId:   th.BasicUser.Id,
		Category: model.PREFERENCE_CATEGORY_NOTIFICATIONS,
		Name:     model.PREFERENCE_NAME_EMAIL_INTERVAL,
		Value:    "60",
	}})
	require.NoError(t, nErr)

	// test that notifications aren't sent before interval
	job.checkPendingNotifications(time.Unix(10001, 0), func(string, []*batchedNotification) {})

	require.NotNil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Len(t, job.pendingNotifications[th.BasicUser.Id], 1, "shouldn't have sent queued post")

	// test that notifications are cleared if the user has acted
	channelMember, err = th.store.Channel().GetMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	channelMember.LastViewedAt = 10001000
	_, err = th.store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)

	// We reset the interval to something shorter
	nErr = th.store.Preference().Save(&model.Preferences{{
		UserId:   th.BasicUser.Id,
		Category: model.PREFERENCE_CATEGORY_NOTIFICATIONS,
		Name:     model.PREFERENCE_NAME_EMAIL_INTERVAL,
		Value:    "10",
	}})
	require.NoError(t, nErr)

	var wasCalled int32
	job.checkPendingNotifications(time.Unix(10050, 0), func(string, []*batchedNotification) {
		atomic.StoreInt32(&wasCalled, int32(1))
	})

	// A hack to check whether the handler was called.
	// It's not straightforward to just wait for it using a channel because the test should
	// NOT call the handler, and it will be called only if the test fails.
	time.Sleep(1 * time.Second)
	// We do a check outside the email handler, because otherwise, failing from
	// inside the handler doesn't let the .Go() function exit cleanly, and it gets
	// stuck during server shutdown, trying to wait for the goroutine to exit
	require.Equal(t, atomic.LoadInt32(&wasCalled), int32(0), "email handler should not have been called")

	require.Nil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Empty(t, job.pendingNotifications[th.BasicUser.Id], "should've remove queued post since user acted")

	// test that notifications are sent if enough time passes since the first message
	job.pendingNotifications[th.BasicUser.Id] = []*batchedNotification{
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10060000,
				Message:   "post1",
			},
			teamName: th.BasicTeam.Name,
		},
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10090000,
				Message:   "post2",
			},
			teamName: th.BasicTeam.Name,
		},
	}

	received := make(chan *model.Post, 2)

	job.checkPendingNotifications(time.Unix(10130, 0), func(s string, notifications []*batchedNotification) {
		for _, notification := range notifications {
			received <- notification.post
		}
	})

	require.Nil(t, job.pendingNotifications[th.BasicUser.Id], "shouldn't have sent queued post")

	select {
	case post := <-received:
		require.Equal(t, post.Message, "post1", "should've received post1 first")
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for first post notification")
	}

	select {
	case post := <-received:
		require.Equal(t, post.Message, "post2", "should've received post2 second")
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for second post notification")
	}
}

/**
 * Ensures that email batch interval defaults to 15 minutes for users that haven't explicitly set this preference
 */
func TestCheckPendingNotificationsDefaultInterval(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	job := NewEmailBatchingJob(th.service, 128)

	// bypasses recent user activity check
	require.NotNil(t, th.store)
	require.NotNil(t, th.store.Channel())

	require.NotNil(t, th.BasicUser)
	require.NotNil(t, th.BasicChannel)
	channelMember, err := th.store.Channel().GetMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	channelMember.LastViewedAt = 9999000
	_, err = th.store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)

	job.pendingNotifications[th.BasicUser.Id] = []*batchedNotification{
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10000000,
			},
			teamName: th.BasicTeam.Name,
		},
	}

	// notifications should not be sent 1s after post was created, because default batch interval is 15mins
	job.checkPendingNotifications(time.Unix(10001, 0), func(string, []*batchedNotification) {})
	require.NotNil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Len(t, job.pendingNotifications[th.BasicUser.Id], 1, "shouldn't have sent queued post")

	// notifications should be sent 901s after post was created, because default batch interval is 15mins
	job.checkPendingNotifications(time.Unix(10901, 0), func(string, []*batchedNotification) {})
	require.Nil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Empty(t, job.pendingNotifications[th.BasicUser.Id], "should have sent queued post")
}

/**
 * Ensures that email batch interval defaults to 15 minutes if user preference is invalid
 */
func TestCheckPendingNotificationsCantParseInterval(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	job := NewEmailBatchingJob(th.service, 128)

	require.NotNil(t, th.store)
	require.NotNil(t, th.store.Channel())
	require.NotNil(t, th.BasicChannel)
	require.NotNil(t, th.BasicUser)
	// bypasses recent user activity check
	channelMember, err := th.store.Channel().GetMember(context.Background(), th.BasicChannel.Id, th.BasicUser.Id)
	require.NoError(t, err)
	channelMember.LastViewedAt = 9999000
	_, err = th.store.Channel().UpdateMember(channelMember)
	require.NoError(t, err)

	// preference value is not an integer, so we'll fall back to the default 15min value
	nErr := th.store.Preference().Save(&model.Preferences{{
		UserId:   th.BasicUser.Id,
		Category: model.PREFERENCE_CATEGORY_NOTIFICATIONS,
		Name:     model.PREFERENCE_NAME_EMAIL_INTERVAL,
		Value:    "notAnIntegerValue",
	}})
	require.NoError(t, nErr)

	job.pendingNotifications[th.BasicUser.Id] = []*batchedNotification{
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10000000,
			},
			teamName: th.BasicTeam.Name,
		},
	}

	// notifications should not be sent 1s after post was created, because default batch interval is 15mins
	job.checkPendingNotifications(time.Unix(10001, 0), func(string, []*batchedNotification) {})
	require.NotNil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Len(t, job.pendingNotifications[th.BasicUser.Id], 1, "shouldn't have sent queued post")

	// notifications should be sent 901s after post was created, because default batch interval is 15mins
	job.checkPendingNotifications(time.Unix(10901, 0), func(string, []*batchedNotification) {})

	require.Nil(t, job.pendingNotifications[th.BasicUser.Id], "should have sent queued post")
}
