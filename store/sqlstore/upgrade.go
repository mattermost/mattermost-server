// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/timezones"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	CurrentSchemaVersion   = Version5370
	Version5380            = "5.38.0"
	Version5370            = "5.37.0"
	Version5360            = "5.36.0"
	Version5350            = "5.35.0"
	Version5340            = "5.34.0"
	Version5330            = "5.33.0"
	Version5320            = "5.32.0"
	Version5310            = "5.31.0"
	Version5300            = "5.30.0"
	Version5291            = "5.29.1"
	Version5290            = "5.29.0"
	Version5281            = "5.28.1"
	Version5280            = "5.28.0"
	Version5270            = "5.27.0"
	Version5260            = "5.26.0"
	Version5250            = "5.25.0"
	Version5240            = "5.24.0"
	Version5230            = "5.23.0"
	Version5220            = "5.22.0"
	Version5210            = "5.21.0"
	Version5200            = "5.20.0"
	Version5190            = "5.19.0"
	Version5180            = "5.18.0"
	Version5170            = "5.17.0"
	Version5160            = "5.16.0"
	Version5150            = "5.15.0"
	Version5140            = "5.14.0"
	Version5130            = "5.13.0"
	Version5120            = "5.12.0"
	Version5110            = "5.11.0"
	Version5100            = "5.10.0"
	Version590             = "5.9.0"
	Version580             = "5.8.0"
	Version570             = "5.7.0"
	Version560             = "5.6.0"
	Version550             = "5.5.0"
	Version540             = "5.4.0"
	Version530             = "5.3.0"
	Version520             = "5.2.0"
	Version510             = "5.1.0"
	Version500             = "5.0.0"
	Version4100            = "4.10.0"
	Version490             = "4.9.0"
	Version481             = "4.8.1"
	Version480             = "4.8.0"
	Version472             = "4.7.2"
	Version471             = "4.7.1"
	Version470             = "4.7.0"
	Version460             = "4.6.0"
	Version450             = "4.5.0"
	Version440             = "4.4.0"
	Version430             = "4.3.0"
	Version420             = "4.2.0"
	Version410             = "4.1.0"
	Version400             = "4.0.0"
	Version3100            = "3.10.0"
	Version390             = "3.9.0"
	Version380             = "3.8.0"
	Version370             = "3.7.0"
	Version360             = "3.6.0"
	Version350             = "3.5.0"
	Version340             = "3.4.0"
	Version330             = "3.3.0"
	Version320             = "3.2.0"
	Version310             = "3.1.0"
	Version300             = "3.0.0"
	OldestSupportedVersion = Version300
)

const (
	ExitVersionSave                 = 1003
	ExitThemeMigration              = 1004
	ExitTeamInviteIDMigrationFailed = 1006
)

// upgradeDatabase attempts to migrate the schema to the latest supported version.
// The value of model.CurrentVersion is accepted as a parameter for unit testing, but it is not
// used to stop migrations at that version.
func upgradeDatabase(sqlStore *SqlStore, currentModelVersionString string) error {
	currentModelVersion, err := semver.Parse(currentModelVersionString)
	if err != nil {
		return errors.Wrapf(err, "failed to parse current model version %s", currentModelVersionString)
	}

	nextUnsupportedMajorVersion := semver.Version{
		Major: currentModelVersion.Major + 1,
	}

	oldestSupportedVersion, err := semver.Parse(OldestSupportedVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to parse oldest supported version %s", OldestSupportedVersion)
	}

	var currentSchemaVersion *semver.Version
	currentSchemaVersionString := sqlStore.GetCurrentSchemaVersion()
	if currentSchemaVersionString != "" {
		currentSchemaVersion, err = semver.New(currentSchemaVersionString)
		if err != nil {
			return errors.Wrapf(err, "failed to parse database schema version %s", currentSchemaVersionString)
		}
	}

	// Assume a fresh database if no schema version has been recorded.
	if currentSchemaVersion == nil {
		if err := sqlStore.System().SaveOrUpdate(&model.System{Name: "Version", Value: currentModelVersion.String()}); err != nil {
			return errors.Wrap(err, "failed to initialize schema version for fresh database")
		}

		currentSchemaVersion = &currentModelVersion
		mlog.Info("The database schema version has been set", mlog.String("version", currentSchemaVersion.String()))
		return nil
	}

	// Upgrades prior to the oldest supported version are not supported.
	if currentSchemaVersion.LT(oldestSupportedVersion) {
		return errors.Errorf("Database schema version %s is no longer supported. This Mattermost server supports automatic upgrades from schema version %s through schema version %s. Please manually upgrade to at least version %s before continuing.", *currentSchemaVersion, oldestSupportedVersion, currentModelVersion, oldestSupportedVersion)
	}

	// Allow forwards compatibility only within the same major version.
	if currentSchemaVersion.GTE(nextUnsupportedMajorVersion) {
		return errors.Errorf("Database schema version %s is not supported. This Mattermost server supports only >=%s, <%s. Please upgrade to at least version %s before continuing.", *currentSchemaVersion, currentModelVersion, nextUnsupportedMajorVersion, nextUnsupportedMajorVersion)
	} else if currentSchemaVersion.GT(currentModelVersion) {
		mlog.Warn("The database schema version and model versions do not match", mlog.String("schema_version", currentSchemaVersion.String()), mlog.String("model_version", currentModelVersion.String()))
	}

	// Otherwise, apply any necessary migrations. Note that these methods currently invoke
	// os.Exit instead of returning an error.
	upgradeDatabaseToVersion31(sqlStore)
	upgradeDatabaseToVersion32(sqlStore)
	upgradeDatabaseToVersion33(sqlStore)
	upgradeDatabaseToVersion34(sqlStore)
	upgradeDatabaseToVersion35(sqlStore)
	upgradeDatabaseToVersion36(sqlStore)
	upgradeDatabaseToVersion37(sqlStore)
	upgradeDatabaseToVersion38(sqlStore)
	upgradeDatabaseToVersion39(sqlStore)
	upgradeDatabaseToVersion310(sqlStore)
	upgradeDatabaseToVersion40(sqlStore)
	upgradeDatabaseToVersion41(sqlStore)
	upgradeDatabaseToVersion42(sqlStore)
	upgradeDatabaseToVersion43(sqlStore)
	upgradeDatabaseToVersion44(sqlStore)
	upgradeDatabaseToVersion45(sqlStore)
	upgradeDatabaseToVersion46(sqlStore)
	upgradeDatabaseToVersion47(sqlStore)
	upgradeDatabaseToVersion471(sqlStore)
	upgradeDatabaseToVersion472(sqlStore)
	upgradeDatabaseToVersion48(sqlStore)
	upgradeDatabaseToVersion481(sqlStore)
	upgradeDatabaseToVersion49(sqlStore)
	upgradeDatabaseToVersion410(sqlStore)
	upgradeDatabaseToVersion50(sqlStore)
	upgradeDatabaseToVersion51(sqlStore)
	upgradeDatabaseToVersion52(sqlStore)
	upgradeDatabaseToVersion53(sqlStore)
	upgradeDatabaseToVersion54(sqlStore)
	upgradeDatabaseToVersion55(sqlStore)
	upgradeDatabaseToVersion56(sqlStore)
	upgradeDatabaseToVersion57(sqlStore)
	upgradeDatabaseToVersion58(sqlStore)
	upgradeDatabaseToVersion59(sqlStore)
	upgradeDatabaseToVersion510(sqlStore)
	upgradeDatabaseToVersion511(sqlStore)
	upgradeDatabaseToVersion512(sqlStore)
	upgradeDatabaseToVersion513(sqlStore)
	upgradeDatabaseToVersion514(sqlStore)
	upgradeDatabaseToVersion515(sqlStore)
	upgradeDatabaseToVersion516(sqlStore)
	upgradeDatabaseToVersion517(sqlStore)
	upgradeDatabaseToVersion518(sqlStore)
	upgradeDatabaseToVersion519(sqlStore)
	upgradeDatabaseToVersion520(sqlStore)
	upgradeDatabaseToVersion521(sqlStore)
	upgradeDatabaseToVersion522(sqlStore)
	upgradeDatabaseToVersion523(sqlStore)
	upgradeDatabaseToVersion524(sqlStore)
	upgradeDatabaseToVersion525(sqlStore)
	upgradeDatabaseToVersion526(sqlStore)
	upgradeDatabaseToVersion527(sqlStore)
	upgradeDatabaseToVersion528(sqlStore)
	upgradeDatabaseToVersion5281(sqlStore)
	upgradeDatabaseToVersion529(sqlStore)
	upgradeDatabaseToVersion5291(sqlStore)
	upgradeDatabaseToVersion530(sqlStore)
	upgradeDatabaseToVersion531(sqlStore)
	upgradeDatabaseToVersion532(sqlStore)
	upgradeDatabaseToVersion533(sqlStore)
	upgradeDatabaseToVersion534(sqlStore)
	upgradeDatabaseToVersion535(sqlStore)
	upgradeDatabaseToVersion536(sqlStore)
	upgradeDatabaseToVersion537(sqlStore)
	upgradeDatabaseToVersion538(sqlStore)

	return nil
}

