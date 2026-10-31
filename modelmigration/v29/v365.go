// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddCommitCommentTable(_ctx context.Context, engine base.EngineMigration) error {
	type CommitComment struct {
		CommentID int64  `xorm:"pk"`
		RepoID    int64  `xorm:"INDEX(s) NOT NULL"`
		CommitSHA string `xorm:"VARCHAR(64) INDEX(s) NOT NULL"`
	}
	return engine.Sync(new(CommitComment))
}
