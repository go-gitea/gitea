// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint
import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AdjustIssueWatchIndexOrder(x *xorm.Engine) error {
	type IssueWatch struct {
		ID          int64              `xorm:"pk autoincr"`
		UserID      int64              `xorm:"UNIQUE(watch) NOT NULL"`
		IssueID     int64              `xorm:"UNIQUE(watch) NOT NULL"`
		IsWatching  bool               `xorm:"NOT NULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
	}
	// Drop the old index :(user_id,issue_id)
	// Then automatically created new index => (issue_id,user_id)
	return x.DropIndexes(new(IssueWatch))
}