func saveSchemaVersion(sqlStore *SqlStore, version string) {
	if err := sqlStore.System().SaveOrUpdate(&model.System{Name: "Version", Value: version}); err != nil {
		mlog.Critical(err.Error())
		time.Sleep(time.Second)
		os.Exit(ExitVersionSave)
	}

	mlog.Warn("The database schema version has been upgraded", mlog.String("version", version))
}

func shouldPerformUpgrade(sqlStore *SqlStore, currentSchemaVersion string, expectedSchemaVersion string) bool {
	storedSchemaVersion := sqlStore.GetCurrentSchemaVersion()

	storedVersion, err := semver.Parse(storedSchemaVersion)
	if err != nil {
		mlog.Error("Error parsing stored schema version", mlog.Err(err))
		return false
	}

	currentVersion, err := semver.Parse(currentSchemaVersion)
	if err != nil {
		mlog.Error("Error parsing current schema version", mlog.Err(err))
		return false
	}

	if storedVersion.Major == currentVersion.Major && storedVersion.Minor == currentVersion.Minor {
		mlog.Warn("Attempting to upgrade the database schema version",
			mlog.String("stored_version", storedSchemaVersion), mlog.String("current_version", currentSchemaVersion), mlog.String("new_version", expectedSchemaVersion))
		return true
	}

	return false
}

func upgradeDatabaseToVersion31(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version300, Version310) {
		sqlStore.CreateColumnIfNotExists("OutgoingWebhooks", "ContentType", "varchar(128)", "varchar(128)", "")
		saveSchemaVersion(sqlStore, Version310)
	}
}

func upgradeDatabaseToVersion32(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version310, Version320) {
		saveSchemaVersion(sqlStore, Version320)
	}
}

func themeMigrationFailed(err error) {
	mlog.Critical("Failed to migrate User.ThemeProps to Preferences table", mlog.Err(err))
	time.Sleep(time.Second)
	os.Exit(ExitThemeMigration)
}

func upgradeDatabaseToVersion33(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version320, Version330) {
		if sqlStore.DoesColumnExist("Users", "ThemeProps") {
			params := map[string]interface{}{
				"Category": model.PREFERENCE_CATEGORY_THEME,
				"Name":     "",
			}

			transaction, err := sqlStore.GetMaster().Begin()
			if err != nil {
				themeMigrationFailed(err)
			}
			defer finalizeTransaction(transaction)

			// increase size of Value column of Preferences table to match the size of the ThemeProps column
			if sqlStore.DriverName() == model.DATABASE_DRIVER_POSTGRES {
				if _, err := transaction.ExecNoTimeout("ALTER TABLE Preferences ALTER COLUMN Value TYPE varchar(2000)"); err != nil {
					themeMigrationFailed(err)
					return
				}
			} else if sqlStore.DriverName() == model.DATABASE_DRIVER_MYSQL {
				if _, err := transaction.ExecNoTimeout("ALTER TABLE Preferences MODIFY Value text"); err != nil {
					themeMigrationFailed(err)
					return
				}
			}

			// copy data across
			if _, err := transaction.ExecNoTimeout(
				`INSERT INTO
					Preferences(UserId, Category, Name, Value)
				SELECT
					Id, '`+model.PREFERENCE_CATEGORY_THEME+`', '', ThemeProps
				FROM
					Users
				WHERE
					Users.ThemeProps != 'null'`, params); err != nil {
				themeMigrationFailed(err)
				return
			}

			// delete old data
			if _, err := transaction.ExecNoTimeout("ALTER TABLE Users DROP COLUMN ThemeProps"); err != nil {
				themeMigrationFailed(err)
				return
			}

			if err := transaction.Commit(); err != nil {
				themeMigrationFailed(err)
				return
			}

			// rename solarized_* code themes to solarized-* to match client changes in 3.0
			var data model.Preferences
			if _, err := sqlStore.GetMaster().Select(&data, "SELECT * FROM Preferences WHERE Category = '"+model.PREFERENCE_CATEGORY_THEME+"' AND Value LIKE '%solarized_%'"); err == nil {
				for i := range data {
					data[i].Value = strings.Replace(data[i].Value, "solarized_", "solarized-", -1)
				}

				sqlStore.Preference().Save(&data)
			}
		}

		sqlStore.CreateColumnIfNotExists("OAuthApps", "IsTrusted", "tinyint(1)", "boolean", "0")
		sqlStore.CreateColumnIfNotExists("OAuthApps", "IconURL", "varchar(512)", "varchar(512)", "")
		sqlStore.CreateColumnIfNotExists("OAuthAccessData", "ClientId", "varchar(26)", "varchar(26)", "")
		sqlStore.CreateColumnIfNotExists("OAuthAccessData", "UserId", "varchar(26)", "varchar(26)", "")
		sqlStore.CreateColumnIfNotExists("OAuthAccessData", "ExpiresAt", "bigint", "bigint", "0")

		if sqlStore.DoesColumnExist("OAuthAccessData", "AuthCode") {
			sqlStore.RemoveIndexIfExists("idx_oauthaccessdata_auth_code", "OAuthAccessData")
			sqlStore.RemoveColumnIfExists("OAuthAccessData", "AuthCode")
		}

		sqlStore.RemoveColumnIfExists("Users", "LastActivityAt")
		sqlStore.RemoveColumnIfExists("Users", "LastPingAt")

		sqlStore.CreateColumnIfNotExists("OutgoingWebhooks", "TriggerWhen", "tinyint", "integer", "0")

		saveSchemaVersion(sqlStore, Version330)
	}
}

func upgradeDatabaseToVersion34(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version330, Version340) {
		sqlStore.CreateColumnIfNotExists("Status", "Manual", "BOOLEAN", "BOOLEAN", "0")
		sqlStore.CreateColumnIfNotExists("Status", "ActiveChannel", "varchar(26)", "varchar(26)", "")

		saveSchemaVersion(sqlStore, Version340)
	}
}

