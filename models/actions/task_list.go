// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/container"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type TaskList []*ActionTask

func (tasks TaskList) GetJobIDs() []int64 {
	return container.FilterSlice(tasks, func(t *ActionTask) (int64, bool) {
		return t.JobID, t.JobID != 0
	})
}

func (tasks TaskList) LoadJobs(ctx context.Context) error {
	jobIDs := tasks.GetJobIDs()
	jobs := make(map[int64]*ActionRunJob, len(jobIDs))
	if err := db.GetEngine(ctx).In("id", jobIDs).Find(&jobs); err != nil {
		return err
	}
	for _, t := range tasks {
		if t.JobID > 0 && t.Job == nil {
			t.Job = jobs[t.JobID]
		}
	}

	// TODO: Replace with "ActionJobList(maps.Values(jobs))" once available
	var jobsList ActionJobList = make([]*ActionRunJob, 0, len(jobs))
	for _, j := range jobs {
		jobsList = append(jobsList, j)
	}
	return jobsList.LoadAttributes(ctx, true)
}

// LoadSteps loads the steps of the tasks whose steps are not loaded yet
func (tasks TaskList) LoadSteps(ctx context.Context) error {
	taskIDs := container.FilterSlice(tasks, func(t *ActionTask) (int64, bool) {
		return t.ID, t.Steps == nil
	})
	if len(taskIDs) == 0 {
		return nil
	}
	var steps []*ActionTaskStep
	if err := db.GetEngine(ctx).In("task_id", taskIDs).OrderBy("`index` ASC").Find(&steps); err != nil {
		return err
	}
	stepsByTask := make(map[int64][]*ActionTaskStep, len(taskIDs))
	for _, step := range steps {
		stepsByTask[step.TaskID] = append(stepsByTask[step.TaskID], step)
	}
	for _, t := range tasks {
		if t.Steps == nil {
			t.Steps = util.SliceNilAsEmpty(stepsByTask[t.ID])
		}
	}
	return nil
}

func (tasks TaskList) LoadAttributes(ctx context.Context) error {
	return tasks.LoadJobs(ctx)
}

type FindTaskOptions struct {
	db.ListOptions
	RepoID        int64
	JobID         int64
	OwnerID       int64
	CommitSHA     string
	Status        Status
	UpdatedBefore timeutil.TimeStamp
	StartedBefore timeutil.TimeStamp
	RunnerID      int64
}

func (opts FindTaskOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.JobID > 0 {
		cond = cond.And(builder.Eq{"job_id": opts.JobID})
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

func (opts FindTaskOptions) ToOrders() string {
	return "`id` DESC"
}
