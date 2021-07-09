// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package utils

import (
	"path/filepath"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

const (
	LogRotateSize           = 10000
	LogFilename             = "mattermost.log"
	LogNotificationFilename = "notifications.log"
)

type fileLocationFunc func(string) string

func MloggerConfigFromLoggerConfig(s *model.LogSettings, getFileFunc fileLocationFunc) *mlog.LoggerConfiguration {
	return &mlog.LoggerConfiguration{
		EnableConsole: *s.EnableConsole,
		ConsoleJSON:   *s.ConsoleJSON,
		ConsoleLevel:  strings.ToLower(*s.ConsoleLevel),
		EnableFile:    *s.EnableFile,
		FileJSON:      *s.FileJSON,
		FileLevel:     strings.ToLower(*s.FileLevel),
		FileLocation:  getFileFunc(*s.FileLocation),
		EnableColor:   *s.EnableColor,
	}
}

func GetLogFileLocation(fileLocation string) string {
	if fileLocation == "" {
		fileLocation, _ = fileutils.FindDir("logs")
	}

	return filepath.Join(fileLocation, LogFilename)
}

func GetNotificationsLogFileLocation(fileLocation string) string {
	if fileLocation == "" {
		fileLocation, _ = fileutils.FindDir("logs")
	}

	return filepath.Join(fileLocation, LogNotificationFilename)
}

func GetLogSettingsFromNotificationsLogSettings(notificationLogSettings *model.NotificationLogSettings) *model.LogSettings {
	return &model.LogSettings{
		ConsoleJSON:           notificationLogSettings.ConsoleJSON,
		ConsoleLevel:          notificationLogSettings.ConsoleLevel,
		EnableConsole:         notificationLogSettings.EnableConsole,
		EnableFile:            notificationLogSettings.EnableFile,
		FileJSON:              notificationLogSettings.FileJSON,
		FileLevel:             notificationLogSettings.FileLevel,
		FileLocation:          notificationLogSettings.FileLocation,
		AdvancedLoggingConfig: notificationLogSettings.AdvancedLoggingConfig,
		EnableColor:           notificationLogSettings.EnableColor,
	}
}

// DON'T USE THIS Modify the level on the app logger
func DisableDebugLogForTest() {
	mlog.GloballyDisableDebugLogForTest()
}

// DON'T USE THIS Modify the level on the app logger
func EnableDebugLogForTest() {
	mlog.GloballyEnableDebugLogForTest()
}
