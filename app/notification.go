// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/markdown"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

func (a *App) SendNotifications(post *model.Post, team *model.Team, channel *model.Channel, sender *model.User, parentPostList *model.PostList, setOnline bool) ([]string, error) {
	// Do not send notifications in archived channels
	if channel.DeleteAt > 0 {
		return []string{}, nil
	}

	pchan := make(chan store.StoreResult, 1)
	go func() {
		props, err := a.Srv().Store.User().GetAllProfilesInChannel(context.Background(), channel.ID, true)
		pchan <- store.StoreResult{Data: props, NErr: err}
		close(pchan)
	}()

	cmnchan := make(chan store.StoreResult, 1)
	go func() {
		props, err := a.Srv().Store.Channel().GetAllChannelMembersNotifyPropsForChannel(channel.ID, true)
		cmnchan <- store.StoreResult{Data: props, NErr: err}
		close(cmnchan)
	}()

	var gchan chan store.StoreResult
	if a.allowGroupMentions(post) {
		gchan = make(chan store.StoreResult, 1)
		go func() {
			groupsMap, err := a.getGroupsAllowedForReferenceInChannel(channel, team)
			gchan <- store.StoreResult{Data: groupsMap, NErr: err}
			close(gchan)
		}()
	}

	var fchan chan store.StoreResult
	if len(post.FileIDs) != 0 {
		fchan = make(chan store.StoreResult, 1)
		go func() {
			fileInfos, err := a.Srv().Store.FileInfo().GetForPost(post.ID, true, false, true)
			fchan <- store.StoreResult{Data: fileInfos, NErr: err}
			close(fchan)
		}()
	}

	result := <-pchan
	if result.NErr != nil {
		return nil, result.NErr
	}
	profileMap := result.Data.(map[string]*model.User)

	result = <-cmnchan
	if result.NErr != nil {
		return nil, result.NErr
	}
	channelMemberNotifyPropsMap := result.Data.(map[string]model.StringMap)

	groups := make(map[string]*model.Group)
	if gchan != nil {
		result = <-gchan
		if result.NErr != nil {
			return nil, result.NErr
		}
		groups = result.Data.(map[string]*model.Group)
	}

	mentions := &ExplicitMentions{}
	allActivityPushUserIDs := []string{}
	var allowChannelMentions bool
	var keywords map[string][]string
	if channel.Type == model.ChannelTypeDirect {
		otherUserID := channel.GetOtherUserIDForDM(post.UserID)

		_, ok := profileMap[otherUserID]
		if ok {
			mentions.addMention(otherUserID, DMMention)
		}

		if post.GetProp("from_webhook") == "true" {
			mentions.addMention(post.UserID, DMMention)
		}
	} else {
		allowChannelMentions = a.allowChannelMentions(post, len(profileMap))
		keywords = a.getMentionKeywordsInChannel(profileMap, allowChannelMentions, channelMemberNotifyPropsMap)

		mentions = getExplicitMentions(post, keywords, groups)
		// Add an implicit mention when a user is added to a channel
		// even if the user has set 'username mentions' to false in account settings.
		if post.Type == model.PostTypeAddToChannel {
			addedUserID, ok := post.GetProp(model.PostPropsAddedUserID).(string)
			if ok {
				mentions.addMention(addedUserID, KeywordMention)
			}
		}

		// Iterate through all groups that were mentioned and insert group members into the list of mentions or potential mentions
		for _, group := range mentions.GroupMentions {
			anyUsersMentionedByGroup, err := a.insertGroupMentions(group, channel, profileMap, mentions)
			if err != nil {
				return nil, err
			}

			if !anyUsersMentionedByGroup {
				a.sendNoUsersNotifiedByGroupInChannel(sender, post, channel, group)
			}
		}

		// get users that have comment thread mentions enabled
		if post.RootID != "" && parentPostList != nil {
			for _, threadPost := range parentPostList.Posts {
				profile := profileMap[threadPost.UserID]
				if profile == nil {
					continue
				}
				// If this is the root post and it was posted by an OAuth bot, don't notify the user
				if threadPost.ID == parentPostList.Order[0] && threadPost.IsFromOAuthBot() {
					continue
				}
				if profile.NotifyProps[model.CommentsNotifyProp] == model.CommentsNotifyAny || (profile.NotifyProps[model.CommentsNotifyProp] == model.CommentsNotifyRoot && threadPost.ID == parentPostList.Order[0]) {
					mentionType := ThreadMention
					if threadPost.ID == parentPostList.Order[0] {
						mentionType = CommentMention
					}

					mentions.addMention(threadPost.UserID, mentionType)
				}
			}
		}

		// prevent the user from mentioning themselves
		if post.GetProp("from_webhook") != "true" {
			mentions.removeMention(post.UserID)
		}

		go func() {
			_, err := a.sendOutOfChannelMentions(sender, post, channel, mentions.OtherPotentialMentions)
			if err != nil {
				mlog.Error("Failed to send warning for out of channel mentions", mlog.String("user_id", sender.ID), mlog.String("post_id", post.ID), mlog.Err(err))
			}
		}()

		// find which users in the channel are set up to always receive mobile notifications
		for _, profile := range profileMap {
			if (profile.NotifyProps[model.PushNotifyProp] == model.UserNotifyAll ||
				channelMemberNotifyPropsMap[profile.ID][model.PushNotifyProp] == model.ChannelNotifyAll) &&
				(post.UserID != profile.ID || post.GetProp("from_webhook") == "true") &&
				!post.IsSystemMessage() {
				allActivityPushUserIDs = append(allActivityPushUserIDs, profile.ID)
			}
		}
	}

	mentionedUsersList := make(model.StringArray, 0, len(mentions.Mentions))
	updateMentionChans := []chan *model.AppError{}
	mentionAutofollowChans := []chan *model.AppError{}
	threadParticipants := map[string]bool{post.UserID: true}
	if *a.Config().ServiceSettings.ThreadAutoFollow && post.RootID != "" {
		var rootMentions *ExplicitMentions
		if parentPostList != nil {
			threadParticipants[parentPostList.Posts[parentPostList.Order[0]].UserID] = true
			if channel.Type != model.ChannelTypeDirect {
				rootPost := parentPostList.Posts[parentPostList.Order[0]]
				rootMentions = getExplicitMentions(rootPost, keywords, groups)
				for id := range rootMentions.Mentions {
					threadParticipants[id] = true
				}
			}
		}
		for id := range mentions.Mentions {
			threadParticipants[id] = true
		}
		// for each mention, make sure to update thread autofollow (if enabled) and update increment mention count
		for id := range threadParticipants {
			mac := make(chan *model.AppError, 1)
			go func(userID string) {
				defer close(mac)
				mentionType, incrementMentions := mentions.Mentions[userID]
				// if the user was not explicitly mentioned, check if they explicitly unfollowed the thread
				if !incrementMentions {
					membership, err := a.Srv().Store.Thread().GetMembershipForUser(userID, post.RootID)
					var nfErr *store.ErrNotFound

					if err != nil && !errors.As(err, &nfErr) {
						mac <- model.NewAppError("SendNotifications", "app.channel.autofollow.app_error", nil, err.Error(), http.StatusInternalServerError)
						return
					}

					if membership != nil && !membership.Following {
						return
					}
				}
				if mentionType == ThreadMention || mentionType == CommentMention {
					incrementMentions = false
				}

				opts := store.ThreadMembershipOpts{
					Following:             true,
					IncrementMentions:     incrementMentions,
					UpdateFollowing:       *a.Config().ServiceSettings.ThreadAutoFollow,
					UpdateViewedTimestamp: userID == post.UserID,
					UpdateParticipants:    userID == post.UserID,
				}
				_, err := a.Srv().Store.Thread().MaintainMembership(userID, post.RootID, opts)
				if err != nil {
					mac <- model.NewAppError("SendNotifications", "app.channel.autofollow.app_error", nil, err.Error(), http.StatusInternalServerError)
					return
				}

				mac <- nil
			}(id)
			mentionAutofollowChans = append(mentionAutofollowChans, mac)
		}
	}
	for id := range mentions.Mentions {
		mentionedUsersList = append(mentionedUsersList, id)

		umc := make(chan *model.AppError, 1)
		go func(userID string) {
			defer close(umc)
			nErr := a.Srv().Store.Channel().IncrementMentionCount(post.ChannelID, userID, *a.Config().ServiceSettings.ThreadAutoFollow, post.RootID == "")
			if nErr != nil {
				umc <- model.NewAppError("SendNotifications", "app.channel.increment_mention_count.app_error", nil, nErr.Error(), http.StatusInternalServerError)
				return
			}
			umc <- nil
		}(id)
		updateMentionChans = append(updateMentionChans, umc)
	}

	notification := &PostNotification{
		Post:       post.Clone(),
		Channel:    channel,
		ProfileMap: profileMap,
		Sender:     sender,
	}

	if *a.Config().EmailSettings.SendEmailNotifications {
		for _, id := range mentionedUsersList {
			if profileMap[id] == nil {
				continue
			}

			//If email verification is required and user email is not verified don't send email.
			if *a.Config().EmailSettings.RequireEmailVerification && !profileMap[id].EmailVerified {
				mlog.Debug("Skipped sending notification email, address not verified.", mlog.String("user_email", profileMap[id].Email), mlog.String("user_id", id))
				continue
			}

			if a.userAllowsEmail(profileMap[id], channelMemberNotifyPropsMap[id], post) {
				senderProfileImage, _, err := a.GetProfileImage(sender)
				if err != nil {
					a.Log().Warn("Unable to get the sender user profile image.", mlog.String("user_id", sender.ID), mlog.Err(err))
				}
				if err := a.sendNotificationEmail(notification, profileMap[id], team, senderProfileImage); err != nil {
					mlog.Warn("Unable to send notification email.", mlog.Err(err))
				}
			}
		}
	}

	// Check for channel-wide mentions in channels that have too many members for those to work
	if int64(len(profileMap)) > *a.Config().TeamSettings.MaxNotificationsPerChannel {
		T := i18n.GetUserTranslations(sender.Locale)

		if mentions.HereMentioned {
			a.SendEphemeralPost(
				post.UserID,
				&model.Post{
					ChannelID: post.ChannelID,
					Message:   T("api.post.disabled_here", map[string]interface{}{"Users": *a.Config().TeamSettings.MaxNotificationsPerChannel}),
					CreateAt:  post.CreateAt + 1,
				},
			)
		}

		if mentions.ChannelMentioned {
			a.SendEphemeralPost(
				post.UserID,
				&model.Post{
					ChannelID: post.ChannelID,
					Message:   T("api.post.disabled_channel", map[string]interface{}{"Users": *a.Config().TeamSettings.MaxNotificationsPerChannel}),
					CreateAt:  post.CreateAt + 1,
				},
			)
		}

		if mentions.AllMentioned {
			a.SendEphemeralPost(
				post.UserID,
				&model.Post{
					ChannelID: post.ChannelID,
					Message:   T("api.post.disabled_all", map[string]interface{}{"Users": *a.Config().TeamSettings.MaxNotificationsPerChannel}),
					CreateAt:  post.CreateAt + 1,
				},
			)
		}
	}

	// Make sure all mention updates are complete to prevent race
	// Probably better to batch these DB updates in the future
	// MUST be completed before push notifications send
	for _, umc := range updateMentionChans {
		if err := <-umc; err != nil {
			mlog.Warn(
				"Failed to update mention count",
				mlog.String("post_id", post.ID),
				mlog.String("channel_id", post.ChannelID),
				mlog.Err(err),
			)
		}
	}

	// Log the problems that might have occurred while auto following the thread
	for _, mac := range mentionAutofollowChans {
		if err := <-mac; err != nil {
			mlog.Warn(
				"Failed to update thread autofollow from mention",
				mlog.String("post_id", post.ID),
				mlog.String("channel_id", post.ChannelID),
				mlog.Err(err),
			)
		}
	}
	sendPushNotifications := false
	if *a.Config().EmailSettings.SendPushNotifications {
		pushServer := *a.Config().EmailSettings.PushNotificationServer
		if license := a.Srv().License(); pushServer == model.MHPNS && (license == nil || !*license.Features.MHPNS) {
			mlog.Warn("Push notifications are disabled. Go to System Console > Notifications > Mobile Push to enable them.")
			sendPushNotifications = false
		} else {
			sendPushNotifications = true
		}
	}

	if sendPushNotifications {
		for _, id := range mentionedUsersList {
			if profileMap[id] == nil {
				continue
			}

			var status *model.Status
			var err *model.AppError
			if status, err = a.GetStatus(id); err != nil {
				status = &model.Status{UserID: id, Status: model.StatusOffline, Manual: false, LastActivityAt: 0, ActiveChannel: ""}
			}

			if ShouldSendPushNotification(profileMap[id], channelMemberNotifyPropsMap[id], true, status, post) {
				mentionType := mentions.Mentions[id]

				replyToThreadType := ""
				if mentionType == ThreadMention {
					replyToThreadType = model.CommentsNotifyAny
				} else if mentionType == CommentMention {
					replyToThreadType = model.CommentsNotifyRoot
				}

				a.sendPushNotification(
					notification,
					profileMap[id],
					mentionType == KeywordMention || mentionType == ChannelMention || mentionType == DMMention,
					mentionType == ChannelMention,
					replyToThreadType,
				)
			} else {
				// register that a notification was not sent
				a.NotificationsLog().Debug("Notification not sent",
					mlog.String("ackId", ""),
					mlog.String("type", model.PushTypeMessage),
					mlog.String("userId", id),
					mlog.String("postId", post.ID),
					mlog.String("status", model.PushNotSent),
				)
			}
		}

		for _, id := range allActivityPushUserIDs {
			if profileMap[id] == nil {
				continue
			}

			if _, ok := mentions.Mentions[id]; !ok {
				var status *model.Status
				var err *model.AppError
				if status, err = a.GetStatus(id); err != nil {
					status = &model.Status{UserID: id, Status: model.StatusOffline, Manual: false, LastActivityAt: 0, ActiveChannel: ""}
				}

				if ShouldSendPushNotification(profileMap[id], channelMemberNotifyPropsMap[id], false, status, post) {
					a.sendPushNotification(
						notification,
						profileMap[id],
						false,
						false,
						"",
					)
				} else {
					// register that a notification was not sent
					a.NotificationsLog().Debug("Notification not sent",
						mlog.String("ackId", ""),
						mlog.String("type", model.PushTypeMessage),
						mlog.String("userId", id),
						mlog.String("postId", post.ID),
						mlog.String("status", model.PushNotSent),
					)
				}
			}
		}
	}

	message := model.NewWebSocketEvent(model.WebsocketEventPosted, "", post.ChannelID, "", nil)

	// Note that PreparePostForClient should've already been called by this point
	message.Add("post", post.ToJSON())

	message.Add("channel_type", channel.Type)
	message.Add("channel_display_name", notification.GetChannelName(model.ShowUsername, ""))
	message.Add("channel_name", channel.Name)
	message.Add("sender_name", notification.GetSenderName(model.ShowUsername, *a.Config().ServiceSettings.EnablePostUsernameOverride))
	message.Add("team_id", team.ID)
	message.Add("set_online", setOnline)

	if len(post.FileIDs) != 0 && fchan != nil {
		message.Add("otherFile", "true")

		var infos []*model.FileInfo
		if result := <-fchan; result.NErr != nil {
			mlog.Warn("Unable to get fileInfo for push notifications.", mlog.String("post_id", post.ID), mlog.Err(result.NErr))
		} else {
			infos = result.Data.([]*model.FileInfo)
		}

		for _, info := range infos {
			if info.IsImage() {
				message.Add("image", "true")
				break
			}
		}
	}

	if len(mentionedUsersList) != 0 {
		message.Add("mentions", model.ArrayToJSON(mentionedUsersList))
	}

	a.Publish(message)

	// If this is a reply in a thread, notify participants
	if a.Config().FeatureFlags.CollapsedThreads && *a.Config().ServiceSettings.CollapsedThreads != model.CollapsedThreadsDisabled && post.RootID != "" {
		followers, err := a.Srv().Store.Thread().GetThreadFollowers(post.RootID)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get thread %q followers", post.RootID)
		}
		for _, uid := range followers {
			sendEvent := *a.Config().ServiceSettings.CollapsedThreads == model.CollapsedThreadsDefaultOn
			// check if a participant has overridden collapsed threads settings
			if preference, err := a.Srv().Store.Preference().Get(uid, model.PreferenceCategoryDisplaySettings, model.PreferenceNameCollapsedThreadsEnabled); err == nil {
				sendEvent = preference.Value == "on"
			}
			if sendEvent {
				message := model.NewWebSocketEvent(model.WebsocketEventThreadUpdated, team.ID, "", uid, nil)
				threadMembership, err := a.Srv().Store.Thread().GetMembershipForUser(uid, post.RootID)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot get thread membership %q for user %q", post.RootID, uid)
				}
				userThread, err := a.Srv().Store.Thread().GetThreadForUser(channel.TeamID, threadMembership, true)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot get thread %q for user %q", post.RootID, uid)
				}
				if userThread != nil {
					a.sanitizeProfiles(userThread.Participants, false)
					userThread.Post.SanitizeProps()
					message.Add("thread", userThread.ToJSON())
					a.Publish(message)
				}
			}
		}

	}
	return mentionedUsersList, nil
}

