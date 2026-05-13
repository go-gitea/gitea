// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "xorm.io/xorm"

// AddCommitCommentTable creates the commit_comment junction table.
// Each row maps a commit (repo_id + commit_sha) to a Comment row
// (type = CommentTypeCommitComment). Keeping a separate table with an
// index on commit_sha avoids a full-table scan on the large comment table.
func AddCommitCommentTable(x *xorm.Engine) error {
	type CommitComment struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"INDEX NOT NULL"`
		CommitSHA string `xorm:"VARCHAR(64) INDEX NOT NULL"`
		CommentID int64  `xorm:"UNIQUE NOT NULL"`
	}
	return x.Sync(new(CommitComment))
}
