// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

// ActionTaskOutput represents an output of ActionTask.
// So the outputs are bound to a task, that means when a completed job has been rerun,
// the outputs of the job will be reset because the task is new.
// It's by design, to avoid the outputs of the old task to be mixed with the new task.
type ActionTaskOutput struct {
	ID          int64
	TaskID      int64  `xorm:"INDEX UNIQUE(task_id_output_key)"`
	OutputKey   string `xorm:"VARCHAR(255) UNIQUE(task_id_output_key)"`
	OutputValue string `xorm:"MEDIUMTEXT"`
}

func init() {
	db.RegisterModel(new(ActionTaskOutput))
}

// FindTaskOutputByTaskID returns the outputs of the task.
func FindTaskOutputByTaskID(ctx context.Context, taskID int64) ([]*ActionTaskOutput, error) {
	var outputs []*ActionTaskOutput
	return outputs, db.GetEngine(ctx).Where("task_id=?", taskID).Find(&outputs)
}

// FindTaskOutputKeyByTaskID returns the keys of the outputs of the task.
func FindTaskOutputKeyByTaskID(ctx context.Context, taskID int64) ([]string, error) {
	var keys []string
	return keys, db.GetEngine(ctx).Table(ActionTaskOutput{}).Where("task_id=?", taskID).Cols("output_key").Find(&keys)
}

// InsertTaskOutputIfNotExist inserts a new task output if it does not exist.
func InsertTaskOutputIfNotExist(ctx context.Context, taskID int64, key, value string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)
		if exist, err := sess.Exist(&ActionTaskOutput{TaskID: taskID, OutputKey: key}); err != nil {
			return err
		} else if exist {
			return nil
		}
		_, err := sess.Insert(&ActionTaskOutput{
			TaskID:      taskID,
			OutputKey:   key,
			OutputValue: value,
		})
		return err
	})
}
