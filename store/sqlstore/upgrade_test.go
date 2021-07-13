// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestStoreUpgradeDotRelease(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		sqlStore := ss.(*SqlStore)
		saveSchemaVersion(sqlStore, "5.33.1")
		err := upgradeDatabase(sqlStore, CurrentSchemaVersion)
		require.NoError(t, err)
		require.Equal(t, CurrentSchemaVersion, sqlStore.GetCurrentSchemaVersion())
	})
}

func TestStoreUpgrade(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		sqlStore := ss.(*SqlStore)

		t.Run("invalid currentModelVersion", func(t *testing.T) {
			err := upgradeDatabase(sqlStore, "notaversion")
			require.EqualError(t, err, "failed to parse current model version notaversion: No Major.Minor.Patch elements found")
		})

		t.Run("upgrade from invalid version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, "invalid")
			err := upgradeDatabase(sqlStore, "5.8.0")
			require.EqualError(t, err, "failed to parse database schema version invalid: No Major.Minor.Patch elements found")
			require.Equal(t, "invalid", sqlStore.GetCurrentSchemaVersion())
		})

		t.Run("upgrade from unsupported version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, "2.0.0")
			err := upgradeDatabase(sqlStore, "5.8.0")
			require.EqualError(t, err, "Database schema version 2.0.0 is no longer supported. This Mattermost server supports automatic upgrades from schema version 3.0.0 through schema version 5.8.0. Please manually upgrade to at least version 3.0.0 before continuing.")
			require.Equal(t, "2.0.0", sqlStore.GetCurrentSchemaVersion())
		})

		t.Run("upgrade from earliest supported version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, Version300)
			err := upgradeDatabase(sqlStore, CurrentSchemaVersion)
			require.NoError(t, err)
			require.Equal(t, CurrentSchemaVersion, sqlStore.GetCurrentSchemaVersion())
		})

		t.Run("upgrade from no existing version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, "")
			err := upgradeDatabase(sqlStore, CurrentSchemaVersion)
			require.NoError(t, err)
			require.Equal(t, CurrentSchemaVersion, sqlStore.GetCurrentSchemaVersion())
		})

		t.Run("upgrade schema running earlier minor version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, "5.1.0")
			err := upgradeDatabase(sqlStore, "5.8.0")
			require.NoError(t, err)
			// Assert CurrentSchemaVersion, not 5.8.0, since the migrations will move
			// past 5.8.0 regardless of the input parameter.
			require.Equal(t, CurrentSchemaVersion, sqlStore.GetCurrentSchemaVersion())
		})

		t.Run("upgrade schema running later minor version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, "5.99.0")
			err := upgradeDatabase(sqlStore, "5.8.0")
			require.NoError(t, err)
			require.Equal(t, "5.99.0", sqlStore.GetCurrentSchemaVersion())
		})

		t.Run("upgrade schema running earlier major version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, "4.1.0")
			err := upgradeDatabase(sqlStore, CurrentSchemaVersion)
			require.NoError(t, err)
			require.Equal(t, CurrentSchemaVersion, sqlStore.GetCurrentSchemaVersion())
		})

		t.Run("upgrade schema running later major version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, "6.0.0")
			err := upgradeDatabase(sqlStore, "5.8.0")
			require.EqualError(t, err, "Database schema version 6.0.0 is not supported. This Mattermost server supports only >=5.8.0, <6.0.0. Please upgrade to at least version 6.0.0 before continuing.")
			require.Equal(t, "6.0.0", sqlStore.GetCurrentSchemaVersion())
		})
	})
}

