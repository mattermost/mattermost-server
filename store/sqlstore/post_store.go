// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/gorp"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/searchlayer"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type SQLPostStore struct {
	*SQLStore
	metrics           einterfaces.MetricsInterface
	maxPostSizeOnce   sync.Once
	maxPostSizeCached int
}

type postWithExtra struct {
	ThreadReplyCount   int64
	IsFollowing        *bool
	ThreadParticipants model.StringArray
	model.Post
}

func (s *SQLPostStore) ClearCaches() {
}

func postSliceColumnsWithTypes() []struct {
	Name string
	Type reflect.Kind
} {
	return []struct {
		Name string
		Type reflect.Kind
	}{
		{"Id", reflect.String},
		{"CreateAt", reflect.Int64},
		{"UpdateAt", reflect.Int64},
		{"EditAt", reflect.Int64},
		{"DeleteAt", reflect.Int64},
		{"IsPinned", reflect.Bool},
		{"UserId", reflect.String},
		{"ChannelId", reflect.String},
		{"RootId", reflect.String},
		{"ParentId", reflect.String},
		{"OriginalId", reflect.String},
		{"Message", reflect.String},
		{"Type", reflect.String},
		{"Props", reflect.Map},
		{"Hashtags", reflect.String},
		{"Filenames", reflect.Slice},
		{"FileIds", reflect.Slice},
		{"HasReactions", reflect.Bool},
		{"RemoteId", reflect.String},
	}
}

func postToSlice(post *model.Post) []interface{} {
	return []interface{}{
		post.ID,
		post.CreateAt,
		post.UpdateAt,
		post.EditAt,
		post.DeleteAt,
		post.IsPinned,
		post.UserID,
		post.ChannelID,
		post.RootID,
		post.ParentID,
		post.OriginalID,
		post.Message,
		post.Type,
		model.StringInterfaceToJSON(post.Props),
		post.Hashtags,
		model.ArrayToJSON(post.Filenames),
		model.ArrayToJSON(post.FileIDs),
		post.HasReactions,
		post.RemoteID,
	}
}

func postSliceColumns() []string {
	colInfos := postSliceColumnsWithTypes()
	cols := make([]string, len(colInfos))
	for i, colInfo := range colInfos {
		cols[i] = colInfo.Name
	}
	return cols
}

func postSliceCoalesceQuery() string {
	colInfos := postSliceColumnsWithTypes()
	cols := make([]string, len(colInfos))
	for i, colInfo := range colInfos {
		var defaultValue string
		switch colInfo.Type {
		case reflect.String:
			defaultValue = "''"
		case reflect.Int64:
			defaultValue = "0"
		case reflect.Bool:
			defaultValue = "false"
		case reflect.Map:
			defaultValue = "'{}'"
		case reflect.Slice:
			defaultValue = "'[]'"
		}
		cols[i] = "COALESCE(Posts." + colInfo.Name + "," + defaultValue + ") AS " + colInfo.Name
	}
	return strings.Join(cols, ",")
}

func newSQLPostStore(sqlStore *SQLStore, metrics einterfaces.MetricsInterface) store.PostStore {
	s := &SQLPostStore{
		SQLStore:          sqlStore,
		metrics:           metrics,
		maxPostSizeCached: model.PostMessageMaxRunesV1,
	}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Post{}, "Posts").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("ChannelId").SetMaxSize(26)
		table.ColMap("RootId").SetMaxSize(26)
		table.ColMap("ParentId").SetMaxSize(26)
		table.ColMap("OriginalId").SetMaxSize(26)
		table.ColMap("Message").SetMaxSize(model.PostMessageMaxBytesV2)
		table.ColMap("Type").SetMaxSize(26)
		table.ColMap("Hashtags").SetMaxSize(1000)
		table.ColMap("Props").SetMaxSize(8000)
		table.ColMap("Filenames").SetMaxSize(model.PostFilenamesMaxRunes)
		table.ColMap("FileIds").SetMaxSize(300)
		table.ColMap("RemoteId").SetMaxSize(26)
	}

	return s
}

func (s *SQLPostStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_posts_update_at", "Posts", "UpdateAt")
	s.CreateIndexIfNotExists("idx_posts_create_at", "Posts", "CreateAt")
	s.CreateIndexIfNotExists("idx_posts_delete_at", "Posts", "DeleteAt")
	s.CreateIndexIfNotExists("idx_posts_root_id", "Posts", "RootId")
	s.CreateIndexIfNotExists("idx_posts_user_id", "Posts", "UserId")
	s.CreateIndexIfNotExists("idx_posts_is_pinned", "Posts", "IsPinned")

	s.CreateCompositeIndexIfNotExists("idx_posts_channel_id_update_at", "Posts", []string{"ChannelId", "UpdateAt"})
	s.CreateCompositeIndexIfNotExists("idx_posts_channel_id_delete_at_create_at", "Posts", []string{"ChannelId", "DeleteAt", "CreateAt"})

	s.CreateFullTextIndexIfNotExists("idx_posts_message_txt", "Posts", "Message")
	s.CreateFullTextIndexIfNotExists("idx_posts_hashtags_txt", "Posts", "Hashtags")
}

func (s *SQLPostStore) SaveMultiple(posts []*model.Post) ([]*model.Post, int, error) {
	channelNewPosts := make(map[string]int)
	channelNewRootPosts := make(map[string]int)
	maxDateNewPosts := make(map[string]int64)
	rootIDs := make(map[string]int)
	maxDateRootIDs := make(map[string]int64)
	for idx, post := range posts {
		if post.ID != "" && !post.IsRemote() {
			return nil, idx, store.NewErrInvalidInput("Post", "id", post.ID)
		}
		post.PreSave()
		maxPostSize := s.GetMaxPostSize()
		if err := post.IsValid(maxPostSize); err != nil {
			return nil, idx, err
		}

		if currentChannelCount, ok := channelNewPosts[post.ChannelID]; !ok {
			if post.IsJoinLeaveMessage() {
				channelNewPosts[post.ChannelID] = 0
			} else {
				channelNewPosts[post.ChannelID] = 1
			}
			maxDateNewPosts[post.ChannelID] = post.CreateAt
		} else {
			if !post.IsJoinLeaveMessage() {
				channelNewPosts[post.ChannelID] = currentChannelCount + 1
			}
			if post.CreateAt > maxDateNewPosts[post.ChannelID] {
				maxDateNewPosts[post.ChannelID] = post.CreateAt
			}
		}

		if post.RootID == "" {
			if currentChannelCount, ok := channelNewRootPosts[post.ChannelID]; !ok {
				if post.IsJoinLeaveMessage() {
					channelNewRootPosts[post.ChannelID] = 0
				} else {
					channelNewRootPosts[post.ChannelID] = 1
				}
			} else {
				if !post.IsJoinLeaveMessage() {
					channelNewRootPosts[post.ChannelID] = currentChannelCount + 1
				}
			}
			continue
		}

		if currentRootCount, ok := rootIDs[post.RootID]; !ok {
			rootIDs[post.RootID] = 1
			maxDateRootIDs[post.RootID] = post.CreateAt
		} else {
			rootIDs[post.RootID] = currentRootCount + 1
			if post.CreateAt > maxDateRootIDs[post.RootID] {
				maxDateRootIDs[post.RootID] = post.CreateAt
			}
		}
	}

	builder := s.getQueryBuilder().Insert("Posts").Columns(postSliceColumns()...)
	for _, post := range posts {
		builder = builder.Values(postToSlice(post)...)
	}
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, -1, errors.Wrap(err, "post_tosql")
	}

	transaction, err := s.GetMaster().Begin()
	if err != nil {
		return posts, -1, errors.Wrap(err, "begin_transaction")
	}

	defer finalizeTransaction(transaction)

	if _, err = transaction.Exec(query, args...); err != nil {
		return nil, -1, errors.Wrap(err, "failed to save Post")
	}

	if err = s.updateThreadsFromPosts(transaction, posts); err != nil {
		mlog.Warn("Error updating posts, thread update failed", mlog.Err(err))
	}

	if err = transaction.Commit(); err != nil {
		// don't need to rollback here since the transaction is already closed
		return posts, -1, errors.Wrap(err, "commit_transaction")
	}

	for channelID, count := range channelNewPosts {
		countRoot := channelNewRootPosts[channelID]

		if _, err = s.GetMaster().Exec("UPDATE Channels SET LastPostAt = GREATEST(:LastPostAt, LastPostAt), TotalMsgCount = TotalMsgCount + :Count, TotalMsgCountRoot = TotalMsgCountRoot + :CountRoot WHERE Id = :ChannelId", map[string]interface{}{"LastPostAt": maxDateNewPosts[channelID], "ChannelId": channelID, "Count": count, "CountRoot": countRoot}); err != nil {
			mlog.Warn("Error updating Channel LastPostAt.", mlog.Err(err))
		}
	}

	for rootID := range rootIDs {
		if _, err = s.GetMaster().Exec("UPDATE Posts SET UpdateAt = :UpdateAt WHERE Id = :RootId", map[string]interface{}{"UpdateAt": maxDateRootIDs[rootID], "RootId": rootID}); err != nil {
			mlog.Warn("Error updating Post UpdateAt.", mlog.Err(err))
		}
	}

	var unknownRepliesPosts []*model.Post
	for _, post := range posts {
		if post.RootID == "" {
			count, ok := rootIDs[post.ID]
			if ok {
				post.ReplyCount += int64(count)
			}
		} else {
			unknownRepliesPosts = append(unknownRepliesPosts, post)
		}
	}

	if len(unknownRepliesPosts) > 0 {
		if err := s.populateReplyCount(unknownRepliesPosts); err != nil {
			mlog.Warn("Unable to populate the reply count in some posts.", mlog.Err(err))
		}
	}

	return posts, -1, nil
}

func (s *SQLPostStore) Save(post *model.Post) (*model.Post, error) {
	posts, _, err := s.SaveMultiple([]*model.Post{post})
	if err != nil {
		return nil, err
	}
	return posts[0], nil
}

func (s *SQLPostStore) populateReplyCount(posts []*model.Post) error {
	rootIDs := []string{}
	for _, post := range posts {
		rootIDs = append(rootIDs, post.RootID)
	}
	countList := []struct {
		RootID string
		Count  int64
	}{}
	query := s.getQueryBuilder().Select("RootId, COUNT(Id) AS Count").From("Posts").Where(sq.Eq{"RootId": rootIDs}).Where(sq.Eq{"DeleteAt": 0}).GroupBy("RootId")

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "post_tosql")
	}
	_, err = s.GetMaster().Select(&countList, queryString, args...)
	if err != nil {
		return errors.Wrap(err, "failed to count Posts")
	}

	counts := map[string]int64{}
	for _, count := range countList {
		counts[count.RootID] = count.Count
	}

	for _, post := range posts {
		count, ok := counts[post.RootID]
		if !ok {
			post.ReplyCount = 0
		}
		post.ReplyCount = count
	}

	return nil
}

