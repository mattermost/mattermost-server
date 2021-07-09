// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestSystemStore(t *testing.T, ss store.Store) {
	t.Run("", func(t *testing.T) { testSystemStore(t, ss) })
	t.Run("SaveOrUpdate", func(t *testing.T) { testSystemStoreSaveOrUpdate(t, ss) })
	t.Run("PermanentDeleteByName", func(t *testing.T) { testSystemStorePermanentDeleteByName(t, ss) })
	t.Run("InsertIfExists", func(t *testing.T) {
		testInsertIfExists(t, ss)
	})
	t.Run("SaveOrUpdateWithWarnMetricHandling", func(t *testing.T) { testSystemStoreSaveOrUpdateWithWarnMetricHandling(t, ss) })
	t.Run("GetByNameNoEntries", func(t *testing.T) { testSystemStoreGetByNameNoEntries(t, ss) })
}

func testSystemStore(t *testing.T, ss store.Store) {
	system := &model.System{Name: model.NewID(), Value: "value"}
	err := ss.System().Save(system)
	require.NoError(t, err)

	systems, _ := ss.System().Get()

	require.Equal(t, system.Value, systems[system.Name])

	system.Value = "value2"
	err = ss.System().Update(system)
	require.NoError(t, err)

	systems2, _ := ss.System().Get()
	require.Equal(t, system.Value, systems2[system.Name])

	rsystem, _ := ss.System().GetByName(system.Name)
	require.Equal(t, system.Value, rsystem.Value)
}

func testSystemStoreSaveOrUpdate(t *testing.T, ss store.Store) {
	system := &model.System{Name: model.NewID(), Value: "value"}

	err := ss.System().SaveOrUpdate(system)
	require.NoError(t, err)

	system.Value = "value2"

	err = ss.System().SaveOrUpdate(system)
	require.NoError(t, err)
}

func testSystemStoreSaveOrUpdateWithWarnMetricHandling(t *testing.T, ss store.Store) {
	system := &model.System{Name: model.NewID(), Value: "value"}

	err := ss.System().SaveOrUpdateWithWarnMetricHandling(system)
	require.NoError(t, err)

	_, err = ss.System().GetByName(model.SystemWarnMetricLastRunTimestampKey)
	assert.Error(t, err)

	system.Name = "warn_metric_number_of_active_users_100"
	system.Value = model.WarnMetricStatusRunonce
	err = ss.System().SaveOrUpdateWithWarnMetricHandling(system)
	require.NoError(t, err)

	val1, nerr := ss.System().GetByName(model.SystemWarnMetricLastRunTimestampKey)
	assert.NoError(t, nerr)

	system.Name = "warn_metric_number_of_active_users_100"
	system.Value = model.WarnMetricStatusAck
	err = ss.System().SaveOrUpdateWithWarnMetricHandling(system)
	require.NoError(t, err)

	val2, nerr := ss.System().GetByName(model.SystemWarnMetricLastRunTimestampKey)
	assert.NoError(t, nerr)
	assert.Equal(t, val1, val2)
}

func testSystemStoreGetByNameNoEntries(t *testing.T, ss store.Store) {
	res, nErr := ss.System().GetByName(model.SystemFirstAdminVisitMarketplace)
	_, ok := nErr.(*store.ErrNotFound)
	require.Error(t, nErr)
	assert.True(t, ok)
	assert.Nil(t, res)
}

func testSystemStorePermanentDeleteByName(t *testing.T, ss store.Store) {
	s1 := &model.System{Name: model.NewID(), Value: "value"}
	s2 := &model.System{Name: model.NewID(), Value: "value"}

	err := ss.System().Save(s1)
	require.NoError(t, err)
	err = ss.System().Save(s2)
	require.NoError(t, err)

	_, err = ss.System().GetByName(s1.Name)
	assert.NoError(t, err)

	_, err = ss.System().GetByName(s2.Name)
	assert.NoError(t, err)

	_, err = ss.System().PermanentDeleteByName(s1.Name)
	assert.NoError(t, err)

	_, err = ss.System().GetByName(s1.Name)
	assert.Error(t, err)

	_, err = ss.System().GetByName(s2.Name)
	assert.NoError(t, err)

	_, err = ss.System().PermanentDeleteByName(s2.Name)
	assert.NoError(t, err)

	_, err = ss.System().GetByName(s1.Name)
	assert.Error(t, err)

	_, err = ss.System().GetByName(s2.Name)
	assert.Error(t, err)
}

func testInsertIfExists(t *testing.T, ss store.Store) {
	t.Run("Serial", func(t *testing.T) {
		s1 := &model.System{Name: model.SystemClusterEncryptionKey, Value: "somekey"}

		s2, err := ss.System().InsertIfExists(s1)
		require.NoError(t, err)
		assert.Equal(t, s1.Value, s2.Value)

		s1New := &model.System{Name: model.SystemClusterEncryptionKey, Value: "anotherKey"}

		s3, err := ss.System().InsertIfExists(s1New)
		require.NoError(t, err)
		assert.Equal(t, s1.Value, s3.Value)
	})

	t.Run("Concurrent", func(t *testing.T) {
		var s2, s3 *model.System
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			s1 := &model.System{Name: model.SystemClusterEncryptionKey, Value: "firstKey"}
			var err error
			s2, err = ss.System().InsertIfExists(s1)
			require.NoError(t, err)
		}()

		go func() {
			defer wg.Done()
			s1 := &model.System{Name: model.SystemClusterEncryptionKey, Value: "secondKey"}
			var err error
			s3, err = ss.System().InsertIfExists(s1)
			require.NoError(t, err)
		}()
		wg.Wait()
		assert.Equal(t, s2.Value, s3.Value)
	})
}
