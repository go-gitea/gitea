// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddProjectWorkflow(_ context.Context, x base.EngineMigration) error {
	type ProjectWorkflow struct {
		ID              int64
		ProjectID       int64 `xorm:"INDEX"`
		WorkflowEvent   string
		WorkflowFilters string `xorm:"TEXT JSON"`
		WorkflowActions string `xorm:"TEXT JSON"`
		// SchemaVersion allows the shape of WorkflowFilters/WorkflowActions to change
		// in the future without an offline rewrite of every row, following the same
		// pattern as HookTask.PayloadVersion.
		SchemaVersion int                `xorm:"DEFAULT 1"`
		Enabled       bool               `xorm:"DEFAULT true NOT NULL"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(&ProjectWorkflow{})
}
