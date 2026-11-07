// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package packet

import (
	"archive/zip"
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
	_ "github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck/rules"
)

const (
	pushSubject    = "EmailSettings.PushNotificationServer"
	siteURLSubject = "ServiceSettings.SiteURL"

	// The fixtures run 11.0, which has ended.
	versionSubject = "11.0"

	complianceDirSubject      = "ComplianceSettings.Directory"
	expiresAtSubject          = "license.expires_at"
	filestoreDiskSubject      = "FileSettings.Directory"
	globalRelayTimeoutSubject = "MessageExportSettings.GlobalRelaySettings.SMTPServerTimeout"
	seatsSubject              = "license.users"
)

type finding struct {
	Code    string
	State   healthcheck.State
	Subject string
	Scope   string
}

func fixtureFiles(t *testing.T, name string) map[string][]byte {
	t.Helper()

	dir := filepath.Join("testdata", name)
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)], err = os.ReadFile(path)
		return err
	})
	require.NoError(t, err)
	require.NotEmpty(t, files)

	return files
}

func zipFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		require.NoError(t, err)
		_, err = w.Write(files[name])
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	return buf.Bytes()
}

func readFiles(t *testing.T, files map[string][]byte) *Packet {
	t.Helper()

	data := zipFiles(t, files)
	p, err := Read(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.NotNil(t, p)

	return p
}

func evaluate(t *testing.T, snapshot *healthcheck.Snapshot) []finding {
	t.Helper()

	collectedAt := snapshot.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)
	}
	now := func() time.Time { return collectedAt }

	registry := healthcheck.Builtin()
	store := healthcheck.NewMemoryStore()
	engine := healthcheck.NewEngine(healthcheck.EngineOpts{Registry: registry, Now: now})
	reconciler := healthcheck.NewReconciler(healthcheck.ReconcilerOpts{Store: store, Registry: registry, Now: now})

	_, err := reconciler.Reconcile(engine.Evaluate(snapshot))
	require.NoError(t, err)

	stored, err := store.List(model.HealthFindingFilter{Muted: model.MutedIncluded})
	require.NoError(t, err)

	findings := make([]finding, 0, len(stored))
	for _, f := range stored {
		findings = append(findings, finding{Code: f.Code, State: healthcheck.State(f.State), Subject: f.Subject, Scope: f.Scope})
	}
	return findings
}

