// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main_test

import (
	"database/sql"
	"testing"

	"github.com/mattermost/mattermost-server/v5/app/plugin_api_tests"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/shared/driver"
	"github.com/mattermost/mattermost-server/v5/store/sqlstore"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
)

type MyPlugin struct {
	plugin.MattermostPlugin
	config plugin_api_tests.BasicConfig
	t      *testing.T
}

func (p *MyPlugin) OnConfigurationChange() error {
	if err := p.API.LoadPluginConfiguration(&p.config); err != nil {
		return err
	}
	return nil
}

func (p *MyPlugin) MessageWillBePosted(_ *plugin.Context, _ *model.Post) (*model.Post, string) {
	store := sqlstore.New(p.API.GetUnsanitizedConfig().SQLSettings, nil)
	store.GetMaster().Db.Close()

	for _, isMaster := range []bool{true, false} {
		// We replace the master DB with master and replica both just to make
		// gorp APIs work.
		store.GetMaster().Db = sql.OpenDB(driver.NewConnector(p.Driver, isMaster))

		// Testing with a handful of stores
		storetest.TestPostStore(p.t, store, store)
		storetest.TestUserStore(p.t, store, store)
		storetest.TestTeamStore(p.t, store)
		storetest.TestChannelStore(p.t, store, store)
		storetest.TestBotStore(p.t, store, store)

		store.GetMaster().Db.Close()
	}

	// Use the API to instantiate the driver
	// And then run the full suite of tests.
	return nil, "OK"
}

// TestDBAPI is a test function which actually runs a plugin. The objective
// is to run the storetest suite from inside a plugin.
//
// The test runner compiles the test code to a binary, and runs it as a normal
// binary. But under the hood, a test runs.
func TestDBAPI(t *testing.T) {
	plugin.ClientMain(&MyPlugin{t: t})
}