func upgradeDatabaseToVersion35(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version340, Version350) {
		sqlStore.GetMaster().ExecNoTimeout("UPDATE Users SET Roles = 'system_user' WHERE Roles = ''")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE Users SET Roles = 'system_user system_admin' WHERE Roles = 'system_admin'")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE TeamMembers SET Roles = 'team_user' WHERE Roles = ''")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE TeamMembers SET Roles = 'team_user team_admin' WHERE Roles = 'admin'")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE ChannelMembers SET Roles = 'channel_user' WHERE Roles = ''")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE ChannelMembers SET Roles = 'channel_user channel_admin' WHERE Roles = 'admin'")

		// The rest of the migration from Filenames -> FileIds is done lazily in api.GetFileInfosForPost
		sqlStore.CreateColumnIfNotExists("Posts", "FileIds", "varchar(150)", "varchar(150)", "[]")

		// Increase maximum length of the Channel table Purpose column.
		if sqlStore.GetMaxLengthOfColumnIfExists("Channels", "Purpose") != "250" {
			sqlStore.AlterColumnTypeIfExists("Channels", "Purpose", "varchar(250)", "varchar(250)")
		}

		sqlStore.Session().RemoveAllSessions()

		saveSchemaVersion(sqlStore, Version350)
	}
}

func upgradeDatabaseToVersion36(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version350, Version360) {
		sqlStore.CreateColumnIfNotExists("Posts", "HasReactions", "tinyint", "boolean", "0")

		// Add a Position column to users.
		sqlStore.CreateColumnIfNotExists("Users", "Position", "varchar(64)", "varchar(64)", "")

		// Remove ActiveChannel column from Status
		sqlStore.RemoveColumnIfExists("Status", "ActiveChannel")

		saveSchemaVersion(sqlStore, Version360)
	}
}

func upgradeDatabaseToVersion37(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version360, Version370) {
		// Add EditAt column to Posts
		sqlStore.CreateColumnIfNotExists("Posts", "EditAt", " bigint", " bigint", "0")

		saveSchemaVersion(sqlStore, Version370)
	}
}

func upgradeDatabaseToVersion38(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version370, Version380) {
		// Add the IsPinned column to posts.
		sqlStore.CreateColumnIfNotExists("Posts", "IsPinned", "boolean", "boolean", "0")

		saveSchemaVersion(sqlStore, Version380)
	}
}

func upgradeDatabaseToVersion39(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version380, Version390) {
		sqlStore.CreateColumnIfNotExists("OAuthAccessData", "Scope", "varchar(128)", "varchar(128)", model.DEFAULT_SCOPE)
		sqlStore.RemoveTableIfExists("PasswordRecovery")

		saveSchemaVersion(sqlStore, Version390)
	}
}

func upgradeDatabaseToVersion310(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version390, Version3100) {
		saveSchemaVersion(sqlStore, Version3100)
	}
}

func upgradeDatabaseToVersion40(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version3100, Version400) {
		saveSchemaVersion(sqlStore, Version400)
	}
}

func upgradeDatabaseToVersion41(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version400, Version410) {
		// Increase maximum length of the Users table Roles column.
		if sqlStore.GetMaxLengthOfColumnIfExists("Users", "Roles") != "256" {
			sqlStore.AlterColumnTypeIfExists("Users", "Roles", "varchar(256)", "varchar(256)")
		}

		sqlStore.RemoveTableIfExists("JobStatuses")

		saveSchemaVersion(sqlStore, Version410)
	}
}

func upgradeDatabaseToVersion42(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version410, Version420) {
		saveSchemaVersion(sqlStore, Version420)
	}
}

func upgradeDatabaseToVersion43(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version420, Version430) {
		saveSchemaVersion(sqlStore, Version430)
	}
}

func upgradeDatabaseToVersion44(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version430, Version440) {
		// Add the IsActive column to UserAccessToken.
		sqlStore.CreateColumnIfNotExists("UserAccessTokens", "IsActive", "boolean", "boolean", "1")

		saveSchemaVersion(sqlStore, Version440)
	}
}

func upgradeDatabaseToVersion45(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version440, Version450) {
		saveSchemaVersion(sqlStore, Version450)
	}
}

func upgradeDatabaseToVersion46(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version450, Version460) {
		sqlStore.CreateColumnIfNotExists("IncomingWebhooks", "Username", "varchar(64)", "varchar(64)", "")
		sqlStore.CreateColumnIfNotExists("IncomingWebhooks", "IconURL", "varchar(1024)", "varchar(1024)", "")
		saveSchemaVersion(sqlStore, Version460)
	}
}

func upgradeDatabaseToVersion47(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version460, Version470) {
		sqlStore.AlterColumnTypeIfExists("Users", "Position", "varchar(128)", "varchar(128)")
		sqlStore.AlterColumnTypeIfExists("OAuthAuthData", "State", "varchar(1024)", "varchar(1024)")
		sqlStore.RemoveColumnIfExists("ChannelMemberHistory", "Email")
		sqlStore.RemoveColumnIfExists("ChannelMemberHistory", "Username")
		saveSchemaVersion(sqlStore, Version470)
	}
}

func upgradeDatabaseToVersion471(sqlStore *SqlStore) {
	// If any new instances started with 4.7, they would have the bad Email column on the
	// ChannelMemberHistory table. So for those cases we need to do an upgrade between
	// 4.7.0 and 4.7.1
	if shouldPerformUpgrade(sqlStore, Version470, Version471) {
		sqlStore.RemoveColumnIfExists("ChannelMemberHistory", "Email")
		saveSchemaVersion(sqlStore, Version471)
	}
}

func upgradeDatabaseToVersion472(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version471, Version472) {
		sqlStore.RemoveIndexIfExists("idx_channels_displayname", "Channels")
		saveSchemaVersion(sqlStore, Version472)
	}
}

func upgradeDatabaseToVersion48(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version472, Version480) {
		saveSchemaVersion(sqlStore, Version480)
	}
}

func upgradeDatabaseToVersion481(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version480, Version481) {
		sqlStore.RemoveIndexIfExists("idx_channels_displayname", "Channels")
		saveSchemaVersion(sqlStore, Version481)
	}
}

func upgradeDatabaseToVersion49(sqlStore *SqlStore) {
	// This version of Mattermost includes an App-Layer migration which migrates from hard-coded roles configured by
	// a number of parameters in `config.json` to a `Roles` table in the database. The migration code can be seen
	// in the file `app/app.go` in the function `DoAdvancedPermissionsMigration()`.

	if shouldPerformUpgrade(sqlStore, Version481, Version490) {
		defaultTimezone := timezones.DefaultUserTimezone()
		defaultTimezoneValue, err := json.Marshal(defaultTimezone)
		if err != nil {
			mlog.Critical(err.Error())
		}
		sqlStore.CreateColumnIfNotExists("Users", "Timezone", "varchar(256)", "varchar(256)", string(defaultTimezoneValue))
		sqlStore.RemoveIndexIfExists("idx_channels_displayname", "Channels")
		saveSchemaVersion(sqlStore, Version490)
	}
}

func upgradeDatabaseToVersion410(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version490, Version4100) {

		sqlStore.RemoveIndexIfExists("Name_2", "Channels")
		sqlStore.RemoveIndexIfExists("Name_2", "Emoji")
		sqlStore.RemoveIndexIfExists("ClientId_2", "OAuthAccessData")

		saveSchemaVersion(sqlStore, Version4100)
		sqlStore.GetMaster().ExecNoTimeout("UPDATE Users SET AuthData=LOWER(AuthData) WHERE AuthService = 'saml'")
	}
}

