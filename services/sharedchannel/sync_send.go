// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sharedchannel

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/remotecluster"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type syncTask struct {
	id         string
	channelID  string
	remoteID   string
	AddedAt    time.Time
	retryCount int
	retryMsg   *syncMsg
	schedule   time.Time
}

func newSyncTask(channelID string, remoteID string, retryMsg *syncMsg) syncTask {
	var retryID string
	if retryMsg != nil {
		retryID = retryMsg.ID
	}

	return syncTask{
		id:        channelID + remoteID + retryID, // combination of ids to avoid duplicates
		channelID: channelID,
		remoteID:  remoteID, // empty means update all remote clusters
		retryMsg:  retryMsg,
		schedule:  time.Now(),
	}
}

// incRetry increments the retry counter and returns true if MaxRetries not exceeded.
func (st *syncTask) incRetry() bool {
	st.retryCount++
	return st.retryCount <= MaxRetries
}

// NotifyChannelChanged is called to indicate that a shared channel has been modified,
// thus triggering an update to all remote clusters.
func (scs *Service) NotifyChannelChanged(channelID string) {
	if rcs := scs.server.GetRemoteClusterService(); rcs == nil {
		return
	}

	task := newSyncTask(channelID, "", nil)
	task.schedule = time.Now().Add(NotifyMinimumDelay)
	scs.addTask(task)
}

// NotifyUserProfileChanged is called to indicate that a user belonging to at least one
// shared channel has modified their user profile (name, username, email, custom status, profile image)
func (scs *Service) NotifyUserProfileChanged(userID string) {
	if rcs := scs.server.GetRemoteClusterService(); rcs == nil {
		return
	}

	scusers, err := scs.server.GetStore().SharedChannel().GetUsersForUser(userID)
	if err != nil {
		scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Failed to fetch shared channel users",
			mlog.String("userID", userID),
			mlog.Err(err),
		)
		return
	}
	if len(scusers) == 0 {
		return
	}

	notified := make(map[string]struct{})

	for _, user := range scusers {
		// update every channel + remote combination they belong to.
		// Redundant updates (ie. to same remote for multiple channels) will be
		// filtered out.
		combo := user.ChannelID + user.RemoteID
		if _, ok := notified[combo]; ok {
			continue
		}
		notified[combo] = struct{}{}
		task := newSyncTask(user.ChannelID, user.RemoteID, nil)
		task.schedule = time.Now().Add(NotifyMinimumDelay)
		scs.addTask(task)
	}
}

// ForceSyncForRemote causes all channels shared with the remote to be synchronized.
func (scs *Service) ForceSyncForRemote(rc *model.RemoteCluster) {
	if rcs := scs.server.GetRemoteClusterService(); rcs == nil {
		return
	}

	// fetch all channels shared with this remote.
	opts := model.SharedChannelRemoteFilterOpts{
		RemoteID: rc.RemoteID,
	}
	scrs, err := scs.server.GetStore().SharedChannel().GetRemotes(opts)
	if err != nil {
		scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Failed to fetch shared channel remotes",
			mlog.String("remote", rc.DisplayName),
			mlog.String("remoteId", rc.RemoteID),
			mlog.Err(err),
		)
		return
	}

	for _, scr := range scrs {
		task := newSyncTask(scr.ChannelID, rc.RemoteID, nil)
		task.schedule = time.Now().Add(NotifyMinimumDelay)
		scs.addTask(task)
	}
}

// addTask adds or re-adds a task to the queue.
func (scs *Service) addTask(task syncTask) {
	task.AddedAt = time.Now()
	scs.mux.Lock()
	if _, ok := scs.tasks[task.id]; !ok {
		scs.tasks[task.id] = task
	}
	scs.mux.Unlock()

	// wake up the sync goroutine
	select {
	case scs.changeSignal <- struct{}{}:
	default:
		// that's ok, the sync routine is already busy
	}
}

// syncLoop is called via a dedicated goroutine to wait for notifications of channel changes and
// updates each remote based on those changes.
func (scs *Service) syncLoop(done chan struct{}) {
	// create a timer to periodically check the task queue, but only if there is
	// a delayed task in the queue.
	delay := time.NewTimer(NotifyMinimumDelay)
	defer stopTimer(delay)

	// wait for channel changed signal and update for oldest task.
	for {
		select {
		case <-scs.changeSignal:
			if wait := scs.doSync(); wait > 0 {
				stopTimer(delay)
				delay.Reset(wait)
			}
		case <-delay.C:
			if wait := scs.doSync(); wait > 0 {
				delay.Reset(wait)
			}
		case <-done:
			return
		}
	}
}