func (s *SQLPostStore) Update(newPost *model.Post, oldPost *model.Post) (*model.Post, error) {
	newPost.UpdateAt = model.GetMillis()
	newPost.PreCommit()

	oldPost.DeleteAt = newPost.UpdateAt
	oldPost.UpdateAt = newPost.UpdateAt
	oldPost.OriginalID = oldPost.ID
	oldPost.ID = model.NewID()
	oldPost.PreCommit()

	maxPostSize := s.GetMaxPostSize()

	if err := newPost.IsValid(maxPostSize); err != nil {
		return nil, err
	}

	if _, err := s.GetMaster().Update(newPost); err != nil {
		return nil, errors.Wrapf(err, "failed to update Post with id=%s", newPost.ID)
	}

	time := model.GetMillis()
	s.GetMaster().Exec("UPDATE Channels SET LastPostAt = :LastPostAt  WHERE Id = :ChannelId AND LastPostAt < :LastPostAt", map[string]interface{}{"LastPostAt": time, "ChannelId": newPost.ChannelID})

	if newPost.RootID != "" {
		s.GetMaster().Exec("UPDATE Posts SET UpdateAt = :UpdateAt WHERE Id = :RootId AND UpdateAt < :UpdateAt", map[string]interface{}{"UpdateAt": time, "RootId": newPost.RootID})
		s.GetMaster().Exec("UPDATE Threads SET LastReplyAt = :UpdateAt WHERE PostId = :RootId", map[string]interface{}{"UpdateAt": time, "RootId": newPost.RootID})
	}

	// mark the old post as deleted
	s.GetMaster().Insert(oldPost)

	return newPost, nil
}

func (s *SQLPostStore) OverwriteMultiple(posts []*model.Post) ([]*model.Post, int, error) {
	updateAt := model.GetMillis()
	maxPostSize := s.GetMaxPostSize()
	for idx, post := range posts {
		post.UpdateAt = updateAt
		if appErr := post.IsValid(maxPostSize); appErr != nil {
			return nil, idx, appErr
		}
	}

	tx, err := s.GetMaster().Begin()
	if err != nil {
		return nil, -1, errors.Wrap(err, "begin_transaction")
	}
	for idx, post := range posts {
		if _, err = tx.Update(post); err != nil {
			txErr := tx.Rollback()
			if txErr != nil {
				return nil, idx, errors.Wrap(txErr, "rollback_transaction")
			}

			return nil, idx, errors.Wrap(err, "failed to update Post")
		}
		if post.RootID != "" {
			tx.Exec("UPDATE Threads SET LastReplyAt = :UpdateAt WHERE PostId = :RootId", map[string]interface{}{"UpdateAt": updateAt, "RootId": post.ID})
		}
	}
	err = tx.Commit()
	if err != nil {
		return nil, -1, errors.Wrap(err, "commit_transaction")
	}

	return posts, -1, nil
}

func (s *SQLPostStore) Overwrite(post *model.Post) (*model.Post, error) {
	posts, _, err := s.OverwriteMultiple([]*model.Post{post})
	if err != nil {
		return nil, err
	}

	return posts[0], nil
}

func (s *SQLPostStore) GetFlaggedPosts(userID string, offset int, limit int) (*model.PostList, error) {
	pl := model.NewPostList()

	var posts []*model.Post
	if _, err := s.GetReplica().Select(&posts, "SELECT *, (SELECT count(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN p.RootId = '' THEN p.Id ELSE p.RootId END) AND Posts.DeleteAt = 0) as ReplyCount FROM Posts p WHERE Id IN (SELECT Name FROM Preferences WHERE UserId = :UserId AND Category = :Category) AND DeleteAt = 0 ORDER BY CreateAt DESC LIMIT :Limit OFFSET :Offset", map[string]interface{}{"UserId": userID, "Category": model.PreferenceCategoryFlaggedPost, "Offset": offset, "Limit": limit}); err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}

	for _, post := range posts {
		pl.AddPost(post)
		pl.AddOrder(post.ID)
	}

	return pl, nil
}

func (s *SQLPostStore) GetFlaggedPostsForTeam(userID, teamID string, offset int, limit int) (*model.PostList, error) {
	pl := model.NewPostList()

	var posts []*model.Post

	query := `
            SELECT
                A.*, (SELECT count(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN A.RootId = '' THEN A.Id ELSE A.RootId END) AND Posts.DeleteAt = 0) as ReplyCount
            FROM
                (SELECT
                    *
                FROM
                    Posts
                WHERE
                    Id
                IN
                    (SELECT
                        Name
                    FROM
                        Preferences
                    WHERE
                        UserId = :UserId
                        AND Category = :Category)
                        AND DeleteAt = 0
                ) as A
            INNER JOIN Channels as B
                ON B.Id = A.ChannelId
            WHERE B.TeamId = :TeamId OR B.TeamId = ''
            ORDER BY CreateAt DESC
            LIMIT :Limit OFFSET :Offset`

	if _, err := s.GetReplica().Select(&posts, query, map[string]interface{}{"UserId": userID, "Category": model.PreferenceCategoryFlaggedPost, "Offset": offset, "Limit": limit, "TeamId": teamID}); err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}

	for _, post := range posts {
		pl.AddPost(post)
		pl.AddOrder(post.ID)
	}

	return pl, nil
}

func (s *SQLPostStore) GetFlaggedPostsForChannel(userID, channelID string, offset int, limit int) (*model.PostList, error) {
	pl := model.NewPostList()

	var posts []*model.Post
	query := `
		SELECT
			*, (SELECT count(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN p.RootId = '' THEN p.Id ELSE p.RootId END) AND Posts.DeleteAt = 0) as ReplyCount
		FROM Posts p
		WHERE
			Id IN (SELECT Name FROM Preferences WHERE UserId = :UserId AND Category = :Category)
			AND ChannelId = :ChannelId
			AND DeleteAt = 0
		ORDER BY CreateAt DESC
		LIMIT :Limit OFFSET :Offset`

	if _, err := s.GetReplica().Select(&posts, query, map[string]interface{}{"UserId": userID, "Category": model.PreferenceCategoryFlaggedPost, "ChannelId": channelID, "Offset": offset, "Limit": limit}); err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}
	for _, post := range posts {
		pl.AddPost(post)
		pl.AddOrder(post.ID)
	}

	return pl, nil
}
func (s *SQLPostStore) getPostWithCollapsedThreads(id, userID string, extended bool) (*model.PostList, error) {
	if id == "" {
		return nil, store.NewErrInvalidInput("Post", "id", id)
	}

	var columns []string
	for _, c := range postSliceColumns() {
		columns = append(columns, "Posts."+c)
	}
	columns = append(columns,
		"COALESCE(Threads.ReplyCount, 0) as ThreadReplyCount",
		"COALESCE(Threads.LastReplyAt, 0) as LastReplyAt",
		"COALESCE(Threads.Participants, '[]') as ThreadParticipants",
		"COALESCE(ThreadMemberships.Following, false) as IsFollowing",
	)
	var post postWithExtra

	postFetchQuery, args, _ := s.getQueryBuilder().
		Select(columns...).
		From("Posts").
		LeftJoin("Threads ON Threads.PostId = Id").
		LeftJoin("ThreadMemberships ON ThreadMemberships.PostId = Id AND ThreadMemberships.UserId = ?", userID).
		Where(sq.Eq{"DeleteAt": 0}).
		Where(sq.Eq{"Id": id}).ToSql()

	err := s.GetReplica().SelectOne(&post, postFetchQuery, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Post", id)
		}

		return nil, errors.Wrapf(err, "failed to get Post with id=%s", id)
	}

	var posts []*model.Post
	_, err = s.GetReplica().Select(&posts, "SELECT * FROM Posts WHERE Posts.RootId = :RootId AND DeleteAt = 0", map[string]interface{}{"RootId": id})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts for thread %s", id)
	}

	list, err := s.prepareThreadedResponse([]*postWithExtra{&post}, extended, false)
	if err != nil {
		return nil, err
	}
	for _, p := range posts {
		list.AddPost(p)
		list.AddOrder(p.ID)
	}
	return list, nil
}

func (s *SQLPostStore) Get(ctx context.Context, id string, skipFetchThreads, collapsedThreads, collapsedThreadsExtended bool, userID string) (*model.PostList, error) {
	if collapsedThreads {
		return s.getPostWithCollapsedThreads(id, userID, collapsedThreadsExtended)
	}
	pl := model.NewPostList()

	if id == "" {
		return nil, store.NewErrInvalidInput("Post", "id", id)
	}

	var post model.Post
	postFetchQuery := "SELECT p.*, (SELECT count(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN p.RootId = '' THEN p.Id ELSE p.RootId END) AND Posts.DeleteAt = 0) as ReplyCount FROM Posts p WHERE p.Id = :Id AND p.DeleteAt = 0"
	err := s.DBFromContext(ctx).SelectOne(&post, postFetchQuery, map[string]interface{}{"Id": id})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Post", id)
		}

		return nil, errors.Wrapf(err, "failed to get Post with id=%s", id)
	}
	pl.AddPost(&post)
	pl.AddOrder(id)
	if !skipFetchThreads {
		rootID := post.RootID

		if rootID == "" {
			rootID = post.ID
		}

		if rootID == "" {
			return nil, errors.Wrapf(err, "invalid rootId with value=%s", rootID)
		}

		var posts []*model.Post
		_, err = s.GetReplica().Select(&posts, "SELECT *, (SELECT count(Id) FROM Posts WHERE Posts.RootId = (CASE WHEN p.RootId = '' THEN p.Id ELSE p.RootId END) AND Posts.DeleteAt = 0) as ReplyCount FROM Posts p WHERE (Id = :Id OR RootId = :RootId) AND DeleteAt = 0", map[string]interface{}{"Id": rootID, "RootId": rootID})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find Posts")
		}

		for _, p := range posts {
			pl.AddPost(p)
			pl.AddOrder(p.ID)
		}
	}
	return pl, nil
}

func (s *SQLPostStore) GetSingle(id string, inclDeleted bool) (*model.Post, error) {
	query := s.getQueryBuilder().
		Select("*").
		From("Posts").
		Where(sq.Eq{"Id": id})

	if !inclDeleted {
		query = query.Where(sq.Eq{"DeleteAt": 0})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "getsingleincldeleted_tosql")
	}

	var post model.Post
	err = s.GetReplica().SelectOne(&post, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Post", id)
		}

		return nil, errors.Wrapf(err, "failed to get Post with id=%s", id)
	}
	return &post, nil
}

type etagPosts struct {
	ID       string
	UpdateAt int64
}

