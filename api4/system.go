// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"runtime"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/cache"
	"github.com/mattermost/mattermost-server/v5/services/upgrader"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	RedirectLocationCacheSize = 10000
	DefaultServerBusySeconds  = 3600
	MaxServerBusySeconds      = 86400
)

var redirectLocationDataCache = cache.NewLRU(cache.LRUOptions{
	Size: RedirectLocationCacheSize,
})

func (api *API) InitSystem() {
	api.BaseRoutes.System.Handle("/ping", api.APIHandler(getSystemPing)).Methods("GET")

	api.BaseRoutes.System.Handle("/timezones", api.APISessionRequired(getSupportedTimezones)).Methods("GET")

	api.BaseRoutes.APIRoot.Handle("/audits", api.APISessionRequired(getAudits)).Methods("GET")
	api.BaseRoutes.APIRoot.Handle("/email/test", api.APISessionRequired(testEmail)).Methods("POST")
	api.BaseRoutes.APIRoot.Handle("/site_url/test", api.APISessionRequired(testSiteURL)).Methods("POST")
	api.BaseRoutes.APIRoot.Handle("/file/s3_test", api.APISessionRequired(testS3)).Methods("POST")
	api.BaseRoutes.APIRoot.Handle("/database/recycle", api.APISessionRequired(databaseRecycle)).Methods("POST")
	api.BaseRoutes.APIRoot.Handle("/caches/invalidate", api.APISessionRequired(invalidateCaches)).Methods("POST")

	api.BaseRoutes.APIRoot.Handle("/logs", api.APISessionRequired(getLogs)).Methods("GET")
	api.BaseRoutes.APIRoot.Handle("/logs", api.APIHandler(postLog)).Methods("POST")

	api.BaseRoutes.APIRoot.Handle("/analytics/old", api.APISessionRequired(getAnalytics)).Methods("GET")

	api.BaseRoutes.APIRoot.Handle("/redirect_location", api.APISessionRequiredTrustRequester(getRedirectLocation)).Methods("GET")

	api.BaseRoutes.APIRoot.Handle("/notifications/ack", api.APISessionRequired(pushNotificationAck)).Methods("POST")

	api.BaseRoutes.APIRoot.Handle("/server_busy", api.APISessionRequired(setServerBusy)).Methods("POST")
	api.BaseRoutes.APIRoot.Handle("/server_busy", api.APISessionRequired(getServerBusyExpires)).Methods("GET")
	api.BaseRoutes.APIRoot.Handle("/server_busy", api.APISessionRequired(clearServerBusy)).Methods("DELETE")
	api.BaseRoutes.APIRoot.Handle("/upgrade_to_enterprise", api.APISessionRequired(upgradeToEnterprise)).Methods("POST")
	api.BaseRoutes.APIRoot.Handle("/upgrade_to_enterprise/status", api.APISessionRequired(upgradeToEnterpriseStatus)).Methods("GET")
	api.BaseRoutes.APIRoot.Handle("/restart", api.APISessionRequired(restart)).Methods("POST")
	api.BaseRoutes.APIRoot.Handle("/warn_metrics/status", api.APISessionRequired(getWarnMetricsStatus)).Methods("GET")
	api.BaseRoutes.APIRoot.Handle("/warn_metrics/ack/{warn_metric_id:[A-Za-z0-9-_]+}", api.APIHandler(sendWarnMetricAckEmail)).Methods("POST")
	api.BaseRoutes.APIRoot.Handle("/warn_metrics/trial-license-ack/{warn_metric_id:[A-Za-z0-9-_]+}", api.APIHandler(requestTrialLicenseAndAckWarnMetric)).Methods("POST")
	api.BaseRoutes.System.Handle("/notices/{team_id:[A-Za-z0-9]+}", api.APISessionRequired(getProductNotices)).Methods("GET")
	api.BaseRoutes.System.Handle("/notices/view", api.APISessionRequired(updateViewedProductNotices)).Methods("PUT")

	api.BaseRoutes.System.Handle("/support_packet", api.APISessionRequired(generateSupportPacket)).Methods("GET")
}

