// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automergequeue

import (
	"context"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
)

var AutoMergeQueue *queue.WorkerPoolQueue[string]

var AddToQueue = func(pr *issues_model.PullRequest, sha string) {
	log.Trace("Adding pullID: %d to the pull requests patch checking queue with sha %s", pr.ID, sha)
	if err := AutoMergeQueue.Push(fmt.Sprintf("%d_%s", pr.ID, sha)); err != nil {
		log.Error("Error adding pullID: %d to the pull requests patch checking queue %v", pr.ID, err)
	}
}

// StartPRCheckAndAutoMerge start an automerge check and auto merge task for a pull request
func StartPRCheckAndAutoMerge(ctx context.Context, pull *issues_model.PullRequest) {
	if pull == nil || pull.HasMerged || !pull.CanAutoMerge() {
		return
	}

	if err := pull.LoadBaseRepo(ctx); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, pull.BaseRepo)
	if err != nil {
		log.Error("OpenRepository: %v", err)
		return
	}
	defer gitRepo.Close()
	commitID, err := gitRepo.GetRefCommitID(pull.GetGitHeadRefName())
	if err != nil {
		log.Error("GetRefCommitID: %v", err)
		return
	}

	AddToQueue(pull, commitID)
}
