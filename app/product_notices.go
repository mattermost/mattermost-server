// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	date_constraints "github.com/reflog/dateconstraints"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"
)

const MaxRepeatViewings = 3
const MinSecondsBetweenRepeatViewings = 60 * 60

// http request cache
var noticesCache = utils.RequestCache{}

// cached counts that are used during notice condition validation
var cachedPostCount int64
var cachedUserCount int64

var cachedDBMSVersion string

// previously fetched notices
var cachedNotices model.ProductNotices
var rcStripRegexp = regexp.MustCompile(`(.*?)(-rc\d+)(.*?)`)

func cleanupVersion(originalVersion string) string {
	// clean up BuildNumber to remove release- prefix, -rc suffix and a hash part of the version
	version := strings.Replace(originalVersion, "release-", "", 1)
	version = rcStripRegexp.ReplaceAllString(version, `$1$3`)
	versionParts := strings.Split(version, ".")
	var versionPartsOut []string
	for _, part := range versionParts {
		if _, err := strconv.ParseInt(part, 10, 16); err == nil {
			versionPartsOut = append(versionPartsOut, part)
		}
	}
	return strings.Join(versionPartsOut, ".")
}

func noticeMatchesConditions(config *model.Config, preferences store.PreferenceStore, userID string,
	client model.NoticeClientType, clientVersion string, postCount int64, userCount int64, isSystemAdmin bool,
	isTeamAdmin bool, isCloud bool, sku, dbName, dbVer, searchEngineName, searchEngineVer string,
	notice *model.ProductNotice) (bool, error) {
	cnd := notice.Conditions

	// check client type
	if cnd.ClientType != nil {
		if !cnd.ClientType.Matches(client) {
			return false, nil
		}
	}

	// check if client version is in notice range
	clientVersions := cnd.DesktopVersion
	if client == model.NoticeClientTypeMobileAndroid || client == model.NoticeClientTypeMobileIos {
		clientVersions = cnd.MobileVersion
	}

	clientVersionParsed, err := semver.NewVersion(clientVersion)
	if err != nil {
		return false, errors.Wrapf(err, "Cannot parse version range %s", clientVersion)
	}

	for _, v := range clientVersions {
		c, err2 := semver.NewConstraint(v)
		if err2 != nil {
			return false, errors.Wrapf(err2, "Cannot parse version range %s", v)
		}
		if !c.Check(clientVersionParsed) {
			return false, nil
		}
	}

	// check if notice date range matches current
	if cnd.DisplayDate != nil {
		y, m, d := time.Now().UTC().Date()
		trunc := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		c, err2 := date_constraints.NewConstraint(*cnd.DisplayDate)
		if err2 != nil {
			return false, errors.Wrapf(err2, "Cannot parse date range %s", *cnd.DisplayDate)
		}
		if !c.Check(&trunc) {
			return false, nil
		}
	}

	// check if current server version is notice range
	if !isCloud && cnd.ServerVersion != nil {
		version := cleanupVersion(model.BuildNumber)
		serverVersion, err := semver.NewVersion(version)
		if err != nil {
			mlog.Warn("Build number is not in semver format", mlog.String("build_number", version))
			return false, nil
		}
		for _, v := range cnd.ServerVersion {
			c, err := semver.NewConstraint(v)
			if err != nil {
				return false, errors.Wrapf(err, "Cannot parse version range %s", v)
			}
			if !c.Check(serverVersion) {
				return false, nil
			}
		}
	}

	// check if sku matches our license
	if cnd.Sku != nil {
		if !cnd.Sku.Matches(sku) {
			return false, nil
		}
	}

	// check the target audience
	if cnd.Audience != nil {
		if !cnd.Audience.Matches(isSystemAdmin, isTeamAdmin) {
			return false, nil
		}

	}

	// check user count condition against previously calculated total user count
	if cnd.NumberOfUsers != nil && userCount > 0 {
		if userCount < *cnd.NumberOfUsers {
			return false, nil
		}
	}

	// check post count condition against previously calculated total post count
	if cnd.NumberOfPosts != nil && postCount > 0 {
		if postCount < *cnd.NumberOfPosts {
			return false, nil
		}
	}

	if cnd.DeprecatingDependency != nil {
		extDepVersion, err := semver.NewVersion(cnd.DeprecatingDependency.MinimumVersion)
		if err != nil {
			return false, errors.Wrapf(err, "Cannot parse external dependency version %s", cnd.DeprecatingDependency.MinimumVersion)
		}

		switch cnd.DeprecatingDependency.Name {
		case model.DatabaseDriverMysql, model.DatabaseDriverPostgres:
			if dbName != cnd.DeprecatingDependency.Name {
				return false, nil
			}
			serverDBMSVersion, err := semver.NewVersion(dbVer)
			if err != nil {
				return false, errors.Wrapf(err, "Cannot parse DBMS version %s", dbVer)
			}
			return extDepVersion.GreaterThan(serverDBMSVersion), nil
		case model.SearchengineElasticsearch:
			if searchEngineName != model.SearchengineElasticsearch {
				return false, nil
			}
			semverESVersion, err := semver.NewVersion(searchEngineVer)
			if err != nil {
				return false, errors.Wrapf(err, "Cannot parse search engine version %s", searchEngineVer)
			}
			return extDepVersion.GreaterThan(semverESVersion), nil
		default:
			return false, nil
		}
	}

	// check if our server config matches the notice
	for k, v := range cnd.ServerConfig {
		if !validateConfigEntry(config, k, v) {
			return false, nil
		}
	}

	// check if user's config matches the notice
	for k, v := range cnd.UserConfig {
		res, err := validateUserConfigEntry(preferences, userID, k, v)
		if err != nil {
			return false, err
		}
		if !res {
			return false, nil
		}
	}

	// check the type of installation
	if cnd.InstanceType != nil {
		if !cnd.InstanceType.Matches(isCloud) {
			return false, nil
		}
	}

	return true, nil
}