func (a *App) userAllowsEmail(user *model.User, channelMemberNotificationProps model.StringMap, post *model.Post) bool {
	userAllowsEmails := user.NotifyProps[model.EmailNotifyProp] != "false"
	if channelEmail, ok := channelMemberNotificationProps[model.EmailNotifyProp]; ok {
		if channelEmail != model.ChannelNotifyDefault {
			userAllowsEmails = channelEmail != "false"
		}
	}

	// Remove the user as recipient when the user has muted the channel.
	if channelMuted, ok := channelMemberNotificationProps[model.MarkUnreadNotifyProp]; ok {
		if channelMuted == model.ChannelMarkUnreadMention {
			mlog.Debug("Channel muted for user", mlog.String("user_id", user.ID), mlog.String("channel_mute", channelMuted))
			userAllowsEmails = false
		}
	}

	var status *model.Status
	var err *model.AppError
	if status, err = a.GetStatus(user.ID); err != nil {
		status = &model.Status{
			UserID:         user.ID,
			Status:         model.StatusOffline,
			Manual:         false,
			LastActivityAt: 0,
			ActiveChannel:  "",
		}
	}

	autoResponderRelated := status.Status == model.StatusOutOfOffice || post.Type == model.PostTypeAutoResponder
	emailNotificationsAllowedForStatus := status.Status != model.StatusOnline && status.Status != model.StatusDnd

	return userAllowsEmails && emailNotificationsAllowedForStatus && user.DeleteAt == 0 && !autoResponderRelated
}

