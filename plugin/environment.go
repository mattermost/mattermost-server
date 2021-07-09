// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin

import (
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

var ErrNotFound = errors.New("Item not found")

type apiImplCreatorFunc func(*model.Manifest) API

// registeredPlugin stores the state for a given plugin that has been activated
// or attempted to be activated this server run.
//
// If an installed plugin is missing from the env.registeredPlugins map, then the
// plugin is configured as disabled and has not been activated during this server run.
type registeredPlugin struct {
	BundleInfo *model.BundleInfo
	State      int

	supervisor *supervisor
}

// PrepackagedPlugin is a plugin prepackaged with the server and found on startup.
type PrepackagedPlugin struct {
	Path      string
	IconData  string
	Manifest  *model.Manifest
	Signature []byte
}

// Environment represents the execution environment of active plugins.
//
// It is meant for use by the Mattermost server to manipulate, interact with and report on the set
// of active plugins.
type Environment struct {
	registeredPlugins      sync.Map
	pluginHealthCheckJob   *PluginHealthCheckJob
	logger                 *mlog.Logger
	metrics                einterfaces.MetricsInterface
	newAPIImpl             apiImplCreatorFunc
	dbDriver               Driver
	pluginDir              string
	webappPluginDir        string
	prepackagedPlugins     []*PrepackagedPlugin
	prepackagedPluginsLock sync.RWMutex
}

func NewEnvironment(newAPIImpl apiImplCreatorFunc,
	dbDriver Driver,
	pluginDir string, webappPluginDir string,
	logger *mlog.Logger,
	metrics einterfaces.MetricsInterface) (*Environment, error) {
	return &Environment{
		logger:          logger,
		metrics:         metrics,
		newAPIImpl:      newAPIImpl,
		dbDriver:        dbDriver,
		pluginDir:       pluginDir,
		webappPluginDir: webappPluginDir,
	}, nil
}

// Performs a full scan of the given path.
//
// This function will return info for all subdirectories that appear to be plugins (i.e. all
// subdirectories containing plugin manifest files, regardless of whether they could actually be
// parsed).
//
// Plugins are found non-recursively and paths beginning with a dot are always ignored.
func scanSearchPath(path string) ([]*model.BundleInfo, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var ret []*model.BundleInfo
	for _, file := range files {
		if !file.IsDir() || file.Name()[0] == '.' {
			continue
		}
		info := model.BundleInfoForPath(filepath.Join(path, file.Name()))
		if info.Manifest != nil {
			ret = append(ret, info)
		}
	}
	return ret, nil
}

// Returns a list of all plugins within the environment.
func (env *Environment) Available() ([]*model.BundleInfo, error) {
	return scanSearchPath(env.pluginDir)
}

// Returns a list of prepackaged plugins available in the local prepackaged_plugins folder.
// The list content is immutable and should not be modified.
func (env *Environment) PrepackagedPlugins() []*PrepackagedPlugin {
	env.prepackagedPluginsLock.RLock()
	defer env.prepackagedPluginsLock.RUnlock()

	return env.prepackagedPlugins
}

// Returns a list of all currently active plugins within the environment.
// The returned list should not be modified.
func (env *Environment) Active() []*model.BundleInfo {
	activePlugins := []*model.BundleInfo{}
	env.registeredPlugins.Range(func(key, value interface{}) bool {
		plugin := value.(registeredPlugin)
		if env.IsActive(plugin.BundleInfo.Manifest.ID) {
			activePlugins = append(activePlugins, plugin.BundleInfo)
		}

		return true
	})

	return activePlugins
}

// IsActive returns true if the plugin with the given id is active.
func (env *Environment) IsActive(id string) bool {
	return env.GetPluginState(id) == model.PluginStateRunning
}

// GetPluginState returns the current state of a plugin (disabled, running, or error)
func (env *Environment) GetPluginState(id string) int {
	rp, ok := env.registeredPlugins.Load(id)
	if !ok {
		return model.PluginStateNotRunning
	}

	return rp.(registeredPlugin).State
}

// setPluginState sets the current state of a plugin (disabled, running, or error)
func (env *Environment) setPluginState(id string, state int) {
	if rp, ok := env.registeredPlugins.Load(id); ok {
		p := rp.(registeredPlugin)
		p.State = state
		env.registeredPlugins.Store(id, p)
	}
}

// PublicFilesPath returns a path and true if the plugin with the given id is active.
// It returns an empty string and false if the path is not set or invalid
func (env *Environment) PublicFilesPath(id string) (string, error) {
	if !env.IsActive(id) {
		return "", fmt.Errorf("plugin not found: %v", id)
	}
	return filepath.Join(env.pluginDir, id, "public"), nil
}

// Statuses returns a list of plugin statuses representing the state of every plugin
func (env *Environment) Statuses() (model.PluginStatuses, error) {
	plugins, err := env.Available()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get plugin statuses")
	}

	pluginStatuses := make(model.PluginStatuses, 0, len(plugins))
	for _, plugin := range plugins {
		// For now we don't handle bad manifests, we should
		if plugin.Manifest == nil {
			continue
		}

		pluginState := env.GetPluginState(plugin.Manifest.ID)

		status := &model.PluginStatus{
			PluginID:    plugin.Manifest.ID,
			PluginPath:  filepath.Dir(plugin.ManifestPath),
			State:       pluginState,
			Name:        plugin.Manifest.Name,
			Description: plugin.Manifest.Description,
			Version:     plugin.Manifest.Version,
		}

		pluginStatuses = append(pluginStatuses, status)
	}

	return pluginStatuses, nil
}

