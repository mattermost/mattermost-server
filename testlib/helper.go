// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package testlib

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/searchlayer"
	"github.com/mattermost/mattermost-server/v5/store/sqlstore"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
	"github.com/mattermost/mattermost-server/v5/utils"
)

type MainHelper struct {
	Settings         *model.SQLSettings
	Store            store.Store
	SearchEngine     *searchengine.Broker
	SQLStore         *sqlstore.SQLStore
	ClusterInterface *FakeClusterInterface

	status           int
	testResourcePath string
	replicas         []string
}

type HelperOptions struct {
	EnableStore     bool
	EnableResources bool
	WithReadReplica bool
}

func NewMainHelper() *MainHelper {
	return NewMainHelperWithOptions(&HelperOptions{
		EnableStore:     true,
		EnableResources: true,
	})
}

func NewMainHelperWithOptions(options *HelperOptions) *MainHelper {
	var mainHelper MainHelper
	flag.Parse()

	// Setup a global logger to catch tests logging outside of app context
	// The global logger will be stomped by apps initializing but that's fine for testing.
	// Ideally this won't happen.
	mlog.InitGlobalLogger(mlog.NewLogger(&mlog.LoggerConfiguration{
		EnableConsole: true,
		ConsoleJSON:   true,
		ConsoleLevel:  "error",
		EnableFile:    false,
	}))

	utils.TranslationsPreInit()

	if options != nil {
		if options.EnableStore && !testing.Short() {
			mainHelper.setupStore(options.WithReadReplica)
		}

		if options.EnableResources {
			mainHelper.setupResources()
		}
	}

	return &mainHelper
}

func (h *MainHelper) Main(m *testing.M) {
	if h.testResourcePath != "" {
		prevDir, err := os.Getwd()
		if err != nil {
			panic("Failed to get current working directory: " + err.Error())
		}

		err = os.Chdir(h.testResourcePath)
		if err != nil {
			panic(fmt.Sprintf("Failed to set current working directory to %s: %s", h.testResourcePath, err.Error()))
		}

		defer func() {
			err := os.Chdir(prevDir)
			if err != nil {
				panic(fmt.Sprintf("Failed to restore current working directory to %s: %s", prevDir, err.Error()))
			}
		}()
	}

	h.status = m.Run()
}

func (h *MainHelper) setupStore(withReadReplica bool) {
	driverName := os.Getenv("MM_SQLSETTINGS_DRIVERNAME")
	if driverName == "" {
		driverName = model.DatabaseDriverPostgres
	}

	h.Settings = storetest.MakeSQLSettings(driverName, withReadReplica)
	h.replicas = h.Settings.DataSourceReplicas

	config := &model.Config{}
	config.SetDefaults()

	h.SearchEngine = searchengine.NewBroker(config, nil)
	h.ClusterInterface = &FakeClusterInterface{}
	h.SQLStore = sqlstore.New(*h.Settings, nil)
	h.Store = searchlayer.NewSearchLayer(&TestStore{
		h.SQLStore,
	}, h.SearchEngine, config)
}

func (h *MainHelper) ToggleReplicasOff() {
	if h.SQLStore.GetLicense() == nil {
		panic("expecting a license to use this")
	}
	h.Settings.DataSourceReplicas = []string{}
	lic := h.SQLStore.GetLicense()
	h.SQLStore = sqlstore.New(*h.Settings, nil)
	h.SQLStore.UpdateLicense(lic)
}

func (h *MainHelper) ToggleReplicasOn() {
	if h.SQLStore.GetLicense() == nil {
		panic("expecting a license to use this")
	}
	h.Settings.DataSourceReplicas = h.replicas
	lic := h.SQLStore.GetLicense()
	h.SQLStore = sqlstore.New(*h.Settings, nil)
	h.SQLStore.UpdateLicense(lic)
}

func (h *MainHelper) setupResources() {
	var err error
	h.testResourcePath, err = SetupTestResources()
	if err != nil {
		panic("failed to setup test resources: " + err.Error())
	}
}

