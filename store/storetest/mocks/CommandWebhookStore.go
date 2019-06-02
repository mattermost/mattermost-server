// Code generated by mockery v1.0.0. DO NOT EDIT.

// Regenerate this file using `make store-mocks`.

package mocks

import mock "github.com/stretchr/testify/mock"
import model "github.com/mattermost/mattermost-server/model"
import store "github.com/tomasmik/mattermost-server/store"

// CommandWebhookStore is an autogenerated mock type for the CommandWebhookStore type
type CommandWebhookStore struct {
	mock.Mock
}

// Cleanup provides a mock function with given fields:
func (_m *CommandWebhookStore) Cleanup() {
	_m.Called()
}

// Get provides a mock function with given fields: id
func (_m *CommandWebhookStore) Get(id string) store.StoreChannel {
	ret := _m.Called(id)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// Save provides a mock function with given fields: webhook
func (_m *CommandWebhookStore) Save(webhook *model.CommandWebhook) store.StoreChannel {
	ret := _m.Called(webhook)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(*model.CommandWebhook) store.StoreChannel); ok {
		r0 = rf(webhook)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// TryUse provides a mock function with given fields: id, limit
func (_m *CommandWebhookStore) TryUse(id string, limit int) store.StoreChannel {
	ret := _m.Called(id, limit)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, int) store.StoreChannel); ok {
		r0 = rf(id, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}
