// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	ChannelTypeOpen    = "O"
	ChannelTypePrivate = "P"
	ChannelTypeDirect  = "D"
	ChannelTypeGroup   = "G"

	ChannelGroupMaxUsers       = 8
	ChannelGroupMinUsers       = 3
	DefaultChannelName         = "town-square"
	ChannelDisplayNameMaxRunes = 64
	ChannelNameMinLength       = 2
	ChannelNameMaxLength       = 64
	ChannelHeaderMaxRunes      = 1024
	ChannelPurposeMaxRunes     = 250
	ChannelCacheSize           = 25000

	ChannelSortByUsername = "username"
	ChannelSortByStatus   = "status"
)

type Channel struct {
	ID                string                 `json:"id"`
	CreateAt          int64                  `json:"create_at"`
	UpdateAt          int64                  `json:"update_at"`
	DeleteAt          int64                  `json:"delete_at"`
	TeamID            string                 `json:"team_id"`
	Type              string                 `json:"type"`
	DisplayName       string                 `json:"display_name"`
	Name              string                 `json:"name"`
	Header            string                 `json:"header"`
	Purpose           string                 `json:"purpose"`
	LastPostAt        int64                  `json:"last_post_at"`
	TotalMsgCount     int64                  `json:"total_msg_count"`
	ExtraUpdateAt     int64                  `json:"extra_update_at"`
	CreatorID         string                 `json:"creator_id"`
	SchemeID          *string                `json:"scheme_id"`
	Props             map[string]interface{} `json:"props" db:"-"`
	GroupConstrained  *bool                  `json:"group_constrained"`
	Shared            *bool                  `json:"shared"`
	TotalMsgCountRoot int64                  `json:"total_msg_count_root"`
	PolicyID          *string                `json:"policy_id" db:"-"`
}

type ChannelWithTeamData struct {
	Channel
	TeamDisplayName string `json:"team_display_name"`
	TeamName        string `json:"team_name"`
	TeamUpdateAt    int64  `json:"team_update_at"`
}

type ChannelsWithCount struct {
	Channels   *ChannelListWithTeamData `json:"channels"`
	TotalCount int64                    `json:"total_count"`
}

type ChannelPatch struct {
	DisplayName      *string `json:"display_name"`
	Name             *string `json:"name"`
	Header           *string `json:"header"`
	Purpose          *string `json:"purpose"`
	GroupConstrained *bool   `json:"group_constrained"`
}

type ChannelForExport struct {
	Channel
	TeamName   string
	SchemeName *string
}

type DirectChannelForExport struct {
	Channel
	Members *[]string
}

type ChannelModeration struct {
	Name  string                 `json:"name"`
	Roles *ChannelModeratedRoles `json:"roles"`
}

type ChannelModeratedRoles struct {
	Guests  *ChannelModeratedRole `json:"guests"`
	Members *ChannelModeratedRole `json:"members"`
}

type ChannelModeratedRole struct {
	Value   bool `json:"value"`
	Enabled bool `json:"enabled"`
}

type ChannelModerationPatch struct {
	Name  *string                     `json:"name"`
	Roles *ChannelModeratedRolesPatch `json:"roles"`
}

type ChannelModeratedRolesPatch struct {
	Guests  *bool `json:"guests"`
	Members *bool `json:"members"`
}

// ChannelSearchOpts contains options for searching channels.
//
// NotAssociatedToGroup will exclude channels that have associated, active GroupChannels records.
// ExcludeDefaultChannels will exclude the configured default channels (ex 'town-square' and 'off-topic').
// IncludeDeleted will include channel records where DeleteAt != 0.
// ExcludeChannelNames will exclude channels from the results by name.
// Paginate whether to paginate the results.
// Page page requested, if results are paginated.
// PerPage number of results per page, if paginated.
//
type ChannelSearchOpts struct {
	NotAssociatedToGroup     string
	ExcludeDefaultChannels   bool
	IncludeDeleted           bool
	Deleted                  bool
	ExcludeChannelNames      []string
	TeamIDs                  []string
	GroupConstrained         bool
	ExcludeGroupConstrained  bool
	PolicyID                 string
	ExcludePolicyConstrained bool
	IncludePolicyID          bool
	Public                   bool
	Private                  bool
	Page                     *int
	PerPage                  *int
}

type ChannelMemberCountByGroup struct {
	GroupID                     string `db:"-" json:"group_id"`
	ChannelMemberCount          int64  `db:"-" json:"channel_member_count"`
	ChannelMemberTimezonesCount int64  `db:"-" json:"channel_member_timezones_count"`
}

type ChannelOption func(channel *Channel)

func WithID(ID string) ChannelOption {
	return func(channel *Channel) {
		channel.ID = ID
	}
}

func (o *Channel) DeepCopy() *Channel {
	copy := *o
	if copy.SchemeID != nil {
		copy.SchemeID = NewString(*o.SchemeID)
	}
	return &copy
}

func (o *Channel) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func (o *ChannelPatch) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func (o *ChannelsWithCount) ToJSON() []byte {
	b, _ := json.Marshal(o)
	return b
}

func ChannelsWithCountFromJSON(data io.Reader) *ChannelsWithCount {
	var o *ChannelsWithCount
	json.NewDecoder(data).Decode(&o)
	return o
}

