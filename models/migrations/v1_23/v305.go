// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// SplitActionTableAsTwoTables splits the action table as two tables.
func SplitActionTableAsTwoTables(x *xorm.Engine) error {
	// 1 create new table
	type UserFeed struct {
		ID          int64              `xorm:"pk autoincr"`
		UserID      int64              `xorm:"UNIQUE(s)"` // Receiver user id.
		ActivityID  int64              `xorm:"UNIQUE(s)"` // refer to action table
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
	}

	type UserActivity struct {
		ID          int64 `xorm:"pk autoincr"`
		OpType      int
		ActUserID   int64 // Action user id.
		RepoID      int64
		CommentID   int64
		IsDeleted   bool `xorm:"NOT NULL DEFAULT false"`
		RefName     string
		IsPrivate   bool               `xorm:"NOT NULL DEFAULT false"`
		Content     string             `xorm:"TEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
	}

	if err := x.Sync(new(UserFeed), new(UserActivity)); err != nil {
		return err
	}

	// 2 copy data from action table to new table
	if _, err := x.Exec("INSERT INTO `user_feed` (`user_id`, `action_id`, `created_unix`) SELECT `user_id`, `id`, `created_unix` FROM `action`"); err != nil {
		return err
	}

	// 3 merge records from action table
	if _, err := x.Exec("INSERT INTO `user_activity` (`op_type`, `act_user_id`, `repo_id`, `comment_id`, `is_deleted`, `ref_name`, `is_private`, `content`, `created_unix`) SELECT `op_type`, `act_user_id`, `repo_id`, `comment_id`, `is_deleted`, `ref_name`, `is_private`, `content`, `created_unix` FROM `action`"); err != nil {
		return err
	}

	// 4 update user_feed table to update action_id because of the merge records

	// 5 drop column from action table

	return fmt.Errorf("not implemented")
}
