// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

// TaskStep represents a step of Task
type TaskStep struct {
	ID        int64
	Name      string
	TaskID    int64 `xorm:"index unique(task_number)"`
	Number    int64 `xorm:"index unique(task_number)"`
	Result    runnerv1.Result
	Status    Status `xorm:"index"`
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

func (step *TaskStep) TakeTime() time.Duration {
	if step.Started == 0 {
		return 0
	}
	started := step.Started.AsTime()
	if step.Status.IsDone() {
		return step.Stopped.AsTime().Sub(started)
	}
	step.Stopped.AsTime().Sub(started)
	return time.Since(started).Truncate(time.Second)
}

func init() {
	db.RegisterModel(new(TaskStep))
}

func GetTaskStepsByTaskID(ctx context.Context, taskID int64) ([]*TaskStep, error) {
	var steps []*TaskStep
	return steps, db.GetEngine(ctx).Where("task_id=?", taskID).OrderBy("number").Find(&steps)
}
