// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"

	"github.com/stretchr/testify/require"
)

func TestListExports(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("no permissions", func(t *testing.T) {
		exports, resp := th.Client.ListExports()
		require.NotNil(t, resp.Error)
		require.Equal(t, "api.context.permissions.app_error", resp.Error.ID)
		require.Nil(t, exports)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		exports, resp := c.ListExports()
		require.Nil(t, resp.Error)
		require.Empty(t, exports)
	}, "no exports")

	dataDir, found := fileutils.FindDir("data")
	require.True(t, found)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		exportDir := filepath.Join(dataDir, *th.App.Config().ExportSettings.Directory)
		err := os.Mkdir(exportDir, 0700)
		require.NoError(t, err)
		defer os.RemoveAll(exportDir)

		f, err := os.Create(filepath.Join(exportDir, "export.zip"))
		require.NoError(t, err)
		f.Close()

		exports, resp := c.ListExports()
		require.Nil(t, resp.Error)
		require.Len(t, exports, 1)
		require.Equal(t, exports[0], "export.zip")
	}, "expected exports")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		value := *th.App.Config().ExportSettings.Directory
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExportSettings.Directory = value + "new" })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExportSettings.Directory = value })

		exportDir := filepath.Join(dataDir, value+"new")
		err := os.Mkdir(exportDir, 0700)
		require.NoError(t, err)
		defer os.RemoveAll(exportDir)

		exports, resp := c.ListExports()
		require.Nil(t, resp.Error)
		require.Empty(t, exports)

		f, err := os.Create(filepath.Join(exportDir, "export.zip"))
		require.NoError(t, err)
		f.Close()

		exports, resp = c.ListExports()
		require.Nil(t, resp.Error)
		require.Len(t, exports, 1)
		require.Equal(t, "export.zip", exports[0])
	}, "change export directory")
}

func TestDeleteExport(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("no permissions", func(t *testing.T) {
		ok, resp := th.Client.DeleteExport("export.zip")
		require.NotNil(t, resp.Error)
		require.Equal(t, "api.context.permissions.app_error", resp.Error.ID)
		require.False(t, ok)
	})

	dataDir, found := fileutils.FindDir("data")
	require.True(t, found)
	exportDir := filepath.Join(dataDir, *th.App.Config().ExportSettings.Directory)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		err := os.Mkdir(exportDir, 0700)
		require.NoError(t, err)
		defer os.RemoveAll(exportDir)
		exportName := "export.zip"
		f, err := os.Create(filepath.Join(exportDir, exportName))
		require.NoError(t, err)
		f.Close()

		exports, resp := c.ListExports()
		require.Nil(t, resp.Error)
		require.Len(t, exports, 1)
		require.Equal(t, exports[0], exportName)

		ok, resp := c.DeleteExport(exportName)
		require.Nil(t, resp.Error)
		require.True(t, ok)

		exports, resp = c.ListExports()
		require.Nil(t, resp.Error)
		require.Empty(t, exports)

		// verify idempotence
		ok, resp = c.DeleteExport(exportName)
		require.Nil(t, resp.Error)
		require.True(t, ok)
	}, "successfully delete export")
}

func TestDownloadExport(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("no permissions", func(t *testing.T) {
		var buf bytes.Buffer
		n, resp := th.Client.DownloadExport("export.zip", &buf, 0)
		require.NotNil(t, resp.Error)
		require.Equal(t, "api.context.permissions.app_error", resp.Error.ID)
		require.Zero(t, n)
	})

	dataDir, found := fileutils.FindDir("data")
	require.True(t, found)
	exportDir := filepath.Join(dataDir, *th.App.Config().ExportSettings.Directory)

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		var buf bytes.Buffer
		n, resp := c.DownloadExport("export.zip", &buf, 0)
		require.NotNil(t, resp.Error)
		require.Equal(t, "api.export.export_not_found.app_error", resp.Error.ID)
		require.Zero(t, n)
	}, "not found")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		err := os.Mkdir(exportDir, 0700)
		require.NoError(t, err)
		defer os.RemoveAll(exportDir)

		data := randomBytes(t, 1024*1024)
		var buf bytes.Buffer
		exportName := "export.zip"
		err = ioutil.WriteFile(filepath.Join(exportDir, exportName), data, 0600)
		require.NoError(t, err)

		n, resp := c.DownloadExport(exportName, &buf, 0)
		require.Nil(t, resp.Error)
		require.Equal(t, len(data), int(n))
		require.Equal(t, data, buf.Bytes())
	}, "full download")

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		err := os.Mkdir(exportDir, 0700)
		require.NoError(t, err)
		defer os.RemoveAll(exportDir)

		data := randomBytes(t, 1024*1024)
		var buf bytes.Buffer
		exportName := "export.zip"
		err = ioutil.WriteFile(filepath.Join(exportDir, exportName), data, 0600)
		require.NoError(t, err)

		offset := 1024 * 512
		n, resp := c.DownloadExport(exportName, &buf, int64(offset))
		require.Nil(t, resp.Error)
		require.Equal(t, len(data)-offset, int(n))
		require.Equal(t, data[offset:], buf.Bytes())
	}, "download with offset")
}

func BenchmarkDownloadExport(b *testing.B) {
	th := Setup(b)
	defer th.TearDown()

	dataDir, found := fileutils.FindDir("data")
	require.True(b, found)
	exportDir := filepath.Join(dataDir, *th.App.Config().ExportSettings.Directory)

	err := os.Mkdir(exportDir, 0700)
	require.NoError(b, err)
	defer os.RemoveAll(exportDir)

	exportName := "export.zip"
	f, err := os.Create(filepath.Join(exportDir, exportName))
	require.NoError(b, err)
	f.Close()

	err = os.Truncate(filepath.Join(exportDir, exportName), 1024*1024*1024)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outFilePath := filepath.Join(dataDir, fmt.Sprintf("export%d.zip", i))
		outFile, _ := os.Create(outFilePath)
		th.SystemAdminClient.DownloadExport(exportName, outFile, 0)
		outFile.Close()
		os.Remove(outFilePath)
	}
}
