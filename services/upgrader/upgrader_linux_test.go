// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package upgrader

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestCanIUpgradeToE0(t *testing.T) {
	t.Run("when you are already in an enterprise build", func(t *testing.T) {
		buildEnterprise := model.BuildEnterpriseReady
		model.BuildEnterpriseReady = "true"
		defer func() {
			model.BuildEnterpriseReady = buildEnterprise
		}()
		require.Error(t, CanIUpgradeToE0())
	})

	t.Run("when you are not in an enterprise build", func(t *testing.T) {
		buildEnterprise := model.BuildEnterpriseReady
		model.BuildEnterpriseReady = "false"
		defer func() {
			model.BuildEnterpriseReady = buildEnterprise
		}()
		require.NoError(t, CanIUpgradeToE0())
	})
}

func TestGetCurrentVersionTgzURL(t *testing.T) {
	t.Run("get release version in regular version", func(t *testing.T) {
		currentVersion := model.CurrentVersion
		buildNumber := model.CurrentVersion
		model.CurrentVersion = "5.22.0"
		model.BuildNumber = "5.22.0"
		defer func() {
			model.CurrentVersion = currentVersion
			model.BuildNumber = buildNumber
		}()
		require.Equal(t, "https://releases.mattermost.com/5.22.0/mattermost-5.22.0-linux-amd64.tar.gz", getCurrentVersionTgzURL())
	})

	t.Run("get release version in dev version", func(t *testing.T) {
		currentVersion := model.CurrentVersion
		buildNumber := model.CurrentVersion
		model.CurrentVersion = "5.22.0"
		model.BuildNumber = "5.22.0-dev"
		defer func() {
			model.CurrentVersion = currentVersion
			model.BuildNumber = buildNumber
		}()
		require.Equal(t, "https://releases.mattermost.com/5.22.0/mattermost-5.22.0-linux-amd64.tar.gz", getCurrentVersionTgzURL())
	})

	t.Run("get release version in rc version", func(t *testing.T) {
		currentVersion := model.CurrentVersion
		buildNumber := model.CurrentVersion
		model.CurrentVersion = "5.22.0"
		model.BuildNumber = "5.22.0-rc2"
		defer func() {
			model.CurrentVersion = currentVersion
			model.BuildNumber = buildNumber
		}()
		require.Equal(t, "https://releases.mattermost.com/5.22.0-rc2/mattermost-5.22.0-rc2-linux-amd64.tar.gz", getCurrentVersionTgzURL())
	})
}

func TestExtractBinary(t *testing.T) {
	t.Run("extract from empty file", func(t *testing.T) {
		tmpMockTarGz, err := ioutil.TempFile("", "mock_tgz")
		require.NoError(t, err)
		defer os.Remove(tmpMockTarGz.Name())
		tmpMockTarGz.Close()

		tmpMockExecutable, err := ioutil.TempFile("", "mock_exe")
		require.NoError(t, err)
		defer os.Remove(tmpMockExecutable.Name())
		tmpMockExecutable.Close()

		extractBinary(tmpMockExecutable.Name(), tmpMockTarGz.Name())
	})

	t.Run("extract from empty tar.gz file", func(t *testing.T) {
		tmpMockTarGz, err := ioutil.TempFile("", "mock_tgz")
		require.NoError(t, err)
		defer os.Remove(tmpMockTarGz.Name())
		gz := gzip.NewWriter(tmpMockTarGz)
		tw := tar.NewWriter(gz)
		tw.Close()
		gz.Close()
		tmpMockTarGz.Close()

		tmpMockExecutable, err := ioutil.TempFile("", "mock_exe")
		require.NoError(t, err)
		defer os.Remove(tmpMockExecutable.Name())
		tmpMockExecutable.Close()

		require.Error(t, extractBinary(tmpMockExecutable.Name(), tmpMockTarGz.Name()))
	})

	t.Run("extract from tar.gz without mattermost/bin/mattermost file", func(t *testing.T) {
		tmpMockTarGz, err := ioutil.TempFile("", "mock_tgz")
		require.NoError(t, err)
		defer os.Remove(tmpMockTarGz.Name())
		gz := gzip.NewWriter(tmpMockTarGz)
		tw := tar.NewWriter(gz)

		tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     "test-filename",
			Size:     4,
		})
		tw.Write([]byte("test"))

		gz.Close()
		tmpMockTarGz.Close()

		tmpMockExecutable, err := ioutil.TempFile("", "mock_exe")
		require.NoError(t, err)
		defer os.Remove(tmpMockExecutable.Name())
		tmpMockExecutable.Close()

		require.Error(t, extractBinary(tmpMockExecutable.Name(), tmpMockTarGz.Name()))
	})

	t.Run("extract from tar.gz with mattermost/bin/mattermost file", func(t *testing.T) {
		tmpMockTarGz, err := ioutil.TempFile("", "mock_tgz")
		require.NoError(t, err)
		defer os.Remove(tmpMockTarGz.Name())
		gz := gzip.NewWriter(tmpMockTarGz)
		tw := tar.NewWriter(gz)

		tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     "mattermost/bin/mattermost",
			Size:     4,
		})
		tw.Write([]byte("test"))

		gz.Close()
		tmpMockTarGz.Close()

		tmpMockExecutable, err := ioutil.TempFile("", "mock_exe")
		require.NoError(t, err)
		defer os.Remove(tmpMockExecutable.Name())
		tmpMockExecutable.Close()

		require.NoError(t, extractBinary(tmpMockExecutable.Name(), tmpMockTarGz.Name()))
		tmpMockExecutableAfter, err := os.Open(tmpMockExecutable.Name())
		require.NoError(t, err)
		defer tmpMockExecutableAfter.Close()
		bytes, err := ioutil.ReadAll(tmpMockExecutableAfter)
		require.NoError(t, err)
		require.Equal(t, []byte("test"), bytes)
	})
}
