// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"crypto/subtle"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

const (
	FileTeamID = "noteam"

	PreviewImageType   = "image/jpeg"
	ThumbnailImageType = "image/jpeg"
)

var UnsafeContentTypes = [...]string{
	"application/javascript",
	"application/ecmascript",
	"text/javascript",
	"text/ecmascript",
	"application/x-javascript",
	"text/html",
}

var MediaContentTypes = [...]string{
	"image/jpeg",
	"image/png",
	"image/bmp",
	"image/gif",
	"image/tiff",
	"video/avi",
	"video/mpeg",
	"video/mp4",
	"audio/mpeg",
	"audio/wav",
}

const maxMultipartFormDataBytes = 10 * 1024 // 10Kb

func (api *API) InitFile() {
	api.BaseRoutes.Files.Handle("", api.ApiSessionRequired(uploadFileStream)).Methods("POST")
	api.BaseRoutes.File.Handle("", api.ApiSessionRequiredTrustRequester(getFile)).Methods("GET")
	api.BaseRoutes.File.Handle("/thumbnail", api.ApiSessionRequiredTrustRequester(getFileThumbnail)).Methods("GET")
	api.BaseRoutes.File.Handle("/link", api.ApiSessionRequired(getFileLink)).Methods("GET")
	api.BaseRoutes.File.Handle("/preview", api.ApiSessionRequiredTrustRequester(getFilePreview)).Methods("GET")
	api.BaseRoutes.File.Handle("/info", api.ApiSessionRequired(getFileInfo)).Methods("GET")

	api.BaseRoutes.Team.Handle("/files/search", api.ApiSessionRequiredDisableWhenBusy(searchFiles)).Methods("POST")

	api.BaseRoutes.PublicFile.Handle("", api.ApiHandler(getPublicFile)).Methods("GET")

}

func parseMultipartRequestHeader(req *http.Request) (boundary string, err error) {
	v := req.Header.Get("Content-Type")
	if v == "" {
		return "", http.ErrNotMultipart
	}
	d, params, err := mime.ParseMediaType(v)
	if err != nil || d != "multipart/form-data" {
		return "", http.ErrNotMultipart
	}
	boundary, ok := params["boundary"]
	if !ok {
		return "", http.ErrMissingBoundary
	}

	return boundary, nil
}

func multipartReader(req *http.Request, stream io.Reader) (*multipart.Reader, error) {
	boundary, err := parseMultipartRequestHeader(req)
	if err != nil {
		return nil, err
	}

	if stream != nil {
		return multipart.NewReader(stream, boundary), nil
	}

	return multipart.NewReader(req.Body, boundary), nil
}

