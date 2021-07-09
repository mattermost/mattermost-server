// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestCreateJob(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	job := &model.Job{
		Type: model.JobTypeMessageExport,
		Data: map[string]string{
			"thing": "stuff",
		},
	}

	_, resp := th.SystemManagerClient.CreateJob(job)
	CheckForbiddenStatus(t, resp)

	received, resp := th.SystemAdminClient.CreateJob(job)
	require.Nil(t, resp.Error)

	defer th.App.Srv().Store.Job().Delete(received.ID)

	job = &model.Job{
		Type: model.NewID(),
	}

	_, resp = th.SystemAdminClient.CreateJob(job)
	CheckBadRequestStatus(t, resp)

	job.Type = model.JobTypeElasticsearchPostIndexing
	_, resp = th.Client.CreateJob(job)
	CheckForbiddenStatus(t, resp)
}

func TestGetJob(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	job := &model.Job{
		ID:     model.NewID(),
		Status: model.JobStatusPending,
		Type:   model.JobTypeMessageExport,
	}
	_, err := th.App.Srv().Store.Job().Save(job)
	require.NoError(t, err)

	defer th.App.Srv().Store.Job().Delete(job.ID)

	received, resp := th.SystemAdminClient.GetJob(job.ID)
	require.Nil(t, resp.Error)

	require.Equal(t, job.ID, received.ID, "incorrect job received")
	require.Equal(t, job.Status, received.Status, "incorrect job received")

	_, resp = th.SystemAdminClient.GetJob("1234")
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.GetJob(job.ID)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetJob(model.NewID())
	CheckNotFoundStatus(t, resp)
}

func TestGetJobs(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	jobType := model.JobTypeDataRetention

	t0 := model.GetMillis()
	jobs := []*model.Job{
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: t0 + 1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: t0,
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: t0 + 2,
		},
	}

	for _, job := range jobs {
		_, err := th.App.Srv().Store.Job().Save(job)
		require.NoError(t, err)
		defer th.App.Srv().Store.Job().Delete(job.ID)
	}

	received, resp := th.SystemAdminClient.GetJobs(0, 2)
	require.Nil(t, resp.Error)

	require.Len(t, received, 2, "received wrong number of jobs")
	require.Equal(t, jobs[2].ID, received[0].ID, "should've received newest job first")
	require.Equal(t, jobs[0].ID, received[1].ID, "should've received second newest job second")

	received, resp = th.SystemAdminClient.GetJobs(1, 2)
	require.Nil(t, resp.Error)

	require.Equal(t, jobs[1].ID, received[0].ID, "should've received oldest job last")

	_, resp = th.Client.GetJobs(0, 60)
	CheckForbiddenStatus(t, resp)
}

func TestGetJobsByType(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	jobType := model.JobTypeDataRetention

	jobs := []*model.Job{
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: 1000,
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: 999,
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: 1001,
		},
		{
			ID:       model.NewID(),
			Type:     model.NewID(),
			CreateAt: 1002,
		},
	}

	for _, job := range jobs {
		_, err := th.App.Srv().Store.Job().Save(job)
		require.NoError(t, err)
		defer th.App.Srv().Store.Job().Delete(job.ID)
	}

	received, resp := th.SystemAdminClient.GetJobsByType(jobType, 0, 2)
	require.Nil(t, resp.Error)

	require.Len(t, received, 2, "received wrong number of jobs")
	require.Equal(t, jobs[2].ID, received[0].ID, "should've received newest job first")
	require.Equal(t, jobs[0].ID, received[1].ID, "should've received second newest job second")

	received, resp = th.SystemAdminClient.GetJobsByType(jobType, 1, 2)
	require.Nil(t, resp.Error)

	require.Len(t, received, 1, "received wrong number of jobs")
	require.Equal(t, jobs[1].ID, received[0].ID, "should've received oldest job last")

	_, resp = th.SystemAdminClient.GetJobsByType("", 0, 60)
	CheckNotFoundStatus(t, resp)

	_, resp = th.SystemAdminClient.GetJobsByType(strings.Repeat("a", 33), 0, 60)
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.GetJobsByType(jobType, 0, 60)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemManagerClient.GetJobsByType(model.JobTypeElasticsearchPostIndexing, 0, 60)
	require.Nil(t, resp.Error)
}

