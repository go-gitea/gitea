// Copyright 2019 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// TaskType defines task type
type TaskType int

const TaskTypeMigrateRepo TaskType = iota // migrate repository from external or local disk

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
	TaskStatusQueued   TaskStatus = iota // 0 task is queued
	TaskStatusRunning                    // 1 task is running
	TaskStatusStopped                    // 2 task is stopped (never used)
	TaskStatusFailed                     // 3 task is failed
	TaskStatusFinished                   // 4 task is finished
)
