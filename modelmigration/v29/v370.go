// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

func AddCommitComment(_ context.Context, x base.EngineMigration) error {
	type CommitComment struct {
		ID             int64  `xorm:"pk autoincr"`
		RepoID         int64  `xorm:"INDEX NOT NULL"`
		CommitSHA      string `xorm:"VARCHAR(64) INDEX NOT NULL"`
		PosterID       int64  `xorm:"INDEX NOT NULL"`
		TreePath       string `xorm:"VARCHAR(4000)"`
		Line           int64
		Content        string             `xorm:"LONGTEXT"`
		ContentVersion int                `xorm:"NOT NULL DEFAULT 0"`
		CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	type Attachment struct {
		CommitCommentID int64 `xorm:"INDEX NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(CommitComment), new(Attachment))
	return err
}
