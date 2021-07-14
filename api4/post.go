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
	api.BaseRoutes.Posts.Handle("", api.ApiSessionRequired(createPost)).Methods("POST")
	api.BaseRoutes.Post.Handle("", api.ApiSessionRequired(getPost)).Methods("GET")
	api.BaseRoutes.Posts.Handle("/ids", api.ApiSessionRequired(getPosts)).Methods("POST")
	api.BaseRoutes.Post.Handle("", api.ApiSessionRequired(deletePost)).Methods("DELETE")
	api.BaseRoutes.Posts.Handle("/ephemeral", api.ApiSessionRequired(createEphemeralPost)).Methods("POST")
	api.BaseRoutes.Post.Handle("/thread", api.ApiSessionRequired(getPostThread)).Methods("GET")
	api.BaseRoutes.Post.Handle("/files/info", api.ApiSessionRequired(getFileInfosForPost)).Methods("GET")
	api.BaseRoutes.PostsForChannel.Handle("", api.ApiSessionRequired(getPostsForChannel)).Methods("GET")
	api.BaseRoutes.PostsForUser.Handle("/flagged", api.ApiSessionRequired(getFlaggedPostsForUser)).Methods("GET")

	api.BaseRoutes.ChannelForUser.Handle("/posts/unread", api.ApiSessionRequired(getPostsForChannelAroundLastUnread)).Methods("GET")

	api.BaseRoutes.Team.Handle("/posts/search", api.ApiSessionRequiredDisableWhenBusy(searchPosts)).Methods("POST")
	api.BaseRoutes.Post.Handle("", api.ApiSessionRequired(updatePost)).Methods("PUT")
	api.BaseRoutes.Post.Handle("/patch", api.ApiSessionRequired(patchPost)).Methods("PUT")
	api.BaseRoutes.PostForUser.Handle("/set_unread", api.ApiSessionRequired(setPostUnread)).Methods("POST")
	api.BaseRoutes.Post.Handle("/pin", api.ApiSessionRequired(pinPost)).Methods("POST")
	api.BaseRoutes.Post.Handle("/unpin", api.ApiSessionRequired(unpinPost)).Methods("POST")
}

func createPost(c *Context, w http.ResponseWriter, r *http.Request) {
	post := model.PostFromJson(r.Body)
	if post == nil {
		c.SetInvalidParam("post")
		return
	}

	post.UserId = c.AppContext.Session().UserId

	auditRec := c.MakeAuditRecord("createPost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	auditRec.AddMeta("post", post)

	hasPermission := false
	if c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), post.ChannelId, model.PERMISSION_CREATE_POST) {
		hasPermission = true
	} else if channel, err := c.App.GetChannel(post.ChannelId); err == nil {
		// Temporary permission check method until advanced permissions, please do not copy
		if channel.Type == model.CHANNEL_OPEN && c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamId, model.PERMISSION_CREATE_POST_PUBLIC) {
			hasPermission = true
		}
	}

	if !hasPermission {
		c.SetPermissionError(model.PERMISSION_CREATE_POST)
		return
	}

	if post.CreateAt != 0 && !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_MANAGE_SYSTEM) {
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

	rp, err := c.App.CreatePostAsUser(c.AppContext, c.App.PostWithProxyRemovedFromImageURLs(post), c.AppContext.Session().Id, setOnlineBool)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.Success()
	auditRec.AddMeta("post", rp) // overwrite meta

	if setOnlineBool {
		c.App.SetStatusOnline(c.AppContext.Session().UserId, false)
	}

	c.App.UpdateLastActivityAtIfNeeded(*c.AppContext.Session())
	c.ExtendSessionExpiryIfNeeded(w, r)

	w.WriteHeader(http.StatusCreated)

	// Note that rp has already had PreparePostForClient called on it by App.CreatePost
	w.Write([]byte(rp.ToJson()))
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

	ephRequest.Post.UserId = c.AppContext.Session().UserId
	ephRequest.Post.CreateAt = model.GetMillis()

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PERMISSION_CREATE_POST_EPHEMERAL) {
		c.SetPermissionError(model.PERMISSION_CREATE_POST_EPHEMERAL)
		return
	}

	rp := c.App.SendEphemeralPost(ephRequest.UserID, c.App.PostWithProxyRemovedFromImageURLs(ephRequest.Post))

	w.WriteHeader(http.StatusCreated)
	rp = model.AddPostActionCookies(rp, c.App.PostActionCookieSecret())
	rp = c.App.PreparePostForClient(rp, true, false, c.AppContext.Session().UserId)
	w.Write([]byte(rp.ToJson()))
}

func getPostsForChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelId()
	if c.Err != nil {
		return
	}

	afterPost := r.URL.Query().Get("after")
	if afterPost != "" && !model.IsValidId(afterPost) {
		c.SetInvalidParam("after")
		return
	}

	beforePost := r.URL.Query().Get("before")
	if beforePost != "" && !model.IsValidId(beforePost) {
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
	channelId := c.Params.ChannelId
	page := c.Params.Page
	perPage := c.Params.PerPage

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channelId, model.PERMISSION_READ_CHANNEL) {
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	var list *model.PostList
	var err *model.AppError
	etag := ""

	if since > 0 {
		list, err = c.App.GetPostsSince(model.GetPostsSinceOptions{ChannelId: channelId, Time: since, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, CollapsedThreadsExtended: collapsedThreadsExtended, UserId: c.AppContext.Session().UserId})
	} else if afterPost != "" {
		etag = c.App.GetPostsEtag(channelId, collapsedThreads)

		if c.HandleEtag(etag, "Get Posts After", w, r) {
			return
		}

		list, err = c.App.GetPostsAfterPost(model.GetPostsOptions{ChannelId: channelId, PostId: afterPost, Page: page, PerPage: perPage, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, UserId: c.AppContext.Session().UserId})
	} else if beforePost != "" {
		etag = c.App.GetPostsEtag(channelId, collapsedThreads)

		if c.HandleEtag(etag, "Get Posts Before", w, r) {
			return
		}

		list, err = c.App.GetPostsBeforePost(model.GetPostsOptions{ChannelId: channelId, PostId: beforePost, Page: page, PerPage: perPage, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, CollapsedThreadsExtended: collapsedThreadsExtended, UserId: c.AppContext.Session().UserId})
	} else {
		etag = c.App.GetPostsEtag(channelId, collapsedThreads)

		if c.HandleEtag(etag, "Get Posts", w, r) {
			return
		}

		list, err = c.App.GetPostsPage(model.GetPostsOptions{ChannelId: channelId, Page: page, PerPage: perPage, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, CollapsedThreadsExtended: collapsedThreadsExtended, UserId: c.AppContext.Session().UserId})
	}

	if err != nil {
		c.Err = err
		return
	}

	if etag != "" {
		w.Header().Set(model.HEADER_ETAG_SERVER, etag)
	}

	c.App.AddCursorIdsForPostList(list, afterPost, beforePost, since, page, perPage, collapsedThreads)
	clientPostList := c.App.PreparePostListForClient(list, c.AppContext.Session().UserId)

	w.Write([]byte(clientPostList.ToJson()))
}

func getPostsForChannelAroundLastUnread(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId().RequireChannelId()
	if c.Err != nil {
		return
	}

	userId := c.Params.UserId
	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), userId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	channelId := c.Params.ChannelId
	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channelId, model.PERMISSION_READ_CHANNEL) {
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	if c.Params.LimitAfter == 0 {
		c.SetInvalidUrlParam("limit_after")
		return
	}

	skipFetchThreads := r.URL.Query().Get("skipFetchThreads") == "true"
	collapsedThreads := r.URL.Query().Get("collapsedThreads") == "true"
	collapsedThreadsExtended := r.URL.Query().Get("collapsedThreadsExtended") == "true"

	postList, err := c.App.GetPostsForChannelAroundLastUnread(channelId, userId, c.Params.LimitBefore, c.Params.LimitAfter, skipFetchThreads, collapsedThreads, collapsedThreadsExtended)
	if err != nil {
		c.Err = err
		return
	}

	etag := ""
	if len(postList.Order) == 0 {
		etag = c.App.GetPostsEtag(channelId, collapsedThreads)

		if c.HandleEtag(etag, "Get Posts", w, r) {
			return
		}

		postList, err = c.App.GetPostsPage(model.GetPostsOptions{ChannelId: channelId, Page: app.PageDefault, PerPage: c.Params.LimitBefore, SkipFetchThreads: skipFetchThreads, CollapsedThreads: collapsedThreads, CollapsedThreadsExtended: collapsedThreadsExtended, UserId: c.AppContext.Session().UserId})
		if err != nil {
			c.Err = err
			return
		}
	}

	postList.NextPostId = c.App.GetNextPostIdFromPostList(postList, collapsedThreads)
	postList.PrevPostId = c.App.GetPrevPostIdFromPostList(postList, collapsedThreads)

	clientPostList := c.App.PreparePostListForClient(postList, c.AppContext.Session().UserId)

	if etag != "" {
		w.Header().Set(model.HEADER_ETAG_SERVER, etag)
	}
	w.Write([]byte(clientPostList.ToJson()))
}

