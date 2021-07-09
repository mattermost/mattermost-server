// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"errors"
	"testing"

	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestJobStore(t *testing.T, ss store.Store) {
	t.Run("JobSaveGet", func(t *testing.T) { testJobSaveGet(t, ss) })
	t.Run("JobGetAllByType", func(t *testing.T) { testJobGetAllByType(t, ss) })
	t.Run("JobGetAllByTypePage", func(t *testing.T) { testJobGetAllByTypePage(t, ss) })
	t.Run("JobGetAllByTypesPage", func(t *testing.T) { testJobGetAllByTypesPage(t, ss) })
	t.Run("JobGetAllPage", func(t *testing.T) { testJobGetAllPage(t, ss) })
	t.Run("JobGetAllByStatus", func(t *testing.T) { testJobGetAllByStatus(t, ss) })
	t.Run("GetNewestJobByStatusAndType", func(t *testing.T) { testJobStoreGetNewestJobByStatusAndType(t, ss) })
	t.Run("GetNewestJobByStatusesAndType", func(t *testing.T) { testJobStoreGetNewestJobByStatusesAndType(t, ss) })
	t.Run("GetCountByStatusAndType", func(t *testing.T) { testJobStoreGetCountByStatusAndType(t, ss) })
	t.Run("JobUpdateOptimistically", func(t *testing.T) { testJobUpdateOptimistically(t, ss) })
	t.Run("JobUpdateStatusUpdateStatusOptimistically", func(t *testing.T) { testJobUpdateStatusUpdateStatusOptimistically(t, ss) })
	t.Run("JobDelete", func(t *testing.T) { testJobDelete(t, ss) })
}

func testJobSaveGet(t *testing.T, ss store.Store) {
	job := &model.Job{
		ID:     model.NewID(),
		Type:   model.NewID(),
		Status: model.NewID(),
		Data: map[string]string{
			"Processed":     "0",
			"Total":         "12345",
			"LastProcessed": "abcd",
		},
	}

	_, err := ss.Job().Save(job)
	require.NoError(t, err)

	defer ss.Job().Delete(job.ID)

	received, err := ss.Job().Get(job.ID)
	require.NoError(t, err)
	require.Equal(t, job.ID, received.ID, "received incorrect job after save")
	require.Equal(t, "12345", received.Data["Total"])
}

func testJobGetAllByType(t *testing.T, ss store.Store) {
	jobType := model.NewID()

	jobs := []*model.Job{
		{
			ID:   model.NewID(),
			Type: jobType,
		},
		{
			ID:   model.NewID(),
			Type: jobType,
		},
		{
			ID:   model.NewID(),
			Type: model.NewID(),
		},
	}

	for _, job := range jobs {
		_, err := ss.Job().Save(job)
		require.NoError(t, err)
		defer ss.Job().Delete(job.ID)
	}

	received, err := ss.Job().GetAllByType(jobType)
	require.NoError(t, err)
	require.Len(t, received, 2)
	require.ElementsMatch(t, []string{jobs[0].ID, jobs[1].ID}, []string{received[0].ID, received[1].ID})
}

func testJobGetAllByTypePage(t *testing.T, ss store.Store) {
	jobType := model.NewID()

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
		_, err := ss.Job().Save(job)
		require.NoError(t, err)
		defer ss.Job().Delete(job.ID)
	}

	received, err := ss.Job().GetAllByTypePage(jobType, 0, 2)
	require.NoError(t, err)
	require.Len(t, received, 2)
	require.Equal(t, received[0].ID, jobs[2].ID, "should've received newest job first")
	require.Equal(t, received[1].ID, jobs[0].ID, "should've received second newest job second")

	received, err = ss.Job().GetAllByTypePage(jobType, 2, 2)
	require.NoError(t, err)
	require.Len(t, received, 1)
	require.Equal(t, received[0].ID, jobs[1].ID, "should've received oldest job last")
}

func testJobGetAllByTypesPage(t *testing.T, ss store.Store) {
	jobType := model.NewID()
	jobType2 := model.NewID()

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
			Type:     jobType2,
			CreateAt: 1001,
		},
		{
			ID:       model.NewID(),
			Type:     model.NewID(),
			CreateAt: 1002,
		},
	}

	for _, job := range jobs {
		_, err := ss.Job().Save(job)
		require.NoError(t, err)
		defer ss.Job().Delete(job.ID)
	}

	// test return all
	jobTypes := []string{jobType, jobType2}
	received, err := ss.Job().GetAllByTypesPage(jobTypes, 0, 4)
	require.NoError(t, err)
	require.Len(t, received, 3)
	require.Equal(t, received[0].ID, jobs[2].ID, "should've received newest job first")
	require.Equal(t, received[1].ID, jobs[0].ID, "should've received second newest job second")

	// test paging
	jobTypes = []string{jobType, jobType2}
	received, err = ss.Job().GetAllByTypesPage(jobTypes, 0, 2)
	require.NoError(t, err)
	require.Len(t, received, 2)
	require.Equal(t, received[0].ID, jobs[2].ID, "should've received newest job first")
	require.Equal(t, received[1].ID, jobs[0].ID, "should've received second newest job second")

	received, err = ss.Job().GetAllByTypesPage(jobTypes, 2, 2)
	require.NoError(t, err)
	require.Len(t, received, 1)
	require.Equal(t, received[0].ID, jobs[1].ID, "should've received oldest job last")
}

