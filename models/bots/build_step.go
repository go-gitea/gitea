// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

type BuildStep struct {
	ID      int64
	StageID int64 `xorm:"index"`
	Number  int64
	Name    string
	Kind    string
	Type    string
	Status  core.BuildStatus
	Started timeutil.TimeStamp
	Stopped timeutil.TimeStamp
	Created timeutil.TimeStamp `xorm:"created"`
}

func (bj BuildStep) TableName() string {
	return "bots_build_step"
}

func init() {
	db.RegisterModel(new(BuildStep))
}
