// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

var (
	// ErrReadOnlyConfiguration is returned when an attempt to modify a read-only configuration is made.
	ErrReadOnlyConfiguration = errors.New("configuration is read-only")
)

// FileStore is a config store backed by a file such as config/config.json.
//
// It also uses the folder containing the configuration file for storing other configuration files.
// Not to be used directly. Only to be used as a backing store for config.Store
type FileStore struct {
	path string
}

// NewFileStore creates a new instance of a config store backed by the given file path.
func NewFileStore(path string) (fs *FileStore, err error) {
	resolvedPath, err := resolveConfigFilePath(path)
	if err != nil {
		return nil, err
	}

	return &FileStore{
		path: resolvedPath,
	}, nil
}

// resolveConfigFilePath attempts to resolve the given configuration file path to an absolute path.
//
// Consideration is given to maintaining backwards compatibility when resolving the path to the
// configuration file.
func resolveConfigFilePath(path string) (string, error) {
	// Absolute paths are explicit and require no resolution.
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Search for the relative path to the file in the config folder, taking into account
	// various common starting points.
	if configFile := fileutils.FindFile(filepath.Join("config", path)); configFile != "" {
		return configFile, nil
	}

	// Search for the relative path in the current working directory, also taking into account
	// various common starting points.
	if configFile := fileutils.FindPath(path, []string{"."}, nil); configFile != "" {
		return configFile, nil
	}

	// Otherwise, search for the config/ folder using the same heuristics as above, and build
	// an absolute path anchored there and joining the given input path (or plain filename).
	if configFolder, found := fileutils.FindDir("config"); found {
		return filepath.Join(configFolder, path), nil
	}

	// Fail altogether if we can't even find the config/ folder. This should only happen if
	// the executable is relocated away from the supporting files.
	return "", fmt.Errorf("failed to find config file %s", path)
}

// resolveFilePath uses the name if name is absolute path.
// otherwise returns the combined path/name
func (fs *FileStore) resolveFilePath(name string) string {
	// Absolute paths are explicit and require no resolution.
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(filepath.Dir(fs.path), name)
}

// Set replaces the current configuration in its entirety and updates the backing store.
func (fs *FileStore) Set(newCfg *model.Config) error {
	if *newCfg.ClusterSettings.Enable && *newCfg.ClusterSettings.ReadOnlyConfig {
		return ErrReadOnlyConfiguration
	}

	return fs.persist(newCfg)
}

// persist writes the configuration to the configured file.
func (fs *FileStore) persist(cfg *model.Config) error {
	b, err := marshalConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to serialize")
	}

	err = ioutil.WriteFile(fs.path, b, 0600)
	if err != nil {
		return errors.Wrap(err, "failed to write file")
	}

	return nil
}

// Load updates the current configuration from the backing store.
func (fs *FileStore) Load() ([]byte, error) {
	f, err := os.Open(fs.path)
	if os.IsNotExist(err) {
		return nil, nil

	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s for reading", fs.path)
	}
	defer f.Close()

	fileBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return fileBytes, nil
}

// GetFile fetches the contents of a previously persisted configuration file.
func (fs *FileStore) GetFile(name string) ([]byte, error) {
	resolvedPath := fs.resolveFilePath(name)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file from %s", resolvedPath)
	}

	return data, nil
}

// GetFilePath returns the resolved path of a configuration file.
// The file may not necessarily exist.
func (fs *FileStore) GetFilePath(name string) string {
	return fs.resolveFilePath(name)
}

// SetFile sets or replaces the contents of a configuration file.
func (fs *FileStore) SetFile(name string, data []byte) error {
	resolvedPath := fs.resolveFilePath(name)

	err := ioutil.WriteFile(resolvedPath, data, 0600)
	if err != nil {
		return errors.Wrapf(err, "failed to write file to %s", resolvedPath)
	}

	return nil
}

// HasFile returns true if the given file was previously persisted.
func (fs *FileStore) HasFile(name string) (bool, error) {
	if name == "" {
		return false, nil
	}

	resolvedPath := fs.resolveFilePath(name)

	_, err := os.Stat(resolvedPath)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, "failed to check if file exists")
	}

	return true, nil
}

// RemoveFile removes a previously persisted configuration file.
func (fs *FileStore) RemoveFile(name string) error {
	if filepath.IsAbs(name) {
		// Don't delete absolute filenames, as may be mounted drive, etc.
		mlog.Debug("Skipping removal of configuration file with absolute path", mlog.String("filename", name))
		return nil
	}
	resolvedPath := filepath.Join(filepath.Dir(fs.path), name)

	err := os.Remove(resolvedPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to remove file")
	}

	return nil
}

// String returns the path to the file backing the config.
func (fs *FileStore) String() string {
	return "file://" + fs.path
}

// Close cleans up resources associated with the store.
func (fs *FileStore) Close() error {
	return nil
}
