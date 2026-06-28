// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/log"

	"go.yaml.in/yaml/v4"
	"xorm.io/builder"
)

// markMatrixAsEvaluatedAndSkip marks a job as evaluated and skipped unconditionally.
// Used when matrix cannot be expanded or dependency checks fail (no concurrent
// insertion has taken place, so no race is possible).
func markMatrixAsEvaluatedAndSkip(ctx context.Context, job *actions_model.ActionRunJob, reason string) error {
	job.IsMatrixEvaluated = true
	job.Status = actions_model.StatusSkipped
	if _, err := actions_model.UpdateRunJob(ctx, job, nil, "is_matrix_evaluated", "status"); err != nil {
		log.Error("Failed to mark job %d (JobID: %s) as evaluated and skipped: %v", job.ID, job.JobID, err)
		return err
	}
	log.Debug("Marked job %d (JobID: %s) as evaluated and skipped: %s", job.ID, job.JobID, reason)
	return nil
}

// checkTaskNeedsReady verifies if all task dependencies are completed.
// Returns (taskNeeds, allDone, error)
func checkTaskNeedsReady(ctx context.Context, job *actions_model.ActionRunJob) (map[string]*TaskNeed, bool, error) {
	taskNeeds, err := FindTaskNeeds(ctx, job)
	if err != nil {
		return nil, false, fmt.Errorf("find task needs: %w", err)
	}

	log.Debug("Found %d task needs for job %d (JobID: %s)", len(taskNeeds), job.ID, job.JobID)

	var pendingNeeds []string
	for jobID, taskNeed := range taskNeeds {
		if !taskNeed.Result.IsDone() {
			pendingNeeds = append(pendingNeeds, fmt.Sprintf("%s(%s)", jobID, taskNeed.Result))
		}
	}

	if len(pendingNeeds) > 0 {
		log.Debug("Matrix re-evaluation deferred for job %d: pending needs: %v", job.ID, pendingNeeds)
		return taskNeeds, false, nil
	}

	return taskNeeds, true, nil
}

