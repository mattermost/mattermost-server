// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"encoding/json"
	"reflect"
	"sync"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils/jsonutils"
)

var (
	// ErrReadOnlyStore is returned when an attempt to modify a read-only
	// configuration store is made.
	ErrReadOnlyStore = errors.New("configuration store is read-only")
)

// Store is the higher level object that handles storing and retrieval of config data.
// To do so it relies on a variety of backing stores (e.g. file, database, memory).
type Store struct {
	emitter
	backingStore BackingStore

	configLock           sync.RWMutex
	config               *model.Config
	configNoEnv          *model.Config
	configCustomDefaults *model.Config

	readOnly   bool
	readOnlyFF bool
}

// BackingStore defines the behaviour exposed by the underlying store
// implementation (e.g. file, database).
type BackingStore interface {
	// Set replaces the current configuration in its entirety and updates the backing store.
	Set(*model.Config) error

	// Load retrieves the configuration stored. If there is no configuration stored
	// the io.ReadCloser will be nil
	Load() ([]byte, error)

	// GetFile fetches the contents of a previously persisted configuration file.
	// If no such file exists, an empty byte array will be returned without error.
	GetFile(name string) ([]byte, error)

	// SetFile sets or replaces the contents of a configuration file.
	SetFile(name string, data []byte) error

	// HasFile returns true if the given file was previously persisted.
	HasFile(name string) (bool, error)

	// RemoveFile removes a previously persisted configuration file.
	RemoveFile(name string) error

	// String describes the backing store for the config.
	String() string

	// Close cleans up resources associated with the store.
	Close() error
}

// NewStoreFromBacking creates and returns a new config store given a backing store.
func NewStoreFromBacking(backingStore BackingStore, customDefaults *model.Config, readOnly bool) (*Store, error) {
	store := &Store{
		backingStore:         backingStore,
		configCustomDefaults: customDefaults,
		readOnly:             readOnly,
		readOnlyFF:           true,
	}

	if err := store.Load(); err != nil {
		return nil, errors.Wrap(err, "unable to load on store creation")
	}

	return store, nil
}

// NewStoreFromDSN creates and returns a new config store backed by either a database or file store
// depending on the value of the given data source name string.
func NewStoreFromDSN(dsn string, readOnly bool, customDefaults *model.Config) (*Store, error) {
	var err error
	var backingStore BackingStore
	if IsDatabaseDSN(dsn) {
		backingStore, err = NewDatabaseStore(dsn)
	} else {
		backingStore, err = NewFileStore(dsn)
	}
	if err != nil {
		return nil, err
	}

	store, err := NewStoreFromBacking(backingStore, customDefaults, readOnly)
	if err != nil {
		backingStore.Close()
		return nil, errors.Wrap(err, "failed to create store")
	}

	return store, nil
}

// NewTestMemoryStore returns a new config store backed by a memory store
// to be used for testing purposes.
func NewTestMemoryStore() *Store {
	memoryStore, err := NewMemoryStore()
	if err != nil {
		panic("failed to initialize memory store: " + err.Error())
	}

	configStore, err := NewStoreFromBacking(memoryStore, nil, false)
	if err != nil {
		panic("failed to initialize config store: " + err.Error())
	}

	return configStore
}

// Get fetches the current, cached configuration.
func (s *Store) Get() *model.Config {
	s.configLock.RLock()
	defer s.configLock.RUnlock()
	return s.config
}

// GetNoEnv fetches the current cached configuration without environment variable overrides.
func (s *Store) GetNoEnv() *model.Config {
	s.configLock.RLock()
	defer s.configLock.RUnlock()
	return s.configNoEnv
}

// GetEnvironmentOverrides fetches the configuration fields overridden by environment variables.
func (s *Store) GetEnvironmentOverrides() map[string]interface{} {
	return generateEnvironmentMap(GetEnvironment(), nil)
}

// GetEnvironmentOverridesWithFilter fetches the configuration fields overridden by environment variables.
// If filter is not nil and returns false for a struct field, that field will be omitted.
func (s *Store) GetEnvironmentOverridesWithFilter(filter func(reflect.StructField) bool) map[string]interface{} {
	return generateEnvironmentMap(GetEnvironment(), filter)
}

