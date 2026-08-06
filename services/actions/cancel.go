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
func CancelRun(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) (*actions_model.ActionRun, error) {
	var updatedJobs []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(ctx context.Context) (err error) {
		updatedJobs, err = actions_model.CancelJobs(ctx, jobs)
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

	CreateCommitStatusForRunJobs(ctx, run, jobs...)
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
