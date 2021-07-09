// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package remotecluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/filestore"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type SendFileResultFunc func(us *model.UploadSession, rc *model.RemoteCluster, resp *Response, err error)

type sendFileTask struct {
	rc *model.RemoteCluster
	us *model.UploadSession
	fi *model.FileInfo
	rp ReaderProvider
	f  SendFileResultFunc
}

type ReaderProvider interface {
	FileReader(path string) (filestore.ReadCloseSeeker, *model.AppError)
}

// SendFile asynchronously sends a file to a remote cluster.
//
// `ctx` determines behaviour when the outbound queue is full. A timeout or deadline context will return a
// BufferFullError if the task cannot be enqueued before the timeout. A background context will block indefinitely.
//
// Nil or error return indicates success or failure of task enqueue only.
//
// An optional callback can be provided that receives the response from the remote cluster. The `err` provided to the
// callback is regarding file delivery only. The `resp` contains the decoded bytes returned from the remote.
// If a callback is provided it should return quickly.
func (rcs *Service) SendFile(ctx context.Context, us *model.UploadSession, fi *model.FileInfo, rc *model.RemoteCluster, rp ReaderProvider, f SendFileResultFunc) error {
	task := sendFileTask{
		rc: rc,
		us: us,
		fi: fi,
		rp: rp,
		f:  f,
	}
	return rcs.enqueueTask(ctx, rc.RemoteID, task)
}

// sendFile is called when a sendFileTask is popped from the send channel.
func (rcs *Service) sendFile(task sendFileTask) {
	fi, err := rcs.sendFileToRemote(SendTimeout, task)
	var response Response

	if err != nil {
		rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceError, "Remote Cluster send file failed",
			mlog.String("remote", task.rc.DisplayName),
			mlog.String("uploadId", task.us.ID),
			mlog.Err(err),
		)
		response.Status = ResponseStatusFail
		response.Err = err.Error()
	} else {
		rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceDebug, "Remote Cluster file sent successfully",
			mlog.String("remote", task.rc.DisplayName),
			mlog.String("uploadId", task.us.ID),
		)
		response.Status = ResponseStatusOK
		response.SetPayload(fi)
	}

	// If callback provided then call it with the results.
	if task.f != nil {
		task.f(task.us, task.rc, &response, err)
	}
}

func (rcs *Service) sendFileToRemote(timeout time.Duration, task sendFileTask) (*model.FileInfo, error) {
	rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceDebug, "sending file to remote...",
		mlog.String("remote", task.rc.DisplayName),
		mlog.String("uploadId", task.us.ID),
		mlog.String("file_path", task.us.Path),
	)

	r, appErr := task.rp.FileReader(task.fi.Path) // get Reader for the file
	if appErr != nil {
		return nil, fmt.Errorf("error opening file while sending file to remote %s: %w", task.rc.RemoteID, appErr)
	}
	defer r.Close()

	u, err := url.Parse(task.rc.SiteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid siteURL while sending file to remote %s: %w", task.rc.RemoteID, err)
	}
	u.Path = path.Join(u.Path, model.ApiUrlSuffix, "remotecluster", "upload", task.us.ID)

	req, err := http.NewRequest("POST", u.String(), r)
	if err != nil {
		return nil, err
	}

	req.Header.Set(model.HeaderRemoteclusterID, task.rc.RemoteID)
	req.Header.Set(model.HeaderRemoteclusterToken, task.rc.RemoteToken)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := rcs.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response: %d - %s", resp.StatusCode, resp.Status)
	}

	// body should be a FileInfo
	var fi model.FileInfo
	if err := json.Unmarshal(body, &fi); err != nil {
		return nil, fmt.Errorf("unexpected response body: %w", err)
	}

	return &fi, nil
}
