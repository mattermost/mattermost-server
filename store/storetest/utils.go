// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"math/rand"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// NewTestId is used for testing as a replacement for model.NewId(). It is a [A-Z0-9] string 26
// characters long. It replaces every odd character with a digit.
func NewTestId() string {
	newId := []byte(model.NewId())

	for i := 1; i < len(newId); i = i + 2 {
		newId[i] = 48 + newId[i-1]%10
	}

	return string(newId)
}
