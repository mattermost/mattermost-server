// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package active_users

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/jobs"
	tjobs "github.com/mattermost/mattermost-server/v5/jobs/interfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	JobName = "ActiveUsers"
)

type Worker struct {
	name      string
	stop      chan bool
	stopped   chan bool
	jobs      chan model.Job
	jobServer *jobs.JobServer
	app       *app.App
}

func init() {
	app.RegisterJobsActiveUsersInterface(func(s *app.Server) tjobs.ActiveUsersJobInterface {
		a := app.New(app.ServerConnector(s))
		return &ActiveUsersJobInterfaceImpl{a}
	})
}

type ActiveUsersJobInterfaceImpl struct {
	App *app.App
}

func (m *ActiveUsersJobInterfaceImpl) MakeWorker() model.Worker {
	worker := Worker{
		name:      JobName,
		stop:      make(chan bool, 1),
		stopped:   make(chan bool, 1),
		jobs:      make(chan model.Job),
		jobServer: m.App.Srv().Jobs,
		app:       m.App,
	}
	return &worker
}

func (worker *Worker) Run() {
	mlog.Debug("Worker started", mlog.String("worker", worker.name))

	defer func() {
		mlog.Debug("Worker finished", mlog.String("worker", worker.name))
		worker.stopped <- true
	}()

	for {
		select {
		case <-worker.stop:
			mlog.Debug("Worker received stop signal", mlog.String("worker", worker.name))
			return
		case job := <-worker.jobs:
			mlog.Debug("Worker received a new candidate job.", mlog.String("worker", worker.name))
			worker.DoJob(&job)
		}
	}
}

func (worker *Worker) Stop() {
	mlog.Debug("Worker stopping", mlog.String("worker", worker.name))
	worker.stop <- true
	<-worker.stopped
}

func (worker *Worker) JobChannel() chan<- model.Job {
	return worker.jobs
}

func (worker *Worker) DoJob(job *model.Job) {
	if claimed, err := worker.jobServer.ClaimJob(job); err != nil {
		mlog.Warn("Worker experienced an error while trying to claim job",
			mlog.String("worker", worker.name),
			mlog.String("job_id", job.ID),
			mlog.String("error", err.Error()))
		return
	} else if !claimed {
		return
	}

	count, err := worker.app.Srv().Store.User().Count(model.UserCountOptions{IncludeDeleted: false})

	if err != nil {
		mlog.Error("Worker: Failed to get active user count", mlog.String("worker", worker.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
		worker.setJobError(job, model.NewAppError("DoJob", "app.user.get_total_users_count.app_error", nil, err.Error(), http.StatusInternalServerError))
		return
	}

	if worker.app.Metrics() != nil {
		worker.app.Metrics().ObserveEnabledUsers(count)
	}

	mlog.Info("Worker: Job is complete", mlog.String("worker", worker.name), mlog.String("job_id", job.ID))
	worker.setJobSuccess(job)
}

func (worker *Worker) setJobSuccess(job *model.Job) {
	if err := worker.app.Srv().Jobs.SetJobSuccess(job); err != nil {
		mlog.Error("Worker: Failed to set success for job", mlog.String("worker", worker.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
		worker.setJobError(job, err)
	}
}

func (worker *Worker) setJobError(job *model.Job, appError *model.AppError) {
	if err := worker.app.Srv().Jobs.SetJobError(job, appError); err != nil {
		mlog.Error("Worker: Failed to set job error", mlog.String("worker", worker.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
	}
}
