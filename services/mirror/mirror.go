// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

var mirrorQueue queue.UniqueQueue

// RequestType type of mirror request
type RequestType int

const (
	// PullRequestType for pull mirrors
	PullRequestType RequestType = iota
	// PushRequestType for push mirrors
	PushRequestType
)

// Request for the mirror queue
type Request struct {
	Type   RequestType
	RepoID int64
}

// doMirror causes this request to mirror itself
func doMirror(ctx context.Context, req *Request) {
	switch req.Type {
	case PushRequestType:
		_ = SyncPushMirror(ctx, req.RepoID)
	case PullRequestType:
		_ = SyncPullMirror(ctx, req.RepoID)
	default:
		log.Error("Unknown Request type in queue: %v for RepoID[%d]", req.Type, req.RepoID)
	}
}

// Update checks and updates mirror repositories.
func Update(ctx context.Context) error {
	if !setting.Mirror.Enabled {
		log.Warn("Mirror feature disabled, but cron job enabled: skip update")
		return nil
	}
	log.Trace("Doing: Update")

	handler := func(idx int, bean interface{}) error {
		var item Request
		if m, ok := bean.(*models.Mirror); ok {
			if m.Repo == nil {
				log.Error("Disconnected mirror found: %d", m.ID)
				return nil
			}
			item = Request{
				Type:   PullRequestType,
				RepoID: m.RepoID,
			}
		} else if m, ok := bean.(*models.PushMirror); ok {
			if m.Repo == nil {
				log.Error("Disconnected push-mirror found: %d", m.ID)
				return nil
			}
			item = Request{
				Type:   PushRequestType,
				RepoID: m.RepoID,
			}
		} else {
			log.Error("Unknown bean: %v", bean)
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("Aborted")
		default:
			return mirrorQueue.Push(&item)
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

func queueHandle(data ...queue.Data) {
	for _, datum := range data {
		req := datum.(*Request)
		doMirror(graceful.GetManager().ShutdownContext(), req)
	}
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	if !setting.Mirror.Enabled {
		return
	}
	mirrorQueue = queue.CreateUniqueQueue("mirror", queueHandle, new(Request))

	go graceful.GetManager().RunWithShutdownFns(mirrorQueue.Run)
}

// StartToMirror adds repoID to mirror queue
func StartToMirror(repoID int64) {
	if !setting.Mirror.Enabled {
		return
	}
	go func() {
		err := mirrorQueue.Push(&Request{
			Type:   PushRequestType,
			RepoID: repoID,
		})
		if err != nil {
			log.Error("Unable to push push mirror request to the queue for repo[%d]: Error: %v", repoID, err)
		}
	}()
}

// AddPushMirrorToQueue adds the push mirror to the queue
func AddPushMirrorToQueue(mirrorID int64) {
	if !setting.Mirror.Enabled {
		return
	}
	go func() {

		err := mirrorQueue.Push(&Request{
			Type:   PullRequestType,
			RepoID: mirrorID,
		})
		if err != nil {
			log.Error("Unable to push pull mirror request to the queue for repo[%d]: Error: %v", mirrorID, err)
		}
	}()
}
