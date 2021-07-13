// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func setupConfigFile(t *testing.T, cfg *model.Config) (string, func()) {
	os.Clearenv()
	t.Helper()

	tempDir, err := ioutil.TempDir("", "setupConfigFile")
	require.NoError(t, err)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	var name string
	if cfg != nil {
		f, err := ioutil.TempFile(tempDir, "setupConfigFile")
		require.NoError(t, err)

		cfgData, err := marshalConfig(cfg)
		require.NoError(t, err)

		ioutil.WriteFile(f.Name(), cfgData, 0644)

		name = f.Name()
	}

	return name, func() {
		os.RemoveAll(tempDir)
	}
}

func setupConfigFileStore(t *testing.T, cfg *model.Config) (*Store, func()) {
	t.Helper()
	path, tearDown := setupConfigFile(t, cfg)
	fs, err := NewFileStore(path)
	require.NoError(t, err)
	configStore, err := NewStoreFromBacking(fs, nil, false)
	require.NoError(t, err)
	return configStore, func() {
		tearDown()
		configStore.Close()
	}
}

// getActualFileConfig returns the configuration present in the given file without relying on a config store.
func getActualFileConfig(t *testing.T, path string) *model.Config {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	var actualCfg *model.Config
	err = json.NewDecoder(f).Decode(&actualCfg)
	require.NoError(t, err)

	return actualCfg
}

// assertFileEqualsConfig verifies the on disk contents of the given path equal the given
func assertFileEqualsConfig(t *testing.T, expectedCfg *model.Config, path string) {
	t.Helper()

	actualCfg := getActualFileConfig(t, path)

	assert.Equal(t, expectedCfg, actualCfg)
}

// assertFileNotEqualsConfig verifies the on disk contents of the given path does not equal the given
func assertFileNotEqualsConfig(t *testing.T, expectedCfg *model.Config, path string) {
	t.Helper()

	actualCfg := getActualFileConfig(t, path)

	assert.NotEqual(t, expectedCfg, actualCfg)
}

func TestFileStoreNew(t *testing.T) {
	utils.TranslationsPreInit()

	t.Run("absolute path, initialization required", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, nil, false)
		require.NoError(t, err)
		defer configStore.Close()

		assert.Equal(t, "http://TestStoreNew", *configStore.Get().ServiceSettings.SiteURL)
		assertFileNotEqualsConfig(t, testConfig, path)
	})

	t.Run("absolute path, initialization required, with custom defaults", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, customConfigDefaults, false)
		require.NoError(t, err)
		defer configStore.Close()

		// already existing value should not be affected by the custom
		// defaults
		assert.Equal(t, "http://TestStoreNew", *configStore.Get().ServiceSettings.SiteURL)
		// nonexisting value should be overwritten by the custom
		// defaults
		assert.Equal(t, *customConfigDefaults.DisplaySettings.ExperimentalTimezone, *configStore.Get().DisplaySettings.ExperimentalTimezone)
		assertFileNotEqualsConfig(t, testConfig, path)
	})

	t.Run("absolute path, already minimally configured", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfigNoFF)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, nil, false)
		require.NoError(t, err)
		defer configStore.Close()

		assert.Equal(t, "http://minimal", *configStore.Get().ServiceSettings.SiteURL)
		assertFileEqualsConfig(t, minimalConfigNoFF, path)
	})

	t.Run("absolute path, already minimally configured, with custom defaults", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfigNoFF)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, customConfigDefaults, false)
		require.NoError(t, err)
		defer configStore.Close()

		// as the whole config has default values already, custom
		// defaults should have no effect
		assert.Equal(t, "http://minimal", *configStore.Get().ServiceSettings.SiteURL)
		assert.NotEqual(t, *customConfigDefaults.DisplaySettings.ExperimentalTimezone, *configStore.Get().DisplaySettings.ExperimentalTimezone)
		assertFileEqualsConfig(t, minimalConfigNoFF, path)
	})

	t.Run("absolute path, file does not exist", func(t *testing.T) {
		_, tearDown := setupConfigFile(t, nil)
		defer tearDown()

		tempDir, err := ioutil.TempDir("", "TestFileStoreNew")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		path := filepath.Join(tempDir, "does_not_exist")
		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, nil, false)
		require.NoError(t, err)
		defer configStore.Close()

		assert.Equal(t, "", *configStore.Get().ServiceSettings.SiteURL)
		assertFileNotEqualsConfig(t, testConfig, path)
	})

	t.Run("absolute path, file does not exist, with custom defaults", func(t *testing.T) {
		_, tearDown := setupConfigFile(t, nil)
		defer tearDown()

		tempDir, err := ioutil.TempDir("", "TestFileStoreNew")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		path := filepath.Join(tempDir, "does_not_exist")
		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, customConfigDefaults, false)
		require.NoError(t, err)
		defer configStore.Close()

		assert.Equal(t, *customConfigDefaults.ServiceSettings.SiteURL, *configStore.Get().ServiceSettings.SiteURL)
		assert.Equal(t, *customConfigDefaults.DisplaySettings.ExperimentalTimezone, *configStore.Get().DisplaySettings.ExperimentalTimezone)
	})

	t.Run("absolute path, path to file does not exist", func(t *testing.T) {
		_, tearDown := setupConfigFile(t, nil)
		defer tearDown()

		tempDir, err := ioutil.TempDir("", "TestFileStoreNew")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		path := filepath.Join(tempDir, "does/not/exist")
		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, nil, false)
		require.Nil(t, configStore)
		require.Error(t, err)
	})

	t.Run("relative path, file exists", func(t *testing.T) {
		_, tearDown := setupConfigFile(t, nil)
		defer tearDown()

		err := os.MkdirAll("TestFileStoreNew/a/b/c", 0700)
		require.NoError(t, err)
		defer os.RemoveAll("TestFileStoreNew")

		path := "TestFileStoreNew/a/b/c/config.json"

		cfgData, err := marshalConfig(testConfig)
		require.NoError(t, err)

		ioutil.WriteFile(path, cfgData, 0644)

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, nil, false)
		require.NoError(t, err)
		defer configStore.Close()

		assert.Equal(t, "http://TestStoreNew", *configStore.Get().ServiceSettings.SiteURL)
		assertFileNotEqualsConfig(t, testConfig, path)
	})

	t.Run("relative path, file does not exist", func(t *testing.T) {
		_, tearDown := setupConfigFile(t, nil)
		defer tearDown()

		err := os.MkdirAll("config/TestFileStoreNew/a/b/c", 0700)
		require.NoError(t, err)
		defer os.RemoveAll("config/TestFileStoreNew")

		path := "TestFileStoreNew/a/b/c/config.json"
		fs, err := NewFileStore(path)
		require.NoError(t, err)
		configStore, err := NewStoreFromBacking(fs, nil, false)
		require.NoError(t, err)
		defer configStore.Close()

		assert.Equal(t, "", *configStore.Get().ServiceSettings.SiteURL)
		assertFileNotEqualsConfig(t, testConfig, filepath.Join("config", path))
	})
}

