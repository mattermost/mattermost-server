// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"github.com/mattermost/mattermost-server/v5/model"
)

func MakeEmail() string {
	return "success_" + model.NewID() + "@simulator.amazonses.com"
}
