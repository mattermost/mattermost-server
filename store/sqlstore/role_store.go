// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/gorp"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SqlRoleStore struct {
	*SqlStore
}

type Role struct {
	ID            string
	Name          string
	DisplayName   string
	Description   string
	CreateAt      int64
	UpdateAt      int64
	DeleteAt      int64
	Permissions   string
	SchemeManaged bool
	BuiltIn       bool
}

type channelRolesPermissions struct {
	GuestRoleName                string
	UserRoleName                 string
	AdminRoleName                string
	HigherScopedGuestPermissions string
	HigherScopedUserPermissions  string
	HigherScopedAdminPermissions string
}

func NewRoleFromModel(role *model.Role) *Role {
	permissionsMap := make(map[string]bool)
	permissions := ""

	for _, permission := range role.Permissions {
		if !permissionsMap[permission] {
			permissions += fmt.Sprintf(" %v", permission)
			permissionsMap[permission] = true
		}
	}

	return &Role{
		ID:            role.ID,
		Name:          role.Name,
		DisplayName:   role.DisplayName,
		Description:   role.Description,
		CreateAt:      role.CreateAt,
		UpdateAt:      role.UpdateAt,
		DeleteAt:      role.DeleteAt,
		Permissions:   permissions,
		SchemeManaged: role.SchemeManaged,
		BuiltIn:       role.BuiltIn,
	}
}

func (role Role) ToModel() *model.Role {
	return &model.Role{
		ID:            role.ID,
		Name:          role.Name,
		DisplayName:   role.DisplayName,
		Description:   role.Description,
		CreateAt:      role.CreateAt,
		UpdateAt:      role.UpdateAt,
		DeleteAt:      role.DeleteAt,
		Permissions:   strings.Fields(role.Permissions),
		SchemeManaged: role.SchemeManaged,
		BuiltIn:       role.BuiltIn,
	}
}

func newSqlRoleStore(sqlStore *SqlStore) store.RoleStore {
	s := &SqlRoleStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(Role{}, "Roles").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("Name").SetMaxSize(64).SetUnique(true)
		table.ColMap("DisplayName").SetMaxSize(128)
		table.ColMap("Description").SetMaxSize(1024)
		table.ColMap("Permissions")
	}
	return s
}

func (s *SqlRoleStore) Save(role *model.Role) (*model.Role, error) {
	// Check the role is valid before proceeding.
	if !role.IsValidWithoutID() {
		return nil, store.NewErrInvalidInput("Role", "<any>", fmt.Sprintf("%v", role))
	}

	if role.ID == "" {
		transaction, err := s.GetMaster().Begin()
		if err != nil {
			return nil, errors.Wrap(err, "begin_transaction")
		}
		defer finalizeTransaction(transaction)
		createdRole, err := s.createRole(role, transaction)
		if err != nil {
			_ = transaction.Rollback()
			return nil, errors.Wrap(err, "unable to create Role")
		} else if err := transaction.Commit(); err != nil {
			return nil, errors.Wrap(err, "commit_transaction")
		}
		return createdRole, nil
	}

	dbRole := NewRoleFromModel(role)
	dbRole.UpdateAt = model.GetMillis()
	if rowsChanged, err := s.GetMaster().Update(dbRole); err != nil {
		return nil, errors.Wrap(err, "failed to update Role")
	} else if rowsChanged != 1 {
		return nil, fmt.Errorf("invalid number of updated rows, expected 1 but got %d", rowsChanged)
	}

	return dbRole.ToModel(), nil
}

func (s *SqlRoleStore) createRole(role *model.Role, transaction *gorp.Transaction) (*model.Role, error) {
	// Check the role is valid before proceeding.
	if !role.IsValidWithoutID() {
		return nil, store.NewErrInvalidInput("Role", "<any>", fmt.Sprintf("%v", role))
	}

	dbRole := NewRoleFromModel(role)

	dbRole.ID = model.NewID()
	dbRole.CreateAt = model.GetMillis()
	dbRole.UpdateAt = dbRole.CreateAt

	if err := transaction.Insert(dbRole); err != nil {
		return nil, errors.Wrap(err, "failed to save Role")
	}

	return dbRole.ToModel(), nil
}

