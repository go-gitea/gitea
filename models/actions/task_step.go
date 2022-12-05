// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// BotTaskStep represents a step of BotTask
type BotTaskStep struct {
	ID        int64
	Name      string
	TaskID    int64  `xorm:"index unique(task_number)"`
	Number    int64  `xorm:"index unique(task_number)"`
	RepoID    int64  `xorm:"index"`
	Status    Status `xorm:"index"`
	LogIndex  int64
	LogLength int64
	Started   timeutil.TimeStamp
	Stopped   timeutil.TimeStamp
	Created   timeutil.TimeStamp `xorm:"created"`
	Updated   timeutil.TimeStamp `xorm:"updated"`
}

func (step *BotTaskStep) TakeTime() time.Duration {
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
	db.RegisterModel(new(BotTaskStep))
}

func GetTaskStepsByTaskID(ctx context.Context, taskID int64) ([]*BotTaskStep, error) {
	var steps []*BotTaskStep
	return steps, db.GetEngine(ctx).Where("task_id=?", taskID).OrderBy("number").Find(&steps)
}
