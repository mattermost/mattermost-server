// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
	"github.com/mattermost/mattermost-server/v5/utils/testutils"
)

var testDir = ""

func init() {
	testDir, _ = fileutils.FindDir("tests")
}

// File Section
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func randomBytes(t *testing.T, n int) []byte {
	bb := make([]byte, n)
	_, err := rand.Read(bb)
	require.NoError(t, err)
	return bb
}

func fileBytes(t *testing.T, path string) []byte {
	path = filepath.Join(testDir, path)
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	bb, err := ioutil.ReadAll(f)
	require.NoError(t, err)
	return bb
}

func testDoUploadFileRequest(t testing.TB, c *model.Client4, url string, blob []byte, contentType string,
	contentLength int64) (*model.FileUploadResponse, *model.Response) {
	req, err := http.NewRequest("POST", c.APIURL+c.GetFilesRoute()+url, bytes.NewReader(blob))
	require.NoError(t, err)

	if contentLength != 0 {
		req.ContentLength = contentLength
	}
	req.Header.Set("Content-Type", contentType)
	if c.AuthToken != "" {
		req.Header.Set(model.HeaderAuth, c.AuthType+" "+c.AuthToken)
	}

	resp, err := c.HTTPClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer closeBody(resp)

	if resp.StatusCode >= 300 {
		return nil, model.BuildErrorResponse(resp, model.AppErrorFromJSON(resp.Body))
	}

	return model.FileUploadResponseFromJSON(resp.Body), model.BuildResponse(resp)
}

func testUploadFilesPost(
	t testing.TB,
	c *model.Client4,
	channelID string,
	names []string,
	blobs [][]byte,
	clientIDs []string,
	useChunked bool,
) (*model.FileUploadResponse, *model.Response) {

	// Do not check len(clientIds), leave it entirely to the user to
	// provide. The server will error out if it does not match the number
	// of files, but it's not critical here.
	require.NotEmpty(t, names)
	require.NotEmpty(t, blobs)
	require.Equal(t, len(names), len(blobs))

	fileUploadResponse := &model.FileUploadResponse{}
	for i, blob := range blobs {
		var cl int64
		if useChunked {
			cl = -1
		} else {
			cl = int64(len(blob))
		}
		ct := http.DetectContentType(blob)

		postURL := fmt.Sprintf("?channel_id=%v", url.QueryEscape(channelID)) +
			fmt.Sprintf("&filename=%v", url.QueryEscape(names[i]))
		if len(clientIDs) > i {
			postURL += fmt.Sprintf("&client_id=%v", url.QueryEscape(clientIDs[i]))
		}

		fur, resp := testDoUploadFileRequest(t, c, postURL, blob, ct, cl)
		if resp.Error != nil {
			return nil, resp
		}

		fileUploadResponse.FileInfos = append(fileUploadResponse.FileInfos, fur.FileInfos[0])
		if len(clientIDs) > 0 {
			if len(fur.ClientIDs) > 0 {
				fileUploadResponse.ClientIDs = append(fileUploadResponse.ClientIDs, fur.ClientIDs[0])
			} else {
				fileUploadResponse.ClientIDs = append(fileUploadResponse.ClientIDs, "")
			}
		}
	}

	return fileUploadResponse, nil
}

