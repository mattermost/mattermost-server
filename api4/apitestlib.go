// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	s3 "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/localcachelayer"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/testlib"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/web"
	"github.com/mattermost/mattermost-server/v5/wsapi"
)

type TestHelper struct {
	App         *app.App
	Server      *app.Server
	ConfigStore *config.Store

	Context              *request.Context
	Client               *model.Client4
	BasicUser            *model.User
	BasicUser2           *model.User
	TeamAdminUser        *model.User
	BasicTeam            *model.Team
	BasicChannel         *model.Channel
	BasicPrivateChannel  *model.Channel
	BasicPrivateChannel2 *model.Channel
	BasicDeletedChannel  *model.Channel
	BasicChannel2        *model.Channel
	BasicPost            *model.Post
	Group                *model.Group

	SystemAdminClient *model.Client4
	SystemAdminUser   *model.User
	tempWorkspace     string

	SystemManagerClient *model.Client4
	SystemManagerUser   *model.User

	LocalClient *model.Client4

	IncludeCacheLayer bool
}

var mainHelper *testlib.MainHelper

func SetMainHelper(mh *testlib.MainHelper) {
	mainHelper = mh
}

func setupTestHelper(dbStore store.Store, searchEngine *searchengine.Broker, enterprise bool, includeCache bool,
	updateConfig func(*model.Config), options []app.Option) *TestHelper {
	tempWorkspace, err := ioutil.TempDir("", "apptest")
	if err != nil {
		panic(err)
	}

	memoryStore, err := config.NewMemoryStoreWithOptions(&config.MemoryStoreOptions{IgnoreEnvironmentOverrides: true})
	if err != nil {
		panic("failed to initialize memory store: " + err.Error())
	}

	memoryConfig := &model.Config{}
	memoryConfig.SetDefaults()
	*memoryConfig.PluginSettings.Directory = filepath.Join(tempWorkspace, "plugins")
	*memoryConfig.PluginSettings.ClientDirectory = filepath.Join(tempWorkspace, "webapp")
	memoryConfig.ServiceSettings.EnableLocalMode = model.NewBool(true)
	*memoryConfig.ServiceSettings.LocalModeSocketLocation = filepath.Join(tempWorkspace, "mattermost_local.sock")
	*memoryConfig.AnnouncementSettings.AdminNoticesEnabled = false
	*memoryConfig.AnnouncementSettings.UserNoticesEnabled = false
	*memoryConfig.PluginSettings.AutomaticPrepackagedPlugins = false
	if updateConfig != nil {
		updateConfig(memoryConfig)
	}
	memoryStore.Set(memoryConfig)

	configStore, err := config.NewStoreFromBacking(memoryStore, nil, false)
	if err != nil {
		panic(err)
	}

	options = append(options, app.ConfigStore(configStore))
	if includeCache {
		// Adds the cache layer to the test store
		options = append(options, app.StoreOverride(func(s *app.Server) store.Store {
			lcl, err2 := localcachelayer.NewLocalCacheLayer(dbStore, s.Metrics, s.Cluster, s.CacheProvider)
			if err2 != nil {
				panic(err2)
			}
			return lcl
		}))
	} else {
		options = append(options, app.StoreOverride(dbStore))
	}

	s, err := app.NewServer(options...)
	if err != nil {
		panic(err)
	}

	th := &TestHelper{
		App:               app.New(app.ServerConnector(s)),
		Server:            s,
		ConfigStore:       configStore,
		IncludeCacheLayer: includeCache,
		Context:           &request.Context{},
	}

	if s.SearchEngine != nil && s.SearchEngine.BleveEngine != nil && searchEngine != nil {
		searchEngine.BleveEngine = s.SearchEngine.BleveEngine
	}

	if searchEngine != nil {
		th.App.SetSearchEngine(searchEngine)
	}

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.MaxUsersPerTeam = 50
		*cfg.RateLimitSettings.Enable = false
		*cfg.EmailSettings.SendEmailNotifications = true
		*cfg.ServiceSettings.SiteURL = ""

		// Disable sniffing, otherwise elastic client fails to connect to docker node
		// More details: https://github.com/olivere/elastic/wiki/Sniffing
		*cfg.ElasticsearchSettings.Sniff = false

		*cfg.TeamSettings.EnableOpenServer = true

		// Disable strict password requirements for test
		*cfg.PasswordSettings.MinimumLength = 5
		*cfg.PasswordSettings.Lowercase = false
		*cfg.PasswordSettings.Uppercase = false
		*cfg.PasswordSettings.Symbol = false
		*cfg.PasswordSettings.Number = false

		*cfg.ServiceSettings.ListenAddress = ":0"
	})
	if err := th.Server.Start(); err != nil {
		panic(err)
	}

	Init(th.App, th.App.Srv().Router)
	InitLocal(th.App, th.App.Srv().LocalRouter)
	web.New(th.App, th.App.Srv().Router)
	wsapi.Init(th.App.Srv())

	if enterprise {
		th.App.Srv().SetLicense(model.NewTestLicense())
		th.App.Srv().Jobs.InitWorkers()
		th.App.Srv().Jobs.InitSchedulers()
	} else {
		th.App.Srv().SetLicense(nil)
	}

	th.Client = th.CreateClient()
	th.SystemAdminClient = th.CreateClient()
	th.SystemManagerClient = th.CreateClient()

	// Verify handling of the supported true/false values by randomizing on each run.
	rand.Seed(time.Now().UTC().UnixNano())
	trueValues := []string{"1", "t", "T", "TRUE", "true", "True"}
	falseValues := []string{"0", "f", "F", "FALSE", "false", "False"}
	trueString := trueValues[rand.Intn(len(trueValues))]
	falseString := falseValues[rand.Intn(len(falseValues))]
	mlog.Debug("Configured Client4 bool string values", mlog.String("true", trueString), mlog.String("false", falseString))
	th.Client.SetBoolString(true, trueString)
	th.Client.SetBoolString(false, falseString)

	th.LocalClient = th.CreateLocalClient(*memoryConfig.ServiceSettings.LocalModeSocketLocation)

	if th.tempWorkspace == "" {
		th.tempWorkspace = tempWorkspace
	}

	return th
}

