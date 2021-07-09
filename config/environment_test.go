// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func modifiedDefault(modify func(*model.Config)) *model.Config {
	def := defaultConfig()
	modify(def)
	return def
}

func defaultConfig() *model.Config {
	def := &model.Config{}
	def.SetDefaults()
	return def
}

func TestRemoveEnvOverrides(t *testing.T) {
	var tests = []struct {
		name           string
		inputConfig    *model.Config
		env            map[string]string
		expectedConfig *model.Config
	}{
		{
			name: "basic override",
			inputConfig: modifiedDefault(func(in *model.Config) {
				*in.ServiceSettings.TLSMinVer = "1.4"
			}),
			env: map[string]string{
				"MM_SERVICESETTINGS_TLSMINVER": "1.5",
			},
			expectedConfig: modifiedDefault(func(in *model.Config) {
				*in.ServiceSettings.TLSMinVer = "1.5"
			}),
		},
		{
			name: "feature flags",
			inputConfig: modifiedDefault(func(in *model.Config) {
				in.FeatureFlags.TestFeature = "somevalue"
			}),
			env: map[string]string{
				"MM_FEATUREFLAGS_TESTFEATURE": "correctvalue",
			},
			expectedConfig: modifiedDefault(func(in *model.Config) {
				in.FeatureFlags.TestFeature = "correctvalue"
			}),
		},
		{
			name: "int setting",
			inputConfig: modifiedDefault(func(in *model.Config) {
				*in.ClusterSettings.GossipPort = 500
			}),
			env: map[string]string{
				"MM_CLUSTERSETTINGS_GOSSIPPORT": "600",
			},
			expectedConfig: modifiedDefault(func(in *model.Config) {
				*in.ClusterSettings.GossipPort = 600
			}),
		},
		{
			name: "int64 setting",
			inputConfig: modifiedDefault(func(in *model.Config) {
				*in.ServiceSettings.TLSStrictTransportMaxAge = 500
			}),
			env: map[string]string{
				"MM_SERVICESETTINGS_TLSSTRICTTRANSPORTMAXAGE": "4294967294",
			},
			expectedConfig: modifiedDefault(func(in *model.Config) {
				*in.ServiceSettings.TLSStrictTransportMaxAge = 4294967294
			}),
		},
		{
			name: "bool setting",
			inputConfig: modifiedDefault(func(in *model.Config) {
				*in.ClusterSettings.UseIDAddress = false
			}),
			env: map[string]string{
				"MM_CLUSTERSETTINGS_USEIPADDRESS": "true",
			},
			expectedConfig: modifiedDefault(func(in *model.Config) {
				*in.ClusterSettings.UseIDAddress = true
			}),
		},
		{
			name: "[]string setting",
			inputConfig: modifiedDefault(func(in *model.Config) {
				in.SqlSettings.DataSourceReplicas = []string{"somthing"}
			}),
			env: map[string]string{
				"MM_SQLSETTINGS_DATASOURCEREPLICAS": "otherthing alsothis",
			},
			expectedConfig: modifiedDefault(func(in *model.Config) {
				in.SqlSettings.DataSourceReplicas = []string{"otherthing", "alsothis"}
			}),
		},
		{
			name: "bad env",
			inputConfig: modifiedDefault(func(in *model.Config) {
			}),
			env: map[string]string{
				"MM_SERVICESETTINGS":        "huh?",
				"NOTMM":                     "huh?",
				"MM_NOTEXIST":               "huh?",
				"MM_NOTEXIST_MORE_AND_MORE": "huh?",
				"MM_":                       "huh?",
				"MM":                        "huh?",
				"MM__":                      "huh?",
				"_":                         "huh?",
			},
			expectedConfig: modifiedDefault(func(in *model.Config) {
			}),
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expectedConfig, applyEnvironmentMap(testCase.inputConfig, testCase.env))
		})
	}
}
