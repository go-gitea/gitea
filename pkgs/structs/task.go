// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// TaskType defines task type
type TaskType int

// all kinds of task types
const (
	TaskTypeMigrateRepo TaskType = iota // migrate repository from external or local disk
)

// Name returns the task type name
func (taskType TaskType) Name() string {
	switch taskType {
	case TaskTypeMigrateRepo:
		return "Migrate Repository"
	}
	return ""
}

// TaskStatus defines task status
type TaskStatus int

// enumerate all the kinds of task status
const (
	TaskStatusQueue    TaskStatus = iota // 0 task is queue
	TaskStatusRunning                    // 1 task is running
	TaskStatusStopped                    // 2 task is stopped
	TaskStatusFailed                     // 3 task is failed
	TaskStatusFinished                   // 4 task is finished
)
