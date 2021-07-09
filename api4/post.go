// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

func (api *API) InitPost() {
	api.BaseRoutes.Posts.Handle("", api.APISessionRequired(createPost)).Methods("POST")
	api.BaseRoutes.Post.Handle("", api.APISessionRequired(getPost)).Methods("GET")
	api.BaseRoutes.Post.Handle("", api.APISessionRequired(deletePost)).Methods("DELETE")
	api.BaseRoutes.Posts.Handle("/ephemeral", api.APISessionRequired(createEphemeralPost)).Methods("POST")
	api.BaseRoutes.Post.Handle("/thread", api.APISessionRequired(getPostThread)).Methods("GET")
	api.BaseRoutes.Post.Handle("/files/info", api.APISessionRequired(getFileInfosForPost)).Methods("GET")
	api.BaseRoutes.PostsForChannel.Handle("", api.APISessionRequired(getPostsForChannel)).Methods("GET")
	api.BaseRoutes.PostsForUser.Handle("/flagged", api.APISessionRequired(getFlaggedPostsForUser)).Methods("GET")

	api.BaseRoutes.ChannelForUser.Handle("/posts/unread", api.APISessionRequired(getPostsForChannelAroundLastUnread)).Methods("GET")

	api.BaseRoutes.Team.Handle("/posts/search", api.APISessionRequiredDisableWhenBusy(searchPosts)).Methods("POST")
	api.BaseRoutes.Post.Handle("", api.APISessionRequired(updatePost)).Methods("PUT")
	api.BaseRoutes.Post.Handle("/patch", api.APISessionRequired(patchPost)).Methods("PUT")
	api.BaseRoutes.PostForUser.Handle("/set_unread", api.APISessionRequired(setPostUnread)).Methods("POST")
	api.BaseRoutes.Post.Handle("/pin", api.APISessionRequired(pinPost)).Methods("POST")
	api.BaseRoutes.Post.Handle("/unpin", api.APISessionRequired(unpinPost)).Methods("POST")
}

func createPost(c *Context, w http.ResponseWriter, r *http.Request) {
	post := model.PostFromJSON(r.Body)
	if post == nil {
		c.SetInvalidParam("post")
		return
	}

	post.UserID = c.AppContext.Session().UserID

	auditRec := c.MakeAuditRecord("createPost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	auditRec.AddMeta("post", post)

	hasPermission := false
	if c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), post.ChannelID, model.PermissionCreatePost) {
		hasPermission = true
	} else if channel, err := c.App.GetChannel(post.ChannelID); err == nil {
		// Temporary permission check method until advanced permissions, please do not copy
		if channel.Type == model.ChannelTypeOpen && c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionCreatePostPublic) {
			hasPermission = true
		}
	}

	if !hasPermission {
		c.SetPermissionError(model.PermissionCreatePost)
		return
	}

	if post.CreateAt != 0 && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		post.CreateAt = 0
	}

	setOnline := r.URL.Query().Get("set_online")
	setOnlineBool := true // By default, always set online.
	var err2 error
	if setOnline != "" {
		setOnlineBool, err2 = strconv.ParseBool(setOnline)
		if err2 != nil {
			mlog.Warn("Failed to parse set_online URL query parameter from createPost request", mlog.Err(err2))
			setOnlineBool = true // Set online nevertheless.
		}
	}

	rp, err := c.App.CreatePostAsUser(c.AppContext, c.App.PostWithProxyRemovedFromImageURLs(post), c.AppContext.Session().ID, setOnlineBool)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	auditRec.AddMeta("post", rp) // overwrite meta

	if setOnlineBool {
		c.App.SetStatusOnline(c.AppContext.Session().UserID, false)
	}

	c.App.UpdateLastActivityAtIfNeeded(*c.AppContext.Session())
	c.ExtendSessionExpiryIfNeeded(w, r)

	w.WriteHeader(http.StatusCreated)

	// Note that rp has already had PreparePostForClient called on it by App.CreatePost
	w.Write([]byte(rp.ToJSON()))
}