func TestReadGoldenPackets(t *testing.T) {
	cases := []struct {
		name     string
		expected []finding
	}{
		{
			name: "standalone",
			expected: []finding{
				{Code: "COMPLIANCE_NO_DIR", State: healthcheck.StateFiring, Subject: complianceDirSubject},
				{Code: "DB_POSTS_DEAD_TUPLES", State: healthcheck.StateFiring, Subject: "Database.PostsDeadTuples"},
				{Code: "JOB_FAILED", State: healthcheck.StateFiring, Subject: model.JobTypeLdapSync},
				{Code: "LDAP_PROBE_FAILED", State: healthcheck.StateUnknown, Subject: "LdapSettings.LdapServer"},
				{Code: "LDAP_RECOMMENDED_AT_SCALE", State: healthcheck.StateFiring, Subject: "LdapSettings.Enable"},
				{Code: "LDAP_SYNC_NO_BASEDN", State: healthcheck.StateFiring, Subject: "LdapSettings.BaseDN"},
				{Code: "LICENSE_EXPIRED", State: healthcheck.StateUnknown, Subject: expiresAtSubject},
				{Code: "LICENSE_EXPIRING", State: healthcheck.StateUnknown, Subject: expiresAtSubject},
				{Code: "METRICS_OFF", State: healthcheck.StateFiring, Subject: "MetricsSettings.Enable"},
				{Code: "NODE_GOMAXPROCS_ABOVE_CPU_LIMIT", State: healthcheck.StateUnknown, Subject: "Server.GoMaxProcs", Scope: "mm.example.com"},
				{Code: "NODE_HOST_LOAD_HIGH", State: healthcheck.StateUnknown, Subject: "Server.LoadAverage15m", Scope: "mm.example.com"},
				{Code: "PLUGIN_NOT_RUNNING", State: healthcheck.StateFiring, Subject: "PluginSettings.PluginStates"},
				{Code: "PUSH_ID_ONLY", State: healthcheck.StateFiring, Subject: "EmailSettings.PushNotificationContents"},
				{Code: "PUSH_TEST_PROXY", State: healthcheck.StateFiring, Subject: pushSubject},
				{Code: "SEATS_LIMIT_REACHED", State: healthcheck.StateUnknown, Subject: seatsSubject},
				{Code: "SESSION_EXTEND_OFF", State: healthcheck.StateFiring, Subject: "ServiceSettings.ExtendSessionLengthWithActivity"},
				{Code: "SITE_URL_HTTP", State: healthcheck.StateFiring, Subject: siteURLSubject},
				{Code: "VERSION_EOL", State: healthcheck.StateFiring, Subject: versionSubject},
			},
		},
		{
			name: "ha",
			expected: []finding{
				{Code: "DB_HA_FAILOVER_STORM_RISK", State: healthcheck.StateFiring, Subject: "SqlSettings.MaxOpenConns"},
				{Code: "DB_POSTS_DEAD_TUPLES", State: healthcheck.StateUnknown, Subject: "Database.PostsDeadTuples"},
				{Code: "ES_LIVE_BATCH_SYNC", State: healthcheck.StateFiring, Subject: "ElasticsearchSettings.LiveIndexingBatchSize"},
				{Code: "EXPORT_GR_TIMEOUT_LOW", State: healthcheck.StateFiring, Subject: globalRelayTimeoutSubject},
				{Code: "FILESTORE_DISK_FULL", State: healthcheck.StateFiring, Subject: filestoreDiskSubject, Scope: "app-3.example.com"},
				{Code: "FILESTORE_DISK_LOW", State: healthcheck.StateFiring, Subject: filestoreDiskSubject, Scope: "app-1.example.com"},
				{Code: "FILESTORE_LOCAL_IN_CLUSTER", State: healthcheck.StateFiring, Subject: "FileSettings.Directory"},
				{Code: "JOB_STUCK", State: healthcheck.StateFiring, Subject: model.JobTypeMessageExport},
				{Code: "JOB_WEDGED_AT_ZERO", State: healthcheck.StateFiring, Subject: model.JobTypeMessageExport},
				{Code: "LICENSE_EXPIRING", State: healthcheck.StateFiring, Subject: expiresAtSubject},
				{Code: "NODE_GOMAXPROCS_ABOVE_CPU_LIMIT", State: healthcheck.StateUnknown, Subject: "Server.GoMaxProcs", Scope: "app-1.example.com"},
				{Code: "NODE_GOMAXPROCS_ABOVE_CPU_LIMIT", State: healthcheck.StateUnknown, Subject: "Server.GoMaxProcs", Scope: "app-2.example.com"},
				{Code: "NODE_GOMAXPROCS_ABOVE_CPU_LIMIT", State: healthcheck.StateUnknown, Subject: "Server.GoMaxProcs", Scope: "app-3.example.com"},
				{Code: "NODE_HOST_LOAD_HIGH", State: healthcheck.StateUnknown, Subject: "Server.LoadAverage15m", Scope: "app-1.example.com"},
				{Code: "NODE_HOST_LOAD_HIGH", State: healthcheck.StateUnknown, Subject: "Server.LoadAverage15m", Scope: "app-2.example.com"},
				{Code: "NODE_HOST_LOAD_HIGH", State: healthcheck.StateUnknown, Subject: "Server.LoadAverage15m", Scope: "app-3.example.com"},
				{Code: "OAUTH_PROBE_FAILED", State: healthcheck.StateUnknown, Subject: model.ServiceGitlab},
				{Code: "OAUTH_PROBE_FAILED", State: healthcheck.StateUnknown, Subject: model.ServiceGoogle},
				{Code: "OAUTH_PROBE_FAILED", State: healthcheck.StateUnknown, Subject: model.ServiceOffice365},
				{Code: "OAUTH_PROBE_FAILED", State: healthcheck.StateUnknown, Subject: model.ServiceOpenid},
				{Code: "PUSH_BAD_SCHEME", State: healthcheck.StateFiring, Subject: pushSubject},
				{Code: "PUSH_ID_ONLY", State: healthcheck.StateFiring, Subject: "EmailSettings.PushNotificationContents"},
				{Code: "SAML_ENCRYPT_OFF", State: healthcheck.StateFiring, Subject: "SamlSettings.Encrypt"},
				{Code: "VERSION_EOL", State: healthcheck.StateFiring, Subject: versionSubject},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Sorted so parallel rule PRs insert at different lines instead of all appending.
			require.True(t, slices.IsSortedFunc(tc.expected, func(a, b finding) int { return cmp.Compare(a.Code, b.Code) }), "keep expected findings sorted by code")
			p := readFiles(t, fixtureFiles(t, tc.name))

			assert.Empty(t, p.Warnings)
			assert.ElementsMatch(t, tc.expected, evaluate(t, p.Snapshot))
		})
	}
}

