// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"io"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func TestCreateUploadSession(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	us := &model.UploadSession{
		Type:      model.UploadTypeAttachment,
		UserID:    th.BasicUser.ID,
		ChannelID: th.BasicChannel.ID,
		Filename:  "upload",
		FileSize:  8 * 1024 * 1024,
	}

	t.Run("FileSize over limit", func(t *testing.T) {
		maxFileSize := *th.App.Config().FileSettings.MaxFileSize
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = us.FileSize - 1 })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.FileSettings.MaxFileSize = maxFileSize })
		u, err := th.App.CreateUploadSession(us)
		require.NotNil(t, err)
		require.Equal(t, "app.upload.create.upload_too_large.app_error", err.ID)
		require.Nil(t, u)
	})

	t.Run("invalid Id", func(t *testing.T) {
		u, err := th.App.CreateUploadSession(us)
		require.NotNil(t, err)
		require.Equal(t, "model.upload_session.is_valid.id.app_error", err.ID)
		require.Nil(t, u)
	})

	t.Run("invalid UserId", func(t *testing.T) {
		us.ID = model.NewID()
		us.UserID = ""
		u, err := th.App.CreateUploadSession(us)
		require.NotNil(t, err)
		require.Equal(t, "model.upload_session.is_valid.user_id.app_error", err.ID)
		require.Nil(t, u)
	})

	t.Run("invalid ChannelId", func(t *testing.T) {
		us.UserID = th.BasicUser.ID
		us.ChannelID = ""
		u, err := th.App.CreateUploadSession(us)
		require.NotNil(t, err)
		require.Equal(t, "model.upload_session.is_valid.channel_id.app_error", err.ID)
		require.Nil(t, u)
	})

	t.Run("non-existing channel", func(t *testing.T) {
		us.ChannelID = model.NewID()
		u, err := th.App.CreateUploadSession(us)
		require.NotNil(t, err)
		require.Equal(t, "app.upload.create.incorrect_channel_id.app_error", err.ID)
		require.Nil(t, u)
	})

	t.Run("deleted channel", func(t *testing.T) {
		ch := th.CreateChannel(th.BasicTeam)
		th.App.DeleteChannel(th.Context, ch, th.BasicUser.ID)
		us.ChannelID = ch.ID
		u, err := th.App.CreateUploadSession(us)
		require.NotNil(t, err)
		require.Equal(t, "app.upload.create.cannot_upload_to_deleted_channel.app_error", err.ID)
		require.Nil(t, u)
	})

	t.Run("success", func(t *testing.T) {
		us.ChannelID = th.BasicChannel.ID
		u, err := th.App.CreateUploadSession(us)
		require.Nil(t, err)
		require.NotEmpty(t, u)
	})
}

