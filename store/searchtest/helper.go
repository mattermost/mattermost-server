// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchtest

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SearchTestHelper struct {
	Store              store.Store
	Team               *model.Team
	AnotherTeam        *model.Team
	User               *model.User
	User2              *model.User
	UserAnotherTeam    *model.User
	ChannelBasic       *model.Channel
	ChannelPrivate     *model.Channel
	ChannelAnotherTeam *model.Channel
	ChannelDeleted     *model.Channel
}

func (th *SearchTestHelper) SetupBasicFixtures() error {
	// Remove users from previous tests
	err := th.cleanAllUsers()
	if err != nil {
		return err
	}

	// Create teams
	team, err := th.createTeam("searchtest-team", "Searchtest team", model.TeamOpen)
	if err != nil {
		return err
	}
	anotherTeam, err := th.createTeam("another-searchtest-team", "Another Searchtest team", model.TeamOpen)
	if err != nil {
		return err
	}

	// Create users
	user, err := th.createUser("basicusername1", "basicnickname1", "basicfirstname1", "basiclastname1")
	if err != nil {
		return err
	}
	user2, err := th.createUser("basicusername2", "basicnickname2", "basicfirstname2", "basiclastname2")
	if err != nil {
		return err
	}
	useranother, err := th.createUser("basicusername3", "basicnickname3", "basicfirstname3", "basiclastname3")
	if err != nil {
		return err
	}

	// Create channels
	channelBasic, err := th.createChannel(team.ID, "channel-a", "ChannelA", "", model.ChannelTypeOpen, false)
	if err != nil {
		return err
	}
	channelPrivate, err := th.createChannel(team.ID, "channel-private", "ChannelPrivate", "", model.ChannelTypePrivate, false)
	if err != nil {
		return err
	}
	channelDeleted, err := th.createChannel(team.ID, "channel-deleted", "ChannelA (deleted)", "", model.ChannelTypeOpen, true)
	if err != nil {
		return err
	}
	channelAnotherTeam, err := th.createChannel(anotherTeam.ID, "channel-a", "ChannelA", "", model.ChannelTypeOpen, false)
	if err != nil {
		return err
	}

	err = th.addUserToTeams(user, []string{team.ID, anotherTeam.ID})
	if err != nil {
		return err
	}

	err = th.addUserToTeams(user2, []string{team.ID, anotherTeam.ID})
	if err != nil {
		return err
	}

	err = th.addUserToTeams(useranother, []string{anotherTeam.ID})
	if err != nil {
		return err
	}

	err = th.addUserToChannels(user, []string{channelBasic.ID, channelPrivate.ID, channelDeleted.ID})
	if err != nil {
		return err
	}

	err = th.addUserToChannels(user2, []string{channelPrivate.ID, channelDeleted.ID})
	if err != nil {
		return err
	}

	err = th.addUserToChannels(useranother, []string{channelAnotherTeam.ID})
	if err != nil {
		return err
	}

	th.Team = team
	th.AnotherTeam = anotherTeam
	th.User = user
	th.User2 = user2
	th.UserAnotherTeam = useranother
	th.ChannelBasic = channelBasic
	th.ChannelPrivate = channelPrivate
	th.ChannelAnotherTeam = channelAnotherTeam
	th.ChannelDeleted = channelDeleted

	return nil
}

func (th *SearchTestHelper) CleanFixtures() error {
	err := th.deleteChannels([]*model.Channel{
		th.ChannelBasic, th.ChannelPrivate, th.ChannelAnotherTeam, th.ChannelDeleted,
	})
	if err != nil {
		return err
	}

	err = th.deleteTeam(th.Team)
	if err != nil {
		return err
	}

	err = th.deleteTeam(th.AnotherTeam)
	if err != nil {
		return err
	}

	err = th.cleanAllUsers()
	if err != nil {
		return err
	}

	return nil
}

func (th *SearchTestHelper) createTeam(name, displayName, teamType string) (*model.Team, error) {
	return th.Store.Team().Save(&model.Team{
		Name:        name,
		DisplayName: displayName,
		Type:        teamType,
	})
}

func (th *SearchTestHelper) deleteTeam(team *model.Team) error {
	err := th.Store.Team().RemoveAllMembersByTeam(team.ID)
	if err != nil {
		return err
	}
	return th.Store.Team().PermanentDelete(team.ID)
}

func (th *SearchTestHelper) makeEmail() string {
	return "success_" + model.NewID() + "@simulator.amazon.com"
}

func (th *SearchTestHelper) createUser(username, nickname, firstName, lastName string) (*model.User, error) {
	return th.Store.User().Save(&model.User{
		Username:  username,
		Password:  username,
		Nickname:  nickname,
		FirstName: firstName,
		LastName:  lastName,
		Email:     th.makeEmail(),
	})
}