func TestFileStoreGet(t *testing.T) {
	configStore, tearDown := setupConfigFileStore(t, testConfig)
	defer tearDown()

	cfg := configStore.Get()
	assert.Equal(t, "http://TestStoreNew", *cfg.ServiceSettings.SiteURL)

	cfg2 := configStore.Get()
	assert.Equal(t, "http://TestStoreNew", *cfg.ServiceSettings.SiteURL)

	assert.True(t, cfg == cfg2, "Get() returned different configuration instances")

	newCfg := &model.Config{}
	_, _, err := configStore.Set(newCfg)
	require.NoError(t, err)

	assert.False(t, newCfg == cfg, "returned config should have been different from original")
}

func TestFileStoreGetEnivironmentOverrides(t *testing.T) {
	t.Run("get override for a string variable", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, "http://TestStoreNew", *fs.Get().ServiceSettings.SiteURL)
		assert.Empty(t, fs.GetEnvironmentOverrides())

		os.Setenv("MM_SERVICESETTINGS_SITEURL", "http://override")
		defer os.Unsetenv("MM_SERVICESETTINGS_SITEURL")

		fsInner, err = NewFileStore(path)
		require.NoError(t, err)
		fs, err = NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, "http://override", *fs.Get().ServiceSettings.SiteURL)
		assert.Equal(t, map[string]interface{}{"ServiceSettings": map[string]interface{}{"SiteURL": true}}, fs.GetEnvironmentOverrides())
	})

	t.Run("get override for a string variable, with custom defaults", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, customConfigDefaults, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, "http://TestStoreNew", *fs.Get().ServiceSettings.SiteURL)
		assert.Empty(t, fs.GetEnvironmentOverrides())

		os.Setenv("MM_SERVICESETTINGS_SITEURL", "http://override")
		defer os.Unsetenv("MM_SERVICESETTINGS_SITEURL")

		fsInner, err = NewFileStore(path)
		require.NoError(t, err)
		fs, err = NewStoreFromBacking(fsInner, customConfigDefaults, false)
		require.NoError(t, err)
		defer fs.Close()

		// environment override should take priority over the custom default value
		assert.Equal(t, "http://override", *fs.Get().ServiceSettings.SiteURL)
		assert.Equal(t, map[string]interface{}{"ServiceSettings": map[string]interface{}{"SiteURL": true}}, fs.GetEnvironmentOverrides())
	})

	t.Run("get override for a bool variable", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, false, *fs.Get().PluginSettings.EnableUploads)
		assert.Empty(t, fs.GetEnvironmentOverrides())

		os.Setenv("MM_PLUGINSETTINGS_ENABLEUPLOADS", "true")
		defer os.Unsetenv("MM_PLUGINSETTINGS_ENABLEUPLOADS")

		fsInner, err = NewFileStore(path)
		require.NoError(t, err)
		fs, err = NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, true, *fs.Get().PluginSettings.EnableUploads)
		assert.Equal(t, map[string]interface{}{"PluginSettings": map[string]interface{}{"EnableUploads": true}}, fs.GetEnvironmentOverrides())
	})

	t.Run("get override for an int variable", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, model.TEAM_SETTINGS_DEFAULT_MAX_USERS_PER_TEAM, *fs.Get().TeamSettings.MaxUsersPerTeam)
		assert.Empty(t, fs.GetEnvironmentOverrides())

		os.Setenv("MM_TEAMSETTINGS_MAXUSERSPERTEAM", "3000")
		defer os.Unsetenv("MM_TEAMSETTINGS_MAXUSERSPERTEAM")

		fsInner, err = NewFileStore(path)
		require.NoError(t, err)
		fs, err = NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, 3000, *fs.Get().TeamSettings.MaxUsersPerTeam)
		assert.Equal(t, map[string]interface{}{"TeamSettings": map[string]interface{}{"MaxUsersPerTeam": true}}, fs.GetEnvironmentOverrides())
	})

	t.Run("get override for an int64 variable", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, int64(63072000), *fs.Get().ServiceSettings.TLSStrictTransportMaxAge)
		assert.Empty(t, fs.GetEnvironmentOverrides())

		os.Setenv("MM_SERVICESETTINGS_TLSSTRICTTRANSPORTMAXAGE", "123456")
		defer os.Unsetenv("MM_SERVICESETTINGS_TLSSTRICTTRANSPORTMAXAGE")

		fsInner, err = NewFileStore(path)
		require.NoError(t, err)
		fs, err = NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, int64(123456), *fs.Get().ServiceSettings.TLSStrictTransportMaxAge)
		assert.Equal(t, map[string]interface{}{"ServiceSettings": map[string]interface{}{"TLSStrictTransportMaxAge": true}}, fs.GetEnvironmentOverrides())
	})

	t.Run("get override for a slice variable - one value", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, []string{}, fs.Get().SqlSettings.DataSourceReplicas)
		assert.Empty(t, fs.GetEnvironmentOverrides())

		os.Setenv("MM_SQLSETTINGS_DATASOURCEREPLICAS", "user:pwd@db:5432/test-db")
		defer os.Unsetenv("MM_SQLSETTINGS_DATASOURCEREPLICAS")

		fsInner, err = NewFileStore(path)
		require.NoError(t, err)
		fs, err = NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, []string{"user:pwd@db:5432/test-db"}, fs.Get().SqlSettings.DataSourceReplicas)
		assert.Equal(t, map[string]interface{}{"SqlSettings": map[string]interface{}{"DataSourceReplicas": true}}, fs.GetEnvironmentOverrides())
	})

	t.Run("get override for a slice variable - three values", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, testConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, []string{}, fs.Get().SqlSettings.DataSourceReplicas)
		assert.Empty(t, fs.GetEnvironmentOverrides())

		os.Setenv("MM_SQLSETTINGS_DATASOURCEREPLICAS", "user:pwd@db:5432/test-db user:pwd@db2:5433/test-db2 user:pwd@db3:5434/test-db3")
		defer os.Unsetenv("MM_SQLSETTINGS_DATASOURCEREPLICAS")

		fsInner, err = NewFileStore(path)
		require.NoError(t, err)
		fs, err = NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, []string{"user:pwd@db:5432/test-db", "user:pwd@db2:5433/test-db2", "user:pwd@db3:5434/test-db3"}, fs.Get().SqlSettings.DataSourceReplicas)
		assert.Equal(t, map[string]interface{}{"SqlSettings": map[string]interface{}{"DataSourceReplicas": true}}, fs.GetEnvironmentOverrides())
	})
}