func stopTimer(timer *time.Timer) {
	timer.Stop()
	select {
	case <-timer.C:
	default:
	}
}

// doSync checks the task queue for any tasks to be processed and processes all that are ready.
// If any delayed tasks remain in queue then the duration until the next scheduled task is returned.
func (scs *Service) doSync() time.Duration {
	var task syncTask
	var ok bool
	var shortestWait time.Duration

	for {
		task, ok, shortestWait = scs.removeOldestTask()
		if !ok {
			break
		}
		if err := scs.processTask(task); err != nil {
			// put task back into map so it will update again
			if task.incRetry() {
				scs.addTask(task)
			} else {
				scs.server.GetLogger().Error("Failed to synchronize shared channel",
					mlog.String("channelId", task.channelID),
					mlog.String("remoteId", task.remoteID),
					mlog.Err(err),
				)
			}
		}
	}
	return shortestWait
}

// removeOldestTask removes and returns the oldest task in the task map.
// A task coming in via NotifyChannelChanged must stay in queue for at least
// `NotifyMinimumDelay` to ensure we don't go nuts trying to sync during a bulk update.
// If no tasks are available then false is returned.
func (scs *Service) removeOldestTask() (syncTask, bool, time.Duration) {
	scs.mux.Lock()
	defer scs.mux.Unlock()

	var oldestTask syncTask
	var oldestKey string
	var shortestWait time.Duration

	for key, task := range scs.tasks {
		// check if task is ready
		if wait := time.Until(task.schedule); wait > 0 {
			if wait < shortestWait || shortestWait == 0 {
				shortestWait = wait
			}
			continue
		}
		// task is ready; check if it's the oldest ready task
		if task.AddedAt.Before(oldestTask.AddedAt) || oldestTask.AddedAt.IsZero() {
			oldestKey = key
			oldestTask = task
		}
	}

	if oldestKey != "" {
		delete(scs.tasks, oldestKey)
		return oldestTask, true, shortestWait
	}
	return oldestTask, false, shortestWait
}

// processTask updates one or more remote clusters with any new channel content.
func (scs *Service) processTask(task syncTask) error {
	var err error
	var remotes []*model.RemoteCluster

	if task.remoteID == "" {
		filter := model.RemoteClusterQueryFilter{
			InChannel:     task.channelID,
			OnlyConfirmed: true,
		}
		remotes, err = scs.server.GetStore().RemoteCluster().GetAll(filter)
		if err != nil {
			return err
		}
	} else {
		rc, err := scs.server.GetStore().RemoteCluster().Get(task.remoteID)
		if err != nil {
			return err
		}
		if !rc.IsOnline() {
			return fmt.Errorf("Failed updating shared channel '%s' for offline remote cluster '%s'", task.channelID, rc.DisplayName)
		}
		remotes = []*model.RemoteCluster{rc}
	}

	for _, rc := range remotes {
		rtask := task
		rtask.remoteID = rc.RemoteID
		if err := scs.syncForRemote(rtask, rc); err != nil {
			// retry...
			if rtask.incRetry() {
				scs.addTask(rtask)
			} else {
				scs.server.GetLogger().Error("Failed to synchronize shared channel for remote cluster",
					mlog.String("channelId", rtask.channelID),
					mlog.String("remote", rc.DisplayName),
					mlog.Err(err),
				)
			}
		}
	}
	return nil
}

func (scs *Service) handlePostError(postID string, task syncTask, rc *model.RemoteCluster) {
	if task.retryMsg != nil && len(task.retryMsg.Posts) == 1 && task.retryMsg.Posts[0].ID == postID {
		// this was a retry for specific post that failed previously. Try again if within MaxRetries.
		if task.incRetry() {
			scs.addTask(task)
		} else {
			scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "error syncing post",
				mlog.String("remote", rc.DisplayName),
				mlog.String("post_id", postID),
			)
		}
		return
	}

	// this post failed as part of a group of posts. Retry as an individual post.
	post, err := scs.server.GetStore().Post().GetSingle(postID, true)
	if err != nil {
		scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "error fetching post for sync retry",
			mlog.String("remote", rc.DisplayName),
			mlog.String("post_id", postID),
		)
		return
	}

	syncMsg := newSyncMsg(task.channelID)
	syncMsg.Posts = []*model.Post{post}

	scs.addTask(newSyncTask(task.channelID, task.remoteID, syncMsg))
}

