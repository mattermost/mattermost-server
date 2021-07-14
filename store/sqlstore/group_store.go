// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type selectType int

const (
	selectGroups selectType = iota
	selectCountGroups
)

type groupTeam struct {
	model.GroupSyncable
	TeamId string `db:"TeamId"`
}

type groupChannel struct {
	model.GroupSyncable
	ChannelId string `db:"ChannelId"`
}

type groupTeamJoin struct {
	groupTeam
	TeamDisplayName string `db:"TeamDisplayName"`
	TeamType        string `db:"TeamType"`
}

type groupChannelJoin struct {
	groupChannel
	ChannelDisplayName string `db:"ChannelDisplayName"`
	TeamDisplayName    string `db:"TeamDisplayName"`
	TeamType           string `db:"TeamType"`
	ChannelType        string `db:"ChannelType"`
	TeamID             string `db:"TeamId"`
}

type SqlGroupStore struct {
	*SqlStore
}

func newSqlGroupStore(sqlStore *SqlStore) store.GroupStore {
	s := &SqlGroupStore{SqlStore: sqlStore}
	for _, db := range sqlStore.GetAllConns() {
		groups := db.AddTableWithName(model.Group{}, "UserGroups").SetKeys(false, "Id")
		groups.ColMap("Id").SetMaxSize(26)
		groups.ColMap("Name").SetMaxSize(model.GroupNameMaxLength).SetUnique(true)
		groups.ColMap("DisplayName").SetMaxSize(model.GroupDisplayNameMaxLength)
		groups.ColMap("Description").SetMaxSize(model.GroupDescriptionMaxLength)
		groups.ColMap("Source").SetMaxSize(model.GroupSourceMaxLength)
		groups.ColMap("RemoteId").SetMaxSize(model.GroupRemoteIDMaxLength)
		groups.SetUniqueTogether("Source", "RemoteId")

		groupMembers := db.AddTableWithName(model.GroupMember{}, "GroupMembers").SetKeys(false, "GroupId", "UserId")
		groupMembers.ColMap("GroupId").SetMaxSize(26)
		groupMembers.ColMap("UserId").SetMaxSize(26)

		groupTeams := db.AddTableWithName(groupTeam{}, "GroupTeams").SetKeys(false, "GroupId", "TeamId")
		groupTeams.ColMap("GroupId").SetMaxSize(26)
		groupTeams.ColMap("TeamId").SetMaxSize(26)

		groupChannels := db.AddTableWithName(groupChannel{}, "GroupChannels").SetKeys(false, "GroupId", "ChannelId")
		groupChannels.ColMap("GroupId").SetMaxSize(26)
		groupChannels.ColMap("ChannelId").SetMaxSize(26)
	}
	return s
}

func (s *SqlGroupStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_groupmembers_create_at", "GroupMembers", "CreateAt")
	s.CreateIndexIfNotExists("idx_usergroups_remote_id", "UserGroups", "RemoteId")
	s.CreateIndexIfNotExists("idx_usergroups_delete_at", "UserGroups", "DeleteAt")
	s.CreateIndexIfNotExists("idx_groupteams_teamid", "GroupTeams", "TeamId")
	s.CreateIndexIfNotExists("idx_groupchannels_channelid", "GroupChannels", "ChannelId")
	s.CreateColumnIfNotExistsNoDefault("Channels", "GroupConstrained", "tinyint(1)", "boolean")
	s.CreateColumnIfNotExistsNoDefault("Teams", "GroupConstrained", "tinyint(1)", "boolean")
	s.CreateIndexIfNotExists("idx_groupteams_schemeadmin", "GroupTeams", "SchemeAdmin")
	s.CreateIndexIfNotExists("idx_groupchannels_schemeadmin", "GroupChannels", "SchemeAdmin")
}

func (s *SqlGroupStore) Create(group *model.Group) (*model.Group, error) {
	if group.Id != "" {
		return nil, store.NewErrInvalidInput("Group", "id", group.Id)
	}

	if err := group.IsValidForCreate(); err != nil {
		return nil, err
	}

	group.Id = model.NewId()
	group.CreateAt = model.GetMillis()
	group.UpdateAt = group.CreateAt

	if err := s.GetMaster().Insert(group); err != nil {
		if IsUniqueConstraintError(err, []string{"Name", "groups_name_key"}) {
			return nil, errors.Wrapf(err, "Group with name %s already exists", *group.Name)
		}
		return nil, errors.Wrap(err, "failed to save Group")
	}

	return group, nil
}

func (s *SqlGroupStore) Get(groupId string) (*model.Group, error) {
	var group *model.Group
	if err := s.GetReplica().SelectOne(&group, "SELECT * from UserGroups WHERE Id = :Id", map[string]interface{}{"Id": groupId}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Group", groupId)
		}
		return nil, errors.Wrapf(err, "failed to get Group with id=%s", groupId)
	}

	return group, nil
}

func (s *SqlGroupStore) GetByName(name string, opts model.GroupSearchOpts) (*model.Group, error) {
	var group *model.Group
	query := s.getQueryBuilder().Select("*").From("UserGroups").Where(sq.Eq{"Name": name})
	if opts.FilterAllowReference {
		query = query.Where("AllowReference = true")
	}

	queryString, args, err := query.ToSql()

	if err != nil {
		return nil, errors.Wrap(err, "get_by_name_tosql")
	}
	if err := s.GetReplica().SelectOne(&group, queryString, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Group", fmt.Sprintf("name=%s", name))
		}
		return nil, errors.Wrapf(err, "failed to get Group with name=%s", name)
	}

	return group, nil
}

func (s *SqlGroupStore) GetByIDs(groupIDs []string) ([]*model.Group, error) {
	var groups []*model.Group
	query := s.getQueryBuilder().Select("*").From("UserGroups").Where(sq.Eq{"Id": groupIDs})
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_by_ids_tosql")
	}
	if _, err := s.GetReplica().Select(&groups, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Groups by ids")
	}
	return groups, nil
}

func (s *SqlGroupStore) GetByRemoteID(remoteID string, groupSource model.GroupSource) (*model.Group, error) {
	var group *model.Group
	if err := s.GetReplica().SelectOne(&group, "SELECT * from UserGroups WHERE RemoteId = :RemoteId AND Source = :Source", map[string]interface{}{"RemoteId": remoteID, "Source": groupSource}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Group", fmt.Sprintf("remoteId=%s", remoteID))
		}
		return nil, errors.Wrapf(err, "failed to get Group with remoteId=%s", remoteID)
	}

	return group, nil
}

func (s *SqlGroupStore) GetAllBySource(groupSource model.GroupSource) ([]*model.Group, error) {
	var groups []*model.Group

	if _, err := s.GetReplica().Select(&groups, "SELECT * from UserGroups WHERE DeleteAt = 0 AND Source = :Source", map[string]interface{}{"Source": groupSource}); err != nil {
		return nil, errors.Wrapf(err, "failed to find Groups by groupSource=%v", groupSource)
	}

	return groups, nil
}

func (s *SqlGroupStore) GetByUser(userId string) ([]*model.Group, error) {
	var groups []*model.Group

	query := `
		SELECT
			UserGroups.*
		FROM
			GroupMembers
			JOIN UserGroups ON UserGroups.Id = GroupMembers.GroupId
		WHERE
			GroupMembers.DeleteAt = 0
			AND UserId = :UserId`

	if _, err := s.GetReplica().Select(&groups, query, map[string]interface{}{"UserId": userId}); err != nil {
		return nil, errors.Wrapf(err, "failed to find Groups with userId=%s", userId)
	}

	return groups, nil
}

