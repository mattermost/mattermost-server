// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

// Test for MM-13598 where an invalid integration URL was causing a crash
func TestPostActionInvalidURL(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := model.PostActionIntegrationRequestFromJson(r.Body)
		assert.NotNil(t, request)
	}))
	defer ts.Close()

	interactivePost := model.Post{
		Message:       "Interactive post",
		ChannelId:     th.BasicChannel.Id,
		PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
		UserId:        th.BasicUser.Id,
		Props: model.StringInterface{
			"attachments": []*model.SlackAttachment{
				{
					Text: "hello",
					Actions: []*model.PostAction{
						{
							Integration: &model.PostActionIntegration{
								URL: ":test",
							},
							Name: "action",
							Type: "some_type",
						},
					},
				},
			},
		},
	}

	post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
	require.Nil(t, err)
	attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
	require.True(t, ok)
	require.NotEmpty(t, attachments[0].Actions)
	require.NotEmpty(t, attachments[0].Actions[0].Id)

	_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "missing protocol scheme"))
}

func TestPostActionEmptyResponse(t *testing.T) {
	t.Run("Empty response on post action", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		channel := th.BasicChannel

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
		})

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer ts.Close()

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     channel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									Context: model.StringInterface{
										"s": "foo",
										"n": 3,
									},
									URL: ts.URL,
								},
								Name:       "action",
								Type:       "some_type",
								DataSource: "some_source",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)

		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.Nil(t, err)
	})
}

