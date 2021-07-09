// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SqlFileInfoStore struct {
	*SqlStore
	metrics     einterfaces.MetricsInterface
	queryFields []string
}

func (fs SqlFileInfoStore) ClearCaches() {
}

func newSqlFileInfoStore(sqlStore *SqlStore, metrics einterfaces.MetricsInterface) store.FileInfoStore {
	s := &SqlFileInfoStore{
		SqlStore: sqlStore,
		metrics:  metrics,
	}

	s.queryFields = []string{
		"FileInfo.Id",
		"FileInfo.CreatorId",
		"FileInfo.PostId",
		"FileInfo.CreateAt",
		"FileInfo.UpdateAt",
		"FileInfo.DeleteAt",
		"FileInfo.Path",
		"FileInfo.ThumbnailPath",
		"FileInfo.PreviewPath",
		"FileInfo.Name",
		"FileInfo.Extension",
		"FileInfo.Size",
		"FileInfo.MimeType",
		"FileInfo.Width",
		"FileInfo.Height",
		"FileInfo.HasPreviewImage",
		"FileInfo.MiniPreview",
		"Coalesce(FileInfo.Content, '') AS Content",
		"Coalesce(FileInfo.RemoteId, '') AS RemoteId",
	}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.FileInfo{}, "FileInfo").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("CreatorId").SetMaxSize(26)
		table.ColMap("PostId").SetMaxSize(26)
		table.ColMap("Path").SetMaxSize(512)
		table.ColMap("ThumbnailPath").SetMaxSize(512)
		table.ColMap("PreviewPath").SetMaxSize(512)
		table.ColMap("Name").SetMaxSize(256)
		table.ColMap("Content").SetMaxSize(0)
		table.ColMap("Extension").SetMaxSize(64)
		table.ColMap("MimeType").SetMaxSize(256)
		table.ColMap("RemoteId").SetMaxSize(26)
	}

	return s
}

func (fs SqlFileInfoStore) createIndexesIfNotExists() {
	fs.CreateIndexIfNotExists("idx_fileinfo_update_at", "FileInfo", "UpdateAt")
	fs.CreateIndexIfNotExists("idx_fileinfo_create_at", "FileInfo", "CreateAt")
	fs.CreateIndexIfNotExists("idx_fileinfo_delete_at", "FileInfo", "DeleteAt")
	fs.CreateIndexIfNotExists("idx_fileinfo_postid_at", "FileInfo", "PostId")
	fs.CreateIndexIfNotExists("idx_fileinfo_extension_at", "FileInfo", "Extension")
	fs.CreateFullTextIndexIfNotExists("idx_fileinfo_name_txt", "FileInfo", "Name")
	if fs.DriverName() == model.DatabaseDriverPostgres {
		fs.CreateFullTextFuncIndexIfNotExists("idx_fileinfo_name_splitted", "FileInfo", "Translate(Name, '.,-', '   ')")
	}
	fs.CreateFullTextIndexIfNotExists("idx_fileinfo_content_txt", "FileInfo", "Content")
}

func (fs SqlFileInfoStore) Save(info *model.FileInfo) (*model.FileInfo, error) {
	info.PreSave()
	if err := info.IsValid(); err != nil {
		return nil, err
	}

	if err := fs.GetMaster().Insert(info); err != nil {
		return nil, errors.Wrap(err, "failed to save FileInfo")
	}
	return info, nil
}

func (fs SqlFileInfoStore) GetByIDs(ids []string) ([]*model.FileInfo, error) {
	query := fs.getQueryBuilder().
		Select(append(fs.queryFields, "COALESCE(P.ChannelId, '') as ChannelId")...).
		From("FileInfo").
		LeftJoin("Posts as P ON FileInfo.PostId=P.Id").
		Where(sq.Eq{"FileInfo.Id": ids}).
		Where(sq.Eq{"FileInfo.DeleteAt": 0}).
		OrderBy("FileInfo.CreateAt DESC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "file_info_tosql")
	}

	var infos []*model.FileInfo
	if _, err := fs.GetReplica().Select(&infos, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find FileInfos")
	}
	return infos, nil
}

func (fs SqlFileInfoStore) Upsert(info *model.FileInfo) (*model.FileInfo, error) {
	info.PreSave()
	if err := info.IsValid(); err != nil {
		return nil, err
	}

	n, err := fs.GetMaster().Update(info)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update FileInfo")
	}
	if n == 0 {
		if err = fs.GetMaster().Insert(info); err != nil {
			return nil, errors.Wrap(err, "failed to save FileInfo")
		}
	}
	return info, nil
}

