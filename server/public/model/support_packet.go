// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import "time"

const (
	CurrentSupportPacketVersion = 2
	SupportPacketErrorFile      = "warning.txt"

	SupportPacketDiagnosticsFileName = "diagnostics.yaml"
	SupportPacketConfigFileName      = "sanitized_config.json"
	SupportPacketStatsFileName       = "stats.yaml"
	SupportPacketJobsFileName        = "jobs.yaml"
	SupportPacketPluginsFileName     = "plugins.json"
)

type SupportPacketLicense struct {
	Company         string `yaml:"company"`
	Users           int    `yaml:"users"`
	SkuShortName    string `yaml:"sku_short_name"`
	IsTrial         bool   `yaml:"is_trial,omitempty"`
	IsGovSKU        bool   `yaml:"is_gov_sku,omitempty"`
	IsNonProduction bool   `yaml:"is_non_production,omitempty"`
	IsCloud         bool   `yaml:"is_cloud,omitempty"`
	// ExpiresAt is in epoch milliseconds.
	ExpiresAt           int64 `yaml:"expires_at,omitempty"`
	IsSeatCountEnforced bool  `yaml:"is_seat_count_enforced,omitempty"`
	ExtraUsers          *int  `yaml:"extra_users,omitempty"`
	// Features is nil in packets from servers that predate it.
	Features *Features `yaml:"features,omitempty"`
}

func (l *SupportPacketLicense) IsMattermostEntry() bool {
	return l != nil && l.SkuShortName == LicenseShortSkuMattermostEntry
}

// MinimumEnterprise reports whether the license is at least Enterprise, as MinimumEnterpriseLicense does for a License.
func (l *SupportPacketLicense) MinimumEnterprise() bool {
	return l != nil && MinimumEnterpriseLicense(&License{SkuShortName: l.SkuShortName})
}

