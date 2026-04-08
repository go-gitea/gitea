// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"slices"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/util"

	"github.com/nektos/act/pkg/model"
	"go.yaml.in/yaml/v4"
)

// GetFailedJobsForRerun returns the failed or cancelled jobs in a run.
func GetFailedJobsForRerun(allJobs []*actions_model.ActionRunJob) []*actions_model.ActionRunJob {
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
func RerunWorkflowRunJobs(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun, triggerUser *user_model.User, jobsToRerun []*actions_model.ActionRunJob) (*actions_model.ActionRunAttempt, error) {
	plan, err := buildRerunPlan(ctx, repo, run, triggerUser, jobsToRerun)
	if err != nil {
		return nil, err
	}
	return execRerunPlan(ctx, plan)
}

type rerunPlan struct {
	run          *actions_model.ActionRun
	templateJobs actions_model.ActionJobList
	rerunJobIDs  container.Set[string]
	vars         map[string]string
	newAttempt   *actions_model.ActionRunAttempt
}

func buildRerunPlan(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun, triggerUser *user_model.User, jobsToRerun []*actions_model.ActionRunJob) (*rerunPlan, error) {
	if !run.Status.IsDone() {
		return nil, util.NewInvalidArgumentErrorf("this workflow run is not done")
	}
	if run.RepoID != repo.ID {
		return nil, util.NewInvalidArgumentErrorf("run %d does not belong to repo %d", run.ID, repo.ID)
	}
	for _, job := range jobsToRerun {
		if job.RunID != run.ID {
			return nil, util.NewInvalidArgumentErrorf("job %d does not belong to workflow run %d", job.ID, run.ID)
		}
	}
	if triggerUser == nil {
		return nil, util.NewInvalidArgumentErrorf("trigger user is required")
	}

	if err := run.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	cfgUnit := repo.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	if cfg.IsWorkflowDisabled(run.WorkflowID) {
		return nil, util.NewInvalidArgumentErrorf("workflow %s is disabled", run.WorkflowID)
	}

	templateAttempt, hasTemplateAttempt, err := run.GetLatestAttempt(ctx)
	if err != nil {
		return nil, err
	}

	var templateJobs actions_model.ActionJobList
	if hasTemplateAttempt {
		templateJobs, err = actions_model.GetRunJobsByRunAndAttemptID(ctx, run.ID, templateAttempt.ID)
	} else {
		templateJobs, err = actions_model.GetRunJobsByRunAndAttemptID(ctx, run.ID, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("load template jobs: %w", err)
	}

	plan := &rerunPlan{
		run:          run,
		templateJobs: templateJobs,
	}
	if len(templateJobs) == 0 {
		return plan, nil
	}

	if err := plan.expandRerunJobIDs(jobsToRerun); err != nil {
		return nil, err
	}

	plan.vars, err = actions_model.GetVariablesOfRun(ctx, run)
	if err != nil {
		return nil, fmt.Errorf("get run %d variables: %w", run.ID, err)
	}

	attemptNum := int64(1)
	if hasTemplateAttempt {
		attemptNum = templateAttempt.Attempt + 1
	}
	plan.newAttempt = &actions_model.ActionRunAttempt{
		RepoID:        run.RepoID,
		RunID:         run.ID,
		Attempt:       attemptNum,
		TriggerUserID: triggerUser.ID,
		Status:        actions_model.StatusWaiting,
	}

	if run.RawConcurrency != "" {
		var rawConcurrency model.RawConcurrency
		if err := yaml.Unmarshal([]byte(run.RawConcurrency), &rawConcurrency); err != nil {
			return nil, fmt.Errorf("unmarshal raw concurrency: %w", err)
		}
		if err := EvaluateRunConcurrencyFillModel(ctx, run, plan.newAttempt, &rawConcurrency, plan.vars, nil); err != nil {
			return nil, err
		}
	}

	return plan, nil
}

func execRerunPlan(ctx context.Context, plan *rerunPlan) (*actions_model.ActionRunAttempt, error) {
	if len(plan.templateJobs) == 0 {
		return nil, nil //nolint:nilnil // a rerun plan with no template jobs is a valid no-op and creates no new attempt
	}

	var newJobs actions_model.ActionJobList
	var cancelledConcurrencyJobs []*actions_model.ActionRunJob

	err := db.WithTx(ctx, func(ctx context.Context) error {
		newAttemptStatus, jobsToCancel, err := PrepareToStartRunWithConcurrency(ctx, plan.newAttempt)
		if err != nil {
			return err
		}
		cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
		plan.newAttempt.Status = newAttemptStatus
		shouldBlock := newAttemptStatus == actions_model.StatusBlocked

		if err := db.Insert(ctx, plan.newAttempt); err != nil {
			return err
		}

		hasWaitingJobs := false
		newJobs = make(actions_model.ActionJobList, 0, len(plan.templateJobs))
		for i, templateJob := range plan.templateJobs {
			newJob := cloneRunJobForAttempt(templateJob, plan.newAttempt)
			// Legacy template jobs have no attempt association and therefore no AttemptJobID.
			// When rerun creates a new attempt for them, assign a stable non-zero AttemptJobID using the same parsed job order as initial run creation.
			if templateJob.RunAttemptID == 0 && newJob.RunAttemptID != 0 && newJob.AttemptJobID == 0 {
				newJob.AttemptJobID = int64(i + 1)
			}
			if plan.rerunJobIDs.Contains(templateJob.JobID) {
				shouldBlockJob := shouldBlock || plan.hasRerunDependency(templateJob)

				newJob.Status = util.Iif(shouldBlockJob, actions_model.StatusBlocked, actions_model.StatusWaiting)
				newJob.TaskID = 0
				newJob.SourceTaskID = 0
				newJob.Started = 0
				newJob.Stopped = 0
				newJob.ConcurrencyGroup = ""
				newJob.ConcurrencyCancel = false
				newJob.IsConcurrencyEvaluated = false

				if newJob.RawConcurrency != "" && !shouldBlockJob {
					if err := EvaluateJobConcurrencyFillModel(ctx, plan.run, newJob, plan.vars, nil); err != nil {
						return fmt.Errorf("evaluate job concurrency: %w", err)
					}
					newJob.Status, jobsToCancel, err = PrepareToStartJobWithConcurrency(ctx, newJob)
					if err != nil {
						return fmt.Errorf("prepare to start job with concurrency: %w", err)
					}
					cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
				}
			} else {
				newJob.TaskID = 0
				newJob.SourceTaskID = templateJob.EffectiveTaskID()
				newJob.Started = templateJob.Started
				newJob.Stopped = templateJob.Stopped
			}

			if err := db.Insert(ctx, newJob); err != nil {
				return err
			}
			hasWaitingJobs = hasWaitingJobs || newJob.Status == actions_model.StatusWaiting
			newJobs = append(newJobs, newJob)
		}

		plan.newAttempt.Status = actions_model.AggregateJobStatus(newJobs)
		if err := actions_model.UpdateRunAttempt(ctx, plan.newAttempt, "status"); err != nil {
			return err
		}

		plan.run.Started = 0
		plan.run.Stopped = 0
		plan.run.Status = plan.newAttempt.Status
		plan.run.LatestAttemptID = plan.newAttempt.ID
		if err := actions_model.UpdateRun(ctx, plan.run, "started", "stopped", "status", "latest_attempt_id"); err != nil {
			return err
		}

		if hasWaitingJobs {
			if err := actions_model.IncreaseTaskVersion(ctx, plan.run.OwnerID, plan.run.RepoID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := plan.run.LoadAttributes(ctx); err != nil {
		return nil, err
	}
	for _, job := range newJobs {
		job.Run = plan.run
	}

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, cancelledConcurrencyJobs)
	EmitJobsIfReadyByJobs(cancelledConcurrencyJobs)

	CreateCommitStatusForRunJobs(ctx, plan.run, newJobs...)
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, newJobs)

	return plan.newAttempt, nil
}

func (p *rerunPlan) expandRerunJobIDs(jobsToRerun []*actions_model.ActionRunJob) error {
	templateJobIDs := make(container.Set[string])
	for _, job := range p.templateJobs {
		templateJobIDs.Add(job.JobID)
	}

	if len(jobsToRerun) == 0 {
		p.rerunJobIDs = templateJobIDs
		return nil
	}

	rerunJobIDs := make(container.Set[string])
	for _, job := range jobsToRerun {
		if !templateJobIDs.Contains(job.JobID) {
			return util.NewInvalidArgumentErrorf("job %q does not exist in the latest attempt", job.JobID)
		}
		rerunJobIDs.Add(job.JobID)
	}

	for {
		found := false
		for _, job := range p.templateJobs {
			if rerunJobIDs.Contains(job.JobID) {
				continue
			}
			for _, need := range job.Needs {
				if rerunJobIDs.Contains(need) {
					found = true
					rerunJobIDs.Add(job.JobID)
					break
				}
			}
		}
		if !found {
			break
		}
	}

	p.rerunJobIDs = rerunJobIDs
	return nil
}

func (p *rerunPlan) hasRerunDependency(job *actions_model.ActionRunJob) bool {
	for _, need := range job.Needs {
		if p.rerunJobIDs.Contains(need) {
			return true
		}
	}
	return false
}

func cloneRunJobForAttempt(templateJob *actions_model.ActionRunJob, attempt *actions_model.ActionRunAttempt) *actions_model.ActionRunJob {
	return &actions_model.ActionRunJob{
		RunID:                  templateJob.RunID,
		RunAttemptID:           attempt.ID,
		RepoID:                 templateJob.RepoID,
		OwnerID:                templateJob.OwnerID,
		CommitSHA:              templateJob.CommitSHA,
		IsForkPullRequest:      templateJob.IsForkPullRequest,
		Name:                   templateJob.Name,
		Attempt:                attempt.Attempt,
		WorkflowPayload:        slices.Clone(templateJob.WorkflowPayload),
		JobID:                  templateJob.JobID,
		AttemptJobID:           templateJob.AttemptJobID,
		Needs:                  slices.Clone(templateJob.Needs),
		RunsOn:                 slices.Clone(templateJob.RunsOn),
		Status:                 templateJob.Status,
		RawConcurrency:         templateJob.RawConcurrency,
		IsConcurrencyEvaluated: templateJob.IsConcurrencyEvaluated,
		ConcurrencyGroup:       templateJob.ConcurrencyGroup,
		ConcurrencyCancel:      templateJob.ConcurrencyCancel,
		TokenPermissions:       templateJob.TokenPermissions,
	}
}