func validateUserConfigEntry(preferences store.PreferenceStore, userID string, key string, expectedValue interface{}) (bool, error) {
	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return false, errors.New("Invalid format of user config. Must be in form of Category.SettingName")
	}
	if _, ok := expectedValue.(string); !ok {
		return false, errors.New("Invalid format of user config. Value should be string")
	}
	pref, err := preferences.Get(userID, parts[0], parts[1])
	if err != nil {
		return false, nil
	}
	return pref.Value == expectedValue, nil
}

func validateConfigEntry(conf *model.Config, path string, expectedValue interface{}) bool {
	value, found := config.GetValueByPath(strings.Split(path, "."), *conf)
	if !found {
		return false
	}
	vt := reflect.ValueOf(value)
	if vt.IsNil() {
		return expectedValue == nil
	}
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}
	val := vt.Interface()
	return val == expectedValue
}

// GetProductNotices is called from the frontend to fetch the product notices that are relevant to the caller
func (a *App) GetProductNotices(c *request.Context, userID, teamID string, client model.NoticeClientType, clientVersion string, locale string) (model.NoticeMessages, *model.AppError) {
	isSystemAdmin := a.SessionHasPermissionTo(*c.Session(), model.PermissionManageSystem)
	isTeamAdmin := a.SessionHasPermissionToTeam(*c.Session(), teamID, model.PermissionManageTeam)

	// check if notices for regular users are disabled
	if !*a.Srv().Config().AnnouncementSettings.UserNoticesEnabled && !isSystemAdmin {
		return []model.NoticeMessage{}, nil
	}

	// check if notices for admins are disabled
	if !*a.Srv().Config().AnnouncementSettings.AdminNoticesEnabled && (isTeamAdmin || isSystemAdmin) {
		return []model.NoticeMessage{}, nil
	}

	views, err := a.Srv().Store.ProductNotices().GetViews(userID)
	if err != nil {
		return nil, model.NewAppError("GetProductNotices", "api.system.update_viewed_notices.failed", nil, err.Error(), http.StatusBadRequest)
	}

	sku := a.Srv().ClientLicense()["SkuShortName"]
	isCloud := a.Srv().License() != nil && *a.Srv().License().Features.Cloud
	dbName := *a.Srv().Config().SqlSettings.DriverName

	var searchEngineName, searchEngineVersion string
	if engine := a.Srv().SearchEngine; engine != nil && engine.ElasticsearchEngine != nil {
		searchEngineName = engine.ElasticsearchEngine.GetName()
		searchEngineVersion = engine.ElasticsearchEngine.GetFullVersion()
	}

	filteredNotices := make([]model.NoticeMessage, 0)

	for noticeIndex, notice := range cachedNotices {
		// check if the notice has been viewed already
		var view *model.ProductNoticeViewState
		for viewIndex, v := range views {
			if v.NoticeID == notice.ID {
				view = &views[viewIndex]
				break
			}
		}
		if view != nil {
			repeatable := notice.Repeatable != nil && *notice.Repeatable
			if repeatable {
				if view.Viewed > MaxRepeatViewings {
					continue
				}
				if (time.Now().UTC().Unix() - view.Timestamp) < MinSecondsBetweenRepeatViewings {
					continue
				}
			} else if view.Viewed > 0 {
				continue
			}
		}
		result, err := noticeMatchesConditions(
			a.Config(),
			a.Srv().Store.Preference(),
			userID,
			client,
			clientVersion,
			cachedPostCount,
			cachedUserCount,
			isSystemAdmin,
			isTeamAdmin,
			isCloud,
			sku,
			dbName,
			cachedDBMSVersion,
			searchEngineName,
			searchEngineVersion,
			&cachedNotices[noticeIndex])
		if err != nil {
			return nil, model.NewAppError("GetProductNotices", "api.system.update_notices.validating_failed", nil, err.Error(), http.StatusBadRequest)
		}
		if result {
			selectedLocale := "en"
			filteredNotices = append(filteredNotices, model.NoticeMessage{
				NoticeMessageInternal: notice.LocalizedMessages[selectedLocale],
				ID:                    notice.ID,
				TeamAdminOnly:         notice.TeamAdminOnly(),
				SysAdminOnly:          notice.SysAdminOnly(),
			})
		}
	}

	return filteredNotices, nil
}