func testUploadFilesMultipart(
	t testing.TB,
	c *model.Client4,
	channelID string,
	names []string,
	blobs [][]byte,
	clientIDs []string,
) (
	fileUploadResponse *model.FileUploadResponse,
	response *model.Response,
) {
	// Do not check len(clientIds), leave it entirely to the user to
	// provide. The server will error out if it does not match the number
	// of files, but it's not critical here.
	require.NotEmpty(t, names)
	require.NotEmpty(t, blobs)
	require.Equal(t, len(names), len(blobs))

	mwBody := &bytes.Buffer{}
	mw := multipart.NewWriter(mwBody)

	err := mw.WriteField("channel_id", channelID)
	require.NoError(t, err)

	for i, blob := range blobs {
		ct := http.DetectContentType(blob)
		if len(clientIDs) > i {
			err = mw.WriteField("client_ids", clientIDs[i])
			require.NoError(t, err)
		}

		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="files"; filename="%s"`, escapeQuotes(names[i])))
		h.Set("Content-Type", ct)

		// If we error here, writing to mw, the deferred handler
		part, err := mw.CreatePart(h)
		require.NoError(t, err)

		_, err = io.Copy(part, bytes.NewReader(blob))
		require.NoError(t, err)
	}

	require.NoError(t, mw.Close())
	return testDoUploadFileRequest(t, c, "", mwBody.Bytes(), mw.FormDataContentType(), -1)
}

func TestUploadFiles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	if *th.App.Config().FileSettings.DriverName == "" {
		t.Skip("skipping because no file driver is enabled")
	}

	channel := th.BasicChannel
	date := time.Now().Format("20060102")

	// Get better error messages
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableDeveloper = true })

	tests := []struct {
		title     string
		client    *model.Client4
		blobs     [][]byte
		names     []string
		clientIDs []string

		skipSuccessValidation       bool
		skipPayloadValidation       bool
		skipSimplePost              bool
		skipMultipart               bool
		channelID                   string
		useChunkedInSimplePost      bool
		expectedCreatorID           string
		expectedPayloadNames        []string
		expectImage                 bool
		expectedImageWidths         []int
		expectedImageHeights        []int
		expectedImageThumbnailNames []string
		expectedImagePreviewNames   []string
		expectedImageHasPreview     []bool
		expectedImageMiniPreview    []bool
		setupConfig                 func(a *app.App) func(a *app.App)
		checkResponse               func(t *testing.T, resp *model.Response)
	}{
		// Upload a bunch of files, mixed images and non-images
		{
			title:                    "Happy",
			names:                    []string{"test.png", "testgif.gif", "testplugin.tar.gz", "test-search.md", "test.tiff"},
			expectedCreatorID:        th.BasicUser.ID,
			expectedImageMiniPreview: []bool{true, true, false, false, true},
		},
		// Upload a bunch of files, with clientIds
		{
			title:                    "Happy client_ids",
			names:                    []string{"test.png", "testgif.gif", "testplugin.tar.gz", "test-search.md", "test.tiff"},
			clientIDs:                []string{"1", "2", "3", "4", "5"},
			expectedImageMiniPreview: []bool{true, true, false, false, true},
			expectedCreatorID:        th.BasicUser.ID,
		},
		// Upload a bunch of images. testgif.gif is an animated GIF,
		// so it does not have HasPreviewImage set.
		{
			title:                    "Happy images",
			names:                    []string{"test.png", "testgif.gif"},
			expectImage:              true,
			expectedCreatorID:        th.BasicUser.ID,
			expectedImageWidths:      []int{408, 118},
			expectedImageHeights:     []int{336, 118},
			expectedImageHasPreview:  []bool{true, false},
			expectedImageMiniPreview: []bool{true, true},
		},
		{
			title:                    "Happy invalid image",
			names:                    []string{"testgif.gif"},
			blobs:                    [][]byte{fileBytes(t, "test-search.md")},
			skipPayloadValidation:    true,
			expectedCreatorID:        th.BasicUser.ID,
			expectedImageMiniPreview: []bool{false},
		},
		// Simple POST, chunked encoding
		{
			title:                    "Happy image chunked post",
			skipMultipart:            true,
			useChunkedInSimplePost:   true,
			names:                    []string{"test.png"},
			expectImage:              true,
			expectedImageWidths:      []int{408},
			expectedImageHeights:     []int{336},
			expectedImageHasPreview:  []bool{true},
			expectedCreatorID:        th.BasicUser.ID,
			expectedImageMiniPreview: []bool{true},
		},
		// Image thumbnail and preview: size and orientation. Note that
		// the expected image dimensions remain the same regardless of the
		// orientation - what we save in FileInfo is used by the
		// clients to size UI elements, so the dimensions are "actual".
		{
			title:                       "Happy image thumbnail/preview 1",
			names:                       []string{"orientation_test_1.jpeg"},
			expectedImageThumbnailNames: []string{"orientation_test_1_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"orientation_test_1_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{2860},
			expectedImageHeights:        []int{1578},
			expectedImageHasPreview:     []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
			expectedImageMiniPreview:    []bool{true},
		},
		{
			title:                       "Happy image thumbnail/preview 2",
			names:                       []string{"orientation_test_2.jpeg"},
			expectedImageThumbnailNames: []string{"orientation_test_2_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"orientation_test_2_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{2860},
			expectedImageHeights:        []int{1578},
			expectedImageHasPreview:     []bool{true},
			expectedImageMiniPreview:    []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		{
			title:                       "Happy image thumbnail/preview 3",
			names:                       []string{"orientation_test_3.jpeg"},
			expectedImageThumbnailNames: []string{"orientation_test_3_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"orientation_test_3_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{2860},
			expectedImageHeights:        []int{1578},
			expectedImageHasPreview:     []bool{true},
			expectedImageMiniPreview:    []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		{
			title:                       "Happy image thumbnail/preview 4",
			names:                       []string{"orientation_test_4.jpeg"},
			expectedImageThumbnailNames: []string{"orientation_test_4_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"orientation_test_4_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{2860},
			expectedImageHeights:        []int{1578},
			expectedImageHasPreview:     []bool{true},
			expectedImageMiniPreview:    []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		{
			title:                       "Happy image thumbnail/preview 5",
			names:                       []string{"orientation_test_5.jpeg"},
			expectedImageThumbnailNames: []string{"orientation_test_5_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"orientation_test_5_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{2860},
			expectedImageHeights:        []int{1578},
			expectedImageHasPreview:     []bool{true},
			expectedImageMiniPreview:    []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		{
			title:                       "Happy image thumbnail/preview 6",
			names:                       []string{"orientation_test_6.jpeg"},
			expectedImageThumbnailNames: []string{"orientation_test_6_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"orientation_test_6_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{2860},
			expectedImageHeights:        []int{1578},
			expectedImageHasPreview:     []bool{true},
			expectedImageMiniPreview:    []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		{
			title:                       "Happy image thumbnail/preview 7",
			names:                       []string{"orientation_test_7.jpeg"},
			expectedImageThumbnailNames: []string{"orientation_test_7_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"orientation_test_7_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{2860},
			expectedImageHeights:        []int{1578},
			expectedImageHasPreview:     []bool{true},
			expectedImageMiniPreview:    []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		{
			title:                       "Happy image thumbnail/preview 8",
			names:                       []string{"orientation_test_8.jpeg"},
			expectedImageThumbnailNames: []string{"orientation_test_8_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"orientation_test_8_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{2860},
			expectedImageHeights:        []int{1578},
			expectedImageHasPreview:     []bool{true},
			expectedImageMiniPreview:    []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		// TIFF preview test
		{
			title:                       "Happy image thumbnail/preview 9",
			names:                       []string{"test.tiff"},
			expectedImageThumbnailNames: []string{"test_expected_tiff_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"test_expected_tiff_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{701},
			expectedImageHeights:        []int{701},
			expectedImageHasPreview:     []bool{true},
			expectedImageMiniPreview:    []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		// Extremely wide image test
		{
			title:                       "Happy image thumbnail/preview 10",
			names:                       []string{"10000x1.png"},
			expectedImageThumbnailNames: []string{"10000x1_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"10000x1_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{10000},
			expectedImageHeights:        []int{1},
			expectedImageHasPreview:     []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		// Extremely high image test
		{
			title:                       "Happy image thumbnail/preview 11",
			names:                       []string{"1x10000.png"},
			expectedImageThumbnailNames: []string{"1x10000_expected_thumb.jpeg"},
			expectedImagePreviewNames:   []string{"1x10000_expected_preview.jpeg"},
			expectImage:                 true,
			expectedImageWidths:         []int{1},
			expectedImageHeights:        []int{10000},
			expectedImageHasPreview:     []bool{true},
			expectedCreatorID:           th.BasicUser.ID,
		},
		// animated GIF
		{
			title:                       "Happy image thumbnail/preview 12",
			names:                       []string{"testgif.gif"},
			expectedImageThumbnailNames: []string{"testgif_expected_thumbnail.jpg"},
			expectedImagePreviewNames:   []string{"testgif_expected_preview.jpg"},
			expectImage:                 true,
			expectedImageWidths:         []int{118},
			expectedImageHeights:        []int{118},
			expectedImageHasPreview:     []bool{false},
			expectedCreatorID:           th.BasicUser.ID,
		},
		{
			title:                    "Happy admin",
			client:                   th.SystemAdminClient,
			names:                    []string{"test.png"},
			expectedImageMiniPreview: []bool{true},
			expectedCreatorID:        th.SystemAdminUser.ID,
		},
		{
			title:                  "Happy stream",
			useChunkedInSimplePost: true,
			skipPayloadValidation:  true,
			names:                  []string{"1Mb-stream"},
			blobs:                  [][]byte{randomBytes(t, 1024*1024)},
			setupConfig: func(a *app.App) func(a *app.App) {
				maxFileSize := *a.Config().FileSettings.MaxFileSize
				a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = 1024 * 1024 })
				return func(a *app.App) {
					a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = maxFileSize })
				}
			},
			expectedImageMiniPreview: []bool{false},
			expectedCreatorID:        th.BasicUser.ID,
		},
		// Error cases
		{
			title:                 "Error channel_id does not exist",
			channelID:             model.NewID(),
			names:                 []string{"test.png"},
			skipSuccessValidation: true,
			checkResponse:         CheckForbiddenStatus,
		},
		{
			// on simple post this uploads the last file
			// successfully, without a ClientId
			title:                    "Error too few client_ids",
			skipSimplePost:           true,
			names:                    []string{"test.png", "testplugin.tar.gz", "test-search.md"},
			clientIDs:                []string{"1", "4"},
			expectedImageMiniPreview: []bool{true, false, false},
			skipSuccessValidation:    true,
			checkResponse:            CheckBadRequestStatus,
		},
		{
			title:                    "Error invalid channel_id",
			channelID:                "../../junk",
			names:                    []string{"test.png"},
			expectedImageMiniPreview: []bool{true},
			skipSuccessValidation:    true,
			checkResponse:            CheckBadRequestStatus,
		},
		{
			title:                 "Error admin channel_id does not exist",
			client:                th.SystemAdminClient,
			channelID:             model.NewID(),
			names:                 []string{"test.png"},
			skipSuccessValidation: true,
			checkResponse:         CheckForbiddenStatus,
		},
		{
			title:                 "Error admin invalid channel_id",
			client:                th.SystemAdminClient,
			channelID:             "../../junk",
			names:                 []string{"test.png"},
			skipSuccessValidation: true,
			checkResponse:         CheckBadRequestStatus,
		},
		{
			title:                 "Error admin disabled uploads",
			client:                th.SystemAdminClient,
			names:                 []string{"test.png"},
			skipSuccessValidation: true,
			checkResponse:         CheckNotImplementedStatus,
			setupConfig: func(a *app.App) func(a *app.App) {
				enableFileAttachments := *a.Config().FileSettings.EnableFileAttachments
				a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.EnableFileAttachments = false })
				return func(a *app.App) {
					a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.EnableFileAttachments = enableFileAttachments })
				}
			},
		},
		{
			title:                 "Error file too large",
			names:                 []string{"test.png"},
			skipSuccessValidation: true,
			checkResponse:         CheckRequestEntityTooLargeStatus,
			setupConfig: func(a *app.App) func(a *app.App) {
				maxFileSize := *a.Config().FileSettings.MaxFileSize
				a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = 279590 })
				return func(a *app.App) {
					a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = maxFileSize })
				}
			},
		},
		// File too large (chunked, simple POST only, multipart would've been redundant with above)
		{
			title:                    "File too large chunked",
			useChunkedInSimplePost:   true,
			skipMultipart:            true,
			names:                    []string{"test.png"},
			skipSuccessValidation:    true,
			checkResponse:            CheckRequestEntityTooLargeStatus,
			expectedImageMiniPreview: []bool{false},
			setupConfig: func(a *app.App) func(a *app.App) {
				maxFileSize := *a.Config().FileSettings.MaxFileSize
				a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = 279590 })
				return func(a *app.App) {
					a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = maxFileSize })
				}
			},
		},
		{
			title:                 "Error stream too large",
			skipPayloadValidation: true,
			names:                 []string{"1Mb-stream"},
			blobs:                 [][]byte{randomBytes(t, 1024*1024)},
			skipSuccessValidation: true,
			checkResponse:         CheckRequestEntityTooLargeStatus,
			setupConfig: func(a *app.App) func(a *app.App) {
				maxFileSize := *a.Config().FileSettings.MaxFileSize
				a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = 10 * 1024 })
				return func(a *app.App) {
					a.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = maxFileSize })
				}
			},
		},
	}

	for _, useMultipart := range []bool{true, false} {
		for _, tc := range tests {
			if tc.skipMultipart && useMultipart || tc.skipSimplePost && !useMultipart {
				continue
			}

			title := ""
			if useMultipart {
				title = "multipart "
			} else {
				title = "simple "
			}
			if tc.title != "" {
				title += tc.title + " "
			}
			title += fmt.Sprintf("%v", tc.names)

			t.Run(title, func(t *testing.T) {
				// Apply any necessary config changes
				if tc.setupConfig != nil {
					restoreConfig := tc.setupConfig(th.App)
					if restoreConfig != nil {
						defer restoreConfig(th.App)
					}
				}

				// Set the default values
				client := th.Client
				if tc.client != nil {
					client = tc.client
				}
				channelID := channel.ID
				if tc.channelID != "" {
					channelID = tc.channelID
				}

				blobs := tc.blobs
				if len(blobs) == 0 {
					for _, name := range tc.names {
						blobs = append(blobs, fileBytes(t, name))
					}
				}

				var fileResp *model.FileUploadResponse
				var resp *model.Response
				if useMultipart {
					fileResp, resp = testUploadFilesMultipart(t, client, channelID, tc.names, blobs, tc.clientIDs)
				} else {
					fileResp, resp = testUploadFilesPost(t, client, channelID, tc.names, blobs, tc.clientIDs, tc.useChunkedInSimplePost)
				}

				if tc.checkResponse != nil {
					tc.checkResponse(t, resp)
				} else {
					if resp != nil {
						require.Nil(t, resp.Error)
					}
				}
				if tc.skipSuccessValidation {
					return
				}

				require.NotNil(t, fileResp, "Nil fileResp")
				require.NotEqual(t, 0, len(fileResp.FileInfos), "Empty FileInfos")
				require.Equal(t, len(tc.names), len(fileResp.FileInfos), "Mismatched actual or expected FileInfos")

				for i, ri := range fileResp.FileInfos {
					// The returned file info from the upload call will be missing some fields that will be stored in the database
					assert.Equal(t, ri.CreatorID, tc.expectedCreatorID, "File should be assigned to user")
					assert.Equal(t, ri.PostID, "", "File shouldn't have a post Id")
					assert.Equal(t, ri.Path, "", "File path should not be set on returned info")
					assert.Equal(t, ri.ThumbnailPath, "", "File thumbnail path should not be set on returned info")
					assert.Equal(t, ri.PreviewPath, "", "File preview path should not be set on returned info")
					if len(tc.expectedImageMiniPreview) == len(fileResp.FileInfos) {
						assert.Equal(t, ri.MiniPreview != nil, tc.expectedImageMiniPreview[i], "File: %s mini preview state unexpected", tc.names[i])
					}
					if len(tc.clientIDs) > i {
						assert.True(t, len(fileResp.ClientIDs) == len(tc.clientIDs),
							fmt.Sprintf("Wrong number of clientIds returned, expected %v, got %v", len(tc.clientIDs), len(fileResp.ClientIDs)))
						assert.Equal(t, fileResp.ClientIDs[i], tc.clientIDs[i],
							fmt.Sprintf("Wrong clientId returned, expected %v, got %v", tc.clientIDs[i], fileResp.ClientIDs[i]))
					}

					dbInfo, err := th.App.Srv().Store.FileInfo().Get(ri.ID)
					require.NoError(t, err)
					assert.Equal(t, dbInfo.ID, ri.ID, "File id from response should match one stored in database")
					assert.Equal(t, dbInfo.CreatorID, tc.expectedCreatorID, "F ile should be assigned to user")
					assert.Equal(t, dbInfo.PostID, "", "File shouldn't have a post")
					assert.NotEqual(t, dbInfo.Path, "", "File path should be set in database")
					_, fname := filepath.Split(dbInfo.Path)
					ext := filepath.Ext(fname)
					name := fname[:len(fname)-len(ext)]
					expectedDir := fmt.Sprintf("%v/teams/%v/channels/%v/users/%s/%s", date, FileTeamID, channel.ID, ri.CreatorID, ri.ID)
					expectedPath := fmt.Sprintf("%s/%s", expectedDir, fname)
					assert.Equal(t, dbInfo.Path, expectedPath,
						fmt.Sprintf("File %v saved to:%q, expected:%q", dbInfo.Name, dbInfo.Path, expectedPath))

					if tc.expectImage {
						expectedThumbnailPath := fmt.Sprintf("%s/%s_thumb.jpg", expectedDir, name)
						expectedPreviewPath := fmt.Sprintf("%s/%s_preview.jpg", expectedDir, name)
						assert.Equal(t, dbInfo.ThumbnailPath, expectedThumbnailPath,
							fmt.Sprintf("Thumbnail for %v saved to:%q, expected:%q", dbInfo.Name, dbInfo.ThumbnailPath, expectedThumbnailPath))
						assert.Equal(t, dbInfo.PreviewPath, expectedPreviewPath,
							fmt.Sprintf("Preview for %v saved to:%q, expected:%q", dbInfo.Name, dbInfo.PreviewPath, expectedPreviewPath))

						assert.True(t,
							dbInfo.HasPreviewImage == tc.expectedImageHasPreview[i],
							fmt.Sprintf("Image: HasPreviewImage should be set for %s", dbInfo.Name))
						assert.True(t,
							dbInfo.Width == tc.expectedImageWidths[i] && dbInfo.Height == tc.expectedImageHeights[i],
							fmt.Sprintf("Image dimensions: expected %dwx%dh, got %vwx%dh",
								tc.expectedImageWidths[i], tc.expectedImageHeights[i],
								dbInfo.Width, dbInfo.Height))
					}

					if !tc.skipPayloadValidation {
						compare := func(get func(string) ([]byte, *model.Response), name string) {
							data, resp := get(ri.ID)
							require.NotNil(t, resp)
							require.Nil(t, resp.Error)

							expected, err := ioutil.ReadFile(filepath.Join(testDir, name))
							require.NoError(t, err)
							if !bytes.Equal(data, expected) {
								tf, err := ioutil.TempFile("", fmt.Sprintf("test_%v_*_%s", i, name))
								defer tf.Close()
								require.NoError(t, err)
								_, err = io.Copy(tf, bytes.NewReader(data))
								require.NoError(t, err)
								t.Errorf("Actual data mismatched %s, written to %q - expected %d bytes, got %d.", name, tf.Name(), len(expected), len(data))
							}
						}
						if len(tc.expectedPayloadNames) == 0 {
							tc.expectedPayloadNames = tc.names
						}

						compare(client.GetFile, tc.expectedPayloadNames[i])
						if len(tc.expectedImageThumbnailNames) > i {
							compare(client.GetFileThumbnail, tc.expectedImageThumbnailNames[i])
						}
						if len(tc.expectedImageThumbnailNames) > i {
							compare(client.GetFilePreview, tc.expectedImagePreviewNames[i])
						}
					}

					th.cleanupTestFile(dbInfo)
				}
			})

		}
	}
}

func TestGetFile(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	if *th.App.Config().FileSettings.DriverName == "" {
		t.Skip("skipping because no file driver is enabled")
	}

	sent, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	fileResp, resp := Client.UploadFile(sent, channel.ID, "test.png")
	CheckNoError(t, resp)

	fileID := fileResp.FileInfos[0].ID

	data, resp := Client.GetFile(fileID)
	CheckNoError(t, resp)
	require.NotEqual(t, 0, len(data), "should not be empty")

	for i := range data {
		require.Equal(t, sent[i], data[i], "received file didn't match sent one")
	}

	_, resp = Client.GetFile("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFile(model.NewID())
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetFile(fileID)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetFile(fileID)
	CheckNoError(t, resp)
}

func TestGetFileHeaders(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	channel := th.BasicChannel

	if *th.App.Config().FileSettings.DriverName == "" {
		t.Skip("skipping because no file driver is enabled")
	}

	testHeaders := func(data []byte, filename string, expectedContentType string, getInline bool) func(*testing.T) {
		return func(t *testing.T) {
			fileResp, resp := Client.UploadFile(data, channel.ID, filename)
			CheckNoError(t, resp)

			fileID := fileResp.FileInfos[0].ID

			_, resp = Client.GetFile(fileID)
			CheckNoError(t, resp)

			CheckStartsWith(t, resp.Header.Get("Content-Type"), expectedContentType, "returned incorrect Content-Type")

			if getInline {
				CheckStartsWith(t, resp.Header.Get("Content-Disposition"), "inline", "returned incorrect Content-Disposition")
			} else {
				CheckStartsWith(t, resp.Header.Get("Content-Disposition"), "attachment", "returned incorrect Content-Disposition")
			}

			_, resp = Client.DownloadFile(fileID, true)
			CheckNoError(t, resp)

			CheckStartsWith(t, resp.Header.Get("Content-Type"), expectedContentType, "returned incorrect Content-Type")
			CheckStartsWith(t, resp.Header.Get("Content-Disposition"), "attachment", "returned incorrect Content-Disposition")
		}
	}

	data := []byte("ABC")

	t.Run("png", testHeaders(data, "test.png", "image/png", true))
	t.Run("gif", testHeaders(data, "test.gif", "image/gif", true))
	t.Run("mp4", testHeaders(data, "test.mp4", "video/mp4", true))
	t.Run("mp3", testHeaders(data, "test.mp3", "audio/mpeg", true))
	t.Run("pdf", testHeaders(data, "test.pdf", "application/pdf", false))
	t.Run("txt", testHeaders(data, "test.txt", "text/plain", false))
	t.Run("html", testHeaders(data, "test.html", "text/plain", false))
	t.Run("js", testHeaders(data, "test.js", "text/plain", false))
	t.Run("go", testHeaders(data, "test.go", "application/octet-stream", false))
	t.Run("zip", testHeaders(data, "test.zip", "application/zip", false))
	// Not every platform can recognize these
	//t.Run("exe", testHeaders(data, "test.exe", "application/x-ms", false))
	t.Run("no extension", testHeaders(data, "test", "application/octet-stream", false))
	t.Run("no extension 2", testHeaders([]byte("<html></html>"), "test", "application/octet-stream", false))
}

func TestGetFileThumbnail(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	if *th.App.Config().FileSettings.DriverName == "" {
		t.Skip("skipping because no file driver is enabled")
	}

	sent, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	fileResp, resp := Client.UploadFile(sent, channel.ID, "test.png")
	CheckNoError(t, resp)

	fileID := fileResp.FileInfos[0].ID

	data, resp := Client.GetFileThumbnail(fileID)
	CheckNoError(t, resp)
	require.NotEqual(t, 0, len(data), "should not be empty")

	_, resp = Client.GetFileThumbnail("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFileThumbnail(model.NewID())
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetFileThumbnail(fileID)
	CheckUnauthorizedStatus(t, resp)

	otherUser := th.CreateUser()
	Client.Login(otherUser.Email, otherUser.Password)
	_, resp = Client.GetFileThumbnail(fileID)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = th.SystemAdminClient.GetFileThumbnail(fileID)
	CheckNoError(t, resp)
}

func TestGetFileLink(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	if *th.App.Config().FileSettings.DriverName == "" {
		t.Skip("skipping because no file driver is enabled")
	}

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.EnablePublicLink = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.PublicLinkSalt = model.NewRandomString(32) })

	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	fileResp, uploadResp := Client.UploadFile(data, channel.ID, "test.png")
	CheckNoError(t, uploadResp)

	fileID := fileResp.FileInfos[0].ID

	_, resp := Client.GetFileLink(fileID)
	CheckBadRequestStatus(t, resp)

	// Hacky way to assign file to a post (usually would be done by CreatePost call)
	err = th.App.Srv().Store.FileInfo().AttachToPost(fileID, th.BasicPost.ID, th.BasicUser.ID)
	require.NoError(t, err)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.EnablePublicLink = false })
	_, resp = Client.GetFileLink(fileID)
	CheckNotImplementedStatus(t, resp)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.EnablePublicLink = true })
	link, resp := Client.GetFileLink(fileID)
	CheckNoError(t, resp)
	require.NotEqual(t, "", link, "should've received public link")

	_, resp = Client.GetFileLink("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFileLink(model.NewID())
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetFileLink(fileID)
	CheckUnauthorizedStatus(t, resp)

	otherUser := th.CreateUser()
	Client.Login(otherUser.Email, otherUser.Password)
	_, resp = Client.GetFileLink(fileID)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = th.SystemAdminClient.GetFileLink(fileID)
	CheckNoError(t, resp)

	fileInfo, err := th.App.Srv().Store.FileInfo().Get(fileID)
	require.NoError(t, err)
	th.cleanupTestFile(fileInfo)
}

func TestGetFilePreview(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	if *th.App.Config().FileSettings.DriverName == "" {
		t.Skip("skipping because no file driver is enabled")
	}

	sent, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	fileResp, resp := Client.UploadFile(sent, channel.ID, "test.png")
	CheckNoError(t, resp)
	fileID := fileResp.FileInfos[0].ID

	data, resp := Client.GetFilePreview(fileID)
	CheckNoError(t, resp)
	require.NotEqual(t, 0, len(data), "should not be empty")

	_, resp = Client.GetFilePreview("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFilePreview(model.NewID())
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetFilePreview(fileID)
	CheckUnauthorizedStatus(t, resp)

	otherUser := th.CreateUser()
	Client.Login(otherUser.Email, otherUser.Password)
	_, resp = Client.GetFilePreview(fileID)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = th.SystemAdminClient.GetFilePreview(fileID)
	CheckNoError(t, resp)
}

func TestGetFileInfo(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	channel := th.BasicChannel

	if *th.App.Config().FileSettings.DriverName == "" {
		t.Skip("skipping because no file driver is enabled")
	}

	sent, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	fileResp, resp := Client.UploadFile(sent, channel.ID, "test.png")
	CheckNoError(t, resp)
	fileID := fileResp.FileInfos[0].ID

	info, resp := Client.GetFileInfo(fileID)
	CheckNoError(t, resp)

	require.NoError(t, err)
	require.Equal(t, fileID, info.ID, "got incorrect file")
	require.Equal(t, user.ID, info.CreatorID, "file should be assigned to user")
	require.Equal(t, "", info.PostID, "file shouldn't have a post")
	require.Equal(t, "", info.Path, "file path shouldn't have been returned to client")
	require.Equal(t, "", info.ThumbnailPath, "file thumbnail path shouldn't have been returned to client")
	require.Equal(t, "", info.PreviewPath, "file preview path shouldn't have been returned to client")
	require.Equal(t, "image/png", info.MimeType, "mime type should've been image/png")

	_, resp = Client.GetFileInfo("junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetFileInfo(model.NewID())
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetFileInfo(fileID)
	CheckUnauthorizedStatus(t, resp)

	otherUser := th.CreateUser()
	Client.Login(otherUser.Email, otherUser.Password)
	_, resp = Client.GetFileInfo(fileID)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = th.SystemAdminClient.GetFileInfo(fileID)
	CheckNoError(t, resp)
}

func TestGetPublicFile(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.EnablePublicLink = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.PublicLinkSalt = model.NewRandomString(32) })

	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	fileResp, httpResp := Client.UploadFile(data, channel.ID, "test.png")
	CheckNoError(t, httpResp)

	fileID := fileResp.FileInfos[0].ID

	// Hacky way to assign file to a post (usually would be done by CreatePost call)
	err = th.App.Srv().Store.FileInfo().AttachToPost(fileID, th.BasicPost.ID, th.BasicUser.ID)
	require.NoError(t, err)

	info, err := th.App.Srv().Store.FileInfo().Get(fileID)
	require.NoError(t, err)
	link := th.App.GeneratePublicLink(Client.URL, info)

	resp, err := http.Get(link)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "failed to get image with public link")

	resp, err = http.Get(link[:strings.LastIndex(link, "?")])
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "should've failed to get image with public link without hash", resp.Status)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.EnablePublicLink = false })

	resp, err = http.Get(link)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode, "should've failed to get image with disabled public link")

	// test after the salt has changed
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.EnablePublicLink = true })
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.PublicLinkSalt = model.NewRandomString(32) })

	resp, err = http.Get(link)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "should've failed to get image with public link after salt changed")

	resp, err = http.Get(link)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "should've failed to get image with public link after salt changed")

	fileInfo, err := th.App.Srv().Store.FileInfo().Get(fileID)
	require.NoError(t, err)
	require.NoError(t, th.cleanupTestFile(fileInfo))

	th.cleanupTestFile(info)
	link = th.App.GeneratePublicLink(Client.URL, info)
	resp, err = http.Get(link)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "should've failed to get file after it is deleted")
}

func TestSearchFiles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	experimentalViewArchivedChannels := *th.App.Config().TeamSettings.ExperimentalViewArchivedChannels
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.TeamSettings.ExperimentalViewArchivedChannels = &experimentalViewArchivedChannels
		})
	}()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.ExperimentalViewArchivedChannels = true
	})
	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	th.LoginBasic()
	Client := th.Client

	filename := "search for fileInfo1"
	fileInfo1, appErr := th.App.UploadFile(th.Context, data, th.BasicChannel.ID, filename)
	require.Nil(t, appErr)
	err = th.App.Srv().Store.FileInfo().AttachToPost(fileInfo1.ID, th.BasicPost.ID, th.BasicUser.ID)
	require.NoError(t, err)

	filename = "search for fileInfo2"
	fileInfo2, appErr := th.App.UploadFile(th.Context, data, th.BasicChannel.ID, filename)
	require.Nil(t, appErr)
	err = th.App.Srv().Store.FileInfo().AttachToPost(fileInfo2.ID, th.BasicPost.ID, th.BasicUser.ID)
	require.NoError(t, err)

	filename = "tagged search for fileInfo3"
	fileInfo3, appErr := th.App.UploadFile(th.Context, data, th.BasicChannel.ID, filename)
	require.Nil(t, appErr)
	err = th.App.Srv().Store.FileInfo().AttachToPost(fileInfo3.ID, th.BasicPost.ID, th.BasicUser.ID)
	require.NoError(t, err)

	filename = "tagged for fileInfo4"
	fileInfo4, appErr := th.App.UploadFile(th.Context, data, th.BasicChannel.ID, filename)
	require.Nil(t, appErr)
	err = th.App.Srv().Store.FileInfo().AttachToPost(fileInfo4.ID, th.BasicPost.ID, th.BasicUser.ID)
	require.NoError(t, err)

	archivedChannel := th.CreatePublicChannel()
	fileInfo5, appErr := th.App.UploadFile(th.Context, data, archivedChannel.ID, "tagged for fileInfo3")
	require.Nil(t, appErr)
	post := &model.Post{ChannelID: archivedChannel.ID, Message: model.NewID() + "a"}
	rpost, resp := Client.CreatePost(post)
	CheckNoError(t, resp)
	err = th.App.Srv().Store.FileInfo().AttachToPost(fileInfo5.ID, rpost.ID, th.BasicUser.ID)
	require.NoError(t, err)
	th.Client.DeleteChannel(archivedChannel.ID)

	terms := "search"
	isOrSearch := false
	timezoneOffset := 5
	searchParams := model.SearchParameter{
		Terms:          &terms,
		IsOrSearch:     &isOrSearch,
		TimeZoneOffset: &timezoneOffset,
	}
	fileInfos, resp := Client.SearchFilesWithParams(th.BasicTeam.ID, &searchParams)
	CheckNoError(t, resp)
	require.Len(t, fileInfos.Order, 3, "wrong search")

	terms = "search"
	page := 0
	perPage := 2
	searchParams = model.SearchParameter{
		Terms:          &terms,
		IsOrSearch:     &isOrSearch,
		TimeZoneOffset: &timezoneOffset,
		Page:           &page,
		PerPage:        &perPage,
	}
	fileInfos2, resp := Client.SearchFilesWithParams(th.BasicTeam.ID, &searchParams)
	CheckNoError(t, resp)
	// We don't support paging for DB search yet, modify this when we do.
	require.Len(t, fileInfos2.Order, 3, "Wrong number of fileInfos")
	assert.Equal(t, fileInfos.Order[0], fileInfos2.Order[0])
	assert.Equal(t, fileInfos.Order[1], fileInfos2.Order[1])

	page = 1
	searchParams = model.SearchParameter{
		Terms:          &terms,
		IsOrSearch:     &isOrSearch,
		TimeZoneOffset: &timezoneOffset,
		Page:           &page,
		PerPage:        &perPage,
	}
	fileInfos2, resp = Client.SearchFilesWithParams(th.BasicTeam.ID, &searchParams)
	CheckNoError(t, resp)
	// We don't support paging for DB search yet, modify this when we do.
	require.Empty(t, fileInfos2.Order, "Wrong number of fileInfos")

	fileInfos, resp = Client.SearchFiles(th.BasicTeam.ID, "search", false)
	CheckNoError(t, resp)
	require.Len(t, fileInfos.Order, 3, "wrong search")

	fileInfos, resp = Client.SearchFiles(th.BasicTeam.ID, "fileInfo2", false)
	CheckNoError(t, resp)
	require.Len(t, fileInfos.Order, 1, "wrong number of fileInfos")
	require.Equal(t, fileInfo2.ID, fileInfos.Order[0], "wrong search")

	terms = "tagged"
	includeDeletedChannels := true
	searchParams = model.SearchParameter{
		Terms:                  &terms,
		IsOrSearch:             &isOrSearch,
		TimeZoneOffset:         &timezoneOffset,
		IncludeDeletedChannels: &includeDeletedChannels,
	}
	fileInfos, resp = Client.SearchFilesWithParams(th.BasicTeam.ID, &searchParams)
	CheckNoError(t, resp)
	require.Len(t, fileInfos.Order, 3, "wrong search")

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TeamSettings.ExperimentalViewArchivedChannels = false
	})

	fileInfos, resp = Client.SearchFilesWithParams(th.BasicTeam.ID, &searchParams)
	CheckNoError(t, resp)
	require.Len(t, fileInfos.Order, 2, "wrong search")

	fileInfos, _ = Client.SearchFiles(th.BasicTeam.ID, "*", false)
	require.Empty(t, fileInfos.Order, "searching for just * shouldn't return any results")

	fileInfos, resp = Client.SearchFiles(th.BasicTeam.ID, "fileInfo1 fileInfo2", true)
	CheckNoError(t, resp)
	require.Len(t, fileInfos.Order, 2, "wrong search results")

	_, resp = Client.SearchFiles("junk", "#sgtitlereview", false)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.SearchFiles(model.NewID(), "#sgtitlereview", false)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.SearchFiles(th.BasicTeam.ID, "", false)
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.SearchFiles(th.BasicTeam.ID, "#sgtitlereview", false)
	CheckUnauthorizedStatus(t, resp)
}
