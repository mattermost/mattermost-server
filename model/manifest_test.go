// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestIsValid(t *testing.T) {
	testCases := []struct {
		Title       string
		manifest    *Manifest
		ExpectError bool
	}{
		{"Invalid Id", &Manifest{Id: "some id", Name: "some name"}, true},
		{"Invalid Name", &Manifest{Id: "com.company.test", Name: "  "}, true},
		{"Invalid homePageURL", &Manifest{Id: "com.company.test", Name: "some name", HomepageURL: "some url"}, true},
		{"Invalid supportURL", &Manifest{Id: "com.company.test", Name: "some name", SupportURL: "some url"}, true},
		{"Invalid ReleaseNotesURL", &Manifest{Id: "com.company.test", Name: "some name", ReleaseNotesURL: "some url"}, true},
		{"Invalid version", &Manifest{Id: "com.company.test", Name: "some name", HomepageURL: "http://someurl.com", SupportURL: "http://someotherurl.com", Version: "version"}, true},
		{"Invalid min version", &Manifest{Id: "com.company.test", Name: "some name", HomepageURL: "http://someurl.com", SupportURL: "http://someotherurl.com", Version: "5.10.0", MinServerVersion: "version"}, true},
		{"SettingSchema error", &Manifest{Id: "com.company.test", Name: "some name", HomepageURL: "http://someurl.com", SupportURL: "http://someotherurl.com", Version: "5.10.0", MinServerVersion: "5.10.8", SettingsSchema: &PluginSettingsSchema{
			Settings: []*PluginSetting{{Type: "Invalid"}},
		}}, true},
		{"Minimal valid manifest", &Manifest{Id: "com.company.test", Name: "some name"}, false},
		{"Happy case", &Manifest{
			Id:               "com.company.test",
			Name:             "thename",
			Description:      "thedescription",
			HomepageURL:      "http://someurl.com",
			SupportURL:       "http://someotherurl.com",
			ReleaseNotesURL:  "http://someotherurl.com/releases/v0.0.1",
			Version:          "0.0.1",
			MinServerVersion: "5.6.0",
			Server: &ManifestServer{
				Executable: "theexecutable",
			},
			Webapp: &ManifestWebapp{
				BundlePath: "thebundlepath",
			},
			SettingsSchema: &PluginSettingsSchema{
				Header: "theheadertext",
				Footer: "thefootertext",
				Settings: []*PluginSetting{
					{
						Key:         "thesetting",
						DisplayName: "thedisplayname",
						Type:        "dropdown",
						HelpText:    "thehelptext",
						Options: []*PluginOption{
							{
								DisplayName: "theoptiondisplayname",
								Value:       "thevalue",
							},
						},
						Default: "thedefault",
					},
				},
			},
		}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.Title, func(t *testing.T) {
			err := tc.manifest.IsValid()
			if tc.ExpectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsValidSettingsSchema(t *testing.T) {
	testCases := []struct {
		Title          string
		settingsSchema *PluginSettingsSchema
		ExpectError    bool
	}{
		{"Invalid Setting", &PluginSettingsSchema{Settings: []*PluginSetting{{Type: "invalid"}}}, true},
		{"Happy case", &PluginSettingsSchema{Settings: []*PluginSetting{{Type: "text"}}}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.Title, func(t *testing.T) {
			err := tc.settingsSchema.isValid()
			if tc.ExpectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSettingIsValid(t *testing.T) {
	for name, test := range map[string]struct {
		Setting     PluginSetting
		ExpectError bool
	}{
		"Invalid setting type": {
			PluginSetting{Type: "invalid"},
			true,
		},
		"RegenerateHelpText error": {
			PluginSetting{Type: "text", RegenerateHelpText: "some text"},
			true,
		},
		"Placeholder error": {
			PluginSetting{Type: "bool", Placeholder: "some text"},
			true,
		},
		"Nil Options": {
			PluginSetting{Type: "bool"},
			false,
		},
		"Options error": {
			PluginSetting{Type: "generated", Options: []*PluginOption{}},
			true,
		},
		"Options displayName error": {
			PluginSetting{
				Type: "radio",
				Options: []*PluginOption{{
					Value: "some value",
				}},
			},
			true,
		},
		"Options value error": {
			PluginSetting{
				Type: "radio",
				Options: []*PluginOption{{
					DisplayName: "some name",
				}},
			},
			true,
		},
		"Happy case": {
			PluginSetting{
				Type: "radio",
				Options: []*PluginOption{{
					DisplayName: "Name",
					Value:       "value",
				}},
			},
			false,
		},
		"Valid number setting": {
			PluginSetting{
				Type:    "number",
				Default: 10,
			},
			false,
		},
		"Placeholder is disallowed for bool settings": {
			PluginSetting{
				Type:        "bool",
				Placeholder: "some Text",
			},
			true,
		},
		"Placeholder is allowed for text settings": {
			PluginSetting{
				Type:        "text",
				Placeholder: "some Text",
			},
			false,
		},
		"Placeholder is allowed for long text settings": {
			PluginSetting{
				Type:        "longtext",
				Placeholder: "some Text",
			},
			false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := test.Setting.isValid()
			if test.ExpectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertTypeToPluginSettingType(t *testing.T) {
	testCases := []struct {
		Title               string
		Type                string
		ExpectedSettingType PluginSettingType
		ExpectError         bool
	}{
		{"bool", "bool", Bool, false},
		{"dropdown", "dropdown", Dropdown, false},
		{"generated", "generated", Generated, false},
		{"radio", "radio", Radio, false},
		{"text", "text", Text, false},
		{"longtext", "longtext", LongText, false},
		{"username", "username", Username, false},
		{"custom", "custom", Custom, false},
		{"invalid", "invalid", Bool, true},
	}

	for _, tc := range testCases {
		t.Run(tc.Title, func(t *testing.T) {
			settingType, err := convertTypeToPluginSettingType(tc.Type)
			if !tc.ExpectError {
				assert.Equal(t, settingType, tc.ExpectedSettingType)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestFindManifest(t *testing.T) {
	for _, tc := range []struct {
		Filename       string
		Contents       string
		ExpectError    bool
		ExpectNotExist bool
	}{
		{"foo", "bar", true, true},
		{"plugin.json", "bar", true, false},
		{"plugin.json", `{"id": "foo"}`, false, false},
		{"plugin.json", `{"id": "FOO"}`, false, false},
		{"plugin.yaml", `id: foo`, false, false},
		{"plugin.yaml", "bar", true, false},
		{"plugin.yml", `id: foo`, false, false},
		{"plugin.yml", `id: FOO`, false, false},
		{"plugin.yml", "bar", true, false},
	} {
		dir, err := ioutil.TempDir("", "mm-plugin-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		path := filepath.Join(dir, tc.Filename)
		f, err := os.Create(path)
		require.NoError(t, err)
		_, err = f.WriteString(tc.Contents)
		f.Close()
		require.NoError(t, err)

		m, mpath, err := FindManifest(dir)
		assert.True(t, (err != nil) == tc.ExpectError, tc.Filename)
		assert.True(t, (err != nil && os.IsNotExist(err)) == tc.ExpectNotExist, tc.Filename)
		if !tc.ExpectNotExist {
			assert.Equal(t, path, mpath, tc.Filename)
		} else {
			assert.Empty(t, mpath, tc.Filename)
		}
		if !tc.ExpectError {
			require.NotNil(t, m, tc.Filename)
			assert.NotEmpty(t, m.Id, tc.Filename)
			assert.Equal(t, strings.ToLower(m.Id), m.Id)
		}
	}
}

func TestManifestUnmarshal(t *testing.T) {
	expected := Manifest{
		Id:               "theid",
		HomepageURL:      "https://example.com",
		SupportURL:       "https://example.com/support",
		IconPath:         "assets/icon.svg",
		MinServerVersion: "5.6.0",
		Server: &ManifestServer{
			Executable: "theexecutable",
			Executables: map[string]string{
				"linux-amd64":   "theexecutable-linux-amd64",
				"darwin-amd64":  "theexecutable-darwin-amd64",
				"windows-amd64": "theexecutable-windows-amd64",
				"linux-arm64":   "theexecutable-linux-arm64",
			},
		},
		Webapp: &ManifestWebapp{
			BundlePath: "thebundlepath",
		},
		SettingsSchema: &PluginSettingsSchema{
			Header: "theheadertext",
			Footer: "thefootertext",
			Settings: []*PluginSetting{
				{
					Key:                "thesetting",
					DisplayName:        "thedisplayname",
					Type:               "dropdown",
					HelpText:           "thehelptext",
					RegenerateHelpText: "theregeneratehelptext",
					Placeholder:        "theplaceholder",
					Options: []*PluginOption{
						{
							DisplayName: "theoptiondisplayname",
							Value:       "thevalue",
						},
					},
					Default: "thedefault",
				},
			},
		},
	}

	t.Run("yaml", func(t *testing.T) {
		var yamlResult Manifest
		require.NoError(t, yaml.Unmarshal([]byte(`
id: theid
homepage_url: https://example.com
support_url: https://example.com/support
icon_path: assets/icon.svg
min_server_version: 5.6.0
server:
    executable: theexecutable
    executables:
          linux-amd64: theexecutable-linux-amd64
          darwin-amd64: theexecutable-darwin-amd64
          windows-amd64: theexecutable-windows-amd64
          linux-arm64: theexecutable-linux-arm64
webapp:
    bundle_path: thebundlepath
settings_schema:
    header: theheadertext
    footer: thefootertext
    settings:
        - key: thesetting
          display_name: thedisplayname
          type: dropdown
          help_text: thehelptext
          regenerate_help_text: theregeneratehelptext
          placeholder: theplaceholder
          options:
              - display_name: theoptiondisplayname
                value: thevalue
          default: thedefault
`), &yamlResult))
		assert.Equal(t, expected, yamlResult)
	})

	t.Run("json", func(t *testing.T) {
		var jsonResult Manifest
		require.NoError(t, json.Unmarshal([]byte(`{
	"id": "theid",
	"homepage_url": "https://example.com",
	"support_url": "https://example.com/support",
	"icon_path": "assets/icon.svg",
	"min_server_version": "5.6.0",
	"server": {
		"executable": "theexecutable",
		"executables": {
			"linux-amd64": "theexecutable-linux-amd64",
			"darwin-amd64": "theexecutable-darwin-amd64",
			"windows-amd64": "theexecutable-windows-amd64",
			"linux-arm64": "theexecutable-linux-arm64"
		}
	},
	"webapp": {
		"bundle_path": "thebundlepath"
	},
    "settings_schema": {
        "header": "theheadertext",
        "footer": "thefootertext",
        "settings": [
			{
				"key": "thesetting",
				"display_name": "thedisplayname",
				"type": "dropdown",
				"help_text": "thehelptext",
				"regenerate_help_text": "theregeneratehelptext",
				"placeholder": "theplaceholder",
				"options": [
					{
						"display_name": "theoptiondisplayname",
						"value": "thevalue"
					}
				],
				"default": "thedefault"
			}
		]
    }
	}`), &jsonResult))
		assert.Equal(t, expected, jsonResult)
	})
}

func TestFindManifest_FileErrors(t *testing.T) {
	for _, tc := range []string{"plugin.yaml", "plugin.json"} {
		dir, err := ioutil.TempDir("", "mm-plugin-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		path := filepath.Join(dir, tc)
		require.NoError(t, os.Mkdir(path, 0700))

		m, mpath, err := FindManifest(dir)
		assert.Nil(t, m)
		assert.Equal(t, path, mpath)
		assert.Error(t, err, tc)
		assert.False(t, os.IsNotExist(err), tc)
	}
}

func TestFindManifest_FolderPermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping test while running as root: can't effectively remove permissions")
	}

	for _, tc := range []string{"plugin.yaml", "plugin.json"} {
		dir, err := ioutil.TempDir("", "mm-plugin-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		path := filepath.Join(dir, tc)
		require.NoError(t, os.Mkdir(path, 0700))

		// User does not have permission in the plugin folder
		err = os.Chmod(dir, 0066)
		require.NoError(t, err)

		m, mpath, err := FindManifest(dir)
		assert.Nil(t, m)
		assert.Equal(t, "", mpath)
		assert.Error(t, err, tc)
		assert.False(t, os.IsNotExist(err), tc)
	}
}

func TestManifestJson(t *testing.T) {
	manifest := &Manifest{
		Id: "theid",
		Server: &ManifestServer{
			Executable: "theexecutable",
			Executables: map[string]string{
				"linux-amd64":   "linux-amd64/path/to/executable",
				"darwin-amd64":  "darwin-amd64/path/to/executable",
				"windows-amd64": "windows-amd64/path/to/executable",
				"linux-arm64":   "linux-arm64/path/to/executable",
			},
		},
		Webapp: &ManifestWebapp{
			BundlePath: "thebundlepath",
		},
		SettingsSchema: &PluginSettingsSchema{
			Header: "theheadertext",
			Footer: "thefootertext",
			Settings: []*PluginSetting{
				{
					Key:                "thesetting",
					DisplayName:        "thedisplayname",
					Type:               "dropdown",
					HelpText:           "thehelptext",
					RegenerateHelpText: "theregeneratehelptext",
					Placeholder:        "theplaceholder",
					Options: []*PluginOption{
						{
							DisplayName: "theoptiondisplayname",
							Value:       "thevalue",
						},
					},
					Default: "thedefault",
				},
			},
		},
	}

	json := manifest.ToJson()
	newManifest := ManifestFromJson(strings.NewReader(json))
	assert.Equal(t, newManifest, manifest)
	assert.Equal(t, newManifest.ToJson(), json)
	assert.Equal(t, ManifestFromJson(strings.NewReader("junk")), (*Manifest)(nil))

	manifestList := []*Manifest{manifest}
	json = ManifestListToJson(manifestList)
	newManifestList := ManifestListFromJson(strings.NewReader(json))
	assert.Equal(t, newManifestList, manifestList)
	assert.Equal(t, ManifestListToJson(newManifestList), json)
}

func TestManifestHasClient(t *testing.T) {
	manifest := &Manifest{
		Id: "theid",
		Server: &ManifestServer{
			Executable: "theexecutable",
		},
		Webapp: &ManifestWebapp{
			BundlePath: "thebundlepath",
		},
	}

	assert.True(t, manifest.HasClient())

	manifest.Webapp = nil
	assert.False(t, manifest.HasClient())
}

func TestManifestClientManifest(t *testing.T) {
	manifest := &Manifest{
		Id:               "theid",
		Name:             "thename",
		Description:      "thedescription",
		Version:          "0.0.1",
		MinServerVersion: "5.6.0",
		Server: &ManifestServer{
			Executable: "theexecutable",
		},
		Webapp: &ManifestWebapp{
			BundlePath: "thebundlepath",
			BundleHash: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		},
		SettingsSchema: &PluginSettingsSchema{
			Header: "theheadertext",
			Footer: "thefootertext",
			Settings: []*PluginSetting{
				{
					Key:                "thesetting",
					DisplayName:        "thedisplayname",
					Type:               "dropdown",
					HelpText:           "thehelptext",
					RegenerateHelpText: "theregeneratehelptext",
					Placeholder:        "theplaceholder",
					Options: []*PluginOption{
						{
							DisplayName: "theoptiondisplayname",
							Value:       "thevalue",
						},
					},
					Default: "thedefault",
				},
			},
		},
	}

	sanitized := manifest.ClientManifest()

	assert.Equal(t, manifest.Id, sanitized.Id)
	assert.Equal(t, manifest.Version, sanitized.Version)
	assert.Equal(t, manifest.MinServerVersion, sanitized.MinServerVersion)
	assert.Equal(t, "/static/theid/theid_000102030405060708090a0b0c0d0e0f_bundle.js", sanitized.Webapp.BundlePath)
	assert.Equal(t, manifest.Webapp.BundleHash, sanitized.Webapp.BundleHash)
	assert.Equal(t, manifest.SettingsSchema, sanitized.SettingsSchema)
	assert.Empty(t, sanitized.Name)
	assert.Empty(t, sanitized.Description)
	assert.Empty(t, sanitized.Server)

	assert.NotEmpty(t, manifest.Id)
	assert.NotEmpty(t, manifest.Version)
	assert.NotEmpty(t, manifest.MinServerVersion)
	assert.NotEmpty(t, manifest.Webapp)
	assert.NotEmpty(t, manifest.Name)
	assert.NotEmpty(t, manifest.Description)
	assert.NotEmpty(t, manifest.Server)
	assert.NotEmpty(t, manifest.SettingsSchema)
}

func TestManifestGetExecutableForRuntime(t *testing.T) {
	testCases := []struct {
		Description        string
		Manifest           *Manifest
		GoOs               string
		GoArch             string
		ExpectedExecutable string
	}{
		{
			"no server",
			&Manifest{},
			"linux",
			"amd64",
			"",
		},
		{
			"no executable",
			&Manifest{
				Server: &ManifestServer{},
			},
			"linux",
			"amd64",
			"",
		},
		{
			"single executable",
			&Manifest{
				Server: &ManifestServer{
					Executable: "path/to/executable",
				},
			},
			"linux",
			"amd64",
			"path/to/executable",
		},
		{
			"single executable, different runtime",
			&Manifest{
				Server: &ManifestServer{
					Executable: "path/to/executable",
				},
			},
			"darwin",
			"amd64",
			"path/to/executable",
		},
		{
			"multiple executables, no match",
			&Manifest{
				Server: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
						"linux-arm64":   "linux-arm64/path/to/executable",
					},
				},
			},
			"other",
			"amd64",
			"",
		},
		{
			"multiple executables, linux-amd64 match",
			&Manifest{
				Server: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
						"linux-arm64":   "linux-arm64/path/to/executable",
					},
				},
			},
			"linux",
			"amd64",
			"linux-amd64/path/to/executable",
		},
		{
			"multiple executables, linux-amd64 match, single executable ignored",
			&Manifest{
				Server: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
						"linux-arm64":   "linux-arm64/path/to/executable",
					},
					Executable: "path/to/executable",
				},
			},
			"linux",
			"amd64",
			"linux-amd64/path/to/executable",
		},
		{
			"multiple executables, darwin-amd64 match",
			&Manifest{
				Server: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
						"linux-arm64":   "linux-arm64/path/to/executable",
					},
				},
			},
			"darwin",
			"amd64",
			"darwin-amd64/path/to/executable",
		},
		{
			"multiple executables, windows-amd64 match",
			&Manifest{
				Server: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
						"linux-arm64":   "linux-arm64/path/to/executable",
					},
				},
			},
			"windows",
			"amd64",
			"windows-amd64/path/to/executable",
		},
		{
			"multiple executables, no match, single executable fallback",
			&Manifest{
				Server: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
						"linux-arm64":   "linux-arm64/path/to/executable",
					},
					Executable: "path/to/executable",
				},
			},
			"other",
			"amd64",
			"path/to/executable",
		},
		{
			"deprecated backend field, ignored since server present",
			&Manifest{
				Server: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
						"linux-arm64":   "linux-arm64/path/to/executable",
					},
				},
				Backend: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
					},
				},
			},
			"linux",
			"amd64",
			"linux-amd64/path/to/executable",
		},
		{
			"deprecated backend field used, since no server present",
			&Manifest{
				Backend: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
					},
				},
			},
			"linux",
			"amd64",
			"linux-amd64/path/to/executable",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			assert.Equal(
				t,
				testCase.ExpectedExecutable,
				testCase.Manifest.GetExecutableForRuntime(testCase.GoOs, testCase.GoArch),
			)
		})
	}
}