func (s *SqlGroupStore) Update(group *model.Group) (*model.Group, error) {
	var retrievedGroup *model.Group
	if err := s.GetReplica().SelectOne(&retrievedGroup, "SELECT * FROM UserGroups WHERE Id = :Id", map[string]interface{}{"Id": group.Id}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Group", group.Id)
		}
		return nil, errors.Wrapf(err, "failed to get Group with id=%s", group.Id)
	}

	// If updating DeleteAt it can only be to 0
	if group.DeleteAt != retrievedGroup.DeleteAt && group.DeleteAt != 0 {
		return nil, errors.New("DeleteAt should be 0 when updating")
	}

	// Reset these properties, don't update them based on input
	group.CreateAt = retrievedGroup.CreateAt
	group.UpdateAt = model.GetMillis()

	if err := group.IsValidForUpdate(); err != nil {
		return nil, err
	}

	rowsChanged, err := s.GetMaster().Update(group)
	if err != nil {
		if IsUniqueConstraintError(err, []string{"Name", "groups_name_key"}) {
			return nil, errors.Wrapf(err, "Group with name %s already exists", *group.Name)
		}
		return nil, errors.Wrap(err, "failed to update Group")
	}
	if rowsChanged > 1 {
		return nil, errors.Wrapf(err, "multiple Groups were update: %d", rowsChanged)
	}

	return group, nil
}

func (s *SqlGroupStore) Delete(groupID string) (*model.Group, error) {
	var group *model.Group
	if err := s.GetReplica().SelectOne(&group, "SELECT * from UserGroups WHERE Id = :Id AND DeleteAt = 0", map[string]interface{}{"Id": groupID}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Group", groupID)
		}
		return nil, errors.Wrapf(err, "failed to get Group with id=%s", groupID)
	}

	time := model.GetMillis()
	group.DeleteAt = time
	group.UpdateAt = time

	if _, err := s.GetMaster().Update(group); err != nil {
		return nil, errors.Wrapf(err, "failed to update Group with id=%s", groupID)
	}

	return group, nil
}

func (s *SqlGroupStore) GetMemberUsers(groupID string) ([]*model.User, error) {
	var groupMembers []*model.User

	query := `
		SELECT
			Users.*
		FROM
			GroupMembers
			JOIN Users ON Users.Id = GroupMembers.UserId
		WHERE
			GroupMembers.DeleteAt = 0
			AND Users.DeleteAt = 0
			AND GroupId = :GroupId`

	if _, err := s.GetReplica().Select(&groupMembers, query, map[string]interface{}{"GroupId": groupID}); err != nil {
		return nil, errors.Wrapf(err, "failed to find member Users for Group with id=%s", groupID)
	}

	return groupMembers, nil
}

func (s *SqlGroupStore) GetMemberUsersPage(groupID string, page int, perPage int) ([]*model.User, error) {
	var groupMembers []*model.User

	query := `
		SELECT
			Users.*
		FROM
			GroupMembers
			JOIN Users ON Users.Id = GroupMembers.UserId
		WHERE
			GroupMembers.DeleteAt = 0
			AND Users.DeleteAt = 0
			AND GroupId = :GroupId
		ORDER BY
			GroupMembers.CreateAt DESC
		LIMIT
			:Limit
		OFFSET
			:Offset`

	if _, err := s.GetReplica().Select(&groupMembers, query, map[string]interface{}{"GroupId": groupID, "Limit": perPage, "Offset": page * perPage}); err != nil {
		return nil, errors.Wrapf(err, "failed to find member Users for Group with id=%s", groupID)
	}

	return groupMembers, nil
}

func (s *SqlGroupStore) GetMemberCount(groupID string) (int64, error) {
	query := `
		SELECT
			count(*)
		FROM
			GroupMembers
			JOIN Users ON Users.Id = GroupMembers.UserId
		WHERE
			GroupMembers.GroupId = :GroupId
			AND Users.DeleteAt = 0`

	count, err := s.GetReplica().SelectInt(query, map[string]interface{}{"GroupId": groupID})
	if err != nil {
		return int64(0), errors.Wrapf(err, "failed to count member Users for Group with id=%s", groupID)
	}

	return count, nil
}

func (s *SqlGroupStore) GetMemberUsersInTeam(groupID string, teamID string) ([]*model.User, error) {
	var groupMembers []*model.User

	query := `
		SELECT
			Users.*
		FROM
			GroupMembers
			JOIN Users ON Users.Id = GroupMembers.UserId
		WHERE
			GroupId = :GroupId
			AND GroupMembers.UserId IN (
				SELECT TeamMembers.UserId
				FROM TeamMembers
				JOIN Teams ON Teams.Id = :TeamId
				WHERE TeamMembers.TeamId = Teams.Id
				AND TeamMembers.DeleteAt = 0
			)
			AND GroupMembers.DeleteAt = 0
			AND Users.DeleteAt = 0
		`

	if _, err := s.GetReplica().Select(&groupMembers, query, map[string]interface{}{"GroupId": groupID, "TeamId": teamID}); err != nil {
		return nil, errors.Wrapf(err, "failed to member Users for groupId=%s and teamId=%s", groupID, teamID)
	}

	return groupMembers, nil
}

func (s *SqlGroupStore) GetMemberUsersNotInChannel(groupID string, channelID string) ([]*model.User, error) {
	var groupMembers []*model.User

	query := `
		SELECT
			Users.*
		FROM
			GroupMembers
			JOIN Users ON Users.Id = GroupMembers.UserId
		WHERE
			GroupId = :GroupId
			AND GroupMembers.UserId NOT IN (
				SELECT ChannelMembers.UserId
				FROM ChannelMembers
				WHERE ChannelMembers.ChannelId = :ChannelId
			)
			AND GroupMembers.UserId IN (
				SELECT TeamMembers.UserId
				FROM TeamMembers
				JOIN Channels ON Channels.Id = :ChannelId
				JOIN Teams ON Teams.Id = Channels.TeamId
				WHERE TeamMembers.TeamId = Teams.Id
				AND TeamMembers.DeleteAt = 0
			)
			AND GroupMembers.DeleteAt = 0
			AND Users.DeleteAt = 0
		`

	if _, err := s.GetReplica().Select(&groupMembers, query, map[string]interface{}{"GroupId": groupID, "ChannelId": channelID}); err != nil {
		return nil, errors.Wrapf(err, "failed to member Users for groupId=%s and channelId!=%s", groupID, channelID)
	}

	return groupMembers, nil
}

