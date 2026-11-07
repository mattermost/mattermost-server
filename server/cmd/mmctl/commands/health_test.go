// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/cmd/mmctl/printer"
)

func healthTestFindings(lastSeen time.Time) []*model.HealthFinding {
	return []*model.HealthFinding{
		{
			Fingerprint: "00000000000000000000000000000001",
			Code:        "PUSH_BAD_SCHEME",
			Severity:    "critical",
			State:       "firing",
			Area:        model.AreaNotifications,
			Title:       "Push notification server is not HTTPS",
			Message:     "The push server is http://push.example.com, so notifications are sent unencrypted.",
			Remediation: "Set Push Notification Server to an https:// URL.",
			LastSeenAt:  lastSeen.UnixMilli(),
		},
		{
			Fingerprint: "00000000000000000000000000000002",
			Code:        "SITE_URL_HTTP",
			Severity:    "warning",
			State:       "unknown",
			Area:        model.AreaCluster,
			Title:       "Site URL is not HTTPS",
			Message:     "Config section unavailable.",
			Remediation: "Serve via HTTPS and update SiteURL accordingly.",
			LastSeenAt:  lastSeen.UnixMilli(),
		},
		{
			Fingerprint: "00000000000000000000000000000003",
			Code:        "SITE_URL_EMPTY",
			Severity:    "critical",
			State:       "resolved",
			Area:        model.AreaCluster,
			Title:       "Site URL is empty",
			Remediation: "Set SiteURL to the public HTTPS URL of the server.",
			LastSeenAt:  lastSeen.UnixMilli(),
		},
	}
}

func healthTestList(lastSeen time.Time) *model.HealthFindingList {
	return &model.HealthFindingList{EvaluatedAt: lastSeen.UnixMilli(), Findings: healthTestFindings(lastSeen)}
}

func healthTestCommand(includeResolved, includeMuted bool) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("include-resolved", includeResolved, "")
	cmd.Flags().Bool("include-muted", includeMuted, "")
	return cmd
}

func (s *MmctlUnitTestSuite) printedHealthFindings() string {
	s.Require().Len(printer.GetLines(), 1)
	output, ok := printer.GetLines()[0].(string)
	s.Require().True(ok)
	return output
}