func uploadFileStream(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().FileSettings.EnableFileAttachments {
		c.Err = model.NewAppError("uploadFileStream",
			"api.file.attachments.disabled.app_error",
			nil, "", http.StatusNotImplemented)
		return
	}

	// Parse the post as a regular form (in practice, use the URL values
	// since we never expect a real application/x-www-form-urlencoded
	// form).
	if r.Form == nil {
		err := r.ParseForm()
		if err != nil {
			c.Err = model.NewAppError("uploadFileStream",
				"api.file.upload_file.read_request.app_error",
				nil, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if r.ContentLength == 0 {
		c.Err = model.NewAppError("uploadFileStream",
			"api.file.upload_file.read_request.app_error",
			nil, "Content-Length should not be 0", http.StatusBadRequest)
		return
	}

	timestamp := time.Now()
	var fileUploadResponse *model.FileUploadResponse

	_, err := parseMultipartRequestHeader(r)
	switch err {
	case nil:
		fileUploadResponse = uploadFileMultipart(c, r, nil, timestamp)

	case http.ErrNotMultipart:
		fileUploadResponse = uploadFileSimple(c, r, timestamp)

	default:
		c.Err = model.NewAppError("uploadFileStream",
			"api.file.upload_file.read_request.app_error",
			nil, err.Error(), http.StatusBadRequest)
	}
	if c.Err != nil {
		return
	}

	// Write the response values to the output upon return
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fileUploadResponse.ToJSON()))
}

// uploadFileSimple uploads a file from a simple POST with the file in the request body
func uploadFileSimple(c *Context, r *http.Request, timestamp time.Time) *model.FileUploadResponse {
	// Simple POST with the file in the body and all metadata in the args.
	c.RequireChannelID()
	c.RequireFilename()
	if c.Err != nil {
		return nil
	}

	auditRec := c.MakeAuditRecord("uploadFileSimple", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel_id", c.Params.ChannelID)

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionUploadFile) {
		c.SetPermissionError(model.PermissionUploadFile)
		return nil
	}

	clientID := r.Form.Get("client_id")
	auditRec.AddMeta("client_id", clientID)

	info, appErr := c.App.UploadFileX(c.AppContext, c.Params.ChannelID, c.Params.Filename, r.Body,
		app.UploadFileSetTeamID(FileTeamID),
		app.UploadFileSetUserID(c.AppContext.Session().UserID),
		app.UploadFileSetTimestamp(timestamp),
		app.UploadFileSetContentLength(r.ContentLength),
		app.UploadFileSetClientID(clientID))
	if appErr != nil {
		c.Err = appErr
		return nil
	}
	auditRec.AddMeta("file", info)

	fileUploadResponse := &model.FileUploadResponse{
		FileInfos: []*model.FileInfo{info},
	}
	if clientID != "" {
		fileUploadResponse.ClientIDs = []string{clientID}
	}
	auditRec.Success()
	return fileUploadResponse
}

// uploadFileMultipart parses and uploads file(s) from a mime/multipart
// request.  It pre-buffers up to the first part which is either the (a)
// `channel_id` value, or (b) a file. Then in case of (a) it re-processes the
// entire message recursively calling itself in stream mode. In case of (b) it
// calls to uploadFileMultipartLegacy for legacy support
func uploadFileMultipart(c *Context, r *http.Request, asStream io.Reader, timestamp time.Time) *model.FileUploadResponse {

	expectClientIDs := true
	var clientIDs []string
	resp := model.FileUploadResponse{
		FileInfos: []*model.FileInfo{},
		ClientIDs: []string{},
	}

	var buf *bytes.Buffer
	var mr *multipart.Reader
	var err error
	if asStream == nil {
		// We need to buffer until we get the channel_id, or the first file.
		buf = &bytes.Buffer{}
		mr, err = multipartReader(r, io.TeeReader(r.Body, buf))
	} else {
		mr, err = multipartReader(r, asStream)
	}
	if err != nil {
		c.Err = model.NewAppError("uploadFileMultipart",
			"api.file.upload_file.read_request.app_error",
			nil, err.Error(), http.StatusBadRequest)
		return nil
	}

	nFiles := 0
NextPart:
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.Err = model.NewAppError("uploadFileMultipart",
				"api.file.upload_file.read_request.app_error",
				nil, err.Error(), http.StatusBadRequest)
			return nil
		}

		// Parse any form fields in the multipart.
		formname := part.FormName()
		if formname == "" {
			continue
		}
		filename := part.FileName()
		if filename == "" {
			var b bytes.Buffer
			_, err = io.CopyN(&b, part, maxMultipartFormDataBytes)
			if err != nil && err != io.EOF {
				c.Err = model.NewAppError("uploadFileMultipart",
					"api.file.upload_file.read_form_value.app_error",
					map[string]interface{}{"Formname": formname},
					err.Error(), http.StatusBadRequest)
				return nil
			}
			v := b.String()

			switch formname {
			case "channel_id":
				if c.Params.ChannelID != "" && c.Params.ChannelID != v {
					c.Err = model.NewAppError("uploadFileMultipart",
						"api.file.upload_file.multiple_channel_ids.app_error",
						nil, "", http.StatusBadRequest)
					return nil
				}
				if v != "" {
					c.Params.ChannelID = v
				}

				// Got channel_id, re-process the entire post
				// in the streaming mode.
				if asStream == nil {
					return uploadFileMultipart(c, r, io.MultiReader(buf, r.Body), timestamp)
				}

			case "client_ids":
				if !expectClientIDs {
					c.SetInvalidParam("client_ids")
					return nil
				}
				clientIDs = append(clientIDs, v)

			default:
				c.SetInvalidParam(formname)
				return nil
			}

			continue NextPart
		}

		// A file part.

		if c.Params.ChannelID == "" && asStream == nil {
			// Got file before channel_id, fall back to legacy buffered mode
			mr, err = multipartReader(r, io.MultiReader(buf, r.Body))
			if err != nil {
				c.Err = model.NewAppError("uploadFileMultipart",
					"api.file.upload_file.read_request.app_error",
					nil, err.Error(), http.StatusBadRequest)
				return nil
			}

			return uploadFileMultipartLegacy(c, mr, timestamp)
		}

		c.RequireChannelID()
		if c.Err != nil {
			return nil
		}
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionUploadFile) {
			c.SetPermissionError(model.PermissionUploadFile)
			return nil
		}

		// If there's no clientIds when the first file comes, expect
		// none later.
		if nFiles == 0 && len(clientIDs) == 0 {
			expectClientIDs = false
		}

		// Must have a exactly one client ID for each file.
		clientID := ""
		if expectClientIDs {
			if nFiles >= len(clientIDs) {
				c.SetInvalidParam("client_ids")
				return nil
			}

			clientID = clientIDs[nFiles]
		}

		auditRec := c.MakeAuditRecord("uploadFileMultipart", audit.Fail)
		auditRec.AddMeta("channel_id", c.Params.ChannelID)
		auditRec.AddMeta("client_id", clientID)

		info, appErr := c.App.UploadFileX(c.AppContext, c.Params.ChannelID, filename, part,
			app.UploadFileSetTeamID(FileTeamID),
			app.UploadFileSetUserID(c.AppContext.Session().UserID),
			app.UploadFileSetTimestamp(timestamp),
			app.UploadFileSetContentLength(-1),
			app.UploadFileSetClientID(clientID))
		if appErr != nil {
			c.Err = appErr
			c.LogAuditRec(auditRec)
			return nil
		}
		auditRec.AddMeta("file", info)

		auditRec.Success()
		c.LogAuditRec(auditRec)

		// add to the response
		resp.FileInfos = append(resp.FileInfos, info)
		if expectClientIDs {
			resp.ClientIDs = append(resp.ClientIDs, clientID)
		}

		nFiles++
	}

	// Verify that the number of ClientIds matched the number of files.
	if expectClientIDs && len(clientIDs) != nFiles {
		c.Err = model.NewAppError("uploadFileMultipart",
			"api.file.upload_file.incorrect_number_of_client_ids.app_error",
			map[string]interface{}{"NumClientIds": len(clientIDs), "NumFiles": nFiles},
			"", http.StatusBadRequest)
		return nil
	}

	return &resp
}

