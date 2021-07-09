// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
)

const (
	DefaultGetUsersForSyncLimit = 100
)

type SQLSharedChannelStore struct {
	*SQLStore
}

func newSQLSharedChannelStore(sqlStore *SQLStore) store.SharedChannelStore {
	s := &SQLSharedChannelStore{
		SQLStore: sqlStore,
	}

	for _, db := range sqlStore.GetAllConns() {
		tableSharedChannels := db.AddTableWithName(model.SharedChannel{}, "SharedChannels").SetKeys(false, "ChannelId")
		tableSharedChannels.ColMap("ChannelId").SetMaxSize(26)
		tableSharedChannels.ColMap("TeamId").SetMaxSize(26)
		tableSharedChannels.ColMap("CreatorId").SetMaxSize(26)
		tableSharedChannels.ColMap("ShareName").SetMaxSize(64)
		tableSharedChannels.SetUniqueTogether("ShareName", "TeamId")
		tableSharedChannels.ColMap("ShareDisplayName").SetMaxSize(64)
		tableSharedChannels.ColMap("SharePurpose").SetMaxSize(250)
		tableSharedChannels.ColMap("ShareHeader").SetMaxSize(1024)
		tableSharedChannels.ColMap("RemoteId").SetMaxSize(26)

		tableSharedChannelRemotes := db.AddTableWithName(model.SharedChannelRemote{}, "SharedChannelRemotes").SetKeys(false, "Id", "ChannelId")
		tableSharedChannelRemotes.ColMap("Id").SetMaxSize(26)
		tableSharedChannelRemotes.ColMap("ChannelId").SetMaxSize(26)
		tableSharedChannelRemotes.ColMap("CreatorId").SetMaxSize(26)
		tableSharedChannelRemotes.ColMap("RemoteId").SetMaxSize(26)
		tableSharedChannelRemotes.ColMap("LastPostId").SetMaxSize(26)
		tableSharedChannelRemotes.SetUniqueTogether("ChannelId", "RemoteId")

		tableSharedChannelUsers := db.AddTableWithName(model.SharedChannelUser{}, "SharedChannelUsers").SetKeys(false, "Id")
		tableSharedChannelUsers.ColMap("Id").SetMaxSize(26)
		tableSharedChannelUsers.ColMap("UserId").SetMaxSize(26)
		tableSharedChannelUsers.ColMap("RemoteId").SetMaxSize(26)
		tableSharedChannelUsers.ColMap("ChannelId").SetMaxSize(26)
		tableSharedChannelUsers.SetUniqueTogether("UserId", "ChannelId", "RemoteId")

		tableSharedChannelFiles := db.AddTableWithName(model.SharedChannelAttachment{}, "SharedChannelAttachments").SetKeys(false, "Id")
		tableSharedChannelFiles.ColMap("Id").SetMaxSize(26)
		tableSharedChannelFiles.ColMap("FileId").SetMaxSize(26)
		tableSharedChannelFiles.ColMap("RemoteId").SetMaxSize(26)
		tableSharedChannelFiles.SetUniqueTogether("FileId", "RemoteId")
	}

	return s
}

func (s SQLSharedChannelStore) createIndexesIfNotExists() {
	s.CreateIndexIfNotExists("idx_sharedchannelusers_remote_id", "SharedChannelUsers", "RemoteId")
}

// Save inserts a new shared channel record.
func (s SQLSharedChannelStore) Save(sc *model.SharedChannel) (*model.SharedChannel, error) {
	sc.PreSave()
	if err := sc.IsValid(); err != nil {
		return nil, err
	}

	// make sure the shared channel is associated with a real channel.
	channel, err := s.stores.channel.Get(sc.ChannelID, true)
	if err != nil {
		return nil, fmt.Errorf("invalid channel: %w", err)
	}

	transaction, err := s.GetMaster().Begin()
	if err != nil {
		return nil, errors.Wrap(err, "begin_transaction")
	}
	defer finalizeTransaction(transaction)

	if err := transaction.Insert(sc); err != nil {
		return nil, errors.Wrapf(err, "save_shared_channel: ChannelId=%s", sc.ChannelID)
	}

	// set `Shared` flag in Channels table if needed
	if channel.Shared == nil || !*channel.Shared {
		if err := s.stores.channel.SetShared(channel.ID, true); err != nil {
			return nil, err
		}
	}

	if err := transaction.Commit(); err != nil {
		return nil, errors.Wrap(err, "commit_transaction")
	}
	return sc, nil
}

