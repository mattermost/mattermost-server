// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
	"github.com/mattermost/mattermost-server/v5/utils/testutils"
)

func TestCreateBot(t *testing.T) {
	t.Run("create bot without permissions", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		_, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})

		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})

	t.Run("create bot without config permissions", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.Config().ServiceSettings.EnableBotAccountCreation = model.NewBool(false)

		_, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})

		CheckErrorMessage(t, resp, "api.bot.create_disabled")
	})

	t.Run("create bot with permissions", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		}

		createdBot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)
		require.Equal(t, bot.Username, createdBot.Username)
		require.Equal(t, bot.DisplayName, createdBot.DisplayName)
		require.Equal(t, bot.Description, createdBot.Description)
		require.Equal(t, th.BasicUser.ID, createdBot.OwnerID)
	})

	t.Run("create invalid bot", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		_, resp := th.Client.CreateBot(&model.Bot{
			Username:    "username",
			DisplayName: "a bot",
			Description: strings.Repeat("x", 1025),
		})

		CheckErrorMessage(t, resp, "model.bot.is_valid.description.app_error")
	})

	t.Run("bot attempt to create bot fails", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserAccessTokens = true })
		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionEditOtherUsers.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID+" "+model.SystemUserAccessTokenRoleID, false)

		bot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(bot.UserID)
		th.App.UpdateUserRoles(bot.UserID, model.TeamUserRoleID+" "+model.SystemUserAccessTokenRoleID, false)

		rtoken, resp := th.Client.CreateUserAccessToken(bot.UserID, "test token")
		CheckNoError(t, resp)
		th.Client.AuthToken = rtoken.Token

		_, resp = th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			OwnerID:     bot.UserID,
			DisplayName: "a bot2",
			Description: "bot2",
		})
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})

}

