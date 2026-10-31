// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddWatchRepoIDIndex indexes watch by repo_id, the unique (user_id, repo_id) index can't serve repo-only lookups.
func AddWatchRepoIDIndex(_ context.Context, x base.EngineMigration) error {
	type Watch struct {
		RepoID int64 `xorm:"INDEX"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Watch))
	return err
}