func TestReadPostsVacuumInProgress(t *testing.T) {
	files := fixtureFiles(t, "standalone")
	files[model.SupportPacketDiagnosticsFileName] = bytes.Replace(
		files[model.SupportPacketDiagnosticsFileName],
		[]byte("  posts_vacuum_in_progress: false\n"),
		[]byte("  posts_vacuum_in_progress: true\n"),
		1,
	)

	snapshot := readFiles(t, files).Snapshot
	diag, ok := snapshot.LeaderDiag(model.SectionDatabaseStats)
	require.True(t, ok)
	require.NotNil(t, diag.Database.PostsVacuumInProgress)
	assert.True(t, *diag.Database.PostsVacuumInProgress)

	assert.Contains(t, evaluate(t, snapshot), finding{
		Code:    "DB_POSTS_DEAD_TUPLES",
		State:   healthcheck.StateUnknown,
		Subject: "Database.PostsDeadTuples",
	})
}

func TestReadSnapshotFields(t *testing.T) {
	p := readFiles(t, fixtureFiles(t, "standalone"))
	s := p.Snapshot

	assert.Empty(t, p.Warnings)
	assert.Equal(t, time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC), s.CollectedAt)
	assert.Equal(t, "11.0.4", s.Version.Current)
	assert.Empty(t, s.Version.Latest)
	assert.False(t, s.IsCloud())

	seats, ok := s.LicenseSeats()
	require.True(t, ok)
	assert.Equal(t, 500, seats)
	_, ok = s.LicenseExpiresAt()
	assert.False(t, ok)

	license, ok := s.License()
	require.True(t, ok)
	assert.Equal(t, model.LicenseShortSkuEnterprise, license.SkuShortName)
	assert.Equal(t, 500, license.Users)
	idLoaded, ok := s.LicenseFeature(healthcheck.FeatureIDLoaded)
	require.True(t, ok)
	assert.True(t, idLoaded)

	for _, section := range []model.WorkspaceSection{model.SectionConfig, model.SectionStats, model.SectionJobs, model.SectionPlugins} {
		assert.True(t, s.Has(section), "section %q", section)
	}
	present, _ := s.SectionErr(model.SectionVersion)
	assert.False(t, present)

	dsn, ok := s.DataSource()
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(dsn, "postgres://****:****@db.example.com"))

	users, ok := s.Stat(func(stats *model.SupportPacketStats) *int64 { return stats.RegisteredUsers })
	require.True(t, ok)
	assert.Equal(t, int64(312), users)

	enabled, ok := s.PluginEnabled("com.mattermost.calls")
	require.True(t, ok)
	assert.True(t, enabled)

	jobs, ok := s.JobsFor(model.JobTypeDataRetention)
	require.True(t, ok)
	assert.Empty(t, jobs)

	nodes := s.Nodes()
	require.Len(t, nodes, 1)
	assert.Equal(t, "mm.example.com", nodes[0].Hostname)
	assert.True(t, nodes[0].IsLeader)
	assert.Nil(t, nodes[0].ClusterInfo)
	for _, section := range model.AllNodeSections() {
		assert.True(t, nodes[0].Has(section), "section %q", section)
	}
	version, ok := nodes[0].NodeVersion()
	require.True(t, ok)
	assert.Equal(t, "11.0.4", version)
}

