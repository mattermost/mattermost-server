// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func TestClusterDiscoveryStore(t *testing.T, ss store.Store) {
	t.Run("", func(t *testing.T) { testClusterDiscoveryStore(t, ss) })
	t.Run("Delete", func(t *testing.T) { testClusterDiscoveryStoreDelete(t, ss) })
	t.Run("LastPing", func(t *testing.T) { testClusterDiscoveryStoreLastPing(t, ss) })
	t.Run("Exists", func(t *testing.T) { testClusterDiscoveryStoreExists(t, ss) })
	t.Run("ClusterDiscoveryGetStore", func(t *testing.T) { testClusterDiscoveryGetStore(t, ss) })
}

func testClusterDiscoveryStore(t *testing.T, ss store.Store) {
	discovery := &model.ClusterDiscovery{
		ClusterName: "cluster_name",
		Hostname:    "hostname" + model.NewID(),
		Type:        "test_test",
	}

	err := ss.ClusterDiscovery().Save(discovery)
	require.NoError(t, err)

	err = ss.ClusterDiscovery().Cleanup()
	require.NoError(t, err)
}

func testClusterDiscoveryStoreDelete(t *testing.T, ss store.Store) {
	discovery := &model.ClusterDiscovery{
		ClusterName: "cluster_name",
		Hostname:    "hostname" + model.NewID(),
		Type:        "test_test",
	}

	err := ss.ClusterDiscovery().Save(discovery)
	require.NoError(t, err)

	_, err = ss.ClusterDiscovery().Delete(discovery)
	require.NoError(t, err)
}

func testClusterDiscoveryStoreLastPing(t *testing.T, ss store.Store) {
	discovery := &model.ClusterDiscovery{
		ClusterName: "cluster_name_lastPing",
		Hostname:    "hostname" + model.NewID(),
		Type:        "test_test_lastPing" + model.NewID(),
	}

	err := ss.ClusterDiscovery().Save(discovery)
	require.NoError(t, err)

	err = ss.ClusterDiscovery().SetLastPingAt(discovery)
	require.NoError(t, err)

	ttime := model.GetMillis()

	time.Sleep(1 * time.Second)

	err = ss.ClusterDiscovery().SetLastPingAt(discovery)
	require.NoError(t, err)

	list, err := ss.ClusterDiscovery().GetAll(discovery.Type, "cluster_name_lastPing")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.Less(t, int64(500), list[0].LastPingAt-ttime)

	discovery2 := &model.ClusterDiscovery{
		ClusterName: "cluster_name_missing",
		Hostname:    "hostname" + model.NewID(),
		Type:        "test_test_missing",
	}

	err = ss.ClusterDiscovery().SetLastPingAt(discovery2)
	require.NoError(t, err)
}

func testClusterDiscoveryStoreExists(t *testing.T, ss store.Store) {
	discovery := &model.ClusterDiscovery{
		ClusterName: "cluster_name_Exists",
		Hostname:    "hostname" + model.NewID(),
		Type:        "test_test_Exists" + model.NewID(),
	}

	err := ss.ClusterDiscovery().Save(discovery)
	require.NoError(t, err)

	val, err := ss.ClusterDiscovery().Exists(discovery)
	require.NoError(t, err)
	assert.True(t, val)

	discovery.ClusterName = "cluster_name_Exists2"

	val, err = ss.ClusterDiscovery().Exists(discovery)
	require.NoError(t, err)
	assert.False(t, val)
}

func testClusterDiscoveryGetStore(t *testing.T, ss store.Store) {
	testType1 := model.NewID()

	discovery1 := &model.ClusterDiscovery{
		ClusterName: "cluster_name",
		Hostname:    "hostname1",
		Type:        testType1,
	}
	require.NoError(t, ss.ClusterDiscovery().Save(discovery1))

	discovery2 := &model.ClusterDiscovery{
		ClusterName: "cluster_name",
		Hostname:    "hostname2",
		Type:        testType1,
	}
	require.NoError(t, ss.ClusterDiscovery().Save(discovery2))

	discovery3 := &model.ClusterDiscovery{
		ClusterName: "cluster_name",
		Hostname:    "hostname3",
		Type:        testType1,
		CreateAt:    1,
		LastPingAt:  1,
	}
	require.NoError(t, ss.ClusterDiscovery().Save(discovery3))

	testType2 := model.NewID()

	discovery4 := &model.ClusterDiscovery{
		ClusterName: "cluster_name",
		Hostname:    "hostname1",
		Type:        testType2,
	}
	require.NoError(t, ss.ClusterDiscovery().Save(discovery4))

	list, err := ss.ClusterDiscovery().GetAll(testType1, "cluster_name")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	list, err = ss.ClusterDiscovery().GetAll(testType2, "cluster_name")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	list, err = ss.ClusterDiscovery().GetAll(model.NewID(), "cluster_name")
	require.NoError(t, err)
	assert.Empty(t, list)
}
