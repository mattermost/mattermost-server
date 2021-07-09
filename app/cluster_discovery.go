// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	DiscoveryServiceWritePing = 60 * time.Second
)

type ClusterDiscoveryService struct {
	model.ClusterDiscovery
	srv  *Server
	stop chan bool
}

func (s *Server) NewClusterDiscoveryService() *ClusterDiscoveryService {
	ds := &ClusterDiscoveryService{
		ClusterDiscovery: model.ClusterDiscovery{},
		srv:              s,
		stop:             make(chan bool),
	}

	return ds
}

func (a *App) NewClusterDiscoveryService() *ClusterDiscoveryService {
	return a.Srv().NewClusterDiscoveryService()
}

func (cds *ClusterDiscoveryService) Start() {
	err := cds.srv.Store.ClusterDiscovery().Cleanup()
	if err != nil {
		mlog.Warn("ClusterDiscoveryService failed to cleanup the outdated cluster discovery information", mlog.Err(err))
	}

	exists, err := cds.srv.Store.ClusterDiscovery().Exists(&cds.ClusterDiscovery)
	if err != nil {
		mlog.Warn("ClusterDiscoveryService failed to check if row exists", mlog.String("ClusterDiscovery", cds.ClusterDiscovery.ToJSON()), mlog.Err(err))
	} else if exists {
		if _, err := cds.srv.Store.ClusterDiscovery().Delete(&cds.ClusterDiscovery); err != nil {
			mlog.Warn("ClusterDiscoveryService failed to start clean", mlog.String("ClusterDiscovery", cds.ClusterDiscovery.ToJSON()), mlog.Err(err))
		}
	}

	if err := cds.srv.Store.ClusterDiscovery().Save(&cds.ClusterDiscovery); err != nil {
		mlog.Error("ClusterDiscoveryService failed to save", mlog.String("ClusterDiscovery", cds.ClusterDiscovery.ToJSON()), mlog.Err(err))
		return
	}

	go func() {
		mlog.Debug("ClusterDiscoveryService ping writer started", mlog.String("ClusterDiscovery", cds.ClusterDiscovery.ToJSON()))
		ticker := time.NewTicker(DiscoveryServiceWritePing)
		defer func() {
			ticker.Stop()
			if _, err := cds.srv.Store.ClusterDiscovery().Delete(&cds.ClusterDiscovery); err != nil {
				mlog.Warn("ClusterDiscoveryService failed to cleanup", mlog.String("ClusterDiscovery", cds.ClusterDiscovery.ToJSON()), mlog.Err(err))
			}
			mlog.Debug("ClusterDiscoveryService ping writer stopped", mlog.String("ClusterDiscovery", cds.ClusterDiscovery.ToJSON()))
		}()

		for {
			select {
			case <-ticker.C:
				if err := cds.srv.Store.ClusterDiscovery().SetLastPingAt(&cds.ClusterDiscovery); err != nil {
					mlog.Error("ClusterDiscoveryService failed to write ping", mlog.String("ClusterDiscovery", cds.ClusterDiscovery.ToJSON()), mlog.Err(err))
				}
			case <-cds.stop:
				return
			}
		}
	}()
}

func (cds *ClusterDiscoveryService) Stop() {
	cds.stop <- true
}

func (s *Server) IsLeader() bool {
	if s.License() != nil && *s.Config().ClusterSettings.Enable && s.Cluster != nil {
		return s.Cluster.IsLeader()
	}
	return true
}

func (a *App) IsLeader() bool {
	return a.Srv().IsLeader()
}

func (a *App) GetClusterID() string {
	if a.Cluster() == nil {
		return ""
	}

	return a.Cluster().GetClusterID()
}
