// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

var allowedPermissions = []string{
	model.PermissionCreateTeam.ID,
	model.PermissionManageIncomingWebhooks.ID,
	model.PermissionManageOutgoingWebhooks.ID,
	model.PermissionManageSlashCommands.ID,
	model.PermissionManageOAuth.ID,
	model.PermissionManageSystemWideOAuth.ID,
	model.PermissionCreateEmojis.ID,
	model.PermissionDeleteEmojis.ID,
	model.PermissionEditOthersPosts.ID,
}

var notAllowedPermissions = []string{
	model.PermissionSysconsoleWriteUserManagementSystemRoles.ID,
	model.PermissionSysconsoleReadUserManagementSystemRoles.ID,
	model.PermissionManageRoles.ID,
}

func (api *API) InitRole() {
	api.BaseRoutes.Roles.Handle("/{role_id:[A-Za-z0-9]+}", api.APISessionRequiredTrustRequester(getRole)).Methods("GET")
	api.BaseRoutes.Roles.Handle("/name/{role_name:[a-z0-9_]+}", api.APISessionRequiredTrustRequester(getRoleByName)).Methods("GET")
	api.BaseRoutes.Roles.Handle("/names", api.APISessionRequiredTrustRequester(getRolesByNames)).Methods("POST")
	api.BaseRoutes.Roles.Handle("/{role_id:[A-Za-z0-9]+}/patch", api.APISessionRequired(patchRole)).Methods("PUT")
}

func getRole(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireRoleID()
	if c.Err != nil {
		return
	}

	role, err := c.App.GetRole(c.Params.RoleID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(role.ToJSON()))
}

func getRoleByName(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireRoleName()
	if c.Err != nil {
		return
	}

	role, err := c.App.GetRoleByName(r.Context(), c.Params.RoleName)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(role.ToJSON()))
}

func getRolesByNames(c *Context, w http.ResponseWriter, r *http.Request) {
	rolenames := model.ArrayFromJSON(r.Body)

	if len(rolenames) == 0 {
		c.SetInvalidParam("rolenames")
		return
	}

	cleanedRoleNames, valid := model.CleanRoleNames(rolenames)
	if !valid {
		c.SetInvalidParam("rolename")
		return
	}

	roles, err := c.App.GetRolesByNames(cleanedRoleNames)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.RoleListToJSON(roles)))
}

func patchRole(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireRoleID()
	if c.Err != nil {
		return
	}

	patch := model.RolePatchFromJSON(r.Body)
	if patch == nil {
		c.SetInvalidParam("role")
		return
	}

	auditRec := c.MakeAuditRecord("patchRole", audit.Fail)
	defer c.LogAuditRec(auditRec)

	oldRole, err := c.App.GetRole(c.Params.RoleID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("role", oldRole)

	// manage_system permission is required to patch system_admin
	requiredPermission := model.PermissionSysconsoleWriteUserManagementPermissions
	specialProtectedSystemRoles := append(model.NewSystemRoleIDs, model.SystemAdminRoleID)
	for _, roleID := range specialProtectedSystemRoles {
		if oldRole.Name == roleID {
			requiredPermission = model.PermissionManageSystem
		}
	}
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), requiredPermission) {
		c.SetPermissionError(requiredPermission)
		return
	}

	isGuest := oldRole.Name == model.SystemGuestRoleID || oldRole.Name == model.TeamGuestRoleID || oldRole.Name == model.ChannelGuestRoleID
	if c.App.Srv().License() == nil && patch.Permissions != nil {
		if isGuest {
			c.Err = model.NewAppError("Api4.PatchRoles", "api.roles.patch_roles.license.error", nil, "", http.StatusNotImplemented)
			return
		}

		changedPermissions := model.PermissionsChangedByPatch(oldRole, patch)
		for _, permission := range changedPermissions {
			allowed := false
			for _, allowedPermission := range allowedPermissions {
				if permission == allowedPermission {
					allowed = true
				}
			}

			if !allowed {
				c.Err = model.NewAppError("Api4.PatchRoles", "api.roles.patch_roles.license.error", nil, "", http.StatusNotImplemented)
				return
			}
		}
	}

	if patch.Permissions != nil {
		deltaPermissions := model.PermissionsChangedByPatch(oldRole, patch)

		for _, permission := range deltaPermissions {
			notAllowed := false
			for _, notAllowedPermission := range notAllowedPermissions {
				if permission == notAllowedPermission {
					notAllowed = true
				}
			}

			if notAllowed {
				c.Err = model.NewAppError("Api4.PatchRoles", "api.roles.patch_roles.not_allowed_permission.error", nil, "Cannot add or remove permission: "+permission, http.StatusNotImplemented)
				return
			}
		}

		*patch.Permissions = model.UniqueStrings(*patch.Permissions)
	}

	if c.App.Srv().License() != nil && isGuest && !*c.App.Srv().License().Features.GuestAccountsPermissions {
		c.Err = model.NewAppError("Api4.PatchRoles", "api.roles.patch_roles.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	if oldRole.Name == model.TeamAdminRoleID || oldRole.Name == model.ChannelAdminRoleID || oldRole.Name == model.SystemUserRoleID || oldRole.Name == model.TeamUserRoleID || oldRole.Name == model.ChannelUserRoleID || oldRole.Name == model.SystemGuestRoleID || oldRole.Name == model.TeamGuestRoleID || oldRole.Name == model.ChannelGuestRoleID {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementPermissions) {
			c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementPermissions)
			return
		}
	} else {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementSystemRoles) {
			c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementSystemRoles)
			return
		}
	}

	role, err := c.App.PatchRole(oldRole, patch)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("patch", role)
	c.LogAudit("")

	w.Write([]byte(role.ToJSON()))
}
