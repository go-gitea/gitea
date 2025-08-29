// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateTableIssueDevLink(x *xorm.Engine) error {
	type IssueDevLink struct {
		ID           int64 `xorm:"pk autoincr"`
		IssueID      int64 `xorm:"INDEX"`
		LinkType     int
		LinkedRepoID int64              `xorm:"INDEX"` // it can link to self repo or other repo
		LinkID       int64              // branch id in branch table or pull request id
		CreatedUnix  timeutil.TimeStamp `xorm:"INDEX created"`
	}
	return x.Sync(new(IssueDevLink))
}
