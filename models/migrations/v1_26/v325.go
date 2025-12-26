// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddProjectWorkflow(x *xorm.Engine) error {
	type ProjectWorkflow struct {
		ID              int64
		ProjectID       int64              `xorm:"INDEX"`
		WorkflowEvent   string             `xorm:"INDEX"`
		WorkflowFilters string             `xorm:"TEXT JSON"`
		WorkflowActions string             `xorm:"TEXT JSON"`
		Enabled         bool               `xorm:"DEFAULT true"`
		CreatedUnix     timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(&ProjectWorkflow{})
}
