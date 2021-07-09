// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitBrand() {
	api.BaseRoutes.Brand.Handle("/image", api.APIHandlerTrustRequester(getBrandImage)).Methods("GET")
	api.BaseRoutes.Brand.Handle("/image", api.APISessionRequired(uploadBrandImage)).Methods("POST")
	api.BaseRoutes.Brand.Handle("/image", api.APISessionRequired(deleteBrandImage)).Methods("DELETE")
}

func getBrandImage(c *Context, w http.ResponseWriter, r *http.Request) {
	// No permission check required

	img, err := c.App.GetBrandImage()
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write(nil)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(img)
}

func uploadBrandImage(c *Context, w http.ResponseWriter, r *http.Request) {
	defer io.Copy(ioutil.Discard, r.Body)

	if r.ContentLength > *c.App.Config().FileSettings.MaxFileSize {
		c.Err = model.NewAppError("uploadBrandImage", "api.admin.upload_brand_image.too_large.app_error", nil, "", http.StatusRequestEntityTooLarge)
		return
	}

	if err := r.ParseMultipartForm(*c.App.Config().FileSettings.MaxFileSize); err != nil {
		c.Err = model.NewAppError("uploadBrandImage", "api.admin.upload_brand_image.parse.app_error", nil, "", http.StatusBadRequest)
		return
	}

	m := r.MultipartForm

	imageArray, ok := m.File["image"]
	if !ok {
		c.Err = model.NewAppError("uploadBrandImage", "api.admin.upload_brand_image.no_file.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if len(imageArray) <= 0 {
		c.Err = model.NewAppError("uploadBrandImage", "api.admin.upload_brand_image.array.app_error", nil, "", http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord("uploadBrandImage", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionEditBrand) {
		c.SetPermissionError(model.PermissionEditBrand)
		return
	}

	if err := c.App.SaveBrandImage(imageArray[0]); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	w.WriteHeader(http.StatusCreated)
	ReturnStatusOK(w)
}

func deleteBrandImage(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord("deleteBrandImage", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionEditBrand) {
		c.SetPermissionError(model.PermissionEditBrand)
		return
	}

	if err := c.App.DeleteBrandImage(); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	ReturnStatusOK(w)
}