// uploadFileMultipartLegacy reads, buffers, and then uploads the message,
// borrowing from http.ParseMultipartForm.  If successful it returns a
// *model.FileUploadResponse filled in with the individual model.FileInfo's.
func uploadFileMultipartLegacy(c *Context, mr *multipart.Reader,
	timestamp time.Time) *model.FileUploadResponse {

	// Parse the entire form.
	form, err := mr.ReadForm(*c.App.Config().FileSettings.MaxFileSize)
	if err != nil {
		c.Err = model.NewAppError("uploadFileMultipartLegacy",
			"api.file.upload_file.read_request.app_error",
			nil, err.Error(), http.StatusInternalServerError)
		return nil
	}

	// get and validate the channel Id, permission to upload there.
	if len(form.Value["channel_id"]) == 0 {
		c.SetInvalidParam("channel_id")
		return nil
	}
	channelID := form.Value["channel_id"][0]
	c.Params.ChannelID = channelID
	c.RequireChannelID()
	if c.Err != nil {
		return nil
	}
	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channelID, model.PermissionUploadFile) {
		c.SetPermissionError(model.PermissionUploadFile)
		return nil
	}

	// Check that we have either no client IDs, or one per file.
	clientIDs := form.Value["client_ids"]
	fileHeaders := form.File["files"]
	if len(clientIDs) != 0 && len(clientIDs) != len(fileHeaders) {
		c.Err = model.NewAppError("uploadFilesMultipartBuffered",
			"api.file.upload_file.incorrect_number_of_client_ids.app_error",
			map[string]interface{}{"NumClientIds": len(clientIDs), "NumFiles": len(fileHeaders)},
			"", http.StatusBadRequest)
		return nil
	}

	resp := model.FileUploadResponse{
		FileInfos: []*model.FileInfo{},
		ClientIDs: []string{},
	}

	for i, fileHeader := range fileHeaders {
		f, err := fileHeader.Open()
		if err != nil {
			c.Err = model.NewAppError("uploadFileMultipartLegacy",
				"api.file.upload_file.read_request.app_error",
				nil, err.Error(), http.StatusBadRequest)
			return nil
		}

		clientID := ""
		if len(clientIDs) > 0 {
			clientID = clientIDs[i]
		}

		auditRec := c.MakeAuditRecord("uploadFileMultipartLegacy", audit.Fail)
		defer c.LogAuditRec(auditRec)
		auditRec.AddMeta("channel_id", channelID)
		auditRec.AddMeta("client_id", clientID)

		info, appErr := c.App.UploadFileX(c.AppContext, c.Params.ChannelID, fileHeader.Filename, f,
			app.UploadFileSetTeamID(FileTeamID),
			app.UploadFileSetUserID(c.AppContext.Session().UserID),
			app.UploadFileSetTimestamp(timestamp),
			app.UploadFileSetContentLength(-1),
			app.UploadFileSetClientID(clientID))
		f.Close()
		if appErr != nil {
			c.Err = appErr
			c.LogAuditRec(auditRec)
			return nil
		}
		auditRec.AddMeta("file", info)

		auditRec.Success()
		c.LogAuditRec(auditRec)

		resp.FileInfos = append(resp.FileInfos, info)
		if clientID != "" {
			resp.ClientIDs = append(resp.ClientIDs, clientID)
		}
	}

	return &resp
}