func createEphemeralPost(c *Context, w http.ResponseWriter, r *http.Request) {
	ephRequest := model.PostEphemeral{}

	json.NewDecoder(r.Body).Decode(&ephRequest)
	if ephRequest.UserID == "" {
		c.SetInvalidParam("user_id")
		return
	}

	if ephRequest.Post == nil {
		c.SetInvalidParam("post")
		return
	}

	ephRequest.Post.UserID = c.AppContext.Session().UserID
	ephRequest.Post.CreateAt = model.GetMillis()

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionCreatePostEphemeral) {
		c.SetPermissionError(model.PermissionCreatePostEphemeral)
		return
	}

	rp := c.App.SendEphemeralPost(ephRequest.UserID, c.App.PostWithProxyRemovedFromImageURLs(ephRequest.Post))

	w.WriteHeader(http.StatusCreated)
	rp = model.AddPostActionCookies(rp, c.App.PostActionCookieSecret())
	rp = c.App.PreparePostForClient(rp, true, false)
	w.Write([]byte(rp.ToJSON()))
}

func getPostsForChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelID()
	if c.Err != nil {
		return
	}

	afterPost := r.URL.Query().Get("after")
	if afterPost != "" && !model.IsValidID(afterPost) {
		c.SetInvalidParam("after")
		return
	}

	beforePost := r.URL.Query().Get("before")
	if beforePost != "" && !model.IsValidID(beforePost) {
		c.SetInvalidParam("before")
		return
	}

	sinceString := r.URL.Query().Get("since")
	var since int64
	var parseError error
	if sinceString != "" {
		since, parseError = strconv.ParseInt(sinceString, 10, 64)
		if parseError != nil {
			c.SetInvalidParam("since")
			return
		}
	}
	skipFetchThreads := r.URL.Query().Get("skipFetchThreads") == "true"
	collapsedThreads := r.URL.Query().Get("collapsedThreads") == "true"
	collapsedThreadsExtended := r.URL.Query().Get("collapsedThreadsExtended") == "true"
	channelID := c.Params.ChannelID
	page := c.Params.Page
	perPage := c.Params.PerPage

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	var list *model.PostList
	var err *model.AppError
	etag := ""

	if since > 0 {
		list, err = c.App.GetPostsSince(model.GetPostsSinceOptions{ChannelID: channelID, Time: since, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, CollapsedThreadsExtended: collapsedThreadsExtended, UserID: c.AppContext.Session().UserID})
	} else if afterPost != "" {
		etag = c.App.GetPostsEtag(channelID, collapsedThreads)

		if c.HandleEtag(etag, "Get Posts After", w, r) {
			return
		}

		list, err = c.App.GetPostsAfterPost(model.GetPostsOptions{ChannelID: channelID, PostID: afterPost, Page: page, PerPage: perPage, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, UserID: c.AppContext.Session().UserID})
	} else if beforePost != "" {
		etag = c.App.GetPostsEtag(channelID, collapsedThreads)

		if c.HandleEtag(etag, "Get Posts Before", w, r) {
			return
		}

		list, err = c.App.GetPostsBeforePost(model.GetPostsOptions{ChannelID: channelID, PostID: beforePost, Page: page, PerPage: perPage, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, CollapsedThreadsExtended: collapsedThreadsExtended, UserID: c.AppContext.Session().UserID})
	} else {
		etag = c.App.GetPostsEtag(channelID, collapsedThreads)

		if c.HandleEtag(etag, "Get Posts", w, r) {
			return
		}

		list, err = c.App.GetPostsPage(model.GetPostsOptions{ChannelID: channelID, Page: page, PerPage: perPage, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, CollapsedThreadsExtended: collapsedThreadsExtended, UserID: c.AppContext.Session().UserID})
	}

	if err != nil {
		c.Err = err
		return
	}

	if etag != "" {
		w.Header().Set(model.HeaderEtagServer, etag)
	}

	c.App.AddCursorIDsForPostList(list, afterPost, beforePost, since, page, perPage, collapsedThreads)
	clientPostList := c.App.PreparePostListForClient(list)

	w.Write([]byte(clientPostList.ToJSON()))
}