func TestPatchBot(t *testing.T) {
	t.Run("patch non-existent bot", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.PatchBot(model.NewID(), &model.BotPatch{})
			CheckNotFoundStatus(t, resp)
		})
	})

	t.Run("system admin and local client can patch any bot", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot created by a user",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			botPatch := &model.BotPatch{
				Username:    sToP(GenerateTestUsername()),
				DisplayName: sToP("an updated bot"),
				Description: sToP("updated bot"),
			}
			patchedBot, patchResp := client.PatchBot(createdBot.UserID, botPatch)
			CheckOKStatus(t, patchResp)
			require.Equal(t, *botPatch.Username, patchedBot.Username)
			require.Equal(t, *botPatch.DisplayName, patchedBot.DisplayName)
			require.Equal(t, *botPatch.Description, patchedBot.Description)
			require.Equal(t, th.BasicUser.ID, patchedBot.OwnerID)
		}, "bot created by user")

		createdBotSystemAdmin, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "another bot",
			Description: "bot created by system admin user",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBotSystemAdmin.UserID)

		th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
			botPatch := &model.BotPatch{
				Username:    sToP(GenerateTestUsername()),
				DisplayName: sToP("an updated bot"),
				Description: sToP("updated bot"),
			}
			patchedBot, patchResp := client.PatchBot(createdBotSystemAdmin.UserID, botPatch)
			CheckOKStatus(t, patchResp)
			require.Equal(t, *botPatch.Username, patchedBot.Username)
			require.Equal(t, *botPatch.DisplayName, patchedBot.DisplayName)
			require.Equal(t, *botPatch.Description, patchedBot.Description)
			require.Equal(t, th.SystemAdminUser.ID, patchedBot.OwnerID)
		}, "bot created by system admin")
	})

	t.Run("patch someone else's bot without permission", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		_, resp = th.Client.PatchBot(createdBot.UserID, &model.BotPatch{})
		CheckErrorMessage(t, resp, "store.sql_bot.get.missing.app_error")
	})

	t.Run("patch someone else's bot without permission, but with read others permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		_, resp = th.Client.PatchBot(createdBot.UserID, &model.BotPatch{})
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})

	t.Run("patch someone else's bot with permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionManageOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		botPatch := &model.BotPatch{
			Username:    sToP(GenerateTestUsername()),
			DisplayName: sToP("an updated bot"),
			Description: sToP("updated bot"),
		}

		patchedBot, resp := th.Client.PatchBot(createdBot.UserID, botPatch)
		CheckOKStatus(t, resp)
		require.Equal(t, *botPatch.Username, patchedBot.Username)
		require.Equal(t, *botPatch.DisplayName, patchedBot.DisplayName)
		require.Equal(t, *botPatch.Description, patchedBot.Description)
		require.Equal(t, th.SystemAdminUser.ID, patchedBot.OwnerID)

		// Continue through the bot update process (call UpdateUserRoles), then
		// get the bot, to make sure the patched bot was correctly saved.
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageRoles.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		success, resp := th.Client.UpdateUserRoles(createdBot.UserID, model.SystemUserRoleID)
		CheckOKStatus(t, resp)
		require.True(t, success)

		bots, resp := th.Client.GetBots(0, 2, "")
		CheckOKStatus(t, resp)
		require.Len(t, bots, 1)
		require.Equal(t, []*model.Bot{patchedBot}, bots)
	})

	t.Run("patch my bot without permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		botPatch := &model.BotPatch{
			Username:    sToP(GenerateTestUsername()),
			DisplayName: sToP("an updated bot"),
			Description: sToP("updated bot"),
		}

		_, resp = th.Client.PatchBot(createdBot.UserID, botPatch)
		CheckErrorMessage(t, resp, "store.sql_bot.get.missing.app_error")
	})

	t.Run("patch my bot without permission, but with read permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		botPatch := &model.BotPatch{
			Username:    sToP(GenerateTestUsername()),
			DisplayName: sToP("an updated bot"),
			Description: sToP("updated bot"),
		}

		_, resp = th.Client.PatchBot(createdBot.UserID, botPatch)
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})

	t.Run("patch my bot with permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		botPatch := &model.BotPatch{
			Username:    sToP(GenerateTestUsername()),
			DisplayName: sToP("an updated bot"),
			Description: sToP("updated bot"),
		}

		patchedBot, resp := th.Client.PatchBot(createdBot.UserID, botPatch)
		CheckOKStatus(t, resp)
		require.Equal(t, *botPatch.Username, patchedBot.Username)
		require.Equal(t, *botPatch.DisplayName, patchedBot.DisplayName)
		require.Equal(t, *botPatch.Description, patchedBot.Description)
		require.Equal(t, th.BasicUser.ID, patchedBot.OwnerID)
	})

	t.Run("partial patch my bot with permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		}

		createdBot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		botPatch := &model.BotPatch{
			Username: sToP(GenerateTestUsername()),
		}

		patchedBot, resp := th.Client.PatchBot(createdBot.UserID, botPatch)
		CheckOKStatus(t, resp)
		require.Equal(t, *botPatch.Username, patchedBot.Username)
		require.Equal(t, bot.DisplayName, patchedBot.DisplayName)
		require.Equal(t, bot.Description, patchedBot.Description)
		require.Equal(t, th.BasicUser.ID, patchedBot.OwnerID)
	})

	t.Run("update bot, internally managed fields ignored", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		createdBot, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		r, err := th.Client.DoAPIPut(th.Client.GetBotRoute(createdBot.UserID), `{"creator_id":"`+th.BasicUser2.ID+`"}`)
		require.Nil(t, err)
		defer func() {
			_, _ = ioutil.ReadAll(r.Body)
			_ = r.Body.Close()
		}()
		patchedBot := model.BotFromJSON(r.Body)
		resp = model.BuildResponse(r)
		CheckOKStatus(t, resp)

		require.Equal(t, th.BasicUser.ID, patchedBot.OwnerID)
	})
}

