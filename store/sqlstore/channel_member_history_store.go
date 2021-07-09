// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SqlChannelMemberHistoryStore struct {
	*SqlStore
}

func newSqlChannelMemberHistoryStore(sqlStore *SqlStore) store.ChannelMemberHistoryStore {
	s := &SqlChannelMemberHistoryStore{
		SqlStore: sqlStore,
	}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.ChannelMemberHistory{}, "ChannelMemberHistory").SetKeys(false, "ChannelId", "UserId", "JoinTime")
		table.ColMap("ChannelId").SetMaxSize(26)
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("JoinTime").SetNotNull(true)
	}

	return s
}

func (s SqlChannelMemberHistoryStore) LogJoinEvent(userID string, channelID string, joinTime int64) error {
	channelMemberHistory := &model.ChannelMemberHistory{
		UserID:    userID,
		ChannelID: channelID,
		JoinTime:  joinTime,
	}

	if err := s.GetMaster().Insert(channelMemberHistory); err != nil {
		return errors.Wrapf(err, "LogJoinEvent userId=%s channelId=%s joinTime=%d", userID, channelID, joinTime)
	}
	return nil
}

func (s SqlChannelMemberHistoryStore) LogLeaveEvent(userID string, channelID string, leaveTime int64) error {
	query, params, err := s.getQueryBuilder().
		Update("ChannelMemberHistory").
		Set("LeaveTime", leaveTime).
		Where(sq.And{
			sq.Eq{"UserId": userID},
			sq.Eq{"ChannelId": channelID},
			sq.Eq{"LeaveTime": nil},
		}).ToSql()
	if err != nil {
		return errors.Wrap(err, "channel_member_history_to_sql")
	}
	sqlResult, err := s.GetMaster().Exec(query, params...)
	if err != nil {
		return errors.Wrapf(err, "LogLeaveEvent userId=%s channelId=%s leaveTime=%d", userID, channelID, leaveTime)
	}

	if rows, err := sqlResult.RowsAffected(); err == nil && rows != 1 {
		// there was no join event to update - this is best effort, so no need to raise an error
		mlog.Warn("Channel join event for user and channel not found", mlog.String("user", userID), mlog.String("channel", channelID))
	}
	return nil
}

func (s SqlChannelMemberHistoryStore) GetUsersInChannelDuring(startTime int64, endTime int64, channelID string) ([]*model.ChannelMemberHistoryResult, error) {
	useChannelMemberHistory, err := s.hasDataAtOrBefore(startTime)
	if err != nil {
		return nil, errors.Wrapf(err, "hasDataAtOrBefore startTime=%d endTime=%d channelId=%s", startTime, endTime, channelID)
	}

	if useChannelMemberHistory {
		// the export period starts after the ChannelMemberHistory table was first introduced, so we can use the
		// data from it for our export
		channelMemberHistories, err2 := s.getFromChannelMemberHistoryTable(startTime, endTime, channelID)
		if err2 != nil {
			return nil, errors.Wrapf(err2, "getFromChannelMemberHistoryTable startTime=%d endTime=%d channelId=%s", startTime, endTime, channelID)
		}
		return channelMemberHistories, nil
	}
	// the export period starts before the ChannelMemberHistory table was introduced, so we need to fake the
	// data by assuming that anybody who has ever joined the channel in question was present during the export period.
	// this may not always be true, but it's better than saying that somebody wasn't there when they were
	channelMemberHistories, err := s.getFromChannelMembersTable(startTime, endTime, channelID)
	if err != nil {
		return nil, errors.Wrapf(err, "getFromChannelMembersTable startTime=%d endTime=%d channelId=%s", startTime, endTime, channelID)
	}
	return channelMemberHistories, nil
}

func (s SqlChannelMemberHistoryStore) hasDataAtOrBefore(time int64) (bool, error) {
	type NullableCountResult struct {
		Min sql.NullInt64
	}
	query, _, err := s.getQueryBuilder().Select("MIN(JoinTime) as Min").From("ChannelMemberHistory").ToSql()
	if err != nil {
		return false, errors.Wrap(err, "channel_member_history_to_sql")
	}
	var result NullableCountResult
	if err := s.GetReplica().SelectOne(&result, query); err != nil {
		return false, err
	} else if result.Min.Valid {
		return result.Min.Int64 <= time, nil
	} else {
		// if the result was null, there are no rows in the table, so there is no data from before
		return false, nil
	}
}

func (s SqlChannelMemberHistoryStore) getFromChannelMemberHistoryTable(startTime int64, endTime int64, channelID string) ([]*model.ChannelMemberHistoryResult, error) {
	query, args, err := s.getQueryBuilder().
		Select("cmh.*, u.Email, u.Username, Bots.UserId IS NOT NULL AS IsBot, u.DeleteAt AS UserDeleteAt").
		From("ChannelMemberHistory cmh").
		Join("Users u ON cmh.UserId = u.Id").
		LeftJoin("Bots ON Bots.UserId = u.Id").
		Where(sq.And{
			sq.Eq{"cmh.ChannelId": channelID},
			sq.LtOrEq{"cmh.JoinTime": endTime},
			sq.Or{
				sq.Eq{"cmh.LeaveTime": nil},
				sq.GtOrEq{"cmh.LeaveTime": startTime},
			},
		}).
		OrderBy("cmh.JoinTime ASC").ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "channel_member_history_to_sql")
	}
	var histories []*model.ChannelMemberHistoryResult
	if _, err := s.GetReplica().Select(&histories, query, args...); err != nil {
		return nil, err
	}

	return histories, nil
}

