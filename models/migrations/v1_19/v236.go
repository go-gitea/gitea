// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateSecretsTable(x *xorm.Engine) error {
	type Secret struct {
		ID          int64
		UserID      int64              `xorm:"index NOTNULL"`
		RepoID      int64              `xorm:"index NOTNULL"`
		Name        string             `xorm:"NOTNULL"`
		Data        string             `xorm:"LONGTEXT"`
		PullRequest bool               `xorm:"NOTNULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOTNULL"`
	}

	return x.Sync(new(Secret))
}