func upgradeDatabaseToVersion50(sqlStore *SqlStore) {
	// This version of Mattermost includes an App-Layer migration which migrates from hard-coded emojis configured
	// in `config.json` to a `Permission` in the database. The migration code can be seen
	// in the file `app/app.go` in the function `DoEmojisPermissionsMigration()`.

	// This version of Mattermost also includes a online-migration which migrates some roles from the `Roles` columns of
	// TeamMember and ChannelMember rows to the new SchemeAdmin and SchemeUser columns. If you need to downgrade to a
	// version of Mattermost prior to 5.0, you should take your server offline and run the following SQL statements
	// prior to launching the downgraded version:
	//
	//    UPDATE Teams SET SchemeId = NULL;
	//    UPDATE Channels SET SchemeId = NULL;
	//    UPDATE TeamMembers SET Roles = CONCAT(Roles, ' team_user'), SchemeUser = NULL where SchemeUser = 1;
	//    UPDATE TeamMembers SET Roles = CONCAT(Roles, ' team_admin'), SchemeAdmin = NULL where SchemeAdmin = 1;
	//    UPDATE ChannelMembers SET Roles = CONCAT(Roles, ' channel_user'), SchemeUser = NULL where SchemeUser = 1;
	//    UPDATE ChannelMembers SET Roles = CONCAT(Roles, ' channel_admin'), SchemeAdmin = NULL where SchemeAdmin = 1;
	//    DELETE from Systems WHERE Name = 'migration_advanced_permissions_phase_2';

	if shouldPerformUpgrade(sqlStore, Version4100, Version500) {
		sqlStore.CreateColumnIfNotExistsNoDefault("Channels", "SchemeId", "varchar(26)", "varchar(26)")

		sqlStore.CreateColumnIfNotExistsNoDefault("ChannelMembers", "SchemeUser", "boolean", "boolean")
		sqlStore.CreateColumnIfNotExistsNoDefault("ChannelMembers", "SchemeAdmin", "boolean", "boolean")

		sqlStore.CreateColumnIfNotExists("Roles", "BuiltIn", "boolean", "boolean", "0")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE Roles SET BuiltIn=true")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE Roles SET SchemeManaged=false WHERE Name NOT IN ('system_user', 'system_admin', 'team_user', 'team_admin', 'channel_user', 'channel_admin')")
		sqlStore.CreateColumnIfNotExists("IncomingWebhooks", "ChannelLocked", "boolean", "boolean", "0")

		sqlStore.RemoveIndexIfExists("idx_channels_txt", "Channels")

		saveSchemaVersion(sqlStore, Version500)
	}
}

func upgradeDatabaseToVersion51(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version500, Version510) {
		saveSchemaVersion(sqlStore, Version510)
	}
}

func upgradeDatabaseToVersion52(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version510, Version520) {
		sqlStore.CreateColumnIfNotExists("OutgoingWebhooks", "Username", "varchar(64)", "varchar(64)", "")
		sqlStore.CreateColumnIfNotExists("OutgoingWebhooks", "IconURL", "varchar(1024)", "varchar(1024)", "")
		saveSchemaVersion(sqlStore, Version520)
	}
}

func upgradeDatabaseToVersion53(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version520, Version530) {
		saveSchemaVersion(sqlStore, Version530)
	}
}

func upgradeDatabaseToVersion54(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version530, Version540) {
		sqlStore.AlterColumnTypeIfExists("OutgoingWebhooks", "Description", "varchar(500)", "varchar(500)")
		sqlStore.AlterColumnTypeIfExists("IncomingWebhooks", "Description", "varchar(500)", "varchar(500)")
		if err := sqlStore.Channel().MigratePublicChannels(); err != nil {
			mlog.Critical("Failed to migrate PublicChannels table", mlog.Err(err))
			time.Sleep(time.Second)
			os.Exit(ExitGenericFailure)
		}
		saveSchemaVersion(sqlStore, Version540)
	}
}

func upgradeDatabaseToVersion55(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version540, Version550) {
		saveSchemaVersion(sqlStore, Version550)
	}
}

func upgradeDatabaseToVersion56(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version550, Version560) {
		sqlStore.CreateColumnIfNotExists("PluginKeyValueStore", "ExpireAt", "bigint(20)", "bigint", "0")

		// migrating user's accepted terms of service data into the new table
		sqlStore.GetMaster().ExecNoTimeout("INSERT INTO UserTermsOfService SELECT Id, AcceptedTermsOfServiceId as TermsOfServiceId, :CreateAt FROM Users WHERE AcceptedTermsOfServiceId != \"\" AND AcceptedTermsOfServiceId IS NOT NULL", map[string]interface{}{"CreateAt": model.GetMillis()})

		if sqlStore.DriverName() == model.DATABASE_DRIVER_POSTGRES {
			sqlStore.RemoveIndexIfExists("idx_users_email_lower", "lower(Email)")
			sqlStore.RemoveIndexIfExists("idx_users_username_lower", "lower(Username)")
			sqlStore.RemoveIndexIfExists("idx_users_nickname_lower", "lower(Nickname)")
			sqlStore.RemoveIndexIfExists("idx_users_firstname_lower", "lower(FirstName)")
			sqlStore.RemoveIndexIfExists("idx_users_lastname_lower", "lower(LastName)")
		}

		saveSchemaVersion(sqlStore, Version560)
	}

}

func upgradeDatabaseToVersion57(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version560, Version570) {
		saveSchemaVersion(sqlStore, Version570)
	}
}

func upgradeDatabaseToVersion58(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version570, Version580) {
		// idx_channels_txt was removed in `upgradeDatabaseToVersion50`, but merged as part of
		// v5.1, so the migration wouldn't apply to anyone upgrading from v5.0. Remove it again to
		// bring the upgraded (from v5.0) and fresh install schemas back in sync.
		sqlStore.RemoveIndexIfExists("idx_channels_txt", "Channels")

		// Fix column types and defaults where gorp converged on a different schema value than the
		// original migration.
		sqlStore.AlterColumnTypeIfExists("OutgoingWebhooks", "Description", "text", "VARCHAR(500)")
		sqlStore.AlterColumnTypeIfExists("IncomingWebhooks", "Description", "text", "VARCHAR(500)")
		sqlStore.AlterColumnTypeIfExists("OutgoingWebhooks", "IconURL", "text", "VARCHAR(1024)")
		sqlStore.RemoveDefaultIfColumnExists("OutgoingWebhooks", "Username")
		if sqlStore.DriverName() == model.DATABASE_DRIVER_POSTGRES {
			sqlStore.RemoveDefaultIfColumnExists("OutgoingWebhooks", "IconURL")
		}
		sqlStore.AlterDefaultIfColumnExists("OutgoingWebhooks", "Username", model.NewString("NULL"), nil)
		sqlStore.AlterDefaultIfColumnExists("PluginKeyValueStore", "ExpireAt", model.NewString("NULL"), model.NewString("NULL"))

		saveSchemaVersion(sqlStore, Version580)
	}
}

func upgradeDatabaseToVersion59(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version580, Version590) {
		saveSchemaVersion(sqlStore, Version590)
	}
}

