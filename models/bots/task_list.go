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

type TaskList []*Task

type FindTaskOptions struct {
	db.ListOptions
	Status        Status
	UpdatedBefore timeutil.TimeStamp
	StartedBefore timeutil.TimeStamp
}

func (opts FindTaskOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.Status > StatusUnknown {
		cond = cond.And(builder.Eq{"status": opts.Status})
	}
	if opts.UpdatedBefore > 0 {
		cond = cond.And(builder.Lt{"updated": opts.UpdatedBefore})
	}
	if opts.StartedBefore > 0 {
		cond = cond.And(builder.Lt{"started": opts.StartedBefore})
	}
	return cond
}

func FindTasks(ctx context.Context, opts FindTaskOptions) (TaskList, int64, error) {
	e := db.GetEngine(ctx).Where(opts.toConds())
	if opts.PageSize > 0 && opts.Page >= 1 {
		e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	var tasks TaskList
	total, err := e.FindAndCount(&tasks)
	return tasks, total, err
}

func CountTasks(ctx context.Context, opts FindTaskOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds()).Count(new(Task))
}