//nolint:unparam
func (s *SQLPostStore) InvalidateLastPostTimeCache(channelID string) {
}

//nolint:unparam
func (s *SQLPostStore) GetEtag(channelID string, allowFromCache, collapsedThreads bool) string {
	q := s.getQueryBuilder().Select("Id", "UpdateAt").From("Posts").Where(sq.Eq{"ChannelId": channelID}).OrderBy("UpdateAt DESC").Limit(1)
	if collapsedThreads {
		q.Where(sq.Eq{"RootId": ""})
	}
	sql, args, _ := q.ToSql()

	var et etagPosts
	err := s.GetReplica().SelectOne(&et, sql, args...)
	var result string
	if err != nil {
		result = fmt.Sprintf("%v.%v", model.CurrentVersion, model.GetMillis())
	} else {
		result = fmt.Sprintf("%v.%v", model.CurrentVersion, et.UpdateAt)
	}

	return result
}

func (s *SQLPostStore) Delete(postID string, time int64, deleteByID string) error {
	var post model.Post
	err := s.GetReplica().SelectOne(&post, "SELECT * FROM Posts WHERE Id = :Id AND DeleteAt = 0", map[string]interface{}{"Id": postID})
	if err != nil {
		if err == sql.ErrNoRows {
			return store.NewErrNotFound("Post", postID)
		}

		return errors.Wrapf(err, "failed to delete Post with id=%s", postID)
	}

	post.AddProp(model.PostPropsDeleteBy, deleteByID)

	_, err = s.GetMaster().Exec("UPDATE Posts SET DeleteAt = :DeleteAt, UpdateAt = :UpdateAt, Props = :Props WHERE Id = :Id OR RootId = :RootId", map[string]interface{}{"DeleteAt": time, "UpdateAt": time, "Id": postID, "RootId": postID, "Props": model.StringInterfaceToJSON(post.GetProps())})
	if err != nil {
		return errors.Wrap(err, "failed to update Posts")
	}

	return s.cleanupThreads(post.ID, post.RootID, false)
}

func (s *SQLPostStore) permanentDelete(postID string) error {
	var post model.Post
	err := s.GetReplica().SelectOne(&post, "SELECT * FROM Posts WHERE Id = :Id AND DeleteAt = 0", map[string]interface{}{"Id": postID})
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrapf(err, "failed to get Post with id=%s", postID)
	}
	if err = s.cleanupThreads(post.ID, post.RootID, true); err != nil {
		return errors.Wrapf(err, "failed to cleanup threads for Post with id=%s", postID)
	}

	if _, err = s.GetMaster().Exec("DELETE FROM Posts WHERE Id = :Id OR RootId = :RootId", map[string]interface{}{"Id": postID, "RootId": postID}); err != nil {
		return errors.Wrapf(err, "failed to delete Post with id=%s", postID)
	}

	return nil
}

type postIDs struct {
	ID     string
	RootID string
	UserID string
}

func (s *SQLPostStore) permanentDeleteAllCommentByUser(userID string) error {
	results := []postIDs{}
	_, err := s.GetMaster().Select(&results, "Select Id, RootId FROM Posts WHERE UserId = :UserId AND RootId != ''", map[string]interface{}{"UserId": userID})
	if err != nil {
		return errors.Wrapf(err, "failed to fetch Posts with userId=%s", userID)
	}

	for _, ids := range results {
		if err = s.cleanupThreads(ids.ID, ids.RootID, true); err != nil {
			return err
		}
	}

	_, err = s.GetMaster().Exec("DELETE FROM Posts WHERE UserId = :UserId AND RootId != ''", map[string]interface{}{"UserId": userID})
	if err != nil {
		return errors.Wrapf(err, "failed to delete Posts with userId=%s", userID)
	}
	return nil
}

func (s *SQLPostStore) PermanentDeleteByUser(userID string) error {
	// First attempt to delete all the comments for a user
	if err := s.permanentDeleteAllCommentByUser(userID); err != nil {
		return err
	}

	// Now attempt to delete all the root posts for a user. This will also
	// delete all the comments for each post
	found := true
	count := 0

	for found {
		var ids []string
		_, err := s.GetMaster().Select(&ids, "SELECT Id FROM Posts WHERE UserId = :UserId LIMIT 1000", map[string]interface{}{"UserId": userID})
		if err != nil {
			return errors.Wrapf(err, "failed to find Posts with userId=%s", userID)
		}

		found = false
		for _, id := range ids {
			found = true
			if err = s.permanentDelete(id); err != nil {
				return err
			}
		}

		// This is a fail safe, give up if more than 10k messages
		count++
		if count >= 10 {
			return errors.Wrapf(err, "too many Posts to delete with userId=%s", userID)
		}
	}

	return nil
}

func (s *SQLPostStore) PermanentDeleteByChannel(channelID string) error {
	results := []postIDs{}
	_, err := s.GetMaster().Select(&results, "SELECT Id, RootId, UserId FROM Posts WHERE ChannelId = :ChannelId", map[string]interface{}{"ChannelId": channelID})
	if err != nil {
		return errors.Wrapf(err, "failed to fetch Posts with channelId=%s", channelID)
	}

	for _, ids := range results {
		if err = s.cleanupThreads(ids.ID, ids.RootID, true); err != nil {
			return err
		}
	}

	if _, err := s.GetMaster().Exec("DELETE FROM Posts WHERE ChannelId = :ChannelId", map[string]interface{}{"ChannelId": channelID}); err != nil {
		return errors.Wrapf(err, "failed to delete Posts with channelId=%s", channelID)
	}
	return nil
}

func (s *SQLPostStore) prepareThreadedResponse(posts []*postWithExtra, extended, reversed bool) (*model.PostList, error) {
	list := model.NewPostList()
	var userIDs []string
	userIDMap := map[string]bool{}
	for _, thread := range posts {
		for _, participantID := range thread.ThreadParticipants {
			if _, ok := userIDMap[participantID]; !ok {
				userIDMap[participantID] = true
				userIDs = append(userIDs, participantID)
			}
		}
	}
	var users []*model.User
	if extended {
		var err error
		users, err = s.User().GetProfileByIDs(context.Background(), userIDs, &store.UserGetByIDsOpts{}, true)
		if err != nil {
			return nil, err
		}
	} else {
		for _, userID := range userIDs {
			users = append(users, &model.User{ID: userID})
		}
	}
	processPost := func(p *postWithExtra) error {
		p.Post.ReplyCount = p.ThreadReplyCount
		if p.IsFollowing != nil {
			p.Post.IsFollowing = model.NewBool(*p.IsFollowing)
		}
		for _, th := range p.ThreadParticipants {
			var participant *model.User
			for _, u := range users {
				if u.ID == th {
					participant = u
					break
				}
			}
			if participant == nil {
				return errors.New("cannot find thread participant with id=" + th)
			}
			p.Post.Participants = append(p.Post.Participants, participant)
		}
		return nil
	}

	l := len(posts)
	for i := range posts {
		idx := i
		// We need to flip the order if we selected backwards

		if reversed {
			idx = l - i - 1
		}
		if err := processPost(posts[idx]); err != nil {
			return nil, err
		}
		list.AddPost(&posts[idx].Post)
		list.AddOrder(posts[idx].ID)
	}

	return list, nil
}

func (s *SQLPostStore) getPostsCollapsedThreads(options model.GetPostsOptions) (*model.PostList, error) {
	var columns []string
	for _, c := range postSliceColumns() {
		columns = append(columns, "Posts."+c)
	}
	columns = append(columns,
		"COALESCE(Threads.ReplyCount, 0) as ThreadReplyCount",
		"COALESCE(Threads.LastReplyAt, 0) as LastReplyAt",
		"COALESCE(Threads.Participants, '[]') as ThreadParticipants",
		"COALESCE(ThreadMemberships.Following, false) as IsFollowing",
	)
	var posts []*postWithExtra
	offset := options.PerPage * options.Page

	postFetchQuery, args, _ := s.getQueryBuilder().
		Select(columns...).
		From("Posts").
		LeftJoin("Threads ON Threads.PostId = Id").
		LeftJoin("ThreadMemberships ON ThreadMemberships.PostId = Id AND ThreadMemberships.UserId = ?", options.UserID).
		Where(sq.Eq{"DeleteAt": 0}).
		Where(sq.Eq{"Posts.ChannelId": options.ChannelID}).
		Where(sq.Eq{"RootId": ""}).
		Limit(uint64(options.PerPage)).
		Offset(uint64(offset)).
		OrderBy("CreateAt DESC").ToSql()

	_, err := s.GetReplica().Select(&posts, postFetchQuery, args...)

	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts with channelId=%s", options.ChannelID)
	}

	return s.prepareThreadedResponse(posts, options.CollapsedThreadsExtended, false)
}

func (s *SQLPostStore) GetPosts(options model.GetPostsOptions, _ bool) (*model.PostList, error) {
	if options.PerPage > 1000 {
		return nil, store.NewErrInvalidInput("Post", "<options.PerPage>", options.PerPage)
	}
	if options.CollapsedThreads {
		return s.getPostsCollapsedThreads(options)
	}
	offset := options.PerPage * options.Page

	rpc := make(chan store.StoreResult, 1)
	go func() {
		posts, err := s.getRootPosts(options.ChannelID, offset, options.PerPage, options.SkipFetchThreads)
		rpc <- store.StoreResult{Data: posts, NErr: err}
		close(rpc)
	}()
	cpc := make(chan store.StoreResult, 1)
	go func() {
		posts, err := s.getParentsPosts(options.ChannelID, offset, options.PerPage, options.SkipFetchThreads)
		cpc <- store.StoreResult{Data: posts, NErr: err}
		close(cpc)
	}()

	list := model.NewPostList()

	rpr := <-rpc
	if rpr.NErr != nil {
		return nil, rpr.NErr
	}

	cpr := <-cpc
	if cpr.NErr != nil {
		return nil, cpr.NErr
	}

	posts := rpr.Data.([]*model.Post)
	parents := cpr.Data.([]*model.Post)

	for _, p := range posts {
		list.AddPost(p)
		list.AddOrder(p.ID)
	}

	for _, p := range parents {
		list.AddPost(p)
	}

	list.MakeNonNil()

	return list, nil
}

