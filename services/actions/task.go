// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	secret_model "code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	notify_service "code.gitea.io/gitea/services/notify"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"xorm.io/builder"
)

func PickTask(ctx context.Context, runner *actions_model.ActionRunner) (*runnerv1.Task, bool, error) {
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
			if task.Status == actions_model.StatusWaiting || task.Status == actions_model.StatusRunning || task.Status == actions_model.StatusBlocked {
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

	// TODO: we now need to filter task_id and runs_on labeles in the memory for effeciency.
	// It can be optimized by adding more conditions in the SQL query once
	// the database schema is ready
	jobs, err := actions_model.GetWaitingRunJobsForRunner(ctx, runner)
	if err != nil {
		return nil, false, err
	}
	if len(jobs) == 0 {
		return nil, false, nil
	}

	var job *actions_model.ActionRunJob
	log.Trace("runner labels: %v", runner.AgentLabels)
	for _, v := range jobs {
		if v.TaskID == 0 && runner.CanMatchLabels(v.RunsOn) {
			job = v
			break
		}
	}
	if job == nil {
		return nil, false, nil
	}

	var (
		task       *runnerv1.Task
		actionTask *actions_model.ActionTask
	)

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		// create task from the job
		now := timeutil.TimeStampNow()
		t, err := actions_model.InsertActionTaskFromJob(ctx, job, runner, now)
		if err != nil {
			return err
		}

		// update job status
		job.Attempt++
		job.Started = now
		job.Status = actions_model.StatusRunning
		job.TaskID = t.ID
		if n, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"task_id": 0}, "attempt", "started", "status", "task_id"); err != nil {
			return err
		} else if n != 1 {
			// return nil will not roll back the transaction, so we need to return
			// an error here to roll back the transaction and let the runner retry,
			// but we should not treat it as an actual error.
			return actions_model.ErrTaskAssignedToOtherRunner
		}

		if err := t.LoadAttributes(ctx); err != nil {
			return fmt.Errorf("task LoadAttributes: %w", err)
		}

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

		taskContext, err := generateTaskContext(t)
		if err != nil {
			return fmt.Errorf("generateTaskContext: %w", err)
		}

		actionTask = t
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

	CreateCommitStatusForRunJobs(ctx, job.Run, job)
	notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, actionTask)
	// job.Run is loaded inside the transaction before UpdateRunJob sets run.Started,
	// so Started is zero only on the very first pick-up of that run.
	if job.Run.Started.IsZero() {
		NotifyWorkflowRunStatusUpdateWithReload(ctx, job)
	}

	return task, true, nil
}

func generateTaskContext(t *actions_model.ActionTask) (*structpb.Struct, error) {
	giteaRuntimeToken, err := CreateAuthorizationToken(t.ID, t.Job.RunID, t.JobID)
	if err != nil {
		return nil, err
	}

	gitCtx := GenerateGiteaContext(t.Job.Run, t.Job)
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
