// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	rpprof "runtime/pprof"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/utils"
	"github.com/mattermost/mattermost/server/v8/config"
	"github.com/mattermost/mattermost/server/v8/platform/shared/mail"
)

const (
	envVarInstallType = "MM_INSTALL_TYPE"
	unknownDataPoint  = "unknown"
)

// diagnosticsYAMLComments annotates non-obvious fields in diagnostics.yaml
// with inline (and a few group-header) comments so support engineers reading
// the file under triage conditions have units, omitempty semantics, and
// cumulative-vs-point-in-time semantics visible at a glance.
var diagnosticsYAMLComments = yaml.CommentMap{
	// server: — grouped into Machine / Capacity / Process lifecycle / Software
	"$.server.os":                              {yaml.HeadComment(" Machine")},
	"$.server.cpu_cores":                       {yaml.HeadComment(" Capacity (hardware → effective quota)"), yaml.LineComment(" logical CPUs visible to the OS")},
	"$.server.total_memory_mb":                 {yaml.LineComment(" host/VM total RAM; may exceed container limit")},
	"$.server.available_memory_mb":             {yaml.LineComment(" host/VM MemAvailable from /proc/meminfo; Linux only")},
	"$.server.swap_total_mb":                   {yaml.LineComment(" host/VM total swap; Linux only")},
	"$.server.swap_used_mb":                    {yaml.LineComment(" host/VM used swap; Linux only")},
	"$.server.container_cpu_limit":             {yaml.LineComment(" cgroup v2 CPU quota in CPUs; Linux only, omitted if no limit set")},
	"$.server.container_memory_limit_mb":       {yaml.LineComment(" cgroup v2 memory quota in MB; Linux only, omitted if no limit set")},
	"$.server.container_memory_usage_mb":       {yaml.LineComment(" cgroup v2 memory.current in MB, including page cache; Linux only")},
	"$.server.container_memory_working_set_mb": {yaml.LineComment(" cgroup v2 memory.current minus inactive file cache in MB; Linux only")},
	"$.server.container_oom_kills":             {yaml.LineComment(" cgroup v2 OOM kills since the cgroup was created; Linux only")},
	"$.server.load_average_1m":                 {yaml.LineComment(" host-wide runnable and uninterruptible tasks averaged over 1 minute; Linux only")},
	"$.server.load_average_5m":                 {yaml.LineComment(" host-wide runnable and uninterruptible tasks averaged over 5 minutes; Linux only")},
	"$.server.load_average_15m":                {yaml.LineComment(" host-wide runnable and uninterruptible tasks averaged over 15 minutes; Linux only")},
	"$.server.container_cpu_periods":           {yaml.LineComment(" cgroup v2 CPU scheduling periods since the cgroup was created; Linux only")},
	"$.server.container_cpu_throttled_periods": {yaml.LineComment(" throttled periods divided by total periods is the CPU throttling ratio")},
	"$.server.container_cpu_throttled_seconds": {yaml.LineComment(" total cgroup v2 CPU throttling time since the cgroup was created")},
	"$.server.go_max_procs":                    {yaml.LineComment(" maximum CPUs Go can execute simultaneously")},
	"$.server.process_id":                      {yaml.HeadComment(" Process lifecycle")},
	"$.server.started_at":                      {yaml.LineComment(" when Mattermost process started")},
	"$.server.host_started_at":                 {yaml.LineComment(" when the host OS booted; omitted if unavailable")},
	"$.server.process_rss_mb":                  {yaml.LineComment(" resident memory of the Mattermost process only; excludes plugin child processes")},
	"$.server.go_memory_limit_mb":              {yaml.LineComment(" Go runtime memory limit in MB; omitted when no limit is set")},
	"$.server.open_file_descriptors":           {yaml.LineComment(" current open FDs for this process")},
	"$.server.max_file_descriptors":            {yaml.LineComment(" system limit (ulimit -n)")},
	"$.server.version":                         {yaml.HeadComment(" Software")},

	"$.config.audit_logging_configured": {yaml.LineComment(" this server's audit check of its current config and license; offline health checks use it")},

	// database: sql.DBStats cumulative counters (lifetime of process; all drivers)
	"$.database.master_pool_wait_count":                  {yaml.LineComment(" cumulative; total times a goroutine waited for a connection since process start")},
	"$.database.master_pool_wait_duration_ms":            {yaml.LineComment(" cumulative wait time across all goroutines since process start")},
	"$.database.master_connections_closed_max_idle":      {yaml.LineComment(" cumulative; connections closed because the idle pool was full")},
	"$.database.master_connections_closed_max_lifetime":  {yaml.LineComment(" cumulative; connections closed for exceeding ConnMaxLifetime")},
	"$.database.replica_pool_wait_count":                 {yaml.LineComment(" cumulative across all replicas; see master_pool_wait_count")},
	"$.database.replica_pool_wait_duration_ms":           {yaml.LineComment(" cumulative across all replicas")},
	"$.database.replica_connections_closed_max_idle":     {yaml.LineComment(" cumulative across all replicas")},
	"$.database.replica_connections_closed_max_lifetime": {yaml.LineComment(" cumulative across all replicas")},

	// database: PostgreSQL-only fields (omitted on MySQL)
	"$.database.cache_hit_ratio":                {yaml.HeadComment(" PostgreSQL-only (these fields are omitted on MySQL)"), yaml.LineComment(" blks_hit / (blks_hit + blks_read) from pg_stat_database; cumulative since stats reset")},
	"$.database.deadlocks":                      {yaml.LineComment(" cumulative since pg_stat_database reset")},
	"$.database.temp_files":                     {yaml.LineComment(" cumulative count of temp files created since stats reset")},
	"$.database.temp_bytes_mb":                  {yaml.LineComment(" cumulative bytes written to temp files, in MB")},
	"$.database.rollbacks":                      {yaml.LineComment(" cumulative transaction rollbacks since stats reset")},
	"$.database.idle_in_transaction_count":      {yaml.LineComment(" point-in-time count from pg_stat_activity")},
	"$.database.longest_query_duration_seconds": {yaml.LineComment(" point-in-time; max age of any active query right now")},
	"$.database.waiting_for_lock_count":         {yaml.LineComment(" point-in-time count of backends waiting on a Lock wait_event_type")},
	"$.database.posts_dead_tuples":              {yaml.LineComment(" n_dead_tup for the posts table from pg_stat_user_tables")},
	"$.database.posts_live_tuples":              {yaml.LineComment(" n_live_tup for the posts table from pg_stat_user_tables; an estimate")},
	"$.database.posts_vacuum_in_progress":       {yaml.LineComment(" whether a VACUUM, VACUUM FULL or CLUSTER of posts is running")},
	"$.database.posts_last_vacuum":              {yaml.LineComment(" last manual vacuum on posts; null if never manually vacuumed (then omitted)")},
	"$.database.posts_last_autovacuum":          {yaml.LineComment(" last autovacuum on posts; null if never autovacuumed (then omitted)")},

	// file_store: local driver only fields
	"$.file_store.filesystem_type": {yaml.LineComment(" local driver only (e.g. ext4, xfs); omitted for s3 and other remote drivers")},
	"$.file_store.total_mb":        {yaml.LineComment(" local driver only; capacity of the volume hosting FileSettings.Directory")},
	"$.file_store.available_mb":    {yaml.LineComment(" local driver only; free space remaining on that volume")},
}

