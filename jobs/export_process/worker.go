// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package export_process

import (
	"io"
	"path/filepath"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/jobs"
	tjobs "github.com/mattermost/mattermost-server/v5/jobs/interfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func init() {
	app.RegisterJobsExportProcessInterface(func(s *app.Server) tjobs.ExportProcessInterface {
		a := app.New(app.ServerConnector(s))
		return &ExportProcessInterfaceImpl{a}
	})
}

type ExportProcessInterfaceImpl struct {
	app *app.App
}

type ExportProcessWorker struct {
	name        string
	stopChan    chan struct{}
	stoppedChan chan struct{}
	jobsChan    chan model.Job
	jobServer   *jobs.JobServer
	app         *app.App
}

func (i *ExportProcessInterfaceImpl) MakeWorker() model.Worker {
	return &ExportProcessWorker{
		name:        "ExportProcess",
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		jobsChan:    make(chan model.Job),
		jobServer:   i.app.Srv().Jobs,
		app:         i.app,
	}
}

func (w *ExportProcessWorker) JobChannel() chan<- model.Job {
	return w.jobsChan
}

func (w *ExportProcessWorker) Run() {
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

func (w *ExportProcessWorker) Stop() {
	mlog.Debug("Worker stopping", mlog.String("worker", w.name))
	close(w.stopChan)
	<-w.stoppedChan
}

func (w *ExportProcessWorker) doJob(job *model.Job) {
	if claimed, err := w.jobServer.ClaimJob(job); err != nil {
		mlog.Warn("Worker experienced an error while trying to claim job",
			mlog.String("worker", w.name),
			mlog.String("job_id", job.ID),
			mlog.String("error", err.Error()))
		return
	} else if !claimed {
		return
	}

	opts := app.BulkExportOpts{
		CreateArchive: true,
	}

	includeAttachments, ok := job.Data["include_attachments"]
	if ok && includeAttachments == "true" {
		opts.IncludeAttachments = true
	}

	outPath := *w.app.Config().ExportSettings.Directory
	exportFilename := model.NewID() + "_export.zip"

	rd, wr := io.Pipe()

	errCh := make(chan *model.AppError, 1)
	go func() {
		defer close(errCh)
		_, appErr := w.app.WriteFile(rd, filepath.Join(outPath, exportFilename))
		errCh <- appErr
	}()

	appErr := w.app.BulkExport(wr, outPath, opts)
	if err := wr.Close(); err != nil {
		mlog.Warn("Worker: error closing writer")
	}
	if appErr != nil {
		w.setJobError(job, appErr)
		return
	}

	if appErr := <-errCh; appErr != nil {
		w.setJobError(job, appErr)
		return
	}

	mlog.Info("Worker: Job is complete", mlog.String("worker", w.name), mlog.String("job_id", job.ID))
	w.setJobSuccess(job)
}

func (w *ExportProcessWorker) setJobSuccess(job *model.Job) {
	if err := w.app.Srv().Jobs.SetJobSuccess(job); err != nil {
		mlog.Error("Worker: Failed to set success for job", mlog.String("worker", w.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
		w.setJobError(job, err)
	}
}

func (w *ExportProcessWorker) setJobError(job *model.Job, appError *model.AppError) {
	if err := w.app.Srv().Jobs.SetJobError(job, appError); err != nil {
		mlog.Error("Worker: Failed to set job error", mlog.String("worker", w.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
	}
}
