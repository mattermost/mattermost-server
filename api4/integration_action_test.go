// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

type testHandler struct {
	t *testing.T
}

func (th *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bb, err := ioutil.ReadAll(r.Body)
	assert.NoError(th.t, err)
	assert.NotEmpty(th.t, string(bb))
	poir := model.PostActionIntegrationRequestFromJSON(bytes.NewReader(bb))
	assert.NotEmpty(th.t, poir.UserID)
	assert.NotEmpty(th.t, poir.UserName)
	assert.NotEmpty(th.t, poir.ChannelID)
	assert.NotEmpty(th.t, poir.ChannelName)
	assert.NotEmpty(th.t, poir.TeamID)
	assert.NotEmpty(th.t, poir.TeamName)
	assert.NotEmpty(th.t, poir.PostID)
	assert.NotEmpty(th.t, poir.TriggerID)
	assert.Equal(th.t, "button", poir.Type)
	assert.Equal(th.t, "test-value", poir.Context["test-key"])
	w.Write([]byte("{}"))
	w.WriteHeader(200)
}

func TestPostActionCookies(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	handler := &testHandler{t}
	server := httptest.NewServer(handler)

	for name, test := range map[string]struct {
		Action             model.PostAction
		ExpectedSucess     bool
		ExpectedStatusCode int
	}{
		"32 character ID": {
			Action: model.PostAction{
				ID:   model.NewID(),
				Name: "Test-action",
				Type: model.PostActionTypeButton,
				Integration: &model.PostActionIntegration{
					URL: server.URL,
					Context: map[string]interface{}{
						"test-key": "test-value",
					},
				},
			},
			ExpectedSucess:     true,
			ExpectedStatusCode: http.StatusOK,
		},
		"6 character ID": {
			Action: model.PostAction{
				ID:   "someID",
				Name: "Test-action",
				Type: model.PostActionTypeButton,
				Integration: &model.PostActionIntegration{
					URL: server.URL,
					Context: map[string]interface{}{
						"test-key": "test-value",
					},
				},
			},
			ExpectedSucess:     true,
			ExpectedStatusCode: http.StatusOK,
		},
		"Empty ID": {
			Action: model.PostAction{
				ID:   "",
				Name: "Test-action",
				Type: model.PostActionTypeButton,
				Integration: &model.PostActionIntegration{
					URL: server.URL,
					Context: map[string]interface{}{
						"test-key": "test-value",
					},
				},
			},
			ExpectedSucess:     false,
			ExpectedStatusCode: http.StatusNotFound,
		},
	} {
		t.Run(name, func(t *testing.T) {
			post := &model.Post{
				ID:        model.NewID(),
				Type:      model.PostTypeEphemeral,
				UserID:    th.BasicUser.ID,
				ChannelID: th.BasicChannel.ID,
				CreateAt:  model.GetMillis(),
				UpdateAt:  model.GetMillis(),
				Props: map[string]interface{}{
					"attachments": []*model.SlackAttachment{
						{
							Title:     "some-title",
							TitleLink: "https://some-url.com",
							Text:      "some-text",
							ImageURL:  "https://some-other-url.com",
							Actions:   []*model.PostAction{&test.Action},
						},
					},
				},
			}

			assert.Equal(t, 32, len(th.App.PostActionCookieSecret()))
			post = model.AddPostActionCookies(post, th.App.PostActionCookieSecret())

			ok, resp := Client.DoPostActionWithCookie(post.ID, test.Action.ID, "", test.Action.Cookie)
			require.NotNil(t, resp)
			if test.ExpectedSucess {
				assert.True(t, ok)
				assert.Nil(t, resp.Error)
			} else {
				assert.False(t, ok)
				assert.NotNil(t, resp.Error)
			}
			assert.Equal(t, test.ExpectedStatusCode, resp.StatusCode)
			assert.NotNil(t, resp.RequestID)
			assert.NotNil(t, resp.ServerVersion)
		})
	}
}

