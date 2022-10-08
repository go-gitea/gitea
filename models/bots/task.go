// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// Task represents a distribution of job
type Task struct {
	ID        int64
	JobID     int64
	Attempt   int64
	RunnerID  int64  `xorm:"index"`
	LogToFile bool   // read log from database or from storage
	LogUrl    string // url of the log file in storage
	Result    int64  // TODO: use runnerv1.Result
	Started   timeutil.TimeStamp
	Stopped   timeutil.TimeStamp
	Created   timeutil.TimeStamp `xorm:"created"`
	Updated   timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(Task))
}

func (Task) TableName() string {
	return "bots_task"
}
