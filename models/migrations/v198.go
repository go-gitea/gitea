// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addTableIssueContentHistory(x *xorm.Engine) error {
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
	if err := sess.Sync2(new(IssueContentHistory)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return sess.Commit()
}
