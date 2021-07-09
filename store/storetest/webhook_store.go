// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestWebhookStore(t *testing.T, ss store.Store) {
	t.Run("SaveIncoming", func(t *testing.T) { testWebhookStoreSaveIncoming(t, ss) })
	t.Run("UpdateIncoming", func(t *testing.T) { testWebhookStoreUpdateIncoming(t, ss) })
	t.Run("GetIncoming", func(t *testing.T) { testWebhookStoreGetIncoming(t, ss) })
	t.Run("GetIncomingList", func(t *testing.T) { testWebhookStoreGetIncomingList(t, ss) })
	t.Run("GetIncomingListByUser", func(t *testing.T) { testWebhookStoreGetIncomingListByUser(t, ss) })
	t.Run("GetIncomingByTeam", func(t *testing.T) { testWebhookStoreGetIncomingByTeam(t, ss) })
	t.Run("GetIncomingByTeamByUser", func(t *testing.T) { TestWebhookStoreGetIncomingByTeamByUser(t, ss) })
	t.Run("GetIncomingByTeamByChannel", func(t *testing.T) { testWebhookStoreGetIncomingByChannel(t, ss) })
	t.Run("DeleteIncoming", func(t *testing.T) { testWebhookStoreDeleteIncoming(t, ss) })
	t.Run("DeleteIncomingByChannel", func(t *testing.T) { testWebhookStoreDeleteIncomingByChannel(t, ss) })
	t.Run("DeleteIncomingByUser", func(t *testing.T) { testWebhookStoreDeleteIncomingByUser(t, ss) })
	t.Run("SaveOutgoing", func(t *testing.T) { testWebhookStoreSaveOutgoing(t, ss) })
	t.Run("GetOutgoing", func(t *testing.T) { testWebhookStoreGetOutgoing(t, ss) })
	t.Run("GetOutgoingList", func(t *testing.T) { testWebhookStoreGetOutgoingList(t, ss) })
	t.Run("GetOutgoingListByUser", func(t *testing.T) { testWebhookStoreGetOutgoingListByUser(t, ss) })
	t.Run("GetOutgoingByChannel", func(t *testing.T) { testWebhookStoreGetOutgoingByChannel(t, ss) })
	t.Run("GetOutgoingByChannelByUser", func(t *testing.T) { testWebhookStoreGetOutgoingByChannelByUser(t, ss) })
	t.Run("GetOutgoingByTeam", func(t *testing.T) { testWebhookStoreGetOutgoingByTeam(t, ss) })
	t.Run("GetOutgoingByTeamByUser", func(t *testing.T) { testWebhookStoreGetOutgoingByTeamByUser(t, ss) })
	t.Run("DeleteOutgoing", func(t *testing.T) { testWebhookStoreDeleteOutgoing(t, ss) })
	t.Run("DeleteOutgoingByChannel", func(t *testing.T) { testWebhookStoreDeleteOutgoingByChannel(t, ss) })
	t.Run("DeleteOutgoingByUser", func(t *testing.T) { testWebhookStoreDeleteOutgoingByUser(t, ss) })
	t.Run("UpdateOutgoing", func(t *testing.T) { testWebhookStoreUpdateOutgoing(t, ss) })
	t.Run("CountIncoming", func(t *testing.T) { testWebhookStoreCountIncoming(t, ss) })
	t.Run("CountOutgoing", func(t *testing.T) { testWebhookStoreCountOutgoing(t, ss) })
}

func testWebhookStoreSaveIncoming(t *testing.T, ss store.Store) {
	o1 := buildIncomingWebhook()

	_, err := ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err, "couldn't save item")

	_, err = ss.Webhook().SaveIncoming(o1)
	require.Error(t, err, "shouldn't be able to update from save")
}

