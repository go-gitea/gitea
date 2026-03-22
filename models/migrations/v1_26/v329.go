// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

func AddCommitCodeCommentTable(x *xorm.Engine) error {
	type CommitCodeComment struct {
		ID          int64  `xorm:"pk autoincr"`
		RepoID      int64  `xorm:"INDEX"`
		CommitSHA   string `xorm:"VARCHAR(64) INDEX"`
		PosterID    int64  `xorm:"INDEX"`
		TreePath    string `xorm:"VARCHAR(4000)"`
		Line        int64
		Content     string `xorm:"LONGTEXT"`
		Patch       string `xorm:"LONGTEXT"`
		CreatedUnix int64  `xorm:"INDEX created"`
		UpdatedUnix int64  `xorm:"INDEX updated"`
	}
	return x.Sync(new(CommitCodeComment))
}
