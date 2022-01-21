// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
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
	Type   SyncType
	RepoID int64
}

// doMirrorSync causes this request to mirror itself
func doMirrorSync(ctx context.Context, req *SyncRequest) {
	switch req.Type {
	case PushMirrorType:
		_ = SyncPushMirror(ctx, req.RepoID)
	case PullMirrorType:
		_ = SyncPullMirror(ctx, req.RepoID)
	default:
		log.Error("Unknown Request type in queue: %v for RepoID[%d]", req.Type, req.RepoID)
	}
}

var errLimit = fmt.Errorf("reached limit")

// Update checks and updates mirror repositories.
func Update(ctx context.Context, pullLimit, pushLimit int) error {
	if !setting.Mirror.Enabled {
		log.Warn("Mirror feature disabled, but cron job enabled: skip update")
		return nil
	}
	log.Trace("Doing: Update")

	requested := 0

	handler := func(idx int, bean interface{}, limit int) error {
		var item SyncRequest
		if m, ok := bean.(*repo_model.Mirror); ok {
			if m.Repo == nil {
				log.Error("Disconnected mirror found: %d", m.ID)
				return nil
			}
			item = SyncRequest{
				Type:   PullMirrorType,
				RepoID: m.RepoID,
			}
		} else if m, ok := bean.(*repo_model.PushMirror); ok {
			if m.Repo == nil {
				log.Error("Disconnected push-mirror found: %d", m.ID)
				return nil
			}
			item = SyncRequest{
				Type:   PushMirrorType,
				RepoID: m.RepoID,
			}
		} else {
			log.Error("Unknown bean: %v", bean)
			return nil
		}

		// Check we've not been cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("aborted")
		default:
		}

		// Check if this request is already in the queue
		has, err := mirrorQueue.Has(&item)
		if err != nil {
			return err
		}
		if has {
			return nil
		}

		// Push to the Queue
		if err := mirrorQueue.Push(&item); err != nil {
			return err
		}

		requested++
		if limit > 0 && requested > limit {
			return errLimit
		}
		return nil
	}

	if pullLimit != 0 {
		if err := repo_model.MirrorsIterate(func(idx int, bean interface{}) error {
			return handler(idx, bean, pullLimit)
		}); err != nil && err != errLimit {
			log.Error("MirrorsIterate: %v", err)
			return err
		}
	}
	if pushLimit != 0 {
		if err := repo_model.PushMirrorsIterate(func(idx int, bean interface{}) error {
			return handler(idx, bean, pushLimit)
		}); err != nil && err != errLimit {
			log.Error("PushMirrorsIterate: %v", err)
			return err
		}
	}
	log.Trace("Finished: Update")
	return nil
}

func queueHandle(data ...queue.Data) {
	for _, datum := range data {
		req := datum.(*SyncRequest)
		doMirrorSync(graceful.GetManager().ShutdownContext(), req)
	}
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	if !setting.Mirror.Enabled {
		return
	}
	mirrorQueue = queue.CreateUniqueQueue("mirror", queueHandle, new(SyncRequest))

	go graceful.GetManager().RunWithShutdownFns(mirrorQueue.Run)
}

// StartToMirror adds repoID to mirror queue
func StartToMirror(repoID int64) {
	if !setting.Mirror.Enabled {
		return
	}
	go func() {
		err := mirrorQueue.Push(&SyncRequest{
			Type:   PullMirrorType,
			RepoID: repoID,
		})
		if err != nil {
			log.Error("Unable to push sync request for to the queue for push mirror repo[%d]: Error: %v", repoID, err)
		}
	}()
}

// AddPushMirrorToQueue adds the push mirror to the queue
func AddPushMirrorToQueue(mirrorID int64) {
	if !setting.Mirror.Enabled {
		return
	}
	go func() {
		err := mirrorQueue.Push(&SyncRequest{
			Type:   PushMirrorType,
			RepoID: mirrorID,
		})
		if err != nil {
			log.Error("Unable to push sync request to the queue for pull mirror repo[%d]: Error: %v", mirrorID, err)
		}
	}()
}
