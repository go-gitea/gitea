// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddBranchTable(x *xorm.Engine) error {
	type Branch struct {
		ID          int64
		RepoID      int64  `xorm:"index UNIQUE(s)"`
		Name        string `xorm:"UNIQUE(s) NOT NULL"`
		Commit      string
		PusherID    int64
		CommitTime  timeutil.TimeStamp // The commit
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(new(Branch))
}
