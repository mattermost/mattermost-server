// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func TestDesanitize(t *testing.T) {
	actual := &model.Config{}
	actual.SetDefaults()

	// These setting should be ignored
	actual.LdapSettings.Enable = model.NewBool(false)
	actual.FileSettings.DriverName = model.NewString("s3")

	// These settings should be desanitized into target.
	actual.LdapSettings.BindPassword = model.NewString("bind_password")
	actual.FileSettings.PublicLinkSalt = model.NewString("public_link_salt")
	actual.FileSettings.AmazonS3SecretAccessKey = model.NewString("amazon_s3_secret_access_key")
	actual.EmailSettings.SMTPPassword = model.NewString("smtp_password")
	actual.GitLabSettings.Secret = model.NewString("secret")
	actual.OpenIDSettings.Secret = model.NewString("secret")
	actual.SQLSettings.DataSource = model.NewString("data_source")
	actual.SQLSettings.AtRestEncryptKey = model.NewString("at_rest_encrypt_key")
	actual.ElasticsearchSettings.Password = model.NewString("password")
	actual.SQLSettings.DataSourceReplicas = append(actual.SQLSettings.DataSourceReplicas, "replica0")
	actual.SQLSettings.DataSourceReplicas = append(actual.SQLSettings.DataSourceReplicas, "replica1")
	actual.SQLSettings.DataSourceSearchReplicas = append(actual.SQLSettings.DataSourceSearchReplicas, "search_replica0")
	actual.SQLSettings.DataSourceSearchReplicas = append(actual.SQLSettings.DataSourceSearchReplicas, "search_replica1")

	target := &model.Config{}
	target.SetDefaults()

	// These setting should be ignored
	target.LdapSettings.Enable = model.NewBool(true)
	target.FileSettings.DriverName = model.NewString("file")

	// These settings should be updated from actual
	target.LdapSettings.BindPassword = model.NewString(model.FakeSetting)
	target.FileSettings.PublicLinkSalt = model.NewString(model.FakeSetting)
	target.FileSettings.AmazonS3SecretAccessKey = model.NewString(model.FakeSetting)
	target.EmailSettings.SMTPPassword = model.NewString(model.FakeSetting)
	target.GitLabSettings.Secret = model.NewString(model.FakeSetting)
	target.OpenIDSettings.Secret = model.NewString(model.FakeSetting)
	target.SQLSettings.DataSource = model.NewString(model.FakeSetting)
	target.SQLSettings.AtRestEncryptKey = model.NewString(model.FakeSetting)
	target.ElasticsearchSettings.Password = model.NewString(model.FakeSetting)
	target.SQLSettings.DataSourceReplicas = []string{model.FakeSetting, model.FakeSetting}
	target.SQLSettings.DataSourceSearchReplicas = []string{model.FakeSetting, model.FakeSetting}

	actualClone := actual.Clone()
	desanitize(actual, target)
	assert.Equal(t, actualClone, actual, "actual should not have been changed")

	// Verify the settings that should have been left untouched in target
	assert.True(t, *target.LdapSettings.Enable, "LdapSettings.Enable should not have changed")
	assert.Equal(t, "file", *target.FileSettings.DriverName, "FileSettings.DriverName should not have been changed")

	// Verify the settings that should have been desanitized into target
	assert.Equal(t, *actual.LdapSettings.BindPassword, *target.LdapSettings.BindPassword)
	assert.Equal(t, *actual.FileSettings.PublicLinkSalt, *target.FileSettings.PublicLinkSalt)
	assert.Equal(t, *actual.FileSettings.AmazonS3SecretAccessKey, *target.FileSettings.AmazonS3SecretAccessKey)
	assert.Equal(t, *actual.EmailSettings.SMTPPassword, *target.EmailSettings.SMTPPassword)
	assert.Equal(t, *actual.GitLabSettings.Secret, *target.GitLabSettings.Secret)
	assert.Equal(t, *actual.OpenIDSettings.Secret, *target.OpenIDSettings.Secret)
	assert.Equal(t, *actual.SQLSettings.DataSource, *target.SQLSettings.DataSource)
	assert.Equal(t, *actual.SQLSettings.AtRestEncryptKey, *target.SQLSettings.AtRestEncryptKey)
	assert.Equal(t, *actual.ElasticsearchSettings.Password, *target.ElasticsearchSettings.Password)
	assert.Equal(t, actual.SQLSettings.DataSourceReplicas, target.SQLSettings.DataSourceReplicas)
	assert.Equal(t, actual.SQLSettings.DataSourceSearchReplicas, target.SQLSettings.DataSourceSearchReplicas)
	assert.Equal(t, actual.ServiceSettings.SplitKey, target.ServiceSettings.SplitKey)
}

