// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/testlib"
	"github.com/mattermost/mattermost/server/v8/config"
	emocks "github.com/mattermost/mattermost/server/v8/einterfaces/mocks"
	semocks "github.com/mattermost/mattermost/server/v8/platform/services/searchengine/mocks"
	fmocks "github.com/mattermost/mattermost/server/v8/platform/shared/filestore/mocks"
)

type fixedDBStatsStore struct {
	store.Store
	masterStats    sql.DBStats
	replicaStats   sql.DBStats
	replicasOnline int
}

func (s *fixedDBStatsStore) GetDiagnostics(_ request.CTX) (*store.DatabaseDiagnostics, error) {
	diagnostics := &store.DatabaseDiagnostics{
		MasterConnectionsInUse:              s.masterStats.InUse,
		MasterConnectionsIdle:               s.masterStats.Idle,
		MasterPoolWaitCount:                 s.masterStats.WaitCount,
		MasterPoolWaitDurationMs:            s.masterStats.WaitDuration.Milliseconds(),
		MasterConnectionsClosedMaxIdle:      s.masterStats.MaxIdleClosed,
		MasterConnectionsClosedMaxLifetime:  s.masterStats.MaxLifetimeClosed,
		ReplicaConnectionsInUse:             s.replicaStats.InUse,
		ReplicaConnectionsIdle:              s.replicaStats.Idle,
		ReplicaPoolWaitCount:                s.replicaStats.WaitCount,
		ReplicaPoolWaitDurationMs:           s.replicaStats.WaitDuration.Milliseconds(),
		ReplicaConnectionsClosedMaxIdle:     s.replicaStats.MaxIdleClosed,
		ReplicaConnectionsClosedMaxLifetime: s.replicaStats.MaxLifetimeClosed,
		ReplicasOnline:                      s.replicasOnline,
	}

	return diagnostics, nil
}

type failingDiagnosticsStore struct {
	store.Store
	schemaVersionErr error
	dbVersionErr     error
	diagnostics      *store.DatabaseDiagnostics
	diagnosticsErr   error
}

func (s *failingDiagnosticsStore) GetDBSchemaVersion() (int, error) {
	if s.schemaVersionErr != nil {
		return 0, s.schemaVersionErr
	}
	return s.Store.GetDBSchemaVersion()
}

func (s *failingDiagnosticsStore) GetDbVersion(numerical bool) (string, error) {
	if s.dbVersionErr != nil {
		return "", s.dbVersionErr
	}
	return s.Store.GetDbVersion(numerical)
}

func (s *failingDiagnosticsStore) GetDiagnostics(rctx request.CTX) (*store.DatabaseDiagnostics, error) {
	if s.diagnosticsErr != nil {
		return s.diagnostics, s.diagnosticsErr
	}
	return s.Store.GetDiagnostics(rctx)
}

// shortCPUProfileDuration keeps GenerateSupportPacket calls fast in tests
// that don't care about the CPU profile's contents.
var shortCPUProfileDuration = model.NewPointer(100 * time.Millisecond)

