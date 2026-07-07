// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
)

// CancelRunJobs cancels the provided workflow run jobs and emits the related status updates.
func CancelRunJobs(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) error {
	var updatedJobs []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		cancelledJobs, err := actions_model.CancelJobs(ctx, jobs)
		if err != nil {
			return fmt.Errorf("cancel jobs: %w", err)
		}
		updatedJobs = append(updatedJobs, cancelledJobs...)
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
