// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestLicenseStore(t *testing.T, ss store.Store) {
	t.Run("Save", func(t *testing.T) { testLicenseStoreSave(t, ss) })
	t.Run("Get", func(t *testing.T) { testLicenseStoreGet(t, ss) })
}

func testLicenseStoreSave(t *testing.T, ss store.Store) {
	l1 := model.LicenseRecord{}
	l1.ID = model.NewID()
	l1.Bytes = "junk"

	_, err := ss.License().Save(&l1)
	require.NoError(t, err, "couldn't save license record")

	_, err = ss.License().Save(&l1)
	require.NoError(t, err, "shouldn't fail on trying to save existing license record")

	l1.ID = ""

	_, err = ss.License().Save(&l1)
	require.Error(t, err, "should fail on invalid license")
}

func testLicenseStoreGet(t *testing.T, ss store.Store) {
	l1 := model.LicenseRecord{}
	l1.ID = model.NewID()
	l1.Bytes = "junk"

	_, err := ss.License().Save(&l1)
	require.NoError(t, err)

	record, err := ss.License().Get(l1.ID)
	require.NoError(t, err, "couldn't get license")

	require.Equal(t, record.Bytes, l1.Bytes, "license bytes didn't match")

	_, err = ss.License().Get("missing")
	require.Error(t, err, "should fail on get license")
}