// Get fetches a shared channel by channel_id.
func (s SQLSharedChannelStore) Get(channelID string) (*model.SharedChannel, error) {
	var sc model.SharedChannel

	query := s.getQueryBuilder().
		Select("*").
		From("SharedChannels").
		Where(sq.Eq{"SharedChannels.ChannelId": channelID})

	squery, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "getsharedchannel_tosql")
	}

	if err := s.GetReplica().SelectOne(&sc, squery, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("SharedChannel", channelID)
		}
		return nil, errors.Wrapf(err, "failed to find shared channel with ChannelId=%s", channelID)
	}
	return &sc, nil
}

// HasChannel returns whether a given channelID is a shared channel or not.
func (s SQLSharedChannelStore) HasChannel(channelID string) (bool, error) {
	builder := s.getQueryBuilder().
		Select("1").
		Prefix("SELECT EXISTS (").
		From("SharedChannels").
		Where(sq.Eq{"SharedChannels.ChannelId": channelID}).
		Suffix(")")

	query, args, err := builder.ToSql()
	if err != nil {
		return false, errors.Wrapf(err, "get_shared_channel_exists_tosql")
	}

	var exists bool
	if err := s.GetReplica().SelectOne(&exists, query, args...); err != nil {
		return exists, errors.Wrapf(err, "failed to get shared channel for channel_id=%s", channelID)
	}
	return exists, nil
}

// GetAll fetches a paginated list of shared channels filtered by SharedChannelSearchOpts.
func (s SQLSharedChannelStore) GetAll(offset, limit int, opts model.SharedChannelFilterOpts) ([]*model.SharedChannel, error) {
	if opts.ExcludeHome && opts.ExcludeRemote {
		return nil, errors.New("cannot exclude home and remote shared channels")
	}

	safeConv := func(offset, limit int) (uint64, uint64, error) {
		if offset < 0 {
			return 0, 0, errors.New("offset must be positive integer")
		}
		if limit < 0 {
			return 0, 0, errors.New("limit must be positive integer")
		}
		return uint64(offset), uint64(limit), nil
	}

	safeOffset, safeLimit, err := safeConv(offset, limit)
	if err != nil {
		return nil, err
	}

	query := s.getSharedChannelsQuery(opts, false)
	query = query.OrderBy("sc.ShareDisplayName, sc.ShareName").Limit(safeLimit).Offset(safeOffset)

	squery, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create query")
	}

	var channels []*model.SharedChannel
	_, err = s.GetReplica().Select(&channels, squery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shared channels")
	}
	return channels, nil
}

// GetAllCount returns the number of shared channels that would be fetched using SharedChannelSearchOpts.
func (s SQLSharedChannelStore) GetAllCount(opts model.SharedChannelFilterOpts) (int64, error) {
	if opts.ExcludeHome && opts.ExcludeRemote {
		return 0, errors.New("cannot exclude home and remote shared channels")
	}

	query := s.getSharedChannelsQuery(opts, true)
	squery, args, err := query.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to create query")
	}

	count, err := s.GetReplica().SelectInt(squery, args...)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count channels")
	}
	return count, nil
}

func (s SQLSharedChannelStore) getSharedChannelsQuery(opts model.SharedChannelFilterOpts, forCount bool) sq.SelectBuilder {
	var selectStr string
	if forCount {
		selectStr = "count(sc.ChannelId)"
	} else {
		selectStr = "sc.*"
	}

	query := s.getQueryBuilder().
		Select(selectStr).
		From("SharedChannels AS sc")

	if opts.TeamID != "" {
		query = query.Where(sq.Eq{"sc.TeamId": opts.TeamID})
	}

	if opts.CreatorID != "" {
		query = query.Where(sq.Eq{"sc.CreatorId": opts.CreatorID})
	}

	if opts.ExcludeHome {
		query = query.Where(sq.NotEq{"sc.Home": true})
	}

	if opts.ExcludeRemote {
		query = query.Where(sq.Eq{"sc.Home": true})
	}

	return query
}