func (s *SqlGroupStore) UpsertMember(groupID string, userID string) (*model.GroupMember, error) {
	member := &model.GroupMember{
		GroupId:  groupID,
		UserId:   userID,
		CreateAt: model.GetMillis(),
		DeleteAt: 0,
	}

	if err := member.IsValid(); err != nil {
		return nil, err
	}

	var retrievedGroup *model.Group
	if err := s.GetReplica().SelectOne(&retrievedGroup, "SELECT * FROM UserGroups WHERE Id = :Id", map[string]interface{}{"Id": groupID}); err != nil {
		return nil, errors.Wrapf(err, "failed to get UserGroup with groupId=%s and userId=%s", groupID, userID)
	}

	query := s.getQueryBuilder().
		Insert("GroupMembers").
		Columns("GroupId", "UserId", "CreateAt", "DeleteAt").
		Values(member.GroupId, member.UserId, member.CreateAt, member.DeleteAt)

	if s.DriverName() == model.DATABASE_DRIVER_MYSQL {
		query = query.SuffixExpr(sq.Expr("ON DUPLICATE KEY UPDATE CreateAt = ?, DeleteAt = ?", member.CreateAt, member.DeleteAt))
	} else if s.DriverName() == model.DATABASE_DRIVER_POSTGRES {
		query = query.SuffixExpr(sq.Expr("ON CONFLICT (groupid, userid) DO UPDATE SET CreateAt = ?, DeleteAt = ?", member.CreateAt, member.DeleteAt))
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate sqlquery")
	}

	if _, err = s.GetMaster().Exec(queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to save GroupMember")
	}
	return member, nil
}

func (s *SqlGroupStore) DeleteMember(groupID string, userID string) (*model.GroupMember, error) {
	var retrievedMember *model.GroupMember
	if err := s.GetReplica().SelectOne(&retrievedMember, "SELECT * FROM GroupMembers WHERE GroupId = :GroupId AND UserId = :UserId AND DeleteAt = 0", map[string]interface{}{"GroupId": groupID, "UserId": userID}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("GroupMember", fmt.Sprintf("groupId=%s, userId=%s", groupID, userID))
		}
		return nil, errors.Wrapf(err, "failed to get GroupMember with groupId=%s and userId=%s", groupID, userID)
	}

	retrievedMember.DeleteAt = model.GetMillis()

	if _, err := s.GetMaster().Update(retrievedMember); err != nil {
		return nil, errors.Wrapf(err, "failed to update GroupMember with groupId=%s and userId=%s", groupID, userID)
	}

	return retrievedMember, nil
}

func (s *SqlGroupStore) PermanentDeleteMembersByUser(userId string) error {
	if _, err := s.GetMaster().Exec("DELETE FROM GroupMembers WHERE UserId = :UserId", map[string]interface{}{"UserId": userId}); err != nil {
		return errors.Wrapf(err, "failed to permanent delete GroupMember with userId=%s", userId)
	}
	return nil
}

func (s *SqlGroupStore) CreateGroupSyncable(groupSyncable *model.GroupSyncable) (*model.GroupSyncable, error) {
	if err := groupSyncable.IsValid(); err != nil {
		return nil, err
	}

	// Reset values that shouldn't be updatable by parameter
	groupSyncable.DeleteAt = 0
	groupSyncable.CreateAt = model.GetMillis()
	groupSyncable.UpdateAt = groupSyncable.CreateAt

	var insertErr error

	switch groupSyncable.Type {
	case model.GroupSyncableTypeTeam:
		if _, err := s.Team().Get(groupSyncable.SyncableId); err != nil {
			return nil, err
		}

		insertErr = s.GetMaster().Insert(groupSyncableToGroupTeam(groupSyncable))
	case model.GroupSyncableTypeChannel:
		var channel *model.Channel
		channel, err := s.Channel().Get(groupSyncable.SyncableId, false)
		if err != nil {
			return nil, err
		}
		insertErr = s.GetMaster().Insert(groupSyncableToGroupChannel(groupSyncable))
		groupSyncable.TeamID = channel.TeamId
	default:
		return nil, fmt.Errorf("invalid GroupSyncableType: %s", groupSyncable.Type)
	}

	if insertErr != nil {
		return nil, errors.Wrap(insertErr, "unable to insert GroupSyncable")
	}

	return groupSyncable, nil
}

func (s *SqlGroupStore) GetGroupSyncable(groupID string, syncableID string, syncableType model.GroupSyncableType) (*model.GroupSyncable, error) {
	groupSyncable, err := s.getGroupSyncable(groupID, syncableID, syncableType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("GroupSyncable", fmt.Sprintf("groupId=%s, syncableId=%s, syncableType=%s", groupID, syncableID, syncableType))
		}
		return nil, errors.Wrapf(err, "failed to find GroupSyncable with groupId=%s, syncableId=%s, syncableType=%s", groupID, syncableID, syncableType)
	}

	return groupSyncable, nil
}

func (s *SqlGroupStore) getGroupSyncable(groupID string, syncableID string, syncableType model.GroupSyncableType) (*model.GroupSyncable, error) {
	var err error
	var result interface{}

	switch syncableType {
	case model.GroupSyncableTypeTeam:
		result, err = s.GetReplica().Get(groupTeam{}, groupID, syncableID)
	case model.GroupSyncableTypeChannel:
		result, err = s.GetReplica().Get(groupChannel{}, groupID, syncableID)
	}

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, sql.ErrNoRows
	}

	groupSyncable := model.GroupSyncable{}
	switch syncableType {
	case model.GroupSyncableTypeTeam:
		groupTeam := result.(*groupTeam)
		groupSyncable.SyncableId = groupTeam.TeamId
		groupSyncable.GroupId = groupTeam.GroupId
		groupSyncable.AutoAdd = groupTeam.AutoAdd
		groupSyncable.CreateAt = groupTeam.CreateAt
		groupSyncable.DeleteAt = groupTeam.DeleteAt
		groupSyncable.UpdateAt = groupTeam.UpdateAt
		groupSyncable.Type = syncableType
	case model.GroupSyncableTypeChannel:
		groupChannel := result.(*groupChannel)
		groupSyncable.SyncableId = groupChannel.ChannelId
		groupSyncable.GroupId = groupChannel.GroupId
		groupSyncable.AutoAdd = groupChannel.AutoAdd
		groupSyncable.CreateAt = groupChannel.CreateAt
		groupSyncable.DeleteAt = groupChannel.DeleteAt
		groupSyncable.UpdateAt = groupChannel.UpdateAt
		groupSyncable.Type = syncableType
	default:
		return nil, fmt.Errorf("unable to convert syncableType: %s", syncableType.String())
	}

	return &groupSyncable, nil
}

func (s *SqlGroupStore) GetAllGroupSyncablesByGroupId(groupID string, syncableType model.GroupSyncableType) ([]*model.GroupSyncable, error) {
	args := map[string]interface{}{"GroupId": groupID}

	groupSyncables := []*model.GroupSyncable{}

	switch syncableType {
	case model.GroupSyncableTypeTeam:
		sqlQuery := `
			SELECT
				GroupTeams.*,
				Teams.DisplayName AS TeamDisplayName,
				Teams.Type AS TeamType
			FROM
				GroupTeams
				JOIN Teams ON Teams.Id = GroupTeams.TeamId
			WHERE
				GroupId = :GroupId AND GroupTeams.DeleteAt = 0`

		results := []*groupTeamJoin{}
		_, err := s.GetReplica().Select(&results, sqlQuery, args)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find GroupTeams with groupId=%s", groupID)
		}
		for _, result := range results {
			groupSyncable := &model.GroupSyncable{
				SyncableId:      result.TeamId,
				GroupId:         result.GroupId,
				AutoAdd:         result.AutoAdd,
				CreateAt:        result.CreateAt,
				DeleteAt:        result.DeleteAt,
				UpdateAt:        result.UpdateAt,
				Type:            syncableType,
				TeamDisplayName: result.TeamDisplayName,
				TeamType:        result.TeamType,
				SchemeAdmin:     result.SchemeAdmin,
			}
			groupSyncables = append(groupSyncables, groupSyncable)
		}
	case model.GroupSyncableTypeChannel:
		sqlQuery := `
			SELECT
				GroupChannels.*,
				Channels.DisplayName AS ChannelDisplayName,
				Teams.DisplayName AS TeamDisplayName,
				Channels.Type As ChannelType,
				Teams.Type As TeamType,
				Teams.Id AS TeamId
			FROM
				GroupChannels
				JOIN Channels ON Channels.Id = GroupChannels.ChannelId
				JOIN Teams ON Teams.Id = Channels.TeamId
			WHERE
				GroupId = :GroupId AND GroupChannels.DeleteAt = 0`

		results := []*groupChannelJoin{}
		_, err := s.GetReplica().Select(&results, sqlQuery, args)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find GroupChannels with groupId=%s", groupID)
		}
		for _, result := range results {
			groupSyncable := &model.GroupSyncable{
				SyncableId:         result.ChannelId,
				GroupId:            result.GroupId,
				AutoAdd:            result.AutoAdd,
				CreateAt:           result.CreateAt,
				DeleteAt:           result.DeleteAt,
				UpdateAt:           result.UpdateAt,
				Type:               syncableType,
				ChannelDisplayName: result.ChannelDisplayName,
				ChannelType:        result.ChannelType,
				TeamDisplayName:    result.TeamDisplayName,
				TeamType:           result.TeamType,
				TeamID:             result.TeamID,
				SchemeAdmin:        result.SchemeAdmin,
			}
			groupSyncables = append(groupSyncables, groupSyncable)
		}
	}

	return groupSyncables, nil
}

