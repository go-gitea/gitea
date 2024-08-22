// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

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
		LinkIndex    string             // branch name, pull request number or commit sha
		CreatedUnix  timeutil.TimeStamp `xorm:"INDEX created"`
	}
	return x.Sync(new(IssueDevLink))
}