func TestGenerateSupportPacket(t *testing.T) {
	mainHelper.Parallel(t)

	th := Setup(t)

	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.RemoveAll(dir)
		assert.NoError(t, err)
	})

	// Override log root path to allow log file reads from our temp directory
	th.Service.SetLogRootPathOverride(dir)

	th.Service.UpdateConfig(func(cfg *model.Config) {
		*cfg.LogSettings.FileLocation = dir
	})

	logLocation := config.GetLogFileLocation(dir)

	genMockLogFiles := func() {
		d1 := []byte("hello\ngo\n")
		genErr := os.WriteFile(logLocation, d1, 0600)
		require.NoError(t, genErr)
	}
	genMockLogFiles()

	getFileNames := func(t *testing.T, fileDatas []model.FileData) []string {
		var rFileNames []string
		for _, fileData := range fileDatas {
			require.NotNil(t, fileData)
			assert.Positive(t, len(fileData.Body))

			rFileNames = append(rFileNames, fileData.Filename)
		}
		return rFileNames
	}

	expectedFileNames := []string{
		"diagnostics.yaml",
		"sanitized_config.json",
		"cpu.prof",
		"heap.prof",
		"goroutines",
	}

	expectedFileNamesWithLogs := append(expectedFileNames, "mattermost.log")

	var fileDatas []model.FileData

	t.Run("generate Support Packet with logs", func(t *testing.T) {
		fileDatas, err = th.Service.GenerateSupportPacket(th.Context, &model.SupportPacketOptions{
			CPUProfileDuration: shortCPUProfileDuration,
			IncludeLogs:        true,
		})
		require.NoError(t, err)
		rFileNames := getFileNames(t, fileDatas)

		assert.ElementsMatch(t, expectedFileNamesWithLogs, rFileNames)
	})

	t.Run("generate Support Packet without logs", func(t *testing.T) {
		fileDatas, err = th.Service.GenerateSupportPacket(th.Context, &model.SupportPacketOptions{
			CPUProfileDuration: shortCPUProfileDuration,
			IncludeLogs:        false,
		})
		require.NoError(t, err)
		rFileNames := getFileNames(t, fileDatas)

		assert.ElementsMatch(t, expectedFileNames, rFileNames)
	})

	t.Run("zero CPU profile duration skips cpu.prof", func(t *testing.T) {
		fileDatas, err = th.Service.GenerateSupportPacket(th.Context, &model.SupportPacketOptions{
			CPUProfileDuration: model.NewPointer(time.Duration(0)),
			IncludeLogs:        true,
		})
		require.NoError(t, err)
		rFileNames := getFileNames(t, fileDatas)

		assert.NotContains(t, rFileNames, "cpu.prof")
		expectedWithoutCPU := []string{"diagnostics.yaml", "sanitized_config.json", "heap.prof", "goroutines", "mattermost.log"}
		assert.ElementsMatch(t, expectedWithoutCPU, rFileNames)
	})

	t.Run("remove the log files and ensure that an error is returned", func(t *testing.T) {
		err = os.Remove(logLocation)
		require.NoError(t, err)
		t.Cleanup(genMockLogFiles)

		fileDatas, err = th.Service.GenerateSupportPacket(th.Context, &model.SupportPacketOptions{
			CPUProfileDuration: shortCPUProfileDuration,
			IncludeLogs:        true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed read mattermost log file")
		rFileNames := getFileNames(t, fileDatas)

		assert.ElementsMatch(t, expectedFileNames, rFileNames)
	})

	t.Run("with advanced logs", func(t *testing.T) {
		optLDAP := map[string]string{
			"filename": path.Join(dir, "ldap.log"),
		}
		dataLDAP, err := json.Marshal(optLDAP)
		require.NoError(t, err)

		cfg := mlog.LoggerConfiguration{
			"ldap-file": mlog.TargetCfg{
				Type:   "file",
				Format: "json",
				Levels: []mlog.Level{
					mlog.LvlLDAPError,
					mlog.LvlLDAPWarn,
					mlog.LvlLDAPInfo,
					mlog.LvlLDAPDebug,
				},
				Options: dataLDAP,
			},
		}
		cfgData, err := json.Marshal(cfg)
		require.NoError(t, err)

		th.Service.UpdateConfig(func(c *model.Config) {
			c.LogSettings.AdvancedLoggingJSON = cfgData
		})

		th.Service.Logger().LogM([]mlog.Level{mlog.LvlLDAPInfo}, "Some LDAP info")
		err = th.Service.Logger().Flush()
		require.NoError(t, err)

		fileDatas, err = th.Service.GenerateSupportPacket(th.Context, &model.SupportPacketOptions{
			CPUProfileDuration: shortCPUProfileDuration,
			IncludeLogs:        true,
		})
		require.NoError(t, err)
		rFileNames := getFileNames(t, fileDatas)

		assert.ElementsMatch(t, append(expectedFileNamesWithLogs, "ldap.log"), rFileNames)

		found := false
		for _, fileData := range fileDatas {
			if fileData.Filename == "ldap.log" {
				testlib.AssertLog(t, bytes.NewBuffer(fileData.Body), mlog.LvlLDAPInfo.Name, "Some LDAP info")
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestGetSupportPacketDiagnostics(t *testing.T) {
	t.Skip("Skipped due to flakiness — tracked in https://mattermost.atlassian.net/browse/MM-71672")

	th := Setup(t).InitBasic(t)

	th.Service.installTypeOverride = "docker"

	licenseUsers := 100
	license := model.NewTestLicense("ldap")
	license.SkuShortName = model.LicenseShortSkuEnterprise
	license.Features.Users = new(licenseUsers)
	ok := th.Service.SetLicense(license)
	require.True(t, ok)

	getDiagnostics := func(t *testing.T) *model.SupportPacketDiagnostics {
		t.Helper()

		d, err := th.Service.GetSupportPacketDiagnostics(th.Context)
		require.NotNil(t, d)
		assert.NoError(t, err)
		return d.Diagnostics
	}

	t.Run("Happy path", func(t *testing.T) {
		d := getDiagnostics(t)

		assert.Equal(t, 2, d.Version)

		/* License */
		assert.Equal(t, "My awesome Company", d.License.Company)
		assert.Equal(t, licenseUsers, d.License.Users)
		assert.Equal(t, model.LicenseShortSkuEnterprise, d.License.SkuShortName)
		assert.Equal(t, false, d.License.IsTrial)
		assert.Equal(t, false, d.License.IsGovSKU)
		assert.False(t, d.License.IsCloud)

		/* Server information */
		assert.NotEmpty(t, d.Server.OS)
		assert.NotEmpty(t, d.Server.Architecture)
		require.NotNil(t, d.Server.Hostname)
		assert.NotEmpty(t, *d.Server.Hostname)
		assert.Equal(t, model.CurrentVersion, d.Server.Version)
		// BuildHash is not present in tests
		assert.NotEmpty(t, d.Server.GoVersion)
		assert.Equal(t, "docker", d.Server.InstallationType)
		assert.Positive(t, d.Server.CPUCores)
		require.NotNil(t, d.Server.GoMaxProcs)
		assert.Positive(t, *d.Server.GoMaxProcs)
		assert.Equal(t, d.Server.ContainerCPUPeriods == nil, d.Server.ContainerCPUThrottledPeriods == nil)
		if runtime.GOOS == "linux" {
			require.NotNil(t, d.Server.LoadAverage1m)
			require.NotNil(t, d.Server.LoadAverage5m)
			require.NotNil(t, d.Server.LoadAverage15m)
		} else {
			assert.Nil(t, d.Server.LoadAverage1m)
			assert.Nil(t, d.Server.LoadAverage5m)
			assert.Nil(t, d.Server.LoadAverage15m)
		}
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			require.NotNil(t, d.Server.TotalMemoryMB)
			assert.Positive(t, *d.Server.TotalMemoryMB)
		} else {
			assert.Nil(t, d.Server.TotalMemoryMB)
		}
		require.NotNil(t, d.Server.OpenFileDescriptors)
		require.NotNil(t, d.Server.MaxFileDescriptors)
		assert.True(t, *d.Server.OpenFileDescriptors == -1 || *d.Server.OpenFileDescriptors > 0, "OpenFileDescriptors should be -1 (unsupported) or positive, got %d", *d.Server.OpenFileDescriptors)
		assert.True(t, *d.Server.MaxFileDescriptors == -1 || *d.Server.MaxFileDescriptors > 0, "MaxFileDescriptors should be -1 (unsupported) or positive, got %d", *d.Server.MaxFileDescriptors)
		assert.Positive(t, d.Server.ProcessID)
		assert.False(t, d.Server.StartedAt.IsZero())
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			assert.False(t, d.Server.HostStartedAt.IsZero())
			assert.True(t, !d.Server.HostStartedAt.After(d.Server.StartedAt))
		} else {
			assert.True(t, d.Server.HostStartedAt.IsZero(), "HostStartedAt should be zero on unsupported platforms")
		}

		/* Config */
		assert.Equal(t, "memory://", d.Config.Source)
		assert.Equal(t, new(false), d.Config.AuditLoggingConfigured)

		/* DB */
		require.NotNil(t, d.Database.Type)
		assert.NotEmpty(t, *d.Database.Type)
		require.NotNil(t, d.Database.Version)
		assert.NotEmpty(t, *d.Database.Version)
		require.NotNil(t, d.Database.SchemaVersion)
		assert.NotEmpty(t, *d.Database.SchemaVersion)
		assert.NotZero(t, d.Database.MasterConnections)
		assert.Zero(t, d.Database.ReplicaConnections)
		assert.Zero(t, d.Database.SearchConnections)
		require.NotNil(t, d.Database.MasterConnectionsInUse)
		assert.GreaterOrEqual(t, *d.Database.MasterConnectionsInUse, 0)
		for _, v := range []*int{d.Database.MasterConnectionsIdle, d.Database.ReplicaConnectionsInUse, d.Database.ReplicaConnectionsIdle, d.Database.ReplicasOnline} {
			require.NotNil(t, v)
			assert.GreaterOrEqual(t, *v, 0)
		}
		for _, v := range []*int64{
			d.Database.MasterPoolWaitCount,
			d.Database.MasterPoolWaitDurationMs,
			d.Database.MasterConnectionsClosedMaxIdle,
			d.Database.MasterConnectionsClosedMaxLifetime,
			d.Database.ReplicaPoolWaitCount,
			d.Database.ReplicaPoolWaitDurationMs,
			d.Database.ReplicaConnectionsClosedMaxIdle,
			d.Database.ReplicaConnectionsClosedMaxLifetime,
		} {
			require.NotNil(t, v)
			assert.GreaterOrEqual(t, *v, int64(0))
		}

		/* File store */
		assert.Equal(t, "OK", d.FileStore.Status)
		assert.Empty(t, d.FileStore.Error)
		assert.Equal(t, "local", d.FileStore.Driver)
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			require.NotNil(t, d.FileStore.FilesystemType)
			require.NotNil(t, d.FileStore.TotalMB)
			require.NotNil(t, d.FileStore.AvailableMB)
			assert.NotEmpty(t, *d.FileStore.FilesystemType, "FilesystemType should not be empty on supported platforms")
			assert.Positive(t, *d.FileStore.TotalMB, "TotalMB should be positive on supported platforms")
			assert.Positive(t, *d.FileStore.AvailableMB, "AvailableMB should be positive on supported platforms")
		} else {
			assert.Nil(t, d.FileStore.FilesystemType)
			assert.Nil(t, d.FileStore.TotalMB)
			assert.Nil(t, d.FileStore.AvailableMB)
		}

		/* Websockets */
		assert.Zero(t, d.Websocket.Connections)

		/* Cluster */
		assert.Empty(t, d.Cluster.ID)
		require.NotNil(t, d.Cluster.NumberOfNodes)
		assert.Equal(t, 1, *d.Cluster.NumberOfNodes)
		require.NotNil(t, d.Cluster.NumberOfConnectedNodes)
		assert.Equal(t, 1, *d.Cluster.NumberOfConnectedNodes)
		assert.False(t, d.Cluster.IsLeader)

		/* LDAP */
		assert.Equal(t, model.StatusDisabled, d.LDAP.Status)
		assert.Empty(t, d.LDAP.Error)
		assert.Empty(t, d.LDAP.ServerName)
		assert.Empty(t, d.LDAP.ServerVersion)

		/* SAML */
		assert.Empty(t, d.SAML.ProviderType)

		/* Elastic Search */
		assert.Equal(t, model.StatusDisabled, d.ElasticSearch.Status)
		assert.Empty(t, d.ElasticSearch.ServerVersion)
		assert.Empty(t, d.ElasticSearch.ServerPlugins)

		/* OAuth Providers (all disabled by default) */
		assert.Equal(t, model.StatusDisabled, d.OAuthProviders.GitLab.Status)
		assert.Equal(t, model.StatusDisabled, d.OAuthProviders.Google.Status)
		assert.Equal(t, model.StatusDisabled, d.OAuthProviders.Office365.Status)
		assert.Equal(t, model.StatusDisabled, d.OAuthProviders.OpenID.Status)
	})

	t.Run("database-only nodes count as nodes but not as connected", func(t *testing.T) {
		cluster := emocks.NewClusterInterface(t)
		cluster.On("SendClusterMessage", mock.Anything).Return().Maybe()
		cluster.On("GetClusterId").Return("cluster-id")
		cluster.On("IsLeader").Return(true)
		cluster.On("GetClusterInfos").Return([]*model.ClusterInfo{
			{Id: "a", Hostname: "app-1", IPAddress: "10.0.0.1"},
			{Id: "b", Hostname: "app-2", IPAddress: "10.0.0.2"},
			{Id: "c", Hostname: "app-3"},
		}, nil)
		originalCluster := th.Service.clusterIFace
		t.Cleanup(func() {
			th.Service.clusterIFace = originalCluster
		})
		th.Service.clusterIFace = cluster

		d := getDiagnostics(t)
		require.NotNil(t, d.Cluster.NumberOfNodes)
		assert.Equal(t, 3, *d.Cluster.NumberOfNodes)
		require.NotNil(t, d.Cluster.NumberOfConnectedNodes)
		assert.Equal(t, 2, *d.Cluster.NumberOfConnectedNodes)
	})

	t.Run("only the cluster leader writes is_leader", func(t *testing.T) {
		originalCluster := th.Service.clusterIFace
		t.Cleanup(func() {
			th.Service.clusterIFace = originalCluster
		})

		leaders := 0
		for _, isLeader := range []bool{true, false, false} {
			cluster := emocks.NewClusterInterface(t)
			cluster.On("SendClusterMessage", mock.Anything).Return().Maybe()
			cluster.On("GetClusterId").Return("cluster-id")
			cluster.On("IsLeader").Return(isLeader)
			cluster.On("GetClusterInfos").Return([]*model.ClusterInfo{{Id: "a"}, {Id: "b"}, {Id: "c"}}, nil)
			th.Service.clusterIFace = cluster

			fileData, err := supportPacketDiagnosticsFile(getDiagnostics(t), nil)
			require.NoError(t, err)
			if strings.Contains(string(fileData.Body), "is_leader: true") {
				leaders++
			}
		}
		assert.Equal(t, 1, leaders)
	})

	t.Run("cloud license writes is_cloud", func(t *testing.T) {
		cloudLicense := model.NewTestLicense("cloud")
		require.True(t, th.Service.SetLicense(cloudLicense))
		t.Cleanup(func() {
			require.True(t, th.Service.SetLicense(license))
		})

		d := getDiagnostics(t)
		assert.True(t, d.License.IsCloud)

		fileData, err := supportPacketDiagnosticsFile(d, nil)
		require.NoError(t, err)
		assert.Contains(t, string(fileData.Body), "is_cloud: true")
	})

	t.Run("trial recognized by duration writes is_trial", func(t *testing.T) {
		trialLicense := model.NewTestLicense("ldap")
		trialLicense.IsTrial = false
		trialLicense.StartsAt = model.GetMillis()
		trialLicense.ExpiresAt = trialLicense.StartsAt + (30*24*time.Hour + 8*time.Hour).Milliseconds()
		require.True(t, trialLicense.IsTrialLicense())
		require.True(t, th.Service.SetLicense(trialLicense))
		t.Cleanup(func() {
			require.True(t, th.Service.SetLicense(license))
		})

		assert.True(t, getDiagnostics(t).License.IsTrial)
	})

	t.Run("enforced license writes expiry, seat limit and features", func(t *testing.T) {
		enforcedLicense := model.NewTestLicense("ldap")
		enforcedLicense.IsSeatCountEnforced = true
		enforcedLicense.ExtraUsers = new(5)
		require.True(t, th.Service.SetLicense(enforcedLicense))
		t.Cleanup(func() {
			require.True(t, th.Service.SetLicense(license))
		})

		d := getDiagnostics(t)
		assert.Equal(t, enforcedLicense.ExpiresAt, d.License.ExpiresAt)
		assert.True(t, d.License.IsSeatCountEnforced)
		assert.Equal(t, new(5), d.License.ExtraUsers)
		require.NotNil(t, d.License.Features)
		assert.Equal(t, new(true), d.License.Features.LDAP)

		fileData, err := supportPacketDiagnosticsFile(d, nil)
		require.NoError(t, err)
		body := string(fileData.Body)
		assert.Contains(t, body, "expires_at: "+strconv.FormatInt(enforcedLicense.ExpiresAt, 10)+"\n")
		assert.Contains(t, body, "is_seat_count_enforced: true\n")
		assert.Contains(t, body, "extra_users: 5\n")
		assert.Contains(t, body, "    ldap: true\n")
	})

	t.Run("audit_logging_configured records this server's audit targets", func(t *testing.T) {
		setAudit := func(t *testing.T, fileEnabled bool, advanced string) {
			t.Helper()
			prior := th.Service.Config().Clone().ExperimentalAuditSettings
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.ExperimentalAuditSettings.FileEnabled = new(fileEnabled)
				cfg.ExperimentalAuditSettings.FileName = new("audit.log")
				cfg.ExperimentalAuditSettings.AdvancedLoggingJSON = json.RawMessage(advanced)
			})
			t.Cleanup(func() {
				th.Service.UpdateConfig(func(cfg *model.Config) {
					cfg.ExperimentalAuditSettings = prior
				})
			})
		}
		auditTarget := `{"audit": {"type": "file", "format": "json", "levels": [{"id": 100, "name": "audit-api"}], "options": {"filename": "audit.log"}}}`

		t.Run("file enabled", func(t *testing.T) {
			setAudit(t, true, "{}")
			d := getDiagnostics(t)
			assert.Equal(t, new(true), d.Config.AuditLoggingConfigured)

			fileData, err := supportPacketDiagnosticsFile(d, nil)
			require.NoError(t, err)
			assert.Contains(t, string(fileData.Body), "  audit_logging_configured: true #")
		})

		t.Run("advanced target with advanced logging licensed", func(t *testing.T) {
			setAudit(t, false, auditTarget)
			assert.Equal(t, new(true), getDiagnostics(t).Config.AuditLoggingConfigured)
		})

		t.Run("advanced target without advanced logging license", func(t *testing.T) {
			setAudit(t, false, auditTarget)
			unlicensed := model.NewTestLicense("ldap")
			unlicensed.Features.AdvancedLogging = new(false)
			require.True(t, th.Service.SetLicense(unlicensed))
			t.Cleanup(func() {
				require.True(t, th.Service.SetLicense(license))
			})

			assert.Equal(t, new(false), getDiagnostics(t).Config.AuditLoggingConfigured)
		})
	})

	t.Run("filestore fails", func(t *testing.T) {
		fb := &fmocks.FileBackend{}
		err := SetFileStore(fb)(th.Service)
		require.NoError(t, err)
		fb.On("DriverName").Return("mock")
		fb.On("TestConnection").Return(errors.New("all broken"))

		packet := getDiagnostics(t)

		assert.Equal(t, "FAIL", packet.FileStore.Status)
		assert.Equal(t, "all broken", packet.FileStore.Error)
		assert.Equal(t, "mock", packet.FileStore.Driver)
	})

	t.Run("s3 driver omits disk space fields", func(t *testing.T) {
		orig := th.Service.filestore
		t.Cleanup(func() {
			err := SetFileStore(orig)(th.Service)
			require.NoError(t, err)
		})

		fb := &fmocks.FileBackend{}
		err := SetFileStore(fb)(th.Service)
		require.NoError(t, err)
		fb.On("DriverName").Return("amazons3")
		fb.On("TestConnection").Return(nil)

		packet := getDiagnostics(t)

		assert.Equal(t, "OK", packet.FileStore.Status)
		assert.Equal(t, "amazons3", packet.FileStore.Driver)
		assert.Nil(t, packet.FileStore.FilesystemType)
		assert.Nil(t, packet.FileStore.TotalMB)
		assert.Nil(t, packet.FileStore.AvailableMB)
	})

	t.Run("no LDAP info if LDAP sync is disabled", func(t *testing.T) {
		ldapMock := &emocks.LdapDiagnosticInterface{}
		originalLDAP := th.Service.ldapDiagnostic
		t.Cleanup(func() {
			th.Service.ldapDiagnostic = originalLDAP
		})
		th.Service.ldapDiagnostic = ldapMock

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusDisabled, packet.LDAP.Status)
		assert.Equal(t, "", packet.LDAP.ServerName)
		assert.Equal(t, "", packet.LDAP.ServerVersion)
	})

	th.Service.UpdateConfig(func(cfg *model.Config) {
		cfg.LdapSettings.EnableSync = new(true)
	})

	t.Run("no LDAP vendor info found", func(t *testing.T) {
		ldapMock := &emocks.LdapDiagnosticInterface{}
		ldapMock.On(
			"GetVendorNameAndVendorVersion",
			mock.AnythingOfType("*request.Context"),
		).Return("", "", nil)
		ldapMock.On(
			"RunTest",
			mock.AnythingOfType("*request.Context"),
		).Return(nil)
		originalLDAP := th.Service.ldapDiagnostic
		t.Cleanup(func() {
			th.Service.ldapDiagnostic = originalLDAP
		})
		th.Service.ldapDiagnostic = ldapMock

		packet := getDiagnostics(t)

		assert.Equal(t, "OK", packet.LDAP.Status)
		assert.Empty(t, packet.LDAP.Error)
		assert.Equal(t, "unknown", packet.LDAP.ServerName)
		assert.Equal(t, "unknown", packet.LDAP.ServerVersion)
	})

	t.Run("found LDAP vendor info", func(t *testing.T) {
		ldapMock := &emocks.LdapDiagnosticInterface{}
		ldapMock.On(
			"GetVendorNameAndVendorVersion",
			mock.AnythingOfType("*request.Context"),
		).Return("some vendor", "v1.0.0", nil)
		ldapMock.On(
			"RunTest",
			mock.AnythingOfType("*request.Context"),
		).Return(nil)
		originalLDAP := th.Service.ldapDiagnostic
		t.Cleanup(func() {
			th.Service.ldapDiagnostic = originalLDAP
		})
		th.Service.ldapDiagnostic = ldapMock

		packet := getDiagnostics(t)

		assert.Equal(t, "OK", packet.LDAP.Status)
		assert.Empty(t, packet.LDAP.Error)
		assert.Equal(t, "some vendor", packet.LDAP.ServerName)
		assert.Equal(t, "v1.0.0", packet.LDAP.ServerVersion)
	})

	t.Run("LDAP test fails", func(t *testing.T) {
		ldapMock := &emocks.LdapDiagnosticInterface{}
		ldapMock.On(
			"GetVendorNameAndVendorVersion",
			mock.AnythingOfType("*request.Context"),
		).Return("some vendor", "v1.0.0", nil)
		ldapMock.On(
			"RunTest",
			mock.AnythingOfType("*request.Context"),
		).Return(model.NewAppError("", "some error", nil, "", 0))
		originalLDAP := th.Service.ldapDiagnostic
		t.Cleanup(func() {
			th.Service.ldapDiagnostic = originalLDAP
		})
		th.Service.ldapDiagnostic = ldapMock

		packet := getDiagnostics(t)

		assert.Equal(t, "FAIL", packet.LDAP.Status)
		assert.Equal(t, "some error", packet.LDAP.Error)
		assert.Equal(t, "unknown", packet.LDAP.ServerName)
		assert.Equal(t, "unknown", packet.LDAP.ServerVersion)
	})

	t.Run("SAML disabled", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.SamlSettings.Enable = new(false)
		})

		packet := getDiagnostics(t)

		assert.Empty(t, packet.SAML.ProviderType)
		assert.Equal(t, model.StatusDisabled, packet.SAML.Status)
		assert.Empty(t, packet.SAML.Error)
	})

	t.Run("SAML enabled with passing diagnostic", func(t *testing.T) {
		diagMock := &emocks.SamlDiagnosticInterface{}
		diagMock.On(
			"RunSupportPacketTest",
			mock.AnythingOfType("*request.Context"),
			mock.AnythingOfType("model.SamlSettings"),
		).Return(nil)
		originalSAMLDiag := th.Service.samlDiagnostic
		t.Cleanup(func() { th.Service.samlDiagnostic = originalSAMLDiag })
		th.Service.samlDiagnostic = diagMock

		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.SamlSettings.Enable = model.NewPointer(true)
			cfg.SamlSettings.Verify = model.NewPointer(false)
			cfg.SamlSettings.Encrypt = model.NewPointer(false)
			cfg.SamlSettings.IdpURL = model.NewPointer("http://localhost:8484/realms/mattermost/protocol/saml")
			cfg.SamlSettings.IdpDescriptorURL = model.NewPointer("http://localhost:8484/realms/mattermost")
			cfg.SamlSettings.ServiceProviderIdentifier = model.NewPointer("mattermost")
			cfg.SamlSettings.IdpCertificateFile = model.NewPointer("saml-idp.crt")
			cfg.SamlSettings.EmailAttribute = model.NewPointer("email")
			cfg.SamlSettings.UsernameAttribute = model.NewPointer("username")
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusOk, packet.SAML.Status)
		assert.Empty(t, packet.SAML.Error)
		assert.Equal(t, "Keycloak", packet.SAML.ProviderType)
	})

	t.Run("SAML diagnostics enterprise interface override", func(t *testing.T) {
		diagMock := &emocks.SamlDiagnosticInterface{}
		diagMock.On(
			"RunSupportPacketTest",
			mock.AnythingOfType("*request.Context"),
			mock.AnythingOfType("model.SamlSettings"),
		).Return(errors.New("enterprise check failed"))
		originalSAMLDiag := th.Service.samlDiagnostic
		t.Cleanup(func() {
			th.Service.samlDiagnostic = originalSAMLDiag
		})
		th.Service.samlDiagnostic = diagMock

		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.SamlSettings.Enable = model.NewPointer(true)
			cfg.SamlSettings.Verify = model.NewPointer(false)
			cfg.SamlSettings.Encrypt = model.NewPointer(false)
			cfg.SamlSettings.IdpURL = model.NewPointer("http://localhost:8484/realms/mattermost/protocol/saml")
			cfg.SamlSettings.IdpDescriptorURL = model.NewPointer("http://localhost:8484/realms/mattermost")
			cfg.SamlSettings.ServiceProviderIdentifier = model.NewPointer("mattermost")
			cfg.SamlSettings.IdpCertificateFile = model.NewPointer("saml-idp.crt")
			cfg.SamlSettings.EmailAttribute = model.NewPointer("email")
			cfg.SamlSettings.UsernameAttribute = model.NewPointer("username")
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.SAML.Status)
		assert.Equal(t, "enterprise check failed", packet.SAML.Error)
	})

	t.Run("SAML enabled with Keycloak provider", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.SamlSettings.Enable = new(true)
			cfg.SamlSettings.Verify = new(false)
			cfg.SamlSettings.Encrypt = new(false)
			cfg.SamlSettings.IdpURL = new("http://localhost:8484/realms/mattermost/protocol/saml")
			cfg.SamlSettings.IdpDescriptorURL = new("http://localhost:8484/realms/mattermost")
			cfg.SamlSettings.ServiceProviderIdentifier = new("mattermost")
			cfg.SamlSettings.IdpCertificateFile = new("saml-idp.crt")
			cfg.SamlSettings.EmailAttribute = new("email")
			cfg.SamlSettings.UsernameAttribute = new("username")
		})

		packet := getDiagnostics(t)

		assert.Equal(t, "Keycloak", packet.SAML.ProviderType)
	})

	t.Run("SAML enabled with ADFS provider", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.SamlSettings.Enable = new(true)
			cfg.SamlSettings.Verify = new(false)
			cfg.SamlSettings.Encrypt = new(false)
			cfg.SamlSettings.IdpURL = new("https://adfs.company.com/adfs/ls")
			cfg.SamlSettings.IdpDescriptorURL = new("https://adfs.company.com/adfs/services/trust")
			cfg.SamlSettings.ServiceProviderIdentifier = new("mattermost")
			cfg.SamlSettings.IdpCertificateFile = new("saml-idp.crt")
			cfg.SamlSettings.EmailAttribute = new("email")
			cfg.SamlSettings.UsernameAttribute = new("username")
		})

		packet := getDiagnostics(t)

		assert.Equal(t, "ADFS", packet.SAML.ProviderType)
	})

	t.Run("SAML enabled with unknown provider", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.SamlSettings.Enable = new(true)
			cfg.SamlSettings.Verify = new(false)
			cfg.SamlSettings.Encrypt = new(false)
			cfg.SamlSettings.IdpURL = new("https://custom-saml.example.com/sso/login")
			cfg.SamlSettings.IdpDescriptorURL = new("https://custom-saml.example.com/sso")
			cfg.SamlSettings.ServiceProviderIdentifier = new("mattermost")
			cfg.SamlSettings.IdpCertificateFile = new("saml-idp.crt")
			cfg.SamlSettings.EmailAttribute = new("email")
			cfg.SamlSettings.UsernameAttribute = new("username")
		})

		packet := getDiagnostics(t)

		assert.Equal(t, "unknown", packet.SAML.ProviderType)
	})

	t.Run("Elasticsearch config test when indexing disabled", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.ElasticsearchSettings.Backend = model.NewPointer(model.ElasticsearchSettingsESBackend)
			cfg.ElasticsearchSettings.EnableIndexing = new(false)
		})

		esMock := &semocks.SearchEngineInterface{}
		esMock.On("GetFullVersion").Return("7.10.0")
		esMock.On("GetPlugins").Return([]string{"plugin1", "plugin2"})
		originalES := th.Service.SearchEngine.ElasticsearchEngine
		t.Cleanup(func() {
			th.Service.SearchEngine.ElasticsearchEngine = originalES
		})
		th.Service.SearchEngine.ElasticsearchEngine = esMock

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusDisabled, packet.ElasticSearch.Status)
		assert.Equal(t, model.ElasticsearchSettingsESBackend, packet.ElasticSearch.Backend)
		assert.Equal(t, "7.10.0", packet.ElasticSearch.ServerVersion)
		assert.Equal(t, []string{"plugin1", "plugin2"}, packet.ElasticSearch.ServerPlugins)
		assert.Empty(t, packet.ElasticSearch.Error)
	})

	t.Run("Elasticsearch config test when indexing enabled and config valid", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.ElasticsearchSettings.Backend = model.NewPointer(model.ElasticsearchSettingsOSBackend)
			cfg.ElasticsearchSettings.EnableIndexing = new(true)
		})

		esMock := &semocks.SearchEngineInterface{}
		esMock.On("GetFullVersion").Return("2.5.0")
		esMock.On("GetPlugins").Return([]string{"opensearch-plugin"})
		esMock.On("TestConfig", mock.AnythingOfType("*request.Context"), mock.Anything).Return(nil)
		originalES := th.Service.SearchEngine.ElasticsearchEngine
		t.Cleanup(func() {
			th.Service.SearchEngine.ElasticsearchEngine = originalES
		})
		th.Service.SearchEngine.ElasticsearchEngine = esMock

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusOk, packet.ElasticSearch.Status)
		assert.Equal(t, model.ElasticsearchSettingsOSBackend, packet.ElasticSearch.Backend)
		assert.Equal(t, "2.5.0", packet.ElasticSearch.ServerVersion)
		assert.Equal(t, []string{"opensearch-plugin"}, packet.ElasticSearch.ServerPlugins)
		assert.Empty(t, packet.ElasticSearch.Error)
	})

	t.Run("Elasticsearch config test when indexing enabled and config invalid", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.ElasticsearchSettings.Backend = model.NewPointer(model.ElasticsearchSettingsESBackend)
			cfg.ElasticsearchSettings.EnableIndexing = new(true)
		})

		esMock := &semocks.SearchEngineInterface{}
		esMock.On("GetFullVersion").Return("7.10.0")
		esMock.On("GetPlugins").Return([]string{"plugin1", "plugin2"})
		esMock.On("TestConfig", mock.AnythingOfType("*request.Context"), mock.Anything).Return(
			model.NewAppError("TestConfig", "ent.elasticsearch.test_config.connection_failed", nil, "connection refused", 500))
		originalES := th.Service.SearchEngine.ElasticsearchEngine
		t.Cleanup(func() {
			th.Service.SearchEngine.ElasticsearchEngine = originalES
		})
		th.Service.SearchEngine.ElasticsearchEngine = esMock

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.ElasticSearch.Status)
		assert.Equal(t, model.ElasticsearchSettingsESBackend, packet.ElasticSearch.Backend)
		assert.Equal(t, "7.10.0", packet.ElasticSearch.ServerVersion)
		assert.Equal(t, []string{"plugin1", "plugin2"}, packet.ElasticSearch.ServerPlugins)
		assert.Equal(t, "TestConfig: ent.elasticsearch.test_config.connection_failed, connection refused", packet.ElasticSearch.Error)
	})

	t.Run("push notifications disabled", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.EmailSettings.SendPushNotifications = new(false)
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.EmailSettings.SendPushNotifications = new(true)
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusDisabled, packet.Notifications.Push.Status)
		assert.Empty(t, packet.Notifications.Push.Error)
	})

	t.Run("push notifications reachable", func(t *testing.T) {
		pushServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/version", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer pushServer.Close()

		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.EmailSettings.SendPushNotifications = new(true)
			cfg.EmailSettings.PushNotificationServer = new(pushServer.URL)
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.EmailSettings.SendPushNotifications = new(true)
				cfg.EmailSettings.PushNotificationServer = model.NewPointer(model.GenericNotificationServer)
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusOk, packet.Notifications.Push.Status)
		assert.Empty(t, packet.Notifications.Push.Error)
	})

	t.Run("push notifications unreachable", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.EmailSettings.SendPushNotifications = new(true)
			cfg.EmailSettings.PushNotificationServer = new("http://localhost:1")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.EmailSettings.SendPushNotifications = new(true)
				cfg.EmailSettings.PushNotificationServer = model.NewPointer(model.GenericNotificationServer)
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.Notifications.Push.Status)
		assert.NotEmpty(t, packet.Notifications.Push.Error)
	})

	t.Run("SMTP unused", func(t *testing.T) {
		requireVerification := *th.Service.Config().EmailSettings.RequireEmailVerification
		emailInvitations := *th.Service.Config().ServiceSettings.EnableEmailInvitations
		signInWithEmail := *th.Service.Config().EmailSettings.EnableSignInWithEmail
		signInWithUsername := *th.Service.Config().EmailSettings.EnableSignInWithUsername
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.EmailSettings.SendEmailNotifications = new(false)
			cfg.EmailSettings.RequireEmailVerification = new(false)
			cfg.ServiceSettings.EnableEmailInvitations = new(false)
			cfg.EmailSettings.EnableSignInWithEmail = new(false)
			cfg.EmailSettings.EnableSignInWithUsername = new(false)
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.EmailSettings.SendEmailNotifications = new(true)
				cfg.EmailSettings.RequireEmailVerification = new(requireVerification)
				cfg.ServiceSettings.EnableEmailInvitations = new(emailInvitations)
				cfg.EmailSettings.EnableSignInWithEmail = new(signInWithEmail)
				cfg.EmailSettings.EnableSignInWithUsername = new(signInWithUsername)
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusDisabled, packet.Notifications.Email.Status)
		assert.Empty(t, packet.Notifications.Email.Error)
	})

	t.Run("email notifications reachable", func(t *testing.T) {
		l, listenErr := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, listenErr)
		defer l.Close()

		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				go func(c net.Conn) {
					defer c.Close()
					rw := bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c))
					_, _ = rw.WriteString("220 localhost ESMTP Test\r\n")
					rw.Flush()
					for {
						line, err := rw.ReadString('\n')
						if err != nil {
							return
						}
						if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "QUIT") {
							_, _ = rw.WriteString("221 Bye\r\n")
							rw.Flush()
							return
						}
						_, _ = rw.WriteString("250 OK\r\n")
						rw.Flush()
					}
				}(conn)
			}
		}()

		tcpAddr := l.Addr().(*net.TCPAddr)
		smtpPort := strconv.Itoa(tcpAddr.Port)

		// MM_EMAILSETTINGS_SMTPSERVER may be set in CI and would override UpdateConfig.
		// Use t.Setenv so the env var is updated before UpdateConfig calls Store.Set(),
		// which re-reads GetEnvironment() (os.Environ()) and applies overrides.
		t.Setenv("MM_EMAILSETTINGS_SMTPSERVER", "127.0.0.1")

		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.EmailSettings.SendEmailNotifications = new(true)
			cfg.EmailSettings.SMTPServer = new("127.0.0.1")
			cfg.EmailSettings.SMTPPort = new(smtpPort)
			cfg.EmailSettings.EnableSMTPAuth = new(false)
			cfg.EmailSettings.ConnectionSecurity = new("")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.EmailSettings.SendEmailNotifications = new(true)
				cfg.EmailSettings.SMTPServer = model.NewPointer(model.EmailSMTPDefaultServer)
				cfg.EmailSettings.SMTPPort = model.NewPointer(model.EmailSMTPDefaultPort)
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusOk, packet.Notifications.Email.Status)
		assert.Empty(t, packet.Notifications.Email.Error)
	})

	t.Run("email notifications unreachable", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.EmailSettings.SendEmailNotifications = new(true)
			cfg.EmailSettings.SMTPServer = new("localhost")
			cfg.EmailSettings.SMTPPort = new("1")
			cfg.EmailSettings.SMTPServerTimeout = new(1)
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.EmailSettings.SendEmailNotifications = new(true)
				cfg.EmailSettings.SMTPServer = model.NewPointer(model.EmailSMTPDefaultServer)
				cfg.EmailSettings.SMTPPort = model.NewPointer(model.EmailSMTPDefaultPort)
				cfg.EmailSettings.SMTPServerTimeout = new(10)
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.Notifications.Email.Status)
		assert.NotEmpty(t, packet.Notifications.Email.Error)
	})

	t.Run("email invitations probe SMTP with notification emails off", func(t *testing.T) {
		emailInvitations := *th.Service.Config().ServiceSettings.EnableEmailInvitations
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.EmailSettings.SendEmailNotifications = new(false)
			cfg.ServiceSettings.EnableEmailInvitations = new(true)
			cfg.EmailSettings.SMTPServer = new("localhost")
			cfg.EmailSettings.SMTPPort = new("1")
			cfg.EmailSettings.SMTPServerTimeout = new(1)
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.EmailSettings.SendEmailNotifications = new(true)
				cfg.ServiceSettings.EnableEmailInvitations = new(emailInvitations)
				cfg.EmailSettings.SMTPServer = model.NewPointer(model.EmailSMTPDefaultServer)
				cfg.EmailSettings.SMTPPort = model.NewPointer(model.EmailSMTPDefaultPort)
				cfg.EmailSettings.SMTPServerTimeout = new(10)
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.Notifications.Email.Status)
		assert.NotEmpty(t, packet.Notifications.Email.Error)
	})

	t.Run("maps connection pool diagnostics for master and replica", func(t *testing.T) {
		originalStore := th.Service.Store
		customStore := &fixedDBStatsStore{
			Store: originalStore,
			masterStats: sql.DBStats{
				InUse:             3,
				Idle:              7,
				WaitCount:         11,
				WaitDuration:      2*time.Second + 25*time.Millisecond,
				MaxIdleClosed:     13,
				MaxLifetimeClosed: 17,
			},
			replicaStats: sql.DBStats{
				InUse:             5,
				Idle:              9,
				WaitCount:         19,
				WaitDuration:      4*time.Second + 90*time.Millisecond,
				MaxIdleClosed:     23,
				MaxLifetimeClosed: 29,
			},
			replicasOnline: 2,
		}
		th.Service.Store = customStore
		t.Cleanup(func() {
			th.Service.Store = originalStore
		})

		packet := getDiagnostics(t)
		require.NotNil(t, packet.Database.MasterConnectionsInUse)
		assert.Equal(t, 3, *packet.Database.MasterConnectionsInUse)
		assert.Equal(t, new(7), packet.Database.MasterConnectionsIdle)
		assert.Equal(t, new(int64(11)), packet.Database.MasterPoolWaitCount)
		assert.Equal(t, new(int64(2025)), packet.Database.MasterPoolWaitDurationMs)
		assert.Equal(t, new(int64(13)), packet.Database.MasterConnectionsClosedMaxIdle)
		assert.Equal(t, new(int64(17)), packet.Database.MasterConnectionsClosedMaxLifetime)
		assert.Equal(t, new(5), packet.Database.ReplicaConnectionsInUse)
		assert.Equal(t, new(9), packet.Database.ReplicaConnectionsIdle)
		assert.Equal(t, new(int64(19)), packet.Database.ReplicaPoolWaitCount)
		assert.Equal(t, new(int64(4090)), packet.Database.ReplicaPoolWaitDurationMs)
		assert.Equal(t, new(int64(23)), packet.Database.ReplicaConnectionsClosedMaxIdle)
		assert.Equal(t, new(int64(29)), packet.Database.ReplicaConnectionsClosedMaxLifetime)
		assert.Equal(t, new(2), packet.Database.ReplicasOnline)
	})

	t.Run("OpenID disabled", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.OpenIdSettings.Enable = model.NewPointer(false)
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusDisabled, packet.OAuthProviders.OpenID.Status)
		assert.Empty(t, packet.OAuthProviders.OpenID.Error)
	})

	t.Run("OpenID reachable via discovery endpoint", func(t *testing.T) {
		idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/.well-known/openid-configuration", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issuer":"https://idp.example.com","authorization_endpoint":"https://idp.example.com/auth"}`))
		}))
		defer idp.Close()

		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.OpenIdSettings.Enable = model.NewPointer(true)
			cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer(idp.URL + "/.well-known/openid-configuration")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.OpenIdSettings.Enable = model.NewPointer(false)
				cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer("")
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusOk, packet.OAuthProviders.OpenID.Status)
		assert.Empty(t, packet.OAuthProviders.OpenID.Error)
	})

	t.Run("OpenID discovery endpoint returns invalid JSON", func(t *testing.T) {
		idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer idp.Close()

		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.OpenIdSettings.Enable = model.NewPointer(true)
			cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer(idp.URL + "/.well-known/openid-configuration")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.OpenIdSettings.Enable = model.NewPointer(false)
				cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer("")
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.OAuthProviders.OpenID.Status)
		assert.Contains(t, packet.OAuthProviders.OpenID.Error, "valid JSON")
	})

	t.Run("OpenID discovery endpoint missing issuer field", func(t *testing.T) {
		idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"authorization_endpoint":"https://idp.example.com/auth"}`))
		}))
		defer idp.Close()

		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.OpenIdSettings.Enable = model.NewPointer(true)
			cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer(idp.URL + "/.well-known/openid-configuration")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.OpenIdSettings.Enable = model.NewPointer(false)
				cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer("")
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.OAuthProviders.OpenID.Status)
		assert.Contains(t, packet.OAuthProviders.OpenID.Error, "issuer")
	})

	t.Run("OpenID discovery endpoint unreachable", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.OpenIdSettings.Enable = model.NewPointer(true)
			cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer("http://127.0.0.1:1/.well-known/openid-configuration")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.OpenIdSettings.Enable = model.NewPointer(false)
				cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer("")
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.OAuthProviders.OpenID.Status)
		assert.NotEmpty(t, packet.OAuthProviders.OpenID.Error)
	})

	t.Run("GitLab enabled with reachable token endpoint", func(t *testing.T) {
		// GitLab has no DiscoveryEndpoint by default, so we fall through to the
		// TokenEndpoint host probe. Token endpoints reject GETs, so any HTTP
		// response (including 4xx/5xx) is treated as reachable.
		idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}))
		defer idp.Close()

		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.GitLabSettings.Enable = model.NewPointer(true)
			cfg.GitLabSettings.DiscoveryEndpoint = model.NewPointer("")
			cfg.GitLabSettings.TokenEndpoint = model.NewPointer(idp.URL + "/oauth/token")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.GitLabSettings.Enable = model.NewPointer(false)
				cfg.GitLabSettings.TokenEndpoint = model.NewPointer("")
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusOk, packet.OAuthProviders.GitLab.Status)
		assert.Empty(t, packet.OAuthProviders.GitLab.Error)
	})

	t.Run("GitLab enabled with unreachable token endpoint", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.GitLabSettings.Enable = model.NewPointer(true)
			cfg.GitLabSettings.DiscoveryEndpoint = model.NewPointer("")
			cfg.GitLabSettings.TokenEndpoint = model.NewPointer("http://127.0.0.1:1/oauth/token")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.GitLabSettings.Enable = model.NewPointer(false)
				cfg.GitLabSettings.TokenEndpoint = model.NewPointer("")
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.OAuthProviders.GitLab.Status)
		assert.NotEmpty(t, packet.OAuthProviders.GitLab.Error)
	})

	t.Run("GitLab enabled with no endpoints configured", func(t *testing.T) {
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.GitLabSettings.Enable = model.NewPointer(true)
			cfg.GitLabSettings.DiscoveryEndpoint = model.NewPointer("")
			cfg.GitLabSettings.TokenEndpoint = model.NewPointer("")
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.GitLabSettings.Enable = model.NewPointer(false)
			})
		})

		packet := getDiagnostics(t)

		assert.Equal(t, model.StatusFail, packet.OAuthProviders.GitLab.Status)
		assert.Contains(t, packet.OAuthProviders.GitLab.Error, "no discovery or token endpoint")
	})
}

