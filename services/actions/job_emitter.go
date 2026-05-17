// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

var jobEmitterQueue *queue.WorkerPoolQueue[*jobUpdate]

type jobUpdate struct {
	RunID int64
}

func EmitJobsIfReadyByRun(runID int64) error {
	err := jobEmitterQueue.Push(&jobUpdate{
		RunID: runID,
	})
	if errors.Is(err, queue.ErrAlreadyInQueue) {
		return nil
	}
	return err
}

func EmitJobsIfReadyByJobs(jobs []*actions_model.ActionRunJob) {
	checkedRuns := make(container.Set[int64])
	for _, job := range jobs {
		if !job.Status.IsDone() || checkedRuns.Contains(job.RunID) {
			continue
		}
		if err := EmitJobsIfReadyByRun(job.RunID); err != nil {
			log.Error("Check jobs of run %d: %v", job.RunID, err)
		}
		checkedRuns.Add(job.RunID)
	}
}

func jobEmitterQueueHandler(items ...*jobUpdate) []*jobUpdate {
	ctx := graceful.GetManager().ShutdownContext()
	var ret []*jobUpdate
	for _, update := range items {
		if err := checkJobsByRunID(ctx, update.RunID); err != nil {
			log.Error("check run %d: %v", update.RunID, err)
			ret = append(ret, update)
		}
	}
	return ret
}

func checkJobsByRunID(ctx context.Context, runID int64) error {
	run, exist, err := db.GetByID[actions_model.ActionRun](ctx, runID)
	if !exist {
		return fmt.Errorf("run %d does not exist", runID)
	}
	if err != nil {
		return fmt.Errorf("get action run: %w", err)
	}
	var jobs, updatedJobs, cancelledJobs []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		// check jobs of the current run
		if js, ujs, cjs, err := checkJobsOfCurrentRunAttempt(ctx, run); err != nil {
			return err
		} else {
			jobs = append(jobs, js...)
			updatedJobs = append(updatedJobs, ujs...)
			cancelledJobs = append(cancelledJobs, cjs...)
		}
		if js, ujs, cjs, err := checkRunConcurrency(ctx, run); err != nil {
			return err
		} else {
			jobs = append(jobs, js...)
			updatedJobs = append(updatedJobs, ujs...)
			cancelledJobs = append(cancelledJobs, cjs...)
		}
		return nil
	}); err != nil {
		return err
	}
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, cancelledJobs)
	EmitJobsIfReadyByJobs(cancelledJobs)
	if err := createCommitStatusesForJobsByRun(ctx, jobs); err != nil {
		return err
	}
	NotifyWorkflowJobsStatusUpdate(ctx, updatedJobs...)
	runJobs := make(map[int64][]*actions_model.ActionRunJob)
	for _, job := range jobs {
		runJobs[job.RunID] = append(runJobs[job.RunID], job)
	}
	runUpdatedJobs := make(map[int64][]*actions_model.ActionRunJob)
	for _, uj := range updatedJobs {
		runUpdatedJobs[uj.RunID] = append(runUpdatedJobs[uj.RunID], uj)
	}
	for runID, js := range runJobs {
		if len(runUpdatedJobs[runID]) == 0 {
			continue
		}
		runUpdated := true
		for _, job := range js {
			if !job.Status.IsDone() {
				runUpdated = false
				break
			}
		}
		if runUpdated {
			NotifyWorkflowRunStatusUpdateWithReload(ctx, js[0].RepoID, js[0].RunID)
		}
	}
	return nil
}

func createCommitStatusesForJobsByRun(ctx context.Context, jobs []*actions_model.ActionRunJob) error {
	runJobs := make(map[int64][]*actions_model.ActionRunJob)
	for _, job := range jobs {
		runJobs[job.RunID] = append(runJobs[job.RunID], job)
	}

	for jobRunID, jobList := range runJobs {
		run, err := actions_model.GetRunByRepoAndID(ctx, jobList[0].RepoID, jobRunID)
		if err != nil {
			return fmt.Errorf("get action run %d: %w", jobRunID, err)
		}
		CreateCommitStatusForRunJobs(ctx, run, jobList...)
	}
	return nil
}

