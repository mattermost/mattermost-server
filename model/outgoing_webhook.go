// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type OutgoingWebhook struct {
	ID           string      `json:"id"`
	Token        string      `json:"token"`
	CreateAt     int64       `json:"create_at"`
	UpdateAt     int64       `json:"update_at"`
	DeleteAt     int64       `json:"delete_at"`
	CreatorID    string      `json:"creator_id"`
	ChannelID    string      `json:"channel_id"`
	TeamID       string      `json:"team_id"`
	TriggerWords StringArray `json:"trigger_words"`
	TriggerWhen  int         `json:"trigger_when"`
	CallbackURLs StringArray `json:"callback_urls"`
	DisplayName  string      `json:"display_name"`
	Description  string      `json:"description"`
	ContentType  string      `json:"content_type"`
	Username     string      `json:"username"`
	IconURL      string      `json:"icon_url"`
}

type OutgoingWebhookPayload struct {
	Token       string `json:"token"`
	TeamID      string `json:"team_id"`
	TeamDomain  string `json:"team_domain"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Timestamp   int64  `json:"timestamp"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	PostID      string `json:"post_id"`
	Text        string `json:"text"`
	TriggerWord string `json:"trigger_word"`
	FileIDs     string `json:"file_ids"`
}

type OutgoingWebhookResponse struct {
	Text         *string            `json:"text"`
	Username     string             `json:"username"`
	IconURL      string             `json:"icon_url"`
	Props        StringInterface    `json:"props"`
	Attachments  []*SlackAttachment `json:"attachments"`
	Type         string             `json:"type"`
	ResponseType string             `json:"response_type"`
}

const OutgoingHookResponseTypeComment = "comment"

func (o *OutgoingWebhookPayload) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func (o *OutgoingWebhookPayload) ToFormValues() string {
	v := url.Values{}
	v.Set("token", o.Token)
	v.Set("team_id", o.TeamID)
	v.Set("team_domain", o.TeamDomain)
	v.Set("channel_id", o.ChannelID)
	v.Set("channel_name", o.ChannelName)
	v.Set("timestamp", strconv.FormatInt(o.Timestamp/1000, 10))
	v.Set("user_id", o.UserID)
	v.Set("user_name", o.UserName)
	v.Set("post_id", o.PostID)
	v.Set("text", o.Text)
	v.Set("trigger_word", o.TriggerWord)
	v.Set("file_ids", o.FileIDs)

	return v.Encode()
}

func (o *OutgoingWebhook) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func OutgoingWebhookFromJSON(data io.Reader) *OutgoingWebhook {
	var o *OutgoingWebhook
	json.NewDecoder(data).Decode(&o)
	return o
}

func OutgoingWebhookListToJSON(l []*OutgoingWebhook) string {
	b, _ := json.Marshal(l)
	return string(b)
}

func OutgoingWebhookListFromJSON(data io.Reader) []*OutgoingWebhook {
	var o []*OutgoingWebhook
	json.NewDecoder(data).Decode(&o)
	return o
}

func (o *OutgoingWebhookResponse) ToJSON() string {
	b, _ := json.Marshal(o)
	return string(b)
}

func OutgoingWebhookResponseFromJSON(data io.Reader) (*OutgoingWebhookResponse, error) {
	var o *OutgoingWebhookResponse
	err := json.NewDecoder(data).Decode(&o)
	if err == io.EOF {
		return nil, nil
	}
	return o, err
}

func (o *OutgoingWebhook) IsValid() *AppError {

	if !IsValidID(o.ID) {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.id.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.Token) != 26 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.token.app_error", nil, "", http.StatusBadRequest)
	}

	if o.CreateAt == 0 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.create_at.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if o.UpdateAt == 0 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.update_at.app_error", nil, "id="+o.ID, http.StatusBadRequest)
	}

	if !IsValidID(o.CreatorID) {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.user_id.app_error", nil, "", http.StatusBadRequest)
	}

	if o.ChannelID != "" && !IsValidID(o.ChannelID) {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.channel_id.app_error", nil, "", http.StatusBadRequest)
	}

	if !IsValidID(o.TeamID) {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.team_id.app_error", nil, "", http.StatusBadRequest)
	}

	if len(fmt.Sprintf("%s", o.TriggerWords)) > 1024 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.words.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.TriggerWords) != 0 {
		for _, triggerWord := range o.TriggerWords {
			if triggerWord == "" {
				return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.trigger_words.app_error", nil, "", http.StatusBadRequest)
			}
		}
	}

	if len(o.CallbackURLs) == 0 || len(fmt.Sprintf("%s", o.CallbackURLs)) > 1024 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.callback.app_error", nil, "", http.StatusBadRequest)
	}

	for _, callback := range o.CallbackURLs {
		if !IsValidHttpURL(callback) {
			return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.url.app_error", nil, "", http.StatusBadRequest)
		}
	}

	if len(o.DisplayName) > 64 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.display_name.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.Description) > 500 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.description.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.ContentType) > 128 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.content_type.app_error", nil, "", http.StatusBadRequest)
	}

	if o.TriggerWhen > 1 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.is_valid.content_type.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.Username) > 64 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.username.app_error", nil, "", http.StatusBadRequest)
	}

	if len(o.IconURL) > 1024 {
		return NewAppError("OutgoingWebhook.IsValid", "model.outgoing_hook.icon_url.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (o *OutgoingWebhook) PreSave() {
	if o.ID == "" {
		o.ID = NewID()
	}

	if o.Token == "" {
		o.Token = NewID()
	}

	o.CreateAt = GetMillis()
	o.UpdateAt = o.CreateAt
}

func (o *OutgoingWebhook) PreUpdate() {
	o.UpdateAt = GetMillis()
}

func (o *OutgoingWebhook) TriggerWordExactMatch(word string) bool {
	if word == "" {
		return false
	}

	for _, trigger := range o.TriggerWords {
		if trigger == word {
			return true
		}
	}

	return false
}

func (o *OutgoingWebhook) TriggerWordStartsWith(word string) bool {
	if word == "" {
		return false
	}

	for _, trigger := range o.TriggerWords {
		if strings.HasPrefix(word, trigger) {
			return true
		}
	}

	return false
}

func (o *OutgoingWebhook) GetTriggerWord(word string, isExactMatch bool) (triggerWord string) {
	if word == "" {
		return
	}

	if isExactMatch {
		for _, trigger := range o.TriggerWords {
			if trigger == word {
				triggerWord = trigger
				break
			}
		}
	} else {
		for _, trigger := range o.TriggerWords {
			if strings.HasPrefix(word, trigger) {
				triggerWord = trigger
				break
			}
		}
	}

	return triggerWord
}
