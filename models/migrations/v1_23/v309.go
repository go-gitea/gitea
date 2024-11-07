// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type pullAutoMerge struct {
	ID                     int64  `xorm:"pk autoincr"`
	PullID                 int64  `xorm:"UNIQUE"`
	DoerID                 int64  `xorm:"INDEX NOT NULL"`
	MergeStyle             string `xorm:"varchar(30)"`
	Message                string `xorm:"LONGTEXT"`
	DeleteBranchAfterMerge bool
	CreatedUnix            timeutil.TimeStamp `xorm:"created"`
}

// TableName return database table name for xorm
func (pullAutoMerge) TableName() string {
	return "pull_auto_merge"
}

func AddDeleteBranchAfterMergeForAutoMerge(x *xorm.Engine) error {
	return x.Sync(new(pullAutoMerge))
}
