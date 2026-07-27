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
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"go.yaml.in/yaml/v4"
	"xorm.io/builder"
)

// expandDeferredMatrix expands a deferred-matrix placeholder once its needs are done and the job is
// known to run, using the needs' outputs: the placeholder becomes the first combination and the rest
// are returned as inserted siblings, all left Blocked so the caller's resolver still applies the
// concurrency gate.
//
// It runs inside the caller's transaction (job_emitter's resolver) and must not open a nested
// db.WithTx, which would reuse the ambient session and roll the whole emitter pass back on error.
// The three outcomes are reported through the job itself:
//   - expanded: IsMatrixDeferred is cleared and the job stays StatusBlocked.
//   - the workflow's fault (a matrix that cannot resolve, or one too large): a terminal status is
//     persisted here, reported by the job leaving StatusBlocked.
//   - not now: the job is left deferred and blocked for the next emitter pass to retry. This covers
//     a transient failure before anything was written, and losing the claim to a concurrent pass.
//
// A returned error is reserved for a failure after the placeholder was claimed and must roll the
// caller's transaction back: committing it would drop the remaining combinations for good.
func expandDeferredMatrix(ctx context.Context, job *actions_model.ActionRunJob, vars map[string]string) ([]*actions_model.ActionRunJob, error) {
	if !job.IsMatrixDeferred {
		return nil, nil
	}

	// failTerminal fails the job here rather than through the resolver's status map, which a
	// reusable caller (a job may be both) would drop: its branch only handles waiting and skipped.
	failTerminal := func(cause error) ([]*actions_model.ActionRunJob, error) {
		log.Warn("Matrix expansion failed for job %d (JobID: %s): %v", job.ID, job.JobID, cause)
		prevStatus, prevStopped := job.Status, job.Stopped
		job.IsMatrixDeferred, job.Status = false, actions_model.StatusFailure
		job.Stopped = timeutil.TimeStampNow()
		affected, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"is_matrix_deferred": true},
			"is_matrix_deferred", "status", "stopped")
		if err != nil {
			return nil, fmt.Errorf("fail deferred matrix job %d: %w", job.ID, err)
		}
		if affected != 1 {
			// A concurrent pass already advanced the row. Restore the in-memory state so this pass
			// does not report a failure that was never persisted.
			job.IsMatrixDeferred, job.Status, job.Stopped = true, prevStatus, prevStopped
		}
		return nil, nil
	}

	// retryLater leaves the placeholder untouched. It is only used before anything is written, so a
	// transient failure neither fails the job nor aborts the emitter pass for the whole run.
	retryLater := func(cause error) ([]*actions_model.ActionRunJob, error) {
		log.Error("Matrix expansion of job %d (JobID: %s) postponed to the next pass: %v", job.ID, job.JobID, cause)
		return nil, nil
	}

	// The resolver only calls this once every need is done, as it does for job concurrency.
	results, err := findJobNeedsAndFillJobResults(ctx, job)
	if err != nil {
		return retryLater(fmt.Errorf("find needs: %w", err))
	}

	if err := job.LoadAttributes(ctx); err != nil {
		return retryLater(fmt.Errorf("load attributes: %w", err))
	}

	// The payload still carries the raw, unevaluated matrix: planning only erases the needs.
	var baseSWF jobparser.SingleWorkflow
	if err := yaml.Unmarshal(job.WorkflowPayload, &baseSWF); err != nil {
		return failTerminal(fmt.Errorf("unmarshal payload: %w", err))
	}
	_, parsedJob := baseSWF.Job()
	if parsedJob == nil {
		return failTerminal(errors.New("payload contains no job"))
	}

	// `strategy` may reference the inputs context as well as needs, so resolve it like `if:` does.
	inputs, err := getInputsForJob(ctx, job.Run, job)
	if err != nil {
		return retryLater(fmt.Errorf("get inputs: %w", err))
	}
	// Job URLs encode an AttemptJobID below MaxJobNumPerRun, so a runtime fromJson() must not push
	// past it. The headroom is read before expanding, both because rejecting the expansion has to
	// leave the job as failTerminal expects to find it, and so an oversized output cannot make the
	// expansion materialize one job per combination before the limit is checked.
	maxAttemptJobID, err := actions_model.GetMaxAttemptJobID(ctx, job.RunID)
	if err != nil {
		return retryLater(fmt.Errorf("read attempt_job_id counter: %w", err))
	}
	// The placeholder already owns an id, so the last sibling takes maxAttemptJobID+len-1.
	maxCombinations := int(actions_model.MaxJobNumPerRun - maxAttemptJobID)

	giteaCtx := GenerateGiteaContext(ctx, job.Run, nil, job)
	expandedJobs, err := jobparser.ExpandMatrixWithNeeds(job.JobID, parsedJob, giteaCtx.ToGitHubContext(), results, vars, inputs, maxCombinations)
	if err != nil {
		return failTerminal(fmt.Errorf("expand matrix: %w", err))
	}
	// Combinations differ only in what the matrix feeds: the name, the payload, and a
	// runs-on/continue-on-error that may interpolate matrix.*.
	applyCombo := func(dst *actions_model.ActionRunJob, combo *jobparser.Job) error {
		swf := baseSWF
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
		sibling.ID, sibling.TaskID, sibling.SourceTaskID = 0, 0, 0
		sibling.Started, sibling.Stopped, sibling.IsMatrixDeferred = 0, 0, false
		sibling.Status = actions_model.StatusBlocked
		sibling.Needs = slices.Clone(job.Needs)
		if err := applyCombo(&sibling, combo); err != nil {
			return failTerminal(err)
		}
		siblings = append(siblings, &sibling)
	}

	// Reusing the placeholder leaves no phantom skipped job behind to poison downstream needs. The
	// conditional update is an atomic claim: only the caller that flips IsMatrixDeferred inserts.
	beforeClaim := *job
	if err := applyCombo(job, expandedJobs[0]); err != nil {
		*job = beforeClaim // failTerminal only persists the status, so do not keep the half-applied combination
		return failTerminal(err)
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
	// Allocate from the run-wide counter so the ids cannot collide with ones handed out later by
	// reusable-caller expansion or reruns. Allocating after the claim keeps a lost race from
	// burning ids out of the run's budget.
	for _, sibling := range siblings {
		if sibling.AttemptJobID, err = actions_model.GetNextAttemptJobID(ctx, job.RunID); err != nil {
			return nil, fmt.Errorf("alloc attempt_job_id for job %d: %w", job.ID, err)
		}
		if sibling.AttemptJobID >= actions_model.MaxJobNumPerRun {
			// The check above reserved this headroom, so another writer must have taken it in
			// between. Roll the pass back and let the next one re-check and fail the job for good.
			return nil, fmt.Errorf("attempt_job_id %d of job %d exceeds the per-run job limit of %d", sibling.AttemptJobID, job.ID, actions_model.MaxJobNumPerRun)
		}
	}
	if err := db.Insert(ctx, siblings); err != nil {
		return nil, fmt.Errorf("insert matrix siblings of job %d: %w", job.ID, err)
	}
	return siblings, nil
}