func TestDownloadJob(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	jobName := model.NewID()
	job := &model.Job{
		ID:   jobName,
		Type: model.JobTypeMessageExport,
		Data: map[string]string{
			"export_type": "csv",
		},
		Status: model.JobStatusSuccess,
	}

	// DownloadExportResults is not set to true so we should get a not implemented error status
	_, resp := th.Client.DownloadJob(job.ID)
	CheckNotImplementedStatus(t, resp)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.MessageExportSettings.DownloadExportResults = true
	})

	// Normal user cannot download the results of these job (non-existent job)
	_, resp = th.Client.DownloadJob(job.ID)
	CheckNotFoundStatus(t, resp)

	// System admin trying to download the results of a non-existent job
	_, resp = th.SystemAdminClient.DownloadJob(job.ID)
	CheckNotFoundStatus(t, resp)

	// Here we have a job that exist in our database but the results do not exist therefore when we try to download the results
	// as a system admin, we should get a not found status.
	_, err := th.App.Srv().Store.Job().Save(job)
	require.NoError(t, err)
	defer th.App.Srv().Store.Job().Delete(job.ID)

	filePath := "./data/export/" + job.ID + "/testdat.txt"
	mkdirAllErr := os.MkdirAll(filepath.Dir(filePath), 0770)
	require.NoError(t, mkdirAllErr)
	os.Create(filePath)

	// Normal user cannot download the results of these job (not the right permission)
	_, resp = th.Client.DownloadJob(job.ID)
	CheckForbiddenStatus(t, resp)

	// System manager with default permissions cannot download the results of these job (Doesn't have correct permissions)
	_, resp = th.SystemManagerClient.DownloadJob(job.ID)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.DownloadJob(job.ID)
	CheckBadRequestStatus(t, resp)

	job.Data["is_downloadable"] = "true"
	updateStatus, err := th.App.Srv().Store.Job().UpdateOptimistically(job, model.JobStatusSuccess)
	require.True(t, updateStatus)
	require.NoError(t, err)

	_, resp = th.SystemAdminClient.DownloadJob(job.ID)
	CheckNotFoundStatus(t, resp)

	// Now we stub the results of the job into the same directory and try to download it again
	// This time we should successfully retrieve the results without any error
	filePath = "./data/export/" + job.ID + ".zip"
	mkdirAllErr = os.MkdirAll(filepath.Dir(filePath), 0770)
	require.NoError(t, mkdirAllErr)
	os.Create(filePath)

	_, resp = th.SystemAdminClient.DownloadJob(job.ID)
	require.Nil(t, resp.Error)

	// Here we are creating a new job which doesn't have type of message export
	jobName = model.NewID()
	job = &model.Job{
		ID:   jobName,
		Type: model.JobTypeCloud,
		Data: map[string]string{
			"export_type": "csv",
		},
		Status: model.JobStatusSuccess,
	}
	_, err = th.App.Srv().Store.Job().Save(job)
	require.NoError(t, err)
	defer th.App.Srv().Store.Job().Delete(job.ID)

	// System admin shouldn't be able to download since the job type is not message export
	_, resp = th.SystemAdminClient.DownloadJob(job.ID)
	CheckBadRequestStatus(t, resp)
}

func TestCancelJob(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	jobType := model.JobTypeMessageExport
	jobs := []*model.Job{
		{
			ID:     model.NewID(),
			Type:   jobType,
			Status: model.JobStatusPending,
		},
		{
			ID:     model.NewID(),
			Type:   jobType,
			Status: model.JobStatusInProgress,
		},
		{
			ID:     model.NewID(),
			Type:   jobType,
			Status: model.JobStatusSuccess,
		},
	}

	for _, job := range jobs {
		_, err := th.App.Srv().Store.Job().Save(job)
		require.NoError(t, err)
		defer th.App.Srv().Store.Job().Delete(job.ID)
	}

	_, resp := th.Client.CancelJob(jobs[0].ID)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.CancelJob(jobs[0].ID)
	require.Nil(t, resp.Error)

	_, resp = th.SystemAdminClient.CancelJob(jobs[1].ID)
	require.Nil(t, resp.Error)

	_, resp = th.SystemAdminClient.CancelJob(jobs[2].ID)
	CheckInternalErrorStatus(t, resp)

	_, resp = th.SystemAdminClient.CancelJob(model.NewID())
	CheckNotFoundStatus(t, resp)
}