func TestManifestHasServer(t *testing.T) {
	testCases := []struct {
		Description string
		Manifest    *Manifest
		Expected    bool
	}{
		{
			"no server",
			&Manifest{},
			false,
		},
		{
			"no executable, but server still considered present",
			&Manifest{
				Server: &ManifestServer{},
			},
			true,
		},
		{
			"single executable",
			&Manifest{
				Server: &ManifestServer{
					Executable: "path/to/executable",
				},
			},
			true,
		},
		{
			"multiple executables",
			&Manifest{
				Server: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
					},
				},
			},
			true,
		},
		{
			"single executable defined via deprecated backend",
			&Manifest{
				Backend: &ManifestServer{
					Executable: "path/to/executable",
				},
			},
			true,
		},
		{
			"multiple executables defined via deprecated backend",
			&Manifest{
				Backend: &ManifestServer{
					Executables: map[string]string{
						"linux-amd64":   "linux-amd64/path/to/executable",
						"darwin-amd64":  "darwin-amd64/path/to/executable",
						"windows-amd64": "windows-amd64/path/to/executable",
					},
				},
			},
			true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			assert.Equal(t, testCase.Expected, testCase.Manifest.HasServer())
		})
	}
}

func TestManifestHasWebapp(t *testing.T) {
	testCases := []struct {
		Description string
		Manifest    *Manifest
		Expected    bool
	}{
		{
			"no webapp",
			&Manifest{},
			false,
		},
		{
			"no bundle path, but webapp still considered present",
			&Manifest{
				Webapp: &ManifestWebapp{},
			},
			true,
		},
		{
			"bundle path defined",
			&Manifest{
				Webapp: &ManifestWebapp{
					BundlePath: "path/to/bundle",
				},
			},
			true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			assert.Equal(t, testCase.Expected, testCase.Manifest.HasWebapp())
		})
	}
}