// findBlockedRunIDByConcurrency finds a blocked concurrent run in a repo and returns 0 when there is no blocked run.
func findBlockedRunIDByConcurrency(ctx context.Context, repoID int64, concurrencyGroup string) (int64, error) {
	if concurrencyGroup == "" {
		return 0, nil
	}
	cAttempts, cJobs, err := actions_model.GetConcurrentRunAttemptsAndJobs(ctx, repoID, concurrencyGroup, []actions_model.Status{actions_model.StatusBlocked})
	if err != nil {
		return 0, fmt.Errorf("find concurrent runs and jobs: %w", err)
	}

	if len(cAttempts) > 0 {
		return cAttempts[0].RunID, nil
	}
	if len(cJobs) > 0 {
		return cJobs[0].RunID, nil
	}

	return 0, nil
}

func checkBlockedConcurrentRun(ctx context.Context, repoID, runID int64) (jobs, updatedJobs, cancelledJobs []*actions_model.ActionRunJob, err error) {
	concurrentRun, err := actions_model.GetRunByRepoAndID(ctx, repoID, runID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get run %d: %w", runID, err)
	}
	if concurrentRun.NeedApproval {
		return nil, nil, nil, nil
	}

	return checkJobsOfCurrentRunAttempt(ctx, concurrentRun)
}

// checkRunConcurrency rechecks runs blocked by concurrency that may become unblocked after the current run releases a workflow-level or job-level concurrency group.
func checkRunConcurrency(ctx context.Context, run *actions_model.ActionRun) (jobs, updatedJobs, cancelledJobs []*actions_model.ActionRunJob, err error) {
	checkedConcurrencyGroup := make(container.Set[string])

	collect := func(concurrencyGroup string) error {
		concurrentRunID, err := findBlockedRunIDByConcurrency(ctx, run.RepoID, concurrencyGroup)
		if err != nil {
			return fmt.Errorf("find blocked run by concurrency: %w", err)
		}
		if concurrentRunID > 0 {
			js, ujs, cjs, err := checkBlockedConcurrentRun(ctx, run.RepoID, concurrentRunID)
			if err != nil {
				return err
			}
			jobs = append(jobs, js...)
			updatedJobs = append(updatedJobs, ujs...)
			cancelledJobs = append(cancelledJobs, cjs...)
		}
		checkedConcurrencyGroup.Add(concurrencyGroup)
		return nil
	}

	// check run (workflow-level) concurrency
	runConcurrencyGroup, _, err := run.GetEffectiveConcurrency(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("GetEffectiveConcurrency: %w", err)
	}
	if runConcurrencyGroup != "" {
		if err := collect(runConcurrencyGroup); err != nil {
			return nil, nil, nil, err
		}
	}

	// check job concurrency
	runJobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(ctx, run.RepoID, run.ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("find run %d jobs: %w", run.ID, err)
	}
	for _, job := range runJobs {
		if !job.Status.IsDone() {
			continue
		}
		if job.ConcurrencyGroup == "" || checkedConcurrencyGroup.Contains(job.ConcurrencyGroup) {
			continue
		}
		if err := collect(job.ConcurrencyGroup); err != nil {
			return nil, nil, nil, err
		}
	}
	return jobs, updatedJobs, cancelledJobs, nil
}

