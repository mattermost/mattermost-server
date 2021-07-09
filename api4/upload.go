// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitUpload() {
	api.BaseRoutes.Uploads.Handle("", api.ApiSessionRequired(createUpload)).Methods("POST")
	api.BaseRoutes.Upload.Handle("", api.ApiSessionRequired(getUpload)).Methods("GET")
	api.BaseRoutes.Upload.Handle("", api.ApiSessionRequired(uploadData)).Methods("POST")
}

func createUpload(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().FileSettings.EnableFileAttachments {
		c.Err = model.NewAppError("createUpload",
			"api.file.attachments.disabled.app_error",
			nil, "", http.StatusNotImplemented)
		return
	}

	us := model.UploadSessionFromJSON(r.Body)
	if us == nil {
		c.SetInvalidParam("upload")
		return
	}

	// these are not supported for client uploads; shared channels only.
	us.RemoteID = ""
	us.ReqFileID = ""

	auditRec := c.MakeAuditRecord("createUpload", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("upload", us)

	if us.Type == model.UploadTypeImport {
		if !c.IsSystemAdmin() {
			c.SetPermissionError(model.PermissionManageSystem)
			return
		}
	} else {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), us.ChannelID, model.PermissionUploadFile) {
			c.SetPermissionError(model.PermissionUploadFile)
			return
		}
		us.Type = model.UploadTypeAttachment
	}

	us.ID = model.NewID()
	if c.AppContext.Session().UserID != "" {
		us.UserID = c.AppContext.Session().UserID
	}
	us, err := c.App.CreateUploadSession(us)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(us.ToJSON()))
}

func getUpload(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUploadID()
	if c.Err != nil {
		return
	}

	us, err := c.App.GetUploadSession(c.Params.UploadID)
	if err != nil {
		c.Err = err
		return
	}

	if us.UserID != c.AppContext.Session().UserID && !c.IsSystemAdmin() {
		c.Err = model.NewAppError("getUpload", "api.upload.get_upload.forbidden.app_error", nil, "", http.StatusForbidden)
		return
	}

	w.Write([]byte(us.ToJSON()))
}

func uploadData(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().FileSettings.EnableFileAttachments {
		c.Err = model.NewAppError("uploadData", "api.file.attachments.disabled.app_error",
			nil, "", http.StatusNotImplemented)
		return
	}

	c.RequireUploadID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("uploadData", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("upload_id", c.Params.UploadID)

	us, err := c.App.GetUploadSession(c.Params.UploadID)
	if err != nil {
		c.Err = err
		return
	}

	if us.Type == model.UploadTypeImport {
		if !c.IsSystemAdmin() {
			c.SetPermissionError(model.PermissionManageSystem)
			return
		}
	} else {
		if us.UserID != c.AppContext.Session().UserID || !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), us.ChannelID, model.PermissionUploadFile) {
			c.SetPermissionError(model.PermissionUploadFile)
			return
		}
	}

	info, err := doUploadData(c, us, r)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	if info == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Write([]byte(info.ToJSON()))
}

func doUploadData(c *Context, us *model.UploadSession, r *http.Request) (*model.FileInfo, *model.AppError) {
	boundary, parseErr := parseMultipartRequestHeader(r)
	if parseErr != nil && !errors.Is(parseErr, http.ErrNotMultipart) {
		return nil, model.NewAppError("uploadData", "api.upload.upload_data.invalid_content_type",
			nil, parseErr.Error(), http.StatusBadRequest)
	}

	var rd io.Reader
	if boundary != "" {
		mr := multipart.NewReader(r.Body, boundary)
		p, partErr := mr.NextPart()
		if partErr != nil {
			return nil, model.NewAppError("uploadData", "api.upload.upload_data.multipart_error",
				nil, partErr.Error(), http.StatusBadRequest)
		}
		rd = p
	} else {
		if r.ContentLength > (us.FileSize - us.FileOffset) {
			return nil, model.NewAppError("uploadData", "api.upload.upload_data.invalid_content_length",
				nil, "", http.StatusBadRequest)
		}
		rd = r.Body
	}

	return c.App.UploadData(c.AppContext, us, rd)
}
