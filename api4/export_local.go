// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitExportLocal() {
	api.BaseRoutes.Exports.Handle("", api.APILocal(listExports)).Methods("GET")
	api.BaseRoutes.Export.Handle("", api.APILocal(deleteExport)).Methods("DELETE")
	api.BaseRoutes.Export.Handle("", api.APILocal(downloadExport)).Methods("GET")
}
