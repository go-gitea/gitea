// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

// AddActionQueueIndexes adds the composite indexes the Actions job lookups need:
// "pickup" (task_id, status, updated) matches the runner-poll query's WHERE task_id=0 AND status=waiting
// ORDER BY updated, id, while (repo_id, status) on both action_run_job and action_run backs the
// repository-scoped status lookups, which so far had to scan every row of a repository.
func AddActionQueueIndexes(_ context.Context, x base.EngineMigration) error {
	type ActionRunJob struct {
		RepoID  int64              `xorm:"index(repo_status)"`
		TaskID  int64              `xorm:"index(pickup)"`
		Status  int                `xorm:"index(pickup) index(repo_status)"`
		Updated timeutil.TimeStamp `xorm:"index(pickup)"`
	}

	type ActionRun struct {
		RepoID int64 `xorm:"index(repo_status)"`
		Status int   `xorm:"index(repo_status)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRunJob), new(ActionRun))
	return err
}