// SupportPacketDiagnostics reports one node's diagnostics. A nil pointer field was not collected, which is not the same as zero.
type SupportPacketDiagnostics struct {
	Version int `yaml:"version"`

	License SupportPacketLicense `yaml:"license"`

	Server struct {
		// Machine
		OS               string  `yaml:"os"`
		Architecture     string  `yaml:"architecture"`
		Hostname         *string `yaml:"hostname,omitempty"`
		InstallationType string  `yaml:"installation_type"`

		// Capacity
		CPUCores                     int      `yaml:"cpu_cores"`
		TotalMemoryMB                *uint64  `yaml:"total_memory_mb,omitempty"`
		AvailableMemoryMB            *uint64  `yaml:"available_memory_mb,omitempty"`
		SwapTotalMB                  *uint64  `yaml:"swap_total_mb,omitempty"`
		SwapUsedMB                   *uint64  `yaml:"swap_used_mb,omitempty"`
		ContainerCPULimit            float64  `yaml:"container_cpu_limit,omitempty"`
		ContainerMemoryLimitMB       uint64   `yaml:"container_memory_limit_mb,omitempty"`
		ContainerMemoryUsageMB       *uint64  `yaml:"container_memory_usage_mb,omitempty"`
		ContainerMemoryWorkingSetMB  *uint64  `yaml:"container_memory_working_set_mb,omitempty"`
		ContainerOOMKills            *uint64  `yaml:"container_oom_kills,omitempty"`
		LoadAverage1m                *float64 `yaml:"load_average_1m,omitempty"`
		LoadAverage5m                *float64 `yaml:"load_average_5m,omitempty"`
		LoadAverage15m               *float64 `yaml:"load_average_15m,omitempty"`
		ContainerCPUPeriods          *uint64  `yaml:"container_cpu_periods,omitempty"`
		ContainerCPUThrottledPeriods *uint64  `yaml:"container_cpu_throttled_periods,omitempty"`
		ContainerCPUThrottledSeconds *float64 `yaml:"container_cpu_throttled_seconds,omitempty"`
		GoMaxProcs                   *int     `yaml:"go_max_procs,omitempty"`

		// Process lifecycle
		ProcessID           int       `yaml:"process_id"`
		StartedAt           time.Time `yaml:"started_at"`
		HostStartedAt       time.Time `yaml:"host_started_at,omitempty"`
		ProcessRSSMB        *uint64   `yaml:"process_rss_mb,omitempty"`
		GoMemoryLimitMB     *uint64   `yaml:"go_memory_limit_mb,omitempty"`
		OpenFileDescriptors *int64    `yaml:"open_file_descriptors,omitempty"`
		MaxFileDescriptors  *int64    `yaml:"max_file_descriptors,omitempty"`

		// Software
		Version   string `yaml:"version"`
		BuildHash string `yaml:"build_hash"`
		GoVersion string `yaml:"go_version"`
	} `yaml:"server"`

	Config struct {
		Source string `yaml:"store_type"`
		// AuditLoggingConfigured is the server's own audit check of its config, license, environment and
		// platform at packet time. The audit logger applies its targets only at startup, so after a runtime
		// config or license change this can differ from what the server is writing.
		AuditLoggingConfigured *bool `yaml:"audit_logging_configured,omitempty"`
	} `yaml:"config"`

	Database struct {
		Type                                *string    `yaml:"type,omitempty"`
		Version                             *string    `yaml:"version,omitempty"`
		SchemaVersion                       *string    `yaml:"schema_version,omitempty"`
		MasterConnections                   int        `yaml:"master_connections"`
		ReplicaConnections                  int        `yaml:"replica_connections"`
		SearchConnections                   int        `yaml:"search_connections"`
		MasterConnectionsInUse              *int       `yaml:"master_connections_in_use,omitempty"`
		MasterConnectionsIdle               *int       `yaml:"master_connections_idle,omitempty"`
		MasterPoolWaitCount                 *int64     `yaml:"master_pool_wait_count,omitempty"`
		MasterPoolWaitDurationMs            *int64     `yaml:"master_pool_wait_duration_ms,omitempty"`
		MasterConnectionsClosedMaxIdle      *int64     `yaml:"master_connections_closed_max_idle,omitempty"`
		MasterConnectionsClosedMaxLifetime  *int64     `yaml:"master_connections_closed_max_lifetime,omitempty"`
		ReplicaConnectionsInUse             *int       `yaml:"replica_connections_in_use,omitempty"`
		ReplicaConnectionsIdle              *int       `yaml:"replica_connections_idle,omitempty"`
		ReplicaPoolWaitCount                *int64     `yaml:"replica_pool_wait_count,omitempty"`
		ReplicaPoolWaitDurationMs           *int64     `yaml:"replica_pool_wait_duration_ms,omitempty"`
		ReplicaConnectionsClosedMaxIdle     *int64     `yaml:"replica_connections_closed_max_idle,omitempty"`
		ReplicaConnectionsClosedMaxLifetime *int64     `yaml:"replica_connections_closed_max_lifetime,omitempty"`
		ReplicasOnline                      *int       `yaml:"replicas_online,omitempty"`
		CacheHitRatio                       *float64   `yaml:"cache_hit_ratio,omitempty"`
		Deadlocks                           *int64     `yaml:"deadlocks,omitempty"`
		TempFiles                           *int64     `yaml:"temp_files,omitempty"`
		TempBytesMB                         *float64   `yaml:"temp_bytes_mb,omitempty"`
		Rollbacks                           *int64     `yaml:"rollbacks,omitempty"`
		IdleInTransactionCount              *int64     `yaml:"idle_in_transaction_count,omitempty"`
		LongestQueryDurationSeconds         *float64   `yaml:"longest_query_duration_seconds,omitempty"`
		WaitingForLockCount                 *int64     `yaml:"waiting_for_lock_count,omitempty"`
		PostsDeadTuples                     *int64     `yaml:"posts_dead_tuples,omitempty"`
		PostsLiveTuples                     *int64     `yaml:"posts_live_tuples,omitempty"`
		PostsVacuumInProgress               *bool      `yaml:"posts_vacuum_in_progress,omitempty"`
		PostsLastVacuum                     *time.Time `yaml:"posts_last_vacuum,omitempty"`
		PostsLastAutovacuum                 *time.Time `yaml:"posts_last_autovacuum,omitempty"`
	} `yaml:"database"`

	FileStore struct {
		Status         string  `yaml:"file_status"`
		Error          string  `yaml:"error,omitempty"`
		Driver         string  `yaml:"file_driver"`
		FilesystemType *string `yaml:"filesystem_type,omitempty"`
		TotalMB        *uint64 `yaml:"total_mb,omitempty"`
		AvailableMB    *uint64 `yaml:"available_mb,omitempty"`
	} `yaml:"file_store"`

	Websocket struct {
		Connections int `yaml:"connections"`
	} `yaml:"websocket"`

	Cluster struct {
		ID            string `yaml:"id"`
		NumberOfNodes *int   `yaml:"number_of_nodes,omitempty"`
		// NumberOfConnectedNodes leaves out nodes that are registered in the database but not in the gossip cluster.
		NumberOfConnectedNodes *int `yaml:"number_of_connected_nodes,omitempty"`
		IsLeader               bool `yaml:"is_leader,omitempty"`
	} `yaml:"cluster"`

	Notifications struct {
		Email struct {
			Status string `yaml:"status"`
			Error  string `yaml:"error,omitempty"`
		} `yaml:"email,omitempty"`
		Push struct {
			Status string `yaml:"status"`
			Error  string `yaml:"error,omitempty"`
		} `yaml:"push,omitempty"`
	} `yaml:"notifications,omitempty"`

	LDAP struct {
		Status        string `yaml:"status,omitempty"`
		Error         string `yaml:"error,omitempty"`
		ServerName    string `yaml:"server_name,omitempty"`
		ServerVersion string `yaml:"server_version,omitempty"`
	} `yaml:"ldap"`

	SAML struct {
		ProviderType string `yaml:"provider_type,omitempty"`
		Status       string `yaml:"status,omitempty"`
		Error        string `yaml:"error,omitempty"`
	} `yaml:"saml"`

	ElasticSearch struct {
		Status        string   `yaml:"status,omitempty"`
		Backend       string   `yaml:"backend,omitempty"`
		ServerVersion string   `yaml:"server_version,omitempty"`
		ServerPlugins []string `yaml:"server_plugins,omitempty"`
		Error         string   `yaml:"error,omitempty"`
	} `yaml:"elastic"`

	OAuthProviders OAuthProviders `yaml:"oauth_providers,omitempty"`
}