func (th *SearchTestHelper) createGuest(username, nickname, firstName, lastName string) (*model.User, error) {
	return th.Store.User().Save(&model.User{
		Username:  username,
		Password:  username,
		Nickname:  nickname,
		FirstName: firstName,
		LastName:  lastName,
		Email:     th.makeEmail(),
		Roles:     model.SystemGuestRoleID,
	})
}

func (th *SearchTestHelper) deleteUser(user *model.User) error {
	return th.Store.User().PermanentDelete(user.ID)
}

func (th *SearchTestHelper) cleanAllUsers() error {
	users, err := th.Store.User().GetAll()
	if err != nil {
		return err
	}

	for _, u := range users {
		err := th.deleteUser(u)
		if err != nil {
			return err
		}
	}

	return nil
}

func (th *SearchTestHelper) createBot(username, displayName, ownerID string) (*model.Bot, error) {
	botModel := &model.Bot{
		Username:    username,
		DisplayName: displayName,
		OwnerID:     ownerID,
	}

	user, err := th.Store.User().Save(model.UserFromBot(botModel))
	if err != nil {
		return nil, errors.New(err.Error())
	}

	botModel.UserID = user.ID
	bot, err := th.Store.Bot().Save(botModel)
	if err != nil {
		th.Store.User().PermanentDelete(bot.UserID)
		return nil, errors.New(err.Error())
	}

	return bot, nil
}

func (th *SearchTestHelper) deleteBot(botID string) error {
	err := th.Store.Bot().PermanentDelete(botID)
	if err != nil {
		return errors.New(err.Error())
	}
	return nil
}

func (th *SearchTestHelper) createChannel(teamID, name, displayName, purpose, channelType string, deleted bool) (*model.Channel, error) {
	channel, err := th.Store.Channel().Save(&model.Channel{
		TeamID:      teamID,
		DisplayName: displayName,
		Name:        name,
		Type:        channelType,
		Purpose:     purpose,
	}, 999)
	if err != nil {
		return nil, err
	}

	if deleted {
		err := th.Store.Channel().Delete(channel.ID, model.GetMillis())
		if err != nil {
			return nil, err
		}
	}

	return channel, nil
}

