// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetRole(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	role := &model.Role{
		Name:          model.NewID(),
		DisplayName:   model.NewID(),
		Description:   model.NewID(),
		Permissions:   []string{"manage_system", "create_public_channel"},
		SchemeManaged: true,
	}

	role, err := th.App.Srv().Store.Role().Save(role)
	require.NoError(t, err)
	defer th.App.Srv().Store.Job().Delete(role.ID)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		received, resp := client.GetRole(role.ID)
		CheckNoError(t, resp)

		assert.Equal(t, received.ID, role.ID)
		assert.Equal(t, received.Name, role.Name)
		assert.Equal(t, received.DisplayName, role.DisplayName)
		assert.Equal(t, received.Description, role.Description)
		assert.EqualValues(t, received.Permissions, role.Permissions)
		assert.Equal(t, received.SchemeManaged, role.SchemeManaged)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.GetRole("1234")
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetRole(model.NewID())
		CheckNotFoundStatus(t, resp)
	})
}

func TestGetRoleByName(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	role := &model.Role{
		Name:          model.NewID(),
		DisplayName:   model.NewID(),
		Description:   model.NewID(),
		Permissions:   []string{"manage_system", "create_public_channel"},
		SchemeManaged: true,
	}

	role, err := th.App.Srv().Store.Role().Save(role)
	assert.NoError(t, err)
	defer th.App.Srv().Store.Job().Delete(role.ID)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		received, resp := client.GetRoleByName(role.Name)
		CheckNoError(t, resp)

		assert.Equal(t, received.ID, role.ID)
		assert.Equal(t, received.Name, role.Name)
		assert.Equal(t, received.DisplayName, role.DisplayName)
		assert.Equal(t, received.Description, role.Description)
		assert.EqualValues(t, received.Permissions, role.Permissions)
		assert.Equal(t, received.SchemeManaged, role.SchemeManaged)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.GetRoleByName(strings.Repeat("abcdefghij", 10))
		CheckBadRequestStatus(t, resp)

		_, resp = client.GetRoleByName(model.NewID())
		CheckNotFoundStatus(t, resp)
	})
}

func TestGetRolesByNames(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	role1 := &model.Role{
		Name:          model.NewID(),
		DisplayName:   model.NewID(),
		Description:   model.NewID(),
		Permissions:   []string{"manage_system", "create_public_channel"},
		SchemeManaged: true,
	}
	role2 := &model.Role{
		Name:          model.NewID(),
		DisplayName:   model.NewID(),
		Description:   model.NewID(),
		Permissions:   []string{"manage_system", "delete_private_channel"},
		SchemeManaged: true,
	}
	role3 := &model.Role{
		Name:          model.NewID(),
		DisplayName:   model.NewID(),
		Description:   model.NewID(),
		Permissions:   []string{"manage_system", "manage_public_channel_properties"},
		SchemeManaged: true,
	}

	role1, err := th.App.Srv().Store.Role().Save(role1)
	assert.NoError(t, err)
	defer th.App.Srv().Store.Job().Delete(role1.ID)

	role2, err = th.App.Srv().Store.Role().Save(role2)
	assert.NoError(t, err)
	defer th.App.Srv().Store.Job().Delete(role2.ID)

	role3, err = th.App.Srv().Store.Role().Save(role3)
	assert.NoError(t, err)
	defer th.App.Srv().Store.Job().Delete(role3.ID)

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		// Check all three roles can be found.
		received, resp := client.GetRolesByNames([]string{role1.Name, role2.Name, role3.Name})
		CheckNoError(t, resp)

		assert.Contains(t, received, role1)
		assert.Contains(t, received, role2)
		assert.Contains(t, received, role3)

		// Check a list of non-existent roles.
		_, resp = client.GetRolesByNames([]string{model.NewID(), model.NewID()})
		CheckNoError(t, resp)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		// Empty list should error.
		_, resp := client.GetRolesByNames([]string{})
		CheckBadRequestStatus(t, resp)
	})

	th.TestForAllClients(t, func(t *testing.T, client *model.Client4) {
		// Invalid role name should error.
		_, resp := client.GetRolesByNames([]string{model.NewID(), model.NewID(), "!!!!!!"})
		CheckBadRequestStatus(t, resp)

		// Empty/whitespace rolenames should be ignored.
		_, resp = client.GetRolesByNames([]string{model.NewID(), model.NewID(), "", "    "})
		CheckNoError(t, resp)
	})

}