func (s *SqlRoleStore) Get(roleID string) (*model.Role, error) {
	var dbRole Role

	if err := s.GetReplica().SelectOne(&dbRole, "SELECT * from Roles WHERE Id = :Id", map[string]interface{}{"Id": roleID}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Role", roleID)
		}
		return nil, errors.Wrap(err, "failed to get Role")
	}

	return dbRole.ToModel(), nil
}

func (s *SqlRoleStore) GetAll() ([]*model.Role, error) {
	var dbRoles []Role

	if _, err := s.GetReplica().Select(&dbRoles, "SELECT * from Roles", map[string]interface{}{}); err != nil {
		return nil, errors.Wrap(err, "failed to find Roles")
	}

	var roles []*model.Role
	for _, dbRole := range dbRoles {
		roles = append(roles, dbRole.ToModel())
	}
	return roles, nil
}

func (s *SqlRoleStore) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var dbRole Role
	if err := s.DBFromContext(ctx).SelectOne(&dbRole, "SELECT * from Roles WHERE Name = :Name", map[string]interface{}{"Name": name}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Role", fmt.Sprintf("name=%s", name))
		}
		return nil, errors.Wrapf(err, "failed to find Roles with name=%s", name)
	}

	return dbRole.ToModel(), nil
}

func (s *SqlRoleStore) GetByNames(names []string) ([]*model.Role, error) {
	if len(names) == 0 {
		return []*model.Role{}, nil
	}

	query := s.getQueryBuilder().
		Select("Id, Name, DisplayName, Description, CreateAt, UpdateAt, DeleteAt, Permissions, SchemeManaged, BuiltIn").
		From("Roles").
		Where(sq.Eq{"Name": names})
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "role_tosql")
	}

	rows, err := s.GetReplica().Db.Query(queryString, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Roles")
	}

	var roles []*model.Role
	defer rows.Close()
	for rows.Next() {
		var role Role
		err = rows.Scan(
			&role.ID, &role.Name, &role.DisplayName, &role.Description,
			&role.CreateAt, &role.UpdateAt, &role.DeleteAt, &role.Permissions,
			&role.SchemeManaged, &role.BuiltIn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan values")
		}
		roles = append(roles, role.ToModel())
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "unable to iterate over rows")
	}

	return roles, nil
}

func (s *SqlRoleStore) Delete(roleID string) (*model.Role, error) {
	// Get the role.
	var role *Role
	if err := s.GetReplica().SelectOne(&role, "SELECT * from Roles WHERE Id = :Id", map[string]interface{}{"Id": roleID}); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Role", roleID)
		}
		return nil, errors.Wrapf(err, "failed to get Role with id=%s", roleID)
	}

	time := model.GetMillis()
	role.DeleteAt = time
	role.UpdateAt = time

	if rowsChanged, err := s.GetMaster().Update(role); err != nil {
		return nil, errors.Wrap(err, "failed to update Role")
	} else if rowsChanged != 1 {
		return nil, errors.Wrapf(err, "invalid number of updated rows, expected 1 but got %d", rowsChanged)
	}
	return role.ToModel(), nil
}

func (s *SqlRoleStore) PermanentDeleteAll() error {
	if _, err := s.GetMaster().Exec("DELETE FROM Roles"); err != nil {
		return errors.Wrap(err, "failed to delete Roles")
	}

	return nil
}

