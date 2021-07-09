// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

func TestHealthCheckJob(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	tearDown, _, _ := SetAppEnvironmentWithPlugins(t, []string{
		`
			package main

			import (
				"github.com/mattermost/mattermost-server/v5/model"
				"github.com/mattermost/mattermost-server/v5/plugin"
			)

			type MyPlugin struct {
				plugin.MattermostPlugin
			}

			func (p *MyPlugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
				panic("Uncaught error")
			}

			func main() {
				plugin.ClientMain(&MyPlugin{})
			}
			`,
	}, th.App, th.NewPluginAPI)
	defer tearDown()

	env := th.App.GetPluginsEnvironment()
	job := env.GetPluginHealthCheckJob()
	require.NotNil(t, job)
	bundles := env.Active()
	require.Equal(t, 1, len(bundles))

	id := bundles[0].Manifest.ID

	// First health check
	hooks, err := env.HooksForPlugin(id)
	require.NoError(t, err)
	hooks.MessageWillBePosted(&plugin.Context{}, &model.Post{})
	job.CheckPlugin(id)
	bundles = env.Active()
	require.Equal(t, 1, len(bundles))
	require.Equal(t, id, bundles[0].Manifest.ID)
	require.Equal(t, model.PluginStateRunning, env.GetPluginState(id))

	// Second health check
	hooks, err = env.HooksForPlugin(id)
	require.NoError(t, err)
	hooks.MessageWillBePosted(&plugin.Context{}, &model.Post{})
	job.CheckPlugin(id)
	bundles = env.Active()
	require.Equal(t, 1, len(bundles))
	require.Equal(t, id, bundles[0].Manifest.ID)
	require.Equal(t, model.PluginStateRunning, env.GetPluginState(id))

	// Third health check, plugin should be deactivated by the job
	hooks, err = env.HooksForPlugin(id)
	require.NoError(t, err)
	hooks.MessageWillBePosted(&plugin.Context{}, &model.Post{})
	job.CheckPlugin(id)
	bundles = env.Active()
	require.Equal(t, 0, len(bundles))
	require.Equal(t, model.PluginStateFailedToStayRunning, env.GetPluginState(id))

	// Activated manually, plugin should stay active
	env.Activate(id)
	job.CheckPlugin(id)
	bundles = env.Active()
	require.Equal(t, 1, len(bundles))
	require.Equal(t, model.PluginStateRunning, env.GetPluginState(id))
}