func (s *SQLPostStore) getPostsSinceCollapsedThreads(options model.GetPostsSinceOptions) (*model.PostList, error) {
	var columns []string
	for _, c := range postSliceColumns() {
		columns = append(columns, "Posts."+c)
	}
	columns = append(columns,
		"COALESCE(Threads.ReplyCount, 0) as ThreadReplyCount",
		"COALESCE(Threads.LastReplyAt, 0) as LastReplyAt",
		"COALESCE(Threads.Participants, '[]') as ThreadParticipants",
		"COALESCE(ThreadMemberships.Following, false) as IsFollowing",
	)
	var posts []*postWithExtra

	postFetchQuery, args, _ := s.getQueryBuilder().
		Select(columns...).
		From("Posts").
		LeftJoin("Threads ON Threads.PostId = Id").
		LeftJoin("ThreadMemberships ON ThreadMemberships.PostId = Id AND ThreadMemberships.UserId = ?", options.UserID).
		Where(sq.Eq{"DeleteAt": 0}).
		Where(sq.Eq{"Posts.ChannelId": options.ChannelID}).
		Where(sq.Gt{"UpdateAt": options.Time}).
		Where(sq.Eq{"RootId": ""}).
		OrderBy("CreateAt DESC").ToSql()

	_, err := s.GetReplica().Select(&posts, postFetchQuery, args...)

	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts with channelId=%s", options.ChannelID)
	}
	return s.prepareThreadedResponse(posts, options.CollapsedThreadsExtended, false)
}

//nolint:unparam
func (s *SQLPostStore) GetPostsSince(options model.GetPostsSinceOptions, allowFromCache bool) (*model.PostList, error) {
	if options.CollapsedThreads {
		return s.getPostsSinceCollapsedThreads(options)
	}

	var posts []*model.Post

	order := "DESC"
	if options.SortAscending {
		order = "ASC"
	}

	replyCountQuery1 := ""
	replyCountQuery2 := ""
	if options.SkipFetchThreads {
		replyCountQuery1 = `, (SELECT COUNT(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN p1.RootId = '' THEN p1.Id ELSE p1.RootId END) AND Posts.DeleteAt = 0) as ReplyCount`
		replyCountQuery2 = `, (SELECT COUNT(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN cte.RootId = '' THEN cte.Id ELSE cte.RootId END) AND Posts.DeleteAt = 0) as ReplyCount`
	}
	var query string

	// union of IDs and then join to get full posts is faster in mysql
	if s.DriverName() == model.DatabaseDriverMysql {
		query = `SELECT *` + replyCountQuery1 + ` FROM Posts p1 JOIN (
			(SELECT
              Id
			  FROM
				  Posts p2
			  WHERE
				  (UpdateAt > :Time
					  AND ChannelId = :ChannelId)
				  LIMIT 1000)
			  UNION
				  (SELECT
					  Id
				  FROM
					  Posts p3
				  WHERE
					  Id
				  IN
					  (SELECT * FROM (SELECT
						  RootId
					  FROM
						  Posts
					  WHERE
						  UpdateAt > :Time
							  AND ChannelId = :ChannelId
					  LIMIT 1000) temp_tab))
			) j ON p1.Id = j.Id
          ORDER BY CreateAt ` + order
	} else if s.DriverName() == model.DatabaseDriverPostgres {
		query = `WITH cte AS (SELECT
		       *
		FROM
		       Posts
		WHERE
		       UpdateAt > :Time AND ChannelId = :ChannelId
		       LIMIT 1000)
		(SELECT *` + replyCountQuery2 + ` FROM cte)
		UNION
		(SELECT *` + replyCountQuery1 + ` FROM Posts p1 WHERE id in (SELECT rootid FROM cte))
		ORDER BY CreateAt ` + order
	}
	_, err := s.GetReplica().Select(&posts, query, map[string]interface{}{"ChannelId": options.ChannelID, "Time": options.Time})

	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts with channelId=%s", options.ChannelID)
	}

	list := model.NewPostList()

	for _, p := range posts {
		list.AddPost(p)
		if p.UpdateAt > options.Time {
			list.AddOrder(p.ID)
		}
	}

	return list, nil
}

func (s *SQLPostStore) HasAutoResponsePostByUserSince(options model.GetPostsSinceOptions, userID string) (bool, error) {
	query := `
		SELECT 1
		FROM
			Posts
		WHERE
			UpdateAt >= :Time
			AND
			ChannelId = :ChannelId
			AND
			UserId = :UserId
			AND
			Type = :Type
		LIMIT 1`

	exist, err := s.GetReplica().SelectInt(query, map[string]interface{}{
		"ChannelId": options.ChannelID,
		"Time":      options.Time,
		"UserId":    userID,
		"Type":      model.PostTypeAutoResponder,
	})

	if err != nil {
		return false, errors.Wrapf(err,
			"failed to check if autoresponse posts in channelId=%s for userId=%s since %s", options.ChannelID, userID, time.Unix(options.Time, 0).Format(time.RFC3339))
	}

	return exist > 0, nil
}

func (s *SQLPostStore) GetPostsSinceForSync(options model.GetPostsSinceForSyncOptions, cursor model.GetPostsSinceForSyncCursor, limit int) ([]*model.Post, model.GetPostsSinceForSyncCursor, error) {
	query := s.getQueryBuilder().
		Select("*").
		From("Posts").
		Where(sq.Or{sq.Gt{"UpdateAt": cursor.LastPostUpdateAt}, sq.And{sq.Eq{"UpdateAt": cursor.LastPostUpdateAt}, sq.Gt{"Id": cursor.LastPostID}}}).
		OrderBy("UpdateAt", "Id").
		Limit(uint64(limit))

	if options.ChannelID != "" {
		query = query.Where(sq.Eq{"ChannelId": options.ChannelID})
	}

	if !options.IncludeDeleted {
		query = query.Where(sq.Eq{"DeleteAt": 0})
	}

	if options.ExcludeRemoteID != "" {
		query = query.Where(sq.NotEq{"COALESCE(Posts.RemoteId,'')": options.ExcludeRemoteID})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, cursor, errors.Wrap(err, "getpostssinceforsync_tosql")
	}

	var posts []*model.Post
	_, err = s.GetReplica().Select(&posts, queryString, args...)
	if err != nil {
		return nil, cursor, errors.Wrapf(err, "error getting Posts with channelId=%s", options.ChannelID)
	}

	if len(posts) != 0 {
		cursor.LastPostUpdateAt = posts[len(posts)-1].UpdateAt
		cursor.LastPostID = posts[len(posts)-1].ID
	}
	return posts, cursor, nil
}

func (s *SQLPostStore) GetPostsBefore(options model.GetPostsOptions) (*model.PostList, error) {
	return s.getPostsAround(true, options)
}

func (s *SQLPostStore) GetPostsAfter(options model.GetPostsOptions) (*model.PostList, error) {
	return s.getPostsAround(false, options)
}

func (s *SQLPostStore) getPostsAround(before bool, options model.GetPostsOptions) (*model.PostList, error) {
	if options.Page < 0 {
		return nil, store.NewErrInvalidInput("Post", "<options.Page>", options.Page)
	}

	if options.PerPage < 0 {
		return nil, store.NewErrInvalidInput("Post", "<options.PerPage>", options.PerPage)
	}

	offset := options.Page * options.PerPage
	var posts []*postWithExtra
	var parents []*model.Post

	var direction string
	var sort string
	if before {
		direction = "<"
		sort = "DESC"
	} else {
		direction = ">"
		sort = "ASC"
	}
	table := "Posts p"
	// We force MySQL to use the right index to prevent it from accidentally
	// using the index_merge_intersection optimization.
	// See MM-27575.
	if s.DriverName() == model.DatabaseDriverMysql {
		table += " USE INDEX(idx_posts_channel_id_delete_at_create_at)"
	}
	columns := []string{"p.*"}
	if options.CollapsedThreads {
		columns = append(columns,
			"COALESCE(Threads.ReplyCount, 0) as ThreadReplyCount",
			"COALESCE(Threads.LastReplyAt, 0) as LastReplyAt",
			"COALESCE(Threads.Participants, '[]') as ThreadParticipants",
			"COALESCE(ThreadMemberships.Following, false) as IsFollowing",
		)
	}
	query := s.getQueryBuilder().Select(columns...)
	replyCountSubQuery := s.getQueryBuilder().Select("COUNT(Posts.Id)").From("Posts").Where(sq.Expr("Posts.RootId = (CASE WHEN p.RootId = '' THEN p.Id ELSE p.RootId END) AND Posts.DeleteAt = 0"))

	conditions := sq.And{
		sq.Expr(`CreateAt `+direction+` (SELECT CreateAt FROM Posts WHERE Id = ?)`, options.PostID),
		sq.Eq{"p.ChannelId": options.ChannelID},
		sq.Eq{"DeleteAt": int(0)},
	}
	if options.CollapsedThreads {
		conditions = append(conditions, sq.Eq{"RootId": ""})
		query = query.LeftJoin("Threads ON Threads.PostId = p.Id").LeftJoin("ThreadMemberships ON ThreadMemberships.PostId = p.Id AND ThreadMemberships.UserId=?", options.UserID)
	} else {
		query = query.Column(sq.Alias(replyCountSubQuery, "ReplyCount"))
	}
	query = query.From(table).
		Where(conditions).
		// Adding ChannelId and DeleteAt order columns
		// to let mysql choose the "idx_posts_channel_id_delete_at_create_at" index always.
		// See MM-24170.
		OrderBy("p.ChannelId", "DeleteAt", "CreateAt "+sort).
		Limit(uint64(options.PerPage)).
		Offset(uint64(offset))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "post_tosql")
	}
	_, err = s.GetMaster().Select(&posts, queryString, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts with channelId=%s", options.ChannelID)
	}

	if !options.CollapsedThreads && len(posts) > 0 {
		rootIDs := []string{}
		for _, post := range posts {
			rootIDs = append(rootIDs, post.ID)
			if post.RootID != "" {
				rootIDs = append(rootIDs, post.RootID)
			}
		}
		rootQuery := s.getQueryBuilder().Select("p.*")
		idQuery := sq.Or{
			sq.Eq{"Id": rootIDs},
		}
		rootQuery = rootQuery.Column(sq.Alias(replyCountSubQuery, "ReplyCount"))
		if !options.SkipFetchThreads {
			idQuery = append(idQuery, sq.Eq{"RootId": rootIDs}) // preserve original behaviour
		}

		rootQuery = rootQuery.From("Posts p").
			Where(sq.And{
				idQuery,
				sq.Eq{"ChannelId": options.ChannelID},
				sq.Eq{"DeleteAt": 0},
			}).
			OrderBy("CreateAt DESC")

		rootQueryString, rootArgs, nErr := rootQuery.ToSql()

		if nErr != nil {
			return nil, errors.Wrap(nErr, "post_tosql")
		}
		_, nErr = s.GetMaster().Select(&parents, rootQueryString, rootArgs...)
		if nErr != nil {
			return nil, errors.Wrapf(nErr, "failed to find Posts with channelId=%s", options.ChannelID)
		}
	}

	list, err := s.prepareThreadedResponse(posts, options.CollapsedThreadsExtended, !before)
	if err != nil {
		return nil, err
	}

	for _, p := range parents {
		list.AddPost(p)
	}

	return list, nil
}

