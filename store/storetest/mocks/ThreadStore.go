// Code generated by mockery v1.0.0. DO NOT EDIT.

// Regenerate this file using `make store-mocks`.

package mocks

import (
	model "github.com/mattermost/mattermost-server/v5/model"
	mock "github.com/stretchr/testify/mock"
)

// ThreadStore is an autogenerated mock type for the ThreadStore type
type ThreadStore struct {
	mock.Mock
}

// CollectThreadsWithNewerReplies provides a mock function with given fields: userId, channelIds, timestamp
func (_m *ThreadStore) CollectThreadsWithNewerReplies(userId string, channelIds []string, timestamp int64) ([]string, error) {
	ret := _m.Called(userId, channelIds, timestamp)

	var r0 []string
	if rf, ok := ret.Get(0).(func(string, []string, int64) []string); ok {
		r0 = rf(userId, channelIds, timestamp)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, []string, int64) error); ok {
		r1 = rf(userId, channelIds, timestamp)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Delete provides a mock function with given fields: postID
func (_m *ThreadStore) Delete(postID string) error {
	ret := _m.Called(postID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(postID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteMembershipForUser provides a mock function with given fields: userId, postID
func (_m *ThreadStore) DeleteMembershipForUser(userId string, postID string) error {
	ret := _m.Called(userId, postID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string) error); ok {
		r0 = rf(userId, postID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: id
func (_m *ThreadStore) Get(id string) (*model.Thread, error) {
	ret := _m.Called(id)

	var r0 *model.Thread
	if rf, ok := ret.Get(0).(func(string) *model.Thread); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Thread)
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

// GetMembershipForUser provides a mock function with given fields: userId, postID
func (_m *ThreadStore) GetMembershipForUser(userId string, postID string) (*model.ThreadMembership, error) {
	ret := _m.Called(userId, postID)

	var r0 *model.ThreadMembership
	if rf, ok := ret.Get(0).(func(string, string) *model.ThreadMembership); ok {
		r0 = rf(userId, postID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.ThreadMembership)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(userId, postID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMembershipsForUser provides a mock function with given fields: userId, teamID
func (_m *ThreadStore) GetMembershipsForUser(userId string, teamID string) ([]*model.ThreadMembership, error) {
	ret := _m.Called(userId, teamID)

	var r0 []*model.ThreadMembership
	if rf, ok := ret.Get(0).(func(string, string) []*model.ThreadMembership); ok {
		r0 = rf(userId, teamID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.ThreadMembership)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(userId, teamID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPosts provides a mock function with given fields: threadID, since
func (_m *ThreadStore) GetPosts(threadID string, since int64) ([]*model.Post, error) {
	ret := _m.Called(threadID, since)

	var r0 []*model.Post
	if rf, ok := ret.Get(0).(func(string, int64) []*model.Post); ok {
		r0 = rf(threadID, since)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.Post)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, int64) error); ok {
		r1 = rf(threadID, since)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetThreadFollowers provides a mock function with given fields: threadID
func (_m *ThreadStore) GetThreadFollowers(threadID string) ([]string, error) {
	ret := _m.Called(threadID)

	var r0 []string
	if rf, ok := ret.Get(0).(func(string) []string); ok {
		r0 = rf(threadID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(threadID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetThreadForUser provides a mock function with given fields: userID, teamID, threadId, extended
func (_m *ThreadStore) GetThreadForUser(userID string, teamID string, threadId string, extended bool) (*model.ThreadResponse, error) {
	ret := _m.Called(userID, teamID, threadId, extended)

	var r0 *model.ThreadResponse
	if rf, ok := ret.Get(0).(func(string, string, string, bool) *model.ThreadResponse); ok {
		r0 = rf(userID, teamID, threadId, extended)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.ThreadResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, bool) error); ok {
		r1 = rf(userID, teamID, threadId, extended)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetThreadsForUser provides a mock function with given fields: userId, teamID, opts
func (_m *ThreadStore) GetThreadsForUser(userId string, teamID string, opts model.GetUserThreadsOpts) (*model.Threads, error) {
	ret := _m.Called(userId, teamID, opts)

	var r0 *model.Threads
	if rf, ok := ret.Get(0).(func(string, string, model.GetUserThreadsOpts) *model.Threads); ok {
		r0 = rf(userId, teamID, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Threads)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, model.GetUserThreadsOpts) error); ok {
		r1 = rf(userId, teamID, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MaintainMembership provides a mock function with given fields: userID, postID, following, incrementMentions, updateFollowing, updateViewedTimestamp, updateParticipants
func (_m *ThreadStore) MaintainMembership(userID string, postID string, following bool, incrementMentions bool, updateFollowing bool, updateViewedTimestamp bool, updateParticipants bool) (*model.ThreadMembership, error) {
	ret := _m.Called(userID, postID, following, incrementMentions, updateFollowing, updateViewedTimestamp, updateParticipants)

	var r0 *model.ThreadMembership
	if rf, ok := ret.Get(0).(func(string, string, bool, bool, bool, bool, bool) *model.ThreadMembership); ok {
		r0 = rf(userID, postID, following, incrementMentions, updateFollowing, updateViewedTimestamp, updateParticipants)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.ThreadMembership)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, bool, bool, bool, bool, bool) error); ok {
		r1 = rf(userID, postID, following, incrementMentions, updateFollowing, updateViewedTimestamp, updateParticipants)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MarkAllAsRead provides a mock function with given fields: userID, teamID
func (_m *ThreadStore) MarkAllAsRead(userID string, teamID string) error {
	ret := _m.Called(userID, teamID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string) error); ok {
		r0 = rf(userID, teamID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MarkAsRead provides a mock function with given fields: userID, threadID, timestamp
func (_m *ThreadStore) MarkAsRead(userID string, threadID string, timestamp int64) error {
	ret := _m.Called(userID, threadID, timestamp)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, int64) error); ok {
		r0 = rf(userID, threadID, timestamp)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Save provides a mock function with given fields: thread
func (_m *ThreadStore) Save(thread *model.Thread) (*model.Thread, error) {
	ret := _m.Called(thread)

	var r0 *model.Thread
	if rf, ok := ret.Get(0).(func(*model.Thread) *model.Thread); ok {
		r0 = rf(thread)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Thread)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*model.Thread) error); ok {
		r1 = rf(thread)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SaveMembership provides a mock function with given fields: membership
func (_m *ThreadStore) SaveMembership(membership *model.ThreadMembership) (*model.ThreadMembership, error) {
	ret := _m.Called(membership)

	var r0 *model.ThreadMembership
	if rf, ok := ret.Get(0).(func(*model.ThreadMembership) *model.ThreadMembership); ok {
		r0 = rf(membership)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.ThreadMembership)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*model.ThreadMembership) error); ok {
		r1 = rf(membership)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SaveMultiple provides a mock function with given fields: thread
func (_m *ThreadStore) SaveMultiple(thread []*model.Thread) ([]*model.Thread, int, error) {
	ret := _m.Called(thread)

	var r0 []*model.Thread
	if rf, ok := ret.Get(0).(func([]*model.Thread) []*model.Thread); ok {
		r0 = rf(thread)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.Thread)
		}
	}

	var r1 int
	if rf, ok := ret.Get(1).(func([]*model.Thread) int); ok {
		r1 = rf(thread)
	} else {
		r1 = ret.Get(1).(int)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func([]*model.Thread) error); ok {
		r2 = rf(thread)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Update provides a mock function with given fields: thread
func (_m *ThreadStore) Update(thread *model.Thread) (*model.Thread, error) {
	ret := _m.Called(thread)

	var r0 *model.Thread
	if rf, ok := ret.Get(0).(func(*model.Thread) *model.Thread); ok {
		r0 = rf(thread)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Thread)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*model.Thread) error); ok {
		r1 = rf(thread)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateMembership provides a mock function with given fields: membership
func (_m *ThreadStore) UpdateMembership(membership *model.ThreadMembership) (*model.ThreadMembership, error) {
	ret := _m.Called(membership)

	var r0 *model.ThreadMembership
	if rf, ok := ret.Get(0).(func(*model.ThreadMembership) *model.ThreadMembership); ok {
		r0 = rf(membership)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.ThreadMembership)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*model.ThreadMembership) error); ok {
		r1 = rf(membership)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateUnreadsByChannel provides a mock function with given fields: userId, changedThreads, timestamp, updateViewedTimestamp
func (_m *ThreadStore) UpdateUnreadsByChannel(userId string, changedThreads []string, timestamp int64, updateViewedTimestamp bool) error {
	ret := _m.Called(userId, changedThreads, timestamp, updateViewedTimestamp)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []string, int64, bool) error); ok {
		r0 = rf(userId, changedThreads, timestamp, updateViewedTimestamp)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