func getFlaggedPostsForUser(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}

	channelId := r.URL.Query().Get("channel_id")
	teamId := r.URL.Query().Get("team_id")

	var posts *model.PostList
	var err *model.AppError

	if channelId != "" {
		posts, err = c.App.GetFlaggedPostsForChannel(c.Params.UserId, channelId, c.Params.Page, c.Params.PerPage)
	} else if teamId != "" {
		posts, err = c.App.GetFlaggedPostsForTeam(c.Params.UserId, teamId, c.Params.Page, c.Params.PerPage)
	} else {
		posts, err = c.App.GetFlaggedPosts(c.Params.UserId, c.Params.Page, c.Params.PerPage)
	}
	if err != nil {
		c.Err = err
		return
	}

	pl := model.NewPostList()
	channelReadPermission := make(map[string]bool)

	for _, post := range posts.Posts {
		allowed, ok := channelReadPermission[post.ChannelId]

		if !ok {
			allowed = false

			if c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), post.ChannelId, model.PERMISSION_READ_CHANNEL) {
				allowed = true
			}

			channelReadPermission[post.ChannelId] = allowed
		}

		if !allowed {
			continue
		}

		pl.AddPost(post)
		pl.AddOrder(post.Id)
	}

	pl.SortByCreateAt()
	w.Write([]byte(c.App.PreparePostListForClient(pl, c.AppContext.Session().UserId).ToJson()))
}

func getPost(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	post, err := c.App.GetSinglePost(c.Params.PostId)
	if err != nil {
		c.Err = err
		return
	}

	channel, err := c.App.GetChannel(post.ChannelId)
	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.Id, model.PERMISSION_READ_CHANNEL) {
		if channel.Type == model.CHANNEL_OPEN {
			if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamId, model.PERMISSION_READ_PUBLIC_CHANNEL) {
				c.SetPermissionError(model.PERMISSION_READ_PUBLIC_CHANNEL)
				return
			}
		} else {
			c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
			return
		}
	}

	post = c.App.PreparePostForClient(post, false, false, c.AppContext.Session().UserId)

	if c.HandleEtag(post.Etag(), "Get Post", w, r) {
		return
	}

	w.Header().Set(model.HEADER_ETAG_SERVER, post.Etag())
	w.Write([]byte(post.ToJson()))
}

