// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/sharedchannel"
)

// SharedChannelServiceIFace is the interface to the shared channel service
type SharedChannelServiceIFace interface {
	Shutdown() error
	Start() error
	NotifyChannelChanged(channelID string)
	NotifyUserProfileChanged(userID string)
	SendChannelInvite(channel *model.Channel, userID string, rc *model.RemoteCluster, options ...sharedchannel.InviteOption) error
	Active() bool
}

type MockOptionSharedChannelService func(service *mockSharedChannelService)

func MockOptionSharedChannelServiceWithActive(active bool) MockOptionSharedChannelService {
	return func(mrcs *mockSharedChannelService) {
		mrcs.active = active
	}
}

func NewMockSharedChannelService(service SharedChannelServiceIFace, options ...MockOptionSharedChannelService) *mockSharedChannelService {
	mrcs := &mockSharedChannelService{service, true, []string{}, []string{}, 0}
	for _, option := range options {
		option(mrcs)
	}
	return mrcs
}

type mockSharedChannelService struct {
	SharedChannelServiceIFace
	active                   bool
	channelNotifications     []string
	userProfileNotifications []string
	numInvitations           int
}

func (mrcs *mockSharedChannelService) NotifyChannelChanged(channelID string) {
	mrcs.channelNotifications = append(mrcs.channelNotifications, channelID)
}

func (mrcs *mockSharedChannelService) NotifyUserProfileChanged(userID string) {
	mrcs.userProfileNotifications = append(mrcs.userProfileNotifications, userID)
}

func (mrcs *mockSharedChannelService) Shutdown() error {
	return nil
}

func (mrcs *mockSharedChannelService) Start() error {
	return nil
}

func (mrcs *mockSharedChannelService) Active() bool {
	return mrcs.active
}

func (mrcs *mockSharedChannelService) SendChannelInvite(channel *model.Channel, userID string, rc *model.RemoteCluster, options ...sharedchannel.InviteOption) error {
	mrcs.numInvitations += 1
	return nil
}

func (mrcs *mockSharedChannelService) NumInvitations() int {
	return mrcs.numInvitations
}
