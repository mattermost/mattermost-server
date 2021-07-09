// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func TestGetPing(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		t.Run("healthy", func(t *testing.T) {
			status, resp := client.GetPing()
			CheckNoError(t, resp)
			assert.Equal(t, model.StatusOk, status)
		})

		t.Run("unhealthy", func(t *testing.T) {
			goRoutineHealthThreshold := *th.App.Config().ServiceSettings.GoroutineHealthThreshold
			defer func() {
				th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.GoroutineHealthThreshold = goRoutineHealthThreshold })
			}()

			th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.GoroutineHealthThreshold = 10 })
			status, resp := client.GetPing()
			CheckInternalErrorStatus(t, resp)
			assert.Equal(t, model.StatusUnhealthy, status)
		})
	}, "basic ping")

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		t.Run("healthy", func(t *testing.T) {
			status, resp := client.GetPingWithServerStatus()

			CheckNoError(t, resp)
			assert.Equal(t, model.StatusOk, status)
		})

		t.Run("unhealthy", func(t *testing.T) {
			oldDriver := th.App.Config().FileSettings.DriverName
			badDriver := "badDriverName"
			th.App.Config().FileSettings.DriverName = &badDriver
			defer func() {
				th.App.Config().FileSettings.DriverName = oldDriver
			}()

			status, resp := client.GetPingWithServerStatus()
			CheckInternalErrorStatus(t, resp)
			assert.Equal(t, model.StatusUnhealthy, status)
		})
	}, "with server status")

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		th.App.ReloadConfig()
		resp, appErr := client.DoApiGet(client.GetSystemRoute()+"/ping", "")
		require.Nil(t, appErr)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		respBytes, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		respString := string(respBytes)
		require.NotContains(t, respString, "TestFeatureFlag")

		// Run the environment variable override code to test
		os.Setenv("MM_FEATUREFLAGS_TESTFEATURE", "testvalueunique")
		defer os.Unsetenv("MM_FEATUREFLAGS_TESTFEATURE")
		th.App.ReloadConfig()

		resp, appErr = client.DoApiGet(client.GetSystemRoute()+"/ping", "")
		require.Nil(t, appErr)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		respBytes, err = ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		respString = string(respBytes)
		require.Contains(t, respString, "testvalue")
	}, "ping feature flag test")
}

