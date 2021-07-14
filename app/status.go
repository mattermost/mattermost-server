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

	"time"
)

func (a *App) AddStatusCacheSkipClusterSend(status *model.Status) {
	a.Srv().statusCache.Set(status.UserId, status)
}

func (a *App) AddStatusCache(status *model.Status) {
	a.AddStatusCacheSkipClusterSend(status)

	if a.Cluster() != nil {
		msg := &model.ClusterMessage{
			Event:    model.CLUSTER_EVENT_UPDATE_STATUS,
			SendType: model.CLUSTER_SEND_BEST_EFFORT,
			Data:     status.ToClusterJson(),
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

func (a *App) GetStatusesByIds(userIDs []string) (map[string]interface{}, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return map[string]interface{}{}, nil
	}

	statusMap := map[string]interface{}{}
	metrics := a.Metrics()

	missingUserIds := []string{}
	for _, userID := range userIDs {
		var status *model.Status
		if err := a.Srv().statusCache.Get(userID, &status); err == nil {
			statusMap[userID] = status.Status
			if metrics != nil {
				metrics.IncrementMemCacheHitCounter("Status")
			}
		} else {
			missingUserIds = append(missingUserIds, userID)
			if metrics != nil {
				metrics.IncrementMemCacheMissCounter("Status")
			}
		}
	}

	if len(missingUserIds) > 0 {
		statuses, err := a.Srv().Store.Status().GetByIds(missingUserIds)
		if err != nil {
			return nil, model.NewAppError("GetStatusesByIds", "app.status.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}

		for _, s := range statuses {
			a.AddStatusCacheSkipClusterSend(s)
			statusMap[s.UserId] = s.Status
		}

	}

	// For the case where the user does not have a row in the Status table and cache
	for _, userID := range missingUserIds {
		if _, ok := statusMap[userID]; !ok {
			statusMap[userID] = model.STATUS_OFFLINE
		}
	}

	return statusMap, nil
}

//GetUserStatusesByIds used by apiV4
func (a *App) GetUserStatusesByIds(userIDs []string) ([]*model.Status, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return []*model.Status{}, nil
	}

	var statusMap []*model.Status
	metrics := a.Metrics()

	missingUserIds := []string{}
	for _, userID := range userIDs {
		var status *model.Status
		if err := a.Srv().statusCache.Get(userID, &status); err == nil {
			statusMap = append(statusMap, status)
			if metrics != nil {
				metrics.IncrementMemCacheHitCounter("Status")
			}
		} else {
			missingUserIds = append(missingUserIds, userID)
			if metrics != nil {
				metrics.IncrementMemCacheMissCounter("Status")
			}
		}
	}

	if len(missingUserIds) > 0 {
		statuses, err := a.Srv().Store.Status().GetByIds(missingUserIds)
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
	for i := 0; i < len(missingUserIds); i++ {
		missingUserId := missingUserIds[i]
		for _, userMap := range statusMap {
			if missingUserId == userMap.UserId {
				missingUserIds = append(missingUserIds[:i], missingUserIds[i+1:]...)
				i--
				break
			}
		}
	}
	for _, userID := range missingUserIds {
		statusMap = append(statusMap, &model.Status{UserId: userID, Status: "offline"})
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

	var oldStatus string = model.STATUS_OFFLINE
	var oldTime int64
	var oldManual bool
	var status *model.Status
	var err *model.AppError

	if status, err = a.GetStatus(userID); err != nil {
		status = &model.Status{UserId: userID, Status: model.STATUS_ONLINE, Manual: false, LastActivityAt: model.GetMillis(), ActiveChannel: ""}
		broadcast = true
	} else {
		if status.Manual && !manual {
			return // manually set status always overrides non-manual one
		}

		if status.Status != model.STATUS_ONLINE {
			broadcast = true
		}

		oldStatus = status.Status
		oldTime = status.LastActivityAt
		oldManual = status.Manual

		status.Status = model.STATUS_ONLINE
		status.Manual = false // for "online" there's no manual setting
		status.LastActivityAt = model.GetMillis()
	}

	a.AddStatusCache(status)

	// Only update the database if the status has changed, the status has been manually set,
	// or enough time has passed since the previous action
	if status.Status != oldStatus || status.Manual != oldManual || status.LastActivityAt-oldTime > model.STATUS_MIN_UPDATE_TIME {
		if broadcast {
			if err := a.Srv().Store.Status().SaveOrUpdate(status); err != nil {
				mlog.Warn("Failed to save status", mlog.String("user_id", userID), mlog.Err(err), mlog.String("user_id", userID))
			}
		} else {
			if err := a.Srv().Store.Status().UpdateLastActivityAt(status.UserId, status.LastActivityAt); err != nil {
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
	event := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_STATUS_CHANGE, "", "", status.UserId, nil)
	event.Add("status", status.Status)
	event.Add("user_id", status.UserId)
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

	var MONS = status.MondayStart
	var MONE = status.MondayEnd
	var TUES = status.TuesdayStart
	var TUEE = status.TuesdayEnd
	var WEDS = status.WednesdayStart
	var WEDE = status.WednesdayEnd
	var THUS = status.ThursdayStart
	var THUE = status.ThursdayEnd
	var FRIS = status.FridayStart
	var FRIE = status.FridayEnd
	var SATS = status.SaturdayStart
	var SATE = status.SaturdayEnd
	var SUNS = status.SundayStart
	var SUNE = status.SundayEnd
	var MODE = status.Mode

	status = &model.Status{UserId: userID, Status: model.STATUS_OFFLINE, Manual: manual, LastActivityAt: model.GetMillis(), ActiveChannel: ""}
	status.MondayStart = MONS
	status.MondayEnd = MONE
	status.TuesdayStart = TUES
	status.TuesdayEnd = TUEE
	status.WednesdayStart = WEDS
	status.WednesdayEnd = WEDE
	status.ThursdayStart = THUS
	status.ThursdayEnd = THUE
	status.FridayStart = FRIS
	status.FridayEnd = FRIE
	status.SaturdayStart = SATS
	status.SaturdayEnd = SATE
	status.SundayStart = SUNS
	status.SundayEnd = SUNE
	status.Mode = MODE

	a.SaveAndBroadcastStatus(status)
}

func (a *App) SetStatusAwayIfNeeded(userID string, manual bool) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userID)

	if err != nil {
		status = &model.Status{UserId: userID, Status: model.STATUS_OFFLINE, Manual: manual, LastActivityAt: 0, ActiveChannel: ""}
	}

	if !manual && status.Manual {
		return // manually set status always overrides non-manual one
	}

	if !manual {
		if status.Status == model.STATUS_AWAY {
			return
		}

		if !a.IsUserAway(status.LastActivityAt) {
			return
		}
	}

	status.Status = model.STATUS_AWAY
	status.Manual = manual
	status.ActiveChannel = ""

	a.SaveAndBroadcastStatus(status)
}

func (a *App) SetStatusDoNotDisturbSchedule(userId, currentDayOfTheWeek, currentTime string) {
	currentStatus, err := a.GetStatus(userId)

	if err != nil {
		return
	}
	startTime := ""
	endTime := ""

	switch currentDayOfTheWeek {
	case "Mon":
		startTime = currentStatus.MondayStart
		endTime = currentStatus.MondayEnd
	case "Tue":
		startTime = currentStatus.TuesdayStart
		endTime = currentStatus.TuesdayEnd
	case "Wed":
		startTime = currentStatus.WednesdayStart
		endTime = currentStatus.WednesdayEnd
	case "Thu":
		startTime = currentStatus.ThursdayStart
		endTime = currentStatus.ThursdayEnd
	case "Fri":
		startTime = currentStatus.FridayStart
		endTime = currentStatus.FridayEnd
	case "Sat":
		startTime = currentStatus.SaturdayStart
		endTime = currentStatus.SaturdayEnd
	case "Sun":
		startTime = currentStatus.SundayStart
		endTime = currentStatus.SundayEnd
	}

	if startTime == "" && endTime == "" {
		return
	}

	newLayout := "15:04"
	check, _ := time.Parse(newLayout, currentTime)
	start, _ := time.Parse(newLayout, startTime)
	end, _ := time.Parse(newLayout, endTime)

	if a.InTimeSpan(start, end, check) {
		a.SetStatusOnline(userId, true)
	} else {
		a.SetStatusDoNotDisturbScheduled(userId)
	}
}

func (a *App) InTimeSpan(start, end, check time.Time) bool {
	if check.After(end) {
		return false
	}
	if end.After(start) {
		return check.After(start)
	}
	return check.Before(start)
}

func (a *App) SetStatusSchedule(userId, mondayStart, mondayEnd, tuesdayStart, tuesdayEnd, wednesdayStart, wednesdayEnd, thursdayStart, thursdayEnd, fridayStart, fridayEnd, saturdayStart, saturdayEnd, sundayStart, sundayEnd string, mode int64) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}
	status, err := a.GetStatus(userId)

	if err != nil {
		return
	}

	status.MondayStart = mondayStart
	status.MondayEnd = mondayEnd
	status.TuesdayStart = tuesdayStart
	status.TuesdayEnd = tuesdayEnd
	status.WednesdayStart = wednesdayStart
	status.WednesdayEnd = wednesdayEnd
	status.ThursdayStart = thursdayStart
	status.ThursdayEnd = thursdayEnd
	status.FridayStart = fridayStart
	status.FridayEnd = fridayEnd
	status.SaturdayStart = saturdayStart
	status.SaturdayEnd = saturdayEnd
	status.SundayStart = sundayStart
	status.SundayEnd = sundayEnd
	status.Mode = mode

	a.SaveAndBroadcastStatus(status)
}

func (a *App) SetStatusDoNotDisturbScheduled(userID string) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userID)

	if err != nil {
		return
	}

	status.Status = model.STATUS_DND
	status.Manual = true

	a.SaveAndBroadcastStatus(status)
}

// SetStatusDoNotDisturbTimed takes endtime in unix epoch format in UTC
// and sets status of given userId to dnd which will be restored back after endtime
func (a *App) SetStatusDoNotDisturbTimed(userId string, endtime int64) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userId)

	if err != nil {
		status = &model.Status{UserId: userId, Status: model.STATUS_OFFLINE, Manual: false, LastActivityAt: 0, ActiveChannel: ""}
	}

	status.PrevStatus = status.Status
	status.Status = model.STATUS_DND
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
		status = &model.Status{UserId: userID, Status: model.STATUS_OFFLINE, Manual: false, LastActivityAt: 0, ActiveChannel: ""}
	}

	status.Status = model.STATUS_DND
	status.Manual = true

	a.SaveAndBroadcastStatus(status)
}

