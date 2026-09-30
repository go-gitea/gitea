// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddWorkflowPathToActions(_ context.Context, x base.EngineMigration) error {
	type ActionRun struct {
		WorkflowPath string `xorm:"TEXT"`
	}
	type ActionSchedule struct {
		WorkflowPath string `xorm:"TEXT"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRun), new(ActionSchedule))
	return err
}
