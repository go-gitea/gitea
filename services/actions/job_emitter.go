// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"

	"github.com/nektos/act/pkg/jobparser"
	act_model "github.com/nektos/act/pkg/model"
	"xorm.io/builder"
)

var jobEmitterQueue *queue.WorkerPoolQueue[*jobUpdate]

type jobUpdate struct {
	RunID int64
}

func EmitJobsIfReady(runID int64) error {
	err := jobEmitterQueue.Push(&jobUpdate{
		RunID: runID,
	})
	if errors.Is(err, queue.ErrAlreadyInQueue) {
		return nil
	}
	return err
}

func jobEmitterQueueHandler(items ...*jobUpdate) []*jobUpdate {
	ctx := graceful.GetManager().ShutdownContext()
	var ret []*jobUpdate
	for _, update := range items {
		if err := checkJobsByRunID(ctx, update.RunID); err != nil {
			ret = append(ret, update)
		}
	}
	return ret
}

func checkJobsByRunID(ctx context.Context, runID int64) error {
	run, exist, err := db.GetByID[actions_model.ActionRun](ctx, runID)
	if err != nil {
		return fmt.Errorf("get action run: %w", err)
	}
	if !exist {
		return fmt.Errorf("action run %d does not exist", runID)
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		// check jobs of the current run
		if err := checkJobsOfRun(ctx, run); err != nil {
			return err
		}

		// check jobs by the concurrency group of the run
		if len(run.ConcurrencyGroup) == 0 {
			return nil
		}
		concurrentActionRuns, err := db.Find[actions_model.ActionRun](ctx, &actions_model.FindRunOptions{
			RepoID:           run.RepoID,
			ConcurrencyGroup: run.ConcurrencyGroup,
			Status: []actions_model.Status{
				actions_model.StatusBlocked,
			},
			SortType: "oldest",
		})
		if err != nil {
			return fmt.Errorf("find action run with concurrency group %s: %w", run.ConcurrencyGroup, err)
		}
		for _, cRun := range concurrentActionRuns {
			if cRun.NeedApproval {
				continue
			}
			if err := checkJobsOfRun(ctx, cRun); err != nil {
				return err
			}
			break // only run one blocked action run with the same concurrency group
		}
		return nil
	})
}

func checkJobsOfRun(ctx context.Context, run *actions_model.ActionRun) error {
	jobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		return err
	}

	vars, err := actions_model.GetVariablesOfRun(ctx, run)
	if err != nil {
		return fmt.Errorf("get run %d variables: %w", run.ID, err)
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
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
			}
		}
		return nil
	}); err != nil {
		return err
	}
	CreateCommitStatus(ctx, jobs...)
	return nil
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

func (r *jobStatusResolver) resolve(ctx context.Context) map[int64]actions_model.Status {
	ret := map[int64]actions_model.Status{}
	for id, status := range r.statuses {
		if status != actions_model.StatusBlocked {
			continue
		}
		allDone, allSucceed := true, true
		for _, need := range r.needs[id] {
			needStatus := r.statuses[need]
			if !needStatus.IsDone() {
				allDone = false
			}
			if needStatus.In(actions_model.StatusFailure, actions_model.StatusCancelled, actions_model.StatusSkipped) {
				allSucceed = false
			}
		}
		if allDone {
			// check concurrency
			blockedByJobConcurrency, err := checkJobConcurrency(ctx, r.jobMap[id], r.vars)
			if err != nil {
				log.Error("Check run %d job %d concurrency: %v. This job will stay blocked.")
				continue
			}

			if blockedByJobConcurrency {
				continue
			}

			if allSucceed {
				ret[id] = actions_model.StatusWaiting
			} else {
				// Check if the job has an "if" condition
				hasIf := false
				if wfJobs, _ := jobparser.Parse(r.jobMap[id].WorkflowPayload); len(wfJobs) == 1 {
					_, wfJob := wfJobs[0].Job()
					hasIf = len(wfJob.If.Value) > 0
				}

				if hasIf {
					// act_runner will check the "if" condition
					ret[id] = actions_model.StatusWaiting
				} else {
					// If the "if" condition is empty and not all dependent jobs completed successfully,
					// the job should be skipped.
					ret[id] = actions_model.StatusSkipped
				}
			}
		}
	}
	return ret
}

func checkJobConcurrency(ctx context.Context, actionRunJob *actions_model.ActionRunJob, vars map[string]string) (bool, error) {
	if len(actionRunJob.RawConcurrencyGroup) == 0 {
		return false, nil
	}

	run := actionRunJob.Run

	if len(actionRunJob.ConcurrencyGroup) == 0 {
		rawConcurrency := &act_model.RawConcurrency{
			Group:            actionRunJob.RawConcurrencyGroup,
			CancelInProgress: actionRunJob.RawConcurrencyCancel,
		}

		gitCtx := jobparser.ToGitContext(GenerateGitContext(run, actionRunJob))

		actWorkflow, err := act_model.ReadWorkflow(bytes.NewReader(actionRunJob.WorkflowPayload))
		if err != nil {
			return false, fmt.Errorf("read workflow: %w", err)
		}
		actJob := actWorkflow.GetJob(actionRunJob.JobID)

		task, err := actions_model.GetTaskByID(ctx, actionRunJob.TaskID)
		if err != nil {
			return false, fmt.Errorf("get task by id: %w", err)
		}
		taskNeeds, err := FindTaskNeeds(ctx, task)
		if err != nil {
			return false, fmt.Errorf("find task needs: %w", err)
		}

		jobResults := make(map[string]*jobparser.JobResult, len(taskNeeds))
		for jobID, taskNeed := range taskNeeds {
			jobResult := &jobparser.JobResult{
				Result:  taskNeed.Result.String(),
				Outputs: taskNeed.Outputs,
			}
			jobResults[jobID] = jobResult
		}

		actionRunJob.ConcurrencyGroup, actionRunJob.ConcurrencyCancel = jobparser.InterpolatJobConcurrency(rawConcurrency, actJob, gitCtx, vars, jobResults)
		if _, err := actions_model.UpdateRunJob(ctx, &actions_model.ActionRunJob{
			ID:                actionRunJob.ID,
			ConcurrencyGroup:  actionRunJob.ConcurrencyGroup,
			ConcurrencyCancel: actionRunJob.ConcurrencyCancel,
		}, nil); err != nil {
			return false, fmt.Errorf("update run job: %w", err)
		}
	}

	if actionRunJob.ConcurrencyCancel {
		// cancel previous jobs in the same concurrency group
		previousJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{
			RepoID:           actionRunJob.RepoID,
			ConcurrencyGroup: actionRunJob.ConcurrencyGroup,
			Statuses: []actions_model.Status{
				actions_model.StatusRunning,
				actions_model.StatusWaiting,
				actions_model.StatusBlocked,
			},
		})
		if err != nil {
			return false, fmt.Errorf("find previous jobs: %w", err)
		}
		if err := actions_model.CancelJobs(ctx, previousJobs); err != nil {
			return false, fmt.Errorf("cancel previous jobs: %w", err)
		}
		// we have cancelled all previous jobs, so this job does not need to be blocked
		return false, nil
	}

	waitingConcurrentJobsNum, err := db.Count[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{
		RepoID:           actionRunJob.RepoID,
		ConcurrencyGroup: actionRunJob.ConcurrencyGroup,
		Statuses:         []actions_model.Status{actions_model.StatusWaiting},
	})
	if err != nil {
		return false, fmt.Errorf("count waiting jobs: %w", err)
	}

	return waitingConcurrentJobsNum > 0, nil
}