func (s *SqlRoleStore) channelHigherScopedPermissionsQuery(roleNames []string) string {
	sqlTmpl := `
		SELECT
			'' AS GuestRoleName,
			RoleSchemes.DefaultChannelUserRole AS UserRoleName,
			RoleSchemes.DefaultChannelAdminRole AS AdminRoleName,
			'' AS HigherScopedGuestPermissions,
			UserRoles.Permissions AS HigherScopedUserPermissions,
			AdminRoles.Permissions AS HigherScopedAdminPermissions
		FROM
			Schemes AS RoleSchemes
			JOIN Channels ON Channels.SchemeId = RoleSchemes.Id
			JOIN Teams ON Teams.Id = Channels.TeamId
			JOIN Schemes ON Schemes.Id = Teams.SchemeId
			RIGHT JOIN Roles AS UserRoles ON UserRoles.Name = Schemes.DefaultChannelUserRole
			RIGHT JOIN Roles AS AdminRoles ON AdminRoles.Name = Schemes.DefaultChannelAdminRole
		WHERE
			RoleSchemes.DefaultChannelUserRole IN ('%[1]s')
			OR RoleSchemes.DefaultChannelAdminRole IN ('%[1]s')

		UNION

		SELECT
			RoleSchemes.DefaultChannelGuestRole AS GuestRoleName,
			'' AS UserRoleName,
			'' AS AdminRoleName,
			GuestRoles.Permissions AS HigherScopedGuestPermissions,
			'' AS HigherScopedUserPermissions,
			'' AS HigherScopedAdminPermissions
		FROM
			Schemes AS RoleSchemes
			JOIN Channels ON Channels.SchemeId = RoleSchemes.Id
			JOIN Teams ON Teams.Id = Channels.TeamId
			JOIN Schemes ON Schemes.Id = Teams.SchemeId
			RIGHT JOIN Roles AS GuestRoles ON GuestRoles.Name = Schemes.DefaultChannelGuestRole
		WHERE
			RoleSchemes.DefaultChannelGuestRole IN ('%[1]s')

		UNION

		SELECT
			Schemes.DefaultChannelGuestRole AS GuestRoleName,
			Schemes.DefaultChannelUserRole AS UserRoleName,
			Schemes.DefaultChannelAdminRole AS AdminRoleName,
			GuestRoles.Permissions AS HigherScopedGuestPermissions,
			UserRoles.Permissions AS HigherScopedUserPermissions,
			AdminRoles.Permissions AS HigherScopedAdminPermissions
		FROM
			Schemes
			JOIN Channels ON Channels.SchemeId = Schemes.Id
			JOIN Teams ON Teams.Id = Channels.TeamId
			JOIN Roles AS GuestRoles ON GuestRoles.Name = '%[2]s'
			JOIN Roles AS UserRoles ON UserRoles.Name = '%[3]s'
			JOIN Roles AS AdminRoles ON AdminRoles.Name = '%[4]s'
		WHERE
			(Schemes.DefaultChannelGuestRole IN ('%[1]s')
			OR Schemes.DefaultChannelUserRole IN ('%[1]s')
			OR Schemes.DefaultChannelAdminRole IN ('%[1]s'))
		AND (Teams.SchemeId = ''
			OR Teams.SchemeId IS NULL)
	`

	// The below three channel role names are referenced by their name value because there is no system scheme
	// record that ships with Mattermost, otherwise the system scheme would be referenced by name and the channel
	// roles would be referenced by their column names.
	return fmt.Sprintf(
		sqlTmpl,
		strings.Join(roleNames, "', '"),
		model.ChannelGuestRoleID,
		model.ChannelUserRoleID,
		model.ChannelAdminRoleID,
	)
}

