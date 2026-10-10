// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	"gitea.dev/models/db"
	"gitea.dev/modules/container"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

// ActionTaskStep represents a step of ActionTask
type ActionTaskStep struct {
	ID        int64
	Name      string `xorm:"VARCHAR(255)"` // the step name, for display purpose only, it will be truncated if it is too long
	TaskID    int64  `xorm:"index unique(task_index)"`
	Index     int64  `xorm:"index unique(task_index)"`
	RepoID    int64  `xorm:"index"`
	Status    Status `xorm:"index"`
	LogIndex  int64
	LogLength int64
	Stage     int64 `xorm:"NOT NULL DEFAULT 0"` // a runnerv1.StepStage, unspecified for the steps parsed from the workflow file instead of reported by the runner
	Number    int64 `xorm:"NOT NULL DEFAULT 0"` // the index in the workflow file of the step a PRE, MAIN or POST stage belongs to
	Started   timeutil.TimeStamp
	Stopped   timeutil.TimeStamp
	Created   timeutil.TimeStamp `xorm:"created"`
	Updated   timeutil.TimeStamp `xorm:"updated"`
}

func (step *ActionTaskStep) Duration() time.Duration {
	return calculateDuration(step.Started, step.Stopped, step.Status, step.Updated)
}

// StepStage returns the stored stage, which was validated against the enum when the runner reported it
func (step *ActionTaskStep) StepStage() runnerv1.StepStage {
	return runnerv1.StepStage(step.Stage)
}

// IsReported reports whether the runner reported the step instead of Gitea parsing it from the workflow file.
func (step *ActionTaskStep) IsReported() bool {
	return step.StepStage() != runnerv1.StepStage_STEP_STAGE_UNSPECIFIED
}

// WorkflowStepIndex returns the index of the step in the workflow file, or -1 for a step that is not written there.
func (step *ActionTaskStep) WorkflowStepIndex() int64 {
	switch step.StepStage() {
	case runnerv1.StepStage_STEP_STAGE_UNSPECIFIED:
		return step.Index
	case runnerv1.StepStage_STEP_STAGE_MAIN:
		return step.Number
	}
	return -1
}

func init() {
	db.RegisterModel(new(ActionTaskStep))
}

func GetTaskStepsByTaskID(ctx context.Context, taskID int64) ([]*ActionTaskStep, error) {
	var steps []*ActionTaskStep
	return steps, db.GetEngine(ctx).Where("task_id=?", taskID).OrderBy("`index` ASC").Find(&steps)
}

// applyState applies the state the runner reported for the step, nil meaning it reported none.
func (step *ActionTaskStep) applyState(state *runnerv1.StepState, now timeutil.TimeStamp) {
	var result runnerv1.Result
	if state != nil {
		result = state.Result
		step.LogIndex = state.LogIndex
		step.LogLength = state.LogLength
		if step.Started == 0 && state.StartedAt != nil {
			step.Started = now
		}
	}
	if result != runnerv1.Result_RESULT_UNSPECIFIED {
		step.Status = StatusFromResult(result)
		step.Stopped = util.IfZero(step.Stopped, now)
	} else if step.Started != 0 {
		step.Status = StatusRunning
	}
}

// maxReportedSteps bounds the steps a runner can make Gitea store for one task.
const maxReportedSteps = 1000

type reportedStepKey struct {
	stage  runnerv1.StepStage
	number int64
}

func newReportedStepKey(stage runnerv1.StepStage, number int64) reportedStepKey {
	switch stage {
	case runnerv1.StepStage_STEP_STAGE_PRE, runnerv1.StepStage_STEP_STAGE_MAIN, runnerv1.StepStage_STEP_STAGE_POST:
		return reportedStepKey{stage: stage, number: number}
	}
	return reportedStepKey{stage: stage} // the other stages appear once and ignore the number
}

func validateReportedSteps(steps []*runnerv1.StepState) error {
	if len(steps) > maxReportedSteps {
		return fmt.Errorf("%d steps exceed the limit of %d", len(steps), maxReportedSteps)
	}
	seen := make(container.Set[reportedStepKey], len(steps))
	for i, v := range steps {
		if _, ok := runnerv1.StepStage_name[int32(v.Stage)]; !ok || v.Stage == runnerv1.StepStage_STEP_STAGE_UNSPECIFIED {
			return fmt.Errorf("step %d has invalid stage %d", i, v.Stage)
		}
		if v.Id != int64(i) || v.Number < 0 || v.LogIndex < 0 || v.LogLength < 0 {
			return fmt.Errorf("step %d has invalid id %d, number %d or log range %d+%d", i, v.Id, v.Number, v.LogIndex, v.LogLength)
		}
		if !seen.Add(newReportedStepKey(v.Stage, v.Number)) {
			return fmt.Errorf("step %d duplicates stage %s of number %d", i, v.Stage, v.Number)
		}
	}
	return nil
}

// updateReportedSteps replaces the steps of the task with the ones the runner reported.
// A step keeps its times and status when the runner reports it again at another position.
func updateReportedSteps(ctx context.Context, task *ActionTask, reported []*runnerv1.StepState, now timeutil.TimeStamp) error {
	e := db.GetEngine(ctx)
	sameLayout := len(task.Steps) == len(reported)
	for i := 0; sameLayout && i < len(reported); i++ {
		step := task.Steps[i]
		sameLayout = step.IsReported() && newReportedStepKey(step.StepStage(), step.Number) == newReportedStepKey(reported[i].Stage, reported[i].Number)
	}

	if sameLayout {
		for i, step := range task.Steps {
			step.Name = util.EllipsisDisplayString(reported[i].Name, 255)
			step.applyState(reported[i], now)
			if _, err := e.ID(step.ID).Update(step); err != nil {
				return err
			}
		}
		return nil
	}

	known := make(map[reportedStepKey]*ActionTaskStep, len(task.Steps))
	for _, step := range task.Steps {
		if step.IsReported() {
			known[newReportedStepKey(step.StepStage(), step.Number)] = step
		}
	}
	steps := make([]*ActionTaskStep, len(reported))
	for i, v := range reported {
		key := newReportedStepKey(v.Stage, v.Number)
		step := &ActionTaskStep{
			Name:   util.EllipsisDisplayString(v.Name, 255),
			TaskID: task.ID,
			Index:  int64(i),
			RepoID: task.RepoID,
			Stage:  int64(key.stage),
			Number: key.number,
			Status: StatusWaiting,
		}
		if old, ok := known[key]; ok {
			step.Status, step.Started, step.Stopped = old.Status, old.Started, old.Stopped
		}
		step.applyState(v, now)
		steps[i] = step
	}
	if _, err := e.Delete(&ActionTaskStep{TaskID: task.ID}); err != nil {
		return err
	}
	if _, err := e.Insert(steps); err != nil {
		return err
	}
	task.Steps = steps
	return nil
}
