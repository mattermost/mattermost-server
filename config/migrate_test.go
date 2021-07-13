// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

type cleanUpFn func(store *Store)

func TestMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping migration test in short mode")
	}
	files := []string{
		"IdpCertificateFile",
		"PublicCertificateFile",
		"PrivateKeyFile",
		"internal.crt",
		"internal2.crt",
	}
	filesData := make([]string, len(files))
	for i := range files {
		// Generate random data for each file, ensuring that stale data from a past test
		// won't generate a false positive.
		filesData[i] = model.NewId()
	}

	setup := func(t *testing.T) {
		os.Clearenv()
		t.Helper()

		tempDir, err := ioutil.TempDir("", "TestMigrate")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(tempDir)
		})

		err = os.Chdir(tempDir)
		require.NoError(t, err)

		truncateTables(t)
	}

	setupSource := func(t *testing.T, source *Store) cleanUpFn {
		t.Helper()

		cfg := source.Get()
		originalCfg := cfg.Clone()
		cfg.ServiceSettings.SiteURL = model.NewString("http://example.com")
		cfg.SamlSettings.IdpCertificateFile = &files[0]
		cfg.SamlSettings.PublicCertificateFile = &files[1]
		cfg.SamlSettings.PrivateKeyFile = &files[2]
		cfg.PluginSettings.SignaturePublicKeyFiles = []string{
			files[3],
			files[4],
		}
		cfg.SqlSettings.DataSourceReplicas = []string{
			"mysql://mmuser:password@tcp(replicahost:3306)/mattermost",
		}
		cfg.SqlSettings.DataSourceSearchReplicas = []string{
			"mysql://mmuser:password@tcp(searchreplicahost:3306)/mattermost",
		}

		_, _, err := source.Set(cfg)
		require.NoError(t, err)

		for i, file := range files {
			err = source.SetFile(file, []byte(filesData[i]))
			require.NoError(t, err)
		}

		return func(store *Store) {
			_, _, err := store.Set(originalCfg)
			require.NoError(t, err)
		}
	}

	assertDestination := func(t *testing.T, destination *Store, source *Store) {
		t.Helper()

		for i, file := range files {
			hasFile, err := destination.HasFile(file)
			require.NoError(t, err)
			require.Truef(t, hasFile, "destination missing file %s", file)

			actualData, err := destination.GetFile(file)
			require.NoError(t, err)
			assert.Equalf(t, []byte(filesData[i]), actualData, "destination has wrong contents for file %s", file)
		}

		assert.Equal(t, source.Get(), destination.Get())
	}

	t.Run("database to file", func(t *testing.T) {
		setup(t)

		pwd, err := os.Getwd()
		require.NoError(t, err)

		sqlSettings := mainHelper.GetSQLSettings()
		destinationDSN := path.Join(pwd, "config-custom.json")
		sourceDSN := getDsn(*sqlSettings.DriverName, *sqlSettings.DataSource)

		sourcedb, err := NewDatabaseStore(sourceDSN)
		require.NoError(t, err)
		source, err := NewStoreFromBacking(sourcedb, nil, false)
		require.NoError(t, err)
		defer source.Close()

		cleanUp := setupSource(t, source)
		err = Migrate(sourceDSN, destinationDSN)
		require.NoError(t, err)

		destinationfile, err := NewFileStore(destinationDSN)
		require.NoError(t, err)
		destination, err := NewStoreFromBacking(destinationfile, nil, false)
		require.NoError(t, err)
		defer destination.Close()
		defer cleanUp(destination)

		assertDestination(t, destination, source)
	})

	t.Run("file to database", func(t *testing.T) {
		setup(t)

		pwd, err := os.Getwd()
		require.NoError(t, err)

		sqlSettings := mainHelper.GetSQLSettings()
		sourceDSN := path.Join(pwd, "config-custom.json")
		destinationDSN := getDsn(*sqlSettings.DriverName, *sqlSettings.DataSource)

		sourcefile, err := NewFileStore(sourceDSN)
		require.NoError(t, err)
		source, err := NewStoreFromBacking(sourcefile, nil, false)
		require.NoError(t, err)
		defer source.Close()

		cleanUp := setupSource(t, source)
		err = Migrate(sourceDSN, destinationDSN)
		require.NoError(t, err)

		destinationdb, err := NewDatabaseStore(destinationDSN)
		require.NoError(t, err)
		destination, err := NewStoreFromBacking(destinationdb, nil, false)
		require.NoError(t, err)
		defer destination.Close()
		defer cleanUp(destination)

		assertDestination(t, destination, source)
	})
}