func (fs SqlFileInfoStore) get(id string, fromMaster bool) (*model.FileInfo, error) {
	info := &model.FileInfo{}

	query := fs.getQueryBuilder().
		Select(fs.queryFields...).
		From("FileInfo").
		Where(sq.Eq{"Id": id}).
		Where(sq.Eq{"DeleteAt": 0})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "file_info_tosql")
	}

	db := fs.GetReplica()
	if fromMaster {
		db = fs.GetMaster()
	}

	if err := db.SelectOne(info, queryString, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("FileInfo", id)
		}
		return nil, errors.Wrapf(err, "failed to get FileInfo with id=%s", id)
	}
	return info, nil
}

func (fs SqlFileInfoStore) Get(id string) (*model.FileInfo, error) {
	return fs.get(id, false)
}

func (fs SqlFileInfoStore) GetFromMaster(id string) (*model.FileInfo, error) {
	return fs.get(id, true)
}

func (fs SqlFileInfoStore) GetWithOptions(page, perPage int, opt *model.GetFileInfosOptions) ([]*model.FileInfo, error) {
	if perPage < 0 {
		return nil, store.NewErrLimitExceeded("perPage", perPage, "value used in pagination while getting FileInfos")
	} else if page < 0 {
		return nil, store.NewErrLimitExceeded("page", page, "value used in pagination while getting FileInfos")
	}
	if perPage == 0 {
		return nil, nil
	}

	if opt == nil {
		opt = &model.GetFileInfosOptions{}
	}

	query := fs.getQueryBuilder().
		Select(fs.queryFields...).
		From("FileInfo")

	if len(opt.ChannelIDs) > 0 {
		query = query.Join("Posts ON FileInfo.PostId = Posts.Id").
			Where(sq.Eq{"Posts.ChannelId": opt.ChannelIDs})
	}

	if len(opt.UserIDs) > 0 {
		query = query.Where(sq.Eq{"FileInfo.CreatorId": opt.UserIDs})
	}

	if opt.Since > 0 {
		query = query.Where(sq.GtOrEq{"FileInfo.CreateAt": opt.Since})
	}

	if !opt.IncludeDeleted {
		query = query.Where("FileInfo.DeleteAt = 0")
	}

	if opt.SortBy == "" {
		opt.SortBy = model.FileinfoSortByCreated
	}
	sortDirection := "ASC"
	if opt.SortDescending {
		sortDirection = "DESC"
	}

	switch opt.SortBy {
	case model.FileinfoSortByCreated:
		query = query.OrderBy("FileInfo.CreateAt " + sortDirection)
	case model.FileinfoSortBySize:
		query = query.OrderBy("FileInfo.Size " + sortDirection)
	default:
		return nil, store.NewErrInvalidInput("FileInfo", "<sortOption>", opt.SortBy)
	}

	query = query.OrderBy("FileInfo.Id ASC") // secondary sort for sort stability

	query = query.Limit(uint64(perPage)).Offset(uint64(perPage * page))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "file_info_tosql")
	}
	var infos []*model.FileInfo
	if _, err := fs.GetReplica().Select(&infos, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find FileInfos")
	}
	return infos, nil
}

func (fs SqlFileInfoStore) GetByPath(path string) (*model.FileInfo, error) {
	info := &model.FileInfo{}

	query := fs.getQueryBuilder().
		Select(fs.queryFields...).
		From("FileInfo").
		Where(sq.Eq{"Path": path}).
		Where(sq.Eq{"DeleteAt": 0}).
		Limit(1)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "file_info_tosql")
	}

	if err := fs.GetReplica().SelectOne(info, queryString, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("FileInfo", fmt.Sprintf("path=%s", path))
		}

		return nil, errors.Wrapf(err, "failed to get FileInfo with path=%s", path)
	}
	return info, nil
}

func (fs SqlFileInfoStore) InvalidateFileInfosForPostCache(postID string, deleted bool) {
}

