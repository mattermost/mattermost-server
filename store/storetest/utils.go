// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"github.com/mattermost/mattermost-server/v5/model"
)

// This function has a copy of it in app/helper_test
// NewTestId is used for testing as a replacement for model.NewId(). It is a [A-Z0-9] string 26
// characters long. It replaces every odd character with a digit.
func NewTestID() string {
	newID := []byte(model.NewID())

	for i := 1; i < len(newID); i = i + 2 {
		newID[i] = 48 + newID[i-1]%10
	}

	return string(newID)
}
