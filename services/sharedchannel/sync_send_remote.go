// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sharedchannel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/wiggin77/merror"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/remotecluster"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type sendSyncMsgResultFunc func(syncResp SyncResponse, err error)

type attachment struct {
	fi   *model.FileInfo
	post *model.Post
}

type syncData struct {
	task syncTask
	rc   *model.RemoteCluster
	scr  *model.SharedChannelRemote

	users         map[string]*model.User
	profileImages map[string]*model.User
	posts         []*model.Post
	reactions     []*model.Reaction
	attachments   []attachment

	resultRepeat     bool
	resultNextCursor model.GetPostsSinceForSyncCursor
}

func newSyncData(task syncTask, rc *model.RemoteCluster, scr *model.SharedChannelRemote) *syncData {
	return &syncData{
		task:             task,
		rc:               rc,
		scr:              scr,
		users:            make(map[string]*model.User),
		profileImages:    make(map[string]*model.User),
		resultNextCursor: model.GetPostsSinceForSyncCursor{LastPostUpdateAt: scr.LastPostUpdateAt, LastPostID: scr.LastPostID},
	}
}

func (sd *syncData) isEmpty() bool {
	return len(sd.users) == 0 && len(sd.profileImages) == 0 && len(sd.posts) == 0 && len(sd.reactions) == 0 && len(sd.attachments) == 0
}

func (sd *syncData) isCursorChanged() bool {
	return sd.scr.LastPostUpdateAt != sd.resultNextCursor.LastPostUpdateAt || sd.scr.LastPostID != sd.resultNextCursor.LastPostID
}

// syncForRemote updates a remote cluster with any new posts/reactions for a specific
// channel. If many changes are found, only the oldest X changes are sent and the channel
// is re-added to the task map. This ensures no channels are starved for updates even if some
// channels are very active.
// Returning an error forces a retry on the task.
func (scs *Service) syncForRemote(task syncTask, rc *model.RemoteCluster) error {
	rcs := scs.server.GetRemoteClusterService()
	if rcs == nil {
		return fmt.Errorf("cannot update remote cluster %s for channel id %s; Remote Cluster Service not enabled", rc.Name, task.channelID)
	}

	scr, err := scs.server.GetStore().SharedChannel().GetRemoteByIDs(task.channelID, rc.RemoteID)
	if err != nil {
		return err
	}

	// if this is retrying a failed msg, just send it again.
	if task.retryMsg != nil {
		sd := newSyncData(task, rc, scr)
		sd.users = task.retryMsg.Users
		sd.posts = task.retryMsg.Posts
		sd.reactions = task.retryMsg.Reactions
		return scs.sendSyncData(sd)
	}

	sd := newSyncData(task, rc, scr)

	// schedule another sync if the repeat flag is set at some point.
	defer func(rpt *bool) {
		if *rpt {
			scs.addTask(newSyncTask(task.channelID, task.remoteID, nil))
		}
	}(&sd.resultRepeat)

	// fetch new posts or retry post.
	if err := scs.fetchPostsForSync(sd); err != nil {
		return fmt.Errorf("cannot fetch posts for sync %v: %w", sd, err)
	}

	if !rc.IsOnline() {
		if len(sd.posts) != 0 {
			scs.notifyRemoteOffline(sd.posts, rc)
		}
		sd.resultRepeat = false
		return nil
	}

	// fetch users that have updated their user profile or image.
	if err := scs.fetchUsersForSync(sd); err != nil {
		return fmt.Errorf("cannot fetch users for sync %v: %w", sd, err)
	}

	// fetch reactions for posts
	if err := scs.fetchReactionsForSync(sd); err != nil {
		return fmt.Errorf("cannot fetch reactions for sync %v: %w", sd, err)
	}

	// fetch users associated with posts & reactions
	if err := scs.fetchPostUsersForSync(sd); err != nil {
		return fmt.Errorf("cannot fetch post users for sync %v: %w", sd, err)
	}

	// filter out any posts that don't need to be sent.
	scs.filterPostsForSync(sd)

	// fetch attachments for posts
	if err := scs.fetchPostAttachmentsForSync(sd); err != nil {
		return fmt.Errorf("cannot fetch post attachments for sync %v: %w", sd, err)
	}

	if sd.isEmpty() {
		scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceDebug, "Not sending sync data; everything filtered out",
			mlog.String("remote", rc.DisplayName),
			mlog.String("channel_id", task.channelID),
			mlog.Bool("repeat", sd.resultRepeat),
		)
		if sd.isCursorChanged() {
			scs.updateCursorForRemote(sd.scr.ID, sd.rc, sd.resultNextCursor)
		}
		return nil
	}

	scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceDebug, "Sending sync data",
		mlog.String("remote", rc.DisplayName),
		mlog.String("channel_id", task.channelID),
		mlog.Bool("repeat", sd.resultRepeat),
		mlog.Int("users", len(sd.users)),
		mlog.Int("images", len(sd.profileImages)),
		mlog.Int("posts", len(sd.posts)),
		mlog.Int("reactions", len(sd.reactions)),
		mlog.Int("attachments", len(sd.attachments)),
	)

	return scs.sendSyncData(sd)
}

