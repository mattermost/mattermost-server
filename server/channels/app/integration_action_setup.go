// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"maps"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// maxMmBlocksActionsCloneDepth caps recursion in cloneMmBlocksActionsProp.
// ValidateMmBlocksActions bounds top-level entry count and key length but
// does not bound nesting depth inside spec.Context — a bot/plugin could
// otherwise stash a pathologically nested object that drives stack
// exhaustion on the restore path. 64 is well past any plausible legitimate
// nesting; deeper input is treated as malicious and truncated.
const maxMmBlocksActionsCloneDepth = 64

// cloneMmBlocksActionsProp deep-clones the post.props.mm_blocks_actions value.
// Each per-action entry can carry nested context / query maps (and arrays
// inside those), so the clone walks the structure recursively — a shallow
// clone at any level would leave nested objects aliased back to the live
// post's props, defeating the restore-after-invalid-response guarantee.
func cloneMmBlocksActionsProp(v any) any {
	return cloneMmBlocksActionsPropAt(v, 0)
}

func cloneMmBlocksActionsPropAt(v any, depth int) any {
	if depth > maxMmBlocksActionsCloneDepth {
		// Defense-in-depth: drop the subtree rather than risk stack
		// exhaustion. The restore path that calls this helper is on a
		// rare branch (plugin response is invalid), and pathological
		// nesting at this depth is not a legitimate use case.
		return nil
	}
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, child := range typed {
			out[k] = cloneMmBlocksActionsPropAt(child, depth+1)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = cloneMmBlocksActionsPropAt(child, depth+1)
		}
		return out
	default:
		// Scalars (string/number/bool/nil) are immutable — safe to share.
		return v
	}
}

// postActionSetup holds resolved state for one interactive post action before the upstream HTTP call.
type postActionSetup struct {
	upstreamURL          string
	upstreamRequest      *model.PostActionIntegrationRequest
	datasource           string
	retain               map[string]any
	remove               []string
	originalProps        map[string]any
	originalIsPinned     bool
	originalHasReactions bool
	rootPostId           string
	ephemeralInteractive bool
	ephemeralChannelID   string
	ephemeralRootPostID  string
	ephemeralSetUserID   bool
	// post is set when the action was read from a stored post.
	post *model.Post
}

func (a *App) resolvePostActionSetup(
	rctx request.CTX,
	postID, actionID, userID string,
	legacyCookie *model.PostActionCookie,
	mmBlocksCookie *model.MmBlocksActionCookie,
	clientQuery map[string]string,
	integrationFormat string,
) (*postActionSetup, string, *model.AppError) {
	upstreamRequest := &model.PostActionIntegrationRequest{
		UserId: userID,
		PostId: postID,
	}

	userChan := a.startUpstreamUserFetch(rctx, userID)
	postChan := a.startUpstreamPostFetch(rctx, postID)
	channelChan := a.startUpstreamChannelFetch(postID)

	postResult := <-postChan
	if postResult.NErr != nil {
		setup, gotoURL, appErr := a.resolvePostActionSetupFromCookies(postID, actionID, legacyCookie, mmBlocksCookie, clientQuery, upstreamRequest, postResult)
		return a.finishPostActionSetup(setup, gotoURL, appErr, userChan)
	}

	post := postResult.Data
	chResult := <-channelChan
	if chResult.NErr != nil {
		return nil, "", model.NewAppError("DoPostActionWithCookie", "app.channel.get_for_post.app_error", nil, "", http.StatusInternalServerError).Wrap(chResult.NErr)
	}

	setup, gotoURL, appErr := a.resolvePostActionSetupFromPost(post, chResult.Data, actionID, clientQuery, integrationFormat, upstreamRequest)
	return a.finishPostActionSetup(setup, gotoURL, appErr, userChan)
}

func (a *App) startUpstreamPostFetch(rctx request.CTX, postID string) <-chan store.StoreResult[*model.Post] {
	postChan := make(chan store.StoreResult[*model.Post], 1)
	go func() {
		post, err := a.Srv().Store().Post().GetSingle(rctx, postID, false)
		postChan <- store.StoreResult[*model.Post]{Data: post, NErr: err}
		close(postChan)
	}()
	return postChan
}

func (a *App) startUpstreamChannelFetch(postID string) <-chan store.StoreResult[*model.Channel] {
	channelChan := make(chan store.StoreResult[*model.Channel], 1)
	go func() {
		channel, err := a.Srv().Store().Channel().GetForPost(postID)
		channelChan <- store.StoreResult[*model.Channel]{Data: channel, NErr: err}
		close(channelChan)
	}()
	return channelChan
}