func getPosts(c *Context, w http.ResponseWriter, r *http.Request) {
	var postIDs []string
	json.NewDecoder(r.Body).Decode(&postIDs)
	if postIDs == nil {
		c.SetInvalidParam("post_ids")
		return
	}

	var posts []*model.Post

	for _, postID := range postIDs {
		post, err := c.App.GetSinglePost(postID)
		if err != nil {
			if err.StatusCode == http.StatusNotFound {
				continue
			} else {
				c.Err = err
				return
			}
		}

		channel, err := c.App.GetChannel(post.ChannelId)
		if err != nil {
			c.Err = err
			return
		}

		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.Id, model.PERMISSION_READ_CHANNEL) {
			if channel.Type == model.CHANNEL_OPEN {
				if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamId, model.PERMISSION_READ_PUBLIC_CHANNEL) {
					continue
				}
			} else {
				continue
			}
		}

		post = c.App.PreparePostForClient(post, false, false, c.AppContext.Session().UserId)

		posts = append(posts, post)
	}

	b, err := json.Marshal(posts)
	if err != nil {
		c.Err = model.NewAppError("getPosts", "api.post.search_posts.invalid_body.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(b)
}

func deletePost(c *Context, w http.ResponseWriter, _ *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("deletePost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	auditRec.AddMeta("post_id", c.Params.PostId)

	post, err := c.App.GetSinglePost(c.Params.PostId)
	if err != nil {
		c.SetPermissionError(model.PERMISSION_DELETE_POST)
		return
	}
	auditRec.AddMeta("post", post)

	if c.AppContext.Session().UserId == post.UserId {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), post.ChannelId, model.PERMISSION_DELETE_POST) {
			c.SetPermissionError(model.PERMISSION_DELETE_POST)
			return
		}
	} else {
		if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), post.ChannelId, model.PERMISSION_DELETE_OTHERS_POSTS) {
			c.SetPermissionError(model.PERMISSION_DELETE_OTHERS_POSTS)
			return
		}
	}

	if _, err := c.App.DeletePost(c.Params.PostId, c.AppContext.Session().UserId); err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

func getPostThread(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}
	skipFetchThreads := r.URL.Query().Get("skipFetchThreads") == "true"
	collapsedThreads := r.URL.Query().Get("collapsedThreads") == "true"
	collapsedThreadsExtended := r.URL.Query().Get("collapsedThreadsExtended") == "true"
	list, err := c.App.GetPostThread(c.Params.PostId, skipFetchThreads, collapsedThreads, collapsedThreadsExtended, c.AppContext.Session().UserId)
	if err != nil {
		c.Err = err
		return
	}

	post, ok := list.Posts[c.Params.PostId]
	if !ok {
		c.SetInvalidUrlParam("post_id")
		return
	}

	channel, err := c.App.GetChannel(post.ChannelId)
	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionToChannel(*c.AppContext.Session(), channel.Id, model.PERMISSION_READ_CHANNEL) {
		if channel.Type == model.CHANNEL_OPEN {
			if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamId, model.PERMISSION_READ_PUBLIC_CHANNEL) {
				c.SetPermissionError(model.PERMISSION_READ_PUBLIC_CHANNEL)
				return
			}
		} else {
			c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
			return
		}
	}

	if c.HandleEtag(list.Etag(), "Get Post Thread", w, r) {
		return
	}

	clientPostList := c.App.PreparePostListForClient(list, c.AppContext.Session().UserId)

	w.Header().Set(model.HEADER_ETAG_SERVER, clientPostList.Etag())

	w.Write([]byte(clientPostList.ToJson()))
}

func searchPosts(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PERMISSION_VIEW_TEAM) {
		c.SetPermissionError(model.PERMISSION_VIEW_TEAM)
		return
	}

	params, jsonErr := model.SearchParameterFromJson(r.Body)
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

	results, err := c.App.SearchPostsInTeamForUser(c.AppContext, terms, c.AppContext.Session().UserId, c.Params.TeamId, isOrSearch, includeDeletedChannels, timeZoneOffset, page, perPage)

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

	clientPostList := c.App.PreparePostListForClient(results.PostList, c.AppContext.Session().UserId)

	results = model.MakePostSearchResults(clientPostList, results.Matches)

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write([]byte(results.ToJson()))
}

func updatePost(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	post := model.PostFromJson(r.Body)

	if post == nil {
		c.SetInvalidParam("post")
		return
	}

	auditRec := c.MakeAuditRecord("updatePost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)

	// The post being updated in the payload must be the same one as indicated in the URL.
	if post.Id != c.Params.PostId {
		c.SetInvalidParam("id")
		return
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostId, model.PERMISSION_EDIT_POST) {
		c.SetPermissionError(model.PERMISSION_EDIT_POST)
		return
	}

	originalPost, err := c.App.GetSinglePost(c.Params.PostId)
	if err != nil {
		c.SetPermissionError(model.PERMISSION_EDIT_POST)
		return
	}
	auditRec.AddMeta("post", originalPost)

	// Updating the file_ids of a post is not a supported operation and will be ignored
	post.FileIds = originalPost.FileIds

	if c.AppContext.Session().UserId != originalPost.UserId {
		if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostId, model.PERMISSION_EDIT_OTHERS_POSTS) {
			c.SetPermissionError(model.PERMISSION_EDIT_OTHERS_POSTS)
			return
		}
	}

	post.Id = c.Params.PostId

	rpost, err := c.App.UpdatePost(c.AppContext, c.App.PostWithProxyRemovedFromImageURLs(post), false)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("update", rpost)

	w.Write([]byte(rpost.ToJson()))
}

