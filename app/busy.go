// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	TimestampFormat = "Mon Jan 2 15:04:05 -0700 MST 2006"
)

// Busy represents the busy state of the server. A server marked busy
// will have non-critical services disabled. If a Cluster is provided
// any changes will be propagated to each node.
type Busy struct {
	busy    int32 // protected via atomic for fast IsBusy calls
	mux     sync.RWMutex
	timer   *time.Timer
	expires time.Time

	cluster einterfaces.ClusterInterface
}

// NewBusy creates a new Busy instance with optional cluster which will
// be notified of busy state changes.
func NewBusy(cluster einterfaces.ClusterInterface) *Busy {
	return &Busy{cluster: cluster}
}

// IsBusy returns true if the server has been marked as busy.
func (b *Busy) IsBusy() bool {
	if b == nil {
		return false
	}
	return atomic.LoadInt32(&b.busy) != 0
}

// Set marks the server as busy for dur duration and notifies cluster nodes.
func (b *Busy) Set(dur time.Duration) {
	b.mux.Lock()
	defer b.mux.Unlock()

	// minimum 1 second
	if dur < (time.Second * 1) {
		dur = time.Second * 1
	}

	b.setWithoutNotify(dur)

	if b.cluster != nil {
		sbs := &model.ServerBusyState{Busy: true, Expires: b.expires.Unix(), ExpiresTS: b.expires.UTC().Format(TimestampFormat)}
		b.notifyServerBusyChange(sbs)
	}
}

// must hold mutex
func (b *Busy) setWithoutNotify(dur time.Duration) {
	b.clearWithoutNotify()
	atomic.StoreInt32(&b.busy, 1)
	b.expires = time.Now().Add(dur)
	b.timer = time.AfterFunc(dur, func() {
		b.mux.Lock()
		b.clearWithoutNotify()
		b.mux.Unlock()
	})
}

// ClearBusy marks the server as not busy and notifies cluster nodes.
func (b *Busy) Clear() {
	b.mux.Lock()
	defer b.mux.Unlock()

	b.clearWithoutNotify()

	if b.cluster != nil {
		sbs := &model.ServerBusyState{Busy: false, Expires: time.Time{}.Unix(), ExpiresTS: ""}
		b.notifyServerBusyChange(sbs)
	}
}

// must hold mutex
func (b *Busy) clearWithoutNotify() {
	if b.timer != nil {
		b.timer.Stop() // don't drain timer.C channel for AfterFunc timers.
	}
	b.timer = nil
	b.expires = time.Time{}
	atomic.StoreInt32(&b.busy, 0)
}

// Expires returns the expected time that the server
// will be marked not busy. This expiry can be extended
// via additional calls to SetBusy.
func (b *Busy) Expires() time.Time {
	b.mux.RLock()
	defer b.mux.RUnlock()
	return b.expires
}

// notifyServerBusyChange informs all cluster members of a server busy state change.
func (b *Busy) notifyServerBusyChange(sbs *model.ServerBusyState) {
	if b.cluster == nil {
		return
	}
	msg := &model.ClusterMessage{
		Event:            model.ClusterEventBusyStateChanged,
		SendType:         model.ClusterSendReliable,
		WaitForAllToSend: true,
		Data:             sbs.ToJSON(),
	}
	b.cluster.SendClusterMessage(msg)
}

// ClusterEventChanged is called when a CLUSTER_EVENT_BUSY_STATE_CHANGED is received.
func (b *Busy) ClusterEventChanged(sbs *model.ServerBusyState) {
	b.mux.Lock()
	defer b.mux.Unlock()

	if sbs.Busy {
		expires := time.Unix(sbs.Expires, 0)
		dur := time.Until(expires)
		if dur > 0 {
			b.setWithoutNotify(dur)
		}
	} else {
		b.clearWithoutNotify()
	}
}

func (b *Busy) ToJSON() string {
	b.mux.RLock()
	defer b.mux.RUnlock()

	sbs := &model.ServerBusyState{
		Busy:      atomic.LoadInt32(&b.busy) != 0,
		Expires:   b.expires.Unix(),
		ExpiresTS: b.expires.UTC().Format(TimestampFormat),
	}
	return sbs.ToJSON()
}
