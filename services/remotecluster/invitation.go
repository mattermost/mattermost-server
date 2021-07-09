// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package remotecluster

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mattermost/mattermost-server/v5/model"
)

// AcceptInvitation is called when accepting an invitation to connect with a remote cluster.
func (rcs *Service) AcceptInvitation(invite *model.RemoteClusterInvite, name string, displayName, creatorID string, teamID string, siteURL string) (*model.RemoteCluster, error) {
	rc := &model.RemoteCluster{
		RemoteID:     invite.RemoteID,
		RemoteTeamID: invite.RemoteTeamID,
		Name:         name,
		DisplayName:  displayName,
		Token:        model.NewID(),
		RemoteToken:  invite.Token,
		SiteURL:      invite.SiteURL,
		CreatorID:    creatorID,
	}

	rcSaved, err := rcs.server.GetStore().RemoteCluster().Save(rc)
	if err != nil {
		return nil, err
	}

	// confirm the invitation with the originating site
	frame, err := makeConfirmFrame(rcSaved, teamID, siteURL)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s", rcSaved.SiteURL, ConfirmInviteURL)

	resp, err := rcs.sendFrameToRemote(PingTimeout, rc, frame, url)
	if err != nil {
		rcs.server.GetStore().RemoteCluster().Delete(rcSaved.RemoteID)
		return nil, err
	}

	var response Response
	err = json.Unmarshal(resp, &response)
	if err != nil {
		rcs.server.GetStore().RemoteCluster().Delete(rcSaved.RemoteID)
		return nil, fmt.Errorf("invalid response from remote server: %w", err)
	}

	if !response.IsSuccess() {
		rcs.server.GetStore().RemoteCluster().Delete(rcSaved.RemoteID)
		return nil, errors.New(response.Err)
	}

	// issue the first ping right away. The goroutine will exit when ping completes or PingTimeout exceeded.
	go rcs.pingRemote(rcSaved)

	return rcSaved, nil
}

func makeConfirmFrame(rc *model.RemoteCluster, teamID string, siteURL string) (*model.RemoteClusterFrame, error) {
	confirm := model.RemoteClusterInvite{
		RemoteID:     rc.RemoteID,
		RemoteTeamID: teamID,
		SiteURL:      siteURL,
		Token:        rc.Token,
	}
	confirmRaw, err := json.Marshal(confirm)
	if err != nil {
		return nil, err
	}

	msg := model.NewRemoteClusterMsg(InvitationTopic, confirmRaw)

	frame := &model.RemoteClusterFrame{
		RemoteID: rc.RemoteID,
		Msg:      msg,
	}
	return frame, nil
}
