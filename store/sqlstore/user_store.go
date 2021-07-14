// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/gorp"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	MaxGroupChannelsForProfiles = 50
)

var (
	UserSearchTypeNames_NO_FULL_NAME = []string{"Username", "Nickname"}
	UserSearchTypeNames              = []string{"Username", "FirstName", "LastName", "Nickname"}
	UserSearchTypeAll_NO_FULL_NAME   = []string{"Username", "Nickname", "Email"}
	UserSearchTypeAll                = []string{"Username", "FirstName", "LastName", "Nickname", "Email"}
)

type SqlUserStore struct {
	*SqlStore
	metrics einterfaces.MetricsInterface

	// usersQuery is a starting point for all queries that return one or more Users.
	usersQuery sq.SelectBuilder
}

func (us *SqlUserStore) ClearCaches() {}

func (us SqlUserStore) InvalidateProfileCacheForUser(userId string) {}

func newSqlUserStore(sqlStore *SqlStore, metrics einterfaces.MetricsInterface) store.UserStore {
	us := &SqlUserStore{
		SqlStore: sqlStore,
		metrics:  metrics,
	}

	// note: we are providing field names explicitly here to maintain order of columns (needed when using raw queries)
	us.usersQuery = us.getQueryBuilder().
		Select("u.Id", "u.CreateAt", "u.UpdateAt", "u.DeleteAt", "u.Username", "u.Password", "u.AuthData", "u.AuthService", "u.Email", "u.EmailVerified", "u.Nickname", "u.FirstName", "u.LastName", "u.Position", "u.Roles", "u.AllowMarketing", "u.Props", "u.NotifyProps", "u.LastPasswordUpdate", "u.LastPictureUpdate", "u.FailedAttempts", "u.Locale", "u.Timezone", "u.MfaActive", "u.MfaSecret",
			"b.UserId IS NOT NULL AS IsBot", "COALESCE(b.Description, '') AS BotDescription", "COALESCE(b.LastIconUpdate, 0) AS BotLastIconUpdate", "u.RemoteId").
		From("Users u").
		LeftJoin("Bots b ON ( b.UserId = u.Id )")

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.User{}, "Users").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("Username").SetMaxSize(64).SetUnique(true)
		table.ColMap("Password").SetMaxSize(128)
		table.ColMap("AuthData").SetMaxSize(128).SetUnique(true)
		table.ColMap("AuthService").SetMaxSize(32)
		table.ColMap("Email").SetMaxSize(128).SetUnique(true)
		table.ColMap("Nickname").SetMaxSize(64)
		table.ColMap("FirstName").SetMaxSize(64)
		table.ColMap("LastName").SetMaxSize(64)
		table.ColMap("Roles").SetMaxSize(256)
		table.ColMap("Props").SetMaxSize(4000)
		table.ColMap("NotifyProps").SetMaxSize(2000)
		table.ColMap("Locale").SetMaxSize(5)
		table.ColMap("MfaSecret").SetMaxSize(128)
		table.ColMap("RemoteId").SetMaxSize(26)
		table.ColMap("Position").SetMaxSize(128)
		table.ColMap("Timezone").SetMaxSize(256)
	}

	return us
}

func (us SqlUserStore) createIndexesIfNotExists() {
	us.CreateIndexIfNotExists("idx_users_update_at", "Users", "UpdateAt")
	us.CreateIndexIfNotExists("idx_users_create_at", "Users", "CreateAt")
	us.CreateIndexIfNotExists("idx_users_delete_at", "Users", "DeleteAt")

	if us.DriverName() == model.DATABASE_DRIVER_POSTGRES {
		us.CreateIndexIfNotExists("idx_users_email_lower_textpattern", "Users", "lower(Email) text_pattern_ops")
		us.CreateIndexIfNotExists("idx_users_username_lower_textpattern", "Users", "lower(Username) text_pattern_ops")
		us.CreateIndexIfNotExists("idx_users_nickname_lower_textpattern", "Users", "lower(Nickname) text_pattern_ops")
		us.CreateIndexIfNotExists("idx_users_firstname_lower_textpattern", "Users", "lower(FirstName) text_pattern_ops")
		us.CreateIndexIfNotExists("idx_users_lastname_lower_textpattern", "Users", "lower(LastName) text_pattern_ops")
	}

	us.CreateFullTextIndexIfNotExists("idx_users_all_txt", "Users", strings.Join(UserSearchTypeAll, ", "))
	us.CreateFullTextIndexIfNotExists("idx_users_all_no_full_name_txt", "Users", strings.Join(UserSearchTypeAll_NO_FULL_NAME, ", "))
	us.CreateFullTextIndexIfNotExists("idx_users_names_txt", "Users", strings.Join(UserSearchTypeNames, ", "))
	us.CreateFullTextIndexIfNotExists("idx_users_names_no_full_name_txt", "Users", strings.Join(UserSearchTypeNames_NO_FULL_NAME, ", "))
}

func (us SqlUserStore) Save(user *model.User) (*model.User, error) {
	if user.Id != "" && !user.IsRemote() {
		return nil, store.NewErrInvalidInput("User", "id", user.Id)
	}

	user.PreSave()
	if err := user.IsValid(); err != nil {
		return nil, err
	}

	if err := us.GetMaster().Insert(user); err != nil {
		if IsUniqueConstraintError(err, []string{"Email", "users_email_key", "idx_users_email_unique"}) {
			return nil, store.NewErrInvalidInput("User", "email", user.Email)
		}
		if IsUniqueConstraintError(err, []string{"Username", "users_username_key", "idx_users_username_unique"}) {
			return nil, store.NewErrInvalidInput("User", "username", user.Username)
		}
		return nil, errors.Wrapf(err, "failed to save User with userId=%s", user.Id)
	}

	return user, nil
}

func (us SqlUserStore) DeactivateGuests() ([]string, error) {
	curTime := model.GetMillis()
	updateQuery := us.getQueryBuilder().Update("Users").
		Set("UpdateAt", curTime).
		Set("DeleteAt", curTime).
		Where(sq.Eq{"Roles": "system_guest"}).
		Where(sq.Eq{"DeleteAt": 0})

	queryString, args, err := updateQuery.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "deactivate_guests_tosql")
	}

	_, err = us.GetMaster().Exec(queryString, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update Users with roles=system_guest")
	}

	selectQuery := us.getQueryBuilder().Select("Id").From("Users").Where(sq.Eq{"DeleteAt": curTime})

	queryString, args, err = selectQuery.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "deactivate_guests_tosql")
	}

	userIds := []string{}
	_, err = us.GetMaster().Select(&userIds, queryString, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	return userIds, nil
}

func (us SqlUserStore) Update(user *model.User, trustedUpdateData bool) (*model.UserUpdate, error) {
	user.PreUpdate()

	if err := user.IsValid(); err != nil {
		return nil, err
	}

	oldUserResult, err := us.GetMaster().Get(model.User{}, user.Id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get User with userId=%s", user.Id)
	}

	if oldUserResult == nil {
		return nil, store.NewErrInvalidInput("User", "id", user.Id)
	}

	oldUser := oldUserResult.(*model.User)
	user.CreateAt = oldUser.CreateAt
	user.AuthData = oldUser.AuthData
	user.AuthService = oldUser.AuthService
	user.Password = oldUser.Password
	user.LastPasswordUpdate = oldUser.LastPasswordUpdate
	user.LastPictureUpdate = oldUser.LastPictureUpdate
	user.EmailVerified = oldUser.EmailVerified
	user.FailedAttempts = oldUser.FailedAttempts
	user.MfaSecret = oldUser.MfaSecret
	user.MfaActive = oldUser.MfaActive

	if !trustedUpdateData {
		user.Roles = oldUser.Roles
		user.DeleteAt = oldUser.DeleteAt
	}

	if user.IsOAuthUser() {
		if !trustedUpdateData {
			user.Email = oldUser.Email
		}
	} else if user.IsLDAPUser() && !trustedUpdateData {
		if user.Username != oldUser.Username || user.Email != oldUser.Email {
			return nil, store.NewErrInvalidInput("User", "id", user.Id)
		}
	} else if user.Email != oldUser.Email {
		user.EmailVerified = false
	}

	if user.Username != oldUser.Username {
		user.UpdateMentionKeysFromUsername(oldUser.Username)
	}

	count, err := us.GetMaster().Update(user)
	if err != nil {
		if IsUniqueConstraintError(err, []string{"Email", "users_email_key", "idx_users_email_unique"}) {
			return nil, store.NewErrConflict("Email", err, user.Email)
		}
		if IsUniqueConstraintError(err, []string{"Username", "users_username_key", "idx_users_username_unique"}) {
			return nil, store.NewErrConflict("Username", err, user.Username)
		}
		return nil, errors.Wrapf(err, "failed to update User with userId=%s", user.Id)
	}

	if count > 1 {
		return nil, fmt.Errorf("multiple users were update: userId=%s, count=%d", user.Id, count)
	}

	user.Sanitize(map[string]bool{})
	oldUser.Sanitize(map[string]bool{})
	return &model.UserUpdate{New: user, Old: oldUser}, nil
}