// fetchUsersForSync populates the sync data with any channel users who updated their user profile
// since the last sync.
func (scs *Service) fetchUsersForSync(sd *syncData) error {
	filter := model.GetUsersForSyncFilter{
		ChannelID: sd.task.channelID,
		Limit:     MaxUsersPerSync,
	}
	users, err := scs.server.GetStore().SharedChannel().GetUsersForSync(filter)
	if err != nil {
		return err
	}

	for _, u := range users {
		if u.GetRemoteID() != sd.rc.RemoteID {
			sd.users[u.ID] = u
		}
	}

	filter.CheckProfileImage = true
	usersImage, err := scs.server.GetStore().SharedChannel().GetUsersForSync(filter)
	if err != nil {
		return err
	}

	for _, u := range usersImage {
		if u.GetRemoteID() != sd.rc.RemoteID {
			sd.profileImages[u.ID] = u
		}
	}
	return nil
}

// fetchPostsForSync populates the sync data with any new posts since the last sync.
func (scs *Service) fetchPostsForSync(sd *syncData) error {
	options := model.GetPostsSinceForSyncOptions{
		ChannelID:      sd.task.channelID,
		IncludeDeleted: true,
	}
	cursor := model.GetPostsSinceForSyncCursor{
		LastPostUpdateAt: sd.scr.LastPostUpdateAt,
		LastPostID:       sd.scr.LastPostID,
	}

	posts, nextCursor, err := scs.server.GetStore().Post().GetPostsSinceForSync(options, cursor, MaxPostsPerSync)
	if err != nil {
		return fmt.Errorf("could not fetch new posts for sync: %w", err)
	}

	// Append the posts individually, checking for root posts that might appear later in the list.
	// This is due to the UpdateAt collision handling algorithm where the order of posts is not based
	// on UpdateAt or CreateAt when the posts have the same UpdateAt value. Here we are guarding
	// against a root post with the same UpdateAt (and probably the same CreateAt) appearing later
	// in the list and must be sync'd before the child post. This is and edge case that likely only
	// happens during load testing or bulk imports.
	for _, p := range posts {
		if p.RootID != "" {
			root, err := scs.server.GetStore().Post().GetSingle(p.RootID, true)
			if err == nil {
				if (root.CreateAt >= cursor.LastPostUpdateAt || root.UpdateAt >= cursor.LastPostUpdateAt) && !containsPost(sd.posts, root) {
					sd.posts = append(sd.posts, root)
				}
			}
		}
		sd.posts = append(sd.posts, p)
	}

	sd.resultNextCursor = nextCursor
	sd.resultRepeat = len(posts) == MaxPostsPerSync
	return nil
}

func containsPost(posts []*model.Post, post *model.Post) bool {
	for _, p := range posts {
		if p.ID == post.ID {
			return true
		}
	}
	return false
}

// fetchReactionsForSync populates the sync data with any new reactions since the last sync.
func (scs *Service) fetchReactionsForSync(sd *syncData) error {
	merr := merror.New()
	for _, post := range sd.posts {
		// any reactions originating from the remote cluster are filtered out
		reactions, err := scs.server.GetStore().Reaction().GetForPostSince(post.ID, sd.scr.LastPostUpdateAt, sd.rc.RemoteID, true)
		if err != nil {
			merr.Append(fmt.Errorf("could not get reactions for post %s: %w", post.ID, err))
			continue
		}
		sd.reactions = append(sd.reactions, reactions...)
	}
	return merr.ErrorOrNil()
}

