// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/mattermost/gorp"
	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/pkg/errors"
)

type SqlRetentionPolicyStore struct {
	*SqlStore
	metrics einterfaces.MetricsInterface
}

func newSqlRetentionPolicyStore(sqlStore *SqlStore, metrics einterfaces.MetricsInterface) store.RetentionPolicyStore {
	s := &SqlRetentionPolicyStore{
		SqlStore: sqlStore,
		metrics:  metrics,
	}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.RetentionPolicy{}, "RetentionPolicies")
		table.SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("DisplayName").SetMaxSize(64)

		tableC := db.AddTableWithName(model.RetentionPolicyChannel{}, "RetentionPoliciesChannels")
		tableC.SetKeys(false, "ChannelId")
		tableC.ColMap("PolicyId").SetMaxSize(26)
		tableC.ColMap("ChannelId").SetMaxSize(26)

		tableT := db.AddTableWithName(model.RetentionPolicyTeam{}, "RetentionPoliciesTeams")
		tableT.SetKeys(false, "TeamId")
		tableT.ColMap("PolicyId").SetMaxSize(26)
		tableT.ColMap("TeamId").SetMaxSize(26)
	}

	return s
}

func (s *SqlRetentionPolicyStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("IDX_RetentionPolicies_DisplayName", "RetentionPolicies", "DisplayName")
	s.CreateIndexIfNotExists("IDX_RetentionPoliciesChannels_PolicyId", "RetentionPoliciesChannels", "PolicyId")
	s.CreateIndexIfNotExists("IDX_RetentionPoliciesTeams_PolicyId", "RetentionPoliciesTeams", "PolicyId")
	s.CreateForeignKeyIfNotExists("RetentionPoliciesChannels", "PolicyId", "RetentionPolicies", "Id", true)
	s.CreateForeignKeyIfNotExists("RetentionPoliciesTeams", "PolicyId", "RetentionPolicies", "Id", true)
}

// executePossiblyEmptyQuery only executes the query if it is non-empty. This helps avoid
// having to check for MySQL, which, unlike Postgres, does not allow empty queries.
func executePossiblyEmptyQuery(txn *gorp.Transaction, query string, args ...interface{}) (sql.Result, error) {
	if query == "" {
		return nil, nil
	}
	return txn.Exec(query, args...)
}

func (s *SqlRetentionPolicyStore) Save(policy *model.RetentionPolicyWithTeamAndChannelIDs) (*model.RetentionPolicyWithTeamAndChannelCounts, error) {
	// Strategy:
	// 1. Insert new policy
	// 2. Insert new channels into policy
	// 3. Insert new teams into policy

	if err := s.checkTeamsExist(policy.TeamIDs); err != nil {
		return nil, err
	}
	if err := s.checkChannelsExist(policy.ChannelIDs); err != nil {
		return nil, err
	}

	policy.ID = model.NewID()

	policyInsertQuery, policyInsertArgs, err := s.getQueryBuilder().
		Insert("RetentionPolicies").
		Columns("Id", "DisplayName", "PostDuration").
		Values(policy.ID, policy.DisplayName, policy.PostDuration).
		ToSql()
	if err != nil {
		return nil, err
	}

	channelsInsertQuery, channelsInsertArgs, err := s.buildInsertRetentionPoliciesChannelsQuery(policy.ID, policy.ChannelIDs)
	if err != nil {
		return nil, err
	}

	teamsInsertQuery, teamsInsertArgs, err := s.buildInsertRetentionPoliciesTeamsQuery(policy.ID, policy.TeamIDs)
	if err != nil {
		return nil, err
	}

	policySelectQuery, policySelectProps := s.buildGetPolicyQuery(policy.ID)

	txn, err := s.GetMaster().Begin()
	if err != nil {
		return nil, err
	}
	defer finalizeTransaction(txn)
	// Create a new policy in RetentionPolicies
	if _, err = txn.Exec(policyInsertQuery, policyInsertArgs...); err != nil {
		return nil, err
	}
	// Insert the channel IDs into RetentionPoliciesChannels
	if _, err = executePossiblyEmptyQuery(txn, channelsInsertQuery, channelsInsertArgs...); err != nil {
		return nil, err
	}
	// Insert the team IDs into RetentionPoliciesTeams
	if _, err = executePossiblyEmptyQuery(txn, teamsInsertQuery, teamsInsertArgs...); err != nil {
		return nil, err
	}
	// Select the new policy (with team/channel counts) which we just created
	var newPolicy model.RetentionPolicyWithTeamAndChannelCounts
	if err = txn.SelectOne(&newPolicy, policySelectQuery, policySelectProps); err != nil {
		return nil, err
	}
	if err = txn.Commit(); err != nil {
		return nil, err
	}
	return &newPolicy, nil
}

