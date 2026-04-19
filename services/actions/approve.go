// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

func ApproveRuns(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, runIDs []int64) error {
	updatedJobs := make([]*actions_model.ActionRunJob, 0)
	cancelledConcurrencyJobs := make([]*actions_model.ActionRunJob, 0)

	err := db.WithTx(ctx, func(ctx context.Context) (err error) {
		for _, runID := range runIDs {
			run, err := actions_model.GetRunByRepoAndID(ctx, repo.ID, runID)
			if err != nil {
				return err
			}
			run.NeedApproval = false
			run.ApprovedBy = doer.ID
			if err := actions_model.UpdateRun(ctx, run, "need_approval", "approved_by"); err != nil {
				return err
			}
			jobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(ctx, repo.ID, run.ID)
			if err != nil {
				return err
			}
			for _, job := range jobs {
				// Skip jobs with `needs`: they stay blocked until their dependencies finish,
				// at which point job_emitter will evaluate and start them.
				if len(job.Needs) > 0 {
					continue
				}
				var jobsToCancel []*actions_model.ActionRunJob
				job.Status, jobsToCancel, err = PrepareToStartJobWithConcurrency(ctx, job)
				if err != nil {
					return err
				}
				cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
				if job.Status == actions_model.StatusWaiting {
					n, err := actions_model.UpdateRunJob(ctx, job, nil, "status")
					if err != nil {
						return err
					}
					if n > 0 {
						updatedJobs = append(updatedJobs, job)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, updatedJobs)
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, cancelledConcurrencyJobs)

	EmitJobsIfReadyByJobs(cancelledConcurrencyJobs)

	return nil
}
