// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchtest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

var searchUserStoreTests = []searchTest{
	{
		Name: "Should retrieve all users in a channel if the search term is empty",
		Fn:   testGetAllUsersInChannelWithEmptyTerm,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should honor channel restrictions when autocompleting users",
		Fn:   testHonorChannelRestrictionsAutocompletingUsers,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should honor team restrictions when autocompleting users",
		Fn:   testHonorTeamRestrictionsAutocompletingUsers,
		Tags: []string{EngineElasticSearch, EngineBleve},
	},
	{
		Name:        "Should return nothing if the user can't access the channels of a given search",
		Fn:          testShouldReturnNothingWithoutProperAccess,
		Tags:        []string{EngineAll},
		Skip:        true,
		SkipMessage: "Failing when the ListOfAllowedChannels property is empty",
	},
	{
		Name: "Should autocomplete for user using username",
		Fn:   testAutocompleteUserByUsername,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should autocomplete user searching by first name",
		Fn:   testAutocompleteUserByFirstName,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should autocomplete user searching by last name",
		Fn:   testAutocompleteUserByLastName,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should autocomplete for user using nickname",
		Fn:   testAutocompleteUserByNickName,
		Tags: []string{EngineAll},
	},
	{
		Name:        "Should autocomplete for user using email",
		Fn:          testAutocompleteUserByEmail,
		Tags:        []string{EngineAll},
		Skip:        true,
		SkipMessage: "Failing for multiple different reasons in the engines",
	},
	{
		Name: "Should be able not to match specific queries with mail",
		Fn:   testShouldNotMatchSpecificQueriesEmail,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should be able to autocomplete a user by part of its username splitted by Dot",
		Fn:   testAutocompleteUserByUsernameWithDot,
		Tags: []string{EngineElasticSearch, EngineBleve},
	},
	{
		Name: "Should be able to autocomplete a user by part of its username splitted by underscore",
		Fn:   testAutocompleteUserByUsernameWithUnderscore,
		Tags: []string{EngineElasticSearch, EngineBleve},
	},
	{
		Name: "Should be able to autocomplete a user by part of its username splitted by hyphen",
		Fn:   testAutocompleteUserByUsernameWithHyphen,
		Tags: []string{EngineElasticSearch, EngineBleve},
	},
	{
		Name: "Should escape the percentage character",
		Fn:   testShouldEscapePercentageCharacter,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should escape the dash character",
		Fn:   testShouldEscapeUnderscoreCharacter,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should be able to search inactive users",
		Fn:   testShouldBeAbleToSearchInactiveUsers,
		Tags: []string{EngineMySql, EnginePostgres, EngineElasticSearch},
	},
	{
		Name: "Should be able to search filtering by role",
		Fn:   testShouldBeAbleToSearchFilteringByRole,
		Tags: []string{EngineMySql, EnginePostgres, EngineElasticSearch},
	},
	{
		Name: "Should ignore leading @ when searching users",
		Fn:   testShouldIgnoreLeadingAtSymbols,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should search users in a case insensitive manner",
		Fn:   testSearchUsersShouldBeCaseInsensitive,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support one or two character usernames and first/last names in search",
		Fn:   testSearchOneTwoCharUsersnameAndFirstLastNames,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support Korean characters",
		Fn:   testShouldSupportKoreanCharacters,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support search with a hyphen at the end of the term",
		Fn:   testSearchWithHyphenAtTheEndOfTheTerm,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support search all users in a team",
		Fn:   testSearchUsersInTeam,
		Tags: []string{EngineElasticSearch},
	},
	{
		Name: "Should support search users by full name",
		Fn:   testSearchUsersByFullName,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support search all users in a team with username containing a dot",
		Fn:   testSearchUsersInTeamUsernameWithDot,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support search all users in a team with username containing a hyphen",
		Fn:   testSearchUsersInTeamUsernameWithHyphen,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support search all users in a team with username containing a underscore",
		Fn:   testSearchUsersInTeamUsernameWithUnderscore,
		Tags: []string{EngineAll},
	},
}

func TestSearchUserStore(t *testing.T, s store.Store, testEngine *SearchTestEngine) {
	th := &SearchTestHelper{
		Store: s,
	}
	err := th.SetupBasicFixtures()
	require.NoError(t, err)
	defer th.CleanFixtures()
	runTestSearch(t, testEngine, searchUserStoreTests, th)
}