func patchPost(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	post := model.PostPatchFromJson(r.Body)

	if post == nil {
		c.SetInvalidParam("post")
		return
	}

	auditRec := c.MakeAuditRecord("patchPost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)

	// Updating the file_ids of a post is not a supported operation and will be ignored
	post.FileIds = nil

	originalPost, err := c.App.GetSinglePost(c.Params.PostId)
	if err != nil {
		c.SetPermissionError(model.PERMISSION_EDIT_POST)
		return
	}
	auditRec.AddMeta("post", originalPost)

	var permission *model.Permission
	if c.AppContext.Session().UserId == originalPost.UserId {
		permission = model.PERMISSION_EDIT_POST
	} else {
		permission = model.PERMISSION_EDIT_OTHERS_POSTS
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostId, permission) {
		c.SetPermissionError(permission)
		return
	}

	patchedPost, err := c.App.PatchPost(c.AppContext, c.Params.PostId, c.App.PostPatchWithProxyRemovedFromImageURLs(post))
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddMeta("patch", patchedPost)

	w.Write([]byte(patchedPost.ToJson()))
}

func setPostUnread(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId().RequireUserId()
	if c.Err != nil {
		return
	}

	props := model.MapBoolFromJson(r.Body)
	collapsedThreadsSupported := props["collapsed_threads_supported"]

	if c.AppContext.Session().UserId != c.Params.UserId && !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PERMISSION_EDIT_OTHER_USERS)
		return
	}
	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostId, model.PERMISSION_READ_CHANNEL) {
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	state, err := c.App.MarkChannelAsUnreadFromPost(c.Params.PostId, c.Params.UserId, collapsedThreadsSupported, false)
	if err != nil {
		c.Err = err
		return
	}
	w.Write([]byte(state.ToJson()))
}

func saveIsPinnedPost(c *Context, w http.ResponseWriter, isPinned bool) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("saveIsPinnedPost", audit.Fail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostId, model.PERMISSION_READ_CHANNEL) {
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	// Restrict pinning if the experimental read-only-town-square setting is on.
	user, err := c.App.GetUser(c.AppContext.Session().UserId)
	if err != nil {
		c.Err = err
		return
	}

	post, err := c.App.GetSinglePost(c.Params.PostId)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddMeta("post", post)

	channel, err := c.App.GetChannel(post.ChannelId)
	if err != nil {
		c.Err = err
		return
	}

	if c.App.Srv().License() != nil &&
		*c.App.Config().TeamSettings.ExperimentalTownSquareIsReadOnly &&
		channel.Name == model.DEFAULT_CHANNEL &&
		!c.App.RolesGrantPermission(user.GetRoles(), model.PERMISSION_MANAGE_SYSTEM.Id) {
		c.Err = model.NewAppError("saveIsPinnedPost", "api.post.save_is_pinned_post.town_square_read_only", nil, "", http.StatusForbidden)
		return
	}

	patch := &model.PostPatch{}
	patch.IsPinned = model.NewBool(isPinned)

	patchedPost, err := c.App.PatchPost(c.AppContext, c.Params.PostId, patch)
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
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), c.Params.PostId, model.PERMISSION_READ_CHANNEL) {
		c.SetPermissionError(model.PERMISSION_READ_CHANNEL)
		return
	}

	infos, err := c.App.GetFileInfosForPostWithMigration(c.Params.PostId)
	if err != nil {
		c.Err = err
		return
	}

	if c.HandleEtag(model.GetEtagForFileInfos(infos), "Get File Infos For Post", w, r) {
		return
	}

	w.Header().Set("Cache-Control", "max-age=2592000, private")
	w.Header().Set(model.HEADER_ETAG_SERVER, model.GetEtagForFileInfos(infos))
	w.Write([]byte(model.FileInfosToJson(infos)))
}
