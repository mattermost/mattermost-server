// Code generated by mockery v1.0.0. DO NOT EDIT.

// Regenerate this file using `make plugin-mocks`.

package plugintest

import mock "github.com/stretchr/testify/mock"
import model "github.com/mattermost/mattermost-server/model"

// Helpers is an autogenerated mock type for the Helpers type
type Helpers struct {
	mock.Mock
}

// EnsureBot provides a mock function with given fields: bot
func (_m *Helpers) EnsureBot(bot *model.Bot) (string, error) {
	ret := _m.Called(bot)

	var r0 string
	if rf, ok := ret.Get(0).(func(*model.Bot) string); ok {
		r0 = rf(bot)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*model.Bot) error); ok {
		r1 = rf(bot)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// KVCompareAndSetJSON provides a mock function with given fields: key, oldValue, newValue
func (_m *Helpers) KVCompareAndSetJSON(key string, oldValue interface{}, newValue interface{}) error {
	ret := _m.Called(key, oldValue, newValue)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, interface{}, interface{}) error); ok {
		r0 = rf(key, oldValue, newValue)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// KVGetJSON provides a mock function with given fields: key, value
func (_m *Helpers) KVGetJSON(key string, value interface{}) error {
	ret := _m.Called(key, value)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, interface{}) error); ok {
		r0 = rf(key, value)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// KVSetJSON provides a mock function with given fields: key, value
func (_m *Helpers) KVSetJSON(key string, value interface{}) error {
	ret := _m.Called(key, value)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, interface{}) error); ok {
		r0 = rf(key, value)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// KVSetWithExpiryJSON provides a mock function with given fields: key, value, expireInSeconds
func (_m *Helpers) KVSetWithExpiryJSON(key string, value interface{}, expireInSeconds int64) error {
	ret := _m.Called(key, value, expireInSeconds)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, interface{}, int64) error); ok {
		r0 = rf(key, value, expireInSeconds)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
