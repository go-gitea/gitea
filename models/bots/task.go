// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/google/uuid"
)

// TaskStatus represents a task status
type TaskStatus int

// enumerate all the statuses of bot task
const (
	TaskPending  TaskStatus = iota // wait for assign
	TaskAssigned                   // assigned to a runner
	TaskRunning                    // running
	TaskFailed
	TaskFinished
	TaskCanceled
	TaskTimeout
)

// Task represnets bot tasks
type Task struct {
	ID            int64
	UUID          string `xorm:"CHAR(36)"`
	RepoID        int64  `xorm:"index"`
	TriggerUserID int64
	Ref           string
	CommitSHA     string
	Event         webhook.HookEventType
	Token         string             // token for this task
	Grant         string             // permissions for this task
	EventPayload  string             `xorm:"LONGTEXT"`
	RunnerID      int64              `xorm:"index"`
	Status        TaskStatus         `xorm:"index"`
	Created       timeutil.TimeStamp `xorm:"created"`
	StartTime     timeutil.TimeStamp
	EndTime       timeutil.TimeStamp
	Updated       timeutil.TimeStamp `xorm:"updated"`
}

// TableName represents a bot task
func (Task) TableName() string {
	return "actions_task"
}

// InsertTask inserts a bot task
func InsertTask(t *Task) error {
	if t.UUID == "" {
		t.UUID = uuid.New().String()
	}
	return db.Insert(db.DefaultContext, t)
}

// UpdateTask updates bot task
func UpdateTask(t *Task, cols ...string) error {
	_, err := db.GetEngine(db.DefaultContext).ID(t.ID).Cols(cols...).Update(t)
	return err
}

// ErrTaskNotExist represents an error for bot task not exist
type ErrTaskNotExist struct {
	UUID string
}

func (err ErrTaskNotExist) Error() string {
	return fmt.Sprintf("Bot task [%s] is not exist", err.UUID)
}

// GetTaskByUUID gets bot task by uuid
func GetTaskByUUID(taskUUID string) (*Task, error) {
	var task Task
	has, err := db.GetEngine(db.DefaultContext).Where("uuid=?", taskUUID).Get(&task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTaskNotExist{
			UUID: taskUUID,
		}
	}
	return &task, nil
}

// GetCurTask return the task for the bot
func GetCurTask(runnerID int64) (*Task, error) {
	var tasks []Task
	// FIXME: for test, just return all tasks
	err := db.GetEngine(db.DefaultContext).Where("status=?", TaskPending).Find(&tasks)
	// err := x.Where("runner_id = ?", botID).
	// And("status=?", BotTaskPending).
	// Find(&tasks)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return &tasks[0], err
}

// AssignTaskToRunner assign a task to a runner
func AssignTaskToRunner(taskID int64, runnerID int64) error {
	cnt, err := db.GetEngine(db.DefaultContext).
		Where("runner_id=0").
		And("id=?", taskID).
		Cols("runner_id").
		Update(&Task{
			RunnerID: runnerID,
		})
	if err != nil {
		return err
	}
	if cnt != 1 {
		return errors.New("assign faild")
	}
	return nil
}

type TaskStage struct{}

type StageStep struct{}
