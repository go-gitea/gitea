// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

func AddCommitCommentTable(x *xorm.Engine) error {
	// CommitComment is a junction table that maps commit-specific context
	// (repo, commit SHA) to a Comment entry. The actual comment content,
	// tree_path, line, poster, etc. live in the Comment table with
	// type = CommentTypeCommitComment (39).
	type CommitComment struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"INDEX NOT NULL"`
		CommitSHA string `xorm:"VARCHAR(64) INDEX NOT NULL"`
		CommentID int64  `xorm:"UNIQUE NOT NULL"`
	}

	return x.Sync(new(CommitComment))
}
