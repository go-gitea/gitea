// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
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
		updatedJobs, err = actions_model.CancelJobs(ctx, jobs, force)
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
