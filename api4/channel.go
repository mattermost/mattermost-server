// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func (api *API) InitChannel() {
	api.BaseRoutes.Channels.Handle("", api.ApiSessionRequired(getAllChannels)).Methods("GET")
	api.BaseRoutes.Channels.Handle("", api.ApiSessionRequired(createChannel)).Methods("POST")
	api.BaseRoutes.Channels.Handle("/direct", api.ApiSessionRequired(createDirectChannel)).Methods("POST")
	api.BaseRoutes.Channels.Handle("/search", api.ApiSessionRequiredDisableWhenBusy(searchAllChannels)).Methods("POST")
	api.BaseRoutes.Channels.Handle("/group/search", api.ApiSessionRequiredDisableWhenBusy(searchGroupChannels)).Methods("POST")
	api.BaseRoutes.Channels.Handle("/group", api.ApiSessionRequired(createGroupChannel)).Methods("POST")
	api.BaseRoutes.Channels.Handle("/members/{user_id:[A-Za-z0-9]+}/view", api.ApiSessionRequired(viewChannel)).Methods("POST")
	api.BaseRoutes.Channels.Handle("/{channel_id:[A-Za-z0-9]+}/scheme", api.ApiSessionRequired(updateChannelScheme)).Methods("PUT")

	api.BaseRoutes.ChannelsForTeam.Handle("", api.ApiSessionRequired(getPublicChannelsForTeam)).Methods("GET")
	api.BaseRoutes.ChannelsForTeam.Handle("/deleted", api.ApiSessionRequired(getDeletedChannelsForTeam)).Methods("GET")
	api.BaseRoutes.ChannelsForTeam.Handle("/private", api.ApiSessionRequired(getPrivateChannelsForTeam)).Methods("GET")
	api.BaseRoutes.ChannelsForTeam.Handle("/ids", api.ApiSessionRequired(getPublicChannelsByIDsForTeam)).Methods("POST")
	api.BaseRoutes.ChannelsForTeam.Handle("/search", api.ApiSessionRequiredDisableWhenBusy(searchChannelsForTeam)).Methods("POST")
	api.BaseRoutes.ChannelsForTeam.Handle("/search_archived", api.ApiSessionRequiredDisableWhenBusy(searchArchivedChannelsForTeam)).Methods("POST")
	api.BaseRoutes.ChannelsForTeam.Handle("/autocomplete", api.ApiSessionRequired(autocompleteChannelsForTeam)).Methods("GET")
	api.BaseRoutes.ChannelsForTeam.Handle("/search_autocomplete", api.ApiSessionRequired(autocompleteChannelsForTeamForSearch)).Methods("GET")
	api.BaseRoutes.User.Handle("/teams/{team_id:[A-Za-z0-9]+}/channels", api.ApiSessionRequired(getChannelsForTeamForUser)).Methods("GET")

	api.BaseRoutes.ChannelCategories.Handle("", api.ApiSessionRequired(getCategoriesForTeamForUser)).Methods("GET")
	api.BaseRoutes.ChannelCategories.Handle("", api.ApiSessionRequired(createCategoryForTeamForUser)).Methods("POST")
	api.BaseRoutes.ChannelCategories.Handle("", api.ApiSessionRequired(updateCategoriesForTeamForUser)).Methods("PUT")
	api.BaseRoutes.ChannelCategories.Handle("/order", api.ApiSessionRequired(getCategoryOrderForTeamForUser)).Methods("GET")
	api.BaseRoutes.ChannelCategories.Handle("/order", api.ApiSessionRequired(updateCategoryOrderForTeamForUser)).Methods("PUT")
	api.BaseRoutes.ChannelCategories.Handle("/{category_id:[A-Za-z0-9_-]+}", api.ApiSessionRequired(getCategoryForTeamForUser)).Methods("GET")
	api.BaseRoutes.ChannelCategories.Handle("/{category_id:[A-Za-z0-9_-]+}", api.ApiSessionRequired(updateCategoryForTeamForUser)).Methods("PUT")
	api.BaseRoutes.ChannelCategories.Handle("/{category_id:[A-Za-z0-9_-]+}", api.ApiSessionRequired(deleteCategoryForTeamForUser)).Methods("DELETE")

	api.BaseRoutes.Channel.Handle("", api.ApiSessionRequired(getChannel)).Methods("GET")
	api.BaseRoutes.Channel.Handle("", api.ApiSessionRequired(updateChannel)).Methods("PUT")
	api.BaseRoutes.Channel.Handle("/patch", api.ApiSessionRequired(patchChannel)).Methods("PUT")
	api.BaseRoutes.Channel.Handle("/convert", api.ApiSessionRequired(convertChannelToPrivate)).Methods("POST")
	api.BaseRoutes.Channel.Handle("/privacy", api.ApiSessionRequired(updateChannelPrivacy)).Methods("PUT")
	api.BaseRoutes.Channel.Handle("/restore", api.ApiSessionRequired(restoreChannel)).Methods("POST")
	api.BaseRoutes.Channel.Handle("", api.ApiSessionRequired(deleteChannel)).Methods("DELETE")
	api.BaseRoutes.Channel.Handle("/stats", api.ApiSessionRequired(getChannelStats)).Methods("GET")
	api.BaseRoutes.Channel.Handle("/pinned", api.ApiSessionRequired(getPinnedPosts)).Methods("GET")
	api.BaseRoutes.Channel.Handle("/timezones", api.ApiSessionRequired(getChannelMembersTimezones)).Methods("GET")
	api.BaseRoutes.Channel.Handle("/members_minus_group_members", api.ApiSessionRequired(channelMembersMinusGroupMembers)).Methods("GET")
	api.BaseRoutes.Channel.Handle("/move", api.ApiSessionRequired(moveChannel)).Methods("POST")
	api.BaseRoutes.Channel.Handle("/member_counts_by_group", api.ApiSessionRequired(channelMemberCountsByGroup)).Methods("GET")

	api.BaseRoutes.ChannelForUser.Handle("/unread", api.ApiSessionRequired(getChannelUnread)).Methods("GET")

	api.BaseRoutes.ChannelByName.Handle("", api.ApiSessionRequired(getChannelByName)).Methods("GET")
	api.BaseRoutes.ChannelByNameForTeamName.Handle("", api.ApiSessionRequired(getChannelByNameForTeamName)).Methods("GET")

	api.BaseRoutes.ChannelMembers.Handle("", api.ApiSessionRequired(getChannelMembers)).Methods("GET")
	api.BaseRoutes.ChannelMembers.Handle("/ids", api.ApiSessionRequired(getChannelMembersByIDs)).Methods("POST")
	api.BaseRoutes.ChannelMembers.Handle("", api.ApiSessionRequired(addChannelMember)).Methods("POST")
	api.BaseRoutes.ChannelMembersForUser.Handle("", api.ApiSessionRequired(getChannelMembersForUser)).Methods("GET")
	api.BaseRoutes.ChannelMember.Handle("", api.ApiSessionRequired(getChannelMember)).Methods("GET")
	api.BaseRoutes.ChannelMember.Handle("", api.ApiSessionRequired(removeChannelMember)).Methods("DELETE")
	api.BaseRoutes.ChannelMember.Handle("/roles", api.ApiSessionRequired(updateChannelMemberRoles)).Methods("PUT")
	api.BaseRoutes.ChannelMember.Handle("/schemeRoles", api.ApiSessionRequired(updateChannelMemberSchemeRoles)).Methods("PUT")
	api.BaseRoutes.ChannelMember.Handle("/notify_props", api.ApiSessionRequired(updateChannelMemberNotifyProps)).Methods("PUT")

	api.BaseRoutes.ChannelModerations.Handle("", api.ApiSessionRequired(getChannelModerations)).Methods("GET")
	api.BaseRoutes.ChannelModerations.Handle("/patch", api.ApiSessionRequired(patchChannelModerations)).Methods("PUT")
}

func createChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	channel := model.ChannelFromJSON(r.Body)
	if channel == nil {
		c.SetInvalidParam("channel")
		return
	}

	auditRec := c.MakeAuditRecord("createChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)

	if channel.Type == model.ChannelTypeOpen && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionCreatePublicChannel) {
		c.SetPermissionError(model.PermissionCreatePublicChannel)
		return
	}

	if channel.Type == model.ChannelTypePrivate && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionCreatePrivateChannel) {
		c.SetPermissionError(model.PermissionCreatePrivateChannel)
		return
	}

	sc, err := c.App.CreateChannelWithUser(c.AppContext, channel, c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("channel", sc) // overwrite meta
	c.LogAudit("name=" + channel.Name)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(sc.ToJSON()))
}

func updateChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	channel := model.ChannelFromJSON(r.Body)

	if channel == nil {
		c.SetInvalidParam("channel")
		return
	}

	// The channel being updated in the payload must be the same one as indicated in the URL.
	if channel.ID != c.Params.ChannelID {
		c.SetInvalidParam("channel_id")
		return
	}

	auditRec := c.MakeAuditRecord("updateChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)

	originalOldChannel, err := c.App.GetChannel(channel.ID)
	if err != nil {
		c.Err = err
		return
	}
	oldChannel := originalOldChannel.DeepCopy()

	auditRec.AddMeta("channel", oldChannel)

	switch oldChannel.Type {
	case model.ChannelTypeOpen:
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionManagePublicChannelProperties) {
			c.SetPermissionError(model.PermissionManagePublicChannelProperties)
			return
		}

	case model.ChannelTypePrivate:
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionManagePrivateChannelProperties) {
			c.SetPermissionError(model.PermissionManagePrivateChannelProperties)
			return
		}

	case model.ChannelTypeGroup, model.ChannelTypeDirect:
		// Modifying the header is not linked to any specific permission for group/dm channels, so just check for membership.
		if _, errGet := c.App.GetChannelMember(context.Background(), channel.ID, c.AppContext.Session().UserID); errGet != nil {
			c.Err = model.NewAppError("updateChannel", "api.channel.patch_update_channel.forbidden.app_error", nil, "", http.StatusForbidden)
			return
		}

	default:
		c.Err = model.NewAppError("updateChannel", "api.channel.patch_update_channel.forbidden.app_error", nil, "", http.StatusForbidden)
		return
	}

	if oldChannel.DeleteAt > 0 {
		c.Err = model.NewAppError("updateChannel", "api.channel.update_channel.deleted.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if channel.Type != "" && channel.Type != oldChannel.Type {
		c.Err = model.NewAppError("updateChannel", "api.channel.update_channel.typechange.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if oldChannel.Name == model.DefaultChannelName {
		if channel.Name != "" && channel.Name != oldChannel.Name {
			c.Err = model.NewAppError("updateChannel", "api.channel.update_channel.tried.app_error", map[string]interface{}{"Channel": model.DefaultChannelName}, "", http.StatusBadRequest)
			return
		}
	}

	oldChannel.Header = channel.Header
	oldChannel.Purpose = channel.Purpose

	oldChannelDisplayName := oldChannel.DisplayName

	if channel.DisplayName != "" {
		oldChannel.DisplayName = channel.DisplayName
	}

	if channel.Name != "" {
		oldChannel.Name = channel.Name
		auditRec.AddMeta("new_channel_name", oldChannel.Name)
	}

	if channel.GroupConstrained != nil {
		oldChannel.GroupConstrained = channel.GroupConstrained
	}

	updatedChannel, err := c.App.UpdateChannel(oldChannel)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("update", updatedChannel)

	if oldChannelDisplayName != channel.DisplayName {
		if err := c.App.PostUpdateChannelDisplayNameMessage(c.AppContext, c.AppContext.Session().UserID, channel, oldChannelDisplayName, channel.DisplayName); err != nil {
			mlog.Warn("Error while posting channel display name message", mlog.Err(err))
		}
	}

	auditRec.Success()
	c.LogAudit("name=" + channel.Name)

	w.Write([]byte(oldChannel.ToJSON()))
}

func convertChannelToPrivate(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	oldPublicChannel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("convertChannelToPrivate", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", oldPublicChannel)

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionConvertPublicChannelToPrivate) {
		c.SetPermissionError(model.PermissionConvertPublicChannelToPrivate)
		return
	}

	if oldPublicChannel.Type == model.ChannelTypePrivate {
		c.Err = model.NewAppError("convertChannelToPrivate", "api.channel.convert_channel_to_private.private_channel_error", nil, "", http.StatusBadRequest)
		return
	}

	if oldPublicChannel.Name == model.DefaultChannelName {
		c.Err = model.NewAppError("convertChannelToPrivate", "api.channel.convert_channel_to_private.default_channel_error", nil, "", http.StatusBadRequest)
		return
	}

	user, err := c.App.GetUser(c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("user", user)

	oldPublicChannel.Type = model.ChannelTypePrivate

	rchannel, err := c.App.UpdateChannelPrivacy(c.AppContext, oldPublicChannel, user)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + rchannel.Name)

	w.Write([]byte(rchannel.ToJSON()))
}

func updateChannelPrivacy(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	props := model.StringInterfaceFromJSON(r.Body)
	privacy, ok := props["privacy"].(string)
	if !ok || (privacy != model.ChannelTypeOpen && privacy != model.ChannelTypePrivate) {
		c.SetInvalidParam("privacy")
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("updateChannelPrivacy", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("new_type", privacy)

	if privacy == model.ChannelTypeOpen && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionConvertPrivateChannelToPublic) {
		c.SetPermissionError(model.PermissionConvertPrivateChannelToPublic)
		return
	}

	if privacy == model.ChannelTypePrivate && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionConvertPublicChannelToPrivate) {
		c.SetPermissionError(model.PermissionConvertPublicChannelToPrivate)
		return
	}

	if channel.Name == model.DefaultChannelName && privacy == model.ChannelTypePrivate {
		c.Err = model.NewAppError("updateChannelPrivacy", "api.channel.update_channel_privacy.default_channel_error", nil, "", http.StatusBadRequest)
		return
	}

	user, err := c.App.GetUser(c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("user", user)

	channel.Type = privacy

	updatedChannel, err := c.App.UpdateChannelPrivacy(c.AppContext, channel, user)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + updatedChannel.Name)

	w.Write([]byte(updatedChannel.ToJSON()))
}

func patchChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}
	patch := model.ChannelPatchFromJSON(r.Body)
	if patch == nil {
		c.SetInvalidParam("channel")
		return
	}

	originalOldChannel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}
	oldChannel := originalOldChannel.DeepCopy()

	auditRec := c.MakeAuditRecord("patchChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", oldChannel)

	switch oldChannel.Type {
	case model.ChannelTypeOpen:
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionManagePublicChannelProperties) {
			c.SetPermissionError(model.PermissionManagePublicChannelProperties)
			return
		}

	case model.ChannelTypePrivate:
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionManagePrivateChannelProperties) {
			c.SetPermissionError(model.PermissionManagePrivateChannelProperties)
			return
		}

	case model.ChannelTypeGroup, model.ChannelTypeDirect:
		// Modifying the header is not linked to any specific permission for group/dm channels, so just check for membership.
		if _, err = c.App.GetChannelMember(context.Background(), c.Params.ChannelID, c.AppContext.Session().UserID); err != nil {
			c.Err = model.NewAppError("patchChannel", "api.channel.patch_update_channel.forbidden.app_error", nil, "", http.StatusForbidden)
			return
		}

	default:
		c.Err = model.NewAppError("patchChannel", "api.channel.patch_update_channel.forbidden.app_error", nil, "", http.StatusForbidden)
		return
	}

	rchannel, err := c.App.PatchChannel(c.AppContext, oldChannel, patch, c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	err = c.App.FillInChannelProps(rchannel)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("")
	auditRec.AddMeta("patch", rchannel)

	w.Write([]byte(rchannel.ToJSON()))
}

func restoreChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}
	teamID := channel.TeamID

	auditRec := c.MakeAuditRecord("restoreChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), teamID, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	channel, err = c.App.RestoreChannel(c.AppContext, channel, c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + channel.Name)

	w.Write([]byte(channel.ToJSON()))
}

func createDirectChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	userIDs := model.ArrayFromJSON(r.Body)
	allowed := false

	if len(userIDs) != 2 {
		c.SetInvalidParam("user_ids")
		return
	}

	for _, id := range userIDs {
		if !model.IsValidID(id) {
			c.SetInvalidParam("user_id")
			return
		}
		if id == c.AppContext.Session().UserID {
			allowed = true
		}
	}

	auditRec := c.MakeAuditRecord("createDirectChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionCreateDirectChannel) {
		c.SetPermissionError(model.PermissionCreateDirectChannel)
		return
	}

	if !allowed && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	otherUserID := userIDs[0]
	if c.AppContext.Session().UserID == otherUserID {
		otherUserID = userIDs[1]
	}

	auditRec.AddMeta("other_user_id", otherUserID)

	canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserID, otherUserID)
	if err != nil {
		c.Err = err
		return
	}

	if !canSee {
		c.SetPermissionError(model.PermissionViewMembers)
		return
	}

	sc, err := c.App.GetOrCreateDirectChannel(c.AppContext, userIDs[0], userIDs[1])
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("channel", sc)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(sc.ToJSON()))
}

