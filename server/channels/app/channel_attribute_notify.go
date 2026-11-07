// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

const (
	// channelAttributeNotifyAssignmentLimit is an absolute ceiling on the
	// number of (channel admin, affected channel) rows one notify run will
	// accumulate across every page it fetches -- a backstop against a single
	// pathologically large install, not the normal path. Below this, a run
	// pages through every assignment rather than stopping at the first
	// page, so "Notify all channel admins" is not a claim a truncated first
	// page could falsify: a retry after the cooldown would otherwise always
	// re-fetch the same first page (a stable, deterministic order) and
	// re-notify the same admins forever, never reaching the rest.
	channelAttributeNotifyAssignmentLimit = 20000
	// channelAttributeNotifyAssignmentPageSize is the page size used to walk
	// up to channelAttributeNotifyAssignmentLimit rows.
	channelAttributeNotifyAssignmentPageSize = 2000
	// channelAttributeNotifyChannelsPerDM is the number of channels listed
	// inline in a single admin's DM before collapsing to "and N more".
	channelAttributeNotifyChannelsPerDM = 20
	// channelAttributeNotifyCoolOffMillis is the minimum time between two
	// notify runs for the same field, to keep a double-click or a page
	// refresh-and-reclick from re-spamming every affected channel admin.
	channelAttributeNotifyCoolOffMillis = 60 * 60 * 1000
)

func channelAttributeNotifyThrottleKey(fieldID string) string {
	return "channel_attribute_notify_last_sent_" + fieldID
}

// classificationFieldName is the name of classification's linked fields.
const classificationFieldName = "classification"

// channelAttributeDisplayName is the name channel admins and the notifying
// admin see for a channel attribute: the field's own display_name, else its
// template's, else its name. The System Console copies the template's
// display_name onto linked fields, but one created through the API or a plugin
// need not carry it, so it is looked up rather than shown as the internal
// name. classification writes no display_name and is labelled as the webapp's
// getPropertyFieldLabel labels it.
func (a *App) channelAttributeDisplayName(rctx request.CTX, groupID string, field *model.PropertyField) string {
	if field == nil {
		return ""
	}
	if dn := propertyFieldDisplayName(field); dn != "" {
		return dn
	}
	if field.LinkedFieldID != nil && *field.LinkedFieldID != "" {
		template, appErr := a.GetPropertyField(rctx, groupID, *field.LinkedFieldID)
		if appErr != nil {
			rctx.Logger().Warn("Failed to load the template of a channel attribute for its display name", mlog.String("field_id", field.ID), mlog.Err(appErr))
		} else if dn := propertyFieldDisplayName(template); dn != "" {
			return dn
		}
	}
	if field.Name == classificationFieldName {
		return "Classification"
	}
	return field.Name
}

func propertyFieldDisplayName(field *model.PropertyField) string {
	if field.Attrs == nil {
		return ""
	}
	dn, _ := field.Attrs[model.PropertyFieldAttrDisplayName].(string)
	return strings.TrimSpace(dn)
}

// channelAttributeNotifyMessagePreview renders the confirmation modal's
// preview of the DM channel admins will receive. It is intentionally the
// generic, no-channel-list form -- the real DM (see
// channelAttributeNotifyDMBody) is rendered per admin with their own affected
// channels, which this preview cannot enumerate for everyone at once. It is
// empty in create mode, where notify is unreachable from the UI.
func channelAttributeNotifyMessagePreview(attributeDisplayName, viewerLocale string) string {
	if attributeDisplayName == "" {
		return ""
	}
	T := i18n.GetUserTranslations(viewerLocale)
	return T("app.channel_attributes.notify_missing_value.preview", model.StringInterface{
		"AttributeName": attributeDisplayName,
	})
}

// channelAttributeNotifyChannelRef is enough about one channel to build a
// markdown permalink in a DM -- plain display names are ambiguous once a
// single admin's affected channels span more than one team.
type channelAttributeNotifyChannelRef struct {
	teamName    string
	channelName string
	displayName string
}