func getPostsForChannelAroundLastUnread(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID().RequireChannelID()
	if c.Err != nil {
		return
	}

	userID := c.Params.UserID
	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), userID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	channelID := c.Params.ChannelID
	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channelID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	if c.Params.LimitAfter == 0 {
		c.SetInvalidURLParam("limit_after")
		return
	}

	skipFetchThreads := r.URL.Query().Get("skipFetchThreads") == "true"
	collapsedThreads := r.URL.Query().Get("collapsedThreads") == "true"
	collapsedThreadsExtended := r.URL.Query().Get("collapsedThreadsExtended") == "true"

	postList, err := c.App.GetPostsForChannelAroundLastUnread(channelID, userID, c.Params.LimitBefore, c.Params.LimitAfter, skipFetchThreads, collapsedThreads, collapsedThreadsExtended)
	if err != nil {
		c.Err = err
		return
	}

	etag := ""
	if len(postList.Order) == 0 {
		etag = c.App.GetPostsEtag(channelID, collapsedThreads)

		if c.HandleEtag(etag, "Get Posts", w, r) {
			return
		}

		postList, err = c.App.GetPostsPage(model.GetPostsOptions{ChannelID: channelID, Page: app.PageDefault, PerPage: c.Params.LimitBefore, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, CollapsedThreadsExtended: collapsedThreadsExtended, UserID: c.AppContext.Session().UserID})
		if err != nil {
			c.Err = err
			return
		}
	}

	postList.NextPostID = c.App.GetNextPostIDFromPostList(postList, collapsedThreads)
	postList.PrevPostID = c.App.GetPrevPostIDFromPostList(postList, collapsedThreads)

	clientPostList := c.App.PreparePostListForClient(postList)

	if etag != "" {
		w.Header().Set(model.HeaderEtagServer, etag)
	}
	w.Write([]byte(clientPostList.ToJSON()))
}

func getFlaggedPostsForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	channelID := r.URL.Query().Get("channel_id")
	teamID := r.URL.Query().Get("team_id")

	var posts *model.PostList
	var err *model.AppError

	if channelID != "" {
		posts, err = c.App.GetFlaggedPostsForChannel(c.Params.UserID, channelID, c.Params.Page, c.Params.PerPage)
	} else if teamID != "" {
		posts, err = c.App.GetFlaggedPostsForTeam(c.Params.UserID, teamID, c.Params.Page, c.Params.PerPage)
	} else {
		posts, err = c.App.GetFlaggedPosts(c.Params.UserID, c.Params.Page, c.Params.PerPage)
	}
	if err != nil {
		c.Err = err
		return
	}

	pl := model.NewPostList()
	channelReadPermission := make(map[string]bool)

	for _, post := range posts.Posts {
		allowed, ok := channelReadPermission[post.ChannelID]

		if !ok {
			allowed = false

			if c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), post.ChannelID, model.PermissionReadChannel) {
				allowed = true
			}

			channelReadPermission[post.ChannelID] = allowed
		}

		if !allowed {
			continue
		}

		pl.AddPost(post)
		pl.AddOrder(post.ID)
	}

	pl.SortByCreateAt()
	w.Write([]byte(c.App.PreparePostListForClient(pl).ToJSON()))
}

func getPost(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostID()
	if c.Err != nil {
		return
	}

	post, err := c.App.GetSinglePost(c.Params.PostID)
	if err != nil {
		c.Err = err
		return
	}

	channel, err := c.App.GetChannel(post.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionReadChannel) {
		if channel.Type == model.ChannelTypeOpen {
			if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionReadPublicChannel) {
				c.SetPermissionError(model.PermissionReadPublicChannel)
				return
			}
		} else {
			c.SetPermissionError(model.PermissionReadChannel)
			return
		}
	}

	post = c.App.PreparePostForClient(post, false, false)

	if c.HandleEtag(post.Etag(), "Get Post", w, r) {
		return
	}

	w.Header().Set(model.HeaderEtagServer, post.Etag())
	w.Write([]byte(post.ToJSON()))
}