func (s *MmctlUnitTestSuite) TestHealthCheckCmd() {
	lastSeen := time.Now().Add(-5 * time.Minute)

	s.Run("default view shows firing and unknown findings only", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		s.client.
			EXPECT().
			GetHealthFindings(context.TODO(), model.HealthFindingFilter{}).
			Return(healthTestList(lastSeen), &model.Response{}, nil).
			Times(1)

		err := healthCheckCmdF(s.client, healthTestCommand(false, false), []string{})
		s.Require().NoError(err)

		output := s.printedHealthFindings()
		s.Contains(output, "Push notification server is not HTTPS")
		s.Contains(output, "Site URL is not HTTPS")
		s.NotContains(output, "Site URL is empty")
		s.NotContains(output, "Resolved")
	})

	s.Run("--include-resolved adds resolved findings", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		s.client.
			EXPECT().
			GetHealthFindings(context.TODO(), model.HealthFindingFilter{}).
			Return(healthTestList(lastSeen), &model.Response{}, nil).
			Times(1)

		err := healthCheckCmdF(s.client, healthTestCommand(true, false), []string{})
		s.Require().NoError(err)

		output := s.printedHealthFindings()
		s.Contains(output, "Push notification server is not HTTPS")
		s.Contains(output, "Site URL is not HTTPS")
		s.Contains(output, "Resolved\n  Site URL is empty")
	})

	s.Run("--include-muted asks the server for muted findings", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		s.client.
			EXPECT().
			GetHealthFindings(context.TODO(), model.HealthFindingFilter{Muted: model.MutedIncluded}).
			Return(&model.HealthFindingList{Findings: []*model.HealthFinding{}}, &model.Response{}, nil).
			Times(1)

		err := healthCheckCmdF(s.client, healthTestCommand(false, true), []string{})
		s.Require().NoError(err)
	})

	s.Run("--json prints the same rows as the table", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatJSON)
		list := healthTestList(lastSeen)
		findings := list.Findings

		s.client.
			EXPECT().
			GetHealthFindings(context.TODO(), model.HealthFindingFilter{}).
			Return(list, &model.Response{}, nil).
			Times(1)

		err := healthCheckCmdF(s.client, healthTestCommand(false, false), []string{})
		s.Require().NoError(err)

		s.Require().Len(printer.GetLines(), 1)
		s.Equal(&model.HealthFindingList{EvaluatedAt: lastSeen.UnixMilli(), Findings: []*model.HealthFinding{findings[0], findings[1]}}, printer.GetLines()[0])
	})

	s.Run("--json with --include-resolved prints every row", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatJSON)
		list := healthTestList(lastSeen)

		s.client.
			EXPECT().
			GetHealthFindings(context.TODO(), model.HealthFindingFilter{}).
			Return(list, &model.Response{}, nil).
			Times(1)

		err := healthCheckCmdF(s.client, healthTestCommand(true, false), []string{})
		s.Require().NoError(err)

		s.Require().Len(printer.GetLines(), 1)
		s.Equal(list, printer.GetLines()[0])
	})

	s.Run("every finding muted still reports the evaluation", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		s.client.
			EXPECT().
			GetHealthFindings(context.TODO(), model.HealthFindingFilter{}).
			Return(&model.HealthFindingList{EvaluatedAt: lastSeen.UnixMilli(), Findings: []*model.HealthFinding{}}, &model.Response{}, nil).
			Times(1)

		err := healthCheckCmdF(s.client, healthTestCommand(false, false), []string{})
		s.Require().NoError(err)

		output := s.printedHealthFindings()
		s.True(strings.HasPrefix(output, "Last evaluated "), output)
		s.True(strings.HasSuffix(output, "\n\nNo findings."), output)
	})

	s.Run("client error", func() {
		printer.Clean()

		s.client.
			EXPECT().
			GetHealthFindings(context.TODO(), model.HealthFindingFilter{}).
			Return(nil, &model.Response{}, errors.New("mock error")).
			Times(1)

		err := healthCheckCmdF(s.client, healthTestCommand(false, false), []string{})
		s.Require().EqualError(err, "failed to get health findings: mock error")
		s.Empty(printer.GetLines())
	})
}

