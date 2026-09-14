// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddRunnerGroups(_ context.Context, x base.EngineMigration) error {
	type ActionRunner struct {
		GroupID int64 `xorm:"INDEX NOT NULL DEFAULT 0"`
	}
	type ActionRunJob struct {
		RunsOnGroup string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionRunner), new(ActionRunJob)); err != nil {
		return err
	}

	type ActionRunnerGroup struct {
		ID      int64  `xorm:"pk autoincr"`
		OwnerID int64  `xorm:"UNIQUE(owner_name) NOT NULL DEFAULT 0"`
		Name    string `xorm:"VARCHAR(255) UNIQUE(owner_name) NOT NULL"`
	}
	type ActionRunnerAccess struct {
		ID      int64 `xorm:"pk autoincr"`
		GroupID int64 `xorm:"UNIQUE(group_repo) NOT NULL"`
		RepoID  int64 `xorm:"INDEX UNIQUE(group_repo) NOT NULL"`
	}
	return x.Sync(new(ActionRunnerGroup), new(ActionRunnerAccess)) // plain Sync, the ignore flags above would skip the unique indexes
}