func (ps *PlatformService) GenerateSupportPacket(rctx request.CTX, options *model.SupportPacketOptions) ([]model.FileData, error) {
	functions := map[string]func(request.CTX) (*model.FileData, error){
		"diagnostics": func(rctx request.CTX) (*model.FileData, error) {
			nodeDiagnostics, err := ps.GetSupportPacketDiagnostics(rctx)
			return supportPacketDiagnosticsFile(nodeDiagnostics.Diagnostics, err)
		},
		"config": func(rctx request.CTX) (*model.FileData, error) {
			return supportPacketConfigFile(ps.GetSupportPacketConfig(rctx))
		},
		"heap profile": ps.getHeapProfile,
		"goroutines":   ps.getGoroutineProfile,
	}

	cpuProfileDur := cpuProfileDuration
	if options != nil && options.CPUProfileDuration != nil {
		cpuProfileDur = *options.CPUProfileDuration
	}
	if cpuProfileDur > 0 {
		functions["cpu profile"] = func(rctx request.CTX) (*model.FileData, error) {
			return ps.getCPUProfile(rctx, cpuProfileDur)
		}
	}

	if options != nil && options.IncludeLogs {
		functions["mattermost log"] = ps.GetLogFile
	}

	var (
		fileDatas []model.FileData
		rErr      *multierror.Error
	)

	for name, fn := range functions {
		fileData, err := fn(rctx)
		if err != nil {
			rctx.Logger().Error("Failed to generate file for Support Packet",
				mlog.String("file", name),
				mlog.Err(err),
			)
			rErr = multierror.Append(rErr, err)
		}

		if fileData != nil {
			fileDatas = append(fileDatas, *fileData)
		}
	}

	if options != nil && options.IncludeLogs {
		advancedLogs, err := ps.GetAdvancedLogs(rctx)
		if err != nil {
			rctx.Logger().Error("Failed to read advanced log files for Support Packet", mlog.Err(err))
			rErr = multierror.Append(rErr, err)
		}

		for _, log := range advancedLogs {
			fileDatas = append(fileDatas, *log)
		}
	}

	return fileDatas, rErr.ErrorOrNil()
}

func supportPacketDiagnosticsFile(d *model.SupportPacketDiagnostics, err error) (*model.FileData, error) {
	return YAMLFile(model.SupportPacketDiagnosticsFileName, d, err, yaml.WithComment(diagnosticsYAMLComments))
}

