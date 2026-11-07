// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// toChannelAttributeAdmins narrows *model.User down to the subset safe to
// expose over the channel-attribute compliance endpoints (see
// model.ChannelAttributeAdmin's doc comment for why).
func toChannelAttributeAdmins(users []*model.User) []*model.ChannelAttributeAdmin {
	admins := make([]*model.ChannelAttributeAdmin, len(users))
	for i, u := range users {
		admins[i] = &model.ChannelAttributeAdmin{
			Id:        u.Id,
			Username:  u.Username,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Nickname:  u.Nickname,
		}
	}
	return admins
}

// GetChannelsMissingAttributeValue returns a paginated list of active
// channels lacking a value for the given channel attribute field, with each
// channel's channel admins.
func (a *App) GetChannelsMissingAttributeValue(rctx request.CTX, groupID string, field *model.PropertyField, page, perPage int) (*model.ChannelsMissingAttributeValueList, *model.AppError) {
	opts := store.ChannelSearchOpts{
		MissingPropertyValueGroupID: groupID,
		MissingPropertyValueFieldID: field.ID,
	}

	channels, err := a.Srv().Store().Channel().GetAllChannels(page*perPage, perPage, opts)
	if err != nil {
		return nil, model.NewAppError("GetChannelsMissingAttributeValue", "app.channel.missing_property_values.list.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	total, err := a.Srv().Store().Channel().GetAllChannelsCount(opts)
	if err != nil {
		return nil, model.NewAppError("GetChannelsMissingAttributeValue", "app.channel.missing_property_values.count.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	channelIDs := make([]string, len(channels))
	for i, ch := range channels {
		channelIDs[i] = ch.Id
	}

	admins, err := a.Srv().Store().Channel().GetChannelAdminsInfoByChannelIds(channelIDs)
	if err != nil {
		return nil, model.NewAppError("GetChannelsMissingAttributeValue", "app.channel.get_channel_admins.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	remoteIDs, err := a.Srv().Store().SharedChannel().GetRemoteChannelIds(channelIDs)
	if err != nil {
		return nil, model.NewAppError("GetChannelsMissingAttributeValue", "app.channel.missing_property_values.remote_channels.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	remoteSet := make(map[string]bool, len(remoteIDs))
	for _, id := range remoteIDs {
		remoteSet[id] = true
	}

	result := &model.ChannelsMissingAttributeValueList{
		TotalCount: total,
		Channels:   make([]*model.ChannelMissingAttributeValue, 0, len(channels)),
	}
	for _, ch := range channels {
		result.Channels = append(result.Channels, &model.ChannelMissingAttributeValue{
			ChannelId:          ch.Id,
			ChannelName:        ch.Name,
			ChannelDisplayName: ch.DisplayName,
			ChannelType:        ch.Type,
			TeamId:             ch.TeamId,
			TeamDisplayName:    ch.TeamDisplayName,
			IsLocal:            !remoteSet[ch.Id],
			ChannelAdmins:      toChannelAttributeAdmins(admins[ch.Id]),
		})
	}

	return result, nil
}

// GetChannelAttributeComplianceSummary returns the count-only view used by the
// Required toggle's banner and the notify confirmation modal. field is nil in
// create mode. viewerLocale renders MessagePreview in the requesting
// sysadmin's own locale -- it is a preview of the DM, not the DM itself, which
// is rendered per-recipient in each channel admin's own locale.
func (a *App) GetChannelAttributeComplianceSummary(rctx request.CTX, groupID string, field *model.PropertyField, viewerLocale string) (*model.ChannelAttributeComplianceSummary, *model.AppError) {
	localOpts := store.ChannelSearchOpts{ExcludeRemote: true}
	allOpts := store.ChannelSearchOpts{}
	fieldID := ""
	if field != nil {
		fieldID = field.ID
		localOpts.MissingPropertyValueGroupID = groupID
		localOpts.MissingPropertyValueFieldID = fieldID
		allOpts.MissingPropertyValueGroupID = groupID
		allOpts.MissingPropertyValueFieldID = fieldID
	}

	missingLocal, err := a.Srv().Store().Channel().GetAllChannelsCount(localOpts)
	if err != nil {
		return nil, model.NewAppError("GetChannelAttributeComplianceSummary", "app.channel.missing_property_values.count_local.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	missingAll, err := a.Srv().Store().Channel().GetAllChannelsCount(allOpts)
	if err != nil {
		return nil, model.NewAppError("GetChannelAttributeComplianceSummary", "app.channel.missing_property_values.count_all.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Aggregated in the database rather than derived from
	// GetChannelAdminsForChannelsMissingPropertyValue's row list, which is
	// capped at channelAttributeNotifyAssignmentLimit for delivery -- deriving
	// these counts from a possibly-truncated row list would understate
	// UniqueAdminCount and overstate ChannelsWithoutAdminCount (channels whose
	// admin rows fell past the cap would wrongly count as adminless) on an
	// install with enough affected channels to exceed it.
	uniqueAdminCount, channelsWithAdminCount, err := a.Srv().Store().Channel().CountChannelAdminAssignmentsForChannelsMissingPropertyValue(groupID, fieldID)
	if err != nil {
		return nil, model.NewAppError("GetChannelAttributeComplianceSummary", "app.channel_attributes.notify_assignment_counts.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	required := field != nil && model.IsPropertyFieldRequired(field)

	return &model.ChannelAttributeComplianceSummary{
		FieldId:                   fieldID,
		Required:                  required,
		MissingChannelCount:       missingLocal,
		SharedChannelCount:        missingAll - missingLocal,
		UniqueAdminCount:          uniqueAdminCount,
		ChannelsWithoutAdminCount: missingLocal - channelsWithAdminCount,
		MessagePreview:            channelAttributeNotifyMessagePreview(a.channelAttributeDisplayName(rctx, groupID, field), viewerLocale),
	}, nil
}
