// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

const RemoteClusterSiteURLUniqueIndex = "remote_clusters_site_url_unique"

type sqlRemoteClusterStore struct {
	*SqlStore
}

func newSqlRemoteClusterStore(sqlStore *SqlStore) store.RemoteClusterStore {
	s := &sqlRemoteClusterStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.RemoteCluster{}, "RemoteClusters").SetKeys(false, "RemoteId", "Name")
		table.ColMap("RemoteId").SetMaxSize(26)
		table.ColMap("RemoteTeamId").SetMaxSize(26)
		table.ColMap("Name").SetMaxSize(64)
		table.ColMap("DisplayName").SetMaxSize(64)
		table.ColMap("SiteURL").SetMaxSize(512)
		table.ColMap("Token").SetMaxSize(26)
		table.ColMap("RemoteToken").SetMaxSize(26)
		table.ColMap("Topics").SetMaxSize(512)
		table.ColMap("CreatorId").SetMaxSize(26)
	}
	return s
}

func (s sqlRemoteClusterStore) Save(remoteCluster *model.RemoteCluster) (*model.RemoteCluster, error) {
	remoteCluster.PreSave()
	if err := remoteCluster.IsValid(); err != nil {
		return nil, err
	}

	if err := s.GetMaster().Insert(remoteCluster); err != nil {
		return nil, errors.Wrap(err, "failed to save RemoteCluster")
	}
	return remoteCluster, nil
}

func (s sqlRemoteClusterStore) Update(remoteCluster *model.RemoteCluster) (*model.RemoteCluster, error) {
	remoteCluster.PreUpdate()
	if err := remoteCluster.IsValid(); err != nil {
		return nil, err
	}

	if _, err := s.GetMaster().Update(remoteCluster); err != nil {
		return nil, errors.Wrap(err, "failed to update RemoteCluster")
	}
	return remoteCluster, nil
}

func (s sqlRemoteClusterStore) Delete(remoteId string) (bool, error) {
	squery, args, err := s.getQueryBuilder().
		Delete("RemoteClusters").
		Where(sq.Eq{"RemoteId": remoteId}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "delete_remote_cluster_tosql")
	}

	result, err := s.GetMaster().Exec(squery, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to delete RemoteCluster")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "failed to determine rows affected")
	}

	return count > 0, nil
}

func (s sqlRemoteClusterStore) Get(remoteId string) (*model.RemoteCluster, error) {
	query := s.getQueryBuilder().
		Select("*").
		From("RemoteClusters").
		Where(sq.Eq{"RemoteId": remoteId})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "remote_cluster_get_tosql")
	}

	var rc model.RemoteCluster
	if err := s.GetReplica().SelectOne(&rc, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find RemoteCluster")
	}
	return &rc, nil
}

func (s sqlRemoteClusterStore) GetAll(filter model.RemoteClusterQueryFilter) ([]*model.RemoteCluster, error) {
	query := s.getQueryBuilder().
		Select("rc.*").
		From("RemoteClusters rc")

	if filter.InChannel != "" {
		query = query.Where("rc.RemoteId IN (SELECT scr.RemoteId FROM SharedChannelRemotes scr WHERE scr.ChannelId = ?)", filter.InChannel)
	}

	if filter.NotInChannel != "" {
		query = query.Where("rc.RemoteId NOT IN (SELECT scr.RemoteId FROM SharedChannelRemotes scr WHERE scr.ChannelId = ?)", filter.NotInChannel)
	}

	if filter.ExcludeOffline {
		query = query.Where(sq.Gt{"rc.LastPingAt": model.GetMillis() - model.RemoteOfflineAfterMillis})
	}

	if filter.CreatorId != "" {
		query = query.Where(sq.Eq{"rc.CreatorId": filter.CreatorId})
	}

	if filter.OnlyConfirmed {
		query = query.Where(sq.NotEq{"rc.SiteURL": ""})
	}

	if filter.Topic != "" {
		trimmed := strings.TrimSpace(filter.Topic)
		if trimmed == "" || trimmed == "*" {
			return nil, errors.New("invalid topic")
		}
		queryTopic := fmt.Sprintf("%% %s %%", trimmed)
		query = query.Where(sq.Or{sq.Like{"rc.Topics": queryTopic}, sq.Eq{"rc.Topics": "*"}})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "remote_cluster_getall_tosql")
	}

	var list []*model.RemoteCluster
	if _, err := s.GetReplica().Select(&list, queryString, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find RemoteClusters")
	}
	return list, nil
}

func (s sqlRemoteClusterStore) UpdateTopics(remoteClusterid string, topics string) (*model.RemoteCluster, error) {
	rc, err := s.Get(remoteClusterid)
	if err != nil {
		return nil, err
	}
	rc.Topics = topics

	rc.PreUpdate()

	if _, err = s.GetMaster().Update(rc); err != nil {
		return nil, err
	}
	return rc, nil
}

func (s sqlRemoteClusterStore) SetLastPingAt(remoteClusterId string) error {
	query := s.getQueryBuilder().
		Update("RemoteClusters").
		Set("LastPingAt", model.GetMillis()).
		Where(sq.Eq{"RemoteId": remoteClusterId})

	queryString, args, err := query.ToSql()
	if err != nil {
		return errors.Wrap(err, "remote_cluster_tosql")
	}

	if _, err := s.GetMaster().Exec(queryString, args...); err != nil {
		return errors.Wrap(err, "failed to update RemoteCluster")
	}
	return nil
}

func (s *sqlRemoteClusterStore) createIndexesIfNotExists() {
}
