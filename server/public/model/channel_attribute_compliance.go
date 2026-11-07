// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// ChannelAttributeAdmin is the subset of a channel admin's profile surfaced by
// the required-channel-attribute compliance endpoints. It deliberately omits
// email and auth-provider fields present on User -- this data is reachable by
// anyone who can edit the property field, which is a much lower bar than
// reading arbitrary user profiles.
type ChannelAttributeAdmin struct {
	Id        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Nickname  string `json:"nickname"`
}

// ChannelMissingAttributeValue describes one channel that lacks a value for a
// required-candidate channel attribute.
type ChannelMissingAttributeValue struct {
	ChannelId          string      `json:"channel_id"`
	ChannelName        string      `json:"channel_name"`
	ChannelDisplayName string      `json:"channel_display_name"`
	ChannelType        ChannelType `json:"channel_type"`
	TeamId             string      `json:"team_id"`
	TeamDisplayName    string      `json:"team_display_name"`
	// IsLocal is false for shared/remote channels. Those are informational
	// only -- they are excluded from the required-attribute gate and count.
	IsLocal bool `json:"is_local"`
	// ChannelAdmins is empty when the channel has no channel admin to notify.
	ChannelAdmins []*ChannelAttributeAdmin `json:"channel_admins"`
}

// ChannelsMissingAttributeValueList is the paginated response for the
// "channels without a value" list.
type ChannelsMissingAttributeValueList struct {
	Channels   []*ChannelMissingAttributeValue `json:"channels"`
	TotalCount int64                           `json:"total_count"`
}

// ChannelAttributeComplianceSummary is the lightweight, count-only view used
// by the Required toggle's banner. MessagePreview is server-rendered so the
// notify confirmation modal never drifts from the string the server actually
// sends.
type ChannelAttributeComplianceSummary struct {
	FieldId                   string `json:"field_id,omitempty"`
	Required                  bool   `json:"required"`
	MissingChannelCount       int64  `json:"missing_channel_count"`
	SharedChannelCount        int64  `json:"shared_channel_count"`
	UniqueAdminCount          int64  `json:"unique_admin_count"`
	ChannelsWithoutAdminCount int64  `json:"channels_without_admin_count"`
	MessagePreview            string `json:"message_preview"`
}

// ChannelAttributeNotifyResult is returned synchronously by the notify
// endpoint; delivery of the underlying DMs happens asynchronously afterward,
// so the counts describe who was queued to receive a DM, not who received one.
type ChannelAttributeNotifyResult struct {
	QueuedAdminCount          int64 `json:"queued_admin_count"`
	QueuedChannelCount        int64 `json:"queued_channel_count"`
	ChannelsWithoutAdminCount int64 `json:"channels_without_admin_count"`
	// Truncated is true when the (admin, channel) assignment scan hit its
	// batch cap -- some admins may not have been queued for every one of
	// their affected channels.
	Truncated bool `json:"truncated"`
}

// ChannelAttributeAdminAssignment is one (channel admin, affected channel)
// pair, used internally to build the notify fan-out. Not serialized over the
// wire.
type ChannelAttributeAdminAssignment struct {
	UserId             string
	Username           string
	Locale             string
	ChannelId          string
	ChannelName        string
	ChannelDisplayName string
	TeamId             string
	TeamName           string
}
