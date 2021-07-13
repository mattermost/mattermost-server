// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/api4"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/testlib"
)

var coverprofileCounters map[string]int = make(map[string]int)

var mainHelper *testlib.MainHelper

type testHelper struct {
	*api4.TestHelper

	config            *model.Config
	tempDir           string
	configFilePath    string
	disableAutoConfig bool
}

// Setup creates an instance of testHelper.
func Setup(t testing.TB) *testHelper {
	dir, err := testlib.SetupTestResources()
	if err != nil {
		panic("failed to create temporary directory: " + err.Error())
	}

	api4TestHelper := api4.Setup(t)

	testHelper := &testHelper{
		TestHelper:     api4TestHelper,
		tempDir:        dir,
		configFilePath: filepath.Join(dir, "config-helper.json"),
	}

	config := &model.Config{}
	config.SetDefaults()
	testHelper.SetConfig(config)

	return testHelper
}

// Setup creates an instance of testHelper.
func SetupWithStoreMock(t testing.TB) *testHelper {
	dir, err := testlib.SetupTestResources()
	if err != nil {
		panic("failed to create temporary directory: " + err.Error())
	}

	api4TestHelper := api4.SetupWithStoreMock(t)
	systemStore := mocks.SystemStore{}
	systemStore.On("Get").Return(make(model.StringMap), nil)
	licenseStore := mocks.LicenseStore{}
	licenseStore.On("Get", "").Return(&model.LicenseRecord{}, nil)
	api4TestHelper.App.Srv().Store.(*mocks.Store).On("System").Return(&systemStore)
	api4TestHelper.App.Srv().Store.(*mocks.Store).On("License").Return(&licenseStore)

	testHelper := &testHelper{
		TestHelper:     api4TestHelper,
		tempDir:        dir,
		configFilePath: filepath.Join(dir, "config-helper.json"),
	}

	config := &model.Config{}
	config.SetDefaults()
	testHelper.SetConfig(config)

	return testHelper
}

// InitBasic simply proxies to api4.InitBasic, while still returning a testHelper.
func (h *testHelper) InitBasic() *testHelper {
	h.TestHelper.InitBasic()
	return h
}

// TemporaryDirectory returns the temporary directory created for user by the test helper.
func (h *testHelper) TemporaryDirectory() string {
	return h.tempDir
}

// Config returns the configuration passed to a running command.
func (h *testHelper) Config() *model.Config {
	return h.config.Clone()
}

// ConfigPath returns the path to the temporary config file passed to a running command.
func (h *testHelper) ConfigPath() string {
	return h.configFilePath
}

// SetConfig replaces the configuration passed to a running command.
func (h *testHelper) SetConfig(config *model.Config) {
	if !testing.Short() {
		config.SqlSettings = *mainHelper.GetSQLSettings()
	}

	// Disable strict password requirements for test
	*config.PasswordSettings.MinimumLength = 5
	*config.PasswordSettings.Lowercase = false
	*config.PasswordSettings.Uppercase = false
	*config.PasswordSettings.Symbol = false
	*config.PasswordSettings.Number = false

	h.config = config

	if err := ioutil.WriteFile(h.configFilePath, []byte(config.ToJson()), 0600); err != nil {
		panic("failed to write file " + h.configFilePath + ": " + err.Error())
	}
}

// SetAutoConfig configures whether the --config flag is automatically passed to a running command.
func (h *testHelper) SetAutoConfig(autoConfig bool) {
	h.disableAutoConfig = !autoConfig
}

// TearDown cleans up temporary files and assets created during the life of the test helper.
func (h *testHelper) TearDown() {
	h.TestHelper.TearDown()
	os.RemoveAll(h.tempDir)
}

func (h *testHelper) execArgs(t *testing.T, args []string) []string {
	ret := []string{"-test.v", "-test.run", "ExecCommand"}
	if coverprofile := flag.Lookup("test.coverprofile").Value.String(); coverprofile != "" {
		dir := filepath.Dir(coverprofile)
		base := filepath.Base(coverprofile)
		baseParts := strings.SplitN(base, ".", 2)
		name := strings.Replace(t.Name(), "/", "_", -1)
		coverprofileCounters[name] = coverprofileCounters[name] + 1
		baseParts[0] = fmt.Sprintf("%v-%v-%v", baseParts[0], name, coverprofileCounters[name])
		ret = append(ret, "-test.coverprofile", filepath.Join(dir, strings.Join(baseParts, ".")))
	}

	ret = append(ret, "--")

	// Unless the test passes a `--config` of its own, create a temporary one from the default
	// configuration with the current test database applied.
	hasConfig := h.disableAutoConfig
	for _, arg := range args {
		if arg == "--config" {
			hasConfig = true
			break
		}
	}

	if !hasConfig {
		ret = append(ret, "--config", h.configFilePath)
	}

	ret = append(ret, args...)

	return ret
}

func (h *testHelper) cmd(t *testing.T, args []string) *exec.Cmd {
	path, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(path, h.execArgs(t, args)...)

	cmd.Env = []string{}
	for _, env := range os.Environ() {
		// Ignore MM_SQLSETTINGS_DATASOURCE from the environment, since we override.
		if strings.HasPrefix(env, "MM_SQLSETTINGS_DATASOURCE=") {
			continue
		}

		cmd.Env = append(cmd.Env, env)
	}

	return cmd
}

// CheckCommand invokes the test binary, returning the output modified for assertion testing.
func (h *testHelper) CheckCommand(t *testing.T, args ...string) string {
	output, err := h.cmd(t, args).CombinedOutput()
	require.NoError(t, err, string(output))
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(string(output)), "PASS"))
}

// RunCommand invokes the test binary, returning only any error.
func (h *testHelper) RunCommand(t *testing.T, args ...string) error {
	return h.cmd(t, args).Run()
}

// RunCommandWithOutput is a variant of RunCommand that returns the unmodified output and any error.
func (h *testHelper) RunCommandWithOutput(t *testing.T, args ...string) (string, error) {
	cmd := h.cmd(t, args)

	var buf bytes.Buffer
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer

	done := make(chan bool)
	go func() {
		io.Copy(&buf, reader)
		close(done)
	}()

	err := cmd.Run()
	writer.Close()
	<-done

	return buf.String(), err
}
