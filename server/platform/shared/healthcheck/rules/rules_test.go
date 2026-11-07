// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package rules

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinRegistry(t *testing.T) {
	registry := healthcheck.Builtin()
	require.NoError(t, registry.Validate())

	var codes []string
	for _, rule := range registry.Rules() {
		codes = append(codes, rule.Code)
	}

	// One block per rule package, in rules.go import order, so concurrent rule PRs insert in different places.
	assert.ElementsMatch(t, []string{
		// auth
		"LDAP_PORT_TLS_MISMATCH",
		"LDAP_PORT_PLAIN_MISMATCH",
		"LDAP_SKIP_CERT",
		"LDAP_QUERY_TIMEOUT_LOW",
		"LDAP_SYNC_NO_BASEDN",
		"LDAP_RECOMMENDED_AT_SCALE",
		"LDAP_ID_IS_EMAIL",
		"LDAP_SYNC_DUTY_CYCLE",
		"LDAP_PROBE_FAILED",
		"OAUTH_PROBE_FAILED",
		"SAML_PROBE_FAILED",
		"SAML_SIGNATURE_SHA1",
		"SAML_VERIFY_OFF",
		"SAML_ENCRYPT_OFF",
		"SAML_ID_IS_EMAIL",
		"SAML_ID_MUTABLE",
		"SESSION_EXTEND_OFF",

		// cluster
		"CLUSTER_GOSSIP_UNENCRYPTED",
		"CLUSTER_LRU_CACHE",
		"CLUSTER_SINGLE_NODE",
		"CLUSTER_BAD_ADVERTISE",
		"CLUSTER_CONFIG_WRITABLE",
		"CLUSTER_VERSION_SKEW",
		"NODE_NOT_RESPONDING",

		// compliance
		"AUDIT_LOG_OFF",
		"COMPLIANCE_NO_DIR",
		"RETENTION_TIGHT_BATCHES",
		"EXPORT_GR_TIMEOUT_LOW",

		// database
		"DB_HA_FAILOVER_STORM_RISK",
		"DB_POSTGRES_UNSUPPORTED",
		"DB_POOL_SATURATED",
		"DB_POSTS_DEAD_TUPLES",
		"DB_REPLICA_UNUSED",
		"DB_NO_READ_REPLICA",
		"DB_NO_SEARCH_REPLICA",
		"DB_TRACE_ON",
		"DB_MAX_OPEN_CONNS_LOW",
		"DB_QUERY_TIMEOUT_HIGH",
		"DB_IDLE_EXCEEDS_OPEN",

		// jobs
		"JOB_STUCK",
		"JOB_WEDGED_AT_ZERO",
		"JOB_FAILED",

		// license
		"LICENSE_EXPIRED",
		"LICENSE_EXPIRING",
		"SEATS_OVER_DEPLOYED",
		"SEATS_LIMIT_REACHED",
		"SEATS_NEAR_CAPACITY",

		// notifications
		"PUSH_ID_ONLY",
		"MAX_NOTIFICATIONS_PER_CHANNEL",
		"SMTP_UNREACHABLE",
		"PUSH_EMPTY_URL",
		"PUSH_BAD_SCHEME",
		"PUSH_TEST_PROXY",
		"PUSH_PROXY_UNREACHABLE",

		// platform
		"FILESTORE_LOCAL_IN_CLUSTER",
		"FILESTORE_S3_NO_BUCKET",
		"FILESTORE_S3_SSL_OFF",
		"FILESTORE_DISK_LOW",
		"FILESTORE_DISK_FULL",
		"FILESTORE_UNREACHABLE",
		"ENV_UNSUPPORTED_OS",
		"BIND_PRIVILEGED_PORT",
		"NODE_FD_EXHAUSTION",
		"NODE_CONTAINER_MEMORY_PRESSURE",
		"NODE_CONTAINER_OOM_KILLS",
		"NODE_HOST_MEMORY_LOW",
		"NODE_GO_MEMORY_LIMIT_ABOVE_CONTAINER",
		"NODE_CPU_THROTTLED",
		"NODE_GOMAXPROCS_ABOVE_CPU_LIMIT",
		"NODE_HOST_LOAD_HIGH",
		"LOG_DEBUG_PROD",
		"LOG_FILE_OFF",
		"METRICS_OFF",
		"PLUGINS_OFF",
		"PLUGIN_HEALTH_OFF",
		"PLUGIN_NOT_RUNNING",
		"RATELIMIT_OFF_LARGE",
		"RATELIMIT_STORE_SMALL",
		"SITE_URL_EMPTY",
		"SITE_URL_HTTP",

		// search
		"ES_LOCALHOST_URL",
		"ES_SERVER_ERROR",
		"SEARCH_UNAVAILABLE",
		"SCALE_ES_REQUIRED",
		"SCALE_ES_RECOMMENDED",
		"ES_LIVE_BATCH_SYNC",
		"ES_LIVE_BATCH_TOO_HIGH",
		"ES_VERSION_UNSUPPORTED",
		"ES_MISSING_ICU",
		"ES_SKIP_TLS_VERIFY",

		// version
		"VERSION_BEHIND_ONE",
		"VERSION_BEHIND",
		"VERSION_EOL",
		"VERSION_ESR_NEAR_EOL",
	}, codes)
}

