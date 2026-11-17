// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automergequeue

import (
	"context"
	"errors"
	"strconv"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"

	"xorm.io/builder"
)

// AutoMergeItem is for the unique queue, so the item type can't be JSON which doesn't have deterministic key order.
// Since the queue is a unique queue, the item must contain commit ID, otherwise the new commit ID will be ignored.
type AutoMergeItem string

var AutoMergeQueue *queue.WorkerPoolQueue[AutoMergeItem]

var AddToQueue = func(item AutoMergeItem) {
	if err := AutoMergeQueue.Push(item); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
		log.Error("Error adding %v to the automerge queue: %v", item, err)
	}
}

func StartAutoMergeCheckByPullCommit(pullID int64, commitID string) {
	AddToQueue(AutoMergeItem("pr:" + strconv.FormatInt(pullID, 10) + ":" + commitID))
}

func StartAutoMergeCheckByPullHead(ctx context.Context, pull *issues_model.PullRequest) {
	if err := pull.LoadBaseRepo(ctx); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return
	}

	gitRepo, err := git.OpenRepository(ctx, pull.BaseRepo)
	if err != nil {
		log.Error("OpenRepository: %v", err)
		return
	}
	defer gitRepo.Close()

	commitID, err := gitRepo.GetRefCommitID(ctx, pull.GetGitHeadRefName())
	if err != nil {
		log.Error("GetRefCommitID: %v", err)
		return
	}
	StartAutoMergeCheckByPullCommit(pull.ID, commitID)
}

// StartAutoMergeCheckIfScheduled avoids opening the git repository for events on pull requests without a scheduled auto merge
func StartAutoMergeCheckIfScheduled(ctx context.Context, pull *issues_model.PullRequest) {
	scheduled, err := db.Exist[pull_model.AutoMerge](ctx, builder.Eq{"pull_id": pull.ID})
	if err != nil {
		log.Error("Exist AutoMerge: %v", err)
		return
	}
	if scheduled {
		StartAutoMergeCheckByPullHead(ctx, pull)
	}
}