func TestOpenDialog(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	_, triggerID, err := model.GenerateTriggerID(th.BasicUser.ID, th.App.AsymmetricSigningKey())
	require.Nil(t, err)

	request := model.OpenDialogRequest{
		TriggerID: triggerID,
		URL:       "http://localhost:8065",
		Dialog: model.Dialog{
			CallbackID: "callbackid",
			Title:      "Some Title",
			Elements: []model.DialogElement{
				{
					DisplayName: "Element Name",
					Name:        "element_name",
					Type:        "text",
					Placeholder: "Enter a value",
				},
			},
			SubmitLabel:    "Submit",
			NotifyOnCancel: false,
			State:          "somestate",
		},
	}

	pass, resp := Client.OpenInteractiveDialog(request)
	CheckNoError(t, resp)
	assert.True(t, pass)

	// Should fail on bad trigger ID
	request.TriggerID = "junk"
	pass, resp = Client.OpenInteractiveDialog(request)
	CheckBadRequestStatus(t, resp)
	assert.False(t, pass)

	// URL is required
	request.TriggerID = triggerID
	request.URL = ""
	pass, resp = Client.OpenInteractiveDialog(request)
	CheckBadRequestStatus(t, resp)
	assert.False(t, pass)

	// Should pass with markdown formatted introduction text
	request.URL = "http://localhost:8065"
	request.Dialog.IntroductionText = "**Some** _introduction text"
	pass, resp = Client.OpenInteractiveDialog(request)
	CheckNoError(t, resp)
	assert.True(t, pass)

	// Should pass with empty introduction text
	request.Dialog.IntroductionText = ""
	pass, resp = Client.OpenInteractiveDialog(request)
	CheckNoError(t, resp)
	assert.True(t, pass)

	// Should pass with no elements
	request.Dialog.Elements = nil
	pass, resp = Client.OpenInteractiveDialog(request)
	CheckNoError(t, resp)
	assert.True(t, pass)
	request.Dialog.Elements = []model.DialogElement{}
	pass, resp = Client.OpenInteractiveDialog(request)
	CheckNoError(t, resp)
	assert.True(t, pass)
}

func TestSubmitDialog(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "localhost,127.0.0.1"
	})

	submit := model.SubmitDialogRequest{
		CallbackID: "callbackid",
		State:      "somestate",
		UserID:     th.BasicUser.ID,
		ChannelID:  th.BasicChannel.ID,
		TeamID:     th.BasicTeam.ID,
		Submission: map[string]interface{}{"somename": "somevalue"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request model.SubmitDialogRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		require.NoError(t, err)

		assert.Equal(t, request.URL, "")
		assert.Equal(t, request.UserID, submit.UserID)
		assert.Equal(t, request.ChannelID, submit.ChannelID)
		assert.Equal(t, request.TeamID, submit.TeamID)
		assert.Equal(t, request.CallbackID, submit.CallbackID)
		assert.Equal(t, request.State, submit.State)
		val, ok := request.Submission["somename"].(string)
		require.True(t, ok)
		assert.Equal(t, "somevalue", val)
	}))
	defer ts.Close()

	submit.URL = ts.URL

	submitResp, resp := Client.SubmitInteractiveDialog(submit)
	CheckNoError(t, resp)
	assert.NotNil(t, submitResp)

	submit.URL = ""
	submitResp, resp = Client.SubmitInteractiveDialog(submit)
	CheckBadRequestStatus(t, resp)
	assert.Nil(t, submitResp)

	submit.URL = ts.URL
	submit.ChannelID = model.NewID()
	submitResp, resp = Client.SubmitInteractiveDialog(submit)
	CheckForbiddenStatus(t, resp)
	assert.Nil(t, submitResp)

	submit.URL = ts.URL
	submit.ChannelID = th.BasicChannel.ID
	submit.TeamID = model.NewID()
	submitResp, resp = Client.SubmitInteractiveDialog(submit)
	CheckForbiddenStatus(t, resp)
	assert.Nil(t, submitResp)
}
