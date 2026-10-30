// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type actionTaskRunnerIDID struct {
	ID       int64 `xorm:"pk autoincr"`
	RunnerID int64
}

func (*actionTaskRunnerIDID) TableName() string {
	return "action_task"
}

func (*actionTaskRunnerIDID) TableIndices() []*schemas.Index {
	runnerIDIndex := schemas.NewIndex("runner_id_id", schemas.IndexType)
	runnerIDIndex.AddColumn("runner_id", "id")
	return []*schemas.Index{runnerIDIndex}
}

func AddActionTaskRunnerIDIDIndex(ctx context.Context, x base.EngineMigration) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(actionTaskRunnerIDID)); err != nil {
		return err
	}
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "action_task")
	if err != nil {
		return err
	}
	if idx, ok := indexes["runner_id"]; ok {
		_, err = x.Exec(x.Dialect().DropIndexSQL("action_task", idx))
	}
	return err
}
