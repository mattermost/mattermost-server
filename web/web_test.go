// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store/localcachelayer"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/utils"
)

var APIClient *model.Client4
var URL string

type TestHelper struct {
	App     app.AppIface
	Context *request.Context
	Server  *app.Server
	Web     *Web

	BasicUser    *model.User
	BasicChannel *model.Channel
	BasicTeam    *model.Team

	SystemAdminUser *model.User

	tempWorkspace string

	IncludeCacheLayer bool
}

func SetupWithStoreMock(tb testing.TB) *TestHelper {
	if testing.Short() {
		tb.SkipNow()
	}

	th := setupTestHelper(false)
	emptyMockStore := mocks.Store{}
	emptyMockStore.On("Close").Return(nil)
	th.App.Srv().Store = &emptyMockStore
	return th
}

func Setup(tb testing.TB) *TestHelper {
	if testing.Short() {
		tb.SkipNow()
	}
	store := mainHelper.GetStore()
	store.DropAllTables()
	return setupTestHelper(true)
}

func setupTestHelper(includeCacheLayer bool) *TestHelper {
	memoryStore := config.NewTestMemoryStore()
	newConfig := memoryStore.Get().Clone()
	*newConfig.AnnouncementSettings.AdminNoticesEnabled = false
	*newConfig.AnnouncementSettings.UserNoticesEnabled = false
	*newConfig.PluginSettings.AutomaticPrepackagedPlugins = false
	memoryStore.Set(newConfig)
	var options []app.Option
	options = append(options, app.ConfigStore(memoryStore))
	options = append(options, app.StoreOverride(mainHelper.Store))

	mlog.DisableZap()

	s, err := app.NewServer(options...)
	if err != nil {
		panic(err)
	}
	if includeCacheLayer {
		// Adds the cache layer to the test store
		s.Store, err = localcachelayer.NewLocalCacheLayer(s.Store, s.Metrics, s.Cluster, s.CacheProvider)
		if err != nil {
			panic(err)
		}
	}

	prevListenAddress := *s.Config().ServiceSettings.ListenAddress
	s.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.ListenAddress = ":0" })
	serverErr := s.Start()
	if serverErr != nil {
		panic(serverErr)
	}
	s.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.ListenAddress = prevListenAddress })

	// Disable strict password requirements for test
	s.UpdateConfig(func(cfg *model.Config) {
		*cfg.PasswordSettings.MinimumLength = 5
		*cfg.PasswordSettings.Lowercase = false
		*cfg.PasswordSettings.Uppercase = false
		*cfg.PasswordSettings.Symbol = false
		*cfg.PasswordSettings.Number = false
	})

	ctx := &request.Context{}
	a := app.New(app.ServerConnector(s))

	web := New(a, s.Router)
	URL = fmt.Sprintf("http://localhost:%v", s.ListenAddr.Port)
	APIClient = model.NewAPIv4Client(URL)

	s.Store.MarkSystemRanUnitTests()

	s.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.EnableOpenServer = true
	})

	th := &TestHelper{
		App:               a,
		Context:           ctx,
		Server:            s,
		Web:               web,
		IncludeCacheLayer: includeCacheLayer,
	}

	return th
}

func (th *TestHelper) InitPlugins() *TestHelper {
	pluginDir := filepath.Join(th.tempWorkspace, "plugins")
	webappDir := filepath.Join(th.tempWorkspace, "webapp")

	th.App.InitPlugins(th.Context, pluginDir, webappDir)

	return th
}

func (th *TestHelper) NewPluginAPI(manifest *model.Manifest) plugin.API {
	return th.App.NewPluginAPI(th.Context, manifest)
}

