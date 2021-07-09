// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package testlib

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/filestore"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

const (
	resourceTypeFile = iota
	resourceTypeFolder
)

const (
	actionCopy = iota
	actionSymlink
)

const root = "___mattermost-server"

type testResourceDetails struct {
	src     string
	dest    string
	resType int8
	action  int8
}

// getCommonBaseSearchPaths() is a custom version of what fileutils exposes. At some point, consolidate.
func getCommonBaseSearchPaths() []string {
	paths := []string{
		".",
		"..",
		"../..",
		"../../..",
		"../../../..",
	}

	// this enables the server to be used in tests from a different repository
	if mmPath := os.Getenv("MM_SERVER_PATH"); mmPath != "" {
		paths = append(paths, mmPath)
	}

	return paths
}

func findFile(path string) string {
	return fileutils.FindPath(path, getCommonBaseSearchPaths(), func(fileInfo os.FileInfo) bool {
		return !fileInfo.IsDir()
	})
}

func findDir(dir string) (string, bool) {
	if dir == root {
		srcPath := findFile("go.mod")
		if srcPath == "" {
			return "./", false
		}

		return path.Dir(srcPath), true
	}

	found := fileutils.FindPath(dir, getCommonBaseSearchPaths(), func(fileInfo os.FileInfo) bool {
		return fileInfo.IsDir()
	})
	if found == "" {
		return "./", false
	}

	return found, true
}

func getTestResourcesToSetup() []testResourceDetails {
	var srcPath string
	var found bool

	var testResourcesToSetup = []testResourceDetails{
		{root, "mattermost-server", resourceTypeFolder, actionSymlink},
		{"go.mod", "go.mod", resourceTypeFile, actionSymlink},
		{"i18n", "i18n", resourceTypeFolder, actionSymlink},
		{"templates", "templates", resourceTypeFolder, actionSymlink},
		{"tests", "tests", resourceTypeFolder, actionSymlink},
		{"fonts", "fonts", resourceTypeFolder, actionSymlink},
		{"utils/policies-roles-mapping.json", "utils/policies-roles-mapping.json", resourceTypeFile, actionSymlink},
	}

	// Finding resources and setting full path to source to be used for further processing
	for i, testResource := range testResourcesToSetup {
		if testResource.resType == resourceTypeFile {
			srcPath = findFile(testResource.src)
			if srcPath == "" {
				panic(fmt.Sprintf("Failed to find file %s", testResource.src))
			}

			testResourcesToSetup[i].src = srcPath
		} else if testResource.resType == resourceTypeFolder {
			srcPath, found = findDir(testResource.src)
			if !found {
				panic(fmt.Sprintf("Failed to find folder %s", testResource.src))
			}

			testResourcesToSetup[i].src = srcPath
		} else {
			panic(fmt.Sprintf("Invalid resource type: %d", testResource.resType))
		}
	}

	return testResourcesToSetup
}

func CopyFile(src, dst string) error {
	fileBackend, err := filestore.NewFileBackend(filestore.FileBackendSettings{DriverName: "local", Directory: ""})
	if err != nil {
		return errors.Wrapf(err, "failed to copy file %s to %s", src, dst)
	}
	if err = fileBackend.CopyFile(src, dst); err != nil {
		return errors.Wrapf(err, "failed to copy file %s to %s", src, dst)
	}
	return nil
}

func SetupTestResources() (string, error) {
	testResourcesToSetup := getTestResourcesToSetup()

	tempDir, err := ioutil.TempDir("", "testlib")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary directory")
	}

	pluginsDir := path.Join(tempDir, "plugins")
	err = os.Mkdir(pluginsDir, 0700)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create plugins directory %s", pluginsDir)
	}

	clientDir := path.Join(tempDir, "client")
	err = os.Mkdir(clientDir, 0700)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create client directory %s", clientDir)
	}

	err = setupConfig(path.Join(tempDir, "config"))
	if err != nil {
		return "", errors.Wrap(err, "failed to setup config")
	}

	var resourceDestInTemp string

	// Setting up test resources in temp.
	// Action in each resource tells whether it needs to be copied or just symlinked
	for _, testResource := range testResourcesToSetup {
		resourceDestInTemp = filepath.Join(tempDir, testResource.dest)

		if testResource.action == actionCopy {
			if testResource.resType == resourceTypeFile {
				if err = CopyFile(testResource.src, resourceDestInTemp); err != nil {
					return "", err
				}
			} else if testResource.resType == resourceTypeFolder {
				err = utils.CopyDir(testResource.src, resourceDestInTemp)
				if err != nil {
					return "", errors.Wrapf(err, "failed to copy folder %s to %s", testResource.src, resourceDestInTemp)
				}
			}
		} else if testResource.action == actionSymlink {
			destDir := path.Dir(resourceDestInTemp)
			if destDir != "." {
				err = os.MkdirAll(destDir, os.ModePerm)
				if err != nil {
					return "", errors.Wrapf(err, "failed to make dir %s", destDir)
				}
			}

			err = os.Symlink(testResource.src, resourceDestInTemp)
			if err != nil {
				return "", errors.Wrapf(err, "failed to symlink %s to %s", testResource.src, resourceDestInTemp)
			}
		} else {
			return "", errors.Wrapf(err, "Invalid action: %d", testResource.action)
		}

	}

	return tempDir, nil
}

func setupConfig(configDir string) error {
	var err error
	var config model.Config

	config.SetDefaults()

	err = os.Mkdir(configDir, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create config directory %s", configDir)
	}

	configJSON := path.Join(configDir, "config.json")
	err = ioutil.WriteFile(configJSON, []byte(config.ToJSON()), 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write config to %s", configJSON)
	}

	return nil
}
