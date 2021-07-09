// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SQLComplianceStore struct {
	*SQLStore
}

func newSQLComplianceStore(sqlStore *SQLStore) store.ComplianceStore {
	s := &SQLComplianceStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Compliance{}, "Compliances").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("Status").SetMaxSize(64)
		table.ColMap("Desc").SetMaxSize(512)
		table.ColMap("Type").SetMaxSize(64)
		table.ColMap("Keywords").SetMaxSize(512)
		table.ColMap("Emails").SetMaxSize(1024)
	}

	return s
}

func (s SQLComplianceStore) createIndexesIfNotExists() {
}

func (s SQLComplianceStore) Save(compliance *model.Compliance) (*model.Compliance, error) {
	compliance.PreSave()
	if err := compliance.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(compliance); err != nil {
		return nil, errors.Wrap(err, "failed to save Compliance")
	}
	return compliance, nil
}

func (s SQLComplianceStore) Update(compliance *model.Compliance) (*model.Compliance, error) {
	if err := compliance.IsValid(); err != nil {
		return nil, err
	}

	if _, err := s.GetMaster().Update(compliance); err != nil {
		return nil, errors.Wrap(err, "failed to update Compliance")
	}
	return compliance, nil
}

func (s SQLComplianceStore) GetAll(offset, limit int) (model.Compliances, error) {
	query := "SELECT * FROM Compliances ORDER BY CreateAt DESC LIMIT :Limit OFFSET :Offset"

	var compliances model.Compliances
	if _, err := s.GetReplica().Select(&compliances, query, map[string]interface{}{"Offset": offset, "Limit": limit}); err != nil {
		return nil, errors.Wrap(err, "failed to find all Compliances")
	}
	return compliances, nil
}

func (s SQLComplianceStore) Get(id string) (*model.Compliance, error) {
	obj, err := s.GetReplica().Get(model.Compliance{}, id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get Compliance with id=%s", id)
	}
	if obj == nil {
		return nil, store.NewErrNotFound("Compliance", id)
	}
	return obj.(*model.Compliance), nil
}