// Update updates the shared channel.
func (s SQLSharedChannelStore) Update(sc *model.SharedChannel) (*model.SharedChannel, error) {
	if err := sc.IsValid(); err != nil {
		return nil, err
	}

	count, err := s.GetMaster().Update(sc)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update shared channel with channelId=%s", sc.ChannelID)
	}

	if count != 1 {
		return nil, fmt.Errorf("expected number of shared channels to be updated is 1 but was %d", count)
	}
	return sc, nil
}

// Delete deletes a single shared channel plus associated SharedChannelRemotes.
// Returns true if shared channel found and deleted, false if not found.
func (s SQLSharedChannelStore) Delete(channelID string) (bool, error) {
	transaction, err := s.GetMaster().Begin()
	if err != nil {
		return false, errors.Wrap(err, "DeleteSharedChannel: begin_transaction")
	}
	defer finalizeTransaction(transaction)

	squery, args, err := s.getQueryBuilder().
		Delete("SharedChannels").
		Where(sq.Eq{"SharedChannels.ChannelId": channelID}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "delete_shared_channel_tosql")
	}

	result, err := transaction.Exec(squery, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to delete SharedChannel")
	}

	// Also remove remotes from SharedChannelRemotes (if any).
	squery, args, err = s.getQueryBuilder().
		Delete("SharedChannelRemotes").
		Where(sq.Eq{"ChannelId": channelID}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "delete_shared_channel_remotes_tosql")
	}

	_, err = transaction.Exec(squery, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to delete SharedChannelRemotes")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "failed to determine rows affected")
	}

	if count > 0 {
		// unset the channel's Shared flag
		if err = s.Channel().SetShared(channelID, false); err != nil {
			return false, errors.Wrap(err, "error unsetting channel share flag")
		}
	}

	if err = transaction.Commit(); err != nil {
		return false, errors.Wrap(err, "commit_transaction")
	}

	return count > 0, nil
}

// SaveRemote inserts a new shared channel remote record.
func (s SQLSharedChannelStore) SaveRemote(remote *model.SharedChannelRemote) (*model.SharedChannelRemote, error) {
	remote.PreSave()
	if err := remote.IsValid(); err != nil {
		return nil, err
	}

	// make sure the shared channel remote is associated with a real channel.
	if _, err := s.stores.channel.Get(remote.ChannelID, true); err != nil {
		return nil, fmt.Errorf("invalid channel: %w", err)
	}

	if err := s.GetMaster().Insert(remote); err != nil {
		return nil, errors.Wrapf(err, "save_shared_channel_remote: channel_id=%s, id=%s", remote.ChannelID, remote.ID)
	}
	return remote, nil
}

// Update updates the shared channel remote.
func (s SQLSharedChannelStore) UpdateRemote(remote *model.SharedChannelRemote) (*model.SharedChannelRemote, error) {
	if err := remote.IsValid(); err != nil {
		return nil, err
	}

	count, err := s.GetMaster().Update(remote)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update shared channel remote with remoteId=%s", remote.ID)
	}

	if count != 1 {
		return nil, fmt.Errorf("expected number of shared channel remotes to be updated is 1 but was %d", count)
	}
	return remote, nil
}

// GetRemote fetches a shared channel remote by id.
func (s SQLSharedChannelStore) GetRemote(id string) (*model.SharedChannelRemote, error) {
	var remote model.SharedChannelRemote

	query := s.getQueryBuilder().
		Select("*").
		From("SharedChannelRemotes").
		Where(sq.Eq{"SharedChannelRemotes.Id": id})

	squery, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "get_shared_channel_remote_tosql")
	}

	if err := s.GetReplica().SelectOne(&remote, squery, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("SharedChannelRemote", id)
		}
		return nil, errors.Wrapf(err, "failed to find shared channel remote with id=%s", id)
	}
	return &remote, nil
}

