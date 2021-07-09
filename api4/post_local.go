// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitPostLocal() {
	api.BaseRoutes.Post.Handle("", api.APILocal(getPost)).Methods("GET")

	api.BaseRoutes.PostsForChannel.Handle("", api.APILocal(getPostsForChannel)).Methods("GET")
}
