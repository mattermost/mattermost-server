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

func makeBotWithUser(t *testing.T, ss store.Store, bot *model.Bot) (*model.Bot, *model.User) {
	user, err := ss.User().Save(model.UserFromBot(bot))
	require.NoError(t, err)

	bot.UserID = user.ID
	bot, nErr := ss.Bot().Save(bot)
	require.NoError(t, nErr)

	return bot, user
}

func TestBotStore(t *testing.T, ss store.Store, s SqlStore) {
	t.Run("Get", func(t *testing.T) { testBotStoreGet(t, ss, s) })
	t.Run("GetAll", func(t *testing.T) { testBotStoreGetAll(t, ss, s) })
	t.Run("Save", func(t *testing.T) { testBotStoreSave(t, ss) })
	t.Run("Update", func(t *testing.T) { testBotStoreUpdate(t, ss) })
	t.Run("PermanentDelete", func(t *testing.T) { testBotStorePermanentDelete(t, ss) })
}

func testBotStoreGet(t *testing.T, ss store.Store, s SqlStore) {
	deletedBot, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:       "deleted_bot",
		Description:    "A deleted bot",
		OwnerID:        model.NewID(),
		LastIconUpdate: model.GetMillis(),
	})
	deletedBot.DeleteAt = 1
	deletedBot, err := ss.Bot().Update(deletedBot)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(deletedBot.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(deletedBot.UserID)) }()

	permanentlyDeletedBot, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:       "permanently_deleted_bot",
		Description:    "A permanently deleted bot",
		OwnerID:        model.NewID(),
		LastIconUpdate: model.GetMillis(),
		DeleteAt:       0,
	})
	require.NoError(t, ss.Bot().PermanentDelete(permanentlyDeletedBot.UserID))
	defer func() { require.NoError(t, ss.User().PermanentDelete(permanentlyDeletedBot.UserID)) }()

	b1, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:       "b1",
		Description:    "The first bot",
		OwnerID:        model.NewID(),
		LastIconUpdate: model.GetMillis(),
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b1.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b1.UserID)) }()

	b2, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:       "b2",
		Description:    "The second bot",
		OwnerID:        model.NewID(),
		LastIconUpdate: 0,
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b2.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b2.UserID)) }()

	// Artificially set b2.LastIconUpdate to NULL to verify handling of same.
	_, sqlErr := s.GetMaster().Exec("UPDATE Bots SET LastIconUpdate = NULL WHERE UserId = '" + b2.UserID + "'")
	require.NoError(t, sqlErr)

	t.Run("get non-existent bot", func(t *testing.T) {
		_, err := ss.Bot().Get("unknown", false)
		require.Error(t, err)
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(err, &nfErr))
	})

	t.Run("get deleted bot", func(t *testing.T) {
		_, err := ss.Bot().Get(deletedBot.UserID, false)
		require.Error(t, err)
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(err, &nfErr))
	})

	t.Run("get deleted bot, include deleted", func(t *testing.T) {
		bot, err := ss.Bot().Get(deletedBot.UserID, true)
		require.NoError(t, err)
		require.Equal(t, deletedBot, bot)
	})

	t.Run("get permanently deleted bot", func(t *testing.T) {
		_, err := ss.Bot().Get(permanentlyDeletedBot.UserID, false)
		require.Error(t, err)
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(err, &nfErr))
	})

	t.Run("get bot 1", func(t *testing.T) {
		bot, err := ss.Bot().Get(b1.UserID, false)
		require.NoError(t, err)
		require.Equal(t, b1, bot)
	})

	t.Run("get bot 2", func(t *testing.T) {
		bot, err := ss.Bot().Get(b2.UserID, false)
		require.NoError(t, err)
		require.Equal(t, b2, bot)
	})
}