func upgradeDatabaseToVersion510(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version590, Version5100) {
		sqlStore.CreateColumnIfNotExistsNoDefault("Channels", "GroupConstrained", "tinyint(4)", "boolean")

		sqlStore.CreateIndexIfNotExists("idx_groupteams_teamid", "GroupTeams", "TeamId")
		sqlStore.CreateIndexIfNotExists("idx_groupchannels_channelid", "GroupChannels", "ChannelId")

		saveSchemaVersion(sqlStore, Version5100)
	}
}

func upgradeDatabaseToVersion511(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5100, Version5110) {
		// Enforce all teams have an InviteID set
		var teams []*model.Team
		if _, err := sqlStore.GetReplica().Select(&teams, "SELECT * FROM Teams WHERE InviteId = ''"); err != nil {
			mlog.Error("Error fetching Teams without InviteID", mlog.Err(err))
		} else {
			for _, team := range teams {
				team.InviteId = model.NewId()
				if _, err := sqlStore.Team().Update(team); err != nil {
					mlog.Error("Error updating Team InviteIDs", mlog.String("team_id", team.Id), mlog.Err(err))
				}
			}
		}

		saveSchemaVersion(sqlStore, Version5110)
	}
}

func upgradeDatabaseToVersion512(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5110, Version5120) {
		sqlStore.CreateColumnIfNotExistsNoDefault("ChannelMembers", "SchemeGuest", "boolean", "boolean")
		sqlStore.CreateColumnIfNotExistsNoDefault("Schemes", "DefaultTeamGuestRole", "text", "VARCHAR(64)")
		sqlStore.CreateColumnIfNotExistsNoDefault("Schemes", "DefaultChannelGuestRole", "text", "VARCHAR(64)")

		sqlStore.GetMaster().ExecNoTimeout("UPDATE Schemes SET DefaultTeamGuestRole = '', DefaultChannelGuestRole = ''")

		// Saturday, January 24, 2065 5:20:00 AM GMT. To remove all personal access token sessions.
		sqlStore.GetMaster().ExecNoTimeout("DELETE FROM Sessions WHERE ExpiresAt > 3000000000000")

		saveSchemaVersion(sqlStore, Version5120)
	}
}

func upgradeDatabaseToVersion513(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5120, Version5130) {
		// The previous jobs ran once per minute, cluttering the Jobs table with somewhat useless entries. Clean that up.
		sqlStore.GetMaster().ExecNoTimeout("DELETE FROM Jobs WHERE Type = 'plugins'")

		saveSchemaVersion(sqlStore, Version5130)
	}
}

func upgradeDatabaseToVersion514(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5130, Version5140) {
		saveSchemaVersion(sqlStore, Version5140)
	}
}

func upgradeDatabaseToVersion515(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5140, Version5150) {
		saveSchemaVersion(sqlStore, Version5150)
	}
}

func upgradeDatabaseToVersion516(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5150, Version5160) {
		if sqlStore.DriverName() == model.DATABASE_DRIVER_POSTGRES {
			sqlStore.GetMaster().ExecNoTimeout("ALTER TABLE Tokens ALTER COLUMN Extra TYPE varchar(2048)")
		} else if sqlStore.DriverName() == model.DATABASE_DRIVER_MYSQL {
			sqlStore.GetMaster().ExecNoTimeout("ALTER TABLE Tokens MODIFY Extra text")
		}
		saveSchemaVersion(sqlStore, Version5160)

		// Fix mismatches between the canonical and migrated schemas.
		sqlStore.AlterColumnTypeIfExists("Schemes", "DefaultTeamGuestRole", "varchar(64)", "VARCHAR(64)")
		sqlStore.AlterColumnTypeIfExists("Schemes", "DefaultChannelGuestRole", "varchar(64)", "VARCHAR(64)")
		sqlStore.AlterColumnTypeIfExists("Teams", "AllowedDomains", "text", "VARCHAR(1000)")
		sqlStore.AlterColumnTypeIfExists("Channels", "GroupConstrained", "tinyint(1)", "boolean")
		sqlStore.AlterColumnTypeIfExists("Teams", "GroupConstrained", "tinyint(1)", "boolean")

		// One known mismatch remains: ChannelMembers.SchemeGuest. The requisite migration
		// is left here for posterity, but we're avoiding fix this given the corresponding
		// table rewrite in most MySQL and Postgres instances.
		// sqlStore.AlterColumnTypeIfExists("ChannelMembers", "SchemeGuest", "tinyint(4)", "boolean")

		sqlStore.CreateIndexIfNotExists("idx_groupteams_teamid", "GroupTeams", "TeamId")
		sqlStore.CreateIndexIfNotExists("idx_groupchannels_channelid", "GroupChannels", "ChannelId")
	}
}

func upgradeDatabaseToVersion517(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5160, Version5170) {
		saveSchemaVersion(sqlStore, Version5170)
	}
}

func upgradeDatabaseToVersion518(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5170, Version5180) {
		saveSchemaVersion(sqlStore, Version5180)
	}
}

func upgradeDatabaseToVersion519(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5180, Version5190) {
		saveSchemaVersion(sqlStore, Version5190)
	}
}

func upgradeDatabaseToVersion520(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5190, Version5200) {
		sqlStore.CreateColumnIfNotExistsNoDefault("Bots", "LastIconUpdate", "bigint", "bigint")

		sqlStore.CreateColumnIfNotExists("GroupTeams", "SchemeAdmin", "boolean", "boolean", "0")
		sqlStore.CreateIndexIfNotExists("idx_groupteams_schemeadmin", "GroupTeams", "SchemeAdmin")

		sqlStore.CreateColumnIfNotExists("GroupChannels", "SchemeAdmin", "boolean", "boolean", "0")
		sqlStore.CreateIndexIfNotExists("idx_groupchannels_schemeadmin", "GroupChannels", "SchemeAdmin")

		saveSchemaVersion(sqlStore, Version5200)
	}
}

func upgradeDatabaseToVersion521(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5200, Version5210) {
		saveSchemaVersion(sqlStore, Version5210)
	}
}

func upgradeDatabaseToVersion522(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5210, Version5220) {
		sqlStore.CreateIndexIfNotExists("idx_teams_scheme_id", "Teams", "SchemeId")
		sqlStore.CreateIndexIfNotExists("idx_channels_scheme_id", "Channels", "SchemeId")
		sqlStore.CreateIndexIfNotExists("idx_channels_scheme_id", "Channels", "SchemeId")
		sqlStore.CreateIndexIfNotExists("idx_schemes_channel_guest_role", "Schemes", "DefaultChannelGuestRole")
		sqlStore.CreateIndexIfNotExists("idx_schemes_channel_user_role", "Schemes", "DefaultChannelUserRole")
		sqlStore.CreateIndexIfNotExists("idx_schemes_channel_admin_role", "Schemes", "DefaultChannelAdminRole")

		saveSchemaVersion(sqlStore, Version5220)
	}
}

func upgradeDatabaseToVersion523(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5220, Version5230) {
		saveSchemaVersion(sqlStore, Version5230)
	}
}

func upgradeDatabaseToVersion524(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5230, Version5240) {
		sqlStore.CreateColumnIfNotExists("UserGroups", "AllowReference", "boolean", "boolean", "0")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE UserGroups SET Name = null, AllowReference = false")
		sqlStore.AlterPrimaryKey("Reactions", []string{"PostId", "UserId", "EmojiName"})

		saveSchemaVersion(sqlStore, Version5240)
	}
}

func upgradeDatabaseToVersion525(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5240, Version5250) {
		saveSchemaVersion(sqlStore, Version5250)
	}
}

func upgradeDatabaseToVersion526(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5250, Version5260) {
		sqlStore.CreateColumnIfNotExists("Sessions", "ExpiredNotify", "boolean", "boolean", "0")

		saveSchemaVersion(sqlStore, Version5260)
	}
}