func (s *SQLPostStore) GetPostIDBeforeTime(channelID string, time int64, collapsedThreads bool) (string, error) {
	return s.getPostIDAroundTime(channelID, time, true, collapsedThreads)
}

func (s *SQLPostStore) GetPostIDAfterTime(channelID string, time int64, collapsedThreads bool) (string, error) {
	return s.getPostIDAroundTime(channelID, time, false, collapsedThreads)
}

func (s *SQLPostStore) getPostIDAroundTime(channelID string, time int64, before bool, collapsedThreads bool) (string, error) {
	var direction sq.Sqlizer
	var sort string
	if before {
		direction = sq.Lt{"CreateAt": time}
		sort = "DESC"
	} else {
		direction = sq.Gt{"CreateAt": time}
		sort = "ASC"
	}

	table := "Posts"
	// We force MySQL to use the right index to prevent it from accidentally
	// using the index_merge_intersection optimization.
	// See MM-27575.
	if s.DriverName() == model.DatabaseDriverMysql {
		table += " USE INDEX(idx_posts_channel_id_delete_at_create_at)"
	}

	conditions := sq.And{
		direction,
		sq.Eq{"ChannelId": channelID},
		sq.Eq{"DeleteAt": int(0)},
	}
	if collapsedThreads {
		conditions = sq.And{conditions, sq.Eq{"RootId": ""}}
	}
	query := s.getQueryBuilder().
		Select("Id").
		From(table).
		Where(conditions).
		// Adding ChannelId and DeleteAt order columns
		// to let mysql choose the "idx_posts_channel_id_delete_at_create_at" index always.
		// See MM-23369.
		OrderBy("ChannelId", "DeleteAt", "CreateAt "+sort).
		Limit(1)

	queryString, args, err := query.ToSql()
	if err != nil {
		return "", errors.Wrap(err, "post_tosql")
	}

	var postID string
	if err := s.GetMaster().SelectOne(&postID, queryString, args...); err != nil {
		if err != sql.ErrNoRows {
			return "", errors.Wrapf(err, "failed to get Post id with channelId=%s", channelID)
		}
	}

	return postID, nil
}

func (s *SQLPostStore) GetPostAfterTime(channelID string, time int64, collapsedThreads bool) (*model.Post, error) {
	table := "Posts"
	// We force MySQL to use the right index to prevent it from accidentally
	// using the index_merge_intersection optimization.
	// See MM-27575.
	if s.DriverName() == model.DatabaseDriverMysql {
		table += " USE INDEX(idx_posts_channel_id_delete_at_create_at)"
	}
	conditions := sq.And{
		sq.Gt{"CreateAt": time},
		sq.Eq{"ChannelId": channelID},
		sq.Eq{"DeleteAt": int(0)},
	}
	if collapsedThreads {
		conditions = sq.And{conditions, sq.Eq{"RootId": ""}}
	}
	query := s.getQueryBuilder().
		Select("*").
		From(table).
		Where(conditions).
		// Adding ChannelId and DeleteAt order columns
		// to let mysql choose the "idx_posts_channel_id_delete_at_create_at" index always.
		// See MM-23369.
		OrderBy("ChannelId", "DeleteAt", "CreateAt ASC").
		Limit(1)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "post_tosql")
	}

	var post *model.Post
	if err := s.GetMaster().SelectOne(&post, queryString, args...); err != nil {
		if err != sql.ErrNoRows {
			return nil, errors.Wrapf(err, "failed to get Post with channelId=%s", channelID)
		}
	}

	return post, nil
}

func (s *SQLPostStore) getRootPosts(channelID string, offset int, limit int, skipFetchThreads bool) ([]*model.Post, error) {
	var posts []*model.Post
	var fetchQuery string
	if skipFetchThreads {
		fetchQuery = "SELECT p.*, (SELECT COUNT(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN p.RootId = '' THEN p.Id ELSE p.RootId END) AND Posts.DeleteAt = 0) as ReplyCount FROM Posts p WHERE ChannelId = :ChannelId AND DeleteAt = 0 ORDER BY CreateAt DESC LIMIT :Limit OFFSET :Offset"
	} else {
		fetchQuery = "SELECT * FROM Posts WHERE ChannelId = :ChannelId AND DeleteAt = 0 ORDER BY CreateAt DESC LIMIT :Limit OFFSET :Offset"
	}
	_, err := s.GetReplica().Select(&posts, fetchQuery, map[string]interface{}{"ChannelId": channelID, "Offset": offset, "Limit": limit})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}
	return posts, nil
}

func (s *SQLPostStore) getParentsPosts(channelID string, offset int, limit int, skipFetchThreads bool) ([]*model.Post, error) {
	if s.DriverName() == model.DatabaseDriverPostgres {
		return s.getParentsPostsPostgreSQL(channelID, offset, limit, skipFetchThreads)
	}

	// query parent Ids first
	var roots []*struct {
		RootID string
	}
	rootQuery := `
		SELECT DISTINCT
			q.RootId
		FROM
			(SELECT
				RootId
			FROM
				Posts
			WHERE
				ChannelId = :ChannelId
					AND DeleteAt = 0
			ORDER BY CreateAt DESC
			LIMIT :Limit OFFSET :Offset) q
		WHERE q.RootId != ''`

	_, err := s.GetReplica().Select(&roots, rootQuery, map[string]interface{}{"ChannelId": channelID, "Offset": offset, "Limit": limit})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}
	if len(roots) == 0 {
		return nil, nil
	}
	params := make(map[string]interface{})
	placeholders := make([]string, len(roots))
	for idx, r := range roots {
		key := fmt.Sprintf(":Root%v", idx)
		params[key[1:]] = r.RootID
		placeholders[idx] = key
	}
	placeholderString := strings.Join(placeholders, ", ")
	params["ChannelId"] = channelID
	replyCountQuery := ""
	whereStatement := "p.Id IN (" + placeholderString + ")"
	if skipFetchThreads {
		replyCountQuery = `, (SELECT COUNT(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN p.RootId = '' THEN p.Id ELSE p.RootId END) AND Posts.DeleteAt = 0) as ReplyCount`
	} else {
		whereStatement += " OR p.RootId IN (" + placeholderString + ")"
	}
	var posts []*model.Post
	_, err = s.GetReplica().Select(&posts, `
		SELECT p.*`+replyCountQuery+`
		FROM
			Posts p
		WHERE
			(`+whereStatement+`)
				AND ChannelId = :ChannelId
				AND DeleteAt = 0
		ORDER BY CreateAt`,
		params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}
	return posts, nil
}

func (s *SQLPostStore) getParentsPostsPostgreSQL(channelID string, offset int, limit int, skipFetchThreads bool) ([]*model.Post, error) {
	var posts []*model.Post
	replyCountQuery := ""
	onStatement := "q1.RootId = q2.Id"
	if skipFetchThreads {
		replyCountQuery = ` ,(SELECT COUNT(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN q2.RootId = '' THEN q2.Id ELSE q2.RootId END) AND Posts.DeleteAt = 0) as ReplyCount`
	} else {
		onStatement += " OR q1.RootId = q2.RootId"
	}
	_, err := s.GetReplica().Select(&posts,
		`SELECT q2.*`+replyCountQuery+`
        FROM
            Posts q2
                INNER JOIN
            (SELECT DISTINCT
                q3.RootId
            FROM
                (SELECT
                    RootId
                FROM
                    Posts
                WHERE
                    ChannelId = :ChannelId1
                        AND DeleteAt = 0
                ORDER BY CreateAt DESC
                LIMIT :Limit OFFSET :Offset) q3
            WHERE q3.RootId != '') q1
            ON `+onStatement+`
        WHERE
            ChannelId = :ChannelId2
                AND DeleteAt = 0
        ORDER BY CreateAt`,
		map[string]interface{}{"ChannelId1": channelID, "Offset": offset, "Limit": limit, "ChannelId2": channelID})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts with channelId=%s", channelID)
	}
	return posts, nil
}

var specialSearchChar = []string{
	"<",
	">",
	"+",
	"-",
	"(",
	")",
	"~",
	"@",
	":",
}

func (s *SQLPostStore) buildCreateDateFilterClause(params *model.SearchParams, queryParams map[string]interface{}) (string, map[string]interface{}) {
	searchQuery := ""
	// handle after: before: on: filters
	if params.OnDate != "" {
		onDateStart, onDateEnd := params.GetOnDateMillis()
		queryParams["OnDateStart"] = strconv.FormatInt(onDateStart, 10)
		queryParams["OnDateEnd"] = strconv.FormatInt(onDateEnd, 10)

		// between `on date` start of day and end of day
		searchQuery += "AND CreateAt BETWEEN :OnDateStart AND :OnDateEnd "
	} else {

		if params.ExcludedDate != "" {
			excludedDateStart, excludedDateEnd := params.GetExcludedDateMillis()
			queryParams["ExcludedDateStart"] = strconv.FormatInt(excludedDateStart, 10)
			queryParams["ExcludedDateEnd"] = strconv.FormatInt(excludedDateEnd, 10)

			searchQuery += "AND CreateAt NOT BETWEEN :ExcludedDateStart AND :ExcludedDateEnd "
		}

		if params.AfterDate != "" {
			afterDate := params.GetAfterDateMillis()
			queryParams["AfterDate"] = strconv.FormatInt(afterDate, 10)

			// greater than `after date`
			searchQuery += "AND CreateAt >= :AfterDate "
		}

		if params.BeforeDate != "" {
			beforeDate := params.GetBeforeDateMillis()
			queryParams["BeforeDate"] = strconv.FormatInt(beforeDate, 10)

			// less than `before date`
			searchQuery += "AND CreateAt <= :BeforeDate "
		}

		if params.ExcludedAfterDate != "" {
			afterDate := params.GetExcludedAfterDateMillis()
			queryParams["ExcludedAfterDate"] = strconv.FormatInt(afterDate, 10)

			searchQuery += "AND CreateAt < :ExcludedAfterDate "
		}

		if params.ExcludedBeforeDate != "" {
			beforeDate := params.GetExcludedBeforeDateMillis()
			queryParams["ExcludedBeforeDate"] = strconv.FormatInt(beforeDate, 10)

			searchQuery += "AND CreateAt > :ExcludedBeforeDate "
		}
	}

	return searchQuery, queryParams
}

func (s *SQLPostStore) buildSearchChannelFilterClause(channels []string, paramPrefix string, exclusion bool, queryParams map[string]interface{}, byName bool) (string, map[string]interface{}) {
	if len(channels) == 0 {
		return "", queryParams
	}

	clauseSlice := []string{}
	for i, channel := range channels {
		paramName := paramPrefix + strconv.FormatInt(int64(i), 10)
		clauseSlice = append(clauseSlice, ":"+paramName)
		queryParams[paramName] = channel
	}
	clause := strings.Join(clauseSlice, ", ")
	if byName {
		if exclusion {
			return "AND Name NOT IN (" + clause + ")", queryParams
		}
		return "AND Name IN (" + clause + ")", queryParams
	}

	if exclusion {
		return "AND Id NOT IN (" + clause + ")", queryParams
	}
	return "AND Id IN (" + clause + ")", queryParams
}