func TestGetSupportPacketDiagnosticsSectionErrors(t *testing.T) {
	th := Setup(t)

	setStore := func(t *testing.T, s *failingDiagnosticsStore) {
		originalStore := th.Service.Store
		s.Store = originalStore
		th.Service.Store = s
		t.Cleanup(func() {
			th.Service.Store = originalStore
		})
	}

	requireSectionErrorsWithContext := func(t *testing.T, rctx request.CTX, failed map[model.NodeSection][]string) *model.SupportPacketDiagnostics {
		t.Helper()

		nodeDiagnostics, err := th.Service.GetSupportPacketDiagnostics(rctx)
		require.NotNil(t, nodeDiagnostics)
		require.NotNil(t, nodeDiagnostics.Diagnostics)
		if len(failed) == 0 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}

		require.Len(t, nodeDiagnostics.Errors, len(model.AllNodeSections()))
		for _, section := range model.AllNodeSections() {
			sectionErr, ok := nodeDiagnostics.Errors[section]
			require.True(t, ok, "section %q should be present", section)

			messages, isFailed := failed[section]
			if !isFailed {
				assert.NoError(t, sectionErr, "section %q", section)
				continue
			}
			require.Error(t, sectionErr, "section %q", section)
			for _, msg := range messages {
				assert.ErrorContains(t, sectionErr, msg)
				assert.ErrorContains(t, err, msg)
			}
		}
		return nodeDiagnostics.Diagnostics
	}
	requireSectionErrors := func(t *testing.T, failed map[model.NodeSection][]string) *model.SupportPacketDiagnostics {
		t.Helper()
		return requireSectionErrorsWithContext(t, th.Context, failed)
	}

	setLDAP := func(t *testing.T, ldapMock *emocks.LdapDiagnosticInterface) {
		originalLDAP := th.Service.ldapDiagnostic
		t.Cleanup(func() {
			th.Service.ldapDiagnostic = originalLDAP
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.LdapSettings.EnableSync = model.NewPointer(false)
			})
		})
		th.Service.ldapDiagnostic = ldapMock
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.LdapSettings.EnableSync = model.NewPointer(true)
		})
	}

	t.Run("all sections present and clean", func(t *testing.T) {
		requireSectionErrors(t, nil)
	})

	t.Run("DB schema version fails", func(t *testing.T) {
		setStore(t, &failingDiagnosticsStore{schemaVersionErr: errors.New("schema down")})

		d := requireSectionErrors(t, map[model.NodeSection][]string{
			model.SectionDatabaseIdentity: {"error while getting DB type and schema version"},
		})
		assert.Nil(t, d.Database.Type)
		assert.Nil(t, d.Database.SchemaVersion)
	})

	t.Run("DB version fails", func(t *testing.T) {
		setStore(t, &failingDiagnosticsStore{dbVersionErr: errors.New("version down")})

		d := requireSectionErrors(t, map[model.NodeSection][]string{
			model.SectionDatabaseIdentity: {"error while getting DB version"},
		})
		assert.Nil(t, d.Database.Version)
	})

	t.Run("both DB identity sites fail", func(t *testing.T) {
		setStore(t, &failingDiagnosticsStore{
			schemaVersionErr: errors.New("schema down"),
			dbVersionErr:     errors.New("version down"),
		})

		requireSectionErrors(t, map[model.NodeSection][]string{
			model.SectionDatabaseIdentity: {"schema down", "version down"},
		})
	})

	t.Run("store diagnostics fail", func(t *testing.T) {
		setStore(t, &failingDiagnosticsStore{diagnosticsErr: errors.New("stats down")})

		d := requireSectionErrors(t, map[model.NodeSection][]string{
			model.SectionDatabaseStats: {"error while collecting support packet database diagnostics"},
		})
		assert.Nil(t, d.Database.MasterConnectionsInUse)
		assert.Nil(t, d.Database.MasterConnectionsIdle)
		assert.Nil(t, d.Database.MasterPoolWaitCount)
		assert.Nil(t, d.Database.MasterPoolWaitDurationMs)
		assert.Nil(t, d.Database.MasterConnectionsClosedMaxIdle)
		assert.Nil(t, d.Database.MasterConnectionsClosedMaxLifetime)
		assert.Nil(t, d.Database.ReplicaConnectionsInUse)
		assert.Nil(t, d.Database.ReplicaConnectionsIdle)
		assert.Nil(t, d.Database.ReplicaPoolWaitCount)
		assert.Nil(t, d.Database.ReplicaPoolWaitDurationMs)
		assert.Nil(t, d.Database.ReplicaConnectionsClosedMaxIdle)
		assert.Nil(t, d.Database.ReplicaConnectionsClosedMaxLifetime)
		assert.Nil(t, d.Database.ReplicasOnline)
	})

	t.Run("partial store diagnostics fail only the stats section", func(t *testing.T) {
		deadTuples := int64(200_000)
		lastVacuum := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
		setStore(t, &failingDiagnosticsStore{
			diagnostics: &store.DatabaseDiagnostics{
				MasterConnectionsInUse: 3,
				PostsDeadTuples:        &deadTuples,
				PostsLastVacuum:        &lastVacuum,
			},
			diagnosticsErr: errors.New("one pg_stat query failed"),
		})

		d := requireSectionErrors(t, map[model.NodeSection][]string{
			model.SectionDatabaseStats: {"error while collecting support packet database diagnostics"},
		})
		require.NotNil(t, d.Database.MasterConnectionsInUse)
		assert.Equal(t, 3, *d.Database.MasterConnectionsInUse)
		assert.Equal(t, &deadTuples, d.Database.PostsDeadTuples)
		assert.Equal(t, &lastVacuum, d.Database.PostsLastVacuum)
	})

	t.Run("disk space fails", func(t *testing.T) {
		originalFileStore := th.Service.filestore
		originalDir := *th.Service.Config().FileSettings.Directory
		t.Cleanup(func() {
			err := SetFileStore(originalFileStore)(th.Service)
			require.NoError(t, err)
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.FileSettings.Directory = model.NewPointer(originalDir)
			})
		})

		fb := &fmocks.FileBackend{}
		fb.On("DriverName").Return(model.ImageDriverLocal)
		fb.On("TestConnection").Return(nil)
		err := SetFileStore(fb)(th.Service)
		require.NoError(t, err)
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.FileSettings.Directory = model.NewPointer(filepath.Join(t.TempDir(), "missing"))
		})

		d := requireSectionErrors(t, map[model.NodeSection][]string{
			model.SectionFilestoreDisk: {"error while getting disk space info"},
		})
		assert.Nil(t, d.FileStore.FilesystemType)
		assert.Nil(t, d.FileStore.TotalMB)
		assert.Nil(t, d.FileStore.AvailableMB)
	})

	t.Run("cluster infos fail", func(t *testing.T) {
		cluster := emocks.NewClusterInterface(t)
		// Setup's background config publishes reach whichever cluster is installed.
		cluster.On("SendClusterMessage", mock.Anything).Return().Maybe()
		cluster.On("GetClusterId").Return("cluster-id")
		cluster.On("IsLeader").Return(false)
		cluster.On("GetClusterInfos").Return(nil, errors.New("gossip down"))
		originalCluster := th.Service.clusterIFace
		t.Cleanup(func() {
			th.Service.clusterIFace = originalCluster
		})
		th.Service.clusterIFace = cluster

		d := requireSectionErrors(t, map[model.NodeSection][]string{
			model.SectionCluster: {"error while getting cluster infos"},
		})
		assert.Nil(t, d.Cluster.NumberOfNodes)
		assert.Nil(t, d.Cluster.NumberOfConnectedNodes)
	})

	t.Run("LDAP vendor info fails", func(t *testing.T) {
		ldapMock := &emocks.LdapDiagnosticInterface{}
		ldapMock.On("RunTest", mock.AnythingOfType("*request.Context")).Return(nil)
		ldapMock.On("GetVendorNameAndVendorVersion", mock.AnythingOfType("*request.Context")).Return("", "", errors.New("vendor down"))
		originalLDAP := th.Service.ldapDiagnostic
		t.Cleanup(func() {
			th.Service.ldapDiagnostic = originalLDAP
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.LdapSettings.EnableSync = model.NewPointer(false)
			})
		})
		th.Service.ldapDiagnostic = ldapMock
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.LdapSettings.EnableSync = model.NewPointer(true)
		})

		requireSectionErrors(t, map[model.NodeSection][]string{
			model.SectionLDAPProbe: {"error while getting LDAP vendor info"},
		})
	})

	t.Run("failed probe stays present and clean", func(t *testing.T) {
		ldapMock := &emocks.LdapDiagnosticInterface{}
		ldapMock.On("RunTest", mock.AnythingOfType("*request.Context")).Return(model.NewAppError("", "bind failed", nil, "", 0))
		originalLDAP := th.Service.ldapDiagnostic
		t.Cleanup(func() {
			th.Service.ldapDiagnostic = originalLDAP
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.LdapSettings.EnableSync = model.NewPointer(false)
			})
		})
		th.Service.ldapDiagnostic = ldapMock
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.LdapSettings.EnableSync = model.NewPointer(true)
		})

		requireSectionErrors(t, nil)
	})

	t.Run("probe still running at the deadline", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		// The push probe reaches the real push proxy by default, which could outlast the short deadline.
		th.Service.UpdateConfig(func(cfg *model.Config) {
			cfg.EmailSettings.SendPushNotifications = model.NewPointer(false)
		})
		t.Cleanup(func() {
			th.Service.UpdateConfig(func(cfg *model.Config) {
				cfg.EmailSettings.SendPushNotifications = model.NewPointer(true)
			})
		})
		ldapMock := &emocks.LdapDiagnosticInterface{}
		ldapMock.On("RunTest", mock.AnythingOfType("*request.Context")).
			Run(func(mock.Arguments) { <-release }).
			Return(model.NewAppError("", "released", nil, "", 0))
		setLDAP(t, ldapMock)

		// Long enough for the local collection, which also uses the context for its database queries.
		ctx, cancel := context.WithTimeout(th.Context.Context(), 2*time.Second)
		t.Cleanup(cancel)

		start := time.Now()
		d := requireSectionErrorsWithContext(t, th.Context.WithContext(ctx), map[model.NodeSection][]string{
			model.SectionLDAPProbe: {"probe did not complete", context.DeadlineExceeded.Error()},
		})
		assert.Less(t, time.Since(start), 10*time.Second)
		assert.Empty(t, d.LDAP.Status, "a probe that did not finish has no status")
		assert.Contains(t, d.LDAP.Error, "probe did not complete")
	})

	t.Run("probes run concurrently", func(t *testing.T) {
		originalFileStore := th.Service.filestore
		t.Cleanup(func() {
			err := SetFileStore(originalFileStore)(th.Service)
			require.NoError(t, err)
		})

		// Each probe waits for the other to start, so they only both succeed when they run at the same time.
		ldapStarted := make(chan struct{})
		fileStoreStarted := make(chan struct{})
		waitFor := func(started chan struct{}) error {
			select {
			case <-started:
				return nil
			case <-time.After(10 * time.Second):
				return errors.New("other probe did not start")
			}
		}

		fb := &fmocks.FileBackend{}
		fb.On("DriverName").Return("amazons3")
		fb.On("TestConnection").Return(func() error {
			close(fileStoreStarted)
			return waitFor(ldapStarted)
		})
		err := SetFileStore(fb)(th.Service)
		require.NoError(t, err)

		ldapMock := &emocks.LdapDiagnosticInterface{}
		ldapMock.On("RunTest", mock.AnythingOfType("*request.Context")).Return(func(request.CTX) *model.AppError {
			close(ldapStarted)
			if err := waitFor(fileStoreStarted); err != nil {
				return model.NewAppError("", "timeout", nil, "", 0).Wrap(err)
			}
			return nil
		})
		ldapMock.On("GetVendorNameAndVendorVersion", mock.AnythingOfType("*request.Context")).Return("OpenLDAP", "2.6", nil)
		setLDAP(t, ldapMock)

		d := requireSectionErrors(t, nil)
		assert.Equal(t, model.StatusOk, d.FileStore.Status, d.FileStore.Error)
		assert.Equal(t, model.StatusOk, d.LDAP.Status, d.LDAP.Error)
		assert.Equal(t, "OpenLDAP", d.LDAP.ServerName)
	})
}

