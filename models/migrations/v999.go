// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func createIssuePriorityTables(x *xorm.Engine) error {
	// Priority represents a priority of repository for issues.
	type Priority struct {
		ID              int64 `xorm:"pk autoincr"`
		RepoID          int64 `xorm:"INDEX"`
		OrgID           int64 `xorm:"INDEX"`
		Name            string
		Description     string
		Color           string `xorm:"VARCHAR(7)"`
		Weight          int
		NumIssues       int
		NumClosedIssues int
		CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	if err := x.Sync2(new(Priority)); err != nil {
		return err
	}

	type Comment struct {
		ID         int64 `xorm:"pk autoincr"`
		PriorityID int64
	}
	if err := x.Sync2(new(Comment)); err != nil {
		return err
	}

	type Issue struct {
		ID         int64 `xorm:"pk autoincr"`
		PriorityID int64
	}

	return x.Sync2(new(Issue))
}
