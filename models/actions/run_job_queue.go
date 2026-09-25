// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	"gitea.dev/models/db"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// QueueJobsOptions scopes the build queue to a repo, an owner or, when both are zero, the instance.
type QueueJobsOptions struct {
	RepoID  int64
	OwnerID int64
	Status  Status
}

func (opts QueueJobsOptions) session(ctx context.Context) *xorm.Session {
	// a reusable-workflow caller only tracks its children, it never occupies a runner itself
	sess := db.GetEngine(ctx).Table("action_run_job").Where(builder.Eq{"`action_run_job`.is_reusable_caller": false})
	if opts.RepoID > 0 {
		sess = sess.And(builder.Eq{"`action_run_job`.repo_id": opts.RepoID})
	}
	if opts.OwnerID > 0 {
		sess = sess.Join("INNER", "repository", "repository.id = `action_run_job`.repo_id AND repository.owner_id = ?", opts.OwnerID)
	}
	return sess.And(opts.statusCond())
}

var (
	// keep in sync with CreateTaskForRunner
	queuedJobsCond = builder.Eq{"`action_run_job`.status": StatusWaiting, "`action_run_job`.task_id": 0}
	// a cancelling job still occupies its runner
	runningJobsCond = builder.In("`action_run_job`.status", StatusRunning, StatusCancelling)
)

func (opts QueueJobsOptions) statusCond() builder.Cond {
	switch opts.Status {
	case StatusRunning:
		return runningJobsCond
	case StatusWaiting:
		return queuedJobsCond
	default:
		return builder.Or(runningJobsCond, queuedJobsCond)
	}
}

// active jobs first by start time, then queued jobs in pickup order
var queueJobsOrderBy = fmt.Sprintf(
	"CASE WHEN `action_run_job`.status IN (%d, %d) THEN 0 ELSE 1 END ASC, CASE WHEN `action_run_job`.status IN (%d, %d) THEN `action_run_job`.started ELSE `action_run_job`.updated END ASC, `action_run_job`.id ASC",
	StatusRunning, StatusCancelling, StatusRunning, StatusCancelling)

// FindQueueJobs returns one page of the build queue and its total count.
func FindQueueJobs(ctx context.Context, opts QueueJobsOptions, page, pageSize int) ([]*ActionRunJob, int64, error) {
	total, err := opts.session(ctx).Count(new(ActionRunJob))
	if err != nil || total == 0 {
		return nil, total, err
	}

	// Auto-refresh can shrink the queue under a user still on page 2; show the last page instead of empty.
	page = min(page, int((total+int64(pageSize)-1)/int64(pageSize)))

	jobs := make([]*ActionRunJob, 0, pageSize)
	return jobs, total, opts.session(ctx).
		Cols("`action_run_job`.id", "`action_run_job`.repo_id", "`action_run_job`.name", "`action_run_job`.status", // skip the payload columns
			"`action_run_job`.run_id", "`action_run_job`.runs_on", "`action_run_job`.updated", "`action_run_job`.started", "`action_run_job`.task_id").
		OrderBy(queueJobsOrderBy).
		Limit(pageSize, (page-1)*pageSize).
		Find(&jobs)
}

// QueueFilterRepoIDs returns up to limit ids of the repositories with queued or running jobs.
func QueueFilterRepoIDs(ctx context.Context, limit int) ([]int64, error) {
	var ids []int64
	return ids, QueueJobsOptions{}.session(ctx).Distinct("`action_run_job`.repo_id").Limit(limit).Find(&ids)
}
