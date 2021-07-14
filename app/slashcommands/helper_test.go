// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/localcachelayer"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type TestHelper struct {
	App          *app.App
	Context      *request.Context
	Server       *app.Server
	BasicTeam    *model.Team
	BasicUser    *model.User
	BasicUser2   *model.User
	BasicChannel *model.Channel
	BasicPost    *model.Post

	SystemAdminUser   *model.User
	LogBuffer         *bytes.Buffer
	IncludeCacheLayer bool

	tempWorkspace string
}

func setupTestHelper(dbStore store.Store, enterprise bool, includeCacheLayer bool, tb testing.TB, configSet func(*model.Config)) *TestHelper {
	tempWorkspace, err := ioutil.TempDir("", "apptest")
	if err != nil {
		panic(err)
	}

	memoryStore := config.NewTestMemoryStore()

	config := memoryStore.Get()
	if configSet != nil {
		configSet(config)
	}
	*config.PluginSettings.Directory = filepath.Join(tempWorkspace, "plugins")
	*config.PluginSettings.ClientDirectory = filepath.Join(tempWorkspace, "webapp")
	*config.PluginSettings.AutomaticPrepackagedPlugins = false
	*config.LogSettings.EnableSentry = false // disable error reporting during tests
	memoryStore.Set(config)

	buffer := &bytes.Buffer{}

	var options []app.Option
	options = append(options, app.ConfigStore(memoryStore))
	if includeCacheLayer {
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
	options = append(options, app.SetLogger(mlog.NewTestingLogger(tb, buffer)))

	s, err := app.NewServer(options...)
	if err != nil {
		panic(err)
	}

	th := &TestHelper{
		App:               app.New(app.ServerConnector(s)),
		Context:           &request.Context{},
		Server:            s,
		LogBuffer:         buffer,
		IncludeCacheLayer: includeCacheLayer,
	}

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.MaxUsersPerTeam = 50 })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.RateLimitSettings.Enable = false })
	prevListenAddress := *th.App.Config().ServiceSettings.ListenAddress
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.ListenAddress = ":0" })
	serverErr := th.Server.Start()
	if serverErr != nil {
		panic(serverErr)
	}

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.ListenAddress = prevListenAddress })

	th.App.Srv().SearchEngine = mainHelper.SearchEngine

	th.App.Srv().Store.MarkSystemRanUnitTests()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableOpenServer = true })

	// Disable strict password requirements for test
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PasswordSettings.MinimumLength = 5
		*cfg.PasswordSettings.Lowercase = false
		*cfg.PasswordSettings.Uppercase = false
		*cfg.PasswordSettings.Symbol = false
		*cfg.PasswordSettings.Number = false
	})

	if enterprise {
		th.App.Srv().SetLicense(model.NewTestLicense())
		th.App.Srv().Jobs.InitWorkers()
		th.App.Srv().Jobs.InitSchedulers()
	} else {
		th.App.Srv().SetLicense(nil)
	}

	if th.tempWorkspace == "" {
		th.tempWorkspace = tempWorkspace
	}

	return th
}

func setup(tb testing.TB) *TestHelper {
	if testing.Short() {
		tb.SkipNow()
	}
	dbStore := mainHelper.GetStore()
	dbStore.DropAllTables()
	dbStore.MarkSystemRanUnitTests()

	return setupTestHelper(dbStore, false, true, tb, nil)
}

var initBasicOnce sync.Once
var userCache struct {
	SystemAdminUser *model.User
	BasicUser       *model.User
	BasicUser2      *model.User
}

func (th *TestHelper) initBasic() *TestHelper {
	// create users once and cache them because password hashing is slow
	initBasicOnce.Do(func() {
		th.SystemAdminUser = th.createUser()
		th.App.UpdateUserRoles(th.SystemAdminUser.Id, model.SYSTEM_USER_ROLE_ID+" "+model.SYSTEM_ADMIN_ROLE_ID, false)
		th.SystemAdminUser, _ = th.App.GetUser(th.SystemAdminUser.Id)
		userCache.SystemAdminUser = th.SystemAdminUser.DeepCopy()

		th.BasicUser = th.createUser()
		th.BasicUser, _ = th.App.GetUser(th.BasicUser.Id)
		userCache.BasicUser = th.BasicUser.DeepCopy()

		th.BasicUser2 = th.createUser()
		th.BasicUser2, _ = th.App.GetUser(th.BasicUser2.Id)
		userCache.BasicUser2 = th.BasicUser2.DeepCopy()
	})
	// restore cached users
	th.SystemAdminUser = userCache.SystemAdminUser.DeepCopy()
	th.BasicUser = userCache.BasicUser.DeepCopy()
	th.BasicUser2 = userCache.BasicUser2.DeepCopy()
	mainHelper.GetSQLStore().GetMaster().Insert(th.SystemAdminUser, th.BasicUser, th.BasicUser2)

	th.BasicTeam = th.createTeam()

	th.linkUserToTeam(th.BasicUser, th.BasicTeam)
	th.linkUserToTeam(th.BasicUser2, th.BasicTeam)
	th.BasicChannel = th.CreateChannel(th.BasicTeam)
	th.BasicPost = th.createPost(th.BasicChannel)
	return th
}