func upgradeDatabaseToVersion527(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5260, Version5270) {
		saveSchemaVersion(sqlStore, Version5270)
	}
}

func upgradeDatabaseToVersion528(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5270, Version5280) {
		if err := precheckMigrationToVersion528(sqlStore); err != nil {
			mlog.Critical("Error upgrading DB schema to 5.28.0", mlog.Err(err))
			os.Exit(ExitGenericFailure)
		}

		sqlStore.CreateColumnIfNotExistsNoDefault("Commands", "PluginId", "VARCHAR(190)", "VARCHAR(190)")
		sqlStore.GetMaster().ExecNoTimeout("UPDATE Commands SET PluginId = '' WHERE PluginId IS NULL")

		sqlStore.AlterColumnTypeIfExists("Teams", "Type", "VARCHAR(255)", "VARCHAR(255)")
		sqlStore.AlterColumnTypeIfExists("Teams", "SchemeId", "VARCHAR(26)", "VARCHAR(26)")
		sqlStore.AlterColumnTypeIfExists("IncomingWebhooks", "Username", "varchar(255)", "varchar(255)")
		sqlStore.AlterColumnTypeIfExists("IncomingWebhooks", "IconURL", "text", "varchar(1024)")

		saveSchemaVersion(sqlStore, Version5280)
	}
}

func upgradeDatabaseToVersion5281(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5280, Version5281) {
		sqlStore.CreateColumnIfNotExistsNoDefault("FileInfo", "MiniPreview", "MEDIUMBLOB", "bytea")

		saveSchemaVersion(sqlStore, Version5281)
	}
}

func precheckMigrationToVersion528(sqlStore *SqlStore) error {
	teamsQuery, _, err := sqlStore.getQueryBuilder().Select(`COALESCE(SUM(CASE
				WHEN CHAR_LENGTH(SchemeId) > 26 THEN 1
				ELSE 0
			END),0) as schemeidwrong,
			COALESCE(SUM(CASE
				WHEN CHAR_LENGTH(Type) > 255 THEN 1
				ELSE 0
			END),0) as typewrong`).
		From("Teams").ToSql()
	if err != nil {
		return err
	}
	webhooksQuery, _, err := sqlStore.getQueryBuilder().Select(`COALESCE(SUM(CASE
				WHEN CHAR_LENGTH(Username) > 255 THEN 1
				ELSE 0
			END),0) as usernamewrong,
			COALESCE(SUM(CASE
				WHEN CHAR_LENGTH(IconURL) > 1024 THEN 1
				ELSE 0
			END),0) as iconurlwrong`).
		From("IncomingWebhooks").ToSql()
	if err != nil {
		return err
	}

	var schemeIDWrong, typeWrong int
	row := sqlStore.GetMaster().Db.QueryRow(teamsQuery)
	if err = row.Scan(&schemeIDWrong, &typeWrong); err != nil && err != sql.ErrNoRows {
		return err
	} else if err == nil && schemeIDWrong > 0 {
		return errors.New("Migration failure: " +
			"Teams column SchemeId has data larger that 26 characters")
	} else if err == nil && typeWrong > 0 {
		return errors.New("Migration failure: " +
			"Teams column Type has data larger that 255 characters")
	}

	var usernameWrong, iconURLWrong int
	row = sqlStore.GetMaster().Db.QueryRow(webhooksQuery)
	if err = row.Scan(&usernameWrong, &iconURLWrong); err != nil && err != sql.ErrNoRows {
		mlog.Error("Error fetching IncomingWebhooks columns data", mlog.Err(err))
	} else if err == nil && usernameWrong > 0 {
		return errors.New("Migration failure: " +
			"IncomingWebhooks column Username has data larger that 255 characters")
	} else if err == nil && iconURLWrong > 0 {
		return errors.New("Migration failure: " +
			"IncomingWebhooks column IconURL has data larger that 1024 characters")
	}

	return nil
}

func upgradeDatabaseToVersion529(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5281, Version5290) {
		sqlStore.AlterColumnTypeIfExists("SidebarCategories", "Id", "VARCHAR(128)", "VARCHAR(128)")
		sqlStore.RemoveDefaultIfColumnExists("SidebarCategories", "Id")
		sqlStore.AlterColumnTypeIfExists("SidebarChannels", "CategoryId", "VARCHAR(128)", "VARCHAR(128)")
		sqlStore.RemoveDefaultIfColumnExists("SidebarChannels", "CategoryId")

		sqlStore.CreateColumnIfNotExistsNoDefault("Threads", "ChannelId", "VARCHAR(26)", "VARCHAR(26)")

		updateThreadChannelsQuery := "UPDATE Threads INNER JOIN Posts ON Posts.Id=Threads.PostId SET Threads.ChannelId=Posts.ChannelId WHERE Threads.ChannelId IS NULL"
		if sqlStore.DriverName() == model.DATABASE_DRIVER_POSTGRES {
			updateThreadChannelsQuery = "UPDATE Threads SET ChannelId=Posts.ChannelId FROM Posts WHERE Posts.Id=Threads.PostId AND Threads.ChannelId IS NULL"
		}
		if _, err := sqlStore.GetMaster().ExecNoTimeout(updateThreadChannelsQuery); err != nil {
			mlog.Error("Error updating ChannelId in Threads table", mlog.Err(err))
		}

		saveSchemaVersion(sqlStore, Version5290)
	}
}

func upgradeDatabaseToVersion5291(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5290, Version5291) {
		saveSchemaVersion(sqlStore, Version5291)
	}
}

func upgradeDatabaseToVersion530(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5291, Version5300) {
		sqlStore.CreateColumnIfNotExistsNoDefault("FileInfo", "Content", "longtext", "text")

		sqlStore.CreateColumnIfNotExists("SidebarCategories", "Muted", "tinyint(1)", "boolean", "0")

		saveSchemaVersion(sqlStore, Version5300)
	}
}

func upgradeDatabaseToVersion531(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5300, Version5310) {
		saveSchemaVersion(sqlStore, Version5310)
	}
}

const RemoteClusterSiteURLUniqueIndex = "remote_clusters_site_url_unique"

func hasMissingMigrationsVersion532(sqlStore *SqlStore) bool {
	scIdInfo, err := sqlStore.GetColumnInfo("Posts", "FileIds")
	if err != nil {
		mlog.Error("Error getting column info for migration check",
			mlog.String("table", "Posts"),
			mlog.String("column", "FileIds"),
			mlog.Err(err),
		)
		return true
	}

	if sqlStore.DriverName() == model.DATABASE_DRIVER_POSTGRES {
		if !sqlStore.IsVarchar(scIdInfo.DataType) || scIdInfo.CharMaximumLength != 300 {
			return true
		}
	}

	if !sqlStore.DoesColumnExist("Channels", "Shared") {
		return true
	}

	if !sqlStore.DoesColumnExist("ThreadMemberships", "UnreadMentions") {
		return true
	}

	if !sqlStore.DoesColumnExist("Reactions", "UpdateAt") {
		return true
	}

	if !sqlStore.DoesColumnExist("Reactions", "DeleteAt") {
		return true
	}

	return false
}

