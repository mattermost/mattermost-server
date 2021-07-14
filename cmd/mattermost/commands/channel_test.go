// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestJoinChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.CreatePublicChannel()

	th.CheckCommand(t, "channel", "add", th.BasicTeam.Name+":"+channel.Name, th.BasicUser2.Email)

	// Joining twice should succeed
	th.CheckCommand(t, "channel", "add", th.BasicTeam.Name+":"+channel.Name, th.BasicUser2.Email)

	// should fail because channel does not exist
	require.Error(t, th.RunCommand(t, "channel", "add", th.BasicTeam.Name+":"+channel.Name+"asdf", th.BasicUser2.Email))
}

func TestRemoveChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.CreatePublicChannel()

	t.Run("should fail because channel does not exist", func(t *testing.T) {
		require.Error(t, th.RunCommand(t, "channel", "remove", th.BasicTeam.Name+":doesnotexist", th.BasicUser2.Email))
	})

	t.Run("should remove user from channel", func(t *testing.T) {
		th.CheckCommand(t, "channel", "add", th.BasicTeam.Name+":"+channel.Name, th.BasicUser2.Email)
		isMember, _ := th.App.UserBelongsToChannels(th.BasicUser2.Id, []string{channel.Id})
		assert.True(t, isMember)

		th.CheckCommand(t, "channel", "remove", th.BasicTeam.Name+":"+channel.Name, th.BasicUser2.Email)
		isMember, _ = th.App.UserBelongsToChannels(th.BasicUser2.Id, []string{channel.Id})
		assert.False(t, isMember)
	})

	t.Run("should not fail removing non member user from channel", func(t *testing.T) {
		isMember, _ := th.App.UserBelongsToChannels(th.BasicUser2.Id, []string{channel.Id})
		assert.False(t, isMember)
		th.CheckCommand(t, "channel", "remove", th.BasicTeam.Name+":"+channel.Name, th.BasicUser2.Email)
	})

	t.Run("should throw error if both --all-users flag and user email are passed", func(t *testing.T) {
		require.Error(t, th.RunCommand(t, "channel", "remove", "--all-users", th.BasicUser.Email))
	})

	t.Run("should remove all users from channel", func(t *testing.T) {
		th.CheckCommand(t, "channel", "add", th.BasicTeam.Name+":"+channel.Name, th.BasicUser.Email)
		th.CheckCommand(t, "channel", "add", th.BasicTeam.Name+":"+channel.Name, th.BasicUser2.Email)
		count, _ := th.App.GetChannelMemberCount(channel.Id)
		assert.Equal(t, count, int64(2))

		th.CheckCommand(t, "channel", "remove", th.BasicTeam.Name+":"+channel.Name, "--all-users")
		th.App.ClearChannelCaches()
		count, _ = th.App.GetChannelMemberCount(channel.Id)
		assert.Equal(t, count, int64(0))
	})

	t.Run("should remove multiple users from channel", func(t *testing.T) {
		th.CheckCommand(t, "channel", "add", th.BasicTeam.Name+":"+channel.Name, th.BasicUser.Email)
		th.CheckCommand(t, "channel", "add", th.BasicTeam.Name+":"+channel.Name, th.BasicUser2.Email)
		th.App.ClearChannelCaches()
		count, _ := th.App.GetChannelMemberCount(channel.Id)
		assert.Equal(t, count, int64(2))

		th.CheckCommand(t, "channel", "remove", th.BasicTeam.Name+":"+channel.Name, th.BasicUser.Email, th.BasicUser2.Email)
		th.App.ClearChannelCaches()
		count, _ = th.App.GetChannelMemberCount(channel.Id)
		assert.Equal(t, count, int64(0))
	})
}

func TestMoveChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	team1 := th.BasicTeam
	team2 := th.CreateTeam()
	user1 := th.BasicUser
	th.LinkUserToTeam(user1, team2)
	channel := th.BasicChannel

	th.LinkUserToTeam(user1, team1)
	th.LinkUserToTeam(user1, team2)

	adminEmail := user1.Email
	adminUsername := user1.Username
	origin := team1.Name + ":" + channel.Name
	dest := team2.Name

	th.CheckCommand(t, "channel", "add", origin, adminEmail)

	// should fail with nil because errors are logged instead of returned when a channel does not exist
	th.CheckCommand(t, "channel", "move", dest, team1.Name+":doesnotexist", "--username", adminUsername)

	th.CheckCommand(t, "channel", "move", dest, origin, "--username", adminUsername)
}

func TestListChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.CreatePublicChannel()
	th.Client.Must(th.Client.DeleteChannel(channel.Id))
	privateChannel := th.CreatePrivateChannel()

	output := th.CheckCommand(t, "channel", "list", th.BasicTeam.Name)

	require.True(t, strings.Contains(output, "town-square"), "should have channels")

	require.True(t, strings.Contains(output, channel.Name+" (archived)"), "should have archived channel")

	require.True(t, strings.Contains(output, privateChannel.Name+" (private)"), "should have private channel")

	th.Client.Must(th.Client.DeleteChannel(privateChannel.Id))

	output = th.CheckCommand(t, "channel", "list", th.BasicTeam.Name)

	require.True(t, strings.Contains(output, privateChannel.Name+" (archived) (private)"), "should have a channel both archived and private")
}

func TestRestoreChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.CreatePublicChannel()
	th.Client.Must(th.Client.DeleteChannel(channel.Id))

	th.CheckCommand(t, "channel", "restore", th.BasicTeam.Name+":"+channel.Name)

	// restoring twice should succeed
	th.CheckCommand(t, "channel", "restore", th.BasicTeam.Name+":"+channel.Name)
}

func TestCreateChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewId()
	commonName := "name" + id
	team, _ := th.App.Srv().Store.Team().GetByName(th.BasicTeam.Name)

	t.Run("should create public channel", func(t *testing.T) {
		th.CheckCommand(t, "channel", "create", "--display_name", commonName, "--team", th.BasicTeam.Name, "--name", commonName)
		channel, err := th.App.GetChannelByName(commonName, team.Id, false)
		require.Nil(t, err)
		assert.Equal(t, commonName, channel.Name)
		assert.Equal(t, model.CHANNEL_OPEN, channel.Type)
	})

	t.Run("should create private channel", func(t *testing.T) {
		name := commonName + "-private"
		th.CheckCommand(t, "channel", "create", "--display_name", name, "--team", th.BasicTeam.Name, "--name", name, "--private")
		channel, err := th.App.GetChannelByName(name, team.Id, false)
		require.Nil(t, err)
		assert.Equal(t, name, channel.Name)
		assert.Equal(t, model.CHANNEL_PRIVATE, channel.Type)
	})

	t.Run("should create channel with header and purpose", func(t *testing.T) {
		name := commonName + "-withhp"
		th.CheckCommand(t, "channel", "create", "--display_name", name, "--team", th.BasicTeam.Name, "--name", name, "--header", "this is a header", "--purpose", "this is the purpose")
		channel, err := th.App.GetChannelByName(name, team.Id, false)
		require.Nil(t, err)
		assert.Equal(t, name, channel.Name)
		assert.Equal(t, model.CHANNEL_OPEN, channel.Type)
		assert.Equal(t, "this is a header", channel.Header)
		assert.Equal(t, "this is the purpose", channel.Purpose)
	})

	t.Run("should not create channel if name already exists on the same team", func(t *testing.T) {
		output, err := th.RunCommandWithOutput(t, "channel", "create", "--display_name", commonName, "--team", th.BasicTeam.Name, "--name", commonName)
		require.Error(t, err)
		require.Contains(t, output, "A channel with that name already exists on the same team.")
	})

	t.Run("should not create channel without display name", func(t *testing.T) {
		output, err := th.RunCommandWithOutput(t, "channel", "create", "--display_name", "", "--team", th.BasicTeam.Name, "--name", commonName)
		require.Error(t, err)
		require.Contains(t, output, "Display Name is required")
	})

	t.Run("should not create channel without name", func(t *testing.T) {
		output, err := th.RunCommandWithOutput(t, "channel", "create", "--display_name", commonName, "--team", th.BasicTeam.Name, "--name", "")
		require.Error(t, err)
		require.Contains(t, output, "Name is required")
	})

	t.Run("should not create channel without team", func(t *testing.T) {
		output, err := th.RunCommandWithOutput(t, "channel", "create", "--display_name", commonName, "--team", "", "--name", commonName)
		require.Error(t, err)
		require.Contains(t, output, "Team is required")
	})

	t.Run("should not create channel with unexisting team", func(t *testing.T) {
		output, err := th.RunCommandWithOutput(t, "channel", "create", "--display_name", commonName, "--team", th.BasicTeam.Name+"-unexisting", "--name", commonName)
		require.Error(t, err)
		require.Contains(t, output, "Unable to find team:")
	})
}

func TestRenameChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.CreatePublicChannel()
	th.CheckCommand(t, "channel", "rename", th.BasicTeam.Name+":"+channel.Name, "newchannelname10", "--display_name", "New Display Name")

	// Get the channel from the DB
	updatedChannel, _ := th.App.GetChannel(channel.Id)
	assert.Equal(t, "newchannelname10", updatedChannel.Name)
	assert.Equal(t, "New Display Name", updatedChannel.DisplayName)
}