func (a *App) startUpstreamUserFetch(rctx request.CTX, userID string) <-chan store.StoreResult[*model.User] {
	userChan := make(chan store.StoreResult[*model.User], 1)
	go func() {
		user, err := a.Srv().Store().User().Get(rctx, userID)
		userChan <- store.StoreResult[*model.User]{Data: user, NErr: err}
		close(userChan)
	}()
	return userChan
}

func (a *App) startUpstreamTeamFetch(teamID string) <-chan store.StoreResult[*model.Team] {
	if teamID == "" {
		return nil
	}

	teamChan := make(chan store.StoreResult[*model.Team], 1)
	go func() {
		defer close(teamChan)

		team, err := a.Srv().Store().Team().Get(teamID)
		teamChan <- store.StoreResult[*model.Team]{Data: team, NErr: err}
	}()
	return teamChan
}

func (a *App) populateUpstreamRequestIdentity(
	upstreamRequest *model.PostActionIntegrationRequest,
	userChan <-chan store.StoreResult[*model.User],
) *model.AppError {
	teamChan := a.startUpstreamTeamFetch(upstreamRequest.TeamId)

	ur := <-userChan
	if ur.NErr != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(ur.NErr, &nfErr):
			return model.NewAppError("DoPostActionWithCookie", MissingAccountError, nil, "", http.StatusNotFound).Wrap(ur.NErr)
		default:
			return model.NewAppError("DoPostActionWithCookie", "app.user.get.app_error", nil, "", http.StatusInternalServerError).Wrap(ur.NErr)
		}
	}
	upstreamRequest.UserName = ur.Data.Username

	if teamChan != nil {
		if tr, ok := <-teamChan; ok {
			if tr.NErr != nil {
				var nfErr *store.ErrNotFound
				switch {
				case errors.As(tr.NErr, &nfErr):
					return model.NewAppError("DoPostActionWithCookie", "app.team.get.find.app_error", nil, "", http.StatusNotFound).Wrap(tr.NErr)
				default:
					return model.NewAppError("DoPostActionWithCookie", "app.team.get.finding.app_error", nil, "", http.StatusInternalServerError).Wrap(tr.NErr)
				}
			}
			upstreamRequest.TeamName = tr.Data.Name
		}
	}

	return nil
}

func (a *App) finishPostActionSetup(
	setup *postActionSetup,
	gotoURL string,
	appErr *model.AppError,
	userChan <-chan store.StoreResult[*model.User],
) (*postActionSetup, string, *model.AppError) {
	if appErr != nil {
		return nil, "", appErr
	}
	if gotoURL != "" {
		return nil, gotoURL, nil
	}
	if appErr := a.populateUpstreamRequestIdentity(setup.upstreamRequest, userChan); appErr != nil {
		return nil, "", appErr
	}
	return setup, "", nil
}

func (a *App) resolvePostActionSetupFromCookies(
	postID, actionID string,
	legacyCookie *model.PostActionCookie,
	mmBlocksCookie *model.MmBlocksActionCookie,
	clientQuery map[string]string,
	upstreamRequest *model.PostActionIntegrationRequest,
	postResult store.StoreResult[*model.Post],
) (*postActionSetup, string, *model.AppError) {
	if legacyCookie == nil && mmBlocksCookie == nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(postResult.NErr, &nfErr):
			return nil, "", model.NewAppError("DoPostActionWithCookie", "app.post.get.app_error", nil, "", http.StatusNotFound).Wrap(postResult.NErr)
		default:
			return nil, "", model.NewAppError("DoPostActionWithCookie", "app.post.get.app_error", nil, "", http.StatusInternalServerError).Wrap(postResult.NErr)
		}
	}
	if legacyCookie != nil && mmBlocksCookie != nil {
		return nil, "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "multiple action cookies set", http.StatusBadRequest)
	}

	var nfPost *store.ErrNotFound
	ephemeral := errors.As(postResult.NErr, &nfPost)

	if legacyCookie != nil {
		return a.setupFromLegacyCookie(postID, legacyCookie, upstreamRequest, ephemeral)
	}
	return a.setupFromMmBlocksCookie(postID, actionID, mmBlocksCookie, clientQuery, upstreamRequest, ephemeral)
}

