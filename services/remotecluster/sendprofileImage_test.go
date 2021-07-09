// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package remotecluster

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	imageWidth  = 128
	imageHeight = 128
)

func TestService_sendProfileImageToRemote(t *testing.T) {
	hadPing := disablePing
	disablePing = true
	defer func() { disablePing = hadPing }()

	shouldError := &flag{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer io.Copy(ioutil.Discard, r.Body)

		if shouldError.get() {
			w.WriteHeader(http.StatusInternalServerError)
			resp := make(map[string]string)
			resp[model.STATUS] = model.StatusFail
			w.Write([]byte(model.MapToJSON(resp)))
			return
		}

		status := model.StatusOk
		defer func(s *string) {
			if *s != model.StatusOk {
				w.WriteHeader(http.StatusInternalServerError)
			}
			resp := make(map[string]string)
			resp[model.STATUS] = *s
			w.Write([]byte(model.MapToJSON(resp)))
		}(&status)

		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			status = model.StatusFail
			assert.Fail(t, "connect parse multipart form", err)
			return
		}
		m := r.MultipartForm
		if m == nil {
			status = model.StatusFail
			assert.Fail(t, "multipart form missing")
			return
		}

		imageArray, ok := m.File["image"]
		if !ok || len(imageArray) != 1 {
			status = model.StatusFail
			assert.Fail(t, "image missing")
			return
		}

		imageData := imageArray[0]
		file, err := imageData.Open()
		if err != nil {
			status = model.StatusFail
			assert.Fail(t, "cannot open multipart form file")
			return
		}
		defer file.Close()

		img, err := png.Decode(file)
		if err != nil || imageWidth != img.Bounds().Max.X || imageHeight != img.Bounds().Max.Y {
			status = model.StatusFail
			assert.Fail(t, "cannot decode png", err)
			return
		}
	}))
	defer ts.Close()

	rc := makeRemoteCluster("remote_test_profile_image", ts.URL, TestTopics)

	user := &model.User{
		ID:       model.NewID(),
		RemoteID: model.NewString(rc.RemoteID),
	}

	provider := testImageProvider{}

	mockServer := newMockServer(t, makeRemoteClusters(NumRemotes, ts.URL))
	mockServer.SetUser(user)
	service, err := NewRemoteClusterService(mockServer)
	require.NoError(t, err)

	err = service.Start()
	require.NoError(t, err)
	defer service.Shutdown()

	t.Run("Server response 200", func(t *testing.T) {
		shouldError.set(false)

		resultFunc := func(userID string, rc *model.RemoteCluster, resp *Response, err error) {
			assert.Equal(t, user.ID, userID, "user ids should match")
			assert.NoError(t, err)
			assert.True(t, resp.IsSuccess())
		}

		task := sendProfileImageTask{
			rc:       rc,
			userID:   user.ID,
			provider: provider,
			f:        resultFunc,
		}

		err := service.sendProfileImageToRemote(time.Second*15, task)
		assert.NoError(t, err, "request should not error")
	})

	t.Run("Server response 500", func(t *testing.T) {
		shouldError.set(true)

		resultFunc := func(userID string, rc *model.RemoteCluster, resp *Response, err error) {
			assert.Equal(t, user.ID, userID, "user ids should match")
			assert.False(t, resp.IsSuccess())
		}

		task := sendProfileImageTask{
			rc:       rc,
			userID:   user.ID,
			provider: provider,
			f:        resultFunc,
		}

		err := service.sendProfileImageToRemote(time.Second*15, task)
		assert.Error(t, err, "request should error")
	})
}

type testImageProvider struct {
}

func (tip testImageProvider) GetProfileImage(user *model.User) ([]byte, bool, *model.AppError) {
	img := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{imageWidth, imageHeight}})
	red := color.RGBA{255, 50, 50, 0xff}

	for x := 0; x < imageWidth; x++ {
		for y := 0; y < imageHeight; y++ {
			img.Set(x, y, red)
		}
	}

	buf := &bytes.Buffer{}
	png.Encode(buf, img)

	return buf.Bytes(), true, nil
}

type flag struct {
	mux sync.RWMutex
	b   bool
}

func (f *flag) get() bool {
	f.mux.RLock()
	defer f.mux.RUnlock()
	return f.b
}

func (f *flag) set(b bool) {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.b = b
}
