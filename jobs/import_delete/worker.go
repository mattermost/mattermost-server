// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package import_delete

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/jobs"
	tjobs "github.com/mattermost/mattermost-server/v5/jobs/interfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

func init() {
	app.RegisterJobsImportDeleteInterface(func(s *app.Server) tjobs.ImportDeleteInterface {
		a := app.New(app.ServerConnector(s))
		return &ImportDeleteInterfaceImpl{a}
	})
}

type ImportDeleteInterfaceImpl struct {
	app *app.App
}

type ImportDeleteWorker struct {
	name        string
	stopChan    chan struct{}
	stoppedChan chan struct{}
	jobsChan    chan model.Job
	jobServer   *jobs.JobServer
	app         *app.App
}

func (i *ImportDeleteInterfaceImpl) MakeWorker() model.Worker {
	return &ImportDeleteWorker{
		name:        "ImportDelete",
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
		jobsChan:    make(chan model.Job),
		jobServer:   i.app.Srv().Jobs,
		app:         i.app,
	}
}

func (w *ImportDeleteWorker) JobChannel() chan<- model.Job {
	return w.jobsChan
}

func (w *ImportDeleteWorker) Run() {
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

func (w *ImportDeleteWorker) Stop() {
	mlog.Debug("Worker stopping", mlog.String("worker", w.name))
	close(w.stopChan)
	<-w.stoppedChan
}

func (w *ImportDeleteWorker) doJob(job *model.Job) {
	if claimed, err := w.jobServer.ClaimJob(job); err != nil {
		mlog.Warn("Worker experienced an error while trying to claim job",
			mlog.String("worker", w.name),
			mlog.String("job_id", job.ID),
			mlog.String("error", err.Error()))
		return
	} else if !claimed {
		return
	}

	importPath := *w.app.Config().ImportSettings.Directory
	retentionTime := time.Duration(*w.app.Config().ImportSettings.RetentionDays) * 24 * time.Hour
	imports, appErr := w.app.ListDirectory(importPath)
	if appErr != nil {
		w.setJobError(job, appErr)
		return
	}

	var hasErrs bool
	for i := range imports {
		filename := filepath.Base(imports[i])
		modTime, appErr := w.app.FileModTime(filepath.Join(importPath, filename))
		if appErr != nil {
			mlog.Debug("Worker: Failed to get file modification time",
				mlog.Err(appErr), mlog.String("import", imports[i]))
			hasErrs = true
			continue
		}

		if time.Now().After(modTime.Add(retentionTime)) {
			// expected format if uploaded through the API is
			// ${uploadID}_${filename}${app.IncompleteUploadSuffix}
			minLen := 26 + 1 + len(app.IncompleteUploadSuffix)

			// check if it's an incomplete upload and attempt to delete its session.
			if len(filename) > minLen && filepath.Ext(filename) == app.IncompleteUploadSuffix {
				uploadID := filename[:26]
				if storeErr := w.app.Srv().Store.UploadSession().Delete(uploadID); storeErr != nil {
					mlog.Debug("Worker: Failed to delete UploadSession",
						mlog.Err(storeErr), mlog.String("upload_id", uploadID))
					hasErrs = true
					continue
				}
			} else {
				// check if fileinfo exists and if so delete it.
				filePath := filepath.Join(imports[i])
				info, storeErr := w.app.Srv().Store.FileInfo().GetByPath(filePath)
				var nfErr *store.ErrNotFound
				if storeErr != nil && !errors.As(storeErr, &nfErr) {
					mlog.Debug("Worker: Failed to get FileInfo",
						mlog.Err(storeErr), mlog.String("path", filePath))
					hasErrs = true
					continue
				} else if storeErr == nil {
					if storeErr = w.app.Srv().Store.FileInfo().PermanentDelete(info.ID); storeErr != nil {
						mlog.Debug("Worker: Failed to delete FileInfo",
							mlog.Err(storeErr), mlog.String("file_id", info.ID))
						hasErrs = true
						continue
					}
				}
			}

			// remove file data from storage.
			if appErr := w.app.RemoveFile(imports[i]); appErr != nil {
				mlog.Debug("Worker: Failed to remove file",
					mlog.Err(appErr), mlog.String("import", imports[i]))
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

func (w *ImportDeleteWorker) setJobSuccess(job *model.Job) {
	if err := w.app.Srv().Jobs.SetJobSuccess(job); err != nil {
		mlog.Error("Worker: Failed to set success for job", mlog.String("worker", w.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
		w.setJobError(job, err)
	}
}

func (w *ImportDeleteWorker) setJobError(job *model.Job, appError *model.AppError) {
	if err := w.app.Srv().Jobs.SetJobError(job, appError); err != nil {
		mlog.Error("Worker: Failed to set job error", mlog.String("worker", w.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
	}
}