func TestGetAudits(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	audits, resp := th.SystemAdminClient.GetAudits(0, 100, "")
	CheckNoError(t, resp)
	require.NotEmpty(t, audits, "should not be empty")

	audits, resp = th.SystemAdminClient.GetAudits(0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, audits, 1, "should only be 1")

	audits, resp = th.SystemAdminClient.GetAudits(1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, audits, 1, "should only be 1")

	_, resp = th.SystemAdminClient.GetAudits(-1, -1, "")
	CheckNoError(t, resp)

	_, resp = Client.GetAudits(0, 100, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetAudits(0, 100, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestEmailTest(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	config := model.Config{
		ServiceSettings: model.ServiceSettings{
			SiteURL: model.NewString(""),
		},
		EmailSettings: model.EmailSettings{
			SMTPServer:                        model.NewString(""),
			SMTPPort:                          model.NewString(""),
			SMTPPassword:                      model.NewString(""),
			FeedbackName:                      model.NewString(""),
			FeedbackEmail:                     model.NewString("some-addr@test.com"),
			ReplyToAddress:                    model.NewString("some-addr@test.com"),
			ConnectionSecurity:                model.NewString(""),
			SMTPUsername:                      model.NewString(""),
			EnableSMTPAuth:                    model.NewBool(false),
			SkipServerCertificateVerification: model.NewBool(true),
			SendEmailNotifications:            model.NewBool(false),
			SMTPServerTimeout:                 model.NewInt(15),
		},
		FileSettings: model.FileSettings{
			DriverName: model.NewString(model.ImageDriverLocal),
			Directory:  model.NewString(dir),
		},
	}

	t.Run("as system user", func(t *testing.T) {
		_, resp := Client.TestEmail(&config)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.TestEmail(&config)
		CheckErrorMessage(t, resp, "api.admin.test_email.missing_server")
		CheckBadRequestStatus(t, resp)

		inbucket_host := os.Getenv("CI_INBUCKET_HOST")
		if inbucket_host == "" {
			inbucket_host = "localhost"
		}

		inbucket_port := os.Getenv("CI_INBUCKET_SMTP_PORT")
		if inbucket_port == "" {
			inbucket_port = "10025"
		}

		*config.EmailSettings.SMTPServer = inbucket_host
		*config.EmailSettings.SMTPPort = inbucket_port
		_, resp = th.SystemAdminClient.TestEmail(&config)
		CheckOKStatus(t, resp)
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		_, resp := th.SystemAdminClient.TestEmail(&config)
		CheckForbiddenStatus(t, resp)
	})
}

func TestGenerateSupportPacket(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	t.Run("As a System Administrator", func(t *testing.T) {
		l := model.NewTestLicense()
		th.App.Srv().SetLicense(l)

		file, resp := th.SystemAdminClient.GenerateSupportPacket()
		require.Nil(t, resp.Error)
		require.NotZero(t, len(file))
	})

	t.Run("As a Regular User", func(t *testing.T) {
		_, resp := th.Client.GenerateSupportPacket()
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Server with no License", func(t *testing.T) {
		ok, resp := th.SystemAdminClient.RemoveLicenseFile()
		CheckNoError(t, resp)
		require.True(t, ok)

		_, resp = th.SystemAdminClient.GenerateSupportPacket()
		CheckForbiddenStatus(t, resp)
	})
}

func TestSiteURLTest(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/valid/api/v4/system/ping") {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(400)
		}
	}))
	defer ts.Close()

	validSiteURL := ts.URL + "/valid"
	invalidSiteURL := ts.URL + "/invalid"

	t.Run("as system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.TestSiteURL("")
		CheckBadRequestStatus(t, resp)

		_, resp = th.SystemAdminClient.TestSiteURL(invalidSiteURL)
		CheckBadRequestStatus(t, resp)

		_, resp = th.SystemAdminClient.TestSiteURL(validSiteURL)
		CheckOKStatus(t, resp)
	})

	t.Run("as system user", func(t *testing.T) {
		_, resp := Client.TestSiteURL(validSiteURL)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		_, resp := Client.TestSiteURL(validSiteURL)
		CheckForbiddenStatus(t, resp)
	})
}

func TestDatabaseRecycle(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	t.Run("as system user", func(t *testing.T) {
		_, resp := Client.DatabaseRecycle()
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.DatabaseRecycle()
		CheckNoError(t, resp)
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		_, resp := th.SystemAdminClient.DatabaseRecycle()
		CheckForbiddenStatus(t, resp)
	})
}

func TestInvalidateCaches(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	t.Run("as system user", func(t *testing.T) {
		ok, resp := Client.InvalidateCaches()
		CheckForbiddenStatus(t, resp)
		require.False(t, ok, "should not clean the cache due to no permission.")
	})

	t.Run("as system admin", func(t *testing.T) {
		ok, resp := th.SystemAdminClient.InvalidateCaches()
		CheckNoError(t, resp)
		require.True(t, ok, "should clean the cache")
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		ok, resp := th.SystemAdminClient.InvalidateCaches()
		CheckForbiddenStatus(t, resp)
		require.False(t, ok, "should not clean the cache due to no permission.")
	})
}

func TestGetLogs(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	for i := 0; i < 20; i++ {
		mlog.Info(strconv.Itoa(i))
	}

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		logs, resp := c.GetLogs(0, 10)
		CheckNoError(t, resp)
		require.Len(t, logs, 10)

		for i := 10; i < 20; i++ {
			assert.Containsf(t, logs[i-10], fmt.Sprintf(`"msg":"%d"`, i), "Log line doesn't contain correct message")
		}

		logs, resp = c.GetLogs(1, 10)
		CheckNoError(t, resp)
		require.Len(t, logs, 10)

		logs, resp = c.GetLogs(-1, -1)
		CheckNoError(t, resp)
		require.NotEmpty(t, logs, "should not be empty")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })
		_, resp := th.Client.GetLogs(0, 10)
		CheckForbiddenStatus(t, resp)
	})

	_, resp := th.Client.GetLogs(0, 10)
	CheckForbiddenStatus(t, resp)

	th.Client.Logout()
	_, resp = th.Client.GetLogs(0, 10)
	CheckUnauthorizedStatus(t, resp)
}

