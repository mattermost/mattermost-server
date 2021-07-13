// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin

import (
	"database/sql/driver"
)

// ResultContainer contains the output from the LastInsertID
// and RowsAffected methods for a given set of rows.
// It is used to embed another round-trip to the server,
// and helping to avoid tracking results on the server.
type ResultContainer struct {
	LastID            int64
	LastIDError       error
	RowsAffected      int64
	RowsAffectedError error
}

// Driver is a sql driver interface that is used by plugins to perform
// raw SQL queries without opening DB connections by themselves. This interface
// is not subject to backward compatibility guarantees and is only meant to be
// used by plugins built by the Mattermost team.
type Driver interface {
	// Connection
	Conn(isMaster bool) (string, error)
	ConnPing(connID string) error
	ConnClose(connID string) error
	ConnQuery(connID, q string, args []driver.NamedValue) (string, error)         // rows
	ConnExec(connID, q string, args []driver.NamedValue) (ResultContainer, error) // result

	// Transaction
	Tx(connID string, opts driver.TxOptions) (string, error)
	TxCommit(txID string) error
	TxRollback(txID string) error

	// Statement
	Stmt(connID, q string) (string, error)
	StmtClose(stID string) error
	StmtNumInput(stID string) int
	StmtQuery(stID string, args []driver.NamedValue) (string, error)         // rows
	StmtExec(stID string, args []driver.NamedValue) (ResultContainer, error) // result

	// Rows
	RowsColumns(rowsID string) []string
	RowsClose(rowsID string) error
	RowsNext(rowsID string, dest []driver.Value) error
	RowsHasNextResultSet(rowsID string) bool
	RowsNextResultSet(rowsID string) error
	RowsColumnTypeDatabaseTypeName(rowsID string, index int) string
	RowsColumnTypePrecisionScale(rowsID string, index int) (int64, int64, bool)

	// TODO: add this
	// RowsColumnScanType(rowsID string, index int) reflect.Type

	// Note: the following cannot be implemented because either MySQL or PG
	// does not support it. So this implementation has to be a common subset
	// of both DB implementations.
	// RowsColumnTypeLength(rowsID string, index int) (int64, bool)
	// RowsColumnTypeNullable(rowsID string, index int) (bool, bool)
	// ResetSession(ctx context.Context) error
	// IsValid() bool
}
