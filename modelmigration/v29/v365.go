// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddActionTaskRunnerIDIDIndex indexes "WHERE runner_id=? ORDER BY id DESC", which the runner task list uses.
// Without it, databases may walk the whole primary key backwards to find the latest tasks of a rarely used runner.
func AddActionTaskRunnerIDIDIndex(_ context.Context, x base.EngineMigration) error {
	// RunnerID must stay before ID: xorm orders composite index columns by field order
	type ActionTask struct {
		RunnerID int64 `xorm:"index(runner_id_id)"`
		ID       int64 `xorm:"pk autoincr index(runner_id_id)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionTask))
	return err
}
