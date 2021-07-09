// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const (
	CommandMethodPost = "P"
	CommandMethodGet  = "G"
	MinTriggerLength  = 1
	MaxTriggerLength  = 128
)

type Command struct {
	ID               string `json:"id"`
	Token            string `json:"token"`
	CreateAt         int64  `json:"create_at"`
	UpdateAt         int64  `json:"update_at"`
	DeleteAt         int64  `json:"delete_at"`
	CreatorID        string `json:"creator_id"`
	TeamID           string `json:"team_id"`
	Trigger          string `json:"trigger"`
	Method           string `json:"method"`
	Username         string `json:"username"`
	IconURL          string `json:"icon_url"`
	AutoComplete     bool   `json:"auto_complete"`
	AutoCompleteDesc string `json:"auto_complete_desc"`
	AutoCompleteHint string `json:"auto_complete_hint"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description"`
	URL              string `json:"url"`
	// PluginId records the id of the plugin that created this Command. If it is blank, the Command
	// was not created by a plugin.
	PluginID         string            `json:"plugin_id"`
	AutocompleteData *AutocompleteData `db:"-" json:"autocomplete_data,omitempty"`
	// AutocompleteIconData is a base64 encoded svg
	AutocompleteIconData string `db:"-" json:"autocomplete_icon_data,omitempty"`
}

func (o *Command) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func CommandFromJSON(data io.Reader) *Command {
	var o *Command
	json.NewDecoder(data).Decode(&o)
	return o
}

func CommandListToJSON(l []*Command) string {
	b, _ := json.Marshal(l)
	return string(b)
}

func CommandListFromJSON(data io.Reader) []*Command {
	var o []*Command
	json.NewDecoder(data).Decode(&o)
	return o
}

func (o *Command) IsValid() *AppError {

	if !IsValidID(o.ID) {
		return NewAppError("Command.IsValid", "model.command.is_valid.id.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.Token) != 26 {
		return NewAppError("Command.IsValid", "model.command.is_valid.token.app_error", nil, "", http.StatusBadRequest)
	}

	if o.CreateAt == 0 {
		return NewAppError("Command.IsValid", "model.command.is_valid.create_at.app_error", nil, "", http.StatusBadRequest)
	}

	if o.UpdateAt == 0 {
		return NewAppError("Command.IsValid", "model.command.is_valid.update_at.app_error", nil, "", http.StatusBadRequest)
	}

	// If the CreatorId is blank, this should be a command created by a plugin.
	if o.CreatorID == "" && !IsValidPluginID(o.PluginID) {
		return NewAppError("Command.IsValid", "model.command.is_valid.plugin_id.app_error", nil, "", http.StatusBadRequest)
	}

	// If the PluginId is blank, this should be a command associated with a userId.
	if o.PluginID == "" && !IsValidID(o.CreatorID) {
		return NewAppError("Command.IsValid", "model.command.is_valid.user_id.app_error", nil, "", http.StatusBadRequest)
	}

	if o.CreatorID != "" && o.PluginID != "" {
		return NewAppError("Command.IsValid", "model.command.is_valid.plugin_id.app_error", nil, "command cannot have both a CreatorId and a PluginId", http.StatusBadRequest)
	}

	if !IsValidID(o.TeamID) {
		return NewAppError("Command.IsValid", "model.command.is_valid.team_id.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.Trigger) < MinTriggerLength || len(o.Trigger) > MaxTriggerLength || strings.Index(o.Trigger, "/") == 0 || strings.Contains(o.Trigger, " ") {
		return NewAppError("Command.IsValid", "model.command.is_valid.trigger.app_error", nil, "", http.StatusBadRequest)
	}

	if o.URL == "" || len(o.URL) > 1024 {
		return NewAppError("Command.IsValid", "model.command.is_valid.url.app_error", nil, "", http.StatusBadRequest)
	}

	if !IsValidHTTPURL(o.URL) {
		return NewAppError("Command.IsValid", "model.command.is_valid.url_http.app_error", nil, "", http.StatusBadRequest)
	}

	if !(o.Method == CommandMethodGet || o.Method == CommandMethodPost) {
		return NewAppError("Command.IsValid", "model.command.is_valid.method.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.DisplayName) > 64 {
		return NewAppError("Command.IsValid", "model.command.is_valid.display_name.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.Description) > 128 {
		return NewAppError("Command.IsValid", "model.command.is_valid.description.app_error", nil, "", http.StatusBadRequest)
	}

	if o.AutocompleteData != nil {
		if err := o.AutocompleteData.IsValid(); err != nil {
			return NewAppError("Command.IsValid", "model.command.is_valid.autocomplete_data.app_error", nil, err.Error(), http.StatusBadRequest)
		}
	}

	return nil
}

func (o *Command) PreSave() {
	if o.ID == "" {
		o.ID = NewID()
	}

	if o.Token == "" {
		o.Token = NewID()
	}

	o.CreateAt = GetMillis()
	o.UpdateAt = o.CreateAt
}

func (o *Command) PreUpdate() {
	o.UpdateAt = GetMillis()
}

func (o *Command) Sanitize() {
	o.Token = ""
	o.CreatorID = ""
	o.Method = ""
	o.URL = ""
	o.Username = ""
	o.IconURL = ""
}
