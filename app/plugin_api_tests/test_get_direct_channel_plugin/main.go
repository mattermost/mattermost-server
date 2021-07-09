// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"github.com/mattermost/mattermost-server/v5/app/plugin_api_tests"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

type MyPlugin struct {
	plugin.MattermostPlugin
	configuration plugin_api_tests.BasicConfig
}

func (p *MyPlugin) OnConfigurationChange() error {
	if err := p.API.LoadPluginConfiguration(&p.configuration); err != nil {
		return err
	}
	return nil
}

func (p *MyPlugin) MessageWillBePosted(_ *plugin.Context, _ *model.Post) (*model.Post, string) {
	dm1, err := p.API.GetDirectChannel(p.configuration.BasicUserID, p.configuration.BasicUser2ID)
	if err != nil {
		return nil, err.Error()
	}
	if plugin_api_tests.IsEmpty(dm1) {
		return nil, "dm1 is empty"
	}

	dm2, err := p.API.GetDirectChannel(p.configuration.BasicUserID, p.configuration.BasicUserID)
	if err != nil {
		return nil, err.Error()
	}
	if plugin_api_tests.IsEmpty(dm2) {
		return nil, "dm2 is empty"
	}

	dm3, err := p.API.GetDirectChannel(p.configuration.BasicUserID, model.NewID())
	if err == nil {
		return nil, "Expected to get error while fetching incorrect channel"
	}
	if !plugin_api_tests.IsEmpty(dm3) {
		return nil, "dm3 is NOT empty"
	}
	return nil, "OK"
}

func main() {
	plugin.ClientMain(&MyPlugin{})
}
