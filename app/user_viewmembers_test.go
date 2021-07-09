// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestResctrictedViewMembers(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	user1 := th.CreateUser()
	user1.Nickname = "test user1"
	user1.Username = "test-user-1"
	th.App.UpdateUser(user1, false)
	user2 := th.CreateUser()
	user2.Username = "test-user-2"
	user2.Nickname = "test user2"
	th.App.UpdateUser(user2, false)
	user3 := th.CreateUser()
	user3.Username = "test-user-3"
	user3.Nickname = "test user3"
	th.App.UpdateUser(user3, false)
	user4 := th.CreateUser()
	user4.Username = "test-user-4"
	user4.Nickname = "test user4"
	th.App.UpdateUser(user4, false)
	user5 := th.CreateUser()
	user5.Username = "test-user-5"
	user5.Nickname = "test user5"
	th.App.UpdateUser(user5, false)

	// user1 is member of all the channels and teams because is the creator
	th.BasicUser = user1

	team1 := th.CreateTeam()
	team2 := th.CreateTeam()

	channel1 := th.CreateChannel(team1)
	channel2 := th.CreateChannel(team1)
	channel3 := th.CreateChannel(team2)

	th.LinkUserToTeam(user1, team1)
	th.LinkUserToTeam(user2, team1)
	th.LinkUserToTeam(user3, team2)
	th.LinkUserToTeam(user4, team1)
	th.LinkUserToTeam(user4, team2)

	th.AddUserToChannel(user1, channel1)
	th.AddUserToChannel(user2, channel2)
	th.AddUserToChannel(user3, channel3)
	th.AddUserToChannel(user4, channel1)
	th.AddUserToChannel(user4, channel3)

	th.App.SetStatusOnline(user1.ID, true)
	th.App.SetStatusOnline(user2.ID, true)
	th.App.SetStatusOnline(user3.ID, true)
	th.App.SetStatusOnline(user4.ID, true)
	th.App.SetStatusOnline(user5.ID, true)

	t.Run("SearchUsers", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			Search          model.UserSearch
			ExpectedResults []string
		}{
			{
				"without restrictions team1",
				nil,
				model.UserSearch{Term: "test", TeamID: team1.ID},
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"without restrictions team2",
				nil,
				model.UserSearch{Term: "test", TeamID: team2.ID},
				[]string{user3.ID, user4.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				model.UserSearch{Term: "test", TeamID: team1.ID},
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				model.UserSearch{Term: "test", TeamID: team2.ID},
				[]string{user4.ID},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				model.UserSearch{Term: "test", TeamID: team1.ID},
				[]string{user1.ID, user4.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				model.UserSearch{Term: "test", TeamID: team2.ID},
				[]string{user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				model.UserSearch{Term: "test", TeamID: team1.ID},
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				options := model.UserSearchOptions{Limit: 100, ViewRestrictions: tc.Restrictions}
				results, err := th.App.SearchUsers(&tc.Search, &options)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("SearchUsersInTeam", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			ExpectedResults []string
		}{
			{
				"without restrictions team1",
				nil,
				team1.ID,
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"without restrictions team2",
				nil,
				team2.ID,
				[]string{user3.ID, user4.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team1.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team1.ID,
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				options := model.UserSearchOptions{Limit: 100, ViewRestrictions: tc.Restrictions}
				results, err := th.App.SearchUsersInTeam(tc.TeamID, "test", &options)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("AutocompleteUsersInTeam", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			ExpectedResults []string
		}{
			{
				"without restrictions team1",
				nil,
				team1.ID,
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"without restrictions team2",
				nil,
				team2.ID,
				[]string{user3.ID, user4.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team1.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team1.ID,
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				options := model.UserSearchOptions{Limit: 100, ViewRestrictions: tc.Restrictions}
				results, err := th.App.AutocompleteUsersInTeam(tc.TeamID, "tes", &options)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results.InTeam {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("AutocompleteUsersInChannel", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			ChannelID       string
			ExpectedResults []string
		}{
			{
				"without restrictions channel1",
				nil,
				team1.ID,
				channel1.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"without restrictions channel3",
				nil,
				team2.ID,
				channel3.ID,
				[]string{user1.ID, user3.ID, user4.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				channel1.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team2.ID,
				channel3.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team1.ID,
				channel1.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team2.ID,
				channel3.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team1.ID,
				channel1.ID,
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				options := model.UserSearchOptions{Limit: 100, ViewRestrictions: tc.Restrictions}
				results, err := th.App.AutocompleteUsersInChannel(tc.TeamID, tc.ChannelID, "tes", &options)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results.InChannel {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetNewUsersForTeam", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			ExpectedResults []string
		}{
			{
				"without restrictions team1",
				nil,
				team1.ID,
				[]string{user2.ID, user4.ID},
			},
			{
				"without restrictions team2",
				nil,
				team2.ID,
				[]string{user3.ID, user4.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				[]string{user2.ID, user4.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team1.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team1.ID,
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				results, err := th.App.GetNewUsersForTeamPage(tc.TeamID, 0, 2, false, tc.Restrictions)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetRecentlyActiveUsersForTeamPage", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			ExpectedResults []string
		}{
			{
				"without restrictions team1",
				nil,
				team1.ID,
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"without restrictions team2",
				nil,
				team2.ID,
				[]string{user3.ID, user4.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team1.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team1.ID,
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				results, err := th.App.GetRecentlyActiveUsersForTeamPage(tc.TeamID, 0, 3, false, tc.Restrictions)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)

				results, err = th.App.GetRecentlyActiveUsersForTeamPage(tc.TeamID, 0, 1, false, tc.Restrictions)
				require.Nil(t, err)
				if len(tc.ExpectedResults) > 1 {
					assert.Len(t, results, 1)
				} else {
					assert.Len(t, results, len(tc.ExpectedResults))
				}
			})
		}
	})

	t.Run("GetUsers", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			ExpectedResults []string
		}{
			{
				"without restrictions",
				nil,
				[]string{user1.ID, user2.ID, user3.ID, user4.ID, user5.ID},
			},
			{
				"with team restrictions",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"with channel restrictions",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				[]string{user1.ID, user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				options := model.UserGetOptions{Page: 0, PerPage: 100, ViewRestrictions: tc.Restrictions}
				results, err := th.App.GetUsers(&options)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetUsersWithoutTeam", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			ExpectedResults []string
		}{
			{
				"without restrictions",
				nil,
				[]string{user5.ID},
			},
			{
				"with team restrictions",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				[]string{},
			},
			{
				"with channel restrictions",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				[]string{},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				results, err := th.App.GetUsersWithoutTeam(&model.UserGetOptions{Page: 0, PerPage: 100, ViewRestrictions: tc.Restrictions})
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetUsersNotInTeam", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			ExpectedResults []string
		}{
			{
				"without restrictions team1",
				nil,
				team1.ID,
				[]string{user3.ID, user5.ID},
			},
			{
				"without restrictions team2",
				nil,
				team2.ID,
				[]string{user1.ID, user2.ID, user5.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team2.ID,
				[]string{user1.ID, user2.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				[]string{},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team2.ID,
				[]string{user1.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team1.ID,
				[]string{},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team2.ID,
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				results, err := th.App.GetUsersNotInTeam(tc.TeamID, false, 0, 100, tc.Restrictions)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetUsersNotInChannel", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			ChannelID       string
			ExpectedResults []string
		}{
			{
				"without restrictions channel1",
				nil,
				team1.ID,
				channel1.ID,
				[]string{user2.ID},
			},
			{
				"without restrictions channel2",
				nil,
				team1.ID,
				channel2.ID,
				[]string{user4.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				channel1.ID,
				[]string{user2.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team2.ID},
				},
				team1.ID,
				channel1.ID,
				[]string{},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel2.ID},
				},
				team1.ID,
				channel1.ID,
				[]string{user2.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel2.ID},
				},
				team1.ID,
				channel2.ID,
				[]string{},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team1.ID,
				channel1.ID,
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				results, err := th.App.GetUsersNotInChannel(tc.TeamID, tc.ChannelID, false, 0, 100, tc.Restrictions)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetUsersByIds", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			UserIDs         []string
			ExpectedResults []string
		}{
			{
				"without restrictions",
				nil,
				[]string{user1.ID, user2.ID, user3.ID},
				[]string{user1.ID, user2.ID, user3.ID},
			},
			{
				"with team restrictions",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				[]string{user1.ID, user2.ID, user3.ID},
				[]string{user1.ID, user2.ID},
			},
			{
				"with channel restrictions",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				[]string{user1.ID, user2.ID, user3.ID},
				[]string{user1.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				[]string{user1.ID, user2.ID, user3.ID},
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				results, err := th.App.GetUsersByIDs(tc.UserIDs, &store.UserGetByIDsOpts{
					IsAdmin:          false,
					ViewRestrictions: tc.Restrictions,
				})
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetUsersByUsernames", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			Usernames       []string
			ExpectedResults []string
		}{
			{
				"without restrictions",
				nil,
				[]string{user1.Username, user2.Username, user3.Username},
				[]string{user1.ID, user2.ID, user3.ID},
			},
			{
				"with team restrictions",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				[]string{user1.Username, user2.Username, user3.Username},
				[]string{user1.ID, user2.ID},
			},
			{
				"with channel restrictions",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				[]string{user1.Username, user2.Username, user3.Username},
				[]string{user1.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				[]string{user1.Username, user2.Username, user3.Username},
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				results, err := th.App.GetUsersByUsernames(tc.Usernames, false, tc.Restrictions)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.ID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetTotalUsersStats", func(t *testing.T) {
		testCases := []struct {
			Name           string
			Restrictions   *model.ViewUsersRestrictions
			ExpectedResult int64
		}{
			{
				"without restrictions",
				nil,
				5,
			},
			{
				"with team restrictions",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				3,
			},
			{
				"with channel restrictions",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				2,
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				0,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				result, err := th.App.GetTotalUsersStats(tc.Restrictions)
				require.Nil(t, err)
				assert.Equal(t, tc.ExpectedResult, result.TotalUsersCount)
			})
		}
	})

	t.Run("GetTeamMembers", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			ExpectedResults []string
		}{
			{
				"without restrictions team1",
				nil,
				team1.ID,
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"without restrictions team2",
				nil,
				team2.ID,
				[]string{user3.ID, user4.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				[]string{user1.ID, user2.ID, user4.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team1.ID,
				[]string{user1.ID, user4.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team2.ID,
				[]string{user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team1.ID,
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				getTeamMemberOptions := &model.TeamMembersGetOptions{
					ViewRestrictions: tc.Restrictions,
				}
				results, err := th.App.GetTeamMembers(tc.TeamID, 0, 100, getTeamMemberOptions)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.UserID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})

	t.Run("GetTeamMembersByIds", func(t *testing.T) {
		testCases := []struct {
			Name            string
			Restrictions    *model.ViewUsersRestrictions
			TeamID          string
			UserIDs         []string
			ExpectedResults []string
		}{
			{
				"without restrictions team1",
				nil,
				team1.ID,
				[]string{user1.ID, user2.ID, user3.ID},
				[]string{user1.ID, user2.ID},
			},
			{
				"without restrictions team2",
				nil,
				team2.ID,
				[]string{user1.ID, user2.ID, user3.ID},
				[]string{user3.ID},
			},
			{
				"with team restrictions with valid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team1.ID,
				[]string{user1.ID, user2.ID, user3.ID},
				[]string{user1.ID, user2.ID},
			},
			{
				"with team restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Teams: []string{team1.ID},
				},
				team2.ID,
				[]string{user2.ID, user4.ID},
				[]string{user4.ID},
			},
			{
				"with channel restrictions with valid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team1.ID,
				[]string{user2.ID, user4.ID},
				[]string{user4.ID},
			},
			{
				"with channel restrictions with invalid team",
				&model.ViewUsersRestrictions{
					Channels: []string{channel1.ID},
				},
				team2.ID,
				[]string{user2.ID, user4.ID},
				[]string{user4.ID},
			},
			{
				"with restricting everything",
				&model.ViewUsersRestrictions{
					Channels: []string{},
					Teams:    []string{},
				},
				team1.ID,
				[]string{user1.ID, user2.ID, user2.ID, user4.ID},
				[]string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				results, err := th.App.GetTeamMembersByIDs(tc.TeamID, tc.UserIDs, tc.Restrictions)
				require.Nil(t, err)
				ids := []string{}
				for _, result := range results {
					ids = append(ids, result.UserID)
				}
				assert.ElementsMatch(t, tc.ExpectedResults, ids)
			})
		}
	})
}
