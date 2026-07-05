// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"
)

// AIRreviewTask represents a queued AI code review job.
type AIRreviewTask struct {
	PRID  int64
	Event string // "opened" or "synchronized"
}

var reviewQueue *queue.WorkerPoolQueue[AIRreviewTask]

func handler(items ...AIRreviewTask) []AIRreviewTask {
	ctx := graceful.GetManager().HammerContext()
	for _, task := range items {
		if err := RunReview(ctx, &task); err != nil {
			log.Error("aireview: failed to review PR %d (event=%s): %v", task.PRID, task.Event, err)
		}
	}
	return nil
}

// Init initializes the AI review queue. Must be called after settings are loaded.
func Init() error {
	reviewQueue = queue.CreateSimpleQueue(
		graceful.GetManager().ShutdownContext(),
		"ai_review",
		handler,
	)
	if reviewQueue == nil {
		return nil
	}
	go graceful.GetManager().RunWithCancel(reviewQueue)
	return nil
}
