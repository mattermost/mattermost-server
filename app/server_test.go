// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/filestore"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func newServerWithConfig(t *testing.T, f func(cfg *model.Config)) (*Server, error) {
	configStore, err := config.NewMemoryStore()
	require.NoError(t, err)
	store, err := config.NewStoreFromBacking(configStore, nil, false)
	require.NoError(t, err)
	cfg := store.Get()
	f(cfg)

	store.Set(cfg)

	return NewServer(ConfigStore(store))
}

func TestStartServerSuccess(t *testing.T) {
	s, err := newServerWithConfig(t, func(cfg *model.Config) {
		*cfg.ServiceSettings.ListenAddress = ":0"
	})
	require.NoError(t, err)

	serverErr := s.Start()

	client := &http.Client{}
	checkEndpoint(t, client, "http://localhost:"+strconv.Itoa(s.ListenAddr.Port)+"/")

	s.Shutdown()
	require.NoError(t, serverErr)
}

func TestReadReplicaDisabledBasedOnLicense(t *testing.T) {
	t.Skip("TODO: fix flaky test")
	cfg := model.Config{}
	cfg.SetDefaults()
	driverName := os.Getenv("MM_SQLSETTINGS_DRIVERNAME")
	if driverName == "" {
		driverName = model.DATABASE_DRIVER_POSTGRES
	}
	dsn := ""
	if driverName == model.DATABASE_DRIVER_POSTGRES {
		dsn = os.Getenv("TEST_DATABASE_POSTGRESQL_DSN")
	} else {
		dsn = os.Getenv("TEST_DATABASE_MYSQL_DSN")
	}
	cfg.SqlSettings = *storetest.MakeSqlSettings(driverName, false)
	if dsn != "" {
		cfg.SqlSettings.DataSource = &dsn
	}
	cfg.SqlSettings.DataSourceReplicas = []string{*cfg.SqlSettings.DataSource}
	cfg.SqlSettings.DataSourceSearchReplicas = []string{*cfg.SqlSettings.DataSource}

	t.Run("Read Replicas with no License", func(t *testing.T) {
		s, err := NewServer(func(server *Server) error {
			configStore := config.NewTestMemoryStore()
			configStore.Set(&cfg)
			server.configStore = configStore
			return nil
		})
		require.NoError(t, err)
		defer s.Shutdown()
		require.Same(t, s.sqlStore.GetMaster(), s.sqlStore.GetReplica())
		require.Len(t, s.Config().SqlSettings.DataSourceReplicas, 1)
	})

	t.Run("Read Replicas With License", func(t *testing.T) {
		s, err := NewServer(func(server *Server) error {
			configStore := config.NewTestMemoryStore()
			configStore.Set(&cfg)
			server.licenseValue.Store(model.NewTestLicense())
			return nil
		})
		require.NoError(t, err)
		defer s.Shutdown()
		require.NotSame(t, s.sqlStore.GetMaster(), s.sqlStore.GetReplica())
		require.Len(t, s.Config().SqlSettings.DataSourceReplicas, 1)
	})

	t.Run("Search Replicas with no License", func(t *testing.T) {
		s, err := NewServer(func(server *Server) error {
			configStore := config.NewTestMemoryStore()
			configStore.Set(&cfg)
			server.configStore = configStore
			return nil
		})
		require.NoError(t, err)
		defer s.Shutdown()
		require.Same(t, s.sqlStore.GetMaster(), s.sqlStore.GetSearchReplica())
		require.Len(t, s.Config().SqlSettings.DataSourceSearchReplicas, 1)
	})

	t.Run("Search Replicas With License", func(t *testing.T) {
		s, err := NewServer(func(server *Server) error {
			configStore := config.NewTestMemoryStore()
			configStore.Set(&cfg)
			server.configStore = configStore
			server.licenseValue.Store(model.NewTestLicense())
			return nil
		})
		require.NoError(t, err)
		defer s.Shutdown()
		require.NotSame(t, s.sqlStore.GetMaster(), s.sqlStore.GetSearchReplica())
		require.Len(t, s.Config().SqlSettings.DataSourceSearchReplicas, 1)
	})
}