// ReEvaluateMatrixForJobWithNeeds expands the matrix strategy of a job once all its dependent
// jobs are done, using their outputs, and inserts the resulting ActionRunJobs. The original
// placeholder job is reused as the first combination.
// Returns nil, nil if the job is not ready yet or has nothing to do.
func ReEvaluateMatrixForJobWithNeeds(ctx context.Context, job *actions_model.ActionRunJob, vars map[string]string) ([]*actions_model.ActionRunJob, error) {
	if job.IsMatrixEvaluated || job.RawStrategy == "" {
		return nil, nil
	}

	log.Debug("Starting matrix re-evaluation for job %d (JobID: %s)", job.ID, job.JobID)

	// skipWithError marks the job as evaluated+skipped and wraps any secondary error.
	skipWithError := func(origErr error) ([]*actions_model.ActionRunJob, error) {
		if markErr := markMatrixAsEvaluatedAndSkip(ctx, job, origErr.Error()); markErr != nil {
			return nil, fmt.Errorf("%w; additionally failed to mark as evaluated: %v", origErr, markErr)
		}
		return nil, origErr
	}

	taskNeeds, allDone, err := checkTaskNeedsReady(ctx, job)
	if err != nil {
		log.Error("Matrix re-evaluation error for job %d: check task needs: %v", job.ID, err)
		return skipWithError(fmt.Errorf("check task needs: %w", err))
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
		return skipWithError(fmt.Errorf("unmarshal payload: %w", err))
	}
	_, parsedJob := baseSWF.Job()
	if parsedJob == nil {
		return skipWithError(errors.New("payload contains no job"))
	}
	var rawStrategy jobparser.Strategy
	if err := yaml.Unmarshal([]byte(job.RawStrategy), &rawStrategy); err != nil {
		return skipWithError(fmt.Errorf("unmarshal raw strategy: %w", err))
	}
	parsedJob.Strategy = rawStrategy
	if err := parsedJob.RawNeeds.Encode(job.Needs); err != nil {
		return skipWithError(fmt.Errorf("encode needs: %w", err))
	}

	expandedJobs, err := jobparser.ExpandMatrixWithNeeds(job.JobID, parsedJob, giteaCtx.ToGitHubContext(), results, vars, nil)
	if err != nil {
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, fmt.Sprintf("matrix expansion failed: %v", err))
	}

	// One workflow payload per combination; needs are kept on the model and erased from the
	// payload, as at initial planning time.
	type matrixCombo struct {
		name    string
		payload []byte
		runsOn  []string
		needs   []string
	}
	combos := make([]matrixCombo, 0, len(expandedJobs))
	for _, expanded := range expandedJobs {
		combo := matrixCombo{name: expanded.Name, runsOn: expanded.RunsOn(), needs: expanded.Needs()}
		swf := baseSWF
		if err := swf.SetJob(job.JobID, expanded.EraseNeeds()); err != nil {
			return nil, fmt.Errorf("set expanded job %s: %w", job.JobID, err)
		}
		if combo.payload, err = swf.Marshal(); err != nil {
			return nil, fmt.Errorf("marshal expanded job %s: %w", job.JobID, err)
		}
		combos = append(combos, combo)
	}

	if len(combos) == 0 {
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, "matrix expanded to no combinations")
	}

	// Cap expansion at MaxJobNumPerRun: a runtime fromJson() value must not create unbounded jobs,
	// and the AttemptJobIDs (used in job URLs) must stay below the limit. Siblings take the IDs
	// above the current max, so that highest value is what must stay in range.
	maxAttemptJobID, err := actions_model.GetMaxAttemptJobID(ctx, job.RunID, job.RunAttemptID)
	if err != nil {
		return nil, fmt.Errorf("get max attempt job id for job %d: %w", job.ID, err)
	}
	if maxAttemptJobID+int64(len(combos))-1 >= actions_model.MaxJobNumPerRun {
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, fmt.Sprintf("matrix expansion to %d combinations would exceed the per-run job limit of %d", len(combos), actions_model.MaxJobNumPerRun))
	}

	// Reuse the placeholder as the first combination and insert the rest as siblings: no phantom
	// skipped job is left to poison downstream needs, and siblings inherit attempt + permissions.
	children := make([]*actions_model.ActionRunJob, 0, len(combos)-1)
	if err := db.WithTx(ctx, func(txCtx context.Context) error {
		for i := 1; i < len(combos); i++ {
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
				AttemptJobID:      maxAttemptJobID + int64(i),
				Needs:             combos[i].needs,
				RunsOn:            combos[i].runsOn,
				TokenPermissions:  job.TokenPermissions,
				Status:            actions_model.StatusWaiting,
			})
		}

		// Atomic claim: only one concurrent caller flips the placeholder out of Blocked.
		job.Name = combos[0].name
		job.WorkflowPayload = combos[0].payload
		job.RunsOn = combos[0].runsOn
		job.IsMatrixEvaluated = true
		job.Status = actions_model.StatusWaiting
		affected, err := actions_model.UpdateRunJob(txCtx, job,
			builder.Eq{"is_matrix_evaluated": false, "status": actions_model.StatusBlocked},
			"name", "workflow_payload", "runs_on", "is_matrix_evaluated", "status")
		if err != nil {
			return err
		}
		if affected != 1 {
			children = nil
			return nil
		}
		return actions_model.InsertActionRunJobs(txCtx, children)
	}); err != nil {
		return nil, fmt.Errorf("expand matrix for job %d: %w", job.ID, err)
	}

	if children != nil {
		if err := actions_model.IncreaseTaskVersion(ctx, job.OwnerID, job.RepoID); err != nil {
			log.Error("IncreaseTaskVersion after matrix expand for job %d: %v", job.ID, err)
		}
	}

	return children, nil
}
