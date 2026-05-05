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
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"gitea.com/gitea/runner/act/model"
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
//
// The three stages below (legacy backfill, plan build, plan exec) deliberately run in separate DB transactions
// rather than one big outer transaction:
//   - execRerunPlan performs slow work (loading variables, YAML unmarshal, concurrency expression evaluation)
//     before opening its own transaction, so the tx stays focused on inserts/updates.
//   - The legacy backfill is idempotent-friendly: if it succeeds but a later stage fails, a subsequent rerun
//     will observe run.LatestAttemptID != 0 and skip the backfill, continuing naturally. No data corruption
//     or stuck state results from partial progress.
//
// Fast validations that can catch failures early (workflow disabled, run not done, etc.) are therefore
// pushed into validateRerun so we rarely enter createOriginalAttemptForLegacyRun only to fail afterwards.
func RerunWorkflowRunJobs(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun, triggerUser *user_model.User, jobsToRerun []*actions_model.ActionRunJob) (*actions_model.ActionRunAttempt, error) {
	if err := validateRerun(ctx, run, repo, triggerUser, jobsToRerun); err != nil {
		return nil, err
	}

	if run.LatestAttemptID == 0 {
		if err := createOriginalAttemptForLegacyRun(ctx, run); err != nil {
			return nil, fmt.Errorf("create attempt for legacy run: %w", err)
		}
	}

	plan, err := buildRerunPlan(ctx, run, triggerUser, jobsToRerun)
	if err != nil {
		return nil, err
	}
	return execRerunPlan(ctx, plan)
}

func validateRerun(ctx context.Context, run *actions_model.ActionRun, repo *repo_model.Repository, triggerUser *user_model.User, jobsToRerun []*actions_model.ActionRunJob) error {
	if !run.Status.IsDone() {
		return util.NewInvalidArgumentErrorf("this workflow run is not done")
	}
	if repo == nil {
		return util.NewInvalidArgumentErrorf("repo is required")
	}
	if run.RepoID != repo.ID {
		return util.NewInvalidArgumentErrorf("run %d does not belong to repo %d", run.ID, repo.ID)
	}
	for _, job := range jobsToRerun {
		if job.RunID != run.ID {
			return util.NewInvalidArgumentErrorf("job %d does not belong to workflow run %d", job.ID, run.ID)
		}
	}
	if triggerUser == nil {
		return util.NewInvalidArgumentErrorf("trigger user is required")
	}
	cfgUnit := repo.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	if cfg.IsWorkflowDisabled(run.WorkflowID) {
		return util.NewInvalidArgumentErrorf("workflow %s is disabled", run.WorkflowID)
	}

	// Legacy runs (LatestAttemptID == 0) conceptually have only attempt 1, so they can never be at the cap.
	// For non-legacy runs, look up the latest attempt and reject when its number is already at the configured cap.
	if run.LatestAttemptID > 0 {
		latestAttempt, has, err := run.GetLatestAttempt(ctx)
		if err != nil {
			return fmt.Errorf("GetLatestAttempt: %w", err)
		}
		if has && latestAttempt.Attempt >= setting.Actions.MaxRerunAttempts {
			return util.NewInvalidArgumentErrorf("workflow run has reached the maximum of %d attempts", setting.Actions.MaxRerunAttempts)
		}
	}

	return nil
}

// rerunPlan is a read-only snapshot of the inputs needed to execute a rerun.
// It holds no to-be-persisted entities and no intermediate evaluation results;
// execRerunPlan constructs and evaluates the new ActionRunAttempt itself.
type rerunPlan struct {
	run             *actions_model.ActionRun
	templateAttempt *actions_model.ActionRunAttempt
	templateJobs    actions_model.ActionJobList
	rerunJobIDs     container.Set[string]
	triggerUser     *user_model.User
}

