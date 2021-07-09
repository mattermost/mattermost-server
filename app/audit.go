// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"net/http"
	"os/user"

	"github.com/hashicorp/go-multierror"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

const (
	RestLevelID        = 240
	RestContentLevelID = 241
	RestPermsLevelID   = 242
	CLILevelID         = 243
)

var (
	LevelAPI     = mlog.LvlAuditAPI
	LevelContent = mlog.LvlAuditContent
	LevelPerms   = mlog.LvlAuditPerms
	LevelCLI     = mlog.LvlAuditCLI
)

func (a *App) GetAudits(userID string, limit int) (model.Audits, *model.AppError) {
	audits, err := a.Srv().Store.Audit().Get(userID, 0, limit)
	if err != nil {
		var outErr *store.ErrOutOfBounds
		switch {
		case errors.As(err, &outErr):
			return nil, model.NewAppError("GetAudits", "app.audit.get.limit.app_error", nil, err.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("GetAudits", "app.audit.get.finding.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return audits, nil
}

func (a *App) GetAuditsPage(userID string, page int, perPage int) (model.Audits, *model.AppError) {
	audits, err := a.Srv().Store.Audit().Get(userID, page*perPage, perPage)
	if err != nil {
		var outErr *store.ErrOutOfBounds
		switch {
		case errors.As(err, &outErr):
			return nil, model.NewAppError("GetAuditsPage", "app.audit.get.limit.app_error", nil, err.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("GetAuditsPage", "app.audit.get.finding.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return audits, nil
}

// LogAuditRec logs an audit record using default LvlAuditCLI.
func (a *App) LogAuditRec(rec *audit.Record, err error) {
	a.LogAuditRecWithLevel(rec, mlog.LvlAuditCLI, err)
}

// LogAuditRecWithLevel logs an audit record using specified Level.
func (a *App) LogAuditRecWithLevel(rec *audit.Record, level mlog.LogLevel, err error) {
	if rec == nil {
		return
	}
	if err != nil {
		if appErr, ok := err.(*model.AppError); ok {
			rec.AddMeta("err", appErr.Error())
			rec.AddMeta("code", appErr.StatusCode)
		} else {
			rec.AddMeta("err", err)
		}
		rec.Fail()
	}
	a.Srv().Audit.LogRecord(level, *rec)
}

// MakeAuditRecord creates a audit record pre-populated with defaults.
func (a *App) MakeAuditRecord(event string, initialStatus string) *audit.Record {
	var userID string
	user, err := user.Current()
	if err == nil {
		userID = fmt.Sprintf("%s:%s", user.Uid, user.Username)
	}

	rec := &audit.Record{
		APIPath:   "",
		Event:     event,
		Status:    initialStatus,
		UserID:    userID,
		SessionID: "",
		Client:    fmt.Sprintf("server %s-%s", model.BuildNumber, model.BuildHash),
		IPAddress: "",
		Meta:      audit.Meta{audit.KeyClusterID: a.GetClusterID()},
	}
	rec.AddMetaTypeConverter(model.AuditModelTypeConv)

	return rec
}

func (s *Server) configureAudit(adt *audit.Audit, bAllowAdvancedLogging bool) error {
	var errs error

	adt.OnQueueFull = s.onAuditTargetQueueFull
	adt.OnError = s.onAuditError

	// Configure target for rotating file output (E0, E10)
	if *s.Config().ExperimentalAuditSettings.FileEnabled {
		opts := audit.FileOptions{
			Filename:   *s.Config().ExperimentalAuditSettings.FileName,
			MaxSize:    *s.Config().ExperimentalAuditSettings.FileMaxSizeMB,
			MaxAge:     *s.Config().ExperimentalAuditSettings.FileMaxAgeDays,
			MaxBackups: *s.Config().ExperimentalAuditSettings.FileMaxBackups,
			Compress:   *s.Config().ExperimentalAuditSettings.FileCompress,
		}

		maxQueueSize := *s.Config().ExperimentalAuditSettings.FileMaxQueueSize
		if maxQueueSize <= 0 {
			maxQueueSize = audit.DefMaxQueueSize
		}

		filter := adt.MakeFilter(LevelAPI, LevelContent, LevelPerms, LevelCLI)
		formatter := adt.MakeJSONFormatter()
		formatter.DisableTimestamp = false
		target, err := audit.NewFileTarget(filter, formatter, opts, maxQueueSize)
		if err != nil {
			errs = multierror.Append(err)
		} else {
			mlog.Debug("File audit target created successfully", mlog.String("filename", opts.Filename))
			adt.AddTarget(target)
		}
	}

	// Advanced logging for audit requires license.
	dsn := *s.Config().ExperimentalAuditSettings.AdvancedLoggingConfig
	if !bAllowAdvancedLogging || dsn == "" {
		return errs
	}
	cfg, err := config.NewLogConfigSrc(dsn, s.configStore)
	if err != nil {
		errs = multierror.Append(fmt.Errorf("invalid config for audit, %w", err))
		return errs
	}
	mlog.Debug("Loaded audit configuration", mlog.String("source", dsn))

	for name, t := range cfg.Get() {
		if len(t.Levels) == 0 {
			t.Levels = mlog.MLvlAuditAll
		}
		target, err := mlog.NewLogrTarget(name, t)
		if err != nil {
			errs = multierror.Append(err)
			continue
		}
		if target != nil {
			adt.AddTarget(target)
		}
	}
	return errs
}

func (s *Server) onAuditTargetQueueFull(qname string, maxQSize int) bool {
	mlog.Error("Audit queue full, dropping record.", mlog.String("qname", qname), mlog.Int("queueSize", maxQSize))
	return true // drop it
}

func (s *Server) onAuditError(err error) {
	mlog.Error("Audit Error", mlog.Err(err))
}
