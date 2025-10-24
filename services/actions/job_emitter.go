// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"

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
	var jobs, updatedJobs []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		// check jobs of the current run
		if js, ujs, err := checkJobsOfRun(ctx, run); err != nil {
			return err
		} else {
			jobs = append(jobs, js...)
			updatedJobs = append(updatedJobs, ujs...)
		}
		if js, ujs, err := checkRunConcurrency(ctx, run); err != nil {
			return err
		} else {
			jobs = append(jobs, js...)
			updatedJobs = append(updatedJobs, ujs...)
		}
		return nil
	}); err != nil {
		return err
	}
	CreateCommitStatusForRunJobs(ctx, run, jobs...)
	for _, job := range updatedJobs {
		_ = job.LoadAttributes(ctx)
		notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, nil)
	}
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
			NotifyWorkflowRunStatusUpdateWithReload(ctx, jobs[0])
		}
	}
	return nil
}

// findBlockedRunByConcurrency finds the blocked concurrent run in a repo and returns `nil, nil` when there is no blocked run.
func findBlockedRunByConcurrency(ctx context.Context, repoID int64, concurrencyGroup string) (*actions_model.ActionRun, error) {
	if concurrencyGroup == "" {
		return nil, nil
	}
	cRuns, cJobs, err := actions_model.GetConcurrentRunsAndJobs(ctx, repoID, concurrencyGroup, []actions_model.Status{actions_model.StatusBlocked})
	if err != nil {
		return nil, fmt.Errorf("find concurrent runs and jobs: %w", err)
	}

	// There can be at most one blocked run or job
	var concurrentRun *actions_model.ActionRun
	if len(cRuns) > 0 {
		concurrentRun = cRuns[0]
	} else if len(cJobs) > 0 {
		jobRun, exist, err := db.GetByID[actions_model.ActionRun](ctx, cJobs[0].RunID)
		if !exist {
			return nil, fmt.Errorf("run %d does not exist", cJobs[0].RunID)
		}
		if err != nil {
			return nil, fmt.Errorf("get run by job %d: %w", cJobs[0].ID, err)
		}
		concurrentRun = jobRun
	}

	return concurrentRun, nil
}

func checkRunConcurrency(ctx context.Context, run *actions_model.ActionRun) (jobs, updatedJobs []*actions_model.ActionRunJob, err error) {
	checkedConcurrencyGroup := make(container.Set[string])

	// check run (workflow-level) concurrency
	if run.ConcurrencyGroup != "" {
		concurrentRun, err := findBlockedRunByConcurrency(ctx, run.RepoID, run.ConcurrencyGroup)
		if err != nil {
			return nil, nil, fmt.Errorf("find blocked run by concurrency: %w", err)
		}
		if concurrentRun != nil && !concurrentRun.NeedApproval {
			js, ujs, err := checkJobsOfRun(ctx, concurrentRun)
			if err != nil {
				return nil, nil, err
			}
			jobs = append(jobs, js...)
			updatedJobs = append(updatedJobs, ujs...)
		}
		checkedConcurrencyGroup.Add(run.ConcurrencyGroup)
	}

	// check job concurrency
	runJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		return nil, nil, fmt.Errorf("find run %d jobs: %w", run.ID, err)
	}
	for _, job := range runJobs {
		if !job.Status.IsDone() {
			continue
		}
		if job.ConcurrencyGroup == "" && checkedConcurrencyGroup.Contains(job.ConcurrencyGroup) {
			continue
		}
		concurrentRun, err := findBlockedRunByConcurrency(ctx, job.RepoID, job.ConcurrencyGroup)
		if err != nil {
			return nil, nil, fmt.Errorf("find blocked run by concurrency: %w", err)
		}
		if concurrentRun != nil && !concurrentRun.NeedApproval {
			js, ujs, err := checkJobsOfRun(ctx, concurrentRun)
			if err != nil {
				return nil, nil, err
			}
			jobs = append(jobs, js...)
			updatedJobs = append(updatedJobs, ujs...)
		}
		checkedConcurrencyGroup.Add(job.ConcurrencyGroup)
	}
	return jobs, updatedJobs, nil
}

func checkJobsOfRun(ctx context.Context, run *actions_model.ActionRun) (jobs, updatedJobs []*actions_model.ActionRunJob, err error) {
	jobs, err = db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		return nil, nil, err
	}
	vars, err := actions_model.GetVariablesOfRun(ctx, run)
	if err != nil {
		return nil, nil, err
	}

	if err = db.WithTx(ctx, func(ctx context.Context) error {
		for _, job := range jobs {
			job.Run = run
		}

		updates := newJobStatusResolver(jobs, vars).Resolve(ctx)
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
		return nil, nil, err
	}

	return jobs, updatedJobs, nil
}

func NotifyWorkflowRunStatusUpdateWithReload(ctx context.Context, job *actions_model.ActionRunJob) {
	job.Run = nil
	if err := job.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}
	notify_service.WorkflowRunStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job.Run)
}

type jobStatusResolver struct {
	statuses map[int64]actions_model.Status
	needs    map[int64][]int64
	jobMap   map[int64]*actions_model.ActionRunJob
	vars     map[string]string
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
			newStatus, err = PrepareToStartJobWithConcurrency(ctx, actionRunJob)
			if err != nil {
				log.Error("ShouldBlockJobByConcurrency failed, this job will stay blocked: job: %d, err: %v", id, err)
			}
		}

		if newStatus != actions_model.StatusBlocked {
			ret[id] = newStatus
		}
	}
	return ret
}

func updateConcurrencyEvaluationForJobWithNeeds(ctx context.Context, actionRunJob *actions_model.ActionRunJob, vars map[string]string) error {
	if setting.IsInTesting && actionRunJob.RepoID == 0 {
		return nil // for testing purpose only, no repo, no evaluation
	}

	err := EvaluateJobConcurrencyFillModel(ctx, actionRunJob.Run, actionRunJob, vars)
	if err != nil {
		return fmt.Errorf("evaluate job concurrency: %w", err)
	}

	if _, err := actions_model.UpdateRunJob(ctx, actionRunJob, nil, "concurrency_group", "concurrency_cancel", "is_concurrency_evaluated"); err != nil {
		return fmt.Errorf("update run job: %w", err)
	}
	return nil
}
