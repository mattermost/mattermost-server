// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mail"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func (s *Server) GetLogs(page, perPage int) ([]string, *model.AppError) {
	var lines []string

	license := s.License()
	if license != nil && *license.Features.Cluster && s.Cluster != nil && *s.Config().ClusterSettings.Enable {
		if info := s.Cluster.GetMyClusterInfo(); info != nil {
			lines = append(lines, "-----------------------------------------------------------------------------------------------------------")
			lines = append(lines, "-----------------------------------------------------------------------------------------------------------")
			lines = append(lines, info.Hostname)
			lines = append(lines, "-----------------------------------------------------------------------------------------------------------")
			lines = append(lines, "-----------------------------------------------------------------------------------------------------------")
		} else {
			mlog.Error("Could not get cluster info")
		}
	}

	melines, err := s.GetLogsSkipSend(page, perPage)
	if err != nil {
		return nil, err
	}

	lines = append(lines, melines...)

	if s.Cluster != nil && *s.Config().ClusterSettings.Enable {
		clines, err := s.Cluster.GetLogs(page, perPage)
		if err != nil {
			return nil, err
		}

		lines = append(lines, clines...)
	}

	return lines, nil
}

func (a *App) GetLogs(page, perPage int) ([]string, *model.AppError) {
	return a.Srv().GetLogs(page, perPage)
}

func (s *Server) GetLogsSkipSend(page, perPage int) ([]string, *model.AppError) {
	var lines []string

	if *s.Config().LogSettings.EnableFile {
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), mlog.DefaultFlushTimeout)
		defer timeoutCancel()
		mlog.Flush(timeoutCtx)

		logFile := utils.GetLogFileLocation(*s.Config().LogSettings.FileLocation)
		file, err := os.Open(logFile)
		if err != nil {
			return nil, model.NewAppError("getLogs", "api.admin.file_read_error", nil, err.Error(), http.StatusInternalServerError)
		}

		defer file.Close()

		var newLine = []byte{'\n'}
		var lineCount int
		const searchPos = -1
		b := make([]byte, 1)
		var endOffset int64 = 0

		// if the file exists and it's last byte is '\n' - skip it
		var stat os.FileInfo
		if stat, err = os.Stat(logFile); err == nil {
			if _, err = file.ReadAt(b, stat.Size()-1); err == nil && b[0] == newLine[0] {
				endOffset = -1
			}
		}
		lineEndPos, err := file.Seek(endOffset, io.SeekEnd)
		if err != nil {
			return nil, model.NewAppError("getLogs", "api.admin.file_read_error", nil, err.Error(), http.StatusInternalServerError)
		}
		for {
			pos, err := file.Seek(searchPos, io.SeekCurrent)
			if err != nil {
				return nil, model.NewAppError("getLogs", "api.admin.file_read_error", nil, err.Error(), http.StatusInternalServerError)
			}

			_, err = file.ReadAt(b, pos)
			if err != nil {
				return nil, model.NewAppError("getLogs", "api.admin.file_read_error", nil, err.Error(), http.StatusInternalServerError)
			}

			if b[0] == newLine[0] || pos == 0 {
				lineCount++
				if lineCount > page*perPage {
					line := make([]byte, lineEndPos-pos)
					_, err := file.ReadAt(line, pos)
					if err != nil {
						return nil, model.NewAppError("getLogs", "api.admin.file_read_error", nil, err.Error(), http.StatusInternalServerError)
					}
					lines = append(lines, string(line))
				}
				if pos == 0 {
					break
				}
				lineEndPos = pos
			}

			if len(lines) == perPage {
				break
			}
		}

		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	} else {
		lines = append(lines, "")
	}

	return lines, nil
}

func (a *App) GetLogsSkipSend(page, perPage int) ([]string, *model.AppError) {
	return a.Srv().GetLogsSkipSend(page, perPage)
}

func (a *App) GetClusterStatus() []*model.ClusterInfo {
	infos := make([]*model.ClusterInfo, 0)

	if a.Cluster() != nil {
		infos = a.Cluster().GetClusterInfos()
	}

	return infos
}