func testBotStoreGetAll(t *testing.T, ss store.Store, s SqlStore) {
	OwnerID1 := model.NewID()
	OwnerID2 := model.NewID()

	deletedBot, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:       "deleted_bot",
		Description:    "A deleted bot",
		OwnerID:        OwnerID1,
		LastIconUpdate: model.GetMillis(),
	})
	deletedBot.DeleteAt = 1
	deletedBot, err := ss.Bot().Update(deletedBot)
	require.NoError(t, err)
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(deletedBot.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(deletedBot.UserID)) }()

	permanentlyDeletedBot, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:       "permanently_deleted_bot",
		Description:    "A permanently deleted bot",
		OwnerID:        OwnerID1,
		LastIconUpdate: model.GetMillis(),
		DeleteAt:       0,
	})
	require.NoError(t, ss.Bot().PermanentDelete(permanentlyDeletedBot.UserID))
	defer func() { require.NoError(t, ss.User().PermanentDelete(permanentlyDeletedBot.UserID)) }()

	b1, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:       "b1",
		Description:    "The first bot",
		OwnerID:        OwnerID1,
		LastIconUpdate: model.GetMillis(),
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b1.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b1.UserID)) }()

	b2, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:       "b2",
		Description:    "The second bot",
		OwnerID:        OwnerID1,
		LastIconUpdate: 0,
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b2.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b2.UserID)) }()

	// Artificially set b2.LastIconUpdate to NULL to verify handling of same.
	_, sqlErr := s.GetMaster().Exec("UPDATE Bots SET LastIconUpdate = NULL WHERE UserId = '" + b2.UserID + "'")
	require.NoError(t, sqlErr)

	t.Run("get original bots", func(t *testing.T) {
		bot, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 10})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			b1,
			b2,
		}, bot)
	})

	b3, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:    "b3",
		Description: "The third bot",
		OwnerID:     OwnerID1,
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b3.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b3.UserID)) }()

	b4, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:    "b4",
		Description: "The fourth bot",
		OwnerID:     OwnerID2,
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b4.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b4.UserID)) }()

	deletedUser := model.User{
		Email:    MakeEmail(),
		Username: model.NewID(),
	}
	_, err1 := ss.User().Save(&deletedUser)
	require.NoError(t, err1, "couldn't save user")

	deletedUser.DeleteAt = model.GetMillis()
	_, err2 := ss.User().Update(&deletedUser, true)
	require.NoError(t, err2, "couldn't delete user")

	defer func() { require.NoError(t, ss.User().PermanentDelete(deletedUser.ID)) }()
	ob5, _ := makeBotWithUser(t, ss, &model.Bot{
		Username:    "ob5",
		Description: "Orphaned bot 5",
		OwnerID:     deletedUser.ID,
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b4.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b4.UserID)) }()

	t.Run("get newly created bot stoo", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 10})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			b1,
			b2,
			b3,
			b4,
			ob5,
		}, bots)
	})

	t.Run("get orphaned", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 10, OnlyOrphaned: true})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			ob5,
		}, bots)
	})

	t.Run("get page=0, per_page=2", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 2})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			b1,
			b2,
		}, bots)
	})

	t.Run("get page=1, limit=2", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 1, PerPage: 2})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			b3,
			b4,
		}, bots)
	})

	t.Run("get page=5, perpage=1000", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 5, PerPage: 1000})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{}, bots)
	})

	t.Run("get offset=0, limit=2, include deleted", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 2, IncludeDeleted: true})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			deletedBot,
			b1,
		}, bots)
	})

	t.Run("get offset=2, limit=2, include deleted", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 1, PerPage: 2, IncludeDeleted: true})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			b2,
			b3,
		}, bots)
	})

	t.Run("get offset=0, limit=10, creator id 1", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 10, OwnerID: OwnerID1})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			b1,
			b2,
			b3,
		}, bots)
	})

	t.Run("get offset=0, limit=10, creator id 2", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 10, OwnerID: OwnerID2})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			b4,
		}, bots)
	})

	t.Run("get offset=0, limit=10, include deleted, creator id 1", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 10, IncludeDeleted: true, OwnerID: OwnerID1})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			deletedBot,
			b1,
			b2,
			b3,
		}, bots)
	})

	t.Run("get offset=0, limit=10, include deleted, creator id 2", func(t *testing.T) {
		bots, err := ss.Bot().GetAll(&model.BotGetOptions{Page: 0, PerPage: 10, IncludeDeleted: true, OwnerID: OwnerID2})
		require.NoError(t, err)
		require.Equal(t, []*model.Bot{
			b4,
		}, bots)
	})
}

func testBotStoreSave(t *testing.T, ss store.Store) {
	t.Run("invalid bot", func(t *testing.T) {
		bot := &model.Bot{
			UserID:      model.NewID(),
			Username:    "invalid bot",
			Description: "description",
		}

		_, err := ss.Bot().Save(bot)
		require.Error(t, err)
		var appErr *model.AppError
		require.True(t, errors.As(err, &appErr))
		// require.Equal(t, "model.bot.is_valid.username.app_error", err.Id)
	})

	t.Run("normal bot", func(t *testing.T) {
		bot := &model.Bot{
			Username:    "normal_bot",
			Description: "description",
			OwnerID:     model.NewID(),
		}

		user, err := ss.User().Save(model.UserFromBot(bot))
		require.NoError(t, err)
		defer func() { require.NoError(t, ss.User().PermanentDelete(user.ID)) }()
		bot.UserID = user.ID

		returnedNewBot, nErr := ss.Bot().Save(bot)
		require.NoError(t, nErr)
		defer func() { require.NoError(t, ss.Bot().PermanentDelete(bot.UserID)) }()

		// Verify the returned bot matches the saved bot, modulo expected changes
		require.NotEqual(t, 0, returnedNewBot.CreateAt)
		require.NotEqual(t, 0, returnedNewBot.UpdateAt)
		require.Equal(t, returnedNewBot.CreateAt, returnedNewBot.UpdateAt)
		bot.UserID = returnedNewBot.UserID
		bot.CreateAt = returnedNewBot.CreateAt
		bot.UpdateAt = returnedNewBot.UpdateAt
		bot.DeleteAt = 0
		require.Equal(t, bot, returnedNewBot)

		// Verify the actual bot in the database matches the saved bot.
		actualNewBot, nErr := ss.Bot().Get(bot.UserID, false)
		require.NoError(t, nErr)
		require.Equal(t, bot, actualNewBot)
	})
}