// GetRemoteByIds fetches a shared channel remote by channel id and remote cluster id.
func (s SQLSharedChannelStore) GetRemoteByIDs(channelID string, remoteID string) (*model.SharedChannelRemote, error) {
	var remote model.SharedChannelRemote

	query := s.getQueryBuilder().
		Select("*").
		From("SharedChannelRemotes").
		Where(sq.Eq{"SharedChannelRemotes.ChannelId": channelID}).
		Where(sq.Eq{"SharedChannelRemotes.RemoteId": remoteID})

	squery, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "get_shared_channel_remote_by_ids_tosql")
	}

	if err := s.GetReplica().SelectOne(&remote, squery, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("SharedChannelRemote", fmt.Sprintf("channelId=%s, remoteId=%s", channelID, remoteID))
		}
		return nil, errors.Wrapf(err, "failed to find shared channel remote with channelId=%s, remoteId=%s", channelID, remoteID)
	}
	return &remote, nil
}

// GetRemotes fetches all shared channel remotes associated with channel_id.
func (s SQLSharedChannelStore) GetRemotes(opts model.SharedChannelRemoteFilterOpts) ([]*model.SharedChannelRemote, error) {
	var remotes []*model.SharedChannelRemote

	query := s.getQueryBuilder().
		Select("*").
		From("SharedChannelRemotes")

	if opts.ChannelID != "" {
		query = query.Where(sq.Eq{"ChannelId": opts.ChannelID})
	}

	if opts.RemoteID != "" {
		query = query.Where(sq.Eq{"RemoteId": opts.RemoteID})
	}

	if !opts.InclUnconfirmed {
		query = query.Where(sq.Eq{"IsInviteConfirmed": true})
	}

	squery, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "get_shared_channel_remotes_tosql")
	}

	if _, err := s.GetReplica().Select(&remotes, squery, args...); err != nil {
		if err != sql.ErrNoRows {
			return nil, errors.Wrapf(err, "failed to get shared channel remotes for channel_id=%s; remote_id=%s",
				opts.ChannelID, opts.RemoteID)
		}
	}
	return remotes, nil
}

// HasRemote returns whether a given remoteId and channelId are present in the shared channel remotes or not.
func (s SQLSharedChannelStore) HasRemote(channelID string, remoteID string) (bool, error) {
	builder := s.getQueryBuilder().
		Select("1").
		Prefix("SELECT EXISTS (").
		From("SharedChannelRemotes").
		Where(sq.Eq{"RemoteId": remoteID}).
		Where(sq.Eq{"ChannelId": channelID}).
		Suffix(")")

	query, args, err := builder.ToSql()
	if err != nil {
		return false, errors.Wrapf(err, "get_shared_channel_hasremote_tosql")
	}

	var hasRemote bool
	if err := s.GetReplica().SelectOne(&hasRemote, query, args...); err != nil {
		return hasRemote, errors.Wrapf(err, "failed to get channel remotes for channel_id=%s", channelID)
	}
	return hasRemote, nil
}

// GetRemoteForUser returns a remote cluster for the given userId only if the user belongs to at least one channel
// shared with the remote.
func (s SQLSharedChannelStore) GetRemoteForUser(remoteID string, userID string) (*model.RemoteCluster, error) {
	builder := s.getQueryBuilder().
		Select("rc.*").
		From("RemoteClusters AS rc").
		Join("SharedChannelRemotes AS scr ON rc.RemoteId = scr.RemoteId").
		Join("ChannelMembers AS cm ON scr.ChannelId = cm.ChannelId").
		Where(sq.Eq{"rc.RemoteId": remoteID}).
		Where(sq.Eq{"cm.UserId": userID})

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "get_remote_for_user_tosql")
	}

	var rc model.RemoteCluster
	if err := s.GetReplica().SelectOne(&rc, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("RemoteCluster", remoteID)
		}
		return nil, errors.Wrapf(err, "failed to get remote for user_id=%s", userID)
	}
	return &rc, nil
}

