// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package users

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store/sqlstore"
	"github.com/pkg/errors"
)

func (us *UserService) ReturnSessionToPool(session *model.Session) {
	if session != nil {
		session.ID = ""
		us.sessionPool.Put(session)
	}
}

func (us *UserService) CreateSession(session *model.Session) (*model.Session, error) {
	session.Token = ""

	session, err := us.sessionStore.Save(session)
	if err != nil {
		return nil, err
	}

	us.AddSessionToCache(session)

	return session, nil
}

func (us *UserService) GetSession(token string) (*model.Session, error) {
	var session = us.sessionPool.Get().(*model.Session)
	if err := us.sessionCache.Get(token, session); err == nil {
		if us.metrics != nil {
			us.metrics.IncrementMemCacheHitCounterSession()
		}
	} else {
		if us.metrics != nil {
			us.metrics.IncrementMemCacheMissCounterSession()
		}
	}

	if session.ID != "" {
		return session, nil
	}

	return us.GetSessionContext(sqlstore.WithMaster(context.Background()), token)
}

func (us *UserService) GetSessionContext(ctx context.Context, token string) (*model.Session, error) {
	return us.sessionStore.Get(ctx, token)
}

func (us *UserService) GetSessions(userID string) ([]*model.Session, error) {
	return us.sessionStore.GetSessions(userID)
}

func (us *UserService) AddSessionToCache(session *model.Session) {
	us.sessionCache.SetWithExpiry(session.Token, session, time.Duration(int64(*us.config().ServiceSettings.SessionCacheInMinutes))*time.Minute)
}

func (us *UserService) SessionCacheLength() int {
	if l, err := us.sessionCache.Len(); err == nil {
		return l
	}
	return 0
}

func (us *UserService) ClearUserSessionCacheLocal(userID string) {
	if keys, err := us.sessionCache.Keys(); err == nil {
		var session *model.Session
		for _, key := range keys {
			if err := us.sessionCache.Get(key, &session); err == nil {
				if session.UserID == userID {
					us.sessionCache.Remove(key)
					if us.metrics != nil {
						us.metrics.IncrementMemCacheInvalidationCounterSession()
					}
				}
			}
		}
	}
}

func (us *UserService) ClearAllUsersSessionCacheLocal() {
	us.sessionCache.Purge()
}

func (us *UserService) ClearUserSessionCache(userID string) {
	us.ClearUserSessionCacheLocal(userID)

	if us.cluster != nil {
		msg := &model.ClusterMessage{
			Event:    model.ClusterEventClearSessionCacheForUser,
			SendType: model.ClusterSendReliable,
			Data:     userID,
		}
		us.cluster.SendClusterMessage(msg)
	}
}

func (us *UserService) ClearAllUsersSessionCache() {
	us.ClearAllUsersSessionCacheLocal()

	if us.cluster != nil {
		msg := &model.ClusterMessage{
			Event:    model.ClusterEventClearSessionCacheForAllUsers,
			SendType: model.ClusterSendReliable,
		}
		us.cluster.SendClusterMessage(msg)
	}
}

func (us *UserService) GetSessionByID(sessionID string) (*model.Session, error) {
	return us.sessionStore.Get(context.Background(), sessionID)
}

func (us *UserService) RevokeSessionsFromAllUsers() error {
	// revoke tokens before sessions so they can't be used to relogin
	nErr := us.oAuthStore.RemoveAllAccessData()
	if nErr != nil {
		return errors.Wrap(DeleteAllAccessDataError, nErr.Error())
	}
	err := us.sessionStore.RemoveAllSessions()
	if err != nil {
		return err
	}

	us.ClearAllUsersSessionCache()
	return nil
}

func (us *UserService) RevokeSessionsForDeviceID(userID string, deviceID string, currentSessionID string) error {
	sessions, err := us.sessionStore.GetSessions(userID)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if session.DeviceID == deviceID && session.ID != currentSessionID {
			mlog.Debug("Revoking sessionId for userId. Re-login with the same device Id", mlog.String("session_id", session.ID), mlog.String("user_id", userID))
			if err := us.RevokeSession(session); err != nil {
				mlog.Warn("Could not revoke session for device", mlog.String("device_id", deviceID), mlog.Err(err))
			}
		}
	}

	return nil
}

func (us *UserService) RevokeSession(session *model.Session) error {
	if session.IsOAuth {
		if err := us.RevokeAccessToken(session.Token); err != nil {
			return err
		}
	} else {
		if err := us.sessionStore.Remove(session.ID); err != nil {
			return errors.Wrap(DeleteSessionError, err.Error())
		}
	}

	us.ClearUserSessionCache(session.UserID)

	return nil
}

func (us *UserService) RevokeAccessToken(token string) error {
	session, _ := us.GetSession(token)

	defer us.ReturnSessionToPool(session)

	schan := make(chan error, 1)
	go func() {
		schan <- us.sessionStore.Remove(token)
		close(schan)
	}()

	if _, err := us.oAuthStore.GetAccessData(token); err != nil {
		return errors.Wrap(GetTokenError, err.Error())
	}

	if err := us.oAuthStore.RemoveAccessData(token); err != nil {
		return errors.Wrap(DeleteTokenError, err.Error())
	}

	if err := <-schan; err != nil {
		return errors.Wrap(DeleteSessionError, err.Error())
	}

	if session != nil {
		us.ClearUserSessionCache(session.UserID)
	}

	return nil
}

// SetSessionExpireInDays sets the session's expiry the specified number of days
// relative to either the session creation date or the current time, depending
// on the `ExtendSessionOnActivity` config setting.
func (us *UserService) SetSessionExpireInDays(session *model.Session, days int) {
	if session.CreateAt == 0 || *us.config().ServiceSettings.ExtendSessionLengthWithActivity {
		session.ExpiresAt = model.GetMillis() + (1000 * 60 * 60 * 24 * int64(days))
	} else {
		session.ExpiresAt = session.CreateAt + (1000 * 60 * 60 * 24 * int64(days))
	}
}

func (us *UserService) UpdateSessionsIsGuest(userID string, isGuest bool) error {
	sessions, err := us.GetSessions(userID)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		session.AddProp(model.SessionPropIsGuest, fmt.Sprintf("%t", isGuest))
		err := us.sessionStore.UpdateProps(session)
		if err != nil {
			mlog.Warn("Unable to update isGuest session", mlog.Err(err))
			continue
		}
		us.AddSessionToCache(session)
	}
	return nil
}

func (us *UserService) RevokeAllSessions(userID string) error {
	sessions, err := us.sessionStore.GetSessions(userID)
	if err != nil {
		return errors.Wrap(GetSessionError, err.Error())
	}
	for _, session := range sessions {
		if session.IsOAuth {
			us.RevokeAccessToken(session.Token)
		} else {
			if err := us.sessionStore.Remove(session.ID); err != nil {
				return errors.Wrap(DeleteSessionError, err.Error())
			}
		}
	}

	us.ClearUserSessionCache(userID)

	return nil
}
