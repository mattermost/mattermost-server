// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mlog_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func TestLoggingBeforeInitialized(t *testing.T) {
	require.NotPanics(t, func() {
		// None of these should segfault before mlog is globally configured
		mlog.Info("info log")
		mlog.Debug("debug log")
		mlog.Warn("warning log")
		mlog.Error("error log")
		mlog.Critical("critical log")
	})
}

func TestLoggingAfterInitialized(t *testing.T) {
	testCases := []struct {
		Description         string
		LoggerConfiguration *mlog.LoggerConfiguration
		ExpectedLogs        []string
	}{
		{
			"file logging, json, debug",
			&mlog.LoggerConfiguration{
				EnableConsole: false,
				EnableFile:    true,
				FileJSON:      true,
				FileLevel:     mlog.LevelDebug,
			},
			[]string{
				`{"level":"debug","ts":0,"caller":"mlog/global_test.go:0","msg":"real debug log"}`,
				`{"level":"info","ts":0,"caller":"mlog/global_test.go:0","msg":"real info log"}`,
				`{"level":"warn","ts":0,"caller":"mlog/global_test.go:0","msg":"real warning log"}`,
				`{"level":"error","ts":0,"caller":"mlog/global_test.go:0","msg":"real error log"}`,
				`{"level":"error","ts":0,"caller":"mlog/global_test.go:0","msg":"real critical log"}`,
			},
		},
		{
			"file logging, json, error",
			&mlog.LoggerConfiguration{
				EnableConsole: false,
				EnableFile:    true,
				FileJSON:      true,
				FileLevel:     mlog.LevelError,
			},
			[]string{
				`{"level":"error","ts":0,"caller":"mlog/global_test.go:0","msg":"real error log"}`,
				`{"level":"error","ts":0,"caller":"mlog/global_test.go:0","msg":"real critical log"}`,
			},
		},
		{
			"file logging, non-json, debug",
			&mlog.LoggerConfiguration{
				EnableConsole: false,
				EnableFile:    true,
				FileJSON:      false,
				FileLevel:     mlog.LevelDebug,
			},
			[]string{
				`TIME	debug	mlog/global_test.go:0	real debug log`,
				`TIME	info	mlog/global_test.go:0	real info log`,
				`TIME	warn	mlog/global_test.go:0	real warning log`,
				`TIME	error	mlog/global_test.go:0	real error log`,
				`TIME	error	mlog/global_test.go:0	real critical log`,
			},
		},
		{
			"file logging, non-json, error",
			&mlog.LoggerConfiguration{
				EnableConsole: false,
				EnableFile:    true,
				FileJSON:      false,
				FileLevel:     mlog.LevelError,
			},
			[]string{
				`TIME	error	mlog/global_test.go:0	real error log`,
				`TIME	error	mlog/global_test.go:0	real critical log`,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			var filePath string
			if testCase.LoggerConfiguration.EnableFile {
				tempDir, err := ioutil.TempDir(os.TempDir(), "TestLoggingAfterInitialized")
				require.NoError(t, err)
				defer os.Remove(tempDir)

				filePath = filepath.Join(tempDir, "file.log")
				testCase.LoggerConfiguration.FileLocation = filePath
			}

			logger := mlog.NewLogger(testCase.LoggerConfiguration)
			mlog.InitGlobalLogger(logger)

			mlog.Debug("real debug log")
			mlog.Info("real info log")
			mlog.Warn("real warning log")
			mlog.Error("real error log")
			mlog.Critical("real critical log")

			if testCase.LoggerConfiguration.EnableFile {
				logs, err := ioutil.ReadFile(filePath)
				require.NoError(t, err)

				actual := strings.TrimSpace(string(logs))

				if testCase.LoggerConfiguration.FileJSON {
					reTs := regexp.MustCompile(`"ts":[0-9\.]+`)
					reCaller := regexp.MustCompile(`"caller":"([^"]+):[0-9\.]+"`)
					actual = reTs.ReplaceAllString(actual, `"ts":0`)
					actual = reCaller.ReplaceAllString(actual, `"caller":"$1:0"`)
				} else {
					actualRows := strings.Split(actual, "\n")
					for i, actualRow := range actualRows {
						actualFields := strings.Split(actualRow, "\t")
						if len(actualFields) > 3 {
							actualFields[0] = "TIME"
							reCaller := regexp.MustCompile(`([^"]+):[0-9\.]+`)
							actualFields[2] = reCaller.ReplaceAllString(actualFields[2], "$1:0")
							actualRows[i] = strings.Join(actualFields, "\t")
						}
					}

					actual = strings.Join(actualRows, "\n")
				}
				require.ElementsMatch(t, testCase.ExpectedLogs, strings.Split(actual, "\n"))
			}
		})
	}
}
