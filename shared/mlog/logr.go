// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mlog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/mattermost/logr"
	logrFmt "github.com/mattermost/logr/format"
	"github.com/mattermost/logr/target"
	"go.uber.org/zap/zapcore"
)

const (
	DefaultMaxTargetQueue = 1000
	DefaultSysLogPort     = 514
)

type LogLevel struct {
	ID         logr.LevelID
	Name       string
	Stacktrace bool
}

type LogTarget struct {
	Type         string // one of "console", "file", "tcp", "syslog", "none".
	Format       string // one of "json", "plain"
	Levels       []LogLevel
	Options      json.RawMessage
	MaxQueueSize int
}

type LogTargetCfg map[string]*LogTarget
type LogrCleanup func() error

func newLogr() *logr.Logger {
	lgr := &logr.Logr{}
	lgr.OnExit = func(int) {}
	lgr.OnPanic = func(interface{}) {}
	lgr.OnLoggerError = onLoggerError
	lgr.OnQueueFull = onQueueFull
	lgr.OnTargetQueueFull = onTargetQueueFull

	logger := lgr.NewLogger()
	return &logger
}

func logrAddTargets(logger *logr.Logger, targets LogTargetCfg) error {
	lgr := logger.Logr()
	var errs error
	for name, t := range targets {
		target, err := NewLogrTarget(name, t)
		if err != nil {
			errs = multierror.Append(err)
			continue
		}
		if target != nil {
			target.SetName(name)
			lgr.AddTarget(target)
		}
	}
	return errs
}

// NewLogrTarget creates a `logr.Target` based on a target config.
// Can be used when parsing custom config files, or when programmatically adding
// built-in targets. Use `mlog.AddTarget` to add custom targets.
func NewLogrTarget(name string, t *LogTarget) (logr.Target, error) {
	formatter, err := newFormatter(name, t.Format)
	if err != nil {
		return nil, err
	}
	filter := newFilter(t.Levels)

	if t.MaxQueueSize == 0 {
		t.MaxQueueSize = DefaultMaxTargetQueue
	}

	switch t.Type {
	case "console":
		return newConsoleTarget(name, t, filter, formatter)
	case "file":
		return newFileTarget(name, t, filter, formatter)
	case "syslog":
		return newSyslogTarget(name, t, filter, formatter)
	case "tcp":
		return newTCPTarget(name, t, filter, formatter)
	case "none":
		return nil, nil
	}
	return nil, fmt.Errorf("invalid type '%s' for target %s", t.Type, name)
}

func newFilter(levels []LogLevel) logr.Filter {
	filter := &logr.CustomFilter{}
	for _, lvl := range levels {
		filter.Add(logr.Level(lvl))
	}
	return filter
}

func newFormatter(name string, format string) (logr.Formatter, error) {
	switch format {
	case "json", "":
		return &logrFmt.JSON{}, nil
	case "plain":
		return &logrFmt.Plain{Delim: " | "}, nil
	default:
		return nil, fmt.Errorf("invalid format '%s' for target %s", format, name)
	}
}

func newConsoleTarget(name string, t *LogTarget, filter logr.Filter, formatter logr.Formatter) (logr.Target, error) {
	type consoleOptions struct {
		Out string `json:"Out"`
	}
	options := &consoleOptions{}
	if err := json.Unmarshal(t.Options, options); err != nil {
		return nil, err
	}

	var w io.Writer
	switch options.Out {
	case "stdout", "":
		w = os.Stdout
	case "stderr":
		w = os.Stderr
	default:
		return nil, fmt.Errorf("invalid out '%s' for target %s", options.Out, name)
	}

	newTarget := target.NewWriterTarget(filter, formatter, w, t.MaxQueueSize)
	return newTarget, nil
}

func newFileTarget(name string, t *LogTarget, filter logr.Filter, formatter logr.Formatter) (logr.Target, error) {
	type fileOptions struct {
		Filename   string `json:"Filename"`
		MaxSize    int    `json:"MaxSizeMB"`
		MaxAge     int    `json:"MaxAgeDays"`
		MaxBackups int    `json:"MaxBackups"`
		Compress   bool   `json:"Compress"`
	}
	options := &fileOptions{}
	if err := json.Unmarshal(t.Options, options); err != nil {
		return nil, err
	}
	return newFileTargetWithOpts(name, t, target.FileOptions(*options), filter, formatter)
}

func newFileTargetWithOpts(name string, t *LogTarget, opts target.FileOptions, filter logr.Filter, formatter logr.Formatter) (logr.Target, error) {
	if opts.Filename == "" {
		return nil, fmt.Errorf("missing 'Filename' option for target %s", name)
	}
	if err := checkFileWritable(opts.Filename); err != nil {
		return nil, fmt.Errorf("error writing to 'Filename' for target %s: %w", name, err)
	}

	newTarget := target.NewFileTarget(filter, formatter, opts, t.MaxQueueSize)
	return newTarget, nil
}

func newSyslogTarget(name string, t *LogTarget, filter logr.Filter, formatter logr.Formatter) (logr.Target, error) {
	options := &SyslogParams{}
	if err := json.Unmarshal(t.Options, options); err != nil {
		return nil, err
	}

	if options.IP == "" {
		return nil, fmt.Errorf("missing 'IP' option for target %s", name)
	}
	if options.Port == 0 {
		options.Port = DefaultSysLogPort
	}
	return NewSyslogTarget(filter, formatter, options, t.MaxQueueSize)
}

func newTCPTarget(name string, t *LogTarget, filter logr.Filter, formatter logr.Formatter) (logr.Target, error) {
	options := &TCPParams{}
	if err := json.Unmarshal(t.Options, options); err != nil {
		return nil, err
	}

	if options.IP == "" {
		return nil, fmt.Errorf("missing 'IP' option for target %s", name)
	}
	if options.Port == 0 {
		return nil, fmt.Errorf("missing 'Port' option for target %s", name)
	}
	return NewTCPTarget(filter, formatter, options, t.MaxQueueSize)
}

func checkFileWritable(filename string) error {
	// try opening/creating the file for writing
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func isLevelEnabled(logger *logr.Logger, level logr.Level) bool {
	if logger == nil || logger.Logr() == nil {
		return false
	}

	status := logger.Logr().IsLevelEnabled(level)
	return status.Enabled
}

// zapToLogr converts Zap fields to Logr fields.
// This will not be needed once Logr is used for all logging.
func zapToLogr(zapFields []Field) logr.Fields {
	encoder := zapcore.NewMapObjectEncoder()
	for _, zapField := range zapFields {
		zapField.AddTo(encoder)
	}
	return logr.Fields(encoder.Fields)
}

// mlogLevelToLogrLevel converts a mlog logger level to
// an array of discrete Logr levels.
func mlogLevelToLogrLevels(level string) []LogLevel {
	levels := make([]LogLevel, 0)
	levels = append(levels, LvlError, LvlPanic, LvlFatal, LvlStdLog)

	switch level {
	case LevelDebug:
		levels = append(levels, LvlDebug)
		fallthrough
	case LevelInfo:
		levels = append(levels, LvlInfo)
		fallthrough
	case LevelWarn:
		levels = append(levels, LvlWarn)
	}
	return levels
}