func TestReadHALayout(t *testing.T) {
	files := fixtureFiles(t, "ha")
	for name := range files {
		base := filepath.Base(name)
		if !strings.Contains(name, "/") {
			require.NotEqual(t, model.SupportPacketDiagnosticsFileName, base)
			require.NotEqual(t, model.SupportPacketConfigFileName, base)
		}
	}

	s := readFiles(t, files).Snapshot

	nodes := s.Nodes()
	require.Len(t, nodes, 3)
	for _, node := range nodes {
		require.NotNil(t, node.Diagnostics, node.Hostname)
		for _, section := range model.AllNodeSections() {
			assert.True(t, node.Has(section), "node %q section %q", node.Hostname, section)
		}
	}

	require.True(t, s.Has(model.SectionConfig))
	siteURL, ok := s.ConfigString(func(cfg *model.Config) *string { return cfg.ServiceSettings.SiteURL })
	require.True(t, ok)
	assert.Equal(t, "https://chat.example.com", siteURL)
	assert.True(t, s.Has(model.SectionStats))
}

func TestReadLeader(t *testing.T) {
	const unknownLeaderWarning = "The packet does not record which node is the leader"
	leaderOf := func(t *testing.T, files map[string][]byte) (string, []string) {
		t.Helper()

		p := readFiles(t, files)
		s := p.Snapshot
		leader, ok := s.Leader()
		require.True(t, ok)

		leaders := 0
		for _, node := range s.Nodes() {
			if node.IsLeader {
				leaders++
			}
		}
		assert.Equal(t, 1, leaders)

		return leader.Hostname, p.Warnings
	}

	t.Run("the node that reports is_leader leads", func(t *testing.T) {
		leader, warnings := leaderOf(t, fixtureFiles(t, "ha"))
		assert.Equal(t, "app-2.example.com", leader)
		assert.NotContains(t, strings.Join(warnings, "\n"), unknownLeaderWarning)
	})

	t.Run("an older packet without is_leader falls back to the first hostname", func(t *testing.T) {
		files := fixtureFiles(t, "ha")
		name := "app-2.example.com/" + model.SupportPacketDiagnosticsFileName
		require.Contains(t, string(files[name]), "  is_leader: true\n")
		files[name] = bytes.Replace(files[name], []byte("  is_leader: true\n"), nil, 1)

		leader, warnings := leaderOf(t, files)
		assert.Equal(t, "app-1.example.com", leader)
		assert.Contains(t, warnings, "The packet does not record which node is the leader, so the configuration was read from app-1.example.com. Configuration findings may not match the leader's configuration.")
	})

	t.Run("a standalone packet's only node leads", func(t *testing.T) {
		leader, warnings := leaderOf(t, fixtureFiles(t, "standalone"))
		assert.Equal(t, "mm.example.com", leader)
		assert.NotContains(t, strings.Join(warnings, "\n"), unknownLeaderWarning)
	})
}