func ChannelFromJSON(data io.Reader) *Channel {
	var o *Channel
	json.NewDecoder(data).Decode(&o)
	return o
}

func ChannelPatchFromJSON(data io.Reader) *ChannelPatch {
	var o *ChannelPatch
	json.NewDecoder(data).Decode(&o)
	return o
}

func ChannelModerationsFromJSON(data io.Reader) []*ChannelModeration {
	var o []*ChannelModeration
	json.NewDecoder(data).Decode(&o)
	return o
}

func ChannelModerationsPatchFromJSON(data io.Reader) []*ChannelModerationPatch {
	var o []*ChannelModerationPatch
	json.NewDecoder(data).Decode(&o)
	return o
}

func ChannelMemberCountsByGroupFromJSON(data io.Reader) []*ChannelMemberCountByGroup {
	var o []*ChannelMemberCountByGroup
	json.NewDecoder(data).Decode(&o)
	return o
}

func (o *Channel) Etag() string {
	return Etag(o.ID, o.UpdateAt)
}

func (o *Channel) IsValid() *AppError {
	if !IsValidID(o.ID) {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.id.app_error", nil, "", http.StatusBadRequest)
	}

	if o.CreateAt == 0 {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.create_at.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if o.UpdateAt == 0 {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.update_at.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(o.DisplayName) > ChannelDisplayNameMaxRunes {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.display_name.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if !IsValidChannelIdentifier(o.Name) {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.2_or_more.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if !(o.Type == ChannelTypeOpen || o.Type == ChannelTypePrivate || o.Type == ChannelTypeDirect || o.Type == ChannelTypeGroup) {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.type.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(o.Header) > ChannelHeaderMaxRunes {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.header.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(o.Purpose) > ChannelPurposeMaxRunes {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.purpose.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if len(o.CreatorID) > 26 {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.creator_id.app_error", nil, "", http.StatusBadRequest)
	}

	userIDs := strings.Split(o.Name, "__")
	if o.Type != ChannelTypeDirect && len(userIDs) == 2 && IsValidID(userIDs[0]) && IsValidID(userIDs[1]) {
		return NewAppError("Channel.IsValid", "model.channel.is_valid.name.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (o *Channel) PreSave() {
	if o.ID == "" {
		o.ID = NewID()
	}

	o.Name = SanitizeUnicode(o.Name)
	o.DisplayName = SanitizeUnicode(o.DisplayName)

	o.CreateAt = GetMillis()
	o.UpdateAt = o.CreateAt
	o.ExtraUpdateAt = 0
}

func (o *Channel) PreUpdate() {
	o.UpdateAt = GetMillis()
	o.Name = SanitizeUnicode(o.Name)
	o.DisplayName = SanitizeUnicode(o.DisplayName)
}

func (o *Channel) IsGroupOrDirect() bool {
	return o.Type == ChannelTypeDirect || o.Type == ChannelTypeGroup
}

func (o *Channel) IsOpen() bool {
	return o.Type == ChannelTypeOpen
}

func (o *Channel) Patch(patch *ChannelPatch) {
	if patch.DisplayName != nil {
		o.DisplayName = *patch.DisplayName
	}

	if patch.Name != nil {
		o.Name = *patch.Name
	}

	if patch.Header != nil {
		o.Header = *patch.Header
	}

	if patch.Purpose != nil {
		o.Purpose = *patch.Purpose
	}

	if patch.GroupConstrained != nil {
		o.GroupConstrained = patch.GroupConstrained
	}
}

func (o *Channel) MakeNonNil() {
	if o.Props == nil {
		o.Props = make(map[string]interface{})
	}
}

func (o *Channel) AddProp(key string, value interface{}) {
	o.MakeNonNil()

	o.Props[key] = value
}

func (o *Channel) IsGroupConstrained() bool {
	return o.GroupConstrained != nil && *o.GroupConstrained
}

func (o *Channel) IsShared() bool {
	return o.Shared != nil && *o.Shared
}

func (o *Channel) GetOtherUserIDForDM(userID string) string {
	if o.Type != ChannelTypeDirect {
		return ""
	}

	userIDs := strings.Split(o.Name, "__")

	var otherUserID string

	if userIDs[0] != userIDs[1] {
		if userIDs[0] == userID {
			otherUserID = userIDs[1]
		} else {
			otherUserID = userIDs[0]
		}
	}

	return otherUserID
}

func GetDMNameFromIDs(userID1, userID2 string) string {
	if userID1 > userID2 {
		return userID2 + "__" + userID1
	}
	return userID1 + "__" + userID2
}

func GetGroupDisplayNameFromUsers(users []*User, truncate bool) string {
	usernames := make([]string, len(users))
	for index, user := range users {
		usernames[index] = user.Username
	}

	sort.Strings(usernames)

	name := strings.Join(usernames, ", ")

	if truncate && len(name) > ChannelNameMaxLength {
		name = name[:ChannelNameMaxLength]
	}

	return name
}

func GetGroupNameFromUserIDs(userIDs []string) string {
	sort.Strings(userIDs)

	h := sha1.New()
	for _, id := range userIDs {
		io.WriteString(h, id)
	}

	return hex.EncodeToString(h.Sum(nil))
}
