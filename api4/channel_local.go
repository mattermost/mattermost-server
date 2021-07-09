// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitChannelLocal() {
	api.BaseRoutes.Channels.Handle("", api.APILocal(getAllChannels)).Methods("GET")
	api.BaseRoutes.Channels.Handle("", api.APILocal(localCreateChannel)).Methods("POST")
	api.BaseRoutes.Channel.Handle("", api.APILocal(getChannel)).Methods("GET")
	api.BaseRoutes.ChannelByName.Handle("", api.APILocal(getChannelByName)).Methods("GET")
	api.BaseRoutes.Channel.Handle("", api.APILocal(localDeleteChannel)).Methods("DELETE")
	api.BaseRoutes.Channel.Handle("/patch", api.APILocal(localPatchChannel)).Methods("PUT")
	api.BaseRoutes.Channel.Handle("/move", api.APILocal(localMoveChannel)).Methods("POST")
	api.BaseRoutes.Channel.Handle("/privacy", api.APILocal(localUpdateChannelPrivacy)).Methods("PUT")
	api.BaseRoutes.Channel.Handle("/restore", api.APILocal(localRestoreChannel)).Methods("POST")

	api.BaseRoutes.ChannelMember.Handle("", api.APILocal(localRemoveChannelMember)).Methods("DELETE")
	api.BaseRoutes.ChannelMember.Handle("", api.APILocal(getChannelMember)).Methods("GET")
	api.BaseRoutes.ChannelMembers.Handle("", api.APILocal(localAddChannelMember)).Methods("POST")
	api.BaseRoutes.ChannelMembers.Handle("", api.APILocal(getChannelMembers)).Methods("GET")

	api.BaseRoutes.ChannelsForTeam.Handle("", api.APILocal(getPublicChannelsForTeam)).Methods("GET")
	api.BaseRoutes.ChannelsForTeam.Handle("/deleted", api.APILocal(getDeletedChannelsForTeam)).Methods("GET")
	api.BaseRoutes.ChannelsForTeam.Handle("/private", api.APILocal(getPrivateChannelsForTeam)).Methods("GET")

	api.BaseRoutes.ChannelByName.Handle("", api.APILocal(getChannelByName)).Methods("GET")
	api.BaseRoutes.ChannelByNameForTeamName.Handle("", api.APILocal(getChannelByNameForTeamName)).Methods("GET")
}

func localCreateChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	channel := model.ChannelFromJSON(r.Body)
	if channel == nil {
		c.SetInvalidParam("channel")
		return
	}

	auditRec := c.MakeAuditRecord("localCreateChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)

	sc, err := c.App.CreateChannel(c.AppContext, channel, false)
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

func localUpdateChannelPrivacy(c *Context, w http.ResponseWriter, r *http.Request) {
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

	auditRec := c.MakeAuditRecord("localUpdateChannelPrivacy", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("new_type", privacy)

	if channel.Name == model.DefaultChannelName && privacy == model.ChannelTypePrivate {
		c.Err = model.NewAppError("updateChannelPrivacy", "api.channel.update_channel_privacy.default_channel_error", nil, "", http.StatusBadRequest)
		return
	}
	channel.Type = privacy

	updatedChannel, err := c.App.UpdateChannelPrivacy(c.AppContext, channel, nil)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + updatedChannel.Name)

	w.Write([]byte(updatedChannel.ToJSON()))
}

func localRestoreChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("localRestoreChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)

	channel, err = c.App.RestoreChannel(c.AppContext, channel, "")
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + channel.Name)

	w.Write([]byte(channel.ToJSON()))
}

func localAddChannelMember(c *Context, w http.ResponseWriter, r *http.Request) {
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

	auditRec := c.MakeAuditRecord("localAddChannelMember", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		c.Err = model.NewAppError("localAddChannelMember", "api.channel.add_user_to_channel.type.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if channel.IsGroupConstrained() {
		nonMembers, err := c.App.FilterNonGroupChannelMembers([]string{member.UserID}, channel)
		if err != nil {
			if v, ok := err.(*model.AppError); ok {
				c.Err = v
			} else {
				c.Err = model.NewAppError("localAddChannelMember", "api.channel.add_members.error", nil, err.Error(), http.StatusBadRequest)
			}
			return
		}
		if len(nonMembers) > 0 {
			c.Err = model.NewAppError("localAddChannelMember", "api.channel.add_members.user_denied", map[string]interface{}{"UserIDs": nonMembers}, "", http.StatusBadRequest)
			return
		}
	}

	cm, err := c.App.AddChannelMember(c.AppContext, member.UserID, channel, app.ChannelMemberOpts{
		PostRootID: postRootID,
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

func localRemoveChannelMember(c *Context, w http.ResponseWriter, r *http.Request) {
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

	if !(channel.Type == model.ChannelTypeOpen || channel.Type == model.ChannelTypePrivate) {
		c.Err = model.NewAppError("removeChannelMember", "api.channel.remove_channel_member.type.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if channel.IsGroupConstrained() && !user.IsBot {
		c.Err = model.NewAppError("removeChannelMember", "api.channel.remove_member.group_constrained.app_error", nil, "", http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord("localRemoveChannelMember", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)
	auditRec.AddMeta("remove_user_id", user.ID)

	if err = c.App.RemoveUserFromChannel(c.AppContext, c.Params.UserID, "", channel); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + channel.Name + " user_id=" + c.Params.UserID)

	ReturnStatusOK(w)
}

func localPatchChannel(c *Context, w http.ResponseWriter, r *http.Request) {
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
	channel := originalOldChannel.DeepCopy()

	auditRec := c.MakeAuditRecord("localPatchChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel", channel)

	channel.Patch(patch)
	rchannel, err := c.App.UpdateChannel(channel)
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

func localMoveChannel(c *Context, w http.ResponseWriter, r *http.Request) {
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

	auditRec := c.MakeAuditRecord("localMoveChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channel_id", channel.ID)
	auditRec.AddMeta("channel_name", channel.Name)
	auditRec.AddMeta("team_id", team.ID)
	auditRec.AddMeta("team_name", team.Name)

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		c.Err = model.NewAppError("moveChannel", "api.channel.move_channel.type.invalid", nil, "", http.StatusForbidden)
		return
	}

	err = c.App.RemoveAllDeactivatedMembersFromChannel(channel)
	if err != nil {
		c.Err = err
		return
	}

	if force {
		err = c.App.RemoveUsersFromChannelNotMemberOfTeam(c.AppContext, nil, channel, team)
		if err != nil {
			c.Err = err
			return
		}
	}

	err = c.App.MoveChannel(c.AppContext, team, channel, nil)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("channel=" + channel.Name)
	c.LogAudit("team=" + team.Name)

	w.Write([]byte(channel.ToJSON()))
}

func localDeleteChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	channel, err := c.App.GetChannel(c.Params.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	auditRec := c.MakeAuditRecord("localDeleteChannel", audit.Fail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("channeld", channel)

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		c.Err = model.NewAppError("localDeleteChannel", "api.channel.delete_channel.type.invalid", nil, "", http.StatusBadRequest)
		return
	}

	if c.Params.Permanent {
		err = c.App.PermanentDeleteChannel(channel)
	} else {
		err = c.App.DeleteChannel(c.AppContext, channel, "")
	}
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	c.LogAudit("name=" + channel.Name)

	ReturnStatusOK(w)
}
