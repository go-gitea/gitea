// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// SplitActionTableAsTwoTables splits the action table as two tables.
func SplitActionTableAsTwoTables(x *xorm.Engine) error {
	// 1 create new table
	type UserFeed struct {
		ID          int64              `xorm:"pk autoincr"`
		UserID      int64              `xorm:"UNIQUE(s)"` // Receiver user id.
		ActionID    int64              `xorm:"UNIQUE(s)"` // refer to action table
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
	}

	if err := x.Sync(new(UserFeed)); err != nil {
		return err
	}

	// 2 copy data from action table to new table
	if _, err := x.Exec("INSERT INTO `user_feed` (`user_id`, `action_id`, `created_unix`) SELECT `user_id`, `id`, `created_unix` FROM `action`"); err != nil {
		return err
	}

	// 3 merge records from action table
	type result struct {
		IssueID   int64
		ProjectID int64
		Cnt       int
	}
	var results []result
	if err := x.Select("op_type, act_user_id, repo_id, count(*) as cnt").
		Table("project_issue").
		GroupBy("issue_id, project_id").
		Having("count(*) > 1").
		Find(&results); err != nil {
		return err
	}

	// 4 update user_feed table to update action_id because of the merge records

	// 5 drop column from action table
}
