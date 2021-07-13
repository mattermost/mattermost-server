// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
)

// MemoryStore implements the Store interface. It is meant primarily for testing.
// Not to be used directly. Only to be used as a backing store for config.Store
type MemoryStore struct {
	allowEnvironmentOverrides bool
	validate                  bool
	files                     map[string][]byte
	savedConfig               *model.Config
}

// MemoryStoreOptions makes configuration of the memory store explicit.
type MemoryStoreOptions struct {
	IgnoreEnvironmentOverrides bool
	SkipValidation             bool
	InitialConfig              *model.Config
	InitialFiles               map[string][]byte
}

// NewMemoryStore creates a new MemoryStore instance with default options.
func NewMemoryStore() (*MemoryStore, error) {
	return NewMemoryStoreWithOptions(&MemoryStoreOptions{})
}

// NewMemoryStoreWithOptions creates a new MemoryStore instance.
func NewMemoryStoreWithOptions(options *MemoryStoreOptions) (*MemoryStore, error) {
	savedConfig := options.InitialConfig
	if savedConfig == nil {
		savedConfig = &model.Config{}
		savedConfig.SetDefaults()
	}

	initialFiles := options.InitialFiles
	if initialFiles == nil {
		initialFiles = make(map[string][]byte)
	}

	ms := &MemoryStore{
		allowEnvironmentOverrides: !options.IgnoreEnvironmentOverrides,
		validate:                  !options.SkipValidation,
		files:                     initialFiles,
		savedConfig:               savedConfig,
	}

	return ms, nil
}

// Set replaces the current configuration in its entirety.
func (ms *MemoryStore) Set(newCfg *model.Config) error {
	return ms.persist(newCfg)
}

// persist copies the active config to the saved config.
func (ms *MemoryStore) persist(cfg *model.Config) error {
	ms.savedConfig = cfg.Clone()

	return nil
}

// Load applies environment overrides to the default config as if a re-load had occurred.
func (ms *MemoryStore) Load() ([]byte, error) {
	cfgBytes, err := marshalConfig(ms.savedConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize config")
	}

	return cfgBytes, nil

}

// GetFile fetches the contents of a previously persisted configuration file.
func (ms *MemoryStore) GetFile(name string) ([]byte, error) {
	data, ok := ms.files[name]
	if !ok {
		return nil, fmt.Errorf("file %s not stored", name)
	}

	return data, nil
}

// SetFile sets or replaces the contents of a configuration file.
func (ms *MemoryStore) SetFile(name string, data []byte) error {
	ms.files[name] = data

	return nil
}

// HasFile returns true if the given file was previously persisted.
func (ms *MemoryStore) HasFile(name string) (bool, error) {
	_, ok := ms.files[name]
	return ok, nil
}

// RemoveFile removes a previously persisted configuration file.
func (ms *MemoryStore) RemoveFile(name string) error {
	delete(ms.files, name)

	return nil
}

// String returns a hard-coded description, as there is no backing store.
func (ms *MemoryStore) String() string {
	return "memory://"
}

// Close does nothing for a memory store.
func (ms *MemoryStore) Close() error {
	return nil
}
