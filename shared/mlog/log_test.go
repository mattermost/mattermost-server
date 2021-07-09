// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mlog_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

// Test race condition when shutting down advanced logging. This test must run with the -race flag in order to verify
// that there is no race.
func TestLogger_ShutdownAdvancedLoggingRace(t *testing.T) {
	logger := mlog.NewLogger(&mlog.LoggerConfiguration{
		EnableConsole: true,
		ConsoleJSON:   true,
		EnableFile:    false,
		FileLevel:     mlog.LevelInfo,
	})
	started := make(chan bool)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		started <- true

		for {
			select {
			case <-ctx.Done():
				return
			default:
				logger.Debug("testing...")
			}
		}
	}()

	<-started

	err := logger.ShutdownAdvancedLogging(ctx)
	require.NoError(t, err)

	cancel()
	wg.Wait()
}