func upgradeDatabaseToVersion532(sqlStore *SqlStore) {
	if hasMissingMigrationsVersion532(sqlStore) {
		// this migration was reverted on MySQL due to performance reasons. Doing
		// it only on PostgreSQL for the time being.
		if sqlStore.DriverName() == model.DATABASE_DRIVER_POSTGRES {
			// allow 10 files per post
			sqlStore.AlterColumnTypeIfExists("Posts", "FileIds", "text", "varchar(300)")
		}

		sqlStore.CreateColumnIfNotExists("ThreadMemberships", "UnreadMentions", "bigint", "bigint", "0")
		// Shared channels support
		sqlStore.CreateColumnIfNotExistsNoDefault("Channels", "Shared", "tinyint(1)", "boolean")
		sqlStore.CreateColumnIfNotExistsNoDefault("Reactions", "UpdateAt", "bigint", "bigint")
		sqlStore.CreateColumnIfNotExistsNoDefault("Reactions", "DeleteAt", "bigint", "bigint")
	}

	if shouldPerformUpgrade(sqlStore, Version5310, Version5320) {
		saveSchemaVersion(sqlStore, Version5320)
	}
}

func upgradeDatabaseToVersion533(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5320, Version5330) {
		saveSchemaVersion(sqlStore, Version5330)
	}
}

func upgradeDatabaseToVersion534(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5330, Version5340) {
		saveSchemaVersion(sqlStore, Version5340)
	}
}

func upgradeDatabaseToVersion535(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5340, Version5350) {
		sqlStore.AlterColumnTypeIfExists("Roles", "Permissions", "longtext", "text")

		sqlStore.CreateColumnIfNotExists("SidebarCategories", "Collapsed", "tinyint(1)", "boolean", "0")

		// Shared channels support
		sqlStore.CreateColumnIfNotExistsNoDefault("Reactions", "RemoteId", "VARCHAR(26)", "VARCHAR(26)")
		sqlStore.CreateColumnIfNotExistsNoDefault("Users", "RemoteId", "VARCHAR(26)", "VARCHAR(26)")
		sqlStore.CreateColumnIfNotExistsNoDefault("Posts", "RemoteId", "VARCHAR(26)", "VARCHAR(26)")
		sqlStore.CreateColumnIfNotExistsNoDefault("FileInfo", "RemoteId", "VARCHAR(26)", "VARCHAR(26)")
		sqlStore.CreateColumnIfNotExists("UploadSessions", "RemoteId", "VARCHAR(26)", "VARCHAR(26)", "")
		sqlStore.CreateColumnIfNotExists("UploadSessions", "ReqFileId", "VARCHAR(26)", "VARCHAR(26)", "")
		if _, err := sqlStore.GetMaster().ExecNoTimeout("UPDATE UploadSessions SET RemoteId='', ReqFileId='' WHERE RemoteId IS NULL"); err != nil {
			mlog.Error("Error updating RemoteId,ReqFileId in UploadsSession table", mlog.Err(err))
		}
		uniquenessColumns := []string{"SiteUrl", "RemoteTeamId"}
		if sqlStore.DriverName() == model.DATABASE_DRIVER_MYSQL {
			uniquenessColumns = []string{"RemoteTeamId", "SiteUrl(168)"}
		}
		sqlStore.CreateUniqueCompositeIndexIfNotExists(RemoteClusterSiteURLUniqueIndex, "RemoteClusters", uniquenessColumns)

		rootCountMigration(sqlStore)

		saveSchemaVersion(sqlStore, Version5350)
	}
}

func upgradeDatabaseToVersion536(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5350, Version5360) {
		sqlStore.CreateColumnIfNotExists("SharedChannelUsers", "ChannelId", "VARCHAR(26)", "VARCHAR(26)", "")
		sqlStore.CreateColumnIfNotExists("SharedChannelRemotes", "LastPostUpdateAt", "bigint", "bigint", "0")
		sqlStore.CreateColumnIfNotExists("SharedChannelRemotes", "LastPostId", "VARCHAR(26)", "VARCHAR(26)", "")

		saveSchemaVersion(sqlStore, Version5360)
	}
}

func rootCountMigration(sqlStore *SqlStore) {
	totalMsgCountRootExists := sqlStore.DoesColumnExist("Channels", "TotalMsgCountRoot")
	msgCountRootExists := sqlStore.DoesColumnExist("ChannelMembers", "MsgCountRoot")

	sqlStore.CreateColumnIfNotExists("ChannelMembers", "MentionCountRoot", "bigint", "bigint", "0")
	sqlStore.AlterDefaultIfColumnExists("ChannelMembers", "MentionCountRoot", model.NewString("0"), model.NewString("0"))

	mentionCountRootCTE := `
		SELECT ChannelId, COALESCE(SUM(UnreadMentions), 0) AS UnreadMentions, UserId
		FROM ThreadMemberships
		LEFT JOIN Threads ON ThreadMemberships.PostId = Threads.PostId
		GROUP BY Threads.ChannelId, ThreadMemberships.UserId
	`
	updateMentionCountRootQuery := `
		UPDATE ChannelMembers INNER JOIN (` + mentionCountRootCTE + `) AS q ON
			q.ChannelId = ChannelMembers.ChannelId AND
			q.UserId=ChannelMembers.UserId AND
			ChannelMembers.MentionCount > 0
		SET MentionCountRoot = ChannelMembers.MentionCount - q.UnreadMentions
	`
	if sqlStore.DriverName() == model.DATABASE_DRIVER_POSTGRES {
		updateMentionCountRootQuery = `
			WITH q AS (` + mentionCountRootCTE + `)
			UPDATE channelmembers
			SET MentionCountRoot = ChannelMembers.MentionCount - q.UnreadMentions
			FROM q
			WHERE
				q.ChannelId = ChannelMembers.ChannelId AND
				q.UserId = ChannelMembers.UserId AND
				ChannelMembers.MentionCount > 0
		`
	}
	if _, err := sqlStore.GetMaster().ExecNoTimeout(updateMentionCountRootQuery); err != nil {
		mlog.Error("Error updating ChannelId in Threads table", mlog.Err(err))
	}
	sqlStore.CreateColumnIfNotExists("Channels", "TotalMsgCountRoot", "bigint", "bigint", "0")
	sqlStore.CreateColumnIfNotExistsNoDefault("Channels", "LastRootPostAt", "bigint", "bigint")
	defer sqlStore.RemoveColumnIfExists("Channels", "LastRootPostAt")

	sqlStore.CreateColumnIfNotExists("ChannelMembers", "MsgCountRoot", "bigint", "bigint", "0")
	sqlStore.AlterDefaultIfColumnExists("ChannelMembers", "MsgCountRoot", model.NewString("0"), model.NewString("0"))

	forceIndex := ""
	if sqlStore.DriverName() == model.DATABASE_DRIVER_MYSQL {
		forceIndex = "FORCE INDEX(idx_posts_channel_id_update_at)"
	}
	totalMsgCountRootCTE := `
		SELECT Channels.Id channelid, COALESCE(COUNT(*),0) newcount, COALESCE(MAX(Posts.CreateAt), 0) as lastpost
		FROM Channels
		LEFT JOIN Posts ` + forceIndex + ` ON Channels.Id = Posts.ChannelId
		WHERE Posts.RootId = ''
		GROUP BY Channels.Id
	`
	channelsCTE := "SELECT TotalMsgCountRoot, Id, LastRootPostAt from Channels"
	updateChannels := `
		WITH q AS (` + totalMsgCountRootCTE + `)
		UPDATE Channels SET TotalMsgCountRoot = q.newcount, LastRootPostAt=q.lastpost
		FROM q where q.channelid=Channels.Id;
	`
	updateChannelMembers := `
		WITH q as (` + channelsCTE + `)
		UPDATE ChannelMembers CM SET MsgCountRoot=TotalMsgCountRoot
		FROM q WHERE q.id=CM.ChannelId AND LastViewedAt >= q.lastrootpostat;
	`
	if sqlStore.DriverName() == model.DATABASE_DRIVER_MYSQL {
		updateChannels = `
			UPDATE Channels
			INNER Join (` + totalMsgCountRootCTE + `) as q
			ON q.channelid=Channels.Id
			SET TotalMsgCountRoot = q.newcount, LastRootPostAt=q.lastpost;
		`
		updateChannelMembers = `
			UPDATE ChannelMembers CM
			INNER JOIN (` + channelsCTE + `) as q
			ON q.id=CM.ChannelId and LastViewedAt >= q.lastrootpostat
			SET MsgCountRoot=TotalMsgCountRoot
			`
	}

	if !totalMsgCountRootExists {
		if _, err := sqlStore.GetMaster().ExecNoTimeout(updateChannels); err != nil {
			mlog.Error("Error updating Channels table", mlog.Err(err))
		}
	}
	if !msgCountRootExists {
		if _, err := sqlStore.GetMaster().ExecNoTimeout(updateChannelMembers); err != nil {
			mlog.Error("Error updating ChannelMembers table", mlog.Err(err))
		}
	}
}

