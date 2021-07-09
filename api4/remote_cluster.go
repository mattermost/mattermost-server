// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/remotecluster"
)

func (api *API) InitRemoteCluster() {
	api.BaseRoutes.RemoteCluster.Handle("/ping", api.RemoteClusterTokenRequired(remoteClusterPing)).Methods("POST")
	api.BaseRoutes.RemoteCluster.Handle("/msg", api.RemoteClusterTokenRequired(remoteClusterAcceptMessage)).Methods("POST")
	api.BaseRoutes.RemoteCluster.Handle("/confirm_invite", api.RemoteClusterTokenRequired(remoteClusterConfirmInvite)).Methods("POST")
	api.BaseRoutes.RemoteCluster.Handle("/upload/{upload_id:[A-Za-z0-9]+}", api.RemoteClusterTokenRequired(uploadRemoteData)).Methods("POST")
	api.BaseRoutes.RemoteCluster.Handle("/{user_id:[A-Za-z0-9]+}/image", api.RemoteClusterTokenRequired(remoteSetProfileImage)).Methods("POST")
}

func remoteClusterPing(c *Context, w http.ResponseWriter, r *http.Request) {
	// make sure remote cluster service is enabled.
	if _, appErr := c.App.GetRemoteClusterService(); appErr != nil {
		c.Err = appErr
		return
	}

	frame, appErr := model.RemoteClusterFrameFromJSON(r.Body)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if appErr = frame.IsValid(); appErr != nil {
		c.Err = appErr
		return
	}

	remoteID := c.GetRemoteID(r)
	if remoteID != frame.RemoteID {
		c.SetInvalidRemoteIDError(frame.RemoteID)
		return
	}

	rc, err := c.App.GetRemoteCluster(frame.RemoteID)
	if err != nil {
		c.SetInvalidRemoteIDError(frame.RemoteID)
		return
	}

	ping, err := model.RemoteClusterPingFromRawJSON(frame.Msg.Payload)
	if err != nil {
		c.SetInvalidParam("msg.payload")
		return
	}
	ping.RecvAt = model.GetMillis()

	if metrics := c.App.Metrics(); metrics != nil {
		metrics.IncrementRemoteClusterMsgReceivedCounter(rc.RemoteID)
	}

	resp, _ := json.Marshal(ping)
	w.Write(resp)
}