func TestFileStoreSet(t *testing.T) {
	t.Run("defaults required", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, minimalConfig)
		defer tearDown()

		oldCfg := configStore.Get().Clone()
		newCfg := &model.Config{}

		retCfg, newConfig, err := configStore.Set(newCfg)
		require.NoError(t, err)
		require.Equal(t, oldCfg, retCfg)
		require.NotEqual(t, newCfg, newConfig)

		assert.Equal(t, "", *configStore.Get().ServiceSettings.SiteURL)
	})

	t.Run("desanitization required", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, ldapConfig)
		defer tearDown()

		newCfg := &model.Config{}
		newCfg.LdapSettings.BindPassword = model.NewString(model.FAKE_SETTING)

		_, newConfig, err := configStore.Set(newCfg)
		require.NoError(t, err)
		require.NotEqual(t, newCfg, newConfig)

		assert.Equal(t, "password", *configStore.Get().LdapSettings.BindPassword)
	})

	t.Run("invalid", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, emptyConfig)
		defer tearDown()

		newCfg := &model.Config{}
		newCfg.ServiceSettings.SiteURL = model.NewString("invalid")

		_, _, err := configStore.Set(newCfg)
		if assert.Error(t, err) {
			assert.EqualError(t, err, "new configuration is invalid: Config.IsValid: model.config.is_valid.site_url.app_error, ")
		}

		assert.Equal(t, "", *configStore.Get().ServiceSettings.SiteURL)
	})

	t.Run("read-only", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, readOnlyConfig)
		defer tearDown()

		newReadOnlyConfig := readOnlyConfig.Clone()
		newReadOnlyConfig.ServiceSettings = model.ServiceSettings{
			SiteURL: model.NewString("http://test"),
		}
		_, _, err := configStore.Set(newReadOnlyConfig)
		if assert.Error(t, err) {
			assert.Equal(t, ErrReadOnlyConfiguration, errors.Cause(err))
		}

		assert.Equal(t, "", *configStore.Get().ServiceSettings.SiteURL)
	})

	t.Run("persist failed", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, emptyConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		fsInner.path = ""

		newCfg := &model.Config{}

		_, _, err = fs.Set(newCfg)
		if assert.Error(t, err) {
			assert.True(t, strings.HasPrefix(err.Error(), "failed to persist: failed to write file"))
		}

		assert.Equal(t, "", *fs.Get().ServiceSettings.SiteURL)
	})

	t.Run("listeners notified", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, emptyConfig)
		defer tearDown()

		called := make(chan bool, 1)
		callback := func(oldCfg, newCfg *model.Config) {
			require.NotEqual(t, oldCfg, newCfg)
			called <- true
		}
		configStore.AddListener(callback)

		newCfg := minimalConfig

		_, _, err := configStore.Set(newCfg)
		require.NoError(t, err)

		require.True(t, wasCalled(called, 5*time.Second), "callback should have been called when config written")
	})

	t.Run("listeners notified, only env change", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, minimalConfig)
		defer tearDown()

		called := make(chan bool, 1)
		callback := func(oldCfg, newCfg *model.Config) {
			require.NotEqual(t, oldCfg, newCfg)
			expectedConfig := minimalConfig.Clone()
			expectedConfig.ServiceSettings.SiteURL = model.NewString("http://override")
			require.Equal(t, minimalConfig, oldCfg)
			require.Equal(t, expectedConfig, newCfg)
			called <- true
		}
		configStore.AddListener(callback)

		newCfg := minimalConfig
		os.Setenv("MM_SERVICESETTINGS_SITEURL", "http://override")
		defer os.Unsetenv("MM_SERVICESETTINGS_SITEURL")

		_, _, err := configStore.Set(newCfg)
		require.NoError(t, err)

		require.True(t, wasCalled(called, 5*time.Second), "callback should have been called when config changed")
	})

	t.Run("listeners notified, feature flags change only", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, minimalConfig)
		defer tearDown()

		expectedOldConfig := minimalConfig.Clone()
		var expectedNewConfig *model.Config
		called := make(chan bool, 1)
		callback := func(oldCfg, newCfg *model.Config) {
			require.NotEqual(t, oldCfg, newCfg)
			require.Equal(t, expectedOldConfig, oldCfg)
			require.Equal(t, expectedNewConfig, newCfg)
			called <- true
		}
		configStore.AddListener(callback)

		configStore.SetReadOnlyFF(true)

		expectedNewConfig = minimalConfig.Clone()
		expectedNewConfig.FeatureFlags.TestFeature = "test"
		_, _, err := configStore.Set(expectedNewConfig)
		require.NoError(t, err)

		require.False(t, wasCalled(called, 5*time.Second))

		configStore.SetReadOnlyFF(false)

		expectedNewConfig.FeatureFlags.TestFeature = "test2"
		_, _, err = configStore.Set(expectedNewConfig)
		require.NoError(t, err)

		require.True(t, wasCalled(called, 5*time.Second))
	})
}