// OAuthProviderStatus reports the connectivity status of a single OAuth2/OpenID Connect provider.
type OAuthProviderStatus struct {
	Status string `yaml:"status,omitempty"` // ok / fail / disabled
	Error  string `yaml:"error,omitempty"`
}

// OAuthProviders aggregates the connectivity status for the configured OAuth2/OpenID Connect providers.
type OAuthProviders struct {
	GitLab    OAuthProviderStatus `yaml:"gitlab,omitempty"`
	Google    OAuthProviderStatus `yaml:"google,omitempty"`
	Office365 OAuthProviderStatus `yaml:"office365,omitempty"`
	OpenID    OAuthProviderStatus `yaml:"openid,omitempty"`
}

// SupportPacketStats reports workspace-scale analytics. Every field is a pointer: a nil value means the underlying query failed, which is not the same as zero.
type SupportPacketStats struct {
	RegisteredUsers     *int64 `yaml:"registered_users,omitempty"`
	ActiveUsers         *int64 `yaml:"active_users,omitempty"`
	DailyActiveUsers    *int64 `yaml:"daily_active_users,omitempty"`
	MonthlyActiveUsers  *int64 `yaml:"monthly_active_users,omitempty"`
	DeactivatedUsers    *int64 `yaml:"deactivated_users,omitempty"`
	Guests              *int64 `yaml:"guests,omitempty"`
	SingleChannelGuests *int64 `yaml:"single_channel_guests,omitempty"`
	BotAccounts         *int64 `yaml:"bot_accounts,omitempty"`
	Posts               *int64 `yaml:"posts,omitempty"`
	Channels            *int64 `yaml:"channels,omitempty"`
	Teams               *int64 `yaml:"teams,omitempty"`
	SlashCommands       *int64 `yaml:"slash_commands,omitempty"`
	IncomingWebhooks    *int64 `yaml:"incoming_webhooks,omitempty"`
	OutgoingWebhooks    *int64 `yaml:"outgoing_webhooks,omitempty"`
}