func getFile(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireFileID()
	if c.Err != nil {
		return
	}

	forceDownload, _ := strconv.ParseBool(r.URL.Query().Get("download"))

	auditRec := c.MakeAuditRecord("getFile", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("force_download", forceDownload)

	info, err := c.App.GetFileInfo(c.Params.FileID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("file", info)

	if info.CreatorID != c.AppContext.Session().UserID && !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), info.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	fileReader, err := c.App.FileReader(info.Path)
	if err != nil {
		c.Err = err
		c.Err.StatusCode = http.StatusNotFound
		return
	}
	defer fileReader.Close()

	auditRec.Success()

	writeFileResponse(info.Name, info.MimeType, info.Size, time.Unix(0, info.UpdateAt*int64(1000*1000)), *c.App.Config().ServiceSettings.WebserverMode, fileReader, forceDownload, w, r)
}

func getFileThumbnail(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireFileID()
	if c.Err != nil {
		return
	}

	forceDownload, _ := strconv.ParseBool(r.URL.Query().Get("download"))
	info, err := c.App.GetFileInfo(c.Params.FileID)
	if err != nil {
		c.Err = err
		return
	}

	if info.CreatorID != c.AppContext.Session().UserID && !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), info.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	if info.ThumbnailPath == "" {
		c.Err = model.NewAppError("getFileThumbnail", "api.file.get_file_thumbnail.no_thumbnail.app_error", nil, "file_id="+info.ID, http.StatusBadRequest)
		return
	}

	fileReader, err := c.App.FileReader(info.ThumbnailPath)
	if err != nil {
		c.Err = err
		c.Err.StatusCode = http.StatusNotFound
		return
	}
	defer fileReader.Close()

	writeFileResponse(info.Name, ThumbnailImageType, 0, time.Unix(0, info.UpdateAt*int64(1000*1000)), *c.App.Config().ServiceSettings.WebserverMode, fileReader, forceDownload, w, r)
}

