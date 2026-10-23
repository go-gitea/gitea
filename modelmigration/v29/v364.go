// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddProjectWorkflowTable(_ctx context.Context, x base.EngineMigration) error {
	type ProjectWorkflow struct {
		ID        int64  `xorm:"pk autoincr"`
		ProjectID int64  `xorm:"UNIQUE(project_event) NOT NULL"`
		Event     string `xorm:"UNIQUE(project_event) VARCHAR(32) NOT NULL"`
		ColumnID  int64  `xorm:"NOT NULL"`
		Enabled   bool   `xorm:"NOT NULL DEFAULT false"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	return x.Sync(new(ProjectWorkflow))
}
