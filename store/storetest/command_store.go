// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestCommandStore(t *testing.T, ss store.Store) {
	t.Run("Save", func(t *testing.T) { testCommandStoreSave(t, ss) })
	t.Run("Get", func(t *testing.T) { testCommandStoreGet(t, ss) })
	t.Run("GetByTeam", func(t *testing.T) { testCommandStoreGetByTeam(t, ss) })
	t.Run("GetByTrigger", func(t *testing.T) { testCommandStoreGetByTrigger(t, ss) })
	t.Run("Delete", func(t *testing.T) { testCommandStoreDelete(t, ss) })
	t.Run("DeleteByTeam", func(t *testing.T) { testCommandStoreDeleteByTeam(t, ss) })
	t.Run("DeleteByUser", func(t *testing.T) { testCommandStoreDeleteByUser(t, ss) })
	t.Run("Update", func(t *testing.T) { testCommandStoreUpdate(t, ss) })
	t.Run("CommandCount", func(t *testing.T) { testCommandCount(t, ss) })
}

func testCommandStoreSave(t *testing.T, ss store.Store) {
	o1 := model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger"

	_, nErr := ss.Command().Save(&o1)
	require.NoError(t, nErr)

	_, err := ss.Command().Save(&o1)
	require.Error(t, err, "shouldn't be able to update from save")
}

func testCommandStoreGet(t *testing.T, ss store.Store) {
	o1 := &model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger"

	o1, nErr := ss.Command().Save(o1)
	require.NoError(t, nErr)

	r1, nErr := ss.Command().Get(o1.ID)
	require.NoError(t, nErr)
	require.Equal(t, r1.CreateAt, o1.CreateAt, "invalid returned command")

	_, err := ss.Command().Get("123")
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}

func testCommandStoreGetByTeam(t *testing.T, ss store.Store) {
	o1 := &model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger"

	o1, nErr := ss.Command().Save(o1)
	require.NoError(t, nErr)

	r1, nErr := ss.Command().GetByTeam(o1.TeamID)
	require.NoError(t, nErr)
	require.NotEmpty(t, r1, "no command returned")
	require.Equal(t, r1[0].CreateAt, o1.CreateAt, "invalid returned command")

	result, nErr := ss.Command().GetByTeam("123")
	require.NoError(t, nErr)
	require.Empty(t, result, "no commands should have returned")
}

func testCommandStoreGetByTrigger(t *testing.T, ss store.Store) {
	o1 := &model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger1"

	o2 := &model.Command{}
	o2.CreatorID = model.NewID()
	o2.Method = model.CommandMethodPost
	o2.TeamID = model.NewID()
	o2.URL = "http://nowhere.com/"
	o2.Trigger = "trigger1"

	o1, nErr := ss.Command().Save(o1)
	require.NoError(t, nErr)

	_, nErr = ss.Command().Save(o2)
	require.NoError(t, nErr)

	var r1 *model.Command
	r1, nErr = ss.Command().GetByTrigger(o1.TeamID, o1.Trigger)
	require.NoError(t, nErr)
	require.Equal(t, r1.ID, o1.ID, "invalid returned command")

	nErr = ss.Command().Delete(o1.ID, model.GetMillis())
	require.NoError(t, nErr)

	_, err := ss.Command().GetByTrigger(o1.TeamID, o1.Trigger)
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}

func testCommandStoreDelete(t *testing.T, ss store.Store) {
	o1 := &model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger"

	o1, nErr := ss.Command().Save(o1)
	require.NoError(t, nErr)

	r1, nErr := ss.Command().Get(o1.ID)
	require.NoError(t, nErr)
	require.Equal(t, r1.CreateAt, o1.CreateAt, "invalid returned command")

	nErr = ss.Command().Delete(o1.ID, model.GetMillis())
	require.NoError(t, nErr)

	_, err := ss.Command().Get(o1.ID)
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}

func testCommandStoreDeleteByTeam(t *testing.T, ss store.Store) {
	o1 := &model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger"

	o1, nErr := ss.Command().Save(o1)
	require.NoError(t, nErr)

	r1, nErr := ss.Command().Get(o1.ID)
	require.NoError(t, nErr)
	require.Equal(t, r1.CreateAt, o1.CreateAt, "invalid returned command")

	nErr = ss.Command().PermanentDeleteByTeam(o1.TeamID)
	require.NoError(t, nErr)

	_, err := ss.Command().Get(o1.ID)
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}

func testCommandStoreDeleteByUser(t *testing.T, ss store.Store) {
	o1 := &model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger"

	o1, nErr := ss.Command().Save(o1)
	require.NoError(t, nErr)

	r1, nErr := ss.Command().Get(o1.ID)
	require.NoError(t, nErr)
	require.Equal(t, r1.CreateAt, o1.CreateAt, "invalid returned command")

	nErr = ss.Command().PermanentDeleteByUser(o1.CreatorID)
	require.NoError(t, nErr)

	_, err := ss.Command().Get(o1.ID)
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr))
}

func testCommandStoreUpdate(t *testing.T, ss store.Store) {
	o1 := &model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger"

	o1, nErr := ss.Command().Save(o1)
	require.NoError(t, nErr)

	o1.Token = model.NewID()

	_, nErr = ss.Command().Update(o1)
	require.NoError(t, nErr)

	o1.URL = "junk"

	_, err := ss.Command().Update(o1)
	require.Error(t, err)
}

func testCommandCount(t *testing.T, ss store.Store) {
	o1 := &model.Command{}
	o1.CreatorID = model.NewID()
	o1.Method = model.CommandMethodPost
	o1.TeamID = model.NewID()
	o1.URL = "http://nowhere.com/"
	o1.Trigger = "trigger"

	o1, nErr := ss.Command().Save(o1)
	require.NoError(t, nErr)

	r1, nErr := ss.Command().AnalyticsCommandCount("")
	require.NoError(t, nErr)
	require.NotZero(t, r1, "should be at least 1 command")

	r2, nErr := ss.Command().AnalyticsCommandCount(o1.TeamID)
	require.NoError(t, nErr)
	require.Equal(t, r2, int64(1), "should be 1 command")
}
