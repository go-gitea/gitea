// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

type SpecList []*ActionScheduleSpec

func (specs SpecList) GetScheduleIDs() []int64 {
	ids := make(container.Set[int64], len(specs))
	for _, spec := range specs {
		ids.Add(spec.ScheduleID)
	}
	return ids.Values()
}

func (specs SpecList) LoadSchedules(ctx context.Context) error {
	scheduleIDs := specs.GetScheduleIDs()
	schedules, err := GetSchedulesMapByIDs(ctx, scheduleIDs)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		spec.Schedule = schedules[spec.ScheduleID]
	}

	repoIDs := specs.GetRepoIDs()
	repos, err := GetReposMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		spec.Repo = repos[spec.RepoID]
	}

	return nil
}

func (specs SpecList) GetRepoIDs() []int64 {
	ids := make(container.Set[int64], len(specs))
	for _, spec := range specs {
		ids.Add(spec.RepoID)
	}
	return ids.Values()
}

func (specs SpecList) LoadRepos(ctx context.Context) error {
	repoIDs := specs.GetRepoIDs()
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		spec.Repo = repos[spec.RepoID]
	}
	return nil
}

type FindSpecOptions struct {
	db.ListOptions
	RepoID int64
	Next   int64
}

func (opts FindSpecOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	if opts.Next > 0 {
		cond = cond.And(builder.Lte{"next": opts.Next})
	}

	return cond
}

func FindSpecs(ctx context.Context, opts FindSpecOptions) (SpecList, int64, error) {
	e := db.GetEngine(ctx).Where(opts.toConds())
	if opts.PageSize > 0 && opts.Page >= 1 {
		e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	var specs SpecList
	total, err := e.Desc("id").FindAndCount(&specs)
	if err != nil {
		return nil, 0, err
	}

	if err := specs.LoadSchedules(ctx); err != nil {
		return nil, 0, err
	}
	return specs, total, nil
}

func CountSpecs(ctx context.Context, opts FindSpecOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds()).Count(new(ActionScheduleSpec))
}
