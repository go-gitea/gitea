// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
)

// GetFailedRerunJobs returns the failed or cancelled jobs in a run.
func GetFailedRerunJobs(allJobs []*actions_model.ActionRunJob) []*actions_model.ActionRunJob {
	var jobsToRerun []*actions_model.ActionRunJob

	for _, job := range allJobs {
		if job.Status == actions_model.StatusFailure || job.Status == actions_model.StatusCancelled {
			jobsToRerun = append(jobsToRerun, job)
		}
	}

	return jobsToRerun
}

// RerunWorkflowRunJobs reruns the given jobs of a workflow run.
// An empty jobsToRerun means rerunning the whole run. Otherwise jobsToRerun contains only the user-requested target jobs;
// downstream dependent jobs are expanded internally while building the rerun plan.
func RerunWorkflowRunJobs(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun, jobsToRerun []*actions_model.ActionRunJob) (*actions_model.ActionRunAttempt, error) {
	plan, err := buildRerunPlan(ctx, repo, run, jobsToRerun)
	if err != nil {
		return nil, err
	}
	return execRerunPlan(ctx, plan)
}