func SetupEnterprise(tb testing.TB) *TestHelper {
	if testing.Short() {
		tb.SkipNow()
	}

	if mainHelper == nil {
		tb.SkipNow()
	}

	dbStore := mainHelper.GetStore()
	dbStore.DropAllTables()
	dbStore.MarkSystemRanUnitTests()
	mainHelper.PreloadMigrations()
	searchEngine := mainHelper.GetSearchEngine()
	th := setupTestHelper(dbStore, searchEngine, true, true, nil, nil)
	th.InitLogin()
	return th
}

func Setup(tb testing.TB) *TestHelper {
	if testing.Short() {
		tb.SkipNow()
	}

	if mainHelper == nil {
		tb.SkipNow()
	}

	dbStore := mainHelper.GetStore()
	dbStore.DropAllTables()
	dbStore.MarkSystemRanUnitTests()
	mainHelper.PreloadMigrations()
	searchEngine := mainHelper.GetSearchEngine()
	th := setupTestHelper(dbStore, searchEngine, false, true, nil, nil)
	th.InitLogin()
	return th
}

func SetupConfig(tb testing.TB, updateConfig func(cfg *model.Config)) *TestHelper {
	if testing.Short() {
		tb.SkipNow()
	}

	if mainHelper == nil {
		tb.SkipNow()
	}

	dbStore := mainHelper.GetStore()
	dbStore.DropAllTables()
	dbStore.MarkSystemRanUnitTests()
	searchEngine := mainHelper.GetSearchEngine()
	th := setupTestHelper(dbStore, searchEngine, false, true, updateConfig, nil)
	th.InitLogin()
	return th
}

func SetupConfigWithStoreMock(tb testing.TB, updateConfig func(cfg *model.Config)) *TestHelper {
	th := setupTestHelper(testlib.GetMockStoreForSetupFunctions(), nil, false, false, updateConfig, nil)
	statusMock := mocks.StatusStore{}
	statusMock.On("UpdateExpiredDNDStatuses").Return([]*model.Status{}, nil)
	statusMock.On("Get", "user1").Return(&model.Status{UserId: "user1", Status: model.STATUS_ONLINE}, nil)
	statusMock.On("UpdateLastActivityAt", "user1", mock.Anything).Return(nil)
	statusMock.On("SaveOrUpdate", mock.AnythingOfType("*model.Status")).Return(nil)
	emptyMockStore := mocks.Store{}
	emptyMockStore.On("Close").Return(nil)
	emptyMockStore.On("Status").Return(&statusMock)
	th.App.Srv().Store = &emptyMockStore
	return th
}

func SetupWithStoreMock(tb testing.TB) *TestHelper {
	th := setupTestHelper(testlib.GetMockStoreForSetupFunctions(), nil, false, false, nil, nil)
	statusMock := mocks.StatusStore{}
	statusMock.On("UpdateExpiredDNDStatuses").Return([]*model.Status{}, nil)
	statusMock.On("Get", "user1").Return(&model.Status{UserId: "user1", Status: model.STATUS_ONLINE}, nil)
	statusMock.On("UpdateLastActivityAt", "user1", mock.Anything).Return(nil)
	statusMock.On("SaveOrUpdate", mock.AnythingOfType("*model.Status")).Return(nil)
	emptyMockStore := mocks.Store{}
	emptyMockStore.On("Close").Return(nil)
	emptyMockStore.On("Status").Return(&statusMock)
	th.App.Srv().Store = &emptyMockStore
	return th
}

func SetupEnterpriseWithStoreMock(tb testing.TB) *TestHelper {
	th := setupTestHelper(testlib.GetMockStoreForSetupFunctions(), nil, true, false, nil, nil)
	statusMock := mocks.StatusStore{}
	statusMock.On("UpdateExpiredDNDStatuses").Return([]*model.Status{}, nil)
	statusMock.On("Get", "user1").Return(&model.Status{UserId: "user1", Status: model.STATUS_ONLINE}, nil)
	statusMock.On("UpdateLastActivityAt", "user1", mock.Anything).Return(nil)
	statusMock.On("SaveOrUpdate", mock.AnythingOfType("*model.Status")).Return(nil)
	emptyMockStore := mocks.Store{}
	emptyMockStore.On("Close").Return(nil)
	emptyMockStore.On("Status").Return(&statusMock)
	th.App.Srv().Store = &emptyMockStore
	return th
}

