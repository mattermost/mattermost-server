// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTestID(t *testing.T) {
	rg := regexp.MustCompile(`(\S\d){13}`)

	for i := 0; i < 1000; i++ {
		id := NewTestID()
		require.LessOrEqual(t, len(id), 26, "test ids shouldn't be longer than 26 chars")
		require.Regexp(t, rg, id, "test ids should have pattern e.g a1b2c3...")
	}
}