func (s *SQLPostStore) buildSearchUserFilterClause(users []string, paramPrefix string, exclusion bool, queryParams map[string]interface{}, byUsername bool) (string, map[string]interface{}) {
	if len(users) == 0 {
		return "", queryParams
	}
	clauseSlice := []string{}
	for i, user := range users {
		paramName := paramPrefix + strconv.FormatInt(int64(i), 10)
		clauseSlice = append(clauseSlice, ":"+paramName)
		queryParams[paramName] = user
	}
	clause := strings.Join(clauseSlice, ", ")
	if byUsername {
		if exclusion {
			return "AND Username NOT IN (" + clause + ")", queryParams
		}
		return "AND Username IN (" + clause + ")", queryParams
	}
	if exclusion {
		return "AND Id NOT IN (" + clause + ")", queryParams
	}
	return "AND Id IN (" + clause + ")", queryParams
}

func (s *SQLPostStore) buildSearchPostFilterClause(fromUsers []string, excludedUsers []string, queryParams map[string]interface{}, userByUsername bool) (string, map[string]interface{}) {
	if len(fromUsers) == 0 && len(excludedUsers) == 0 {
		return "", queryParams
	}

	filterQuery := `
		AND UserId IN (
			SELECT
				Id
			FROM
				Users,
				TeamMembers
			WHERE
				TeamMembers.TeamId = :TeamId
				AND Users.Id = TeamMembers.UserId
				FROM_USER_FILTER
				EXCLUDED_USER_FILTER)`

	fromUserClause, queryParams := s.buildSearchUserFilterClause(fromUsers, "FromUser", false, queryParams, userByUsername)
	filterQuery = strings.Replace(filterQuery, "FROM_USER_FILTER", fromUserClause, 1)

	excludedUserClause, queryParams := s.buildSearchUserFilterClause(excludedUsers, "ExcludedUser", true, queryParams, userByUsername)
	filterQuery = strings.Replace(filterQuery, "EXCLUDED_USER_FILTER", excludedUserClause, 1)

	return filterQuery, queryParams
}

func (s *SQLPostStore) Search(teamID string, userID string, params *model.SearchParams) (*model.PostList, error) {
	return s.search(teamID, userID, params, true, true)
}

func (s *SQLPostStore) search(teamID string, userID string, params *model.SearchParams, channelsByName bool, userByUsername bool) (*model.PostList, error) {
	queryParams := map[string]interface{}{
		"TeamId": teamID,
		"UserId": userID,
	}

	list := model.NewPostList()
	if params.Terms == "" && params.ExcludedTerms == "" &&
		len(params.InChannels) == 0 && len(params.ExcludedChannels) == 0 &&
		len(params.FromUsers) == 0 && len(params.ExcludedUsers) == 0 &&
		params.OnDate == "" && params.AfterDate == "" && params.BeforeDate == "" {
		return list, nil
	}

	var posts []*model.Post

	deletedQueryPart := "AND DeleteAt = 0"
	if params.IncludeDeletedChannels {
		deletedQueryPart = ""
	}

	userIDPart := "AND UserId = :UserId"
	if params.SearchWithoutUserID {
		userIDPart = ""
	}

	searchQuery := `
			SELECT
				* ,(SELECT COUNT(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN q2.RootId = '' THEN q2.Id ELSE q2.RootId END) AND Posts.DeleteAt = 0) as ReplyCount
			FROM
				Posts q2
			WHERE
				DeleteAt = 0
				AND Type NOT LIKE '` + model.PostSystemMessagePrefix + `%'
				POST_FILTER
				AND ChannelId IN (
					SELECT
						Id
					FROM
						Channels,
						ChannelMembers
					WHERE
						Id = ChannelId
							AND (TeamId = :TeamId OR TeamId = '')
							` + userIDPart + `
							` + deletedQueryPart + `
							IN_CHANNEL_FILTER
							EXCLUDED_CHANNEL_FILTER)
				CREATEDATE_CLAUSE
				SEARCH_CLAUSE
				ORDER BY CreateAt DESC
			LIMIT 100`

	inChannelClause, queryParams := s.buildSearchChannelFilterClause(params.InChannels, "InChannel", false, queryParams, channelsByName)
	searchQuery = strings.Replace(searchQuery, "IN_CHANNEL_FILTER", inChannelClause, 1)

	excludedChannelClause, queryParams := s.buildSearchChannelFilterClause(params.ExcludedChannels, "ExcludedChannel", true, queryParams, channelsByName)
	searchQuery = strings.Replace(searchQuery, "EXCLUDED_CHANNEL_FILTER", excludedChannelClause, 1)

	postFilterClause, queryParams := s.buildSearchPostFilterClause(params.FromUsers, params.ExcludedUsers, queryParams, userByUsername)
	searchQuery = strings.Replace(searchQuery, "POST_FILTER", postFilterClause, 1)

	createDateFilterClause, queryParams := s.buildCreateDateFilterClause(params, queryParams)
	searchQuery = strings.Replace(searchQuery, "CREATEDATE_CLAUSE", createDateFilterClause, 1)

	termMap := map[string]bool{}
	terms := params.Terms
	excludedTerms := params.ExcludedTerms

	searchType := "Message"
	if params.IsHashtag {
		searchType = "Hashtags"
		for _, term := range strings.Split(terms, " ") {
			termMap[strings.ToUpper(term)] = true
		}
	}

	// these chars have special meaning and can be treated as spaces
	for _, c := range specialSearchChar {
		terms = strings.Replace(terms, c, " ", -1)
		excludedTerms = strings.Replace(excludedTerms, c, " ", -1)
	}

	if terms == "" && excludedTerms == "" {
		// we've already confirmed that we have a channel or user to search for
		searchQuery = strings.Replace(searchQuery, "SEARCH_CLAUSE", "", 1)
	} else if s.DriverName() == model.DatabaseDriverPostgres {
		// Parse text for wildcards
		if wildcard, err := regexp.Compile(`\*($| )`); err == nil {
			terms = wildcard.ReplaceAllLiteralString(terms, ":* ")
			excludedTerms = wildcard.ReplaceAllLiteralString(excludedTerms, ":* ")
		}

		excludeClause := ""
		if excludedTerms != "" {
			excludeClause = " & !(" + strings.Join(strings.Fields(excludedTerms), " | ") + ")"
		}

		if params.OrTerms {
			queryParams["Terms"] = "(" + strings.Join(strings.Fields(terms), " | ") + ")" + excludeClause
		} else if strings.HasPrefix(terms, `"`) && strings.HasSuffix(terms, `"`) {
			queryParams["Terms"] = "(" + strings.Join(strings.Fields(terms), " <-> ") + ")" + excludeClause
		} else {
			queryParams["Terms"] = "(" + strings.Join(strings.Fields(terms), " & ") + ")" + excludeClause
		}

		searchClause := fmt.Sprintf("AND to_tsvector('english', %s) @@  to_tsquery('english', :Terms)", searchType)
		searchQuery = strings.Replace(searchQuery, "SEARCH_CLAUSE", searchClause, 1)
	} else if s.DriverName() == model.DatabaseDriverMysql {
		if searchType == "Message" {
			var err error
			terms, err = removeMysqlStopWordsFromTerms(terms)
			if err != nil {
				return nil, errors.Wrap(err, "failed to remove Mysql stop-words from terms")
			}

			if terms == "" {
				return list, nil
			}
		}

		searchClause := fmt.Sprintf("AND MATCH (%s) AGAINST (:Terms IN BOOLEAN MODE)", searchType)
		searchQuery = strings.Replace(searchQuery, "SEARCH_CLAUSE", searchClause, 1)

		excludeClause := ""
		if excludedTerms != "" {
			excludeClause = " -(" + excludedTerms + ")"
		}

		if params.OrTerms {
			queryParams["Terms"] = terms + excludeClause
		} else {
			splitTerms := []string{}
			for _, t := range strings.Fields(terms) {
				splitTerms = append(splitTerms, "+"+t)
			}
			queryParams["Terms"] = strings.Join(splitTerms, " ") + excludeClause
		}
	}

	_, err := s.GetSearchReplica().Select(&posts, searchQuery, queryParams)
	if err != nil {
		mlog.Warn("Query error searching posts.", mlog.Err(err))
		// Don't return the error to the caller as it is of no use to the user. Instead return an empty set of search results.
	} else {
		for _, p := range posts {
			if searchType == "Hashtags" {
				exactMatch := false
				for _, tag := range strings.Split(p.Hashtags, " ") {
					if termMap[strings.ToUpper(tag)] {
						exactMatch = true
						break
					}
				}
				if !exactMatch {
					continue
				}
			}
			list.AddPost(p)
			list.AddOrder(p.ID)
		}
	}
	list.MakeNonNil()
	return list, nil
}

func removeMysqlStopWordsFromTerms(terms string) (string, error) {
	stopWords := make([]string, len(searchlayer.MySQLStopWords))
	copy(stopWords, searchlayer.MySQLStopWords)
	re, err := regexp.Compile(fmt.Sprintf(`^(%s)$`, strings.Join(stopWords, "|")))
	if err != nil {
		return "", err
	}

	newTerms := make([]string, 0)
	separatedTerms := strings.Fields(terms)
	for _, term := range separatedTerms {
		term = strings.TrimSpace(term)
		if term = re.ReplaceAllString(term, ""); term != "" {
			newTerms = append(newTerms, term)
		}
	}
	return strings.Join(newTerms, " "), nil
}

func (s *SQLPostStore) AnalyticsUserCountsWithPostsByDay(teamID string) (model.AnalyticsRows, error) {
	query :=
		`SELECT DISTINCT
		        DATE(FROM_UNIXTIME(Posts.CreateAt / 1000)) AS Name,
		        COUNT(DISTINCT Posts.UserId) AS Value
		FROM Posts`

	if teamID != "" {
		query += " INNER JOIN Channels ON Posts.ChannelId = Channels.Id AND Channels.TeamId = :TeamId AND"
	} else {
		query += " WHERE"
	}

	query += ` Posts.CreateAt >= :StartTime AND Posts.CreateAt <= :EndTime
		GROUP BY DATE(FROM_UNIXTIME(Posts.CreateAt / 1000))
		ORDER BY Name DESC
		LIMIT 30`

	if s.DriverName() == model.DatabaseDriverPostgres {
		query =
			`SELECT
				TO_CHAR(DATE(TO_TIMESTAMP(Posts.CreateAt / 1000)), 'YYYY-MM-DD') AS Name, COUNT(DISTINCT Posts.UserId) AS Value
			FROM Posts`

		if teamID != "" {
			query += " INNER JOIN Channels ON Posts.ChannelId = Channels.Id AND Channels.TeamId = :TeamId AND"
		} else {
			query += " WHERE"
		}

		query += ` Posts.CreateAt >= :StartTime AND Posts.CreateAt <= :EndTime
			GROUP BY DATE(TO_TIMESTAMP(Posts.CreateAt / 1000))
			ORDER BY Name DESC
			LIMIT 30`
	}

	end := utils.MillisFromTime(utils.EndOfDay(utils.Yesterday()))
	start := utils.MillisFromTime(utils.StartOfDay(utils.Yesterday().AddDate(0, 0, -31)))

	var rows model.AnalyticsRows
	_, err := s.GetReplica().Select(
		&rows,
		query,
		map[string]interface{}{"TeamId": teamID, "StartTime": start, "EndTime": end})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts with teamId=%s", teamID)
	}
	return rows, nil
}