func (th *TestHelper) createTeam() *model.Team {
	id := model.NewId()
	team := &model.Team{
		DisplayName: "dn_" + id,
		Name:        "name" + id,
		Email:       "success+" + id + "@simulator.amazonses.com",
		Type:        model.TEAM_OPEN,
	}

	utils.DisableDebugLogForTest()
	var err *model.AppError
	if team, err = th.App.CreateTeam(th.Context, team); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return team
}

func (th *TestHelper) createUser() *model.User {
	return th.createUserOrGuest(false)
}

func (th *TestHelper) createGuest() *model.User {
	return th.createUserOrGuest(true)
}

func (th *TestHelper) createUserOrGuest(guest bool) *model.User {
	id := model.NewId()

	user := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}

	utils.DisableDebugLogForTest()
	var err *model.AppError
	if guest {
		if user, err = th.App.CreateGuest(th.Context, user); err != nil {
			panic(err)
		}
	} else {
		if user, err = th.App.CreateUser(th.Context, user); err != nil {
			panic(err)
		}
	}
	utils.EnableDebugLogForTest()
	return user
}

type ChannelOption func(*model.Channel)

func WithShared(v bool) ChannelOption {
	return func(channel *model.Channel) {
		channel.Shared = model.NewBool(v)
	}
}

func (th *TestHelper) CreateChannel(team *model.Team, options ...ChannelOption) *model.Channel {
	return th.createChannel(team, model.CHANNEL_OPEN, options...)
}

func (th *TestHelper) createPrivateChannel(team *model.Team) *model.Channel {
	return th.createChannel(team, model.CHANNEL_PRIVATE)
}

func (th *TestHelper) createChannel(team *model.Team, channelType string, options ...ChannelOption) *model.Channel {
	id := model.NewId()

	channel := &model.Channel{
		DisplayName: "dn_" + id,
		Name:        "name_" + id,
		Type:        channelType,
		TeamId:      team.Id,
		CreatorId:   th.BasicUser.Id,
	}

	for _, option := range options {
		option(channel)
	}

	utils.DisableDebugLogForTest()
	var err *model.AppError
	if channel, err = th.App.CreateChannel(th.Context, channel, true); err != nil {
		panic(err)
	}

	if channel.IsShared() {
		id := model.NewId()
		_, err := th.App.SaveSharedChannel(&model.SharedChannel{
			ChannelId:        channel.Id,
			TeamId:           channel.TeamId,
			Home:             false,
			ReadOnly:         false,
			ShareName:        "shared-" + id,
			ShareDisplayName: "shared-" + id,
			CreatorId:        th.BasicUser.Id,
			RemoteId:         model.NewId(),
		})
		if err != nil {
			panic(err)
		}
	}
	utils.EnableDebugLogForTest()
	return channel
}

func (th *TestHelper) createChannelWithAnotherUser(team *model.Team, channelType, userID string) *model.Channel {
	id := model.NewId()

	channel := &model.Channel{
		DisplayName: "dn_" + id,
		Name:        "name_" + id,
		Type:        channelType,
		TeamId:      team.Id,
		CreatorId:   userID,
	}

	utils.DisableDebugLogForTest()
	var err *model.AppError
	if channel, err = th.App.CreateChannel(th.Context, channel, true); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return channel
}

func (th *TestHelper) createDmChannel(user *model.User) *model.Channel {
	utils.DisableDebugLogForTest()
	var err *model.AppError
	var channel *model.Channel
	if channel, err = th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, user.Id); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return channel
}

func (th *TestHelper) createGroupChannel(user1 *model.User, user2 *model.User) *model.Channel {
	utils.DisableDebugLogForTest()
	var err *model.AppError
	var channel *model.Channel
	if channel, err = th.App.CreateGroupChannel([]string{th.BasicUser.Id, user1.Id, user2.Id}, th.BasicUser.Id); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return channel
}

func (th *TestHelper) createPost(channel *model.Channel) *model.Post {
	id := model.NewId()

	post := &model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: channel.Id,
		Message:   "message_" + id,
		CreateAt:  model.GetMillis() - 10000,
	}

	utils.DisableDebugLogForTest()
	var err *model.AppError
	if post, err = th.App.CreatePost(th.Context, post, channel, false, true); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return post
}

func (th *TestHelper) linkUserToTeam(user *model.User, team *model.Team) {
	utils.DisableDebugLogForTest()

	_, err := th.App.JoinUserToTeam(th.Context, team, user, "")
	if err != nil {
		panic(err)
	}

	utils.EnableDebugLogForTest()
}

func (th *TestHelper) addUserToChannel(user *model.User, channel *model.Channel) *model.ChannelMember {
	utils.DisableDebugLogForTest()

	member, err := th.App.AddUserToChannel(user, channel, false)
	if err != nil {
		panic(err)
	}

	utils.EnableDebugLogForTest()

	return member
}

func (th *TestHelper) shutdownApp() {
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

func (th *TestHelper) tearDown() {
	if th.IncludeCacheLayer {
		// Clean all the caches
		th.App.Srv().InvalidateAllCaches()
	}
	th.shutdownApp()
	if th.tempWorkspace != "" {
		os.RemoveAll(th.tempWorkspace)
	}
}

func (th *TestHelper) removePermissionFromRole(permission string, roleName string) {
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

func (th *TestHelper) addPermissionToRole(permission string, roleName string) {
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