func (th *TestHelper) InitBasic() *TestHelper {
	th.SystemAdminUser, _ = th.App.CreateUser(th.Context, &model.User{Email: model.NewID() + "success+test@simulator.amazonses.com", Nickname: "Corey Hulen", Password: "passwd1", EmailVerified: true, Roles: model.SystemAdminRoleID})

	user, _ := th.App.CreateUser(th.Context, &model.User{Email: model.NewID() + "success+test@simulator.amazonses.com", Nickname: "Corey Hulen", Password: "passwd1", EmailVerified: true, Roles: model.SystemUserRoleID})

	team, _ := th.App.CreateTeam(th.Context, &model.Team{DisplayName: "Name", Name: "z-z-" + model.NewID() + "a", Email: user.Email, Type: model.TeamOpen})

	th.App.JoinUserToTeam(th.Context, team, user, "")

	channel, _ := th.App.CreateChannel(th.Context, &model.Channel{DisplayName: "Test API Name", Name: "zz" + model.NewID() + "a", Type: model.ChannelTypeOpen, TeamID: team.ID, CreatorID: user.ID}, true)

	th.BasicUser = user
	th.BasicChannel = channel
	th.BasicTeam = team

	return th
}

func (th *TestHelper) TearDown() {
	if th.IncludeCacheLayer {
		// Clean all the caches
		th.App.Srv().InvalidateAllCaches()
	}
	th.Server.Shutdown()
}

func TestStaticFilesRequest(t *testing.T) {
	th := Setup(t).InitPlugins()
	defer th.TearDown()

	pluginID := "com.mattermost.sample"

	// Setup the directory directly in the plugin working path.
	pluginDir := filepath.Join(*th.App.Config().PluginSettings.Directory, pluginID)
	err := os.MkdirAll(pluginDir, 0777)
	require.NoError(t, err)
	pluginDir, err = filepath.Abs(pluginDir)
	require.NoError(t, err)

	// Compile the backend
	backend := filepath.Join(pluginDir, "backend.exe")
	pluginCode := `
	package main

	import (
		"github.com/mattermost/mattermost-server/v5/plugin"
	)

	type MyPlugin struct {
		plugin.MattermostPlugin
	}

	func main() {
		plugin.ClientMain(&MyPlugin{})
	}
`
	utils.CompileGo(t, pluginCode, backend)

	// Write out the frontend
	mainJS := `var x = alert();`
	mainJSPath := filepath.Join(pluginDir, "main.js")
	require.NoError(t, err)
	err = ioutil.WriteFile(mainJSPath, []byte(mainJS), 0777)
	require.NoError(t, err)

	// Write the plugin.json manifest
	pluginManifest := `{"id": "com.mattermost.sample", "server": {"executable": "backend.exe"}, "webapp": {"bundle_path":"main.js"}, "settings_schema": {"settings": []}}`
	ioutil.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginManifest), 0600)

	// Activate the plugin
	manifest, activated, reterr := th.App.GetPluginsEnvironment().Activate(pluginID)
	require.NoError(t, reterr)
	require.NotNil(t, manifest)
	require.True(t, activated)

	// Verify access to the bundle with requisite headers
	req, _ := http.NewRequest("GET", "/static/plugins/com.mattermost.sample/com.mattermost.sample_724ed0e2ebb2b841_bundle.js", nil)
	res := httptest.NewRecorder()
	th.Web.MainRouter.ServeHTTP(res, req)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, mainJS, res.Body.String())
	assert.Equal(t, []string{"max-age=31556926, public"}, res.Result().Header[http.CanonicalHeaderKey("Cache-Control")])

	// Verify cached access to the bundle with an If-Modified-Since timestamp in the future
	future := time.Now().Add(24 * time.Hour)
	req, _ = http.NewRequest("GET", "/static/plugins/com.mattermost.sample/com.mattermost.sample_724ed0e2ebb2b841_bundle.js", nil)
	req.Header.Add("If-Modified-Since", future.Format(time.RFC850))
	res = httptest.NewRecorder()
	th.Web.MainRouter.ServeHTTP(res, req)
	assert.Equal(t, http.StatusNotModified, res.Code)
	assert.Empty(t, res.Body.String())
	assert.Equal(t, []string{"max-age=31556926, public"}, res.Result().Header[http.CanonicalHeaderKey("Cache-Control")])

	// Verify access to the bundle with an If-Modified-Since timestamp in the past
	past := time.Now().Add(-24 * time.Hour)
	req, _ = http.NewRequest("GET", "/static/plugins/com.mattermost.sample/com.mattermost.sample_724ed0e2ebb2b841_bundle.js", nil)
	req.Header.Add("If-Modified-Since", past.Format(time.RFC850))
	res = httptest.NewRecorder()
	th.Web.MainRouter.ServeHTTP(res, req)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, mainJS, res.Body.String())
	assert.Equal(t, []string{"max-age=31556926, public"}, res.Result().Header[http.CanonicalHeaderKey("Cache-Control")])

	// Verify handling of 404.
	req, _ = http.NewRequest("GET", "/static/plugins/com.mattermost.sample/404.js", nil)
	res = httptest.NewRecorder()
	th.Web.MainRouter.ServeHTTP(res, req)
	assert.Equal(t, http.StatusNotFound, res.Code)
	assert.Equal(t, "404 page not found\n", res.Body.String())
	assert.Equal(t, []string{"no-cache, public"}, res.Result().Header[http.CanonicalHeaderKey("Cache-Control")])
}

