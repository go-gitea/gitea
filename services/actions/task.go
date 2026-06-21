// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"sync"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	secret_model "gitea.dev/models/secret"
	"gitea.dev/modules/setting"

	"google.golang.org/protobuf/types/known/structpb"
)

var (
	taskPickSem     chan struct{}
	taskPickSemOnce sync.Once
)

func taskPickLimiter() chan struct{} {
	taskPickSemOnce.Do(func() {
		taskPickSem = make(chan struct{}, setting.Actions.MaxConcurrentTaskPicks)
	})
	return taskPickSem
}

// TryPickTask attempts to assign a task to the runner, bounding the number of
// concurrent assignment transactions to avoid a thundering herd when many
// runners poll at once. When the concurrency limit is reached it returns
// throttled=true without touching the DB, so the caller can let the runner
// retry on its next poll instead of advancing its tasks version.
func TryPickTask(ctx context.Context, runner *actions_model.ActionRunner) (task *runnerv1.Task, ok, throttled bool, err error) {
	sem := taskPickLimiter()
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	default:
		return nil, false, true, nil
	}
	task, ok, err = PickTask(ctx, runner)
	return task, ok, false, err
}

func PickTask(ctx context.Context, runner *actions_model.ActionRunner) (*runnerv1.Task, bool, error) {
	var (
		task       *runnerv1.Task
		job        *actions_model.ActionRunJob
		actionTask *actions_model.ActionTask
	)

	if runner.IsDisabled {
		return nil, false, nil
	}

	if runner.Ephemeral {
		var task actions_model.ActionTask
		has, err := db.GetEngine(ctx).Where("runner_id = ?", runner.ID).Get(&task)
		// Let the runner retry the request, do not allow to proceed
		if err != nil {
			return nil, false, err
		}
		if has {
			if task.Status.In(actions_model.StatusWaiting, actions_model.StatusRunning, actions_model.StatusBlocked, actions_model.StatusCancelling) {
				return nil, false, nil
			}
			// task has been finished, remove it
			_, err = db.DeleteByID[actions_model.ActionRunner](ctx, runner.ID)
			if err != nil {
				return nil, false, err
			}
			return nil, false, errors.New("runner has been removed")
		}
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		t, ok, err := actions_model.CreateTaskForRunner(ctx, runner)
		if err != nil {
			return fmt.Errorf("CreateTaskForRunner: %w", err)
		}
		if !ok {
			return nil
		}

		if err := t.LoadAttributes(ctx); err != nil {
			return fmt.Errorf("task LoadAttributes: %w", err)
		}
		job = t.Job
		actionTask = t

		secrets, err := secret_model.GetSecretsOfTask(ctx, t)
		if err != nil {
			return fmt.Errorf("GetSecretsOfTask: %w", err)
		}

		vars, err := actions_model.GetVariablesOfRun(ctx, t.Job.Run)
		if err != nil {
			return fmt.Errorf("GetVariablesOfRun: %w", err)
		}

		needs, err := findTaskNeeds(ctx, job)
		if err != nil {
			return fmt.Errorf("findTaskNeeds: %w", err)
		}

		taskContext, err := generateTaskContext(ctx, t)
		if err != nil {
			return fmt.Errorf("generateTaskContext: %w", err)
		}

		task = &runnerv1.Task{
			Id:              t.ID,
			WorkflowPayload: t.Job.WorkflowPayload,
			Context:         taskContext,
			Secrets:         secrets,
			Vars:            vars,
			Needs:           needs,
		}

		return nil
	}); err != nil {
		return nil, false, err
	}

	if task == nil {
		return nil, false, nil
	}

	CreateCommitStatusForRunJobs(ctx, job.Run, job)
	NotifyWorkflowJobStatusUpdateWithTask(ctx, job, actionTask)
	// job.Run is loaded inside the transaction before UpdateRunJob sets run.Started,
	// so Started is zero only on the very first pick-up of that run.
	if job.Run.Started.IsZero() {
		NotifyWorkflowRunStatusUpdateWithReload(ctx, job.RepoID, job.RunID)
	}

	return task, true, nil
}

func generateTaskContext(ctx context.Context, t *actions_model.ActionTask) (*structpb.Struct, error) {
	giteaRuntimeToken, err := CreateAuthorizationToken(t.ID, t.Job.RunID, t.JobID)
	if err != nil {
		return nil, err
	}

	gitCtx := GenerateGiteaContext(ctx, t.Job.Run, nil, t.Job)
	gitCtx["token"] = t.Token
	gitCtx["gitea_runtime_token"] = giteaRuntimeToken

	return structpb.NewStruct(gitCtx)
}

func findTaskNeeds(ctx context.Context, taskJob *actions_model.ActionRunJob) (map[string]*runnerv1.TaskNeed, error) {
	taskNeeds, err := FindTaskNeeds(ctx, taskJob)
	if err != nil {
		return nil, err
	}
	ret := make(map[string]*runnerv1.TaskNeed, len(taskNeeds))
	for jobID, taskNeed := range taskNeeds {
		ret[jobID] = &runnerv1.TaskNeed{
			Outputs: taskNeed.Outputs,
			Result:  runnerv1.Result(taskNeed.Result),
		}
	}
	return ret, nil
}
