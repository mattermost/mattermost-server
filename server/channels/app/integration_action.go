// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Integration Action Flow
//
// 1. An integration creates an interactive message button or menu.
// 2. A user clicks on a button or selects an option from the menu.
// 3. The client sends a request to server to complete the post action, calling DoPostActionWithCookie below.
// 4. DoPostActionWithCookie will send an HTTP POST request to the integration containing contextual data, including
// an encoded and signed trigger ID. Slash commands also include trigger IDs in their payloads.
// 5. The integration performs any actions it needs to and optionally makes a request back to the MM server
// using the trigger ID to open an interactive dialog.
// 6. If that optional request is made, OpenInteractiveDialog sends a WebSocket event to all connected clients
// for the relevant user, telling them to display the dialog.
// 7. The user fills in the dialog and submits it, where SubmitInteractiveDialog will submit it back to the
// integration for handling.

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/utils"
)

func (a *App) DoPostActionWithCookie(rctx request.CTX, postID, actionId, userID, selectedOption string, legacyCookie *model.PostActionCookie, mmBlocksCookie *model.MmBlocksActionCookie, clientQuery map[string]string, integrationFormat string) (string, string, *model.AppError) {
	// Bound the per-click query at the App boundary so any caller — REST
	// handler, plugin, future internal trigger — gets the same enforcement.
	if err := model.ValidateActionQuery(clientQuery); err != nil {
		return "", "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.query.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	setup, gotoURL, appErr := a.resolvePostActionSetup(rctx, postID, actionId, userID, legacyCookie, mmBlocksCookie, clientQuery, integrationFormat)
	if appErr != nil {
		return "", "", appErr
	}
	if gotoURL != "" {
		return "", gotoURL, nil
	}

	// Plugin routes on a stored post are dispatched when the post was created by
	// a bot, incoming webhook, plugin, or OAuth app. The decision uses the markers
	// CreatePost persists for those authors, the same classes that keep
	// mm_blocks_actions. The check runs before DoActionRequest.
	if appErr = a.pluginActionRouteError(rctx, setup); appErr != nil {
		return "", "", appErr
	}

	upstreamRequest := setup.upstreamRequest

	if selectedOption != "" {
		if upstreamRequest.Context == nil {
			upstreamRequest.Context = map[string]any{}
		}
		upstreamRequest.Context["selected_option"] = selectedOption
		upstreamRequest.DataSource = setup.datasource
	}

	clientTriggerId, _, appErr := upstreamRequest.GenerateTriggerId(a.AsymmetricSigningKey())
	if appErr != nil {
		return "", "", appErr
	}

	requestJSON, err := json.Marshal(upstreamRequest)
	if err != nil {
		return "", "", model.NewAppError("DoPostActionWithCookie", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Log request, regardless of whether destination is internal or external
	rctx.Logger().Info("DoPostActionWithCookie POST request, through DoActionRequest",
		mlog.String("url", setup.upstreamURL),
		mlog.String("user_id", upstreamRequest.UserId),
		mlog.String("post_id", upstreamRequest.PostId),
		mlog.String("channel_id", upstreamRequest.ChannelId),
		mlog.String("team_id", upstreamRequest.TeamId),
	)

	ctx, cancel := context.WithTimeout(a.Srv().Platform().GoContext(), time.Duration(*a.Config().ServiceSettings.OutgoingIntegrationRequestsTimeout)*time.Second)
	defer cancel()
	resp, appErr := a.DoActionRequest(rctx.WithContext(ctx), setup.upstreamURL, requestJSON)
	if appErr != nil {
		return "", "", appErr
	}
	defer resp.Body.Close()

	var response model.PostActionIntegrationResponse
	limitedReader := io.LimitReader(resp.Body, MaxIntegrationResponseSize)
	respBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	if len(respBytes) > 0 {
		if err = json.Unmarshal(respBytes, &response); err != nil {
			return "", "", model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		}
	}

	if response.Update != nil {
		if appErr = a.applyPostActionUpdate(rctx, setup, postID, userID, response.Update); appErr != nil {
			return "", "", appErr
		}
	}

	if response.EphemeralText != "" {
		ephemeralPost := &model.Post{
			Message:   response.EphemeralText,
			ChannelId: upstreamRequest.ChannelId,
			RootId:    setup.rootPostId,
			UserId:    userID,
		}

		if !response.SkipSlackParsing {
			ephemeralPost.Message = model.ParseSlackLinksToMarkdown(response.EphemeralText)
		}

		for key, value := range setup.retain {
			ephemeralPost.AddProp(key, value)
		}
		a.SendEphemeralPost(rctx, userID, ephemeralPost)
	}

	return clientTriggerId, response.GotoLocation, nil
}

// pluginActionRouteError reports when a stored post's action URL is a plugin
// route and the post author is not a bot, incoming webhook, plugin, or OAuth app.
// Ephemeral actions have no stored post and keep the current dispatch.
func (a *App) pluginActionRouteError(rctx request.CTX, setup *postActionSetup) *model.AppError {
	if setup == nil || setup.post == nil || !a.actionURLIsPluginRoute(setup.upstreamURL) {
		return nil
	}

	dispatch, appErr := a.storedPostDispatchesPluginAction(rctx, setup.post)
	if appErr != nil {
		return appErr
	}
	if dispatch {
		return nil
	}

	return model.NewAppError("DoPostActionWithCookie", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest)
}

// storedPostDispatchesPluginAction reports whether the stored post was created
// by a bot, incoming webhook, plugin, or OAuth app. Those entry points persist
// from_bot, from_webhook, from_plugin, and from_oauth_app; a bot user is also
// read from the author record. A normal create does not set those props.
func (a *App) storedPostDispatchesPluginAction(rctx request.CTX, post *model.Post) (bool, *model.AppError) {
	if postPropIsTrue(post, model.PostPropsFromBot) ||
		postPropIsTrue(post, model.PostPropsFromWebhook) ||
		postPropIsTrue(post, model.PostPropsFromPlugin) ||
		postPropIsTrue(post, model.PostPropsFromOAuthApp) {
		return true, nil
	}

	user, appErr := a.GetUser(rctx, post.UserId)
	if appErr != nil {
		return false, appErr
	}
	return user.IsBot, nil
}

func postPropIsTrue(post *model.Post, key string) bool {
	if post == nil {
		return false
	}
	return post.GetProp(key) == "true"
}

// actionURLIsPluginRoute reports whether rawURL is a plugin route handled by
// DoActionRequest in-process (a relative /plugins/ or plugins/ path) or by the
// SiteURL client in getPostActionClient (same host, path under the plugin
// prefix, including a configured subpath).
func (a *App) actionURLIsPluginRoute(rawURL string) bool {
	cleaned := path.Clean(rawURL)
	if strings.HasPrefix(cleaned, "/plugins/") || strings.HasPrefix(cleaned, "plugins/") {
		return true
	}

	inURL, err := url.Parse(rawURL)
	if err != nil || inURL == nil {
		return false
	}
	siteURL, _ := url.Parse(*a.Config().ServiceSettings.SiteURL)
	if siteURL == nil || inURL.Hostname() != siteURL.Hostname() {
		return false
	}
	subpath, _ := utils.GetSubpathFromConfig(a.Config())
	return strings.HasPrefix(path.Clean(inURL.Path), path.Join(subpath, "plugins"))
}

// DoActionRequest performs an HTTP POST request to an integration's action endpoint.
// Caller must consume and close returned http.Response as necessary.
// For internal requests, requests are routed directly to a plugin ServerHTTP hook
func (a *App) DoActionRequest(rctx request.CTX, rawURL string, body []byte) (*http.Response, *model.AppError) {
	inURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, model.NewAppError("DoActionRequest", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	rawURLPath := path.Clean(rawURL)
	if strings.HasPrefix(rawURLPath, "/plugins/") || strings.HasPrefix(rawURLPath, "plugins/") {
		return a.DoLocalRequest(rctx, rawURLPath, body)
	}

	req, err := http.NewRequestWithContext(rctx.Context(), "POST", rawURL, bytes.NewReader(body))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			rctx.Logger().Info("Outgoing Integration Action request timed out. Consider increasing ServiceSettings.OutgoingIntegrationRequestsTimeout.")
		}
		return nil, model.NewAppError("DoActionRequest", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpClient := a.getPostActionClient(rctx, inURL, req)

	resp, httpErr := httpClient.Do(req)
	if httpErr != nil {
		return nil, model.NewAppError("DoActionRequest", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(httpErr)
	}

	if resp.StatusCode != http.StatusOK {
		// Preserve 429 and 503 because they carry retry semantics (RFC 6585,
		// RFC 7231) that downstream HTTP clients legitimately need. Map other
		// 5xx responses to 502 Bad Gateway since the failure is upstream of
		// MM. Sanitize all other non-200 responses to 400 so plugin-specific
		// 4xx codes do not leak misleading semantics to MM API clients.
		status := http.StatusBadRequest
		switch {
		case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode == http.StatusServiceUnavailable:
			status = resp.StatusCode
		case resp.StatusCode >= 500:
			status = http.StatusBadGateway
		}
		return resp, model.NewAppError("DoActionRequest", "api.post.do_action.action_integration.app_error", nil, fmt.Sprintf("status=%v", resp.StatusCode), status)
	}

	return resp, nil
}

// getPostActionClient returns the client used to call an integration action. Callers of
// DoActionRequest give the request a deadline derived from
// ServiceSettings.OutgoingIntegrationRequestsTimeout, so the client adds no timeout of its own,
// which would cap a configured value above httpservice.RequestTimeout. A request that arrives
// without a deadline still gets the configured timeout instead of running unbounded.
func (a *App) getPostActionClient(rctx request.CTX, inURL *url.URL, req *http.Request) *http.Client {
	// Allow access to plugin routes for action buttons
	var httpClient *http.Client
	subpath, _ := utils.GetSubpathFromConfig(a.Config())
	siteURL, _ := url.Parse(*a.Config().ServiceSettings.SiteURL)
	if inURL.Hostname() == siteURL.Hostname() && strings.HasPrefix(path.Clean(inURL.Path), path.Join(subpath, "plugins")) {
		req.Header.Set(model.HeaderAuth, "Bearer "+rctx.Session().Token)
		httpClient = a.HTTPService().MakeClient(true)
	} else {
		httpClient = a.HTTPService().MakeClient(false)
	}

	if _, ok := rctx.Context().Deadline(); ok {
		httpClient.Timeout = 0
	} else {
		httpClient.Timeout = time.Duration(*a.Config().ServiceSettings.OutgoingIntegrationRequestsTimeout) * time.Second
	}

	return httpClient
}

type LocalResponseWriter struct {
	data    []byte
	headers http.Header
	status  int
}

func (w *LocalResponseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *LocalResponseWriter) Write(bytes []byte) (int, error) {
	w.data = make([]byte, len(bytes))
	copy(w.data, bytes)
	return len(w.data), nil
}

func (w *LocalResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (a *App) doPluginRequest(rctx request.CTX, method, rawURL string, values url.Values, body []byte) (*http.Response, *model.AppError) {
	return a.ch.doPluginRequest(rctx, method, rawURL, values, body)
}

func (ch *Channels) doPluginRequest(rctx request.CTX, method, rawURL string, values url.Values, body []byte) (*http.Response, *model.AppError) {
	rawURL = strings.TrimPrefix(rawURL, "/")
	inURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, model.NewAppError("doPluginRequest", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	result := strings.Split(path.Clean(inURL.Path), "/")
	if len(result) < 2 {
		return nil, model.NewAppError("doPluginRequest", "api.post.do_action.action_integration.app_error", nil, "err=Unable to find pluginId", http.StatusBadRequest)
	}

	if result[0] != "plugins" {
		return nil, model.NewAppError("doPluginRequest", "api.post.do_action.action_integration.app_error", nil, "err=plugins not in path", http.StatusBadRequest)
	}

	pluginID := result[1]

	path := strings.TrimPrefix(inURL.Path, "plugins/"+pluginID)

	base, err := url.Parse(path)
	if err != nil {
		return nil, model.NewAppError("doPluginRequest", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	// merge the rawQuery params (if any) with the function's provided values
	rawValues := inURL.Query()
	if len(rawValues) != 0 {
		if values == nil {
			values = make(url.Values)
		}
		for k, vs := range rawValues {
			for _, v := range vs {
				values.Add(k, v)
			}
		}
	}
	if values != nil {
		base.RawQuery = values.Encode()
	}

	w := &LocalResponseWriter{}
	r, err := http.NewRequest(method, base.String(), bytes.NewReader(body))
	if err != nil {
		return nil, model.NewAppError("doPluginRequest", "api.post.do_action.action_integration.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	r.Header.Set("Mattermost-User-Id", rctx.Session().UserId)
	r.Header.Set(model.HeaderAuth, "Bearer "+rctx.Session().Token)
	params := make(map[string]string)
	params["plugin_id"] = pluginID
	r = mux.SetURLVars(r, params)

	ch.ServePluginRequest(w, r)

	resp := &http.Response{
		StatusCode: w.status,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     w.headers,
		Body:       io.NopCloser(bytes.NewReader(w.data)),
	}
	if resp.StatusCode == 0 {
		resp.StatusCode = http.StatusOK
	}

	return resp, nil
}

type MailToLinkContent struct {
	MetricId      string `json:"metric_id"`
	MailRecipient string `json:"mail_recipient"`
	MailCC        string `json:"mail_cc"`
	MailSubject   string `json:"mail_subject"`
	MailBody      string `json:"mail_body"`
}

func (mlc *MailToLinkContent) ToJSON() string {
	b, _ := json.Marshal(mlc)
	return string(b)
}

func (a *App) DoLocalRequest(rctx request.CTX, rawURL string, body []byte) (*http.Response, *model.AppError) {
	return a.doPluginRequest(rctx, "POST", rawURL, nil, body)
}

func (a *App) OpenInteractiveDialog(rctx request.CTX, request model.OpenDialogRequest) *model.AppError {
	timeout := time.Duration(*a.Config().ServiceSettings.OutgoingIntegrationRequestsTimeout) * time.Second
	clientTriggerId, userID, channelID, appErr := request.DecodeAndVerifyTriggerId(a.AsymmetricSigningKey(), timeout)
	if appErr != nil {
		return appErr
	}

	request.TriggerId = clientTriggerId
	request.ChannelId = channelID

	if dialogErr := request.IsValid(); dialogErr != nil {
		rctx.Logger().Warn("Interactive dialog is invalid", mlog.Err(dialogErr))
	}

	jsonRequest, err := json.Marshal(request)
	if err != nil {
		a.ch.srv.Log().Warn("Error encoding request", mlog.Err(err))
	}

	message := model.NewWebSocketEvent(model.WebsocketEventOpenDialog, "", "", userID, nil, "")
	message.Add("dialog", string(jsonRequest))
	a.Publish(message)

	return nil
}

func (a *App) SubmitInteractiveDialog(rctx request.CTX, request model.SubmitDialogRequest) (*model.SubmitDialogResponse, *model.AppError) {
	url := request.URL
	request.URL = ""

	// Validate submitted file IDs exist and belong to the submitting user.
	// Dedup without sorting so the order the user selected the files is preserved
	// when the request is forwarded to the integration.
	request.FileIds = model.RemoveDuplicateStringsNonSort(request.FileIds)
	if len(request.FileIds) > model.MaxDialogFileIds {
		return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.too_many_file_ids",
			map[string]any{"Max": model.MaxDialogFileIds}, "", http.StatusBadRequest)
	}
	declaredFileIDs := make(map[string]bool, len(request.FileIds))
	for _, fileID := range request.FileIds {
		declaredFileIDs[fileID] = true
	}

	// Validate the declared file IDs with a single batched lookup (one DB roundtrip
	// instead of one per file). This is the primary ownership check, so a store
	// error fails closed.
	if len(request.FileIds) > 0 {
		declaredFiles, nErr := a.Srv().Store().FileInfo().GetByIds(request.FileIds, false, false, false)
		if nErr != nil {
			return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.get_file_info_error", nil, "", http.StatusInternalServerError).Wrap(nErr)
		}
		foundFileIDs := make(map[string]bool, len(declaredFiles))
		for _, fileInfo := range declaredFiles {
			foundFileIDs[fileInfo.Id] = true
			if fileInfo.CreatorId != request.UserId {
				return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.file_not_owned", map[string]any{"FileId": fileInfo.Id}, "", http.StatusForbidden)
			}
		}
		// Any declared ID not returned doesn't exist (or was deleted) — reject it.
		for _, fileID := range request.FileIds {
			if !foundFileIDs[fileID] {
				return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.invalid_file_id", map[string]any{"FileId": fileID}, "", http.StatusBadRequest)
			}
		}
	}

	// Defense in depth: integrations read raw field values from request.Submission
	// (e.g. submission["my_file"]), not request.FileIds. Because the submit request
	// does not carry the dialog's element definitions, the server cannot tell which
	// submission fields are file pickers, so a malicious client could smuggle another
	// user's file ID through a submission value while sending a benign FileIds list.
	// Collect ID-shaped tokens from submission values (recursing into arrays/objects),
	// resolve them in a single batch, and enforce the same ownership check on any that
	// are real files. Tokens that don't resolve to a file (e.g. user/channel select
	// IDs, or ID-shaped free text) are ordinary values and ignored.
	//
	// The scan is bounded in both breadth (ID-shaped tokens collected) and depth
	// (recursion into nested arrays/objects). Hitting either bound must not silently
	// skip remaining values — breadth overflow fails closed so a padded submission
	// cannot smuggle an unchecked file ID past the cap.
	const maxSubmissionScanDepth = 100
	candidateFileIDs := make([]string, 0)
	seenCandidate := make(map[string]bool)
	scanLimitExceeded := false
	var collectIDs func(v any, depth int)
	collectIDs = func(v any, depth int) {
		if depth > maxSubmissionScanDepth || scanLimitExceeded {
			return
		}
		switch typed := v.(type) {
		case string:
			for tok := range strings.SplitSeq(typed, ",") {
				tok = strings.TrimSpace(tok)
				if tok == "" || declaredFileIDs[tok] || seenCandidate[tok] || !model.IsValidId(tok) {
					continue
				}
				if len(candidateFileIDs) >= model.MaxDialogSubmissionIDShapedTokenScan {
					scanLimitExceeded = true
					return
				}
				seenCandidate[tok] = true
				candidateFileIDs = append(candidateFileIDs, tok)
			}
		case []any:
			for _, e := range typed {
				if scanLimitExceeded {
					return
				}
				collectIDs(e, depth+1)
			}
		case []string:
			for _, e := range typed {
				if scanLimitExceeded {
					return
				}
				collectIDs(e, depth+1)
			}
		case map[string]any:
			for _, e := range typed {
				if scanLimitExceeded {
					return
				}
				collectIDs(e, depth+1)
			}
		}
	}
	for _, raw := range request.Submission {
		if scanLimitExceeded {
			break
		}
		collectIDs(raw, 0)
	}

	if scanLimitExceeded {
		return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.too_many_submission_ids",
			map[string]any{"Max": model.MaxDialogSubmissionIDShapedTokenScan}, "", http.StatusBadRequest)
	}

	if len(candidateFileIDs) > 0 {
		// allowFromCache=false: ownership must be decided from current DB state, not a
		// possibly stale per-file cache entry.
		submissionFiles, nErr := a.Srv().Store().FileInfo().GetByIds(candidateFileIDs, false, false, false)
		if nErr != nil {
			// Defense-in-depth scan: a transient store error must not block an otherwise
			// valid submission whose tokens may just be coincidental ID-shaped text. The
			// primary FileIds ownership check above already ran fail-closed.
			rctx.Logger().Warn("Could not resolve submission file IDs for ownership check", mlog.Err(nErr))
		} else {
			for _, fileInfo := range submissionFiles {
				if fileInfo.CreatorId != request.UserId {
					return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.file_not_owned", map[string]any{"FileId": fileInfo.Id}, "", http.StatusForbidden)
				}
			}
			// Only tokens that are real files count toward the limit; combine with the
			// declared FileIds so the per-submission ceiling is MaxDialogFileIds total.
			if len(declaredFileIDs)+len(submissionFiles) > model.MaxDialogFileIds {
				return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.too_many_file_ids",
					map[string]any{"Max": model.MaxDialogFileIds}, "", http.StatusBadRequest)
			}
		}
	}

	// Preserve Type field for field refresh functionality, otherwise default to dialog_submission
	if request.Type != "refresh" {
		request.Type = "dialog_submission"
	}

	b, err := json.Marshal(request)
	if err != nil {
		return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.json_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	// Log request, regardless of whether destination is internal or external
	rctx.Logger().Info("SubmitInteractiveDialog POST request, through DoActionRequest",
		mlog.String("url", url),
		mlog.String("user_id", request.UserId),
		mlog.String("channel_id", request.ChannelId),
		mlog.String("team_id", request.TeamId),
		mlog.Bool("cancelled", request.Cancelled),
	)

	ctx, cancel := context.WithTimeout(a.Srv().Platform().GoContext(), time.Duration(*a.Config().ServiceSettings.OutgoingIntegrationRequestsTimeout)*time.Second)
	defer cancel()
	resp, appErr := a.DoActionRequest(rctx.WithContext(ctx), url, b)
	if appErr != nil {
		return nil, appErr
	}
	defer resp.Body.Close()

	// Limit response size to prevent OOM attacks
	limitedReader := io.LimitReader(resp.Body, MaxDialogResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.read_body_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	var response model.SubmitDialogResponse
	if len(body) == 0 {
		// Don't fail, an empty response is acceptable
		return &response, nil
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.decode_json_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Validate the response
	if err := response.IsValid(); err != nil {
		if strings.Contains(err.Error(), "invalid form") {
			rctx.Logger().Info("Interactive dialog is invalid", mlog.Err(err))
		} else {
			return nil, model.NewAppError("SubmitInteractiveDialog", "app.submit_interactive_dialog.invalid_response", nil, err.Error(), http.StatusBadRequest)
		}
	}

	return &response, nil
}

func (a *App) ExecuteDialogAction(rctx request.CTX, userID string, req model.ExecuteDialogActionRequest) (string, *model.AppError) {
	if !model.IsValidLookupURL(req.URL) {
		return "", model.NewAppError("ExecuteDialogAction", "api.post.do_action.action_integration.app_error", nil, "invalid URL", http.StatusBadRequest)
	}

	if err := model.ValidateActionQuery(req.Context); err != nil {
		return "", model.NewAppError("ExecuteDialogAction", "api.post.do_action.action_integration.app_error", nil, err.Error(), http.StatusBadRequest)
	}

	ctx := make(map[string]any, len(req.Context))
	for k, v := range req.Context {
		ctx[k] = v
	}

	upstreamRequest := &model.PostActionIntegrationRequest{
		Type:      "dialog_action",
		UserId:    userID,
		ChannelId: req.ChannelId,
		TeamId:    req.TeamId,
		Context:   ctx,
	}

	user, userErr := a.Srv().Store().User().Get(rctx, userID)
	if userErr != nil {
		return "", model.NewAppError("ExecuteDialogAction", "app.user.get.app_error", nil, "", http.StatusInternalServerError).Wrap(userErr)
	}
	upstreamRequest.UserName = user.Username

	channel, channelErr := a.GetChannel(rctx, req.ChannelId)
	if channelErr != nil {
		return "", channelErr
	}
	upstreamRequest.ChannelName = channel.Name

	if req.TeamId != "" {
		team, teamErr := a.Srv().Store().Team().Get(req.TeamId)
		if teamErr != nil {
			return "", model.NewAppError("ExecuteDialogAction", "app.team.get.finding.app_error", nil, "", http.StatusInternalServerError).Wrap(teamErr)
		}
		upstreamRequest.TeamName = team.Name
	}

	clientTriggerId, _, appErr := upstreamRequest.GenerateTriggerId(a.AsymmetricSigningKey())
	if appErr != nil {
		return "", appErr
	}

	requestJSON, err := json.Marshal(upstreamRequest)
	if err != nil {
		return "", model.NewAppError("ExecuteDialogAction", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	rctx.Logger().Info("ExecuteDialogAction POST request, through DoActionRequest",
		mlog.String("url", req.URL),
		mlog.String("user_id", userID),
		mlog.String("channel_id", req.ChannelId),
		mlog.String("team_id", req.TeamId),
	)

	timeoutCtx, cancel := context.WithTimeout(a.Srv().Platform().GoContext(), time.Duration(*a.Config().ServiceSettings.OutgoingIntegrationRequestsTimeout)*time.Second)
	defer cancel()
	resp, appErr := a.DoActionRequest(rctx.WithContext(timeoutCtx), req.URL, requestJSON)
	if appErr != nil {
		// DoActionRequest can return a non-nil response together with an error
		// (e.g. a non-200 status). Drain and close it so the HTTP connection
		// isn't leaked on the error path.
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, MaxDialogResponseSize))
			_ = resp.Body.Close()
		}
		return "", appErr
	}
	defer resp.Body.Close()

	// Drain response body to allow HTTP connection reuse
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, MaxDialogResponseSize))

	// DoActionRequest routes plugin-relative URLs ("/plugins/...") through
	// DoLocalRequest, which never inspects the plugin handler's response status, so a
	// failing action arrives here with appErr == nil. Checking it here rather than in
	// DoActionRequest keeps the change scoped to dialog actions: the other
	// DoActionRequest callers parse the response body and would change behaviour.
	//
	// Without this the client is told the action succeeded, so an action_button's error
	// state is unreachable no matter what the integration returns.
	if resp.StatusCode != http.StatusOK {
		// Mirrors the status mapping DoActionRequest applies to non-plugin URLs:
		// preserve the retry-carrying 429/503, map other 5xx to 502 since the failure
		// is upstream of MM, and sanitize everything else to 400.
		status := http.StatusBadRequest
		switch {
		case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode == http.StatusServiceUnavailable:
			status = resp.StatusCode
		case resp.StatusCode >= 500:
			status = http.StatusBadGateway
		}
		return "", model.NewAppError("ExecuteDialogAction", "api.post.do_action.action_integration.app_error", nil, fmt.Sprintf("status=%v", resp.StatusCode), status)
	}

	return clientTriggerId, nil
}

func (a *App) LookupInteractiveDialog(rctx request.CTX, request model.SubmitDialogRequest) (*model.LookupDialogResponse, *model.AppError) {
	url := request.URL
	request.URL = ""
	request.Type = "dialog_lookup"

	b, err := json.Marshal(request)
	if err != nil {
		return nil, model.NewAppError("LookupInteractiveDialog", "app.lookup_interactive_dialog.json_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	// Log request, regardless of whether destination is internal or external
	rctx.Logger().Info("LookupInteractiveDialog POST request, through DoActionRequest",
		mlog.String("url", url),
		mlog.String("user_id", request.UserId),
		mlog.String("channel_id", request.ChannelId),
		mlog.String("team_id", request.TeamId),
	)

	ctx, cancel := context.WithTimeout(a.Srv().Platform().GoContext(), time.Duration(*a.Config().ServiceSettings.OutgoingIntegrationRequestsTimeout)*time.Second)
	defer cancel()
	resp, appErr := a.DoActionRequest(rctx.WithContext(ctx), url, b)
	if appErr != nil {
		return nil, appErr
	}
	defer resp.Body.Close()

	// Limit response size to prevent OOM attacks
	limitedReader := io.LimitReader(resp.Body, MaxDialogResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, model.NewAppError("LookupInteractiveDialog", "app.lookup_interactive_dialog.read_body_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	var response model.LookupDialogResponse
	if len(body) == 0 {
		// Return empty response if no data
		return &response, nil
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, model.NewAppError("LookupInteractiveDialog", "app.lookup_interactive_dialog.decode_json_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	return &response, nil
}
