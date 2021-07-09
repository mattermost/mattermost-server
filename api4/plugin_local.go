// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitPluginLocal() {
	api.BaseRoutes.Plugins.Handle("", api.APILocal(uploadPlugin)).Methods("POST")
	api.BaseRoutes.Plugins.Handle("", api.APILocal(getPlugins)).Methods("GET")
	api.BaseRoutes.Plugins.Handle("/install_from_url", api.APILocal(installPluginFromURL)).Methods("POST")
	api.BaseRoutes.Plugin.Handle("", api.APILocal(removePlugin)).Methods("DELETE")
	api.BaseRoutes.Plugin.Handle("/enable", api.APILocal(enablePlugin)).Methods("POST")
	api.BaseRoutes.Plugin.Handle("/disable", api.APILocal(disablePlugin)).Methods("POST")
	api.BaseRoutes.Plugins.Handle("/marketplace", api.APILocal(installMarketplacePlugin)).Methods("POST")
	api.BaseRoutes.Plugins.Handle("/marketplace", api.APILocal(getMarketplacePlugins)).Methods("GET")
}
