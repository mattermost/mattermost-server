// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
)

func ImportLineFromTeam(team *model.TeamForExport) *LineImportData {
	return &LineImportData{
		Type: "team",
		Team: &TeamImportData{
			Name:            &team.Name,
			DisplayName:     &team.DisplayName,
			Type:            &team.Type,
			Description:     &team.Description,
			AllowOpenInvite: &team.AllowOpenInvite,
			Scheme:          team.SchemeName,
		},
	}
}

func ImportLineFromChannel(channel *model.ChannelForExport) *LineImportData {
	return &LineImportData{
		Type: "channel",
		Channel: &ChannelImportData{
			Team:        &channel.TeamName,
			Name:        &channel.Name,
			DisplayName: &channel.DisplayName,
			Type:        &channel.Type,
			Header:      &channel.Header,
			Purpose:     &channel.Purpose,
			Scheme:      channel.SchemeName,
		},
	}
}

func ImportLineFromDirectChannel(channel *model.DirectChannelForExport) *LineImportData {
	channelMembers := *channel.Members
	if len(channelMembers) == 1 {
		channelMembers = []string{channelMembers[0], channelMembers[0]}
	}
	return &LineImportData{
		Type: "direct_channel",
		DirectChannel: &DirectChannelImportData{
			Header:  &channel.Header,
			Members: &channelMembers,
		},
	}
}

func ImportLineFromUser(user *model.User, exportedPrefs map[string]*string) *LineImportData {
	// Bulk Importer doesn't accept "empty string" for AuthService.
	var authService *string
	if user.AuthService != "" {
		authService = &user.AuthService
	}

	return &LineImportData{
		Type: "user",
		User: &UserImportData{
			Username:           &user.Username,
			Email:              &user.Email,
			AuthService:        authService,
			AuthData:           user.AuthData,
			Nickname:           &user.Nickname,
			FirstName:          &user.FirstName,
			LastName:           &user.LastName,
			Position:           &user.Position,
			Roles:              &user.Roles,
			Locale:             &user.Locale,
			UseMarkdownPreview: exportedPrefs["UseMarkdownPreview"],
			UseFormatting:      exportedPrefs["UseFormatting"],
			ShowUnreadSection:  exportedPrefs["ShowUnreadSection"],
			Theme:              exportedPrefs["Theme"],
			UseMilitaryTime:    exportedPrefs["UseMilitaryTime"],
			CollapsePreviews:   exportedPrefs["CollapsePreviews"],
			MessageDisplay:     exportedPrefs["MessageDisplay"],
			ChannelDisplayMode: exportedPrefs["ChannelDisplayMode"],
			TutorialStep:       exportedPrefs["TutorialStep"],
			EmailInterval:      exportedPrefs["EmailInterval"],
			DeleteAt:           &user.DeleteAt,
		},
	}
}

func ImportUserTeamDataFromTeamMember(member *model.TeamMemberForExport) *UserTeamImportData {
	rolesList := strings.Fields(member.Roles)
	if member.SchemeAdmin {
		rolesList = append(rolesList, model.TeamAdminRoleID)
	}
	if member.SchemeUser {
		rolesList = append(rolesList, model.TeamUserRoleID)
	}
	if member.SchemeGuest {
		rolesList = append(rolesList, model.TeamGuestRoleID)
	}
	roles := strings.Join(rolesList, " ")
	return &UserTeamImportData{
		Name:  &member.TeamName,
		Roles: &roles,
	}
}

func ImportUserChannelDataFromChannelMemberAndPreferences(member *model.ChannelMemberForExport, preferences *model.Preferences) *UserChannelImportData {
	rolesList := strings.Fields(member.Roles)
	if member.SchemeAdmin {
		rolesList = append(rolesList, model.ChannelAdminRoleID)
	}
	if member.SchemeUser {
		rolesList = append(rolesList, model.ChannelUserRoleID)
	}
	if member.SchemeGuest {
		rolesList = append(rolesList, model.ChannelGuestRoleID)
	}
	props := member.NotifyProps
	notifyProps := UserChannelNotifyPropsImportData{}

	desktop, exist := props[model.DesktopNotifyProp]
	if exist {
		notifyProps.Desktop = &desktop
	}
	mobile, exist := props[model.PushNotifyProp]
	if exist {
		notifyProps.Mobile = &mobile
	}
	markUnread, exist := props[model.MarkUnreadNotifyProp]
	if exist {
		notifyProps.MarkUnread = &markUnread
	}

	favorite := false
	for _, preference := range *preferences {
		if member.ChannelID == preference.Name {
			favorite = true
		}
	}

	roles := strings.Join(rolesList, " ")
	return &UserChannelImportData{
		Name:        &member.ChannelName,
		Roles:       &roles,
		NotifyProps: &notifyProps,
		Favorite:    &favorite,
	}
}

func ImportLineForPost(post *model.PostForExport) *LineImportData {
	return &LineImportData{
		Type: "post",
		Post: &PostImportData{
			Team:     &post.TeamName,
			Channel:  &post.ChannelName,
			User:     &post.Username,
			Message:  &post.Message,
			Props:    &post.Props,
			CreateAt: &post.CreateAt,
		},
	}
}

func ImportLineForDirectPost(post *model.DirectPostForExport) *LineImportData {
	channelMembers := *post.ChannelMembers
	if len(channelMembers) == 1 {
		channelMembers = []string{channelMembers[0], channelMembers[0]}
	}
	return &LineImportData{
		Type: "direct_post",
		DirectPost: &DirectPostImportData{
			ChannelMembers: &channelMembers,
			User:           &post.User,
			Message:        &post.Message,
			Props:          &post.Props,
			CreateAt:       &post.CreateAt,
		},
	}
}

func ImportReplyFromPost(post *model.ReplyForExport) *ReplyImportData {
	return &ReplyImportData{
		User:     &post.Username,
		Message:  &post.Message,
		CreateAt: &post.CreateAt,
	}
}

func ImportReactionFromPost(user *model.User, reaction *model.Reaction) *ReactionImportData {
	return &ReactionImportData{
		User:      &user.Username,
		EmojiName: &reaction.EmojiName,
		CreateAt:  &reaction.CreateAt,
	}
}

func ImportLineFromEmoji(emoji *model.Emoji, filePath string) *LineImportData {
	return &LineImportData{
		Type: "emoji",
		Emoji: &EmojiImportData{
			Name:  &emoji.Name,
			Image: &filePath,
		},
	}
}
