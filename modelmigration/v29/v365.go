// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

func AddActionTaskRunnerIDCreatedIndex(ctx context.Context, x base.EngineMigration) error {
	type ActionTask struct {
		RunnerID int64              `xorm:"index(runner_id_created)"`
		Created  timeutil.TimeStamp `xorm:"created index(runner_id_created)"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionTask)); err != nil {
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