// channelAttributeAdminBatch is one recipient's worth of the notify fan-out:
// every channel of theirs missing a value, in the order the assignment query
// returned them (by channel display name).
type channelAttributeAdminBatch struct {
	username string
	locale   string
	channels []channelAttributeNotifyChannelRef
}

// claimChannelAttributeNotifyThrottle atomically claims the per-field notify
// cooldown in one store call, so two concurrent requests (a double-click, or
// two HA nodes) cannot both observe "cooldown elapsed" before either writes --
// unlike the read-then-write CanNotifyAdmin/FinishSendAdminNotifyPost pattern
// this used to mirror, which has exactly that race. Reports whether this call
// is the one that claimed it (false means someone else's claim already
// stands) and, when it is, the token that claim was written under --
// releaseChannelAttributeNotifyThrottle needs it back to release only this
// specific claim, not whatever happens to be there when release runs.
func (a *App) claimChannelAttributeNotifyThrottle(fieldID string) (claimed bool, token string, appErr *model.AppError) {
	token = strconv.FormatInt(model.GetMillis(), 10)
	claimed, err := a.Srv().Store().System().TryClaimIfOlderThan(
		channelAttributeNotifyThrottleKey(fieldID),
		token,
		channelAttributeNotifyCoolOffMillis,
	)
	if err != nil {
		return false, "", model.NewAppError("NotifyChannelAdminsOfMissingAttribute", "app.channel_attributes.notify_throttle_claim.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return claimed, token, nil
}

// releaseChannelAttributeNotifyThrottle undoes claimChannelAttributeNotifyThrottle
// after a run that reached zero recipients (system bot missing, or every DM
// failed to send): without this, a sysadmin retrying immediately after a
// fully-failed notify would be locked out for the rest of the hour despite
// nobody having actually been notified.
//
// Deletes conditionally on token, the exact value this run's claim wrote,
// rather than unconditionally by name: an unconditional delete would still
// remove whatever is in that row *now*, which is wrong if this run's own
// claim outlived the cooldown window before getting here (a slow request, a
// stalled goroutine) and a second, legitimate run has since claimed in its
// place -- releasing that one out from under it would let a third run
// double up with it. Best-effort otherwise -- a failure here just means the
// cooldown outlives the run that never delivered anything, same as any
// other throttle-store hiccup already tolerated elsewhere in this file.
func (a *App) releaseChannelAttributeNotifyThrottle(rctx request.CTX, fieldID, token string) {
	if _, err := a.Srv().Store().System().DeleteIfValueEquals(channelAttributeNotifyThrottleKey(fieldID), token); err != nil {
		rctx.Logger().Warn("Failed to release channel attribute notify cooldown after a fully-failed run",
			mlog.String("field_id", fieldID), mlog.Err(err))
	}
}

// fetchAllChannelAdminAssignments pages through every (channel admin,
// affected channel) row for field, up to channelAttributeNotifyAssignmentLimit
// total, rather than a single capped query -- see that constant's doc comment
// for why a single page is not good enough for a "notify all" action.
// Reports whether the absolute ceiling was hit (channels beyond it are
// simply never reached by this or any future run of this feature, an
// accepted limit on a pathologically large install).
func (a *App) fetchAllChannelAdminAssignments(groupID, fieldID string) ([]*model.ChannelAttributeAdminAssignment, bool, error) {
	var all []*model.ChannelAttributeAdminAssignment
	offset := 0
	for {
		page, err := a.Srv().Store().Channel().GetChannelAdminsForChannelsMissingPropertyValue(
			groupID, fieldID, channelAttributeNotifyAssignmentPageSize, offset)
		if err != nil {
			return nil, false, err
		}
		all = append(all, page...)
		if len(page) < channelAttributeNotifyAssignmentPageSize {
			return all, false, nil
		}
		if len(all) >= channelAttributeNotifyAssignmentLimit {
			return all, true, nil
		}
		offset += channelAttributeNotifyAssignmentPageSize
	}
}

// NotifyChannelAdminsOfMissingAttribute sends a batched system-bot DM to every
// unique channel admin of a channel missing a value for field, throttled per
// field. Counts are computed synchronously so the caller can render them
// immediately; delivery happens in the background.
func (a *App) NotifyChannelAdminsOfMissingAttribute(rctx request.CTX, groupID string, field *model.PropertyField) (*model.ChannelAttributeNotifyResult, *model.AppError) {
	// Claimed atomically, and first: closes the double-click/two-HA-node
	// race a separate check-then-write could not. Everything below that can
	// still fail (transient store errors, a fully-failed delivery) releases
	// this claim before returning, so a real "nobody was notified" outcome
	// never costs the full hour -- see releaseChannelAttributeNotifyThrottle
	// and its call sites.
	claimed, token, appErr := a.claimChannelAttributeNotifyThrottle(field.ID)
	if appErr != nil {
		return nil, appErr
	}
	if !claimed {
		return nil, model.NewAppError("NotifyChannelAdminsOfMissingAttribute", "api.property_field.notify_missing_values.throttled.app_error", nil, "", http.StatusTooManyRequests)
	}

	missingLocal, err := a.Srv().Store().Channel().GetAllChannelsCount(store.ChannelSearchOpts{
		ExcludeRemote:               true,
		MissingPropertyValueGroupID: groupID,
		MissingPropertyValueFieldID: field.ID,
	})
	if err != nil {
		a.releaseChannelAttributeNotifyThrottle(rctx, field.ID, token)
		return nil, model.NewAppError("NotifyChannelAdminsOfMissingAttribute", "app.channel.missing_property_values.count.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	assignments, truncated, err := a.fetchAllChannelAdminAssignments(groupID, field.ID)
	if err != nil {
		a.releaseChannelAttributeNotifyThrottle(rctx, field.ID, token)
		return nil, model.NewAppError("NotifyChannelAdminsOfMissingAttribute", "app.channel_attributes.notify_assignments.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Aggregated separately (see the identical comment in
	// GetChannelAttributeComplianceSummary): only ChannelsWithoutAdminCount
	// reads from this, since it is reported as an independent fact about the
	// server, distinct from QueuedAdminCount/QueuedChannelCount below,
	// which describe who this run queued for delivery.
	_, exactChannelsWithAdminCount, err := a.Srv().Store().Channel().CountChannelAdminAssignmentsForChannelsMissingPropertyValue(groupID, field.ID)
	if err != nil {
		a.releaseChannelAttributeNotifyThrottle(rctx, field.ID, token)
		return nil, model.NewAppError("NotifyChannelAdminsOfMissingAttribute", "app.channel_attributes.notify_assignment_counts.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	batches := make(map[string]*channelAttributeAdminBatch)
	channelsWithAdmin := make(map[string]bool)
	for _, row := range assignments {
		b, ok := batches[row.UserId]
		if !ok {
			b = &channelAttributeAdminBatch{username: row.Username, locale: row.Locale}
			batches[row.UserId] = b
		}
		b.channels = append(b.channels, channelAttributeNotifyChannelRef{
			teamName:    row.TeamName,
			channelName: row.ChannelName,
			displayName: row.ChannelDisplayName,
		})
		channelsWithAdmin[row.ChannelId] = true
	}

	result := &model.ChannelAttributeNotifyResult{
		QueuedAdminCount:          int64(len(batches)),
		QueuedChannelCount:        int64(len(channelsWithAdmin)),
		ChannelsWithoutAdminCount: missingLocal - exactChannelsWithAdminCount,
		Truncated:                 truncated,
	}

	displayName := a.channelAttributeDisplayName(rctx, groupID, field)
	fieldID := field.ID

	a.Srv().Go(func() {
		a.sendChannelAttributeNotifyDMs(rctx, batches, displayName, fieldID, token)
	})

	return result, nil
}

func (a *App) sendChannelAttributeNotifyDMs(rctx request.CTX, batches map[string]*channelAttributeAdminBatch, attributeDisplayName, fieldID, throttleToken string) {
	if len(batches) == 0 {
		// Every channel missing the value has no channel admin at all -- the
		// D5 "report only" outcome, not a failure. Still nobody was
		// notified, so this releases the cooldown the same as the
		// GetSystemBot/fully-failed-delivery cases below: an admin who fixes
		// this by adding themselves as a channel admin should not have to
		// wait out the hour to retry.
		a.releaseChannelAttributeNotifyThrottle(rctx, fieldID, throttleToken)
		return
	}

	systemBot, appErr := a.GetSystemBot(rctx)
	if appErr != nil {
		rctx.Logger().Error("Failed to get system bot to notify channel admins of a missing required channel attribute",
			mlog.String("field_id", fieldID), mlog.Err(appErr))
		// Nobody can be reached without the bot -- release the cooldown so a
		// retry (once whatever is wrong with GetSystemBot is fixed) is not
		// stuck waiting out the full hour for a run that notified no one.
		a.releaseChannelAttributeNotifyThrottle(rctx, fieldID, throttleToken)
		return
	}

	var delivered int
	for userID, batch := range batches {
		channel, appErr := a.GetOrCreateDirectChannel(rctx, userID, systemBot.UserId)
		if appErr != nil {
			rctx.Logger().Warn("Failed to get direct channel to notify channel admin of a missing required channel attribute",
				mlog.String("user_id", userID), mlog.String("field_id", fieldID), mlog.Err(appErr))
			continue
		}

		T := i18n.GetUserTranslations(batch.locale)
		post := &model.Post{
			ChannelId: channel.Id,
			UserId:    systemBot.UserId,
			Type:      model.PostTypeDefault,
			Message: T("app.channel_attributes.notify_missing_value.dm", model.StringInterface{
				"AttributeName": attributeDisplayName,
				"ChannelList":   channelAttributeNotifyChannelList(batch.channels, T),
			}),
			Props: model.StringInterface{
				"property_field_id": fieldID,
				"attribute_name":    attributeDisplayName,
			},
		}

		if _, _, appErr := a.CreatePost(rctx, post, channel, model.CreatePostFlags{SetOnline: true}); appErr != nil {
			rctx.Logger().Warn("Failed to send missing required channel attribute notification",
				mlog.String("user_id", userID), mlog.String("field_id", fieldID), mlog.Err(appErr))
			continue
		}
		delivered++
	}

	// Every admin failed (direct channel or post creation) -- same reasoning
	// as the GetSystemBot case above: nobody was actually notified, so the
	// cooldown should not stand in the way of an immediate retry.
	if delivered == 0 {
		a.releaseChannelAttributeNotifyThrottle(rctx, fieldID, throttleToken)
	}
}

// escapeChannelAttributeMarkdownLinkText escapes backslashes and square
// brackets so a channel display name, which any channel admin controls, cannot
// terminate the link text it is inserted into and redirect the link.
func escapeChannelAttributeMarkdownLinkText(displayName string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(displayName)
}

// channelAttributeNotifyChannelList renders the bulleted channel list for a
// single admin's DM as markdown permalinks -- a bare ~channel-name mention
// only auto-links within the team currently being viewed, and one admin's
// affected channels routinely span teams. Collapses beyond
// channelAttributeNotifyChannelsPerDM so an admin of hundreds of channels
// doesn't receive an unreadable wall of text.
func channelAttributeNotifyChannelList(channels []channelAttributeNotifyChannelRef, T i18n.TranslateFunc) string {
	shown := channels
	var more int
	if len(shown) > channelAttributeNotifyChannelsPerDM {
		more = len(shown) - channelAttributeNotifyChannelsPerDM
		shown = shown[:channelAttributeNotifyChannelsPerDM]
	}

	var b strings.Builder
	for _, ref := range shown {
		link := "/" + ref.teamName + "/channels/" + ref.channelName
		b.WriteString(T("app.channel_attributes.notify_missing_value.dm_channel_line", model.StringInterface{
			"ChannelDisplayName": escapeChannelAttributeMarkdownLinkText(ref.displayName),
			"ChannelLink":        link,
		}))
	}
	if more > 0 {
		b.WriteString(T("app.channel_attributes.notify_missing_value.dm_more", model.StringInterface{"Count": more}))
	}
	return b.String()
}
