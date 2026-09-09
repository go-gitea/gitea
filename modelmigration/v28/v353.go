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
		Groups []string `xorm:"JSON TEXT"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionRunner)); err != nil {
		return err
	}

	type ActionRunnerGroupRef struct {
		ID        int64  `xorm:"pk autoincr"`
		GroupName string `xorm:"VARCHAR(255) UNIQUE(group_repo) NOT NULL"`
		RepoID    int64  `xorm:"INDEX UNIQUE(group_repo) NOT NULL"`
	}
	return x.Sync(new(ActionRunnerGroupRef)) // plain Sync, the ignore flags above would skip the unique index
}
