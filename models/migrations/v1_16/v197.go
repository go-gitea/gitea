// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"xorm.io/xorm"
)

func AddRenamedBranchTable(x *xorm.Engine) error {
	type RenamedBranch struct {
		ID          int64 `xorm:"pk autoincr"`
		RepoID      int64 `xorm:"INDEX NOT NULL"`
		From        string
		To          string
		CreatedUnix int64 `xorm:"created"`
	}
	return x.Sync(new(RenamedBranch))
}
