// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"bytes"
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	defaultPluginKeyFetchLimit = 10
)

type SQLPluginStore struct {
	*SQLStore
}

func newSQLPluginStore(sqlStore *SQLStore) store.PluginStore {
	s := &SQLPluginStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.PluginKeyValue{}, "PluginKeyValueStore").SetKeys(false, "PluginId", "Key")
		table.ColMap("PluginId").SetMaxSize(190)
		table.ColMap("Key").SetMaxSize(50)
		table.ColMap("Value").SetMaxSize(8192)
	}

	return s
}

func (ps SQLPluginStore) createIndexesIfNotExists() {
}

func (ps SQLPluginStore) SaveOrUpdate(kv *model.PluginKeyValue) (*model.PluginKeyValue, error) {
	if err := kv.IsValid(); err != nil {
		return nil, err
	}

	if kv.Value == nil {
		// Setting a key to nil is the same as removing it
		err := ps.Delete(kv.PluginID, kv.Key)
		if err != nil {
			return nil, err
		}

		return kv, nil
	}

	if ps.DriverName() == model.DatabaseDriverPostgres {
		// Unfortunately PostgreSQL pre-9.5 does not have an atomic upsert, so we use
		// separate update and insert queries to accomplish our upsert
		if rowsAffected, err := ps.GetMaster().Update(kv); err != nil {
			return nil, errors.Wrap(err, "failed to update PluginKeyValue")
		} else if rowsAffected == 0 {
			// No rows were affected by the update, so let's try an insert
			if err := ps.GetMaster().Insert(kv); err != nil {
				return nil, errors.Wrap(err, "failed to save PluginKeyValue")
			}
		}
	} else if ps.DriverName() == model.DatabaseDriverMysql {
		query := ps.getQueryBuilder().
			Insert("PluginKeyValueStore").
			Columns("PluginId", "PKey", "PValue", "ExpireAt").
			Values(kv.PluginID, kv.Key, kv.Value, kv.ExpireAt).
			SuffixExpr(sq.Expr("ON DUPLICATE KEY UPDATE PValue = ?, ExpireAt = ?", kv.Value, kv.ExpireAt))

		queryString, args, err := query.ToSql()
		if err != nil {
			return nil, errors.Wrap(err, "plugin_tosql")
		}

		if _, err := ps.GetMaster().Exec(queryString, args...); err != nil {
			return nil, errors.Wrap(err, "failed to upsert PluginKeyValue")
		}
	}

	return kv, nil
}