func (a *App) sendNoUsersNotifiedByGroupInChannel(sender *model.User, post *model.Post, channel *model.Channel, group *model.Group) {
	T := i18n.GetUserTranslations(sender.Locale)
	ephemeralPost := &model.Post{
		UserID:    sender.ID,
		RootID:    post.RootID,
		ParentID:  post.ParentID,
		ChannelID: channel.ID,
		Message:   T("api.post.check_for_out_of_channel_group_users.message.none", model.StringInterface{"GroupName": group.Name}),
	}
	a.SendEphemeralPost(post.UserID, ephemeralPost)
}

// sendOutOfChannelMentions sends an ephemeral post to the sender of a post if any of the given potential mentions
// are outside of the post's channel. Returns whether or not an ephemeral post was sent.
func (a *App) sendOutOfChannelMentions(sender *model.User, post *model.Post, channel *model.Channel, potentialMentions []string) (bool, error) {
	outOfChannelUsers, outOfGroupsUsers, err := a.filterOutOfChannelMentions(sender, post, channel, potentialMentions)
	if err != nil {
		return false, err
	}

	if len(outOfChannelUsers) == 0 && len(outOfGroupsUsers) == 0 {
		return false, nil
	}

	a.SendEphemeralPost(post.UserID, makeOutOfChannelMentionPost(sender, post, outOfChannelUsers, outOfGroupsUsers))

	return true, nil
}