func supportPacketConfigFile(c *model.SupportPacketConfig, err error) (*model.FileData, error) {
	return JSONFile(model.SupportPacketConfigFileName, c, err)
}

// GetSupportPacketDiagnostics collects this node's diagnostics. Every node section is
// recorded in Errors; a section is set to an error if any of its collection steps failed.
// The returned error aggregates all failures.
func (ps *PlatformService) GetSupportPacketDiagnostics(rctx request.CTX) (*model.NodeDiagnostics, error) {
	var (
		rErr *multierror.Error
		err  error
		d    model.SupportPacketDiagnostics
	)

	sectionErrors := make(model.SectionErrors, len(model.AllNodeSections()))
	for _, section := range model.AllNodeSections() {
		sectionErrors[section] = nil
	}
	fail := func(section model.NodeSection, failure error) {
		rErr = multierror.Append(rErr, failure)
		sectionErrors[section] = multierror.Append(sectionErrors[section], failure)
	}
	// Start the network probes first so that they overlap with the local collection below.
	collectProbes := startNetworkProbes(rctx.Context(), ps.networkProbes(rctx, &d), fail)

	d.Version = model.CurrentSupportPacketVersion

	/* License */
	if license := ps.License(); license != nil {
		d.License.Company = license.Customer.Company
		d.License.Users = model.SafeDereference(license.Features.Users)
		d.License.SkuShortName = license.SkuShortName
		d.License.IsTrial = license.IsTrialLicense()
		d.License.IsGovSKU = license.IsGovSku
		d.License.IsNonProduction = license.IsNonProduction
		d.License.IsCloud = license.IsCloud()
		d.License.ExpiresAt = license.ExpiresAt
		d.License.IsSeatCountEnforced = license.IsSeatCountEnforced
		d.License.ExtraUsers = license.ExtraUsers
		d.License.Features = license.Features
	}

	/* Server */
	d.Server.OS = runtime.GOOS
	d.Server.Architecture = runtime.GOARCH
	// Note: These values represent the host machine's resources, not any
	// container limits (e.g., Docker or Kubernetes) that may be in effect.
	d.Server.CPUCores = runtime.NumCPU()
	d.Server.GoMaxProcs = new(runtime.GOMAXPROCS(0))
	totalMemoryBytes, err := getTotalMemory()
	if err != nil {
		fail(model.SectionServerHost, errors.Wrap(err, "error while getting total memory"))
	} else {
		d.Server.TotalMemoryMB = new(totalMemoryBytes / 1024 / 1024)
	}
	cgroupV2, err := getCgroupV2Info()
	if err != nil {
		err = errors.Wrap(err, "error while resolving cgroup v2")
		fail(model.SectionServerMemory, err)
		fail(model.SectionServerCPU, err)
	}
	containerLimits, err := getContainerLimits(cgroupV2)
	d.Server.ContainerCPULimit = containerLimits.CPULimit
	if err != nil {
		fail(model.SectionServerCPU, errors.Wrap(err, "error while getting container limits"))
	}
	memory, memoryErr := getMemoryDiagnostics(cgroupV2)
	if memoryErr != nil {
		fail(model.SectionServerMemory, errors.Wrap(memoryErr, "error while getting memory diagnostics"))
	}
	d.Server.AvailableMemoryMB = memory.AvailableMemoryMB
	d.Server.SwapTotalMB = memory.SwapTotalMB
	d.Server.SwapUsedMB = memory.SwapUsedMB
	d.Server.ProcessRSSMB = memory.ProcessRSSMB
	d.Server.ContainerMemoryLimitMB = memory.ContainerMemoryLimitMB
	d.Server.ContainerMemoryUsageMB = memory.ContainerMemoryUsageMB
	d.Server.ContainerMemoryWorkingSetMB = memory.ContainerMemoryWorkingSetMB
	d.Server.ContainerOOMKills = memory.ContainerOOMKills
	if goMemoryLimit := debug.SetMemoryLimit(-1); goMemoryLimit != math.MaxInt64 {
		goMemoryLimitMB := uint64(0)
		if goMemoryLimit > 0 {
			goMemoryLimitMB = uint64((goMemoryLimit-1)/(1024*1024) + 1)
		}
		d.Server.GoMemoryLimitMB = &goMemoryLimitMB
	}
	cpuStats, err := getCPUStats(cgroupV2)
	if err != nil {
		fail(model.SectionServerCPU, errors.Wrap(err, "error while getting CPU statistics"))
	} else {
		d.Server.LoadAverage1m = cpuStats.LoadAverage1m
		d.Server.LoadAverage5m = cpuStats.LoadAverage5m
		d.Server.LoadAverage15m = cpuStats.LoadAverage15m
		d.Server.ContainerCPUPeriods = cpuStats.ContainerCPUPeriods
		d.Server.ContainerCPUThrottledPeriods = cpuStats.ContainerCPUThrottledPeriods
		d.Server.ContainerCPUThrottledSeconds = cpuStats.ContainerCPUThrottledSeconds
	}
	serverHostname, err := os.Hostname()
	if err != nil {
		fail(model.SectionServerHost, errors.Wrap(err, "error while getting hostname"))
	} else {
		d.Server.Hostname = new(serverHostname)
	}
	d.Server.ProcessID = os.Getpid()
	d.Server.StartedAt = ps.startTime.UTC()
	if hostUptimeSeconds, hostUptimeErr := getHostUptimeSeconds(); hostUptimeErr == nil {
		d.Server.HostStartedAt = time.Now().Add(-time.Duration(hostUptimeSeconds) * time.Second).UTC()
	}
	d.Server.Version = model.CurrentVersion
	d.Server.BuildHash = model.BuildHash
	d.Server.GoVersion = runtime.Version()
	installationType := ps.installTypeOverride
	if installationType == "" {
		installationType = os.Getenv(envVarInstallType)
	}
	if installationType == "" {
		installationType = unknownDataPoint
	}
	d.Server.InstallationType = installationType
	openFileDescriptors, err := getOpenFileDescriptors()
	if err != nil {
		fail(model.SectionServerFDs, errors.Wrap(err, "error while getting open file descriptor count"))
	} else {
		d.Server.OpenFileDescriptors = new(openFileDescriptors)
	}
	maxFileDescriptors, err := getMaxFileDescriptors()
	if err != nil {
		fail(model.SectionServerFDs, errors.Wrap(err, "error while getting max file descriptor limit"))
	} else {
		d.Server.MaxFileDescriptors = new(maxFileDescriptors)
	}

	/* Config */
	d.Config.Source = ps.DescribeConfig()
	d.Config.AuditLoggingConfigured = new(config.IsAuditLoggingActive(ps.Config().ExperimentalAuditSettings, ps.License().HasAdvancedLogging()))

	/* DB */
	databaseType, schemaVersion, err := ps.DatabaseTypeAndSchemaVersion()
	if err != nil {
		fail(model.SectionDatabaseIdentity, errors.Wrap(err, "error while getting DB type and schema version"))
	} else {
		d.Database.Type = new(databaseType)
		d.Database.SchemaVersion = new(schemaVersion)
	}

	databaseVersion, err := ps.Store.GetDbVersion(false)
	if err != nil {
		fail(model.SectionDatabaseIdentity, errors.Wrap(err, "error while getting DB version"))
	} else {
		d.Database.Version = new(databaseVersion)
	}
	d.Database.MasterConnections = ps.Store.TotalMasterDbConnections()
	d.Database.ReplicaConnections = ps.Store.TotalReadDbConnections()
	d.Database.SearchConnections = ps.Store.TotalSearchDbConnections()

	err = ps.applyStoreDiagnostics(rctx, &d)
	if err != nil {
		fail(model.SectionDatabaseStats, err)
	}

	/* File store */
	d.FileStore.Driver = ps.FileBackend().DriverName()
	if d.FileStore.Driver == model.ImageDriverLocal {
		dir := model.SafeDereference(ps.Config().FileSettings.Directory)
		if dir == "" {
			dir = model.FileSettingsDefaultDirectory
		}
		di, diskErr := getDiskInfo(dir)
		if diskErr != nil {
			fail(model.SectionFilestoreDisk, errors.Wrap(diskErr, "error while getting disk space info"))
		} else {
			d.FileStore.FilesystemType = new(di.FilesystemType)
			d.FileStore.TotalMB = new(di.TotalMB)
			d.FileStore.AvailableMB = new(di.AvailableMB)
		}
	}

	/* Websockets */
	d.Websocket.Connections = ps.TotalWebsocketConnections()

	/* Cluster */
	if cluster := ps.Cluster(); cluster != nil {
		d.Cluster.ID = cluster.GetClusterId()
		d.Cluster.IsLeader = cluster.IsLeader()
		clusterInfo, e := cluster.GetClusterInfos()
		if e != nil {
			fail(model.SectionCluster, errors.Wrap(e, "error while getting cluster infos"))
		} else {
			d.Cluster.NumberOfNodes = new(max(len(clusterInfo), 1)) // clusterInfo is empty if the node is the only one in the cluster
			d.Cluster.NumberOfConnectedNodes = new(max(connectedClusterNodes(clusterInfo), 1))
		}
	} else {
		d.Cluster.NumberOfNodes = new(1)
		d.Cluster.NumberOfConnectedNodes = new(1)
	}

	/* SAML */
	if idpDescriptorURL := model.SafeDereference(ps.Config().SamlSettings.IdpDescriptorURL); idpDescriptorURL != "" {
		d.SAML.ProviderType = detectSAMLProviderType(idpDescriptorURL)
	}
	if samlDiagnostic := ps.SamlDiagnostic(); samlDiagnostic != nil && model.SafeDereference(ps.Config().SamlSettings.Enable) {
		if err = samlDiagnostic.RunSupportPacketTest(rctx, ps.Config().SamlSettings); err != nil {
			d.SAML.Status = model.StatusFail
			d.SAML.Error = err.Error()
		} else {
			d.SAML.Status = model.StatusOk
		}
	} else {
		d.SAML.Status = model.StatusDisabled
	}

	/* Elastic Search */
	if se := ps.SearchEngine.ElasticsearchEngine; se != nil {
		d.ElasticSearch.Backend = *ps.Config().ElasticsearchSettings.Backend
		d.ElasticSearch.ServerVersion = se.GetFullVersion()
		d.ElasticSearch.ServerPlugins = se.GetPlugins()
	}

	collectProbes()

	return &model.NodeDiagnostics{Diagnostics: &d, Errors: sectionErrors}, rErr.ErrorOrNil()
}