func TestStartServerPortUnavailable(t *testing.T) {
	s, err := NewServer()
	require.NoError(t, err)

	// Listen on the next available port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	// Attempt to listen on the port used above.
	s.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ListenAddress = listener.Addr().String()
	})

	serverErr := s.Start()
	s.Shutdown()
	require.Error(t, serverErr)
}

func TestStartServerNoS3Bucket(t *testing.T) {
	s3Host := os.Getenv("CI_MINIO_HOST")
	if s3Host == "" {
		s3Host = "localhost"
	}

	s3Port := os.Getenv("CI_MINIO_PORT")
	if s3Port == "" {
		s3Port = "9000"
	}

	s3Endpoint := fmt.Sprintf("%s:%s", s3Host, s3Port)

	s, err := NewServer(func(server *Server) error {
		configStore, _ := config.NewFileStore("config.json")
		store, _ := config.NewStoreFromBacking(configStore, nil, false)
		server.configStore = store
		server.UpdateConfig(func(cfg *model.Config) {
			cfg.FileSettings = model.FileSettings{
				DriverName:              model.NewString(model.IMAGE_DRIVER_S3),
				AmazonS3AccessKeyId:     model.NewString(model.MINIO_ACCESS_KEY),
				AmazonS3SecretAccessKey: model.NewString(model.MINIO_SECRET_KEY),
				AmazonS3Bucket:          model.NewString("nosuchbucket"),
				AmazonS3Endpoint:        model.NewString(s3Endpoint),
				AmazonS3Region:          model.NewString(""),
				AmazonS3PathPrefix:      model.NewString(""),
				AmazonS3SSL:             model.NewBool(false),
			}
		})
		return nil
	})
	require.NoError(t, err)

	// ensure that a new bucket was created
	backend, appErr := s.FileBackend()
	require.Nil(t, appErr)
	err = backend.(*filestore.S3FileBackend).TestConnection()
	require.NoError(t, err)
}

func TestStartServerTLSSuccess(t *testing.T) {
	s, err := newServerWithConfig(t, func(cfg *model.Config) {
		testDir, _ := fileutils.FindDir("tests")

		*cfg.ServiceSettings.ListenAddress = ":0"
		*cfg.ServiceSettings.ConnectionSecurity = "TLS"
		*cfg.ServiceSettings.TLSKeyFile = path.Join(testDir, "tls_test_key.pem")
		*cfg.ServiceSettings.TLSCertFile = path.Join(testDir, "tls_test_cert.pem")
	})
	require.NoError(t, err)

	serverErr := s.Start()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}
	checkEndpoint(t, client, "https://localhost:"+strconv.Itoa(s.ListenAddr.Port)+"/")

	s.Shutdown()
	require.NoError(t, serverErr)
}

func TestDatabaseTypeAndMattermostVersion(t *testing.T) {
	sqlDrivernameEnvironment := os.Getenv("MM_SQLSETTINGS_DRIVERNAME")

	if sqlDrivernameEnvironment != "" {
		defer os.Setenv("MM_SQLSETTINGS_DRIVERNAME", sqlDrivernameEnvironment)
	} else {
		defer os.Unsetenv("MM_SQLSETTINGS_DRIVERNAME")
	}

	os.Setenv("MM_SQLSETTINGS_DRIVERNAME", "postgres")

	th := Setup(t)
	defer th.TearDown()

	databaseType, mattermostVersion := th.Server.DatabaseTypeAndMattermostVersion()
	assert.Equal(t, "postgres", databaseType)
	assert.Equal(t, "5.31.0", mattermostVersion)

	os.Setenv("MM_SQLSETTINGS_DRIVERNAME", "mysql")

	th2 := Setup(t)
	defer th2.TearDown()

	databaseType, mattermostVersion = th2.Server.DatabaseTypeAndMattermostVersion()
	assert.Equal(t, "mysql", databaseType)
	assert.Equal(t, "5.31.0", mattermostVersion)
}