func testGetAllUsersInChannelWithEmptyTerm(t *testing.T, th *SearchTestHelper) {
	options := &model.UserSearchOptions{
		AllowFullNames: true,
		Limit:          model.UserSearchDefaultLimit,
	}
	users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
	require.NoError(t, err)
	th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
	th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)

	t.Run("Should be able to correctly honor limit when autocompleting", func(t *testing.T) {
		result, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		require.Len(t, result.InChannel, 1)
		require.Len(t, result.OutOfChannel, 1)
	})

	t.Run("Return all users in team", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})

	t.Run("Return all users in teams even though some of them don't have a team associated", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		userAlternate, err := th.createUser("user-alternate", "user-alternate", "user", "alternate")
		require.NoError(t, err)
		defer th.deleteUser(userAlternate)
		userGuest, err := th.createGuest("user-guest", "user-guest", "user", "guest")
		require.NoError(t, err)
		defer th.deleteUser(userGuest)

		// In case teamId and channelId are empty our current logic goes through Search
		users, err := th.Store.User().Search("", "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, th.User2, th.UserAnotherTeam,
			userAlternate, userGuest}, users)
	})
}

func testHonorChannelRestrictionsAutocompletingUsers(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("user-alternate", "user-alternate", "user", "alternate")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	guest, err := th.createGuest("guest", "guest", "guest", "one")
	require.NoError(t, err)
	err = th.addUserToTeams(guest, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(guest, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	defer th.deleteUser(guest)
	t.Run("Autocomplete users with channel restrictions", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Channels: []string{th.ChannelBasic.ID}}
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, userAlternate, guest}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Autocomplete users with term and channel restrictions", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Channels: []string{th.ChannelBasic.ID}}
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "alt", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Autocomplete users with all channels restricted", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Teams: []string{}, Channels: []string{}}
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Autocomplete users with all channels restricted but with empty team", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Teams: []string{}, Channels: []string{}}
		users, err := th.Store.User().AutocompleteUsersInChannel("", th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Autocomplete users with empty team and channels restricted", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Channels: []string{th.ChannelBasic.ID}}
		// In case teamId and channelId are empty our current logic goes through Search
		users, err := th.Store.User().Search("", "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate, guest, th.User}, users)
	})
}

func testHonorTeamRestrictionsAutocompletingUsers(t *testing.T, th *SearchTestHelper) {
	t.Run("Should return results for users in the team", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Teams: []string{th.Team.ID}}
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
	t.Run("Should return empty because we're filtering all the teams", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Teams: []string{}, Channels: []string{}}
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return empty when searching in one team and filtering by another", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Teams: []string{th.AnotherTeam.ID}}
		users, err := th.Store.User().Search(th.Team.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users)

		acusers, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, acusers.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, acusers.OutOfChannel)
	})
}
func testShouldReturnNothingWithoutProperAccess(t *testing.T, th *SearchTestHelper) {
	t.Run("Should return results users for the defined channel in the list", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ListOfAllowedChannels = []string{th.ChannelBasic.ID}
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return empty because we're filtering all the channels", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		options.ListOfAllowedChannels = []string{}
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
}
func testAutocompleteUserByUsername(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternateusername", "alternatenick", "user", "alternate")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	options := createDefaultOptions(false, false, false)
	users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "basicusername", options)
	require.NoError(t, err)
	th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
	th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
}
func testAutocompleteUserByFirstName(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("user-alternate", "user-alternate", "altfirstname", "lastname")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should autocomplete users when the first name is unique", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "altfirstname", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should autocomplete users for in the channel and out of the channel with the same first name", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "basicfirstname", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
}
func testAutocompleteUserByLastName(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("user-alternate", "user-alternate", "firstname", "altlastname")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should return results when the last name is unique", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "altlastname", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return results for in the channel and out of the channel with the same last name", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "basiclastname", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
}
func testAutocompleteUserByNickName(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternateusername", "alternatenickname", "firstname", "altlastname")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should return results when the nickname is unique", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "alternatenickname", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return users that share the same part of the nickname", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "basicnickname", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
}
func testAutocompleteUserByEmail(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternateusername", "alternatenickname", "firstname", "altlastname")
	require.NoError(t, err)
	userAlternate.Email = "useralt@test.email.com"
	_, err = th.Store.User().Update(userAlternate, false)
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should autocomplete users when the email is unique", func(t *testing.T) {
		options := createDefaultOptions(false, true, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "useralt@test.email.com", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should autocomplete users that share the same email user prefix", func(t *testing.T) {
		options := createDefaultOptions(false, true, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "success_", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
	t.Run("Should autocomplete users that share the same email domain", func(t *testing.T) {
		options := createDefaultOptions(false, true, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "simulator.amazon.com", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
	t.Run("Should search users when the email is unique", func(t *testing.T) {
		options := createDefaultOptions(false, true, false)
		users, err := th.Store.User().Search(th.Team.ID, "useralt@test.email.com", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users)
	})
	t.Run("Should search users that share the same email user prefix", func(t *testing.T) {
		options := createDefaultOptions(false, true, false)
		users, err := th.Store.User().Search(th.Team.ID, "success_", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users)
	})
	t.Run("Should search users that share the same email domain", func(t *testing.T) {
		options := createDefaultOptions(false, true, false)
		users, err := th.Store.User().Search(th.Team.ID, "simulator.amazon.com", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users)
	})
}
func testShouldNotMatchSpecificQueriesEmail(t *testing.T, th *SearchTestHelper) {
	options := createDefaultOptions(false, false, false)
	users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "success_", options)
	require.NoError(t, err)
	th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.InChannel)
	th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
}
func testAutocompleteUserByUsernameWithDot(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate.username", "alternatenickname", "firstname", "altlastname")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should return results when searching for the whole username with Dot", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "alternate.username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return results when searching for part of the username including the Dot", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, ".username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return results when searching for part of the username not including the Dot", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
}
func testAutocompleteUserByUsernameWithUnderscore(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate_username", "alternatenickname", "firstname", "altlastname")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should return results when searching for the whole username with underscore", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "alternate_username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return results when searching for part of the username including the underscore", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "_username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return results when searching for part of the username not including the underscore", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
}
func testAutocompleteUserByUsernameWithHyphen(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate-username", "alternatenickname", "firstname", "altlastname")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should return results when searching for the whole username with hyphen", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "alternate-username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return results when searching for part of the username including the hyphen", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "-username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should return results when searching for part of the username not including the hyphen", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "username", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
}
func testShouldEscapePercentageCharacter(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternateusername", "alternate%nickname", "firstname", "altlastname")
	require.NoError(t, err)

	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should autocomplete users escaping percentage symbol", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "alternate%", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should search users escaping percentage symbol", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "alternate%", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users)
	})
}
func testShouldEscapeUnderscoreCharacter(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate_username", "alternatenickname", "firstname", "altlastname")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should autocomplete users escaping underscore symbol", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "alternate_", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should search users escaping underscore symbol", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "alternate_", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users)
	})
}