func searchGroupChannels(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.ChannelSearchFromJSON(r.Body)
	if props == nil {
		c.SetInvalidParam("channel_search")
		return
	}

	groupChannels, err := c.App.SearchGroupChannels(c.AppContext.Session().UserID, props.Term)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(groupChannels.ToJSON()))
}

func createGroupChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	userIDs := model.ArrayFromJSON(r.Body)

	if len(userIDs) == 0 {
		c.SetInvalidParam("user_ids")
		return
	}

	found := false
	for _, id := range userIDs {
		if !model.IsValidID(id) {
			c.SetInvalidParam("user_id")
			return
		}
		if id == c.AppContext.Session().UserID {
			found = true
		}
	}

	if !found {
		userIDs = append(userIDs, c.AppContext.Session().UserID)
	}

	auditRec := c.MakeAuditRecord("createGroupChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionCreateGroupChannel) {
		c.SetPermissionError(model.PermissionCreateGroupChannel)
		return
	}

	canSeeAll := true
	for _, id := range userIDs {
		if c.AppContext.Session().UserID != id {
			canSee, err := c.App.UserCanSeeOtherUser(c.AppContext.Session().UserID, id)
			if err != nil {
				c.Err = err
				return
			}
			if !canSee {
				canSeeAll = false
			}
		}
	}

	if !canSeeAll {
		c.SetPermissionError(model.PermissionViewMembers)
		return
	}

	groupChannel, err := c.App.CreateGroupChannel(userIDs, c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("channel", groupChannel)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(groupChannel.ToJSON()))
}

func getChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	if channel.Type == model.ChannelTypeOpen {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionReadPublicChannel) && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
			c.SetPermissionError(model.PermissionReadPublicChannel)
			return
		}
	} else {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
			c.SetPermissionError(model.PermissionReadChannel)
			return
		}
	}

	err = c.App.FillInChannelProps(channel)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(channel.ToJSON()))
}

func getChannelUnread(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID().RequireUserID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	channelUnread, err := c.App.GetChannelUnread(c.Params.ChannelID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(channelUnread.ToJSON()))
}

func getChannelStats(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	memberCount, err := c.App.GetChannelMemberCount(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	guestCount, err := c.App.GetChannelGuestCount(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	pinnedPostCount, err := c.App.GetChannelPinnedPostCount(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	stats := model.ChannelStats{ChannelID: c.Params.ChannelID, MemberCount: memberCount, GuestCount: guestCount, PinnedPostCount: pinnedPostCount}
	w.Write([]byte(stats.ToJSON()))
}

func getPinnedPosts(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	posts, err := c.App.GetPinnedPosts(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	if c.HandleEtag(posts.Etag(), "Get Pinned Posts", w, r) {
		return
	}

	clientPostList := c.App.PreparePostListForClient(posts)

	w.Header().Set(model.HeaderEtagServer, clientPostList.Etag())
	w.Write([]byte(clientPostList.ToJSON()))
}

func getAllChannels(c *Context, w http.ResponseWriter, r *http.Request) {
	permissions := []*model.Permission{
		model.PermissionSysconsoleReadUserManagementGroups,
		model.PermissionSysconsoleReadUserManagementChannels,
	}
	if !c.App.SessionHasPermissionToAny(*c.AppContext.Session(), permissions) {
		c.SetPermissionError(permissions...)
		return
	}
	// Only system managers may use the ExcludePolicyConstrained parameter
	if c.Params.ExcludePolicyConstrained && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadComplianceDataRetentionPolicy) {
		c.SetPermissionError(model.PermissionSysconsoleReadComplianceDataRetentionPolicy)
		return
	}

	opts := model.ChannelSearchOpts{
		NotAssociatedToGroup:     c.Params.NotAssociatedToGroup,
		ExcludeDefaultChannels:   c.Params.ExcludeDefaultChannels,
		IncludeDeleted:           c.Params.IncludeDeleted,
		ExcludePolicyConstrained: c.Params.ExcludePolicyConstrained,
	}
	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadComplianceDataRetentionPolicy) {
		opts.IncludePolicyID = true
	}

	channels, err := c.App.GetAllChannels(c.Params.Page, c.Params.PerPage, opts)
	if err != nil {
		c.Err = err
		return
	}

	var payload []byte
	if c.Params.IncludeTotalCount {
		totalCount, err := c.App.GetAllChannelsCount(opts)
		if err != nil {
			c.Err = err
			return
		}
		cwc := &model.ChannelsWithCount{
			Channels:   channels,
			TotalCount: totalCount,
		}
		payload = cwc.ToJSON()
	} else {
		payload = []byte(channels.ToJSON())
	}

	w.Write(payload)
}

func getPublicChannelsForTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionListTeamChannels) {
		c.SetPermissionError(model.PermissionListTeamChannels)
		return
	}

	channels, err := c.App.GetPublicChannelsForTeam(c.Params.TeamID, c.Params.Page*c.Params.PerPage, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	err = c.App.FillInChannelsProps(channels)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(channels.ToJSON()))
}

func getDeletedChannelsForTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	channels, err := c.App.GetDeletedChannels(c.Params.TeamID, c.Params.Page*c.Params.PerPage, c.Params.PerPage, c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	err = c.App.FillInChannelsProps(channels)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(channels.ToJSON()))
}

func getPrivateChannelsForTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	channels, err := c.App.GetPrivateChannelsForTeam(c.Params.TeamID, c.Params.Page*c.Params.PerPage, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	err = c.App.FillInChannelsProps(channels)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(channels.ToJSON()))
}

func getPublicChannelsByIDsForTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	channelIDs := model.ArrayFromJSON(r.Body)
	if len(channelIDs) == 0 {
		c.SetInvalidParam("channel_ids")
		return
	}

	for _, cid := range channelIDs {
		if !model.IsValidID(cid) {
			c.SetInvalidParam("channel_id")
			return
		}
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	channels, err := c.App.GetPublicChannelsByIDsForTeam(c.Params.TeamID, channelIDs)
	if err != nil {
		c.Err = err
		return
	}

	err = c.App.FillInChannelsProps(channels)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(channels.ToJSON()))
}

func getChannelsForTeamForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	query := r.URL.Query()
	lastDeleteAt, nErr := strconv.Atoi(query.Get("last_delete_at"))
	if nErr != nil {
		lastDeleteAt = 0
	}
	if lastDeleteAt < 0 {
		c.SetInvalidUrlParam("last_delete_at")
		return
	}

	channels, err := c.App.GetChannelsForUser(c.Params.TeamID, c.Params.UserID, c.Params.IncludeDeleted, lastDeleteAt)
	if err != nil {
		c.Err = err
		return
	}

	if c.HandleEtag(channels.Etag(), "Get Channels", w, r) {
		return
	}

	err = c.App.FillInChannelsProps(channels)
	if err != nil {
		c.Err = err
		return
	}

	w.Header().Set(model.HeaderEtagServer, channels.Etag())
	w.Write([]byte(channels.ToJSON()))
}

func autocompleteChannelsForTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionListTeamChannels) {
		c.SetPermissionError(model.PermissionListTeamChannels)
		return
	}

	name := r.URL.Query().Get("name")

	channels, err := c.App.AutocompleteChannels(c.Params.TeamID, name)
	if err != nil {
		c.Err = err
		return
	}

	// Don't fill in channels props, since unused by client and potentially expensive.

	w.Write([]byte(channels.ToJSON()))
}

func autocompleteChannelsForTeamForSearch(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	name := r.URL.Query().Get("name")

	channels, err := c.App.AutocompleteChannelsForSearch(c.Params.TeamID, c.AppContext.Session().UserID, name)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(channels.ToJSON()))
}

func searchChannelsForTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	props := model.ChannelSearchFromJSON(r.Body)
	if props == nil {
		c.SetInvalidParam("channel_search")
		return
	}

	var channels *model.ChannelList
	var err *model.AppError
	if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionListTeamChannels) {
		channels, err = c.App.SearchChannels(c.Params.TeamID, props.Term)
	} else {
		// If the user is not a team member, return a 404
		if _, err = c.App.GetTeamMember(c.Params.TeamID, c.AppContext.Session().UserID); err != nil {
			c.Err = err
			return
		}

		channels, err = c.App.SearchChannelsForUser(c.AppContext.Session().UserID, c.Params.TeamID, props.Term)
	}

	if err != nil {
		c.Err = err
		return
	}

	// Don't fill in channels props, since unused by client and potentially expensive.

	w.Write([]byte(channels.ToJSON()))
}

func searchArchivedChannelsForTeam(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	props := model.ChannelSearchFromJSON(r.Body)
	if props == nil {
		c.SetInvalidParam("channel_search")
		return
	}

	var channels *model.ChannelList
	var err *model.AppError
	if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionListTeamChannels) {
		channels, err = c.App.SearchArchivedChannels(c.Params.TeamID, props.Term, c.AppContext.Session().UserID)
	} else {
		// If the user is not a team member, return a 404
		if _, err = c.App.GetTeamMember(c.Params.TeamID, c.AppContext.Session().UserID); err != nil {
			c.Err = err
			return
		}

		channels, err = c.App.SearchArchivedChannels(c.Params.TeamID, props.Term, c.AppContext.Session().UserID)
	}

	if err != nil {
		c.Err = err
		return
	}

	// Don't fill in channels props, since unused by client and potentially expensive.

	w.Write([]byte(channels.ToJSON()))
}

func searchAllChannels(c *Context, w http.ResponseWriter, r *http.Request) {
	props := model.ChannelSearchFromJSON(r.Body)
	if props == nil {
		c.SetInvalidParam("channel_search")
		return
	}
	// Only system managers may use the ExcludePolicyConstrained field
	if props.ExcludePolicyConstrained && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadComplianceDataRetentionPolicy) {
		c.SetPermissionError(model.PermissionSysconsoleReadComplianceDataRetentionPolicy)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementChannels) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementChannels)
		return
	}
	includeDeleted, _ := strconv.ParseBool(r.URL.Query().Get("include_deleted"))
	includeDeleted = includeDeleted || props.IncludeDeleted

	opts := model.ChannelSearchOpts{
		NotAssociatedToGroup:     props.NotAssociatedToGroup,
		ExcludeDefaultChannels:   props.ExcludeDefaultChannels,
		TeamIDs:                  props.TeamIDs,
		GroupConstrained:         props.GroupConstrained,
		ExcludeGroupConstrained:  props.ExcludeGroupConstrained,
		ExcludePolicyConstrained: props.ExcludePolicyConstrained,
		Public:                   props.Public,
		Private:                  props.Private,
		IncludeDeleted:           includeDeleted,
		Deleted:                  props.Deleted,
		Page:                     props.Page,
		PerPage:                  props.PerPage,
	}
	if c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadComplianceDataRetentionPolicy) {
		opts.IncludePolicyID = true
	}

	channels, totalCount, appErr := c.App.SearchAllChannels(props.Term, opts)
	if appErr != nil {
		c.Err = appErr
		return
	}

	// Don't fill in channels props, since unused by client and potentially expensive.
	var payload []byte
	if props.Page != nil && props.PerPage != nil {
		data := model.ChannelsWithCount{Channels: channels, TotalCount: totalCount}
		payload = data.ToJSON()
	} else {
		payload = []byte(channels.ToJSON())
	}

	w.Write(payload)
}

func deleteChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("deleteChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channeld", channel)

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		c.Err = model.NewAppError("deleteChannel", "api.channel.delete_channel.type.invalid", nil, "", http.StatusBadRequest)
		return
	}

	if channel.Type == model.ChannelTypeOpen && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionDeletePublicChannel) {
		c.SetPermissionError(model.PermissionDeletePublicChannel)
		return
	}

	if channel.Type == model.ChannelTypePrivate && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionDeletePrivateChannel) {
		c.SetPermissionError(model.PermissionDeletePrivateChannel)
		return
	}

	if c.Params.Permanent {
		if *c.App.Config().ServiceSettings.EnableAPIChannelDeletion {
			err = c.App.PermanentDeleteChannel(channel)
		} else {
			err = model.NewAppError("deleteChannel", "api.user.delete_channel.not_enabled.app_error", nil, "channelId="+c.Params.ChannelID, http.StatusUnauthorized)
		}
	} else {
		err = c.App.DeleteChannel(c.AppContext, channel, c.AppContext.Session().UserID)
	}
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + channel.Name)

	ReturnStatusOK(w)
}

func getChannelByName(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID().RequireChannelName()
	if c.Err != nil {
		return
	}

	includeDeleted, _ := strconv.ParseBool(r.URL.Query().Get("include_deleted"))
	channel, appErr := c.App.GetChannelByName(c.Params.ChannelName, c.Params.TeamID, includeDeleted)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if channel.Type == model.ChannelTypeOpen {
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionReadPublicChannel) && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionReadChannel) {
			c.SetPermissionError(model.PermissionReadPublicChannel)
			return
		}
	} else {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionReadChannel) {
			c.Err = model.NewAppError("getChannelByName", "app.channel.get_by_name.missing.app_error", nil, "teamId="+channel.TeamID+", "+"name="+channel.Name+"", http.StatusNotFound)
			return
		}
	}

	appErr = c.App.FillInChannelProps(channel)
	if appErr != nil {
		c.Err = appErr
		return
	}

	w.Write([]byte(channel.ToJSON()))
}

func getChannelByNameForTeamName(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamName().RequireChannelName()
	if c.Err != nil {
		return
	}

	includeDeleted, _ := strconv.ParseBool(r.URL.Query().Get("include_deleted"))
	channel, appErr := c.App.GetChannelByNameForTeamName(c.Params.ChannelName, c.Params.TeamName, includeDeleted)
	if appErr != nil {
		c.Err = appErr
		return
	}

	teamOk := c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionReadPublicChannel)
	channelOk := c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionReadChannel)

	if channel.Type == model.ChannelTypeOpen {
		if !teamOk && !channelOk {
			c.SetPermissionError(model.PermissionReadPublicChannel)
			return
		}
	} else if !channelOk {
		c.Err = model.NewAppError("getChannelByNameForTeamName", "app.channel.get_by_name.missing.app_error", nil, "teamId="+channel.TeamID+", "+"name="+channel.Name+"", http.StatusNotFound)
		return
	}

	appErr = c.App.FillInChannelProps(channel)
	if appErr != nil {
		c.Err = appErr
		return
	}

	w.Write([]byte(channel.ToJSON()))
}

func getChannelMembers(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	members, err := c.App.GetChannelMembersPage(c.Params.ChannelID, c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(members.ToJSON()))
}

func getChannelMembersTimezones(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	membersTimezones, err := c.App.GetChannelMembersTimezones(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.ArrayToJSON(membersTimezones)))
}

func getChannelMembersByIDs(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	userIDs := model.ArrayFromJSON(r.Body)
	if len(userIDs) == 0 {
		c.SetInvalidParam("user_ids")
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	members, err := c.App.GetChannelMembersByIDs(c.Params.ChannelID, userIDs)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(members.ToJSON()))
}

func getChannelMember(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID().RequireUserID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	member, err := c.App.GetChannelMember(app.WithMaster(context.Background()), c.Params.ChannelID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(member.ToJSON()))
}

func getChannelMembersForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	if c.AppContext.Session().UserID != c.Params.UserID && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	members, err := c.App.GetChannelMembersForUser(c.Params.TeamID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(members.ToJSON()))
}

func viewChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	view := model.ChannelViewFromJSON(r.Body)
	if view == nil {
		c.SetInvalidParam("channel_view")
		return
	}

	// Validate view struct
	// Check IDs are valid or blank. Blank IDs are used to denote focus loss or initial channel view.
	if view.ChannelID != "" && !model.IsValidID(view.ChannelID) {
		c.SetInvalidParam("channel_view.channel_id")
		return
	}
	if view.PrevChannelID != "" && !model.IsValidID(view.PrevChannelID) {
		c.SetInvalidParam("channel_view.prev_channel_id")
		return
	}

	times, err := c.App.ViewChannel(view, c.Params.UserID, c.AppContext.Session().ID, view.CollapsedThreadsSupported)
	if err != nil {
		c.Err = err
		return
	}

	c.App.UpdateLastActivityAtIfNeeded(*c.AppContext.Session())
	c.ExtendSessionExpiryIfNeeded(w, r)

	// Returning {"status": "OK", ...} for backwards compatibility
	resp := &model.ChannelViewResponse{
		Status:            "OK",
		LastViewedAtTimes: times,
	}

	w.Write([]byte(resp.ToJSON()))
}

func updateChannelMemberRoles(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID().RequireUserID()
	if c.Err != nil {
		return
	}

	props := model.MapFromJSON(r.Body)

	newRoles := props["roles"]
	if !(model.IsValidUserRoles(newRoles)) {
		c.SetInvalidParam("roles")
		return
	}

	auditRec := c.MakeAuditRecord("updateChannelMemberRoles", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel_id", c.Params.ChannelID)
	auditRec.AddMeta("roles", newRoles)

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionManageChannelRoles) {
		c.SetPermissionError(model.PermissionManageChannelRoles)
		return
	}

	if _, err := c.App.UpdateChannelMemberRoles(c.Params.ChannelID, c.Params.UserID, newRoles); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	ReturnStatusOK(w)
}

