// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/graceful"
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
		if err := checkJobsOfRun(ctx, update.RunID); err != nil {
			ret = append(ret, update)
		}
	}
	return ret
}

func checkJobsOfRun(ctx context.Context, runID int64) error {
	jobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: runID})
	if err != nil {
		return err
	}
	var updatedjobs []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		idToJobs := make(map[string][]*actions_model.ActionRunJob, len(jobs))
		for _, job := range jobs {
			idToJobs[job.JobID] = append(idToJobs[job.JobID], job)
		}

		updates := newJobStatusResolver(jobs).Resolve()
		for _, job := range jobs {
			if status, ok := updates[job.ID]; ok {
				job.Status = status
				if n, err := actions_model.UpdateRunJob(ctx, job, builder.Eq{"status": actions_model.StatusBlocked}, "status"); err != nil {
					return err
				} else if n != 1 {
					return fmt.Errorf("no affected for updating blocked job %v", job.ID)
				}
				updatedjobs = append(updatedjobs, job)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	CreateCommitStatus(ctx, jobs...)
	for _, job := range updatedjobs {
		_ = job.LoadAttributes(ctx)
		notify_service.WorkflowJobStatusUpdate(ctx, job.Run.Repo, job.Run.TriggerUser, job, nil)
	}
	return nil
}

type jobStatusResolver struct {
	statuses map[int64]actions_model.Status
	needs    map[int64][]int64
	jobMap   map[int64]*actions_model.ActionRunJob
}

func newJobStatusResolver(jobs actions_model.ActionJobList) *jobStatusResolver {
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
	}
}

func (r *jobStatusResolver) Resolve() map[int64]actions_model.Status {
	ret := map[int64]actions_model.Status{}
	for i := 0; i < len(r.statuses); i++ {
		updated := r.resolve()
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

func (r *jobStatusResolver) resolve() map[int64]actions_model.Status {
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
