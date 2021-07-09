// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/sharedchannel"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

var sharedChannelEventsForSync model.StringArray = []string{
	model.WebsocketEventPosted,
	model.WebsocketEventPostEdited,
	model.WebsocketEventPostDeleted,
	model.WebsocketEventReactionAdded,
	model.WebsocketEventReactionRemoved,
}

var sharedChannelEventsForInvitation model.StringArray = []string{
	model.WebsocketEventDirectAdded,
}

// SharedChannelSyncHandler is called when a websocket event is received by a cluster node.
// Only on the leader node it will notify the sync service to perform necessary updates to the remote for the given
// shared channel.
func (s *Server) SharedChannelSyncHandler(event *model.WebSocketEvent) {
	syncService := s.GetSharedChannelSyncService()
	if isEligibleForEvents(syncService, event, sharedChannelEventsForSync) {
		err := handleContentSync(s, syncService, event)
		if err != nil {
			mlog.Warn(
				err.Error(),
				mlog.String("event", event.EventType()),
				mlog.String("action", "content_sync"),
			)
		}
	} else if isEligibleForEvents(syncService, event, sharedChannelEventsForInvitation) {
		err := handleInvitation(s, syncService, event)
		if err != nil {
			mlog.Warn(
				err.Error(),
				mlog.String("event", event.EventType()),
				mlog.String("action", "invitation"),
			)
		}
	}
}

func isEligibleForEvents(syncService SharedChannelServiceIFace, event *model.WebSocketEvent, events model.StringArray) bool {
	return syncServiceEnabled(syncService) &&
		eventHasChannel(event) &&
		events.Contains(event.EventType())
}

func eventHasChannel(event *model.WebSocketEvent) bool {
	return event.GetBroadcast() != nil &&
		event.GetBroadcast().ChannelID != ""
}

func syncServiceEnabled(syncService SharedChannelServiceIFace) bool {
	return syncService != nil &&
		syncService.Active()
}

func handleContentSync(s *Server, syncService SharedChannelServiceIFace, event *model.WebSocketEvent) error {
	channel, err := findChannel(s, event.GetBroadcast().ChannelID)
	if err != nil {
		return err
	}

	if channel != nil && channel.IsShared() {
		syncService.NotifyChannelChanged(channel.ID)
	}

	return nil
}

func handleInvitation(s *Server, syncService SharedChannelServiceIFace, event *model.WebSocketEvent) error {
	channel, err := findChannel(s, event.GetBroadcast().ChannelID)
	if err != nil {
		return err
	}

	if channel == nil || !channel.IsShared() {
		return nil
	}

	creator, err := getUserFromEvent(s, event, "creator_id")
	if err != nil {
		return err
	}
	// This is a termination condition, since on the other end when we are processing
	// the invite we are re-triggering a model.WEBSOCKET_EVENT_DIRECT_ADDED, which will call this handler.
	// When the creator is remote, it means that this is a DM that was not originated from the current server
	// and therefore we do not need to do anything.
	if creator == nil || creator.IsRemote() {
		return nil
	}

	participant, err := getUserFromEvent(s, event, "teammate_id")
	if err != nil {
		return err
	}

	if participant == nil {
		return nil
	}

	rc, err := s.Store.RemoteCluster().Get(*participant.RemoteID)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("couldn't find remote cluster %s, for creating shared channel invitation for a DM", *participant.RemoteID))
	}

	return syncService.SendChannelInvite(channel, creator.ID, rc, sharedchannel.WithDirectParticipantID(creator.ID), sharedchannel.WithDirectParticipantID(participant.ID))
}

func getUserFromEvent(s *Server, event *model.WebSocketEvent, key string) (*model.User, error) {
	userID, ok := event.GetData()[key].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("received websocket message that is eligible for sending an invitation but message does not have `%s` present", key)
	}

	user, err := s.Store.User().Get(context.Background(), userID)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't find user for creating shared channel invitation for a DM")
	}

	return user, nil
}

func findChannel(server *Server, channelID string) (*model.Channel, error) {
	channel, err := server.Store.Channel().Get(channelID, true)
	if err != nil {
		return nil, errors.Wrap(err, "received websocket message that is eligible for shared channel sync but channel does not exist")
	}

	return channel, nil
}
