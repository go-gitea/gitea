// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ResetRunTimes resets the start and stop times for a run when it is done, for rerun
func ResetRunTimes(ctx context.Context, run *actions_model.ActionRun) error {
	if run.Status.IsDone() {
		run.PreviousDuration = run.Duration()
		run.Started = 0
		run.Stopped = 0
		return actions_model.UpdateRun(ctx, run, "started", "stopped", "previous_duration")
	}
	return nil
}

// RerunJob reruns a job, handling concurrency and status updates
func RerunJob(ctx context.Context, job *actions_model.ActionRunJob, shouldBlock bool) error {
	status := job.Status
	if !status.IsDone() || !job.Run.Status.IsDone() {
		return nil
	}

	job.TaskID = 0
	job.Status = util.Iif(shouldBlock, actions_model.StatusBlocked, actions_model.StatusWaiting)
	job.Started = 0
	job.Stopped = 0

	job.ConcurrencyGroup = ""
	job.ConcurrencyCancel = false
	job.IsConcurrencyEvaluated = false
	if err := job.LoadRun(ctx); err != nil {
		return err
	}

	vars, err := actions_model.GetVariablesOfRun(ctx, job.Run)
	if err != nil {
		return fmt.Errorf("get run %d variables: %w", job.Run.ID, err)
	}

	if job.RawConcurrency != "" && !shouldBlock {
		err = EvaluateJobConcurrencyFillModel(ctx, job.Run, job, vars, nil)
		if err != nil {
			return fmt.Errorf("evaluate job concurrency: %w", err)
		}

		job.Status, err = PrepareToStartJobWithConcurrency(ctx, job)
		if err != nil {
			return err
		}
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		updateCols := []string{"task_id", "status", "started", "stopped", "concurrency_group", "concurrency_cancel", "is_concurrency_evaluated"}
		_, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"status": status}, updateCols...)
		return err
	}); err != nil {
		return err
	}

	CreateCommitStatusForRunJobs(ctx, job.Run, job)

	return nil
}

// GetAllRerunJobs get all jobs that need to be rerun when job should be rerun
func GetAllRerunJobs(job *actions_model.ActionRunJob, allJobs []*actions_model.ActionRunJob) []*actions_model.ActionRunJob {
	rerunJobs := []*actions_model.ActionRunJob{job}
	rerunJobsIDSet := make(container.Set[string])
	rerunJobsIDSet.Add(job.JobID)

	for {
		found := false
		for _, j := range allJobs {
			if rerunJobsIDSet.Contains(j.JobID) {
				continue
			}
			for _, need := range j.Needs {
				if rerunJobsIDSet.Contains(need) {
					found = true
					rerunJobs = append(rerunJobs, j)
					rerunJobsIDSet.Add(j.JobID)
					break
				}
			}
		}
		if !found {
			break
		}
	}

	return rerunJobs
}
