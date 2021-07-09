// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"strings"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
)

type JoinProvider struct {
}

const (
	CmdJoin = "join"
)

func init() {
	app.RegisterCommandProvider(&JoinProvider{})
}

func (*JoinProvider) GetTrigger() string {
	return CmdJoin
}

func (*JoinProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdJoin,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_join.desc"),
		AutoCompleteHint: T("api.command_join.hint"),
		DisplayName:      T("api.command_join.name"),
	}
}

func (*JoinProvider) DoCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	channelName := strings.ToLower(message)

	if strings.HasPrefix(message, "~") {
		channelName = message[1:]
	}

	channel, err := a.Srv().Store.Channel().GetByName(args.TeamID, channelName, true)
	if err != nil {
		return &model.CommandResponse{Text: args.T("api.command_join.list.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	if channel.Name != channelName {
		return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: args.T("api.command_join.missing.app_error")}
	}

	switch channel.Type {
	case model.ChannelTypeOpen:
		if !a.HasPermissionToChannel(args.UserID, channel.ID, model.PermissionJoinPublicChannels) {
			return &model.CommandResponse{Text: args.T("api.command_join.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}
	case model.ChannelTypePrivate:
		if !a.HasPermissionToChannel(args.UserID, channel.ID, model.PermissionReadChannel) {
			return &model.CommandResponse{Text: args.T("api.command_join.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}
	default:
		return &model.CommandResponse{Text: args.T("api.command_join.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	if appErr := a.JoinChannel(c, channel, args.UserID); appErr != nil {
		return &model.CommandResponse{Text: args.T("api.command_join.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	team, appErr := a.GetTeam(channel.TeamID)
	if appErr != nil {
		return &model.CommandResponse{Text: args.T("api.command_join.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	return &model.CommandResponse{GotoLocation: args.SiteURL + "/" + team.Name + "/channels/" + channel.Name}
}
