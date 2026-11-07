// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package database

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultConfig() *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	return cfg
}

// newSnapshot builds a snapshot with each non-nil source collected; diag becomes the leader's diagnostics.
func newSnapshot(cfg *model.Config, stats *model.SupportPacketStats, diag *model.SupportPacketDiagnostics) *healthcheck.Snapshot {
	var nodes []*healthcheck.NodeSnapshot
	if diag != nil {
		nodes = []*healthcheck.NodeSnapshot{{
			Hostname:    "app-1.example.com",
			IsLeader:    true,
			Diagnostics: &model.NodeDiagnostics{Diagnostics: diag},
		}}
	}

	s := healthcheck.NewSnapshot(nodes)
	s.Sections = map[model.WorkspaceSection]error{}
	if cfg != nil {
		s.Config = &model.SupportPacketConfig{Config: cfg}
		s.Sections[model.SectionConfig] = nil
	}
	if stats != nil {
		s.Stats = stats
		s.Sections[model.SectionStats] = nil
	}
	return s
}

type want struct {
	state     healthcheck.State
	messageID string
	value     *float64
}

var (
	resolvedWant       = want{state: healthcheck.StateResolved}
	unknownConfigWant  = want{state: healthcheck.StateUnknown, messageID: healthcheck.ReasonConfigUnavailable}
	unknownStatsWant   = want{state: healthcheck.StateUnknown, messageID: healthcheck.ReasonStatsUnavailable}
	unknownDiagWant    = want{state: healthcheck.StateUnknown, messageID: healthcheck.ReasonDiagnosticsUnavailable}
	unknownLicenseWant = want{state: healthcheck.StateUnknown, messageID: healthcheck.ReasonLicenseUnavailable}
)

// onClusterLicense puts diag on an Enterprise license with the Cluster feature.
func onClusterLicense(diag *model.SupportPacketDiagnostics) *model.SupportPacketDiagnostics {
	diag.License = model.SupportPacketLicense{SkuShortName: model.LicenseShortSkuEnterprise, Features: &model.Features{Cluster: new(true)}}
	return diag
}

func withLicense(diag *model.SupportPacketDiagnostics, license model.SupportPacketLicense) *model.SupportPacketDiagnostics {
	diag.License = license
	return diag
}

func firingWant(messageID string) want {
	return want{state: healthcheck.StateFiring, messageID: messageID}
}

func (w want) withValue(v float64) want {
	w.value = &v
	return w
}

func assertResult(t *testing.T, rule healthcheck.Rule, s *healthcheck.Snapshot, expected want) {
	t.Helper()
	assertResults(t, rule.Eval(s), expected, rule.Code)
}

func assertResults(t *testing.T, results []healthcheck.Result, expected want, msgAndArgs ...any) {
	t.Helper()

	require.Len(t, results, 1, msgAndArgs...)
	assert.Equal(t, expected.state, results[0].State, msgAndArgs...)
	assert.Equal(t, expected.messageID, results[0].MessageID, msgAndArgs...)
	assert.Equal(t, expected.value, results[0].Value, msgAndArgs...)
}

func TestRegistered(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code       string
		severity   healthcheck.Severity
		volatility healthcheck.Volatility
		subject    string
		nodeScoped bool
	}{
		{"DB_NO_READ_REPLICA", healthcheck.SeverityInfo, healthcheck.VolatilityThreshold, "SqlSettings.DataSourceReplicas", false},
		{"DB_NO_SEARCH_REPLICA", healthcheck.SeverityWarning, healthcheck.VolatilityThreshold, "SqlSettings.DataSourceSearchReplicas", false},
		{"DB_TRACE_ON", healthcheck.SeverityWarning, healthcheck.VolatilityStable, "SqlSettings.Trace", false},
		{"DB_MAX_OPEN_CONNS_LOW", healthcheck.SeverityInfo, healthcheck.VolatilityThreshold, "SqlSettings.MaxOpenConns", false},
		{"DB_QUERY_TIMEOUT_HIGH", healthcheck.SeverityWarning, healthcheck.VolatilityStable, "SqlSettings.QueryTimeout", false},
		{"DB_HA_FAILOVER_STORM_RISK", healthcheck.SeverityWarning, healthcheck.VolatilityTopology, "SqlSettings.MaxOpenConns", false},
		{"DB_POSTGRES_UNSUPPORTED", healthcheck.SeverityWarning, healthcheck.VolatilityStable, "Database.Version", false},
		{"DB_POOL_SATURATED", healthcheck.SeverityCritical, healthcheck.VolatilityThreshold, "SqlSettings.MaxOpenConns", true},
		{"DB_POSTS_DEAD_TUPLES", healthcheck.SeverityWarning, healthcheck.VolatilityThreshold, "Database.PostsDeadTuples", false},
		{"DB_REPLICA_UNUSED", healthcheck.SeverityWarning, healthcheck.VolatilityProbe, "SqlSettings.DataSourceReplicas", false},
		{"DB_IDLE_EXCEEDS_OPEN", healthcheck.SeverityInfo, healthcheck.VolatilityStable, "SqlSettings.MaxIdleConns", false},
	}

	registry := healthcheck.Builtin()
	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()

			rule, ok := registry.Get(tc.code)
			require.True(t, ok)
			assert.Equal(t, model.AreaDatabase, rule.Area)
			assert.Equal(t, tc.severity, rule.Severity)
			assert.Equal(t, tc.volatility, rule.Volatility)
			assert.Equal(t, healthcheck.SurfaceProduct, rule.Surface)
			assert.Equal(t, tc.subject, rule.Subject)
			assert.False(t, rule.AppliesToCloud)
			assert.Equal(t, !tc.nodeScoped, rule.Eval != nil)
			assert.Equal(t, tc.nodeScoped, rule.EvalNode != nil)
		})
	}
}