func TestSaveSchemaVersion(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		sqlStore := ss.(*SqlStore)

		t.Run("set earliest version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, Version300)
			props, err := ss.System().Get()
			require.NoError(t, err)

			require.Equal(t, Version300, props["Version"])
			require.Equal(t, Version300, sqlStore.GetCurrentSchemaVersion())
		})

		t.Run("set current version", func(t *testing.T) {
			saveSchemaVersion(sqlStore, CurrentSchemaVersion)
			props, err := ss.System().Get()
			require.NoError(t, err)

			require.Equal(t, CurrentSchemaVersion, props["Version"])
			require.Equal(t, CurrentSchemaVersion, sqlStore.GetCurrentSchemaVersion())
		})
	})
}
func createChannelMemberWithLastViewAt(ss store.Store, channelId, userId string, lastViewAt int64) *model.ChannelMember {
	m := model.ChannelMember{}
	m.ChannelId = channelId
	m.UserId = userId
	m.LastViewedAt = lastViewAt
	m.NotifyProps = model.GetDefaultChannelNotifyProps()
	cm, _ := ss.Channel().SaveMember(&m)
	return cm
}
func createPostWithTimestamp(ss store.Store, channelId, userId, rootId, parentId string, timestamp int64) *model.Post {
	m := model.Post{}
	m.CreateAt = timestamp
	m.ChannelId = channelId
	m.UserId = userId
	m.RootId = rootId
	m.ParentId = parentId
	m.Message = "zz" + model.NewId() + "b"
	p, _ := ss.Post().Save(&m)
	return p
}

func createChannelWithLastPostAt(ss store.Store, teamId, creatorId string, lastPostAt, msgCount, rootCount int64) (*model.Channel, error) {
	m := model.Channel{}
	m.TeamId = teamId
	m.TotalMsgCount = msgCount
	m.TotalMsgCountRoot = rootCount
	m.LastPostAt = lastPostAt
	m.CreatorId = creatorId
	m.DisplayName = "Name"
	m.Name = "zz" + model.NewId() + "b"
	m.Type = model.CHANNEL_OPEN
	return ss.Channel().Save(&m, -1)
}

func TestMsgCountRootMigration(t *testing.T) {
	type TestCaseChannel struct {
		Name                           string
		PostTimes                      []int64
		ReplyTimes                     []int64
		MembershipsLastViewAt          []int64
		ExpectedMembershipMsgCountRoot []int64
	}
	type TestTableEntry struct {
		name string
		data []TestCaseChannel
	}
	testTable := []TestTableEntry{
		{
			name: "test1",
			data: []TestCaseChannel{
				{
					Name:                           "channel with one post",
					PostTimes:                      []int64{1000},
					ReplyTimes:                     []int64{0},
					MembershipsLastViewAt:          []int64{1},
					ExpectedMembershipMsgCountRoot: []int64{0},
				},
				{
					Name:                           "channel with one post, read",
					PostTimes:                      []int64{1000},
					ReplyTimes:                     []int64{0},
					MembershipsLastViewAt:          []int64{1000},
					ExpectedMembershipMsgCountRoot: []int64{1},
				},
				{
					Name:                           "with one reply, viewed after 2nd root",
					PostTimes:                      []int64{1000, 2000, 3000, 4000},
					ReplyTimes:                     []int64{1001, 0, 0, 0},
					MembershipsLastViewAt:          []int64{2001},
					ExpectedMembershipMsgCountRoot: []int64{0},
				},
				{
					Name:                           "two replies, 3 memberships",
					PostTimes:                      []int64{1000, 2000, 3000},
					ReplyTimes:                     []int64{1001, 2001, 0},
					MembershipsLastViewAt:          []int64{2000, 5000, 0},
					ExpectedMembershipMsgCountRoot: []int64{0, 3, 0},
				},
			},
		},
	}
	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			StoreTest(t, func(t *testing.T, ss store.Store) {
				sqlStore := ss.(*SqlStore)
				team := createTeam(ss)
				for _, testChannel := range testCase.data {
					t.Run(testChannel.Name, func(t *testing.T) {
						lastPostAt := int64(0)
						for i := range testChannel.PostTimes {
							if testChannel.PostTimes[i] > lastPostAt {
								lastPostAt = testChannel.PostTimes[i]
							}
							if testChannel.ReplyTimes[i] > lastPostAt {
								lastPostAt = testChannel.ReplyTimes[i]
							}
						}
						channel, err := createChannelWithLastPostAt(ss, team.Id, model.NewId(), lastPostAt, int64(len(testChannel.PostTimes)+len(testChannel.ReplyTimes)), int64(len(testChannel.PostTimes)))
						require.NoError(t, err)
						var userIds []string
						for _, md := range testChannel.MembershipsLastViewAt {
							user := createUser(ss)
							userIds = append(userIds, user.Id)
							require.NotNil(t, user)
							cm := createChannelMemberWithLastViewAt(ss, channel.Id, user.Id, md)
							require.NotNil(t, cm)
						}
						for i, pt := range testChannel.PostTimes {
							rt := testChannel.ReplyTimes[i]
							post := createPostWithTimestamp(ss, channel.Id, model.NewId(), "", "", pt)
							require.NotNil(t, post)
							if rt > 0 {
								reply := createPostWithTimestamp(ss, channel.Id, model.NewId(), post.Id, post.Id, rt)
								require.NotNil(t, reply)
							}
						}

						_, err = sqlStore.GetMaster().Exec(`ALTER TABLE Channels DROP COLUMN TotalMsgCountRoot`)
						require.NoError(t, err)
						_, err = sqlStore.GetMaster().Exec(`ALTER TABLE ChannelMembers DROP COLUMN MsgCountRoot`)
						require.NoError(t, err)
						rootCountMigration(sqlStore)

						members, err := ss.Channel().GetMembersByIds(channel.Id, userIds)
						require.NoError(t, err)

						for _, m := range *members {
							for i, uid := range userIds {
								if m.UserId == uid {
									assert.Equal(t, testChannel.ExpectedMembershipMsgCountRoot[i], m.MsgCountRoot)
									break
								}
							}
						}

					})
				}
			})
		})
	}
}

