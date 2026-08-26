// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automergequeue

import (
	"errors"
	"strconv"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"
)

type AutoMergeItem string // it is a unique queue, so the item type can't be JSON which doesn't have deterministic key order.

var AutoMergeQueue *queue.WorkerPoolQueue[AutoMergeItem]

var AddToQueue = func(item AutoMergeItem) {
	if err := AutoMergeQueue.Push(item); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
		log.Error("Error adding %v to the automerge queue: %v", item, err)
	}
}

func StartPRCheckAndAutoMerge(pull *issues_model.PullRequest) {
	AddToQueue(AutoMergeItem(strconv.FormatInt(pull.ID, 10)))
}