func TestReadMissingMetadata(t *testing.T) {
	files := fixtureFiles(t, "standalone")
	delete(files, model.PacketMetadataFileName)

	p := readFiles(t, files)
	assert.Len(t, p.Warnings, 1)
	assert.True(t, p.Snapshot.CollectedAt.IsZero())
	assert.Empty(t, p.Snapshot.Version.Current)
}

func TestReadCloud(t *testing.T) {
	files := fixtureFiles(t, "standalone")
	files[model.SupportPacketDiagnosticsFileName] = bytes.Replace(files[model.SupportPacketDiagnosticsFileName], []byte("license:\n"), []byte("license:\n  is_cloud: true\n"), 1)

	snapshot := readFiles(t, files).Snapshot
	assert.True(t, snapshot.IsCloud())

	var codes []string
	for _, f := range evaluate(t, snapshot) {
		codes = append(codes, f.Code)
	}
	assert.ElementsMatch(t, []string{
		"LDAP_PROBE_FAILED",
		"LDAP_RECOMMENDED_AT_SCALE",
		"LDAP_SYNC_NO_BASEDN",
		"PLUGIN_NOT_RUNNING",
		"PUSH_ID_ONLY",
		"PUSH_TEST_PROXY",
	}, codes)
}

func TestReadNodeFileDescriptors(t *testing.T) {
	files := fixtureFiles(t, "ha")
	name := "app-2.example.com/" + model.SupportPacketDiagnosticsFileName
	require.Contains(t, string(files[name]), "open_file_descriptors: 120 ")
	files[name] = bytes.Replace(files[name], []byte("open_file_descriptors: 120 "), []byte("open_file_descriptors: 62260 "), 1)

	var fdFindings []finding
	for _, f := range evaluate(t, readFiles(t, files).Snapshot) {
		if f.Code == "NODE_FD_EXHAUSTION" {
			fdFindings = append(fdFindings, f)
		}
	}
	assert.Equal(t, []finding{{Code: "NODE_FD_EXHAUSTION", State: healthcheck.StateFiring, Subject: "Server.FileDescriptors", Scope: "app-2.example.com"}}, fdFindings)
}

func TestReadLicense(t *testing.T) {
	t.Run("maps the license summary", func(t *testing.T) {
		files := fixtureFiles(t, "standalone")
		files[model.SupportPacketDiagnosticsFileName] = bytes.Replace(files[model.SupportPacketDiagnosticsFileName],
			[]byte("  sku_short_name: enterprise\n"),
			[]byte("  sku_short_name: enterprise\n  is_trial: true\n  expires_at: 1792058400000\n  is_seat_count_enforced: true\n  extra_users: 5\n"), 1)

		license, ok := readFiles(t, files).Snapshot.License()
		require.True(t, ok)
		assert.Equal(t, 500, license.Users)
		assert.Equal(t, model.LicenseShortSkuEnterprise, license.SkuShortName)
		assert.True(t, license.IsTrial)
		assert.Equal(t, int64(1792058400000), license.ExpiresAt)
		assert.True(t, license.IsSeatCountEnforced)
		assert.Equal(t, new(5), license.ExtraUsers)
	})

	t.Run("an older packet has no expiry but keeps its seats", func(t *testing.T) {
		s := readFiles(t, fixtureFiles(t, "standalone")).Snapshot

		license, ok := s.License()
		require.True(t, ok)
		assert.False(t, license.IsSeatCountEnforced)
		assert.Nil(t, license.ExtraUsers)
		_, ok = s.LicenseExpiresAt()
		assert.False(t, ok)
		seats, ok := s.LicenseSeats()
		require.True(t, ok)
		assert.Equal(t, 500, seats)
	})

	t.Run("the leader's summary is read", func(t *testing.T) {
		s := readFiles(t, fixtureFiles(t, "ha")).Snapshot

		license, ok := s.License()
		require.True(t, ok)
		assert.True(t, license.IsSeatCountEnforced)
		expiresAt, ok := s.LicenseExpiresAt()
		require.True(t, ok)
		assert.Equal(t, time.Date(2026, time.October, 15, 10, 0, 0, 0, time.UTC), expiresAt)
	})
}

