// Code generated by mockery v1.0.0. DO NOT EDIT.

// Regenerate this file using `make store-mocks`.

package mocks

import (
	model "github.com/mattermost/mattermost-server/v5/model"
	mock "github.com/stretchr/testify/mock"
)

// RetentionPolicyStore is an autogenerated mock type for the RetentionPolicyStore type
type RetentionPolicyStore struct {
	mock.Mock
}

// AddChannels provides a mock function with given fields: policyId, channelIds
func (_m *RetentionPolicyStore) AddChannels(policyId string, channelIds []string) error {
	ret := _m.Called(policyId, channelIds)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []string) error); ok {
		r0 = rf(policyId, channelIds)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddTeams provides a mock function with given fields: policyId, teamIds
func (_m *RetentionPolicyStore) AddTeams(policyId string, teamIds []string) error {
	ret := _m.Called(policyId, teamIds)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []string) error); ok {
		r0 = rf(policyId, teamIds)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Delete provides a mock function with given fields: id
func (_m *RetentionPolicyStore) Delete(id string) error {
	ret := _m.Called(id)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: id
func (_m *RetentionPolicyStore) Get(id string) (*model.RetentionPolicyEnriched, error) {
	ret := _m.Called(id)

	var r0 *model.RetentionPolicyEnriched
	if rf, ok := ret.Get(0).(func(string) *model.RetentionPolicyEnriched); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.RetentionPolicyEnriched)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAll provides a mock function with given fields:
func (_m *RetentionPolicyStore) GetAll() ([]*model.RetentionPolicyEnriched, error) {
	ret := _m.Called()

	var r0 []*model.RetentionPolicyEnriched
	if rf, ok := ret.Get(0).(func() []*model.RetentionPolicyEnriched); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.RetentionPolicyEnriched)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAllWithCounts provides a mock function with given fields:
func (_m *RetentionPolicyStore) GetAllWithCounts() ([]*model.RetentionPolicyWithCounts, error) {
	ret := _m.Called()

	var r0 []*model.RetentionPolicyWithCounts
	if rf, ok := ret.Get(0).(func() []*model.RetentionPolicyWithCounts); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.RetentionPolicyWithCounts)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Patch provides a mock function with given fields: patch
func (_m *RetentionPolicyStore) Patch(patch *model.RetentionPolicyWithApplied) (*model.RetentionPolicyEnriched, error) {
	ret := _m.Called(patch)

	var r0 *model.RetentionPolicyEnriched
	if rf, ok := ret.Get(0).(func(*model.RetentionPolicyWithApplied) *model.RetentionPolicyEnriched); ok {
		r0 = rf(patch)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.RetentionPolicyEnriched)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*model.RetentionPolicyWithApplied) error); ok {
		r1 = rf(patch)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemoveChannels provides a mock function with given fields: policyId, channelIds
func (_m *RetentionPolicyStore) RemoveChannels(policyId string, channelIds []string) error {
	ret := _m.Called(policyId, channelIds)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []string) error); ok {
		r0 = rf(policyId, channelIds)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveInvalidRows provides a mock function with given fields:
func (_m *RetentionPolicyStore) RemoveInvalidRows() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveTeams provides a mock function with given fields: policyId, teamIds
func (_m *RetentionPolicyStore) RemoveTeams(policyId string, teamIds []string) error {
	ret := _m.Called(policyId, teamIds)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []string) error); ok {
		r0 = rf(policyId, teamIds)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Save provides a mock function with given fields: policy
func (_m *RetentionPolicyStore) Save(policy *model.RetentionPolicyWithApplied) (*model.RetentionPolicyEnriched, error) {
	ret := _m.Called(policy)

	var r0 *model.RetentionPolicyEnriched
	if rf, ok := ret.Get(0).(func(*model.RetentionPolicyWithApplied) *model.RetentionPolicyEnriched); ok {
		r0 = rf(policy)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.RetentionPolicyEnriched)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*model.RetentionPolicyWithApplied) error); ok {
		r1 = rf(policy)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Update provides a mock function with given fields: update
func (_m *RetentionPolicyStore) Update(update *model.RetentionPolicyWithApplied) (*model.RetentionPolicyEnriched, error) {
	ret := _m.Called(update)

	var r0 *model.RetentionPolicyEnriched
	if rf, ok := ret.Get(0).(func(*model.RetentionPolicyWithApplied) *model.RetentionPolicyEnriched); ok {
		r0 = rf(update)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.RetentionPolicyEnriched)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*model.RetentionPolicyWithApplied) error); ok {
		r1 = rf(update)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
