// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetClientConfig(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description    string
		config         *model.Config
		telemetryID    string
		license        *model.License
		expectedFields map[string]string
	}{
		{
			"unlicensed",
			&model.Config{
				EmailSettings: model.EmailSettings{
					EmailNotificationContentsType: model.NewString(model.EmailNotificationContentsFull),
				},
				ThemeSettings: model.ThemeSettings{
					// Ignored, since not licensed.
					AllowCustomThemes: model.NewBool(false),
				},
				ServiceSettings: model.ServiceSettings{
					WebsocketURL:        model.NewString("ws://mattermost.example.com:8065"),
					WebsocketPort:       model.NewInt(80),
					WebsocketSecurePort: model.NewInt(443),
				},
			},
			"",
			nil,
			map[string]string{
				"DiagnosticId":                     "",
				"EmailNotificationContentsType":    "full",
				"AllowCustomThemes":                "true",
				"EnforceMultifactorAuthentication": "false",
				"WebsocketURL":                     "ws://mattermost.example.com:8065",
				"WebsocketPort":                    "80",
				"WebsocketSecurePort":              "443",
			},
		},
		{
			"licensed, but not for theme management",
			&model.Config{
				EmailSettings: model.EmailSettings{
					EmailNotificationContentsType: model.NewString(model.EmailNotificationContentsFull),
				},
				ThemeSettings: model.ThemeSettings{
					// Ignored, since not licensed.
					AllowCustomThemes: model.NewBool(false),
				},
			},
			"tag1",
			&model.License{
				Features: &model.Features{
					ThemeManagement: model.NewBool(false),
				},
			},
			map[string]string{
				"DiagnosticId":                  "tag1",
				"EmailNotificationContentsType": "full",
				"AllowCustomThemes":             "true",
			},
		},
		{
			"licensed for theme management",
			&model.Config{
				EmailSettings: model.EmailSettings{
					EmailNotificationContentsType: model.NewString(model.EmailNotificationContentsFull),
				},
				ThemeSettings: model.ThemeSettings{
					AllowCustomThemes: model.NewBool(false),
				},
			},
			"tag2",
			&model.License{
				Features: &model.Features{
					ThemeManagement: model.NewBool(true),
				},
			},
			map[string]string{
				"DiagnosticId":                  "tag2",
				"EmailNotificationContentsType": "full",
				"AllowCustomThemes":             "false",
			},
		},
		{
			"licensed for enforcement",
			&model.Config{
				ServiceSettings: model.ServiceSettings{
					EnforceMultifactorAuthentication: model.NewBool(true),
				},
			},
			"tag1",
			&model.License{
				Features: &model.Features{
					MFA: model.NewBool(true),
				},
			},
			map[string]string{
				"EnforceMultifactorAuthentication": "true",
			},
		},
		{
			"experimental channel organization enabled",
			&model.Config{
				ServiceSettings: model.ServiceSettings{
					ExperimentalChannelOrganization: model.NewBool(true),
				},
			},
			"tag1",
			nil,
			map[string]string{
				"ExperimentalChannelOrganization": "true",
			},
		},
		{
			"experimental channel organization disabled, but experimental group unread channels on",
			&model.Config{
				ServiceSettings: model.ServiceSettings{
					ExperimentalChannelOrganization: model.NewBool(false),
					ExperimentalGroupUnreadChannels: model.NewString(model.GroupUnreadChannelsDefaultOn),
				},
			},
			"tag1",
			nil,
			map[string]string{
				"ExperimentalChannelOrganization": "true",
			},
		},
		{
			"default marketplace",
			&model.Config{
				PluginSettings: model.PluginSettings{
					MarketplaceURL: model.NewString(model.PluginSettingsDefaultMarketplaceURL),
				},
			},
			"tag1",
			nil,
			map[string]string{
				"IsDefaultMarketplace": "true",
			},
		},
		{
			"non-default marketplace",
			&model.Config{
				PluginSettings: model.PluginSettings{
					MarketplaceURL: model.NewString("http://example.com"),
				},
			},
			"tag1",
			nil,
			map[string]string{
				"IsDefaultMarketplace": "false",
			},
		},
		{
			"enable ShowFullName prop",
			&model.Config{
				PrivacySettings: model.PrivacySettings{
					ShowFullName: model.NewBool(true),
				},
			},
			"tag1",
			nil,
			map[string]string{
				"ShowFullName": "true",
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			testCase.config.SetDefaults()
			if testCase.license != nil {
				testCase.license.Features.SetDefaults()
			}

			configMap := GenerateClientConfig(testCase.config, testCase.telemetryID, testCase.license)
			for expectedField, expectedValue := range testCase.expectedFields {
				actualValue, ok := configMap[expectedField]
				if assert.True(t, ok, fmt.Sprintf("config does not contain %v", expectedField)) {
					assert.Equal(t, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestGetLimitedClientConfig(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description    string
		config         *model.Config
		telemetryID    string
		license        *model.License
		expectedFields map[string]string
	}{
		{
			"unlicensed",
			&model.Config{
				EmailSettings: model.EmailSettings{
					EmailNotificationContentsType: model.NewString(model.EmailNotificationContentsFull),
				},
				ThemeSettings: model.ThemeSettings{
					// Ignored, since not licensed.
					AllowCustomThemes: model.NewBool(false),
				},
				ServiceSettings: model.ServiceSettings{
					WebsocketURL:        model.NewString("ws://mattermost.example.com:8065"),
					WebsocketPort:       model.NewInt(80),
					WebsocketSecurePort: model.NewInt(443),
				},
			},
			"",
			nil,
			map[string]string{
				"DiagnosticId":                     "",
				"EnforceMultifactorAuthentication": "false",
				"WebsocketURL":                     "ws://mattermost.example.com:8065",
				"WebsocketPort":                    "80",
				"WebsocketSecurePort":              "443",
			},
		},
		{
			"password settings",
			&model.Config{
				PasswordSettings: model.PasswordSettings{
					MinimumLength: model.NewInt(15),
					Lowercase:     model.NewBool(true),
					Uppercase:     model.NewBool(true),
					Number:        model.NewBool(true),
					Symbol:        model.NewBool(false),
				},
			},
			"",
			nil,
			map[string]string{
				"PasswordMinimumLength":    "15",
				"PasswordRequireLowercase": "true",
				"PasswordRequireUppercase": "true",
				"PasswordRequireNumber":    "true",
				"PasswordRequireSymbol":    "false",
			},
		},
		{
			"Feature Flags",
			&model.Config{
				FeatureFlags: &model.FeatureFlags{
					TestFeature: "myvalue",
				},
			},
			"",
			nil,
			map[string]string{
				"FeatureFlagTestFeature": "myvalue",
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			testCase.config.SetDefaults()
			if testCase.license != nil {
				testCase.license.Features.SetDefaults()
			}

			configMap := GenerateLimitedClientConfig(testCase.config, testCase.telemetryID, testCase.license)
			for expectedField, expectedValue := range testCase.expectedFields {
				actualValue, ok := configMap[expectedField]
				if assert.True(t, ok, fmt.Sprintf("config does not contain %v", expectedField)) {
					assert.Equal(t, expectedValue, actualValue)
				}
			}
		})
	}
}
