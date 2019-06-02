// Code generated by mockery v1.0.0. DO NOT EDIT.

// Regenerate this file using `make store-mocks`.

package mocks

import mock "github.com/stretchr/testify/mock"
import model "github.com/mattermost/mattermost-server/model"
import store "github.com/tomasmik/mattermost-server/store"

// ChannelStore is an autogenerated mock type for the ChannelStore type
type ChannelStore struct {
	mock.Mock
}

// AnalyticsDeletedTypeCount provides a mock function with given fields: teamId, channelType
func (_m *ChannelStore) AnalyticsDeletedTypeCount(teamId string, channelType string) store.StoreChannel {
	ret := _m.Called(teamId, channelType)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(teamId, channelType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// AnalyticsTypeCount provides a mock function with given fields: teamId, channelType
func (_m *ChannelStore) AnalyticsTypeCount(teamId string, channelType string) store.StoreChannel {
	ret := _m.Called(teamId, channelType)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(teamId, channelType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// AutocompleteInTeam provides a mock function with given fields: teamId, term, includeDeleted
func (_m *ChannelStore) AutocompleteInTeam(teamId string, term string, includeDeleted bool) store.StoreChannel {
	ret := _m.Called(teamId, term, includeDeleted)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, bool) store.StoreChannel); ok {
		r0 = rf(teamId, term, includeDeleted)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// AutocompleteInTeamForSearch provides a mock function with given fields: teamId, userId, term, includeDeleted
func (_m *ChannelStore) AutocompleteInTeamForSearch(teamId string, userId string, term string, includeDeleted bool) store.StoreChannel {
	ret := _m.Called(teamId, userId, term, includeDeleted)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, string, bool) store.StoreChannel); ok {
		r0 = rf(teamId, userId, term, includeDeleted)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// ClearAllCustomRoleAssignments provides a mock function with given fields:
func (_m *ChannelStore) ClearAllCustomRoleAssignments() store.StoreChannel {
	ret := _m.Called()

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func() store.StoreChannel); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// ClearCaches provides a mock function with given fields:
func (_m *ChannelStore) ClearCaches() {
	_m.Called()
}

// CreateDirectChannel provides a mock function with given fields: userId, otherUserId
func (_m *ChannelStore) CreateDirectChannel(userId string, otherUserId string) (*model.Channel, *model.AppError) {
	ret := _m.Called(userId, otherUserId)

	var r0 *model.Channel
	if rf, ok := ret.Get(0).(func(string, string) *model.Channel); ok {
		r0 = rf(userId, otherUserId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Channel)
		}
	}

	var r1 *model.AppError
	if rf, ok := ret.Get(1).(func(string, string) *model.AppError); ok {
		r1 = rf(userId, otherUserId)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.AppError)
		}
	}

	return r0, r1
}

// Delete provides a mock function with given fields: channelId, time
func (_m *ChannelStore) Delete(channelId string, time int64) *model.AppError {
	ret := _m.Called(channelId, time)

	var r0 *model.AppError
	if rf, ok := ret.Get(0).(func(string, int64) *model.AppError); ok {
		r0 = rf(channelId, time)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.AppError)
		}
	}

	return r0
}

// Get provides a mock function with given fields: id, allowFromCache
func (_m *ChannelStore) Get(id string, allowFromCache bool) (*model.Channel, *model.AppError) {
	ret := _m.Called(id, allowFromCache)

	var r0 *model.Channel
	if rf, ok := ret.Get(0).(func(string, bool) *model.Channel); ok {
		r0 = rf(id, allowFromCache)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Channel)
		}
	}

	var r1 *model.AppError
	if rf, ok := ret.Get(1).(func(string, bool) *model.AppError); ok {
		r1 = rf(id, allowFromCache)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.AppError)
		}
	}

	return r0, r1
}