func testBotStoreUpdate(t *testing.T, ss store.Store) {
	t.Run("invalid bot should fail to update", func(t *testing.T) {
		existingBot, _ := makeBotWithUser(t, ss, &model.Bot{
			Username: "existing_bot",
			OwnerID:  model.NewID(),
		})
		defer func() { require.NoError(t, ss.Bot().PermanentDelete(existingBot.UserID)) }()
		defer func() { require.NoError(t, ss.User().PermanentDelete(existingBot.UserID)) }()

		bot := existingBot.Clone()
		bot.Username = "invalid username"
		_, err := ss.Bot().Update(bot)
		require.Error(t, err)
		var appErr *model.AppError
		require.True(t, errors.As(err, &appErr))
		require.Equal(t, "model.bot.is_valid.username.app_error", appErr.ID)
	})

	t.Run("existing bot should update", func(t *testing.T) {
		existingBot, _ := makeBotWithUser(t, ss, &model.Bot{
			Username: "existing_bot",
			OwnerID:  model.NewID(),
		})
		defer func() { require.NoError(t, ss.Bot().PermanentDelete(existingBot.UserID)) }()
		defer func() { require.NoError(t, ss.User().PermanentDelete(existingBot.UserID)) }()

		bot := existingBot.Clone()
		bot.OwnerID = model.NewID()
		bot.Description = "updated description"
		bot.CreateAt = 999999       // Ignored
		bot.UpdateAt = 999999       // Ignored
		bot.LastIconUpdate = 100000 // Allowed
		bot.DeleteAt = 100000       // Allowed

		returnedBot, err := ss.Bot().Update(bot)
		require.NoError(t, err)

		// Verify the returned bot matches the updated bot, modulo expected timestamp changes
		require.Equal(t, existingBot.CreateAt, returnedBot.CreateAt)
		require.NotEqual(t, bot.UpdateAt, returnedBot.UpdateAt, "update should have advanced UpdateAt")
		require.True(t, returnedBot.UpdateAt > bot.UpdateAt, "update should have advanced UpdateAt")
		require.NotEqual(t, 99999, returnedBot.UpdateAt, "should have ignored user-provided UpdateAt")
		require.Equal(t, bot.LastIconUpdate, returnedBot.LastIconUpdate, "should have marked icon as updated")
		require.Equal(t, bot.DeleteAt, returnedBot.DeleteAt, "should have marked bot as deleted")
		bot.CreateAt = returnedBot.CreateAt
		bot.UpdateAt = returnedBot.UpdateAt

		// Verify the actual (now deleted) bot in the database
		actualBot, err := ss.Bot().Get(bot.UserID, true)
		require.NoError(t, err)
		require.Equal(t, bot, actualBot)
	})

	t.Run("deleted bot should update, restoring", func(t *testing.T) {
		existingBot, _ := makeBotWithUser(t, ss, &model.Bot{
			Username: "existing_bot",
			OwnerID:  model.NewID(),
		})
		defer func() { require.NoError(t, ss.Bot().PermanentDelete(existingBot.UserID)) }()
		defer func() { require.NoError(t, ss.User().PermanentDelete(existingBot.UserID)) }()

		existingBot.DeleteAt = 100000
		existingBot, err := ss.Bot().Update(existingBot)
		require.NoError(t, err)

		bot := existingBot.Clone()
		bot.DeleteAt = 0

		returnedBot, err := ss.Bot().Update(bot)
		require.NoError(t, err)

		// Verify the returned bot matches the updated bot, modulo expected timestamp changes
		require.EqualValues(t, 0, returnedBot.DeleteAt)
		bot.UpdateAt = returnedBot.UpdateAt

		// Verify the actual bot in the database
		actualBot, err := ss.Bot().Get(bot.UserID, false)
		require.NoError(t, err)
		require.Equal(t, bot, actualBot)
	})
}

func testBotStorePermanentDelete(t *testing.T, ss store.Store) {
	b1, _ := makeBotWithUser(t, ss, &model.Bot{
		Username: "b1",
		OwnerID:  model.NewID(),
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b1.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b1.UserID)) }()

	b2, _ := makeBotWithUser(t, ss, &model.Bot{
		Username: "b2",
		OwnerID:  model.NewID(),
	})
	defer func() { require.NoError(t, ss.Bot().PermanentDelete(b2.UserID)) }()
	defer func() { require.NoError(t, ss.User().PermanentDelete(b2.UserID)) }()

	t.Run("permanently delete a non-existent bot", func(t *testing.T) {
		err := ss.Bot().PermanentDelete("unknown")
		require.NoError(t, err)
	})

	t.Run("permanently delete bot", func(t *testing.T) {
		err := ss.Bot().PermanentDelete(b1.UserID)
		require.NoError(t, err)

		_, err = ss.Bot().Get(b1.UserID, false)
		require.Error(t, err)
		var nfErr *store.ErrNotFound
		require.True(t, errors.As(err, &nfErr))
	})
}
