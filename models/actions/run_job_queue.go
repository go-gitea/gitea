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

// RunningJobsOptions selects the jobs currently occupying a runner.
func RunningJobsOptions(repoID, ownerID int64) FindRunJobOptions {
	return FindRunJobOptions{
		RepoID:           repoID,
		OwnerID:          ownerID,
		Statuses:         []Status{StatusRunning},
		IsReusableCaller: optional.Some(false),
		OrderBy:          RunningJobsOrderBy,
	}
}

// QueueFilterRepoIDs returns the ids of the repositories that currently have a queued or running job in
// the given scope (see QueuedJobsOptions), so the build-queue filters only offer values that can match.
// At most limit ids are returned; the list is bounded by pending work rather than by repository count.
func QueueFilterRepoIDs(ctx context.Context, repoID, ownerID int64, limit int) ([]int64, error) {
	opts := QueuedJobsOptions(repoID, ownerID)
	cond := builder.Or(opts.ToConds(), builder.Eq{"`action_run_job`.status": StatusRunning})
	if repoID > 0 {
		cond = cond.And(builder.Eq{"`action_run_job`.repo_id": repoID}) // the running branch of the OR carries no scope of its own
	}

	sess := db.GetEngine(ctx).Table("action_run_job")
	for _, join := range opts.ToJoins() {
		if err := join(sess); err != nil {
			return nil, err
		}
	}
	ids := make([]int64, 0, 10)
	return ids, sess.Where(cond).
		Distinct("`action_run_job`.repo_id").Cols("`action_run_job`.repo_id").Limit(limit).Find(&ids)
}

// MoveQueuedJob repositions a waiting job in the instance-wide build queue (first page only).
// QueueRank is a global ordering key, so the window renumbered here is always the instance-wide queue:
// renumbering a repo- or owner-scoped window would let its jobs jump every other repository's queue.
//
// afterID is the id of the row that should end up immediately before the moved job (0 = move to head).
//
// The first page is renumbered into evenly spaced negative ranks (more negative = picked earlier),
// placed strictly ahead of the following page's head. Untouched, rank-0 jobs keep their natural FIFO
// position at the tail, and a newly queued job (rank 0) never jumps ahead of a manually curated queue.
// Ranks are written with NoAutoTime so the Updated FIFO tiebreak is preserved.
//
// It returns false (with a nil error) when the moved job or afterID neighbour is no longer on the
// first page, i.e. the client's view is stale and should refresh.
func MoveQueuedJob(ctx context.Context, movedID, afterID int64) (bool, error) {
	err := db.WithTx(ctx, func(ctx context.Context) error {
		opts := QueuedJobsOptions(0, 0)
		// One row past the page is the rebalance anchor, so the page costs a single query.
		opts.ListOptions = db.ListOptions{Page: 1, PageSize: QueuePageSize + 1}
		window, err := db.Find[ActionRunJob](ctx, opts)
		if err != nil {
			return err
		}
		// Anchor below the following page's head (0 = the natural-FIFO tail when there is no next page),
		// so the whole renumbered page stays ahead of every rank-0 job.
		var hi int64
		if len(window) > QueuePageSize {
			hi = window[QueuePageSize].QueueRank
			window = window[:QueuePageSize]
		}

		movedIdx := slices.IndexFunc(window, func(j *ActionRunJob) bool { return j.ID == movedID })
		if movedIdx < 0 {
			return errQueueStale
		}

		// Build the new page order: drop the moved row, reinsert after afterID (or at head).
		moved := window[movedIdx]
		newOrder := slices.Delete(slices.Clone(window), movedIdx, movedIdx+1)
		insertPos := 0
		if afterID != 0 {
			afterIdx := slices.IndexFunc(newOrder, func(j *ActionRunJob) bool { return j.ID == afterID })
			if afterIdx < 0 {
				return errQueueStale
			}
			insertPos = afterIdx + 1
		}
		newOrder = slices.Insert(newOrder, insertPos, moved)

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
