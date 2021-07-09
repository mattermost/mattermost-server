// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/gorp"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type SQLThreadStore struct {
	*SQLStore
}

func (s *SQLThreadStore) ClearCaches() {
}

func newSQLThreadStore(sqlStore *SQLStore) store.ThreadStore {
	s := &SQLThreadStore{
		SQLStore: sqlStore,
	}

	for _, db := range sqlStore.GetAllConns() {
		tableThreads := db.AddTableWithName(model.Thread{}, "Threads").SetKeys(false, "PostId")
		tableThreads.ColMap("PostId").SetMaxSize(26)
		tableThreads.ColMap("ChannelId").SetMaxSize(26)
		tableThreads.ColMap("Participants").SetMaxSize(0)
		tableThreadMemberships := db.AddTableWithName(model.ThreadMembership{}, "ThreadMemberships").SetKeys(false, "PostId", "UserId")
		tableThreadMemberships.ColMap("PostId").SetMaxSize(26)
		tableThreadMemberships.ColMap("UserId").SetMaxSize(26)
	}

	return s
}

func threadSliceColumns() []string {
	return []string{"PostId", "ChannelId", "LastReplyAt", "ReplyCount", "Participants"}
}

func threadToSlice(thread *model.Thread) []interface{} {
	return []interface{}{
		thread.PostID,
		thread.ChannelID,
		thread.LastReplyAt,
		thread.ReplyCount,
		thread.Participants,
	}
}

func (s *SQLThreadStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_thread_memberships_last_update_at", "ThreadMemberships", "LastUpdated")
	s.CreateIndexIfNotExists("idx_thread_memberships_last_view_at", "ThreadMemberships", "LastViewed")
	s.CreateIndexIfNotExists("idx_thread_memberships_user_id", "ThreadMemberships", "UserId")
	s.CreateIndexIfNotExists("idx_threads_channel_id", "Threads", "ChannelId")
}

func (s *SQLThreadStore) SaveMultiple(threads []*model.Thread) ([]*model.Thread, int, error) {
	builder := s.getQueryBuilder().Insert("Threads").Columns(threadSliceColumns()...)
	for _, thread := range threads {
		builder = builder.Values(threadToSlice(thread)...)
	}
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, -1, errors.Wrap(err, "thread_tosql")
	}

	if _, err := s.GetMaster().Exec(query, args...); err != nil {
		return nil, -1, errors.Wrap(err, "failed to save Post")
	}

	return threads, -1, nil
}

func (s *SQLThreadStore) Save(thread *model.Thread) (*model.Thread, error) {
	threads, _, err := s.SaveMultiple([]*model.Thread{thread})
	if err != nil {
		return nil, err
	}
	return threads[0], nil
}

func (s *SQLThreadStore) Update(thread *model.Thread) (*model.Thread, error) {
	return s.update(s.GetMaster(), thread)
}

func (s *SQLThreadStore) update(ex gorp.SqlExecutor, thread *model.Thread) (*model.Thread, error) {
	if _, err := ex.Update(thread); err != nil {
		return nil, errors.Wrapf(err, "failed to update thread with id=%s", thread.PostID)
	}

	return thread, nil
}

func (s *SQLThreadStore) Get(id string) (*model.Thread, error) {
	return s.get(s.GetReplica(), id)
}

func (s *SQLThreadStore) get(ex gorp.SqlExecutor, id string) (*model.Thread, error) {
	var thread model.Thread
	query, args, _ := s.getQueryBuilder().Select("*").From("Threads").Where(sq.Eq{"PostId": id}).ToSql()
	err := ex.SelectOne(&thread, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, errors.Wrapf(err, "failed to get thread with id=%s", id)
	}
	return &thread, nil
}