func TestPublicFilesRequest(t *testing.T) {
	th := Setup(t).InitPlugins()
	defer th.TearDown()

	pluginDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	webappPluginDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(pluginDir)
	defer os.RemoveAll(webappPluginDir)

	env, err := plugin.NewEnvironment(th.NewPluginAPI, app.NewDriverImpl(th.Server), pluginDir, webappPluginDir, th.App.Log(), nil)
	require.NoError(t, err)

	pluginID := "com.mattermost.sample"
	pluginCode :=
		`
	package main

	import (
		"github.com/mattermost/mattermost-server/v5/plugin"
	)

	type MyPlugin struct {
		plugin.MattermostPlugin
	}

	func main() {
		plugin.ClientMain(&MyPlugin{})
	}

	`
	// Compile and write the plugin
	backend := filepath.Join(pluginDir, pluginID, "backend.exe")
	utils.CompileGo(t, pluginCode, backend)

	// Write the plugin.json manifest
	pluginManifest := `{"id": "com.mattermost.sample", "server": {"executable": "backend.exe"}, "settings_schema": {"settings": []}}`
	ioutil.WriteFile(filepath.Join(pluginDir, pluginID, "plugin.json"), []byte(pluginManifest), 0600)

	// Write the test public file
	helloHTML := `Hello from the static files public folder for the com.mattermost.sample plugin!`
	htmlFolderPath := filepath.Join(pluginDir, pluginID, "public")
	os.MkdirAll(htmlFolderPath, os.ModePerm)
	htmlFilePath := filepath.Join(htmlFolderPath, "hello.html")

	htmlFileErr := ioutil.WriteFile(htmlFilePath, []byte(helloHTML), 0600)
	assert.NoError(t, htmlFileErr)

	nefariousHTML := `You shouldn't be able to get here!`
	htmlFileErr = ioutil.WriteFile(filepath.Join(pluginDir, pluginID, "nefarious-file-access.html"), []byte(nefariousHTML), 0600)
	assert.NoError(t, htmlFileErr)

	manifest, activated, reterr := env.Activate(pluginID)
	require.NoError(t, reterr)
	require.NotNil(t, manifest)
	require.True(t, activated)

	th.App.SetPluginsEnvironment(env)

	req, _ := http.NewRequest("GET", "/plugins/com.mattermost.sample/public/hello.html", nil)
	res := httptest.NewRecorder()
	th.Web.MainRouter.ServeHTTP(res, req)
	assert.Equal(t, helloHTML, res.Body.String())

	req, _ = http.NewRequest("GET", "/plugins/com.mattermost.sample/nefarious-file-access.html", nil)
	res = httptest.NewRecorder()
	th.Web.MainRouter.ServeHTTP(res, req)
	assert.Equal(t, 404, res.Code)

	req, _ = http.NewRequest("GET", "/plugins/com.mattermost.sample/public/../nefarious-file-access.html", nil)
	res = httptest.NewRecorder()
	th.Web.MainRouter.ServeHTTP(res, req)
	assert.Equal(t, 301, res.Code)
}