func (us SqlUserStore) UpdateLastPictureUpdate(userId string) error {
	curTime := model.GetMillis()

	if _, err := us.GetMaster().Exec("UPDATE Users SET LastPictureUpdate = :Time, UpdateAt = :Time WHERE Id = :UserId", map[string]interface{}{"Time": curTime, "UserId": userId}); err != nil {
		return errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}

	return nil
}

func (us SqlUserStore) ResetLastPictureUpdate(userId string) error {
	curTime := model.GetMillis()

	if _, err := us.GetMaster().Exec("UPDATE Users SET LastPictureUpdate = :PictureUpdateTime, UpdateAt = :UpdateTime WHERE Id = :UserId", map[string]interface{}{"PictureUpdateTime": 0, "UpdateTime": curTime, "UserId": userId}); err != nil {
		return errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}

	return nil
}

func (us SqlUserStore) UpdateUpdateAt(userId string) (int64, error) {
	curTime := model.GetMillis()

	if _, err := us.GetMaster().Exec("UPDATE Users SET UpdateAt = :Time WHERE Id = :UserId", map[string]interface{}{"Time": curTime, "UserId": userId}); err != nil {
		return curTime, errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}

	return curTime, nil
}

func (us SqlUserStore) UpdatePassword(userId, hashedPassword string) error {
	updateAt := model.GetMillis()

	if _, err := us.GetMaster().Exec("UPDATE Users SET Password = :Password, LastPasswordUpdate = :LastPasswordUpdate, UpdateAt = :UpdateAt, AuthData = NULL, AuthService = '', FailedAttempts = 0 WHERE Id = :UserId", map[string]interface{}{"Password": hashedPassword, "LastPasswordUpdate": updateAt, "UpdateAt": updateAt, "UserId": userId}); err != nil {
		return errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}

	return nil
}

func (us SqlUserStore) UpdateFailedPasswordAttempts(userId string, attempts int) error {
	if _, err := us.GetMaster().Exec("UPDATE Users SET FailedAttempts = :FailedAttempts WHERE Id = :UserId", map[string]interface{}{"FailedAttempts": attempts, "UserId": userId}); err != nil {
		return errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}

	return nil
}

func (us SqlUserStore) UpdateAuthData(userId string, service string, authData *string, email string, resetMfa bool) (string, error) {
	updateAt := model.GetMillis()

	query := `
			UPDATE
			     Users
			SET
			     Password = '',
			     LastPasswordUpdate = :LastPasswordUpdate,
			     UpdateAt = :UpdateAt,
			     FailedAttempts = 0,
			     AuthService = :AuthService,
			     AuthData = :AuthData`

	if email != "" {
		query += ", Email = lower(:Email)"
	}

	if resetMfa {
		query += ", MfaActive = false, MfaSecret = ''"
	}

	query += " WHERE Id = :UserId"

	if _, err := us.GetMaster().Exec(query, map[string]interface{}{"LastPasswordUpdate": updateAt, "UpdateAt": updateAt, "UserId": userId, "AuthService": service, "AuthData": authData, "Email": email}); err != nil {
		if IsUniqueConstraintError(err, []string{"Email", "users_email_key", "idx_users_email_unique", "AuthData", "users_authdata_key"}) {
			return "", store.NewErrInvalidInput("User", "id", userId)
		}
		return "", errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}
	return userId, nil
}

// ResetAuthDataToEmailForUsers resets the AuthData of users whose AuthService
// is |service| to their Email. If userIDs is non-empty, only the users whose
// IDs are in userIDs will be affected. If dryRun is true, only the number
// of users who *would* be affected is returned; otherwise, the number of
// users who actually were affected is returned.
func (us SqlUserStore) ResetAuthDataToEmailForUsers(service string, userIDs []string, includeDeleted bool, dryRun bool) (int, error) {
	whereEquals := sq.Eq{"AuthService": service}
	if len(userIDs) > 0 {
		whereEquals["Id"] = userIDs
	}
	if !includeDeleted {
		whereEquals["DeleteAt"] = 0
	}

	if dryRun {
		builder := us.getQueryBuilder().
			Select("COUNT(*)").
			From("Users").
			Where(whereEquals)
		query, args, err := builder.ToSql()
		if err != nil {
			return 0, errors.Wrap(err, "select_count_users_tosql")
		}
		numAffected, err := us.GetReplica().SelectInt(query, args...)
		return int(numAffected), err
	}
	builder := us.getQueryBuilder().
		Update("Users").
		Set("AuthData", sq.Expr("Email")).
		Where(whereEquals)
	query, args, err := builder.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "update_users_tosql")
	}
	result, err := us.GetMaster().Exec(query, args...)
	if err != nil {
		return 0, errors.Wrap(err, "failed to update users' AuthData")
	}
	numAffected, err := result.RowsAffected()
	return int(numAffected), err
}

func (us SqlUserStore) UpdateMfaSecret(userId, secret string) error {
	updateAt := model.GetMillis()

	if _, err := us.GetMaster().Exec("UPDATE Users SET MfaSecret = :Secret, UpdateAt = :UpdateAt WHERE Id = :UserId", map[string]interface{}{"Secret": secret, "UpdateAt": updateAt, "UserId": userId}); err != nil {
		return errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}

	return nil
}

func (us SqlUserStore) UpdateMfaActive(userId string, active bool) error {
	updateAt := model.GetMillis()

	if _, err := us.GetMaster().Exec("UPDATE Users SET MfaActive = :Active, UpdateAt = :UpdateAt WHERE Id = :UserId", map[string]interface{}{"Active": active, "UpdateAt": updateAt, "UserId": userId}); err != nil {
		return errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}

	return nil
}

// GetMany returns a list of users for the provided list of ids
func (us SqlUserStore) GetMany(ctx context.Context, ids []string) ([]*model.User, error) {
	query := us.usersQuery.Where(sq.Eq{"Id": ids})
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "users_get_many_tosql")
	}

	var users []*model.User
	if _, err := us.SqlStore.DBFromContext(ctx).Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "users_get_many_select")
	}

	return users, nil
}

func (us SqlUserStore) Get(ctx context.Context, id string) (*model.User, error) {
	query := us.usersQuery.Where("Id = ?", id)
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "users_get_tosql")
	}
	row := us.SqlStore.DBFromContext(ctx).Db.QueryRow(queryString, args...)

	var user model.User
	var props, notifyProps, timezone []byte
	err = row.Scan(&user.Id, &user.CreateAt, &user.UpdateAt, &user.DeleteAt, &user.Username,
		&user.Password, &user.AuthData, &user.AuthService, &user.Email, &user.EmailVerified,
		&user.Nickname, &user.FirstName, &user.LastName, &user.Position, &user.Roles,
		&user.AllowMarketing, &props, &notifyProps, &user.LastPasswordUpdate, &user.LastPictureUpdate,
		&user.FailedAttempts, &user.Locale, &timezone, &user.MfaActive, &user.MfaSecret,
		&user.IsBot, &user.BotDescription, &user.BotLastIconUpdate, &user.RemoteId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("User", id)
		}
		return nil, errors.Wrapf(err, "failed to get User with userId=%s", id)

	}
	if err = json.Unmarshal(props, &user.Props); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal user props")
	}
	if err = json.Unmarshal(notifyProps, &user.NotifyProps); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal user notify props")
	}
	if err = json.Unmarshal(timezone, &user.Timezone); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal user timezone")
	}

	return &user, nil
}

func (us SqlUserStore) GetAll() ([]*model.User, error) {
	query := us.usersQuery.OrderBy("Username ASC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_all_users_tosql")
	}

	var data []*model.User
	if _, err := us.GetReplica().Select(&data, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}
	return data, nil
}

