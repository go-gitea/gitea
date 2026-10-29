// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// actionTaskRunnerIDID lists only the composite index, so the other indexes of the table must be kept
type actionTaskRunnerIDID struct {
	ID       int64 `xorm:"pk autoincr"`
	RunnerID int64
}

func (*actionTaskRunnerIDID) TableName() string {
	return "action_task"
}

// TableIndices implements xorm's TableIndices interface
func (*actionTaskRunnerIDID) TableIndices() []*schemas.Index {
	runnerIDIndex := schemas.NewIndex("runner_id_id", schemas.IndexType)
	runnerIDIndex.AddColumn("runner_id", "id")
	return []*schemas.Index{runnerIDIndex}
}

// AddActionTaskRunnerIDIDIndex indexes "WHERE runner_id=? ORDER BY id DESC", which the runner task list uses.
// Without it, databases may walk the whole primary key backwards to find the latest tasks of a rarely used runner.
func AddActionTaskRunnerIDIDIndex(_ context.Context, x base.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(actionTaskRunnerIDID))
	return err
}