/* Test disabled for now so we don't requrie the client to build. Maybe re-enable after client gets moved out.
func TestStatic(t *testing.T) {
	Setup()

	// add a short delay to make sure the server is ready to receive requests
	time.Sleep(1 * time.Second)

	resp, err := http.Get(URL + "/static/root.html")

	assert.NoErrorf(t, err, "got error while trying to get static files %v", err)
	assert.Equalf(t, resp.StatusCode, http.StatusOK, "couldn't get static files %v", resp.StatusCode)
}
*/

func TestCheckClientCompatability(t *testing.T) {
	//Browser Name, UA String, expected result (if the browser should fail the test false and if it should pass the true)
	type uaTest struct {
		Name      string // Name of Browser
		UserAgent string // Useragent of Browser
		Result    bool   // Expected result (true if browser should be compatible, false if browser shouldn't be compatible)
	}
	var uaTestParameters = []uaTest{
		{"Mozilla 40.1", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.1", true},
		{"Chrome 60", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.90 Safari/537.36", true},
		{"Chrome Mobile", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.113 Mobile Safari/537.36", true},
		{"MM Classic App", "Mozilla/5.0 (Linux; Android 8.0.0; Nexus 5X Build/OPR6.170623.013; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/61.0.3163.81 Mobile Safari/537.36 Web-Atoms-Mobile-WebView", true},
		{"MM App 3.7.1", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Mattermost/3.7.1 Chrome/56.0.2924.87 Electron/1.6.11 Safari/537.36", true},
		{"Franz 4.0.4", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Franz/4.0.4 Chrome/52.0.2743.82 Electron/1.3.1 Safari/537.36", true},
		{"Edge 14", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.79 Safari/537.36 Edge/14.14393", true},
		{"Internet Explorer 9", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 7.1; Trident/5.0", false},
		{"Internet Explorer 11", "Mozilla/5.0 (Windows NT 10.0; WOW64; Trident/7.0; rv:11.0) like Gecko", false},
		{"Internet Explorer 11 2", "Mozilla/5.0 (Windows NT 10.0; WOW64; Trident/7.0; .NET4.0C; .NET4.0E; .NET CLR 2.0.50727; .NET CLR 3.0.30729; .NET CLR 3.5.30729; Zoom 3.6.0; rv:11.0) like Gecko", false},
		{"Internet Explorer 11 (Compatibility Mode) 1", "Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 10.0; WOW64; Trident/7.0; .NET4.0C; .NET4.0E; .NET CLR 2.0.50727; .NET CLR 3.0.30729; .NET CLR 3.5.30729; .NET CLR 1.1.4322; InfoPath.3; Zoom 3.6.0)", false},
		{"Internet Explorer 11 (Compatibility Mode) 2", "Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 10.0; WOW64; Trident/7.0; .NET4.0C; .NET4.0E; .NET CLR 2.0.50727; .NET CLR 3.0.30729; .NET CLR 3.5.30729; Zoom 3.6.0)", false},
		{"Safari 12", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/12.0 Safari/605.1.15", true},
		{"Safari 11", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Safari/604.1.38", false},
		{"Safari 10", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_3) AppleWebKit/602.4.8 (KHTML, like Gecko) Version/10.0.3 Safari/602.4.8", false},
		{"Safari 9", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_3) AppleWebKit/601.4.4 (KHTML, like Gecko) Version/9.0.3 Safari/601.4.4", false},
		{"Safari 8", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_4) AppleWebKit/600.7.12 (KHTML, like Gecko) Version/8.0.7 Safari/600.7.12", false},
		{"Safari Mobile 12", "Mozilla/5.0 (iPhone; CPU iPhone OS 12_0 like macOS) AppleWebKit/602.1.50 (KHTML, like Gecko) Version/12.0 Mobile/14A5335d Safari/602.1.50", true},
		{"Safari Mobile 9", "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B137 Safari/601.1", false},
	}
	for _, browser := range uaTestParameters {
		t.Run(browser.Name, func(t *testing.T) {
			result := CheckClientCompatibility(browser.UserAgent)
			require.Equalf(t, result, browser.Result, "user agent test failed for %s", browser.Name)
		})
	}
}
