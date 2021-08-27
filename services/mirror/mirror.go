// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
)

// mirrorQueue holds an UniqueQueue object of the mirror
var mirrorQueue = sync.NewUniqueQueue(setting.Repository.MirrorQueueLength)

// Update checks and updates mirror repositories.
func Update(ctx context.Context) error {
	log.Trace("Doing: Update")

	handler := func(idx int, bean interface{}) error {
		var item string
		if m, ok := bean.(*models.Mirror); ok {
			if m.Repo == nil {
				log.Error("Disconnected mirror found: %d", m.ID)
				return nil
			}
			item = fmt.Sprintf("pull %d", m.RepoID)
		} else if m, ok := bean.(*models.PushMirror); ok {
			if m.Repo == nil {
				log.Error("Disconnected push-mirror found: %d", m.ID)
				return nil
			}
			item = fmt.Sprintf("push %d", m.ID)
		} else {
			log.Error("Unknown bean: %v", bean)
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("Aborted")
		default:
			mirrorQueue.Add(item)
			return nil
		}
	}

	if err := models.MirrorsIterate(handler); err != nil {
		log.Error("MirrorsIterate: %v", err)
		return err
	}
	if err := models.PushMirrorsIterate(handler); err != nil {
		log.Error("PushMirrorsIterate: %v", err)
		return err
	}
	log.Trace("Finished: Update")
	return nil
}

// syncMirrors checks and syncs mirrors.
// FIXME: graceful: this should be a persistable queue
func syncMirrors(ctx context.Context) {
	// Start listening on new sync requests.
	for {
		select {
		case <-ctx.Done():
			mirrorQueue.Close()
			return
		case item := <-mirrorQueue.Queue():
			id, _ := strconv.ParseInt(item[5:], 10, 64)
			if strings.HasPrefix(item, "pull") {
				_ = SyncPullMirror(ctx, id)
			} else if strings.HasPrefix(item, "push") {
				_ = SyncPushMirror(ctx, id)
			} else {
				log.Error("Unknown item in queue: %v", item)
			}
			mirrorQueue.Remove(item)
		}
	}
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	go graceful.GetManager().RunWithShutdownContext(syncMirrors)
}

// StartToMirror adds repoID to mirror queue
func StartToMirror(repoID int64) {
	go mirrorQueue.Add(fmt.Sprintf("pull %d", repoID))
}

// AddPushMirrorToQueue adds the push mirror to the queue
func AddPushMirrorToQueue(mirrorID int64) {
	go mirrorQueue.Add(fmt.Sprintf("push %d", mirrorID))
}