func TestGenerateSupportPacket(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	d1 := []byte("hello\ngo\n")
	err := ioutil.WriteFile("mattermost.log", d1, 0777)
	require.NoError(t, err)
	err = ioutil.WriteFile("notifications.log", d1, 0777)
	require.NoError(t, err)

	fileDatas := th.App.GenerateSupportPacket()
	testFiles := []string{"support_packet.yaml", "plugins.json", "sanitized_config.json", "mattermost.log", "notifications.log"}
	for i, fileData := range fileDatas {
		require.NotNil(t, fileData)
		assert.Equal(t, testFiles[i], fileData.Filename)
		assert.Positive(t, len(fileData.Body))
	}

	// Remove these two files and ensure that warning.txt file is generated
	err = os.Remove("notifications.log")
	require.NoError(t, err)
	err = os.Remove("mattermost.log")
	require.NoError(t, err)
	fileDatas = th.App.GenerateSupportPacket()
	testFiles = []string{"support_packet.yaml", "plugins.json", "sanitized_config.json", "warning.txt"}
	for i, fileData := range fileDatas {
		require.NotNil(t, fileData)
		assert.Equal(t, testFiles[i], fileData.Filename)
		assert.Positive(t, len(fileData.Body))
	}
}

func TestGetNotificationsLog(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// Disable notifications file to get an error
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.NotificationLogSettings.EnableFile = false
	})

	fileData, warning := th.App.getNotificationsLog()
	assert.Nil(t, fileData)
	assert.Equal(t, warning, "Unable to retrieve notifications.log because LogSettings: EnableFile is false in config.json")

	// Enable notifications file but delete any notifications file to get an error trying to read the file
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.NotificationLogSettings.EnableFile = true
	})

	// If any previous notifications.log file, lets delete it
	os.Remove("notifications.log")

	fileData, warning = th.App.getNotificationsLog()
	assert.Nil(t, fileData)
	assert.Contains(t, warning, "ioutil.ReadFile(notificationsLog) Error:")

	// Happy path where we have file and no warning
	d1 := []byte("hello\ngo\n")
	err := ioutil.WriteFile("notifications.log", d1, 0777)
	defer os.Remove("notifications.log")
	require.NoError(t, err)

	fileData, warning = th.App.getNotificationsLog()
	require.NotNil(t, fileData)
	assert.Equal(t, "notifications.log", fileData.Filename)
	assert.Positive(t, len(fileData.Body))
	assert.Empty(t, warning)
}

func TestGetMattermostLog(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// disable mattermost log file setting in config so we should get an warning
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.LogSettings.EnableFile = false
	})

	fileData, warning := th.App.getMattermostLog()
	assert.Nil(t, fileData)
	assert.Equal(t, "Unable to retrieve mattermost.log because LogSettings: EnableFile is false in config.json", warning)

	// We enable the setting but delete any mattermost log file
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.LogSettings.EnableFile = true
	})

	// If any previous mattermost.log file, lets delete it
	os.Remove("mattermost.log")

	fileData, warning = th.App.getMattermostLog()
	assert.Nil(t, fileData)
	assert.Contains(t, warning, "ioutil.ReadFile(mattermostLog) Error:")

	// Happy path where we get a log file and no warning
	d1 := []byte("hello\ngo\n")
	err := ioutil.WriteFile("mattermost.log", d1, 0777)
	defer os.Remove("mattermost.log")
	require.NoError(t, err)

	fileData, warning = th.App.getMattermostLog()
	require.NotNil(t, fileData)
	assert.Equal(t, "mattermost.log", fileData.Filename)
	assert.Positive(t, len(fileData.Body))
	assert.Empty(t, warning)
}

func TestCreateSanitizedConfigFile(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// Happy path where we have a sanitized config file with no warning
	fileData, warning := th.App.createSanitizedConfigFile()
	require.NotNil(t, fileData)
	assert.Equal(t, "sanitized_config.json", fileData.Filename)
	assert.Positive(t, len(fileData.Body))
	assert.Empty(t, warning)
}

