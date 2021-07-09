// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

func (a *App) AddStatusCacheSkipClusterSend(status *model.Status) {
	a.Srv().statusCache.Set(status.UserID, status)
}

func (a *App) AddStatusCache(status *model.Status) {
	a.AddStatusCacheSkipClusterSend(status)

	if a.Cluster() != nil {
		msg := &model.ClusterMessage{
			Event:    model.ClusterEventUpdateStatus,
			SendType: model.ClusterSendBestEffort,
			Data:     status.ToClusterJSON(),
		}
		a.Cluster().SendClusterMessage(msg)
	}
}

func (a *App) GetAllStatuses() map[string]*model.Status {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return map[string]*model.Status{}
	}

	statusMap := map[string]*model.Status{}
	if userIDs, err := a.Srv().statusCache.Keys(); err == nil {
		for _, userID := range userIDs {
			status := a.GetStatusFromCache(userID)
			if status != nil {
				statusMap[userID] = status
			}
		}
	}
	return statusMap
}

func (a *App) GetStatusesByIDs(userIDs []string) (map[string]interface{}, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return map[string]interface{}{}, nil
	}

	statusMap := map[string]interface{}{}
	metrics := a.Metrics()

	missingUserIDs := []string{}
	for _, userID := range userIDs {
		var status *model.Status
		if err := a.Srv().statusCache.Get(userID, &status); err == nil {
			statusMap[userID] = status.Status
			if metrics != nil {
				metrics.IncrementMemCacheHitCounter("Status")
			}
		} else {
			missingUserIDs = append(missingUserIDs, userID)
			if metrics != nil {
				metrics.IncrementMemCacheMissCounter("Status")
			}
		}
	}

	if len(missingUserIDs) > 0 {
		statuses, err := a.Srv().Store.Status().GetByIDs(missingUserIDs)
		if err != nil {
			return nil, model.NewAppError("GetStatusesByIds", "app.status.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}

		for _, s := range statuses {
			a.AddStatusCacheSkipClusterSend(s)
			statusMap[s.UserID] = s.Status
		}

	}

	// For the case where the user does not have a row in the Status table and cache
	for _, userID := range missingUserIDs {
		if _, ok := statusMap[userID]; !ok {
			statusMap[userID] = model.StatusOffline
		}
	}

	return statusMap, nil
}

//GetUserStatusesByIds used by apiV4
func (a *App) GetUserStatusesByIDs(userIDs []string) ([]*model.Status, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return []*model.Status{}, nil
	}

	var statusMap []*model.Status
	metrics := a.Metrics()

	missingUserIDs := []string{}
	for _, userID := range userIDs {
		var status *model.Status
		if err := a.Srv().statusCache.Get(userID, &status); err == nil {
			statusMap = append(statusMap, status)
			if metrics != nil {
				metrics.IncrementMemCacheHitCounter("Status")
			}
		} else {
			missingUserIDs = append(missingUserIDs, userID)
			if metrics != nil {
				metrics.IncrementMemCacheMissCounter("Status")
			}
		}
	}

	if len(missingUserIDs) > 0 {
		statuses, err := a.Srv().Store.Status().GetByIDs(missingUserIDs)
		if err != nil {
			return nil, model.NewAppError("GetUserStatusesByIds", "app.status.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}

		for _, s := range statuses {
			a.AddStatusCacheSkipClusterSend(s)
		}

		statusMap = append(statusMap, statuses...)

	}

	// For the case where the user does not have a row in the Status table and cache
	// remove the existing ids from missingUserIds and then create a offline state for the missing ones
	// This also return the status offline for the non-existing Ids in the system
	for i := 0; i < len(missingUserIDs); i++ {
		missingUserID := missingUserIDs[i]
		for _, userMap := range statusMap {
			if missingUserID == userMap.UserID {
				missingUserIDs = append(missingUserIDs[:i], missingUserIDs[i+1:]...)
				i--
				break
			}
		}
	}
	for _, userID := range missingUserIDs {
		statusMap = append(statusMap, &model.Status{UserID: userID, Status: "offline"})
	}

	return statusMap, nil
}

// SetStatusLastActivityAt sets the last activity at for a user on the local app server and updates
// status to away if needed. Used by the WS to set status to away if an 'online' device disconnects
// while an 'away' device is still connected
func (a *App) SetStatusLastActivityAt(userID string, activityAt int64) {
	var status *model.Status
	var err *model.AppError
	if status, err = a.GetStatus(userID); err != nil {
		return
	}

	status.LastActivityAt = activityAt

	a.AddStatusCacheSkipClusterSend(status)
	a.SetStatusAwayIfNeeded(userID, false)
}

func (a *App) SetStatusOnline(userID string, manual bool) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	broadcast := false

	var oldStatus string = model.StatusOffline
	var oldTime int64
	var oldManual bool
	var status *model.Status
	var err *model.AppError

	if status, err = a.GetStatus(userID); err != nil {
		status = &model.Status{UserID: userID, Status: model.StatusOnline, Manual: false, LastActivityAt: model.GetMillis(), ActiveChannel: ""}
		broadcast = true
	} else {
		if status.Manual && !manual {
			return // manually set status always overrides non-manual one
		}

		if status.Status != model.StatusOnline {
			broadcast = true
		}

		oldStatus = status.Status
		oldTime = status.LastActivityAt
		oldManual = status.Manual

		status.Status = model.StatusOnline
		status.Manual = false // for "online" there's no manual setting
		status.LastActivityAt = model.GetMillis()
	}

	a.AddStatusCache(status)

	// Only update the database if the status has changed, the status has been manually set,
	// or enough time has passed since the previous action
	if status.Status != oldStatus || status.Manual != oldManual || status.LastActivityAt-oldTime > model.StatusMinUpdateTime {
		if broadcast {
			if err := a.Srv().Store.Status().SaveOrUpdate(status); err != nil {
				mlog.Warn("Failed to save status", mlog.String("user_id", userID), mlog.Err(err), mlog.String("user_id", userID))
			}
		} else {
			if err := a.Srv().Store.Status().UpdateLastActivityAt(status.UserID, status.LastActivityAt); err != nil {
				mlog.Error("Failed to save status", mlog.String("user_id", userID), mlog.Err(err), mlog.String("user_id", userID))
			}
		}
	}

	if broadcast {
		a.BroadcastStatus(status)
	}
}

func (a *App) BroadcastStatus(status *model.Status) {
	if a.Srv().Busy.IsBusy() {
		// this is considered a non-critical service and will be disabled when server busy.
		return
	}
	event := model.NewWebSocketEvent(model.WebsocketEventStatusChange, "", "", status.UserID, nil)
	event.Add("status", status.Status)
	event.Add("user_id", status.UserID)
	a.Publish(event)
}

func (a *App) SetStatusOffline(userID string, manual bool) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userID)
	if err == nil && status.Manual && !manual {
		return // manually set status always overrides non-manual one
	}

	status = &model.Status{UserID: userID, Status: model.StatusOffline, Manual: manual, LastActivityAt: model.GetMillis(), ActiveChannel: ""}

	a.SaveAndBroadcastStatus(status)
}

