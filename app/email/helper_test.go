// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package email

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/services/users"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/shared/templates"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/testlib"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type TestHelper struct {
	service     *Service
	configStore *config.Store
	store       store.Store
	workspace   string

	BasicTeam    *model.Team
	BasicChannel *model.Channel
	BasicUser    *model.User
	BasicUser2   *model.User

	SystemAdminUser *model.User
	LogBuffer       *bytes.Buffer
}

func Setup(tb testing.TB) *TestHelper {
	if testing.Short() {
		tb.SkipNow()
	}
	dbStore := mainHelper.GetStore()
	dbStore.DropAllTables()
	dbStore.MarkSystemRanUnitTests()
	mainHelper.PreloadMigrations()

	return setupTestHelper(dbStore, tb)
}

func SetupWithStoreMock(tb testing.TB) *TestHelper {
	mockStore := testlib.GetMockStoreForSetupFunctions()
	th := setupTestHelper(mockStore, tb)
	statusMock := mocks.StatusStore{}
	statusMock.On("UpdateExpiredDNDStatuses").Return([]*model.Status{}, nil)
	statusMock.On("Get", "user1").Return(&model.Status{UserId: "user1", Status: model.STATUS_ONLINE}, nil)
	statusMock.On("UpdateLastActivityAt", "user1", mock.Anything).Return(nil)
	statusMock.On("SaveOrUpdate", mock.AnythingOfType("*model.Status")).Return(nil)
	emptyMockStore := mocks.Store{}
	emptyMockStore.On("Close").Return(nil)
	emptyMockStore.On("Status").Return(&statusMock)
	th.service.store = &emptyMockStore
	return th
}

func setupTestHelper(s store.Store, tb testing.TB) *TestHelper {
	tempWorkspace, err := ioutil.TempDir("", "userservicetest")
	if err != nil {
		panic(err)
	}

	configStore := config.NewTestMemoryStore()

	config := configStore.Get()
	*config.PluginSettings.Directory = filepath.Join(tempWorkspace, "plugins")
	*config.PluginSettings.ClientDirectory = filepath.Join(tempWorkspace, "webapp")
	*config.PluginSettings.AutomaticPrepackagedPlugins = false
	*config.LogSettings.EnableSentry = false // disable error reporting during tests
	*config.AnnouncementSettings.AdminNoticesEnabled = false
	*config.AnnouncementSettings.UserNoticesEnabled = false
	*config.TeamSettings.MaxUsersPerTeam = 50
	*config.RateLimitSettings.Enable = false
	*config.TeamSettings.EnableOpenServer = true
	// Disable strict password requirements for test
	*config.PasswordSettings.MinimumLength = 5
	*config.PasswordSettings.Lowercase = false
	*config.PasswordSettings.Uppercase = false
	*config.PasswordSettings.Symbol = false
	*config.PasswordSettings.Number = false
	configStore.Set(config)

	licenseFn := func() *model.License { return model.NewTestLicense() }

	us, err := users.New(users.ServiceConfig{
		UserStore:    s.User(),
		SessionStore: s.Session(),
		OAuthStore:   s.OAuth(),
		ConfigFn:     configStore.Get,
		LicenseFn:    licenseFn,
	})
	if err != nil {
		panic(err)
	}

	templatesDir, ok := templates.GetTemplateDirectory()
	if !ok {
		panic("failed find server templates")
	}
	htmlTemplateWatcher, errorsChan, err := templates.NewWithWatcher(templatesDir)
	if err != nil {
		panic(err)
	}

	go func() {
		for err2 := range errorsChan {
			mlog.Error("Server templates error", mlog.Err(err2))
		}
	}()

	service := &Service{
		store:              s,
		userService:        us,
		license:            licenseFn,
		config:             configStore.Get,
		templatesContainer: htmlTemplateWatcher,
		goFn:               func(f func()) { go f() },
	}

	if err := service.setUpRateLimiters(); err != nil {
		panic(err)
	}

	return &TestHelper{
		service:     service,
		configStore: configStore,
		store:       s,
		LogBuffer:   &bytes.Buffer{},
		workspace:   tempWorkspace,
	}
}