func TestStartNetworkProbes(t *testing.T) {
	type outcome struct {
		status, errMsg string
		sectionErrs    []error
	}
	// collect runs a single probe in section SectionPushProbe and returns what it recorded.
	collect := func(ctx context.Context, probe networkProbe) outcome {
		var o outcome
		probe.name, probe.section, probe.status, probe.errMsg = "push", model.SectionPushProbe, &o.status, &o.errMsg
		startNetworkProbes(ctx, []networkProbe{probe}, func(section model.NodeSection, err error) {
			assert.Equal(t, model.SectionPushProbe, section)
			o.sectionErrs = append(o.sectionErrs, err)
		})()
		return o
	}

	t.Run("records a reachable service", func(t *testing.T) {
		o := collect(context.Background(), networkProbe{run: func(context.Context) error { return nil }})
		assert.Equal(t, outcome{status: model.StatusOk}, o)
	})

	t.Run("records an unreachable service without failing the section", func(t *testing.T) {
		o := collect(context.Background(), networkProbe{run: func(context.Context) error { return errors.New("refused") }})
		assert.Equal(t, outcome{status: model.StatusFail, errMsg: "refused"}, o)
	})

	t.Run("runs record after run and fails the section with its error", func(t *testing.T) {
		var name string
		o := collect(context.Background(), networkProbe{
			run: func(context.Context) error { name = "OpenLDAP"; return nil },
			record: func() error {
				assert.Equal(t, "OpenLDAP", name)
				return errors.New("vendor down")
			},
		})
		assert.Equal(t, model.StatusOk, o.status)
		require.Len(t, o.sectionErrs, 1)
		assert.EqualError(t, o.sectionErrs[0], "vendor down")
	})

	t.Run("passes the context to the probe", func(t *testing.T) {
		type key struct{}
		ctx := context.WithValue(context.Background(), key{}, "value")
		o := collect(ctx, networkProbe{run: func(ctx context.Context) error {
			if ctx.Value(key{}) != "value" {
				return errors.New("wrong context")
			}
			return nil
		}})
		assert.Equal(t, model.StatusOk, o.status, o.errMsg)
	})

	t.Run("gives up when the context ends", func(t *testing.T) {
		release := make(chan struct{})
		defer close(release)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		recorded := false
		o := collect(ctx, networkProbe{
			run:    func(context.Context) error { <-release; return nil },
			record: func() error { recorded = true; return nil },
		})
		assert.Empty(t, o.status, "a probe that did not finish has no status")
		assert.Equal(t, "push: probe did not complete: context deadline exceeded", o.errMsg)
		require.Len(t, o.sectionErrs, 1)
		assert.ErrorIs(t, o.sectionErrs[0], context.DeadlineExceeded)
		assert.False(t, recorded)
	})

	t.Run("records probes that finished before the context ended", func(t *testing.T) {
		// With the deadline passed and results waiting, select picks either case, so repeat to catch a wrong pick.
		for range 20 {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				var o outcome
				collectProbes := startNetworkProbes(ctx, []networkProbe{{
					name: "push", section: model.SectionPushProbe, status: &o.status, errMsg: &o.errMsg,
					run: func(context.Context) error { return nil },
				}}, func(_ model.NodeSection, err error) { o.sectionErrs = append(o.sectionErrs, err) })

				synctest.Wait() // the probe has sent its result
				cancel()
				collectProbes()
				assert.Equal(t, outcome{status: model.StatusOk}, o)
			})
		}
	})

	t.Run("returns a panic as an error", func(t *testing.T) {
		o := collect(context.Background(), networkProbe{run: func(context.Context) error { panic("boom") }})
		assert.Empty(t, o.status)
		assert.Equal(t, "push: probe panicked: boom", o.errMsg)
		require.Len(t, o.sectionErrs, 1)
	})

	t.Run("records nothing without probes", func(t *testing.T) {
		failed := false
		startNetworkProbes(context.Background(), nil, func(model.NodeSection, error) { failed = true })()
		assert.False(t, failed)
	})
}