func testJobGetAllPage(t *testing.T, ss store.Store) {
	jobType := model.NewID()
	createAtTime := model.GetMillis()

	jobs := []*model.Job{
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: createAtTime + 1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: createAtTime,
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: createAtTime + 2,
		},
	}

	for _, job := range jobs {
		_, err := ss.Job().Save(job)
		require.NoError(t, err)
		defer ss.Job().Delete(job.ID)
	}

	received, err := ss.Job().GetAllPage(0, 2)
	require.NoError(t, err)
	require.Len(t, received, 2)
	require.Equal(t, received[0].ID, jobs[2].ID, "should've received newest job first")
	require.Equal(t, received[1].ID, jobs[0].ID, "should've received second newest job second")

	received, err = ss.Job().GetAllPage(2, 2)
	require.NoError(t, err)
	require.NotEmpty(t, received)
	require.Equal(t, received[0].ID, jobs[1].ID, "should've received oldest job last")
}

func testJobGetAllByStatus(t *testing.T, ss store.Store) {
	jobType := model.NewID()
	status := model.NewID()

	jobs := []*model.Job{
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: 1000,
			Status:   status,
			Data: map[string]string{
				"test": "data",
			},
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: 999,
			Status:   status,
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: 1001,
			Status:   status,
		},
		{
			ID:       model.NewID(),
			Type:     jobType,
			CreateAt: 1002,
			Status:   model.NewID(),
		},
	}

	for _, job := range jobs {
		_, err := ss.Job().Save(job)
		require.NoError(t, err)
		defer ss.Job().Delete(job.ID)
	}

	received, err := ss.Job().GetAllByStatus(status)
	require.NoError(t, err)
	require.Len(t, received, 3)
	require.Equal(t, received[0].ID, jobs[1].ID)
	require.Equal(t, received[1].ID, jobs[0].ID)
	require.Equal(t, received[2].ID, jobs[2].ID)
	require.Equal(t, "data", received[1].Data["test"], "should've received job data field back as saved")
}

func testJobStoreGetNewestJobByStatusAndType(t *testing.T, ss store.Store) {
	jobType1 := model.NewID()
	jobType2 := model.NewID()
	status1 := model.NewID()
	status2 := model.NewID()

	jobs := []*model.Job{
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 1001,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 1000,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType2,
			CreateAt: 1003,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 1004,
			Status:   status2,
		},
	}

	for _, job := range jobs {
		_, err := ss.Job().Save(job)
		require.NoError(t, err)
		defer ss.Job().Delete(job.ID)
	}

	received, err := ss.Job().GetNewestJobByStatusAndType(status1, jobType1)
	assert.NoError(t, err)
	assert.EqualValues(t, jobs[0].ID, received.ID)

	received, err = ss.Job().GetNewestJobByStatusAndType(model.NewID(), model.NewID())
	assert.Error(t, err)
	var nfErr *store.ErrNotFound
	assert.True(t, errors.As(err, &nfErr))
	assert.Nil(t, received)
}

func testJobStoreGetNewestJobByStatusesAndType(t *testing.T, ss store.Store) {
	jobType1 := model.NewID()
	jobType2 := model.NewID()
	status1 := model.NewID()
	status2 := model.NewID()

	jobs := []*model.Job{
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 1001,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 1000,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType2,
			CreateAt: 1003,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 1004,
			Status:   status2,
		},
	}

	for _, job := range jobs {
		_, err := ss.Job().Save(job)
		require.NoError(t, err)
		defer ss.Job().Delete(job.ID)
	}

	received, err := ss.Job().GetNewestJobByStatusesAndType([]string{status1, status2}, jobType1)
	assert.NoError(t, err)
	assert.EqualValues(t, jobs[3].ID, received.ID)

	received, err = ss.Job().GetNewestJobByStatusesAndType([]string{model.NewID(), model.NewID()}, model.NewID())
	assert.Error(t, err)
	var nfErr *store.ErrNotFound
	assert.True(t, errors.As(err, &nfErr))
	assert.Nil(t, received)

	received, err = ss.Job().GetNewestJobByStatusesAndType([]string{status2}, jobType2)
	assert.Error(t, err)
	assert.True(t, errors.As(err, &nfErr))
	assert.Nil(t, received)

	received, err = ss.Job().GetNewestJobByStatusesAndType([]string{status1}, jobType2)
	assert.NoError(t, err)
	assert.EqualValues(t, jobs[2].ID, received.ID)

	received, err = ss.Job().GetNewestJobByStatusesAndType([]string{}, jobType1)
	assert.Error(t, err)
	assert.True(t, errors.As(err, &nfErr))
	assert.Nil(t, received)
}

