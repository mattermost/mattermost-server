// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sharedchannel

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func TestProcessPermalinkToRemote(t *testing.T) {
	scs := &Service{
		server: &MockServerIface{},
		app:    &MockAppIface{},
	}

	mockStore := &mocks.Store{}
	mockPostStore := mocks.PostStore{}
	utils.TranslationsPreInit()

	pl := &model.PostList{}
	mockPostStore.On("Get", context.Background(), "postID", true, false, false, "").Return(pl, nil)

	mockStore.On("Post").Return(&mockPostStore)

	mockServer := scs.server.(*MockServerIface)
	mockServer.On("GetStore").Return(mockStore)

	mockApp := scs.app.(*MockAppIface)
	mockApp.On("SendEphemeralPost", "user", mock.AnythingOfType("*model.Post")).Return(&model.Post{}).Times(1)
	defer mockApp.AssertExpectations(t)

	t.Run("same channel", func(t *testing.T) {
		post := &model.Post{
			Message:   "hello world https://comm.matt.com/team/pl/postID link",
			ChannelID: "sourceChan",
			UserID:    "user",
		}

		*pl = model.PostList{
			Order: []string{"1"},
			Posts: map[string]*model.Post{
				"1": {
					ChannelID: "sourceChan",
					UserID:    "user",
				},
			},
		}

		out := scs.processPermalinkToRemote(post)
		assert.Equal(t, "hello world https://comm.matt.com/team/plshared/postID link", out)
	})

	t.Run("different channel", func(t *testing.T) {
		post := &model.Post{
			Message:   "hello world https://comm.matt.com/team/pl/postID link https://comm.matt.com/team/pl/postID ",
			ChannelID: "sourceChan",
			UserID:    "user",
		}

		*pl = model.PostList{
			Order: []string{"1"},
			Posts: map[string]*model.Post{
				"1": {
					ChannelID: "otherChan",
				},
			},
		}
		out := scs.processPermalinkToRemote(post)
		assert.Equal(t, "hello world https://comm.matt.com/team/pl/postID link https://comm.matt.com/team/pl/postID ", out)
	})
}

func TestProcessPermalinkFromRemote(t *testing.T) {
	t.Run("has match", func(t *testing.T) {
		parsed, _ := url.Parse("http://mysite.com")
		scs := &Service{
			server:  &MockServerIface{},
			siteURL: parsed,
		}

		out := scs.processPermalinkFromRemote(&model.Post{Message: "hello world https://comm.matt.com/team/plshared/postID link"},
			&model.Team{Name: "myteam"})
		assert.Equal(t,
			"hello world http://mysite.com/myteam/pl/postID link",
			out)
	})

	t.Run("does not match", func(t *testing.T) {
		parsed, _ := url.Parse("http://mysite.com")
		scs := &Service{
			server:  &MockServerIface{},
			siteURL: parsed,
		}

		out := scs.processPermalinkFromRemote(&model.Post{Message: "hello world https://comm.matt.com/team/pl/postID link"},
			&model.Team{Name: "myteam"})
		assert.Equal(t,
			"hello world https://comm.matt.com/team/pl/postID link",
			out)
	})
}
