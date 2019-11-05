// Copyright (c) 2016-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package app

import (
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-server/model"
)

func (a *App) GetPreferencesForUser(userId string) (model.Preferences, *model.AppError) {
	preferences, err := a.Srv.Store.Preference().GetAll(userId)
	if err != nil {
		err.StatusCode = http.StatusBadRequest
		return nil, err
	}
	return preferences, nil
}

func (a *App) GetPreferenceByCategoryForUser(userId string, category string) (model.Preferences, *model.AppError) {
	preferences, err := a.Srv.Store.Preference().GetCategory(userId, category)
	if err != nil {
		err.StatusCode = http.StatusBadRequest
		return nil, err
	}
	if len(preferences) == 0 {
		err := model.NewAppError("getPreferenceCategory", "api.preference.preferences_category.get.app_error", nil, "", http.StatusNotFound)
		return nil, err
	}
	return preferences, nil
}

func (a *App) GetPreferenceByCategoryAndNameForUser(userId string, category string, preferenceName string) (*model.Preference, *model.AppError) {
	res, err := a.Srv.Store.Preference().Get(userId, category, preferenceName)
	if err != nil {
		err.StatusCode = http.StatusBadRequest
		return nil, err
	}
	return res, nil
}

func (a *App) UpdatePreferences(userId string, preferences model.Preferences) *model.AppError {
	for _, preference := range preferences {
		if userId != preference.UserId {
			return model.NewAppError("savePreferences", "api.preference.update_preferences.set.app_error", nil,
				"userId="+userId+", preference.UserId="+preference.UserId, http.StatusForbidden)
		}
	}

	if err := a.Srv.Store.Preference().Save(&preferences); err != nil {
		err.StatusCode = http.StatusBadRequest
		return err
	}

	message := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_PREFERENCES_CHANGED, "", "", userId, nil)
	message.Add("preferences", preferences.ToJson())
	a.Publish(message)

	return nil
}

func (a *App) UpdatePreferencesForAll(preferences model.Preferences) *model.AppError {
	if err := a.Srv.Store.Preference().SaveForAll(preferences); err != nil {
		err.StatusCode = http.StatusBadRequest
		return err
	}

	fmt.Printf("\n\n\n\n\nAM I GOING TO PUBLISH WEBSOCKET EVENT?\n\n\n\n\n\n?")
	message := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_PREFERENCES_CHANGED, "", "", a.Session.UserId, nil)
	message.Add("preferences", preferences.ToJson())
	a.Publish(message)

	return nil
}

func (a *App) DeletePreferences(userId string, preferences model.Preferences) *model.AppError {
	for _, preference := range preferences {
		if userId != preference.UserId {
			err := model.NewAppError("deletePreferences", "api.preference.delete_preferences.delete.app_error", nil,
				"userId="+userId+", preference.UserId="+preference.UserId, http.StatusForbidden)
			return err
		}
	}

	for _, preference := range preferences {
		if err := a.Srv.Store.Preference().Delete(userId, preference.Category, preference.Name); err != nil {
			err.StatusCode = http.StatusBadRequest
			return err
		}
	}

	message := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_PREFERENCES_DELETED, "", "", userId, nil)
	message.Add("preferences", preferences.ToJson())
	a.Publish(message)

	return nil
}
