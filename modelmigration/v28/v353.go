// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddCommitCommentJunction creates the commit_comment junction table that
// anchors Comment rows (CommentTypeCommitComment) onto a repo commit diff
// coordinate without adding indexes to the main comment table.
func AddCommitCommentJunction(_ context.Context, x base.EngineMigration) error {
	type CommitComment struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"INDEX(s) NOT NULL"`
		CommitSHA string `xorm:"VARCHAR(64) INDEX(s) NOT NULL"`
		TreePath  string `xorm:"VARCHAR(4000) NOT NULL"`
		Line      int64  `xorm:"NOT NULL"`
		CommentID int64  `xorm:"UNIQUE INDEX NOT NULL"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(CommitComment))
	return err
}
