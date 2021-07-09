// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestCommandWebhookStore(t *testing.T, ss store.Store) {
	t.Run("", func(t *testing.T) { testCommandWebhookStore(t, ss) })
}

func testCommandWebhookStore(t *testing.T, ss store.Store) {
	cws := ss.CommandWebhook()

	h1 := &model.CommandWebhook{}
	h1.CommandID = model.NewID()
	h1.UserID = model.NewID()
	h1.ChannelID = model.NewID()
	h1, err := cws.Save(h1)
	require.NoError(t, err)

	var r1 *model.CommandWebhook
	r1, nErr := cws.Get(h1.ID)
	require.NoError(t, nErr)
	assert.Equal(t, *r1, *h1, "invalid returned webhook")

	_, nErr = cws.Get("123")
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(nErr, &nfErr), "Should have set the status as not found for missing id")

	h2 := &model.CommandWebhook{}
	h2.CreateAt = model.GetMillis() - 2*model.CommandWebhookLifetime
	h2.CommandID = model.NewID()
	h2.UserID = model.NewID()
	h2.ChannelID = model.NewID()
	h2, err = cws.Save(h2)
	require.NoError(t, err)

	_, nErr = cws.Get(h2.ID)
	require.Error(t, nErr, "Should have set the status as not found for expired webhook")
	require.True(t, errors.As(nErr, &nfErr), "Should have set the status as not found for expired webhook")

	cws.Cleanup()

	_, nErr = cws.Get(h1.ID)
	require.NoError(t, nErr, "Should have no error getting unexpired webhook")

	_, nErr = cws.Get(h2.ID)
	require.True(t, errors.As(nErr, &nfErr), "Should have set the status as not found for expired webhook")

	nErr = cws.TryUse(h1.ID, 1)
	require.NoError(t, nErr, "Should be able to use webhook once")

	nErr = cws.TryUse(h1.ID, 1)
	require.Error(t, nErr, "Should be able to use webhook once")
	var invErr *store.ErrInvalidInput
	require.True(t, errors.As(nErr, &invErr), "Should be able to use webhook once")
}