func (ps SQLPluginStore) CompareAndSet(kv *model.PluginKeyValue, oldValue []byte) (bool, error) {
	if err := kv.IsValid(); err != nil {
		return false, err
	}

	if kv.Value == nil {
		// Setting a key to nil is the same as removing it
		return ps.CompareAndDelete(kv, oldValue)
	}

	if oldValue == nil {
		// Delete any existing, expired value.
		query := ps.getQueryBuilder().
			Delete("PluginKeyValueStore").
			Where(sq.Eq{"PluginId": kv.PluginID}).
			Where(sq.Eq{"PKey": kv.Key}).
			Where(sq.NotEq{"ExpireAt": int(0)}).
			Where(sq.Lt{"ExpireAt": model.GetMillis()})

		queryString, args, err := query.ToSql()
		if err != nil {
			return false, errors.Wrap(err, "plugin_tosql")
		}

		if _, err := ps.GetMaster().Exec(queryString, args...); err != nil {
			return false, errors.Wrap(err, "failed to delete PluginKeyValue")
		}

		// Insert if oldValue is nil
		if err := ps.GetMaster().Insert(kv); err != nil {
			// If the error is from unique constraints violation, it's the result of a
			// race condition, return false and no error. Otherwise we have a real error and
			// need to return it.
			if IsUniqueConstraintError(err, []string{"PRIMARY", "PluginId", "Key", "PKey", "pkey"}) {
				return false, nil
			}
			return false, errors.Wrap(err, "failed to insert PluginKeyValue")
		}
	} else {
		currentTime := model.GetMillis()

		// Update if oldValue is not nil
		query := ps.getQueryBuilder().
			Update("PluginKeyValueStore").
			Set("PValue", kv.Value).
			Set("ExpireAt", kv.ExpireAt).
			Where(sq.Eq{"PluginId": kv.PluginID}).
			Where(sq.Eq{"PKey": kv.Key}).
			Where(sq.Eq{"PValue": oldValue}).
			Where(sq.Or{
				sq.Eq{"ExpireAt": int(0)},
				sq.Gt{"ExpireAt": currentTime},
			})

		queryString, args, err := query.ToSql()
		if err != nil {
			return false, errors.Wrap(err, "plugin_tosql")
		}

		updateResult, err := ps.GetMaster().Exec(queryString, args...)
		if err != nil {
			return false, errors.Wrap(err, "failed to update PluginKeyValue")
		}

		if rowsAffected, err := updateResult.RowsAffected(); err != nil {
			// Failed to update
			return false, errors.Wrap(err, "unable to get rows affected")
		} else if rowsAffected == 0 {
			if ps.DriverName() == model.DatabaseDriverMysql && bytes.Equal(oldValue, kv.Value) {
				// ROW_COUNT on MySQL is zero even if the row existed but no changes to the row were required.
				// Check if the row exists with the required value to distinguish this case. Strictly speaking,
				// this isn't a good use of CompareAndSet anyway, since there's no corresponding guarantee of
				// atomicity. Nevertheless, let's return results consistent with Postgres and with what might
				// be expected in this case.
				query := ps.getQueryBuilder().
					Select("COUNT(*)").
					From("PluginKeyValueStore").
					Where(sq.Eq{"PluginId": kv.PluginID}).
					Where(sq.Eq{"PKey": kv.Key}).
					Where(sq.Eq{"PValue": kv.Value}).
					Where(sq.Or{
						sq.Eq{"ExpireAt": int(0)},
						sq.Gt{"ExpireAt": currentTime},
					})

				queryString, args, err := query.ToSql()
				if err != nil {
					return false, errors.Wrap(err, "plugin_tosql")
				}

				count, err := ps.GetReplica().SelectInt(queryString, args...)
				if err != nil {
					return false, errors.Wrapf(err, "failed to count PluginKeyValue with pluginId=%s and key=%s", kv.PluginID, kv.Key)
				}

				if count == 0 {
					return false, nil
				} else if count == 1 {
					return true, nil
				} else {
					return false, errors.Wrapf(err, "got too many rows when counting PluginKeyValue with pluginId=%s, key=%s, rows=%d", kv.PluginID, kv.Key, count)
				}
			}

			// No rows were affected by the update, where condition was not satisfied,
			// return false, but no error.
			return false, nil
		}
	}

	return true, nil
}

func (ps SQLPluginStore) CompareAndDelete(kv *model.PluginKeyValue, oldValue []byte) (bool, error) {
	if err := kv.IsValid(); err != nil {
		return false, err
	}

	if oldValue == nil {
		// nil can't be stored. Return showing that we didn't do anything
		return false, nil
	}

	query := ps.getQueryBuilder().
		Delete("PluginKeyValueStore").
		Where(sq.Eq{"PluginId": kv.PluginID}).
		Where(sq.Eq{"PKey": kv.Key}).
		Where(sq.Eq{"PValue": oldValue}).
		Where(sq.Or{
			sq.Eq{"ExpireAt": int(0)},
			sq.Gt{"ExpireAt": model.GetMillis()},
		})

	queryString, args, err := query.ToSql()
	if err != nil {
		return false, errors.Wrap(err, "plugin_tosql")
	}

	deleteResult, err := ps.GetMaster().Exec(queryString, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to delete PluginKeyValue")
	}

	if rowsAffected, err := deleteResult.RowsAffected(); err != nil {
		return false, errors.Wrap(err, "unable to get rows affected")
	} else if rowsAffected == 0 {
		return false, nil
	}

	return true, nil
}

