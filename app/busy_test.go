// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
)

func TestBusySet(t *testing.T) {
	cluster := &ClusterMock{Busy: &Busy{}}
	busy := NewBusy(cluster)

	isNotBusy := func() bool {
		return !busy.IsBusy()
	}

	require.False(t, busy.IsBusy())

	busy.Set(time.Millisecond * 100)
	require.True(t, busy.IsBusy())
	require.True(t, compareBusyState(t, busy, cluster.Busy))
	// should automatically expire after 100ms.
	require.Eventually(t, isNotBusy, time.Second*15, time.Millisecond*20)
	// allow a moment for cluster to sync.
	require.Eventually(t, func() bool { return compareBusyState(t, busy, cluster.Busy) }, time.Second*15, time.Millisecond*20)

	// test set after auto expiry.
	busy.Set(time.Second * 30)
	require.True(t, busy.IsBusy())
	require.True(t, compareBusyState(t, busy, cluster.Busy))
	expire := busy.Expires()
	require.Greater(t, expire.Unix(), time.Now().Add(time.Second*10).Unix())

	// test extending existing expiry
	busy.Set(time.Minute * 5)
	require.True(t, busy.IsBusy())
	require.True(t, compareBusyState(t, busy, cluster.Busy))
	expire = busy.Expires()
	require.Greater(t, expire.Unix(), time.Now().Add(time.Minute*2).Unix())

	busy.Clear()
	require.False(t, busy.IsBusy())
	require.True(t, compareBusyState(t, busy, cluster.Busy))
}

func TestBusyExpires(t *testing.T) {
	cluster := &ClusterMock{Busy: &Busy{}}
	busy := NewBusy(cluster)

	isNotBusy := func() bool {
		return !busy.IsBusy()
	}

	// get expiry before it is set
	expire := busy.Expires()
	// should be time.Time zero value
	require.Equal(t, time.Time{}.Unix(), expire.Unix())

	// get expiry after it is set
	busy.Set(time.Minute * 5)
	expire = busy.Expires()
	require.Greater(t, expire.Unix(), time.Now().Add(time.Minute*2).Unix())
	require.True(t, compareBusyState(t, busy, cluster.Busy))

	// get expiry after clear
	busy.Clear()
	expire = busy.Expires()
	// should be time.Time zero value
	require.Equal(t, time.Time{}.Unix(), expire.Unix())
	require.True(t, compareBusyState(t, busy, cluster.Busy))

	// get expiry after auto-expire
	busy.Set(time.Millisecond * 100)
	require.Eventually(t, isNotBusy, time.Second*5, time.Millisecond*20)
	expire = busy.Expires()
	// should be time.Time zero value
	require.Equal(t, time.Time{}.Unix(), expire.Unix())
	// allow a moment for cluster to sync
	require.Eventually(t, func() bool { return compareBusyState(t, busy, cluster.Busy) }, time.Second*15, time.Millisecond*20)
}

func TestBusyRace(t *testing.T) {
	cluster := &ClusterMock{Busy: &Busy{}}
	busy := NewBusy(cluster)

	busy.Set(500 * time.Millisecond)

	// We are sleeping in order to let the race trigger.
	time.Sleep(time.Second)
}

func compareBusyState(t *testing.T, busy1 *Busy, busy2 *Busy) bool {
	t.Helper()
	if busy1.IsBusy() != busy2.IsBusy() {
		t.Logf("busy1:%s;  busy2:%s\n", busy1.ToJSON(), busy2.ToJSON())
		return false
	}
	if busy1.Expires().Unix() != busy2.Expires().Unix() {
		t.Logf("busy1:%s;  busy2:%s\n", busy1.ToJSON(), busy2.ToJSON())
		return false
	}
	return true
}

// ClusterMock simulates the busy state of a cluster.
type ClusterMock struct {
	Busy *Busy
}

func (c *ClusterMock) SendClusterMessage(msg *model.ClusterMessage) {
	sbs := model.ServerBusyStateFromJSON(strings.NewReader(msg.Data))
	c.Busy.ClusterEventChanged(sbs)
}

func (c *ClusterMock) SendClusterMessageToNode(nodeID string, msg *model.ClusterMessage) error {
	return nil
}

func (c *ClusterMock) StartInterNodeCommunication() {}
func (c *ClusterMock) StopInterNodeCommunication()  {}
func (c *ClusterMock) RegisterClusterMessageHandler(event string, crm einterfaces.ClusterMessageHandler) {
}
func (c *ClusterMock) GetClusterID() string                                       { return "cluster_mock" }
func (c *ClusterMock) IsLeader() bool                                             { return false }
func (c *ClusterMock) GetMyClusterInfo() *model.ClusterInfo                       { return nil }
func (c *ClusterMock) GetClusterInfos() []*model.ClusterInfo                      { return nil }
func (c *ClusterMock) NotifyMsg(buf []byte)                                       {}
func (c *ClusterMock) GetClusterStats() ([]*model.ClusterStats, *model.AppError)  { return nil, nil }
func (c *ClusterMock) GetLogs(page, perPage int) ([]string, *model.AppError)      { return nil, nil }
func (c *ClusterMock) GetPluginStatuses() (model.PluginStatuses, *model.AppError) { return nil, nil }
func (c *ClusterMock) ConfigChanged(previousConfig *model.Config, newConfig *model.Config, sendToOtherServer bool) *model.AppError {
	return nil
}
func (c *ClusterMock) HealthScore() int { return 0 }
