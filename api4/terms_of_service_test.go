// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetTermsOfService(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	_, err := th.App.CreateTermsOfService("abc", th.BasicUser.ID)
	require.Nil(t, err)

	termsOfService, resp := Client.GetTermsOfService("")
	CheckNoError(t, resp)

	assert.NotNil(t, termsOfService)
	assert.Equal(t, "abc", termsOfService.Text)
	assert.NotEmpty(t, termsOfService.ID)
	assert.NotEmpty(t, termsOfService.CreateAt)
}

func TestCreateTermsOfService(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	_, resp := Client.CreateTermsOfService("terms of service new", th.BasicUser.ID)
	CheckErrorMessage(t, resp, "api.context.permissions.app_error")
}

func TestCreateTermsOfServiceAdminUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.SystemAdminClient

	termsOfService, resp := Client.CreateTermsOfService("terms of service new", th.SystemAdminUser.ID)
	CheckErrorMessage(t, resp, "api.create_terms_of_service.custom_terms_of_service_disabled.app_error")
	assert.Nil(t, termsOfService)

	th.App.Srv().SetLicense(model.NewTestLicense("EnableCustomTermsOfService"))

	termsOfService, resp = Client.CreateTermsOfService("terms of service new_2", th.SystemAdminUser.ID)
	CheckNoError(t, resp)
	assert.NotEmpty(t, termsOfService.ID)
	assert.NotEmpty(t, termsOfService.CreateAt)
	assert.Equal(t, "terms of service new_2", termsOfService.Text)
	assert.Equal(t, th.SystemAdminUser.ID, termsOfService.UserID)
}