func (s *SqlRoleStore) ChannelHigherScopedPermissions(roleNames []string) (map[string]*model.RolePermissions, error) {
	query := s.channelHigherScopedPermissionsQuery(roleNames)

	var rolesPermissions []*channelRolesPermissions
	if _, err := s.GetReplica().Select(&rolesPermissions, query); err != nil {
		return nil, errors.Wrap(err, "failed to find RolePermissions")
	}

	roleNameHigherScopedPermissions := map[string]*model.RolePermissions{}

	for _, rp := range rolesPermissions {
		roleNameHigherScopedPermissions[rp.GuestRoleName] = &model.RolePermissions{RoleID: model.ChannelGuestRoleID, Permissions: strings.Split(rp.HigherScopedGuestPermissions, " ")}
		roleNameHigherScopedPermissions[rp.UserRoleName] = &model.RolePermissions{RoleID: model.ChannelUserRoleID, Permissions: strings.Split(rp.HigherScopedUserPermissions, " ")}
		roleNameHigherScopedPermissions[rp.AdminRoleName] = &model.RolePermissions{RoleID: model.ChannelAdminRoleID, Permissions: strings.Split(rp.HigherScopedAdminPermissions, " ")}
	}

	return roleNameHigherScopedPermissions, nil
}

func (s *SqlRoleStore) AllChannelSchemeRoles() ([]*model.Role, error) {
	query := s.getQueryBuilder().
		Select("Roles.*").
		From("Schemes").
		Join("Roles ON Schemes.DefaultChannelGuestRole = Roles.Name OR Schemes.DefaultChannelUserRole = Roles.Name OR Schemes.DefaultChannelAdminRole = Roles.Name").
		Where(sq.Eq{"Schemes.Scope": model.SchemeScopeChannel}).
		Where(sq.Eq{"Roles.DeleteAt": 0}).
		Where(sq.Eq{"Schemes.DeleteAt": 0})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "role_tosql")
	}

	var dbRoles []*Role
	if _, err = s.GetReplica().Select(&dbRoles, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Roles")
	}

	var roles []*model.Role
	for _, dbRole := range dbRoles {
		roles = append(roles, dbRole.ToModel())
	}

	return roles, nil
}

// ChannelRolesUnderTeamRole finds all of the channel-scheme roles under the team of the given team-scheme role.
func (s *SqlRoleStore) ChannelRolesUnderTeamRole(roleName string) ([]*model.Role, error) {
	query := s.getQueryBuilder().
		Select("ChannelSchemeRoles.*").
		From("Roles AS HigherScopedRoles").
		Join("Schemes AS HigherScopedSchemes ON (HigherScopedRoles.Name = HigherScopedSchemes.DefaultChannelGuestRole OR HigherScopedRoles.Name = HigherScopedSchemes.DefaultChannelUserRole OR HigherScopedRoles.Name = HigherScopedSchemes.DefaultChannelAdminRole)").
		Join("Teams ON Teams.SchemeId = HigherScopedSchemes.Id").
		Join("Channels ON Channels.TeamId = Teams.Id").
		Join("Schemes AS ChannelSchemes ON Channels.SchemeId = ChannelSchemes.Id").
		Join("Roles AS ChannelSchemeRoles ON (ChannelSchemeRoles.Name = ChannelSchemes.DefaultChannelGuestRole OR ChannelSchemeRoles.Name = ChannelSchemes.DefaultChannelUserRole OR ChannelSchemeRoles.Name = ChannelSchemes.DefaultChannelAdminRole)").
		Where(sq.Eq{"HigherScopedSchemes.Scope": model.SchemeScopeTeam}).
		Where(sq.Eq{"HigherScopedRoles.Name": roleName}).
		Where(sq.Eq{"HigherScopedRoles.DeleteAt": 0}).
		Where(sq.Eq{"HigherScopedSchemes.DeleteAt": 0}).
		Where(sq.Eq{"Teams.DeleteAt": 0}).
		Where(sq.Eq{"Channels.DeleteAt": 0}).
		Where(sq.Eq{"ChannelSchemes.DeleteAt": 0}).
		Where(sq.Eq{"ChannelSchemeRoles.DeleteAt": 0})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "role_tosql")
	}

	var dbRoles []*Role
	if _, err = s.GetReplica().Select(&dbRoles, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Roles")
	}

	var roles []*model.Role
	for _, dbRole := range dbRoles {
		roles = append(roles, dbRole.ToModel())
	}

	return roles, nil
}
