// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/cache"
	cachemocks "github.com/mattermost/mattermost-server/v5/services/cache/mocks"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/sqlstore"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/testlib"
)

var mainHelper *testlib.MainHelper

func getMockCacheProvider() cache.Provider {
	mockCacheProvider := cachemocks.Provider{}
	mockCacheProvider.On("NewCache", mock.Anything).
		Return(cache.NewLRU(cache.LRUOptions{Size: 128}), nil)
	return &mockCacheProvider
}

func getMockStore() *mocks.Store {
	mockStore := mocks.Store{}

	fakeReaction := model.Reaction{PostID: "123"}
	mockReactionsStore := mocks.ReactionStore{}
	mockReactionsStore.On("Save", &fakeReaction).Return(&model.Reaction{}, nil)
	mockReactionsStore.On("Delete", &fakeReaction).Return(&model.Reaction{}, nil)
	mockReactionsStore.On("GetForPost", "123", false).Return([]*model.Reaction{&fakeReaction}, nil)
	mockReactionsStore.On("GetForPost", "123", true).Return([]*model.Reaction{&fakeReaction}, nil)
	mockStore.On("Reaction").Return(&mockReactionsStore)

	fakeRole := model.Role{ID: "123", Name: "role-name"}
	mockRolesStore := mocks.RoleStore{}
	mockRolesStore.On("Save", &fakeRole).Return(&model.Role{}, nil)
	mockRolesStore.On("Delete", "123").Return(&fakeRole, nil)
	mockRolesStore.On("GetByName", context.Background(), "role-name").Return(&fakeRole, nil)
	mockRolesStore.On("GetByNames", []string{"role-name"}).Return([]*model.Role{&fakeRole}, nil)
	mockRolesStore.On("PermanentDeleteAll").Return(nil)
	mockStore.On("Role").Return(&mockRolesStore)

	fakeScheme := model.Scheme{ID: "123", Name: "scheme-name"}
	mockSchemesStore := mocks.SchemeStore{}
	mockSchemesStore.On("Save", &fakeScheme).Return(&model.Scheme{}, nil)
	mockSchemesStore.On("Delete", "123").Return(&model.Scheme{}, nil)
	mockSchemesStore.On("Get", "123").Return(&fakeScheme, nil)
	mockSchemesStore.On("PermanentDeleteAll").Return(nil)
	mockStore.On("Scheme").Return(&mockSchemesStore)

	fakeFileInfo := model.FileInfo{PostID: "123"}
	mockFileInfoStore := mocks.FileInfoStore{}
	mockFileInfoStore.On("GetForPost", "123", true, true, false).Return([]*model.FileInfo{&fakeFileInfo}, nil)
	mockFileInfoStore.On("GetForPost", "123", true, true, true).Return([]*model.FileInfo{&fakeFileInfo}, nil)
	mockStore.On("FileInfo").Return(&mockFileInfoStore)

	fakeWebhook := model.IncomingWebhook{ID: "123"}
	mockWebhookStore := mocks.WebhookStore{}
	mockWebhookStore.On("GetIncoming", "123", true).Return(&fakeWebhook, nil)
	mockWebhookStore.On("GetIncoming", "123", false).Return(&fakeWebhook, nil)
	mockStore.On("Webhook").Return(&mockWebhookStore)

	fakeEmoji := model.Emoji{ID: "123", Name: "name123"}
	ctxEmoji := model.Emoji{ID: "master", Name: "name123"}
	mockEmojiStore := mocks.EmojiStore{}
	mockEmojiStore.On("Get", mock.Anything, "123", true).Return(&fakeEmoji, nil)
	mockEmojiStore.On("Get", mock.Anything, "123", false).Return(&fakeEmoji, nil)
	mockEmojiStore.On("Get", context.Background(), "master", true).Return(&ctxEmoji, nil)
	mockEmojiStore.On("Get", sqlstore.WithMaster(context.Background()), "master", true).Return(&ctxEmoji, nil)
	mockEmojiStore.On("GetByName", mock.Anything, "name123", true).Return(&fakeEmoji, nil)
	mockEmojiStore.On("GetByName", mock.Anything, "name123", false).Return(&fakeEmoji, nil)
	mockEmojiStore.On("GetByName", context.Background(), "master", true).Return(&ctxEmoji, nil)
	mockEmojiStore.On("GetByName", sqlstore.WithMaster(context.Background()), "master", false).Return(&ctxEmoji, nil)
	mockEmojiStore.On("Delete", &fakeEmoji, int64(0)).Return(nil)
	mockEmojiStore.On("Delete", &ctxEmoji, int64(0)).Return(nil)
	mockStore.On("Emoji").Return(&mockEmojiStore)

	mockCount := int64(10)
	mockGuestCount := int64(12)
	channelID := "channel1"
	fakeChannelID := model.Channel{ID: channelID}
	mockChannelStore := mocks.ChannelStore{}
	mockChannelStore.On("ClearCaches").Return()
	mockChannelStore.On("GetMemberCount", "id", true).Return(mockCount, nil)
	mockChannelStore.On("GetMemberCount", "id", false).Return(mockCount, nil)
	mockChannelStore.On("GetGuestCount", "id", true).Return(mockGuestCount, nil)
	mockChannelStore.On("GetGuestCount", "id", false).Return(mockGuestCount, nil)
	mockChannelStore.On("Get", channelID, true).Return(&fakeChannelID, nil)
	mockChannelStore.On("Get", channelID, false).Return(&fakeChannelID, nil)
	mockStore.On("Channel").Return(&mockChannelStore)

	mockPinnedPostsCount := int64(10)
	mockChannelStore.On("GetPinnedPostCount", "id", true).Return(mockPinnedPostsCount, nil)
	mockChannelStore.On("GetPinnedPostCount", "id", false).Return(mockPinnedPostsCount, nil)

	fakePosts := &model.PostList{}
	fakeOptions := model.GetPostsOptions{ChannelID: "123", PerPage: 30}
	mockPostStore := mocks.PostStore{}
	mockPostStore.On("GetPosts", fakeOptions, true).Return(fakePosts, nil)
	mockPostStore.On("GetPosts", fakeOptions, false).Return(fakePosts, nil)
	mockPostStore.On("InvalidateLastPostTimeCache", "12360")

	mockPostStoreOptions := model.GetPostsSinceOptions{
		ChannelID:        "channelId",
		Time:             1,
		SkipFetchThreads: false,
	}

	mockPostStoreEtagResult := fmt.Sprintf("%v.%v", model.CurrentVersion, 1)
	mockPostStore.On("ClearCaches")
	mockPostStore.On("InvalidateLastPostTimeCache", "channelId")
	mockPostStore.On("GetEtag", "channelId", true, false).Return(mockPostStoreEtagResult)
	mockPostStore.On("GetEtag", "channelId", false, false).Return(mockPostStoreEtagResult)
	mockPostStore.On("GetPostsSince", mockPostStoreOptions, true).Return(model.NewPostList(), nil)
	mockPostStore.On("GetPostsSince", mockPostStoreOptions, false).Return(model.NewPostList(), nil)
	mockStore.On("Post").Return(&mockPostStore)

	fakeTermsOfService := model.TermsOfService{ID: "123", CreateAt: 11111, UserID: "321", Text: "Terms of service test"}
	mockTermsOfServiceStore := mocks.TermsOfServiceStore{}
	mockTermsOfServiceStore.On("InvalidateTermsOfService", "123")
	mockTermsOfServiceStore.On("Save", &fakeTermsOfService).Return(&fakeTermsOfService, nil)
	mockTermsOfServiceStore.On("GetLatest", true).Return(&fakeTermsOfService, nil)
	mockTermsOfServiceStore.On("GetLatest", false).Return(&fakeTermsOfService, nil)
	mockTermsOfServiceStore.On("Get", "123", true).Return(&fakeTermsOfService, nil)
	mockTermsOfServiceStore.On("Get", "123", false).Return(&fakeTermsOfService, nil)
	mockStore.On("TermsOfService").Return(&mockTermsOfServiceStore)

	fakeUser := []*model.User{{
		ID:          "123",
		AuthData:    model.NewString("authData"),
		AuthService: "authService",
	}}
	mockUserStore := mocks.UserStore{}
	mockUserStore.On("GetProfileByIds", mock.Anything, []string{"123"}, &store.UserGetByIDsOpts{}, true).Return(fakeUser, nil)
	mockUserStore.On("GetProfileByIds", mock.Anything, []string{"123"}, &store.UserGetByIDsOpts{}, false).Return(fakeUser, nil)

	fakeProfilesInChannelMap := map[string]*model.User{
		"456": {ID: "456"},
	}
	mockUserStore.On("GetAllProfilesInChannel", mock.Anything, "123", true).Return(fakeProfilesInChannelMap, nil)
	mockUserStore.On("GetAllProfilesInChannel", mock.Anything, "123", false).Return(fakeProfilesInChannelMap, nil)

	mockUserStore.On("Get", mock.Anything, "123").Return(fakeUser[0], nil)
	users := []*model.User{
		fakeUser[0],
		{
			ID:          "456",
			AuthData:    model.NewString("authData"),
			AuthService: "authService",
		},
	}
	mockUserStore.On("GetMany", mock.Anything, []string{"123", "456"}).Return(users, nil)
	mockUserStore.On("GetMany", mock.Anything, []string{"123"}).Return(users[0:1], nil)
	mockStore.On("User").Return(&mockUserStore)

	fakeUserTeamIDs := []string{"1", "2", "3"}
	mockTeamStore := mocks.TeamStore{}
	mockTeamStore.On("GetUserTeamIds", "123", true).Return(fakeUserTeamIDs, nil)
	mockTeamStore.On("GetUserTeamIds", "123", false).Return(fakeUserTeamIDs, nil)
	mockStore.On("Team").Return(&mockTeamStore)

	return &mockStore
}

func TestMain(m *testing.M) {
	mlog.DisableZap()
	mainHelper = testlib.NewMainHelperWithOptions(nil)
	defer mainHelper.Close()

	initStores()
	mainHelper.Main(m)
	tearDownStores()
}