func TestGetSupportPacketConfig(t *testing.T) {
	// t.Setenv is correct here: this test verifies that feature flags set via
	// environment variables (the production mechanism) appear in the sanitized
	// config output. UpdateConfig won't work because SetDefaults() resets
	// FeatureFlags before applyEnvironmentMap() re-applies env overrides.
	t.Setenv("MM_FEATUREFLAGS_TestFeature", "true")

	th := Setup(t)

	th.Service.UpdateConfig(func(cfg *model.Config) {
		cfg.ServiceSettings.AllowedUntrustedInternalConnections = new("example.com")
	})

	// Happy path where we have a sanitized config file with no err
	config, err := th.Service.GetSupportPacketConfig(th.Context)
	require.NotNil(t, config)
	assert.NoError(t, err)

	// Ensure sensitive fields are redacted
	assert.Equal(t, model.FakeSetting, *config.Config.FileSettings.PublicLinkSalt)

	// Ensure non-sensitive fields are present
	assert.Equal(t, "example.com", *config.Config.ServiceSettings.AllowedUntrustedInternalConnections)

	// Ensure feature flags are present
	assert.Equal(t, "true", config.FeatureFlags.TestFeature)

	// Ensure DataSource is partially sanitized (not completely replaced with FakeSetting)
	// The default test database connection string should have username/password redacted
	assert.Contains(t, *config.Config.SqlSettings.DataSource, "****:****")
	assert.NotEqual(t, model.FakeSetting, *config.Config.SqlSettings.DataSource)
}