func (s *SQLThreadStore) GetThreadsForUser(userID, teamID string, opts model.GetUserThreadsOpts) (*model.Threads, error) {
	type JoinedThread struct {
		PostID         string
		ReplyCount     int64
		LastReplyAt    int64
		LastViewedAt   int64
		UnreadReplies  int64
		UnreadMentions int64
		Participants   model.StringArray
		model.Post
	}

	unreadRepliesQuery := "SELECT COUNT(Posts.Id) From Posts Where Posts.RootId=ThreadMemberships.PostId AND Posts.CreateAt >= ThreadMemberships.LastViewed"
	fetchConditions := sq.And{
		sq.Or{sq.Eq{"Channels.TeamId": teamID}, sq.Eq{"Channels.TeamId": ""}},
		sq.Eq{"ThreadMemberships.UserId": userID},
		sq.Eq{"ThreadMemberships.Following": true},
	}
	if !opts.Deleted {
		fetchConditions = sq.And{
			fetchConditions,
			sq.Eq{"COALESCE(Posts.DeleteAt, 0)": 0},
		}
	}

	pageSize := uint64(30)
	if opts.PageSize != 0 {
		pageSize = opts.PageSize
	}

	totalUnreadThreadsChan := make(chan store.StoreResult, 1)
	totalCountChan := make(chan store.StoreResult, 1)
	totalUnreadMentionsChan := make(chan store.StoreResult, 1)
	threadsChan := make(chan store.StoreResult, 1)
	go func() {
		repliesQuery, repliesQueryArgs, _ := s.getQueryBuilder().
			Select("COUNT(DISTINCT(Posts.RootId))").
			From("Posts").
			LeftJoin("ThreadMemberships ON Posts.RootId = ThreadMemberships.PostId").
			LeftJoin("Channels ON Posts.ChannelId = Channels.Id").
			Where(fetchConditions).
			Where("Posts.CreateAt >= ThreadMemberships.LastViewed").ToSql()

		totalUnreadThreads, err := s.GetMaster().SelectInt(repliesQuery, repliesQueryArgs...)
		totalUnreadThreadsChan <- store.StoreResult{Data: totalUnreadThreads, NErr: errors.Wrapf(err, "failed to get count unread on threads for user id=%s", userID)}
		close(totalUnreadThreadsChan)
	}()
	go func() {
		newFetchConditions := fetchConditions

		if opts.Unread {
			newFetchConditions = sq.And{newFetchConditions, sq.Expr("ThreadMemberships.LastViewed < Threads.LastReplyAt")}
		}

		threadsQuery, threadsQueryArgs, _ := s.getQueryBuilder().
			Select("COUNT(ThreadMemberships.PostId)").
			LeftJoin("Threads ON Threads.PostId = ThreadMemberships.PostId").
			LeftJoin("Channels ON Threads.ChannelId = Channels.Id").
			LeftJoin("Posts ON Posts.Id = ThreadMemberships.PostId").
			From("ThreadMemberships").
			Where(newFetchConditions).ToSql()

		totalCount, err := s.GetMaster().SelectInt(threadsQuery, threadsQueryArgs...)
		totalCountChan <- store.StoreResult{Data: totalCount, NErr: err}
		close(totalCountChan)
	}()
	go func() {
		mentionsQuery, mentionsQueryArgs, _ := s.getQueryBuilder().
			Select("COALESCE(SUM(ThreadMemberships.UnreadMentions),0)").
			From("ThreadMemberships").
			LeftJoin("Threads ON Threads.PostId = ThreadMemberships.PostId").
			LeftJoin("Posts ON Posts.Id = ThreadMemberships.PostId").
			LeftJoin("Channels ON Threads.ChannelId = Channels.Id").
			Where(fetchConditions).ToSql()
		totalUnreadMentions, err := s.GetMaster().SelectInt(mentionsQuery, mentionsQueryArgs...)
		totalUnreadMentionsChan <- store.StoreResult{Data: totalUnreadMentions, NErr: err}
		close(totalUnreadMentionsChan)
	}()
	go func() {
		newFetchConditions := fetchConditions
		if opts.Since > 0 {
			newFetchConditions = sq.And{newFetchConditions, sq.GtOrEq{"ThreadMemberships.LastUpdated": opts.Since}}
		}
		order := "DESC"
		if opts.Before != "" {
			newFetchConditions = sq.And{
				newFetchConditions,
				sq.Expr(`LastReplyAt < (SELECT LastReplyAt FROM Threads WHERE PostId = ?)`, opts.Before),
			}
		}
		if opts.After != "" {
			order = "ASC"
			newFetchConditions = sq.And{
				newFetchConditions,
				sq.Expr(`LastReplyAt > (SELECT LastReplyAt FROM Threads WHERE PostId = ?)`, opts.After),
			}
		}
		if opts.Unread {
			newFetchConditions = sq.And{newFetchConditions, sq.Expr("ThreadMemberships.LastViewed < Threads.LastReplyAt")}
		}
		var threads []*JoinedThread
		query, args, _ := s.getQueryBuilder().
			Select(`Threads.*,
				` + postSliceCoalesceQuery() + `,
				ThreadMemberships.LastViewed as LastViewedAt,
				ThreadMemberships.UnreadMentions as UnreadMentions`).
			From("Threads").
			Column(sq.Alias(sq.Expr(unreadRepliesQuery), "UnreadReplies")).
			LeftJoin("Posts ON Posts.Id = Threads.PostId").
			LeftJoin("Channels ON Posts.ChannelId = Channels.Id").
			LeftJoin("ThreadMemberships ON ThreadMemberships.PostId = Threads.PostId").
			Where(newFetchConditions).
			OrderBy("Threads.LastReplyAt " + order).
			Limit(pageSize).ToSql()

		_, err := s.GetReplica().Select(&threads, query, args...)
		threadsChan <- store.StoreResult{Data: threads, NErr: err}
		close(threadsChan)
	}()

	threadsResult := <-threadsChan
	if threadsResult.NErr != nil {
		return nil, threadsResult.NErr
	}
	threads := threadsResult.Data.([]*JoinedThread)

	totalUnreadMentionsResult := <-totalUnreadMentionsChan
	if totalUnreadMentionsResult.NErr != nil {
		return nil, totalUnreadMentionsResult.NErr
	}
	totalUnreadMentions := totalUnreadMentionsResult.Data.(int64)

	totalCountResult := <-totalCountChan
	if totalCountResult.NErr != nil {
		return nil, totalCountResult.NErr
	}
	totalCount := totalCountResult.Data.(int64)

	totalUnreadThreadsResult := <-totalUnreadThreadsChan
	if totalUnreadThreadsResult.NErr != nil {
		return nil, totalUnreadThreadsResult.NErr
	}
	totalUnreadThreads := totalUnreadThreadsResult.Data.(int64)

	var userIDs []string
	userIDMap := map[string]bool{}
	for _, thread := range threads {
		for _, participantID := range thread.Participants {
			if _, ok := userIDMap[participantID]; !ok {
				userIDMap[participantID] = true
				userIDs = append(userIDs, participantID)
			}
		}
	}
	var users []*model.User
	if opts.Extended {
		var err error
		users, err = s.User().GetProfileByIDs(context.Background(), userIDs, &store.UserGetByIDsOpts{}, true)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get threads for user id=%s", userID)
		}
	} else {
		for _, userID := range userIDs {
			users = append(users, &model.User{ID: userID})
		}
	}

	result := &model.Threads{
		Total:               totalCount,
		Threads:             []*model.ThreadResponse{},
		TotalUnreadMentions: totalUnreadMentions,
		TotalUnreadThreads:  totalUnreadThreads,
	}

	for _, thread := range threads {
		var participants []*model.User
		for _, participantID := range thread.Participants {
			var participant *model.User
			for _, u := range users {
				if u.ID == participantID {
					participant = u
					break
				}
			}
			if participant == nil {
				return nil, errors.New("cannot find thread participant with id=" + participantID)
			}
			participants = append(participants, participant)
		}
		result.Threads = append(result.Threads, &model.ThreadResponse{
			PostID:         thread.PostID,
			ReplyCount:     thread.ReplyCount,
			LastReplyAt:    thread.LastReplyAt,
			LastViewedAt:   thread.LastViewedAt,
			UnreadReplies:  thread.UnreadReplies,
			UnreadMentions: thread.UnreadMentions,
			Participants:   participants,
			Post:           thread.Post.ToNilIfInvalid(),
		})
	}

	return result, nil
}