func TestFixCRTCountsAndUnreads(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		sqlStore := ss.(*SqlStore)

		team := createTeam(ss)
		uId1 := model.NewId()
		uId2 := model.NewId()
		c1, err := createChannelWithLastPostAt(ss, team.Id, uId1, 0, 0, 0)
		require.NoError(t, err)
		createChannelMemberWithLastViewAt(ss, c1.Id, uId1, 0)
		createChannelMemberWithLastViewAt(ss, c1.Id, uId2, 0)

		// Create a thread
		// user2: root post 1
		//   - user1: reply 1 to root post 1
		//   - user2: reply 2 to root post 1
		//   - user1: reply 3 to root post 1
		//   - user2: reply 4 to root post 1
		rootPost1 := createPostWithTimestamp(ss, c1.Id, uId2, "", "", 1)
		lastReplyAt := int64(40)
		_ = createPostWithTimestamp(ss, c1.Id, uId1, rootPost1.Id, rootPost1.Id, 10)
		_ = createPostWithTimestamp(ss, c1.Id, uId2, rootPost1.Id, rootPost1.Id, 20)
		_ = createPostWithTimestamp(ss, c1.Id, uId1, rootPost1.Id, rootPost1.Id, 30)
		_ = createPostWithTimestamp(ss, c1.Id, uId2, rootPost1.Id, rootPost1.Id, lastReplyAt)

		// Check created thread is good
		goodThread1, err := ss.Thread().Get(rootPost1.Id)
		require.NoError(t, err)
		require.EqualValues(t, 4, goodThread1.ReplyCount)
		require.EqualValues(t, lastReplyAt, goodThread1.LastReplyAt)
		require.ElementsMatch(t, model.StringArray{uId1, uId2}, goodThread1.Participants)

		// Create ThreadMembership
		goodThreadMembership1, err := ss.Thread().SaveMembership(&model.ThreadMembership{
			PostId:         rootPost1.Id,
			UserId:         uId1,
			Following:      true,
			LastViewed:     lastReplyAt + 1,
			LastUpdated:    lastReplyAt + 1,
			UnreadMentions: 0,
		})
		require.NoError(t, err)
		goodThreadMembership2, err := ss.Thread().SaveMembership(&model.ThreadMembership{
			PostId:         rootPost1.Id,
			UserId:         uId2,
			Following:      true,
			LastViewed:     lastReplyAt + 1,
			LastUpdated:    lastReplyAt + 1,
			UnreadMentions: 0,
		})
		require.NoError(t, err)

		// Update channel last viewed at
		// set channel as fully read for user1
		_, err = ss.Channel().UpdateLastViewedAt([]string{c1.Id}, uId1, true)
		require.NoError(t, err)
		// for user2 set channel as read before the last post in thread
		// user2's threadmembership wont change
		cm2, err := ss.Channel().GetMember(context.Background(), c1.Id, uId2)
		require.NoError(t, err)
		cm2.LastViewedAt = lastReplyAt - 10
		cm2, err = ss.Channel().UpdateMember(cm2)
		require.NoError(t, err)

		// Update ThreadMembership with bad data, as we might expect because
		// of previous bugs or changed behaviour
		badThreadMembership1 := *goodThreadMembership1
		badThreadMembership1.LastViewed = 30
		badThreadMembership1.UnreadMentions = 4
		_, err = ss.Thread().UpdateMembership(&badThreadMembership1)
		require.NoError(t, err)

		// Run migration to fix threads and memberships
		fixCRTThreadCountsAndUnreads(sqlStore)

		// Check bad threadMemberships is fixed
		fixedThreadMembership1, err := ss.Thread().GetMembershipForUser(uId1, rootPost1.Id)
		require.NoError(t, err)
		require.EqualValues(t, lastReplyAt, fixedThreadMembership1.LastViewed)
		require.EqualValues(t, int64(0), fixedThreadMembership1.UnreadMentions)
		require.NotEqual(t, goodThreadMembership1.LastUpdated, fixedThreadMembership1.LastUpdated)

		// check good threadMembership is unchanged
		fixedThreadMembership2, err := ss.Thread().GetMembershipForUser(uId2, rootPost1.Id)
		require.NoError(t, err)
		require.Equal(t, goodThreadMembership2, fixedThreadMembership2)
	})
}