func TestBuiltinRulesOnCloud(t *testing.T) {
	var cloud []string
	for _, rule := range healthcheck.Builtin().Rules() {
		if rule.AppliesTo(true) {
			cloud = append(cloud, rule.Code)
		}
	}

	assert.ElementsMatch(t, []string{
		"LDAP_ID_IS_EMAIL",
		"LDAP_PORT_PLAIN_MISMATCH",
		"LDAP_PORT_TLS_MISMATCH",
		"LDAP_PROBE_FAILED",
		"LDAP_QUERY_TIMEOUT_LOW",
		"LDAP_RECOMMENDED_AT_SCALE",
		"LDAP_SKIP_CERT",
		"LDAP_SYNC_DUTY_CYCLE",
		"LDAP_SYNC_NO_BASEDN",
		"OAUTH_PROBE_FAILED",
		"PLUGIN_NOT_RUNNING",
		"PUSH_BAD_SCHEME",
		"PUSH_EMPTY_URL",
		"PUSH_ID_ONLY",
		"PUSH_PROXY_UNREACHABLE",
		"PUSH_TEST_PROXY",
		"SAML_ENCRYPT_OFF",
		"SAML_ID_IS_EMAIL",
		"SAML_ID_MUTABLE",
		"SAML_PROBE_FAILED",
		"SAML_SIGNATURE_SHA1",
		"SAML_VERIFY_OFF",
	}, cloud)
}

// A node that did not reply yields one node-down finding, and the reconciler drops the
// unknown results the node's other per-node rules give.
func TestNodeNotRespondingSuppressesNodeUnknowns(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.ClusterSettings.Enable = new(true)
	cfg.ClusterSettings.ClusterName = new("production")

	node := func(hostname string, leader bool) *healthcheck.NodeSnapshot {
		diag := &model.SupportPacketDiagnostics{License: model.SupportPacketLicense{
			SkuShortName: model.LicenseShortSkuEnterprise,
			Features:     &model.Features{Cluster: new(true)},
		}}
		diag.Server.OpenFileDescriptors = new(int64(120))
		diag.Server.MaxFileDescriptors = new(int64(65536))
		diag.Database.MasterConnectionsInUse = new(10)
		return &healthcheck.NodeSnapshot{Hostname: hostname, IsLeader: leader, Diagnostics: &model.NodeDiagnostics{Diagnostics: diag}}
	}
	s := healthcheck.NewSnapshot([]*healthcheck.NodeSnapshot{
		node("app-1", true),
		node("app-2", false),
		{Hostname: "app-3", CollectErr: healthcheck.ErrNodeNoReply},
	})
	s.Config = &model.SupportPacketConfig{Config: cfg}
	s.Sections = map[model.WorkspaceSection]error{model.SectionConfig: nil}

	now := time.UnixMilli(1_000_000)
	evaluations := healthcheck.NewEngine(healthcheck.EngineOpts{Now: func() time.Time { return now }}).Evaluate(s)

	var nodeDown, app3Unknowns []healthcheck.Evaluation
	for _, evaluation := range evaluations {
		switch {
		case evaluation.Code == "NODE_NOT_RESPONDING" && evaluation.Result.State == healthcheck.StateFiring:
			nodeDown = append(nodeDown, evaluation)
		case evaluation.Result.Scope == "app-3" && evaluation.Result.State == healthcheck.StateUnknown:
			app3Unknowns = append(app3Unknowns, evaluation)
		}
	}
	require.Len(t, nodeDown, 1)
	assert.Equal(t, "app-3", nodeDown[0].Result.Scope)
	require.NotEmpty(t, app3Unknowns, "the down node's per-node rules give unknown before reconcile")

	store := healthcheck.NewMemoryStore()
	reconciler := healthcheck.NewReconciler(healthcheck.ReconcilerOpts{Store: store, Now: func() time.Time { return now }})
	_, err := reconciler.Reconcile(evaluations)
	require.NoError(t, err)

	findings, err := store.List(model.HealthFindingFilter{Muted: model.MutedIncluded})
	require.NoError(t, err)
	var app3 []string
	for _, finding := range findings {
		if finding.Scope == "app-3" {
			app3 = append(app3, finding.Code+":"+finding.State)
		}
	}
	assert.Equal(t, []string{"NODE_NOT_RESPONDING:firing"}, app3)
}