// UpdateRemoteCursor updates the LastPostUpdateAt timestamp and LastPostId for the specified SharedChannelRemote.
func (s SQLSharedChannelStore) UpdateRemoteCursor(id string, cursor model.GetPostsSinceForSyncCursor) error {
	squery, args, err := s.getQueryBuilder().
		Update("SharedChannelRemotes").
		Set("LastPostUpdateAt", cursor.LastPostUpdateAt).
		Set("LastPostId", cursor.LastPostID).
		Where(sq.Eq{"Id": id}).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "update_shared_channel_remote_cursor_tosql")
	}

	result, err := s.GetMaster().Exec(squery, args...)
	if err != nil {
		return errors.Wrap(err, "failed to update cursor for SharedChannelRemote")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to determine rows affected")
	}
	if count == 0 {
		return fmt.Errorf("id not found: %s", id)
	}
	return nil
}

// DeleteRemote deletes a single shared channel remote.
// Returns true if remote found and deleted, false if not found.
func (s SQLSharedChannelStore) DeleteRemote(id string) (bool, error) {
	squery, args, err := s.getQueryBuilder().
		Delete("SharedChannelRemotes").
		Where(sq.Eq{"Id": id}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "delete_shared_channel_remote_tosql")
	}

	result, err := s.GetMaster().Exec(squery, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to delete SharedChannelRemote")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "failed to determine rows affected")
	}

	return count > 0, nil
}

// GetRemotesStatus returns the status for each remote invited to the
// specified shared channel.
func (s SQLSharedChannelStore) GetRemotesStatus(channelID string) ([]*model.SharedChannelRemoteStatus, error) {
	var status []*model.SharedChannelRemoteStatus

	query := s.getQueryBuilder().
		Select("scr.ChannelId, rc.DisplayName, rc.SiteURL, rc.LastPingAt, scr.NextSyncAt, sc.ReadOnly, scr.IsInviteAccepted").
		From("SharedChannelRemotes scr, RemoteClusters rc, SharedChannels sc").
		Where("scr.RemoteId = rc.RemoteId").
		Where("scr.ChannelId = sc.ChannelId").
		Where(sq.Eq{"scr.ChannelId": channelID})

	squery, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "get_shared_channel_remotes_status_tosql")
	}

	if _, err := s.GetReplica().Select(&status, squery, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("SharedChannelRemoteStatus", channelID)
		}
		return nil, errors.Wrapf(err, "failed to get shared channel remote status for channel_id=%s", channelID)
	}
	return status, nil
}

// SaveUser inserts a new shared channel user record to the SharedChannelUsers table.
func (s SQLSharedChannelStore) SaveUser(scUser *model.SharedChannelUser) (*model.SharedChannelUser, error) {
	scUser.PreSave()
	if err := scUser.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(scUser); err != nil {
		return nil, errors.Wrapf(err, "save_shared_channel_user: user_id=%s, remote_id=%s", scUser.UserID, scUser.RemoteID)
	}
	return scUser, nil
}

// GetSingleUser fetches a shared channel user based on userID, channelID and remoteID.
func (s SQLSharedChannelStore) GetSingleUser(userID string, channelID string, remoteID string) (*model.SharedChannelUser, error) {
	var scu model.SharedChannelUser

	squery, args, err := s.getQueryBuilder().
		Select("*").
		From("SharedChannelUsers").
		Where(sq.Eq{"SharedChannelUsers.UserId": userID}).
		Where(sq.Eq{"SharedChannelUsers.RemoteId": remoteID}).
		Where(sq.Eq{"SharedChannelUsers.ChannelId": channelID}).
		ToSql()

	if err != nil {
		return nil, errors.Wrapf(err, "getsharedchannelsingleuser_tosql")
	}

	if err := s.GetReplica().SelectOne(&scu, squery, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("SharedChannelUser", userID)
		}
		return nil, errors.Wrapf(err, "failed to find shared channel user with UserId=%s, ChannelId=%s, RemoteId=%s", userID, channelID, remoteID)
	}
	return &scu, nil
}

