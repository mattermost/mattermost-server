// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package users

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/stretchr/testify/require"
)

const (
	dayInMillis = 86400000
	grace       = 5 * 1000
	thirtyDays  = dayInMillis * 30
)

func TestCache(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	session := &model.Session{
		ID:     model.NewID(),
		Token:  model.NewID(),
		UserID: model.NewID(),
	}

	session2 := &model.Session{
		ID:     model.NewID(),
		Token:  model.NewID(),
		UserID: model.NewID(),
	}

	th.service.sessionCache.SetWithExpiry(session.Token, session, 5*time.Minute)
	th.service.sessionCache.SetWithExpiry(session2.Token, session2, 5*time.Minute)

	keys, err := th.service.sessionCache.Keys()
	require.NoError(t, err)
	require.NotEmpty(t, keys)

	th.service.ClearUserSessionCache(session.UserID)

	rkeys, err := th.service.sessionCache.Keys()
	require.NoError(t, err)
	require.Lenf(t, rkeys, len(keys)-1, "should have one less: %d - %d != 1", len(keys), len(rkeys))
	require.NotEmpty(t, rkeys)

	th.service.ClearAllUsersSessionCache()

	rkeys, err = th.service.sessionCache.Keys()
	require.NoError(t, err)
	require.Empty(t, rkeys)
}

func TestSetSessionExpireInDays(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	now := model.GetMillis()
	createAt := now - (dayInMillis * 20)

	tests := []struct {
		name   string
		extend bool
		create bool
		days   int
		want   int64
	}{
		{name: "zero days, extend", extend: true, create: true, days: 0, want: now},
		{name: "zero days, extend", extend: true, create: false, days: 0, want: now},
		{name: "zero days, no extend", extend: false, create: true, days: 0, want: createAt},
		{name: "zero days, no extend", extend: false, create: false, days: 0, want: now},
		{name: "thirty days, extend", extend: true, create: true, days: 30, want: now + thirtyDays},
		{name: "thirty days, extend", extend: true, create: false, days: 30, want: now + thirtyDays},
		{name: "thirty days, no extend", extend: false, create: true, days: 30, want: createAt + thirtyDays},
		{name: "thirty days, no extend", extend: false, create: false, days: 30, want: now + thirtyDays},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th.UpdateConfig(func(cfg *model.Config) {
				*cfg.ServiceSettings.ExtendSessionLengthWithActivity = tt.extend
			})
			var create int64
			if tt.create {
				create = createAt
			}

			session := &model.Session{
				CreateAt:  create,
				ExpiresAt: model.GetMillis() + dayInMillis,
			}
			th.service.SetSessionExpireInDays(session, tt.days)

			// must be within 5 seconds of expected time.
			require.GreaterOrEqual(t, session.ExpiresAt, tt.want-grace)
			require.LessOrEqual(t, session.ExpiresAt, tt.want+grace)
		})
	}
}

func TestOAuthRevokeAccessToken(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	err := th.service.RevokeAccessToken(model.NewRandomString(16))
	require.Error(t, err, "Should have failed due to an incorrect token")

	session := &model.Session{}
	session.CreateAt = model.GetMillis()
	session.UserID = model.NewID()
	session.Token = model.NewID()
	session.Roles = model.SystemUserRoleID
	th.service.SetSessionExpireInDays(session, 1)

	session, _ = th.service.CreateSession(session)
	err = th.service.RevokeAccessToken(session.Token)
	require.Error(t, err, "Should have failed does not have an access token")

	accessData := &model.AccessData{}
	accessData.Token = session.Token
	accessData.UserID = session.UserID
	accessData.RedirectURI = "http://example.com"
	accessData.ClientID = model.NewID()
	accessData.ExpiresAt = session.ExpiresAt

	_, nErr := th.service.oAuthStore.SaveAccessData(accessData)
	require.NoError(t, nErr)

	err = th.service.RevokeAccessToken(accessData.Token)
	require.NoError(t, err)
}
