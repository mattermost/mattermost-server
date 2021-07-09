// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
package resend_invitation_email

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const TwentyFourHoursInMillis int64 = 86400000

type ResendInvitationEmailWorker struct {
	name    string
	stop    chan bool
	stopped chan bool
	jobs    chan model.Job
	App     *app.App
}

func (rse *ResendInvitationEmailJobInterfaceImpl) MakeWorker() model.Worker {
	worker := ResendInvitationEmailWorker{
		name:    ResendInvitationEmailJob,
		stop:    make(chan bool, 1),
		stopped: make(chan bool, 1),
		jobs:    make(chan model.Job),
		App:     rse.App,
	}
	return &worker
}

func (rseworker *ResendInvitationEmailWorker) Run() {
	mlog.Debug("Worker started", mlog.String("worker", rseworker.name))

	defer func() {
		mlog.Debug("Worker finished", mlog.String("worker", rseworker.name))
		rseworker.stopped <- true
	}()

	for {
		select {
		case <-rseworker.stop:
			mlog.Debug("Worker received stop signal", mlog.String("worker", rseworker.name))
			return
		case job := <-rseworker.jobs:
			mlog.Debug("Worker received a new candidate job.", mlog.String("worker", rseworker.name))
			rseworker.DoJob(&job)
		}
	}
}

func (rseworker *ResendInvitationEmailWorker) Stop() {
	mlog.Debug("Worker stopping", mlog.String("worker", rseworker.name))
	rseworker.stop <- true
	<-rseworker.stopped
}

func (rseworker *ResendInvitationEmailWorker) JobChannel() chan<- model.Job {
	return rseworker.jobs
}

func (rseworker *ResendInvitationEmailWorker) cleanEmailData(emailStringData string) ([]string, error) {
	// emailStringData looks like this ["user1@gmail.com","user2@gmail.com"]
	emails := []string{}
	err := json.Unmarshal([]byte(emailStringData), &emails)
	if err != nil {
		return nil, err
	}

	return emails, nil
}

func (rseworker *ResendInvitationEmailWorker) removeAlreadyJoined(teamID string, emailList []string) []string {
	var notJoinedYet []string
	for _, email := range emailList {
		// check if the user with this email is on the system already
		user, appErr := rseworker.App.GetUserByEmail(email)
		if appErr != nil {
			notJoinedYet = append(notJoinedYet, email)
			continue
		}
		// now we check if they are part of the team already
		userID := []string{user.ID}
		members, appErr := rseworker.App.GetTeamMembersByIDs(teamID, userID, nil)
		if len(members) == 0 || appErr != nil {
			notJoinedYet = append(notJoinedYet, email)
		}
	}

	return notJoinedYet
}

func (rseworker *ResendInvitationEmailWorker) DoJob(job *model.Job) {
	scheduledAt, _ := strconv.ParseInt(job.Data["scheduledAt"], 10, 64)
	now := model.GetMillis()

	elapsedTimeSinceSchedule := now - scheduledAt

	var DurationInMillis int64

	duration := os.Getenv("MM_RESEND_INVITATION_EMAIL_JOB_DURATION")

	DurationInMillis, parseError := strconv.ParseInt(duration, 10, 64)
	if parseError != nil {
		// default to 24 hours
		DurationInMillis = TwentyFourHoursInMillis
	}

	if elapsedTimeSinceSchedule > DurationInMillis {
		teamID := job.Data["teamID"]
		emailListData := job.Data["emailList"]

		emailList, err := rseworker.cleanEmailData(emailListData)
		if err != nil {
			appErr := model.NewAppError("worker: "+rseworker.name, "job_id: "+job.ID, nil, err.Error(), http.StatusInternalServerError)
			mlog.Error("Worker: Failed to clean emails string data", mlog.String("worker", rseworker.name), mlog.String("job_id", job.ID), mlog.String("error", appErr.Error()))
			rseworker.setJobError(job, appErr)
		}

		emailList = rseworker.removeAlreadyJoined(teamID, emailList)

		_, appErr := rseworker.App.InviteNewUsersToTeamGracefully(emailList, teamID, job.Data["senderID"])
		if appErr != nil {
			mlog.Error("Worker: Failed to send emails", mlog.String("worker", rseworker.name), mlog.String("job_id", job.ID), mlog.String("error", appErr.Error()))
			rseworker.setJobError(job, appErr)
		}
		rseworker.setJobSuccess(job)
	}

}

func (rseworker *ResendInvitationEmailWorker) setJobSuccess(job *model.Job) {
	if err := rseworker.App.Srv().Jobs.SetJobSuccess(job); err != nil {
		mlog.Error("Worker: Failed to set success for job", mlog.String("worker", rseworker.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
		rseworker.setJobError(job, err)
	}
}

func (rseworker *ResendInvitationEmailWorker) setJobError(job *model.Job, appError *model.AppError) {
	if err := rseworker.App.Srv().Jobs.SetJobError(job, appError); err != nil {
		mlog.Error("Worker: Failed to set job error", mlog.String("worker", rseworker.name), mlog.String("job_id", job.ID), mlog.String("error", err.Error()))
	}
}
