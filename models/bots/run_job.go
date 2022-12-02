// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bots

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"golang.org/x/exp/slices"
	"xorm.io/builder"
)

// BotRunJob represents a job of a run
type BotRunJob struct {
	ID                int64
	RunID             int64   `xorm:"index"`
	Run               *BotRun `xorm:"-"`
	RepoID            int64   `xorm:"index"`
	OwnerID           int64   `xorm:"index"`
	CommitSHA         string  `xorm:"index"`
	IsForkPullRequest bool
	Name              string
	Attempt           int64
	WorkflowPayload   []byte
	JobID             string   // job id in workflow, not job's id
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
	db.RegisterModel(new(BotRunJob))
}

func (job *BotRunJob) TakeTime() time.Duration {
	if job.Started == 0 {
		return 0
	}
	started := job.Started.AsTime()
	if job.Status.IsDone() {
		return job.Stopped.AsTime().Sub(started)
	}
	job.Stopped.AsTime().Sub(started)
	return time.Since(started).Truncate(time.Second)
}

func (job *BotRunJob) LoadRun(ctx context.Context) error {
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
func (job *BotRunJob) LoadAttributes(ctx context.Context) error {
	if job == nil {
		return nil
	}

	if err := job.LoadRun(ctx); err != nil {
		return err
	}

	return job.Run.LoadAttributes(ctx)
}

// ErrRunJobNotExist represents an error for bot run job not exist
type ErrRunJobNotExist struct {
	ID int64
}

func (err ErrRunJobNotExist) Error() string {
	return fmt.Sprintf("run job [%d] is not exist", err.ID)
}

func GetRunJobByID(ctx context.Context, id int64) (*BotRunJob, error) {
	var job BotRunJob
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&job)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunJobNotExist{
			ID: id,
		}
	}

	return &job, nil
}

func GetRunJobsByRunID(ctx context.Context, runID int64) ([]*BotRunJob, error) {
	var jobs []*BotRunJob
	if err := db.GetEngine(ctx).Where("run_id=?", runID).OrderBy("id").Find(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func UpdateRunJob(ctx context.Context, job *BotRunJob, cond builder.Cond, cols ...string) (int64, error) {
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

	if job.RunID == 0 {
		var err error
		if job, err = GetRunJobByID(ctx, job.ID); err != nil {
			return affected, err
		}
	}

	jobs, err := GetRunJobsByRunID(ctx, job.RunID)
	if err != nil {
		return affected, err
	}

	runStatus := aggregateJobStatus(jobs)

	run := &BotRun{
		ID:     job.RunID,
		Status: runStatus,
	}
	if runStatus.IsDone() {
		run.Stopped = timeutil.TimeStampNow()
	}
	return affected, UpdateRun(ctx, run)
}

func aggregateJobStatus(jobs []*BotRunJob) Status {
	allDone := true
	allWaiting := true
	hasFailure := false
	for _, job := range jobs {
		if !job.Status.IsDone() {
			allDone = false
		}
		if job.Status != StatusWaiting {
			allWaiting = false
		}
		if job.Status == StatusFailure || job.Status == StatusCancelled {
			hasFailure = true
		}
	}
	if allDone {
		if hasFailure {
			return StatusFailure
		}
		return StatusSuccess
	}
	if allWaiting {
		return StatusWaiting
	}
	return StatusRunning
}