func TestManifestMeetMinServerVersion(t *testing.T) {
	for name, test := range map[string]struct {
		MinServerVersion string
		ServerVersion    string
		ShouldError      bool
		ShouldFulfill    bool
	}{
		"generously fulfilled": {
			MinServerVersion: "5.5.0",
			ServerVersion:    "5.6.0",
			ShouldError:      false,
			ShouldFulfill:    true,
		},
		"exactly fulfilled": {
			MinServerVersion: "5.6.0",
			ServerVersion:    "5.6.0",
			ShouldError:      false,
			ShouldFulfill:    true,
		},
		"not fulfilled": {
			MinServerVersion: "5.6.0",
			ServerVersion:    "5.5.0",
			ShouldError:      false,
			ShouldFulfill:    false,
		},
		"fail to parse MinServerVersion": {
			MinServerVersion: "abc",
			ServerVersion:    "5.5.0",
			ShouldError:      true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			manifest := Manifest{
				MinServerVersion: test.MinServerVersion,
			}
			fulfilled, err := manifest.MeetMinServerVersion(test.ServerVersion)

			if test.ShouldError {
				assert.NotNil(err)
				assert.False(fulfilled)
				return
			}
			assert.Nil(err)
			assert.Equal(test.ShouldFulfill, fulfilled)
		})
	}
}
