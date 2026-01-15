// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/nektos/act/pkg/jobparser"
	act_model "github.com/nektos/act/pkg/model"
	"gopkg.in/yaml.v3"
)

// EvaluateRunConcurrencyFillModel evaluates the expressions in a run-level (workflow) concurrency,
// and fills the run's model fields with `concurrency.group` and `concurrency.cancel-in-progress`.
// Workflow-level concurrency doesn't depend on the job outputs, so it can always be evaluated if there is no syntax error.
// See https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#concurrency
func EvaluateRunConcurrencyFillModel(ctx context.Context, run *actions_model.ActionRun, wfRawConcurrency *act_model.RawConcurrency, vars map[string]string) error {
	if err := run.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("run LoadAttributes: %w", err)
	}

	actionsRunCtx := GenerateGiteaContext(run, nil)
	jobResults := map[string]*jobparser.JobResult{"": {}}
	inputs, err := getInputsFromRun(run)
	if err != nil {
		return fmt.Errorf("get inputs: %w", err)
	}

	rawConcurrency, err := yaml.Marshal(wfRawConcurrency)
	if err != nil {
		return fmt.Errorf("marshal raw concurrency: %w", err)
	}
	run.RawConcurrency = string(rawConcurrency)
	run.ConcurrencyGroup, run.ConcurrencyCancel, err = jobparser.EvaluateConcurrency(wfRawConcurrency, "", nil, actionsRunCtx, jobResults, vars, inputs)
	if err != nil {
		return fmt.Errorf("evaluate concurrency: %w", err)
	}
	return nil
}

func findJobNeedsAndFillJobResults(ctx context.Context, job *actions_model.ActionRunJob) (map[string]*jobparser.JobResult, error) {
	taskNeeds, err := FindTaskNeeds(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("find task needs: %w", err)
	}
	jobResults := make(map[string]*jobparser.JobResult, len(taskNeeds))
	for jobID, taskNeed := range taskNeeds {
		jobResult := &jobparser.JobResult{
			Result:  taskNeed.Result.String(),
			Outputs: taskNeed.Outputs,
		}
		jobResults[jobID] = jobResult
	}
	jobResults[job.JobID] = &jobparser.JobResult{
		Needs: job.Needs,
	}
	return jobResults, nil
}

// EvaluateJobConcurrencyFillModel evaluates the expressions in a job-level concurrency,
// and fills the job's model fields with `concurrency.group` and `concurrency.cancel-in-progress`.
// Job-level concurrency may depend on other job's outputs (via `needs`): `concurrency.group: my-group-${{ needs.job1.outputs.out1 }}`
// If the needed jobs haven't been executed yet, this evaluation will also fail.
// See https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idconcurrency
func EvaluateJobConcurrencyFillModel(ctx context.Context, run *actions_model.ActionRun, actionRunJob *actions_model.ActionRunJob, vars map[string]string) error {
	if err := actionRunJob.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("job LoadAttributes: %w", err)
	}

	var rawConcurrency act_model.RawConcurrency
	if err := yaml.Unmarshal([]byte(actionRunJob.RawConcurrency), &rawConcurrency); err != nil {
		return fmt.Errorf("unmarshal raw concurrency: %w", err)
	}

	actionsJobCtx := GenerateGiteaContext(run, actionRunJob)

	jobResults, err := findJobNeedsAndFillJobResults(ctx, actionRunJob)
	if err != nil {
		return fmt.Errorf("find job needs and fill job results: %w", err)
	}

	inputs, err := getInputsFromRun(run)
	if err != nil {
		return fmt.Errorf("get inputs: %w", err)
	}

	workflowJob, err := actionRunJob.ParseJob()
	if err != nil {
		return fmt.Errorf("load job %d: %w", actionRunJob.ID, err)
	}

	actionRunJob.ConcurrencyGroup, actionRunJob.ConcurrencyCancel, err = jobparser.EvaluateConcurrency(&rawConcurrency, actionRunJob.JobID, workflowJob, actionsJobCtx, jobResults, vars, inputs)
	if err != nil {
		return fmt.Errorf("evaluate concurrency: %w", err)
	}
	actionRunJob.IsConcurrencyEvaluated = true
	return nil
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
