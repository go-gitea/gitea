// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/actions/jobparser"

	act_model "gitea.com/gitea/runner/act/model"
	"go.yaml.in/yaml/v4"
)

// EvaluateRunConcurrencyFillModel evaluates the expressions in a run-level (workflow) concurrency,
// and fills the run attempt model with the evaluated `concurrency.group` and `concurrency.cancel-in-progress` values.
// Workflow-level concurrency doesn't depend on the job outputs, so it can always be evaluated if there is no syntax error.
// See https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#concurrency
func EvaluateRunConcurrencyFillModel(ctx context.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, wfRawConcurrency *act_model.RawConcurrency, vars map[string]string, inputs map[string]any) error {
	if err := run.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("run LoadAttributes: %w", err)
	}

	actionsRunCtx := GenerateGiteaContext(ctx, run, attempt, nil)
	jobResults := map[string]*jobparser.JobResult{"": {}}
	if inputs == nil {
		var err error
		inputs, err = getWorkflowDispatchInputsFromRun(run)
		if err != nil {
			return fmt.Errorf("get inputs: %w", err)
		}
	}

	var err error
	attempt.ConcurrencyGroup, attempt.ConcurrencyCancel, err = jobparser.EvaluateConcurrency(wfRawConcurrency, "", nil, actionsRunCtx, jobResults, vars, inputs)
	if err != nil {
		return fmt.Errorf("evaluate concurrency: %w", err)
	}
	return nil
}

// EvaluateJobConcurrencyFillModel evaluates the expressions in a job-level concurrency,
// and fills the job's model fields with `concurrency.group` and `concurrency.cancel-in-progress`.
// Job-level concurrency may depend on other job's outputs (via `needs`): `concurrency.group: my-group-${{ needs.job1.outputs.out1 }}`
// If the needed jobs haven't been executed yet, this evaluation will also fail.
// See https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idconcurrency
func EvaluateJobConcurrencyFillModel(ctx context.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, actionRunJob *actions_model.ActionRunJob, vars map[string]string, inputs map[string]any) error {
	if err := actionRunJob.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("job LoadAttributes: %w", err)
	}

	var rawConcurrency act_model.RawConcurrency
	if err := yaml.Unmarshal([]byte(actionRunJob.RawConcurrency), &rawConcurrency); err != nil {
		return fmt.Errorf("unmarshal raw concurrency: %w", err)
	}

	actionsJobCtx := GenerateGiteaContext(ctx, run, attempt, actionRunJob)

	jobResults, err := findJobNeedsAndFillJobResults(ctx, actionRunJob)
	if err != nil {
		return fmt.Errorf("find job needs and fill job results: %w", err)
	}

	if inputs == nil {
		var err error
		inputs, err = getInputsForJob(ctx, run, actionRunJob)
		if err != nil {
			return fmt.Errorf("get inputs: %w", err)
		}
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