func testJobStoreGetCountByStatusAndType(t *testing.T, ss store.Store) {
	jobType1 := model.NewID()
	jobType2 := model.NewID()
	status1 := model.NewID()
	status2 := model.NewID()

	jobs := []*model.Job{
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 1000,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 999,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType2,
			CreateAt: 1001,
			Status:   status1,
		},
		{
			ID:       model.NewID(),
			Type:     jobType1,
			CreateAt: 1002,
			Status:   status2,
		},
	}

	for _, job := range jobs {
		_, err := ss.Job().Save(job)
		require.NoError(t, err)
		defer ss.Job().Delete(job.ID)
	}

	count, err := ss.Job().GetCountByStatusAndType(status1, jobType1)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)

	count, err = ss.Job().GetCountByStatusAndType(status2, jobType2)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)

	count, err = ss.Job().GetCountByStatusAndType(status1, jobType2)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)

	count, err = ss.Job().GetCountByStatusAndType(status2, jobType1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func testJobUpdateOptimistically(t *testing.T, ss store.Store) {
	job := &model.Job{
		ID:       model.NewID(),
		Type:     model.JobTypeDataRetention,
		CreateAt: model.GetMillis(),
		Status:   model.JobStatusPending,
	}

	_, err := ss.Job().Save(job)
	require.NoError(t, err)
	defer ss.Job().Delete(job.ID)

	job.LastActivityAt = model.GetMillis()
	job.Status = model.JobStatusInProgress
	job.Progress = 50
	job.Data = map[string]string{
		"Foo": "Bar",
	}

	updated, err := ss.Job().UpdateOptimistically(job, model.JobStatusSuccess)
	require.False(t, err != nil && updated)

	time.Sleep(2 * time.Millisecond)

	updated, err = ss.Job().UpdateOptimistically(job, model.JobStatusPending)
	require.NoError(t, err)
	require.True(t, updated)

	updatedJob, err := ss.Job().Get(job.ID)
	require.NoError(t, err)

	require.Equal(t, updatedJob.Type, job.Type)
	require.Equal(t, updatedJob.CreateAt, job.CreateAt)
	require.Equal(t, updatedJob.Status, job.Status)
	require.Greater(t, updatedJob.LastActivityAt, job.LastActivityAt)
	require.Equal(t, updatedJob.Progress, job.Progress)
	require.Equal(t, updatedJob.Data["Foo"], job.Data["Foo"])
}

func testJobUpdateStatusUpdateStatusOptimistically(t *testing.T, ss store.Store) {
	job := &model.Job{
		ID:       model.NewID(),
		Type:     model.JobTypeDataRetention,
		CreateAt: model.GetMillis(),
		Status:   model.JobStatusSuccess,
	}

	var lastUpdateAt int64
	received, err := ss.Job().Save(job)
	require.NoError(t, err)
	lastUpdateAt = received.LastActivityAt

	defer ss.Job().Delete(job.ID)

	time.Sleep(2 * time.Millisecond)

	received, err = ss.Job().UpdateStatus(job.ID, model.JobStatusPending)
	require.NoError(t, err)

	require.Equal(t, model.JobStatusPending, received.Status)
	require.Greater(t, received.LastActivityAt, lastUpdateAt)
	lastUpdateAt = received.LastActivityAt

	time.Sleep(2 * time.Millisecond)

	updated, err := ss.Job().UpdateStatusOptimistically(job.ID, model.JobStatusInProgress, model.JobStatusSuccess)
	require.NoError(t, err)
	require.False(t, updated)

	received, err = ss.Job().Get(job.ID)
	require.NoError(t, err)

	require.Equal(t, model.JobStatusPending, received.Status)
	require.Equal(t, received.LastActivityAt, lastUpdateAt)

	time.Sleep(2 * time.Millisecond)

	updated, err = ss.Job().UpdateStatusOptimistically(job.ID, model.JobStatusPending, model.JobStatusInProgress)
	require.NoError(t, err)
	require.True(t, updated, "should have succeeded")

	var startAtSet int64
	received, err = ss.Job().Get(job.ID)
	require.NoError(t, err)
	require.Equal(t, model.JobStatusInProgress, received.Status)
	require.NotEqual(t, 0, received.StartAt)
	require.Greater(t, received.LastActivityAt, lastUpdateAt)
	lastUpdateAt = received.LastActivityAt
	startAtSet = received.StartAt

	time.Sleep(2 * time.Millisecond)

	updated, err = ss.Job().UpdateStatusOptimistically(job.ID, model.JobStatusInProgress, model.JobStatusSuccess)
	require.NoError(t, err)
	require.True(t, updated, "should have succeeded")

	received, err = ss.Job().Get(job.ID)
	require.NoError(t, err)
	require.Equal(t, model.JobStatusSuccess, received.Status)
	require.Equal(t, startAtSet, received.StartAt)
	require.Greater(t, received.LastActivityAt, lastUpdateAt)
}

func testJobDelete(t *testing.T, ss store.Store) {
	job, err := ss.Job().Save(&model.Job{ID: model.NewID()})
	require.NoError(t, err)

	_, err = ss.Job().Delete(job.ID)
	assert.NoError(t, err)
}