// networkProbe checks one external service on its own goroutine, so that a service that doesn't answer
// can't hold up the collection.
type networkProbe struct {
	name    string
	section model.NodeSection
	// status and errMsg receive the result, which is OK, or Fail with the service's error.
	status, errMsg *string
	// run checks the service and returns its error, or nil if it is reachable.
	run func(ctx context.Context) error
	// record, if set, stores further results once run has returned, and returns an error that fails
	// the section. It runs on the collecting goroutine, so it can read what run left in its closure.
	record func() error
}

// networkProbes returns the enabled probes, and marks the disabled ones in d.
func (ps *PlatformService) networkProbes(rctx request.CTX, d *model.SupportPacketDiagnostics) []networkProbe {
	cfg := ps.Config()
	var probes []networkProbe
	add := func(enabled bool, probe networkProbe) {
		if !enabled {
			*probe.status = model.StatusDisabled
			return
		}
		probes = append(probes, probe)
	}

	// Takes no context; S3 and Azure are bounded by their own request timeout.
	fileBackend := ps.FileBackend()
	add(true, networkProbe{
		name: "file store", section: model.SectionFilestoreConn, status: &d.FileStore.Status, errMsg: &d.FileStore.Error,
		run: func(context.Context) error { return fileBackend.TestConnection() },
	})

	// Takes no context; LdapSettings.QueryTimeout bounds each LDAP request.
	ldap := ps.LdapDiagnostic()
	var serverName, serverVersion string
	var vendorErr error
	add(ldap != nil && (model.SafeDereference(cfg.LdapSettings.Enable) || model.SafeDereference(cfg.LdapSettings.EnableSync)), networkProbe{
		name: "ldap", section: model.SectionLDAPProbe, status: &d.LDAP.Status, errMsg: &d.LDAP.Error,
		run: func(context.Context) error {
			if appErr := ldap.RunTest(rctx); appErr != nil {
				return appErr
			}
			// Only if the LDAP test was successful, try to get the LDAP server info
			serverName, serverVersion, vendorErr = ldap.GetVendorNameAndVendorVersion(rctx)
			return nil
		},
		record: func() error {
			d.LDAP.ServerName = cmp.Or(serverName, unknownDataPoint)
			d.LDAP.ServerVersion = cmp.Or(serverVersion, unknownDataPoint)
			return errors.Wrap(vendorErr, "error while getting LDAP vendor info")
		},
	})

	se := ps.SearchEngine.ElasticsearchEngine
	add(se != nil && model.SafeDereference(cfg.ElasticsearchSettings.EnableIndexing), networkProbe{
		name: "search", section: model.SectionSearchProbe, status: &d.ElasticSearch.Status, errMsg: &d.ElasticSearch.Error,
		run: func(context.Context) error {
			if appErr := se.TestConfig(rctx, cfg); appErr != nil {
				return appErr
			}
			return nil
		},
	})

	// Invitations, verification and password reset emails go through SMTP even with notification emails off.
	mailCfg := supportPacketSMTPConfig(cfg)
	add(model.SafeDereference(cfg.EmailSettings.SendEmailNotifications) ||
		model.SafeDereference(cfg.EmailSettings.RequireEmailVerification) ||
		model.SafeDereference(cfg.ServiceSettings.EnableEmailInvitations) ||
		model.SafeDereference(cfg.EmailSettings.EnableSignInWithEmail) ||
		model.SafeDereference(cfg.EmailSettings.EnableSignInWithUsername), networkProbe{
		name: "smtp", section: model.SectionSMTPProbe, status: &d.Notifications.Email.Status, errMsg: &d.Notifications.Email.Error,
		run: func(ctx context.Context) error { return mail.TestConnection(ctx, mailCfg) },
	})

	for _, provider := range []struct {
		name   string
		sso    *model.SSOSettings
		status *model.OAuthProviderStatus
	}{
		{"gitlab", &cfg.GitLabSettings, &d.OAuthProviders.GitLab},
		{"google", &cfg.GoogleSettings, &d.OAuthProviders.Google},
		{"office365", cfg.Office365Settings.SSOSettings(), &d.OAuthProviders.Office365},
		{"openid", &cfg.OpenIdSettings, &d.OAuthProviders.OpenID},
	} {
		add(model.SafeDereference(provider.sso.Enable), networkProbe{
			name: provider.name, section: model.SectionOAuthProbes, status: &provider.status.Status, errMsg: &provider.status.Error,
			run: func(ctx context.Context) error { return probeOAuthProvider(ctx, provider.sso) },
		})
	}

	pushServerURL := model.SafeDereference(cfg.EmailSettings.PushNotificationServer)
	add(model.SafeDereference(cfg.EmailSettings.SendPushNotifications), networkProbe{
		name: "push", section: model.SectionPushProbe, status: &d.Notifications.Push.Status, errMsg: &d.Notifications.Push.Error,
		run: func(ctx context.Context) error { return ps.testPushProxyConnection(ctx, pushServerURL) },
	})

	return probes
}

