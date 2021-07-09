// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"database/sql"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	defaultMysqlDSN        = "mmuser:mostest@tcp(localhost:3306)/mattermost_test?charset=utf8mb4,utf8&readTimeout=30s&writeTimeout=30s&multiStatements=true"
	defaultPostgresqlDSN   = "postgres://mmuser:mostest@localhost:5432/mattermost_test?sslmode=disable&connect_timeout=10"
	defaultMysqlRootPWD    = "mostest"
	defaultMysqlReplicaDSN = "root:mostest@tcp(localhost:3307)/mattermost_test?charset=utf8mb4,utf8\u0026readTimeout=30s"
)

func getEnv(name, defaultValue string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return defaultValue
}

func log(message string) {
	verbose := false
	if verboseFlag := flag.Lookup("test.v"); verboseFlag != nil {
		verbose = verboseFlag.Value.String() != ""
	}
	if verboseFlag := flag.Lookup("v"); verboseFlag != nil {
		verbose = verboseFlag.Value.String() != ""
	}

	if verbose {
		fmt.Println(message)
	}
}

// MySQLSettings returns the database settings to connect to the MySQL unittesting database.
// The database name is generated randomly and must be created before use.
func MySQLSettings(withReplica bool) *model.SQLSettings {
	dsn := getEnv("TEST_DATABASE_MYSQL_DSN", defaultMysqlDSN)
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		panic("failed to parse dsn " + dsn + ": " + err.Error())
	}

	cfg.DBName = "db" + model.NewID()

	mySQLSettings := databaseSettings("mysql", cfg.FormatDSN())

	if withReplica {
		mySQLSettings.DataSourceReplicas = []string{getEnv("TEST_DATABASE_MYSQL_REPLICA_DSN", defaultMysqlReplicaDSN)}
	}

	return mySQLSettings
}

// PostgresSQLSettings returns the database settings to connect to the PostgreSQL unittesting database.
// The database name is generated randomly and must be created before use.
func PostgreSQLSettings() *model.SQLSettings {
	dsn := getEnv("TEST_DATABASE_POSTGRESQL_DSN", defaultPostgresqlDSN)
	dsnURL, err := url.Parse(dsn)
	if err != nil {
		panic("failed to parse dsn " + dsn + ": " + err.Error())
	}

	// Generate a random database name
	dsnURL.Path = "db" + model.NewID()

	return databaseSettings("postgres", dsnURL.String())
}

func mySQLRootDSN(dsn string) string {
	rootPwd := getEnv("TEST_DATABASE_MYSQL_ROOT_PASSWD", defaultMysqlRootPWD)
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		panic("failed to parse dsn " + dsn + ": " + err.Error())
	}

	cfg.User = "root"
	cfg.Passwd = rootPwd
	cfg.DBName = "mysql"

	return cfg.FormatDSN()
}

func postgreSQLRootDSN(dsn string) string {
	dsnURL, err := url.Parse(dsn)
	if err != nil {
		panic("failed to parse dsn " + dsn + ": " + err.Error())
	}

	// // Assume the unittesting database has the same password.
	// password := ""
	// if dsnUrl.User != nil {
	// 	password, _ = dsnUrl.User.Password()
	// }

	// dsnUrl.User = url.UserPassword("", password)
	dsnURL.Path = "postgres"

	return dsnURL.String()
}

func mySQLDSNDatabase(dsn string) string {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		panic("failed to parse dsn " + dsn + ": " + err.Error())
	}

	return cfg.DBName
}

func postgreSQLDSNDatabase(dsn string) string {
	dsnURL, err := url.Parse(dsn)
	if err != nil {
		panic("failed to parse dsn " + dsn + ": " + err.Error())
	}

	return path.Base(dsnURL.Path)
}

