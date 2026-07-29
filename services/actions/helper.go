// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

func getWorkflowDispatchInputsFromRun(run *actions_model.ActionRun) (map[string]any, error) {
	if run.Event != "workflow_dispatch" {
		return map[string]any{}, nil
	}
	var payload api.WorkflowDispatchPayload
	if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
		return nil, err
	}
	return payload.Inputs, nil
}

// getInputsForJob returns the `inputs.*` top-level expression context for a job's evaluation.
//   - For top-level jobs, it falls back to the run's dispatch inputs (empty for non-dispatch events)
//   - For reusable workflow children (and nested callers), this is the direct parent caller's CallPayload.Inputs
func getInputsForJob(ctx context.Context, run *actions_model.ActionRun, job *actions_model.ActionRunJob) (map[string]any, error) {
	if job.ParentJobID == 0 {
		return getWorkflowDispatchInputsFromRun(run)
	}

	caller, err := actions_model.GetRunJobByRunAndID(ctx, run.ID, job.ParentJobID)
	if err != nil {
		return nil, fmt.Errorf("load caller job %d: %w", job.ParentJobID, err)
	}
	if caller.CallPayload == "" {
		// should not happen - a child job cannot reach this point if its caller's CallPayload hasn't been evaluated
		return map[string]any{}, nil
	}
	var p api.WorkflowCallPayload
	if err := json.Unmarshal([]byte(caller.CallPayload), &p); err != nil {
		return nil, util.NewInvalidArgumentErrorf("decode caller %d payload: %v", caller.ID, err)
	}
	if p.Inputs == nil {
		return map[string]any{}, nil
	}
	return p.Inputs, nil
}

// evaluateJobIf evaluates a job's `if:`
func evaluateJobIf(ctx context.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, job *actions_model.ActionRunJob, vars map[string]string, allNeedsSucceed bool) (bool, error) {
	parsedJob, err := job.ParseJob()
	if err != nil {
		return false, err
	}
	// Empty `if:` reduces to implicit `success()` - true iff every need finished as Success.
	if len(parsedJob.If.Value) == 0 {
		return allNeedsSucceed, nil
	}
	// A deferred-matrix placeholder has no combination yet, so an `if:` reading `matrix.*` can only be
	// decided by the emitter's post-expansion pass, against each combination's own values.
	// always()/failure()/cancelled() opt out of the needs gate this falls back to.
	if job.IsMatrixDeferred && jobparser.ExpressionReadsMatrix(parsedJob.If.Value) {
		return allNeedsSucceed || jobparser.ExpressionIgnoresNeedResults(parsedJob.If.Value), nil
	}
	jobResults, err := findJobNeedsAndFillJobResults(ctx, job)
	if err != nil {
		return false, err
	}
	inputs, err := getInputsForJob(ctx, run, job)
	if err != nil {
		return false, err
	}
	// GenerateGiteaContext dereferences the run's repo and trigger user, so load them here instead of
	// relying on whatever the caller happened to load before.
	if err := run.LoadRepo(ctx); err != nil {
		return false, err
	}
	if err := run.LoadTriggerUser(ctx); err != nil {
		return false, err
	}
	gitCtx := GenerateGiteaContext(ctx, run, attempt, job)
	return jobparser.EvaluateJobIfExpression(job.JobID, parsedJob, gitCtx, jobResults, vars, inputs, job.IsMatrixDeferred)
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