func (a *App) setupFromLegacyCookie(
	postID string,
	cookie *model.PostActionCookie,
	upstreamRequest *model.PostActionIntegrationRequest,
	ephemeral bool,
) (*postActionSetup, string, *model.AppError) {
	if cookie.Integration == nil {
		return nil, "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "no Integration in action cookie", http.StatusBadRequest)
	}
	if postID != cookie.PostId {
		return nil, "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "postId doesn't match", http.StatusBadRequest)
	}

	channel, appErr := a.channelForPostAction(cookie.ChannelId)
	if appErr != nil {
		return nil, "", appErr
	}

	fillUpstreamChannel(upstreamRequest, channel, cookie.ChannelId)
	upstreamRequest.Type = cookie.Type
	// Clone the Context map — later code may add selected_option to it, and
	// we must not mutate the shared source.
	upstreamRequest.Context = maps.Clone(cookie.Integration.Context)

	setup := &postActionSetup{
		upstreamURL:          cookie.Integration.URL,
		upstreamRequest:      upstreamRequest,
		datasource:           cookie.DataSource,
		retain:               cookie.RetainProps,
		remove:               cookie.RemoveProps,
		rootPostId:           cookie.RootPostId,
		ephemeralInteractive: ephemeral,
		ephemeralChannelID:   cookie.ChannelId,
		ephemeralRootPostID:  cookie.RootPostId,
		ephemeralSetUserID:   true,
	}
	return setup, "", nil
}

func (a *App) setupFromMmBlocksCookie(
	postID, actionID string,
	cookie *model.MmBlocksActionCookie,
	clientQuery map[string]string,
	upstreamRequest *model.PostActionIntegrationRequest,
	ephemeral bool,
) (*postActionSetup, string, *model.AppError) {
	if cookie.Kind != model.MmBlocksActionCookieKind {
		return nil, "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "invalid mm_blocks action cookie", http.StatusBadRequest)
	}
	if postID != cookie.PostId {
		return nil, "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "postId doesn't match", http.StatusBadRequest)
	}

	channel, appErr := a.channelForPostAction(cookie.ChannelId)
	if appErr != nil {
		return nil, "", appErr
	}

	mmSpec := cookie.ActionSpec(actionID)
	resolved, err := model.ResolveMmBlocksAction(mmSpec, actionID, clientQuery)
	if appErr := mmBlocksResolveAppError(actionID, err); appErr != nil {
		return nil, "", appErr
	}
	if resolved.OpenURLGoto != "" {
		return nil, resolved.OpenURLGoto, nil
	}

	fillUpstreamChannel(upstreamRequest, channel, cookie.ChannelId)
	upstreamRequest.Type = model.PostActionTypeButton
	upstreamRequest.Context = resolved.Context

	setup := &postActionSetup{
		upstreamURL:          resolved.ExternalURL,
		upstreamRequest:      upstreamRequest,
		retain:               cookie.RetainProps,
		remove:               cookie.RemoveProps,
		rootPostId:           cookie.RootPostId,
		ephemeralInteractive: ephemeral,
		ephemeralChannelID:   cookie.ChannelId,
		ephemeralRootPostID:  cookie.RootPostId,
	}
	return setup, "", nil
}

func (a *App) resolvePostActionSetupFromPost(
	post *model.Post,
	channel *model.Channel,
	actionID string,
	clientQuery map[string]string,
	integrationFormat string,
	upstreamRequest *model.PostActionIntegrationRequest,
) (*postActionSetup, string, *model.AppError) {
	fillUpstreamChannel(upstreamRequest, channel, post.ChannelId)

	switch model.NormalizePostActionIntegrationFormat(integrationFormat) {
	case model.PostActionIntegrationFormatMmBlock,
		model.PostActionIntegrationFormatBlock,
		model.PostActionIntegrationFormatCard:
		if !a.Config().FeatureFlags.MmBlocksEnabled {
			return nil, "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "mm_blocks are not enabled", http.StatusBadRequest)
		}
		mmSpec := post.GetMmBlocksActionSpec(actionID)
		resolved, err := model.ResolveMmBlocksAction(mmSpec, actionID, clientQuery)
		if appErr := mmBlocksResolveAppError(actionID, err); appErr != nil {
			return nil, "", appErr
		}
		if resolved.OpenURLGoto != "" {
			return nil, resolved.OpenURLGoto, nil
		}

		preserve := post.PostActionPreserveState()
		upstreamRequest.Type = model.PostActionTypeButton
		upstreamRequest.Context = resolved.Context

		return &postActionSetup{
			upstreamURL:          resolved.ExternalURL,
			upstreamRequest:      upstreamRequest,
			retain:               preserve.Retain,
			remove:               preserve.Remove,
			originalProps:        preserve.OriginalProps,
			originalIsPinned:     preserve.OriginalIsPinned,
			originalHasReactions: preserve.OriginalHasReactions,
			rootPostId:           preserve.RootPostId,
			post:                 post,
		}, "", nil

	case model.PostActionIntegrationFormatAttachment:
		action := post.GetAction(actionID)
		if action == nil || action.Integration == nil {
			return nil, "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_id.app_error", nil, fmt.Sprintf("action=%v", action), http.StatusNotFound)
		}

		preserve := post.PostActionPreserveState()
		upstreamRequest.Type = action.Type
		// Clone the Context map — the action pointer returned from
		// post.GetAction may alias post.props state. Mutating it directly
		// would leak per-click values (selected_option) into the post's
		// cached integration for subsequent clickers.
		upstreamRequest.Context = maps.Clone(action.Integration.Context)

		return &postActionSetup{
			upstreamURL:          action.Integration.URL,
			upstreamRequest:      upstreamRequest,
			datasource:           action.DataSource,
			retain:               preserve.Retain,
			remove:               preserve.Remove,
			originalProps:        preserve.OriginalProps,
			originalIsPinned:     preserve.OriginalIsPinned,
			originalHasReactions: preserve.OriginalHasReactions,
			rootPostId:           preserve.RootPostId,
			post:                 post,
		}, "", nil
	}

	return nil, "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_id.app_error", nil, fmt.Sprintf("integration_context=%s", integrationFormat), http.StatusBadRequest)
}

