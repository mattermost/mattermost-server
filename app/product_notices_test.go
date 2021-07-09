// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

func TestNoticeValidation(t *testing.T) {
	th := SetupWithStoreMock(t)
	mockStore := th.App.Srv().Store.(*mocks.Store)
	mockRoleStore := mocks.RoleStore{}
	mockSystemStore := mocks.SystemStore{}
	mockUserStore := mocks.UserStore{}
	mockPostStore := mocks.PostStore{}
	mockPreferenceStore := mocks.PreferenceStore{}
	mockStore.On("Role").Return(&mockRoleStore)
	mockStore.On("System").Return(&mockSystemStore)
	mockStore.On("User").Return(&mockUserStore)
	mockStore.On("Post").Return(&mockPostStore)
	mockStore.On("Preference").Return(&mockPreferenceStore)
	mockSystemStore.On("SaveOrUpdate", &model.System{Name: "ActiveLicenseId", Value: ""}).Return(nil)
	mockSystemStore.On("GetByName", "UpgradedFromTE").Return(&model.System{Name: "UpgradedFromTE", Value: "false"}, nil)
	mockSystemStore.On("GetByName", "InstallationDate").Return(&model.System{Name: "InstallationDate", Value: "10"}, nil)
	mockSystemStore.On("GetByName", "FirstServerRunTimestamp").Return(&model.System{Name: "FirstServerRunTimestamp", Value: "10"}, nil)
	mockSystemStore.On("Get").Return(make(model.StringMap), nil)

	mockUserStore.On("Count", model.UserCountOptions{IncludeBotAccounts: false, IncludeDeleted: true, ExcludeRegularUsers: false, TeamID: "", ChannelID: "", ViewRestrictions: (*model.ViewUsersRestrictions)(nil), Roles: []string(nil), ChannelRoles: []string(nil), TeamRoles: []string(nil)}).Return(int64(1), nil)
	mockPreferenceStore.On("Get", "test", "Stuff", "Data").Return(&model.Preference{Value: "test2"}, nil)
	mockPreferenceStore.On("Get", "test", "Stuff", "Data2").Return(&model.Preference{Value: "test"}, nil)
	mockPreferenceStore.On("Get", "test", "Stuff", "Data3").Return(nil, errors.New("Error!"))
	mockPostStore.On("GetMaxPostSize").Return(65535, nil)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.AnnouncementSettings.AdminNoticesEnabled = true
		*cfg.AnnouncementSettings.UserNoticesEnabled = true
	})

	defer th.TearDown()

	type args struct {
		client               model.NoticeClientType
		clientVersion        string
		sku                  string
		postCount, userCount int64
		cloud                bool
		teamAdmin            bool
		systemAdmin          bool
		serverVersion        string
		notice               *model.ProductNotice
		dbmsName             string
		dbmsVer              string
		searchEngineName     string
		searchEngineVer      string
	}
	messages := map[string]model.NoticeMessageInternal{
		"en": {
			Description: "descr",
			Title:       "title",
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		wantOk  bool
	}{
		{
			name: "general notice",
			args: args{
				client:        "mobile",
				clientVersion: "1.2.3",

				notice: &model.ProductNotice{
					Conditions:        model.Conditions{},
					ID:                "123",
					LocalizedMessages: messages,
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "mobile notice",
			args: args{
				client:        "desktop",
				clientVersion: "1.2.3",

				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ClientType: model.NewNoticeClientType(model.NoticeClientTypeMobile),
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with config check",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerConfig: map[string]interface{}{"ServiceSettings.LetsEncryptCertificateCacheFile": "./config/letsencrypt.cache"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with failing config check",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerConfig: map[string]interface{}{"ServiceSettings.ZZ": "test"},
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with failing user check due to bad format",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						UserConfig: map[string]interface{}{"Stuff": "test"},
					},
				},
			},
			wantErr: true,
			wantOk:  false,
		},
		{
			name: "notice with failing user check due to mismatch",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						UserConfig: map[string]interface{}{"Stuff.Data": "test"},
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with working user check",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						UserConfig: map[string]interface{}{"Stuff.Data2": "test"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with user check for property not in database",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						UserConfig: map[string]interface{}{"Stuff.Data3": "stuff"},
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with server version check",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 4.0.0 < 99.0.0"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with server version check that doesn't match",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 99.0.0"},
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with server version check that matches a const",
			args: args{
				serverVersion: "99.1.1",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 99.0.0"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},

		{
			name: "notice with server version check that matches a const",
			args: args{
				serverVersion: "99.1.1",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 99.0.0"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},

		{
			name: "notice with server version check that has rc",
			args: args{
				serverVersion: "99.1.1-rc2",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 99.0.0 < 100.2.2"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},

		{
			name: "notice with server version check that has rc and hash",
			args: args{
				serverVersion: "99.1.1-rc2.abcdef",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 99.0.0 < 100.2.2"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},

		{
			name: "notice with server version check that has release and hash",
			args: args{
				serverVersion: "release-99.1.1.abcdef",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 99.0.0 < 100.2.2"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},

		{
			name: "notice with server version check that has cloud version",
			args: args{
				serverVersion: "cloud.54.abcdef",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 99.0.0 < 100.2.2"},
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with server version check on cloud should ignore version",
			args: args{
				cloud:         true,
				serverVersion: "cloud.54.abcdef",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"> 99.0.0 < 100.2.2"},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},

		{
			name: "notice with server version check that is invalid",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{"99.0.0 + 1.0.0"},
					},
				},
			},
			wantErr: true,
			wantOk:  false,
		},
		{
			name: "notice with user count",
			args: args{
				userCount: 300,
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						NumberOfUsers: model.NewInt64(400),
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with good user count and bad post count",
			args: args{
				userCount: 500,
				postCount: 2000,
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						NumberOfUsers: model.NewInt64(400),
						NumberOfPosts: model.NewInt64(3000),
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with date check",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						DisplayDate: model.NewString("> 2000-03-01T00:00:00Z <= 2999-04-01T00:00:00Z"),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with specific date check",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						DisplayDate: model.NewString(fmt.Sprintf("= %sT00:00:00Z", time.Now().Format("2006-01-02"))),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with date check that doesn't match",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						DisplayDate: model.NewString("> 2999-03-01T00:00:00Z <= 3000-04-01T00:00:00Z"),
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with bad date check",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						DisplayDate: model.NewString("> 2000 -03-01T00:00:00Z <= 2999-04-01T00:00:00Z"),
					},
				},
			},
			wantErr: true,
			wantOk:  false,
		},
		{
			name: "notice with audience check (admin)",
			args: args{
				systemAdmin: true,
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Audience: model.NewNoticeAudience(model.NoticeAudienceSysadmin),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with failing audience check (admin)",
			args: args{
				systemAdmin: false,
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Audience: model.NewNoticeAudience(model.NoticeAudienceSysadmin),
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with audience check (team)",
			args: args{
				teamAdmin: true,
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Audience: model.NewNoticeAudience(model.NoticeAudienceTeamAdmin),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with failing audience check (team)",
			args: args{
				teamAdmin: false,
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Audience: model.NewNoticeAudience(model.NoticeAudienceTeamAdmin),
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with audience check (member)",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Audience: model.NewNoticeAudience(model.NoticeAudienceMember),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with failing audience check (member)",
			args: args{
				systemAdmin: true,
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Audience: model.NewNoticeAudience(model.NoticeAudienceMember),
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with correct sku",
			args: args{
				sku: "e20",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Sku: model.NewNoticeSKU(model.NoticeSKUE20),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with incorrect sku",
			args: args{
				sku: "e20",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Sku: model.NewNoticeSKU(model.NoticeSKUE10),
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice with team sku",
			args: args{
				sku: "",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Sku: model.NewNoticeSKU(model.NoticeSKUTeam),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with sku check for all",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						Sku: model.NewNoticeSKU(model.NoticeSKUAll),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with instance check cloud",
			args: args{
				cloud: true,
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						InstanceType: model.NewNoticeInstanceType(model.NoticeInstanceTypeCloud),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with instance check both",
			args: args{
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						InstanceType: model.NewNoticeInstanceType(model.NoticeInstanceTypeBoth),
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with depreacting an external dependency",
			args: args{
				dbmsName: "mysql",
				dbmsVer:  "5.6",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						DeprecatingDependency: &model.ExternalDependency{
							Name:           "mysql",
							MinimumVersion: "5.7",
						},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
		{
			name: "notice with depreacting an external dependency, on a future version",
			args: args{
				dbmsName:      "mysql",
				dbmsVer:       "5.6",
				serverVersion: "5.32",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						ServerVersion: []string{">=v5.33"},
						DeprecatingDependency: &model.ExternalDependency{
							Name:           "mysql",
							MinimumVersion: "5.7",
						},
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice on a deprecating dependency, server is all good",
			args: args{
				dbmsName: "postgres",
				dbmsVer:  "10",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						DeprecatingDependency: &model.ExternalDependency{
							Name:           "postgres",
							MinimumVersion: "10",
						},
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice on a deprecating dependency, server has different dbms",
			args: args{
				dbmsName: "mysql",
				dbmsVer:  "5.7",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						DeprecatingDependency: &model.ExternalDependency{
							Name:           "postgres",
							MinimumVersion: "10",
						},
					},
				},
			},
			wantErr: false,
			wantOk:  false,
		},
		{
			name: "notice on deprecating elasticsearch, server has unsupported search engine",
			args: args{
				searchEngineName: "elasticsearch",
				searchEngineVer:  "6.4.1",
				notice: &model.ProductNotice{
					Conditions: model.Conditions{
						DeprecatingDependency: &model.ExternalDependency{
							Name:           "elasticsearch",
							MinimumVersion: "7",
						},
					},
				},
			},
			wantErr: false,
			wantOk:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientVersion := tt.args.clientVersion
			if clientVersion == "" {
				clientVersion = "1.2.3"
			}
			model.BuildNumber = tt.args.serverVersion
			if model.BuildNumber == "" {
				model.BuildNumber = "5.26.1"
			}
			if ok, err := noticeMatchesConditions(
				th.App.Config(),
				th.App.Srv().Store.Preference(),
				"test",
				tt.args.client,
				clientVersion,
				tt.args.postCount,
				tt.args.userCount,
				tt.args.systemAdmin,
				tt.args.teamAdmin,
				tt.args.cloud,
				tt.args.sku,
				tt.args.dbmsName,
				tt.args.dbmsVer,
				tt.args.searchEngineName,
				tt.args.searchEngineVer,
				tt.args.notice,
			); (err != nil) != tt.wantErr {
				t.Errorf("noticeMatchesConditions() error = %v, wantErr %v", err, tt.wantErr)
			} else if ok != tt.wantOk {
				t.Errorf("noticeMatchesConditions() result = %v, wantOk %v", ok, tt.wantOk)
			}
		})
	}
}

func TestNoticeFetch(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	notices := model.ProductNotices{model.ProductNotice{
		Conditions: model.Conditions{},
		ID:         "123",
		LocalizedMessages: map[string]model.NoticeMessageInternal{
			"en": {
				Description: "description",
				Title:       "title",
			},
		},
		Repeatable: nil,
	}}
	noticesBytes, err := notices.Marshal()
	require.NoError(t, err)

	notices2 := model.ProductNotices{model.ProductNotice{
		Conditions: model.Conditions{
			NumberOfPosts: model.NewInt64(99999),
		},
		ID: "333",
		LocalizedMessages: map[string]model.NoticeMessageInternal{
			"en": {
				Description: "description",
				Title:       "title",
			},
		},
		Repeatable: nil,
	}}
	noticesBytes2, err := notices2.Marshal()
	require.NoError(t, err)
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "notices.json") {
			w.Write(noticesBytes)
		} else {
			w.Write(noticesBytes2)
		}
	}))
	defer server1.Close()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.AnnouncementSettings.AdminNoticesEnabled = true
		*cfg.AnnouncementSettings.UserNoticesEnabled = true
		*cfg.AnnouncementSettings.NoticesURL = fmt.Sprintf("http://%s/notices.json", server1.Listener.Addr().String())
	})

	// fetch fake notices
	appErr := th.App.UpdateProductNotices()
	require.Nil(t, appErr)

	// get them for specified user
	messages, appErr := th.App.GetProductNotices(&request.Context{}, th.BasicUser.ID, th.BasicTeam.ID, model.NoticeClientTypeAll, "1.2.3", "en")
	require.Nil(t, appErr)
	require.Len(t, messages, 1)

	// mark notices as viewed
	appErr = th.App.UpdateViewedProductNotices(th.BasicUser.ID, []string{messages[0].ID})
	require.Nil(t, appErr)

	// get them again, see that none are returned
	messages, appErr = th.App.GetProductNotices(&request.Context{}, th.BasicUser.ID, th.BasicTeam.ID, model.NoticeClientTypeAll, "1.2.3", "en")
	require.Nil(t, appErr)
	require.Len(t, messages, 0)

	// validate views table
	views, err := th.App.Srv().Store.ProductNotices().GetViews(th.BasicUser.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)

	// fetch another set
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.AnnouncementSettings.NoticesURL = fmt.Sprintf("http://%s/notices2.json", server1.Listener.Addr().String())
	})

	// fetch fake notices
	appErr = th.App.UpdateProductNotices()
	require.Nil(t, appErr)

	// get them again, since conditions don't match we should be zero
	messages, appErr = th.App.GetProductNotices(&request.Context{}, th.BasicUser.ID, th.BasicTeam.ID, model.NoticeClientTypeAll, "1.2.3", "en")
	require.Nil(t, appErr)
	require.Len(t, messages, 0)

	// even though UpdateViewedProductNotices was called previously, the table should be empty, since there's cleanup done during UpdateProductNotices
	views, err = th.App.Srv().Store.ProductNotices().GetViews(th.BasicUser.ID)
	require.NoError(t, err)
	require.Len(t, views, 0)
}
