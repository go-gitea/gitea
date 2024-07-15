// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	actions_model "code.gitea.io/gitea/models/actions"
)

const (
	preStepName  = "Set up job"
	postStepName = "Complete job"
)

// FullSteps returns steps with "Set up job" and "Complete job"
func FullSteps(task *actions_model.ActionTask) []*actions_model.ActionTaskStep {
	if len(task.Steps) == 0 {
		return fullStepsOfEmptySteps(task)
	}

	firstStep := task.Steps[0]
	var logIndex int64

	preStep := &actions_model.ActionTaskStep{
		Name:      preStepName,
		LogLength: task.LogLength,
		Started:   task.Started,
		Status:    actions_model.StatusRunning,
	}

	if firstStep.Status.HasRun() || firstStep.Status.IsRunning() {
		preStep.LogLength = firstStep.LogIndex
		preStep.Stopped = firstStep.Started
		preStep.Status = actions_model.StatusSuccess
	} else if task.Status.IsDone() {
		preStep.Stopped = task.Stopped
		preStep.Status = actions_model.StatusFailure
		if task.Status.IsSkipped() {
			preStep.Status = actions_model.StatusSkipped
		}
	}
	logIndex += preStep.LogLength

	// lastHasRunStep is the last step that has run.
	// For example,
	// 1. preStep(Success) -> step1(Success) -> step2(Running) -> step3(Waiting) -> postStep(Waiting): lastHasRunStep is step1.
	// 2. preStep(Success) -> step1(Success) -> step2(Success) -> step3(Success) -> postStep(Success): lastHasRunStep is step3.
	// 3. preStep(Success) -> step1(Success) -> step2(Failure) -> step3 -> postStep(Waiting): lastHasRunStep is step2.
	// So its Stopped is the Started of postStep when there are no more steps to run.
	var lastHasRunStep *actions_model.ActionTaskStep
	for _, step := range task.Steps {
		if step.Status.HasRun() {
			lastHasRunStep = step
		}
		logIndex += step.LogLength
	}
	if lastHasRunStep == nil {
		lastHasRunStep = preStep
	}

	postStep := &actions_model.ActionTaskStep{
		Name:   postStepName,
		Status: actions_model.StatusWaiting,
	}
	// If the lastHasRunStep is the last step, or it has failed, postStep has started.
	if lastHasRunStep.Status.IsFailure() || lastHasRunStep == task.Steps[len(task.Steps)-1] {
		postStep.LogIndex = logIndex
		postStep.LogLength = task.LogLength - postStep.LogIndex
		postStep.Started = lastHasRunStep.Stopped
		postStep.Status = actions_model.StatusRunning
	}
	if task.Status.IsDone() {
		postStep.Status = task.Status
		postStep.Stopped = task.Stopped
	}
	ret := make([]*actions_model.ActionTaskStep, 0, len(task.Steps)+2)
	ret = append(ret, preStep)
	ret = append(ret, task.Steps...)
	ret = append(ret, postStep)

	return ret
}

func fullStepsOfEmptySteps(task *actions_model.ActionTask) []*actions_model.ActionTaskStep {
	preStep := &actions_model.ActionTaskStep{
		Name:      preStepName,
		LogLength: task.LogLength,
		Started:   task.Started,
		Stopped:   task.Stopped,
		Status:    actions_model.StatusRunning,
	}

	postStep := &actions_model.ActionTaskStep{
		Name:     postStepName,
		LogIndex: task.LogLength,
		Started:  task.Stopped,
		Stopped:  task.Stopped,
		Status:   actions_model.StatusWaiting,
	}

	if task.Status.IsDone() {
		preStep.Status = task.Status
		if preStep.Status.IsSuccess() {
			postStep.Status = actions_model.StatusSuccess
		} else {
			postStep.Status = actions_model.StatusCancelled
		}
	}

	return []*actions_model.ActionTaskStep{
		preStep,
		postStep,
	}
}