func remoteClusterAcceptMessage(c *Context, w http.ResponseWriter, r *http.Request) {
	// make sure remote cluster service is running.
	service, appErr := c.App.GetRemoteClusterService()
	if appErr != nil {
		c.Err = appErr
		return
	}

	frame, appErr := model.RemoteClusterFrameFromJSON(r.Body)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if appErr = frame.IsValid(); appErr != nil {
		c.Err = appErr
		return
	}

	auditRec := c.MakeAuditRecord("remoteClusterAcceptMessage", audit.Fail)
	defer c.LogAuditRec(auditRec)

	remoteID := c.GetRemoteID(r)
	if remoteID != frame.RemoteID {
		c.SetInvalidRemoteIDError(frame.RemoteID)
		return
	}

	rc, err := c.App.GetRemoteCluster(frame.RemoteID)
	if err != nil {
		c.SetInvalidRemoteIDError(frame.RemoteID)
		return
	}
	auditRec.AddMeta("remoteCluster", rc)

	// pass message to Remote Cluster Service and write response
	resp := service.ReceiveIncomingMsg(rc, frame.Msg)

	b, errMarshall := json.Marshal(resp)
	if errMarshall != nil {
		c.Err = model.NewAppError("remoteClusterAcceptMessage", "api.marshal_error", nil, errMarshall.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

func remoteClusterConfirmInvite(c *Context, w http.ResponseWriter, r *http.Request) {
	// make sure remote cluster service is running.
	if _, appErr := c.App.GetRemoteClusterService(); appErr != nil {
		c.Err = appErr
		return
	}

	frame, appErr := model.RemoteClusterFrameFromJSON(r.Body)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if appErr = frame.IsValid(); appErr != nil {
		c.Err = appErr
		return
	}

	auditRec := c.MakeAuditRecord("remoteClusterAcceptInvite", audit.Fail)
	defer c.LogAuditRec(auditRec)

	remoteID := c.GetRemoteID(r)
	if remoteID != frame.RemoteID {
		c.SetInvalidRemoteIDError(frame.RemoteID)
		return
	}

	rc, err := c.App.GetRemoteCluster(frame.RemoteID)
	if err != nil {
		c.SetInvalidRemoteIDError(frame.RemoteID)
		return
	}
	auditRec.AddMeta("remoteCluster", rc)

	if time.Since(model.GetTimeForMillis(rc.CreateAt)) > remotecluster.InviteExpiresAfter {
		c.Err = model.NewAppError("remoteClusterAcceptMessage", "api.context.invitation_expired.error", nil, "", http.StatusBadRequest)
		return
	}

	confirm, appErr := model.RemoteClusterInviteFromRawJSON(frame.Msg.Payload)
	if appErr != nil {
		c.Err = appErr
		return
	}

	rc.RemoteTeamID = confirm.RemoteTeamID
	rc.SiteURL = confirm.SiteURL
	rc.RemoteToken = confirm.Token

	if _, err := c.App.UpdateRemoteCluster(rc); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func uploadRemoteData(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().FileSettings.EnableFileAttachments {
		c.Err = model.NewAppError("uploadRemoteData", "api.file.attachments.disabled.app_error",
			nil, "", http.StatusNotImplemented)
		return
	}

	c.RequireUploadID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("uploadRemoteData", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("upload_id", c.Params.UploadID)

	us, err := c.App.GetUploadSession(c.Params.UploadID)
	if err != nil {
		c.Err = err
		return
	}

	if us.RemoteID != c.GetRemoteID(r) {
		c.Err = model.NewAppError("uploadRemoteData", "api.context.remote_id_mismatch.app_error",
			nil, "", http.StatusUnauthorized)
		return
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

func remoteSetProfileImage(c *Context, w http.ResponseWriter, r *http.Request) {
	defer io.Copy(ioutil.Discard, r.Body)

	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if *c.App.Config().FileSettings.DriverName == "" {
		c.Err = model.NewAppError("remoteUploadProfileImage", "api.user.upload_profile_user.storage.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	if r.ContentLength > *c.App.Config().FileSettings.MaxFileSize {
		c.Err = model.NewAppError("remoteUploadProfileImage", "api.user.upload_profile_user.too_large.app_error", nil, "", http.StatusRequestEntityTooLarge)
		return
	}

	if err := r.ParseMultipartForm(*c.App.Config().FileSettings.MaxFileSize); err != nil {
		c.Err = model.NewAppError("remoteUploadProfileImage", "api.user.upload_profile_user.parse.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	m := r.MultipartForm
	imageArray, ok := m.File["image"]
	if !ok {
		c.Err = model.NewAppError("remoteUploadProfileImage", "api.user.upload_profile_user.no_file.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if len(imageArray) == 0 {
		c.Err = model.NewAppError("remoteUploadProfileImage", "api.user.upload_profile_user.array.app_error", nil, "", http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord("remoteUploadProfileImage", audit.Fail)
	defer c.LogAuditRec(auditRec)
	if imageArray[0] != nil {
		auditRec.AddMeta("filename", imageArray[0].Filename)
	}

	user, err := c.App.GetUser(c.Params.UserID)
	if err != nil || !user.IsRemote() {
		c.SetInvalidUrlParam("user_id")
		return
	}
	auditRec.AddMeta("user", user)

	imageData := imageArray[0]
	if err := c.App.SetProfileImage(c.Params.UserID, imageData); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")

	ReturnStatusOK(w)
}