func (us SqlUserStore) GetAllAfter(limit int, afterId string) ([]*model.User, error) {
	query := us.usersQuery.
		Where("Id > ?", afterId).
		OrderBy("Id ASC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_all_after_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	return users, nil
}

func (us SqlUserStore) GetEtagForAllProfiles() string {
	updateAt, err := us.GetReplica().SelectInt("SELECT UpdateAt FROM Users ORDER BY UpdateAt DESC LIMIT 1")
	if err != nil {
		return fmt.Sprintf("%v.%v", model.CurrentVersion, model.GetMillis())
	}
	return fmt.Sprintf("%v.%v", model.CurrentVersion, updateAt)
}

func (us SqlUserStore) GetAllProfiles(options *model.UserGetOptions) ([]*model.User, error) {
	isPostgreSQL := us.DriverName() == model.DATABASE_DRIVER_POSTGRES
	query := us.usersQuery.
		OrderBy("u.Username ASC").
		Offset(uint64(options.Page * options.PerPage)).Limit(uint64(options.PerPage))

	query = applyViewRestrictionsFilter(query, options.ViewRestrictions, true)

	query = applyRoleFilter(query, options.Role, isPostgreSQL)
	query = applyMultiRoleFilters(query, options.Roles, []string{}, []string{}, isPostgreSQL)

	if options.Inactive {
		query = query.Where("u.DeleteAt != 0")
	} else if options.Active {
		query = query.Where("u.DeleteAt = 0")
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_all_profiles_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to get User profiles")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func applyRoleFilter(query sq.SelectBuilder, role string, isPostgreSQL bool) sq.SelectBuilder {
	if role == "" {
		return query
	}

	if isPostgreSQL {
		roleParam := fmt.Sprintf("%%%s%%", sanitizeSearchTerm(role, "\\"))
		return query.Where("u.Roles LIKE LOWER(?)", roleParam)
	}

	roleParam := fmt.Sprintf("%%%s%%", sanitizeSearchTerm(role, "*"))

	return query.Where("u.Roles LIKE ? ESCAPE '*'", roleParam)
}

func applyMultiRoleFilters(query sq.SelectBuilder, systemRoles []string, teamRoles []string, channelRoles []string, isPostgreSQL bool) sq.SelectBuilder {
	sqOr := sq.Or{}

	if len(systemRoles) > 0 && systemRoles[0] != "" {
		for _, role := range systemRoles {
			queryRole := wildcardSearchTerm(role)
			switch role {
			case model.SYSTEM_USER_ROLE_ID:
				// If querying for a `system_user` ensure that the user is only a system_user.
				sqOr = append(sqOr, sq.Eq{"u.Roles": role})
			case model.SYSTEM_GUEST_ROLE_ID, model.SYSTEM_ADMIN_ROLE_ID, model.SYSTEM_USER_MANAGER_ROLE_ID, model.SYSTEM_READ_ONLY_ADMIN_ROLE_ID, model.SYSTEM_MANAGER_ROLE_ID:
				// If querying for any other roles search using a wildcard.
				if isPostgreSQL {
					sqOr = append(sqOr, sq.ILike{"u.Roles": queryRole})
				} else {
					sqOr = append(sqOr, sq.Like{"u.Roles": queryRole})
				}
			}

		}
	}

	if len(channelRoles) > 0 && channelRoles[0] != "" {
		for _, channelRole := range channelRoles {
			switch channelRole {
			case model.CHANNEL_ADMIN_ROLE_ID:
				if isPostgreSQL {
					sqOr = append(sqOr, sq.And{sq.Eq{"cm.SchemeAdmin": true}, sq.NotILike{"u.Roles": wildcardSearchTerm(model.SYSTEM_ADMIN_ROLE_ID)}})
				} else {
					sqOr = append(sqOr, sq.And{sq.Eq{"cm.SchemeAdmin": true}, sq.NotLike{"u.Roles": wildcardSearchTerm(model.SYSTEM_ADMIN_ROLE_ID)}})
				}
			case model.CHANNEL_USER_ROLE_ID:
				if isPostgreSQL {
					sqOr = append(sqOr, sq.And{sq.Eq{"cm.SchemeUser": true}, sq.Eq{"cm.SchemeAdmin": false}, sq.NotILike{"u.Roles": wildcardSearchTerm(model.SYSTEM_ADMIN_ROLE_ID)}})
				} else {
					sqOr = append(sqOr, sq.And{sq.Eq{"cm.SchemeUser": true}, sq.Eq{"cm.SchemeAdmin": false}, sq.NotLike{"u.Roles": wildcardSearchTerm(model.SYSTEM_ADMIN_ROLE_ID)}})
				}
			case model.CHANNEL_GUEST_ROLE_ID:
				sqOr = append(sqOr, sq.Eq{"cm.SchemeGuest": true})
			}
		}
	}

	if len(teamRoles) > 0 && teamRoles[0] != "" {
		for _, teamRole := range teamRoles {
			switch teamRole {
			case model.TEAM_ADMIN_ROLE_ID:
				if isPostgreSQL {
					sqOr = append(sqOr, sq.And{sq.Eq{"tm.SchemeAdmin": true}, sq.NotILike{"u.Roles": wildcardSearchTerm(model.SYSTEM_ADMIN_ROLE_ID)}})
				} else {
					sqOr = append(sqOr, sq.And{sq.Eq{"tm.SchemeAdmin": true}, sq.NotLike{"u.Roles": wildcardSearchTerm(model.SYSTEM_ADMIN_ROLE_ID)}})
				}
			case model.TEAM_USER_ROLE_ID:
				if isPostgreSQL {
					sqOr = append(sqOr, sq.And{sq.Eq{"tm.SchemeUser": true}, sq.Eq{"tm.SchemeAdmin": false}, sq.NotILike{"u.Roles": wildcardSearchTerm(model.SYSTEM_ADMIN_ROLE_ID)}})
				} else {
					sqOr = append(sqOr, sq.And{sq.Eq{"tm.SchemeUser": true}, sq.Eq{"tm.SchemeAdmin": false}, sq.NotLike{"u.Roles": wildcardSearchTerm(model.SYSTEM_ADMIN_ROLE_ID)}})
				}
			case model.TEAM_GUEST_ROLE_ID:
				sqOr = append(sqOr, sq.Eq{"tm.SchemeGuest": true})
			}
		}
	}

	if len(sqOr) > 0 {
		return query.Where(sqOr)
	}
	return query
}

func applyChannelGroupConstrainedFilter(query sq.SelectBuilder, channelId string) sq.SelectBuilder {
	if channelId == "" {
		return query
	}

	return query.
		Where(`u.Id IN (
				SELECT
					GroupMembers.UserId
				FROM
					Channels
					JOIN GroupChannels ON GroupChannels.ChannelId = Channels.Id
					JOIN UserGroups ON UserGroups.Id = GroupChannels.GroupId
					JOIN GroupMembers ON GroupMembers.GroupId = UserGroups.Id
				WHERE
					Channels.Id = ?
					AND GroupChannels.DeleteAt = 0
					AND UserGroups.DeleteAt = 0
					AND GroupMembers.DeleteAt = 0
				GROUP BY
					GroupMembers.UserId
			)`, channelId)
}

func applyTeamGroupConstrainedFilter(query sq.SelectBuilder, teamId string) sq.SelectBuilder {
	if teamId == "" {
		return query
	}

	return query.
		Where(`u.Id IN (
				SELECT
					GroupMembers.UserId
				FROM
					Teams
					JOIN GroupTeams ON GroupTeams.TeamId = Teams.Id
					JOIN UserGroups ON UserGroups.Id = GroupTeams.GroupId
					JOIN GroupMembers ON GroupMembers.GroupId = UserGroups.Id
				WHERE
					Teams.Id = ?
					AND GroupTeams.DeleteAt = 0
					AND UserGroups.DeleteAt = 0
					AND GroupMembers.DeleteAt = 0
				GROUP BY
					GroupMembers.UserId
			)`, teamId)
}

func (us SqlUserStore) GetEtagForProfiles(teamId string) string {
	updateAt, err := us.GetReplica().SelectInt("SELECT UpdateAt FROM Users, TeamMembers WHERE TeamMembers.TeamId = :TeamId AND Users.Id = TeamMembers.UserId ORDER BY UpdateAt DESC LIMIT 1", map[string]interface{}{"TeamId": teamId})
	if err != nil {
		return fmt.Sprintf("%v.%v", model.CurrentVersion, model.GetMillis())
	}
	return fmt.Sprintf("%v.%v", model.CurrentVersion, updateAt)
}

func (us SqlUserStore) GetProfiles(options *model.UserGetOptions) ([]*model.User, error) {
	isPostgreSQL := us.DriverName() == model.DATABASE_DRIVER_POSTGRES
	query := us.usersQuery.
		Join("TeamMembers tm ON ( tm.UserId = u.Id AND tm.DeleteAt = 0 )").
		Where("tm.TeamId = ?", options.InTeamId).
		OrderBy("u.Username ASC").
		Offset(uint64(options.Page * options.PerPage)).Limit(uint64(options.PerPage))

	query = applyViewRestrictionsFilter(query, options.ViewRestrictions, true)

	query = applyRoleFilter(query, options.Role, isPostgreSQL)
	query = applyMultiRoleFilters(query, options.Roles, options.TeamRoles, options.ChannelRoles, isPostgreSQL)

	if options.Inactive {
		query = query.Where("u.DeleteAt != 0")
	} else if options.Active {
		query = query.Where("u.DeleteAt = 0")
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_etag_for_profiles_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func (us SqlUserStore) InvalidateProfilesInChannelCacheByUser(userId string) {}

func (us SqlUserStore) InvalidateProfilesInChannelCache(channelId string) {}

func (us SqlUserStore) GetProfilesInChannel(options *model.UserGetOptions) ([]*model.User, error) {
	query := us.usersQuery.
		Join("ChannelMembers cm ON ( cm.UserId = u.Id )").
		Where("cm.ChannelId = ?", options.InChannelId).
		OrderBy("u.Username ASC").
		Offset(uint64(options.Page * options.PerPage)).Limit(uint64(options.PerPage))

	if options.Inactive {
		query = query.Where("u.DeleteAt != 0")
	} else if options.Active {
		query = query.Where("u.DeleteAt = 0")
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_profiles_in_channel_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func (us SqlUserStore) GetProfilesInChannelByStatus(options *model.UserGetOptions) ([]*model.User, error) {
	query := us.usersQuery.
		Join("ChannelMembers cm ON ( cm.UserId = u.Id )").
		LeftJoin("Status s ON ( s.UserId = u.Id )").
		Where("cm.ChannelId = ?", options.InChannelId).
		OrderBy(`
			CASE s.Status
				WHEN 'online' THEN 1
				WHEN 'away' THEN 2
				WHEN 'dnd' THEN 3
				ELSE 4
			END
			`).
		OrderBy("u.Username ASC").
		Offset(uint64(options.Page * options.PerPage)).Limit(uint64(options.PerPage))

	if options.Inactive && !options.Active {
		query = query.Where("u.DeleteAt != 0")
	} else if options.Active && !options.Inactive {
		query = query.Where("u.DeleteAt = 0")
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_profiles_in_channel_by_status_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func (us SqlUserStore) GetAllProfilesInChannel(ctx context.Context, channelID string, allowFromCache bool) (map[string]*model.User, error) {
	query := us.usersQuery.
		Join("ChannelMembers cm ON ( cm.UserId = u.Id )").
		Where("cm.ChannelId = ?", channelID).
		Where("u.DeleteAt = 0").
		OrderBy("u.Username ASC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_all_profiles_in_channel_tosql")
	}

	var users []*model.User
	rows, err := us.SqlStore.DBFromContext(ctx).Db.Query(queryString, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	defer rows.Close()
	for rows.Next() {
		var user model.User
		var props, notifyProps, timezone []byte
		if err = rows.Scan(&user.Id, &user.CreateAt, &user.UpdateAt, &user.DeleteAt, &user.Username, &user.Password, &user.AuthData, &user.AuthService, &user.Email, &user.EmailVerified, &user.Nickname, &user.FirstName, &user.LastName, &user.Position, &user.Roles, &user.AllowMarketing, &props, &notifyProps, &user.LastPasswordUpdate, &user.LastPictureUpdate, &user.FailedAttempts, &user.Locale, &timezone, &user.MfaActive, &user.MfaSecret, &user.IsBot, &user.BotDescription, &user.BotLastIconUpdate, &user.RemoteId); err != nil {
			return nil, errors.Wrap(err, "failed to scan values from rows into User entity")
		}
		if err = json.Unmarshal(props, &user.Props); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal user props")
		}
		if err = json.Unmarshal(notifyProps, &user.NotifyProps); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal user notify props")
		}
		if err = json.Unmarshal(timezone, &user.Timezone); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal user timezone")
		}
		users = append(users, &user)
	}
	err = rows.Err()
	if err != nil {
		return nil, errors.Wrap(err, "error while iterating over rows")
	}

	userMap := make(map[string]*model.User)

	for _, u := range users {
		u.Sanitize(map[string]bool{})
		userMap[u.Id] = u
	}

	return userMap, nil
}

func (us SqlUserStore) GetProfilesNotInChannel(teamId string, channelId string, groupConstrained bool, offset int, limit int, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, error) {
	query := us.usersQuery.
		Join("TeamMembers tm ON ( tm.UserId = u.Id AND tm.DeleteAt = 0 AND tm.TeamId = ? )", teamId).
		LeftJoin("ChannelMembers cm ON ( cm.UserId = u.Id AND cm.ChannelId = ? )", channelId).
		Where("cm.UserId IS NULL").
		OrderBy("u.Username ASC").
		Offset(uint64(offset)).Limit(uint64(limit))

	query = applyViewRestrictionsFilter(query, viewRestrictions, true)

	if groupConstrained {
		query = applyChannelGroupConstrainedFilter(query, channelId)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_profiles_not_in_channel_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func (us SqlUserStore) GetProfilesWithoutTeam(options *model.UserGetOptions) ([]*model.User, error) {
	isPostgreSQL := us.DriverName() == model.DATABASE_DRIVER_POSTGRES
	query := us.usersQuery.
		Where(`(
			SELECT
				COUNT(0)
			FROM
				TeamMembers
			WHERE
				TeamMembers.UserId = u.Id
				AND TeamMembers.DeleteAt = 0
		) = 0`).
		OrderBy("u.Username ASC").
		Offset(uint64(options.Page * options.PerPage)).Limit(uint64(options.PerPage))

	query = applyViewRestrictionsFilter(query, options.ViewRestrictions, true)

	query = applyRoleFilter(query, options.Role, isPostgreSQL)

	if options.Inactive {
		query = query.Where("u.DeleteAt != 0")
	} else if options.Active {
		query = query.Where("u.DeleteAt = 0")
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_profiles_without_team_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func (us SqlUserStore) GetProfilesByUsernames(usernames []string, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, error) {
	query := us.usersQuery

	query = applyViewRestrictionsFilter(query, viewRestrictions, true)

	query = query.
		Where(map[string]interface{}{
			"Username": usernames,
		}).
		OrderBy("u.Username ASC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_profiles_by_usernames")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	return users, nil
}

type UserWithLastActivityAt struct {
	model.User
	LastActivityAt int64
}

func (us SqlUserStore) GetRecentlyActiveUsersForTeam(teamId string, offset, limit int, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, error) {
	query := us.usersQuery.
		Column("s.LastActivityAt").
		Join("TeamMembers tm ON (tm.UserId = u.Id AND tm.TeamId = ?)", teamId).
		Join("Status s ON (s.UserId = u.Id)").
		OrderBy("s.LastActivityAt DESC").
		OrderBy("u.Username ASC").
		Offset(uint64(offset)).Limit(uint64(limit))

	query = applyViewRestrictionsFilter(query, viewRestrictions, true)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_recently_active_users_for_team_tosql")
	}

	var users []*UserWithLastActivityAt
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	userList := []*model.User{}

	for _, userWithLastActivityAt := range users {
		u := userWithLastActivityAt.User
		u.Sanitize(map[string]bool{})
		u.LastActivityAt = userWithLastActivityAt.LastActivityAt
		userList = append(userList, &u)
	}

	return userList, nil
}

func (us SqlUserStore) GetNewUsersForTeam(teamId string, offset, limit int, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, error) {
	query := us.usersQuery.
		Join("TeamMembers tm ON (tm.UserId = u.Id AND tm.TeamId = ?)", teamId).
		OrderBy("u.CreateAt DESC").
		OrderBy("u.Username ASC").
		Offset(uint64(offset)).Limit(uint64(limit))

	query = applyViewRestrictionsFilter(query, viewRestrictions, true)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_new_users_for_team_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func (us SqlUserStore) GetProfileByIds(ctx context.Context, userIds []string, options *store.UserGetByIdsOpts, allowFromCache bool) ([]*model.User, error) {
	if options == nil {
		options = &store.UserGetByIdsOpts{}
	}

	users := []*model.User{}
	query := us.usersQuery.
		Where(map[string]interface{}{
			"u.Id": userIds,
		}).
		OrderBy("u.Username ASC")

	if options.Since > 0 {
		query = query.Where(sq.Gt(map[string]interface{}{
			"u.UpdateAt": options.Since,
		}))
	}

	query = applyViewRestrictionsFilter(query, options.ViewRestrictions, true)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_profile_by_ids_tosql")
	}

	if _, err := us.SqlStore.DBFromContext(ctx).Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	return users, nil
}

type UserWithChannel struct {
	model.User
	ChannelId string
}

func (us SqlUserStore) GetProfileByGroupChannelIdsForUser(userId string, channelIds []string) (map[string][]*model.User, error) {
	if len(channelIds) > MaxGroupChannelsForProfiles {
		channelIds = channelIds[0:MaxGroupChannelsForProfiles]
	}

	isMemberQuery := fmt.Sprintf(`
      EXISTS(
        SELECT
          1
        FROM
          ChannelMembers
        WHERE
          UserId = '%s'
        AND
          ChannelId = cm.ChannelId
        )`, userId)

	query := us.getQueryBuilder().
		Select("u.*, cm.ChannelId").
		From("Users u").
		Join("ChannelMembers cm ON u.Id = cm.UserId").
		Join("Channels c ON cm.ChannelId = c.Id").
		Where(sq.Eq{"c.Type": model.CHANNEL_GROUP, "cm.ChannelId": channelIds}).
		Where(isMemberQuery).
		Where(sq.NotEq{"u.Id": userId}).
		OrderBy("u.Username ASC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_profiles_by_group_channel_ids_for_user_tosql")
	}

	usersWithChannel := []*UserWithChannel{}
	if _, err := us.GetReplica().Select(&usersWithChannel, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	usersByChannelId := map[string][]*model.User{}
	for _, user := range usersWithChannel {
		if val, ok := usersByChannelId[user.ChannelId]; ok {
			usersByChannelId[user.ChannelId] = append(val, &user.User)
		} else {
			usersByChannelId[user.ChannelId] = []*model.User{&user.User}
		}
	}

	return usersByChannelId, nil
}

func (us SqlUserStore) GetSystemAdminProfiles() (map[string]*model.User, error) {
	query := us.usersQuery.
		Where("Roles LIKE ?", "%system_admin%").
		OrderBy("u.Username ASC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_system_admin_profiles_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	userMap := make(map[string]*model.User)

	for _, u := range users {
		u.Sanitize(map[string]bool{})
		userMap[u.Id] = u
	}

	return userMap, nil
}

func (us SqlUserStore) GetByEmail(email string) (*model.User, error) {
	query := us.usersQuery.Where("Email = lower(?)", email)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_by_email_tosql")
	}

	user := model.User{}
	if err := us.GetReplica().SelectOne(&user, queryString, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Wrap(store.NewErrNotFound("User", fmt.Sprintf("email=%s", email)), "failed to find User")
		}

		return nil, errors.Wrapf(err, "failed to get User with email=%s", email)
	}

	return &user, nil
}

func (us SqlUserStore) GetByAuth(authData *string, authService string) (*model.User, error) {
	if authData == nil || *authData == "" {
		return nil, store.NewErrInvalidInput("User", "<authData>", "empty or nil")
	}

	query := us.usersQuery.
		Where("u.AuthData = ?", authData).
		Where("u.AuthService = ?", authService)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_by_auth_tosql")
	}

	user := model.User{}
	if err := us.GetReplica().SelectOne(&user, queryString, args...); err == sql.ErrNoRows {
		return nil, store.NewErrNotFound("User", fmt.Sprintf("authData=%s, authService=%s", *authData, authService))
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to find User with authData=%s and authService=%s", *authData, authService)
	}
	return &user, nil
}

func (us SqlUserStore) GetAllUsingAuthService(authService string) ([]*model.User, error) {
	query := us.usersQuery.
		Where("u.AuthService = ?", authService).
		OrderBy("u.Username ASC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_all_using_auth_service_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find Users with authService=%s", authService)
	}

	return users, nil
}

func (us SqlUserStore) GetAllNotInAuthService(authServices []string) ([]*model.User, error) {
	query := us.usersQuery.
		Where(sq.NotEq{"u.AuthService": authServices}).
		OrderBy("u.Username ASC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_all_not_in_auth_service_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find Users with authServices in %v", authServices)
	}

	return users, nil
}

func (us SqlUserStore) GetByUsername(username string) (*model.User, error) {
	query := us.usersQuery.Where("u.Username = lower(?)", username)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_by_username_tosql")
	}

	var user *model.User
	if err := us.GetReplica().SelectOne(&user, queryString, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Wrap(store.NewErrNotFound("User", fmt.Sprintf("username=%s", username)), "failed to find User")
		}

		return nil, errors.Wrapf(err, "failed to find User with username=%s", username)
	}

	return user, nil
}

func (us SqlUserStore) GetForLogin(loginId string, allowSignInWithUsername, allowSignInWithEmail bool) (*model.User, error) {
	query := us.usersQuery
	if allowSignInWithUsername && allowSignInWithEmail {
		query = query.Where("Username = lower(?) OR Email = lower(?)", loginId, loginId)
	} else if allowSignInWithUsername {
		query = query.Where("Username = lower(?)", loginId)
	} else if allowSignInWithEmail {
		query = query.Where("Email = lower(?)", loginId)
	} else {
		return nil, errors.New("sign in with username and email are disabled")
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_for_login_tosql")
	}

	users := []*model.User{}
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	if len(users) == 0 {
		return nil, errors.New("user not found")
	}

	if len(users) > 1 {
		return nil, errors.New("multiple users found")
	}

	return users[0], nil

}

func (us SqlUserStore) VerifyEmail(userId, email string) (string, error) {
	curTime := model.GetMillis()
	if _, err := us.GetMaster().Exec("UPDATE Users SET Email = lower(:email), EmailVerified = true, UpdateAt = :Time WHERE Id = :UserId", map[string]interface{}{"email": email, "Time": curTime, "UserId": userId}); err != nil {
		return "", errors.Wrapf(err, "failed to update Users with userId=%s and email=%s", userId, email)
	}

	return userId, nil
}

func (us SqlUserStore) PermanentDelete(userId string) error {
	if _, err := us.GetMaster().Exec("DELETE FROM Users WHERE Id = :UserId", map[string]interface{}{"UserId": userId}); err != nil {
		return errors.Wrapf(err, "failed to delete User with userId=%s", userId)
	}
	return nil
}

func (us SqlUserStore) Count(options model.UserCountOptions) (int64, error) {
	isPostgreSQL := us.DriverName() == model.DATABASE_DRIVER_POSTGRES
	query := us.getQueryBuilder().Select("COUNT(DISTINCT u.Id)").From("Users AS u")

	if !options.IncludeDeleted {
		query = query.Where("u.DeleteAt = 0")
	}

	if options.IncludeBotAccounts {
		if options.ExcludeRegularUsers {
			query = query.Join("Bots ON u.Id = Bots.UserId")
		}
	} else {
		query = query.LeftJoin("Bots ON u.Id = Bots.UserId").Where("Bots.UserId IS NULL")
		if options.ExcludeRegularUsers {
			// Currently this doesn't make sense because it will always return 0
			return int64(0), errors.New("query with IncludeBotAccounts=false and excludeRegularUsers=true always return 0")
		}
	}

	if options.TeamId != "" {
		query = query.LeftJoin("TeamMembers AS tm ON u.Id = tm.UserId").Where("tm.TeamId = ? AND tm.DeleteAt = 0", options.TeamId)
	} else if options.ChannelId != "" {
		query = query.LeftJoin("ChannelMembers AS cm ON u.Id = cm.UserId").Where("cm.ChannelId = ?", options.ChannelId)
	}
	query = applyViewRestrictionsFilter(query, options.ViewRestrictions, false)
	query = applyMultiRoleFilters(query, options.Roles, options.TeamRoles, options.ChannelRoles, isPostgreSQL)

	if isPostgreSQL {
		query = query.PlaceholderFormat(sq.Dollar)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return int64(0), errors.Wrap(err, "count_tosql")
	}

	count, err := us.GetReplica().SelectInt(queryString, args...)
	if err != nil {
		return int64(0), errors.Wrap(err, "failed to count Users")
	}
	return count, nil
}

func (us SqlUserStore) AnalyticsActiveCount(timePeriod int64, options model.UserCountOptions) (int64, error) {

	time := model.GetMillis() - timePeriod
	query := us.getQueryBuilder().Select("COUNT(*)").From("Status AS s").Where("LastActivityAt > :Time", map[string]interface{}{"Time": time})

	if !options.IncludeBotAccounts {
		query = query.LeftJoin("Bots ON s.UserId = Bots.UserId").Where("Bots.UserId IS NULL")
	}

	if !options.IncludeDeleted {
		query = query.LeftJoin("Users ON s.UserId = Users.Id").Where("Users.DeleteAt = 0")
	}

	queryStr, args, err := query.ToSql()

	if err != nil {
		return 0, errors.Wrap(err, "analytics_active_count_tosql")
	}

	v, err := us.GetReplica().SelectInt(queryStr, args...)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count Users")
	}
	return v, nil
}

func (us SqlUserStore) AnalyticsActiveCountForPeriod(startTime int64, endTime int64, options model.UserCountOptions) (int64, error) {
	query := us.getQueryBuilder().Select("COUNT(*)").From("Status AS s").Where("LastActivityAt > :StartTime AND LastActivityAt <= :EndTime", map[string]interface{}{"StartTime": startTime, "EndTime": endTime})

	if !options.IncludeBotAccounts {
		query = query.LeftJoin("Bots ON s.UserId = Bots.UserId").Where("Bots.UserId IS NULL")
	}

	if !options.IncludeDeleted {
		query = query.LeftJoin("Users ON s.UserId = Users.Id").Where("Users.DeleteAt = 0")
	}

	queryStr, args, err := query.ToSql()

	if err != nil {
		return 0, errors.Wrap(err, "Failed to build query.")
	}

	v, err := us.GetReplica().SelectInt(queryStr, args...)
	if err != nil {
		return 0, errors.Wrap(err, "Unable to get the active users during the requested period.")
	}
	return v, nil
}

func (us SqlUserStore) GetUnreadCount(userId string) (int64, error) {
	query := `
		SELECT SUM(CASE WHEN c.Type = 'D' THEN (c.TotalMsgCount - cm.MsgCount) ELSE cm.MentionCount END)
		FROM Channels c
		INNER JOIN ChannelMembers cm
			ON cm.ChannelId = c.Id
			AND cm.UserId = :UserId
			AND c.DeleteAt = 0
	`
	count, err := us.GetReplica().SelectInt(query, map[string]interface{}{"UserId": userId})
	if err != nil {
		return count, errors.Wrapf(err, "failed to count unread Channels for userId=%s", userId)
	}

	return count, nil
}

func (us SqlUserStore) GetUnreadCountForChannel(userId string, channelId string) (int64, error) {
	count, err := us.GetReplica().SelectInt("SELECT SUM(CASE WHEN c.Type = 'D' THEN (c.TotalMsgCount - cm.MsgCount) ELSE cm.MentionCount END) FROM Channels c INNER JOIN ChannelMembers cm ON c.Id = cm.ChannelId AND cm.ChannelId = :ChannelId AND cm.UserId = :UserId", map[string]interface{}{"ChannelId": channelId, "UserId": userId})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get unread count for channelId=%s and userId=%s", channelId, userId)
	}
	return count, nil
}

