// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"

	"gitea.dev/models/db"
	"gitea.dev/models/unit"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// maxTaskPickAttempts bounds how many candidate waiting jobs a single poll will
// try — whether skipped because they're unpreparable or lost to a concurrent
// runner's claim — before giving up.
const maxTaskPickAttempts = 10

func CreateTaskForRunner(ctx context.Context, runner *ActionRunner) (*ActionTask, bool, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, false, err
	}
	defer committer.Close()

	e := db.GetEngine(ctx)
	jobCond := runnerScopedJobCond(runner)

	// Pick the oldest waiting job the runner's labels can run and prepare a task for
	// it. Label matching is pushed into SQL via the normalized action_run_job_label
	// table (runner labels must cover every required label), so the query stays
	// O(1 row) regardless of backlog size and never skips a matchable job behind an
	// unmatchable head.
	//
	// A job that can't be prepared — its run was deleted out from under it (#37586)
	// or its payload won't parse — is marked failed so it leaves the queue instead of
	// stalling every runner's poll, and the next candidate is tried. The attempt
	// bound keeps a single poll from clearing an unbounded backlog of such jobs.
	log.Trace("runner labels: %v", runner.AgentLabels)
	matchCond := runnerMatchableJobCond(runner.AgentLabels)
	// Jobs already tried this poll and skipped (failed as unpreparable, or lost to a
	// concurrent runner's claim). They are excluded from the next query so the poll
	// advances to a fresh candidate: under MySQL's REPEATABLE READ the transaction's
	// snapshot keeps showing a lost job as still waiting, so without this the poll
	// would re-pick and re-lose the same head until the attempt budget runs out.
	var triedJobIDs []int64
	for range maxTaskPickAttempts {
		job := new(ActionRunJob)
		query := e.Where(builder.Eq{"task_id": 0, "status": StatusWaiting, "is_reusable_caller": false}).
			And(jobCond).
			And(matchCond)
		if len(triedJobIDs) > 0 {
			query = query.And(builder.NotIn("id", triedJobIDs))
		}
		has, err := query.
			Asc("updated", "id").
			Limit(1).
			Get(job)
		if err != nil {
			return nil, false, err
		}
		if !has {
			break
		}
		triedJobIDs = append(triedJobIDs, job.ID)

		if err := job.LoadAttributes(ctx); err != nil {
			if !errors.Is(err, util.ErrNotExist) {
				return nil, false, err
			}
			// The run no longer exists (#37586); fail the orphaned job and move on.
			log.Warn("fail unpreparable action job %d (run %d): %v", job.ID, job.RunID, err)
			if err := failUnpreparableJob(ctx, job); err != nil {
				return nil, false, err
			}
			continue
		}

		workflowJob, err := job.ParseJob()
		if err != nil {
			// A job that never parses would otherwise stall the queue forever.
			log.Warn("fail unparsable action job %d: %v", job.ID, err)
			if err := failUnpreparableJob(ctx, job); err != nil {
				return nil, false, err
			}
			continue
		}

		claimed, err := claimWaitingJob(ctx, job)
		if err != nil {
			return nil, false, err
		}
		if !claimed {
			// Another runner claimed this job between the select and the CAS. Retry
			// with the next candidate instead of giving up, so contending runners
			// spread across distinct jobs rather than all bailing on the same head.
			continue
		}

		task, err := assignJobToRunner(ctx, runner, job, workflowJob)
		if err != nil {
			return nil, false, err
		}

		if err := committer.Commit(); err != nil {
			return nil, false, err
		}
		return task, true, nil
	}

	// No assignable job (or we only found unpreparable ones); commit so any
	// fail-markings above persist, then report no task.
	if err := committer.Commit(); err != nil {
		return nil, false, err
	}
	return nil, false, nil
}

func runnerScopedJobCond(runner *ActionRunner) builder.Cond {
	jobCond := builder.NewCond()
	if runner.RepoID != 0 {
		jobCond = builder.Eq{"repo_id": runner.RepoID}
	} else if runner.OwnerID != 0 {
		jobCond = builder.In("repo_id", builder.Select("`repository`.id").From("repository").
			Join("INNER", "repo_unit", "`repository`.id = `repo_unit`.repo_id").
			Where(builder.Eq{"`repository`.owner_id": runner.OwnerID, "`repo_unit`.type": unit.TypeActions}))
	}
	if jobCond.IsValid() {
		jobCond = builder.In("run_id", builder.Select("id").From("action_run").Where(jobCond))
	}
	return jobCond
}