// RemoveEnvironmentOverrides returns a new config without the environment
// overrides.
func (s *Store) RemoveEnvironmentOverrides(cfg *model.Config) *model.Config {
	s.configLock.RLock()
	defer s.configLock.RUnlock()
	return removeEnvOverrides(cfg, s.configNoEnv, s.GetEnvironmentOverrides())
}

// SetReadOnlyFF sets whether feature flags should be written out to
// config or treated as read-only.
func (s *Store) SetReadOnlyFF(readOnly bool) {
	s.configLock.Lock()
	defer s.configLock.Unlock()
	s.readOnlyFF = readOnly
}

// Set replaces the current configuration in its entirety and updates the backing store.
// It returns both old and new versions of the config.
func (s *Store) Set(newCfg *model.Config) (*model.Config, *model.Config, error) {
	s.configLock.Lock()
	defer s.configLock.Unlock()

	if s.readOnly {
		return nil, nil, ErrReadOnlyStore
	}

	newCfg = newCfg.Clone()
	oldCfg := s.config.Clone()
	oldCfgNoEnv := s.configNoEnv

	// Setting defaults allows us to accept partial config objects.
	newCfg.SetDefaults()

	// Sometimes the config is received with "fake" data in sensitive fields. Apply the real
	// data from the existing config as necessary.
	desanitize(oldCfg, newCfg)

	if err := newCfg.IsValid(); err != nil {
		return nil, nil, errors.Wrap(err, "new configuration is invalid")
	}

	// We attempt to remove any environment override that may be present in the input config.
	newCfgNoEnv := removeEnvOverrides(newCfg, oldCfgNoEnv, s.GetEnvironmentOverrides())

	// Don't store feature flags unless we are on MM cloud
	// MM cloud uses config in the DB as a cache of the feature flag
	// settings in case the management system is down when a pod starts.

	// Backing up feature flags section in case we need to restore them later on.
	oldCfgFF := oldCfg.FeatureFlags
	oldCfgNoEnvFF := oldCfgNoEnv.FeatureFlags
	// Clearing FF sections to avoid both comparing and persisting them.
	if s.readOnlyFF {
		oldCfg.FeatureFlags = nil
		newCfg.FeatureFlags = nil
		newCfgNoEnv.FeatureFlags = nil
	}

	if err := s.backingStore.Set(newCfgNoEnv); err != nil {
		return nil, nil, errors.Wrap(err, "failed to persist")
	}

	// We apply back environment overrides since the input config may or
	// may not have them applied.
	newCfg = applyEnvironmentMap(newCfgNoEnv, GetEnvironment())
	fixConfig(newCfg)
	if err := newCfg.IsValid(); err != nil {
		return nil, nil, errors.Wrap(err, "new configuration is invalid")
	}

	hasChanged, err := equal(oldCfg, newCfg)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to compare configs")
	}

	// We restore the previously cleared feature flags sections back.
	if s.readOnlyFF {
		oldCfg.FeatureFlags = oldCfgFF
		newCfg.FeatureFlags = oldCfgFF
		newCfgNoEnv.FeatureFlags = oldCfgNoEnvFF
	}

	s.configNoEnv = newCfgNoEnv
	s.config = newCfg

	newCfgCopy := newCfg.Clone()

	if hasChanged {
		s.configLock.Unlock()
		s.invokeConfigListeners(oldCfg, newCfgCopy.Clone())
		s.configLock.Lock()
	}

	return oldCfg, newCfgCopy, nil
}

