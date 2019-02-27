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

// TaskStatus defines task status
type TaskStatus int

const (
	TaskStatusQueue    TaskStatus = iota // 0 task is queue
	TaskStatusRunning                    // 1 task is running
	TaskStatusStopped                    // 2 task is stopped
	TaskStatusFailed                     // 3 task is failed
	TaskStatusFinished                   // 4 task is finished
)

// MigrateRepoOptions contains the repository migrate options
type MigrateRepoOptions struct {
	Name                 string
	Description          string
	OriginalURL          string
	IsPrivate            bool
	IsMirror             bool
	RemoteAddr           string
	Wiki                 bool // include wiki repository
	SyncReleasesWithTags bool // sync releases from tags
}
