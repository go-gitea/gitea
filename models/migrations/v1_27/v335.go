// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

func AddProjectWorkflow(x db.EngineMigration) error {
	type ProjectWorkflow struct {
		ID              int64
		ProjectID       int64 `xorm:"INDEX"`
		WorkflowEvent   string
		WorkflowFilters string             `xorm:"TEXT JSON"`
		WorkflowActions string             `xorm:"TEXT JSON"`
		Enabled         bool               `xorm:"DEFAULT true"`
		CreatedUnix     timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(&ProjectWorkflow{})
}
