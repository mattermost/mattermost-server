// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
)

func TestTeamStore(t *testing.T) {
	StoreTest(t, storetest.TestTeamStore)
}

func TestTeamStoreInternalDataTypes(t *testing.T) {
	t.Run("NewTeamMemberFromModel", func(t *testing.T) { testNewTeamMemberFromModel(t) })
	t.Run("TeamMemberWithSchemeRolesToModel", func(t *testing.T) { testTeamMemberWithSchemeRolesToModel(t) })
}

func testNewTeamMemberFromModel(t *testing.T) {
	m := model.TeamMember{
		TeamID:        model.NewID(),
		UserID:        model.NewID(),
		Roles:         "team_user team_admin custom_role",
		DeleteAt:      12345,
		SchemeGuest:   false,
		SchemeUser:    true,
		SchemeAdmin:   true,
		ExplicitRoles: "custom_role",
	}

	db := NewTeamMemberFromModel(&m)

	assert.Equal(t, m.TeamID, db.TeamID)
	assert.Equal(t, m.UserID, db.UserID)
	assert.Equal(t, m.DeleteAt, db.DeleteAt)
	assert.Equal(t, true, db.SchemeGuest.Valid)
	assert.Equal(t, true, db.SchemeUser.Valid)
	assert.Equal(t, true, db.SchemeAdmin.Valid)
	assert.Equal(t, m.SchemeGuest, db.SchemeGuest.Bool)
	assert.Equal(t, m.SchemeUser, db.SchemeUser.Bool)
	assert.Equal(t, m.SchemeAdmin, db.SchemeAdmin.Bool)
	assert.Equal(t, m.ExplicitRoles, db.Roles)
}

func testTeamMemberWithSchemeRolesToModel(t *testing.T) {
	// Test all the non-role-related properties here.
	t.Run("BasicProperties", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			TeamID:                     model.NewID(),
			UserID:                     model.NewID(),
			Roles:                      "custom_role",
			DeleteAt:                   12345,
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: true},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, db.TeamID, m.TeamID)
		assert.Equal(t, db.UserID, m.UserID)
		assert.Equal(t, "custom_role team_user team_admin", m.Roles)
		assert.Equal(t, db.DeleteAt, m.DeleteAt)
		assert.Equal(t, db.SchemeGuest.Bool, m.SchemeGuest)
		assert.Equal(t, db.SchemeUser.Bool, m.SchemeUser)
		assert.Equal(t, db.SchemeAdmin.Bool, m.SchemeAdmin)
		assert.Equal(t, db.Roles, m.ExplicitRoles)
	})

	// Example data *before* the Phase 2 migration has taken place.
	t.Run("Unmigrated_NoScheme_User", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "team_user",
			SchemeGuest:                sql.NullBool{Valid: false, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: false, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: false, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "team_user", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	t.Run("Unmigrated_NoScheme_Admin", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "team_user team_admin",
			SchemeGuest:                sql.NullBool{Valid: false, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: false, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: false, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "team_user team_admin", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, true, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	t.Run("Unmigrated_NoScheme_CustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: false, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: false, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: false, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Unmigrated_NoScheme_UserAndCustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "team_user custom_role",
			SchemeGuest:                sql.NullBool{Valid: false, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: false, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: false, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role team_user", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Unmigrated_NoScheme_AdminAndCustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "team_user team_admin custom_role",
			SchemeGuest:                sql.NullBool{Valid: false, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: false, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: false, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role team_user team_admin", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, true, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Unmigrated_NoScheme_NoRoles", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: false, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: false, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: false, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	// Example data *after* the Phase 2 migration has taken place.
	t.Run("Migrated_NoScheme_User", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "team_user", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	t.Run("Migrated_NoScheme_Admin", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: true},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "team_user team_admin", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, true, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	t.Run("Migrated_NoScheme_Guest", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: true},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "team_guest", m.Roles)
		assert.Equal(t, true, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	t.Run("Migrated_NoScheme_CustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Migrated_NoScheme_UserAndCustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role team_user", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Migrated_NoScheme_AdminAndCustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: true},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role team_user team_admin", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, true, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Migrated_NoScheme_GuestAndCustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: true},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role team_guest", m.Roles)
		assert.Equal(t, true, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Migrated_NoScheme_NoRoles", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: false},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: false},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: false},
		}

		m := db.ToModel()

		assert.Equal(t, "", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	// Example data with a team scheme.
	t.Run("Migrated_TeamScheme_User", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: true, String: "tscheme_guest"},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: true, String: "tscheme_user"},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: true, String: "tscheme_admin"},
		}

		m := db.ToModel()

		assert.Equal(t, "tscheme_user", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	t.Run("Migrated_TeamScheme_Admin", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: true},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: true, String: "tscheme_guest"},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: true, String: "tscheme_user"},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: true, String: "tscheme_admin"},
		}

		m := db.ToModel()

		assert.Equal(t, "tscheme_user tscheme_admin", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, true, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	t.Run("Migrated_TeamScheme_Guest", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: true},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: true, String: "tscheme_guest"},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: true, String: "tscheme_user"},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: true, String: "tscheme_admin"},
		}

		m := db.ToModel()

		assert.Equal(t, "tscheme_guest", m.Roles)
		assert.Equal(t, true, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})

	t.Run("Migrated_TeamScheme_CustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: true, String: "tscheme_guest"},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: true, String: "tscheme_user"},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: true, String: "tscheme_admin"},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Migrated_TeamScheme_UserAndCustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: true, String: "tscheme_guest"},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: true, String: "tscheme_user"},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: true, String: "tscheme_admin"},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role tscheme_user", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Migrated_TeamScheme_AdminAndCustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: true},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: true},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: true, String: "tscheme_guest"},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: true, String: "tscheme_user"},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: true, String: "tscheme_admin"},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role tscheme_user tscheme_admin", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, true, m.SchemeUser)
		assert.Equal(t, true, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Migrated_TeamScheme_GuestAndCustomRole", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "custom_role",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: true},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: true, String: "tscheme_guest"},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: true, String: "tscheme_user"},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: true, String: "tscheme_admin"},
		}

		m := db.ToModel()

		assert.Equal(t, "custom_role tscheme_guest", m.Roles)
		assert.Equal(t, true, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "custom_role", m.ExplicitRoles)
	})

	t.Run("Migrated_TeamScheme_NoRoles", func(t *testing.T) {
		db := teamMemberWithSchemeRoles{
			Roles:                      "",
			SchemeGuest:                sql.NullBool{Valid: true, Bool: false},
			SchemeUser:                 sql.NullBool{Valid: true, Bool: false},
			SchemeAdmin:                sql.NullBool{Valid: true, Bool: false},
			TeamSchemeDefaultGuestRole: sql.NullString{Valid: true, String: "tscheme_guest"},
			TeamSchemeDefaultUserRole:  sql.NullString{Valid: true, String: "tscheme_user"},
			TeamSchemeDefaultAdminRole: sql.NullString{Valid: true, String: "tscheme_admin"},
		}

		m := db.ToModel()

		assert.Equal(t, "", m.Roles)
		assert.Equal(t, false, m.SchemeGuest)
		assert.Equal(t, false, m.SchemeUser)
		assert.Equal(t, false, m.SchemeAdmin)
		assert.Equal(t, "", m.ExplicitRoles)
	})
}
