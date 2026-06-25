// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"slices"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

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
//     (Exception: reusable workflow caller expansion runs inside the tx, see expandReusableWorkflowCaller's doc.)
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
	triggerUser     *user_model.User

	// rerunAttemptJobIDs holds the AttemptJobIDs of jobs that will actually be re-run in the new attempt.
	// If a job here is a reusable caller, the whole subtree under it will be re-run.
	rerunAttemptJobIDs container.Set[int64]

	// ancestorAttemptJobIDs holds the AttemptJobIDs of reusable caller jobs that have only some of their descendants being re-run:
	// the caller itself is NOT re-run as a whole, it stays pass-through and its non-rerun children stay pass-through too.
	ancestorAttemptJobIDs container.Set[int64]

	// skipCloneTemplateJobIDs holds the template-attempt DB row IDs of descendants of any reusable caller in rerunAttemptJobIDs.
	// These jobs should not be cloned, since the caller's lazy expansion will re-insert them fresh.
	skipCloneTemplateJobIDs container.Set[int64]
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
	plan.skipCloneTemplateJobIDs = plan.collectResetCallerDescendants()

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
	var hasWaitingCallerJobs bool

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
		newJobsToRerun = make(actions_model.ActionJobList, 0, len(plan.rerunAttemptJobIDs))

		// templateIDToNewID maps each template-attempt job's DB ID to its newly-inserted clone's DB ID
		templateIDToNewID := make(map[int64]int64, len(plan.templateJobs))

		for _, templateJob := range plan.templateJobs {
			// descendants of a reset reusable caller are not cloned at all, the caller will re-insert them
			if plan.skipCloneTemplateJobIDs.Contains(templateJob.ID) {
				continue
			}

			newJob := cloneRunJobForAttempt(templateJob, newAttempt)

			// Remap ParentJobID from template attempts's DB ID -> new attempt's DB ID.
			if templateJob.ParentJobID != 0 {
				newParentID, ok := templateIDToNewID[templateJob.ParentJobID]
				if !ok {
					return fmt.Errorf("clone order violation: parent job %d not yet cloned for child %d",
						templateJob.ParentJobID, templateJob.ID)
				}
				newJob.ParentJobID = newParentID
			}

			if plan.rerunAttemptJobIDs.Contains(templateJob.AttemptJobID) {
				shouldBlockJob := shouldBlock || plan.hasRerunDependency(templateJob)

				newJob.Status = util.Iif(shouldBlockJob, actions_model.StatusBlocked, actions_model.StatusWaiting)
				newJob.TaskID = 0
				newJob.SourceTaskID = 0
				newJob.Started = 0
				newJob.Stopped = 0
				newJob.ConcurrencyGroup = ""
				newJob.ConcurrencyCancel = false
				newJob.IsConcurrencyEvaluated = false

				if templateJob.IsReusableCaller {
					newJob.IsExpanded = false
					newJob.CallPayload = ""
				}

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

				isAncestor := plan.ancestorAttemptJobIDs.Contains(templateJob.AttemptJobID)
				newJob.Started = util.Iif(isAncestor, 0, templateJob.Started)
				newJob.Stopped = util.Iif(isAncestor, 0, templateJob.Stopped)
			}

			if err := db.Insert(ctx, newJob); err != nil {
				return err
			}
			templateIDToNewID[templateJob.ID] = newJob.ID

			// expand reusable caller
			if newJob.IsReusableCaller && newJob.Status == actions_model.StatusWaiting && !newJob.IsExpanded {
				if err := expandReusableWorkflowCaller(ctx, plan.run, newAttempt, newJob, vars); err != nil {
					return fmt.Errorf("inline trigger caller %d ready: %w", newJob.ID, err)
				}
				// refresh the caller status
				if err := actions_model.RefreshReusableCallerStatus(ctx, newJob); err != nil {
					return fmt.Errorf("refresh caller %d status: %w", newJob.ID, err)
				}
				hasWaitingCallerJobs = true
			}

			// A reusable caller is never dispatched to a runner, so it must not drive the task-version bump.
			hasWaitingJobs = hasWaitingJobs || (newJob.Status == actions_model.StatusWaiting && !newJob.IsReusableCaller)
			newJobs = append(newJobs, newJob)
		}

		// Refresh each ancestor's status from its now-fresh children.
		// `newJobs` is appended top-down (caller before its children), so we walk it in reverse to refresh the deepest ancestor first.
		for _, ancestor := range slices.Backward(newJobs) {
			if !ancestor.IsReusableCaller || !plan.ancestorAttemptJobIDs.Contains(ancestor.AttemptJobID) {
				continue
			}
			if err := actions_model.RefreshReusableCallerStatus(ctx, ancestor); err != nil {
				return fmt.Errorf("refresh ancestor caller %d status: %w", ancestor.ID, err)
			}
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

	// Post-commit kick for expanded callers: let job_emitter resolve its child jobs
	if hasWaitingCallerJobs {
		if err := EmitJobsIfReadyByRun(plan.run.ID); err != nil {
			log.Error("emit run %d after rerun: %v", plan.run.ID, err)
		}
	}

	return newAttempt, nil
}