func (s *SqlGroupStore) UpdateGroupSyncable(groupSyncable *model.GroupSyncable) (*model.GroupSyncable, error) {
	retrievedGroupSyncable, err := s.getGroupSyncable(groupSyncable.GroupId, groupSyncable.SyncableId, groupSyncable.Type)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Wrap(store.NewErrNotFound("GroupSyncable", fmt.Sprintf("groupId=%s, syncableId=%s, syncableType=%s", groupSyncable.GroupId, groupSyncable.SyncableId, groupSyncable.Type)), "GroupSyncable not found")
		}
		return nil, errors.Wrapf(err, "failed to find GroupSyncable with groupId=%s, syncableId=%s, syncableType=%s", groupSyncable.GroupId, groupSyncable.SyncableId, groupSyncable.Type)
	}

	if err := groupSyncable.IsValid(); err != nil {
		return nil, err
	}

	// If updating DeleteAt it can only be to 0
	if groupSyncable.DeleteAt != retrievedGroupSyncable.DeleteAt && groupSyncable.DeleteAt != 0 {
		return nil, errors.New("DeleteAt should be 0 when updating")
	}

	// Reset these properties, don't update them based on input
	groupSyncable.CreateAt = retrievedGroupSyncable.CreateAt
	groupSyncable.UpdateAt = model.GetMillis()

	switch groupSyncable.Type {
	case model.GroupSyncableTypeTeam:
		_, err = s.GetMaster().Update(groupSyncableToGroupTeam(groupSyncable))
	case model.GroupSyncableTypeChannel:
		// We need to get the TeamId so redux can manage channels when teams are unlinked
		var channel *model.Channel
		channel, channelErr := s.Channel().Get(groupSyncable.SyncableId, false)
		if channelErr != nil {
			return nil, channelErr
		}

		_, err = s.GetMaster().Update(groupSyncableToGroupChannel(groupSyncable))

		groupSyncable.TeamID = channel.TeamId
	default:
		return nil, fmt.Errorf("invalid GroupSyncableType: %s", groupSyncable.Type)
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to update GroupSyncable")
	}

	return groupSyncable, nil
}

func (s *SqlGroupStore) DeleteGroupSyncable(groupID string, syncableID string, syncableType model.GroupSyncableType) (*model.GroupSyncable, error) {
	groupSyncable, err := s.getGroupSyncable(groupID, syncableID, syncableType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("GroupSyncable", fmt.Sprintf("groupId=%s, syncableId=%s, syncableType=%s", groupID, syncableID, syncableType))
		}
		return nil, errors.Wrapf(err, "failed to find GroupSyncable with groupId=%s, syncableId=%s, syncableType=%s", groupID, syncableID, syncableType)
	}

	if groupSyncable.DeleteAt != 0 {
		return nil, store.NewErrInvalidInput("GroupSyncable", "<groupId, syncableId, syncableType>", fmt.Sprintf("<%s, %s, %s>", groupSyncable.GroupId, groupSyncable.SyncableId, groupSyncable.Type))
	}

	time := model.GetMillis()
	groupSyncable.DeleteAt = time
	groupSyncable.UpdateAt = time

	switch groupSyncable.Type {
	case model.GroupSyncableTypeTeam:
		_, err = s.GetMaster().Update(groupSyncableToGroupTeam(groupSyncable))
	case model.GroupSyncableTypeChannel:
		_, err = s.GetMaster().Update(groupSyncableToGroupChannel(groupSyncable))
	default:
		return nil, fmt.Errorf("invalid GroupSyncableType: %s", groupSyncable.Type)
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to update GroupSyncable")
	}

	return groupSyncable, nil
}

func (s *SqlGroupStore) TeamMembersToAdd(since int64, teamID *string, includeRemovedMembers bool) ([]*model.UserTeamIDPair, error) {
	builder := s.getQueryBuilder().Select("GroupMembers.UserId", "GroupTeams.TeamId").
		From("GroupMembers").
		Join("GroupTeams ON GroupTeams.GroupId = GroupMembers.GroupId").
		Join("UserGroups ON UserGroups.Id = GroupMembers.GroupId").
		Join("Teams ON Teams.Id = GroupTeams.TeamId").
		Where(sq.Eq{
			"UserGroups.DeleteAt":   0,
			"GroupTeams.DeleteAt":   0,
			"GroupTeams.AutoAdd":    true,
			"GroupMembers.DeleteAt": 0,
			"Teams.DeleteAt":        0,
		})

	if !includeRemovedMembers {
		builder = builder.
			JoinClause("LEFT OUTER JOIN TeamMembers ON TeamMembers.TeamId = GroupTeams.TeamId AND TeamMembers.UserId = GroupMembers.UserId").
			Where(sq.Eq{"TeamMembers.UserId": nil}).
			Where(sq.Or{
				sq.GtOrEq{"GroupMembers.CreateAt": since},
				sq.GtOrEq{"GroupTeams.UpdateAt": since},
			})
	}
	if teamID != nil {
		builder = builder.Where(sq.Eq{"Teams.Id": *teamID})
	}

	query, params, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "team_members_to_add_tosql")
	}

	var teamMembers []*model.UserTeamIDPair

	_, err = s.GetMaster().Select(&teamMembers, query, params...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find UserTeamIDPairs")
	}

	return teamMembers, nil
}

