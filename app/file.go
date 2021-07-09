// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost-server/v5/app/imaging"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/services/docextractor"
	"github.com/mattermost/mattermost-server/v5/shared/filestore"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/utils"

	"github.com/pkg/errors"
)

const (
	maxImageRes                = int64(6048 * 4032) // 24 megapixels, up to ~196MB as a raw image
	imageThumbnailWidth        = 120
	imageThumbnailHeight       = 100
	imagePreviewWidth          = 1920
	miniPreviewImageWidth      = 16
	miniPreviewImageHeight     = 16
	jpegEncQuality             = 90
	maxUploadInitialBufferSize = 1024 * 1024 // 1MB
	maxContentExtractionSize   = 1024 * 1024 // 1MB
)

func (a *App) FileBackend() (filestore.FileBackend, *model.AppError) {
	return a.Srv().FileBackend()
}

func (a *App) CheckMandatoryS3Fields(settings *model.FileSettings) *model.AppError {
	fileBackendSettings := settings.ToFileBackendSettings(false)
	err := fileBackendSettings.CheckMandatoryS3Fields()
	if err != nil {
		return model.NewAppError("CheckMandatoryS3Fields", "api.admin.test_s3.missing_s3_bucket", nil, err.Error(), http.StatusBadRequest)
	}
	return nil
}

func connectionTestErrorToAppError(connTestErr error) *model.AppError {
	switch err := connTestErr.(type) {
	case *filestore.S3FileBackendAuthError:
		return model.NewAppError("TestConnection", "api.file.test_connection_s3_auth.app_error", nil, err.Error(), http.StatusInternalServerError)
	case *filestore.S3FileBackendNoBucketError:
		return model.NewAppError("TestConnection", "api.file.test_connection_s3_bucket_does_not_exist.app_error", nil, err.Error(), http.StatusInternalServerError)
	default:
		return model.NewAppError("TestConnection", "api.file.test_connection.app_error", nil, connTestErr.Error(), http.StatusInternalServerError)
	}
}

func (a *App) TestFileStoreConnection() *model.AppError {
	backend, err := a.FileBackend()
	if err != nil {
		return err
	}
	nErr := backend.TestConnection()
	if nErr != nil {
		return connectionTestErrorToAppError(nErr)
	}
	return nil
}