func TestPostAction(t *testing.T) {
	testCases := []struct {
		Description string
		Channel     func(th *TestHelper) *model.Channel
	}{
		{"public channel", func(th *TestHelper) *model.Channel {
			return th.BasicChannel
		}},
		{"direct channel", func(th *TestHelper) *model.Channel {
			user1 := th.CreateUser()

			return th.CreateDmChannel(user1)
		}},
		{"group channel", func(th *TestHelper) *model.Channel {
			user1 := th.CreateUser()
			user2 := th.CreateUser()

			return th.CreateGroupChannel(user1, user2)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			th := Setup(t).InitBasic()
			defer th.TearDown()

			channel := testCase.Channel(th)

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
			})

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request := model.PostActionIntegrationRequestFromJson(r.Body)
				assert.NotNil(t, request)

				assert.Equal(t, request.UserId, th.BasicUser.Id)
				assert.Equal(t, request.UserName, th.BasicUser.Username)
				assert.Equal(t, request.ChannelId, channel.Id)
				assert.Equal(t, request.ChannelName, channel.Name)
				if channel.Type == model.CHANNEL_DIRECT || channel.Type == model.CHANNEL_GROUP {
					assert.Empty(t, request.TeamId)
					assert.Empty(t, request.TeamName)
				} else {
					assert.Equal(t, request.TeamId, th.BasicTeam.Id)
					assert.Equal(t, request.TeamName, th.BasicTeam.Name)
				}
				assert.True(t, request.TriggerId != "")
				if request.Type == model.POST_ACTION_TYPE_SELECT {
					assert.Equal(t, request.DataSource, "some_source")
					assert.Equal(t, request.Context["selected_option"], "selected")
				} else {
					assert.Equal(t, request.DataSource, "")
				}
				assert.Equal(t, "foo", request.Context["s"])
				assert.EqualValues(t, 3, request.Context["n"])
				fmt.Fprintf(w, `{"post": {"message": "updated"}, "ephemeral_text": "foo"}`)
			}))
			defer ts.Close()

			interactivePost := model.Post{
				Message:       "Interactive post",
				ChannelId:     channel.Id,
				PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
				UserId:        th.BasicUser.Id,
				Props: model.StringInterface{
					"attachments": []*model.SlackAttachment{
						{
							Text: "hello",
							Actions: []*model.PostAction{
								{
									Integration: &model.PostActionIntegration{
										Context: model.StringInterface{
											"s": "foo",
											"n": 3,
										},
										URL: ts.URL,
									},
									Name:       "action",
									Type:       "some_type",
									DataSource: "some_source",
								},
							},
						},
					},
				},
			}

			post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
			require.Nil(t, err)

			attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
			require.True(t, ok)

			require.NotEmpty(t, attachments[0].Actions)
			require.NotEmpty(t, attachments[0].Actions[0].Id)

			menuPost := model.Post{
				Message:       "Interactive post",
				ChannelId:     channel.Id,
				PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
				UserId:        th.BasicUser.Id,
				Props: model.StringInterface{
					"attachments": []*model.SlackAttachment{
						{
							Text: "hello",
							Actions: []*model.PostAction{
								{
									Integration: &model.PostActionIntegration{
										Context: model.StringInterface{
											"s": "foo",
											"n": 3,
										},
										URL: ts.URL,
									},
									Name:       "action",
									Type:       model.POST_ACTION_TYPE_SELECT,
									DataSource: "some_source",
								},
							},
						},
					},
				},
			}

			post2, err := th.App.CreatePostAsUser(th.Context, &menuPost, "", true)
			require.Nil(t, err)

			attachments2, ok := post2.GetProp("attachments").([]*model.SlackAttachment)
			require.True(t, ok)

			require.NotEmpty(t, attachments2[0].Actions)
			require.NotEmpty(t, attachments2[0].Actions[0].Id)

			clientTriggerId, err := th.App.DoPostAction(th.Context, post.Id, "notavalidid", th.BasicUser.Id, "")
			require.NotNil(t, err)
			assert.Equal(t, http.StatusNotFound, err.StatusCode)
			assert.True(t, clientTriggerId == "")

			clientTriggerId, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
			require.Nil(t, err)
			assert.True(t, len(clientTriggerId) == 26)

			clientTriggerId, err = th.App.DoPostAction(th.Context, post2.Id, attachments2[0].Actions[0].Id, th.BasicUser.Id, "selected")
			require.Nil(t, err)
			assert.True(t, len(clientTriggerId) == 26)

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			})

			_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
			require.NotNil(t, err)
			require.True(t, strings.Contains(err.Error(), "address forbidden"))

			interactivePostPlugin := model.Post{
				Message:       "Interactive post",
				ChannelId:     channel.Id,
				PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
				UserId:        th.BasicUser.Id,
				Props: model.StringInterface{
					"attachments": []*model.SlackAttachment{
						{
							Text: "hello",
							Actions: []*model.PostAction{
								{
									Integration: &model.PostActionIntegration{
										Context: model.StringInterface{
											"s": "foo",
											"n": 3,
										},
										URL: ts.URL + "/plugins/myplugin/myaction",
									},
									Name:       "action",
									Type:       "some_type",
									DataSource: "some_source",
								},
							},
						},
					},
				},
			}

			postplugin, err := th.App.CreatePostAsUser(th.Context, &interactivePostPlugin, "", true)
			require.Nil(t, err)

			attachmentsPlugin, ok := postplugin.GetProp("attachments").([]*model.SlackAttachment)
			require.True(t, ok)

			_, err = th.App.DoPostAction(th.Context, postplugin.Id, attachmentsPlugin[0].Actions[0].Id, th.BasicUser.Id, "")
			require.Nil(t, err)

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ServiceSettings.SiteURL = "http://127.1.1.1"
			})

			interactivePostSiteURL := model.Post{
				Message:       "Interactive post",
				ChannelId:     channel.Id,
				PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
				UserId:        th.BasicUser.Id,
				Props: model.StringInterface{
					"attachments": []*model.SlackAttachment{
						{
							Text: "hello",
							Actions: []*model.PostAction{
								{
									Integration: &model.PostActionIntegration{
										Context: model.StringInterface{
											"s": "foo",
											"n": 3,
										},
										URL: "http://127.1.1.1/plugins/myplugin/myaction",
									},
									Name:       "action",
									Type:       "some_type",
									DataSource: "some_source",
								},
							},
						},
					},
				},
			}

			postSiteURL, err := th.App.CreatePostAsUser(th.Context, &interactivePostSiteURL, "", true)
			require.Nil(t, err)

			attachmentsSiteURL, ok := postSiteURL.GetProp("attachments").([]*model.SlackAttachment)
			require.True(t, ok)

			_, err = th.App.DoPostAction(th.Context, postSiteURL.Id, attachmentsSiteURL[0].Actions[0].Id, th.BasicUser.Id, "")
			require.NotNil(t, err)
			require.False(t, strings.Contains(err.Error(), "address forbidden"))

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ServiceSettings.SiteURL = ts.URL + "/subpath"
			})

			interactivePostSubpath := model.Post{
				Message:       "Interactive post",
				ChannelId:     channel.Id,
				PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
				UserId:        th.BasicUser.Id,
				Props: model.StringInterface{
					"attachments": []*model.SlackAttachment{
						{
							Text: "hello",
							Actions: []*model.PostAction{
								{
									Integration: &model.PostActionIntegration{
										Context: model.StringInterface{
											"s": "foo",
											"n": 3,
										},
										URL: ts.URL + "/subpath/plugins/myplugin/myaction",
									},
									Name:       "action",
									Type:       "some_type",
									DataSource: "some_source",
								},
							},
						},
					},
				},
			}

			postSubpath, err := th.App.CreatePostAsUser(th.Context, &interactivePostSubpath, "", true)
			require.Nil(t, err)

			attachmentsSubpath, ok := postSubpath.GetProp("attachments").([]*model.SlackAttachment)
			require.True(t, ok)

			_, err = th.App.DoPostAction(th.Context, postSubpath.Id, attachmentsSubpath[0].Actions[0].Id, th.BasicUser.Id, "")
			require.Nil(t, err)

		})
	}
}