func TestGetBot(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	bot1, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "a bot",
		Description: "the first bot",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot1.UserID)

	bot2, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "another bot",
		Description: "the second bot",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot2.UserID)

	deletedBot, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		Description: "a deleted bot",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(deletedBot.UserID)
	deletedBot, resp = th.SystemAdminClient.DisableBot(deletedBot.UserID)
	CheckOKStatus(t, resp)

	th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
	th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	myBot, resp := th.Client.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "my bot",
		Description: "a bot created by non-admin",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(myBot.UserID)
	th.RemovePermissionFromRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)

	t.Run("get unknown bot", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		_, resp := th.Client.GetBot(model.NewID(), "")
		CheckNotFoundStatus(t, resp)
	})

	t.Run("get bot1", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		bot, resp := th.Client.GetBot(bot1.UserID, "")
		CheckOKStatus(t, resp)
		require.Equal(t, bot1, bot)

		bot, resp = th.Client.GetBot(bot1.UserID, bot.Etag())
		CheckEtag(t, bot, resp)
	})

	t.Run("get bot2", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		bot, resp := th.Client.GetBot(bot2.UserID, "")
		CheckOKStatus(t, resp)
		require.Equal(t, bot2, bot)

		bot, resp = th.Client.GetBot(bot2.UserID, bot.Etag())
		CheckEtag(t, bot, resp)
	})

	t.Run("get bot1 without PermissionReadOthersBots permission", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		_, resp := th.Client.GetBot(bot1.UserID, "")
		CheckErrorMessage(t, resp, "store.sql_bot.get.missing.app_error")
	})

	t.Run("get myBot without ReadBots OR ReadOthersBots permissions", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		_, resp := th.Client.GetBot(myBot.UserID, "")
		CheckErrorMessage(t, resp, "store.sql_bot.get.missing.app_error")
	})

	t.Run("get deleted bot", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		_, resp := th.Client.GetBot(deletedBot.UserID, "")
		CheckNotFoundStatus(t, resp)
	})

	t.Run("get deleted bot, include deleted", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		bot, resp := th.Client.GetBotIncludeDeleted(deletedBot.UserID, "")
		CheckOKStatus(t, resp)
		require.NotEqual(t, 0, bot.DeleteAt)
		deletedBot.UpdateAt = bot.UpdateAt
		deletedBot.DeleteAt = bot.DeleteAt
		require.Equal(t, deletedBot, bot)

		bot, resp = th.Client.GetBotIncludeDeleted(deletedBot.UserID, bot.Etag())
		CheckEtag(t, bot, resp)
	})
}