func TestReadMissingStats(t *testing.T) {
	files := fixtureFiles(t, "standalone")
	delete(files, model.SupportPacketStatsFileName)

	s := readFiles(t, files).Snapshot

	present, err := s.SectionErr(model.SectionStats)
	assert.False(t, present)
	assert.NoError(t, err)
	_, ok := s.Stat(func(stats *model.SupportPacketStats) *int64 { return stats.RegisteredUsers })
	assert.False(t, ok)

	findings := evaluate(t, s)
	assert.Contains(t, findings, finding{Code: "SITE_URL_HTTP", State: healthcheck.StateFiring, Subject: siteURLSubject})
	assert.Contains(t, findings, finding{Code: "PUSH_TEST_PROXY", State: healthcheck.StateFiring, Subject: pushSubject})
}

func TestReadUnparsableStats(t *testing.T) {
	files := fixtureFiles(t, "standalone")
	files[model.SupportPacketStatsFileName] = []byte("registered_users: [not a number\n")

	s := readFiles(t, files).Snapshot

	present, err := s.SectionErr(model.SectionStats)
	assert.True(t, present)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse stats.yaml")
	assert.Nil(t, s.Stats)
	assert.True(t, s.Has(model.SectionConfig))
}

func TestReadNewerDiagnosticsVersion(t *testing.T) {
	files := fixtureFiles(t, "ha")
	name := "app-2.example.com/" + model.SupportPacketDiagnosticsFileName
	files[name] = bytes.Replace(files[name], []byte("version: 2\n"), []byte("version: 3\nfuture_section:\n  value: 1\n"), 1)

	p := readFiles(t, files)

	require.Len(t, p.Warnings, 1)
	assert.Contains(t, p.Warnings[0], "newer server")
	assert.Contains(t, evaluate(t, p.Snapshot), finding{Code: "PUSH_BAD_SCHEME", State: healthcheck.StateFiring, Subject: pushSubject})
}

func TestReadUnparsableNodeDiagnostics(t *testing.T) {
	files := fixtureFiles(t, "ha")
	files["app-3.example.com/"+model.SupportPacketDiagnosticsFileName] = []byte("server: [unclosed\n")

	p := readFiles(t, files)

	require.Len(t, p.Warnings, 1)
	assert.Contains(t, p.Warnings[0], "app-3.example.com/diagnostics.yaml")

	nodes := p.Snapshot.Nodes()
	require.Len(t, nodes, 3)
	for _, node := range nodes {
		if node.Hostname == "app-3.example.com" {
			assert.Nil(t, node.Diagnostics)
		} else {
			assert.NotNil(t, node.Diagnostics, node.Hostname)
		}
	}

	leader, ok := p.Snapshot.Leader()
	require.True(t, ok)
	assert.Equal(t, "app-2.example.com", leader.Hostname)
	assert.Contains(t, evaluate(t, p.Snapshot), finding{Code: "PUSH_BAD_SCHEME", State: healthcheck.StateFiring, Subject: pushSubject})
}

func TestReadGenerationErrors(t *testing.T) {
	files := fixtureFiles(t, "standalone")
	files[model.SupportPacketErrorFile] = []byte("1 error occurred:\n\t* failed to get plugin list for Support Packet\n\n")

	p := readFiles(t, files)

	require.Len(t, p.Warnings, 1)
	assert.Equal(t, "The server reported errors while generating the packet, so some data may be missing: 1 error occurred:\n\t* failed to get plugin list for Support Packet", p.Warnings[0])
	assert.True(t, p.Snapshot.Has(model.SectionPlugins))
}

