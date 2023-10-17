// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateActionTasksVersionTable(x *xorm.Engine) error {
	type ActionTasksVersion struct {
		ID          int64 `xorm:"pk autoincr"`
		OwnerID     int64 `xorm:"UNIQUE(owner_repo)"`
		RepoID      int64 `xorm:"INDEX UNIQUE(owner_repo)"`
		Version     int64
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(new(ActionTasksVersion))
}
