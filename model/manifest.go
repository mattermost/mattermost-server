// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type PluginOption struct {
	// The display name for the option.
	DisplayName string `json:"display_name" yaml:"display_name"`

	// The string value for the option.
	Value string `json:"value" yaml:"value"`
}

type PluginSettingType int

const (
	Bool PluginSettingType = iota
	Dropdown
	Generated
	Radio
	Text
	LongText
	Number
	Username
	Custom
)

type PluginSetting struct {
	// The key that the setting will be assigned to in the configuration file.
	Key string `json:"key" yaml:"key"`

	// The display name for the setting.
	DisplayName string `json:"display_name" yaml:"display_name"`

	// The type of the setting.
	//
	// "bool" will result in a boolean true or false setting.
	//
	// "dropdown" will result in a string setting that allows the user to select from a list of
	// pre-defined options.
	//
	// "generated" will result in a string setting that is set to a random, cryptographically secure
	// string.
	//
	// "radio" will result in a string setting that allows the user to select from a short selection
	// of pre-defined options.
	//
	// "text" will result in a string setting that can be typed in manually.
	//
	// "longtext" will result in a multi line string that can be typed in manually.
	//
	// "number" will result in in integer setting that can be typed in manually.
	//
	// "username" will result in a text setting that will autocomplete to a username.
	//
	// "custom" will result in a custom defined setting and will load the custom component registered for the Web App System Console.
	Type string `json:"type" yaml:"type"`

	// The help text to display to the user. Supports Markdown formatting.
	HelpText string `json:"help_text" yaml:"help_text"`

	// The help text to display alongside the "Regenerate" button for settings of the "generated" type.
	RegenerateHelpText string `json:"regenerate_help_text,omitempty" yaml:"regenerate_help_text,omitempty"`

	// The placeholder to display for "generated", "text", "longtext", "number" and "username" types when blank.
	Placeholder string `json:"placeholder" yaml:"placeholder"`

	// The default value of the setting.
	Default interface{} `json:"default" yaml:"default"`

	// For "radio" or "dropdown" settings, this is the list of pre-defined options that the user can choose
	// from.
	Options []*PluginOption `json:"options,omitempty" yaml:"options,omitempty"`
}

type PluginSettingsSchema struct {
	// Optional text to display above the settings. Supports Markdown formatting.
	Header string `json:"header" yaml:"header"`

	// Optional text to display below the settings. Supports Markdown formatting.
	Footer string `json:"footer" yaml:"footer"`

	// A list of setting definitions.
	Settings []*PluginSetting `json:"settings" yaml:"settings"`
}

// The plugin manifest defines the metadata required to load and present your plugin. The manifest
// file should be named plugin.json or plugin.yaml and placed in the top of your
// plugin bundle.
//
// Example plugin.json:
//
//
//    {
//      "id": "com.mycompany.myplugin",
//      "name": "My Plugin",
//      "description": "This is my plugin",
//      "homepage_url": "https://example.com",
//      "support_url": "https://example.com/support",
//      "release_notes_url": "https://example.com/releases/v0.0.1",
//      "icon_path": "assets/logo.svg",
//      "version": "0.1.0",
//      "min_server_version": "5.6.0",
//      "server": {
//        "executables": {
//          "linux-amd64": "server/dist/plugin-linux-amd64",
//          "darwin-amd64": "server/dist/plugin-darwin-amd64",
//          "windows-amd64": "server/dist/plugin-windows-amd64.exe"
//        }
//      },
//      "webapp": {
//          "bundle_path": "webapp/dist/main.js"
//      },
//      "settings_schema": {
//        "header": "Some header text",
//        "footer": "Some footer text",
//        "settings": [{
//          "key": "someKey",
//          "display_name": "Enable Extra Feature",
//          "type": "bool",
//          "help_text": "When true, an extra feature will be enabled!",
//          "default": "false"
//        }]
//      },
//      "props": {
//        "someKey": "someData"
//      }
//    }
type Manifest struct {
	// The id is a globally unique identifier that represents your plugin. Ids must be at least
	// 3 characters, at most 190 characters and must match ^[a-zA-Z0-9-_\.]+$.
	// Reverse-DNS notation using a name you control is a good option, e.g. "com.mycompany.myplugin".
	Id string `json:"id" yaml:"id"`

	// The name to be displayed for the plugin.
	Name string `json:"name" yaml:"name"`

	// A description of what your plugin is and does.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// HomepageURL is an optional link to learn more about the plugin.
	HomepageURL string `json:"homepage_url,omitempty" yaml:"homepage_url,omitempty"`

	// SupportURL is an optional URL where plugin issues can be reported.
	SupportURL string `json:"support_url,omitempty" yaml:"support_url,omitempty"`

	// ReleaseNotesURL is an optional URL where a changelog for the release can be found.
	ReleaseNotesURL string `json:"release_notes_url,omitempty" yaml:"release_notes_url,omitempty"`

	// A relative file path in the bundle that points to the plugins svg icon for use with the Plugin Marketplace.
	// This should be relative to the root of your bundle and the location of the manifest file. Bitmap image formats are not supported.
	IconPath string `json:"icon_path,omitempty" yaml:"icon_path,omitempty"`

	// A version number for your plugin. Semantic versioning is recommended: http://semver.org
	Version string `json:"version" yaml:"version"`

	// The minimum Mattermost server version required for your plugin.
	//
	// Minimum server version: 5.6
	MinServerVersion string `json:"min_server_version,omitempty" yaml:"min_server_version,omitempty"`

	// Server defines the server-side portion of your plugin.
	Server *ManifestServer `json:"server,omitempty" yaml:"server,omitempty"`

	// Backend is a deprecated flag for defining the server-side portion of your plugin. Going forward, use Server instead.
	Backend *ManifestServer `json:"backend,omitempty" yaml:"backend,omitempty"`

	// If your plugin extends the web app, you'll need to define webapp.
	Webapp *ManifestWebapp `json:"webapp,omitempty" yaml:"webapp,omitempty"`

	// To allow administrators to configure your plugin via the Mattermost system console, you can
	// provide your settings schema.
	SettingsSchema *PluginSettingsSchema `json:"settings_schema,omitempty" yaml:"settings_schema,omitempty"`

	// Plugins can store any kind of data in Props to allow other plugins to use it.
	Props map[string]interface{} `json:"props,omitempty" yaml:"props,omitempty"`

	// RequiredConfig defines any required server configuration fields for the plugin to function properly.
	//
	// Use the plugin helpers CheckRequiredServerConfiguration method to enforce this.
	RequiredConfig *Config `json:"required_configuration,omitempty" yaml:"required_configuration,omitempty"`
}

