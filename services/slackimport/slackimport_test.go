// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slackimport

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestSlackConvertTimeStamp(t *testing.T) {
	assert.EqualValues(t, slackConvertTimeStamp("1469785419.000033"), 1469785419000)
}

func TestSlackConvertChannelName(t *testing.T) {
	for _, tc := range []struct {
		nameInput string
		idInput   string
		output    string
	}{
		{"test-channel", "C0G08DLQH", "test-channel"},
		{"_test_channel_", "C0G04DLQH", "test_channel"},
		{"__test", "C0G07DLQH", "test"},
		{"-t", "C0G06DLQH", "slack-channel-t"},
		{"a", "C0G05DLQH", "slack-channel-a"},
		{"случайный", "C0G05DLQD", "c0g05dlqd"},
	} {
		assert.Equal(t, slackConvertChannelName(tc.nameInput, tc.idInput), tc.output, "nameInput = %v", tc.nameInput)
	}
}

func TestSlackConvertUserMentions(t *testing.T) {
	users := []slackUser{
		{ID: "U00000A0A", Username: "firstuser"},
		{ID: "U00000B1B", Username: "seconduser"},
	}

	posts := map[string][]slackPost{
		"test-channel": {
			{
				Text: "<!channel>: Hi guys.",
			},
			{
				Text: "Calling <!here|@here>.",
			},
			{
				Text: "Yo <!everyone>.",
			},
			{
				Text: "Regular user test <@U00000B1B|seconduser> and <@U00000A0A>.",
			},
		},
	}

	expectedPosts := map[string][]slackPost{
		"test-channel": {
			{
				Text: "@channel: Hi guys.",
			},
			{
				Text: "Calling @here.",
			},
			{
				Text: "Yo @all.",
			},
			{
				Text: "Regular user test @seconduser and @firstuser.",
			},
		},
	}

	assert.Equal(t, expectedPosts, slackConvertUserMentions(users, posts))
}

func TestSlackConvertChannelMentions(t *testing.T) {
	channels := []slackChannel{
		{ID: "C000AA00A", Name: "one"},
		{ID: "C000BB11B", Name: "two"},
	}

	posts := map[string][]slackPost{
		"test-channel": {
			{
				Text: "Go to <#C000AA00A>.",
			},
			{
				User: "U00000A0A",
				Text: "Try <#C000BB11B|two> for this.",
			},
		},
	}

	expectedPosts := map[string][]slackPost{
		"test-channel": {
			{
				Text: "Go to ~one.",
			},
			{
				User: "U00000A0A",
				Text: "Try ~two for this.",
			},
		},
	}

	assert.Equal(t, expectedPosts, slackConvertChannelMentions(channels, posts))
}

func TestSlackParseChannels(t *testing.T) {
	file, err := os.Open("tests/slack-import-test-channels.json")
	require.NoError(t, err)
	defer file.Close()

	channels, err := slackParseChannels(file, "O")
	require.NoError(t, err)
	assert.Equal(t, 6, len(channels))
}

func TestSlackParseDirectMessages(t *testing.T) {
	file, err := os.Open("tests/slack-import-test-direct-messages.json")
	require.NoError(t, err)
	defer file.Close()

	channels, err := slackParseChannels(file, "D")
	require.NoError(t, err)
	assert.Equal(t, 4, len(channels))
}

func TestSlackParsePrivateChannels(t *testing.T) {
	file, err := os.Open("tests/slack-import-test-private-channels.json")
	require.NoError(t, err)
	defer file.Close()

	channels, err := slackParseChannels(file, "P")
	require.NoError(t, err)
	assert.Equal(t, 1, len(channels))
}

func TestSlackParseGroupDirectMessages(t *testing.T) {
	file, err := os.Open("tests/slack-import-test-group-direct-messages.json")
	require.NoError(t, err)
	defer file.Close()

	channels, err := slackParseChannels(file, "G")
	require.NoError(t, err)
	assert.Equal(t, 3, len(channels))
}

func TestSlackParseUsers(t *testing.T) {
	file, err := os.Open("tests/slack-import-test-users.json")
	require.NoError(t, err)
	defer file.Close()

	users, err := slackParseUsers(file)
	require.NoError(t, err)
	assert.Equal(t, 11, len(users))
}

func TestSlackParsePosts(t *testing.T) {
	file, err := os.Open("tests/slack-import-test-posts.json")
	require.NoError(t, err)
	defer file.Close()

	posts, err := slackParsePosts(file)
	require.NoError(t, err)
	assert.Equal(t, 9, len(posts))
}

func TestSlackParseMultipleAttachments(t *testing.T) {
	file, err := os.Open("tests/slack-import-test-posts.json")
	require.NoError(t, err)
	defer file.Close()

	posts, err := slackParsePosts(file)
	require.NoError(t, err)
	assert.Equal(t, 2, len(posts[8].Files))
}