func (us SqlUserStore) GetAnyUnreadPostCountForChannel(userId string, channelId string) (int64, error) {
	count, err := us.GetReplica().SelectInt("SELECT SUM(c.TotalMsgCount - cm.MsgCount) FROM Channels c INNER JOIN ChannelMembers cm ON c.Id = cm.ChannelId AND cm.ChannelId = :ChannelId AND cm.UserId = :UserId", map[string]interface{}{"ChannelId": channelId, "UserId": userId})
	if err != nil {
		return count, errors.Wrapf(err, "failed to get any unread count for channelId=%s and userId=%s", channelId, userId)
	}
	return count, nil
}

func (us SqlUserStore) Search(teamId string, term string, options *model.UserSearchOptions) ([]*model.User, error) {
	query := us.usersQuery.
		OrderBy("Username ASC").
		Limit(uint64(options.Limit))

	if teamId != "" {
		query = query.Join("TeamMembers tm ON ( tm.UserId = u.Id AND tm.DeleteAt = 0 AND tm.TeamId = ? )", teamId)
	}
	return us.performSearch(query, term, options)
}

func (us SqlUserStore) SearchWithoutTeam(term string, options *model.UserSearchOptions) ([]*model.User, error) {
	query := us.usersQuery.
		Where(`(
				SELECT
					COUNT(0)
				FROM
					TeamMembers
				WHERE
					TeamMembers.UserId = u.Id
					AND TeamMembers.DeleteAt = 0
			) = 0`).
		OrderBy("u.Username ASC").
		Limit(uint64(options.Limit))

	return us.performSearch(query, term, options)
}