func generateSupportPacket(c *Context, w http.ResponseWriter, r *http.Request) {
	const FileMime = "application/zip"
	const OutputDirectory = "support_packet"

	// Checking to see if the user is a admin of any sort or not
	// If they are a admin, they should theoritcally have access to one or more of the system console read permissions
	if !c.App.SessionHasPermissionToAny(*c.AppContext.Session(), model.SysconsoleReadPermissions) {
		c.SetPermissionError(model.SysconsoleReadPermissions...)
		return
	}

	// Checking to see if the server has a e10 or e20 license (this feature is only permitted for servers with licenses)
	if c.App.Srv().License() == nil {
		c.Err = model.NewAppError("Api4.generateSupportPacket", "api.no_license", nil, "", http.StatusForbidden)
		return
	}

	fileDatas := c.App.GenerateSupportPacket()

	// Constructing the ZIP file name as per spec (mattermost_support_packet_YYYY-MM-DD-HH-MM.zip)
	now := time.Now()
	outputZipFilename := fmt.Sprintf("mattermost_support_packet_%s.zip", now.Format("2006-01-02-03-04"))

	fileStorageBackend, fileBackendErr := c.App.FileBackend()
	if fileBackendErr != nil {
		c.Err = fileBackendErr
		return
	}

	// We do this incase we get concurrent requests, we will always have a unique directory.
	// This is to avoid the situation where we try to write to the same directory while we are trying to delete it (further down)
	outputDirectoryToUse := OutputDirectory + "_" + model.NewID()
	err := c.App.CreateZipFileAndAddFiles(fileStorageBackend, fileDatas, outputZipFilename, outputDirectoryToUse)
	if err != nil {
		c.Err = model.NewAppError("Api4.generateSupportPacket", "api.unable_to_create_zip_file", nil, err.Error(), http.StatusForbidden)
		return
	}

	fileBytes, err := fileStorageBackend.ReadFile(path.Join(outputDirectoryToUse, outputZipFilename))
	defer fileStorageBackend.RemoveDirectory(outputDirectoryToUse)
	if err != nil {
		c.Err = model.NewAppError("Api4.generateSupportPacket", "api.unable_to_read_file_from_backend", nil, err.Error(), http.StatusForbidden)
		return
	}
	fileBytesReader := bytes.NewReader(fileBytes)

	// Send the zip file back to client
	// We are able to pass 0 for content size due to the fact that Golang's serveContent (https://golang.org/src/net/http/fs.go)
	// already sets that for us
	writeFileResponse(outputZipFilename, FileMime, 0, now, *c.App.Config().ServiceSettings.WebserverMode, fileBytesReader, true, w, r)
}

