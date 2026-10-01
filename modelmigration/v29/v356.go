// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

// CoverActionRunJobPickupIndex rebuilds the pickup index to cover the runner pickup query, avoiding SQL Server lookup deadlocks
func CoverActionRunJobPickupIndex(ctx context.Context, x base.EngineMigration) error {
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "action_run_job")
	if err != nil {
		return err
	}
	if idx, ok := indexes["pickup"]; ok {
		if _, err := x.Exec(x.Dialect().DropIndexSQL("action_run_job", idx)); err != nil {
			return err
		}
	}

	pickup := schemas.NewIndex("pickup", schemas.IndexType)
	pickup.AddColumn("task_id", "status", "is_reusable_caller", "updated", "id", "repo_id")
	_, err = x.Exec(x.Dialect().CreateIndexSQL("action_run_job", pickup))
	return err
}