func getFileLink(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireFileID()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().FileSettings.EnablePublicLink {
		c.Err = model.NewAppError("getPublicLink", "api.file.get_public_link.disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	auditRec := c.MakeAuditRecord("getFileLink", audit.Fail)
	defer c.LogAuditRec(auditRec)

	info, err := c.App.GetFileInfo(c.Params.FileID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("file", info)

	if info.CreatorID != c.AppContext.Session().UserID && !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), info.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	if info.PostID == "" {
		c.Err = model.NewAppError("getPublicLink", "api.file.get_public_link.no_post.app_error", nil, "file_id="+info.ID, http.StatusBadRequest)
		return
	}

	resp := make(map[string]string)
	link := c.App.GeneratePublicLink(c.GetSiteURLHeader(), info)
	resp["link"] = link

	auditRec.Success()
	auditRec.AddMeta("link", link)

	w.Write([]byte(model.MapToJSON(resp)))
}

func getFilePreview(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireFileID()
	if c.Err != nil {
		return
	}

	forceDownload, _ := strconv.ParseBool(r.URL.Query().Get("download"))
	info, err := c.App.GetFileInfo(c.Params.FileID)
	if err != nil {
		c.Err = err
		return
	}

	if info.CreatorID != c.AppContext.Session().UserID && !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), info.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	if info.PreviewPath == "" {
		c.Err = model.NewAppError("getFilePreview", "api.file.get_file_preview.no_preview.app_error", nil, "file_id="+info.ID, http.StatusBadRequest)
		return
	}

	fileReader, err := c.App.FileReader(info.PreviewPath)
	if err != nil {
		c.Err = err
		c.Err.StatusCode = http.StatusNotFound
		return
	}
	defer fileReader.Close()

	writeFileResponse(info.Name, PreviewImageType, 0, time.Unix(0, info.UpdateAt*int64(1000*1000)), *c.App.Config().ServiceSettings.WebserverMode, fileReader, forceDownload, w, r)
}

func getFileInfo(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireFileID()
	if c.Err != nil {
		return
	}

	info, err := c.App.GetFileInfo(c.Params.FileID)
	if err != nil {
		c.Err = err
		return
	}

	if info.CreatorID != c.AppContext.Session().UserID && !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), info.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	w.Header().Set("Cache-Control", "max-age=2592000, private")
	w.Write([]byte(info.ToJSON()))
}