func (us SqlUserStore) SearchNotInTeam(notInTeamId string, term string, options *model.UserSearchOptions) ([]*model.User, error) {
	query := us.usersQuery.
		LeftJoin("TeamMembers tm ON ( tm.UserId = u.Id AND tm.DeleteAt = 0 AND tm.TeamId = ? )", notInTeamId).
		Where("tm.UserId IS NULL").
		OrderBy("u.Username ASC").
		Limit(uint64(options.Limit))

	if options.GroupConstrained {
		query = applyTeamGroupConstrainedFilter(query, notInTeamId)
	}

	return us.performSearch(query, term, options)
}

func (us SqlUserStore) SearchNotInChannel(teamId string, channelId string, term string, options *model.UserSearchOptions) ([]*model.User, error) {
	query := us.usersQuery.
		LeftJoin("ChannelMembers cm ON ( cm.UserId = u.Id AND cm.ChannelId = ? )", channelId).
		Where("cm.UserId IS NULL").
		OrderBy("Username ASC").
		Limit(uint64(options.Limit))

	if teamId != "" {
		query = query.Join("TeamMembers tm ON ( tm.UserId = u.Id AND tm.DeleteAt = 0 AND tm.TeamId = ? )", teamId)
	}

	if options.GroupConstrained {
		query = applyChannelGroupConstrainedFilter(query, channelId)
	}

	return us.performSearch(query, term, options)
}