func (fs SqlFileInfoStore) GetForPost(postID string, readFromMaster, includeDeleted, allowFromCache bool) ([]*model.FileInfo, error) {
	var infos []*model.FileInfo

	dbmap := fs.GetReplica()

	if readFromMaster {
		dbmap = fs.GetMaster()
	}

	query := fs.getQueryBuilder().
		Select(fs.queryFields...).
		From("FileInfo").
		Where(sq.Eq{"PostId": postID}).
		OrderBy("CreateAt")

	if !includeDeleted {
		query = query.Where("DeleteAt = 0")
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "file_info_tosql")
	}

	if _, err := dbmap.Select(&infos, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find FileInfos with postId=%s", postID)
	}
	return infos, nil
}

func (fs SqlFileInfoStore) GetForUser(userID string) ([]*model.FileInfo, error) {
	var infos []*model.FileInfo

	dbmap := fs.GetReplica()

	query := fs.getQueryBuilder().
		Select(fs.queryFields...).
		From("FileInfo").
		Where(sq.Eq{"CreatorId": userID}).
		Where(sq.Eq{"DeleteAt": 0}).
		OrderBy("CreateAt")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "file_info_tosql")
	}

	if _, err := dbmap.Select(&infos, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find FileInfos with creatorId=%s", userID)
	}
	return infos, nil
}

func (fs SqlFileInfoStore) AttachToPost(fileID, postID, creatorID string) error {
	sqlResult, err := fs.GetMaster().Exec(`
		UPDATE
			FileInfo
		SET
			PostId = :PostId
		WHERE
			Id = :Id
			AND PostId = ''
			AND (CreatorId = :CreatorId OR CreatorId = 'nouser')
	`, map[string]interface{}{
		"PostId":    postID,
		"Id":        fileID,
		"CreatorId": creatorID,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to update FileInfo with id=%s and postId=%s", fileID, postID)
	}

	count, err := sqlResult.RowsAffected()
	if err != nil {
		// RowsAffected should never fail with the MySQL or Postgres drivers
		return errors.Wrap(err, "unable to retrieve rows affected")
	} else if count == 0 {
		// Could not attach the file to the post
		return store.NewErrInvalidInput("FileInfo", "<id, postId, creatorId>", fmt.Sprintf("<%s, %s, %s>", fileID, postID, creatorID))
	}
	return nil
}

func (fs SqlFileInfoStore) SetContent(fileID, content string) error {
	query := fs.getQueryBuilder().
		Update("FileInfo").
		Set("Content", content).
		Where(sq.Eq{"Id": fileID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "file_info_tosql")
	}

	_, err = fs.GetMaster().Exec(queryString, args...)
	if err != nil {
		return errors.Wrapf(err, "failed to update FileInfo content with id=%s", fileID)
	}

	return nil
}

func (fs SqlFileInfoStore) DeleteForPost(postID string) (string, error) {
	if _, err := fs.GetMaster().Exec(
		`UPDATE
				FileInfo
			SET
				DeleteAt = :DeleteAt
			WHERE
				PostId = :PostId`, map[string]interface{}{"DeleteAt": model.GetMillis(), "PostId": postID}); err != nil {
		return "", errors.Wrapf(err, "failed to update FileInfo with postId=%s", postID)
	}
	return postID, nil
}

func (fs SqlFileInfoStore) PermanentDelete(fileID string) error {
	if _, err := fs.GetMaster().Exec(
		`DELETE FROM
				FileInfo
			WHERE
				Id = :FileId`, map[string]interface{}{"FileId": fileID}); err != nil {
		return errors.Wrapf(err, "failed to delete FileInfo with id=%s", fileID)
	}
	return nil
}

func (fs SqlFileInfoStore) PermanentDeleteBatch(endTime int64, limit int64) (int64, error) {
	var query string
	if fs.DriverName() == "postgres" {
		query = "DELETE from FileInfo WHERE Id = any (array (SELECT Id FROM FileInfo WHERE CreateAt < :EndTime LIMIT :Limit))"
	} else {
		query = "DELETE from FileInfo WHERE CreateAt < :EndTime LIMIT :Limit"
	}

	sqlResult, err := fs.GetMaster().Exec(query, map[string]interface{}{"EndTime": endTime, "Limit": limit})
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete FileInfos in batch")
	}

	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return 0, errors.Wrapf(err, "unable to retrieve rows affected")
	}

	return rowsAffected, nil
}

func (fs SqlFileInfoStore) PermanentDeleteByUser(userID string) (int64, error) {
	query := "DELETE from FileInfo WHERE CreatorId = :CreatorId"

	sqlResult, err := fs.GetMaster().Exec(query, map[string]interface{}{"CreatorId": userID})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to delete FileInfo with creatorId=%s", userID)
	}

	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return 0, errors.Wrapf(err, "unable to retrieve rows affected")
	}

	return rowsAffected, nil
}

