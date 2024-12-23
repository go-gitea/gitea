// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"slices"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ActionRunJob represents a job of a run
type ActionRunJob struct {
	ID                int64
	RunID             int64      `xorm:"index"`
	Run               *ActionRun `xorm:"-"`
	RepoID            int64      `xorm:"index"`
	OwnerID           int64      `xorm:"index"`
	CommitSHA         string     `xorm:"index"`
	IsForkPullRequest bool
	Name              string `xorm:"VARCHAR(255)"`
	Attempt           int64
	WorkflowPayload   []byte
	JobID             string   `xorm:"VARCHAR(255)"` // job id in workflow, not job's id
	Needs             []string `xorm:"JSON TEXT"`
	RunsOn            []string `xorm:"JSON TEXT"`
	TaskID            int64    // the latest task of the job
	Status            Status   `xorm:"index"`
	Started           timeutil.TimeStamp
	Stopped           timeutil.TimeStamp
	Created           timeutil.TimeStamp `xorm:"created"`
	Updated           timeutil.TimeStamp `xorm:"updated index"`
}

func init() {
	db.RegisterModel(new(ActionRunJob))
}

func (job *ActionRunJob) Duration() time.Duration {
	return calculateDuration(job.Started, job.Stopped, job.Status)
}

func (job *ActionRunJob) LoadRun(ctx context.Context) error {
	if job.Run == nil {
		run, err := GetRunByID(ctx, job.RunID)
		if err != nil {
			return err
		}
		job.Run = run
	}
	return nil
}

// LoadAttributes load Run if not loaded
func (job *ActionRunJob) LoadAttributes(ctx context.Context) error {
	if job == nil {
		return nil
	}

	if err := job.LoadRun(ctx); err != nil {
		return err
	}

	return job.Run.LoadAttributes(ctx)
}

func GetRunJobByID(ctx context.Context, id int64) (*ActionRunJob, error) {
	var job ActionRunJob
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&job)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("run job with id %d: %w", id, util.ErrNotExist)
	}

	return &job, nil
}

func GetRunJobsByRunID(ctx context.Context, runID int64) ([]*ActionRunJob, error) {
	var jobs []*ActionRunJob
	if err := db.GetEngine(ctx).Where("run_id=?", runID).OrderBy("id").Find(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func UpdateRunJob(ctx context.Context, job *ActionRunJob, cond builder.Cond, cols ...string) (int64, error) {
	e := db.GetEngine(ctx)

	sess := e.ID(job.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}

	if cond != nil {
		sess.Where(cond)
	}

	affected, err := sess.Update(job)
	if err != nil {
		return 0, err
	}

	if affected == 0 || (!slices.Contains(cols, "status") && job.Status == 0) {
		return affected, nil
	}

	if affected != 0 && slices.Contains(cols, "status") && job.Status.IsWaiting() {
		// if the status of job changes to waiting again, increase tasks version.
		if err := IncreaseTaskVersion(ctx, job.OwnerID, job.RepoID); err != nil {
			return 0, err
		}
	}

	if job.RunID == 0 {
		var err error
		if job, err = GetRunJobByID(ctx, job.ID); err != nil {
			return 0, err
		}
	}

	{
		// Other goroutines may aggregate the status of the run and update it too.
		// So we need load the run and its jobs before updating the run.
		run, err := GetRunByID(ctx, job.RunID)
		if err != nil {
			return 0, err
		}
		jobs, err := GetRunJobsByRunID(ctx, job.RunID)
		if err != nil {
			return 0, err
		}
		run.Status = AggregateJobStatus(jobs)
		if run.Started.IsZero() && run.Status.IsRunning() {
			run.Started = timeutil.TimeStampNow()
		}
		if run.Stopped.IsZero() && run.Status.IsDone() {
			run.Stopped = timeutil.TimeStampNow()
		}
		if err := UpdateRun(ctx, run, "status", "started", "stopped"); err != nil {
			return 0, fmt.Errorf("update run %d: %w", run.ID, err)
		}
	}

	return affected, nil
}

func AggregateJobStatus(jobs []*ActionRunJob) Status {
	allSuccessOrSkipped := len(jobs) != 0
	allSkipped := len(jobs) != 0
	var hasFailure, hasCancelled, hasWaiting, hasRunning, hasBlocked bool
	for _, job := range jobs {
		allSuccessOrSkipped = allSuccessOrSkipped && (job.Status == StatusSuccess || job.Status == StatusSkipped)
		allSkipped = allSkipped && job.Status == StatusSkipped
		hasFailure = hasFailure || job.Status == StatusFailure
		hasCancelled = hasCancelled || job.Status == StatusCancelled
		hasWaiting = hasWaiting || job.Status == StatusWaiting
		hasRunning = hasRunning || job.Status == StatusRunning
		hasBlocked = hasBlocked || job.Status == StatusBlocked
	}
	switch {
	case allSkipped:
		return StatusSkipped
	case allSuccessOrSkipped:
		return StatusSuccess
	case hasCancelled:
		return StatusCancelled
	case hasFailure:
		return StatusFailure
	case hasRunning:
		return StatusRunning
	case hasWaiting:
		return StatusWaiting
	case hasBlocked:
		return StatusBlocked
	default:
		return StatusUnknown // it shouldn't happen
	}
}