// checkJobsOfCurrentRunAttempt resolves blocked jobs of the run's latest attempt.
func checkJobsOfCurrentRunAttempt(ctx context.Context, run *actions_model.ActionRun) (jobs, updatedJobs, cancelledJobs []*actions_model.ActionRunJob, err error) {
	jobs, err = actions_model.GetRunJobsByRunAndAttemptID(ctx, run.ID, run.LatestAttemptID)
	if err != nil {
		return nil, nil, nil, err
	}
	// The resolver below only considers needs and job-level concurrency, so a run blocked
	// solely by run-level concurrency would have its jobs unblocked here. checkRunConcurrency
	// re-evaluates when the holding run finishes.
	if run.Status.IsBlocked() {
		attempt, has, err := run.GetLatestAttempt(ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("GetLatestAttempt: %w", err)
		}
		if has {
			shouldBlock, err := shouldBlockRunByConcurrency(ctx, attempt)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("shouldBlockRunByConcurrency: %w", err)
			}
			if shouldBlock {
				return jobs, nil, nil, nil
			}
		}
	}
	vars, err := actions_model.GetVariablesOfRun(ctx, run)
	if err != nil {
		return nil, nil, nil, err
	}
	resolver := newJobStatusResolver(jobs, vars)

	if err = db.WithTx(ctx, func(ctx context.Context) error {
		// Expand any deferred-matrix placeholders whose `needs` are now done
		// and all succeeded. This must happen inside the transaction so the
		// resolver below sees the newly created children atomically.
		expanded, err := expandDeferredMatrixPlaceholders(ctx, run, jobs)
		if err != nil {
			return fmt.Errorf("expandDeferredMatrixPlaceholders: %w", err)
		}
		if expanded {
			jobs, err = actions_model.GetRunJobsByRunAndAttemptID(ctx, run.ID, run.LatestAttemptID)
			if err != nil {
				return fmt.Errorf("reload jobs after matrix expansion: %w", err)
			}
			resolver = newJobStatusResolver(jobs, vars)
		}

		for _, job := range jobs {
			job.Run = run
		}

		updates := resolver.Resolve(ctx)
		for _, job := range jobs {
			if status, ok := updates[job.ID]; ok {
				job.Status = status
				if n, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"status": actions_model.StatusBlocked}, "status"); err != nil {
					return err
				} else if n != 1 {
					return fmt.Errorf("no affected for updating blocked job %v", job.ID)
				}
				updatedJobs = append(updatedJobs, job)
			}
		}
		return nil
	}); err != nil {
		return nil, nil, nil, err
	}

	return jobs, updatedJobs, resolver.cancelledJobs, nil
}

type jobStatusResolver struct {
	statuses      map[int64]actions_model.Status
	needs         map[int64][]int64
	jobMap        map[int64]*actions_model.ActionRunJob
	vars          map[string]string
	cancelledJobs []*actions_model.ActionRunJob
}

func newJobStatusResolver(jobs actions_model.ActionJobList, vars map[string]string) *jobStatusResolver {
	idToJobs := make(map[string][]*actions_model.ActionRunJob, len(jobs))
	jobMap := make(map[int64]*actions_model.ActionRunJob)
	for _, job := range jobs {
		idToJobs[job.JobID] = append(idToJobs[job.JobID], job)
		jobMap[job.ID] = job
	}

	statuses := make(map[int64]actions_model.Status, len(jobs))
	needs := make(map[int64][]int64, len(jobs))
	for _, job := range jobs {
		statuses[job.ID] = job.Status
		for _, need := range job.Needs {
			for _, v := range idToJobs[need] {
				needs[job.ID] = append(needs[job.ID], v.ID)
			}
		}
	}
	return &jobStatusResolver{
		statuses: statuses,
		needs:    needs,
		jobMap:   jobMap,
		vars:     vars,
	}
}

func (r *jobStatusResolver) Resolve(ctx context.Context) map[int64]actions_model.Status {
	ret := map[int64]actions_model.Status{}
	for i := 0; i < len(r.statuses); i++ {
		updated := r.resolve(ctx)
		if len(updated) == 0 {
			return ret
		}
		for k, v := range updated {
			ret[k] = v
			r.statuses[k] = v
		}
	}
	return ret
}

func (r *jobStatusResolver) resolveCheckNeeds(id int64) (allDone, allSucceed bool) {
	allDone, allSucceed = true, true
	for _, need := range r.needs[id] {
		needStatus := r.statuses[need]
		if !needStatus.IsDone() {
			allDone = false
		}
		if needStatus.In(actions_model.StatusFailure, actions_model.StatusCancelled, actions_model.StatusSkipped) {
			allSucceed = false
		}
	}
	return allDone, allSucceed
}