// expandRerunJobIDs computes rerunAttemptJobIDs and ancestorAttemptJobIDs from the user-selected jobsToRerun.
func (p *rerunPlan) expandRerunJobIDs(jobsToRerun []*actions_model.ActionRunJob) error {
	// Empty jobsToRerun: rerun the whole latest attempt
	if len(jobsToRerun) == 0 {
		all := make(container.Set[int64], len(p.templateJobs))
		for _, job := range p.templateJobs {
			all.Add(job.AttemptJobID)
		}
		p.rerunAttemptJobIDs = all
		p.ancestorAttemptJobIDs = make(container.Set[int64])
		return nil
	}

	byID := make(map[int64]*actions_model.ActionRunJob, len(p.templateJobs))
	byAttemptJobID := make(map[int64]*actions_model.ActionRunJob, len(p.templateJobs))
	for _, job := range p.templateJobs {
		byID[job.ID] = job
		byAttemptJobID[job.AttemptJobID] = job
	}

	for _, job := range jobsToRerun {
		if _, ok := byID[job.ID]; !ok {
			return util.NewInvalidArgumentErrorf("job %q does not exist in the latest attempt", job.JobID)
		}
	}

	rerunSet := make(container.Set[int64])
	ancestorSet := make(container.Set[int64])
	queue := make([]*actions_model.ActionRunJob, 0, len(jobsToRerun))

	for _, job := range jobsToRerun {
		j := byID[job.ID]
		rerunSet.Add(j.AttemptJobID)
		queue = append(queue, j)
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		// same-scope downstream: siblings whose Needs reference cur.JobID join the rerun set
		for _, candidate := range p.templateJobs {
			if candidate.ParentJobID != cur.ParentJobID {
				continue
			}
			if rerunSet.Contains(candidate.AttemptJobID) || ancestorSet.Contains(candidate.AttemptJobID) {
				continue
			}
			if !slices.Contains(candidate.Needs, cur.JobID) {
				continue
			}
			rerunSet.Add(candidate.AttemptJobID)
			queue = append(queue, candidate)
		}

		// escalate to parent caller as an ancestor so its own siblings get checked next round
		if cur.ParentJobID == 0 {
			continue
		}
		parent, ok := byID[cur.ParentJobID]
		if !ok {
			continue
		}
		if rerunSet.Contains(parent.AttemptJobID) || ancestorSet.Contains(parent.AttemptJobID) {
			continue
		}
		ancestorSet.Add(parent.AttemptJobID)
		queue = append(queue, parent)
	}

	// remove entries whose parent-caller chain already has a rerunSet member
	for atID := range ancestorSet {
		cur := byAttemptJobID[atID]
		for cur.ParentJobID != 0 {
			parent, ok := byID[cur.ParentJobID]
			if !ok {
				break
			}
			if rerunSet.Contains(parent.AttemptJobID) {
				delete(ancestorSet, atID)
				break
			}
			cur = parent
		}
	}

	p.rerunAttemptJobIDs = rerunSet
	p.ancestorAttemptJobIDs = ancestorSet
	return nil
}

// hasRerunDependency reports whether `job` has a needs-reference that points to a job which is itself being rerun (in rerunAttemptJobIDs)
// or is an ancestor caller whose subtree is being rerun (in ancestorAttemptJobIDs).
// Either case means `job` should start in Blocked status.
func (p *rerunPlan) hasRerunDependency(job *actions_model.ActionRunJob) bool {
	if len(job.Needs) == 0 {
		return false
	}
	needSet := container.SetOf(job.Needs...)
	for _, sibling := range p.templateJobs {
		if sibling.ParentJobID != job.ParentJobID {
			continue
		}
		if !needSet.Contains(sibling.JobID) {
			continue
		}
		if p.rerunAttemptJobIDs.Contains(sibling.AttemptJobID) || p.ancestorAttemptJobIDs.Contains(sibling.AttemptJobID) {
			return true
		}
	}
	return false
}

// collectResetCallerDescendants walks p.templateJobs and returns the DB IDs of every transitive descendant of any reusable caller whose AttemptJobID is in p.rerunAttemptJobIDs.
// These descendants must NOT be cloned by execRerunPlan: the reset caller will re-insert them with template-matched AttemptJobIDs.
func (p *rerunPlan) collectResetCallerDescendants() container.Set[int64] {
	out := make(container.Set[int64])
	for _, tj := range p.templateJobs {
		if !tj.IsReusableCaller || !p.rerunAttemptJobIDs.Contains(tj.AttemptJobID) {
			continue
		}
		// If this caller's row ID is already in `out`, it means an outer caller has already covered its whole subtree.
		// Skip the redundant walk.
		if out.Contains(tj.ID) {
			continue
		}
		for _, child := range actions_model.CollectAllDescendantJobs(tj, p.templateJobs) {
			out.Add(child.ID)
		}
	}
	return out
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
		ContinueOnError:        templateJob.ContinueOnError,
		Status:                 templateJob.Status,
		RawConcurrency:         templateJob.RawConcurrency,
		IsConcurrencyEvaluated: templateJob.IsConcurrencyEvaluated,
		ConcurrencyGroup:       templateJob.ConcurrencyGroup,
		ConcurrencyCancel:      templateJob.ConcurrencyCancel,
		TokenPermissions:       templateJob.TokenPermissions,

		// reusable workflow fields
		IsReusableCaller:        templateJob.IsReusableCaller,
		CallUses:                templateJob.CallUses,
		ReusableWorkflowContent: slices.Clone(templateJob.ReusableWorkflowContent),
		CallSecrets:             templateJob.CallSecrets,
		CallPayload:             templateJob.CallPayload,
		IsExpanded:              templateJob.IsExpanded,
		ParentJobID:             templateJob.ParentJobID, // remapped by execRerunPlan
		WorkflowSourceRepoID:    templateJob.WorkflowSourceRepoID,
		WorkflowSourceCommitSHA: templateJob.WorkflowSourceCommitSHA,
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
