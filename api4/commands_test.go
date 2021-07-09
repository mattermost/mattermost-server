// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"strings"
	"testing"
	"time"

	_ "github.com/mattermost/mattermost-server/v5/app/slashcommands"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestEchoCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel1 := th.BasicChannel

	echoTestString := "/echo test"

	r1 := Client.Must(Client.ExecuteCommand(channel1.ID, echoTestString)).(*model.CommandResponse)
	require.NotNil(t, r1, "Echo command failed to execute")

	r1 = Client.Must(Client.ExecuteCommand(channel1.ID, "/echo ")).(*model.CommandResponse)
	require.NotNil(t, r1, "Echo command failed to execute")

	time.Sleep(100 * time.Millisecond)

	p1 := Client.Must(Client.GetPostsForChannel(channel1.ID, 0, 2, "", false)).(*model.PostList)
	require.Len(t, p1.Order, 2, "Echo command failed to send")
}

func TestGroupmsgCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	team := th.BasicTeam
	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()
	user4 := th.CreateUser()
	user5 := th.CreateUser()
	user6 := th.CreateUser()
	user7 := th.CreateUser()
	user8 := th.CreateUser()
	user9 := th.CreateUser()
	th.LinkUserToTeam(user3, team)
	th.LinkUserToTeam(user4, team)

	rs1 := Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/groupmsg "+user2.Username+","+user3.Username)).(*model.CommandResponse)

	group1 := model.GetGroupNameFromUserIDs([]string{user1.ID, user2.ID, user3.ID})
	require.True(t, strings.HasSuffix(rs1.GotoLocation, "/"+team.Name+"/channels/"+group1), "failed to create group channel")

	rs2 := Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/groupmsg "+user3.Username+","+user4.Username+" foobar")).(*model.CommandResponse)
	group2 := model.GetGroupNameFromUserIDs([]string{user1.ID, user3.ID, user4.ID})

	require.True(t, strings.HasSuffix(rs2.GotoLocation, "/"+team.Name+"/channels/"+group2), "failed to create second direct channel")

	result := Client.Must(Client.SearchPosts(team.ID, "foobar", false)).(*model.PostList)
	require.NotEqual(t, 0, len(result.Order), "post did not get sent to direct message")

	rs3 := Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/groupmsg "+user2.Username+","+user3.Username)).(*model.CommandResponse)
	require.True(t, strings.HasSuffix(rs3.GotoLocation, "/"+team.Name+"/channels/"+group1), "failed to go back to existing group channel")

	Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/groupmsg "+user2.Username+" foobar"))
	Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/groupmsg "+user2.Username+","+user3.Username+","+user4.Username+","+user5.Username+","+user6.Username+","+user7.Username+","+user8.Username+","+user9.Username+" foobar"))
	Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/groupmsg junk foobar"))
	Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/groupmsg junk,junk2 foobar"))
}

func TestInvitePeopleCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	r1 := Client.Must(Client.ExecuteCommand(channel.ID, "/invite_people test@example.com")).(*model.CommandResponse)
	require.NotNil(t, r1, "Command failed to execute")

	r2 := Client.Must(Client.ExecuteCommand(channel.ID, "/invite_people test1@example.com test2@example.com")).(*model.CommandResponse)
	require.NotNil(t, r2, "Command failed to execute")

	r3 := Client.Must(Client.ExecuteCommand(channel.ID, "/invite_people")).(*model.CommandResponse)
	require.NotNil(t, r3, "Command failed to execute")
}

