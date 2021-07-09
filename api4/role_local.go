// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitRoleLocal() {
	api.BaseRoutes.Roles.Handle("/{role_id:[A-Za-z0-9]+}", api.APILocal(getRole)).Methods("GET")
	api.BaseRoutes.Roles.Handle("/name/{role_name:[a-z0-9_]+}", api.APILocal(getRoleByName)).Methods("GET")
	api.BaseRoutes.Roles.Handle("/names", api.APILocal(getRolesByNames)).Methods("POST")
	api.BaseRoutes.Roles.Handle("/{role_id:[A-Za-z0-9]+}/patch", api.APILocal(patchRole)).Methods("PUT")
}
