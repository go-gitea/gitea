// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	"gitea.dev/actionslib/pkg/model"
	actions_model "gitea.dev/models/actions"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// getWorkflowDispatchInputsFromRun returns the `inputs.*` context for a workflow_dispatch job's
// server-side `if:` evaluation. run.EventPayload (and thus `github.event.inputs`) keeps GitHub's
// raw string values, so `type: boolean` inputs are re-coerced here from job's own WorkflowPayload,
// which still carries the workflow's `on:` declaration (see SingleWorkflow.CloneHeader).
// job may be nil for workflow-level (run) concurrency evaluation, which has no job in scope; the
// coercion is then skipped and boolean inputs are compared as the raw dispatch strings.
func getWorkflowDispatchInputsFromRun(run *actions_model.ActionRun, job *actions_model.ActionRunJob) (map[string]any, error) {
	if run.Event != "workflow_dispatch" {
		return map[string]any{}, nil
	}
	var payload api.WorkflowDispatchPayload
	if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
		return nil, err
	}
	if job == nil {
		return payload.Inputs, nil
	}
	swf, _, err := jobparser.ParseRawSingleWorkflow(job.WorkflowPayload)
	if err != nil {
		return nil, fmt.Errorf("parse job %d workflow payload: %w", job.ID, err)
	}
	if dispatch := (&model.Workflow{RawOn: swf.RawOn}).WorkflowDispatchConfig(); dispatch != nil {
		coerceDispatchInputTypes(dispatch, payload.Inputs)
	}
	return payload.Inputs, nil
}

// getInputsForJob returns the `inputs.*` top-level expression context for a job's evaluation.
//   - For top-level jobs, it falls back to the run's dispatch inputs (empty for non-dispatch events)
//   - For reusable workflow children (and nested callers), this is the direct parent caller's CallPayload.Inputs
func getInputsForJob(ctx context.Context, run *actions_model.ActionRun, job *actions_model.ActionRunJob) (map[string]any, error) {
	if job.ParentJobID == 0 {
		return getWorkflowDispatchInputsFromRun(run, job)
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

// pullRequestTargetBaseSHA returns the base branch commit of a pull_request_target run, and whether the run is one.
func pullRequestTargetBaseSHA(run *actions_model.ActionRun) (string, bool) {
	if run.TriggerEvent != actions_module.GithubEventPullRequestTarget {
		return "", false
	}
	payload, err := run.GetPullRequestEventPayload()
	if err != nil {
		log.Error("run %d: get pull request event payload: %v", run.ID, err)
		return "", false
	}
	if payload.PullRequest == nil || payload.PullRequest.Base == nil || payload.PullRequest.Base.Sha == "" {
		return "", false
	}
	return payload.PullRequest.Base.Sha, true
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
