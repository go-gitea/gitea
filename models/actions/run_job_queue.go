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

// queueJobCols lists the columns the build-queue view reads. The row also carries several payload/blob
// columns (WorkflowPayload, DeferredMatrixPayload, ReusableWorkflowContent) that the queue never touches,
// so restricting the SELECT keeps its 3-second auto-refresh cheap.
// Qualified with the table name: an owner-scoped query joins `repository`, whose own "id" column
// would otherwise make an unqualified "id" ambiguous.
var queueJobCols = []string{
	"`action_run_job`.id", "`action_run_job`.repo_id", "`action_run_job`.name", "`action_run_job`.status",
	"`action_run_job`.run_id", "`action_run_job`.runs_on", "`action_run_job`.updated", "`action_run_job`.started",
	"`action_run_job`.task_id", "`action_run_job`.source_task_id",
}

// QueueJobsOptions scopes a build-queue query: RepoID>0 → a single repo; OwnerID>0 → an org/user;
// both zero → the whole instance. Status narrows the list to running or queued jobs, StatusUnknown lists both.
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
	// queuedJobsCond matches the jobs a runner may still pick up: waiting and not yet claimed by a task.
	// Keep it in sync with CreateTaskForRunner, which claims jobs oldest-ready-first.
	queuedJobsCond  = builder.Eq{"`action_run_job`.status": StatusWaiting, "`action_run_job`.task_id": 0}
	runningJobsCond = builder.Eq{"`action_run_job`.status": StatusRunning}
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

// queueJobsOrderBy puts the running jobs first (StatusRunning sorts above StatusWaiting), then orders each
// group by the very timestamp the row displays: a running job by its start, a queued one by its pickup order.
var queueJobsOrderBy = fmt.Sprintf(
	"`action_run_job`.status DESC, CASE WHEN `action_run_job`.status = %d THEN `action_run_job`.started ELSE `action_run_job`.updated END ASC, `action_run_job`.id ASC",
	StatusRunning)

// FindQueueJobs returns one page of the build queue and its total count.
func FindQueueJobs(ctx context.Context, opts QueueJobsOptions, page, pageSize int) ([]*ActionRunJob, int64, error) {
	total, err := opts.session(ctx).Count(new(ActionRunJob))
	if err != nil || total == 0 {
		return nil, total, err
	}

	jobs := make([]*ActionRunJob, 0, pageSize)
	return jobs, total, opts.session(ctx).
		Cols(queueJobCols...).
		OrderBy(queueJobsOrderBy).
		Limit(pageSize, (page-1)*pageSize).
		Find(&jobs)
}

// QueueFilterRepoIDs returns the ids of the repositories that currently have a queued or running job in
// the given scope, so the build-queue filters only offer values that can match. At most limit ids are
// returned; the list is bounded by pending work rather than by repository count.
func QueueFilterRepoIDs(ctx context.Context, opts QueueJobsOptions, limit int) ([]int64, error) {
	ids := make([]int64, 0, 10)
	return ids, opts.session(ctx).
		Distinct("`action_run_job`.repo_id").Cols("`action_run_job`.repo_id").Limit(limit).Find(&ids)
}