func (r *jobStatusResolver) resolveJobHasIfCondition(actionRunJob *actions_model.ActionRunJob) (hasIf bool) {
	// FIXME evaluate this on the server side
	if job, err := actionRunJob.ParseJob(); err == nil {
		return len(job.If.Value) > 0
	}
	return hasIf
}

func (r *jobStatusResolver) resolve(ctx context.Context) map[int64]actions_model.Status {
	ret := map[int64]actions_model.Status{}
	for id, status := range r.statuses {
		actionRunJob := r.jobMap[id]
		if status != actions_model.StatusBlocked {
			continue
		}
		allDone, allSucceed := r.resolveCheckNeeds(id)
		if !allDone {
			continue
		}

		// update concurrency and check whether the job can run now
		err := updateConcurrencyEvaluationForJobWithNeeds(ctx, actionRunJob, r.vars)
		if err != nil {
			// The err can be caused by different cases: database error, or syntax error, or the needed jobs haven't completed
			// At the moment there is no way to distinguish them.
			// Actually, for most cases, the error is caused by "syntax error" / "the needed jobs haven't completed (skipped?)"
			// TODO: if workflow or concurrency expression has syntax error, there should be a user error message, need to show it to end users
			log.Debug("updateConcurrencyEvaluationForJobWithNeeds failed, this job will stay blocked: job: %d, err: %v", id, err)
			continue
		}

		shouldStartJob := true
		if !allSucceed {
			// Not all dependent jobs completed successfully:
			// * if the job has "if" condition, it can be started, then the act_runner will evaluate the "if" condition.
			// * otherwise, the job should be skipped.
			shouldStartJob = r.resolveJobHasIfCondition(actionRunJob)
		}

		newStatus := util.Iif(shouldStartJob, actions_model.StatusWaiting, actions_model.StatusSkipped)
		if newStatus == actions_model.StatusWaiting {
			var cancelledJobs []*actions_model.ActionRunJob
			newStatus, cancelledJobs, err = PrepareToStartJobWithConcurrency(ctx, actionRunJob)
			if err != nil {
				log.Error("ShouldBlockJobByConcurrency failed, this job will stay blocked: job: %d, err: %v", id, err)
			} else {
				r.cancelledJobs = append(r.cancelledJobs, cancelledJobs...)
			}
		}

		if newStatus != actions_model.StatusBlocked {
			ret[id] = newStatus
		}
	}
	return ret
}

// expandDeferredMatrixPlaceholders finds any deferred-matrix placeholder
// jobs (RawMatrix != "") whose `needs:` are all done and all succeeded, and
// expands them into N concrete child RunJobs — one per matrix combination.
// The placeholder row is deleted; new child rows replace it.
//
// Placeholders whose needs aren't all done yet, or whose needs include a
// failure/cancelled/skipped, are left untouched: the resolver downstream
// either keeps them blocked or transitions them to Skipped via the standard
// flow.
//
// Returns true if any placeholder was expanded (caller should reload jobs).
func expandDeferredMatrixPlaceholders(ctx context.Context, run *actions_model.ActionRun, jobs []*actions_model.ActionRunJob) (bool, error) {
	// Index jobs by JobID for the readiness check.
	byJobID := make(map[string][]*actions_model.ActionRunJob, len(jobs))
	for _, j := range jobs {
		byJobID[j.JobID] = append(byJobID[j.JobID], j)
	}

	expanded := false
	for _, placeholder := range jobs {
		if placeholder.RawMatrix == "" || placeholder.Status != actions_model.StatusBlocked {
			continue
		}
		allDone, allSucceed := needsReadyForExpansion(placeholder, byJobID)
		if !allDone {
			observeDeferredCheck()
			continue
		}
		if !allSucceed {
			// An upstream failure: leave for the resolver, which will skip
			// the placeholder. No metric counter — that's a workflow
			// outcome, not a matrix-expansion outcome.
			continue
		}
		if err := expandOnePlaceholder(ctx, run, placeholder); err != nil {
			return expanded, fmt.Errorf("expand placeholder job %d: %w", placeholder.ID, err)
		}
		expanded = true
	}
	return expanded, nil
}

