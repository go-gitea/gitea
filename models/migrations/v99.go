// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addTaskTable(x *xorm.Engine) error {
	type Task struct {
		ID             int64
		DoerID         int64 `xorm:"index"` // operator
		OwnerID        int64 `xorm:"index"` // repo owner id, when creating, the repoID maybe zero
		RepoID         int64 `xorm:"index"`
		Type           structs.TaskType
		Status         structs.TaskStatus `xorm:"index"`
		StartTime      timeutil.TimeStamp
		EndTime        timeutil.TimeStamp
		PayloadContent string             `xorm:"TEXT"`
		Errors         string             `xorm:"TEXT"` // if task failed, saved the error reason
		Created        timeutil.TimeStamp `xorm:"created"`
	}

	type Repository struct {
		Status int `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync2(new(Task), new(Repository))
}
