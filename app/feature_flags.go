// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"os"
	"time"

	"github.com/mattermost/mattermost-server/v5/app/featureflag"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

// setupFeatureFlags called on startup and when the cluster leader changes.
// Starts or stops the synchronization of feature flags from upstream management.
func (s *Server) setupFeatureFlags() {
	s.featureFlagSynchronizerMutex.Lock()
	defer s.featureFlagSynchronizerMutex.Unlock()
	splitKey := *s.Config().ServiceSettings.SplitKey
	splitConfigured := splitKey != ""
	syncFeatureFlags := splitConfigured && s.IsLeader()

	s.configStore.SetReadOnlyFF(!splitConfigured)

	if syncFeatureFlags {
		if err := s.startFeatureFlagUpdateJob(); err != nil {
			s.Log.Warn("Unable to setup synchronization with feature flag management. Will fallback to cache.", mlog.Err(err))
		}
	} else {
		s.stopFeatureFlagUpdateJob()
	}

	if err := s.configStore.Load(); err != nil {
		s.Log.Warn("Unable to load config store after feature flag setup.", mlog.Err(err))
	}
}

func (s *Server) updateFeatureFlagValuesFromManagment() {
	newCfg := s.configStore.GetNoEnv().Clone()
	oldFlags := *newCfg.FeatureFlags
	newFlags := s.featureFlagSynchronizer.UpdateFeatureFlagValues(oldFlags)
	oldFlagsBytes, _ := json.Marshal(oldFlags)
	newFlagsBytes, _ := json.Marshal(newFlags)
	s.Log.Debug("Checking feature flags from management service", mlog.String("old_flags", string(oldFlagsBytes)), mlog.String("new_flags", string(newFlagsBytes)))
	if oldFlags != newFlags {
		s.Log.Debug("Feature flag change detected, updating config")
		*newCfg.FeatureFlags = newFlags
		s.SaveConfig(newCfg, true)
	}
}

func (s *Server) startFeatureFlagUpdateJob() error {
	// Can be run multiple times
	if s.featureFlagSynchronizer != nil {
		return nil
	}

	var log *mlog.Logger
	if *s.Config().ServiceSettings.DebugSplit {
		log = s.Log
	}

	attributes := map[string]interface{}{}

	// if we are part of a cloud installation, add its installation and group id
	if installationID := os.Getenv("MM_CLOUD_INSTALLATION_ID"); installationID != "" {
		attributes["installation_id"] = installationID
	}
	if groupID := os.Getenv("MM_CLOUD_GROUP_ID"); groupID != "" {
		attributes["group_id"] = groupID
	}

	synchronizer, err := featureflag.NewSynchronizer(featureflag.SyncParams{
		ServerID:   s.TelemetryID(),
		SplitKey:   *s.Config().ServiceSettings.SplitKey,
		Log:        log,
		Attributes: attributes,
	})
	if err != nil {
		return err
	}

	s.featureFlagStop = make(chan struct{})
	s.featureFlagStopped = make(chan struct{})
	s.featureFlagSynchronizer = synchronizer
	syncInterval := *s.Config().ServiceSettings.FeatureFlagSyncIntervalSeconds

	go func() {
		ticker := time.NewTicker(time.Duration(syncInterval) * time.Second)
		defer ticker.Stop()
		defer close(s.featureFlagStopped)
		if err := synchronizer.EnsureReady(); err != nil {
			s.Log.Warn("Problem connecting to feature flag management. Will fallback to cloud cache.", mlog.Err(err))
			return
		}
		s.updateFeatureFlagValuesFromManagment()
		for {
			select {
			case <-s.featureFlagStop:
				return
			case <-ticker.C:
				s.updateFeatureFlagValuesFromManagment()
			}
		}
	}()

	return nil
}

func (s *Server) stopFeatureFlagUpdateJob() {
	if s.featureFlagSynchronizer != nil {
		close(s.featureFlagStop)
		<-s.featureFlagStopped
		s.featureFlagSynchronizer.Close()
		s.featureFlagSynchronizer = nil
	}
}