func (s SQLComplianceStore) ComplianceExport(job *model.Compliance, cursor model.ComplianceExportCursor, limit int) ([]*model.CompliancePost, model.ComplianceExportCursor, error) {
	props := map[string]interface{}{"EndTime": job.EndAt, "Limit": limit}

	keywordQuery := ""
	keywords := strings.Fields(strings.TrimSpace(strings.ToLower(strings.Replace(job.Keywords, ",", " ", -1))))
	if len(keywords) > 0 {
		clauses := make([]string, len(keywords))

		for i, keyword := range keywords {
			keyword = sanitizeSearchTerm(keyword, "\\")
			clauses[i] = "LOWER(Posts.Message) LIKE :Keyword" + strconv.Itoa(i)
			props["Keyword"+strconv.Itoa(i)] = "%" + keyword + "%"
		}

		keywordQuery = "AND (" + strings.Join(clauses, " OR ") + ")"
	}

	emailQuery := ""
	emails := strings.Fields(strings.TrimSpace(strings.ToLower(strings.Replace(job.Emails, ",", " ", -1))))
	if len(emails) > 0 {
		clauses := make([]string, len(emails))

		for i, email := range emails {
			clauses[i] = "Users.Email = :Email" + strconv.Itoa(i)
			props["Email"+strconv.Itoa(i)] = email
		}

		emailQuery = "AND (" + strings.Join(clauses, " OR ") + ")"
	}

	// The idea is to first iterate over the channel posts, and then when we run out of those,
	// start iterating over the direct message posts.

	var channelPosts []*model.CompliancePost
	channelsQuery := ""
	if !cursor.ChannelsQueryCompleted {
		if cursor.LastChannelsQueryPostCreateAt == 0 {
			cursor.LastChannelsQueryPostCreateAt = job.StartAt
		}
		props["LastPostCreateAt"] = cursor.LastChannelsQueryPostCreateAt
		props["LastPostId"] = cursor.LastChannelsQueryPostID
		channelsQuery = `
		SELECT
			Teams.Name AS TeamName,
			Teams.DisplayName AS TeamDisplayName,
			Channels.Name AS ChannelName,
			Channels.DisplayName AS ChannelDisplayName,
			Channels.Type AS ChannelType,
			Users.Username AS UserUsername,
			Users.Email AS UserEmail,
			Users.Nickname AS UserNickname,
			Posts.Id AS PostId,
			Posts.CreateAt AS PostCreateAt,
			Posts.UpdateAt AS PostUpdateAt,
			Posts.DeleteAt AS PostDeleteAt,
			Posts.RootId AS PostRootId,
			Posts.ParentId AS PostParentId,
			Posts.OriginalId AS PostOriginalId,
			Posts.Message AS PostMessage,
			Posts.Type AS PostType,
			Posts.Props AS PostProps,
			Posts.Hashtags AS PostHashtags,
			Posts.FileIds AS PostFileIds,
			Bots.UserId IS NOT NULL AS IsBot
		FROM
			Teams,
			Channels,
			Users,
			Posts
		LEFT JOIN
			Bots ON Bots.UserId = Posts.UserId
		WHERE
			Teams.Id = Channels.TeamId
				AND Posts.ChannelId = Channels.Id
				AND Posts.UserId = Users.Id
				AND (
					Posts.CreateAt > :LastPostCreateAt
					OR (Posts.CreateAt = :LastPostCreateAt AND Posts.Id > :LastPostId)
				)
				AND Posts.CreateAt < :EndTime
				` + emailQuery + `
				` + keywordQuery + `
		ORDER BY Posts.CreateAt, Posts.Id
		LIMIT :Limit`
		if _, err := s.GetReplica().Select(&channelPosts, channelsQuery, props); err != nil {
			return nil, cursor, errors.Wrap(err, "unable to export compliance")
		}
		if len(channelPosts) < limit {
			cursor.ChannelsQueryCompleted = true
		} else {
			cursor.LastChannelsQueryPostCreateAt = channelPosts[len(channelPosts)-1].PostCreateAt
			cursor.LastChannelsQueryPostID = channelPosts[len(channelPosts)-1].PostID
		}
	}

	var directMessagePosts []*model.CompliancePost
	directMessagesQuery := ""
	if !cursor.DirectMessagesQueryCompleted && len(channelPosts) < limit {
		if cursor.LastDirectMessagesQueryPostCreateAt == 0 {
			cursor.LastDirectMessagesQueryPostCreateAt = job.StartAt
		}
		props["LastPostCreateAt"] = cursor.LastDirectMessagesQueryPostCreateAt
		props["LastPostId"] = cursor.LastDirectMessagesQueryPostID
		props["Limit"] = limit - len(channelPosts)
		directMessagesQuery = `
		SELECT
			'direct-messages' AS TeamName,
			'Direct Messages' AS TeamDisplayName,
			Channels.Name AS ChannelName,
			Channels.DisplayName AS ChannelDisplayName,
			Channels.Type AS ChannelType,
			Users.Username AS UserUsername,
			Users.Email AS UserEmail,
			Users.Nickname AS UserNickname,
			Posts.Id AS PostId,
			Posts.CreateAt AS PostCreateAt,
			Posts.UpdateAt AS PostUpdateAt,
			Posts.DeleteAt AS PostDeleteAt,
			Posts.RootId AS PostRootId,
			Posts.ParentId AS PostParentId,
			Posts.OriginalId AS PostOriginalId,
			Posts.Message AS PostMessage,
			Posts.Type AS PostType,
			Posts.Props AS PostProps,
			Posts.Hashtags AS PostHashtags,
			Posts.FileIds AS PostFileIds,
			Bots.UserId IS NOT NULL AS IsBot
		FROM
			Channels,
			Users,
			Posts
		LEFT JOIN
			Bots ON Bots.UserId = Posts.UserId
		WHERE
			Channels.TeamId = ''
				AND Posts.ChannelId = Channels.Id
				AND Posts.UserId = Users.Id
				AND (
					Posts.CreateAt > :LastPostCreateAt
					OR (Posts.CreateAt = :LastPostCreateAt AND Posts.Id > :LastPostId)
				)
				AND Posts.CreateAt < :EndTime
				` + emailQuery + `
				` + keywordQuery + `
		ORDER BY Posts.CreateAt, Posts.Id
		LIMIT :Limit`

		if _, err := s.GetReplica().Select(&directMessagePosts, directMessagesQuery, props); err != nil {
			return nil, cursor, errors.Wrap(err, "unable to export compliance")
		}
		if len(directMessagePosts) < limit {
			cursor.DirectMessagesQueryCompleted = true
		} else {
			cursor.LastDirectMessagesQueryPostCreateAt = directMessagePosts[len(directMessagePosts)-1].PostCreateAt
			cursor.LastDirectMessagesQueryPostID = directMessagePosts[len(directMessagePosts)-1].PostID
		}
	}

	return append(channelPosts, directMessagePosts...), cursor, nil
}