func TestFixInvalidLocales(t *testing.T) {
	utils.TranslationsPreInit()

	cfg := &model.Config{}
	cfg.SetDefaults()

	*cfg.LocalizationSettings.DefaultServerLocale = "en"
	*cfg.LocalizationSettings.DefaultClientLocale = "en"
	*cfg.LocalizationSettings.AvailableLocales = ""

	changed := FixInvalidLocales(cfg)
	assert.False(t, changed)

	*cfg.LocalizationSettings.DefaultServerLocale = "junk"
	changed = FixInvalidLocales(cfg)
	assert.True(t, changed)
	assert.Equal(t, "en", *cfg.LocalizationSettings.DefaultServerLocale)

	*cfg.LocalizationSettings.DefaultServerLocale = ""
	changed = FixInvalidLocales(cfg)
	assert.True(t, changed)
	assert.Equal(t, "en", *cfg.LocalizationSettings.DefaultServerLocale)

	*cfg.LocalizationSettings.AvailableLocales = "en"
	*cfg.LocalizationSettings.DefaultServerLocale = "de"
	changed = FixInvalidLocales(cfg)
	assert.False(t, changed)
	assert.NotContains(t, *cfg.LocalizationSettings.AvailableLocales, *cfg.LocalizationSettings.DefaultServerLocale, "DefaultServerLocale should not be added to AvailableLocales")

	*cfg.LocalizationSettings.AvailableLocales = ""
	*cfg.LocalizationSettings.DefaultClientLocale = "junk"
	changed = FixInvalidLocales(cfg)
	assert.True(t, changed)
	assert.Equal(t, "en", *cfg.LocalizationSettings.DefaultClientLocale)

	*cfg.LocalizationSettings.DefaultClientLocale = ""
	changed = FixInvalidLocales(cfg)
	assert.True(t, changed)
	assert.Equal(t, "en", *cfg.LocalizationSettings.DefaultClientLocale)

	*cfg.LocalizationSettings.AvailableLocales = "en"
	*cfg.LocalizationSettings.DefaultClientLocale = "de"
	changed = FixInvalidLocales(cfg)
	assert.True(t, changed)
	assert.Contains(t, *cfg.LocalizationSettings.AvailableLocales, *cfg.LocalizationSettings.DefaultServerLocale, "DefaultClientLocale should have been added to AvailableLocales")

	// validate AvailableLocales
	*cfg.LocalizationSettings.DefaultServerLocale = "en"
	*cfg.LocalizationSettings.DefaultClientLocale = "en"
	*cfg.LocalizationSettings.AvailableLocales = "junk"
	changed = FixInvalidLocales(cfg)
	assert.True(t, changed)
	assert.Equal(t, "", *cfg.LocalizationSettings.AvailableLocales)

	*cfg.LocalizationSettings.AvailableLocales = "en,de,junk"
	changed = FixInvalidLocales(cfg)
	assert.True(t, changed)
	assert.Equal(t, "", *cfg.LocalizationSettings.AvailableLocales)

	*cfg.LocalizationSettings.DefaultServerLocale = "fr"
	*cfg.LocalizationSettings.DefaultClientLocale = "de"
	*cfg.LocalizationSettings.AvailableLocales = "en"
	changed = FixInvalidLocales(cfg)
	assert.True(t, changed)
	assert.NotContains(t, *cfg.LocalizationSettings.AvailableLocales, *cfg.LocalizationSettings.DefaultServerLocale, "DefaultServerLocale should not be added to AvailableLocales")
	assert.Contains(t, *cfg.LocalizationSettings.AvailableLocales, *cfg.LocalizationSettings.DefaultClientLocale, "DefaultClientLocale should have been added to AvailableLocales")
}