func (s *MmctlUnitTestSuite) TestHealthPrintFindings() {
	lastSeen := time.Date(2026, time.September, 24, 14, 2, 0, 0, time.Local)
	now := lastSeen.Add(38 * time.Minute)

	s.Run("plain output", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		printHealthFindings(healthTestList(lastSeen), false, now)

		s.Equal(`Last evaluated 2026-09-24 14:02 (38 minutes ago)

Notifications
  CRITICAL  Push notification server is not HTTPS
            The push server is http://push.example.com, so notifications are sent unencrypted.
            → Set Push Notification Server to an https:// URL.

Could not evaluate
  WARNING   Site URL is not HTTPS
            Config section unavailable.

1 critical, 1 could not be evaluated`, s.printedHealthFindings())
	})

	s.Run("plain output has no code, state or fingerprint", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)
		findings := healthTestFindings(lastSeen)

		printHealthFindings(&model.HealthFindingList{EvaluatedAt: lastSeen.UnixMilli(), Findings: findings}, true, now)

		output := s.printedHealthFindings()
		for _, finding := range findings {
			s.NotContains(output, finding.Code)
			s.NotContains(output, finding.Fingerprint)
		}
		for _, state := range []string{"firing", "unknown", "resolved"} {
			s.NotContains(output, state)
		}
	})

	s.Run("groups firing findings by area, sorted by severity then title", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)
		findings := []*model.HealthFinding{
			{Severity: "warning", State: "firing", Area: model.AreaNotifications, Title: "B warning", LastSeenAt: lastSeen.UnixMilli()},
			{Severity: "critical", State: "firing", Area: model.AreaNotifications, Title: "Z critical", LastSeenAt: lastSeen.UnixMilli()},
			{Severity: "warning", State: "firing", Area: model.AreaNotifications, Title: "A warning", LastSeenAt: lastSeen.UnixMilli()},
			{Severity: "info", State: "firing", Area: model.AreaCluster, Title: "Node info", Scope: "node-1", MutedAt: 1, LastSeenAt: lastSeen.UnixMilli()},
		}

		printHealthFindings(&model.HealthFindingList{EvaluatedAt: lastSeen.UnixMilli(), Findings: findings}, false, now)

		s.Equal(`Last evaluated 2026-09-24 14:02 (38 minutes ago)

Cluster
  INFO      Node info on node-1 [muted]

Notifications
  CRITICAL  Z critical
  WARNING   A warning
  WARNING   B warning

1 critical, 2 warnings, 1 info`, s.printedHealthFindings())
	})

	s.Run("stored findings with nothing firing or unknown", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		printHealthFindings(&model.HealthFindingList{EvaluatedAt: lastSeen.UnixMilli(), Findings: healthTestFindings(lastSeen)[2:]}, false, now)

		s.Equal("Last evaluated 2026-09-24 14:02 (38 minutes ago)\n\nNo findings.", s.printedHealthFindings())
	})

	s.Run("never evaluated", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		printHealthFindings(&model.HealthFindingList{Findings: []*model.HealthFinding{}}, false, now)

		s.Equal("Not evaluated yet. The first check runs within an hour of enabling the feature.", s.printedHealthFindings())
	})

	s.Run("evaluation time comes from the server, not the shown findings", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)
		list := &model.HealthFindingList{EvaluatedAt: lastSeen.UnixMilli(), Findings: healthTestFindings(lastSeen.Add(-time.Hour))}

		printHealthFindings(list, false, now)

		s.Contains(s.printedHealthFindings(), "Last evaluated 2026-09-24 14:02 (38 minutes ago)\n")
	})

	s.Run("evaluation older than two hours appears stopped", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		printHealthFindings(healthTestList(lastSeen), false, lastSeen.Add(2*time.Hour+time.Minute))

		s.Contains(s.printedHealthFindings(), "Last evaluated 2026-09-24 14:02 (121 minutes ago). Evaluation appears to have stopped.\n")
	})

	s.Run("evaluation two hours old is not reported as stopped", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		printHealthFindings(healthTestList(lastSeen), false, lastSeen.Add(2*time.Hour))

		s.NotContains(s.printedHealthFindings(), "Evaluation appears to have stopped")
	})

	s.Run("json output when never evaluated has an empty list", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatJSON)

		printHealthFindings(&model.HealthFindingList{Findings: []*model.HealthFinding{}}, false, now)

		s.Require().Len(printer.GetLines(), 1)
		s.Equal(&model.HealthFindingList{Findings: []*model.HealthFinding{}}, printer.GetLines()[0])
	})
}

const healthTestPacketGeneratedAt = 1790330400000

func healthTestPacketFiles() map[string]string {
	return map[string]string{
		model.PacketMetadataFileName: "version: 1\ntype: support-packet\ngenerated_at: 1790330400000\nserver_version: 11.0.4\n",
		model.SupportPacketDiagnosticsFileName: `version: 2
license:
  users: 500
  sku_short_name: enterprise
  expires_at: 1821866400000
  features:
    metrics: true
    compliance: true
    data_retention: true
    message_export: true
server:
  os: linux
  hostname: mm.example.com
  open_file_descriptors: 120
  max_file_descriptors: 65536
  version: 11.0.4
database:
  type: postgres
  version: "15.8"
  master_connections_in_use: 1
  posts_dead_tuples: 0
  posts_live_tuples: 1000
  posts_vacuum_in_progress: false
  posts_last_vacuum: 2026-09-25T09:00:00Z
elastic:
  status: disabled
file_store:
  file_status: OK
  file_driver: local
  total_mb: 102400
  available_mb: 51200
notifications:
  email:
    status: OK
`,
		model.SupportPacketStatsFileName:   "registered_users: 100\nactive_users: 100\nsingle_channel_guests: 0\nposts: 1000\n",
		model.SupportPacketJobsFileName:    "data_retention_jobs: []\n",
		model.SupportPacketConfigFileName:  healthTestPacketConfig(),
		model.SupportPacketPluginsFileName: `{"enabled": [], "disabled": []}`,
	}
}