func testShouldBeAbleToSearchInactiveUsers(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("basicusernamealternate", "alternatenickname", "firstname", "altlastname")
	require.NoError(t, err)
	userAlternate.DeleteAt = model.GetMillis()
	_, err = th.Store.User().Update(userAlternate, true)
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should autocomplete inactive users if we allow it", func(t *testing.T) {
		options := createDefaultOptions(false, false, true)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "basicusername", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
	t.Run("Should search inactive users if we allow it", func(t *testing.T) {
		options := createDefaultOptions(false, false, true)
		users, err := th.Store.User().Search(th.Team.ID, "basicusername", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, th.User2, userAlternate}, users)
	})
	t.Run("Shouldn't autocomplete inactive users if we don't allow it", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "basicusername", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
	t.Run("Shouldn't search inactive users if we don't allow it", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "basicusername", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, th.User2}, users)
	})
}

func testShouldBeAbleToSearchFilteringByRole(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("basicusernamealternate", "alternatenickname", "firstname", "altlastname")
	require.NoError(t, err)
	userAlternate.Roles = "system_admin system_user"
	_, err = th.Store.User().Update(userAlternate, true)
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	userAlternate2, err := th.createUser("basicusernamealternate2", "alternatenickname2", "firstname2", "altlastname2")
	require.NoError(t, err)
	userAlternate2.Roles = "system_user"
	_, err = th.Store.User().Update(userAlternate2, true)
	require.NoError(t, err)
	defer th.deleteUser(userAlternate2)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToTeams(userAlternate2, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should autocomplete users filtering by roles", func(t *testing.T) {
		options := createDefaultOptions(false, false, true)
		options.Role = "system_admin"
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should search users filtering by roles", func(t *testing.T) {
		options := createDefaultOptions(false, false, true)
		options.Role = "system_admin"
		users, err := th.Store.User().Search(th.Team.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users)
	})
}

func testShouldIgnoreLeadingAtSymbols(t *testing.T, th *SearchTestHelper) {
	t.Run("Should autocomplete ignoring the @ symbol at the beginning", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "@basicusername", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
	})
	t.Run("Should search ignoring the @ symbol at the beginning", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "@basicusername", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, th.User2}, users)
	})
}

func testSearchUsersShouldBeCaseInsensitive(t *testing.T, th *SearchTestHelper) {
	options := createDefaultOptions(false, false, false)
	users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "BaSiCUsErNaMe", options)
	require.NoError(t, err)
	th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users.InChannel)
	th.assertUsersMatchInAnyOrder(t, []*model.User{th.User2}, users.OutOfChannel)
}

