// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"
	"errors"
	"fmt"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/setting"
)

// doMirrorSync causes this request to mirror itself
func doMirrorSync(ctx context.Context, req *SyncRequest) {
	if req.ReferenceID == 0 {
		log.Warn("Skipping mirror sync request, no mirror ID was specified")
		return
	}
	switch req.Type {
	case PushMirrorType:
		_ = SyncPushMirror(ctx, req.ReferenceID)
	case PullMirrorType:
		_ = SyncPullMirror(ctx, req.ReferenceID)
	default:
		log.Error("Unknown Request type in queue: %v for MirrorID[%d]", req.Type, req.ReferenceID)
	}
}

var errLimit = errors.New("reached limit")

func describeMirrorSync(mirrorType SyncType, repo *repo_model.Repository) string {
	repoName := "unknown repository"
	if repo != nil {
		repoName = repo.FullName()
	}

	switch mirrorType {
	case PullMirrorType:
		return "pull mirror repository " + repoName
	case PushMirrorType:
		return "push mirror repository " + repoName
	default:
		return "mirror repository " + repoName
	}
}

func queueMirrorSync(ctx context.Context, repo *repo_model.Repository, mirrorType SyncType, referenceID int64) error {
	mirrorDesc := describeMirrorSync(mirrorType, repo)

	select {
	case <-ctx.Done():
		return db.ErrCancelledf("before queueing %s", mirrorDesc)
	default:
	}

	if err := PushToQueue(mirrorType, referenceID); err != nil {
		if err == queue.ErrAlreadyInQueue {
			log.Trace("%s already queued for sync", mirrorDesc)
			return nil
		}
		return fmt.Errorf("queue %s: %w", mirrorDesc, err)
	}

	return nil
}

// Update checks and updates mirror repositories.
func Update(ctx context.Context, pullLimit, pushLimit int) error {
	if !setting.Mirror.Enabled {
		log.Warn("Mirror feature disabled, but cron job enabled: skip update")
		return nil
	}
	log.Trace("Doing: Update")

	handler := func(bean any) error {
		var repo *repo_model.Repository
		var mirrorType SyncType
		var referenceID int64

		if m, ok := bean.(*repo_model.Mirror); ok {
			if m.GetRepository(ctx) == nil {
				log.Error("Disconnected mirror found: %d", m.ID)
				return nil
			}
			repo = m.Repo
			mirrorType = PullMirrorType
			referenceID = m.RepoID
		} else if m, ok := bean.(*repo_model.PushMirror); ok {
			if m.GetRepository(ctx) == nil {
				log.Error("Disconnected push-mirror found: %d", m.ID)
				return nil
			}
			repo = m.Repo
			mirrorType = PushMirrorType
			referenceID = m.ID
		} else {
			log.Error("Unknown bean: %v", bean)
			return nil
		}

		return queueMirrorSync(ctx, repo, mirrorType, referenceID)
	}

	pullMirrorsRequested := 0
	if pullLimit != 0 {
		if err := repo_model.MirrorsIterate(ctx, pullLimit, func(_ int, bean any) error {
			if err := handler(bean); err != nil {
				return err
			}
			pullMirrorsRequested++
			return nil
		}); err != nil && err != errLimit {
			log.Error("MirrorsIterate: %v", err)
			return err
		}
	}

	pushMirrorsRequested := 0
	if pushLimit != 0 {
		if err := repo_model.PushMirrorsIterate(ctx, pushLimit, func(idx int, bean any) error {
			if err := handler(bean); err != nil {
				return err
			}
			pushMirrorsRequested++
			return nil
		}); err != nil && err != errLimit {
			log.Error("PushMirrorsIterate: %v", err)
			return err
		}
	}
	log.Trace("Finished: Update: %d pull mirrors and %d push mirrors queued", pullMirrorsRequested, pushMirrorsRequested)
	return nil
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	StartSyncMirrors()
}