// PreloadMigrations preloads the migrations and roles into the database
// so that they are not run again when the migrations happen every time
// the server is started.
// This change is forward-compatible with new migrations and only new migrations
// will get executed.
// Only if the schema of either roles or systems table changes, this will break.
// In that case, just update the migrations or comment this out for the time being.
// In the worst case, only an optimization is lost.
//
// Re-generate the files with:
// pg_dump -a -h localhost -U mmuser -d <> --no-comments --inserts -t roles -t systems
// mysqldump -u root -p <> --no-create-info --extended-insert=FALSE Systems Roles
// And keep only the permission related rows in the systems table output.
func (h *MainHelper) PreloadMigrations() {
	var buf []byte
	var err error
	basePath := os.Getenv("MM_SERVER_PATH")
	relPath := "testlib/testdata"
	switch *h.Settings.DriverName {
	case model.DatabaseDriverPostgres:
		var finalPath string
		if basePath != "" {
			finalPath = filepath.Join(basePath, relPath, "postgres_migration_warmup.sql")
		} else {
			finalPath = filepath.Join("mattermost-server", relPath, "postgres_migration_warmup.sql")
		}
		buf, err = ioutil.ReadFile(finalPath)
		if err != nil {
			panic(fmt.Errorf("cannot read file: %v", err))
		}
	case model.DatabaseDriverMysql:
		var finalPath string
		if basePath != "" {
			finalPath = filepath.Join(basePath, relPath, "mysql_migration_warmup.sql")
		} else {
			finalPath = filepath.Join("mattermost-server", relPath, "mysql_migration_warmup.sql")
		}
		buf, err = ioutil.ReadFile(finalPath)
		if err != nil {
			panic(fmt.Errorf("cannot read file: %v", err))
		}
	}
	handle := h.SQLStore.GetMaster()
	_, err = handle.Exec(string(buf))
	if err != nil {
		panic(errors.Wrap(err, "Error preloading migrations. Check if you have &multiStatements=true in your DSN if you are using MySQL. Or perhaps the schema changed? If yes, then update the warmup files accordingly"))
	}
}

func (h *MainHelper) Close() error {
	if h.SQLStore != nil {
		h.SQLStore.Close()
	}
	if h.Settings != nil {
		storetest.CleanupSQLSettings(h.Settings)
	}
	if h.testResourcePath != "" {
		os.RemoveAll(h.testResourcePath)
	}

	if r := recover(); r != nil {
		log.Fatalln(r)
	}

	os.Exit(h.status)

	return nil
}

func (h *MainHelper) GetSQLSettings() *model.SQLSettings {
	if h.Settings == nil {
		panic("MainHelper not initialized with database access.")
	}

	return h.Settings
}

func (h *MainHelper) GetStore() store.Store {
	if h.Store == nil {
		panic("MainHelper not initialized with store.")
	}

	return h.Store
}

func (h *MainHelper) GetSQLStore() *sqlstore.SQLStore {
	if h.SQLStore == nil {
		panic("MainHelper not initialized with sql store.")
	}

	return h.SQLStore
}

func (h *MainHelper) GetClusterInterface() *FakeClusterInterface {
	if h.ClusterInterface == nil {
		panic("MainHelper not initialized with cluster interface.")
	}

	return h.ClusterInterface
}

func (h *MainHelper) GetSearchEngine() *searchengine.Broker {
	if h.SearchEngine == nil {
		panic("MainHelper not initialized with search engine")
	}

	return h.SearchEngine
}

func (h *MainHelper) SetReplicationLagForTesting(seconds int) error {
	if dn := h.SQLStore.DriverName(); dn != model.DatabaseDriverMysql {
		return fmt.Errorf("method not implemented for %q database driver, only %q is supported", dn, model.DatabaseDriverMysql)
	}

	err := h.execOnEachReplica("STOP SLAVE SQL_THREAD FOR CHANNEL ''")
	if err != nil {
		return err
	}

	err = h.execOnEachReplica(fmt.Sprintf("CHANGE MASTER TO MASTER_DELAY = %d", seconds))
	if err != nil {
		return err
	}

	err = h.execOnEachReplica("START SLAVE SQL_THREAD FOR CHANNEL ''")
	if err != nil {
		return err
	}

	return nil
}

func (h *MainHelper) execOnEachReplica(query string, args ...interface{}) error {
	for _, replica := range h.SQLStore.Replicas {
		_, err := replica.Exec(query, args...)
		if err != nil {
			return err
		}
	}
	return nil
}
