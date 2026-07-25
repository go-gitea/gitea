// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
)

func ApproveRuns(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, runIDs []int64) error {
	updatedJobs := make([]*actions_model.ActionRunJob, 0)
	cancelledConcurrencyJobs := make([]*actions_model.ActionRunJob, 0)
	// Track runs whose reusable callers were just expanded so we can re-emit after the tx commits.
	expandedCallerRunIDs := make(container.Set[int64])

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
				if job.Status != actions_model.StatusWaiting {
					continue
				}
				n, err := actions_model.UpdateRunJob(ctx, job, nil, "status")
				if err != nil {
					return err
				}
				if n == 0 {
					continue
				}
				updatedJobs = append(updatedJobs, job)

				// A top-level reusable caller was just unblocked by approval, expand it
				if job.IsReusableCaller && !job.IsExpanded {
					attempt, has, err := run.GetLatestAttempt(ctx)
					if err != nil {
						return fmt.Errorf("get latest attempt of run %d: %w", run.ID, err)
					}
					if !has {
						return errors.New("run has no attempt")
					}
					vars, err := actions_model.GetVariablesOfRun(ctx, run)
					if err != nil {
						return err
					}
					if err := expandReusableWorkflowCaller(ctx, run, attempt, job, vars); err != nil {
						return fmt.Errorf("expand caller %d on approval: %w", job.ID, err)
					}
					if err := actions_model.RefreshReusableCallerStatus(ctx, job); err != nil {
						return fmt.Errorf("refresh caller %d status after approval-time expansion: %w", job.ID, err)
					}
					expandedCallerRunIDs.Add(run.ID)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Re-emit AFTER the tx commits so the newly inserted callee rows transition Blocked -> Waiting.
	for runID := range expandedCallerRunIDs {
		if err := EmitJobsIfReadyByRun(runID); err != nil {
			log.Error("emit run %d after approval-time caller expansion: %v", runID, err)
		}
	}

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, updatedJobs)
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, cancelledConcurrencyJobs)

	EmitJobsIfReadyByJobs(cancelledConcurrencyJobs)

	return nil
}