// healthTestPacketConfig starts from defaults so new rules see a realistic config without editing this fixture.
func healthTestPacketConfig() string {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.ServiceSettings.SiteURL = model.NewPointer("https://chat.example.com")
	cfg.GuestAccountsSettings.Enable = model.NewPointer(true)
	cfg.EmailSettings.SendPushNotifications = model.NewPointer(true)
	cfg.EmailSettings.PushNotificationServer = model.NewPointer("http://push.example.com")
	cfg.EmailSettings.PushNotificationContents = model.NewPointer(model.IdLoadedNotification)
	cfg.SqlSettings.DataSource = model.NewPointer("postgres://****:****@db.example.com:5432/mattermost")
	cfg.RateLimitSettings.Enable = model.NewPointer(true)
	cfg.ExperimentalAuditSettings.FileEnabled = model.NewPointer(true)
	cfg.MetricsSettings.Enable = model.NewPointer(true)

	b, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (s *MmctlUnitTestSuite) writeHealthTestPacket(files map[string]string) string {
	path := filepath.Join(s.T().TempDir(), "packet.zip")
	f, err := os.Create(path)
	s.Require().NoError(err)
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, body := range files {
		w, createErr := zw.Create(name)
		s.Require().NoError(createErr)
		_, err = w.Write([]byte(body))
		s.Require().NoError(err)
	}
	s.Require().NoError(zw.Close())

	return path
}

func healthPacketTestCommand(path string, includeResolved, includeMuted bool) *cobra.Command {
	cmd := healthTestCommand(includeResolved, includeMuted)
	cmd.Flags().String("packet", path, "")
	return cmd
}

func (s *MmctlUnitTestSuite) TestHealthCheckPacket() {
	packetPath := s.writeHealthTestPacket(healthTestPacketFiles())
	evaluatedAt := time.UnixMilli(healthTestPacketGeneratedAt).Format("2006-01-02 15:04")
	packetWithFilestoreDisk := func(totalMB, availableMB uint64) string {
		files := healthTestPacketFiles()
		files[model.SupportPacketDiagnosticsFileName] = strings.Replace(
			files[model.SupportPacketDiagnosticsFileName],
			"  total_mb: 102400\n  available_mb: 51200\n",
			"  total_mb: "+strconv.FormatUint(totalMB, 10)+"\n  available_mb: "+strconv.FormatUint(availableMB, 10)+"\n",
			1,
		)
		return s.writeHealthTestPacket(files)
	}

	s.Run("runs with no server configured", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)
		viper.Set("config", filepath.Join(s.T().TempDir(), "missing-config"))
		viper.Set("local-socket-path", filepath.Join(s.T().TempDir(), "missing.socket"))
		s.T().Cleanup(func() {
			viper.Set("config", "")
			viper.Set("local-socket-path", model.LocalModeSocketPath)
		})

		err := healthCheckRunE(healthTestCommand(false, false), []string{})
		s.Require().ErrorContains(err, "failed to create client")

		err = healthCheckRunE(healthPacketTestCommand(packetPath, false, false), []string{})
		s.Require().NoError(err)
		s.Contains(s.printedHealthFindings(), "PushNotificationServer does not use https://")
	})

	s.Run("plain output opens with the disclaimer and uses the packet's time", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		err := healthCheckPacketCmdF(healthPacketTestCommand(packetPath, false, false), []string{})
		s.Require().NoError(err)

		s.Equal(healthPacketDisclaimer+`

Last evaluated `+evaluatedAt+` (0 minutes ago)

Notifications
  CRITICAL  PushNotificationServer does not use https://
            PushNotificationServer http://push.example.com uses http://, so push notifications are sent in cleartext.
            → Prefix PushNotificationServer with https:// and verify reachability from the app node.

Version
  CRITICAL  Mattermost version is past end-of-life
            Running Mattermost 11.0.4. Support for this release line ended on 2026-01-15. No security patches ship for this release line. Upgrade to the latest release.
            → Plan a staged upgrade to a supported Extended Support Release or the latest release. Supported versions are listed at https://docs.mattermost.com/product-overview/mattermost-server-releases.html.

Could not evaluate
  WARNING   GOMAXPROCS exceeds the container CPU limit on mm.example.com
            Server diagnostics unavailable.
  WARNING   Host memory is running low on mm.example.com
            Server diagnostics unavailable.
  INFO      Host load is high on mm.example.com
            Server diagnostics unavailable.

2 critical, 3 could not be evaluated`, s.printedHealthFindings())
	})

	s.Run("plain output localizes a full filestore disk finding", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		err := healthCheckPacketCmdF(healthPacketTestCommand(packetWithFilestoreDisk(102400, 4096), false, false), []string{})
		s.Require().NoError(err)

		output := s.printedHealthFindings()
		s.Contains(output, "CRITICAL  File storage disk is nearly full on mm.example.com")
		s.Contains(output, "The volume containing FileSettings.Directory has 4.0 GiB (4.00%) available of 100.0 GiB. File uploads and other file operations can fail when the volume fills.")
		s.Contains(output, "Immediately expand the volume or remove unrelated data from its filesystem; do not manually delete files managed by Mattermost from FileSettings.Directory. Keep at least 10% or 10 GiB available, and never less than 1 GiB.")
	})

	s.Run("plain output localizes a low filestore disk finding", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		err := healthCheckPacketCmdF(healthPacketTestCommand(packetWithFilestoreDisk(102400, 9216), false, false), []string{})
		s.Require().NoError(err)

		output := s.printedHealthFindings()
		s.Contains(output, "WARNING   File storage disk space is low on mm.example.com")
		s.Contains(output, "The volume containing FileSettings.Directory has 9.0 GiB (9.00%) available of 100.0 GiB.")
		s.Contains(output, "Expand the volume or remove unrelated data from its filesystem; do not manually delete files managed by Mattermost from FileSettings.Directory. Keep at least 10% or 10 GiB available, and never less than 1 GiB.")
	})

	s.Run("an old packet is not reported as stopped evaluation", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		err := healthCheckPacketCmdF(healthPacketTestCommand(packetPath, false, false), []string{})
		s.Require().NoError(err)
		s.NotContains(s.printedHealthFindings(), "Evaluation appears to have stopped")
	})

	s.Run("--include-resolved finds nothing resolved in a single evaluation", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)

		err := healthCheckPacketCmdF(healthPacketTestCommand(packetPath, true, false), []string{})
		s.Require().NoError(err)
		output := s.printedHealthFindings()
		s.Contains(output, "PushNotificationServer does not use https://")
		s.NotContains(output, "\nResolved\n")
	})

	s.Run("--include-muted is rejected", func() {
		printer.Clean()

		err := healthCheckRunE(healthPacketTestCommand(packetPath, false, true), []string{})
		s.Require().ErrorContains(err, "--include-muted cannot be used with --packet")
		s.Empty(printer.GetLines())
	})

	s.Run("--json prints findings and sends the notes to stderr", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatJSON)

		err := healthCheckPacketCmdF(healthPacketTestCommand(packetPath, false, false), []string{})
		s.Require().NoError(err)

		s.Require().Len(printer.GetLines(), 1)
		list, ok := printer.GetLines()[0].(*model.HealthFindingList)
		s.Require().True(ok)
		s.Equal(int64(healthTestPacketGeneratedAt), list.EvaluatedAt)
		findings := list.Findings
		s.Require().Len(findings, 5)
		codes := make([]string, 0, len(findings))
		for _, finding := range findings {
			codes = append(codes, finding.Code)
		}
		s.ElementsMatch([]string{
			"VERSION_EOL",
			"PUSH_BAD_SCHEME",
			"NODE_HOST_MEMORY_LOW",
			"NODE_GOMAXPROCS_ABOVE_CPU_LIMIT",
			"NODE_HOST_LOAD_HIGH",
		}, codes)

		pushIndex := slices.IndexFunc(findings, func(finding *model.HealthFinding) bool { return finding.Code == "PUSH_BAD_SCHEME" })
		s.Require().NotEqual(-1, pushIndex)
		s.Equal("PushNotificationServer does not use https://", findings[pushIndex].Title)
		s.Equal(time.UnixMilli(healthTestPacketGeneratedAt).UnixMilli(), findings[pushIndex].FirstSeenAt)
		s.Equal([]string{healthPacketDisclaimer}, printer.GetErrorLines())
	})

	s.Run("renders a Posts dead tuples finding after a stale vacuum", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatJSON)
		files := healthTestPacketFiles()
		files[model.SupportPacketDiagnosticsFileName] = strings.Replace(
			files[model.SupportPacketDiagnosticsFileName],
			"  posts_dead_tuples: 0\n",
			"  posts_dead_tuples: 200001\n",
			1,
		)
		files[model.SupportPacketDiagnosticsFileName] = strings.Replace(
			files[model.SupportPacketDiagnosticsFileName],
			"  posts_last_vacuum: 2026-09-25T09:00:00Z\n",
			"  posts_last_vacuum: 2026-09-10T10:00:00Z\n",
			1,
		)
		files[model.SupportPacketDiagnosticsFileName] = strings.Replace(
			files[model.SupportPacketDiagnosticsFileName],
			"  posts_live_tuples: 1000\n",
			"  posts_live_tuples: 500000\n",
			1,
		)

		cmd := newTestCmd(s.T(), HealthCheckCmd)
		s.Require().NoError(cmd.Flags().Set("packet", s.writeHealthTestPacket(files)))
		s.Require().NoError(healthCheckRunE(cmd, []string{}))
		s.Require().Len(printer.GetLines(), 1)
		list, ok := printer.GetLines()[0].(*model.HealthFindingList)
		s.Require().True(ok)

		index := slices.IndexFunc(list.Findings, func(f *model.HealthFinding) bool {
			return f.Code == "DB_POSTS_DEAD_TUPLES"
		})
		s.Require().NotEqual(-1, index)
		finding := list.Findings[index]
		s.Equal("Posts table has many dead tuples", finding.Title)
		s.Equal("The Posts table has 200001 estimated dead tuples (40.0% of an estimated 500000 live rows), and its most recent vacuum was 15 days ago at 2026-09-10T10:00:00Z. Autovacuum does not appear to be keeping up with this table. Table and index bloat can degrade query performance and increase disk use.", finding.Message)
		s.Contains(finding.Remediation, "verify autovacuum is enabled")
	})

	s.Run("renders a Posts dead tuples finding after a recent vacuum", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatJSON)
		files := healthTestPacketFiles()
		files[model.SupportPacketDiagnosticsFileName] = strings.Replace(
			files[model.SupportPacketDiagnosticsFileName],
			"  posts_dead_tuples: 0\n",
			"  posts_dead_tuples: 200001\n",
			1,
		)
		files[model.SupportPacketDiagnosticsFileName] = strings.Replace(
			files[model.SupportPacketDiagnosticsFileName],
			"  posts_live_tuples: 1000\n",
			"  posts_live_tuples: 500000\n",
			1,
		)

		cmd := newTestCmd(s.T(), HealthCheckCmd)
		s.Require().NoError(cmd.Flags().Set("packet", s.writeHealthTestPacket(files)))
		s.Require().NoError(healthCheckRunE(cmd, []string{}))
		s.Require().Len(printer.GetLines(), 1)
		list, ok := printer.GetLines()[0].(*model.HealthFindingList)
		s.Require().True(ok)

		index := slices.IndexFunc(list.Findings, func(f *model.HealthFinding) bool {
			return f.Code == "DB_POSTS_DEAD_TUPLES"
		})
		s.Require().NotEqual(-1, index)
		finding := list.Findings[index]
		s.Equal("The Posts table has 200001 estimated dead tuples (40.0% of an estimated 500000 live rows), even though it was last vacuumed at 2026-09-25T09:00:00Z. Vacuum is running but most likely cannot remove them because something is holding back cleanup, such as a long-running or idle-in-transaction session, an inactive replication slot, or an abandoned prepared transaction. Table and index bloat can degrade query performance and increase disk use.", finding.Message)
		s.Contains(finding.Remediation, "pg_replication_slots")
	})

	s.Run("audit logging is judged by the server that generated the packet, not this machine", func() {
		var cfg model.Config
		s.Require().NoError(json.Unmarshal([]byte(healthTestPacketConfig()), &cfg))
		cfg.ExperimentalAuditSettings.FileEnabled = model.NewPointer(false)
		noAuditTargets, err := json.Marshal(cfg)
		s.Require().NoError(err)

		reportsAuditLogOff := func(serverAuditActive bool) bool {
			files := healthTestPacketFiles()
			files[model.SupportPacketDiagnosticsFileName] += "config:\n  audit_logging_configured: " + strconv.FormatBool(serverAuditActive) + "\n"
			files[model.SupportPacketStatsFileName] = "registered_users: 6000\n"
			files[model.SupportPacketConfigFileName] = string(noAuditTargets)

			printer.Clean()
			printer.SetFormat(printer.FormatJSON)
			cmd := newTestCmd(s.T(), HealthCheckCmd)
			s.Require().NoError(cmd.Flags().Set("packet", s.writeHealthTestPacket(files)))
			s.Require().NoError(healthCheckRunE(cmd, []string{}))
			s.Require().Len(printer.GetLines(), 1)
			list, ok := printer.GetLines()[0].(*model.HealthFindingList)
			s.Require().True(ok)
			return slices.ContainsFunc(list.Findings, func(f *model.HealthFinding) bool { return f.Code == "AUDIT_LOG_OFF" })
		}

		s.True(reportsAuditLogOff(false))
		s.False(reportsAuditLogOff(true), "the server audits through targets its config doesn't show")

		s.T().Setenv("MM_EXPERIMENTALAUDITSETTINGS_ADDITIONAL", `{"audit": {"type": "file", "format": "json", "levels": [{"id": 100, "name": "audit-api"}], "options": {"filename": "audit.log"}}}`)
		s.True(reportsAuditLogOff(false), "audit targets on the evaluating machine say nothing about the server")
	})

	s.Run("packet warnings follow the disclaimer and contain no em dash", func() {
		printer.Clean()
		printer.SetFormat(printer.FormatPlain)
		files := healthTestPacketFiles()
		delete(files, model.PacketMetadataFileName)
		files[model.SupportPacketDiagnosticsFileName] = "version: 99\n"

		err := healthCheckPacketCmdF(healthPacketTestCommand(s.writeHealthTestPacket(files), true, false), []string{})
		s.Require().NoError(err)

		output := s.printedHealthFindings()
		s.True(strings.HasPrefix(output, healthPacketDisclaimer+"\n\n"))
		s.Contains(output, "newer server")
		s.Contains(output, "no metadata.yaml")
		s.NotContains(output, "—")
	})

	s.Run("an unreadable packet is an error", func() {
		printer.Clean()

		err := healthCheckPacketCmdF(healthPacketTestCommand(filepath.Join(s.T().TempDir(), "missing.zip"), false, false), []string{})
		s.Require().ErrorContains(err, "failed to read Support Packet")
		s.Empty(printer.GetLines())
	})
}