func (s *SQLThreadStore) GetThreadFollowers(threadID string) ([]string, error) {
	var users []string
	query, args, _ := s.getQueryBuilder().
		Select("ThreadMemberships.UserId").
		From("ThreadMemberships").
		Where(sq.Eq{"PostId": threadID}).ToSql()
	_, err := s.GetReplica().Select(&users, query, args...)

	if err != nil {
		return nil, err
	}
	return users, nil
}

func (s *SQLThreadStore) GetThreadForUser(teamID string, threadMembership *model.ThreadMembership, extended bool) (*model.ThreadResponse, error) {
	if !threadMembership.Following {
		return nil, nil // in case the thread is not followed anymore - return nil error to be interpreted as 404
	}

	type JoinedThread struct {
		PostID         string
		Following      bool
		ReplyCount     int64
		LastReplyAt    int64
		LastViewedAt   int64
		UnreadReplies  int64
		UnreadMentions int64
		Participants   model.StringArray
		model.Post
	}

	unreadRepliesQuery, unreadRepliesArgs := sq.
		Select("COUNT(Posts.Id)").
		From("Posts").
		Where(sq.And{
			sq.Eq{"Posts.RootId": threadMembership.PostID},
			sq.GtOrEq{"Posts.CreateAt": threadMembership.LastViewed},
			sq.Eq{"Posts.DeleteAt": 0},
		}).MustSql()

	fetchConditions := sq.And{
		sq.Or{sq.Eq{"Channels.TeamId": teamID}, sq.Eq{"Channels.TeamId": ""}},
		sq.Eq{"Threads.PostId": threadMembership.PostID},
	}

	var thread JoinedThread
	query, threadArgs, _ := s.getQueryBuilder().
		Select("Threads.*, Posts.*").
		From("Threads").
		Column(sq.Alias(sq.Expr(unreadRepliesQuery), "UnreadReplies")).
		LeftJoin("Posts ON Posts.Id = Threads.PostId").
		LeftJoin("Channels ON Posts.ChannelId = Channels.Id").
		Where(fetchConditions).ToSql()

	args := append(unreadRepliesArgs, threadArgs...)

	err := s.GetReplica().SelectOne(&thread, query, args...)
	if err != nil {
		return nil, err
	}

	thread.LastViewedAt = threadMembership.LastViewed
	thread.UnreadMentions = threadMembership.UnreadMentions

	var users []*model.User
	if extended {
		var err error
		users, err = s.User().GetProfileByIDs(context.Background(), thread.Participants, &store.UserGetByIDsOpts{}, true)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get thread for user id=%s", threadMembership.UserID)
		}
	} else {
		for _, userID := range thread.Participants {
			users = append(users, &model.User{ID: userID})
		}
	}

	result := &model.ThreadResponse{
		PostID:         thread.PostID,
		ReplyCount:     thread.ReplyCount,
		LastReplyAt:    thread.LastReplyAt,
		LastViewedAt:   thread.LastViewedAt,
		UnreadReplies:  thread.UnreadReplies,
		UnreadMentions: thread.UnreadMentions,
		Participants:   users,
		Post:           thread.Post.ToNilIfInvalid(),
	}

	return result, nil
}
func (s *SQLThreadStore) MarkAllAsReadInChannels(userID string, channelIDs []string) error {
	var threadIDs []string

	query, args, _ := s.getQueryBuilder().
		Select("ThreadMemberships.PostId").
		Join("Threads ON Threads.PostId = ThreadMemberships.PostId").
		Join("Channels ON Threads.ChannelId = Channels.Id").
		From("ThreadMemberships").
		Where(sq.Eq{"Threads.ChannelId": channelIDs}).
		Where(sq.Eq{"ThreadMemberships.UserId": userID}).
		ToSql()

	_, err := s.GetReplica().Select(&threadIDs, query, args...)
	if err != nil {
		return errors.Wrapf(err, "failed to get thread membership with userid=%s", userID)
	}

	timestamp := model.GetMillis()
	query, args, _ = s.getQueryBuilder().
		Update("ThreadMemberships").
		Where(sq.Eq{"PostId": threadIDs}).
		Where(sq.Eq{"UserId": userID}).
		Set("LastViewed", timestamp).
		Set("UnreadMentions", 0).
		ToSql()
	if _, err := s.GetMaster().Exec(query, args...); err != nil {
		return errors.Wrapf(err, "failed to update thread read state for user id=%s", userID)
	}
	return nil

}
func (s *SQLThreadStore) MarkAllAsRead(userID, teamID string) error {
	memberships, err := s.GetMembershipsForUser(userID, teamID)
	if err != nil {
		return err
	}
	var membershipIDs []string
	for _, m := range memberships {
		membershipIDs = append(membershipIDs, m.PostID)
	}
	timestamp := model.GetMillis()
	query, args, _ := s.getQueryBuilder().
		Update("ThreadMemberships").
		Where(sq.Eq{"PostId": membershipIDs}).
		Where(sq.Eq{"UserId": userID}).
		Set("LastViewed", timestamp).
		Set("UnreadMentions", 0).
		ToSql()
	if _, err := s.GetMaster().Exec(query, args...); err != nil {
		return errors.Wrapf(err, "failed to update thread read state for user id=%s", userID)
	}
	return nil
}