func (s *SqlGroupStore) ChannelMembersToAdd(since int64, channelID *string, includeRemovedMembers bool) ([]*model.UserChannelIDPair, error) {
	builder := s.getQueryBuilder().Select("GroupMembers.UserId", "GroupChannels.ChannelId").
		From("GroupMembers").
		Join("GroupChannels ON GroupChannels.GroupId = GroupMembers.GroupId").
		Join("UserGroups ON UserGroups.Id = GroupMembers.GroupId").
		Join("Channels ON Channels.Id = GroupChannels.ChannelId").
		Where(sq.Eq{
			"UserGroups.DeleteAt":    0,
			"GroupChannels.DeleteAt": 0,
			"GroupChannels.AutoAdd":  true,
			"GroupMembers.DeleteAt":  0,
			"Channels.DeleteAt":      0,
		})

	if !includeRemovedMembers {
		builder = builder.
			JoinClause("LEFT OUTER JOIN ChannelMemberHistory ON ChannelMemberHistory.ChannelId = GroupChannels.ChannelId AND ChannelMemberHistory.UserId = GroupMembers.UserId").
			Where(sq.Eq{
				"ChannelMemberHistory.UserId":    nil,
				"ChannelMemberHistory.LeaveTime": nil,
			}).
			Where(sq.Or{
				sq.GtOrEq{"GroupMembers.CreateAt": since},
				sq.GtOrEq{"GroupChannels.UpdateAt": since},
			})
	}
	if channelID != nil {
		builder = builder.Where(sq.Eq{"Channels.Id": *channelID})
	}

	query, params, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "channel_members_to_add_tosql")
	}

	var channelMembers []*model.UserChannelIDPair

	_, err = s.GetMaster().Select(&channelMembers, query, params...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find UserChannelIDPairs")
	}

	return channelMembers, nil
}

func groupSyncableToGroupTeam(groupSyncable *model.GroupSyncable) *groupTeam {
	return &groupTeam{
		GroupSyncable: *groupSyncable,
		TeamId:        groupSyncable.SyncableId,
	}
}

func groupSyncableToGroupChannel(groupSyncable *model.GroupSyncable) *groupChannel {
	return &groupChannel{
		GroupSyncable: *groupSyncable,
		ChannelId:     groupSyncable.SyncableId,
	}
}

func (s *SqlGroupStore) TeamMembersToRemove(teamID *string) ([]*model.TeamMember, error) {
	whereStmt := `
		(TeamMembers.TeamId,
			TeamMembers.UserId)
		NOT IN (
			SELECT
				Teams.Id AS TeamId,
				GroupMembers.UserId
			FROM
				Teams
				JOIN GroupTeams ON GroupTeams.TeamId = Teams.Id
				JOIN UserGroups ON UserGroups.Id = GroupTeams.GroupId
				JOIN GroupMembers ON GroupMembers.GroupId = UserGroups.Id
			WHERE
				Teams.GroupConstrained = TRUE
				AND GroupTeams.DeleteAt = 0
				AND UserGroups.DeleteAt = 0
				AND Teams.DeleteAt = 0
				AND GroupMembers.DeleteAt = 0
			GROUP BY
				Teams.Id,
				GroupMembers.UserId)`

	builder := s.getQueryBuilder().Select(
		"TeamMembers.TeamId",
		"TeamMembers.UserId",
		"TeamMembers.Roles",
		"TeamMembers.DeleteAt",
		"TeamMembers.SchemeUser",
		"TeamMembers.SchemeAdmin",
		"(TeamMembers.SchemeGuest IS NOT NULL AND TeamMembers.SchemeGuest) AS SchemeGuest",
	).
		From("TeamMembers").
		Join("Teams ON Teams.Id = TeamMembers.TeamId").
		LeftJoin("Bots ON Bots.UserId = TeamMembers.UserId").
		Where(sq.Eq{"TeamMembers.DeleteAt": 0, "Teams.DeleteAt": 0, "Teams.GroupConstrained": true, "Bots.UserId": nil}).
		Where(whereStmt)

	if teamID != nil {
		builder = builder.Where(sq.Eq{"TeamMembers.TeamId": *teamID})
	}

	query, params, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "team_members_to_remove_tosql")
	}

	var teamMembers []*model.TeamMember

	_, err = s.GetReplica().Select(&teamMembers, query, params...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find TeamMembers")
	}

	return teamMembers, nil
}

func (s *SqlGroupStore) CountGroupsByChannel(channelId string, opts model.GroupSearchOpts) (int64, error) {
	countQuery := s.groupsBySyncableBaseQuery(model.GroupSyncableTypeChannel, selectCountGroups, channelId, opts)

	countQueryString, args, err := countQuery.ToSql()
	if err != nil {
		return int64(0), errors.Wrap(err, "count_groups_by_channel_tosql")
	}

	count, err := s.GetReplica().SelectInt(countQueryString, args...)
	if err != nil {
		return int64(0), errors.Wrapf(err, "failed to count Groups by channel with channelId=%s", channelId)
	}

	return count, nil
}

func (s *SqlGroupStore) GetGroupsByChannel(channelId string, opts model.GroupSearchOpts) ([]*model.GroupWithSchemeAdmin, error) {
	query := s.groupsBySyncableBaseQuery(model.GroupSyncableTypeChannel, selectGroups, channelId, opts)

	if opts.PageOpts != nil {
		offset := uint64(opts.PageOpts.Page * opts.PageOpts.PerPage)
		query = query.OrderBy("ug.DisplayName").Limit(uint64(opts.PageOpts.PerPage)).Offset(offset)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_groups_by_channel_tosql")
	}

	var groups []*model.GroupWithSchemeAdmin

	_, err = s.GetReplica().Select(&groups, queryString, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Groups with channelId=%s", channelId)
	}

	return groups, nil
}

func (s *SqlGroupStore) ChannelMembersToRemove(channelID *string) ([]*model.ChannelMember, error) {
	whereStmt := `
		(ChannelMembers.ChannelId,
			ChannelMembers.UserId)
		NOT IN (
			SELECT
				Channels.Id AS ChannelId,
				GroupMembers.UserId
			FROM
				Channels
				JOIN GroupChannels ON GroupChannels.ChannelId = Channels.Id
				JOIN UserGroups ON UserGroups.Id = GroupChannels.GroupId
				JOIN GroupMembers ON GroupMembers.GroupId = UserGroups.Id
			WHERE
				Channels.GroupConstrained = TRUE
				AND GroupChannels.DeleteAt = 0
				AND UserGroups.DeleteAt = 0
				AND Channels.DeleteAt = 0
				AND GroupMembers.DeleteAt = 0
			GROUP BY
				Channels.Id,
				GroupMembers.UserId)`

	builder := s.getQueryBuilder().Select(
		"ChannelMembers.ChannelId",
		"ChannelMembers.UserId",
		"ChannelMembers.LastViewedAt",
		"ChannelMembers.MsgCount",
		"ChannelMembers.MsgCountRoot",
		"ChannelMembers.MentionCount",
		"ChannelMembers.MentionCountRoot",
		"ChannelMembers.NotifyProps",
		"ChannelMembers.LastUpdateAt",
		"ChannelMembers.LastUpdateAt",
		"ChannelMembers.SchemeUser",
		"ChannelMembers.SchemeAdmin",
		"(ChannelMembers.SchemeGuest IS NOT NULL AND ChannelMembers.SchemeGuest) AS SchemeGuest",
	).
		From("ChannelMembers").
		Join("Channels ON Channels.Id = ChannelMembers.ChannelId").
		LeftJoin("Bots ON Bots.UserId = ChannelMembers.UserId").
		Where(sq.Eq{"Channels.DeleteAt": 0, "Channels.GroupConstrained": true, "Bots.UserId": nil}).
		Where(whereStmt)

	if channelID != nil {
		builder = builder.Where(sq.Eq{"ChannelMembers.ChannelId": *channelID})
	}

	query, params, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "channel_members_to_remove_tosql")
	}

	var channelMembers []*model.ChannelMember

	_, err = s.GetReplica().Select(&channelMembers, query, params...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find ChannelMembers")
	}

	return channelMembers, nil
}