func TestGetBots(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	bot1, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "a bot",
		Description: "the first bot",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot1.UserID)

	deletedBot1, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		Description: "a deleted bot",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(deletedBot1.UserID)
	deletedBot1, resp = th.SystemAdminClient.DisableBot(deletedBot1.UserID)
	CheckOKStatus(t, resp)

	bot2, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "another bot",
		Description: "the second bot",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot2.UserID)

	bot3, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "another bot",
		Description: "the third bot",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot3.UserID)

	deletedBot2, resp := th.SystemAdminClient.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		Description: "a deleted bot",
	})
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(deletedBot2.UserID)
	deletedBot2, resp = th.SystemAdminClient.DisableBot(deletedBot2.UserID)
	CheckOKStatus(t, resp)

	th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
	th.App.UpdateUserRoles(th.BasicUser2.ID, model.TeamUserRoleID, false)
	th.LoginBasic2()
	orphanedBot, resp := th.Client.CreateBot(&model.Bot{
		Username:    GenerateTestUsername(),
		Description: "an oprphaned bot",
	})
	CheckCreatedStatus(t, resp)
	th.LoginBasic()
	defer th.App.PermanentDeleteBot(orphanedBot.UserID)
	// Automatic deactivation disabled
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.DisableBotsWhenOwnerIsDeactivated = false
	})
	_, resp = th.SystemAdminClient.DeleteUser(th.BasicUser2.ID)
	CheckOKStatus(t, resp)

	t.Run("get bots, page=0, perPage=10", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{bot1, bot2, bot3, orphanedBot}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBots(0, 10, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBots(0, 10, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots, page=0, perPage=1", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{bot1}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBots(0, 1, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBots(0, 1, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots, page=1, perPage=2", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{bot3, orphanedBot}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBots(1, 2, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBots(1, 2, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots, page=2, perPage=2", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBots(2, 2, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBots(2, 2, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots, page=0, perPage=10, include deleted", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{bot1, deletedBot1, bot2, bot3, deletedBot2, orphanedBot}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBotsIncludeDeleted(0, 10, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBotsIncludeDeleted(0, 10, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots, page=0, perPage=1, include deleted", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{bot1}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBotsIncludeDeleted(0, 1, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBotsIncludeDeleted(0, 1, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots, page=1, perPage=2, include deleted", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{bot2, bot3}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBotsIncludeDeleted(1, 2, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBotsIncludeDeleted(1, 2, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots, page=2, perPage=2, include deleted", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{deletedBot2, orphanedBot}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBotsIncludeDeleted(2, 2, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBotsIncludeDeleted(2, 2, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots, page=0, perPage=10, only orphaned", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		expectedBotList := []*model.Bot{orphanedBot}
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bots, resp := client.GetBotsOrphaned(0, 10, "")
			CheckOKStatus(t, resp)
			require.Equal(t, expectedBotList, bots)
		})

		botList := model.BotList(expectedBotList)
		bots, resp := th.Client.GetBotsOrphaned(0, 10, botList.Etag())
		CheckEtag(t, bots, resp)
	})

	t.Run("get bots without permission", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageOthersBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)

		_, resp := th.Client.GetBots(0, 10, "")
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})
}

func TestDisableBot(t *testing.T) {
	t.Run("disable non-existent bot", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.DisableBot(model.NewID())
			CheckNotFoundStatus(t, resp)
		})
	})

	t.Run("disable bot without permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}

		createdBot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		_, resp = th.Client.DisableBot(createdBot.UserID)
		CheckErrorMessage(t, resp, "store.sql_bot.get.missing.app_error")
	})

	t.Run("disable bot without permission, but with read permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}

		createdBot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		_, resp = th.Client.DisableBot(createdBot.UserID)
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})

	t.Run("disable bot with permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bot, resp := th.Client.CreateBot(&model.Bot{
				Username:    GenerateTestUsername(),
				Description: "bot",
			})
			CheckCreatedStatus(t, resp)
			defer th.App.PermanentDeleteBot(bot.UserID)

			disabledBot, resp := client.DisableBot(bot.UserID)
			CheckOKStatus(t, resp)
			bot.UpdateAt = disabledBot.UpdateAt
			bot.DeleteAt = disabledBot.DeleteAt
			require.Equal(t, bot, disabledBot)

			// Check bot disabled
			disab, resp := th.SystemAdminClient.GetBotIncludeDeleted(bot.UserID, "")
			CheckOKStatus(t, resp)
			require.NotZero(t, disab.DeleteAt)

			// Disabling should be idempotent.
			disabledBot2, resp := client.DisableBot(bot.UserID)
			CheckOKStatus(t, resp)
			require.Equal(t, bot, disabledBot2)
		})
	})
}
func TestEnableBot(t *testing.T) {
	t.Run("enable non-existent bot", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			_, resp := th.Client.EnableBot(model.NewID())
			CheckNotFoundStatus(t, resp)
		})
	})

	t.Run("enable bot without permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}

		createdBot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		_, resp = th.SystemAdminClient.DisableBot(createdBot.UserID)
		CheckOKStatus(t, resp)

		_, resp = th.Client.EnableBot(createdBot.UserID)
		CheckErrorMessage(t, resp, "store.sql_bot.get.missing.app_error")
	})

	t.Run("enable bot without permission, but with read permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}

		createdBot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		_, resp = th.SystemAdminClient.DisableBot(createdBot.UserID)
		CheckOKStatus(t, resp)

		_, resp = th.Client.EnableBot(createdBot.UserID)
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")
	})

	t.Run("enable bot with permission", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.TeamUserRoleID)
		th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			bot, resp := th.Client.CreateBot(&model.Bot{
				Username:    GenerateTestUsername(),
				Description: "bot",
			})
			CheckCreatedStatus(t, resp)
			defer th.App.PermanentDeleteBot(bot.UserID)

			_, resp = th.SystemAdminClient.DisableBot(bot.UserID)
			CheckOKStatus(t, resp)

			enabledBot1, resp := client.EnableBot(bot.UserID)
			CheckOKStatus(t, resp)
			bot.UpdateAt = enabledBot1.UpdateAt
			bot.DeleteAt = enabledBot1.DeleteAt
			require.Equal(t, bot, enabledBot1)

			// Check bot enabled
			enab, resp := th.SystemAdminClient.GetBotIncludeDeleted(bot.UserID, "")
			CheckOKStatus(t, resp)
			require.Zero(t, enab.DeleteAt)

			// Disabling should be idempotent.
			enabledBot2, resp := client.EnableBot(bot.UserID)
			CheckOKStatus(t, resp)
			require.Equal(t, bot, enabledBot2)
		})
	})
}

