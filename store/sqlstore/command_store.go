// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SqlCommandStore struct {
	*SqlStore

	commandsQuery sq.SelectBuilder
}

func newSqlCommandStore(sqlStore *SqlStore) store.CommandStore {
	s := &SqlCommandStore{SqlStore: sqlStore}

	s.commandsQuery = s.getQueryBuilder().
		Select("*").
		From("Commands")
	for _, db := range sqlStore.GetAllConns() {
		tableo := db.AddTableWithName(model.Command{}, "Commands").SetKeys(false, "Id")
		tableo.ColMap("Id").SetMaxSize(26)
		tableo.ColMap("Token").SetMaxSize(26)
		tableo.ColMap("CreatorId").SetMaxSize(26)
		tableo.ColMap("TeamId").SetMaxSize(26)
		tableo.ColMap("Trigger").SetMaxSize(128)
		tableo.ColMap("URL").SetMaxSize(1024)
		tableo.ColMap("Method").SetMaxSize(1)
		tableo.ColMap("Username").SetMaxSize(64)
		tableo.ColMap("IconURL").SetMaxSize(1024)
		tableo.ColMap("AutoCompleteDesc").SetMaxSize(1024)
		tableo.ColMap("AutoCompleteHint").SetMaxSize(1024)
		tableo.ColMap("DisplayName").SetMaxSize(64)
		tableo.ColMap("Description").SetMaxSize(128)
		tableo.ColMap("PluginId").SetMaxSize(190)
	}

	return s
}

func (s SqlCommandStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_command_team_id", "Commands", "TeamId")
	s.CreateIndexIfNotExists("idx_command_update_at", "Commands", "UpdateAt")
	s.CreateIndexIfNotExists("idx_command_create_at", "Commands", "CreateAt")
	s.CreateIndexIfNotExists("idx_command_delete_at", "Commands", "DeleteAt")
}

func (s SqlCommandStore) Save(command *model.Command) (*model.Command, error) {
	if command.ID != "" {
		return nil, store.NewErrInvalidInput("Command", "CommandId", command.ID)
	}

	command.PreSave()
	if err := command.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(command); err != nil {
		return nil, errors.Wrapf(err, "insert: command_id=%s", command.ID)
	}

	return command, nil
}

func (s SqlCommandStore) Get(id string) (*model.Command, error) {
	var command model.Command

	query, args, err := s.commandsQuery.
		Where(sq.Eq{"Id": id, "DeleteAt": 0}).ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "commands_tosql")
	}
	if err = s.GetReplica().SelectOne(&command, query, args...); err == sql.ErrNoRows {
		return nil, store.NewErrNotFound("Command", id)
	} else if err != nil {
		return nil, errors.Wrapf(err, "selectone: command_id=%s", id)
	}

	return &command, nil
}

func (s SqlCommandStore) GetByTeam(teamID string) ([]*model.Command, error) {
	var commands []*model.Command

	sql, args, err := s.commandsQuery.
		Where(sq.Eq{"TeamId": teamID, "DeleteAt": 0}).ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "commands_tosql")
	}
	if _, err := s.GetReplica().Select(&commands, sql, args...); err != nil {
		return nil, errors.Wrapf(err, "select: team_id=%s", teamID)
	}

	return commands, nil
}

func (s SqlCommandStore) GetByTrigger(teamID string, trigger string) (*model.Command, error) {
	var command model.Command
	var triggerStr string
	if s.DriverName() == "mysql" {
		triggerStr = "`Trigger`"
	} else {
		triggerStr = "\"trigger\""
	}

	query, args, err := s.commandsQuery.
		Where(sq.Eq{"TeamId": teamID, "DeleteAt": 0, triggerStr: trigger}).ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "commands_tosql")
	}

	if err := s.GetReplica().SelectOne(&command, query, args...); err == sql.ErrNoRows {
		errorID := "teamId=" + teamID + ", trigger=" + trigger
		return nil, store.NewErrNotFound("Command", errorID)
	} else if err != nil {
		return nil, errors.Wrapf(err, "selectone: team_id=%s, trigger=%s", teamID, trigger)
	}

	return &command, nil
}

func (s SqlCommandStore) Delete(commandID string, time int64) error {
	sql, args, err := s.getQueryBuilder().
		Update("Commands").
		SetMap(sq.Eq{"DeleteAt": time, "UpdateAt": time}).
		Where(sq.Eq{"Id": commandID}).ToSql()
	if err != nil {
		return errors.Wrapf(err, "commands_tosql")
	}

	_, err = s.GetMaster().Exec(sql, args...)
	if err != nil {
		errors.Wrapf(err, "delete: command_id=%s", commandID)
	}

	return nil
}

func (s SqlCommandStore) PermanentDeleteByTeam(teamID string) error {
	sql, args, err := s.getQueryBuilder().
		Delete("Commands").
		Where(sq.Eq{"TeamId": teamID}).ToSql()
	if err != nil {
		return errors.Wrapf(err, "commands_tosql")
	}
	_, err = s.GetMaster().Exec(sql, args...)
	if err != nil {
		return errors.Wrapf(err, "delete: team_id=%s", teamID)
	}
	return nil
}

func (s SqlCommandStore) PermanentDeleteByUser(userID string) error {
	sql, args, err := s.getQueryBuilder().
		Delete("Commands").
		Where(sq.Eq{"CreatorId": userID}).ToSql()
	if err != nil {
		return errors.Wrapf(err, "commands_tosql")
	}
	_, err = s.GetMaster().Exec(sql, args...)
	if err != nil {
		return errors.Wrapf(err, "delete: user_id=%s", userID)
	}

	return nil
}

func (s SqlCommandStore) Update(cmd *model.Command) (*model.Command, error) {
	cmd.UpdateAt = model.GetMillis()

	if err := cmd.IsValid(); err != nil {
		return nil, err
	}

	if _, err := s.GetMaster().Update(cmd); err != nil {
		return nil, errors.Wrapf(err, "update: command_id=%s", cmd.ID)
	}

	return cmd, nil
}

func (s SqlCommandStore) AnalyticsCommandCount(teamID string) (int64, error) {
	query := s.getQueryBuilder().
		Select("COUNT(*)").
		From("Commands").
		Where(sq.Eq{"DeleteAt": 0})

	if teamID != "" {
		query = query.Where(sq.Eq{"TeamId": teamID})
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, errors.Wrapf(err, "commands_tosql")
	}

	c, err := s.GetReplica().SelectInt(sql, args...)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to count the commands: team_id=%s", teamID)
	}
	return c, nil
}
