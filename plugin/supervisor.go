// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	plugin "github.com/hashicorp/go-plugin"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type supervisor struct {
	lock        sync.RWMutex
	client      *plugin.Client
	hooks       Hooks
	implemented [TotalHooksID]bool
	pid         int
}

func newSupervisor(pluginInfo *model.BundleInfo, apiImpl API, driver Driver, parentLogger *mlog.Logger, metrics einterfaces.MetricsInterface) (retSupervisor *supervisor, retErr error) {
	sup := supervisor{}
	defer func() {
		if retErr != nil {
			sup.Shutdown()
		}
	}()

	wrappedLogger := pluginInfo.WrapLogger(parentLogger)

	hclogAdaptedLogger := &hclogAdapter{
		wrappedLogger: wrappedLogger.WithCallerSkip(1),
		extrasKey:     "wrapped_extras",
	}

	pluginMap := map[string]plugin.Plugin{
		"hooks": &hooksPlugin{
			log:        wrappedLogger,
			driverImpl: driver,
			apiImpl:    &apiTimerLayer{pluginInfo.Manifest.ID, apiImpl, metrics},
		},
	}

	executable := filepath.Clean(filepath.Join(
		".",
		pluginInfo.Manifest.GetExecutableForRuntime(runtime.GOOS, runtime.GOARCH),
	))
	if strings.HasPrefix(executable, "..") {
		return nil, fmt.Errorf("invalid backend executable")
	}
	executable = filepath.Join(pluginInfo.Path, executable)

	cmd := exec.Command(executable)

	sup.client = plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshake,
		Plugins:         pluginMap,
		Cmd:             cmd,
		SyncStdout:      wrappedLogger.With(mlog.String("source", "plugin_stdout")).StdLogWriter(),
		SyncStderr:      wrappedLogger.With(mlog.String("source", "plugin_stderr")).StdLogWriter(),
		Logger:          hclogAdaptedLogger,
		StartTimeout:    time.Second * 3,
	})

	rpcClient, err := sup.client.Client()
	if err != nil {
		return nil, err
	}

	sup.pid = cmd.Process.Pid

	raw, err := rpcClient.Dispense("hooks")
	if err != nil {
		return nil, err
	}

	sup.hooks = &hooksTimerLayer{pluginInfo.Manifest.ID, raw.(Hooks), metrics}

	impl, err := sup.hooks.Implemented()
	if err != nil {
		return nil, err
	}
	for _, hookName := range impl {
		if hookID, ok := hookNameToID[hookName]; ok {
			sup.implemented[hookID] = true
		}
	}

	return &sup, nil
}

func (sup *supervisor) Shutdown() {
	sup.lock.RLock()
	defer sup.lock.RUnlock()
	if sup.client != nil {
		sup.client.Kill()
	}
}

func (sup *supervisor) Hooks() Hooks {
	sup.lock.RLock()
	defer sup.lock.RUnlock()
	return sup.hooks
}

// PerformHealthCheck checks the plugin through an an RPC ping.
func (sup *supervisor) PerformHealthCheck() error {
	// No need for a lock here because Ping is read-locked.
	if pingErr := sup.Ping(); pingErr != nil {
		for pingFails := 1; pingFails < HealthCheckPingFailLimit; pingFails++ {
			pingErr = sup.Ping()
			if pingErr == nil {
				break
			}
		}
		if pingErr != nil {
			return fmt.Errorf("plugin RPC connection is not responding")
		}
	}

	return nil
}

// Ping checks that the RPC connection with the plugin is alive and healthy.
func (sup *supervisor) Ping() error {
	sup.lock.RLock()
	defer sup.lock.RUnlock()
	client, err := sup.client.Client()
	if err != nil {
		return err
	}

	return client.Ping()
}

func (sup *supervisor) Implements(hookID int) bool {
	sup.lock.RLock()
	defer sup.lock.RUnlock()
	return sup.implemented[hookID]
}
