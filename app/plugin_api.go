// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

type PluginAPI struct {
	id       string
	app      *App
	ctx      *request.Context
	logger   *mlog.SugarLogger
	manifest *model.Manifest
}

func NewPluginAPI(a *App, c *request.Context, manifest *model.Manifest) *PluginAPI {
	return &PluginAPI{
		id:       manifest.Id,
		manifest: manifest,
		ctx:      c,
		app:      a,
		logger:   a.Log().With(mlog.String("plugin_id", manifest.Id)).Sugar(),
	}
}

func (api *PluginAPI) LoadPluginConfiguration(dest interface{}) error {
	finalConfig := make(map[string]interface{})

	// First set final config to defaults
	if api.manifest.SettingsSchema != nil {
		for _, setting := range api.manifest.SettingsSchema.Settings {
			finalConfig[strings.ToLower(setting.Key)] = setting.Default
		}
	}

	// If we have settings given we override the defaults with them
	for setting, value := range api.app.Config().PluginSettings.Plugins[api.id] {
		finalConfig[strings.ToLower(setting)] = value
	}

	pluginSettingsJsonBytes, err := json.Marshal(finalConfig)
	if err != nil {
		api.logger.Error("Error marshaling config for plugin", mlog.Err(err))
		return nil
	}
	err = json.Unmarshal(pluginSettingsJsonBytes, dest)
	if err != nil {
		api.logger.Error("Error unmarshaling config for plugin", mlog.Err(err))
	}
	return nil
}

func (api *PluginAPI) RegisterCommand(command *model.Command) error {
	return api.app.RegisterPluginCommand(api.id, command)
}

func (api *PluginAPI) UnregisterCommand(teamID, trigger string) error {
	api.app.UnregisterPluginCommand(api.id, teamID, trigger)
	return nil
}

func (api *PluginAPI) ExecuteSlashCommand(commandArgs *model.CommandArgs) (*model.CommandResponse, error) {
	user, appErr := api.app.GetUser(commandArgs.UserId)
	if appErr != nil {
		return nil, appErr
	}
	commandArgs.T = i18n.GetUserTranslations(user.Locale)
	commandArgs.SiteURL = api.app.GetSiteURL()
	response, appErr := api.app.ExecuteCommand(api.ctx, commandArgs)
	if appErr != nil {
		return response, appErr
	}
	return response, nil
}

func (api *PluginAPI) GetSession(sessionID string) (*model.Session, *model.AppError) {
	session, err := api.app.GetSessionById(sessionID)

	if err != nil {
		return nil, err
	}

	return session, nil
}

func (api *PluginAPI) GetConfig() *model.Config {
	return api.app.GetSanitizedConfig()
}

// GetUnsanitizedConfig gets the configuration for a system admin without removing secrets.
func (api *PluginAPI) GetUnsanitizedConfig() *model.Config {
	return api.app.Config().Clone()
}

func (api *PluginAPI) SaveConfig(config *model.Config) *model.AppError {
	_, _, err := api.app.SaveConfig(config, true)
	return err
}

func (api *PluginAPI) GetPluginConfig() map[string]interface{} {
	cfg := api.app.GetSanitizedConfig()
	if pluginConfig, isOk := cfg.PluginSettings.Plugins[api.manifest.Id]; isOk {
		return pluginConfig
	}
	return map[string]interface{}{}
}

func (api *PluginAPI) SavePluginConfig(pluginConfig map[string]interface{}) *model.AppError {
	cfg := api.app.GetSanitizedConfig()
	cfg.PluginSettings.Plugins[api.manifest.Id] = pluginConfig
	_, _, err := api.app.SaveConfig(cfg, true)
	return err
}

func (api *PluginAPI) GetBundlePath() (string, error) {
	bundlePath, err := filepath.Abs(filepath.Join(*api.GetConfig().PluginSettings.Directory, api.manifest.Id))
	if err != nil {
		return "", err
	}

	return bundlePath, err
}

func (api *PluginAPI) GetLicense() *model.License {
	return api.app.Srv().License()
}

func (api *PluginAPI) GetServerVersion() string {
	return model.CurrentVersion
}

func (api *PluginAPI) GetSystemInstallDate() (int64, *model.AppError) {
	return api.app.Srv().getSystemInstallDate()
}

func (api *PluginAPI) GetDiagnosticId() string {
	return api.app.TelemetryId()
}

func (api *PluginAPI) GetTelemetryId() string {
	return api.app.TelemetryId()
}

func (api *PluginAPI) CreateTeam(team *model.Team) (*model.Team, *model.AppError) {
	return api.app.CreateTeam(api.ctx, team)
}

func (api *PluginAPI) DeleteTeam(teamID string) *model.AppError {
	return api.app.SoftDeleteTeam(teamID)
}

func (api *PluginAPI) GetTeams() ([]*model.Team, *model.AppError) {
	return api.app.GetAllTeams()
}

func (api *PluginAPI) GetTeam(teamID string) (*model.Team, *model.AppError) {
	return api.app.GetTeam(teamID)
}

func (api *PluginAPI) SearchTeams(term string) ([]*model.Team, *model.AppError) {
	teams, _, err := api.app.SearchAllTeams(&model.TeamSearch{Term: term})
	return teams, err
}

func (api *PluginAPI) GetTeamByName(name string) (*model.Team, *model.AppError) {
	return api.app.GetTeamByName(name)
}

func (api *PluginAPI) GetTeamsUnreadForUser(userID string) ([]*model.TeamUnread, *model.AppError) {
	return api.app.GetTeamsUnreadForUser("", userID)
}