// SupportPacketJobList contains the list of latest run enterprise job runs.
// It is included in the Support Packet.
type SupportPacketJobList struct {
	LDAPSyncJobs               []*Job `yaml:"ldap_sync_jobs"`
	DataRetentionJobs          []*Job `yaml:"data_retention_jobs"`
	MessageExportJobs          []*Job `yaml:"message_export_jobs"`
	ElasticPostIndexingJobs    []*Job `yaml:"elastic_post_indexing_jobs"`
	ElasticPostAggregationJobs []*Job `yaml:"elastic_post_aggregation_jobs"`
	MigrationJobs              []*Job `yaml:"migration_jobs"`
}

// SupportPacketPermissionInfo contains the list of schemes and the list of roles.
// It is included in the Support Packet.
type SupportPacketPermissionInfo struct {
	Roles   []*Role   `yaml:"roles"`
	Schemes []*Scheme `yaml:"schemes"`
}

// SupportPacketConfig contains the Mattermost configuration. In contrast to [Config], it also contains the list of Feature Flags.
// It is included in the Support Packet.
type SupportPacketConfig struct {
	*Config
	FeatureFlags FeatureFlags `json:"FeatureFlags"`
}

// SupportPacketPluginList contains the list of enabled and disabled plugins.
// It is included in the Support Packet.
type SupportPacketPluginList struct {
	Enabled  []Manifest `json:"enabled"`
	Disabled []Manifest `json:"disabled"`
}

// SupportPacketDatabaseSchema contains the database schema information.
// It is included in the Support Packet.
type SupportPacketDatabaseSchema struct {
	DatabaseCollation string          `yaml:"database_collation,omitempty"`
	DatabaseEncoding  string          `yaml:"database_encoding,omitempty"`
	Tables            []DatabaseTable `yaml:"tables"`
}

// DatabaseTable represents a table in the database schema.
type DatabaseTable struct {
	Name      string            `yaml:"name"`
	Collation string            `yaml:"collation,omitempty"`
	Options   map[string]string `yaml:"options,omitempty"`
	Columns   []DatabaseColumn  `yaml:"columns"`
	Indexes   []DatabaseIndex   `yaml:"indexes,omitempty"`
}

// DatabaseColumn represents a column in a database table.
type DatabaseColumn struct {
	Name       string `yaml:"name"`
	DataType   string `yaml:"data_type"`
	MaxLength  int64  `yaml:"max_length,omitempty"`
	IsNullable bool   `yaml:"is_nullable"`
}

// DatabaseIndex represents an index in a database table.
type DatabaseIndex struct {
	Name       string `yaml:"name"`
	Definition string `yaml:"definition"`
}

type FileData struct {
	Filename string
	Body     []byte
}

type SupportPacketOptions struct {
	IncludeLogs   bool     // IncludeLogs is the option to include server logs
	PluginPackets []string // PluginPackets is a list of pluginids to call hooks
	// CPUProfileDuration lets programmatic callers (e.g. tests) override how long to sample
	// the CPU profile for. It is not exposed via the HTTP API.
	// nil leaves the server default in place; 0 skips CPU profile generation entirely.
	CPUProfileDuration *time.Duration
}
