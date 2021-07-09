// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package migrations

import (
	"os"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
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

	SystemAdminUser *model.User

	tempWorkspace string
}

func setupTestHelper(enterprise bool) *TestHelper {
	store := mainHelper.GetStore()
	store.DropAllTables()

	memoryStore := config.NewTestMemoryStore()
	newConfig := memoryStore.Get().Clone()
	*newConfig.AnnouncementSettings.AdminNoticesEnabled = false
	*newConfig.AnnouncementSettings.UserNoticesEnabled = false
	memoryStore.Set(newConfig)

	var options []app.Option
	options = append(options, app.ConfigStore(memoryStore))
	options = append(options, app.StoreOverride(mainHelper.Store))
	options = append(options, app.SkipPostInitializiation())

	s, err := app.NewServer(options...)
	if err != nil {
		panic(err)
	}
	// Adds the cache layer to the test store
	s.Store, err = localcachelayer.NewLocalCacheLayer(s.Store, s.Metrics, s.Cluster, s.CacheProvider)
	if err != nil {
		panic(err)
	}

	th := &TestHelper{
		App:     app.New(app.ServerConnector(s)),
		Context: &request.Context{},
		Server:  s,
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

	th.App.DoAppMigrations()

	th.App.Srv().Store.MarkSystemRanUnitTests()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableOpenServer = true })

	if enterprise {
		th.App.Srv().SetLicense(model.NewTestLicense())
		th.App.Srv().Jobs.InitWorkers()
		th.App.Srv().Jobs.InitSchedulers()
	} else {
		th.App.Srv().SetLicense(nil)
	}

	return th
}

func SetupEnterprise() *TestHelper {
	return setupTestHelper(true)
}

func Setup() *TestHelper {
	return setupTestHelper(false)
}

func (th *TestHelper) InitBasic() *TestHelper {
	th.SystemAdminUser = th.CreateUser()
	th.App.UpdateUserRoles(th.SystemAdminUser.ID, model.SystemUserRoleID+" "+model.SystemAdminRoleID, false)
	th.SystemAdminUser, _ = th.App.GetUser(th.SystemAdminUser.ID)

	th.BasicTeam = th.CreateTeam()
	th.BasicUser = th.CreateUser()
	th.LinkUserToTeam(th.BasicUser, th.BasicTeam)
	th.BasicUser2 = th.CreateUser()
	th.LinkUserToTeam(th.BasicUser2, th.BasicTeam)
	th.BasicChannel = th.CreateChannel(th.BasicTeam)
	th.BasicPost = th.CreatePost(th.BasicChannel)

	return th
}

func (*TestHelper) MakeEmail() string {
	return "success_" + model.NewID() + "@simulator.amazonses.com"
}

func (th *TestHelper) CreateTeam() *model.Team {
	id := model.NewID()
	team := &model.Team{
		DisplayName: "dn_" + id,
		Name:        "name" + id,
		Email:       "success+" + id + "@simulator.amazonses.com",
		Type:        model.TeamOpen,
	}

	utils.DisableDebugLogForTest()
	var err *model.AppError
	if team, err = th.App.CreateTeam(th.Context, team); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return team
}

func (th *TestHelper) CreateUser() *model.User {
	id := model.NewID()

	user := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}

	utils.DisableDebugLogForTest()
	var err *model.AppError
	if user, err = th.App.CreateUser(th.Context, user); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return user
}

func (th *TestHelper) CreateChannel(team *model.Team) *model.Channel {
	return th.createChannel(team, model.ChannelTypeOpen)
}

func (th *TestHelper) createChannel(team *model.Team, channelType string) *model.Channel {
	id := model.NewID()

	channel := &model.Channel{
		DisplayName: "dn_" + id,
		Name:        "name_" + id,
		Type:        channelType,
		TeamID:      team.ID,
		CreatorID:   th.BasicUser.ID,
	}

	utils.DisableDebugLogForTest()
	var err *model.AppError
	if channel, err = th.App.CreateChannel(th.Context, channel, true); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return channel
}

func (th *TestHelper) CreateDmChannel(user *model.User) *model.Channel {
	utils.DisableDebugLogForTest()
	var err *model.AppError
	var channel *model.Channel
	if channel, err = th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.ID, user.ID); err != nil {
		panic(err)
	}
	utils.EnableDebugLogForTest()
	return channel
}

func (th *TestHelper) CreatePost(channel *model.Channel) *model.Post {
	id := model.NewID()

	post := &model.Post{
		UserID:    th.BasicUser.ID,
		ChannelID: channel.ID,
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

func (th *TestHelper) TearDown() {
	// Clean all the caches
	th.App.Srv().InvalidateAllCaches()
	th.Server.Shutdown()
	if th.tempWorkspace != "" {
		os.RemoveAll(th.tempWorkspace)
	}
}

func (*TestHelper) ResetRoleMigration() {
	sqlStore := mainHelper.GetSQLStore()
	if _, err := sqlStore.GetMaster().Exec("DELETE from Roles"); err != nil {
		panic(err)
	}

	mainHelper.GetClusterInterface().SendClearRoleCacheMessage()

	if _, err := sqlStore.GetMaster().Exec("DELETE from Systems where Name = :Name", map[string]interface{}{"Name": model.AdvancedPermissionsMigrationKey}); err != nil {
		panic(err)
	}
}

func (th *TestHelper) DeleteAllJobsByTypeAndMigrationKey(jobType string, migrationKey string) {
	jobs, err := th.App.Srv().Store.Job().GetAllByType(model.JobTypeMigrations)
	if err != nil {
		panic(err)
	}

	for _, job := range jobs {
		if key, ok := job.Data[JobDataKeyMigration]; ok && key == migrationKey {
			if _, err = th.App.Srv().Store.Job().Delete(job.ID); err != nil {
				panic(err)
			}
		}
	}
}
