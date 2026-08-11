// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/modules/optional"

	"xorm.io/builder"
)

// QueuePageSize is the number of queued jobs per build-queue page. Reordering renumbers exactly
// the first page (the head runners pick from).
const QueuePageSize = 50

// queueRankStep spaces ranks assigned during a full page renumber (more negative = earlier pickup).
const queueRankStep int64 = 1 << 16

// errQueueStale signals that the client's queue view no longer matches the first page
// (a job left the queue or a named neighbour disappeared).
var errQueueStale = errors.New("actions queue view is stale")

// QueuedJobsOptions selects waiting, unclaimed, non-reusable-caller jobs in runner pickup order
// for the same scopes the build queue view uses:
// repoID>0 → a single repo; ownerID>0 → an org/user; both 0 → the whole instance.
// Keep the predicate/order in sync with CreateTaskForRunner.
func QueuedJobsOptions(repoID, ownerID int64) FindRunJobOptions {
	return FindRunJobOptions{
		RepoID:           repoID,
		OwnerID:          ownerID,
		Statuses:         []Status{StatusWaiting},
		IsReusableCaller: optional.Some(false),
		HasTask:          optional.Some(false),
		OrderBy:          QueuedJobsOrderBy,
	}
}

// queueRankAtIndex returns the queue_rank of the waiting job at the given 0-based position in the
// scope's pickup order, so callers can anchor a rebalance to a job just outside the current page.
// found is false when no such row exists (idx past the end, or negative).
func queueRankAtIndex(ctx context.Context, repoID, ownerID int64, idx int) (rank int64, found bool, err error) {
	if idx < 0 {
		return 0, false, nil
	}
	opts := QueuedJobsOptions(repoID, ownerID)
	opts.ListOptions = db.ListOptions{Page: idx + 1, PageSize: 1}
	rows, err := db.Find[ActionRunJob](ctx, opts)
	if err != nil {
		return 0, false, err
	}
	if len(rows) == 0 {
		return 0, false, nil
	}
	return rows[0].QueueRank, true, nil
}

// MoveQueuedJob repositions a waiting job at the head of the build queue (first page only).
//
// scope: repoID>0 for a repo queue; ownerID>0 for an org/user queue; both 0 for the instance-wide queue.
// afterID is the id of the row that should end up immediately before the moved job (0 = move to head).
//
// The first page is renumbered into evenly spaced negative ranks (more negative = picked earlier),
// placed strictly ahead of the following page's head. Untouched, rank-0 jobs keep their natural FIFO
// position at the tail, and a newly queued job (rank 0) never jumps ahead of a manually curated queue.
// Ranks are written with NoAutoTime so the Updated FIFO tiebreak is preserved.
//
// It returns false (with a nil error) when the moved job or afterID neighbour is no longer on the
// first page, i.e. the client's view is stale and should refresh.
func MoveQueuedJob(ctx context.Context, repoID, ownerID, movedID, afterID int64) (bool, error) {
	err := db.WithTx(ctx, func(ctx context.Context) error {
		opts := QueuedJobsOptions(repoID, ownerID)
		opts.ListOptions = db.ListOptions{Page: 1, PageSize: QueuePageSize}
		window, err := db.Find[ActionRunJob](ctx, opts)
		if err != nil {
			return err
		}

		idxByID := make(map[int64]int, len(window))
		for i, j := range window {
			idxByID[j.ID] = i
		}
		movedIdx, found := idxByID[movedID]
		if !found {
			return errQueueStale
		}
		if afterID != 0 {
			if _, ok := idxByID[afterID]; !ok {
				return errQueueStale
			}
		}
		moved := window[movedIdx]

		// Build the new page order: drop the moved row, reinsert after afterID (or at head).
		newOrder := make([]*ActionRunJob, 0, len(window))
		for _, j := range window {
			if j.ID != movedID {
				newOrder = append(newOrder, j)
			}
		}
		insertPos := 0
		if afterID != 0 {
			for i, j := range newOrder {
				if j.ID == afterID {
					insertPos = i + 1
					break
				}
			}
		}
		newOrder = slices.Insert(newOrder, insertPos, moved)

		// Anchor below the following page's head (0 = the natural-FIFO tail when there is no next page),
		// so the whole renumbered page stays ahead of every rank-0 job.
		hi, hiOK, err := queueRankAtIndex(ctx, repoID, ownerID, len(window))
		if err != nil {
			return err
		}
		if !hiOK {
			hi = 0
		}

		n := int64(len(newOrder))
		for i, job := range newOrder {
			newRank := hi - (n-int64(i))*queueRankStep
			if job.QueueRank == newRank {
				continue
			}
			if _, err := updateJobQueueRank(ctx, job.ID, newRank); err != nil {
				return err
			}
		}

		// Wake idle runners so the new order takes effect on the next poll rather than after a timeout.
		return IncreaseTaskVersion(ctx, moved.OwnerID, moved.RepoID)
	})
	if errors.Is(err, errQueueStale) {
		return false, nil
	}
	return err == nil, err
}

// updateJobQueueRank sets a job's QueueRank without bumping Updated (the queue FIFO tiebreak).
// The status/task_id guard ensures a job that has just been claimed or finished is never reordered;
// such an update simply affects zero rows.
func updateJobQueueRank(ctx context.Context, jobID, rank int64) (int64, error) {
	return db.GetEngine(ctx).ID(jobID).
		Where(builder.Eq{"status": StatusWaiting, "task_id": 0}).
		Cols("queue_rank").NoAutoTime().
		Update(&ActionRunJob{QueueRank: rank})
}
