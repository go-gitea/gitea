// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

// TaskStep represents a step of Task
type TaskStep struct {
	ID        int64
	TaskID    int64
	Number    int64
	Result    runnerv1.Result
	LogIndex  int64
	LogLength int64
	Started   timeutil.TimeStamp
	Stopped   timeutil.TimeStamp
	Created   timeutil.TimeStamp `xorm:"created"`
	Updated   timeutil.TimeStamp `xorm:"updated"`
}

func (TaskStep) TableName() string {
	return "bots_task_step"
}

func init() {
	db.RegisterModel(new(TaskStep))
}
