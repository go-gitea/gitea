// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddIssueWIP(_ context.Context, x base.EngineMigration) error {
	type Issue struct {
		RepoID   int64 `xorm:"INDEX(repo_wip)"`
		IsClosed bool  `xorm:"INDEX(repo_wip)"`
		IsPull   bool  `xorm:"INDEX(repo_wip)"`
		IsWIP    bool  `xorm:"NOT NULL DEFAULT false INDEX(repo_wip)"`
	}
	// Startup backfills using the current prefixes, also covering later configuration changes.
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Issue))
	return err
}
