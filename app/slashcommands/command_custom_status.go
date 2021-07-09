// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type CustomStatusProvider struct {
}

const (
	CmdCustomStatus      = app.CmdCustomStatusTrigger
	CmdCustomStatusClear = "clear"
)

func init() {
	app.RegisterCommandProvider(&CustomStatusProvider{})
}

func (*CustomStatusProvider) GetTrigger() string {
	return CmdCustomStatus
}

func (*CustomStatusProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdCustomStatus,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_custom_status.desc"),
		AutoCompleteHint: T("api.command_custom_status.hint"),
		DisplayName:      T("api.command_custom_status.name"),
	}
}

func (*CustomStatusProvider) DoCommand(a *app.App, c *request.Context, args *model.CommandArgs, message string) *model.CommandResponse {
	if !*a.Config().TeamSettings.EnableCustomUserStatuses {
		return nil
	}

	message = strings.TrimSpace(message)
	if message == CmdCustomStatusClear {
		if err := a.RemoveCustomStatus(args.UserID); err != nil {
			mlog.Debug(err.Error())
			return &model.CommandResponse{Text: args.T("api.command_custom_status.clear.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}

		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         args.T("api.command_custom_status.clear.success"),
		}
	}

	customStatus := GetCustomStatus(message)
	customStatus.PreSave()
	if err := a.SetCustomStatus(args.UserID, customStatus); err != nil {
		mlog.Debug(err.Error())
		return &model.CommandResponse{Text: args.T("api.command_custom_status.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text: args.T("api.command_custom_status.success", map[string]interface{}{
			"EmojiName":     ":" + customStatus.Emoji + ":",
			"StatusMessage": customStatus.Text,
		}),
	}
}

func GetCustomStatus(message string) *model.CustomStatus {
	customStatus := &model.CustomStatus{
		Emoji: model.DefaultCustomStatusEmoji,
		Text:  message,
	}

	firstEmojiLocations := model.EmojiPattern.FindIndex([]byte(message))
	if len(firstEmojiLocations) > 0 && firstEmojiLocations[0] == 0 {
		// emoji found at starting index
		customStatus.Emoji = message[firstEmojiLocations[0]+1 : firstEmojiLocations[1]-1]
		customStatus.Text = strings.TrimSpace(message[firstEmojiLocations[1]:])
		return customStatus
	}

	if message == "" {
		return customStatus
	}

	spaceSeparatedMessage := strings.Fields(message)
	if len(spaceSeparatedMessage) == 0 {
		return customStatus
	}

	emojiString := spaceSeparatedMessage[0]
	var unicode []string
	for utf8.RuneCountInString(emojiString) >= 1 {
		codepoint, size := utf8.DecodeRuneInString(emojiString)
		code := model.RuneToHexadecimalString(codepoint)
		unicode = append(unicode, code)
		emojiString = emojiString[size:]
	}

	unicodeString := removeUnicodeSkinTone(strings.Join(unicode, "-"))
	emoji, count := model.GetEmojiNameFromUnicode(unicodeString)
	if count > 0 {
		customStatus.Emoji = emoji
		textString := strings.Join(spaceSeparatedMessage[1:], " ")
		customStatus.Text = strings.TrimSpace(textString)
	}

	return customStatus
}

func removeUnicodeSkinTone(unicodeString string) string {
	skinToneDetectorRegex := regexp.MustCompile("-(1f3fb|1f3fc|1f3fd|1f3fe|1f3ff)")
	skinToneLocations := skinToneDetectorRegex.FindIndex([]byte(unicodeString))

	if len(skinToneLocations) == 0 {
		return unicodeString
	}
	if _, count := model.GetEmojiNameFromUnicode(unicodeString); count > 0 {
		return unicodeString
	}
	unicodeWithRemovedSkinTone := unicodeString[:skinToneLocations[0]] + unicodeString[skinToneLocations[1]:]
	unicodeWithVariationSelector := unicodeString[:skinToneLocations[0]] + "-fe0f" + unicodeString[skinToneLocations[1]:]
	if _, count := model.GetEmojiNameFromUnicode(unicodeWithRemovedSkinTone); count > 0 {
		unicodeString = unicodeWithRemovedSkinTone
	} else if _, count := model.GetEmojiNameFromUnicode(unicodeWithVariationSelector); count > 0 {
		unicodeString = unicodeWithVariationSelector
	}

	return unicodeString
}
