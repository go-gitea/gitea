// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

func AddCommitCommentTable(x base.EngineMigration) error {
	type CommitComment struct {
		ID          int64              `xorm:"pk autoincr"`
		RepoID      int64              `xorm:"INDEX(s) NOT NULL"`
		CommitSHA   string             `xorm:"VARCHAR(64) INDEX(s) NOT NULL"`
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
