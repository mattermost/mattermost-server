// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitGroupLocal() {
	api.BaseRoutes.Channels.Handle("/{channel_id:[A-Za-z0-9]+}/groups", api.APILocal(getGroupsByChannel)).Methods("GET")
	api.BaseRoutes.Teams.Handle("/{team_id:[A-Za-z0-9]+}/groups", api.APILocal(getGroupsByTeam)).Methods("GET")
}
