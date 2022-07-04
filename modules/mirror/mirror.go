// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

var mirrorQueue queue.UniqueQueue

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
	ReferenceID int64 // RepoID for pull mirror, MirrorID fro push mirror
}

// StartSyncMirrors starts a go routine to sync the mirrors
func StartSyncMirrors(queueHandle func(data ...queue.Data) []queue.Data) {
	if !setting.Mirror.Enabled {
		return
	}
	mirrorQueue = queue.CreateUniqueQueue("mirror", queueHandle, new(SyncRequest))

	go graceful.GetManager().RunWithShutdownFns(mirrorQueue.Run)
}

// AddPullMirrorToQueue adds repoID to mirror queue
func AddPullMirrorToQueue(repoID int64) {
	if !setting.Mirror.Enabled {
		return
	}
	go func() {
		err := PushToQueue(PullMirrorType, repoID)
		if err != nil {
			log.Error("Unable to push sync request for to the queue for pull mirror repo[%d]: Error: %v", repoID, err)
			return
		}
	}()
}

// AddPushMirrorToQueue adds the push mirror to the queue
func AddPushMirrorToQueue(mirrorID int64) {
	if !setting.Mirror.Enabled {
		return
	}
	go func() {
		err := PushToQueue(PushMirrorType, mirrorID)
		if err != nil {
			log.Error("Unable to push sync request to the queue for pull mirror repo[%d]: Error: %v", mirrorID, err)
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