func updateChannelMemberSchemeRoles(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID().RequireUserID()
	if c.Err != nil {
		return
	}

	schemeRoles := model.SchemeRolesFromJSON(r.Body)
	if schemeRoles == nil {
		c.SetInvalidParam("scheme_roles")
		return
	}

	auditRec := c.MakeAuditRecord("updateChannelMemberSchemeRoles", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel_id", c.Params.ChannelID)
	auditRec.AddMeta("roles", schemeRoles)

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionManageChannelRoles) {
		c.SetPermissionError(model.PermissionManageChannelRoles)
		return
	}

	if _, err := c.App.UpdateChannelMemberSchemeRoles(c.Params.ChannelID, c.Params.UserID, schemeRoles.SchemeGuest, schemeRoles.SchemeUser, schemeRoles.SchemeAdmin); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	ReturnStatusOK(w)
}

func updateChannelMemberNotifyProps(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID().RequireUserID()
	if c.Err != nil {
		return
	}

	props := model.MapFromJSON(r.Body)
	if props == nil {
		c.SetInvalidParam("notify_props")
		return
	}

	auditRec := c.MakeAuditRecord("updateChannelMemberNotifyProps", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel_id", c.Params.ChannelID)
	auditRec.AddMeta("props", props)

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	_, err := c.App.UpdateChannelMemberNotifyProps(props, c.Params.ChannelID, c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	ReturnStatusOK(w)
}

func addChannelMember(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	props := model.StringInterfaceFromJSON(r.Body)
	userID, ok := props["user_id"].(string)
	if !ok || !model.IsValidID(userID) {
		c.SetInvalidParam("user_id")
		return
	}

	member := &model.ChannelMember{
		ChannelID: c.Params.ChannelID,
		UserID:    userID,
	}

	postRootID, ok := props["post_root_id"].(string)
	if ok && postRootID != "" && !model.IsValidID(postRootID) {
		c.SetInvalidParam("post_root_id")
		return
	}

	if ok && len(postRootID) == 26 {
		rootPost, err := c.App.GetSinglePost(postRootID)
		if err != nil {
			c.Err = err
			return
		}
		if rootPost.ChannelID != member.ChannelID {
			c.SetInvalidParam("post_root_id")
			return
		}
	}

	channel, err := c.App.GetChannel(member.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("addChannelMember", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		c.Err = model.NewAppError("addUserToChannel", "api.channel.add_user_to_channel.type.app_error", nil, "", http.StatusBadRequest)
		return
	}

	isNewMembership := false
	if _, err = c.App.GetChannelMember(context.Background(), member.ChannelID, member.UserID); err != nil {
		if err.ID == app.MissingChannelMemberError {
			isNewMembership = true
		} else {
			c.Err = err
			return
		}
	}

	isSelfAdd := member.UserID == c.AppContext.Session().UserID

	if channel.Type == model.ChannelTypeOpen {
		if isSelfAdd && isNewMembership {
			if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionJoinPublicChannels) {
				c.SetPermissionError(model.PermissionJoinPublicChannels)
				return
			}
		} else if isSelfAdd && !isNewMembership {
			// nothing to do, since already in the channel
		} else if !isSelfAdd {
			if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionManagePublicChannelMembers) {
				c.SetPermissionError(model.PermissionManagePublicChannelMembers)
				return
			}
		}
	}

	if channel.Type == model.ChannelTypePrivate {
		if isSelfAdd && isNewMembership {
			if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionManagePrivateChannelMembers) {
				c.SetPermissionError(model.PermissionManagePrivateChannelMembers)
				return
			}
		} else if isSelfAdd && !isNewMembership {
			// nothing to do, since already in the channel
		} else if !isSelfAdd {
			if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionManagePrivateChannelMembers) {
				c.SetPermissionError(model.PermissionManagePrivateChannelMembers)
				return
			}
		}
	}

	if channel.IsGroupConstrained() {
		nonMembers, err := c.App.FilterNonGroupChannelMembers([]string{member.UserID}, channel)
		if err != nil {
			if v, ok := err.(*model.AppError); ok {
				c.Err = v
			} else {
				c.Err = model.NewAppError("addChannelMember", "api.channel.add_members.error", nil, err.Error(), http.StatusBadRequest)
			}
			return
		}
		if len(nonMembers) > 0 {
			c.Err = model.NewAppError("addChannelMember", "api.channel.add_members.user_denied", map[string]interface{}{"UserIDs": nonMembers}, "", http.StatusBadRequest)
			return
		}
	}

	cm, err := c.App.AddChannelMember(c.AppContext, member.UserID, channel, app.ChannelMemberOpts{
		UserRequestorID: c.AppContext.Session().UserID,
		PostRootID:      postRootID,
	})
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("add_user_id", cm.UserID)
	c.LogAudit("name=" + channel.Name + " user_id=" + cm.UserID)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(cm.ToJSON()))
}

