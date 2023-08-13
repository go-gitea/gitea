// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddTableIssueContentHistory(x *xorm.Engine) error {
	type IssueContentHistory struct {
		ID             int64 `xorm:"pk autoincr"`
		PosterID       int64
		IssueID        int64              `xorm:"INDEX"`
		CommentID      int64              `xorm:"INDEX"`
		EditedUnix     timeutil.TimeStamp `xorm:"INDEX"`
		ContentText    string             `xorm:"LONGTEXT"`
		IsFirstCreated bool
		IsDeleted      bool
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Sync(new(IssueContentHistory)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return sess.Commit()
}