func TestUploadData(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	us := &model.UploadSession{
		ID:        model.NewID(),
		Type:      model.UploadTypeAttachment,
		UserID:    th.BasicUser.ID,
		ChannelID: th.BasicChannel.ID,
		Filename:  "upload",
		FileSize:  8 * 1024 * 1024,
	}

	us, uploadSessionAppErr := th.App.CreateUploadSession(us)
	require.Nil(t, uploadSessionAppErr)
	require.NotEmpty(t, us)

	data := make([]byte, us.FileSize)
	_, err2 := rand.Read(data)
	require.NoError(t, err2)

	t.Run("write error", func(t *testing.T) {
		rd := &io.LimitedReader{
			R: bytes.NewReader(data),
			N: 1024 * 1024,
		}

		ok, appErr := th.App.FileExists(us.Path)
		require.False(t, ok)
		require.Nil(t, appErr)

		u := *us
		u.Path = ""
		info, appErr := th.App.UploadData(th.Context, &u, rd)
		require.Nil(t, info)
		require.NotNil(t, appErr)
		require.NotEqual(t, "app.upload.upload_data.first_part_too_small.app_error", appErr.ID)
	})

	t.Run("first part too small", func(t *testing.T) {
		rd := &io.LimitedReader{
			R: bytes.NewReader(data),
			N: 1024 * 1024,
		}

		ok, appErr := th.App.FileExists(us.Path)
		require.False(t, ok)
		require.Nil(t, appErr)

		info, appErr := th.App.UploadData(th.Context, us, rd)
		require.Nil(t, info)
		require.NotNil(t, appErr)
		require.Equal(t, "app.upload.upload_data.first_part_too_small.app_error", appErr.ID)

		ok, appErr = th.App.FileExists(us.Path)
		require.False(t, ok)
		require.Nil(t, appErr)
	})

	t.Run("resume success", func(t *testing.T) {
		rd := &io.LimitedReader{
			R: bytes.NewReader(data),
			N: 5 * 1024 * 1024,
		}
		info, appErr := th.App.UploadData(th.Context, us, rd)
		require.Nil(t, info)
		require.Nil(t, appErr)

		rd = &io.LimitedReader{
			R: bytes.NewReader(data[5*1024*1024:]),
			N: 3 * 1024 * 1024,
		}
		info, appErr = th.App.UploadData(th.Context, us, rd)
		require.Nil(t, appErr)
		require.NotEmpty(t, info)

		d, appErr := th.App.ReadFile(us.Path)
		require.Nil(t, appErr)
		require.Equal(t, data, d)
	})

	t.Run("all at once success", func(t *testing.T) {
		us.ID = model.NewID()
		var appErr *model.AppError
		us, appErr = th.App.CreateUploadSession(us)
		require.Nil(t, appErr)
		require.NotEmpty(t, us)

		info, appErr := th.App.UploadData(th.Context, us, bytes.NewReader(data))
		require.Nil(t, appErr)
		require.NotEmpty(t, info)

		d, appErr := th.App.ReadFile(us.Path)
		require.Nil(t, appErr)
		require.Equal(t, data, d)
	})

	t.Run("small file success", func(t *testing.T) {
		us.ID = model.NewID()
		us.FileSize = 1024 * 1024
		var appErr *model.AppError
		us, appErr = th.App.CreateUploadSession(us)
		require.Nil(t, appErr)
		require.NotEmpty(t, us)

		rd := &io.LimitedReader{
			R: bytes.NewReader(data),
			N: 1024 * 1024,
		}
		info, appErr := th.App.UploadData(th.Context, us, rd)
		require.Nil(t, appErr)
		require.NotEmpty(t, info)

		d, appErr := th.App.ReadFile(us.Path)
		require.Nil(t, appErr)
		require.Equal(t, data[:1024*1024], d)
	})

	t.Run("image processing", func(t *testing.T) {
		testDir, _ := fileutils.FindDir("tests")
		data, err := ioutil.ReadFile(filepath.Join(testDir, "test.png"))
		require.NoError(t, err)
		require.NotEmpty(t, data)

		us.ID = model.NewID()
		us.Filename = "test.png"
		us.FileSize = int64(len(data))
		var appErr *model.AppError
		us, appErr = th.App.CreateUploadSession(us)
		require.Nil(t, appErr)
		require.NotEmpty(t, us)

		info, appErr := th.App.UploadData(th.Context, us, bytes.NewReader(data))
		require.Nil(t, appErr)
		require.NotEmpty(t, info)
		require.NotZero(t, info.Width)
		require.NotZero(t, info.Height)
		require.NotEmpty(t, info.ThumbnailPath)
		require.NotEmpty(t, info.PreviewPath)
	})
}

func TestUploadDataConcurrent(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	us := &model.UploadSession{
		ID:        model.NewID(),
		Type:      model.UploadTypeAttachment,
		UserID:    th.BasicUser.ID,
		ChannelID: th.BasicChannel.ID,
		Filename:  "upload",
		FileSize:  8 * 1024 * 1024,
	}

	var appErr *model.AppError
	us, appErr = th.App.CreateUploadSession(us)
	require.Nil(t, appErr)
	require.NotEmpty(t, us)

	data := make([]byte, us.FileSize)
	_, err2 := rand.Read(data)
	require.NoError(t, err2)

	var nErrs int32
	var wg sync.WaitGroup
	n := 8
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			rd := &io.LimitedReader{
				R: bytes.NewReader(data),
				N: 5 * 1024 * 1024,
			}
			u := *us
			_, err := th.App.UploadData(th.Context, &u, rd)
			if err != nil && err.ID == "app.upload.upload_data.concurrent.app_error" {
				atomic.AddInt32(&nErrs, 1)
			}
		}()
	}

	wg.Wait()

	// Verify that only 1 request was able to perform the upload.
	require.Equal(t, int32(n-1), nErrs)
	nErrs = 0

	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			rd := &io.LimitedReader{
				R: bytes.NewReader(data[5*1024*1024:]),
				N: 3 * 1024 * 1024,
			}
			u := *us
			u.FileOffset = 5 * 1024 * 1024
			_, err := th.App.UploadData(th.Context, &u, rd)
			if err != nil && err.ID == "app.upload.upload_data.concurrent.app_error" {
				atomic.AddInt32(&nErrs, 1)
			}
		}()
	}

	wg.Wait()

	// Verify that only 1 request was able to finish the upload.
	require.Equal(t, int32(n-1), nErrs)

	d, appErr := th.App.ReadFile(us.Path)
	require.Nil(t, appErr)
	require.Equal(t, data, d)
}