func (s *SQLThreadStore) MarkAsRead(userID, threadID string, timestamp int64) error {
	query, args, _ := s.getQueryBuilder().
		Update("ThreadMemberships").
		Where(sq.Eq{"UserId": userID}).
		Where(sq.Eq{"PostId": threadID}).
		Set("LastViewed", timestamp).
		ToSql()
	if _, err := s.GetMaster().Exec(query, args...); err != nil {
		return errors.Wrapf(err, "failed to update thread read state for user id=%s thread_id=%v", userID, threadID)
	}
	return nil
}

func (s *SQLThreadStore) Delete(threadID string) error {
	query, args, _ := s.getQueryBuilder().Delete("Threads").Where(sq.Eq{"PostId": threadID}).ToSql()
	if _, err := s.GetMaster().Exec(query, args...); err != nil {
		return errors.Wrap(err, "failed to update threads")
	}

	return nil
}

func (s *SQLThreadStore) SaveMembership(membership *model.ThreadMembership) (*model.ThreadMembership, error) {
	return s.saveMembership(s.GetMaster(), membership)
}

func (s *SQLThreadStore) saveMembership(ex gorp.SqlExecutor, membership *model.ThreadMembership) (*model.ThreadMembership, error) {
	if err := ex.Insert(membership); err != nil {
		return nil, errors.Wrapf(err, "failed to save thread membership with postid=%s userid=%s", membership.PostID, membership.UserID)
	}

	return membership, nil
}