func deletePost(c *Context, w http.ResponseWriter, _ *http.Request) {
	c.RequirePostID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("deletePost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	auditRec.AddMeta("post_id", c.Params.PostID)

	post, err := c.App.GetSinglePost(c.Params.PostID)
	if err != nil {
		c.SetPermissionError(model.PermissionDeletePost)
		return
	}
	auditRec.AddMeta("post", post)

	if c.AppContext.Session().UserID == post.UserID {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), post.ChannelID, model.PermissionDeletePost) {
			c.SetPermissionError(model.PermissionDeletePost)
			return
		}
	} else {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), post.ChannelID, model.PermissionDeleteOthersPosts) {
			c.SetPermissionError(model.PermissionDeleteOthersPosts)
			return
		}
	}

	if _, err := c.App.DeletePost(c.Params.PostID, c.AppContext.Session().UserID); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func getPostThread(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostID()
	if c.Err != nil {
		return
	}
	skipFetchThreads := r.URL.Query().Get("skipFetchThreads") == "true"
	collapsedThreads := r.URL.Query().Get("collapsedThreads") == "true"
	collapsedThreadsExtended := r.URL.Query().Get("collapsedThreadsExtended") == "true"
	list, err := c.App.GetPostThread(c.Params.PostID, skipFetchThreads, collapsedThreads, collapsedThreadsExtended, c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	post, ok := list.Posts[c.Params.PostID]
	if !ok {
		c.SetInvalidURLParam("post_id")
		return
	}

	channel, err := c.App.GetChannel(post.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.ID, model.PermissionReadChannel) {
		if channel.Type == model.ChannelTypeOpen {
			if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamID, model.PermissionReadPublicChannel) {
				c.SetPermissionError(model.PermissionReadPublicChannel)
				return
			}
		} else {
			c.SetPermissionError(model.PermissionReadChannel)
			return
		}
	}

	if c.HandleEtag(list.Etag(), "Get Post Thread", w, r) {
		return
	}

	clientPostList := c.App.PreparePostListForClient(list)

	w.Header().Set(model.HeaderEtagServer, clientPostList.Etag())

	w.Write([]byte(clientPostList.ToJSON()))
}

func searchPosts(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamID, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	params, jsonErr := model.SearchParameterFromJSON(r.Body)
	if jsonErr != nil {
		c.Err = model.NewAppError("searchPosts", "api.post.search_posts.invalid_body.app_error", nil, jsonErr.Error(), http.StatusBadRequest)
		return
	}

	if params.Terms == nil || *params.Terms == "" {
		c.SetInvalidParam("terms")
		return
	}
	terms := *params.Terms

	timeZoneOffset := 0
	if params.TimeZoneOffset != nil {
		timeZoneOffset = *params.TimeZoneOffset
	}

	isOrSearch := false
	if params.IsOrSearch != nil {
		isOrSearch = *params.IsOrSearch
	}

	page := 0
	if params.Page != nil {
		page = *params.Page
	}

	perPage := 60
	if params.PerPage != nil {
		perPage = *params.PerPage
	}

	includeDeletedChannels := false
	if params.IncludeDeletedChannels != nil {
		includeDeletedChannels = *params.IncludeDeletedChannels
	}

	startTime := time.Now()

	results, err := c.App.SearchPostsInTeamForUser(c.AppContext, terms, c.AppContext.Session().UserID, c.Params.TeamID, isOrSearch, includeDeletedChannels, timeZoneOffset, page, perPage)

	elapsedTime := float64(time.Since(startTime)) / float64(time.Second)
	metrics := c.App.Metrics()
	if metrics != nil {
		metrics.IncrementPostsSearchCounter()
		metrics.ObservePostsSearchDuration(elapsedTime)
	}

	if err != nil {
		c.Err = err
		return
	}

	clientPostList := c.App.PreparePostListForClient(results.PostList)

	results = model.MakePostSearchResults(clientPostList, results.Matches)

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write([]byte(results.ToJSON()))
}

func updatePost(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostID()
	if c.Err != nil {
		return
	}

	post := model.PostFromJSON(r.Body)

	if post == nil {
		c.SetInvalidParam("post")
		return
	}

	auditRec := c.MakeAuditRecord("updatePost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)

	// The post being updated in the payload must be the same one as indicated in the URL.
	if post.ID != c.Params.PostID {
		c.SetInvalidParam("id")
		return
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostID, model.PermissionEditPost) {
		c.SetPermissionError(model.PermissionEditPost)
		return
	}

	originalPost, err := c.App.GetSinglePost(c.Params.PostID)
	if err != nil {
		c.SetPermissionError(model.PermissionEditPost)
		return
	}
	auditRec.AddMeta("post", originalPost)

	// Updating the file_ids of a post is not a supported operation and will be ignored
	post.FileIDs = originalPost.FileIDs

	if c.AppContext.Session().UserID != originalPost.UserID {
		if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostID, model.PermissionEditOthersPosts) {
			c.SetPermissionError(model.PermissionEditOthersPosts)
			return
		}
	}

	post.ID = c.Params.PostID

	rpost, err := c.App.UpdatePost(c.AppContext, c.App.PostWithProxyRemovedFromImageURLs(post), false)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("update", rpost)

	w.Write([]byte(rpost.ToJSON()))
}

