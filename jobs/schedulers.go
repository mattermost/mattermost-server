// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package jobs

import (
	"errors"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type Schedulers struct {
	stop                 chan bool
	stopped              chan bool
	configChanged        chan *model.Config
	clusterLeaderChanged chan bool
	listenerID           string
	jobs                 *JobServer
	isLeader             bool
	running              bool

	schedulers   []model.Scheduler
	nextRunTimes []*time.Time
}

var (
	ErrSchedulersNotRunning    = errors.New("job schedulers are not running")
	ErrSchedulersRunning       = errors.New("job schedulers are running")
	ErrSchedulersUninitialized = errors.New("job schedulers are not initialized")
)

func (srv *JobServer) InitSchedulers() error {
	srv.mut.Lock()
	defer srv.mut.Unlock()
	if srv.schedulers != nil && srv.schedulers.running {
		return ErrSchedulersRunning
	}
	mlog.Debug("Initialising schedulers.")

	schedulers := &Schedulers{
		stop:                 make(chan bool),
		stopped:              make(chan bool),
		configChanged:        make(chan *model.Config),
		clusterLeaderChanged: make(chan bool, 1),
		jobs:                 srv,
		isLeader:             true,
	}

	if srv.DataRetentionJob != nil {
		schedulers.schedulers = append(schedulers.schedulers, srv.DataRetentionJob.MakeScheduler())
	}

	if srv.MessageExportJob != nil {
		schedulers.schedulers = append(schedulers.schedulers, srv.MessageExportJob.MakeScheduler())
	}

	if elasticsearchAggregatorInterface := srv.ElasticsearchAggregator; elasticsearchAggregatorInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, elasticsearchAggregatorInterface.MakeScheduler())
	}

	if ldapSyncInterface := srv.LdapSync; ldapSyncInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, ldapSyncInterface.MakeScheduler())
	}

	if migrationsInterface := srv.Migrations; migrationsInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, migrationsInterface.MakeScheduler())
	}

	if pluginsInterface := srv.Plugins; pluginsInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, pluginsInterface.MakeScheduler())
	}

	if expiryNotifyInterface := srv.ExpiryNotify; expiryNotifyInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, expiryNotifyInterface.MakeScheduler())
	}

	if activeUsersInterface := srv.ActiveUsers; activeUsersInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, activeUsersInterface.MakeScheduler())
	}

	if productNoticesInterface := srv.ProductNotices; productNoticesInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, productNoticesInterface.MakeScheduler())
	}

	if cloudInterface := srv.Cloud; cloudInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, cloudInterface.MakeScheduler())
	}

	if resendInvitationEmailInterface := srv.ResendInvitationEmails; resendInvitationEmailInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, resendInvitationEmailInterface.MakeScheduler())
	}

	if importDeleteInterface := srv.ImportDelete; importDeleteInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, importDeleteInterface.MakeScheduler())
	}

	if exportDeleteInterface := srv.ExportDelete; exportDeleteInterface != nil {
		schedulers.schedulers = append(schedulers.schedulers, exportDeleteInterface.MakeScheduler())
	}

	schedulers.nextRunTimes = make([]*time.Time, len(schedulers.schedulers))
	srv.schedulers = schedulers

	return nil
}