// fetchPostUsersForSync populates the sync data with all users associated with posts.
func (scs *Service) fetchPostUsersForSync(sd *syncData) error {
	sc, err := scs.server.GetStore().SharedChannel().Get(sd.task.channelID)
	if err != nil {
		return fmt.Errorf("cannot determine teamID: %w", err)
	}

	type p2mm struct {
		post       *model.Post
		mentionMap model.UserMentionMap
	}

	userIDs := make(map[string]p2mm)

	for _, reaction := range sd.reactions {
		userIDs[reaction.UserID] = p2mm{}
	}

	for _, post := range sd.posts {
		// add author
		userIDs[post.UserID] = p2mm{}

		// get mentions and users for each mention
		mentionMap := scs.app.MentionsToTeamMembers(post.Message, sc.TeamID)
		for _, userID := range mentionMap {
			userIDs[userID] = p2mm{
				post:       post,
				mentionMap: mentionMap,
			}
		}
	}

	merr := merror.New()

	for userID, v := range userIDs {
		user, err := scs.server.GetStore().User().Get(context.Background(), userID)
		if err != nil {
			merr.Append(fmt.Errorf("could not get user %s: %w", userID, err))
			continue
		}

		sync, syncImage, err2 := scs.shouldUserSync(user, sd.task.channelID, sd.rc)
		if err2 != nil {
			merr.Append(fmt.Errorf("could not check should sync user %s: %w", userID, err))
			continue
		}

		if sync {
			sd.users[user.ID] = sanitizeUserForSync(user)
		}

		if syncImage {
			sd.profileImages[user.ID] = sanitizeUserForSync(user)
		}

		// if this was a mention then put the real username in place of the username+remotename, but only
		// when sending to the remote that the user belongs to.
		if v.post != nil && user.RemoteID != nil && *user.RemoteID == sd.rc.RemoteID {
			fixMention(v.post, v.mentionMap, user)
		}
	}
	return merr.ErrorOrNil()
}

// fetchPostAttachmentsForSync populates the sync data with any file attachments for new posts.
func (scs *Service) fetchPostAttachmentsForSync(sd *syncData) error {
	merr := merror.New()
	for _, post := range sd.posts {
		fis, err := scs.server.GetStore().FileInfo().GetForPost(post.ID, false, true, true)
		if err != nil {
			merr.Append(fmt.Errorf("could not get file attachment info for post %s: %w", post.ID, err))
			continue
		}

		for _, fi := range fis {
			if scs.shouldSyncAttachment(fi, sd.rc) {
				sd.attachments = append(sd.attachments, attachment{fi: fi, post: post})
			}
		}
	}
	return merr.ErrorOrNil()
}

// filterPostsforSync removes any posts that do not need to sync.
func (scs *Service) filterPostsForSync(sd *syncData) {
	filtered := make([]*model.Post, 0, len(sd.posts))

	for _, p := range sd.posts {
		// Don't resend an existing post where only the reactions changed.
		// Posts we must send:
		//   - new posts (EditAt == 0)
		//   - edited posts (EditAt >= LastPostUpdateAt)
		//   - deleted posts (DeleteAt > 0)
		if p.EditAt > 0 && p.EditAt < sd.scr.LastPostUpdateAt && p.DeleteAt == 0 {
			continue
		}

		// Don't send a deleted post if it is just the original copy from an edit.
		if p.DeleteAt > 0 && p.OriginalID != "" {
			continue
		}

		// don't sync a post back to the remote it came from.
		if p.GetRemoteID() == sd.rc.RemoteID {
			continue
		}

		// parse out all permalinks in the message.
		p.Message = scs.processPermalinkToRemote(p)

		filtered = append(filtered, p)
	}
	sd.posts = filtered
}

// sendSyncData sends all the collected users, posts, reactions, images, and attachments to the
// remote cluster.
// The order of items sent is important: users -> attachments -> posts -> reactions -> profile images
func (scs *Service) sendSyncData(sd *syncData) error {
	merr := merror.New()

	// send users
	if len(sd.users) != 0 {
		if err := scs.sendUserSyncData(sd); err != nil {
			merr.Append(fmt.Errorf("cannot send user sync data: %w", err))
		}
	}

	// send attachments
	if len(sd.attachments) != 0 {
		scs.sendAttachmentSyncData(sd)
	}

	// send posts
	if len(sd.posts) != 0 {
		if err := scs.sendPostSyncData(sd); err != nil {
			merr.Append(fmt.Errorf("cannot send post sync data: %w", err))
		}
	} else if sd.isCursorChanged() {
		scs.updateCursorForRemote(sd.scr.ID, sd.rc, sd.resultNextCursor)
	}

	// send reactions
	if len(sd.reactions) != 0 {
		if err := scs.sendReactionSyncData(sd); err != nil {
			merr.Append(fmt.Errorf("cannot send reaction sync data: %w", err))
		}
	}

	// send user profile images
	if len(sd.profileImages) != 0 {
		scs.sendProfileImageSyncData(sd)
	}

	return merr.ErrorOrNil()
}

