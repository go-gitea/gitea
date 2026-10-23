// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddProjectWorkflow(_ context.Context, x base.EngineMigration) error {
	type ProjectWorkflow struct {
		ID              int64 `xorm:"pk autoincr"`
		ProjectID       int64 `xorm:"INDEX"`
		WorkflowEvent   string
		WorkflowFilters string             `xorm:"TEXT JSON"`
		WorkflowActions string             `xorm:"TEXT JSON"`
		Enabled         bool               `xorm:"DEFAULT true NOT NULL"`
		CreatedUnix     timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(&ProjectWorkflow{})
}