func (a *App) FilterUsersByVisible(viewer *model.User, otherUsers []*model.User) ([]*model.User, *model.AppError) {
	result := []*model.User{}
	for _, user := range otherUsers {
		canSee, err := a.UserCanSeeOtherUser(viewer.ID, user.ID)
		if err != nil {
			return nil, err
		}
		if canSee {
			result = append(result, user)
		}
	}
	return result, nil
}

func (a *App) filterOutOfChannelMentions(sender *model.User, post *model.Post, channel *model.Channel, potentialMentions []string) ([]*model.User, []*model.User, error) {
	if post.IsSystemMessage() {
		return nil, nil, nil
	}

	if channel.TeamID == "" || channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		return nil, nil, nil
	}

	if len(potentialMentions) == 0 {
		return nil, nil, nil
	}

	users, err := a.Srv().Store.User().GetProfilesByUsernames(potentialMentions, &model.ViewUsersRestrictions{Teams: []string{channel.TeamID}})
	if err != nil {
		return nil, nil, err
	}

	// Filter out inactive users and bots
	allUsers := model.UserSlice(users).FilterByActive(true)
	allUsers = allUsers.FilterWithoutBots()
	allUsers, appErr := a.FilterUsersByVisible(sender, allUsers)
	if appErr != nil {
		return nil, nil, appErr
	}

	if len(allUsers) == 0 {
		return nil, nil, nil
	}

	// Differentiate between users who can and can't be added to the channel
	var outOfChannelUsers model.UserSlice
	var outOfGroupsUsers model.UserSlice
	if channel.IsGroupConstrained() {
		nonMemberIDs, err := a.FilterNonGroupChannelMembers(allUsers.IDs(), channel)
		if err != nil {
			return nil, nil, err
		}

		outOfChannelUsers = allUsers.FilterWithoutID(nonMemberIDs)
		outOfGroupsUsers = allUsers.FilterByID(nonMemberIDs)
	} else {
		outOfChannelUsers = allUsers
	}

	return outOfChannelUsers, outOfGroupsUsers, nil
}

