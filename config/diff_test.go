// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"testing"

	"github.com/mattermost/mattermost-server/v5/model"

	"github.com/stretchr/testify/require"
)

func defaultConfigGen() *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	return cfg
}

func BenchmarkDiff(b *testing.B) {
	b.Run("equal empty", func(b *testing.B) {
		baseCfg := &model.Config{}
		actualCfg := &model.Config{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = Diff(baseCfg, actualCfg)
		}
	})

	b.Run("equal with defaults", func(b *testing.B) {
		baseCfg := defaultConfigGen()
		actualCfg := defaultConfigGen()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = Diff(baseCfg, actualCfg)
		}
	})

	b.Run("actual empty", func(b *testing.B) {
		baseCfg := defaultConfigGen()
		actualCfg := &model.Config{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = Diff(baseCfg, actualCfg)
		}
	})

	b.Run("base empty", func(b *testing.B) {
		baseCfg := &model.Config{}
		actualCfg := defaultConfigGen()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = Diff(baseCfg, actualCfg)
		}
	})

	b.Run("some diffs", func(b *testing.B) {
		baseCfg := defaultConfigGen()
		actualCfg := defaultConfigGen()
		baseCfg.ServiceSettings.SiteURL = model.NewString("http://localhost")
		baseCfg.ServiceSettings.ReadTimeout = model.NewInt(300)
		baseCfg.SQLSettings.QueryTimeout = model.NewInt(0)
		actualCfg.PluginSettings.EnableUploads = nil
		actualCfg.TeamSettings.MaxChannelsPerTeam = model.NewInt64(100000)
		actualCfg.FeatureFlags = nil
		actualCfg.SQLSettings.DataSourceReplicas = []string{
			"ds0",
			"ds1",
			"ds2",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = Diff(baseCfg, actualCfg)
		}
	})
}