// needsReadyForExpansion returns whether all of placeholder.Needs are present
// and done in the snapshot, and whether they all succeeded (or were skipped,
// which we treat as success for the purpose of expansion — same as the
// resolver's standard `allSucceed` check).
func needsReadyForExpansion(placeholder *actions_model.ActionRunJob, byJobID map[string][]*actions_model.ActionRunJob) (allDone, allSucceed bool) {
	allDone, allSucceed = true, true
	for _, needID := range placeholder.Needs {
		siblings := byJobID[needID]
		if len(siblings) == 0 {
			// Need not present in this snapshot — treat as not done.
			allDone = false
			continue
		}
		for _, s := range siblings {
			if !s.Status.IsDone() {
				allDone = false
			}
			if s.Status.In(actions_model.StatusFailure, actions_model.StatusCancelled, actions_model.StatusSkipped) {
				allSucceed = false
			}
		}
	}
	return allDone, allSucceed
}

// expandOnePlaceholder builds the upstream needs-outputs map, calls
// jobparser.ExpandDeferredMatrix, inserts N child RunJobs, and deletes the
// placeholder row. All operations run in the caller's transaction.
func expandOnePlaceholder(ctx context.Context, run *actions_model.ActionRun, placeholder *actions_model.ActionRunJob) (err error) {
	timings := newMatrixTimings()
	childrenCount := 0
	defer func() {
		outcome := "success"
		if err != nil {
			outcome = "failure"
		}
		timings.end(outcome, childrenCount)
	}()

	taskNeeds, err := FindTaskNeeds(ctx, placeholder)
	if err != nil {
		return fmt.Errorf("FindTaskNeeds: %w", err)
	}
	needsOutputs := make(map[string]map[string]string, len(taskNeeds))
	for needID, tn := range taskNeeds {
		needsOutputs[needID] = tn.Outputs
	}

	vars, err := actions_model.GetVariablesOfRun(ctx, run)
	if err != nil {
		return fmt.Errorf("GetVariablesOfRun: %w", err)
	}

	runAttempt, has, err := run.GetLatestAttempt(ctx)
	if err != nil {
		return fmt.Errorf("GetLatestAttempt: %w", err)
	}
	if !has {
		return fmt.Errorf("no latest attempt for run %d", run.ID)
	}

	// GenerateGiteaContext reads run.TriggerUser and run.Repo, which are
	// lazy-loaded — explicitly load them so the context build doesn't panic.
	if err := run.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("run.LoadAttributes: %w", err)
	}

	giteaCtx := GenerateGiteaContext(ctx, run, runAttempt, nil)
	timings.startParse()
	expanded, err := jobparser.ExpandDeferredMatrix(
		placeholder.WorkflowPayload,
		placeholder.Needs,
		needsOutputs,
		jobparser.WithVars(vars),
		jobparser.WithGitContext(giteaCtx.ToGitHubContext()),
	)
	timings.endParse()
	if err != nil {
		return fmt.Errorf("ExpandDeferredMatrix: %w", err)
	}

	// Allocate child AttemptJobIDs starting after the maximum currently in
	// use within this attempt — keeps the (run_id, attempt_id, attempt_job_id)
	// uniqueness invariant from migration v331.
	maxAttemptJobID, err := maxAttemptJobIDForAttempt(ctx, run.ID, placeholder.RunAttemptID)
	if err != nil {
		return fmt.Errorf("maxAttemptJobIDForAttempt: %w", err)
	}

	children := make([]*actions_model.ActionRunJob, 0, len(expanded))
	for i, swf := range expanded {
		id, job := swf.Job()
		// Mirror InsertRun's payload contract: children are dispatched to
		// runners, so their needs must be erased from the serialized payload.
		job.EraseNeeds()
		if err := swf.SetJob(id, job); err != nil {
			return fmt.Errorf("SetJob: %w", err)
		}
		payload, err := swf.Marshal()
		if err != nil {
			return fmt.Errorf("marshal expanded SingleWorkflow: %w", err)
		}

		var pinned map[string][]any
		_ = job.Strategy.RawMatrix.Decode(&pinned)
		matrixValues := make(map[string]any, len(pinned))
		for k, v := range pinned {
			if len(v) == 1 {
				matrixValues[k] = v[0]
			}
		}

		children = append(children, &actions_model.ActionRunJob{
			RunID:             placeholder.RunID,
			RunAttemptID:      placeholder.RunAttemptID,
			RepoID:            placeholder.RepoID,
			OwnerID:           placeholder.OwnerID,
			CommitSHA:         placeholder.CommitSHA,
			IsForkPullRequest: placeholder.IsForkPullRequest,
			Name:              util.EllipsisDisplayString(job.Name, 255),
			Attempt:           placeholder.Attempt,
			WorkflowPayload:   payload,
			JobID:             id,
			AttemptJobID:      maxAttemptJobID + int64(i+1),
			Needs:             placeholder.Needs,
			RunsOn:            job.RunsOn(),
			Status:            actions_model.StatusWaiting,
			TokenPermissions:  placeholder.TokenPermissions,
			MatrixValues:      matrixValues,
		})
	}
	timings.startInsert()
	if err := actions_model.InsertActionRunJobs(ctx, children); err != nil {
		return fmt.Errorf("InsertActionRunJobs: %w", err)
	}
	timings.endInsert()
	childrenCount = len(children)

	if _, err := db.GetEngine(ctx).ID(placeholder.ID).Delete(&actions_model.ActionRunJob{}); err != nil {
		return fmt.Errorf("delete placeholder %d: %w", placeholder.ID, err)
	}

	if err := actions_model.IncreaseTaskVersion(ctx, placeholder.OwnerID, placeholder.RepoID); err != nil {
		return fmt.Errorf("IncreaseTaskVersion: %w", err)
	}
	return nil
}

