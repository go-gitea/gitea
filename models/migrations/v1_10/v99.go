// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddTaskTable(x *xorm.Engine) error {
	// TaskType defines task type
	type TaskType int

	// TaskStatus defines task status
	type TaskStatus int

	type Task struct {
		ID             int64
		DoerID         int64 `xorm:"index"` // operator
		OwnerID        int64 `xorm:"index"` // repo owner id, when creating, the repoID maybe zero
		RepoID         int64 `xorm:"index"`
		Type           TaskType
		Status         TaskStatus `xorm:"index"`
		StartTime      timeutil.TimeStamp
		EndTime        timeutil.TimeStamp
		PayloadContent string             `xorm:"TEXT"`
		Errors         string             `xorm:"TEXT"` // if task failed, saved the error reason
		Created        timeutil.TimeStamp `xorm:"created"`
	}

	type Repository struct {
		Status int `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(Task), new(Repository))
}