func TestFileStoreLoad(t *testing.T) {
	t.Run("file no longer exists", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, emptyConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		os.Remove(path)

		err = fs.Load()
		require.NoError(t, err)
		assertFileNotEqualsConfig(t, emptyConfig, path)
	})

	t.Run("honour environment", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, minimalConfig)
		defer tearDown()

		assert.Equal(t, "http://minimal", *configStore.Get().ServiceSettings.SiteURL)

		os.Setenv("MM_SERVICESETTINGS_SITEURL", "http://override")
		defer os.Unsetenv("MM_SERVICESETTINGS_SITEURL")

		err := configStore.Load()
		require.NoError(t, err)
		assert.Equal(t, "http://override", *configStore.Get().ServiceSettings.SiteURL)
		assert.Equal(t, map[string]interface{}{"ServiceSettings": map[string]interface{}{"SiteURL": true}}, configStore.GetEnvironmentOverrides())
	})

	t.Run("do not persist environment variables - string", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		os.Setenv("MM_SERVICESETTINGS_SITEURL", "http://overridePersistEnvVariables")
		defer os.Unsetenv("MM_SERVICESETTINGS_SITEURL")

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, "http://overridePersistEnvVariables", *fs.Get().ServiceSettings.SiteURL)

		_, _, err = fs.Set(fs.Get())
		require.NoError(t, err)

		assert.Equal(t, "http://overridePersistEnvVariables", *fs.Get().ServiceSettings.SiteURL)
		assert.Equal(t, map[string]interface{}{"ServiceSettings": map[string]interface{}{"SiteURL": true}}, fs.GetEnvironmentOverrides())
		// check that on disk config does not include overwritten variable
		actualConfig := getActualFileConfig(t, path)
		assert.Equal(t, "http://minimal", *actualConfig.ServiceSettings.SiteURL)
	})

	t.Run("do not persist environment variables - boolean", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		os.Setenv("MM_PLUGINSETTINGS_ENABLEUPLOADS", "true")
		defer os.Unsetenv("MM_PLUGINSETTINGS_ENABLEUPLOADS")

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, true, *fs.Get().PluginSettings.EnableUploads)

		_, _, err = fs.Set(fs.Get())
		require.NoError(t, err)

		assert.Equal(t, true, *fs.Get().PluginSettings.EnableUploads)
		assert.Equal(t, map[string]interface{}{"PluginSettings": map[string]interface{}{"EnableUploads": true}}, fs.GetEnvironmentOverrides())
		// check that on disk config does not include overwritten variable
		actualConfig := getActualFileConfig(t, path)
		assert.Equal(t, false, *actualConfig.PluginSettings.EnableUploads)
	})

	t.Run("do not persist environment variables - int", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		os.Setenv("MM_TEAMSETTINGS_MAXUSERSPERTEAM", "3000")
		defer os.Unsetenv("MM_TEAMSETTINGS_MAXUSERSPERTEAM")

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, 3000, *fs.Get().TeamSettings.MaxUsersPerTeam)

		_, _, err = fs.Set(fs.Get())
		require.NoError(t, err)

		assert.Equal(t, 3000, *fs.Get().TeamSettings.MaxUsersPerTeam)
		assert.Equal(t, map[string]interface{}{"TeamSettings": map[string]interface{}{"MaxUsersPerTeam": true}}, fs.GetEnvironmentOverrides())
		// check that on disk config does not include overwritten variable
		actualConfig := getActualFileConfig(t, path)
		assert.Equal(t, model.TEAM_SETTINGS_DEFAULT_MAX_USERS_PER_TEAM, *actualConfig.TeamSettings.MaxUsersPerTeam)
	})

	t.Run("do not persist environment variables - int64", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		os.Setenv("MM_SERVICESETTINGS_TLSSTRICTTRANSPORTMAXAGE", "123456")
		defer os.Unsetenv("MM_SERVICESETTINGS_TLSSTRICTTRANSPORTMAXAGE")

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, int64(123456), *fs.Get().ServiceSettings.TLSStrictTransportMaxAge)

		_, _, err = fs.Set(fs.Get())
		require.NoError(t, err)

		assert.Equal(t, int64(123456), *fs.Get().ServiceSettings.TLSStrictTransportMaxAge)
		assert.Equal(t, map[string]interface{}{"ServiceSettings": map[string]interface{}{"TLSStrictTransportMaxAge": true}}, fs.GetEnvironmentOverrides())
		// check that on disk config does not include overwritten variable
		actualConfig := getActualFileConfig(t, path)
		assert.Equal(t, int64(63072000), *actualConfig.ServiceSettings.TLSStrictTransportMaxAge)
	})

	t.Run("do not persist environment variables - string slice beginning with default", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		os.Setenv("MM_SQLSETTINGS_DATASOURCEREPLICAS", "user:pwd@db:5432/test-db")
		defer os.Unsetenv("MM_SQLSETTINGS_DATASOURCEREPLICAS")

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, []string{"user:pwd@db:5432/test-db"}, fs.Get().SqlSettings.DataSourceReplicas)

		_, _, err = fs.Set(fs.Get())
		require.NoError(t, err)

		assert.Equal(t, []string{"user:pwd@db:5432/test-db"}, fs.Get().SqlSettings.DataSourceReplicas)
		assert.Equal(t, map[string]interface{}{"SqlSettings": map[string]interface{}{"DataSourceReplicas": true}}, fs.GetEnvironmentOverrides())
		// check that on disk config does not include overwritten variable
		actualConfig := getActualFileConfig(t, path)
		assert.Equal(t, []string{}, actualConfig.SqlSettings.DataSourceReplicas)
	})

	t.Run("do not persist environment variables - string slice beginning with slice of three", func(t *testing.T) {
		modifiedMinimalConfig := minimalConfig.Clone()
		modifiedMinimalConfig.SqlSettings.DataSourceReplicas = []string{"user:pwd@db:5432/test-db", "user:pwd@db2:5433/test-db2", "user:pwd@db3:5434/test-db3"}
		path, tearDown := setupConfigFile(t, modifiedMinimalConfig)
		defer tearDown()

		os.Setenv("MM_SQLSETTINGS_DATASOURCEREPLICAS", "user:pwd@db:5432/test-db")
		defer os.Unsetenv("MM_SQLSETTINGS_DATASOURCEREPLICAS")

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		assert.Equal(t, []string{"user:pwd@db:5432/test-db"}, fs.Get().SqlSettings.DataSourceReplicas)

		_, _, err = fs.Set(fs.Get())
		require.NoError(t, err)

		assert.Equal(t, []string{"user:pwd@db:5432/test-db"}, fs.Get().SqlSettings.DataSourceReplicas)
		assert.Equal(t, map[string]interface{}{"SqlSettings": map[string]interface{}{"DataSourceReplicas": true}}, fs.GetEnvironmentOverrides())
		// check that on disk config does not include overwritten variable
		actualConfig := getActualFileConfig(t, path)
		assert.Equal(t, []string{"user:pwd@db:5432/test-db", "user:pwd@db2:5433/test-db2", "user:pwd@db3:5434/test-db3"}, actualConfig.SqlSettings.DataSourceReplicas)
	})

	t.Run("invalid", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, emptyConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		cfgData, err := marshalConfig(invalidConfig)
		require.NoError(t, err)

		ioutil.WriteFile(path, cfgData, 0644)

		err = fs.Load()
		if assert.Error(t, err) {
			assert.EqualError(t, err, "invalid config: Config.IsValid: model.config.is_valid.site_url.app_error, ")
		}
	})

	t.Run("invalid environment value", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, emptyConfig)
		defer tearDown()

		os.Setenv("MM_SERVICESETTINGS_SITEURL", "invalid_url")
		defer os.Unsetenv("MM_SERVICESETTINGS_SITEURL")

		newCfg := minimalConfig
		_, _, err := configStore.Set(newCfg)
		require.Error(t, err)
		require.EqualError(t, err, "new configuration is invalid: Config.IsValid: model.config.is_valid.site_url.app_error, ")
	})

	t.Run("fixes required", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, fixesRequiredConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		err = fs.Load()
		require.NoError(t, err)
		assertFileNotEqualsConfig(t, fixesRequiredConfig, path)
		assert.Equal(t, "http://trailingslash", *fs.Get().ServiceSettings.SiteURL)
		assert.Equal(t, "/path/to/directory/", *fs.Get().FileSettings.Directory)
		assert.Equal(t, "en", *fs.Get().LocalizationSettings.DefaultServerLocale)
		assert.Equal(t, "en", *fs.Get().LocalizationSettings.DefaultClientLocale)
	})

	t.Run("listeners notifed", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, emptyConfig)
		defer tearDown()

		fsInner, err := NewFileStore(path)
		require.NoError(t, err)
		fs, err := NewStoreFromBacking(fsInner, nil, false)
		require.NoError(t, err)
		defer fs.Close()

		called := make(chan bool, 1)
		callback := func(oldCfg, newCfg *model.Config) {
			called <- true
		}
		fs.AddListener(callback)

		cfgData, err := marshalConfig(minimalConfig)
		require.NoError(t, err)

		err = ioutil.WriteFile(path, cfgData, 0644)
		require.NoError(t, err)

		err = fs.Load()
		require.NoError(t, err)

		require.True(t, wasCalled(called, 5*time.Second), "callback should have been called when config changed on load")
	})

	t.Run("no change", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, testConfig)
		defer tearDown()

		called := make(chan bool, 1)
		callback := func(oldCfg, newCfg *model.Config) {
			called <- true
		}
		configStore.AddListener(callback)

		err := configStore.Load()
		require.NoError(t, err)

		require.False(t, wasCalled(called, 5*time.Second), "callback should not have been called if nothing changed")
	})

	t.Run("listeners notified, only env change", func(t *testing.T) {
		configStore, tearDown := setupConfigFileStore(t, minimalConfig)
		defer tearDown()

		time.Sleep(1 * time.Second)

		called := make(chan bool, 1)
		callback := func(oldCfg, newCfg *model.Config) {
			require.NotEqual(t, oldCfg, newCfg)
			expectedConfig := minimalConfig.Clone()
			expectedConfig.ServiceSettings.SiteURL = model.NewString("http://override")
			require.Equal(t, minimalConfig, oldCfg)
			require.Equal(t, expectedConfig, newCfg)
			called <- true
		}
		configStore.AddListener(callback)

		os.Setenv("MM_SERVICESETTINGS_SITEURL", "http://override")
		defer os.Unsetenv("MM_SERVICESETTINGS_SITEURL")

		err := configStore.Load()
		require.NoError(t, err)

		require.True(t, wasCalled(called, 5*time.Second), "callback should have been called when config changed")
	})
}