func makeOutOfChannelMentionPost(sender *model.User, post *model.Post, outOfChannelUsers, outOfGroupsUsers []*model.User) *model.Post {
	allUsers := model.UserSlice(append(outOfChannelUsers, outOfGroupsUsers...))

	ocUsers := model.UserSlice(outOfChannelUsers)
	ocUsernames := ocUsers.Usernames()
	ocUserIDs := ocUsers.IDs()

	ogUsers := model.UserSlice(outOfGroupsUsers)
	ogUsernames := ogUsers.Usernames()

	T := i18n.GetUserTranslations(sender.Locale)

	ephemeralPostID := model.NewID()
	var message string
	if len(outOfChannelUsers) == 1 {
		message = T("api.post.check_for_out_of_channel_mentions.message.one", map[string]interface{}{
			"Username": ocUsernames[0],
		})
	} else if len(outOfChannelUsers) > 1 {
		preliminary, final := splitAtFinal(ocUsernames)

		message = T("api.post.check_for_out_of_channel_mentions.message.multiple", map[string]interface{}{
			"Usernames":    strings.Join(preliminary, ", @"),
			"LastUsername": final,
		})
	}

	if len(outOfGroupsUsers) == 1 {
		if message != "" {
			message += "\n"
		}

		message += T("api.post.check_for_out_of_channel_groups_mentions.message.one", map[string]interface{}{
			"Username": ogUsernames[0],
		})
	} else if len(outOfGroupsUsers) > 1 {
		preliminary, final := splitAtFinal(ogUsernames)

		if message != "" {
			message += "\n"
		}

		message += T("api.post.check_for_out_of_channel_groups_mentions.message.multiple", map[string]interface{}{
			"Usernames":    strings.Join(preliminary, ", @"),
			"LastUsername": final,
		})
	}

	props := model.StringInterface{
		model.PropsAddChannelMember: model.StringInterface{
			"post_id": ephemeralPostID,

			"usernames":                allUsers.Usernames(), // Kept for backwards compatibility of mobile app.
			"not_in_channel_usernames": ocUsernames,

			"user_ids":                allUsers.IDs(), // Kept for backwards compatibility of mobile app.
			"not_in_channel_user_ids": ocUserIDs,

			"not_in_groups_usernames": ogUsernames,
			"not_in_groups_user_ids":  ogUsers.IDs(),
		},
	}

	return &model.Post{
		ID:        ephemeralPostID,
		RootID:    post.RootID,
		ChannelID: post.ChannelID,
		Message:   message,
		CreateAt:  post.CreateAt + 1,
		Props:     props,
	}
}

func splitAtFinal(items []string) (preliminary []string, final string) {
	if len(items) == 0 {
		return
	}
	preliminary = items[:len(items)-1]
	final = items[len(items)-1]
	return
}

type ExplicitMentions struct {
	// Mentions contains the ID of each user that was mentioned and how they were mentioned.
	Mentions map[string]MentionType

	// Contains a map of groups that were mentioned
	GroupMentions map[string]*model.Group

	// OtherPotentialMentions contains a list of strings that looked like mentions, but didn't have
	// a corresponding keyword.
	OtherPotentialMentions []string

	// HereMentioned is true if the message contained @here.
	HereMentioned bool

	// AllMentioned is true if the message contained @all.
	AllMentioned bool

	// ChannelMentioned is true if the message contained @channel.
	ChannelMentioned bool
}