func removeChannelMember(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID().RequireUserID()
	if c.Err != nil {
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	user, err := c.App.GetUser(c.Params.UserID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("removeChannelMember", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("remove_user_id", user.ID)

	if !(channel.Type == model.ChannelTypeOpen || channel.Type == model.ChannelTypePrivate) {
		c.Err = model.NewAppError("removeChannelMember", "api.channel.remove_channel_member.type.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if channel.IsGroupConstrained() && (c.Params.UserID != c.AppContext.Session().UserID) && !user.IsBot {
		c.Err = model.NewAppError("removeChannelMember", "api.channel.remove_member.group_constrained.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if c.Params.UserID != c.AppContext.Session().UserID {
		if channel.Type == model.ChannelTypeOpen && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionManagePublicChannelMembers) {
			c.SetPermissionError(model.PermissionManagePublicChannelMembers)
			return
		}

		if channel.Type == model.ChannelTypePrivate && !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionManagePrivateChannelMembers) {
			c.SetPermissionError(model.PermissionManagePrivateChannelMembers)
			return
		}
	}

	if err = c.App.RemoveUserFromChannel(c.AppContext, c.Params.UserID, c.AppContext.Session().UserID, channel); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + channel.Name + " user_id=" + c.Params.UserID)

	ReturnStatusOK(w)
}

func updateChannelScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	schemeID := model.SchemeIDFromJSON(r.Body)
	if schemeID == nil || !model.IsValidID(*schemeID) {
		c.SetInvalidParam("scheme_id")
		return
	}

	auditRec := c.MakeAuditRecord("updateChannelScheme", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("new_scheme_id", schemeID)

	if c.App.Srv().License() == nil {
		c.Err = model.NewAppError("Api4.UpdateChannelScheme", "api.channel.update_channel_scheme.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	scheme, err := c.App.GetScheme(*schemeID)
	if err != nil {
		c.Err = err
		return
	}

	if scheme.Scope != model.SchemeScopeChannel {
		c.Err = model.NewAppError("Api4.UpdateChannelScheme", "api.channel.update_channel_scheme.scheme_scope.error", nil, "", http.StatusBadRequest)
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("old_scheme_id", channel.SchemeID)

	channel.SchemeID = &scheme.ID

	_, err = c.App.UpdateChannelScheme(channel)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	ReturnStatusOK(w)
}

func channelMembersMinusGroupMembers(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	groupIDsParam := groupIDsQueryParamRegex.ReplaceAllString(c.Params.GroupIDs, "")

	if len(groupIDsParam) < 26 {
		c.SetInvalidParam("group_ids")
		return
	}

	groupIDs := []string{}
	for _, gid := range strings.Split(c.Params.GroupIDs, ",") {
		if !model.IsValidID(gid) {
			c.SetInvalidParam("group_ids")
			return
		}
		groupIDs = append(groupIDs, gid)
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementChannels) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementChannels)
		return
	}

	users, totalCount, err := c.App.ChannelMembersMinusGroupMembers(
		c.Params.ChannelID,
		groupIDs,
		c.Params.Page,
		c.Params.PerPage,
	)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(&model.UsersWithGroupsAndCount{
		Users: users,
		Count: totalCount,
	})
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.channelMembersMinusGroupMembers", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func channelMemberCountsByGroup(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil {
		c.Err = model.NewAppError("Api4.channelMemberCountsByGroup", "api.channel.channel_member_counts_by_group.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), c.Params.ChannelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	includeTimezones := r.URL.Query().Get("include_timezones") == "true"

	channelMemberCounts, err := c.App.GetMemberCountsByGroup(app.WithMaster(context.Background()), c.Params.ChannelID, includeTimezones)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(channelMemberCounts)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.channelMemberCountsByGroup", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func getChannelModerations(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil {
		c.Err = model.NewAppError("Api4.GetChannelModerations", "api.channel.get_channel_moderations.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementChannels) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementChannels)
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	channelModerations, err := c.App.GetChannelModerationsForChannel(channel)
	if err != nil {
		c.Err = err
		return
	}

	b, marshalErr := json.Marshal(channelModerations)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.getChannelModerations", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

func patchChannelModerations(c *Context, w http.ResponseWriter, r *http.Request) {
	if c.App.Srv().License() == nil {
		c.Err = model.NewAppError("Api4.patchChannelModerations", "api.channel.patch_channel_moderations.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("patchChannelModerations", audit.Fail)
	defer c.LogAuditRec(auditRec)

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementChannels) {
		c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementChannels)
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("channel", channel)

	channelModerationsPatch := model.ChannelModerationsPatchFromJSON(r.Body)
	channelModerations, err := c.App.PatchChannelModerationsForChannel(channel, channelModerationsPatch)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("patch", channelModerationsPatch)

	b, marshalErr := json.Marshal(channelModerations)
	if marshalErr != nil {
		c.Err = model.NewAppError("Api4.patchChannelModerations", "api.marshal_error", nil, marshalErr.Error(), http.StatusInternalServerError)
		return
	}

	auditRec.Success()
	w.Write(b)
}

func moveChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	props := model.StringInterfaceFromJSON(r.Body)
	teamID, ok := props["team_id"].(string)
	if !ok {
		c.SetInvalidParam("team_id")
		return
	}

	force, ok := props["force"].(bool)
	if !ok {
		c.SetInvalidParam("force")
		return
	}

	team, err := c.App.GetTeam(teamID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("moveChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel_id", channel.ID)
	auditRec.AddMeta("channel_name", channel.Name)
	auditRec.AddMeta("team_id", team.ID)
	auditRec.AddMeta("team_name", team.Name)

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		c.Err = model.NewAppError("moveChannel", "api.channel.move_channel.type.invalid", nil, "", http.StatusForbidden)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	user, err := c.App.GetUser(c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	err = c.App.RemoveAllDeactivatedMembersFromChannel(channel)
	if err != nil {
		c.Err = err
		return
	}

	if force {
		err = c.App.RemoveUsersFromChannelNotMemberOfTeam(c.AppContext, user, channel, team)
		if err != nil {
			c.Err = err
			return
		}
	}

	err = c.App.MoveChannel(c.AppContext, team, channel, user)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("channel=" + channel.Name)
	c.LogAudit("team=" + team.Name)

	w.Write([]byte(channel.ToJSON()))
}