// Load updates the current configuration from the backing store, possibly initializing.
func (s *Store) Load() error {
	s.configLock.Lock()
	defer s.configLock.Unlock()

	oldCfg := &model.Config{}
	if s.config != nil {
		oldCfg = s.config.Clone()
	}

	configBytes, err := s.backingStore.Load()
	if err != nil {
		return err
	}

	loadedCfg := &model.Config{}
	if len(configBytes) != 0 {
		if err = json.Unmarshal(configBytes, &loadedCfg); err != nil {
			return jsonutils.HumanizeJSONError(err, configBytes)
		}
	}

	// If we have custom defaults set, the initial config is merged on
	// top of them and we delete them not to be used again in the
	// configuration reloads
	if s.configCustomDefaults != nil {
		var mErr error
		loadedCfg, mErr = Merge(s.configCustomDefaults, loadedCfg, nil)
		if mErr != nil {
			return errors.Wrap(mErr, "failed to merge custom config defaults")
		}
		s.configCustomDefaults = nil
	}

	// We set the SiteURL to empty (if nil) so that the following call to
	// SetDefaults() will generate missing data. This avoids an additional write
	// to the backing store.
	if loadedCfg.ServiceSettings.SiteURL == nil {
		loadedCfg.ServiceSettings.SiteURL = model.NewString("")
	}

	// Setting defaults allows us to accept partial config objects.
	loadedCfg.SetDefaults()

	// No need to clone here since the below call to applyEnvironmentMap
	// already does that internally.
	loadedCfgNoEnv := loadedCfg
	fixConfig(loadedCfgNoEnv)

	loadedCfg = applyEnvironmentMap(loadedCfg, GetEnvironment())
	fixConfig(loadedCfg)
	if err := loadedCfg.IsValid(); err != nil {
		return errors.Wrap(err, "invalid config")
	}

	// Backing up feature flags section in case we need to restore them later on.
	oldCfgFF := oldCfg.FeatureFlags
	loadedCfgFF := loadedCfg.FeatureFlags
	loadedCfgNoEnvFF := loadedCfgNoEnv.FeatureFlags
	// Clearing FF sections to avoid both comparing and persisting them.
	if s.readOnlyFF {
		oldCfg.FeatureFlags = nil
		loadedCfg.FeatureFlags = nil
		loadedCfgNoEnv.FeatureFlags = nil
	}

	// Check for changes that may have happened on load to the backing store.
	hasChanged, err := equal(oldCfg, loadedCfg)
	if err != nil {
		return errors.Wrap(err, "failed to compare configs")
	}

	// We write back to the backing store only if the store is not read-only
	// and the config has either changed or is missing.
	if !s.readOnly && (hasChanged || len(configBytes) == 0) {
		err := s.backingStore.Set(loadedCfgNoEnv)
		if err != nil && !errors.Is(err, ErrReadOnlyConfiguration) {
			return errors.Wrap(err, "failed to persist")
		}
	}

	// We restore the previously cleared feature flags sections back.
	if s.readOnlyFF {
		oldCfg.FeatureFlags = oldCfgFF
		loadedCfg.FeatureFlags = loadedCfgFF
		loadedCfgNoEnv.FeatureFlags = loadedCfgNoEnvFF
	}

	s.config = loadedCfg
	s.configNoEnv = loadedCfgNoEnv

	loadedCfgCopy := loadedCfg.Clone()

	if hasChanged {
		s.configLock.Unlock()
		s.invokeConfigListeners(oldCfg, loadedCfgCopy)
		s.configLock.Lock()
	}

	return nil
}

// GetFile fetches the contents of a previously persisted configuration file.
// If no such file exists, an empty byte array will be returned without error.
func (s *Store) GetFile(name string) ([]byte, error) {
	s.configLock.RLock()
	defer s.configLock.RUnlock()
	return s.backingStore.GetFile(name)
}

// SetFile sets or replaces the contents of a configuration file.
func (s *Store) SetFile(name string, data []byte) error {
	s.configLock.Lock()
	defer s.configLock.Unlock()
	if s.readOnly {
		return ErrReadOnlyStore
	}
	return s.backingStore.SetFile(name, data)
}

// HasFile returns true if the given file was previously persisted.
func (s *Store) HasFile(name string) (bool, error) {
	s.configLock.RLock()
	defer s.configLock.RUnlock()
	return s.backingStore.HasFile(name)
}

// RemoveFile removes a previously persisted configuration file.
func (s *Store) RemoveFile(name string) error {
	s.configLock.Lock()
	defer s.configLock.Unlock()
	if s.readOnly {
		return ErrReadOnlyStore
	}
	return s.backingStore.RemoveFile(name)
}

// String describes the backing store for the config.
func (s *Store) String() string {
	return s.backingStore.String()
}

// Close cleans up resources associated with the store.
func (s *Store) Close() error {
	s.configLock.Lock()
	defer s.configLock.Unlock()
	return s.backingStore.Close()
}

// IsReadOnly returns whether or not the store is read-only.
func (s *Store) IsReadOnly() bool {
	s.configLock.RLock()
	defer s.configLock.RUnlock()
	return s.readOnly
}