func TestPostActionProps(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := model.PostActionIntegrationRequestFromJson(r.Body)
		assert.NotNil(t, request)

		fmt.Fprintf(w, `{
			"update": {
				"message": "updated",
				"has_reactions": true,
				"is_pinned": false,
				"props": {
					"from_webhook":true,
					"override_username":"new_override_user",
					"override_icon_url":"new_override_icon",
					"A":"AA"
				}
			},
			"ephemeral_text": "foo"
		}`)
	}))
	defer ts.Close()

	interactivePost := model.Post{
		Message:       "Interactive post",
		ChannelId:     th.BasicChannel.Id,
		PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
		UserId:        th.BasicUser.Id,
		HasReactions:  false,
		IsPinned:      true,
		Props: model.StringInterface{
			"attachments": []*model.SlackAttachment{
				{
					Text: "hello",
					Actions: []*model.PostAction{
						{
							Integration: &model.PostActionIntegration{
								Context: model.StringInterface{
									"s": "foo",
									"n": 3,
								},
								URL: ts.URL,
							},
							Name:       "action",
							Type:       "some_type",
							DataSource: "some_source",
						},
					},
				},
			},
			"override_icon_url": "old_override_icon",
			"from_webhook":      false,
			"B":                 "BB",
		},
	}

	post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
	require.Nil(t, err)
	attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
	require.True(t, ok)

	clientTriggerId, err := th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
	require.Nil(t, err)
	assert.True(t, len(clientTriggerId) == 26)

	newPost, nErr := th.App.Srv().Store.Post().GetSingle(post.Id, false)
	require.NoError(t, nErr)

	assert.True(t, newPost.IsPinned)
	assert.False(t, newPost.HasReactions)
	assert.Nil(t, newPost.GetProp("B"))
	assert.Nil(t, newPost.GetProp("override_username"))
	assert.Equal(t, "AA", newPost.GetProp("A"))
	assert.Equal(t, "old_override_icon", newPost.GetProp("override_icon_url"))
	assert.Equal(t, false, newPost.GetProp("from_webhook"))
}

