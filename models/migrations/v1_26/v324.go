// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddCommitCommentTable(x *xorm.Engine) error {
	type CommitComment struct {
		ID               int64  `xorm:"pk autoincr"`
		RepoID           int64  `xorm:"INDEX"`
		CommitSHA        string `xorm:"VARCHAR(64) INDEX"`
		TreePath         string `xorm:"VARCHAR(4000)"`
		Line             int64
		Content          string `xorm:"LONGTEXT"`
		ContentVersion   int    `xorm:"NOT NULL DEFAULT 0"`
		PosterID         int64  `xorm:"INDEX"`
		OriginalAuthor   string
		OriginalAuthorID int64
		CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix      timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	return x.Sync2(new(CommitComment))
}
