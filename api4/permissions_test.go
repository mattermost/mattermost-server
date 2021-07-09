// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetAncillaryPermissions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	var subsectionPermissions []string
	var expectedAncillaryPermissions []string
	t.Run("Valid Case, Passing in SubSection Permissions", func(t *testing.T) {
		subsectionPermissions = []string{model.PermissionSysconsoleReadReportingSiteStatistics.ID}
		expectedAncillaryPermissions = []string{model.PermissionGetAnalytics.ID}
		actualAncillaryPermissions, resp := th.Client.GetAncillaryPermissions(subsectionPermissions)
		CheckNoError(t, resp)
		assert.Equal(t, append(subsectionPermissions, expectedAncillaryPermissions...), actualAncillaryPermissions)
	})

	t.Run("Invalid Case, Passing in SubSection Permissions That Don't Exist", func(t *testing.T) {
		subsectionPermissions = []string{"All", "The", "Things", "She", "Said", "Running", "Through", "My", "Head"}
		expectedAncillaryPermissions = []string{}
		actualAncillaryPermissions, resp := th.Client.GetAncillaryPermissions(subsectionPermissions)
		CheckNoError(t, resp)
		assert.Equal(t, append(subsectionPermissions, expectedAncillaryPermissions...), actualAncillaryPermissions)
	})

	t.Run("Invalid Case, Passing in nothing", func(t *testing.T) {
		subsectionPermissions = []string{}
		expectedAncillaryPermissions = []string{}
		_, resp := th.Client.GetAncillaryPermissions(subsectionPermissions)
		CheckBadRequestStatus(t, resp)
	})
}
