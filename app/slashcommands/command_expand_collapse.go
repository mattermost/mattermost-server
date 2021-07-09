// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"strconv"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
)

type ExpandProvider struct {
}

type CollapseProvider struct {
}

const (
	CmdExpand   = "expand"
	CmdCollapse = "collapse"
)

func init() {
	app.RegisterCommandProvider(&ExpandProvider{})
	app.RegisterCommandProvider(&CollapseProvider{})
}

func (*ExpandProvider) GetTrigger() string {
	return CmdExpand
}

func (*CollapseProvider) GetTrigger() string {
	return CmdCollapse
}

func (*ExpandProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdExpand,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_expand.desc"),
		DisplayName:      T("api.command_expand.name"),
	}
}

func (*CollapseProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdCollapse,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_collapse.desc"),
		DisplayName:      T("api.command_collapse.name"),
	}
}

func (*ExpandProvider) DoCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	return setCollapsePreference(a, args, false)
}

func (*CollapseProvider) DoCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	return setCollapsePreference(a, args, true)
}

func setCollapsePreference(a *app.App, args *model.CommandArgs, isCollapse bool) *model.CommandResponse {
	pref := model.Preference{
		UserID:   args.UserID,
		Category: model.PreferenceCategoryDisplaySettings,
		Name:     model.PreferenceNameCollapseSetting,
		Value:    strconv.FormatBool(isCollapse),
	}

	if err := a.Srv().Store.Preference().Save(&model.Preferences{pref}); err != nil {
		return &model.CommandResponse{Text: args.T("api.command_expand_collapse.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	socketMessage := model.NewWebSocketEvent(model.WebsocketEventPreferenceChanged, "", "", args.UserID, nil)
	socketMessage.Add("preference", pref.ToJSON())
	a.Publish(socketMessage)

	var rmsg string

	if isCollapse {
		rmsg = args.T("api.command_collapse.success")
	} else {
		rmsg = args.T("api.command_expand.success")
	}
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: rmsg}
}
