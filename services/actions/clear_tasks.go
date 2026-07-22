// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/actions"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	webhook_module "gitea.dev/modules/webhook"
)

// StopZombieTasks stops tasks in running/cancelling status that haven't been updated for a long time
func StopZombieTasks(ctx context.Context) error {
	return stopTasksByStatuses(ctx, actions_model.FindTaskOptions{
		UpdatedBefore: timeutil.TimeStamp(time.Now().Add(-setting.Actions.ZombieTaskTimeout).Unix()),
	}, actions_model.StatusRunning, actions_model.StatusCancelling)
}

// StopEndlessTasks stops tasks in running/cancelling status with continuous updates that don't end for a long time
func StopEndlessTasks(ctx context.Context) error {
	return stopTasksByStatuses(ctx, actions_model.FindTaskOptions{
		StartedBefore: timeutil.TimeStamp(time.Now().Add(-setting.Actions.EndlessTaskTimeout).Unix()),
	}, actions_model.StatusRunning, actions_model.StatusCancelling)
}

func stopTasksByStatuses(ctx context.Context, opts actions_model.FindTaskOptions, statuses ...actions_model.Status) error {
	for _, status := range statuses {
		optsByStatus := opts
		optsByStatus.Status = status
		if err := stopTasks(ctx, optsByStatus); err != nil {
			return err
		}
	}

	return nil
}

func CancelPreviousJobs(ctx context.Context, repoID int64, ref, workflowID string, event webhook_module.HookEventType) error {
	jobs, err := actions_model.CancelPreviousJobs(ctx, repoID, ref, workflowID, event)
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, jobs)
	EmitJobsIfReadyByJobs(jobs)
	return err
}

func CleanRepoScheduleTasks(ctx context.Context, repo *repo_model.Repository) error {
	jobs, err := actions_model.CleanRepoScheduleTasks(ctx, repo)
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, jobs)
	EmitJobsIfReadyByJobs(jobs)
	return err
}

func shouldBlockJobByConcurrency(ctx context.Context, job *actions_model.ActionRunJob) (bool, error) {
	if job.RawConcurrency != "" && !job.IsConcurrencyEvaluated {
		// when the job depends on other jobs, we cannot evaluate its concurrency, so it should be blocked and will be evaluated again when its dependencies are done
		return true, nil
	}

	if job.ConcurrencyGroup == "" || job.ConcurrencyCancel {
		return false, nil
	}

	attempts, jobs, err := actions_model.GetConcurrentRunAttemptsAndJobs(ctx, job.RepoID, job.ConcurrencyGroup, []actions_model.Status{actions_model.StatusRunning, actions_model.StatusCancelling})
	if err != nil {
		return false, fmt.Errorf("GetConcurrentRunAttemptsAndJobs: %w", err)
	}

	return len(attempts) > 0 || len(jobs) > 0, nil
}

// PrepareToStartJobWithConcurrency prepares a job to start by its evaluated concurrency group and cancelling previous jobs if necessary.
// It returns the new status of the job (either StatusBlocked or StatusWaiting), any cancelled jobs, and any error encountered during the process.
func PrepareToStartJobWithConcurrency(ctx context.Context, job *actions_model.ActionRunJob) (actions_model.Status, []*actions_model.ActionRunJob, error) {
	shouldBlock, err := shouldBlockJobByConcurrency(ctx, job)
	if err != nil {
		return actions_model.StatusBlocked, nil, err
	}

	// even if the current job is blocked, we still need to cancel previous "waiting/blocked" jobs in the same concurrency group
	jobs, err := actions_model.CancelPreviousJobsByJobConcurrency(ctx, job)
	if err != nil {
		return actions_model.StatusBlocked, nil, fmt.Errorf("CancelPreviousJobsByJobConcurrency: %w", err)
	}

	return util.Iif(shouldBlock, actions_model.StatusBlocked, actions_model.StatusWaiting), jobs, nil
}

