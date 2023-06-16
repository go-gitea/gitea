// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateVariableTable(x *xorm.Engine) error {
	type ActionVariable struct {
		ID          int64              `xorm:"pk autoincr"`
		OwnerID     int64              `xorm:"UNIQUE(owner_repo_title)"`
		RepoID      int64              `xorm:"UNIQUE(owner_repo_title)"`
		Title       string             `xorm:"UNIQUE(owner_repo_title) NOT NULL"`
		Content     string             `xorm:"LONGTEXT NOT NULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(new(ActionVariable))
}
