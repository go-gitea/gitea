// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/nektos/act/pkg/jobparser"
)

// Task represents a distribution of job
type Task struct {
	ID        int64
	JobID     int64
	Job       *RunJob     `xorm:"-"`
	Steps     []*TaskStep `xorm:"-"`
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

// LoadAttributes load Job Steps if not loaded
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
	if err := task.Job.LoadAttributes(ctx); err != nil {
		return err
	}

	if task.Steps == nil { // be careful, an empty slice (not nil) also means loaded
		steps, err := GetTaskStepsByTaskID(ctx, task.ID)
		if err != nil {
			return err
		}
		task.Steps = steps
	}

	return nil
}

func CreateTaskForRunner(runner *Runner) (*Task, bool, error) {
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
	labels := append(runner.AgentLabels, runner.CustomLabels...)
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

	var wolkflowJob *jobparser.Job
	if gots, err := jobparser.Parse(job.WorkflowPayload); err != nil {
		return nil, false, fmt.Errorf("parse workflow of job %d: %w", job.ID, err)
	} else if len(gots) != 1 {
		return nil, false, fmt.Errorf("workflow of job %d: not signle workflow", job.ID)
	} else {
		_, wolkflowJob = gots[0].Job()
	}

	if err := db.Insert(ctx, task); err != nil {
		return nil, false, err
	}

	steps := make([]*TaskStep, len(wolkflowJob.Steps))
	for i, v := range wolkflowJob.Steps {
		steps[i] = &TaskStep{
			Name:   v.String(),
			TaskID: task.ID,
			Number: int64(i),
		}
	}
	if err := db.Insert(ctx, steps); err != nil {
		return nil, false, err
	}
	task.Steps = steps

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

func UpdateTask(state *runnerv1.TaskState) error {
	stepStates := map[int64]*runnerv1.StepState{}
	for _, v := range state.Steps {
		stepStates[v.Id] = v
	}

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	task := &Task{}
	if _, err := db.GetEngine(ctx).ID(state.Id).Get(task); err != nil {
		return err
	}

	task.Result = state.Result
	task.Stopped = timeutil.TimeStamp(state.StoppedAt.AsTime().Unix())

	if _, err := db.GetEngine(ctx).ID(task.ID).Update(task); err != nil {
		return err
	}

	if err := task.LoadAttributes(ctx); err != nil {
		return err
	}

	for _, step := range task.Steps {
		if v, ok := stepStates[step.Number]; ok {
			step.Result = v.Result
			step.LogIndex = v.LogIndex
			step.LogLength = v.LogLength
			if _, err := db.GetEngine(ctx).ID(step.ID).Update(step); err != nil {
				return err
			}
		}
	}

	if err := commiter.Commit(); err != nil {
		return err
	}

	return nil
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