func testWebhookStoreUpdateIncoming(t *testing.T, ss store.Store) {

	var err error

	o1 := buildIncomingWebhook()
	o1, err = ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err, "unable to save webhook")

	previousUpdatedAt := o1.UpdateAt

	o1.DisplayName = "TestHook"
	time.Sleep(10 * time.Millisecond)

	webhook, err := ss.Webhook().UpdateIncoming(o1)
	require.NoError(t, err)

	require.NotEqual(t, webhook.UpdateAt, previousUpdatedAt, "should have updated the UpdatedAt of the hook")

	require.Equal(t, "TestHook", webhook.DisplayName, "display name is not updated")
}

func testWebhookStoreGetIncoming(t *testing.T, ss store.Store) {
	var err error

	o1 := buildIncomingWebhook()
	o1, err = ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err, "unable to save webhook")

	webhook, err := ss.Webhook().GetIncoming(o1.ID, false)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	webhook, err = ss.Webhook().GetIncoming(o1.ID, true)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	_, err = ss.Webhook().GetIncoming("123", false)
	require.Error(t, err, "Missing id should have failed")

	_, err = ss.Webhook().GetIncoming("123", true)
	require.Error(t, err, "Missing id should have failed")

	_, err = ss.Webhook().GetIncoming("123", true)
	require.Error(t, err)
	var nfErr *store.ErrNotFound
	require.True(t, errors.As(err, &nfErr), "Should have set the status as not found for missing id")
}

func testWebhookStoreGetIncomingList(t *testing.T, ss store.Store) {
	o1 := &model.IncomingWebhook{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.TeamID = model.NewID()

	var err error
	o1, err = ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err, "unable to save webhook")

	hooks, err := ss.Webhook().GetIncomingList(0, 1000)
	require.NoError(t, err)

	found := false
	for _, hook := range hooks {
		if hook.ID == o1.ID {
			found = true
		}
	}
	require.True(t, found, "missing webhook")

	hooks, err = ss.Webhook().GetIncomingList(0, 1)
	require.NoError(t, err)
	require.Len(t, hooks, 1, "only 1 should be returned")
}

func testWebhookStoreGetIncomingListByUser(t *testing.T, ss store.Store) {
	o1 := &model.IncomingWebhook{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.TeamID = model.NewID()

	o1, err := ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err)

	t.Run("GetIncomingListByUser, known user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetIncomingListByUser(o1.UserID, 0, 100)
		require.NoError(t, err)
		require.Equal(t, 1, len(hooks))
		require.Equal(t, o1.CreateAt, hooks[0].CreateAt)
	})

	t.Run("GetIncomingListByUser, unknown user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetIncomingListByUser("123465", 0, 100)
		require.NoError(t, err)
		require.Equal(t, 0, len(hooks))
	})
}

func testWebhookStoreGetIncomingByTeam(t *testing.T, ss store.Store) {
	var err error

	o1 := buildIncomingWebhook()
	o1, err = ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err)

	hooks, err := ss.Webhook().GetIncomingByTeam(o1.TeamID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, hooks[0].CreateAt, o1.CreateAt, "invalid returned webhook")

	hooks, err = ss.Webhook().GetIncomingByTeam("123", 0, 100)
	require.NoError(t, err)
	require.Empty(t, hooks, "no webhooks should have returned")
}

func TestWebhookStoreGetIncomingByTeamByUser(t *testing.T, ss store.Store) {
	var err error

	o1 := buildIncomingWebhook()
	o1, err = ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err)

	o2 := buildIncomingWebhook()
	o2.TeamID = o1.TeamID //Set both to the same team
	o2, err = ss.Webhook().SaveIncoming(o2)
	require.NoError(t, err)

	t.Run("GetIncomingByTeamByUser, no user filter", func(t *testing.T) {
		hooks, err := ss.Webhook().GetIncomingByTeam(o1.TeamID, 0, 100)
		require.NoError(t, err)
		require.Equal(t, len(hooks), 2)
	})

	t.Run("GetIncomingByTeamByUser, known user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetIncomingByTeamByUser(o1.TeamID, o1.UserID, 0, 100)
		require.NoError(t, err)
		require.Equal(t, len(hooks), 1)
		require.Equal(t, hooks[0].CreateAt, o1.CreateAt)
	})

	t.Run("GetIncomingByTeamByUser, unknown user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetIncomingByTeamByUser(o2.TeamID, "123465", 0, 100)
		require.NoError(t, err)
		require.Equal(t, len(hooks), 0)
	})
}

