// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
)

type commitComment struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"INDEX(repo_commit) NOT NULL"`
	CommitSHA string `xorm:"INDEX(repo_commit) NOT NULL"`
	CommentID int64  `xorm:"INDEX UNIQUE NOT NULL"`
}

func (commitComment) TableName() string {
	return "commit_comment"
}

func AddCommitCommentTable(_ context.Context, x base.EngineMigration) error {
	return x.Sync(new(commitComment))
}