func TestFileStoreSave(t *testing.T) {
	store, tearDown := setupConfigFileStore(t, minimalConfig)
	defer tearDown()

	newCfg := &model.Config{
		ServiceSettings: model.ServiceSettings{
			SiteURL: model.NewString("http://new"),
		},
	}

	t.Run("set with automatic save", func(t *testing.T) {
		_, _, err := store.Set(newCfg)
		require.NoError(t, err)

		err = store.Load()
		require.NoError(t, err)

		assert.Equal(t, "http://new", *store.Get().ServiceSettings.SiteURL)
	})
}

func TestFileGetFile(t *testing.T) {
	path, tearDown := setupConfigFile(t, minimalConfig)
	defer tearDown()

	fs, err := NewFileStore(path)
	require.NoError(t, err)
	defer fs.Close()

	t.Run("get empty filename", func(t *testing.T) {
		_, err := fs.GetFile("")
		require.Error(t, err)
	})

	t.Run("get non-existent file", func(t *testing.T) {
		_, err := fs.GetFile("unknown")
		require.Error(t, err)
	})

	t.Run("get empty file", func(t *testing.T) {
		err := os.MkdirAll("config", 0700)
		require.NoError(t, err)

		f, err := ioutil.TempFile("config", "empty-file")
		require.NoError(t, err)
		defer os.Remove(f.Name())

		err = ioutil.WriteFile(f.Name(), nil, 0777)
		require.NoError(t, err)

		data, err := fs.GetFile(f.Name())
		require.NoError(t, err)
		require.Empty(t, data)
	})

	t.Run("get non-empty file", func(t *testing.T) {
		err := os.MkdirAll("config", 0700)
		require.NoError(t, err)

		f, err := ioutil.TempFile("config", "test-file")
		require.NoError(t, err)
		defer os.Remove(f.Name())

		err = ioutil.WriteFile(f.Name(), []byte("test"), 0777)
		require.NoError(t, err)

		data, err := fs.GetFile(f.Name())
		require.NoError(t, err)
		require.Equal(t, []byte("test"), data)
	})

	t.Run("get via absolute path", func(t *testing.T) {
		err := fs.SetFile("new", []byte("new file"))
		require.NoError(t, err)

		data, err := fs.GetFile(filepath.Join(filepath.Dir(path), "new"))

		require.NoError(t, err)
		require.Equal(t, []byte("new file"), data)
	})

}

