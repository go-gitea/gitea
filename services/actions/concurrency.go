// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"gopkg.in/yaml.v3"

	"github.com/nektos/act/pkg/jobparser"
	act_model "github.com/nektos/act/pkg/model"
)

func EvaluateWorkflowConcurrency(ctx context.Context, run *actions_model.ActionRun, rc *act_model.RawConcurrency, vars map[string]string) (string, bool, error) {
	if err := run.LoadAttributes(ctx); err != nil {
		return "", false, fmt.Errorf("run LoadAttributes: %w", err)
	}

	gitCtx := GenerateGiteaContext(run, nil)
	jobResults := map[string]*jobparser.JobResult{"": {}}
	inputs, err := getInputsFromRun(run)
	if err != nil {
		return "", false, fmt.Errorf("get inputs: %w", err)
	}

	concurrencyGroup, concurrencyCancel, err := jobparser.EvaluateConcurrency(rc, "", nil, gitCtx, jobResults, vars, inputs)
	if err != nil {
		return "", false, fmt.Errorf("evaluate concurrency: %w", err)
	}

	return concurrencyGroup, concurrencyCancel, nil
}

func EvaluateJobConcurrency(ctx context.Context, run *actions_model.ActionRun, actionRunJob *actions_model.ActionRunJob, vars map[string]string, jobResults map[string]*jobparser.JobResult) (string, bool, error) {
	if err := actionRunJob.LoadAttributes(ctx); err != nil {
		return "", false, fmt.Errorf("job LoadAttributes: %w", err)
	}

	var rawConcurrency act_model.RawConcurrency
	if err := yaml.Unmarshal([]byte(actionRunJob.RawConcurrency), &rawConcurrency); err != nil {
		return "", false, fmt.Errorf("unmarshal raw concurrency: %w", err)
	}

	gitCtx := GenerateGiteaContext(run, actionRunJob)
	if jobResults == nil {
		jobResults = map[string]*jobparser.JobResult{}
	}
	jobResults[actionRunJob.JobID] = &jobparser.JobResult{
		Needs: actionRunJob.Needs,
	}
	inputs, err := getInputsFromRun(run)
	if err != nil {
		return "", false, fmt.Errorf("get inputs: %w", err)
	}

	singleWorkflows, err := jobparser.Parse(actionRunJob.WorkflowPayload)
	if err != nil {
		return "", false, fmt.Errorf("parse single workflow: %w", err)
	} else if len(singleWorkflows) != 1 {
		return "", false, errors.New("not single workflow")
	}
	_, singleWorkflowJob := singleWorkflows[0].Job()

	concurrencyGroup, concurrencyCancel, err := jobparser.EvaluateConcurrency(&rawConcurrency, actionRunJob.JobID, singleWorkflowJob, gitCtx, jobResults, vars, inputs)
	if err != nil {
		return "", false, fmt.Errorf("evaluate concurrency: %w", err)
	}

	return concurrencyGroup, concurrencyCancel, nil
}

func getInputsFromRun(run *actions_model.ActionRun) (map[string]any, error) {
	if run.Event != "workflow_dispatch" {
		return map[string]any{}, nil
	}
	var payload api.WorkflowDispatchPayload
	if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
		return nil, err
	}
	return payload.Inputs, nil
}