func (s *SQLThreadStore) UpdateMembership(membership *model.ThreadMembership) (*model.ThreadMembership, error) {
	return s.updateMembership(s.GetMaster(), membership)
}

func (s *SQLThreadStore) updateMembership(ex gorp.SqlExecutor, membership *model.ThreadMembership) (*model.ThreadMembership, error) {
	if _, err := ex.Update(membership); err != nil {
		return nil, errors.Wrapf(err, "failed to update thread membership with postid=%s userid=%s", membership.PostID, membership.UserID)
	}

	return membership, nil
}

func (s *SQLThreadStore) GetMembershipsForUser(userID, teamID string) ([]*model.ThreadMembership, error) {
	var memberships []*model.ThreadMembership

	query, args, _ := s.getQueryBuilder().
		Select("ThreadMemberships.*").
		Join("Threads ON Threads.PostId = ThreadMemberships.PostId").
		Join("Channels ON Threads.ChannelId = Channels.Id").
		From("ThreadMemberships").
		Where(sq.Or{sq.Eq{"Channels.TeamId": teamID}, sq.Eq{"Channels.TeamId": ""}}).
		Where(sq.Eq{"ThreadMemberships.UserId": userID}).
		ToSql()

	_, err := s.GetReplica().Select(&memberships, query, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get thread membership with userid=%s", userID)
	}
	return memberships, nil
}

func (s *SQLThreadStore) GetMembershipForUser(userID, postID string) (*model.ThreadMembership, error) {
	return s.getMembershipForUser(s.GetReplica(), userID, postID)
}

func (s *SQLThreadStore) getMembershipForUser(ex gorp.SqlExecutor, userID, postID string) (*model.ThreadMembership, error) {
	var membership model.ThreadMembership
	err := ex.SelectOne(&membership, "SELECT * from ThreadMemberships WHERE UserId = :UserId AND PostId = :PostId", map[string]interface{}{"UserId": userID, "PostId": postID})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Thread", postID)
		}
		return nil, errors.Wrapf(err, "failed to get thread membership with userid=%s postid=%s", userID, postID)
	}
	return &membership, nil
}

func (s *SQLThreadStore) DeleteMembershipForUser(userID string, postID string) error {
	if _, err := s.GetMaster().Exec("DELETE FROM ThreadMemberships Where PostId = :PostId AND UserId = :UserId", map[string]interface{}{"PostId": postID, "UserId": userID}); err != nil {
		return errors.Wrap(err, "failed to update thread membership")
	}

	return nil
}

