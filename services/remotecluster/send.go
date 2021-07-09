// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package remotecluster

import (
	"context"
	"hash/fnv"
)

// enqueueTask adds a task to one of the send channels based on remoteId.
//
// There are a number  of send channels (`MaxConcurrentSends`) to allow for sending to multiple
// remotes concurrently, while preserving message order for each remote.
func (rcs *Service) enqueueTask(ctx context.Context, remoteID string, task interface{}) error {
	if ctx == nil {
		ctx = context.Background()
	}

	h := hash(remoteID)
	idx := h % uint32(len(rcs.send))

	select {
	case rcs.send[idx] <- task:
		return nil
	case <-ctx.Done():
		return NewBufferFullError(cap(rcs.send))
	}
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// sendLoop is called by each goroutine created for the send pool and waits for sendTask's until the
// done channel is signalled.
//
// Each goroutine in the pool is assigned a specific channel, and tasks are placed in the
// channel corresponding to the remoteId.
func (rcs *Service) sendLoop(idx int, done chan struct{}) {
	for {
		select {
		case t := <-rcs.send[idx]:
			switch task := t.(type) {
			case sendMsgTask:
				rcs.sendMsg(task)
			case sendFileTask:
				rcs.sendFile(task)
			case sendProfileImageTask:
				rcs.sendProfileImage(task)
			}
		case <-done:
			return
		}
	}
}
