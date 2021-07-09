// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	PageDefault        = 0
	PerPageDefault     = 60
	PerPageMaximum     = 200
	LogsPerPageDefault = 10000
	LogsPerPageMaximum = 10000
	LimitDefault       = 60
	LimitMaximum       = 200
)

type Params struct {
	UserID                    string
	TeamID                    string
	InviteID                  string
	TokenID                   string
	ThreadID                  string
	Timestamp                 int64
	ChannelID                 string
	PostID                    string
	PolicyID                  string
	FileID                    string
	Filename                  string
	UploadID                  string
	PluginID                  string
	CommandID                 string
	HookID                    string
	ReportID                  string
	EmojiID                   string
	AppID                     string
	Email                     string
	Username                  string
	TeamName                  string
	ChannelName               string
	PreferenceName            string
	EmojiName                 string
	Category                  string
	Service                   string
	JobID                     string
	JobType                   string
	ActionID                  string
	RoleID                    string
	RoleName                  string
	SchemeID                  string
	Scope                     string
	GroupID                   string
	Page                      int
	PerPage                   int
	LogsPerPage               int
	Permanent                 bool
	RemoteID                  string
	SyncableID                string
	SyncableType              model.GroupSyncableType
	BotUserID                 string
	Q                         string
	IsLinked                  *bool
	IsConfigured              *bool
	NotAssociatedToTeam       string
	NotAssociatedToChannel    string
	Paginate                  *bool
	IncludeMemberCount        bool
	NotAssociatedToGroup      string
	ExcludeDefaultChannels    bool
	LimitAfter                int
	LimitBefore               int
	GroupIDs                  string
	IncludeTotalCount         bool
	IncludeDeleted            bool
	FilterAllowReference      bool
	FilterParentTeamPermitted bool
	CategoryID                string
	WarnMetricID              string
	ExportName                string
	ExcludePolicyConstrained  bool

	// Cloud
	InvoiceID string
}

