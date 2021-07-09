// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	TenMinutes = 600000
)

func TestSessionStore(t *testing.T, ss store.Store) {
	// Run serially to prevent interfering with other tests
	testSessionCleanup(t, ss)

	t.Run("Save", func(t *testing.T) { testSessionStoreSave(t, ss) })
	t.Run("SessionGet", func(t *testing.T) { testSessionGet(t, ss) })
	t.Run("SessionGetWithDeviceId", func(t *testing.T) { testSessionGetWithDeviceID(t, ss) })
	t.Run("SessionRemove", func(t *testing.T) { testSessionRemove(t, ss) })
	t.Run("SessionRemoveAll", func(t *testing.T) { testSessionRemoveAll(t, ss) })
	t.Run("SessionRemoveByUser", func(t *testing.T) { testSessionRemoveByUser(t, ss) })
	t.Run("SessionRemoveToken", func(t *testing.T) { testSessionRemoveToken(t, ss) })
	t.Run("SessionUpdateDeviceId", func(t *testing.T) { testSessionUpdateDeviceID(t, ss) })
	t.Run("SessionUpdateDeviceId2", func(t *testing.T) { testSessionUpdateDeviceID2(t, ss) })
	t.Run("UpdateExpiresAt", func(t *testing.T) { testSessionStoreUpdateExpiresAt(t, ss) })
	t.Run("UpdateLastActivityAt", func(t *testing.T) { testSessionStoreUpdateLastActivityAt(t, ss) })
	t.Run("SessionCount", func(t *testing.T) { testSessionCount(t, ss) })
	t.Run("GetSessionsExpired", func(t *testing.T) { testGetSessionsExpired(t, ss) })
	t.Run("UpdateExpiredNotify", func(t *testing.T) { testUpdateExpiredNotify(t, ss) })
}

func testSessionStoreSave(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	_, err := ss.Session().Save(s1)
	require.NoError(t, err)
}

func testSessionGet(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	s2 := &model.Session{}
	s2.UserID = s1.UserID

	s2, err = ss.Session().Save(s2)
	require.NoError(t, err)

	s3 := &model.Session{}
	s3.UserID = s1.UserID
	s3.ExpiresAt = 1

	s3, err = ss.Session().Save(s3)
	require.NoError(t, err)

	session, err := ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.Equal(t, session.ID, s1.ID, "should match")

	data, err := ss.Session().GetSessions(s1.UserID)
	require.NoError(t, err)
	require.Len(t, data, 3, "should match len")
}

func testSessionGetWithDeviceID(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()
	s1.ExpiresAt = model.GetMillis() + 10000

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	s2 := &model.Session{}
	s2.UserID = s1.UserID
	s2.DeviceID = model.NewID()
	s2.ExpiresAt = model.GetMillis() + 10000

	s2, err = ss.Session().Save(s2)
	require.NoError(t, err)

	s3 := &model.Session{}
	s3.UserID = s1.UserID
	s3.ExpiresAt = 1
	s3.DeviceID = model.NewID()

	s3, err = ss.Session().Save(s3)
	require.NoError(t, err)

	data, err := ss.Session().GetSessionsWithActiveDeviceIDs(s1.UserID)
	require.NoError(t, err)
	require.Len(t, data, 1, "should match len")
}

func testSessionRemove(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	session, err := ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.Equal(t, session.ID, s1.ID, "should match")

	removeErr := ss.Session().Remove(s1.ID)
	require.NoError(t, removeErr)

	_, err = ss.Session().Get(context.Background(), s1.ID)
	require.Error(t, err, "should have been removed")
}

func testSessionRemoveAll(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	session, err := ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.Equal(t, session.ID, s1.ID, "should match")

	removeErr := ss.Session().RemoveAllSessions()
	require.NoError(t, removeErr)

	_, err = ss.Session().Get(context.Background(), s1.ID)
	require.Error(t, err, "should have been removed")
}

func testSessionRemoveByUser(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	session, err := ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.Equal(t, session.ID, s1.ID, "should match")

	deleteErr := ss.Session().PermanentDeleteSessionsByUser(s1.UserID)
	require.NoError(t, deleteErr)

	_, err = ss.Session().Get(context.Background(), s1.ID)
	require.Error(t, err, "should have been removed")
}

func testSessionRemoveToken(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	session, err := ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.Equal(t, session.ID, s1.ID, "should match")

	removeErr := ss.Session().Remove(s1.Token)
	require.NoError(t, removeErr)

	_, err = ss.Session().Get(context.Background(), s1.ID)
	require.Error(t, err, "should have been removed")

	data, err := ss.Session().GetSessions(s1.UserID)
	require.NoError(t, err)
	require.Empty(t, data, "should match len")
}

func testSessionUpdateDeviceID(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	_, err = ss.Session().UpdateDeviceID(s1.ID, model.PushNotifyApple+":1234567890", s1.ExpiresAt)
	require.NoError(t, err)

	s2 := &model.Session{}
	s2.UserID = model.NewID()

	s2, err = ss.Session().Save(s2)
	require.NoError(t, err)

	_, err = ss.Session().UpdateDeviceID(s2.ID, model.PushNotifyApple+":1234567890", s1.ExpiresAt)
	require.NoError(t, err)
}

