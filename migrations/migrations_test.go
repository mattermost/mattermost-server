// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetMigrationState(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	th := Setup()
	defer th.TearDown()

	migrationKey := model.NewID()

	th.DeleteAllJobsByTypeAndMigrationKey(model.JobTypeMigrations, migrationKey)

	// Test with no job yet.
	state, job, err := GetMigrationState(migrationKey, th.App.Srv().Store)
	assert.Nil(t, err)
	assert.Nil(t, job)
	assert.Equal(t, "unscheduled", state)

	// Test with the system table showing the migration as done.
	system := model.System{
		Name:  migrationKey,
		Value: "true",
	}
	nErr := th.App.Srv().Store.System().Save(&system)
	assert.NoError(t, nErr)

	state, job, err = GetMigrationState(migrationKey, th.App.Srv().Store)
	assert.Nil(t, err)
	assert.Nil(t, job)
	assert.Equal(t, "completed", state)

	_, nErr = th.App.Srv().Store.System().PermanentDeleteByName(migrationKey)
	assert.NoError(t, nErr)

	// Test with a job scheduled in "pending" state.
	j1 := &model.Job{
		ID:       model.NewID(),
		CreateAt: model.GetMillis(),
		Data: map[string]string{
			JobDataKeyMigration: migrationKey,
		},
		Status: model.JobStatusPending,
		Type:   model.JobTypeMigrations,
	}

	j1, nErr = th.App.Srv().Store.Job().Save(j1)
	require.NoError(t, nErr)

	state, job, err = GetMigrationState(migrationKey, th.App.Srv().Store)
	assert.Nil(t, err)
	assert.Equal(t, j1.ID, job.ID)
	assert.Equal(t, "in_progress", state)

	// Test with a job scheduled in "in progress" state.
	j2 := &model.Job{
		ID:       model.NewID(),
		CreateAt: j1.CreateAt + 1,
		Data: map[string]string{
			JobDataKeyMigration: migrationKey,
		},
		Status: model.JobStatusInProgress,
		Type:   model.JobTypeMigrations,
	}

	j2, nErr = th.App.Srv().Store.Job().Save(j2)
	require.NoError(t, nErr)

	state, job, err = GetMigrationState(migrationKey, th.App.Srv().Store)
	assert.Nil(t, err)
	assert.Equal(t, j2.ID, job.ID)
	assert.Equal(t, "in_progress", state)

	// Test with a job scheduled in "error" state.
	j3 := &model.Job{
		ID:       model.NewID(),
		CreateAt: j2.CreateAt + 1,
		Data: map[string]string{
			JobDataKeyMigration: migrationKey,
		},
		Status: model.JobStatusError,
		Type:   model.JobTypeMigrations,
	}

	j3, nErr = th.App.Srv().Store.Job().Save(j3)
	require.NoError(t, nErr)

	state, job, err = GetMigrationState(migrationKey, th.App.Srv().Store)
	assert.Nil(t, err)
	assert.Equal(t, j3.ID, job.ID)
	assert.Equal(t, "unscheduled", state)
}