func Test_searchChannelCmdF(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.CreatePublicChannel()
	channel2 := th.CreatePublicChannel()
	channel3 := th.CreatePrivateChannel()
	channel4 := th.CreatePrivateChannel()
	th.Client.DeleteChannel(channel2.Id)
	th.Client.DeleteChannel(channel4.Id)

	tests := []struct {
		Name     string
		Args     []string
		Expected string
	}{
		{
			"Success find Channel in any team",
			[]string{"channel", "search", channel.Name},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s", channel.Name, channel.DisplayName, channel.Id),
		},
		{
			"Failed find Channel in any team",
			[]string{"channel", "search", channel.Name + "404"},
			fmt.Sprintf("Channel %s is not found in any team", channel.Name+"404"),
		},
		{
			"Success find Channel with param team ID",
			[]string{"channel", "search", "--team", channel.TeamId, channel.Name},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s", channel.Name, channel.DisplayName, channel.Id),
		},
		{
			"Failed find Channel with param team ID",
			[]string{"channel", "search", "--team", channel.TeamId, channel.Name + "404"},
			fmt.Sprintf("Channel %s is not found in team %s", channel.Name+"404", channel.TeamId),
		},
		{
			"Success find archived Channel in any team",
			[]string{"channel", "search", channel2.Name},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s (archived)", channel2.Name, channel2.DisplayName, channel2.Id),
		},
		{
			"Success find archived Channel with param team ID",
			[]string{"channel", "search", "--team", channel2.TeamId, channel2.Name},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s (archived)", channel2.Name, channel2.DisplayName, channel2.Id),
		},
		{
			"Success find private Channel in any team",
			[]string{"channel", "search", channel3.Name},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s (private)", channel3.Name, channel3.DisplayName, channel3.Id),
		},
		{
			"Success find private Channel with param team ID",
			[]string{"channel", "search", "--team", channel3.TeamId, channel3.Name},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s (private)", channel3.Name, channel3.DisplayName, channel3.Id),
		},
		{
			"Success find both archived and private Channel in any team",
			[]string{"channel", "search", channel4.Name},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s (archived) (private)", channel4.Name, channel4.DisplayName, channel4.Id),
		},
		{
			"Success find both archived and private Channel with param team ID",
			[]string{"channel", "search", "--team", channel4.TeamId, channel4.Name},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s (archived) (private)", channel4.Name, channel4.DisplayName, channel4.Id),
		},
		{
			"Failed find team",
			[]string{"channel", "search", "--team", channel.TeamId + "404", channel.Name},
			fmt.Sprintf("Team %s is not found", channel.TeamId+"404"),
		},
		{
			"Success find Channel with param team ID",
			[]string{"channel", "search", channel.Name, "--team", channel.TeamId},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s", channel.Name, channel.DisplayName, channel.Id),
		},
		{
			"Success find Channel with param team ID",
			[]string{"channel", "search", channel.Name, "--team=" + channel.TeamId},
			fmt.Sprintf("Channel Name: %s, Display Name: %s, Channel ID: %s", channel.Name, channel.DisplayName, channel.Id),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert.Contains(t, th.CheckCommand(t, test.Args...), test.Expected)
		})
	}
}

func TestModifyChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel1 := th.CreatePrivateChannel()
	channel2 := th.CreatePrivateChannel()

	th.CheckCommand(t, "channel", "modify", "--public", th.BasicTeam.Name+":"+channel1.Name, "--username", th.BasicUser2.Email)
	res, err := th.App.GetChannel(channel1.Id)
	require.Nil(t, err)
	assert.Equal(t, model.CHANNEL_OPEN, res.Type)

	// should fail because user doesn't exist
	require.Error(t, th.RunCommand(t, "channel", "modify", "--public", th.BasicTeam.Name+":"+channel2.Name, "--username", "idonotexist"))

	pchannel1 := th.CreatePublicChannel()
	pchannel2 := th.CreatePublicChannel()

	th.CheckCommand(t, "channel", "modify", "--private", th.BasicTeam.Name+":"+pchannel1.Name, "--username", th.BasicUser2.Email)
	res, err = th.App.GetChannel(pchannel1.Id)
	require.Nil(t, err)
	assert.Equal(t, model.CHANNEL_PRIVATE, res.Type)

	// should fail because user doesn't exist
	require.Error(t, th.RunCommand(t, "channel", "modify", "--private", th.BasicTeam.Name+":"+pchannel2.Name, "--username", "idonotexist"))
}