func (s *SQLPostStore) AnalyticsPostCountsByDay(options *model.AnalyticsPostCountsOptions) (model.AnalyticsRows, error) {

	query :=
		`SELECT
		        DATE(FROM_UNIXTIME(Posts.CreateAt / 1000)) AS Name,
		        COUNT(Posts.Id) AS Value
		    FROM Posts`

	if options.BotsOnly {
		query += " INNER JOIN Bots ON Posts.UserId = Bots.Userid"
	}

	if options.TeamID != "" {
		query += " INNER JOIN Channels ON Posts.ChannelId = Channels.Id AND Channels.TeamId = :TeamId AND"
	} else {
		query += " WHERE"
	}

	query += ` Posts.CreateAt <= :EndTime
		            AND Posts.CreateAt >= :StartTime
		GROUP BY DATE(FROM_UNIXTIME(Posts.CreateAt / 1000))
		ORDER BY Name DESC
		LIMIT 30`

	if s.DriverName() == model.DatabaseDriverPostgres {
		query =
			`SELECT
				TO_CHAR(DATE(TO_TIMESTAMP(Posts.CreateAt / 1000)), 'YYYY-MM-DD') AS Name, Count(Posts.Id) AS Value
			FROM Posts`

		if options.BotsOnly {
			query += " INNER JOIN Bots ON Posts.UserId = Bots.Userid"
		}

		if options.TeamID != "" {
			query += " INNER JOIN Channels ON Posts.ChannelId = Channels.Id  AND Channels.TeamId = :TeamId AND"
		} else {
			query += " WHERE"
		}

		query += ` Posts.CreateAt <= :EndTime
			            AND Posts.CreateAt >= :StartTime
			GROUP BY DATE(TO_TIMESTAMP(Posts.CreateAt / 1000))
			ORDER BY Name DESC
			LIMIT 30`
	}

	end := utils.MillisFromTime(utils.EndOfDay(utils.Yesterday()))
	start := utils.MillisFromTime(utils.StartOfDay(utils.Yesterday().AddDate(0, 0, -31)))
	if options.YesterdayOnly {
		start = utils.MillisFromTime(utils.StartOfDay(utils.Yesterday().AddDate(0, 0, -1)))
	}

	var rows model.AnalyticsRows
	_, err := s.GetReplica().Select(
		&rows,
		query,
		map[string]interface{}{"TeamId": options.TeamID, "StartTime": start, "EndTime": end})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts with teamId=%s", options.TeamID)
	}
	return rows, nil
}

func (s *SQLPostStore) AnalyticsPostCount(teamID string, mustHaveFile bool, mustHaveHashtag bool) (int64, error) {
	query := s.getQueryBuilder().
		Select("COUNT(p.Id) AS Value").
		From("Posts p")

	if teamID != "" {
		query = query.
			Join("Channels c ON (c.Id = p.ChannelId)").
			Where(sq.Eq{"c.TeamId": teamID})
	}

	if mustHaveFile {
		query = query.Where(sq.Or{sq.NotEq{"p.FileIds": "[]"}, sq.NotEq{"p.Filenames": "[]"}})
	}

	if mustHaveHashtag {
		query = query.Where(sq.NotEq{"p.Hashtags": ""})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "post_tosql")
	}

	v, err := s.GetReplica().SelectInt(queryString, args...)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count Posts")
	}

	return v, nil
}

func (s *SQLPostStore) GetPostsCreatedAt(channelID string, time int64) ([]*model.Post, error) {
	query := `SELECT * FROM Posts WHERE CreateAt = :CreateAt AND ChannelId = :ChannelId`

	var posts []*model.Post
	_, err := s.GetReplica().Select(&posts, query, map[string]interface{}{"CreateAt": time, "ChannelId": channelID})

	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Posts with channelId=%s", channelID)
	}
	return posts, nil
}

func (s *SQLPostStore) GetPostsByIDs(postIDs []string) ([]*model.Post, error) {
	keys, params := MapStringsToQueryParams(postIDs, "Post")

	query := `SELECT p.*, (SELECT count(Posts.Id) FROM Posts WHERE Posts.RootId = (CASE WHEN p.RootId = '' THEN p.Id ELSE p.RootId END) AND Posts.DeleteAt = 0) as ReplyCount FROM Posts p WHERE p.Id IN ` + keys + ` ORDER BY CreateAt DESC`

	var posts []*model.Post
	_, err := s.GetReplica().Select(&posts, query, params)

	if err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}
	return posts, nil
}

func (s *SQLPostStore) GetPostsBatchForIndexing(startTime int64, endTime int64, limit int) ([]*model.PostForIndexing, error) {
	var posts []*model.PostForIndexing
	_, err := s.GetSearchReplica().Select(&posts,
		`SELECT
			PostsQuery.*, Channels.TeamId, ParentPosts.CreateAt ParentCreateAt
		FROM (
			SELECT
				*
			FROM
				Posts
			WHERE
				Posts.CreateAt >= :StartTime
			AND
				Posts.CreateAt < :EndTime
			ORDER BY
				CreateAt ASC
			LIMIT
				1000
			)
		AS
			PostsQuery
		LEFT JOIN
			Channels
		ON
			PostsQuery.ChannelId = Channels.Id
		LEFT JOIN
			Posts ParentPosts
		ON
			PostsQuery.RootId = ParentPosts.Id`,
		map[string]interface{}{"StartTime": startTime, "EndTime": endTime, "NumPosts": limit})

	if err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}
	return posts, nil
}

// PermanentDeleteBatchForRetentionPolicies deletes a batch of records which are affected by
// the global or a granular retention policy.
// See `genericPermanentDeleteBatchForRetentionPolicies` for details.
func (s *SQLPostStore) PermanentDeleteBatchForRetentionPolicies(now, globalPolicyEndTime, limit int64, cursor model.RetentionPolicyCursor) (int64, model.RetentionPolicyCursor, error) {
	builder := s.getQueryBuilder().
		Select("Posts.Id").
		From("Posts")
	return genericPermanentDeleteBatchForRetentionPolicies(RetentionPolicyBatchDeletionInfo{
		BaseBuilder:         builder,
		Table:               "Posts",
		TimeColumn:          "CreateAt",
		PrimaryKeys:         []string{"Id"},
		ChannelIDTable:      "Posts",
		NowMillis:           now,
		GlobalPolicyEndTime: globalPolicyEndTime,
		Limit:               limit,
	}, s.SQLStore, cursor)
}

// DeleteOrphanedRows removes entries from Posts when a corresponding channel no longer exists.
func (s *SQLPostStore) DeleteOrphanedRows(limit int) (deleted int64, err error) {
	// We need the extra level of nesting to deal with MySQL's locking
	const query = `
	DELETE FROM Posts WHERE Id IN (
		SELECT * FROM (
			SELECT Posts.Id FROM Posts
			LEFT JOIN Channels ON Posts.ChannelId = Channels.Id
			WHERE Channels.Id IS NULL
			LIMIT :Limit
		) AS A
	)`
	props := map[string]interface{}{"Limit": limit}
	result, err := s.GetMaster().Exec(query, props)
	if err != nil {
		return
	}
	deleted, err = result.RowsAffected()
	return
}

func (s *SQLPostStore) PermanentDeleteBatch(endTime int64, limit int64) (int64, error) {
	var query string
	if s.DriverName() == "postgres" {
		query = "DELETE from Posts WHERE Id = any (array (SELECT Id FROM Posts WHERE CreateAt < :EndTime LIMIT :Limit))"
	} else {
		query = "DELETE from Posts WHERE CreateAt < :EndTime LIMIT :Limit"
	}

	sqlResult, err := s.GetMaster().Exec(query, map[string]interface{}{"EndTime": endTime, "Limit": limit})
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete Posts")
	}

	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete Posts")
	}
	return rowsAffected, nil
}

func (s *SQLPostStore) GetOldest() (*model.Post, error) {
	var post model.Post
	err := s.GetReplica().SelectOne(&post, "SELECT * FROM Posts ORDER BY CreateAt LIMIT 1")
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Post", "none")
		}

		return nil, errors.Wrap(err, "failed to get oldest Post")
	}

	return &post, nil
}

func (s *SQLPostStore) determineMaxPostSize() int {
	var maxPostSizeBytes int32

	if s.DriverName() == model.DatabaseDriverPostgres {
		// The Post.Message column in Postgres has historically been VARCHAR(4000), but
		// may be manually enlarged to support longer posts.
		if err := s.GetReplica().SelectOne(&maxPostSizeBytes, `
			SELECT
				COALESCE(character_maximum_length, 0)
			FROM
				information_schema.columns
			WHERE
				table_name = 'posts'
			AND	column_name = 'message'
		`); err != nil {
			mlog.Warn("Unable to determine the maximum supported post size", mlog.Err(err))
		}
	} else if s.DriverName() == model.DatabaseDriverMysql {
		// The Post.Message column in MySQL has historically been TEXT, with a maximum
		// limit of 65535.
		if err := s.GetReplica().SelectOne(&maxPostSizeBytes, `
			SELECT
				COALESCE(CHARACTER_MAXIMUM_LENGTH, 0)
			FROM
				INFORMATION_SCHEMA.COLUMNS
			WHERE
				table_schema = DATABASE()
			AND	table_name = 'Posts'
			AND	column_name = 'Message'
			LIMIT 0, 1
		`); err != nil {
			mlog.Warn("Unable to determine the maximum supported post size", mlog.Err(err))
		}
	} else {
		mlog.Warn("No implementation found to determine the maximum supported post size")
	}

	// Assume a worst-case representation of four bytes per rune.
	maxPostSize := int(maxPostSizeBytes) / 4

	// To maintain backwards compatibility, don't yield a maximum post
	// size smaller than the previous limit, even though it wasn't
	// actually possible to store 4000 runes in all cases.
	if maxPostSize < model.PostMessageMaxRunesV1 {
		maxPostSize = model.PostMessageMaxRunesV1
	}

	mlog.Info("Post.Message has size restrictions", mlog.Int("max_characters", maxPostSize), mlog.Int32("max_bytes", maxPostSizeBytes))

	return maxPostSize
}

