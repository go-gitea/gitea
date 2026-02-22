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
	notify_service "code.gitea.io/gitea/services/notify"
	"xorm.io/builder"

	"github.com/nektos/act/pkg/model"
	"go.yaml.in/yaml/v4"
)

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

func Rerun(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob, job *actions_model.ActionRunJob) error {
	if run.Status.IsDone() {
		// reset run's start and stop time
		run.PreviousDuration = run.Duration()
		run.Started = 0
		run.Stopped = 0
	}
	newRunStatus := actions_model.StatusWaiting
	if run.ParentJobID > 0 {
		if err := run.LoadParentJob(ctx); err != nil {
			return err
		}
		if run.ParentJob.Status.IsBlocked() {
			newRunStatus = actions_model.StatusBlocked
		}
	}
	run.Status = newRunStatus

	if run.RawConcurrency != "" {
		if run.Status.IsBlocked() {
			run.ConcurrencyGroup = ""
			run.ConcurrencyCancel = false
		} else {
			var rawConcurrency model.RawConcurrency
			if err := yaml.Unmarshal([]byte(run.RawConcurrency), &rawConcurrency); err != nil {
				return fmt.Errorf("unmarshal raw concurrency: %w", err)
			}

			vars, err := actions_model.GetVariablesOfRun(ctx, run)
			if err != nil {
				return fmt.Errorf("get run %d variables: %w", run.ID, err)
			}

			err = EvaluateRunConcurrencyFillModel(ctx, run, &rawConcurrency, vars)
			if err != nil {
				return fmt.Errorf("evaluate run concurrency: %w", err)
			}

			run.Status, err = PrepareToStartRunWithConcurrency(ctx, run)
			if err != nil {
				return err
			}
		}
	}
	if err := actions_model.UpdateRun(ctx, run, "started", "stopped", "previous_duration", "status", "concurrency_group", "concurrency_cancel"); err != nil {
		return fmt.Errorf("update run: %w", err)
	}

	if err := run.LoadAttributes(ctx); err != nil {
		return err
	}
	notify_service.WorkflowRunStatusUpdate(ctx, run.Repo, run.TriggerUser, run)

	isRunBlocked := run.Status == actions_model.StatusBlocked
	if job == nil { // rerun all jobs
		for _, j := range jobs {
			// if the job has needs, it should be set to "blocked" status to wait for other jobs
			shouldBlockJob := len(j.Needs) > 0 || isRunBlocked
			if err := rerunJob(ctx, j, shouldBlockJob); err != nil {
				return fmt.Errorf("rerun job: %w", err)
			}
		}
		return nil
	}

	jobsToRerun := GetAllRerunJobs(job, jobs)
	for _, j := range jobsToRerun {
		// jobs other than the specified one should be set to "blocked" status
		shouldBlockJob := j.JobID != job.JobID || isRunBlocked
		if err := rerunJob(ctx, j, shouldBlockJob); err != nil {
			return fmt.Errorf("rerun job: %w", err)
		}
	}

	// If this is a child run, rerun the parent job and its downstream jobs
	if run.ParentJobID > 0 {
		parentJob := run.ParentJob
		if err := parentJob.LoadRun(ctx); err != nil {
			return err
		}
		parentRunJobs, err := actions_model.GetRunJobsByRunID(ctx, parentJob.RunID)
		if err != nil {
			return fmt.Errorf("get parent run jobs: %w", err)
		}
		if err := Rerun(ctx, parentJob.Run, parentRunJobs, parentJob); err != nil {
			return fmt.Errorf("rerun parent run: %w", err)
		}
	}

	return nil
}

func rerunJob(ctx context.Context, job *actions_model.ActionRunJob, shouldBlock bool) error {
	oldStatus := job.Status
	newStatus := util.Iif(shouldBlock, actions_model.StatusBlocked, actions_model.StatusWaiting)

	if oldStatus == newStatus {
		return nil
	}

	if oldStatus.IsDone() {
		job.TaskID = 0
		job.Started = 0
		job.Stopped = 0
		job.ConcurrencyGroup = ""
		job.ConcurrencyCancel = false
		job.IsConcurrencyEvaluated = false
	}
	job.Status = newStatus

	if err := job.LoadAttributes(ctx); err != nil {
		return err
	}

	if job.RawConcurrency != "" && !shouldBlock {
		vars, err := actions_model.GetVariablesOfRun(ctx, job.Run)
		if err != nil {
			return fmt.Errorf("get run %d variables: %w", job.Run.ID, err)
		}

		err = EvaluateJobConcurrencyFillModel(ctx, job.Run, job, vars)
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
		_, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"status": oldStatus}, updateCols...)
		return err
	}); err != nil {
		return err
	}

	// if the job uses a reusable workflow and the child run has been created
	if job.ChildRunID > 0 {
		// job status may be StatusWaiting or StatusBlocked when calling rerunReusableWorkflowRun
		if err := rerunReusableWorkflowRun(ctx, job); err != nil {
			return fmt.Errorf("rerunReusableWorkflowRun: %w", err)
		}
	}

	CreateCommitStatusForRunJobs(ctx, job.Run, job)
	notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, nil)

	return nil
}

func rerunReusableWorkflowRun(ctx context.Context, parentJob *actions_model.ActionRunJob) error {
	childRun, err := actions_model.GetRunByRepoAndID(ctx, parentJob.RepoID, parentJob.ChildRunID)
	if err != nil {
		return fmt.Errorf("GetRunByRepoAndID: %w", err)
	}
	childRunJobs, err := actions_model.GetRunJobsByRunID(ctx, childRun.ID)
	if err != nil {
		return fmt.Errorf("GetRunJobsByRunID: %w", err)
	}

	switch parentJob.Status {
	case actions_model.StatusWaiting,
		actions_model.StatusBlocked:
		if err := Rerun(ctx, childRun, childRunJobs, nil); err != nil {
			return fmt.Errorf("rerun child run: %w", err)
		}
		return nil
	case actions_model.StatusSkipped:
		return markChildRunJobsSkipped(ctx, childRunJobs)
	default:
		return fmt.Errorf("invalid parent job status: %s", parentJob.Status.String())
	}
}
