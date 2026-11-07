// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package store

import "time"

// DatabaseDiagnostics is a snapshot of database health and pool state.
// Pointer fields are nil when the underlying metric is unavailable
// (non-Postgres driver, query failure, or no matching row).
type DatabaseDiagnostics struct {
	MasterConnectionsInUse              int
	MasterConnectionsIdle               int
	MasterPoolWaitCount                 int64
	MasterPoolWaitDurationMs            int64
	MasterConnectionsClosedMaxIdle      int64
	MasterConnectionsClosedMaxLifetime  int64
	ReplicaConnectionsInUse             int
	ReplicaConnectionsIdle              int
	ReplicaPoolWaitCount                int64
	ReplicaPoolWaitDurationMs           int64
	ReplicaConnectionsClosedMaxIdle     int64
	ReplicaConnectionsClosedMaxLifetime int64
	ReplicasOnline                      int
	CacheHitRatio                       *float64
	Deadlocks                           *int64
	TempFiles                           *int64
	TempBytesMB                         *float64
	Rollbacks                           *int64
	IdleInTransactionCount              *int64
	LongestQueryDurationSeconds         *float64
	WaitingForLockCount                 *int64
	PostsDeadTuples                     *int64
	PostsLiveTuples                     *int64
	PostsVacuumInProgress               *bool
	PostsLastVacuum                     *time.Time
	PostsLastAutovacuum                 *time.Time
}