func (a *App) SetStatusAwayIfNeeded(userID string, manual bool) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userID)

	if err != nil {
		status = &model.Status{UserID: userID, Status: model.StatusOffline, Manual: manual, LastActivityAt: 0, ActiveChannel: ""}
	}

	if !manual && status.Manual {
		return // manually set status always overrides non-manual one
	}

	if !manual {
		if status.Status == model.StatusAway {
			return
		}

		if !a.IsUserAway(status.LastActivityAt) {
			return
		}
	}

	status.Status = model.StatusAway
	status.Manual = manual
	status.ActiveChannel = ""

	a.SaveAndBroadcastStatus(status)
}

// SetStatusDoNotDisturbTimed takes endtime in unix epoch format in UTC
// and sets status of given userId to dnd which will be restored back after endtime
func (a *App) SetStatusDoNotDisturbTimed(userID string, endtime int64) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userID)

	if err != nil {
		status = &model.Status{UserID: userID, Status: model.StatusOffline, Manual: false, LastActivityAt: 0, ActiveChannel: ""}
	}

	status.PrevStatus = status.Status
	status.Status = model.StatusDnd
	status.Manual = true

	status.DNDEndTime = endtime

	a.SaveAndBroadcastStatus(status)
}

func (a *App) SetStatusDoNotDisturb(userID string) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userID)

	if err != nil {
		status = &model.Status{UserID: userID, Status: model.StatusOffline, Manual: false, LastActivityAt: 0, ActiveChannel: ""}
	}

	status.Status = model.StatusDnd
	status.Manual = true

	a.SaveAndBroadcastStatus(status)
}

func (a *App) SaveAndBroadcastStatus(status *model.Status) {
	a.AddStatusCache(status)

	if err := a.Srv().Store.Status().SaveOrUpdate(status); err != nil {
		mlog.Warn("Failed to save status", mlog.String("user_id", status.UserID), mlog.Err(err))
	}

	a.BroadcastStatus(status)
}