func testSearchOneTwoCharUsersnameAndFirstLastNames(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("ho", "alternatenickname", "zi", "k")
	require.NoError(t, err)

	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should support two characters in the full name", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "zi", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should support two characters in the username", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "ho", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
}

func testShouldSupportKoreanCharacters(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate-username", "alternate-nickname", "서강준", "안신원")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)

	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	t.Run("Should support hanja korean characters", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "서강준", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
	t.Run("Should support hangul korean characters", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "안신원", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
	})
}

func testSearchWithHyphenAtTheEndOfTheTerm(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate-username", "alternate-nickname", "altfirst", "altlast")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	options := createDefaultOptions(true, false, false)
	users, err := th.Store.User().AutocompleteUsersInChannel(th.Team.ID, th.ChannelBasic.ID, "alternate-", options)
	require.NoError(t, err)
	th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users.InChannel)
	th.assertUsersMatchInAnyOrder(t, []*model.User{}, users.OutOfChannel)
}

func testSearchUsersInTeam(t *testing.T, th *SearchTestHelper) {
	t.Run("Should return all the team users", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, th.User2}, users)
	})
	t.Run("Should return all the team users with no team id", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search("", "basicusername", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, th.User2, th.UserAnotherTeam}, users)
	})
	t.Run("Should return all the team users filtered by username", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "basicusername1", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users)
	})
	t.Run("Should not return spurious results", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "falseuser", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users)
	})
	t.Run("Should return all the team users filtered by username and with channel restrictions", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Channels: []string{th.ChannelBasic.ID}}
		users, err := th.Store.User().Search(th.Team.ID, "basicusername", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users)
	})
	t.Run("Should return all the team users filtered by username and with all channel restricted", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		options.ViewRestrictions = &model.ViewUsersRestrictions{Channels: []string{}}
		users, err := th.Store.User().Search(th.Team.ID, "basicusername1", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users)
	})
	t.Run("Should honor the limit when searching users in team", func(t *testing.T) {
		optionsWithLimit := &model.UserSearchOptions{
			Limit: 1,
		}

		users, err := th.Store.User().Search(th.Team.ID, "", optionsWithLimit)
		require.NoError(t, err)
		require.Len(t, users, 1)
	})
}

func testSearchUsersInTeamUsernameWithDot(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate.username", "altnickname", "altfirst", "altlast")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	options := createDefaultOptions(true, false, false)
	users, err := th.Store.User().Search(th.Team.ID, "alternate.", options)
	require.NoError(t, err)
	th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users)
}

func testSearchUsersInTeamUsernameWithHyphen(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate-username", "altnickname", "altfirst", "altlast")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	options := createDefaultOptions(true, false, false)
	users, err := th.Store.User().Search(th.Team.ID, "alternate-", options)
	require.NoError(t, err)
	th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users)
}

func testSearchUsersInTeamUsernameWithUnderscore(t *testing.T, th *SearchTestHelper) {
	userAlternate, err := th.createUser("alternate_username", "altnickname", "altfirst", "altlast")
	require.NoError(t, err)
	defer th.deleteUser(userAlternate)
	err = th.addUserToTeams(userAlternate, []string{th.Team.ID})
	require.NoError(t, err)
	err = th.addUserToChannels(userAlternate, []string{th.ChannelBasic.ID})
	require.NoError(t, err)
	options := createDefaultOptions(true, false, false)
	users, err := th.Store.User().Search(th.Team.ID, "alternate_", options)
	require.NoError(t, err)
	th.assertUsersMatchInAnyOrder(t, []*model.User{userAlternate}, users)
}

func testSearchUsersByFullName(t *testing.T, th *SearchTestHelper) {
	t.Run("Should search users by full name", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "basicfirstname", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User, th.User2}, users)
	})
	t.Run("Should search user by full name", func(t *testing.T) {
		options := createDefaultOptions(true, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "basicfirstname1", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{th.User}, users)
	})
	t.Run("Should return empty when search by full name and is deactivated", func(t *testing.T) {
		options := createDefaultOptions(false, false, false)
		users, err := th.Store.User().Search(th.Team.ID, "basicfirstname1", options)
		require.NoError(t, err)
		th.assertUsersMatchInAnyOrder(t, []*model.User{}, users)
	})
}

func createDefaultOptions(allowFullName, allowEmails, allowInactive bool) *model.UserSearchOptions {
	return &model.UserSearchOptions{
		AllowFullNames: allowFullName,
		AllowEmails:    allowEmails,
		AllowInactive:  allowInactive,
		Limit:          model.UserSearchDefaultLimit,
	}
}