// GetMaxPostSize returns the maximum number of runes that may be stored in a post.
func (s *SQLPostStore) GetMaxPostSize() int {
	s.maxPostSizeOnce.Do(func() {
		s.maxPostSizeCached = s.determineMaxPostSize()
	})
	return s.maxPostSizeCached
}

func (s *SQLPostStore) GetParentsForExportAfter(limit int, afterID string) ([]*model.PostForExport, error) {
	for {
		var rootIDs []string
		_, err := s.GetReplica().Select(&rootIDs,
			`SELECT
				Id
			FROM
				Posts
			WHERE
				Id > :AfterId
				AND RootId = ''
				AND DeleteAt = 0
			ORDER BY Id
			LIMIT :Limit`,
			map[string]interface{}{"Limit": limit, "AfterId": afterID})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find Posts")
		}

		var postsForExport []*model.PostForExport
		if len(rootIDs) == 0 {
			return postsForExport, nil
		}

		keys, params := MapStringsToQueryParams(rootIDs, "PostId")
		_, err = s.GetSearchReplica().Select(&postsForExport, `
			SELECT
				p1.*,
				Users.Username as Username,
				Teams.Name as TeamName,
				Channels.Name as ChannelName
			FROM
				(Select * FROM Posts WHERE Id IN `+keys+`) p1
			INNER JOIN
				Channels ON p1.ChannelId = Channels.Id
			INNER JOIN
				Teams ON Channels.TeamId = Teams.Id
			INNER JOIN
				Users ON p1.UserId = Users.Id
			WHERE
				Channels.DeleteAt = 0
				AND Teams.DeleteAt = 0
			ORDER BY
				p1.Id`,
			params)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find Posts")
		}

		if len(postsForExport) == 0 {
			// All of the posts were in channels or teams that were deleted.
			// Update the afterId and try again.
			afterID = rootIDs[len(rootIDs)-1]
			continue
		}

		return postsForExport, nil
	}
}

func (s *SQLPostStore) GetRepliesForExport(rootID string) ([]*model.ReplyForExport, error) {
	var posts []*model.ReplyForExport
	_, err := s.GetSearchReplica().Select(&posts, `
			SELECT
				Posts.*,
				Users.Username as Username
			FROM
				Posts
			INNER JOIN
				Users ON Posts.UserId = Users.Id
			WHERE
				Posts.RootId = :RootId
				AND Posts.DeleteAt = 0
			ORDER BY
				Posts.Id`,
		map[string]interface{}{"RootId": rootID})

	if err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}

	return posts, nil
}

func (s *SQLPostStore) GetDirectPostParentsForExportAfter(limit int, afterID string) ([]*model.DirectPostForExport, error) {
	query := s.getQueryBuilder().
		Select("p.*", "Users.Username as User").
		From("Posts p").
		Join("Channels ON p.ChannelId = Channels.Id").
		Join("Users ON p.UserId = Users.Id").
		Where(sq.And{
			sq.Gt{"p.Id": afterID},
			sq.Eq{"p.ParentId": string("")},
			sq.Eq{"p.DeleteAt": int(0)},
			sq.Eq{"Channels.DeleteAt": int(0)},
			sq.Eq{"Users.DeleteAt": int(0)},
			sq.Eq{"Channels.Type": []string{"D", "G"}},
		}).
		OrderBy("p.Id").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "post_tosql")
	}

	var posts []*model.DirectPostForExport
	if _, err = s.GetReplica().Select(&posts, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Posts")
	}
	var channelIDs []string
	for _, post := range posts {
		channelIDs = append(channelIDs, post.ChannelID)
	}
	query = s.getQueryBuilder().
		Select("u.Username as Username, ChannelId, UserId, cm.Roles as Roles, LastViewedAt, MsgCount, MentionCount, MentionCountRoot, cm.NotifyProps as NotifyProps, LastUpdateAt, SchemeUser, SchemeAdmin, (SchemeGuest IS NOT NULL AND SchemeGuest) as SchemeGuest").
		From("ChannelMembers cm").
		Join("Users u ON ( u.Id = cm.UserId )").
		Where(sq.Eq{
			"cm.ChannelId": channelIDs,
		})

	queryString, args, err = query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "post_tosql")
	}

	var channelMembers []*model.ChannelMemberForExport
	if _, err := s.GetReplica().Select(&channelMembers, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find ChannelMembers")
	}

	// Build a map of channels and their posts
	postsChannelMap := make(map[string][]*model.DirectPostForExport)
	for _, post := range posts {
		post.ChannelMembers = &[]string{}
		postsChannelMap[post.ChannelID] = append(postsChannelMap[post.ChannelID], post)
	}

	// Build a map of channels and their members
	channelMembersMap := make(map[string][]string)
	for _, member := range channelMembers {
		channelMembersMap[member.ChannelID] = append(channelMembersMap[member.ChannelID], member.Username)
	}

	// Populate each post ChannelMembers extracting it from the channelMembersMap
	for channelID := range channelMembersMap {
		for _, post := range postsChannelMap[channelID] {
			*post.ChannelMembers = channelMembersMap[channelID]
		}
	}
	return posts, nil
}

//nolint:unparam
func (s *SQLPostStore) SearchPostsInTeamForUser(paramsList []*model.SearchParams, userID, teamID string, page, perPage int) (*model.PostSearchResults, error) {
	// Since we don't support paging for DB search, we just return nothing for later pages
	if page > 0 {
		return model.MakePostSearchResults(model.NewPostList(), nil), nil
	}

	if err := model.IsSearchParamsListValid(paramsList); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup

	pchan := make(chan store.StoreResult, len(paramsList))

	for _, params := range paramsList {
		// remove any unquoted term that contains only non-alphanumeric chars
		// ex: abcd "**" && abc     >>     abcd "**" abc
		params.Terms = removeNonAlphaNumericUnquotedTerms(params.Terms, " ")

		wg.Add(1)

		go func(params *model.SearchParams) {
			defer wg.Done()
			postList, err := s.search(teamID, userID, params, false, false)
			pchan <- store.StoreResult{Data: postList, NErr: err}
		}(params)
	}

	wg.Wait()
	close(pchan)

	posts := model.NewPostList()

	for result := range pchan {
		if result.NErr != nil {
			return nil, result.NErr
		}
		data := result.Data.(*model.PostList)
		posts.Extend(data)
	}

	posts.SortByCreateAt()

	return model.MakePostSearchResults(posts, nil), nil
}

func (s *SQLPostStore) GetOldestEntityCreationTime() (int64, error) {
	query := s.getQueryBuilder().Select("MIN(min_createat) min_createat").
		Suffix(`FROM (
					(SELECT MIN(createat) min_createat FROM Posts)
					UNION
					(SELECT MIN(createat) min_createat FROM Users)
					UNION
					(SELECT MIN(createat) min_createat FROM Channels)
				) entities`)
	queryString, _, err := query.ToSql()
	if err != nil {
		return -1, errors.Wrap(err, "post_tosql")
	}
	row := s.GetReplica().Db.QueryRow(queryString)
	var oldest int64
	if err := row.Scan(&oldest); err != nil {
		return -1, errors.Wrap(err, "unable to scan oldest entity creation time")
	}
	return oldest, nil
}

func (s *SQLPostStore) cleanupThreads(postID, rootID string, permanent bool) error {
	if permanent {
		if _, err := s.GetMaster().Exec("DELETE FROM Threads WHERE PostId = :Id", map[string]interface{}{"Id": postID}); err != nil {
			return errors.Wrap(err, "failed to delete Threads")
		}
		if _, err := s.GetMaster().Exec("DELETE FROM ThreadMemberships WHERE PostId = :Id", map[string]interface{}{"Id": postID}); err != nil {
			return errors.Wrap(err, "failed to delete ThreadMemberships")
		}
		return nil
	}
	if rootID != "" {
		thread, err := s.Thread().Get(rootID)
		if err != nil {
			var nfErr *store.ErrNotFound
			if !errors.As(err, &nfErr) {
				return errors.Wrap(err, "failed to get a thread")
			}
		}
		if thread != nil {
			thread.ReplyCount -= 1
			if _, err = s.Thread().Update(thread); err != nil {
				return errors.Wrap(err, "failed to update thread")
			}
		}
	}
	return nil
}

func (s *SQLPostStore) updateThreadsFromPosts(transaction *gorp.Transaction, posts []*model.Post) error {
	postsByRoot := map[string][]*model.Post{}
	var rootIDs []string
	for _, post := range posts {
		// skip if post is not a part of a thread
		if post.RootID == "" {
			continue
		}
		rootIDs = append(rootIDs, post.RootID)
		postsByRoot[post.RootID] = append(postsByRoot[post.RootID], post)
	}
	if len(rootIDs) == 0 {
		return nil
	}
	now := model.GetMillis()
	threadsByRootsSQL, threadsByRootsArgs, _ := s.getQueryBuilder().Select("*").From("Threads").Where(sq.Eq{"PostId": rootIDs}).ToSql()
	var threadsByRoots []*model.Thread
	if _, err := transaction.Select(&threadsByRoots, threadsByRootsSQL, threadsByRootsArgs...); err != nil {
		return err
	}

	threadByRoot := map[string]*model.Thread{}
	for _, thread := range threadsByRoots {
		threadByRoot[thread.PostID] = thread
	}

	for rootID, posts := range postsByRoot {
		if thread, found := threadByRoot[rootID]; !found {
			// calculate participants
			var participants model.StringArray
			if _, err := transaction.Select(&participants, "SELECT DISTINCT UserId FROM Posts WHERE RootId=:RootId OR Id=:RootId", map[string]interface{}{"RootId": rootID}); err != nil {
				return err
			}
			// calculate reply count
			count, err := transaction.SelectInt("SELECT COUNT(Id) FROM Posts WHERE RootId=:RootId And DeleteAt=0", map[string]interface{}{"RootId": rootID})
			if err != nil {
				return err
			}
			// no metadata entry, create one
			if err := transaction.Insert(&model.Thread{
				PostID:       rootID,
				ChannelID:    posts[0].ChannelID,
				ReplyCount:   count,
				LastReplyAt:  now,
				Participants: participants,
			}); err != nil {
				return err
			}
		} else {
			// metadata exists, update it
			thread.LastReplyAt = now
			for _, post := range posts {
				thread.ReplyCount += 1
				if !thread.Participants.Contains(post.UserID) {
					thread.Participants = append(thread.Participants, post.UserID)
				}
			}
			if _, err := transaction.Update(thread); err != nil {
				return err
			}
		}
	}
	return nil
}