// startNetworkProbes runs the probes concurrently and returns a function that records their results. It
// stops waiting when ctx ends: a probe still running then fails its section and keeps an empty status, so
// readers treat it as unknown rather than failed. The abandoned probe must stop on its own.
func startNetworkProbes(ctx context.Context, probes []networkProbe, fail func(model.NodeSection, error)) (collect func()) {
	type result struct {
		i   int
		err error
		// panicked is set when run panicked instead of returning.
		panicked error
	}
	// Buffered so that an abandoned probe can still send its result and exit.
	results := make(chan result, len(probes))
	for i, probe := range probes {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					results <- result{i: i, panicked: fmt.Errorf("probe panicked: %v", r)}
				}
			}()
			results <- result{i: i, err: probe.run(ctx)}
		}()
	}

	notCompleted := func(probe networkProbe, err error) {
		err = errors.Wrap(err, probe.name)
		fail(probe.section, err)
		*probe.errMsg = err.Error()
	}
	return func() {
		finished := make([]bool, len(probes))
		record := func(r result) {
			finished[r.i] = true
			probe := probes[r.i]
			if r.panicked != nil {
				notCompleted(probe, r.panicked)
				return
			}
			*probe.status = model.StatusOk
			if r.err != nil {
				*probe.status, *probe.errMsg = model.StatusFail, r.err.Error()
			}
			if probe.record != nil {
				if err := probe.record(); err != nil {
					fail(probe.section, err)
				}
			}
		}
		for range probes {
			select {
			case r := <-results:
				record(r)
			case <-ctx.Done():
				// select picks at random between ready cases, so results may still be waiting, for example
				// when the local collection used up the deadline after every probe had finished.
				for len(results) > 0 {
					record(<-results)
				}
				for i, probe := range probes {
					if !finished[i] {
						notCompleted(probe, errors.Wrap(context.Cause(ctx), "probe did not complete"))
					}
				}
				return
			}
		}
	}
}