// GetUsersForUser fetches all shared channel user records based on userID.
func (s SQLSharedChannelStore) GetUsersForUser(userID string) ([]*model.SharedChannelUser, error) {
	squery, args, err := s.getQueryBuilder().
		Select("*").
		From("SharedChannelUsers").
		Where(sq.Eq{"SharedChannelUsers.UserId": userID}).
		ToSql()

	if err != nil {
		return nil, errors.Wrapf(err, "getsharedchanneluser_tosql")
	}

	var users []*model.SharedChannelUser
	if _, err := s.GetReplica().Select(&users, squery, args...); err != nil {
		if err == sql.ErrNoRows {
			return make([]*model.SharedChannelUser, 0), nil
		}
		return nil, errors.Wrapf(err, "failed to find shared channel user with UserId=%s", userID)
	}
	return users, nil
}

// GetUsersForSync fetches all shared channel users that need to be synchronized, meaning their
// `SharedChannelUsers.LastSyncAt` is less than or equal to `User.UpdateAt`.
func (s SQLSharedChannelStore) GetUsersForSync(filter model.GetUsersForSyncFilter) ([]*model.User, error) {
	if filter.Limit <= 0 {
		filter.Limit = DefaultGetUsersForSyncLimit
	}

	query := s.getQueryBuilder().
		Select("u.*").
		Distinct().
		From("Users AS u").
		Join("SharedChannelUsers AS scu ON u.Id = scu.UserId").
		OrderBy("u.Id").
		Limit(filter.Limit)

	if filter.CheckProfileImage {
		query = query.Where("scu.LastSyncAt < u.LastPictureUpdate")
	} else {
		query = query.Where("scu.LastSyncAt < u.UpdateAt")
	}

	if filter.ChannelID != "" {
		query = query.Where(sq.Eq{"scu.ChannelId": filter.ChannelID})
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrapf(err, "getsharedchannelusersforsync_tosql")
	}

	var users []*model.User
	if _, err := s.GetReplica().Select(&users, sqlQuery, args...); err != nil {
		if err == sql.ErrNoRows {
			return make([]*model.User, 0), nil
		}
		return nil, errors.Wrapf(err, "failed to fetch shared channel users with ChannelId=%s",
			filter.ChannelID)
	}
	return users, nil
}