func testWebhookStoreGetIncomingByChannel(t *testing.T, ss store.Store) {
	o1 := buildIncomingWebhook()

	o1, err := ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err, "unable to save webhook")

	webhooks, err := ss.Webhook().GetIncomingByChannel(o1.ChannelID)
	require.NoError(t, err)
	require.Equal(t, webhooks[0].CreateAt, o1.CreateAt, "invalid returned webhook")

	webhooks, err = ss.Webhook().GetIncomingByChannel("123")
	require.NoError(t, err)
	require.Empty(t, webhooks, "no webhooks should have returned")
}

func testWebhookStoreDeleteIncoming(t *testing.T, ss store.Store) {
	var err error

	o1 := buildIncomingWebhook()
	o1, err = ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err, "unable to save webhook")

	webhook, err := ss.Webhook().GetIncoming(o1.ID, true)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	err = ss.Webhook().DeleteIncoming(o1.ID, model.GetMillis())
	require.NoError(t, err)

	webhook, err = ss.Webhook().GetIncoming(o1.ID, true)
	require.Error(t, err)
}

func testWebhookStoreDeleteIncomingByChannel(t *testing.T, ss store.Store) {
	var err error

	o1 := buildIncomingWebhook()
	o1, err = ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err, "unable to save webhook")

	webhook, err := ss.Webhook().GetIncoming(o1.ID, true)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	err = ss.Webhook().PermanentDeleteIncomingByChannel(o1.ChannelID)
	require.NoError(t, err)

	_, err = ss.Webhook().GetIncoming(o1.ID, true)
	require.Error(t, err, "Missing id should have failed")
}

func testWebhookStoreDeleteIncomingByUser(t *testing.T, ss store.Store) {
	var err error

	o1 := buildIncomingWebhook()
	o1, err = ss.Webhook().SaveIncoming(o1)
	require.NoError(t, err, "unable to save webhook")

	webhook, err := ss.Webhook().GetIncoming(o1.ID, true)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	err = ss.Webhook().PermanentDeleteIncomingByUser(o1.UserID)
	require.NoError(t, err)

	_, err = ss.Webhook().GetIncoming(o1.ID, true)
	require.Error(t, err, "Missing id should have failed")
}

func buildIncomingWebhook() *model.IncomingWebhook {
	o1 := &model.IncomingWebhook{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.TeamID = model.NewID()

	return o1
}

func testWebhookStoreSaveOutgoing(t *testing.T, ss store.Store) {
	o1 := model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}
	o1.Username = "test-user-name"
	o1.IconURL = "http://nowhere.com/icon"

	_, err := ss.Webhook().SaveOutgoing(&o1)
	require.NoError(t, err, "couldn't save item")

	_, err = ss.Webhook().SaveOutgoing(&o1)
	require.Error(t, err, "shouldn't be able to update from save")
}

func testWebhookStoreGetOutgoing(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}
	o1.Username = "test-user-name"
	o1.IconURL = "http://nowhere.com/icon"

	o1, _ = ss.Webhook().SaveOutgoing(o1)

	webhook, err := ss.Webhook().GetOutgoing(o1.ID)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	_, err = ss.Webhook().GetOutgoing("123")
	require.Error(t, err, "Missing id should have failed")
}

func testWebhookStoreGetOutgoingListByUser(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, err := ss.Webhook().SaveOutgoing(o1)
	require.NoError(t, err)

	t.Run("GetOutgoingListByUser, known user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetOutgoingListByUser(o1.CreatorID, 0, 100)
		require.NoError(t, err)
		require.Equal(t, 1, len(hooks))
		require.Equal(t, o1.CreateAt, hooks[0].CreateAt)
	})

	t.Run("GetOutgoingListByUser, unknown user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetOutgoingListByUser("123465", 0, 100)
		require.NoError(t, err)
		require.Equal(t, 0, len(hooks))
	})
}

