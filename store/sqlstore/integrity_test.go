// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func createAudit(ss store.Store, userID, sessionID string) *model.Audit {
	audit := model.Audit{
		UserID:    userID,
		SessionID: sessionID,
		IDAddress: "ipaddress",
		Action:    "Action",
	}
	ss.Audit().Save(&audit)
	return &audit
}

func createChannel(ss store.Store, teamID, creatorID string) *model.Channel {
	m := model.Channel{}
	m.TeamID = teamID
	m.CreatorID = creatorID
	m.DisplayName = "Name"
	m.Name = "zz" + model.NewID() + "b"
	m.Type = model.ChannelTypeOpen
	c, _ := ss.Channel().Save(&m, -1)
	return c
}

func createChannelWithSchemeID(ss store.Store, schemeID *string) *model.Channel {
	m := model.Channel{}
	m.SchemeID = schemeID
	m.TeamID = model.NewID()
	m.CreatorID = model.NewID()
	m.DisplayName = "Name"
	m.Name = "zz" + model.NewID() + "b"
	m.Type = model.ChannelTypeOpen
	c, _ := ss.Channel().Save(&m, -1)
	return c
}

func createCommand(ss store.Store, userID, teamID string) *model.Command {
	m := model.Command{}
	m.CreatorID = userID
	m.Method = model.CommandMethodPost
	m.TeamID = teamID
	m.URL = "http://nowhere.com/"
	m.Trigger = "trigger"
	cmd, _ := ss.Command().Save(&m)
	return cmd
}

func createChannelMember(ss store.Store, channelID, userID string) *model.ChannelMember {
	m := model.ChannelMember{}
	m.ChannelID = channelID
	m.UserID = userID
	m.NotifyProps = model.GetDefaultChannelNotifyProps()
	cm, _ := ss.Channel().SaveMember(&m)
	return cm
}

func createChannelMemberHistory(ss store.Store, channelID, userID string) *model.ChannelMemberHistory {
	m := model.ChannelMemberHistory{}
	m.ChannelID = channelID
	m.UserID = userID
	ss.ChannelMemberHistory().LogJoinEvent(userID, channelID, model.GetMillis())
	return &m
}

func createChannelWithTeamID(ss store.Store, id string) *model.Channel {
	return createChannel(ss, id, model.NewID())
}

func createChannelWithCreatorID(ss store.Store, id string) *model.Channel {
	return createChannel(ss, model.NewID(), id)
}

func createChannelMemberWithChannelID(ss store.Store, id string) *model.ChannelMember {
	return createChannelMember(ss, id, model.NewID())
}

func createCommandWebhook(ss store.Store, commandID, userID, channelID string) *model.CommandWebhook {
	m := model.CommandWebhook{}
	m.CommandID = commandID
	m.UserID = userID
	m.ChannelID = channelID
	cwh, _ := ss.CommandWebhook().Save(&m)
	return cwh
}

func createCompliance(ss store.Store, userID string) *model.Compliance {
	m := model.Compliance{}
	m.UserID = userID
	m.Desc = "Audit"
	m.Status = model.ComplianceStatusFailed
	m.StartAt = model.GetMillis() - 1
	m.EndAt = model.GetMillis() + 1
	m.Type = model.ComplianceTypeAdhoc
	c, _ := ss.Compliance().Save(&m)
	return c
}

func createEmoji(ss store.Store, userID string) *model.Emoji {
	m := model.Emoji{}
	m.CreatorID = userID
	m.Name = "emoji"
	emoji, _ := ss.Emoji().Save(&m)
	return emoji
}

func createFileInfo(ss store.Store, postID, userID string) *model.FileInfo {
	m := model.FileInfo{}
	m.PostID = postID
	m.CreatorID = userID
	m.Path = "some/path/to/file"
	info, _ := ss.FileInfo().Save(&m)
	return info
}

func createIncomingWebhook(ss store.Store, userID, channelID, teamID string) *model.IncomingWebhook {
	m := model.IncomingWebhook{}
	m.UserID = userID
	m.ChannelID = channelID
	m.TeamID = teamID
	wh, _ := ss.Webhook().SaveIncoming(&m)
	return wh
}