// also used to test /open (see command_open_test.go)
func testJoinCommands(t *testing.T, alias string) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	team := th.BasicTeam
	user2 := th.BasicUser2

	channel0 := &model.Channel{DisplayName: "00", Name: "00" + model.NewID() + "a", Type: model.ChannelTypeOpen, TeamID: team.ID}
	channel0 = Client.Must(Client.CreateChannel(channel0)).(*model.Channel)

	channel1 := &model.Channel{DisplayName: "AA", Name: "aa" + model.NewID() + "a", Type: model.ChannelTypeOpen, TeamID: team.ID}
	channel1 = Client.Must(Client.CreateChannel(channel1)).(*model.Channel)
	Client.Must(Client.RemoveUserFromChannel(channel1.ID, th.BasicUser.ID))

	channel2 := &model.Channel{DisplayName: "BB", Name: "bb" + model.NewID() + "a", Type: model.ChannelTypeOpen, TeamID: team.ID}
	channel2 = Client.Must(Client.CreateChannel(channel2)).(*model.Channel)
	Client.Must(Client.RemoveUserFromChannel(channel2.ID, th.BasicUser.ID))

	channel3 := Client.Must(Client.CreateDirectChannel(th.BasicUser.ID, user2.ID)).(*model.Channel)

	rs5 := Client.Must(Client.ExecuteCommand(channel0.ID, "/"+alias+" "+channel2.Name)).(*model.CommandResponse)
	require.True(t, strings.HasSuffix(rs5.GotoLocation, "/"+team.Name+"/channels/"+channel2.Name), "failed to join channel")

	rs6 := Client.Must(Client.ExecuteCommand(channel0.ID, "/"+alias+" "+channel3.Name)).(*model.CommandResponse)
	require.False(t, strings.HasSuffix(rs6.GotoLocation, "/"+team.Name+"/channels/"+channel3.Name), "should not have joined direct message channel")

	c1 := Client.Must(Client.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser.ID, false, "")).([]*model.Channel)

	found := false
	for _, c := range c1 {
		if c.ID == channel2.ID {
			found = true
		}
	}
	require.True(t, found, "did not join channel")

	// test case insensitively
	channel4 := &model.Channel{DisplayName: "BB", Name: "bb" + model.NewID() + "a", Type: model.ChannelTypeOpen, TeamID: team.ID}
	channel4 = Client.Must(Client.CreateChannel(channel4)).(*model.Channel)
	Client.Must(Client.RemoveUserFromChannel(channel4.ID, th.BasicUser.ID))
	rs7 := Client.Must(Client.ExecuteCommand(channel0.ID, "/"+alias+" "+strings.ToUpper(channel4.Name))).(*model.CommandResponse)
	require.True(t, strings.HasSuffix(rs7.GotoLocation, "/"+team.Name+"/channels/"+channel4.Name), "failed to join channel")
}

func TestJoinCommands(t *testing.T) {
	testJoinCommands(t, "join")
}

func TestLoadTestHelpCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	enableTesting := *th.App.Config().ServiceSettings.EnableTesting
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = enableTesting })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = true })

	rs := Client.Must(Client.ExecuteCommand(channel.ID, "/test help")).(*model.CommandResponse)
	require.True(t, strings.Contains(rs.Text, "Mattermost testing commands to help"), rs.Text)

	time.Sleep(2 * time.Second)
}

func TestLoadTestSetupCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	enableTesting := *th.App.Config().ServiceSettings.EnableTesting
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = enableTesting })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = true })

	rs := Client.Must(Client.ExecuteCommand(channel.ID, "/test setup fuzz 1 1 1")).(*model.CommandResponse)
	require.Equal(t, "Created environment", rs.Text, rs.Text)

	time.Sleep(2 * time.Second)
}

func TestLoadTestUsersCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	enableTesting := *th.App.Config().ServiceSettings.EnableTesting
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = enableTesting })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = true })

	rs := Client.Must(Client.ExecuteCommand(channel.ID, "/test users fuzz 1 2")).(*model.CommandResponse)
	require.Equal(t, "Added users", rs.Text, rs.Text)

	time.Sleep(2 * time.Second)
}

func TestLoadTestChannelsCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	enableTesting := *th.App.Config().ServiceSettings.EnableTesting
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = enableTesting })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = true })

	rs := Client.Must(Client.ExecuteCommand(channel.ID, "/test channels fuzz 1 2")).(*model.CommandResponse)
	require.Equal(t, "Added channels", rs.Text, rs.Text)

	time.Sleep(2 * time.Second)
}

func TestLoadTestPostsCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	enableTesting := *th.App.Config().ServiceSettings.EnableTesting
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = enableTesting })
	}()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableTesting = true })

	rs := Client.Must(Client.ExecuteCommand(channel.ID, "/test posts fuzz 2 3 2")).(*model.CommandResponse)
	require.Equal(t, "Added posts", rs.Text, rs.Text)

	time.Sleep(2 * time.Second)
}

func TestLeaveCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	team := th.BasicTeam
	user2 := th.BasicUser2

	channel1 := &model.Channel{DisplayName: "AA", Name: "aa" + model.NewID() + "a", Type: model.ChannelTypeOpen, TeamID: team.ID}
	channel1 = Client.Must(Client.CreateChannel(channel1)).(*model.Channel)
	Client.Must(Client.AddChannelMember(channel1.ID, th.BasicUser.ID))

	channel2 := &model.Channel{DisplayName: "BB", Name: "bb" + model.NewID() + "a", Type: model.ChannelTypePrivate, TeamID: team.ID}
	channel2 = Client.Must(Client.CreateChannel(channel2)).(*model.Channel)
	Client.Must(Client.AddChannelMember(channel2.ID, th.BasicUser.ID))
	Client.Must(Client.AddChannelMember(channel2.ID, user2.ID))

	channel3 := Client.Must(Client.CreateDirectChannel(th.BasicUser.ID, user2.ID)).(*model.Channel)

	rs1 := Client.Must(Client.ExecuteCommand(channel1.ID, "/leave")).(*model.CommandResponse)
	require.True(t, strings.HasSuffix(rs1.GotoLocation, "/"+team.Name+"/channels/"+model.DefaultChannelName), "failed to leave open channel 1")

	rs2 := Client.Must(Client.ExecuteCommand(channel2.ID, "/leave")).(*model.CommandResponse)
	require.True(t, strings.HasSuffix(rs2.GotoLocation, "/"+team.Name+"/channels/"+model.DefaultChannelName), "failed to leave private channel 1")

	_, err := Client.ExecuteCommand(channel3.ID, "/leave")
	require.NotNil(t, err, "should fail leaving direct channel")

	cdata := Client.Must(Client.GetChannelsForTeamForUser(th.BasicTeam.ID, th.BasicUser.ID, false, "")).([]*model.Channel)

	found := false
	for _, c := range cdata {
		if c.ID == channel1.ID || c.ID == channel2.ID {
			found = true
		}
	}
	require.False(t, found, "did not leave right channels")

	for _, c := range cdata {
		if c.Name == model.DefaultChannelName {
			_, err := Client.RemoveUserFromChannel(c.ID, th.BasicUser.ID)
			require.NotNil(t, err, "should have errored on leaving default channel")
			break
		}
	}
}

func TestLogoutTestCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.Client.Must(th.Client.ExecuteCommand(th.BasicChannel.ID, "/logout"))
}

func TestMeCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	testString := "/me hello"

	r1 := Client.Must(Client.ExecuteCommand(channel.ID, testString)).(*model.CommandResponse)
	require.NotNil(t, r1, "Command failed to execute")

	time.Sleep(100 * time.Millisecond)

	p1 := Client.Must(Client.GetPostsForChannel(channel.ID, 0, 2, "", false)).(*model.PostList)
	require.Len(t, p1.Order, 2, "Command failed to send")

	pt := p1.Posts[p1.Order[0]].Type
	require.Equal(t, model.PostTypeMe, pt, "invalid post type")

	msg := p1.Posts[p1.Order[0]].Message
	want := "*hello*"
	require.Equal(t, want, msg, "invalid me response")
}

func TestMsgCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	team := th.BasicTeam
	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()
	th.LinkUserToTeam(user3, team)

	Client.Must(Client.CreateDirectChannel(th.BasicUser.ID, user2.ID))
	Client.Must(Client.CreateDirectChannel(th.BasicUser.ID, user3.ID))

	rs1 := Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/msg "+user2.Username)).(*model.CommandResponse)
	require.Condition(t, func() bool {
		return strings.HasSuffix(rs1.GotoLocation, "/"+team.Name+"/channels/"+user1.ID+"__"+user2.ID) ||
			strings.HasSuffix(rs1.GotoLocation, "/"+team.Name+"/channels/"+user2.ID+"__"+user1.ID)
	}, "failed to create direct channel")

	rs2 := Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/msg "+user3.Username+" foobar")).(*model.CommandResponse)
	require.Condition(t, func() bool {
		return strings.HasSuffix(rs2.GotoLocation, "/"+team.Name+"/channels/"+user1.ID+"__"+user3.ID) ||
			strings.HasSuffix(rs2.GotoLocation, "/"+team.Name+"/channels/"+user3.ID+"__"+user1.ID)
	}, "failed to create second direct channel")

	result := Client.Must(Client.SearchPosts(th.BasicTeam.ID, "foobar", false)).(*model.PostList)
	require.NotEqual(t, 0, len(result.Order), "post did not get sent to direct message")

	rs3 := Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/msg "+user2.Username)).(*model.CommandResponse)
	require.Condition(t, func() bool {
		return strings.HasSuffix(rs3.GotoLocation, "/"+team.Name+"/channels/"+user1.ID+"__"+user2.ID) ||
			strings.HasSuffix(rs3.GotoLocation, "/"+team.Name+"/channels/"+user2.ID+"__"+user1.ID)
	}, "failed to go back to existing direct channel")

	Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/msg "+th.BasicUser.Username+" foobar"))
	Client.Must(Client.ExecuteCommand(th.BasicChannel.ID, "/msg junk foobar"))
}

func TestOpenCommands(t *testing.T) {
	testJoinCommands(t, "open")
}

func TestSearchCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.Client.Must(th.Client.ExecuteCommand(th.BasicChannel.ID, "/search"))
}

func TestSettingsCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.Client.Must(th.Client.ExecuteCommand(th.BasicChannel.ID, "/settings"))
}

func TestShortcutsCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.Client.Must(th.Client.ExecuteCommand(th.BasicChannel.ID, "/shortcuts"))
}

func TestShrugCommand(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	testString := "/shrug"

	r1 := Client.Must(Client.ExecuteCommand(channel.ID, testString)).(*model.CommandResponse)
	require.NotNil(t, r1, "Command failed to execute")

	time.Sleep(100 * time.Millisecond)

	p1 := Client.Must(Client.GetPostsForChannel(channel.ID, 0, 2, "", false)).(*model.PostList)
	require.Len(t, p1.Order, 2, "Command failed to send")
	require.Equal(t, `¯\\\_(ツ)\_/¯`, p1.Posts[p1.Order[0]].Message, "invalid shrug response")
}

func TestStatusCommands(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	commandAndTest(t, th, "away")
	commandAndTest(t, th, "offline")
	commandAndTest(t, th, "online")
}

func commandAndTest(t *testing.T, th *TestHelper, status string) {
	Client := th.Client
	channel := th.BasicChannel
	user := th.BasicUser

	r1 := Client.Must(Client.ExecuteCommand(channel.ID, "/"+status)).(*model.CommandResponse)
	require.NotEqual(t, "Command failed to execute", r1)

	time.Sleep(1000 * time.Millisecond)

	rstatus := Client.Must(Client.GetUserStatus(user.ID, "")).(*model.Status)
	require.Equal(t, status, rstatus.Status, "Error setting status")
}