func testWebhookStoreGetOutgoingList(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, _ = ss.Webhook().SaveOutgoing(o1)

	o2 := &model.OutgoingWebhook{}
	o2.ChannelID = model.NewID()
	o2.CreatorID = model.NewID()
	o2.TeamID = model.NewID()
	o2.CallbackURLs = []string{"http://nowhere.com/"}

	o2, _ = ss.Webhook().SaveOutgoing(o2)

	r1, err := ss.Webhook().GetOutgoingList(0, 1000)
	require.NoError(t, err)
	hooks := r1
	found1 := false
	found2 := false

	for _, hook := range hooks {
		if hook.CreateAt != o1.CreateAt {
			found1 = true
		}

		if hook.CreateAt != o2.CreateAt {
			found2 = true
		}
	}

	require.True(t, found1, "missing hook1")
	require.True(t, found2, "missing hook2")

	result, err := ss.Webhook().GetOutgoingList(0, 2)
	require.NoError(t, err)
	require.Len(t, result, 2, "wrong number of hooks returned")
}

func testWebhookStoreGetOutgoingByChannel(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, _ = ss.Webhook().SaveOutgoing(o1)

	r1, err := ss.Webhook().GetOutgoingByChannel(o1.ChannelID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, r1[0].CreateAt, o1.CreateAt, "invalid returned webhook")

	result, err := ss.Webhook().GetOutgoingByChannel("123", -1, -1)
	require.NoError(t, err)
	require.Empty(t, result, "no webhooks should have returned")
}

func testWebhookStoreGetOutgoingByChannelByUser(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, err := ss.Webhook().SaveOutgoing(o1)
	require.NoError(t, err)

	o2 := &model.OutgoingWebhook{}
	o2.ChannelID = o1.ChannelID
	o2.CreatorID = model.NewID()
	o2.TeamID = model.NewID()
	o2.CallbackURLs = []string{"http://nowhere.com/"}

	o2, err = ss.Webhook().SaveOutgoing(o2)
	require.NoError(t, err)

	t.Run("GetOutgoingByChannelByUser, no user filter", func(t *testing.T) {
		hooks, err := ss.Webhook().GetOutgoingByChannel(o1.ChannelID, 0, 100)
		require.NoError(t, err)
		require.Equal(t, len(hooks), 2)
	})

	t.Run("GetOutgoingByChannelByUser, known user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetOutgoingByChannelByUser(o1.ChannelID, o1.CreatorID, 0, 100)
		require.NoError(t, err)
		require.Equal(t, 1, len(hooks))
		require.Equal(t, o1.CreateAt, hooks[0].CreateAt)
	})

	t.Run("GetOutgoingByChannelByUser, unknown user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetOutgoingByChannelByUser(o1.ChannelID, "123465", 0, 100)
		require.NoError(t, err)
		require.Equal(t, 0, len(hooks))
	})
}

func testWebhookStoreGetOutgoingByTeam(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, _ = ss.Webhook().SaveOutgoing(o1)

	r1, err := ss.Webhook().GetOutgoingByTeam(o1.TeamID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, r1[0].CreateAt, o1.CreateAt, "invalid returned webhook")

	result, err := ss.Webhook().GetOutgoingByTeam("123", -1, -1)
	require.NoError(t, err)
	require.Empty(t, result, "no webhooks should have returned")
}