func TestGetCPUProfile(t *testing.T) {
	th := Setup(t)

	fileData, err := th.Service.getCPUProfile(th.Context, 100*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, "cpu.prof", fileData.Filename)
	assert.Positive(t, len(fileData.Body))
}

func TestGetHeapProfile(t *testing.T) {
	th := Setup(t)

	fileData, err := th.Service.getHeapProfile(th.Context)
	require.NoError(t, err)
	assert.Equal(t, "heap.prof", fileData.Filename)
	assert.Positive(t, len(fileData.Body))
}

func TestGetGoroutineProfile(t *testing.T) {
	th := Setup(t)

	fileData, err := th.Service.getGoroutineProfile(th.Context)
	require.NoError(t, err)
	assert.Equal(t, "goroutines", fileData.Filename)
	assert.Positive(t, len(fileData.Body))
}

func TestDetectSAMLProviderType(t *testing.T) {
	tests := []struct {
		name             string
		idpDescriptorURL string
		expectedProvider string
	}{
		{
			name:             "Keycloak provider",
			idpDescriptorURL: "http://localhost:8484/realms/mattermost",
			expectedProvider: "Keycloak",
		},
		{
			name:             "ADFS provider",
			idpDescriptorURL: "https://localhost/adfs/services/trust",
			expectedProvider: "ADFS",
		},
		{
			name:             "ADFS provider with bare /adfs path (no trailing slash)",
			idpDescriptorURL: "https://adfs.company.com/adfs",
			expectedProvider: "ADFS",
		},
		{
			name:             "Azure AD provider with login.microsoftonline.com",
			idpDescriptorURL: "https://login.microsoftonline.com/12345/saml2",
			expectedProvider: "Azure AD",
		},
		{
			name:             "Azure AD provider with sts.windows.net",
			idpDescriptorURL: "https://sts.windows.net/12345/",
			expectedProvider: "Azure AD",
		},
		{
			name:             "Okta provider",
			idpDescriptorURL: "https://company.okta.com/app/mattermost/saml",
			expectedProvider: "Okta",
		},
		{
			name:             "Okta preview provider",
			idpDescriptorURL: "https://company.oktapreview.com/app/mattermost/saml",
			expectedProvider: "Okta",
		},
		{
			name:             "Auth0 provider",
			idpDescriptorURL: "https://company.auth0.com/samlp/12345",
			expectedProvider: "Auth0",
		},
		{
			name:             "OneLogin provider",
			idpDescriptorURL: "https://app.onelogin.com/saml/metadata/12345",
			expectedProvider: "OneLogin",
		},
		{
			name:             "Google provider",
			idpDescriptorURL: "https://accounts.google.com/o/saml2?idpid=12345",
			expectedProvider: "Google Workspace",
		},
		{
			name:             "JumpCloud provider",
			idpDescriptorURL: "https://sso.jumpcloud.com/saml2/example",
			expectedProvider: "JumpCloud",
		},
		{
			name:             "Duo provider",
			idpDescriptorURL: "https://sso.duo.com/saml2/sp/12345",
			expectedProvider: "Duo",
		},
		{
			name:             "Centrify provider",
			idpDescriptorURL: "https://company.centrify.com/saml2",
			expectedProvider: "Centrify",
		},
		{
			name:             "Shibboleth provider with shibboleth.net",
			idpDescriptorURL: "https://idp.shibboleth.net/idp/shibboleth",
			expectedProvider: "Shibboleth",
		},
		{
			name:             "Shibboleth provider with /idp/shibboleth path",
			idpDescriptorURL: "https://university.edu/idp/shibboleth",
			expectedProvider: "Shibboleth",
		},
		{
			name:             "Case insensitive - Azure AD",
			idpDescriptorURL: "https://LOGIN.MICROSOFTONLINE.COM/12345/saml2",
			expectedProvider: "Azure AD",
		},
		{
			name:             "Case insensitive - Okta",
			idpDescriptorURL: "https://COMPANY.OKTA.COM/app/mattermost/saml",
			expectedProvider: "Okta",
		},
		{
			name:             "Unknown provider",
			idpDescriptorURL: "https://custom-saml.example.com/sso",
			expectedProvider: "unknown",
		},
		{
			name:             "Empty URL",
			idpDescriptorURL: "",
			expectedProvider: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectSAMLProviderType(tt.idpDescriptorURL)
			assert.Equal(t, tt.expectedProvider, result)
		})
	}
}

