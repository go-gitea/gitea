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
	// workflow_payload can be large and is never needed when listing tasks
	if err := db.GetEngine(ctx).In("id", jobIDs).Omit("workflow_payload").Find(&jobs); err != nil {
		return err
	}
	for _, t := range tasks {
		if t.JobID > 0 && t.Job == nil {
			t.Job = jobs[t.JobID]
		}
	}

	// Load repos and runs here instead of via ActionJobList.LoadAttributes,
	// so that the large event_payload column of the runs can be skipped.
	jobsList := make(ActionJobList, 0, len(jobs))
	for _, j := range jobs {
		jobsList = append(jobsList, j)
	}
	if err := jobsList.LoadRepos(ctx); err != nil {
		return err
	}
	runIDs := jobsList.GetRunIDs()
	runs := make(map[int64]*ActionRun, len(runIDs))
	if err := db.GetEngine(ctx).In("id", runIDs).Omit("event_payload").Find(&runs); err != nil {
		return err
	}
	for _, j := range jobsList {
		if j.Run == nil {
			j.Run = runs[j.RunID]
		}
		if j.Run != nil {
			j.Run.Repo = j.Repo
		}
	}
	return nil
}

func (tasks TaskList) LoadSteps(ctx context.Context) error {
	taskIDs := container.FilterSlice(tasks, func(task *ActionTask) (int64, bool) {
		return task.ID, task.Steps == nil
	})
	if len(taskIDs) == 0 {
		return nil
	}
	var steps []*ActionTaskStep
	if err := db.GetEngine(ctx).In("task_id", taskIDs).OrderBy("task_id, `index`").Find(&steps); err != nil {
		return err
	}
	stepsByTask := make(map[int64][]*ActionTaskStep, len(taskIDs))
	for _, step := range steps {
		stepsByTask[step.TaskID] = append(stepsByTask[step.TaskID], step)
	}
	for _, task := range tasks {
		if task.Steps == nil {
			task.Steps = util.SliceNilAsEmpty(stepsByTask[task.ID])
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

// FindTasksWithCappedCount finds a page of tasks for list pages. Unlike db.FindAndCount it skips the
// potentially huge log_indexes column, and when maxCount > 0 the returned total never exceeds maxCount,
// so a runner or repository with millions of tasks does not need a full COUNT(*).
func FindTasksWithCappedCount(ctx context.Context, opts FindTaskOptions, maxCount int64) ([]*ActionTask, int64, error) {
	cond := opts.ToConds()
	sess := db.GetEngine(ctx).Where(cond).Omit("log_indexes").OrderBy(opts.ToOrders())
	if !opts.IsListAll() && opts.GetPageSize() > 0 && opts.GetPage() >= 1 {
		sess.Limit(opts.GetPageSize(), (opts.GetPage()-1)*opts.GetPageSize())
	}
	tasks := make([]*ActionTask, 0, max(opts.GetPageSize(), 0))
	if err := sess.Find(&tasks); err != nil {
		return nil, 0, err
	}

	if maxCount <= 0 {
		total, err := db.GetEngine(ctx).Where(cond).Count(new(ActionTask))
		return tasks, total, err
	}
	// a short first page already tells the exact total
	if opts.GetPage() <= 1 && int64(len(tasks)) < int64(opts.GetPageSize()) {
		return tasks, int64(len(tasks)), nil
	}
	// selecting at most maxCount ids is bounded by the limit and portable across databases
	ids := make([]int64, 0, 64)
	if err := db.GetEngine(ctx).Table("action_task").Where(cond).Cols("id").Limit(int(maxCount)).Find(&ids); err != nil {
		return nil, 0, err
	}
	return tasks, int64(len(ids)), nil
}