func (fs SqlFileInfoStore) Search(paramsList []*model.SearchParams, userID, teamID string, page, perPage int) (*model.FileInfoList, error) {
	// Since we don't support paging for DB search, we just return nothing for later pages
	if page > 0 {
		return model.NewFileInfoList(), nil
	}
	if err := model.IsSearchParamsListValid(paramsList); err != nil {
		return nil, err
	}
	query := fs.getQueryBuilder().
		Select(append(fs.queryFields, "Coalesce(P.ChannelId, '') AS ChannelId")...).
		From("FileInfo").
		LeftJoin("Posts as P ON FileInfo.PostId=P.Id").
		LeftJoin("Channels as C ON C.Id=P.ChannelId").
		LeftJoin("ChannelMembers as CM ON C.Id=CM.ChannelId").
		Where(sq.Or{sq.Eq{"C.TeamId": teamID}, sq.Eq{"C.TeamId": ""}}).
		Where(sq.Eq{"FileInfo.DeleteAt": 0}).
		OrderBy("FileInfo.CreateAt DESC").
		Limit(100)

	for _, params := range paramsList {
		params.Terms = removeNonAlphaNumericUnquotedTerms(params.Terms, " ")

		if !params.IncludeDeletedChannels {
			query = query.Where(sq.Eq{"C.DeleteAt": 0})
		}

		if !params.SearchWithoutUserID {
			query = query.Where(sq.Eq{"CM.UserId": userID})
		}

		if len(params.InChannels) != 0 {
			query = query.Where(sq.Eq{"C.Id": params.InChannels})
		}

		if len(params.Extensions) != 0 {
			query = query.Where(sq.Eq{"FileInfo.Extension": params.Extensions})
		}

		if len(params.ExcludedExtensions) != 0 {
			query = query.Where(sq.NotEq{"FileInfo.Extension": params.ExcludedExtensions})
		}

		if len(params.ExcludedChannels) != 0 {
			query = query.Where(sq.NotEq{"C.Id": params.ExcludedChannels})
		}

		if len(params.FromUsers) != 0 {
			query = query.Where(sq.Eq{"FileInfo.CreatorId": params.FromUsers})
		}

		if len(params.ExcludedUsers) != 0 {
			query = query.Where(sq.NotEq{"FileInfo.CreatorId": params.ExcludedUsers})
		}

		// handle after: before: on: filters
		if params.OnDate != "" {
			onDateStart, onDateEnd := params.GetOnDateMillis()
			query = query.Where(sq.Expr("FileInfo.CreateAt BETWEEN ? AND ?", strconv.FormatInt(onDateStart, 10), strconv.FormatInt(onDateEnd, 10)))
		} else {
			if params.ExcludedDate != "" {
				excludedDateStart, excludedDateEnd := params.GetExcludedDateMillis()
				query = query.Where(sq.Expr("FileInfo.CreateAt NOT BETWEEN ? AND ?", strconv.FormatInt(excludedDateStart, 10), strconv.FormatInt(excludedDateEnd, 10)))
			}

			if params.AfterDate != "" {
				afterDate := params.GetAfterDateMillis()
				query = query.Where(sq.GtOrEq{"FileInfo.CreateAt": strconv.FormatInt(afterDate, 10)})
			}

			if params.BeforeDate != "" {
				beforeDate := params.GetBeforeDateMillis()
				query = query.Where(sq.LtOrEq{"FileInfo.CreateAt": strconv.FormatInt(beforeDate, 10)})
			}

			if params.ExcludedAfterDate != "" {
				afterDate := params.GetExcludedAfterDateMillis()
				query = query.Where(sq.Lt{"FileInfo.CreateAt": strconv.FormatInt(afterDate, 10)})
			}

			if params.ExcludedBeforeDate != "" {
				beforeDate := params.GetExcludedBeforeDateMillis()
				query = query.Where(sq.Gt{"FileInfo.CreateAt": strconv.FormatInt(beforeDate, 10)})
			}
		}

		terms := params.Terms
		excludedTerms := params.ExcludedTerms

		// these chars have special meaning and can be treated as spaces
		for _, c := range specialSearchChar {
			terms = strings.Replace(terms, c, " ", -1)
			excludedTerms = strings.Replace(excludedTerms, c, " ", -1)
		}

		if terms == "" && excludedTerms == "" {
			// we've already confirmed that we have a channel or user to search for
		} else if fs.DriverName() == model.DatabaseDriverPostgres {
			// Parse text for wildcards
			if wildcard, err := regexp.Compile(`\*($| )`); err == nil {
				terms = wildcard.ReplaceAllLiteralString(terms, ":* ")
				excludedTerms = wildcard.ReplaceAllLiteralString(excludedTerms, ":* ")
			}

			excludeClause := ""
			if excludedTerms != "" {
				excludeClause = " & !(" + strings.Join(strings.Fields(excludedTerms), " | ") + ")"
			}

			queryTerms := ""
			if params.OrTerms {
				queryTerms = "(" + strings.Join(strings.Fields(terms), " | ") + ")" + excludeClause
			} else {
				queryTerms = "(" + strings.Join(strings.Fields(terms), " & ") + ")" + excludeClause
			}

			query = query.Where(sq.Or{
				sq.Expr("to_tsvector('english', FileInfo.Name) @@  to_tsquery('english', ?)", queryTerms),
				sq.Expr("to_tsvector('english', Translate(FileInfo.Name, '.,-', '   ')) @@  to_tsquery('english', ?)", queryTerms),
				sq.Expr("to_tsvector('english', FileInfo.Content) @@  to_tsquery('english', ?)", queryTerms),
			})
		} else if fs.DriverName() == model.DatabaseDriverMysql {
			var err error
			terms, err = removeMysqlStopWordsFromTerms(terms)
			if err != nil {
				return nil, errors.Wrap(err, "failed to remove Mysql stop-words from terms")
			}

			if terms == "" {
				return model.NewFileInfoList(), nil
			}

			excludeClause := ""
			if excludedTerms != "" {
				excludeClause = " -(" + excludedTerms + ")"
			}

			queryTerms := ""
			if params.OrTerms {
				queryTerms = terms + excludeClause
			} else {
				splitTerms := []string{}
				for _, t := range strings.Fields(terms) {
					splitTerms = append(splitTerms, "+"+t)
				}
				queryTerms = strings.Join(splitTerms, " ") + excludeClause
			}
			query = query.Where(sq.Or{
				sq.Expr("MATCH (FileInfo.Name) AGAINST (? IN BOOLEAN MODE)", queryTerms),
				sq.Expr("MATCH (FileInfo.Content) AGAINST (? IN BOOLEAN MODE)", queryTerms),
			})
		}
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "file_info_tosql")
	}

	list := model.NewFileInfoList()
	fileInfos := []*model.FileInfo{}
	_, err = fs.GetSearchReplica().Select(&fileInfos, queryString, args...)
	if err != nil {
		mlog.Warn("Query error searching files.", mlog.Err(err))
		// Don't return the error to the caller as it is of no use to the user. Instead return an empty set of search results.
	} else {
		for _, f := range fileInfos {
			list.AddFileInfo(f)
			list.AddOrder(f.ID)
		}
	}
	list.MakeNonNil()
	return list, nil
}