// maxAttemptJobIDForAttempt returns the largest AttemptJobID currently in
// use within a (run, attempt). Returns 0 when no rows exist.
func maxAttemptJobIDForAttempt(ctx context.Context, runID, runAttemptID int64) (int64, error) {
	var maxID int64
	if _, err := db.GetEngine(ctx).
		Select("COALESCE(MAX(attempt_job_id), 0)").
		Table("action_run_job").
		Where("run_id = ? AND run_attempt_id = ?", runID, runAttemptID).
		Get(&maxID); err != nil {
		return 0, err
	}
	return maxID, nil
}

func updateConcurrencyEvaluationForJobWithNeeds(ctx context.Context, actionRunJob *actions_model.ActionRunJob, vars map[string]string) error {
	if setting.IsInTesting && actionRunJob.RepoID == 0 {
		return nil // for testing purpose only, no repo, no evaluation
	}

	// Legacy jobs (created before migration v331) have RunAttemptID=0 and no attempt record.
	var attempt *actions_model.ActionRunAttempt
	if actionRunJob.RunAttemptID > 0 {
		var err error
		attempt, err = actions_model.GetRunAttemptByRepoAndID(ctx, actionRunJob.RepoID, actionRunJob.RunAttemptID)
		if err != nil {
			return fmt.Errorf("GetRunAttemptByRepoAndID: %w", err)
		}
	}
	if err := EvaluateJobConcurrencyFillModel(ctx, actionRunJob.Run, attempt, actionRunJob, vars, nil); err != nil {
		return fmt.Errorf("evaluate job concurrency: %w", err)
	}

	if _, err := actions_model.UpdateRunJob(ctx, actionRunJob, nil, "concurrency_group", "concurrency_cancel", "is_concurrency_evaluated"); err != nil {
		return fmt.Errorf("update run job: %w", err)
	}
	return nil
}