func TestDiff(t *testing.T) {
	tcs := []struct {
		name   string
		base   *model.Config
		actual *model.Config
		diffs  ConfigDiffs
		err    string
	}{
		{
			"nil",
			nil,
			nil,
			nil,
			"input configs should not be nil",
		},
		{
			"empty",
			&model.Config{},
			&model.Config{},
			nil,
			"",
		},
		{
			"defaults",
			defaultConfigGen(),
			defaultConfigGen(),
			nil,
			"",
		},
		{
			"default base, actual empty",
			defaultConfigGen(),
			&model.Config{},
			ConfigDiffs{
				{
					"",
					*defaultConfigGen(),
					model.Config{},
				},
			},
			"",
		},
		{
			"empty base, actual default",
			&model.Config{},
			defaultConfigGen(),
			ConfigDiffs{
				{
					"",
					model.Config{},
					*defaultConfigGen(),
				},
			},
			"",
		},
		{
			"string change",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.ServiceSettings.SiteURL = model.NewString("http://changed")
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:      "ServiceSettings.SiteURL",
					BaseVal:   *defaultConfigGen().ServiceSettings.SiteURL,
					ActualVal: "http://changed",
				},
			},
			"",
		},
		{
			"string nil",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.ServiceSettings.SiteURL = nil
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:    "ServiceSettings.SiteURL",
					BaseVal: defaultConfigGen().ServiceSettings.SiteURL,
					ActualVal: func() *string {
						return nil
					}(),
				},
			},
			"",
		},
		{
			"bool change",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.PluginSettings.Enable = model.NewBool(!*cfg.PluginSettings.Enable)
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:      "PluginSettings.Enable",
					BaseVal:   true,
					ActualVal: false,
				},
			},
			"",
		},
		{
			"bool nil",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.PluginSettings.Enable = nil
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:    "PluginSettings.Enable",
					BaseVal: defaultConfigGen().PluginSettings.Enable,
					ActualVal: func() *bool {
						return nil
					}(),
				},
			},
			"",
		},
		{
			"int change",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.ServiceSettings.ReadTimeout = model.NewInt(0)
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:      "ServiceSettings.ReadTimeout",
					BaseVal:   *defaultConfigGen().ServiceSettings.ReadTimeout,
					ActualVal: 0,
				},
			},
			"",
		},
		{
			"int nil",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.ServiceSettings.ReadTimeout = nil
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:    "ServiceSettings.ReadTimeout",
					BaseVal: defaultConfigGen().ServiceSettings.ReadTimeout,
					ActualVal: func() *int {
						return nil
					}(),
				},
			},
			"",
		},
		{
			"slice addition",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.SQLSettings.DataSourceReplicas = []string{
					"ds0",
					"ds1",
				}
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:    "SqlSettings.DataSourceReplicas",
					BaseVal: defaultConfigGen().SQLSettings.DataSourceReplicas,
					ActualVal: []string{
						"ds0",
						"ds1",
					},
				},
			},
			"",
		},
		{
			"slice deletion",
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.SQLSettings.DataSourceReplicas = []string{
					"ds0",
					"ds1",
				}
				return cfg
			}(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.SQLSettings.DataSourceReplicas = []string{
					"ds0",
				}
				return cfg
			}(),
			ConfigDiffs{
				{
					Path: "SqlSettings.DataSourceReplicas",
					BaseVal: []string{
						"ds0",
						"ds1",
					},
					ActualVal: []string{
						"ds0",
					},
				},
			},
			"",
		},
		{
			"slice nil",
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.SQLSettings.DataSourceReplicas = []string{
					"ds0",
					"ds1",
				}
				return cfg
			}(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.SQLSettings.DataSourceReplicas = nil
				return cfg
			}(),
			ConfigDiffs{
				{
					Path: "SqlSettings.DataSourceReplicas",
					BaseVal: []string{
						"ds0",
						"ds1",
					},
					ActualVal: func() []string {
						return nil
					}(),
				},
			},
			"",
		},
		{
			"map change",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.PluginSettings.PluginStates["com.mattermost.nps"] = &model.PluginState{
					Enable: !cfg.PluginSettings.PluginStates["com.mattermost.nps"].Enable,
				}
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:    "PluginSettings.PluginStates",
					BaseVal: defaultConfigGen().PluginSettings.PluginStates,
					ActualVal: map[string]*model.PluginState{
						"com.mattermost.nps": {
							Enable: !defaultConfigGen().PluginSettings.PluginStates["com.mattermost.nps"].Enable,
						},
					},
				},
			},
			"",
		},
		{
			"map addition",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.PluginSettings.PluginStates["com.mattermost.newplugin"] = &model.PluginState{
					Enable: true,
				}
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:    "PluginSettings.PluginStates",
					BaseVal: defaultConfigGen().PluginSettings.PluginStates,
					ActualVal: map[string]*model.PluginState{
						"com.mattermost.nps": {
							Enable: defaultConfigGen().PluginSettings.PluginStates["com.mattermost.nps"].Enable,
						},
						"com.mattermost.newplugin": {
							Enable: true,
						},
					},
				},
			},
			"",
		},
		{
			"map deletion",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				delete(cfg.PluginSettings.PluginStates, "com.mattermost.nps")
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:    "PluginSettings.PluginStates",
					BaseVal: defaultConfigGen().PluginSettings.PluginStates,
					ActualVal: func() interface{} {
						return map[string]*model.PluginState{}
					}(),
				},
			},
			"",
		},
		{
			"map nil",
			defaultConfigGen(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.PluginSettings.PluginStates = nil
				return cfg
			}(),
			ConfigDiffs{
				{
					Path:    "PluginSettings.PluginStates",
					BaseVal: defaultConfigGen().PluginSettings.PluginStates,
					ActualVal: func() map[string]*model.PluginState {
						return nil
					}(),
				},
			},
			"",
		},
		{
			"map type change",
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.PluginSettings.Plugins = map[string]map[string]interface{}{
					"com.mattermost.newplugin": {
						"key": true,
					},
				}
				return cfg
			}(),
			func() *model.Config {
				cfg := defaultConfigGen()
				cfg.PluginSettings.Plugins = map[string]map[string]interface{}{
					"com.mattermost.newplugin": {
						"key": "string",
					},
				}
				return cfg
			}(),
			ConfigDiffs{
				{
					Path: "PluginSettings.Plugins",
					BaseVal: func() interface{} {
						return map[string]map[string]interface{}{
							"com.mattermost.newplugin": {
								"key": true,
							},
						}
					}(),
					ActualVal: func() interface{} {
						return map[string]map[string]interface{}{
							"com.mattermost.newplugin": {
								"key": "string",
							},
						}
					}(),
				},
			},
			"",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			diffs, err := Diff(tc.base, tc.actual)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				require.Nil(t, diffs)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.diffs, diffs)
		})
	}
}