// GetAll provides a mock function with given fields: teamId
func (_m *ChannelStore) GetAll(teamId string) store.StoreChannel {
	ret := _m.Called(teamId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(teamId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetAllChannelMembersForUser provides a mock function with given fields: userId, allowFromCache, includeDeleted
func (_m *ChannelStore) GetAllChannelMembersForUser(userId string, allowFromCache bool, includeDeleted bool) store.StoreChannel {
	ret := _m.Called(userId, allowFromCache, includeDeleted)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, bool, bool) store.StoreChannel); ok {
		r0 = rf(userId, allowFromCache, includeDeleted)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetAllChannelMembersNotifyPropsForChannel provides a mock function with given fields: channelId, allowFromCache
func (_m *ChannelStore) GetAllChannelMembersNotifyPropsForChannel(channelId string, allowFromCache bool) store.StoreChannel {
	ret := _m.Called(channelId, allowFromCache)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, bool) store.StoreChannel); ok {
		r0 = rf(channelId, allowFromCache)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetAllChannels provides a mock function with given fields: page, perPage, opts
func (_m *ChannelStore) GetAllChannels(page int, perPage int, opts store.ChannelSearchOpts) store.StoreChannel {
	ret := _m.Called(page, perPage, opts)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(int, int, store.ChannelSearchOpts) store.StoreChannel); ok {
		r0 = rf(page, perPage, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetAllChannelsForExportAfter provides a mock function with given fields: limit, afterId
func (_m *ChannelStore) GetAllChannelsForExportAfter(limit int, afterId string) store.StoreChannel {
	ret := _m.Called(limit, afterId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(int, string) store.StoreChannel); ok {
		r0 = rf(limit, afterId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetAllDirectChannelsForExportAfter provides a mock function with given fields: limit, afterId
func (_m *ChannelStore) GetAllDirectChannelsForExportAfter(limit int, afterId string) store.StoreChannel {
	ret := _m.Called(limit, afterId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(int, string) store.StoreChannel); ok {
		r0 = rf(limit, afterId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetByName provides a mock function with given fields: team_id, name, allowFromCache
func (_m *ChannelStore) GetByName(team_id string, name string, allowFromCache bool) store.StoreChannel {
	ret := _m.Called(team_id, name, allowFromCache)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, bool) store.StoreChannel); ok {
		r0 = rf(team_id, name, allowFromCache)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetByNameIncludeDeleted provides a mock function with given fields: team_id, name, allowFromCache
func (_m *ChannelStore) GetByNameIncludeDeleted(team_id string, name string, allowFromCache bool) store.StoreChannel {
	ret := _m.Called(team_id, name, allowFromCache)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, bool) store.StoreChannel); ok {
		r0 = rf(team_id, name, allowFromCache)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetByNames provides a mock function with given fields: team_id, names, allowFromCache
func (_m *ChannelStore) GetByNames(team_id string, names []string, allowFromCache bool) store.StoreChannel {
	ret := _m.Called(team_id, names, allowFromCache)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, []string, bool) store.StoreChannel); ok {
		r0 = rf(team_id, names, allowFromCache)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetChannelCounts provides a mock function with given fields: teamId, userId
func (_m *ChannelStore) GetChannelCounts(teamId string, userId string) store.StoreChannel {
	ret := _m.Called(teamId, userId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(teamId, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetChannelMembersForExport provides a mock function with given fields: userId, teamId
func (_m *ChannelStore) GetChannelMembersForExport(userId string, teamId string) store.StoreChannel {
	ret := _m.Called(userId, teamId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(userId, teamId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetChannelMembersTimezones provides a mock function with given fields: channelId
func (_m *ChannelStore) GetChannelMembersTimezones(channelId string) store.StoreChannel {
	ret := _m.Called(channelId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(channelId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetChannelUnread provides a mock function with given fields: channelId, userId
func (_m *ChannelStore) GetChannelUnread(channelId string, userId string) (*model.ChannelUnread, *model.AppError) {
	ret := _m.Called(channelId, userId)

	var r0 *model.ChannelUnread
	if rf, ok := ret.Get(0).(func(string, string) *model.ChannelUnread); ok {
		r0 = rf(channelId, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.ChannelUnread)
		}
	}

	var r1 *model.AppError
	if rf, ok := ret.Get(1).(func(string, string) *model.AppError); ok {
		r1 = rf(channelId, userId)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.AppError)
		}
	}

	return r0, r1
}

// GetChannels provides a mock function with given fields: teamId, userId, includeDeleted
func (_m *ChannelStore) GetChannels(teamId string, userId string, includeDeleted bool) store.StoreChannel {
	ret := _m.Called(teamId, userId, includeDeleted)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, bool) store.StoreChannel); ok {
		r0 = rf(teamId, userId, includeDeleted)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetChannelsBatchForIndexing provides a mock function with given fields: startTime, endTime, limit
func (_m *ChannelStore) GetChannelsBatchForIndexing(startTime int64, endTime int64, limit int) store.StoreChannel {
	ret := _m.Called(startTime, endTime, limit)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(int64, int64, int) store.StoreChannel); ok {
		r0 = rf(startTime, endTime, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetChannelsByIds provides a mock function with given fields: channelIds
func (_m *ChannelStore) GetChannelsByIds(channelIds []string) store.StoreChannel {
	ret := _m.Called(channelIds)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func([]string) store.StoreChannel); ok {
		r0 = rf(channelIds)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetChannelsByScheme provides a mock function with given fields: schemeId, offset, limit
func (_m *ChannelStore) GetChannelsByScheme(schemeId string, offset int, limit int) store.StoreChannel {
	ret := _m.Called(schemeId, offset, limit)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, int, int) store.StoreChannel); ok {
		r0 = rf(schemeId, offset, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetDeleted provides a mock function with given fields: team_id, offset, limit
func (_m *ChannelStore) GetDeleted(team_id string, offset int, limit int) store.StoreChannel {
	ret := _m.Called(team_id, offset, limit)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, int, int) store.StoreChannel); ok {
		r0 = rf(team_id, offset, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetDeletedByName provides a mock function with given fields: team_id, name
func (_m *ChannelStore) GetDeletedByName(team_id string, name string) store.StoreChannel {
	ret := _m.Called(team_id, name)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(team_id, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetForPost provides a mock function with given fields: postId
func (_m *ChannelStore) GetForPost(postId string) store.StoreChannel {
	ret := _m.Called(postId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(postId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetFromMaster provides a mock function with given fields: id
func (_m *ChannelStore) GetFromMaster(id string) (*model.Channel, *model.AppError) {
	ret := _m.Called(id)

	var r0 *model.Channel
	if rf, ok := ret.Get(0).(func(string) *model.Channel); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Channel)
		}
	}

	var r1 *model.AppError
	if rf, ok := ret.Get(1).(func(string) *model.AppError); ok {
		r1 = rf(id)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.AppError)
		}
	}

	return r0, r1
}

// GetMember provides a mock function with given fields: channelId, userId
func (_m *ChannelStore) GetMember(channelId string, userId string) (*model.ChannelMember, *model.AppError) {
	ret := _m.Called(channelId, userId)

	var r0 *model.ChannelMember
	if rf, ok := ret.Get(0).(func(string, string) *model.ChannelMember); ok {
		r0 = rf(channelId, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.ChannelMember)
		}
	}

	var r1 *model.AppError
	if rf, ok := ret.Get(1).(func(string, string) *model.AppError); ok {
		r1 = rf(channelId, userId)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.AppError)
		}
	}

	return r0, r1
}

// GetMemberCount provides a mock function with given fields: channelId, allowFromCache
func (_m *ChannelStore) GetMemberCount(channelId string, allowFromCache bool) store.StoreChannel {
	ret := _m.Called(channelId, allowFromCache)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, bool) store.StoreChannel); ok {
		r0 = rf(channelId, allowFromCache)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetMemberCountFromCache provides a mock function with given fields: channelId
func (_m *ChannelStore) GetMemberCountFromCache(channelId string) int64 {
	ret := _m.Called(channelId)

	var r0 int64
	if rf, ok := ret.Get(0).(func(string) int64); ok {
		r0 = rf(channelId)
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

// GetMemberForPost provides a mock function with given fields: postId, userId
func (_m *ChannelStore) GetMemberForPost(postId string, userId string) store.StoreChannel {
	ret := _m.Called(postId, userId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(postId, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetMembers provides a mock function with given fields: channelId, offset, limit
func (_m *ChannelStore) GetMembers(channelId string, offset int, limit int) store.StoreChannel {
	ret := _m.Called(channelId, offset, limit)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, int, int) store.StoreChannel); ok {
		r0 = rf(channelId, offset, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetMembersByIds provides a mock function with given fields: channelId, userIds
func (_m *ChannelStore) GetMembersByIds(channelId string, userIds []string) store.StoreChannel {
	ret := _m.Called(channelId, userIds)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, []string) store.StoreChannel); ok {
		r0 = rf(channelId, userIds)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetMembersForUser provides a mock function with given fields: teamId, userId
func (_m *ChannelStore) GetMembersForUser(teamId string, userId string) store.StoreChannel {
	ret := _m.Called(teamId, userId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(teamId, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetMembersForUserWithPagination provides a mock function with given fields: teamId, userId, page, perPage
func (_m *ChannelStore) GetMembersForUserWithPagination(teamId string, userId string, page int, perPage int) store.StoreChannel {
	ret := _m.Called(teamId, userId, page, perPage)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, int, int) store.StoreChannel); ok {
		r0 = rf(teamId, userId, page, perPage)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetMoreChannels provides a mock function with given fields: teamId, userId, offset, limit
func (_m *ChannelStore) GetMoreChannels(teamId string, userId string, offset int, limit int) store.StoreChannel {
	ret := _m.Called(teamId, userId, offset, limit)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, int, int) store.StoreChannel); ok {
		r0 = rf(teamId, userId, offset, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetPinnedPosts provides a mock function with given fields: channelId
func (_m *ChannelStore) GetPinnedPosts(channelId string) store.StoreChannel {
	ret := _m.Called(channelId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(channelId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetPublicChannelsByIdsForTeam provides a mock function with given fields: teamId, channelIds
func (_m *ChannelStore) GetPublicChannelsByIdsForTeam(teamId string, channelIds []string) store.StoreChannel {
	ret := _m.Called(teamId, channelIds)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, []string) store.StoreChannel); ok {
		r0 = rf(teamId, channelIds)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetPublicChannelsForTeam provides a mock function with given fields: teamId, offset, limit
func (_m *ChannelStore) GetPublicChannelsForTeam(teamId string, offset int, limit int) store.StoreChannel {
	ret := _m.Called(teamId, offset, limit)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, int, int) store.StoreChannel); ok {
		r0 = rf(teamId, offset, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// GetTeamChannels provides a mock function with given fields: teamId
func (_m *ChannelStore) GetTeamChannels(teamId string) store.StoreChannel {
	ret := _m.Called(teamId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(teamId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// IncrementMentionCount provides a mock function with given fields: channelId, userId
func (_m *ChannelStore) IncrementMentionCount(channelId string, userId string) store.StoreChannel {
	ret := _m.Called(channelId, userId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(channelId, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// InvalidateAllChannelMembersForUser provides a mock function with given fields: userId
func (_m *ChannelStore) InvalidateAllChannelMembersForUser(userId string) {
	_m.Called(userId)
}

// InvalidateCacheForChannelMembersNotifyProps provides a mock function with given fields: channelId
func (_m *ChannelStore) InvalidateCacheForChannelMembersNotifyProps(channelId string) {
	_m.Called(channelId)
}

// InvalidateChannel provides a mock function with given fields: id
func (_m *ChannelStore) InvalidateChannel(id string) {
	_m.Called(id)
}

// InvalidateChannelByName provides a mock function with given fields: teamId, name
func (_m *ChannelStore) InvalidateChannelByName(teamId string, name string) {
	_m.Called(teamId, name)
}

// InvalidateMemberCount provides a mock function with given fields: channelId
func (_m *ChannelStore) InvalidateMemberCount(channelId string) {
	_m.Called(channelId)
}

// IsUserInChannelUseCache provides a mock function with given fields: userId, channelId
func (_m *ChannelStore) IsUserInChannelUseCache(userId string, channelId string) bool {
	ret := _m.Called(userId, channelId)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string, string) bool); ok {
		r0 = rf(userId, channelId)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// MigrateChannelMembers provides a mock function with given fields: fromChannelId, fromUserId
func (_m *ChannelStore) MigrateChannelMembers(fromChannelId string, fromUserId string) store.StoreChannel {
	ret := _m.Called(fromChannelId, fromUserId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(fromChannelId, fromUserId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// MigratePublicChannels provides a mock function with given fields:
func (_m *ChannelStore) MigratePublicChannels() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PermanentDelete provides a mock function with given fields: channelId
func (_m *ChannelStore) PermanentDelete(channelId string) store.StoreChannel {
	ret := _m.Called(channelId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(channelId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// PermanentDeleteByTeam provides a mock function with given fields: teamId
func (_m *ChannelStore) PermanentDeleteByTeam(teamId string) store.StoreChannel {
	ret := _m.Called(teamId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(teamId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// PermanentDeleteMembersByChannel provides a mock function with given fields: channelId
func (_m *ChannelStore) PermanentDeleteMembersByChannel(channelId string) store.StoreChannel {
	ret := _m.Called(channelId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(channelId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// PermanentDeleteMembersByUser provides a mock function with given fields: userId
func (_m *ChannelStore) PermanentDeleteMembersByUser(userId string) store.StoreChannel {
	ret := _m.Called(userId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// RemoveAllDeactivatedMembers provides a mock function with given fields: channelId
func (_m *ChannelStore) RemoveAllDeactivatedMembers(channelId string) store.StoreChannel {
	ret := _m.Called(channelId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string) store.StoreChannel); ok {
		r0 = rf(channelId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// RemoveMember provides a mock function with given fields: channelId, userId
func (_m *ChannelStore) RemoveMember(channelId string, userId string) store.StoreChannel {
	ret := _m.Called(channelId, userId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string) store.StoreChannel); ok {
		r0 = rf(channelId, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// ResetAllChannelSchemes provides a mock function with given fields:
func (_m *ChannelStore) ResetAllChannelSchemes() store.StoreChannel {
	ret := _m.Called()

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func() store.StoreChannel); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// Restore provides a mock function with given fields: channelId, time
func (_m *ChannelStore) Restore(channelId string, time int64) *model.AppError {
	ret := _m.Called(channelId, time)

	var r0 *model.AppError
	if rf, ok := ret.Get(0).(func(string, int64) *model.AppError); ok {
		r0 = rf(channelId, time)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.AppError)
		}
	}

	return r0
}

// Save provides a mock function with given fields: channel, maxChannelsPerTeam
func (_m *ChannelStore) Save(channel *model.Channel, maxChannelsPerTeam int64) (*model.Channel, *model.AppError) {
	ret := _m.Called(channel, maxChannelsPerTeam)

	var r0 *model.Channel
	if rf, ok := ret.Get(0).(func(*model.Channel, int64) *model.Channel); ok {
		r0 = rf(channel, maxChannelsPerTeam)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Channel)
		}
	}

	var r1 *model.AppError
	if rf, ok := ret.Get(1).(func(*model.Channel, int64) *model.AppError); ok {
		r1 = rf(channel, maxChannelsPerTeam)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.AppError)
		}
	}

	return r0, r1
}

// SaveDirectChannel provides a mock function with given fields: channel, member1, member2
func (_m *ChannelStore) SaveDirectChannel(channel *model.Channel, member1 *model.ChannelMember, member2 *model.ChannelMember) (*model.Channel, *model.AppError) {
	ret := _m.Called(channel, member1, member2)

	var r0 *model.Channel
	if rf, ok := ret.Get(0).(func(*model.Channel, *model.ChannelMember, *model.ChannelMember) *model.Channel); ok {
		r0 = rf(channel, member1, member2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Channel)
		}
	}

	var r1 *model.AppError
	if rf, ok := ret.Get(1).(func(*model.Channel, *model.ChannelMember, *model.ChannelMember) *model.AppError); ok {
		r1 = rf(channel, member1, member2)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.AppError)
		}
	}

	return r0, r1
}

// SaveMember provides a mock function with given fields: member
func (_m *ChannelStore) SaveMember(member *model.ChannelMember) store.StoreChannel {
	ret := _m.Called(member)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(*model.ChannelMember) store.StoreChannel); ok {
		r0 = rf(member)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// SearchAllChannels provides a mock function with given fields: term, opts
func (_m *ChannelStore) SearchAllChannels(term string, opts store.ChannelSearchOpts) store.StoreChannel {
	ret := _m.Called(term, opts)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, store.ChannelSearchOpts) store.StoreChannel); ok {
		r0 = rf(term, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// SearchInTeam provides a mock function with given fields: teamId, term, includeDeleted
func (_m *ChannelStore) SearchInTeam(teamId string, term string, includeDeleted bool) store.StoreChannel {
	ret := _m.Called(teamId, term, includeDeleted)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, bool) store.StoreChannel); ok {
		r0 = rf(teamId, term, includeDeleted)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// SearchMore provides a mock function with given fields: userId, teamId, term
func (_m *ChannelStore) SearchMore(userId string, teamId string, term string) store.StoreChannel {
	ret := _m.Called(userId, teamId, term)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, string, string) store.StoreChannel); ok {
		r0 = rf(userId, teamId, term)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// SetDeleteAt provides a mock function with given fields: channelId, deleteAt, updateAt
func (_m *ChannelStore) SetDeleteAt(channelId string, deleteAt int64, updateAt int64) *model.AppError {
	ret := _m.Called(channelId, deleteAt, updateAt)

	var r0 *model.AppError
	if rf, ok := ret.Get(0).(func(string, int64, int64) *model.AppError); ok {
		r0 = rf(channelId, deleteAt, updateAt)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.AppError)
		}
	}

	return r0
}

// Update provides a mock function with given fields: channel
func (_m *ChannelStore) Update(channel *model.Channel) (*model.Channel, *model.AppError) {
	ret := _m.Called(channel)

	var r0 *model.Channel
	if rf, ok := ret.Get(0).(func(*model.Channel) *model.Channel); ok {
		r0 = rf(channel)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Channel)
		}
	}

	var r1 *model.AppError
	if rf, ok := ret.Get(1).(func(*model.Channel) *model.AppError); ok {
		r1 = rf(channel)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.AppError)
		}
	}

	return r0, r1
}

// UpdateLastViewedAt provides a mock function with given fields: channelIds, userId
func (_m *ChannelStore) UpdateLastViewedAt(channelIds []string, userId string) store.StoreChannel {
	ret := _m.Called(channelIds, userId)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func([]string, string) store.StoreChannel); ok {
		r0 = rf(channelIds, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// UpdateMember provides a mock function with given fields: member
func (_m *ChannelStore) UpdateMember(member *model.ChannelMember) store.StoreChannel {
	ret := _m.Called(member)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(*model.ChannelMember) store.StoreChannel); ok {
		r0 = rf(member)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}

// UserBelongsToChannels provides a mock function with given fields: userId, channelIds
func (_m *ChannelStore) UserBelongsToChannels(userId string, channelIds []string) store.StoreChannel {
	ret := _m.Called(userId, channelIds)

	var r0 store.StoreChannel
	if rf, ok := ret.Get(0).(func(string, []string) store.StoreChannel); ok {
		r0 = rf(userId, channelIds)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(store.StoreChannel)
		}
	}

	return r0
}