func patchPost(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostID()
	if c.Err != nil {
		return
	}

	post := model.PostPatchFromJSON(r.Body)

	if post == nil {
		c.SetInvalidParam("post")
		return
	}

	auditRec := c.MakeAuditRecord("patchPost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)

	// Updating the file_ids of a post is not a supported operation and will be ignored
	post.FileIDs = nil

	originalPost, err := c.App.GetSinglePost(c.Params.PostID)
	if err != nil {
		c.SetPermissionError(model.PermissionEditPost)
		return
	}
	auditRec.AddMeta("post", originalPost)

	var permission *model.Permission
	if c.AppContext.Session().UserID == originalPost.UserID {
		permission = model.PermissionEditPost
	} else {
		permission = model.PermissionEditOthersPosts
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostID, permission) {
		c.SetPermissionError(permission)
		return
	}

	patchedPost, err := c.App.PatchPost(c.AppContext, c.Params.PostID, c.App.PostPatchWithProxyRemovedFromImageURLs(post))
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("patch", patchedPost)

	w.Write([]byte(patchedPost.ToJSON()))
}

func setPostUnread(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostID().RequireUserID()
	if c.Err != nil {
		return
	}

	props := model.MapBoolFromJSON(r.Body)
	collapsedThreadsSupported := props["collapsed_threads_supported"]

	if c.AppContext.Session().UserID != c.Params.UserID && !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserID) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}
	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	state, err := c.App.MarkChannelAsUnreadFromPost(c.Params.PostID, c.Params.UserID, collapsedThreadsSupported, false)
	if err != nil {
		c.Err = err
		return
	}
	w.Write([]byte(state.ToJSON()))
}

func saveIsPinnedPost(c *Context, w http.ResponseWriter, isPinned bool) {
	c.RequirePostID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("saveIsPinnedPost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	// Restrict pinning if the experimental read-only-town-square setting is on.
	user, err := c.App.GetUser(c.AppContext.Session().UserID)
	if err != nil {
		c.Err = err
		return
	}

	post, err := c.App.GetSinglePost(c.Params.PostID)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("post", post)

	channel, err := c.App.GetChannel(post.ChannelID)
	if err != nil {
		c.Err = err
		return
	}

	if c.App.Srv().License() != nil &&
		*c.App.Config().TeamSettings.ExperimentalTownSquareIsReadOnly &&
		channel.Name == model.DefaultChannelName &&
		!c.App.RolesGrantPermission(user.GetRoles(), model.PermissionManageSystem.ID) {
		c.Err = model.NewAppError("saveIsPinnedPost", "api.post.save_is_pinned_post.town_square_read_only", nil, "", http.StatusForbidden)
		return
	}

	patch := &model.PostPatch{}
	patch.IsPinned = model.NewBool(isPinned)

	patchedPost, err := c.App.PatchPost(c.AppContext, c.Params.PostID, patch)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("patch", patchedPost)

	auditRec.Success()
	ReturnStatusOK(w)
}

func pinPost(c *Context, w http.ResponseWriter, _ *http.Request) {
	saveIsPinnedPost(c, w, true)
}

func unpinPost(c *Context, w http.ResponseWriter, _ *http.Request) {
	saveIsPinnedPost(c, w, false)
}

func getFileInfosForPost(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostID()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostID, model.PermissionReadChannel) {
		c.SetPermissionError(model.PermissionReadChannel)
		return
	}

	infos, err := c.App.GetFileInfosForPostWithMigration(c.Params.PostID)
	if err != nil {
		c.Err = err
		return
	}

	if c.HandleEtag(model.GetEtagForFileInfos(infos), "Get File Infos For Post", w, r) {
		return
	}

	w.Header().Set("Cache-Control", "max-age=2592000, private")
	w.Header().Set(model.HeaderEtagServer, model.GetEtagForFileInfos(infos))
	w.Write([]byte(model.FileInfosToJSON(infos)))
}