func TestIsDatabaseDSN(t *testing.T) {
	testCases := []struct {
		Name     string
		DSN      string
		Expected bool
	}{
		{
			Name:     "Mysql DSN",
			DSN:      "mysql://localhost",
			Expected: true,
		},
		{
			Name:     "Postgresql DSN",
			DSN:      "postgres://localhost",
			Expected: true,
		},
		{
			Name:     "Empty DSN",
			DSN:      "",
			Expected: false,
		},
		{
			Name:     "Default file DSN",
			DSN:      "config.json",
			Expected: false,
		},
		{
			Name:     "Relative path DSN",
			DSN:      "configuration/config.json",
			Expected: false,
		},
		{
			Name:     "Absolute path DSN",
			DSN:      "/opt/mattermost/configuration/config.json",
			Expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			assert.Equal(t, tc.Expected, IsDatabaseDSN(tc.DSN))
		})
	}
}

func TestStripPassword(t *testing.T) {
	for name, test := range map[string]struct {
		DSN         string
		Schema      string
		ExpectedOut string
	}{
		"mysql": {
			DSN:         "mysql://mmuser:password@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s",
			Schema:      "mysql",
			ExpectedOut: "mysql://mmuser:@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s",
		},
		"mysql idempotent": {
			DSN:         "mysql://mmuser:@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s",
			Schema:      "mysql",
			ExpectedOut: "mysql://mmuser:@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s",
		},
		"mysql: password with : and @": {
			DSN:         "mysql://mmuser:p:assw@ord@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s",
			Schema:      "mysql",
			ExpectedOut: "mysql://mmuser:@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s",
		},
		"mysql: password with @ and :": {
			DSN:         "mysql://mmuser:pa@sswo:rd@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s",
			Schema:      "mysql",
			ExpectedOut: "mysql://mmuser:@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s",
		},
		"postgres": {
			DSN:         "postgres://mmuser:password@localhost:5432/mattermost?sslmode=disable&connect_timeout=10",
			Schema:      "postgres",
			ExpectedOut: "postgres://mmuser:@localhost:5432/mattermost?sslmode=disable&connect_timeout=10",
		},
		"pipe": {
			DSN:         "mysql://user@unix(/path/to/socket)/dbname",
			Schema:      "mysql",
			ExpectedOut: "mysql://user@unix(/path/to/socket)/dbname",
		},
		"malformed without :": {
			DSN:         "postgres://mmuserpassword@localhost:5432/mattermost?sslmode=disable&connect_timeout=10",
			Schema:      "postgres",
			ExpectedOut: "postgres://mmuserpassword@localhost:5432/mattermost?sslmode=disable&connect_timeout=10",
		},
		"malformed without @": {
			DSN:         "postgres://mmuser:passwordlocalhost:5432/mattermost?sslmode=disable&connect_timeout=10",
			Schema:      "postgres",
			ExpectedOut: "(omitted due to error parsing the DSN)",
		},
	} {
		t.Run(name, func(t *testing.T) {
			out := stripPassword(test.DSN, test.Schema)

			assert.Equal(t, test.ExpectedOut, out)
		})
	}
}

func TestIsJSONMap(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "good json", data: `{"local_tcp": {
			"Type": "tcp","Format": "json","Levels": [
				{"ID": 5,"Name": "debug","Stacktrace": false}
			],
			"Options": {"ip": "localhost","port": 18065},
			"MaxQueueSize": 1000}}
			`, want: true,
		},
		{name: "empty json", data: "{}", want: true},
		{name: "string json", data: `"test"`, want: false},
		{name: "array json", data: `["test1", "test2"]`, want: false},
		{name: "bad json", data: `{huh?}`, want: false},
		{name: "filename", data: "/tmp/logger.conf", want: false},
		{name: "mysql dsn", data: "mysql://mmuser:@tcp(localhost:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s", want: false},
		{name: "postgres dsn", data: "postgres://mmuser:passwordlocalhost:5432/mattermost?sslmode=disable&connect_timeout=10", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isJSONMap(tt.data); got != tt.want {
				t.Errorf("isJSONMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		diff, err := equal(nil, nil)
		require.NoError(t, err)
		require.False(t, diff)
	})

	t.Run("no diff", func(t *testing.T) {
		old := minimalConfig.Clone()
		new := minimalConfig.Clone()
		diff, err := equal(old, new)
		require.NoError(t, err)
		require.False(t, diff)
	})

	t.Run("diff", func(t *testing.T) {
		old := minimalConfig.Clone()
		new := minimalConfig.Clone()
		new.SQLSettings = model.SQLSettings{}
		diff, err := equal(old, new)
		require.NoError(t, err)
		require.True(t, diff)
	})
}
