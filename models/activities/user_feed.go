// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import "code.gitea.io/gitea/modules/timeutil"

type UserFeed struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(s)"` // Receiver user id.
	ActionID    int64              `xorm:"UNIQUE(s)"` // refer to action table
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}