func (s SqlChannelMemberHistoryStore) getFromChannelMembersTable(startTime int64, endTime int64, channelID string) ([]*model.ChannelMemberHistoryResult, error) {
	query, args, err := s.getQueryBuilder().
		Select("ch.ChannelId, ch.UserId, u.Email, u.Username, Bots.UserId IS NOT NULL AS IsBot, u.DeleteAt AS UserDeleteAt").
		Distinct().
		From("ChannelMembers ch").
		Join("Users u ON ch.UserId = u.id").
		LeftJoin("Bots ON Bots.UserId = u.id").
		Where(sq.Eq{"ch.ChannelId": channelID}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "channel_member_history_to_sql")
	}

	var histories []*model.ChannelMemberHistoryResult
	if _, err := s.GetReplica().Select(&histories, query, args...); err != nil {
		return nil, err
	}
	// we have to fill in the join/leave times, because that data doesn't exist in the channel members table
	for _, channelMemberHistory := range histories {
		channelMemberHistory.JoinTime = startTime
		channelMemberHistory.LeaveTime = model.NewInt64(endTime)
	}
	return histories, nil
}

// PermanentDeleteBatchForRetentionPolicies deletes a batch of records which are affected by
// the global or a granular retention policy.
// See `genericPermanentDeleteBatchForRetentionPolicies` for details.
func (s SqlChannelMemberHistoryStore) PermanentDeleteBatchForRetentionPolicies(now, globalPolicyEndTime, limit int64, cursor model.RetentionPolicyCursor) (int64, model.RetentionPolicyCursor, error) {
	builder := s.getQueryBuilder().
		Select("ChannelMemberHistory.ChannelId, ChannelMemberHistory.UserId, ChannelMemberHistory.JoinTime").
		From("ChannelMemberHistory")
	return genericPermanentDeleteBatchForRetentionPolicies(RetentionPolicyBatchDeletionInfo{
		BaseBuilder:         builder,
		Table:               "ChannelMemberHistory",
		TimeColumn:          "LeaveTime",
		PrimaryKeys:         []string{"ChannelId", "UserId", "JoinTime"},
		ChannelIDTable:      "ChannelMemberHistory",
		NowMillis:           now,
		GlobalPolicyEndTime: globalPolicyEndTime,
		Limit:               limit,
	}, s.SqlStore, cursor)
}

// DeleteOrphanedRows removes entries from ChannelMemberHistory when a corresponding channel no longer exists.
func (s SqlChannelMemberHistoryStore) DeleteOrphanedRows(limit int) (deleted int64, err error) {
	// We need the extra level of nesting to deal with MySQL's locking
	const query = `
	DELETE FROM ChannelMemberHistory WHERE (ChannelId, UserId, JoinTime) IN (
		SELECT * FROM (
			SELECT ChannelId, UserId, JoinTime FROM ChannelMemberHistory
			LEFT JOIN Channels ON ChannelMemberHistory.ChannelId = Channels.Id
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

func (s SqlChannelMemberHistoryStore) PermanentDeleteBatch(endTime int64, limit int64) (int64, error) {
	var (
		query string
		args  []interface{}
		err   error
	)

	if s.DriverName() == model.DatabaseDriverPostgres {
		var innerSelect string
		innerSelect, args, err = s.getQueryBuilder().
			Select("ctid").
			From("ChannelMemberHistory").
			Where(sq.And{
				sq.NotEq{"LeaveTime": nil},
				sq.LtOrEq{"LeaveTime": endTime},
			}).Limit(uint64(limit)).
			ToSql()
		if err != nil {
			return 0, errors.Wrap(err, "channel_member_history_to_sql")
		}
		query, _, err = s.getQueryBuilder().
			Delete("ChannelMemberHistory").
			Where(fmt.Sprintf(
				"ctid IN (%s)", innerSelect,
			)).ToSql()
	} else {
		query, args, err = s.getQueryBuilder().
			Delete("ChannelMemberHistory").
			Where(sq.And{
				sq.NotEq{"LeaveTime": nil},
				sq.LtOrEq{"LeaveTime": endTime},
			}).
			Limit(uint64(limit)).ToSql()
	}
	if err != nil {
		return 0, errors.Wrap(err, "channel_member_history_to_sql")
	}
	sqlResult, err := s.GetMaster().Exec(query, args...)
	if err != nil {
		return 0, errors.Wrapf(err, "PermanentDeleteBatch endTime=%d limit=%d", endTime, limit)
	}

	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return 0, errors.Wrapf(err, "PermanentDeleteBatch endTime=%d limit=%d", endTime, limit)
	}
	return rowsAffected, nil
}
