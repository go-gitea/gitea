// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// RunJob represents a job of a run
type RunJob struct {
	ID              int64
	RunID           int64
	Name            string
	WorkflowPayload string           `xorm:"LONGTEXT"`
	Needs           []int64          `xorm:"JSON TEXT"`
	TaskID          int64            // the latest task of the job
	Status          core.BuildStatus `xorm:"index"`
	StartTime       timeutil.TimeStamp
	EndTime         timeutil.TimeStamp
	Created         timeutil.TimeStamp `xorm:"created"`
	Updated         timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(RunJob))
}

func (RunJob) TableName() string {
	return "bots_run_job"
}
