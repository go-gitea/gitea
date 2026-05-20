// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
)

// CancelRun cancels all cancellable jobs in a run, updates commit statuses,
// and fires downstream notifications including job-emitter queue entries.
func CancelRun(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) error {
	var updatedJobs []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		cancelled, err := actions_model.CancelJobs(ctx, jobs)
		if err != nil {
			return fmt.Errorf("CancelJobs: %w", err)
		}
		updatedJobs = cancelled
		return nil
	}); err != nil {
		return err
	}

	CreateCommitStatusForRunJobs(ctx, run, jobs...)
	EmitJobsIfReadyByJobs(updatedJobs)
	NotifyWorkflowJobsStatusUpdate(ctx, updatedJobs...)
	if len(updatedJobs) > 0 {
		NotifyWorkflowRunStatusUpdateWithReload(ctx, run.RepoID, run.ID)
	}
	return nil
}
