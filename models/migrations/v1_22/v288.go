// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type Blocking struct {
	ID          int64 `xorm:"pk autoincr"`
	BlockerID   int64 `xorm:"UNIQUE(block)"`
	BlockeeID   int64 `xorm:"UNIQUE(block)"`
	Note        string
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

func (*Blocking) TableName() string {
	return "user_blocking"
}

func AddUserBlockingTable(x *xorm.Engine) error {
	return x.Sync(&Blocking{})
}