func TestCreatePluginsFile(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// Happy path where we have a plugins file with no warning
	fileData, warning := th.App.createPluginsFile()
	require.NotNil(t, fileData)
	assert.Equal(t, "plugins.json", fileData.Filename)
	assert.Positive(t, len(fileData.Body))
	assert.Empty(t, warning)

	// Turn off plugins so we can get an error
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.Enable = false
	})

	// Plugins off in settings so no fileData and we get a warning instead
	fileData, warning = th.App.createPluginsFile()
	assert.Nil(t, fileData)
	assert.Contains(t, warning, "c.App.GetPlugins() Error:")
}

func TestGenerateSupportPacketYaml(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// Happy path where we have a support packet yaml file without any warnings
	fileData, warning := th.App.generateSupportPacketYaml()
	require.NotNil(t, fileData)
	assert.Equal(t, "support_packet.yaml", fileData.Filename)
	assert.Positive(t, len(fileData.Body))
	assert.Empty(t, warning)

}

func TestStartServerTLSVersion(t *testing.T) {
	configStore, _ := config.NewMemoryStore()
	store, _ := config.NewStoreFromBacking(configStore, nil, false)
	cfg := store.Get()
	testDir, _ := fileutils.FindDir("tests")

	*cfg.ServiceSettings.ListenAddress = ":0"
	*cfg.ServiceSettings.ConnectionSecurity = "TLS"
	*cfg.ServiceSettings.TLSMinVer = "1.2"
	*cfg.ServiceSettings.TLSKeyFile = path.Join(testDir, "tls_test_key.pem")
	*cfg.ServiceSettings.TLSCertFile = path.Join(testDir, "tls_test_cert.pem")

	store.Set(cfg)

	s, err := NewServer(ConfigStore(store))
	require.NoError(t, err)

	serverErr := s.Start()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MaxVersion:         tls.VersionTLS11,
		},
	}

	client := &http.Client{Transport: tr}
	err = checkEndpoint(t, client, "https://localhost:"+strconv.Itoa(s.ListenAddr.Port)+"/")

	if !strings.Contains(err.Error(), "remote error: tls: protocol version not supported") {
		t.Errorf("Expected protocol version error, got %s", err)
	}

	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	err = checkEndpoint(t, client, "https://localhost:"+strconv.Itoa(s.ListenAddr.Port)+"/")

	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}

	s.Shutdown()
	require.NoError(t, serverErr)
}

func TestStartServerTLSOverwriteCipher(t *testing.T) {
	s, err := newServerWithConfig(t, func(cfg *model.Config) {
		testDir, _ := fileutils.FindDir("tests")

		*cfg.ServiceSettings.ListenAddress = ":0"
		*cfg.ServiceSettings.ConnectionSecurity = "TLS"
		cfg.ServiceSettings.TLSOverwriteCiphers = []string{
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		}
		*cfg.ServiceSettings.TLSKeyFile = path.Join(testDir, "tls_test_key.pem")
		*cfg.ServiceSettings.TLSCertFile = path.Join(testDir, "tls_test_cert.pem")
	})
	require.NoError(t, err)

	err = s.Start()
	require.NoError(t, err)

	defer s.Shutdown()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			},
			MaxVersion: tls.VersionTLS12,
		},
	}

	client := &http.Client{Transport: tr}
	err = checkEndpoint(t, client, "https://localhost:"+strconv.Itoa(s.ListenAddr.Port)+"/")
	require.Error(t, err, "Expected error due to Cipher mismatch")
	require.Contains(t, err.Error(), "remote error: tls: handshake failure", "Expected protocol version error")

	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			MaxVersion: tls.VersionTLS12,
		},
	}

	err = checkEndpoint(t, client, "https://localhost:"+strconv.Itoa(s.ListenAddr.Port)+"/")
	require.NoError(t, err)
}

func checkEndpoint(t *testing.T, client *http.Client, url string) error {
	res, err := client.Get(url)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("Response code was %d; want %d", res.StatusCode, http.StatusNotFound)
	}

	return nil
}