func createOAuthAccessData(ss store.Store, userID string) *model.AccessData {
	m := model.AccessData{}
	m.ClientID = model.NewID()
	m.UserID = userID
	m.Token = model.NewID()
	m.RefreshToken = model.NewID()
	m.RedirectURI = "http://example.com"
	ad, _ := ss.OAuth().SaveAccessData(&m)
	return ad
}

func createOAuthApp(ss store.Store, userID string) *model.OAuthApp {
	m := model.OAuthApp{}
	m.CreatorID = userID
	m.CallbackURLs = []string{"https://nowhere.com"}
	m.Homepage = "https://nowhere.com"
	m.ID = ""
	m.Name = "TestApp" + model.NewID()
	app, _ := ss.OAuth().SaveApp(&m)
	return app
}

func createOAuthAuthData(ss store.Store, userID string) *model.AuthData {
	m := model.AuthData{}
	m.ClientID = model.NewID()
	m.UserID = userID
	m.Code = model.NewID()
	m.RedirectURI = "http://example.com"
	ad, _ := ss.OAuth().SaveAuthData(&m)
	return ad
}

func createOutgoingWebhook(ss store.Store, userID, channelID, teamID string) *model.OutgoingWebhook {
	m := model.OutgoingWebhook{}
	m.CreatorID = userID
	m.ChannelID = channelID
	m.TeamID = teamID
	m.Token = model.NewID()
	m.CallbackURLs = []string{"http://nowhere.com/"}
	wh, _ := ss.Webhook().SaveOutgoing(&m)
	return wh
}

func createPost(ss store.Store, channelID, userID, rootID, parentID string) *model.Post {
	m := model.Post{}
	m.ChannelID = channelID
	m.UserID = userID
	m.RootID = rootID
	m.ParentID = parentID
	m.Message = "zz" + model.NewID() + "b"
	p, _ := ss.Post().Save(&m)
	return p
}

func createPostWithChannelID(ss store.Store, id string) *model.Post {
	return createPost(ss, id, model.NewID(), "", "")
}

func createPostWithUserID(ss store.Store, id string) *model.Post {
	return createPost(ss, model.NewID(), id, "", "")
}

func createPreferences(ss store.Store, userID string) *model.Preferences {
	preferences := model.Preferences{
		{
			UserID:   userID,
			Name:     model.NewID(),
			Category: model.PreferenceCategoryDirectChannelShow,
			Value:    "somevalue",
		},
	}
	ss.Preference().Save(&preferences)
	return &preferences
}

func createReaction(ss store.Store, userID, postID string) *model.Reaction {
	reaction := &model.Reaction{
		UserID:    userID,
		PostID:    postID,
		EmojiName: model.NewID(),
	}
	reaction, _ = ss.Reaction().Save(reaction)
	return reaction
}

func createDefaultRoles(ss store.Store) {
	ss.Role().Save(&model.Role{
		Name:        model.TeamAdminRoleID,
		DisplayName: model.TeamAdminRoleID,
		Permissions: []string{
			model.PermissionDeleteOthersPosts.ID,
		},
	})

	ss.Role().Save(&model.Role{
		Name:        model.TeamUserRoleID,
		DisplayName: model.TeamUserRoleID,
		Permissions: []string{
			model.PermissionViewTeam.ID,
			model.PermissionAddUserToTeam.ID,
		},
	})

	ss.Role().Save(&model.Role{
		Name:        model.TeamGuestRoleID,
		DisplayName: model.TeamGuestRoleID,
		Permissions: []string{
			model.PermissionViewTeam.ID,
		},
	})

	ss.Role().Save(&model.Role{
		Name:        model.ChannelAdminRoleID,
		DisplayName: model.ChannelAdminRoleID,
		Permissions: []string{
			model.PermissionManagePublicChannelMembers.ID,
			model.PermissionManagePrivateChannelMembers.ID,
		},
	})

	ss.Role().Save(&model.Role{
		Name:        model.ChannelUserRoleID,
		DisplayName: model.ChannelUserRoleID,
		Permissions: []string{
			model.PermissionReadChannel.ID,
			model.PermissionCreatePost.ID,
		},
	})

	ss.Role().Save(&model.Role{
		Name:        model.ChannelGuestRoleID,
		DisplayName: model.ChannelGuestRoleID,
		Permissions: []string{
			model.PermissionReadChannel.ID,
			model.PermissionCreatePost.ID,
		},
	})
}

