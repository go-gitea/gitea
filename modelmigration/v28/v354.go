// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

// AddActionQueueIndexes indexes the runner pickup query and repository-scoped status lookups.
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