func (s *Server) InvalidateAllCaches() *model.AppError {
	debug.FreeOSMemory()
	s.InvalidateAllCachesSkipSend()

	if s.Cluster != nil {

		msg := &model.ClusterMessage{
			Event:            model.CLUSTER_EVENT_INVALIDATE_ALL_CACHES,
			SendType:         model.CLUSTER_SEND_RELIABLE,
			WaitForAllToSend: true,
		}

		s.Cluster.SendClusterMessage(msg)
	}

	return nil
}

func (s *Server) InvalidateAllCachesSkipSend() {
	mlog.Info("Purging all caches")
	s.userService.ClearAllUsersSessionCacheLocal()
	s.statusCache.Purge()
	s.Store.Team().ClearCaches()
	s.Store.Channel().ClearCaches()
	s.Store.User().ClearCaches()
	s.Store.Post().ClearCaches()
	s.Store.FileInfo().ClearCaches()
	s.Store.Webhook().ClearCaches()
	linkCache.Purge()
	s.LoadLicense()
}

func (a *App) RecycleDatabaseConnection() {
	mlog.Info("Attempting to recycle database connections.")

	// This works by setting 10 seconds as the max conn lifetime for all DB connections.
	// This allows in gradually closing connections as they expire. In future, we can think
	// of exposing this as a param from the REST api.
	a.Srv().Store.RecycleDBConnections(10 * time.Second)

	mlog.Info("Finished recycling database connections.")
}

func (a *App) TestSiteURL(siteURL string) *model.AppError {
	url := fmt.Sprintf("%s/api/v4/system/ping", siteURL)
	res, err := http.Get(url)
	if err != nil || res.StatusCode != 200 {
		return model.NewAppError("testSiteURL", "app.admin.test_site_url.failure", nil, "", http.StatusBadRequest)
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, res.Body)
		_ = res.Body.Close()
	}()

	return nil
}

func (a *App) TestEmail(userID string, cfg *model.Config) *model.AppError {
	if *cfg.EmailSettings.SMTPServer == "" {
		return model.NewAppError("testEmail", "api.admin.test_email.missing_server", nil, i18n.T("api.context.invalid_param.app_error", map[string]interface{}{"Name": "SMTPServer"}), http.StatusBadRequest)
	}

	// if the user hasn't changed their email settings, fill in the actual SMTP password so that
	// the user can verify an existing SMTP connection
	if *cfg.EmailSettings.SMTPPassword == model.FAKE_SETTING {
		if *cfg.EmailSettings.SMTPServer == *a.Config().EmailSettings.SMTPServer &&
			*cfg.EmailSettings.SMTPPort == *a.Config().EmailSettings.SMTPPort &&
			*cfg.EmailSettings.SMTPUsername == *a.Config().EmailSettings.SMTPUsername {
			*cfg.EmailSettings.SMTPPassword = *a.Config().EmailSettings.SMTPPassword
		} else {
			return model.NewAppError("testEmail", "api.admin.test_email.reenter_password", nil, "", http.StatusBadRequest)
		}
	}
	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	T := i18n.GetUserTranslations(user.Locale)
	license := a.Srv().License()
	mailConfig := a.Srv().MailServiceConfig()
	if err := mail.SendMailUsingConfig(user.Email, T("api.admin.test_email.subject"), T("api.admin.test_email.body"), mailConfig, license != nil && *license.Features.Compliance, ""); err != nil {
		return model.NewAppError("testEmail", "app.admin.test_email.failure", map[string]interface{}{"Error": err.Error()}, "", http.StatusInternalServerError)
	}

	return nil
}

// serverBusyStateChanged is called when a CLUSTER_EVENT_BUSY_STATE_CHANGED is received.
func (s *Server) serverBusyStateChanged(sbs *model.ServerBusyState) {
	s.Busy.ClusterEventChanged(sbs)
	if sbs.Busy {
		mlog.Warn("server busy state activitated via cluster event - non-critical services disabled", mlog.Int64("expires_sec", sbs.Expires))
	} else {
		mlog.Info("server busy state cleared via cluster event - non-critical services enabled")
	}
}