func SetupWithServerOptions(tb testing.TB, options []app.Option) *TestHelper {
	if testing.Short() {
		tb.SkipNow()
	}

	if mainHelper == nil {
		tb.SkipNow()
	}

	dbStore := mainHelper.GetStore()
	dbStore.DropAllTables()
	dbStore.MarkSystemRanUnitTests()
	mainHelper.PreloadMigrations()
	searchEngine := mainHelper.GetSearchEngine()
	th := setupTestHelper(dbStore, searchEngine, false, true, nil, options)
	th.InitLogin()
	return th
}

func (th *TestHelper) ShutdownApp() {
	done := make(chan bool)
	go func() {
		th.Server.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		// panic instead of fatal to terminate all tests in this package, otherwise the
		// still running App could spuriously fail subsequent tests.
		panic("failed to shutdown App within 30 seconds")
	}
}

func (th *TestHelper) TearDown() {
	utils.DisableDebugLogForTest()
	if th.IncludeCacheLayer {
		// Clean all the caches
		th.App.Srv().InvalidateAllCaches()
	}

	th.ShutdownApp()

	utils.EnableDebugLogForTest()
}

var initBasicOnce sync.Once
var userCache struct {
	SystemAdminUser   *model.User
	SystemManagerUser *model.User
	TeamAdminUser     *model.User
	BasicUser         *model.User
	BasicUser2        *model.User
}

func (th *TestHelper) InitLogin() *TestHelper {
	th.waitForConnectivity()

	// create users once and cache them because password hashing is slow
	initBasicOnce.Do(func() {
		th.SystemAdminUser = th.CreateUser()
		th.App.UpdateUserRoles(th.SystemAdminUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_ADMIN_ROLE_ID, false)
		th.SystemAdminUser, _ = th.App.GetUser(th.SystemAdminUser.Id)
		userCache.SystemAdminUser = th.SystemAdminUser.DeepCopy()

		th.SystemManagerUser = th.CreateUser()
		th.App.UpdateUserRoles(th.SystemManagerUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_MANAGER_ROLE_ID, false)
		th.SystemManagerUser, _ = th.App.GetUser(th.SystemManagerUser.Id)
		userCache.SystemManagerUser = th.SystemManagerUser.DeepCopy()

		th.TeamAdminUser = th.CreateUser()
		th.App.UpdateUserRoles(th.TeamAdminUser.Id, model.SYSTEM_USER_ROLE_ID, false)
		th.TeamAdminUser, _ = th.App.GetUser(th.TeamAdminUser.Id)
		userCache.TeamAdminUser = th.TeamAdminUser.DeepCopy()

		th.BasicUser = th.CreateUser()
		th.BasicUser, _ = th.App.GetUser(th.BasicUser.Id)
		userCache.BasicUser = th.BasicUser.DeepCopy()

		th.BasicUser2 = th.CreateUser()
		th.BasicUser2, _ = th.App.GetUser(th.BasicUser2.Id)
		userCache.BasicUser2 = th.BasicUser2.DeepCopy()
	})
	// restore cached users
	th.SystemAdminUser = userCache.SystemAdminUser.DeepCopy()
	th.SystemManagerUser = userCache.SystemManagerUser.DeepCopy()
	th.TeamAdminUser = userCache.TeamAdminUser.DeepCopy()
	th.BasicUser = userCache.BasicUser.DeepCopy()
	th.BasicUser2 = userCache.BasicUser2.DeepCopy()
	mainHelper.GetSQLStore().GetMaster().Insert(th.SystemAdminUser, th.TeamAdminUser, th.BasicUser, th.BasicUser2, th.SystemManagerUser)
	// restore non hashed password for login
	th.SystemAdminUser.Password = "Pa$$word11"
	th.TeamAdminUser.Password = "Pa$$word11"
	th.BasicUser.Password = "Pa$$word11"
	th.BasicUser2.Password = "Pa$$word11"
	th.SystemManagerUser.Password = "Pa$$word11"

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		th.LoginSystemAdmin()
		wg.Done()
	}()
	go func() {
		th.LoginSystemManager()
		wg.Done()
	}()
	go func() {
		th.LoginTeamAdmin()
		wg.Done()
	}()
	wg.Wait()
	return th
}

func (th *TestHelper) InitBasic() *TestHelper {
	th.BasicTeam = th.CreateTeam()
	th.BasicChannel = th.CreatePublicChannel()
	th.BasicPrivateChannel = th.CreatePrivateChannel()
	th.BasicPrivateChannel2 = th.CreatePrivateChannel()
	th.BasicDeletedChannel = th.CreatePublicChannel()
	th.BasicChannel2 = th.CreatePublicChannel()
	th.BasicPost = th.CreatePost()
	th.LinkUserToTeam(th.BasicUser, th.BasicTeam)
	th.LinkUserToTeam(th.BasicUser2, th.BasicTeam)
	th.App.AddUserToChannel(th.BasicUser, th.BasicChannel, false)
	th.App.AddUserToChannel(th.BasicUser2, th.BasicChannel, false)
	th.App.AddUserToChannel(th.BasicUser, th.BasicChannel2, false)
	th.App.AddUserToChannel(th.BasicUser2, th.BasicChannel2, false)
	th.App.AddUserToChannel(th.BasicUser, th.BasicPrivateChannel, false)
	th.App.AddUserToChannel(th.BasicUser2, th.BasicPrivateChannel, false)
	th.App.AddUserToChannel(th.BasicUser, th.BasicDeletedChannel, false)
	th.App.AddUserToChannel(th.BasicUser2, th.BasicDeletedChannel, false)
	th.App.UpdateUserRoles(th.BasicUser.Id, model.SYSTEM_USER_ROLE_ID, false)
	th.Client.DeleteChannel(th.BasicDeletedChannel.Id)
	th.LoginBasic()
	th.Group = th.CreateGroup()

	return th
}

