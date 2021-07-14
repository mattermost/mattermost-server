// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitPostLocal() {
	api.BaseRoutes.Post.Handle("", api.ApiLocal(getPost)).Methods("GET")
	api.BaseRoutes.Posts.Handle("/ids", api.ApiLocal(getPosts)).Methods("POST")

	api.BaseRoutes.PostsForChannel.Handle("", api.ApiLocal(getPostsForChannel)).Methods("GET")
}