func testWebhookStoreGetOutgoingByTeamByUser(t *testing.T, ss store.Store) {
	var err error

	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, err = ss.Webhook().SaveOutgoing(o1)
	require.NoError(t, err)

	o2 := &model.OutgoingWebhook{}
	o2.ChannelID = model.NewID()
	o2.CreatorID = model.NewID()
	o2.TeamID = o1.TeamID
	o2.CallbackURLs = []string{"http://nowhere.com/"}

	o2, err = ss.Webhook().SaveOutgoing(o2)
	require.NoError(t, err)

	t.Run("GetOutgoingByTeamByUser, no user filter", func(t *testing.T) {
		hooks, err := ss.Webhook().GetOutgoingByTeam(o1.TeamID, 0, 100)
		require.NoError(t, err)
		require.Equal(t, len(hooks), 2)
	})

	t.Run("GetOutgoingByTeamByUser, known user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetOutgoingByTeamByUser(o1.TeamID, o1.CreatorID, 0, 100)
		require.NoError(t, err)
		require.Equal(t, len(hooks), 1)
		require.Equal(t, hooks[0].CreateAt, o1.CreateAt)
	})

	t.Run("GetOutgoingByTeamByUser, unknown user filtered", func(t *testing.T) {
		hooks, err := ss.Webhook().GetOutgoingByTeamByUser(o2.TeamID, "123465", 0, 100)
		require.NoError(t, err)
		require.Equal(t, len(hooks), 0)
	})
}

func testWebhookStoreDeleteOutgoing(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, _ = ss.Webhook().SaveOutgoing(o1)

	webhook, err := ss.Webhook().GetOutgoing(o1.ID)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	err = ss.Webhook().DeleteOutgoing(o1.ID, model.GetMillis())
	require.NoError(t, err)

	_, err = ss.Webhook().GetOutgoing(o1.ID)
	require.Error(t, err, "Missing id should have failed")
}

func testWebhookStoreDeleteOutgoingByChannel(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, _ = ss.Webhook().SaveOutgoing(o1)

	webhook, err := ss.Webhook().GetOutgoing(o1.ID)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	err = ss.Webhook().PermanentDeleteOutgoingByChannel(o1.ChannelID)
	require.NoError(t, err)

	_, err = ss.Webhook().GetOutgoing(o1.ID)
	require.Error(t, err, "Missing id should have failed")
}

func testWebhookStoreDeleteOutgoingByUser(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	o1, _ = ss.Webhook().SaveOutgoing(o1)

	webhook, err := ss.Webhook().GetOutgoing(o1.ID)
	require.NoError(t, err)
	require.Equal(t, webhook.CreateAt, o1.CreateAt, "invalid returned webhook")

	err = ss.Webhook().PermanentDeleteOutgoingByUser(o1.CreatorID)
	require.NoError(t, err)

	_, err = ss.Webhook().GetOutgoing(o1.ID)
	require.Error(t, err, "Missing id should have failed")
}

func testWebhookStoreUpdateOutgoing(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}
	o1.Username = "test-user-name"
	o1.IconURL = "http://nowhere.com/icon"

	o1, _ = ss.Webhook().SaveOutgoing(o1)

	o1.Token = model.NewID()
	o1.Username = "another-test-user-name"

	_, err := ss.Webhook().UpdateOutgoing(o1)
	require.NoError(t, err)
}

func testWebhookStoreCountIncoming(t *testing.T, ss store.Store) {
	o1 := &model.IncomingWebhook{}
	o1.ChannelID = model.NewID()
	o1.UserID = model.NewID()
	o1.TeamID = model.NewID()

	_, _ = ss.Webhook().SaveIncoming(o1)

	c, err := ss.Webhook().AnalyticsIncomingCount("")
	require.NoError(t, err)

	require.NotEqual(t, 0, c, "should have at least 1 incoming hook")
}

func testWebhookStoreCountOutgoing(t *testing.T, ss store.Store) {
	o1 := &model.OutgoingWebhook{}
	o1.ChannelID = model.NewID()
	o1.CreatorID = model.NewID()
	o1.TeamID = model.NewID()
	o1.CallbackURLs = []string{"http://nowhere.com/"}

	_, err := ss.Webhook().SaveOutgoing(o1)
	require.NoError(t, err)

	r, err := ss.Webhook().AnalyticsOutgoingCount("")
	require.NoError(t, err)
	require.NotEqual(t, 0, r, "should have at least 1 outgoing hook")
}
