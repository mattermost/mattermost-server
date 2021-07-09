// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
)

func TestMuteCommandNoChannel(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	if testing.Short() {
		t.SkipNow()
	}

	channel1 := th.BasicChannel
	channel1M, channel1MError := th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)

	assert.Nil(t, channel1MError, "User is not a member of channel 1")
	assert.NotEqual(
		t,
		channel1M.NotifyProps[model.MarkUnreadNotifyProp],
		model.ChannelNotifyMention,
		"Channel shouldn't be muted on initial setup",
	)

	cmd := &MuteProvider{}
	resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:      i18n.IdentityTfunc(),
		UserID: th.BasicUser.ID,
	}, "")
	assert.Equal(t, "api.command_mute.no_channel.error", resp.Text)
}

func TestMuteCommandNoArgs(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	channel1 := th.BasicChannel
	channel1M, _ := th.App.GetChannelMember(context.Background(), channel1.ID, th.BasicUser.ID)

	assert.Equal(t, model.ChannelNotifyAll, channel1M.NotifyProps[model.MarkUnreadNotifyProp])

	cmd := &MuteProvider{}

	// First mute the channel
	resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:         i18n.IdentityTfunc(),
		ChannelID: channel1.ID,
		UserID:    th.BasicUser.ID,
	}, "")
	assert.Equal(t, "api.command_mute.success_mute", resp.Text)

	// Now unmute the channel
	time.Sleep(time.Millisecond)
	resp = cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:         i18n.IdentityTfunc(),
		ChannelID: channel1.ID,
		UserID:    th.BasicUser.ID,
	}, "")

	assert.Equal(t, "api.command_mute.success_unmute", resp.Text)
}

func TestMuteCommandSpecificChannel(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	if testing.Short() {
		t.SkipNow()
	}

	channel1 := th.BasicChannel
	channel2, _ := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "AA",
		Name:        "aa" + model.NewID() + "a",
		Type:        model.ChannelTypeOpen,
		TeamID:      th.BasicTeam.ID,
		CreatorID:   th.BasicUser.ID,
	}, true)

	channel2M, _ := th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)

	assert.Equal(t, model.ChannelNotifyAll, channel2M.NotifyProps[model.MarkUnreadNotifyProp])

	cmd := &MuteProvider{}

	// First mute the channel
	resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:         i18n.IdentityTfunc(),
		ChannelID: channel1.ID,
		UserID:    th.BasicUser.ID,
	}, channel2.Name)
	assert.Equal(t, "api.command_mute.success_mute", resp.Text)
	channel2M, _ = th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
	assert.Equal(t, model.ChannelNotifyMention, channel2M.NotifyProps[model.MarkUnreadNotifyProp])

	// Now unmute the channel
	resp = cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:         i18n.IdentityTfunc(),
		ChannelID: channel1.ID,
		UserID:    th.BasicUser.ID,
	}, "~"+channel2.Name)

	assert.Equal(t, "api.command_mute.success_unmute", resp.Text)
	channel2M, _ = th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
	assert.Equal(t, model.ChannelNotifyAll, channel2M.NotifyProps[model.MarkUnreadNotifyProp])
}

func TestMuteCommandNotMember(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	if testing.Short() {
		t.SkipNow()
	}

	channel1 := th.BasicChannel
	channel2, _ := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "AA",
		Name:        "aa" + model.NewID() + "a",
		Type:        model.ChannelTypeOpen,
		TeamID:      th.BasicTeam.ID,
		CreatorID:   th.BasicUser.ID,
	}, false)

	cmd := &MuteProvider{}

	// First mute the channel
	resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:         i18n.IdentityTfunc(),
		ChannelID: channel1.ID,
		UserID:    th.BasicUser.ID,
	}, channel2.Name)
	assert.Equal(t, "api.command_mute.not_member.error", resp.Text)
}

func TestMuteCommandNotChannel(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	if testing.Short() {
		t.SkipNow()
	}

	channel1 := th.BasicChannel

	cmd := &MuteProvider{}

	// First mute the channel
	resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:         i18n.IdentityTfunc(),
		ChannelID: channel1.ID,
		UserID:    th.BasicUser.ID,
	}, "~noexists")
	assert.Equal(t, "api.command_mute.error", resp.Text)
}

func TestMuteCommandDMChannel(t *testing.T) {
	th := setup(t).initBasic()
	defer th.tearDown()

	if testing.Short() {
		t.SkipNow()
	}

	channel2, _ := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.ID, th.BasicUser2.ID)
	channel2M, _ := th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)

	assert.Equal(t, model.ChannelNotifyAll, channel2M.NotifyProps[model.MarkUnreadNotifyProp])

	cmd := &MuteProvider{}

	// First mute the channel
	resp := cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:         i18n.IdentityTfunc(),
		ChannelID: channel2.ID,
		UserID:    th.BasicUser.ID,
	}, "")
	assert.Equal(t, "api.command_mute.success_mute_direct_msg", resp.Text)
	time.Sleep(time.Millisecond)
	channel2M, _ = th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
	assert.Equal(t, model.ChannelNotifyMention, channel2M.NotifyProps[model.MarkUnreadNotifyProp])

	// Now unmute the channel
	resp = cmd.DoCommand(th.App, th.Context, &model.CommandArgs{
		T:         i18n.IdentityTfunc(),
		ChannelID: channel2.ID,
		UserID:    th.BasicUser.ID,
	}, "")

	assert.Equal(t, "api.command_mute.success_unmute_direct_msg", resp.Text)
	time.Sleep(time.Millisecond)
	channel2M, _ = th.App.GetChannelMember(context.Background(), channel2.ID, th.BasicUser.ID)
	assert.Equal(t, model.ChannelNotifyAll, channel2M.NotifyProps[model.MarkUnreadNotifyProp])
}
