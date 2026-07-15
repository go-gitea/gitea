// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

func AddCommitCommentTable(x db.EngineMigration) error {
	type CommitComment struct {
		ID        int64             `xorm:"pk autoincr"`
		RepoID    int64             `xorm:"INDEX NOT NULL DEFAULT 0"`
		CommitSHA string            `xorm:"VARCHAR(64) INDEX NOT NULL DEFAULT ''"`
		TreePath  string            `xorm:"VARCHAR(4000) NOT NULL DEFAULT ''"`
		Line      int64             `xorm:"NOT NULL DEFAULT 0"`
		Content   string            `xorm:"LONGTEXT NOT NULL"`
		Patch     string            `xorm:"LONGTEXT"`
		PosterID  int64             `xorm:"INDEX NOT NULL DEFAULT 0"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	return x.Sync(new(CommitComment))
}
