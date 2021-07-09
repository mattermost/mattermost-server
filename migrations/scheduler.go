// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package migrations

import (
	"time"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	MigrationJobWedgedTimeoutMilliseconds = 3600000 // 1 hour
)

type Scheduler struct {
	srv                    *app.Server
	allMigrationsCompleted bool
}

func (m *MigrationsJobInterfaceImpl) MakeScheduler() model.Scheduler {
	return &Scheduler{m.srv, false}
}

func (scheduler *Scheduler) Name() string {
	return "MigrationsScheduler"
}

func (scheduler *Scheduler) JobType() string {
	return model.JobTypeMigrations
}

func (scheduler *Scheduler) Enabled(_ *model.Config) bool {
	return true
}

//nolint:unparam
func (scheduler *Scheduler) NextScheduleTime(cfg *model.Config, now time.Time, pendingJobs bool, lastSuccessfulJob *model.Job) *time.Time {
	if scheduler.allMigrationsCompleted {
		return nil
	}

	nextTime := time.Now().Add(60 * time.Second)
	return &nextTime
}

//nolint:unparam
func (scheduler *Scheduler) ScheduleJob(cfg *model.Config, pendingJobs bool, lastSuccessfulJob *model.Job) (*model.Job, *model.AppError) {
	mlog.Debug("Scheduling Job", mlog.String("scheduler", scheduler.Name()))

	// Work through the list of migrations in order. Schedule the first one that isn't done (assuming it isn't in progress already).
	for _, key := range MakeMigrationsList() {
		state, job, err := GetMigrationState(key, scheduler.srv.Store)
		if err != nil {
			mlog.Error("Failed to determine status of migration: ", mlog.String("scheduler", scheduler.Name()), mlog.String("migration_key", key), mlog.String("error", err.Error()))
			return nil, nil
		}

		if state == MigrationStateInProgress {
			// Check the migration job isn't wedged.
			if job != nil && job.LastActivityAt < model.GetMillis()-MigrationJobWedgedTimeoutMilliseconds && job.CreateAt < model.GetMillis()-MigrationJobWedgedTimeoutMilliseconds {
				mlog.Warn("Job appears to be wedged. Rescheduling another instance.", mlog.String("scheduler", scheduler.Name()), mlog.String("wedged_job_id", job.ID), mlog.String("migration_key", key))
				if err := scheduler.srv.Jobs.SetJobError(job, nil); err != nil {
					mlog.Error("Worker: Failed to set job error", mlog.String("scheduler", scheduler.Name()), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
				}
				return scheduler.createJob(key, job)
			}

			return nil, nil
		}

		if state == MigrationStateCompleted {
			// This migration is done. Continue to check the next.
			continue
		}

		if state == MigrationStateUnscheduled {
			mlog.Debug("Scheduling a new job for migration.", mlog.String("scheduler", scheduler.Name()), mlog.String("migration_key", key))
			return scheduler.createJob(key, job)
		}

		mlog.Error("Unknown migration state. Not doing anything.", mlog.String("migration_state", state))
		return nil, nil
	}

	// If we reached here, then there aren't any migrations left to run.
	scheduler.allMigrationsCompleted = true
	mlog.Debug("All migrations are complete.", mlog.String("scheduler", scheduler.Name()))

	return nil, nil
}

func (scheduler *Scheduler) createJob(migrationKey string, lastJob *model.Job) (*model.Job, *model.AppError) {
	var lastDone string
	if lastJob != nil {
		lastDone = lastJob.Data[JobDataKeyMigrationLastDone]
	}

	data := map[string]string{
		JobDataKeyMigration:         migrationKey,
		JobDataKeyMigrationLastDone: lastDone,
	}

	job, err := scheduler.srv.Jobs.CreateJob(model.JobTypeMigrations, data)
	if err != nil {
		return nil, err
	}
	return job, nil
}