// MaintainMembership creates or updates a thread membership for the given user
// and post. This method is used to update the state of a membership in response
// to some events like:
// - post creation (mentions handling)
// - channel marked unread
// - user explicitly following a thread
func (s *SQLThreadStore) MaintainMembership(userID, postID string, opts store.ThreadMembershipOpts) (*model.ThreadMembership, error) {
	trx, err := s.GetMaster().Begin()
	if err != nil {
		return nil, errors.Wrap(err, "begin_transaction")
	}
	defer finalizeTransaction(trx)

	membership, err := s.getMembershipForUser(trx, userID, postID)
	now := utils.MillisFromTime(time.Now())
	// if memebership exists, update it if:
	// a. user started/stopped following a thread
	// b. mention count changed
	// c. user viewed a thread
	if err == nil {
		followingNeedsUpdate := (opts.UpdateFollowing && !membership.Following || membership.Following != opts.Following)
		if followingNeedsUpdate || opts.IncrementMentions || opts.UpdateViewedTimestamp {
			if followingNeedsUpdate {
				membership.Following = opts.Following
			}
			if opts.UpdateViewedTimestamp {
				membership.LastViewed = now
			}
			membership.LastUpdated = now
			if opts.IncrementMentions {
				membership.UnreadMentions += 1
			}
			if _, err = s.updateMembership(trx, membership); err != nil {
				return nil, err
			}
		}

		if err = trx.Commit(); err != nil {
			return nil, errors.Wrap(err, "commit_transaction")
		}

		return membership, err
	}

	var nfErr *store.ErrNotFound
	if !errors.As(err, &nfErr) {
		return nil, errors.Wrap(err, "failed to get thread membership")
	}

	membership = &model.ThreadMembership{
		PostID:      postID,
		UserID:      userID,
		Following:   opts.Following,
		LastUpdated: now,
	}
	if opts.IncrementMentions {
		membership.UnreadMentions = 1
	}
	if opts.UpdateViewedTimestamp {
		membership.LastViewed = now
	}
	membership, err = s.saveMembership(trx, membership)
	if err != nil {
		return nil, err
	}

	if opts.UpdateParticipants {
		thread, getErr := s.get(trx, postID)
		if getErr != nil {
			return nil, getErr
		}
		if thread != nil && !thread.Participants.Contains(userID) {
			thread.Participants = append(thread.Participants, userID)
			if _, err = s.update(trx, thread); err != nil {
				return nil, err
			}
		}
	}

	if err = trx.Commit(); err != nil {
		return nil, errors.Wrap(err, "commit_transaction")
	}

	return membership, err
}

func (s *SQLThreadStore) CollectThreadsWithNewerReplies(userID string, channelIDs []string, timestamp int64) ([]string, error) {
	var changedThreads []string
	query, args, _ := s.getQueryBuilder().
		Select("Threads.PostId").
		From("Threads").
		LeftJoin("ChannelMembers ON ChannelMembers.ChannelId=Threads.ChannelId").
		Where(sq.And{
			sq.Eq{"Threads.ChannelId": channelIDs},
			sq.Eq{"ChannelMembers.UserId": userID},
			sq.Or{
				sq.Expr("Threads.LastReplyAt >= ChannelMembers.LastViewedAt"),
				sq.GtOrEq{"Threads.LastReplyAt": timestamp},
			},
		}).
		ToSql()
	if _, err := s.GetReplica().Select(&changedThreads, query, args...); err != nil {
		return nil, errors.Wrap(err, "failed to fetch threads")
	}
	return changedThreads, nil
}

func (s *SQLThreadStore) UpdateUnreadsByChannel(userID string, changedThreads []string, timestamp int64, updateViewedTimestamp bool) error {
	if len(changedThreads) == 0 {
		return nil
	}

	qb := s.getQueryBuilder().
		Update("ThreadMemberships").
		Where(sq.Eq{"UserId": userID, "PostId": changedThreads}).
		Set("LastUpdated", timestamp)

	if updateViewedTimestamp {
		qb = qb.Set("LastViewed", timestamp)
	}
	updateQuery, updateArgs, _ := qb.ToSql()

	if _, err := s.GetMaster().Exec(updateQuery, updateArgs...); err != nil {
		return errors.Wrap(err, "failed to update thread membership")
	}

	return nil
}