// buildRerunPlan constructs a rerunPlan for the given workflow run without writing to the database.
// jobsToRerun contains only the user-requested target jobs. An empty jobsToRerun means the entire run should be rerun.
// It loads the latest attempt as a template and expands jobsToRerun to include all transitive downstream dependents.
// The construction of new-attempt and concurrency evaluation are deferred to execRerunPlan so that the plan remains a pure input snapshot.
func buildRerunPlan(ctx context.Context, run *actions_model.ActionRun, triggerUser *user_model.User, jobsToRerun []*actions_model.ActionRunJob) (*rerunPlan, error) {
	if err := run.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	templateAttempt, hasTemplateAttempt, err := run.GetLatestAttempt(ctx)
	if err != nil {
		return nil, err
	}
	if !hasTemplateAttempt {
		return nil, util.NewNotExistErrorf("latest attempt not found")
	}

	templateJobs, err := actions_model.GetRunJobsByRunAndAttemptID(ctx, run.ID, templateAttempt.ID)
	if err != nil {
		return nil, fmt.Errorf("load template jobs: %w", err)
	}
	if len(templateJobs) == 0 {
		return nil, util.NewNotExistErrorf("no template jobs")
	}

	plan := &rerunPlan{
		run:             run,
		templateAttempt: templateAttempt,
		templateJobs:    templateJobs,
		triggerUser:     triggerUser,
	}

	if err := plan.expandRerunJobIDs(jobsToRerun); err != nil {
		return nil, err
	}

	return plan, nil
}