func (s *SqlRetentionPolicyStore) checkTeamsExist(teamIDs []string) error {
	if len(teamIDs) > 0 {
		teamsSelectQuery, teamsSelectArgs, err := s.getQueryBuilder().
			Select("Id").
			From("Teams").
			Where(sq.Eq{"Id": teamIDs}).
			ToSql()
		if err != nil {
			return err
		}
		var rows []*string
		_, err = s.GetReplica().Select(&rows, teamsSelectQuery, teamsSelectArgs...)
		if err != nil {
			return err
		}
		if len(rows) == len(teamIDs) {
			return nil
		}
		retrievedIDs := make(map[string]bool)
		for _, teamID := range rows {
			retrievedIDs[*teamID] = true
		}
		for _, teamID := range teamIDs {
			if _, ok := retrievedIDs[teamID]; !ok {
				return store.NewErrNotFound("Team", teamID)
			}
		}
	}
	return nil
}

func (s *SqlRetentionPolicyStore) checkChannelsExist(channelIDs []string) error {
	if len(channelIDs) > 0 {
		channelsSelectQuery, channelsSelectArgs, err := s.getQueryBuilder().
			Select("Id").
			From("Channels").
			Where(sq.Eq{"Id": channelIDs}).
			ToSql()
		if err != nil {
			return err
		}
		var rows []*string
		_, err = s.GetReplica().Select(&rows, channelsSelectQuery, channelsSelectArgs...)
		if err != nil {
			return err
		}
		if len(rows) == len(channelIDs) {
			return nil
		}
		retrievedIDs := make(map[string]bool)
		for _, channelID := range rows {
			retrievedIDs[*channelID] = true
		}
		for _, channelID := range channelIDs {
			if _, ok := retrievedIDs[channelID]; !ok {
				return store.NewErrNotFound("Channel", channelID)
			}
		}
	}
	return nil
}

func (s *SqlRetentionPolicyStore) buildInsertRetentionPoliciesChannelsQuery(policyID string, channelIDs []string) (query string, args []interface{}, err error) {
	if len(channelIDs) > 0 {
		builder := s.getQueryBuilder().
			Insert("RetentionPoliciesChannels").
			Columns("PolicyId", "ChannelId")
		for _, channelID := range channelIDs {
			builder = builder.Values(policyID, channelID)
		}
		query, args, err = builder.ToSql()
	}
	return
}

func (s *SqlRetentionPolicyStore) buildInsertRetentionPoliciesTeamsQuery(policyID string, teamIDs []string) (query string, args []interface{}, err error) {
	if len(teamIDs) > 0 {
		builder := s.getQueryBuilder().
			Insert("RetentionPoliciesTeams").
			Columns("PolicyId", "TeamId")
		for _, teamID := range teamIDs {
			builder = builder.Values(policyID, teamID)
		}
		query, args, err = builder.ToSql()
	}
	return
}