type MentionType int

const (
	// Different types of mentions ordered by their priority from lowest to highest

	// A placeholder that should never be used in practice
	NoMention MentionType = iota

	// The post is in a thread that the user has commented on
	ThreadMention

	// The post is a comment on a thread started by the user
	CommentMention

	// The post contains an at-channel, at-all, or at-here
	ChannelMention

	// The post is a DM
	DMMention

	// The post contains an at-mention for the user
	KeywordMention

	// The post contains a group mention for the user
	GroupMention
)

func (m *ExplicitMentions) addMention(userID string, mentionType MentionType) {
	if m.Mentions == nil {
		m.Mentions = make(map[string]MentionType)
	}

	if currentType, ok := m.Mentions[userID]; ok && currentType >= mentionType {
		return
	}

	m.Mentions[userID] = mentionType
}

func (m *ExplicitMentions) addGroupMention(word string, groups map[string]*model.Group) bool {
	if strings.HasPrefix(word, "@") {
		word = word[1:]
	} else {
		// Only allow group mentions when mentioned directly with @group-name
		return false
	}

	group, groupFound := groups[word]
	if !groupFound {
		group = groups[strings.ToLower(word)]
	}

	if group == nil {
		return false
	}

	if m.GroupMentions == nil {
		m.GroupMentions = make(map[string]*model.Group)
	}

	if group.Name != nil {
		m.GroupMentions[*group.Name] = group
	}

	return true
}

func (m *ExplicitMentions) addMentions(userIDs []string, mentionType MentionType) {
	for _, userID := range userIDs {
		m.addMention(userID, mentionType)
	}
}

func (m *ExplicitMentions) removeMention(userID string) {
	delete(m.Mentions, userID)
}

// Given a message and a map mapping mention keywords to the users who use them, returns a map of mentioned
// users and a slice of potential mention users not in the channel and whether or not @here was mentioned.
func getExplicitMentions(post *model.Post, keywords map[string][]string, groups map[string]*model.Group) *ExplicitMentions {
	ret := &ExplicitMentions{}

	buf := ""
	mentionsEnabledFields := getMentionsEnabledFields(post)
	for _, message := range mentionsEnabledFields {
		markdown.Inspect(message, func(node interface{}) bool {
			text, ok := node.(*markdown.Text)
			if !ok {
				ret.processText(buf, keywords, groups)
				buf = ""
				return true
			}
			buf += text.Text
			return false
		})
	}
	ret.processText(buf, keywords, groups)

	return ret
}

// Given a post returns the values of the fields in which mentions are possible.
// post.message, preText and text in the attachment are enabled.
func getMentionsEnabledFields(post *model.Post) model.StringArray {
	ret := []string{}

	ret = append(ret, post.Message)
	for _, attachment := range post.Attachments() {

		if attachment.Pretext != "" {
			ret = append(ret, attachment.Pretext)
		}
		if attachment.Text != "" {
			ret = append(ret, attachment.Text)
		}
	}
	return ret
}

// allowChannelMentions returns whether or not the channel mentions are allowed for the given post.
func (a *App) allowChannelMentions(post *model.Post, numProfiles int) bool {
	if !a.HasPermissionToChannel(post.UserID, post.ChannelID, model.PermissionUseChannelMentions) {
		return false
	}

	if post.Type == model.PostTypeHeaderChange || post.Type == model.PostTypePurposeChange {
		return false
	}

	if int64(numProfiles) >= *a.Config().TeamSettings.MaxNotificationsPerChannel {
		return false
	}

	return true
}

// allowGroupMentions returns whether or not the group mentions are allowed for the given post.
func (a *App) allowGroupMentions(post *model.Post) bool {
	if license := a.Srv().License(); license == nil || !*license.Features.LDAPGroups {
		return false
	}

	if !a.HasPermissionToChannel(post.UserID, post.ChannelID, model.PermissionUseGroupMentions) {
		return false
	}

	if post.Type == model.PostTypeHeaderChange || post.Type == model.PostTypePurposeChange {
		return false
	}

	return true
}

// getGroupsAllowedForReferenceInChannel returns a map of groups allowed for reference in a given channel and team.
func (a *App) getGroupsAllowedForReferenceInChannel(channel *model.Channel, team *model.Team) (map[string]*model.Group, error) {
	var err error
	groupsMap := make(map[string]*model.Group)
	opts := model.GroupSearchOpts{FilterAllowReference: true}

	if channel.IsGroupConstrained() || (team != nil && team.IsGroupConstrained()) {
		var groups []*model.GroupWithSchemeAdmin
		if channel.IsGroupConstrained() {
			groups, err = a.Srv().Store.Group().GetGroupsByChannel(channel.ID, opts)
		} else {
			groups, err = a.Srv().Store.Group().GetGroupsByTeam(team.ID, opts)
		}
		if err != nil {
			return nil, errors.Wrap(err, "unable to get groups")
		}
		for _, group := range groups {
			if group.Group.Name != nil {
				groupsMap[*group.Group.Name] = &group.Group
			}
		}
		return groupsMap, nil
	}

	groups, err := a.Srv().Store.Group().GetGroups(0, 0, opts)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get groups")
	}
	for _, group := range groups {
		if group.Name != nil {
			groupsMap[*group.Name] = group
		}
	}

	return groupsMap, nil
}

