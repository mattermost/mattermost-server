// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
)

type RenameProvider struct {
}

const (
	CmdRename = "rename"
)

func init() {
	app.RegisterCommandProvider(&RenameProvider{})
}

func (*RenameProvider) GetTrigger() string {
	return CmdRename
}

func (*RenameProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	renameAutocompleteData := model.NewAutocompleteData(CmdRename, T("api.command_channel_rename.hint"), T("api.command_channel_rename.desc"))
	renameAutocompleteData.AddTextArgument(T("api.command_channel_rename.hint"), "[text]", "")
	return &model.Command{
		Trigger:          CmdRename,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_channel_rename.desc"),
		AutoCompleteHint: T("api.command_channel_rename.hint"),
		DisplayName:      T("api.command_channel_rename.name"),
		AutocompleteData: renameAutocompleteData,
	}
}

func (*RenameProvider) DoCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	channel, err := a.GetChannel(args.ChannelID)
	if err != nil {
		return &model.CommandResponse{
			Text:         args.T("api.command_channel_rename.channel.app_error"),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	switch channel.Type {
	case model.ChannelTypeOpen:
		if !a.HasPermissionToChannel(args.UserID, args.ChannelID, model.PermissionManagePublicChannelProperties) {
			return &model.CommandResponse{
				Text:         args.T("api.command_channel_rename.permission.app_error"),
				ResponseType: model.CommandResponseTypeEphemeral,
			}
		}
	case model.ChannelTypePrivate:
		if !a.HasPermissionToChannel(args.UserID, args.ChannelID, model.PermissionManagePrivateChannelProperties) {
			return &model.CommandResponse{
				Text:         args.T("api.command_channel_rename.permission.app_error"),
				ResponseType: model.CommandResponseTypeEphemeral,
			}
		}
	default:
		return &model.CommandResponse{Text: args.T("api.command_channel_rename.direct_group.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	if message == "" {
		return &model.CommandResponse{
			Text:         args.T("api.command_channel_rename.message.app_error"),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	} else if len(message) > model.ChannelNameMaxLength {
		return &model.CommandResponse{
			Text: args.T("api.command_channel_rename.too_long.app_error", map[string]interface{}{
				"Length": model.ChannelNameMaxLength,
			}),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	} else if len(message) < model.ChannelNameMinLength {
		return &model.CommandResponse{
			Text: args.T("api.command_channel_rename.too_short.app_error", map[string]interface{}{
				"Length": model.ChannelNameMinLength,
			}),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	patch := &model.ChannelPatch{
		DisplayName: new(string),
	}
	*patch.DisplayName = message

	_, err = a.PatchChannel(c, channel, patch, args.UserID)
	if err != nil {
		return &model.CommandResponse{
			Text:         args.T("api.command_channel_rename.update_channel.app_error"),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	return &model.CommandResponse{}
}
