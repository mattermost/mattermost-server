// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/einterfaces/mocks"
	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetSamlMetadata(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()
	Client := th.Client

	_, resp := Client.GetSamlMetadata()
	CheckNotImplementedStatus(t, resp)

	// Rest is tested by enterprise tests
}

func TestSamlCompleteCSRFPass(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	url := th.Client.Url + "/login/sso/saml"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return
	}

	cookie1 := &http.Cookie{
		Name:  model.SessionCookieUser,
		Value: th.BasicUser.Username,
	}
	cookie2 := &http.Cookie{
		Name:  model.SessionCookieToken,
		Value: th.Client.AuthToken,
	}
	req.AddCookie(cookie1)
	req.AddCookie(cookie2)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	defer resp.Body.Close()
}

func TestSamlResetID(t *testing.T) {
	th := SetupEnterprise(t).InitBasic()
	defer th.TearDown()
	th.App.Srv().Saml = &mocks.SamlInterface{}

	user := th.BasicUser
	_, appErr := th.App.UpdateUserAuth(user.ID, &model.UserAuth{
		AuthData:    model.NewString(model.NewID()),
		AuthService: model.UserAuthServiceSaml,
	})
	require.Nil(t, appErr)

	_, resp := th.Client.ResetSamlAuthDataToEmail(false, false, nil)
	CheckForbiddenStatus(t, resp)

	numAffected, resp := th.SystemAdminClient.ResetSamlAuthDataToEmail(false, false, nil)
	CheckOKStatus(t, resp)
	require.Equal(t, int64(1), numAffected)
}