func (fs SqlFileInfoStore) CountAll() (int64, error) {
	query := fs.getQueryBuilder().
		Select("COUNT(*)").
		From("FileInfo").
		Where("DeleteAt = 0")

	queryString, args, err := query.ToSql()
	if err != nil {
		return int64(0), errors.Wrap(err, "count_tosql")
	}

	count, err := fs.GetReplica().SelectInt(queryString, args...)
	if err != nil {
		return int64(0), errors.Wrap(err, "failed to count Files")
	}
	return count, nil
}

func (fs SqlFileInfoStore) GetFilesBatchForIndexing(startTime, endTime int64, limit int) ([]*model.FileForIndexing, error) {
	var files []*model.FileForIndexing
	sql, args, _ := fs.getQueryBuilder().
		Select(append(fs.queryFields, "Coalesce(p.ChannelId, '') AS ChannelId")...).
		From("FileInfo").
		LeftJoin("Posts AS p ON FileInfo.PostId = p.Id").
		Where(sq.GtOrEq{"FileInfo.CreateAt": startTime}).
		Where(sq.Lt{"FileInfo.CreateAt": endTime}).
		OrderBy("FileInfo.CreateAt").
		Limit(uint64(limit)).
		ToSql()
	_, err := fs.GetSearchReplica().Select(&files, sql, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Files")
	}

	return files, nil
}