func TestAssignBot(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("claim non-existent bot", func(t *testing.T) {
		th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
			_, resp := client.AssignBot(model.NewID(), model.NewID())
			CheckNotFoundStatus(t, resp)
		})
	})

	t.Run("system admin and local mode assign bot", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.SystemUserRoleID)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}
		bot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(bot.UserID)

		before, resp := th.Client.GetBot(bot.UserID, "")
		CheckOKStatus(t, resp)
		require.Equal(t, th.BasicUser.ID, before.OwnerID)

		_, resp = th.SystemAdminClient.AssignBot(bot.UserID, th.SystemAdminUser.ID)
		CheckOKStatus(t, resp)

		// Original owner doesn't have read others bots permission, therefore can't see bot anymore
		_, resp = th.Client.GetBot(bot.UserID, "")
		CheckNotFoundStatus(t, resp)

		// System admin can see creator ID has changed
		after, resp := th.SystemAdminClient.GetBot(bot.UserID, "")
		CheckOKStatus(t, resp)
		require.Equal(t, th.SystemAdminUser.ID, after.OwnerID)

		// Assign back to user without permissions to manage, using local mode
		_, resp = th.LocalClient.AssignBot(bot.UserID, th.BasicUser.ID)
		CheckOKStatus(t, resp)

		after, resp = th.SystemAdminClient.GetBot(bot.UserID, "")
		CheckOKStatus(t, resp)
		require.Equal(t, th.BasicUser.ID, after.OwnerID)
	})

	t.Run("random user assign bot", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.SystemUserRoleID)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}
		createdBot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(createdBot.UserID)

		th.LoginBasic2()

		// Without permission to read others bots it doesn't exist
		_, resp = th.Client.AssignBot(createdBot.UserID, th.BasicUser2.ID)
		CheckErrorMessage(t, resp, "store.sql_bot.get.missing.app_error")

		// With permissions to read we don't have permissions to modify
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.SystemUserRoleID)
		_, resp = th.Client.AssignBot(createdBot.UserID, th.BasicUser2.ID)
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")

		th.LoginBasic()
	})

	t.Run("delegated user assign bot", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.SystemUserRoleID)
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.EnableBotAccountCreation = true
		})

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}
		bot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(bot.UserID)

		// Simulate custom role by just changing the system user role
		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionManageOthersBots.ID, model.SystemUserRoleID)
		th.LoginBasic2()

		_, resp = th.Client.AssignBot(bot.UserID, th.BasicUser2.ID)
		CheckOKStatus(t, resp)

		after, resp := th.SystemAdminClient.GetBot(bot.UserID, "")
		CheckOKStatus(t, resp)
		require.Equal(t, th.BasicUser2.ID, after.OwnerID)
	})

	t.Run("bot assigned to bot fails", func(t *testing.T) {
		defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

		th.AddPermissionToRole(model.PermissionCreateBot.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionReadBots.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionReadOthersBots.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionManageBots.ID, model.SystemUserRoleID)
		th.AddPermissionToRole(model.PermissionManageOthersBots.ID, model.SystemUserRoleID)

		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}
		bot, resp := th.Client.CreateBot(bot)
		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(bot.UserID)

		bot2, resp := th.Client.CreateBot(&model.Bot{
			Username:    GenerateTestUsername(),
			DisplayName: "a bot",
			Description: "bot",
		})

		CheckCreatedStatus(t, resp)
		defer th.App.PermanentDeleteBot(bot2.UserID)

		_, resp = th.Client.AssignBot(bot.UserID, bot2.UserID)
		CheckErrorMessage(t, resp, "api.context.permissions.app_error")

	})
}