// Start starts the schedulers. This call is not safe for concurrent use.
// Synchronization should be implemented by the caller.
func (schedulers *Schedulers) Start() {
	schedulers.listenerID = schedulers.jobs.ConfigService.AddConfigListener(schedulers.handleConfigChange)

	go func() {
		mlog.Info("Starting schedulers.")

		defer func() {
			mlog.Info("Schedulers stopped.")
			close(schedulers.stopped)
		}()

		now := time.Now()
		for idx, scheduler := range schedulers.schedulers {
			if !scheduler.Enabled(schedulers.jobs.Config()) {
				schedulers.nextRunTimes[idx] = nil
			} else {
				schedulers.setNextRunTime(schedulers.jobs.Config(), idx, now, false)
			}
		}

		for {
			timer := time.NewTimer(1 * time.Minute)
			select {
			case <-schedulers.stop:
				mlog.Debug("Schedulers received stop signal.")
				timer.Stop()
				return
			case now = <-timer.C:
				cfg := schedulers.jobs.Config()

				for idx, nextTime := range schedulers.nextRunTimes {
					if nextTime == nil {
						continue
					}

					if time.Now().After(*nextTime) {
						scheduler := schedulers.schedulers[idx]
						if scheduler == nil || !schedulers.isLeader || !scheduler.Enabled(cfg) {
							continue
						}
						if _, err := schedulers.scheduleJob(cfg, scheduler); err != nil {
							mlog.Error("Failed to schedule job", mlog.String("scheduler", scheduler.Name()), mlog.Err(err))
							continue
						}
						schedulers.setNextRunTime(cfg, idx, now, true)
					}
				}
			case newCfg := <-schedulers.configChanged:
				for idx, scheduler := range schedulers.schedulers {
					if !schedulers.isLeader || !scheduler.Enabled(newCfg) {
						schedulers.nextRunTimes[idx] = nil
					} else {
						schedulers.setNextRunTime(newCfg, idx, now, false)
					}
				}
			case isLeader := <-schedulers.clusterLeaderChanged:
				for idx := range schedulers.schedulers {
					schedulers.isLeader = isLeader
					if !isLeader {
						schedulers.nextRunTimes[idx] = nil
					} else {
						schedulers.setNextRunTime(schedulers.jobs.Config(), idx, now, false)
					}
				}
			}
			timer.Stop()
		}
	}()

	schedulers.running = true
}

// Stop stops the schedulers. This call is not safe for concurrent use.
// Synchronization should be implemented by the caller.
func (schedulers *Schedulers) Stop() {
	mlog.Info("Stopping schedulers.")
	close(schedulers.stop)
	<-schedulers.stopped
	schedulers.jobs.ConfigService.RemoveConfigListener(schedulers.listenerID)
	schedulers.listenerID = ""
	schedulers.running = false
}

func (schedulers *Schedulers) setNextRunTime(cfg *model.Config, idx int, now time.Time, pendingJobs bool) {
	scheduler := schedulers.schedulers[idx]

	if !pendingJobs {
		pj, err := schedulers.jobs.CheckForPendingJobsByType(scheduler.JobType())
		if err != nil {
			mlog.Error("Failed to set next job run time", mlog.Err(err))
			schedulers.nextRunTimes[idx] = nil
			return
		}
		pendingJobs = pj
	}

	lastSuccessfulJob, err := schedulers.jobs.GetLastSuccessfulJobByType(scheduler.JobType())
	if err != nil {
		mlog.Error("Failed to set next job run time", mlog.Err(err))
		schedulers.nextRunTimes[idx] = nil
		return
	}

	schedulers.nextRunTimes[idx] = scheduler.NextScheduleTime(cfg, now, pendingJobs, lastSuccessfulJob)
	mlog.Debug("Next run time for scheduler", mlog.String("scheduler_name", scheduler.Name()), mlog.String("next_runtime", fmt.Sprintf("%v", schedulers.nextRunTimes[idx])))
}

func (schedulers *Schedulers) scheduleJob(cfg *model.Config, scheduler model.Scheduler) (*model.Job, *model.AppError) {
	pendingJobs, err := schedulers.jobs.CheckForPendingJobsByType(scheduler.JobType())
	if err != nil {
		return nil, err
	}

	lastSuccessfulJob, err2 := schedulers.jobs.GetLastSuccessfulJobByType(scheduler.JobType())
	if err2 != nil {
		return nil, err
	}

	return scheduler.ScheduleJob(cfg, pendingJobs, lastSuccessfulJob)
}

func (schedulers *Schedulers) handleConfigChange(_, newConfig *model.Config) {
	mlog.Debug("Schedulers received config change.")
	select {
	case schedulers.configChanged <- newConfig:
	case <-schedulers.stop:
	}
}

func (schedulers *Schedulers) handleClusterLeaderChange(isLeader bool) {
	select {
	case schedulers.clusterLeaderChanged <- isLeader:
	default:
		mlog.Debug("Sending cluster leader change message to schedulers failed.")

		// Drain the buffered channel to make room for the latest change.
		select {
		case <-schedulers.clusterLeaderChanged:
		default:
		}

		// Enqueue the latest change. This operation is safe due to this method
		// being called under lock.
		schedulers.clusterLeaderChanged <- isLeader
	}
}