func (ps SQLPluginStore) SetWithOptions(pluginID string, key string, value []byte, opt model.PluginKVSetOptions) (bool, error) {
	if err := opt.IsValid(); err != nil {
		return false, err
	}

	kv, err := model.NewPluginKeyValueFromOptions(pluginID, key, value, opt)
	if err != nil {
		return false, err
	}

	if opt.Atomic {
		return ps.CompareAndSet(kv, opt.OldValue)
	}

	savedKv, nErr := ps.SaveOrUpdate(kv)
	if nErr != nil {
		return false, nErr
	}

	return savedKv != nil, nil
}

func (ps SQLPluginStore) Get(pluginID, key string) (*model.PluginKeyValue, error) {
	currentTime := model.GetMillis()
	query := ps.getQueryBuilder().Select("PluginId, PKey, PValue, ExpireAt").
		From("PluginKeyValueStore").
		Where(sq.Eq{"PluginId": pluginID}).
		Where(sq.Eq{"PKey": key}).
		Where(sq.Or{sq.Eq{"ExpireAt": 0}, sq.Gt{"ExpireAt": currentTime}})
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "plugin_tosql")
	}

	row := ps.GetReplica().Db.QueryRow(queryString, args...)
	var kv model.PluginKeyValue
	if err := row.Scan(&kv.PluginID, &kv.Key, &kv.Value, &kv.ExpireAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("PluginKeyValue", fmt.Sprintf("pluginId=%s, key=%s", pluginID, key))
		}
		return nil, errors.Wrapf(err, "failed to get PluginKeyValue with pluginId=%s and key=%s", pluginID, key)
	}

	return &kv, nil
}

func (ps SQLPluginStore) Delete(pluginID, key string) error {
	query := ps.getQueryBuilder().
		Delete("PluginKeyValueStore").
		Where(sq.Eq{"PluginId": pluginID}).
		Where(sq.Eq{"Pkey": key})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "plugin_tosql")
	}

	if _, err := ps.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrapf(err, "failed to delete PluginKeyValue with pluginId=%s and key=%s", pluginID, key)
	}
	return nil
}

func (ps SQLPluginStore) DeleteAllForPlugin(pluginID string) error {
	query := ps.getQueryBuilder().
		Delete("PluginKeyValueStore").
		Where(sq.Eq{"PluginId": pluginID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "plugin_tosql")
	}

	if _, err := ps.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrapf(err, "failed to get all PluginKeyValues with pluginId=%s ", pluginID)
	}
	return nil
}

func (ps SQLPluginStore) DeleteAllExpired() error {
	currentTime := model.GetMillis()
	query := ps.getQueryBuilder().
		Delete("PluginKeyValueStore").
		Where(sq.NotEq{"ExpireAt": 0}).
		Where(sq.Lt{"ExpireAt": currentTime})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "plugin_tosql")
	}

	if _, err := ps.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrap(err, "failed to delete all expired PluginKeyValues")
	}
	return nil
}

func (ps SQLPluginStore) List(pluginID string, offset int, limit int) ([]string, error) {
	if limit <= 0 {
		limit = defaultPluginKeyFetchLimit
	}

	if offset <= 0 {
		offset = 0
	}

	var keys []string

	query := ps.getQueryBuilder().
		Select("Pkey").
		From("PluginKeyValueStore").
		Where(sq.Eq{"PluginId": pluginID}).
		Where(sq.Or{
			sq.Eq{"ExpireAt": int(0)},
			sq.Gt{"ExpireAt": model.GetMillis()},
		}).
		OrderBy("PKey").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "plugin_tosql")
	}

	_, err = ps.GetReplica().Select(&keys, queryString, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get PluginKeyValues with pluginId=%s", pluginID)
	}

	return keys, nil
}