func (s *SqlRetentionPolicyStore) Patch(patch *model.RetentionPolicyWithTeamAndChannelIDs) (*model.RetentionPolicyWithTeamAndChannelCounts, error) {
	// Strategy:
	// 1. Update policy attributes
	// 2. Delete existing channels from policy
	// 3. Insert new channels into policy
	// 4. Delete existing teams from policy
	// 5. Insert new teams into policy
	// 6. Read new policy

	var err error
	if err = s.checkTeamsExist(patch.TeamIDs); err != nil {
		return nil, err
	}
	if err = s.checkChannelsExist(patch.ChannelIDs); err != nil {
		return nil, err
	}

	policyUpdateQuery := ""
	policyUpdateArgs := []interface{}{}
	if patch.DisplayName != "" || patch.PostDuration != nil {
		builder := s.getQueryBuilder().Update("RetentionPolicies")
		if patch.DisplayName != "" {
			builder = builder.Set("DisplayName", patch.DisplayName)
		}
		if patch.PostDuration != nil {
			builder = builder.Set("PostDuration", *patch.PostDuration)
		}
		policyUpdateQuery, policyUpdateArgs, err = builder.
			Where(sq.Eq{"Id": patch.ID}).
			ToSql()
		if err != nil {
			return nil, err
		}
	}

	channelsDeleteQuery := ""
	channelsDeleteArgs := []interface{}{}
	channelsInsertQuery := ""
	channelsInsertArgs := []interface{}{}
	if patch.ChannelIDs != nil {
		channelsDeleteQuery, channelsDeleteArgs, err = s.getQueryBuilder().
			Delete("RetentionPoliciesChannels").
			Where(sq.Eq{"PolicyId": patch.ID}).
			ToSql()
		if err != nil {
			return nil, err
		}

		channelsInsertQuery, channelsInsertArgs, err = s.buildInsertRetentionPoliciesChannelsQuery(patch.ID, patch.ChannelIDs)
		if err != nil {
			return nil, err
		}
	}

	teamsDeleteQuery := ""
	teamsDeleteArgs := []interface{}{}
	teamsInsertQuery := ""
	teamsInsertArgs := []interface{}{}
	if patch.TeamIDs != nil {
		teamsDeleteQuery, teamsDeleteArgs, err = s.getQueryBuilder().
			Delete("RetentionPoliciesTeams").
			Where(sq.Eq{"PolicyId": patch.ID}).
			ToSql()
		if err != nil {
			return nil, err
		}

		teamsInsertQuery, teamsInsertArgs, err = s.buildInsertRetentionPoliciesTeamsQuery(patch.ID, patch.TeamIDs)
		if err != nil {
			return nil, err
		}
	}

	policySelectQuery, policySelectProps := s.buildGetPolicyQuery(patch.ID)

	txn, err := s.GetMaster().Begin()
	if err != nil {
		return nil, err
	}
	defer finalizeTransaction(txn)
	// Update the fields of the policy in RetentionPolicies
	if _, err = executePossiblyEmptyQuery(txn, policyUpdateQuery, policyUpdateArgs...); err != nil {
		return nil, err
	}
	// Remove all channels from the policy in RetentionPoliciesChannels
	if _, err = executePossiblyEmptyQuery(txn, channelsDeleteQuery, channelsDeleteArgs...); err != nil {
		return nil, err
	}
	// Insert the new channels for the policy in RetentionPoliciesChannels
	if _, err = executePossiblyEmptyQuery(txn, channelsInsertQuery, channelsInsertArgs...); err != nil {
		return nil, err
	}
	// Remove all teams from the policy in RetentionPoliciesTeams
	if _, err = executePossiblyEmptyQuery(txn, teamsDeleteQuery, teamsDeleteArgs...); err != nil {
		return nil, err
	}
	// Insert the new teams for the policy in RetentionPoliciesTeams
	if _, err = executePossiblyEmptyQuery(txn, teamsInsertQuery, teamsInsertArgs...); err != nil {
		return nil, err
	}
	// Select the policy which we just updated
	var newPolicy model.RetentionPolicyWithTeamAndChannelCounts
	if err = txn.SelectOne(&newPolicy, policySelectQuery, policySelectProps); err != nil {
		return nil, err
	}
	if err = txn.Commit(); err != nil {
		return nil, err
	}
	return &newPolicy, nil
}

func (s *SqlRetentionPolicyStore) buildGetPolicyQuery(id string) (query string, props map[string]interface{}) {
	return s.buildGetPoliciesQuery(id, 0, 1)
}