func TestPatchRole(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	role := &model.Role{
		Name:          model.NewID(),
		DisplayName:   model.NewID(),
		Description:   model.NewID(),
		Permissions:   []string{"manage_system", "create_public_channel", "manage_slash_commands"},
		SchemeManaged: true,
	}

	role, err := th.App.Srv().Store.Role().Save(role)
	assert.NoError(t, err)
	defer th.App.Srv().Store.Job().Delete(role.ID)

	patch := &model.RolePatch{
		Permissions: &[]string{"manage_system", "create_public_channel", "manage_incoming_webhooks", "manage_outgoing_webhooks"},
	}

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {

		// Cannot edit a system admin
		adminRole, err := th.App.Srv().Store.Role().GetByName(context.Background(), "system_admin")
		assert.NoError(t, err)
		defer th.App.Srv().Store.Job().Delete(adminRole.ID)

		_, resp := client.PatchRole(adminRole.ID, patch)
		CheckNotImplementedStatus(t, resp)

		// Cannot give other roles read / write to system roles or manage roles because only system admin can do these actions
		systemManager, err := th.App.Srv().Store.Role().GetByName(context.Background(), "system_manager")
		assert.NoError(t, err)
		defer th.App.Srv().Store.Job().Delete(systemManager.ID)

		patchWriteSystemRoles := &model.RolePatch{
			Permissions: &[]string{model.PermissionSysconsoleWriteUserManagementSystemRoles.ID},
		}

		_, resp = client.PatchRole(systemManager.ID, patchWriteSystemRoles)
		CheckNotImplementedStatus(t, resp)

		patchReadSystemRoles := &model.RolePatch{
			Permissions: &[]string{model.PermissionSysconsoleReadUserManagementSystemRoles.ID},
		}

		_, resp = client.PatchRole(systemManager.ID, patchReadSystemRoles)
		CheckNotImplementedStatus(t, resp)

		patchManageRoles := &model.RolePatch{
			Permissions: &[]string{model.PermissionManageRoles.ID},
		}

		_, resp = client.PatchRole(systemManager.ID, patchManageRoles)
		CheckNotImplementedStatus(t, resp)
	})

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		received, resp := client.PatchRole(role.ID, patch)
		CheckNoError(t, resp)

		assert.Equal(t, received.ID, role.ID)
		assert.Equal(t, received.Name, role.Name)
		assert.Equal(t, received.DisplayName, role.DisplayName)
		assert.Equal(t, received.Description, role.Description)
		assert.EqualValues(t, received.Permissions, []string{"manage_system", "create_public_channel", "manage_incoming_webhooks", "manage_outgoing_webhooks"})
		assert.Equal(t, received.SchemeManaged, role.SchemeManaged)

		// Check a no-op patch succeeds.
		_, resp = client.PatchRole(role.ID, patch)
		CheckNoError(t, resp)

		_, resp = client.PatchRole("junk", patch)
		CheckBadRequestStatus(t, resp)
	})

	_, resp := th.Client.PatchRole(model.NewID(), patch)
	CheckNotFoundStatus(t, resp)

	_, resp = th.Client.PatchRole(role.ID, patch)
	CheckForbiddenStatus(t, resp)

	// Check a change that the license would not allow.
	patch = &model.RolePatch{
		Permissions: &[]string{"manage_system", "manage_incoming_webhooks", "manage_outgoing_webhooks"},
	}

	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		_, resp := client.PatchRole(role.ID, patch)
		CheckNotImplementedStatus(t, resp)
	})

	// Add a license.
	license := model.NewTestLicense()
	license.Features.GuestAccountsPermissions = model.NewBool(false)
	th.App.Srv().SetLicense(license)

	// Try again, should succeed
	th.TestForSystemAdminAndLocal(t, func(t *testing.T, client *model.Client4) {
		received, resp := client.PatchRole(role.ID, patch)
		CheckNoError(t, resp)

		assert.Equal(t, received.ID, role.ID)
		assert.Equal(t, received.Name, role.Name)
		assert.Equal(t, received.DisplayName, role.DisplayName)
		assert.Equal(t, received.Description, role.Description)
		assert.EqualValues(t, received.Permissions, []string{"manage_system", "manage_incoming_webhooks", "manage_outgoing_webhooks"})
		assert.Equal(t, received.SchemeManaged, role.SchemeManaged)

		t.Run("Check guest permissions editing without E20 license", func(t *testing.T) {
			license := model.NewTestLicense()
			license.Features.GuestAccountsPermissions = model.NewBool(false)
			th.App.Srv().SetLicense(license)

			guestRole, err := th.App.Srv().Store.Role().GetByName(context.Background(), "system_guest")
			require.NoError(t, err)
			received, resp = client.PatchRole(guestRole.ID, patch)
			CheckNotImplementedStatus(t, resp)
		})

		t.Run("Check guest permissions editing with E20 license", func(t *testing.T) {
			license := model.NewTestLicense()
			license.Features.GuestAccountsPermissions = model.NewBool(true)
			th.App.Srv().SetLicense(license)
			guestRole, err := th.App.Srv().Store.Role().GetByName(context.Background(), "system_guest")
			require.NoError(t, err)
			_, resp = client.PatchRole(guestRole.ID, patch)
			CheckNoError(t, resp)
		})
	})
}
