// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddActionRunCommitSHAIndex indexes the runs lookup by commit, which the API `head_sha` filter uses.
func AddActionRunCommitSHAIndex(_ context.Context, x base.EngineMigration) error {
	type ActionRun struct {
		CommitSHA string `xorm:"index"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRun))
	return err
}