func (us SqlUserStore) SearchInChannel(channelId string, term string, options *model.UserSearchOptions) ([]*model.User, error) {
	query := us.usersQuery.
		Join("ChannelMembers cm ON ( cm.UserId = u.Id AND cm.ChannelId = ? )", channelId).
		OrderBy("Username ASC").
		Limit(uint64(options.Limit))

	return us.performSearch(query, term, options)
}

func (us SqlUserStore) SearchInGroup(groupID string, term string, options *model.UserSearchOptions) ([]*model.User, error) {
	query := us.usersQuery.
		Join("GroupMembers gm ON ( gm.UserId = u.Id AND gm.GroupId = ? )", groupID).
		OrderBy("Username ASC").
		Limit(uint64(options.Limit))

	return us.performSearch(query, term, options)
}

var spaceFulltextSearchChar = []string{
	"<",
	">",
	"+",
	"-",
	"(",
	")",
	"~",
	":",
	"*",
	"\"",
	"!",
	"@",
}

func generateSearchQuery(query sq.SelectBuilder, terms []string, fields []string, isPostgreSQL bool) sq.SelectBuilder {
	escapedFields := make([]string, len(fields))
	for i, field := range fields {
		escapedFields[i] = "`" + field + "`"
	}

	if isPostgreSQL {
		return generatePostgresSearchQuery(query, terms, escapedFields)
	}
	return generateMySQLSearchQuery(query, terms, escapedFields)
}

func generatePostgresSearchQuery(query sq.SelectBuilder, terms []string, fields []string) sq.SelectBuilder {
	processedTerms := make([]string, len(terms))
	for i, term := range terms {
		processedTerms[i] = strings.TrimLeft(term, "@") + ":*"
	}

	query = query.Where(
		fmt.Sprintf("((to_tsvector('english', %s)) @@ to_tsquery('english', ?))", strings.Join(fields, " || ' ' || ")),
		strings.Join(processedTerms, " & "))
	return query
}

func generateMySQLSearchQuery(query sq.SelectBuilder, terms []string, fields []string) sq.SelectBuilder {
	processedTerms := make([]string, len(terms))
	for i, term := range terms {
		processedTerms[i] = "+" + strings.TrimLeft(term, "@") + "*"
	}
	query = query.Where(
		fmt.Sprintf("MATCH (%s) AGAINST(? IN BOOLEAN MODE)", strings.Join(fields, ", ")),
		strings.Join(processedTerms, ""))
	return query
}