func (th *TestHelper) waitForConnectivity() {
	for i := 0; i < 1000; i++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%v", th.App.Srv().ListenAddr.Port))
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(time.Millisecond * 20)
	}
	panic("unable to connect")
}

func (th *TestHelper) CreateClient() *model.Client4 {
	return model.NewAPIv4Client(fmt.Sprintf("http://localhost:%v", th.App.Srv().ListenAddr.Port))
}

// ToDo: maybe move this to NewAPIv4SocketClient and reuse it in mmctl
func (th *TestHelper) CreateLocalClient(socketPath string) *model.Client4 {
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	return &model.Client4{
		ApiUrl:     "http://_" + model.API_URL_SUFFIX,
		HttpClient: httpClient,
	}
}

func (th *TestHelper) CreateWebSocketClient() (*model.WebSocketClient, *model.AppError) {
	return model.NewWebSocketClient4(fmt.Sprintf("ws://localhost:%v", th.App.Srv().ListenAddr.Port), th.Client.AuthToken)
}

func (th *TestHelper) CreateWebSocketSystemAdminClient() (*model.WebSocketClient, *model.AppError) {
	return model.NewWebSocketClient4(fmt.Sprintf("ws://localhost:%v", th.App.Srv().ListenAddr.Port), th.SystemAdminClient.AuthToken)
}

func (th *TestHelper) CreateWebSocketSystemManagerClient() (*model.WebSocketClient, *model.AppError) {
	return model.NewWebSocketClient4(fmt.Sprintf("ws://localhost:%v", th.App.Srv().ListenAddr.Port), th.SystemManagerClient.AuthToken)
}

func (th *TestHelper) CreateWebSocketClientWithClient(client *model.Client4) (*model.WebSocketClient, *model.AppError) {
	return model.NewWebSocketClient4(fmt.Sprintf("ws://localhost:%v", th.App.Srv().ListenAddr.Port), client.AuthToken)
}

func (th *TestHelper) CreateBotWithSystemAdminClient() *model.Bot {
	return th.CreateBotWithClient((th.SystemAdminClient))
}

func (th *TestHelper) CreateBotWithClient(client *model.Client4) *model.Bot {
	bot := &model.Bot{
		Username:    GenerateTestUsername(),
		DisplayName: "a bot",
		Description: "bot",
	}

	utils.DisableDebugLogForTest()
	rbot, resp := client.CreateBot(bot)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
	return rbot
}

func (th *TestHelper) CreateUser() *model.User {
	return th.CreateUserWithClient(th.Client)
}

func (th *TestHelper) CreateTeam() *model.Team {
	return th.CreateTeamWithClient(th.Client)
}

func (th *TestHelper) CreateTeamWithClient(client *model.Client4) *model.Team {
	id := model.NewId()
	team := &model.Team{
		DisplayName: "dn_" + id,
		Name:        GenerateTestTeamName(),
		Email:       th.GenerateTestEmail(),
		Type:        model.TEAM_OPEN,
	}

	utils.DisableDebugLogForTest()
	rteam, resp := client.CreateTeam(team)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
	return rteam
}

func (th *TestHelper) CreateUserWithClient(client *model.Client4) *model.User {
	id := model.NewId()

	user := &model.User{
		Email:     th.GenerateTestEmail(),
		Username:  GenerateTestUsername(),
		Nickname:  "nn_" + id,
		FirstName: "f_" + id,
		LastName:  "l_" + id,
		Password:  "Pa$$word11",
	}

	utils.DisableDebugLogForTest()
	ruser, response := client.CreateUser(user)
	if response.Error != nil {
		panic(response.Error)
	}

	ruser.Password = "Pa$$word11"
	_, err := th.App.Srv().Store.User().VerifyEmail(ruser.Id, ruser.Email)
	if err != nil {
		return nil
	}
	utils.EnableDebugLogForTest()
	return ruser
}

func (th *TestHelper) CreateUserWithAuth(authService string) *model.User {
	id := model.NewId()
	user := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		EmailVerified: true,
		AuthService:   authService,
	}
	user, err := th.App.CreateUser(th.Context, user)
	if err != nil {
		panic(err)
	}
	return user
}