func (a *App) SetStatusOutOfOffice(userID string) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userID)

	if err != nil {
		status = &model.Status{UserID: userID, Status: model.StatusOutOfOffice, Manual: false, LastActivityAt: 0, ActiveChannel: ""}
	}

	status.Status = model.StatusOutOfOffice
	status.Manual = true

	a.SaveAndBroadcastStatus(status)
}

func (a *App) GetStatusFromCache(userID string) *model.Status {
	var status *model.Status
	if err := a.Srv().statusCache.Get(userID, &status); err == nil {
		statusCopy := &model.Status{}
		*statusCopy = *status
		return statusCopy
	}

	return nil
}

func (a *App) GetStatus(userID string) (*model.Status, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return &model.Status{}, nil
	}

	status := a.GetStatusFromCache(userID)
	if status != nil {
		return status, nil
	}

	status, err := a.Srv().Store.Status().Get(userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetStatus", "app.status.get.missing.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetStatus", "app.status.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return status, nil
}

func (a *App) IsUserAway(lastActivityAt int64) bool {
	return model.GetMillis()-lastActivityAt >= *a.Config().TeamSettings.UserStatusAwayTimeout*1000
}

// UpdateDNDStatusOfUsers is a recurring task which is started when server starts
// which unsets dnd status of users if needed and saves and broadcasts it
func (a *App) UpdateDNDStatusOfUsers() {
	mlog.Debug("UpdateDNDStatusOfUsers: scheduled run started")
	statuses, err := a.UpdateExpiredDNDStatuses()
	if err != nil {
		mlog.Warn("Failed to fetch dnd statues from store", mlog.String("err", err.Error()))
		return
	}
	for i := range statuses {
		a.AddStatusCache(statuses[i])
		a.BroadcastStatus(statuses[i])
	}
}

func (a *App) SetCustomStatus(userID string, cs *model.CustomStatus) *model.AppError {
	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	user.SetCustomStatus(cs)
	_, updateErr := a.UpdateUser(user, true)
	if updateErr != nil {
		return err
	}

	if err := a.addRecentCustomStatus(userID, cs); err != nil {
		a.Log().Error("Can't add recent custom status for", mlog.String("userID", userID), mlog.Err(err))
	}

	return nil
}

func (a *App) RemoveCustomStatus(userID string) *model.AppError {
	user, err := a.GetUser(userID)
	if err != nil {
		return err
	}

	user.ClearCustomStatus()
	_, updateErr := a.UpdateUser(user, true)
	if updateErr != nil {
		return err
	}

	return nil
}

func (a *App) addRecentCustomStatus(userID string, status *model.CustomStatus) *model.AppError {
	var newRCS *model.RecentCustomStatuses

	pref, err := a.GetPreferenceByCategoryAndNameForUser(userID, model.PreferenceCategoryCustomStatus, model.PreferenceNameRecentCustomStatuses)
	if err != nil || pref.Value == "" {
		newRCS = &model.RecentCustomStatuses{*status}
	} else {
		existingRCS := model.RecentCustomStatusesFromJSON(strings.NewReader(pref.Value))
		newRCS = existingRCS.Add(status)
	}

	pref = &model.Preference{
		UserID:   userID,
		Category: model.PreferenceCategoryCustomStatus,
		Name:     model.PreferenceNameRecentCustomStatuses,
		Value:    newRCS.ToJSON(),
	}
	if err := a.UpdatePreferences(userID, model.Preferences{*pref}); err != nil {
		return err
	}

	return nil
}

func (a *App) RemoveRecentCustomStatus(userID string, status *model.CustomStatus) *model.AppError {
	pref, err := a.GetPreferenceByCategoryAndNameForUser(userID, model.PreferenceCategoryCustomStatus, model.PreferenceNameRecentCustomStatuses)
	if err != nil {
		return err
	}

	if pref.Value == "" {
		return model.NewAppError("RemoveRecentCustomStatus", "api.custom_status.recent_custom_statuses.delete.app_error", nil, "", http.StatusBadRequest)
	}

	existingRCS := model.RecentCustomStatusesFromJSON(strings.NewReader(pref.Value))
	if !existingRCS.Contains(status) {
		return model.NewAppError("RemoveRecentCustomStatus", "api.custom_status.recent_custom_statuses.delete.app_error", nil, "", http.StatusBadRequest)
	}

	newRCS := existingRCS.Remove(status)
	pref.Value = newRCS.ToJSON()

	if err := a.UpdatePreferences(userID, model.Preferences{*pref}); err != nil {
		return err
	}

	return nil
}