func (s SQLComplianceStore) MessageExport(cursor model.MessageExportCursor, limit int) ([]*model.MessageExport, model.MessageExportCursor, error) {
	props := map[string]interface{}{
		"LastPostUpdateAt": cursor.LastPostUpdateAt,
		"LastPostId":       cursor.LastPostID,
		"Limit":            limit,
	}
	query :=
		`SELECT
			Posts.Id AS PostId,
			Posts.CreateAt AS PostCreateAt,
			Posts.UpdateAt AS PostUpdateAt,
			Posts.DeleteAt AS PostDeleteAt,
			Posts.Message AS PostMessage,
			Posts.Type AS PostType,
			Posts.Props AS PostProps,
			Posts.OriginalId AS PostOriginalId,
			Posts.RootId AS PostRootId,
			Posts.FileIds AS PostFileIds,
			Teams.Id AS TeamId,
			Teams.Name AS TeamName,
			Teams.DisplayName AS TeamDisplayName,
			Channels.Id AS ChannelId,
			CASE
				WHEN Channels.Type = 'D' THEN 'Direct Message'
				WHEN Channels.Type = 'G' THEN 'Group Message'
				ELSE Channels.DisplayName
			END AS ChannelDisplayName,
			Channels.Name AS ChannelName,
			Channels.Type AS ChannelType,
			Users.Id AS UserId,
			Users.Email AS UserEmail,
			Users.Username,
			Bots.UserId IS NOT NULL AS IsBot
		FROM
			Posts
		LEFT OUTER JOIN Channels ON Posts.ChannelId = Channels.Id
		LEFT OUTER JOIN Teams ON Channels.TeamId = Teams.Id
		LEFT OUTER JOIN Users ON Posts.UserId = Users.Id
		LEFT JOIN Bots ON Bots.UserId = Posts.UserId
		WHERE (
			Posts.UpdateAt > :LastPostUpdateAt
			OR (
				Posts.UpdateAt = :LastPostUpdateAt
				AND Posts.Id > :LastPostId
			)
		) AND Posts.Type NOT LIKE 'system_%'
		ORDER BY PostUpdateAt, PostId
		LIMIT :Limit`

	var cposts []*model.MessageExport
	if _, err := s.GetReplica().Select(&cposts, query, props); err != nil {
		return nil, cursor, errors.Wrap(err, "unable to export messages")
	}
	if len(cposts) > 0 {
		cursor.LastPostUpdateAt = *cposts[len(cposts)-1].PostUpdateAt
		cursor.LastPostID = *cposts[len(cposts)-1].PostID
	}
	return cposts, cursor, nil
}
