// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewStoreFromDSN(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	sqlSettings := mainHelper.GetSQLSettings()

	tempDir, err := ioutil.TempDir("", "TestNewStore")
	require.NoError(t, err)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "config"), 0700))

	t.Run("database dsn", func(t *testing.T) {
		ds, err := NewStoreFromDSN(getDsn(*sqlSettings.DriverName, *sqlSettings.DataSource), false, nil)
		require.NoError(t, err)
		ds.Close()
	})

	t.Run("file dsn", func(t *testing.T) {
		fs, err := NewStoreFromDSN("config.json", false, nil)
		require.NoError(t, err)
		fs.Close()
	})
}

func TestNewStoreReadOnly(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	sqlSettings := mainHelper.GetSQLSettings()

	tempDir, err := ioutil.TempDir("", "TestNewStore")
	require.NoError(t, err)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "config"), 0700))

	t.Run("database dsn", func(t *testing.T) {
		ds, err := NewStoreFromDSN(getDsn(*sqlSettings.DriverName, *sqlSettings.DataSource), true, nil)
		require.NoError(t, err)

		t.Run("Set", func(t *testing.T) {
			oldCfg, newCfg, err := ds.Set(emptyConfig)
			require.Nil(t, oldCfg)
			require.Nil(t, newCfg)
			require.Equal(t, ErrReadOnlyStore, err)
		})

		t.Run("SetFile", func(t *testing.T) {
			err := ds.SetFile("config.json", []byte{})
			require.Equal(t, ErrReadOnlyStore, err)
		})

		t.Run("RemoveFile", func(t *testing.T) {
			err := ds.RemoveFile("config.json")
			require.Equal(t, ErrReadOnlyStore, err)
		})

		ds.Close()
	})

	t.Run("file dsn", func(t *testing.T) {
		fs, err := NewStoreFromDSN("config.json", true, nil)
		require.NoError(t, err)

		t.Run("Set", func(t *testing.T) {
			oldCfg, newCfg, err := fs.Set(emptyConfig)
			require.Nil(t, oldCfg)
			require.Nil(t, newCfg)
			require.Equal(t, ErrReadOnlyStore, err)
		})

		t.Run("SetFile", func(t *testing.T) {
			err := fs.SetFile("config.json", []byte{})
			require.Equal(t, ErrReadOnlyStore, err)
		})

		t.Run("RemoveFile", func(t *testing.T) {
			err := fs.RemoveFile("config.json")
			require.Equal(t, ErrReadOnlyStore, err)
		})

		fs.Close()
	})
}
