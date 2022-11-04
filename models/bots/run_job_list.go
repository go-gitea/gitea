// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/builder"
)

type RunJobList []*RunJob

func (jobs RunJobList) GetRunIDs() []int64 {
	var runIDsMap = make(map[int64]struct{})
	for _, j := range jobs {
		if j.RunID == 0 {
			continue
		}
		runIDsMap[j.RunID] = struct{}{}
	}
	var runIDs = make([]int64, 0, len(runIDsMap))
	for runID := range runIDsMap {
		runIDs = append(runIDs, runID)
	}
	return runIDs
}

func (jobs RunJobList) LoadRuns(ctx context.Context, withRepo bool) error {
	runIDs := jobs.GetRunIDs()
	runs := make(map[int64]*Run, len(runIDs))
	if err := db.GetEngine(ctx).In("id", runIDs).Find(&runs); err != nil {
		return err
	}
	for _, j := range jobs {
		if j.RunID > 0 && j.Run == nil {
			j.Run = runs[j.RunID]
		}
	}
	if withRepo {
		var runsList RunList = make([]*Run, 0, len(runs))
		for _, r := range runs {
			runsList = append(runsList, r)
		}
		return runsList.LoadRepos()
	}
	return nil
}

func (jobs RunJobList) LoadAttributes(ctx context.Context, withRepo bool) error {
	if err := jobs.LoadRuns(ctx, withRepo); err != nil {
		return err
	}
	return nil
}

type FindRunJobOptions struct {
	db.ListOptions
	RunID         int64
	Statuses      []Status
	StartedBefore timeutil.TimeStamp
}

func (opts FindRunJobOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RunID > 0 {
		cond = cond.And(builder.Eq{"run_id": opts.RunID})
	}
	if len(opts.Statuses) > 0 {
		cond = cond.And(builder.In("status", opts.Statuses))
	}
	if opts.StartedBefore > 0 {
		cond = cond.And(builder.Lt{"started": opts.StartedBefore})
	}
	return cond
}

func FindRunJobs(ctx context.Context, opts FindRunJobOptions) (RunJobList, int64, error) {
	e := db.GetEngine(ctx).Where(opts.toConds())
	if opts.PageSize > 0 && opts.Page >= 1 {
		e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	var tasks RunJobList
	total, err := e.FindAndCount(&tasks)
	return tasks, total, err
}

func CountRunJobs(ctx context.Context, opts FindRunJobOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds()).Count(new(RunJob))
}
