// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	BOT_DISPLAY_NAME_MAX_RUNES   = USER_FIRST_NAME_MAX_RUNES
	BOT_DESCRIPTION_MAX_RUNES    = 1024
	BOT_CREATOR_ID_MAX_RUNES     = KEY_VALUE_PLUGIN_ID_MAX_RUNES // UserId or PluginId
	BOT_WARN_METRIC_BOT_USERNAME = "mattermost-advisor"
	BOT_SYSTEM_BOT_USERNAME      = "system-bot"
)

// Bot is a special type of User meant for programmatic interactions.
// Note that the primary key of a bot is the UserId, and matches the primary key of the
// corresponding user.
type Bot struct {
	UserId         string `json:"user_id"`
	Username       string `json:"username"`
	DisplayName    string `json:"display_name,omitempty"`
	Description    string `json:"description,omitempty"`
	OwnerId        string `json:"owner_id"`
	LastIconUpdate int64  `json:"last_icon_update,omitempty"`
	CreateAt       int64  `json:"create_at"`
	UpdateAt       int64  `json:"update_at"`
	DeleteAt       int64  `json:"delete_at"`
}

// BotPatch is a description of what fields to update on an existing bot.
type BotPatch struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	Description *string `json:"description"`
}

// BotGetOptions acts as a filter on bulk bot fetching queries.
type BotGetOptions struct {
	OwnerId        string
	IncludeDeleted bool
	OnlyOrphaned   bool
	Page           int
	PerPage        int
}

// BotList is a list of bots.
type BotList []*Bot

// Trace describes the minimum information required to identify a bot for the purpose of logging.
func (b *Bot) Trace() map[string]interface{} {
	return map[string]interface{}{"user_id": b.UserId}
}

// Clone returns a shallow copy of the bot.
func (b *Bot) Clone() *Bot {
	copy := *b
	return &copy
}

// IsValidCreate validates bot for Create call. This skips validations of fields that are auto-filled on Create
func (b *Bot) IsValidCreate() *AppError {
	if !IsValidUsername(b.Username) {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.username.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if utf8.RuneCountInString(b.DisplayName) > BOT_DISPLAY_NAME_MAX_RUNES {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.user_id.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if utf8.RuneCountInString(b.Description) > BOT_DESCRIPTION_MAX_RUNES {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.description.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if b.OwnerId == "" || utf8.RuneCountInString(b.OwnerId) > BOT_CREATOR_ID_MAX_RUNES {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.creator_id.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	return nil
}

// IsValid validates the bot and returns an error if it isn't configured correctly.
func (b *Bot) IsValid() *AppError {

	if !IsValidId(b.UserId) {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.user_id.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if b.CreateAt == 0 {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.create_at.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if b.UpdateAt == 0 {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.update_at.app_error", b.Trace(), "", http.StatusBadRequest)
	}
	return b.IsValidCreate()
}

// PreSave should be run before saving a new bot to the database.
func (b *Bot) PreSave() {
	b.CreateAt = GetMillis()
	b.UpdateAt = b.CreateAt
	b.DeleteAt = 0
}

// PreUpdate should be run before saving an updated bot to the database.
func (b *Bot) PreUpdate() {
	b.UpdateAt = GetMillis()
}

// Etag generates an etag for caching.
func (b *Bot) Etag() string {
	return Etag(b.UserId, b.UpdateAt)
}

// ToJson serializes the bot to json.
func (b *Bot) ToJson() []byte {
	data, _ := json.Marshal(b)
	return data
}

// BotFromJson deserializes a bot from json.
func BotFromJson(data io.Reader) *Bot {
	var bot *Bot
	json.NewDecoder(data).Decode(&bot)
	return bot
}

// Patch modifies an existing bot with optional fields from the given patch.
// TODO 6.0: consider returning a boolean to indicate whether or not the patch
// applied any changes.
func (b *Bot) Patch(patch *BotPatch) {
	if patch.Username != nil {
		b.Username = *patch.Username
	}

	if patch.DisplayName != nil {
		b.DisplayName = *patch.DisplayName
	}

	if patch.Description != nil {
		b.Description = *patch.Description
	}
}

// WouldPatch returns whether or not the given patch would be applied or not.
func (b *Bot) WouldPatch(patch *BotPatch) bool {
	if patch == nil {
		return false
	}
	if patch.Username != nil && *patch.Username != b.Username {
		return true
	}
	if patch.DisplayName != nil && *patch.DisplayName != b.DisplayName {
		return true
	}
	if patch.Description != nil && *patch.Description != b.Description {
		return true
	}
	return false
}

// ToJson serializes the bot patch to json.
func (b *BotPatch) ToJson() []byte {
	data, err := json.Marshal(b)
	if err != nil {
		return nil
	}

	return data
}

// BotPatchFromJson deserializes a bot patch from json.
func BotPatchFromJson(data io.Reader) *BotPatch {
	decoder := json.NewDecoder(data)
	var botPatch BotPatch
	err := decoder.Decode(&botPatch)
	if err != nil {
		return nil
	}

	return &botPatch
}

// UserFromBot returns a user model describing the bot fields stored in the User store.
func UserFromBot(b *Bot) *User {
	return &User{
		Id:        b.UserId,
		Username:  b.Username,
		Email:     NormalizeEmail(fmt.Sprintf("%s@localhost", b.Username)),
		FirstName: b.DisplayName,
		Roles:     SYSTEM_USER_ROLE_ID,
	}
}

// BotFromUser returns a bot model given a user model
func BotFromUser(u *User) *Bot {
	return &Bot{
		OwnerId:     u.Id,
		UserId:      u.Id,
		Username:    u.Username,
		DisplayName: u.GetDisplayName(SHOW_USERNAME),
	}
}

// BotListFromJson deserializes a list of bots from json.
func BotListFromJson(data io.Reader) BotList {
	var bots BotList
	json.NewDecoder(data).Decode(&bots)
	return bots
}

// ToJson serializes a list of bots to json.
func (l *BotList) ToJson() []byte {
	b, _ := json.Marshal(l)
	return b
}

// Etag computes the etag for a list of bots.
func (l *BotList) Etag() string {
	id := "0"
	var t int64 = 0
	var delta int64 = 0

	for _, v := range *l {
		if v.UpdateAt > t {
			t = v.UpdateAt
			id = v.UserId
		}

	}

	return Etag(id, t, delta, len(*l))
}

// MakeBotNotFoundError creates the error returned when a bot does not exist, or when the user isn't allowed to query the bot.
// The errors must the same in both cases to avoid leaking that a user is a bot.
func MakeBotNotFoundError(userId string) *AppError {
	return NewAppError("SqlBotStore.Get", "store.sql_bot.get.missing.app_error", map[string]interface{}{"user_id": userId}, "", http.StatusNotFound)
}

func IsBotDMChannel(channel *Channel, botUserID string) bool {
	if channel.Type != CHANNEL_DIRECT {
		return false
	}

	if !strings.HasPrefix(channel.Name, botUserID+"__") && !strings.HasSuffix(channel.Name, "__"+botUserID) {
		return false
	}

	return true
}
