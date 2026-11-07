// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package database

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
)

func init() {
	healthcheck.Register(
		dbNoReadReplica,
		dbNoSearchReplica,
		dbTraceOn,
		dbMaxOpenConnsLow,
		dbQueryTimeoutHigh,
		dbHAFailoverStormRisk,
		dbPostgresUnsupported,
		dbPoolSaturated,
		dbPostsDeadTuples,
		dbReplicaUnused,
		dbIdleExceedsOpen,
	)
}

const (
	databaseConsolePath = "/admin_console/environment/database"
	databaseDocsURL     = "https://docs.mattermost.com/administration-guide/configure/environment-configuration-settings.html#database"

	dataSourceReplicasSubject       = "SqlSettings.DataSourceReplicas"
	dataSourceSearchReplicasSubject = "SqlSettings.DataSourceSearchReplicas"
	maxOpenConnsSubject             = "SqlSettings.MaxOpenConns"
)

func unknownConfig() healthcheck.Result {
	return healthcheck.Unknown(healthcheck.ReasonConfigUnavailable)
}

func unknownStats() healthcheck.Result {
	return healthcheck.Unknown(healthcheck.ReasonStatsUnavailable)
}

func unknownDiagnostics() healthcheck.Result {
	return healthcheck.Unknown(healthcheck.ReasonDiagnosticsUnavailable)
}

func maxOpenConns(s *healthcheck.Snapshot) (int, bool) {
	return s.ConfigInt(func(cfg *model.Config) *int { return cfg.SqlSettings.MaxOpenConns })
}
