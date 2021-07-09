// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// https://github.com/mattermost/mattermost-plugin-starter-template/issues/115
func TestClient4TrimTrailingSlash(t *testing.T) {
	slashes := []int{0, 1, 5}
	baseURL := "https://foo.com:1234"

	for _, s := range slashes {
		testURL := baseURL + strings.Repeat("/", s)
		client := NewAPIv4Client(testURL)
		assert.Equal(t, baseURL, client.URL)
		assert.Equal(t, baseURL+APIURLSuffix, client.APIURL)
	}
}

// https://github.com/mattermost/mattermost-server/v5/issues/8205
func TestClient4CreatePost(t *testing.T) {
	post := &Post{
		Props: map[string]interface{}{
			"attachments": []*SlackAttachment{
				{
					Actions: []*PostAction{
						{
							Integration: &PostActionIntegration{
								Context: map[string]interface{}{
									"foo": "bar",
								},
								URL: "http://foo.com",
							},
							Name: "Foo",
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attachments := PostFromJSON(r.Body).Attachments()
		assert.Equal(t, []*SlackAttachment{
			{
				Actions: []*PostAction{
					{
						Integration: &PostActionIntegration{
							Context: map[string]interface{}{
								"foo": "bar",
							},
							URL: "http://foo.com",
						},
						Name: "Foo",
					},
				},
			},
		}, attachments)
	}))

	client := NewAPIv4Client(server.URL)
	_, resp := client.CreatePost(post)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient4SetToken(t *testing.T) {
	expected := NewID()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(HeaderAuth)

		token := strings.Split(authHeader, HeaderBearer)

		if len(token) < 2 {
			t.Errorf("wrong authorization header format, got %s, expected: %s %s", authHeader, HeaderBearer, expected)
		}

		assert.Equal(t, expected, strings.TrimSpace(token[1]))
	}))

	client := NewAPIv4Client(server.URL)
	client.SetToken(expected)

	_, resp := client.GetMe("")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient4MockSession(t *testing.T) {
	expected := NewID()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(HeaderAuth)

		token := strings.Split(authHeader, HeaderBearer)

		if len(token) < 2 {
			t.Errorf("wrong authorization header format, got %s, expected: %s %s", authHeader, HeaderBearer, expected)
		}

		assert.Equal(t, expected, strings.TrimSpace(token[1]))
	}))

	client := NewAPIv4Client(server.URL)
	client.MockSession(expected)

	_, resp := client.GetMe("")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
