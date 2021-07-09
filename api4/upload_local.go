// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

func (api *API) InitUploadLocal() {
	api.BaseRoutes.Uploads.Handle("", api.APILocal(createUpload)).Methods("POST")
	api.BaseRoutes.Upload.Handle("", api.APILocal(getUpload)).Methods("GET")
	api.BaseRoutes.Upload.Handle("", api.APILocal(uploadData)).Methods("POST")
}
