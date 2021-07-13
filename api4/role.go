// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

var allowedPermissions = []string{
	model.PERMISSION_CREATE_TEAM.Id,
	model.PERMISSION_MANAGE_INCOMING_WEBHOOKS.Id,
	model.PERMISSION_MANAGE_OUTGOING_WEBHOOKS.Id,
	model.PERMISSION_MANAGE_SLASH_COMMANDS.Id,
	model.PERMISSION_MANAGE_OAUTH.Id,
	model.PERMISSION_MANAGE_SYSTEM_WIDE_OAUTH.Id,
	model.PERMISSION_CREATE_EMOJIS.Id,
	model.PERMISSION_DELETE_EMOJIS.Id,
	model.PERMISSION_EDIT_OTHERS_POSTS.Id,
}

var notAllowedPermissions = []string{
	model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_SYSTEM_ROLES.Id,
	model.PERMISSION_SYSCONSOLE_READ_USERMANAGEMENT_SYSTEM_ROLES.Id,
	model.PERMISSION_MANAGE_ROLES.Id,
}

func (api *API) InitRole() {
	api.BaseRoutes.Roles.Handle("", api.ApiSessionRequired(getAllRoles)).Methods("GET")
	api.BaseRoutes.Roles.Handle("/{role_id:[A-Za-z0-9]+}", api.ApiSessionRequiredTrustRequester(getRole)).Methods("GET")
	api.BaseRoutes.Roles.Handle("/name/{role_name:[a-z0-9_]+}", api.ApiSessionRequiredTrustRequester(getRoleByName)).Methods("GET")
	api.BaseRoutes.Roles.Handle("/names", api.ApiSessionRequiredTrustRequester(getRolesByNames)).Methods("POST")
	api.BaseRoutes.Roles.Handle("/{role_id:[A-Za-z0-9]+}/patch", api.ApiSessionRequired(patchRole)).Methods("PUT")
}

func getAllRoles(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.App.Session(), model.PERMISSION_MANAGE_SYSTEM) {
		c.SetPermissionError(model.PERMISSION_MANAGE_SYSTEM)
		return
	}

	roles, err := c.App.GetAllRoles()
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.RoleListToJson(roles)))
}

func getRole(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireRoleId()
	if c.Err != nil {
		return
	}

	role, err := c.App.GetRole(c.Params.RoleId)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(role.ToJson()))
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

	w.Write([]byte(role.ToJson()))
}

func getRolesByNames(c *Context, w http.ResponseWriter, r *http.Request) {
	rolenames := model.ArrayFromJson(r.Body)

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

	w.Write([]byte(model.RoleListToJson(roles)))
}

func patchRole(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireRoleId()
	if c.Err != nil {
		return
	}

	patch := model.RolePatchFromJson(r.Body)
	if patch == nil {
		c.SetInvalidParam("role")
		return
	}

	auditRec := c.MakeAuditRecord("patchRole", audit.Fail)
	defer c.LogAuditRec(auditRec)

	oldRole, err := c.App.GetRole(c.Params.RoleId)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("role", oldRole)

	// manage_system permission is required to patch system_admin
	requiredPermission := model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_PERMISSIONS
	specialProtectedSystemRoles := append(model.NewSystemRoleIDs, model.SYSTEM_ADMIN_ROLE_ID)
	for _, roleID := range specialProtectedSystemRoles {
		if oldRole.Name == roleID {
			requiredPermission = model.PERMISSION_MANAGE_SYSTEM
		}
	}
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), requiredPermission) {
		c.SetPermissionError(requiredPermission)
		return
	}

	isGuest := oldRole.Name == model.SYSTEM_GUEST_ROLE_ID || oldRole.Name == model.TEAM_GUEST_ROLE_ID || oldRole.Name == model.CHANNEL_GUEST_ROLE_ID
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

	if oldRole.Name == model.TEAM_ADMIN_ROLE_ID || oldRole.Name == model.CHANNEL_ADMIN_ROLE_ID || oldRole.Name == model.SYSTEM_USER_ROLE_ID || oldRole.Name == model.TEAM_USER_ROLE_ID || oldRole.Name == model.CHANNEL_USER_ROLE_ID || oldRole.Name == model.SYSTEM_GUEST_ROLE_ID || oldRole.Name == model.TEAM_GUEST_ROLE_ID || oldRole.Name == model.CHANNEL_GUEST_ROLE_ID {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_PERMISSIONS) {
			c.SetPermissionError(model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_PERMISSIONS)
			return
		}
	} else {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_SYSTEM_ROLES) {
			c.SetPermissionError(model.PERMISSION_SYSCONSOLE_WRITE_USERMANAGEMENT_SYSTEM_ROLES)
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

	w.Write([]byte(role.ToJson()))
}
