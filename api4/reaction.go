// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
)

func (api *API) InitReaction() {
	api.BaseRoutes.Reactions.Handle("", api.ApiSessionRequired(saveReaction)).Methods("POST")
	api.BaseRoutes.Post.Handle("/reactions", api.ApiSessionRequired(getReactions)).Methods("GET")
	api.BaseRoutes.ReactionByNameForPostForUser.Handle("", api.ApiSessionRequired(deleteReaction)).Methods("DELETE")
	api.BaseRoutes.Posts.Handle("/ids/reactions", api.ApiSessionRequired(getBulkReactions)).Methods("POST")
}

func saveReaction(c *Context, w http.ResponseWriter, r *http.Request) {
	reaction := model.ReactionFromJSON(r.Body)
	if reaction == nil {
		c.SetInvalidParam("reaction")
		return
	}

	if !model.IsValidID(reaction.UserID) || !model.IsValidID(reaction.PostID) || reaction.EmojiName == "" || len(reaction.EmojiName) > model.EmojiNameMaxLength {
		c.Err = model.NewAppError("saveReaction", "api.reaction.save_reaction.invalid.app_error", nil, "", http.StatusBadRequest)
		return
	}

	if reaction.UserID != c.AppContext.Session().UserID {
		c.Err = model.NewAppError("saveReaction", "api.reaction.save_reaction.user_id.app_error", nil, "", http.StatusForbidden)
		return
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), reaction.PostID, model.PermissionAddReaction) {
		c.SetPermissionError(model.PermissionAddReaction)
		return
	}

	reaction, err := c.App.SaveReactionForPost(c.AppContext, reaction)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(reaction.ToJSON()))
}

func getReactions(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	reactions, err := c.App.GetReactionsForPost(c.Params.PostID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.ReactionsToJSON(reactions)))
}

func deleteReaction(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	c.RequirePostID()
	if c.Err != nil {
		return
	}

	c.RequireEmojiName()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostID, model.PermissionRemoveReaction) {
		c.SetPermissionError(model.PermissionRemoveReaction)
		return
	}

	if c.Params.UserID != c.AppContext.Session().UserID && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionRemoveOthersReactions) {
		c.SetPermissionError(model.PermissionRemoveOthersReactions)
		return
	}

	reaction := &model.Reaction{
		UserID:    c.Params.UserID,
		PostID:    c.Params.PostID,
		EmojiName: c.Params.EmojiName,
	}

	err := c.App.DeleteReactionForPost(c.AppContext, reaction)
	if err != nil {
		c.Err = err
		return
	}

	ReturnStatusOK(w)
}

func getBulkReactions(c *Context, w http.ResponseWriter, r *http.Request) {
	postIDs := model.ArrayFromJSON(r.Body)
	for _, postID := range postIDs {
		if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), postID, model.PermissionReadChannel) {
			c.SetPermissionError(model.PermissionReadChannel)
			return
		}
	}
	reactions, err := c.App.GetBulkReactionsForPosts(postIDs)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.MapPostIDToReactionsToJSON(reactions)))
}