func TestSetBotIconImage(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	user := th.BasicUser

	defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

	th.AddPermissionToRole(model.PermissionCreateBot.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionManageBots.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionReadBots.ID, model.SystemUserRoleID)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	bot := &model.Bot{
		Username:    GenerateTestUsername(),
		Description: "bot",
	}
	bot, resp := th.Client.CreateBot(bot)
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot.UserID)

	badData, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	goodData, err := testutils.ReadTestFile("test.svg")
	require.NoError(t, err)

	// SetBotIconImage only allowed for bots
	_, resp = th.SystemAdminClient.SetBotIconImage(user.ID, goodData)
	CheckNotFoundStatus(t, resp)

	// png/jpg is not allowed
	ok, resp := th.Client.SetBotIconImage(bot.UserID, badData)
	require.False(t, ok, "Should return false, set icon image only allows svg")
	CheckBadRequestStatus(t, resp)

	ok, resp = th.Client.SetBotIconImage(model.NewID(), badData)
	require.False(t, ok, "Should return false, set icon image not allowed")
	CheckNotFoundStatus(t, resp)

	_, resp = th.Client.SetBotIconImage(bot.UserID, goodData)
	CheckNoError(t, resp)

	// status code returns either forbidden or unauthorized
	// note: forbidden is set as default at Client4.SetBotIconImage when request is terminated early by server
	th.Client.Logout()
	_, resp = th.Client.SetBotIconImage(bot.UserID, badData)
	if resp.StatusCode == http.StatusForbidden {
		CheckForbiddenStatus(t, resp)
	} else if resp.StatusCode == http.StatusUnauthorized {
		CheckUnauthorizedStatus(t, resp)
	} else {
		require.Fail(t, "Should have failed either forbidden or unauthorized")
	}

	_, resp = th.SystemAdminClient.SetBotIconImage(bot.UserID, goodData)
	CheckNoError(t, resp)

	fpath := fmt.Sprintf("/bots/%v/icon.svg", bot.UserID)
	actualData, appErr := th.App.ReadFile(fpath)
	require.Nil(t, appErr)
	require.NotNil(t, actualData)
	require.Equal(t, goodData, actualData)

	info := &model.FileInfo{Path: fpath}
	err = th.cleanupTestFile(info)
	require.NoError(t, err)
}

func TestGetBotIconImage(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

	th.AddPermissionToRole(model.PermissionCreateBot.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionManageBots.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionReadBots.ID, model.SystemUserRoleID)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	bot := &model.Bot{
		Username:    GenerateTestUsername(),
		Description: "bot",
	}
	bot, resp := th.Client.CreateBot(bot)
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot.UserID)

	// Get icon image for user with no icon
	data, resp := th.Client.GetBotIconImage(bot.UserID)
	CheckNotFoundStatus(t, resp)
	require.Equal(t, 0, len(data))

	// Set an icon image
	path, _ := fileutils.FindDir("tests")
	svgFile, fileErr := os.Open(filepath.Join(path, "test.svg"))
	require.NoError(t, fileErr)
	defer svgFile.Close()

	expectedData, err := ioutil.ReadAll(svgFile)
	require.NoError(t, err)

	svgFile.Seek(0, 0)
	fpath := fmt.Sprintf("/bots/%v/icon.svg", bot.UserID)
	_, appErr := th.App.WriteFile(svgFile, fpath)
	require.Nil(t, appErr)

	data, resp = th.Client.GetBotIconImage(bot.UserID)
	CheckNoError(t, resp)
	require.Equal(t, expectedData, data)

	_, resp = th.Client.GetBotIconImage("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = th.Client.GetBotIconImage(model.NewID())
	CheckNotFoundStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.GetBotIconImage(bot.UserID)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetBotIconImage(bot.UserID)
	CheckNoError(t, resp)

	info := &model.FileInfo{Path: "/bots/" + bot.UserID + "/icon.svg"}
	err = th.cleanupTestFile(info)
	require.NoError(t, err)
}

