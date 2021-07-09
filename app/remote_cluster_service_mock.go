// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/remotecluster"
)

// MockOptionRemoteClusterService a mock of the remote cluster service
type MockOptionRemoteClusterService func(service *mockRemoteClusterService)

func MockOptionRemoteClusterServiceWithActive(active bool) MockOptionRemoteClusterService {
	return func(mrcs *mockRemoteClusterService) {
		mrcs.active = active
	}
}

func NewMockRemoteClusterService(service remotecluster.RemoteClusterServiceIFace, options ...MockOptionRemoteClusterService) *mockRemoteClusterService {
	mrcs := &mockRemoteClusterService{service, true}
	for _, option := range options {
		option(mrcs)
	}
	return mrcs
}

type mockRemoteClusterService struct {
	remotecluster.RemoteClusterServiceIFace
	active bool
}

func (mrcs *mockRemoteClusterService) Shutdown() error {
	return nil
}

func (mrcs *mockRemoteClusterService) Start() error {
	return nil
}

func (mrcs *mockRemoteClusterService) Active() bool {
	return mrcs.active
}

func (mrcs *mockRemoteClusterService) AddTopicListener(topic string, listener remotecluster.TopicListener) string {
	return model.NewID()
}

func (mrcs *mockRemoteClusterService) RemoveTopicListener(listenerID string) {
}

func (mrcs *mockRemoteClusterService) AddConnectionStateListener(listener remotecluster.ConnectionStateListener) string {
	return model.NewID()
}

func (mrcs *mockRemoteClusterService) RemoveConnectionStateListener(listenerID string) {
}

func (mrcs *mockRemoteClusterService) SendMsg(ctx context.Context, msg model.RemoteClusterMsg, rc *model.RemoteCluster, f remotecluster.SendMsgResultFunc) error {
	return nil
}

func (mrcs *mockRemoteClusterService) SendFile(ctx context.Context, us *model.UploadSession, fi *model.FileInfo, rc *model.RemoteCluster, rp remotecluster.ReaderProvider, f remotecluster.SendFileResultFunc) error {
	return nil
}

func (mrcs *mockRemoteClusterService) AcceptInvitation(invite *model.RemoteClusterInvite, name string, displayName string, creatorID string, teamID string, siteURL string) (*model.RemoteCluster, error) {
	return nil, nil
}

func (mrcs *mockRemoteClusterService) ReceiveIncomingMsg(rc *model.RemoteCluster, msg model.RemoteClusterMsg) remotecluster.Response {
	return remotecluster.Response{}
}
