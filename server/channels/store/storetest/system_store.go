// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestSystemStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("", func(t *testing.T) { testSystemStore(t, rctx, ss) })
	t.Run("SaveOrUpdate", func(t *testing.T) { testSystemStoreSaveOrUpdate(t, rctx, ss) })
	t.Run("PermanentDeleteByName", func(t *testing.T) { testSystemStorePermanentDeleteByName(t, rctx, ss) })
	t.Run("InsertIfExists", func(t *testing.T) {
		testInsertIfExists(t, rctx, ss)
	})
	t.Run("GetByNameNoEntries", func(t *testing.T) { testSystemStoreGetByNameNoEntries(t, rctx, ss) })
	t.Run("TryClaimIfOlderThan", func(t *testing.T) { testSystemStoreTryClaimIfOlderThan(t, rctx, ss) })
	t.Run("DeleteIfValueEquals", func(t *testing.T) { testSystemStoreDeleteIfValueEquals(t, rctx, ss) })
}

func testSystemStoreTryClaimIfOlderThan(t *testing.T, rctx request.CTX, ss store.Store) {
	name := model.NewId()

	// Real millisecond timestamps throughout, as every production caller
	// provides -- the WHERE clause compares the stored value against
	// (now - minAgeMillis), so an unrealistic tiny literal like "1" would
	// always read as "ancient" and defeat the cooldown check being tested
	// here regardless of minAgeMillis.
	firstValue := strconv.FormatInt(model.GetMillis(), 10)
	claimed, err := ss.System().TryClaimIfOlderThan(name, firstValue, 60000)
	require.NoError(t, err)
	assert.True(t, claimed, "an absent row must always be claimable")

	secondValue := strconv.FormatInt(model.GetMillis(), 10)
	claimed, err = ss.System().TryClaimIfOlderThan(name, secondValue, 60000)
	require.NoError(t, err)
	assert.False(t, claimed, "a fresh claim must refuse a second claim within the cooldown")

	got, err := ss.System().GetByName(name)
	require.NoError(t, err)
	assert.Equal(t, firstValue, got.Value, "a refused claim must not overwrite the winning claim's value")

	// Backdate the row past the cooldown window directly, then confirm a new
	// claim is allowed and does overwrite the value.
	require.NoError(t, ss.System().Update(&model.System{Name: name, Value: "0"}))
	thirdValue := strconv.FormatInt(model.GetMillis(), 10)
	claimed, err = ss.System().TryClaimIfOlderThan(name, thirdValue, 60000)
	require.NoError(t, err)
	assert.True(t, claimed, "a claim older than the cooldown must be claimable again")

	got, err = ss.System().GetByName(name)
	require.NoError(t, err)
	assert.Equal(t, thirdValue, got.Value)

	t.Run("a malformed marker is treated as expired, not a permanent error", func(t *testing.T) {
		malformed := model.NewId()
		require.NoError(t, ss.System().Save(&model.System{Name: malformed, Value: "not-a-number"}))

		claimed, err := ss.System().TryClaimIfOlderThan(malformed, "1", 60000)
		require.NoError(t, err, "a non-numeric existing value must not make the claim error out")
		assert.True(t, claimed, "a malformed marker must be treated as old enough to claim, matching the pre-atomic-claim ParseInt-failure behavior")
	})
}

func testSystemStoreDeleteIfValueEquals(t *testing.T, rctx request.CTX, ss store.Store) {
	name := model.NewId()
	require.NoError(t, ss.System().Save(&model.System{Name: name, Value: "original"}))

	deleted, err := ss.System().DeleteIfValueEquals(name, "not-the-current-value")
	require.NoError(t, err)
	assert.False(t, deleted, "a mismatched expected value must not delete anything")

	_, err = ss.System().GetByName(name)
	assert.NoError(t, err, "the row must still be there after a refused delete")

	deleted, err = ss.System().DeleteIfValueEquals(name, "original")
	require.NoError(t, err)
	assert.True(t, deleted)

	_, err = ss.System().GetByName(name)
	assert.Error(t, err, "the row must be gone after a matched delete")
}

func testSystemStore(t *testing.T, rctx request.CTX, ss store.Store) {
	system := &model.System{Name: model.NewId(), Value: "value"}
	err := ss.System().Save(system)
	require.NoError(t, err)

	system2 := &model.System{Name: model.NewId(), Value: "value2"}
	err = ss.System().Save(system2)
	require.NoError(t, err)

	systems, err := ss.System().Get()
	require.NoError(t, err)
	require.Equal(t, system.Value, systems[system.Name])

	system.Value = "value1"
	err = ss.System().Update(system)
	require.NoError(t, err)

	systems2, err := ss.System().Get()
	require.NoError(t, err)
	require.Equal(t, system.Value, systems2[system.Name])
	require.Equal(t, system2.Value, systems2[system2.Name])

	rsystem, err := ss.System().GetByName(system.Name)
	require.NoError(t, err)
	require.Equal(t, system.Value, rsystem.Value)
}

func testSystemStoreSaveOrUpdate(t *testing.T, rctx request.CTX, ss store.Store) {
	system := &model.System{Name: model.NewId(), Value: "value"}

	err := ss.System().SaveOrUpdate(system)
	require.NoError(t, err)

	res, err := ss.System().GetByName(system.Name)
	require.NoError(t, err)
	assert.Equal(t, system.Value, res.Value)

	system.Value = "value2"

	err = ss.System().SaveOrUpdate(system)
	require.NoError(t, err)

	res, err = ss.System().GetByName(system.Name)
	require.NoError(t, err)
	assert.Equal(t, system.Value, res.Value)
}

func testSystemStoreGetByNameNoEntries(t *testing.T, rctx request.CTX, ss store.Store) {
	res, nErr := ss.System().GetByName(model.SystemFirstAdminVisitMarketplace)
	_, ok := nErr.(*store.ErrNotFound)
	require.Error(t, nErr)
	assert.True(t, ok)
	assert.Nil(t, res)
}

func testSystemStorePermanentDeleteByName(t *testing.T, rctx request.CTX, ss store.Store) {
	s1 := &model.System{Name: model.NewId(), Value: "value"}
	s2 := &model.System{Name: model.NewId(), Value: "value"}

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

func testInsertIfExists(t *testing.T, rctx request.CTX, ss store.Store) {
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
