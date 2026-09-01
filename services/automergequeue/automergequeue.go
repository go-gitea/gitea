// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automergequeue

import (
	"context"
	"errors"
	"strconv"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"
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