func shouldBlockRunByConcurrency(ctx context.Context, attempt *actions_model.ActionRunAttempt) (bool, error) {
	if attempt.ConcurrencyGroup == "" || attempt.ConcurrencyCancel {
		return false, nil
	}

	attempts, jobs, err := actions_model.GetConcurrentRunAttemptsAndJobs(ctx, attempt.RepoID, attempt.ConcurrencyGroup, []actions_model.Status{actions_model.StatusRunning, actions_model.StatusCancelling})
	if err != nil {
		return false, fmt.Errorf("find concurrent runs and jobs: %w", err)
	}

	return len(attempts) > 0 || len(jobs) > 0, nil
}

// PrepareToStartRunWithConcurrency prepares a run attempt to start by its evaluated concurrency group and cancelling previous jobs if necessary.
// It returns the new status of the run attempt (either StatusBlocked or StatusWaiting), any cancelled jobs, and any error encountered during the process.
func PrepareToStartRunWithConcurrency(ctx context.Context, attempt *actions_model.ActionRunAttempt) (actions_model.Status, []*actions_model.ActionRunJob, error) {
	shouldBlock, err := shouldBlockRunByConcurrency(ctx, attempt)
	if err != nil {
		return actions_model.StatusBlocked, nil, err
	}

	// even if the current run is blocked, we still need to cancel previous "waiting/blocked" jobs in the same concurrency group
	jobs, err := actions_model.CancelPreviousJobsByRunConcurrency(ctx, attempt)
	if err != nil {
		return actions_model.StatusBlocked, nil, fmt.Errorf("CancelPreviousJobsByRunConcurrency: %w", err)
	}

	return util.Iif(shouldBlock, actions_model.StatusBlocked, actions_model.StatusWaiting), jobs, nil
}

func stopTasks(ctx context.Context, opts actions_model.FindTaskOptions) error {
	tasks, err := db.Find[actions_model.ActionTask](ctx, opts)
	if err != nil {
		return fmt.Errorf("find tasks: %w", err)
	}

	jobs := make([]*actions_model.ActionRunJob, 0, len(tasks))
	for _, task := range tasks {
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			stopStatus := actions_model.StatusFailure
			if task.Status == actions_model.StatusCancelling {
				stopStatus = actions_model.StatusCancelled
			}
			if err := actions_model.StopTask(ctx, task.ID, stopStatus); err != nil {
				return err
			}
			if err := task.LoadJob(ctx); err != nil {
				return err
			}
			jobs = append(jobs, task.Job)
			return nil
		}); err != nil {
			log.Warn("Cannot stop task %v: %v", task.ID, err)
			continue
		}

		remove, err := actions.TransferLogs(ctx, task.LogFilename)
		if err != nil {
			log.Warn("Cannot transfer logs of task %v: %v", task.ID, err)
			continue
		}
		task.LogInStorage = true
		if err := actions_model.UpdateTask(ctx, task, "log_in_storage"); err != nil {
			log.Warn("Cannot update task %v: %v", task.ID, err)
			continue
		}
		remove()
	}

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, jobs)
	EmitJobsIfReadyByJobs(jobs)

	return nil
}

// CancelAbandonedJobs cancels jobs that have not been picked by any runner for a long time
func CancelAbandonedJobs(ctx context.Context) error {
	abandonedJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{
		Statuses:      []actions_model.Status{actions_model.StatusWaiting, actions_model.StatusBlocked},
		UpdatedBefore: timeutil.TimeStampNow().AddDuration(-setting.Actions.AbandonedJobTimeout),
	})
	if err != nil {
		log.Warn("find abandoned jobs: %v", err)
		return err
	}

	updatedJobs, err := actions_model.CancelJobs(ctx, abandonedJobs)
	if err != nil {
		log.Warn("cancel abandoned jobs: %v", err)
	}

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, updatedJobs)
	EmitJobsIfReadyByJobs(updatedJobs)

	return nil
}