func databaseSettings(driver, dataSource string) *model.SQLSettings {
	settings := &model.SQLSettings{
		DriverName:                  &driver,
		DataSource:                  &dataSource,
		DataSourceReplicas:          []string{},
		DataSourceSearchReplicas:    []string{},
		MaxIdleConns:                new(int),
		ConnMaxLifetimeMilliseconds: new(int),
		ConnMaxIdleTimeMilliseconds: new(int),
		MaxOpenConns:                new(int),
		Trace:                       model.NewBool(false),
		AtRestEncryptKey:            model.NewString(model.NewRandomString(32)),
		QueryTimeout:                new(int),
	}
	*settings.MaxIdleConns = 10
	*settings.ConnMaxLifetimeMilliseconds = 3600000
	*settings.ConnMaxIdleTimeMilliseconds = 300000
	*settings.MaxOpenConns = 100
	*settings.QueryTimeout = 60

	return settings
}

// execAsRoot executes the given sql as root against the testing database
func execAsRoot(settings *model.SQLSettings, sqlCommand string) error {
	var dsn string
	var driver = *settings.DriverName

	switch driver {
	case model.DatabaseDriverMysql:
		dsn = mySQLRootDSN(*settings.DataSource)
	case model.DatabaseDriverPostgres:
		dsn = postgreSQLRootDSN(*settings.DataSource)
	default:
		return fmt.Errorf("unsupported driver %s", driver)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to %s database as root", driver)
	}
	defer db.Close()
	if _, err = db.Exec(sqlCommand); err != nil {
		return errors.Wrapf(err, "failed to execute `%s` against %s database as root", sqlCommand, driver)
	}

	return nil
}

func replaceMySQLDatabaseName(dsn, newDBName string) string {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		panic("failed to parse dsn " + dsn + ": " + err.Error())
	}
	cfg.DBName = newDBName
	return cfg.FormatDSN()
}

// MakeSqlSettings creates a randomly named database and returns the corresponding sql settings
func MakeSQLSettings(driver string, withReplica bool) *model.SQLSettings {
	var settings *model.SQLSettings
	var dbName string

	switch driver {
	case model.DatabaseDriverMysql:
		settings = MySQLSettings(withReplica)
		dbName = mySQLDSNDatabase(*settings.DataSource)
		newDSRs := []string{}
		for _, dataSource := range settings.DataSourceReplicas {
			newDSRs = append(newDSRs, replaceMySQLDatabaseName(dataSource, dbName))
		}
		settings.DataSourceReplicas = newDSRs
	case model.DatabaseDriverPostgres:
		settings = PostgreSQLSettings()
		dbName = postgreSQLDSNDatabase(*settings.DataSource)
	default:
		panic("unsupported driver " + driver)
	}

	if err := execAsRoot(settings, "CREATE DATABASE "+dbName); err != nil {
		panic("failed to create temporary database " + dbName + ": " + err.Error())
	}

	switch driver {
	case model.DatabaseDriverMysql:
		if err := execAsRoot(settings, "GRANT ALL PRIVILEGES ON "+dbName+".* TO 'mmuser'"); err != nil {
			panic("failed to grant mmuser permission to " + dbName + ":" + err.Error())
		}
	case model.DatabaseDriverPostgres:
		if err := execAsRoot(settings, "GRANT ALL PRIVILEGES ON DATABASE \""+dbName+"\" TO mmuser"); err != nil {
			panic("failed to grant mmuser permission to " + dbName + ":" + err.Error())
		}
	default:
		panic("unsupported driver " + driver)
	}

	log("Created temporary " + driver + " database " + dbName)

	return settings
}

func CleanupSQLSettings(settings *model.SQLSettings) {
	var driver = *settings.DriverName
	var dbName string

	switch driver {
	case model.DatabaseDriverMysql:
		dbName = mySQLDSNDatabase(*settings.DataSource)
	case model.DatabaseDriverPostgres:
		dbName = postgreSQLDSNDatabase(*settings.DataSource)
	default:
		panic("unsupported driver " + driver)
	}

	if err := execAsRoot(settings, "DROP DATABASE "+dbName); err != nil {
		panic("failed to drop temporary database " + dbName + ": " + err.Error())
	}

	log("Dropped temporary database " + dbName)
}
