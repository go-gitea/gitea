// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/log"

	"go.yaml.in/yaml/v4"
	"xorm.io/builder"
)

// markMatrixEvaluated marks a job as evaluated with a terminal status (skipped or failure) so it is
// not re-evaluated. Used when the matrix cannot be expanded (failure) or expands to nothing (skipped).
func markMatrixEvaluated(ctx context.Context, job *actions_model.ActionRunJob, status actions_model.Status, reason string) error {
	job.IsMatrixEvaluated = true
	job.Status = status
	if _, err := actions_model.UpdateRunJob(ctx, job, nil, "is_matrix_evaluated", "status"); err != nil {
		log.Error("Failed to mark job %d (JobID: %s) as evaluated (%s): %v", job.ID, job.JobID, status, err)
		return err
	}
	log.Debug("Marked job %d (JobID: %s) as evaluated with status %s: %s", job.ID, job.JobID, status, reason)
	return nil
}

// checkTaskNeedsReady verifies if all task dependencies are completed.
// Returns (taskNeeds, allDone, error)
func checkTaskNeedsReady(ctx context.Context, job *actions_model.ActionRunJob) (map[string]*TaskNeed, bool, error) {
	taskNeeds, err := FindTaskNeeds(ctx, job)
	if err != nil {
		return nil, false, fmt.Errorf("find task needs: %w", err)
	}

	for _, taskNeed := range taskNeeds {
		if !taskNeed.Result.IsDone() {
			return taskNeeds, false, nil
		}
	}

	return taskNeeds, true, nil
}