func TestSlackSanitiseChannelProperties(t *testing.T) {
	c1 := model.Channel{
		DisplayName: "display-name",
		Name:        "name",
		Purpose:     "The channel purpose",
		Header:      "The channel header",
	}

	c1s := slackSanitiseChannelProperties(c1)
	assert.Equal(t, c1, c1s)

	c2 := model.Channel{
		DisplayName: strings.Repeat("abcdefghij", 7),
		Name:        strings.Repeat("abcdefghij", 7),
		Purpose:     strings.Repeat("0123456789", 30),
		Header:      strings.Repeat("0123456789", 120),
	}

	c2s := slackSanitiseChannelProperties(c2)
	assert.Equal(t, model.Channel{
		DisplayName: strings.Repeat("abcdefghij", 6) + "abcd",
		Name:        strings.Repeat("abcdefghij", 6) + "abcd",
		Purpose:     strings.Repeat("0123456789", 25),
		Header:      strings.Repeat("0123456789", 102) + "0123",
	}, c2s)
}

func TestSlackConvertPostsMarkup(t *testing.T) {
	input := make(map[string][]slackPost)
	input["test"] = []slackPost{
		{
			Text: "This message contains a link to <https://google.com|Google>.",
		},
		{
			Text: "This message contains a mailto link to <mailto:me@example.com|me@example.com> in it.",
		},
		{
			Text: "This message contains a *bold* word.",
		},
		{
			Text: "This is not a * bold * word.",
		},
		{
			Text: `There is *no bold word
in this*.`,
		},
		{
			Text: "*This* is not a*bold* word.*This* is a bold word, *and* this; *and* this too.",
		},
		{
			Text: "This message contains a ~strikethrough~ word.",
		},
		{
			Text: "This is not a ~ strikethrough ~ word.",
		},
		{
			Text: `There is ~no strikethrough word
in this~.`,
		},
		{
			Text: "~This~ is not a~strikethrough~ word.~This~ is a strikethrough word, ~and~ this; ~and~ this too.",
		},
		{
			Text: `This message contains multiple paragraphs blockquotes
&gt;&gt;&gt;first
second
third`,
		},
		{
			Text: `This message contains single paragraph blockquotes
&gt;something
&gt;another thing`,
		},
		{
			Text: "This message has no > block quote",
		},
	}

	expectedOutput := make(map[string][]slackPost)
	expectedOutput["test"] = []slackPost{
		{
			Text: "This message contains a link to [Google](https://google.com).",
		},
		{
			Text: "This message contains a mailto link to [me@example.com](mailto:me@example.com) in it.",
		},
		{
			Text: "This message contains a **bold** word.",
		},
		{
			Text: "This is not a * bold * word.",
		},
		{
			Text: `There is *no bold word
in this*.`,
		},
		{
			Text: "**This** is not a*bold* word.**This** is a bold word, **and** this; **and** this too.",
		},
		{
			Text: "This message contains a ~~strikethrough~~ word.",
		},
		{
			Text: "This is not a ~ strikethrough ~ word.",
		},
		{
			Text: `There is ~no strikethrough word
in this~.`,
		},
		{
			Text: "~~This~~ is not a~strikethrough~ word.~~This~~ is a strikethrough word, ~~and~~ this; ~~and~~ this too.",
		},
		{
			Text: `This message contains multiple paragraphs blockquotes
>first
>second
>third`,
		},
		{
			Text: `This message contains single paragraph blockquotes
>something
>another thing`,
		},
		{
			Text: "This message has no > block quote",
		},
	}

	assert.Equal(t, expectedOutput, slackConvertPostsMarkup(input))
}

func TestOldImportChannel(t *testing.T) {
	u1 := &model.User{
		ID:       model.NewID(),
		Username: "test-user-1",
	}
	u2 := &model.User{
		ID:       model.NewID(),
		Username: "test-user-2",
	}
	store := &mocks.Store{}
	config := &model.Config{}
	config.SetDefaults()

	t.Run("No panic on direct channel", func(t *testing.T) {
		//ch := th.CreateDmChannel(u1)
		ch := &model.Channel{
			Type: model.ChannelTypeDirect,
			Name: "test-channel",
		}
		users := map[string]*model.User{
			u2.ID: u2,
		}
		sCh := slackChannel{
			ID:      "someid",
			Members: []string{u1.ID, "randomID"},
			Creator: "randomID2",
		}

		actions := Actions{}

		importer := New(store, actions, config)
		_ = importer.oldImportChannel(ch, sCh, users)
	})

	t.Run("No panic on direct channel with 1 member", func(t *testing.T) {
		ch := &model.Channel{
			Type: model.ChannelTypeDirect,
			Name: "test-channel",
		}
		users := map[string]*model.User{
			u1.ID: u1,
		}
		sCh := slackChannel{
			ID:      "someid",
			Members: []string{u1.ID},
			Creator: "randomID2",
		}

		actions := Actions{}

		importer := New(store, actions, config)
		_ = importer.oldImportChannel(ch, sCh, users)
	})

	t.Run("No panic on group channel", func(t *testing.T) {
		ch := &model.Channel{
			Type: model.ChannelTypeGroup,
			Name: "test-channel",
		}
		users := map[string]*model.User{
			u1.ID: u1,
		}
		sCh := slackChannel{
			ID:      "someid",
			Members: []string{u1.ID},
			Creator: "randomID2",
		}
		actions := Actions{}

		importer := New(store, actions, config)
		_ = importer.oldImportChannel(ch, sCh, users)
	})
}
