// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/base"
	"gitea.dev/modules/container"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

type ActionJobList []*ActionRunJob

func (jobs ActionJobList) GetRunIDs() []int64 {
	return container.FilterSlice(jobs, func(j *ActionRunJob) (int64, bool) {
		return j.RunID, j.RunID != 0
	})
}

// SortMatrixGroupsByName natural-sorts each contiguous run of jobs that share a JobID
// so matrix expansions (e.g. "test (1)", "test (2)", "test (10)") appear in human order.
// Input is expected to be in DB id order so JobID groups are contiguous; cross-group order is preserved.
func (jobs ActionJobList) SortMatrixGroupsByName() {
	for i := 0; i < len(jobs); {
		j := i + 1
		for j < len(jobs) && jobs[j].JobID == jobs[i].JobID {
			j++
		}
		slices.SortFunc(jobs[i:j], func(a, b *ActionRunJob) int {
			return base.NaturalSortCompare(a.Name, b.Name)
		})
		i = j
	}
}

func (jobs ActionJobList) LoadRepos(ctx context.Context) error {
	repoIDs := container.FilterSlice(jobs, func(j *ActionRunJob) (int64, bool) {
		return j.RepoID, j.RepoID != 0 && j.Repo == nil
	})
	if len(repoIDs) == 0 {
		return nil
	}

	repos := make(map[int64]*repo_model.Repository, len(repoIDs))
	if err := db.GetEngine(ctx).In("id", repoIDs).Find(&repos); err != nil {
		return err
	}
	for _, j := range jobs {
		if j.RepoID > 0 && j.Repo == nil {
			j.Repo = repos[j.RepoID]
		}
	}
	return nil
}

func (jobs ActionJobList) LoadRuns(ctx context.Context, withRepo bool) error {
	if withRepo {
		if err := jobs.LoadRepos(ctx); err != nil {
			return err
		}
	}

	runIDs := jobs.GetRunIDs()
	runs := make(map[int64]*ActionRun, len(runIDs))
	if err := db.GetEngine(ctx).In("id", runIDs).Find(&runs); err != nil {
		return err
	}
	for _, j := range jobs {
		if j.Run == nil {
			j.Run = runs[j.RunID]
		}
		if j.Run != nil {
			j.Run.Repo = j.Repo
		}
	}
	return nil
}

func (jobs ActionJobList) LoadAttributes(ctx context.Context, withRepo bool) error {
	return jobs.LoadRuns(ctx, withRepo)
}

// QueuedJobsOrderBy mirrors the runner pickup order (see CreateTaskForRunner): waiting jobs are
// claimed by queue_rank first (manually reordered jobs carry a negative rank and sort ahead of the
// natural, rank-0 FIFO block), then oldest-ready-first keyed on (updated, id).
// Keep this in sync with the ORDER BY / keyset cursor in CreateTaskForRunner.
const QueuedJobsOrderBy db.SearchOrderBy = "`action_run_job`.queue_rank ASC, `action_run_job`.updated ASC, `action_run_job`.id ASC"

// RunningJobsOrderBy lists currently running jobs longest-running-first.
const RunningJobsOrderBy db.SearchOrderBy = "`action_run_job`.started ASC, `action_run_job`.id ASC"

type FindRunJobOptions struct {
	db.ListOptions
	RunID            int64
	RunAttemptID     optional.Option[int64] // use optional to allow filtering by zero (legacy jobs have run_attempt_id=0)
	RepoID           int64
	OwnerID          int64
	CommitSHA        string
	Statuses         []Status
	IsReusableCaller optional.Option[bool] // use optional to filter reusable-caller rows in/out; nil means no restriction
	HasTask          optional.Option[bool] // false: task_id = 0 (unclaimed); true: task_id != 0 (claimed); nil: no restriction
	UpdatedBefore    timeutil.TimeStamp
	ConcurrencyGroup string
	OrderBy          db.SearchOrderBy
	// AccessibleRepoIDsSubQuery, when non-nil, restricts results to the repo IDs selected by the
	// subquery (the caller's accessible repos). A nil value means no restriction. Using a subquery
	// instead of a materialized ID slice avoids exceeding DB parameter limits for large owners.
	AccessibleRepoIDsSubQuery *builder.Builder
}

var JobOrderByMap = map[string]map[string]db.SearchOrderBy{
	"asc":  {"id": "`action_run_job`.id ASC"},
	"desc": {"id": "`action_run_job`.id DESC"},
}

func (opts FindRunJobOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RunID > 0 {
		cond = cond.And(builder.Eq{"`action_run_job`.run_id": opts.RunID})
	}
	if opts.RunAttemptID.Has() {
		cond = cond.And(builder.Eq{"`action_run_job`.run_attempt_id": opts.RunAttemptID.Value()})
	}
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"`action_run_job`.repo_id": opts.RepoID})
	}
	if opts.CommitSHA != "" {
		cond = cond.And(builder.Eq{"`action_run_job`.commit_sha": opts.CommitSHA})
	}
	if len(opts.Statuses) > 0 {
		cond = cond.And(builder.In("`action_run_job`.status", opts.Statuses))
	}
	if opts.IsReusableCaller.Has() {
		cond = cond.And(builder.Eq{"`action_run_job`.is_reusable_caller": opts.IsReusableCaller.Value()})
	}
	if opts.HasTask.Has() {
		if opts.HasTask.Value() {
			cond = cond.And(builder.Neq{"`action_run_job`.task_id": 0})
		} else {
			cond = cond.And(builder.Eq{"`action_run_job`.task_id": 0})
		}
	}
	if opts.UpdatedBefore > 0 {
		cond = cond.And(builder.Lt{"`action_run_job`.updated": opts.UpdatedBefore})
	}
	if opts.ConcurrencyGroup != "" {
		if opts.RepoID == 0 {
			panic("Invalid FindRunJobOptions: repo_id is required")
		}
		cond = cond.And(builder.Eq{"`action_run_job`.concurrency_group": opts.ConcurrencyGroup})
	}
	if opts.AccessibleRepoIDsSubQuery != nil {
		cond = cond.And(builder.In("`action_run_job`.repo_id", opts.AccessibleRepoIDsSubQuery))
	}
	return cond
}

func (opts FindRunJobOptions) ToJoins() []db.JoinFunc {
	if opts.OwnerID > 0 {
		return []db.JoinFunc{
			func(sess db.Engine) error {
				sess.Join("INNER", "repository", "repository.id = repo_id AND repository.owner_id = ?", opts.OwnerID)
				return nil
			},
		}
	}
	return nil
}

func (opts FindRunJobOptions) ToOrders() string {
	return string(opts.OrderBy)
}

var _ db.FindOptionsOrder = FindRunJobOptions{}
