// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"io"
	"net/http"
	"unicode/utf8"
)

// SharedChannel represents a channel that can be synchronized with a remote cluster.
// If "home" is true, then the shared channel is homed locally and "SharedChannelRemote"
// table contains the remote clusters that have been invited.
// If "home" is false, then the shared channel is homed remotely, and "RemoteId"
// field points to the remote cluster connection in "RemoteClusters" table.
type SharedChannel struct {
	ChannelID        string `json:"id"`
	TeamID           string `json:"team_id"`
	Home             bool   `json:"home"`
	ReadOnly         bool   `json:"readonly"`
	ShareName        string `json:"name"`
	ShareDisplayName string `json:"display_name"`
	SharePurpose     string `json:"purpose"`
	ShareHeader      string `json:"header"`
	CreatorID        string `json:"creator_id"`
	CreateAt         int64  `json:"create_at"`
	UpdateAt         int64  `json:"update_at"`
	RemoteID         string `json:"remote_id,omitempty"` // if not "home"
	Type             string `db:"-"`
}

func (sc *SharedChannel) ToJSON() string {
	b, _ := json.Marshal(sc)
	return string(b)
}

func SharedChannelFromJSON(data io.Reader) (*SharedChannel, error) {
	var sc *SharedChannel
	err := json.NewDecoder(data).Decode(&sc)
	return sc, err
}

func (sc *SharedChannel) IsValid() *AppError {
	if !IsValidID(sc.ChannelID) {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.id.app_error", nil, "ChannelId="+sc.ChannelID, http.StatusBadRequest)
	}

	if sc.Type != ChannelTypeDirect && !IsValidID(sc.TeamID) {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.id.app_error", nil, "TeamId="+sc.TeamID, http.StatusBadRequest)
	}

	if sc.CreateAt == 0 {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.create_at.app_error", nil, "id="+sc.ChannelID, http.StatusBadRequest)
	}

	if sc.UpdateAt == 0 {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.update_at.app_error", nil, "id="+sc.ChannelID, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(sc.ShareDisplayName) > ChannelDisplayNameMaxRunes {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.display_name.app_error", nil, "id="+sc.ChannelID, http.StatusBadRequest)
	}

	if !IsValidChannelIdentifier(sc.ShareName) {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.2_or_more.app_error", nil, "id="+sc.ChannelID, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(sc.ShareHeader) > ChannelHeaderMaxRunes {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.header.app_error", nil, "id="+sc.ChannelID, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(sc.SharePurpose) > ChannelPurposeMaxRunes {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.purpose.app_error", nil, "id="+sc.ChannelID, http.StatusBadRequest)
	}

	if !IsValidID(sc.CreatorID) {
		return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.creator_id.app_error", nil, "CreatorId="+sc.CreatorID, http.StatusBadRequest)
	}

	if !sc.Home {
		if !IsValidID(sc.RemoteID) {
			return NewAppError("SharedChannel.IsValid", "model.channel.is_valid.id.app_error", nil, "RemoteId="+sc.RemoteID, http.StatusBadRequest)
		}
	}
	return nil
}

func (sc *SharedChannel) PreSave() {
	sc.ShareName = SanitizeUnicode(sc.ShareName)
	sc.ShareDisplayName = SanitizeUnicode(sc.ShareDisplayName)

	sc.CreateAt = GetMillis()
	sc.UpdateAt = sc.CreateAt
}

func (sc *SharedChannel) PreUpdate() {
	sc.UpdateAt = GetMillis()
	sc.ShareName = SanitizeUnicode(sc.ShareName)
	sc.ShareDisplayName = SanitizeUnicode(sc.ShareDisplayName)
}

// SharedChannelRemote represents a remote cluster that has been invited
// to a shared channel.
type SharedChannelRemote struct {
	ID                string `json:"id"`
	ChannelID         string `json:"channel_id"`
	CreatorID         string `json:"creator_id"`
	CreateAt          int64  `json:"create_at"`
	UpdateAt          int64  `json:"update_at"`
	IsInviteAccepted  bool   `json:"is_invite_accepted"`
	IsInviteConfirmed bool   `json:"is_invite_confirmed"`
	RemoteID          string `json:"remote_id"`
	LastPostUpdateAt  int64  `json:"last_post_update_at"`
	LastPostID        string `json:"last_post_id"`
}

func (sc *SharedChannelRemote) ToJSON() string {
	b, _ := json.Marshal(sc)
	return string(b)
}

func SharedChannelRemoteFromJSON(data io.Reader) (*SharedChannelRemote, error) {
	var sc *SharedChannelRemote
	err := json.NewDecoder(data).Decode(&sc)
	return sc, err
}

