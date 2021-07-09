// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"

	"github.com/mattermost/gorp"
)

// storeContextKey is the base type for all context keys for the store.
type storeContextKey string

// contextValue is a type to hold some pre-determined context values.
type contextValue string

// Different possible values of contextValue.
const (
	useMaster contextValue = "useMaster"
)

// WithMaster adds the context value that master DB should be selected for this request.
func WithMaster(ctx context.Context) context.Context {
	return context.WithValue(ctx, storeContextKey(useMaster), true)
}

// hasMaster is a helper function to check whether master DB should be selected or not.
func hasMaster(ctx context.Context) bool {
	if v := ctx.Value(storeContextKey(useMaster)); v != nil {
		if res, ok := v.(bool); ok && res {
			return true
		}
	}
	return false
}

// DBFromContext is a helper utility that returns the DB handle from a given context.
func (ss *SQLStore) DBFromContext(ctx context.Context) *gorp.DbMap {
	if hasMaster(ctx) {
		return ss.GetMaster()
	}
	return ss.GetReplica()
}