// UpdateViewedProductNotices is called from the frontend to mark a set of notices as 'viewed' by user
func (a *App) UpdateViewedProductNotices(userID string, noticeIDs []string) *model.AppError {
	if err := a.Srv().Store.ProductNotices().View(userID, noticeIDs); err != nil {
		return model.NewAppError("UpdateViewedProductNotices", "api.system.update_viewed_notices.failed", nil, err.Error(), http.StatusBadRequest)
	}
	return nil
}

// UpdateViewedProductNoticesForNewUser is called when new user is created to mark all current notices for this
// user as viewed in order to avoid showing them imminently on first login
func (a *App) UpdateViewedProductNoticesForNewUser(userID string) {
	var noticeIDs []string
	for _, notice := range cachedNotices {
		noticeIDs = append(noticeIDs, notice.ID)
	}
	if err := a.Srv().Store.ProductNotices().View(userID, noticeIDs); err != nil {
		mlog.Error("Cannot update product notices viewed state for user", mlog.String("userId", userID))
	}
}

// UpdateProductNotices is called periodically from a scheduled worker to fetch new notices and update the cache
func (a *App) UpdateProductNotices() *model.AppError {
	url := *a.Srv().Config().AnnouncementSettings.NoticesURL
	skip := *a.Srv().Config().AnnouncementSettings.NoticesSkipCache
	mlog.Debug("Will fetch notices from", mlog.String("url", url), mlog.Bool("skip_cache", skip))
	var err error
	cachedPostCount, err = a.Srv().Store.Post().AnalyticsPostCount("", false, false)
	if err != nil {
		mlog.Warn("Failed to fetch post count", mlog.String("error", err.Error()))
	}

	cachedUserCount, err = a.Srv().Store.User().Count(model.UserCountOptions{IncludeDeleted: true})
	if err != nil {
		mlog.Warn("Failed to fetch user count", mlog.String("error", err.Error()))
	}

	cachedDBMSVersion, err = a.Srv().Store.GetDbVersion(false)
	if err != nil {
		mlog.Warn("Failed to get DBMS version", mlog.String("error", err.Error()))
	}

	cachedDBMSVersion = strings.Split(cachedDBMSVersion, " ")[0] // get rid of trailing strings attached to the version

	data, err := utils.GetURLWithCache(url, &noticesCache, skip)
	if err != nil {
		return model.NewAppError("UpdateProductNotices", "api.system.update_notices.fetch_failed", nil, err.Error(), http.StatusBadRequest)
	}
	cachedNotices, err = model.UnmarshalProductNotices(data)
	if err != nil {
		return model.NewAppError("UpdateProductNotices", "api.system.update_notices.parse_failed", nil, err.Error(), http.StatusBadRequest)
	}

	if err := a.Srv().Store.ProductNotices().ClearOldNotices(&cachedNotices); err != nil {
		return model.NewAppError("UpdateProductNotices", "api.system.update_notices.clear_failed", nil, err.Error(), http.StatusBadRequest)
	}
	return nil
}