func getSystemPing(c *Context, w http.ResponseWriter, r *http.Request) {
	reqs := c.App.Config().ClientRequirements

	s := make(map[string]string)
	s[model.STATUS] = model.StatusOk
	s["AndroidLatestVersion"] = reqs.AndroidLatestVersion
	s["AndroidMinVersion"] = reqs.AndroidMinVersion
	s["DesktopLatestVersion"] = reqs.DesktopLatestVersion
	s["DesktopMinVersion"] = reqs.DesktopMinVersion
	s["IosLatestVersion"] = reqs.IosLatestVersion
	s["IosMinVersion"] = reqs.IosMinVersion

	testflag := c.App.Config().FeatureFlags.TestFeature
	if testflag != "off" {
		s["TestFeatureFlag"] = testflag
	}

	actualGoroutines := runtime.NumGoroutine()
	if *c.App.Config().ServiceSettings.GoroutineHealthThreshold > 0 && actualGoroutines >= *c.App.Config().ServiceSettings.GoroutineHealthThreshold {
		mlog.Warn("The number of running goroutines is over the health threshold", mlog.Int("goroutines", actualGoroutines), mlog.Int("health_threshold", *c.App.Config().ServiceSettings.GoroutineHealthThreshold))
		s[model.STATUS] = model.StatusUnhealthy
	}

	// Enhanced ping health check:
	// If an extra form value is provided then perform extra health checks for
	// database and file storage backends.
	if r.FormValue("get_server_status") != "" {
		dbStatusKey := "database_status"
		s[dbStatusKey] = model.StatusOk

		writeErr := c.App.DBHealthCheckWrite()
		if writeErr != nil {
			mlog.Warn("Unable to write to database.", mlog.Err(writeErr))
			s[dbStatusKey] = model.StatusUnhealthy
			s[model.STATUS] = model.StatusUnhealthy
		}

		writeErr = c.App.DBHealthCheckDelete()
		if writeErr != nil {
			mlog.Warn("Unable to remove ping health check value from database.", mlog.Err(writeErr))
			s[dbStatusKey] = model.StatusUnhealthy
			s[model.STATUS] = model.StatusUnhealthy
		}

		if s[dbStatusKey] == model.StatusOk {
			mlog.Debug("Able to write to database.")
		}

		filestoreStatusKey := "filestore_status"
		s[filestoreStatusKey] = model.StatusOk
		appErr := c.App.TestFileStoreConnection()
		if appErr != nil {
			s[filestoreStatusKey] = model.StatusUnhealthy
			s[model.STATUS] = model.StatusUnhealthy
		}

		w.Header().Set(model.STATUS, s[model.STATUS])
		w.Header().Set(dbStatusKey, s[dbStatusKey])
		w.Header().Set(filestoreStatusKey, s[filestoreStatusKey])
	}

	if s[model.STATUS] != model.StatusOk {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Write([]byte(model.MapToJSON(s)))
}

func testEmail(c *Context, w http.ResponseWriter, r *http.Request) {
	cfg := model.ConfigFromJSON(r.Body)
	if cfg == nil {
		cfg = c.App.Config()
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionTestEmail) {
		c.SetPermissionError(model.PermissionTestEmail)
		return
	}

	if *c.App.Config().ExperimentalSettings.RestrictSystemAdmin {
		c.Err = model.NewAppError("testEmail", "api.restricted_system_admin", nil, "", http.StatusForbidden)
		return
	}

	err := c.App.TestEmail(c.AppContext.Session().UserID, cfg)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func testSiteURL(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionTestSiteURL) {
		c.SetPermissionError(model.PermissionTestSiteURL)
		return
	}

	if *c.App.Config().ExperimentalSettings.RestrictSystemAdmin {
		c.Err = model.NewAppError("testSiteURL", "api.restricted_system_admin", nil, "", http.StatusForbidden)
		return
	}

	props := model.MapFromJSON(r.Body)
	siteURL := props["site_url"]
	if siteURL == "" {
		c.SetInvalidParam("site_url")
		return
	}

	err := c.App.TestSiteURL(siteURL)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func getAudits(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("getAudits", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionReadAudits) {
		c.SetPermissionError(model.PermissionReadAudits)
		return
	}

	audits, err := c.App.GetAuditsPage("", c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("page", c.Params.Page)
	auditRec.AddMeta("audits_per_page", c.Params.LogsPerPage)

	w.Write([]byte(audits.ToJSON()))
}

func databaseRecycle(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionRecycleDatabaseConnections) {
		c.SetPermissionError(model.PermissionRecycleDatabaseConnections)
		return
	}

	auditRec := c.MakeAuditRecord("databaseRecycle", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if *c.App.Config().ExperimentalSettings.RestrictSystemAdmin {
		c.Err = model.NewAppError("databaseRecycle", "api.restricted_system_admin", nil, "", http.StatusForbidden)
		return
	}

	c.App.RecycleDatabaseConnection()

	auditRec.Success()
	ReturnStatusOK(w)
}

func invalidateCaches(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionInvalidateCaches) {
		c.SetPermissionError(model.PermissionInvalidateCaches)
		return
	}

	auditRec := c.MakeAuditRecord("invalidateCaches", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if *c.App.Config().ExperimentalSettings.RestrictSystemAdmin {
		c.Err = model.NewAppError("invalidateCaches", "api.restricted_system_admin", nil, "", http.StatusForbidden)
		return
	}

	err := c.App.Srv().InvalidateAllCaches()
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	ReturnStatusOK(w)
}

func getLogs(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("getLogs", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if *c.App.Config().ExperimentalSettings.RestrictSystemAdmin {
		c.Err = model.NewAppError("getLogs", "api.restricted_system_admin", nil, "", http.StatusForbidden)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionGetLogs) {
		c.SetPermissionError(model.PermissionGetLogs)
		return
	}

	lines, err := c.App.GetLogs(c.Params.Page, c.Params.LogsPerPage)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.AddMeta("page", c.Params.Page)
	auditRec.AddMeta("logs_per_page", c.Params.LogsPerPage)

	w.Write([]byte(model.ArrayToJSON(lines)))
}

func postLog(c *Context, w http.ResponseWriter, r *http.Request) {
	forceToDebug := false

	if !*c.App.Config().ServiceSettings.EnableDeveloper {
		if c.AppContext.Session().UserID == "" {
			c.Err = model.NewAppError("postLog", "api.context.permissions.app_error", nil, "", http.StatusForbidden)
			return
		}

		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
			forceToDebug = true
		}
	}

	m := model.MapFromJSON(r.Body)
	lvl := m["level"]
	msg := m["message"]

	if len(msg) > 400 {
		msg = msg[0:399]
	}

	msg = "Client Logs API Endpoint Message: " + msg
	fields := []mlog.Field{
		mlog.String("type", "client_message"),
		mlog.String("user_agent", c.AppContext.UserAgent()),
	}

	if !forceToDebug && lvl == "ERROR" {
		mlog.Error(msg, fields...)
	} else {
		mlog.Debug(msg, fields...)
	}

	m["message"] = msg
	w.Write([]byte(model.MapToJSON(m)))
}

func getAnalytics(c *Context, w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	teamID := r.URL.Query().Get("team_id")

	if name == "" {
		name = "standard"
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionGetAnalytics) {
		c.SetPermissionError(model.PermissionGetAnalytics)
		return
	}

	rows, err := c.App.GetAnalytics(name, teamID)
	if err != nil {
		c.Err = err
		return
	}

	if rows == nil {
		c.SetInvalidParam("name")
		return
	}

	w.Write([]byte(rows.ToJSON()))
}

func getSupportedTimezones(c *Context, w http.ResponseWriter, r *http.Request) {
	supportedTimezones := c.App.Timezones().GetSupported()
	if supportedTimezones == nil {
		supportedTimezones = make([]string, 0)
	}

	b, err := json.Marshal(supportedTimezones)
	if err != nil {
		c.Logger.Warn("Unable to marshal JSON in timezones.", mlog.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Write(b)
}

func testS3(c *Context, w http.ResponseWriter, r *http.Request) {
	cfg := model.ConfigFromJSON(r.Body)
	if cfg == nil {
		cfg = c.App.Config()
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionTestS3) {
		c.SetPermissionError(model.PermissionTestS3)
		return
	}

	if *c.App.Config().ExperimentalSettings.RestrictSystemAdmin {
		c.Err = model.NewAppError("testS3", "api.restricted_system_admin", nil, "", http.StatusForbidden)
		return
	}

	err := c.App.CheckMandatoryS3Fields(&cfg.FileSettings)
	if err != nil {
		c.Err = err
		return
	}

	if *cfg.FileSettings.AmazonS3SecretAccessKey == model.FakeSetting {
		cfg.FileSettings.AmazonS3SecretAccessKey = c.App.Config().FileSettings.AmazonS3SecretAccessKey
	}

	appErr := c.App.TestFileStoreConnectionWithConfig(&cfg.FileSettings)
	if appErr != nil {
		c.Err = appErr
		return
	}

	ReturnStatusOK(w)
}

func getRedirectLocation(c *Context, w http.ResponseWriter, r *http.Request) {
	m := make(map[string]string)
	m["location"] = ""

	if !*c.App.Config().ServiceSettings.EnableLinkPreviews {
		w.Write([]byte(model.MapToJSON(m)))
		return
	}

	url := r.URL.Query().Get("url")
	if url == "" {
		c.SetInvalidParam("url")
		return
	}

	var location string
	if err := redirectLocationDataCache.Get(url, &location); err == nil {
		m["location"] = location
		w.Write([]byte(model.MapToJSON(m)))
		return
	}

	client := c.App.HTTPService().MakeClient(false)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	res, err := client.Head(url)
	if err != nil {
		// Cache failures to prevent retries.
		redirectLocationDataCache.SetWithExpiry(url, "", 1*time.Hour)
		// Always return a success status and a JSON string to limit information returned to client.
		w.Write([]byte(model.MapToJSON(m)))
		return
	}
	defer func() {
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
	}()

	location = res.Header.Get("Location")
	redirectLocationDataCache.SetWithExpiry(url, location, 1*time.Hour)
	m["location"] = location

	w.Write([]byte(model.MapToJSON(m)))
}

func pushNotificationAck(c *Context, w http.ResponseWriter, r *http.Request) {
	ack, err := model.PushNotificationAckFromJSON(r.Body)
	if err != nil {
		c.Err = model.NewAppError("pushNotificationAck",
			"api.push_notifications_ack.message.parse.app_error",
			nil,
			err.Error(),
			http.StatusBadRequest,
		)
		return
	}

	if !*c.App.Config().EmailSettings.SendPushNotifications {
		c.Err = model.NewAppError("pushNotificationAck", "api.push_notification.disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	err = c.App.SendAckToPushProxy(ack)
	if ack.IsIDLoaded {
		if err != nil {
			// Log the error only, then continue to fetch notification message
			c.App.NotificationsLog().Error("Notification ack not sent to push proxy",
				mlog.String("ackId", ack.ID),
				mlog.String("type", ack.NotificationType),
				mlog.String("postId", ack.PostID),
				mlog.String("status", err.Error()),
			)
		}

		notificationInterface := c.App.Notification()

		if notificationInterface == nil {
			c.Err = model.NewAppError("pushNotificationAck", "api.system.id_loaded.not_available.app_error", nil, "", http.StatusFound)
			return
		}

		msg, appError := notificationInterface.GetNotificationMessage(ack, c.AppContext.Session().UserID)
		if appError != nil {
			c.Err = model.NewAppError("pushNotificationAck", "api.push_notification.id_loaded.fetch.app_error", nil, appError.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte(msg.ToJSON()))

		return
	} else if err != nil {
		c.Err = model.NewAppError("pushNotificationAck", "api.push_notifications_ack.forward.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	ReturnStatusOK(w)
}

func setServerBusy(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	// number of seconds to keep server marked busy
	secs := r.URL.Query().Get("seconds")
	if secs == "" {
		secs = strconv.FormatInt(DefaultServerBusySeconds, 10)
	}

	i, err := strconv.ParseInt(secs, 10, 64)
	if err != nil || i <= 0 || i > MaxServerBusySeconds {
		c.SetInvalidURLParam(fmt.Sprintf("seconds must be 1 - %d", MaxServerBusySeconds))
		return
	}

	auditRec := c.MakeAuditRecord("setServerBusy", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("seconds", i)

	c.App.Srv().Busy.Set(time.Second * time.Duration(i))
	mlog.Warn("server busy state activated - non-critical services disabled", mlog.Int64("seconds", i))

	auditRec.Success()
	ReturnStatusOK(w)
}

func clearServerBusy(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	auditRec := c.MakeAuditRecord("clearServerBusy", audit.Fail)
	defer c.LogAuditRec(auditRec)

	c.App.Srv().Busy.Clear()
	mlog.Info("server busy state cleared - non-critical services enabled")

	auditRec.Success()
	ReturnStatusOK(w)
}

func getServerBusyExpires(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}
	w.Write([]byte(c.App.Srv().Busy.ToJSON()))
}

func upgradeToEnterprise(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("upgradeToEnterprise", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	if model.BuildEnterpriseReady == "true" {
		c.Err = model.NewAppError("upgradeToEnterprise", "api.upgrade_to_enterprise.already-enterprise.app_error", nil, "", http.StatusTooManyRequests)
		return
	}

	percentage, _ := c.App.Srv().UpgradeToE0Status()

	if percentage > 0 {
		c.Err = model.NewAppError("upgradeToEnterprise", "api.upgrade_to_enterprise.app_error", nil, "", http.StatusTooManyRequests)
		return
	}
	if percentage == 100 {
		c.Err = model.NewAppError("upgradeToEnterprise", "api.upgrade_to_enterprise.already-done.app_error", nil, "", http.StatusTooManyRequests)
		return
	}

	if err := c.App.Srv().CanIUpgradeToE0(); err != nil {
		var ipErr *upgrader.InvalidPermissions
		var iaErr *upgrader.InvalidArch
		switch {
		case errors.As(err, &ipErr):
			params := map[string]interface{}{
				"MattermostUsername": ipErr.MattermostUsername,
				"FileUsername":       ipErr.FileUsername,
				"Path":               ipErr.Path,
			}
			if ipErr.ErrType == "invalid-user-and-permission" {
				c.Err = model.NewAppError("upgradeToEnterprise", "api.upgrade_to_enterprise.invalid-user-and-permission.app_error", params, err.Error(), http.StatusForbidden)
			} else if ipErr.ErrType == "invalid-user" {
				c.Err = model.NewAppError("upgradeToEnterprise", "api.upgrade_to_enterprise.invalid-user.app_error", params, err.Error(), http.StatusForbidden)
			} else if ipErr.ErrType == "invalid-permission" {
				c.Err = model.NewAppError("upgradeToEnterprise", "api.upgrade_to_enterprise.invalid-permission.app_error", params, err.Error(), http.StatusForbidden)
			}
		case errors.As(err, &iaErr):
			c.Err = model.NewAppError("upgradeToEnterprise", "api.upgrade_to_enterprise.system_not_supported.app_error", nil, err.Error(), http.StatusForbidden)
		default:
			c.Err = model.NewAppError("upgradeToEnterprise", "api.upgrade_to_enterprise.generic_error.app_error", nil, err.Error(), http.StatusForbidden)
		}
		return
	}

	c.App.Srv().Go(func() {
		c.App.Srv().UpgradeToE0()
	})

	auditRec.Success()
	w.WriteHeader(http.StatusAccepted)
	ReturnStatusOK(w)
}

func upgradeToEnterpriseStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	percentage, err := c.App.Srv().UpgradeToE0Status()
	var s map[string]interface{}
	if err != nil {
		var isErr *upgrader.InvalidSignature
		switch {
		case errors.As(err, &isErr):
			appErr := model.NewAppError("upgradeToEnterpriseStatus", "api.upgrade_to_enterprise_status.app_error", nil, err.Error(), http.StatusBadRequest)
			s = map[string]interface{}{"percentage": 0, "error": appErr.Message}
		default:
			appErr := model.NewAppError("upgradeToEnterpriseStatus", "api.upgrade_to_enterprise_status.signature.app_error", nil, err.Error(), http.StatusBadRequest)
			s = map[string]interface{}{"percentage": 0, "error": appErr.Message}
		}
	} else {
		s = map[string]interface{}{"percentage": percentage, "error": nil}
	}

	w.Write([]byte(model.StringInterfaceToJSON(s)))
}

func restart(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("restartServer", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
	time.Sleep(1 * time.Second)

	go func() {
		c.App.Srv().Restart()
	}()
}

func getWarnMetricsStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionToAny(*c.AppContext.Session(), model.SysconsoleReadPermissions) {
		c.SetPermissionError(model.SysconsoleReadPermissions...)
		return
	}

	license := c.App.Srv().License()
	if license != nil {
		mlog.Debug("License is present, skip.")
		return
	}

	status, err := c.App.GetWarnMetricsStatus()
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.MapWarnMetricStatusToJSON(status)))
}

func sendWarnMetricAckEmail(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("sendWarnMetricAckEmail", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	license := c.App.Srv().License()
	if license != nil {
		mlog.Debug("License is present, skip.")
		return
	}

	user, appErr := c.App.GetUser(c.AppContext.Session().UserID)
	if appErr != nil {
		c.Err = appErr
		return
	}

	ack := model.SendWarnMetricAckFromJSON(r.Body)
	if ack == nil {
		c.SetInvalidParam("ack")
		return
	}

	appErr = c.App.NotifyAndSetWarnMetricAck(c.Params.WarnMetricID, user, ack.ForceAck, false)
	if appErr != nil {
		c.Err = appErr
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func requestTrialLicenseAndAckWarnMetric(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("requestTrialLicenseAndAckWarnMetric", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	if model.BuildEnterpriseReady != "true" {
		mlog.Debug("Not Enterprise Edition, skip.")
		return
	}

	license := c.App.Srv().License()
	if license != nil {
		mlog.Debug("License is present, skip.")
		return
	}

	if err := c.App.RequestLicenseAndAckWarnMetric(c.AppContext, c.Params.WarnMetricID, false); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func getProductNotices(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	client, parseError := model.NoticeClientTypeFromString(r.URL.Query().Get("client"))
	if parseError != nil {
		c.SetInvalidParam("client")
		return
	}
	clientVersion := r.URL.Query().Get("clientVersion")
	locale := r.URL.Query().Get("locale")

	notices, err := c.App.GetProductNotices(c.AppContext, c.AppContext.Session().UserID, c.Params.TeamID, client, clientVersion, locale)

	if err != nil {
		c.Err = err
		return
	}
	result, _ := notices.Marshal()
	_, _ = w.Write(result)
}

func updateViewedProductNotices(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("updateViewedProductNotices", audit.Fail)
	defer c.LogAuditRec(auditRec)
	c.LogAudit("attempt")

	ids := model.ArrayFromJSON(r.Body)
	err := c.App.UpdateViewedProductNotices(c.AppContext.Session().UserID, ids)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}