// Given a map of user IDs to profiles, returns a list of mention
// keywords for all users in the channel.
func (a *App) getMentionKeywordsInChannel(profiles map[string]*model.User, allowChannelMentions bool, channelMemberNotifyPropsMap map[string]model.StringMap) map[string][]string {
	keywords := make(map[string][]string)

	for _, profile := range profiles {
		addMentionKeywordsForUser(
			keywords,
			profile,
			channelMemberNotifyPropsMap[profile.ID],
			a.GetStatusFromCache(profile.ID),
			allowChannelMentions,
		)
	}

	return keywords
}

// insertGroupMentions adds group members in the channel to Mentions, adds group members not in the channel to OtherPotentialMentions
// returns false if no group members present in the team that the channel belongs to
func (a *App) insertGroupMentions(group *model.Group, channel *model.Channel, profileMap map[string]*model.User, mentions *ExplicitMentions) (bool, *model.AppError) {
	var err error
	var groupMembers []*model.User
	outOfChannelGroupMembers := []*model.User{}
	isGroupOrDirect := channel.IsGroupOrDirect()

	if isGroupOrDirect {
		groupMembers, err = a.Srv().Store.Group().GetMemberUsers(group.ID)
	} else {
		groupMembers, err = a.Srv().Store.Group().GetMemberUsersInTeam(group.ID, channel.TeamID)
	}

	if err != nil {
		return false, model.NewAppError("insertGroupMentions", "app.select_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if mentions.Mentions == nil {
		mentions.Mentions = make(map[string]MentionType)
	}

	for _, member := range groupMembers {
		if _, ok := profileMap[member.ID]; ok {
			mentions.Mentions[member.ID] = GroupMention
		} else {
			outOfChannelGroupMembers = append(outOfChannelGroupMembers, member)
		}
	}

	potentialGroupMembersMentioned := []string{}
	for _, user := range outOfChannelGroupMembers {
		potentialGroupMembersMentioned = append(potentialGroupMembersMentioned, user.Username)
	}
	if mentions.OtherPotentialMentions == nil {
		mentions.OtherPotentialMentions = potentialGroupMembersMentioned
	} else {
		mentions.OtherPotentialMentions = append(mentions.OtherPotentialMentions, potentialGroupMembersMentioned...)
	}

	return isGroupOrDirect || len(groupMembers) > 0, nil
}

// addMentionKeywordsForUser adds the mention keywords for a given user to the given keyword map. Returns the provided keyword map.
func addMentionKeywordsForUser(keywords map[string][]string, profile *model.User, channelNotifyProps map[string]string, status *model.Status, allowChannelMentions bool) map[string][]string {
	userMention := "@" + strings.ToLower(profile.Username)
	keywords[userMention] = append(keywords[userMention], profile.ID)

	// Add all the user's mention keys
	for _, k := range profile.GetMentionKeys() {
		// note that these are made lower case so that we can do a case insensitive check for them
		key := strings.ToLower(k)

		if key != "" {
			keywords[key] = append(keywords[key], profile.ID)
		}
	}

	// If turned on, add the user's case sensitive first name
	if profile.NotifyProps[model.FirstNameNotifyProp] == "true" && profile.FirstName != "" {
		keywords[profile.FirstName] = append(keywords[profile.FirstName], profile.ID)
	}

	// Add @channel and @all to keywords if user has them turned on and the server allows them
	if allowChannelMentions {
		// Ignore channel mentions if channel is muted and channel mention setting is default
		ignoreChannelMentions := channelNotifyProps[model.IgnoreChannelMentionsNotifyProp] == model.IgnoreChannelMentionsOn || (channelNotifyProps[model.MarkUnreadNotifyProp] == model.UserNotifyMention && channelNotifyProps[model.IgnoreChannelMentionsNotifyProp] == model.IgnoreChannelMentionsDefault)

		if profile.NotifyProps[model.ChannelMentionsNotifyProp] == "true" && !ignoreChannelMentions {
			keywords["@channel"] = append(keywords["@channel"], profile.ID)
			keywords["@all"] = append(keywords["@all"], profile.ID)

			if status != nil && status.Status == model.StatusOnline {
				keywords["@here"] = append(keywords["@here"], profile.ID)
			}
		}
	}

	return keywords
}

// Represents either an email or push notification and contains the fields required to send it to any user.
type PostNotification struct {
	Channel    *model.Channel
	Post       *model.Post
	ProfileMap map[string]*model.User
	Sender     *model.User
}

// Returns the name of the channel for this notification. For direct messages, this is the sender's name
// preceded by an at sign. For group messages, this is a comma-separated list of the members of the
// channel, with an option to exclude the recipient of the message from that list.
func (n *PostNotification) GetChannelName(userNameFormat, excludeID string) string {
	switch n.Channel.Type {
	case model.ChannelTypeDirect:
		return n.Sender.GetDisplayNameWithPrefix(userNameFormat, "@")
	case model.ChannelTypeGroup:
		names := []string{}
		for _, user := range n.ProfileMap {
			if user.ID != excludeID {
				names = append(names, user.GetDisplayName(userNameFormat))
			}
		}

		sort.Strings(names)

		return strings.Join(names, ", ")
	default:
		return n.Channel.DisplayName
	}
}

// Returns the name of the sender of this notification, accounting for things like system messages
// and whether or not the username has been overridden by an integration.
func (n *PostNotification) GetSenderName(userNameFormat string, overridesAllowed bool) string {
	if n.Post.IsSystemMessage() {
		return i18n.T("system.message.name")
	}

	if overridesAllowed && n.Channel.Type != model.ChannelTypeDirect {
		if value, ok := n.Post.GetProps()["override_username"]; ok && n.Post.GetProp("from_webhook") == "true" {
			return value.(string)
		}
	}

	return n.Sender.GetDisplayNameWithPrefix(userNameFormat, "@")
}

// checkForMention checks if there is a mention to a specific user or to the keywords here / channel / all
func (m *ExplicitMentions) checkForMention(word string, keywords map[string][]string, groups map[string]*model.Group) bool {
	var mentionType MentionType

	switch strings.ToLower(word) {
	case "@here":
		m.HereMentioned = true
		mentionType = ChannelMention
	case "@channel":
		m.ChannelMentioned = true
		mentionType = ChannelMention
	case "@all":
		m.AllMentioned = true
		mentionType = ChannelMention
	default:
		mentionType = KeywordMention
	}

	m.addGroupMention(word, groups)

	if ids, match := keywords[strings.ToLower(word)]; match {
		m.addMentions(ids, mentionType)
		return true
	}

	// Case-sensitive check for first name
	if ids, match := keywords[word]; match {
		m.addMentions(ids, mentionType)
		return true
	}

	return false
}

// isKeywordMultibyte checks if a word containing a multibyte character contains a multibyte keyword
func isKeywordMultibyte(keywords map[string][]string, word string) ([]string, bool) {
	ids := []string{}
	match := false
	var multibyteKeywords []string
	for keyword := range keywords {
		if len(keyword) != utf8.RuneCountInString(keyword) {
			multibyteKeywords = append(multibyteKeywords, keyword)
		}
	}

	if len(word) != utf8.RuneCountInString(word) {
		for _, key := range multibyteKeywords {
			if strings.Contains(word, key) {
				ids, match = keywords[key]
			}
		}
	}
	return ids, match
}

// Processes text to filter mentioned users and other potential mentions
func (m *ExplicitMentions) processText(text string, keywords map[string][]string, groups map[string]*model.Group) {
	systemMentions := map[string]bool{"@here": true, "@channel": true, "@all": true}

	for _, word := range strings.FieldsFunc(text, func(c rune) bool {
		// Split on any whitespace or punctuation that can't be part of an at mention or emoji pattern
		return !(c == ':' || c == '.' || c == '-' || c == '_' || c == '@' || unicode.IsLetter(c) || unicode.IsNumber(c))
	}) {
		// skip word with format ':word:' with an assumption that it is an emoji format only
		if word[0] == ':' && word[len(word)-1] == ':' {
			continue
		}

		word = strings.TrimLeft(word, ":.-_")

		if m.checkForMention(word, keywords, groups) {
			continue
		}

		foundWithoutSuffix := false
		wordWithoutSuffix := word

		for wordWithoutSuffix != "" && strings.LastIndexAny(wordWithoutSuffix, ".-:_") == (len(wordWithoutSuffix)-1) {
			wordWithoutSuffix = wordWithoutSuffix[0 : len(wordWithoutSuffix)-1]

			if m.checkForMention(wordWithoutSuffix, keywords, groups) {
				foundWithoutSuffix = true
				break
			}
		}

		if foundWithoutSuffix {
			continue
		}

		if _, ok := systemMentions[word]; !ok && strings.HasPrefix(word, "@") {
			// No need to bother about unicode as we are looking for ASCII characters.
			last := word[len(word)-1]
			switch last {
			// If the word is possibly at the end of a sentence, remove that character.
			case '.', '-', ':':
				word = word[:len(word)-1]
			}
			m.OtherPotentialMentions = append(m.OtherPotentialMentions, word[1:])
		} else if strings.ContainsAny(word, ".-:") {
			// This word contains a character that may be the end of a sentence, so split further
			splitWords := strings.FieldsFunc(word, func(c rune) bool {
				return c == '.' || c == '-' || c == ':'
			})

			for _, splitWord := range splitWords {
				if m.checkForMention(splitWord, keywords, groups) {
					continue
				}
				if _, ok := systemMentions[splitWord]; !ok && strings.HasPrefix(splitWord, "@") {
					m.OtherPotentialMentions = append(m.OtherPotentialMentions, splitWord[1:])
				}
			}
		}

		if ids, match := isKeywordMultibyte(keywords, word); match {
			m.addMentions(ids, KeywordMention)
		}
	}
}

func (a *App) GetNotificationNameFormat(user *model.User) string {
	if !*a.Config().PrivacySettings.ShowFullName {
		return model.ShowUsername
	}

	data, err := a.Srv().Store.Preference().Get(user.ID, model.PreferenceCategoryDisplaySettings, model.PreferenceNameNameFormat)
	if err != nil {
		return *a.Config().TeamSettings.TeammateNameDisplay
	}

	return data.Value
}
