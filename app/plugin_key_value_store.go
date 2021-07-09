// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

func getKeyHash(key string) string {
	hash := sha256.New()
	hash.Write([]byte(key))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func (a *App) SetPluginKey(pluginID string, key string, value []byte) *model.AppError {
	return a.SetPluginKeyWithExpiry(pluginID, key, value, 0)
}

func (a *App) SetPluginKeyWithExpiry(pluginID string, key string, value []byte, expireInSeconds int64) *model.AppError {
	options := model.PluginKVSetOptions{
		ExpireInSeconds: expireInSeconds,
	}
	_, err := a.SetPluginKeyWithOptions(pluginID, key, value, options)
	return err
}

func (a *App) CompareAndSetPluginKey(pluginID string, key string, oldValue, newValue []byte) (bool, *model.AppError) {
	options := model.PluginKVSetOptions{
		Atomic:   true,
		OldValue: oldValue,
	}
	return a.SetPluginKeyWithOptions(pluginID, key, newValue, options)
}

func (a *App) SetPluginKeyWithOptions(pluginID string, key string, value []byte, options model.PluginKVSetOptions) (bool, *model.AppError) {
	if err := options.IsValid(); err != nil {
		mlog.Debug("Failed to set plugin key value with options", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
		return false, err
	}

	updated, err := a.Srv().Store.Plugin().SetWithOptions(pluginID, key, value, options)
	if err != nil {
		mlog.Error("Failed to set plugin key value with options", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return false, appErr
		default:
			return false, model.NewAppError("SetPluginKeyWithOptions", "app.plugin_store.save.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	// Clean up a previous entry using the hashed key, if it exists.
	if err := a.Srv().Store.Plugin().Delete(pluginID, getKeyHash(key)); err != nil {
		mlog.Warn("Failed to clean up previously hashed plugin key value", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
	}

	return updated, nil
}

func (a *App) CompareAndDeletePluginKey(pluginID string, key string, oldValue []byte) (bool, *model.AppError) {
	kv := &model.PluginKeyValue{
		PluginID: pluginID,
		Key:      key,
	}

	deleted, err := a.Srv().Store.Plugin().CompareAndDelete(kv, oldValue)
	if err != nil {
		mlog.Error("Failed to compare and delete plugin key value", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return deleted, appErr
		default:
			return false, model.NewAppError("CompareAndDeletePluginKey", "app.plugin_store.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	// Clean up a previous entry using the hashed key, if it exists.
	if err := a.Srv().Store.Plugin().Delete(pluginID, getKeyHash(key)); err != nil {
		mlog.Warn("Failed to clean up previously hashed plugin key value", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
	}

	return deleted, nil
}

func (a *App) GetPluginKey(pluginID string, key string) ([]byte, *model.AppError) {
	if kv, err := a.Srv().Store.Plugin().Get(pluginID, key); err == nil {
		return kv.Value, nil
	} else if nfErr := new(store.ErrNotFound); !errors.As(err, &nfErr) {
		mlog.Error("Failed to query plugin key value", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
		return nil, model.NewAppError("GetPluginKey", "app.plugin_store.get.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	// Lookup using the hashed version of the key for keys written prior to v5.6.
	if kv, err := a.Srv().Store.Plugin().Get(pluginID, getKeyHash(key)); err == nil {
		return kv.Value, nil
	} else if nfErr := new(store.ErrNotFound); !errors.As(err, &nfErr) {
		mlog.Error("Failed to query plugin key value using hashed key", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
		return nil, model.NewAppError("GetPluginKey", "app.plugin_store.get.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil, nil
}

func (a *App) DeletePluginKey(pluginID string, key string) *model.AppError {
	if err := a.Srv().Store.Plugin().Delete(pluginID, getKeyHash(key)); err != nil {
		mlog.Error("Failed to delete plugin key value", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
		return model.NewAppError("DeletePluginKey", "app.plugin_store.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	// Also delete the key without hashing
	if err := a.Srv().Store.Plugin().Delete(pluginID, key); err != nil {
		mlog.Error("Failed to delete plugin key value using hashed key", mlog.String("plugin_id", pluginID), mlog.String("key", key), mlog.Err(err))
		return model.NewAppError("DeletePluginKey", "app.plugin_store.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (a *App) DeleteAllKeysForPlugin(pluginID string) *model.AppError {
	if err := a.Srv().Store.Plugin().DeleteAllForPlugin(pluginID); err != nil {
		mlog.Error("Failed to delete all plugin key values", mlog.String("plugin_id", pluginID), mlog.Err(err))
		return model.NewAppError("DeleteAllKeysForPlugin", "app.plugin_store.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (a *App) DeleteAllExpiredPluginKeys() *model.AppError {
	if a.Srv() == nil {
		return nil
	}

	if err := a.Srv().Store.Plugin().DeleteAllExpired(); err != nil {
		mlog.Error("Failed to delete all expired plugin key values", mlog.Err(err))
		return model.NewAppError("DeleteAllExpiredPluginKeys", "app.plugin_store.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (a *App) ListPluginKeys(pluginID string, page, perPage int) ([]string, *model.AppError) {
	data, err := a.Srv().Store.Plugin().List(pluginID, page*perPage, perPage)

	if err != nil {
		mlog.Error("Failed to list plugin key values", mlog.Int("page", page), mlog.Int("perPage", perPage), mlog.Err(err))
		return nil, model.NewAppError("ListPluginKeys", "app.plugin_store.list.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return data, nil
}