// buildGetPoliciesQuery builds a query to select information for the policy with the specified
// ID, or, if `id` is the empty string, from all policies. The results returned will be sorted by
// policy display name and ID.
func (s *SqlRetentionPolicyStore) buildGetPoliciesQuery(id string, offset, limit int) (query string, props map[string]interface{}) {
	props = map[string]interface{}{"Offset": offset, "Limit": limit}
	whereIDEqualsPolicyID := ""
	if id != "" {
		whereIDEqualsPolicyID = "WHERE RetentionPolicies.Id = :PolicyId"
		props["PolicyId"] = id
	}
	query = `
	SELECT RetentionPolicies.Id,
	       RetentionPolicies.DisplayName,
	       RetentionPolicies.PostDuration,
	       A.Count AS ChannelCount,
	       B.Count AS TeamCount
	FROM RetentionPolicies
	INNER JOIN (
		SELECT RetentionPolicies.Id,
		       COUNT(RetentionPoliciesChannels.ChannelId) AS Count
		FROM RetentionPolicies
		LEFT JOIN RetentionPoliciesChannels ON RetentionPolicies.Id = RetentionPoliciesChannels.PolicyId
		` + whereIDEqualsPolicyID + `
		GROUP BY RetentionPolicies.Id
		ORDER BY RetentionPolicies.DisplayName, RetentionPolicies.Id
		LIMIT :Limit
		OFFSET :Offset
	) AS A ON RetentionPolicies.Id = A.Id
	INNER JOIN (
		SELECT RetentionPolicies.Id,
		       COUNT(RetentionPoliciesTeams.TeamId) AS Count
		FROM RetentionPolicies
		LEFT JOIN RetentionPoliciesTeams ON RetentionPolicies.Id = RetentionPoliciesTeams.PolicyId
		` + whereIDEqualsPolicyID + `
		GROUP BY RetentionPolicies.Id
		ORDER BY RetentionPolicies.DisplayName, RetentionPolicies.Id
		LIMIT :Limit
		OFFSET :Offset
	) AS B ON RetentionPolicies.Id = B.Id
	ORDER BY RetentionPolicies.DisplayName, RetentionPolicies.Id`
	return
}

func (s *SqlRetentionPolicyStore) Get(id string) (*model.RetentionPolicyWithTeamAndChannelCounts, error) {
	query, props := s.buildGetPolicyQuery(id)
	var policy model.RetentionPolicyWithTeamAndChannelCounts
	if err := s.GetReplica().SelectOne(&policy, query, props); err != nil {
		return nil, err
	}
	return &policy, nil
}

func (s *SqlRetentionPolicyStore) GetAll(offset, limit int) (policies []*model.RetentionPolicyWithTeamAndChannelCounts, err error) {
	query, props := s.buildGetPoliciesQuery("", offset, limit)
	_, err = s.GetReplica().Select(&policies, query, props)
	return
}

func (s *SqlRetentionPolicyStore) GetCount() (int64, error) {
	return s.GetReplica().SelectInt("SELECT COUNT(*) FROM RetentionPolicies")
}

func (s *SqlRetentionPolicyStore) Delete(id string) error {
	builder := s.getQueryBuilder().
		Delete("RetentionPolicies").
		Where(sq.Eq{"Id": id})
	result, err := builder.RunWith(s.GetMaster()).Exec()
	if err != nil {
		return err
	}
	numRowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	} else if numRowsAffected == 0 {
		return errors.New("policy not found")
	}
	return nil
}

func (s *SqlRetentionPolicyStore) GetChannels(policyID string, offset, limit int) (channels model.ChannelListWithTeamData, err error) {
	const query = `
	SELECT Channels.*,
	       Teams.DisplayName AS TeamDisplayName,
	       Teams.Name AS TeamName,
	       Teams.UpdateAt AS TeamUpdateAt
	FROM RetentionPoliciesChannels
	INNER JOIN Channels ON RetentionPoliciesChannels.ChannelId = Channels.Id
	INNER JOIN Teams ON Channels.TeamId = Teams.Id
	WHERE RetentionPoliciesChannels.PolicyId = :PolicyId
	ORDER BY Channels.DisplayName, Channels.Id
	LIMIT :Limit
	OFFSET :Offset`
	props := map[string]interface{}{"PolicyId": policyID, "Limit": limit, "Offset": offset}
	_, err = s.GetReplica().Select(&channels, query, props)
	for _, channel := range channels {
		channel.PolicyID = model.NewString(policyID)
	}
	return
}

func (s *SqlRetentionPolicyStore) GetChannelsCount(policyID string) (int64, error) {
	const query = `
	SELECT COUNT(*)
	FROM RetentionPolicies
	INNER JOIN RetentionPoliciesChannels ON RetentionPolicies.Id = RetentionPoliciesChannels.PolicyId
	WHERE RetentionPolicies.Id = :PolicyId`
	props := map[string]interface{}{"PolicyId": policyID}
	return s.GetReplica().SelectInt(query, props)
}