// GetManifest returns a manifest for a given pluginId.
// Returns ErrNotFound if plugin is not found.
func (env *Environment) GetManifest(pluginID string) (*model.Manifest, error) {
	plugins, err := env.Available()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get plugin statuses")
	}

	for _, plugin := range plugins {
		if plugin.Manifest != nil && plugin.Manifest.ID == pluginID {
			return plugin.Manifest, nil
		}
	}

	return nil, ErrNotFound
}

func (env *Environment) Activate(id string) (manifest *model.Manifest, activated bool, reterr error) {
	// Check if we are already active
	if env.IsActive(id) {
		return nil, false, nil
	}

	plugins, err := env.Available()
	if err != nil {
		return nil, false, err
	}
	var pluginInfo *model.BundleInfo
	for _, p := range plugins {
		if p.Manifest != nil && p.Manifest.ID == id {
			if pluginInfo != nil {
				return nil, false, fmt.Errorf("multiple plugins found: %v", id)
			}
			pluginInfo = p
		}
	}
	if pluginInfo == nil {
		return nil, false, fmt.Errorf("plugin not found: %v", id)
	}

	rp := newRegisteredPlugin(pluginInfo)
	env.registeredPlugins.Store(id, rp)

	defer func() {
		if reterr == nil {
			env.setPluginState(id, model.PluginStateRunning)
		} else {
			env.setPluginState(id, model.PluginStateFailedToStart)
		}
	}()

	if pluginInfo.Manifest.MinServerVersion != "" {
		fulfilled, err := pluginInfo.Manifest.MeetMinServerVersion(model.CurrentVersion)
		if err != nil {
			return nil, false, fmt.Errorf("%v: %v", err.Error(), id)
		}
		if !fulfilled {
			return nil, false, fmt.Errorf("plugin requires Mattermost %v: %v", pluginInfo.Manifest.MinServerVersion, id)
		}
	}

	componentActivated := false

	if pluginInfo.Manifest.HasWebapp() {
		updatedManifest, err := env.UnpackWebappBundle(id)
		if err != nil {
			return nil, false, errors.Wrapf(err, "unable to generate webapp bundle: %v", id)
		}
		pluginInfo.Manifest.Webapp.BundleHash = updatedManifest.Webapp.BundleHash

		componentActivated = true
	}

	if pluginInfo.Manifest.HasServer() {
		sup, err := newSupervisor(pluginInfo, env.newAPIImpl(pluginInfo.Manifest), env.dbDriver, env.logger, env.metrics)
		if err != nil {
			return nil, false, errors.Wrapf(err, "unable to start plugin: %v", id)
		}

		if err := sup.Hooks().OnActivate(); err != nil {
			sup.Shutdown()
			return nil, false, err
		}
		rp.supervisor = sup
		env.registeredPlugins.Store(id, rp)

		componentActivated = true
	}

	if !componentActivated {
		return nil, false, fmt.Errorf("unable to start plugin: must at least have a web app or server component")
	}

	return pluginInfo.Manifest, true, nil
}