// ReEvaluateMatrixForJobWithNeeds expands the matrix strategy of a job once all its dependent
// jobs are done, using their outputs, and inserts the resulting ActionRunJobs. The original
// placeholder job is reused as the first combination and kept Blocked so the caller's resolver
// still evaluates its `if:` and job-level concurrency; the inserted siblings are also Blocked and
// resolved on the follow-up pass triggered by the returned slice being non-empty.
// Returns nil, nil if the job is not ready yet or has nothing to do; the returned slice holds only
// the newly-inserted siblings (empty when nothing was inserted, e.g. a single combination or a
// lost expansion race).
func ReEvaluateMatrixForJobWithNeeds(ctx context.Context, job *actions_model.ActionRunJob, vars map[string]string) ([]*actions_model.ActionRunJob, error) {
	if job.IsMatrixEvaluated || job.RawStrategy == "" {
		return nil, nil
	}

	log.Debug("Starting matrix re-evaluation for job %d (JobID: %s)", job.ID, job.JobID)

	// failWithError marks the job as evaluated+failed and wraps any secondary error. A genuine error
	// (malformed needs output, internal decode failure) must surface as a failure, not a silent skip.
	failWithError := func(origErr error) ([]*actions_model.ActionRunJob, error) {
		if markErr := markMatrixEvaluated(ctx, job, actions_model.StatusFailure, origErr.Error()); markErr != nil {
			return nil, fmt.Errorf("%w; additionally failed to mark as evaluated: %v", origErr, markErr)
		}
		return nil, origErr
	}

	taskNeeds, allDone, err := checkTaskNeedsReady(ctx, job)
	if err != nil {
		log.Error("Matrix re-evaluation error for job %d: check task needs: %v", job.ID, err)
		return failWithError(fmt.Errorf("check task needs: %w", err))
	}
	if !allDone {
		return nil, nil
	}

	if err := job.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("load job attributes: %w", err)
	}

	giteaCtx := GenerateGiteaContext(ctx, job.Run, nil, job)

	results := make(map[string]*jobparser.JobResult, len(taskNeeds))
	for needID, need := range taskNeeds {
		results[needID] = &jobparser.JobResult{Result: need.Result.String(), Outputs: need.Outputs}
	}

	// Rebuild the job from its own payload + stored raw strategy and re-attach needs (erased from
	// the payload) so needs.*.outputs.* resolves. No synthetic workflow, no stub jobs, no re-parse.
	var baseSWF jobparser.SingleWorkflow
	if err := yaml.Unmarshal(job.WorkflowPayload, &baseSWF); err != nil {
		return failWithError(fmt.Errorf("unmarshal payload: %w", err))
	}
	_, parsedJob := baseSWF.Job()
	if parsedJob == nil {
		return failWithError(errors.New("payload contains no job"))
	}
	var rawStrategy jobparser.Strategy
	if err := yaml.Unmarshal([]byte(job.RawStrategy), &rawStrategy); err != nil {
		return failWithError(fmt.Errorf("unmarshal raw strategy: %w", err))
	}
	parsedJob.Strategy = rawStrategy
	if err := parsedJob.RawNeeds.Encode(job.Needs); err != nil {
		return failWithError(fmt.Errorf("encode needs: %w", err))
	}

	expandedJobs, err := jobparser.ExpandMatrixWithNeeds(job.JobID, parsedJob, giteaCtx.ToGitHubContext(), results, vars, nil)
	if err != nil {
		return failWithError(fmt.Errorf("matrix expansion failed: %w", err))
	}

	// An empty matrix (e.g. fromJson('[]')) yields no combinations: skip the job, matching GitHub.
	if len(expandedJobs) == 0 {
		return nil, markMatrixEvaluated(ctx, job, actions_model.StatusSkipped, "matrix expanded to no combinations")
	}

	// One workflow payload per combination; needs are kept on the model and erased from the
	// payload, as at initial planning time.
	type matrixCombo struct {
		name            string
		payload         []byte
		runsOn          []string
		needs           []string
		continueOnError bool
	}
	combos := make([]matrixCombo, 0, len(expandedJobs))
	for _, expanded := range expandedJobs {
		combo := matrixCombo{
			name:            expanded.Name,
			runsOn:          expanded.RunsOn(),
			needs:           expanded.Needs(),
			continueOnError: expanded.GetContinueOnError(),
		}
		swf := baseSWF
		if err := swf.SetJob(job.JobID, expanded.EraseNeeds()); err != nil {
			return nil, fmt.Errorf("set expanded job %s: %w", job.JobID, err)
		}
		if combo.payload, err = swf.Marshal(); err != nil {
			return nil, fmt.Errorf("marshal expanded job %s: %w", job.JobID, err)
		}
		combos = append(combos, combo)
	}

	// Cap expansion at MaxJobNumPerRun: a runtime fromJson() value must not create unbounded jobs.
	maxAttemptJobID, err := actions_model.GetMaxAttemptJobID(ctx, job.RunID, job.RunAttemptID)
	if err != nil {
		return nil, fmt.Errorf("get max attempt job id for job %d: %w", job.ID, err)
	}
	if maxAttemptJobID+int64(len(combos))-1 >= actions_model.MaxJobNumPerRun {
		return failWithError(fmt.Errorf("matrix expansion to %d combinations would exceed the per-run job limit of %d", len(combos), actions_model.MaxJobNumPerRun))
	}

	// Reuse the placeholder as the first combination and insert the rest as siblings: no phantom
	// skipped job is left to poison downstream needs, and siblings inherit attempt + permissions.
	// This runs inside the caller's transaction (job_emitter's resolver); do NOT open a nested
	// db.WithTx here, as reusing the ambient session and closing it on error would roll back the
	// whole emitter pass.
	//
	// Atomic claim: only one concurrent caller flips is_matrix_evaluated for this placeholder. The
	// placeholder stays Blocked so the resolver still runs its `if:`/concurrency gates on combo 0.
	job.Name = combos[0].name
	job.WorkflowPayload = combos[0].payload
	job.RunsOn = combos[0].runsOn
	job.ContinueOnError = combos[0].continueOnError
	job.IsMatrixEvaluated = true
	affected, err := actions_model.UpdateRunJob(ctx, job,
		builder.Eq{"is_matrix_evaluated": false, "status": actions_model.StatusBlocked},
		"name", "workflow_payload", "runs_on", "continue_on_error", "is_matrix_evaluated")
	if err != nil {
		return nil, fmt.Errorf("claim placeholder for job %d: %w", job.ID, err)
	}
	if affected != 1 {
		// Another concurrent caller won the claim; leave the siblings to it.
		return nil, nil
	}

	children := make([]*actions_model.ActionRunJob, 0, len(combos)-1)
	for i := 1; i < len(combos); i++ {
		// Allocate AttemptJobIDs from the run-wide atomic counter so they never collide with IDs
		// handed out later by reusable-caller expansion or reruns.
		attemptJobID, err := actions_model.GetNextAttemptJobID(ctx, job.RunID)
		if err != nil {
			return nil, fmt.Errorf("alloc attempt_job_id for job %d: %w", job.ID, err)
		}
		children = append(children, &actions_model.ActionRunJob{
			RunID:             job.RunID,
			RunAttemptID:      job.RunAttemptID,
			RepoID:            job.RepoID,
			OwnerID:           job.OwnerID,
			CommitSHA:         job.CommitSHA,
			IsForkPullRequest: job.IsForkPullRequest,
			Name:              combos[i].name,
			Attempt:           job.Attempt,
			WorkflowPayload:   combos[i].payload,
			JobID:             job.JobID,
			AttemptJobID:      attemptJobID,
			Needs:             combos[i].needs,
			RunsOn:            combos[i].runsOn,
			ContinueOnError:   combos[i].continueOnError,
			RawConcurrency:    job.RawConcurrency,
			TokenPermissions:  job.TokenPermissions,
			Status:            actions_model.StatusBlocked,
		})
	}

	if err := actions_model.InsertActionRunJobs(ctx, children); err != nil {
		return nil, fmt.Errorf("insert matrix siblings for job %d: %w", job.ID, err)
	}

	if len(children) > 0 {
		if err := actions_model.IncreaseTaskVersion(ctx, job.OwnerID, job.RepoID); err != nil {
			log.Error("IncreaseTaskVersion after matrix expand for job %d: %v", job.ID, err)
		}
	}

	return children, nil
}
