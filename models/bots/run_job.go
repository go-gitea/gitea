// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/builder"

	"golang.org/x/exp/slices"
)

// RunJob represents a job of a run
type RunJob struct {
	ID              int64
	RunID           int64 `xorm:"index"`
	Run             *Run  `xorm:"-"`
	Name            string
	Ready           bool // ready to be executed
	Attempt         int64
	WorkflowPayload []byte
	JobID           string   // job id in workflow, not job's id
	Needs           []int64  `xorm:"JSON TEXT"`
	RunsOn          []string `xorm:"JSON TEXT"`
	TaskID          int64    // the latest task of the job
	Status          Status   `xorm:"index"`
	Started         timeutil.TimeStamp
	Stopped         timeutil.TimeStamp
	Created         timeutil.TimeStamp `xorm:"created"`
	Updated         timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(RunJob))
}

func (RunJob) TableName() string {
	return "bots_run_job"
}

// LoadAttributes load Run if not loaded
func (job *RunJob) LoadAttributes(ctx context.Context) error {
	if job == nil {
		return nil
	}

	if job.Run == nil {
		run, err := GetRunByID(ctx, job.RunID)
		if err != nil {
			return err
		}
		job.Run = run
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

func GetRunJobByID(ctx context.Context, id int64) (*RunJob, error) {
	var job RunJob
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

func GetRunJobsByRunID(ctx context.Context, runID int64) ([]*RunJob, error) {
	var jobs []*RunJob
	if err := db.GetEngine(ctx).Where("run_id=?", runID).OrderBy("id").Find(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func UpdateRunJob(ctx context.Context, job *RunJob, cond builder.Cond, cols ...string) (int64, error) {
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

	run := &Run{
		ID:     job.RunID,
		Status: runStatus,
	}
	if runStatus.IsDone() {
		run.Stopped = timeutil.TimeStampNow()
	}
	return affected, UpdateRun(ctx, run)
}

func aggregateJobStatus(jobs []*RunJob) Status {
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
		if job.Status == StatusFailure {
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