type ManifestServer struct {
	// Executables are the paths to your executable binaries, specifying multiple entry
	// points for different platforms when bundled together in a single plugin.
	Executables map[string]string `json:"executables,omitempty" yaml:"executables,omitempty"`

	// Executable is the path to your executable binary. This should be relative to the root
	// of your bundle and the location of the manifest file.
	//
	// On Windows, this file must have a ".exe" extension.
	//
	// If your plugin is compiled for multiple platforms, consider bundling them together
	// and using the Executables field instead.
	Executable string `json:"executable" yaml:"executable"`
}

// ManifestExecutables is a legacy structure capturing a subet of the known platform executables.
type ManifestExecutables struct {
	// LinuxAmd64 is the path to your executable binary for the corresponding platform
	LinuxAmd64 string `json:"linux-amd64,omitempty" yaml:"linux-amd64,omitempty"`
	// DarwinAmd64 is the path to your executable binary for the corresponding platform
	DarwinAmd64 string `json:"darwin-amd64,omitempty" yaml:"darwin-amd64,omitempty"`
	// WindowsAmd64 is the path to your executable binary for the corresponding platform
	// This file must have a ".exe" extension
	WindowsAmd64 string `json:"windows-amd64,omitempty" yaml:"windows-amd64,omitempty"`
}

type ManifestWebapp struct {
	// The path to your webapp bundle. This should be relative to the root of your bundle and the
	// location of the manifest file.
	BundlePath string `json:"bundle_path" yaml:"bundle_path"`

	// BundleHash is the 64-bit FNV-1a hash of the webapp bundle, computed when the plugin is loaded
	BundleHash []byte `json:"-"`
}

func (m *Manifest) ToJson() string {
	b, _ := json.Marshal(m)
	return string(b)
}

func ManifestListToJson(m []*Manifest) string {
	b, _ := json.Marshal(m)
	return string(b)
}

func ManifestFromJson(data io.Reader) *Manifest {
	var m *Manifest
	json.NewDecoder(data).Decode(&m)
	return m
}

func ManifestListFromJson(data io.Reader) []*Manifest {
	var manifests []*Manifest
	json.NewDecoder(data).Decode(&manifests)
	return manifests
}

func (m *Manifest) HasClient() bool {
	return m.Webapp != nil
}

func (m *Manifest) ClientManifest() *Manifest {
	cm := new(Manifest)
	*cm = *m
	cm.Name = ""
	cm.Description = ""
	cm.Server = nil
	if cm.Webapp != nil {
		cm.Webapp = new(ManifestWebapp)
		*cm.Webapp = *m.Webapp
		cm.Webapp.BundlePath = "/static/" + m.Id + "/" + fmt.Sprintf("%s_%x_bundle.js", m.Id, m.Webapp.BundleHash)
	}
	return cm
}

// GetExecutableForRuntime returns the path to the executable for the given runtime architecture.
//
// If the manifest defines multiple executables, but none match, or if only a single executable
// is defined, the Executable field will be returned. This method does not guarantee that the
// resulting binary can actually execute on the given platform.
func (m *Manifest) GetExecutableForRuntime(goOs, goArch string) string {
	server := m.Server

	// Support the deprecated backend parameter.
	if server == nil {
		server = m.Backend
	}

	if server == nil {
		return ""
	}

	var executable string
	if len(server.Executables) > 0 {
		osArch := fmt.Sprintf("%s-%s", goOs, goArch)
		executable = server.Executables[osArch]
	}

	if executable == "" {
		executable = server.Executable
	}

	return executable
}

