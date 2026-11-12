// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddScopedWorkflowRepoFilter(_ context.Context, x base.EngineMigration) error {
	type ActionScopedWorkflowSource struct {
		RepoFilter string `xorm:"TEXT 'repo_filter'"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true, IgnoreConstrains: true}, new(ActionScopedWorkflowSource))
	return err
}