func TestPanicLog(t *testing.T) {
	// Creating a temp file to collect logs
	tmpfile, err := ioutil.TempFile("", "mlog")
	if err != nil {
		require.NoError(t, err)
	}

	defer func() {
		require.NoError(t, tmpfile.Close())
		require.NoError(t, os.Remove(tmpfile.Name()))
	}()

	// This test requires Zap file target for now.
	mlog.EnableZap()
	defer mlog.DisableZap()

	// Creating logger to log to console and temp file
	logger := mlog.NewLogger(&mlog.LoggerConfiguration{
		EnableConsole: true,
		ConsoleJson:   true,
		EnableFile:    true,
		FileLocation:  tmpfile.Name(),
		FileLevel:     mlog.LevelInfo,
	})

	// Creating a server with logger
	s, err := NewServer(SetLogger(logger))
	require.NoError(t, err)

	// Route for just panicing
	s.Router.HandleFunc("/panic", func(writer http.ResponseWriter, request *http.Request) {
		s.Log.Info("inside panic handler")
		panic("log this panic")
	})

	testDir, _ := fileutils.FindDir("tests")
	s.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ListenAddress = ":0"
		*cfg.ServiceSettings.ConnectionSecurity = "TLS"
		*cfg.ServiceSettings.TLSKeyFile = path.Join(testDir, "tls_test_key.pem")
		*cfg.ServiceSettings.TLSCertFile = path.Join(testDir, "tls_test_cert.pem")
	})
	serverErr := s.Start()
	require.NoError(t, serverErr)

	// Calling panic route
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}
	client.Get("https://localhost:" + strconv.Itoa(s.ListenAddr.Port) + "/panic")
	s.Shutdown()

	// Checking whether panic was logged
	var panicLogged = false
	var infoLogged = false

	_, err = tmpfile.Seek(0, 0)
	require.NoError(t, err)

	scanner := bufio.NewScanner(tmpfile)
	for scanner.Scan() {
		if !infoLogged && strings.Contains(scanner.Text(), "inside panic handler") {
			infoLogged = true
		}
		if strings.Contains(scanner.Text(), "log this panic") {
			panicLogged = true
			break
		}
	}

	if !infoLogged {
		t.Error("Info log line was supposed to be logged")
	}

	if !panicLogged {
		t.Error("Panic was supposed to be logged")
	}
}

func TestSentry(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	testDir, _ := fileutils.FindDir("tests")

	t.Run("sentry is disabled, should not receive a report", func(t *testing.T) {
		data := make(chan bool, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Log("Received sentry request for some reason")
			data <- true
		}))
		defer server.Close()

		// make sure we don't report anything when sentry is disabled
		_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
		dsn, err := sentry.NewDsn(fmt.Sprintf("http://test:test@localhost:%s/123", port))
		require.NoError(t, err)
		SentryDSN = dsn.String()

		s, err := newServerWithConfig(t, func(cfg *model.Config) {
			*cfg.ServiceSettings.ListenAddress = ":0"
			*cfg.LogSettings.EnableSentry = false
			*cfg.ServiceSettings.ConnectionSecurity = "TLS"
			*cfg.ServiceSettings.TLSKeyFile = path.Join(testDir, "tls_test_key.pem")
			*cfg.ServiceSettings.TLSCertFile = path.Join(testDir, "tls_test_cert.pem")
			*cfg.LogSettings.EnableDiagnostics = true
		})
		require.NoError(t, err)

		s.Router.HandleFunc("/panic", func(writer http.ResponseWriter, request *http.Request) {
			panic("log this panic")
		})

		require.NoError(t, s.Start())
		defer s.Shutdown()

		resp, err := client.Get("https://localhost:" + strconv.Itoa(s.ListenAddr.Port) + "/panic")
		require.Nil(t, resp)
		require.True(t, errors.Is(err, io.EOF), fmt.Sprintf("unexpected error: %s", err))

		sentry.Flush(time.Second)
		select {
		case <-data:
			require.Fail(t, "Sentry received a message, even though it's disabled!")
		case <-time.After(time.Second):
			t.Log("Sentry request didn't arrive. Good!")
		}
	})

	t.Run("sentry is enabled, report should be received", func(t *testing.T) {
		data := make(chan bool, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Log("Received sentry request!")
			data <- true
		}))
		defer server.Close()

		_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
		dsn, err := sentry.NewDsn(fmt.Sprintf("http://test:test@localhost:%s/123", port))
		require.NoError(t, err)
		SentryDSN = dsn.String()

		s, err := newServerWithConfig(t, func(cfg *model.Config) {
			*cfg.ServiceSettings.ListenAddress = ":0"
			*cfg.ServiceSettings.ConnectionSecurity = "TLS"
			*cfg.ServiceSettings.TLSKeyFile = path.Join(testDir, "tls_test_key.pem")
			*cfg.ServiceSettings.TLSCertFile = path.Join(testDir, "tls_test_cert.pem")
			*cfg.LogSettings.EnableSentry = true
			*cfg.LogSettings.EnableDiagnostics = true
		})
		require.NoError(t, err)

		// Route for just panicing
		s.Router.HandleFunc("/panic", func(writer http.ResponseWriter, request *http.Request) {
			panic("log this panic")
		})

		require.NoError(t, s.Start())
		defer s.Shutdown()

		resp, err := client.Get("https://localhost:" + strconv.Itoa(s.ListenAddr.Port) + "/panic")
		require.Nil(t, resp)
		require.True(t, errors.Is(err, io.EOF), fmt.Sprintf("unexpected error: %s", err))

		sentry.Flush(time.Second)
		select {
		case <-data:
			t.Log("Sentry request arrived. Good!")
		case <-time.After(time.Second * 10):
			require.Fail(t, "Sentry report didn't arrive")
		}
	})
}

