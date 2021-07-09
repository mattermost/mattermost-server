// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"context"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type InviteProvider struct {
}

const (
	CmdInvite = "invite"
)

func init() {
	app.RegisterCommandProvider(&InviteProvider{})
}

func (*InviteProvider) GetTrigger() string {
	return CmdInvite
}

func (*InviteProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdInvite,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_invite.desc"),
		AutoCompleteHint: T("api.command_invite.hint"),
		DisplayName:      T("api.command_invite.name"),
	}
}

func (*InviteProvider) DoCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	if message == "" {
		return &model.CommandResponse{
			Text:         args.T("api.command_invite.missing_message.app_error"),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	splitMessage := strings.SplitN(message, " ", 2)
	targetUsername := splitMessage[0]
	targetUsername = strings.TrimPrefix(targetUsername, "@")

	userProfile, nErr := a.Srv().Store.User().GetByUsername(targetUsername)
	if nErr != nil {
		mlog.Error(nErr.Error())
		return &model.CommandResponse{
			Text:         args.T("api.command_invite.missing_user.app_error"),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	if userProfile.DeleteAt != 0 {
		return &model.CommandResponse{
			Text:         args.T("api.command_invite.missing_user.app_error"),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	var channelToJoin *model.Channel
	var err *model.AppError
	// User set a channel to add the invited user
	if len(splitMessage) > 1 && splitMessage[1] != "" {
		targetChannelName := strings.TrimPrefix(strings.TrimSpace(splitMessage[1]), "~")

		if channelToJoin, err = a.GetChannelByName(targetChannelName, args.TeamID, false); err != nil {
			return &model.CommandResponse{
				Text: args.T("api.command_invite.channel.error", map[string]interface{}{
					"Channel": targetChannelName,
				}),
				ResponseType: model.CommandResponseTypeEphemeral,
			}
		}
	} else {
		channelToJoin, err = a.GetChannel(args.ChannelID)
		if err != nil {
			return &model.CommandResponse{
				Text:         args.T("api.command_invite.channel.app_error"),
				ResponseType: model.CommandResponseTypeEphemeral,
			}
		}
	}

	// Permissions Check
	switch channelToJoin.Type {
	case model.ChannelTypeOpen:
		if !a.HasPermissionToChannel(args.UserID, channelToJoin.ID, model.PermissionManagePublicChannelMembers) {
			return &model.CommandResponse{
				Text: args.T("api.command_invite.permission.app_error", map[string]interface{}{
					"User":    userProfile.Username,
					"Channel": channelToJoin.Name,
				}),
				ResponseType: model.CommandResponseTypeEphemeral,
			}
		}
	case model.ChannelTypePrivate:
		if !a.HasPermissionToChannel(args.UserID, channelToJoin.ID, model.PermissionManagePrivateChannelMembers) {
			if _, err = a.GetChannelMember(context.Background(), channelToJoin.ID, args.UserID); err == nil {
				// User doing the inviting is a member of the channel.
				return &model.CommandResponse{
					Text: args.T("api.command_invite.permission.app_error", map[string]interface{}{
						"User":    userProfile.Username,
						"Channel": channelToJoin.Name,
					}),
					ResponseType: model.CommandResponseTypeEphemeral,
				}
			}
			// User doing the inviting is *not* a member of the channel.
			return &model.CommandResponse{
				Text: args.T("api.command_invite.private_channel.app_error", map[string]interface{}{
					"Channel": channelToJoin.Name,
				}),
				ResponseType: model.CommandResponseTypeEphemeral,
			}
		}
	default:
		return &model.CommandResponse{
			Text:         args.T("api.command_invite.directchannel.app_error"),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	// Check if user is already in the channel
	_, err = a.GetChannelMember(context.Background(), channelToJoin.ID, userProfile.ID)
	if err == nil {
		return &model.CommandResponse{
			Text: args.T("api.command_invite.user_already_in_channel.app_error", map[string]interface{}{
				"User": userProfile.Username,
			}),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	if _, err := a.AddChannelMember(c, userProfile.ID, channelToJoin, app.ChannelMemberOpts{
		UserRequestorID: args.UserID,
	}); err != nil {
		var text string
		if err.ID == "api.channel.add_members.user_denied" {
			text = args.T("api.command_invite.group_constrained_user_denied")
		} else if err.ID == "app.team.get_member.missing.app_error" ||
			err.ID == "api.channel.add_user.to.channel.failed.deleted.app_error" {
			text = args.T("api.command_invite.user_not_in_team.app_error", map[string]interface{}{
				"Username": userProfile.Username,
			})
		} else {
			text = args.T("api.command_invite.fail.app_error")
		}
		return &model.CommandResponse{
			Text:         text,
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	if args.ChannelID != channelToJoin.ID {
		return &model.CommandResponse{
			Text: args.T("api.command_invite.success", map[string]interface{}{
				"User":    userProfile.Username,
				"Channel": channelToJoin.Name,
			}),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	return &model.CommandResponse{}
}
