// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

type ActionJobList []*ActionRunJob

func (jobs ActionJobList) GetRunIDs() []int64 {
	return container.FilterSlice(jobs, func(j *ActionRunJob) (int64, bool) {
		return j.RunID, j.RunID != 0
	})
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
		if j.RunID > 0 && j.Run == nil {
			j.Run = runs[j.RunID]
			j.Run.Repo = j.Repo
		}
	}
	return nil
}

func (jobs ActionJobList) LoadAttributes(ctx context.Context, withRepo bool) error {
	return jobs.LoadRuns(ctx, withRepo)
}

type FindRunJobOptions struct {
	db.ListOptions
	RunID         int64
	RepoID        int64
	OwnerID       int64
	CommitSHA     string
	Statuses      []Status
	UpdatedBefore timeutil.TimeStamp
}

func (opts FindRunJobOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RunID > 0 {
		cond = cond.And(builder.Eq{"run_id": opts.RunID})
	}
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.CommitSHA != "" {
		cond = cond.And(builder.Eq{"commit_sha": opts.CommitSHA})
	}
	if len(opts.Statuses) > 0 {
		cond = cond.And(builder.In("status", opts.Statuses))
	}
	if opts.UpdatedBefore > 0 {
		cond = cond.And(builder.Lt{"updated": opts.UpdatedBefore})
	}
	return cond
}