func (a *App) SaveAndBroadcastStatus(status *model.Status) {
	a.AddStatusCache(status)

	if err := a.Srv().Store.Status().SaveOrUpdate(status); err != nil {
		mlog.Warn("Failed to save status", mlog.String("user_id", status.UserId), mlog.Err(err))
	}

	a.BroadcastStatus(status)
}

func (a *App) SetStatusOutOfOffice(userID string) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}

	status, err := a.GetStatus(userID)

	if err != nil {
		status = &model.Status{UserId: userID, Status: model.STATUS_OUT_OF_OFFICE, Manual: false, LastActivityAt: 0, ActiveChannel: ""}
	}

	status.Status = model.STATUS_OUT_OF_OFFICE
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

	pref, err := a.GetPreferenceByCategoryAndNameForUser(userID, model.PREFERENCE_CATEGORY_CUSTOM_STATUS, model.PREFERENCE_NAME_RECENT_CUSTOM_STATUSES)
	if err != nil || pref.Value == "" {
		newRCS = &model.RecentCustomStatuses{*status}
	} else {
		existingRCS := model.RecentCustomStatusesFromJson(strings.NewReader(pref.Value))
		newRCS = existingRCS.Add(status)
	}

	pref = &model.Preference{
		UserId:   userID,
		Category: model.PREFERENCE_CATEGORY_CUSTOM_STATUS,
		Name:     model.PREFERENCE_NAME_RECENT_CUSTOM_STATUSES,
		Value:    newRCS.ToJson(),
	}
	if err := a.UpdatePreferences(userID, model.Preferences{*pref}); err != nil {
		return err
	}

	return nil
}

func (a *App) RemoveRecentCustomStatus(userID string, status *model.CustomStatus) *model.AppError {
	pref, err := a.GetPreferenceByCategoryAndNameForUser(userID, model.PREFERENCE_CATEGORY_CUSTOM_STATUS, model.PREFERENCE_NAME_RECENT_CUSTOM_STATUSES)
	if err != nil {
		return err
	}

	if pref.Value == "" {
		return model.NewAppError("RemoveRecentCustomStatus", "api.custom_status.recent_custom_statuses.delete.app_error", nil, "", http.StatusBadRequest)
	}

	existingRCS := model.RecentCustomStatusesFromJson(strings.NewReader(pref.Value))
	if !existingRCS.Contains(status) {
		return model.NewAppError("RemoveRecentCustomStatus", "api.custom_status.recent_custom_statuses.delete.app_error", nil, "", http.StatusBadRequest)
	}

	newRCS := existingRCS.Remove(status)
	pref.Value = newRCS.ToJson()

	if err := a.UpdatePreferences(userID, model.Preferences{*pref}); err != nil {
		return err
	}

	return nil
}
