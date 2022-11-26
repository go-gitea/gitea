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

func (tasks TaskList) GetJobIDs() []int64 {
	jobIDsMap := make(map[int64]struct{})
	for _, t := range tasks {
		if t.JobID == 0 {
			continue
		}
		jobIDsMap[t.JobID] = struct{}{}
	}
	jobIDs := make([]int64, 0, len(jobIDsMap))
	for jobID := range jobIDsMap {
		jobIDs = append(jobIDs, jobID)
	}
	return jobIDs
}

func (tasks TaskList) LoadJobs(ctx context.Context) error {
	jobIDs := tasks.GetJobIDs()
	jobs := make(map[int64]*RunJob, len(jobIDs))
	if err := db.GetEngine(ctx).In("id", jobIDs).Find(&jobs); err != nil {
		return err
	}
	for _, t := range tasks {
		if t.JobID > 0 && t.Job == nil {
			t.Job = jobs[t.JobID]
		}
	}

	var jobsList RunJobList = make([]*RunJob, 0, len(jobs))
	for _, j := range jobs {
		jobsList = append(jobsList, j)
	}
	return jobsList.LoadAttributes(ctx, true)
}

func (tasks TaskList) LoadAttributes(ctx context.Context) error {
	return tasks.LoadJobs(ctx)
}

type FindTaskOptions struct {
	db.ListOptions
	RepoID        int64
	OwnerID       int64
	CommitSHA     string
	Status        Status
	UpdatedBefore timeutil.TimeStamp
	StartedBefore timeutil.TimeStamp
	RunnerID      int64
	IDOrderDesc   bool
}

func (opts FindTaskOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.CommitSHA != "" {
		cond = cond.And(builder.Eq{"commit_sha": opts.CommitSHA})
	}
	if opts.Status > StatusUnknown {
		cond = cond.And(builder.Eq{"status": opts.Status})
	}
	if opts.UpdatedBefore > 0 {
		cond = cond.And(builder.Lt{"updated": opts.UpdatedBefore})
	}
	if opts.StartedBefore > 0 {
		cond = cond.And(builder.Lt{"started": opts.StartedBefore})
	}
	if opts.RunnerID > 0 {
		cond = cond.And(builder.Eq{"runner_id": opts.RunnerID})
	}
	return cond
}

func FindTasks(ctx context.Context, opts FindTaskOptions) (TaskList, int64, error) {
	e := db.GetEngine(ctx).Where(opts.toConds())
	if opts.PageSize > 0 && opts.Page >= 1 {
		e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	if opts.IDOrderDesc {
		e.OrderBy("id DESC")
	}
	var tasks TaskList
	total, err := e.FindAndCount(&tasks)
	return tasks, total, err
}

func CountTasks(ctx context.Context, opts FindTaskOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds()).Count(new(Task))
}
