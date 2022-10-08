// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// TaskLog represents a task's log, every task has a standalone table
type TaskLog struct {
	ID      int64
	Content string             `xorm:"BINARY"`
	Created timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(TaskLog))
}

func GetTaskLogTableName(taskID int64) string {
	return fmt.Sprintf("bots_task_log_%d", taskID)
}

// CreateTaskLog table for a build
func CreateTaskLog(buildID int64) error {
	return db.GetEngine(db.DefaultContext).
		Table(GetBuildLogTableName(buildID)).
		Sync2(new(BuildLog))
}

func GetTaskLogs(taskID, index, length int64) (logs []*TaskLog, err error) {
	sess := db.GetEngine(db.DefaultContext).Table(GetBuildLogTableName(taskID)).
		Where("id>=?", index)

	if length > 0 {
		sess.Limit(int(length))
	}

	err = sess.Find(&logs)

	return
}
