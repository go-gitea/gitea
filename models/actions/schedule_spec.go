// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
)

// ActionScheduleSpec represents a schedule spec of a workflow file
type ActionScheduleSpec struct {
	ID         int64
	RepoID     int64                  `xorm:"index"`
	Repo       *repo_model.Repository `xorm:"-"`
	ScheduleID int64                  `xorm:"index"`
	Schedule   *ActionSchedule        `xorm:"-"`

	Spec string
}

func init() {
	db.RegisterModel(new(ActionScheduleSpec))
}