func (env *Environment) RemovePlugin(id string) {
	if _, ok := env.registeredPlugins.Load(id); ok {
		env.registeredPlugins.Delete(id)
	}
}

// Deactivates the plugin with the given id.
func (env *Environment) Deactivate(id string) bool {
	p, ok := env.registeredPlugins.Load(id)
	if !ok {
		return false
	}

	isActive := env.IsActive(id)

	env.setPluginState(id, model.PluginStateNotRunning)

	if !isActive {
		return false
	}

	rp := p.(registeredPlugin)
	if rp.supervisor != nil {
		if err := rp.supervisor.Hooks().OnDeactivate(); err != nil {
			env.logger.Error("Plugin OnDeactivate() error", mlog.String("plugin_id", rp.BundleInfo.Manifest.ID), mlog.Err(err))
		}
		rp.supervisor.Shutdown()
	}

	return true
}

// RestartPlugin deactivates, then activates the plugin with the given id.
func (env *Environment) RestartPlugin(id string) error {
	env.Deactivate(id)
	_, _, err := env.Activate(id)
	return err
}

// Shutdown deactivates all plugins and gracefully shuts down the environment.
func (env *Environment) Shutdown() {
	if env.pluginHealthCheckJob != nil {
		env.pluginHealthCheckJob.Cancel()
	}

	var wg sync.WaitGroup
	env.registeredPlugins.Range(func(key, value interface{}) bool {
		rp := value.(registeredPlugin)

		if rp.supervisor == nil || !env.IsActive(rp.BundleInfo.Manifest.ID) {
			return true
		}

		wg.Add(1)

		done := make(chan bool)
		go func() {
			defer close(done)
			if err := rp.supervisor.Hooks().OnDeactivate(); err != nil {
				env.logger.Error("Plugin OnDeactivate() error", mlog.String("plugin_id", rp.BundleInfo.Manifest.ID), mlog.Err(err))
			}
		}()

		go func() {
			defer wg.Done()

			select {
			case <-time.After(10 * time.Second):
				env.logger.Warn("Plugin OnDeactivate() failed to complete in 10 seconds", mlog.String("plugin_id", rp.BundleInfo.Manifest.ID))
			case <-done:
			}

			rp.supervisor.Shutdown()
		}()

		return true
	})

	wg.Wait()

	env.registeredPlugins.Range(func(key, value interface{}) bool {
		env.registeredPlugins.Delete(key)

		return true
	})
}

// UnpackWebappBundle unpacks webapp bundle for a given plugin id on disk.
func (env *Environment) UnpackWebappBundle(id string) (*model.Manifest, error) {
	plugins, err := env.Available()
	if err != nil {
		return nil, errors.New("Unable to get available plugins")
	}
	var manifest *model.Manifest
	for _, p := range plugins {
		if p.Manifest != nil && p.Manifest.ID == id {
			if manifest != nil {
				return nil, fmt.Errorf("multiple plugins found: %v", id)
			}
			manifest = p.Manifest
		}
	}
	if manifest == nil {
		return nil, fmt.Errorf("plugin not found: %v", id)
	}

	bundlePath := filepath.Clean(manifest.Webapp.BundlePath)
	if bundlePath == "" || bundlePath[0] == '.' {
		return nil, fmt.Errorf("invalid webapp bundle path")
	}
	bundlePath = filepath.Join(env.pluginDir, id, bundlePath)
	destinationPath := filepath.Join(env.webappPluginDir, id)

	if err = os.RemoveAll(destinationPath); err != nil {
		return nil, errors.Wrapf(err, "unable to remove old webapp bundle directory: %v", destinationPath)
	}

	if err = utils.CopyDir(filepath.Dir(bundlePath), destinationPath); err != nil {
		return nil, errors.Wrapf(err, "unable to copy webapp bundle directory: %v", id)
	}

	sourceBundleFilepath := filepath.Join(destinationPath, filepath.Base(bundlePath))

	sourceBundleFileContents, err := ioutil.ReadFile(sourceBundleFilepath)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read webapp bundle: %v", id)
	}

	hash := fnv.New64a()
	if _, err = hash.Write(sourceBundleFileContents); err != nil {
		return nil, errors.Wrapf(err, "unable to generate hash for webapp bundle: %v", id)
	}
	manifest.Webapp.BundleHash = hash.Sum([]byte{})

	if err = os.Rename(
		sourceBundleFilepath,
		filepath.Join(destinationPath, fmt.Sprintf("%s_%x_bundle.js", id, manifest.Webapp.BundleHash)),
	); err != nil {
		return nil, errors.Wrapf(err, "unable to rename webapp bundle: %v", id)
	}

	return manifest, nil
}