func (us SqlUserStore) performSearch(query sq.SelectBuilder, term string, options *model.UserSearchOptions) ([]*model.User, error) {
	term = sanitizeSearchTerm(term, "*")

	var searchType []string
	if options.AllowEmails {
		if options.AllowFullNames {
			searchType = UserSearchTypeAll
		} else {
			searchType = UserSearchTypeAll_NO_FULL_NAME
		}
	} else {
		if options.AllowFullNames {
			searchType = UserSearchTypeNames
		} else {
			searchType = UserSearchTypeNames_NO_FULL_NAME
		}
	}

	isPostgreSQL := us.DriverName() == model.DATABASE_DRIVER_POSTGRES

	query = applyRoleFilter(query, options.Role, isPostgreSQL)
	query = applyMultiRoleFilters(query, options.Roles, options.TeamRoles, options.ChannelRoles, isPostgreSQL)

	if !options.AllowInactive {
		query = query.Where("u.DeleteAt = 0")
	}

	if strings.TrimSpace(term) != "" {
		query = generateSearchQuery(query, strings.Fields(term), searchType, isPostgreSQL)
	}

	query = applyViewRestrictionsFilter(query, options.ViewRestrictions, true)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "perform_search_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find Users with term=%s and searchType=%v", term, searchType)
	}
	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func (us SqlUserStore) AnalyticsGetInactiveUsersCount() (int64, error) {
	count, err := us.GetReplica().SelectInt("SELECT COUNT(Id) FROM Users WHERE DeleteAt > 0")
	if err != nil {
		return int64(0), errors.Wrap(err, "failed to count inactive Users")
	}
	return count, nil
}

func (us SqlUserStore) AnalyticsGetExternalUsers(hostDomain string) (bool, error) {
	count, err := us.GetReplica().SelectInt("SELECT COUNT(Id) FROM Users WHERE LOWER(Email) NOT LIKE :HostDomain", map[string]interface{}{"HostDomain": "%@" + strings.ToLower(hostDomain)})
	if err != nil {
		return false, errors.Wrap(err, "failed to count inactive Users")
	}
	return count > 0, nil
}

func (us SqlUserStore) AnalyticsGetGuestCount() (int64, error) {
	count, err := us.GetReplica().SelectInt("SELECT count(*) FROM Users WHERE Roles LIKE :Roles and DeleteAt = 0", map[string]interface{}{"Roles": "%system_guest%"})
	if err != nil {
		return int64(0), errors.Wrap(err, "failed to count guest Users")
	}
	return count, nil
}

func (us SqlUserStore) AnalyticsGetSystemAdminCount() (int64, error) {
	count, err := us.GetReplica().SelectInt("SELECT count(*) FROM Users WHERE Roles LIKE :Roles and DeleteAt = 0", map[string]interface{}{"Roles": "%system_admin%"})
	if err != nil {
		return int64(0), errors.Wrap(err, "failed to count system admin Users")
	}
	return count, nil
}

func (us SqlUserStore) GetProfilesNotInTeam(teamId string, groupConstrained bool, offset int, limit int, viewRestrictions *model.ViewUsersRestrictions) ([]*model.User, error) {
	var users []*model.User
	query := us.usersQuery.
		LeftJoin("TeamMembers tm ON ( tm.UserId = u.Id AND tm.DeleteAt = 0 AND tm.TeamId = ? )", teamId).
		Where("tm.UserId IS NULL").
		OrderBy("u.Username ASC").
		Offset(uint64(offset)).Limit(uint64(limit))

	query = applyViewRestrictionsFilter(query, viewRestrictions, true)

	if groupConstrained {
		query = applyTeamGroupConstrainedFilter(query, teamId)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_profiles_not_in_team_tosql")
	}

	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}
	return users, nil
}

func (us SqlUserStore) GetEtagForProfilesNotInTeam(teamId string) string {
	querystr := `
		SELECT
			CONCAT(MAX(UpdateAt), '.', COUNT(Id)) as etag
		FROM
			Users as u
		LEFT JOIN TeamMembers tm
			ON tm.UserId = u.Id
			AND tm.TeamId = :TeamId
			AND tm.DeleteAt = 0
		WHERE
			tm.UserId IS NULL
	`
	etag, err := us.GetReplica().SelectStr(querystr, map[string]interface{}{"TeamId": teamId})
	if err != nil {
		return fmt.Sprintf("%v.%v", model.CurrentVersion, model.GetMillis())
	}

	return fmt.Sprintf("%v.%v", model.CurrentVersion, etag)
}

func (us SqlUserStore) ClearAllCustomRoleAssignments() error {
	builtInRoles := model.MakeDefaultRoles()
	lastUserId := strings.Repeat("0", 26)

	for {
		var transaction *gorp.Transaction
		var err error

		if transaction, err = us.GetMaster().Begin(); err != nil {
			return errors.Wrap(err, "begin_transaction")
		}
		defer finalizeTransaction(transaction)

		var users []*model.User
		if _, err := transaction.Select(&users, "SELECT * from Users WHERE Id > :Id ORDER BY Id LIMIT 1000", map[string]interface{}{"Id": lastUserId}); err != nil {
			return errors.Wrapf(err, "failed to find Users with id > %s", lastUserId)
		}

		if len(users) == 0 {
			break
		}

		for _, user := range users {
			lastUserId = user.Id

			var newRoles []string

			for _, role := range strings.Fields(user.Roles) {
				for name := range builtInRoles {
					if name == role {
						newRoles = append(newRoles, role)
						break
					}
				}
			}

			newRolesString := strings.Join(newRoles, " ")
			if newRolesString != user.Roles {
				if _, err := transaction.Exec("UPDATE Users SET Roles = :Roles WHERE Id = :Id", map[string]interface{}{"Roles": newRolesString, "Id": user.Id}); err != nil {
					return errors.Wrap(err, "failed to update Users")
				}
			}
		}

		if err := transaction.Commit(); err != nil {
			return errors.Wrap(err, "commit_transaction")
		}
	}

	return nil
}

func (us SqlUserStore) InferSystemInstallDate() (int64, error) {
	createAt, err := us.GetReplica().SelectInt("SELECT CreateAt FROM Users WHERE CreateAt IS NOT NULL ORDER BY CreateAt ASC LIMIT 1")
	if err != nil {
		return 0, errors.Wrap(err, "failed to infer system install date")
	}

	return createAt, nil
}

func (us SqlUserStore) GetUsersBatchForIndexing(startTime, endTime int64, limit int) ([]*model.UserForIndexing, error) {
	var users []*model.User
	usersQuery, args, _ := us.usersQuery.
		Where(sq.GtOrEq{"u.CreateAt": startTime}).
		Where(sq.Lt{"u.CreateAt": endTime}).
		OrderBy("u.CreateAt").
		Limit(uint64(limit)).
		ToSql()
	_, err := us.GetSearchReplica().Select(&users, usersQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	userIds := []string{}
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}

	var channelMembers []*model.ChannelMember
	channelMembersQuery, args, _ := us.getQueryBuilder().
		Select(`
				cm.ChannelId,
				cm.UserId,
				cm.Roles,
				cm.LastViewedAt,
				cm.MsgCount,
				cm.MentionCount,
				cm.MentionCountRoot,
				cm.NotifyProps,
				cm.LastUpdateAt,
				cm.SchemeUser,
				cm.SchemeAdmin,
				(cm.SchemeGuest IS NOT NULL AND cm.SchemeGuest) as SchemeGuest
			`).
		From("ChannelMembers cm").
		Join("Channels c ON cm.ChannelId = c.Id").
		Where(sq.Eq{"c.Type": "O", "cm.UserId": userIds}).
		ToSql()
	_, err = us.GetSearchReplica().Select(&channelMembers, channelMembersQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find ChannelMembers")
	}

	var teamMembers []*model.TeamMember
	teamMembersQuery, args, _ := us.getQueryBuilder().
		Select("TeamId, UserId, Roles, DeleteAt, (SchemeGuest IS NOT NULL AND SchemeGuest) as SchemeGuest, SchemeUser, SchemeAdmin").
		From("TeamMembers").
		Where(sq.Eq{"UserId": userIds, "DeleteAt": 0}).
		ToSql()
	_, err = us.GetSearchReplica().Select(&teamMembers, teamMembersQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find TeamMembers")
	}

	userMap := map[string]*model.UserForIndexing{}
	for _, user := range users {
		userMap[user.Id] = &model.UserForIndexing{
			Id:          user.Id,
			Username:    user.Username,
			Nickname:    user.Nickname,
			FirstName:   user.FirstName,
			LastName:    user.LastName,
			Roles:       user.Roles,
			CreateAt:    user.CreateAt,
			DeleteAt:    user.DeleteAt,
			TeamsIds:    []string{},
			ChannelsIds: []string{},
		}
	}

	for _, c := range channelMembers {
		if userMap[c.UserId] != nil {
			userMap[c.UserId].ChannelsIds = append(userMap[c.UserId].ChannelsIds, c.ChannelId)
		}
	}
	for _, t := range teamMembers {
		if userMap[t.UserId] != nil {
			userMap[t.UserId].TeamsIds = append(userMap[t.UserId].TeamsIds, t.TeamId)
		}
	}

	usersForIndexing := []*model.UserForIndexing{}
	for _, user := range userMap {
		usersForIndexing = append(usersForIndexing, user)
	}
	sort.Slice(usersForIndexing, func(i, j int) bool {
		return usersForIndexing[i].CreateAt < usersForIndexing[j].CreateAt
	})

	return usersForIndexing, nil
}

