// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/nektos/act/pkg/jobparser"
)

// Task represents a distribution of job
type Task struct {
	ID       int64
	JobID    int64
	Job      *RunJob     `xorm:"-"`
	Steps    []*TaskStep `xorm:"-"`
	Attempt  int64
	RunnerID int64 `xorm:"index"`
	Result   runnerv1.Result
	Started  timeutil.TimeStamp
	Stopped  timeutil.TimeStamp

	LogURL     string      // dbfs:///a/b.log or s3://endpoint.com/a/b.log and etc.
	LogLength  int64       // lines count
	LogSize    int64       // blob size
	LogIndexes *LogIndexes `xorm:"BLOB"` // line number to offset
	LogExpired bool

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
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

// FullSteps returns steps with "Set up job" and "Complete job"
func (task *Task) FullSteps() []*TaskStep {
	var firstStep, lastStep *TaskStep
	if l := len(task.Steps); l > 0 {
		firstStep = task.Steps[0]
		lastStep = task.Steps[l-1]
	}
	headStep := &TaskStep{
		Name:      "Set up job",
		LogIndex:  0,
		LogLength: task.LogLength,
		Started:   task.Started,
	}
	if firstStep != nil {
		headStep.LogLength = firstStep.LogIndex
		headStep.Stopped = firstStep.Started
	}
	tailStep := &TaskStep{
		Name:    "Complete job",
		Stopped: task.Stopped,
	}
	if lastStep != nil {
		tailStep.LogIndex = lastStep.LogIndex + lastStep.LogLength
		tailStep.LogLength = task.LogLength - tailStep.LogIndex
		tailStep.Started = lastStep.Stopped
	}
	steps := make([]*TaskStep, 0, len(task.Steps)+2)
	steps = append(steps, headStep)
	steps = append(steps, task.Steps...)
	steps = append(steps, tailStep)

	return steps
}

type LogIndexes []int64

func (i *LogIndexes) FromDB(b []byte) error {
	reader := bytes.NewReader(b)
	for {
		v, err := binary.ReadVarint(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("binary ReadVarint: %w", err)
		}
		*i = append(*i, v)
	}
}

func (i *LogIndexes) ToDB() ([]byte, error) {
	var buf []byte
	for _, v := range *i {
		buf = binary.AppendVarint(buf, v)
	}
	return buf, nil
}

// ErrTaskNotExist represents an error for bot task not exist
type ErrTaskNotExist struct {
	ID int64
}

func (err ErrTaskNotExist) Error() string {
	return fmt.Sprintf("task [%d] is not exist", err.ID)
}

func GetTaskByID(ctx context.Context, id int64) (*Task, error) {
	var task Task
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTaskNotExist{
			ID: id,
		}
	}

	return &task, nil
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
	log.Trace("runner labels: %v", labels)
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

func UpdateTask(ctx context.Context, task *Task, cols ...string) error {
	sess := db.GetEngine(ctx).ID(task.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	_, err := sess.Update(task)
	return err
}

func UpdateTaskByState(state *runnerv1.TaskState) error {
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