func TestPostLog(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	enableDev := *th.App.Config().ServiceSettings.EnableDeveloper
	defer func() {
		*th.App.Config().ServiceSettings.EnableDeveloper = enableDev
	}()
	*th.App.Config().ServiceSettings.EnableDeveloper = true

	message := make(map[string]string)
	message["level"] = "ERROR"
	message["message"] = "this is a test"

	_, resp := Client.PostLog(message)
	CheckNoError(t, resp)

	*th.App.Config().ServiceSettings.EnableDeveloper = false

	_, resp = Client.PostLog(message)
	CheckNoError(t, resp)

	*th.App.Config().ServiceSettings.EnableDeveloper = true

	Client.Logout()

	_, resp = Client.PostLog(message)
	CheckNoError(t, resp)

	*th.App.Config().ServiceSettings.EnableDeveloper = false

	_, resp = Client.PostLog(message)
	CheckForbiddenStatus(t, resp)

	logMessage, resp := th.SystemAdminClient.PostLog(message)
	CheckNoError(t, resp)
	require.NotEmpty(t, logMessage, "should return the log message")

}

func TestGetAnalyticsOld(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	rows, resp := Client.GetAnalyticsOld("", "")
	CheckForbiddenStatus(t, resp)
	require.Nil(t, rows, "should be nil")
	rows, resp = th.SystemAdminClient.GetAnalyticsOld("", "")
	CheckNoError(t, resp)

	found := false
	found2 := false
	for _, row := range rows {
		if row.Name == "unique_user_count" {
			found = true
		} else if row.Name == "inactive_user_count" {
			found2 = true
			assert.True(t, row.Value >= 0)
		}
	}

	assert.True(t, found, "should return unique user count")
	assert.True(t, found2, "should return inactive user count")

	_, resp = th.SystemAdminClient.GetAnalyticsOld("post_counts_day", "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetAnalyticsOld("user_counts_with_posts_day", "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetAnalyticsOld("extra_counts", "")
	CheckNoError(t, resp)

	rows, resp = th.SystemAdminClient.GetAnalyticsOld("", th.BasicTeam.ID)
	CheckNoError(t, resp)

	for _, row := range rows {
		if row.Name == "inactive_user_count" {
			assert.Equal(t, float64(-1), row.Value, "inactive user count should be -1 when team specified")
		}
	}

	rows2, resp2 := th.SystemAdminClient.GetAnalyticsOld("standard", "")
	CheckNoError(t, resp2)
	assert.Equal(t, "total_websocket_connections", rows2[5].Name)
	assert.Equal(t, float64(0), rows2[5].Value)

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	time.Sleep(100 * time.Millisecond)
	rows2, resp2 = th.SystemAdminClient.GetAnalyticsOld("standard", "")
	CheckNoError(t, resp2)
	assert.Equal(t, "total_websocket_connections", rows2[5].Name)
	assert.Equal(t, float64(1), rows2[5].Value)

	WebSocketClient.Close()

	rows2, resp2 = th.SystemAdminClient.GetAnalyticsOld("standard", "")
	CheckNoError(t, resp2)
	assert.Equal(t, "total_websocket_connections", rows2[5].Name)
	assert.Equal(t, float64(0), rows2[5].Value)

	Client.Logout()
	_, resp = Client.GetAnalyticsOld("", th.BasicTeam.ID)
	CheckUnauthorizedStatus(t, resp)
}

func TestS3TestConnection(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	s3Host := os.Getenv("CI_MINIO_HOST")
	if s3Host == "" {
		s3Host = "localhost"
	}

	s3Port := os.Getenv("CI_MINIO_PORT")
	if s3Port == "" {
		s3Port = "9000"
	}

	s3Endpoint := fmt.Sprintf("%s:%s", s3Host, s3Port)
	config := model.Config{
		FileSettings: model.FileSettings{
			DriverName:              model.NewString(model.ImageDriverS3),
			AmazonS3AccessKeyID:     model.NewString(model.MinioAccessKey),
			AmazonS3SecretAccessKey: model.NewString(model.MinioSecretKey),
			AmazonS3Bucket:          model.NewString(""),
			AmazonS3Endpoint:        model.NewString(s3Endpoint),
			AmazonS3Region:          model.NewString(""),
			AmazonS3PathPrefix:      model.NewString(""),
			AmazonS3SSL:             model.NewBool(false),
		},
	}

	t.Run("as system user", func(t *testing.T) {
		_, resp := Client.TestS3Connection(&config)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.TestS3Connection(&config)
		CheckBadRequestStatus(t, resp)
		require.Equal(t, resp.Error.Message, "S3 Bucket is required", "should return error - missing s3 bucket")
		// If this fails, check the test configuration to ensure minio is setup with the
		// `mattermost-test` bucket defined by model.MINIO_BUCKET.
		*config.FileSettings.AmazonS3Bucket = model.MinioBucket
		config.FileSettings.AmazonS3PathPrefix = model.NewString("")
		*config.FileSettings.AmazonS3Region = "us-east-1"
		_, resp = th.SystemAdminClient.TestS3Connection(&config)
		CheckOKStatus(t, resp)

		config.FileSettings.AmazonS3Region = model.NewString("")
		_, resp = th.SystemAdminClient.TestS3Connection(&config)
		CheckOKStatus(t, resp)

		config.FileSettings.AmazonS3Bucket = model.NewString("Wrong_bucket")
		_, resp = th.SystemAdminClient.TestS3Connection(&config)
		CheckInternalErrorStatus(t, resp)
		assert.Equal(t, "api.file.test_connection_s3_bucket_does_not_exist.app_error", resp.Error.ID)

		*config.FileSettings.AmazonS3Bucket = "shouldnotcreatenewbucket"
		_, resp = th.SystemAdminClient.TestS3Connection(&config)
		CheckInternalErrorStatus(t, resp)
		assert.Equal(t, "api.file.test_connection_s3_bucket_does_not_exist.app_error", resp.Error.ID)
	})

	t.Run("with incorrect credentials", func(t *testing.T) {
		configCopy := config
		*configCopy.FileSettings.AmazonS3AccessKeyID = "invalidaccesskey"
		_, resp := th.SystemAdminClient.TestS3Connection(&configCopy)
		CheckInternalErrorStatus(t, resp)
		assert.Equal(t, "api.file.test_connection_s3_auth.app_error", resp.Error.ID)
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		_, resp := th.SystemAdminClient.TestS3Connection(&config)
		CheckForbiddenStatus(t, resp)
	})
}

func TestSupportedTimezones(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	supportedTimezonesFromConfig := th.App.Timezones().GetSupported()
	supportedTimezones, resp := Client.GetSupportedTimezone()

	CheckNoError(t, resp)
	assert.Equal(t, supportedTimezonesFromConfig, supportedTimezones)
}

func TestRedirectLocation(t *testing.T) {
	expected := "https://mattermost.com/wp-content/themes/mattermostv2/img/logo-light.svg"

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Location", expected)
		res.WriteHeader(http.StatusFound)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	mockBitlyLink := testServer.URL

	th := Setup(t)
	defer th.TearDown()
	Client := th.Client
	enableLinkPreviews := *th.App.Config().ServiceSettings.EnableLinkPreviews
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableLinkPreviews = enableLinkPreviews })
	}()

	*th.App.Config().ServiceSettings.EnableLinkPreviews = true
	*th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.1"

	_, resp := th.SystemAdminClient.GetRedirectLocation("https://mattermost.com/", "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetRedirectLocation("", "")
	CheckBadRequestStatus(t, resp)

	actual, resp := th.SystemAdminClient.GetRedirectLocation(mockBitlyLink, "")
	CheckNoError(t, resp)
	assert.Equal(t, expected, actual)

	// Check cached value
	actual, resp = th.SystemAdminClient.GetRedirectLocation(mockBitlyLink, "")
	CheckNoError(t, resp)
	assert.Equal(t, expected, actual)

	*th.App.Config().ServiceSettings.EnableLinkPreviews = false
	actual, resp = th.SystemAdminClient.GetRedirectLocation("https://mattermost.com/", "")
	CheckNoError(t, resp)
	assert.Equal(t, actual, "")

	actual, resp = th.SystemAdminClient.GetRedirectLocation("", "")
	CheckNoError(t, resp)
	assert.Equal(t, actual, "")

	actual, resp = th.SystemAdminClient.GetRedirectLocation(mockBitlyLink, "")
	CheckNoError(t, resp)
	assert.Equal(t, actual, "")

	Client.Logout()
	_, resp = Client.GetRedirectLocation("", "")
	CheckUnauthorizedStatus(t, resp)
}