// UpdateUserLastSyncAt updates the LastSyncAt timestamp for the specified SharedChannelUser.
func (s SQLSharedChannelStore) UpdateUserLastSyncAt(userID string, channelID string, remoteID string) error {
	args := map[string]interface{}{"UserId": userID, "ChannelId": channelID, "RemoteId": remoteID}

	var query string
	if s.DriverName() == model.DatabaseDriverPostgres {
		query = `
		UPDATE
			SharedChannelUsers AS scu
		SET
			LastSyncAt = GREATEST(Users.UpdateAt, Users.LastPictureUpdate)
		FROM
			Users
		WHERE
			Users.Id = scu.UserId AND scu.UserId = :UserId AND scu.ChannelId = :ChannelId AND scu.RemoteId = :RemoteId
		`
	} else if s.DriverName() == model.DatabaseDriverMysql {
		query = `
		UPDATE
			SharedChannelUsers AS scu
		INNER JOIN
			Users ON scu.UserId = Users.Id
		SET
			LastSyncAt = GREATEST(Users.UpdateAt, Users.LastPictureUpdate)
		WHERE
			scu.UserId = :UserId AND scu.ChannelId = :ChannelId AND scu.RemoteId = :RemoteId
		`
	} else {
		return errors.New("unsupported DB driver " + s.DriverName())
	}

	result, err := s.GetMaster().Exec(query, args)
	if err != nil {
		return fmt.Errorf("failed to update LastSyncAt for SharedChannelUser with userId=%s, channelId=%s, remoteId=%s: %w",
			userID, channelID, remoteID, err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to determine rows affected")
	}
	if count == 0 {
		return fmt.Errorf("SharedChannelUser not found: userId=%s, channelId=%s, remoteId=%s", userID, channelID, remoteID)
	}
	return nil
}

// SaveAttachment inserts a new shared channel file attachment record to the SharedChannelFiles table.
func (s SQLSharedChannelStore) SaveAttachment(attachment *model.SharedChannelAttachment) (*model.SharedChannelAttachment, error) {
	attachment.PreSave()
	if err := attachment.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(attachment); err != nil {
		return nil, errors.Wrapf(err, "save_shared_channel_attachment: file_id=%s, remote_id=%s", attachment.FileID, attachment.RemoteID)
	}
	return attachment, nil
}

// UpsertAttachment inserts a new shared channel file attachment record to the SharedChannelFiles table or updates its
// LastSyncAt.
func (s SQLSharedChannelStore) UpsertAttachment(attachment *model.SharedChannelAttachment) (string, error) {
	attachment.PreSave()
	if err := attachment.IsValid(); err != nil {
		return "", err
	}

	params := map[string]interface{}{
		"Id":         attachment.ID,
		"FileId":     attachment.FileID,
		"RemoteId":   attachment.RemoteID,
		"CreateAt":   attachment.CreateAt,
		"LastSyncAt": attachment.LastSyncAt,
	}

	if s.DriverName() == model.DatabaseDriverMysql {
		if _, err := s.GetMaster().Exec(
			`INSERT INTO
				SharedChannelAttachments
				(Id, FileId, RemoteId, CreateAt, LastSyncAt)
			VALUES
				(:Id, :FileId, :RemoteId, :CreateAt, :LastSyncAt)
			ON DUPLICATE KEY UPDATE
				LastSyncAt = :LastSyncAt`, params); err != nil {
			return "", err
		}
	} else if s.DriverName() == model.DatabaseDriverPostgres {
		if _, err := s.GetMaster().Exec(
			`INSERT INTO
				SharedChannelAttachments
				(Id, FileId, RemoteId, CreateAt, LastSyncAt)
			VALUES
				(:Id, :FileId, :RemoteId, :CreateAt, :LastSyncAt)
			ON CONFLICT (Id)
				DO UPDATE SET LastSyncAt = :LastSyncAt`, params); err != nil {
			return "", err
		}
	}
	return attachment.ID, nil
}

// GetAttachment fetches a shared channel file attachment record based on file_id and remoteId.
func (s SQLSharedChannelStore) GetAttachment(fileID string, remoteID string) (*model.SharedChannelAttachment, error) {
	var attachment model.SharedChannelAttachment

	squery, args, err := s.getQueryBuilder().
		Select("*").
		From("SharedChannelAttachments").
		Where(sq.Eq{"SharedChannelAttachments.FileId": fileID}).
		Where(sq.Eq{"SharedChannelAttachments.RemoteId": remoteID}).
		ToSql()

	if err != nil {
		return nil, errors.Wrapf(err, "getsharedchannelattachment_tosql")
	}

	if err := s.GetReplica().SelectOne(&attachment, squery, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("SharedChannelAttachment", fileID)
		}
		return nil, errors.Wrapf(err, "failed to find shared channel attachment with FileId=%s, RemoteId=%s", fileID, remoteID)
	}
	return &attachment, nil
}

// UpdateAttachmentLastSyncAt updates the LastSyncAt timestamp for the specified SharedChannelAttachment.
func (s SQLSharedChannelStore) UpdateAttachmentLastSyncAt(id string, syncTime int64) error {
	squery, args, err := s.getQueryBuilder().
		Update("SharedChannelAttachments").
		Set("LastSyncAt", syncTime).
		Where(sq.Eq{"Id": id}).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "update_shared_channel_attachment_last_sync_at_tosql")
	}

	result, err := s.GetMaster().Exec(squery, args...)
	if err != nil {
		return errors.Wrap(err, "failed to update LastSycnAt for SharedChannelAttachment")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to determine rows affected")
	}
	if count == 0 {
		return fmt.Errorf("id not found: %s", id)
	}
	return nil
}