func (api *PluginAPI) UpdateTeam(team *model.Team) (*model.Team, *model.AppError) {
	return api.app.UpdateTeam(team)
}

func (api *PluginAPI) GetTeamsForUser(userID string) ([]*model.Team, *model.AppError) {
	return api.app.GetTeamsForUser(userID)
}

func (api *PluginAPI) CreateTeamMember(teamID, userID string) (*model.TeamMember, *model.AppError) {
	return api.app.AddTeamMember(api.ctx, teamID, userID)
}

func (api *PluginAPI) CreateTeamMembers(teamID string, userIDs []string, requestorId string) ([]*model.TeamMember, *model.AppError) {
	members, err := api.app.AddTeamMembers(api.ctx, teamID, userIDs, requestorId, false)
	if err != nil {
		return nil, err
	}
	return model.TeamMembersWithErrorToTeamMembers(members), nil
}

func (api *PluginAPI) CreateTeamMembersGracefully(teamID string, userIDs []string, requestorId string) ([]*model.TeamMemberWithError, *model.AppError) {
	return api.app.AddTeamMembers(api.ctx, teamID, userIDs, requestorId, true)
}

func (api *PluginAPI) DeleteTeamMember(teamID, userID, requestorId string) *model.AppError {
	return api.app.RemoveUserFromTeam(api.ctx, teamID, userID, requestorId)
}

func (api *PluginAPI) GetTeamMembers(teamID string, page, perPage int) ([]*model.TeamMember, *model.AppError) {
	return api.app.GetTeamMembers(teamID, page*perPage, perPage, nil)
}

func (api *PluginAPI) GetTeamMember(teamID, userID string) (*model.TeamMember, *model.AppError) {
	return api.app.GetTeamMember(teamID, userID)
}

func (api *PluginAPI) GetTeamMembersForUser(userID string, page int, perPage int) ([]*model.TeamMember, *model.AppError) {
	return api.app.GetTeamMembersForUserWithPagination(userID, page, perPage)
}

func (api *PluginAPI) UpdateTeamMemberRoles(teamID, userID, newRoles string) (*model.TeamMember, *model.AppError) {
	return api.app.UpdateTeamMemberRoles(teamID, userID, newRoles)
}

func (api *PluginAPI) GetTeamStats(teamID string) (*model.TeamStats, *model.AppError) {
	return api.app.GetTeamStats(teamID, nil)
}

func (api *PluginAPI) CreateUser(user *model.User) (*model.User, *model.AppError) {
	return api.app.CreateUser(api.ctx, user)
}

func (api *PluginAPI) DeleteUser(userID string) *model.AppError {
	user, err := api.app.GetUser(userID)
	if err != nil {
		return err
	}
	_, err = api.app.UpdateActive(api.ctx, user, false)
	return err
}

func (api *PluginAPI) GetUsers(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
	return api.app.GetUsers(options)
}

func (api *PluginAPI) GetUser(userID string) (*model.User, *model.AppError) {
	return api.app.GetUser(userID)
}

func (api *PluginAPI) GetUserByEmail(email string) (*model.User, *model.AppError) {
	return api.app.GetUserByEmail(email)
}

func (api *PluginAPI) GetUserByUsername(name string) (*model.User, *model.AppError) {
	return api.app.GetUserByUsername(name)
}

func (api *PluginAPI) GetUsersByUsernames(usernames []string) ([]*model.User, *model.AppError) {
	return api.app.GetUsersByUsernames(usernames, true, nil)
}

func (api *PluginAPI) GetUsersInTeam(teamID string, page int, perPage int) ([]*model.User, *model.AppError) {
	options := &model.UserGetOptions{InTeamId: teamID, Page: page, PerPage: perPage}
	return api.app.GetUsersInTeam(options)
}

func (api *PluginAPI) GetPreferencesForUser(userID string) ([]model.Preference, *model.AppError) {
	return api.app.GetPreferencesForUser(userID)
}

func (api *PluginAPI) UpdatePreferencesForUser(userID string, preferences []model.Preference) *model.AppError {
	return api.app.UpdatePreferences(userID, preferences)
}

func (api *PluginAPI) DeletePreferencesForUser(userID string, preferences []model.Preference) *model.AppError {
	return api.app.DeletePreferences(userID, preferences)
}

func (api *PluginAPI) CreateUserAccessToken(token *model.UserAccessToken) (*model.UserAccessToken, *model.AppError) {
	return api.app.CreateUserAccessToken(token)
}

func (api *PluginAPI) RevokeUserAccessToken(tokenID string) *model.AppError {
	accessToken, err := api.app.GetUserAccessToken(tokenID, false)
	if err != nil {
		return err
	}

	return api.app.RevokeUserAccessToken(accessToken)
}

func (api *PluginAPI) UpdateUser(user *model.User) (*model.User, *model.AppError) {
	return api.app.UpdateUser(user, true)
}

func (api *PluginAPI) UpdateUserActive(userID string, active bool) *model.AppError {
	return api.app.UpdateUserActive(api.ctx, userID, active)
}

func (api *PluginAPI) GetUserStatus(userID string) (*model.Status, *model.AppError) {
	return api.app.GetStatus(userID)
}

func (api *PluginAPI) GetUserStatusesByIds(userIDs []string) ([]*model.Status, *model.AppError) {
	return api.app.GetUserStatusesByIds(userIDs)
}