func TestFileSetFile(t *testing.T) {
	path, tearDown := setupConfigFile(t, minimalConfig)
	defer tearDown()

	fs, err := NewFileStore(path)
	require.NoError(t, err)
	defer fs.Close()

	t.Run("set new file", func(t *testing.T) {
		err := fs.SetFile("new", []byte("new file"))
		require.NoError(t, err)

		data, err := fs.GetFile("new")
		require.NoError(t, err)
		require.Equal(t, []byte("new file"), data)
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		err := fs.SetFile("existing", []byte("existing file"))
		require.NoError(t, err)

		err = fs.SetFile("existing", []byte("overwritten file"))
		require.NoError(t, err)

		data, err := fs.GetFile("existing")
		require.NoError(t, err)
		require.Equal(t, []byte("overwritten file"), data)
	})

	t.Run("set via absolute path", func(t *testing.T) {
		absolutePath := filepath.Join(filepath.Dir(path), "new")
		err := fs.SetFile(absolutePath, []byte("new file"))
		require.NoError(t, err)

		data, err := fs.GetFile("new")

		require.NoError(t, err)
		require.Equal(t, []byte("new file"), data)
	})

	t.Run("should set right permissions", func(t *testing.T) {
		absolutePath := filepath.Join(filepath.Dir(path), "new")
		err := fs.SetFile(absolutePath, []byte("data"))
		require.NoError(t, err)
		fi, err := os.Stat(absolutePath)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), fi.Mode().Perm())
	})
}

