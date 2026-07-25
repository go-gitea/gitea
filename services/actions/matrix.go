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
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"go.yaml.in/yaml/v4"
	"xorm.io/builder"
)

// expandDeferredMatrix expands a deferred-matrix placeholder once its needs are done, using their
// outputs: the placeholder becomes the first combination and the rest are returned as inserted
// siblings, all left Blocked so the caller's resolver still applies the `if:`/concurrency gates.
//
// It runs inside the caller's transaction (job_emitter's resolver) and must not open a nested
// db.WithTx, which would reuse the ambient session and roll the whole emitter pass back on error.
// A returned error is always an infrastructure failure and must roll that transaction back, since
// the placeholder may already be claimed; a bad matrix is the workflow's fault and is instead
// persisted as a terminal job status, reported by job.Status leaving StatusBlocked.
func expandDeferredMatrix(ctx context.Context, job *actions_model.ActionRunJob, vars map[string]string) ([]*actions_model.ActionRunJob, error) {
	if !job.IsMatrixDeferred {
		return nil, nil
	}

	// failTerminal fails the job here rather than through the resolver's status map, which a
	// reusable caller (a job may be both) would drop: its branch only handles waiting and skipped.
	failTerminal := func(cause error) ([]*actions_model.ActionRunJob, error) {
		log.Warn("Matrix expansion failed for job %d (JobID: %s): %v", job.ID, job.JobID, cause)
		job.IsMatrixDeferred, job.Status = false, actions_model.StatusFailure
		job.Stopped = timeutil.TimeStampNow()
		if _, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"is_matrix_deferred": true},
			"is_matrix_deferred", "status", "stopped"); err != nil {
			return nil, fmt.Errorf("fail deferred matrix job %d: %w", job.ID, err)
		}
		return nil, nil
	}

	// The resolver only calls this once every need is done, as it does for job concurrency.
	results, err := findJobNeedsAndFillJobResults(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("find needs of job %d: %w", job.ID, err)
	}

	if err := job.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("load attributes of job %d: %w", job.ID, err)
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
		return nil, fmt.Errorf("get inputs for job %d: %w", job.ID, err)
	}
	giteaCtx := GenerateGiteaContext(ctx, job.Run, nil, job)
	expandedJobs, err := jobparser.ExpandMatrixWithNeeds(job.JobID, parsedJob, giteaCtx.ToGitHubContext(), results, vars, inputs)
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
		if err := applyCombo(&sibling, combo); err != nil {
			return failTerminal(err)
		}
		// Allocate from the run-wide counter so the ids cannot collide with ones handed out later by
		// reusable-caller expansion or reruns. Job URLs encode an AttemptJobID below
		// MaxJobNumPerRun, so a runtime fromJson() must not push past it.
		if sibling.AttemptJobID, err = actions_model.GetNextAttemptJobID(ctx, job.RunID); err != nil {
			return nil, fmt.Errorf("alloc attempt_job_id for job %d: %w", job.ID, err)
		}
		if sibling.AttemptJobID >= actions_model.MaxJobNumPerRun {
			return failTerminal(fmt.Errorf("expanding the matrix to %d combinations exceeds the per-run job limit of %d", len(expandedJobs), actions_model.MaxJobNumPerRun))
		}
		siblings = append(siblings, &sibling)
	}

	// Reusing the placeholder leaves no phantom skipped job behind to poison downstream needs. The
	// conditional update is an atomic claim: only the caller that flips IsMatrixDeferred inserts.
	if err := applyCombo(job, expandedJobs[0]); err != nil {
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
		// Another concurrent caller won the claim; leave the siblings to it.
		return nil, nil
	}

	if len(siblings) == 0 {
		return nil, nil
	}
	if err := db.Insert(ctx, siblings); err != nil {
		return nil, fmt.Errorf("insert matrix siblings of job %d: %w", job.ID, err)
	}
	return siblings, nil
}