func fillUpstreamChannel(req *model.PostActionIntegrationRequest, channel *model.Channel, channelID string) {
	req.ChannelId = channelID
	req.ChannelName = channel.Name
	req.TeamId = channel.TeamId
}

func (a *App) channelForPostAction(channelID string) (*model.Channel, *model.AppError) {
	channel, err := a.Srv().Store().Channel().Get(channelID, true)
	if err != nil {
		errCtx := map[string]any{"channel_id": channelID}
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("DoPostActionWithCookie", "app.channel.get.existing.app_error", errCtx, "", http.StatusNotFound).Wrap(err)
		default:
			return nil, model.NewAppError("DoPostActionWithCookie", "app.channel.get.find.app_error", errCtx, "", http.StatusInternalServerError).Wrap(err)
		}
	}
	return channel, nil
}

func mmBlocksResolveAppError(actionID string, err error) *model.AppError {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, model.ErrMmBlocksActionNotFound):
		return model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_id.app_error", nil, fmt.Sprintf("mm_blocks action_id=%s", actionID), http.StatusNotFound)
	default:
		return model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
}

func (a *App) applyPostActionUpdate(
	rctx request.CTX,
	setup *postActionSetup,
	postID, userID string,
	update *model.Post,
) *model.AppError {
	update.Id = postID

	if update.GetProps() == nil {
		switch {
		case setup.originalProps != nil:
			update.SetProps(setup.originalProps)
		case setup.ephemeralInteractive && len(setup.retain) > 0:
			props := make(model.StringInterface)
			maps.Copy(props, setup.retain)
			update.SetProps(props)
		default:
			update.SetProps(setup.originalProps)
		}
	} else {
		for key, value := range setup.retain {
			update.AddProp(key, value)
		}
		for _, key := range setup.remove {
			update.DelProp(key)
		}
	}
	update.IsPinned = setup.originalIsPinned
	update.HasReactions = setup.originalHasReactions

	// Validate mm_blocks_actions on update responses. Since
	// AllowMmBlocksActionsUpdate bypasses the non-integration guard in
	// UpdatePost, and mm_blocks_actions are not in PostActionRetainPropKeys,
	// a bad response would otherwise permanently replace the post's valid
	// mm_blocks_actions. Keep the original value (if any) and log a warning
	// so integration authors can diagnose.
	if update.GetProp(model.PostPropsMmBlocksActions) != nil {
		originalMmBlocks := any(nil)
		if setup.originalProps != nil {
			originalMmBlocks = setup.originalProps[model.PostPropsMmBlocksActions]
		}
		if originalMmBlocks == nil {
			rctx.Logger().Info("Dropping mm_blocks_actions from plugin update response: original post had none",
				mlog.String("post_id", postID),
				mlog.String("url", setup.upstreamURL),
			)
			update.DelProp(model.PostPropsMmBlocksActions)
		} else if err := model.ValidateMmBlocksActions(update); err != nil {
			rctx.Logger().Info("Restoring original mm_blocks_actions: plugin update response was invalid",
				mlog.String("post_id", postID),
				mlog.String("url", setup.upstreamURL),
				mlog.Err(err),
			)
			update.AddProp(model.PostPropsMmBlocksActions, cloneMmBlocksActionsProp(originalMmBlocks))
		}
	}

	if setup.ephemeralInteractive {
		update.ChannelId = setup.ephemeralChannelID
		if update.RootId == "" && setup.ephemeralRootPostID != "" && setup.ephemeralRootPostID != postID {
			update.RootId = setup.ephemeralRootPostID
		}
		if setup.ephemeralSetUserID && update.UserId == "" {
			update.UserId = userID
		}
		a.UpdateEphemeralPost(rctx, userID, update)
		return nil
	}

	if _, _, appErr := a.UpdatePost(rctx, update, &model.UpdatePostOptions{SafeUpdate: false, AllowMmBlocksActionsUpdate: true}); appErr != nil {
		return appErr
	}
	return nil
}