func TestFileHasFile(t *testing.T) {
	t.Run("has non-existent", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		has, err := fs.HasFile("non-existent")
		require.NoError(t, err)
		require.False(t, has)
	})

	t.Run("has existing", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		err = fs.SetFile("existing", []byte("existing file"))
		require.NoError(t, err)

		has, err := fs.HasFile("existing")
		require.NoError(t, err)
		require.True(t, has)
	})

	t.Run("has manually created file", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		err = os.MkdirAll("config", 0700)
		require.NoError(t, err)

		f, err := ioutil.TempFile("config", "test-file")
		require.NoError(t, err)
		defer os.Remove(f.Name())

		err = ioutil.WriteFile(f.Name(), []byte("test"), 0777)
		require.NoError(t, err)

		has, err := fs.HasFile(f.Name())
		require.NoError(t, err)
		require.True(t, has)
	})

	t.Run("has empty string", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		has, err := fs.HasFile("")
		require.NoError(t, err)
		require.False(t, has)
	})

	t.Run("has via absolute path", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		err = fs.SetFile("existing", []byte("existing file"))
		require.NoError(t, err)

		has, err := fs.HasFile(filepath.Join(filepath.Dir(path), "existing"))
		require.NoError(t, err)
		require.True(t, has)
	})

}

