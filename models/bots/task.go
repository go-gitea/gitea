// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// Task represents a distribution of job
type Task struct {
	ID        int64
	JobID     int64
	Attempt   int64
	RunnerID  int64  `xorm:"index"`
	LogToFile bool   // read log from database or from storage
	LogUrl    string // url of the log file in storage
	Result    int64  // TODO: use runnerv1.Result
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

func CreateTask(runner *Runner) (task *Task, job *RunJob, run *Run, ok bool, err error) {
	ctx, commiter, err := db.TxContext()
	if err != nil {
		return
	}
	defer commiter.Close()

	var jobs []*RunJob
	if err = db.GetEngine(ctx).Where("task_id = 0 AND ready = true").OrderBy("id").Find(jobs); err != nil {
		return
	}

	labels := append(runner.AgentLabels, runner.CustomLabels...)
	for _, v := range jobs {
		if isSubset(v.RunsOn, labels) {
			job = v
			break
		}
	}
	if job == nil {
		return
	}

	now := timeutil.TimeStampNow()
	job.Attempt++
	job.Started = now
	job.Status = core.StatusRunning

	task = &Task{
		JobID:    job.ID,
		Attempt:  job.Attempt,
		RunnerID: runner.ID,
		Started:  now,
	}

	if err = db.Insert(ctx, task); err != nil {
		return
	}

	job.TaskID = task.ID
	if _, err = db.GetEngine(ctx).ID(job.ID).Update(job); err != nil {
		return
	}

	run = &Run{}
	if _, err = db.GetEngine(ctx).ID(job.RunID).Get(run); err != nil {
		return
	}

	if err = commiter.Commit(); err != nil {
		return
	}

	ok = true
	return
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
