// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin

import (
	"sync"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	HealthCheckInterval           = 30 * time.Second // How often the health check should run
	HealthCheckDeactivationWindow = 60 * time.Minute // How long we wait for num fails to occur before deactivating the plugin
	HealthCheckPingFailLimit      = 3                // How many times we call RPC ping in a row before it is considered a failure
	HealthCheckNumRestartsLimit   = 3                // How many times we restart a plugin before we deactivate it
)

type PluginHealthCheckJob struct {
	cancel            chan struct{}
	cancelled         chan struct{}
	cancelOnce        sync.Once
	env               *Environment
	failureTimestamps sync.Map
}

// run continuously performs health checks on all active plugins, on a timer.
func (job *PluginHealthCheckJob) run() {
	mlog.Debug("Plugin health check job starting.")
	defer close(job.cancelled)

	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			activePlugins := job.env.Active()
			for _, plugin := range activePlugins {
				job.CheckPlugin(plugin.Manifest.ID)
			}
		case <-job.cancel:
			return
		}
	}
}

// CheckPlugin determines the plugin's health status, then handles the error or success case.
// If the plugin passes the health check, do nothing.
// If the plugin fails the health check, the function either restarts or deactivates the plugin, based on the quantity and frequency of its failures.
func (job *PluginHealthCheckJob) CheckPlugin(id string) {
	err := job.env.PerformHealthCheck(id)
	if err == nil {
		return
	}

	mlog.Warn("Health check failed for plugin", mlog.String("id", id), mlog.Err(err))
	timestamps := job.getStoredTimestamps(id)
	timestamps = append(timestamps, time.Now())

	if shouldDeactivatePlugin(timestamps) {
		// Order matters here, must deactivate first and then set plugin state
		mlog.Debug("Deactivating plugin due to multiple crashes", mlog.String("id", id))
		job.env.Deactivate(id)

		// Reset timestamp state for this plugin
		job.failureTimestamps.Delete(id)
		job.env.setPluginState(id, model.PluginStateFailedToStayRunning)
	} else {
		mlog.Debug("Restarting plugin due to failed health check", mlog.String("id", id))
		if err := job.env.RestartPlugin(id); err != nil {
			mlog.Error("Failed to restart plugin", mlog.String("id", id), mlog.Err(err))
		}

		// Store this failure so we can continue to monitor the plugin
		job.failureTimestamps.Store(id, removeStaleTimestamps(timestamps))
	}
}

// getStoredTimestamps returns the stored failure timestamps for a plugin.
func (job *PluginHealthCheckJob) getStoredTimestamps(id string) []time.Time {
	timestamps, ok := job.failureTimestamps.Load(id)
	if !ok {
		timestamps = []time.Time{}
	}
	return timestamps.([]time.Time)
}

func newPluginHealthCheckJob(env *Environment) *PluginHealthCheckJob {
	return &PluginHealthCheckJob{
		cancel:    make(chan struct{}),
		cancelled: make(chan struct{}),
		env:       env,
	}
}

func (job *PluginHealthCheckJob) Cancel() {
	job.cancelOnce.Do(func() {
		close(job.cancel)
	})
	<-job.cancelled
}

// shouldDeactivatePlugin determines if a plugin needs to be deactivated after the plugin has failed (HealthCheckNumRestartsLimit) times,
// within the configured time window (HealthCheckDeactivationWindow).
func shouldDeactivatePlugin(failedTimestamps []time.Time) bool {
	if len(failedTimestamps) < HealthCheckNumRestartsLimit {
		return false
	}

	index := len(failedTimestamps) - HealthCheckNumRestartsLimit
	return time.Since(failedTimestamps[index]) <= HealthCheckDeactivationWindow
}

// removeStaleTimestamps only keeps the last HealthCheckNumRestartsLimit items in timestamps.
func removeStaleTimestamps(timestamps []time.Time) []time.Time {
	if len(timestamps) > HealthCheckNumRestartsLimit {
		timestamps = timestamps[len(timestamps)-HealthCheckNumRestartsLimit:]
	}

	return timestamps
}
