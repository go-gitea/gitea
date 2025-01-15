// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// StarList ...
type StarList struct {
	ID   int64 `xorm:"pk autoincr"`
	UID  int64 `xorm:INDEX uid`
	SID  int64 `xorm:INDEX sid`
	Name string
	Desc string

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(StarList))
}
