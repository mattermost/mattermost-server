// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitBotLocal() {
	api.BaseRoutes.Bot.Handle("", api.APILocal(getBot)).Methods("GET")
	api.BaseRoutes.Bot.Handle("", api.APILocal(patchBot)).Methods("PUT")
	api.BaseRoutes.Bot.Handle("/disable", api.APILocal(disableBot)).Methods("POST")
	api.BaseRoutes.Bot.Handle("/enable", api.APILocal(enableBot)).Methods("POST")
	api.BaseRoutes.Bot.Handle("/convert_to_user", api.APILocal(convertBotToUser)).Methods("POST")
	api.BaseRoutes.Bot.Handle("/assign/{user_id:[A-Za-z0-9]+}", api.APILocal(assignBot)).Methods("POST")

	api.BaseRoutes.Bots.Handle("", api.APILocal(getBots)).Methods("GET")
}
