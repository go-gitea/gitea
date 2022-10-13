// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

// TaskLog represents a task's log, every task has a standalone table
type TaskLog struct {
	ID        int64
	Timestamp timeutil.TimeStamp
	Content   string `xorm:"LONGTEXT"`
}

func init() {
	db.RegisterModel(new(TaskLog))
}

func GetTaskLogTableName(taskID int64) string {
	return fmt.Sprintf("bots_task_log_%d", taskID)
}

// CreateTaskLog table for a task
func CreateTaskLog(taskID int64) error {
	return db.GetEngine(db.DefaultContext).
		Table(GetTaskLogTableName(taskID)).
		Sync(new(TaskLog))
}

func GetTaskLogs(taskID, index, length int64) (logs []*TaskLog, err error) {
	sess := db.GetEngine(db.DefaultContext).Table(GetBuildLogTableName(taskID)).
		Where("id>=?", index).OrderBy("id")

	if length > 0 {
		sess.Limit(int(length))
	}

	err = sess.Find(&logs)

	return
}

func InsertTaskLogs(taskID int64, logs []*TaskLog) (int64, error) {
	if err := CreateTaskLog(taskID); err != nil {
		return 0, err
	}
	table := GetTaskLogTableName(taskID)

	// TODO: A more complete way to insert logs
	// Be careful:
	//   - the id of a log can be 0
	//   - some logs may already exist in db
	//   - if use exec, consider different databases
	//   - the input should be ordered by id
	//   - the ids should be continuously increasing
	//   - the min id of input should be 1 + (the max id in db)

	if len(logs) == 0 {
		return 0, fmt.Errorf("no logs")
	}
	ack := logs[0].ID

	sess := db.GetEngine(db.DefaultContext)
	for _, v := range logs {
		_, err := sess.Exec(fmt.Sprintf("INSERT IGNORE INTO %s (id, timestamp, content) VALUES (?,?,?)", table), v.ID, v.Timestamp, []byte(v.Content))
		if err != nil {
			log.Error("insert log %d of task %d: %v", v.ID, taskID, err)
			break
		}
		ack = v.ID + 1
	}

	return ack, nil
}
