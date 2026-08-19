// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automergequeue

import (
	"errors"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"
)

// Item is a queue item, either a known pull request, or a repository commit whose pull requests still need to be found
type Item struct {
	PullID   int64  `json:"pull_id,omitempty"`
	RepoID   int64  `json:"repo_id,omitempty"`
	CommitID string `json:"commit_id,omitempty"`
}

var AutoMergeQueue *queue.WorkerPoolQueue[Item]

func push(item Item) {
	if err := AutoMergeQueue.Push(item); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
		log.Error("Error adding %v to the automerge queue: %v", item, err)
	}
}

var AddToQueue = func(pull *issues_model.PullRequest) {
	log.Trace("Adding pullID: %d to the automerge queue", pull.ID)
	push(Item{PullID: pull.ID})
}

// StartPRCheckAndAutoMerge start an automerge check and auto merge task for a pull request
func StartPRCheckAndAutoMerge(pull *issues_model.PullRequest) {
	if pull == nil || pull.HasMerged || !pull.IsStatusMergeable() {
		return
	}
	AddToQueue(pull)
}

// StartPRCheckAndAutoMergeBySHA start an automerge check and auto merge task for all pull requests of a repository and SHA
func StartPRCheckAndAutoMergeBySHA(repoID int64, commitID string) {
	log.Trace("Adding repoID: %d commitID: %s to the automerge queue", repoID, commitID)
	push(Item{RepoID: repoID, CommitID: commitID})
}