func TestDeleteBotIconImage(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	defer th.RestoreDefaultRolePermissions(th.SaveDefaultRolePermissions())

	th.AddPermissionToRole(model.PermissionCreateBot.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionManageBots.ID, model.SystemUserRoleID)
	th.AddPermissionToRole(model.PermissionReadBots.ID, model.SystemUserRoleID)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	bot := &model.Bot{
		Username:    GenerateTestUsername(),
		Description: "bot",
	}
	bot, resp := th.Client.CreateBot(bot)
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot.UserID)

	// Get icon image for user with no icon
	data, resp := th.Client.GetBotIconImage(bot.UserID)
	CheckNotFoundStatus(t, resp)
	require.Equal(t, 0, len(data))

	// Set an icon image
	svgData, err := testutils.ReadTestFile("test.svg")
	require.NoError(t, err)

	_, resp = th.Client.SetBotIconImage(bot.UserID, svgData)
	CheckNoError(t, resp)

	fpath := fmt.Sprintf("/bots/%v/icon.svg", bot.UserID)
	exists, appErr := th.App.FileExists(fpath)
	require.Nil(t, appErr)
	require.True(t, exists, "icon.svg needs to exist for the user")

	data, resp = th.Client.GetBotIconImage(bot.UserID)
	CheckNoError(t, resp)
	require.Equal(t, svgData, data)

	success, resp := th.Client.DeleteBotIconImage("junk")
	CheckBadRequestStatus(t, resp)
	require.False(t, success)

	success, resp = th.Client.DeleteBotIconImage(model.NewID())
	CheckNotFoundStatus(t, resp)
	require.False(t, success)

	success, resp = th.Client.DeleteBotIconImage(bot.UserID)
	CheckNoError(t, resp)
	require.True(t, success)

	th.Client.Logout()
	success, resp = th.Client.DeleteBotIconImage(bot.UserID)
	CheckUnauthorizedStatus(t, resp)
	require.False(t, success)

	exists, appErr = th.App.FileExists(fpath)
	require.Nil(t, appErr)
	require.False(t, exists, "icon.svg should not for the user")
}

func TestConvertBotToUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.AddPermissionToRole(model.PermissionCreateBot.ID, model.TeamUserRoleID)
	th.App.UpdateUserRoles(th.BasicUser.ID, model.TeamUserRoleID, false)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})

	bot := &model.Bot{
		Username:    GenerateTestUsername(),
		Description: "bot",
	}
	bot, resp := th.Client.CreateBot(bot)
	CheckCreatedStatus(t, resp)
	defer th.App.PermanentDeleteBot(bot.UserID)

	_, resp = th.Client.ConvertBotToUser(bot.UserID, &model.UserPatch{}, false)
	CheckBadRequestStatus(t, resp)

	user, resp := th.Client.ConvertBotToUser(bot.UserID, &model.UserPatch{Password: model.NewString("password")}, false)
	CheckForbiddenStatus(t, resp)
	require.Nil(t, user)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		bot := &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "bot",
		}
		bot, resp := th.SystemAdminClient.CreateBot(bot)
		CheckCreatedStatus(t, resp)

		user, resp := client.ConvertBotToUser(bot.UserID, &model.UserPatch{}, false)
		CheckBadRequestStatus(t, resp)
		require.Nil(t, user)

		user, resp = client.ConvertBotToUser(bot.UserID, &model.UserPatch{Password: model.NewString("password")}, false)
		CheckNoError(t, resp)
		require.NotNil(t, user)
		require.Equal(t, bot.UserID, user.ID)

		bot, resp = client.GetBot(bot.UserID, "")
		CheckNotFoundStatus(t, resp)

		bot = &model.Bot{
			Username:    GenerateTestUsername(),
			Description: "systemAdminBot",
		}
		bot, resp = th.SystemAdminClient.CreateBot(bot)
		CheckCreatedStatus(t, resp)

		user, resp = client.ConvertBotToUser(bot.UserID, &model.UserPatch{Password: model.NewString("password")}, true)
		CheckNoError(t, resp)
		require.NotNil(t, user)
		require.Equal(t, bot.UserID, user.ID)
		require.Contains(t, user.GetRoles(), model.SystemAdminRoleID)

		bot, resp = client.GetBot(bot.UserID, "")
		CheckNotFoundStatus(t, resp)
	})
}

func sToP(s string) *string {
	return &s
}
