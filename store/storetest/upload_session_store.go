// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestUploadSessionStore(t *testing.T, ss store.Store) {
	t.Run("UploadSessionStoreSaveGet", func(t *testing.T) { testUploadSessionStoreSaveGet(t, ss) })
	t.Run("UploadSessionStoreUpdate", func(t *testing.T) { testUploadSessionStoreUpdate(t, ss) })
	t.Run("UploadSessionStoreGetForUser", func(t *testing.T) { testUploadSessionStoreGetForUser(t, ss) })
	t.Run("UploadSessionStoreDelete", func(t *testing.T) { testUploadSessionStoreDelete(t, ss) })
}

func testUploadSessionStoreSaveGet(t *testing.T, ss store.Store) {
	var session *model.UploadSession

	t.Run("saving nil session should fail", func(t *testing.T) {
		us, err := ss.UploadSession().Save(nil)
		require.Error(t, err)
		require.Nil(t, us)
	})

	t.Run("saving empty session should fail", func(t *testing.T) {
		session = &model.UploadSession{}
		us, err := ss.UploadSession().Save(session)
		require.Error(t, err)
		require.Nil(t, us)
	})

	t.Run("saving valid session should succeed", func(t *testing.T) {
		session = &model.UploadSession{
			Type:      model.UploadTypeAttachment,
			UserID:    model.NewID(),
			ChannelID: model.NewID(),
			Filename:  "test",
			FileSize:  1024,
			Path:      "/tmp/test",
		}
		us, err := ss.UploadSession().Save(session)
		require.NoError(t, err)
		require.NotNil(t, us)
		require.NotEmpty(t, us)
	})

	t.Run("getting non-existing session should fail", func(t *testing.T) {
		us, err := ss.UploadSession().Get("fake")
		require.Error(t, err)
		require.Nil(t, us)
	})

	t.Run("getting existing session should succeed", func(t *testing.T) {
		us, err := ss.UploadSession().Get(session.ID)
		require.NoError(t, err)
		require.NotNil(t, us)
		require.Equal(t, session, us)
	})
}

func testUploadSessionStoreUpdate(t *testing.T, ss store.Store) {
	session := &model.UploadSession{
		Type:      model.UploadTypeAttachment,
		UserID:    model.NewID(),
		ChannelID: model.NewID(),
		Filename:  "test",
		FileSize:  1024,
		Path:      "/tmp/test",
	}

	t.Run("updating nil session should fail", func(t *testing.T) {
		err := ss.UploadSession().Update(nil)
		require.Error(t, err)
	})

	t.Run("updating invalid session should fail", func(t *testing.T) {
		err := ss.UploadSession().Update(&model.UploadSession{})
		require.Error(t, err)
	})

	t.Run("updating non-existing session should fail", func(t *testing.T) {
		err := ss.UploadSession().Update(&model.UploadSession{})
		require.Error(t, err)
	})

	t.Run("updating existing session should succeed", func(t *testing.T) {
		us, err := ss.UploadSession().Save(session)
		require.NoError(t, err)
		require.NotNil(t, us)
		require.NotEmpty(t, us)

		us.FileOffset = 512
		err = ss.UploadSession().Update(us)
		require.NoError(t, err)

		updated, err := ss.UploadSession().Get(us.ID)
		require.NoError(t, err)
		require.NotNil(t, us)
		require.Equal(t, us, updated)
	})
}

func testUploadSessionStoreGetForUser(t *testing.T, ss store.Store) {
	userID := model.NewID()

	sessions := []*model.UploadSession{
		{
			Type:      model.UploadTypeAttachment,
			UserID:    userID,
			ChannelID: model.NewID(),
			Filename:  "test0",
			FileSize:  1024,
			Path:      "/tmp/test0",
		},
		{
			Type:      model.UploadTypeAttachment,
			UserID:    model.NewID(),
			ChannelID: model.NewID(),
			Filename:  "test1",
			FileSize:  1024,
			Path:      "/tmp/test1",
		},
		{
			Type:      model.UploadTypeAttachment,
			UserID:    userID,
			ChannelID: model.NewID(),
			Filename:  "test2",
			FileSize:  1024,
			Path:      "/tmp/test2",
		},
		{
			Type:      model.UploadTypeAttachment,
			UserID:    userID,
			ChannelID: model.NewID(),
			Filename:  "test3",
			FileSize:  1024,
			Path:      "/tmp/test3",
		},
	}

	t.Run("should return no sessions", func(t *testing.T) {
		us, err := ss.UploadSession().GetForUser(userID)
		require.NoError(t, err)
		require.NotNil(t, us)
		require.Empty(t, us)
	})

	for i := 0; i < len(sessions); i++ {
		us, err := ss.UploadSession().Save(sessions[i])
		require.NoError(t, err)
		require.NotNil(t, us)
		require.NotEmpty(t, us)
		// We need this to make sure the ordering is consistent.
		time.Sleep(1 * time.Millisecond)
	}

	t.Run("should return existing sessions", func(t *testing.T) {
		us, err := ss.UploadSession().GetForUser(userID)
		require.NoError(t, err)
		require.NotNil(t, us)
		require.NotEmpty(t, us)
		require.Len(t, us, 3)

		require.Equal(t, sessions[0], us[0])
		require.Equal(t, sessions[2], us[1])
		require.Equal(t, sessions[3], us[2])
	})
}

func testUploadSessionStoreDelete(t *testing.T, ss store.Store) {
	session := &model.UploadSession{
		ID:        model.NewID(),
		Type:      model.UploadTypeAttachment,
		UserID:    model.NewID(),
		ChannelID: model.NewID(),
		Filename:  "test",
		FileSize:  1024,
		Path:      "/tmp/test",
	}

	t.Run("deleting invalid id should fail", func(t *testing.T) {
		err := ss.UploadSession().Delete("invalidId")
		require.Error(t, err)
	})

	t.Run("deleting existing session should succeed", func(t *testing.T) {
		us, err := ss.UploadSession().Save(session)
		require.NoError(t, err)
		require.NotNil(t, us)
		require.NotEmpty(t, us)

		err = ss.UploadSession().Delete(session.ID)
		require.NoError(t, err)

		us, err = ss.UploadSession().Get(us.ID)
		require.Error(t, err)
		require.Nil(t, us)
		require.IsType(t, &store.ErrNotFound{}, err)
	})
}
