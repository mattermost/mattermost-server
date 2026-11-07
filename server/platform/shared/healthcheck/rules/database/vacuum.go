// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package database

import (
	"strconv"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
)

const (
	postsVacuumDeadTuples = 100_000
	postsVacuumDeadRatio  = 0.2
	postsVacuumStaleAfter = 7 * 24 * time.Hour
)

var dbPostsDeadTuples = healthcheck.Rule{
	Code:     "DB_POSTS_DEAD_TUPLES",
	Area:     model.AreaDatabase,
	Severity: healthcheck.SeverityWarning,
	RuleText: model.RuleText{
		TitleID:       healthcheck.TranslationId("health.rule.db_posts_dead_tuples.title"),
		RemediationID: healthcheck.TranslationId("health.rule.db_posts_dead_tuples.remediation"),
		DocsURL:       "https://www.postgresql.org/docs/current/routine-vacuuming.html",
	},
	Surface:    healthcheck.SurfaceProduct,
	Volatility: healthcheck.VolatilityThreshold,
	Subject:    "Database.PostsDeadTuples",
	Eval:       healthcheck.Single(evalDBPostsDeadTuples),
}

func evalDBPostsDeadTuples(s *healthcheck.Snapshot) healthcheck.Result {
	identity, ok := s.LeaderDiag(model.SectionDatabaseIdentity)
	if !ok || identity.Database.Type == nil {
		return unknownDiagnostics()
	}
	if *identity.Database.Type != model.DatabaseDriverPostgres {
		return healthcheck.Resolved()
	}

	diag, ok := s.LeaderDiag(model.SectionDatabaseStats)
	if !ok || diag.Database.PostsDeadTuples == nil {
		return unknownDiagnostics()
	}
	dead := *diag.Database.PostsDeadTuples
	if dead < 0 {
		return unknownDiagnostics()
	}
	if dead <= postsVacuumDeadTuples {
		return healthcheck.Resolved().WithValue(float64(dead))
	}

	// Live tuples come from the same pg_stat_user_tables row as the dead tuples, so both are
	// sampled together. Like autovacuum's reltuples they are an estimate, which avoids depending
	// on an exact COUNT(*) of Posts that is slowest on the largest installs.
	if diag.Database.PostsLiveTuples == nil || *diag.Database.PostsLiveTuples <= 0 {
		return unknownDiagnostics()
	}
	posts := *diag.Database.PostsLiveTuples

	deadRatio := float64(dead) / float64(posts)
	if deadRatio <= postsVacuumDeadRatio {
		return healthcheck.Resolved().WithValue(float64(dead))
	}

	if diag.Database.PostsVacuumInProgress == nil {
		return unknownDiagnostics()
	}
	if *diag.Database.PostsVacuumInProgress {
		return healthcheck.Unknown(healthcheck.TranslationId("health.rule.db_posts_dead_tuples.in_progress"))
	}

	// The thresholds track autovacuum's default trigger, so a healthy autovacuum starts a run
	// within about a minute of crossing them. The last vacuum time only selects the likely cause.
	firing := func(messageID string) healthcheck.Result {
		return healthcheck.Firing(messageID).
			WithDetail("dead_percent", strconv.FormatFloat(100*deadRatio, 'f', 1, 64)).
			WithDetail("dead_tuples", strconv.FormatInt(dead, 10)).
			WithDetail("live_tuples", strconv.FormatInt(posts, 10)).
			WithValue(float64(dead))
	}

	lastVacuum, vacuumed := postsLastVacuum(diag)
	if !vacuumed {
		return firing(healthcheck.TranslationId("health.rule.db_posts_dead_tuples.message.never"))
	}
	age, ok := s.Since(lastVacuum)
	if !ok {
		return healthcheck.Unknown(healthcheck.ReasonCollectedAtUnknown)
	}
	if age < 0 {
		return unknownDiagnostics()
	}
	if age <= postsVacuumStaleAfter {
		// Vacuum ran recently but left the dead tuples: something is holding back cleanup.
		return firing(healthcheck.TranslationId("health.rule.db_posts_dead_tuples.message.blocked")).
			WithDetail("last_vacuum", lastVacuum.Format(time.RFC3339))
	}

	return firing(healthcheck.TranslationId("health.rule.db_posts_dead_tuples.message.stale")).
		WithDetail("age_days", strconv.Itoa(int(age.Hours()/24))).
		WithDetail("last_vacuum", lastVacuum.Format(time.RFC3339))
}

func postsLastVacuum(diag *model.SupportPacketDiagnostics) (time.Time, bool) {
	manual := diag.Database.PostsLastVacuum
	auto := diag.Database.PostsLastAutovacuum
	switch {
	case manual == nil && auto == nil:
		return time.Time{}, false
	case manual == nil:
		return *auto, true
	case auto == nil || manual.After(*auto):
		return *manual, true
	default:
		return *auto, true
	}
}