func TestSubmitInteractiveDialog(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	submit := model.SubmitDialogRequest{
		UserId:     th.BasicUser.Id,
		ChannelId:  th.BasicChannel.Id,
		TeamId:     th.BasicTeam.Id,
		CallbackId: "someid",
		State:      "somestate",
		Submission: map[string]interface{}{
			"name1": "value1",
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request model.SubmitDialogRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		require.NoError(t, err)
		assert.NotNil(t, request)

		assert.Equal(t, request.URL, "")
		assert.Equal(t, request.UserId, submit.UserId)
		assert.Equal(t, request.ChannelId, submit.ChannelId)
		assert.Equal(t, request.TeamId, submit.TeamId)
		assert.Equal(t, request.CallbackId, submit.CallbackId)
		assert.Equal(t, request.State, submit.State)
		val, ok := request.Submission["name1"].(string)
		require.True(t, ok)
		assert.Equal(t, "value1", val)

		resp := model.SubmitDialogResponse{
			Error:  "some generic error",
			Errors: map[string]string{"name1": "some error"},
		}

		b, _ := json.Marshal(resp)

		w.Write(b)
	}))
	defer ts.Close()

	setupPluginApiTest(t,
		`
		package main

		import (
			"net/http"
			"github.com/mattermost/mattermost-server/v5/plugin"
			"github.com/mattermost/mattermost-server/v5/model"
		)

		type MyPlugin struct {
			plugin.MattermostPlugin
		}

		func (p *MyPlugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
			errReply := "some error"
 			if r.URL.Query().Get("abc") == "xyz" {
				errReply = "some other error"
			}
			response := &model.SubmitDialogResponse{
				Errors: map[string]string{"name1": errReply},
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(response.ToJson())
		}

		func main() {
			plugin.ClientMain(&MyPlugin{})
		}
		`, `{"id": "myplugin", "backend": {"executable": "backend.exe"}}`, "myplugin", th.App, th.Context)

	hooks, err2 := th.App.GetPluginsEnvironment().HooksForPlugin("myplugin")
	require.NoError(t, err2)
	require.NotNil(t, hooks)

	submit.URL = ts.URL

	resp, err := th.App.SubmitInteractiveDialog(th.Context, submit)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "some generic error", resp.Error)
	assert.Equal(t, "some error", resp.Errors["name1"])

	submit.URL = ""
	resp, err = th.App.SubmitInteractiveDialog(th.Context, submit)
	assert.NotNil(t, err)
	assert.Nil(t, resp)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
		*cfg.ServiceSettings.SiteURL = ts.URL
	})

	submit.URL = "/notvalid/myplugin/myaction"
	resp, err = th.App.SubmitInteractiveDialog(th.Context, submit)
	assert.NotNil(t, err)
	require.Nil(t, resp)

	submit.URL = "/plugins/myplugin/myaction"
	resp, err = th.App.SubmitInteractiveDialog(th.Context, submit)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "some error", resp.Errors["name1"])

	submit.URL = "/plugins/myplugin/myaction?abc=xyz"
	resp, err = th.App.SubmitInteractiveDialog(th.Context, submit)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "some other error", resp.Errors["name1"])
}

func TestPostActionRelativeURL(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := model.PostActionIntegrationRequestFromJson(r.Body)
		assert.NotNil(t, request)
		fmt.Fprintf(w, `{"post": {"message": "updated"}, "ephemeral_text": "foo"}`)
	}))
	defer ts.Close()

	t.Run("invalid relative URL", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ts.URL
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "/notaplugin/some/path",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.NotNil(t, err)
	})

	t.Run("valid relative URL without SiteURL set", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ""
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "/plugins/myplugin/myaction",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.NotNil(t, err)
	})

	t.Run("valid relative URL with SiteURL set", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ts.URL
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "/plugins/myplugin/myaction",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.NotNil(t, err)

	})

	t.Run("valid (but dirty) relative URL with SiteURL set", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ts.URL
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "//plugins/myplugin///myaction",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.NotNil(t, err)
	})

	t.Run("valid relative URL with SiteURL set and no leading slash", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ts.URL
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "plugins/myplugin/myaction",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.NotNil(t, err)
	})
}

