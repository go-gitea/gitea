// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddActionRunRepoWorkflowIndex covers the DISTINCT workflow_id lookup of the Actions list page.
func AddActionRunRepoWorkflowIndex(_ context.Context, x base.EngineMigration) error {
	type ActionRun struct {
		RepoID      int64  `xorm:"index(repo_workflow)"`
		WorkflowID  string `xorm:"index(repo_workflow)"`
		IsScopedRun bool   `xorm:"NOT NULL DEFAULT false index(repo_workflow)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRun))
	return err
}
