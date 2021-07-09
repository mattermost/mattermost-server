// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

type nilReadSeeker struct {
}

func (r *nilReadSeeker) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (r *nilReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

type testFile struct {
	Name, Body string
}

func makeInMemoryGzipTarFile(t *testing.T, files []testFile) *bytes.Reader {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)

	tgz := tar.NewWriter(gzWriter)

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}
		err := tgz.WriteHeader(hdr)
		require.NoError(t, err, "failed to write %s to in-memory tar file", file.Name)
		_, err = tgz.Write([]byte(file.Body))
		require.NoError(t, err, "failed to write body of %s to in-memory tar file", file.Name)
	}
	err := tgz.Close()
	require.NoError(t, err, "failed to close in-memory tar file")

	err = gzWriter.Close()
	require.NoError(t, err, "failed to close in-memory tar.gz file")

	return bytes.NewReader(buf.Bytes())
}

type byBundleInfoID []*model.BundleInfo

func (b byBundleInfoID) Len() int           { return len(b) }
func (b byBundleInfoID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byBundleInfoID) Less(i, j int) bool { return b[i].Manifest.ID < b[j].Manifest.ID }

func TestInstallPluginLocally(t *testing.T) {
	t.Run("invalid tar", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()

		actualManifest, appErr := th.App.installPluginLocally(&nilReadSeeker{}, nil, installPluginLocallyOnlyIfNew)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.plugin.extract.app_error", appErr.ID, appErr.Error())
		require.Nil(t, actualManifest)
	})

	t.Run("missing manifest", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()

		reader := makeInMemoryGzipTarFile(t, []testFile{
			{"test", "test file"},
		})

		actualManifest, appErr := th.App.installPluginLocally(reader, nil, installPluginLocallyOnlyIfNew)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.plugin.manifest.app_error", appErr.ID, appErr.Error())
		require.Nil(t, actualManifest)
	})

	installPlugin := func(t *testing.T, th *TestHelper, id, version string, installationStrategy pluginInstallationStrategy) (*model.Manifest, *model.AppError) {
		t.Helper()

		manifest := &model.Manifest{
			ID:      id,
			Version: version,
		}
		reader := makeInMemoryGzipTarFile(t, []testFile{
			{"plugin.json", manifest.ToJSON()},
		})

		actualManifest, appError := th.App.installPluginLocally(reader, nil, installationStrategy)
		if actualManifest != nil {
			require.Equal(t, manifest, actualManifest)
		}

		return actualManifest, appError
	}

	t.Run("invalid plugin id", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()

		actualManifest, appErr := installPlugin(t, th, "invalid#plugin#id", "version", installPluginLocallyOnlyIfNew)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.plugin.invalid_id.app_error", appErr.ID, appErr.Error())
		require.Nil(t, actualManifest)
	})

	// The following tests fail mysteriously on CI due to an unexpected bundle being present.
	// This exists to clean up manually until we figure out what test isn't cleaning up after
	// itself.
	cleanExistingBundles := func(t *testing.T, th *TestHelper) {
		pluginsEnvironment := th.App.GetPluginsEnvironment()
		require.NotNil(t, pluginsEnvironment)
		bundleInfos, err := pluginsEnvironment.Available()
		require.NoError(t, err)

		for _, bundleInfo := range bundleInfos {
			err := th.App.removePluginLocally(bundleInfo.Manifest.ID)
			require.Nilf(t, err, "failed to remove existing plugin %s", bundleInfo.Manifest.ID)
		}
	}

	assertBundleInfoManifests := func(t *testing.T, th *TestHelper, manifests []*model.Manifest) {
		pluginsEnvironment := th.App.GetPluginsEnvironment()
		require.NotNil(t, pluginsEnvironment)
		bundleInfos, err := pluginsEnvironment.Available()
		require.NoError(t, err)

		sort.Sort(byBundleInfoID(bundleInfos))

		actualManifests := make([]*model.Manifest, 0, len(bundleInfos))
		for _, bundleInfo := range bundleInfos {
			actualManifests = append(actualManifests, bundleInfo.Manifest)
		}

		require.Equal(t, manifests, actualManifests)
	}

	t.Run("no plugins already installed", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()
		cleanExistingBundles(t, th)

		manifest, appErr := installPlugin(t, th, "valid", "0.0.1", installPluginLocallyOnlyIfNew)
		require.Nil(t, appErr)
		require.NotNil(t, manifest)

		assertBundleInfoManifests(t, th, []*model.Manifest{manifest})
	})

	t.Run("different plugin already installed", func(t *testing.T) {
		th := Setup(t)
		defer th.TearDown()
		cleanExistingBundles(t, th)

		otherManifest, appErr := installPlugin(t, th, "other", "0.0.1", installPluginLocallyOnlyIfNew)
		require.Nil(t, appErr)
		require.NotNil(t, otherManifest)

		manifest, appErr := installPlugin(t, th, "valid", "0.0.1", installPluginLocallyOnlyIfNew)
		require.Nil(t, appErr)
		require.NotNil(t, manifest)

		assertBundleInfoManifests(t, th, []*model.Manifest{otherManifest, manifest})
	})

	t.Run("same plugin already installed", func(t *testing.T) {
		t.Run("install only if new", func(t *testing.T) {
			th := Setup(t)
			defer th.TearDown()
			cleanExistingBundles(t, th)

			existingManifest, appErr := installPlugin(t, th, "valid", "0.0.1", installPluginLocallyOnlyIfNew)
			require.Nil(t, appErr)
			require.NotNil(t, existingManifest)

			manifest, appErr := installPlugin(t, th, "valid", "0.0.1", installPluginLocallyOnlyIfNew)
			require.NotNil(t, appErr)
			require.Equal(t, "app.plugin.install_id.app_error", appErr.ID, appErr.Error())
			require.Nil(t, manifest)

			assertBundleInfoManifests(t, th, []*model.Manifest{existingManifest})
		})

		t.Run("install if upgrade, but older", func(t *testing.T) {
			th := Setup(t)
			defer th.TearDown()
			cleanExistingBundles(t, th)

			existingManifest, appErr := installPlugin(t, th, "valid", "0.0.2", installPluginLocallyOnlyIfNewOrUpgrade)
			require.Nil(t, appErr)
			require.NotNil(t, existingManifest)

			manifest, appErr := installPlugin(t, th, "valid", "0.0.1", installPluginLocallyOnlyIfNewOrUpgrade)
			require.Nil(t, appErr)
			require.Nil(t, manifest)

			assertBundleInfoManifests(t, th, []*model.Manifest{existingManifest})
		})

		t.Run("install if upgrade, but same version", func(t *testing.T) {
			th := Setup(t)
			defer th.TearDown()
			cleanExistingBundles(t, th)

			existingManifest, appErr := installPlugin(t, th, "valid", "0.0.2", installPluginLocallyOnlyIfNewOrUpgrade)
			require.Nil(t, appErr)
			require.NotNil(t, existingManifest)

			manifest, appErr := installPlugin(t, th, "valid", "0.0.2", installPluginLocallyOnlyIfNewOrUpgrade)
			require.Nil(t, appErr)
			require.Nil(t, manifest)

			assertBundleInfoManifests(t, th, []*model.Manifest{existingManifest})
		})

		t.Run("install if upgrade, newer version", func(t *testing.T) {
			th := Setup(t)
			defer th.TearDown()
			cleanExistingBundles(t, th)

			existingManifest, appErr := installPlugin(t, th, "valid", "0.0.2", installPluginLocallyOnlyIfNewOrUpgrade)
			require.Nil(t, appErr)
			require.NotNil(t, existingManifest)

			manifest, appErr := installPlugin(t, th, "valid", "0.0.3", installPluginLocallyOnlyIfNewOrUpgrade)
			require.Nil(t, appErr)
			require.NotNil(t, manifest)

			assertBundleInfoManifests(t, th, []*model.Manifest{manifest})
		})

		t.Run("install always, old version", func(t *testing.T) {
			th := Setup(t)
			defer th.TearDown()
			cleanExistingBundles(t, th)

			existingManifest, appErr := installPlugin(t, th, "valid", "0.0.2", installPluginLocallyAlways)
			require.Nil(t, appErr)
			require.NotNil(t, existingManifest)

			manifest, appErr := installPlugin(t, th, "valid", "0.0.1", installPluginLocallyAlways)
			require.Nil(t, appErr)
			require.NotNil(t, manifest)

			assertBundleInfoManifests(t, th, []*model.Manifest{manifest})
		})
	})
}

func TestInstallPluginAlreadyActive(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	path, _ := fileutils.FindDir("tests")
	reader, err := os.Open(filepath.Join(path, "testplugin.tar.gz"))
	require.NoError(t, err)

	actualManifest, appError := th.App.InstallPlugin(reader, true)
	require.NotNil(t, actualManifest)
	require.Nil(t, appError)
	appError = th.App.EnablePlugin(actualManifest.ID)
	require.Nil(t, appError)

	pluginsEnvironment := th.App.GetPluginsEnvironment()
	require.NotNil(t, pluginsEnvironment)
	bundleInfos, err := pluginsEnvironment.Available()
	require.NoError(t, err)
	require.NotEmpty(t, bundleInfos)
	for _, bundleInfo := range bundleInfos {
		if bundleInfo.Manifest.ID == actualManifest.ID {
			err := os.RemoveAll(bundleInfo.Path)
			require.NoError(t, err)
		}
	}

	actualManifest, appError = th.App.InstallPlugin(reader, true)
	require.NotNil(t, appError)
	require.Nil(t, actualManifest)
	require.Equal(t, "app.plugin.restart.app_error", appError.ID)
}