func supportPacketSMTPConfig(cfg *model.Config) *mail.SMTPConfig {
	emailSettings := cfg.EmailSettings
	return &mail.SMTPConfig{
		Hostname:                          utils.GetHostnameFromSiteURL(model.SafeDereference(cfg.ServiceSettings.SiteURL)),
		ConnectionSecurity:                model.SafeDereference(emailSettings.ConnectionSecurity),
		SkipServerCertificateVerification: model.SafeDereference(emailSettings.SkipServerCertificateVerification),
		ServerName:                        model.SafeDereference(emailSettings.SMTPServer),
		Server:                            model.SafeDereference(emailSettings.SMTPServer),
		Port:                              model.SafeDereference(emailSettings.SMTPPort),
		ServerTimeout:                     model.SafeDereference(emailSettings.SMTPServerTimeout),
		Username:                          model.SafeDereference(emailSettings.SMTPUsername),
		Password:                          model.SafeDereference(emailSettings.SMTPPassword),
		EnableSMTPAuth:                    model.SafeDereference(emailSettings.EnableSMTPAuth),
		SendEmailNotifications:            true,
		FeedbackName:                      model.SafeDereference(emailSettings.FeedbackName),
		FeedbackEmail:                     model.SafeDereference(emailSettings.FeedbackEmail),
		ReplyToAddress:                    model.SafeDereference(emailSettings.ReplyToAddress),
	}
}

