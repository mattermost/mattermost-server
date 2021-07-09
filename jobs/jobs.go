// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package jobs

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	CancelWatcherPollingInterval = 5000
)

func (srv *JobServer) CreateJob(jobType string, jobData map[string]string) (*model.Job, *model.AppError) {
	job := model.Job{
		ID:       model.NewID(),
		Type:     jobType,
		CreateAt: model.GetMillis(),
		Status:   model.JobStatusPending,
		Data:     jobData,
	}

	if err := job.IsValid(); err != nil {
		return nil, err
	}

	if _, err := srv.Store.Job().Save(&job); err != nil {
		return nil, model.NewAppError("CreateJob", "app.job.save.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return &job, nil
}

func (srv *JobServer) GetJob(id string) (*model.Job, *model.AppError) {
	job, err := srv.Store.Job().Get(id)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetJob", "app.job.get.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetJob", "app.job.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return job, nil
}

func (srv *JobServer) ClaimJob(job *model.Job) (bool, *model.AppError) {
	updated, err := srv.Store.Job().UpdateStatusOptimistically(job.ID, model.JobStatusPending, model.JobStatusInProgress)
	if err != nil {
		return false, model.NewAppError("ClaimJob", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if updated && srv.metrics != nil {
		srv.metrics.IncrementJobActive(job.Type)
	}

	return updated, nil
}

func (srv *JobServer) SetJobProgress(job *model.Job, progress int64) *model.AppError {
	job.Status = model.JobStatusInProgress
	job.Progress = progress

	if _, err := srv.Store.Job().UpdateOptimistically(job, model.JobStatusInProgress); err != nil {
		return model.NewAppError("SetJobProgress", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (srv *JobServer) SetJobWarning(job *model.Job) *model.AppError {
	if _, err := srv.Store.Job().UpdateStatus(job.ID, model.JobStatusWarning); err != nil {
		return model.NewAppError("SetJobWarning", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (srv *JobServer) SetJobSuccess(job *model.Job) *model.AppError {
	if _, err := srv.Store.Job().UpdateStatus(job.ID, model.JobStatusSuccess); err != nil {
		return model.NewAppError("SetJobSuccess", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if srv.metrics != nil {
		srv.metrics.DecrementJobActive(job.Type)
	}

	return nil
}

func (srv *JobServer) SetJobError(job *model.Job, jobError *model.AppError) *model.AppError {
	if jobError == nil {
		_, err := srv.Store.Job().UpdateStatus(job.ID, model.JobStatusError)
		if err != nil {
			return model.NewAppError("SetJobError", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
		}

		if srv.metrics != nil {
			srv.metrics.DecrementJobActive(job.Type)
		}

		return nil
	}

	job.Status = model.JobStatusError
	job.Progress = -1
	if job.Data == nil {
		job.Data = make(map[string]string)
	}
	job.Data["error"] = jobError.Message
	if jobError.DetailedError != "" {
		job.Data["error"] += " — " + jobError.DetailedError
	}
	updated, err := srv.Store.Job().UpdateOptimistically(job, model.JobStatusInProgress)
	if err != nil {
		return model.NewAppError("SetJobError", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	if updated && srv.metrics != nil {
		srv.metrics.DecrementJobActive(job.Type)
	}

	if !updated {
		updated, err = srv.Store.Job().UpdateOptimistically(job, model.JobStatusCancelRequested)
		if err != nil {
			return model.NewAppError("SetJobError", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
		if !updated {
			return model.NewAppError("SetJobError", "jobs.set_job_error.update.error", nil, "id="+job.ID, http.StatusInternalServerError)
		}
	}

	return nil
}

func (srv *JobServer) SetJobCanceled(job *model.Job) *model.AppError {
	if _, err := srv.Store.Job().UpdateStatus(job.ID, model.JobStatusCanceled); err != nil {
		return model.NewAppError("SetJobCanceled", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if srv.metrics != nil {
		srv.metrics.DecrementJobActive(job.Type)
	}

	return nil
}

func (srv *JobServer) UpdateInProgressJobData(job *model.Job) *model.AppError {
	job.Status = model.JobStatusInProgress
	job.LastActivityAt = model.GetMillis()
	if _, err := srv.Store.Job().UpdateOptimistically(job, model.JobStatusInProgress); err != nil {
		return model.NewAppError("UpdateInProgressJobData", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (srv *JobServer) RequestCancellation(jobID string) *model.AppError {
	updated, err := srv.Store.Job().UpdateStatusOptimistically(jobID, model.JobStatusPending, model.JobStatusCanceled)
	if err != nil {
		return model.NewAppError("RequestCancellation", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	if updated {
		if srv.metrics != nil {
			job, err := srv.GetJob(jobID)
			if err != nil {
				return model.NewAppError("RequestCancellation", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
			}

			srv.metrics.DecrementJobActive(job.Type)
		}

		return nil
	}

	updated, err = srv.Store.Job().UpdateStatusOptimistically(jobID, model.JobStatusInProgress, model.JobStatusCancelRequested)
	if err != nil {
		return model.NewAppError("RequestCancellation", "app.job.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if updated {
		return nil
	}

	return model.NewAppError("RequestCancellation", "jobs.request_cancellation.status.error", nil, "id="+jobID, http.StatusInternalServerError)
}

func (srv *JobServer) CancellationWatcher(ctx context.Context, jobID string, cancelChan chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			mlog.Debug("CancellationWatcher for Job Aborting as job has finished.", mlog.String("job_id", jobID))
			return
		case <-time.After(CancelWatcherPollingInterval * time.Millisecond):
			mlog.Debug("CancellationWatcher for Job started polling.", mlog.String("job_id", jobID))
			if jobStatus, err := srv.Store.Job().Get(jobID); err == nil {
				if jobStatus.Status == model.JobStatusCancelRequested {
					close(cancelChan)
					return
				}
			}
		}
	}
}

func GenerateNextStartDateTime(now time.Time, nextStartTime time.Time) *time.Time {
	nextTime := time.Date(now.Year(), now.Month(), now.Day(), nextStartTime.Hour(), nextStartTime.Minute(), 0, 0, time.Local)

	if !now.Before(nextTime) {
		nextTime = nextTime.AddDate(0, 0, 1)
	}

	return &nextTime
}

func (srv *JobServer) CheckForPendingJobsByType(jobType string) (bool, *model.AppError) {
	count, err := srv.Store.Job().GetCountByStatusAndType(model.JobStatusPending, jobType)
	if err != nil {
		return false, model.NewAppError("CheckForPendingJobsByType", "app.job.get_count_by_status_and_type.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return count > 0, nil
}

func (srv *JobServer) GetLastSuccessfulJobByType(jobType string) (*model.Job, *model.AppError) {
	statuses := []string{model.JobStatusSuccess}
	if jobType == model.JobTypeMessageExport {
		statuses = []string{model.JobStatusWarning, model.JobStatusSuccess}
	}
	job, err := srv.Store.Job().GetNewestJobByStatusesAndType(statuses, jobType)
	var nfErr *store.ErrNotFound
	if err != nil && !errors.As(err, &nfErr) {
		return nil, model.NewAppError("GetLastSuccessfulJobByType", "app.job.get_newest_job_by_status_and_type.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return job, nil
}