func (a *App) TestFileStoreConnectionWithConfig(cfg *model.FileSettings) *model.AppError {
	license := a.Srv().License()
	backend, err := filestore.NewFileBackend(cfg.ToFileBackendSettings(license != nil && *license.Features.Compliance))
	if err != nil {
		return model.NewAppError("FileBackend", "api.file.no_driver.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	nErr := backend.TestConnection()
	if nErr != nil {
		return connectionTestErrorToAppError(nErr)
	}
	return nil
}

func (a *App) ReadFile(path string) ([]byte, *model.AppError) {
	return a.srv.ReadFile(path)
}

func (s *Server) fileReader(path string) (filestore.ReadCloseSeeker, *model.AppError) {
	backend, err := s.FileBackend()
	if err != nil {
		return nil, err
	}
	result, nErr := backend.Reader(path)
	if nErr != nil {
		return nil, model.NewAppError("FileReader", "api.file.file_reader.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	return result, nil
}

// Caller must close the first return value
func (a *App) FileReader(path string) (filestore.ReadCloseSeeker, *model.AppError) {
	return a.Srv().fileReader(path)
}

func (a *App) FileExists(path string) (bool, *model.AppError) {
	return a.Srv().fileExists(path)
}

func (s *Server) fileExists(path string) (bool, *model.AppError) {
	backend, err := s.FileBackend()
	if err != nil {
		return false, err
	}
	result, nErr := backend.FileExists(path)
	if nErr != nil {
		return false, model.NewAppError("FileExists", "api.file.file_exists.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	return result, nil
}

func (a *App) FileSize(path string) (int64, *model.AppError) {
	backend, err := a.FileBackend()
	if err != nil {
		return 0, err
	}
	size, nErr := backend.FileSize(path)
	if nErr != nil {
		return 0, model.NewAppError("FileSize", "api.file.file_size.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	return size, nil
}

func (a *App) FileModTime(path string) (time.Time, *model.AppError) {
	backend, err := a.FileBackend()
	if err != nil {
		return time.Time{}, err
	}
	modTime, nErr := backend.FileModTime(path)
	if nErr != nil {
		return time.Time{}, model.NewAppError("FileModTime", "api.file.file_mod_time.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	return modTime, nil
}

func (a *App) MoveFile(oldPath, newPath string) *model.AppError {
	backend, err := a.FileBackend()
	if err != nil {
		return err
	}
	nErr := backend.MoveFile(oldPath, newPath)
	if nErr != nil {
		return model.NewAppError("MoveFile", "api.file.move_file.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (a *App) WriteFile(fr io.Reader, path string) (int64, *model.AppError) {
	return a.Srv().writeFile(fr, path)
}

func (s *Server) writeFile(fr io.Reader, path string) (int64, *model.AppError) {
	backend, err := s.FileBackend()
	if err != nil {
		return 0, err
	}

	result, nErr := backend.WriteFile(fr, path)
	if nErr != nil {
		return result, model.NewAppError("WriteFile", "api.file.write_file.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	return result, nil
}

func (a *App) AppendFile(fr io.Reader, path string) (int64, *model.AppError) {
	backend, err := a.FileBackend()
	if err != nil {
		return 0, err
	}

	result, nErr := backend.AppendFile(fr, path)
	if nErr != nil {
		return result, model.NewAppError("AppendFile", "api.file.append_file.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	return result, nil
}

func (a *App) RemoveFile(path string) *model.AppError {
	return a.Srv().removeFile(path)
}

func (s *Server) removeFile(path string) *model.AppError {
	backend, err := s.FileBackend()
	if err != nil {
		return err
	}
	nErr := backend.RemoveFile(path)
	if nErr != nil {
		return model.NewAppError("RemoveFile", "api.file.remove_file.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (a *App) ListDirectory(path string) ([]string, *model.AppError) {
	return a.Srv().listDirectory(path)
}

func (s *Server) listDirectory(path string) ([]string, *model.AppError) {
	backend, err := s.FileBackend()
	if err != nil {
		return nil, err
	}
	paths, nErr := backend.ListDirectory(path)
	if nErr != nil {
		return nil, model.NewAppError("ListDirectory", "api.file.list_directory.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	return paths, nil
}

func (a *App) RemoveDirectory(path string) *model.AppError {
	backend, err := a.FileBackend()
	if err != nil {
		return err
	}
	nErr := backend.RemoveDirectory(path)
	if nErr != nil {
		return model.NewAppError("RemoveDirectory", "api.file.remove_directory.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (a *App) getInfoForFilename(post *model.Post, teamID, channelID, userID, oldID, filename string) *model.FileInfo {
	name, _ := url.QueryUnescape(filename)
	pathPrefix := fmt.Sprintf("teams/%s/channels/%s/users/%s/%s/", teamID, channelID, userID, oldID)
	path := pathPrefix + name

	// Open the file and populate the fields of the FileInfo
	data, err := a.ReadFile(path)
	if err != nil {
		mlog.Error(
			"File not found when migrating post to use FileInfos",
			mlog.String("post_id", post.ID),
			mlog.String("filename", filename),
			mlog.String("path", path),
			mlog.Err(err),
		)
		return nil
	}

	info, err := model.GetInfoForBytes(name, bytes.NewReader(data), len(data))
	if err != nil {
		mlog.Warn(
			"Unable to fully decode file info when migrating post to use FileInfos",
			mlog.String("post_id", post.ID),
			mlog.String("filename", filename),
			mlog.Err(err),
		)
	}

	// Generate a new ID because with the old system, you could very rarely get multiple posts referencing the same file
	info.ID = model.NewID()
	info.CreatorID = post.UserID
	info.PostID = post.ID
	info.CreateAt = post.CreateAt
	info.UpdateAt = post.UpdateAt
	info.Path = path

	if info.IsImage() {
		nameWithoutExtension := name[:strings.LastIndex(name, ".")]
		info.PreviewPath = pathPrefix + nameWithoutExtension + "_preview.jpg"
		info.ThumbnailPath = pathPrefix + nameWithoutExtension + "_thumb.jpg"
	}

	return info
}

func (a *App) findTeamIDForFilename(post *model.Post, id, filename string) string {
	name, _ := url.QueryUnescape(filename)

	// This post is in a direct channel so we need to figure out what team the files are stored under.
	teams, err := a.Srv().Store.Team().GetTeamsByUserID(post.UserID)
	if err != nil {
		mlog.Error("Unable to get teams when migrating post to use FileInfo", mlog.Err(err), mlog.String("post_id", post.ID))
		return ""
	}

	if len(teams) == 1 {
		// The user has only one team so the post must've been sent from it
		return teams[0].ID
	}

	for _, team := range teams {
		path := fmt.Sprintf("teams/%s/channels/%s/users/%s/%s/%s", team.ID, post.ChannelID, post.UserID, id, name)
		if ok, err := a.FileExists(path); ok && err == nil {
			// Found the team that this file was posted from
			return team.ID
		}
	}

	return ""
}

var fileMigrationLock sync.Mutex
var oldFilenameMatchExp *regexp.Regexp = regexp.MustCompile(`^\/([a-z\d]{26})\/([a-z\d]{26})\/([a-z\d]{26})\/([^\/]+)$`)

// Parse the path from the Filename of the form /{channelID}/{userID}/{uid}/{nameWithExtension}
func parseOldFilenames(filenames []string, channelID, userID string) [][]string {
	parsed := [][]string{}
	for _, filename := range filenames {
		matches := oldFilenameMatchExp.FindStringSubmatch(filename)
		if len(matches) != 5 {
			mlog.Error("Failed to parse old Filename", mlog.String("filename", filename))
			continue
		}
		if matches[1] != channelID {
			mlog.Error("ChannelId in Filename does not match", mlog.String("channel_id", channelID), mlog.String("matched", matches[1]))
		} else if matches[2] != userID {
			mlog.Error("UserId in Filename does not match", mlog.String("user_id", userID), mlog.String("matched", matches[2]))
		} else {
			parsed = append(parsed, matches[1:])
		}
	}
	return parsed
}

// Creates and stores FileInfos for a post created before the FileInfos table existed.
func (a *App) MigrateFilenamesToFileInfos(post *model.Post) []*model.FileInfo {
	if len(post.Filenames) == 0 {
		mlog.Warn("Unable to migrate post to use FileInfos with an empty Filenames field", mlog.String("post_id", post.ID))
		return []*model.FileInfo{}
	}

	channel, errCh := a.Srv().Store.Channel().Get(post.ChannelID, true)
	// There's a weird bug that rarely happens where a post ends up with duplicate Filenames so remove those
	filenames := utils.RemoveDuplicatesFromStringArray(post.Filenames)
	if errCh != nil {
		mlog.Error(
			"Unable to get channel when migrating post to use FileInfos",
			mlog.String("post_id", post.ID),
			mlog.String("channel_id", post.ChannelID),
			mlog.Err(errCh),
		)
		return []*model.FileInfo{}
	}

	// Parse and validate filenames before further processing
	parsedFilenames := parseOldFilenames(filenames, post.ChannelID, post.UserID)

	if len(parsedFilenames) == 0 {
		mlog.Error("Unable to parse filenames")
		return []*model.FileInfo{}
	}

	// Find the team that was used to make this post since its part of the file path that isn't saved in the Filename
	var teamID string
	if channel.TeamID == "" {
		// This post was made in a cross-team DM channel, so we need to find where its files were saved
		teamID = a.findTeamIDForFilename(post, parsedFilenames[0][2], parsedFilenames[0][3])
	} else {
		teamID = channel.TeamID
	}

	// Create FileInfo objects for this post
	infos := make([]*model.FileInfo, 0, len(filenames))
	if teamID == "" {
		mlog.Error(
			"Unable to find team id for files when migrating post to use FileInfos",
			mlog.String("filenames", strings.Join(filenames, ",")),
			mlog.String("post_id", post.ID),
		)
	} else {
		for _, parsed := range parsedFilenames {
			info := a.getInfoForFilename(post, teamID, parsed[0], parsed[1], parsed[2], parsed[3])
			if info == nil {
				continue
			}

			infos = append(infos, info)
		}
	}

	// Lock to prevent only one migration thread from trying to update the post at once, preventing duplicate FileInfos from being created
	fileMigrationLock.Lock()
	defer fileMigrationLock.Unlock()

	result, nErr := a.Srv().Store.Post().Get(context.Background(), post.ID, false, false, false, "")
	if nErr != nil {
		mlog.Error("Unable to get post when migrating post to use FileInfos", mlog.Err(nErr), mlog.String("post_id", post.ID))
		return []*model.FileInfo{}
	}

	if newPost := result.Posts[post.ID]; len(newPost.Filenames) != len(post.Filenames) {
		// Another thread has already created FileInfos for this post, so just return those
		var fileInfos []*model.FileInfo
		fileInfos, nErr = a.Srv().Store.FileInfo().GetForPost(post.ID, true, false, false)
		if nErr != nil {
			mlog.Error("Unable to get FileInfos for migrated post", mlog.Err(nErr), mlog.String("post_id", post.ID))
			return []*model.FileInfo{}
		}

		mlog.Debug("Post already migrated to use FileInfos", mlog.String("post_id", post.ID))
		return fileInfos
	}

	mlog.Debug("Migrating post to use FileInfos", mlog.String("post_id", post.ID))

	savedInfos := make([]*model.FileInfo, 0, len(infos))
	fileIDs := make([]string, 0, len(filenames))
	for _, info := range infos {
		if _, nErr = a.Srv().Store.FileInfo().Save(info); nErr != nil {
			mlog.Error(
				"Unable to save file info when migrating post to use FileInfos",
				mlog.String("post_id", post.ID),
				mlog.String("file_info_id", info.ID),
				mlog.String("file_info_path", info.Path),
				mlog.Err(nErr),
			)
			continue
		}

		savedInfos = append(savedInfos, info)
		fileIDs = append(fileIDs, info.ID)
	}

	// Copy and save the updated post
	newPost := post.Clone()

	newPost.Filenames = []string{}
	newPost.FileIDs = fileIDs

	// Update Posts to clear Filenames and set FileIds
	if _, nErr = a.Srv().Store.Post().Update(newPost, post); nErr != nil {
		mlog.Error(
			"Unable to save migrated post when migrating to use FileInfos",
			mlog.String("new_file_ids", strings.Join(newPost.FileIDs, ",")),
			mlog.String("old_filenames", strings.Join(post.Filenames, ",")),
			mlog.String("post_id", post.ID),
			mlog.Err(nErr),
		)
		return []*model.FileInfo{}
	}
	return savedInfos
}

func (a *App) GeneratePublicLink(siteURL string, info *model.FileInfo) string {
	hash := GeneratePublicLinkHash(info.ID, *a.Config().FileSettings.PublicLinkSalt)
	return fmt.Sprintf("%s/files/%v/public?h=%s", siteURL, info.ID, hash)
}

func GeneratePublicLinkHash(fileID, salt string) string {
	hash := sha256.New()
	hash.Write([]byte(salt))
	hash.Write([]byte(fileID))

	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
}

func (a *App) UploadMultipartFiles(c *request.Context, teamID string, channelID string, userID string, fileHeaders []*multipart.FileHeader, clientIDs []string, now time.Time) (*model.FileUploadResponse, *model.AppError) {
	files := make([]io.ReadCloser, len(fileHeaders))
	filenames := make([]string, len(fileHeaders))

	for i, fileHeader := range fileHeaders {
		file, fileErr := fileHeader.Open()
		if fileErr != nil {
			return nil, model.NewAppError("UploadFiles", "api.file.upload_file.read_request.app_error",
				map[string]interface{}{"Filename": fileHeader.Filename}, fileErr.Error(), http.StatusBadRequest)
		}

		// Will be closed after UploadFiles returns
		defer file.Close()

		files[i] = file
		filenames[i] = fileHeader.Filename
	}

	return a.UploadFiles(c, teamID, channelID, userID, files, filenames, clientIDs, now)
}

// Uploads some files to the given team and channel as the given user. files and filenames should have
// the same length. clientIds should either not be provided or have the same length as files and filenames.
// The provided files should be closed by the caller so that they are not leaked.
func (a *App) UploadFiles(c *request.Context, teamID string, channelID string, userID string, files []io.ReadCloser, filenames []string, clientIDs []string, now time.Time) (*model.FileUploadResponse, *model.AppError) {
	if *a.Config().FileSettings.DriverName == "" {
		return nil, model.NewAppError("UploadFiles", "api.file.upload_file.storage.app_error", nil, "", http.StatusNotImplemented)
	}

	if len(filenames) != len(files) || (len(clientIDs) > 0 && len(clientIDs) != len(files)) {
		return nil, model.NewAppError("UploadFiles", "api.file.upload_file.incorrect_number_of_files.app_error", nil, "", http.StatusBadRequest)
	}

	resStruct := &model.FileUploadResponse{
		FileInfos: []*model.FileInfo{},
		ClientIDs: []string{},
	}

	previewPathList := []string{}
	thumbnailPathList := []string{}
	imageDataList := [][]byte{}

	for i, file := range files {
		buf := bytes.NewBuffer(nil)
		io.Copy(buf, file)
		data := buf.Bytes()

		info, data, err := a.DoUploadFileExpectModification(c, now, teamID, channelID, userID, filenames[i], data)
		if err != nil {
			return nil, err
		}

		if info.PreviewPath != "" || info.ThumbnailPath != "" {
			previewPathList = append(previewPathList, info.PreviewPath)
			thumbnailPathList = append(thumbnailPathList, info.ThumbnailPath)
			imageDataList = append(imageDataList, data)
		}

		resStruct.FileInfos = append(resStruct.FileInfos, info)

		if len(clientIDs) > 0 {
			resStruct.ClientIDs = append(resStruct.ClientIDs, clientIDs[i])
		}
	}

	a.HandleImages(previewPathList, thumbnailPathList, imageDataList)

	return resStruct, nil
}

// UploadFile uploads a single file in form of a completely constructed byte array for a channel.
func (a *App) UploadFile(c *request.Context, data []byte, channelID string, filename string) (*model.FileInfo, *model.AppError) {
	_, err := a.GetChannel(channelID)
	if err != nil && channelID != "" {
		return nil, model.NewAppError("UploadFile", "api.file.upload_file.incorrect_channelId.app_error",
			map[string]interface{}{"channelId": channelID}, "", http.StatusBadRequest)
	}

	info, _, appError := a.DoUploadFileExpectModification(c, time.Now(), "noteam", channelID, "nouser", filename, data)
	if appError != nil {
		return nil, appError
	}

	if info.PreviewPath != "" || info.ThumbnailPath != "" {
		previewPathList := []string{info.PreviewPath}
		thumbnailPathList := []string{info.ThumbnailPath}
		imageDataList := [][]byte{data}

		a.HandleImages(previewPathList, thumbnailPathList, imageDataList)
	}

	return info, nil
}

func (a *App) DoUploadFile(c *request.Context, now time.Time, rawTeamID string, rawChannelID string, rawUserID string, rawFilename string, data []byte) (*model.FileInfo, *model.AppError) {
	info, _, err := a.DoUploadFileExpectModification(c, now, rawTeamID, rawChannelID, rawUserID, rawFilename, data)
	return info, err
}

func UploadFileSetTeamID(teamID string) func(t *UploadFileTask) {
	return func(t *UploadFileTask) {
		t.TeamID = filepath.Base(teamID)
	}
}

func UploadFileSetUserID(userID string) func(t *UploadFileTask) {
	return func(t *UploadFileTask) {
		t.UserID = filepath.Base(userID)
	}
}

func UploadFileSetTimestamp(timestamp time.Time) func(t *UploadFileTask) {
	return func(t *UploadFileTask) {
		t.Timestamp = timestamp
	}
}

func UploadFileSetContentLength(contentLength int64) func(t *UploadFileTask) {
	return func(t *UploadFileTask) {
		t.ContentLength = contentLength
	}
}

func UploadFileSetClientID(clientID string) func(t *UploadFileTask) {
	return func(t *UploadFileTask) {
		t.ClientID = clientID
	}
}

func UploadFileSetRaw() func(t *UploadFileTask) {
	return func(t *UploadFileTask) {
		t.Raw = true
	}
}

type UploadFileTask struct {
	// File name.
	Name string

	ChannelID string
	TeamID    string
	UserID    string

	// Time stamp to use when creating the file.
	Timestamp time.Time

	// The value of the Content-Length http header, when available.
	ContentLength int64

	// The file data stream.
	Input io.Reader

	// An optional, client-assigned Id field.
	ClientID string

	// If Raw, do not execute special processing for images, just upload
	// the file.  Plugins are still invoked.
	Raw bool

	//=============================================================
	// Internal state

	buf          *bytes.Buffer
	limit        int64
	limitedInput io.Reader
	teeInput     io.Reader
	fileinfo     *model.FileInfo
	maxFileSize  int64

	// Cached image data that (may) get initialized in preprocessImage and
	// is used in postprocessImage
	decoded          image.Image
	imageType        string
	imageOrientation int

	// Testing: overrideable dependency functions
	pluginsEnvironment *plugin.Environment
	writeFile          func(io.Reader, string) (int64, *model.AppError)
	saveToDatabase     func(*model.FileInfo) (*model.FileInfo, error)

	imgDecoder *imaging.Decoder
	imgEncoder *imaging.Encoder
}

func (t *UploadFileTask) init(a *App) {
	t.buf = &bytes.Buffer{}
	if t.ContentLength > 0 {
		t.limit = t.ContentLength
	} else {
		t.limit = t.maxFileSize
	}

	if t.ContentLength > 0 && t.ContentLength < maxUploadInitialBufferSize {
		t.buf.Grow(int(t.ContentLength))
	} else {
		t.buf.Grow(maxUploadInitialBufferSize)
	}

	t.fileinfo = model.NewInfo(filepath.Base(t.Name))
	t.fileinfo.ID = model.NewID()
	t.fileinfo.CreatorID = t.UserID
	t.fileinfo.CreateAt = t.Timestamp.UnixNano() / int64(time.Millisecond)
	t.fileinfo.Path = t.pathPrefix() + t.Name

	t.limitedInput = &io.LimitedReader{
		R: t.Input,
		N: t.limit + 1,
	}
	t.teeInput = io.TeeReader(t.limitedInput, t.buf)

	t.pluginsEnvironment = a.GetPluginsEnvironment()
	t.writeFile = a.WriteFile
	t.saveToDatabase = a.Srv().Store.FileInfo().Save
}

// UploadFileX uploads a single file as specified in t. It applies the upload
// constraints, executes plugins and image processing logic as needed. It
// returns a filled-out FileInfo and an optional error. A plugin may reject the
// upload, returning a rejection error. In this case FileInfo would have
// contained the last "good" FileInfo before the execution of that plugin.
func (a *App) UploadFileX(c *request.Context, channelID, name string, input io.Reader,
	opts ...func(*UploadFileTask)) (*model.FileInfo, *model.AppError) {

	t := &UploadFileTask{
		ChannelID:   filepath.Base(channelID),
		Name:        filepath.Base(name),
		Input:       input,
		maxFileSize: *a.Config().FileSettings.MaxFileSize,
		imgDecoder:  a.srv.imgDecoder,
		imgEncoder:  a.srv.imgEncoder,
	}
	for _, o := range opts {
		o(t)
	}

	if *a.Config().FileSettings.DriverName == "" {
		return nil, t.newAppError("api.file.upload_file.storage.app_error", http.StatusNotImplemented)
	}
	if t.ContentLength > t.maxFileSize {
		return nil, t.newAppError("api.file.upload_file.too_large_detailed.app_error", http.StatusRequestEntityTooLarge, "Length", t.ContentLength, "Limit", t.maxFileSize)
	}

	t.init(a)

	var aerr *model.AppError
	if !t.Raw && t.fileinfo.IsImage() {
		aerr = t.preprocessImage()
		if aerr != nil {
			return t.fileinfo, aerr
		}
	}

	written, aerr := t.writeFile(io.MultiReader(t.buf, t.limitedInput), t.fileinfo.Path)
	if aerr != nil {
		return nil, aerr
	}

	if written > t.maxFileSize {
		if fileErr := a.RemoveFile(t.fileinfo.Path); fileErr != nil {
			mlog.Error("Failed to remove file", mlog.Err(fileErr))
		}
		return nil, t.newAppError("api.file.upload_file.too_large_detailed.app_error", http.StatusRequestEntityTooLarge, "Length", t.ContentLength, "Limit", t.maxFileSize)
	}

	t.fileinfo.Size = written

	file, aerr := a.FileReader(t.fileinfo.Path)
	if aerr != nil {
		return nil, aerr
	}
	defer file.Close()

	aerr = a.runPluginsHook(c, t.fileinfo, file)
	if aerr != nil {
		return nil, aerr
	}

	if !t.Raw && t.fileinfo.IsImage() {
		file, aerr = a.FileReader(t.fileinfo.Path)
		if aerr != nil {
			return nil, aerr
		}
		defer file.Close()
		t.postprocessImage(file)
	}

	if _, err := t.saveToDatabase(t.fileinfo); err != nil {
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("UploadFileX", "app.file_info.save.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	if *a.Config().FileSettings.ExtractContent {
		infoCopy := *t.fileinfo
		a.Srv().Go(func() {
			err := a.ExtractContentFromFileInfo(&infoCopy)
			if err != nil {
				mlog.Error("Failed to extract file content", mlog.Err(err), mlog.String("fileInfoId", infoCopy.ID))
			}
		})
	}

	return t.fileinfo, nil
}

func (t *UploadFileTask) preprocessImage() *model.AppError {
	// If SVG, attempt to extract dimensions and then return
	if t.fileinfo.MimeType == "image/svg+xml" {
		svgInfo, err := imaging.ParseSVG(t.teeInput)
		if err != nil {
			mlog.Warn("Failed to parse SVG", mlog.Err(err))
		}
		if svgInfo.Width > 0 && svgInfo.Height > 0 {
			t.fileinfo.Width = svgInfo.Width
			t.fileinfo.Height = svgInfo.Height
		}
		t.fileinfo.HasPreviewImage = false
		return nil
	}

	// If we fail to decode, return "as is".
	w, h, err := imaging.GetDimensions(t.teeInput)
	if err != nil {
		return nil
	}
	t.fileinfo.Width = w
	t.fileinfo.Height = h

	if err = checkImageResolutionLimit(w, h); err != nil {
		return t.newAppError("api.file.upload_file.large_image_detailed.app_error", http.StatusBadRequest)
	}

	t.fileinfo.HasPreviewImage = true
	nameWithoutExtension := t.Name[:strings.LastIndex(t.Name, ".")]
	t.fileinfo.PreviewPath = t.pathPrefix() + nameWithoutExtension + "_preview.jpg"
	t.fileinfo.ThumbnailPath = t.pathPrefix() + nameWithoutExtension + "_thumb.jpg"

	// check the image orientation with goexif; consume the bytes we
	// already have first, then keep Tee-ing from input.
	// TODO: try to reuse exif's .Raw buffer rather than Tee-ing
	if t.imageOrientation, err = imaging.GetImageOrientation(io.MultiReader(bytes.NewReader(t.buf.Bytes()), t.teeInput)); err == nil &&
		(t.imageOrientation == imaging.RotatedCWMirrored ||
			t.imageOrientation == imaging.RotatedCCW ||
			t.imageOrientation == imaging.RotatedCCWMirrored ||
			t.imageOrientation == imaging.RotatedCW) {
		t.fileinfo.Width, t.fileinfo.Height = t.fileinfo.Height, t.fileinfo.Width
	}

	// For animated GIFs disable the preview; since we have to Decode gifs
	// anyway, cache the decoded image for later.
	if t.fileinfo.MimeType == "image/gif" {
		gifConfig, err := gif.DecodeAll(io.MultiReader(bytes.NewReader(t.buf.Bytes()), t.teeInput))
		if err == nil {
			if len(gifConfig.Image) > 0 {
				t.fileinfo.HasPreviewImage = false
				t.decoded = gifConfig.Image[0]
				t.imageType = "gif"
			}
		}
	}

	return nil
}

func (t *UploadFileTask) postprocessImage(file io.Reader) {
	// don't try to process SVG files
	if t.fileinfo.MimeType == "image/svg+xml" {
		return
	}

	decoded, imgType := t.decoded, t.imageType
	if decoded == nil {
		var err error
		var release func()
		decoded, imgType, release, err = t.imgDecoder.DecodeMemBounded(file)
		if err != nil {
			mlog.Error("Unable to decode image", mlog.Err(err))
			return
		}
		defer release()
	}

	// Fill in the background of a potentially-transparent png file as white
	if imgType == "png" {
		imaging.FillImageTransparency(decoded, image.White)
	}

	decoded = imaging.MakeImageUpright(decoded, t.imageOrientation)
	if decoded == nil {
		return
	}

	writeJPEG := func(img image.Image, path string) {
		r, w := io.Pipe()
		go func() {
			err := t.imgEncoder.EncodeJPEG(w, img, jpegEncQuality)
			if err != nil {
				mlog.Error("Unable to encode image as jpeg", mlog.String("path", path), mlog.Err(err))
				w.CloseWithError(err)
			} else {
				w.Close()
			}
		}()
		_, aerr := t.writeFile(r, path)
		if aerr != nil {
			mlog.Error("Unable to upload", mlog.String("path", path), mlog.Err(aerr))
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(3)
	// Generating thumbnail and preview regardless of HasPreviewImage value.
	// This is needed on mobile in case of animated GIFs.
	go func() {
		defer wg.Done()
		writeJPEG(imaging.GenerateThumbnail(decoded, imageThumbnailWidth, imageThumbnailHeight), t.fileinfo.ThumbnailPath)
	}()

	go func() {
		defer wg.Done()
		writeJPEG(imaging.GeneratePreview(decoded, imagePreviewWidth), t.fileinfo.PreviewPath)
	}()

	go func() {
		defer wg.Done()
		if t.fileinfo.MiniPreview == nil {
			if miniPreview, err := imaging.GenerateMiniPreviewImage(decoded,
				miniPreviewImageWidth, miniPreviewImageHeight, jpegEncQuality); err != nil {
				mlog.Info("Unable to generate mini preview image", mlog.Err(err))
			} else {
				t.fileinfo.MiniPreview = &miniPreview
			}
		}
	}()
	wg.Wait()
}

func (t UploadFileTask) pathPrefix() string {
	return t.Timestamp.Format("20060102") +
		"/teams/" + t.TeamID +
		"/channels/" + t.ChannelID +
		"/users/" + t.UserID +
		"/" + t.fileinfo.ID + "/"
}

func (t UploadFileTask) newAppError(id string, httpStatus int, extra ...interface{}) *model.AppError {
	params := map[string]interface{}{
		"Name":          t.Name,
		"Filename":      t.Name,
		"ChannelId":     t.ChannelID,
		"TeamId":        t.TeamID,
		"UserId":        t.UserID,
		"ContentLength": t.ContentLength,
		"ClientId":      t.ClientID,
	}
	if t.fileinfo != nil {
		params["Width"] = t.fileinfo.Width
		params["Height"] = t.fileinfo.Height
	}
	for i := 0; i+1 < len(extra); i += 2 {
		params[fmt.Sprintf("%v", extra[i])] = extra[i+1]
	}

	return model.NewAppError("uploadFileTask", id, params, "", httpStatus)
}

func (a *App) DoUploadFileExpectModification(c *request.Context, now time.Time, rawTeamID string, rawChannelID string, rawUserID string, rawFilename string, data []byte) (*model.FileInfo, []byte, *model.AppError) {
	filename := filepath.Base(rawFilename)
	teamID := filepath.Base(rawTeamID)
	channelID := filepath.Base(rawChannelID)
	userID := filepath.Base(rawUserID)

	info, err := model.GetInfoForBytes(filename, bytes.NewReader(data), len(data))
	if err != nil {
		err.StatusCode = http.StatusBadRequest
		return nil, data, err
	}

	if orientation, err := imaging.GetImageOrientation(bytes.NewReader(data)); err == nil &&
		(orientation == imaging.RotatedCWMirrored ||
			orientation == imaging.RotatedCCW ||
			orientation == imaging.RotatedCCWMirrored ||
			orientation == imaging.RotatedCW) {
		info.Width, info.Height = info.Height, info.Width
	}

	info.ID = model.NewID()
	info.CreatorID = userID
	info.CreateAt = now.UnixNano() / int64(time.Millisecond)

	pathPrefix := now.Format("20060102") + "/teams/" + teamID + "/channels/" + channelID + "/users/" + userID + "/" + info.ID + "/"
	info.Path = pathPrefix + filename

	if info.IsImage() {
		if limitErr := checkImageResolutionLimit(info.Width, info.Height); limitErr != nil {
			err := model.NewAppError("uploadFile", "api.file.upload_file.large_image.app_error", map[string]interface{}{"Filename": filename}, limitErr.Error(), http.StatusBadRequest)
			return nil, data, err
		}

		nameWithoutExtension := filename[:strings.LastIndex(filename, ".")]
		info.PreviewPath = pathPrefix + nameWithoutExtension + "_preview.jpg"
		info.ThumbnailPath = pathPrefix + nameWithoutExtension + "_thumb.jpg"
	}

	if pluginsEnvironment := a.GetPluginsEnvironment(); pluginsEnvironment != nil {
		var rejectionError *model.AppError
		pluginContext := pluginContext(c)
		pluginsEnvironment.RunMultiPluginHook(func(hooks plugin.Hooks) bool {
			var newBytes bytes.Buffer
			replacementInfo, rejectionReason := hooks.FileWillBeUploaded(pluginContext, info, bytes.NewReader(data), &newBytes)
			if rejectionReason != "" {
				rejectionError = model.NewAppError("DoUploadFile", "File rejected by plugin. "+rejectionReason, nil, "", http.StatusBadRequest)
				return false
			}
			if replacementInfo != nil {
				info = replacementInfo
			}
			if newBytes.Len() != 0 {
				data = newBytes.Bytes()
				info.Size = int64(len(data))
			}

			return true
		}, plugin.FileWillBeUploadedID)
		if rejectionError != nil {
			return nil, data, rejectionError
		}
	}

	if _, err := a.WriteFile(bytes.NewReader(data), info.Path); err != nil {
		return nil, data, err
	}

	if _, err := a.Srv().Store.FileInfo().Save(info); err != nil {
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, data, appErr
		default:
			return nil, data, model.NewAppError("DoUploadFileExpectModification", "app.file_info.save.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	if *a.Config().FileSettings.ExtractContent {
		infoCopy := *info
		a.Srv().Go(func() {
			err := a.ExtractContentFromFileInfo(&infoCopy)
			if err != nil {
				mlog.Error("Failed to extract file content", mlog.Err(err), mlog.String("fileInfoId", infoCopy.ID))
			}
		})
	}

	return info, data, nil
}

func (a *App) HandleImages(previewPathList []string, thumbnailPathList []string, fileData [][]byte) {
	wg := new(sync.WaitGroup)

	for i := range fileData {
		img, release, err := prepareImage(a.srv.imgDecoder, bytes.NewReader(fileData[i]))
		if err != nil {
			mlog.Debug("Failed to prepare image", mlog.Err(err))
			continue
		}
		wg.Add(2)
		go func(img image.Image, path string) {
			defer wg.Done()
			a.generateThumbnailImage(img, path)
		}(img, thumbnailPathList[i])

		go func(img image.Image, path string) {
			defer wg.Done()
			a.generatePreviewImage(img, path)
		}(img, previewPathList[i])

		wg.Wait()
		release()
	}
}

func prepareImage(imgDecoder *imaging.Decoder, imgData io.ReadSeeker) (img image.Image, release func(), err error) {
	// Decode image bytes into Image object
	var imgType string
	img, imgType, release, err = imgDecoder.DecodeMemBounded(imgData)
	if err != nil {
		return nil, nil, fmt.Errorf("prepareImage: failed to decode image: %w", err)
	}

	// Fill in the background of a potentially-transparent png file as white
	if imgType == "png" {
		imaging.FillImageTransparency(img, image.White)
	}

	imgData.Seek(0, io.SeekStart)

	// Flip the image to be upright
	orientation, err := imaging.GetImageOrientation(imgData)
	if err != nil {
		mlog.Debug("GetImageOrientation failed", mlog.Err(err))
	}
	img = imaging.MakeImageUpright(img, orientation)

	return img, release, nil
}

func (a *App) generateThumbnailImage(img image.Image, thumbnailPath string) {
	var buf bytes.Buffer
	if err := a.srv.imgEncoder.EncodeJPEG(&buf, imaging.GenerateThumbnail(img, imageThumbnailWidth, imageThumbnailHeight), jpegEncQuality); err != nil {
		mlog.Error("Unable to encode image as jpeg", mlog.String("path", thumbnailPath), mlog.Err(err))
		return
	}

	if _, err := a.WriteFile(&buf, thumbnailPath); err != nil {
		mlog.Error("Unable to upload thumbnail", mlog.String("path", thumbnailPath), mlog.Err(err))
		return
	}
}

func (a *App) generatePreviewImage(img image.Image, previewPath string) {
	var buf bytes.Buffer
	preview := imaging.GeneratePreview(img, imagePreviewWidth)

	if err := a.srv.imgEncoder.EncodeJPEG(&buf, preview, jpegEncQuality); err != nil {
		mlog.Error("Unable to encode image as preview jpg", mlog.Err(err), mlog.String("path", previewPath))
		return
	}

	if _, err := a.WriteFile(&buf, previewPath); err != nil {
		mlog.Error("Unable to upload preview", mlog.Err(err), mlog.String("path", previewPath))
		return
	}
}

// generateMiniPreview updates mini preview if needed
// will save fileinfo with the preview added
func (a *App) generateMiniPreview(fi *model.FileInfo) {
	if fi.IsImage() && fi.MiniPreview == nil {
		file, err := a.FileReader(fi.Path)
		if err != nil {
			mlog.Debug("error reading image file", mlog.Err(err))
			return
		}
		defer file.Close()
		img, release, imgErr := prepareImage(a.srv.imgDecoder, file)
		if imgErr != nil {
			mlog.Debug("generateMiniPreview: prepareImage failed", mlog.Err(imgErr))
			return
		}
		defer release()
		if miniPreview, err := imaging.GenerateMiniPreviewImage(img,
			miniPreviewImageWidth, miniPreviewImageHeight, jpegEncQuality); err != nil {
			mlog.Info("Unable to generate mini preview image", mlog.Err(err))
		} else {
			fi.MiniPreview = &miniPreview
		}
		if _, appErr := a.Srv().Store.FileInfo().Upsert(fi); appErr != nil {
			mlog.Debug("creating mini preview failed", mlog.Err(appErr))
		} else {
			a.Srv().Store.FileInfo().InvalidateFileInfosForPostCache(fi.PostID, false)
		}
	}
}

func (a *App) generateMiniPreviewForInfos(fileInfos []*model.FileInfo) {
	wg := new(sync.WaitGroup)

	wg.Add(len(fileInfos))
	for _, fileInfo := range fileInfos {
		go func(fi *model.FileInfo) {
			defer wg.Done()
			a.generateMiniPreview(fi)
		}(fileInfo)
	}
	wg.Wait()
}

func (a *App) GetFileInfo(fileID string) (*model.FileInfo, *model.AppError) {
	fileInfo, err := a.Srv().Store.FileInfo().Get(fileID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetFileInfo", "app.file_info.get.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetFileInfo", "app.file_info.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	a.generateMiniPreview(fileInfo)
	return fileInfo, nil
}

func (a *App) GetFileInfos(page, perPage int, opt *model.GetFileInfosOptions) ([]*model.FileInfo, *model.AppError) {
	fileInfos, err := a.Srv().Store.FileInfo().GetWithOptions(page, perPage, opt)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var ltErr *store.ErrLimitExceeded
		switch {
		case errors.As(err, &invErr):
			return nil, model.NewAppError("GetFileInfos", "app.file_info.get_with_options.app_error", nil, invErr.Error(), http.StatusBadRequest)
		case errors.As(err, &ltErr):
			return nil, model.NewAppError("GetFileInfos", "app.file_info.get_with_options.app_error", nil, ltErr.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("GetFileInfos", "app.file_info.get_with_options.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	a.generateMiniPreviewForInfos(fileInfos)

	return fileInfos, nil
}

func (a *App) GetFile(fileID string) ([]byte, *model.AppError) {
	info, err := a.GetFileInfo(fileID)
	if err != nil {
		return nil, err
	}

	data, err := a.ReadFile(info.Path)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (a *App) CopyFileInfos(userID string, fileIDs []string) ([]string, *model.AppError) {
	var newFileIDs []string

	now := model.GetMillis()

	for _, fileID := range fileIDs {
		fileInfo, err := a.Srv().Store.FileInfo().Get(fileID)
		if err != nil {
			var nfErr *store.ErrNotFound
			switch {
			case errors.As(err, &nfErr):
				return nil, model.NewAppError("CopyFileInfos", "app.file_info.get.app_error", nil, nfErr.Error(), http.StatusNotFound)
			default:
				return nil, model.NewAppError("CopyFileInfos", "app.file_info.get.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}

		fileInfo.ID = model.NewID()
		fileInfo.CreatorID = userID
		fileInfo.CreateAt = now
		fileInfo.UpdateAt = now
		fileInfo.PostID = ""

		if _, err := a.Srv().Store.FileInfo().Save(fileInfo); err != nil {
			var appErr *model.AppError
			switch {
			case errors.As(err, &appErr):
				return nil, appErr
			default:
				return nil, model.NewAppError("CopyFileInfos", "app.file_info.save.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}

		newFileIDs = append(newFileIDs, fileInfo.ID)
	}

	return newFileIDs, nil
}

// This function zip's up all the files in fileDatas array and then saves it to the directory specified with the specified zip file name
// Ensure the zip file name ends with a .zip
func (a *App) CreateZipFileAndAddFiles(fileBackend filestore.FileBackend, fileDatas []model.FileData, zipFileName, directory string) error {
	// Create Zip File (temporarily stored on disk)
	conglomerateZipFile, err := os.Create(zipFileName)
	if err != nil {
		return err
	}
	defer os.Remove(zipFileName)

	// Create a new zip archive.
	zipFileWriter := zip.NewWriter(conglomerateZipFile)

	// Populate Zip file with File Datas array
	err = populateZipfile(zipFileWriter, fileDatas)
	if err != nil {
		return err
	}

	conglomerateZipFile.Seek(0, 0)
	_, err = fileBackend.WriteFile(conglomerateZipFile, path.Join(directory, zipFileName))
	if err != nil {
		return err
	}

	return nil
}

// This is a implementation of Go's example of writing files to zip (with slight modification)
// https://golang.org/src/archive/zip/example_test.go
func populateZipfile(w *zip.Writer, fileDatas []model.FileData) error {
	defer w.Close()
	for _, fd := range fileDatas {
		f, err := w.Create(fd.Filename)
		if err != nil {
			return err
		}

		_, err = f.Write(fd.Body)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *App) SearchFilesInTeamForUser(c *request.Context, terms string, userID string, teamID string, isOrSearch bool, includeDeletedChannels bool, timeZoneOffset int, page, perPage int) (*model.FileInfoList, *model.AppError) {
	paramsList := model.ParseSearchParams(strings.TrimSpace(terms), timeZoneOffset)
	includeDeleted := includeDeletedChannels && *a.Config().TeamSettings.ExperimentalViewArchivedChannels

	if !*a.Config().ServiceSettings.EnableFileSearch {
		return nil, model.NewAppError("SearchFilesInTeamForUser", "store.sql_file_info.search.disabled", nil, fmt.Sprintf("teamId=%v userId=%v", teamID, userID), http.StatusNotImplemented)
	}

	finalParamsList := []*model.SearchParams{}

	for _, params := range paramsList {
		params.OrTerms = isOrSearch
		params.IncludeDeletedChannels = includeDeleted
		// Don't allow users to search for "*"
		if params.Terms != "*" {
			// Convert channel names to channel IDs
			params.InChannels = a.convertChannelNamesToChannelIDs(c, params.InChannels, userID, teamID, includeDeletedChannels)
			params.ExcludedChannels = a.convertChannelNamesToChannelIDs(c, params.ExcludedChannels, userID, teamID, includeDeletedChannels)

			// Convert usernames to user IDs
			params.FromUsers = a.convertUserNameToUserIDs(params.FromUsers)
			params.ExcludedUsers = a.convertUserNameToUserIDs(params.ExcludedUsers)

			finalParamsList = append(finalParamsList, params)
		}
	}

	// If the processed search params are empty, return empty search results.
	if len(finalParamsList) == 0 {
		return model.NewFileInfoList(), nil
	}

	fileInfoSearchResults, nErr := a.Srv().Store.FileInfo().Search(finalParamsList, userID, teamID, page, perPage)
	if nErr != nil {
		var appErr *model.AppError
		switch {
		case errors.As(nErr, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("SearchPostsInTeamForUser", "app.post.search.app_error", nil, nErr.Error(), http.StatusInternalServerError)
		}
	}

	return fileInfoSearchResults, nil
}

func (a *App) ExtractContentFromFileInfo(fileInfo *model.FileInfo) error {
	file, aerr := a.FileReader(fileInfo.Path)
	if aerr != nil {
		return errors.Wrap(aerr, "failed to open file for extract file content")
	}
	defer file.Close()
	text, err := docextractor.Extract(fileInfo.Name, file, docextractor.ExtractSettings{
		ArchiveRecursion: *a.Config().FileSettings.ArchiveRecursion,
	})
	if err != nil {
		return errors.Wrap(err, "failed to extract file content")
	}
	if text != "" {
		if len(text) > maxContentExtractionSize {
			text = text[0:maxContentExtractionSize]
		}
		if storeErr := a.Srv().Store.FileInfo().SetContent(fileInfo.ID, text); storeErr != nil {
			return errors.Wrap(storeErr, "failed to save the extracted file content")
		}
	}
	return nil
}
