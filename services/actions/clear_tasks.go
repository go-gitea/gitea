// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

// StopZombieTasks stops the task which have running status, but haven't been updated for a long time
func StopZombieTasks(ctx context.Context) error {
	return stopTasks(ctx, actions_model.FindTaskOptions{
		Status:        actions_model.StatusRunning,
		UpdatedBefore: timeutil.TimeStamp(time.Now().Add(-setting.Actions.ZombieTaskTimeout).Unix()),
	})
}

// StopEndlessTasks stops the tasks which have running status and continuous updates, but don't end for a long time
func StopEndlessTasks(ctx context.Context) error {
	return stopTasks(ctx, actions_model.FindTaskOptions{
		Status:        actions_model.StatusRunning,
		StartedBefore: timeutil.TimeStamp(time.Now().Add(-setting.Actions.EndlessTaskTimeout).Unix()),
	})
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

	attempts, jobs, err := actions_model.GetConcurrentRunAttemptsAndJobs(ctx, job.RepoID, job.ConcurrencyGroup, []actions_model.Status{actions_model.StatusRunning})
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

	attempts, jobs, err := actions_model.GetConcurrentRunAttemptsAndJobs(ctx, attempt.RepoID, attempt.ConcurrencyGroup, []actions_model.Status{actions_model.StatusRunning})
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
			if err := actions_model.StopTask(ctx, task.ID, actions_model.StatusFailure); err != nil {
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
	jobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{
		Statuses:      []actions_model.Status{actions_model.StatusWaiting, actions_model.StatusBlocked},
		UpdatedBefore: timeutil.TimeStampNow().AddDuration(-setting.Actions.AbandonedJobTimeout),
	})
	if err != nil {
		log.Warn("find abandoned tasks: %v", err)
		return err
	}

	now := timeutil.TimeStampNow()

	updatedJobs := []*actions_model.ActionRunJob{}

	for _, job := range jobs {
		job.Status = actions_model.StatusCancelled
		job.Stopped = now
		updated := false
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			n, err := actions_model.UpdateRunJob(ctx, job, nil, "status", "stopped")
			if err != nil {
				return err
			}
			if err := job.LoadAttributes(ctx); err != nil {
				return err
			}
			updated = n > 0
			return nil
		}); err != nil {
			log.Warn("cancel abandoned job %v: %v", job.ID, err)
			// go on
		}
		if job.Run == nil || job.Run.Repo == nil {
			continue // error occurs during loading attributes, the following code that depends on "Run.Repo" will fail, so ignore and skip
		}
		if updated {
			CreateCommitStatusForRunJobs(ctx, job.Run, job)
			updatedJobs = append(updatedJobs, job)
		}
	}

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, updatedJobs)
	EmitJobsIfReadyByJobs(updatedJobs)

	return nil
}
