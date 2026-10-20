// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddReleaseIDToReaction(_ctx context.Context, x base.EngineMigration) error {
	type Reaction struct {
		ID int64 `xorm:"pk autoincr"`

		UserID           int64  `xorm:"UNIQUE(s) NOT NULL"` // first unique column implies INDEX
		IssueID          int64  `xorm:"UNIQUE(s) INDEX(issue_comment) NOT NULL"`
		CommentID        int64  `xorm:"UNIQUE(s) INDEX(issue_comment) INDEX NOT NULL"`
		ReleaseID        int64  `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT(0)"`
		OriginalAuthorID int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT(0)"`
		OriginalAuthor   string `xorm:"UNIQUE(s) NOT NULL DEFAULT('')"`
		Type             string `xorm:"UNIQUE(s) NOT NULL"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	}
	return x.Sync(new(Reaction))
}
