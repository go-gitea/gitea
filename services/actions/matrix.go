// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"slices"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// expandDeferredMatrix expands a deferred-matrix placeholder once its needs are done and the job is
// known to run, using the needs' outputs: the placeholder becomes the first combination and the rest
// are returned as inserted siblings, all left Blocked so the caller's resolver still applies the
// concurrency gate.
//
// It runs inside the caller's transaction (job_emitter's resolver) and must not open a nested
// db.WithTx, which would reuse the ambient session and roll the whole emitter pass back on error.
// The three outcomes are:
//   - expanded: IsMatrixDeferred is cleared and the job stays StatusBlocked.
//   - the workflow's fault (a matrix that cannot resolve, or one too large): a terminal status is
//     persisted here, reported by the job leaving StatusBlocked. IsMatrixDeferred stays set, marking
//     the payload as still unexpanded for a later rerun to re-derive.
//   - not now: losing the claim to a concurrent pass leaves the job deferred and blocked, for the
//     pass that won the claim to carry through.
//
// Every other failure is returned, which aborts the emitter pass and lets the job-emitter queue retry
// it. Swallowing one would strand the placeholder: nothing re-emits a run whose needs have all already
// finished, so there would be no next pass to retry in. After the placeholder has been claimed a
// returned error additionally has to roll the caller's transaction back, because committing it would
// drop the remaining combinations for good.
func expandDeferredMatrix(ctx context.Context, job *actions_model.ActionRunJob, vars map[string]string) ([]*actions_model.ActionRunJob, error) {
	if !job.IsMatrixDeferred {
		return nil, nil
	}

	// failTerminal fails the job here rather than through the resolver's status map, which a
	// reusable caller (a job may be both) would drop: its branch only handles waiting and skipped.
	// The commit status is unaffected by the surviving flag: suppression only applies while the job
	// is not done.
	failTerminal := func(cause error) error {
		log.Warn("Matrix expansion failed for job %d (JobID: %s): %v", job.ID, job.JobID, cause)
		prevStatus, prevStopped := job.Status, job.Stopped
		job.Status = actions_model.StatusFailure
		job.Stopped = timeutil.TimeStampNow()
		// The flag survives, so it alone no longer tells a fresh placeholder from one a concurrent
		// pass already failed: the status has to be part of the condition.
		affected, err := actions_model.UpdateRunJob(ctx, job,
			builder.Eq{"is_matrix_deferred": true, "status": actions_model.StatusBlocked},
			"status", "stopped")
		if err != nil {
			return fmt.Errorf("fail deferred matrix job %d: %w", job.ID, err)
		}
		if affected != 1 {
			// A concurrent pass already advanced the row. Restore the in-memory state so this pass
			// does not report a failure that was never persisted.
			job.Status, job.Stopped = prevStatus, prevStopped
		}
		return nil
	}

	// The resolver only calls this once every need is done, as it does for job concurrency.
	results, err := findJobNeedsAndFillJobResults(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("find needs: %w", err)
	}

	if err := job.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("load attributes: %w", err)
	}

	// The payload still carries the raw, unevaluated matrix: planning only erases the needs.
	baseSWF, parsedJob, err := jobparser.ParseRawSingleWorkflow(job.WorkflowPayload)
	if err != nil {
		return nil, failTerminal(fmt.Errorf("parse payload: %w", err))
	}

	// `strategy` may reference the inputs context as well as needs, so resolve it like `if:` does.
	inputs, err := getInputsForJob(ctx, job.Run, job)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			// A malformed payload never becomes readable, so retrying would requeue the run forever.
			return nil, failTerminal(fmt.Errorf("get inputs: %w", err))
		}
		return nil, fmt.Errorf("get inputs: %w", err)
	}

	existingJobs, err := actions_model.CountRunJobsByRunAndAttemptID(ctx, job.RunID, job.RunAttemptID)
	if err != nil {
		return nil, fmt.Errorf("count jobs of attempt %d: %w", job.RunAttemptID, err)
	}
	// The placeholder is reused as the first combination, so the attempt only grows by len-1.
	maxCombinations := int(actions_model.MaxJobNumPerRun - existingJobs + 1)

	giteaCtx := GenerateGiteaContext(ctx, job.Run, nil, job)
	expandedJobs, err := jobparser.ExpandMatrixWithNeeds(job.JobID, parsedJob, giteaCtx.ToGitHubContext(), results, vars, inputs, maxCombinations)
	if err != nil {
		return nil, failTerminal(fmt.Errorf("expand matrix: %w", err))
	}
	// Combinations differ only in what the matrix feeds: the name, the payload, and a
	// runs-on/continue-on-error that may interpolate matrix.*.
	applyCombo := func(dst *actions_model.ActionRunJob, combo *jobparser.Job) error {
		swf := baseSWF.CloneHeader()
		if err := swf.SetJob(job.JobID, combo.EraseNeeds()); err != nil {
			return fmt.Errorf("set expanded job: %w", err)
		}
		payload, err := swf.Marshal()
		if err != nil {
			return fmt.Errorf("marshal expanded job: %w", err)
		}
		dst.Name = util.EllipsisDisplayString(combo.Name, 255)
		dst.WorkflowPayload, dst.RunsOn = payload, combo.RunsOn()
		dst.ContinueOnError = combo.GetContinueOnError()
		return nil
	}

	siblings := make([]*actions_model.ActionRunJob, 0, len(expandedJobs)-1)
	for _, combo := range expandedJobs[1:] {
		// Inherit from the placeholder rather than listing fields, so a sibling cannot silently lose
		// one (scope, permissions, `uses:`) as the job model grows.
		sibling := *job
		sibling.ID, sibling.TaskID, sibling.SourceTaskID, sibling.AttemptJobID = 0, 0, 0, 0
		sibling.Started, sibling.Stopped, sibling.IsMatrixDeferred = 0, 0, false
		sibling.Status = actions_model.StatusBlocked
		sibling.Needs = slices.Clone(job.Needs)
		// Only the placeholder keeps the raw payload, which is what identifies it as the group's anchor.
		sibling.DeferredMatrixPayload = nil
		if err := applyCombo(&sibling, combo); err != nil {
			return nil, failTerminal(err)
		}
		siblings = append(siblings, &sibling)
	}

	// Keep AttemptJobIDs stable across attempts (best-effort). A single combination reuses the
	// placeholder's row, so there is nothing to look up.
	if len(siblings) > 0 {
		parentAttemptJobID := int64(0)
		if job.ParentJobID > 0 {
			parent, err := actions_model.GetRunJobByRunAndID(ctx, job.RunID, job.ParentJobID)
			if err != nil {
				return nil, fmt.Errorf("load parent of job %d: %w", job.ID, err)
			}
			parentAttemptJobID = parent.AttemptJobID
		}
		priorCombos, err := actions_model.GetPriorAttemptMatrixCombos(ctx, job.RunID, job.RunAttemptID, parentAttemptJobID, job.JobID)
		if err != nil {
			return nil, fmt.Errorf("lookup prior attempt combos of job %d: %w", job.ID, err)
		}
		usedIDs := container.SetOf(job.AttemptJobID)
		for _, sibling := range siblings {
			if prior, ok := priorCombos[sibling.Name]; ok && usedIDs.Add(prior.AttemptJobID) {
				sibling.AttemptJobID = prior.AttemptJobID
			}
		}
	}

	// Reusing the placeholder leaves no phantom skipped job behind to poison downstream needs. The
	// conditional update is an atomic claim: only the caller that flips IsMatrixDeferred inserts.
	beforeClaim := *job
	if err := applyCombo(job, expandedJobs[0]); err != nil {
		return nil, failTerminal(err)
	}
	job.IsMatrixDeferred = false
	affected, err := actions_model.UpdateRunJob(ctx, job,
		builder.Eq{"is_matrix_deferred": true, "status": actions_model.StatusBlocked},
		"name", "workflow_payload", "runs_on", "continue_on_error", "is_matrix_deferred")
	if err != nil {
		return nil, fmt.Errorf("claim placeholder of job %d: %w", job.ID, err)
	}
	if affected != 1 {
		// A concurrent pass won the claim and owns the siblings. Restore the in-memory state so this
		// pass leaves the job alone and picks the winner's rows up on the next one.
		*job = beforeClaim
		return nil, nil
	}

	if len(siblings) == 0 {
		return nil, nil
	}
	for _, sibling := range siblings {
		if sibling.AttemptJobID != 0 {
			continue // matched a prior attempt above
		}
		if sibling.AttemptJobID, err = actions_model.GetNextAttemptJobID(ctx, job.RunID); err != nil {
			return nil, fmt.Errorf("alloc attempt_job_id for job %d: %w", job.ID, err)
		}
	}
	if err := db.Insert(ctx, siblings); err != nil {
		return nil, fmt.Errorf("insert matrix siblings of job %d: %w", job.ID, err)
	}
	return siblings, nil
}

// restoreDeferredMatrixPlaceholder rewinds a rerun clone of a dynamic-matrix combination into the unexpanded placeholder it grew from
func restoreDeferredMatrixPlaceholder(clone *actions_model.ActionRunJob) error {
	_, parsed, err := jobparser.ParseRawSingleWorkflow(clone.DeferredMatrixPayload)
	if err != nil {
		return fmt.Errorf("parse deferred matrix payload: %w", err)
	}
	clone.Name = util.EllipsisDisplayString(parsed.Name, 255)
	clone.WorkflowPayload = slices.Clone(clone.DeferredMatrixPayload)
	clone.RunsOn = parsed.RunsOn()
	clone.ContinueOnError = parsed.GetContinueOnError()
	clone.IsMatrixDeferred = true
	return nil
}
