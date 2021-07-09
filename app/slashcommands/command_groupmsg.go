// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type groupmsgProvider struct {
}

const (
	CmdGroupMsg = "groupmsg"
)

func init() {
	app.RegisterCommandProvider(&groupmsgProvider{})
}

func (*groupmsgProvider) GetTrigger() string {
	return CmdGroupMsg
}

func (*groupmsgProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdGroupMsg,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_groupmsg.desc"),
		AutoCompleteHint: T("api.command_groupmsg.hint"),
		DisplayName:      T("api.command_groupmsg.name"),
	}
}

func (*groupmsgProvider) DoCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	targetUsers := map[string]*model.User{}
	targetUsersSlice := []string{args.UserID}
	invalidUsernames := []string{}

	users, parsedMessage := groupMsgUsernames(message)

	for _, username := range users {
		username = strings.TrimSpace(username)
		username = strings.TrimPrefix(username, "@")
		targetUser, nErr := a.Srv().Store.User().GetByUsername(username)
		if nErr != nil {
			invalidUsernames = append(invalidUsernames, username)
			continue
		}

		canSee, err := a.UserCanSeeOtherUser(args.UserID, targetUser.ID)
		if err != nil {
			return &model.CommandResponse{Text: args.T("api.command_groupmsg.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}

		if !canSee {
			invalidUsernames = append(invalidUsernames, username)
			continue
		}

		_, exists := targetUsers[targetUser.ID]
		if !exists && targetUser.ID != args.UserID {
			targetUsers[targetUser.ID] = targetUser
			targetUsersSlice = append(targetUsersSlice, targetUser.ID)
		}
	}

	if len(invalidUsernames) > 0 {
		invalidUsersString := map[string]interface{}{
			"Users": "@" + strings.Join(invalidUsernames, ", @"),
		}
		return &model.CommandResponse{
			Text:         args.T("api.command_groupmsg.invalid_user.app_error", len(invalidUsernames), invalidUsersString),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	if len(targetUsersSlice) == 2 {
		return app.GetCommandProvider("msg").DoCommand(a, c, args, fmt.Sprintf("%s %s", targetUsers[targetUsersSlice[1]].Username, parsedMessage))
	}

	if len(targetUsersSlice) < model.ChannelGroupMinUsers {
		minUsers := map[string]interface{}{
			"MinUsers": model.ChannelGroupMinUsers - 1,
		}
		return &model.CommandResponse{
			Text:         args.T("api.command_groupmsg.min_users.app_error", minUsers),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	if len(targetUsersSlice) > model.ChannelGroupMaxUsers {
		maxUsers := map[string]interface{}{
			"MaxUsers": model.ChannelGroupMaxUsers - 1,
		}
		return &model.CommandResponse{
			Text:         args.T("api.command_groupmsg.max_users.app_error", maxUsers),
			ResponseType: model.CommandResponseTypeEphemeral,
		}
	}

	var groupChannel *model.Channel
	var channelErr *model.AppError

	if a.HasPermissionTo(args.UserID, model.PermissionCreateGroupChannel) {
		groupChannel, channelErr = a.CreateGroupChannel(targetUsersSlice, args.UserID)
		if channelErr != nil {
			mlog.Error(channelErr.Error())
			return &model.CommandResponse{Text: args.T("api.command_groupmsg.group_fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}
	} else {
		groupChannel, channelErr = a.GetGroupChannel(targetUsersSlice)
		if channelErr != nil {
			return &model.CommandResponse{Text: args.T("api.command_groupmsg.permission.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}
	}

	if parsedMessage != "" {
		post := &model.Post{}
		post.Message = parsedMessage
		post.ChannelID = groupChannel.ID
		post.UserID = args.UserID
		if _, err := a.CreatePostMissingChannel(c, post, true); err != nil {
			return &model.CommandResponse{Text: args.T("api.command_groupmsg.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}
	}

	team, err := a.GetTeam(args.TeamID)
	if err != nil {
		return &model.CommandResponse{Text: args.T("api.command_groupmsg.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	return &model.CommandResponse{GotoLocation: args.SiteURL + "/" + team.Name + "/channels/" + groupChannel.Name, Text: "", ResponseType: model.CommandResponseTypeEphemeral}
}

func groupMsgUsernames(message string) ([]string, string) {
	result := []string{}
	resultMessage := ""
	for idx, part := range strings.Split(message, ",") {
		clean := strings.TrimPrefix(strings.TrimSpace(part), "@")
		split := strings.Fields(clean)
		if len(split) > 0 {
			result = append(result, split[0])
		}
		if len(split) > 1 {
			splitted := strings.SplitN(message, ",", idx+1)
			resultMessage = strings.TrimPrefix(strings.TrimSpace(splitted[len(splitted)-1]), "@")
			resultMessage = strings.TrimSpace(strings.TrimPrefix(resultMessage, split[0]))
			break
		}
	}
	return result, resultMessage
}