func (th *TestHelper) SetupLdapConfig() {
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableMultifactorAuthentication = true
		*cfg.LdapSettings.Enable = true
		*cfg.LdapSettings.EnableSync = true
		*cfg.LdapSettings.LdapServer = "dockerhost"
		*cfg.LdapSettings.BaseDN = "dc=mm,dc=test,dc=com"
		*cfg.LdapSettings.BindUsername = "cn=admin,dc=mm,dc=test,dc=com"
		*cfg.LdapSettings.BindPassword = "mostest"
		*cfg.LdapSettings.FirstNameAttribute = "cn"
		*cfg.LdapSettings.LastNameAttribute = "sn"
		*cfg.LdapSettings.NicknameAttribute = "cn"
		*cfg.LdapSettings.EmailAttribute = "mail"
		*cfg.LdapSettings.UsernameAttribute = "uid"
		*cfg.LdapSettings.IdAttribute = "cn"
		*cfg.LdapSettings.LoginIdAttribute = "uid"
		*cfg.LdapSettings.SkipCertificateVerification = true
		*cfg.LdapSettings.GroupFilter = ""
		*cfg.LdapSettings.GroupDisplayNameAttribute = "cN"
		*cfg.LdapSettings.GroupIdAttribute = "entRyUuId"
		*cfg.LdapSettings.MaxPageSize = 0
	})
	th.App.Srv().SetLicense(model.NewTestLicense("ldap"))
}

func (th *TestHelper) SetupSamlConfig() {
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.SamlSettings.Enable = true
		*cfg.SamlSettings.Verify = false
		*cfg.SamlSettings.Encrypt = false
		*cfg.SamlSettings.IdpUrl = "https://does.notmatter.com"
		*cfg.SamlSettings.IdpDescriptorUrl = "https://localhost/adfs/services/trust"
		*cfg.SamlSettings.AssertionConsumerServiceURL = "https://localhost/login/sso/saml"
		*cfg.SamlSettings.ServiceProviderIdentifier = "https://localhost/login/sso/saml"
		*cfg.SamlSettings.IdpCertificateFile = app.SamlIdpCertificateName
		*cfg.SamlSettings.PrivateKeyFile = app.SamlPrivateKeyName
		*cfg.SamlSettings.PublicCertificateFile = app.SamlPublicCertificateName
		*cfg.SamlSettings.EmailAttribute = "Email"
		*cfg.SamlSettings.UsernameAttribute = "Username"
		*cfg.SamlSettings.FirstNameAttribute = "FirstName"
		*cfg.SamlSettings.LastNameAttribute = "LastName"
		*cfg.SamlSettings.NicknameAttribute = ""
		*cfg.SamlSettings.PositionAttribute = ""
		*cfg.SamlSettings.LocaleAttribute = ""
		*cfg.SamlSettings.SignatureAlgorithm = model.SAML_SETTINGS_SIGNATURE_ALGORITHM_SHA256
		*cfg.SamlSettings.CanonicalAlgorithm = model.SAML_SETTINGS_CANONICAL_ALGORITHM_C14N11
	})
	th.App.Srv().SetLicense(model.NewTestLicense("saml"))
}

func (th *TestHelper) CreatePublicChannel() *model.Channel {
	return th.CreateChannelWithClient(th.Client, model.CHANNEL_OPEN)
}

func (th *TestHelper) CreatePrivateChannel() *model.Channel {
	return th.CreateChannelWithClient(th.Client, model.CHANNEL_PRIVATE)
}

func (th *TestHelper) CreateChannelWithClient(client *model.Client4, channelType string) *model.Channel {
	return th.CreateChannelWithClientAndTeam(client, channelType, th.BasicTeam.Id)
}

func (th *TestHelper) CreateChannelWithClientAndTeam(client *model.Client4, channelType string, teamId string) *model.Channel {
	id := model.NewId()

	channel := &model.Channel{
		DisplayName: "dn_" + id,
		Name:        GenerateTestChannelName(),
		Type:        channelType,
		TeamId:      teamId,
	}

	utils.DisableDebugLogForTest()
	rchannel, resp := client.CreateChannel(channel)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
	return rchannel
}

func (th *TestHelper) CreatePost() *model.Post {
	return th.CreatePostWithClient(th.Client, th.BasicChannel)
}

func (th *TestHelper) CreatePinnedPost() *model.Post {
	return th.CreatePinnedPostWithClient(th.Client, th.BasicChannel)
}

func (th *TestHelper) CreateMessagePost(message string) *model.Post {
	return th.CreateMessagePostWithClient(th.Client, th.BasicChannel, message)
}

func (th *TestHelper) CreatePostWithClient(client *model.Client4, channel *model.Channel) *model.Post {
	id := model.NewId()

	post := &model.Post{
		ChannelId: channel.Id,
		Message:   "message_" + id,
	}

	utils.DisableDebugLogForTest()
	rpost, resp := client.CreatePost(post)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
	return rpost
}

func (th *TestHelper) CreatePinnedPostWithClient(client *model.Client4, channel *model.Channel) *model.Post {
	id := model.NewId()

	post := &model.Post{
		ChannelId: channel.Id,
		Message:   "message_" + id,
		IsPinned:  true,
	}

	utils.DisableDebugLogForTest()
	rpost, resp := client.CreatePost(post)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
	return rpost
}

func (th *TestHelper) CreateMessagePostWithClient(client *model.Client4, channel *model.Channel, message string) *model.Post {
	post := &model.Post{
		ChannelId: channel.Id,
		Message:   message,
	}

	utils.DisableDebugLogForTest()
	rpost, resp := client.CreatePost(post)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
	return rpost
}

