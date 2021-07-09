// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package export_delete

import (
	"path/filepath"
	"time"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/jobs"
	tjobs "github.com/mattermost/mattermost-server/v5/jobs/interfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func init() {
	app.RegisterJobsExportDeleteInterface(func(s *app.Server) tjobs.ExportDeleteInterface {
		a := app.New(app.ServerConnector(s))
		return &ExportDeleteInterfaceImpl{a}
	})
}

type ExportDeleteInterfaceImpl struct {
	app *app.App
}

type ExportDeleteWorker struct {
	name        string
	stopChan    chan struct{}
	stoppedChan chan struct{}
	jobsChan    chan model.Job
	jobServer   *jobs.JobServer
	app         *app.App
}

func (i *ExportDeleteInterfaceImpl) MakeWorker() model.Worker {
	return &ExportDeleteWorker{
		name:        "ExportDelete",
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		jobsChan:    make(chan model.Job),
		jobServer:   i.app.Srv().Jobs,
		app:         i.app,
	}
}

func (w *ExportDeleteWorker) JobChannel() chan<- model.Job {
	return w.jobsChan
}

func (w *ExportDeleteWorker) Run() {
	mlog.Debug("Worker started", mlog.String("worker", w.name))

	defer func() {
		mlog.Debug("Worker finished", mlog.String("worker", w.name))
		close(w.stoppedChan)
	}()

	for {
		select {
		case <-w.stopChan:
			mlog.Debug("Worker received stop signal", mlog.String("worker", w.name))
			return
		case job := <-w.jobsChan:
			mlog.Debug("Worker received a new candidate job.", mlog.String("worker", w.name))
			w.doJob(&job)
		}
	}
}

func (w *ExportDeleteWorker) Stop() {
	mlog.Debug("Worker stopping", mlog.String("worker", w.name))
	close(w.stopChan)
	<-w.stoppedChan
}

func (w *ExportDeleteWorker) doJob(job *model.Job) {
	if claimed, err := w.jobServer.ClaimJob(job); err != nil {
		mlog.Warn("Worker experienced an error while trying to claim job",
			mlog.String("worker", w.name),
			mlog.String("job_id", job.ID),
			mlog.String("error", err.Error()))
		return
	} else if !claimed {
		return
	}

	exportPath := *w.app.Config().ExportSettings.Directory
	retentionTime := time.Duration(*w.app.Config().ExportSettings.RetentionDays) * 24 * time.Hour
	exports, appErr := w.app.ListDirectory(exportPath)
	if appErr != nil {
		w.setJobError(job, appErr)
		return
	}

	var hasErrs bool
	for i := range exports {
		filename := filepath.Base(exports[i])
		modTime, appErr := w.app.FileModTime(filepath.Join(exportPath, filename))
		if appErr != nil {
			mlog.Debug("Worker: Failed to get file modification time",
				mlog.Err(appErr), mlog.String("export", exports[i]))
			hasErrs = true
			continue
		}

		if time.Now().After(modTime.Add(retentionTime)) {
			// remove file data from storage.
			if appErr := w.app.RemoveFile(exports[i]); appErr != nil {
				mlog.Debug("Worker: Failed to remove file",
					mlog.Err(appErr), mlog.String("export", exports[i]))
				hasErrs = true
				continue
			}
		}
	}

	if hasErrs {
		mlog.Warn("Worker: errors occurred")
	}

	mlog.Info("Worker: Job is complete", mlog.String("worker", w.name), mlog.String("job_id", job.ID))
	w.setJobSuccess(job)
}

func (w *ExportDeleteWorker) setJobSuccess(job *model.Job) {
	if err := w.app.Srv().Jobs.SetJobSuccess(job); err != nil {
		mlog.Error("Worker: Failed to set success for job", mlog.String("worker", w.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
		w.setJobError(job, err)
	}
}

func (w *ExportDeleteWorker) setJobError(job *model.Job, appError *model.AppError) {
	if err := w.app.Srv().Jobs.SetJobError(job, appError); err != nil {
		mlog.Error("Worker: Failed to set job error", mlog.String("worker", w.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
	}
}
