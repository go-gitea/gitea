// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"

	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

// Task represents a distribution of job
type Task struct {
	ID        int64
	JobID     int64
	Job       *RunJob `xorm:"-"`
	Attempt   int64
	RunnerID  int64  `xorm:"index"`
	LogToFile bool   // read log from database or from storage
	LogURL    string // url of the log file in storage
	Result    runnerv1.Result
	Started   timeutil.TimeStamp
	Stopped   timeutil.TimeStamp
	Created   timeutil.TimeStamp `xorm:"created"`
	Updated   timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(Task))
}

func (Task) TableName() string {
	return "bots_task"
}

// LoadAttributes load Job if not loaded
func (task *Task) LoadAttributes(ctx context.Context) error {
	if task == nil {
		return nil
	}

	if task.Job == nil {
		job, err := GetRunJobByID(ctx, task.JobID)
		if err != nil {
			return err
		}
		task.Job = job
	}

	return task.Job.LoadAttributes(ctx)
}

func CreateTask(runner *Runner) (*Task, bool, error) {
	ctx, commiter, err := db.TxContext()
	if err != nil {
		return nil, false, err
	}
	defer commiter.Close()

	var jobs []*RunJob
	if err := db.GetEngine(ctx).Where("task_id=? AND ready=?", 0, true).OrderBy("id").Find(&jobs); err != nil {
		return nil, false, err
	}

	// TODO: a more efficient way to filter labels
	var job *RunJob
	labels := append([]string{}, append(runner.AgentLabels, runner.CustomLabels...)...)
	for _, v := range jobs {
		if isSubset(labels, v.RunsOn) {
			job = v
			break
		}
	}
	if job == nil {
		return nil, false, nil
	}

	now := timeutil.TimeStampNow()
	job.Attempt++
	job.Started = now
	job.Status = core.StatusRunning

	task := &Task{
		JobID:    job.ID,
		Attempt:  job.Attempt,
		RunnerID: runner.ID,
		Started:  now,
	}

	if err := db.Insert(ctx, task); err != nil {
		return nil, false, err
	}

	job.TaskID = task.ID
	if _, err := db.GetEngine(ctx).ID(job.ID).Update(job); err != nil {
		return nil, false, err
	}

	task.Job = job
	if err := task.Job.LoadAttributes(ctx); err != nil {
		return nil, false, err
	}

	if err := commiter.Commit(); err != nil {
		return nil, false, err
	}

	return task, true, nil
}

func isSubset(set, subset []string) bool {
	m := make(map[string]struct{}, len(set))
	for _, v := range set {
		m[v] = struct{}{}
	}
	for _, v := range subset {
		if _, ok := m[v]; !ok {
			return false
		}
	}
	return true
}