// HooksForPlugin returns the hooks API for the plugin with the given id.
//
// Consider using RunMultiPluginHook instead.
func (env *Environment) HooksForPlugin(id string) (Hooks, error) {
	if p, ok := env.registeredPlugins.Load(id); ok {
		rp := p.(registeredPlugin)
		if rp.supervisor != nil && env.IsActive(id) {
			return rp.supervisor.Hooks(), nil
		}
	}

	return nil, fmt.Errorf("plugin not found: %v", id)
}

// RunMultiPluginHook invokes hookRunnerFunc for each active plugin that implements the given hookId.
//
// If hookRunnerFunc returns false, iteration will not continue. The iteration order among active
// plugins is not specified.
func (env *Environment) RunMultiPluginHook(hookRunnerFunc func(hooks Hooks) bool, hookID int) {
	startTime := time.Now()

	env.registeredPlugins.Range(func(key, value interface{}) bool {
		rp := value.(registeredPlugin)

		if rp.supervisor == nil || !rp.supervisor.Implements(hookID) || !env.IsActive(rp.BundleInfo.Manifest.ID) {
			return true
		}

		hookStartTime := time.Now()
		result := hookRunnerFunc(rp.supervisor.Hooks())

		if env.metrics != nil {
			elapsedTime := float64(time.Since(hookStartTime)) / float64(time.Second)
			env.metrics.ObservePluginMultiHookIterationDuration(rp.BundleInfo.Manifest.ID, elapsedTime)
		}

		return result
	})

	if env.metrics != nil {
		elapsedTime := float64(time.Since(startTime)) / float64(time.Second)
		env.metrics.ObservePluginMultiHookDuration(elapsedTime)
	}
}

// PerformHealthCheck uses the active plugin's supervisor to verify if the plugin has crashed.
func (env *Environment) PerformHealthCheck(id string) error {
	p, ok := env.registeredPlugins.Load(id)
	if !ok {
		return nil
	}
	rp := p.(registeredPlugin)

	sup := rp.supervisor
	if sup == nil {
		return nil
	}
	return sup.PerformHealthCheck()
}

// SetPrepackagedPlugins saves prepackaged plugins in the environment.
func (env *Environment) SetPrepackagedPlugins(plugins []*PrepackagedPlugin) {
	env.prepackagedPluginsLock.Lock()
	env.prepackagedPlugins = plugins
	env.prepackagedPluginsLock.Unlock()
}

func newRegisteredPlugin(bundle *model.BundleInfo) registeredPlugin {
	state := model.PluginStateNotRunning
	return registeredPlugin{State: state, BundleInfo: bundle}
}

// InitPluginHealthCheckJob starts a new job if one is not running and is set to enabled, or kills an existing one if set to disabled.
func (env *Environment) InitPluginHealthCheckJob(enable bool) {
	// Config is set to enable. No job exists, start a new job.
	if enable && env.pluginHealthCheckJob == nil {
		mlog.Debug("Enabling plugin health check job", mlog.Duration("interval_s", HealthCheckInterval))

		job := newPluginHealthCheckJob(env)
		env.pluginHealthCheckJob = job
		go job.run()
	}

	// Config is set to disable. Job exists, kill existing job.
	if !enable && env.pluginHealthCheckJob != nil {
		mlog.Debug("Disabling plugin health check job")

		env.pluginHealthCheckJob.Cancel()
		env.pluginHealthCheckJob = nil
	}
}

// GetPluginHealthCheckJob returns the configured PluginHealthCheckJob, if any.
func (env *Environment) GetPluginHealthCheckJob() *PluginHealthCheckJob {
	return env.pluginHealthCheckJob
}
