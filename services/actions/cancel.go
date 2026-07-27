// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
)

// CancelRun cancels all cancellable jobs in a run, updates commit statuses,
// and fires downstream notifications including job-emitter queue entries.
// It returns the run's post-cancellation state so callers don't need to re-fetch it.
func CancelRun(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) (*actions_model.ActionRun, error) {
	var updatedJobs []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		cancelled, err := actions_model.CancelJobs(ctx, jobs)
		if err != nil {
			return fmt.Errorf("CancelJobs: %w", err)
		}
		updatedJobs = cancelled
		return nil
	}); err != nil {
		return nil, err
	}

	CreateCommitStatusForRunJobs(ctx, run, jobs...)
	EmitJobsIfReadyByJobs(updatedJobs)
	NotifyWorkflowJobsStatusUpdate(ctx, updatedJobs...)
	if len(updatedJobs) == 0 {
		return run, nil
	}

	reloaded, err := actions_model.GetRunByRepoAndID(ctx, run.RepoID, run.ID)
	if err != nil {
		return nil, fmt.Errorf("GetRunByRepoAndID: %w", err)
	}
	NotifyWorkflowRunStatusUpdate(ctx, reloaded)
	return reloaded, nil
}