func TestPostActionRelativePluginURL(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	setupPluginApiTest(t,
		`
		package main

		import (
			"net/http"
			"github.com/mattermost/mattermost-server/v5/plugin"
			"github.com/mattermost/mattermost-server/v5/model"
		)

		type MyPlugin struct {
			plugin.MattermostPlugin
		}

		func (p *MyPlugin) 	ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
			response := &model.PostActionIntegrationResponse{}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(response.ToJson())
		}

		func main() {
			plugin.ClientMain(&MyPlugin{})
		}
		`, `{"id": "myplugin", "backend": {"executable": "backend.exe"}}`, "myplugin", th.App, th.Context)

	hooks, err2 := th.App.GetPluginsEnvironment().HooksForPlugin("myplugin")
	require.NoError(t, err2)
	require.NotNil(t, hooks)

	t.Run("invalid relative URL", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ""
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "/notaplugin/some/path",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.NotNil(t, err)
	})

	t.Run("valid relative URL", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ""
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "/plugins/myplugin/myaction",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.Nil(t, err)
	})

	t.Run("valid (but dirty) relative URL", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ""
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "//plugins/myplugin///myaction",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.Nil(t, err)
	})

	t.Run("valid relative URL and no leading slash", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.AllowedUntrustedInternalConnections = ""
			*cfg.ServiceSettings.SiteURL = ""
		})

		interactivePost := model.Post{
			Message:       "Interactive post",
			ChannelId:     th.BasicChannel.Id,
			PendingPostId: model.NewId() + ":" + fmt.Sprint(model.GetMillis()),
			UserId:        th.BasicUser.Id,
			Props: model.StringInterface{
				"attachments": []*model.SlackAttachment{
					{
						Text: "hello",
						Actions: []*model.PostAction{
							{
								Integration: &model.PostActionIntegration{
									URL: "plugins/myplugin/myaction",
								},
								Name: "action",
								Type: "some_type",
							},
						},
					},
				},
			},
		}

		post, err := th.App.CreatePostAsUser(th.Context, &interactivePost, "", true)
		require.Nil(t, err)
		attachments, ok := post.GetProp("attachments").([]*model.SlackAttachment)
		require.True(t, ok)
		require.NotEmpty(t, attachments[0].Actions)
		require.NotEmpty(t, attachments[0].Actions[0].Id)

		_, err = th.App.DoPostAction(th.Context, post.Id, attachments[0].Actions[0].Id, th.BasicUser.Id, "")
		require.Nil(t, err)
	})
}

func TestDoPluginRequest(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	setupPluginApiTest(t,
		`
		package main

		import (
			"net/http"
			"reflect"
			"sort"

			"github.com/mattermost/mattermost-server/v5/plugin"
		)

		type MyPlugin struct {
			plugin.MattermostPlugin
		}

		func (p *MyPlugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("abc") != "xyz" {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("could not find param abc=xyz"))
				return
			}

			multiple := q["multiple"]
			if len(multiple) != 3 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("param multiple should have 3 values"))
				return
			}
			sort.Strings(multiple)
			if !reflect.DeepEqual(multiple, []string{"1 first", "2 second", "3 third"}) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("param multiple not correct"))
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}

		func main() {
			plugin.ClientMain(&MyPlugin{})
		}
		`, `{"id": "myplugin", "backend": {"executable": "backend.exe"}}`, "myplugin", th.App, th.Context)

	hooks, err2 := th.App.GetPluginsEnvironment().HooksForPlugin("myplugin")
	require.NoError(t, err2)
	require.NotNil(t, hooks)

	resp, err := th.App.doPluginRequest(th.Context, "GET", "/plugins/myplugin", nil, nil)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "could not find param abc=xyz", string(body))

	resp, err = th.App.doPluginRequest(th.Context, "GET", "/plugins/myplugin?abc=xyz", nil, nil)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(t, "param multiple should have 3 values", string(body))

	resp, err = th.App.doPluginRequest(th.Context, "GET", "/plugins/myplugin",
		url.Values{"abc": []string{"xyz"}, "multiple": []string{"1 first", "2 second", "3 third"}}, nil)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(t, "OK", string(body))

	resp, err = th.App.doPluginRequest(th.Context, "GET", "/plugins/myplugin?abc=xyz&multiple=1%20first",
		url.Values{"multiple": []string{"2 second", "3 third"}}, nil)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(t, "OK", string(body))

	resp, err = th.App.doPluginRequest(th.Context, "GET", "/plugins/myplugin?abc=xyz&multiple=1%20first&multiple=3%20third",
		url.Values{"multiple": []string{"2 second"}}, nil)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(t, "OK", string(body))

	resp, err = th.App.doPluginRequest(th.Context, "GET", "/plugins/myplugin?multiple=1%20first&multiple=3%20third",
		url.Values{"multiple": []string{"2 second"}, "abc": []string{"xyz"}}, nil)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(t, "OK", string(body))

	resp, err = th.App.doPluginRequest(th.Context, "GET", "/plugins/myplugin?multiple=1%20first&multiple=3%20third",
		url.Values{"multiple": []string{"4 fourth"}, "abc": []string{"xyz"}}, nil)
	assert.Nil(t, err)
	require.NotNil(t, resp)
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(t, "param multiple not correct", string(body))
}