func getPublicFile(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireFileID()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().FileSettings.EnablePublicLink {
		c.Err = model.NewAppError("getPublicFile", "api.file.get_public_link.disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	info, err := c.App.GetFileInfo(c.Params.FileID)
	if err != nil {
		c.Err = err
		return
	}

	hash := r.URL.Query().Get("h")

	if hash == "" {
		c.Err = model.NewAppError("getPublicFile", "api.file.get_file.public_invalid.app_error", nil, "", http.StatusBadRequest)
		utils.RenderWebAppError(c.App.Config(), w, r, c.Err, c.App.AsymmetricSigningKey())
		return
	}

	if subtle.ConstantTimeCompare([]byte(hash), []byte(app.GeneratePublicLinkHash(info.ID, *c.App.Config().FileSettings.PublicLinkSalt))) != 1 {
		c.Err = model.NewAppError("getPublicFile", "api.file.get_file.public_invalid.app_error", nil, "", http.StatusBadRequest)
		utils.RenderWebAppError(c.App.Config(), w, r, c.Err, c.App.AsymmetricSigningKey())
		return
	}

	fileReader, err := c.App.FileReader(info.Path)
	if err != nil {
		c.Err = err
		c.Err.StatusCode = http.StatusNotFound
		return
	}
	defer fileReader.Close()

	writeFileResponse(info.Name, info.MimeType, info.Size, time.Unix(0, info.UpdateAt*int64(1000*1000)), *c.App.Config().ServiceSettings.WebserverMode, fileReader, false, w, r)
}

func writeFileResponse(filename string, contentType string, contentSize int64, lastModification time.Time, webserverMode string, fileReader io.ReadSeeker, forceDownload bool, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if contentSize > 0 {
		contentSizeStr := strconv.Itoa(int(contentSize))
		if webserverMode == "gzip" {
			w.Header().Set("X-Uncompressed-Content-Length", contentSizeStr)
		} else {
			w.Header().Set("Content-Length", contentSizeStr)
		}
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	} else {
		for _, unsafeContentType := range UnsafeContentTypes {
			if strings.HasPrefix(contentType, unsafeContentType) {
				contentType = "text/plain"
				break
			}
		}
	}

	w.Header().Set("Content-Type", contentType)

	var toDownload bool
	if forceDownload {
		toDownload = true
	} else {
		isMediaType := false

		for _, mediaContentType := range MediaContentTypes {
			if strings.HasPrefix(contentType, mediaContentType) {
				isMediaType = true
				break
			}
		}

		toDownload = !isMediaType
	}

	filename = url.PathEscape(filename)

	if toDownload {
		w.Header().Set("Content-Disposition", "attachment;filename=\""+filename+"\"; filename*=UTF-8''"+filename)
	} else {
		w.Header().Set("Content-Disposition", "inline;filename=\""+filename+"\"; filename*=UTF-8''"+filename)
	}

	// prevent file links from being embedded in iframes
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "Frame-ancestors 'none'")

	http.ServeContent(w, r, filename, lastModification, fileReader)
}

func searchFiles(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	params, jsonErr := model.SearchParameterFromJSON(r.Body)
	if jsonErr != nil {
		c.Err = model.NewAppError("searchFiles", "api.post.search_files.invalid_body.app_error", nil, jsonErr.Error(), http.StatusBadRequest)
		return
	}

	if params.Terms == nil || *params.Terms == "" {
		c.SetInvalidParam("terms")
		return
	}
	terms := *params.Terms

	timeZoneOffset := 0
	if params.TimeZoneOffset != nil {
		timeZoneOffset = *params.TimeZoneOffset
	}

	isOrSearch := false
	if params.IsOrSearch != nil {
		isOrSearch = *params.IsOrSearch
	}

	page := 0
	if params.Page != nil {
		page = *params.Page
	}

	perPage := 60
	if params.PerPage != nil {
		perPage = *params.PerPage
	}

	includeDeletedChannels := false
	if params.IncludeDeletedChannels != nil {
		includeDeletedChannels = *params.IncludeDeletedChannels
	}

	startTime := time.Now()

	results, err := c.App.SearchFilesInTeamForUser(c.AppContext, terms, c.AppContext.Session().UserID, c.Params.TeamID, isOrSearch, includeDeletedChannels, timeZoneOffset, page, perPage)

	elapsedTime := float64(time.Since(startTime)) / float64(time.Second)
	metrics := c.App.Metrics()
	if metrics != nil {
		metrics.IncrementFilesSearchCounter()
		metrics.ObserveFilesSearchDuration(elapsedTime)
	}

	if err != nil {
		c.Err = err
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write([]byte(results.ToJSON()))
}
