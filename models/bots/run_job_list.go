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

type FindRunJobOptions struct {
	db.ListOptions
	Status        Status
	StartedBefore timeutil.TimeStamp
}

func (opts FindRunJobOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.Status > StatusUnknown {
		cond = cond.And(builder.Eq{"status": opts.Status})
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