func (th *TestHelper) CreateMessagePostNoClient(channel *model.Channel, message string, createAtTime int64) *model.Post {
	post, err := th.App.Srv().Store.Post().Save(&model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: channel.Id,
		Message:   message,
		CreateAt:  createAtTime,
	})

	if err != nil {
		panic(err)
	}

	return post
}

func (th *TestHelper) CreateDmChannel(user *model.User) *model.Channel {
	utils.DisableDebugLogForTest()
	var err *model.AppError
	var channel *model.Channel
	if channel, err = th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, user.Id); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return channel
}

func (th *TestHelper) LoginBasic() {
	th.LoginBasicWithClient(th.Client)
}

func (th *TestHelper) LoginBasic2() {
	th.LoginBasic2WithClient(th.Client)
}

func (th *TestHelper) LoginTeamAdmin() {
	th.LoginTeamAdminWithClient(th.Client)
}

func (th *TestHelper) LoginSystemAdmin() {
	th.LoginSystemAdminWithClient(th.SystemAdminClient)
}

func (th *TestHelper) LoginSystemManager() {
	th.LoginSystemManagerWithClient(th.SystemManagerClient)
}

func (th *TestHelper) LoginBasicWithClient(client *model.Client4) {
	utils.DisableDebugLogForTest()
	_, resp := client.Login(th.BasicUser.Email, th.BasicUser.Password)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
}

func (th *TestHelper) LoginBasic2WithClient(client *model.Client4) {
	utils.DisableDebugLogForTest()
	_, resp := client.Login(th.BasicUser2.Email, th.BasicUser2.Password)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
}

func (th *TestHelper) LoginTeamAdminWithClient(client *model.Client4) {
	utils.DisableDebugLogForTest()
	_, resp := client.Login(th.TeamAdminUser.Email, th.TeamAdminUser.Password)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
}

func (th *TestHelper) LoginSystemManagerWithClient(client *model.Client4) {
	utils.DisableDebugLogForTest()
	_, resp := client.Login(th.SystemManagerUser.Email, th.SystemManagerUser.Password)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
}

func (th *TestHelper) LoginSystemAdminWithClient(client *model.Client4) {
	utils.DisableDebugLogForTest()
	_, resp := client.Login(th.SystemAdminUser.Email, th.SystemAdminUser.Password)
	if resp.Error != nil {
		panic(resp.Error)
	}
	utils.EnableDebugLogForTest()
}

func (th *TestHelper) UpdateActiveUser(user *model.User, active bool) {
	utils.DisableDebugLogForTest()

	_, err := th.App.UpdateActive(th.Context, user, active)
	if err != nil {
		panic(err)
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) LinkUserToTeam(user *model.User, team *model.Team) {
	utils.DisableDebugLogForTest()

	_, err := th.App.JoinUserToTeam(th.Context, team, user, "")
	if err != nil {
		panic(err)
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) AddUserToChannel(user *model.User, channel *model.Channel) *model.ChannelMember {
	utils.DisableDebugLogForTest()

	member, err := th.App.AddUserToChannel(user, channel, false)
	if err != nil {
		panic(err)
	}

	utils.EnableDebugLogForTest()

	return member
}

func (th *TestHelper) GenerateTestEmail() string {
	if *th.App.Config().EmailSettings.SMTPServer != "localhost" && os.Getenv("CI_INBUCKET_PORT") == "" {
		return strings.ToLower("success+" + model.NewId() + "@simulator.amazonses.com")
	}
	return strings.ToLower(model.NewId() + "@localhost")
}

func (th *TestHelper) CreateGroup() *model.Group {
	id := model.NewId()
	group := &model.Group{
		Name:        model.NewString("n-" + id),
		DisplayName: "dn_" + id,
		Source:      model.GroupSourceLdap,
		RemoteId:    "ri_" + id,
	}

	utils.DisableDebugLogForTest()
	group, err := th.App.CreateGroup(group)
	if err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return group
}

// TestForSystemAdminAndLocal runs a test function for both
// SystemAdmin and Local clients. Several endpoints work in the same
// way when used by a fully privileged user and through the local
// mode, so this helper facilitates checking both
func (th *TestHelper) TestForSystemAdminAndLocal(t *testing.T, f func(*testing.T, *model.Client4), name ...string) {
	var testName string
	if len(name) > 0 {
		testName = name[0] + "/"
	}

	t.Run(testName+"SystemAdminClient", func(t *testing.T) {
		f(t, th.SystemAdminClient)
	})

	t.Run(testName+"LocalClient", func(t *testing.T) {
		f(t, th.LocalClient)
	})
}

// TestForAllClients runs a test function for all the clients
// registered in the TestHelper
func (th *TestHelper) TestForAllClients(t *testing.T, f func(*testing.T, *model.Client4), name ...string) {
	var testName string
	if len(name) > 0 {
		testName = name[0] + "/"
	}

	t.Run(testName+"Client", func(t *testing.T) {
		f(t, th.Client)
	})

	t.Run(testName+"SystemAdminClient", func(t *testing.T) {
		f(t, th.SystemAdminClient)
	})

	t.Run(testName+"LocalClient", func(t *testing.T) {
		f(t, th.LocalClient)
	})
}

func GenerateTestUsername() string {
	return "fakeuser" + model.NewRandomString(10)
}

func GenerateTestTeamName() string {
	return "faketeam" + model.NewRandomString(6)
}

func GenerateTestChannelName() string {
	return "fakechannel" + model.NewRandomString(10)
}

func GenerateTestAppName() string {
	return "fakeoauthapp" + model.NewRandomString(10)
}

func GenerateTestId() string {
	return model.NewId()
}

func CheckUserSanitization(t *testing.T, user *model.User) {
	t.Helper()

	require.Equal(t, "", user.Password, "password wasn't blank")
	require.Empty(t, user.AuthData, "auth data wasn't blank")
	require.Equal(t, "", user.MfaSecret, "mfa secret wasn't blank")
}

func CheckEtag(t *testing.T, data interface{}, resp *model.Response) {
	t.Helper()

	require.Empty(t, data)
	require.Equal(t, resp.StatusCode, http.StatusNotModified, "wrong status code for etag")
}

func CheckNoError(t *testing.T, resp *model.Response) {
	t.Helper()

	require.Nil(t, resp.Error, "expected no error")
}

func checkHTTPStatus(t *testing.T, resp *model.Response, expectedStatus int, expectError bool) {
	t.Helper()

	require.NotNilf(t, resp, "Unexpected nil response, expected http:%v, expectError:%v", expectedStatus, expectError)
	if expectError {
		require.NotNil(t, resp.Error, "Expected a non-nil error and http status:%v, got nil, %v", expectedStatus, resp.StatusCode)
	} else {
		require.Nil(t, resp.Error, "Expected no error and http status:%v, got %q, http:%v", expectedStatus, resp.Error, resp.StatusCode)
	}
	require.Equalf(t, expectedStatus, resp.StatusCode, "Expected http status:%v, got %v (err: %q)", expectedStatus, resp.StatusCode, resp.Error)
}

func CheckOKStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusOK, false)
}

func CheckCreatedStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusCreated, false)
}

func CheckForbiddenStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusForbidden, true)
}

func CheckUnauthorizedStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusUnauthorized, true)
}

func CheckNotFoundStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusNotFound, true)
}

func CheckBadRequestStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusBadRequest, true)
}

func CheckNotImplementedStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusNotImplemented, true)
}

func CheckRequestEntityTooLargeStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusRequestEntityTooLarge, true)
}

func CheckInternalErrorStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusInternalServerError, true)
}

func CheckServiceUnavailableStatus(t *testing.T, resp *model.Response) {
	t.Helper()
	checkHTTPStatus(t, resp, http.StatusServiceUnavailable, true)
}

func CheckErrorMessage(t *testing.T, resp *model.Response, errorId string) {
	t.Helper()

	require.NotNilf(t, resp.Error, "should have errored with message: %s", errorId)
	require.Equalf(t, errorId, resp.Error.Id, "incorrect error message, actual: %s, expected: %s", resp.Error.Id, errorId)
}

func CheckStartsWith(t *testing.T, value, prefix, message string) {
	require.True(t, strings.HasPrefix(value, prefix), message, value)
}

// Similar to s3.New() but allows initialization of signature v2 or signature v4 client.
// If signV2 input is false, function always returns signature v4.
//
// Additionally this function also takes a user defined region, if set
// disables automatic region lookup.
func s3New(endpoint, accessKey, secretKey string, secure bool, signV2 bool, region string) (*s3.Client, error) {
	var creds *credentials.Credentials
	if signV2 {
		creds = credentials.NewStatic(accessKey, secretKey, "", credentials.SignatureV2)
	} else {
		creds = credentials.NewStatic(accessKey, secretKey, "", credentials.SignatureV4)
	}

	opts := s3.Options{
		Creds:  creds,
		Secure: secure,
		Region: region,
	}
	return s3.New(endpoint, &opts)
}

