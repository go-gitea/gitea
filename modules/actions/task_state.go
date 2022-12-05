// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	bots_model "code.gitea.io/gitea/models/actions"
)

const (
	preStepName  = "Set up job"
	postStepName = "Complete job"
)

// FullSteps returns steps with "Set up job" and "Complete job"
func FullSteps(task *bots_model.BotTask) []*bots_model.BotTaskStep {
	if len(task.Steps) == 0 {
		return fullStepsOfEmptySteps(task)
	}

	firstStep := task.Steps[0]
	var logIndex int64

	preStep := &bots_model.BotTaskStep{
		Name:      preStepName,
		LogLength: task.LogLength,
		Started:   task.Started,
		Status:    bots_model.StatusRunning,
	}

	if firstStep.Status.HasRun() || firstStep.Status.IsRunning() {
		preStep.LogLength = firstStep.LogIndex
		preStep.Stopped = firstStep.Started
		preStep.Status = bots_model.StatusSuccess
	} else if task.Status.IsDone() {
		preStep.Stopped = task.Stopped
		preStep.Status = bots_model.StatusFailure
	}
	logIndex += preStep.LogLength

	var lastHasRunStep *bots_model.BotTaskStep
	for _, step := range task.Steps {
		if step.Status.HasRun() {
			lastHasRunStep = step
		}
		logIndex += step.LogLength
	}
	if lastHasRunStep == nil {
		lastHasRunStep = preStep
	}

	postStep := &bots_model.BotTaskStep{
		Name:   postStepName,
		Status: bots_model.StatusWaiting,
	}
	if task.Status.IsDone() {
		postStep.LogIndex = logIndex
		postStep.LogLength = task.LogLength - postStep.LogIndex
		postStep.Status = task.Status
		postStep.Started = lastHasRunStep.Stopped
		postStep.Stopped = task.Stopped
	}
	ret := make([]*bots_model.BotTaskStep, 0, len(task.Steps)+2)
	ret = append(ret, preStep)
	ret = append(ret, task.Steps...)
	ret = append(ret, postStep)

	return ret
}

func fullStepsOfEmptySteps(task *bots_model.BotTask) []*bots_model.BotTaskStep {
	preStep := &bots_model.BotTaskStep{
		Name:      preStepName,
		LogLength: task.LogLength,
		Started:   task.Started,
		Stopped:   task.Stopped,
		Status:    bots_model.StatusRunning,
	}

	postStep := &bots_model.BotTaskStep{
		Name:     postStepName,
		LogIndex: task.LogLength,
		Started:  task.Stopped,
		Stopped:  task.Stopped,
		Status:   bots_model.StatusWaiting,
	}

	if task.Status.IsDone() {
		preStep.Status = task.Status
		if preStep.Status.IsSuccess() {
			postStep.Status = bots_model.StatusSuccess
		} else {
			postStep.Status = bots_model.StatusCancelled
		}
	}

	return []*bots_model.BotTaskStep{
		preStep,
		postStep,
	}
}
