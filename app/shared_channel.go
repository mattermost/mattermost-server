// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func (a *App) checkChannelNotShared(channelId string) error {
	// check that channel exists.
	if _, err := a.GetChannel(channelId); err != nil {
		return fmt.Errorf("cannot share this channel: %w", err)
	}

	// Check channel is not already shared.
	if _, err := a.GetSharedChannel(channelId); err == nil {
		var errNotFound *store.ErrNotFound
		if errors.As(err, &errNotFound) {
			return errors.New("channel is already shared")
		}
		return fmt.Errorf("cannot find channel: %w", err)
	}
	return nil
}

func (a *App) checkChannelIsShared(channelId string) error {
	if _, err := a.GetSharedChannel(channelId); err != nil {
		var errNotFound *store.ErrNotFound
		if errors.As(err, &errNotFound) {
			return errors.New("channel is not shared")
		}
		return fmt.Errorf("cannot find channel: %w", err)
	}
	return nil
}

func (a *App) CheckCanInviteToSharedChannel(channelId string) error {
	sc, err := a.GetSharedChannel(channelId)
	if err != nil {
		var errNotFound *store.ErrNotFound
		if errors.As(err, &errNotFound) {
			return errors.New("channel is not shared")
		}
		return fmt.Errorf("cannot find channel: %w", err)
	}

	if !sc.Home {
		return errors.New("channel is homed on a remote cluster")
	}
	return nil
}

// SharedChannels

func (a *App) SaveSharedChannel(sc *model.SharedChannel) (*model.SharedChannel, error) {
	if err := a.checkChannelNotShared(sc.ChannelId); err != nil {
		return nil, err
	}
	return a.Srv().Store.SharedChannel().Save(sc)
}

func (a *App) GetSharedChannel(channelID string) (*model.SharedChannel, error) {
	return a.Srv().Store.SharedChannel().Get(channelID)
}

func (a *App) HasSharedChannel(channelID string) (bool, error) {
	return a.Srv().Store.SharedChannel().HasChannel(channelID)
}

func (a *App) GetSharedChannels(page int, perPage int, opts model.SharedChannelFilterOpts) ([]*model.SharedChannel, *model.AppError) {
	channels, err := a.Srv().Store.SharedChannel().GetAll(page*perPage, perPage, opts)
	if err != nil {
		return nil, model.NewAppError("GetSharedChannels", "app.channel.get_channels.not_found.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return channels, nil
}

func (a *App) GetSharedChannelsCount(opts model.SharedChannelFilterOpts) (int64, error) {
	return a.Srv().Store.SharedChannel().GetAllCount(opts)
}

func (a *App) UpdateSharedChannel(sc *model.SharedChannel) (*model.SharedChannel, error) {
	return a.Srv().Store.SharedChannel().Update(sc)
}

func (a *App) DeleteSharedChannel(channelID string) (bool, error) {
	return a.Srv().Store.SharedChannel().Delete(channelID)
}

// SharedChannelRemotes

func (a *App) SaveSharedChannelRemote(remote *model.SharedChannelRemote) (*model.SharedChannelRemote, error) {
	if err := a.checkChannelIsShared(remote.ChannelId); err != nil {
		return nil, err
	}
	return a.Srv().Store.SharedChannel().SaveRemote(remote)
}

func (a *App) GetSharedChannelRemote(id string) (*model.SharedChannelRemote, error) {
	return a.Srv().Store.SharedChannel().GetRemote(id)
}

func (a *App) GetSharedChannelRemoteByIds(channelID string, remoteID string) (*model.SharedChannelRemote, error) {
	return a.Srv().Store.SharedChannel().GetRemoteByIds(channelID, remoteID)
}

func (a *App) GetSharedChannelRemotes(opts model.SharedChannelRemoteFilterOpts) ([]*model.SharedChannelRemote, error) {
	return a.Srv().Store.SharedChannel().GetRemotes(opts)
}

// HasRemote returns whether a given channelID is present in the channel remotes or not.
func (a *App) HasRemote(channelID string, remoteID string) (bool, error) {
	return a.Srv().Store.SharedChannel().HasRemote(channelID, remoteID)
}

func (a *App) GetRemoteClusterForUser(remoteID string, userID string) (*model.RemoteCluster, *model.AppError) {
	rc, err := a.Srv().Store.SharedChannel().GetRemoteForUser(remoteID, userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetRemoteClusterForUser", "api.context.remote_id_invalid.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetRemoteClusterForUser", "api.context.remote_id_invalid.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return rc, nil
}

func (a *App) UpdateSharedChannelRemoteCursor(id string, cursor model.GetPostsSinceForSyncCursor) error {
	return a.Srv().Store.SharedChannel().UpdateRemoteCursor(id, cursor)
}

func (a *App) DeleteSharedChannelRemote(id string) (bool, error) {
	return a.Srv().Store.SharedChannel().DeleteRemote(id)
}

func (a *App) GetSharedChannelRemotesStatus(channelID string) ([]*model.SharedChannelRemoteStatus, error) {
	if err := a.checkChannelIsShared(channelID); err != nil {
		return nil, err
	}
	return a.Srv().Store.SharedChannel().GetRemotesStatus(channelID)
}

// SharedChannelUsers

func (a *App) NotifySharedChannelUserUpdate(user *model.User) {
	a.sendUpdatedUserEvent(*user)
}

// onUserProfileChange is called when a user's profile has changed
// (username, email, profile image, ...)
func (a *App) onUserProfileChange(userID string) {
	syncService := a.Srv().GetSharedChannelSyncService()
	if syncService == nil || !syncService.Active() {
		return
	}
	syncService.NotifyUserProfileChanged(userID)
}