func (th *TestHelper) cleanupTestFile(info *model.FileInfo) error {
	cfg := th.App.Config()
	if *cfg.FileSettings.DriverName == model.IMAGE_DRIVER_S3 {
		endpoint := *cfg.FileSettings.AmazonS3Endpoint
		accessKey := *cfg.FileSettings.AmazonS3AccessKeyId
		secretKey := *cfg.FileSettings.AmazonS3SecretAccessKey
		secure := *cfg.FileSettings.AmazonS3SSL
		signV2 := *cfg.FileSettings.AmazonS3SignV2
		region := *cfg.FileSettings.AmazonS3Region
		s3Clnt, err := s3New(endpoint, accessKey, secretKey, secure, signV2, region)
		if err != nil {
			return err
		}
		bucket := *cfg.FileSettings.AmazonS3Bucket
		if err := s3Clnt.RemoveObject(context.Background(), bucket, info.Path, s3.RemoveObjectOptions{}); err != nil {
			return err
		}

		if info.ThumbnailPath != "" {
			if err := s3Clnt.RemoveObject(context.Background(), bucket, info.ThumbnailPath, s3.RemoveObjectOptions{}); err != nil {
				return err
			}
		}

		if info.PreviewPath != "" {
			if err := s3Clnt.RemoveObject(context.Background(), bucket, info.PreviewPath, s3.RemoveObjectOptions{}); err != nil {
				return err
			}
		}
	} else if *cfg.FileSettings.DriverName == model.IMAGE_DRIVER_LOCAL {
		if err := os.Remove(*cfg.FileSettings.Directory + info.Path); err != nil {
			return err
		}

		if info.ThumbnailPath != "" {
			if err := os.Remove(*cfg.FileSettings.Directory + info.ThumbnailPath); err != nil {
				return err
			}
		}

		if info.PreviewPath != "" {
			if err := os.Remove(*cfg.FileSettings.Directory + info.PreviewPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (th *TestHelper) MakeUserChannelAdmin(user *model.User, channel *model.Channel) {
	utils.DisableDebugLogForTest()

	if cm, err := th.App.GetChannelMember(context.Background(), channel.Id, user.Id); err == nil {
		if _, err = th.App.UpdateChannelMemberSchemeRoles(channel.Id, user.Id, cm.SchemeGuest, cm.SchemeUser, true); err != nil {
			utils.EnableDebugLogForTest()
			panic(err)
		}
	} else {
		utils.EnableDebugLogForTest()
		panic(err)
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) UpdateUserToTeamAdmin(user *model.User, team *model.Team) {
	utils.DisableDebugLogForTest()

	if tm, err := th.App.Srv().Store.Team().GetMember(context.Background(), team.Id, user.Id); err == nil {
		tm.SchemeAdmin = true
		if _, err = th.App.Srv().Store.Team().UpdateMember(tm); err != nil {
			utils.EnableDebugLogForTest()
			panic(err)
		}
	} else {
		utils.EnableDebugLogForTest()
		panic(err)
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) UpdateUserToNonTeamAdmin(user *model.User, team *model.Team) {
	utils.DisableDebugLogForTest()

	if tm, err := th.App.Srv().Store.Team().GetMember(context.Background(), team.Id, user.Id); err == nil {
		tm.SchemeAdmin = false
		if _, err = th.App.Srv().Store.Team().UpdateMember(tm); err != nil {
			utils.EnableDebugLogForTest()
			panic(err)
		}
	} else {
		utils.EnableDebugLogForTest()
		panic(err)
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) SaveDefaultRolePermissions() map[string][]string {
	utils.DisableDebugLogForTest()

	results := make(map[string][]string)

	for _, roleName := range []string{
		"system_user",
		"system_admin",
		"team_user",
		"team_admin",
		"channel_user",
		"channel_admin",
	} {
		role, err1 := th.App.GetRoleByName(context.Background(), roleName)
		if err1 != nil {
			utils.EnableDebugLogForTest()
			panic(err1)
		}

		results[roleName] = role.Permissions
	}

	utils.EnableDebugLogForTest()
	return results
}

func (th *TestHelper) RestoreDefaultRolePermissions(data map[string][]string) {
	utils.DisableDebugLogForTest()

	for roleName, permissions := range data {
		role, err1 := th.App.GetRoleByName(context.Background(), roleName)
		if err1 != nil {
			utils.EnableDebugLogForTest()
			panic(err1)
		}

		if strings.Join(role.Permissions, " ") == strings.Join(permissions, " ") {
			continue
		}

		role.Permissions = permissions

		_, err2 := th.App.UpdateRole(role)
		if err2 != nil {
			utils.EnableDebugLogForTest()
			panic(err2)
		}
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) RemovePermissionFromRole(permission string, roleName string) {
	utils.DisableDebugLogForTest()

	role, err1 := th.App.GetRoleByName(context.Background(), roleName)
	if err1 != nil {
		utils.EnableDebugLogForTest()
		panic(err1)
	}

	var newPermissions []string
	for _, p := range role.Permissions {
		if p != permission {
			newPermissions = append(newPermissions, p)
		}
	}

	if strings.Join(role.Permissions, " ") == strings.Join(newPermissions, " ") {
		utils.EnableDebugLogForTest()
		return
	}

	role.Permissions = newPermissions

	_, err2 := th.App.UpdateRole(role)
	if err2 != nil {
		utils.EnableDebugLogForTest()
		panic(err2)
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) AddPermissionToRole(permission string, roleName string) {
	utils.DisableDebugLogForTest()

	role, err1 := th.App.GetRoleByName(context.Background(), roleName)
	if err1 != nil {
		utils.EnableDebugLogForTest()
		panic(err1)
	}

	for _, existingPermission := range role.Permissions {
		if existingPermission == permission {
			utils.EnableDebugLogForTest()
			return
		}
	}

	role.Permissions = append(role.Permissions, permission)

	_, err2 := th.App.UpdateRole(role)
	if err2 != nil {
		utils.EnableDebugLogForTest()
		panic(err2)
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) SetupTeamScheme() *model.Scheme {
	return th.SetupScheme(model.SCHEME_SCOPE_TEAM)
}

func (th *TestHelper) SetupChannelScheme() *model.Scheme {
	return th.SetupScheme(model.SCHEME_SCOPE_CHANNEL)
}

func (th *TestHelper) SetupScheme(scope string) *model.Scheme {
	scheme, err := th.App.CreateScheme(&model.Scheme{
		Name:        model.NewId(),
		DisplayName: model.NewId(),
		Scope:       scope,
	})
	if err != nil {
		panic(err)
	}
	return scheme
}