func upgradeDatabaseToVersion537(sqlStore *SqlStore) {
	if shouldPerformUpgrade(sqlStore, Version5360, Version5370) {
		sqlStore.RemoveIndexIfExists("idx_posts_channel_id", "Posts")
		sqlStore.RemoveIndexIfExists("idx_channels_name", "Channels")
		sqlStore.RemoveIndexIfExists("idx_publicchannels_name", "PublicChannels")
		sqlStore.RemoveIndexIfExists("idx_channelmembers_channel_id", "ChannelMembers")
		sqlStore.RemoveIndexIfExists("idx_emoji_name", "Emoji")
		sqlStore.RemoveIndexIfExists("idx_oauthaccessdata_client_id", "OAuthAccessData")
		sqlStore.RemoveIndexIfExists("idx_oauthauthdata_client_id", "OAuthAuthData")
		sqlStore.RemoveIndexIfExists("idx_preferences_user_id", "Preferences")
		sqlStore.RemoveIndexIfExists("idx_notice_views_user_id", "ProductNoticeViewState")
		sqlStore.RemoveIndexIfExists("idx_notice_views_user_notice", "ProductNoticeViewState")
		sqlStore.RemoveIndexIfExists("idx_status_user_id", "Status")
		sqlStore.RemoveIndexIfExists("idx_teammembers_team_id", "TeamMembers")
		sqlStore.RemoveIndexIfExists("idx_teams_name", "Teams")
		sqlStore.RemoveIndexIfExists("idx_user_access_tokens_token", "UserAccessTokens")
		sqlStore.RemoveIndexIfExists("idx_user_terms_of_service_user_id", "UserTermsOfService")
		sqlStore.RemoveIndexIfExists("idx_users_email", "Users")
		sqlStore.RemoveIndexIfExists("idx_sharedchannelusers_user_id", "SharedChannelUsers")
		sqlStore.RemoveIndexIfExists("IDX_RetentionPolicies_DisplayName_Id", "RetentionPolicies")
		sqlStore.CreateIndexIfNotExists("IDX_RetentionPolicies_DisplayName", "RetentionPolicies", "DisplayName")

		sqlStore.CreateColumnIfNotExistsNoDefault("Status", "DNDEndTime", "bigint", "bigint")
		sqlStore.CreateColumnIfNotExistsNoDefault("Status", "PrevStatus", "VARCHAR(32)", "VARCHAR(32)")

		saveSchemaVersion(sqlStore, Version5370)
	}
}

func upgradeDatabaseToVersion538(sqlStore *SqlStore) {
	// TODO: uncomment when the time arrive to upgrade the DB for 5.38
	// if shouldPerformUpgrade(sqlStore, Version5370, Version5380) {
	fixCRTChannelMembershipCounts(sqlStore)
	fixCRTThreadCountsAndUnreads(sqlStore)

	// 	saveSchemaVersion(sqlStore, Version5380)
	// }
}

// fixCRTThreadCountsAndUnreads Marks threads as read for users where the last
// reply time of the thread is earlier than the time the user viewed the channel.
// Marking a thread means setting the mention count to zero and setting the
// last viewed at time of the the thread as the last viewed at time
// of the channel
func fixCRTThreadCountsAndUnreads(sqlStore *SqlStore) {
	threadMembershipsCTE := `
		SELECT PostId, UserId, ChannelMembers.LastViewedAt as CM_LastViewedAt, Threads.LastReplyAt
		FROM Threads
		INNER JOIN ChannelMembers on ChannelMembers.ChannelId = Threads.ChannelId
		WHERE Threads.LastReplyAt <= ChannelMembers.LastViewedAt
	`
	updateThreadMembershipQuery := `
		WITH q as (` + threadMembershipsCTE + `)
		UPDATE ThreadMemberships set LastViewed = q.CM_LastViewedAt, UnreadMentions = 0, LastUpdated = :Now
		FROM q WHERE ThreadMemberships.Postid = q.PostId AND ThreadMemberships.UserId = q.UserId
	`
	if sqlStore.DriverName() == model.DATABASE_DRIVER_MYSQL {
		updateThreadMembershipQuery = `
			UPDATE ThreadMemberships
			INNER JOIN (` + threadMembershipsCTE + `) as q
			ON ThreadMemberships.Postid = q.PostId AND ThreadMemberships.UserId = q.UserId
			SET LastViewed = q.CM_LastViewedAt, UnreadMentions = 0, LastUpdated = :Now
		`
	}

	if _, err := sqlStore.GetMaster().ExecNoTimeout(updateThreadMembershipQuery, map[string]interface{}{"Now": model.GetMillis()}); err != nil {
		mlog.Error("Error updating lastviewedat and unreadmentions of threadmemberships", mlog.Err(err))
	}
}

// fixCRTChannelMembershipCounts fixes the channel counts, i.e. the total message count,
// total root message count, mention count, and mention count in root messages for users
// who have viewed the channel after the last post in the channel
func fixCRTChannelMembershipCounts(sqlStore *SqlStore) {
	channelMembershipsCountsAndMentions := `
		UPDATE ChannelMembers
		SET MentionCount=0, MentionCountRoot=0, MsgCount=Channels.TotalMsgCount, MsgCountRoot=Channels.TotalMsgCountRoot, LastUpdateAt = :Now
		FROM Channels
		WHERE ChannelMembers.Channelid = Channels.Id AND ChannelMembers.LastViewedAt >= Channels.LastPostAt;
	`

	if sqlStore.DriverName() == model.DATABASE_DRIVER_MYSQL {
		channelMembershipsCountsAndMentions = `
			UPDATE ChannelMembers
			INNER JOIN Channels on Channels.Id = ChannelMembers.ChannelId
			SET MentionCount=0, MentionCountRoot=0, MsgCount=Channels.TotalMsgCount, MsgCountRoot=Channels.TotalMsgCountRoot, LastUpdateAt = :Now
			WHERE ChannelMembers.LastViewedAt >= Channels.LastPostAt;
		`
	}

	if _, err := sqlStore.GetMaster().ExecNoTimeout(channelMembershipsCountsAndMentions, map[string]interface{}{"Now": model.GetMillis()}); err != nil {
		mlog.Error("Error updating counts and unreads for channelmemberships", mlog.Err(err))
	}
}
