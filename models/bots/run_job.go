// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// RunJob represents a job of a run
type RunJob struct {
	ID              int64
	RunID           int64
	Run             *Run `xorm:"-"`
	Name            string
	Ready           bool // ready to be executed
	Attempt         int64
	WorkflowPayload []byte
	JobID           string           // job id in workflow, not job's id
	Needs           []int64          `xorm:"JSON TEXT"`
	RunsOn          []string         `xorm:"JSON TEXT"`
	TaskID          int64            // the latest task of the job
	Status          core.BuildStatus `xorm:"index"`
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
	if err := job.Run.LoadAttributes(ctx); err != nil {
		return err
	}

	return nil
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
		return nil, ErrRunNotExist{
			ID: id,
		}
	}

	return &job, nil
}