func (th *TestHelper) InitBasic() *TestHelper {
	th.BasicTeam = th.CreateTeam()

	th.SystemAdminUser = th.CreateUser()
	th.SystemAdminUser, _ = th.service.userService.GetUser(th.SystemAdminUser.Id)
	th.addUserToTeam(th.BasicTeam, th.SystemAdminUser)

	th.BasicUser = th.CreateUser()
	th.BasicUser, _ = th.service.userService.GetUser(th.BasicUser.Id)
	th.addUserToTeam(th.BasicTeam, th.BasicUser)

	th.BasicUser2 = th.CreateUser()
	th.BasicUser2, _ = th.service.userService.GetUser(th.BasicUser2.Id)
	th.addUserToTeam(th.BasicTeam, th.BasicUser2)

	th.BasicChannel = th.createChannel(th.BasicTeam, model.CHANNEL_OPEN)
	th.addUserToChannel(th.BasicChannel, th.SystemAdminUser)
	th.addUserToChannel(th.BasicChannel, th.BasicUser)
	th.addUserToChannel(th.BasicChannel, th.BasicUser2)

	return th
}

func (th *TestHelper) CreateTeam() *model.Team {
	id := model.NewId()
	team := &model.Team{
		DisplayName: "dn_" + id,
		Name:        "name" + id,
		Email:       "success+" + id + "@simulator.amazonses.com",
		Type:        model.TEAM_OPEN,
	}

	utils.DisableDebugLogForTest()
	var err error
	if team, err = th.store.Team().Save(team); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return team
}

func (th *TestHelper) createChannel(team *model.Team, channelType string) *model.Channel {
	id := model.NewId()

	channel := &model.Channel{
		DisplayName: "dn_" + id,
		Name:        "name_" + id,
		Type:        channelType,
		TeamId:      team.Id,
		CreatorId:   th.BasicUser.Id,
	}

	utils.DisableDebugLogForTest()
	var err error
	if channel, err = th.store.Channel().Save(channel, *th.configStore.Get().TeamSettings.MaxChannelsPerTeam); err != nil {
		panic(err)
	}

	utils.EnableDebugLogForTest()
	return channel
}

func (th *TestHelper) addUserToChannel(channel *model.Channel, user *model.User) *model.ChannelMember {
	newMember := &model.ChannelMember{
		ChannelId:   channel.Id,
		UserId:      user.Id,
		NotifyProps: model.GetDefaultChannelNotifyProps(),
		SchemeGuest: user.IsGuest(),
		SchemeUser:  !user.IsGuest(),
	}

	var err error
	newMember, err = th.store.Channel().SaveMember(newMember)
	if err != nil {
		panic(err)
	}

	return newMember
}

func (th *TestHelper) addUserToTeam(team *model.Team, user *model.User) *model.TeamMember {
	tm := &model.TeamMember{
		TeamId:      team.Id,
		UserId:      user.Id,
		SchemeGuest: user.IsGuest(),
		SchemeUser:  !user.IsGuest(),
	}

	var err error
	tm, err = th.store.Team().SaveMember(tm, *th.service.config().TeamSettings.MaxUsersPerTeam)
	if err != nil {
		panic(err)
	}

	return tm
}

func (th *TestHelper) CreateUser() *model.User {
	return th.CreateUserOrGuest(false)
}

func (th *TestHelper) CreateGuest() *model.User {
	return th.CreateUserOrGuest(true)
}

func (th *TestHelper) CreateUserOrGuest(guest bool) *model.User {
	id := model.NewId()

	user := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}

	var err error
	if guest {
		if user, err = th.service.userService.CreateUser(user, users.UserCreateOptions{Guest: true}); err != nil {
			panic(err)
		}
	} else {
		if user, err = th.service.userService.CreateUser(user, users.UserCreateOptions{}); err != nil {
			panic(err)
		}
	}
	return user
}

func (th *TestHelper) TearDown() {
	th.configStore.Close()

	th.store.Close()

	if th.workspace != "" {
		os.RemoveAll(th.workspace)
	}
}

func (th *TestHelper) UpdateConfig(f func(*model.Config)) {
	if th.configStore.IsReadOnly() {
		return
	}
	old := th.configStore.Get()
	updated := old.Clone()
	f(updated)
	if _, _, err := th.configStore.Set(updated); err != nil {
		panic(err)
	}
}

func (th *TestHelper) ConfigureInbucketMail() {
	inbucket_host := os.Getenv("CI_INBUCKET_HOST")
	if inbucket_host == "" {
		inbucket_host = "localhost"
	}
	inbucket_port := os.Getenv("CI_INBUCKET_SMTP_PORT")
	if inbucket_port == "" {
		inbucket_port = "10025"
	}
	th.UpdateConfig(func(cfg *model.Config) {
		*cfg.EmailSettings.SMTPServer = inbucket_host
		*cfg.EmailSettings.SMTPPort = inbucket_port
	})
}
