// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime"
	"strconv"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mail"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	PropSecurityURL      = "https://securityupdatecheck.mattermost.com"
	SecurityUpdatePeriod = 86400000 // 24 hours in milliseconds.

	PropSecurityID              = "id"
	PropSecurityBuild           = "b"
	PropSecurityEnterpriseReady = "be"
	PropSecurityDatabase        = "db"
	PropSecurityOS              = "os"
	PropSecurityUserCount       = "uc"
	PropSecurityTeamCount       = "tc"
	PropSecurityActiveUserCount = "auc"
	PropSecurityUnitTests       = "ut"
)

func (s *Server) DoSecurityUpdateCheck() {
	if !*s.Config().ServiceSettings.EnableSecurityFixAlert {
		return
	}

	props, err := s.Store.System().Get()
	if err != nil {
		return
	}

	lastSecurityTime, _ := strconv.ParseInt(props[model.SystemLastSecurityTime], 10, 0)
	currentTime := model.GetMillis()

	if (currentTime - lastSecurityTime) > SecurityUpdatePeriod {
		mlog.Debug("Checking for security update from Mattermost")

		v := url.Values{}

		v.Set(PropSecurityID, s.TelemetryID())
		v.Set(PropSecurityBuild, model.CurrentVersion+"."+model.BuildNumber)
		v.Set(PropSecurityEnterpriseReady, model.BuildEnterpriseReady)
		v.Set(PropSecurityDatabase, *s.Config().SQLSettings.DriverName)
		v.Set(PropSecurityOS, runtime.GOOS)

		if props[model.SystemRanUnitTests] != "" {
			v.Set(PropSecurityUnitTests, "1")
		} else {
			v.Set(PropSecurityUnitTests, "0")
		}

		systemSecurityLastTime := &model.System{Name: model.SystemLastSecurityTime, Value: strconv.FormatInt(currentTime, 10)}
		if lastSecurityTime == 0 {
			s.Store.System().Save(systemSecurityLastTime)
		} else {
			s.Store.System().Update(systemSecurityLastTime)
		}

		if count, err := s.Store.User().Count(model.UserCountOptions{IncludeDeleted: true}); err == nil {
			v.Set(PropSecurityUserCount, strconv.FormatInt(count, 10))
		}

		if ucr, err := s.Store.Status().GetTotalActiveUsersCount(); err == nil {
			v.Set(PropSecurityActiveUserCount, strconv.FormatInt(ucr, 10))
		}

		if teamCount, err := s.Store.Team().AnalyticsTeamCount(nil); err == nil {
			v.Set(PropSecurityTeamCount, strconv.FormatInt(teamCount, 10))
		}

		res, err := http.Get(PropSecurityURL + "/security?" + v.Encode())
		if err != nil {
			mlog.Error("Failed to get security update information from Mattermost.")
			return
		}

		defer res.Body.Close()

		bulletins := model.SecurityBulletinsFromJSON(res.Body)

		for _, bulletin := range bulletins {
			if bulletin.AppliesToVersion == model.CurrentVersion {
				if props["SecurityBulletin_"+bulletin.ID] == "" {
					users, userErr := s.Store.User().GetSystemAdminProfiles()
					if userErr != nil {
						mlog.Error("Failed to get system admins for security update information from Mattermost.")
						return
					}

					resBody, err := http.Get(PropSecurityURL + "/bulletins/" + bulletin.ID)
					if err != nil {
						mlog.Error("Failed to get security bulletin details")
						return
					}

					body, err := ioutil.ReadAll(resBody.Body)
					resBody.Body.Close()
					if err != nil || resBody.StatusCode != 200 {
						mlog.Error("Failed to read security bulletin details")
						return
					}

					for _, user := range users {
						mlog.Info("Sending security bulletin", mlog.String("bulletin_id", bulletin.ID), mlog.String("user_email", user.Email))
						license := s.License()
						mailConfig := s.MailServiceConfig()
						mail.SendMailUsingConfig(user.Email, i18n.T("mattermost.bulletin.subject"), string(body), mailConfig, license != nil && *license.Features.Compliance, "")
					}

					bulletinSeen := &model.System{Name: "SecurityBulletin_" + bulletin.ID, Value: bulletin.ID}
					s.Store.System().Save(bulletinSeen)
				}
			}
		}
	}
}
