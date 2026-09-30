// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	"gitea.dev/actionslib/pkg/expreval"
	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/actions/jobparser"
)

// CancelRun cancels a run's cancellable jobs and returns the run's post-cancellation state.
// A runner that supports it gets to run its post-cancel cleanup before the job reaches its final status.
func CancelRun(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) (*actions_model.ActionRun, error) {
	return cancelRun(ctx, run, jobs, false)
}

// ForceCancelRun cancels a run like CancelRun, but does not wait for the runners to acknowledge it:
// the jobs are marked cancelled at once and whatever a runner reports for them afterwards is discarded.
func ForceCancelRun(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) (*actions_model.ActionRun, error) {
	return cancelRun(ctx, run, jobs, true)
}

func cancelRun(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob, force bool) (*actions_model.ActionRun, error) {
	var updatedJobs []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(ctx context.Context) (err error) {
		updatedJobs, err = actions_model.CancelJobs(ctx, cancellableJobs(run, jobs, force), force)
		if err != nil {
			return fmt.Errorf("CancelJobs: %w", err)
		}
		if len(updatedJobs) > 0 {
			return nil // a job update already refreshed the run
		}
		return actions_model.SettleRunAfterCancel(ctx, run)
	}); err != nil {
		return nil, err
	}

	// updatedJobs, not jobs: cancelOneJob re-reads the cancelled rows, the input ones still carry their pre-cancel status
	CreateCommitStatusForRunJobs(ctx, run, updatedJobs...)
	EmitJobsIfReadyByJobs(updatedJobs)
	NotifyWorkflowJobsStatusUpdate(ctx, updatedJobs...)

	reloaded, err := actions_model.GetRunByRepoAndID(ctx, run.RepoID, run.ID)
	if err != nil {
		return nil, fmt.Errorf("GetRunByRepoAndID: %w", err)
	}
	if len(updatedJobs) > 0 || reloaded.Status != run.Status {
		NotifyWorkflowRunStatusUpdate(ctx, reloaded)
	}
	return reloaded, nil
}

func cancellableJobs(run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob, force bool) []*actions_model.ActionRunJob {
	if force || (run.Started.IsZero() && !run.Status.In(actions_model.StatusRunning, actions_model.StatusCancelling)) {
		return jobs
	}
	toCancel := make([]*actions_model.ActionRunJob, 0, len(jobs))
	for _, job := range jobs {
		if !runsAfterCancellation(job) {
			toCancel = append(toCancel, job)
		}
	}
	return toCancel
}

func runsAfterCancellation(job *actions_model.ActionRunJob) bool {
	if job.Status.IsDone() {
		return false
	}
	parsed, err := job.ParseJob()
	if err != nil || parsed.If.Value == "" {
		return false
	}
	condition := jobparser.IfExpression(parsed.If.Value)
	return expreval.CallsFunction(condition, "always") && !expreval.CallsFunction(condition, "cancelled")
}