func (api *PluginAPI) UpdateUserStatus(userID, status string) (*model.Status, *model.AppError) {
	switch status {
	case model.STATUS_ONLINE:
		api.app.SetStatusOnline(userID, true)
	case model.STATUS_OFFLINE:
		api.app.SetStatusOffline(userID, true)
	case model.STATUS_AWAY:
		api.app.SetStatusAwayIfNeeded(userID, true)
	case model.STATUS_DND:
		api.app.SetStatusDoNotDisturb(userID)
	default:
		return nil, model.NewAppError("UpdateUserStatus", "plugin.api.update_user_status.bad_status", nil, "unrecognized status", http.StatusBadRequest)
	}

	return api.app.GetStatus(userID)
}

func (api *PluginAPI) SetUserStatusTimedDND(userID string, endTime int64) (*model.Status, *model.AppError) {
	// read-after-write bug which will fail if there are replicas.
	// it works for now because we have a cache in between.
	// FIXME: make SetStatusDoNotDisturbTimed return updated status
	api.app.SetStatusDoNotDisturbTimed(userID, endTime)
	return api.app.GetStatus(userID)
}

func (api *PluginAPI) GetUsersInChannel(channelID, sortBy string, page, perPage int) ([]*model.User, *model.AppError) {
	switch sortBy {
	case model.CHANNEL_SORT_BY_USERNAME:
		return api.app.GetUsersInChannel(&model.UserGetOptions{
			InChannelId: channelID,
			Page:        page,
			PerPage:     perPage,
		})
	case model.CHANNEL_SORT_BY_STATUS:
		return api.app.GetUsersInChannelByStatus(&model.UserGetOptions{
			InChannelId: channelID,
			Page:        page,
			PerPage:     perPage,
		})
	default:
		return nil, model.NewAppError("GetUsersInChannel", "plugin.api.get_users_in_channel", nil, "invalid sort option", http.StatusBadRequest)
	}
}

