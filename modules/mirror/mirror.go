// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

var mirrorQueue *queue.WorkerPoolQueue[*SyncRequest]

// SyncType type of sync request
type SyncType int

const (
	// PullMirrorType for pull mirrors
	PullMirrorType SyncType = iota
	// PushMirrorType for push mirrors
	PushMirrorType
)

// SyncRequest for the mirror queue
type SyncRequest struct {
	Type        SyncType
	ReferenceID int64 // RepoID for pull mirror, MirrorID for push mirror
}

// StartSyncMirrors starts a go routine to sync the mirrors
func StartSyncMirrors(queueHandle func(data ...*SyncRequest) []*SyncRequest) {
	if !setting.Mirror.Enabled {
		return
	}
	mirrorQueue = queue.CreateUniqueQueue("mirror", queueHandle)

	go graceful.GetManager().RunWithShutdownFns(mirrorQueue.Run)
}

// AddPullMirrorToQueue adds repoID to mirror queue
func AddPullMirrorToQueue(repoID int64) {
	addMirrorToQueue(PullMirrorType, repoID)
}

// AddPushMirrorToQueue adds the push mirror to the queue
func AddPushMirrorToQueue(mirrorID int64) {
	addMirrorToQueue(PushMirrorType, mirrorID)
}

func addMirrorToQueue(syncType SyncType, referenceID int64) {
	if !setting.Mirror.Enabled {
		return
	}
	go func() {
		if err := PushToQueue(syncType, referenceID); err != nil {
			log.Error("Unable to push sync request for to the queue for pull mirror repo[%d]. Error: %v", referenceID, err)
		}
	}()
}

// PushToQueue adds the sync request to the queue
func PushToQueue(mirrorType SyncType, referenceID int64) error {
	return mirrorQueue.Push(&SyncRequest{
		Type:        mirrorType,
		ReferenceID: referenceID,
	})
}
