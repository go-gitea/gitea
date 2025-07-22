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
	notify_service "code.gitea.io/gitea/services/notify"

	"github.com/nektos/act/pkg/jobparser"
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
			log.Error("check run %d: %v", update.RunID, err)
			ret = append(ret, update)
		}
	}
	return ret
}

func checkJobsByRunID(ctx context.Context, runID int64) error {
	run, err := actions_model.GetRunByID(ctx, runID)
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
		// check run (workflow-level) concurrency
		concurrentRunIDs := make(container.Set[int64])
		concurrentRunIDs.Add(run.ID)
		if run.ConcurrencyGroup != "" {
			concurrentRuns, err := db.Find[actions_model.ActionRun](ctx, actions_model.FindRunOptions{
				RepoID:           run.RepoID,
				ConcurrencyGroup: run.ConcurrencyGroup,
				Status:           []actions_model.Status{actions_model.StatusBlocked},
			})
			if err != nil {
				return err
			}
			for _, concurrentRun := range concurrentRuns {
				if concurrentRunIDs.Contains(concurrentRun.ID) {
					continue
				}
				concurrentRunIDs.Add(concurrentRun.ID)
				if concurrentRun.NeedApproval {
					continue
				}
				if js, ujs, err := checkJobsOfRun(ctx, concurrentRun); err != nil {
					return err
				} else {
					jobs = append(jobs, js...)
					updatedJobs = append(updatedJobs, ujs...)
				}
				updatedRun, err := actions_model.GetRunByID(ctx, concurrentRun.ID)
				if err != nil {
					return err
				}
				if updatedRun.Status == actions_model.StatusWaiting {
					// only run one blocked action run in the same concurrency group
					break
				}
			}
		}

		// check job concurrency
		runJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
		if err != nil {
			return err
		}
		for _, job := range runJobs {
			if job.Status.IsDone() && job.ConcurrencyGroup != "" {
				waitingConcurrentJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{
					RepoID:           job.RepoID,
					ConcurrencyGroup: job.ConcurrencyGroup,
					Statuses:         []actions_model.Status{actions_model.StatusWaiting},
				})
				if err != nil {
					return err
				}
				if len(waitingConcurrentJobs) == 0 {
					blockedConcurrentJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{
						RepoID:           job.RepoID,
						ConcurrencyGroup: job.ConcurrencyGroup,
						Statuses:         []actions_model.Status{actions_model.StatusBlocked},
					})
					if err != nil {
						return err
					}
					for _, concurrentJob := range blockedConcurrentJobs {
						if concurrentRunIDs.Contains(concurrentJob.RunID) {
							continue
						}
						concurrentRunIDs.Add(concurrentJob.RunID)
						concurrentRun, err := actions_model.GetRunByID(ctx, concurrentJob.RunID)
						if err != nil {
							return err
						}
						if concurrentRun.NeedApproval {
							continue
						}
						if js, ujs, err := checkJobsOfRun(ctx, concurrentRun); err != nil {
							return err
						} else {
							jobs = append(jobs, js...)
							updatedJobs = append(updatedJobs, ujs...)
						}
						updatedJob, err := actions_model.GetRunJobByID(ctx, concurrentJob.ID)
						if err != nil {
							return err
						}
						if updatedJob.Status == actions_model.StatusWaiting {
							break
						}
					}
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	CreateCommitStatus(ctx, jobs...)
	for _, job := range updatedJobs {
		_ = job.LoadAttributes(ctx)
		notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, nil)
	}
	if len(jobs) > 0 {
		runUpdated := true
		for _, job := range jobs {
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
			blockedByJobConcurrency, err := checkConcurrencyForJobWithNeeds(ctx, r.jobMap[id], r.vars)
			if err != nil {
				log.Error("Check job %d concurrency: %v. This job will stay blocked.", id, err)
				continue
			}

			if blockedByJobConcurrency {
				continue
			}

			if err := CancelJobsByJobConcurrency(ctx, r.jobMap[id]); err != nil {
				log.Error("Cancel previous jobs for job %d: %v", id, err)
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

func checkConcurrencyForJobWithNeeds(ctx context.Context, actionRunJob *actions_model.ActionRunJob, vars map[string]string) (bool, error) {
	if actionRunJob.RawConcurrencyGroup == "" {
		return false, nil
	}
	if err := actionRunJob.LoadAttributes(ctx); err != nil {
		return false, err
	}

	if !actionRunJob.IsConcurrencyEvaluated {
		taskNeeds, err := FindTaskNeeds(ctx, actionRunJob)
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

		actionRunJob.ConcurrencyGroup, actionRunJob.ConcurrencyCancel, err = EvaluateJobConcurrency(ctx, actionRunJob.Run, actionRunJob, vars, jobResults)
		if err != nil {
			return false, fmt.Errorf("evaluate job concurrency: %w", err)
		}
		actionRunJob.IsConcurrencyEvaluated = true

		if _, err := actions_model.UpdateRunJob(ctx, actionRunJob, nil, "concurrency_group", "concurrency_cancel", "is_concurrency_evaluated"); err != nil {
			return false, fmt.Errorf("update run job: %w", err)
		}
	}

	return actions_model.ShouldBlockJobByConcurrency(ctx, actionRunJob)
}
