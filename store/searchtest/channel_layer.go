// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchtest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

var searchChannelStoreTests = []searchTest{
	{
		Name: "Should be able to autocomplete a channel by name",
		Fn:   testAutocompleteChannelByName,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should be able to autocomplete a channel by display name",
		Fn:   testAutocompleteChannelByDisplayName,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should be able to autocomplete a channel by a part of its name when has parts splitted by - character",
		Fn:   testAutocompleteChannelByNameSplittedWithDashChar,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should be able to autocomplete a channel by a part of its name when has parts splitted by _ character",
		Fn:   testAutocompleteChannelByNameSplittedWithUnderscoreChar,
		Tags: []string{EngineMySQL, EngineElasticSearch, EngineBleve},
	},
	{
		Name: "Should be able to autocomplete a channel by a part of its display name when has parts splitted by whitespace character",
		Fn:   testAutocompleteChannelByDisplayNameSplittedByWhitespaces,
		Tags: []string{EngineMySQL, EngineElasticSearch, EngineBleve},
	},
	{
		Name: "Should be able to autocomplete retrieving all channels if the term is empty",
		Fn:   testAutocompleteAllChannelsIfTermIsEmpty,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should be able to autocomplete channels in a case insensitive manner",
		Fn:   testSearchChannelsInCaseInsensitiveManner,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should autocomplete only returning public channels",
		Fn:   testSearchOnlyPublicChannels,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support to autocomplete having a hyphen as the last character",
		Fn:   testSearchShouldSupportHavingHyphenAsLastCharacter,
		Tags: []string{EngineAll},
	},
	{
		Name: "Should support to autocomplete with archived channels",
		Fn:   testSearchShouldSupportAutocompleteWithArchivedChannels,
		Tags: []string{EngineAll},
	},
}

func TestSearchChannelStore(t *testing.T, s store.Store, testEngine *SearchTestEngine) {
	th := &SearchTestHelper{
		Store: s,
	}
	err := th.SetupBasicFixtures()
	require.NoError(t, err)
	defer th.CleanFixtures()
	runTestSearch(t, testEngine, searchChannelStoreTests, th)
}

func testAutocompleteChannelByName(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel-alternate", "Channel Alternate", "Channel Alternate", model.ChannelTypeOpen, false)
	require.NoError(t, err)
	defer th.deleteChannel(alternate)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "channel-a", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID, alternate.ID}, res)
}

func testAutocompleteChannelByDisplayName(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel-alternate", "ChannelAlternate", "", model.ChannelTypeOpen, false)
	require.NoError(t, err)
	defer th.deleteChannel(alternate)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "ChannelA", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID, alternate.ID}, res)
}

func testAutocompleteChannelByNameSplittedWithDashChar(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel-alternate", "ChannelAlternate", "", model.ChannelTypeOpen, false)
	require.NoError(t, err)
	defer th.deleteChannel(alternate)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "channel-a", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID, alternate.ID}, res)
}

func testAutocompleteChannelByNameSplittedWithUnderscoreChar(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel_alternate", "ChannelAlternate", "", model.ChannelTypeOpen, false)
	require.NoError(t, err)
	defer th.deleteChannel(alternate)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "channel_a", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{alternate.ID}, res)
}

func testAutocompleteChannelByDisplayNameSplittedByWhitespaces(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel-alternate", "Channel Alternate", "", model.ChannelTypeOpen, false)
	require.NoError(t, err)

	defer th.deleteChannel(alternate)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "Channel A", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{alternate.ID}, res)
}
func testAutocompleteAllChannelsIfTermIsEmpty(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel-alternate", "Channel Alternate", "", model.ChannelTypeOpen, false)
	require.NoError(t, err)
	other, err := th.createChannel(th.Team.ID, "other-channel", "Other Channel", "", model.ChannelTypeOpen, false)
	require.NoError(t, err)
	defer th.deleteChannel(alternate)
	defer th.deleteChannel(other)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID, alternate.ID, other.ID}, res)
}

func testSearchChannelsInCaseInsensitiveManner(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel-alternate", "ChannelAlternate", "", model.ChannelTypeOpen, false)
	require.NoError(t, err)
	defer th.deleteChannel(alternate)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "channela", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID, alternate.ID}, res)
	res, err = th.Store.Channel().AutocompleteInTeam(th.Team.ID, "ChAnNeL-a", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID, alternate.ID}, res)
}

func testSearchOnlyPublicChannels(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel-alternate", "ChannelAlternate", "", model.ChannelTypePrivate, false)
	require.NoError(t, err)
	defer th.deleteChannel(alternate)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "channel-a", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID}, res)
}

func testSearchShouldSupportHavingHyphenAsLastCharacter(t *testing.T, th *SearchTestHelper) {
	alternate, err := th.createChannel(th.Team.ID, "channel-alternate", "ChannelAlternate", "", model.ChannelTypeOpen, false)
	require.NoError(t, err)
	defer th.deleteChannel(alternate)
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "channel-", false)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID, alternate.ID}, res)
}

func testSearchShouldSupportAutocompleteWithArchivedChannels(t *testing.T, th *SearchTestHelper) {
	res, err := th.Store.Channel().AutocompleteInTeam(th.Team.ID, "channel-", true)
	require.NoError(t, err)
	th.checkChannelIDsMatch(t, []string{th.ChannelBasic.ID, th.ChannelDeleted.ID}, res)
}
