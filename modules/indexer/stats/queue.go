// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package stats

import (
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

// statsQueue represents a queue to handle repository stats updates
var statsQueue *queue.WorkerPoolQueue[int64]

// handle passed PR IDs and test the PRs
func handler(items ...int64) []int64 {
	for _, opts := range items {
		if err := indexer.Index(opts); err != nil {
			if !setting.IsInTesting {
				log.Error("stats queue indexer.Index(%d) failed: %v", opts, err)
			}
		}
	}
	return nil
}

func initStatsQueue() error {
	statsQueue = queue.CreateUniqueQueue("repo_stats_update", handler)
	if statsQueue == nil {
		return fmt.Errorf("Unable to create repo_stats_update Queue")
	}

	go graceful.GetManager().RunWithShutdownFns(statsQueue.Run)

	return nil
}

// UpdateRepoIndexer update a repository's entries in the indexer
func UpdateRepoIndexer(repo *repo_model.Repository) error {
	if err := statsQueue.Push(repo.ID); err != nil {
		if err != queue.ErrAlreadyInQueue {
			return err
		}
		log.Debug("Repo ID: %d already queued", repo.ID)
	}
	return nil
}
