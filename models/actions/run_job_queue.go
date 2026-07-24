// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/modules/optional"

	"xorm.io/builder"
)

// queueRankStep spaces manually assigned queue ranks so an insertion between two neighbours
// almost always has integer room without rewriting the whole page.
const queueRankStep int64 = 1 << 16

// queueScopeOpts builds the FindRunJobOptions that select the waiting, unclaimed, non-reusable-caller
// jobs a runner could pick up, for the same scope the build queue view uses:
// repoID>0 → a single repo; ownerID>0 → an org/user; both 0 → the whole instance.
func queueScopeOpts(repoID, ownerID int64) FindRunJobOptions {
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
	opts := queueScopeOpts(repoID, ownerID)
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

// MoveQueuedJob repositions a waiting job in the build queue so runners pick it up in the new order.
//
// scope: repoID>0 for a repo queue; ownerID>0 for an org/user queue; both 0 for the instance-wide queue.
// afterID / beforeID are the ids of the rows that should end up immediately before / after the moved job
// (0 when it was dropped at the top / bottom of the list). page/pageSize describe the page the admin was
// viewing; reordering is bounded to that page so it stays cheap regardless of the total queue size.
//
// The dropped page is renumbered into evenly spaced negative ranks (more negative = picked earlier), placed
// strictly ahead of the following page's head. Untouched, rank-0 jobs therefore keep their natural FIFO
// position at the tail, and a newly queued job (rank 0) never jumps ahead of a manually curated queue.
// Ranks are written with NoAutoTime so the Updated FIFO tiebreak is preserved.
//
// It returns false (with a nil error) when the moved job or a named neighbour is no longer queueable on that
// page, i.e. the client's view is stale and should refresh.
func MoveQueuedJob(ctx context.Context, repoID, ownerID int64, page, pageSize int, movedID, afterID, beforeID int64) (ok bool, err error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}

	err = db.WithTx(ctx, func(ctx context.Context) error {
		offset := (page - 1) * pageSize

		opts := queueScopeOpts(repoID, ownerID)
		opts.ListOptions = db.ListOptions{Page: page, PageSize: pageSize}
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
			return nil // moved row left the page (claimed/finished) → stale
		}
		if afterID != 0 {
			if _, ok := idxByID[afterID]; !ok {
				return nil // neighbour gone → stale
			}
		}
		if beforeID != 0 {
			if _, ok := idxByID[beforeID]; !ok {
				return nil
			}
		}
		moved := window[movedIdx]

		// Build the new page order: drop the moved row, reinsert it relative to its neighbour.
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
		} else if beforeID != 0 {
			for i, j := range newOrder {
				if j.ID == beforeID {
					insertPos = i
					break
				}
			}
		}
		newOrder = slices.Insert(newOrder, insertPos, moved)

		// Anchor below the following page's head (0 = the natural-FIFO tail when there is no next page),
		// so the whole renumbered page stays ahead of every rank-0 job.
		hi, hiOK, err := queueRankAtIndex(ctx, repoID, ownerID, offset+len(window))
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
		if err := IncreaseTaskVersion(ctx, moved.OwnerID, moved.RepoID); err != nil {
			return err
		}
		ok = true
		return nil
	})
	return ok, err
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