func (api *PluginAPI) GetLDAPUserAttributes(userID string, attributes []string) (map[string]string, *model.AppError) {
	if api.app.Ldap() == nil {
		return nil, model.NewAppError("GetLdapUserAttributes", "ent.ldap.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	user, err := api.app.GetUser(userID)
	if err != nil {
		return nil, err
	}

	if user.AuthData == nil {
		return map[string]string{}, nil
	}

	// Only bother running the query if the user's auth service is LDAP or it's SAML and sync is enabled.
	if user.AuthService == model.USER_AUTH_SERVICE_LDAP ||
		(user.AuthService == model.USER_AUTH_SERVICE_SAML && *api.app.Config().SamlSettings.EnableSyncWithLdap) {
		return api.app.Ldap().GetUserAttributes(*user.AuthData, attributes)
	}

	return map[string]string{}, nil
}

func (api *PluginAPI) CreateChannel(channel *model.Channel) (*model.Channel, *model.AppError) {
	return api.app.CreateChannel(api.ctx, channel, false)
}

func (api *PluginAPI) DeleteChannel(channelID string) *model.AppError {
	channel, err := api.app.GetChannel(channelID)
	if err != nil {
		return err
	}
	return api.app.DeleteChannel(api.ctx, channel, "")
}

func (api *PluginAPI) GetPublicChannelsForTeam(teamID string, page, perPage int) ([]*model.Channel, *model.AppError) {
	channels, err := api.app.GetPublicChannelsForTeam(teamID, page*perPage, perPage)
	if err != nil {
		return nil, err
	}
	return *channels, err
}

func (api *PluginAPI) GetChannel(channelID string) (*model.Channel, *model.AppError) {
	return api.app.GetChannel(channelID)
}

func (api *PluginAPI) GetChannelByName(teamID, name string, includeDeleted bool) (*model.Channel, *model.AppError) {
	return api.app.GetChannelByName(name, teamID, includeDeleted)
}

func (api *PluginAPI) GetChannelByNameForTeamName(teamName, channelName string, includeDeleted bool) (*model.Channel, *model.AppError) {
	return api.app.GetChannelByNameForTeamName(channelName, teamName, includeDeleted)
}

func (api *PluginAPI) GetChannelsForTeamForUser(teamID, userID string, includeDeleted bool) ([]*model.Channel, *model.AppError) {
	channels, err := api.app.GetChannelsForUser(teamID, userID, includeDeleted, 0)
	if err != nil {
		return nil, err
	}
	return *channels, err
}

func (api *PluginAPI) GetChannelStats(channelID string) (*model.ChannelStats, *model.AppError) {
	memberCount, err := api.app.GetChannelMemberCount(channelID)
	if err != nil {
		return nil, err
	}
	guestCount, err := api.app.GetChannelMemberCount(channelID)
	if err != nil {
		return nil, err
	}
	return &model.ChannelStats{ChannelId: channelID, MemberCount: memberCount, GuestCount: guestCount}, nil
}

func (api *PluginAPI) GetDirectChannel(userID1, userID2 string) (*model.Channel, *model.AppError) {
	return api.app.GetOrCreateDirectChannel(api.ctx, userID1, userID2)
}

func (api *PluginAPI) GetGroupChannel(userIDs []string) (*model.Channel, *model.AppError) {
	return api.app.CreateGroupChannel(userIDs, "")
}

func (api *PluginAPI) UpdateChannel(channel *model.Channel) (*model.Channel, *model.AppError) {
	return api.app.UpdateChannel(channel)
}

func (api *PluginAPI) SearchChannels(teamID string, term string) ([]*model.Channel, *model.AppError) {
	channels, err := api.app.SearchChannels(teamID, term)
	if err != nil {
		return nil, err
	}
	return *channels, err
}

func (api *PluginAPI) CreateChannelSidebarCategory(userID, teamID string, newCategory *model.SidebarCategoryWithChannels) (*model.SidebarCategoryWithChannels, *model.AppError) {
	return api.app.CreateSidebarCategory(userID, teamID, newCategory)
}

func (api *PluginAPI) GetChannelSidebarCategories(userID, teamID string) (*model.OrderedSidebarCategories, *model.AppError) {
	return api.app.GetSidebarCategories(userID, teamID)
}

func (api *PluginAPI) UpdateChannelSidebarCategories(userID, teamID string, categories []*model.SidebarCategoryWithChannels) ([]*model.SidebarCategoryWithChannels, *model.AppError) {
	return api.app.UpdateSidebarCategories(userID, teamID, categories)
}

func (api *PluginAPI) SearchUsers(search *model.UserSearch) ([]*model.User, *model.AppError) {
	pluginSearchUsersOptions := &model.UserSearchOptions{
		IsAdmin:       true,
		AllowInactive: search.AllowInactive,
		Limit:         search.Limit,
	}
	return api.app.SearchUsers(search, pluginSearchUsersOptions)
}

func (api *PluginAPI) SearchPostsInTeam(teamID string, paramsList []*model.SearchParams) ([]*model.Post, *model.AppError) {
	postList, err := api.app.SearchPostsInTeam(teamID, paramsList)
	if err != nil {
		return nil, err
	}
	return postList.ToSlice(), nil
}

func (api *PluginAPI) SearchPostsInTeamForUser(teamID string, userID string, searchParams model.SearchParameter) (*model.PostSearchResults, *model.AppError) {
	var terms string
	if searchParams.Terms != nil {
		terms = *searchParams.Terms
	}

	timeZoneOffset := 0
	if searchParams.TimeZoneOffset != nil {
		timeZoneOffset = *searchParams.TimeZoneOffset
	}

	isOrSearch := false
	if searchParams.IsOrSearch != nil {
		isOrSearch = *searchParams.IsOrSearch
	}

	page := 0
	if searchParams.Page != nil {
		page = *searchParams.Page
	}

	perPage := 100
	if searchParams.PerPage != nil {
		perPage = *searchParams.PerPage
	}

	includeDeletedChannels := false
	if searchParams.IncludeDeletedChannels != nil {
		includeDeletedChannels = *searchParams.IncludeDeletedChannels
	}

	return api.app.SearchPostsInTeamForUser(api.ctx, terms, userID, teamID, isOrSearch, includeDeletedChannels, timeZoneOffset, page, perPage)
}

func (api *PluginAPI) AddChannelMember(channelID, userID string) (*model.ChannelMember, *model.AppError) {
	channel, err := api.GetChannel(channelID)
	if err != nil {
		return nil, err
	}

	return api.app.AddChannelMember(api.ctx, userID, channel, ChannelMemberOpts{
		// For now, don't allow overriding these via the plugin API.
		UserRequestorID: "",
		PostRootID:      "",
	})
}

func (api *PluginAPI) AddUserToChannel(channelID, userID, asUserID string) (*model.ChannelMember, *model.AppError) {
	channel, err := api.GetChannel(channelID)
	if err != nil {
		return nil, err
	}

	return api.app.AddChannelMember(api.ctx, userID, channel, ChannelMemberOpts{
		UserRequestorID: asUserID,
	})
}

func (api *PluginAPI) GetChannelMember(channelID, userID string) (*model.ChannelMember, *model.AppError) {
	return api.app.GetChannelMember(context.Background(), channelID, userID)
}

func (api *PluginAPI) GetChannelMembers(channelID string, page, perPage int) (*model.ChannelMembers, *model.AppError) {
	return api.app.GetChannelMembersPage(channelID, page, perPage)
}

func (api *PluginAPI) GetChannelMembersByIds(channelID string, userIDs []string) (*model.ChannelMembers, *model.AppError) {
	return api.app.GetChannelMembersByIds(channelID, userIDs)
}

func (api *PluginAPI) GetChannelMembersForUser(teamID, userID string, page, perPage int) ([]*model.ChannelMember, *model.AppError) {
	return api.app.GetChannelMembersForUserWithPagination(teamID, userID, page, perPage)
}

func (api *PluginAPI) UpdateChannelMemberRoles(channelID, userID, newRoles string) (*model.ChannelMember, *model.AppError) {
	return api.app.UpdateChannelMemberRoles(channelID, userID, newRoles)
}

func (api *PluginAPI) UpdateChannelMemberNotifications(channelID, userID string, notifications map[string]string) (*model.ChannelMember, *model.AppError) {
	return api.app.UpdateChannelMemberNotifyProps(notifications, channelID, userID)
}

func (api *PluginAPI) DeleteChannelMember(channelID, userID string) *model.AppError {
	return api.app.LeaveChannel(api.ctx, channelID, userID)
}

func (api *PluginAPI) GetGroup(groupId string) (*model.Group, *model.AppError) {
	return api.app.GetGroup(groupId)
}

func (api *PluginAPI) GetGroupByName(name string) (*model.Group, *model.AppError) {
	return api.app.GetGroupByName(name, model.GroupSearchOpts{})
}

func (api *PluginAPI) GetGroupMemberUsers(groupID string, page, perPage int) ([]*model.User, *model.AppError) {
	users, _, err := api.app.GetGroupMemberUsersPage(groupID, page, perPage)

	return users, err
}

func (api *PluginAPI) GetGroupsBySource(groupSource model.GroupSource) ([]*model.Group, *model.AppError) {
	return api.app.GetGroupsBySource(groupSource)
}

func (api *PluginAPI) GetGroupsForUser(userID string) ([]*model.Group, *model.AppError) {
	return api.app.GetGroupsByUserId(userID)
}

func (api *PluginAPI) CreatePost(post *model.Post) (*model.Post, *model.AppError) {
	return api.app.CreatePostMissingChannel(api.ctx, post, true)
}

func (api *PluginAPI) AddReaction(reaction *model.Reaction) (*model.Reaction, *model.AppError) {
	return api.app.SaveReactionForPost(api.ctx, reaction)
}

func (api *PluginAPI) RemoveReaction(reaction *model.Reaction) *model.AppError {
	return api.app.DeleteReactionForPost(api.ctx, reaction)
}

func (api *PluginAPI) GetReactions(postID string) ([]*model.Reaction, *model.AppError) {
	return api.app.GetReactionsForPost(postID)
}

func (api *PluginAPI) SendEphemeralPost(userID string, post *model.Post) *model.Post {
	return api.app.SendEphemeralPost(userID, post)
}

func (api *PluginAPI) UpdateEphemeralPost(userID string, post *model.Post) *model.Post {
	return api.app.UpdateEphemeralPost(userID, post)
}

func (api *PluginAPI) DeleteEphemeralPost(userID, postID string) {
	api.app.DeleteEphemeralPost(userID, postID)
}

func (api *PluginAPI) DeletePost(postID string) *model.AppError {
	_, err := api.app.DeletePost(postID, api.id)
	return err
}

func (api *PluginAPI) GetPostThread(postID string) (*model.PostList, *model.AppError) {
	return api.app.GetPostThread(postID, false, false, false, "")
}

func (api *PluginAPI) GetPost(postID string) (*model.Post, *model.AppError) {
	return api.app.GetSinglePost(postID)
}

func (api *PluginAPI) GetPostsSince(channelID string, time int64) (*model.PostList, *model.AppError) {
	return api.app.GetPostsSince(model.GetPostsSinceOptions{ChannelId: channelID, Time: time})
}

func (api *PluginAPI) GetPostsAfter(channelID, postID string, page, perPage int) (*model.PostList, *model.AppError) {
	return api.app.GetPostsAfterPost(model.GetPostsOptions{ChannelId: channelID, PostId: postID, Page: page, PerPage: perPage})
}

func (api *PluginAPI) GetPostsBefore(channelID, postID string, page, perPage int) (*model.PostList, *model.AppError) {
	return api.app.GetPostsBeforePost(model.GetPostsOptions{ChannelId: channelID, PostId: postID, Page: page, PerPage: perPage})
}

func (api *PluginAPI) GetPostsForChannel(channelID string, page, perPage int) (*model.PostList, *model.AppError) {
	return api.app.GetPostsPage(model.GetPostsOptions{ChannelId: channelID, Page: page, PerPage: perPage})
}

func (api *PluginAPI) UpdatePost(post *model.Post) (*model.Post, *model.AppError) {
	return api.app.UpdatePost(api.ctx, post, false)
}

func (api *PluginAPI) GetProfileImage(userID string) ([]byte, *model.AppError) {
	user, err := api.app.GetUser(userID)
	if err != nil {
		return nil, err
	}

	data, _, err := api.app.GetProfileImage(user)
	return data, err
}

func (api *PluginAPI) SetProfileImage(userID string, data []byte) *model.AppError {
	_, err := api.app.GetUser(userID)
	if err != nil {
		return err
	}

	return api.app.SetProfileImageFromFile(userID, bytes.NewReader(data))
}

func (api *PluginAPI) GetEmojiList(sortBy string, page, perPage int) ([]*model.Emoji, *model.AppError) {
	return api.app.GetEmojiList(page, perPage, sortBy)
}

func (api *PluginAPI) GetEmojiByName(name string) (*model.Emoji, *model.AppError) {
	return api.app.GetEmojiByName(name)
}

func (api *PluginAPI) GetEmoji(emojiId string) (*model.Emoji, *model.AppError) {
	return api.app.GetEmoji(emojiId)
}

func (api *PluginAPI) CopyFileInfos(userID string, fileIDs []string) ([]string, *model.AppError) {
	return api.app.CopyFileInfos(userID, fileIDs)
}

func (api *PluginAPI) GetFileInfo(fileID string) (*model.FileInfo, *model.AppError) {
	return api.app.GetFileInfo(fileID)
}

func (api *PluginAPI) GetFileInfos(page, perPage int, opt *model.GetFileInfosOptions) ([]*model.FileInfo, *model.AppError) {
	return api.app.GetFileInfos(page, perPage, opt)
}

func (api *PluginAPI) GetFileLink(fileID string) (string, *model.AppError) {
	if !*api.app.Config().FileSettings.EnablePublicLink {
		return "", model.NewAppError("GetFileLink", "plugin_api.get_file_link.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	info, err := api.app.GetFileInfo(fileID)
	if err != nil {
		return "", err
	}

	if info.PostId == "" {
		return "", model.NewAppError("GetFileLink", "plugin_api.get_file_link.no_post.app_error", nil, "file_id="+info.Id, http.StatusBadRequest)
	}

	return api.app.GeneratePublicLink(api.app.GetSiteURL(), info), nil
}

func (api *PluginAPI) ReadFile(path string) ([]byte, *model.AppError) {
	return api.app.ReadFile(path)
}

func (api *PluginAPI) GetFile(fileID string) ([]byte, *model.AppError) {
	return api.app.GetFile(fileID)
}

func (api *PluginAPI) UploadFile(data []byte, channelID string, filename string) (*model.FileInfo, *model.AppError) {
	return api.app.UploadFile(api.ctx, data, channelID, filename)
}

func (api *PluginAPI) GetEmojiImage(emojiId string) ([]byte, string, *model.AppError) {
	return api.app.GetEmojiImage(emojiId)
}

func (api *PluginAPI) GetTeamIcon(teamID string) ([]byte, *model.AppError) {
	team, err := api.app.GetTeam(teamID)
	if err != nil {
		return nil, err
	}

	data, err := api.app.GetTeamIcon(team)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (api *PluginAPI) SetTeamIcon(teamID string, data []byte) *model.AppError {
	team, err := api.app.GetTeam(teamID)
	if err != nil {
		return err
	}

	return api.app.SetTeamIconFromFile(team, bytes.NewReader(data))
}

func (api *PluginAPI) OpenInteractiveDialog(dialog model.OpenDialogRequest) *model.AppError {
	return api.app.OpenInteractiveDialog(dialog)
}

func (api *PluginAPI) RemoveTeamIcon(teamID string) *model.AppError {
	_, err := api.app.GetTeam(teamID)
	if err != nil {
		return err
	}

	err = api.app.RemoveTeamIcon(teamID)
	if err != nil {
		return err
	}
	return nil
}

// Mail Section

func (api *PluginAPI) SendMail(to, subject, htmlBody string) *model.AppError {
	if to == "" {
		return model.NewAppError("SendMail", "plugin_api.send_mail.missing_to", nil, "", http.StatusBadRequest)
	}

	if subject == "" {
		return model.NewAppError("SendMail", "plugin_api.send_mail.missing_subject", nil, "", http.StatusBadRequest)
	}

	if htmlBody == "" {
		return model.NewAppError("SendMail", "plugin_api.send_mail.missing_htmlbody", nil, "", http.StatusBadRequest)
	}

	if err := api.app.Srv().EmailService.sendNotificationMail(to, subject, htmlBody); err != nil {
		return model.NewAppError("SendMail", "plugin_api.send_mail.missing_htmlbody", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

// Plugin Section

func (api *PluginAPI) GetPlugins() ([]*model.Manifest, *model.AppError) {
	plugins, err := api.app.GetPlugins()
	if err != nil {
		return nil, err
	}
	var manifests []*model.Manifest
	for _, manifest := range plugins.Active {
		manifests = append(manifests, &manifest.Manifest)
	}
	for _, manifest := range plugins.Inactive {
		manifests = append(manifests, &manifest.Manifest)
	}
	return manifests, nil
}

func (api *PluginAPI) EnablePlugin(id string) *model.AppError {
	return api.app.EnablePlugin(id)
}

func (api *PluginAPI) DisablePlugin(id string) *model.AppError {
	return api.app.DisablePlugin(id)
}

func (api *PluginAPI) RemovePlugin(id string) *model.AppError {
	return api.app.RemovePlugin(id)
}

func (api *PluginAPI) GetPluginStatus(id string) (*model.PluginStatus, *model.AppError) {
	return api.app.GetPluginStatus(id)
}

func (api *PluginAPI) InstallPlugin(file io.Reader, replace bool) (*model.Manifest, *model.AppError) {
	if !*api.app.Config().PluginSettings.Enable || !*api.app.Config().PluginSettings.EnableUploads {
		return nil, model.NewAppError("installPlugin", "app.plugin.upload_disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	fileBuffer, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, model.NewAppError("InstallPlugin", "api.plugin.upload.file.app_error", nil, "", http.StatusBadRequest)
	}

	return api.app.InstallPlugin(bytes.NewReader(fileBuffer), replace)
}

// KV Store Section

func (api *PluginAPI) KVSetWithOptions(key string, value []byte, options model.PluginKVSetOptions) (bool, *model.AppError) {
	return api.app.SetPluginKeyWithOptions(api.id, key, value, options)
}

func (api *PluginAPI) KVSet(key string, value []byte) *model.AppError {
	return api.app.SetPluginKey(api.id, key, value)
}

func (api *PluginAPI) KVCompareAndSet(key string, oldValue, newValue []byte) (bool, *model.AppError) {
	return api.app.CompareAndSetPluginKey(api.id, key, oldValue, newValue)
}

func (api *PluginAPI) KVCompareAndDelete(key string, oldValue []byte) (bool, *model.AppError) {
	return api.app.CompareAndDeletePluginKey(api.id, key, oldValue)
}

func (api *PluginAPI) KVSetWithExpiry(key string, value []byte, expireInSeconds int64) *model.AppError {
	return api.app.SetPluginKeyWithExpiry(api.id, key, value, expireInSeconds)
}

func (api *PluginAPI) KVGet(key string) ([]byte, *model.AppError) {
	return api.app.GetPluginKey(api.id, key)
}

func (api *PluginAPI) KVDelete(key string) *model.AppError {
	return api.app.DeletePluginKey(api.id, key)
}

func (api *PluginAPI) KVDeleteAll() *model.AppError {
	return api.app.DeleteAllKeysForPlugin(api.id)
}

func (api *PluginAPI) KVList(page, perPage int) ([]string, *model.AppError) {
	return api.app.ListPluginKeys(api.id, page, perPage)
}

func (api *PluginAPI) PublishWebSocketEvent(event string, payload map[string]interface{}, broadcast *model.WebsocketBroadcast) {
	ev := model.NewWebSocketEvent(fmt.Sprintf("custom_%v_%v", api.id, event), "", "", "", nil)
	ev = ev.SetBroadcast(broadcast).SetData(payload)
	api.app.Publish(ev)
}

func (api *PluginAPI) HasPermissionTo(userID string, permission *model.Permission) bool {
	return api.app.HasPermissionTo(userID, permission)
}

func (api *PluginAPI) HasPermissionToTeam(userID, teamID string, permission *model.Permission) bool {
	return api.app.HasPermissionToTeam(userID, teamID, permission)
}

func (api *PluginAPI) HasPermissionToChannel(userID, channelID string, permission *model.Permission) bool {
	return api.app.HasPermissionToChannel(userID, channelID, permission)
}

func (api *PluginAPI) LogDebug(msg string, keyValuePairs ...interface{}) {
	api.logger.Debug(msg, keyValuePairs...)
}
func (api *PluginAPI) LogInfo(msg string, keyValuePairs ...interface{}) {
	api.logger.Info(msg, keyValuePairs...)
}
func (api *PluginAPI) LogError(msg string, keyValuePairs ...interface{}) {
	api.logger.Error(msg, keyValuePairs...)
}
func (api *PluginAPI) LogWarn(msg string, keyValuePairs ...interface{}) {
	api.logger.Warn(msg, keyValuePairs...)
}

func (api *PluginAPI) CreateBot(bot *model.Bot) (*model.Bot, *model.AppError) {
	// Bots created by a plugin should use the plugin's ID for the creator field, unless
	// otherwise specified by the plugin.
	if bot.OwnerId == "" {
		bot.OwnerId = api.id
	}
	// Bots cannot be owners of other bots
	if user, err := api.app.GetUser(bot.OwnerId); err == nil {
		if user.IsBot {
			return nil, model.NewAppError("CreateBot", "plugin_api.bot_cant_create_bot", nil, "", http.StatusBadRequest)
		}
	}

	return api.app.CreateBot(api.ctx, bot)
}

func (api *PluginAPI) PatchBot(userID string, botPatch *model.BotPatch) (*model.Bot, *model.AppError) {
	return api.app.PatchBot(userID, botPatch)
}

func (api *PluginAPI) GetBot(userID string, includeDeleted bool) (*model.Bot, *model.AppError) {
	return api.app.GetBot(userID, includeDeleted)
}

func (api *PluginAPI) GetBots(options *model.BotGetOptions) ([]*model.Bot, *model.AppError) {
	bots, err := api.app.GetBots(options)

	return []*model.Bot(bots), err
}

func (api *PluginAPI) UpdateBotActive(userID string, active bool) (*model.Bot, *model.AppError) {
	return api.app.UpdateBotActive(api.ctx, userID, active)
}

func (api *PluginAPI) PermanentDeleteBot(userID string) *model.AppError {
	return api.app.PermanentDeleteBot(userID)
}

func (api *PluginAPI) GetBotIconImage(userID string) ([]byte, *model.AppError) {
	if _, err := api.app.GetBot(userID, true); err != nil {
		return nil, err
	}

	return api.app.GetBotIconImage(userID)
}

func (api *PluginAPI) SetBotIconImage(userID string, data []byte) *model.AppError {
	if _, err := api.app.GetBot(userID, true); err != nil {
		return err
	}

	return api.app.SetBotIconImage(userID, bytes.NewReader(data))
}

func (api *PluginAPI) DeleteBotIconImage(userID string) *model.AppError {
	if _, err := api.app.GetBot(userID, true); err != nil {
		return err
	}

	return api.app.DeleteBotIconImage(userID)
}

func (api *PluginAPI) PublishUserTyping(userID, channelID, parentId string) *model.AppError {
	return api.app.PublishUserTyping(userID, channelID, parentId)
}

func (api *PluginAPI) PluginHTTP(request *http.Request) *http.Response {
	split := strings.SplitN(request.URL.Path, "/", 3)
	if len(split) != 3 {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       ioutil.NopCloser(bytes.NewBufferString("Not enough URL. Form of URL should be /<pluginid>/*")),
		}
	}
	destinationPluginId := split[1]
	newURL, err := url.Parse("/" + split[2])
	newURL.RawQuery = request.URL.Query().Encode()
	request.URL = newURL
	if destinationPluginId == "" || err != nil {
		message := "No plugin specified. Form of URL should be /<pluginid>/*"
		if err != nil {
			message = "Form of URL should be /<pluginid>/* Error: " + err.Error()
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       ioutil.NopCloser(bytes.NewBufferString(message)),
		}
	}
	responseTransfer := &PluginResponseWriter{}
	api.app.ServeInterPluginRequest(responseTransfer, request, api.id, destinationPluginId)
	return responseTransfer.GenerateResponse()
}

func (api *PluginAPI) CreateCommand(cmd *model.Command) (*model.Command, error) {
	cmd.CreatorId = ""
	cmd.PluginId = api.id

	cmd, appErr := api.app.createCommand(cmd)

	if appErr != nil {
		return cmd, appErr
	}

	return cmd, nil
}

func (api *PluginAPI) ListCommands(teamID string) ([]*model.Command, error) {
	ret := make([]*model.Command, 0)

	cmds, err := api.ListPluginCommands(teamID)
	if err != nil {
		return nil, err
	}
	ret = append(ret, cmds...)

	cmds, err = api.ListBuiltInCommands()
	if err != nil {
		return nil, err
	}
	ret = append(ret, cmds...)

	cmds, err = api.ListCustomCommands(teamID)
	if err != nil {
		return nil, err
	}
	ret = append(ret, cmds...)

	return ret, nil
}

func (api *PluginAPI) ListCustomCommands(teamID string) ([]*model.Command, error) {
	// Plugins are allowed to bypass the a.Config().ServiceSettings.EnableCommands setting.
	return api.app.Srv().Store.Command().GetByTeam(teamID)
}

func (api *PluginAPI) ListPluginCommands(teamID string) ([]*model.Command, error) {
	commands := make([]*model.Command, 0)
	seen := make(map[string]bool)

	for _, cmd := range api.app.PluginCommandsForTeam(teamID) {
		if !seen[cmd.Trigger] {
			seen[cmd.Trigger] = true
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

func (api *PluginAPI) ListBuiltInCommands() ([]*model.Command, error) {
	commands := make([]*model.Command, 0)
	seen := make(map[string]bool)

	for _, value := range commandProviders {
		if cmd := value.GetCommand(api.app, i18n.T); cmd != nil {
			cpy := *cmd
			if cpy.AutoComplete && !seen[cpy.Trigger] {
				cpy.Sanitize()
				seen[cpy.Trigger] = true
				commands = append(commands, &cpy)
			}
		}
	}

	return commands, nil
}

func (api *PluginAPI) GetCommand(commandID string) (*model.Command, error) {
	return api.app.Srv().Store.Command().Get(commandID)
}

func (api *PluginAPI) UpdateCommand(commandID string, updatedCmd *model.Command) (*model.Command, error) {
	oldCmd, err := api.GetCommand(commandID)
	if err != nil {
		return nil, err
	}

	updatedCmd.Trigger = strings.ToLower(updatedCmd.Trigger)
	updatedCmd.Id = oldCmd.Id
	updatedCmd.Token = oldCmd.Token
	updatedCmd.CreateAt = oldCmd.CreateAt
	updatedCmd.UpdateAt = model.GetMillis()
	updatedCmd.DeleteAt = oldCmd.DeleteAt
	updatedCmd.PluginId = api.id
	if updatedCmd.TeamId == "" {
		updatedCmd.TeamId = oldCmd.TeamId
	}

	return api.app.Srv().Store.Command().Update(updatedCmd)
}

func (api *PluginAPI) DeleteCommand(commandID string) error {
	err := api.app.Srv().Store.Command().Delete(commandID, model.GetMillis())
	if err != nil {
		return err
	}

	return nil
}

func (api *PluginAPI) CreateOAuthApp(app *model.OAuthApp) (*model.OAuthApp, *model.AppError) {
	return api.app.CreateOAuthApp(app)
}

func (api *PluginAPI) GetOAuthApp(appID string) (*model.OAuthApp, *model.AppError) {
	return api.app.GetOAuthApp(appID)
}

func (api *PluginAPI) UpdateOAuthApp(app *model.OAuthApp) (*model.OAuthApp, *model.AppError) {
	oldApp, err := api.GetOAuthApp(app.Id)
	if err != nil {
		return nil, err
	}

	return api.app.UpdateOauthApp(oldApp, app)
}

func (api *PluginAPI) DeleteOAuthApp(appID string) *model.AppError {
	return api.app.DeleteOAuthApp(appID)
}

// PublishPluginClusterEvent broadcasts a plugin event to all other running instances of
// the calling plugin.
func (api *PluginAPI) PublishPluginClusterEvent(ev model.PluginClusterEvent,
	opts model.PluginClusterEventSendOptions) error {
	if api.app.Cluster() == nil {
		return nil
	}

	msg := &model.ClusterMessage{
		Event:            model.CLUSTER_EVENT_PLUGIN_EVENT,
		SendType:         opts.SendType,
		WaitForAllToSend: false,
		Props: map[string]string{
			"PluginID": api.id,
			"EventID":  ev.Id,
		},
		Data: string(ev.Data),
	}

	// If TargetId is empty we broadcast to all other cluster nodes.
	if opts.TargetId == "" {
		api.app.Cluster().SendClusterMessage(msg)
	} else {
		if err := api.app.Cluster().SendClusterMessageToNode(opts.TargetId, msg); err != nil {
			return fmt.Errorf("failed to send message to cluster node %q: %w", opts.TargetId, err)
		}
	}

	return nil
}

// RequestTrialLicense requests a trial license and installs it in the server
func (api *PluginAPI) RequestTrialLicense(requesterID string, users int, termsAccepted bool, receiveEmailsAccepted bool) *model.AppError {
	if *api.app.Config().ExperimentalSettings.RestrictSystemAdmin {
		return model.NewAppError("RequestTrialLicense", "api.restricted_system_admin", nil, "", http.StatusForbidden)
	}

	if !termsAccepted {
		return model.NewAppError("RequestTrialLicense", "api.license.request-trial.bad-request.terms-not-accepted", nil, "", http.StatusBadRequest)
	}

	if users == 0 {
		return model.NewAppError("RequestTrialLicense", "api.license.request-trial.bad-request", nil, "", http.StatusBadRequest)
	}

	requester, err := api.app.GetUser(requesterID)
	if err != nil {
		return err
	}

	trialLicenseRequest := &model.TrialLicenseRequest{
		ServerID:              api.app.TelemetryId(),
		Name:                  requester.GetDisplayName(model.SHOW_FULLNAME),
		Email:                 requester.Email,
		SiteName:              *api.app.Config().TeamSettings.SiteName,
		SiteURL:               *api.app.Config().ServiceSettings.SiteURL,
		Users:                 users,
		TermsAccepted:         termsAccepted,
		ReceiveEmailsAccepted: receiveEmailsAccepted,
	}

	if trialLicenseRequest.SiteURL == "" {
		return model.NewAppError("RequestTrialLicense", "api.license.request_trial_license.no-site-url.app_error", nil, "", http.StatusBadRequest)
	}

	return api.app.Srv().RequestTrialLicense(trialLicenseRequest)
}