func TestSetServerBusy(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	const secs = 30

	t.Run("as system user", func(t *testing.T) {
		ok, resp := th.Client.SetServerBusy(secs)
		CheckForbiddenStatus(t, resp)
		require.False(t, ok, "should not set server busy due to no permission")
		require.False(t, th.App.Srv().Busy.IsBusy(), "server should not be marked busy")
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		ok, resp := c.SetServerBusy(secs)
		CheckNoError(t, resp)
		require.True(t, ok, "should set server busy successfully")
		require.True(t, th.App.Srv().Busy.IsBusy(), "server should be marked busy")
	}, "as system admin")
}

func TestSetServerBusyInvalidParam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		params := []int{-1, 0, MaxServerBusySeconds + 1}
		for _, p := range params {
			ok, resp := c.SetServerBusy(p)
			CheckBadRequestStatus(t, resp)
			require.False(t, ok, "should not set server busy due to invalid param ", p)
			require.False(t, th.App.Srv().Busy.IsBusy(), "server should not be marked busy due to invalid param ", p)
		}
	}, "as system admin, invalid param")
}

func TestClearServerBusy(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().Busy.Set(time.Second * 30)
	t.Run("as system user", func(t *testing.T) {
		ok, resp := th.Client.ClearServerBusy()
		CheckForbiddenStatus(t, resp)
		require.False(t, ok, "should not clear server busy flag due to no permission.")
		require.True(t, th.App.Srv().Busy.IsBusy(), "server should be marked busy")
	})

	th.App.Srv().Busy.Set(time.Second * 30)
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		ok, resp := c.ClearServerBusy()
		CheckNoError(t, resp)
		require.True(t, ok, "should clear server busy flag successfully")
		require.False(t, th.App.Srv().Busy.IsBusy(), "server should not be marked busy")
	}, "as system admin")
}