func TestAdminAdvisor(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// creating a system user to whole admin advisor will send post
	user := model.User{
		Email:       strings.ToLower(model.NewId()) + "success+test@example.com",
		Nickname:    "Darth Vader",
		Username:    "vader" + model.NewId(),
		Password:    "passwd1",
		AuthService: "",
		Roles:       model.SYSTEM_ADMIN_ROLE_ID,
	}
	ruser, err := th.App.CreateUser(th.Context, &user)
	assert.Nil(t, err, "User should be created")
	defer th.App.PermanentDeleteUser(th.Context, &user)

	t.Run("Should notify admin of un-configured support email", func(t *testing.T) {
		doCheckAdminSupportStatus(th.App, th.Context)

		bot, err := th.App.GetUserByUsername(model.BOT_WARN_METRIC_BOT_USERNAME)
		assert.NotNil(t, bot, "Bot should have been created now")
		assert.Nil(t, err, "No error should be generated")

		channel, err := th.App.getDirectChannel(bot.Id, ruser.Id)
		assert.NotNil(t, channel, "DM channel should exist between Admin Advisor and system admin")
		assert.Nil(t, err, "No error should be generated")
	})

	t.Run("Should NOT notify admin when support email is configured", func(t *testing.T) {
		th.App.UpdateConfig(func(m *model.Config) {
			email := "success+test@example.com"
			m.SupportSettings.SupportEmail = &email
		})

		bot, err := th.App.GetUserByUsername(model.BOT_WARN_METRIC_BOT_USERNAME)
		assert.NotNil(t, bot, "Bot should be already created")
		assert.Nil(t, err, "No error should be generated")

		channel, err := th.App.getDirectChannel(bot.Id, ruser.Id)
		assert.NotNil(t, channel, "DM channel should already exist")
		assert.Nil(t, err, "No error should be generated")

		err = th.App.PermanentDeleteChannel(channel)
		assert.Nil(t, err, "No error should be generated")

		doCheckAdminSupportStatus(th.App, th.Context)

		channel, err = th.App.getDirectChannel(bot.Id, ruser.Id)
		assert.NotNil(t, channel, "DM channel should exist between Admin Advisor and system admin")
		assert.Nil(t, err, "No error should be generated")

		posts, err := th.App.GetPosts(channel.Id, 0, 100)
		assert.Nil(t, err, "No error should be generated")
		assert.Equal(t, 0, len(posts.Posts))
	})
}