func (ps *PlatformService) applyStoreDiagnostics(rctx request.CTX, diagnostics *model.SupportPacketDiagnostics) error {
	storeDiagnostics, err := ps.Store.GetDiagnostics(rctx)
	if storeDiagnostics == nil {
		if err != nil {
			return errors.Wrap(err, "error while collecting support packet database diagnostics")
		}
		return nil
	}

	diagnostics.Database.MasterConnectionsInUse = new(storeDiagnostics.MasterConnectionsInUse)
	diagnostics.Database.MasterConnectionsIdle = new(storeDiagnostics.MasterConnectionsIdle)
	diagnostics.Database.MasterPoolWaitCount = new(storeDiagnostics.MasterPoolWaitCount)
	diagnostics.Database.MasterPoolWaitDurationMs = new(storeDiagnostics.MasterPoolWaitDurationMs)
	diagnostics.Database.MasterConnectionsClosedMaxIdle = new(storeDiagnostics.MasterConnectionsClosedMaxIdle)
	diagnostics.Database.MasterConnectionsClosedMaxLifetime = new(storeDiagnostics.MasterConnectionsClosedMaxLifetime)
	diagnostics.Database.ReplicaConnectionsInUse = new(storeDiagnostics.ReplicaConnectionsInUse)
	diagnostics.Database.ReplicaConnectionsIdle = new(storeDiagnostics.ReplicaConnectionsIdle)
	diagnostics.Database.ReplicaPoolWaitCount = new(storeDiagnostics.ReplicaPoolWaitCount)
	diagnostics.Database.ReplicaPoolWaitDurationMs = new(storeDiagnostics.ReplicaPoolWaitDurationMs)
	diagnostics.Database.ReplicaConnectionsClosedMaxIdle = new(storeDiagnostics.ReplicaConnectionsClosedMaxIdle)
	diagnostics.Database.ReplicaConnectionsClosedMaxLifetime = new(storeDiagnostics.ReplicaConnectionsClosedMaxLifetime)
	diagnostics.Database.ReplicasOnline = new(storeDiagnostics.ReplicasOnline)
	diagnostics.Database.CacheHitRatio = storeDiagnostics.CacheHitRatio
	diagnostics.Database.Deadlocks = storeDiagnostics.Deadlocks
	diagnostics.Database.TempFiles = storeDiagnostics.TempFiles
	diagnostics.Database.TempBytesMB = storeDiagnostics.TempBytesMB
	diagnostics.Database.Rollbacks = storeDiagnostics.Rollbacks
	diagnostics.Database.IdleInTransactionCount = storeDiagnostics.IdleInTransactionCount
	diagnostics.Database.LongestQueryDurationSeconds = storeDiagnostics.LongestQueryDurationSeconds
	diagnostics.Database.WaitingForLockCount = storeDiagnostics.WaitingForLockCount
	diagnostics.Database.PostsDeadTuples = storeDiagnostics.PostsDeadTuples
	diagnostics.Database.PostsLiveTuples = storeDiagnostics.PostsLiveTuples
	diagnostics.Database.PostsVacuumInProgress = storeDiagnostics.PostsVacuumInProgress
	diagnostics.Database.PostsLastVacuum = storeDiagnostics.PostsLastVacuum
	diagnostics.Database.PostsLastAutovacuum = storeDiagnostics.PostsLastAutovacuum

	if err != nil {
		return errors.Wrap(err, "error while collecting support packet database diagnostics")
	}

	return nil
}

// probeOAuthProvider checks connectivity for an OAuth2/OpenID Connect provider.
// If the provider has a DiscoveryEndpoint configured, it issues an HTTP GET to
// that URL and verifies the response is a valid OIDC discovery document.
// Otherwise it probes the TokenEndpoint host: any HTTP response (including
// 4xx/5xx) is treated as reachable, since token endpoints typically reject GETs.
// Callers only probe enabled providers.
func probeOAuthProvider(ctx context.Context, sso *model.SSOSettings) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if discoveryEndpoint := model.SafeDereference(sso.DiscoveryEndpoint); discoveryEndpoint != "" {
		return probeOIDCDiscovery(ctx, discoveryEndpoint)
	}
	if tokenEndpoint := model.SafeDereference(sso.TokenEndpoint); tokenEndpoint != "" {
		return probeOAuthTokenEndpoint(ctx, tokenEndpoint)
	}
	return errors.New("no discovery or token endpoint configured")
}

