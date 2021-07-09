// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mfa

import (
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/dgryski/dgoogauth"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSecret(t *testing.T) {
	userID := "user-id"
	userEmail := "sample@sample.com"
	siteURL := "http://localhost:8065"

	t.Run("fail on store action fail", func(t *testing.T) {
		storeMock := mocks.UserStore{}
		storeMock.On("UpdateMfaSecret", userID, mock.AnythingOfType("string")).Return(func(userID string, secret string) error {
			return errors.New("failed to update mfa secret")
		})

		_, _, err := New(&storeMock).GenerateSecret(siteURL, userEmail, userID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to store mfa secret")
	})

	t.Run("Successful generate secret", func(t *testing.T) {
		storeMock := mocks.UserStore{}
		storeMock.On("UpdateMfaSecret", userID, mock.AnythingOfType("string")).Return(func(userID string, secret string) error {
			return nil
		})

		secret, img, err := New(&storeMock).GenerateSecret(siteURL, userEmail, userID)
		require.NoError(t, err)
		assert.Len(t, secret, 32)
		require.NotEmpty(t, img, "no image set")
	})
}

func TestGetIssuerFromUrl(t *testing.T) {
	cases := []struct {
		Input    string
		Expected string
	}{
		{"http://somewebsite.com", url.QueryEscape("somewebsite.com")},
		{"https://somewebsite.com", url.QueryEscape("somewebsite.com")},
		{"https://some.website.com", url.QueryEscape("some.website.com")},
		{" https://www.somewebsite.com", url.QueryEscape("somewebsite.com")},
		{"http://somewebsite.com/chat", url.QueryEscape("somewebsite.com/chat")},
		{"somewebsite.com ", url.QueryEscape("somewebsite.com")},
		{"http://localhost:8065", url.QueryEscape("localhost:8065")},
		{"", "Mattermost"},
		{"  ", "Mattermost"},
	}

	for _, c := range cases {
		assert.Equal(t, c.Expected, getIssuerFromUrl(c.Input))
	}
}

func TestActivate(t *testing.T) {
	userID := "user-id"
	userMfaSecret := newRandomBase32String(mfaSecretSize)

	token := dgoogauth.ComputeCode(userMfaSecret, time.Now().UTC().Unix()/30)

	t.Run("fail on wrongly formatted token", func(t *testing.T) {
		err := New(nil).Activate(userMfaSecret, userID, "invalid-token")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to parse the token")
	})

	t.Run("fail on invalid token", func(t *testing.T) {
		err := New(nil).Activate(userMfaSecret, userID, "000000")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid mfa token")
	})

	t.Run("fail on store action fail", func(t *testing.T) {
		storeMock := mocks.UserStore{}
		storeMock.On("UpdateMfaActive", userID, true).Return(func(userID string, active bool) error {
			return errors.New("failed to update mfa active")
		})

		err := New(&storeMock).Activate(userMfaSecret, userID, fmt.Sprintf("%06d", token))
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to store mfa active")
	})

	t.Run("Successful activate", func(t *testing.T) {
		storeMock := mocks.UserStore{}
		storeMock.On("UpdateMfaActive", userID, true).Return(func(userID string, active bool) error {
			return nil
		})

		err := New(&storeMock).Activate(userMfaSecret, userID, fmt.Sprintf("%06d", token))
		require.NoError(t, err)
	})
}

func TestDeactivate(t *testing.T) {
	userID := "user-id"

	t.Run("fail on store UpdateMfaActive action fail", func(t *testing.T) {
		storeMock := mocks.UserStore{}
		storeMock.On("UpdateMfaActive", userID, false).Return(func(userID string, active bool) error {
			return errors.New("failed to update mfa active")
		})
		storeMock.On("UpdateMfaSecret", userID, "").Return(func(userID string, secret string) error {
			return errors.New("failed to update mfa secret")
		})

		err := New(&storeMock).Deactivate(userID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to store mfa active")
	})

	t.Run("fail on store UpdateMfaSecret action fail", func(t *testing.T) {
		storeMock := mocks.UserStore{}
		storeMock.On("UpdateMfaActive", userID, false).Return(func(userID string, active bool) error {
			return nil
		})
		storeMock.On("UpdateMfaSecret", userID, "").Return(func(userID string, secret string) error {
			return errors.New("failed to update mfa secret")
		})

		err := New(&storeMock).Deactivate(userID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to store mfa secret")
	})

	t.Run("Successful deactivate", func(t *testing.T) {
		storeMock := mocks.UserStore{}
		storeMock.On("UpdateMfaActive", userID, false).Return(func(userID string, active bool) error {
			return nil
		})
		storeMock.On("UpdateMfaSecret", userID, "").Return(func(userID string, secret string) error {
			return nil
		})

		err := New(&storeMock).Deactivate(userID)
		require.NoError(t, err)
	})
}

func TestValidateToken(t *testing.T) {
	secret := newRandomBase32String(mfaSecretSize)
	token := dgoogauth.ComputeCode(secret, time.Now().UTC().Unix()/30)

	t.Run("fail on wrongly formatted token", func(t *testing.T) {
		ok, err := New(nil).ValidateToken(secret, "invalid-token")
		require.Error(t, err)
		require.False(t, ok)
		require.Contains(t, err.Error(), "unable to parse the token")
	})

	t.Run("fail on invalid token", func(t *testing.T) {
		ok, err := New(nil).ValidateToken(secret, "000000")
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("valid token", func(t *testing.T) {
		ok, err := New(nil).ValidateToken(secret, fmt.Sprintf("%06d", token))
		require.NoError(t, err)
		require.True(t, ok)
	})
}

func TestRandomBase32String(t *testing.T) {
	for i := 0; i < 1000; i++ {
		str := newRandomBase32String(i)
		require.Len(t, str, base32.StdEncoding.EncodedLen(i))
	}
}