func TestSupportPacketMarshalGolden(t *testing.T) {
	t.Parallel()

	cacheHitRatio := 0.99
	deadlocks := int64(3)
	tempFiles := int64(4)
	tempBytesMB := 12.5
	rollbacks := int64(2)
	idleInTxCount := int64(1)
	longestQuerySeconds := 7.25
	waitingForLock := int64(5)
	postsDeadTuples := int64(9)
	postsLiveTuples := int64(41)
	postsLastVacuum := time.Date(2026, 7, 9, 11, 12, 13, 0, time.UTC)
	postsLastAutovacuum := time.Date(2026, 7, 10, 12, 13, 14, 0, time.UTC)

	diagnostics := &model.SupportPacketDiagnostics{
		Version: 2,
	}
	diagnostics.License.Company = "Example Co"
	diagnostics.License.Users = 150
	diagnostics.License.SkuShortName = "enterprise"
	diagnostics.License.IsTrial = true
	diagnostics.License.IsNonProduction = true
	diagnostics.License.ExpiresAt = time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC).UnixMilli()
	diagnostics.License.Features = &model.Features{IDLoadedPushNotifications: new(true), LDAP: new(false)}
	diagnostics.Server.OS = "linux"
	diagnostics.Server.Architecture = "amd64"
	diagnostics.Server.Hostname = new("mm-host")
	diagnostics.Server.InstallationType = "docker"
	diagnostics.Server.CPUCores = 8
	diagnostics.Server.TotalMemoryMB = new(uint64(32768))
	diagnostics.Server.AvailableMemoryMB = new(uint64(24576))
	diagnostics.Server.SwapTotalMB = new(uint64(4096))
	diagnostics.Server.SwapUsedMB = new(uint64(1024))
	diagnostics.Server.ContainerCPULimit = 6.5
	diagnostics.Server.ContainerMemoryLimitMB = 16384
	diagnostics.Server.ContainerMemoryUsageMB = new(uint64(12288))
	diagnostics.Server.ContainerMemoryWorkingSetMB = new(uint64(10240))
	diagnostics.Server.ContainerOOMKills = new(uint64(2))
	diagnostics.Server.LoadAverage1m = new(1.25)
	diagnostics.Server.LoadAverage5m = new(1.5)
	diagnostics.Server.LoadAverage15m = new(2.75)
	diagnostics.Server.ContainerCPUPeriods = new(uint64(10000))
	diagnostics.Server.ContainerCPUThrottledPeriods = new(uint64(1250))
	diagnostics.Server.ContainerCPUThrottledSeconds = new(42.5)
	diagnostics.Server.GoMaxProcs = new(6)
	diagnostics.Server.ProcessID = 90210
	diagnostics.Server.StartedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	diagnostics.Server.HostStartedAt = time.Date(2025, 12, 30, 0, 0, 0, 0, time.UTC)
	diagnostics.Server.ProcessRSSMB = new(uint64(2048))
	diagnostics.Server.GoMemoryLimitMB = new(uint64(13107))
	diagnostics.Server.OpenFileDescriptors = new(int64(512))
	diagnostics.Server.MaxFileDescriptors = new(int64(8192))
	diagnostics.Server.Version = "11.0.0"
	diagnostics.Server.BuildHash = "abc123"
	diagnostics.Server.GoVersion = "go1.26"
	diagnostics.Config.Source = "memory://"
	diagnostics.Config.AuditLoggingConfigured = new(true)
	diagnostics.Database.Type = new("postgres")
	diagnostics.Database.Version = new("16.4")
	diagnostics.Database.SchemaVersion = new("123")
	diagnostics.Database.MasterConnections = 40
	diagnostics.Database.ReplicaConnections = 20
	diagnostics.Database.SearchConnections = 10
	diagnostics.Database.MasterConnectionsInUse = new(5)
	diagnostics.Database.MasterConnectionsIdle = new(35)
	diagnostics.Database.MasterPoolWaitCount = new(int64(100))
	diagnostics.Database.MasterPoolWaitDurationMs = new(int64(220))
	diagnostics.Database.MasterConnectionsClosedMaxIdle = new(int64(2))
	diagnostics.Database.MasterConnectionsClosedMaxLifetime = new(int64(1))
	diagnostics.Database.ReplicaConnectionsInUse = new(3)
	diagnostics.Database.ReplicaConnectionsIdle = new(17)
	diagnostics.Database.ReplicaPoolWaitCount = new(int64(12))
	diagnostics.Database.ReplicaPoolWaitDurationMs = new(int64(66))
	diagnostics.Database.ReplicaConnectionsClosedMaxIdle = new(int64(4))
	diagnostics.Database.ReplicaConnectionsClosedMaxLifetime = new(int64(3))
	diagnostics.Database.ReplicasOnline = new(1)
	diagnostics.Database.CacheHitRatio = &cacheHitRatio
	diagnostics.Database.Deadlocks = &deadlocks
	diagnostics.Database.TempFiles = &tempFiles
	diagnostics.Database.TempBytesMB = &tempBytesMB
	diagnostics.Database.Rollbacks = &rollbacks
	diagnostics.Database.IdleInTransactionCount = &idleInTxCount
	diagnostics.Database.LongestQueryDurationSeconds = &longestQuerySeconds
	diagnostics.Database.WaitingForLockCount = &waitingForLock
	diagnostics.Database.PostsDeadTuples = &postsDeadTuples
	diagnostics.Database.PostsLiveTuples = &postsLiveTuples
	diagnostics.Database.PostsVacuumInProgress = new(true)
	diagnostics.Database.PostsLastVacuum = &postsLastVacuum
	diagnostics.Database.PostsLastAutovacuum = &postsLastAutovacuum
	diagnostics.FileStore.Status = model.StatusOk
	diagnostics.FileStore.Driver = model.ImageDriverLocal
	diagnostics.FileStore.FilesystemType = new("ext4")
	diagnostics.FileStore.TotalMB = new(uint64(204800))
	diagnostics.FileStore.AvailableMB = new(uint64(102400))
	diagnostics.Websocket.Connections = 77
	diagnostics.Cluster.ID = "cluster-id"
	diagnostics.Cluster.NumberOfNodes = new(3)
	diagnostics.Cluster.NumberOfConnectedNodes = new(2)
	diagnostics.Notifications.Email.Status = model.StatusOk
	diagnostics.Notifications.Push.Status = model.StatusFail
	diagnostics.Notifications.Push.Error = "proxy timeout"
	diagnostics.LDAP.Status = model.StatusOk
	diagnostics.LDAP.ServerName = "OpenLDAP"
	diagnostics.LDAP.ServerVersion = "2.6"
	diagnostics.SAML.ProviderType = "Keycloak"
	diagnostics.SAML.Status = model.StatusDisabled
	diagnostics.ElasticSearch.Status = model.StatusOk
	diagnostics.ElasticSearch.Backend = model.ElasticsearchSettingsESBackend
	diagnostics.ElasticSearch.ServerVersion = "8.0.0"
	diagnostics.ElasticSearch.ServerPlugins = []string{"analysis-icu", "ingest-attachment"}
	diagnostics.OAuthProviders.GitLab = model.OAuthProviderStatus{Status: model.StatusOk}
	diagnostics.OAuthProviders.Google = model.OAuthProviderStatus{Status: model.StatusFail, Error: "dial tcp timeout"}
	diagnostics.OAuthProviders.Office365 = model.OAuthProviderStatus{Status: model.StatusDisabled}
	diagnostics.OAuthProviders.OpenID = model.OAuthProviderStatus{Status: model.StatusOk}

	cases := []struct {
		name     string
		filename string
		marshal  func() (*model.FileData, error)
	}{
		{
			name:     "diagnostics",
			filename: "diagnostics.yaml",
			marshal: func() (*model.FileData, error) {
				return supportPacketDiagnosticsFile(diagnostics, nil)
			},
		},
		{
			name:     "config",
			filename: "sanitized_config.json",
			marshal: func() (*model.FileData, error) {
				return supportPacketConfigFile(&model.SupportPacketConfig{
					Config: &model.Config{
						ServiceSettings: model.ServiceSettings{
							SiteURL: model.NewPointer("https://example.test"),
						},
						FeatureFlags: &model.FeatureFlags{TestFeature: "true"},
					},
					FeatureFlags: model.FeatureFlags{TestFeature: "true"},
				}, nil)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fileData, err := tc.marshal()
			require.NoError(t, err)
			require.NotNil(t, fileData)
			require.Equal(t, tc.filename, fileData.Filename)

			expected, err := os.ReadFile(filepath.Join(server.GetPackagePath(), "channels", "app", "platform", "testdata", "support_packet", tc.filename))
			require.NoError(t, err)
			require.Equal(t, string(expected), string(fileData.Body))
		})
	}
}

