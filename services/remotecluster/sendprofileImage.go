// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package remotecluster

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type SendProfileImageResultFunc func(userID string, rc *model.RemoteCluster, resp *Response, err error)

type sendProfileImageTask struct {
	rc       *model.RemoteCluster
	userID   string
	provider ProfileImageProvider
	f        SendProfileImageResultFunc
}

type ProfileImageProvider interface {
	GetProfileImage(user *model.User) ([]byte, bool, *model.AppError)
}

// SendProfileImage asynchronously sends a user's profile image to a remote cluster.
//
// `ctx` determines behaviour when the outbound queue is full. A timeout or deadline context will return a
// BufferFullError if the task cannot be enqueued before the timeout. A background context will block indefinitely.
//
// Nil or error return indicates success or failure of task enqueue only.
//
// An optional callback can be provided that receives the response from the remote cluster. The `err` provided to the
// callback is regarding image delivery only. The `resp` contains the decoded bytes returned from the remote.
// If a callback is provided it should return quickly.
func (rcs *Service) SendProfileImage(ctx context.Context, userID string, rc *model.RemoteCluster, provider ProfileImageProvider, f SendProfileImageResultFunc) error {
	task := sendProfileImageTask{
		rc:       rc,
		userID:   userID,
		provider: provider,
		f:        f,
	}
	return rcs.enqueueTask(ctx, rc.RemoteID, task)
}

// sendProfileImage is called when a sendProfileImageTask is popped from the send channel.
func (rcs *Service) sendProfileImage(task sendProfileImageTask) {
	err := rcs.sendProfileImageToRemote(SendTimeout, task)
	var response Response

	if err != nil {
		rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceError, "Remote Cluster send profile image failed",
			mlog.String("remote", task.rc.DisplayName),
			mlog.String("UserId", task.userID),
			mlog.Err(err),
		)
		response.Status = ResponseStatusFail
		response.Err = err.Error()
	} else {
		rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceDebug, "Remote Cluster profile image sent successfully",
			mlog.String("remote", task.rc.DisplayName),
			mlog.String("UserId", task.userID),
		)
		response.Status = ResponseStatusOK
	}

	// If callback provided then call it with the results.
	if task.f != nil {
		task.f(task.userID, task.rc, &response, err)
	}
}

func (rcs *Service) sendProfileImageToRemote(timeout time.Duration, task sendProfileImageTask) error {
	rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceDebug, "sending profile image to remote...",
		mlog.String("remote", task.rc.DisplayName),
		mlog.String("UserId", task.userID),
	)

	user, err := rcs.server.GetStore().User().Get(context.Background(), task.userID)
	if err != nil {
		return fmt.Errorf("error fetching user while sending profile image to remote %s: %w", task.rc.RemoteID, err)
	}

	img, _, appErr := task.provider.GetProfileImage(user) // get Reader for the file
	if appErr != nil {
		return fmt.Errorf("error fetching profile image for user (%s) while sending to remote %s: %w", task.userID, task.rc.RemoteID, appErr)
	}

	u, err := url.Parse(task.rc.SiteURL)
	if err != nil {
		return fmt.Errorf("invalid siteURL while sending file to remote %s: %w", task.rc.RemoteID, err)
	}
	u.Path = path.Join(u.Path, model.APIURLSuffix, "remotecluster", task.userID, "image")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "profile.png")
	if err != nil {
		return err
	}

	if _, err = io.Copy(part, bytes.NewBuffer(img)); err != nil {
		return err
	}

	if err = writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set(model.HeaderRemoteclusterID, task.rc.RemoteID)
	req.Header.Set(model.HeaderRemoteclusterToken, task.rc.RemoteToken)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := rcs.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response: %d - %s", resp.StatusCode, resp.Status)
	}
	return nil
}