func TestGetServerBusy(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().Busy.Set(time.Second * 30)

	t.Run("as system user", func(t *testing.T) {
		_, resp := th.Client.GetServerBusy()
		CheckForbiddenStatus(t, resp)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		sbs, resp := c.GetServerBusy()
		expires := time.Unix(sbs.Expires, 0)
		CheckNoError(t, resp)
		require.Greater(t, expires.Unix(), time.Now().Unix())
	}, "as system admin")
}

func TestGetServerBusyExpires(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().Busy.Set(time.Second * 30)

	t.Run("as system user", func(t *testing.T) {
		_, resp := th.Client.GetServerBusyExpires()
		CheckForbiddenStatus(t, resp)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, c *model.Client4) {
		expires, resp := c.GetServerBusyExpires()
		CheckNoError(t, resp)
		require.Greater(t, expires.Unix(), time.Now().Unix())
	}, "as system admin")
}

func TestServerBusy503(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.Srv().Busy.Set(time.Second * 30)

	t.Run("search users while busy", func(t *testing.T) {
		us := &model.UserSearch{Term: "test"}
		_, resp := th.SystemAdminClient.SearchUsers(us)
		CheckServiceUnavailableStatus(t, resp)
	})

	t.Run("search teams while busy", func(t *testing.T) {
		ts := &model.TeamSearch{}
		_, resp := th.SystemAdminClient.SearchTeams(ts)
		CheckServiceUnavailableStatus(t, resp)
	})

	t.Run("search channels while busy", func(t *testing.T) {
		cs := &model.ChannelSearch{}
		_, resp := th.SystemAdminClient.SearchChannels("foo", cs)
		CheckServiceUnavailableStatus(t, resp)
	})

	t.Run("search archived channels while busy", func(t *testing.T) {
		cs := &model.ChannelSearch{}
		_, resp := th.SystemAdminClient.SearchArchivedChannels("foo", cs)
		CheckServiceUnavailableStatus(t, resp)
	})

	th.App.Srv().Busy.Clear()

	t.Run("search users while not busy", func(t *testing.T) {
		us := &model.UserSearch{Term: "test"}
		_, resp := th.SystemAdminClient.SearchUsers(us)
		CheckNoError(t, resp)
	})
}

func TestPushNotificationAck(t *testing.T) {
	th := Setup(t)
	api := Init(th.App, th.Server.Router)
	session, _ := th.App.GetSession(th.Client.AuthToken)
	defer th.TearDown()
	t.Run("should return error when the ack body is not passed", func(t *testing.T) {
		handler := api.ApiHandler(pushNotificationAck)
		resp := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v4/notifications/ack", nil)
		req.Header.Set(model.HeaderAuth, "Bearer "+session.Token)

		handler.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.NotNil(t, resp.Body)
	})
}