func (sc *SharedChannelRemote) IsValid() *AppError {
	if !IsValidID(sc.ID) {
		return NewAppError("SharedChannelRemote.IsValid", "model.channel.is_valid.id.app_error", nil, "Id="+sc.ID, http.StatusBadRequest)
	}

	if !IsValidID(sc.ChannelID) {
		return NewAppError("SharedChannelRemote.IsValid", "model.channel.is_valid.id.app_error", nil, "ChannelId="+sc.ChannelID, http.StatusBadRequest)
	}

	if sc.CreateAt == 0 {
		return NewAppError("SharedChannelRemote.IsValid", "model.channel.is_valid.create_at.app_error", nil, "id="+sc.ChannelID, http.StatusBadRequest)
	}

	if sc.UpdateAt == 0 {
		return NewAppError("SharedChannelRemote.IsValid", "model.channel.is_valid.update_at.app_error", nil, "id="+sc.ChannelID, http.StatusBadRequest)
	}

	if !IsValidID(sc.CreatorID) {
		return NewAppError("SharedChannelRemote.IsValid", "model.channel.is_valid.creator_id.app_error", nil, "id="+sc.CreatorID, http.StatusBadRequest)
	}
	return nil
}

func (sc *SharedChannelRemote) PreSave() {
	if sc.ID == "" {
		sc.ID = NewID()
	}
	sc.CreateAt = GetMillis()
	sc.UpdateAt = sc.CreateAt
}

func (sc *SharedChannelRemote) PreUpdate() {
	sc.UpdateAt = GetMillis()
}

type SharedChannelRemoteStatus struct {
	ChannelID        string `json:"channel_id"`
	DisplayName      string `json:"display_name"`
	SiteURL          string `json:"site_url"`
	LastPingAt       int64  `json:"last_ping_at"`
	NextSyncAt       int64  `json:"next_sync_at"`
	ReadOnly         bool   `json:"readonly"`
	IsInviteAccepted bool   `json:"is_invite_accepted"`
	Token            string `json:"token"`
}

// SharedChannelUser stores a lastSyncAt timestamp on behalf of a remote cluster for
// each user that has been synchronized.
type SharedChannelUser struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	ChannelID  string `json:"channel_id"`
	RemoteID   string `json:"remote_id"`
	CreateAt   int64  `json:"create_at"`
	LastSyncAt int64  `json:"last_sync_at"`
}

func (scu *SharedChannelUser) PreSave() {
	scu.ID = NewID()
	scu.CreateAt = GetMillis()
}

func (scu *SharedChannelUser) IsValid() *AppError {
	if !IsValidID(scu.ID) {
		return NewAppError("SharedChannelUser.IsValid", "model.channel.is_valid.id.app_error", nil, "Id="+scu.ID, http.StatusBadRequest)
	}

	if !IsValidID(scu.UserID) {
		return NewAppError("SharedChannelUser.IsValid", "model.channel.is_valid.id.app_error", nil, "UserId="+scu.UserID, http.StatusBadRequest)
	}

	if !IsValidID(scu.ChannelID) {
		return NewAppError("SharedChannelUser.IsValid", "model.channel.is_valid.id.app_error", nil, "ChannelId="+scu.ChannelID, http.StatusBadRequest)
	}

	if !IsValidID(scu.RemoteID) {
		return NewAppError("SharedChannelUser.IsValid", "model.channel.is_valid.id.app_error", nil, "RemoteId="+scu.RemoteID, http.StatusBadRequest)
	}

	if scu.CreateAt == 0 {
		return NewAppError("SharedChannelUser.IsValid", "model.channel.is_valid.create_at.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

type GetUsersForSyncFilter struct {
	CheckProfileImage bool
	ChannelID         string
	Limit             uint64
}

// SharedChannelAttachment stores a lastSyncAt timestamp on behalf of a remote cluster for
// each file attachment that has been synchronized.
type SharedChannelAttachment struct {
	ID         string `json:"id"`
	FileID     string `json:"file_id"`
	RemoteID   string `json:"remote_id"`
	CreateAt   int64  `json:"create_at"`
	LastSyncAt int64  `json:"last_sync_at"`
}

func (scf *SharedChannelAttachment) PreSave() {
	if scf.ID == "" {
		scf.ID = NewID()
	}
	if scf.CreateAt == 0 {
		scf.CreateAt = GetMillis()
		scf.LastSyncAt = scf.CreateAt
	} else {
		scf.LastSyncAt = GetMillis()
	}
}

func (scf *SharedChannelAttachment) IsValid() *AppError {
	if !IsValidID(scf.ID) {
		return NewAppError("SharedChannelAttachment.IsValid", "model.channel.is_valid.id.app_error", nil, "Id="+scf.ID, http.StatusBadRequest)
	}

	if !IsValidID(scf.FileID) {
		return NewAppError("SharedChannelAttachment.IsValid", "model.channel.is_valid.id.app_error", nil, "FileId="+scf.FileID, http.StatusBadRequest)
	}

	if !IsValidID(scf.RemoteID) {
		return NewAppError("SharedChannelAttachment.IsValid", "model.channel.is_valid.id.app_error", nil, "RemoteId="+scf.RemoteID, http.StatusBadRequest)
	}

	if scf.CreateAt == 0 {
		return NewAppError("SharedChannelAttachment.IsValid", "model.channel.is_valid.create_at.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

type SharedChannelFilterOpts struct {
	TeamID        string
	CreatorID     string
	ExcludeHome   bool
	ExcludeRemote bool
}

type SharedChannelRemoteFilterOpts struct {
	ChannelID       string
	RemoteID        string
	InclUnconfirmed bool
}