func createScheme(ss store.Store) *model.Scheme {
	m := model.Scheme{}
	m.DisplayName = model.NewID()
	m.Name = model.NewID()
	m.Description = model.NewID()
	m.Scope = model.SchemeScopeChannel
	s, _ := ss.Scheme().Save(&m)
	return s
}

func createSession(ss store.Store, userID string) *model.Session {
	m := model.Session{}
	m.UserID = userID
	s, _ := ss.Session().Save(&m)
	return s
}

func createStatus(ss store.Store, userID string) *model.Status {
	m := model.Status{}
	m.UserID = userID
	m.Status = model.StatusOnline
	ss.Status().SaveOrUpdate(&m)
	return &m
}

func createTeam(ss store.Store) *model.Team {
	m := model.Team{}
	m.DisplayName = "DisplayName"
	m.Type = model.TeamOpen
	m.Email = "test@example.com"
	m.Name = "z-z-z" + model.NewRandomTeamName() + "b"
	t, _ := ss.Team().Save(&m)
	return t
}

func createTeamMember(ss store.Store, teamID, userID string) *model.TeamMember {
	m := model.TeamMember{}
	m.TeamID = teamID
	m.UserID = userID
	tm, _ := ss.Team().SaveMember(&m, -1)
	return tm
}

func createTeamWithSchemeID(ss store.Store, schemeID *string) *model.Team {
	m := model.Team{}
	m.SchemeID = schemeID
	m.DisplayName = "DisplayName"
	m.Type = model.TeamOpen
	m.Email = "test@example.com"
	m.Name = "z-z-z" + model.NewID() + "b"
	t, _ := ss.Team().Save(&m)
	return t
}

func createUser(ss store.Store) *model.User {
	m := model.User{}
	m.Username = model.NewID()
	m.Email = m.Username + "@example.com"
	user, _ := ss.User().Save(&m)
	return user
}

func createUserAccessToken(ss store.Store, userID string) *model.UserAccessToken {
	m := model.UserAccessToken{}
	m.UserID = userID
	m.Token = model.NewID()
	uat, _ := ss.UserAccessToken().Save(&m)
	return uat
}

func TestCheckIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		ss.DropAllTables()
		t.Run("generate reports with no records", func(t *testing.T) {
			results := ss.CheckIntegrity()
			require.NotNil(t, results)
			for result := range results {
				require.IsType(t, model.IntegrityCheckResult{}, result)
				require.NoError(t, result.Err)
				switch data := result.Data.(type) {
				case model.RelationalIntegrityCheckData:
					require.Empty(t, data.Records)
				}
			}
		})
	})
}

func TestCheckParentChildIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		t.Run("should receive an error", func(t *testing.T) {
			config := relationalCheckConfig{
				parentName:   "NotValid",
				parentIDAttr: "NotValid",
				childName:    "NotValid",
				childIDAttr:  "NotValid",
			}
			result := checkParentChildIntegrity(store, config)
			require.Error(t, result.Err)
			require.Empty(t, result.Data)
		})
	})
}

func TestCheckChannelsCommandWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkChannelsCommandWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			channelID := model.NewID()
			cwh := createCommandWebhook(ss, model.NewID(), model.NewID(), channelID)
			result := checkChannelsCommandWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &channelID,
				ChildID:  &cwh.ID,
			}, data.Records[0])
			dbmap.Delete(cwh)
		})
	})
}

func TestCheckChannelsChannelMemberHistoryIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkChannelsChannelMemberHistoryIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			channel := createChannel(ss, model.NewID(), model.NewID())
			user := createUser(ss)
			cmh := createChannelMemberHistory(ss, channel.ID, user.ID)
			dbmap.Delete(channel)
			result := checkChannelsChannelMemberHistoryIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &cmh.ChannelID,
			}, data.Records[0])
			dbmap.Delete(user)
			dbmap.Exec(`DELETE FROM ChannelMemberHistory`)
		})
	})
}

func TestCheckChannelsChannelMembersIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkChannelsChannelMembersIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			channel := createChannel(ss, model.NewID(), model.NewID())
			member := createChannelMemberWithChannelID(ss, channel.ID)
			dbmap.Delete(channel)
			result := checkChannelsChannelMembersIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &member.ChannelID,
			}, data.Records[0])
			ss.Channel().PermanentDeleteMembersByChannel(member.ChannelID)
		})
	})
}

func TestCheckChannelsIncomingWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkChannelsIncomingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			channelID := model.NewID()
			wh := createIncomingWebhook(ss, model.NewID(), channelID, model.NewID())
			result := checkChannelsIncomingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &channelID,
				ChildID:  &wh.ID,
			}, data.Records[0])
			dbmap.Delete(wh)
		})
	})
}

func TestCheckChannelsOutgoingWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkChannelsOutgoingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			channel := createChannel(ss, model.NewID(), model.NewID())
			channelID := channel.ID
			wh := createOutgoingWebhook(ss, model.NewID(), channelID, model.NewID())
			dbmap.Delete(channel)
			result := checkChannelsOutgoingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &channelID,
				ChildID:  &wh.ID,
			}, data.Records[0])
			dbmap.Delete(wh)
		})
	})
}

func TestCheckChannelsPostsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkChannelsPostsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			post := createPostWithChannelID(ss, model.NewID())
			result := checkChannelsPostsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &post.ChannelID,
				ChildID:  &post.ID,
			}, data.Records[0])
			dbmap.Delete(post)
		})
	})
}

func TestCheckCommandsCommandWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkCommandsCommandWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			commandID := model.NewID()
			cwh := createCommandWebhook(ss, commandID, model.NewID(), model.NewID())
			result := checkCommandsCommandWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &commandID,
				ChildID:  &cwh.ID,
			}, data.Records[0])
			dbmap.Delete(cwh)
		})
	})
}

func TestCheckPostsFileInfoIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkPostsFileInfoIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			postID := model.NewID()
			info := createFileInfo(ss, postID, model.NewID())
			result := checkPostsFileInfoIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &postID,
				ChildID:  &info.ID,
			}, data.Records[0])
			dbmap.Delete(info)
		})
	})
}

func TestCheckPostsPostsParentIDIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkPostsPostsParentIDIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with no records", func(t *testing.T) {
			root := createPost(ss, model.NewID(), model.NewID(), "", "")
			parent := createPost(ss, model.NewID(), model.NewID(), root.ID, root.ID)
			post := createPost(ss, model.NewID(), model.NewID(), root.ID, parent.ID)
			result := checkPostsPostsParentIDIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
			dbmap.Delete(parent)
			dbmap.Delete(root)
			dbmap.Delete(post)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			root := createPost(ss, model.NewID(), model.NewID(), "", "")
			parent := createPost(ss, model.NewID(), model.NewID(), root.ID, root.ID)
			parentID := parent.ID
			post := createPost(ss, model.NewID(), model.NewID(), root.ID, parent.ID)
			dbmap.Delete(parent)
			result := checkPostsPostsParentIDIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &parentID,
				ChildID:  &post.ID,
			}, data.Records[0])
			dbmap.Delete(root)
			dbmap.Delete(post)
		})
	})
}

func TestCheckPostsPostsRootIDIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkPostsPostsRootIDIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			root := createPost(ss, model.NewID(), model.NewID(), "", "")
			rootID := root.ID
			post := createPost(ss, model.NewID(), model.NewID(), root.ID, root.ID)
			dbmap.Delete(root)
			result := checkPostsPostsRootIDIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &rootID,
				ChildID:  &post.ID,
			}, data.Records[0])
			dbmap.Delete(post)
		})
	})
}

func TestCheckPostsReactionsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkPostsReactionsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			postID := model.NewID()
			reaction := createReaction(ss, model.NewID(), postID)
			result := checkPostsReactionsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &postID,
			}, data.Records[0])
			dbmap.Delete(reaction)
		})
	})
}

func TestCheckSchemesChannelsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkSchemesChannelsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			createDefaultRoles(ss)
			scheme := createScheme(ss)
			schemeID := scheme.ID
			channel := createChannelWithSchemeID(ss, &schemeID)
			dbmap.Delete(scheme)
			result := checkSchemesChannelsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &schemeID,
				ChildID:  &channel.ID,
			}, data.Records[0])
			dbmap.Delete(channel)
		})
	})
}

func TestCheckSchemesTeamsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkSchemesTeamsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			createDefaultRoles(ss)
			scheme := createScheme(ss)
			schemeID := scheme.ID
			team := createTeamWithSchemeID(ss, &schemeID)
			dbmap.Delete(scheme)
			result := checkSchemesTeamsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &schemeID,
				ChildID:  &team.ID,
			}, data.Records[0])
			dbmap.Delete(team)
		})
	})
}

func TestCheckSessionsAuditsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkSessionsAuditsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			userID := model.NewID()
			session := createSession(ss, model.NewID())
			sessionID := session.ID
			audit := createAudit(ss, userID, sessionID)
			dbmap.Delete(session)
			result := checkSessionsAuditsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &sessionID,
				ChildID:  &audit.ID,
			}, data.Records[0])
			ss.Audit().PermanentDeleteByUser(userID)
		})
	})
}

func TestCheckTeamsChannelsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkTeamsChannelsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			channel := createChannelWithTeamID(ss, model.NewID())
			result := checkTeamsChannelsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &channel.TeamID,
				ChildID:  &channel.ID,
			}, data.Records[0])
			dbmap.Delete(channel)
		})

		t.Run("should not include direct channel with empty teamid", func(t *testing.T) {
			channel := createChannelWithTeamID(ss, model.NewID())
			userA := createUser(ss)
			userB := createUser(ss)
			direct, err := ss.Channel().CreateDirectChannel(userA, userB)
			require.NoError(t, err)
			require.NotNil(t, direct)
			result := checkTeamsChannelsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &channel.TeamID,
				ChildID:  &channel.ID,
			}, data.Records[0])
			dbmap.Delete(channel)
			dbmap.Delete(userA)
			dbmap.Delete(userB)
			dbmap.Delete(direct)
		})

		t.Run("should include direct channel with non empty teamid", func(t *testing.T) {
			channel := createChannelWithTeamID(ss, model.NewID())
			userA := createUser(ss)
			userB := createUser(ss)
			direct, err := ss.Channel().CreateDirectChannel(userA, userB)
			require.NoError(t, err)
			require.NotNil(t, direct)
			_, err = dbmap.Exec(`UPDATE Channels SET TeamId = 'test' WHERE Id = '` + direct.ID + `'`)
			require.NoError(t, err)
			result := checkTeamsChannelsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 2)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &channel.TeamID,
				ChildID:  &channel.ID,
			}, data.Records[0])
			require.Equal(t, model.OrphanedRecord{
				ParentID: model.NewString("test"),
				ChildID:  &direct.ID,
			}, data.Records[1])
			dbmap.Delete(channel)
			dbmap.Delete(userA)
			dbmap.Delete(userB)
			dbmap.Delete(direct)
			dbmap.Exec("DELETE FROM ChannelMembers")
		})
	})
}

func TestCheckTeamsCommandsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkTeamsCommandsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			teamID := model.NewID()
			cmd := createCommand(ss, model.NewID(), teamID)
			result := checkTeamsCommandsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &teamID,
				ChildID:  &cmd.ID,
			}, data.Records[0])
			dbmap.Delete(cmd)
		})
	})
}

func TestCheckTeamsIncomingWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkTeamsIncomingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			teamID := model.NewID()
			wh := createIncomingWebhook(ss, model.NewID(), model.NewID(), teamID)
			result := checkTeamsIncomingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &teamID,
				ChildID:  &wh.ID,
			}, data.Records[0])
			dbmap.Delete(wh)
		})
	})
}

func TestCheckTeamsOutgoingWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkTeamsOutgoingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			teamID := model.NewID()
			wh := createOutgoingWebhook(ss, model.NewID(), model.NewID(), teamID)
			result := checkTeamsOutgoingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &teamID,
				ChildID:  &wh.ID,
			}, data.Records[0])
			dbmap.Delete(wh)
		})
	})
}

func TestCheckTeamsTeamMembersIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkTeamsTeamMembersIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			team := createTeam(ss)
			member := createTeamMember(ss, team.ID, model.NewID())
			dbmap.Delete(team)
			result := checkTeamsTeamMembersIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &team.ID,
			}, data.Records[0])
			ss.Team().RemoveAllMembersByTeam(member.TeamID)
		})
	})
}

func TestCheckUsersAuditsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersAuditsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			audit := createAudit(ss, userID, model.NewID())
			dbmap.Delete(user)
			result := checkUsersAuditsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &audit.ID,
			}, data.Records[0])
			ss.Audit().PermanentDeleteByUser(userID)
		})
	})
}

func TestCheckUsersCommandWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersCommandWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			userID := model.NewID()
			cwh := createCommandWebhook(ss, model.NewID(), userID, model.NewID())
			result := checkUsersCommandWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &cwh.ID,
			}, data.Records[0])
			dbmap.Delete(cwh)
		})
	})
}

func TestCheckUsersChannelsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersChannelsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			channel := createChannelWithCreatorID(ss, model.NewID())
			result := checkUsersChannelsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &channel.CreatorID,
				ChildID:  &channel.ID,
			}, data.Records[0])
			dbmap.Delete(channel)
		})
	})
}

func TestCheckUsersChannelMemberHistoryIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersChannelMemberHistoryIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			channel := createChannel(ss, model.NewID(), model.NewID())
			cmh := createChannelMemberHistory(ss, channel.ID, user.ID)
			dbmap.Delete(user)
			result := checkUsersChannelMemberHistoryIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &cmh.UserID,
			}, data.Records[0])
			dbmap.Delete(channel)
			dbmap.Exec(`DELETE FROM ChannelMemberHistory`)
		})
	})
}

func TestCheckUsersChannelMembersIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersChannelMembersIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			channel := createChannelWithCreatorID(ss, user.ID)
			member := createChannelMember(ss, channel.ID, user.ID)
			dbmap.Delete(user)
			result := checkUsersChannelMembersIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &member.UserID,
			}, data.Records[0])
			dbmap.Delete(channel)
			ss.Channel().PermanentDeleteMembersByUser(member.UserID)
		})
	})
}

func TestCheckUsersCommandsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersCommandsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			userID := model.NewID()
			cmd := createCommand(ss, userID, model.NewID())
			result := checkUsersCommandsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &cmd.ID,
			}, data.Records[0])
			dbmap.Delete(cmd)
		})
	})
}

func TestCheckUsersCompliancesIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersCompliancesIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			compliance := createCompliance(ss, userID)
			dbmap.Delete(user)
			result := checkUsersCompliancesIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &compliance.ID,
			}, data.Records[0])
			dbmap.Delete(compliance)
		})
	})
}

func TestCheckUsersEmojiIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersEmojiIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			emoji := createEmoji(ss, userID)
			dbmap.Delete(user)
			result := checkUsersEmojiIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &emoji.ID,
			}, data.Records[0])
			dbmap.Delete(emoji)
		})
	})
}

func TestCheckUsersFileInfoIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersFileInfoIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			info := createFileInfo(ss, model.NewID(), userID)
			dbmap.Delete(user)
			result := checkUsersFileInfoIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &info.ID,
			}, data.Records[0])
			dbmap.Delete(info)
		})
	})
}

func TestCheckUsersIncomingWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersIncomingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			userID := model.NewID()
			wh := createIncomingWebhook(ss, userID, model.NewID(), model.NewID())
			result := checkUsersIncomingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &wh.ID,
			}, data.Records[0])
			dbmap.Delete(wh)
		})
	})
}

func TestCheckUsersOAuthAccessDataIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersOAuthAccessDataIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			ad := createOAuthAccessData(ss, userID)
			dbmap.Delete(user)
			result := checkUsersOAuthAccessDataIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &ad.Token,
			}, data.Records[0])
			ss.OAuth().RemoveAccessData(ad.Token)
		})
	})
}

func TestCheckUsersOAuthAppsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersOAuthAppsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			app := createOAuthApp(ss, userID)
			dbmap.Delete(user)
			result := checkUsersOAuthAppsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &app.ID,
			}, data.Records[0])
			ss.OAuth().DeleteApp(app.ID)
		})
	})
}

func TestCheckUsersOAuthAuthDataIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersOAuthAuthDataIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			ad := createOAuthAuthData(ss, userID)
			dbmap.Delete(user)
			result := checkUsersOAuthAuthDataIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &ad.Code,
			}, data.Records[0])
			ss.OAuth().RemoveAuthData(ad.Code)
		})
	})
}

func TestCheckUsersOutgoingWebhooksIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersOutgoingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			userID := model.NewID()
			wh := createOutgoingWebhook(ss, userID, model.NewID(), model.NewID())
			result := checkUsersOutgoingWebhooksIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &wh.ID,
			}, data.Records[0])
			dbmap.Delete(wh)
		})
	})
}

func TestCheckUsersPostsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersPostsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			post := createPostWithUserID(ss, model.NewID())
			result := checkUsersPostsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &post.UserID,
				ChildID:  &post.ID,
			}, data.Records[0])
			dbmap.Delete(post)
		})
	})
}

func TestCheckUsersPreferencesIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersPreferencesIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with no records", func(t *testing.T) {
			user := createUser(ss)
			require.NotNil(t, user)
			userID := user.ID
			preferences := createPreferences(ss, userID)
			require.NotNil(t, preferences)
			result := checkUsersPreferencesIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
			dbmap.Exec(`DELETE FROM Preferences`)
			dbmap.Delete(user)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			require.NotNil(t, user)
			userID := user.ID
			preferences := createPreferences(ss, userID)
			require.NotNil(t, preferences)
			dbmap.Delete(user)
			result := checkUsersPreferencesIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
			}, data.Records[0])
			dbmap.Exec(`DELETE FROM Preferences`)
			dbmap.Delete(user)
		})
	})
}

func TestCheckUsersReactionsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersReactionsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			reaction := createReaction(ss, user.ID, model.NewID())
			dbmap.Delete(user)
			result := checkUsersReactionsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
			}, data.Records[0])
			dbmap.Delete(reaction)
		})
	})
}

func TestCheckUsersSessionsIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersSessionsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			userID := model.NewID()
			session := createSession(ss, userID)
			result := checkUsersSessionsIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &session.ID,
			}, data.Records[0])
			dbmap.Delete(session)
		})
	})
}

func TestCheckUsersStatusIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersStatusIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			status := createStatus(ss, user.ID)
			dbmap.Delete(user)
			result := checkUsersStatusIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
			}, data.Records[0])
			dbmap.Delete(status)
		})
	})
}

func TestCheckUsersTeamMembersIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersTeamMembersIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			team := createTeam(ss)
			member := createTeamMember(ss, team.ID, user.ID)
			dbmap.Delete(user)
			result := checkUsersTeamMembersIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &member.UserID,
			}, data.Records[0])
			ss.Team().RemoveAllMembersByTeam(member.TeamID)
			dbmap.Delete(team)
		})
	})
}

func TestCheckUsersUserAccessTokensIntegrity(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		store := ss.(*SqlStore)
		dbmap := store.GetMaster()

		t.Run("should generate a report with no records", func(t *testing.T) {
			result := checkUsersUserAccessTokensIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Empty(t, data.Records)
		})

		t.Run("should generate a report with one record", func(t *testing.T) {
			user := createUser(ss)
			userID := user.ID
			uat := createUserAccessToken(ss, user.ID)
			dbmap.Delete(user)
			result := checkUsersUserAccessTokensIntegrity(store)
			require.NoError(t, result.Err)
			data := result.Data.(model.RelationalIntegrityCheckData)
			require.Len(t, data.Records, 1)
			require.Equal(t, model.OrphanedRecord{
				ParentID: &userID,
				ChildID:  &uat.ID,
			}, data.Records[0])
			ss.UserAccessToken().Delete(uat.ID)
		})
	})
}
