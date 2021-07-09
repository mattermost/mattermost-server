// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchengine

import (
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
)

type SearchEngineInterface interface {
	Start() *model.AppError
	Stop() *model.AppError
	GetFullVersion() string
	GetVersion() int
	GetPlugins() []string
	UpdateConfig(cfg *model.Config)
	GetName() string
	IsActive() bool
	IsIndexingEnabled() bool
	IsSearchEnabled() bool
	IsAutocompletionEnabled() bool
	IsIndexingSync() bool
	IndexPost(post *model.Post, teamID string) *model.AppError
	SearchPosts(channels *model.ChannelList, searchParams []*model.SearchParams, page, perPage int) ([]string, model.PostSearchMatches, *model.AppError)
	DeletePost(post *model.Post) *model.AppError
	DeleteChannelPosts(channelID string) *model.AppError
	DeleteUserPosts(userID string) *model.AppError
	IndexChannel(channel *model.Channel) *model.AppError
	SearchChannels(teamID, term string) ([]string, *model.AppError)
	DeleteChannel(channel *model.Channel) *model.AppError
	IndexUser(user *model.User, teamsIDs, channelsIDs []string) *model.AppError
	SearchUsersInChannel(teamID, channelID string, restrictedToChannels []string, term string, options *model.UserSearchOptions) ([]string, []string, *model.AppError)
	SearchUsersInTeam(teamID string, restrictedToChannels []string, term string, options *model.UserSearchOptions) ([]string, *model.AppError)
	DeleteUser(user *model.User) *model.AppError
	IndexFile(file *model.FileInfo, channelID string) *model.AppError
	SearchFiles(channels *model.ChannelList, searchParams []*model.SearchParams, page, perPage int) ([]string, *model.AppError)
	DeleteFile(fileID string) *model.AppError
	DeletePostFiles(postID string) *model.AppError
	DeleteUserFiles(userID string) *model.AppError
	DeleteFilesBatch(endTime, limit int64) *model.AppError
	TestConfig(cfg *model.Config) *model.AppError
	PurgeIndexes() *model.AppError
	RefreshIndexes() *model.AppError
	DataRetentionDeleteIndexes(cutoff time.Time) *model.AppError
}