func (us SqlUserStore) GetTeamGroupUsers(teamID string) ([]*model.User, error) {
	query := applyTeamGroupConstrainedFilter(us.usersQuery, teamID)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_team_group_users_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func (us SqlUserStore) GetChannelGroupUsers(channelID string) ([]*model.User, error) {
	query := applyChannelGroupConstrainedFilter(us.usersQuery, channelID)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_channel_group_users_tosql")
	}

	var users []*model.User
	if _, err := us.GetReplica().Select(&users, queryString, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Users")
	}

	for _, u := range users {
		u.Sanitize(map[string]bool{})
	}

	return users, nil
}

func applyViewRestrictionsFilter(query sq.SelectBuilder, restrictions *model.ViewUsersRestrictions, distinct bool) sq.SelectBuilder {
	if restrictions == nil {
		return query
	}

	// If you have no access to teams or channels, return and empty result.
	if restrictions.Teams != nil && len(restrictions.Teams) == 0 && restrictions.Channels != nil && len(restrictions.Channels) == 0 {
		return query.Where("1 = 0")
	}

	teams := make([]interface{}, len(restrictions.Teams))
	for i, v := range restrictions.Teams {
		teams[i] = v
	}
	channels := make([]interface{}, len(restrictions.Channels))
	for i, v := range restrictions.Channels {
		channels[i] = v
	}
	resultQuery := query
	if restrictions.Teams != nil && len(restrictions.Teams) > 0 {
		resultQuery = resultQuery.Join(fmt.Sprintf("TeamMembers rtm ON ( rtm.UserId = u.Id AND rtm.DeleteAt = 0 AND rtm.TeamId IN (%s))", sq.Placeholders(len(teams))), teams...)
	}
	if restrictions.Channels != nil && len(restrictions.Channels) > 0 {
		resultQuery = resultQuery.Join(fmt.Sprintf("ChannelMembers rcm ON ( rcm.UserId = u.Id AND rcm.ChannelId IN (%s))", sq.Placeholders(len(channels))), channels...)
	}

	if distinct {
		return resultQuery.Distinct()
	}

	return resultQuery
}

func (us SqlUserStore) PromoteGuestToUser(userId string) error {
	transaction, err := us.GetMaster().Begin()
	if err != nil {
		return errors.Wrap(err, "begin_transaction")
	}
	defer finalizeTransaction(transaction)

	user, err := us.Get(context.Background(), userId)
	if err != nil {
		return err
	}

	roles := user.GetRoles()

	for idx, role := range roles {
		if role == "system_guest" {
			roles[idx] = "system_user"
		}
	}

	curTime := model.GetMillis()
	query := us.getQueryBuilder().Update("Users").
		Set("Roles", strings.Join(roles, " ")).
		Set("UpdateAt", curTime).
		Where(sq.Eq{"Id": userId})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "promote_guest_to_user_tosql")
	}

	if _, err = transaction.Exec(queryString, args...); err != nil {
		return errors.Wrapf(err, "failed to update User with userId=%s", userId)
	}

	query = us.getQueryBuilder().Update("ChannelMembers").
		Set("SchemeUser", true).
		Set("SchemeGuest", false).
		Where(sq.Eq{"UserId": userId})

	queryString, args, err = query.ToSql()
	if err != nil {
		return errors.Wrap(err, "promote_guest_to_user_tosql")
	}

	if _, err = transaction.Exec(queryString, args...); err != nil {
		return errors.Wrapf(err, "failed to update ChannelMembers with userId=%s", userId)
	}

	query = us.getQueryBuilder().Update("TeamMembers").
		Set("SchemeUser", true).
		Set("SchemeGuest", false).
		Where(sq.Eq{"UserId": userId})

	queryString, args, err = query.ToSql()
	if err != nil {
		return errors.Wrap(err, "promote_guest_to_user_tosql")
	}

	if _, err := transaction.Exec(queryString, args...); err != nil {
		return errors.Wrapf(err, "failed to update TeamMembers with userId=%s", userId)
	}

	if err := transaction.Commit(); err != nil {
		return errors.Wrap(err, "commit_transaction")
	}
	return nil
}

func (us SqlUserStore) DemoteUserToGuest(userID string) (*model.User, error) {
	transaction, err := us.GetMaster().Begin()
	if err != nil {
		return nil, errors.Wrap(err, "begin_transaction")
	}
	defer finalizeTransaction(transaction)

	user, err := us.Get(context.Background(), userID)
	if err != nil {
		return nil, err
	}

	roles := user.GetRoles()

	newRoles := []string{}
	for _, role := range roles {
		if role == model.SYSTEM_USER_ROLE_ID {
			newRoles = append(newRoles, model.SYSTEM_GUEST_ROLE_ID)
		} else if role != model.SYSTEM_ADMIN_ROLE_ID {
			newRoles = append(newRoles, role)
		}
	}

	curTime := model.GetMillis()
	newRolesDBStr := strings.Join(newRoles, " ")
	query := us.getQueryBuilder().Update("Users").
		Set("Roles", newRolesDBStr).
		Set("UpdateAt", curTime).
		Where(sq.Eq{"Id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "demote_user_to_guest_tosql")
	}

	if _, err = transaction.Exec(queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to update User with userId=%s", userID)
	}

	user.Roles = newRolesDBStr
	user.UpdateAt = curTime

	query = us.getQueryBuilder().Update("ChannelMembers").
		Set("SchemeUser", false).
		Set("SchemeAdmin", false).
		Set("SchemeGuest", true).
		Where(sq.Eq{"UserId": userID})

	queryString, args, err = query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "demote_user_to_guest_tosql")
	}

	if _, err = transaction.Exec(queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to update ChannelMembers with userId=%s", userID)
	}

	query = us.getQueryBuilder().Update("TeamMembers").
		Set("SchemeUser", false).
		Set("SchemeAdmin", false).
		Set("SchemeGuest", true).
		Where(sq.Eq{"UserId": userID})

	queryString, args, err = query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "demote_user_to_guest_tosql")
	}

	if _, err := transaction.Exec(queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to update TeamMembers with userId=%s", userID)
	}

	if err := transaction.Commit(); err != nil {
		return nil, errors.Wrap(err, "commit_transaction")
	}
	return user, nil
}

func (us SqlUserStore) AutocompleteUsersInChannel(teamId, channelId, term string, options *model.UserSearchOptions) (*model.UserAutocompleteInChannel, error) {
	var usersInChannel, usersNotInChannel []*model.User
	g := errgroup.Group{}
	g.Go(func() (err error) {
		usersInChannel, err = us.SearchInChannel(channelId, term, options)
		return err
	})
	g.Go(func() (err error) {
		usersNotInChannel, err = us.SearchNotInChannel(teamId, channelId, term, options)
		return err
	})
	err := g.Wait()
	if err != nil {
		return nil, err
	}

	return &model.UserAutocompleteInChannel{
		InChannel:    usersInChannel,
		OutOfChannel: usersNotInChannel,
	}, nil
}

// GetKnownUsers returns the list of user ids of users with any direct
// relationship with a user. That means any user sharing any channel, including
// direct and group channels.
func (us SqlUserStore) GetKnownUsers(userId string) ([]string, error) {
	var userIds []string
	usersQuery, args, _ := us.getQueryBuilder().
		Select("DISTINCT ocm.UserId").
		From("ChannelMembers AS cm").
		Join("ChannelMembers AS ocm ON ocm.ChannelId = cm.ChannelId").
		Where(sq.NotEq{"ocm.UserId": userId}).
		Where(sq.Eq{"cm.UserId": userId}).
		ToSql()
	_, err := us.GetSearchReplica().Select(&userIds, usersQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find ChannelMembers")
	}

	return userIds, nil
}