func (m *Manifest) HasServer() bool {
	return m.Server != nil || m.Backend != nil
}

func (m *Manifest) HasWebapp() bool {
	return m.Webapp != nil
}

func (m *Manifest) MeetMinServerVersion(serverVersion string) (bool, error) {
	minServerVersion, err := semver.Parse(m.MinServerVersion)
	if err != nil {
		return false, errors.New("failed to parse MinServerVersion")
	}
	sv := semver.MustParse(serverVersion)
	if sv.LT(minServerVersion) {
		return false, nil
	}
	return true, nil
}

func (m *Manifest) IsValid() error {
	if !IsValidPluginId(m.Id) {
		return errors.New("invalid plugin ID")
	}

	if strings.TrimSpace(m.Name) == "" {
		return errors.New("a plugin name is needed")
	}

	if m.HomepageURL != "" && !IsValidHttpUrl(m.HomepageURL) {
		return errors.New("invalid HomepageURL")
	}

	if m.SupportURL != "" && !IsValidHttpUrl(m.SupportURL) {
		return errors.New("invalid SupportURL")
	}

	if m.ReleaseNotesURL != "" && !IsValidHttpUrl(m.ReleaseNotesURL) {
		return errors.New("invalid ReleaseNotesURL")
	}

	if m.Version != "" {
		_, err := semver.Parse(m.Version)
		if err != nil {
			return errors.Wrap(err, "failed to parse Version")
		}
	}

	if m.MinServerVersion != "" {
		_, err := semver.Parse(m.MinServerVersion)
		if err != nil {
			return errors.Wrap(err, "failed to parse MinServerVersion")
		}
	}

	if m.SettingsSchema != nil {
		err := m.SettingsSchema.isValid()
		if err != nil {
			return errors.Wrap(err, "invalid settings schema")
		}
	}

	return nil
}

func (s *PluginSettingsSchema) isValid() error {
	for _, setting := range s.Settings {
		err := setting.isValid()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *PluginSetting) isValid() error {
	pluginSettingType, err := convertTypeToPluginSettingType(s.Type)
	if err != nil {
		return err
	}

	if s.RegenerateHelpText != "" && pluginSettingType != Generated {
		return errors.New("should not set RegenerateHelpText for setting type that is not generated")
	}

	if s.Placeholder != "" && !(pluginSettingType == Generated ||
		pluginSettingType == Text ||
		pluginSettingType == LongText ||
		pluginSettingType == Number ||
		pluginSettingType == Username) {
		return errors.New("should not set Placeholder for setting type not in text, generated or username")
	}

	if s.Options != nil {
		if pluginSettingType != Radio && pluginSettingType != Dropdown {
			return errors.New("should not set Options for setting type not in radio or dropdown")
		}

		for _, option := range s.Options {
			if option.DisplayName == "" || option.Value == "" {
				return errors.New("should not have empty Displayname or Value for any option")
			}
		}
	}

	return nil
}

func convertTypeToPluginSettingType(t string) (PluginSettingType, error) {
	var settingType PluginSettingType
	switch t {
	case "bool":
		return Bool, nil
	case "dropdown":
		return Dropdown, nil
	case "generated":
		return Generated, nil
	case "radio":
		return Radio, nil
	case "text":
		return Text, nil
	case "number":
		return Number, nil
	case "longtext":
		return LongText, nil
	case "username":
		return Username, nil
	case "custom":
		return Custom, nil
	default:
		return settingType, errors.New("invalid setting type: " + t)
	}
}

// FindManifest will find and parse the manifest in a given directory.
//
// In all cases other than a does-not-exist error, path is set to the path of the manifest file that was
// found.
//
// Manifests are JSON or YAML files named plugin.json, plugin.yaml, or plugin.yml.
func FindManifest(dir string) (manifest *Manifest, path string, err error) {
	for _, name := range []string{"plugin.yml", "plugin.yaml"} {
		path = filepath.Join(dir, name)
		f, ferr := os.Open(path)
		if ferr != nil {
			if !os.IsNotExist(ferr) {
				return nil, "", ferr
			}
			continue
		}
		b, ioerr := ioutil.ReadAll(f)
		f.Close()
		if ioerr != nil {
			return nil, path, ioerr
		}
		var parsed Manifest
		err = yaml.Unmarshal(b, &parsed)
		if err != nil {
			return nil, path, err
		}
		manifest = &parsed
		manifest.Id = strings.ToLower(manifest.Id)
		return manifest, path, nil
	}

	path = filepath.Join(dir, "plugin.json")
	f, ferr := os.Open(path)
	if ferr != nil {
		if os.IsNotExist(ferr) {
			path = ""
		}
		return nil, path, ferr
	}
	defer f.Close()
	var parsed Manifest
	err = json.NewDecoder(f).Decode(&parsed)
	if err != nil {
		return nil, path, err
	}
	manifest = &parsed
	manifest.Id = strings.ToLower(manifest.Id)
	return manifest, path, nil
}
