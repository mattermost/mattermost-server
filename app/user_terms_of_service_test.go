// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserTermsOfService(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	userTermsOfService, err := th.App.GetUserTermsOfService(th.BasicUser.ID)
	checkError(t, err)
	assert.Nil(t, userTermsOfService)
	assert.Equal(t, "app.user_terms_of_service.get_by_user.no_rows.app_error", err.ID)

	termsOfService, err := th.App.CreateTermsOfService("terms of service", th.BasicUser.ID)
	checkNoError(t, err)

	err = th.App.SaveUserTermsOfService(th.BasicUser.ID, termsOfService.ID, true)
	checkNoError(t, err)

	userTermsOfService, err = th.App.GetUserTermsOfService(th.BasicUser.ID)
	checkNoError(t, err)
	assert.NotNil(t, userTermsOfService)
	assert.NotEmpty(t, userTermsOfService)

	assert.Equal(t, th.BasicUser.ID, userTermsOfService.UserID)
	assert.Equal(t, termsOfService.ID, userTermsOfService.TermsOfServiceID)
	assert.NotEmpty(t, userTermsOfService.CreateAt)
}
