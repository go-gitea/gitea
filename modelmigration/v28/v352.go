// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func CreateTableIssueDevLink(_ context.Context, x base.EngineMigration) error {
	type IssueDevLink struct {
		ID           int64 `xorm:"pk autoincr"`
		IssueID      int64 `xorm:"INDEX"`
		LinkType     int
		LinkedRepoID int64 `xorm:"INDEX"`
		LinkID       int64
		CreatedUnix  timeutil.TimeStamp `xorm:"INDEX created"`
	}
	return x.Sync(new(IssueDevLink))
}