// notifyRemoteOffline creates an ephemeral post to the author for any posts created recently to remotes
// that are offline.
func (scs *Service) notifyRemoteOffline(posts []*model.Post, rc *model.RemoteCluster) {
	// only send one ephemeral post per author.
	notified := make(map[string]bool)

	// range the slice in reverse so the newest posts are visited first; this ensures an ephemeral
	// get added where it is mostly likely to be seen.
	for i := len(posts) - 1; i >= 0; i-- {
		post := posts[i]
		if didNotify := notified[post.UserID]; didNotify {
			continue
		}

		postCreateAt := model.GetTimeForMillis(post.CreateAt)

		if post.DeleteAt == 0 && post.UserID != "" && time.Since(postCreateAt) < NotifyRemoteOfflineThreshold {
			T := scs.getUserTranslations(post.UserID)
			ephemeral := &model.Post{
				ChannelID: post.ChannelID,
				Message:   T("sharedchannel.cannot_deliver_post", map[string]interface{}{"Remote": rc.DisplayName}),
				CreateAt:  post.CreateAt + 1,
			}
			scs.app.SendEphemeralPost(post.UserID, ephemeral)

			notified[post.UserID] = true
		}
	}
}

func (scs *Service) updateCursorForRemote(scrID string, rc *model.RemoteCluster, cursor model.GetPostsSinceForSyncCursor) {
	if err := scs.server.GetStore().SharedChannel().UpdateRemoteCursor(scrID, cursor); err != nil {
		scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "error updating cursor for shared channel remote",
			mlog.String("remote", rc.DisplayName),
			mlog.Err(err),
		)
		return
	}
	scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceDebug, "updated cursor for remote",
		mlog.String("remote_id", rc.RemoteID),
		mlog.String("remote", rc.DisplayName),
		mlog.Int64("last_post_update_at", cursor.LastPostUpdateAt),
		mlog.String("last_post_id", cursor.LastPostID),
	)
}

func (scs *Service) getUserTranslations(userID string) i18n.TranslateFunc {
	var locale string
	user, err := scs.server.GetStore().User().Get(context.Background(), userID)
	if err == nil {
		locale = user.Locale
	}

	if locale == "" {
		locale = model.DefaultLocale
	}
	return i18n.GetUserTranslations(locale)
}

// shouldUserSync determines if a user needs to be synchronized.
// User should be synchronized if it has no entry in the SharedChannelUsers table for the specified channel,
// or there is an entry but the LastSyncAt is less than user.UpdateAt
func (scs *Service) shouldUserSync(user *model.User, channelID string, rc *model.RemoteCluster) (sync bool, syncImage bool, err error) {
	// don't sync users with the remote they originated from.
	if user.RemoteID != nil && *user.RemoteID == rc.RemoteID {
		return false, false, nil
	}

	scu, err := scs.server.GetStore().SharedChannel().GetSingleUser(user.ID, channelID, rc.RemoteID)
	if err != nil {
		if _, ok := err.(errNotFound); !ok {
			return false, false, err
		}

		// user not in the SharedChannelUsers table, so we must add them.
		scu = &model.SharedChannelUser{
			UserID:    user.ID,
			RemoteID:  rc.RemoteID,
			ChannelID: channelID,
		}
		if _, err = scs.server.GetStore().SharedChannel().SaveUser(scu); err != nil {
			scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Error adding user to shared channel users",
				mlog.String("remote_id", rc.RemoteID),
				mlog.String("user_id", user.ID),
				mlog.String("channel_id", user.ID),
				mlog.Err(err),
			)
		}
		return true, true, nil
	}

	return user.UpdateAt > scu.LastSyncAt, user.LastPictureUpdate > scu.LastSyncAt, nil
}

func (scs *Service) syncProfileImage(user *model.User, channelID string, rc *model.RemoteCluster) {
	rcs := scs.server.GetRemoteClusterService()
	if rcs == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), ProfileImageSyncTimeout)
	defer cancel()

	rcs.SendProfileImage(ctx, user.ID, rc, scs.app, func(userID string, rc *model.RemoteCluster, resp *remotecluster.Response, err error) {
		if resp.IsSuccess() {
			scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceDebug, "Users profile image synchronized",
				mlog.String("remote_id", rc.RemoteID),
				mlog.String("user_id", user.ID),
			)

			if err2 := scs.server.GetStore().SharedChannel().UpdateUserLastSyncAt(user.ID, channelID, rc.RemoteID); err2 != nil {
				scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Error updating users LastSyncTime after profile image update",
					mlog.String("remote_id", rc.RemoteID),
					mlog.String("user_id", user.ID),
					mlog.Err(err2),
				)
			}
			return
		}

		scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Error synchronizing users profile image",
			mlog.String("remote_id", rc.RemoteID),
			mlog.String("user_id", user.ID),
			mlog.Err(err),
		)
	})
}