func TestReadInvalidPackets(t *testing.T) {
	t.Run("not a zip", func(t *testing.T) {
		data := []byte("definitely not a zip")
		_, err := Read(bytes.NewReader(data), int64(len(data)))
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to open the Support Packet")
	})

	t.Run("zip without diagnostics.yaml", func(t *testing.T) {
		files := fixtureFiles(t, "standalone")
		delete(files, model.SupportPacketDiagnosticsFileName)
		data := zipFiles(t, files)

		_, err := Read(bytes.NewReader(data), int64(len(data)))
		require.Error(t, err)
		assert.ErrorContains(t, err, "no diagnostics.yaml found")
	})

	t.Run("truncated zip never panics", func(t *testing.T) {
		data := zipFiles(t, fixtureFiles(t, "ha"))

		for size := range data {
			require.NotPanics(t, func() {
				p, err := Read(bytes.NewReader(data[:size]), int64(size))
				if err == nil {
					require.NotNil(t, p)
				}
			}, "size %d", size)
		}
	})

	t.Run("corrupt member data is a section error", func(t *testing.T) {
		data := zipFiles(t, fixtureFiles(t, "standalone"))
		offset := bytes.Index(data, []byte("registered_users"))
		require.Positive(t, offset)
		data[offset] ^= 0xff

		p, err := Read(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		present, sectionErr := p.Snapshot.SectionErr(model.SectionStats)
		assert.True(t, present)
		assert.Error(t, sectionErr)
	})

	t.Run("oversized member is a section error", func(t *testing.T) {
		files := fixtureFiles(t, "standalone")
		files[model.SupportPacketStatsFileName] = make([]byte, maxMemberSize+1)

		s := readFiles(t, files).Snapshot
		present, err := s.SectionErr(model.SectionStats)
		assert.True(t, present)
		assert.ErrorContains(t, err, "stats.yaml: larger than 64 MiB")
		assert.Nil(t, s.Stats)
	})

	t.Run("too many nodes", func(t *testing.T) {
		standalone := fixtureFiles(t, "standalone")
		files := map[string][]byte{}
		for i := range maxNodes + 1 {
			node := fmt.Sprintf("app-%d", i)
			files[path.Join(node, model.SupportPacketDiagnosticsFileName)] = standalone[model.SupportPacketDiagnosticsFileName]
			files[path.Join(node, model.SupportPacketConfigFileName)] = standalone[model.SupportPacketConfigFileName]
		}
		data := zipFiles(t, files)

		_, err := Read(bytes.NewReader(data), int64(len(data)))
		assert.ErrorContains(t, err, "the Support Packet has 101 nodes, more than the 100")
	})

	t.Run("reads past the total budget are errors", func(t *testing.T) {
		files := fixtureFiles(t, "ha")
		data := zipFiles(t, files)
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)

		budget := len(files["app-1.example.com/diagnostics.yaml"]) + len(files["app-2.example.com/diagnostics.yaml"])
		p, err := readPacket(&budgetedZip{zr: zr, remaining: int64(budget)})
		require.NoError(t, err)

		hostnames := make([]string, 0, len(p.Snapshot.Nodes()))
		for _, node := range p.Snapshot.Nodes() {
			if _, ok := node.Diag(); ok {
				hostnames = append(hostnames, node.Hostname)
			}
		}
		assert.Equal(t, []string{"app-1.example.com", "app-2.example.com"}, hostnames)
		assert.Contains(t, p.Warnings, "failed to read app-3.example.com/diagnostics.yaml: the packet exceeds the total read limit. The node's diagnostics are ignored.")

		present, err := p.Snapshot.SectionErr(model.SectionStats)
		assert.True(t, present)
		assert.ErrorContains(t, err, "total read limit")
	})

	t.Run("oversized members count against the total budget", func(t *testing.T) {
		data := zipFiles(t, map[string][]byte{
			"big":   make([]byte, maxMemberSize+1),
			"small": []byte("small"),
		})
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		z := &budgetedZip{zr: zr, remaining: maxMemberSize + 3}

		_, _, err = z.readRaw("big")
		assert.ErrorContains(t, err, "larger than 64 MiB")

		_, _, err = z.readRaw("small")
		assert.ErrorContains(t, err, "total read limit")
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := ReadFile(filepath.Join(t.TempDir(), "missing.zip"))
		require.Error(t, err)
	})
}

func TestReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "packet.zip")
	require.NoError(t, os.WriteFile(path, zipFiles(t, fixtureFiles(t, "standalone")), 0o600))

	p, err := ReadFile(path)
	require.NoError(t, err)
	assert.Len(t, p.Snapshot.Nodes(), 1)
}

func TestSplitNodeFiles(t *testing.T) {
	root, byNode := splitNodeFiles([]string{
		"metadata.yaml",
		"stats.yaml",
		"app-1/diagnostics.yaml",
		"app-1/sanitized_config.json",
		"app-1/advancedLogs/audit.log",
		"app-2/diagnostics.yaml",
		"app-2/sanitized_config.json",
		"com.mattermost.calls/diagnostics.yaml",
	})

	assert.ElementsMatch(t, []string{"metadata.yaml", "stats.yaml", "com.mattermost.calls/diagnostics.yaml"}, root)
	assert.Equal(t, map[string][]string{
		"app-1": {"diagnostics.yaml", "sanitized_config.json", "advancedLogs/audit.log"},
		"app-2": {"diagnostics.yaml", "sanitized_config.json"},
	}, byNode)
}

// The reader ignores unknown keys, so a fixture key the writer renamed or dropped would
// silently decode to its zero value.
func TestFixturesMatchTheModel(t *testing.T) {
	strictYAML := func(data []byte, v any) error {
		return yaml.UnmarshalWithOptions(data, v, yaml.Strict())
	}
	strictJSON := func(data []byte, v any) error {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		return decoder.Decode(v)
	}

	decoders := map[string]func([]byte) error{
		model.SupportPacketDiagnosticsFileName: func(data []byte) error {
			return strictYAML(data, &model.SupportPacketDiagnostics{})
		},
		model.SupportPacketConfigFileName: func(data []byte) error {
			return strictJSON(data, &model.SupportPacketConfig{})
		},
		model.SupportPacketStatsFileName: func(data []byte) error {
			return strictYAML(data, &model.SupportPacketStats{})
		},
		model.SupportPacketJobsFileName: func(data []byte) error {
			return strictYAML(data, &model.SupportPacketJobList{})
		},
		model.SupportPacketPluginsFileName: func(data []byte) error {
			return strictJSON(data, &model.SupportPacketPluginList{})
		},
		model.PacketMetadataFileName: func(data []byte) error {
			return strictYAML(data, &model.PacketMetadata{})
		},
	}

	for _, fixture := range []string{"standalone", "ha"} {
		files := fixtureFiles(t, fixture)
		root, byNode := splitNodeFiles(slices.Collect(maps.Keys(files)))

		members := map[string]string{}
		for _, name := range root {
			members[name] = name
		}
		for node, names := range byNode {
			for _, name := range names {
				members[path.Join(node, name)] = name
			}
		}

		for member, name := range members {
			decode, ok := decoders[name]
			if !ok {
				// Plugin diagnostics are free-form, so there is no model to check them against.
				continue
			}
			t.Run(fixture+"/"+member, func(t *testing.T) {
				assert.NoError(t, decode(files[member]))
			})
		}
	}
}

// The reader runs inside mmctl, so it must not pull in the server.
func TestImportsStayOutOfTheServer(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", ".").Output()
	require.NoError(t, err)

	allowed := []string{
		"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck",
		"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck/packet",
	}
	deps := strings.Fields(string(out))
	require.Contains(t, deps, allowed[0])
	for _, dep := range deps {
		if strings.HasPrefix(dep, "github.com/mattermost/mattermost/server/v8") {
			assert.Contains(t, allowed, dep)
		}
	}
}
