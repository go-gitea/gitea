// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/optional"

	"xorm.io/builder"
)

// QueuePageSize is the number of queued jobs per build-queue page.
const QueuePageSize = 50

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
