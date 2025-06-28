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
	return container.FilterSlice(specs, func(spec *ActionScheduleSpec) (int64, bool) {
		return spec.ScheduleID, true
	})
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
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		spec.Repo = repos[spec.RepoID]
	}

	return nil
}

func (specs SpecList) GetRepoIDs() []int64 {
	return container.FilterSlice(specs, func(spec *ActionScheduleSpec) (int64, bool) {
		return spec.RepoID, true
	})
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

func (opts FindSpecOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	if opts.Next > 0 {
		cond = cond.And(builder.Lte{"next": opts.Next})
	}

	return cond
}

func (opts FindSpecOptions) ToOrders() string {
	return "`id` DESC"
}

func FindSpecs(ctx context.Context, opts FindSpecOptions) (SpecList, int64, error) {
	specs, total, err := db.FindAndCount[ActionScheduleSpec](ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	if err := SpecList(specs).LoadSchedules(ctx); err != nil {
		return nil, 0, err
	}
	return specs, total, nil
}
