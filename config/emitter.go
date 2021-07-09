// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

// Listener is a callback function invoked when the configuration changes.
type Listener func(oldCfg, newCfg *model.Config)

// emitter enables threadsafe registration and broadcasting to configuration listeners
type emitter struct {
	listeners sync.Map
}

// AddListener adds a callback function to invoke when the configuration is modified.
func (e *emitter) AddListener(listener Listener) string {
	id := model.NewID()
	e.listeners.Store(id, listener)
	return id
}

// RemoveListener removes a callback function using an id returned from AddListener.
func (e *emitter) RemoveListener(id string) {
	e.listeners.Delete(id)
}

// invokeConfigListeners synchronously notifies all listeners about the configuration change.
func (e *emitter) invokeConfigListeners(oldCfg, newCfg *model.Config) {
	e.listeners.Range(func(key, value interface{}) bool {
		listener := value.(Listener)
		listener(oldCfg, newCfg)
		return true
	})
}

// srcEmitter enables threadsafe registration and broadcasting to configuration listeners
type logSrcEmitter struct {
	listeners sync.Map
}

// AddListener adds a callback function to invoke when the configuration is modified.
func (e *logSrcEmitter) AddListener(listener LogSrcListener) string {
	id := model.NewID()
	e.listeners.Store(id, listener)
	return id
}

// RemoveListener removes a callback function using an id returned from AddListener.
func (e *logSrcEmitter) RemoveListener(id string) {
	e.listeners.Delete(id)
}

// invokeConfigListeners synchronously notifies all listeners about the configuration change.
func (e *logSrcEmitter) invokeConfigListeners(oldCfg, newCfg mlog.LogTargetCfg) {
	e.listeners.Range(func(key, value interface{}) bool {
		listener := value.(LogSrcListener)
		listener(oldCfg, newCfg)
		return true
	})
}
