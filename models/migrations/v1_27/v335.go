// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddCommitCommentTable(x db.EngineMigration) error {
	type CommitComment struct {
		ID          int64              `xorm:"pk autoincr"`
		RepoID      int64              `xorm:"INDEX NOT NULL"`
		CommitSHA   string             `xorm:"VARCHAR(64) INDEX NOT NULL"`
		TreePath    string             `xorm:"VARCHAR(4000) NOT NULL"`
		Line        int64              `xorm:"NOT NULL"`
		PosterID    int64              `xorm:"INDEX NOT NULL"`
		Content     string             `xorm:"LONGTEXT NOT NULL"`
		Patch       string             `xorm:"LONGTEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(CommitComment))
	return err
}