func TestFixCRTChannelUnreads(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		sqlStore := ss.(*SqlStore)

		team := createTeam(ss)
		uId1 := model.NewId()
		uId2 := model.NewId()
		channelLastPostAt := int64(100)
		channelMsgCount := int64(200)
		channelMsgCountRoot := int64(100)
		c1, err := createChannelWithLastPostAt(ss, team.Id, uId1, channelLastPostAt, channelMsgCount, channelMsgCountRoot)
		require.NoError(t, err)

		// Make a membership entry
		cm1, err := ss.Channel().SaveMember(&model.ChannelMember{
			ChannelId:        c1.Id,
			UserId:           uId1,
			LastViewedAt:     channelLastPostAt - 50,
			NotifyProps:      model.GetDefaultChannelNotifyProps(),
			MsgCount:         80,
			MsgCountRoot:     40,
			MentionCount:     5,
			MentionCountRoot: 5,
		})
		require.NoError(t, err)

		// make a bad membership entry
		// LastViewed at newer than channel LastPostAt and with unreads and mentions
		cm2, err := ss.Channel().SaveMember(&model.ChannelMember{
			ChannelId:        c1.Id,
			UserId:           uId2,
			LastViewedAt:     channelLastPostAt + 50,
			NotifyProps:      model.GetDefaultChannelNotifyProps(),
			MsgCount:         80,
			MsgCountRoot:     40,
			MentionCount:     5,
			MentionCountRoot: 5,
		})
		require.NoError(t, err)

		fixCRTChannelMembershipCounts(sqlStore)

		cm1AfterFix, err := ss.Channel().GetMember(context.Background(), c1.Id, uId1)
		require.NoError(t, err)
		// Migration should not affect this channelmembership
		require.Equal(t, *cm1, *cm1AfterFix)

		cm2AfterFix, err := ss.Channel().GetMember(context.Background(), c1.Id, uId2)
		require.NoError(t, err)
		// Check that the channelmembership is fixed
		require.NotEqual(t, *cm2, *cm2AfterFix)
		require.EqualValues(t, 0, cm2AfterFix.MentionCount)
		require.EqualValues(t, 0, cm2AfterFix.MentionCountRoot)
		require.Equal(t, channelMsgCount, cm2AfterFix.MsgCount)
		require.Equal(t, channelMsgCountRoot, cm2AfterFix.MsgCountRoot)
		require.NotEqual(t, cm2.LastUpdateAt, cm2AfterFix.LastUpdateAt)
	})
}