func (s *SqlRetentionPolicyStore) AddChannels(policyID string, channelIDs []string) error {
	if len(channelIDs) == 0 {
		return nil
	}
	if err := s.checkChannelsExist(channelIDs); err != nil {
		return err
	}
	builder := s.getQueryBuilder().
		Insert("RetentionPoliciesChannels").
		Columns("policyId", "channelId")
	for _, channelID := range channelIDs {
		builder = builder.Values(policyID, channelID)
	}
	_, err := builder.RunWith(s.GetMaster()).Exec()
	if err != nil {
		switch dbErr := err.(type) {
		case *pq.Error:
			if dbErr.Code == PGForeignKeyViolationErrorCode {
				return store.NewErrNotFound("RetentionPolicy", policyID)
			}
		case *mysql.MySQLError:
			if dbErr.Number == MySQLForeignKeyViolationErrorCode {
				return store.NewErrNotFound("RetentionPolicy", policyID)
			}
		}
	}
	return err
}

func (s *SqlRetentionPolicyStore) RemoveChannels(policyID string, channelIDs []string) error {
	if len(channelIDs) == 0 {
		return nil
	}
	builder := s.getQueryBuilder().
		Delete("RetentionPoliciesChannels").
		Where(sq.And{
			sq.Eq{"PolicyId": policyID},
			sq.Eq{"ChannelId": channelIDs},
		})
	_, err := builder.RunWith(s.GetMaster()).Exec()
	return err
}

func (s *SqlRetentionPolicyStore) GetTeams(policyID string, offset, limit int) (teams []*model.Team, err error) {
	const query = `
	SELECT Teams.* FROM RetentionPoliciesTeams
	INNER JOIN Teams ON RetentionPoliciesTeams.TeamId = Teams.Id
	WHERE RetentionPoliciesTeams.PolicyId = :PolicyId
	ORDER BY Teams.DisplayName, Teams.Id
	LIMIT :Limit
	OFFSET :Offset`
	props := map[string]interface{}{"PolicyId": policyID, "Limit": limit, "Offset": offset}
	_, err = s.GetReplica().Select(&teams, query, props)
	for _, team := range teams {
		team.PolicyID = &policyID
	}
	return
}

func (s *SqlRetentionPolicyStore) GetTeamsCount(policyID string) (int64, error) {
	const query = `
	SELECT COUNT(*)
	FROM RetentionPolicies
	INNER JOIN RetentionPoliciesTeams ON RetentionPolicies.Id = RetentionPoliciesTeams.PolicyId
	WHERE RetentionPolicies.Id = :PolicyId`
	props := map[string]interface{}{"PolicyId": policyID}
	return s.GetReplica().SelectInt(query, props)
}

func (s *SqlRetentionPolicyStore) AddTeams(policyID string, teamIDs []string) error {
	if len(teamIDs) == 0 {
		return nil
	}
	if err := s.checkTeamsExist(teamIDs); err != nil {
		return err
	}
	builder := s.getQueryBuilder().
		Insert("RetentionPoliciesTeams").
		Columns("PolicyId", "TeamId")
	for _, teamID := range teamIDs {
		builder = builder.Values(policyID, teamID)
	}
	_, err := builder.RunWith(s.GetMaster()).Exec()
	return err
}

func (s *SqlRetentionPolicyStore) RemoveTeams(policyID string, teamIDs []string) error {
	if len(teamIDs) == 0 {
		return nil
	}
	builder := s.getQueryBuilder().
		Delete("RetentionPoliciesTeams").
		Where(sq.And{
			sq.Eq{"PolicyId": policyID},
			sq.Eq{"TeamId": teamIDs},
		})
	_, err := builder.RunWith(s.GetMaster()).Exec()
	return err
}