func ParamsFromRequest(r *http.Request) *Params {
	params := &Params{}

	props := mux.Vars(r)
	query := r.URL.Query()

	if val, ok := props["user_id"]; ok {
		params.UserID = val
	}

	if val, ok := props["team_id"]; ok {
		params.TeamID = val
	}

	if val, ok := props["category_id"]; ok {
		params.CategoryID = val
	}

	if val, ok := props["invite_id"]; ok {
		params.InviteID = val
	}

	if val, ok := props["token_id"]; ok {
		params.TokenID = val
	}

	if val, ok := props["thread_id"]; ok {
		params.ThreadID = val
	}

	if val, ok := props["channel_id"]; ok {
		params.ChannelID = val
	} else {
		params.ChannelID = query.Get("channel_id")
	}

	if val, ok := props["post_id"]; ok {
		params.PostID = val
	}

	if val, ok := props["policy_id"]; ok {
		params.PolicyID = val
	}

	if val, ok := props["file_id"]; ok {
		params.FileID = val
	}

	params.Filename = query.Get("filename")

	if val, ok := props["upload_id"]; ok {
		params.UploadID = val
	}

	if val, ok := props["plugin_id"]; ok {
		params.PluginID = val
	}

	if val, ok := props["command_id"]; ok {
		params.CommandID = val
	}

	if val, ok := props["hook_id"]; ok {
		params.HookID = val
	}

	if val, ok := props["report_id"]; ok {
		params.ReportID = val
	}

	if val, ok := props["emoji_id"]; ok {
		params.EmojiID = val
	}

	if val, ok := props["app_id"]; ok {
		params.AppID = val
	}

	if val, ok := props["email"]; ok {
		params.Email = val
	}

	if val, ok := props["username"]; ok {
		params.Username = val
	}

	if val, ok := props["team_name"]; ok {
		params.TeamName = strings.ToLower(val)
	}

	if val, ok := props["channel_name"]; ok {
		params.ChannelName = strings.ToLower(val)
	}

	if val, ok := props["category"]; ok {
		params.Category = val
	}

	if val, ok := props["service"]; ok {
		params.Service = val
	}

	if val, ok := props["preference_name"]; ok {
		params.PreferenceName = val
	}

	if val, ok := props["emoji_name"]; ok {
		params.EmojiName = val
	}

	if val, ok := props["job_id"]; ok {
		params.JobID = val
	}

	if val, ok := props["job_type"]; ok {
		params.JobType = val
	}

	if val, ok := props["action_id"]; ok {
		params.ActionID = val
	}

	if val, ok := props["role_id"]; ok {
		params.RoleID = val
	}

	if val, ok := props["role_name"]; ok {
		params.RoleName = val
	}

	if val, ok := props["scheme_id"]; ok {
		params.SchemeID = val
	}

	if val, ok := props["group_id"]; ok {
		params.GroupID = val
	}

	if val, ok := props["remote_id"]; ok {
		params.RemoteID = val
	}

	if val, ok := props["invoice_id"]; ok {
		params.InvoiceID = val
	}

	params.Scope = query.Get("scope")

	if val, err := strconv.Atoi(query.Get("page")); err != nil || val < 0 {
		params.Page = PageDefault
	} else {
		params.Page = val
	}

	if val, err := strconv.ParseInt(props["timestamp"], 10, 64); err != nil || val < 0 {
		params.Timestamp = 0
	} else {
		params.Timestamp = val
	}

	if val, err := strconv.ParseBool(query.Get("permanent")); err == nil {
		params.Permanent = val
	}

	if val, err := strconv.Atoi(query.Get("per_page")); err != nil || val < 0 {
		params.PerPage = PerPageDefault
	} else if val > PerPageMaximum {
		params.PerPage = PerPageMaximum
	} else {
		params.PerPage = val
	}

	if val, err := strconv.Atoi(query.Get("logs_per_page")); err != nil || val < 0 {
		params.LogsPerPage = LogsPerPageDefault
	} else if val > LogsPerPageMaximum {
		params.LogsPerPage = LogsPerPageMaximum
	} else {
		params.LogsPerPage = val
	}

	if val, err := strconv.Atoi(query.Get("limit_after")); err != nil || val < 0 {
		params.LimitAfter = LimitDefault
	} else if val > LimitMaximum {
		params.LimitAfter = LimitMaximum
	} else {
		params.LimitAfter = val
	}

	if val, err := strconv.Atoi(query.Get("limit_before")); err != nil || val < 0 {
		params.LimitBefore = LimitDefault
	} else if val > LimitMaximum {
		params.LimitBefore = LimitMaximum
	} else {
		params.LimitBefore = val
	}

	if val, ok := props["syncable_id"]; ok {
		params.SyncableID = val
	}

	if val, ok := props["syncable_type"]; ok {
		switch val {
		case "teams":
			params.SyncableType = model.GroupSyncableTypeTeam
		case "channels":
			params.SyncableType = model.GroupSyncableTypeChannel
		}
	}

	if val, ok := props["bot_user_id"]; ok {
		params.BotUserID = val
	}

	params.Q = query.Get("q")

	if val, err := strconv.ParseBool(query.Get("is_linked")); err == nil {
		params.IsLinked = &val
	}

	if val, err := strconv.ParseBool(query.Get("is_configured")); err == nil {
		params.IsConfigured = &val
	}

	params.NotAssociatedToTeam = query.Get("not_associated_to_team")
	params.NotAssociatedToChannel = query.Get("not_associated_to_channel")

	if val, err := strconv.ParseBool(query.Get("filter_allow_reference")); err == nil {
		params.FilterAllowReference = val
	}

	if val, err := strconv.ParseBool(query.Get("filter_parent_team_permitted")); err == nil {
		params.FilterParentTeamPermitted = val
	}

	if val, err := strconv.ParseBool(query.Get("paginate")); err == nil {
		params.Paginate = &val
	}

	if val, err := strconv.ParseBool(query.Get("include_member_count")); err == nil {
		params.IncludeMemberCount = val
	}

	params.NotAssociatedToGroup = query.Get("not_associated_to_group")

	if val, err := strconv.ParseBool(query.Get("exclude_default_channels")); err == nil {
		params.ExcludeDefaultChannels = val
	}

	params.GroupIDs = query.Get("group_ids")

	if val, err := strconv.ParseBool(query.Get("include_total_count")); err == nil {
		params.IncludeTotalCount = val
	}

	if val, err := strconv.ParseBool(query.Get("include_deleted")); err == nil {
		params.IncludeDeleted = val
	}

	if val, ok := props["warn_metric_id"]; ok {
		params.WarnMetricID = val
	}

	if val, ok := props["export_name"]; ok {
		params.ExportName = val
	}

	if val, err := strconv.ParseBool(query.Get("exclude_policy_constrained")); err == nil {
		params.ExcludePolicyConstrained = val
	}

	return params
}
