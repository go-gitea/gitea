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
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ApproveRuns returns the approved runs in the same order as runIDs.
func ApproveRuns(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, runIDs []int64) ([]*actions_model.ActionRun, error) {
	updatedJobs := make([]*actions_model.ActionRunJob, 0)
	cancelledConcurrencyJobs := make([]*actions_model.ActionRunJob, 0)
	approvedRunIDs := make(container.Set[int64])

	err := db.WithTx(ctx, func(ctx context.Context) (err error) {
		for _, runID := range runIDs {
			run, err := actions_model.GetRunByRepoAndID(ctx, repo.ID, runID)
			if err != nil {
				return err
			}
			if !run.IsAwaitingApproval() {
				continue
			}
			run.NeedApproval = false
			run.ApprovedBy = doer.ID
			if err := actions_model.UpdateRun(ctx, run, "need_approval", "approved_by"); err != nil {
				return err
			}
			approvedRunIDs.Add(run.ID)
			jobs, err := actions_model.GetLatestAttemptJobsByRun(ctx, run)
			if err != nil {
				return err
			}
			attempt, hasAttempt, err := run.GetLatestAttempt(ctx)
			if err != nil {
				return fmt.Errorf("get latest attempt of run %d: %w", run.ID, err)
			}
			if hasAttempt && attempt.ConcurrencyGroup != "" {
				status, jobsToCancel, err := PrepareToStartRunWithConcurrency(ctx, attempt)
				if err != nil {
					return err
				}
				cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
				if status == actions_model.StatusBlocked {
					continue
				}
			}

			// approval unblocks every job at once, so max-parallel has to cap them here too
			slots := maxParallelSlots{}
			for _, job := range jobs {
				slots.hold(job, job.Status)
			}

			for _, job := range jobs {
				// Only approval-blocked jobs are released here, job_emitter starts those with `needs`
				if job.Status != actions_model.StatusBlocked || len(job.Needs) > 0 {
					continue
				}
				if slices.ContainsFunc(cancelledConcurrencyJobs, func(cancelled *actions_model.ActionRunJob) bool { return cancelled.ID == job.ID }) {
					continue // cancelled by a sibling's concurrency in this loop
				}
				// A slot-starved job cannot start, skip the following checks.
				if !slots.available(job) {
					continue
				}
				var jobsToCancel []*actions_model.ActionRunJob
				job.Status, jobsToCancel, err = PrepareToStartJobWithConcurrency(ctx, job)
				if err != nil {
					return err
				}
				cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
				applyMaxParallel(job, slots)
				if job.Status != actions_model.StatusWaiting {
					continue
				}
				n, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"status": actions_model.StatusBlocked}, "status")
				if err != nil {
					return err
				}
				if n == 0 {
					continue
				}
				updatedJobs = append(updatedJobs, job)

				// A top-level reusable caller was just unblocked by approval, expand it
				if job.IsReusableCaller && !job.IsExpanded {
					if !hasAttempt {
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
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// The emitter skipped these runs while they awaited approval
	for runID := range approvedRunIDs {
		if err := EmitJobsIfReadyByRun(runID); err != nil {
			log.Error("emit run %d after approval: %v", runID, err)
		}
	}

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, updatedJobs)
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, cancelledConcurrencyJobs)

	EmitJobsIfReadyByJobs(cancelledConcurrencyJobs)

	// The batches above already notified every run whose jobs changed, which is the only way
	// approving alters a run's status, so reload purely to answer the caller.
	reloaded, err := actions_model.GetRunsByRepoAndID(ctx, repo.ID, runIDs)
	if err != nil {
		return nil, fmt.Errorf("GetRunsByRepoAndID: %w", err)
	}
	runsByID := make(map[int64]*actions_model.ActionRun, len(reloaded))
	for _, run := range reloaded {
		run.Repo = repo // the caller resolved runIDs against this repo, so spare every consumer a reload
		runsByID[run.ID] = run
	}

	approvedRuns := make([]*actions_model.ActionRun, 0, len(runIDs))
	for _, runID := range runIDs {
		run := runsByID[runID]
		if run == nil {
			return nil, util.NewNotExistErrorf("run %d no longer exists after approval", runID)
		}
		approvedRuns = append(approvedRuns, run)
	}

	return approvedRuns, nil
}