// DeleteOrphanedRows removes entries from RetentionPoliciesChannels and RetentionPoliciesTeams
// where a channel or team no longer exists.
func (s *SqlRetentionPolicyStore) DeleteOrphanedRows(limit int) (deleted int64, err error) {
	// We need the extra level of nesting to deal with MySQL's locking
	const rpcDeleteQuery = `
	DELETE FROM RetentionPoliciesChannels WHERE ChannelId IN (
		SELECT * FROM (
			SELECT ChannelId FROM RetentionPoliciesChannels
			LEFT JOIN Channels ON RetentionPoliciesChannels.ChannelId = Channels.Id
			WHERE Channels.Id IS NULL
			LIMIT :Limit
		) AS A
	)`
	const rptDeleteQuery = `
	DELETE FROM RetentionPoliciesTeams WHERE TeamId IN (
		SELECT * FROM (
			SELECT TeamId FROM RetentionPoliciesTeams
			LEFT JOIN Teams ON RetentionPoliciesTeams.TeamId = Teams.Id
			WHERE Teams.Id IS NULL
			LIMIT :Limit
		) AS A
	)`
	props := map[string]interface{}{"Limit": limit}
	result, err := s.GetMaster().Exec(rpcDeleteQuery, props)
	if err != nil {
		return
	}
	rpcDeleted, err := result.RowsAffected()
	if err != nil {
		return
	}
	result, err = s.GetMaster().Exec(rptDeleteQuery, props)
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

func (s *SqlRetentionPolicyStore) GetTeamPoliciesForUser(userID string, offset, limit int) (policies []*model.RetentionPolicyForTeam, err error) {
	const query = `
	SELECT Teams.Id, RetentionPolicies.PostDuration
	FROM Users
	INNER JOIN TeamMembers ON Users.Id = TeamMembers.UserId
	INNER JOIN Teams ON TeamMembers.TeamId = Teams.Id
	INNER JOIN RetentionPoliciesTeams ON Teams.Id = RetentionPoliciesTeams.TeamId
	INNER JOIN RetentionPolicies ON RetentionPoliciesTeams.PolicyId = RetentionPolicies.Id
	WHERE Users.Id = :UserId
		AND TeamMembers.DeleteAt = 0
		AND Teams.DeleteAt = 0
	ORDER BY Teams.Id
	LIMIT :Limit
	OFFSET :Offset`
	props := map[string]interface{}{"UserId": userID, "Limit": limit, "Offset": offset}
	_, err = s.GetReplica().Select(&policies, query, props)
	return
}

func (s *SqlRetentionPolicyStore) GetTeamPoliciesCountForUser(userID string) (int64, error) {
	const query = `
	SELECT COUNT(*)
	FROM Users
	INNER JOIN TeamMembers ON Users.Id = TeamMembers.UserId
	INNER JOIN Teams ON TeamMembers.TeamId = Teams.Id
	INNER JOIN RetentionPoliciesTeams ON Teams.Id = RetentionPoliciesTeams.TeamId
	INNER JOIN RetentionPolicies ON RetentionPoliciesTeams.PolicyId = RetentionPolicies.Id
	WHERE Users.Id = :UserId
		AND TeamMembers.DeleteAt = 0
		AND Teams.DeleteAt = 0`
	props := map[string]interface{}{"UserId": userID}
	return s.GetReplica().SelectInt(query, props)
}

func (s *SqlRetentionPolicyStore) GetChannelPoliciesForUser(userID string, offset, limit int) (policies []*model.RetentionPolicyForChannel, err error) {
	const query = `
	SELECT Channels.Id, RetentionPolicies.PostDuration
	FROM Users
	INNER JOIN ChannelMembers ON Users.Id = ChannelMembers.UserId
	INNER JOIN Channels ON ChannelMembers.ChannelId = Channels.Id
	INNER JOIN RetentionPoliciesChannels ON Channels.Id = RetentionPoliciesChannels.ChannelId
	INNER JOIN RetentionPolicies ON RetentionPoliciesChannels.PolicyId = RetentionPolicies.Id
	WHERE Users.Id = :UserId
		AND Channels.DeleteAt = 0
	ORDER BY Channels.Id
	LIMIT :Limit
	OFFSET :Offset`
	props := map[string]interface{}{"UserId": userID, "Limit": limit, "Offset": offset}
	_, err = s.GetReplica().Select(&policies, query, props)
	return
}

func (s *SqlRetentionPolicyStore) GetChannelPoliciesCountForUser(userID string) (int64, error) {
	const query = `
	SELECT COUNT(*)
	FROM Users
	INNER JOIN ChannelMembers ON Users.Id = ChannelMembers.UserId
	INNER JOIN Channels ON ChannelMembers.ChannelId = Channels.Id
	INNER JOIN RetentionPoliciesChannels ON Channels.Id = RetentionPoliciesChannels.ChannelId
	INNER JOIN RetentionPolicies ON RetentionPoliciesChannels.PolicyId = RetentionPolicies.Id
	WHERE Users.Id = :UserId
		AND Channels.DeleteAt = 0`
	props := map[string]interface{}{"UserId": userID}
	return s.GetReplica().SelectInt(query, props)
}

// RetentionPolicyBatchDeletionInfo gives information on how to delete records
// under a retention policy; see `genericPermanentDeleteBatchForRetentionPolicies`.
//
// `BaseBuilder` should already have selected the primary key(s) for the main table
// and should be joined to a table with a ChannelId column, which will be used to join
// on the Channels table.
// `Table` is the name of the table from which records are being deleted.
// `TimeColumn` is the name of the column which contains the timestamp of the record.
// `PrimaryKeys` contains the primary keys of `table`. It should be the same as the
// `From` clause in `baseBuilder`.
// `ChannelIDTable` is the table which contains the ChannelId column, it may be the
// same as `table`, or will be different if a join was used.
// `NowMillis` must be a Unix timestamp in milliseconds and is used by the granular
// policies; if `nowMillis - timestamp(record)` is greater than
// the post duration of a granular policy, than the record will be deleted.
// `GlobalPolicyEndTime` is used by the global policy; any record older than this time
// will be deleted by the global policy if it does not fall under a granular policy.
// To disable the granular policies, set `NowMillis` to 0.
// To disable the global policy, set `GlobalPolicyEndTime` to 0.
type RetentionPolicyBatchDeletionInfo struct {
	BaseBuilder         sq.SelectBuilder
	Table               string
	TimeColumn          string
	PrimaryKeys         []string
	ChannelIDTable      string
	NowMillis           int64
	GlobalPolicyEndTime int64
	Limit               int64
}

// genericPermanentDeleteBatchForRetentionPolicies is a helper function for tables
// which need to delete records for granular and global policies.
func genericPermanentDeleteBatchForRetentionPolicies(
	r RetentionPolicyBatchDeletionInfo,
	s *SqlStore,
	cursor model.RetentionPolicyCursor,
) (int64, model.RetentionPolicyCursor, error) {
	baseBuilder := r.BaseBuilder.InnerJoin("Channels ON " + r.ChannelIDTable + ".ChannelId = Channels.Id")

	scopedTimeColumn := r.Table + "." + r.TimeColumn
	nowStr := strconv.FormatInt(r.NowMillis, 10)
	// A record falls under the scope of a granular retention policy if:
	// 1. The policy's post duration is >= 0
	// 2. The record's lifespan has not exceeded the policy's post duration
	const millisecondsInADay = 24 * 60 * 60 * 1000
	fallsUnderGranularPolicy := sq.And{
		sq.GtOrEq{"RetentionPolicies.PostDuration": 0},
		sq.Expr(nowStr + " - " + scopedTimeColumn + " > RetentionPolicies.PostDuration * " + strconv.FormatInt(millisecondsInADay, 10)),
	}

	// If the caller wants to disable the global policy from running
	if r.GlobalPolicyEndTime <= 0 {
		cursor.GlobalPoliciesDone = true
	}
	// If the caller wants to disable the granular policies from running
	if r.NowMillis <= 0 {
		cursor.ChannelPoliciesDone = true
		cursor.TeamPoliciesDone = true
	}

	var totalRowsAffected int64

	// First, delete all of the records which fall under the scope of a channel-specific policy
	if !cursor.ChannelPoliciesDone {
		channelPoliciesBuilder := baseBuilder.
			InnerJoin("RetentionPoliciesChannels ON " + r.ChannelIDTable + ".ChannelId = RetentionPoliciesChannels.ChannelId").
			InnerJoin("RetentionPolicies ON RetentionPoliciesChannels.PolicyId = RetentionPolicies.Id").
			Where(fallsUnderGranularPolicy).
			Limit(uint64(r.Limit))
		rowsAffected, err := genericRetentionPoliciesDeletion(channelPoliciesBuilder, r, s)
		if err != nil {
			return 0, cursor, err
		}
		if rowsAffected < r.Limit {
			cursor.ChannelPoliciesDone = true
		}
		totalRowsAffected += rowsAffected
		r.Limit -= rowsAffected
	}

	// Next, delete all of the records which fall under the scope of a team-specific policy
	if cursor.ChannelPoliciesDone && !cursor.TeamPoliciesDone {
		// Channel-specific policies override team-specific policies.
		teamPoliciesBuilder := baseBuilder.
			LeftJoin("RetentionPoliciesChannels ON " + r.ChannelIDTable + ".ChannelId = RetentionPoliciesChannels.ChannelId").
			InnerJoin("RetentionPoliciesTeams ON Channels.TeamId = RetentionPoliciesTeams.TeamId").
			InnerJoin("RetentionPolicies ON RetentionPoliciesTeams.PolicyId = RetentionPolicies.Id").
			Where(sq.And{
				sq.Eq{"RetentionPoliciesChannels.PolicyId": nil},
				sq.Expr("RetentionPoliciesTeams.PolicyId = RetentionPolicies.Id"),
			}).
			Where(fallsUnderGranularPolicy).
			Limit(uint64(r.Limit))
		rowsAffected, err := genericRetentionPoliciesDeletion(teamPoliciesBuilder, r, s)
		if err != nil {
			return 0, cursor, err
		}
		if rowsAffected < r.Limit {
			cursor.TeamPoliciesDone = true
		}
		totalRowsAffected += rowsAffected
		r.Limit -= rowsAffected
	}

	// Finally, delete all of the records which fall under the scope of the global policy
	if cursor.ChannelPoliciesDone && cursor.TeamPoliciesDone && !cursor.GlobalPoliciesDone {
		// Granular policies override the global policy.
		globalPolicyBuilder := baseBuilder.
			LeftJoin("RetentionPoliciesChannels ON " + r.ChannelIDTable + ".ChannelId = RetentionPoliciesChannels.ChannelId").
			LeftJoin("RetentionPoliciesTeams ON Channels.TeamId = RetentionPoliciesTeams.TeamId").
			LeftJoin("RetentionPolicies ON RetentionPoliciesChannels.PolicyId = RetentionPolicies.Id").
			Where(sq.And{
				sq.Eq{"RetentionPoliciesChannels.PolicyId": nil},
				sq.Eq{"RetentionPoliciesTeams.PolicyId": nil},
			}).
			Where(sq.Lt{scopedTimeColumn: r.GlobalPolicyEndTime}).
			Limit(uint64(r.Limit))
		rowsAffected, err := genericRetentionPoliciesDeletion(globalPolicyBuilder, r, s)
		if err != nil {
			return 0, cursor, err
		}
		if rowsAffected < r.Limit {
			cursor.GlobalPoliciesDone = true
		}
		totalRowsAffected += rowsAffected
	}

	return totalRowsAffected, cursor, nil
}

// genericRetentionPoliciesDeletion actually executes the DELETE query using a sq.SelectBuilder
// which selects the rows to delete.
func genericRetentionPoliciesDeletion(
	builder sq.SelectBuilder,
	r RetentionPolicyBatchDeletionInfo,
	s *SqlStore,
) (rowsAffected int64, err error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, r.Table+"_tosql")
	}
	if s.DriverName() == model.DatabaseDriverPostgres {
		primaryKeysStr := "(" + strings.Join(r.PrimaryKeys, ",") + ")"
		query = `
		DELETE FROM ` + r.Table + ` WHERE ` + primaryKeysStr + ` IN (
		` + query + `
		)`
	} else {
		// MySQL does not support the LIMIT clause in a subquery with IN
		clauses := make([]string, len(r.PrimaryKeys))
		for i, key := range r.PrimaryKeys {
			clauses[i] = r.Table + "." + key + " = A." + key
		}
		joinClause := strings.Join(clauses, " AND ")
		query = `
		DELETE ` + r.Table + ` FROM ` + r.Table + ` INNER JOIN (
		` + query + `
		) AS A ON ` + joinClause
	}
	result, err := s.GetMaster().Exec(query, args...)
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete "+r.Table)
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected for "+r.Table)
	}
	return
}
