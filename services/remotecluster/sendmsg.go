// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package remotecluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/wiggin77/merror"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type SendMsgResultFunc func(msg model.RemoteClusterMsg, rc *model.RemoteCluster, resp *Response, err error)

type sendMsgTask struct {
	rc  *model.RemoteCluster
	msg model.RemoteClusterMsg
	f   SendMsgResultFunc
}

// BroadcastMsg asynchronously sends a message to all remote clusters interested in the message's topic.
//
// `ctx` determines behaviour when the outbound queue is full. A timeout or deadline context will return a
// BufferFullError if the message cannot be enqueued before the timeout. A background context will block indefinitely.
//
// An optional callback can be provided that receives the success or fail result of sending to each remote cluster.
// Success or fail is regarding message delivery only.  If a callback is provided it should return quickly.
func (rcs *Service) BroadcastMsg(ctx context.Context, msg model.RemoteClusterMsg, f SendMsgResultFunc) error {
	// get list of interested remotes.
	filter := model.RemoteClusterQueryFilter{
		Topic: msg.Topic,
	}
	list, err := rcs.server.GetStore().RemoteCluster().GetAll(filter)
	if err != nil {
		return err
	}

	errs := merror.New()

	for _, rc := range list {
		if err := rcs.SendMsg(ctx, msg, rc, f); err != nil {
			errs.Append(err)
		}
	}
	return errs.ErrorOrNil()
}

// SendMsg asynchronously sends a message to a remote cluster.
//
// `ctx` determines behaviour when the outbound queue is full. A timeout or deadline context will return a
// BufferFullError if the message cannot be enqueued before the timeout. A background context will block indefinitely.
//
// Nil or error return indicates success or failure of message enqueue only.
//
// An optional callback can be provided that receives the response from the remote cluster. The `err` provided to the
// callback is regarding response decoding only. The `resp` contains the decoded bytes returned from the remote.
// If a callback is provided it should return quickly.
func (rcs *Service) SendMsg(ctx context.Context, msg model.RemoteClusterMsg, rc *model.RemoteCluster, f SendMsgResultFunc) error {
	task := sendMsgTask{
		rc:  rc,
		msg: msg,
		f:   f,
	}
	return rcs.enqueueTask(ctx, rc.RemoteID, task)
}

// sendMsg is called when a sendMsgTask is popped from the send channel.
func (rcs *Service) sendMsg(task sendMsgTask) {
	var errResp error
	var response Response

	// Ensure a panic from the callback does not exit the pool goroutine.
	defer func() {
		if errResp != nil {
			response.Err = errResp.Error()
		}

		// If callback provided then call it with the results.
		if task.f != nil {
			task.f(task.msg, task.rc, &response, errResp)
		}
	}()

	frame := &model.RemoteClusterFrame{
		RemoteID: task.rc.RemoteID,
		Msg:      task.msg,
	}

	u, err := url.Parse(task.rc.SiteURL)
	if err != nil {
		rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceError, "Invalid siteURL while sending message to remote",
			mlog.String("remote", task.rc.DisplayName),
			mlog.String("msgId", task.msg.ID),
			mlog.Err(err),
		)
		errResp = err
		return
	}
	u.Path = path.Join(u.Path, SendMsgURL)

	respJSON, err := rcs.sendFrameToRemote(SendTimeout, task.rc, frame, u.String())

	if err != nil {
		rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceError, "Remote Cluster send message failed",
			mlog.String("remote", task.rc.DisplayName),
			mlog.String("msgId", task.msg.ID),
			mlog.Err(err),
		)
		errResp = err
	} else {
		rcs.server.GetLogger().Log(mlog.LvlRemoteClusterServiceDebug, "Remote Cluster message sent successfully",
			mlog.String("remote", task.rc.DisplayName),
			mlog.String("msgId", task.msg.ID),
		)

		if err = json.Unmarshal(respJSON, &response); err != nil {
			rcs.server.GetLogger().Error("Invalid response sending message to remote cluster",
				mlog.String("remote", task.rc.DisplayName),
				mlog.Err(err),
			)
			errResp = err
		}
	}
}

func (rcs *Service) sendFrameToRemote(timeout time.Duration, rc *model.RemoteCluster, frame *model.RemoteClusterFrame, url string) ([]byte, error) {
	body, err := json.Marshal(frame)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(model.HeaderRemoteclusterID, rc.RemoteID)
	req.Header.Set(model.HeaderRemoteclusterToken, rc.RemoteToken)

	resp, err := rcs.httpClient.Do(req.WithContext(ctx))
	if metrics := rcs.server.GetMetrics(); metrics != nil {
		if err != nil || resp.StatusCode != http.StatusOK {
			metrics.IncrementRemoteClusterMsgErrorsCounter(frame.RemoteID, os.IsTimeout(err))
		} else {
			metrics.IncrementRemoteClusterMsgSentCounter(frame.RemoteID)
		}
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("unexpected response: %d - %s", resp.StatusCode, resp.Status)
	}
	return body, nil
}