// sendUserSyncData sends the collected user updates to the remote cluster.
func (scs *Service) sendUserSyncData(sd *syncData) error {
	msg := newSyncMsg(sd.task.channelID)
	msg.Users = sd.users

	err := scs.sendSyncMsgToRemote(msg, sd.rc, func(syncResp SyncResponse, errResp error) {
		for _, userID := range syncResp.UsersSyncd {
			if err := scs.server.GetStore().SharedChannel().UpdateUserLastSyncAt(userID, sd.task.channelID, sd.rc.RemoteID); err != nil {
				scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Cannot update shared channel user LastSyncAt",
					mlog.String("user_id", userID),
					mlog.String("channel_id", sd.task.channelID),
					mlog.String("remote_id", sd.rc.RemoteID),
					mlog.Err(err),
				)
			}
		}
		if len(syncResp.UserErrors) != 0 {
			scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Response indicates error for user(s) sync",
				mlog.String("channel_id", sd.task.channelID),
				mlog.String("remote_id", sd.rc.RemoteID),
				mlog.Any("users", syncResp.UserErrors),
			)
		}
	})
	return err
}

// sendAttachmentSyncData sends the collected post updates to the remote cluster.
func (scs *Service) sendAttachmentSyncData(sd *syncData) {
	for _, a := range sd.attachments {
		if err := scs.sendAttachmentForRemote(a.fi, a.post, sd.rc); err != nil {
			scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Cannot sync post attachment",
				mlog.String("post_id", a.post.ID),
				mlog.String("channel_id", sd.task.channelID),
				mlog.String("remote_id", sd.rc.RemoteID),
				mlog.Err(err),
			)
		}
		// updating SharedChannelAttachments with LastSyncAt is already done.
	}
}

// sendPostSyncData sends the collected post updates to the remote cluster.
func (scs *Service) sendPostSyncData(sd *syncData) error {
	msg := newSyncMsg(sd.task.channelID)
	msg.Posts = sd.posts

	return scs.sendSyncMsgToRemote(msg, sd.rc, func(syncResp SyncResponse, errResp error) {
		if len(syncResp.PostErrors) != 0 {
			scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Response indicates error for post(s) sync",
				mlog.String("channel_id", sd.task.channelID),
				mlog.String("remote_id", sd.rc.RemoteID),
				mlog.Any("posts", syncResp.PostErrors),
			)

			for _, postID := range syncResp.PostErrors {
				scs.handlePostError(postID, sd.task, sd.rc)
			}
		}
		scs.updateCursorForRemote(sd.scr.ID, sd.rc, sd.resultNextCursor)
	})
}

// sendReactionSyncData sends the collected reaction updates to the remote cluster.
func (scs *Service) sendReactionSyncData(sd *syncData) error {
	msg := newSyncMsg(sd.task.channelID)
	msg.Reactions = sd.reactions

	return scs.sendSyncMsgToRemote(msg, sd.rc, func(syncResp SyncResponse, errResp error) {
		if len(syncResp.ReactionErrors) != 0 {
			scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Response indicates error for reactions(s) sync",
				mlog.String("channel_id", sd.task.channelID),
				mlog.String("remote_id", sd.rc.RemoteID),
				mlog.Any("reaction_posts", syncResp.ReactionErrors),
			)
		}
	})
}

// sendProfileImageSyncData sends the collected user profile image updates to the remote cluster.
func (scs *Service) sendProfileImageSyncData(sd *syncData) {
	for _, user := range sd.profileImages {
		scs.syncProfileImage(user, sd.task.channelID, sd.rc)
	}
}

// sendSyncMsgToRemote synchronously sends the sync message to the remote cluster.
func (scs *Service) sendSyncMsgToRemote(msg *syncMsg, rc *model.RemoteCluster, f sendSyncMsgResultFunc) error {
	rcs := scs.server.GetRemoteClusterService()
	if rcs == nil {
		return fmt.Errorf("cannot update remote cluster %s for channel id %s; Remote Cluster Service not enabled", rc.Name, msg.ChannelID)
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	rcMsg := model.NewRemoteClusterMsg(TopicSync, b)

	ctx, cancel := context.WithTimeout(context.Background(), remotecluster.SendTimeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	err = rcs.SendMsg(ctx, rcMsg, rc, func(rcMsg model.RemoteClusterMsg, rc *model.RemoteCluster, rcResp *remotecluster.Response, errResp error) {
		defer wg.Done()

		var syncResp SyncResponse
		if err2 := json.Unmarshal(rcResp.Payload, &syncResp); err2 != nil {
			scs.server.GetLogger().Log(mlog.LvlSharedChannelServiceError, "Invalid sync msg response from remote cluster",
				mlog.String("remote", rc.Name),
				mlog.String("channel_id", msg.ChannelID),
				mlog.Err(err2),
			)
			return
		}

		if f != nil {
			f(syncResp, errResp)
		}
	})

	wg.Wait()
	return err
}