// claimWaitingJob CAS-marks a waiting job running so assignment queries no longer
// return it. Returns false when another runner claimed it first.
func claimWaitingJob(ctx context.Context, job *ActionRunJob) (bool, error) {
	now := timeutil.TimeStampNow()
	n, err := db.GetEngine(ctx).
		Where(builder.Eq{"id": job.ID, "task_id": 0, "status": StatusWaiting}).
		Cols("status", "started").
		Update(&ActionRunJob{Status: StatusRunning, Started: now})
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, nil
	}
	job.Status = StatusRunning
	job.Started = now
	return true, nil
}

// assignJobToRunner creates the task (and its steps) for a job already claimed via
// claimWaitingJob and links it back to the job row.
func assignJobToRunner(ctx context.Context, runner *ActionRunner, job *ActionRunJob, workflowJob *jobparser.Job) (*ActionTask, error) {
	e := db.GetEngine(ctx)

	task := &ActionTask{
		JobID:             job.ID,
		Attempt:           job.Attempt,
		RunnerID:          runner.ID,
		Started:           job.Started,
		Status:            StatusRunning,
		RepoID:            job.RepoID,
		OwnerID:           job.OwnerID,
		CommitSHA:         job.CommitSHA,
		IsForkPullRequest: job.IsForkPullRequest,
	}
	task.GenerateAndFillToken()

	if _, err := e.Insert(task); err != nil {
		return nil, err
	}

	task.LogFilename = logFileName(job.Run.Repo.FullName(), task.ID)
	if err := UpdateTask(ctx, task, "log_filename"); err != nil {
		return nil, err
	}

	if len(workflowJob.Steps) > 0 {
		steps := make([]*ActionTaskStep, len(workflowJob.Steps))
		for i, v := range workflowJob.Steps {
			steps[i] = &ActionTaskStep{
				Name:   makeTaskStepDisplayName(v, 255),
				TaskID: task.ID,
				Index:  int64(i),
				RepoID: task.RepoID,
				Status: StatusWaiting,
			}
		}
		if _, err := e.Insert(steps); err != nil {
			return nil, err
		}
		task.Steps = steps
	}

	job.TaskID = task.ID
	// Persist "status"/"started" alongside task_id (no-op values, already set by
	// claimWaitingJob) so UpdateRunJob re-aggregates the attempt/run status to
	// running. Without it the run stays "waiting", which leaves stale run status
	// and lets run-level concurrency admit jobs it should block.
	if _, err := UpdateRunJob(ctx, job, builder.Eq{"task_id": 0, "status": StatusRunning}, "task_id", "status", "started"); err != nil {
		return nil, err
	}

	task.Job = job
	return task, nil
}

// failUnpreparableJob marks a waiting job failed so it leaves the queue instead of
// stalling task assignment for every runner. The CAS guards against a concurrent claim.
//
// It prefers UpdateRunJob so attempt/run status re-aggregate when the run still
// exists. When aggregation can't load the run (#37586), the row update already
// succeeded or a direct fallback clears the waiting queue.
func failUnpreparableJob(ctx context.Context, job *ActionRunJob) error {
	job.Status = StatusFailure
	job.Stopped = timeutil.TimeStampNow()
	n, err := UpdateRunJob(ctx, job, builder.Eq{"task_id": 0, "status": StatusWaiting}, "status", "stopped")
	if n > 0 || err == nil {
		return nil
	}
	_, fallbackErr := db.GetEngine(ctx).
		Where(builder.Eq{"id": job.ID, "task_id": 0, "status": StatusWaiting}).
		Cols("status", "stopped").
		Update(&ActionRunJob{Status: StatusFailure, Stopped: job.Stopped})
	if fallbackErr != nil {
		// Surface the fallback's failure — the earlier UpdateRunJob error is often the
		// expected util.ErrNotExist (#37586) and would hide the real DB error here.
		return fallbackErr
	}
	return nil
}