func testSessionUpdateDeviceID2(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	_, err = ss.Session().UpdateDeviceID(s1.ID, model.PushNotifyAppleReactNative+":1234567890", s1.ExpiresAt)
	require.NoError(t, err)

	s2 := &model.Session{}
	s2.UserID = model.NewID()

	s2, err = ss.Session().Save(s2)
	require.NoError(t, err)

	_, err = ss.Session().UpdateDeviceID(s2.ID, model.PushNotifyAppleReactNative+":1234567890", s1.ExpiresAt)
	require.NoError(t, err)
}

func testSessionStoreUpdateExpiresAt(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	err = ss.Session().UpdateExpiresAt(s1.ID, 1234567890)
	require.NoError(t, err)

	session, err := ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.EqualValues(t, session.ExpiresAt, 1234567890, "ExpiresAt not updated correctly")
}

func testSessionStoreUpdateLastActivityAt(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	err = ss.Session().UpdateLastActivityAt(s1.ID, 1234567890)
	require.NoError(t, err)

	session, err := ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.EqualValues(t, session.LastActivityAt, 1234567890, "LastActivityAt not updated correctly")
}

func testSessionCount(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()
	s1.ExpiresAt = model.GetMillis() + 100000

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	count, err := ss.Session().AnalyticsSessionCount()
	require.NoError(t, err)
	require.NotZero(t, count, "should have at least 1 session")
}

func testSessionCleanup(t *testing.T, ss store.Store) {
	now := model.GetMillis()

	s1 := &model.Session{}
	s1.UserID = model.NewID()
	s1.ExpiresAt = 0 // never expires

	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	s2 := &model.Session{}
	s2.UserID = s1.UserID
	s2.ExpiresAt = now + 1000000 // expires in the future

	s2, err = ss.Session().Save(s2)
	require.NoError(t, err)

	s3 := &model.Session{}
	s3.UserID = model.NewID()
	s3.ExpiresAt = 1 // expired

	s3, err = ss.Session().Save(s3)
	require.NoError(t, err)

	s4 := &model.Session{}
	s4.UserID = model.NewID()
	s4.ExpiresAt = 2 // expired

	s4, err = ss.Session().Save(s4)
	require.NoError(t, err)

	ss.Session().Cleanup(now, 1)

	_, err = ss.Session().Get(context.Background(), s1.ID)
	assert.NoError(t, err)

	_, err = ss.Session().Get(context.Background(), s2.ID)
	assert.NoError(t, err)

	_, err = ss.Session().Get(context.Background(), s3.ID)
	assert.Error(t, err)

	_, err = ss.Session().Get(context.Background(), s4.ID)
	assert.Error(t, err)

	removeErr := ss.Session().Remove(s1.ID)
	require.NoError(t, removeErr)

	removeErr = ss.Session().Remove(s2.ID)
	require.NoError(t, removeErr)
}

func testGetSessionsExpired(t *testing.T, ss store.Store) {
	now := model.GetMillis()

	// Clear existing sessions.
	err := ss.Session().RemoveAllSessions()
	require.NoError(t, err)

	s1 := &model.Session{}
	s1.UserID = model.NewID()
	s1.DeviceID = model.NewID()
	s1.ExpiresAt = 0 // never expires
	s1, err = ss.Session().Save(s1)
	require.NoError(t, err)

	s2 := &model.Session{}
	s2.UserID = model.NewID()
	s2.DeviceID = model.NewID()
	s2.ExpiresAt = now - TenMinutes // expired within threshold
	s2, err = ss.Session().Save(s2)
	require.NoError(t, err)

	s3 := &model.Session{}
	s3.UserID = model.NewID()
	s3.DeviceID = model.NewID()
	s3.ExpiresAt = now - (TenMinutes * 100) // expired outside threshold
	s3, err = ss.Session().Save(s3)
	require.NoError(t, err)

	s4 := &model.Session{}
	s4.UserID = model.NewID()
	s4.ExpiresAt = now - TenMinutes // expired within threshold, but not mobile
	s4, err = ss.Session().Save(s4)
	require.NoError(t, err)

	s5 := &model.Session{}
	s5.UserID = model.NewID()
	s5.DeviceID = model.NewID()
	s5.ExpiresAt = now + (TenMinutes * 100000) // not expired
	s5, err = ss.Session().Save(s5)
	require.NoError(t, err)

	sessions, err := ss.Session().GetSessionsExpired(TenMinutes*2, true, true) // mobile only
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, s2.ID, sessions[0].ID)

	sessions, err = ss.Session().GetSessionsExpired(TenMinutes*2, false, true) // all client types
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	expected := []string{s2.ID, s4.ID}
	for _, sess := range sessions {
		require.Contains(t, expected, sess.ID)
	}
}

func testUpdateExpiredNotify(t *testing.T, ss store.Store) {
	s1 := &model.Session{}
	s1.UserID = model.NewID()
	s1.DeviceID = model.NewID()
	s1.ExpiresAt = model.GetMillis() + TenMinutes
	s1, err := ss.Session().Save(s1)
	require.NoError(t, err)

	session, err := ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.False(t, session.ExpiredNotify)

	err = ss.Session().UpdateExpiredNotify(session.ID, true)
	require.NoError(t, err)
	session, err = ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.True(t, session.ExpiredNotify)

	err = ss.Session().UpdateExpiredNotify(session.ID, false)
	require.NoError(t, err)
	session, err = ss.Session().Get(context.Background(), s1.ID)
	require.NoError(t, err)
	require.False(t, session.ExpiredNotify)
}