func probeOIDCDiscovery(ctx context.Context, discoveryURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer drainAndCloseBody(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("discovery endpoint returned unexpected status %d", resp.StatusCode)
	}
	// Cap the discovery document at 1 MiB; real OIDC discovery responses are a few KiB.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return errors.Wrap(err, "failed to read discovery response")
	}
	var doc struct {
		Issuer string `json:"issuer"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return errors.Wrap(err, "discovery endpoint did not return valid JSON")
	}
	if doc.Issuer == "" {
		return fmt.Errorf("discovery endpoint response missing required 'issuer' field")
	}
	return nil
}

func probeOAuthTokenEndpoint(ctx context.Context, tokenURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer drainAndCloseBody(resp.Body)
	return nil
}

// drainAndCloseBody fully reads and discards an HTTP response body (up to 1 MiB
// to bound a misbehaving server) and closes it. Draining before closing allows
// net/http to return the underlying TCP connection to the idle pool for
// keep-alive reuse on subsequent requests.
func drainAndCloseBody(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 1<<20))
	_ = body.Close()
}

// TODO: move this into its own push proxy package once one exists (see also pushNotificationClient in server.go)
func (ps *PlatformService) testPushProxyConnection(ctx context.Context, serverURL string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	versionURL, err := url.JoinPath(serverURL, "version")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer drainAndCloseBody(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("push proxy returned unexpected status %d", resp.StatusCode)
	}
	return nil
}

// GetSupportPacketConfig returns the support packet's sanitized config payload.
func (ps *PlatformService) GetSupportPacketConfig(rctx request.CTX) (*model.SupportPacketConfig, error) {
	config := ps.getSanitizedConfig(rctx, &model.SanitizeOptions{PartiallyRedactDataSources: true})
	spConfig := model.SupportPacketConfig{
		Config:       config,
		FeatureFlags: *config.FeatureFlags,
	}
	return &spConfig, nil
}

func (ps *PlatformService) getCPUProfile(_ request.CTX, duration time.Duration) (*model.FileData, error) {
	var b bytes.Buffer

	err := rpprof.StartCPUProfile(&b)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start CPU profile")
	}

	time.Sleep(duration)

	rpprof.StopCPUProfile()

	fileData := &model.FileData{
		Filename: "cpu.prof",
		Body:     b.Bytes(),
	}
	return fileData, nil
}

func (ps *PlatformService) getHeapProfile(_ request.CTX) (*model.FileData, error) {
	var b bytes.Buffer

	err := rpprof.Lookup("heap").WriteTo(&b, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to lookup heap profile")
	}

	fileData := &model.FileData{
		Filename: "heap.prof",
		Body:     b.Bytes(),
	}
	return fileData, nil
}

func (ps *PlatformService) getGoroutineProfile(_ request.CTX) (*model.FileData, error) {
	var b bytes.Buffer

	err := rpprof.Lookup("goroutine").WriteTo(&b, 2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to lookup goroutine profile")
	}

	fileData := &model.FileData{
		Filename: "goroutines",
		Body:     b.Bytes(),
	}
	return fileData, nil
}

// detectSAMLProviderType attempts to identify the SAML provider type based on the IdpDescriptorURL.
// It returns "unknown" if the provider cannot be identified.
func detectSAMLProviderType(idpDescriptorURL string) string {
	if idpDescriptorURL == "" {
		return unknownDataPoint
	}

	// Normalize URL to lowercase for case-insensitive matching
	normalizedURL := strings.ToLower(idpDescriptorURL)

	// Check for common SAML provider patterns in the EntityID/IdpDescriptorURL
	// Order matters: more specific patterns should come before generic ones
	switch {
	case strings.Contains(normalizedURL, "login.microsoftonline.com") || strings.Contains(normalizedURL, "sts.windows.net"):
		return "Azure AD"
	case strings.Contains(normalizedURL, ".okta.com") || strings.Contains(normalizedURL, ".oktapreview.com"):
		return "Okta"
	case strings.Contains(normalizedURL, ".auth0.com"):
		return "Auth0"
	case strings.Contains(normalizedURL, ".onelogin.com"):
		return "OneLogin"
	case strings.Contains(normalizedURL, "accounts.google.com"):
		return "Google Workspace"
	case strings.Contains(normalizedURL, "sso.jumpcloud.com"):
		return "JumpCloud"
	case strings.Contains(normalizedURL, "duo.com/saml2"):
		return "Duo"
	case strings.Contains(normalizedURL, ".centrify.com"):
		return "Centrify"
	case strings.Contains(normalizedURL, "/realms/"):
		return "Keycloak"
	case strings.Contains(normalizedURL, "/adfs") || strings.Contains(normalizedURL, "/federationmetadata/"):
		return "ADFS"
	case strings.Contains(normalizedURL, "shibboleth.net") || strings.Contains(normalizedURL, "/idp/shibboleth"):
		return "Shibboleth"
	default:
		return unknownDataPoint
	}
}

// YAMLFile marshals a typed support-packet section into FileData. A non-nil v is written even
// when err is set, so a partially failed collector still produces its file.
func YAMLFile[T any](filename string, v *T, err error, opts ...yaml.EncodeOption) (*model.FileData, error) {
	if v == nil {
		return nil, err
	}

	body, marshalErr := yaml.MarshalWithOptions(v, opts...)
	if marshalErr != nil {
		return nil, multierror.Append(err, errors.Wrapf(marshalErr, "failed to marshal %s into yaml", filename))
	}

	return &model.FileData{
		Filename: filename,
		Body:     body,
	}, err
}

// JSONFile marshals a typed support-packet section into FileData using 4-space indentation.
func JSONFile[T any](filename string, v *T, err error) (*model.FileData, error) {
	if v == nil {
		return nil, err
	}

	body, marshalErr := json.MarshalIndent(v, "", "    ")
	if marshalErr != nil {
		return nil, multierror.Append(err, errors.Wrapf(marshalErr, "failed to marshal %s into json", filename))
	}

	return &model.FileData{
		Filename: filename,
		Body:     body,
	}, err
}

// connectedClusterNodes counts the gossip members in infos. GetClusterInfos also returns nodes
// known only from their database registration, and those carry no IP address.
func connectedClusterNodes(infos []*model.ClusterInfo) int {
	connected := 0
	for _, info := range infos {
		if info != nil && info.IPAddress != "" {
			connected++
		}
	}
	return connected
}