func (th *SearchTestHelper) createDirectChannel(teamID, name, displayName string, users []*model.User) (*model.Channel, error) {
	channel := &model.Channel{
		TeamID:      teamID,
		Name:        name,
		DisplayName: displayName,
		Type:        model.ChannelTypeDirect,
	}

	m1 := &model.ChannelMember{}
	m1.ChannelID = channel.ID
	m1.UserID = users[0].ID
	m1.NotifyProps = model.GetDefaultChannelNotifyProps()

	m2 := &model.ChannelMember{}
	m2.ChannelID = channel.ID
	m2.UserID = users[0].ID
	m2.NotifyProps = model.GetDefaultChannelNotifyProps()

	channel, err := th.Store.Channel().SaveDirectChannel(channel, m1, m2)
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func (th *SearchTestHelper) createGroupChannel(teamID, displayName string, users []*model.User) (*model.Channel, error) {
	userIDS := make([]string, len(users))
	for _, user := range users {
		userIDS = append(userIDS, user.ID)
	}

	group := &model.Channel{
		TeamID:      teamID,
		Name:        model.GetGroupNameFromUserIDs(userIDS),
		DisplayName: displayName,
		Type:        model.ChannelTypeGroup,
	}

	channel, err := th.Store.Channel().Save(group, 10000)
	if err != nil {
		return nil, errors.New(err.Error())
	}

	for _, user := range users {
		err := th.addUserToChannels(user, []string{channel.ID})
		if err != nil {
			return nil, err
		}
	}

	return channel, nil

}

func (th *SearchTestHelper) deleteChannel(channel *model.Channel) error {
	err := th.Store.Channel().PermanentDeleteMembersByChannel(channel.ID)
	if err != nil {
		return err
	}

	return th.Store.Channel().PermanentDelete(channel.ID)
}

func (th *SearchTestHelper) deleteChannels(channels []*model.Channel) error {
	for _, channel := range channels {
		err := th.deleteChannel(channel)
		if err != nil {
			return err
		}
	}

	return nil
}

func (th *SearchTestHelper) createPostModel(userID, channelID, message, hashtags, postType string, createAt int64, pinned bool) *model.Post {
	return &model.Post{
		Message:       message,
		ChannelID:     channelID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        userID,
		Hashtags:      hashtags,
		IsPinned:      pinned,
		CreateAt:      createAt,
		Type:          postType,
	}
}

func (th *SearchTestHelper) createPost(userID, channelID, message, hashtags, postType string, createAt int64, pinned bool) (*model.Post, error) {
	var creationTime int64 = 1000000
	if createAt > 0 {
		creationTime = createAt
	}
	postModel := th.createPostModel(userID, channelID, message, hashtags, postType, creationTime, pinned)
	return th.Store.Post().Save(postModel)
}

func (th *SearchTestHelper) createFileInfoModel(creatorID, postID, name, content, extension, mimeType string, createAt, size int64) *model.FileInfo {
	return &model.FileInfo{
		CreatorID: creatorID,
		PostID:    postID,
		CreateAt:  createAt,
		UpdateAt:  createAt,
		DeleteAt:  0,
		Name:      name,
		Content:   content,
		Path:      name,
		Extension: extension,
		Size:      size,
		MimeType:  mimeType,
	}
}

func (th *SearchTestHelper) createFileInfo(creatorID, postID, name, content, extension, mimeType string, createAt, size int64) (*model.FileInfo, error) {
	var creationTime int64 = 1000000
	if createAt > 0 {
		creationTime = createAt
	}
	fileInfoModel := th.createFileInfoModel(creatorID, postID, name, content, extension, mimeType, creationTime, size)
	return th.Store.FileInfo().Save(fileInfoModel)
}

func (th *SearchTestHelper) createReply(userID, message, hashtags string, parent *model.Post, createAt int64, pinned bool) (*model.Post, error) {
	replyModel := th.createPostModel(userID, parent.ChannelID, message, hashtags, parent.Type, createAt, pinned)
	replyModel.ParentID = parent.ID
	replyModel.RootID = parent.ID
	return th.Store.Post().Save(replyModel)
}

func (th *SearchTestHelper) deleteUserPosts(userID string) error {
	err := th.Store.Post().PermanentDeleteByUser(userID)
	if err != nil {
		return errors.New(err.Error())
	}
	return nil
}

func (th *SearchTestHelper) deleteUserFileInfos(userID string) error {
	if _, err := th.Store.FileInfo().PermanentDeleteByUser(userID); err != nil {
		return errors.New(err.Error())
	}
	return nil
}

func (th *SearchTestHelper) addUserToTeams(user *model.User, teamIDS []string) error {
	for _, teamID := range teamIDS {
		_, err := th.Store.Team().SaveMember(&model.TeamMember{TeamID: teamID, UserID: user.ID}, -1)
		if err != nil {
			return errors.New(err.Error())
		}
	}

	return nil
}

func (th *SearchTestHelper) addUserToChannels(user *model.User, channelIDS []string) error {
	for _, channelID := range channelIDS {
		_, err := th.Store.Channel().SaveMember(&model.ChannelMember{
			ChannelID:   channelID,
			UserID:      user.ID,
			NotifyProps: model.GetDefaultChannelNotifyProps(),
		})
		if err != nil {
			return errors.New(err.Error())
		}
	}

	return nil
}

func (th *SearchTestHelper) assertUsersMatchInAnyOrder(t *testing.T, expected, actual []*model.User) {
	expectedUsernames := make([]string, 0, len(expected))
	for _, user := range expected {
		user.Sanitize(map[string]bool{})
		expectedUsernames = append(expectedUsernames, user.Username)
	}

	actualUsernames := make([]string, 0, len(actual))
	for _, user := range actual {
		user.Sanitize(map[string]bool{})
		actualUsernames = append(actualUsernames, user.Username)
	}

	if assert.ElementsMatch(t, expectedUsernames, actualUsernames) {
		assert.ElementsMatch(t, expected, actual)
	}
}

func (th *SearchTestHelper) checkPostInSearchResults(t *testing.T, postID string, searchResults map[string]*model.Post) {
	t.Helper()
	postIDS := make([]string, len(searchResults))
	for ID := range searchResults {
		postIDS = append(postIDS, ID)
	}
	assert.Contains(t, postIDS, postID, "Did not find expected post in search results.")
}

func (th *SearchTestHelper) checkFileInfoInSearchResults(t *testing.T, fileID string, searchResults map[string]*model.FileInfo) {
	t.Helper()
	fileIDS := make([]string, len(searchResults))
	for ID := range searchResults {
		fileIDS = append(fileIDS, ID)
	}
	assert.Contains(t, fileIDS, fileID, "Did not find expected file in search results.")
}

func (th *SearchTestHelper) checkChannelIDsMatch(t *testing.T, expected []string, results *model.ChannelList) {
	t.Helper()
	channelIDs := make([]string, len(*results))
	for i, channel := range *results {
		channelIDs[i] = channel.ID
	}
	require.ElementsMatch(t, expected, channelIDs)
}

type ByChannelDisplayName model.ChannelList

func (s ByChannelDisplayName) Len() int { return len(s) }
func (s ByChannelDisplayName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ByChannelDisplayName) Less(i, j int) bool {
	if s[i].DisplayName != s[j].DisplayName {
		return s[i].DisplayName < s[j].DisplayName
	}

	return s[i].ID < s[j].ID
}
