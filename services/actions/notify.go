// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

// NotifyWorkflowJobsAndRunsStatusUpdate notifies status changes for a batch of jobs and the runs they affect.
// Use it when a workflow operation updates multiple jobs and runs.
func NotifyWorkflowJobsAndRunsStatusUpdate(ctx context.Context, jobs []*actions_model.ActionRunJob) {
	if len(jobs) == 0 {
		return
	}

	// The input jobs may belong to different runs, so track each affected run.
	runs := make(map[int64]*actions_model.ActionRun, len(jobs))
	jobsByRunID := make(map[int64][]*actions_model.ActionRunJob)

	for _, job := range jobs {
		if err := job.LoadAttributes(ctx); err != nil {
			log.Error("Failed to load job attributes: %v", err)
			continue
		}
		CreateCommitStatusForRunJobs(ctx, job.Run, job)

		if _, ok := runs[job.RunID]; !ok {
			runs[job.RunID] = job.Run
		}
		if _, ok := jobsByRunID[job.RunID]; !ok {
			jobsByRunID[job.RunID] = make([]*actions_model.ActionRunJob, 0)
		}
		jobsByRunID[job.RunID] = append(jobsByRunID[job.RunID], job)
	}

	for _, run := range runs {
		NotifyWorkflowRunStatusUpdate(ctx, run)
	}

	for _, jobs := range jobsByRunID {
		NotifyWorkflowJobsStatusUpdate(ctx, jobs...)
	}
}

// NotifyWorkflowRunStatusUpdateWithReload reloads the run before notifying its status update.
// Use it when only repo/run IDs are available or when the in-memory run may be stale after job updates.
func NotifyWorkflowRunStatusUpdateWithReload(ctx context.Context, repoID, runID int64) {
	run, err := actions_model.GetRunByRepoAndID(ctx, repoID, runID)
	if err != nil {
		log.Error("GetRunByRepoAndID: %v", err)
		return
	}
	NotifyWorkflowRunStatusUpdate(ctx, run)
}

// NotifyWorkflowRunStatusUpdate notifies a run status update using the latest attempt trigger user when available.
// Use it for run-level notifications when the caller already has the run model loaded.
func NotifyWorkflowRunStatusUpdate(ctx context.Context, run *actions_model.ActionRun) {
	if err := run.LoadAttributes(ctx); err != nil {
		log.Error("run.LoadAttributes: %v", err)
		return
	}
	triggerUser := run.TriggerUser
	if run.LatestAttemptID > 0 {
		attempt, err := actions_model.GetRunAttemptByRepoAndID(ctx, run.RepoID, run.LatestAttemptID)
		if err != nil {
			log.Error("GetRunAttemptByRepoAndID: %v", err)
			return
		}
		if err := attempt.LoadAttributes(ctx); err != nil {
			log.Error("attempt.LoadAttributes: %v", err)
			return
		}
		triggerUser = attempt.TriggerUser
	}
	notify_service.WorkflowRunStatusUpdate(ctx, run.Repo, triggerUser, run)
}

// NotifyWorkflowJobsStatusUpdate notifies status updates for jobs without task.
// Use it for batch or single-job notifications after state changes.
func NotifyWorkflowJobsStatusUpdate(ctx context.Context, jobs ...*actions_model.ActionRunJob) {
	jobsByAttempt := make(map[int64][]*actions_model.ActionRunJob)
	for _, job := range jobs {
		if _, ok := jobsByAttempt[job.RunAttemptID]; !ok {
			jobsByAttempt[job.RunAttemptID] = make([]*actions_model.ActionRunJob, 0)
		}
		jobsByAttempt[job.RunAttemptID] = append(jobsByAttempt[job.RunAttemptID], job)
	}

	for attemptID, js := range jobsByAttempt {
		if attemptID == 0 {
			for _, job := range js {
				if err := job.LoadAttributes(ctx); err != nil {
					log.Error("job.LoadAttributes: %v", err)
					continue
				}
				notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, nil)
			}
			continue
		}

		attempt, err := actions_model.GetRunAttemptByRepoAndID(ctx, js[0].RepoID, attemptID)
		if err != nil {
			log.Error("GetRunAttemptByRepoAndID: %v", err)
			continue
		}
		if err := attempt.LoadAttributes(ctx); err != nil {
			log.Error("attempt.LoadAttributes: %v", err)
			continue
		}
		for _, job := range js {
			notify_service.WorkflowJobStatusUpdate(ctx, attempt.Run.Repo, attempt.TriggerUser, job, nil)
		}
	}
}

// NotifyWorkflowJobStatusUpdateWithTask notifies a single job status update when a concrete task is available.
// Use it for runner/task lifecycle callbacks so the notification includes the originating task context.
func NotifyWorkflowJobStatusUpdateWithTask(ctx context.Context, job *actions_model.ActionRunJob, task *actions_model.ActionTask) {
	if job.RunAttemptID == 0 {
		if err := job.LoadAttributes(ctx); err != nil {
			log.Error("job.LoadAttributes: %v", err)
			return
		}
		notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, task)
		return
	}

	attempt, err := actions_model.GetRunAttemptByRepoAndID(ctx, job.RepoID, job.RunAttemptID)
	if err != nil {
		log.Error("GetRunAttemptByRepoAndID: %v", err)
		return
	}
	if err := attempt.LoadAttributes(ctx); err != nil {
		log.Error("attempt.LoadAttributes: %v", err)
		return
	}
	notify_service.WorkflowJobStatusUpdate(ctx, attempt.Run.Repo, attempt.TriggerUser, job, task)
}
