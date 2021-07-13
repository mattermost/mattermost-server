// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"bytes"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/mattermost/mattermost-server/v5/api4"
	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/manualtesting"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/web"
	"github.com/mattermost/mattermost-server/v5/wsapi"
)

var serverCmd = &cobra.Command{
	Use:          "server",
	Short:        "Run the Mattermost server",
	RunE:         serverCmdF,
	SilenceUsage: true,
}

func init() {
	RootCmd.AddCommand(serverCmd)
	RootCmd.RunE = serverCmdF
}

func serverCmdF(command *cobra.Command, args []string) error {
	interruptChan := make(chan os.Signal, 1)

	if err := utils.TranslationsPreInit(); err != nil {
		return errors.Wrap(err, "unable to load Mattermost translation files")
	}

	customDefaults, err := loadCustomDefaults()
	if err != nil {
		mlog.Warn("Error loading custom configuration defaults: " + err.Error())
	}

	configStore, err := config.NewStoreFromDSN(getConfigDSN(command, config.GetEnvironment()), false, customDefaults)
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}
	defer configStore.Close()

	return runServer(configStore, interruptChan)
}

func runServer(configStore *config.Store, interruptChan chan os.Signal) error {
	// Setting the highest traceback level from the code.
	// This is done to print goroutines from all threads (see golang.org/issue/13161)
	// and also preserve a crash dump for later investigation.
	debug.SetTraceback("crash")

	options := []app.Option{
		app.ConfigStore(configStore),
		app.RunEssentialJobs,
		app.JoinCluster,
		app.StartSearchEngine,
		app.StartMetrics,
	}
	server, err := app.NewServer(options...)
	if err != nil {
		mlog.Critical(err.Error())
		return err
	}
	defer server.Shutdown()
	// We add this after shutdown so that it can be called
	// before server shutdown happens as it can close
	// the advanced logger and prevent the mlog call from working properly.
	defer func() {
		// A panic pass-through layer which just logs it
		// and sends it upwards.
		if x := recover(); x != nil {
			var buf bytes.Buffer
			pprof.Lookup("goroutine").WriteTo(&buf, 2)
			mlog.Critical("A panic occurred",
				mlog.Any("error", x),
				mlog.String("stack", buf.String()))
			panic(x)
		}
	}()

	a := app.New(app.ServerConnector(server))
	api := api4.Init(a, server.Router)

	wsapi.Init(server)
	web.New(a, server.Router)
	api4.InitLocal(a, server.LocalRouter)

	serverErr := server.Start()
	if serverErr != nil {
		mlog.Critical(serverErr.Error())
		return serverErr
	}

	// If we allow testing then listen for manual testing URL hits
	if *server.Config().ServiceSettings.EnableTesting {
		manualtesting.Init(api)
	}

	notifyReady()

	// wait for kill signal before attempting to gracefully shutdown
	// the running service
	signal.Notify(interruptChan, syscall.SIGINT, syscall.SIGTERM)
	<-interruptChan

	return nil
}

func notifyReady() {
	// If the environment vars provide a systemd notification socket,
	// notify systemd that the server is ready.
	systemdSocket := os.Getenv("NOTIFY_SOCKET")
	if systemdSocket != "" {
		mlog.Info("Sending systemd READY notification.")

		err := sendSystemdReadyNotification(systemdSocket)
		if err != nil {
			mlog.Error(err.Error())
		}
	}
}

func sendSystemdReadyNotification(socketPath string) error {
	msg := "READY=1"
	addr := &net.UnixAddr{
		Name: socketPath,
		Net:  "unixgram",
	}
	conn, err := net.DialUnix(addr.Net, nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte(msg))
	return err
}