// execRerunPlan executes the rerun plan built by buildRerunPlan.
// It loads run variables, constructs the new ActionRunAttempt and evaluates run-level concurrency (all outside the transaction to keep the tx short).
// Inside a single database transaction it then inserts the new attempt, clones all template jobs, evaluates job-level concurrency for rerun jobs,
// and updates the run's latest_attempt_id.
// Jobs not in the rerun set are cloned as pass-through: their status is preserved and SourceTaskID points to the original task so the UI can still display their results.
// The attempt's final status is derived only from the rerun jobs, not the pass-through jobs.
// Notifications and commit statuses are sent after the transaction commits.
func execRerunPlan(ctx context.Context, plan *rerunPlan) (*actions_model.ActionRunAttempt, error) {
	vars, err := actions_model.GetVariablesOfRun(ctx, plan.run)
	if err != nil {
		return nil, fmt.Errorf("get run %d variables: %w", plan.run.ID, err)
	}

	newAttempt := &actions_model.ActionRunAttempt{
		RepoID:        plan.run.RepoID,
		RunID:         plan.run.ID,
		Attempt:       plan.templateAttempt.Attempt + 1,
		TriggerUserID: plan.triggerUser.ID,
		Status:        actions_model.StatusWaiting,
	}

	if plan.run.RawConcurrency != "" {
		var rawConcurrency model.RawConcurrency
		if err := yaml.Unmarshal([]byte(plan.run.RawConcurrency), &rawConcurrency); err != nil {
			return nil, fmt.Errorf("unmarshal raw concurrency: %w", err)
		}
		if err := EvaluateRunConcurrencyFillModel(ctx, plan.run, newAttempt, &rawConcurrency, vars, nil); err != nil {
			return nil, err
		}
	}

	var newJobs, newJobsToRerun actions_model.ActionJobList
	var cancelledConcurrencyJobs []*actions_model.ActionRunJob

	err = db.WithTx(ctx, func(ctx context.Context) error {
		newAttemptStatus, jobsToCancel, err := PrepareToStartRunWithConcurrency(ctx, newAttempt)
		if err != nil {
			return err
		}
		cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
		newAttempt.Status = newAttemptStatus
		shouldBlock := newAttemptStatus == actions_model.StatusBlocked

		if err := db.Insert(ctx, newAttempt); err != nil {
			if _, getErr := actions_model.GetRunAttemptByRunIDAndAttemptNum(ctx, plan.run.ID, newAttempt.Attempt); getErr == nil {
				return util.NewAlreadyExistErrorf("workflow run attempt %d for run %d already exists", newAttempt.Attempt, plan.run.ID)
			}
			return err
		}

		plan.run.LatestAttemptID = newAttempt.ID
		if err := actions_model.UpdateRun(ctx, plan.run, "latest_attempt_id"); err != nil {
			return err
		}

		hasWaitingJobs := false
		newJobs = make(actions_model.ActionJobList, 0, len(plan.templateJobs))
		newJobsToRerun = make(actions_model.ActionJobList, 0, len(plan.rerunJobIDs))
		for _, templateJob := range plan.templateJobs {
			newJob := cloneRunJobForAttempt(templateJob, newAttempt)
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
					if err := EvaluateJobConcurrencyFillModel(ctx, plan.run, newAttempt, newJob, vars, nil); err != nil {
						return fmt.Errorf("evaluate job concurrency: %w", err)
					}
					newJob.Status, jobsToCancel, err = PrepareToStartJobWithConcurrency(ctx, newJob)
					if err != nil {
						return fmt.Errorf("prepare to start job with concurrency: %w", err)
					}
					cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
				}

				newJobsToRerun = append(newJobsToRerun, newJob)
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

		newAttempt.Status = actions_model.AggregateJobStatus(newJobsToRerun)
		if err := actions_model.UpdateRunAttempt(ctx, newAttempt, "status"); err != nil {
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

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, cancelledConcurrencyJobs)
	EmitJobsIfReadyByJobs(cancelledConcurrencyJobs)

	CreateCommitStatusForRunJobs(ctx, plan.run, newJobs...)
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, newJobsToRerun)

	return newAttempt, nil
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

// createOriginalAttemptForLegacyRun creates a real attempt=1 for a legacy run and updates the existing legacy jobs and artifacts in place
// so the original execution becomes attempt-aware before the rerun plan is built and all subsequent logic can use real attempts.
// Tasks are not modified: they reference jobs by JobID, so updating jobs implicitly carries the new attempt linkage.
func createOriginalAttemptForLegacyRun(ctx context.Context, run *actions_model.ActionRun) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		jobs, err := actions_model.GetRunJobsByRunAndAttemptID(ctx, run.ID, 0)
		if err != nil {
			return fmt.Errorf("load legacy run jobs: %w", err)
		}
		if len(jobs) == 0 {
			return fmt.Errorf("run %d has no jobs", run.ID)
		}

		originalAttempt := &actions_model.ActionRunAttempt{
			RepoID:        run.RepoID,
			RunID:         run.ID,
			Attempt:       1,
			TriggerUserID: run.TriggerUserID,

			// Legacy concurrency fields on ActionRun are intentionally NOT backfilled onto this original attempt.
			// They only matter while a run is actively being scheduled, and backfilling them for completed legacy runs
			// would add migration/runtime cost without changing any future concurrency behavior.

			Status:  run.Status,
			Created: run.Created,
			Started: run.Started,
			Stopped: run.Stopped,
		}

		// Use NoAutoTime so xorm does not overwrite Created with the current time on insert.
		if _, err := db.GetEngine(ctx).NoAutoTime().Insert(originalAttempt); err != nil {
			if _, getErr := actions_model.GetRunAttemptByRunIDAndAttemptNum(ctx, run.ID, originalAttempt.Attempt); getErr == nil {
				return util.NewAlreadyExistErrorf("workflow run attempt %d for run %d already exists", originalAttempt.Attempt, run.ID)
			}
			return err
		}

		// backfill attempt related fields for jobs
		for i, job := range jobs {
			job.RunAttemptID = originalAttempt.ID
			job.Attempt = originalAttempt.Attempt
			job.AttemptJobID = int64(i + 1)
			if _, err := db.GetEngine(ctx).ID(job.ID).Cols("run_attempt_id", "attempt", "attempt_job_id").Update(job); err != nil {
				return fmt.Errorf("backfill legacy run jobs: %w", err)
			}
		}

		// backfill "run_attempt_id" field for artifacts
		if _, err := db.GetEngine(ctx).
			Where("run_id=? AND run_attempt_id=0", run.ID).
			Cols("run_attempt_id").
			Update(&actions_model.ActionArtifact{RunAttemptID: originalAttempt.ID}); err != nil {
			return fmt.Errorf("backfill legacy artifacts: %w", err)
		}

		// update "latest_attempt_id" for the run
		run.LatestAttemptID = originalAttempt.ID
		return actions_model.UpdateRun(ctx, run, "latest_attempt_id")
	})
}
