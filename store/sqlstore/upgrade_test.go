// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestStoreUpgradeDotRelease(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		sqlStore := ss.(*SQLStore)
		saveSchemaVersion(sqlStore, "5.33.1")
		err := upgradeDatabase(sqlStore, CurrentSchemaVersion)
		require.NoError(t, err)
		require.Equal(t, CurrentSchemaVersion, sqlStore.GetCurrentSchemaVersion())
	})
}

func TestStoreUpgrade(t *testing.T) {
	StoreTest(t, func(t *testing.T, ss store.Store) {
		sqlStore := ss.(*SQLStore)

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
		sqlStore := ss.(*SQLStore)

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
func createChannelMemberWithLastViewAt(ss store.Store, channelID, userID string, lastViewAt int64) *model.ChannelMember {
	m := model.ChannelMember{}
	m.ChannelID = channelID
	m.UserID = userID
	m.LastViewedAt = lastViewAt
	m.NotifyProps = model.GetDefaultChannelNotifyProps()
	cm, _ := ss.Channel().SaveMember(&m)
	return cm
}
func createPostWithTimestamp(ss store.Store, channelID, userID, rootID, parentID string, timestamp int64) *model.Post {
	m := model.Post{}
	m.CreateAt = timestamp
	m.ChannelID = channelID
	m.UserID = userID
	m.RootID = rootID
	m.ParentID = parentID
	m.Message = "zz" + model.NewID() + "b"
	p, _ := ss.Post().Save(&m)
	return p
}

func createChannelWithLastPostAt(ss store.Store, teamID, creatorID string, lastPostAt, msgCount, rootCount int64) (*model.Channel, error) {
	m := model.Channel{}
	m.TeamID = teamID
	m.TotalMsgCount = msgCount
	m.TotalMsgCountRoot = rootCount
	m.LastPostAt = lastPostAt
	m.CreatorID = creatorID
	m.DisplayName = "Name"
	m.Name = "zz" + model.NewID() + "b"
	m.Type = model.ChannelTypeOpen
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
				sqlStore := ss.(*SQLStore)
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
						channel, err := createChannelWithLastPostAt(ss, team.ID, model.NewID(), lastPostAt, int64(len(testChannel.PostTimes)+len(testChannel.ReplyTimes)), int64(len(testChannel.PostTimes)))
						require.NoError(t, err)
						var userIDs []string
						for _, md := range testChannel.MembershipsLastViewAt {
							user := createUser(ss)
							userIDs = append(userIDs, user.ID)
							require.NotNil(t, user)
							cm := createChannelMemberWithLastViewAt(ss, channel.ID, user.ID, md)
							require.NotNil(t, cm)
						}
						for i, pt := range testChannel.PostTimes {
							rt := testChannel.ReplyTimes[i]
							post := createPostWithTimestamp(ss, channel.ID, model.NewID(), "", "", pt)
							require.NotNil(t, post)
							if rt > 0 {
								reply := createPostWithTimestamp(ss, channel.ID, model.NewID(), post.ID, post.ID, rt)
								require.NotNil(t, reply)
							}
						}

						_, err = sqlStore.GetMaster().Exec(`ALTER TABLE Channels DROP COLUMN TotalMsgCountRoot`)
						require.NoError(t, err)
						_, err = sqlStore.GetMaster().Exec(`ALTER TABLE ChannelMembers DROP COLUMN MsgCountRoot`)
						require.NoError(t, err)
						rootCountMigration(sqlStore)

						members, err := ss.Channel().GetMembersByIDs(channel.ID, userIDs)
						require.NoError(t, err)

						for _, m := range *members {
							for i, uid := range userIDs {
								if m.UserID == uid {
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
