// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	secret_model "code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func pickTask(ctx context.Context, runner *actions_model.ActionRunner) (*runnerv1.Task, bool, error) {
	t, ok, err := actions_model.CreateTaskForRunner(ctx, runner)
	if err != nil {
		return nil, false, fmt.Errorf("CreateTaskForRunner: %w", err)
	}
	if !ok {
		return nil, false, nil
	}

	secrets, err := secret_model.GetSecretsOfTask(ctx, t)
	if err != nil {
		return nil, false, fmt.Errorf("GetSecretsOfTask: %w", err)
	}

	vars, err := actions_model.GetVariablesOfRun(ctx, t.Job.Run)
	if err != nil {
		return nil, false, fmt.Errorf("GetVariablesOfRun: %w", err)
	}

	actions.CreateCommitStatus(ctx, t.Job)

	task := &runnerv1.Task{
		Id:              t.ID,
		WorkflowPayload: t.Job.WorkflowPayload,
		Context:         generateTaskContext(t),
		Secrets:         secrets,
		Vars:            vars,
	}

	if needs, err := findTaskNeeds(ctx, t); err != nil {
		log.Error("Cannot find needs for task %v: %v", t.ID, err)
		// Go on with empty needs.
		// If return error, the task will be wild, which means the runner will never get it when it has been assigned to the runner.
		// In contrast, missing needs is less serious.
		// And the task will fail and the runner will report the error in the logs.
	} else {
		task.Needs = needs
	}

	return task, true, nil
}

func generateTaskContext(t *actions_model.ActionTask) *structpb.Struct {
	giteaRuntimeToken, err := actions.CreateAuthorizationToken(t.ID, t.Job.RunID, t.JobID)
	if err != nil {
		log.Error("actions.CreateAuthorizationToken failed: %v", err)
	}

	gitCtx := actions.GenerateGiteaContext(t.Job.Run, t.Job)
	gitCtx["token"] = t.Token
	gitCtx["gitea_runtime_token"] = giteaRuntimeToken

	taskContext, err := structpb.NewStruct(gitCtx)
	if err != nil {
		log.Error("structpb.NewStruct failed: %v", err)
	}

	return taskContext
}

func findTaskNeeds(ctx context.Context, task *actions_model.ActionTask) (map[string]*runnerv1.TaskNeed, error) {
	if err := task.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("task LoadAttributes: %w", err)
	}
	taskNeeds, err := actions.FindTaskNeeds(ctx, task.Job)
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