func (s *SqlGroupStore) groupsBySyncableBaseQuery(st model.GroupSyncableType, t selectType, syncableID string, opts model.GroupSearchOpts) sq.SelectBuilder {
	selectStrs := map[selectType]string{
		selectGroups:      "ug.*, gs.SchemeAdmin AS SyncableSchemeAdmin",
		selectCountGroups: "COUNT(*)",
	}

	var table string
	var idCol string
	if st == model.GroupSyncableTypeTeam {
		table = "GroupTeams"
		idCol = "TeamId"
	} else {
		table = "GroupChannels"
		idCol = "ChannelId"
	}

	query := s.getQueryBuilder().
		Select(selectStrs[t]).
		From(fmt.Sprintf("%s gs", table)).
		LeftJoin("UserGroups ug ON gs.GroupId = ug.Id").
		Where(fmt.Sprintf("ug.DeleteAt = 0 AND gs.%s = ? AND gs.DeleteAt = 0", idCol), syncableID)

	if opts.IncludeMemberCount && t == selectGroups {
		query = s.getQueryBuilder().
			Select(fmt.Sprintf("ug.*, coalesce(Members.MemberCount, 0) AS MemberCount, Group%ss.SchemeAdmin AS SyncableSchemeAdmin", st)).
			From("UserGroups ug").
			LeftJoin("(SELECT GroupMembers.GroupId, COUNT(*) AS MemberCount FROM GroupMembers LEFT JOIN Users ON Users.Id = GroupMembers.UserId WHERE GroupMembers.DeleteAt = 0 AND Users.DeleteAt = 0 GROUP BY GroupId) AS Members ON Members.GroupId = ug.Id").
			LeftJoin(fmt.Sprintf("%[1]s ON %[1]s.GroupId = ug.Id", table)).
			Where(fmt.Sprintf("ug.DeleteAt = 0 AND %[1]s.DeleteAt = 0 AND %[1]s.%[2]s = ?", table, idCol), syncableID).
			OrderBy("ug.DisplayName")
	}

	if opts.FilterAllowReference && t == selectGroups {
		query = query.Where("ug.AllowReference = true")
	}

	if opts.Q != "" {
		pattern := fmt.Sprintf("%%%s%%", sanitizeSearchTerm(opts.Q, "\\"))
		operatorKeyword := "ILIKE"
		if s.DriverName() == model.DATABASE_DRIVER_MYSQL {
			operatorKeyword = "LIKE"
		}
		query = query.Where(fmt.Sprintf("(ug.Name %[1]s ? OR ug.DisplayName %[1]s ?)", operatorKeyword), pattern, pattern)
	}

	return query
}

func (s *SqlGroupStore) getGroupsAssociatedToChannelsByTeam(teamID string, opts model.GroupSearchOpts) sq.SelectBuilder {
	query := s.getQueryBuilder().
		Select("gc.ChannelId, ug.*, gc.SchemeAdmin AS SyncableSchemeAdmin").
		From("UserGroups ug").
		LeftJoin(`
			(SELECT
				GroupChannels.GroupId, GroupChannels.ChannelId, GroupChannels.DeleteAt, GroupChannels.SchemeAdmin
			FROM
				GroupChannels
			LEFT JOIN
				Channels ON (Channels.Id = GroupChannels.ChannelId)
			WHERE
				GroupChannels.DeleteAt = 0
				AND Channels.DeleteAt = 0
				AND Channels.TeamId = ?) AS gc ON gc.GroupId = ug.Id`, teamID).
		Where("ug.DeleteAt = 0 AND gc.DeleteAt = 0").
		OrderBy("ug.DisplayName")

	if opts.IncludeMemberCount {
		query = s.getQueryBuilder().
			Select("gc.ChannelId, ug.*, coalesce(Members.MemberCount, 0) AS MemberCount, gc.SchemeAdmin AS SyncableSchemeAdmin").
			From("UserGroups ug").
			LeftJoin(`
				(SELECT
					GroupChannels.ChannelId, GroupChannels.DeleteAt, GroupChannels.GroupId, GroupChannels.SchemeAdmin
				FROM
					GroupChannels
				LEFT JOIN
					Channels ON (Channels.Id = GroupChannels.ChannelId)
				WHERE
					GroupChannels.DeleteAt = 0
					AND Channels.DeleteAt = 0
					AND Channels.TeamId = ?) AS gc ON gc.GroupId = ug.Id`, teamID).
			LeftJoin(`(
				SELECT
					GroupMembers.GroupId, COUNT(*) AS MemberCount
				FROM
					GroupMembers
				LEFT JOIN
					Users ON Users.Id = GroupMembers.UserId
				WHERE
					GroupMembers.DeleteAt = 0
					AND Users.DeleteAt = 0
				GROUP BY GroupId) AS Members
			ON Members.GroupId = ug.Id`).
			Where("ug.DeleteAt = 0 AND gc.DeleteAt = 0").
			OrderBy("ug.DisplayName")
	}

	if opts.FilterAllowReference {
		query = query.Where("ug.AllowReference = true")
	}

	if opts.Q != "" {
		pattern := fmt.Sprintf("%%%s%%", sanitizeSearchTerm(opts.Q, "\\"))
		operatorKeyword := "ILIKE"
		if s.DriverName() == model.DATABASE_DRIVER_MYSQL {
			operatorKeyword = "LIKE"
		}
		query = query.Where(fmt.Sprintf("(ug.Name %[1]s ? OR ug.DisplayName %[1]s ?)", operatorKeyword), pattern, pattern)
	}

	return query
}

func (s *SqlGroupStore) CountGroupsByTeam(teamId string, opts model.GroupSearchOpts) (int64, error) {
	countQuery := s.groupsBySyncableBaseQuery(model.GroupSyncableTypeTeam, selectCountGroups, teamId, opts)

	countQueryString, args, err := countQuery.ToSql()
	if err != nil {
		return int64(0), errors.Wrap(err, "count_groups_by_team_tosql")
	}

	count, err := s.GetReplica().SelectInt(countQueryString, args...)
	if err != nil {
		return int64(0), errors.Wrapf(err, "failed to count Groups with teamId=%s", teamId)
	}

	return count, nil
}

func (s *SqlGroupStore) GetGroupsByTeam(teamId string, opts model.GroupSearchOpts) ([]*model.GroupWithSchemeAdmin, error) {
	query := s.groupsBySyncableBaseQuery(model.GroupSyncableTypeTeam, selectGroups, teamId, opts)

	if opts.PageOpts != nil {
		offset := uint64(opts.PageOpts.Page * opts.PageOpts.PerPage)
		query = query.OrderBy("ug.DisplayName").Limit(uint64(opts.PageOpts.PerPage)).Offset(offset)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_groups_by_team_tosql")
	}

	var groups []*model.GroupWithSchemeAdmin

	_, err = s.GetReplica().Select(&groups, queryString, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Groups with teamId=%s", teamId)
	}

	return groups, nil
}

