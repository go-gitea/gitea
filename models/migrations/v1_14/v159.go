// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func UpdateReactionConstraint(x *xorm.Engine) error {
	// Reaction represents a reactions on issues and comments.
	type Reaction struct {
		ID               int64              `xorm:"pk autoincr"`
		Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
		IssueID          int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
		CommentID        int64              `xorm:"INDEX UNIQUE(s)"`
		UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
		OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
		OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
		CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := base.RecreateTable(sess, &Reaction{}); err != nil {
		return err
	}

	return sess.Commit()
}
