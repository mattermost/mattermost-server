// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type sqlClusterDiscoveryStore struct {
	*SQLStore
}

func newSQLClusterDiscoveryStore(sqlStore *SQLStore) store.ClusterDiscoveryStore {
	s := &sqlClusterDiscoveryStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.ClusterDiscovery{}, "ClusterDiscovery").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("Type").SetMaxSize(64)
		table.ColMap("ClusterName").SetMaxSize(64)
		table.ColMap("Hostname").SetMaxSize(512)
	}

	return s
}

func (s sqlClusterDiscoveryStore) Save(ClusterDiscovery *model.ClusterDiscovery) error {
	ClusterDiscovery.PreSave()
	if err := ClusterDiscovery.IsValid(); err != nil {
		return err
	}

	if err := s.GetMaster().Insert(ClusterDiscovery); err != nil {
		return errors.Wrap(err, "failed to save ClusterDiscovery")
	}
	return nil
}

func (s sqlClusterDiscoveryStore) Delete(ClusterDiscovery *model.ClusterDiscovery) (bool, error) {
	query := s.getQueryBuilder().
		Delete("ClusterDiscovery").
		Where(sq.Eq{"Type": ClusterDiscovery.Type}).
		Where(sq.Eq{"ClusterName": ClusterDiscovery.ClusterName}).
		Where(sq.Eq{"Hostname": ClusterDiscovery.Hostname})

	queryString, args, err := query.ToSql()
	if err != nil {
		return false, errors.Wrap(err, "cluster_discovery_tosql")
	}

	count, err := s.GetMaster().SelectInt(queryString, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to delete ClusterDiscovery")
	}
	if count == 0 {
		return false, nil
	}
	return true, nil
}

func (s sqlClusterDiscoveryStore) Exists(ClusterDiscovery *model.ClusterDiscovery) (bool, error) {
	query := s.getQueryBuilder().
		Select("COUNT(*)").
		From("ClusterDiscovery").
		Where(sq.Eq{"Type": ClusterDiscovery.Type}).
		Where(sq.Eq{"ClusterName": ClusterDiscovery.ClusterName}).
		Where(sq.Eq{"Hostname": ClusterDiscovery.Hostname})

	queryString, args, err := query.ToSql()
	if err != nil {
		return false, errors.Wrap(err, "cluster_discovery_tosql")
	}

	count, err := s.GetMaster().SelectInt(queryString, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to count ClusterDiscovery")
	}
	if count == 0 {
		return false, nil
	}
	return true, nil
}

func (s sqlClusterDiscoveryStore) GetAll(ClusterDiscoveryType, clusterName string) ([]*model.ClusterDiscovery, error) {
	query := s.getQueryBuilder().
		Select("*").
		From("ClusterDiscovery").
		Where(sq.Eq{"Type": ClusterDiscoveryType}).
		Where(sq.Eq{"ClusterName": clusterName}).
		Where(sq.Gt{"LastPingAt": model.GetMillis() - model.CDSOfflineAfterMillis})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "cluster_discovery_tosql")
	}

	var list []*model.ClusterDiscovery
	if _, err := s.GetMaster().Select(&list, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find ClusterDiscovery")
	}
	return list, nil
}

func (s sqlClusterDiscoveryStore) SetLastPingAt(ClusterDiscovery *model.ClusterDiscovery) error {
	query := s.getQueryBuilder().
		Update("ClusterDiscovery").
		Set("LastPingAt", model.GetMillis()).
		Where(sq.Eq{"Type": ClusterDiscovery.Type}).
		Where(sq.Eq{"ClusterName": ClusterDiscovery.ClusterName}).
		Where(sq.Eq{"Hostname": ClusterDiscovery.Hostname})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "cluster_discovery_tosql")
	}

	if _, err := s.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrap(err, "failed to update ClusterDiscovery")
	}
	return nil
}

func (s sqlClusterDiscoveryStore) Cleanup() error {
	query := s.getQueryBuilder().
		Delete("ClusterDiscovery").
		Where(sq.Lt{"LastPingAt": model.GetMillis() - model.CDSOfflineAfterMillis})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "cluster_discovery_tosql")
	}

	if _, err := s.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrap(err, "failed to delete ClusterDiscoveries")
	}
	return nil
}