func (s *SqlGroupStore) GetGroupsAssociatedToChannelsByTeam(teamId string, opts model.GroupSearchOpts) (map[string][]*model.GroupWithSchemeAdmin, error) {
	query := s.getGroupsAssociatedToChannelsByTeam(teamId, opts)

	if opts.PageOpts != nil {
		offset := uint64(opts.PageOpts.Page * opts.PageOpts.PerPage)
		query = query.OrderBy("ug.DisplayName").Limit(uint64(opts.PageOpts.PerPage)).Offset(offset)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_groups_associated_to_channel_by_team_tosql")
	}

	var tgroups []*model.GroupsAssociatedToChannelWithSchemeAdmin

	_, err = s.GetReplica().Select(&tgroups, queryString, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Groups with teamId=%s", teamId)
	}

	groups := map[string][]*model.GroupWithSchemeAdmin{}
	for _, tgroup := range tgroups {
		var group = model.GroupWithSchemeAdmin{}
		group.Group = tgroup.Group
		group.SchemeAdmin = tgroup.SchemeAdmin

		if val, ok := groups[tgroup.ChannelId]; ok {
			groups[tgroup.ChannelId] = append(val, &group)
		} else {
			groups[tgroup.ChannelId] = []*model.GroupWithSchemeAdmin{&group}
		}
	}

	return groups, nil
}

func (s *SqlGroupStore) GetGroups(page, perPage int, opts model.GroupSearchOpts) ([]*model.Group, error) {
	var groups []*model.Group

	groupsQuery := s.getQueryBuilder().Select("g.*")

	if opts.IncludeMemberCount {
		groupsQuery = s.getQueryBuilder().
			Select("g.*, coalesce(Members.MemberCount, 0) AS MemberCount").
			LeftJoin("(SELECT GroupMembers.GroupId, COUNT(*) AS MemberCount FROM GroupMembers LEFT JOIN Users ON Users.Id = GroupMembers.UserId WHERE GroupMembers.DeleteAt = 0 AND Users.DeleteAt = 0 GROUP BY GroupId) AS Members ON Members.GroupId = g.Id")
	}

	groupsQuery = groupsQuery.
		From("UserGroups g").
		OrderBy("g.DisplayName")

	if opts.Since > 0 {
		groupsQuery = groupsQuery.Where(sq.Gt{
			"g.UpdateAt": opts.Since,
		})
	} else {
		groupsQuery = groupsQuery.Where("g.DeleteAt = 0")
	}

	if perPage != 0 {
		groupsQuery = groupsQuery.
			Limit(uint64(perPage)).
			Offset(uint64(page * perPage))
	}

	if opts.FilterAllowReference {
		groupsQuery = groupsQuery.Where("g.AllowReference = true")
	}

	if opts.Q != "" {
		pattern := fmt.Sprintf("%%%s%%", sanitizeSearchTerm(opts.Q, "\\"))
		operatorKeyword := "ILIKE"
		if s.DriverName() == model.DATABASE_DRIVER_MYSQL {
			operatorKeyword = "LIKE"
		}
		groupsQuery = groupsQuery.Where(fmt.Sprintf("(g.Name %[1]s ? OR g.DisplayName %[1]s ?)", operatorKeyword), pattern, pattern)
	}

	if len(opts.NotAssociatedToTeam) == 26 {
		groupsQuery = groupsQuery.Where(`
			g.Id NOT IN (
				SELECT
					Id
				FROM
					UserGroups
					JOIN GroupTeams ON GroupTeams.GroupId = UserGroups.Id
				WHERE
					GroupTeams.DeleteAt = 0
					AND UserGroups.DeleteAt = 0
					AND GroupTeams.TeamId = ?
			)
		`, opts.NotAssociatedToTeam)
	}

	if len(opts.NotAssociatedToChannel) == 26 {
		groupsQuery = groupsQuery.Where(`
			g.Id NOT IN (
				SELECT
					Id
				FROM
					UserGroups
					JOIN GroupChannels ON GroupChannels.GroupId = UserGroups.Id
				WHERE
					GroupChannels.DeleteAt = 0
					AND UserGroups.DeleteAt = 0
					AND GroupChannels.ChannelId = ?
			)
		`, opts.NotAssociatedToChannel)
	}

	if opts.FilterParentTeamPermitted && len(opts.NotAssociatedToChannel) == 26 {
		groupsQuery = groupsQuery.Where(`
			CASE
			WHEN (
				SELECT
					Teams.GroupConstrained
				FROM
					Teams
					JOIN Channels ON Channels.TeamId = Teams.Id
				WHERE
					Channels.Id = ?
			) THEN g.Id IN (
				SELECT
					GroupId
				FROM
					GroupTeams
				WHERE
					GroupTeams.DeleteAt = 0
					AND GroupTeams.TeamId = (
						SELECT
							TeamId
						FROM
							Channels
						WHERE
							Id = ?
					)
			)
			ELSE TRUE
		END
		`, opts.NotAssociatedToChannel, opts.NotAssociatedToChannel)
	}

	queryString, args, err := groupsQuery.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_groups_tosql")
	}

	if _, err = s.GetReplica().Select(&groups, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Groups")
	}

	return groups, nil
}

func (s *SqlGroupStore) teamMembersMinusGroupMembersQuery(teamID string, groupIDs []string, isCount bool) sq.SelectBuilder {
	var selectStr string

	if isCount {
		selectStr = "count(DISTINCT Users.Id)"
	} else {
		tmpl := "Users.*, coalesce(TeamMembers.SchemeGuest, false), TeamMembers.SchemeAdmin, TeamMembers.SchemeUser, %s AS GroupIDs"
		if s.DriverName() == model.DATABASE_DRIVER_MYSQL {
			selectStr = fmt.Sprintf(tmpl, "group_concat(UserGroups.Id)")
		} else {
			selectStr = fmt.Sprintf(tmpl, "string_agg(UserGroups.Id, ',')")
		}
	}

	subQuery := s.getQueryBuilder().Select("GroupMembers.UserId").
		From("GroupMembers").
		Join("UserGroups ON UserGroups.Id = GroupMembers.GroupId").
		Where("GroupMembers.DeleteAt = 0").
		Where(fmt.Sprintf("GroupMembers.GroupId IN ('%s')", strings.Join(groupIDs, "', '")))

	query, _ := subQuery.MustSql()

	builder := s.getQueryBuilder().Select(selectStr).
		From("TeamMembers").
		Join("Teams ON Teams.Id = TeamMembers.TeamId").
		Join("Users ON Users.Id = TeamMembers.UserId").
		LeftJoin("Bots ON Bots.UserId = TeamMembers.UserId").
		LeftJoin("GroupMembers ON GroupMembers.UserId = Users.Id").
		LeftJoin("UserGroups ON UserGroups.Id = GroupMembers.GroupId").
		Where("TeamMembers.DeleteAt = 0").
		Where("Teams.DeleteAt = 0").
		Where("Users.DeleteAt = 0").
		Where("Bots.UserId IS NULL").
		Where("Teams.Id = ?", teamID).
		Where(fmt.Sprintf("Users.Id NOT IN (%s)", query))

	if !isCount {
		builder = builder.GroupBy("Users.Id, TeamMembers.SchemeGuest, TeamMembers.SchemeAdmin, TeamMembers.SchemeUser")
	}

	return builder
}

// TeamMembersMinusGroupMembers returns the set of users on the given team minus the set of users in the given
// groups.
func (s *SqlGroupStore) TeamMembersMinusGroupMembers(teamID string, groupIDs []string, page, perPage int) ([]*model.UserWithGroups, error) {
	query := s.teamMembersMinusGroupMembersQuery(teamID, groupIDs, false)
	query = query.OrderBy("Users.Username ASC").Limit(uint64(perPage)).Offset(uint64(page * perPage))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "team_members_minus_group_members")
	}

	var users []*model.UserWithGroups
	if _, err = s.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find UserWithGroups")
	}

	return users, nil
}

