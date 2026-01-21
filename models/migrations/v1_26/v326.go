// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddCommitComments(x *xorm.Engine) error {
	type CommitComment struct {
		ID          int64              `xorm:"pk autoincr"`
		RepoID      int64              `xorm:"INDEX"`
		CommitSHA   string             `xorm:"VARCHAR(64) INDEX"`
		PosterID    int64              `xorm:"INDEX"`
		Path        string             `xorm:"VARCHAR(4000)"`
		Line        int64              `xorm:"INDEX"`
		Content     string             `xorm:"LONGTEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	type CommitCommentReaction struct {
		ID               int64              `xorm:"pk autoincr"`
		Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
		CommitCommentID  int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
		UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
		OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
		OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
		CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
	}

	return x.Sync(new(CommitComment), new(CommitCommentReaction))
}
