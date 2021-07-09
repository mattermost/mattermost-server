// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/mattermost/mattermost-server/v5/store/storetest"
)

func TestThreadStore(t *testing.T) {
	StoreTestWithSQLStore(t, storetest.TestThreadStore)
}