// CountTeamMembersMinusGroupMembers returns the count of the set of users on the given team minus the set of users
// in the given groups.
func (s *SqlGroupStore) CountTeamMembersMinusGroupMembers(teamID string, groupIDs []string) (int64, error) {
	queryString, args, err := s.teamMembersMinusGroupMembersQuery(teamID, groupIDs, true).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "count_team_members_minus_group_members_tosql")
	}

	var count int64
	if count, err = s.GetReplica().SelectInt(queryString, args...); err != nil {
		return 0, errors.Wrap(err, "failed to count TeamMembers minus GroupMembers")
	}

	return count, nil
}

func (s *SqlGroupStore) channelMembersMinusGroupMembersQuery(channelID string, groupIDs []string, isCount bool) sq.SelectBuilder {
	var selectStr string

	if isCount {
		selectStr = "count(DISTINCT Users.Id)"
	} else {
		tmpl := "Users.*, coalesce(ChannelMembers.SchemeGuest, false), ChannelMembers.SchemeAdmin, ChannelMembers.SchemeUser, %s AS GroupIDs"
		if s.DriverName() == model.DATABASE_DRIVER_MYSQL {
			selectStr = fmt.Sprintf(tmpl, "group_concat(UserGroups.Id)")
		} else {
			selectStr = fmt.Sprintf(tmpl, "string_agg(UserGroups.Id, ',')")
		}
	}

	subQuery := s.getQueryBuilder().Select("GroupMembers.UserId").
		From("GroupMembers").
		Join("UserGroups ON UserGroups.Id = GroupMembers.GroupId").
		Where("GroupMembers.DeleteAt = 0").
		Where(fmt.Sprintf("GroupMembers.GroupId IN ('%s')", strings.Join(groupIDs, "', '")))

	query, _ := subQuery.MustSql()

	builder := s.getQueryBuilder().Select(selectStr).
		From("ChannelMembers").
		Join("Channels ON Channels.Id = ChannelMembers.ChannelId").
		Join("Users ON Users.Id = ChannelMembers.UserId").
		LeftJoin("Bots ON Bots.UserId = ChannelMembers.UserId").
		LeftJoin("GroupMembers ON GroupMembers.UserId = Users.Id").
		LeftJoin("UserGroups ON UserGroups.Id = GroupMembers.GroupId").
		Where("Channels.DeleteAt = 0").
		Where("Users.DeleteAt = 0").
		Where("Bots.UserId IS NULL").
		Where("Channels.Id = ?", channelID).
		Where(fmt.Sprintf("Users.Id NOT IN (%s)", query))

	if !isCount {
		builder = builder.GroupBy("Users.Id, ChannelMembers.SchemeGuest, ChannelMembers.SchemeAdmin, ChannelMembers.SchemeUser")
	}

	return builder
}

// ChannelMembersMinusGroupMembers returns the set of users in the given channel minus the set of users in the given
// groups.
func (s *SqlGroupStore) ChannelMembersMinusGroupMembers(channelID string, groupIDs []string, page, perPage int) ([]*model.UserWithGroups, error) {
	query := s.channelMembersMinusGroupMembersQuery(channelID, groupIDs, false)
	query = query.OrderBy("Users.Username ASC").Limit(uint64(perPage)).Offset(uint64(page * perPage))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "channel_members_minus_group_members_tosql")
	}

	var users []*model.UserWithGroups
	if _, err = s.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find UserWithGroups")
	}

	return users, nil
}

// CountChannelMembersMinusGroupMembers returns the count of the set of users in the given channel minus the set of users
// in the given groups.
func (s *SqlGroupStore) CountChannelMembersMinusGroupMembers(channelID string, groupIDs []string) (int64, error) {
	queryString, args, err := s.channelMembersMinusGroupMembersQuery(channelID, groupIDs, true).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "count_channel_members_minus_group_members_tosql")
	}

	var count int64
	if count, err = s.GetReplica().SelectInt(queryString, args...); err != nil {
		return 0, errors.Wrap(err, "failed to count ChannelMembers")
	}

	return count, nil
}

func (s *SqlGroupStore) AdminRoleGroupsForSyncableMember(userID, syncableID string, syncableType model.GroupSyncableType) ([]string, error) {
	var groupIds []string

	query := fmt.Sprintf(`
		SELECT
			GroupMembers.GroupId
		FROM
			GroupMembers
		INNER JOIN
			Group%[1]ss ON Group%[1]ss.GroupId = GroupMembers.GroupId
		WHERE
			GroupMembers.UserId = :UserId
			AND GroupMembers.DeleteAt = 0
			AND %[1]sId = :%[1]sId
			AND Group%[1]ss.DeleteAt = 0
			AND Group%[1]ss.SchemeAdmin = TRUE`, syncableType)

	_, err := s.GetReplica().Select(&groupIds, query, map[string]interface{}{"UserId": userID, fmt.Sprintf("%sId", syncableType): syncableID})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Group ids")
	}

	return groupIds, nil
}

func (s *SqlGroupStore) PermittedSyncableAdmins(syncableID string, syncableType model.GroupSyncableType) ([]string, error) {
	builder := s.getQueryBuilder().Select("UserId").
		From(fmt.Sprintf("Group%ss", syncableType)).
		Join(fmt.Sprintf("GroupMembers ON GroupMembers.GroupId = Group%ss.GroupId AND Group%[1]ss.SchemeAdmin = TRUE AND GroupMembers.DeleteAt = 0", syncableType.String())).Where(fmt.Sprintf("Group%[1]ss.%[1]sId = ?", syncableType.String()), syncableID)

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "permitted_syncable_admins_tosql")
	}

	var userIDs []string
	if _, err = s.GetMaster().Select(&userIDs, query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find User ids")
	}

	return userIDs, nil
}

func (s *SqlGroupStore) GroupCount() (int64, error) {
	return s.countTable("UserGroups")
}

func (s *SqlGroupStore) GroupTeamCount() (int64, error) {
	return s.countTable("GroupTeams")
}

func (s *SqlGroupStore) GroupChannelCount() (int64, error) {
	return s.countTable("GroupChannels")
}

func (s *SqlGroupStore) GroupMemberCount() (int64, error) {
	return s.countTable("GroupMembers")
}

func (s *SqlGroupStore) DistinctGroupMemberCount() (int64, error) {
	return s.countTableWithSelectAndWhere("COUNT(DISTINCT UserId)", "GroupMembers", nil)
}

func (s *SqlGroupStore) GroupCountWithAllowReference() (int64, error) {
	return s.countTableWithSelectAndWhere("COUNT(*)", "UserGroups", sq.Eq{"AllowReference": true, "DeleteAt": 0})
}

func (s *SqlGroupStore) countTable(tableName string) (int64, error) {
	return s.countTableWithSelectAndWhere("COUNT(*)", tableName, nil)
}

func (s *SqlGroupStore) countTableWithSelectAndWhere(selectStr, tableName string, whereStmt map[string]interface{}) (int64, error) {
	if whereStmt == nil {
		whereStmt = sq.Eq{"DeleteAt": 0}
	}

	query := s.getQueryBuilder().Select(selectStr).From(tableName).Where(whereStmt)

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "count_table_with_select_and_where_tosql")
	}

	count, err := s.GetReplica().SelectInt(sql, args...)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to count from table %s", tableName)
	}

	return count, nil
}