func TestSupportPacketYAMLFileAndJSONFile(t *testing.T) {
	t.Parallel()

	t.Run("YAMLFile returns file and accumulated error", func(t *testing.T) {
		type payload struct {
			Value string `yaml:"value"`
		}

		collectorErr := errors.New("collector failed")
		fileData, err := YAMLFile("stats.yaml", &payload{Value: "ok"}, collectorErr)
		require.NotNil(t, fileData)
		require.Error(t, err)
		require.ErrorContains(t, err, "collector failed")
		require.Equal(t, "stats.yaml", fileData.Filename)
		require.NotEmpty(t, fileData.Body)
	})

	t.Run("JSONFile returns file and accumulated error", func(t *testing.T) {
		type payload struct {
			Value string `json:"value"`
		}

		collectorErr := errors.New("collector failed")
		fileData, err := JSONFile("plugins.json", &payload{Value: "ok"}, collectorErr)
		require.NotNil(t, fileData)
		require.Error(t, err)
		require.ErrorContains(t, err, "collector failed")
		require.Equal(t, "plugins.json", fileData.Filename)
		require.NotEmpty(t, fileData.Body)
	})

	t.Run("nil value returns nil file and original error", func(t *testing.T) {
		collectorErr := errors.New("collector failed")
		type payload struct {
			Value string `yaml:"value"`
		}

		fileData, err := YAMLFile[payload]("stats.yaml", nil, collectorErr)
		require.Nil(t, fileData)
		require.ErrorIs(t, err, collectorErr)
	})

	t.Run("marshal error is appended to collector error", func(t *testing.T) {
		type badPayload struct {
			Bad chan int `json:"bad" yaml:"bad"`
		}

		collectorErr := errors.New("collector failed")
		fileData, err := JSONFile("bad.json", &badPayload{Bad: make(chan int)}, collectorErr)
		require.Nil(t, fileData)
		require.ErrorContains(t, err, "collector failed")
		require.ErrorContains(t, err, "failed to marshal bad.json into json")
	})

	t.Run("yaml marshal error is appended to collector error", func(t *testing.T) {
		type badPayload struct {
			Bad chan int `json:"bad" yaml:"bad"`
		}

		collectorErr := errors.New("collector failed")
		fileData, err := YAMLFile("bad.yaml", &badPayload{Bad: make(chan int)}, collectorErr)
		require.Nil(t, fileData)
		require.ErrorContains(t, err, "collector failed")
		require.ErrorContains(t, err, "failed to marshal bad.yaml into yaml")
	})
}