func TestFileRemoveFile(t *testing.T) {
	t.Run("remove non-existent", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		err = fs.RemoveFile("non-existent")
		require.NoError(t, err)
	})

	t.Run("remove existing", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		err = fs.SetFile("existing", []byte("existing file"))
		require.NoError(t, err)

		err = fs.RemoveFile("existing")
		require.NoError(t, err)

		has, err := fs.HasFile("existing")
		require.NoError(t, err)
		require.False(t, has)

		_, err = fs.GetFile("existing")
		require.Error(t, err)
	})

	t.Run("remove manually created file", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		err = os.MkdirAll("config", 0700)
		require.NoError(t, err)

		f, err := ioutil.TempFile("config", "test-file")
		require.NoError(t, err)
		defer os.Remove(f.Name())

		err = ioutil.WriteFile(f.Name(), []byte("test"), 0777)
		require.NoError(t, err)

		err = fs.RemoveFile(f.Name())
		require.NoError(t, err)

		has, err := fs.HasFile("existing")
		require.NoError(t, err)
		require.False(t, has)

		_, err = fs.GetFile("existing")
		require.Error(t, err)
	})

	t.Run("don't remove via absolute path", func(t *testing.T) {
		path, tearDown := setupConfigFile(t, minimalConfig)
		defer tearDown()

		fs, err := NewFileStore(path)
		require.NoError(t, err)
		defer fs.Close()

		err = fs.SetFile("existing", []byte("existing file"))
		require.NoError(t, err)

		filename := filepath.Join(filepath.Dir(path), "existing")
		err = fs.RemoveFile(filename)
		require.NoError(t, err)

		has, err := fs.HasFile(filename)
		require.NoError(t, err)
		require.True(t, has)

	})
}

func TestFileStoreString(t *testing.T) {
	path, tearDown := setupConfigFile(t, emptyConfig)
	defer tearDown()

	fs, err := NewFileStore(path)
	require.NoError(t, err)
	defer fs.Close()

	assert.Equal(t, "file://"+path, fs.String())
}

// wasCalled reports whether a given callback channel was called
// within the specified time duration or not.
func wasCalled(c chan bool, duration time.Duration) bool {
	select {
	case <-c:
		return true
	case <-time.After(duration):
	}
	return false
}

func TestFileStoreReadOnly(t *testing.T) {
	path, tearDown := setupConfigFile(t, emptyConfig)
	defer tearDown()
	fsInner, err := NewFileStore(path)
	require.NoError(t, err)
	fs, err := NewStoreFromBacking(fsInner, nil, true)
	require.NoError(t, err)
	defer fs.Close()

	called := make(chan bool, 1)
	callback := func(oldCfg, newCfg *model.Config) {
		called <- true
	}
	fs.AddListener(callback)

	cfg, _, err := fs.Set(minimalConfig)
	require.Nil(t, cfg)
	require.Equal(t, ErrReadOnlyStore, err)

	require.False(t, wasCalled(called, 1*time.Second), "callback should not have been called since config is read-only")
}

func TestFileStoreSetReadOnlyFF(t *testing.T) {
	t.Run("read only true", func(t *testing.T) {
		store, tearDown := setupConfigFileStore(t, minimalConfig)
		defer tearDown()
		config := store.Get()
		require.Equal(t, minimalConfig.FeatureFlags, config.FeatureFlags)

		newCfg := config.Clone()
		newCfg.FeatureFlags.TestFeature = "test"

		// store has read-only FF by default.
		_, _, err := store.Set(newCfg)
		require.NoError(t, err)

		config = store.Get()
		require.Equal(t, minimalConfig.FeatureFlags, config.FeatureFlags)
	})

	t.Run("read only false", func(t *testing.T) {
		store, tearDown := setupConfigFileStore(t, minimalConfig)
		defer tearDown()
		config := store.Get()
		require.Equal(t, minimalConfig.FeatureFlags, config.FeatureFlags)

		newCfg := config.Clone()
		newCfg.FeatureFlags.TestFeature = "test"

		store.SetReadOnlyFF(false)

		_, _, err := store.Set(newCfg)
		require.NoError(t, err)

		config = store.Get()
		require.Equal(t, newCfg.FeatureFlags, config.FeatureFlags)
	})
}