func (s *SQLThreadStore) GetPosts(threadID string, since int64) ([]*model.Post, error) {
	query, args, _ := s.getQueryBuilder().
		Select("*").
		From("Posts").
		Where(sq.Eq{"RootId": threadID}).
		Where(sq.Eq{"DeleteAt": 0}).
		Where(sq.GtOrEq{"UpdateAt": since}).ToSql()
	var result []*model.Post
	if _, err := s.GetReplica().Select(&result, query, args...); err != nil {
		return nil, errors.Wrap(err, "failed to fetch thread posts")
	}
	return result, nil
}

// PermanentDeleteBatchForRetentionPolicies deletes a batch of records which are affected by
// the global or a granular retention policy.
// See `genericPermanentDeleteBatchForRetentionPolicies` for details.
func (s *SQLThreadStore) PermanentDeleteBatchForRetentionPolicies(now, globalPolicyEndTime, limit int64, cursor model.RetentionPolicyCursor) (int64, model.RetentionPolicyCursor, error) {
	builder := s.getQueryBuilder().
		Select("Threads.PostId").
		From("Threads")
	return genericPermanentDeleteBatchForRetentionPolicies(RetentionPolicyBatchDeletionInfo{
		BaseBuilder:         builder,
		Table:               "Threads",
		TimeColumn:          "LastReplyAt",
		PrimaryKeys:         []string{"PostId"},
		ChannelIDTable:      "Threads",
		NowMillis:           now,
		GlobalPolicyEndTime: globalPolicyEndTime,
		Limit:               limit,
	}, s.SQLStore, cursor)
}

// PermanentDeleteBatchThreadMembershipsForRetentionPolicies deletes a batch of records
// which are affected by the global or a granular retention policy.
// See `genericPermanentDeleteBatchForRetentionPolicies` for details.
func (s *SQLThreadStore) PermanentDeleteBatchThreadMembershipsForRetentionPolicies(now, globalPolicyEndTime, limit int64, cursor model.RetentionPolicyCursor) (int64, model.RetentionPolicyCursor, error) {
	builder := s.getQueryBuilder().
		Select("ThreadMemberships.PostId").
		From("ThreadMemberships").
		InnerJoin("Threads ON ThreadMemberships.PostId = Threads.PostId")
	return genericPermanentDeleteBatchForRetentionPolicies(RetentionPolicyBatchDeletionInfo{
		BaseBuilder:         builder,
		Table:               "ThreadMemberships",
		TimeColumn:          "LastUpdated",
		PrimaryKeys:         []string{"PostId"},
		ChannelIDTable:      "Threads",
		NowMillis:           now,
		GlobalPolicyEndTime: globalPolicyEndTime,
		Limit:               limit,
	}, s.SQLStore, cursor)
}

// DeleteOrphanedRows removes orphaned rows from Threads and ThreadMemberships
func (s *SQLThreadStore) DeleteOrphanedRows(limit int) (deleted int64, err error) {
	// We need the extra level of nesting to deal with MySQL's locking
	const threadsQuery = `
	DELETE FROM Threads WHERE PostId IN (
		SELECT * FROM (
			SELECT Threads.PostId FROM Threads
			LEFT JOIN Channels ON Threads.ChannelId = Channels.Id
			WHERE Channels.Id IS NULL
			LIMIT :Limit
		) AS A
	)`
	// We only delete a thread membership if the entire thread no longer exists,
	// not if the root post has been deleted
	const threadMembershipsQuery = `
	DELETE FROM ThreadMemberships WHERE PostId IN (
		SELECT * FROM (
			SELECT ThreadMemberships.PostId FROM ThreadMemberships
			LEFT JOIN Threads ON ThreadMemberships.PostId = Threads.PostId
			WHERE Threads.PostId IS NULL
			LIMIT :Limit
		) AS A
	)`
	props := map[string]interface{}{"Limit": limit}
	result, err := s.GetMaster().Exec(threadsQuery, props)
	if err != nil {
		return
	}
	rpcDeleted, err := result.RowsAffected()
	if err != nil {
		return
	}
	result, err = s.GetMaster().Exec(threadMembershipsQuery, props)
	if err != nil {
		return
	}
	rptDeleted, err := result.RowsAffected()
	if err != nil {
		return
	}
	deleted = rpcDeleted + rptDeleted
	return
}
